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
}

type frame struct {
	saved   []string
	slots   []int
	results int
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
	var out strings.Builder
	out.WriteString(runtime)
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
	return fmt.Sprintf("fn_%s%s", name, suffix)
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
	g.text.WriteString("\t.globl _start\n_start:\n")
	g.emit("mov [rip+rt_stack_top], rsp")
	g.emit("call rt_gc_init")
	if g.table != nil {
		g.emit("call rt_hot_init")
	}
	words := prog.Main.ResultWords
	if words > 2 {
		g.emit("sub rsp, %d", 8*words)
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
	g.emit("mov eax, 60")
	g.emit("syscall")
}

func (g *Generator) variantNames(enum *types.Type) string {
	table := g.newLabel()
	names := make([]string, len(enum.Variants))
	for i, v := range enum.Variants {
		names[i] = g.stringLabel(v.Name)
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
	fmt.Fprintf(&g.data, "\t.p2align 3\n%s:\n\t.quad %d\n", l, len(s))
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

func (g *Generator) function(f *ir.Func) {
	g.fn = f.Name
	g.labels = 0
	mf := lower(f)
	spills, saved := mf.allocate()
	fr := &frame{saved: saved, results: 16 + 8*stackWords(f.ParamWords)}
	words := len(saved) + spills
	for _, n := range f.Slots {
		words += n
		fr.slots = append(fr.slots, words)
	}
	size := (8*(words+outgoing(f)) + 15) &^ 15
	g.label(Symbol(f.Name, g.suffix))
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
}

func outgoing(f *ir.Func) int {
	most := 0
	for _, b := range f.Blocks {
		for _, v := range b.Values {
			if v.Op != ir.OpCall {
				continue
			}
			words := stackWords(v.Target.ParamWords)
			if v.Target.ResultWords > 2 {
				words += v.Target.ResultWords
			}
			most = max(most, words)
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
	for _, in := range mb.instrs {
		g.instr(fr, in)
	}
	switch mb.kind {
	case ir.Plain:
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
	case ir.OpString:
		g.address(fr.loc(in.dst), fmt.Sprintf("[rip+%s]", g.stringLabel(v.Str)))
	case ir.OpUndef:
	case ir.OpAdd, ir.OpSub, ir.OpMul:
		g.arithmetic(fr, in, map[ir.Op]string{ir.OpAdd: "add", ir.OpSub: "sub", ir.OpMul: "mul"}[in.op])
	case ir.OpAnd, ir.OpOr, ir.OpXor:
		g.binary(fr, in, binops[in.op])
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
		g.move(fr.loc(in.dst), fmt.Sprintf("qword ptr [%s+%d]", g.base(fr.loc(in.args[0])), v.Aux))
	case ir.OpStore:
		base := g.base(fr.loc(in.args[0]))
		value := fr.loc(in.args[1])
		if isMem(value) {
			g.move("r11", value)
			value = "r11"
		}
		g.emit("mov qword ptr [%s+%d], %s", base, v.Aux, value)
	case ir.OpSlot:
		g.address(fr.loc(in.dst), fmt.Sprintf("[rbp%+d]", fr.slot(int(v.Aux))))
	case ir.OpCall:
		g.call(fr, in)
	case ir.OpRuntime:
		g.runtimeCall(fr, in)
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

func (g *Generator) call(fr *frame, in *instr) {
	var moves []pmove
	for i, a := range in.args {
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
	g.emit("call %s", g.callTarget(in.value.Target))
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

func (g *Generator) runtimeCall(fr *frame, in *instr) {
	scratch := []string{"r10", "r11"}
	for i, a := range in.args {
		g.move(scratch[i], fr.loc(a))
	}
	for i := range in.args {
		g.move([]string{"rdi", "rsi"}[i], scratch[i])
	}
	g.emit("call rt_%s", in.value.Str)
}

func (g *Generator) result(fr *frame, in *instr) {
	k := int(in.value.Aux)
	call := in.call
	if call.Op == ir.OpRuntime || call.Target.ResultWords <= 2 {
		g.move(fr.loc(in.dst), []string{"rax", "rdx"}[k])
		return
	}
	g.emit("mov r10, qword ptr [rsp+%d]", 8*(stackWords(call.Target.ParamWords)+k))
	g.move(fr.loc(in.dst), "r10")
}
