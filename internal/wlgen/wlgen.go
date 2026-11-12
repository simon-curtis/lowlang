package wlgen

import (
	"encoding/xml"
	"fmt"
	"slices"
	"strings"

	"lowlang/internal/check"
	"lowlang/internal/emit"
)

type Protocol struct {
	Name       string      `xml:"name,attr"`
	Interfaces []Interface `xml:"interface"`
}

type Interface struct {
	Name     string    `xml:"name,attr"`
	Version  int       `xml:"version,attr"`
	Requests []Message `xml:"request"`
	Events   []Message `xml:"event"`
	Enums    []Enum    `xml:"enum"`
}

type Message struct {
	Name  string `xml:"name,attr"`
	Type  string `xml:"type,attr"`
	Since int    `xml:"since,attr"`
	Args  []Arg  `xml:"arg"`
}

type Arg struct {
	Name      string `xml:"name,attr"`
	Type      string `xml:"type,attr"`
	Interface string `xml:"interface,attr"`
	AllowNull bool   `xml:"allow-null,attr"`
}

type Enum struct {
	Name    string  `xml:"name,attr"`
	Entries []Entry `xml:"entry"`
}

type Entry struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

var cTypes = map[string]string{
	"int": "i32", "uint": "u32", "fixed": "i32", "string": "ptr[u8]", "object": "ptr", "new_id": "ptr", "array": "ptr", "fd": "i32",
}

var signatureCodes = map[string]string{
	"int": "i", "uint": "u", "fixed": "f", "string": "s", "object": "o", "new_id": "n", "array": "a", "fd": "h",
}

type generator struct {
	emit.Lines
	interfaces map[string]bool
}

func Parse(data []byte) (*Protocol, error) {
	var p Protocol
	if err := xml.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	if p.Name == "" {
		return nil, fmt.Errorf("not a Wayland protocol: missing <protocol name>")
	}
	return &p, nil
}

func Generate(protocols []*Protocol) (string, error) {
	g := &generator{interfaces: map[string]bool{}}
	var all []Interface
	for _, p := range protocols {
		for _, i := range p.Interfaces {
			if g.interfaces[i.Name] {
				return "", fmt.Errorf("interface %s is declared twice", i.Name)
			}
			g.interfaces[i.Name] = true
			all = append(all, i)
		}
	}
	if err := g.references(all); err != nil {
		return "", err
	}
	g.prologue()
	g.table(all)
	for _, i := range all {
		g.fill(i)
		g.requests(i)
		g.listener(i)
		g.enums(i)
	}
	return g.String(), nil
}

func (g *generator) references(all []Interface) error {
	for _, i := range all {
		for _, m := range slices.Concat(i.Requests, i.Events) {
			for _, a := range m.Args {
				if a.Interface != "" && !g.interfaces[a.Interface] {
					return fmt.Errorf("%s.%s refers to interface %s, which no given protocol declares", i.Name, m.Name, a.Interface)
				}
				if _, ok := cTypes[a.Type]; !ok {
					return fmt.Errorf("%s.%s argument %s has unknown type %s", i.Name, m.Name, a.Name, a.Type)
				}
			}
		}
	}
	return nil
}

func camel(name string) string {
	var b strings.Builder
	for _, part := range strings.Split(name, "_") {
		if part == "" {
			continue
		}
		b.WriteString(strings.ToUpper(part[:1]))
		b.WriteString(part[1:])
	}
	return b.String()
}

func (g *generator) prologue() {
	g.Line(`extern "wayland-client" {`)
	for _, decl := range []string{
		"fn wl_display_connect(name: ptr[u8]) -> ptr",
		"fn wl_display_disconnect(display: ptr)",
		"fn wl_display_roundtrip(display: ptr) -> i32",
		"fn wl_display_dispatch(display: ptr) -> i32",
		"fn wl_display_dispatch_pending(display: ptr) -> i32",
		"fn wl_display_flush(display: ptr) -> i32",
		"fn wl_display_get_fd(display: ptr) -> i32",
		"fn wl_display_prepare_read(display: ptr) -> i32",
		"fn wl_display_read_events(display: ptr) -> i32",
		"fn wl_display_cancel_read(display: ptr)",
		"fn wl_proxy_destroy(proxy: ptr)",
		"fn wl_proxy_get_version(proxy: ptr) -> u32",
		"fn wl_proxy_marshal_flags(proxy: ptr, opcode: u32, interface: ptr, version: u32, flags: u32, ...) -> ptr",
		"fn wl_proxy_add_listener(proxy: ptr, listener: ptr, data: ptr) -> i32",
	} {
		g.Line("    %s", decl)
	}
	g.Line("}")
	g.Line("")
	g.Line(`extern "c" {`)
	g.Line("    fn malloc(n: i64) -> ptr")
	g.Line("}")
	g.Line("")
	g.Line("extern struct WlInterface {")
	g.Line("    name: ptr[u8]")
	g.Line("    version: i32")
	g.Line("    method_count: i32")
	g.Line("    methods: ptr")
	g.Line("    event_count: i32")
	g.Line("    events: ptr")
	g.Line("}")
	g.Line("")
	g.Line("extern struct WlMessage {")
	g.Line("    name: ptr[u8]")
	g.Line("    signature: ptr[u8]")
	g.Line("    types: ptr")
	g.Line("}")
	g.Line("")
	g.Line("fn wl_c_string(s: str) -> ptr[u8] {")
	g.Line("    let p: ptr[u8] = malloc(s.len + 1)")
	g.Line("    let bytes = view(p, s.len + 1)")
	g.Line("    for i in 0..s.len {")
	g.Line("        bytes[i] = s[i]")
	g.Line("    }")
	g.Line("    bytes[s.len] = 0")
	g.Line("    p")
	g.Line("}")
	g.Line("")
	g.Line("fn wl_types(entries: []ptr) -> ptr {")
	g.Line("    let p: ptr[ptr] = malloc(8 * entries.len + 8)")
	g.Line("    let slots = view(p, entries.len)")
	g.Line("    for i in 0..entries.len {")
	g.Line("        slots[i] = entries[i]")
	g.Line("    }")
	g.Line("    p")
	g.Line("}")
	g.Line("")
	g.Line("fn wl_message(name: str, signature: str, types: ptr) -> WlMessage {")
	g.Line("    WlMessage { name: wl_c_string(name), signature: wl_c_string(signature), types }")
	g.Line("}")
	g.Line("")
}

func (g *generator) table(all []Interface) {
	g.Line("struct Interfaces {")
	for _, i := range all {
		g.Line("    %s: ptr", i.Name)
	}
	g.Line("}")
	g.Line("")
	g.Line("fn interfaces() -> Interfaces {")
	g.Line("    let ifs = Interfaces {")
	for _, i := range all {
		g.Line("        %s: malloc(40)", i.Name)
	}
	g.Line("    }")
	for _, i := range all {
		g.Line("    wl_fill_%s(ifs)", i.Name)
	}
	g.Line("    ifs")
	g.Line("}")
	g.Line("")
}

func signature(m Message) (string, []string) {
	var sig strings.Builder
	var types []string
	if m.Since > 1 {
		fmt.Fprintf(&sig, "%d", m.Since)
	}
	for _, a := range m.Args {
		if a.AllowNull {
			sig.WriteString("?")
		}
		if a.Type == "new_id" && a.Interface == "" {
			sig.WriteString("sun")
			types = append(types, "null", "null", "null")
			continue
		}
		sig.WriteString(signatureCodes[a.Type])
		if a.Interface != "" {
			types = append(types, fmt.Sprintf("ifs.%s", a.Interface))
			continue
		}
		types = append(types, "null")
	}
	return sig.String(), types
}

func (g *generator) messages(kind string, msgs []Message) {
	g.Line("    let %s: ptr[WlMessage] = malloc(%d)", kind, 24*max(1, len(msgs)))
	if len(msgs) == 0 {
		return
	}
	g.Line("    let %s_view = view(%s, %d)", kind, kind, len(msgs))
	for k, m := range msgs {
		sig, types := signature(m)
		list := "[]"
		if len(types) > 0 {
			list = fmt.Sprintf("[%s]", strings.Join(types, ", "))
		}
		g.Line("    let %s_types%d: []ptr = %s", kind, k, list)
		g.Line("    %s_view[%d] = wl_message(%q, %q, wl_types(%s_types%d))", kind, k, m.Name, sig, kind, k)
	}
}

func (g *generator) fill(i Interface) {
	g.Line("fn wl_fill_%s(ifs: Interfaces) {", i.Name)
	g.messages("methods", i.Requests)
	g.messages("events", i.Events)
	g.Line("    let target: ptr[WlInterface] = ifs.%s", i.Name)
	g.Line("    store(target, WlInterface { name: wl_c_string(%q), version: %d, method_count: %d, methods, event_count: %d, events })",
		i.Name, i.Version, len(i.Requests), len(i.Events))
	g.Line("}")
	g.Line("")
}

func newID(m Message) (Arg, bool) {
	for _, a := range m.Args {
		if a.Type == "new_id" {
			return a, true
		}
	}
	return Arg{}, false
}

func (g *generator) requests(i Interface) {
	self := check.Ident(i.Name)
	destroy := false
	for opcode, m := range i.Requests {
		name := fmt.Sprintf("%s_%s", i.Name, m.Name)
		destroy = destroy || m.Name == "destroy"
		id, creates := newID(m)
		params := []string{fmt.Sprintf("%s: ptr", self)}
		if creates && id.Interface != "" {
			params = append([]string{"ifs: Interfaces"}, params...)
		}
		var values []string
		for _, a := range m.Args {
			switch {
			case a.Type == "new_id" && a.Interface == "":
				params = append(params, "interface: ptr", "version: u32")
				values = append(values, "description.name", "version", "null")
			case a.Type == "new_id":
				values = append(values, "null")
			default:
				params = append(params, fmt.Sprintf("%s: %s", check.Ident(a.Name), cTypes[a.Type]))
				values = append(values, check.Ident(a.Name))
			}
		}
		result := ""
		if creates {
			result = " -> ptr"
		}
		g.Line("fn %s(%s)%s {", name, strings.Join(params, ", "), result)
		target, version := "null", fmt.Sprintf("wl_proxy_get_version(%s)", self)
		switch {
		case creates && id.Interface == "":
			g.Line("    let described: ptr[WlInterface] = interface")
			g.Line("    let description = load(described)")
			target, version = "interface", "version"
		case creates:
			target = fmt.Sprintf("ifs.%s", id.Interface)
		}
		if version != "version" {
			g.Line("    let version = %s", version)
		}
		flags := 0
		if m.Type == "destructor" {
			flags = 1
		}
		call := fmt.Sprintf("wl_proxy_marshal_flags(%s, %d, %s, version, %d", self, opcode, target, flags)
		if len(values) > 0 {
			call = fmt.Sprintf("%s, %s", call, strings.Join(values, ", "))
		}
		g.Line("    %s)", call)
		g.Line("}")
		g.Line("")
	}
	if !destroy && i.Name != "wl_display" {
		g.Line("fn %s_destroy(%s: ptr) {", i.Name, self)
		g.Line("    wl_proxy_destroy(%s)", self)
		g.Line("}")
		g.Line("")
	}
}

func eventParams(m Message) ([]string, []string) {
	types := []string{"ptr", "ptr"}
	params := []string{"data: ptr", "proxy: ptr"}
	for _, a := range m.Args {
		types = append(types, cTypes[a.Type])
		params = append(params, fmt.Sprintf("%s: %s", check.Ident(a.Name), cTypes[a.Type]))
	}
	return types, params
}

func (g *generator) listener(i Interface) {
	if len(i.Events) == 0 || i.Name == "wl_display" {
		return
	}
	listener := fmt.Sprintf("%sListener", camel(i.Name))
	for _, m := range i.Events {
		_, params := eventParams(m)
		g.Line("fn %s_%s_ignore(%s) {", i.Name, m.Name, strings.Join(params, ", "))
		g.Line("}")
		g.Line("")
	}
	g.Line("extern struct %s {", listener)
	for _, m := range i.Events {
		types, _ := eventParams(m)
		g.Line("    %s: extern fn(%s) = %s_%s_ignore", check.Ident(m.Name), strings.Join(types, ", "), i.Name, m.Name)
	}
	g.Line("}")
	g.Line("")
	self := check.Ident(i.Name)
	g.Line("fn %s_add_listener(%s: ptr, listener: %s, data: ptr) -> i32 {", i.Name, self, listener)
	g.Line("    let copy: ptr[%s] = malloc(%d)", listener, 8*len(i.Events))
	g.Line("    store(copy, listener)")
	g.Line("    wl_proxy_add_listener(%s, copy, data)", self)
	g.Line("}")
	g.Line("")
}

func (g *generator) enums(i Interface) {
	for _, e := range i.Enums {
		for _, entry := range e.Entries {
			g.Line("fn %s_%s_%s() -> u32 {", i.Name, e.Name, entry.Name)
			g.Line("    %s", entry.Value)
			g.Line("}")
			g.Line("")
		}
	}
}
