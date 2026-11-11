package amd64

const (
	maxWeightedDepth = 4
	depthFactor      = 8
	saveCost         = 2
)

func (f *mfunc) loopBodies() []map[*mblock]bool {
	preds := f.predecessors()
	bodies := map[*mblock]map[*mblock]bool{}
	var ordered []map[*mblock]bool
	state := map[*mblock]int{}
	var visit func(mb *mblock)
	visit = func(mb *mblock) {
		state[mb] = 1
		for _, s := range mb.succs {
			switch state[s] {
			case 0:
				visit(s)
			case 1:
				if bodies[s] == nil {
					bodies[s] = map[*mblock]bool{s: true}
					ordered = append(ordered, bodies[s])
				}
				collectLoop(bodies[s], mb, preds)
			}
		}
		state[mb] = 2
	}
	visit(f.blocks[0])
	return ordered
}

func collectLoop(body map[*mblock]bool, latch *mblock, preds map[*mblock][]*mblock) {
	pending := []*mblock{latch}
	for len(pending) > 0 {
		mb := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if body[mb] {
			continue
		}
		body[mb] = true
		pending = append(pending, preds[mb]...)
	}
}

func (f *mfunc) loopDepths() map[*mblock]int {
	depths := map[*mblock]int{}
	for _, body := range f.loopBodies() {
		for mb := range body {
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
