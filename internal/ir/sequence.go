package ir

import (
	"lowlang/internal/ast"
	"lowlang/internal/lexer"
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
	return []*Value{ptr, n, n}
}

func (b *builder) sliceLit(e *ast.SliceLit) []*Value {
	n := b.constant(int64(len(e.Elems)))
	if len(e.Elems) == 0 {
		return []*Value{b.constant(0), n, n}
	}
	elem := e.Type.Elem
	words := (len(e.Elems)*elem.Stride() + 7) / 8
	ptr := b.runtime("alloc", b.constant(int64(words)), b.scan(elem))
	for i, el := range e.Elems {
		b.store(place{addr: ptr, offset: i * elem.Stride(), scalar: scalarOf(elem), tail: elem.Tail()}, b.value(el, elem))
	}
	return []*Value{ptr, n, n}
}

func (b *builder) callNamed(name string, args ...*Value) []*Value {
	return b.invoke(b.named[name], args)
}

func (b *builder) buffer() *Value {
	slot := b.emit(OpSlot)
	slot.Aux = int64(len(b.fn.Slots))
	b.fn.Slots = append(b.fn.Slots, 3)
	zero := b.constant(0)
	b.store(place{addr: slot}, []*Value{zero, zero, zero})
	return slot
}

func (b *builder) appendValue(buf *Value, part ast.Expr) {
	t := part.TypeOf()
	words := b.value(part, t)
	switch {
	case t == types.Str:
		b.callNamed("__append_str", buf, words[0], words[1])
	case t == types.Bool:
		b.callNamed("__append_bool", buf, words[0])
	case t == types.F64:
		b.callNamed("__append_f64", buf, words[0])
	case t == types.F32:
		b.callNamed("__append_f32", buf, words[0])
	default:
		b.callNamed("__append_i64", buf, words[0])
	}
}

func (b *builder) interpolate(e *ast.Interp) []*Value {
	buf := b.buffer()
	for _, part := range e.Parts {
		if lit, ok := part.(*ast.StrLit); ok && lit.Value == "" {
			continue
		}
		b.appendValue(buf, part)
	}
	return b.load(place{addr: buf}, types.Str)
}

func (b *builder) strings(e *ast.Binary) []*Value {
	x := b.value(e.X, types.Str)
	y := b.value(e.Y, types.Str)
	if e.Op == lexer.Plus {
		return b.callNamed("__concat", x[0], x[1], y[0], y[1])
	}
	equal := b.runtime("str_eq", x[0], x[1], y[0], y[1])
	equal.Scalar = true
	if e.Op == lexer.NotEq {
		return []*Value{b.typed(OpNot, types.Bool, equal)}
	}
	return []*Value{equal}
}

func (b *builder) runtimeVoid(name string, args ...*Value) {
	call := b.emit(OpRuntime, args...)
	call.Str = name
}

func (b *builder) builtin(e *ast.Call) []*Value {
	if words, ok := b.concurrencyBuiltin(e); ok {
		return words
	}
	if e.Builtin == "__zero" {
		zero := b.constant(0)
		words := make([]*Value, e.Type.Size())
		for k := range words {
			words[k] = zero
		}
		return words
	}
	if words, ok := b.pointerBuiltin(e); ok {
		return words
	}
	arg := e.Args[0]
	switch e.Builtin {
	case "push":
		b.push(e)
		return nil
	case "delete":
		b.deleteEntry(e)
		return nil
	case "embed":
		v := b.emit(OpEmbed)
		v.Str = e.Embed
		v.Aux = int64(e.Digest)
		v.Scalar = true
		return []*Value{v, b.constant(e.Size)}
	case "__str_raw":
		return []*Value{b.word(arg), b.word(e.Args[1])}
	case "bytes":
		return b.callNamed("__bytes", b.value(arg, types.Str)...)
	case "from_bytes":
		return b.callNamed("__from_bytes", b.value(arg, arg.TypeOf())...)
	case "__bits":
		return []*Value{b.word(arg)}
	case "__hash":
		if arg.TypeOf() == types.Str {
			return b.callNamed("__hash_str", b.value(arg, types.Str)...)
		}
		return b.callNamed("__hash_word", b.word(arg))

	case "__str_view":
		return b.value(arg, arg.TypeOf())[:2]
	case "print_str":
		b.runtimeVoid("print_str", b.value(arg, types.Str)...)
		return nil
	case "print_float":
		name := "__format_f64"
		if arg.TypeOf() == types.F32 {
			name = "__format_f32"
		}
		b.runtimeVoid("print_str", b.callNamed(name, b.word(arg))...)
		return nil
	}
	b.runtimeVoid(e.Builtin, b.word(arg))
	return nil
}

func (b *builder) push(e *ast.Call) {
	ref := e.Args[0].(*ast.Ref)
	elem := ref.Type.Elem
	stride := int64(elem.Stride())
	header := b.address(b.place(ref.Place))
	value := b.value(e.Args[1], elem)
	slice := b.load(place{addr: header}, ref.Type)
	ptr, length, capacity := slice[0], slice[1], slice[2]
	temp := b.newTemp()
	grow := b.newBlock()
	keep := b.newBlock()
	done := b.newBlock()
	b.branch(b.emit(OpEq, length, capacity), grow, keep)
	b.enter(grow)
	grown := b.emit(OpAdd, b.emit(OpMul, capacity, b.constant(2)), b.constant(4))
	used := b.emit(OpMul, length, b.constant(stride))
	moved := b.runtime("grow", ptr, used, b.emit(OpMul, grown, b.constant(stride)), b.scan(elem))
	b.store(place{addr: header}, []*Value{moved})
	b.store(place{addr: header, offset: 16}, []*Value{grown})
	b.writeTemp(temp, []*Value{moved})
	b.jump(done)
	b.enter(keep)
	b.writeTemp(temp, []*Value{ptr})
	b.jump(done)
	b.enter(done)
	base := b.readTemp(temp, 1)[0]
	b.store(place{addr: b.scaled(base, length, int(stride)), scalar: scalarOf(elem), tail: elem.Tail()}, value)
	b.store(place{addr: header, offset: 8}, []*Value{b.emit(OpAdd, length, b.constant(1))})
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
	header := b.newBlock()
	body := b.newBlock()
	next := b.newBlock()
	exit := b.newBlock()
	b.jump(header)
	b.cur = header
	i := b.readTemp(counter, 1)[0]
	b.branch(b.emit(OpLt, i, seq[1]), body, exit)
	b.enter(body)
	if s.KeyLocal != nil {
		b.bind(s.KeyLocal, []*Value{i})
	}
	if s.Iter.TypeOf() == types.Str {
		r := b.callNamed("__utf8_decode", seq[0], seq[1], i)
		b.bind(s.ValueLocal, r[:1])
		b.writeTemp(step, r[1:2])
	} else {
		elem := s.Iter.TypeOf().Elem
		at := place{addr: b.scaled(seq[0], i, elem.Stride()), scalar: scalarOf(elem), tail: elem.Tail()}
		b.bind(s.ValueLocal, b.load(at, elem))
		b.writeTemp(step, []*Value{b.constant(1)})
	}
	b.loopBody(s.Body, next, exit)
	b.enter(next)
	advanced := b.emit(OpAdd, b.readTemp(counter, 1)[0], b.readTemp(step, 1)[0])
	b.writeTemp(counter, []*Value{advanced})
	b.backEdge(header)
	b.seal(header)
	b.enter(exit)
}

func (b *builder) rangeLoop(s *ast.ForIn) {
	t := s.ValueLocal.Type
	lo := b.word(s.Iter)
	hi := b.word(s.End)
	counter := b.newTemp()
	b.writeTemp(counter, []*Value{lo})
	header := b.newBlock()
	body := b.newBlock()
	next := b.newBlock()
	exit := b.newBlock()
	b.jump(header)
	b.cur = header
	i := b.readTemp(counter, 1)[0]
	b.branch(b.typed(OpLt, t, i, hi), body, exit)
	b.enter(body)
	b.bind(s.ValueLocal, []*Value{i})
	b.loopBody(s.Body, next, exit)
	b.enter(next)
	advanced := b.typed(OpAdd, t, b.readTemp(counter, 1)[0], b.constant(1))
	b.writeTemp(counter, []*Value{advanced})
	b.backEdge(header)
	b.seal(header)
	b.enter(exit)
}

func (b *builder) loopBody(body *ast.Block, next, exit *Block) {
	b.loops = append(b.loops, loopTarget{cont: next, brk: exit, depth: len(b.defers)})
	b.block(body)
	b.loops = b.loops[:len(b.loops)-1]
	b.jump(next)
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
