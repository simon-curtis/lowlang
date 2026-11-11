package asm

const (
	branchBoundary   = 32
	branchPaddingMax = 14
	fusedPaddingMax  = 20
	prefixPaddingMax = 5
	csPrefix         = 0x2E
	noLink           = -1
)

type padState struct {
	address       int64
	link          int32
	mf            uint8
	maxBytes      uint8
	maxPrefix     uint8
	cmpSize       uint8
	length        uint8
	lastLength    uint8
	prefixLength  uint8
	defaultPrefix byte
}

func (b *secBuilder) addPad(kind fragKind, mf uint8, max uint8) int32 {
	f := b.openFrag()
	f.kind = kind
	f.pad = int32(len(b.pads))
	b.pads = append(b.pads, padState{link: noLink, mf: mf, maxBytes: max})
	b.startFrag()
	return b.currentFrag() - 1
}

func (b *secBuilder) padOf(f *frag) *padState {
	return &b.pads[f.pad]
}

func (b *secBuilder) endInstruction() {
	if int32(len(b.Data)) > b.openFrag().start {
		b.startFrag()
	}
}

func (a *assembler) addBranchPadding(opcode byte) {
	b := a.cur
	if !a.opts.AlignBranches || b.last != lastOther {
		return
	}
	mf := (opcode & 0x0E) >> 1
	b.addPad(fragPadding, mf, branchPaddingMax)
	b.raiseAlign(branchBoundary)
}

func (a *assembler) addInstructionPadding(f *form, ops []Operand) int32 {
	b := a.cur
	if !a.opts.AlignBranches || f.branch || b.last != lastOther {
		return noLink
	}
	if class := f.fusion(ops); class != fuseNone {
		return b.addPad(fragFused, uint8(class), fusedPaddingMax)
	}
	if len(ops) > 0 {
		return b.addPad(fragPrefix, 0, prefixPaddingMax)
	}
	return noLink
}

func (f *form) fusion(ops []Operand) fuseClass {
	if f.fuse == fuseNone {
		return fuseNone
	}
	var mem, imm bool
	for i := range ops {
		switch ops[i].Kind {
		case OpMem:
			if ops[i].Base == regRIP {
				return fuseNone
			}
			mem = true
		case OpImm:
			imm = true
		}
	}
	if mem && (imm || f.fuse == fuseIncDec) {
		return fuseNone
	}
	return f.fuse
}

func (b *secBuilder) reservePrefixRoom(k int32, e *encoding) {
	f := &b.frags[k]
	st := b.padOf(f)
	room := maxInstructionLength - e.length
	limit := min(prefixPaddingMax, room)
	count := e.prefixes
	switch f.kind {
	case fragPrefix:
		st.maxBytes = 0
		if limit > count {
			st.maxBytes = uint8(limit - count)
		}
	default:
		if limit > count {
			st.maxPrefix = uint8(limit - count)
		}
	}
	st.defaultPrefix = csPrefix
	if e.segment != 0 {
		st.defaultPrefix = e.segment
	}
}

func fusible(cmp fuseClass, jcc uint8) bool {
	const (
		jc  = 1
		jna = 3
		je  = 2
		jl  = 6
		jle = 7
	)
	switch cmp {
	case fuseAluCmp:
		return jcc >= jc && jcc <= jna || jcc == jl || jcc == jle
	case fuseIncDec:
		return jcc == je || jcc == jl || jcc == jle
	case fuseTestAnd:
		return true
	}
	return false
}

func (b *secBuilder) nextNonEmpty(k int) int {
	for k++; k < len(b.frags); k++ {
		f := &b.frags[k]
		if f.kind != fragFixed || f.fix != 0 {
			break
		}
	}
	return k
}

func (b *secBuilder) classify() {
	if len(b.pads) == 0 {
		return
	}
	for k := 0; k < len(b.frags); k++ {
		switch b.frags[k].kind {
		case fragPadding:
			b.padOf(&b.frags[k]).link = int32(b.nextNonEmpty(k))
		case fragFused:
			k = b.classifyFused(k)
		}
	}
	b.classifyPrefixes()
}

func (b *secBuilder) classifyFused(k int) int {
	f := &b.frags[k]
	st := b.padOf(f)
	cmp := b.nextNonEmpty(k)
	pad := b.nextNonEmpty(cmp)
	branch := b.fusibleBranch(k, pad)
	switch {
	case branch >= 0:
		st.link = int32(branch)
		st.cmpSize = uint8(b.frags[cmp].fix)
		b.frags[pad].kind = fragFixed
		return branch
	case st.maxPrefix != 0:
		f.kind = fragPrefix
		st.maxBytes = st.maxPrefix
		st.maxPrefix = 0
	default:
		f.kind = fragFixed
	}
	return k
}

func (b *secBuilder) fusibleBranch(fused, pad int) int {
	if pad >= len(b.frags) || b.frags[pad].kind != fragPadding {
		return noLink
	}
	branch := b.nextNonEmpty(pad)
	if branch >= len(b.frags) {
		return noLink
	}
	target := &b.frags[branch]
	if target.kind != fragJump || !target.cond {
		return noLink
	}
	cmp := fuseClass(b.padOf(&b.frags[fused]).mf)
	if !fusible(cmp, b.padOf(&b.frags[pad]).mf) {
		return noLink
	}
	return branch
}

func (b *secBuilder) classifyPrefixes() {
	for k := 0; k < len(b.frags); k++ {
		if b.frags[k].kind != fragPrefix {
			continue
		}
		k = b.classifyPrefixRun(k)
	}
}

func (b *secBuilder) classifyPrefixRun(first int) int {
	var total int
	n := first
	for ; n < len(b.frags); n++ {
		g := &b.frags[n]
		if g.kind == fragFixed {
			continue
		}
		if g.kind == fragPadding || g.kind == fragFused {
			b.padOf(&b.frags[first]).link = int32(n)
			break
		}
		if g.kind != fragPrefix {
			break
		}
		if total >= fusedPaddingMax {
			total = fusedPaddingMax
			g.kind = fragFixed
			continue
		}
		total += int(b.padOf(g).maxBytes)
	}
	b.padOf(&b.frags[first]).maxPrefix = uint8(total)
	return n
}

func (b *secBuilder) paddingSize(f *frag, address int64) int64 {
	st := b.padOf(f)
	if address == 0 {
		address = f.addr
	}
	address += int64(f.fix)
	branch := &b.frags[st.link]
	size := int64(st.cmpSize) + int64(branch.fix)
	if branch.kind == fragJump {
		size += int64(branch.size)
	}
	offset := address & (branchBoundary - 1)
	if offset+size >= branchBoundary {
		return branchBoundary - offset
	}
	return 0
}

func (a *assembler) paddingGrowth(b *secBuilder, f *frag) int64 {
	st := b.padOf(f)
	size := b.paddingSize(f, 0)
	grow := size - int64(st.length)
	if st.address != 0 && (st.address != f.addr || size != 0) {
		a.line = int(f.line)
		a.fail("branch padding in section %s does not converge", b.Name)
	}
	if grow != 0 {
		st.length = uint8(size)
	}
	return grow
}

func (a *assembler) prefixGrowth(b *secBuilder, k int, stretch int64) int64 {
	f := &b.frags[k]
	st := b.padOf(f)
	if st.link == noLink {
		return int64(st.length) - int64(st.lastLength)
	}
	padding := &b.frags[st.link]
	paddingState := b.padOf(padding)
	if st.address == 0 {
		st.address = padding.addr - (f.addr - stretch)
	}
	b.distribute(k, int(st.link), int(st.prefixLength), true)
	size := b.paddingSize(padding, f.addr+st.address)
	if size == int64(st.prefixLength) {
		paddingState.address = f.addr + size + st.address
		return 0
	}
	if size > int64(st.maxPrefix) {
		paddingState.address = 0
		size = 0
	} else {
		paddingState.address = f.addr + size + st.address
	}
	st.prefixLength = uint8(size)
	b.distribute(k, int(st.link), int(size), false)
	return int64(st.length) - int64(st.lastLength)
}

func (b *secBuilder) distribute(first, end, total int, last bool) {
	left := total
	for n := first; n < end; n++ {
		g := &b.frags[n]
		if g.kind != fragPrefix {
			continue
		}
		gs := b.padOf(g)
		if left == 0 {
			gs.assign(last, 0)
			continue
		}
		if gs.maxBytes == 0 {
			continue
		}
		size := min(int(gs.maxBytes), left)
		left -= size
		gs.assign(last, uint8(size))
	}
}

func (s *padState) assign(last bool, size uint8) {
	if last {
		s.lastLength = size
		return
	}
	s.length = size
}
