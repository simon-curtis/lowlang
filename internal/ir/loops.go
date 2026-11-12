package ir

import "slices"

type loop struct {
	header    *Block
	body      map[*Block]bool
	order     []*Block
	preheader *Block
	latch     *Block
}

func newDominators(fn *Func) *Dominators[*Block] {
	return NewDominators(fn.Blocks[0], blockSuccs, blockPreds)
}

func findLoops(doms *Dominators[*Block]) []*loop {
	var loops []*loop
	for _, natural := range doms.Loops() {
		l := &loop{header: natural.Header, body: natural.Body, order: natural.Order}
		l.settle(natural.Latches)
		loops = append(loops, l)
	}
	return loops
}

func (l *loop) settle(latches []*Block) {
	if len(latches) == 1 {
		l.latch = latches[0]
	}
	var outside []*Block
	for _, p := range l.header.Preds {
		if !l.body[p] {
			outside = append(outside, p)
		}
	}
	if len(outside) == 1 && outside[0].Kind == Plain && len(outside[0].Succs) == 1 {
		l.preheader = outside[0]
	}
}

func (l *loop) invariant(v *Value) bool {
	return v.Op == OpConst || !l.body[v.Block]
}

func (l *loop) invariantArgs(v *Value) bool {
	return !slices.ContainsFunc(v.Args, func(a *Value) bool { return !l.invariant(a) })
}
