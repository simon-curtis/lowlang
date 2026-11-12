package asm

import (
	"encoding/binary"
)

type secBuilder struct {
	Section
	index  int
	frags  []frag
	fixups []textFixup
	pads   []padState
	last   lastKind
}

type pendingDifference struct {
	section *secBuilder
	offset  int64
	size    int
	value   Value
	line    int
}

type sectionSpec struct {
	flags  SectionFlags
	noBits bool
}

var sectionDefaults = map[string]sectionSpec{
	".text":   {flags: FlagAlloc | FlagExec},
	".data":   {flags: FlagAlloc | FlagWrite},
	".bss":    {flags: FlagAlloc | FlagWrite, noBits: true},
	".rodata": {flags: FlagAlloc},
	".tbss":   {flags: FlagAlloc | FlagWrite | FlagTLS, noBits: true},
	".tdata":  {flags: FlagAlloc | FlagWrite | FlagTLS},
}

func (b *secBuilder) exec() bool {
	return b.Flags.Has(FlagExec)
}

func (b *secBuilder) mark() (int64, int32) {
	if b.exec() {
		return int64(len(b.Data)), b.currentFrag()
	}
	if b.NoBits {
		return b.Size, 0
	}
	return int64(len(b.Data)), 0
}

func (b *secBuilder) raiseAlign(n int64) {
	if n > b.Align {
		b.Align = n
	}
}

func (a *assembler) defaultSection(name string) {
	a.section(name, sectionDefaults[name])
}

func (a *assembler) section(name string, spec sectionSpec) {
	if existing, ok := a.sections[name]; ok {
		a.cur = existing
		return
	}
	b := &secBuilder{index: len(a.secOrder)}
	b.Name, b.Flags, b.NoBits = name, spec.flags, spec.noBits
	b.Align = 1
	if b.exec() {
		b.frags = []frag{{}}
	}
	a.sections[name] = b
	a.secOrder = append(a.secOrder, b)
	a.cur = b
}

func (a *assembler) target() *[]byte {
	b := a.cur
	if b.NoBits {
		a.fail("cannot emit data into nobits section %s", b.Name)
	}
	return &b.Data
}

func (a *assembler) emitBytes(data []byte) {
	buf := a.target()
	*buf = append(*buf, data...)
}

func (a *assembler) emitString(text string) {
	buf := a.target()
	*buf = append(*buf, text...)
}

func (a *assembler) emitZero(n int64, fill byte) {
	if n < 0 {
		a.fail("negative size %d", n)
	}
	if a.cur.NoBits {
		a.cur.Size += n
		return
	}
	buf := a.target()
	*buf = appendFill(*buf, n, fill)
}

func appendFill(data []byte, n int64, fill byte) []byte {
	start := len(data)
	data = append(data, make([]byte, n)...)
	if fill != 0 {
		for i := start; i < len(data); i++ {
			data[i] = fill
		}
	}
	return data
}

func (a *assembler) emitAlign(power, max int64) {
	if power < 0 || power > 16 {
		a.fail("bad alignment %d", power)
	}
	if power == 0 {
		return
	}
	n := int64(1) << power
	b := a.cur
	b.raiseAlign(n)
	switch {
	case b.exec():
		b.addAlign(uint8(power), int32(max), a.line)
	case max != 0:
		a.fail("an alignment limit is only supported in executable sections")
	case b.NoBits:
		b.Size = (b.Size + n - 1) &^ (n - 1)
	default:
		pad := (n - int64(len(b.Data))%n) % n
		b.Data = appendFill(b.Data, pad, 0)
	}
}

func (a *assembler) emitConst(size int, v int64) {
	buf := a.target()
	*buf = appendInt(*buf, size, v)
}

func appendInt(data []byte, size int, v int64) []byte {
	switch size {
	case 1:
		return append(data, byte(v))
	case 2:
		return binary.LittleEndian.AppendUint16(data, uint16(v))
	case 4:
		return binary.LittleEndian.AppendUint32(data, uint32(v))
	}
	return binary.LittleEndian.AppendUint64(data, uint64(v))
}

func (a *assembler) emitValue(size int, v Value) {
	if v.IsConst() {
		a.emitConst(size, v.Add)
		return
	}
	b := a.cur
	if b.NoBits {
		a.fail("cannot emit data into nobits section %s", b.Name)
	}
	offset := int64(len(b.Data))
	switch {
	case b.exec():
		a.emitTextValue(size, v)
		return
	case v.Neg != "":
		diff := pendingDifference{section: b, offset: offset, size: size, value: v, line: a.line}
		a.pending = append(a.pending, diff)
	default:
		kind := a.dataFixupKind(size, v.Reloc)
		b.Fixups = append(b.Fixups, Fixup{Offset: offset, Kind: kind, Symbol: v.Sym, Addend: v.Add, Line: a.line})
	}
	b.Data = appendFill(b.Data, int64(size), 0)
}

func (a *assembler) emitTextValue(size int, v Value) {
	b := a.cur
	fixup := textFixup{at: int32(len(b.Data)), size: uint8(size), addend: v.Add, line: int32(a.line)}
	if v.Neg != "" {
		fixup.neg = a.lookup(v.Neg)
	} else {
		fixup.kind = a.dataFixupKind(size, v.Reloc)
	}
	if v.Sym != "" {
		fixup.sym = a.lookup(v.Sym)
	}
	b.addFixup(fixup)
	b.Data = appendFill(b.Data, int64(size), 0)
}

func (a *assembler) dataFixupKind(size int, reloc Reloc) FixupKind {
	switch {
	case size == 8 && reloc == RelocNone:
		return Abs64
	case size == 4 && reloc == RelocNone:
		return Abs32
	case size == 4 && reloc == RelocImgRel:
		return ImgRel32
	}
	a.fail("unsupported relocation for a %d-byte value", size)
	return 0
}

func (a *assembler) differenceValue(plus, minus *symbol, add int64) int64 {
	if plus == nil || plus.state != symLabel || minus.state != symLabel {
		a.fail("symbol difference uses an undefined symbol")
	}
	if plus.section != minus.section {
		a.fail("symbol difference %s - %s spans sections", plus.name, minus.name)
	}
	return a.address(plus) - a.address(minus) + add
}

func (a *assembler) resolveDifference(p pendingDifference) {
	a.line = p.line
	v := p.value
	if v.Sym == "" || v.Reloc != RelocNone {
		a.fail("unsupported symbol difference")
	}
	value := a.differenceValue(a.syms[v.Sym], a.syms[v.Neg], v.Add)
	patch := appendInt(nil, p.size, value)
	copy(p.section.Data[p.offset:], patch)
}

func (a *assembler) address(sym *symbol) int64 {
	if !sym.section.exec() {
		return sym.value
	}
	f := &sym.section.frags[sym.frag]
	return f.addr + sym.value - int64(f.start)
}

func (a *assembler) finish() {
	if len(a.conds) != 0 {
		a.fail("missing .endif")
	}
	for _, b := range a.secOrder {
		if b.exec() {
			a.layout(b)
		}
	}
	for _, b := range a.secOrder {
		if b.exec() {
			a.emitText(b)
		}
	}
	for _, p := range a.pending {
		a.resolveDifference(p)
	}
	for _, b := range a.secOrder {
		if !b.NoBits && !b.exec() {
			b.Size = int64(len(b.Data))
		}
	}
}

func (a *assembler) object() *Object {
	o := &Object{}
	for _, b := range a.secOrder {
		o.Sections = append(o.Sections, &b.Section)
	}
	o.Symbols = make([]Symbol, 0, len(a.symOrder))
	for _, sym := range a.symOrder {
		exported := a.exportSymbol(sym)
		o.Symbols = append(o.Symbols, exported)
	}
	return o
}

func (a *assembler) exportSymbol(sym *symbol) Symbol {
	out := Symbol{Name: sym.name, Global: sym.global, Value: sym.value}
	switch sym.state {
	case symUndefined:
		out.Section = SectionUndefined
		out.Global = true
	case symAbsolute:
		out.Section = SectionAbsolute
	default:
		out.Section = sym.section.index
		out.Value = a.address(sym)
	}
	return out
}
