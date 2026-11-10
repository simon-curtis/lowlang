package amd64

import (
	"slices"

	"lowlang/internal/ir"
)

var (
	callerSaved = []string{"rcx", "rsi", "rdi", "r8", "r9"}
	calleeSaved = []string{"rbx", "r12", "r13", "r14", "r15"}
)

func (mb *mblock) terminatorUses() []*vreg {
	uses := append([]*vreg{}, mb.compare...)
	if mb.control != nil {
		uses = append(uses, mb.control)
	}
	return append(uses, mb.results...)
}

func (f *mfunc) liveness() {
	use := map[*mblock]map[*vreg]bool{}
	def := map[*mblock]map[*vreg]bool{}
	for _, mb := range f.blocks {
		u, d := map[*vreg]bool{}, map[*vreg]bool{}
		for _, in := range mb.instrs {
			for _, a := range in.args {
				if !d[a] && a.konst == nil {
					u[a] = true
				}
			}
			if in.dst != nil {
				d[in.dst] = true
			}
		}
		for _, a := range mb.terminatorUses() {
			if !d[a] && a.konst == nil {
				u[a] = true
			}
		}
		use[mb], def[mb] = u, d
		mb.liveIn, mb.liveOut = map[*vreg]bool{}, map[*vreg]bool{}
	}
	for changed := true; changed; {
		changed = false
		for i := len(f.blocks) - 1; i >= 0; i-- {
			if f.blocks[i].propagate(use, def) {
				changed = true
			}
		}
	}
}

func (mb *mblock) propagate(use, def map[*mblock]map[*vreg]bool) bool {
	changed := false
	for _, s := range mb.succs {
		for r := range s.liveIn {
			if !mb.liveOut[r] {
				mb.liveOut[r] = true
				changed = true
			}
		}
	}
	for r := range use[mb] {
		if !mb.liveIn[r] {
			mb.liveIn[r] = true
			changed = true
		}
	}
	for r := range mb.liveOut {
		if !def[mb][r] && !mb.liveIn[r] {
			mb.liveIn[r] = true
			changed = true
		}
	}
	return changed
}

func (r *vreg) extend(pos int) {
	if r.konst != nil {
		return
	}
	if r.start < 0 || pos < r.start {
		r.start = pos
	}
	if pos > r.end {
		r.end = pos
	}
}

func (f *mfunc) intervals() {
	pos := 0
	for _, mb := range f.blocks {
		mb.start = pos
		pos += 2
		for _, in := range mb.instrs {
			in.pos = pos
			pos += 2
		}
		mb.end = pos
		pos += 2
	}
	for _, mb := range f.blocks {
		mb.extendIntervals()
		mb.markCallCrossings()
	}
}

func (in *instr) isCall() bool {
	return !in.move && (in.op == ir.OpCall || in.op == ir.OpRuntime)
}

type segment struct {
	start int
	end   int
}

func (mb *mblock) markCallCrossings() {
	var calls []int
	for _, in := range mb.instrs {
		if in.isCall() {
			calls = append(calls, in.pos)
		}
	}
	if len(calls) == 0 {
		return
	}
	for r, s := range mb.segments() {
		for _, c := range calls {
			if s.start < c && c < s.end {
				r.crossesCall = true
				break
			}
		}
	}
}

func (mb *mblock) segments() map[*vreg]*segment {
	segs := map[*vreg]*segment{}
	touch := func(r *vreg, pos int) {
		if r.konst != nil {
			return
		}
		if s, ok := segs[r]; ok {
			s.end = pos
			return
		}
		segs[r] = &segment{start: pos, end: pos}
	}
	for r := range mb.liveIn {
		touch(r, mb.start)
	}
	for _, in := range mb.instrs {
		for _, a := range in.args {
			touch(a, in.pos)
		}
		if in.dst != nil {
			touch(in.dst, in.pos)
		}
	}
	for _, a := range mb.terminatorUses() {
		touch(a, mb.end)
	}
	for r := range mb.liveOut {
		touch(r, mb.end)
	}
	return segs
}

func (mb *mblock) extendIntervals() {
	for r := range mb.liveIn {
		r.extend(mb.start)
	}
	for r := range mb.liveOut {
		r.extend(mb.end)
	}
	for _, in := range mb.instrs {
		for _, a := range in.args {
			a.extend(in.pos)
		}
		if in.dst != nil {
			in.dst.extend(in.pos)
		}
		if in.isParam() {
			in.dst.extend(mb.start)
		}
	}
	for _, a := range mb.terminatorUses() {
		a.extend(mb.end)
	}
}

func (in *instr) isParam() bool {
	return !in.move && in.op == ir.OpParam
}

type allocator struct {
	active []*vreg
	free   map[string]bool
	spills int
	used   map[string]bool
}

func (f *mfunc) allocate() (spills int, saved []string) {
	f.liveness()
	f.intervals()
	a := &allocator{free: map[string]bool{}, used: map[string]bool{}}
	for _, r := range append(append([]string{}, callerSaved...), calleeSaved...) {
		a.free[r] = true
	}
	order := slices.Clone(f.vregs)
	slices.SortStableFunc(order, func(x, y *vreg) int { return x.start - y.start })
	for _, r := range order {
		if r.start >= 0 {
			a.assign(r)
		}
	}
	for _, r := range calleeSaved {
		if a.used[r] {
			saved = append(saved, r)
		}
	}
	return a.spills, saved
}

func (a *allocator) assign(cur *vreg) {
	a.expire(cur.start)
	pools := [][]string{callerSaved, calleeSaved}
	if cur.crossesCall {
		pools = [][]string{calleeSaved}
	}
	for _, reg := range cur.preferred() {
		if a.free[reg] && slices.ContainsFunc(pools, func(p []string) bool { return slices.Contains(p, reg) }) {
			a.take(cur, reg)
			return
		}
	}
	for _, pool := range pools {
		for _, reg := range pool {
			if a.free[reg] {
				a.take(cur, reg)
				return
			}
		}
	}
	victim := a.victim(cur.crossesCall)
	if victim == nil || victim.end <= cur.end {
		a.spill(cur)
		return
	}
	reg := victim.reg
	victim.reg = ""
	a.spill(victim)
	a.active = slices.DeleteFunc(a.active, func(r *vreg) bool { return r == victim })
	a.take(cur, reg)
}

func (r *vreg) preferred() []string {
	var regs []string
	if r.hint != "" {
		regs = append(regs, r.hint)
	}
	if r.copyOf != nil && r.copyOf.reg != "" {
		regs = append(regs, r.copyOf.reg)
	}
	return regs
}

func (a *allocator) expire(pos int) {
	a.active = slices.DeleteFunc(a.active, func(r *vreg) bool {
		if r.end <= pos {
			a.free[r.reg] = true
			return true
		}
		return false
	})
}

func (a *allocator) victim(calleeOnly bool) *vreg {
	var best *vreg
	for _, r := range a.active {
		if calleeOnly && !slices.Contains(calleeSaved, r.reg) {
			continue
		}
		if best == nil || r.end > best.end {
			best = r
		}
	}
	return best
}

func (a *allocator) take(r *vreg, reg string) {
	r.reg = reg
	a.free[reg] = false
	a.used[reg] = true
	a.active = append(a.active, r)
}

func (a *allocator) spill(r *vreg) {
	r.spill = a.spills
	a.spills++
}
