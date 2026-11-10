package amd64

import (
	"fmt"

	"lowlang/internal/ir"
)

func (g *Generator) cCall(fr *frame, in *instr) {
	plan := in.value.Plan
	ints, vectors, stack := 0, 0, 0
	if plan.Memory {
		ints = 1
	}
	var moves []pmove
	for i, a := range in.args {
		src := fr.loc(a)
		switch plan.Args[i] {
		case ir.Vector:
			g.toXmm(fmt.Sprintf("xmm%d", vectors), src)
			vectors++
		case ir.Stack:
			if isMem(src) {
				g.move("r10", src)
				src = "r10"
			}
			g.emit("mov qword ptr [rsp+%d], %s", 8*stack, src)
			stack++
		default:
			moves = append(moves, pmove{dst: argRegs[ints], src: src})
			ints++
		}
	}
	g.parallel(moves)
	if plan.Memory {
		g.emit("lea rdi, [rsp+%d]", 8*plan.StackWords)
	}
	g.emit("mov eax, %d", plan.Vectors)
	g.emit("call %s", g.cTarget(plan.Symbol))
	g.safepoint(fr, in)
}

func (g *Generator) cTarget(symbol string) string {
	if g.table == nil {
		return symbol
	}
	return fmt.Sprintf("cfn_%s", symbol)
}

func (g *Generator) trampolines(symbols []string) {
	for _, s := range symbols {
		fmt.Fprintf(&g.text, "\t.globl cfn_%s\ncfn_%s:\n\tjmp %s\n", s, s, s)
	}
}

func (g *Generator) cResult(fr *frame, in *instr, plan *ir.CPlan, k int) {
	dst := fr.loc(in.dst)
	if plan.Memory {
		g.emit("mov r10, qword ptr [rsp+%d]", 8*(plan.StackWords+k))
		g.move(dst, "r10")
		return
	}
	ints, vectors := 0, 0
	for _, c := range plan.Results[:k] {
		if c == ir.Vector {
			vectors++
		} else {
			ints++
		}
	}
	if plan.Results[k] == ir.Integer {
		g.move(dst, []string{"rax", "rdx"}[ints])
		return
	}
	reg := dst
	if !isReg(dst) {
		reg = "r10"
	}
	g.emit("movq %s, xmm%d", reg, vectors)
	g.move(dst, reg)
}
