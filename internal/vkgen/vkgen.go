package vkgen

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"lowlang/internal/check"
	"lowlang/internal/emit"
)

type Selection struct {
	Commands   []string
	Extensions []string
	Types      []string
}

func ParseSelection(text string) (Selection, error) {
	var s Selection
	for i, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		kind, name, ok := strings.Cut(line, " ")
		switch {
		case ok && kind == "command":
			s.Commands = append(s.Commands, strings.TrimSpace(name))
		case ok && kind == "extension":
			s.Extensions = append(s.Extensions, strings.TrimSpace(name))
		case ok && kind == "type":
			s.Types = append(s.Types, strings.TrimSpace(name))
		default:
			return s, fmt.Errorf("line %d: expected \"command NAME\", \"type NAME\" or \"extension NAME\", found %q", i+1, line)
		}
	}
	return s, nil
}

var baseTypes = map[string]string{
	"uint32_t": "u32", "int32_t": "i32", "uint64_t": "i64", "int64_t": "i64", "uint16_t": "u16", "int16_t": "u16",
	"uint8_t": "u8", "int8_t": "u8", "char": "u8", "float": "f32", "double": "f64", "size_t": "i64", "int": "i32",
}

var baseSizes = map[string]int{"u32": 4, "i32": 4, "i64": 8, "u16": 2, "u8": 1, "f32": 4, "f64": 8, "ptr": 8}

type value struct {
	name  string
	v     int64
	ok    bool
	alias string
}

type generator struct {
	reg       *Registry
	types     map[string]*Type
	commands  map[string]*Command
	constants map[string]Enum
	values    map[string][]value
	widths    map[string]string
	selected  []string
	emitted   map[string]bool
	order     []string
	enumUse   map[string]bool
	emit.Lines
}

func Generate(reg *Registry, sel Selection) (string, error) {
	g := &generator{
		reg:       reg,
		types:     map[string]*Type{},
		commands:  map[string]*Command{},
		constants: map[string]Enum{},
		values:    map[string][]value{},
		widths:    map[string]string{},
		emitted:   map[string]bool{},
		enumUse:   map[string]bool{},
	}
	if err := g.index(); err != nil {
		return "", err
	}
	if err := g.collectValues(sel.Extensions); err != nil {
		return "", err
	}
	for _, name := range sel.Commands {
		cmd, err := g.command(name)
		if err != nil {
			return "", err
		}
		if err := g.requireCommand(cmd); err != nil {
			return "", fmt.Errorf("%s: %w", name, err)
		}
	}
	for _, name := range sel.Types {
		if err := g.require(name); err != nil {
			return "", err
		}
	}
	slices.Sort(g.order)
	g.prologue()
	for _, name := range g.order {
		if err := g.emitType(name); err != nil {
			return "", fmt.Errorf("%s: %w", name, err)
		}
	}
	if err := g.emitCommands(sel.Commands); err != nil {
		return "", err
	}
	if err := g.emitConstants(); err != nil {
		return "", err
	}
	return g.String(), nil
}

func (g *generator) index() error {
	for i := range g.reg.Types {
		t := &g.reg.Types[i]
		name := t.Name
		if name == "" {
			name = t.NameInside
		}
		if name == "" {
			name = t.ProtoName
		}
		if name == "" || !forVulkan(t.API) {
			continue
		}
		if _, seen := g.types[name]; !seen {
			g.types[name] = t
		}
	}
	for i := range g.reg.Commands {
		c := &g.reg.Commands[i]
		if !forVulkan(c.API) {
			continue
		}
		name := c.Name
		if name == "" {
			d, err := ParseDecl(c.Proto.Inner)
			if err != nil {
				return fmt.Errorf("command prototype %q: %w", c.Proto.Inner, err)
			}
			name = d.Name
		}
		g.commands[name] = c
	}
	for _, e := range g.reg.Enums {
		if e.Name == "API Constants" {
			for _, c := range e.Values {
				g.constants[c.Name] = c
			}
			continue
		}
		width := "i32"
		if e.Type == "bitmask" {
			width = "u32"
			if e.BitWidth == "64" {
				width = "i64"
			}
		}
		g.widths[e.Name] = width
	}
	return nil
}

func (g *generator) collectValues(extensions []string) error {
	for _, e := range g.reg.Enums {
		if e.Name == "API Constants" {
			continue
		}
		for _, v := range e.Values {
			if !forVulkan(v.API) {
				continue
			}
			if err := g.addValue(e.Name, v, 0); err != nil {
				return err
			}
		}
	}
	for _, f := range g.reg.Features {
		if !forVulkan(f.API) {
			continue
		}
		for _, r := range f.Requires {
			if !forVulkan(r.API) {
				continue
			}
			for _, v := range r.Enums {
				if v.Extends != "" {
					if err := g.addValue(v.Extends, v, 0); err != nil {
						return err
					}
				}
			}
		}
	}
	for _, want := range extensions {
		i := slices.IndexFunc(g.reg.Extensions, func(x Extension) bool { return x.Name == want })
		if i < 0 {
			return fmt.Errorf("unknown extension %s", want)
		}
		ext := g.reg.Extensions[i]
		number, err := strconv.Atoi(ext.Number)
		if err != nil {
			return fmt.Errorf("extension %s has no number", want)
		}
		for _, r := range ext.Requires {
			if !forVulkan(r.API) {
				continue
			}
			for _, v := range r.Enums {
				if v.Extends != "" {
					if err := g.addValue(v.Extends, v, number); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func (g *generator) addValue(enum string, e Enum, extension int) error {
	for _, have := range g.values[enum] {
		if have.name == e.Name {
			return nil
		}
	}
	v, ok, err := enumValue(e, extension)
	if err != nil {
		return fmt.Errorf("%s: %w", e.Name, err)
	}
	g.values[enum] = append(g.values[enum], value{name: e.Name, v: v, ok: ok, alias: e.Alias})
	return nil
}

func (g *generator) command(name string) (*Command, error) {
	cmd, ok := g.commands[name]
	for ok && cmd.Alias != "" {
		cmd, ok = g.commands[cmd.Alias]
	}
	if !ok {
		return nil, fmt.Errorf("unknown command %s", name)
	}
	return cmd, nil
}

func (g *generator) resolve(name string) *Type {
	t := g.types[name]
	for t != nil && t.Alias != "" {
		t = g.types[t.Alias]
	}
	return t
}

func (g *generator) requireCommand(cmd *Command) error {
	proto, err := ParseDecl(cmd.Proto.Inner)
	if err != nil {
		return err
	}
	if err := g.require(proto.Type); err != nil {
		return err
	}
	for _, p := range cmd.Params {
		if !forVulkan(p.API) {
			continue
		}
		d, err := ParseDecl(p.Inner)
		if err != nil {
			return err
		}
		if err := g.require(d.Type); err != nil {
			return err
		}
	}
	return nil
}

func (g *generator) require(name string) error {
	if _, base := baseTypes[name]; base || name == "void" || g.emitted[name] {
		return nil
	}
	t := g.resolve(name)
	if t == nil {
		return fmt.Errorf("unknown type %s", name)
	}
	g.emitted[name] = true
	switch t.Category {
	case "struct", "union":
		g.order = append(g.order, name)
		for _, m := range t.Members {
			if !forVulkan(m.API) {
				continue
			}
			d, err := ParseDecl(m.Inner)
			if err != nil {
				return err
			}
			if d.BitField {
				return fmt.Errorf("%s.%s is a bit-field", name, d.Name)
			}
			if err := g.require(d.Type); err != nil {
				return err
			}
		}
	case "handle":
		g.order = append(g.order, name)
	case "enum":
		g.enumUse[name] = true
	case "bitmask":
		bits := t.Requires
		if bits == "" {
			bits = t.BitValues
		}
		if bits != "" {
			g.enumUse[bits] = true
		}
	}
	return nil
}

func (g *generator) lowType(name string) (string, error) {
	if base, ok := baseTypes[name]; ok {
		return base, nil
	}
	t := g.resolve(name)
	if t == nil {
		return "", fmt.Errorf("unknown type %s", name)
	}
	switch t.Category {
	case "basetype":
		under := typeTag.FindStringSubmatch(t.Inner)
		if under == nil {
			return "ptr", nil
		}
		return g.lowType(under[1])
	case "bitmask":
		if strings.Contains(t.Inner, "VkFlags64") {
			return "i64", nil
		}
		return "u32", nil
	case "enum":
		if width := g.widths[t.Name]; width != "" {
			return width, nil
		}
		return "i32", nil
	case "handle", "struct", "union":
		return name, nil
	}
	return "ptr", nil
}

func (g *generator) pointee(d Decl) (string, error) {
	if d.Type == "void" || d.Stars > 1 {
		return "ptr", nil
	}
	if d.Type == "char" {
		return "ptr[u8]", nil
	}
	t, err := g.lowType(d.Type)
	if err != nil {
		return "", err
	}
	if t == "ptr" {
		return "ptr", nil
	}
	return fmt.Sprintf("ptr[%s]", t), nil
}

func (g *generator) memberType(d Decl) (string, error) {
	if d.Stars > 0 {
		return g.pointee(d)
	}
	return g.lowType(d.Type)
}

func (g *generator) defaultFor(t string) string {
	switch {
	case strings.HasPrefix(t, "ptr"):
		return "null"
	case t == "f32" || t == "f64":
		return "0.0"
	case baseSizes[t] > 0:
		return "0"
	}
	return fmt.Sprintf("%s {}", t)
}

func (g *generator) dimension(d string) (int, error) {
	if n, err := strconv.Atoi(d); err == nil {
		return n, nil
	}
	c, ok := g.constants[d]
	if !ok {
		return 0, fmt.Errorf("unknown array size %s", d)
	}
	v, err := parseNumber(c.Value)
	return int(v), err
}

func (g *generator) count(d Decl) (int, error) {
	n := 1
	for _, dim := range d.Dims {
		k, err := g.dimension(dim)
		if err != nil {
			return 0, err
		}
		n *= k
	}
	return n, nil
}

func (g *generator) layout(name string, stars int) (int, int, error) {
	if stars > 0 {
		return 8, 8, nil
	}
	t, err := g.lowType(name)
	if err != nil {
		return 0, 0, err
	}
	if size, ok := baseSizes[t]; ok {
		return size, size, nil
	}
	ty := g.resolve(name)
	if ty.Category == "handle" {
		return 8, 8, nil
	}
	total, align, union := 0, 1, ty.Category == "union"
	for _, m := range ty.Members {
		if !forVulkan(m.API) {
			continue
		}
		d, err := ParseDecl(m.Inner)
		if err != nil {
			return 0, 0, err
		}
		n, err := g.count(d)
		if err != nil {
			return 0, 0, err
		}
		size, a, err := g.layout(d.Type, d.Stars)
		if err != nil {
			return 0, 0, err
		}
		align = max(align, a)
		if union {
			total = max(total, size*n)
			continue
		}
		total = (total+a-1)/a*a + size*n
	}
	return (total + align - 1) / align * align, align, nil
}

func (g *generator) prologue() {
	g.Line("pub fn vk_make_api_version(variant: u32, major: u32, minor: u32, patch: u32) -> u32 {")
	g.Line("    (variant << 29) | (major << 22) | (minor << 12) | patch")
	g.Line("}")
	g.Line("")
	g.Line("fn vk_string(bytes: []u8) -> str {")
	g.Line("    var n = 0")
	g.Line("    for n < bytes.len && bytes[n] != 0 {")
	g.Line("        n += 1")
	g.Line("    }")
	g.Line("    str(bytes[0..n])")
	g.Line("}")
	g.Line("")
}

func (g *generator) emitType(name string) error {
	t := g.resolve(name)
	if t.Category == "handle" {
		field, def := "i64", "0"
		if strings.Contains(t.Inner, "VK_DEFINE_HANDLE") {
			field, def = "ptr", "null"
		}
		g.Line("pub extern struct %s {", name)
		g.Line("    pub handle: %s = %s", field, def)
		g.Line("}")
		g.Line("")
		return nil
	}
	members := t.Members
	if t.Category == "union" {
		widest, err := g.widest(t)
		if err != nil {
			return err
		}
		members = []Member{widest}
	}
	var arrays []array
	g.Line("pub extern struct %s {", name)
	for _, m := range members {
		if !forVulkan(m.API) {
			continue
		}
		d, err := ParseDecl(m.Inner)
		if err != nil {
			return err
		}
		ty, err := g.memberType(d)
		if err != nil {
			return err
		}
		def := g.defaultFor(ty)
		if d.Name == "sType" && m.Values != "" {
			v, err := g.structureType(m.Values)
			if err != nil {
				return err
			}
			def = strconv.FormatInt(v, 10)
		}
		if len(d.Dims) == 0 {
			g.Line("    pub %s: %s = %s", check.Ident(d.Name), ty, def)
			continue
		}
		n, err := g.count(d)
		if err != nil {
			return err
		}
		for i := range n {
			g.Line("    pub %s_%d: %s = %s", d.Name, i, ty, def)
		}
		d.Dims = []string{strconv.Itoa(n)}
		arrays = append(arrays, array{decl: d, elem: ty})
	}
	g.Line("}")
	g.Line("")
	for _, a := range arrays {
		g.accessor(name, a)
	}
	return nil
}

type array struct {
	decl Decl
	elem string
}

func (g *generator) accessor(owner string, a array) {
	n, _ := strconv.Atoi(a.decl.Dims[0])
	fields := make([]string, n)
	for i := range fields {
		fields[i] = fmt.Sprintf("s.%s_%d", a.decl.Name, i)
	}
	list := strings.Join(fields, ", ")
	if a.decl.Type == "char" && a.decl.Stars == 0 {
		g.Line("pub fn %s_%s(s: %s) -> str {", owner, a.decl.Name, owner)
		g.Line("    let bytes: []u8 = [%s]", list)
		g.Line("    vk_string(bytes)")
		g.Line("}")
		g.Line("")
		return
	}
	g.Line("pub fn %s_%s(s: %s) -> []%s {", owner, a.decl.Name, owner, a.elem)
	g.Line("    [%s]", list)
	g.Line("}")
	g.Line("")
}

func (g *generator) widest(t *Type) (Member, error) {
	var best Member
	size := -1
	for _, m := range t.Members {
		if !forVulkan(m.API) {
			continue
		}
		d, err := ParseDecl(m.Inner)
		if err != nil {
			return best, err
		}
		n, err := g.count(d)
		if err != nil {
			return best, err
		}
		s, _, err := g.layout(d.Type, d.Stars)
		if err != nil {
			return best, err
		}
		if s*n > size {
			best, size = m, s*n
		}
	}
	return best, nil
}

func (g *generator) structureType(name string) (int64, error) {
	for _, v := range g.values["VkStructureType"] {
		if v.name == name && v.ok {
			return v.v, nil
		}
	}
	return 0, fmt.Errorf("unknown structure type %s", name)
}

func firstOptional(optional string) bool {
	first, _, _ := strings.Cut(optional, ",")
	return first == "true"
}

func (g *generator) paramType(p Param, d Decl) (string, bool, error) {
	switch {
	case d.Stars == 0:
		t, err := g.lowType(d.Type)
		return t, false, err
	case d.Stars == 2 && d.Type == "void" && !d.Const:
		return "ptr", true, nil
	case d.Stars > 1 || d.Type == "void" || (d.Type == "char" && d.Const):
		t, err := g.pointee(d)
		return t, false, err
	case p.Len != "" || (d.Const && firstOptional(p.Optional)):
		t, err := g.pointee(d)
		return t, false, err
	}
	t, err := g.lowType(d.Type)
	return t, true, err
}

func (g *generator) emitCommands(names []string) error {
	g.Line(`extern "vulkan" {`)
	for _, name := range names {
		cmd, err := g.command(name)
		if err != nil {
			return err
		}
		proto, err := ParseDecl(cmd.Proto.Inner)
		if err != nil {
			return err
		}
		var params []string
		for _, p := range cmd.Params {
			if !forVulkan(p.API) {
				continue
			}
			d, err := ParseDecl(p.Inner)
			if err != nil {
				return err
			}
			t, mut, err := g.paramType(p, d)
			if err != nil {
				return fmt.Errorf("%s.%s: %w", name, d.Name, err)
			}
			prefix := ""
			if mut {
				prefix = "mut "
			}
			params = append(params, fmt.Sprintf("%s%s: %s", prefix, check.Ident(d.Name), t))
		}
		result := ""
		if proto.Type != "void" || proto.Stars > 0 {
			t, err := g.memberType(proto)
			if err != nil {
				return err
			}
			result = fmt.Sprintf(" -> %s", t)
		}
		g.Line("    pub fn %s(%s)%s", name, strings.Join(params, ", "), result)
	}
	g.Line("}")
	g.Line("")
	return nil
}

func literal(v int64, t string) string {
	switch {
	case t == "u32":
		return strconv.FormatUint(uint64(uint32(v)), 10)
	case v == math.MinInt64:
		return "-9223372036854775807 - 1"
	}
	return strconv.FormatInt(v, 10)
}

var integerConstantTypes = map[string]string{"uint32_t": "u32", "uint64_t": "i64", "int32_t": "i32"}

func (g *generator) constant(name, t, body string) {
	g.Line("pub fn %s() -> %s {", name, t)
	g.Line("    %s", body)
	g.Line("}")
	g.Line("")
}

func (g *generator) emitConstants() error {
	var enums []string
	for name := range g.enumUse {
		enums = append(enums, name)
	}
	enums = append(enums, "VkStructureType", "VkResult")
	slices.Sort(enums)
	enums = slices.Compact(enums)
	done := map[string]bool{}
	for _, enum := range enums {
		t := g.widths[enum]
		if t == "" {
			t = "i32"
		}
		known := map[string]value{}
		for _, v := range g.values[enum] {
			known[v.name] = v
		}
		for _, v := range g.values[enum] {
			target := v
			for !target.ok && target.alias != "" {
				target = known[target.alias]
			}
			if !target.ok || done[v.name] {
				continue
			}
			done[v.name] = true
			g.constant(v.name, t, literal(target.v, t))
		}
	}
	var names []string
	for name := range g.constants {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		c := g.constants[name]
		if c.Alias != "" {
			continue
		}
		switch c.Type {
		case "float":
			g.constant(name, "f32", strings.TrimSuffix(strings.TrimSuffix(c.Value, "F"), "f"))
		case "uint32_t", "uint64_t", "int32_t":
			if err := g.integerConstant(name, c); err != nil {
				return err
			}
		default:
			return fmt.Errorf("API constant %s has unsupported type %q", name, c.Type)
		}
	}
	return nil
}

func (g *generator) integerConstant(name string, c Enum) error {
	v, err := parseNumber(c.Value)
	if err != nil {
		return fmt.Errorf("API constant %s: %w", name, err)
	}
	t := integerConstantTypes[c.Type]
	g.constant(name, t, literal(v, t))
	return nil
}
