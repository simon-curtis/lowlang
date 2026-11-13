package check

import (
	"math"

	"lowlang/internal/ast"
	"lowlang/internal/builtin"
	"lowlang/internal/diag"
	"lowlang/internal/lexer"
	"lowlang/internal/types"
)

const twoPow63 = 9223372036854775808.0

func (c *Checker) declareConsts(files []*ast.File) {
	for _, f := range files {
		c.file = f
		for _, k := range f.Consts {
			c.declareConst(k)
		}
	}
	for _, f := range files {
		c.file = f
		for _, k := range f.Consts {
			c.folded(k)
		}
	}
}

func (c *Checker) declareConst(k *ast.Const) {
	scope := c.local()
	_, generic := scope.generics[k.Name]
	_, dup := scope.consts[k.Name]
	_, fn := scope.funcs[k.Name]
	_, typ := scope.types[k.Name]
	switch {
	case dup:
		c.fail(k.Pos, "constant %s already declared", k.Name)
	case fixedBuiltin(k.Name) != nil:
		c.fail(k.Pos, "%s is a builtin", k.Name)
	case fn || generic:
		c.fail(k.Pos, "constant %s has the name of a function", k.Name)
	case typ || reservedType(k.Name):
		c.fail(k.Pos, "constant %s has the name of a type", k.Name)
	}
	scope.consts[k.Name] = k
}

func (c *Checker) importedConst(alias string, k *ast.Const, pos diag.Pos) *ast.Const {
	if !k.Pub {
		c.fail(pos, "%s.%s is private", alias, k.Name)
	}
	return k
}

func (c *Checker) constArm(p *ast.Pattern, enum *types.Type) bool {
	_, named := c.local().consts[p.Name]
	if !named || c.find(p.Name) != nil || enum.Variant(p.Name) != nil {
		return false
	}
	if len(p.Bindings) > 0 {
		c.fail(p.Pos, "constant %s cannot bind fields", p.Name)
	}
	value := &ast.Ident{Name: p.Name, Pos: p.Pos}
	p.Test = &ast.Binary{Op: lexer.Eq, X: &ast.Subject{Pos: p.Pos}, Y: value, Pos: p.Pos}
	return true
}

func (c *Checker) folded(k *ast.Const) ast.Expr {
	if k.Folded != nil {
		return k.Folded
	}
	if c.folding[k] {
		c.fail(k.Pos, "constant %s refers to itself", k.Name)
	}
	c.folding[k] = true
	next := funcContext{file: k.File, fn: &ast.Func{Name: k.Name, File: k.File, RetType: types.Void}}
	c.enter(next, func() {
		c.pushScope()
		t := c.constType(k)
		if !t.Integer() && !t.Float() && t != types.Bool && t != types.Str {
			c.fail(k.Value.Position(), "constant %s must be a number, bool or str, found %s", k.Name, t)
		}
		k.Folded = c.fold(k.Value)
	})
	return k.Folded
}

func (c *Checker) constType(k *ast.Const) *types.Type {
	if k.TypeName == nil {
		return c.plain(k.Value, nil)
	}
	t := c.resolve(k.TypeName)
	c.expectType(k.Value, t)
	return t
}

func (c *Checker) fold(e ast.Expr) ast.Expr {
	switch e := e.(type) {
	case *ast.IntLit:
		if e.Type.Float() {
			return floatLit(e.Type, float64(e.Value), e.Pos)
		}
		return e
	case *ast.FloatLit, *ast.StrLit, *ast.BoolLit:
		return e
	case *ast.Ident:
		if e.Const != nil {
			return e.Const.Folded
		}
	case *ast.Selector:
		if e.Const != nil {
			return e.Const.Folded
		}
	case *ast.Unary:
		return c.foldUnary(e)
	case *ast.Binary:
		return c.foldBinary(e)
	case *ast.Call:
		if e.Builtin == builtin.Convert {
			return c.foldConvert(e)
		}
	}
	c.fail(e.Position(), "a constant is built from literals, constants, operators and numeric conversions")
	return nil
}

func intLit(t *types.Type, n int64, pos diag.Pos) *ast.IntLit {
	return &ast.IntLit{Typed: ast.Typed{Type: t}, Value: t.Wrap(n), Pos: pos}
}

func floatLit(t *types.Type, f float64, pos diag.Pos) *ast.FloatLit {
	if t.Bits == 32 {
		f = float64(float32(f))
	}
	return &ast.FloatLit{Typed: ast.Typed{Type: t}, Value: f, Pos: pos}
}

func boolLit(b bool, pos diag.Pos) *ast.BoolLit {
	return &ast.BoolLit{Typed: ast.Typed{Type: types.Bool}, Value: b, Pos: pos}
}

func (c *Checker) foldUnary(e *ast.Unary) ast.Expr {
	x := c.fold(e.X)
	switch x := x.(type) {
	case *ast.IntLit:
		if e.Op == lexer.Tilde {
			return intLit(e.Type, ^x.Value, e.Pos)
		}
		return intLit(e.Type, -x.Value, e.Pos)
	case *ast.FloatLit:
		return floatLit(e.Type, -x.Value, e.Pos)
	case *ast.BoolLit:
		return boolLit(!x.Value, e.Pos)
	}
	c.fail(e.Pos, "operator %s is not constant", e.Op)
	return nil
}

func (c *Checker) foldBinary(e *ast.Binary) ast.Expr {
	x, y := c.fold(e.X), c.fold(e.Y)
	switch x := x.(type) {
	case *ast.IntLit:
		return c.foldInt(e, x, y.(*ast.IntLit))
	case *ast.FloatLit:
		return c.foldFloat(e, x.Value, y.(*ast.FloatLit).Value)
	case *ast.StrLit:
		return c.foldStr(e, x.Value, y.(*ast.StrLit).Value)
	case *ast.BoolLit:
		return c.foldBool(e, x.Value, y.(*ast.BoolLit).Value)
	}
	c.fail(e.Pos, "operator %s is not constant", e.Op)
	return nil
}

func (c *Checker) foldInt(e *ast.Binary, xl, yl *ast.IntLit) ast.Expr {
	t := xl.Type
	x, y := xl.Value, yl.Value
	switch e.Op {
	case lexer.Plus:
		return intLit(t, x+y, e.Pos)
	case lexer.Minus:
		return intLit(t, x-y, e.Pos)
	case lexer.Star:
		return intLit(t, x*y, e.Pos)
	case lexer.Amp:
		return intLit(t, x&y, e.Pos)
	case lexer.Pipe:
		return intLit(t, x|y, e.Pos)
	case lexer.Caret:
		return intLit(t, x^y, e.Pos)
	case lexer.Shl, lexer.Shr:
		return intLit(t, shift(e.Op, t, x, y), e.Pos)
	case lexer.Slash, lexer.Percent:
		if y == 0 {
			c.fail(e.Pos, "division by zero")
		}
		return intLit(t, divide(e.Op, t, x, y), e.Pos)
	}
	if t.Signed {
		return boolLit(ordered(e.Op, x < y, x == y), e.Pos)
	}
	return boolLit(ordered(e.Op, uint64(x) < uint64(y), x == y), e.Pos)
}

func shift(op lexer.Kind, t *types.Type, x, count int64) int64 {
	wide := uint64(count) >= 64
	switch {
	case op == lexer.Shl && wide:
		return 0
	case op == lexer.Shl:
		return x << uint64(count)
	case wide && t.Signed:
		return x >> 63
	case wide:
		return 0
	case t.Signed:
		return x >> uint64(count)
	}
	return int64(uint64(x) >> uint64(count))
}

func divide(op lexer.Kind, t *types.Type, x, y int64) int64 {
	switch {
	case !t.Signed && op == lexer.Slash:
		return int64(uint64(x) / uint64(y))
	case !t.Signed:
		return int64(uint64(x) % uint64(y))
	case y == -1 && op == lexer.Slash:
		return -x
	case y == -1:
		return 0
	case op == lexer.Slash:
		return x / y
	}
	return x % y
}

func ordered(op lexer.Kind, less, equal bool) bool {
	switch op {
	case lexer.Eq:
		return equal
	case lexer.NotEq:
		return !equal
	case lexer.Lt:
		return less
	case lexer.LtEq:
		return less || equal
	case lexer.Gt:
		return !less && !equal
	}
	return !less
}

func (c *Checker) foldFloat(e *ast.Binary, x, y float64) ast.Expr {
	t := e.X.TypeOf()
	switch e.Op {
	case lexer.Plus:
		return c.floatResult(e, t, x+y, float32(x)+float32(y))
	case lexer.Minus:
		return c.floatResult(e, t, x-y, float32(x)-float32(y))
	case lexer.Star:
		return c.floatResult(e, t, x*y, float32(x)*float32(y))
	case lexer.Slash:
		return c.floatResult(e, t, x/y, float32(x)/float32(y))
	case lexer.NotEq:
		return boolLit(x != y, e.Pos)
	case lexer.Eq:
		return boolLit(x == y, e.Pos)
	case lexer.Lt:
		return boolLit(x < y, e.Pos)
	case lexer.LtEq:
		return boolLit(x <= y, e.Pos)
	case lexer.Gt:
		return boolLit(x > y, e.Pos)
	}
	return boolLit(x >= y, e.Pos)
}

func (c *Checker) floatResult(e *ast.Binary, t *types.Type, wide float64, narrow float32) ast.Expr {
	if t.Bits == 32 {
		return floatLit(t, float64(narrow), e.Pos)
	}
	return floatLit(t, wide, e.Pos)
}

func (c *Checker) foldStr(e *ast.Binary, x, y string) ast.Expr {
	if e.Op == lexer.Plus {
		return &ast.StrLit{Typed: ast.Typed{Type: types.Str}, Value: x + y, Pos: e.Pos}
	}
	return boolLit(ordered(e.Op, x < y, x == y), e.Pos)
}

func (c *Checker) foldBool(e *ast.Binary, x, y bool) ast.Expr {
	switch e.Op {
	case lexer.AndAnd:
		return boolLit(x && y, e.Pos)
	case lexer.OrOr:
		return boolLit(x || y, e.Pos)
	case lexer.Eq:
		return boolLit(x == y, e.Pos)
	}
	return boolLit(x != y, e.Pos)
}

func (c *Checker) foldConvert(e *ast.Call) ast.Expr {
	to := e.Type
	switch x := c.fold(e.Args[0]).(type) {
	case *ast.IntLit:
		if to.Integer() {
			return intLit(to, x.Value, e.Pos)
		}
		return floatLit(to, integerFloat(x, to), e.Pos)
	case *ast.FloatLit:
		if to.Float() {
			return floatLit(to, x.Value, e.Pos)
		}
		return intLit(to, c.floatInteger(x.Value, to, e.Pos), e.Pos)
	}
	c.fail(e.Pos, "cannot convert a constant to %s", to)
	return nil
}

func integerFloat(x *ast.IntLit, to *types.Type) float64 {
	switch {
	case x.Type.Signed && to.Bits == 32:
		return float64(float32(x.Value))
	case x.Type.Signed:
		return float64(x.Value)
	case to.Bits == 32:
		return float64(float32(uint64(x.Value)))
	}
	return float64(uint64(x.Value))
}

func (c *Checker) floatInteger(f float64, to *types.Type, pos diag.Pos) int64 {
	truncated := math.Trunc(f)
	inRange := truncated >= -twoPow63 && truncated < twoPow63
	switch {
	case to == types.U64 && truncated >= 0 && truncated < 2*twoPow63:
		return int64(uint64(truncated))
	case to == types.U64:
		c.fail(pos, "%v overflows %s", f, to)
	case !inRange:
		return math.MinInt64
	}
	return int64(truncated)
}
