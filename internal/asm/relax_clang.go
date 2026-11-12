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
		if f.kind != fragJump || f.form != jumpShort || fitsShort(a.aim(f)) {
			continue
		}
		f.form = jumpNear
		f.size = f.jumpSize()
		grew = true
	}
	return grew
}
