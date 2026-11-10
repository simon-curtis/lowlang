package amd64

import (
	"fmt"
	"strconv"

	"lowlang/internal/ir"
	"lowlang/internal/types"
)

var narrow = map[string][3]string{
	"rax": {"eax", "ax", "al"}, "rbx": {"ebx", "bx", "bl"}, "rcx": {"ecx", "cx", "cl"},
	"rdx": {"edx", "dx", "dl"}, "rsi": {"esi", "si", "sil"}, "rdi": {"edi", "di", "dil"},
	"r8": {"r8d", "r8w", "r8b"}, "r9": {"r9d", "r9w", "r9b"}, "r10": {"r10d", "r10w", "r10b"},
	"r11": {"r11d", "r11w", "r11b"}, "r12": {"r12d", "r12w", "r12b"}, "r13": {"r13d", "r13w", "r13b"},
	"r14": {"r14d", "r14w", "r14b"}, "r15": {"r15d", "r15w", "r15b"},
}

var signedConditions = map[ir.Op]string{
	ir.OpEq: "e", ir.OpNe: "ne", ir.OpLt: "l", ir.OpLe: "le", ir.OpGt: "g", ir.OpGe: "ge",
}

var unsignedConditions = map[ir.Op]string{
	ir.OpEq: "e", ir.OpNe: "ne", ir.OpLt: "b", ir.OpLe: "be", ir.OpGt: "a", ir.OpGe: "ae",
}

var inverse = map[string]string{
	"e": "ne", "ne": "e", "l": "ge", "ge": "l", "le": "g", "g": "le",
	"b": "ae", "ae": "b", "be": "a", "a": "be", "nz": "z", "z": "nz",
}

func condition(v *ir.Value) string {
	if v.Type != nil && v.Type.Integer() && !v.Type.Signed {
		return unsignedConditions[v.Op]
	}
	return signedConditions[v.Op]
}

func isFloat(t *types.Type) bool {
	return t != nil && t.Float()
}

func precision(t *types.Type) string {
	if t.Bits == 32 {
		return "s"
	}
	return "d"
}

func (g *Generator) normalize(loc string, t *types.Type) {
	if t == nil || t.Bits == 0 || t.Bits == 64 {
		return
	}
	reg := loc
	if !isReg(loc) {
		g.move("r10", loc)
		reg = "r10"
	}
	sub := narrow[reg]
	switch {
	case t.Integer() && t.Signed:
		g.emit("movsxd %s, %s", reg, sub[0])
	case t.Bits == 32:
		g.emit("mov %s, %s", sub[0], sub[0])
	case t.Bits == 16:
		g.emit("movzx %s, %s", sub[0], sub[1])
	default:
		g.emit("movzx %s, %s", sub[0], sub[2])
	}
	g.move(loc, reg)
}

func (g *Generator) toXmm(xmm, loc string) {
	if isImm(loc) {
		g.move("r10", loc)
		loc = "r10"
	}
	g.emit("movq %s, %s", xmm, loc)
}

func (g *Generator) fromXmm(dst string, t *types.Type) {
	if isReg(dst) {
		g.emit("movq %s, xmm0", dst)
	} else {
		g.emit("movq r10, xmm0")
		g.move(dst, "r10")
	}
	g.normalize(dst, t)
}

func (g *Generator) arithmetic(fr *frame, in *instr, op string) {
	t := in.value.Type
	if isFloat(t) {
		g.toXmm("xmm0", fr.loc(in.args[0]))
		g.toXmm("xmm1", fr.loc(in.args[1]))
		g.emit("%ss%s xmm0, xmm1", op, precision(t))
		g.fromXmm(fr.loc(in.dst), t)
		return
	}
	g.binary(fr, in, binops[in.op])
	g.normalize(fr.loc(in.dst), t)
}

func (g *Generator) negate(fr *frame, in *instr) {
	t := in.value.Type
	g.move("r10", fr.loc(in.args[0]))
	switch {
	case isFloat(t):
		g.emit("btc r10, %d", t.Bits-1)
	case in.op == ir.OpCompl:
		g.emit("not r10")
	default:
		g.emit("neg r10")
	}
	g.normalize("r10", t)
	g.move(fr.loc(in.dst), "r10")
}

func (g *Generator) divide(fr *frame, in *instr) {
	t := in.value.Type
	if isFloat(t) {
		g.arithmetic(fr, in, "div")
		return
	}
	done := g.newLabel()
	g.move("rax", fr.loc(in.args[0]))
	g.move("r10", fr.loc(in.args[1]))
	g.emit("test r10, r10")
	g.emit("jz rt_panic_divzero")
	if t != nil && !t.Signed {
		g.emit("xor edx, edx")
		g.emit("div r10")
	} else {
		g.signedDivide(in.op, done)
	}
	if in.op == ir.OpRem {
		g.emit("mov rax, rdx")
	}
	g.label(done)
	g.normalize("rax", t)
	g.move(fr.loc(in.dst), "rax")
}

func (g *Generator) signedDivide(op ir.Op, done string) {
	normal := g.newLabel()
	g.emit("cmp r10, -1")
	g.emit("jne %s", normal)
	if op == ir.OpDiv {
		g.emit("neg rax")
	} else {
		g.emit("xor eax, eax")
	}
	g.emit("jmp %s", done)
	g.label(normal)
	g.emit("cqo")
	g.emit("idiv r10")
}

func (g *Generator) shift(fr *frame, in *instr) {
	t := in.value.Type
	op := "shl"
	if in.op == ir.OpShr {
		op = "shr"
		if t.Signed {
			op = "sar"
		}
	}
	g.move("r10", fr.loc(in.args[0]))
	count := fr.loc(in.args[1])
	if n, err := strconv.ParseInt(count, 10, 64); err == nil {
		if n >= 0 && n < 64 {
			g.emit("%s r10, %d", op, n)
		} else {
			g.overshift(op)
		}
	} else {
		g.variableShift(op, count)
	}
	g.normalize("r10", t)
	g.move(fr.loc(in.dst), "r10")
}

func (g *Generator) variableShift(op, count string) {
	big := g.newLabel()
	done := g.newLabel()
	g.move("r11", "rcx")
	if count != "rcx" {
		g.emit("mov rcx, %s", count)
	}
	g.emit("cmp rcx, 63")
	g.emit("ja %s", big)
	g.emit("%s r10, cl", op)
	g.emit("jmp %s", done)
	g.label(big)
	g.overshift(op)
	g.label(done)
	g.move("rcx", "r11")
}

func (g *Generator) overshift(op string) {
	if op == "sar" {
		g.emit("sar r10, 63")
		return
	}
	g.emit("xor r10d, r10d")
}

func (g *Generator) compareValues(fr *frame, in *instr) {
	t := in.value.Type
	if !isFloat(t) {
		g.compare(fr, in.args[0], in.args[1])
		g.emit("set%s al", condition(in.value))
	} else {
		g.floatCompare(fr, in)
	}
	g.emit("movzx eax, al")
	g.move(fr.loc(in.dst), "rax")
}

func (g *Generator) floatCompare(fr *frame, in *instr) {
	t := in.value.Type
	g.toXmm("xmm0", fr.loc(in.args[0]))
	g.toXmm("xmm1", fr.loc(in.args[1]))
	ucomis := "ucomis" + precision(t)
	switch in.op {
	case ir.OpEq, ir.OpNe:
		g.emit("%s xmm0, xmm1", ucomis)
		if in.op == ir.OpEq {
			g.emit("sete al")
			g.emit("setnp r11b")
			g.emit("and al, r11b")
		} else {
			g.emit("setne al")
			g.emit("setp r11b")
			g.emit("or al, r11b")
		}
	case ir.OpLt:
		g.emit("%s xmm1, xmm0", ucomis)
		g.emit("seta al")
	case ir.OpLe:
		g.emit("%s xmm1, xmm0", ucomis)
		g.emit("setae al")
	case ir.OpGt:
		g.emit("%s xmm0, xmm1", ucomis)
		g.emit("seta al")
	case ir.OpGe:
		g.emit("%s xmm0, xmm1", ucomis)
		g.emit("setae al")
	}
}

func (g *Generator) convert(fr *frame, in *instr) {
	from, to := in.value.From, in.value.Type
	src, dst := fr.loc(in.args[0]), fr.loc(in.dst)
	switch {
	case from.Integer() && to.Integer():
		g.move("r10", src)
		g.normalize("r10", to)
		g.move(dst, "r10")
	case from == types.U64:
		g.move("r10", src)
		g.unsignedToFloat(precision(to))
		g.fromXmm(dst, to)
	case from.Integer():
		g.move("r10", src)
		g.emit("cvtsi2s%s xmm0, r10", precision(to))
		g.fromXmm(dst, to)
	case to == types.U64:
		g.toXmm("xmm0", src)
		g.floatToUnsigned(precision(from))
		g.move(dst, "r10")
	case to.Integer():
		g.toXmm("xmm0", src)
		g.emit("cvtts%s2si r10, xmm0", precision(from))
		g.normalize("r10", to)
		g.move(dst, "r10")
	default:
		g.toXmm("xmm0", src)
		g.emit("cvts%s2s%s xmm0, xmm0", precision(from), precision(to))
		g.fromXmm(dst, to)
	}
}

func (g *Generator) unsignedToFloat(p string) {
	big := g.newLabel()
	done := g.newLabel()
	g.emit("test r10, r10")
	g.emit("js %s", big)
	g.emit("cvtsi2s%s xmm0, r10", p)
	g.emit("jmp %s", done)
	g.label(big)
	g.emit("mov r11, r10")
	g.emit("shr r10, 1")
	g.emit("and r11d, 1")
	g.emit("or r10, r11")
	g.emit("cvtsi2s%s xmm0, r10", p)
	g.emit("adds%s xmm0, xmm0", p)
	g.label(done)
}

func (g *Generator) floatToUnsigned(p string) {
	big := g.newLabel()
	done := g.newLabel()
	limit := int64(0x43E0000000000000)
	if p == "s" {
		limit = 0x5F000000
	}
	g.emit("mov r10, %d", limit)
	g.emit("movq xmm1, r10")
	g.emit("ucomis%s xmm0, xmm1", p)
	g.emit("jae %s", big)
	g.emit("cvtts%s2si r10, xmm0", p)
	g.emit("jmp %s", done)
	g.label(big)
	g.emit("subs%s xmm0, xmm1", p)
	g.emit("cvtts%s2si r10, xmm0", p)
	g.emit("btc r10, 63")
	g.label(done)
}

func (g *Generator) mulHigh(fr *frame, in *instr) {
	g.move("rax", fr.loc(in.args[0]))
	g.move("r10", fr.loc(in.args[1]))
	g.emit("mul r10")
	g.move(fr.loc(in.dst), "rdx")
}

func (g *Generator) floatUnary(fr *frame, in *instr) {
	t := in.value.Type
	g.toXmm("xmm0", fr.loc(in.args[0]))
	if in.op == ir.OpSqrt {
		g.emit("sqrts%s xmm0, xmm0", precision(t))
	} else {
		g.emit("rounds%s xmm0, xmm0, 1", precision(t))
	}
	g.fromXmm(fr.loc(in.dst), t)
}

var widths = map[int]string{8: "byte", 16: "word", 32: "dword"}

func (g *Generator) load(fr *frame, in *instr) {
	t := in.value.Type
	memory := fmt.Sprintf("[%s+%d]", g.base(fr.loc(in.args[0])), in.value.Aux)
	dst := fr.loc(in.dst)
	if t == nil {
		g.move(dst, fmt.Sprintf("qword ptr %s", memory))
		return
	}
	reg := dst
	if !isReg(dst) {
		reg = "r10"
	}
	sized := fmt.Sprintf("%s ptr %s", widths[t.Bits], memory)
	switch {
	case t.Integer() && t.Signed:
		g.emit("movsxd %s, %s", reg, sized)
	case t.Bits == 32:
		g.emit("mov %s, %s", narrow[reg][0], sized)
	default:
		g.emit("movzx %s, %s", narrow[reg][0], sized)
	}
	g.move(dst, reg)
}

func (g *Generator) store(fr *frame, in *instr) {
	t := in.value.Type
	base := g.base(fr.loc(in.args[0]))
	value := fr.loc(in.args[1])
	if isMem(value) {
		g.move("r11", value)
		value = "r11"
	}
	width := "qword"
	if t != nil {
		width = widths[t.Bits]
		if isReg(value) {
			value = narrow[value][map[int]int{8: 2, 16: 1, 32: 0}[t.Bits]]
		}
	}
	g.emit("mov %s ptr [%s+%d], %s", width, base, in.value.Aux, value)
}
