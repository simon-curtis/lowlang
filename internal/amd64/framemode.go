package amd64

import (
	"slices"

	"lowlang/internal/ir"
)

type frameMode int

const (
	fullFrame frameMode = iota
	deferredFrame
	framelessFrame
)

func (mf *mfunc) frameMode(f *ir.Func, saved []string, vectors bool) frameMode {
	if vectors || f.ParamWords > len(argRegs) {
		return fullFrame
	}
	if mf.guard == nil {
		if len(saved) == 0 && len(f.Slots) == 0 && !slices.ContainsFunc(mf.blocks, (*mblock).needsFrame) {
			return framelessFrame
		}
		return fullFrame
	}
	if slices.ContainsFunc(mf.blocks, func(mb *mblock) bool { return mb.light && mb.needsFrame() }) {
		return fullFrame
	}
	return deferredFrame
}

func (mb *mblock) needsFrame() bool {
	if mb.poll || (mb.kind == ir.Ret && len(mb.results) > 2) {
		return true
	}
	if slices.ContainsFunc(mb.instrs, func(in *instr) bool { return !in.frameFree() }) {
		return true
	}
	for _, st := range mb.steps {
		if slices.ContainsFunc(slices.Concat(st.uses, st.defs), func(r *vreg) bool { return r.spill >= 0 }) {
			return true
		}
	}
	return false
}

func (in *instr) frameFree() bool {
	if m := in.memoryOperand(); m != nil && m.mem.base.frame != nil {
		return false
	}
	switch in.op {
	case ir.OpConst, ir.OpEmbed, ir.OpString, ir.OpUndef, ir.OpAdd, ir.OpSub, ir.OpMul, ir.OpAnd, ir.OpOr, ir.OpXor,
		ir.OpMulHigh, ir.OpTrailingZeros, ir.OpShl, ir.OpShr, ir.OpNeg, ir.OpCompl, ir.OpNot, ir.OpConvert,
		ir.OpSqrt, ir.OpFloor, ir.OpEq, ir.OpNe, ir.OpLt, ir.OpLe, ir.OpGt, ir.OpGe, ir.OpFuncAddr, ir.OpHashSeed:
		return true
	case ir.OpLoad, ir.OpStore:
		return in.args[0].mem.base.frame == nil
	case ir.OpParam:
		return int(in.value.Aux) < len(argRegs)
	case ir.OpDiv, ir.OpRem:
		divisor := in.args[1].konst
		return isFloat(in.value.Type) || (divisor != nil && *divisor != 0)
	}
	return false
}
