package check

import (
	"fmt"
	"strings"

	"lowlang/internal/ast"
	"lowlang/internal/lexer"
	"lowlang/internal/parser"
	"lowlang/internal/types"
)

const serdeModule = "std/serde"

type derivation int

const (
	serialize derivation = iota
	deserialize
	describe
)

var derivationNames = [...]string{serialize: "__write", deserialize: "__read", describe: "__describe"}

type synth struct {
	c     *Checker
	pos   lexer.Pos
	bound map[string]*types.Type
	names map[*types.Type]string
	out   strings.Builder
	temps int
}

func (c *Checker) serdeType(name string, pos lexer.Pos) *types.Type {
	if c.file.Module != serdeModule {
		c.fail(pos, "only %s may derive code from types", serdeModule)
	}
	t, ok := c.local().types[name]
	if !ok {
		c.fail(pos, "%s has no type %s", serdeModule, name)
	}
	return t
}

func (c *Checker) serializeCall(e *ast.Call) {
	if len(e.Args) != 2 {
		c.fail(e.Pos, "__serialize takes 2 arguments, found %d", len(e.Args))
	}
	t := c.plain(e.Args[0], nil)
	c.expectType(e.Args[1], c.serdeType("Writer", e.Pos))
	e.Type = types.Void
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
		e.Type = c.result(t, c.serdeType("Error", e.Pos))
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

func (c *Checker) derive(kind derivation, t *types.Type, pos lexer.Pos) *ast.Func {
	key := fmt.Sprintf("%d:%s", kind, typeKey([]*types.Type{t}))
	if fn, ok := c.derived[key]; ok {
		return fn
	}
	s := &synth{c: c, pos: pos, bound: map[string]*types.Type{}, names: map[*types.Type]string{}}
	s.function(kind, t)
	fn := s.parse(kind, t)
	c.derived[key] = fn
	c.within(c.file, fn.Bound, func() { c.signature(fn) })
	c.file.Funcs = append(c.file.Funcs, fn)
	c.pending = append(c.pending, fn)
	return fn
}

func (s *synth) parse(kind derivation, t *types.Type) *ast.Func {
	name := types.InstanceName(derivationNames[kind], []*types.Type{t})
	source := s.out.String()
	tokens, err := lexer.LexAt([]byte(source), lexer.Pos{File: fmt.Sprintf("<%s>", name), Line: 1, Col: 1}, true)
	if err != nil {
		panic(fmt.Sprintf("derived source for %s does not lex: %v\n%s", t, err, source))
	}
	file, err := parser.ParsePrelude(tokens)
	if err != nil {
		panic(fmt.Sprintf("derived source for %s does not parse: %v\n%s", t, err, source))
	}
	fn := file.Funcs[0]
	fn.Name = name
	fn.File = s.c.file
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
	fmt.Fprintf(&s.out, format, args...)
	s.out.WriteByte('\n')
}

func quoted(text string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range text {
		switch r {
		case '"', '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case '{':
			b.WriteString("{{")
		case '}':
			b.WriteString("}}")
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

func (s *synth) function(kind derivation, t *types.Type) {
	self := s.name(t)
	switch kind {
	case serialize:
		s.bound["__W"] = s.c.serdeType("Writer", s.pos)
		s.line("fn derived(v: %s, w: __W) {", self)
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
		s.line("w.begin_seq(v.len)\nfor x in v {\n__serialize(x, w)\n}\nw.end_seq()")
	case t.Kind == types.MapKind:
		s.line("w.begin_map(v.len)\nfor k, x in v {\nw.key(%s)\n__serialize(x, w)\n}\nw.end_map()", s.keyText(t.Key))
	case t.Kind == types.EnumKind && t.Optional:
		s.line("match v {\nsome(x) => __serialize(x, w)\nnone => w.null()\n}")
	case t.Kind == types.PointerKind:
		s.line("__serialize(*v, w)")
	case t.Kind == types.StructKind:
		s.line("w.begin_struct(%s, %d)", quoted(t.Name), len(t.Fields))
		for _, f := range t.Fields {
			s.line("w.field(%s)\n__serialize(v.%s, w)", quoted(f.Name), f.Name)
		}
		s.line("w.end_struct()")
	case t.Kind == types.EnumKind:
		s.serializeEnum(t)
	default:
		s.unsupported(t)
	}
}

func (s *synth) keyText(k *types.Type) string {
	switch {
	case k == types.Str:
		return "k"
	case k.Kind == types.IntKind || k == types.Bool:
		return `"{k}"`
	}
	s.unsupported(k)
	return ""
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
			s.line("%s => w.unit_variant(%s, %s)", v.Name, quoted(t.Name), quoted(v.Name))
			continue
		}
		binds := make([]string, len(v.Fields))
		for i := range v.Fields {
			binds[i] = fmt.Sprintf("f%d", i)
		}
		s.line("%s(%s) => {", v.Name, strings.Join(binds, ", "))
		s.line("w.begin_variant(%s, %s, %d)", quoted(t.Name), quoted(v.Name), len(v.Fields))
		if labelled(v) {
			s.line("w.begin_struct(%s, %d)", quoted(v.Name), len(v.Fields))
			for i := range v.Fields {
				s.line("w.field(%s)\n__serialize(f%d, w)", quoted(labelOf(v, i)), i)
			}
			s.line("w.end_struct()")
		} else {
			s.line("w.begin_seq(%d)", len(v.Fields))
			for i := range v.Fields {
				s.line("__serialize(f%d, w)", i)
			}
			s.line("w.end_seq()")
		}
		s.line("w.end_variant()\n}")
	}
	s.line("}")
}

func bounds(t *types.Type) (int64, int64) {
	if t.Signed {
		limit := int64(1) << (t.Bits - 1)
		return -limit, limit - 1
	}
	return 0, int64(1)<<t.Bits - 1
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
		low, high := bounds(t)
		s.line("let at = r.position()\n%s(checked(r.integer()?, %d, %d, at)?)", self, low, high)
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
		s.line("out[%s] = __deserialize[%s](r)?\n}\nout", s.keyValue(t.Key), s.name(t.Elem))
	case t.Kind == types.EnumKind && t.Optional:
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

func (s *synth) keyValue(k *types.Type) string {
	switch {
	case k == types.Str:
		return "key"
	case k == types.Bool:
		return "key_boolean(key, at)?"
	case k == types.I64:
		return "key_integer(key, at)?"
	case k.Kind == types.IntKind && k != types.U64:
		low, high := bounds(k)
		return fmt.Sprintf("%s(checked(key_integer(key, at)?, %d, %d, at)?)", s.name(k), low, high)
	}
	s.unsupported(k)
	return ""
}

type slot struct {
	label    string
	value    string
	present  string
	t        *types.Type
	optional bool
}

func (s *synth) readFields(names []string, fieldTypes []*types.Type) []slot {
	slots := make([]slot, len(names))
	for i, n := range names {
		slots[i] = slot{
			label:    n,
			value:    s.temp("value"),
			present:  s.temp("seen"),
			t:        fieldTypes[i],
			optional: fieldTypes[i].Kind == types.EnumKind && fieldTypes[i].Optional,
		}
		s.line("var %s: %s = __zero()\nvar %s = false", slots[i].value, s.name(fieldTypes[i]), slots[i].present)
	}
	s.line("r.begin_map()?\nfor {\nlet key = match r.next_key()? {\nsome(k) => k\nnone => break\n}")
	for i, sl := range slots {
		prefix := "} else if"
		if i == 0 {
			prefix = "if"
		}
		s.line("%s key == %s {\n%s = __deserialize[%s](r)?\n%s = true", prefix, quoted(sl.label), sl.value, s.name(sl.t), sl.present)
	}
	if len(slots) == 0 {
		s.line("r.skip()?\n}")
	} else {
		s.line("} else {\nr.skip()?\n}\n}")
	}
	return slots
}

func (s *synth) deserializeStruct(t *types.Type) {
	names := make([]string, len(t.Fields))
	fieldTypes := make([]*types.Type, len(t.Fields))
	for i, f := range t.Fields {
		names[i], fieldTypes[i] = f.Name, f.Type
	}
	slots := s.readFields(names, fieldTypes)
	var inits []string
	for i, f := range t.Fields {
		sl := slots[i]
		if s.c.fieldSources[f] != nil {
			continue
		}
		if !sl.optional {
			s.line("if !%s {\nfail Missing(%s)\n}", sl.present, quoted(f.Name))
		}
		inits = append(inits, fmt.Sprintf("%s: %s", f.Name, sl.value))
	}
	s.line("var out = %s { %s }", s.name(t), strings.Join(inits, ", "))
	for i, f := range t.Fields {
		if s.c.fieldSources[f] != nil {
			s.line("if %s {\nout.%s = %s\n}", slots[i].present, f.Name, slots[i].value)
		}
	}
	s.line("out")
}

func (s *synth) deserializeEnum(t *types.Type) {
	s.line("let at = r.position()\nlet name = r.begin_variant()?")
	for _, v := range t.Variants {
		s.line("if name == %s {", quoted(v.Name))
		args := make([]string, len(v.Fields))
		switch {
		case len(v.Fields) == 0:
		case labelled(v):
			names := make([]string, len(v.Fields))
			for i := range v.Fields {
				names[i] = labelOf(v, i)
			}
			slots := s.readFields(names, v.Fields)
			for i, sl := range slots {
				if !sl.optional {
					s.line("if !%s {\nfail Missing(%s)\n}", sl.present, quoted(sl.label))
				}
				args[i] = sl.value
			}
		default:
			expected := quoted(fmt.Sprintf("%d values for %s", len(v.Fields), v.Name))
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
				return fmt.Sprintf("__S { kind: Ref, name: %s }", quoted(t.Name))
			}
		}
		inner := append(stack, t)
		if t.Kind == types.StructKind {
			return fmt.Sprintf("__S { kind: Struct, name: %s, fields: [%s] }", quoted(t.Name), s.schemaFields(t, inner))
		}
		return fmt.Sprintf("__S { kind: Enum, name: %s, variants: [%s] }", quoted(t.Name), s.schemaVariants(t, inner))
	}
	s.unsupported(t)
	return ""
}

func required(c *Checker, f *types.Field) bool {
	return c.fieldSources[f] == nil && !(f.Type.Kind == types.EnumKind && f.Type.Optional)
}

func (s *synth) schemaFields(t *types.Type, stack []*types.Type) string {
	parts := make([]string, len(t.Fields))
	for i, f := range t.Fields {
		parts[i] = fmt.Sprintf("Field { name: %s, schema: %s, required: %t }", quoted(f.Name), s.schema(f.Type, stack), required(s.c, f))
	}
	return strings.Join(parts, ", ")
}

func (s *synth) schemaVariants(t *types.Type, stack []*types.Type) string {
	parts := make([]string, len(t.Variants))
	for i, v := range t.Variants {
		fields := make([]string, len(v.Fields))
		for j, f := range v.Fields {
			optional := f.Kind == types.EnumKind && f.Optional
			fields[j] = fmt.Sprintf("Field { name: %s, schema: %s, required: %t }", quoted(labelOf(v, j)), s.schema(f, stack), !optional)
		}
		parts[i] = fmt.Sprintf("Variant { name: %s, labelled: %t, fields: [%s] }", quoted(v.Name), labelled(v), strings.Join(fields, ", "))
	}
	return strings.Join(parts, ", ")
}
