package asm

import "encoding/binary"

const maxInstructionLength = 15

const (
	rexBase  = 0x40
	rexWBit  = 8
	rexRBit  = 4
	rexXBit  = 2
	rexBBit  = 1
	prefixOS = 0x66
	prefixF2 = 0xF2
	prefixF3 = 0xF3
	prefixLk = 0xF0
)

var segmentPrefixes = [6]byte{0x26, 0x2E, 0x36, 0x3E, 0x64, 0x65}

type encoding struct {
	bytes       [maxInstructionLength + 1]byte
	length      int
	prefixes    int
	segment     byte
	hasFixup    bool
	fixupAt     int
	fixupKind   FixupKind
	fixupSymbol string
	fixupAddend int64
}

func (e *encoding) put(b byte) {
	e.bytes[e.length] = b
	e.length++
}

func (e *encoding) putInt(size int, v int64) {
	switch size {
	case 1:
		e.bytes[e.length] = byte(v)
	case 2:
		binary.LittleEndian.PutUint16(e.bytes[e.length:], uint16(v))
	case 4:
		binary.LittleEndian.PutUint32(e.bytes[e.length:], uint32(v))
	default:
		binary.LittleEndian.PutUint64(e.bytes[e.length:], uint64(v))
	}
	e.length += size
}

type addressing struct {
	mod      uint8
	rm       uint8
	sib      uint8
	hasSIB   bool
	dispSize int
	disp     int64
	rexX     uint8
	rexB     uint8
	ripValue Value
	ripReloc bool
}

func sizeOfClass(c RegClass) uint8 {
	switch c {
	case ClassGP8, ClassGP8High:
		return 1
	case ClassGP16:
		return 2
	case ClassGP32:
		return 4
	case ClassGP64:
		return 8
	}
	return 0
}

func sizeBit(size uint8) uint8 {
	switch size {
	case 1:
		return size8
	case 2:
		return size16
	case 4:
		return size32
	case 8:
		return size64
	}
	return 0
}

func signedAt(v int64, size uint8) int64 {
	switch size {
	case 2:
		return int64(int16(v))
	case 4:
		return int64(int32(v))
	}
	return v
}

func (s slot) accepts(op *Operand, vec *RegClass) (uint8, bool) {
	switch s.kind {
	case slotGP:
		return s.acceptsGP(op)
	case slotGPMem:
		if op.Kind == OpMem {
			return s.acceptsPointer(op)
		}
		return s.acceptsGP(op)
	case slotMem:
		return 0, op.Kind == OpMem
	case slotAcc:
		if op.Kind != OpReg || op.Reg.Num() != 0 {
			return 0, false
		}
		class := op.Reg.Class()
		return sizeOfClass(class), class != ClassGP8High && sizeOfClass(class) != 0
	case slotCL:
		return 0, op.Kind == OpReg && op.Reg.Class() == ClassGP8 && op.Reg.Num() == 1
	case slotXMM:
		return 0, op.Kind == OpReg && op.Reg.Class() == ClassXMM
	case slotXMMMem:
		return 0, op.Kind == OpMem || (op.Kind == OpReg && op.Reg.Class() == ClassXMM)
	case slotYMM:
		return 0, op.Kind == OpReg && op.Reg.Class() == ClassYMM
	case slotVec:
		return 0, op.Kind == OpReg && acceptsVector(op.Reg.Class(), vec)
	case slotVecMem:
		if op.Kind == OpMem {
			return 0, true
		}
		return 0, op.Kind == OpReg && acceptsVector(op.Reg.Class(), vec)
	}
	return 0, false
}

func acceptsVector(class RegClass, vec *RegClass) bool {
	if class != ClassXMM && class != ClassYMM {
		return false
	}
	if *vec != ClassNone && *vec != class {
		return false
	}
	*vec = class
	return true
}

func (s slot) acceptsGP(op *Operand) (uint8, bool) {
	if op.Kind != OpReg {
		return 0, false
	}
	size := sizeOfClass(op.Reg.Class())
	if size == 0 {
		return 0, false
	}
	if s.size != 0 {
		return 0, size == s.size
	}
	return size, true
}

func (s slot) acceptsPointer(op *Operand) (uint8, bool) {
	p := uint8(op.Size)
	if s.size != 0 {
		return 0, p == 0 || p == s.size
	}
	if p == 0 {
		return 0, true
	}
	return p, sizeBit(p) != 0
}

func (s slot) acceptsImmediate(op *Operand, size uint8) bool {
	if op.Kind != OpImm || !op.Value.IsConst() {
		return false
	}
	v := op.Value.Add
	switch s.kind {
	case slotImm8:
		return v >= -128 && v <= 255
	case slotImm8S:
		w := signedAt(v, size)
		return w >= -128 && w <= 127
	case slotImmOsz:
		return immediateFits(v, size)
	case slotImm64:
		return true
	case slotImmOne:
		return v == 1
	}
	return false
}

func immediateFits(v int64, size uint8) bool {
	switch size {
	case 1:
		return v >= -128 && v <= 255
	case 2:
		return v >= -32768 && v <= 65535
	case 4:
		return v >= -1<<31 && v <= 1<<32-1
	}
	return v >= -1<<31 && v < 1<<31
}

func (s slot) immediateBytes(size uint8) int {
	switch s.kind {
	case slotImm8, slotImm8S:
		return 1
	case slotImmOsz:
		switch size {
		case 1:
			return 1
		case 2:
			return 2
		}
		return 4
	case slotImm64:
		return 8
	}
	return 0
}

func (f *form) match(ops []Operand) (uint8, bool) {
	if len(ops) != int(f.count) {
		return 0, false
	}
	var size uint8
	var vec RegClass
	for i := range ops {
		s := f.slots[i]
		if s.kind >= slotImm8 && s.kind <= slotImmOne {
			continue
		}
		contributed, ok := s.accepts(&ops[i], &vec)
		if !ok {
			return 0, false
		}
		if contributed != 0 {
			if size != 0 && size != contributed {
				return 0, false
			}
			size = contributed
		}
	}
	if f.width != widthFixed && (size == 0 || f.sizes&sizeBit(size) == 0) {
		return 0, false
	}
	for i := range ops {
		s := f.slots[i]
		if s.kind >= slotImm8 && s.kind <= slotImmOne && !s.acceptsImmediate(&ops[i], size) {
			return 0, false
		}
	}
	return size, true
}

func (f *form) immediateSize(size uint8) int {
	total := 0
	for i := 0; i < int(f.count); i++ {
		total += f.slots[i].immediateBytes(size)
	}
	return total
}

func (f *form) vectorLength(ops []Operand) uint8 {
	if f.vexL256 {
		return 1
	}
	for i := 0; i < int(f.count); i++ {
		k := f.slots[i].kind
		if (k == slotVec || k == slotVecMem) && ops[i].Kind == OpReg && ops[i].Reg.Class() == ClassYMM {
			return 1
		}
	}
	return 0
}

func (f *form) needsRexW(size uint8) bool {
	if f.rexW {
		return true
	}
	return f.width != widthFixed && !f.noW && size == 8
}

func (f *form) swapsOperands(ops []Operand) bool {
	if f.swap == 0 || f.count != 2 || ops[0].Kind != OpReg || ops[1].Kind != OpReg {
		return false
	}
	return ops[1].Reg.Num() >= 8 && ops[0].Reg.Num() < 8
}

func (a *assembler) memoryAddressing(op *Operand) addressing {
	var m addressing
	if op.Base == regRIP {
		m.rm = 5
		m.dispSize = 4
		m.disp = op.Value.Add
		if !op.Value.IsConst() {
			m.ripValue = op.Value
			m.ripReloc = true
		}
		return m
	}
	if !op.Value.IsConst() {
		a.fail("symbolic displacement needs a rip-relative operand")
	}
	disp := op.Value.Add
	if disp < -1<<31 || disp >= 1<<31 {
		a.fail("displacement %d does not fit in 32 bits", disp)
	}
	m.disp = disp
	a.checkAddressRegisters(op)
	switch {
	case op.Base == RegNone && op.Index == RegNone:
		m.rm, m.hasSIB, m.sib, m.dispSize = 4, true, 0x25, 4
	case op.Base == RegNone:
		m.rm, m.hasSIB, m.dispSize = 4, true, 4
		m.sib = scaleBits(op.Scale)<<6 | (op.Index.Num()&7)<<3 | 5
		m.rexX = rexXBit * (op.Index.Num() >> 3)
	default:
		a.baseAddressing(&m, op)
	}
	return m
}

func (a *assembler) checkAddressRegisters(op *Operand) {
	if op.Base != RegNone && op.Base.Class() != ClassGP64 {
		a.fail("address register %s is not 64-bit", op.Base)
	}
	if op.Index != RegNone && op.Index.Class() != ClassGP64 {
		a.fail("index register %s is not 64-bit", op.Index)
	}
	if op.Index != RegNone && op.Index.Num() == 4 {
		a.fail("rsp cannot be an index register")
	}
}

func (a *assembler) baseAddressing(m *addressing, op *Operand) {
	base := op.Base.Num()
	m.rexB = rexBBit * (base >> 3)
	needSIB := op.Index != RegNone || base&7 == 4
	switch {
	case m.disp == 0 && base&7 != 5:
		m.mod, m.dispSize = 0, 0
	case m.disp >= -128 && m.disp <= 127:
		m.mod, m.dispSize = 1, 1
	default:
		m.mod, m.dispSize = 2, 4
	}
	if !needSIB {
		m.rm = base & 7
		return
	}
	m.rm, m.hasSIB = 4, true
	index := uint8(4)
	scale := uint8(0)
	if op.Index != RegNone {
		index = op.Index.Num()
		scale = scaleBits(op.Scale)
		m.rexX = rexXBit * (index >> 3)
	}
	m.sib = scale<<6 | (index&7)<<3 | base&7
}

func scaleBits(scale int) uint8 {
	switch scale {
	case 2:
		return 1
	case 4:
		return 2
	case 8:
		return 3
	}
	return 0
}

func (a *assembler) encode(f *form, size uint8, ops []Operand, prefix Prefix) encoding {
	var e encoding
	opcode := f.opcode
	regRef, rmRef := f.reg, f.rm
	if f.swapsOperands(ops) {
		opcode = f.swap
		regRef, rmRef = rmRef, regRef
	}
	if f.width == widthOpcode && size != 1 {
		opcode |= 1
	}
	var rex uint8
	if f.needsRexW(size) {
		rex |= rexWBit
	}
	var mem addressing
	var seg Reg
	regField := f.digit
	if regRef != 0 {
		r := ops[regRef-1].Reg
		regField = r.Num()
		rex |= rexRBit * (regField >> 3)
	}
	rmIsMemory := false
	if f.modrm {
		rmOp := &ops[rmRef-1]
		if rmOp.Kind == OpMem {
			rmIsMemory = true
			seg = rmOp.Seg
			mem = a.memoryAddressing(rmOp)
			rex |= mem.rexX | mem.rexB
		} else {
			mem.mod = 3
			mem.rm = rmOp.Reg.Num() & 7
			rex |= rexBBit * (rmOp.Reg.Num() >> 3)
		}
	}
	if f.plusReg != 0 {
		num := ops[f.plusReg-1].Reg.Num()
		opcode += num & 7
		rex |= rexBBit * (num >> 3)
	}
	rexRequired := a.registerRestrictions(f, ops)
	var vvvv uint8
	if f.vvvv != 0 {
		vvvv = ops[f.vvvv-1].Reg.Num()
	}
	if rmIsMemory && seg != RegNone {
		e.segment = segmentPrefixes[seg.Num()]
		e.put(e.segment)
	}
	if f.vex {
		a.putVEX(&e, f, ops, rex, vvvv)
	} else {
		a.putLegacy(&e, f, size, prefix, rex, rexRequired)
	}
	if !f.vex {
		e.putOpcodeBytes(f)
	}
	e.prefixes = e.length
	e.put(opcode)
	if f.hasTail {
		e.put(f.tail)
	}
	if f.modrm {
		e.put(mem.mod<<6 | (regField&7)<<3 | mem.rm)
		if mem.hasSIB {
			e.put(mem.sib)
		}
	}
	trailing := f.immediateSize(size)
	if f.is4 != 0 {
		trailing++
	}
	a.putDisplacement(&e, f, &mem, opcode, rex != 0 || rexRequired, regField, trailing)
	a.putImmediates(&e, f, size, ops)
	return e
}

func (a *assembler) registerRestrictions(f *form, ops []Operand) bool {
	needsRex := false
	hasHigh := false
	for i := 0; i < int(f.count); i++ {
		if ops[i].Kind != OpReg {
			continue
		}
		switch ops[i].Reg.Class() {
		case ClassGP8High:
			hasHigh = true
		case ClassGP8:
			n := ops[i].Reg.Num()
			needsRex = needsRex || (n >= 4 && n < 8)
		}
	}
	if hasHigh && needsRex {
		a.fail("a high-byte register cannot be encoded with a REX prefix")
	}
	return needsRex
}

func (e *encoding) putOpcodeBytes(f *form) {
	switch f.space {
	case space0F:
		e.put(0x0F)
	case space0F38:
		e.put(0x0F)
		e.put(0x38)
	case space0F3A:
		e.put(0x0F)
		e.put(0x3A)
	}
}

func (a *assembler) putLegacy(e *encoding, f *form, size uint8, prefix Prefix, rex uint8, rexRequired bool) {
	if f.prefix == prefixOS || (f.width != widthFixed && size == 2) {
		e.put(prefixOS)
	}
	switch {
	case f.prefix == prefixF2 || f.prefix == prefixF3:
		e.put(f.prefix)
	case prefix&PrefixRep != 0:
		e.put(prefixF3)
	case prefix&PrefixRepne != 0:
		e.put(prefixF2)
	}
	if prefix&PrefixLock != 0 {
		e.put(prefixLk)
	}
	if rex != 0 || rexRequired {
		e.put(rexBase | rex)
	}
}

func (a *assembler) putVEX(e *encoding, f *form, ops []Operand, rex uint8, vvvv uint8) {
	var pp uint8
	switch f.prefix {
	case prefixOS:
		pp = 1
	case prefixF3:
		pp = 2
	case prefixF2:
		pp = 3
	}
	length := f.vectorLength(ops)
	inverted := (^vvvv & 0xF) << 3
	w := f.vexW || rex&rexWBit != 0
	if !w && f.space == space0F && rex&(rexXBit|rexBBit) == 0 {
		r := uint8(0x80)
		if rex&rexRBit != 0 {
			r = 0
		}
		e.put(0xC5)
		e.put(r | inverted | length<<2 | pp)
		return
	}
	e.put(0xC4)
	e.put((^rex&7)<<5 | uint8(f.space))
	var wBit uint8
	if w {
		wBit = 0x80
	}
	e.put(wBit | inverted | length<<2 | pp)
}

func (a *assembler) putDisplacement(e *encoding, f *form, m *addressing, opcode byte, rexPresent bool, regField uint8, trailing int) {
	if m.dispSize == 0 {
		return
	}
	if !m.ripReloc {
		e.putInt(m.dispSize, m.disp)
		return
	}
	e.hasFixup = true
	e.fixupAt = e.length
	e.fixupSymbol = m.ripValue.Sym
	e.fixupAddend = m.ripValue.Add - 4 - int64(trailing)
	e.fixupKind = PC32
	if m.ripValue.Reloc == RelocGOTPCREL {
		e.fixupKind = a.gotKind(f, opcode, rexPresent, regField)
	}
	e.putInt(4, 0)
}

func (a *assembler) gotKind(f *form, opcode byte, rexPresent bool, regField uint8) FixupKind {
	if f.prefix == prefixOS || f.space != spaceBase {
		return GOTPCREL
	}
	relaxable := (opcode == 0xFF && (regField == 2 || regField == 4 || regField == 6)) ||
		opcode == 0x8B || opcode == 0x85 || opcode|0x38 == 0x3B
	switch {
	case !relaxable:
		return GOTPCREL
	case rexPresent:
		return REXGOTPCRELX
	}
	return GOTPCRELX
}

func (a *assembler) putImmediates(e *encoding, f *form, size uint8, ops []Operand) {
	for i := 0; i < int(f.count); i++ {
		n := f.slots[i].immediateBytes(size)
		if n == 0 {
			continue
		}
		e.putInt(n, ops[i].Value.Add)
	}
	if f.is4 != 0 {
		e.put(ops[f.is4-1].Reg.Num() << 4)
	}
}
