package check

import (
	"fmt"
	"strings"

	"lowlang/internal/ast"
	"lowlang/internal/builtin"
	"lowlang/internal/diag"
	"lowlang/internal/lexer"
	"lowlang/internal/prelude"
	"lowlang/internal/types"
)

func (c *Checker) derivationHome(kind derivation) *ast.File {
	if kind == render || kind == duplicate {
		return c.scope("").home
	}
	return c.file
}

func (c *Checker) renderer(t *types.Type, pos diag.Pos) *ast.Func {
	if t.Abstract() {
		return nil
	}
	return c.derive(render, t, pos)
}

func primitiveAppender(t *types.Type) (string, bool) {
	switch {
	case t == types.Bool:
		return prelude.AppendBool, true
	case t == types.U64:
		return prelude.AppendU64, true
	case t == types.F64:
		return prelude.AppendF64, true
	case t == types.F32:
		return prelude.AppendF32, true
	case t.Integer():
		return prelude.AppendI64, true
	}
	return "", false
}

func (c *Checker) appender(t *types.Type, pos diag.Pos) *ast.Func {
	if name, ok := primitiveAppender(t); ok && !t.Abstract() {
		return c.scope("").funcs[name]
	}
	return c.renderer(t, pos)
}

func (c *Checker) requireValue(pos diag.Pos, what string, t *types.Type) {
	if t == types.Void || t == types.Never {
		c.fail(pos, "%s needs a value, found %s", what, t)
	}
}

func (c *Checker) renderCall(e *ast.Call) {
	t := c.plain(e.Args[0], nil)
	c.requireValue(e.Args[0].Position(), "__render", t)
	e.Type = types.Str
	e.Render = c.renderer(t, e.Pos)
}

func (c *Checker) renderIntoCall(e *ast.Call) {
	c.address(c.mutOf(e, 0, c.universe.Slice(types.U8)))
	c.address(c.mutOf(e, 1, c.universe.Slice(types.I64)))
	t := c.plain(e.Args[2], nil)
	c.exclusive(e.Args)
	e.Builtin, e.Type = builtin.None, types.Void
	e.Target = c.renderer(t, e.Pos)
}

func (c *Checker) addressCall(e *ast.Call) {
	t := c.plain(e.Args[0], nil)
	if t.Kind != types.PointerKind && t.Kind != types.RawKind {
		c.fail(e.Args[0].Position(), "__address needs a pointer, found %s", t)
	}
	e.Type = types.I64
}

func (s *synth) put(text string) {
	s.text.WriteString(text)
}

func (s *synth) flush() {
	if s.text.Len() == 0 {
		return
	}
	text := s.text.String()
	s.text.Reset()
	fmt.Fprintf(&s.out, "%s(mut b, %s)\n", prelude.AppendStr, lexer.Quote(text))
}

func recurse(expr string) string {
	return fmt.Sprintf("__render_into(mut b, mut path, %s)", expr)
}

func bareName(t *types.Type) string {
	name := t.Name
	if open := strings.IndexByte(name, '['); open >= 0 {
		name = name[:open]
	}
	if dot := strings.LastIndexByte(name, '.'); dot >= 0 {
		name = name[dot+1:]
	}
	return name
}

func (s *synth) render(t *types.Type) {
	if name, ok := primitiveAppender(t); ok {
		operand := "v"
		if t.Integer() && t != types.U64 {
			operand = "i64(v)"
		}
		s.line("%s(mut b, %s)", name, operand)
		return
	}
	switch {
	case t == types.Str:
		s.line("%s(mut b, v)", prelude.AppendQuoted)
	case t.Kind == types.SliceKind && t.Elem == types.U8:
		s.line("%s(mut b, v)", prelude.AppendHex)
	case t.Kind == types.SliceKind:
		s.renderSlice()
	case t.Kind == types.MapKind:
		s.renderMap()
	case t.Kind == types.EnumKind && t.Optional:
		s.renderOptional()
	case t.Kind == types.PointerKind:
		s.renderPointer()
	case t.Kind == types.StructKind:
		s.renderStruct(t)
	case t.Kind == types.EnumKind:
		s.renderEnum(t)
	case t.Kind == types.VectorKind:
		s.renderVector(t)
	case t.Kind == types.RawKind:
		s.renderRaw(t)
	default:
		s.put(t.String())
	}
}

func (s *synth) renderSlice() {
	s.put("[")
	s.line("for i, x in v {\nif i > 0 {")
	s.put(", ")
	s.line("}\n%s\n}", recurse("x"))
	s.put("]")
}

func (s *synth) renderMap() {
	s.put("{")
	s.line("var n = 0\nfor k, x in v {\nif n > 0 {")
	s.put(", ")
	s.line("}\nn += 1\n%s", recurse("k"))
	s.put(": ")
	s.line("%s\n}", recurse("x"))
	s.put("}")
}

func (s *synth) renderOptional() {
	s.line("match v {\nsome(x) => {\n%s\n}\nnone => {", recurse("x"))
	s.put("none")
	s.line("}\n}")
}

func (s *synth) renderPointer() {
	s.guardPointer(func() {
		s.put("<cycle>")
		s.line("return")
	}, recurse("*v"))
}

func (s *synth) guardPointer(onCycle func(), target string) {
	s.line("let address = __address(v)\nfor seen in path {\nif seen == address {")
	onCycle()
	s.line("}\n}\npush(mut path, address)\n%s\ntruncate(mut path, path.len - 1)", target)
}

func (s *synth) renderStruct(t *types.Type) {
	s.put(fmt.Sprintf("%s{", bareName(t)))
	for i, f := range t.Fields {
		if i > 0 {
			s.put(", ")
		}
		s.put(fmt.Sprintf("%s: ", f.Name))
		s.line("%s", recurse(fmt.Sprintf("v.%s", f.Name)))
	}
	s.put("}")
}

func (s *synth) renderEnum(t *types.Type) {
	s.line("match v {")
	for _, v := range t.Variants {
		s.renderVariant(v)
	}
	s.line("}")
}

func (s *synth) renderVariant(v *types.Variant) {
	if len(v.Fields) == 0 {
		s.line("%s => {", v.Name)
		s.put(v.Name)
		s.line("}")
		return
	}
	binds := make([]string, len(v.Fields))
	for i := range v.Fields {
		binds[i] = fmt.Sprintf("f%d", i)
	}
	opening, closing := "(", ")"
	if labelled(v) {
		opening, closing = "{", "}"
	}
	s.line("%s(%s) => {", v.Name, strings.Join(binds, ", "))
	s.put(fmt.Sprintf("%s%s", v.Name, opening))
	for i, bind := range binds {
		if i > 0 {
			s.put(", ")
		}
		if labelled(v) {
			s.put(fmt.Sprintf("%s: ", labelOf(v, i)))
		}
		s.line("%s", recurse(bind))
	}
	s.put(closing)
	s.line("}")
}

func (s *synth) renderVector(t *types.Type) {
	s.put("<")
	s.line("for i in 0..%d {\nif i > 0 {", t.Lanes)
	s.put(", ")
	s.line("}\n%s\n}", recurse("__vlane(v, i)"))
	s.put(">")
}

func (s *synth) renderRaw(t *types.Type) {
	s.put(fmt.Sprintf("%s(0x", t.String()))
	s.line("%s(mut b, __address(v), 64, 4, false, 0)", prelude.AppendRadix)
	s.put(")")
}
