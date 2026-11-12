package ir

import (
	"fmt"

	"lowlang/internal/ast"
	"lowlang/internal/lexer"
	"lowlang/internal/prelude"
	"lowlang/internal/types"
)

func elementType(t *types.Type) *types.Type {
	if t == types.Str {
		return types.U8
	}
	return t.Elem
}

func (b *builder) check(index, limit *Value) {
	b.emit(OpCheck, index, limit)
}

func (b *builder) scaled(base, index *Value, stride int) *Value {
	offset := index
	if stride != 1 {
		offset = b.emit(OpMul, index, b.constant(int64(stride)))
	}
	return b.emit(OpAdd, base, offset)
}

func scalarOf(elem *types.Type) *types.Type {
	if elem.Packed() {
		return elem
	}
	return nil
}

func (b *builder) element(e *ast.Index) place {
	seq := b.value(e.X, e.X.TypeOf())
	index := b.word(e.Index)
	b.check(index, seq[1])
	elem := elementType(e.X.TypeOf())
	return place{addr: b.scaled(seq[0], index, elem.Stride()), scalar: scalarOf(elem), tail: elem.Tail()}
}

func (b *builder) slicing(e *ast.Slicing) []*Value {
	seq := b.value(e.X, e.Type)
	lo, hi := b.constant(0), seq[1]
	if e.Lo != nil {
		lo = b.word(e.Lo)
	}
	if e.Hi != nil {
		hi = b.word(e.Hi)
	}
	one := b.constant(1)
	b.check(hi, b.emit(OpAdd, seq[1], one))
	b.check(lo, b.emit(OpAdd, hi, one))
	n := b.emit(OpSub, hi, lo)
	ptr := b.scaled(seq[0], lo, elementType(e.Type).Stride())
	if e.Type == types.Str {
		return []*Value{ptr, n}
	}
	return sliceOf(ptr, n)
}

func (b *builder) sliceLit(e *ast.SliceLit) []*Value {
	if words, ok := b.hoistedSlice(e); ok {
		return words
	}
	return b.filledSlice(e)
}

func (b *builder) filledSlice(e *ast.SliceLit) []*Value {
	n := b.constant(int64(len(e.Elems)))
	if len(e.Elems) == 0 {
		return sliceOf(b.constant(0), n)
	}
	elem := e.Type.Elem
	words := (len(e.Elems)*elem.Stride() + 7) / 8
	ptr := b.runtime(rtAlloc, b.constant(int64(words)), b.scan(elem))
	for i, el := range e.Elems {
		b.store(place{addr: ptr, offset: i * elem.Stride(), scalar: scalarOf(elem), tail: elem.Tail()}, b.value(el, elem))
	}
	return sliceOf(ptr, n)
}

func (b *builder) preludeFunc(name string) *Func {
	f, ok := b.named[name]
	if !ok {
		panic(fmt.Sprintf("ir: the prelude declares no %s", name))
	}
	return f
}

func (b *builder) callNamed(name string, args ...*Value) []*Value {
	return b.invoke(b.preludeFunc(name), args)
}

func (b *builder) preludeField(fn, name string) *types.Field {
	f := b.preludeFunc(fn).Ret.Elem.Field(name)
	if f == nil {
		panic(fmt.Sprintf("ir: the result of %s has no field %s", fn, name))
	}
	return f
}

func (b *builder) strings(e *ast.Binary) []*Value {
	if e.Op == lexer.Plus {
		return b.concatenate(e)
	}
	x := b.value(e.X, types.Str)
	y := b.value(e.Y, types.Str)
	if e.Op != lexer.Eq && e.Op != lexer.NotEq {
		order := b.callNamed(prelude.StrCmp, x[0], x[1], y[0], y[1])[0]
		return []*Value{b.typed(arith[e.Op], types.I64, order, b.constant(0))}
	}
	equal := b.equal(types.Str, x, y)
	if e.Op == lexer.NotEq {
		return []*Value{b.typed(OpNot, types.Bool, equal)}
	}
	return []*Value{equal}
}

func (b *builder) serialize(e *ast.Call) {
	value := b.value(e.Args[0], e.Args[0].TypeOf())
	writer := b.value(e.Args[1], e.Args[1].TypeOf())
	path := b.emptySlice()
	args := append(writer, path)
	args = append(args, value...)
	b.invoke(b.funcs[e.Target], args)
}

func (b *builder) make(elem *types.Type, n *Value) []*Value {
	zero := b.constant(0)
	bytes := b.emit(OpMul, n, b.constant(int64(elem.Stride())))
	data := b.runtime(rtGrow, zero, zero, bytes, b.scan(elem))
	return sliceOf(data, n)
}

func sliceOf(ptr, n *Value) []*Value {
	return []*Value{ptr, n, n}
}

func (b *builder) copySlice(e *ast.Call) *Value {
	t := e.Args[0].TypeOf()
	dst := b.value(e.Args[0], t)
	src := b.value(e.Args[1], e.Args[1].TypeOf())
	n := b.callNamed(prelude.Min, dst[1], src[1])[0]
	stride := b.constant(int64(t.Elem.Stride()))
	b.runtime(rtMemmove, dst[0], src[0], b.emit(OpMul, n, stride))
	return n
}

func (b *builder) forIn(s *ast.ForIn) {
	if s.End != nil {
		b.rangeLoop(s)
		return
	}
	if s.Iter.TypeOf().Kind == types.ChanKind {
		b.chanLoop(s)
		return
	}
	if s.Iter.TypeOf().Kind == types.MapKind {
		b.mapLoop(s)
		return
	}
	seq := b.value(s.Iter, s.Iter.TypeOf())
	counter := b.newTemp()
	step := b.newTemp()
	b.writeTemp(counter, []*Value{b.constant(0)})
	loop := b.openLoop()
	i := b.readTemp(counter, 1)[0]
	b.branch(b.emit(OpLt, i, seq[1]), loop.body, loop.exit)
	b.enter(loop.body)
	if s.KeyLocal != nil {
		b.bind(s.KeyLocal, []*Value{i})
	}
	if s.Iter.TypeOf() == types.Str {
		r := b.callNamed(prelude.Utf8Decode, seq[0], seq[1], i)
		b.bind(s.ValueLocal, r[:1])
		b.writeTemp(step, r[1:2])
	} else {
		elem := s.Iter.TypeOf().Elem
		at := place{addr: b.scaled(seq[0], i, elem.Stride()), scalar: scalarOf(elem), tail: elem.Tail()}
		b.bind(s.ValueLocal, b.load(at, elem))
		b.writeTemp(step, []*Value{b.constant(1)})
	}
	b.loopBody(loop, s.Body)
	advanced := b.emit(OpAdd, b.readTemp(counter, 1)[0], b.readTemp(step, 1)[0])
	b.writeTemp(counter, []*Value{advanced})
	b.closeLoop(loop)
}

func (b *builder) rangeLoop(s *ast.ForIn) {
	if b.fillLoop(s) {
		return
	}
	t := s.ValueLocal.Type
	lo := b.word(s.Iter)
	hi := b.word(s.End)
	counter := b.newTemp()
	b.writeTemp(counter, []*Value{lo})
	loop := b.openLoop()
	i := b.readTemp(counter, 1)[0]
	b.branch(b.typed(OpLt, t, i, hi), loop.body, loop.exit)
	b.enter(loop.body)
	b.bind(s.ValueLocal, []*Value{i})
	b.loopBody(loop, s.Body)
	advanced := b.typed(OpAdd, t, b.readTemp(counter, 1)[0], b.constant(1))
	b.writeTemp(counter, []*Value{advanced})
	b.closeLoop(loop)
}

type loopBlocks struct {
	header, body, next, exit *Block
}

func (b *builder) openLoop() loopBlocks {
	loop := loopBlocks{header: b.newBlock(), body: b.newBlock(), next: b.newBlock(), exit: b.newBlock()}
	b.jump(loop.header)
	b.cur = loop.header
	return loop
}

func (b *builder) loopBody(loop loopBlocks, body *ast.Block) {
	b.loops = append(b.loops, loopTarget{cont: loop.next, brk: loop.exit, depth: len(b.defers)})
	b.block(body)
	b.loops = b.loops[:len(b.loops)-1]
	b.jump(loop.next)
	b.enter(loop.next)
}

func (b *builder) closeLoop(loop loopBlocks) {
	b.backEdge(loop.header)
	b.seal(loop.header)
	b.enter(loop.exit)
}

func (b *builder) coalesce(e *ast.Binary) []*Value {
	optional := e.X.TypeOf()
	words := b.value(e.X, optional)
	temp := b.newTemp()
	present := b.newBlock()
	absent := b.newBlock()
	done := b.newBlock()
	b.branch(words[0], present, absent)
	b.enter(present)
	if e.Type == optional {
		b.writeTemp(temp, words)
	} else {
		b.writeTemp(temp, words[1:])
	}
	b.jump(done)
	b.enter(absent)
	b.writeTemp(temp, b.value(e.Y, e.Type))
	b.jump(done)
	b.enter(done)
	return b.readTemp(temp, e.Type.Size())
}
