package amd64

import (
	_ "embed"
	"fmt"
	"hash/fnv"
	"slices"
	"strings"

	"lowlang/internal/ir"
	"lowlang/internal/types"
)

//go:embed runtime.asm
var runtime string

//go:embed sched.asm
var scheduler string

//go:embed hot.asm
var hotRuntime string

const (
	HotBase   = 0x40000000
	HotSize   = 64 << 20
	TableSize = 4096
)

type Output struct {
	Asm   string
	Funcs map[string]string
}

type Generator struct {
	text    strings.Builder
	data    strings.Builder
	fn      string
	labels  int
	strings map[string]bool
	table   map[string]int
	suffix  string
	maps    []stackMap
	stubs   []pollStub
	tier    tier
	cache   *vectorCache
}

type frame struct {
	fn            *mfunc
	saved         []string
	slots         []int
	firstSlot     int
	vectorScratch int
	results       int
}

type stackMap struct {
	label   string
	offsets []int
	segment bool
}

var argRegs = []string{"rdi", "rsi", "rdx", "rcx", "r8", "r9"}

func stackWords(params int) int {
	return max(0, params-len(argRegs))
}

func newGenerator(table map[string]int, suffix string) *Generator {
	return &Generator{strings: map[string]bool{}, table: table, suffix: suffix}
}

func Generate(prog *ir.Program, table map[string]int) Output {
	g := newGenerator(table, "")
	funcs := map[string]string{}
	for _, f := range prog.Funcs {
		from := g.text.Len()
		g.function(f)
		funcs[f.Name] = g.text.String()[from:]
	}
	g.start(prog)
	if table != nil {
		g.trampolines(prog.CSymbols)
	}
	for _, f := range prog.Funcs {
		if f.Callback {
			g.thunk(f)
		}
	}
	dynamic := 0
	if len(prog.Libraries) > 0 {
		dynamic = 1
	}
	var out strings.Builder
	fmt.Fprintf(&out, "\t.intel_syntax noprefix\n\t.set RT_DYNAMIC, %d\n", dynamic)
	out.WriteString(scheduler)
	out.WriteString(runtime)
	g.writeMaps(&out)
	if table != nil {
		out.WriteString(hotRuntimeSource())
		g.writeTable(&out)
	}
	out.WriteString("\n\t.section .rodata\n")
	out.WriteString(g.data.String())
	out.WriteString("\n\t.text\n")
	out.WriteString(g.text.String())
	return Output{Asm: out.String(), Funcs: funcs}
}

func GeneratePatch(prog *ir.Program, names []string, table map[string]int, suffix string) string {
	g := newGenerator(table, suffix)
	for _, f := range prog.Funcs {
		if slices.Contains(names, f.Name) {
			g.function(f)
		}
	}
	var out strings.Builder
	out.WriteString("\t.intel_syntax noprefix\n\t.text\n")
	out.WriteString(g.text.String())
	out.WriteString("\n\t.section .rodata\n")
	out.WriteString(g.data.String())
	return out.String()
}

func Symbol(name, suffix string) string {
	return fmt.Sprintf("fn_%s%s", mangle(name), suffix)
}

func mangle(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r == '_' || r == '.' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
		default:
			fmt.Fprintf(&b, ".%x", r)
		}
	}
	return b.String()
}

func hotRuntimeSource() string {
	return strings.NewReplacer(
		"HOT_BASE", fmt.Sprint(HotBase),
		"HOT_SIZE", fmt.Sprint(HotSize),
		"PATCH_BYTES", fmt.Sprint(16*TableSize),
	).Replace(hotRuntime)
}

func (g *Generator) writeTable(out *strings.Builder) {
	entries := make([]string, TableSize)
	for i := range entries {
		entries[i] = "0"
	}
	for name, i := range g.table {
		entries[i] = Symbol(name, "")
	}
	out.WriteString("\n\t.data\n\t.p2align 3\n\t.globl fn_table\nfn_table:\n")
	for _, e := range entries {
		fmt.Fprintf(out, "\t.quad %s\n", e)
	}
}

func (g *Generator) emit(format string, args ...any) {
	g.text.WriteString("\t")
	fmt.Fprintf(&g.text, format, args...)
	g.text.WriteString("\n")
}

func (g *Generator) label(name string) {
	fmt.Fprintf(&g.text, "%s:\n", name)
}

func (g *Generator) newLabel() string {
	g.labels++
	return fmt.Sprintf(".L%s_g%d", g.fn, g.labels)
}

func (g *Generator) callTarget(f *ir.Func) string {
	if g.table == nil {
		return Symbol(f.Name, "")
	}
	return fmt.Sprintf("qword ptr [rip+fn_table+%d]", 8*g.table[f.Name])
}

func (g *Generator) start(prog *ir.Program) {
	g.fn = "start"
	g.labels = 0
	dynamic := len(prog.Libraries) > 0
	if dynamic {
		g.text.WriteString("\t.globl main\nmain:\n")
		g.emit("push rbp")
		g.emit("xor ebp, ebp")
		g.emit("mov rdi, rdx")
	} else {
		g.text.WriteString("\t.globl _start\n_start:\n")
		g.emit("mov rax, [rsp]")
		g.emit("lea rdi, [rsp+8*rax+16]")
	}
	g.emit("call rt_cpu_init")
	g.emit("call rt_gc_init")
	g.emit("call rt_sched_init")
	if g.table != nil {
		g.emit("call rt_hot_init")
	}
	g.emit("lea rdi, [rip+rt_program]")
	g.emit("call rt_spawn_main")
	g.emit("mov rax, qword ptr fs:[-24]")
	g.emit("mov rsp, [rax+16]")
	g.emit("jmp rt_schedule")
	g.program(prog, dynamic)
}

func (g *Generator) program(prog *ir.Program, dynamic bool) {
	g.text.WriteString("rt_program:\n")
	g.emit("push rbp")
	g.emit("mov rbp, rsp")
	words := prog.Main.ResultWords
	if words > 2 {
		g.emit("sub rsp, %d", 16*((words+1)/2))
	}
	g.emit("call %s", g.callTarget(prog.Main))
	if prog.MainErr != nil {
		exit := g.newLabel()
		tag, variant := "rax", "rdx"
		if words > 2 {
			tag, variant = "qword ptr [rsp]", "qword ptr [rsp+8]"
		}
		g.emit("cmp %s, 0", tag)
		g.emit("je %s", exit)
		g.emit("mov rax, %s", variant)
		g.emit("lea rcx, [rip+%s]", g.variantNames(prog.MainErr))
		g.emit("mov rdi, [rcx+8*rax]")
		g.emit("call rt_error")
		g.label(exit)
	}
	g.emit("xor edi, edi")
	if dynamic {
		g.emit("call exit")
		return
	}
	g.emit("mov eax, 231")
	g.emit("syscall")
}

func (g *Generator) variantNames(enum *types.Type) string {
	table := g.newLabel()
	names := make([]string, len(enum.Variants))
	for i, v := range enum.Variants {
		names[i] = g.messageLabel(v.Name)
	}
	fmt.Fprintf(&g.data, "\t.p2align 3\n%s:\n\t.quad %s\n", table, strings.Join(names, ","))
	return table
}

func (g *Generator) stringLabel(s string) string {
	h := fnv.New64a()
	h.Write([]byte(s))
	l := fmt.Sprintf(".Lstr_%x", h.Sum64())
	if g.strings[l] {
		return l
	}
	g.strings[l] = true
	fmt.Fprintf(&g.data, "%s:\n", l)
	if len(s) == 0 {
		return l
	}
	bytes := make([]string, len(s))
	for i := range len(s) {
		bytes[i] = fmt.Sprintf("%d", s[i])
	}
	fmt.Fprintf(&g.data, "\t.byte %s\n", strings.Join(bytes, ","))
	return l
}

func (g *Generator) embedLabel(path string, digest uint64) string {
	l := fmt.Sprintf(".Lembed_%x", digest)
	if g.strings[l] {
		return l
	}
	g.strings[l] = true
	fmt.Fprintf(&g.data, "\t.p2align 4\n%s:\n\t.incbin %q\n", l, path)
	return l
}

func (g *Generator) messageLabel(s string) string {
	l := g.newLabel()
	fmt.Fprintf(&g.data, "\t.p2align 3\n%s:\n\t.quad %d\n\t.ascii %q\n", l, len(s), s)
	return l
}

func (g *Generator) function(f *ir.Func) {
	symbol := Symbol(f.Name, g.suffix)
	if !usesVectors(f) {
		g.tier = baseline
		g.body(f, symbol, mangle(f.Name), false)
		return
	}
	g.label(symbol)
	g.emit("cmp byte ptr [rip+rt_avx2], 0")
	g.emit("jne %s.avx2", symbol)
	g.emit("jmp %s.sse2", symbol)
	g.tier = avx2
	g.body(f, fmt.Sprintf("%s.avx2", symbol), fmt.Sprintf("%s.avx2", mangle(f.Name)), true)
	g.tier = baseline
	g.body(f, fmt.Sprintf("%s.sse2", symbol), fmt.Sprintf("%s.sse2", mangle(f.Name)), true)
}

func (g *Generator) body(f *ir.Func, symbol, prefix string, vectors bool) {
	g.fn = prefix
	g.labels = 0
	g.cache = nil
	if vectors && g.tier == avx2 {
		g.cache = vectorTemps(f)
	}
	mf := lower(f, prefix)
	spills, saved := mf.allocate()
	fr := &frame{fn: mf, saved: saved, results: 16 + 8*stackWords(f.ParamWords)}
	words := len(saved) + spills
	fr.firstSlot = words + 1
	for _, n := range f.Slots {
		words += n
		fr.slots = append(fr.slots, words)
	}
	if vectors {
		words += vectorScratchWords
		fr.vectorScratch = -8 * words
	}
	size := (8*(words+outgoing(f)) + 15) &^ 15
	g.label(symbol)
	g.emit("push rbp")
	g.emit("mov rbp, rsp")
	if size > 0 {
		g.emit("sub rsp, %d", size)
	}
	for i, r := range saved {
		g.emit("mov qword ptr [rbp-%d], %s", 8*(i+1), r)
	}
	g.params(fr, mf.blocks[0])
	for i, mb := range mf.blocks {
		var next *mblock
		if i+1 < len(mf.blocks) {
			next = mf.blocks[i+1]
		}
		g.block(fr, mb, next)
	}
	for _, stub := range g.stubs {
		g.label(stub.label)
		g.leaveVectors()
		g.emit("call rt_poll")
		g.emit("jmp %s", stub.resume)
	}
	g.stubs = nil
}

type pollStub struct {
	label  string
	resume string
}

func (g *Generator) poll(resume string) {
	stub := pollStub{label: g.newLabel(), resume: resume}
	g.emit("cmp byte ptr fs:[%d], 0", tlsFlag)
	g.emit("jne %s", stub.label)
	g.stubs = append(g.stubs, stub)
}

func outgoing(f *ir.Func) int {
	most := 0
	for _, b := range f.Blocks {
		for _, v := range b.Values {
			switch v.Op {
			case ir.OpCall:
				words := stackWords(v.Target.ParamWords)
				if v.Target.ResultWords > 2 {
					words += v.Target.ResultWords
				}
				most = max(most, words)
			case ir.OpCCall:
				words := v.Plan.StackWords
				if v.Plan.Memory {
					words += v.Plan.Ret.Size()
				}
				most = max(most, words)
			}
		}
	}
	return most
}

func (fr *frame) loc(r *vreg) string {
	switch {
	case r.konst != nil:
		return fmt.Sprintf("%d", *r.konst)
	case r.reg != "":
		return r.reg
	}
	return fmt.Sprintf("qword ptr [rbp-%d]", 8*(len(fr.saved)+r.spill+1))
}

func (fr *frame) slot(index int) int {
	return -8 * fr.slots[index]
}

func isMem(loc string) bool {
	return strings.HasPrefix(loc, "qword")
}

func isReg(loc string) bool {
	return !isMem(loc) && !isImm(loc)
}

func isImm(loc string) bool {
	return loc[0] == '-' || (loc[0] >= '0' && loc[0] <= '9')
}

func (g *Generator) move(dst, src string) {
	switch {
	case dst == src:
	case isMem(dst) && isMem(src):
		g.emit("mov r10, %s", src)
		g.emit("mov %s, r10", dst)
	default:
		g.emit("mov %s, %s", dst, src)
	}
}

func (g *Generator) block(fr *frame, mb *mblock, next *mblock) {
	g.label(mb.label)
	if g.cache != nil {
		g.cache.reset()
	}
	for _, in := range mb.instrs {
		g.instr(fr, in)
	}
	switch mb.kind {
	case ir.Plain:
		if mb.poll {
			g.poll(mb.succs[0].label)
		}
		if mb.succs[0] != next {
			g.emit("jmp %s", mb.succs[0].label)
		}
	case ir.If:
		g.branch(fr, mb, next)
	case ir.Ret:
		g.ret(fr, mb)
	case ir.Unreachable:
		g.emit("ud2")
	}
}

func (g *Generator) branch(fr *frame, mb *mblock, next *mblock) {
	cond := "nz"
	if mb.fused != nil {
		g.compare(fr, mb.compare[0], mb.compare[1])
		cond = condition(mb.fused)
	} else {
		g.test(fr.loc(mb.control))
	}
	then, otherwise := mb.succs[0], mb.succs[1]
	if then == next {
		g.emit("j%s %s", inverse[cond], otherwise.label)
		return
	}
	g.emit("j%s %s", cond, then.label)
	if otherwise != next {
		g.emit("jmp %s", otherwise.label)
	}
}

func (g *Generator) test(loc string) {
	switch {
	case isReg(loc):
		g.emit("test %s, %s", loc, loc)
	case isImm(loc):
		g.emit("mov r10, %s", loc)
		g.emit("test r10, r10")
	default:
		g.emit("cmp %s, 0", loc)
	}
}

func (g *Generator) compare(fr *frame, a, b *vreg) {
	x := fr.loc(a)
	if !isReg(x) {
		g.move("r10", x)
		x = "r10"
	}
	g.emit("cmp %s, %s", x, fr.loc(b))
}

func (g *Generator) ret(fr *frame, mb *mblock) {
	if len(mb.results) <= 2 {
		for i, r := range mb.results {
			g.move([]string{"rax", "rdx"}[i], fr.loc(r))
		}
	} else {
		for k, r := range mb.results {
			g.move(fmt.Sprintf("qword ptr [rbp+%d]", fr.results+8*k), fr.loc(r))
		}
	}
	for i, r := range fr.saved {
		g.emit("mov %s, qword ptr [rbp-%d]", r, 8*(i+1))
	}
	g.emit("mov rsp, rbp")
	g.emit("pop rbp")
	g.leaveVectors()
	g.emit("ret")
}

var binops = map[ir.Op]string{
	ir.OpAdd: "add", ir.OpSub: "sub", ir.OpMul: "imul", ir.OpAnd: "and", ir.OpOr: "or", ir.OpXor: "xor",
}

func (g *Generator) instr(fr *frame, in *instr) {
	if in.move {
		g.move(fr.loc(in.dst), fr.loc(in.args[0]))
		return
	}
	if in.isParam() {
		return
	}
	v := in.value
	switch in.op {
	case ir.OpConst:
		g.constant(fr.loc(in.dst), v.Aux)
	case ir.OpEmbed:
		g.address(fr.loc(in.dst), fmt.Sprintf("[rip+%s]", g.embedLabel(v.Str, uint64(v.Aux))))
	case ir.OpString:
		g.address(fr.loc(in.dst), fmt.Sprintf("[rip+%s]", g.stringLabel(v.Str)))
	case ir.OpUndef:
	case ir.OpAdd, ir.OpSub, ir.OpMul:
		g.arithmetic(fr, in, map[ir.Op]string{ir.OpAdd: "add", ir.OpSub: "sub", ir.OpMul: "mul"}[in.op])
	case ir.OpAnd, ir.OpOr, ir.OpXor:
		g.binary(fr, in, binops[in.op])
	case ir.OpMulHigh:
		g.mulHigh(fr, in)
	case ir.OpVector, ir.OpVectorScalar:
		g.vector(fr, in)
	case ir.OpTrailingZeros:
		g.move("rax", fr.loc(in.args[0]))
		g.emit("mov edx, 64")
		g.emit("bsf rax, rax")
		g.emit("cmovz rax, rdx")
		g.move(fr.loc(in.dst), "rax")
	case ir.OpShl, ir.OpShr:
		g.shift(fr, in)
	case ir.OpDiv, ir.OpRem:
		g.divide(fr, in)
	case ir.OpNeg, ir.OpCompl:
		g.negate(fr, in)
	case ir.OpNot:
		g.unary(fr, in, "xor r10, 1")
	case ir.OpConvert:
		g.convert(fr, in)
	case ir.OpSqrt, ir.OpFloor:
		g.floatUnary(fr, in)
	case ir.OpEq, ir.OpNe, ir.OpLt, ir.OpLe, ir.OpGt, ir.OpGe:
		g.compareValues(fr, in)
	case ir.OpLoad:
		g.load(fr, in)
	case ir.OpStore:
		g.store(fr, in)
	case ir.OpCheck:
		g.compare(fr, in.args[0], in.args[1])
		g.emit("jae rt_panic_index")
	case ir.OpSlot:
		g.address(fr.loc(in.dst), fmt.Sprintf("[rbp%+d]", fr.slot(int(v.Aux))))
	case ir.OpFuncAddr:
		g.funcAddr(fr.loc(in.dst), v)
	case ir.OpCall:
		g.call(fr, in)
	case ir.OpRuntime:
		g.runtimeCall(fr, in)
	case ir.OpCCall:
		g.cCall(fr, in)
	case ir.OpResult:
		g.result(fr, in)
	}
}

func (g *Generator) base(loc string) string {
	if isReg(loc) {
		return loc
	}
	g.move("r10", loc)
	return "r10"
}

func (g *Generator) address(dst, operand string) {
	if isReg(dst) {
		g.emit("lea %s, %s", dst, operand)
		return
	}
	g.emit("lea r10, %s", operand)
	g.move(dst, "r10")
}

func (g *Generator) constant(dst string, n int64) {
	if isReg(dst) || (n >= -1<<31 && n < 1<<31) {
		g.emit("mov %s, %d", dst, n)
		return
	}
	g.emit("mov r10, %d", n)
	g.emit("mov %s, r10", dst)
}

func (g *Generator) binary(fr *frame, in *instr, op string) {
	dst, a, b := fr.loc(in.dst), fr.loc(in.args[0]), fr.loc(in.args[1])
	commutative := in.op != ir.OpSub
	switch {
	case isReg(dst) && dst != b:
		g.move(dst, a)
		g.emit("%s %s, %s", op, dst, b)
	case isReg(dst) && commutative:
		g.emit("%s %s, %s", op, dst, a)
	default:
		g.move("r10", a)
		g.emit("%s r10, %s", op, b)
		g.move(dst, "r10")
	}
}

func (g *Generator) unary(fr *frame, in *instr, op string) {
	g.move("r10", fr.loc(in.args[0]))
	g.emit("%s", op)
	g.move(fr.loc(in.dst), "r10")
}

func (g *Generator) params(fr *frame, entry *mblock) {
	var moves []pmove
	for _, in := range entry.instrs {
		if !in.isParam() || in.value.Uses == 0 {
			continue
		}
		k := int(in.value.Aux)
		src := fmt.Sprintf("qword ptr [rbp+%d]", 16+8*(k-len(argRegs)))
		if k < len(argRegs) {
			src = argRegs[k]
		}
		moves = append(moves, pmove{dst: fr.loc(in.dst), src: src})
	}
	g.parallel(moves)
}

func (g *Generator) funcAddr(dst string, v *ir.Value) {
	switch {
	case v.Aux == ir.SymbolAddr:
		reg := dst
		if !isReg(dst) {
			reg = "r10"
		}
		g.emit("mov %s, qword ptr [rip+%s@GOTPCREL]", reg, v.Str)
		g.move(dst, reg)
	case v.Aux == ir.ThunkAddr:
		g.address(dst, fmt.Sprintf("[rip+cb_%s]", mangle(v.Target.Name)))
	case g.table != nil:
		g.address(dst, fmt.Sprintf("[rip+fn_table+%d]", 8*g.table[v.Target.Name]))
	default:
		g.address(dst, fmt.Sprintf("[rip+%s]", Symbol(v.Target.Name, "")))
	}
}

func (g *Generator) callee(v *ir.Value) string {
	switch {
	case !v.Indirect:
		return g.callTarget(v.Target)
	case g.table != nil:
		return "qword ptr [r10]"
	}
	return "r10"
}

func (g *Generator) call(fr *frame, in *instr) {
	args := in.args
	var moves []pmove
	if in.value.Indirect {
		moves = append(moves, pmove{dst: "r10", src: fr.loc(args[0])})
		args = args[1:]
	}
	for i, a := range args {
		src := fr.loc(a)
		if i < len(argRegs) {
			moves = append(moves, pmove{dst: argRegs[i], src: src})
			continue
		}
		if isMem(src) {
			g.move("r10", src)
			src = "r10"
		}
		g.emit("mov qword ptr [rsp+%d], %s", 8*(i-len(argRegs)), src)
	}
	g.parallel(moves)
	g.leaveVectors()
	g.emit("call %s", g.callee(in.value))
	g.safepoint(fr, in)
}

type pmove struct {
	dst string
	src string
}

func (g *Generator) parallel(moves []pmove) {
	moves = slices.DeleteFunc(moves, func(m pmove) bool { return m.dst == m.src })
	for len(moves) > 0 {
		i := slices.IndexFunc(moves, func(m pmove) bool {
			return !slices.ContainsFunc(moves, func(o pmove) bool { return o.src == m.dst })
		})
		if i < 0 {
			blocked := moves[0].src
			g.move("r11", blocked)
			for j := range moves {
				if moves[j].src == blocked {
					moves[j].src = "r11"
				}
			}
			continue
		}
		g.move(moves[i].dst, moves[i].src)
		moves = slices.Delete(moves, i, i+1)
	}
}

func (g *Generator) safepoint(fr *frame, in *instr) {
	g.safepointAt(fr, in, g.newLabel())
}

func (g *Generator) safepointAt(fr *frame, in *instr, label string) {
	g.label(label)
	var offsets []int
	for i := range fr.saved {
		offsets = append(offsets, -8*(i+1))
	}
	for _, r := range fr.fn.vregs {
		if r.spill >= 0 && !r.scalar && r.start < in.pos && in.pos < r.end {
			offsets = append(offsets, -8*(len(fr.saved)+r.spill+1))
		}
	}
	if n := len(fr.slots); n > 0 {
		for word := fr.firstSlot; word <= fr.slots[n-1]; word++ {
			offsets = append(offsets, -8*word)
		}
	}
	g.maps = append(g.maps, stackMap{label: label, offsets: offsets})
}

func (g *Generator) writeMaps(out *strings.Builder) {
	out.WriteString("\n\t.section .rodata\n\t.p2align 3\n\t.globl rt_maps\nrt_maps:\n")
	fmt.Fprintf(out, "\t.quad %d\n", len(g.maps))
	for i, m := range g.maps {
		fmt.Fprintf(out, "\t.quad %s, .Lmap%d\n", m.label, i)
	}
	for i, m := range g.maps {
		count := len(m.offsets)
		if m.segment {
			count = -1
		}
		fmt.Fprintf(out, ".Lmap%d:\n\t.quad %d\n", i, count)
		for _, off := range m.offsets {
			fmt.Fprintf(out, "\t.quad %d\n", off)
		}
	}
}

var syscallRegs = []string{"rax", "rdi", "rsi", "rdx", "r10", "r8", "r9"}

func (g *Generator) runtimeCall(fr *frame, in *instr) {
	regs := argRegs
	if in.value.Str == "syscall" {
		regs = syscallRegs
	}
	moves := make([]pmove, len(in.args))
	for i, a := range in.args {
		moves[i] = pmove{dst: regs[i], src: fr.loc(a)}
	}
	g.parallel(moves)
	if in.value.Str == "syscall" {
		g.emit("syscall")
		return
	}
	g.leaveVectors()
	g.emit("call rt_%s", in.value.Str)
	g.safepoint(fr, in)
}

func (g *Generator) result(fr *frame, in *instr) {
	k := int(in.value.Aux)
	call := in.call
	if call.Op == ir.OpCCall {
		g.cResult(fr, in, call.Plan, k)
		return
	}
	if call.Op == ir.OpRuntime || call.Target.ResultWords <= 2 {
		g.move(fr.loc(in.dst), []string{"rax", "rdx"}[k])
		return
	}
	g.emit("mov r10, qword ptr [rsp+%d]", 8*(stackWords(call.Target.ParamWords)+k))
	g.move(fr.loc(in.dst), "r10")
}
