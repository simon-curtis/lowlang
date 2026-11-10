package ir

import (
	"fmt"
	"math"
	"strings"

	"lowlang/internal/ast"
	"lowlang/internal/lexer"
	"lowlang/internal/types"
)

type loopTarget struct {
	cont *Block
	brk  *Block
}

type place struct {
	local  *ast.Local
	word   int
	addr   *Value
	offset int
	scalar *types.Type
}

type builder struct {
	ssa
	cur   *Block
	decl  *ast.Func
	funcs map[*ast.Func]*Func
	named map[string]*Func
	slots map[*ast.Local]*Value
	refs  map[*ast.Local]*Value
	loops []loopTarget
	temps int
}

func Build(files []*ast.File) *Program {
	var decls []*ast.Func
	for _, file := range files {
		decls = append(decls, file.Funcs...)
	}
	p := &Program{}
	funcs := map[*ast.Func]*Func{}
	named := map[string]*Func{}
	for _, d := range decls {
		f := &Func{Name: symbolName(d), ParamWords: paramWords(d), ResultWords: d.RetType.Size(), Ret: d.RetType}
		funcs[d] = f
		named[d.Name] = f
		p.Funcs = append(p.Funcs, f)
		if d.Name == "main" && d.File.Module == "" {
			p.Main = f
			if d.RetType.Kind == types.ResultKind {
				p.MainErr = d.RetType.Err
			}
		}
	}
	for _, d := range decls {
		b := &builder{
			ssa:   newSSA(funcs[d]),
			decl:  d,
			funcs: funcs,
			named: named,
			slots: map[*ast.Local]*Value{},
			refs:  map[*ast.Local]*Value{},
		}
		b.function()
		b.finalize()
	}
	p.Funcs = reachable(p.Main, p.Funcs)
	return p
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
				if v.Op == OpCall {
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
	for i := range d.Params {
		words += d.ParamSize(i)
	}
	return words
}

func (b *builder) function() {
	b.cur = b.fn.newBlock()
	b.seal(b.cur)
	word := 0
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
	if local.Addressed {
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
		return place{addr: b.word(p.Ptr), offset: p.Offset}
	case p.Index != nil:
		element := b.element(p.Index)
		element.offset += p.Offset
		return element
	case p.Local.Ref:
		return place{addr: b.refs[p.Local], offset: p.Offset}
	}
	if slot, ok := b.slots[p.Local]; ok {
		return place{addr: slot, offset: p.Offset}
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
		v := b.typed(OpLoad, p.scalar, p.addr)
		v.Aux = int64(p.offset + 8*k)
		v.Scalar = !t.PointerWords()[k]
		words[k] = v
	}
	return words
}

func (b *builder) store(p place, words []*Value) {
	for k, w := range words {
		if p.addr == nil {
			b.write(varKey{local: p.local, word: p.word + k}, b.cur, w)
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
	for _, s := range blk.Stmts {
		b.stmt(s)
	}
	if blk.Tail == nil {
		return nil
	}
	return b.expr(blk.Tail)
}

func (b *builder) stmt(s ast.Stmt) {
	switch s := s.(type) {
	case *ast.Let:
		b.bind(s.Local, b.value(s.Value, s.Local.Type))
	case *ast.Assign:
		b.assign(s)
	case *ast.ExprStmt:
		b.expr(s.X)
	case *ast.For:
		b.forStmt(s)
	case *ast.ForIn:
		b.forIn(s)
	case *ast.Break:
		b.jump(b.loops[len(b.loops)-1].brk)
		b.dead()
	case *ast.Continue:
		b.jump(b.loops[len(b.loops)-1].cont)
		b.dead()
	case *ast.Return:
		t := types.Void
		var words []*Value
		if s.Value != nil {
			t = s.Value.TypeOf()
			words = b.expr(s.Value)
		}
		b.ret(t, words)
		b.dead()
	case *ast.Fail:
		b.fail(b.value(s.Value, s.Value.TypeOf()))
		b.dead()
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
	t := s.Target.TypeOf()
	source := t
	if s.Op == lexer.ShlAssign || s.Op == lexer.ShrAssign {
		source = s.Value.TypeOf()
	}
	words := b.value(s.Value, source)
	p := b.place(s.Place)
	if op, ok := arith[s.Op]; ok {
		old := b.load(p, t)[0]
		words = []*Value{b.typed(op, t, old, b.pad(words, 1)[0])}
	}
	b.store(p, b.fit(t, words))
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
	b.loops = append(b.loops, loopTarget{cont: header, brk: exit})
	b.block(s.Body)
	b.loops = b.loops[:len(b.loops)-1]
	b.jump(header)
	b.seal(header)
	b.enter(exit)
}

func (b *builder) expr(e ast.Expr) []*Value {
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
			p = &ast.Place{Index: e}
		}
		return b.load(b.place(p), e.Type)
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
		return b.load(b.place(&ast.Place{Local: e.Local}), e.Type)
	case *ast.Selector:
		return b.selector(e)
	case *ast.StructLit:
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
		return b.callNamed("__map_len", b.word(e.X))
	}
	if e.Length {
		return b.value(e.X, e.X.TypeOf())[1:2]
	}
	if p, ok := ast.PlaceOf(e); ok {
		return b.load(b.place(p), e.Type)
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
	}
	return b.callFunc(e)
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
	target := b.funcs[e.Target]
	var args []*Value
	for i, arg := range e.Args {
		if r, ok := arg.(*ast.Ref); ok {
			args = append(args, b.address(b.place(r.Place)))
			continue
		}
		args = append(args, b.value(arg, e.Target.ParamTypes[i])...)
	}
	call := b.emit(OpCall, args...)
	call.Target = target
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
		if arm.Pattern.Variant == nil || last {
			b.jump(targets[i])
			break
		}
		next := b.newBlock()
		match := b.emit(OpEq, subject[0], b.constant(int64(arm.Pattern.Variant.Tag)))
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

func (b *builder) bindPattern(p *ast.Pattern, subject []*Value) {
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
