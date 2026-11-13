package check

import (
	"errors"
	"fmt"
	"hash/fnv"
	"io/fs"
	"slices"
	"strings"

	"lowlang/internal/ast"
	"lowlang/internal/builtin"
	"lowlang/internal/diag"
	"lowlang/internal/lexer"
	"lowlang/internal/types"
)

type loop struct {
	label  string
	broken bool
	used   bool
}

type scope struct {
	types    map[string]*types.Type
	generics map[string]*generic
	funcs    map[string]*ast.Func
	consts   map[string]*ast.Const
	home     *ast.File
}

type Checker struct {
	universe *types.Universe
	phase    phase
	waiting  map[phase][]func()
	modules  map[string]*scope
	funcContext
	counts          map[*ast.Func]int
	generics        map[*types.Type]*generic
	pristine        map[*ast.Func]*ast.Func
	instances       map[*ast.Func]map[string]*ast.Func
	pending         []*ast.Func
	spawns          int
	drops           map[*types.Type]*dropSource
	droppables      map[*types.Type]bool
	dropFuncs       map[*types.Type]*ast.Func
	releaseFuncs    map[*types.Type]*ast.Func
	structFiles     map[*types.Type]*ast.File
	flags           int
	subjects        []*types.Type
	fieldSources    map[*types.Field]*fieldSource
	defaults        map[*types.Field]ast.Expr
	checkingDefault map[*types.Field]bool
	derived         map[string]*ast.Func
	folding         map[*ast.Const]bool
	embedder        EmbedResolver
	unpropagated    *types.Type
}

type Unpropagated struct {
	Failure *diag.Error
	Result  *types.Type
}

func (e *Unpropagated) Error() string {
	return e.Failure.Error()
}

func (e *Unpropagated) Unwrap() error {
	return e.Failure
}

type Embedded struct {
	Path string
	Data []byte
}

type EmbedResolver func(file *ast.File, name string) (Embedded, error)

func Check(files []*ast.File, resolve EmbedResolver) (*types.Universe, error) {
	c := &Checker{
		embedder:        resolve,
		waiting:         map[phase][]func(){},
		modules:         map[string]*scope{},
		counts:          map[*ast.Func]int{},
		generics:        map[*types.Type]*generic{},
		pristine:        map[*ast.Func]*ast.Func{},
		instances:       map[*ast.Func]map[string]*ast.Func{},
		drops:           map[*types.Type]*dropSource{},
		droppables:      map[*types.Type]bool{},
		dropFuncs:       map[*types.Type]*ast.Func{},
		releaseFuncs:    map[*types.Type]*ast.Func{},
		structFiles:     map[*types.Type]*ast.File{},
		fieldSources:    map[*types.Field]*fieldSource{},
		defaults:        map[*types.Field]ast.Expr{},
		checkingDefault: map[*types.Field]bool{},
		derived:         map[string]*ast.Func{},
		folding:         map[*ast.Const]bool{},
	}
	c.universe = types.NewUniverse(c.created)
	failure := diag.Try(func() { c.check(files) })
	switch {
	case failure == nil:
		return c.universe, nil
	case c.unpropagated != nil:
		return nil, &Unpropagated{Failure: failure, Result: c.unpropagated}
	}
	return nil, failure
}

func (c *Checker) scope(module string) *scope {
	s, ok := c.modules[module]
	if !ok {
		s = &scope{types: map[string]*types.Type{}, generics: map[string]*generic{}, funcs: map[string]*ast.Func{}, consts: map[string]*ast.Const{}}
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

func (c *Checker) imported(alias string, pos diag.Pos) *scope {
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

func (c *Checker) importedType(alias, name string, pos diag.Pos) *types.Type {
	t := c.imported(alias, pos).types[name]
	if t == nil {
		c.fail(pos, "module %s has no type %s", alias, name)
	}
	if !t.Pub {
		c.fail(pos, "%s.%s is private", alias, name)
	}
	return t
}

func (c *Checker) importedFunc(alias, name string, pos diag.Pos) *ast.Func {
	fn := c.imported(alias, pos).funcs[name]
	if fn == nil {
		c.fail(pos, "module %s has no function %s", alias, name)
	}
	if !fn.Pub {
		c.fail(pos, "%s.%s is private", alias, name)
	}
	return fn
}

func (c *Checker) fail(pos diag.Pos, format string, args ...any) {
	diag.Fail(pos, format, args...)
}

func (c *Checker) resolve(t *ast.TypeName) *types.Type {
	switch {
	case t.Pointer:
		return c.universe.Pointer(c.contained(t.Elem, "a pointer"))
	case t.Slice:
		return c.universe.Slice(c.contained(t.Elem, "a slice"))
	case t.Map:
		key := c.mapKey(c.resolve(t.Key), t.Key.Pos)
		return c.universe.Map(key, c.contained(t.Elem, "a map"))
	case t.Optional:
		return c.universe.Optional(c.contained(t.Elem, "an optional"))
	case t.Func:
		return c.funcTypeName(t)
	}
	if bound, ok := c.bound[t.Name]; ok && t.Module == "" {
		if t.Args != nil {
			c.fail(t.Pos, "type parameter %s takes no type arguments", t.Name)
		}
		return bound
	}
	if b := LookupBuiltin(t.Name); b != nil && b.Kind == TypeBuiltin && t.Module == "" {
		return c.builtinType(b, t)
	}
	if g := c.lookupGeneric(t.Module, t.Name, t.Pos); g != nil {
		if t.Args == nil {
			c.fail(t.Pos, "%s needs type arguments", t.Name)
		}
		args := make([]*types.Type, len(t.Args))
		for i, a := range t.Args {
			args[i] = c.resolve(a)
		}
		return c.instantiateType(g, args, t.Pos)
	}
	if t.Args != nil {
		c.fail(t.Pos, "%s is not generic", t.Name)
	}
	if t.Module != "" {
		return c.importedType(t.Module, t.Name, t.Pos)
	}
	typ, ok := c.local().types[t.Name]
	if !ok {
		c.fail(t.Pos, "unknown type %s", t.Name)
	}
	return typ
}

func (c *Checker) mapKey(key *types.Type, pos diag.Pos) *types.Type {
	if !key.Hashable() {
		c.fail(pos, "map keys must be integers, bool, str or pointers, found %s", key)
	}
	return key
}

func (c *Checker) funcTypeName(t *ast.TypeName) *types.Type {
	params := make([]*types.Type, len(t.Params))
	for i, p := range t.Params {
		params[i] = c.resolve(p)
		if t.CFunc {
			c.requireC(params[i], p.Pos, "extern fn parameters")
		}
	}
	ret := c.resultOf(t.Result, t.Error)
	if t.CFunc {
		c.requireCResult(ret, t.Pos, "extern fn results")
	}
	return c.universe.Func(params, t.Muts, ret, t.CFunc)
}

func (c *Checker) signatureOf(fn *ast.Func, cfn bool) *types.Type {
	muts := make([]bool, len(fn.Params))
	for i, p := range fn.Params {
		muts[i] = p.Mut
	}
	return c.universe.Func(fn.ParamTypes, muts, fn.RetType, cfn)
}

func (c *Checker) check(files []*ast.File) {
	c.declareTypes(files)
	c.declareFuncs(files)
	c.declareConsts(files)
	var funcs []*ast.Func
	for _, f := range files {
		for _, fn := range f.Funcs {
			if fn.Template == nil {
				funcs = append(funcs, fn)
			}
		}
	}
	c.checkDrops(files)
	c.checkDefaults(files)
	c.checkMain(files)
	for _, fn := range funcs {
		c.function(fn)
	}
	for len(c.pending) > 0 {
		fn := c.pending[0]
		c.pending = c.pending[1:]
		c.function(fn)
	}
}

func (c *Checker) declareType(name string, pos diag.Pos, t *types.Type, pub bool) {
	_, generic := c.local().generics[name]
	if _, dup := c.local().types[name]; dup || reservedType(name) || generic {
		c.fail(pos, "type %s already declared", name)
	}
	t.Module = c.file.Module
	t.Pub = pub
	c.local().types[name] = t
}

func (c *Checker) declareTypes(files []*ast.File) {
	for _, f := range files {
		c.file = f
		if home := c.local(); home.home == nil {
			home.home = f
		}
		for _, d := range f.Enums {
			if len(d.Variants) == 0 {
				c.fail(d.Pos, "enum %s has no variants", d.Name)
			}
			if d.TypeParams != nil {
				c.declareGeneric(d.Name, d.TypeParams, d.Pub, d.Pos).enum = d
				continue
			}
			d.Type = types.NewEnum(qualified(f.Module, d.Name))
			c.declareType(d.Name, d.Pos, d.Type, d.Pub)
		}
		for _, d := range f.Structs {
			if len(d.Fields) == 0 {
				c.fail(d.Pos, "struct %s has no fields", d.Name)
			}
			if d.TypeParams != nil && d.C {
				c.fail(d.Pos, "extern struct %s cannot be generic", d.Name)
			}
			if d.TypeParams != nil {
				c.declareGeneric(d.Name, d.TypeParams, d.Pub, d.Pos).strct = d
				continue
			}
			d.Type = types.NewStruct(qualified(f.Module, d.Name))
			d.Type.C = d.C
			c.declareType(d.Name, d.Pos, d.Type, d.Pub)
		}
	}
	for _, f := range files {
		c.file = f
		for _, d := range f.Enums {
			if d.TypeParams != nil {
				d.Type = c.origin(c.local().generics[d.Name])
				continue
			}
			c.declareVariants(d, d.Type)
		}
		for _, d := range f.Structs {
			if d.TypeParams != nil {
				d.Type = c.origin(c.local().generics[d.Name])
				continue
			}
			c.declareFields(d, d.Type)
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
	c.advance(laid)
}

func (c *Checker) layout(t *types.Type, pos diag.Pos) {
	if err := t.Layout(); err != nil {
		c.fail(pos, "%s", err)
	}
}

func (c *Checker) declareVariants(d *ast.EnumDecl, t *types.Type) {
	for i, vd := range d.Variants {
		if t.Variant(vd.Name) != nil {
			c.fail(vd.Pos, "variant %s already declared in %s", vd.Name, d.Name)
		}
		v := &types.Variant{Name: vd.Name, Tag: i, Labels: vd.Labels}
		for _, f := range vd.Fields {
			v.Fields = append(v.Fields, c.contained(f, "an enum variant"))
		}
		t.Variants = append(t.Variants, v)
	}
	c.declareIndexes(d, t)
}

func (c *Checker) declareIndexes(d *ast.EnumDecl, t *types.Type) {
	t.Indexed = slices.ContainsFunc(d.Variants, func(vd *ast.VariantDecl) bool { return vd.Index != nil })
	if !t.Indexed {
		return
	}
	for i, vd := range d.Variants {
		if vd.Index == nil {
			c.fail(vd.Pos, "variant %s has no index; give every variant of %s an index or none", vd.Name, d.Name)
		}
		if vd.Index.Wide {
			c.fail(vd.Index.Pos, "index of variant %s does not fit in i64", vd.Name)
		}
		t.Variants[i].Index = vd.Index.Value
	}
}

func (c *Checker) declareFields(d *ast.StructDecl, st *types.Type) {
	c.recordStruct(d, st)
	for _, fd := range d.Fields {
		if st.Field(fd.Name) != nil {
			c.fail(fd.Pos, "field %s already declared in %s", fd.Name, d.Name)
		}
		t := c.resolve(fd.TypeName)
		if d.C {
			c.requireC(t, fd.TypeName.Pos, "extern struct fields")
		}
		field := &types.Field{Name: fd.Name, Pub: fd.Pub, Type: t}
		st.Fields = append(st.Fields, field)
		c.recordDefault(d, st, fd, field)
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
	c.advance(ready)
}

func (c *Checker) cSignature(fn *ast.Func) {
	for i, p := range fn.Params {
		c.requireC(fn.ParamTypes[i], p.TypeName.Pos, "extern parameters")
	}
	if fn.Result != nil {
		c.requireCResult(fn.RetType, fn.Result.Pos, "extern results")
	}
}

func (c *Checker) requireC(t *types.Type, pos diag.Pos, what string) {
	if !t.CCompatible() {
		c.fail(pos, "%s must have C types, found %s", what, t)
	}
}

func (c *Checker) requireCResult(t *types.Type, pos diag.Pos, what string) {
	if t != types.Void {
		c.requireC(t, pos, what)
	}
}

func (c *Checker) declareFunc(fn *ast.Func) {
	if _, dup := c.local().funcs[fn.Name]; dup {
		c.fail(fn.Pos, "function %s already declared", fn.Name)
	}
	if fixedBuiltin(fn.Name) != nil {
		c.fail(fn.Pos, "%s is a builtin", fn.Name)
	}
	_, generic := c.local().generics[fn.Name]
	if _, clash := c.local().types[fn.Name]; clash || reservedType(fn.Name) || generic {
		c.fail(fn.Pos, "function %s has the name of a type", fn.Name)
	}
	if fn.TypeParams != nil {
		c.declareGenericFunc(fn)
	} else {
		c.signature(fn)
	}
	c.local().funcs[fn.Name] = fn
}

func (c *Checker) signature(fn *ast.Func) {
	for _, p := range fn.Params {
		fn.ParamTypes = append(fn.ParamTypes, c.resolve(p.TypeName))
	}
	fn.RetType = c.resultOf(fn.Result, fn.Error)
}

func (c *Checker) resultOf(result, errName *ast.TypeName) *types.Type {
	ok := types.Void
	if result != nil {
		ok = c.resolve(result)
	}
	if errName == nil {
		return ok
	}
	err := c.resolve(errName)
	if err.Kind != types.EnumKind {
		c.fail(errName.Pos, "error type must be an enum, found %s", err)
	}
	return c.universe.Result(ok, err)
}

func (c *Checker) checkMain(files []*ast.File) {
	main, ok := c.scope("").funcs["main"]
	if !ok {
		c.fail(entryStart(files), "missing function main")
	}
	ret := main.RetType
	if len(main.Params) > 0 || main.TypeParams != nil || (ret != types.Void && (ret.Kind != types.ResultKind || ret.Ok != types.Void)) {
		c.fail(main.Pos, "main must take no parameters and return nothing or ! E")
	}
}

func entryStart(files []*ast.File) diag.Pos {
	i := slices.IndexFunc(files, func(f *ast.File) bool { return f.Module == "" && f.Path != "" })
	if i < 0 {
		return diag.Start("")
	}
	return diag.Start(files[i].Path)
}

func (c *Checker) function(fn *ast.Func) {
	next := funcContext{file: fn.File, bound: c.bindingsOf(fn), abstract: fn.TypeParams != nil}
	c.enter(next, func() {
		c.body(fn)
		if !c.abstract {
			c.markReadOnly(fn)
		}
	})
}

func (c *Checker) body(fn *ast.Func) {
	c.fn = fn
	c.pushScope()
	defer c.popScope()
	params := c.scopes[len(c.scopes)-1]
	for i, p := range fn.Params {
		if _, dup := params[p.Name]; dup {
			c.fail(p.Pos, "parameter %s already declared", p.Name)
		}
		local := &ast.Local{Name: p.Name, Type: fn.ParamTypes[i], Mutable: p.Mut, Param: true, Ref: p.Mut, Owner: fn}
		params[p.Name] = local
		fn.ParamLocals = append(fn.ParamLocals, local)
	}
	body := c.expr(fn.Body, c.okType())
	if fn.RetType == nil {
		c.handled(fn.Body.Tail, body)
		fn.RetType = c.inferredResult(body)
	}
	if body.Kind == types.ResultKind && body != fn.RetType {
		c.handled(fn.Body.Tail, body)
	}
	if !c.returnable(body) {
		c.fail(fn.Body.Pos, "%s must return %s, body produces %s", c.subject(), c.okType(), body)
	}
	if !c.abstract {
		c.checkMoves(fn)
	}
}

func (c *Checker) subject() string {
	if c.fn.Closure != nil {
		return "the closure"
	}
	return fmt.Sprintf("function %s", c.fn.Name)
}

func (c *Checker) closure(e *ast.Closure, want *types.Type) {
	want = unwrap(want)
	if want != nil && (want.Kind != types.FuncKind || want.C || len(want.Params) != len(e.Params)) {
		want = nil
	}
	fn := &ast.Func{Name: c.closureName(), File: c.file, Closure: e, Params: e.Params, Result: e.Result, Error: e.Error, Body: e.Body, Pos: e.Pos}
	for i, p := range e.Params {
		switch {
		case p.TypeName != nil:
			fn.ParamTypes = append(fn.ParamTypes, c.resolve(p.TypeName))
		case want != nil && want.Params[i] != nil:
			fn.ParamTypes = append(fn.ParamTypes, want.Params[i])
		default:
			c.fail(p.Pos, "cannot infer the type of parameter %s; annotate it", p.Name)
		}
	}
	switch {
	case e.Result != nil || e.Error != nil:
		fn.RetType = c.resultOf(e.Result, e.Error)
	case want != nil && want.Ret != nil:
		fn.RetType = want.Ret
	case want == nil && !e.Inline:
		fn.RetType = types.Void
	}
	e.Func = fn
	if !c.abstract {
		c.file.Funcs = append(c.file.Funcs, fn)
	}
	next := c.funcContext
	next.closures = append(next.closures, e)
	next.loops, next.deferring = nil, false
	c.enter(next, func() { c.body(fn) })
	e.Type = c.signatureOf(fn, false)
}

func (c *Checker) closureName() string {
	c.counts[c.fn]++
	return fmt.Sprintf("%s..c%d", c.fn.Name, c.counts[c.fn])
}

func (c *Checker) funcValue(fn *ast.Func, want *types.Type, pos diag.Pos) (*ast.Func, *types.Type) {
	if fn.Variadic {
		c.fail(pos, "variadic function %s cannot be used as a value", fn.Name)
	}
	want = unwrap(want)
	if fn.TypeParams != nil {
		inst, abstract := c.genericFuncValue(fn, want, pos)
		if inst == nil {
			return fn, abstract
		}
		fn = inst
	}
	callback := fn.Extern == nil && want != nil && want.Kind == types.FuncKind && want.C
	if callback {
		c.callback(fn, pos)
	}
	if fn.Extern == nil && !callback {
		fn.AsValue = true
	}
	return fn, c.signatureOf(fn, fn.Extern != nil || callback)
}

func (c *Checker) callback(fn *ast.Func, pos diag.Pos) {
	for i, t := range fn.ParamTypes {
		if fn.Params[i].Mut || !t.CScalar() {
			c.fail(pos, "%s cannot be a C callback: parameter %s must be a number, bool, ptr or extern fn", fn.Name, fn.Params[i].Name)
		}
	}
	if fn.RetType != types.Void && !fn.RetType.CScalar() {
		c.fail(pos, "%s cannot be a C callback: it must return a number, bool, ptr or extern fn", fn.Name)
	}
	fn.Callback = true
}

func (c *Checker) inferredResult(t *types.Type) *types.Type {
	if t == types.Never {
		return types.Void
	}
	return t
}

func (c *Checker) requireResult(pos diag.Pos, what string) {
	if c.fn.RetType == nil {
		c.fail(pos, "%s in a closure requires its result to be written", what)
	}
}

func (c *Checker) okType() *types.Type {
	if c.fn.RetType == nil {
		return nil
	}
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
	l := &ast.Local{Name: name, Type: t, Mutable: mutable, Owner: c.fn}
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

func (c *Checker) lookup(name string, pos diag.Pos) *ast.Local {
	l := c.find(name)
	if l == nil {
		c.fail(pos, "undefined: %s", name)
	}
	c.capture(l, pos)
	return l
}

func (c *Checker) capture(l *ast.Local, pos diag.Pos) {
	for i := len(c.closures) - 1; i >= 0 && c.closures[i].Func != l.Owner; i-- {
		if l.Ref {
			c.fail(pos, "a closure cannot capture mut parameter %s", l.Name)
		}
		if c.droppable(l.Type) {
			c.fail(pos, "a closure cannot capture %s; %s has drop", l.Name, l.Type)
		}
		l.Captured = true
		if closure := c.closures[i]; !slices.Contains(closure.Captures, l) {
			closure.Captures = append(closure.Captures, l)
		}
	}
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
		var target *loop
		target, s.Depth = c.target(s.Pos, "break", s.Label)
		target.broken = true
		return true
	case *ast.Continue:
		c.leaving(s.Pos, "continue")
		_, s.Depth = c.target(s.Pos, "continue", s.Label)
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
	case *ast.Spawn:
		c.spawn(s)
	case *ast.Send:
		c.send(s)
	}
	return false
}

func (c *Checker) deferStmt(s *ast.Defer) {
	if c.deferring {
		c.fail(s.Pos, "defer cannot be nested in a deferred statement")
	}
	next := c.funcContext
	next.loops, next.deferring = nil, true
	c.enter(next, func() {
		c.pushScope()
		if c.stmt(s.Body) {
			c.fail(s.Pos, "a deferred statement cannot leave its scope")
		}
	})
}

func (c *Checker) leaving(pos diag.Pos, keyword string) {
	if c.deferring {
		c.fail(pos, "%s cannot appear in a deferred statement", keyword)
	}
}

func (c *Checker) destructureLet(s *ast.Let) {
	t := c.plain(s.Value, nil)
	temp := &ast.Local{Type: t, Owner: c.fn}
	c.destructure(s.Pattern, t, temp, s.Mutable)
}

func (c *Checker) destructure(p *ast.StructPattern, t *types.Type, temp *ast.Local, mutable bool) {
	if t.Kind != types.StructKind {
		c.fail(p.Pos, "cannot destructure %s; it is not a struct", t)
	}
	if c.droppable(t) {
		c.fail(p.Pos, "cannot destructure %s; it has drop", t)
	}
	p.Type, p.Temp = t, temp
	root := &ast.Ident{Name: temp.Name, Local: temp}
	root.Type = t
	c.bindFields(p, root, mutable, map[string]bool{})
}

func (c *Checker) bindFields(p *ast.StructPattern, base ast.Expr, mutable bool, bound map[string]bool) {
	listed := map[string]bool{}
	for _, f := range p.Fields {
		field := p.Type.Field(f.Name)
		if field == nil {
			c.fail(f.Pos, "%s has no field %s", p.Type, f.Name)
		}
		if listed[f.Name] {
			c.fail(f.Pos, "field %s listed twice", f.Name)
		}
		listed[f.Name] = true
		c.reachable(field, p.Type, f.Pos)
		source := &ast.Selector{X: base, Name: f.Name, Pos: f.Pos, Field: field}
		source.Type = field.Type
		f.Field, f.Source = field, source
		if f.Nested != nil {
			c.nest(f, field.Type)
			c.bindFields(f.Nested, source, mutable, bound)
			continue
		}
		if bound[f.Bind.Name] {
			c.fail(f.Bind.Pos, "%s is bound twice in this pattern", f.Bind.Name)
		}
		bound[f.Bind.Name] = true
		f.Bind.Local = c.declare(f.Bind.Name, field.Type, mutable)
	}
}

func (c *Checker) nest(f *ast.FieldPattern, t *types.Type) {
	if t.Kind != types.StructKind {
		c.fail(f.Nested.Pos, "cannot destructure field %s of type %s; it is not a struct", f.Name, t)
	}
	f.Nested.Type = t
}

func (c *Checker) let(s *ast.Let) {
	if s.Pattern != nil {
		c.destructureLet(s)
		return
	}
	t := c.bind(s)
	if s.Using {
		c.usingBinding(s, t)
		return
	}
	if c.droppable(t) {
		c.fail(s.Pos, "%s has drop; bind it with using", t)
	}
}

func (c *Checker) bind(s *ast.Let) *types.Type {
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
	return t
}

func (c *Checker) assign(s *ast.Assign) {
	t := c.assignTarget(s)
	op := s.Op.Operator()
	if op != lexer.Assign {
		c.requireOperator(op, t, s.Pos)
	}
	switch {
	case op == lexer.Shl || op == lexer.Shr:
		c.shiftCount(op, s.Value)
	case op != lexer.Assign && isVector(t):
		c.vectorSplatOperand(s, t)
	default:
		value := c.plain(s.Value, t)
		if !t.Accepts(value) {
			c.fail(s.Value.Position(), "cannot assign %s to a place of type %s", value, t)
		}
		c.droppableAssign(s, t)
	}
}

func (c *Checker) requireOperator(op lexer.Kind, t *types.Type, pos diag.Pos) {
	switch {
	case isVector(t):
		c.vectorOperator(op, t, pos)
	case op == lexer.Plus && t == types.Str:
	case op == lexer.Plus || op == lexer.Minus || op == lexer.Star || op == lexer.Slash:
		c.requireNumber(t, op, pos)
	default:
		c.requireInteger(t, op, pos)
	}
}

func (c *Checker) shiftCount(op lexer.Kind, count ast.Expr) {
	c.requireInteger(c.plain(count, nil), op, count.Position())
}

func (c *Checker) assignTarget(s *ast.Assign) *types.Type {
	if entry, ok := s.Target.(*ast.Index); ok {
		if m := c.plain(entry.X, nil); m.Kind == types.MapKind {
			c.expectType(entry.Index, m.Key)
			entry.Type = c.universe.Optional(m.Elem)
			s.Entry = entry
			return m.Elem
		}
	}
	_, t := c.place(s.Target, s.Pos, "assign to %s", "cannot assign to this expression")
	return t
}

func (c *Checker) requireNumber(t *types.Type, op lexer.Kind, pos diag.Pos) {
	if !t.Number() && t != types.Never {
		c.fail(pos, "operator %s requires a number, found %s", op, t)
	}
}

func (c *Checker) requireInteger(t *types.Type, op lexer.Kind, pos diag.Pos) {
	if !t.Integer() && t != types.Never {
		c.fail(pos, "operator %s requires an integer, found %s", op, t)
	}
}

func (c *Checker) mutable(root *ast.Local, pos diag.Pos, action string) {
	switch {
	case root.Param && !root.Ref:
		c.fail(pos, "cannot %s; declare the parameter mut", fmt.Sprintf(action, root.Name))
	case !root.Mutable:
		c.fail(pos, "cannot %s, declared with let", fmt.Sprintf(action, root.Name))
	}
}

func (c *Checker) place(e ast.Expr, pos diag.Pos, action, invalid string) (root, *types.Type) {
	t := c.plain(e, nil)
	where, ok := rootOf(e)
	if k := ast.ConstOf(e); k != nil {
		c.fail(pos, "cannot %s, declared with const", fmt.Sprintf(action, k.Name))
	}
	if !ok {
		c.fail(pos, "%s", invalid)
	}
	if where.local != nil {
		c.mutable(where.local, pos, action)
		where.local.Assigned = true
	}
	return where, t
}

func (c *Checker) ref(r *ast.Ref, want *types.Type) {
	if t := c.borrow(r); t != want {
		c.fail(r.Pos, "expected %s, found %s", want, t)
	}
}

func (c *Checker) borrow(r *ast.Ref) *types.Type {
	t := c.borrowInPlace(r)
	c.address(r)
	return t
}

func (c *Checker) address(r *ast.Ref) {
	if where, _ := rootOf(r.X); where.local != nil && !where.local.Ref {
		where.local.Addressed = true
	}
}

func (c *Checker) mutOf(e *ast.Call, i int, want *types.Type) *ast.Ref {
	r := e.Args[i].(*ast.Ref)
	if r.Type != want {
		c.fail(r.Pos, "expected %s, found %s", want, r.Type)
	}
	return r
}

func (c *Checker) mutSlice(b *Builtin, e *ast.Call) *types.Type {
	r := e.Args[0].(*ast.Ref)
	if r.Type.Kind != types.SliceKind {
		c.fail(r.Pos, "%s needs a slice, found %s", b.Name, r.Type)
	}
	return r.Type
}

func (c *Checker) borrowInPlace(r *ast.Ref) *types.Type {
	_, t := c.place(r.X, r.Pos, "pass %s as mut", "mut argument must be a variable, a field, an element or a dereferenced pointer")
	r.Type = t
	return t
}

func (c *Checker) exclusive(args []ast.Expr) {
	var held []borrowed
	for _, arg := range args {
		r, ok := arg.(*ast.Ref)
		if !ok {
			continue
		}
		where, _ := rootOf(r.X)
		current := borrowed{root: where, bytes: 8 * r.Type.Size()}
		for _, other := range held {
			if current.overlaps(other) {
				c.fail(r.Pos, "%s is already passed as mut in this call", where.local.Name)
			}
		}
		held = append(held, current)
	}
}

func (c *Checker) forStmt(s *ast.For) bool {
	if s.Cond != nil {
		c.expectType(s.Cond, types.Bool)
	}
	l := c.enterLoop(s.Label, s.Pos)
	c.block(s.Body, nil)
	c.exitLoop(l, s.Pos)
	return s.Cond == nil && !l.broken
}

func (c *Checker) forIn(s *ast.ForIn) {
	key, value := c.iteration(s)
	c.pushScope()
	if s.Key != "" {
		s.KeyLocal = c.declare(s.Key, key, false)
	}
	if s.Pattern != nil {
		s.ValueLocal = &ast.Local{Type: value, Owner: c.fn}
		c.destructure(s.Pattern, value, s.ValueLocal, false)
	} else {
		s.ValueLocal = c.declare(s.Value, value, false)
	}
	l := c.enterLoop(s.Label, s.Pos)
	c.block(s.Body, nil)
	c.exitLoop(l, s.Pos)
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
	case types.ChanKind:
		if s.Key != "" {
			c.fail(s.Pos, "a channel loop binds one name")
		}
		return nil, t.Elem
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

func (c *Checker) enterLoop(label string, pos diag.Pos) *loop {
	for _, outer := range c.loops {
		if label != "" && outer.label == label {
			c.fail(pos, "label %s is already used by an enclosing loop", label)
		}
	}
	l := &loop{label: label}
	c.loops = append(c.loops, l)
	return l
}

func (c *Checker) exitLoop(l *loop, pos diag.Pos) {
	c.loops = c.loops[:len(c.loops)-1]
	if l.label != "" && !l.used {
		c.fail(pos, "label %s is not used", l.label)
	}
}

func (c *Checker) target(pos diag.Pos, keyword, label string) (*loop, int) {
	if len(c.loops) == 0 && label == "" {
		c.fail(pos, "%s outside of a loop", keyword)
	}
	for depth := range len(c.loops) {
		l := c.loops[len(c.loops)-1-depth]
		if l.label == label || label == "" {
			l.used = l.used || label != ""
			return l, depth
		}
	}
	c.fail(pos, "%s %s: no enclosing loop is labelled %s", keyword, label, label)
	return nil, 0
}

func (c *Checker) ret(s *ast.Return) {
	if c.fn.RetType == nil {
		c.fn.RetType = types.Void
		if s.Value != nil {
			c.fn.RetType = c.inferredResult(c.plain(s.Value, nil))
		}
		return
	}
	if s.Value == nil {
		if c.okType() != types.Void {
			c.fail(s.Pos, "missing return value of type %s", c.okType())
		}
		return
	}
	t := c.expr(s.Value, c.okType())
	if c.okType() == types.Void && t != c.fn.RetType && t != types.Never {
		c.fail(s.Value.Position(), "%s does not return a value", c.subject())
	}
	if !c.returnable(t) {
		c.fail(s.Value.Position(), "expected %s, found %s", c.okType(), t)
	}
}

func (c *Checker) failStmt(s *ast.Fail) {
	c.requireResult(s.Pos, "fail")
	ret := c.fn.RetType
	if ret.Kind != types.ResultKind {
		c.fail(s.Pos, "fail requires %s to declare an error type", c.subject())
	}
	c.expectType(s.Value, ret.Err)
}

func (c *Checker) handled(e ast.Expr, t *types.Type) {
	if t.Kind == types.ResultKind {
		c.fail(e.Position(), "unhandled error %s: use ?, catch or or", t.Err)
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
		if e.Wide && e.Type != types.U64 {
			c.fail(e.Pos, "%d overflows %s", uint64(e.Value), e.Type)
		}
		if !e.Wide && e.Type.Integer() && !e.Type.Holds(e.Value) {
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
		c.selector(e, want)
	case *ast.Closure:
		c.closure(e, want)
	case *ast.StructLit:
		c.structLit(e, want)
	case *ast.Instance:
		c.fail(e.Pos, "type arguments must be followed by a call")
	case *ast.Ref:
		c.fail(e.Pos, "mut is only allowed as an argument to a mut parameter")
	case *ast.Receiver:
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
	case *ast.Select:
		c.selectExpr(e, want)
	case *ast.Subject:
		if len(c.subjects) == 0 {
			c.fail(e.Pos, "the match subject is only available in a match arm")
		}
		e.Type = c.subjects[len(c.subjects)-1]
	case *ast.Try:
		c.try(e, want)
	case *ast.Catch:
		c.catch(e, want)
	case *ast.Or:
		c.or(e, want)
	case *ast.When:
		c.whenExpr(e, want)
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
	if b := c.builtinNamed(e.Name); b != nil && b.constant != nil {
		b.constant(c, e, want)
		return
	}
	if k, ok := c.local().consts[e.Name]; ok && c.find(e.Name) == nil {
		e.Const, e.Type = k, c.folded(k).TypeOf()
		return
	}
	if fn, ok := c.localFunc(e.Name); ok && c.find(e.Name) == nil {
		e.Func, e.Type = c.funcValue(fn, want, e.Pos)
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
		if bound := c.bound[x.Name]; bound != nil {
			t = bound
		}
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

func (c *Checker) field(t *types.Type, name string, pos diag.Pos) *types.Field {
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

func (c *Checker) reachable(f *types.Field, t *types.Type, pos diag.Pos) {
	if !f.Pub && t.Module != c.file.Module && (c.fn == nil || !c.fn.Trusted) {
		c.fail(pos, "field %s of %s is private", f.Name, t)
	}
}

func (c *Checker) selector(e *ast.Selector, want *types.Type) {
	if id, ok := e.X.(*ast.Ident); ok && c.isModule(id.Name) && c.typeRef(e.X) == nil {
		module := c.imported(id.Name, id.Pos)
		if k := module.consts[e.Name]; k != nil {
			e.Const = c.importedConst(id.Name, k, e.Pos)
			e.Type = c.folded(k).TypeOf()
			return
		}
		if g := module.generics[e.Name]; g == nil {
			e.Func, e.Type = c.funcValue(c.importedFunc(id.Name, e.Name, e.Pos), want, e.Pos)
			return
		}
	}
	if g := c.genericRef(e.X); g != nil {
		e.Type, e.Variant, _ = c.genericEnum(g, e.Name, nil, want, e.Pos)
		c.unitVariant(e.Variant, e.Pos)
		return
	}

	if t := c.typeRef(e.X); t != nil {
		if t.Kind == types.StructKind {
			c.fieldReference(e, t)
			return
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
	if b := LookupBuiltin(e.Name); b != nil && b.Kind == PropertyBuiltin && b.Receives(t.Kind) {
		if !b.Reads(t) {
			c.fail(e.Pos, "%s declares no variant indexes", t)
		}
		e.Property = b.ID
		e.Type = b.Type
		return
	}
	if types.StructOf(t).Field(e.Name) == nil {
		c.shadowedModule(e.X, e.Name)
	}
	e.Field = c.field(types.StructOf(t), e.Name, e.Pos)
	e.Type = e.Field.Type
}

func (c *Checker) fieldReference(e *ast.Selector, t *types.Type) {
	f := c.field(t, e.Name, e.Pos)
	e.Reference = f
	e.Type = c.serdeFieldType(t, f, e.Pos)
}

func IndexType(u *types.Universe, t *types.Type) *types.Type {
	switch {
	case t.Kind == types.MapKind:
		return u.Optional(t.Elem)
	case t.Kind == types.SliceKind:
		return t.Elem
	case t == types.Str:
		return types.U8
	}
	return nil
}

func (c *Checker) index(e *ast.Index) {
	t := c.plain(e.X, nil)
	e.Type = IndexType(c.universe, t)
	switch {
	case e.Type == nil:
		c.fail(e.Pos, "cannot index a value of type %s", t)
	case t.Kind == types.MapKind:
		c.expectType(e.Index, t.Key)
	default:
		c.integer(e.Index)
	}
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
	c.contain(elem, "a slice", e.Pos)
	e.Type = c.universe.Slice(elem)
}

func (c *Checker) mapLit(e *ast.MapLit, want *types.Type) {
	if want != nil && want.Kind == types.MapKind {
		e.Type = want
	} else if len(e.Keys) == 0 {
		c.fail(e.Pos, "cannot infer the type of an empty map; annotate it")
	} else {
		key := c.mapKey(c.plain(e.Keys[0], nil), e.Keys[0].Position())
		e.Type = c.universe.Map(key, c.plain(e.Values[0], nil))
	}
	for i := range e.Keys {
		c.expectType(e.Keys[i], e.Type.Key)
		c.expectType(e.Values[i], e.Type.Elem)
	}
}

func (c *Checker) interp(e *ast.Interp) {
	e.Renders = make([]*ast.Func, len(e.Parts))
	for i, part := range e.Parts {
		t := c.plain(part, nil)
		if spec := e.Specs[i]; spec != nil && spec.Format != nil {
			c.requireFormattable(spec.Format, t)
		}
		if t == types.Str {
			continue
		}
		c.requireValue(part.Position(), "interpolation", t)
		e.Renders[i] = c.appender(t, part.Position())
	}
	e.Type = types.Str
}

func (c *Checker) requireFormattable(f *ast.Format, t *types.Type) {
	if t == types.Never || f.IntegerOnly() && t.Integer() || !f.IntegerOnly() && t.Number() {
		return
	}
	if t.Kind != types.ParamKind {
		c.fail(f.Pos, "cannot format a value of type %s with %q", t, f.Text)
	}
	constraint := "Number"
	if f.IntegerOnly() {
		constraint = "Integer"
	}
	c.fail(f.Pos, "cannot format a value of type %s with %q; declare %s: %s", t, f.Text, t, constraint)
}

func (c *Checker) structLit(e *ast.StructLit, want *types.Type) {
	var t *types.Type
	checked := map[*ast.FieldInit]*types.Type{}
	switch g := c.lookupGeneric(e.Module, e.Name, e.Pos); {
	case g != nil:
		t = c.genericLitType(e, g, want, checked)
	case e.TypeArgs != nil:
		c.fail(e.Pos, "%s is not generic", e.Name)
	case e.Module != "":
		t = c.importedType(e.Module, e.Name, e.Pos)
	case c.bound[e.Name] != nil:
		t = c.bound[e.Name]
	default:
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
		if found, ok := checked[init]; ok {
			if !f.Type.Accepts(found) {
				c.fail(init.Value.Position(), "expected %s, found %s", f.Type, found)
			}
			continue
		}
		c.expectType(init.Value, f.Type)
	}
	var missing []string
	for _, f := range t.Fields {
		if given[f] {
			continue
		}
		if def := c.defaultOf(f); def != nil {
			e.Fields = append(e.Fields, &ast.FieldInit{Name: f.Name, Value: def, Pos: e.Pos, Form: ast.FieldImplied, Field: f})
			continue
		}
		missing = append(missing, f.Name)
	}
	if len(missing) > 0 {
		c.fail(e.Pos, "missing fields for %s: %s", t, strings.Join(missing, ", "))
	}
	e.Type = t
}

func (c *Checker) variant(enum *types.Type, name string, pos diag.Pos) *types.Variant {
	v := enum.Variant(name)
	if v == nil {
		c.fail(pos, "%s has no variant %s", enum, name)
	}
	return v
}

func (c *Checker) unitVariant(v *types.Variant, pos diag.Pos) {
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
	case lexer.LArrow:
		e.Type = c.universe.Optional(c.channel(e.X).Elem)
	case lexer.Tilde:
		e.Type = c.plain(e.X, arithmetic(want))
		if isVector(e.Type) {
			c.vectorUnary(e, e.Type)
			return
		}
		c.requireInteger(e.Type, e.Op, e.Pos)
	default:
		e.Type = c.plain(e.X, arithmetic(want))
		if isVector(e.Type) {
			c.vectorUnary(e, e.Type)
			return
		}
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
	var s *types.Type
	if isVector(t) && literal(second) {
		s = c.plain(second, t.Elem)
	} else {
		s = c.plain(second, t)
	}
	switch {
	case isVector(t) && isVector(s):
		if s != t {
			c.fail(second.Position(), "expected %s, found %s", t, s)
		}
	case isVector(t):
		if !t.Elem.Accepts(s) {
			c.fail(second.Position(), "expected %s or %s, found %s", t, t.Elem, s)
		}
		c.splat(e, second)
	case isVector(s):
		if !s.Elem.Accepts(t) {
			c.fail(first.Position(), "expected %s or %s, found %s", s, s.Elem, t)
		}
		c.splat(e, first)
		t = s
	case !t.Accepts(s):
		c.fail(second.Position(), "expected %s, found %s", t, s)
	}
	return t
}

func (c *Checker) splat(e *ast.Binary, operand ast.Expr) {
	if operand == e.X {
		e.SplatX = true
	} else {
		e.SplatY = true
	}
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
		e.Type = c.plain(e.X, arithmetic(want))
		c.requireOperator(e.Op, e.Type, e.X.Position())
		c.shiftCount(e.Op, e.Y)
	case lexer.Eq, lexer.NotEq:
		t := c.operands(e, nil)
		if isVector(t) {
			e.Type = t
			return
		}
		if !t.Comparable() && t != types.Never {
			c.fail(e.X.Position(), "cannot compare values of type %s", t)
		}
		e.Type = types.Bool
	case lexer.Lt, lexer.LtEq, lexer.Gt, lexer.GtEq:
		t := c.operands(e, nil)
		if isVector(t) {
			e.Type = t
			return
		}
		if !t.Ordered() && t != types.Never {
			c.fail(e.Pos, "operator %s requires a number or str, found %s", e.Op, t)
		}
		e.Type = types.Bool
	default:
		e.Type = c.operands(e, arithmetic(want))
		c.requireOperator(e.Op, e.Type, e.Pos)
	}
}

func (c *Checker) call(e *ast.Call, want *types.Type) {
	if e.Prepended && c.firstMut(e.Fn) {
		prependMut(e)
	}
	switch fn := e.Fn.(type) {
	case *ast.Ident:
		if b := c.builtinNamed(fn.Name); b != nil && b.handler != nil {
			c.callBuiltin(b, e, want)
			return
		}
		if c.find(fn.Name) != nil {
			c.indirect(e)
			return
		}
		if t, ok := c.bound[fn.Name]; ok && t.Number() {
			c.convert(e, t)
			return
		}
		if enum, v := c.inferVariant(want, fn.Name); v != nil {
			c.construct(e, enum, v, nil)
			return
		}
		target, ok := c.localFunc(fn.Name)
		if !ok {
			c.fail(e.Pos, "undefined function: %s", fn.Name)
		}
		c.callFunc(e, target, want)
	case *ast.Selector:
		c.selectorCall(e, fn, want)
	case *ast.Instance:
		c.instanceCall(e, fn, want)
	default:
		c.indirect(e)
	}
}

func (c *Checker) selectorCall(e *ast.Call, sel *ast.Selector, want *types.Type) {
	id, isIdent := sel.X.(*ast.Ident)
	t := c.typeRef(sel.X)
	g := c.genericRef(sel.X)
	switch {
	case t != nil:
		c.variantCall(e, sel, t)
	case g != nil:
		enum, v, found := c.genericEnum(g, sel.Name, e.Args, want, sel.Pos)
		c.construct(e, enum, v, found)
	case isIdent && c.isModule(id.Name) && c.generatesRegex(id.Name, sel.Name):
		c.generatedRegex(e)
	case isIdent && c.isModule(id.Name):
		c.callFunc(e, c.importedFunc(id.Name, sel.Name, sel.Pos), want)
	case isIdent && c.find(id.Name) == nil:
		c.fail(id.Pos, "undefined: %s", id.Name)
	case e.Prepended:
		c.indirect(e)
	default:
		c.method(e, sel, want)
	}
}

func (c *Checker) method(e *ast.Call, sel *ast.Selector, want *types.Type) {
	t := c.plain(sel.X, nil)
	receiver := &ast.Receiver{Typed: ast.Typed{Type: t}, X: sel.X}
	if f := types.StructOf(t).Field(sel.Name); f != nil && f.Type.Kind == types.FuncKind {
		sel.X = receiver
		c.indirect(e)
		return
	}
	e.Fn = &ast.Ident{Name: sel.Name, Pos: sel.Pos}
	e.Args = append([]ast.Expr{receiver}, e.Args...)
	e.Prepended = true
	target := c.receiverFunc(t, sel.Name)
	if target == nil && !c.callable(sel.Name) {
		target = c.importedReceiverFunc(t, sel.Name, sel.Pos)
	}
	if target != nil {
		if len(target.Params) > 0 && target.Params[0].Mut {
			prependMut(e)
		}
		c.callFunc(e, target, want)
		return
	}
	if !c.callable(sel.Name) {
		c.shadowedModule(sel.X, sel.Name)
	}
	c.call(e, want)
}

func (c *Checker) shadowedModule(x ast.Expr, name string) {
	id, ok := x.(*ast.Ident)
	if !ok {
		return
	}
	if path, imported := c.file.Imported(id.Name); imported {
		c.fail(id.Pos, "%s is a local here, not the module %s; rename the local to use %s.%s", id.Name, path, id.Name, name)
	}
}

func ReceiverModule(t *types.Type, current string) (string, bool) {
	named := types.StructOf(t)
	if named.Origin != nil {
		named = named.Origin
	}
	return named.Module, named.Module != current
}

func (c *Checker) receiverFunc(t *types.Type, name string) *ast.Func {
	module, foreign := ReceiverModule(t, c.file.Module)
	if !foreign || c.callable(name) {
		return nil
	}
	fn := c.scope(module).funcs[name]
	if fn == nil || !fn.Pub {
		return nil
	}
	return fn
}

func (c *Checker) importedReceiverFunc(t *types.Type, name string, pos diag.Pos) *ast.Func {
	var found []*ast.Func
	var modules []string
	for _, imp := range c.file.Imports {
		fn := c.scope(imp.Path).funcs[name]
		if fn != nil && fn.Pub && TakesFirst(fn, t) {
			found = append(found, fn)
			modules = append(modules, fmt.Sprintf("%s.%s", imp.Alias, name))
		}
	}
	if len(found) > 1 {
		c.fail(pos, "%s is ambiguous for %s: %s; call one of them by its module", name, t, strings.Join(modules, ", "))
	}
	if len(found) == 0 {
		return nil
	}
	return found[0]
}

func prependMut(e *ast.Call) {
	if _, isRef := e.Args[0].(*ast.Ref); !isRef {
		e.Args[0] = &ast.Ref{X: e.Args[0], Pos: e.Args[0].Position()}
	}
}

func (c *Checker) indirect(e *ast.Call) {
	t := c.plain(e.Fn, nil)
	if t.Kind != types.FuncKind {
		c.fail(e.Pos, "cannot call a value of type %s", t)
	}
	if len(e.Args) != len(t.Params) {
		c.fail(e.Pos, "%s takes %d arguments, found %d", t, len(t.Params), len(e.Args))
	}
	if e.Prepended && t.Muts[0] {
		prependMut(e)
	}
	for i, arg := range e.Args {
		if r := c.mutArgument(arg, t.Muts[i], "%d of %s", i+1, t); r != nil {
			c.ref(r, t.Params[i])
		} else {
			c.expectType(arg, t.Params[i])
		}
	}
	c.exclusive(e.Args)
	e.Type = t.Ret
}

func (c *Checker) print(e *ast.Call) {
	t := c.plain(e.Args[0], nil)
	e.Render = nil
	if t != types.Str {
		c.requireValue(e.Args[0].Position(), "print", t)
		e.Render = c.appender(t, e.Pos)
	}
	e.Type = types.Void
}

func (c *Checker) embed(e *ast.Call) {
	lit, ok := e.Args[0].(*ast.StrLit)
	if !ok {
		c.fail(e.Args[0].Position(), "embed needs a string literal path")
	}
	lit.Type = types.Str
	embedded, err := c.embedder(c.file, lit.Value)
	if errors.Is(err, fs.ErrNotExist) {
		c.fail(lit.Pos, "cannot embed %s: no such file", lit.Value)
	}
	if err != nil {
		c.fail(lit.Pos, "cannot embed %s: %v", lit.Value, err)
	}
	c.embedded(e, embedded.Path, embedded.Data)
}

func (c *Checker) embedded(e *ast.Call, abs string, data []byte) {
	digest := fnv.New64a()
	digest.Write(data)
	e.Embed, e.Size, e.Digest, e.Type = abs, int64(len(data)), digest.Sum64(), types.Str
}

func (c *Checker) deleteEntry(e *ast.Call) {
	r := e.Args[0].(*ast.Ref)
	t := r.Type
	if t.Kind != types.MapKind {
		c.fail(r.Pos, "delete needs a map, found %s", t)
	}
	c.expectType(e.Args[1], t.Key)
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

func (c *Checker) push(b *Builtin, e *ast.Call) {
	t := c.mutSlice(b, e)
	c.expectType(e.Args[1], t.Elem)
	e.Type = types.Void
}

func (c *Checker) pop(b *Builtin, e *ast.Call) {
	t := c.mutSlice(b, e)
	e.Type = c.universe.Optional(t.Elem)
}

func (c *Checker) truncate(b *Builtin, e *ast.Call) {
	c.mutSlice(b, e)
	c.integer(e.Args[1])
	e.Type = types.Void
}

func (c *Checker) syscall(e *ast.Call) {
	for _, arg := range e.Args {
		if t := c.plain(arg, nil); !t.Integer() && t.Kind != types.RawKind && t != types.Bool {
			c.fail(arg.Position(), "__syscall arguments are integers, bool or ptr, found %s", t)
		}
	}
	e.Type = types.I64
}

func (c *Checker) math(b *Builtin, e *ast.Call, want *types.Type) {
	name := b.Name
	var arg *types.Type
	switch {
	case want != nil && want.Float():
		arg = want
	case numericLiterals(e.Args[0]):
		arg = types.F64
	}
	t := c.plain(e.Args[0], arg)
	if !t.Float() {
		c.fail(e.Args[0].Position(), "%s requires f32 or f64, found %s", name, t)
	}
	e.Type = t
}

func (c *Checker) convert(e *ast.Call, to *types.Type) {
	t := c.plain(e.Args[0], nil)
	if !convertible(t, to) {
		c.fail(e.Args[0].Position(), "cannot convert %s to %s", t, to)
	}
	e.Builtin, e.Type = builtin.Convert, to
}

func convertible(from, to *types.Type) bool {
	switch {
	case to.Kind == types.RawKind:
		return from == types.I64 || from == types.U64
	case from.Kind == types.RawKind:
		return to == types.I64 || to == types.U64
	}
	return from.Number()
}

func (c *Checker) variantCall(e *ast.Call, sel *ast.Selector, t *types.Type) {
	if t.Kind != types.EnumKind {
		c.fail(e.Pos, "cannot call this expression")
	}
	c.construct(e, t, c.variant(t, sel.Name, sel.Pos), nil)
}

func (c *Checker) allocate(e *ast.Call, want *types.Type) {
	var elem *types.Type
	if want != nil && want.Kind == types.PointerKind {
		elem = want.Elem
	}
	t := c.plain(e.Args[0], elem)
	if t == types.Void || t == types.Never {
		c.fail(e.Args[0].Position(), "cannot allocate a value of type %s", t)
	}
	c.contain(t, "a pointer", e.Pos)
	e.Type = c.universe.Pointer(t)
}

func (c *Checker) callFunc(e *ast.Call, fn *ast.Func, want *types.Type) {
	if fn.TypeParams != nil {
		c.genericCall(e, fn, nil, want)
		return
	}
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
	c.forwarded(e, fn)
}

func (c *Checker) argument(arg ast.Expr, fn *ast.Func, i int) {
	param := fn.Params[i]
	if r := c.mutArgument(arg, param.Mut, "%s of %s", param.Name, fn.Name); r != nil {
		c.ref(r, fn.ParamTypes[i])
	} else {
		c.expectType(arg, fn.ParamTypes[i])
	}
}

func (c *Checker) mutArgument(arg ast.Expr, mut bool, describe string, args ...any) *ast.Ref {
	r, isRef := arg.(*ast.Ref)
	if mut && !isRef {
		c.fail(arg.Position(), "parameter %s is mut; write mut before the argument", fmt.Sprintf(describe, args...))
	}
	if mut {
		return r
	}
	return nil
}

func (c *Checker) variadic(arg ast.Expr) {
	if _, ok := arg.(*ast.Ref); ok {
		c.fail(arg.Position(), "variadic arguments cannot be mut")
	}
	if t := c.plain(arg, nil); !t.CScalar() {
		c.fail(arg.Position(), "variadic arguments must be numbers, bool or ptr, found %s", t)
	}
}

func (c *Checker) construct(e *ast.Call, enum *types.Type, v *types.Variant, found []*types.Type) {
	if len(e.Args) != len(v.Fields) {
		c.fail(e.Pos, "variant %s takes %d fields, found %d", v.Name, len(v.Fields), len(e.Args))
	}
	for i, arg := range e.Args {
		if found == nil {
			c.expectType(arg, v.Fields[i])
			continue
		}
		if !v.Fields[i].Accepts(found[i]) {
			c.fail(arg.Position(), "expected %s, found %s", v.Fields[i], found[i])
		}
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
	if e.Else == nil {
		c.block(e.Then, want)
		e.Type = types.Void
		return
	}
	then := c.branch(e.Then, want, want)
	hint := want
	if hint == nil && then.Number() && bareNumber(e.Else) {
		hint = then
	}
	other := c.branch(e.Else, hint, want)
	if want == nil && bareNumber(e.Then) && other.Number() {
		then = c.block(e.Then, other)
	}
	e.Type = unify(then, other)
}

func (c *Checker) whenExpr(e *ast.When, want *types.Type) {
	c.expectType(e.Cond, types.Bool)
	t := c.expr(e.X, want)
	switch {
	case t == types.Void || t == types.Never || t.Kind == types.ResultKind:
		c.fail(e.X.Position(), "when needs a value to wrap, found %s", t)
	case want != nil && want.Optional && want.Accepts(t):
		e.Type = want
	case t.Optional:
		e.Type = t
	default:
		e.Type = c.universe.Optional(t)
	}
}

func (c *Checker) branch(e ast.Expr, hint, want *types.Type) *types.Type {
	t := c.expr(e, hint)
	if want != nil && want != types.Void && t != types.Never && t.Kind != types.ResultKind && !want.Accepts(t) {
		c.fail(produced(e).Position(), "expected %s, found %s", want, t)
	}
	return t
}

func bareNumber(e ast.Expr) bool {
	block, ok := e.(*ast.Block)
	if !ok || len(block.Stmts) > 0 {
		return false
	}
	switch block.Tail.(type) {
	case *ast.IntLit, *ast.FloatLit:
		return true
	}
	return false
}

func numericLiterals(e ast.Expr) bool {
	switch e := e.(type) {
	case *ast.IntLit, *ast.FloatLit:
		return true
	case *ast.Unary:
		return numericLiterals(e.X)
	case *ast.Binary:
		return numericLiterals(e.X) && numericLiterals(e.Y)
	case *ast.Block:
		return len(e.Stmts) == 0 && e.Tail != nil && numericLiterals(e.Tail)
	case *ast.If:
		return e.Else != nil && numericLiterals(e.Then) && numericLiterals(e.Else)
	}
	return false
}

func produced(e ast.Expr) ast.Expr {
	if block, ok := e.(*ast.Block); ok && block.Tail != nil {
		return produced(block.Tail)
	}
	return e
}

func (c *Checker) match(e *ast.Match, want *types.Type) {
	t := c.plain(e.X, nil)
	if t == types.Void || t == types.Never {
		c.fail(e.X.Position(), "cannot match on a value of type %s", t)
	}
	e.Type = c.arms(e.Arms, t, want, e.Pos)
}

func (c *Checker) arms(arms []*ast.Arm, enum, want *types.Type, pos diag.Pos) *types.Type {
	result := types.Never
	covered := map[*types.Variant]bool{}
	wildcard, tested := false, false
	for _, arm := range arms {
		if wildcard {
			c.fail(arm.Pattern.Pos, "unreachable arm; an earlier arm matches everything")
		}
		c.pushScope()
		c.pattern(arm.Pattern, enum)
		result = unify(result, c.branch(arm.Body, want, want))
		c.popScope()
		if arm.Pattern.Test != nil {
			tested = true
			continue
		}
		if arm.Pattern.Variant == nil {
			wildcard = true
			continue
		}
		if covered[arm.Pattern.Variant] {
			c.fail(arm.Pattern.Pos, "duplicate arm for %s", arm.Pattern.Name)
		}
		covered[arm.Pattern.Variant] = true
	}
	switch {
	case wildcard:
	case tested || enum.Kind != types.EnumKind:
		c.fail(pos, "match on %s needs a final _ arm", enum)
	default:
		c.exhaustive(enum, covered, pos)
	}
	return result
}

func (c *Checker) exhaustive(enum *types.Type, covered map[*types.Variant]bool, pos diag.Pos) {
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
	if p.Test != nil {
		c.subjects = append(c.subjects, enum)
		c.expectType(p.Test, types.Bool)
		c.subjects = c.subjects[:len(c.subjects)-1]
		return
	}
	if c.constArm(p, enum) {
		c.pattern(p, enum)
		return
	}
	if binds(p.Name) && (enum.Kind != types.EnumKind || enum.Variant(p.Name) == nil) {
		if len(p.Bindings) > 0 {
			c.fail(p.Pos, "%s has no variant %s", enum, p.Name)
		}
		p.Local = c.declare(p.Name, enum, false)
		return
	}
	if p.Name != "_" && enum.Kind != types.EnumKind {
		c.fail(p.Pos, "an arm matching %s starts with a literal or an operator, such as 4, == or <", enum)
	}
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

func binds(name string) bool {
	return name[0] >= 'a' && name[0] <= 'z'
}

func (c *Checker) errorResult(e ast.Expr, keyword string, pos diag.Pos, want *types.Type) *types.Type {
	t := c.expr(e, want)
	if t.Kind != types.ResultKind {
		c.fail(pos, "%s requires an error result, found %s", keyword, t)
	}
	return t
}

func (c *Checker) try(e *ast.Try, want *types.Type) {
	c.leaving(e.Pos, "?")
	c.requireResult(e.Pos, "?")
	t := c.errorResult(e.X, "?", e.Pos, want)
	ret := c.fn.RetType
	if ret.Kind != types.ResultKind {
		c.unpropagated = t
		c.fail(e.Pos, "? requires %s to declare an error type", c.subject())
	}
	if ret.Err != t.Err {
		c.fail(e.Pos, "cannot propagate %s from function with error type %s", t.Err, ret.Err)
	}
	e.Type = t.Ok
}

func (c *Checker) catch(e *ast.Catch, want *types.Type) {
	t := c.errorResult(e.X, "catch", e.Pos, want)
	if e.Map != nil {
		c.mapError(e, t)
		return
	}
	e.Type = handledType(t.Ok, want)
	arms := c.arms(e.Arms, t.Err, e.Type, e.Pos)
	if t.Ok != types.Void && !e.Type.Accepts(arms) {
		c.fail(e.Pos, "catch arms must produce %s, found %s", e.Type, arms)
	}
}

func (c *Checker) mapError(e *ast.Catch, t *types.Type) {
	c.leaving(e.Pos, "catch")
	e.Type = t.Ok
	ret := c.fn.RetType
	if ret.Kind != types.ResultKind {
		c.fail(e.Pos, "catch with a mapper requires %s to declare an error type", c.subject())
	}
	c.subjects = append(c.subjects, t.Err)
	c.expectType(e.Map, ret.Err)
	c.subjects = c.subjects[:len(c.subjects)-1]
}

func (c *Checker) or(e *ast.Or, want *types.Type) {
	t := c.errorResult(e.X, "or", e.Pos, want)
	e.Type = handledType(t.Ok, want)
	fallback := c.branch(e.Fallback, e.Type, e.Type)
	c.handled(e.Fallback, fallback)
}

func handledType(ok, want *types.Type) *types.Type {
	if want != nil && want.Optional && want.Elem == ok {
		return want
	}
	return ok
}
