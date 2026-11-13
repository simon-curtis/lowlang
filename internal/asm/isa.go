package asm

const maxFormSlots = 4

type slotKind uint8

const (
	slotNone slotKind = iota
	slotGP
	slotGPMem
	slotMem
	slotAcc
	slotCL
	slotImm8
	slotImm8S
	slotImmOsz
	slotImm64
	slotImmOne
	slotXMM
	slotXMMMem
	slotYMM
	slotVec
	slotVecMem
)

type slot struct {
	kind slotKind
	size uint8
}

type ref uint8

const (
	r1 ref = iota + 1
	r2
	r3
	r4
)

type opSpace uint8

const (
	spaceBase opSpace = iota
	space0F
	space0F38
	space0F3A
)

type widthMode uint8

const (
	widthFixed widthMode = iota
	widthOpcode
	widthSize
)

const (
	size8 uint8 = 1 << iota
	size16
	size32
	size64
	sizeAll = size8 | size16 | size32 | size64
)

type fuseClass uint8

const (
	fuseNone fuseClass = iota
	fuseTestAnd
	fuseAluCmp
	fuseIncDec
)

type form struct {
	slots   [maxFormSlots]slot
	count   uint8
	prefix  byte
	space   opSpace
	opcode  byte
	tail    byte
	hasTail bool
	modrm   bool
	digit   uint8
	reg     ref
	rm      ref
	vvvv    ref
	is4     ref
	imm     ref
	plusReg ref
	width   widthMode
	sizes   uint8
	rexW    bool
	noW     bool
	vex     bool
	vexW    bool
	vexL256 bool
	swap    byte
	fuse    fuseClass
	branch  bool
}

func newForm(opcode byte, slots ...slot) form {
	f := form{opcode: opcode, sizes: sizeAll, count: uint8(len(slots))}
	copy(f.slots[:], slots)
	return f
}

func (f form) pfx(b byte) form {
	f.prefix = b
	return f
}

func (f form) in(space opSpace) form {
	f.space = space
	return f
}

func (f form) rr(reg, rm ref) form {
	f.modrm = true
	f.reg = reg
	f.rm = rm
	return f
}

func (f form) ext(digit uint8, rm ref) form {
	f.modrm = true
	f.digit = digit
	f.rm = rm
	return f
}

func (f form) plus(r ref) form {
	f.plusReg = r
	return f
}

func (f form) withImm(r ref) form {
	f.imm = r
	return f
}

func (f form) byOpcode() form {
	f.width = widthOpcode
	return f
}

func (f form) bySize() form {
	f.width = widthSize
	return f
}

func (f form) only(mask uint8) form {
	f.sizes = mask
	return f
}

func (f form) w() form {
	f.rexW = true
	return f
}

func (f form) default64() form {
	f.noW = true
	f.sizes = size64
	return f
}

func (f form) tailByte(b byte) form {
	f.tail = b
	f.hasTail = true
	return f
}

func (f form) vexEncoded() form {
	f.vex = true
	return f
}

func (f form) vexW1() form {
	f.vexW = true
	return f
}

func (f form) vexL256Only() form {
	f.vexL256 = true
	return f
}

func (f form) nds(r ref) form {
	f.vvvv = r
	return f
}

func (f form) fourth(r ref) form {
	f.is4 = r
	return f
}

func (f form) swapTo(opcode byte) form {
	f.swap = opcode
	return f
}

func (f form) fusing(class fuseClass) form {
	f.fuse = class
	return f
}

func (f form) branching() form {
	f.branch = true
	return f
}

func gp(size uint8) slot  { return slot{kind: slotGP, size: size} }
func gpm(size uint8) slot { return slot{kind: slotGPMem, size: size} }
func mem(size uint8) slot { return slot{kind: slotMem, size: size} }
func xmm() slot           { return slot{kind: slotXMM} }
func xmmm(size uint8) slot {
	return slot{kind: slotXMMMem, size: size}
}
func vec() slot              { return slot{kind: slotVec} }
func vecm() slot             { return slot{kind: slotVecMem} }
func imm(kind slotKind) slot { return slot{kind: kind} }

var (
	accumulator = slot{kind: slotAcc}
	countCL     = slot{kind: slotCL}
	ymmReg      = slot{kind: slotYMM}
)

type condition struct {
	name string
	code byte
}

var conditions = []condition{
	{"o", 0}, {"no", 1}, {"b", 2}, {"c", 2}, {"nae", 2}, {"ae", 3}, {"nb", 3}, {"nc", 3},
	{"e", 4}, {"z", 4}, {"ne", 5}, {"nz", 5}, {"be", 6}, {"na", 6}, {"a", 7}, {"nbe", 7},
	{"s", 8}, {"ns", 9}, {"p", 10}, {"pe", 10}, {"np", 11}, {"po", 11},
	{"l", 12}, {"nge", 12}, {"ge", 13}, {"nl", 13}, {"le", 14}, {"ng", 14}, {"g", 15}, {"nle", 15},
}

var conditionalJumps = buildConditionalJumps()

func buildConditionalJumps() map[string]byte {
	jumps := make(map[string]byte, len(conditions))
	for _, c := range conditions {
		jumps["j"+c.name] = c.code
	}
	return jumps
}

var isa = buildISA()

type isaBuilder struct {
	forms map[string][]form
}

func (b *isaBuilder) add(name string, forms ...form) {
	b.forms[name] = append(b.forms[name], forms...)
}

func buildISA() map[string][]form {
	b := &isaBuilder{forms: make(map[string][]form, 512)}
	b.addIntegerALU()
	b.addMoves()
	b.addUnary()
	b.addShifts()
	b.addBits()
	b.addConditionals()
	b.addControl()
	b.addFixed()
	b.addSSE()
	b.addVEX()
	return b.forms
}

func (b *isaBuilder) addIntegerALU() {
	alu := []struct {
		name  string
		base  byte
		digit uint8
		fuse  fuseClass
	}{
		{"add", 0x00, 0, fuseAluCmp},
		{"or", 0x08, 1, fuseNone},
		{"adc", 0x10, 2, fuseNone},
		{"sbb", 0x18, 3, fuseNone},
		{"and", 0x20, 4, fuseTestAnd},
		{"sub", 0x28, 5, fuseAluCmp},
		{"xor", 0x30, 6, fuseNone},
		{"cmp", 0x38, 7, fuseAluCmp},
	}
	for _, op := range alu {
		b.add(op.name,
			newForm(op.base, gpm(0), gp(0)).byOpcode().rr(r2, r1).fusing(op.fuse),
			newForm(op.base+2, gp(0), gpm(0)).byOpcode().rr(r1, r2).fusing(op.fuse),
			newForm(0x83, gpm(0), imm(slotImm8S)).bySize().only(size16|size32|size64).ext(op.digit, r1).withImm(r2).fusing(op.fuse),
			newForm(op.base+4, accumulator, imm(slotImmOsz)).byOpcode().withImm(r2).fusing(op.fuse),
			newForm(0x80, gpm(0), imm(slotImmOsz)).byOpcode().ext(op.digit, r1).withImm(r2).fusing(op.fuse),
		)
	}
	b.add("test",
		newForm(0x84, gpm(0), gp(0)).byOpcode().rr(r2, r1).fusing(fuseTestAnd),
		newForm(0x84, gp(0), mem(0)).byOpcode().rr(r1, r2).fusing(fuseTestAnd),
		newForm(0xA8, accumulator, imm(slotImmOsz)).byOpcode().withImm(r2).fusing(fuseTestAnd),
		newForm(0xF6, gpm(0), imm(slotImmOsz)).byOpcode().ext(0, r1).withImm(r2).fusing(fuseTestAnd),
	)
}

func (b *isaBuilder) addMoves() {
	b.add("mov",
		newForm(0x88, gpm(0), gp(0)).byOpcode().rr(r2, r1),
		newForm(0x8A, gp(0), gpm(0)).byOpcode().rr(r1, r2),
		newForm(0xB0, gp(1), imm(slotImm8)).plus(r1).withImm(r2),
		newForm(0xB8, gp(0), imm(slotImmOsz)).bySize().only(size16|size32).plus(r1).withImm(r2),
		newForm(0xC6, gpm(0), imm(slotImmOsz)).byOpcode().ext(0, r1).withImm(r2),
		newForm(0xB8, gp(0), imm(slotImm64)).bySize().only(size64).plus(r1).withImm(r2),
	)
	b.add("movzx",
		newForm(0xB6, gp(0), gpm(1)).in(space0F).bySize().only(size16|size32|size64).rr(r1, r2),
		newForm(0xB7, gp(0), gpm(2)).in(space0F).bySize().only(size32|size64).rr(r1, r2),
	)
	b.add("movsx",
		newForm(0xBE, gp(0), gpm(1)).in(space0F).bySize().only(size16|size32|size64).rr(r1, r2),
		newForm(0xBF, gp(0), gpm(2)).in(space0F).bySize().only(size32|size64).rr(r1, r2),
	)
	b.add("movsxd", newForm(0x63, gp(0), gpm(4)).bySize().only(size64).rr(r1, r2))
	b.add("lea", newForm(0x8D, gp(0), mem(0)).bySize().only(size16|size32|size64).rr(r1, r2))
	b.add("xchg",
		newForm(0x86, gpm(0), gp(0)).byOpcode().rr(r2, r1),
		newForm(0x86, gp(0), mem(0)).byOpcode().rr(r1, r2),
	)
	b.add("cmpxchg", newForm(0xB0, gpm(0), gp(0)).in(space0F).byOpcode().rr(r2, r1))
	b.add("push",
		newForm(0x50, gp(8)).default64().plus(r1),
		newForm(0xFF, gpm(8)).default64().ext(6, r1),
	)
	b.add("pop",
		newForm(0x58, gp(8)).default64().plus(r1),
		newForm(0x8F, gpm(8)).default64().ext(0, r1),
	)
}

func (b *isaBuilder) addUnary() {
	unary := []struct {
		name   string
		opcode byte
		digit  uint8
	}{
		{"not", 0xF6, 2},
		{"neg", 0xF6, 3},
		{"mul", 0xF6, 4},
		{"div", 0xF6, 6},
		{"idiv", 0xF6, 7},
	}
	for _, u := range unary {
		b.add(u.name, newForm(u.opcode, gpm(0)).byOpcode().ext(u.digit, r1))
	}
	b.add("inc", newForm(0xFE, gpm(0)).byOpcode().ext(0, r1).fusing(fuseIncDec))
	b.add("dec", newForm(0xFE, gpm(0)).byOpcode().ext(1, r1).fusing(fuseIncDec))
	b.add("imul",
		newForm(0xF6, gpm(0)).byOpcode().ext(5, r1),
		newForm(0xAF, gp(0), gpm(0)).in(space0F).bySize().only(size16|size32|size64).rr(r1, r2),
		newForm(0x6B, gp(0), gpm(0), imm(slotImm8S)).bySize().only(size16|size32|size64).rr(r1, r2).withImm(r3),
		newForm(0x69, gp(0), gpm(0), imm(slotImmOsz)).bySize().only(size16|size32|size64).rr(r1, r2).withImm(r3),
		newForm(0x6B, gp(0), imm(slotImm8S)).bySize().only(size16|size32|size64).rr(r1, r1).withImm(r2),
		newForm(0x69, gp(0), imm(slotImmOsz)).bySize().only(size16|size32|size64).rr(r1, r1).withImm(r2),
	)
}

func (b *isaBuilder) addShifts() {
	shifts := []struct {
		name  string
		digit uint8
	}{
		{"rol", 0}, {"ror", 1}, {"shl", 4}, {"shr", 5}, {"sar", 7},
	}
	for _, s := range shifts {
		b.add(s.name,
			newForm(0xD0, gpm(0), imm(slotImmOne)).byOpcode().ext(s.digit, r1),
			newForm(0xC0, gpm(0), imm(slotImm8)).byOpcode().ext(s.digit, r1).withImm(r2),
			newForm(0xD2, gpm(0), countCL).byOpcode().ext(s.digit, r1),
		)
	}
}

func (b *isaBuilder) addBits() {
	tests := []struct {
		name  string
		reg   byte
		digit uint8
	}{
		{"bt", 0xA3, 4}, {"bts", 0xAB, 5}, {"btr", 0xB3, 6}, {"btc", 0xBB, 7},
	}
	wide := size16 | size32 | size64
	for _, t := range tests {
		b.add(t.name,
			newForm(t.reg, gpm(0), gp(0)).in(space0F).bySize().only(wide).rr(r2, r1),
			newForm(0xBA, gpm(0), imm(slotImm8)).in(space0F).bySize().only(wide).ext(t.digit, r1).withImm(r2),
		)
	}
	b.add("bsf", newForm(0xBC, gp(0), gpm(0)).in(space0F).bySize().only(wide).rr(r1, r2))
	b.add("bsr", newForm(0xBD, gp(0), gpm(0)).in(space0F).bySize().only(wide).rr(r1, r2))
	b.add("popcnt", newForm(0xB8, gp(0), gpm(0)).pfx(0xF3).in(space0F).bySize().only(wide).rr(r1, r2))
	b.add("rdrand", newForm(0xC7, gp(0)).in(space0F).bySize().only(wide).ext(6, r1))
}

func (b *isaBuilder) addConditionals() {
	wide := size16 | size32 | size64
	for _, c := range conditions {
		b.add("set"+c.name, newForm(0x90+c.code, gpm(1)).in(space0F).ext(0, r1))
		b.add("cmov"+c.name, newForm(0x40+c.code, gp(0), gpm(0)).in(space0F).bySize().only(wide).rr(r1, r2))
	}
}

func (b *isaBuilder) addControl() {
	b.add("call", newForm(0xFF, gpm(8)).default64().ext(2, r1).branching())
	b.add("jmp", newForm(0xFF, gpm(8)).default64().ext(4, r1).branching())
	b.add("ret", newForm(0xC3).branching())
}

func (b *isaBuilder) addFixed() {
	b.add("nop", newForm(0x90))
	b.add("hlt", newForm(0xF4))
	b.add("leave", newForm(0xC9))
	b.add("syscall", newForm(0x05).in(space0F))
	b.add("cpuid", newForm(0xA2).in(space0F))
	b.add("rdtsc", newForm(0x31).in(space0F))
	b.add("pause", newForm(0x90).pfx(0xF3))
	b.add("ud2", newForm(0x0B).in(space0F))
	b.add("cqo", newForm(0x99).w())
	b.add("cld", newForm(0xFC))
	b.add("std", newForm(0xFD))
	b.add("mfence", newForm(0xAE).in(space0F).tailByte(0xF0))
	b.add("xgetbv", newForm(0x01).in(space0F).tailByte(0xD0))
	b.add("stosq", newForm(0xAB).w())
	b.add("movsb", newForm(0xA4))
	b.add("cmpsb", newForm(0xA6))
	b.add("prefetcht0", newForm(0x18, mem(0)).in(space0F).ext(1, r1))
}

type sseOp struct {
	name   string
	prefix byte
	opcode byte
	mem    uint8
}

func (b *isaBuilder) addSSE() {
	b.addSSEMoves()
	b.addSSEArithmetic()
	b.addSSEConversions()
	b.addSSEPacked()
}

func (b *isaBuilder) addSSEMoves() {
	moves := []struct {
		name   string
		prefix byte
		load   byte
		store  byte
		mem    uint8
	}{
		{"movaps", 0, 0x28, 0x29, 16},
		{"movups", 0, 0x10, 0x11, 16},
		{"movdqa", 0x66, 0x6F, 0x7F, 16},
		{"movdqu", 0xF3, 0x6F, 0x7F, 16},
		{"movss", 0xF3, 0x10, 0x11, 4},
		{"movsd", 0xF2, 0x10, 0x11, 8},
	}
	for _, m := range moves {
		b.add(m.name,
			newForm(m.load, xmm(), xmmm(m.mem)).pfx(m.prefix).in(space0F).rr(r1, r2),
			newForm(m.store, xmmm(m.mem), xmm()).pfx(m.prefix).in(space0F).rr(r2, r1),
		)
	}
	b.add("movd",
		newForm(0x6E, xmm(), gpm(4)).pfx(0x66).in(space0F).rr(r1, r2),
		newForm(0x7E, gpm(4), xmm()).pfx(0x66).in(space0F).rr(r2, r1),
	)
	b.add("movq",
		newForm(0x7E, xmm(), xmmm(8)).pfx(0xF3).in(space0F).rr(r1, r2),
		newForm(0xD6, xmmm(8), xmm()).pfx(0x66).in(space0F).rr(r2, r1),
		newForm(0x6E, xmm(), gp(8)).pfx(0x66).in(space0F).w().rr(r1, r2),
		newForm(0x7E, gp(8), xmm()).pfx(0x66).in(space0F).w().rr(r2, r1),
	)
	b.add("movmskpd", newForm(0x50, gp(4), xmm()).pfx(0x66).in(space0F).rr(r1, r2))
	b.add("movmskps", newForm(0x50, gp(4), xmm()).in(space0F).rr(r1, r2))
	b.add("pmovmskb", newForm(0xD7, gp(4), xmm()).pfx(0x66).in(space0F).rr(r1, r2))
}

func (b *isaBuilder) addSSEArithmetic() {
	ops := []sseOp{
		{"addps", 0, 0x58, 16}, {"addpd", 0x66, 0x58, 16}, {"addss", 0xF3, 0x58, 4}, {"addsd", 0xF2, 0x58, 8},
		{"mulps", 0, 0x59, 16}, {"mulpd", 0x66, 0x59, 16}, {"mulss", 0xF3, 0x59, 4}, {"mulsd", 0xF2, 0x59, 8},
		{"subps", 0, 0x5C, 16}, {"subpd", 0x66, 0x5C, 16}, {"subss", 0xF3, 0x5C, 4}, {"subsd", 0xF2, 0x5C, 8},
		{"minps", 0, 0x5D, 16}, {"minpd", 0x66, 0x5D, 16},
		{"maxps", 0, 0x5F, 16}, {"maxpd", 0x66, 0x5F, 16},
		{"divps", 0, 0x5E, 16}, {"divpd", 0x66, 0x5E, 16}, {"divss", 0xF3, 0x5E, 4}, {"divsd", 0xF2, 0x5E, 8},
		{"sqrtss", 0xF3, 0x51, 4}, {"sqrtsd", 0xF2, 0x51, 8},
		{"ucomiss", 0, 0x2E, 4}, {"ucomisd", 0x66, 0x2E, 8},
		{"xorps", 0, 0x57, 16},
		{"cvtss2sd", 0xF3, 0x5A, 4}, {"cvtsd2ss", 0xF2, 0x5A, 8},
	}
	for _, op := range ops {
		b.add(op.name, newForm(op.opcode, xmm(), xmmm(op.mem)).pfx(op.prefix).in(space0F).rr(r1, r2))
	}
	b.add("roundsd", newForm(0x0B, xmm(), xmmm(8), imm(slotImm8)).pfx(0x66).in(space0F3A).rr(r1, r2).withImm(r3))
	b.add("roundss", newForm(0x0A, xmm(), xmmm(4), imm(slotImm8)).pfx(0x66).in(space0F3A).rr(r1, r2).withImm(r3))
	b.add("cmppd", newForm(0xC2, xmm(), xmmm(16), imm(slotImm8)).pfx(0x66).in(space0F).rr(r1, r2).withImm(r3))
	b.add("cmpps", newForm(0xC2, xmm(), xmmm(16), imm(slotImm8)).in(space0F).rr(r1, r2).withImm(r3))
	b.add("pshufd", newForm(0x70, xmm(), xmmm(16), imm(slotImm8)).pfx(0x66).in(space0F).rr(r1, r2).withImm(r3))
}

func (b *isaBuilder) addSSEConversions() {
	convert := []struct {
		name   string
		prefix byte
		opcode byte
		mem    uint8
	}{
		{"cvtsd2si", 0xF2, 0x2D, 8},
		{"cvttsd2si", 0xF2, 0x2C, 8},
		{"cvttss2si", 0xF3, 0x2C, 4},
	}
	narrow := size32 | size64
	for _, c := range convert {
		b.add(c.name, newForm(c.opcode, gp(0), xmmm(c.mem)).pfx(c.prefix).in(space0F).bySize().only(narrow).rr(r1, r2))
	}
	b.add("cvtsi2sd", newForm(0x2A, xmm(), gpm(0)).pfx(0xF2).in(space0F).bySize().only(narrow).rr(r1, r2))
	b.add("cvtsi2ss", newForm(0x2A, xmm(), gpm(0)).pfx(0xF3).in(space0F).bySize().only(narrow).rr(r1, r2))
}

func (b *isaBuilder) addSSEPacked() {
	packed := []struct {
		name   string
		opcode byte
	}{
		{"packsswb", 0x63}, {"paddb", 0xFC}, {"paddw", 0xFD}, {"paddd", 0xFE}, {"paddq", 0xD4},
		{"pand", 0xDB}, {"pandn", 0xDF}, {"por", 0xEB}, {"pxor", 0xEF},
		{"pcmpeqb", 0x74}, {"pcmpeqw", 0x75}, {"pcmpeqd", 0x76}, {"pcmpgtb", 0x64}, {"pcmpgtw", 0x65}, {"pcmpgtd", 0x66},
		{"pmaxub", 0xDE}, {"pminub", 0xDA}, {"pmullw", 0xD5},
		{"psllw", 0xF1}, {"pslld", 0xF2}, {"psllq", 0xF3}, {"psrlw", 0xD1}, {"psrld", 0xD2}, {"psrlq", 0xD3}, {"psrad", 0xE2},
		{"psubb", 0xF8}, {"psubw", 0xF9}, {"psubd", 0xFA}, {"psubq", 0xFB},
		{"punpcklbw", 0x60}, {"punpcklwd", 0x61}, {"punpcklqdq", 0x6C},
	}
	for _, p := range packed {
		b.add(p.name, newForm(p.opcode, xmm(), xmmm(16)).pfx(0x66).in(space0F).rr(r1, r2))
	}
}

type vexOp struct {
	name   string
	prefix byte
	space  opSpace
	opcode byte
}

func (b *isaBuilder) addVEX() {
	b.addVEXMoves()
	b.addVEXThreeOperand()
	b.addVEXMisc()
}

func (b *isaBuilder) addVEXMoves() {
	moves := []struct {
		name   string
		prefix byte
	}{
		{"vmovdqa", 0x66},
		{"vmovdqu", 0xF3},
	}
	for _, m := range moves {
		b.add(m.name,
			newForm(0x6F, vec(), vecm()).vexEncoded().pfx(m.prefix).in(space0F).rr(r1, r2).swapTo(0x7F),
			newForm(0x7F, vecm(), vec()).vexEncoded().pfx(m.prefix).in(space0F).rr(r2, r1),
		)
	}
}

func (b *isaBuilder) addVEXThreeOperand() {
	ops := []vexOp{
		{"vaddps", 0, space0F, 0x58}, {"vaddpd", 0x66, space0F, 0x58},
		{"vmulps", 0, space0F, 0x59}, {"vmulpd", 0x66, space0F, 0x59},
		{"vsubps", 0, space0F, 0x5C}, {"vsubpd", 0x66, space0F, 0x5C},
		{"vminps", 0, space0F, 0x5D}, {"vminpd", 0x66, space0F, 0x5D},
		{"vdivps", 0, space0F, 0x5E}, {"vdivpd", 0x66, space0F, 0x5E},
		{"vmaxps", 0, space0F, 0x5F}, {"vmaxpd", 0x66, space0F, 0x5F},
		{"vpacksswb", 0x66, space0F, 0x63},
		{"vpaddb", 0x66, space0F, 0xFC}, {"vpaddw", 0x66, space0F, 0xFD}, {"vpaddd", 0x66, space0F, 0xFE}, {"vpaddq", 0x66, space0F, 0xD4},
		{"vpand", 0x66, space0F, 0xDB}, {"vpandn", 0x66, space0F, 0xDF}, {"vpor", 0x66, space0F, 0xEB}, {"vpxor", 0x66, space0F, 0xEF},
		{"vpcmpeqb", 0x66, space0F, 0x74}, {"vpcmpeqw", 0x66, space0F, 0x75}, {"vpcmpeqd", 0x66, space0F, 0x76}, {"vpcmpeqq", 0x66, space0F38, 0x29},
		{"vpcmpgtb", 0x66, space0F, 0x64}, {"vpcmpgtw", 0x66, space0F, 0x65}, {"vpcmpgtd", 0x66, space0F, 0x66}, {"vpcmpgtq", 0x66, space0F38, 0x37},
		{"vpmaxsd", 0x66, space0F38, 0x3D}, {"vpmaxud", 0x66, space0F38, 0x3F}, {"vpmaxub", 0x66, space0F, 0xDE}, {"vpmaxuw", 0x66, space0F38, 0x3E},
		{"vpminsd", 0x66, space0F38, 0x39}, {"vpminub", 0x66, space0F, 0xDA}, {"vpminud", 0x66, space0F38, 0x3B}, {"vpminuw", 0x66, space0F38, 0x3A},
		{"vpmulld", 0x66, space0F38, 0x40}, {"vpmullw", 0x66, space0F, 0xD5},
		{"vpshufb", 0x66, space0F38, 0x00},
		{"vpsubb", 0x66, space0F, 0xF8}, {"vpsubw", 0x66, space0F, 0xF9}, {"vpsubd", 0x66, space0F, 0xFA}, {"vpsubq", 0x66, space0F, 0xFB},
	}
	for _, op := range ops {
		b.add(op.name, newForm(op.opcode, vec(), vec(), vecm()).vexEncoded().pfx(op.prefix).in(op.space).rr(r1, r3).nds(r2))
	}
	shifts := []vexOp{
		{"vpsllw", 0x66, space0F, 0xF1}, {"vpslld", 0x66, space0F, 0xF2}, {"vpsllq", 0x66, space0F, 0xF3},
		{"vpsrlw", 0x66, space0F, 0xD1}, {"vpsrld", 0x66, space0F, 0xD2}, {"vpsrlq", 0x66, space0F, 0xD3}, {"vpsrad", 0x66, space0F, 0xE2},
	}
	for _, op := range shifts {
		b.add(op.name, newForm(op.opcode, vec(), vec(), xmmm(16)).vexEncoded().pfx(op.prefix).in(op.space).rr(r1, r3).nds(r2))
	}
	b.add("vcmppd", newForm(0xC2, vec(), vec(), vecm(), imm(slotImm8)).vexEncoded().pfx(0x66).in(space0F).rr(r1, r3).nds(r2).withImm(r4))
	b.add("vcmpps", newForm(0xC2, vec(), vec(), vecm(), imm(slotImm8)).vexEncoded().in(space0F).rr(r1, r3).nds(r2).withImm(r4))
	b.add("vpblendvb", newForm(0x4C, vec(), vec(), vecm(), vec()).vexEncoded().pfx(0x66).in(space0F3A).rr(r1, r3).nds(r2).fourth(r4))
}

func (b *isaBuilder) addVEXMisc() {
	b.add("vzeroupper", newForm(0x77).vexEncoded().in(space0F))
	b.add("vextracti128", newForm(0x39, xmmm(16), ymmReg, imm(slotImm8)).vexEncoded().pfx(0x66).in(space0F3A).vexL256Only().rr(r2, r1).withImm(r3))
	b.add("vmovq", newForm(0x6E, xmm(), gp(8)).vexEncoded().pfx(0x66).in(space0F).vexW1().rr(r1, r2))
	b.add("vmovmskpd", newForm(0x50, gp(4), vec()).vexEncoded().pfx(0x66).in(space0F).rr(r1, r2))
	b.add("vmovmskps", newForm(0x50, gp(4), vec()).vexEncoded().in(space0F).rr(r1, r2))
	b.add("vpmovmskb", newForm(0xD7, gp(4), vec()).vexEncoded().pfx(0x66).in(space0F).rr(r1, r2))
	b.add("vpbroadcastb", newForm(0x78, vec(), xmmm(1)).vexEncoded().pfx(0x66).in(space0F38).rr(r1, r2))
	b.add("vpbroadcastw", newForm(0x79, vec(), xmmm(2)).vexEncoded().pfx(0x66).in(space0F38).rr(r1, r2))
	b.add("vpbroadcastd", newForm(0x58, vec(), xmmm(4)).vexEncoded().pfx(0x66).in(space0F38).rr(r1, r2))
	b.add("vpbroadcastq", newForm(0x59, vec(), xmmm(8)).vexEncoded().pfx(0x66).in(space0F38).rr(r1, r2))
}
