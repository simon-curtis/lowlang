package ir

import "slices"

func (s *ssa) mergeBlocks() bool {
	merged := false
	for _, b := range s.fn.Blocks {
		for absorbable(s.fn, b) {
			absorb(b, b.Succs[0])
			merged = true
		}
	}
	if merged {
		s.fn.Blocks = slices.DeleteFunc(s.fn.Blocks, func(b *Block) bool { return b.merged })
	}
	return merged
}

func absorbable(fn *Func, b *Block) bool {
	if b.merged || b.Kind != Plain || len(b.Succs) != 1 {
		return false
	}
	next := b.Succs[0]
	if next == b || next == fn.Blocks[0] || len(next.Preds) != 1 || next.merged {
		return false
	}
	return len(next.Values) == 0 || next.Values[0].Op != OpPhi
}

func absorb(b, next *Block) {
	for _, v := range next.Values {
		v.Block = b
	}
	b.Values = append(b.Values, next.Values...)
	b.Kind = next.Kind
	b.Control = next.Control
	b.Table = next.Table
	b.Results = next.Results
	b.Poll = b.Poll || next.Poll
	b.Succs = next.Succs
	for _, succ := range b.Succs {
		for i, p := range succ.Preds {
			if p == next {
				succ.Preds[i] = b
			}
		}
	}
	next.merged = true
	next.Values = nil
	next.Succs = nil
	next.Preds = nil
}
