package amd64

import (
	"slices"

	"lowlang/internal/ir"
)

const maxReturnPredecessors = 4

func (mb *mblock) bareReturn() bool {
	return mb.kind == ir.Ret && len(mb.instrs) == 0 && len(mb.copies) == 0 && !mb.poll
}

func (mb *mblock) returnsDirectly() bool {
	return mb.kind == ir.Plain && !mb.poll && len(mb.succs) == 1
}

func (f *mfunc) duplicateReturns() {
	preds := f.predecessors()
	for _, ret := range slices.Clone(f.blocks) {
		from := preds[ret]
		if !ret.bareReturn() || len(from) < 2 || len(from) > maxReturnPredecessors {
			continue
		}
		if !slices.ContainsFunc(from, func(p *mblock) bool { return p.returnsDirectly() }) {
			continue
		}
		for _, p := range from {
			if p.returnsDirectly() {
				p.returnInstead(ret)
			}
		}
		if len(f.predecessors()[ret]) == 0 {
			f.blocks = slices.DeleteFunc(f.blocks, func(mb *mblock) bool { return mb == ret })
		}
	}
}

func (p *mblock) returnInstead(ret *mblock) {
	sources := map[*vreg]*vreg{}
	for _, c := range p.copies {
		sources[c.dst] = c.src
	}
	p.results = nil
	for _, r := range ret.results {
		if src, ok := sources[r]; ok {
			r = src
		}
		p.results = append(p.results, r)
	}
	p.copies = nil
	p.kind = ir.Ret
	p.succs = nil
}
