package ir

import "slices"

type defKey struct {
	variable varKey
	block    int
}

type workspace struct {
	variables  map[defKey]*Value
	sealed     []bool
	incomplete [][]pendingPhi
	reach      []bool
	lattice    []uint8
	uses       []int32

	nonneg   []int8
	defs     []*Value
	visiting []bool

	local       []int32
	escaped     []bool
	slotEscapes []bool
	slotValues  []int32

	slotFirst      []int32
	wordBits       []int16
	slotPromotable []bool

	slotVar    []int32
	slotRemap  []int32
	blockIndex []int32
	chain      []int32
	last       []*Value
	entry      []*Value
	current    []*Value
	phis       [][]*Value
}

const maxRetainedVariables = 1 << 14

func fit[T any](s []T, n int) []T {
	if len(s) >= n {
		return s
	}
	old := len(s)
	s = slices.Grow(s, n-old)[:n]
	clear(s[old:])
	return s
}

func reset[T any](s []T, n int) []T {
	return fit(s[:0], n)
}

func (w *workspace) ssaFor(fn *Func) ssa {
	if w.variables == nil || len(w.variables) > maxRetainedVariables {
		w.variables = map[defKey]*Value{}
	} else {
		clear(w.variables)
	}
	w.sealed = w.sealed[:0]
	clear(w.incomplete)
	w.incomplete = w.incomplete[:0]
	return ssa{fn: fn, ws: w}
}

func newSSA(fn *Func) ssa {
	return (&workspace{}).ssaFor(fn)
}

func (w *workspace) isSealed(b *Block) bool {
	return b.ID < len(w.sealed) && w.sealed[b.ID]
}

func (w *workspace) markSealed(b *Block) {
	w.sealed = fit(w.sealed, b.ID+1)
	w.sealed[b.ID] = true
}

func (w *workspace) addIncomplete(b *Block, p pendingPhi) {
	w.incomplete = fit(w.incomplete, b.ID+1)
	w.incomplete[b.ID] = append(w.incomplete[b.ID], p)
}

func (w *workspace) takeIncomplete(b *Block, i int) (pendingPhi, bool) {
	if b.ID >= len(w.incomplete) || i >= len(w.incomplete[b.ID]) {
		return pendingPhi{}, false
	}
	return w.incomplete[b.ID][i], true
}

func (w *workspace) dropIncomplete(b *Block) {
	if b.ID < len(w.incomplete) {
		w.incomplete[b.ID] = nil
	}
}
