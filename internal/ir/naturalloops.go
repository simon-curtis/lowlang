package ir

import "slices"

type NaturalLoop[N comparable] struct {
	Header  N
	Latches []N
	Body    map[N]bool
	Order   []N
}

func (d *Dominators[N]) Loops() []*NaturalLoop[N] {
	var loops []*NaturalLoop[N]
	for i, header := range d.Nodes {
		latches := d.backEdgeSources(i)
		if len(latches) == 0 {
			continue
		}
		l := &NaturalLoop[N]{Header: header, Latches: latches, Body: map[N]bool{header: true}}
		d.collect(l)
		loops = append(loops, l)
	}
	slices.SortStableFunc(loops, func(a, b *NaturalLoop[N]) int { return len(a.Body) - len(b.Body) })
	return loops
}

func (d *Dominators[N]) backEdgeSources(header int) []N {
	var sources []N
	for _, p := range d.preds(d.Nodes[header]) {
		pi, reachable := d.Index[p]
		if reachable && d.dominates(header, pi) {
			sources = append(sources, p)
		}
	}
	return sources
}

func (d *Dominators[N]) dominates(a, b int) bool {
	for b > a {
		b = d.Idom[b]
	}
	return a == b
}

func (d *Dominators[N]) collect(l *NaturalLoop[N]) {
	work := slices.Clone(l.Latches)
	for len(work) > 0 {
		n := work[len(work)-1]
		work = work[:len(work)-1]
		if l.Body[n] {
			continue
		}
		l.Body[n] = true
		for _, p := range d.preds(n) {
			if _, reachable := d.Index[p]; reachable {
				work = append(work, p)
			}
		}
	}
	for _, n := range d.Nodes {
		if l.Body[n] {
			l.Order = append(l.Order, n)
		}
	}
}
