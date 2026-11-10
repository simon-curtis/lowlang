package amd64

import (
	"fmt"

	"lowlang/internal/ir"
	"lowlang/internal/types"
)

const (
	tlsM      = -24
	tlsG      = -16
	tlsFlag   = -8
	gScanLo   = 16
	gUserSP   = 24
	gPC       = 32
	gFP       = 40
	gLockedM  = 88
	mG0SP     = 24
	inC       = 3
	savedRegs = 5
	argWords  = 14
)

func (g *Generator) cCall(fr *frame, in *instr) {
	plan := in.value.Plan
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
	label := g.newLabel()
	g.enterC(label)
	if plan.StackWords > 0 {
		g.emit("sub rsp, %d", 16*((plan.StackWords+1)/2))
		for i := range plan.StackWords {
			g.emit("mov r11, [r12+%d]", 8*(savedRegs+i))
			g.emit("mov [rsp+%d], r11", 8*i)
		}
	}
	g.leaveVectors()
	g.emit("mov eax, %d", plan.Vectors)
	g.emit("call %s", target)
	g.safepointAt(fr, in, label)
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
	g.emit("mov rax, qword ptr fs:[%d]", tlsG)
	g.emit("mov [rax+%d], rsp", gScanLo)
	g.emit("lea r11, [rsp+%d]", 8*savedRegs)
	g.emit("mov [rax+%d], r11", gUserSP)
	g.emit("mov [rax+%d], rbp", gFP)
	g.emit("lea r11, [rip+%s]", label)
	g.emit("mov [rax+%d], r11", gPC)
	g.emit("mov rbx, qword ptr fs:[%d]", tlsM)
	g.emit("mov r11d, %d", inC)
	g.emit("xchg [rbx], r11")
	g.emit("mov r12, rsp")
	g.emit("mov rsp, [rbx+%d]", mG0SP)
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

func (g *Generator) thunk(f *ir.Func) {
	g.fn = fmt.Sprintf("cb_%s", mangle(f.Name))
	g.labels = 0
	fmt.Fprintf(&g.text, "\t.globl %s\n%s:\n", g.fn, g.fn)
	g.emit("push rbp")
	for _, r := range calleeSaved {
		g.emit("push %s", r)
	}
	g.emit("mov r13, rsp")
	g.emit("sub rsp, %d", 8*argWords)
	g.emit("and rsp, -16")
	g.emit("mov r12, rsp")
	for i, r := range argRegs {
		g.emit("mov [r12+%d], %s", 8*i, r)
	}
	for i := range 8 {
		g.emit("movq qword ptr [r12+%d], xmm%d", 8*(len(argRegs)+i), i)
	}
	g.emit("mov rbx, qword ptr fs:[%d]", tlsM)
	g.emit("mov edi, %d", inC)
	g.emit("call rt_m_running")
	g.emit("mov r14, [rbx+%d]", mG0SP)
	g.emit("mov [rbx+%d], r12", mG0SP)
	g.emit("mov r15, qword ptr fs:[%d]", tlsG)
	g.emit("inc qword ptr [r15+%d]", gLockedM)
	g.emit("mov rsp, [r15+%d]", gScanLo)
	g.emit("and rsp, -16")
	for _, offset := range []int{gScanLo, gUserSP, gPC, gFP, gPC, gFP} {
		g.emit("push qword ptr [r15+%d]", offset)
	}
	g.emit("mov rbp, rsp")
	g.emit("push r12")
	g.emit("push r14")
	g.emit("push qword ptr [r15+%d]", gUserSP)
	g.emit("sub rsp, 8")
	if stack := stackWords(f.ParamWords); stack > 0 {
		g.emit("sub rsp, %d", 16*((stack+1)/2))
	}
	g.thunkArgs(f)
	label := g.newLabel()
	g.emit("call %s", g.callTarget(f))
	g.label(label)
	g.maps = append(g.maps, stackMap{label: label, segment: true})
	if f.Ret.Float() {
		g.emit("movq xmm0, rax")
	}
	for i, offset := range []int{gFP, gPC, gUserSP, gScanLo} {
		g.emit("mov rcx, [rbp+%d]", 16+8*i)
		g.emit("mov [r15+%d], rcx", offset)
	}
	g.emit("dec qword ptr [r15+%d]", gLockedM)
	g.emit("mov rsp, r12")
	g.emit("mov [rbx+%d], r14", mG0SP)
	g.emit("mov ecx, %d", inC)
	g.emit("xchg [rbx], rcx")
	g.emit("mov rsp, r13")
	for i := len(calleeSaved) - 1; i >= 0; i-- {
		g.emit("pop %s", calleeSaved[i])
	}
	g.emit("pop rbp")
	g.emit("ret")
}

func (g *Generator) thunkArgs(f *ir.Func) {
	ints, vectors, spilled := 0, 0, 0
	for k, t := range f.CParams {
		var src string
		switch {
		case t.Float() && vectors < 8:
			src = fmt.Sprintf("[r12+%d]", 8*(len(argRegs)+vectors))
			vectors++
		case !t.Float() && ints < len(argRegs):
			src = fmt.Sprintf("[r12+%d]", 8*ints)
			ints++
		default:
			src = fmt.Sprintf("[r13+%d]", 8*(len(calleeSaved)+2+spilled))
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
