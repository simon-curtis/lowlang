package check

import (
	"fmt"
	"slices"
	"strings"

	"lowlang/internal/ast"
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

type builtinHandler func(c *Checker, e *ast.Call, want *types.Type)

type Builtin struct {
	Name       string
	Kind       BuiltinKind
	Shadowable bool
	MutFirst   bool
	Receivers  []types.Kind
	Params     []string
	Optional   int
	Type       *types.Type

	handler   builtinHandler
	construct func(c *Checker, e *ast.Call, inst *ast.Instance)
	resolve   func(c *Checker, t *ast.TypeName) *types.Type
	constant  func(c *Checker, e *ast.Ident, want *types.Type)
}

func (b *Builtin) Public() bool {
	return !strings.HasPrefix(b.Name, "__")
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

func bare(h func(c *Checker, e *ast.Call)) builtinHandler {
	return func(c *Checker, e *ast.Call, _ *types.Type) { h(c, e) }
}

func newBuiltins() []*Builtin {
	list := []*Builtin{
		{Name: "print", Params: []string{"value"}, handler: bare((*Checker).print)},
		{Name: "new", Params: []string{"value"}, handler: (*Checker).allocate},
		{Name: "push", MutFirst: true, Receivers: []types.Kind{types.SliceKind}, Params: []string{"xs", "value"}, handler: bare((*Checker).push)},
		{Name: "pop", Shadowable: true, MutFirst: true, Receivers: []types.Kind{types.SliceKind}, Params: []string{"xs"}, handler: bare((*Checker).shrink)},
		{Name: "truncate", Shadowable: true, MutFirst: true, Receivers: []types.Kind{types.SliceKind}, Params: []string{"xs", "n"}, handler: bare((*Checker).shrink)},
		{Name: "delete", MutFirst: true, Receivers: []types.Kind{types.MapKind}, Params: []string{"m", "key"}, handler: bare((*Checker).deleteEntry)},
		{Name: "bytes", Params: []string{"s"}, handler: bare((*Checker).bytesCall)},
		{Name: "cstr", Params: []string{"s"}, handler: bare((*Checker).cstrCall)},
		{Name: "view", Params: []string{"p", "n"}, handler: bare((*Checker).viewCall)},
		{Name: "load", Params: []string{"p"}, handler: bare((*Checker).loadCall)},
		{Name: "store", Params: []string{"p", "value"}, handler: bare((*Checker).storeCall)},
		{Name: "pin", Params: []string{"value"}, handler: bare((*Checker).pinCall)},
		{Name: "unpin", Params: []string{"p"}, handler: bare((*Checker).unpinCall)},
		{Name: "embed", Params: []string{"path"}, handler: bare((*Checker).embed)},
		{Name: "sqrt", Params: []string{"x"}, handler: (*Checker).math},
		{Name: "floor", Params: []string{"x"}, handler: (*Checker).math},
		{Name: "exp", Params: []string{"x"}, handler: (*Checker).math},
		{Name: "close", Shadowable: true, Receivers: []types.Kind{types.ChanKind}, Params: []string{"c"}, handler: bare((*Checker).closeCall)},
		{Name: "sleep", Shadowable: true, Params: []string{"ms"}, handler: bare((*Checker).waitCall)},
		{Name: "wait_readable", Shadowable: true, Params: []string{"fd"}, handler: bare((*Checker).waitCall)},
		{Name: "wait_writable", Shadowable: true, Params: []string{"fd"}, handler: bare((*Checker).waitCall)},
	}
	list = append(list, privateBuiltins()...)
	list = append(list, typeBuiltins()...)
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
		{Name: "__str_raw", Params: []string{"address", "len"}, handler: bare((*Checker).strRawCall)},
		{Name: "__slice_ptr", Params: []string{"xs"}, handler: bare((*Checker).slicePtrCall)},
		{Name: "__bits", Params: []string{"x"}, handler: bare((*Checker).bitsCall)},
		{Name: "__str_view", Params: []string{"b"}, handler: bare((*Checker).strViewCall)},
		{Name: "__extend", Params: []string{"b", "s"}, handler: bare((*Checker).extendCall)},
		{Name: "__extend_by", Params: []string{"b", "n"}, handler: bare((*Checker).extendByCall)},
		{Name: "__hash", Params: []string{"value"}, handler: bare((*Checker).hashCall)},
		{Name: "__zero", handler: (*Checker).zeroCall},
		{Name: "__syscall", Params: []string{"number", "a", "b", "c", "d", "e", "f"}, Optional: 6, handler: bare((*Checker).syscall)},
		{Name: "__mulhi", Params: []string{"a", "b"}, handler: bare((*Checker).mulhiCall)},
		{Name: "__make", Params: []string{"n"}, handler: (*Checker).makeCall},
		{Name: "__memmove", Params: []string{"dst", "src"}, handler: bare((*Checker).memmoveCall)},
		{Name: "__ctz", Params: []string{"x"}, handler: bare((*Checker).ctzCall)},
		{Name: "__clock_nanos", Params: []string{"clock"}, handler: bare((*Checker).clockNanosCall)},
		{Name: "__hash_seed", Params: []string{"which"}, handler: bare((*Checker).hashSeedCall)},
		{Name: "__argc", handler: bare((*Checker).argcCall)},
		{Name: "__argv", Params: []string{"i"}, handler: bare((*Checker).argvCall)},
		{Name: "__envv", Params: []string{"i"}, handler: bare((*Checker).argvCall)},
		{Name: "__render", Params: []string{"value"}, handler: bare((*Checker).renderCall)},
		{Name: "__render_into", Params: []string{"b", "path", "value"}, handler: bare((*Checker).renderIntoCall)},
		{Name: "__address", Params: []string{"p"}, handler: bare((*Checker).addressCall)},
		{Name: "__serialize", Params: []string{"value", "w"}, handler: bare((*Checker).serializeCall)},
		{Name: "__serialize_into", Params: []string{"w", "path", "value"}, handler: bare((*Checker).serializeIntoCall)},
		{Name: "__spawn", Params: []string{"f"}, handler: bare((*Checker).spawnCall)},
		{Name: "__current", handler: bare((*Checker).currentCall)},
		{Name: "__park", handler: bare((*Checker).parkCall)},
		{Name: "__ready", Params: []string{"t"}, handler: bare((*Checker).readyCall)},
		{Name: "__lock", Params: []string{"word"}, handler: bare((*Checker).lockCall)},
		{Name: "__unlock", Params: []string{"word"}, handler: bare((*Checker).lockCall)},
		{Name: "__cas", Params: []string{"word", "old", "new"}, handler: bare((*Checker).casCall)},
		{Name: "__panic", Params: []string{"message"}, handler: bare((*Checker).panicCall)},
		{Name: "__context", handler: (*Checker).contextCall},
		{Name: "__set_context", Params: []string{"p"}, handler: bare((*Checker).setContextCall)},
		{Name: "__has_context", handler: bare((*Checker).hasContextCall)},
		{Name: "__vload", Params: []string{"src", "at"}, handler: (*Checker).vectorIntrinsic},
		{Name: "__vstore", Params: []string{"dst", "at", "v"}, handler: (*Checker).vectorIntrinsic},
		{Name: "__vsplat", Params: []string{"x"}, handler: (*Checker).vectorIntrinsic},
		{Name: "__vmask", Params: []string{"v"}, handler: (*Checker).vectorIntrinsic},
		{Name: "__vshuffle", Params: []string{"v", "order"}, handler: (*Checker).vectorIntrinsic},
		{Name: "__vselect", Params: []string{"mask", "a", "b"}, handler: (*Checker).vectorIntrinsic},
		{Name: "__vmin", Params: []string{"a", "b"}, handler: (*Checker).vectorIntrinsic},
		{Name: "__vmax", Params: []string{"a", "b"}, handler: (*Checker).vectorIntrinsic},
		{Name: "__vlane", Params: []string{"v", "i"}, handler: (*Checker).vectorIntrinsic},
		{Name: "__vwith", Params: []string{"v", "i", "x"}, handler: (*Checker).vectorIntrinsic},
	}
}

func typeBuiltins() []*Builtin {
	value := []string{"value"}
	var list []*Builtin
	for _, t := range types.Numeric {
		list = append(list, &Builtin{Name: t.Name, Kind: TypeBuiltin, Type: t, Params: value, handler: converter(t)})
	}
	list = append(list,
		&Builtin{Name: "bool", Kind: TypeBuiltin, Type: types.Bool},
		&Builtin{Name: "str", Kind: TypeBuiltin, Type: types.Str, Params: value, handler: bare((*Checker).strCall)},
		&Builtin{Name: "ptr", Kind: TypeBuiltin},
		&Builtin{Name: "chan", Kind: TypeBuiltin, construct: (*Checker).makeChan, resolve: (*Checker).resolveChan},
	)
	for _, t := range types.Vectors {
		list = append(list, &Builtin{Name: t.Name, Kind: TypeBuiltin, Type: t, Params: value, handler: vectorCaster(t)})
	}
	return list
}

func converter(to *types.Type) builtinHandler {
	return func(c *Checker, e *ast.Call, _ *types.Type) { c.convert(e, to) }
}

func vectorCaster(to *types.Type) builtinHandler {
	return func(c *Checker, e *ast.Call, _ *types.Type) { c.vectorCast(e, to) }
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
	e.Builtin = b.Name
	b.handler(c, e, want)
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

func isTypeName(name string) bool {
	b := builtinIndex[name]
	return b != nil && b.Kind == TypeBuiltin
}

func (c *Checker) callable(name string) bool {
	b := builtinIndex[name]
	return (b != nil && !b.Shadowable) || c.declared(name)
}

func (c *Checker) firstMut(fn ast.Expr) bool {
	switch fn := fn.(type) {
	case *ast.Ident:
		if b := c.builtinNamed(fn.Name); b != nil {
			return b.MutFirst
		}
		if c.find(fn.Name) != nil {
			return false
		}
		target, ok := c.local().funcs[fn.Name]
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
	case b.Type == nil:
		c.fail(t.Pos, "unknown type %s", t.Name)
	}
	return b.Type
}

func (c *Checker) resolveChan(t *ast.TypeName) *types.Type {
	if len(t.Args) != 1 {
		c.fail(t.Pos, "chan takes 1 type argument")
	}
	return c.chanOf(c.contained(t.Args[0], "a channel"))
}

func (c *Checker) noneConstant(e *ast.Ident, _ *types.Type) {
	c.fail(e.Pos, "cannot infer the type of none; annotate the binding")
}

func (c *Checker) nullConstant(e *ast.Ident, want *types.Type) {
	e.Null = true
	e.Type = c.raw(nil)
	if w := unwrap(want); w != nil && w.Kind == types.RawKind {
		e.Type = w
	}
}

func (c *Checker) bytesCall(e *ast.Call) {
	c.expectType(e.Args[0], types.Str)
	e.Type = c.slice(types.U8)
}

func (c *Checker) strCall(e *ast.Call) {
	e.Builtin, e.Type = "from_bytes", types.Str
	if t := c.plain(e.Args[0], nil); t.Kind == types.RawKind {
		c.cString(e.Args[0], t)
		e.Builtin = "from_cstr"
	} else if t != c.slice(types.U8) {
		c.fail(e.Args[0].Position(), "str takes []u8 or ptr[u8], found %s", t)
	}
}

func (c *Checker) cstrCall(e *ast.Call) {
	c.expectType(e.Args[0], types.Str)
	e.Type = c.raw(types.U8)
}

func (c *Checker) viewCall(e *ast.Call) {
	elem := c.typedRaw(e.Args[0], e.Builtin)
	c.integer(e.Args[1])
	e.Type = c.slice(elem)
}

func (c *Checker) loadCall(e *ast.Call) {
	e.Type = c.typedRaw(e.Args[0], e.Builtin)
}

func (c *Checker) storeCall(e *ast.Call) {
	c.expectType(e.Args[1], c.typedRaw(e.Args[0], e.Builtin))
	e.Type = types.Void
}

func (c *Checker) pinCall(e *ast.Call) {
	t := c.plain(e.Args[0], nil)
	if t.Kind != types.PointerKind && t.Kind != types.SliceKind && t.Kind != types.MapKind && t != types.Str {
		c.fail(e.Args[0].Position(), "pin takes a pointer, slice, map or str, found %s", t)
	}
	e.Type = c.raw(nil)
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
		e.Type = c.raw(types.U8)
	case t.Kind == types.SliceKind:
		e.Type = c.raw(t.Elem)
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
	e.Type = c.raw(types.U8)
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
	c.byteBuffer(e)
	c.expectType(e.Args[1], types.Str)
	e.Type = types.Void
}

func (c *Checker) extendByCall(e *ast.Call) {
	c.byteBuffer(e)
	c.expectType(e.Args[1], types.I64)
	e.Type = types.I64
}

func (c *Checker) byteBuffer(e *ast.Call) {
	r, ok := e.Args[0].(*ast.Ref)
	if !ok || c.borrowInPlace(r) != c.slice(types.U8) {
		c.fail(e.Pos, "%s needs a mut []u8", e.Builtin)
	}
}

func (c *Checker) strViewCall(e *ast.Call) {
	c.expectType(e.Args[0], c.slice(types.U8))
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
	e.Type = c.raw(nil)
}

func (c *Checker) parkCall(e *ast.Call) {
	e.Type = types.Void
}

func (c *Checker) readyCall(e *ast.Call) {
	c.expectType(e.Args[0], c.raw(nil))
	e.Type = types.Void
}

func (c *Checker) lockCall(e *ast.Call) {
	c.lockWord(e.Args[0], e.Builtin)
	e.Type = types.Void
}

func (c *Checker) casCall(e *ast.Call) {
	c.lockWord(e.Args[0], e.Builtin)
	c.expectType(e.Args[1], types.I64)
	c.expectType(e.Args[2], types.I64)
	e.Type = types.Bool
}

func (c *Checker) panicCall(e *ast.Call) {
	c.expectType(e.Args[0], types.Str)
	e.Type = types.Never
}
