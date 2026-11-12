package amd64

import "lowlang/internal/ir"

const (
	maxWeightedDepth = 4
	depthFactor      = 8
	saveCost         = 2
)

func (f *mfunc) loops() []*ir.NaturalLoop[*mblock] {
	return f.dominators(f.predecessors()).Loops()
}

func (f *mfunc) loopDepths() map[*mblock]int {
	depths := map[*mblock]int{}
	for _, l := range f.loops() {
		for mb := range l.Body {
			depths[mb]++
		}
	}
	return depths
}

func (f *mfunc) weigh() {
	for _, r := range f.vregs {
		r.weight = 0
	}
	depths := f.loopDepths()
	for _, mb := range f.blocks {
		cost := 1
		for range min(depths[mb], maxWeightedDepth) {
			cost *= depthFactor
		}
		for _, st := range mb.steps {
			for _, r := range st.uses {
				r.weight += cost
			}
			for _, r := range st.defs {
				r.weight += cost
			}
		}
	}
}
