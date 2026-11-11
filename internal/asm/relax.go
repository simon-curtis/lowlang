package asm

const (
	shortReachForward  = 128
	shortReachBackward = -127
)

func (a *assembler) layout(b *secBuilder) {
	b.measureFrags()
	b.classify()
	for a.relaxSegment(b) {
	}
	open := b.openFrag()
	b.Size = open.addr + int64(open.fix)
}

func (a *assembler) relaxSegment(b *secBuilder) bool {
	a.estimateAddresses(b)
	limit := len(b.frags) * len(b.frags)
	for iteration := int32(1); a.relaxPass(b, iteration); iteration++ {
		limit--
		if limit <= 0 {
			a.fail("addresses in section %s do not converge", b.Name)
		}
	}
	changed := false
	for k := range b.frags {
		f := &b.frags[k]
		if f.last != f.addr {
			f.last = f.addr
			changed = true
		}
	}
	return changed
}

func (a *assembler) estimateAddresses(b *secBuilder) {
	var address int64
	var region int32
	for k := range b.frags {
		f := &b.frags[k]
		f.region = region
		f.pass = 0
		f.addr = address
		address += int64(f.fix)
		switch f.kind {
		case fragAlign:
			address += f.alignPadding(address)
			region++
		case fragJump:
			address += int64(a.estimateJump(b, f))
		case fragPadding, fragFused, fragPrefix:
			address += int64(b.padOf(f).length)
		}
	}
}

func (a *assembler) estimateJump(b *secBuilder, f *frag) int32 {
	if f.form == jumpShort && a.mustStayLong(b, f.sym) {
		f.form = jumpForced
		f.size = f.jumpSize()
	}
	return f.size
}

func (a *assembler) mustStayLong(b *secBuilder, sym *symbol) bool {
	return sym.state != symLabel || sym.section != b
}

func (a *assembler) relaxPass(b *secBuilder, iteration int32) bool {
	var stretch int64
	stretched := false
	for k := range b.frags {
		f := &b.frags[k]
		f.pass = iteration
		was := f.addr
		f.addr += stretch
		var growth int64
		switch f.kind {
		case fragAlign:
			growth = a.alignGrowth(f, was)
		case fragJump:
			growth = a.jumpGrowth(b, f, stretch)
		case fragPadding, fragFused:
			growth = a.paddingGrowth(b, f)
		case fragPrefix:
			growth = a.prefixGrowth(b, k, stretch)
		}
		if growth != 0 {
			stretch += growth
			stretched = true
		}
	}
	return stretched
}

func (a *assembler) alignGrowth(f *frag, was int64) int64 {
	fix := int64(f.fix)
	before := f.alignPadding(was + fix)
	after := f.alignPadding(f.addr + fix)
	return after - before
}

func (a *assembler) jumpGrowth(b *secBuilder, f *frag, stretch int64) int64 {
	if f.form != jumpShort {
		return 0
	}
	symFrag := &b.frags[f.sym.frag]
	target := f.off + symFrag.addr + f.sym.value - int64(symFrag.start)
	address := f.addr + int64(f.fix)
	if stretch != 0 && symFrag.pass != f.pass {
		switch {
		case stretch < 0 || symFrag.region == f.region:
			target += stretch
		case target < address:
			return 0
		}
	}
	aim := target - address
	if aim < 0 && aim >= shortReachBackward || aim >= 0 && aim <= shortReachForward {
		return 0
	}
	f.form = jumpNear
	f.size = f.jumpSize()
	return int64(f.size - shortJumpSize)
}
