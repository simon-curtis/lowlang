package check

import (
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"slices"
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

type scope struct {
	types map[string]*types.Type
	funcs map[string]*ast.Func
}

type Checker struct {
	modules   map[string]*scope
	embeds    []string
	builtin   map[string]*types.Type
	file      *ast.File
	results   map[[2]*types.Type]*types.Type
	ptrs      map[*types.Type]*types.Type
	opts      map[*types.Type]*types.Type
	slices    map[*types.Type]*types.Type
	maps      map[[2]*types.Type]*types.Type
	fn        *ast.Func
	scopes    []map[string]*ast.Local
	loops     []*loop
	raws      map[*types.Type]*types.Type
	deferring bool
}

func Check(files []*ast.File) (embeds []string, err error) {
	c := &Checker{
		modules: map[string]*scope{},
		builtin: map[string]*types.Type{"bool": types.Bool, "str": types.Str, "__word": types.Word},
		results: map[[2]*types.Type]*types.Type{},
		ptrs:    map[*types.Type]*types.Type{},
		opts:    map[*types.Type]*types.Type{},
		slices:  map[*types.Type]*types.Type{},
		maps:    map[[2]*types.Type]*types.Type{},
		raws:    map[*types.Type]*types.Type{},
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
		c.builtin[t.Name] = t
	}
	c.check(files)
	return c.embeds, nil
}

func (c *Checker) scope(module string) *scope {
	s, ok := c.modules[module]
	if !ok {
		s = &scope{types: map[string]*types.Type{}, funcs: map[string]*ast.Func{}}
		c.modules[module] = s
	}
	return s
}

func (c *Checker) local() *scope {
	return c.scope(c.file.Module)
}

func qualified(module, name string) string {
	if module == "" {
		return name
	}
	return fmt.Sprintf("%s.%s", module, name)
}

func (c *Checker) imported(alias string, pos lexer.Pos) *scope {
	path, ok := c.file.Imported(alias)
	if !ok {
		c.fail(pos, "unknown module %s", alias)
	}
	return c.scope(path)
}

func (c *Checker) isModule(name string) bool {
	_, ok := c.file.Imported(name)
	return ok && c.find(name) == nil
}

func (c *Checker) importedType(alias, name string, pos lexer.Pos) *types.Type {
	t := c.imported(alias, pos).types[name]
	if t == nil {
		c.fail(pos, "module %s has no type %s", alias, name)
	}
	if !t.Pub {
		c.fail(pos, "%s.%s is private", alias, name)
	}
	return t
}

func (c *Checker) importedFunc(alias, name string, pos lexer.Pos) *ast.Func {
	fn := c.imported(alias, pos).funcs[name]
	if fn == nil {
		c.fail(pos, "module %s has no function %s", alias, name)
	}
	if !fn.Pub {
		c.fail(pos, "%s.%s is private", alias, name)
	}
	return fn
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
	case t.Map:
		key := c.resolve(t.Key)
		if !key.Hashable() {
			c.fail(t.Key.Pos, "map keys must be integers, bool, str or pointers, found %s", key)
		}
		return c.mapOf(key, c.resolve(t.Elem))
	case t.Optional:
		return c.optional(c.resolve(t.Elem))
	case t.Raw:
		if t.Elem == nil {
			return c.raw(nil)
		}
		return c.raw(c.resolve(t.Elem))
	}
	if t.Module != "" {
		return c.importedType(t.Module, t.Name, t.Pos)
	}
	if typ, ok := c.builtin[t.Name]; ok {
		return typ
	}
	typ, ok := c.local().types[t.Name]
	if !ok {
		c.fail(t.Pos, "unknown type %s", t.Name)
	}
	return typ
}

func (c *Checker) raw(elem *types.Type) *types.Type {
	if r, ok := c.raws[elem]; ok {
		return r
	}
	r := types.NewRaw(elem)
	c.raws[elem] = r
	return r
}

func (c *Checker) optional(elem *types.Type) *types.Type {
	if o, ok := c.opts[elem]; ok {
		return o
	}
	o := types.NewOptional(elem)
	c.opts[elem] = o
	if elem.Size() > 0 {
		c.layout(o, lexer.Pos{})
	}
	return o
}

func (c *Checker) mapOf(key, value *types.Type) *types.Type {
	k := [2]*types.Type{key, value}
	if m, ok := c.maps[k]; ok {
		return m
	}
	m := types.NewMap(key, value)
	c.maps[k] = m
	return m
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

func (c *Checker) check(files []*ast.File) {
	c.declareTypes(files)
	c.declareFuncs(files)
	c.checkMain()
	for _, f := range files {
		for _, fn := range f.Funcs {
			c.function(fn)
		}
	}
}

func (c *Checker) declareType(name string, pos lexer.Pos, t *types.Type, pub bool) {
	_, builtin := c.builtin[name]
	if _, dup := c.local().types[name]; dup || builtin {
		c.fail(pos, "type %s already declared", name)
	}
	t.Module = c.file.Module
	t.Pub = pub
	c.local().types[name] = t
}

func (c *Checker) declareTypes(files []*ast.File) {
	for _, f := range files {
		c.file = f
		for _, d := range f.Enums {
			if len(d.Variants) == 0 {
				c.fail(d.Pos, "enum %s has no variants", d.Name)
			}
			d.Type = types.NewEnum(qualified(f.Module, d.Name))
			c.declareType(d.Name, d.Pos, d.Type, d.Pub)
		}
		for _, d := range f.Structs {
			if len(d.Fields) == 0 {
				c.fail(d.Pos, "struct %s has no fields", d.Name)
			}
			d.Type = types.NewStruct(qualified(f.Module, d.Name))
			d.Type.C = d.C
			c.declareType(d.Name, d.Pos, d.Type, d.Pub)
		}
	}
	for _, f := range files {
		c.file = f
		for _, d := range f.Enums {
			c.declareVariants(d)
		}
		for _, d := range f.Structs {
			c.declareFields(d)
		}
	}
	for _, f := range files {
		for _, d := range f.Enums {
			c.layout(d.Type, d.Pos)
		}
		for _, d := range f.Structs {
			c.layout(d.Type, d.Pos)
		}
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
		t := c.resolve(fd.TypeName)
		if d.C && !t.CCompatible() {
			c.fail(fd.TypeName.Pos, "extern struct fields must have C types, found %s", t)
		}
		d.Type.Fields = append(d.Type.Fields, &types.Field{Name: fd.Name, Pub: fd.Pub, Type: t})
	}
}

func (c *Checker) declareFuncs(files []*ast.File) {
	for _, f := range files {
		c.file = f
		for _, fn := range f.Funcs {
			c.declareFunc(fn)
		}
		for _, e := range f.Externs {
			for _, fn := range e.Funcs {
				c.declareFunc(fn)
				c.cSignature(fn)
			}
		}
	}
}

func (c *Checker) cSignature(fn *ast.Func) {
	for i, p := range fn.Params {
		if !fn.ParamTypes[i].CCompatible() {
			c.fail(p.TypeName.Pos, "extern parameters must have C types, found %s", fn.ParamTypes[i])
		}
	}
	if fn.RetType != types.Void && !fn.RetType.CCompatible() {
		c.fail(fn.Result.Pos, "extern results must have C types, found %s", fn.RetType)
	}
}

func (c *Checker) declareFunc(fn *ast.Func) {
	if _, dup := c.local().funcs[fn.Name]; dup {
		c.fail(fn.Pos, "function %s already declared", fn.Name)
	}
	if builtins[fn.Name] {
		c.fail(fn.Pos, "%s is a builtin", fn.Name)
	}
	_, builtin := c.builtin[fn.Name]
	if _, clash := c.local().types[fn.Name]; clash || builtin {
		c.fail(fn.Pos, "function %s has the name of a type", fn.Name)
	}
	c.signature(fn)
	c.local().funcs[fn.Name] = fn
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
	main, ok := c.scope("").funcs["main"]
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
	c.file = fn.File
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
	case ret.Accepts(t) || c.okType().Accepts(t):
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
		c.leaving(s.Pos, "break")
		c.innermostLoop(s.Pos, "break").broken = true
		return true
	case *ast.Continue:
		c.leaving(s.Pos, "continue")
		c.innermostLoop(s.Pos, "continue")
		return true
	case *ast.Return:
		c.leaving(s.Pos, "return")
		c.ret(s)
		return true
	case *ast.Fail:
		c.leaving(s.Pos, "fail")
		c.failStmt(s)
		return true
	case *ast.Defer:
		c.deferStmt(s)
	}
	return false
}

func (c *Checker) deferStmt(s *ast.Defer) {
	if c.deferring {
		c.fail(s.Pos, "defer cannot be nested in a deferred statement")
	}
	c.deferring = true
	loops := c.loops
	c.loops = nil
	c.pushScope()
	if c.stmt(s.Body) {
		c.fail(s.Pos, "a deferred statement cannot leave its scope")
	}
	c.popScope()
	c.loops = loops
	c.deferring = false
}

func (c *Checker) leaving(pos lexer.Pos, keyword string) {
	if c.deferring {
		c.fail(pos, "%s cannot appear in a deferred statement", keyword)
	}
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
	if want != nil && !want.Accepts(t) {
		c.fail(s.Value.Position(), "cannot use %s as %s", t, want)
	}
	if want != nil {
		t = want
	}
	s.Local = c.declare(s.Name, t, s.Mutable)
}

func (c *Checker) assign(s *ast.Assign) {
	t := c.assignTarget(s)
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
	if !t.Accepts(value) {
		c.fail(s.Value.Position(), "cannot assign %s to a place of type %s", value, t)
	}
}

func (c *Checker) assignTarget(s *ast.Assign) *types.Type {
	if entry, ok := s.Target.(*ast.Index); ok {
		if m := c.plain(entry.X, nil); m.Kind == types.MapKind {
			c.key(entry.Index, m)
			entry.Type = c.optional(m.Elem)
			s.Entry = entry
			return m.Elem
		}
	}
	place, t := c.place(s.Target, s.Pos, "assign to %s", "cannot assign to this expression")
	s.Place = place
	return t
}

func (c *Checker) key(e ast.Expr, m *types.Type) {
	c.expectType(e, m.Key)
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
	case types.MapKind:
		if s.Key == "" {
			return nil, t.Key
		}
		return t.Key, t.Elem
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
	if got := c.plain(e, want); !want.Accepts(got) {
		c.fail(e.Position(), "expected %s, found %s", want, got)
	}
}

func (c *Checker) expr(e ast.Expr, want *types.Type) *types.Type {
	switch e := e.(type) {
	case *ast.IntLit:
		want = unwrap(want)
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
		want = unwrap(want)
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
		c.sliceLit(e, unwrap(want))
	case *ast.MapLit:
		c.mapLit(e, unwrap(want))
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

func (c *Checker) inferVariant(want *types.Type, name string) (*types.Type, *types.Variant) {
	for _, t := range []*types.Type{want, unwrap(want)} {
		if t == nil || t.Kind != types.EnumKind {
			continue
		}
		if v := t.Variant(name); v != nil {
			return t, v
		}
	}
	return nil, nil
}

func (c *Checker) ident(e *ast.Ident, want *types.Type) {
	if enum, v := c.inferVariant(want, e.Name); v != nil {
		c.unitVariant(v, e.Pos)
		e.Variant = v
		e.Type = enum
		return
	}
	if e.Name == "none" && c.find(e.Name) == nil {
		c.fail(e.Pos, "cannot infer the type of none; annotate the binding")
	}
	if e.Name == "null" && c.find(e.Name) == nil {
		e.Null = true
		e.Type = c.raw(nil)
		if w := unwrap(want); w != nil && w.Kind == types.RawKind {
			e.Type = w
		}
		return
	}
	e.Local = c.lookup(e.Name, e.Pos)
	e.Type = e.Local.Type
}

func (c *Checker) typeRef(x ast.Expr) *types.Type {
	var t *types.Type
	switch x := x.(type) {
	case *ast.Ident:
		if c.find(x.Name) != nil {
			return nil
		}
		t = c.local().types[x.Name]
	case *ast.Selector:
		id, ok := x.X.(*ast.Ident)
		if !ok || !c.isModule(id.Name) {
			return nil
		}
		if c.imported(id.Name, id.Pos).types[x.Name] == nil {
			return nil
		}
		t = c.importedType(id.Name, x.Name, x.Pos)
	}
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
	c.reachable(f, t, pos)
	return f
}

func (c *Checker) reachable(f *types.Field, t *types.Type, pos lexer.Pos) {
	if !f.Pub && t.Module != c.file.Module {
		c.fail(pos, "field %s of %s is private", f.Name, t)
	}
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
	if e.Name == "len" && (t.Kind == types.SliceKind || t.Kind == types.MapKind || t == types.Str) {
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
	case t.Kind == types.MapKind:
		c.key(e.Index, t)
		e.Type = c.optional(t.Elem)
		return
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

func (c *Checker) mapLit(e *ast.MapLit, want *types.Type) {
	if want != nil && want.Kind == types.MapKind {
		e.Type = want
	} else if len(e.Keys) == 0 {
		c.fail(e.Pos, "cannot infer the type of an empty map; annotate it")
	} else {
		key := c.plain(e.Keys[0], nil)
		if !key.Hashable() {
			c.fail(e.Keys[0].Position(), "map keys must be integers, bool, str or pointers, found %s", key)
		}
		e.Type = c.mapOf(key, c.plain(e.Values[0], nil))
	}
	for i := range e.Keys {
		c.expectType(e.Keys[i], e.Type.Key)
		c.expectType(e.Values[i], e.Type.Elem)
	}
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
	var t *types.Type
	if e.Module != "" {
		t = c.importedType(e.Module, e.Name, e.Pos)
	} else {
		t = c.local().types[e.Name]
	}
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
		c.reachable(f, t, init.Pos)
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

func unwrap(want *types.Type) *types.Type {
	if want != nil && want.Optional {
		return want.Elem
	}
	return want
}

func numeric(want *types.Type) *types.Type {
	want = unwrap(want)
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

func (c *Checker) coalesce(e *ast.Binary) {
	x := c.plain(e.X, nil)
	if !x.Optional {
		c.fail(e.X.Position(), "?? requires an optional, found %s", x)
	}
	y := c.plain(e.Y, x.Elem)
	switch {
	case x.Elem.Accepts(y):
		e.Type = x.Elem
	case y == x:
		e.Type = x
	default:
		c.fail(e.Y.Position(), "expected %s, found %s", x.Elem, y)
	}
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
	case lexer.Coalesce:
		c.coalesce(e)
	case lexer.Shl, lexer.Shr:
		e.Type = c.plain(e.X, numeric(want))
		c.requireInteger(e.Type, e.Op, e.X.Position())
		c.requireInteger(c.plain(e.Y, nil), e.Op, e.Y.Position())
	case lexer.Eq, lexer.NotEq:
		t := c.operands(e, nil)
		if !t.Number() && t != types.Bool && t != types.Str && t.Kind != types.PointerKind && t.Kind != types.RawKind {
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
	"delete": true, "embed": true, "__str_raw": true,
	"__slice_ptr": true, "cstr": true, "view": true, "load": true, "store": true, "pin": true, "unpin": true,
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
		if t, ok := c.builtin[fn.Name]; ok && t.Number() && c.find(fn.Name) == nil {
			c.convert(e, t)
			return
		}
		if enum, v := c.inferVariant(want, fn.Name); v != nil {
			c.construct(e, enum, v)
			return
		}
		target, ok := c.local().funcs[fn.Name]
		if !ok {
			c.fail(e.Pos, "undefined function: %s", fn.Name)
		}
		c.callFunc(e, target)
	case *ast.Selector:
		if id, ok := fn.X.(*ast.Ident); ok && c.isModule(id.Name) && c.typeRef(fn.X) == nil {
			c.callFunc(e, c.importedFunc(id.Name, fn.Name, fn.Pos))
			return
		}
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
		e.Builtin, e.Type = "from_bytes", types.Str
		if t := c.plain(e.Args[0], nil); t.Kind == types.RawKind {
			c.cString(e.Args[0], t)
			e.Builtin = "from_cstr"
		} else if t != c.slice(types.U8) {
			c.fail(e.Args[0].Position(), "str takes []u8 or ptr[u8], found %s", t)
		}
	case "cstr":
		c.single(e, name)
		c.expectType(e.Args[0], types.Str)
		e.Builtin, e.Type = name, c.raw(types.U8)
	case "view":
		if len(e.Args) != 2 {
			c.fail(e.Pos, "view takes 2 arguments, found %d", len(e.Args))
		}
		elem := c.typedRaw(e.Args[0], name)
		c.integer(e.Args[1])
		e.Builtin, e.Type = name, c.slice(elem)
	case "load":
		c.single(e, name)
		e.Builtin, e.Type = name, c.typedRaw(e.Args[0], name)
	case "store":
		if len(e.Args) != 2 {
			c.fail(e.Pos, "store takes 2 arguments, found %d", len(e.Args))
		}
		c.expectType(e.Args[1], c.typedRaw(e.Args[0], name))
		e.Builtin, e.Type = name, types.Void
	case "pin":
		c.single(e, name)
		t := c.plain(e.Args[0], nil)
		if t.Kind != types.PointerKind && t.Kind != types.SliceKind && t.Kind != types.MapKind && t != types.Str {
			c.fail(e.Args[0].Position(), "pin takes a pointer, slice, map or str, found %s", t)
		}
		e.Builtin, e.Type = name, c.raw(nil)
	case "unpin":
		c.single(e, name)
		if t := c.plain(e.Args[0], nil); t.Kind != types.RawKind {
			c.fail(e.Args[0].Position(), "unpin takes the ptr returned by pin, found %s", t)
		}
		e.Builtin, e.Type = name, types.Void
	case "__bits":
		c.single(e, name)
		t := c.plain(e.Args[0], nil)
		e.Builtin, e.Type = name, types.I64
		if t == types.F32 {
			e.Type = types.U32
		} else if t != types.F64 {
			c.fail(e.Pos, "__bits requires a float")
		}
	case "delete":
		c.deleteEntry(e)
	case "embed":
		c.embed(e)
	case "__str_raw":
		if len(e.Args) != 2 {
			c.fail(e.Pos, "__str_raw takes 2 arguments")
		}
		c.integer(e.Args[0])
		c.integer(e.Args[1])
		e.Builtin, e.Type = name, types.Str
	case "__slice_ptr":
		c.single(e, name)
		t := c.plain(e.Args[0], nil)
		if t.Kind != types.SliceKind {
			c.fail(e.Pos, "__slice_ptr requires a slice")
		}
		e.Builtin, e.Type = name, c.raw(t.Elem)
	case "__str_view":
		c.single(e, name)
		c.expectType(e.Args[0], c.slice(types.U8))
		e.Builtin, e.Type = name, types.Str
	default:
		return false
	}
	return true
}

func (c *Checker) embed(e *ast.Call) {
	c.single(e, "embed")
	lit, ok := e.Args[0].(*ast.StrLit)
	if !ok {
		c.fail(e.Args[0].Position(), "embed needs a string literal path")
	}
	path := lit.Value
	if !filepath.IsAbs(path) {
		path = filepath.Join(filepath.Dir(c.file.Path), path)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		c.fail(lit.Pos, "cannot embed %s: %v", lit.Value, err)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		c.fail(lit.Pos, "cannot embed %s: no such file", lit.Value)
	}
	if !slices.Contains(c.embeds, abs) {
		c.embeds = append(c.embeds, abs)
	}
	digest := fnv.New64a()
	digest.Write(data)
	e.Builtin, e.Embed, e.Size, e.Digest, e.Type = "embed", abs, int64(len(data)), digest.Sum64(), types.Str
}

func (c *Checker) deleteEntry(e *ast.Call) {
	if len(e.Args) != 2 {
		c.fail(e.Pos, "delete takes 2 arguments, found %d", len(e.Args))
	}
	r, ok := e.Args[0].(*ast.Ref)
	if !ok {
		c.fail(e.Args[0].Position(), "delete needs a mut map; write delete(mut m, key)")
	}
	t := c.borrow(r)
	if t.Kind != types.MapKind {
		c.fail(r.Pos, "delete needs a map, found %s", t)
	}
	c.key(e.Args[1], t)
	e.Builtin = "delete"
	e.Type = types.Void
}

func (c *Checker) typedRaw(e ast.Expr, name string) *types.Type {
	t := c.plain(e, nil)
	if t.Kind != types.RawKind || t.Elem == nil {
		c.fail(e.Position(), "%s needs a typed ptr[T], found %s", name, t)
	}
	return t.Elem
}

func (c *Checker) cString(e ast.Expr, t *types.Type) {
	if t.Elem != nil && t.Elem != types.U8 {
		c.fail(e.Position(), "str takes ptr[u8], found %s", t)
	}
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
	if id, ok := sel.X.(*ast.Ident); ok && t == nil && c.find(id.Name) == nil {
		c.fail(id.Pos, "undefined: %s", id.Name)
	}
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
	short := len(e.Args) < len(fn.Params)
	if short || (len(e.Args) > len(fn.Params) && !fn.Variadic) {
		c.fail(e.Pos, "%s takes %d arguments, found %d", fn.Name, len(fn.Params), len(e.Args))
	}
	for i, arg := range e.Args {
		if i >= len(fn.Params) {
			c.variadic(arg)
			continue
		}
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

func (c *Checker) variadic(arg ast.Expr) {
	if _, ok := arg.(*ast.Ref); ok {
		c.fail(arg.Position(), "variadic arguments cannot be mut")
	}
	if t := c.plain(arg, nil); !t.CCompatible() || t.Kind == types.StructKind {
		c.fail(arg.Position(), "variadic arguments must be numbers, bool or ptr, found %s", t)
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
	case a.Accepts(b):
		return a
	case b.Accepts(a):
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
	c.leaving(e.Pos, "?")
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
