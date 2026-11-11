package asm

func (a *assembler) needsPLT(sym *symbol) bool {
	return sym.state == symUndefined || sym.global
}

func (a *assembler) assembleBranch(mnemonic string, ops []Operand) bool {
	if len(ops) != 1 || ops[0].Kind != OpImm {
		return false
	}
	switch mnemonic {
	case "call":
		a.call(&ops[0])
	case "jmp":
		a.relaxableJump(opJmpShort, false, &ops[0])
	default:
		code, ok := conditionalJumps[mnemonic]
		if !ok {
			return false
		}
		a.relaxableJump(opJccShort+code, true, &ops[0])
	}
	return true
}

func (a *assembler) branchTarget(op *Operand) (*symbol, int64) {
	v := op.Value
	if v.Sym == "" || v.Neg != "" || op.Size != 0 {
		a.fail("unsupported branch target")
	}
	return a.lookup(v.Sym), v.Add
}

func (a *assembler) relaxableJump(opcode byte, cond bool, op *Operand) {
	if op.Value.Reloc != RelocNone {
		a.fail("unsupported relocation on a jump")
	}
	sym, off := a.branchTarget(op)
	a.addBranchPadding(opcode)
	a.cur.addJump(opcode, cond, sym, off, a.line)
}

func (a *assembler) call(op *Operand) {
	sym, off := a.branchTarget(op)
	kind := PC32
	switch {
	case op.Value.Reloc == RelocPLT:
		kind = PLT32
	case op.Value.Reloc != RelocNone:
		a.fail("unsupported relocation on a call")
	case off == 0 && a.needsPLT(sym):
		kind = PLT32
	}
	b := a.cur
	at := int32(len(b.Data))
	b.Data = append(b.Data, opCallNear, 0, 0, 0, 0)
	b.addFixup(textFixup{at: at + 1, size: 4, kind: kind, sym: sym, addend: off - 4, line: int32(a.line)})
}
