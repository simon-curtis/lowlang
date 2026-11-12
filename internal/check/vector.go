package check

import (
	"strings"

	"lowlang/internal/ast"
	"lowlang/internal/diag"
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

func (c *Checker) vectorOperator(op lexer.Kind, t *types.Type, pos diag.Pos) {
	lane := t.Elem
	switch op {
	case lexer.Percent:
		c.fail(pos, "operator %% is not defined on %s", t)
	case lexer.Slash:
		if !lane.Float() {
			c.fail(pos, "operator / on vectors requires float lanes, found %s", t)
		}
	case lexer.Shl, lexer.Shr:
		if !lane.Integer() {
			c.fail(pos, "operator %s on vectors requires integer lanes, found %s", op, t)
		}
	}
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

func (c *Checker) vectorResult(b *Builtin, e *ast.Call, want *types.Type) *types.Type {
	if want == nil || (!isVector(want) && want.Kind != types.ParamKind) {
		c.fail(e.Pos, "%s needs an expected vector type", b.Name)
	}
	return want
}

func (c *Checker) vectorLoad(b *Builtin, e *ast.Call, want *types.Type) {
	c.vectorSource(e.Args[0])
	c.integer(e.Args[1])
	e.Type = c.vectorResult(b, e, want)
}

func (c *Checker) vectorStore(e *ast.Call) {
	c.vectorSource(e.Args[0])
	c.integer(e.Args[1])
	c.vectorArgument(e.Args[2], nil)
	e.Type = types.Void
}

func (c *Checker) vectorSplat(b *Builtin, e *ast.Call, want *types.Type) {
	v := c.vectorResult(b, e, want)
	c.plain(e.Args[0], laneOf(v))
	e.Type = v
}

func (c *Checker) vectorMask(e *ast.Call) {
	c.vectorArgument(e.Args[0], nil)
	e.Type = types.U64
}

func (c *Checker) vectorPair(e *ast.Call) {
	t := c.vectorArgument(e.Args[0], nil)
	c.vectorArgument(e.Args[1], t)
	e.Type = t
}

func (c *Checker) vectorShuffle(e *ast.Call) {
	c.vectorPair(e)
	if t := e.Type; isVector(t) && t.Elem != types.U8 {
		c.fail(e.Pos, "shuffle needs u8 lanes, found %s", t)
	}
}

func (c *Checker) vectorSelect(e *ast.Call) {
	t := c.vectorArgument(e.Args[0], nil)
	c.vectorArgument(e.Args[1], t)
	c.vectorArgument(e.Args[2], t)
	e.Type = t
}

func (c *Checker) vectorLane(e *ast.Call, want *types.Type) {
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
}

func (c *Checker) vectorWith(e *ast.Call) {
	t := c.vectorArgument(e.Args[0], nil)
	c.integer(e.Args[1])
	c.plain(e.Args[2], laneOf(t))
	e.Type = t
}

func laneOf(t *types.Type) *types.Type {
	if isVector(t) {
		return t.Elem
	}
	return nil
}

func (c *Checker) vectorSplatOperand(s *ast.Assign, t *types.Type) {
	want := t
	if literal(s.Value) {
		want = t.Elem
	}
	value := c.plain(s.Value, want)
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
