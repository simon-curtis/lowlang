package check

import (
	"strings"

	"lowlang/internal/ast"
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
	e.Builtin = "vector_cast"
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
	name := e.Builtin
	switch name {
	case "__vload":
		c.vectorSource(e.Args[0])
		c.integer(e.Args[1])
		e.Type = c.vectorResult(e, want)
	case "__vstore":
		c.vectorSource(e.Args[0])
		c.integer(e.Args[1])
		c.vectorArgument(e.Args[2], nil)
		e.Type = types.Void
	case "__vsplat":
		v := c.vectorResult(e, want)
		var lane *types.Type
		if isVector(v) {
			lane = v.Elem
		}
		c.plain(e.Args[0], lane)
		e.Type = v
	case "__vmask":
		c.vectorArgument(e.Args[0], nil)
		e.Type = types.U64
	case "__vshuffle", "__vmin", "__vmax":
		t := c.vectorArgument(e.Args[0], nil)
		c.vectorArgument(e.Args[1], t)
		if name == "__vshuffle" && isVector(t) && t.Elem != types.U8 {
			c.fail(e.Pos, "shuffle needs u8 lanes, found %s", t)
		}
		e.Type = t
	case "__vselect":
		t := c.vectorArgument(e.Args[0], nil)
		c.vectorArgument(e.Args[1], t)
		c.vectorArgument(e.Args[2], t)
		e.Type = t
	case "__vlane":
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
	case "__vwith":
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
	if s.Op == lexer.PercentAssign || (s.Op == lexer.SlashAssign && !t.Elem.Float()) {
		c.fail(s.Pos, "operator %s is not defined on %s", s.Op, t)
	}
	if s.Op == lexer.ShlAssign || s.Op == lexer.ShrAssign {
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
	e.Builtin = id.Name
}

func (c *Checker) vectorSource(e ast.Expr) {
	t := c.plain(e, nil)
	if t != types.Str && t.Kind != types.SliceKind {
		c.fail(e.Position(), "vectors load from and store to slices or str, found %s", t)
	}
}
