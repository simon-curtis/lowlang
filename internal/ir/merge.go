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
	polls := b.Poll
	next.moveTerminator(b)
	b.Poll = b.Poll || polls
	next.merged = true
	next.Values = nil
	next.Preds = nil
}

func (b *Block) moveTerminator(to *Block) {
	to.Kind = b.Kind
	to.Control = b.Control
	to.Table = b.Table
	to.Results = b.Results
	to.Poll = b.Poll
	to.Succs = b.Succs
	for _, succ := range to.Succs {
		for i, p := range succ.Preds {
			if p == b {
				succ.Preds[i] = to
			}
		}
	}
	b.Kind = Plain
	b.Control = nil
	b.Table = nil
	b.Results = nil
	b.Poll = false
	b.Succs = nil
}

func (b *Block) copyTerminator(from *Block, value func(*Value) *Value, block func(*Block) *Block) {
	b.Kind = from.Kind
	b.Table = from.Table
	b.Poll = from.Poll
	if from.Control != nil {
		b.Control = value(from.Control)
	}
	for _, r := range from.Results {
		b.Results = append(b.Results, value(r))
	}
	for _, p := range from.Preds {
		b.Preds = append(b.Preds, block(p))
	}
	for _, succ := range from.Succs {
		b.Succs = append(b.Succs, block(succ))
	}
}
