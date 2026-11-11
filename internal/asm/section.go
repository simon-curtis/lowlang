package asm

import (
	"encoding/binary"
	"strings"
)

type secBuilder struct {
	Section
	index    int
	exec     bool
	dataOpen bool
}

type pendingDifference struct {
	section *secBuilder
	offset  int64
	size    int
	value   Value
	line    int
}

type sectionDefault struct {
	flags  string
	noBits bool
}

var sectionDefaults = map[string]sectionDefault{
	".text":   {flags: "ax"},
	".data":   {flags: "aw"},
	".bss":    {flags: "aw", noBits: true},
	".rodata": {flags: "a"},
	".tbss":   {flags: "awT", noBits: true},
	".tdata":  {flags: "awT"},
}

func (b *secBuilder) mark() (int64, int) {
	if b.exec {
		b.dataOpen = false
		return 0, len(b.Items)
	}
	if b.NoBits {
		return b.Size, 0
	}
	return int64(len(b.Data)), 0
}

func (b *secBuilder) dataItem(line int) *Item {
	if b.dataOpen {
		return &b.Items[len(b.Items)-1]
	}
	b.Items = append(b.Items, Item{Kind: ItemData, Line: line})
	b.dataOpen = true
	return &b.Items[len(b.Items)-1]
}

func (b *secBuilder) raiseAlign(n int64) {
	if n > b.Align {
		b.Align = n
	}
}

func (a *assembler) section(name, flags string, hasFlags, noBits bool) {
	if existing, ok := a.sections[name]; ok {
		a.cur = existing
		return
	}
	def := sectionDefaults[name]
	if !hasFlags {
		flags = def.flags
		noBits = def.noBits
	}
	b := &secBuilder{index: len(a.secOrder)}
	b.Name, b.Flags, b.NoBits = name, flags, noBits
	b.Align = 1
	b.exec = strings.Contains(flags, "x")
	a.sections[name] = b
	a.secOrder = append(a.secOrder, b)
	a.cur = b
}

func (a *assembler) target() *[]byte {
	b := a.cur
	if b.exec {
		return &b.dataItem(a.line).Data
	}
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

func (a *assembler) emitAlign(power int64) {
	if power < 0 || power > 16 {
		a.fail("bad alignment %d", power)
	}
	n := int64(1) << power
	b := a.cur
	b.raiseAlign(n)
	switch {
	case b.exec:
		b.Items = append(b.Items, Item{Kind: ItemAlign, Line: a.line, Align: n})
		b.dataOpen = false
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
	if b.exec {
		item := b.dataItem(a.line)
		offset := int64(len(item.Data))
		expr := DataExpr{Offset: offset, Size: size, Value: v, Line: a.line}
		item.Exprs = append(item.Exprs, expr)
		item.Data = appendFill(item.Data, int64(size), 0)
		return
	}
	if b.NoBits {
		a.fail("cannot emit data into nobits section %s", b.Name)
	}
	offset := int64(len(b.Data))
	if v.Neg != "" {
		diff := pendingDifference{section: b, offset: offset, size: size, value: v, line: a.line}
		a.pending = append(a.pending, diff)
	} else {
		kind := a.dataFixupKind(size, v.Reloc)
		b.Fixups = append(b.Fixups, Fixup{Offset: offset, Kind: kind, Symbol: v.Sym, Addend: v.Add, Line: a.line})
	}
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

func (a *assembler) resolveDifference(p pendingDifference) {
	a.line = p.line
	v := p.value
	if v.Sym == "" || v.Reloc != RelocNone {
		a.fail("unsupported symbol difference")
	}
	plus, minus := a.syms[v.Sym], a.syms[v.Neg]
	if plus.state != symLabel || minus.state != symLabel {
		a.fail("symbol difference %s - %s uses an undefined symbol", v.Sym, v.Neg)
	}
	if plus.section != minus.section {
		a.fail("symbol difference %s - %s spans sections", v.Sym, v.Neg)
	}
	if plus.section.exec {
		p.section.Unresolved = append(p.section.Unresolved, Span{Offset: p.offset, Size: int64(p.size)})
		return
	}
	value := plus.value - minus.value + v.Add
	patch := appendInt(nil, p.size, value)
	copy(p.section.Data[p.offset:], patch)
}

func (a *assembler) finish() {
	if len(a.conds) != 0 {
		a.fail("missing .endif")
	}
	for _, p := range a.pending {
		a.resolveDifference(p)
	}
	for _, b := range a.secOrder {
		if !b.NoBits && !b.exec {
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
	out := Symbol{Name: sym.name, Global: sym.global, Value: sym.value, Item: sym.item}
	switch sym.state {
	case symUndefined:
		out.Section = SectionUndefined
		out.Global = true
	case symAbsolute:
		out.Section = SectionAbsolute
	default:
		out.Section = sym.section.index
	}
	return out
}
