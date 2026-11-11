package asm

type fragKind uint8

const (
	fragFixed fragKind = iota
	fragJump
	fragAlign
	fragPadding
	fragFused
	fragPrefix
)

type jumpForm uint8

const (
	jumpShort jumpForm = iota
	jumpNear
	jumpForced
)

const (
	shortJumpSize = 1
	nearJumpSize  = 4
	nearCondSize  = 5
)

const (
	opJmpShort  = 0xEB
	opJmpNear   = 0xE9
	opJccShort  = 0x70
	opJccNear   = 0x80
	opCallNear  = 0xE8
	opTwoByte   = 0x0F
	jccNearBias = 0x10
)

type lastKind uint8

const (
	lastOther lastKind = iota
	lastDirective
)

type frag struct {
	addr   int64
	last   int64
	off    int64
	sym    *symbol
	start  int32
	fix    int32
	size   int32
	max    int32
	line   int32
	pass   int32
	region int32
	pad    int32
	kind   fragKind
	form   jumpForm
	cond   bool
	normal bool
	power  uint8
}

type textFixup struct {
	at     int32
	size   uint8
	kind   FixupKind
	sym    *symbol
	neg    *symbol
	addend int64
	line   int32
}

func (b *secBuilder) openFrag() *frag {
	return &b.frags[len(b.frags)-1]
}

func (b *secBuilder) startFrag() {
	b.frags = append(b.frags, frag{start: int32(len(b.Data))})
}

func (b *secBuilder) addJump(opcode byte, cond bool, sym *symbol, off int64, line int) {
	b.Data = append(b.Data, opcode)
	f := b.openFrag()
	f.kind = fragJump
	f.cond = cond
	f.sym = sym
	f.off = off
	f.size = shortJumpSize
	f.line = int32(line)
	f.normal = b.last == lastOther
	b.startFrag()
}

func (b *secBuilder) addAlign(power uint8, max int32, line int) {
	f := b.openFrag()
	f.kind = fragAlign
	f.power = power
	f.max = max
	f.line = int32(line)
	f.normal = b.last == lastOther
	b.startFrag()
}

func (b *secBuilder) addFixup(f textFixup) {
	b.fixups = append(b.fixups, f)
}

func (b *secBuilder) currentFrag() int32 {
	return int32(len(b.frags) - 1)
}

func (b *secBuilder) measureFrags() {
	for k := range b.frags {
		end := int32(len(b.Data))
		if k+1 < len(b.frags) {
			end = b.frags[k+1].start
		}
		b.frags[k].fix = end - b.frags[k].start
	}
}

func (f *frag) jumpSize() int32 {
	if f.cond {
		return nearCondSize
	}
	return nearJumpSize
}

func (f *frag) alignPadding(address int64) int64 {
	mask := int64(1)<<f.power - 1
	pad := (address+mask)&^mask - address
	if f.max != 0 && pad > int64(f.max) {
		return 0
	}
	return pad
}
