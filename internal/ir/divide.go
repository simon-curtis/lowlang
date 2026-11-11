package ir

import (
	"math"
	"math/bits"

	"lowlang/internal/types"
)

type unsignedMagic struct {
	multiplier uint64
	shift      int
	add        bool
}

type signedMagic struct {
	multiplier int64
	shift      int
	add        bool
}

func newUnsignedMagic(d uint64) unsignedMagic {
	log := 63 - bits.LeadingZeros64(d)
	q, rem := bits.Div64(1<<log, 0, d)
	if d-rem < 1<<log {
		return unsignedMagic{multiplier: q + 1, shift: log}
	}
	q += q
	twice := rem + rem
	if twice >= d || twice < rem {
		q++
	}
	return unsignedMagic{multiplier: q + 1, shift: log, add: true}
}

func newSignedMagic(d uint64) signedMagic {
	log := 63 - bits.LeadingZeros64(d)
	q, rem := bits.Div64(1<<(log-1), 0, d)
	if d-rem < 1<<log {
		return signedMagic{multiplier: int64(q + 1), shift: log - 1}
	}
	q += q
	twice := rem + rem
	if twice >= d || twice < rem {
		q++
	}
	return signedMagic{multiplier: int64(q + 1), shift: log, add: true}
}

func magnitude(d int64) uint64 {
	if d < 0 {
		return -uint64(d)
	}
	return uint64(d)
}

func unsignedDivision(v *Value) bool {
	return v.Type != nil && !v.Type.Signed
}

func dividesByConstant(v *Value, d int64) bool {
	if d == 0 || !integerOp(v) {
		return false
	}
	return unsignedDivision(v) || (d != -1 && d != math.MinInt64)
}

func (o *optimizer) operation(v *Value, op Op, t *types.Type, args ...*Value) *Value {
	n := o.fn.newValue(v.Block, op, args...)
	n.Type = t
	n.Scalar = v.Scalar
	o.out = append(o.out, n)
	return n
}

func (o *optimizer) constantFor(v *Value, n int64) *Value {
	c := o.fn.newValue(v.Block, OpConst)
	c.Aux = n
	c.Scalar = true
	o.out = append(o.out, c)
	return c
}

func (o *optimizer) become(v *Value, op Op, args ...*Value) {
	v.Op = op
	v.Args = args
}

func (o *optimizer) divideByConstant(v, x *Value, d int64) {
	if unsignedDivision(v) {
		o.divideUnsigned(v, x, uint64(d))
		return
	}
	o.divideSigned(v, x, d)
}

func (o *optimizer) divideUnsigned(v, x *Value, d uint64) {
	switch {
	case d == 1 && v.Op == OpDiv:
		forwardTo(v, x)
	case d == 1:
		becomeConstant(v, 0)
	case d&(d-1) == 0 && v.Op == OpRem:
		mask := o.constantFor(v, int64(d-1))
		o.become(v, OpAnd, x, mask)
	case d&(d-1) == 0:
		count := o.constantFor(v, int64(bits.TrailingZeros64(d)))
		o.become(v, OpShr, x, count)
	default:
		o.unsignedByMagic(v, x, d)
	}
}

func (o *optimizer) unsignedByMagic(v, x *Value, d uint64) {
	magic := newUnsignedMagic(d)
	multiplier := o.constantFor(v, int64(magic.multiplier))
	scaled := o.operation(v, OpMulHigh, types.U64, x, multiplier)
	if magic.add {
		excess := o.operation(v, OpSub, types.U64, x, scaled)
		one := o.constantFor(v, 1)
		halved := o.operation(v, OpShr, types.U64, excess, one)
		scaled = o.operation(v, OpAdd, types.U64, halved, scaled)
	}
	count := o.constantFor(v, int64(magic.shift))
	if v.Op == OpDiv {
		o.become(v, OpShr, scaled, count)
		return
	}
	q := o.operation(v, OpShr, types.U64, scaled, count)
	divisor := o.constantFor(v, int64(d))
	product := o.operation(v, OpMul, types.U64, q, divisor)
	o.become(v, OpSub, x, product)
}

func (o *optimizer) divideSigned(v, x *Value, d int64) {
	m := magnitude(d)
	switch {
	case m == 1 && v.Op == OpRem:
		becomeConstant(v, 0)
	case m == 1:
		forwardTo(v, x)
	case m&(m-1) == 0:
		o.signedPowerOfTwo(v, x, d, bits.TrailingZeros64(m))
	default:
		o.signedByMagic(v, x, d, m)
	}
}

func (o *optimizer) signedPowerOfTwo(v, x *Value, d int64, k int) {
	wordSign := o.constantFor(v, 63)
	bias := o.operation(v, OpShr, types.U64, x, wordSign)
	if k > 1 {
		signs := o.operation(v, OpShr, types.I64, x, wordSign)
		biasCount := o.constantFor(v, int64(64-k))
		bias = o.operation(v, OpShr, types.U64, signs, biasCount)
	}
	sum := o.operation(v, OpAdd, types.I64, x, bias)
	count := o.constantFor(v, int64(k))
	switch {
	case v.Op == OpRem:
		q := o.operation(v, OpShr, types.I64, sum, count)
		scaled := o.operation(v, OpShl, types.I64, q, count)
		o.become(v, OpSub, x, scaled)
	case d < 0:
		q := o.operation(v, OpShr, types.I64, sum, count)
		o.become(v, OpNeg, q)
	default:
		o.become(v, OpShr, sum, count)
	}
}

func (o *optimizer) signedByMagic(v, x *Value, d int64, m uint64) {
	magic := newSignedMagic(m)
	multiplier := o.constantFor(v, magic.multiplier)
	scaled := o.operation(v, OpMulHigh, types.I64, x, multiplier)
	if magic.add {
		scaled = o.operation(v, OpAdd, types.I64, scaled, x)
	}
	if magic.shift > 0 {
		shift := o.constantFor(v, int64(magic.shift))
		scaled = o.operation(v, OpShr, types.I64, scaled, shift)
	}
	signCount := o.constantFor(v, 63)
	sign := o.operation(v, OpShr, types.U64, x, signCount)
	switch {
	case v.Op == OpRem:
		q := o.operation(v, OpAdd, types.I64, scaled, sign)
		divisor := o.constantFor(v, int64(m))
		product := o.operation(v, OpMul, types.I64, q, divisor)
		o.become(v, OpSub, x, product)
	case d < 0:
		q := o.operation(v, OpAdd, types.I64, scaled, sign)
		o.become(v, OpNeg, q)
	default:
		o.become(v, OpAdd, scaled, sign)
	}
}
