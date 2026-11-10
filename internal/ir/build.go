package ir

import (
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
}

type builder struct {
	ssa
	cur   *Block
	decl  *ast.Func
	funcs map[*ast.Func]*Func
	slots map[*ast.Local]*Value
	refs  map[*ast.Local]*Value
	loops []loopTarget
	temps int
}

func Build(file *ast.File) *Program {
	p := &Program{}
	funcs := map[*ast.Func]*Func{}
	for _, d := range file.Funcs {
		f := &Func{Name: d.Name, ParamWords: paramWords(d), ResultWords: d.RetType.Size()}
		funcs[d] = f
		p.Funcs = append(p.Funcs, f)
		if d.Name == "main" {
			p.Main = f
			if d.RetType.Kind == types.ResultKind {
				p.MainErr = d.RetType.Err
			}
		}
	}
	for _, d := range file.Funcs {
		b := &builder{
			ssa:   newSSA(funcs[d]),
			decl:  d,
			funcs: funcs,
			slots: map[*ast.Local]*Value{},
			refs:  map[*ast.Local]*Value{},
		}
		b.function()
		b.finalize()
	}
	return p
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
	return v
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
	return b.fit(want, b.expr(e))
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
	case p.Local.Ref:
		return place{addr: b.refs[p.Local], offset: p.Offset}
	}
	if slot, ok := b.slots[p.Local]; ok {
		return place{addr: slot, offset: p.Offset}
	}
	return place{local: p.Local, word: p.Offset / 8}
}

func (b *builder) load(p place, n int) []*Value {
	words := make([]*Value, n)
	for k := range words {
		if p.addr == nil {
			words[k] = b.read(varKey{local: p.local, word: p.word + k}, b.cur)
			continue
		}
		v := b.emit(OpLoad, p.addr)
		v.Aux = int64(p.offset + 8*k)
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
		v := b.emit(OpStore, p.addr, w)
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
		if t == ret.Ok {
			results = append(results, words...)
		}
		results = b.pad(results, ret.Size())
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
	lexer.PlusAssign: OpAdd, lexer.MinusAssign: OpSub, lexer.StarAssign: OpMul, lexer.SlashAssign: OpDiv,
	lexer.Eq: OpEq, lexer.NotEq: OpNe, lexer.Lt: OpLt, lexer.LtEq: OpLe, lexer.Gt: OpGt, lexer.GtEq: OpGe,
}

func (b *builder) assign(s *ast.Assign) {
	t := s.Target.TypeOf()
	words := b.value(s.Value, t)
	p := b.place(s.Place)
	if op, ok := arith[s.Op]; ok {
		old := b.load(p, 1)[0]
		words = []*Value{b.emit(op, old, words[0])}
	}
	b.store(p, words)
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
		return []*Value{b.constant(e.Value)}
	case *ast.BoolLit:
		if e.Value {
			return []*Value{b.constant(1)}
		}
		return []*Value{b.constant(0)}
	case *ast.StrLit:
		v := b.emit(OpString)
		v.Str = e.Value
		return []*Value{v}
	case *ast.Ident:
		if e.Variant != nil {
			return b.construct(e.Type, e.Variant, nil)
		}
		return b.load(b.place(&ast.Place{Local: e.Local}), e.Type.Size())
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
	if p, ok := ast.PlaceOf(e); ok {
		return b.load(b.place(p), e.Type.Size())
	}
	words := b.value(e.X, e.X.TypeOf())
	return words[e.Field.Offset : e.Field.Offset+e.Type.Size()]
}

func (b *builder) unary(e *ast.Unary) []*Value {
	switch e.Op {
	case lexer.Star:
		return b.load(b.place(&ast.Place{Ptr: e.X}), e.Type.Size())
	case lexer.Not:
		return []*Value{b.emit(OpNot, b.word(e.X))}
	}
	return []*Value{b.emit(OpNeg, b.word(e.X))}
}

func (b *builder) binary(e *ast.Binary) []*Value {
	if e.Op == lexer.AndAnd || e.Op == lexer.OrOr {
		return b.logical(e)
	}
	x := b.word(e.X)
	y := b.word(e.Y)
	if e.X.TypeOf() != types.Str {
		return []*Value{b.emit(arith[e.Op], x, y)}
	}
	call := b.emit(OpRuntime, x, y)
	call.Str = "str_eq"
	equal := b.result(call, 0)
	if e.Op == lexer.NotEq {
		return []*Value{b.emit(OpNot, equal)}
	}
	return []*Value{equal}
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
	case e.Builtin != "":
		call := b.emit(OpRuntime, b.word(e.Args[0]))
		call.Str = e.Builtin
		return nil
	case e.Variant != nil:
		return b.construct(e.Type, e.Variant, e.Args)
	}
	return b.callFunc(e)
}

func (b *builder) allocate(arg ast.Expr) *Value {
	t := arg.TypeOf()
	words := b.value(arg, t)
	call := b.emit(OpRuntime, b.constant(int64(t.Size())))
	call.Str = "alloc"
	ptr := b.result(call, 0)
	b.store(place{addr: ptr}, words)
	return ptr
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
	return results
}

func (b *builder) ifExpr(e *ast.If) []*Value {
	temp := b.newTemp()
	then := b.newBlock()
	otherwise := b.newBlock()
	done := b.newBlock()
	b.branch(b.word(e.Cond), then, otherwise)
	b.enter(then)
	b.writeTemp(temp, b.fit(e.Type, b.block(e.Then)))
	b.jump(done)
	b.enter(otherwise)
	if e.Else != nil {
		b.writeTemp(temp, b.fit(e.Type, b.expr(e.Else)))
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
		b.writeTemp(temp, b.fit(result, b.expr(arm.Body)))
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
