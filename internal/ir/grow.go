package ir

import (
	"lowlang/internal/ast"
	"lowlang/internal/lexer"
	"lowlang/internal/types"
)

const (
	maxFillBytes   = 1 << 40
	impossibleSize = 1 << 62
)

func (b *builder) count(op Op, x, y *Value) *Value {
	return b.typed(op, types.I64, x, y)
}

func (b *builder) regrow(ptr, length, capacity *Value, elem *types.Type) *Value {
	stride := b.constant(int64(elem.Stride()))
	used := b.emit(OpMul, length, stride)
	size := b.emit(OpMul, capacity, stride)
	return b.runtime("grow", ptr, used, size, b.scan(elem))
}

func (b *builder) extend(e *ast.Call) {
	ref := e.Args[0].(*ast.Ref)
	header := b.place(ref.Place)
	src := b.value(e.Args[1], types.Str)
	base, length, needed := b.room(header, ref.Type, src[1])
	end := b.emit(OpAdd, base, length)
	b.runtimeVoid("memmove", end, src[0], src[1])
	b.store(header.at(1), []*Value{needed})
}

func (b *builder) extendBy(e *ast.Call) *Value {
	ref := e.Args[0].(*ast.Ref)
	header := b.place(ref.Place)
	count := b.word(e.Args[1])
	_, length, needed := b.room(header, ref.Type, count)
	b.store(header.at(1), []*Value{needed})
	return length
}

func (b *builder) room(header place, t *types.Type, extra *Value) (base, length, needed *Value) {
	slice := b.load(header, t)
	ptr, length, capacity := slice[0], slice[1], slice[2]
	needed = b.count(OpAdd, length, extra)
	temp := b.newTemp()
	grow := b.newBlock()
	keep := b.newBlock()
	done := b.newBlock()
	b.branch(b.emit(OpGt, needed, capacity), grow, keep)
	b.enter(grow)
	doubled := b.count(OpMul, capacity, b.constant(2))
	padded := b.count(OpAdd, extra, b.constant(4))
	grown := b.count(OpAdd, doubled, padded)
	moved := b.regrow(ptr, length, grown, types.U8)
	b.store(header, []*Value{moved})
	b.store(header.at(2), []*Value{grown})
	b.writeTemp(temp, []*Value{moved})
	b.jump(done)
	b.enter(keep)
	b.writeTemp(temp, []*Value{ptr})
	b.jump(done)
	b.enter(done)
	return b.readTemp(temp, 1)[0], length, needed
}

func (b *builder) push(e *ast.Call) {
	ref := e.Args[0].(*ast.Ref)
	elem := ref.Type.Elem
	header := b.place(ref.Place)
	value := b.value(e.Args[1], elem)
	slice := b.load(header, ref.Type)
	ptr, length, capacity := slice[0], slice[1], slice[2]
	temp := b.newTemp()
	grow := b.newBlock()
	keep := b.newBlock()
	done := b.newBlock()
	b.branch(b.emit(OpEq, length, capacity), grow, keep)
	b.enter(grow)
	doubled := b.count(OpMul, capacity, b.constant(2))
	grown := b.count(OpAdd, doubled, b.constant(4))
	moved := b.regrow(ptr, length, grown, elem)
	b.store(header, []*Value{moved})
	b.store(header.at(2), []*Value{grown})
	b.writeTemp(temp, []*Value{moved})
	b.jump(done)
	b.enter(keep)
	b.writeTemp(temp, []*Value{ptr})
	b.jump(done)
	b.enter(done)
	base := b.readTemp(temp, 1)[0]
	addr := b.scaled(base, length, elem.Stride())
	b.store(place{addr: addr, scalar: scalarOf(elem), tail: elem.Tail()}, value)
	b.store(header.at(1), []*Value{b.count(OpAdd, length, b.constant(1))})
}

func (b *builder) fillLoop(s *ast.ForIn) bool {
	call, ok := loopPush(s.Body)
	if !ok {
		return false
	}
	ref := call.Args[0].(*ast.Ref)
	local := ref.Place.Local
	if local == nil || ref.Place.Ptr != nil || ref.Place.Index != nil {
		return false
	}
	header := b.place(ref.Place)
	if header.addr != nil || !invariantValue(call.Args[1], local, s.ValueLocal) {
		return false
	}
	b.fill(s, header, ref.Type, call.Args[1])
	return true
}

func loopPush(body *ast.Block) (*ast.Call, bool) {
	var x ast.Expr
	switch {
	case len(body.Stmts) == 1 && body.Tail == nil:
		stmt, ok := body.Stmts[0].(*ast.ExprStmt)
		if !ok {
			return nil, false
		}
		x = stmt.X
	case len(body.Stmts) == 0:
		x = body.Tail
	}
	call, ok := x.(*ast.Call)
	if !ok || call.Builtin != "push" {
		return nil, false
	}
	return call, true
}

func invariantValue(e ast.Expr, stored, counter *ast.Local) bool {
	switch e := e.(type) {
	case *ast.IntLit, *ast.FloatLit, *ast.BoolLit:
		return true
	case *ast.Ident:
		if e.Variant != nil || e.Null {
			return true
		}
		return e.Func == nil && !e.Moved && e.Local != nil && e.Local != stored && e.Local != counter
	case *ast.Unary:
		return (e.Op == lexer.Minus || e.Op == lexer.Tilde || e.Op == lexer.Not) && invariantValue(e.X, stored, counter)
	case *ast.Binary:
		return pureBinary(e) && invariantValue(e.X, stored, counter) && invariantValue(e.Y, stored, counter)
	case *ast.Call:
		return e.Builtin == "convert" && invariantValue(e.Args[0], stored, counter)
	case *ast.Selector:
		plain := e.Field != nil && e.Variant == nil && e.Reference == nil && e.Func == nil && !e.Length
		return plain && e.X.TypeOf().Kind == types.StructKind && invariantValue(e.X, stored, counter)
	}
	return false
}

func pureBinary(e *ast.Binary) bool {
	if e.SplatX || e.SplatY || isVector(e.X.TypeOf()) {
		return false
	}
	if t := e.X.TypeOf(); !t.Integer() && !t.Float() {
		return false
	}
	switch e.Op {
	case lexer.Plus, lexer.Minus, lexer.Star, lexer.Amp, lexer.Pipe, lexer.Caret, lexer.Shl, lexer.Shr,
		lexer.Eq, lexer.NotEq, lexer.Lt, lexer.LtEq, lexer.Gt, lexer.GtEq:
		return true
	}
	return false
}

func zeroWord(v *Value) bool {
	switch v.Op {
	case OpConst:
		return v.Aux == 0
	case OpConvert:
		return zeroWord(v.Args[0])
	}
	return false
}

func allZero(words []*Value) bool {
	for _, w := range words {
		if !zeroWord(w) {
			return false
		}
	}
	return true
}

func (b *builder) fill(s *ast.ForIn, header place, t *types.Type, valueExpr ast.Expr) {
	elem := t.Elem
	lo := b.word(s.Iter)
	hi := b.word(s.End)
	value := b.value(valueExpr, elem)
	nonEmpty := b.typed(OpLt, s.ValueLocal.Type, lo, hi)
	fill := b.newBlock()
	skip := b.newBlock()
	done := b.newBlock()
	b.branch(nonEmpty, fill, skip)
	b.enter(skip)
	b.jump(done)
	b.enter(fill)
	trips := b.count(OpSub, hi, lo)
	b.rejectOversize(trips, elem)
	slice := b.load(header, t)
	ptr, length, capacity := slice[0], slice[1], slice[2]
	needed := b.count(OpAdd, length, trips)
	zero := allZero(value)
	var base, room *Value
	if zeroWord(length) && zeroWord(capacity) {
		base = b.regrow(ptr, length, needed, elem)
		room = needed
	} else {
		base, room = b.reserve(ptr, length, capacity, needed, elem, value, zero)
	}
	if !zero {
		b.fillRange(base, length, needed, value, elem)
	}
	b.store(header, []*Value{base})
	b.store(header.at(1), []*Value{needed})
	b.store(header.at(2), []*Value{room})
	b.jump(done)
	b.enter(done)
}

func (b *builder) reserve(ptr, length, capacity, needed *Value, elem *types.Type, value []*Value, zeroed bool) (base, room *Value) {
	ptrTemp := b.newTemp()
	roomTemp := b.newTemp()
	grow := b.newBlock()
	keep := b.newBlock()
	joined := b.newBlock()
	b.branch(b.emit(OpGt, needed, capacity), grow, keep)
	b.enter(grow)
	moved := b.regrow(ptr, length, needed, elem)
	b.writeTemp(ptrTemp, []*Value{moved})
	b.writeTemp(roomTemp, []*Value{needed})
	b.jump(joined)
	b.enter(keep)
	b.writeTemp(ptrTemp, []*Value{ptr})
	b.writeTemp(roomTemp, []*Value{capacity})
	if zeroed {
		b.fillRange(ptr, length, needed, value, elem)
	}
	b.jump(joined)
	b.enter(joined)
	return b.readTemp(ptrTemp, 1)[0], b.readTemp(roomTemp, 1)[0]
}

func (b *builder) rejectOversize(trips *Value, elem *types.Type) {
	stride := max(elem.Stride(), 1)
	limit := b.constant(int64(maxFillBytes / stride))
	tooMany := b.typed(OpGt, types.U64, trips, limit)
	oversize := b.newBlock()
	fits := b.newBlock()
	b.branch(tooMany, oversize, fits)
	b.enter(oversize)
	zero := b.constant(0)
	b.runtimeVoid("grow", zero, zero, b.constant(impossibleSize), zero)
	b.cur.Kind = Unreachable
	b.enter(fits)
}

func (b *builder) fillRange(base, from, to *Value, value []*Value, elem *types.Type) {
	counter := b.newTemp()
	b.writeTemp(counter, []*Value{from})
	header := b.newBlock()
	body := b.newBlock()
	exit := b.newBlock()
	b.jump(header)
	b.cur = header
	i := b.readTemp(counter, 1)[0]
	b.branch(b.typed(OpLt, types.I64, i, to), body, exit)
	b.enter(body)
	addr := b.scaled(base, i, elem.Stride())
	b.store(place{addr: addr, scalar: scalarOf(elem), tail: elem.Tail()}, value)
	b.writeTemp(counter, []*Value{b.count(OpAdd, i, b.constant(1))})
	b.backEdge(header)
	b.seal(header)
	b.enter(exit)
}
