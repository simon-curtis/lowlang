package amd64

import (
	"fmt"
	"strconv"
	"strings"

	"lowlang/internal/ir"
	"lowlang/internal/types"
)

type tier int

const (
	baseline tier = iota
	avx2
)

const (
	vectorSlotBytes    = 32
	vectorScratchSlots = 3

	vectorScratchWords = vectorScratchSlots*vectorSlotBytes/8 + 1

	vectorRegisters     = 16
	vectorWorkRegisters = 8
)

func usesVectors(f *ir.Func) bool {
	for _, b := range f.Blocks {
		for _, v := range b.Values {
			if v.Op.IsVector() {
				return true
			}
		}
	}
	return false
}

type maddr struct {
	base string
	disp int
}

func (m maddr) at(off int) string {
	return fmt.Sprintf("[%s%+d]", m.base, m.disp+off)
}

func (g *Generator) vectorAddress(fr *frame, in *instr, k int) maddr {
	v := in.value.Args[k]
	if v.Op == ir.OpSlot {
		return maddr{"rbp", fr.slot(int(v.Aux))}
	}
	loc := fr.loc(in.args[k])
	if isReg(loc) {
		return maddr{loc, 0}
	}
	g.move("r10", loc)
	return maddr{"r10", 0}
}

func (fr *frame) scratch(k int) maddr {
	return maddr{"rbp", fr.vectorScratch + vectorSlotBytes*k}
}

type lanes struct {
	t      *types.Type
	halves int
	wide   bool
}

func (g *Generator) shape(t *types.Type) lanes {
	if g.tier == avx2 {
		return lanes{t: t, halves: 1, wide: t.Bits == 256}
	}
	return lanes{t: t, halves: t.Bits / 128}
}

func (l lanes) reg(base, half int) string {
	if l.wide {
		return fmt.Sprintf("ymm%d", base+half)
	}
	return fmt.Sprintf("xmm%d", base+half)
}

func (g *Generator) loadVector(l lanes, base int, m maddr) {
	for h := range l.halves {
		g.emit("%s %s, %s ptr %s", g.vmov(), l.reg(base, h), g.memWidth(l), m.at(16*h))
	}
}

func (g *Generator) storeVector(l lanes, base int, m maddr) {
	for h := range l.halves {
		g.emit("%s %s ptr %s, %s", g.vmov(), g.memWidth(l), m.at(16*h), l.reg(base, h))
	}
}

func (g *Generator) vmov() string {
	if g.tier == avx2 {
		return "vmovdqu"
	}
	return "movdqu"
}

func (g *Generator) op2(l lanes, mnemonic string, dst, src int) {
	for h := range l.halves {
		if g.tier == avx2 {
			g.emit("v%s %s, %s, %s", mnemonic, l.reg(dst, h), l.reg(dst, h), l.reg(src, h))
		} else {
			g.emit("%s %s, %s", mnemonic, l.reg(dst, h), l.reg(src, h))
		}
	}
}

func (g *Generator) cmpFloat(l lanes, dst, src, predicate int) {
	suffix := floatSuffix(l.t)
	for h := range l.halves {
		if g.tier == avx2 {
			g.emit("vcmp%s %s, %s, %s, %d", suffix, l.reg(dst, h), l.reg(dst, h), l.reg(src, h), predicate)
		} else {
			g.emit("cmp%s %s, %s, %d", suffix, l.reg(dst, h), l.reg(src, h), predicate)
		}
	}
}

func (g *Generator) copyRegs(l lanes, dst, src int) {
	for h := range l.halves {
		if g.tier == avx2 {
			g.emit("vmovdqa %s, %s", l.reg(dst, h), l.reg(src, h))
		} else {
			g.emit("movdqa %s, %s", l.reg(dst, h), l.reg(src, h))
		}
	}
}

func intSuffix(t *types.Type) string {
	return map[int]string{8: "b", 16: "w", 32: "d", 64: "q"}[t.Elem.Bits]
}

func floatSuffix(t *types.Type) string {
	if t.Elem.Bits == 32 {
		return "ps"
	}
	return "pd"
}

func signLabel(bits int) string {
	return fmt.Sprintf("rt_vsign%d", bits)
}

func (g *Generator) allOnes(l lanes, reg int) {
	g.op2(l, "pcmpeqd", reg, reg)
}

func (g *Generator) bias(l lanes, reg int) {
	for h := range l.halves {
		if g.tier == avx2 {
			g.emit("vpxor %s, %s, %s ptr [rip+%s]", l.reg(reg, h), l.reg(reg, h), g.memWidth(l), signLabel(l.t.Elem.Bits))
		} else {
			g.emit("movdqu xmm7, xmmword ptr [rip+%s]", signLabel(l.t.Elem.Bits))
			g.emit("pxor %s, xmm7", l.reg(reg, h))
		}
	}
}

func (g *Generator) memWidth(l lanes) string {
	if l.wide {
		return "ymmword"
	}
	return "xmmword"
}

func native(op ir.VectorOp, t *types.Type, at tier) bool {
	bits := t.Elem.Bits
	float := t.Elem.Float()
	switch op {
	case ir.VMul:
		return float || bits == 16 || (bits == 32 && at == avx2)
	case ir.VMin, ir.VMax:
		return float || (bits == 8 && t.Elem == types.U8) || (at == avx2 && bits < 64)
	case ir.VEq, ir.VNe, ir.VLt, ir.VLe, ir.VGt, ir.VGe:
		return float || bits < 64 || at == avx2
	case ir.VShuffle:
		return at == avx2
	case ir.VShl, ir.VShr:
		if bits == 8 {
			return false
		}
		return !(op == ir.VShr && t.Elem == types.I64)
	}
	return true
}

func (g *Generator) vector(fr *frame, in *instr) {
	v := in.value
	t := v.Type
	op := ir.VectorOp(v.Aux)
	l := g.shape(t)
	if v.Op == ir.OpVectorScalar {
		g.vectorMask(fr, in, g.shape(v.From))
		return
	}
	switch op {
	case ir.VCopy:
		g.copyVector(fr, in, l)
	case ir.VSplat:
		g.splat(fr, in, l)
		g.vectorResult(fr, in, l, 0)
	case ir.VSelect:
		g.source(fr, in, 1, l, 0)
		g.source(fr, in, 2, l, 2)
		g.source(fr, in, 3, l, 4)
		g.selectRegs(l)
		g.vectorResult(fr, in, l, 4)
	case ir.VNot, ir.VNeg:
		g.source(fr, in, 1, l, 0)
		g.unaryRegs(l, op)
		g.vectorResult(fr, in, l, 0)
	case ir.VShl, ir.VShr:
		g.source(fr, in, 1, l, 0)
		g.move("rax", fr.loc(in.args[2]))
		if native(op, t, g.tier) {
			g.shiftRegs(l, op)
		} else {
			g.fallback(fr, l, op, false)
		}
		g.vectorResult(fr, in, l, 0)
	default:
		if g.tier == avx2 && g.direct(fr, in, l) {
			return
		}
		g.source(fr, in, 1, l, 0)
		g.source(fr, in, 2, l, 2)
		if native(op, t, g.tier) {
			g.binaryRegs(l, op)
		} else {
			g.fallback(fr, l, op, true)
		}
		g.vectorResult(fr, in, l, 0)
	}
}

func (g *Generator) splat(fr *frame, in *instr, l lanes) {
	g.move("rax", fr.loc(in.args[1]))
	bits := l.t.Elem.Bits
	if g.tier == avx2 {
		g.emit("vmovq xmm0, rax")
		g.emit("vpbroadcast%s %s, xmm0", intSuffix(l.t), l.reg(0, 0))
		return
	}
	g.emit("movq xmm0, rax")
	switch bits {
	case 8:
		g.emit("punpcklbw xmm0, xmm0")
		g.emit("punpcklwd xmm0, xmm0")
		g.emit("pshufd xmm0, xmm0, 0")
	case 16:
		g.emit("punpcklwd xmm0, xmm0")
		g.emit("pshufd xmm0, xmm0, 0")
	case 32:
		g.emit("pshufd xmm0, xmm0, 0")
	default:
		g.emit("punpcklqdq xmm0, xmm0")
	}
	if l.halves == 2 {
		g.emit("movdqa xmm1, xmm0")
	}
}

func (g *Generator) selectRegs(l lanes) {
	if g.tier == avx2 {
		g.emit("vpblendvb %s, %s, %s, %s", l.reg(4, 0), l.reg(4, 0), l.reg(2, 0), l.reg(0, 0))
		return
	}
	for h := range l.halves {
		g.emit("pand %s, %s", l.reg(2, h), l.reg(0, h))
		g.emit("pandn %s, %s", l.reg(0, h), l.reg(4, h))
		g.emit("por %s, %s", l.reg(0, h), l.reg(2, h))
		g.emit("movdqa %s, %s", l.reg(4, h), l.reg(0, h))
	}
}

func (g *Generator) unaryRegs(l lanes, op ir.VectorOp) {
	if op == ir.VNot {
		g.allOnes(l, 6)
		g.op2(l, "pxor", 0, 6)
		return
	}
	if l.t.Elem.Float() {
		g.bias(l, 0)
		return
	}
	g.op2(l, "pxor", 6, 6)
	g.op2(l, "psub"+intSuffix(l.t), 6, 0)
	g.copyRegs(l, 0, 6)
}

func (g *Generator) shiftRegs(l lanes, op ir.VectorOp) {
	mnemonic := "psll"
	if op == ir.VShr {
		mnemonic = "psrl"
		if l.t.Elem.Signed {
			mnemonic = "psra"
		}
	}
	mnemonic += intSuffix(l.t)
	if g.tier == avx2 {
		g.emit("vmovq xmm6, rax")
	} else {
		g.emit("movq xmm6, rax")
	}
	for h := range l.halves {
		if g.tier == avx2 {
			g.emit("v%s %s, %s, xmm6", mnemonic, l.reg(0, h), l.reg(0, h))
		} else {
			g.emit("%s %s, xmm6", mnemonic, l.reg(0, h))
		}
	}
}

var arithmeticNames = map[ir.VectorOp]string{
	ir.VAdd: "add", ir.VSub: "sub", ir.VMul: "mul", ir.VDiv: "div", ir.VMin: "min", ir.VMax: "max",
}

func arithmeticMnemonic(op ir.VectorOp, t *types.Type) string {
	name := arithmeticNames[op]
	s := intSuffix(t)
	switch {
	case t.Elem.Float():
		return name + floatSuffix(t)
	case op == ir.VMul:
		return "pmull" + s
	case op == ir.VMin || op == ir.VMax:
		sign := "u"
		if t.Elem.Signed {
			sign = "s"
		}
		return fmt.Sprintf("p%s%s%s", name, sign, s)
	}
	return "p" + name + s
}

func (g *Generator) binaryRegs(l lanes, op ir.VectorOp) {
	t := l.t
	float := t.Elem.Float()
	switch op {
	case ir.VAdd, ir.VSub, ir.VMul, ir.VDiv, ir.VMin, ir.VMax:
		g.op2(l, arithmeticMnemonic(op, t), 0, 2)
	case ir.VAnd:
		g.op2(l, "pand", 0, 2)
	case ir.VOr:
		g.op2(l, "por", 0, 2)
	case ir.VXor:
		g.op2(l, "pxor", 0, 2)
	case ir.VAndNot:
		g.op2(l, "pandn", 2, 0)
		g.copyRegs(l, 0, 2)
	case ir.VShuffle:
		g.op2(l, "pshufb", 0, 2)
	case ir.VEq, ir.VNe, ir.VLt, ir.VLe, ir.VGt, ir.VGe:
		if float {
			g.compareFloat(l, op)
		} else {
			g.compareInt(l, op)
		}
	}
}

func (g *Generator) compareFloat(l lanes, op ir.VectorOp) {
	if g.tier == avx2 {
		predicate := map[ir.VectorOp]int{ir.VEq: 0, ir.VNe: 4, ir.VLt: 1, ir.VLe: 2, ir.VGt: 14, ir.VGe: 13}[op]
		g.cmpFloat(l, 0, 2, predicate)
		return
	}
	switch op {
	case ir.VEq:
		g.cmpFloat(l, 0, 2, 0)
	case ir.VNe:
		g.cmpFloat(l, 0, 2, 4)
	case ir.VLt:
		g.cmpFloat(l, 0, 2, 1)
	case ir.VLe:
		g.cmpFloat(l, 0, 2, 2)
	case ir.VGt:
		g.cmpFloat(l, 2, 0, 1)
		g.copyRegs(l, 0, 2)
	case ir.VGe:
		g.cmpFloat(l, 2, 0, 2)
		g.copyRegs(l, 0, 2)
	}
}

func (g *Generator) compareInt(l lanes, op ir.VectorOp) {
	s := intSuffix(l.t)
	if op == ir.VEq || op == ir.VNe {
		g.op2(l, "pcmpeq"+s, 0, 2)
		if op == ir.VNe {
			g.allOnes(l, 6)
			g.op2(l, "pxor", 0, 6)
		}
		return
	}
	if !l.t.Elem.Signed {
		g.bias(l, 0)
		g.bias(l, 2)
	}
	switch op {
	case ir.VGt, ir.VLe:
		g.op2(l, "pcmpgt"+s, 0, 2)
	case ir.VLt, ir.VGe:
		g.op2(l, "pcmpgt"+s, 2, 0)
		g.copyRegs(l, 0, 2)
	}
	if op == ir.VLe || op == ir.VGe {
		g.allOnes(l, 6)
		g.op2(l, "pxor", 0, 6)
	}
}

func (g *Generator) vectorMask(fr *frame, in *instr, l lanes) {
	t := l.t
	src := g.maskSource(fr, in, l)
	switch {
	case t.Elem.Bits == 8:
		g.maskHalves(l, "pmovmskb", 16, src)
	case t.Elem.Bits == 16:
		g.maskWords(l, src)
	case t.Elem.Bits == 32:
		g.maskHalves(l, "movmskps", 4, src)
	default:
		g.maskHalves(l, "movmskpd", 2, src)
	}
	g.move(fr.loc(in.dst), "rax")
}

func (g *Generator) maskSource(fr *frame, in *instr, l lanes) int {
	if index, held := in.args[0].vectorRegister(); held {
		return index
	}
	g.source(fr, in, 0, l, 0)
	return 0
}

func (g *Generator) maskHalves(l lanes, mnemonic string, perHalf, src int) {
	if g.tier == avx2 {
		g.emit("v%s eax, %s", mnemonic, l.reg(src, 0))
		return
	}
	g.emit("%s eax, xmm0", mnemonic)
	if l.halves == 2 {
		g.emit("%s edx, xmm1", mnemonic)
		g.emit("shl edx, %d", perHalf)
		g.emit("or eax, edx")
	}
}

func (g *Generator) maskWords(l lanes, src int) {
	switch {
	case g.tier == avx2 && l.wide:
		g.emit("vextracti128 xmm1, ymm%d, 1", src)
		g.emit("vpacksswb xmm0, xmm%d, xmm1", src)
		g.emit("vpmovmskb eax, xmm0")
	case g.tier == avx2:
		g.emit("vpacksswb xmm0, xmm%d, xmm%[1]d", src)
		g.emit("vpmovmskb eax, xmm0")
		g.emit("and eax, 255")
	case l.halves == 2:
		g.emit("packsswb xmm0, xmm1")
		g.emit("pmovmskb eax, xmm0")
	default:
		g.emit("packsswb xmm0, xmm0")
		g.emit("pmovmskb eax, xmm0")
		g.emit("and eax, 255")
	}
}

func laneLoad(t *types.Type, dst string, mem string) string {
	width, narrowed := widths[t.Elem.Bits]
	if !narrowed {
		return fmt.Sprintf("mov %s, qword ptr %s", dst, mem)
	}
	return extension(dst, fmt.Sprintf("%s ptr %s", width, mem), t.Elem)
}

func laneStore(t *types.Type, mem string, src string) string {
	width, narrowed := widths[t.Elem.Bits]
	if !narrowed {
		return fmt.Sprintf("mov qword ptr %s, %s", mem, src)
	}
	return fmt.Sprintf("mov %s ptr %s, %s", width, mem, narrow[src][subRegister[t.Elem.Bits]])
}

func (l lanes) laneAt(m maddr) string {
	return fmt.Sprintf("[%s%+d+r11*%d]", m.base, m.disp, l.t.LaneBytes())
}

func (g *Generator) fallback(fr *frame, l lanes, op ir.VectorOp, binary bool) {
	a, b, out := fr.scratch(0), fr.scratch(1), fr.scratch(2)
	g.storeVector(l, 0, a)
	if binary {
		g.storeVector(l, 2, b)
	}
	saved := fr.scratch(vectorScratchSlots)
	if op == ir.VShl || op == ir.VShr {
		g.emit("mov qword ptr %s, rcx", saved.at(0))
		g.emit("mov rcx, rax")
	}
	loop := g.newLabel()
	g.emit("xor r11d, r11d")
	g.label(loop)
	g.emit("%s", laneLoad(l.t, "rax", l.laneAt(a)))
	if binary {
		g.emit("%s", laneLoad(l.t, "rdx", l.laneAt(b)))
	}
	g.scalarLane(l, op, a)
	g.emit("%s", laneStore(l.t, l.laneAt(out), "rax"))
	g.emit("inc r11")
	g.emit("cmp r11, %d", l.t.Lanes)
	g.emit("jb %s", loop)
	if op == ir.VShl || op == ir.VShr {
		g.emit("mov rcx, qword ptr %s", saved.at(0))
	}
	g.loadVector(l, 0, out)
}

var vectorComparisons = map[ir.VectorOp]ir.Op{
	ir.VEq: ir.OpEq, ir.VNe: ir.OpNe, ir.VLt: ir.OpLt, ir.VLe: ir.OpLe, ir.VGt: ir.OpGt, ir.VGe: ir.OpGe,
}

func (g *Generator) scalarLane(l lanes, op ir.VectorOp, table maddr) {
	signed := l.t.Elem.Signed
	switch op {
	case ir.VMul:
		g.emit("imul rax, rdx")
	case ir.VMin, ir.VMax:
		g.emit("cmp rax, rdx")
		pick := ir.OpGt
		if op == ir.VMax {
			pick = ir.OpLt
		}
		g.emit("cmov%s rax, rdx", conditionOf(pick, signed))
	case ir.VEq, ir.VNe, ir.VLt, ir.VLe, ir.VGt, ir.VGe:
		g.emit("cmp rax, rdx")
		g.emit("set%s al", conditionOf(vectorComparisons[op], signed))
		g.emit("movzx eax, al")
		g.emit("neg rax")
	case ir.VShuffle:
		zero := g.newLabel()
		done := g.newLabel()
		g.emit("test dl, -128")
		g.emit("jnz %s", zero)
		g.emit("and edx, 15")
		g.emit("mov r10, r11")
		g.emit("and r10, 16")
		g.emit("add rdx, r10")
		g.emit("movzx eax, byte ptr [%s%+d+rdx]", table.base, table.disp)
		g.emit("jmp %s", done)
		g.label(zero)
		g.emit("xor eax, eax")
		g.label(done)
	case ir.VShl:
		g.emit("shl rax, cl")
	case ir.VShr:
		if signed {
			g.emit("sar rax, cl")
		} else {
			g.emit("shr rax, cl")
		}
	}
}

func (g *Generator) leaveVectors() {
	if g.tier == avx2 {
		g.emit("vzeroupper")
	}
}

func (r *vreg) vectorRegister() (int, bool) {
	if r.home == nil || r.reg == "" {
		return 0, false
	}
	index, _ := strconv.Atoi(strings.TrimPrefix(r.reg, "ymm"))
	return index, true
}

func (r *vreg) lanes() lanes {
	return lanes{halves: 1, wide: r.wide}
}

func (fr *frame) homeAddress(r *vreg) maddr {
	return maddr{"rbp", fr.slot(int(r.home.Aux))}
}

func (g *Generator) source(fr *frame, in *instr, k int, l lanes, base int) {
	if index, held := in.args[k].vectorRegister(); held {
		g.copyRegs(l, base, index)
		return
	}
	g.loadVector(l, base, g.vectorAddress(fr, in, k))
}

func (g *Generator) vectorResult(fr *frame, in *instr, l lanes, base int) {
	if index, held := in.args[0].vectorRegister(); held {
		g.copyRegs(l, index, base)
		return
	}
	g.storeVector(l, base, g.vectorAddress(fr, in, 0))
}

func (g *Generator) copyVector(fr *frame, in *instr, l lanes) {
	dst, dstHeld := in.args[0].vectorRegister()
	src, srcHeld := in.args[1].vectorRegister()
	switch {
	case dstHeld && srcHeld:
		if dst != src {
			g.copyRegs(l, dst, src)
		}
	case dstHeld:
		g.loadVector(l, dst, g.vectorAddress(fr, in, 1))
	case srcHeld:
		g.storeVector(l, src, g.vectorAddress(fr, in, 0))
	default:
		g.source(fr, in, 1, l, 0)
		g.vectorResult(fr, in, l, 0)
	}
}

func directMnemonic(op ir.VectorOp, t *types.Type) (string, bool) {
	if !native(op, t, avx2) {
		return "", false
	}
	switch op {
	case ir.VAdd, ir.VSub, ir.VMul, ir.VMin, ir.VMax:
		return arithmeticMnemonic(op, t), true
	case ir.VDiv:
		return arithmeticMnemonic(op, t), t.Elem.Float()
	case ir.VAnd:
		return "pand", true
	case ir.VOr:
		return "por", true
	case ir.VXor:
		return "pxor", true
	case ir.VShuffle:
		return "pshufb", true
	case ir.VEq:
		return "pcmpeq" + intSuffix(t), !t.Elem.Float()
	}
	return "", false
}

func (g *Generator) direct(fr *frame, in *instr, l lanes) bool {
	mnemonic, ok := directMnemonic(ir.VectorOp(in.value.Aux), l.t)
	if !ok {
		return false
	}
	first := g.registerOperand(fr, in, 1, l)
	second := g.memoryOperand(fr, in, 2, l)
	dst, held := in.args[0].vectorRegister()
	g.emit("v%s %s, %s, %s", mnemonic, l.reg(dst, 0), first, second)
	if !held {
		g.storeVector(l, dst, g.vectorAddress(fr, in, 0))
	}
	return true
}

func (g *Generator) registerOperand(fr *frame, in *instr, k int, l lanes) string {
	index, held := in.args[k].vectorRegister()
	if !held {
		g.loadVector(l, index, g.vectorAddress(fr, in, k))
	}
	return l.reg(index, 0)
}

func (g *Generator) memoryOperand(fr *frame, in *instr, k int, l lanes) string {
	if index, held := in.args[k].vectorRegister(); held {
		return l.reg(index, 0)
	}
	return fmt.Sprintf("%s ptr %s", g.memWidth(l), g.vectorAddress(fr, in, k).at(0))
}

func (fr *frame) heldVectors(mb *mblock) []*vreg {
	var held []*vreg
	for _, r := range fr.fn.vregs {
		if r.home != nil && r.reg != "" && mb.liveOut[r] {
			held = append(held, r)
		}
	}
	return held
}

func (g *Generator) saveVectors(fr *frame, held []*vreg) {
	for _, r := range held {
		g.spillRegister(fr, r)
	}
}

func (g *Generator) restoreVectors(fr *frame, held []*vreg) {
	for _, r := range held {
		g.fillRegister(fr, r)
	}
}

func (g *Generator) spillVector(fr *frame, in *instr) {
	g.spillRegister(fr, in.args[0])
}

func (g *Generator) fillVector(fr *frame, in *instr) {
	g.fillRegister(fr, in.dst)
}

func (g *Generator) spillRegister(fr *frame, r *vreg) {
	if index, held := r.vectorRegister(); held {
		g.storeVector(r.lanes(), index, fr.homeAddress(r))
	}
}

func (g *Generator) fillRegister(fr *frame, r *vreg) {
	if index, held := r.vectorRegister(); held {
		g.loadVector(r.lanes(), index, fr.homeAddress(r))
	}
}
