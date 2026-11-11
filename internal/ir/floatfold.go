package ir

import (
	"math"

	"lowlang/internal/types"
)

const (
	signBit64 = int64(math.MinInt64)
	signBit32 = int64(1) << 31
	twoPow63  = 9223372036854775808.0
)

func (o *optimizer) foldFloat(v *Value) {
	switch v.Op {
	case OpAdd, OpSub, OpMul, OpDiv:
		foldFloatBinary(v)
	case OpNeg:
		foldFloatNegate(v)
	case OpSqrt, OpFloor:
		foldFloatUnary(v)
	}
}

func floatOperand(c *Value, bits int) (float64, bool) {
	if c.Op != OpConst {
		return 0, false
	}
	f := floatFromBits(c.Aux, bits)
	return f, !math.IsNaN(f)
}

func floatFromBits(n int64, bits int) float64 {
	if bits == 32 {
		return float64(math.Float32frombits(uint32(n)))
	}
	return math.Float64frombits(uint64(n))
}

func bitsFromFloat(f float64, bits int) int64 {
	if bits == 32 {
		return int64(math.Float32bits(float32(f)))
	}
	return int64(math.Float64bits(f))
}

func becomeFloatConstant(v *Value, f float64, bits int) {
	if math.IsNaN(f) {
		return
	}
	becomeConstant(v, bitsFromFloat(f, bits))
	v.FloatBits = bits
}

func foldFloatBinary(v *Value) {
	bits := v.Type.Bits
	x, xok := floatOperand(v.Args[0], bits)
	y, yok := floatOperand(v.Args[1], bits)
	if !xok || !yok {
		return
	}
	if bits == 32 {
		becomeFloatConstant(v, float64(binary32(v.Op, float32(x), float32(y))), bits)
		return
	}
	becomeFloatConstant(v, binary64(v.Op, x, y), bits)
}

func binary32(op Op, x, y float32) float32 {
	var r float32
	switch op {
	case OpAdd:
		r = x + y
	case OpSub:
		r = x - y
	case OpMul:
		r = x * y
	default:
		r = x / y
	}
	return r
}

func binary64(op Op, x, y float64) float64 {
	var r float64
	switch op {
	case OpAdd:
		r = x + y
	case OpSub:
		r = x - y
	case OpMul:
		r = x * y
	default:
		r = x / y
	}
	return r
}

func foldFloatNegate(v *Value) {
	c := v.Args[0]
	if c.Op != OpConst {
		return
	}
	mask := signBit64
	if v.Type.Bits == 32 {
		mask = signBit32
	}
	becomeConstant(v, c.Aux^mask)
	v.FloatBits = v.Type.Bits
}

func foldFloatUnary(v *Value) {
	bits := v.Type.Bits
	x, ok := floatOperand(v.Args[0], bits)
	if !ok {
		return
	}
	if v.Op == OpFloor {
		becomeFloatConstant(v, math.Floor(x), bits)
		return
	}
	becomeFloatConstant(v, math.Sqrt(x), bits)
}

func foldConversion(v *Value) {
	c := v.Args[0]
	if c.Op != OpConst || v.From == nil || v.Type == nil {
		return
	}
	switch {
	case v.From.Integer() && v.Type.Integer():
		becomeConstant(v, wrap(c.Aux, v.Type))
	case v.From.Integer() && v.Type.Float():
		foldIntegerToFloat(v, c)
	case v.From.Float() && v.Type.Integer():
		foldFloatToInteger(v, c)
	case v.From.Float() && v.Type.Float():
		foldFloatToFloat(v, c)
	}
}

func foldIntegerToFloat(v, c *Value) {
	if v.From == types.U64 && c.Aux < 0 {
		return
	}
	if v.Type.Bits == 32 {
		becomeFloatConstant(v, float64(float32(c.Aux)), 32)
		return
	}
	becomeFloatConstant(v, float64(c.Aux), 64)
}

func foldFloatToInteger(v, c *Value) {
	f, ok := floatOperand(c, v.From.Bits)
	if !ok {
		return
	}
	truncated := math.Trunc(f)
	inRange := truncated >= -twoPow63 && truncated < twoPow63
	switch {
	case v.Type == types.U64 && !(inRange && truncated >= 0):
		return
	case !inRange:
		becomeConstant(v, wrap(math.MinInt64, v.Type))
	default:
		becomeConstant(v, wrap(int64(truncated), v.Type))
	}
	v.FloatBits = 0
}

func foldFloatToFloat(v, c *Value) {
	f, ok := floatOperand(c, v.From.Bits)
	if !ok {
		return
	}
	becomeFloatConstant(v, f, v.Type.Bits)
}
