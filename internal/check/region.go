package check

import (
	"fmt"
	"strings"

	"lowlang/internal/ast"
	"lowlang/internal/builtin"
	"lowlang/internal/lexer"
	"lowlang/internal/types"
)

func (c *Checker) region(e *ast.Region, want *types.Type) {
	e.Type = c.regionBody(e.Body, want)
	if e.Type.Abstract() || !e.Type.HasPointers() {
		return
	}
	if blocker := c.copyBlocker(e.Type, map[*types.Type]bool{}); blocker != "" {
		c.fail(e.Pos, "a region cannot return %s: it contains %s", e.Type, blocker)
	}
	e.Copy = c.derive(duplicate, e.Type, e.Pos)
}

func (c *Checker) regionBody(body *ast.Block, want *types.Type) *types.Type {
	outer := c.regionLoops
	c.regions++
	c.regionLoops = len(c.loops)
	defer func() {
		c.regions--
		c.regionLoops = outer
	}()
	return c.block(body, want)
}

func (c *Checker) copyBlocker(t *types.Type, seen map[*types.Type]bool) string {
	if seen[t] {
		return ""
	}
	seen[t] = true
	switch t.Kind {
	case types.FuncKind:
		return "a function"
	case types.ChanKind:
		return "a channel"
	case types.RawKind:
		return "a ptr"
	case types.ResultKind:
		return "an error result"
	case types.SliceKind, types.PointerKind:
		return c.copyBlocker(t.Elem, seen)
	case types.MapKind:
		return c.firstBlocker(seen, t.Key, t.Elem)
	case types.EnumKind:
		return c.variantBlocker(t, seen)
	case types.StructKind:
		return c.structBlocker(t, seen)
	}
	return ""
}

func (c *Checker) variantBlocker(t *types.Type, seen map[*types.Type]bool) string {
	if t.Optional {
		return c.copyBlocker(t.Elem, seen)
	}
	for _, v := range t.Variants {
		if blocker := c.firstBlocker(seen, v.Fields...); blocker != "" {
			return blocker
		}
	}
	return ""
}

func (c *Checker) structBlocker(t *types.Type, seen map[*types.Type]bool) string {
	if t.C {
		return fmt.Sprintf("%s, an extern struct", t)
	}
	if c.droppable(t) {
		return fmt.Sprintf("%s, which has drop", t)
	}
	for _, f := range t.Fields {
		if blocker := c.copyBlocker(f.Type, seen); blocker != "" {
			return blocker
		}
	}
	return ""
}

func (c *Checker) firstBlocker(seen map[*types.Type]bool, list ...*types.Type) string {
	for _, t := range list {
		if blocker := c.copyBlocker(t, seen); blocker != "" {
			return blocker
		}
	}
	return ""
}

func (c *Checker) copyIntoCall(e *ast.Call) {
	c.address(c.mutOf(e, 0, c.universe.Slice(types.I64)))
	t := c.plain(e.Args[1], nil)
	c.exclusive(e.Args)
	e.Builtin, e.Type = builtin.None, t
	if !t.Abstract() {
		e.Target = c.derive(duplicate, t, e.Pos)
	}
}

func copyInto(expr string, t *types.Type) string {
	if !t.HasPointers() {
		return expr
	}
	return fmt.Sprintf("__copy_into(mut path, %s)", expr)
}

func (s *synth) duplicate(t *types.Type) {
	self := s.name(t)
	switch {
	case t == types.Str:
		s.line("if v.len == 0 {\nreturn \"\"\n}\nvar bytes: []u8 = __make(v.len)\n__memmove(bytes, v)\n__str_view(bytes)")
	case t.Kind == types.SliceKind && !t.Elem.HasPointers():
		s.line("if v.len == 0 {\nreturn []\n}\nvar out: %s = __make(v.len)\n__memmove(out, v)\nout", self)
	case t.Kind == types.SliceKind:
		s.line("var out: %s = __make(v.len)\nfor i, x in v {\nout[i] = %s\n}\nout", self, copyInto("x", t.Elem))
	case t.Kind == types.MapKind:
		s.line("var out: %s = [:]\nfor k, x in v {\nout[%s] = %s\n}\nout", self, copyInto("k", t.Key), copyInto("x", t.Elem))
	case isOptional(t):
		s.line("match v {\nsome(x) => %s\nnone => none\n}", copyInto("x", t.Elem))
	case t.Kind == types.PointerKind:
		s.duplicatePointer(t)
	case t.Kind == types.StructKind:
		s.duplicateStruct(t, self)
	case t.Kind == types.EnumKind:
		s.duplicateEnum(t, self)
	default:
		s.unsupported(t)
	}
}

func (s *synth) duplicatePointer(t *types.Type) {
	message := lexer.Quote(fmt.Sprintf("region: cycle through %s", t))
	s.line("let address = __address(v)\nfor seen in path {\nif seen == address {\npanic(%s)\n}\n}", message)
	s.line("push(mut path, address)\nlet out = new(%s)\ntruncate(mut path, path.len - 1)\nout", copyInto("*v", t.Elem))
}

func (s *synth) duplicateStruct(t *types.Type, self string) {
	inits := make([]string, len(t.Fields))
	for i, f := range t.Fields {
		inits[i] = fmt.Sprintf("%s: %s", f.Name, copyInto(fmt.Sprintf("v.%s", f.Name), f.Type))
	}
	s.line("%s { %s }", self, strings.Join(inits, ", "))
}

func (s *synth) duplicateEnum(t *types.Type, self string) {
	s.line("match v {")
	for _, v := range t.Variants {
		s.duplicateVariant(v, self)
	}
	s.line("}")
}

func (s *synth) duplicateVariant(v *types.Variant, self string) {
	if len(v.Fields) == 0 {
		s.line("%s => %s.%s", v.Name, self, v.Name)
		return
	}
	binds := make([]string, len(v.Fields))
	args := make([]string, len(v.Fields))
	for i, f := range v.Fields {
		binds[i] = fmt.Sprintf("f%d", i)
		args[i] = copyInto(binds[i], f)
	}
	s.line("%s(%s) => %s.%s(%s)", v.Name, strings.Join(binds, ", "), self, v.Name, strings.Join(args, ", "))
}
