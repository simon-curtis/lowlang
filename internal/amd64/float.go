package amd64

import (
	"fmt"
	"strings"

	"lowlang/internal/ir"
	"lowlang/internal/types"
)

func xmmTarget(dst string) string {
	if isXmm(dst) {
		return dst
	}
	return "xmm0"
}

func narrowMemory(loc, p string) string {
	if p == "s" {
		return strings.Replace(loc, "qword", "dword", 1)
	}
	return loc
}

func (g *Generator) moveValue(fr *frame, dst string, r *vreg) {
	if r.konst != nil && isXmm(dst) {
		g.loadFloatBits(dst, *r.konst)
		return
	}
	g.move(dst, fr.loc(r))
}

func (g *Generator) xmmRegister(fr *frame, r *vreg, scratch string) string {
	loc := fr.loc(r)
	if isXmm(loc) {
		return loc
	}
	g.moveValue(fr, scratch, r)
	return scratch
}

func (g *Generator) xmmOperand(fr *frame, r *vreg, scratch, p string) string {
	loc := fr.loc(r)
	switch {
	case isXmm(loc):
		return loc
	case isMem(loc):
		return narrowMemory(loc, p)
	case r.konst != nil && *r.konst != 0:
		return narrowMemory(fmt.Sprintf("qword ptr [rip+%s]", g.floatLabel(*r.konst)), p)
	}
	g.moveValue(fr, scratch, r)
	return scratch
}

func (g *Generator) floatArithmetic(fr *frame, in *instr, op string) {
	p := precision(in.value.Type)
	dst := fr.loc(in.dst)
	target := xmmTarget(dst)
	first, second := in.args[0], in.args[1]
	var operand string
	switch {
	case first == second:
		g.moveValue(fr, target, first)
		operand = target
	case fr.loc(second) != target || fr.loc(first) == target:
		g.moveValue(fr, target, first)
		operand = g.xmmOperand(fr, second, "xmm1", p)
	case op == "add" || op == "mul":
		g.moveValue(fr, target, second)
		operand = g.xmmOperand(fr, first, "xmm1", p)
	default:
		g.emit("movaps xmm1, %s", target)
		g.moveValue(fr, target, first)
		operand = "xmm1"
	}
	g.emit("%ss%s %s, %s", op, p, target, operand)
	g.move(dst, target)
}

func (g *Generator) floatNegate(fr *frame, in *instr) {
	dst := fr.loc(in.dst)
	target := xmmTarget(dst)
	g.moveValue(fr, target, in.args[0])
	g.emit("xorps %s, xmmword ptr [rip+%s]", target, g.signMask(in.value.Type.Bits))
	g.move(dst, target)
}

func (g *Generator) floatUnary(fr *frame, in *instr) {
	p := precision(in.value.Type)
	dst := fr.loc(in.dst)
	target := xmmTarget(dst)
	operand := g.xmmOperand(fr, in.args[0], "xmm0", p)
	if operand != target {
		g.emit("xorps %s, %s", target, target)
	}
	if in.op == ir.OpSqrt {
		g.emit("sqrts%s %s, %s", p, target, operand)
	} else {
		g.emit("rounds%s %s, %s, 1", p, target, operand)
	}
	g.move(dst, target)
}

func orderedOperands(op ir.Op, a, b *vreg) (*vreg, *vreg) {
	if op == ir.OpLt || op == ir.OpLe {
		return b, a
	}
	return a, b
}

func (g *Generator) ucomis(fr *frame, t *types.Type, first, second *vreg) {
	p := precision(t)
	register := g.xmmRegister(fr, first, "xmm0")
	operand := g.xmmOperand(fr, second, "xmm1", p)
	g.emit("ucomis%s %s, %s", p, register, operand)
}

func (g *Generator) floatCompare(fr *frame, in *instr) {
	first, second := orderedOperands(in.op, in.args[0], in.args[1])
	g.ucomis(fr, in.value.Type, first, second)
	op := in.op
	if in.negated && (op == ir.OpEq || op == ir.OpNe) {
		op = map[ir.Op]ir.Op{ir.OpEq: ir.OpNe, ir.OpNe: ir.OpEq}[op]
	}
	switch {
	case op == ir.OpEq:
		g.emit("sete al")
		g.emit("setnp r11b")
		g.emit("and al, r11b")
	case op == ir.OpNe:
		g.emit("setne al")
		g.emit("setp r11b")
		g.emit("or al, r11b")
	case in.negated && (op == ir.OpLt || op == ir.OpGt):
		g.emit("setbe al")
	case in.negated:
		g.emit("setb al")
	case op == ir.OpLt || op == ir.OpGt:
		g.emit("seta al")
	default:
		g.emit("setae al")
	}
}

func (g *Generator) floatBranch(fr *frame, mb *mblock, then, otherwise, next *mblock) {
	op := mb.fused.Op
	first, second := orderedOperands(op, mb.compare[0], mb.compare[1])
	g.ucomis(fr, mb.fused.Type, first, second)
	switch op {
	case ir.OpEq:
		g.emit("jne %s", otherwise.label)
		g.emit("jp %s", otherwise.label)
		if then != next {
			g.emit("jmp %s", then.label)
		}
	case ir.OpNe:
		g.emit("jne %s", then.label)
		g.emit("jp %s", then.label)
		if otherwise != next {
			g.emit("jmp %s", otherwise.label)
		}
	case ir.OpLt, ir.OpGt:
		g.jump("a", then, otherwise, next)
	default:
		g.jump("ae", then, otherwise, next)
	}
}

func (g *Generator) integerToFloat(src, dst string, to *types.Type) {
	p := precision(to)
	target := xmmTarget(dst)
	operand := src
	if !isReg(src) {
		g.move("r10", src)
		operand = "r10"
	}
	g.emit("xorps %s, %s", target, target)
	g.emit("cvtsi2s%s %s, %s", p, target, operand)
	g.move(dst, target)
}

func (g *Generator) unsignedToFloat(src, dst string, to *types.Type) {
	p := precision(to)
	target := xmmTarget(dst)
	big := g.newLabel()
	done := g.newLabel()
	g.move("r10", src)
	g.emit("xorps %s, %s", target, target)
	g.emit("test r10, r10")
	g.emit("js %s", big)
	g.emit("cvtsi2s%s %s, r10", p, target)
	g.emit("jmp %s", done)
	g.label(big)
	g.emit("mov r11, r10")
	g.emit("shr r10, 1")
	g.emit("and r11d, 1")
	g.emit("or r10, r11")
	g.emit("cvtsi2s%s %s, r10", p, target)
	g.emit("adds%s %s, %s", p, target, target)
	g.label(done)
	g.move(dst, target)
}

func (g *Generator) floatToInteger(fr *frame, src *vreg, dst string, from, to *types.Type) {
	p := precision(from)
	operand := g.xmmOperand(fr, src, "xmm0", p)
	g.emit("cvtts%s2si r10, %s", p, operand)
	g.normalize("r10", to)
	g.move(dst, "r10")
}

func (g *Generator) floatToUnsigned(fr *frame, src *vreg, dst string, from *types.Type) {
	p := precision(from)
	big := g.newLabel()
	done := g.newLabel()
	limit := int64(0x43E0000000000000)
	if p == "s" {
		limit = 0x5F000000
	}
	g.moveValue(fr, "xmm0", src)
	g.loadFloatBits("xmm1", limit)
	g.emit("ucomis%s xmm0, xmm1", p)
	g.emit("jae %s", big)
	g.emit("cvtts%s2si r10, xmm0", p)
	g.emit("jmp %s", done)
	g.label(big)
	g.emit("subs%s xmm0, xmm1", p)
	g.emit("cvtts%s2si r10, xmm0", p)
	g.emit("btc r10, 63")
	g.label(done)
	g.move(dst, "r10")
}

func (g *Generator) floatToFloat(fr *frame, src *vreg, dst string, from, to *types.Type) {
	operand := g.xmmOperand(fr, src, "xmm0", precision(from))
	target := xmmTarget(dst)
	if to.Bits == 32 {
		if target == operand {
			target = "xmm1"
		}
		g.emit("xorps %s, %s", target, target)
		g.emit("cvtsd2ss %s, %s", target, operand)
		g.move(dst, target)
		return
	}
	g.emit("cvtss2sd %s, %s", target, operand)
	g.move(dst, target)
}

func (g *Generator) floatLoad(dst string, t *types.Type, memory string) {
	if t != nil && t.Bits == 32 {
		g.emit("movss %s, dword ptr %s", dst, memory)
		return
	}
	g.emit("movsd %s, qword ptr %s", dst, memory)
}

func (g *Generator) floatStore(memory string, in *instr, value string) bool {
	t := in.value.Type
	switch {
	case t == nil:
		g.emit("movsd qword ptr %s, %s", memory, value)
	case t.Float() && t.Bits == 32:
		g.emit("movss dword ptr %s, %s", memory, value)
	default:
		return false
	}
	return true
}
