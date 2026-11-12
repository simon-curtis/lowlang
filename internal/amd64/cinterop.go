package amd64

import (
	"fmt"

	"lowlang/internal/ir"
	"lowlang/internal/types"
)

type addressMove struct {
	reg    string
	offset int
}

func (g *Generator) cCall(fr *frame, in *instr) {
	plan := g.plan(in.value.Sig)
	args := in.args
	var moves []pmove
	callee := g.externalTarget(plan.symbol)
	if plan.symbol == "" {
		moves = append(moves, pmove{dst: "r10", src: fr.loc(args[0])})
		args = args[1:]
		callee = "r10"
	}
	moves, addresses := g.cArgs(fr, plan, args, moves)
	g.parallel(moves)
	for _, a := range addresses {
		g.emit("lea %s, [rsp+%d]", a.reg, a.offset)
	}
	if plan.memory {
		g.emit("lea %s, [rsp+%d]", g.platform.intRegs()[0], 8*plan.stackWords)
	}
	label := g.newLabel()
	g.enterC(label)
	if plan.stackWords > 0 {
		g.emit("sub rsp, %d", abi.alignedBytes(plan.stackWords))
		for i := g.platform.registerSlots(); i < plan.stackWords; i++ {
			g.emit("mov r11, [r12+%d]", abi.savedBytes()+8*i)
			g.emit("mov [rsp+%d], r11", 8*i)
		}
	}
	g.leaveVectors()
	g.platform.variadicCount(g, plan)
	g.emit("call %s", callee)
	g.safepointAt(fr, in, label)
	g.leaveC()
}

func (g *Generator) cArgs(fr *frame, plan *cPlan, args []*vreg, moves []pmove) ([]pmove, []addressMove) {
	var addresses []addressMove
	cursor := 0
	for _, arg := range plan.args {
		words := args[cursor : cursor+arg.words]
		cursor += arg.words
		switch {
		case arg.class == byRef:
			offset := 8 * (plan.copyBase() + arg.copy)
			for k, w := range words {
				g.move(fmt.Sprintf("qword ptr [rsp+%d]", offset+8*k), fr.loc(w))
			}
			addresses = g.passAddress(addresses, arg, offset)
		case arg.slot >= 0:
			g.stackArg(arg.slot, fr.loc(words[0]))
		default:
			if arg.xmm != "" {
				moves = append(moves, fr.vectorMove(arg.xmm, words[0]))
			}
			if arg.reg != "" {
				moves = append(moves, pmove{dst: arg.reg, src: fr.loc(words[0])})
			}
		}
	}
	return moves, addresses
}

func (g *Generator) passAddress(addresses []addressMove, arg cArg, offset int) []addressMove {
	if arg.reg != "" {
		return append(addresses, addressMove{reg: arg.reg, offset: offset})
	}
	g.emit("lea r11, [rsp+%d]", offset)
	g.stackArg(arg.slot, "r11")
	return addresses
}

func (g *Generator) stackArg(slot int, src string) {
	g.move(fmt.Sprintf("qword ptr [rsp+%d]", 8*slot), src)
}

func (g *Generator) leaveC() {
	g.emit("TEB_ENTER_CURRENT_G rbx")
	g.emit("mov rsp, r12")
	g.emit("call rt_exitc")
	for i := len(abi.saved) - 1; i >= 0; i-- {
		g.emit("pop %s", abi.saved[i])
	}
}

func (g *Generator) enterC(label string) {
	for _, r := range abi.saved {
		g.emit("push %s", r)
	}
	g.emit("TLS_GET rax, TLS_G")
	g.emit("mov [rax+G_SCANLO], rsp")
	g.emit("lea r11, [rsp+%d]", abi.savedBytes())
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

func (g *Generator) externalTarget(symbol string) string {
	if g.table != nil {
		return fmt.Sprintf("cfn_%s", symbol)
	}
	return g.platform.externalTarget(symbol)
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
	saved := g.platform.thunkSaved()
	preserved := g.platform.preservedVectors()
	g.emit("push rbp")
	for _, r := range saved {
		g.emit("push %s", r)
	}
	ints, vectors := g.platform.intRegs(), g.platform.vectorRegs()
	registerBytes := abi.alignedBytes(len(ints) + vectors)
	g.emit("mov r13, rsp")
	g.emit("sub rsp, %d", registerBytes+16*len(preserved))
	g.emit("and rsp, -16")
	g.emit("mov r12, rsp")
	for i, r := range ints {
		g.emit("mov [r12+%d], %s", 8*i, r)
	}
	for i := range vectors {
		g.emit("movq qword ptr [r12+%d], xmm%d", 8*(len(ints)+i), i)
	}
	for i, r := range preserved {
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
	if stack := abi.stackWords(f.ParamWords); stack > 0 {
		g.emit("sub rsp, %d", abi.alignedBytes(stack))
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
	for i, r := range preserved {
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
	intRegs, vectorRegs := g.platform.intRegs(), g.platform.vectorRegs()
	positional := g.platform.positional()
	for k, t := range f.CParams {
		var src string
		if positional {
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
			src = fmt.Sprintf("[r13+%d]", incoming+8*(g.platform.registerSlots()+spilled))
			spilled++
		}
		g.emit("%s", widen(t, src))
		if reg, ok := abi.argument(k); ok {
			g.emit("mov %s, r10", reg)
			continue
		}
		g.emit("mov %s, r10", abi.outgoing(k))
	}
}

func widen(t *types.Type, src string) string {
	width := t.Width()
	if width == nil {
		return fmt.Sprintf("mov r10, qword ptr %s", src)
	}
	sized := fmt.Sprintf("%s ptr %s", widths[width.Bits], src)
	if width.Bits == 32 {
		return extension("r10", sized, width)
	}
	return fmt.Sprintf("movzx r10, %s", sized)
}

func (g *Generator) cResult(fr *frame, in *instr, plan *cPlan, k int) {
	dst := fr.loc(in.dst)
	if plan.memory {
		g.emit("mov r10, qword ptr [rsp+%d]", 8*(plan.stackWords+k))
		g.move(dst, "r10")
		return
	}
	ints, vectors := 0, 0
	for _, c := range plan.results[:k] {
		if c == vector {
			vectors++
		} else {
			ints++
		}
	}
	if plan.results[k] == integer {
		g.move(dst, abi.results[ints])
		return
	}
	source := fmt.Sprintf("xmm%d", vectors)
	if plan.ret == types.F32 {
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

func (g *Generator) shim(symbol, target string, setup ...string) {
	g.align(functionAlignment)
	fmt.Fprintf(&g.text, "\t.globl %s\n%s:\n", symbol, symbol)
	for _, line := range setup {
		g.emit("%s", line)
	}
	g.emit("jmp %s", target)
}
