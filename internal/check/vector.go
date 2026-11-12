package check

import (
	"strings"

	"lowlang/internal/ast"
	"lowlang/internal/builtin"
	"lowlang/internal/lexer"
	"lowlang/internal/types"
)

func isVector(t *types.Type) bool {
	return t != nil && t.Kind == types.VectorKind
}

func arithmetic(want *types.Type) *types.Type {
	want = unwrap(want)
	if want != nil && (want.Number() || isVector(want)) {
		return want
	}
	return nil
}

func (c *Checker) vectorOperator(e *ast.Binary, t *types.Type) {
	lane := t.Elem
	switch e.Op {
	case lexer.Percent:
		c.fail(e.Pos, "operator %% is not defined on %s", t)
	case lexer.Slash:
		if !lane.Float() {
			c.fail(e.Pos, "operator / on vectors requires float lanes, found %s", t)
		}
	case lexer.Shl, lexer.Shr:
		if !lane.Integer() {
			c.fail(e.Pos, "operator %s on vectors requires integer lanes, found %s", e.Op, t)
		}
	}
	e.Type = t
}

func (c *Checker) vectorShift(e *ast.Binary, t *types.Type) {
	c.vectorOperator(e, t)
	c.integer(e.Y)
}

func (c *Checker) vectorUnary(e *ast.Unary, t *types.Type) {
	if e.Op != lexer.Minus && e.Op != lexer.Tilde {
		c.fail(e.Pos, "operator %s is not defined on %s", e.Op, t)
	}
	if e.Op == lexer.Tilde && t.Elem.Float() {
		c.fail(e.Pos, "operator ~ on vectors requires integer lanes, found %s", t)
	}
	e.Type = t
}

func (c *Checker) vectorCast(e *ast.Call, to *types.Type) {
	from := c.plain(e.Args[0], nil)
	if !isVector(from) || from.Bits != to.Bits {
		c.fail(e.Args[0].Position(), "cannot convert %s to %s; vector casts keep the width", from, to)
	}
	e.Type = to
}

func (c *Checker) vectorArgument(e ast.Expr, want *types.Type) *types.Type {
	t := c.plain(e, want)
	if !isVector(t) && t.Kind != types.ParamKind {
		c.fail(e.Position(), "expected a vector, found %s", t)
	}
	if want != nil && t != want && want.Kind != types.ParamKind {
		c.fail(e.Position(), "expected %s, found %s", want, t)
	}
	return t
}

func (c *Checker) vectorResult(e *ast.Call, want *types.Type) *types.Type {
	if want == nil || (!isVector(want) && want.Kind != types.ParamKind) {
		c.fail(e.Pos, "%s needs an expected vector type", e.Builtin)
	}
	return want
}

func (c *Checker) vectorIntrinsic(e *ast.Call, want *types.Type) {
	switch e.Builtin {
	case builtin.VLoad:
		c.vectorSource(e.Args[0])
		c.integer(e.Args[1])
		e.Type = c.vectorResult(e, want)
	case builtin.VStore:
		c.vectorSource(e.Args[0])
		c.integer(e.Args[1])
		c.vectorArgument(e.Args[2], nil)
		e.Type = types.Void
	case builtin.VSplat:
		v := c.vectorResult(e, want)
		var lane *types.Type
		if isVector(v) {
			lane = v.Elem
		}
		c.plain(e.Args[0], lane)
		e.Type = v
	case builtin.VMask:
		c.vectorArgument(e.Args[0], nil)
		e.Type = types.U64
	case builtin.VShuffle, builtin.VMin, builtin.VMax:
		t := c.vectorArgument(e.Args[0], nil)
		c.vectorArgument(e.Args[1], t)
		if e.Builtin == builtin.VShuffle && isVector(t) && t.Elem != types.U8 {
			c.fail(e.Pos, "shuffle needs u8 lanes, found %s", t)
		}
		e.Type = t
	case builtin.VSelect:
		t := c.vectorArgument(e.Args[0], nil)
		c.vectorArgument(e.Args[1], t)
		c.vectorArgument(e.Args[2], t)
		e.Type = t
	case builtin.VLane:
		t := c.vectorArgument(e.Args[0], nil)
		c.integer(e.Args[1])
		switch {
		case isVector(t):
			e.Type = t.Elem
		case want != nil:
			e.Type = want
		default:
			e.Type = t
		}
	case builtin.VWith:
		t := c.vectorArgument(e.Args[0], nil)
		c.integer(e.Args[1])
		var lane *types.Type
		if isVector(t) {
			lane = t.Elem
		}
		c.plain(e.Args[2], lane)
		e.Type = t
	}
}

func (c *Checker) vectorAssign(s *ast.Assign, t *types.Type) {
	op := s.Op.Operator()
	if op == lexer.Percent || (op == lexer.Slash && !t.Elem.Float()) {
		c.fail(s.Pos, "operator %s is not defined on %s", s.Op, t)
	}
	if op == lexer.Shl || op == lexer.Shr {
		if !t.Elem.Integer() {
			c.fail(s.Pos, "operator %s on vectors requires integer lanes, found %s", s.Op, t)
		}
		c.integer(s.Value)
		return
	}
	var value *types.Type
	if literal(s.Value) {
		value = c.plain(s.Value, t.Elem)
	} else {
		value = c.plain(s.Value, t)
	}
	switch {
	case value == t:
	case t.Elem.Accepts(value):
		s.Splat = true
	default:
		c.fail(s.Value.Position(), "cannot apply %s with %s to %s", s.Op, value, t)
	}
}

func (c *Checker) forwarded(e *ast.Call, fn *ast.Func) {
	if fn.File == nil || fn.File.Module != "std/simd" || fn.Body == nil || len(fn.Body.Stmts) != 0 {
		return
	}
	inner, ok := fn.Body.Tail.(*ast.Call)
	if !ok {
		return
	}
	id, ok := inner.Fn.(*ast.Ident)
	if !ok || !strings.HasPrefix(id.Name, "__") || len(inner.Args) != len(fn.Params) {
		return
	}
	for i, arg := range inner.Args {
		name, ok := arg.(*ast.Ident)
		if !ok || name.Name != fn.Params[i].Name {
			return
		}
	}
	if b := LookupBuiltin(id.Name); b != nil {
		e.Builtin = b.ID
	}
}

func (c *Checker) vectorSource(e ast.Expr) {
	t := c.plain(e, nil)
	if t != types.Str && t.Kind != types.SliceKind {
		c.fail(e.Position(), "vectors load from and store to slices or str, found %s", t)
	}
}
