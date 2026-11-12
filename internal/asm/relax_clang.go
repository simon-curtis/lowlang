package asm

func (a *assembler) relaxClang(b *secBuilder) {
	a.estimateAddresses(b)
	for a.growUnreachable(b) {
		a.estimateAddresses(b)
	}
}

func (a *assembler) growUnreachable(b *secBuilder) bool {
	grew := false
	for k := range b.frags {
		f := &b.frags[k]
		if f.kind != fragJump || f.form != jumpShort || a.reachesShort(b, f) {
			continue
		}
		f.form = jumpNear
		f.size = f.jumpSize()
		grew = true
	}
	return grew
}

func (a *assembler) reachesShort(b *secBuilder, f *frag) bool {
	symFrag := &b.frags[f.sym.frag]
	target := f.off + symFrag.addr + f.sym.value - int64(symFrag.start)
	aim := target - (f.addr + int64(f.fix))
	return aim < 0 && aim >= shortReachBackward || aim >= 0 && aim <= shortReachForward
}
