package asm

const maxOperands = 5

type Prefix uint8

const (
	PrefixLock Prefix = 1 << iota
	PrefixRep
	PrefixRepne
)

type OperandKind uint8

const (
	OpReg OperandKind = iota + 1
	OpMem
	OpImm
)

type Operand struct {
	Kind  OperandKind
	Reg   Reg
	Size  int
	Seg   Reg
	Base  Reg
	Index Reg
	Scale int
	Value Value
}

type Inst struct {
	Mnemonic string
	Prefix   Prefix
	Operands []Operand
}

var prefixes = map[string]Prefix{
	"lock":  PrefixLock,
	"rep":   PrefixRep,
	"repe":  PrefixRep,
	"repz":  PrefixRep,
	"repne": PrefixRepne,
	"repnz": PrefixRepne,
}

var pointerSizes = map[string]int{
	"byte":    1,
	"word":    2,
	"dword":   4,
	"qword":   8,
	"tbyte":   10,
	"xmmword": 16,
	"oword":   16,
	"ymmword": 32,
}

func (a *assembler) instruction(mnemonic string) {
	s := &a.sc
	var prefix Prefix
	for {
		p, ok := prefixes[mnemonic]
		if !ok || s.atEnd() {
			break
		}
		prefix |= p
		mnemonic = s.ident()
		if mnemonic == "" {
			a.fail("expected an instruction after a prefix")
		}
	}
	var ops [maxOperands]Operand
	count := 0
	if !s.atEnd() {
		for {
			if count == maxOperands {
				a.fail("%s has too many operands", mnemonic)
			}
			ops[count] = a.operand()
			count++
			if !s.accept(',') {
				break
			}
		}
	}
	a.expectEnd()
	if !a.cur.exec {
		a.fail("instruction %s in non-executable section %s", mnemonic, a.cur.Name)
	}
	inst := a.newInst()
	inst.Mnemonic = mnemonic
	inst.Prefix = prefix
	inst.Operands = a.keepOperands(ops[:count])
	b := a.cur
	b.Items = append(b.Items, Item{Kind: ItemInst, Line: a.line, Inst: inst})
	b.dataOpen = false
}

func (a *assembler) newInst() *Inst {
	if len(a.insts) == cap(a.insts) {
		a.insts = make([]Inst, 0, 1024)
	}
	a.insts = a.insts[:len(a.insts)+1]
	return &a.insts[len(a.insts)-1]
}

func (a *assembler) keepOperands(ops []Operand) []Operand {
	if cap(a.operands)-len(a.operands) < len(ops) {
		a.operands = make([]Operand, 0, 4096)
	}
	start := len(a.operands)
	a.operands = append(a.operands, ops...)
	return a.operands[start:len(a.operands):len(a.operands)]
}

func (a *assembler) operand() Operand {
	s := &a.sc
	s.skipSpace()
	size := a.pointerSize()
	seg := a.segmentOverride()
	s.skipSpace()
	if s.peek() == '[' {
		return a.memory(size, seg)
	}
	if size == 0 && seg == RegNone {
		if reg, ok := a.registerOperand(); ok {
			return Operand{Kind: OpReg, Reg: reg}
		}
	}
	v := a.parseExpr()
	if seg != RegNone {
		return Operand{Kind: OpMem, Size: size, Seg: seg, Value: v, Scale: 1}
	}
	return Operand{Kind: OpImm, Size: size, Value: v}
}

func (a *assembler) pointerSize() int {
	s := &a.sc
	if !isIdentStart(s.peek()) {
		return 0
	}
	start := s.pos
	size, ok := pointerSizes[s.ident()]
	if !ok {
		s.pos = start
		return 0
	}
	s.skipSpace()
	if s.ident() != "ptr" {
		a.fail("expected ptr after the size keyword")
	}
	s.skipSpace()
	return size
}

func (a *assembler) segmentOverride() Reg {
	s := &a.sc
	if !isIdentStart(s.peek()) {
		return RegNone
	}
	start := s.pos
	name := s.ident()
	reg := lookupReg(name)
	if reg != RegNone && reg.Class() == ClassSegment && s.peek() == ':' {
		s.pos++
		return reg
	}
	s.pos = start
	return RegNone
}

func (a *assembler) registerOperand() (Reg, bool) {
	s := &a.sc
	if !isIdentStart(s.peek()) {
		return RegNone, false
	}
	start := s.pos
	name := s.ident()
	reg := lookupReg(name)
	if reg != RegNone && (s.atEnd() || s.peek() == ',') {
		return reg, true
	}
	s.pos = start
	return RegNone, false
}

func (a *assembler) memory(size int, seg Reg) Operand {
	s := &a.sc
	s.pos++
	op := Operand{Kind: OpMem, Size: size, Seg: seg, Scale: 1}
	sign := int64(1)
	for {
		s.skipSpace()
		if c := s.peek(); c == '-' || c == '+' {
			sign = 1
			if c == '-' {
				sign = -1
			}
			s.pos++
			s.skipSpace()
		}
		if !a.registerTerm(&op, sign) {
			a.displacementTerm(&op, sign)
		}
		sign = 1
		s.skipSpace()
		if s.peek() == ']' {
			s.pos++
			return op
		}
		if c := s.peek(); c != '-' && c != '+' {
			a.fail("unexpected %q in memory operand", s.rest())
		}
	}
}

func (a *assembler) registerTerm(op *Operand, sign int64) bool {
	s := &a.sc
	start := s.pos
	scale, reg, ok := a.scaledRegister()
	if !ok {
		s.pos = start
		return false
	}
	if sign < 0 {
		a.fail("a register cannot be subtracted")
	}
	switch {
	case scale == 0 && op.Base == RegNone:
		op.Base = reg
	case scale == 0 && op.Index == RegNone:
		op.Index, op.Scale = reg, 1
	case scale != 0 && op.Index == RegNone:
		if scale != 1 && scale != 2 && scale != 4 && scale != 8 {
			a.fail("bad index scale %d", scale)
		}
		op.Index, op.Scale = reg, int(scale)
	default:
		a.fail("too many registers in memory operand")
	}
	return true
}

func (a *assembler) scaledRegister() (int64, Reg, bool) {
	s := &a.sc
	if isIdentStart(s.peek()) {
		name := s.ident()
		reg := lookupReg(name)
		if reg == RegNone {
			return 0, RegNone, false
		}
		s.skipSpace()
		if s.peek() != '*' {
			return 0, reg, true
		}
		s.pos++
		s.skipSpace()
		scale := a.parseUnary()
		if !scale.IsConst() {
			a.fail("index scale must be a constant")
		}
		return scale.Add, reg, true
	}
	if !isDigit(s.peek()) {
		return 0, RegNone, false
	}
	scale, ok := a.scaleBeforeRegister()
	if !ok {
		return 0, RegNone, false
	}
	name := s.ident()
	reg := lookupReg(name)
	return scale, reg, reg != RegNone
}

func (a *assembler) scaleBeforeRegister() (int64, bool) {
	s := &a.sc
	if !isDigit(s.peek()) {
		return 0, false
	}
	scale := a.parseNumber()
	s.skipSpace()
	if s.peek() != '*' || !scale.IsConst() {
		return 0, false
	}
	s.pos++
	s.skipSpace()
	if !isIdentStart(s.peek()) {
		return 0, false
	}
	return scale.Add, true
}

func (a *assembler) displacementTerm(op *Operand, sign int64) {
	v := a.parseRank(rankBitwise)
	if sign < 0 {
		v = v.negated()
	}
	op.Value = a.combine(op.Value, v)
}
