package amd64

import (
	"slices"

	"lowlang/internal/ir"
)

const maxJoinSize = 8

type region map[*mblock]bool

func (f *mfunc) predecessors() map[*mblock][]*mblock {
	preds := map[*mblock][]*mblock{}
	for _, mb := range f.blocks {
		for _, s := range mb.succs {
			preds[s] = append(preds[s], mb)
		}
	}
	return preds
}

func (f *mfunc) reach(from, avoid *mblock) region {
	seen := region{}
	var visit func(mb *mblock)
	visit = func(mb *mblock) {
		if seen[mb] || mb == avoid {
			return
		}
		seen[mb] = true
		for _, s := range mb.succs {
			visit(s)
		}
	}
	visit(from)
	return seen
}

func blockSuccessors(mb *mblock) []*mblock {
	return mb.succs
}

func (f *mfunc) dominators(preds map[*mblock][]*mblock) *ir.Dominators[*mblock] {
	return ir.NewDominators(f.blocks[0], blockSuccessors, func(mb *mblock) []*mblock { return preds[mb] })
}

func (mb *mblock) hasCall() bool {
	return slices.ContainsFunc(mb.instrs, func(in *instr) bool { return in.isCall() })
}

func (f *mfunc) guardBlock(preds map[*mblock][]*mblock) *mblock {
	var calls []*mblock
	for _, mb := range f.blocks {
		if mb.hasCall() {
			calls = append(calls, mb)
		}
	}
	if len(calls) == 0 {
		return nil
	}
	d := f.dominators(preds)
	at := d.Index[calls[0]]
	for _, mb := range calls[1:] {
		at = d.Intersect(at, d.Index[mb])
	}
	if at == 0 {
		return nil
	}
	return d.Nodes[at]
}

func (f *mfunc) shrinkWrap() {
	preds := f.predecessors()
	guard := f.guardBlock(preds)
	if guard == nil {
		return
	}
	heavy := f.reach(guard, nil)
	light := f.reach(f.blocks[0], guard)
	if slices.ContainsFunc(preds[guard], func(p *mblock) bool { return heavy[p] }) {
		return
	}
	var joins []*mblock
	for _, mb := range f.blocks {
		if heavy[mb] && light[mb] {
			joins = append(joins, mb)
		}
	}
	if !lightOnly(light) || !cloneable(joins) {
		return
	}
	f.guard = guard
	f.cloneJoins(joins, light)
	for mb := range light {
		mb.light = !heavy[mb]
	}
	f.splitAtGuard(guard, heavy)
}

func lightOnly(light region) bool {
	for mb := range light {
		if mb.hasCall() || mb.poll {
			return false
		}
	}
	return true
}

func cloneable(joins []*mblock) bool {
	count := 0
	for _, j := range joins {
		count += len(j.instrs) + len(j.copies)
		for _, s := range j.succs {
			if !slices.Contains(joins, s) {
				return false
			}
		}
	}
	return count <= maxJoinSize
}

func (mb *mblock) clone() *mblock {
	c := mb.duplicate(mb.label + "c")
	c.light = true
	return c
}

func (mb *mblock) duplicate(label string) *mblock {
	c := *mb
	c.label = label
	c.instrs = nil
	for _, in := range mb.instrs {
		copied := *in
		copied.args = slices.Clone(in.args)
		c.instrs = append(c.instrs, &copied)
	}
	c.copies = nil
	for _, cp := range mb.copies {
		copied := *cp
		c.copies = append(c.copies, &copied)
	}
	c.succs = slices.Clone(mb.succs)
	c.compare = slices.Clone(mb.compare)
	c.results = slices.Clone(mb.results)
	return &c
}

func (f *mfunc) cloneJoins(joins []*mblock, light region) {
	clones := map[*mblock]*mblock{}
	for _, j := range joins {
		clones[j] = j.clone()
	}
	for _, c := range clones {
		for i, s := range c.succs {
			c.succs[i] = clones[s]
		}
	}
	lastRef := map[*mblock]int{}
	for i, mb := range f.blocks {
		if !light[mb] || clones[mb] != nil {
			continue
		}
		for k, s := range mb.succs {
			if c := clones[s]; c != nil {
				mb.succs[k] = c
				lastRef[c] = i
			}
		}
	}
	var blocks []*mblock
	placed := map[*mblock]bool{}
	for i, mb := range f.blocks {
		blocks = append(blocks, mb)
		for _, j := range joins {
			if idx, ok := lastRef[clones[j]]; ok && idx == i && !placed[clones[j]] {
				c := clones[j]
				blocks = append(blocks, c)
				placed[c] = true
			}
		}
	}
	for _, j := range joins {
		if c := clones[j]; !placed[c] {
			blocks = append(blocks, c)
		}
	}
	f.blocks = blocks
}

func (f *mfunc) splitAtGuard(guard *mblock, heavy region) {
	f.analyse()
	for _, v := range slices.Clone(f.vregs) {
		if !guard.liveIn[v] || !v.crossesCall || v.float {
			continue
		}
		fresh := f.newVreg()
		fresh.scalar = v.scalar
		if f.definedIn(heavy, v) {
			f.renameLight(v, fresh)
			guard.heads = append(guard.heads, &phiCopy{dst: v, src: fresh})
			continue
		}
		for mb := range heavy {
			mb.rename(v, fresh)
		}
		guard.heads = append(guard.heads, &phiCopy{dst: fresh, src: v})
	}
	f.resetAnalysis()
}

func (f *mfunc) analyse() {
	f.number()
	f.liveness()
	f.buildRanges()
	f.markCallCrossings()
}

func (f *mfunc) renameLight(from, to *vreg) {
	for _, mb := range f.blocks {
		if mb.light {
			mb.rename(from, to)
		}
	}
}

func (f *mfunc) resetAnalysis() {
	for _, r := range f.vregs {
		r.ranges = nil
		r.crossesCall = false
	}
}

func (f *mfunc) definedIn(blocks region, v *vreg) bool {
	for mb := range blocks {
		for _, in := range mb.instrs {
			if in.dst == v {
				return true
			}
		}
		for _, c := range mb.copies {
			if c.dst == v {
				return true
			}
		}
	}
	return false
}

func swap(list []*vreg, from, to *vreg) {
	for i, r := range list {
		switch {
		case r == from:
			list[i] = to
		case r.mem != nil && r.mem.uses(from):
			list[i] = r.mem.renamed(from, to)
		}
	}
}

func (mb *mblock) rename(from, to *vreg) {
	for _, in := range mb.instrs {
		swap(in.args, from, to)
		if in.dst == from {
			in.dst = to
		}
	}
	for _, c := range mb.copies {
		if c.src == from {
			c.src = to
		}
		if c.dst == from {
			c.dst = to
		}
	}
	swap(mb.compare, from, to)
	swap(mb.results, from, to)
	if mb.control == from {
		mb.control = to
	}
}

func (f *mfunc) markLight() {
	for _, mb := range f.blocks {
		if !mb.light {
			continue
		}
		for r := range mb.liveIn {
			r.light = true
		}
		for r := range mb.liveOut {
			r.light = true
		}
		for _, st := range mb.steps {
			for _, r := range st.uses {
				r.light = true
			}
			for _, r := range st.defs {
				r.light = true
			}
		}
	}
}

func (mb *mblock) restores() bool {
	return mb.kind == ir.Ret && !mb.light
}
