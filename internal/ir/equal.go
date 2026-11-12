package ir

import (
	"lowlang/internal/types"
)

func aggregate(t *types.Type) bool {
	return t.Kind == types.StructKind || t.Kind == types.EnumKind
}

func (b *builder) equal(t *types.Type, x, y []*Value) *Value {
	switch {
	case t == types.Str:
		return b.runtime(rtStrEq, x[0], x[1], y[0], y[1])
	case isC(t):
		return b.cEqual(t, b.spill(x), b.spill(y), 0)
	case t.Kind == types.StructKind:
		return b.fieldsEqual(t, x, y)
	case t.Kind == types.EnumKind:
		return b.enumEqual(t, x, y)
	}
	return b.compareWord(t, x[0], y[0])
}

func (b *builder) compareWord(t *types.Type, x, y *Value) *Value {
	same := b.typed(OpEq, t, x, y)
	same.Scalar = true
	return same
}

func (b *builder) both(acc, next *Value) *Value {
	if acc == nil {
		return next
	}
	return b.typed(OpAnd, types.Bool, acc, next)
}

func (b *builder) fieldsEqual(t *types.Type, x, y []*Value) *Value {
	var acc *Value
	for _, f := range t.Fields {
		end := f.Word() + f.Type.Size()
		same := b.equal(f.Type, x[f.Word():end], y[f.Word():end])
		acc = b.both(acc, same)
	}
	if acc == nil {
		return b.constant(1)
	}
	return acc
}

func (b *builder) variantEqual(v *types.Variant, x, y []*Value) *Value {
	var acc *Value
	for i, f := range v.Fields {
		end := v.Offsets[i] + f.Size()
		same := b.equal(f, x[v.Offsets[i]:end], y[v.Offsets[i]:end])
		acc = b.both(acc, same)
	}
	return acc
}

func (b *builder) enumEqual(t *types.Type, x, y []*Value) *Value {
	tags := b.compareWord(types.I64, x[0], y[0])
	if t.Scalar() {
		return tags
	}
	temp := b.newTemp()
	done := b.newBlock()
	same := b.newBlock()
	b.writeTemp(temp, []*Value{b.constant(0)})
	b.branch(tags, same, done)
	b.enter(same)
	for _, v := range t.Variants {
		if len(v.Fields) == 0 {
			continue
		}
		hit := b.newBlock()
		next := b.newBlock()
		b.branch(b.compareWord(types.I64, x[0], b.constant(int64(v.Tag))), hit, next)
		b.enter(hit)
		b.writeTemp(temp, []*Value{b.variantEqual(v, x, y)})
		b.jump(done)
		b.enter(next)
	}
	b.writeTemp(temp, []*Value{b.constant(1)})
	b.jump(done)
	b.enter(done)
	return b.readTemp(temp, 1)[0]
}

func (b *builder) spill(words []*Value) *Value {
	slot := b.slot(len(words))
	b.store(place{addr: slot}, words)
	return slot
}

func (b *builder) cEqual(t *types.Type, x, y *Value, base int) *Value {
	var acc *Value
	for _, f := range t.Fields {
		at := base + f.Bytes
		var same *Value
		if isC(f.Type) {
			same = b.cEqual(f.Type, x, y, at)
		} else {
			width := f.Type.Width()
			a := b.load(place{addr: x, offset: at, scalar: width}, f.Type)[0]
			c := b.load(place{addr: y, offset: at, scalar: width}, f.Type)[0]
			same = b.compareWord(f.Type, a, c)
		}
		acc = b.both(acc, same)
	}
	return acc
}
