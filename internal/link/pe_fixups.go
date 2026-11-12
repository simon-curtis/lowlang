package link

import (
	"debug/pe"
	"encoding/binary"
	"sort"

	"lowlang/internal/asm"
)

const (
	peRelBasedDir64 uint16 = 10
	peBlockHeader   uint64 = 8
	peRelocChars    uint32 = peReadOnlyData | pe.IMAGE_SCN_MEM_DISCARDABLE
)

func (l *peLinker) rvaOf(sym asm.Symbol) (uint64, bool) {
	if sym.Section < 0 {
		return 0, false
	}
	c, ok := l.chunkOf[sym.Section]
	if !ok {
		return 0, false
	}
	return uint64(int64(c.rva()) + sym.Value), true
}

func (l *peLinker) imageBase() uint64 {
	return peImageBase
}

func (l *peLinker) supports(kind asm.FixupKind) bool {
	switch kind {
	case asm.Abs64, asm.ImgRel32, asm.PC32, asm.PLT32:
		return true
	}
	return false
}

func (l *peLinker) target(_ *fixupSpace, f asm.Fixup) (uint64, bool, error) {
	sym, ok := l.symbols[f.Symbol]
	if !ok {
		return 0, false, nil
	}
	if sym.Section == asm.SectionAbsolute {
		return uint64(sym.Value), true, nil
	}
	rva, ok := l.rvaOf(sym)
	return peImageBase + rva, ok, nil
}

func (l *peLinker) entryRVA(name string) (uint64, bool) {
	sym, ok := l.symbols[name]
	if !ok {
		return 0, false
	}
	return l.rvaOf(sym)
}

func (l *peLinker) findBaseRelocations() {
	for _, c := range l.chunks {
		if c.in == nil {
			continue
		}
		for _, f := range c.in.Fixups {
			if f.Kind != asm.Abs64 {
				continue
			}
			sym, ok := l.symbols[f.Symbol]
			if !ok || sym.Section < 0 {
				continue
			}
			if _, placed := l.chunkOf[sym.Section]; !placed {
				continue
			}
			l.sites = append(l.sites, peSite{chunk: c, offset: uint64(f.Offset)})
		}
	}
}

func (l *peLinker) addRelocations() {
	if len(l.sites) == 0 {
		return
	}
	rvas := make([]uint64, len(l.sites))
	for i, site := range l.sites {
		rvas[i] = site.chunk.rva() + site.offset
	}
	sort.Slice(rvas, func(a, b int) bool { return rvas[a] < rvas[b] })
	data := encodeBaseRelocations(rvas)
	size := uint64(len(data))
	c := &peChunk{name: relocName, data: data, size: size, align: 1, chars: peRelocChars}
	s := &peSection{name: relocName, chars: peRelocChars, chunks: []*peChunk{c}}
	c.section = s
	s.arrange()
	s.place(l.imageSize, l.fileSize)
	l.sections = append(l.sections, s)
	l.imageSize = alignUp(s.rva+s.virtualSize, peSectionAlign)
	l.fileSize += s.rawSize
}

func encodeBaseRelocations(rvas []uint64) []byte {
	var data []byte
	for start := 0; start < len(rvas); {
		page := rvas[start] &^ (pageSize - 1)
		end := start
		for end < len(rvas) && rvas[end]&^(pageSize-1) == page {
			end++
		}
		size := alignUp(peBlockHeader+uint64(end-start)*2, 4)
		data = binary.LittleEndian.AppendUint32(data, uint32(page))
		data = binary.LittleEndian.AppendUint32(data, uint32(size))
		for _, rva := range rvas[start:end] {
			entry := peRelBasedDir64<<12 | uint16(rva-page)
			data = binary.LittleEndian.AppendUint16(data, entry)
		}
		for written := peBlockHeader + uint64(end-start)*2; written < size; written++ {
			data = append(data, 0)
		}
		start = end
	}
	return data
}

func (l *peLinker) applyFixups(image []byte) error {
	var spaces []fixupSpace
	for _, c := range l.chunks {
		if c.in == nil {
			continue
		}
		spaces = append(spaces, fixupSpace{
			name: c.name, addr: peImageBase + c.rva(), offset: c.fileOffset(), size: c.size, fixups: c.in.Fixups,
		})
	}
	return relocate(l, image, spaces)
}
