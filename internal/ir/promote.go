package ir

import "slices"

const maxPromotionCells = 1 << 18

type promoter struct {
	s     *ssa
	fn    *Func
	ws    *workspace
	vars  int
	undef *Value
}

func (s *ssa) promoteSlots() bool {
	fn := s.fn
	if len(fn.Slots) == 0 || len(fn.Blocks[0].Preds) > 0 {
		return false
	}
	ws := s.ws
	ws.promotableSlots(fn)
	ws.blockIndex = reset(ws.blockIndex, fn.blockIDs)
	for i, b := range fn.Blocks {
		ws.blockIndex[b.ID] = int32(i)
	}
	changed := false
	for slot := 0; slot < len(fn.Slots); {
		ws.slotVar = reset(ws.slotVar, len(fn.Slots))
		for i := range ws.slotVar {
			ws.slotVar[i] = -1
		}
		vars := 0
		for ; slot < len(fn.Slots); slot++ {
			if !ws.slotPromotable[slot] {
				continue
			}
			if (vars+fn.Slots[slot])*len(fn.Blocks) > maxPromotionCells {
				if vars > 0 {
					break
				}
				ws.slotPromotable[slot] = false
				continue
			}
			ws.slotVar[slot] = int32(vars)
			vars += fn.Slots[slot]
		}
		if vars == 0 {
			continue
		}
		p := &promoter{s: s, fn: fn, ws: ws, vars: vars}
		if p.promote() {
			changed = true
		}
	}
	return changed
}

func (p *promoter) variable(v *Value) (int, bool) {
	acc := accessOf(v)
	if acc.root.Op != OpSlot || acc.root.Aux >= int64(len(p.ws.slotVar)) {
		return 0, false
	}
	first := p.ws.slotVar[acc.root.Aux]
	if first < 0 {
		return 0, false
	}
	return int(first) + int(acc.off/8), true
}

func (p *promoter) promote() bool {
	blocks := p.fn.Blocks
	cells := len(blocks) * p.vars
	p.ws.last = reset(p.ws.last, cells)
	p.ws.entry = reset(p.ws.entry, cells)
	p.ws.current = reset(p.ws.current, p.vars)
	p.ws.phis = reset(p.ws.phis, len(blocks))
	for i, b := range blocks {
		p.recordStores(i, b)
	}
	changed := false
	for i, b := range blocks {
		if p.rewrite(i, b) {
			changed = true
		}
	}
	if !changed {
		return false
	}
	p.insertPhis()
	for _, b := range blocks {
		p.s.resolveBlock(b)
	}
	return true
}

func (p *promoter) recordStores(i int, b *Block) {
	for _, v := range b.Values {
		if v.Op != OpStore {
			continue
		}
		if k, ok := p.variable(v); ok {
			p.ws.last[i*p.vars+k] = resolve(v.Args[1])
		}
	}
}

func (p *promoter) rewrite(i int, b *Block) bool {
	current := p.ws.current
	clear(current)
	changed := false
	kept := b.Values[:0]
	for _, v := range b.Values {
		if v.Op != OpLoad && v.Op != OpStore {
			kept = append(kept, v)
			continue
		}
		k, ok := p.variable(v)
		if !ok {
			kept = append(kept, v)
			continue
		}
		changed = true
		if v.Op == OpStore {
			current[k] = resolve(v.Args[1])
			continue
		}
		known := current[k]
		if known == nil {
			known = p.atEntry(k, b)
			current[k] = known
		}
		forwardTo(v, known)
		kept = append(kept, v)
	}
	b.Values = kept
	return changed
}

func (p *promoter) index(b *Block) int {
	return int(p.ws.blockIndex[b.ID])
}

func (p *promoter) atEnd(k int, b *Block) *Value {
	if v := p.ws.last[p.index(b)*p.vars+k]; v != nil {
		return resolve(v)
	}
	return p.atEntry(k, b)
}

func (p *promoter) atEntry(k int, b *Block) *Value {
	base := len(p.ws.chain)
	var found *Value
	for found == nil {
		cell := p.index(b)*p.vars + k
		if v := p.ws.entry[cell]; v != nil {
			found = resolve(v)
			break
		}
		p.ws.chain = append(p.ws.chain, int32(cell))
		switch len(b.Preds) {
		case 0:
			found = p.undefined()
		case 1:
			pred := b.Preds[0]
			if v := p.ws.last[p.index(pred)*p.vars+k]; v != nil {
				found = resolve(v)
			} else {
				b = pred
			}
		default:
			found = p.join(k, b, cell)
		}
	}
	for _, cell := range p.ws.chain[base:] {
		p.ws.entry[cell] = found
	}
	p.ws.chain = p.ws.chain[:base]
	return found
}

func (p *promoter) join(k int, b *Block, cell int) *Value {
	phi := p.fn.newValue(b, OpPhi)
	p.ws.entry[cell] = phi
	i := p.index(b)
	p.ws.phis[i] = append(p.ws.phis[i], phi)
	for _, pred := range b.Preds {
		phi.Args = append(phi.Args, p.atEnd(k, pred))
	}
	result := p.collapse(phi)
	p.ws.entry[cell] = result
	return result
}

func (p *promoter) collapse(phi *Value) *Value {
	same, ok := soleOperand(phi)
	if !ok {
		return phi
	}
	if same == nil {
		same = p.undefined()
	}
	phi.forward = same
	return same
}

func (p *promoter) undefined() *Value {
	if p.undef == nil {
		p.undef = p.fn.newValue(p.fn.Blocks[0], OpUndef)
	}
	return p.undef
}

func (p *promoter) insertPhis() {
	if p.undef != nil {
		entry := p.fn.Blocks[0]
		entry.Values = slices.Insert(entry.Values, 0, p.undef)
	}
	for i, b := range p.fn.Blocks {
		var live []*Value
		for _, phi := range p.ws.phis[i] {
			if phi.forward == nil {
				live = append(live, phi)
			}
		}
		if len(live) > 0 {
			b.Values = slices.Concat(live, b.Values)
		}
	}
}

func (s *ssa) compactSlots() {
	fn := s.fn
	if len(fn.Slots) == 0 {
		return
	}
	remap := reset(s.ws.slotRemap, len(fn.Slots))
	s.ws.slotRemap = remap
	for _, b := range fn.Blocks {
		for _, v := range b.Values {
			if v.Op == OpSlot {
				remap[v.Aux] = 1
			}
		}
	}
	kept := 0
	for i, used := range remap {
		if used == 0 {
			remap[i] = -1
			continue
		}
		remap[i] = int32(kept)
		fn.Slots[kept] = fn.Slots[i]
		kept++
	}
	if kept == len(fn.Slots) {
		return
	}
	fn.Slots = fn.Slots[:kept]
	for _, b := range fn.Blocks {
		for _, v := range b.Values {
			if v.Op == OpSlot {
				v.Aux = int64(remap[v.Aux])
			}
		}
	}
}
