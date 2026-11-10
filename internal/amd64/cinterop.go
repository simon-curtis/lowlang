package amd64

import (
	"fmt"

	"lowlang/internal/ir"
	"lowlang/internal/types"
)

func (g *Generator) cCall(fr *frame, in *instr) {
	plan := in.value.Plan
	label := g.newLabel()
	g.emit("lea r10, [rip+%s]", label)
	g.emit("mov [rip+rt_c_pc], r10")
	g.emit("mov [rip+rt_c_fp], rbp")
	g.emit("mov [rip+rt_c_sp], rsp")
	ints, vectors, stack := 0, 0, 0
	if plan.Memory {
		ints = 1
	}
	args := in.args
	var moves []pmove
	target := g.cTarget(plan.Symbol)
	if plan.Symbol == "" {
		moves = append(moves, pmove{dst: "r10", src: fr.loc(args[0])})
		args = args[1:]
		target = "r10"
	}
	for i, a := range args {
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
	g.emit("call %s", target)
	g.safepointAt(fr, in, label)
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

const (
	savedSP   = 8
	savedInts = 16
	savedXmms = 64
	saveWords = 15
)

func (g *Generator) thunk(f *ir.Func) {
	g.fn = fmt.Sprintf("cb_%s", mangle(f.Name))
	g.labels = 0
	fmt.Fprintf(&g.text, "\t.globl %s\n%s:\n", g.fn, g.fn)
	g.emit("push rbp")
	g.emit("push qword ptr [rip+rt_c_pc]")
	g.emit("push qword ptr [rip+rt_c_fp]")
	g.emit("mov rbp, rsp")
	g.emit("sub rsp, %d", (8*(saveWords+stackWords(f.ParamWords))+15)&^15)
	g.emit("mov r10, [rip+rt_c_sp]")
	g.emit("mov [rbp-%d], r10", savedSP)
	for i, r := range argRegs {
		g.emit("mov [rbp-%d], %s", savedInts+8*i, r)
	}
	for i := range 8 {
		g.emit("movq qword ptr [rbp-%d], xmm%d", savedXmms+8*i, i)
	}
	ints, vectors, spilled := 0, 0, 0
	for k, t := range f.CParams {
		var src string
		switch {
		case t.Float() && vectors < 8:
			src = fmt.Sprintf("[rbp-%d]", savedXmms+8*vectors)
			vectors++
		case !t.Float() && ints < len(argRegs):
			src = fmt.Sprintf("[rbp-%d]", savedInts+8*ints)
			ints++
		default:
			src = fmt.Sprintf("[rbp+%d]", 32+8*spilled)
			spilled++
		}
		g.emit("%s", widen(t, src))
		if k < len(argRegs) {
			g.emit("mov %s, r10", argRegs[k])
			continue
		}
		g.emit("mov qword ptr [rsp+%d], r10", 8*(k-len(argRegs)))
	}
	label := g.newLabel()
	g.emit("call %s", g.callTarget(f))
	g.label(label)
	g.maps = append(g.maps, stackMap{label: label, segment: true})
	if f.Ret.Float() {
		g.emit("movq xmm0, rax")
	}
	for _, restore := range []struct {
		global string
		offset int
	}{{"rt_c_fp", 0}, {"rt_c_pc", 8}, {"rt_c_sp", -savedSP}} {
		g.emit("mov r10, [rbp%+d]", restore.offset)
		g.emit("mov [rip+%s], r10", restore.global)
	}
	g.emit("lea rsp, [rbp+16]")
	g.emit("pop rbp")
	g.emit("ret")
}

func widen(t *types.Type, src string) string {
	width := t.Width()
	switch {
	case width == nil:
		return fmt.Sprintf("mov r10, qword ptr %s", src)
	case width.Float() || (width.Bits == 32 && !width.Signed):
		return fmt.Sprintf("mov r10d, dword ptr %s", src)
	case width.Bits == 32:
		return fmt.Sprintf("movsxd r10, dword ptr %s", src)
	}
	extend := "movzx"
	if width.Signed {
		extend = "movsx"
	}
	return fmt.Sprintf("%s r10, %s ptr %s", extend, map[int]string{8: "byte", 16: "word"}[width.Bits], src)
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
