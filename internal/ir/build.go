package ir

import (
	"fmt"
	"math"
	"slices"
	"strings"

	"lowlang/internal/ast"
	"lowlang/internal/lexer"
	"lowlang/internal/target"
	"lowlang/internal/types"
)

type loopTarget struct {
	cont  *Block
	brk   *Block
	depth int
}

type place struct {
	local  *ast.Local
	word   int
	addr   *Value
	offset int
	scalar *types.Type
	tail   int
}

type builder struct {
	ssa
	os       target.OS
	cur      *Block
	decl     *ast.Func
	funcs    map[*ast.Func]*Func
	adapters map[*ast.Func]*Func
	named    map[string]*Func
	slots    map[*ast.Local]*Value
	refs     map[*ast.Local]*Value
	loops    []loopTarget
	temps    int
	defers   [][]ast.Stmt
	subjects [][]*Value
}

func Build(files []*ast.File, os target.OS) *Program {
	var decls []*ast.Func
	for _, file := range files {
		for _, d := range file.Funcs {
			if d.TypeParams == nil {
				decls = append(decls, d)
			}
		}
	}
	p := &Program{OS: os}
	funcs := map[*ast.Func]*Func{}
	adapters := map[*ast.Func]*Func{}
	named := map[string]*Func{}
	for _, d := range decls {
		f := &Func{Name: symbolName(d), ParamWords: paramWords(d), ResultWords: d.RetType.Size(), Ret: d.RetType, Callback: d.Callback}
		if d.Callback {
			f.CParams = d.ParamTypes
		}
		funcs[d] = f
		named[d.Name] = f
		p.Funcs = append(p.Funcs, f)
		if d.AsValue {
			adapter := &Func{Name: fmt.Sprintf("%s..value", f.Name), ParamWords: 1 + f.ParamWords, ResultWords: f.ResultWords, Ret: f.Ret}
			adapters[d] = adapter
			p.Funcs = append(p.Funcs, adapter)
		}
		if d.Name == "main" && d.File.Module == "" {
			p.Main = f
			if d.RetType.Kind == types.ResultKind {
				p.MainErr = d.RetType.Err
			}
		}
	}
	builderFor := func(f *Func, d *ast.Func) *builder {
		return &builder{
			ssa:      newSSA(f),
			os:       os,
			decl:     d,
			funcs:    funcs,
			adapters: adapters,
			named:    named,
			slots:    map[*ast.Local]*Value{},
			refs:     map[*ast.Local]*Value{},
		}
	}
	for _, d := range decls {
		b := builderFor(funcs[d], d)
		b.function()
		b.finalize()
	}
	for d, f := range adapters {
		b := builderFor(f, d)
		b.adapter(funcs[d])
		b.finalize()
	}
	p.Funcs = reachable(p.Main, p.Funcs)
	p.CSymbols = cSymbols(p.Funcs)
	p.Libraries = libraries(files, p.CSymbols)
	p.Imports = imports(files, p.CSymbols)
	return p
}

func cSymbols(funcs []*Func) []string {
	var symbols []string
	for _, f := range funcs {
		for _, b := range f.Blocks {
			for _, v := range b.Values {
				switch {
				case v.Op == OpCCall && v.Plan.Symbol != "":
					symbols = append(symbols, v.Plan.Symbol)
				case v.Op == OpFuncAddr && v.Aux == SymbolAddr:
					symbols = append(symbols, v.Str)
				}
			}
		}
	}
	slices.Sort(symbols)
	return slices.Compact(symbols)
}

func imports(files []*ast.File, used []string) map[string][]string {
	byLibrary := map[string][]string{}
	for _, f := range files {
		for _, e := range f.Externs {
			for _, fn := range e.Funcs {
				if _, found := slices.BinarySearch(used, fn.Symbol); found {
					byLibrary[e.Library] = append(byLibrary[e.Library], fn.Symbol)
				}
			}
		}
	}
	for lib, symbols := range byLibrary {
		slices.Sort(symbols)
		byLibrary[lib] = slices.Compact(symbols)
	}
	return byLibrary
}

func libraries(files []*ast.File, used []string) []string {
	var libs []string
	for _, f := range files {
		for _, e := range f.Externs {
			if !slices.Contains(libs, e.Library) && declaresAny(e, used) {
				libs = append(libs, e.Library)
			}
		}
	}
	slices.Sort(libs)
	return libs
}

func declaresAny(e *ast.Extern, used []string) bool {
	return slices.ContainsFunc(e.Funcs, func(fn *ast.Func) bool {
		_, found := slices.BinarySearch(used, fn.Symbol)
		return found
	})
}

func reachable(main *Func, funcs []*Func) []*Func {
	seen := map[*Func]bool{}
	var visit func(f *Func)
	visit = func(f *Func) {
		if seen[f] {
			return
		}
		seen[f] = true
		for _, b := range f.Blocks {
			for _, v := range b.Values {
				if (v.Op == OpCall && !v.Indirect) || (v.Op == OpFuncAddr && v.Target != nil) {
					visit(v.Target)
				}
			}
		}
	}
	visit(main)
	var kept []*Func
	for _, f := range funcs {
		if seen[f] {
			kept = append(kept, f)
		}
	}
	return kept
}

func symbolName(d *ast.Func) string {
	if d.File.Module == "" {
		return d.Name
	}
	return fmt.Sprintf("%s.%s", strings.ReplaceAll(d.File.Module, "/", "."), d.Name)
}

func paramWords(d *ast.Func) int {
	words := 0
	if d.Closure != nil {
		words = 1
	}
	for i := range d.Params {
		words += d.ParamSize(i)
	}
	return words
}

func (b *builder) function() {
	b.cur = b.fn.newBlock()
	b.seal(b.cur)
	word := 0
	if b.decl.Closure != nil {
		b.environment(b.decl.Closure.Captures)
		word = 1
	}
	for i, local := range b.decl.ParamLocals {
		if b.decl.Params[i].Mut {
			b.refs[local] = b.param(word)
			word++
			continue
		}
		words := make([]*Value, local.Type.Size())
		for k := range words {
			words[k] = b.param(word + k)
		}
		classify(words, local.Type)
		b.bind(local, words)
		word += len(words)
	}
	body := b.expr(b.decl.Body)
	b.ret(b.decl.Body.Type, body)
}

func (b *builder) environment(captures []*ast.Local) {
	env := b.param(0)
	for i, local := range captures {
		box := b.emit(OpLoad, env)
		box.Aux = int64(8 * i)
		b.slots[local] = box
	}
}

func (b *builder) adapter(target *Func) {
	b.cur = b.fn.newBlock()
	b.seal(b.cur)
	var args []*Value
	word := 1
	for i, t := range b.decl.ParamTypes {
		words := make([]*Value, b.decl.ParamSize(i))
		for k := range words {
			words[k] = b.param(word + k)
		}
		if !b.decl.Params[i].Mut {
			classify(words, t)
		}
		args = append(args, words...)
		word += len(words)
	}
	b.cur.Kind = Ret
	b.cur.Results = b.invoke(target, args)
}

func (b *builder) param(word int) *Value {
	v := b.emit(OpParam)
	v.Aux = int64(word)
	return v
}

func (b *builder) emit(op Op, args ...*Value) *Value {
	v := b.fn.newValue(b.cur, op, args...)
	b.cur.Values = append(b.cur.Values, v)
	return v
}

func (b *builder) constant(n int64) *Value {
	v := b.emit(OpConst)
	v.Aux = n
	v.Scalar = true
	return v
}

func classify(words []*Value, t *types.Type) {
	for k, pointer := range t.PointerWords() {
		words[k].Scalar = !pointer
	}
}

func (b *builder) result(call *Value, k int) *Value {
	v := b.emit(OpResult, call)
	v.Aux = int64(k)
	return v
}

func (b *builder) newBlock() *Block {
	return b.fn.newBlock()
}

func link(from, to *Block) {
	from.Succs = append(from.Succs, to)
	to.Preds = append(to.Preds, from)
}

func (b *builder) jump(to *Block) {
	b.cur.Kind = Plain
	link(b.cur, to)
}

func (b *builder) branch(cond *Value, then, otherwise *Block) {
	b.cur.Kind = If
	b.cur.Control = cond
	link(b.cur, then)
	link(b.cur, otherwise)
}

func (b *builder) enter(block *Block) {
	b.seal(block)
	b.cur = block
}

func (b *builder) dead() {
	b.enter(b.newBlock())
}

func (b *builder) newTemp() int {
	b.temps++
	return b.temps
}

func (b *builder) writeTemp(temp int, words []*Value) {
	for k, w := range words {
		b.write(varKey{temp: temp, word: k}, b.cur, w)
	}
}

func (b *builder) readTemp(temp, n int) []*Value {
	words := make([]*Value, n)
	for k := range words {
		words[k] = b.read(varKey{temp: temp, word: k}, b.cur)
	}
	return words
}

func (b *builder) pad(words []*Value, n int) []*Value {
	for len(words) < n {
		words = append(words, b.emit(OpUndef))
	}
	return words[:n]
}

func (b *builder) fit(to *types.Type, words []*Value) []*Value {
	return b.pad(words, to.Size())
}

func (b *builder) value(e ast.Expr, want *types.Type) []*Value {
	return b.coerce(e.TypeOf(), want, b.expr(e))
}

func (b *builder) coerce(from, to *types.Type, words []*Value) []*Value {
	if to.Optional && from == to.Elem {
		words = append([]*Value{b.constant(1)}, words...)
	}
	return b.fit(to, words)
}

func (b *builder) word(e ast.Expr) *Value {
	return b.pad(b.expr(e), 1)[0]
}

func (b *builder) bind(local *ast.Local, words []*Value) {
	if local.Captured {
		box := b.runtime("alloc", b.constant(int64(len(words))), b.scan(local.Type))
		b.slots[local] = box
		b.store(place{addr: box}, words)
		return
	}
	if local.Addressed || isC(local.Type) || isVector(local.Type) {
		slot := b.emit(OpSlot)
		slot.Aux = int64(len(b.fn.Slots))
		b.fn.Slots = append(b.fn.Slots, len(words))
		b.slots[local] = slot
		b.store(place{addr: slot}, words)
		return
	}
	b.store(place{local: local}, words)
}

func (b *builder) place(p *ast.Place) place {
	switch {
	case p.Ptr != nil:
		return place{addr: b.word(p.Ptr), offset: p.Offset, scalar: p.Width, tail: p.Tail}
	case p.Index != nil:
		element := b.element(p.Index)
		element.offset += p.Offset
		element.tail = p.Tail
		if p.Width != nil {
			element.scalar = p.Width
		}
		return element
	case p.Local.Ref:
		return place{addr: b.refs[p.Local], offset: p.Offset, scalar: p.Width, tail: p.Tail}
	}
	if slot, ok := b.slots[p.Local]; ok {
		return place{addr: slot, offset: p.Offset, scalar: p.Width, tail: p.Tail}
	}
	return place{local: p.Local, word: p.Offset / 8}
}

func (b *builder) load(p place, t *types.Type) []*Value {
	words := make([]*Value, t.Size())
	for k := range words {
		if p.addr == nil {
			words[k] = b.read(varKey{local: p.local, word: p.word + k}, b.cur)
			continue
		}
		if p.tail != 0 && k == len(words)-1 {
			words[k] = b.loadTail(p.addr, p.offset+8*k, p.tail)
			continue
		}
		v := b.typed(OpLoad, p.scalar, p.addr)
		v.Aux = int64(p.offset + 8*k)
		v.Scalar = !t.PointerWords()[k]
		words[k] = v
	}
	return words
}

var tailWidths = []struct {
	bytes int
	width *types.Type
}{{4, types.U32}, {2, types.U16}, {1, types.U8}}

func (b *builder) loadTail(addr *Value, offset, tail int) *Value {
	var acc *Value
	at := 0
	for _, part := range tailWidths {
		if tail-at < part.bytes {
			continue
		}
		v := b.typed(OpLoad, part.width, addr)
		v.Aux = int64(offset + at)
		if at > 0 {
			v = b.typed(OpShl, types.I64, v, b.constant(int64(8*at)))
		}
		if acc == nil {
			acc = v
		} else {
			acc = b.typed(OpOr, types.I64, acc, v)
		}
		at += part.bytes
	}
	return acc
}

func (b *builder) storeTail(addr *Value, offset, tail int, word *Value) {
	at := 0
	for _, part := range tailWidths {
		if tail-at < part.bytes {
			continue
		}
		v := word
		if at > 0 {
			v = b.typed(OpShr, types.I64, word, b.constant(int64(8*at)))
		}
		store := b.typed(OpStore, part.width, addr, v)
		store.Aux = int64(offset + at)
		at += part.bytes
	}
}

func (b *builder) store(p place, words []*Value) {
	for k, w := range words {
		if p.addr == nil {
			b.write(varKey{local: p.local, word: p.word + k}, b.cur, w)
			continue
		}
		if p.tail != 0 && k == len(words)-1 {
			b.storeTail(p.addr, p.offset+8*k, p.tail, w)
			continue
		}
		v := b.typed(OpStore, p.scalar, p.addr, w)
		v.Aux = int64(p.offset + 8*k)
	}
}

func (b *builder) address(p place) *Value {
	if p.offset == 0 {
		return p.addr
	}
	return b.emit(OpAdd, p.addr, b.constant(int64(p.offset)))
}

func (b *builder) ret(t *types.Type, words []*Value) {
	ret := b.decl.RetType
	var results []*Value
	switch {
	case ret == types.Void:
	case t == ret:
		results = b.fit(ret, words)
	case ret.Kind == types.ResultKind:
		results = []*Value{b.constant(0)}
		if ret.Ok.Accepts(t) {
			results = append(results, b.coerce(t, ret.Ok, words)...)
		}
		results = b.pad(results, ret.Size())
	case ret.Accepts(t):
		results = b.coerce(t, ret, words)
	default:
		results = b.pad(nil, ret.Size())
	}
	b.cur.Kind = Ret
	b.cur.Results = results
}

func (b *builder) fail(errWords []*Value) {
	results := append([]*Value{b.constant(1)}, errWords...)
	b.cur.Kind = Ret
	b.cur.Results = b.pad(results, b.decl.RetType.Size())
}

func (b *builder) block(blk *ast.Block) []*Value {
	b.defers = append(b.defers, nil)
	for _, s := range blk.Stmts {
		b.stmt(s)
	}
	var words []*Value
	if blk.Tail != nil {
		words = b.expr(blk.Tail)
	}
	b.runDefers(len(b.defers) - 1)
	b.defers = b.defers[:len(b.defers)-1]
	return words
}

func (b *builder) runDefers(from int) {
	for i := len(b.defers) - 1; i >= from; i-- {
		for j := len(b.defers[i]) - 1; j >= 0; j-- {
			b.stmt(b.defers[i][j])
		}
	}
}

func (b *builder) stmt(s ast.Stmt) {
	switch s := s.(type) {
	case *ast.Let:
		if isVector(s.Local.Type) && !s.Local.Captured {
			b.bindVector(s.Local, s.Value)
		} else {
			b.bind(s.Local, b.value(s.Value, s.Local.Type))
		}
		if s.Flag != nil {
			b.bind(s.Flag, []*Value{b.constant(1)})
		}
		if s.Release != nil {
			top := len(b.defers) - 1
			b.defers[top] = append(b.defers[top], s.Release)
		}
	case *ast.Assign:
		b.assign(s)
	case *ast.ExprStmt:
		b.expr(s.X)
	case *ast.For:
		b.forStmt(s)
	case *ast.ForIn:
		b.forIn(s)
	case *ast.Break:
		loop := b.loops[len(b.loops)-1-s.Depth]
		b.runDefers(loop.depth)
		b.jump(loop.brk)
		b.dead()
	case *ast.Continue:
		loop := b.loops[len(b.loops)-1-s.Depth]
		b.runDefers(loop.depth)
		b.jump(loop.cont)
		b.dead()
	case *ast.Return:
		t := types.Void
		var words []*Value
		if s.Value != nil {
			t = s.Value.TypeOf()
			words = b.expr(s.Value)
		}
		b.runDefers(0)
		b.ret(t, words)
		b.dead()
	case *ast.Fail:
		words := b.value(s.Value, s.Value.TypeOf())
		b.runDefers(0)
		b.fail(words)
		b.dead()
	case *ast.Spawn:
		b.block(s.Body)
	case *ast.Send:
		b.send(s)
	case *ast.Defer:
		top := len(b.defers) - 1
		b.defers[top] = append(b.defers[top], s.Body)
	}
}

var arith = map[lexer.Kind]Op{
	lexer.Plus: OpAdd, lexer.Minus: OpSub, lexer.Star: OpMul, lexer.Slash: OpDiv, lexer.Percent: OpRem,
	lexer.Amp: OpAnd, lexer.Pipe: OpOr, lexer.Caret: OpXor, lexer.Shl: OpShl, lexer.Shr: OpShr,
	lexer.PlusAssign: OpAdd, lexer.MinusAssign: OpSub, lexer.StarAssign: OpMul, lexer.SlashAssign: OpDiv,
	lexer.PercentAssign: OpRem, lexer.AmpAssign: OpAnd, lexer.PipeAssign: OpOr, lexer.CaretAssign: OpXor,
	lexer.ShlAssign: OpShl, lexer.ShrAssign: OpShr,
	lexer.Eq: OpEq, lexer.NotEq: OpNe, lexer.Lt: OpLt, lexer.LtEq: OpLe, lexer.Gt: OpGt, lexer.GtEq: OpGe,
}

func (b *builder) typed(op Op, t *types.Type, args ...*Value) *Value {
	v := b.emit(op, args...)
	v.Type = t
	v.Scalar = t != nil && !t.HasPointers()
	return v
}

func (b *builder) assign(s *ast.Assign) {
	if s.Entry != nil {
		b.assignEntry(s)
		return
	}
	if isVector(s.Target.TypeOf()) {
		b.assignVector(s)
		return
	}
	t := s.Target.TypeOf()
	source := t
	if s.Op == lexer.ShlAssign || s.Op == lexer.ShrAssign {
		source = s.Value.TypeOf()
	}
	words := b.value(s.Value, source)
	if s.Release != nil {
		b.stmt(s.Release)
	}
	p := b.place(s.Place)
	if op, ok := arith[s.Op]; ok {
		old := b.load(p, t)[0]
		words = []*Value{b.typed(op, t, old, b.pad(words, 1)[0])}
	}
	b.store(p, b.fit(t, words))
	if s.Flag != nil {
		b.setFlag(s.Flag, 1)
	}
}

func (b *builder) setFlag(flag *ast.Local, live int64) {
	b.store(b.place(&ast.Place{Local: flag}), []*Value{b.constant(live)})
}

func (b *builder) forStmt(s *ast.For) {
	header := b.newBlock()
	body := b.newBlock()
	exit := b.newBlock()
	b.jump(header)
	b.cur = header
	if s.Cond != nil {
		b.branch(b.word(s.Cond), body, exit)
	} else {
		b.jump(body)
	}
	b.enter(body)
	latch := b.newBlock()
	b.loops = append(b.loops, loopTarget{cont: latch, brk: exit, depth: len(b.defers)})
	b.block(s.Body)
	b.loops = b.loops[:len(b.loops)-1]
	b.jump(latch)
	b.enter(latch)
	b.backEdge(header)
	b.seal(header)
	b.enter(exit)
}

func (b *builder) backEdge(header *Block) {
	b.cur.Poll = true
	b.jump(header)
}

func (b *builder) expr(e ast.Expr) []*Value {
	if t := e.TypeOf(); isVector(t) {
		if addr := b.vectorNode(e); addr != nil {
			return b.load(place{addr: addr}, t)
		}
	}
	switch e := e.(type) {
	case *ast.IntLit:
		if e.Type.Float() {
			return []*Value{b.constant(floatBits(e.Type, float64(e.Value)))}
		}
		return []*Value{b.constant(e.Value)}
	case *ast.FloatLit:
		return []*Value{b.constant(floatBits(e.Type, e.Value))}
	case *ast.BoolLit:
		if e.Value {
			return []*Value{b.constant(1)}
		}
		return []*Value{b.constant(0)}
	case *ast.StrLit:
		v := b.emit(OpString)
		v.Str = e.Value
		v.Scalar = true
		return []*Value{v, b.constant(int64(len(e.Value)))}
	case *ast.Index:
		if e.X.TypeOf().Kind == types.MapKind {
			return b.lookup(e)
		}
		p, _ := ast.PlaceOf(e)
		if p == nil {
			p = &ast.Place{Index: e, Tail: e.Type.Tail()}
		}
		return b.load(b.place(p), e.Type)
	case *ast.Receiver:
		return b.expr(e.X)
	case *ast.Slicing:
		return b.slicing(e)
	case *ast.SliceLit:
		return b.sliceLit(e)
	case *ast.MapLit:
		return b.mapLit(e)
	case *ast.Interp:
		return b.interpolate(e)
	case *ast.Ident:
		if e.Variant != nil {
			return b.construct(e.Type, e.Variant, nil)
		}
		if e.Null {
			return []*Value{b.constant(0)}
		}
		if e.Func != nil {
			return b.funcValue(e.Func, e.Type)
		}
		words := b.load(b.place(&ast.Place{Local: e.Local}), e.Type)
		if e.Moved {
			b.setFlag(e.Local.Flag, 0)
		}
		return words
	case *ast.Selector:
		if e.Func != nil {
			return b.funcValue(e.Func, e.Type)
		}
		if e.Reference != nil {
			return b.fieldReference(e)
		}
		return b.selector(e)
	case *ast.Closure:
		return b.closure(e)
	case *ast.StructLit:
		if isC(e.Type) {
			return b.cStruct(e)
		}
		words := make([]*Value, e.Type.Size())
		for _, init := range e.Fields {
			copy(words[init.Field.Offset:], b.value(init.Value, init.Field.Type))
		}
		return words
	case *ast.Unary:
		return b.unary(e)
	case *ast.Binary:
		return b.binary(e)
	case *ast.Call:
		return b.call(e)
	case *ast.If:
		return b.ifExpr(e)
	case *ast.Block:
		return b.block(e)
	case *ast.Match:
		return b.dispatch(e.Arms, b.value(e.X, e.X.TypeOf()), e.Type)
	case *ast.Select:
		return b.selectExpr(e)
	case *ast.Subject:
		return b.subjects[len(b.subjects)-1]
	case *ast.Try:
		return b.try(e)
	case *ast.Catch:
		return b.catch(e)
	}
	return nil
}

func (b *builder) selector(e *ast.Selector) []*Value {
	if e.Variant != nil {
		return b.construct(e.Type, e.Variant, nil)
	}
	if e.Length && e.X.TypeOf().Kind == types.MapKind {
		return b.mapCall("__map_len", e.X.TypeOf(), b.word(e.X))
	}
	if e.Length {
		return b.value(e.X, e.X.TypeOf())[1:2]
	}
	if p, ok := ast.PlaceOf(e); ok {
		return b.load(b.place(p), e.Type)
	}
	if isC(e.X.TypeOf()) {
		return b.cField(e)
	}
	words := b.value(e.X, e.X.TypeOf())
	return words[e.Field.Offset : e.Field.Offset+e.Type.Size()]
}

func (b *builder) unary(e *ast.Unary) []*Value {
	switch e.Op {
	case lexer.Star:
		return b.load(b.place(&ast.Place{Ptr: e.X}), e.Type)
	case lexer.Not:
		return []*Value{b.typed(OpNot, types.Bool, b.word(e.X))}
	case lexer.Tilde:
		return []*Value{b.typed(OpCompl, e.Type, b.word(e.X))}
	case lexer.LArrow:
		return b.receive(e)
	}
	return []*Value{b.typed(OpNeg, e.Type, b.word(e.X))}
}

func floatBits(t *types.Type, f float64) int64 {
	if t.Bits == 32 {
		return int64(math.Float32bits(float32(f)))
	}
	return int64(math.Float64bits(f))
}

func (b *builder) binary(e *ast.Binary) []*Value {
	if e.Op == lexer.AndAnd || e.Op == lexer.OrOr {
		return b.logical(e)
	}
	if e.Op == lexer.Coalesce {
		return b.coalesce(e)
	}
	if e.X.TypeOf() == types.Str {
		return b.strings(e)
	}
	if t := e.X.TypeOf(); aggregate(t) {
		x := b.value(e.X, t)
		y := b.value(e.Y, t)
		same := b.equal(t, x, y)
		if e.Op == lexer.NotEq {
			same = b.typed(OpNot, types.Bool, same)
		}
		return []*Value{same}
	}
	x := b.word(e.X)
	y := b.word(e.Y)
	return []*Value{b.typed(arith[e.Op], e.X.TypeOf(), x, y)}
}

func (b *builder) logical(e *ast.Binary) []*Value {
	temp := b.newTemp()
	rhs := b.newBlock()
	done := b.newBlock()
	x := b.word(e.X)
	b.writeTemp(temp, []*Value{x})
	if e.Op == lexer.AndAnd {
		b.branch(x, rhs, done)
	} else {
		b.branch(x, done, rhs)
	}
	b.enter(rhs)
	b.writeTemp(temp, []*Value{b.word(e.Y)})
	b.jump(done)
	b.enter(done)
	return b.readTemp(temp, 1)
}

func (b *builder) call(e *ast.Call) []*Value {
	switch {
	case e.Builtin == "new":
		return []*Value{b.allocate(e.Args[0])}
	case e.Builtin == "convert":
		return []*Value{b.convert(b.word(e.Args[0]), e.Args[0].TypeOf(), e.Type)}
	case e.Builtin == "sqrt":
		return []*Value{b.typed(OpSqrt, e.Type, b.word(e.Args[0]))}
	case e.Builtin == "floor":
		return []*Value{b.typed(OpFloor, e.Type, b.word(e.Args[0]))}
	case e.Builtin == "exp":
		return []*Value{b.exp(b.word(e.Args[0]), e.Type)}
	case e.Builtin != "":
		return b.builtin(e)
	case e.Variant != nil:
		return b.construct(e.Type, e.Variant, e.Args)
	case e.Target == nil:
		return b.callValue(e)
	}
	return b.callFunc(e)
}

func (b *builder) fieldReference(e *ast.Selector) []*Value {
	name := b.emit(OpString)
	name.Str = e.Reference.Name
	name.Scalar = true
	owner := e.Type.TypeArgs[0]
	index := 0
	for i, f := range owner.Fields {
		if f == e.Reference {
			index = i
		}
	}
	return []*Value{name, b.constant(int64(len(e.Reference.Name))), b.constant(int64(index))}
}

func (b *builder) funcAddr(kind int64, target *Func) *Value {
	v := b.emit(OpFuncAddr)
	v.Aux = kind
	v.Target = target
	v.Scalar = true
	return v
}

func (b *builder) funcValue(fn *ast.Func, t *types.Type) []*Value {
	switch {
	case fn.Extern != nil:
		v := b.funcAddr(SymbolAddr, nil)
		v.Str = fn.Symbol
		return []*Value{v}
	case t.C:
		return []*Value{b.funcAddr(ThunkAddr, b.funcs[fn])}
	}
	return []*Value{b.funcAddr(CodeAddr, b.adapters[fn]), b.constant(0)}
}

func (b *builder) closure(e *ast.Closure) []*Value {
	code := b.funcAddr(CodeAddr, b.funcs[e.Func])
	if len(e.Captures) == 0 {
		return []*Value{code, b.constant(0)}
	}
	env := b.runtime("alloc", b.constant(int64(len(e.Captures))), b.constant(1))
	for i, local := range e.Captures {
		b.store(place{addr: env, offset: 8 * i}, []*Value{b.slots[local]})
	}
	return []*Value{code, env}
}

func (b *builder) arguments(args []ast.Expr, params []*types.Type) []*Value {
	var words []*Value
	for i, arg := range args {
		if r, ok := arg.(*ast.Ref); ok {
			words = append(words, b.address(b.place(r.Place)))
			continue
		}
		words = append(words, b.value(arg, params[i])...)
	}
	return words
}

func (b *builder) callValue(e *ast.Call) []*Value {
	t := e.Fn.TypeOf()
	callee := b.value(e.Fn, t)
	if t.C {
		return b.cInvoke("", t.Params, t.Ret, callee, e.Args)
	}
	args := b.arguments(e.Args, t.Params)
	signature := &Func{Name: t.Name, ParamWords: 1 + len(args), ResultWords: t.Ret.Size(), Ret: t.Ret}
	operands := append(callee, args...)
	call := b.emit(OpCall, operands...)
	call.Indirect = true
	call.Target = signature
	return b.results(call, signature)
}

func (b *builder) convert(x *Value, from, to *types.Type) *Value {
	if from == to {
		return x
	}
	v := b.typed(OpConvert, to, x)
	v.From = from
	return v
}

func (b *builder) exp(x *Value, t *types.Type) *Value {
	call := b.emit(OpRuntime, b.convert(x, t, types.F64))
	call.Str = "exp"
	result := b.result(call, 0)
	result.Scalar = true
	return b.convert(result, types.F64, t)
}

func (b *builder) allocate(arg ast.Expr) *Value {
	t := arg.TypeOf()
	words := b.value(arg, t)
	ptr := b.runtime("alloc", b.constant(int64(t.Size())), b.scan(t))
	b.store(place{addr: ptr}, words)
	return ptr
}

func (b *builder) scan(t *types.Type) *Value {
	if t.HasPointers() {
		return b.constant(1)
	}
	return b.constant(0)
}

func (b *builder) runtime(name string, args ...*Value) *Value {
	call := b.emit(OpRuntime, args...)
	call.Str = name
	return b.result(call, 0)
}

func (b *builder) construct(enum *types.Type, v *types.Variant, args []ast.Expr) []*Value {
	if enum.Scalar() {
		return []*Value{b.constant(int64(v.Tag))}
	}
	words := make([]*Value, enum.Size())
	words[0] = b.constant(int64(v.Tag))
	for i, arg := range args {
		copy(words[v.Offsets[i]:], b.value(arg, v.Fields[i]))
	}
	for k, w := range words {
		if w == nil {
			words[k] = b.emit(OpUndef)
		}
	}
	return words
}

func (b *builder) callFunc(e *ast.Call) []*Value {
	if e.Target.Extern != nil {
		return b.cCall(e)
	}
	args := b.arguments(e.Args, e.Target.ParamTypes)
	return b.invoke(b.funcs[e.Target], args)
}

func (b *builder) invoke(target *Func, args []*Value) []*Value {
	call := b.emit(OpCall, args...)
	call.Target = target
	return b.results(call, target)
}

func (b *builder) results(call *Value, target *Func) []*Value {
	results := make([]*Value, target.ResultWords)
	for k := range results {
		results[k] = b.result(call, k)
	}
	classify(results, target.Ret)
	return results
}

func (b *builder) ifExpr(e *ast.If) []*Value {
	temp := b.newTemp()
	then := b.newBlock()
	otherwise := b.newBlock()
	done := b.newBlock()
	b.branch(b.word(e.Cond), then, otherwise)
	b.enter(then)
	b.writeTemp(temp, b.value(e.Then, e.Type))
	b.jump(done)
	b.enter(otherwise)
	if e.Else != nil {
		b.writeTemp(temp, b.value(e.Else, e.Type))
	}
	b.jump(done)
	b.enter(done)
	return b.readTemp(temp, e.Type.Size())
}

func (b *builder) dispatch(arms []*ast.Arm, subject []*Value, result *types.Type) []*Value {
	temp := b.newTemp()
	done := b.newBlock()
	targets := make([]*Block, len(arms))
	for i, arm := range arms {
		targets[i] = b.newBlock()
		last := i == len(arms)-1
		if (arm.Pattern.Variant == nil && arm.Pattern.Test == nil) || last {
			b.jump(targets[i])
			break
		}
		next := b.newBlock()
		match := b.armMatches(arm.Pattern, subject)
		b.branch(match, targets[i], next)
		b.enter(next)
	}
	for i, arm := range arms {
		if targets[i] == nil {
			break
		}
		b.enter(targets[i])
		b.bindPattern(arm.Pattern, subject)
		b.writeTemp(temp, b.value(arm.Body, result))
		b.jump(done)
	}
	b.enter(done)
	return b.readTemp(temp, result.Size())
}

func (b *builder) armMatches(p *ast.Pattern, subject []*Value) *Value {
	if p.Test == nil {
		return b.emit(OpEq, subject[0], b.constant(int64(p.Variant.Tag)))
	}
	b.subjects = append(b.subjects, subject)
	match := b.word(p.Test)
	b.subjects = b.subjects[:len(b.subjects)-1]
	return match
}

func (b *builder) bindPattern(p *ast.Pattern, subject []*Value) {
	if p.Local != nil {
		b.bind(p.Local, subject[:p.Local.Type.Size()])
	}
	for i, binding := range p.Bindings {
		if binding.Local == nil {
			continue
		}
		offset := p.Variant.Offsets[i]
		b.bind(binding.Local, subject[offset:offset+binding.Local.Type.Size()])
	}
}

func (b *builder) try(e *ast.Try) []*Value {
	r := e.X.TypeOf()
	words := b.value(e.X, r)
	ok := b.newBlock()
	failed := b.newBlock()
	b.branch(words[0], failed, ok)
	b.enter(failed)
	b.runDefers(0)
	b.fail(words[1 : 1+r.Err.Size()])
	b.enter(ok)
	return words[1 : 1+r.Ok.Size()]
}

func (b *builder) catch(e *ast.Catch) []*Value {
	r := e.X.TypeOf()
	words := b.value(e.X, r)
	temp := b.newTemp()
	ok := b.newBlock()
	failed := b.newBlock()
	done := b.newBlock()
	b.branch(words[0], failed, ok)
	b.enter(ok)
	b.writeTemp(temp, words[1:1+r.Ok.Size()])
	b.jump(done)
	b.enter(failed)
	if e.Fallback != nil {
		b.writeTemp(temp, b.value(e.Fallback, e.Type))
	} else {
		b.writeTemp(temp, b.dispatch(e.Arms, words[1:1+r.Err.Size()], e.Type))
	}
	b.jump(done)
	b.enter(done)
	return b.readTemp(temp, e.Type.Size())
}
