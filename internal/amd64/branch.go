package amd64

import (
	"slices"

	"lowlang/internal/ir"
)

const maxThreadingRounds = 4

func (f *mfunc) simplifyBranches() {
	for round := 0; round < maxThreadingRounds && f.threadBranches(); round++ {
	}
	f.peelNegations()
}

func (f *mfunc) threadBranches() bool {
	s := newSelector(f)
	preds := f.predecessors()
	threaded := false
	for _, b := range slices.Clone(f.blocks) {
		if !s.threadable(b) {
			continue
		}
		for _, m := range slices.Clone(preds[b]) {
			if f.threadPredecessor(preds, m, b) {
				threaded = true
				preds = f.predecessors()
			}
		}
	}
	if threaded {
		f.dropUnreachable()
	}
	return threaded
}

func (s *selector) threadable(b *mblock) bool {
	if b.kind != ir.If || b.fused != nil || b.control == nil || b.poll || b.control.fixed() {
		return false
	}
	if len(b.instrs) != 0 || len(b.heads) != 0 || len(b.copies) != 0 {
		return false
	}
	return s.uses[b.control] == 1 && s.defs[b.control] == nil
}

func (f *mfunc) threadPredecessor(preds map[*mblock][]*mblock, m, b *mblock) bool {
	if m == b || m.kind != ir.Plain || len(m.succs) != 1 || m.succs[0] != b || len(m.heads) != 0 || len(m.copies) != 1 {
		return false
	}
	c := m.copies[0]
	if c.dst != b.control {
		return false
	}
	m.copies = nil
	if taken, known := knownTruth(preds, m, c.src); known {
		m.succs = []*mblock{b.succs[taken]}
		return true
	}
	m.kind = ir.If
	m.control = c.src
	m.succs = slices.Clone(b.succs)
	return true
}

func knownTruth(preds map[*mblock][]*mblock, m *mblock, r *vreg) (int, bool) {
	if r.konst != nil {
		if *r.konst != 0 {
			return 0, true
		}
		return 1, true
	}
	from := preds[m]
	if len(from) != 1 {
		return 0, false
	}
	p := from[0]
	if p.kind != ir.If || p.fused != nil || p.control != r || len(p.succs) != 2 || p.succs[0] == p.succs[1] {
		return 0, false
	}
	return slices.Index(p.succs, m), true
}

func (f *mfunc) dropUnreachable() {
	reached := map[*mblock]bool{}
	var visit func(mb *mblock)
	visit = func(mb *mblock) {
		if reached[mb] {
			return
		}
		reached[mb] = true
		for _, s := range mb.succs {
			visit(s)
		}
	}
	visit(f.blocks[0])
	f.blocks = slices.DeleteFunc(f.blocks, func(mb *mblock) bool { return !reached[mb] })
}

func (f *mfunc) peelNegations() {
	s := newSelector(f)
	for _, mb := range f.blocks {
		if mb.kind == ir.If && mb.fused == nil && mb.control != nil {
			mb.peelControl(s)
		}
	}
}

func (mb *mblock) peelControl(s *selector) {
	for len(mb.instrs) > 0 {
		last := mb.instrs[len(mb.instrs)-1]
		if last.dst != mb.control || s.uses[mb.control] != 1 {
			return
		}
		switch {
		case last.op == ir.OpNot:
			mb.control = last.args[0]
			mb.succs[0], mb.succs[1] = mb.succs[1], mb.succs[0]
		case last.op.Compare():
			mb.fused = last.value
			mb.compare = slices.Clone(last.args)
			mb.control = nil
		default:
			return
		}
		mb.instrs = mb.instrs[:len(mb.instrs)-1]
		if mb.fused != nil {
			return
		}
	}
}

func (s *selector) negateCompares() {
	for _, mb := range s.f.blocks {
		local := map[*vreg]*instr{}
		for _, in := range mb.instrs {
			if in.dst != nil {
				local[in.dst] = in
			}
			if in.op == ir.OpNot {
				s.negate(in, local[in.args[0]])
			}
		}
	}
}

func (s *selector) negate(not, compare *instr) {
	if compare == nil || !compare.op.Compare() || s.uses[not.args[0]] != 1 {
		return
	}
	compare.dst = not.dst
	compare.negated = !compare.negated
	s.defs[not.dst] = compare
	s.dead[not] = true
}
