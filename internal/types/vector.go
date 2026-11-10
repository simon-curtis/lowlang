package types

import "fmt"

var Vectors = vectors()

func vectors() []*Type {
	var out []*Type
	for _, lane := range []*Type{U8, U16, I32, U32, I64, U64, F32, F64} {
		for _, bits := range []int{128, 256} {
			lanes := bits / lane.Bits
			out = append(out, &Type{Kind: VectorKind, Name: fmt.Sprintf("%sx%d", lane.Name, lanes), Elem: lane, Bits: bits, Lanes: lanes, size: bits / 64})
		}
	}
	return out
}

func (t *Type) LaneBytes() int {
	return t.Elem.Bits / 8
}

func (t *Type) VectorBytes() int {
	return t.Bits / 8
}
