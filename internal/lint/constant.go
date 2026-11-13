package lint

import (
	"lowlang/internal/ast"
	"lowlang/internal/builtin"
	"lowlang/internal/lexer"
	"lowlang/internal/types"
)

const preferConst = "prefer-const"

func (l *linter) constantFunc(fn *ast.Func) {
	if !returnsConstant(fn) {
		return
	}
	l.report(fn.Pos, preferConst, "%s returns the same value on every call; use const %s: %s = … and drop the () where it is called",
		fn.Name, fn.Name, fn.Result)
}

func returnsConstant(fn *ast.Func) bool {
	switch {
	case !fn.Declared(), fn.Extern != nil, fn.AsValue, fn.Callback:
		return false
	case len(fn.TypeParams) > 0, len(fn.Params) > 0, fn.Result == nil, fn.Error != nil:
		return false
	case !scalar(fn.RetType), len(fn.Body.Stmts) > 0:
		return false
	}
	return constant(fn.Body.Tail)
}

func scalar(t *types.Type) bool {
	return t.Integer() || t.Float() || t == types.Bool || t == types.Str
}

func constant(e ast.Expr) bool {
	switch e := e.(type) {
	case *ast.IntLit, *ast.FloatLit, *ast.StrLit, *ast.BoolLit:
		return true
	case *ast.Ident:
		return e.Const != nil
	case *ast.Selector:
		return e.Const != nil
	case *ast.Unary:
		return foldedUnary(e.Op) && constant(e.X)
	case *ast.Binary:
		return foldedBinary(e.Op) && constant(e.X) && constant(e.Y)
	case *ast.Call:
		return e.Builtin == builtin.Convert && e.Args[0].TypeOf().Number() && constant(e.Args[0])
	}
	return false
}

func foldedUnary(op lexer.Kind) bool {
	switch op {
	case lexer.Minus, lexer.Tilde, lexer.Not:
		return true
	}
	return false
}

func foldedBinary(op lexer.Kind) bool {
	switch op {
	case lexer.Plus, lexer.Minus, lexer.Star, lexer.Slash, lexer.Percent,
		lexer.Amp, lexer.Pipe, lexer.Caret, lexer.Shl, lexer.Shr,
		lexer.Eq, lexer.NotEq, lexer.Lt, lexer.LtEq, lexer.Gt, lexer.GtEq,
		lexer.AndAnd, lexer.OrOr:
		return true
	}
	return false
}
