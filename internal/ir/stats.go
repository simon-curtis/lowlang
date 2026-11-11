package ir

import (
	"fmt"
	"strings"

	"lowlang/internal/types"
)

type Stats struct {
	Funcs int

	Loads                 int
	RedundantLoads        int
	RedundantAcrossCalls  int
	Slots                 int
	PromotableSlots       int
	ChecksAffine          int
	ChecksLoadedLimit     int
	ChecksOther           int
	AllocsOneWord         int
	AllocsTwoToFourWords  int
	AllocsFiveToThirtyTwo int
	AllocsLarger          int
	AllocsVariable        int
	AllocsScanned         int
	AllocsNeverEscaping   int
}

type availableLoad struct {
	root *Value
	off  int64
	size int8
	ty   *types.Type
	bits int
}

func (p *Program) MemoryStats() Stats {
	var s Stats
	ws := &workspace{}
	for _, f := range p.Funcs {
		s.Funcs++
		s.countLoads(f)
		s.countSlots(ws, f)
		s.countChecks(f)
		s.countAllocs(ws, f)
	}
	return s
}

func (s *Stats) countLoads(f *Func) {
	for _, b := range f.Blocks {
		for _, v := range b.Values {
			if v.Op == OpLoad {
				s.Loads++
			}
		}
	}
	s.RedundantLoads += redundantLoads(f, true)
	s.RedundantAcrossCalls += redundantLoads(f, false)
}

func redundantLoads(f *Func, callsClobber bool) int {
	doms := newDominators(f)
	ends := make([][]availableLoad, len(doms.Nodes))
	redundant := 0
	for i, b := range doms.Nodes {
		var state []availableLoad
		if len(b.Preds) == 1 {
			if p, ok := doms.Index[b.Preds[0]]; ok && p < i {
				state = append(state, ends[p]...)
			}
		}
		for _, v := range b.Values {
			switch v.Op {
			case OpLoad:
				if v.Mem&MemTrap != 0 {
					continue
				}
				key := availability(v)
				if containsLoad(state, key) {
					redundant++
					continue
				}
				state = append(state, key)
			case OpStore:
				state = state[:0]
				if v.Type == nil {
					state = append(state, availability(v))
				}
			case OpCall, OpCCall, OpRuntime, OpVector:
				if callsClobber {
					state = state[:0]
				}
			}
		}
		ends[i] = state
	}
	return redundant
}

func availability(v *Value) availableLoad {
	acc := accessOf(v)
	bits := v.FloatBits
	if v.Op == OpStore {
		bits = resolve(v.Args[1]).FloatBits
	}
	return availableLoad{root: acc.root, off: acc.off, size: acc.size, ty: v.Type, bits: bits}
}

func containsLoad(state []availableLoad, key availableLoad) bool {
	for _, have := range state {
		if have == key {
			return true
		}
	}
	return false
}

func (s *Stats) countSlots(ws *workspace, f *Func) {
	s.Slots += len(f.Slots)
	if len(f.Slots) == 0 {
		return
	}
	ws.promotableSlots(f)
	for _, ok := range ws.slotPromotable {
		if ok {
			s.PromotableSlots++
		}
	}
}

func (s *Stats) countChecks(f *Func) {
	loops := findLoops(newDominators(f))
	if len(loops) == 0 {
		return
	}
	claimed := map[*Value]bool{}
	for _, l := range loops {
		for _, b := range l.order {
			for _, v := range b.Values {
				if v.Op != OpCheck || claimed[v] {
					continue
				}
				claimed[v] = true
				switch {
				case l.invariant(v.Args[1]) && affineIndex(l, v.Args[0]):
					s.ChecksAffine++
				case v.Args[1].Op == OpLoad:
					s.ChecksLoadedLimit++
				default:
					s.ChecksOther++
				}
			}
		}
	}
}

func affineIndex(l *loop, v *Value) bool {
	if v.Op == OpAdd && len(v.Args) == 2 && v.Args[1].Op == OpConst {
		v = v.Args[0]
	}
	if v.Op != OpPhi || v.Block != l.header {
		return false
	}
	_, ok := basicInduction(l, v)
	return ok
}

func (s *Stats) countAllocs(ws *workspace, f *Func) {
	ws.localRoots(f)
	for _, b := range f.Blocks {
		for _, v := range b.Values {
			if !isAlloc(v) {
				continue
			}
			words := v.Args[0].Args[0]
			switch {
			case words.Op != OpConst:
				s.AllocsVariable++
			case words.Aux <= 1:
				s.AllocsOneWord++
			case words.Aux <= 4:
				s.AllocsTwoToFourWords++
			case words.Aux <= 32:
				s.AllocsFiveToThirtyTwo++
			default:
				s.AllocsLarger++
			}
			if scan := v.Args[0].Args[1]; scan.Op != OpConst || scan.Aux != 0 {
				s.AllocsScanned++
			}
			if _, local := ws.localIndex(v); local {
				s.AllocsNeverEscaping++
			}
		}
	}
}

func (s Stats) Add(o Stats) Stats {
	return Stats{
		Funcs:                 s.Funcs + o.Funcs,
		Loads:                 s.Loads + o.Loads,
		RedundantLoads:        s.RedundantLoads + o.RedundantLoads,
		RedundantAcrossCalls:  s.RedundantAcrossCalls + o.RedundantAcrossCalls,
		Slots:                 s.Slots + o.Slots,
		PromotableSlots:       s.PromotableSlots + o.PromotableSlots,
		ChecksAffine:          s.ChecksAffine + o.ChecksAffine,
		ChecksLoadedLimit:     s.ChecksLoadedLimit + o.ChecksLoadedLimit,
		ChecksOther:           s.ChecksOther + o.ChecksOther,
		AllocsOneWord:         s.AllocsOneWord + o.AllocsOneWord,
		AllocsTwoToFourWords:  s.AllocsTwoToFourWords + o.AllocsTwoToFourWords,
		AllocsFiveToThirtyTwo: s.AllocsFiveToThirtyTwo + o.AllocsFiveToThirtyTwo,
		AllocsLarger:          s.AllocsLarger + o.AllocsLarger,
		AllocsVariable:        s.AllocsVariable + o.AllocsVariable,
		AllocsScanned:         s.AllocsScanned + o.AllocsScanned,
		AllocsNeverEscaping:   s.AllocsNeverEscaping + o.AllocsNeverEscaping,
	}
}

func (s Stats) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "funcs %d\n", s.Funcs)
	fmt.Fprintf(&b, "loads %d, redundant %d (%s), redundant if calls keep memory %d (%s)\n",
		s.Loads, s.RedundantLoads, percent(s.RedundantLoads, s.Loads), s.RedundantAcrossCalls, percent(s.RedundantAcrossCalls, s.Loads))
	fmt.Fprintf(&b, "slots %d, promotable %d\n", s.Slots, s.PromotableSlots)
	fmt.Fprintf(&b, "checks in loops: affine with invariant limit %d, loaded limit %d, other %d\n",
		s.ChecksAffine, s.ChecksLoadedLimit, s.ChecksOther)
	fmt.Fprintf(&b, "alloc sites: 1 word %d, 2-4 words %d, 5-32 words %d, larger %d, variable %d, scanned %d, never escaping %d\n",
		s.AllocsOneWord, s.AllocsTwoToFourWords, s.AllocsFiveToThirtyTwo, s.AllocsLarger, s.AllocsVariable, s.AllocsScanned, s.AllocsNeverEscaping)
	return b.String()
}

func percent(n, of int) string {
	if of == 0 {
		return "0.0%"
	}
	return fmt.Sprintf("%.1f%%", 100*float64(n)/float64(of))
}
