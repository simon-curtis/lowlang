package amd64

import "lowlang/internal/ir"

type vectorCache struct {
	eligible map[*ir.Value]bool
	last     map[*ir.Value]*ir.Value
	regs     map[*ir.Value]int
	free     []int
}

type slotUse struct {
	block   *ir.Block
	writes  int
	writer  int
	first   int
	last    int
	lastUse *ir.Value
	spread  bool
}

func vectorTemps(f *ir.Func) *vectorCache {
	uses := map[*ir.Value]*slotUse{}
	tainted := map[*ir.Value]bool{}
	calls := map[*ir.Block][]int{}
	for _, b := range f.Blocks {
		for i, v := range b.Values {
			if v.Op.IsCall() {
				calls[b] = append(calls[b], i)
			}
			vector := v.Op.IsVector()
			for k, a := range v.Args {
				if a.Op != ir.OpSlot {
					continue
				}
				if !vector {
					tainted[a] = true
					continue
				}
				u := uses[a]
				if u == nil {
					u = &slotUse{block: b, writer: -1, first: -1}
					uses[a] = u
				}
				if u.block != b {
					u.spread = true
				}
				if v.Op == ir.OpVector && k == 0 {
					u.writes++
					u.writer = i
				} else if u.first < 0 {
					u.first = i
				}
				u.last = i
				u.lastUse = v
			}
		}
	}
	c := &vectorCache{eligible: map[*ir.Value]bool{}, last: map[*ir.Value]*ir.Value{}}
	for slot, u := range uses {
		if tainted[slot] || u.spread || u.writes != 1 || (u.first >= 0 && u.first < u.writer) {
			continue
		}
		blocked := false
		for _, at := range calls[u.block] {
			if at > u.writer && at < u.last {
				blocked = true
			}
		}
		if !blocked {
			c.eligible[slot] = true
			c.last[slot] = u.lastUse
		}
	}
	return c
}

func (c *vectorCache) reset() {
	c.regs = map[*ir.Value]int{}
	c.free = []int{15, 14, 13, 12, 11, 10, 9, 8}
}

func (g *Generator) source(fr *frame, in *instr, k int, l lanes, base int) {
	if g.cache != nil {
		if r, ok := g.cache.regs[in.value.Args[k]]; ok {
			g.emit("vmovdqa %s, %s", l.reg(base, 0), l.reg(r, 0))
			return
		}
	}
	g.loadVector(l, base, g.vectorAddress(fr, in, k))
}

func (g *Generator) vectorResult(fr *frame, in *instr, l lanes, base int) {
	dst := in.value.Args[0]
	if g.cache != nil && g.cache.eligible[dst] && len(g.cache.free) > 0 {
		r := g.cache.free[len(g.cache.free)-1]
		g.cache.free = g.cache.free[:len(g.cache.free)-1]
		g.cache.regs[dst] = r
		g.emit("vmovdqa %s, %s", l.reg(r, 0), l.reg(base, 0))
		return
	}
	g.storeVector(l, base, g.vectorAddress(fr, in, 0))
}

func (g *Generator) release(in *instr) {
	if g.cache == nil {
		return
	}
	for _, a := range in.value.Args {
		r, ok := g.cache.regs[a]
		if ok && g.cache.last[a] == in.value {
			delete(g.cache.regs, a)
			g.cache.free = append(g.cache.free, r)
		}
	}
}
