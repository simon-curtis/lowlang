package amd64

import "lowlang/internal/ir"

func (in *instr) addressesMemory(k int) bool {
	switch in.op {
	case ir.OpLoad, ir.OpStore:
		return k == 0
	case ir.OpVector, ir.OpVectorScalar:
		return addressOperand(in.value, k)
	}
	return false
}

func (f *mfunc) frameAddress(slot *vreg) *instr {
	dst := f.newVreg()
	dst.scalar = slot.scalar
	return &instr{op: ir.OpSlot, dst: dst, value: slot.frame}
}

func (f *mfunc) rematerialize(r *vreg, into *[]*instr) *vreg {
	if r.frame == nil {
		return r
	}
	lea := f.frameAddress(r)
	*into = append(*into, lea)
	return lea.dst
}

func (f *mfunc) rematerializeFrameAddresses() {
	for _, mb := range f.blocks {
		var rebuilt []*instr
		for _, in := range mb.instrs {
			if in.dst != nil && in.dst.frame != nil {
				continue
			}
			for k, arg := range in.args {
				if !in.addressesMemory(k) {
					in.args[k] = f.rematerialize(arg, &rebuilt)
				}
			}
			rebuilt = append(rebuilt, in)
		}
		for i, r := range mb.compare {
			mb.compare[i] = f.rematerialize(r, &rebuilt)
		}
		for i, r := range mb.results {
			mb.results[i] = f.rematerialize(r, &rebuilt)
		}
		for _, c := range mb.copies {
			c.src = f.rematerialize(c.src, &rebuilt)
		}
		if mb.control != nil {
			mb.control = f.rematerialize(mb.control, &rebuilt)
		}
		mb.instrs = rebuilt
	}
}
