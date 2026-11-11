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
	for i, header := range doms.Nodes {
		latches := backEdgeSources(doms, i)
		if len(latches) == 0 {
			continue
		}
		l := &loop{header: header, body: map[*Block]bool{header: true}}
		l.collect(doms, latches)
		l.settle(latches)
		loops = append(loops, l)
	}
	slices.SortStableFunc(loops, func(a, b *loop) int { return len(a.body) - len(b.body) })
	return loops
}

func backEdgeSources(doms *Dominators[*Block], header int) []*Block {
	var sources []*Block
	for _, p := range doms.Nodes[header].Preds {
		pi, reachable := doms.Index[p]
		if reachable && dominates(doms, header, pi) {
			sources = append(sources, p)
		}
	}
	return sources
}

func dominates(doms *Dominators[*Block], a, b int) bool {
	for b > a {
		b = doms.Idom[b]
	}
	return a == b
}

func (l *loop) collect(doms *Dominators[*Block], latches []*Block) {
	work := slices.Clone(latches)
	for len(work) > 0 {
		b := work[len(work)-1]
		work = work[:len(work)-1]
		if l.body[b] {
			continue
		}
		l.body[b] = true
		for _, p := range b.Preds {
			if _, reachable := doms.Index[p]; reachable {
				work = append(work, p)
			}
		}
	}
	for _, b := range doms.Nodes {
		if l.body[b] {
			l.order = append(l.order, b)
		}
	}
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
