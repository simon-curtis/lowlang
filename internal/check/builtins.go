package check

import (
	"fmt"
	"slices"
	"strings"

	"lowlang/internal/ast"
	"lowlang/internal/builtin"
	"lowlang/internal/lexer"
	"lowlang/internal/types"
)

type BuiltinKind int

const (
	FunctionBuiltin BuiltinKind = iota
	TypeBuiltin
	ConstantBuiltin
	PropertyBuiltin
)

const mutMarker = "mut "

type builtinHandler func(c *Checker, b *Builtin, e *ast.Call, want *types.Type)

type Builtin struct {
	ID         builtin.ID
	Name       string
	Kind       BuiltinKind
	Shadowable bool
	Receivers  []types.Kind
	Params     []string
	Optional   int
	TypeArgs   int
	Type       *types.Type

	handler   builtinHandler
	construct func(c *Checker, e *ast.Call, inst *ast.Instance)
	resolve   func(c *Checker, t *ast.TypeName) *types.Type
	constant  func(c *Checker, e *ast.Ident, want *types.Type)
}

func (b *Builtin) Public() bool {
	return !strings.HasPrefix(b.Name, "__")
}

func (b *Builtin) Mut(i int) bool {
	return i < len(b.Params) && strings.HasPrefix(b.Params[i], mutMarker)
}

func (b *Builtin) ParamName(i int) string {
	if i >= len(b.Params) {
		return "value"
	}
	return strings.TrimPrefix(b.Params[i], mutMarker)
}

func (b *Builtin) Receives(kind types.Kind) bool {
	return slices.Contains(b.Receivers, kind)
}

var (
	builtins     []*Builtin
	builtinIndex map[string]*Builtin
)

func init() {
	builtins = newBuiltins()
	builtinIndex = make(map[string]*Builtin, len(builtins))
	for _, b := range builtins {
		builtinIndex[b.Name] = b
	}
}

func Builtins() []*Builtin {
	return builtins
}

func LookupBuiltin(name string) *Builtin {
	return builtinIndex[name]
}

func Reserved(name string) bool {
	return lexer.IsKeyword(name) || builtinIndex[name] != nil
}

func Ident(name string) string {
	if Reserved(name) {
		return fmt.Sprintf("%s_", name)
	}
	return name
}

func reservedType(name string) bool {
	b := builtinIndex[name]
	return b != nil && b.Kind == TypeBuiltin
}

func bare(h func(c *Checker, e *ast.Call)) builtinHandler {
	return func(c *Checker, _ *Builtin, e *ast.Call, _ *types.Type) { h(c, e) }
}

func wanted(h func(c *Checker, e *ast.Call, want *types.Type)) builtinHandler {
	return func(c *Checker, _ *Builtin, e *ast.Call, want *types.Type) { h(c, e, want) }
}

func named(h func(c *Checker, b *Builtin, e *ast.Call)) builtinHandler {
	return func(c *Checker, b *Builtin, e *ast.Call, _ *types.Type) { h(c, b, e) }
}

func newBuiltins() []*Builtin {
	list := []*Builtin{
		{ID: builtin.Print, Name: "print", Params: []string{"value"}, handler: bare((*Checker).print)},
		{ID: builtin.New, Name: "new", Params: []string{"value"}, handler: wanted((*Checker).allocate)},
		{ID: builtin.Push, Name: "push", Receivers: []types.Kind{types.SliceKind}, Params: []string{"mut xs", "value"}, handler: named((*Checker).push)},
		{ID: builtin.Pop, Name: "pop", Shadowable: true, Receivers: []types.Kind{types.SliceKind}, Params: []string{"mut xs"}, handler: named((*Checker).pop)},
		{ID: builtin.Truncate, Name: "truncate", Shadowable: true, Receivers: []types.Kind{types.SliceKind}, Params: []string{"mut xs", "n"}, handler: named((*Checker).truncate)},
		{ID: builtin.Delete, Name: "delete", Receivers: []types.Kind{types.MapKind}, Params: []string{"mut m", "key"}, handler: bare((*Checker).deleteEntry)},
		{ID: builtin.Bytes, Name: "bytes", Params: []string{"s"}, handler: bare((*Checker).bytesCall)},
		{ID: builtin.Cstr, Name: "cstr", Params: []string{"s"}, handler: bare((*Checker).cstrCall)},
		{ID: builtin.View, Name: "view", Params: []string{"p", "n"}, handler: named((*Checker).viewCall)},
		{ID: builtin.Load, Name: "load", Params: []string{"p"}, handler: named((*Checker).loadCall)},
		{ID: builtin.Store, Name: "store", Params: []string{"p", "value"}, handler: named((*Checker).storeCall)},
		{ID: builtin.Pin, Name: "pin", Params: []string{"value"}, handler: bare((*Checker).pinCall)},
		{ID: builtin.Unpin, Name: "unpin", Params: []string{"p"}, handler: bare((*Checker).unpinCall)},
		{ID: builtin.Embed, Name: "embed", Params: []string{"path"}, handler: bare((*Checker).embed)},
		{ID: builtin.Sqrt, Name: "sqrt", Params: []string{"x"}, handler: (*Checker).math},
		{ID: builtin.Floor, Name: "floor", Params: []string{"x"}, handler: (*Checker).math},
		{ID: builtin.Exp, Name: "exp", Params: []string{"x"}, handler: (*Checker).math},
		{ID: builtin.Close, Name: "close", Shadowable: true, Receivers: []types.Kind{types.ChanKind}, Params: []string{"c"}, handler: bare((*Checker).closeCall)},
		{ID: builtin.Sleep, Name: "sleep", Shadowable: true, Params: []string{"ms"}, handler: bare((*Checker).waitCall)},
		{ID: builtin.WaitReadable, Name: "wait_readable", Shadowable: true, Params: []string{"fd"}, handler: bare((*Checker).waitCall)},
		{ID: builtin.WaitWritable, Name: "wait_writable", Shadowable: true, Params: []string{"fd"}, handler: bare((*Checker).waitCall)},
	}
	list = append(list, privateBuiltins()...)
	list = append(list, typeBuiltins()...)
	list = append(list, derivedBuiltins()...)
	return append(list,
		&Builtin{Name: "len", Kind: PropertyBuiltin, Shadowable: true, Receivers: []types.Kind{types.SliceKind, types.MapKind, types.StrKind}, Type: types.I64},
		&Builtin{Name: "true", Kind: ConstantBuiltin, Shadowable: true, Type: types.Bool},
		&Builtin{Name: "false", Kind: ConstantBuiltin, Shadowable: true, Type: types.Bool},
		&Builtin{Name: "none", Kind: ConstantBuiltin, Shadowable: true, constant: (*Checker).noneConstant},
		&Builtin{Name: "null", Kind: ConstantBuiltin, Shadowable: true, constant: (*Checker).nullConstant},
	)
}

func privateBuiltins() []*Builtin {
	return []*Builtin{
		{ID: builtin.StrRaw, Name: "__str_raw", Params: []string{"address", "len"}, handler: bare((*Checker).strRawCall)},
		{ID: builtin.SlicePtr, Name: "__slice_ptr", Params: []string{"xs"}, handler: bare((*Checker).slicePtrCall)},
		{ID: builtin.Bits, Name: "__bits", Params: []string{"x"}, handler: bare((*Checker).bitsCall)},
		{ID: builtin.StrView, Name: "__str_view", Params: []string{"b"}, handler: bare((*Checker).strViewCall)},
		{ID: builtin.Extend, Name: "__extend", Params: []string{"mut b", "s"}, handler: bare((*Checker).extendCall)},
		{ID: builtin.ExtendBy, Name: "__extend_by", Params: []string{"mut b", "n"}, handler: bare((*Checker).extendByCall)},
		{ID: builtin.Hash, Name: "__hash", Params: []string{"value"}, handler: bare((*Checker).hashCall)},
		{ID: builtin.Zero, Name: "__zero", handler: wanted((*Checker).zeroCall)},
		{ID: builtin.Syscall, Name: "__syscall", Params: []string{"number", "a", "b", "c", "d", "e", "f"}, Optional: 6, handler: bare((*Checker).syscall)},
		{ID: builtin.MulHi, Name: "__mulhi", Params: []string{"a", "b"}, handler: bare((*Checker).mulhiCall)},
		{ID: builtin.Make, Name: "__make", Params: []string{"n"}, handler: wanted((*Checker).makeCall)},
		{ID: builtin.Memmove, Name: "__memmove", Params: []string{"dst", "src"}, handler: bare((*Checker).memmoveCall)},
		{ID: builtin.Ctz, Name: "__ctz", Params: []string{"x"}, handler: bare((*Checker).ctzCall)},
		{ID: builtin.ClockNanos, Name: "__clock_nanos", Params: []string{"clock"}, handler: bare((*Checker).clockNanosCall)},
		{ID: builtin.HashSeed, Name: "__hash_seed", Params: []string{"which"}, handler: bare((*Checker).hashSeedCall)},
		{ID: builtin.Argc, Name: "__argc", handler: bare((*Checker).argcCall)},
		{ID: builtin.Argv, Name: "__argv", Params: []string{"i"}, handler: bare((*Checker).argvCall)},
		{ID: builtin.Envv, Name: "__envv", Params: []string{"i"}, handler: bare((*Checker).argvCall)},
		{ID: builtin.Render, Name: "__render", Params: []string{"value"}, handler: bare((*Checker).renderCall)},
		{ID: builtin.RenderInto, Name: "__render_into", Params: []string{"mut b", "mut path", "value"}, handler: bare((*Checker).renderIntoCall)},
		{ID: builtin.Address, Name: "__address", Params: []string{"p"}, handler: bare((*Checker).addressCall)},
		{ID: builtin.Serialize, Name: "__serialize", Params: []string{"value", "w"}, handler: bare((*Checker).serializeCall)},
		{ID: builtin.SerializeInto, Name: "__serialize_into", Params: []string{"w", "mut path", "value"}, handler: bare((*Checker).serializeIntoCall)},
		{ID: builtin.Spawn, Name: "__spawn", Params: []string{"f"}, handler: bare((*Checker).spawnCall)},
		{ID: builtin.Current, Name: "__current", handler: bare((*Checker).currentCall)},
		{ID: builtin.Park, Name: "__park", handler: bare((*Checker).parkCall)},
		{ID: builtin.Ready, Name: "__ready", Params: []string{"t"}, handler: bare((*Checker).readyCall)},
		{ID: builtin.Lock, Name: "__lock", Params: []string{"mut word"}, handler: bare((*Checker).lockCall)},
		{ID: builtin.Unlock, Name: "__unlock", Params: []string{"mut word"}, handler: bare((*Checker).lockCall)},
		{ID: builtin.Cas, Name: "__cas", Params: []string{"mut word", "old", "new"}, handler: bare((*Checker).casCall)},
		{ID: builtin.Panic, Name: "__panic", Params: []string{"message"}, handler: bare((*Checker).panicCall)},
		{ID: builtin.Context, Name: "__context", handler: wanted((*Checker).contextCall)},
		{ID: builtin.SetContext, Name: "__set_context", Params: []string{"p"}, handler: bare((*Checker).setContextCall)},
		{ID: builtin.HasContext, Name: "__has_context", handler: bare((*Checker).hasContextCall)},
		{ID: builtin.LockThread, Name: "__lock_thread", handler: bare((*Checker).lockThreadCall)},
		{ID: builtin.UnlockThread, Name: "__unlock_thread", handler: bare((*Checker).lockThreadCall)},
		{ID: builtin.VLoad, Name: "__vload", Params: []string{"src", "at"}, handler: (*Checker).vectorLoad},
		{ID: builtin.VStore, Name: "__vstore", Params: []string{"dst", "at", "v"}, handler: bare((*Checker).vectorStore)},
		{ID: builtin.VSplat, Name: "__vsplat", Params: []string{"x"}, handler: (*Checker).vectorSplat},
		{ID: builtin.VMask, Name: "__vmask", Params: []string{"v"}, handler: bare((*Checker).vectorMask)},
		{ID: builtin.VShuffle, Name: "__vshuffle", Params: []string{"v", "order"}, handler: bare((*Checker).vectorShuffle)},
		{ID: builtin.VSelect, Name: "__vselect", Params: []string{"mask", "a", "b"}, handler: bare((*Checker).vectorSelect)},
		{ID: builtin.VMin, Name: "__vmin", Params: []string{"a", "b"}, handler: bare((*Checker).vectorPair)},
		{ID: builtin.VMax, Name: "__vmax", Params: []string{"a", "b"}, handler: bare((*Checker).vectorPair)},
		{ID: builtin.VLane, Name: "__vlane", Params: []string{"v", "i"}, handler: wanted((*Checker).vectorLane)},
		{ID: builtin.VWith, Name: "__vwith", Params: []string{"v", "i", "x"}, handler: bare((*Checker).vectorWith)},
	}
}

func derivedBuiltins() []*Builtin {
	return []*Builtin{
		{Name: "__deserialize", Params: []string{"r"}, TypeArgs: 1, construct: (*Checker).deserializeCall},
		{Name: "__schema", TypeArgs: 1, construct: (*Checker).schemaCall},
	}
}

func typeBuiltins() []*Builtin {
	value := []string{"value"}
	list := []*Builtin{
		{ID: builtin.Convert, Name: "ptr", Kind: TypeBuiltin, Params: value, handler: bare((*Checker).ptrConversion), resolve: (*Checker).resolveRaw},
		{ID: builtin.Chan, Name: "chan", Kind: TypeBuiltin, Params: []string{"capacity"}, Optional: 1, TypeArgs: 1, construct: (*Checker).makeChan, resolve: (*Checker).resolveChan},
	}
	for _, declared := range types.BuiltinTypes() {
		name, t := declared.Name, declared.Type
		entry := &Builtin{Name: name, Kind: TypeBuiltin, Type: t}
		switch {
		case t.Kind == types.VectorKind:
			entry.ID, entry.Params, entry.handler = builtin.VectorCast, value, vectorCaster(t)
		case t == types.Str:
			entry.ID, entry.Params, entry.handler = builtin.Str, value, bare((*Checker).strCall)
		case t.Number():
			entry.ID, entry.Params, entry.handler = builtin.Convert, value, converter(t)
		}
		list = append(list, entry)
	}
	return list
}

func (c *Checker) ptrConversion(e *ast.Call) {
	c.convert(e, c.universe.Raw(nil))
}

func converter(to *types.Type) builtinHandler {
	return func(c *Checker, _ *Builtin, e *ast.Call, _ *types.Type) { c.convert(e, to) }
}

func vectorCaster(to *types.Type) builtinHandler {
	return func(c *Checker, _ *Builtin, e *ast.Call, _ *types.Type) { c.vectorCast(e, to) }
}

func (c *Checker) declared(name string) bool {
	scope := c.local()
	_, bound := c.bound[name]
	return scope.funcs[name] != nil || scope.generics[name] != nil || bound || c.find(name) != nil
}

func (c *Checker) builtinNamed(name string) *Builtin {
	b := builtinIndex[name]
	if b == nil || (b.Shadowable && c.declared(name)) {
		return nil
	}
	return b
}

func (c *Checker) callBuiltin(b *Builtin, e *ast.Call, want *types.Type) {
	c.arity(b, e)
	e.Builtin = b.ID
	c.borrowMuts(b, e)
	b.handler(c, b, e, want)
}

func (c *Checker) borrowMuts(b *Builtin, e *ast.Call) {
	for i, arg := range e.Args {
		if r := c.mutArgument(arg, b.Mut(i), "%s of %s", b.ParamName(i), b.Name); r != nil {
			c.borrowInPlace(r)
		}
	}
}

func (c *Checker) typeArity(b *Builtin, inst *ast.Instance) {
	if len(inst.Types) != b.TypeArgs {
		c.fail(inst.Pos, "%s takes %s, found %d", b.Name, plural(b.TypeArgs, "type argument"), len(inst.Types))
	}
}

func (c *Checker) arity(b *Builtin, e *ast.Call) {
	most := len(b.Params)
	least := most - b.Optional
	found := len(e.Args)
	if found >= least && found <= most {
		return
	}
	c.fail(e.Pos, "%s takes %s, found %d", b.Name, argumentCount(least, most), found)
}

func argumentCount(least, most int) string {
	switch {
	case least != most:
		return fmt.Sprintf("%d to %d arguments", least, most)
	case most == 0:
		return "no arguments"
	case most == 1:
		return "1 argument"
	}
	return fmt.Sprintf("%d arguments", most)
}

func (c *Checker) callable(name string) bool {
	b := builtinIndex[name]
	return (b != nil && !b.Shadowable) || c.declared(name)
}

func (c *Checker) firstMut(fn ast.Expr) bool {
	switch fn := fn.(type) {
	case *ast.Ident:
		if b := c.builtinNamed(fn.Name); b != nil {
			return b.Mut(0)
		}
		if c.find(fn.Name) != nil {
			return false
		}
		target, ok := c.localFunc(fn.Name)
		return ok && len(target.Params) > 0 && target.Params[0].Mut
	case *ast.Selector:
		id, isIdent := fn.X.(*ast.Ident)
		if !isIdent || !c.isModule(id.Name) {
			return false
		}
		target := c.importedFunc(id.Name, fn.Name, fn.Pos)
		return len(target.Params) > 0 && target.Params[0].Mut
	}
	return false
}

func (c *Checker) builtinType(b *Builtin, t *ast.TypeName) *types.Type {
	switch {
	case b.resolve != nil:
		return b.resolve(c, t)
	case t.Args != nil:
		c.fail(t.Pos, "%s is not generic", t.Name)
	}
	return b.Type
}

func (c *Checker) resolveChan(t *ast.TypeName) *types.Type {
	if len(t.Args) != 1 {
		c.fail(t.Pos, "chan takes 1 type argument")
	}
	return c.universe.Chan(c.contained(t.Args[0], "a channel"))
}

func (c *Checker) resolveRaw(t *ast.TypeName) *types.Type {
	switch len(t.Args) {
	case 0:
		return c.universe.Raw(nil)
	case 1:
		return c.universe.Raw(c.contained(t.Args[0], "a ptr"))
	}
	c.fail(t.Pos, "ptr takes at most 1 type argument")
	return nil
}

func (c *Checker) noneConstant(e *ast.Ident, _ *types.Type) {
	c.fail(e.Pos, "cannot infer the type of none; annotate the binding")
}

func (c *Checker) nullConstant(e *ast.Ident, want *types.Type) {
	e.Null = true
	e.Type = c.universe.Raw(nil)
	if w := unwrap(want); w != nil && w.Kind == types.RawKind {
		e.Type = w
	}
}

func (c *Checker) bytesCall(e *ast.Call) {
	c.expectType(e.Args[0], types.Str)
	e.Type = c.universe.Slice(types.U8)
}

func (c *Checker) strCall(e *ast.Call) {
	e.Type = types.Str
	if t := c.plain(e.Args[0], nil); t.Kind == types.RawKind {
		c.cString(e.Args[0], t)
	} else if t != c.universe.Slice(types.U8) {
		c.fail(e.Args[0].Position(), "str takes []u8 or ptr[u8], found %s", t)
	}
}

func (c *Checker) cstrCall(e *ast.Call) {
	c.expectType(e.Args[0], types.Str)
	e.Type = c.universe.Raw(types.U8)
}

func (c *Checker) viewCall(b *Builtin, e *ast.Call) {
	elem := c.typedRaw(e.Args[0], b.Name)
	c.integer(e.Args[1])
	e.Type = c.universe.Slice(elem)
}

func (c *Checker) loadCall(b *Builtin, e *ast.Call) {
	e.Type = c.typedRaw(e.Args[0], b.Name)
}

func (c *Checker) storeCall(b *Builtin, e *ast.Call) {
	c.expectType(e.Args[1], c.typedRaw(e.Args[0], b.Name))
	e.Type = types.Void
}

func (c *Checker) pinCall(e *ast.Call) {
	t := c.plain(e.Args[0], nil)
	if t.Kind != types.PointerKind && t.Kind != types.SliceKind && t.Kind != types.MapKind && t != types.Str {
		c.fail(e.Args[0].Position(), "pin takes a pointer, slice, map or str, found %s", t)
	}
	e.Type = c.universe.Raw(nil)
}

func (c *Checker) unpinCall(e *ast.Call) {
	if t := c.plain(e.Args[0], nil); t.Kind != types.RawKind {
		c.fail(e.Args[0].Position(), "unpin takes the ptr returned by pin, found %s", t)
	}
	e.Type = types.Void
}

func (c *Checker) closeCall(e *ast.Call) {
	c.channel(e.Args[0])
	e.Type = types.Void
}

func (c *Checker) waitCall(e *ast.Call) {
	c.integer(e.Args[0])
	e.Type = types.Void
}

func (c *Checker) bitsCall(e *ast.Call) {
	t := c.plain(e.Args[0], nil)
	e.Type = types.I64
	if t == types.F32 {
		e.Type = types.U32
	} else if t != types.F64 {
		c.fail(e.Pos, "__bits requires a float")
	}
}

func (c *Checker) strRawCall(e *ast.Call) {
	c.integer(e.Args[0])
	c.integer(e.Args[1])
	e.Type = types.Str
}

func (c *Checker) slicePtrCall(e *ast.Call) {
	t := c.plain(e.Args[0], nil)
	switch {
	case t == types.Str:
		e.Type = c.universe.Raw(types.U8)
	case t.Kind == types.SliceKind:
		e.Type = c.universe.Raw(t.Elem)
	default:
		c.fail(e.Pos, "__slice_ptr requires a slice or str")
	}
}

func (c *Checker) ctzCall(e *ast.Call) {
	c.expectType(e.Args[0], types.U64)
	e.Type = types.I64
}

func (c *Checker) clockNanosCall(e *ast.Call) {
	c.expectType(e.Args[0], types.I64)
	e.Type = types.I64
}

func (c *Checker) hashSeedCall(e *ast.Call) {
	if lit, ok := e.Args[0].(*ast.IntLit); !ok || lit.Value < 0 || lit.Value > 1 {
		c.fail(e.Pos, "__hash_seed takes the literal 0 or 1")
	}
	c.expectType(e.Args[0], types.I64)
	e.Type = types.U64
}

func (c *Checker) argcCall(e *ast.Call) {
	e.Type = types.I64
}

func (c *Checker) argvCall(e *ast.Call) {
	c.expectType(e.Args[0], types.I64)
	e.Type = c.universe.Raw(types.U8)
}

func (c *Checker) mulhiCall(e *ast.Call) {
	c.expectType(e.Args[0], types.U64)
	c.expectType(e.Args[1], types.U64)
	e.Type = types.U64
}

func (c *Checker) makeCall(e *ast.Call, want *types.Type) {
	if want == nil || want.Kind != types.SliceKind {
		c.fail(e.Pos, "__make needs an expected slice type")
	}
	c.integer(e.Args[0])
	e.Type = want
}

func (c *Checker) memmoveCall(e *ast.Call) {
	dst := c.plain(e.Args[0], nil)
	if dst.Kind != types.SliceKind {
		c.fail(e.Args[0].Position(), "__memmove needs a slice, found %s", dst)
	}
	src := c.plain(e.Args[1], dst)
	if !dst.Accepts(src) && (dst.Elem != types.U8 || src != types.Str) {
		c.fail(e.Args[1].Position(), "expected %s, found %s", dst, src)
	}
	e.Type = types.I64
}

func (c *Checker) extendCall(e *ast.Call) {
	c.mutOf(e, 0, c.universe.Slice(types.U8))
	c.expectType(e.Args[1], types.Str)
	e.Type = types.Void
}

func (c *Checker) extendByCall(e *ast.Call) {
	c.mutOf(e, 0, c.universe.Slice(types.U8))
	c.expectType(e.Args[1], types.I64)
	e.Type = types.I64
}

func (c *Checker) strViewCall(e *ast.Call) {
	c.expectType(e.Args[0], c.universe.Slice(types.U8))
	e.Type = types.Str
}

func (c *Checker) hashCall(e *ast.Call) {
	if t := c.plain(e.Args[0], nil); !t.Hashable() {
		c.fail(e.Args[0].Position(), "__hash requires a hashable value, found %s", t)
	}
	e.Type = types.I64
}

func (c *Checker) zeroCall(e *ast.Call, want *types.Type) {
	if want == nil {
		c.fail(e.Pos, "__zero needs an expected type")
	}
	e.Type = want
}

func (c *Checker) contextCall(e *ast.Call, want *types.Type) {
	if want == nil || want.Kind != types.PointerKind {
		c.fail(e.Pos, "__context needs an expected pointer type")
	}
	e.Type = want
}

func (c *Checker) setContextCall(e *ast.Call) {
	if t := c.plain(e.Args[0], nil); t.Kind != types.PointerKind {
		c.fail(e.Args[0].Position(), "__set_context needs a pointer, found %s", t)
	}
	e.Type = types.Void
}

func (c *Checker) hasContextCall(e *ast.Call) {
	e.Type = types.Bool
}

func (c *Checker) spawnCall(e *ast.Call) {
	t := c.plain(e.Args[0], nil)
	if t.Kind != types.FuncKind || t.C || len(t.Params) > 0 {
		c.fail(e.Pos, "__spawn takes a function value with no parameters")
	}
	e.Type = types.Void
}

func (c *Checker) currentCall(e *ast.Call) {
	e.Type = c.universe.Raw(nil)
}

func (c *Checker) parkCall(e *ast.Call) {
	e.Type = types.Void
}

func (c *Checker) lockThreadCall(e *ast.Call) {
	e.Type = types.Void
}

func (c *Checker) readyCall(e *ast.Call) {
	c.expectType(e.Args[0], c.universe.Raw(nil))
	e.Type = types.Void
}

func (c *Checker) lockCall(e *ast.Call) {
	c.address(c.mutOf(e, 0, types.I64))
	e.Type = types.Void
}

func (c *Checker) casCall(e *ast.Call) {
	c.address(c.mutOf(e, 0, types.I64))
	c.expectType(e.Args[1], types.I64)
	c.expectType(e.Args[2], types.I64)
	e.Type = types.Bool
}

func (c *Checker) panicCall(e *ast.Call) {
	c.expectType(e.Args[0], types.Str)
	e.Type = types.Never
}
