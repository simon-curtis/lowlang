package ir

type access struct {
	root *Value
	off  int64
	size int8
}

func base(v *Value) (*Value, int64) {
	var off int64
	for v.Op == OpAdd && v.Type == nil && len(v.Args) == 2 && v.Args[1].Op == OpConst {
		off += v.Args[1].Aux
		v = resolve(v.Args[0])
	}
	return v, off
}

func accessOf(v *Value) access {
	root, off := base(resolve(v.Args[0]))
	size := int8(8)
	if t := v.Type; t != nil && (t.Bits == 8 || t.Bits == 16 || t.Bits == 32) {
		size = int8(t.Bits / 8)
	}
	return access{root: root, off: off + v.Aux, size: size}
}

func isAlloc(v *Value) bool {
	return v.Op == OpResult && v.Aux == 0 && v.Args[0].Op == OpRuntime && v.Args[0].Str == "alloc"
}

func isRootCandidate(v *Value) bool {
	return v.Op == OpSlot || isAlloc(v)
}

func addressOnlyUse(user *Value, arg int) bool {
	switch {
	case user.Op == OpLoad, user.Op == OpStore:
		return arg == 0
	case user.Op.Compare():
		return true
	case user.Op == OpAdd:
		return arg == 0 && user.Type == nil && len(user.Args) == 2 && user.Args[1].Op == OpConst
	}
	return false
}

func (w *workspace) localRoots(fn *Func) {
	size := fn.values + 1
	w.local = reset(w.local, size)
	w.escaped = reset(w.escaped, size)
	w.slotEscapes = reset(w.slotEscapes, len(fn.Slots))
	w.slotValues = reset(w.slotValues, len(fn.Slots))
	for _, b := range fn.Blocks {
		for _, v := range b.Values {
			if v.Op == OpSlot {
				w.slotValues[v.Aux]++
			}
			for i, a := range v.Args {
				if !addressOnlyUse(v, i) {
					w.escape(a)
				}
			}
		}
		if b.Control != nil {
			w.escape(b.Control)
		}
		for _, r := range b.Results {
			w.escape(r)
		}
	}
	roots := int32(0)
	for _, b := range fn.Blocks {
		for _, v := range b.Values {
			if !isRootCandidate(v) {
				continue
			}
			escapes := w.escaped[v.ID] || (v.Op == OpSlot && w.slotValues[v.Aux] > 1)
			if v.Op == OpSlot {
				w.slotEscapes[v.Aux] = w.slotEscapes[v.Aux] || escapes
			}
			if !escapes {
				roots++
				w.local[v.ID] = roots
			}
		}
	}
}

func (w *workspace) escape(v *Value) {
	if root, _ := base(v); isRootCandidate(root) {
		w.escaped[root.ID] = true
	}
}

func (w *workspace) promotableSlots(fn *Func) {
	w.localRoots(fn)
	w.slotFirst = reset(w.slotFirst, len(fn.Slots)+1)
	words := 0
	for i, n := range fn.Slots {
		w.slotFirst[i] = int32(words)
		words += n
	}
	w.slotFirst[len(fn.Slots)] = int32(words)
	w.wordBits = reset(w.wordBits, words)
	w.slotPromotable = reset(w.slotPromotable, len(fn.Slots))
	for i := range w.slotPromotable {
		w.slotPromotable[i] = !w.slotEscapes[i]
	}
	for _, b := range fn.Blocks {
		for _, v := range b.Values {
			if v.Op == OpLoad || v.Op == OpStore {
				w.checkSlotAccess(fn, v)
			}
		}
	}
}

func (w *workspace) checkSlotAccess(fn *Func, v *Value) {
	acc := accessOf(v)
	if acc.root.Op != OpSlot {
		return
	}
	slot := acc.root.Aux
	if !w.slotPromotable[slot] {
		return
	}
	if v.Type != nil || v.Mem&MemRaw != 0 || acc.off < 0 || acc.off%8 != 0 || acc.off/8 >= int64(fn.Slots[slot]) {
		w.slotPromotable[slot] = false
		return
	}
	bits := v.FloatBits
	if v.Op == OpStore {
		stored := resolve(v.Args[1])
		if stored.Op == OpPhi || stored.Op == OpUndef {
			return
		}
		bits = stored.FloatBits
	}
	word := w.slotFirst[slot] + int32(acc.off/8)
	switch seen := w.wordBits[word]; {
	case seen == 0:
		w.wordBits[word] = int16(bits) + 1
	case int(seen) != bits+1:
		w.slotPromotable[slot] = false
	}
}
