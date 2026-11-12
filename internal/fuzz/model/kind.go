package model

type Kind int

const (
	I64 Kind = iota
	I32
	U64
	U32
	U16
	U8
	F64
	F32
	Bool
)

var IntKinds = []Kind{I64, I32, U64, U32, U16, U8}

var kindNames = [...]string{I64: "i64", I32: "i32", U64: "u64", U32: "u32", U16: "u16", U8: "u8", F64: "f64", F32: "f32", Bool: "bool"}

var kindBits = [...]int{I64: 64, I32: 32, U64: 64, U32: 32, U16: 16, U8: 8, F64: 64, F32: 32, Bool: 1}

func (k Kind) String() string {
	return kindNames[k]
}

func (k Kind) Bits() int {
	return kindBits[k]
}

func (k Kind) IsInt() bool {
	return k <= U8
}

func (k Kind) IsFloat() bool {
	return k == F64 || k == F32
}

func (k Kind) Signed() bool {
	return k == I64 || k == I32
}

func (k Kind) Mask() uint64 {
	if k.Bits() >= 64 {
		return ^uint64(0)
	}
	return 1<<k.Bits() - 1
}

func (k Kind) Wrap(v uint64) uint64 {
	return v & k.Mask()
}

func (k Kind) Extend(v uint64) int64 {
	v = k.Wrap(v)
	if k.Signed() && k.Bits() < 64 && v>>(k.Bits()-1) == 1 {
		return int64(v | ^k.Mask())
	}
	return int64(v)
}

func (k Kind) MinSigned() int64 {
	return -1 << (k.Bits() - 1)
}
