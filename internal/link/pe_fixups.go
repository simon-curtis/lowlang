package link

import (
	"encoding/binary"
	"fmt"
	"math"
	"sort"
	"strings"

	"lowlang/internal/asm"
)

const (
	peRelBasedDir64 uint16 = 10
	pePageSize      uint64 = 0x1000
	peBlockHeader   uint64 = 8
	peRelocChars    uint32 = scnInit | scnRead | scnDiscardable
)

type peTarget struct {
	value    int64
	absolute bool
}

func (l *peLinker) target(name string) (peTarget, bool) {
	sym, ok := l.symbols[name]
	if !ok {
		return peTarget{}, false
	}
	switch sym.Section {
	case asm.SectionUndefined:
		return peTarget{}, false
	case asm.SectionAbsolute:
		return peTarget{value: sym.Value, absolute: true}, true
	}
	c, ok := l.chunkOf[sym.Section]
	if !ok {
		return peTarget{}, false
	}
	return peTarget{value: int64(c.rva()) + sym.Value}, true
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
		page := rvas[start] &^ (pePageSize - 1)
		end := start
		for end < len(rvas) && rvas[end]&^(pePageSize-1) == page {
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
	undefined := map[string]bool{}
	for _, c := range l.chunks {
		if c.in == nil {
			continue
		}
		for _, f := range c.in.Fixups {
			target, ok := l.target(f.Symbol)
			if !ok {
				undefined[f.Symbol] = true
				continue
			}
			if err := l.applyFixup(image, c, f, target); err != nil {
				return err
			}
		}
	}
	if len(undefined) == 0 {
		return nil
	}
	names := make([]string, 0, len(undefined))
	for name := range undefined {
		names = append(names, name)
	}
	sort.Strings(names)
	return fmt.Errorf("link: undefined symbol %s", strings.Join(names, ", "))
}

func (l *peLinker) applyFixup(image []byte, c *peChunk, f asm.Fixup, target peTarget) error {
	width := peFixupWidth(f.Kind)
	if width == 0 {
		return fmt.Errorf("link: %s relocation against %s in %s is not supported in a PE image", f.Kind, f.Symbol, c.name)
	}
	if f.Offset < 0 || uint64(f.Offset)+uint64(width) > c.size {
		return fmt.Errorf("link: %s relocation against %s lies outside %s", f.Kind, f.Symbol, c.name)
	}
	place := int64(c.rva()) + f.Offset
	value := l.fixupValue(f, target, place)
	if err := checkPERange(f.Kind, value); err != nil {
		return fmt.Errorf("link: %s relocation against %s at %s+%#x: %w", f.Kind, f.Symbol, c.name, f.Offset, err)
	}
	field := image[c.fileOffset()+uint64(f.Offset):]
	if width == 8 {
		binary.LittleEndian.PutUint64(field, uint64(value))
		return nil
	}
	binary.LittleEndian.PutUint32(field, uint32(value))
	return nil
}

func (l *peLinker) fixupValue(f asm.Fixup, target peTarget, place int64) int64 {
	base := int64(peImageBase)
	switch f.Kind {
	case asm.Abs64:
		if target.absolute {
			return target.value + f.Addend
		}
		return base + target.value + f.Addend
	case asm.ImgRel32:
		return target.value + f.Addend
	}
	if target.absolute {
		return target.value + f.Addend - (base + place)
	}
	return target.value + f.Addend - place
}

func peFixupWidth(kind asm.FixupKind) int {
	switch kind {
	case asm.Abs64:
		return 8
	case asm.ImgRel32, asm.PC32, asm.PLT32:
		return 4
	}
	return 0
}

func checkPERange(kind asm.FixupKind, value int64) error {
	switch kind {
	case asm.ImgRel32:
		if value < 0 || value > math.MaxUint32 {
			return fmt.Errorf("value %#x does not fit in 32 unsigned bits", value)
		}
	case asm.PC32, asm.PLT32:
		if value < math.MinInt32 || value > math.MaxInt32 {
			return fmt.Errorf("value %#x does not fit in 32 signed bits", value)
		}
	}
	return nil
}
