package asm

import "encoding/binary"

const (
	maxNopSize       = 11
	maxNopRun        = 7
	plainNop         = 0x90
	shortJumpOverMax = 127
)

var nopPatterns = [maxNopSize][]byte{
	{0x90},
	{0x66, 0x90},
	{0x0F, 0x1F, 0x00},
	{0x0F, 0x1F, 0x40, 0x00},
	{0x0F, 0x1F, 0x44, 0x00, 0x00},
	{0x66, 0x0F, 0x1F, 0x44, 0x00, 0x00},
	{0x0F, 0x1F, 0x80, 0x00, 0x00, 0x00, 0x00},
	{0x0F, 0x1F, 0x84, 0x00, 0x00, 0x00, 0x00, 0x00},
	{0x66, 0x0F, 0x1F, 0x84, 0x00, 0x00, 0x00, 0x00, 0x00},
	{0x66, 0x2E, 0x0F, 0x1F, 0x84, 0x00, 0x00, 0x00, 0x00, 0x00},
	{0x66, 0x66, 0x2E, 0x0F, 0x1F, 0x84, 0x00, 0x00, 0x00, 0x00, 0x00},
}

func appendNops(out []byte, count int64, normal bool) []byte {
	if !normal {
		out = append(out, plainNop)
		count--
	}
	if count/maxNopSize > maxNopRun {
		out, count = appendJumpOverNops(out, count)
	}
	if rest := count % maxNopSize; rest != 0 {
		out = append(out, nopPatterns[rest-1]...)
		count -= rest
	}
	longest := nopPatterns[maxNopSize-1]
	for ; count > 0; count -= maxNopSize {
		out = append(out, longest...)
	}
	return out
}

func appendJumpOverNops(out []byte, count int64) ([]byte, int64) {
	disp := count - 2
	if disp <= shortJumpOverMax {
		return append(out, opJmpShort, byte(disp)), disp
	}
	count -= 1 + 4
	out = append(out, opJmpNear)
	out = binary.LittleEndian.AppendUint32(out, uint32(count))
	return out, count
}
