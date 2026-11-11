package link

import (
	"bytes"
	"debug/elf"
	"encoding/binary"
)

const (
	symSize      uint64 = 24
	tableAlign   uint64 = 8
	shdrSize     uint64 = 64
	machineAMD   uint16 = 62
	elfVersion   uint8  = 1
	symtabName          = ".symtab"
	strtabName          = ".strtab"
	shstrtabName        = ".shstrtab"
)

type segment struct {
	kind     elf.ProgType
	flags    elf.ProgFlag
	off      uint64
	addr     uint64
	fileSize uint64
	memSize  uint64
	align    uint64
}

type tables struct {
	symtab      []byte
	strtab      []byte
	shstrtab    []byte
	firstGlobal uint32
	sectionName []uint32
	symtabName  uint32
	strtabName  uint32
	shstrName   uint32
	symOff      uint64
	strOff      uint64
	shstrOff    uint64
	shOff       uint64
	size        uint64
}

type stringTable struct {
	data []byte
}

func newStringTable() *stringTable {
	return &stringTable{data: []byte{0}}
}

func (t *stringTable) add(name string) uint32 {
	offset := uint32(len(t.data))
	t.data = append(t.data, name...)
	t.data = append(t.data, 0)
	return offset
}

func (l *linker) buildTables() *tables {
	t := &tables{}
	l.buildSymbolTable(t)
	names := newStringTable()
	for _, s := range l.sections {
		t.sectionName = append(t.sectionName, names.add(s.in.Name))
	}
	t.symtabName = names.add(symtabName)
	t.strtabName = names.add(strtabName)
	t.shstrName = names.add(shstrtabName)
	t.shstrtab = names.data

	t.symOff = alignUp(l.fileEnd, tableAlign)
	t.strOff = t.symOff + uint64(len(t.symtab))
	t.shstrOff = t.strOff + uint64(len(t.strtab))
	t.shOff = alignUp(t.shstrOff+uint64(len(t.shstrtab)), tableAlign)
	t.size = t.shOff + l.sectionHeaderCount()*shdrSize
	return t
}

func (l *linker) sectionHeaderCount() uint64 {
	return uint64(len(l.sections)) + 4
}

func (l *linker) buildSymbolTable(t *tables) {
	names := newStringTable()
	locals, globals := l.emittedSymbols()
	var symbols []elf.Sym64
	symbols = append(symbols, elf.Sym64{})
	if len(locals) > 0 {
		symbols = append(symbols, elf.Sym64{
			Name:  names.add(FileSymbol),
			Info:  elf.ST_INFO(elf.STB_LOCAL, elf.STT_FILE),
			Shndx: uint16(elf.SHN_ABS),
		})
	}
	for _, entry := range locals {
		symbols = append(symbols, l.symbolEntry(names, entry, elf.STB_LOCAL))
	}
	t.firstGlobal = uint32(len(symbols))
	for _, entry := range globals {
		symbols = append(symbols, l.symbolEntry(names, entry, elf.STB_GLOBAL))
	}
	t.symtab = make([]byte, 0, len(symbols)*int(symSize))
	for i := range symbols {
		t.symtab = binary.LittleEndian.AppendUint32(t.symtab, symbols[i].Name)
		t.symtab = append(t.symtab, symbols[i].Info, symbols[i].Other)
		t.symtab = binary.LittleEndian.AppendUint16(t.symtab, symbols[i].Shndx)
		t.symtab = binary.LittleEndian.AppendUint64(t.symtab, symbols[i].Value)
		t.symtab = binary.LittleEndian.AppendUint64(t.symtab, symbols[i].Size)
	}
	t.strtab = names.data
}

func (l *linker) symbolEntry(names *stringTable, entry linkerSymbol, bind elf.SymBind) elf.Sym64 {
	kind := elf.STT_NOTYPE
	if entry.value.isTLS() {
		kind = elf.STT_TLS
	}
	return elf.Sym64{
		Name:  names.add(entry.name),
		Info:  elf.ST_INFO(bind, kind),
		Shndx: l.symbolIndexOf(entry.value),
		Value: l.emittedValue(entry.value),
	}
}

func (l *linker) writeHeaders(image []byte, entry uint64, t *tables) {
	header := elf.Header64{
		Type:      uint16(elf.ET_EXEC),
		Machine:   machineAMD,
		Version:   uint32(elfVersion),
		Entry:     entry,
		Phoff:     headerSize,
		Shoff:     t.shOff,
		Ehsize:    uint16(headerSize),
		Phentsize: uint16(phdrSize),
		Phnum:     uint16(len(l.segments)),
		Shentsize: uint16(shdrSize),
		Shnum:     uint16(l.sectionHeaderCount()),
		Shstrndx:  uint16(l.sectionHeaderCount() - 1),
	}
	copy(header.Ident[:], elf.ELFMAG)
	header.Ident[elf.EI_CLASS] = byte(elf.ELFCLASS64)
	header.Ident[elf.EI_DATA] = byte(elf.ELFDATA2LSB)
	header.Ident[elf.EI_VERSION] = elfVersion
	header.Ident[elf.EI_OSABI] = byte(elf.ELFOSABI_NONE)
	l.put(image, 0, &header)
	for i, seg := range l.segments {
		prog := elf.Prog64{
			Type: uint32(seg.kind), Flags: uint32(seg.flags), Off: seg.off,
			Vaddr: seg.addr, Paddr: seg.addr, Filesz: seg.fileSize, Memsz: seg.memSize, Align: seg.align,
		}
		l.put(image, headerSize+uint64(i)*phdrSize, &prog)
	}
}

func (l *linker) put(image []byte, at uint64, value any) {
	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, value)
	copy(image[at:], buf.Bytes())
}

func (l *linker) writeTables(image []byte, t *tables) {
	copy(image[t.symOff:], t.symtab)
	copy(image[t.strOff:], t.strtab)
	copy(image[t.shstrOff:], t.shstrtab)
	symtabIndex := uint32(len(l.sections)) + 1
	headers := make([]elf.Section64, 0, l.sectionHeaderCount())
	headers = append(headers, elf.Section64{})
	for i, s := range l.sections {
		headers = append(headers, elf.Section64{
			Name: t.sectionName[i], Type: uint32(s.kind()), Flags: uint64(s.flags()),
			Addr: s.addr, Off: s.off, Size: s.size, Addralign: s.align,
		})
	}
	headers = append(headers,
		elf.Section64{
			Name: t.symtabName, Type: uint32(elf.SHT_SYMTAB), Off: t.symOff, Size: uint64(len(t.symtab)),
			Link: symtabIndex + 1, Info: t.firstGlobal, Addralign: tableAlign, Entsize: symSize,
		},
		elf.Section64{
			Name: t.strtabName, Type: uint32(elf.SHT_STRTAB), Off: t.strOff, Size: uint64(len(t.strtab)), Addralign: 1,
		},
		elf.Section64{
			Name: t.shstrName, Type: uint32(elf.SHT_STRTAB), Off: t.shstrOff, Size: uint64(len(t.shstrtab)), Addralign: 1,
		},
	)
	for i := range headers {
		l.put(image, t.shOff+uint64(i)*shdrSize, &headers[i])
	}
}
