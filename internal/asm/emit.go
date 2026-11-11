package asm

func (k FixupKind) isPCRel() bool {
	switch k {
	case PC32, PLT32, GOTPCREL, GOTPCRELX, REXGOTPCRELX:
		return true
	}
	return false
}

type textEmitter struct {
	a    *assembler
	b    *secBuilder
	out  []byte
	next int
}

func (a *assembler) emitText(b *secBuilder) {
	e := &textEmitter{a: a, b: b, out: make([]byte, 0, b.Size)}
	for k := range b.frags {
		e.frag(k)
	}
	b.Data = e.out
}

func (e *textEmitter) frag(k int) {
	b := e.b
	f := &b.frags[k]
	end := f.start + f.fix
	chunkEnd := end
	if f.kind == fragJump {
		chunkEnd--
	}
	base := int64(len(e.out)) - int64(f.start)
	e.out = append(e.out, b.Data[f.start:chunkEnd]...)
	e.applyFixups(base, chunkEnd)
	switch f.kind {
	case fragJump:
		e.jump(f, b.Data[chunkEnd])
	case fragAlign:
		e.padding(k)
	case fragPadding, fragFused:
		e.branchPadding(f)
	case fragPrefix:
		e.prefixBytes(f)
	}
}

func (e *textEmitter) branchPadding(f *frag) {
	count := int64(e.b.padOf(f).length)
	if count > 0 {
		e.out = appendNops(e.out, count, true)
	}
}

func (e *textEmitter) prefixBytes(f *frag) {
	st := e.b.padOf(f)
	for range st.length {
		e.out = append(e.out, st.defaultPrefix)
	}
}

func (e *textEmitter) padding(k int) {
	f := &e.b.frags[k]
	following := &e.b.frags[k+1]
	count := following.addr - f.addr - int64(f.fix)
	if count > 0 {
		e.out = appendNops(e.out, count, f.normal)
	}
}

func (e *textEmitter) applyFixups(base int64, end int32) {
	b := e.b
	for e.next < len(b.fixups) && b.fixups[e.next].at < end {
		fixup := &b.fixups[e.next]
		e.next++
		e.fixup(fixup, int64(fixup.at)+base)
	}
}

func (e *textEmitter) fixup(f *textFixup, at int64) {
	a := e.a
	a.line = int(f.line)
	switch {
	case f.neg != nil:
		value := a.differenceValue(f.sym, f.neg, f.addend)
		e.patch(at, int(f.size), value)
	case f.kind.isPCRel() && e.resolvesLocally(f):
		value := a.address(f.sym) + f.addend - at
		e.patch(at, int(f.size), value)
	default:
		e.relocate(at, f.kind, f.sym, f.addend, a.line)
	}
}

func (e *textEmitter) resolvesLocally(f *textFixup) bool {
	sym := f.sym
	return (f.kind == PC32 || f.kind == PLT32) && sym.state == symLabel && sym.section == e.b && !sym.global
}

func (e *textEmitter) patch(at int64, size int, value int64) {
	encoded := appendInt(nil, size, value)
	copy(e.out[at:], encoded)
}

func (e *textEmitter) relocate(at int64, kind FixupKind, sym *symbol, addend int64, line int) {
	fixup := Fixup{Offset: at, Kind: kind, Symbol: sym.name, Addend: addend, Line: line}
	e.b.Fixups = append(e.b.Fixups, fixup)
}

func (e *textEmitter) jump(f *frag, opcode byte) {
	if f.form == jumpForced {
		e.forcedJump(f, opcode)
		return
	}
	target := e.a.address(f.sym) + f.off
	field := f.addr + int64(f.fix)
	if f.form == jumpShort {
		e.shortJump(f, opcode, target-field-shortJumpSize)
		return
	}
	e.nearJump(f, opcode, target-field)
}

func (e *textEmitter) shortJump(f *frag, opcode byte, disp int64) {
	if disp < -128 || disp > 127 {
		e.a.line = int(f.line)
		e.a.fail("short jump displacement %d out of range", disp)
	}
	e.out = append(e.out, opcode, byte(disp))
}

func (e *textEmitter) nearJump(f *frag, opcode byte, fromField int64) {
	size := int64(f.size)
	if f.cond {
		e.out = append(e.out, opTwoByte, opcode+jccNearBias)
	} else {
		e.out = append(e.out, opJmpNear)
	}
	e.out = appendInt(e.out, 4, fromField-size)
}

func (e *textEmitter) forcedJump(f *frag, opcode byte) {
	if f.cond {
		e.out = append(e.out, opTwoByte, opcode+jccNearBias)
	} else {
		e.out = append(e.out, opJmpNear)
	}
	at := int64(len(e.out))
	e.out = appendInt(e.out, 4, 0)
	kind := PC32
	if f.off == 0 && e.a.needsPLT(f.sym) {
		kind = PLT32
	}
	e.relocate(at, kind, f.sym, f.off-4, int(f.line))
}
