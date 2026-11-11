package ir

import (
	"slices"

	"lowlang/internal/types"
)

type induction struct {
	phi     *Value
	step    int64
	initArg *Value
	incIdx  int
	initIdx int
	derived map[int64]*Value
}

func (s *ssa) reduceStrength(l *loop) {
	if l.latch == nil || len(l.header.Preds) != 2 {
		return
	}
	for _, phi := range slices.Clone(l.header.Values) {
		if phi.Op != OpPhi {
			break
		}
		iv, ok := basicInduction(l, phi)
		if ok {
			s.reduceMultiples(l, iv)
		}
	}
}

func basicInduction(l *loop, phi *Value) (*induction, bool) {
	initIdx := slices.Index(l.header.Preds, l.preheader)
	incIdx := slices.Index(l.header.Preds, l.latch)
	if initIdx < 0 || incIdx < 0 || len(phi.Args) != 2 || !wordArithmetic(phi.Args[incIdx].Type) {
		return nil, false
	}
	inc := phi.Args[incIdx]
	if (inc.Op != OpAdd && inc.Op != OpSub) || inc.Args[0] != phi || inc.Args[1].Op != OpConst {
		return nil, false
	}
	step := inc.Args[1].Aux
	if inc.Op == OpSub {
		step = -step
	}
	return &induction{phi: phi, step: step, initArg: phi.Args[initIdx], incIdx: incIdx, initIdx: initIdx, derived: map[int64]*Value{}}, true
}

func wordArithmetic(t *types.Type) bool {
	return t == nil || (t.Integer() && t.Bits == 64)
}

func (s *ssa) reduceMultiples(l *loop, iv *induction) {
	for _, b := range l.order {
		for _, v := range b.Values {
			if v.Op == OpMul && v.forward == nil && v.Args[0] == iv.phi && v.Args[1].Op == OpConst && wordArithmetic(v.Type) {
				forwardTo(v, s.derivedInduction(l, iv, v))
			}
		}
	}
}

func (s *ssa) derivedInduction(l *loop, iv *induction, mul *Value) *Value {
	stride := mul.Args[1].Aux
	if p, ok := iv.derived[stride]; ok {
		return p
	}
	start := s.scaledStart(l, iv, mul, stride)
	p := s.fn.newValue(l.header, OpPhi)
	p.Args = make([]*Value, 2)
	l.header.Values = append([]*Value{p}, l.header.Values...)
	next := s.fn.newValue(l.latch, OpAdd, p, s.constantIn(l.latch, iv.step*stride))
	next.Type = mul.Type
	next.Scalar = mul.Scalar
	l.latch.Values = append(l.latch.Values, next)
	p.Args[iv.initIdx] = start
	p.Args[iv.incIdx] = next
	iv.derived[stride] = p
	return p
}

func (s *ssa) scaledStart(l *loop, iv *induction, mul *Value, stride int64) *Value {
	if iv.initArg.Op == OpConst {
		return s.constantIn(l.preheader, iv.initArg.Aux*stride)
	}
	scaled := s.fn.newValue(l.preheader, OpMul, iv.initArg, s.constantIn(l.preheader, stride))
	scaled.Type = mul.Type
	scaled.Scalar = mul.Scalar
	l.preheader.Values = append(l.preheader.Values, scaled)
	return scaled
}

func (s *ssa) constantIn(b *Block, n int64) *Value {
	c := s.fn.newValue(b, OpConst)
	c.Aux = n
	c.Scalar = true
	b.Values = append(b.Values, c)
	return c
}
