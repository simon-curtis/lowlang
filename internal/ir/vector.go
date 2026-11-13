package ir

import (
	"lowlang/internal/ast"
	"lowlang/internal/builtin"
	"lowlang/internal/lexer"
	"lowlang/internal/types"
)

type VectorOp int

const (
	VCopy VectorOp = iota
	VAdd
	VSub
	VMul
	VDiv
	VAnd
	VOr
	VXor
	VAndNot
	VMin
	VMax
	VEq
	VNe
	VLt
	VLe
	VGt
	VGe
	VShuffle
	VShl
	VShr
	VNot
	VNeg
	VSplat
	VSelect
	VMask
)

var vectorOpNames = [...]string{
	VCopy: "copy", VAdd: "add", VSub: "sub", VMul: "mul", VDiv: "div", VAnd: "and", VOr: "or", VXor: "xor",
	VAndNot: "andnot", VMin: "min", VMax: "max", VEq: "eq", VNe: "ne", VLt: "lt", VLe: "le", VGt: "gt", VGe: "ge",
	VShuffle: "shuffle", VShl: "shl", VShr: "shr", VNot: "not", VNeg: "neg", VSplat: "splat", VSelect: "select", VMask: "mask",
}

func (o VectorOp) String() string {
	return vectorOpNames[o]
}

var vectorBinary = map[lexer.Kind]VectorOp{
	lexer.Plus: VAdd, lexer.Minus: VSub, lexer.Star: VMul, lexer.Slash: VDiv,
	lexer.Amp: VAnd, lexer.Pipe: VOr, lexer.Caret: VXor,
	lexer.Eq: VEq, lexer.NotEq: VNe, lexer.Lt: VLt, lexer.LtEq: VLe, lexer.Gt: VGt, lexer.GtEq: VGe,
	lexer.Shl: VShl, lexer.Shr: VShr,
}

func isVector(t *types.Type) bool {
	return t != nil && t.Kind == types.VectorKind
}

func (b *builder) vectorSlot(t *types.Type) *Value {
	return b.slot(t.Size())
}

func (b *builder) vectorOp(op VectorOp, t *types.Type, args ...*Value) {
	v := b.emit(OpVector, args...)
	v.Aux = int64(op)
	v.Type = t
}

func (b *builder) vec(e ast.Expr) *Value {
	if addr := b.vectorNode(e); addr != nil {
		return addr
	}
	if id, ok := e.(*ast.Ident); ok && id.Local != nil {
		if slot, found := b.slots[id.Local]; found {
			return slot
		}
	}
	t := e.TypeOf()
	slot := b.vectorSlot(t)
	b.store(place{addr: slot}, b.value(e, t))
	return slot
}

func (b *builder) vectorNode(e ast.Expr) *Value {
	switch e := e.(type) {
	case *ast.Binary:
		return b.vectorBinary(e)
	case *ast.Unary:
		op := VNeg
		if e.Op == lexer.Tilde {
			op = VNot
		}
		dst := b.vectorSlot(e.Type)
		b.vectorOp(op, e.Type, dst, b.vec(e.X))
		return dst
	case *ast.Call:
		return b.vectorCall(e)
	}
	return nil
}

func (b *builder) operand(e ast.Expr, splat bool, t *types.Type) *Value {
	if !splat {
		return b.vec(e)
	}
	slot := b.vectorSlot(t)
	b.vectorOp(VSplat, t, slot, b.value(e, t.Elem)[0])
	return slot
}

func vectorShift(e *ast.Binary) bool {
	op := vectorBinary[e.Op]
	return op == VShl || op == VShr
}

func vectorSpine(e *ast.Binary) []*ast.Binary {
	spine := []*ast.Binary{e}
	for vectorShift(e) || !e.SplatX {
		next, ok := e.X.(*ast.Binary)
		if !ok {
			break
		}
		spine = append(spine, next)
		e = next
	}
	return spine
}

func (b *builder) vectorBinary(e *ast.Binary) *Value {
	spine := vectorSpine(e)
	slots := make([]*Value, len(spine))
	for i, link := range spine {
		slots[i] = b.vectorSlot(link.Type)
	}
	tail := len(spine) - 1
	first := b.vectorFirst(spine[tail])
	for i := tail; i >= 0; i-- {
		first = b.vectorLink(spine[i], slots[i], first)
	}
	return first
}

func (b *builder) vectorFirst(e *ast.Binary) *Value {
	if vectorShift(e) {
		return b.vec(e.X)
	}
	return b.operand(e.X, e.SplatX, e.Type)
}

func (b *builder) vectorLink(e *ast.Binary, dst, first *Value) *Value {
	t := e.Type
	op := vectorBinary[e.Op]
	if vectorShift(e) {
		b.vectorOp(op, t, dst, first, b.word(e.Y))
		return dst
	}
	b.vectorOp(op, t, dst, first, b.operand(e.Y, e.SplatY, t))
	return dst
}

func (b *builder) vectorAddress(data, at ast.Expr, t *types.Type) *Value {
	source := data.TypeOf()
	stride := 1
	if source.Kind == types.SliceKind {
		stride = source.Elem.Stride()
	}
	words := b.value(data, source)
	index := b.word(at)
	count := (t.VectorBytes() + stride - 1) / stride
	b.check(index, words[1])
	b.check(b.emit(OpAdd, index, b.constant(int64(count-1))), words[1])
	return b.scaled(words[0], index, stride)
}

var vectorBuiltins = map[builtin.ID]VectorOp{builtin.VShuffle: VShuffle, builtin.VMin: VMin, builtin.VMax: VMax}

func (b *builder) vectorCall(e *ast.Call) *Value {
	t := e.Type
	if op, ok := vectorBuiltins[e.Builtin]; ok {
		dst := b.vectorSlot(t)
		b.vectorOp(op, t, dst, b.vec(e.Args[0]), b.vec(e.Args[1]))
		return dst
	}
	switch e.Builtin {
	case builtin.VectorCast:
		dst := b.vectorSlot(t)
		b.vectorOp(VCopy, t, dst, b.vec(e.Args[0]))
		return dst
	case builtin.VLoad:
		dst := b.vectorSlot(t)
		b.vectorOp(VCopy, t, dst, b.vectorAddress(e.Args[0], e.Args[1], t))
		return dst
	case builtin.VSplat:
		dst := b.vectorSlot(t)
		b.vectorOp(VSplat, t, dst, b.value(e.Args[0], t.Elem)[0])
		return dst
	case builtin.VSelect:
		dst := b.vectorSlot(t)
		b.vectorOp(VSelect, t, dst, b.vec(e.Args[0]), b.vec(e.Args[1]), b.vec(e.Args[2]))
		return dst
	case builtin.VWith:
		dst := b.vectorSlot(t)
		b.vectorOp(VCopy, t, dst, b.vec(e.Args[0]))
		b.store(b.lanePlace(dst, e.Args[1], t), b.value(e.Args[2], t.Elem))
		return dst
	}
	return nil
}

func (b *builder) lanePlace(addr *Value, index ast.Expr, t *types.Type) place {
	i := b.word(index)
	b.check(i, b.constant(int64(t.Lanes)))
	return place{addr: b.scaled(addr, i, t.LaneBytes()), scalar: t.Elem.Width()}
}

func (b *builder) assignVector(s *ast.Assign) {
	t := s.Target.TypeOf()
	p := b.placeOf(s.Target)
	dst := p.addr
	if dst != nil {
		dst = b.address(p)
	} else {
		dst = b.vectorSlot(t)
		b.store(place{addr: dst}, b.load(p, t))
	}
	op := s.Op.Operator()
	switch {
	case op == lexer.Assign:
		b.vectorOp(VCopy, t, dst, b.vec(s.Value))
	case op == lexer.Shl || op == lexer.Shr:
		b.vectorOp(vectorBinary[op], t, dst, dst, b.word(s.Value))
	default:
		b.vectorOp(vectorBinary[op], t, dst, dst, b.operand(s.Value, s.Splat, t))
	}
	if p.addr == nil {
		b.store(p, b.load(place{addr: dst}, t))
	}
}

func (b *builder) bindVector(local *ast.Local, e ast.Expr) {
	if id, ok := e.(*ast.Ident); ok && id.Local != nil {
		if _, found := b.slots[id.Local]; found {
			slot := b.vectorSlot(local.Type)
			b.vectorOp(VCopy, local.Type, slot, b.vec(e))
			b.slots[local] = slot
			return
		}
	}
	b.slots[local] = b.vec(e)
}
