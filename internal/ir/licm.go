package ir

import "slices"

func (s *ssa) optimizeLoops() {
	if !slices.ContainsFunc(s.fn.Blocks, func(b *Block) bool { return b.Poll }) {
		return
	}
	doms := newDominators(s.fn)
	for _, l := range findLoops(doms) {
		if l.preheader == nil {
			continue
		}
		s.hoistInvariants(l)
		s.reduceStrength(l)
	}
	for _, b := range s.fn.Blocks {
		s.resolveBlock(b)
	}
}

func (s *ssa) hoistInvariants(l *loop) {
	for _, b := range l.order {
		kept := b.Values[:0]
		for _, v := range b.Values {
			if hoistable(v) && l.invariantArgs(v) {
				s.localiseConstants(l, v)
				v.Block = l.preheader
				l.preheader.Values = append(l.preheader.Values, v)
				continue
			}
			kept = append(kept, v)
		}
		b.Values = kept
	}
}

func hoistable(v *Value) bool {
	switch v.Op {
	case OpMul, OpAnd, OpOr, OpXor, OpShr, OpNeg, OpNot, OpCompl, OpConvert, OpSqrt, OpFloor, OpFuncAddr, OpMulHigh, OpTrailingZeros, OpHashSeed:
		return true
	case OpAdd, OpSub, OpShl:
		return !addressPiece(v)
	case OpDiv:
		return floatValue(v)
	}
	return false
}

func addressPiece(v *Value) bool {
	return v.Type == nil && len(v.Args) == 2 && v.Args[1].Op == OpConst
}

func (s *ssa) localiseConstants(l *loop, v *Value) {
	for i, a := range v.Args {
		if a.Op != OpConst || !l.body[a.Block] {
			continue
		}
		c := s.constantIn(l.preheader, a.Aux)
		c.FloatBits = a.FloatBits
		c.Type = a.Type
		v.Args[i] = c
	}
}
