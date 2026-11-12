package wlgen

import (
	"encoding/xml"
	"fmt"
	"slices"
	"strings"

	"lowlang/internal/check"
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
	out        strings.Builder
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
	return g.out.String(), nil
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

func (g *generator) line(format string, args ...any) {
	fmt.Fprintf(&g.out, format, args...)
	g.out.WriteString("\n")
}

func ident(name string) string {
	if check.Reserved(name) {
		return fmt.Sprintf("%s_", name)
	}
	return name
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
	g.line(`extern "wayland-client" {`)
	for _, decl := range []string{
		"pub fn wl_display_connect(name: ptr[u8]) -> ptr",
		"pub fn wl_display_disconnect(display: ptr)",
		"pub fn wl_display_roundtrip(display: ptr) -> i32",
		"pub fn wl_display_dispatch(display: ptr) -> i32",
		"pub fn wl_display_dispatch_pending(display: ptr) -> i32",
		"pub fn wl_display_flush(display: ptr) -> i32",
		"pub fn wl_display_get_fd(display: ptr) -> i32",
		"pub fn wl_display_prepare_read(display: ptr) -> i32",
		"pub fn wl_display_read_events(display: ptr) -> i32",
		"pub fn wl_display_cancel_read(display: ptr)",
		"pub fn wl_proxy_destroy(proxy: ptr)",
		"pub fn wl_proxy_get_version(proxy: ptr) -> u32",
		"fn wl_proxy_marshal_flags(proxy: ptr, opcode: u32, interface: ptr, version: u32, flags: u32, ...) -> ptr",
		"fn wl_proxy_add_listener(proxy: ptr, listener: ptr, data: ptr) -> i32",
	} {
		g.line("    %s", decl)
	}
	g.line("}")
	g.line("")
	g.line(`extern "c" {`)
	g.line("    fn malloc(n: i64) -> ptr")
	g.line("}")
	g.line("")
	g.line("extern struct WlInterface {")
	g.line("    name: ptr[u8]")
	g.line("    version: i32")
	g.line("    method_count: i32")
	g.line("    methods: ptr")
	g.line("    event_count: i32")
	g.line("    events: ptr")
	g.line("}")
	g.line("")
	g.line("extern struct WlMessage {")
	g.line("    name: ptr[u8]")
	g.line("    signature: ptr[u8]")
	g.line("    types: ptr")
	g.line("}")
	g.line("")
	g.line("fn wl_c_string(s: str) -> ptr[u8] {")
	g.line("    let p: ptr[u8] = malloc(s.len + 1)")
	g.line("    let bytes = view(p, s.len + 1)")
	g.line("    for i in 0..s.len {")
	g.line("        bytes[i] = s[i]")
	g.line("    }")
	g.line("    bytes[s.len] = 0")
	g.line("    p")
	g.line("}")
	g.line("")
	g.line("fn wl_types(entries: []ptr) -> ptr {")
	g.line("    let p: ptr[ptr] = malloc(8 * entries.len + 8)")
	g.line("    let slots = view(p, entries.len)")
	g.line("    for i in 0..entries.len {")
	g.line("        slots[i] = entries[i]")
	g.line("    }")
	g.line("    p")
	g.line("}")
	g.line("")
	g.line("fn wl_message(name: str, signature: str, types: ptr) -> WlMessage {")
	g.line("    WlMessage { name: wl_c_string(name), signature: wl_c_string(signature), types }")
	g.line("}")
	g.line("")
}

func (g *generator) table(all []Interface) {
	g.line("pub struct Interfaces {")
	for _, i := range all {
		g.line("    pub %s: ptr", i.Name)
	}
	g.line("}")
	g.line("")
	g.line("pub fn interfaces() -> Interfaces {")
	g.line("    let ifs = Interfaces {")
	for _, i := range all {
		g.line("        %s: malloc(40)", i.Name)
	}
	g.line("    }")
	for _, i := range all {
		g.line("    wl_fill_%s(ifs)", i.Name)
	}
	g.line("    ifs")
	g.line("}")
	g.line("")
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
	g.line("    let %s: ptr[WlMessage] = malloc(%d)", kind, 24*max(1, len(msgs)))
	if len(msgs) == 0 {
		return
	}
	g.line("    let %s_view = view(%s, %d)", kind, kind, len(msgs))
	for k, m := range msgs {
		sig, types := signature(m)
		list := "[]"
		if len(types) > 0 {
			list = fmt.Sprintf("[%s]", strings.Join(types, ", "))
		}
		g.line("    let %s_types%d: []ptr = %s", kind, k, list)
		g.line("    %s_view[%d] = wl_message(%q, %q, wl_types(%s_types%d))", kind, k, m.Name, sig, kind, k)
	}
}

func (g *generator) fill(i Interface) {
	g.line("fn wl_fill_%s(ifs: Interfaces) {", i.Name)
	g.messages("methods", i.Requests)
	g.messages("events", i.Events)
	g.line("    let target: ptr[WlInterface] = ifs.%s", i.Name)
	g.line("    store(target, WlInterface { name: wl_c_string(%q), version: %d, method_count: %d, methods, event_count: %d, events })",
		i.Name, i.Version, len(i.Requests), len(i.Events))
	g.line("}")
	g.line("")
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
	self := ident(i.Name)
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
				params = append(params, fmt.Sprintf("%s: %s", ident(a.Name), cTypes[a.Type]))
				values = append(values, ident(a.Name))
			}
		}
		result := ""
		if creates {
			result = " -> ptr"
		}
		g.line("pub fn %s(%s)%s {", name, strings.Join(params, ", "), result)
		target, version := "null", fmt.Sprintf("wl_proxy_get_version(%s)", self)
		switch {
		case creates && id.Interface == "":
			g.line("    let described: ptr[WlInterface] = interface")
			g.line("    let description = load(described)")
			target, version = "interface", "version"
		case creates:
			target = fmt.Sprintf("ifs.%s", id.Interface)
		}
		if version != "version" {
			g.line("    let version = %s", version)
		}
		flags := 0
		if m.Type == "destructor" {
			flags = 1
		}
		call := fmt.Sprintf("wl_proxy_marshal_flags(%s, %d, %s, version, %d", self, opcode, target, flags)
		if len(values) > 0 {
			call = fmt.Sprintf("%s, %s", call, strings.Join(values, ", "))
		}
		g.line("    %s)", call)
		g.line("}")
		g.line("")
	}
	if !destroy && i.Name != "wl_display" {
		g.line("pub fn %s_destroy(%s: ptr) {", i.Name, self)
		g.line("    wl_proxy_destroy(%s)", self)
		g.line("}")
		g.line("")
	}
}

func eventParams(m Message) ([]string, []string) {
	types := []string{"ptr", "ptr"}
	params := []string{"data: ptr", "proxy: ptr"}
	for _, a := range m.Args {
		types = append(types, cTypes[a.Type])
		params = append(params, fmt.Sprintf("%s: %s", ident(a.Name), cTypes[a.Type]))
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
		g.line("fn %s_%s_ignore(%s) {", i.Name, m.Name, strings.Join(params, ", "))
		g.line("}")
		g.line("")
	}
	g.line("pub extern struct %s {", listener)
	for _, m := range i.Events {
		types, _ := eventParams(m)
		g.line("    pub %s: extern fn(%s) = %s_%s_ignore", ident(m.Name), strings.Join(types, ", "), i.Name, m.Name)
	}
	g.line("}")
	g.line("")
	self := ident(i.Name)
	g.line("pub fn %s_add_listener(%s: ptr, listener: %s, data: ptr) -> i32 {", i.Name, self, listener)
	g.line("    let copy: ptr[%s] = malloc(%d)", listener, 8*len(i.Events))
	g.line("    store(copy, listener)")
	g.line("    wl_proxy_add_listener(%s, copy, data)", self)
	g.line("}")
	g.line("")
}

func (g *generator) enums(i Interface) {
	for _, e := range i.Enums {
		for _, entry := range e.Entries {
			g.line("pub fn %s_%s_%s() -> u32 {", i.Name, e.Name, entry.Name)
			g.line("    %s", entry.Value)
			g.line("}")
			g.line("")
		}
	}
}
