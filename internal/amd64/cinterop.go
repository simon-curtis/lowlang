package amd64

import (
	"fmt"

	"lowlang/internal/ir"
	"lowlang/internal/target"
	"lowlang/internal/types"
)

func (g *Generator) cCall(fr *frame, in *instr) {
	plan := in.value.Plan
	args := in.args
	var moves []pmove
	callee := g.cTarget(plan.Symbol)
	if plan.Symbol == "" {
		moves = append(moves, pmove{dst: "r10", src: fr.loc(args[0])})
		args = args[1:]
		callee = "r10"
	}
	if g.os == target.Windows {
		moves = g.win64Args(fr, plan, args, moves)
	} else {
		moves = g.sysvArgs(fr, plan, args, moves)
	}
	g.parallel(moves)
	if plan.Memory {
		g.emit("lea %s, [rsp+%d]", g.cIntRegs()[0], 8*plan.StackWords)
	}
	label := g.newLabel()
	g.enterC(label)
	if plan.StackWords > 0 {
		g.emit("sub rsp, %d", 16*((plan.StackWords+1)/2))
		for i := g.cRegisterSlots(); i < plan.StackWords; i++ {
			g.emit("mov r11, [r12+%d]", 8*(len(calleeSaved)+i))
			g.emit("mov [rsp+%d], r11", 8*i)
		}
	}
	g.leaveVectors()
	if g.os != target.Windows {
		g.emit("mov eax, %d", plan.Vectors)
	}
	g.emit("call %s", callee)
	g.safepointAt(fr, in, label)
	g.leaveC()
}

func (g *Generator) sysvArgs(fr *frame, plan *ir.CPlan, args []*vreg, moves []pmove) []pmove {
	ints, vectors, stack := 0, 0, 0
	if plan.Memory {
		ints = 1
	}
	for i, a := range args {
		src := fr.loc(a)
		switch plan.Args[i] {
		case ir.Vector:
			moves = append(moves, fr.vectorMove(fmt.Sprintf("xmm%d", vectors), a))
			vectors++
		case ir.Stack:
			g.stackArg(stack, src)
			stack++
		default:
			moves = append(moves, pmove{dst: argRegs[ints], src: src})
			ints++
		}
	}
	return moves
}

func (g *Generator) stackArg(slot int, src string) {
	g.move(fmt.Sprintf("qword ptr [rsp+%d]", 8*slot), src)
}

func (g *Generator) leaveC() {
	g.emit("TEB_ENTER_CURRENT_G rbx")
	g.emit("mov rsp, r12")
	g.emit("call rt_exitc")
	for i := len(calleeSaved) - 1; i >= 0; i-- {
		g.emit("pop %s", calleeSaved[i])
	}
}

func (g *Generator) enterC(label string) {
	for _, r := range calleeSaved {
		g.emit("push %s", r)
	}
	g.emit("TLS_GET rax, TLS_G")
	g.emit("mov [rax+G_SCANLO], rsp")
	g.emit("lea r11, [rsp+%d]", 8*len(calleeSaved))
	g.emit("mov [rax+G_USERSP], r11")
	g.emit("mov [rax+G_FP], rbp")
	g.emit("lea r11, [rip+%s]", label)
	g.emit("mov [rax+G_PC], r11")
	g.emit("TLS_GET rbx, TLS_M")
	g.emit("mov r11d, INC")
	g.emit("xchg [rbx], r11")
	g.emit("mov r12, rsp")
	g.emit("TEB_ENTER_OS rax, rbx")
	g.emit("mov rsp, [rbx+M_G0SP]")
}

func (g *Generator) cTarget(symbol string) string {
	switch {
	case g.os == target.Windows:
		return fmt.Sprintf("qword ptr [rip+%s]", importSymbol(symbol))
	case g.table != nil:
		return fmt.Sprintf("cfn_%s", symbol)
	}
	return symbol
}

func (g *Generator) cSymbolAddress(symbol string) string {
	if g.os == target.Windows {
		return fmt.Sprintf("qword ptr [rip+%s]", importSymbol(symbol))
	}
	return fmt.Sprintf("qword ptr [rip+%s@GOTPCREL]", symbol)
}

func (g *Generator) trampolines(symbols []string) {
	for _, s := range symbols {
		fmt.Fprintf(&g.text, "\t.globl cfn_%s\ncfn_%s:\n\tjmp %s\n", s, s, s)
	}
}

func (g *Generator) thunk(f *ir.Func) {
	g.fn = fmt.Sprintf("cb_%s", mangle(f.Name))
	g.labels = 0
	g.align(functionAlignment)
	fmt.Fprintf(&g.text, "\t.globl %s\n%s:\n", g.fn, g.fn)
	saved := g.thunkSaved()
	g.emit("push rbp")
	for _, r := range saved {
		g.emit("push %s", r)
	}
	ints, vectors := g.cIntRegs(), g.cVectorRegs()
	registerBytes := (8*(len(ints)+vectors) + 15) &^ 15
	g.emit("mov r13, rsp")
	g.emit("sub rsp, %d", registerBytes+16*len(g.cPreservedVectors()))
	g.emit("and rsp, -16")
	g.emit("mov r12, rsp")
	for i, r := range ints {
		g.emit("mov [r12+%d], %s", 8*i, r)
	}
	for i := range vectors {
		g.emit("movq qword ptr [r12+%d], xmm%d", 8*(len(ints)+i), i)
	}
	for i, r := range g.cPreservedVectors() {
		g.emit("movaps xmmword ptr [r12+%d], %s", registerBytes+16*i, r)
	}
	g.emit("TLS_GET rbx, TLS_M")
	g.emit("mov edi, INC")
	g.emit("call rt_m_running")
	g.emit("mov r14, [rbx+M_G0SP]")
	g.emit("mov [rbx+M_G0SP], r12")
	g.emit("TLS_GET r15, TLS_G")
	g.emit("inc qword ptr [r15+G_LOCKEDM]")
	g.emit("TEB_ENTER_G r15, rbx")
	g.emit("mov rsp, [r15+G_SCANLO]")
	g.emit("and rsp, -16")
	for _, field := range []string{"G_SCANLO", "G_USERSP", "G_PC", "G_FP", "G_PC", "G_FP"} {
		g.emit("push qword ptr [r15+%s]", field)
	}
	g.emit("mov rbp, rsp")
	g.emit("push r12")
	g.emit("push r14")
	g.emit("push qword ptr [r15+G_USERSP]")
	g.emit("sub rsp, 8")
	if stack := stackWords(f.ParamWords); stack > 0 {
		g.emit("sub rsp, %d", 16*((stack+1)/2))
	}
	g.thunkArgs(f, 8*(len(saved)+2))
	label := g.newLabel()
	g.emit("call %s", g.callTarget(f))
	g.label(label)
	g.maps = append(g.maps, stackMap{label: label, segment: true})
	if f.Ret.Float() {
		g.emit("movq xmm0, rax")
	}
	for i, field := range []string{"G_FP", "G_PC", "G_USERSP", "G_SCANLO"} {
		g.emit("mov rcx, [rbp+%d]", 16+8*i)
		g.emit("mov [r15+%s], rcx", field)
	}
	g.emit("dec qword ptr [r15+G_LOCKEDM]")
	g.emit("TEB_ENTER_OS r15, rbx")
	g.emit("mov rsp, r12")
	for i, r := range g.cPreservedVectors() {
		g.emit("movaps %s, xmmword ptr [r12+%d]", r, registerBytes+16*i)
	}
	g.emit("mov [rbx+M_G0SP], r14")
	g.emit("mov ecx, INC")
	g.emit("xchg [rbx], rcx")
	g.emit("mov rsp, r13")
	for i := len(saved) - 1; i >= 0; i-- {
		g.emit("pop %s", saved[i])
	}
	g.emit("pop rbp")
	g.emit("ret")
}

func (g *Generator) thunkArgs(f *ir.Func, incoming int) {
	ints, vectors, spilled := 0, 0, 0
	intRegs, vectorRegs := g.cIntRegs(), g.cVectorRegs()
	for k, t := range f.CParams {
		var src string
		if g.os == target.Windows {
			ints, vectors = k, k
		}
		switch {
		case t.Float() && vectors < vectorRegs:
			src = fmt.Sprintf("[r12+%d]", 8*(len(intRegs)+vectors))
			vectors++
		case !t.Float() && ints < len(intRegs):
			src = fmt.Sprintf("[r12+%d]", 8*ints)
			ints++
		default:
			src = fmt.Sprintf("[r13+%d]", incoming+8*(g.cRegisterSlots()+spilled))
			spilled++
		}
		g.emit("%s", widen(t, src))
		if k < len(argRegs) {
			g.emit("mov %s, r10", argRegs[k])
			continue
		}
		g.emit("mov qword ptr [rsp+%d], r10", 8*(k-len(argRegs)))
	}
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
		g.move(dst, resultRegisters[ints])
		return
	}
	source := fmt.Sprintf("xmm%d", vectors)
	if plan.Ret == types.F32 {
		g.moveSingle(dst, source)
		return
	}
	g.move(dst, source)
}

func (g *Generator) moveSingle(dst, source string) {
	switch {
	case isXmm(dst):
		g.emit("xorps %s, %s", dst, dst)
		g.emit("movss %s, %s", dst, source)
	case isReg(dst):
		g.emit("movd %s, %s", narrow[dst][0], source)
	default:
		g.emit("movd r10d, %s", source)
		g.move(dst, "r10")
	}
}

func (g *Generator) libcShims(symbols []string) {
	for _, symbol := range symbols {
		switch symbol {
		case "atexit":
			g.shim(symbol, "__cxa_atexit", "xor esi, esi", "xor edx, edx")
		case "at_quick_exit":
			g.shim(symbol, "__cxa_at_quick_exit", "xor esi, esi")
		}
	}
}

func (g *Generator) shim(symbol, target string, setup ...string) {
	g.align(functionAlignment)
	fmt.Fprintf(&g.text, "\t.globl %s\n%s:\n", symbol, symbol)
	for _, line := range setup {
		g.emit("%s", line)
	}
	g.emit("jmp %s", target)
}
