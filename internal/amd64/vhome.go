package amd64

import (
	"math/bits"
	"slices"

	"lowlang/internal/ir"
)

const (
	opSpill ir.Op = -1 - iota
	opFill
)

func vectorOperand(op ir.VectorOp, k int) bool {
	switch op {
	case ir.VSplat:
		return k == 0
	case ir.VShl, ir.VShr:
		return k < 2
	}
	return true
}

func addressOperand(v *ir.Value, k int) bool {
	if v.Op == ir.OpVectorScalar {
		return k == 0
	}
	return vectorOperand(ir.VectorOp(v.Aux), k)
}

type slotScan struct {
	f        *ir.Func
	users    map[*ir.Value][]*ir.Value
	escaping map[*ir.Value]bool
	denied   map[int64]bool
	widths   map[int64]int
}

func registerSlots(f *ir.Func) map[int64]bool {
	s := &slotScan{
		f:        f,
		users:    map[*ir.Value][]*ir.Value{},
		escaping: map[*ir.Value]bool{},
		denied:   map[int64]bool{},
		widths:   map[int64]int{},
	}
	s.index()
	for _, b := range f.Blocks {
		for _, v := range b.Values {
			if v.Op == ir.OpSlot {
				s.scan(v)
			}
		}
	}
	registers := map[int64]bool{}
	for slot, width := range s.widths {
		if !s.denied[slot] && width == 8*f.Slots[slot] {
			registers[slot] = true
		}
	}
	return registers
}

func (s *slotScan) index() {
	for _, b := range s.f.Blocks {
		for _, v := range b.Values {
			for _, a := range v.Args {
				s.users[a] = append(s.users[a], v)
			}
		}
		if b.Control != nil {
			s.escaping[b.Control] = true
		}
		for _, r := range b.Results {
			s.escaping[r] = true
		}
	}
}

func (s *slotScan) scan(slot *ir.Value) {
	if s.escaping[slot] {
		s.denied[slot.Aux] = true
		return
	}
	for _, user := range s.users[slot] {
		if !s.permits(slot, user) {
			s.denied[slot.Aux] = true
			return
		}
	}
}

func (s *slotScan) permits(slot, user *ir.Value) bool {
	switch user.Op {
	case ir.OpVector, ir.OpVectorScalar:
		return s.vectorUse(slot, user)
	case ir.OpAdd:
		return s.laneAddress(slot, user)
	}
	return addresses(user, slot)
}

func (s *slotScan) vectorUse(slot, user *ir.Value) bool {
	t := user.Type
	if user.Op == ir.OpVectorScalar {
		t = user.From
	}
	width := t.VectorBytes()
	for k, a := range user.Args {
		if a != slot {
			continue
		}
		if !addressOperand(user, k) {
			return false
		}
		if known := s.widths[slot.Aux]; known != 0 && known != width {
			return false
		}
		s.widths[slot.Aux] = width
	}
	return true
}

func (s *slotScan) laneAddress(slot, add *ir.Value) bool {
	if len(add.Args) != 2 || s.escaping[add] {
		return false
	}
	other := add.Args[0]
	if other == slot {
		other = add.Args[1]
	}
	if other == slot || other.Op == ir.OpSlot {
		return false
	}
	return !slices.ContainsFunc(s.users[add], func(user *ir.Value) bool { return !addresses(user, add) })
}

func addresses(user, address *ir.Value) bool {
	switch user.Op {
	case ir.OpLoad:
		return user.Args[0] == address
	case ir.OpStore:
		return user.Args[0] == address && user.Args[1] != address
	}
	return false
}

func (l *lowering) operand(v *ir.Value, k int) *vreg {
	arg := v.Args[k]
	if arg.Op == ir.OpSlot && v.Op.IsVector() && l.registers[arg.Aux] && addressOperand(v, k) {
		return l.home(arg)
	}
	return l.vreg(arg)
}

func (l *lowering) home(slot *ir.Value) *vreg {
	r := l.homes[slot.Aux]
	if r == nil {
		r = l.fn.newVreg()
		r.scalar = true
		r.home = slot
		r.wide = l.slots[slot.Aux] == vectorSlotBytes/8
		l.homes[slot.Aux] = r
		l.ordered = append(l.ordered, r)
	}
	return r
}

func (r *vreg) homeWords() int {
	if r.wide {
		return vectorSlotBytes / 8
	}
	return vectorSlotBytes / 16
}

func (in *instr) copiesBetweenHomes() bool {
	return in.op == ir.OpVector && ir.VectorOp(in.value.Aux) == ir.VCopy && in.args[0].home != nil && in.args[1].home != nil
}

func (in *instr) clobbersVectors() bool {
	return in.isCall() || in.preservesRegisters()
}

type accessKind int

const (
	readsRegister accessKind = iota
	writesRegister
	readsHome
	storesPartial
	storesFull
)

func (k accessKind) reads() bool {
	return k == readsRegister || k == readsHome || k == storesPartial
}

func (k accessKind) writes() bool {
	return k == writesRegister || k == storesPartial || k == storesFull
}

type access struct {
	index int
	kind  accessKind
}

type homeState int

const (
	inRegister homeState = iota
	inBoth
	inHome
)

type blockLive struct {
	use  []bool
	def  []bool
	in   []bool
	out  []bool
	full map[*instr]bool
}

type storeRun struct {
	covered uint
	stores  []*instr
}

type transfers struct {
	fn    *mfunc
	homes []*vreg
	slots map[int64]*vreg
	index map[*vreg]int
}

func (l *lowering) insertTransfers() {
	if len(l.ordered) == 0 {
		return
	}
	t := &transfers{fn: l.fn, homes: l.ordered, slots: l.homes, index: map[*vreg]int{}}
	for i, h := range t.homes {
		t.index[h] = i
	}
	t.run()
}

func (t *transfers) run() {
	live := map[*mblock]*blockLive{}
	for _, mb := range t.fn.blocks {
		live[mb] = t.summarise(mb)
	}
	t.propagate(live)
	for _, mb := range t.fn.blocks {
		t.rewrite(mb, live[mb])
	}
}

func (t *transfers) homeOf(address *ir.Value) *vreg {
	if address.Op == ir.OpAdd {
		at := slices.IndexFunc(address.Args, func(a *ir.Value) bool { return a.Op == ir.OpSlot })
		if at < 0 {
			return nil
		}
		address = address.Args[at]
	}
	if address.Op != ir.OpSlot {
		return nil
	}
	return t.slots[address.Aux]
}

func (t *transfers) accesses(in *instr, full map[*instr]bool) []access {
	switch in.op {
	case ir.OpVector, ir.OpVectorScalar:
		return t.vectorAccesses(in)
	case ir.OpLoad:
		if home := t.homeOf(in.value.Args[0]); home != nil {
			return []access{{t.index[home], readsHome}}
		}
	case ir.OpStore:
		if home := t.homeOf(in.value.Args[0]); home != nil {
			if full[in] {
				return []access{{t.index[home], storesFull}}
			}
			return []access{{t.index[home], storesPartial}}
		}
	}
	return nil
}

func (t *transfers) vectorAccesses(in *instr) []access {
	var found []access
	operands := in.args
	if in.op == ir.OpVector {
		operands = in.args[1:]
	}
	for _, a := range operands {
		if a.home != nil {
			found = append(found, access{t.index[a], readsRegister})
		}
	}
	if in.op == ir.OpVector && in.args[0].home != nil {
		found = append(found, access{t.index[in.args[0]], writesRegister})
	}
	return found
}

func (t *transfers) words(i int) int {
	return t.homes[i].homeWords()
}

func (t *transfers) wordStore(in *instr) (index, word int, ok bool) {
	if in.op != ir.OpStore || in.value.Type != nil || in.value.Args[0].Op != ir.OpSlot {
		return 0, 0, false
	}
	home := t.slots[in.value.Args[0].Aux]
	offset := in.value.Aux
	if home == nil || offset < 0 || offset%8 != 0 || int(offset/8) >= t.words(t.index[home]) {
		return 0, 0, false
	}
	return t.index[home], int(offset / 8), true
}

func (t *transfers) fullStores(mb *mblock) map[*instr]bool {
	full := map[*instr]bool{}
	runs := map[int]*storeRun{}
	for _, in := range mb.instrs {
		if in.clobbersVectors() {
			clear(runs)
			continue
		}
		if index, word, ok := t.wordStore(in); ok {
			t.extend(runs, index, word, in, full)
			continue
		}
		for _, a := range t.accesses(in, nil) {
			delete(runs, a.index)
		}
	}
	return full
}

func (t *transfers) extend(runs map[int]*storeRun, index, word int, in *instr, full map[*instr]bool) {
	run := runs[index]
	if run == nil {
		run = &storeRun{}
		runs[index] = run
	}
	run.covered |= 1 << word
	run.stores = append(run.stores, in)
	if bits.OnesCount(run.covered) < t.words(index) {
		return
	}
	for _, s := range run.stores {
		full[s] = true
	}
	delete(runs, index)
}

func (t *transfers) summarise(mb *mblock) *blockLive {
	n := len(t.homes)
	b := &blockLive{
		use:  make([]bool, n),
		def:  make([]bool, n),
		in:   make([]bool, n),
		out:  make([]bool, n),
		full: t.fullStores(mb),
	}
	for _, in := range mb.instrs {
		for _, a := range t.accesses(in, b.full) {
			if a.kind.reads() && !b.def[a.index] {
				b.use[a.index] = true
			}
			if a.kind.writes() {
				b.def[a.index] = true
			}
		}
	}
	return b
}

func (t *transfers) propagate(live map[*mblock]*blockLive) {
	for changed := true; changed; {
		changed = false
		for i := len(t.fn.blocks) - 1; i >= 0; i-- {
			mb := t.fn.blocks[i]
			b := live[mb]
			for k := range t.homes {
				out := slices.ContainsFunc(mb.succs, func(s *mblock) bool { return live[s].in[k] })
				in := b.use[k] || (out && !b.def[k])
				if out != b.out[k] || in != b.in[k] {
					b.out[k], b.in[k] = out, in
					changed = true
				}
			}
		}
	}
}

type blockWalk struct {
	t     *transfers
	live  *blockLive
	after map[*instr][]bool
	state []homeState
	out   []*instr
}

func (t *transfers) rewrite(mb *mblock, live *blockLive) {
	w := &blockWalk{t: t, live: live, state: make([]homeState, len(t.homes))}
	w.after = t.liveAfterClobbers(mb, live)
	for _, in := range mb.instrs {
		w.visit(in)
	}
	w.close()
	mb.instrs = w.out
}

func (t *transfers) liveAfterClobbers(mb *mblock, live *blockLive) map[*instr][]bool {
	after := map[*instr][]bool{}
	current := slices.Clone(live.out)
	for i := len(mb.instrs) - 1; i >= 0; i-- {
		in := mb.instrs[i]
		if in.clobbersVectors() {
			after[in] = slices.Clone(current)
		}
		accesses := t.accesses(in, live.full)
		for _, a := range accesses {
			if a.kind.writes() {
				current[a.index] = false
			}
		}
		for _, a := range accesses {
			if a.kind.reads() {
				current[a.index] = true
			}
		}
	}
	return after
}

func (w *blockWalk) visit(in *instr) {
	if after, clobbers := w.after[in]; clobbers {
		w.park(after)
	} else {
		for _, a := range w.t.accesses(in, w.live.full) {
			w.apply(a)
		}
	}
	w.out = append(w.out, in)
}

func (w *blockWalk) apply(a access) {
	switch a.kind {
	case readsRegister:
		if w.state[a.index] == inHome {
			w.fill(a.index)
		}
	case writesRegister:
		w.state[a.index] = inRegister
	case readsHome:
		if w.state[a.index] == inRegister {
			w.spill(a.index)
		}
	case storesPartial:
		if w.state[a.index] == inRegister {
			w.spill(a.index)
		}
		w.state[a.index] = inHome
	case storesFull:
		w.state[a.index] = inHome
	}
}

func (w *blockWalk) park(after []bool) {
	for i, live := range after {
		if !live {
			continue
		}
		if w.state[i] == inRegister {
			w.spill(i)
		}
		w.state[i] = inHome
	}
}

func (w *blockWalk) close() {
	for i, live := range w.live.out {
		if live && w.state[i] == inHome {
			w.fill(i)
		}
	}
}

func (w *blockWalk) spill(i int) {
	home := w.t.homes[i]
	w.out = append(w.out, &instr{op: opSpill, args: []*vreg{home}, value: home.home})
	w.state[i] = inBoth
}

func (w *blockWalk) fill(i int) {
	home := w.t.homes[i]
	w.out = append(w.out, &instr{op: opFill, dst: home, value: home.home})
	w.state[i] = inBoth
}
