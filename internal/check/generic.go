package check

import (
	"fmt"
	"strings"

	"lowlang/internal/ast"
	"lowlang/internal/lexer"
	"lowlang/internal/types"
)

const instanceLimit = 1000

type generic struct {
	name      string
	decls     []*ast.TypeParam
	params    []*types.Type
	pub       bool
	pos       lexer.Pos
	file      *ast.File
	strct     *ast.StructDecl
	enum      *ast.EnumDecl
	instances map[string]*types.Type
	abstract  *types.Type
}

type inference struct {
	name   string
	decls  []*ast.TypeParam
	params []*types.Type
	bound  map[*types.Type]*types.Type
}

var mapFuncs = []string{"__map_new", "__map_clone", "__map_len", "__map_get", "__map_set", "__map_delete", "__map_next", "__map_key", "__map_value"}

func plural(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

func typeKey(args []*types.Type) string {
	parts := make([]string, len(args))
	for i, a := range args {
		parts[i] = fmt.Sprintf("%p", a)
	}
	return strings.Join(parts, ",")
}

func (c *Checker) typeParams(decls []*ast.TypeParam) []*types.Type {
	params := make([]*types.Type, len(decls))
	seen := map[string]bool{}
	for i, d := range decls {
		if seen[d.Name] {
			c.fail(d.Pos, "type parameter %s already declared", d.Name)
		}
		seen[d.Name] = true
		var constraint types.Constraint
		for _, name := range d.Constraints {
			bits, ok := types.Constraints[name]
			if !ok {
				c.fail(d.Pos, "unknown constraint %s; use Eq, Ord, Hash, Number, Integer or Float", name)
			}
			constraint |= bits
		}
		params[i] = types.NewParam(d.Name, constraint)
	}
	return params
}

func bindings(decls []*ast.TypeParam, args []*types.Type) map[string]*types.Type {
	bound := map[string]*types.Type{}
	for i, d := range decls {
		bound[d.Name] = args[i]
	}
	return bound
}

func (c *Checker) bindingsOf(fn *ast.Func) map[string]*types.Type {
	if fn.Bound != nil {
		return fn.Bound
	}
	decl := fn
	if fn.Template != nil {
		decl = fn.Template
	}
	return bindings(decl.TypeParams, fn.TypeArgs)
}

func (c *Checker) within(file *ast.File, bound map[string]*types.Type, run func()) {
	savedFile, savedBound := c.file, c.bound
	c.file, c.bound = file, bound
	run()
	c.file, c.bound = savedFile, savedBound
}

func (c *Checker) satisfy(args []*types.Type, decls []*ast.TypeParam, params []*types.Type, pos lexer.Pos) {
	for i, a := range args {
		if !a.Satisfies(params[i].Constraint) {
			c.fail(pos, "%s does not satisfy %s", a, strings.Join(decls[i].Constraints, " + "))
		}
	}
}

func (c *Checker) declareGeneric(name string, decls []*ast.TypeParam, pub bool, pos lexer.Pos) *generic {
	_, builtin := c.builtin[name]
	_, dupType := c.local().types[name]
	if _, dup := c.local().generics[name]; dup || dupType || builtin {
		c.fail(pos, "type %s already declared", name)
	}
	g := &generic{name: name, decls: decls, params: c.typeParams(decls), pub: pub, pos: pos, file: c.file, instances: map[string]*types.Type{}}
	c.local().generics[name] = g
	return g
}

func (c *Checker) origin(g *generic) *types.Type {
	if g.abstract == nil {
		c.instantiateType(g, g.params, g.pos)
	}
	return g.abstract
}

func (c *Checker) instantiateType(g *generic, args []*types.Type, pos lexer.Pos) *types.Type {
	if len(args) != len(g.params) {
		c.fail(pos, "%s takes %s, found %d", g.name, plural(len(g.params), "type argument"), len(args))
	}
	c.satisfy(args, g.decls, g.params, pos)
	key := typeKey(args)
	if t, ok := g.instances[key]; ok {
		return t
	}
	if len(g.instances) >= instanceLimit {
		c.fail(pos, "too many instances of %s", g.name)
	}
	name := types.InstanceName(qualified(g.file.Module, g.name), args)
	t := types.NewStruct(name)
	if g.enum != nil {
		t = types.NewEnum(name)
	}
	t.Module, t.Pub, t.TypeArgs = g.file.Module, g.pub, args
	g.instances[key] = t
	if g.abstract == nil && typeKey(g.params) == key {
		g.abstract = t
		c.generics[t] = g
	}
	t.Origin = c.origin(g)
	c.within(g.file, bindings(g.decls, args), func() {
		if g.strct != nil {
			c.declareFields(g.strct, t)
		} else {
			c.declareVariants(g.enum, t)
		}
	})
	if c.laid {
		c.layout(t, pos)
	} else {
		c.unlaid = append(c.unlaid, t)
	}
	return t
}

func (c *Checker) lookupGeneric(module, name string, pos lexer.Pos) *generic {
	if module == "" {
		return c.local().generics[name]
	}
	g := c.imported(module, pos).generics[name]
	if g != nil && !g.pub {
		c.fail(pos, "%s.%s is private", module, name)
	}
	return g
}

func (c *Checker) genericRef(x ast.Expr) *generic {
	switch x := x.(type) {
	case *ast.Ident:
		if c.find(x.Name) != nil {
			return nil
		}
		return c.local().generics[x.Name]
	case *ast.Selector:
		id, ok := x.X.(*ast.Ident)
		if !ok || !c.isModule(id.Name) {
			return nil
		}
		return c.lookupGeneric(id.Name, x.Name, x.Pos)
	}
	return nil
}

func (c *Checker) declareGenericFunc(fn *ast.Func) {
	c.pristine[fn] = ast.CloneFunc(fn)
	fn.TypeArgs = c.typeParams(fn.TypeParams)
	c.within(c.file, c.bindingsOf(fn), func() { c.signature(fn) })
}

func (c *Checker) instance(fn *ast.Func, args []*types.Type, pos lexer.Pos) *ast.Func {
	key := typeKey(args)
	if inst, ok := c.instances[fn][key]; ok {
		return inst
	}
	if c.instances[fn] == nil {
		c.instances[fn] = map[string]*ast.Func{}
	}
	if len(c.instances[fn]) >= instanceLimit {
		c.fail(pos, "too many instances of %s", fn.Name)
	}
	inst := ast.CloneFunc(c.pristine[fn])
	inst.Name = types.InstanceName(fn.Name, args)
	inst.TypeParams = nil
	inst.Template, inst.TypeArgs = fn, args
	c.instances[fn][key] = inst
	c.within(fn.File, c.bindingsOf(inst), func() { c.signature(inst) })
	fn.File.Funcs = append(fn.File.Funcs, inst)
	c.pending = append(c.pending, inst)
	return inst
}

func (c *Checker) infer(name string, decls []*ast.TypeParam, params []*types.Type) *inference {
	return &inference{name: name, decls: decls, params: params, bound: map[*types.Type]*types.Type{}}
}

func (in *inference) param(t *types.Type) bool {
	for _, p := range in.params {
		if p == t {
			return true
		}
	}
	return false
}

func (in *inference) unify(pattern, actual *types.Type) {
	if pattern == nil || actual == nil || actual == types.Never || !pattern.Abstract() {
		return
	}
	if in.param(pattern) {
		if _, ok := in.bound[pattern]; !ok {
			in.bound[pattern] = actual
		}
		return
	}
	switch {
	case pattern.Optional && actual.Optional:
		in.unify(pattern.Elem, actual.Elem)
	case pattern.Optional:
		in.unify(pattern.Elem, actual)
	case pattern.Kind == types.ResultKind && actual.Kind != types.ResultKind:
		in.unify(pattern.Ok, actual)
	case pattern.Kind != actual.Kind:
	case pattern.Kind == types.FuncKind && len(pattern.Params) == len(actual.Params):
		in.pairs(pattern.Params, actual.Params)
		in.unify(pattern.Ret, actual.Ret)
	case pattern.Kind == types.ResultKind:
		in.unify(pattern.Ok, actual.Ok)
		in.unify(pattern.Err, actual.Err)
	case pattern.Origin != nil && pattern.Origin == actual.Origin:
		in.pairs(pattern.TypeArgs, actual.TypeArgs)
	default:
		in.unify(pattern.Key, actual.Key)
		in.unify(pattern.Elem, actual.Elem)
	}
}

func (in *inference) pairs(patterns, actuals []*types.Type) {
	for i := range min(len(patterns), len(actuals)) {
		in.unify(patterns[i], actuals[i])
	}
}

func (c *Checker) inferred(in *inference, pos lexer.Pos) []*types.Type {
	args := make([]*types.Type, len(in.params))
	for i, p := range in.params {
		a, ok := in.bound[p]
		if !ok {
			c.fail(pos, "cannot infer type argument %s of %s; write %s[...]", p, in.name, in.name)
		}
		args[i] = a
	}
	c.satisfy(args, in.decls, in.params, pos)
	return args
}

func (c *Checker) substitute(t *types.Type, in *inference, pos lexer.Pos) *types.Type {
	if t == nil || !t.Abstract() {
		return t
	}
	if in.param(t) {
		return in.bound[t]
	}
	sub := func(x *types.Type) *types.Type { return c.substitute(x, in, pos) }
	subs := func(xs []*types.Type) []*types.Type {
		out := make([]*types.Type, len(xs))
		for i, x := range xs {
			if out[i] = sub(x); out[i] == nil {
				return nil
			}
		}
		return out
	}
	switch {
	case t.Kind == types.ParamKind:
		return t
	case t.Optional:
		if e := sub(t.Elem); e != nil {
			return c.optional(e)
		}
	case t.Kind == types.SliceKind:
		if e := sub(t.Elem); e != nil {
			return c.slice(e)
		}
	case t.Kind == types.PointerKind:
		if e := sub(t.Elem); e != nil {
			return c.pointer(e)
		}
	case t.Kind == types.RawKind:
		if e := sub(t.Elem); e != nil {
			return c.raw(e)
		}
	case t.Kind == types.ChanKind:
		if e := sub(t.Elem); e != nil {
			return c.chanOf(e)
		}
	case t.Kind == types.MapKind:
		k, v := sub(t.Key), sub(t.Elem)
		if k != nil && v != nil {
			return c.mapOf(k, v)
		}
	case t.Kind == types.ResultKind:
		ok, err := sub(t.Ok), sub(t.Err)
		if ok != nil && err != nil {
			return c.result(ok, err)
		}
	case t.Kind == types.FuncKind:
		params, ret := subs(t.Params), sub(t.Ret)
		if params != nil && ret != nil {
			return c.funcType(params, t.Muts, ret, t.C)
		}
	case t.Origin != nil:
		if args := subs(t.TypeArgs); args != nil {
			return c.instantiateType(c.generics[t.Origin], args, pos)
		}
	}
	return nil
}

func (c *Checker) genericFuncValue(fn *ast.Func, want *types.Type, pos lexer.Pos) (*ast.Func, *types.Type) {
	if want == nil || want.Kind != types.FuncKind || len(want.Params) != len(fn.Params) {
		c.fail(pos, "cannot infer type arguments of %s; annotate the binding", fn.Name)
	}
	in := c.infer(fn.Name, fn.TypeParams, fn.TypeArgs)
	in.pairs(fn.ParamTypes, want.Params)
	in.unify(fn.RetType, want.Ret)
	args := c.inferred(in, pos)
	if c.concrete(args) {
		return c.instance(fn, args, pos), nil
	}
	params := make([]*types.Type, len(fn.ParamTypes))
	for i, p := range fn.ParamTypes {
		params[i] = c.substitute(p, in, pos)
	}
	return nil, c.funcType(params, want.Muts, c.substitute(fn.RetType, in, pos), want.C)
}

func (c *Checker) concrete(args []*types.Type) bool {
	for _, a := range args {
		if a.Abstract() {
			return false
		}
	}
	return true
}

func (c *Checker) genericCall(e *ast.Call, fn *ast.Func, explicit []*ast.TypeName, want *types.Type) {
	in := c.infer(fn.Name, fn.TypeParams, fn.TypeArgs)
	if explicit != nil {
		if len(explicit) != len(in.params) {
			c.fail(e.Pos, "%s takes %s, found %d", fn.Name, plural(len(in.params), "type argument"), len(explicit))
		}
		for i, t := range explicit {
			in.bound[in.params[i]] = c.resolve(t)
		}
	}
	if len(e.Args) != len(fn.Params) {
		c.fail(e.Pos, "%s takes %d arguments, found %d", fn.Name, len(fn.Params), len(e.Args))
	}
	found := make([]*types.Type, len(e.Args))
	for i, arg := range e.Args {
		r, isRef := arg.(*ast.Ref)
		switch {
		case fn.Params[i].Mut && isRef:
			found[i] = c.borrow(r)
		case fn.Params[i].Mut:
			c.fail(arg.Position(), "parameter %s of %s is mut; write mut before the argument", fn.Params[i].Name, fn.Name)
		default:
			found[i] = c.plain(arg, c.substitute(fn.ParamTypes[i], in, arg.Position()))
		}
		in.unify(fn.ParamTypes[i], found[i])
	}
	in.unify(fn.RetType, unwrap(want))
	args := c.inferred(in, e.Pos)
	for i, arg := range e.Args {
		param := c.substitute(fn.ParamTypes[i], in, arg.Position())
		exact := fn.Params[i].Mut && param != found[i]
		if exact || !param.Accepts(found[i]) {
			c.fail(arg.Position(), "expected %s, found %s", param, found[i])
		}
	}
	c.exclusive(e.Args)
	e.Target = fn
	if c.concrete(args) {
		e.Target = c.instance(fn, args, e.Pos)
	}
	e.Type = c.substitute(fn.RetType, in, e.Pos)
	c.forwarded(e, fn)
}

func (c *Checker) genericTarget(x ast.Expr) *ast.Func {
	var fn *ast.Func
	switch x := x.(type) {
	case *ast.Ident:
		if c.find(x.Name) == nil {
			fn = c.local().funcs[x.Name]
		}
	case *ast.Selector:
		if id, ok := x.X.(*ast.Ident); ok && c.isModule(id.Name) {
			fn = c.importedFunc(id.Name, x.Name, x.Pos)
		}
	}
	if fn == nil || fn.TypeParams == nil {
		return nil
	}
	return fn
}

func (c *Checker) instanceCall(e *ast.Call, inst *ast.Instance, want *types.Type) {
	if c.derivedCall(e, inst) {
		return
	}
	if id, ok := inst.X.(*ast.Ident); ok && id.Name == "chan" && c.find("chan") == nil && c.local().funcs["chan"] == nil {
		c.makeChan(e, inst)
		return
	}
	if fn := c.genericTarget(inst.X); fn != nil {
		c.genericCall(e, fn, inst.Types, want)
		return
	}
	if inst.Fallback == nil {
		c.fail(inst.Pos, "only generic functions take type arguments")
	}
	e.Fn = inst.Fallback
	c.call(e, want)
}

func (c *Checker) genericLitType(e *ast.StructLit, g *generic, want *types.Type, checked map[*ast.FieldInit]*types.Type) *types.Type {
	if g.strct == nil {
		c.fail(e.Pos, "%s is not a struct", e.Name)
	}
	if e.TypeArgs != nil {
		args := make([]*types.Type, len(e.TypeArgs))
		for i, t := range e.TypeArgs {
			args[i] = c.resolve(t)
		}
		return c.instantiateType(g, args, e.Pos)
	}
	origin := c.origin(g)
	if w := unwrap(want); w != nil && w.Origin == origin {
		return w
	}
	in := c.infer(g.name, g.decls, g.params)
	for _, init := range e.Fields {
		f := origin.Field(init.Name)
		if f == nil {
			c.fail(init.Pos, "%s has no field %s", g.name, init.Name)
		}
		checked[init] = c.plain(init.Value, c.substitute(f.Type, in, init.Pos))
		in.unify(f.Type, checked[init])
	}
	return c.instantiateType(g, c.inferred(in, e.Pos), e.Pos)
}

func (c *Checker) genericEnum(g *generic, name string, args []ast.Expr, want *types.Type, pos lexer.Pos) (*types.Type, *types.Variant, []*types.Type) {
	if g.enum == nil {
		c.fail(pos, "%s is a type, not a value", g.name)
	}
	origin := c.origin(g)
	if w := unwrap(want); w != nil && w.Origin == origin {
		return w, c.variant(w, name, pos), nil
	}
	v := c.variant(origin, name, pos)
	if len(args) != len(v.Fields) {
		c.fail(pos, "variant %s takes %d fields, found %d", name, len(v.Fields), len(args))
	}
	in := c.infer(g.name, g.decls, g.params)
	found := make([]*types.Type, len(args))
	for i, arg := range args {
		found[i] = c.plain(arg, c.substitute(v.Fields[i], in, arg.Position()))
		in.unify(v.Fields[i], found[i])
	}
	t := c.instantiateType(g, c.inferred(in, pos), pos)
	return t, t.Variant(name), found
}

func (c *Checker) mapOps(m *types.Type) {
	if m.Abstract() {
		return
	}
	if !c.funcsReady {
		c.waitingMaps = append(c.waitingMaps, m)
		return
	}
	args := []*types.Type{m.Key, m.Elem}
	for _, name := range mapFuncs {
		c.instance(c.scope("").funcs[name], args, lexer.Pos{})
	}
}
