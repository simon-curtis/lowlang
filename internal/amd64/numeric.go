package amd64

import (
	"fmt"
	"strings"

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
	return conditionOf(v.Op, v.Type == nil || !v.Type.Integer() || v.Type.Signed)
}

func conditionOf(op ir.Op, signed bool) string {
	if signed {
		return signedConditions[op]
	}
	return unsignedConditions[op]
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
	g.emit("%s", extension(reg, narrow[reg][subRegister[t.Bits]], t))
	g.move(loc, reg)
}

var subRegister = map[int]int{32: 0, 16: 1, 8: 2}

func extension(dst, src string, t *types.Type) string {
	switch {
	case t.Integer() && t.Signed:
		return fmt.Sprintf("movsxd %s, %s", dst, src)
	case t.Bits == 32:
		return fmt.Sprintf("mov %s, %s", narrow[dst][0], src)
	}
	return fmt.Sprintf("movzx %s, %s", narrow[dst][0], src)
}

func (g *Generator) arithmetic(fr *frame, in *instr) {
	t := in.value.Type
	if isFloat(t) {
		g.floatArithmetic(fr, in)
		return
	}
	g.binary(fr, in, binops[in.op])
	if !in.word32() {
		g.normalize(fr.loc(in.dst), t)
	}
}

func (in *instr) word32() bool {
	t := in.value.Type
	return t != nil && t.Integer() && !t.Signed && t.Bits == 32
}

func dword(loc string) string {
	switch {
	case isReg(loc):
		return narrow[loc][0]
	case isMem(loc):
		return strings.Replace(loc, "qword", "dword", 1)
	}
	return loc
}

func (in *instr) sized() func(string) string {
	if in.word32() {
		return dword
	}
	return func(loc string) string { return loc }
}

func widens(from, to *types.Type) bool {
	if !from.Integer() || !to.Integer() || to.Bits == 0 || from.Bits == 0 {
		return false
	}
	switch {
	case !from.Signed && !to.Signed:
		return to.Bits >= from.Bits
	case !from.Signed:
		return to.Bits > from.Bits
	}
	return to.Signed && to.Bits >= from.Bits
}

func (g *Generator) negate(fr *frame, in *instr) {
	t := in.value.Type
	if isFloat(t) {
		g.floatNegate(fr, in)
		return
	}
	dst := fr.loc(in.dst)
	work := g.workRegister(dst)
	g.move(work, fr.loc(in.args[0]))
	operation, operand := "neg", work
	if in.op == ir.OpCompl {
		operation = "not"
	}
	if in.word32() {
		operand = dword(work)
	}
	g.emit("%s %s", operation, operand)
	if !in.word32() {
		g.normalize(work, t)
	}
	g.move(dst, work)
}

func (g *Generator) divide(fr *frame, in *instr) {
	t := in.value.Type
	if isFloat(t) {
		g.arithmetic(fr, in)
		return
	}
	divisor := in.args[1].konst
	zeroPossible := divisor == nil || *divisor == 0
	minusOnePossible := divisor == nil || *divisor == -1
	done := g.newLabel()
	g.move("rax", fr.loc(in.args[0]))
	g.move("r10", fr.loc(in.args[1]))
	if zeroPossible {
		g.emit("test r10, r10")
		g.emit("jz rt_panic_divzero")
	}
	if t != nil && !t.Signed {
		g.emit("xor edx, edx")
		g.emit("div r10")
	} else {
		g.signedDivide(in.op, done, minusOnePossible)
	}
	if in.op == ir.OpRem {
		g.emit("mov rax, rdx")
	}
	g.label(done)
	g.normalize("rax", t)
	g.move(fr.loc(in.dst), "rax")
}

func (g *Generator) signedDivide(op ir.Op, done string, minusOnePossible bool) {
	if !minusOnePossible {
		g.emit("cqo")
		g.emit("idiv r10")
		return
	}
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
	dst := fr.loc(in.dst)
	count := in.args[1]
	if count.konst == nil {
		g.move("r10", fr.loc(in.args[0]))
		g.variableShift(op, fr.loc(count))
		if op == "shl" {
			g.normalize("r10", t)
		}
		g.move(dst, "r10")
		return
	}
	work := g.workRegister(dst)
	g.move(work, fr.loc(in.args[0]))
	n := *count.konst
	switch {
	case op == "shl" && in.word32() && n >= 0 && n < 32:
		g.emit("shl %s, %d", dword(work), n)
	default:
		g.constantShift(op, work, n)
		if op == "shl" {
			g.normalize(work, t)
		}
	}
	g.move(dst, work)
}

func (g *Generator) constantShift(op, reg string, n int64) {
	if n >= 0 && n < 64 {
		g.emit("%s %s, %d", op, reg, n)
		return
	}
	g.overshift(op, reg)
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
	g.overshift(op, "r10")
	g.label(done)
	g.move("rcx", "r11")
}

func (g *Generator) overshift(op, reg string) {
	if op == "sar" {
		g.emit("sar %s, 63", reg)
		return
	}
	g.emit("xor %s, %s", narrow[reg][0], narrow[reg][0])
}

func (g *Generator) compareValues(fr *frame, in *instr) {
	t := in.value.Type
	if !isFloat(t) {
		g.compare(fr, in.args[0], in.args[1])
		cond := condition(in.value)
		if in.negated {
			cond = inverse[cond]
		}
		g.emit("set%s al", cond)
	} else {
		g.floatCompare(fr, in)
	}
	g.emit("movzx eax, al")
	g.move(fr.loc(in.dst), "rax")
}

func (g *Generator) convert(fr *frame, in *instr) {
	from, to := in.value.From, in.value.Type
	src, dst := fr.loc(in.args[0]), fr.loc(in.dst)
	switch {
	case from.Integer() && to.Integer() && widens(from, to):
		g.move(dst, src)
	case from.Integer() && to.Integer():
		g.move("r10", src)
		g.normalize("r10", to)
		g.move(dst, "r10")
	case from == types.U64:
		g.unsignedToFloat(src, dst, to)
	case from.Integer():
		g.integerToFloat(src, dst, to)
	case to == types.U64:
		g.floatToUnsigned(fr, in.args[0], dst, from)
	case to.Integer():
		g.floatToInteger(fr, in.args[0], dst, from, to)
	default:
		g.floatToFloat(fr, in.args[0], dst, from, to)
	}
}

func (g *Generator) mulHigh(fr *frame, in *instr) {
	g.move("rax", fr.loc(in.args[0]))
	factor := fr.loc(in.args[1])
	if !isReg(factor) {
		g.move("r10", factor)
		factor = "r10"
	}
	if t := in.value.Type; t != nil && t.Signed {
		g.emit("imul %s", factor)
	} else {
		g.emit("mul %s", factor)
	}
	g.move(fr.loc(in.dst), "rdx")
}

var widths = map[int]string{8: "byte", 16: "word", 32: "dword"}

func (g *Generator) load(fr *frame, in *instr) {
	t := in.value.Type
	memory := fr.address(in.args[0])
	dst := fr.loc(in.dst)
	if isXmm(dst) {
		g.floatLoad(dst, t, memory)
		return
	}
	if t == nil {
		g.move(dst, fmt.Sprintf("qword ptr %s", memory))
		return
	}
	reg := dst
	if !isReg(dst) {
		reg = "r10"
	}
	sized := fmt.Sprintf("%s ptr %s", widths[t.Bits], memory)
	g.emit("%s", extension(reg, sized, t))
	g.move(dst, reg)
}

func (g *Generator) store(fr *frame, in *instr) {
	t := in.value.Type
	memory := fr.address(in.args[0])
	value := fr.loc(in.args[1])
	if isXmm(value) {
		if g.floatStore(memory, in, value) {
			return
		}
		g.move("rax", value)
		value = "rax"
	}
	if isMem(value) {
		g.move("rax", value)
		value = "rax"
	}
	width := "qword"
	if t != nil {
		width = widths[t.Bits]
		if isReg(value) {
			value = narrow[value][subRegister[t.Bits]]
		}
	}
	g.emit("mov %s ptr %s, %s", width, memory, value)
}
