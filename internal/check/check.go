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
	ptrs    map[*types.Type]*types.Type
	slices  map[*types.Type]*types.Type
	fn      *ast.Func
	scopes  []map[string]*ast.Local
	loops   []*loop
}

func Check(file *ast.File) (err error) {
	c := &Checker{
		funcs:   map[string]*ast.Func{},
		named:   map[string]*types.Type{"bool": types.Bool, "str": types.Str},
		results: map[[2]*types.Type]*types.Type{},
		ptrs:    map[*types.Type]*types.Type{},
		slices:  map[*types.Type]*types.Type{},
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
	for _, t := range types.Numeric {
		c.named[t.Name] = t
	}
	c.file(file)
	return nil
}

func (c *Checker) fail(pos lexer.Pos, format string, args ...any) {
	panic(bailout{&Error{Pos: pos, Msg: fmt.Sprintf(format, args...)}})
}

func (c *Checker) resolve(t *ast.TypeName) *types.Type {
	switch {
	case t.Pointer:
		return c.pointer(c.resolve(t.Elem))
	case t.Slice:
		return c.slice(c.resolve(t.Elem))
	}
	typ, ok := c.named[t.Name]
	if !ok {
		c.fail(t.Pos, "unknown type %s", t.Name)
	}
	return typ
}

func (c *Checker) slice(elem *types.Type) *types.Type {
	if s, ok := c.slices[elem]; ok {
		return s
	}
	s := types.NewSlice(elem)
	c.slices[elem] = s
	return s
}

func (c *Checker) pointer(elem *types.Type) *types.Type {
	if p, ok := c.ptrs[elem]; ok {
		return p
	}
	p := types.NewPointer(elem)
	c.ptrs[elem] = p
	return p
}

func deref(t *types.Type) *types.Type {
	if t.Kind == types.PointerKind {
		return t.Elem
	}
	return t
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
	c.declareTypes(file)
	c.declareFuncs(file.Funcs)
	c.checkMain()
	for _, fn := range file.Funcs {
		c.function(fn)
	}
}

func (c *Checker) declareType(name string, pos lexer.Pos, t *types.Type) {
	if _, dup := c.named[name]; dup {
		c.fail(pos, "type %s already declared", name)
	}
	c.named[name] = t
}

func (c *Checker) declareTypes(file *ast.File) {
	for _, d := range file.Enums {
		if len(d.Variants) == 0 {
			c.fail(d.Pos, "enum %s has no variants", d.Name)
		}
		d.Type = types.NewEnum(d.Name)
		c.declareType(d.Name, d.Pos, d.Type)
	}
	for _, d := range file.Structs {
		if len(d.Fields) == 0 {
			c.fail(d.Pos, "struct %s has no fields", d.Name)
		}
		d.Type = types.NewStruct(d.Name)
		c.declareType(d.Name, d.Pos, d.Type)
	}
	for _, d := range file.Enums {
		c.declareVariants(d)
	}
	for _, d := range file.Structs {
		c.declareFields(d)
	}
	for _, d := range file.Enums {
		c.layout(d.Type, d.Pos)
	}
	for _, d := range file.Structs {
		c.layout(d.Type, d.Pos)
	}
}

func (c *Checker) layout(t *types.Type, pos lexer.Pos) {
	if err := t.Layout(); err != nil {
		c.fail(pos, "%s", err)
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

func (c *Checker) declareFields(d *ast.StructDecl) {
	for _, fd := range d.Fields {
		if d.Type.Field(fd.Name) != nil {
			c.fail(fd.Pos, "field %s already declared in %s", fd.Name, d.Name)
		}
		d.Type.Fields = append(d.Type.Fields, &types.Field{Name: fd.Name, Type: c.resolve(fd.TypeName)})
	}
}

func (c *Checker) declareFuncs(funcs []*ast.Func) {
	for _, fn := range funcs {
		if _, dup := c.funcs[fn.Name]; dup {
			c.fail(fn.Pos, "function %s already declared", fn.Name)
		}
		if builtins[fn.Name] {
			c.fail(fn.Pos, "%s is a builtin", fn.Name)
		}
		if _, clash := c.named[fn.Name]; clash {
			c.fail(fn.Pos, "function %s has the name of a type", fn.Name)
		}
		c.signature(fn)
		c.funcs[fn.Name] = fn
	}
}

func (c *Checker) signature(fn *ast.Func) {
	for _, p := range fn.Params {
		fn.ParamTypes = append(fn.ParamTypes, c.resolve(p.TypeName))
	}
	fn.RetType = c.signatureResult(fn)
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
		local := &ast.Local{Name: p.Name, Type: fn.ParamTypes[i], Mutable: p.Mut, Param: true, Ref: p.Mut}
		c.scopes[0][p.Name] = local
		fn.ParamLocals = append(fn.ParamLocals, local)
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
	l := &ast.Local{Name: name, Type: t, Mutable: mutable}
	c.scopes[len(c.scopes)-1][name] = l
	return l
}

func (c *Checker) find(name string) *ast.Local {
	for i := len(c.scopes) - 1; i >= 0; i-- {
		if l, ok := c.scopes[i][name]; ok {
			return l
		}
	}
	return nil
}

func (c *Checker) lookup(name string, pos lexer.Pos) *ast.Local {
	l := c.find(name)
	if l == nil {
		c.fail(pos, "undefined: %s", name)
	}
	return l
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
	case *ast.ForIn:
		c.forIn(s)
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
	place, t := c.place(s.Target, s.Pos, "assign to %s", "cannot assign to this expression")
	s.Place = place
	switch s.Op {
	case lexer.Assign:
	case lexer.PlusAssign, lexer.MinusAssign, lexer.StarAssign, lexer.SlashAssign:
		c.requireNumber(t, s.Op, s.Pos)
	default:
		c.requireInteger(t, s.Op, s.Pos)
	}
	if s.Op == lexer.ShlAssign || s.Op == lexer.ShrAssign {
		c.requireInteger(c.plain(s.Value, nil), s.Op, s.Value.Position())
		return
	}
	value := c.plain(s.Value, t)
	if value != t && value != types.Never {
		c.fail(s.Value.Position(), "cannot assign %s to a place of type %s", value, t)
	}
}

func (c *Checker) requireNumber(t *types.Type, op lexer.Kind, pos lexer.Pos) {
	if !t.Number() && t != types.Never {
		c.fail(pos, "operator %s requires a number, found %s", op, t)
	}
}

func (c *Checker) requireInteger(t *types.Type, op lexer.Kind, pos lexer.Pos) {
	if !t.Integer() && t != types.Never {
		c.fail(pos, "operator %s requires an integer, found %s", op, t)
	}
}

func (c *Checker) mutable(root *ast.Local, pos lexer.Pos, action string) {
	switch {
	case root.Param && !root.Ref:
		c.fail(pos, "cannot %s; declare the parameter mut", fmt.Sprintf(action, root.Name))
	case !root.Mutable:
		c.fail(pos, "cannot %s, declared with let", fmt.Sprintf(action, root.Name))
	}
}

func (c *Checker) place(e ast.Expr, pos lexer.Pos, action, invalid string) (*ast.Place, *types.Type) {
	t := c.plain(e, nil)
	place, ok := ast.PlaceOf(e)
	if !ok {
		c.fail(pos, "%s", invalid)
	}
	if place.Local != nil {
		c.mutable(place.Local, pos, action)
	}
	return place, t
}

func (c *Checker) ref(r *ast.Ref, want *types.Type) {
	if t := c.borrow(r); t != want {
		c.fail(r.Pos, "expected %s, found %s", want, t)
	}
}

func (c *Checker) borrow(r *ast.Ref) *types.Type {
	place, t := c.place(r.X, r.Pos, "pass %s as mut", "mut argument must be a variable, a field, an element or a dereferenced pointer")
	if place.Local != nil && !place.Local.Ref {
		place.Local.Addressed = true
	}
	r.Place = place
	r.Type = t
	return t
}

func (c *Checker) exclusive(args []ast.Expr) {
	var refs []*ast.Ref
	for _, arg := range args {
		r, ok := arg.(*ast.Ref)
		if !ok {
			continue
		}
		for _, other := range refs {
			if overlaps(r, other) {
				c.fail(r.Pos, "%s is already passed as mut in this call", r.Place.Local.Name)
			}
		}
		refs = append(refs, r)
	}
}

func overlaps(a, b *ast.Ref) bool {
	pa, pb := a.Place, b.Place
	return pa.Local != nil && pa.Local == pb.Local &&
		pa.Offset < pb.Offset+8*b.Type.Size() &&
		pb.Offset < pa.Offset+8*a.Type.Size()
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

func (c *Checker) forIn(s *ast.ForIn) {
	key, value := c.iteration(s)
	c.pushScope()
	if s.Key != "" {
		s.KeyLocal = c.declare(s.Key, key, false)
	}
	s.ValueLocal = c.declare(s.Value, value, false)
	c.loops = append(c.loops, &loop{})
	c.block(s.Body, nil)
	c.loops = c.loops[:len(c.loops)-1]
	c.popScope()
}

func (c *Checker) iteration(s *ast.ForIn) (key, value *types.Type) {
	if s.End != nil {
		if s.Key != "" {
			c.fail(s.Pos, "a range loop binds one name")
		}
		t := c.rangeType(s.Iter, s.End)
		return nil, t
	}
	t := c.plain(s.Iter, nil)
	switch t.Kind {
	case types.SliceKind:
		return types.I64, t.Elem
	case types.StrKind:
		return types.I64, types.U32
	}
	c.fail(s.Iter.Position(), "cannot iterate over %s", t)
	return nil, nil
}

func (c *Checker) rangeType(lo, hi ast.Expr) *types.Type {
	first, second := lo, hi
	if literal(first) && !literal(second) {
		first, second = second, first
	}
	t := c.plain(first, nil)
	c.expectType(second, t)
	if !t.Integer() {
		c.fail(lo.Position(), "range bounds must be integers, found %s", t)
	}
	return t
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
		if e.Char {
			e.Type = types.U32
		}
		if want != nil && want.Number() {
			e.Type = want
		}
		if e.Type.Integer() && !e.Type.Holds(e.Value) {
			c.fail(e.Pos, "%d overflows %s", e.Value, e.Type)
		}
	case *ast.FloatLit:
		e.Type = types.F64
		if want != nil && want.Float() {
			e.Type = want
		}
	case *ast.StrLit:
		e.Type = types.Str
	case *ast.BoolLit:
		e.Type = types.Bool
	case *ast.Ident:
		c.ident(e, want)
	case *ast.Selector:
		c.selector(e)
	case *ast.StructLit:
		c.structLit(e)
	case *ast.Ref:
		c.fail(e.Pos, "mut is only allowed as an argument to a mut parameter")
	case *ast.Index:
		c.index(e)
	case *ast.Slicing:
		c.slicing(e)
	case *ast.SliceLit:
		c.sliceLit(e, want)
	case *ast.Interp:
		c.interp(e)
	case *ast.Unary:
		c.unary(e, want)
	case *ast.Binary:
		c.binary(e, want)
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

func (c *Checker) typeRef(x ast.Expr) *types.Type {
	id, ok := x.(*ast.Ident)
	if !ok || c.find(id.Name) != nil {
		return nil
	}
	t := c.named[id.Name]
	if t == nil || (t.Kind != types.EnumKind && t.Kind != types.StructKind) {
		return nil
	}
	return t
}

func (c *Checker) field(t *types.Type, name string, pos lexer.Pos) *types.Field {
	if t.Kind != types.StructKind {
		c.fail(pos, "%s has no field %s", t, name)
	}
	f := t.Field(name)
	if f == nil {
		c.fail(pos, "%s has no field %s", t, name)
	}
	return f
}

func (c *Checker) selector(e *ast.Selector) {
	if t := c.typeRef(e.X); t != nil {
		if t.Kind == types.StructKind {
			c.fail(e.X.Position(), "%s is a type, not a value", t)
		}
		if t.Variant(e.Name) == nil {
			c.fail(e.Pos, "%s has no variant %s", t, e.Name)
		}
		e.Variant = t.Variant(e.Name)
		c.unitVariant(e.Variant, e.Pos)
		e.Type = t
		return
	}
	t := c.plain(e.X, nil)
	if e.Name == "len" && (t.Kind == types.SliceKind || t == types.Str) {
		e.Length = true
		e.Type = types.I64
		return
	}
	e.Field = c.field(deref(t), e.Name, e.Pos)
	e.Type = e.Field.Type
}

func (c *Checker) index(e *ast.Index) {
	t := c.plain(e.X, nil)
	switch {
	case t.Kind == types.SliceKind:
		e.Type = t.Elem
	case t == types.Str:
		e.Type = types.U8
	default:
		c.fail(e.Pos, "cannot index a value of type %s", t)
	}
	c.integer(e.Index)
}

func (c *Checker) integer(e ast.Expr) {
	if t := c.plain(e, nil); !t.Integer() && t != types.Never {
		c.fail(e.Position(), "expected an integer, found %s", t)
	}
}

func (c *Checker) slicing(e *ast.Slicing) {
	e.Type = c.plain(e.X, nil)
	if e.Type.Kind != types.SliceKind && e.Type != types.Str {
		c.fail(e.Pos, "cannot slice a value of type %s", e.Type)
	}
	for _, bound := range []ast.Expr{e.Lo, e.Hi} {
		if bound != nil {
			c.integer(bound)
		}
	}
}

func (c *Checker) sliceLit(e *ast.SliceLit, want *types.Type) {
	if want != nil && want.Kind == types.SliceKind {
		e.Type = want
		for _, el := range e.Elems {
			c.expectType(el, want.Elem)
		}
		return
	}
	if len(e.Elems) == 0 {
		c.fail(e.Pos, "cannot infer the type of an empty slice; annotate it")
	}
	elem := c.plain(e.Elems[0], nil)
	for _, el := range e.Elems[1:] {
		c.expectType(el, elem)
	}
	e.Type = c.slice(elem)
}

func (c *Checker) interp(e *ast.Interp) {
	for _, part := range e.Parts {
		t := c.plain(part, nil)
		if !t.Number() && t != types.Bool && t != types.Str {
			c.fail(part.Position(), "cannot interpolate a value of type %s", t)
		}
	}
	e.Type = types.Str
}

func (c *Checker) structLit(e *ast.StructLit) {
	t := c.named[e.Name]
	if t == nil || t.Kind != types.StructKind {
		c.fail(e.Pos, "%s is not a struct", e.Name)
	}
	given := map[*types.Field]bool{}
	for _, init := range e.Fields {
		f := t.Field(init.Name)
		if f == nil {
			c.fail(init.Pos, "%s has no field %s", t, init.Name)
		}
		if given[f] {
			c.fail(init.Pos, "field %s given twice", init.Name)
		}
		given[f] = true
		init.Field = f
		c.expectType(init.Value, f.Type)
	}
	var missing []string
	for _, f := range t.Fields {
		if !given[f] {
			missing = append(missing, f.Name)
		}
	}
	if len(missing) > 0 {
		c.fail(e.Pos, "missing fields for %s: %s", t, strings.Join(missing, ", "))
	}
	e.Type = t
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

func (c *Checker) unary(e *ast.Unary, want *types.Type) {
	switch e.Op {
	case lexer.Star:
		t := c.plain(e.X, nil)
		if t.Kind != types.PointerKind {
			c.fail(e.Pos, "cannot dereference a value of type %s", t)
		}
		e.Type = t.Elem
	case lexer.Not:
		c.expectType(e.X, types.Bool)
		e.Type = types.Bool
	case lexer.Tilde:
		e.Type = c.plain(e.X, numeric(want))
		c.requireInteger(e.Type, e.Op, e.Pos)
	default:
		e.Type = c.plain(e.X, numeric(want))
		c.requireNumber(e.Type, e.Op, e.Pos)
	}
}

func numeric(want *types.Type) *types.Type {
	if want != nil && want.Number() {
		return want
	}
	return nil
}

func literal(e ast.Expr) bool {
	switch e.(type) {
	case *ast.IntLit, *ast.FloatLit:
		return true
	}
	return false
}

func (c *Checker) operands(e *ast.Binary, want *types.Type) *types.Type {
	first, second := e.X, e.Y
	if literal(first) && !literal(second) {
		first, second = second, first
	}
	t := c.plain(first, want)
	c.expectType(second, t)
	return t
}

func (c *Checker) binary(e *ast.Binary, want *types.Type) {
	switch e.Op {
	case lexer.AndAnd, lexer.OrOr:
		c.expectType(e.X, types.Bool)
		c.expectType(e.Y, types.Bool)
		e.Type = types.Bool
	case lexer.Shl, lexer.Shr:
		e.Type = c.plain(e.X, numeric(want))
		c.requireInteger(e.Type, e.Op, e.X.Position())
		c.requireInteger(c.plain(e.Y, nil), e.Op, e.Y.Position())
	case lexer.Eq, lexer.NotEq:
		t := c.operands(e, nil)
		if !t.Number() && t != types.Bool && t != types.Str && t.Kind != types.PointerKind {
			c.fail(e.X.Position(), "cannot compare values of type %s", t)
		}
		e.Type = types.Bool
	case lexer.Lt, lexer.LtEq, lexer.Gt, lexer.GtEq:
		c.requireNumber(c.operands(e, nil), e.Op, e.Pos)
		e.Type = types.Bool
	case lexer.Percent, lexer.Amp, lexer.Pipe, lexer.Caret:
		e.Type = c.operands(e, numeric(want))
		c.requireInteger(e.Type, e.Op, e.Pos)
	default:
		e.Type = c.operands(e, numeric(want))
		if e.Op == lexer.Plus && e.Type == types.Str {
			return
		}
		c.requireNumber(e.Type, e.Op, e.Pos)
	}
}

var builtins = map[string]bool{
	"print": true, "new": true, "sqrt": true, "floor": true, "exp": true, "push": true, "bytes": true,
	"__bits": true, "__str_view": true,
}

func (c *Checker) call(e *ast.Call, want *types.Type) {
	switch fn := e.Fn.(type) {
	case *ast.Ident:
		if fn.Name == "print" {
			c.print(e)
			return
		}
		if fn.Name == "new" {
			c.allocate(e, want)
			return
		}
		if c.special(e, fn.Name) {
			return
		}
		if builtins[fn.Name] {
			c.math(e, fn.Name, want)
			return
		}
		if t, ok := c.named[fn.Name]; ok && t.Number() && c.find(fn.Name) == nil {
			c.convert(e, t)
			return
		}
		if v := c.inferVariant(want, fn.Name); v != nil {
			c.construct(e, want, v)
			return
		}
		target, ok := c.funcs[fn.Name]
		if !ok {
			c.fail(e.Pos, "undefined function: %s", fn.Name)
		}
		c.callFunc(e, target)
	case *ast.Selector:
		c.variantCall(e, fn)
	default:
		c.fail(e.Pos, "cannot call this expression")
	}
}

func (c *Checker) print(e *ast.Call) {
	if len(e.Args) != 1 {
		c.fail(e.Pos, "print takes 1 argument, found %d", len(e.Args))
	}
	t := c.plain(e.Args[0], nil)
	switch {
	case t.Integer():
		e.Builtin = "print_i64"
	case t.Float():
		e.Builtin = "print_float"
	case t == types.Bool:
		e.Builtin = "print_bool"
	case t == types.Str:
		e.Builtin = "print_str"
	default:
		c.fail(e.Args[0].Position(), "cannot print a value of type %s", t)
	}
	e.Type = types.Void
}

func (c *Checker) special(e *ast.Call, name string) bool {
	switch name {
	case "push":
		c.push(e)
	case "bytes":
		c.single(e, name)
		c.expectType(e.Args[0], types.Str)
		e.Builtin, e.Type = name, c.slice(types.U8)
	case "str":
		c.single(e, name)
		c.expectType(e.Args[0], c.slice(types.U8))
		e.Builtin, e.Type = "from_bytes", types.Str
	case "__bits":
		c.single(e, name)
		t := c.plain(e.Args[0], nil)
		e.Builtin, e.Type = name, types.I64
		if t == types.F32 {
			e.Type = types.U32
		} else if t != types.F64 {
			c.fail(e.Pos, "__bits requires a float")
		}
	case "__str_view":
		c.single(e, name)
		c.expectType(e.Args[0], c.slice(types.U8))
		e.Builtin, e.Type = name, types.Str
	default:
		return false
	}
	return true
}

func (c *Checker) push(e *ast.Call) {
	if len(e.Args) != 2 {
		c.fail(e.Pos, "push takes 2 arguments, found %d", len(e.Args))
	}
	r, ok := e.Args[0].(*ast.Ref)
	if !ok {
		c.fail(e.Args[0].Position(), "push needs a mut slice; write push(mut s, value)")
	}
	t := c.borrow(r)
	if t.Kind != types.SliceKind {
		c.fail(r.Pos, "push needs a slice, found %s", t)
	}
	c.expectType(e.Args[1], t.Elem)
	e.Builtin = "push"
	e.Type = types.Void
}

func (c *Checker) single(e *ast.Call, name string) {
	if len(e.Args) != 1 {
		c.fail(e.Pos, "%s takes 1 argument, found %d", name, len(e.Args))
	}
}

func (c *Checker) math(e *ast.Call, name string, want *types.Type) {
	c.single(e, name)
	arg := types.F64
	if want != nil && want.Float() {
		arg = want
	}
	t := c.plain(e.Args[0], arg)
	if !t.Float() {
		c.fail(e.Args[0].Position(), "%s requires f32 or f64, found %s", name, t)
	}
	e.Builtin = name
	e.Type = t
}

func (c *Checker) convert(e *ast.Call, to *types.Type) {
	c.single(e, to.Name)
	t := c.plain(e.Args[0], nil)
	if !t.Number() {
		c.fail(e.Args[0].Position(), "cannot convert %s to %s", t, to)
	}
	e.Builtin = "convert"
	e.Type = to
}

func (c *Checker) variantCall(e *ast.Call, sel *ast.Selector) {
	t := c.typeRef(sel.X)
	if t == nil || t.Kind != types.EnumKind {
		c.fail(e.Pos, "cannot call this expression")
	}
	c.construct(e, t, c.variant(t, sel.Name, sel.Pos))
}

func (c *Checker) allocate(e *ast.Call, want *types.Type) {
	if len(e.Args) != 1 {
		c.fail(e.Pos, "new takes 1 argument, found %d", len(e.Args))
	}
	var elem *types.Type
	if want != nil && want.Kind == types.PointerKind {
		elem = want.Elem
	}
	t := c.plain(e.Args[0], elem)
	if t == types.Void || t == types.Never {
		c.fail(e.Args[0].Position(), "cannot allocate a value of type %s", t)
	}
	e.Builtin = "new"
	e.Type = c.pointer(t)
}

func (c *Checker) callFunc(e *ast.Call, fn *ast.Func) {
	if len(e.Args) != len(fn.Params) {
		c.fail(e.Pos, "%s takes %d arguments, found %d", fn.Name, len(fn.Params), len(e.Args))
	}
	for i, arg := range e.Args {
		c.argument(arg, fn, i)
	}
	c.exclusive(e.Args)
	e.Target = fn
	e.Type = fn.RetType
}

func (c *Checker) argument(arg ast.Expr, fn *ast.Func, i int) {
	r, isRef := arg.(*ast.Ref)
	param := fn.Params[i]
	switch {
	case param.Mut && isRef:
		c.ref(r, fn.ParamTypes[i])
	case param.Mut:
		c.fail(arg.Position(), "parameter %s of %s is mut; write mut before the argument", param.Name, fn.Name)
	default:
		c.expectType(arg, fn.ParamTypes[i])
	}
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
	arms := c.arms(e.Arms, t.Err, t.Ok, e.Pos)
	if t.Ok != types.Void && arms != t.Ok && arms != types.Never {
		c.fail(e.Pos, "catch arms must produce %s, found %s", t.Ok, arms)
	}
}
