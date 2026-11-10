package check

import (
	"fmt"
	"strings"

	"lowlang/internal/ast"
	"lowlang/internal/lexer"
	"lowlang/internal/types"
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
	funcs   map[string]*ast.Func
	named   map[string]*types.Type
	results map[[2]*types.Type]*types.Type
	fn      *ast.Func
	scopes  []map[string]*ast.Local
	loops   []*loop
}

func Check(file *ast.File) (err error) {
	c := &Checker{
		funcs:   map[string]*ast.Func{},
		named:   map[string]*types.Type{"i64": types.I64, "bool": types.Bool, "str": types.Str},
		results: map[[2]*types.Type]*types.Type{},
	}
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

func (c *Checker) resolve(t *ast.TypeName) *types.Type {
	typ, ok := c.named[t.Name]
	if !ok {
		c.fail(t.Pos, "unknown type %s", t.Name)
	}
	return typ
}

func (c *Checker) result(ok, err *types.Type) *types.Type {
	key := [2]*types.Type{ok, err}
	if r, found := c.results[key]; found {
		return r
	}
	r := types.NewResult(ok, err)
	c.results[key] = r
	return r
}

func (c *Checker) file(file *ast.File) {
	c.declareEnums(file.Enums)
	c.declareFuncs(file.Funcs)
	c.checkMain()
	for _, fn := range file.Funcs {
		c.function(fn)
	}
}

func (c *Checker) declareEnums(enums []*ast.EnumDecl) {
	for _, d := range enums {
		if _, dup := c.named[d.Name]; dup {
			c.fail(d.Pos, "type %s already declared", d.Name)
		}
		if len(d.Variants) == 0 {
			c.fail(d.Pos, "enum %s has no variants", d.Name)
		}
		d.Type = types.NewEnum(d.Name)
		c.named[d.Name] = d.Type
	}
	for _, d := range enums {
		c.declareVariants(d)
	}
	for _, d := range enums {
		if err := d.Type.Layout(); err != nil {
			c.fail(d.Pos, "%s", err)
		}
	}
}

func (c *Checker) declareVariants(d *ast.EnumDecl) {
	for i, vd := range d.Variants {
		if d.Type.Variant(vd.Name) != nil {
			c.fail(vd.Pos, "variant %s already declared in %s", vd.Name, d.Name)
		}
		v := &types.Variant{Name: vd.Name, Tag: i}
		for _, f := range vd.Fields {
			v.Fields = append(v.Fields, c.resolve(f))
		}
		d.Type.Variants = append(d.Type.Variants, v)
	}
}

func (c *Checker) declareFuncs(funcs []*ast.Func) {
	for _, fn := range funcs {
		if _, dup := c.funcs[fn.Name]; dup {
			c.fail(fn.Pos, "function %s already declared", fn.Name)
		}
		if fn.Name == "print" {
			c.fail(fn.Pos, "print is a builtin")
		}
		for _, p := range fn.Params {
			fn.ParamTypes = append(fn.ParamTypes, c.resolve(p.TypeName))
		}
		fn.RetType = c.signatureResult(fn)
		c.funcs[fn.Name] = fn
	}
}

func (c *Checker) signatureResult(fn *ast.Func) *types.Type {
	ok := types.Void
	if fn.Result != nil {
		ok = c.resolve(fn.Result)
	}
	if fn.Error == nil {
		return ok
	}
	err := c.resolve(fn.Error)
	if err.Kind != types.EnumKind {
		c.fail(fn.Error.Pos, "error type must be an enum, found %s", err)
	}
	return c.result(ok, err)
}

func (c *Checker) checkMain() {
	main, ok := c.funcs["main"]
	if !ok {
		c.fail(lexer.Pos{Line: 1, Col: 1}, "missing function main")
	}
	ret := main.RetType
	if len(main.Params) > 0 || (ret != types.Void && (ret.Kind != types.ResultKind || ret.Ok != types.Void)) {
		c.fail(main.Pos, "main must take no parameters and return nothing or ! E")
	}
}

func (c *Checker) function(fn *ast.Func) {
	c.fn = fn
	c.scopes = []map[string]*ast.Local{{}}
	for i, p := range fn.Params {
		if _, dup := c.scopes[0][p.Name]; dup {
			c.fail(p.Pos, "parameter %s already declared", p.Name)
		}
		t := fn.ParamTypes[i]
		c.scopes[0][p.Name] = &ast.Local{Name: p.Name, Type: t, Offset: 16 + 8*fn.ParamWords}
		fn.ParamWords += t.Size()
	}
	body := c.expr(fn.Body, c.okType())
	if body.Kind == types.ResultKind && body != fn.RetType {
		c.handled(fn.Body.Tail, body)
	}
	if !c.returnable(body) {
		c.fail(fn.Body.Pos, "function %s must return %s, body produces %s", fn.Name, c.okType(), body)
	}
}

func (c *Checker) okType() *types.Type {
	if c.fn.RetType.Kind == types.ResultKind {
		return c.fn.RetType.Ok
	}
	return c.fn.RetType
}

func (c *Checker) returnable(t *types.Type) bool {
	ret := c.fn.RetType
	switch {
	case t == types.Never || t == ret || t == c.okType():
		return true
	case t.Kind == types.ResultKind:
		return false
	}
	return c.okType() == types.Void
}

func (c *Checker) declare(name string, t *types.Type, mutable bool) *ast.Local {
	l := c.temp(t)
	l.Name = name
	l.Mutable = mutable
	c.scopes[len(c.scopes)-1][name] = l
	return l
}

func (c *Checker) temp(t *types.Type) *ast.Local {
	c.fn.FrameWords += t.Size()
	l := &ast.Local{Type: t, Offset: -8 * c.fn.FrameWords}
	c.fn.Locals = append(c.fn.Locals, l)
	return l
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

func (c *Checker) pushScope() {
	c.scopes = append(c.scopes, map[string]*ast.Local{})
}

func (c *Checker) popScope() {
	c.scopes = c.scopes[:len(c.scopes)-1]
}

func (c *Checker) block(b *ast.Block, want *types.Type) *types.Type {
	c.pushScope()
	defer c.popScope()
	diverges := false
	for _, s := range b.Stmts {
		if c.stmt(s) {
			diverges = true
		}
	}
	b.Type = types.Void
	if b.Tail != nil {
		b.Type = c.expr(b.Tail, want)
	}
	if diverges {
		b.Type = types.Never
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
		t := c.expr(s.X, nil)
		c.handled(s.X, t)
		return t == types.Never
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
	case *ast.Fail:
		c.failStmt(s)
		return true
	}
	return false
}

func (c *Checker) let(s *ast.Let) {
	var want *types.Type
	if s.TypeName != nil {
		want = c.resolve(s.TypeName)
	}
	t := c.plain(s.Value, want)
	if t == types.Void || t == types.Never {
		c.fail(s.Value.Position(), "cannot bind a value of type %s", t)
	}
	if want != nil && want != t {
		c.fail(s.Value.Position(), "cannot use %s as %s", t, want)
	}
	s.Local = c.declare(s.Name, t, s.Mutable)
}

func (c *Checker) assign(s *ast.Assign) {
	l := c.lookup(s.Name, s.Pos)
	if !l.Mutable {
		c.fail(s.Pos, "cannot assign to %s, declared with let", s.Name)
	}
	t := c.plain(s.Value, l.Type)
	if t != l.Type && t != types.Never {
		c.fail(s.Value.Position(), "cannot assign %s to %s of type %s", t, s.Name, l.Type)
	}
	if s.Op != lexer.Assign && l.Type != types.I64 {
		c.fail(s.Pos, "operator %s requires i64, found %s", s.Op, l.Type)
	}
	s.Local = l
}

func (c *Checker) forStmt(s *ast.For) bool {
	if s.Cond != nil {
		c.expectType(s.Cond, types.Bool)
	}
	l := &loop{}
	c.loops = append(c.loops, l)
	c.block(s.Body, nil)
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
	if s.Value == nil {
		if c.okType() != types.Void {
			c.fail(s.Pos, "missing return value of type %s", c.okType())
		}
		return
	}
	t := c.expr(s.Value, c.okType())
	if c.okType() == types.Void && t != c.fn.RetType && t != types.Never {
		c.fail(s.Value.Position(), "function %s does not return a value", c.fn.Name)
	}
	if !c.returnable(t) {
		c.fail(s.Value.Position(), "expected %s, found %s", c.okType(), t)
	}
}

func (c *Checker) failStmt(s *ast.Fail) {
	ret := c.fn.RetType
	if ret.Kind != types.ResultKind {
		c.fail(s.Pos, "fail requires function %s to declare an error type", c.fn.Name)
	}
	c.expectType(s.Value, ret.Err)
}

func (c *Checker) handled(e ast.Expr, t *types.Type) {
	if t.Kind == types.ResultKind {
		c.fail(e.Position(), "unhandled error %s: use ? or catch", t.Err)
	}
}

func (c *Checker) plain(e ast.Expr, want *types.Type) *types.Type {
	t := c.expr(e, want)
	c.handled(e, t)
	return t
}

func (c *Checker) expectType(e ast.Expr, want *types.Type) {
	if got := c.plain(e, want); got != want && got != types.Never {
		c.fail(e.Position(), "expected %s, found %s", want, got)
	}
}

func (c *Checker) expr(e ast.Expr, want *types.Type) *types.Type {
	switch e := e.(type) {
	case *ast.IntLit:
		e.Type = types.I64
	case *ast.StrLit:
		e.Type = types.Str
	case *ast.BoolLit:
		e.Type = types.Bool
	case *ast.Ident:
		c.ident(e, want)
	case *ast.Selector:
		enum := c.enumNamed(e.X)
		e.Variant = c.variant(enum, e.Name, e.Pos)
		c.unitVariant(e.Variant, e.Pos)
		e.Type = enum
	case *ast.Unary:
		c.unary(e)
	case *ast.Binary:
		c.binary(e)
	case *ast.Call:
		c.call(e, want)
	case *ast.If:
		c.ifExpr(e, want)
	case *ast.Block:
		c.block(e, want)
	case *ast.Match:
		c.match(e, want)
	case *ast.Try:
		c.try(e)
	case *ast.Catch:
		c.catch(e)
	}
	return e.TypeOf()
}

func (c *Checker) inferVariant(want *types.Type, name string) *types.Variant {
	if want == nil || want.Kind != types.EnumKind {
		return nil
	}
	return want.Variant(name)
}

func (c *Checker) ident(e *ast.Ident, want *types.Type) {
	if v := c.inferVariant(want, e.Name); v != nil {
		c.unitVariant(v, e.Pos)
		e.Variant = v
		e.Type = want
		return
	}
	e.Local = c.lookup(e.Name, e.Pos)
	e.Type = e.Local.Type
}

func (c *Checker) enumNamed(x ast.Expr) *types.Type {
	id, ok := x.(*ast.Ident)
	if !ok {
		c.fail(x.Position(), "expected an enum name")
	}
	t, ok := c.named[id.Name]
	if !ok || t.Kind != types.EnumKind {
		c.fail(id.Pos, "%s is not an enum", id.Name)
	}
	return t
}

func (c *Checker) variant(enum *types.Type, name string, pos lexer.Pos) *types.Variant {
	v := enum.Variant(name)
	if v == nil {
		c.fail(pos, "%s has no variant %s", enum, name)
	}
	return v
}

func (c *Checker) unitVariant(v *types.Variant, pos lexer.Pos) {
	if len(v.Fields) > 0 {
		c.fail(pos, "variant %s takes %d fields", v.Name, len(v.Fields))
	}
}

func (c *Checker) unary(e *ast.Unary) {
	want := types.I64
	if e.Op == lexer.Not {
		want = types.Bool
	}
	c.expectType(e.X, want)
	e.Type = want
}

func (c *Checker) binary(e *ast.Binary) {
	switch e.Op {
	case lexer.AndAnd, lexer.OrOr:
		c.expectType(e.X, types.Bool)
		c.expectType(e.Y, types.Bool)
		e.Type = types.Bool
	case lexer.Eq, lexer.NotEq:
		x := c.plain(e.X, nil)
		if x != types.I64 && x != types.Bool && x != types.Str {
			c.fail(e.X.Position(), "cannot compare values of type %s", x)
		}
		c.expectType(e.Y, x)
		e.Type = types.Bool
	case lexer.Lt, lexer.LtEq, lexer.Gt, lexer.GtEq:
		c.expectType(e.X, types.I64)
		c.expectType(e.Y, types.I64)
		e.Type = types.Bool
	default:
		c.expectType(e.X, types.I64)
		c.expectType(e.Y, types.I64)
		e.Type = types.I64
	}
}

var printable = map[*types.Type]string{types.I64: "print_i64", types.Str: "print_str", types.Bool: "print_bool"}

func (c *Checker) call(e *ast.Call, want *types.Type) {
	switch fn := e.Fn.(type) {
	case *ast.Ident:
		if fn.Name == "print" {
			c.print(e)
			return
		}
		if v := c.inferVariant(want, fn.Name); v != nil {
			c.construct(e, want, v)
			return
		}
		c.callFunc(e, fn)
	case *ast.Selector:
		enum := c.enumNamed(fn.X)
		c.construct(e, enum, c.variant(enum, fn.Name, fn.Pos))
	default:
		c.fail(e.Pos, "cannot call this expression")
	}
}

func (c *Checker) print(e *ast.Call) {
	if len(e.Args) != 1 {
		c.fail(e.Pos, "print takes 1 argument, found %d", len(e.Args))
	}
	t := c.plain(e.Args[0], nil)
	builtin, ok := printable[t]
	if !ok {
		c.fail(e.Args[0].Position(), "cannot print a value of type %s", t)
	}
	e.Builtin = builtin
	e.Type = types.Void
}

func (c *Checker) callFunc(e *ast.Call, name *ast.Ident) {
	fn, ok := c.funcs[name.Name]
	if !ok {
		c.fail(e.Pos, "undefined function: %s", name.Name)
	}
	if len(e.Args) != len(fn.Params) {
		c.fail(e.Pos, "%s takes %d arguments, found %d", fn.Name, len(fn.Params), len(e.Args))
	}
	for i, arg := range e.Args {
		c.expectType(arg, fn.ParamTypes[i])
	}
	e.Target = fn
	e.Type = fn.RetType
}

func (c *Checker) construct(e *ast.Call, enum *types.Type, v *types.Variant) {
	if len(e.Args) != len(v.Fields) {
		c.fail(e.Pos, "variant %s takes %d fields, found %d", v.Name, len(v.Fields), len(e.Args))
	}
	for i, arg := range e.Args {
		c.expectType(arg, v.Fields[i])
	}
	e.Variant = v
	e.Type = enum
}

func unify(a, b *types.Type) *types.Type {
	switch {
	case a == b || b == types.Never:
		return a
	case a == types.Never:
		return b
	}
	return types.Void
}

func (c *Checker) ifExpr(e *ast.If, want *types.Type) {
	c.expectType(e.Cond, types.Bool)
	then := c.block(e.Then, want)
	if e.Else == nil {
		e.Type = types.Void
		return
	}
	e.Type = unify(then, c.expr(e.Else, want))
}

func (c *Checker) match(e *ast.Match, want *types.Type) {
	t := c.plain(e.X, nil)
	if t.Kind != types.EnumKind {
		c.fail(e.X.Position(), "cannot match on a value of type %s", t)
	}
	e.Temp = c.temp(t)
	e.Type = c.arms(e.Arms, t, want, e.Pos)
}

func (c *Checker) arms(arms []*ast.Arm, enum, want *types.Type, pos lexer.Pos) *types.Type {
	result := types.Never
	covered := map[*types.Variant]bool{}
	wildcard := false
	for _, arm := range arms {
		if wildcard {
			c.fail(arm.Pattern.Pos, "unreachable arm after _")
		}
		c.pushScope()
		c.pattern(arm.Pattern, enum)
		result = unify(result, c.expr(arm.Body, want))
		c.popScope()
		if arm.Pattern.Variant == nil {
			wildcard = true
			continue
		}
		if covered[arm.Pattern.Variant] {
			c.fail(arm.Pattern.Pos, "duplicate arm for %s", arm.Pattern.Name)
		}
		covered[arm.Pattern.Variant] = true
	}
	if !wildcard {
		c.exhaustive(enum, covered, pos)
	}
	return result
}

func (c *Checker) exhaustive(enum *types.Type, covered map[*types.Variant]bool, pos lexer.Pos) {
	var missing []string
	for _, v := range enum.Variants {
		if !covered[v] {
			missing = append(missing, v.Name)
		}
	}
	if len(missing) > 0 {
		c.fail(pos, "missing arms for %s", strings.Join(missing, ", "))
	}
}

func (c *Checker) pattern(p *ast.Pattern, enum *types.Type) {
	if p.Name == "_" {
		if len(p.Bindings) > 0 {
			c.fail(p.Pos, "_ cannot bind fields")
		}
		return
	}
	p.Variant = c.variant(enum, p.Name, p.Pos)
	fields := p.Variant.Fields
	if len(p.Bindings) > 0 && len(p.Bindings) != len(fields) {
		c.fail(p.Pos, "variant %s has %d fields, pattern binds %d", p.Name, len(fields), len(p.Bindings))
	}
	for i, b := range p.Bindings {
		if b.Name != "_" {
			b.Local = c.declare(b.Name, fields[i], false)
		}
	}
}

func (c *Checker) errorResult(e ast.Expr, keyword string, pos lexer.Pos) *types.Type {
	t := c.expr(e, nil)
	if t.Kind != types.ResultKind {
		c.fail(pos, "%s requires an error result, found %s", keyword, t)
	}
	return t
}

func (c *Checker) try(e *ast.Try) {
	t := c.errorResult(e.X, "?", e.Pos)
	ret := c.fn.RetType
	if ret.Kind != types.ResultKind {
		c.fail(e.Pos, "? requires function %s to declare an error type", c.fn.Name)
	}
	if ret.Err != t.Err {
		c.fail(e.Pos, "cannot propagate %s from function with error type %s", t.Err, ret.Err)
	}
	e.Type = t.Ok
}

func (c *Checker) catch(e *ast.Catch) {
	t := c.errorResult(e.X, "catch", e.Pos)
	e.Type = t.Ok
	if e.Fallback != nil {
		c.expectType(e.Fallback, t.Ok)
		return
	}
	e.Temp = c.temp(t.Err)
	arms := c.arms(e.Arms, t.Err, t.Ok, e.Pos)
	if t.Ok != types.Void && arms != t.Ok && arms != types.Never {
		c.fail(e.Pos, "catch arms must produce %s, found %s", t.Ok, arms)
	}
}
