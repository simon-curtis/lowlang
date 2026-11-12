package check

import (
	"fmt"
	"strings"

	"lowlang/internal/ast"
	"lowlang/internal/builtin"
	"lowlang/internal/diag"
	"lowlang/internal/lexer"
	"lowlang/internal/parser"
	"lowlang/internal/types"
)

const (
	serdeModule = "std/serde"
	serdeField  = "Field"
)

type derivation int

const (
	serialize derivation = iota
	deserialize
	describe
	render
)

var derivationNames = [...]string{serialize: "__write", deserialize: "__read", describe: "__describe", render: "__render"}

type synth struct {
	c     *Checker
	home  *ast.File
	pos   diag.Pos
	bound map[string]*types.Type
	names map[*types.Type]string
	out   strings.Builder
	text  strings.Builder
	temps int
}

func (c *Checker) serdeType(name string, pos diag.Pos) *types.Type {
	if c.file.Module != serdeModule {
		c.fail(pos, "only %s may derive code from types", serdeModule)
	}
	t, ok := c.local().types[name]
	if !ok {
		c.fail(pos, "%s has no type %s", serdeModule, name)
	}
	return t
}

func (c *Checker) serdeFieldType(owner *types.Type, f *types.Field, pos diag.Pos) *types.Type {
	serde, loaded := c.modules[serdeModule]
	if !loaded || serde.generics[serdeField] == nil {
		c.fail(pos, "%s.%s refers to a field and needs %s to be imported", owner, f.Name, serdeModule)
	}
	return c.instantiateType(serde.generics[serdeField], []*types.Type{owner, f.Type}, pos)
}

func (c *Checker) serializeCall(e *ast.Call) {
	t := c.plain(e.Args[0], nil)
	c.expectType(e.Args[1], c.serdeType("Writer", e.Pos))
	e.Type = types.Void
	if !t.Abstract() {
		e.Target = c.derive(serialize, t, e.Pos)
	}
}

func (c *Checker) serializeIntoCall(e *ast.Call) {
	c.expectType(e.Args[0], c.serdeType("Writer", e.Pos))
	path, ok := e.Args[1].(*ast.Ref)
	if !ok {
		c.fail(e.Pos, "__serialize_into needs a mut []i64")
	}
	c.ref(path, c.universe.Slice(types.I64))
	t := c.plain(e.Args[2], nil)
	c.exclusive(e.Args)
	e.Builtin, e.Type = builtin.None, types.Void
	if !t.Abstract() {
		e.Target = c.derive(serialize, t, e.Pos)
	}
}

func (c *Checker) derivedCall(e *ast.Call, inst *ast.Instance) bool {
	id, ok := inst.X.(*ast.Ident)
	if !ok || (id.Name != "__deserialize" && id.Name != "__schema") {
		return false
	}
	if len(inst.Types) != 1 {
		c.fail(inst.Pos, "%s takes 1 type argument, found %d", id.Name, len(inst.Types))
	}
	t := c.resolve(inst.Types[0])
	kind := describe
	if id.Name == "__deserialize" {
		kind = deserialize
		if len(e.Args) != 1 {
			c.fail(e.Pos, "__deserialize takes 1 argument, found %d", len(e.Args))
		}
		c.expectType(e.Args[0], c.serdeType("Reader", e.Pos))
		e.Type = c.universe.Result(t, c.serdeType("Error", e.Pos))
	} else {
		if len(e.Args) != 0 {
			c.fail(e.Pos, "__schema takes no arguments, found %d", len(e.Args))
		}
		e.Type = c.serdeType("Schema", e.Pos)
	}
	if !t.Abstract() {
		e.Target = c.derive(kind, t, e.Pos)
	}
	return true
}

func (c *Checker) derive(kind derivation, t *types.Type, pos diag.Pos) *ast.Func {
	key := fmt.Sprintf("%d:%s", kind, typeKey([]*types.Type{t}))
	if fn, ok := c.derived[key]; ok {
		return fn
	}
	s := &synth{c: c, home: c.derivationHome(kind), pos: pos, bound: map[string]*types.Type{}, names: map[*types.Type]string{}}
	s.function(kind, t)
	fn := s.parse(kind, t)
	c.derived[key] = fn
	c.within(s.home, fn.Bound, func() { c.signature(fn) })
	s.home.Funcs = append(s.home.Funcs, fn)
	c.pending = append(c.pending, fn)
	return fn
}

func (s *synth) parse(kind derivation, t *types.Type) *ast.Func {
	name := types.InstanceName(derivationNames[kind], []*types.Type{t})
	source := s.out.String()
	file, err := parser.ParseSource([]byte(source), diag.Start(fmt.Sprintf("<%s>", name)), true)
	if err != nil {
		panic(fmt.Sprintf("derived source for %s does not parse: %v\n%s", t, err, source))
	}
	fn := file.Funcs[0]
	fn.Name = name
	fn.File = s.home
	fn.Bound = s.bound
	fn.Trusted = true
	return fn
}

func (s *synth) name(t *types.Type) string {
	if n, ok := s.names[t]; ok {
		return n
	}
	n := fmt.Sprintf("__X%d", len(s.names))
	s.names[t] = n
	s.bound[n] = t
	return n
}

func (s *synth) temp(prefix string) string {
	s.temps++
	return fmt.Sprintf("%s%d", prefix, s.temps)
}

func (s *synth) line(format string, args ...any) {
	s.flush()
	fmt.Fprintf(&s.out, format, args...)
	s.out.WriteByte('\n')
}

func (s *synth) function(kind derivation, t *types.Type) {
	self := s.name(t)
	switch kind {
	case serialize:
		s.bound["__W"] = s.c.serdeType("Writer", s.pos)
		s.line("fn derived(w: __W, mut path: []i64, v: %s) {", self)
		s.serialize(t)
	case deserialize:
		s.bound["__R"] = s.c.serdeType("Reader", s.pos)
		s.bound["__E"] = s.c.serdeType("Error", s.pos)
		s.line("fn derived(r: __R) -> %s ! __E {", self)
		s.deserialize(t)
	case describe:
		s.bound["__S"] = s.c.serdeType("Schema", s.pos)
		s.line("fn derived() -> __S {")
		s.line("%s", s.schema(t, nil))
	case render:
		s.line("fn derived(mut b: []u8, mut path: []i64, v: %s) {", self)
		s.render(t)
	}
	s.line("}")
}

func (s *synth) unsupported(t *types.Type) {
	s.c.fail(s.pos, "cannot derive serialization for %s", t)
}

func (s *synth) serialize(t *types.Type) {
	switch {
	case t == types.Bool:
		s.line("w.boolean(v)")
	case t == types.U64:
		s.line("w.unsigned(v)")
	case t.Kind == types.IntKind:
		s.line("w.integer(i64(v))")
	case t.Kind == types.FloatKind:
		s.line("w.float(f64(v))")
	case t == types.Str:
		s.line("w.text(v)")
	case t.Kind == types.SliceKind && t.Elem == types.U8:
		s.line("w.binary(v)")
	case t.Kind == types.SliceKind:
		s.line("w.begin_seq()\nfor x in v {\n__serialize_into(w, mut path, x)\n}\nw.end_seq()")
	case t.Kind == types.MapKind:
		s.line("w.begin_map()\nfor k, x in v {\nw.key(%s)\n__serialize_into(w, mut path, x)\n}\nw.end_map()", s.keyCodec(t.Key).write)
	case isOptional(t):
		s.line("match v {\nsome(x) => __serialize_into(w, mut path, x)\nnone => w.null()\n}")
	case t.Kind == types.PointerKind:
		s.serializePointer(t)
	case t.Kind == types.StructKind:
		s.line("w.begin_struct()")
		for _, f := range t.Fields {
			s.line("w.field(%s)\n__serialize_into(w, mut path, v.%s)", lexer.Quote(f.Name), f.Name)
		}
		s.line("w.end_struct()")
	case t.Kind == types.EnumKind:
		s.serializeEnum(t)
	default:
		s.unsupported(t)
	}
}

func (s *synth) serializePointer(t *types.Type) {
	message := lexer.Quote(fmt.Sprintf("serde: cycle through %s", t))
	s.guardPointer(func() {
		s.line("__panic(%s)", message)
	}, "__serialize_into(w, mut path, *v)")
}

func isOptional(t *types.Type) bool {
	return t.Kind == types.EnumKind && t.Optional
}

func required(t *types.Type, defaulted bool) bool {
	return !defaulted && !isOptional(t)
}

type keyCodec struct {
	write string
	read  string
}

func (s *synth) keyCodec(k *types.Type) keyCodec {
	text := keyCodec{write: `"{k}"`}
	switch {
	case k == types.Str:
		return keyCodec{write: "k", read: "key"}
	case k == types.Bool:
		text.read = "key_boolean(key, at)?"
	case k == types.I64:
		text.read = "key_integer(key, at)?"
	case k == types.U64:
		text.read = "key_unsigned(key, at)?"
	case k.Kind == types.IntKind:
		low, high := k.Bounds()
		text.read = fmt.Sprintf("%s(in_range(key_integer(key, at)?, %d, %d, at)?)", s.name(k), low, high)
	default:
		s.unsupported(k)
	}
	return text
}

func labelOf(v *types.Variant, i int) string {
	if i < len(v.Labels) && v.Labels[i] != "" {
		return v.Labels[i]
	}
	return fmt.Sprintf("%d", i)
}

func labelled(v *types.Variant) bool {
	return len(v.Labels) > 0 && v.Labels[0] != ""
}

func (s *synth) serializeEnum(t *types.Type) {
	s.line("match v {")
	for _, v := range t.Variants {
		if len(v.Fields) == 0 {
			s.line("%s => w.unit_variant(%s)", v.Name, lexer.Quote(v.Name))
			continue
		}
		binds := make([]string, len(v.Fields))
		for i := range v.Fields {
			binds[i] = fmt.Sprintf("f%d", i)
		}
		s.line("%s(%s) => {", v.Name, strings.Join(binds, ", "))
		s.line("w.begin_variant(%s)", lexer.Quote(v.Name))
		if labelled(v) {
			s.line("w.begin_struct()")
			for i := range v.Fields {
				s.line("w.field(%s)\n__serialize_into(w, mut path, f%d)", lexer.Quote(labelOf(v, i)), i)
			}
			s.line("w.end_struct()")
		} else {
			s.line("w.begin_seq()")
			for i := range v.Fields {
				s.line("__serialize_into(w, mut path, f%d)", i)
			}
			s.line("w.end_seq()")
		}
		s.line("w.end_variant()\n}")
	}
	s.line("}")
}

func (s *synth) deserialize(t *types.Type) {
	self := s.name(t)
	switch {
	case t == types.Bool:
		s.line("r.boolean()?")
	case t == types.I64:
		s.line("r.integer()?")
	case t == types.U64:
		s.line("r.unsigned()?")
	case t.Kind == types.IntKind:
		low, high := t.Bounds()
		s.line("%s(integer_in(r, %d, %d)?)", self, low, high)
	case t == types.F64:
		s.line("r.float()?")
	case t.Kind == types.FloatKind:
		s.line("f32(r.float()?)")
	case t == types.Str:
		s.line("r.text()?")
	case t.Kind == types.SliceKind && t.Elem == types.U8:
		s.line("r.binary()?")
	case t.Kind == types.SliceKind:
		s.line("var out: %s = []\nr.begin_seq()?\nfor r.next_element()? {\npush(mut out, __deserialize[%s](r)?)\n}\nout", self, s.name(t.Elem))
	case t.Kind == types.MapKind:
		s.line("var out: %s = [:]\nr.begin_map()?\nfor {\nlet at = r.position()", self)
		s.line("let key = match r.next_key()? {\nsome(k) => k\nnone => break\n}")
		s.line("out[%s] = __deserialize[%s](r)?\n}\nout", s.keyCodec(t.Key).read, s.name(t.Elem))
	case isOptional(t):
		s.line("if r.null()? {\nreturn none\n}\n__deserialize[%s](r)?", s.name(t.Elem))
	case t.Kind == types.PointerKind:
		s.line("new(__deserialize[%s](r)?)", s.name(t.Elem))
	case t.Kind == types.StructKind:
		s.deserializeStruct(t)
	case t.Kind == types.EnumKind:
		s.deserializeEnum(t)
	default:
		s.unsupported(t)
	}
}

type member struct {
	name      string
	t         *types.Type
	defaulted bool
}

type slot struct {
	member
	value    string
	present  string
	required bool
}

func (s *synth) structMembers(t *types.Type) []member {
	members := make([]member, len(t.Fields))
	for i, f := range t.Fields {
		members[i] = member{name: f.Name, t: f.Type, defaulted: s.c.fieldSources[f] != nil}
	}
	return members
}

func variantMembers(v *types.Variant) []member {
	members := make([]member, len(v.Fields))
	for i, f := range v.Fields {
		members[i] = member{name: labelOf(v, i), t: f}
	}
	return members
}

func (s *synth) readFields(members []member) []slot {
	slots := make([]slot, len(members))
	for i, m := range members {
		slots[i] = slot{member: m, value: s.temp("value"), present: s.temp("seen"), required: required(m.t, m.defaulted)}
		s.line("var %s: %s = __zero()\nvar %s = false", slots[i].value, s.name(m.t), slots[i].present)
	}
	s.line("r.begin_map()?\nfor {\nlet key = match r.next_field()? {\nsome(k) => k\nnone => break\n}")
	for i, sl := range slots {
		prefix := "} else if"
		if i == 0 {
			prefix = "if"
		}
		s.line("%s key == %s {\n%s = __deserialize[%s](r)?\n%s = true", prefix, lexer.Quote(sl.name), sl.value, s.name(sl.t), sl.present)
	}
	if len(slots) == 0 {
		s.line("r.skip()?\n}")
	} else {
		s.line("} else {\nr.skip()?\n}\n}")
	}
	for _, sl := range slots {
		if sl.required {
			s.line("if !%s {\nfail Missing(%s)\n}", sl.present, lexer.Quote(sl.name))
		}
	}
	return slots
}

func (s *synth) deserializeStruct(t *types.Type) {
	slots := s.readFields(s.structMembers(t))
	var inits []string
	for _, sl := range slots {
		if !sl.defaulted {
			inits = append(inits, fmt.Sprintf("%s: %s", sl.name, sl.value))
		}
	}
	s.line("var out = %s { %s }", s.name(t), strings.Join(inits, ", "))
	for _, sl := range slots {
		if sl.defaulted {
			s.line("if %s {\nout.%s = %s\n}", sl.present, sl.name, sl.value)
		}
	}
	s.line("out")
}

func (s *synth) deserializeEnum(t *types.Type) {
	s.line("let at = r.position()\nlet name = r.begin_variant()?")
	for _, v := range t.Variants {
		s.line("if name == %s {", lexer.Quote(v.Name))
		args := make([]string, len(v.Fields))
		switch {
		case len(v.Fields) == 0:
		case labelled(v):
			for i, sl := range s.readFields(variantMembers(v)) {
				args[i] = sl.value
			}
		default:
			expected := lexer.Quote(fmt.Sprintf("%d values for %s", len(v.Fields), v.Name))
			s.line("r.begin_seq()?")
			for i, f := range v.Fields {
				args[i] = s.temp("item")
				s.line("if !r.next_element()? {\nfail Mismatch(%s, at)\n}\nlet %s = __deserialize[%s](r)?", expected, args[i], s.name(f))
			}
			s.line("if r.next_element()? {\nfail Mismatch(%s, at)\n}", expected)
		}
		s.line("r.end_variant()?")
		if len(v.Fields) == 0 {
			s.line("return %s.%s\n}", s.name(t), v.Name)
		} else {
			s.line("return %s.%s(%s)\n}", s.name(t), v.Name, strings.Join(args, ", "))
		}
	}
	s.line("fail UnknownVariant(name)")
}

func (s *synth) schema(t *types.Type, stack []*types.Type) string {
	switch {
	case t == types.Bool:
		return "__S { kind: Bool }"
	case t == types.U64 || (t.Kind == types.IntKind && !t.Signed):
		return fmt.Sprintf("__S { kind: Uint, bits: %d }", t.Bits)
	case t.Kind == types.IntKind:
		return fmt.Sprintf("__S { kind: Int, bits: %d }", t.Bits)
	case t.Kind == types.FloatKind:
		return fmt.Sprintf("__S { kind: Float, bits: %d }", t.Bits)
	case t == types.Str:
		return "__S { kind: Text }"
	case t.Kind == types.SliceKind && t.Elem == types.U8:
		return "__S { kind: Binary }"
	case t.Kind == types.SliceKind:
		return fmt.Sprintf("__S { kind: Seq, items: [%s] }", s.schema(t.Elem, stack))
	case t.Kind == types.MapKind:
		return fmt.Sprintf("__S { kind: Map, items: [%s, %s] }", s.schema(t.Key, stack), s.schema(t.Elem, stack))
	case t.Kind == types.EnumKind && t.Optional:
		return fmt.Sprintf("__S { kind: Optional, items: [%s] }", s.schema(t.Elem, stack))
	case t.Kind == types.PointerKind:
		return s.schema(t.Elem, stack)
	case t.Kind == types.StructKind || t.Kind == types.EnumKind:
		for _, seen := range stack {
			if seen == t {
				return fmt.Sprintf("__S { kind: Ref, name: %s }", lexer.Quote(t.Name))
			}
		}
		inner := append(stack, t)
		if t.Kind == types.StructKind {
			return fmt.Sprintf("__S { kind: Struct, name: %s, fields: [%s] }", lexer.Quote(t.Name), s.schemaProperties(s.structMembers(t), inner))
		}
		return fmt.Sprintf("__S { kind: Enum, name: %s, variants: [%s] }", lexer.Quote(t.Name), s.schemaVariants(t, inner))
	}
	s.unsupported(t)
	return ""
}

func (s *synth) schemaProperties(members []member, stack []*types.Type) string {
	parts := make([]string, len(members))
	for i, m := range members {
		parts[i] = fmt.Sprintf("Property { name: %s, schema: %s, required: %t }", lexer.Quote(m.name), s.schema(m.t, stack), required(m.t, m.defaulted))
	}
	return strings.Join(parts, ", ")
}

func (s *synth) schemaVariants(t *types.Type, stack []*types.Type) string {
	parts := make([]string, len(t.Variants))
	for i, v := range t.Variants {
		parts[i] = fmt.Sprintf("Variant { name: %s, labelled: %t, fields: [%s] }", lexer.Quote(v.Name), labelled(v), s.schemaProperties(variantMembers(v), stack))
	}
	return strings.Join(parts, ", ")
}
