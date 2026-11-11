package amd64

import (
	"slices"

	"lowlang/internal/ir"
)

var resultRegisters = []string{"rax", "rdx"}

func (in *instr) returnsInRegisters() bool {
	switch in.op {
	case ir.OpRuntime:
		return true
	case ir.OpCall:
		return in.value.Target.ResultWords <= len(resultRegisters)
	}
	return false
}

func (f *mfunc) pinResults() {
	for _, mb := range f.blocks {
		for i, in := range mb.instrs {
			if in.returnsInRegisters() {
				mb.pinChain(i)
			}
		}
	}
}

func (mb *mblock) pinChain(call int) {
	end := call + 1
	var results []*instr
	for end < len(mb.instrs) && mb.instrs[end].op == ir.OpResult && mb.instrs[end].call == mb.instrs[call].value {
		results = append(results, mb.instrs[end])
		end++
	}
	for _, in := range results {
		k := int(in.value.Aux)
		if k < len(resultRegisters) && !in.dst.float && in.value.Uses == 1 && mb.consumes(end, in.dst) {
			in.dst.pinned = resultRegisters[k]
		}
	}
}

func (mb *mblock) consumes(next int, r *vreg) bool {
	if next < len(mb.instrs) {
		return mb.instrs[next].consumes(r)
	}
	return !mb.poll && mb.terminatorConsumes(r)
}

func (in *instr) consumes(r *vreg) bool {
	at := slices.Index(in.args, r)
	if at < 0 {
		return false
	}
	switch in.op {
	case ir.OpAdd, ir.OpSub, ir.OpMul, ir.OpAnd, ir.OpOr, ir.OpXor:
		return !isFloat(in.value.Type)
	case ir.OpNot, ir.OpNeg, ir.OpCompl:
		return !isFloat(in.value.Type)
	case ir.OpStore:
		return at == 1
	case ir.OpCall, ir.OpRuntime:
		return true
	}
	return false
}

func (mb *mblock) terminatorConsumes(r *vreg) bool {
	switch mb.kind {
	case ir.Ret:
		return slices.Contains(mb.results, r)
	case ir.If:
		return mb.control == r || slices.Contains(mb.compare, r)
	}
	return slices.ContainsFunc(mb.copies, func(c *phiCopy) bool { return c.src == r })
}
