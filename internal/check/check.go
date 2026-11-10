package check

import (
	"fmt"

	"lowlang/internal/ast"
	"lowlang/internal/lexer"
)

type Error struct {
	Pos lexer.Pos
	Msg string
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Pos, e.Msg)
}

type bailout struct {
	err *Error
}

type loop struct {
	broken bool
}

type Checker struct {
	funcs  map[string]*ast.Func
	fn     *ast.Func
	scopes []map[string]*ast.Local
	loops  []*loop
}

func Check(file *ast.File) (err error) {
	c := &Checker{funcs: map[string]*ast.Func{}}
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		b, ok := r.(bailout)
		if !ok {
			panic(r)
		}
		err = b.err
	}()
	c.file(file)
	return nil
}

func (c *Checker) fail(pos lexer.Pos, format string, args ...any) {
	panic(bailout{&Error{Pos: pos, Msg: fmt.Sprintf(format, args...)}})
}

var namedTypes = map[string]ast.Type{"i64": ast.I64, "bool": ast.Bool, "str": ast.Str}

func (c *Checker) resolve(t *ast.TypeName) ast.Type {
	if t == nil {
		return ast.Void
	}
	typ, ok := namedTypes[t.Name]
	if !ok {
		c.fail(t.Pos, "unknown type %s", t.Name)
	}
	return typ
}

func (c *Checker) file(file *ast.File) {
	for _, fn := range file.Funcs {
		if _, dup := c.funcs[fn.Name]; dup {
			c.fail(fn.Pos, "function %s already declared", fn.Name)
		}
		if fn.Name == "print" {
			c.fail(fn.Pos, "print is a builtin")
		}
		fn.RetType = c.resolve(fn.Result)
		c.funcs[fn.Name] = fn
	}
	main, ok := c.funcs["main"]
	if !ok {
		c.fail(lexer.Pos{Line: 1, Col: 1}, "missing function main")
	}
	if len(main.Params) > 0 || main.RetType != ast.Void {
		c.fail(main.Pos, "main must take no parameters and return nothing")
	}
	for _, fn := range file.Funcs {
		c.function(fn)
	}
}

func (c *Checker) function(fn *ast.Func) {
	c.fn = fn
	c.scopes = []map[string]*ast.Local{{}}
	for _, p := range fn.Params {
		if _, dup := c.scopes[0][p.Name]; dup {
			c.fail(p.Pos, "parameter %s already declared", p.Name)
		}
		c.declare(&ast.Local{Name: p.Name, Type: c.resolve(p.TypeName), Param: true})
	}
	body := c.block(fn.Body)
	if fn.RetType != ast.Void && body != fn.RetType && body != ast.Never {
		c.fail(fn.Body.Pos, "function %s must return %s, body produces %s", fn.Name, fn.RetType, body)
	}
}

func (c *Checker) declare(l *ast.Local) {
	if l.Param {
		l.Index = len(c.scopes[0])
	} else {
		l.Index = len(c.fn.Locals)
		c.fn.Locals = append(c.fn.Locals, l)
	}
	c.scopes[len(c.scopes)-1][l.Name] = l
}

func (c *Checker) lookup(name string, pos lexer.Pos) *ast.Local {
	for i := len(c.scopes) - 1; i >= 0; i-- {
		if l, ok := c.scopes[i][name]; ok {
			return l
		}
	}
	c.fail(pos, "undefined: %s", name)
	return nil
}

func (c *Checker) block(b *ast.Block) ast.Type {
	c.scopes = append(c.scopes, map[string]*ast.Local{})
	defer func() { c.scopes = c.scopes[:len(c.scopes)-1] }()
	diverges := false
	for _, s := range b.Stmts {
		if c.stmt(s) {
			diverges = true
		}
	}
	b.Type = ast.Void
	if b.Tail != nil {
		b.Type = c.expr(b.Tail)
	}
	if diverges {
		b.Type = ast.Never
	}
	return b.Type
}

func (c *Checker) stmt(s ast.Stmt) bool {
	switch s := s.(type) {
	case *ast.Let:
		c.let(s)
	case *ast.Assign:
		c.assign(s)
	case *ast.ExprStmt:
		return c.expr(s.X) == ast.Never
	case *ast.For:
		return c.forStmt(s)
	case *ast.Break:
		c.innermostLoop(s.Pos, "break").broken = true
		return true
	case *ast.Continue:
		c.innermostLoop(s.Pos, "continue")
		return true
	case *ast.Return:
		c.ret(s)
		return true
	}
	return false
}

func (c *Checker) let(s *ast.Let) {
	t := c.expr(s.Value)
	if t == ast.Void || t == ast.Never {
		c.fail(s.Value.Position(), "cannot bind a value of type %s", t)
	}
	if s.TypeName != nil {
		if want := c.resolve(s.TypeName); want != t {
			c.fail(s.Value.Position(), "cannot use %s as %s", t, want)
		}
	}
	s.Local = &ast.Local{Name: s.Name, Type: t, Mutable: s.Mutable}
	c.declare(s.Local)
}

func (c *Checker) assign(s *ast.Assign) {
	l := c.lookup(s.Name, s.Pos)
	if !l.Mutable {
		c.fail(s.Pos, "cannot assign to %s, declared with let", s.Name)
	}
	t := c.expr(s.Value)
	if t != l.Type {
		c.fail(s.Value.Position(), "cannot assign %s to %s of type %s", t, s.Name, l.Type)
	}
	if s.Op != lexer.Assign && l.Type != ast.I64 {
		c.fail(s.Pos, "operator %s requires i64, found %s", s.Op, l.Type)
	}
	s.Local = l
}

func (c *Checker) forStmt(s *ast.For) bool {
	if s.Cond != nil {
		c.expectType(s.Cond, ast.Bool)
	}
	l := &loop{}
	c.loops = append(c.loops, l)
	c.block(s.Body)
	c.loops = c.loops[:len(c.loops)-1]
	return s.Cond == nil && !l.broken
}

func (c *Checker) innermostLoop(pos lexer.Pos, keyword string) *loop {
	if len(c.loops) == 0 {
		c.fail(pos, "%s outside of a loop", keyword)
	}
	return c.loops[len(c.loops)-1]
}

func (c *Checker) ret(s *ast.Return) {
	want := c.fn.RetType
	if s.Value == nil {
		if want != ast.Void {
			c.fail(s.Pos, "missing return value of type %s", want)
		}
		return
	}
	if want == ast.Void {
		c.fail(s.Value.Position(), "function %s does not return a value", c.fn.Name)
	}
	c.expectType(s.Value, want)
}

func (c *Checker) expectType(e ast.Expr, want ast.Type) {
	if got := c.expr(e); got != want && got != ast.Never {
		c.fail(e.Position(), "expected %s, found %s", want, got)
	}
}

func (c *Checker) expr(e ast.Expr) ast.Type {
	switch e := e.(type) {
	case *ast.IntLit:
		e.Type = ast.I64
	case *ast.StrLit:
		e.Type = ast.Str
	case *ast.BoolLit:
		e.Type = ast.Bool
	case *ast.Ident:
		e.Local = c.lookup(e.Name, e.Pos)
		e.Type = e.Local.Type
	case *ast.Unary:
		c.unary(e)
	case *ast.Binary:
		c.binary(e)
	case *ast.Call:
		c.call(e)
	case *ast.If:
		c.ifExpr(e)
	case *ast.Block:
		c.block(e)
	}
	return e.TypeOf()
}

func (c *Checker) unary(e *ast.Unary) {
	want := ast.I64
	if e.Op == lexer.Not {
		want = ast.Bool
	}
	c.expectType(e.X, want)
	e.Type = want
}

func (c *Checker) binary(e *ast.Binary) {
	switch e.Op {
	case lexer.AndAnd, lexer.OrOr:
		c.expectType(e.X, ast.Bool)
		c.expectType(e.Y, ast.Bool)
		e.Type = ast.Bool
	case lexer.Eq, lexer.NotEq:
		x := c.expr(e.X)
		if x != ast.I64 && x != ast.Bool && x != ast.Str {
			c.fail(e.X.Position(), "cannot compare values of type %s", x)
		}
		c.expectType(e.Y, x)
		e.Type = ast.Bool
	case lexer.Lt, lexer.LtEq, lexer.Gt, lexer.GtEq:
		c.expectType(e.X, ast.I64)
		c.expectType(e.Y, ast.I64)
		e.Type = ast.Bool
	default:
		c.expectType(e.X, ast.I64)
		c.expectType(e.Y, ast.I64)
		e.Type = ast.I64
	}
}

var printable = map[ast.Type]string{ast.I64: "print_i64", ast.Str: "print_str", ast.Bool: "print_bool"}

func (c *Checker) call(e *ast.Call) {
	if e.Name == "print" {
		if len(e.Args) != 1 {
			c.fail(e.Pos, "print takes 1 argument, found %d", len(e.Args))
		}
		t := c.expr(e.Args[0])
		builtin, ok := printable[t]
		if !ok {
			c.fail(e.Args[0].Position(), "cannot print a value of type %s", t)
		}
		e.Builtin = builtin
		e.Type = ast.Void
		return
	}
	fn, ok := c.funcs[e.Name]
	if !ok {
		c.fail(e.Pos, "undefined function: %s", e.Name)
	}
	if len(e.Args) != len(fn.Params) {
		c.fail(e.Pos, "%s takes %d arguments, found %d", fn.Name, len(fn.Params), len(e.Args))
	}
	for i, arg := range e.Args {
		c.expectType(arg, c.resolve(fn.Params[i].TypeName))
	}
	e.Target = fn
	e.Type = fn.RetType
}

func (c *Checker) ifExpr(e *ast.If) {
	c.expectType(e.Cond, ast.Bool)
	then := c.block(e.Then)
	if e.Else == nil {
		e.Type = ast.Void
		return
	}
	other := c.expr(e.Else)
	switch {
	case then == other:
		e.Type = then
	case then == ast.Never:
		e.Type = other
	case other == ast.Never:
		e.Type = then
	default:
		e.Type = ast.Void
	}
}
