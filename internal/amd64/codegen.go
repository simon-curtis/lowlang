package amd64

import (
	_ "embed"
	"fmt"
	"hash/fnv"
	"slices"
	"strconv"
	"strings"

	"lowlang/internal/ir"
	"lowlang/internal/target"
	"lowlang/internal/types"
)

//go:embed layout.asm
var layout string

//go:embed linux_header.asm
var linuxHeader string

//go:embed linux.asm
var linuxRuntime string

//go:embed windows_header.asm
var windowsHeader string

//go:embed windows.asm
var windowsRuntime string

//go:embed runtime.asm
var runtime string

//go:embed sched.asm
var scheduler string

//go:embed gc.asm
var collector string

//go:embed hot.asm
var hotRuntime string

type Output struct {
	Asm   string
	Funcs map[string]string
}

type Generator struct {
	platform   Platform
	plans      map[*ir.CSig]*cPlan
	text       strings.Builder
	data       strings.Builder
	fn         string
	labels     int
	dataLabels map[string]bool
	previous   moved
	table      FuncTable
	suffix     string
	maps       []stackMap
	stubs      []pollStub
	jumps      []jumpTable
	tier       tier
	cache      *vectorCache
}

type moved struct {
	dst string
	src string
}

type frame struct {
	gen           *Generator
	fn            *mfunc
	saved         []string
	slots         []int
	firstSlot     int
	vectorScratch int
	results       int
	size          int
	direct        *vreg
	mode          frameMode
}

type stackMap struct {
	label   string
	offsets []int
	segment bool
}

func newGenerator(platform Platform, table FuncTable, suffix string) *Generator {
	return &Generator{platform: platform, dataLabels: map[string]bool{}, plans: map[*ir.CSig]*cPlan{}, table: table, suffix: suffix}
}

func hotInitMacro(hot bool) string {
	if hot {
		return "\t.macro HOT_INIT\n\tcall rt_hot_init\n\t.endm\n"
	}
	return "\t.macro HOT_INIT\n\t.endm\n"
}

func bit(on bool) int {
	if on {
		return 1
	}
	return 0
}

type symbol struct {
	name  string
	value int
}

func writeSymbols(out *strings.Builder, symbols []symbol) {
	for _, s := range symbols {
		fmt.Fprintf(out, "\t.set %s, %d\n", s.name, s.value)
	}
}

func Generate(prog *ir.Program, platform Platform, table FuncTable) (Output, error) {
	if table != nil && platform.OS() != target.Linux {
		return Output{}, fmt.Errorf("hot reload needs the %s target, not %s", target.Linux, platform.OS())
	}
	g := newGenerator(platform, table, "")
	funcs := map[string]string{}
	for _, f := range prog.Funcs {
		from := g.text.Len()
		g.function(f)
		funcs[f.Name] = g.text.String()[from:]
	}
	g.start(prog)
	if table != nil {
		platform.trampolines(g, prog.CSymbols)
	}
	for _, f := range prog.Funcs {
		if f.Callback {
			g.thunk(f)
		}
	}
	dynamic := platform.dynamic(prog)
	if dynamic {
		platform.shims(g, prog.CSymbols)
	}
	var out strings.Builder
	out.WriteString("\t.intel_syntax noprefix\n")
	writeSymbols(&out, []symbol{{"RT_DYNAMIC", bit(dynamic)}})
	out.WriteString(hotInitMacro(table != nil))
	out.WriteString(layout)
	out.WriteString(platform.header())
	out.WriteString(platform.runtime())
	out.WriteString(scheduler)
	out.WriteString(runtime)
	out.WriteString(collector)
	g.writeMaps(&out)
	writeCells(&out, prog.Cells)
	if table != nil {
		table.writeRuntime(&out)
	}
	platform.imports(&out, prog)
	out.WriteString("\n\t.section .rodata\n")
	out.WriteString(g.data.String())
	out.WriteString("\n\t.text\n")
	out.WriteString(g.text.String())
	return Output{Asm: out.String(), Funcs: funcs}, nil
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

func (g *Generator) emit(format string, args ...any) {
	g.previous = moved{}
	g.text.WriteString("\t")
	fmt.Fprintf(&g.text, format, args...)
	g.text.WriteString("\n")
}

const (
	functionAlignment = 5
	loopAlignment     = 5
	maxAlignedInstrs  = 4
)

func (g *Generator) align(power int) {
	fmt.Fprintf(&g.text, "\t.p2align %d\n", power)
}

func (g *Generator) label(name string) {
	g.previous = moved{}
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
	return fmt.Sprintf("qword ptr %s", g.table.slot(f.Name))
}

func (g *Generator) start(prog *ir.Program) {
	g.fn = "start"
	g.labels = 0
	g.emit("PROGRAM_ENTRY %s", g.platform.entry())
	g.program(prog)
}

func (g *Generator) program(prog *ir.Program) {
	g.text.WriteString("rt_program:\n")
	g.emit("push rbp")
	g.emit("mov rbp, rsp")
	words := prog.Main.ResultWords()
	inRegisters := abi.returnsInRegisters(words)
	if !inRegisters {
		g.emit("sub rsp, %d", abi.alignedBytes(words))
	}
	g.emit("call %s", g.callTarget(prog.Main))
	if prog.MainErr != nil {
		exit := g.newLabel()
		tag, variant := abi.results[0], abi.results[1]
		if !inRegisters {
			tag, variant = abi.stackResult(0, 0), abi.stackResult(0, 1)
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
	g.emit("PROGRAM_EXIT")
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
	if g.dataLabels[l] {
		return l
	}
	g.dataLabels[l] = true
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
	if g.dataLabels[l] {
		return l
	}
	g.dataLabels[l] = true
	fmt.Fprintf(&g.data, "\t.p2align 4\n%s:\n\t.incbin %s\n", l, QuoteString(path))
	return l
}

func QuoteString(text string) string {
	var out strings.Builder
	out.WriteByte('"')
	for i := 0; i < len(text); i++ {
		c := text[i]
		switch {
		case c == '"' || c == '\\':
			out.WriteByte('\\')
			out.WriteByte(c)
		case c >= 0x20 && c < 0x7f:
			out.WriteByte(c)
		default:
			fmt.Fprintf(&out, "\\%03o", c)
		}
	}
	out.WriteByte('"')
	return out.String()
}

func (g *Generator) messageLabel(s string) string {
	l := g.newLabel()
	fmt.Fprintf(&g.data, "\t.p2align 3\n\tMSG_BEGIN %s\n\t.ascii %q\n\tMSG_END\n", l, s)
	return l
}

func (g *Generator) function(f *ir.Func) {
	symbol := Symbol(f.Name, g.suffix)
	if !usesVectors(f) {
		g.tier = baseline
		g.body(f, symbol, mangle(f.Name), false)
		return
	}
	g.align(functionAlignment)
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
	mf := lower(f, prefix, !vectors, !vectors)
	spills, saved := mf.allocate()
	mode := mf.frameMode(f, saved, vectors)
	if mf.guard != nil && len(saved) == 0 && mode != deferredFrame {
		mf = lower(f, prefix, !vectors, false)
		spills, saved = mf.allocate()
		mode = mf.frameMode(f, saved, vectors)
	}
	fr := &frame{gen: g, fn: mf, mode: mode, saved: saved, results: abi.resultsOffset(f.ParamWords)}
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
	size := (8*(words+g.outgoing(f)) + 15) &^ 15
	fr.size = size
	g.align(functionAlignment)
	g.label(symbol)
	if mode == fullFrame {
		g.enterFrame(fr)
		if mf.guard == nil {
			g.saveRegisters(fr)
		}
	}
	g.params(fr, mf.blocks[0])
	emitted := fr.emitted()
	for i, mb := range emitted {
		var next *mblock
		if i+1 < len(emitted) {
			next = emitted[i+1]
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
	g.flushJumpTables()
}

type pollStub struct {
	label  string
	resume string
}

func (g *Generator) poll() {
	stub := pollStub{label: g.newLabel(), resume: g.newLabel()}
	g.emit("TLS_POLL")
	g.emit("jne %s", stub.label)
	g.label(stub.resume)
	g.stubs = append(g.stubs, stub)
}

func (g *Generator) outgoing(f *ir.Func) int {
	most := 0
	for _, b := range f.Blocks {
		for _, v := range b.Values {
			switch v.Op {
			case ir.OpCall:
				signature := v.CallSignature()
				words := abi.stackWords(signature.ParamWords)
				if !abi.returnsInRegisters(signature.ResultWords()) {
					words += signature.ResultWords()
				}
				most = max(most, words)
			case ir.OpCCall:
				most = max(most, g.plan(v.Sig).outgoingWords())
			}
		}
	}
	return most
}

func (fr *frame) loc(r *vreg) string {
	switch {
	case r == fr.direct:
		return "rax"
	case r.pinned != "":
		return r.pinned
	case r.konst != nil:
		return fmt.Sprintf("%d", *r.konst)
	case r.mem != nil:
		return fmt.Sprintf("qword ptr %s", fr.address(r))
	case r.pool != nil:
		return fmt.Sprintf("qword ptr [rip+%s]", fr.gen.floatLabel(*r.pool))
	case r.reg != "":
		return r.reg
	}
	return fmt.Sprintf("qword ptr [rbp-%d]", 8*(len(fr.saved)+r.spill+1))
}

func (fr *frame) slot(index int) int {
	return -8 * fr.slots[index]
}

func isMem(loc string) bool {
	return strings.HasPrefix(loc, "qword") || strings.HasPrefix(loc, "dword")
}

func isReg(loc string) bool {
	return !isMem(loc) && !isImm(loc) && !isXmm(loc)
}

func isImm(loc string) bool {
	return loc[0] == '-' || (loc[0] >= '0' && loc[0] <= '9')
}

func isXmm(loc string) bool {
	return strings.HasPrefix(loc, "xmm")
}

func (g *Generator) move(dst, src string) {
	switch {
	case dst == src, g.previous == moved{dst: src, src: dst}:
	case isXmm(dst):
		g.moveToXmm(dst, src)
	case isXmm(src):
		g.emit("movq %s, %s", dst, src)
	case isMem(dst) && isMem(src):
		g.emit("mov r10, %s", src)
		g.emit("mov %s, r10", dst)
	default:
		g.emit("mov %s, %s", dst, src)
		g.previous = moved{dst: dst, src: src}
	}
}

func (g *Generator) moveToXmm(dst, src string) {
	switch {
	case isXmm(src):
		g.emit("movaps %s, %s", dst, src)
	default:
		g.emit("movq %s, %s", dst, src)
	}
}

func (g *Generator) floatLabel(bits int64) string {
	label := fmt.Sprintf(".Lfc_%x", uint64(bits))
	if !g.dataLabels[label] {
		g.dataLabels[label] = true
		fmt.Fprintf(&g.data, "\t.p2align 3\n%s:\n\t.quad %d\n", label, bits)
	}
	return label
}

func (g *Generator) signMask(bits int) string {
	label := fmt.Sprintf(".Lsign_%d", bits)
	if !g.dataLabels[label] {
		g.dataLabels[label] = true
		fmt.Fprintf(&g.data, "\t.p2align 4\n%s:\n\t.quad 0x%x, 0\n", label, uint64(1)<<(bits-1))
	}
	return label
}

func (g *Generator) loadFloatBits(dst string, bits int64) {
	if bits == 0 {
		g.emit("xorps %s, %s", dst, dst)
		return
	}
	g.emit("movq %s, qword ptr [rip+%s]", dst, g.floatLabel(bits))
}

func (g *Generator) saveRegisters(fr *frame) {
	for i, r := range fr.saved {
		g.emit("mov qword ptr [rbp-%d], %s", 8*(i+1), r)
	}
}

func (fr *frame) empty(mb *mblock) bool {
	if mb.kind != ir.Plain || mb.poll || len(mb.instrs) != 0 || len(mb.heads) != 0 || mb == fr.fn.guard {
		return false
	}
	return !slices.ContainsFunc(mb.copies, func(c *phiCopy) bool { return fr.loc(c.dst) != fr.loc(c.src) })
}

func (fr *frame) alignsLoop(mb *mblock) bool {
	if !mb.poll || mb.kind != ir.If || len(mb.instrs)+len(mb.copies) > maxAlignedInstrs {
		return false
	}
	return slices.ContainsFunc(mb.succs, func(s *mblock) bool { return fr.final(s) == mb })
}

func (fr *frame) final(mb *mblock) *mblock {
	for hops := 0; hops < len(fr.fn.blocks) && fr.empty(mb); hops++ {
		mb = mb.succs[0]
	}
	return mb
}

func (fr *frame) emitted() []*mblock {
	reached := map[*mblock]bool{}
	var visit func(mb *mblock)
	visit = func(mb *mblock) {
		if reached[mb] {
			return
		}
		reached[mb] = true
		for _, s := range mb.succs {
			visit(fr.final(s))
		}
	}
	visit(fr.fn.blocks[0])
	return slices.DeleteFunc(slices.Clone(fr.fn.blocks), func(mb *mblock) bool { return !reached[mb] })
}

func (mb *mblock) returnedLast() *vreg {
	if mb.kind != ir.Ret || len(mb.results) == 0 || len(mb.instrs) == 0 {
		return nil
	}
	last := mb.instrs[len(mb.instrs)-1]
	if last.dst == nil || last.dst != mb.results[0] || last.dst.pinned != "" || last.value.Uses != 1 || last.isCall() || last.isParam() {
		return nil
	}
	return last.dst
}

func (g *Generator) copies(fr *frame, copies []*phiCopy) {
	moves := make([]pmove, len(copies))
	for i, c := range copies {
		moves[i] = fr.vectorMove(fr.loc(c.dst), c.src)
	}
	g.parallel(moves)
}

func (g *Generator) block(fr *frame, mb *mblock, next *mblock) {
	if fr.alignsLoop(mb) {
		g.align(loopAlignment)
	}
	g.label(mb.label)
	if g.cache != nil {
		g.cache.reset()
	}
	if mb == fr.fn.guard {
		if fr.mode == deferredFrame {
			g.enterFrame(fr)
		}
		g.saveRegisters(fr)
	}
	fr.direct = nil
	g.copies(fr, mb.heads)
	fr.direct = mb.returnedLast()
	for _, in := range mb.instrs {
		g.instr(fr, in)
	}
	g.copies(fr, mb.copies)
	if mb.poll {
		g.poll()
	}
	switch mb.kind {
	case ir.Plain:
		target := fr.final(mb.succs[0])
		if target != next {
			g.emit("jmp %s", target.label)
		}
	case ir.If:
		g.branch(fr, mb, next)
	case ir.Switch:
		g.jumpTable(fr, mb)
	case ir.Ret:
		g.ret(fr, mb)
	case ir.Unreachable:
		g.emit("ud2")
	}
}

func (g *Generator) branch(fr *frame, mb *mblock, next *mblock) {
	then, otherwise := fr.final(mb.succs[0]), fr.final(mb.succs[1])
	if then == otherwise {
		if then != next {
			g.emit("jmp %s", then.label)
		}
		return
	}
	cond := "nz"
	for _, r := range mb.compare {
		g.reloadPieces(fr, r)
	}
	switch {
	case mb.fused != nil && isFloat(mb.fused.Type):
		g.floatBranch(fr, mb, then, otherwise, next)
		return
	case mb.fused != nil:
		g.compare(fr, mb.compare[0], mb.compare[1])
		cond = condition(mb.fused)
	default:
		g.test(fr.loc(mb.control))
	}
	g.jump(cond, then, otherwise, next)
}

func (g *Generator) jump(cond string, then, otherwise, next *mblock) {
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
	x, y := fr.loc(a), fr.loc(b)
	switch {
	case a.narrowMemory():
		g.emit("cmp %s ptr %s, %s", widths[a.mem.loaded.Bits], fr.address(a), y)
	case isReg(x) && y == "0":
		g.emit("test %s, %s", x, x)
	case isMem(x) && !isMem(y):
		g.emit("cmp %s, %s", x, y)
	default:
		if !isReg(x) {
			scratch := "r10"
			if a.mem != nil || b.mem != nil {
				scratch = "rax"
			}
			g.move(scratch, x)
			x = scratch
		}
		g.emit("cmp %s, %s", x, y)
	}
}

func (g *Generator) ret(fr *frame, mb *mblock) {
	if abi.returnsInRegisters(len(mb.results)) {
		moves := make([]pmove, len(mb.results))
		for i, r := range mb.results {
			moves[i] = fr.vectorMove(abi.results[i], r)
		}
		g.parallel(moves)
	} else {
		for k, r := range mb.results {
			g.move(fmt.Sprintf("qword ptr [rbp+%d]", fr.results+8*k), fr.loc(r))
		}
	}
	if fr.exitsWithoutFrame(mb) {
		g.emit("ret")
		return
	}
	if mb.restores() {
		for i, r := range fr.saved {
			g.emit("mov %s, qword ptr [rbp-%d]", r, 8*(i+1))
		}
	}
	g.epilogue(fr)
	g.leaveVectors()
	g.emit("ret")
}

func (g *Generator) enterFrame(fr *frame) {
	g.emit("push rbp")
	g.emit("mov rbp, rsp")
	g.platform.reserve(g, fr.size)
}

func (fr *frame) exitsWithoutFrame(mb *mblock) bool {
	return fr.mode == framelessFrame || (fr.mode == deferredFrame && mb.light)
}

func (g *Generator) epilogue(fr *frame) {
	if fr.size == 0 {
		g.emit("pop rbp")
		return
	}
	g.emit("leave")
}

var binops = map[ir.Op]string{
	ir.OpAdd: "add", ir.OpSub: "sub", ir.OpMul: "imul", ir.OpAnd: "and", ir.OpOr: "or", ir.OpXor: "xor",
}

var floatOps = map[ir.Op]string{
	ir.OpAdd: "add", ir.OpSub: "sub", ir.OpMul: "mul", ir.OpDiv: "div",
}

func (g *Generator) instr(fr *frame, in *instr) {
	if in.isParam() {
		return
	}
	v := in.value
	for _, a := range in.args {
		g.reloadPieces(fr, a)
	}
	switch in.op {
	case ir.OpConst:
		g.constant(fr.loc(in.dst), v.Aux)
	case ir.OpEmbed:
		g.address(fr.loc(in.dst), fmt.Sprintf("[rip+%s]", g.embedLabel(v.Str, uint64(v.Aux))))
	case ir.OpString:
		g.address(fr.loc(in.dst), fmt.Sprintf("[rip+%s]", g.stringLabel(v.Str)))
	case ir.OpCell:
		g.address(fr.loc(in.dst), g.cellAddress(v.Aux))
	case ir.OpUndef:
	case ir.OpAdd, ir.OpSub, ir.OpMul:
		g.arithmetic(fr, in)
	case ir.OpAnd, ir.OpOr, ir.OpXor:
		if in.rotate != 0 {
			g.rotate(fr, in)
			break
		}
		g.binary(fr, in, binops[in.op])
	case ir.OpMulHigh:
		g.mulHigh(fr, in)
	case ir.OpHashSeed:
		g.hashSeed(fr.loc(in.dst), v.Aux)
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
		g.logicalNot(fr, in)
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
	case ir.OpSyscall:
		g.syscallCall(fr, in)
	case ir.OpCCall:
		g.cCall(fr, in)
	case ir.OpResult:
		g.result(fr, in)
	}
}

func (g *Generator) address(dst, operand string) {
	if isReg(dst) {
		g.emit("lea %s, %s", dst, operand)
		return
	}
	g.emit("lea r10, %s", operand)
	g.move(dst, "r10")
}

func (g *Generator) hashSeed(dst string, index int64) {
	operand := fmt.Sprintf("qword ptr [rip+rt_hash_seed+%d]", 8*index)
	if isReg(dst) {
		g.emit("mov %s, %s", dst, operand)
		return
	}
	g.emit("mov r10, %s", operand)
	g.move(dst, "r10")
}

func (g *Generator) constant(dst string, n int64) {
	if n == 0 && isReg(dst) {
		g.zero(dst)
		return
	}
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
	if commutative && isImm(a) && !isImm(b) {
		a, b = b, a
	}
	work := "r10"
	if in.memoryOperand() != nil {
		work = "rax"
	}
	direct := isReg(dst) && !fr.overwritesAddress(in, dst)
	fit := in.sized()
	switch {
	case isReg(dst) && g.threeOperand(in.op, dst, a, b, fit):
	case direct && dst != b:
		g.move(fit(dst), fit(a))
		g.emit("%s %s, %s", op, fit(dst), fit(b))
	case direct && commutative:
		g.emit("%s %s, %s", op, fit(dst), fit(a))
	default:
		g.move(fit(work), fit(a))
		g.emit("%s %s, %s", op, fit(work), fit(b))
		g.move(dst, work)
	}
}

func (g *Generator) threeOperand(op ir.Op, dst, a, b string, fit func(string) string) bool {
	switch {
	case op == ir.OpMul && isImm(b) && !isImm(a):
		g.emit("imul %s, %s, %s", fit(dst), fit(a), b)
	case op == ir.OpAdd && isReg(a) && isImm(b) && dst != a:
		g.emit("lea %s, [%s%+d]", fit(dst), a, immediate(b))
	case op == ir.OpSub && isReg(a) && isImm(b) && dst != a && immediate(b) != -1<<31:
		g.emit("lea %s, [%s%+d]", fit(dst), a, -immediate(b))
	case op == ir.OpAdd && isReg(a) && isReg(b) && dst != a && dst != b:
		g.emit("lea %s, [%s+%s]", fit(dst), a, b)
	default:
		return false
	}
	return true
}

func immediate(loc string) int64 {
	n, _ := strconv.ParseInt(loc, 10, 64)
	return n
}

func (g *Generator) workRegister(dst string) string {
	if isReg(dst) {
		return dst
	}
	return "r10"
}

func (g *Generator) logicalNot(fr *frame, in *instr) {
	dst := fr.loc(in.dst)
	work := g.workRegister(dst)
	g.move(work, fr.loc(in.args[0]))
	g.emit("xor %s, 1", work)
	g.move(dst, work)
}

func (g *Generator) params(fr *frame, entry *mblock) {
	var moves []pmove
	for _, in := range entry.instrs {
		if !in.isParam() || in.value.Uses == 0 {
			continue
		}
		k := int(in.value.Aux)
		src := abi.incoming(k)
		if reg, ok := abi.argument(k); ok {
			src = reg
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
		g.emit("mov %s, %s", reg, g.platform.symbolAddress(v.Str))
		g.move(dst, reg)
	case v.Aux == ir.ThunkAddr:
		g.address(dst, fmt.Sprintf("[rip+cb_%s]", mangle(v.Target.Name)))
	case g.table != nil:
		g.address(dst, g.table.slot(v.Target.Name))
	default:
		g.address(dst, fmt.Sprintf("[rip+%s]", Symbol(v.Target.Name, "")))
	}
}

func (g *Generator) callee(v *ir.Value) string {
	switch {
	case !v.Indirect():
		return g.callTarget(v.Target)
	case g.table != nil:
		return "qword ptr [r10]"
	}
	return "r10"
}

func (g *Generator) call(fr *frame, in *instr) {
	args := in.args
	var moves []pmove
	if in.value.Indirect() {
		moves = append(moves, pmove{dst: "r10", src: fr.loc(args[0])})
		args = args[1:]
	}
	for i, a := range args {
		src := fr.loc(a)
		if reg, ok := abi.argument(i); ok {
			moves = append(moves, pmove{dst: reg, src: src})
			continue
		}
		g.move(abi.outgoing(i), src)
	}
	g.parallel(moves)
	g.leaveVectors()
	g.emit("call %s", g.callee(in.value))
	g.safepoint(fr, in)
}

type pmove struct {
	dst  string
	src  string
	bits *int64
}

func (fr *frame) vectorMove(dst string, r *vreg) pmove {
	return pmove{dst: dst, src: fr.loc(r), bits: r.konst}
}

func (g *Generator) zero(reg string) {
	low := narrow[reg][0]
	g.emit("xor %s, %s", low, low)
}

func (g *Generator) performMove(m pmove) {
	if m.bits != nil && isXmm(m.dst) {
		g.loadFloatBits(m.dst, *m.bits)
		return
	}
	if m.src == "0" && isReg(m.dst) {
		g.zero(m.dst)
		return
	}
	g.move(m.dst, m.src)
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
		g.performMove(moves[i])
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
		if r.spill >= 0 && !r.scalar && r.liveAcross(in.pos) {
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

func (g *Generator) growCall(fr *frame, in *instr) {
	moves := make([]pmove, len(abi.grow))
	for i, reg := range abi.grow {
		moves[i] = pmove{dst: reg, src: fr.loc(in.args[i])}
	}
	g.parallel(moves)
	g.leaveVectors()
	variant := "scan"
	if scan := in.args[3].konst; scan != nil && *scan == 0 {
		variant = "noscan"
	}
	g.emit("call %s_%s", in.value.Runtime.Symbol, variant)
	g.safepoint(fr, in)
}

func (g *Generator) runtimeCall(fr *frame, in *instr) {
	if in.preservesRegisters() {
		g.growCall(fr, in)
		return
	}
	if in.literal != nil {
		g.literalEquality(fr, in)
		return
	}
	if in.isStringEquality() {
		g.stringEquality(fr, in)
		return
	}
	moves := make([]pmove, len(in.args))
	for i, a := range in.args {
		moves[i] = pmove{dst: abi.args[i], src: fr.loc(a)}
	}
	g.parallel(moves)
	g.leaveVectors()
	g.emit("call %s", in.value.Runtime.Symbol)
	g.safepoint(fr, in)
}

func (g *Generator) syscallCall(fr *frame, in *instr) {
	moves := make([]pmove, len(in.args))
	for i, a := range in.args {
		moves[i] = pmove{dst: abi.syscall[i], src: fr.loc(a)}
	}
	g.parallel(moves)
	if in.inlineSyscall() {
		g.platform.syscall(g)
		return
	}
	g.leaveVectors()
	g.emit("call %s", in.value.Runtime.Symbol)
	g.safepoint(fr, in)
}

func (g *Generator) result(fr *frame, in *instr) {
	if in.dst.pinned != "" {
		return
	}
	k := int(in.value.Aux)
	call := in.call
	if call.Op == ir.OpCCall {
		g.cResult(fr, in, g.plan(call.Sig), k)
		return
	}
	if call.Op != ir.OpCall || abi.returnsInRegisters(call.CallSignature().ResultWords()) {
		g.move(fr.loc(in.dst), abi.results[k])
		return
	}
	g.move(fr.loc(in.dst), abi.stackResult(call.CallSignature().ParamWords, k))
}
