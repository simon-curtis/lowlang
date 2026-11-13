package amd64

import (
	"math"
	"slices"
	"sort"

	"lowlang/internal/ir"
)

var (
	floatRegs = []string{
		"xmm2", "xmm3", "xmm4", "xmm5", "xmm6", "xmm7", "xmm8",
		"xmm9", "xmm10", "xmm11", "xmm12", "xmm13", "xmm14", "xmm15",
	}
	integerRegs = abi.integers()
)

type segment struct {
	start int
	end   int
}

type step struct {
	pos  int
	uses []*vreg
	defs []*vreg
}

func (r *vreg) fixed() bool {
	return r.konst != nil || r.pool != nil || r.frame != nil || r.mem != nil || r.pinned != ""
}

func (r *vreg) first() int {
	return r.ranges[0].start
}

func (r *vreg) last() int {
	return r.ranges[len(r.ranges)-1].end
}

func (r *vreg) addRange(start, end int) {
	if n := len(r.ranges); n > 0 && r.ranges[n-1].start <= end {
		tail := &r.ranges[n-1]
		tail.start = min(tail.start, start)
		tail.end = max(tail.end, end)
		return
	}
	r.ranges = append(r.ranges, segment{start: start, end: end})
}

func (r *vreg) liveAcross(pos int) bool {
	for _, s := range r.ranges {
		if s.start < pos && pos < s.end {
			return true
		}
	}
	return false
}

func overlaps(a, b []segment) bool {
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		if a[i].start < b[j].end && b[j].start < a[i].end {
			return true
		}
		if a[i].end < b[j].end {
			i++
		} else {
			j++
		}
	}
	return false
}

func (mb *mblock) terminatorUses() []*vreg {
	uses := append([]*vreg{}, mb.compare...)
	if mb.control != nil {
		uses = append(uses, mb.control)
	}
	return append(uses, mb.results...)
}

func registers(regs []*vreg) []*vreg {
	var out []*vreg
	for _, r := range regs {
		switch {
		case r.mem != nil:
			out = append(out, registers(r.mem.pieces())...)
		case !r.fixed():
			out = append(out, r)
		}
	}
	return out
}

func collectable(regs []*vreg) []*vreg {
	var out []*vreg
	for _, r := range regs {
		if !r.scalar {
			out = append(out, r)
		}
	}
	return out
}

func copyStep(pos int, copies []*phiCopy) step {
	st := step{pos: pos}
	for _, c := range copies {
		st.uses = append(st.uses, registers([]*vreg{c.src})...)
		st.defs = append(st.defs, c.dst)
	}
	return st
}

func (f *mfunc) number() {
	pos := 0
	for _, mb := range f.blocks {
		mb.start = pos
		pos += 2
		for _, in := range mb.instrs {
			in.pos = pos
			pos += 2
		}
		mb.copyPos = pos
		pos += 2
		mb.end = pos
		mb.buildSteps()
	}
}

func (mb *mblock) buildSteps() {
	mb.steps = nil
	var params []*vreg
	for _, in := range mb.instrs {
		if in.isParam() {
			params = append(params, in.dst)
		}
	}
	if len(params) > 0 {
		mb.steps = append(mb.steps, step{pos: mb.start, defs: params})
	}
	if len(mb.heads) > 0 {
		mb.steps = append(mb.steps, copyStep(mb.start+1, mb.heads))
	}
	for _, in := range mb.instrs {
		if in.isParam() {
			continue
		}
		st := step{pos: in.pos, uses: registers(in.args)}
		if in.dst != nil {
			st.defs = []*vreg{in.dst}
		}
		mb.steps = append(mb.steps, st)
		if in.op == ir.OpCCall {
			mb.steps = append(mb.steps, step{pos: in.pos + 1, uses: collectable(st.uses)})
		}
	}
	if len(mb.copies) > 0 {
		mb.steps = append(mb.steps, copyStep(mb.copyPos, mb.copies))
	}
	mb.steps = append(mb.steps, step{pos: mb.end, uses: registers(mb.terminatorUses())})
}

func (f *mfunc) liveness() {
	use := map[*mblock]map[*vreg]bool{}
	def := map[*mblock]map[*vreg]bool{}
	for _, mb := range f.blocks {
		u, d := map[*vreg]bool{}, map[*vreg]bool{}
		for _, st := range mb.steps {
			for _, r := range st.uses {
				if !d[r] {
					u[r] = true
				}
			}
			for _, r := range st.defs {
				d[r] = true
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

func (f *mfunc) buildRanges() {
	for _, r := range f.vregs {
		r.ranges = nil
	}
	for i := len(f.blocks) - 1; i >= 0; i-- {
		f.blocks[i].buildRanges()
	}
	for _, r := range f.vregs {
		slices.Reverse(r.ranges)
	}
}

func (mb *mblock) buildRanges() {
	open := map[*vreg]int{}
	for r := range mb.liveOut {
		open[r] = mb.end
	}
	for i := len(mb.steps) - 1; i >= 0; i-- {
		st := mb.steps[i]
		for _, d := range st.defs {
			if end, live := open[d]; live {
				d.addRange(st.pos, end)
				delete(open, d)
				continue
			}
			d.addRange(st.pos, st.pos+1)
		}
		for _, u := range st.uses {
			if _, live := open[u]; !live {
				open[u] = st.pos
			}
		}
	}
	for r, end := range open {
		r.addRange(mb.start, end)
	}
}

func (in *instr) isCall() bool {
	return in.op.IsCall() && in.literal == nil
}

func (in *instr) preservesRegisters() bool {
	return in.op == ir.OpRuntime && in.value.Runtime.Conv == ir.CallGrow
}

func (f *mfunc) markCallCrossings() {
	var clobbering, preserving []int
	for _, mb := range f.blocks {
		for _, in := range mb.instrs {
			switch {
			case in.preservesRegisters():
				preserving = append(preserving, in.pos)
			case in.isCall():
				clobbering = append(clobbering, in.pos)
			}
		}
	}
	for _, r := range f.vregs {
		r.crossesPreserving = r.crossesAny(preserving)
		r.crossesCall = r.crossesAny(clobbering) || (r.crossesPreserving && !r.scalar)
	}
}

func (r *vreg) crossesAny(calls []int) bool {
	if len(calls) == 0 {
		return false
	}
	for _, s := range r.ranges {
		i := sort.SearchInts(calls, s.start+1)
		if i < len(calls) && calls[i] < s.end {
			return true
		}
	}
	return false
}

func (in *instr) isParam() bool {
	return in.op == ir.OpParam
}

type allocator struct {
	assigned map[string][]*vreg
	slots    [][]*vreg
	used     map[string]bool
	cost     func(*vreg) float64
}

type placement struct {
	reg   string
	spill int
}

type outcome struct {
	alloc  *allocator
	placed []placement
	cost   float64
}

var spillCosts = []func(*vreg) float64{
	func(r *vreg) float64 { return float64(r.weight) },
	func(r *vreg) float64 { return float64(r.weight) / float64(r.length) },
	func(r *vreg) float64 { return float64(r.weight) / math.Sqrt(float64(r.length)) },
}

func (r *vreg) measure() {
	r.length = 0
	for _, s := range r.ranges {
		r.length += s.end - s.start
	}
}

func (f *mfunc) allocationOrder() []*vreg {
	var order []*vreg
	for _, r := range f.vregs {
		if len(r.ranges) > 0 {
			r.measure()
			order = append(order, r)
		}
	}
	slices.SortStableFunc(order, func(x, y *vreg) int { return x.first() - y.first() })
	return order
}

func (f *mfunc) place(order []*vreg, cost func(*vreg) float64) outcome {
	for _, r := range order {
		r.reg, r.spill = "", -1
	}
	a := &allocator{assigned: map[string][]*vreg{}, used: map[string]bool{}, cost: cost}
	for _, r := range order {
		a.assign(r)
	}
	o := outcome{alloc: a, placed: make([]placement, len(order))}
	for i, r := range order {
		o.placed[i] = placement{reg: r.reg, spill: r.spill}
		if r.reg == "" {
			o.cost += float64(r.weight)
		}
	}
	for _, reg := range abi.saved {
		if a.used[reg] {
			o.cost += saveCost
		}
	}
	return o
}

func (f *mfunc) allocate() (spills int, saved []string) {
	f.analyse()
	f.markLight()
	f.weigh()
	order := f.allocationOrder()
	var best outcome
	for i, cost := range spillCosts {
		if o := f.place(order, cost); i == 0 || o.cost < best.cost {
			best = o
		}
	}
	for i, r := range order {
		r.reg, r.spill = best.placed[i].reg, best.placed[i].spill
	}
	for _, r := range abi.saved {
		if best.alloc.used[r] {
			saved = append(saved, r)
		}
	}
	return len(best.alloc.slots), saved
}

func (a *allocator) candidates(cur *vreg) []string {
	switch {
	case cur.float:
		return floatRegs
	case cur.crossesCall && cur.light:
		return nil
	case cur.crossesCall:
		return abi.saved
	case cur.light:
		return abi.caller
	}
	return integerRegs
}

func (a *allocator) assign(cur *vreg) {
	a.expire(cur.first())
	if cur.float && (cur.crossesCall || cur.crossesPreserving) {
		a.spill(cur)
		return
	}
	regs := a.candidates(cur)
	for _, reg := range cur.preferred() {
		if slices.Contains(regs, reg) && a.fits(cur, reg) {
			a.take(cur, reg)
			return
		}
	}
	if reg := a.firstFit(cur, regs); reg != "" {
		a.take(cur, reg)
		return
	}
	a.displace(cur, regs)
}

func (a *allocator) fits(cur *vreg, reg string) bool {
	return len(a.conflicts(cur, reg)) == 0
}

func (a *allocator) conflicts(cur *vreg, reg string) []*vreg {
	var found []*vreg
	for _, r := range a.assigned[reg] {
		if overlaps(cur.ranges, r.ranges) {
			found = append(found, r)
		}
	}
	return found
}

func (a *allocator) firstFit(cur *vreg, regs []string) string {
	fallback := ""
	for _, reg := range regs {
		switch {
		case !a.fits(cur, reg):
		case cur.avoids(reg):
			fallback = reg
		default:
			return reg
		}
	}
	return fallback
}

func (r *vreg) avoids(reg string) bool {
	return r.avoid != nil && r.avoid.reg == reg
}

func (a *allocator) cheaperToSpill(r, other *vreg) bool {
	if x, y := a.cost(r), a.cost(other); x != y {
		return x < y
	}
	return r.last() > other.last()
}

func (a *allocator) displace(cur *vreg, regs []string) {
	var victim *vreg
	for _, reg := range regs {
		found := a.conflicts(cur, reg)
		if len(found) != 1 {
			continue
		}
		if victim == nil || a.cheaperToSpill(found[0], victim) {
			victim = found[0]
		}
	}
	if victim == nil || !a.cheaperToSpill(victim, cur) {
		a.spill(cur)
		return
	}
	reg := victim.reg
	a.assigned[reg] = slices.DeleteFunc(a.assigned[reg], func(r *vreg) bool { return r == victim })
	victim.reg = ""
	a.spill(victim)
	a.take(cur, reg)
}

func (r *vreg) preferred() []string {
	var regs []string
	if r.hint != "" {
		regs = append(regs, r.hint)
	}
	for _, s := range r.sources {
		if s.reg != "" {
			regs = append(regs, s.reg)
		}
	}
	for _, s := range r.sinks {
		if s.reg != "" {
			regs = append(regs, s.reg)
		}
	}
	return regs
}

func (a *allocator) expire(pos int) {
	for reg, list := range a.assigned {
		a.assigned[reg] = slices.DeleteFunc(list, func(r *vreg) bool { return r.last() <= pos })
	}
}

func (a *allocator) take(r *vreg, reg string) {
	r.reg = reg
	a.used[reg] = true
	a.assigned[reg] = append(a.assigned[reg], r)
}

func (a *allocator) spill(r *vreg) {
	slot := a.sharedSlot(r)
	if slot < 0 {
		slot = len(a.slots)
		a.slots = append(a.slots, nil)
	}
	r.spill = slot
	a.slots[slot] = append(a.slots[slot], r)
}

func (a *allocator) sharedSlot(r *vreg) int {
	for _, related := range r.copyPeers {
		if related.spill < 0 || related.reg != "" {
			continue
		}
		free := !slices.ContainsFunc(a.slots[related.spill], func(occupant *vreg) bool {
			return overlaps(occupant.ranges, r.ranges)
		})
		if free {
			return related.spill
		}
	}
	return -1
}
