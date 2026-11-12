package link

import (
	"debug/elf"
	"sort"
	"strings"

	"lowlang/internal/asm"
)

const (
	pageSize   uint64 = 0x1000
	headerSize uint64 = 64
	phdrSize   uint64 = 56
)

type class int

const (
	classInterp class = iota
	classHash
	classGnuHash
	classDynsym
	classDynstr
	classVersym
	classVerneed
	classRelaDyn
	classRelaPlt
	classPlt
	classPltGot
	classText
	classRodata
	classTData
	classTBss
	classDynamic
	classGot
	classGotPlt
	classData
	classBss
)

type section struct {
	in          *asm.Section
	class       class
	size        uint64
	align       uint64
	addr        uint64
	off         uint64
	index       int
	shType      elf.SectionType
	shFlags     elf.SectionFlag
	entsize     uint64
	linked      *section
	infoSection *section
	info        uint32
}

func (s *section) end() uint64 {
	return s.addr + s.size
}

func (s *section) flags() elf.SectionFlag {
	flags := elf.SHF_ALLOC
	if strings.Contains(s.in.Flags, "w") {
		flags |= elf.SHF_WRITE
	}
	if strings.Contains(s.in.Flags, "x") {
		flags |= elf.SHF_EXECINSTR
	}
	if s.class == classTData || s.class == classTBss {
		flags |= elf.SHF_TLS
	}
	return flags | s.shFlags
}

func (s *section) kind() elf.SectionType {
	if s.shType != elf.SHT_NULL {
		return s.shType
	}
	if s.in.NoBits {
		return elf.SHT_NOBITS
	}
	return elf.SHT_PROGBITS
}

func classOf(in *asm.Section) (class, bool) {
	flags := in.Flags
	if !strings.Contains(flags, "a") {
		return 0, false
	}
	switch {
	case strings.Contains(flags, "T") && in.NoBits:
		return classTBss, true
	case strings.Contains(flags, "T"):
		return classTData, true
	case strings.Contains(flags, "x"):
		return classText, true
	case in.NoBits:
		return classBss, true
	case strings.Contains(flags, "w"):
		return classData, true
	}
	return classRodata, true
}

func sizeOf(in *asm.Section) uint64 {
	if in.NoBits {
		return uint64(in.Size)
	}
	return uint64(len(in.Data))
}

func (l *linker) collectSections() {
	for i, in := range l.obj.Sections {
		class, loaded := classOf(in)
		size := sizeOf(in)
		if !loaded || size == 0 {
			continue
		}
		s := &section{in: in, class: class, size: size, align: uint64(max(in.Align, 1))}
		l.byInput[i] = s
		l.sections = append(l.sections, s)
	}
}

func (l *linker) orderSections() {
	sort.SliceStable(l.sections, func(a, b int) bool { return l.sections[a].class < l.sections[b].class })
	for i, s := range l.sections {
		s.index = i + 1
	}
}

type cursor struct {
	off  uint64
	addr uint64
}

func (c *cursor) place(s *section) {
	aligned := alignUp(c.addr, s.align)
	shift := aligned - c.addr
	s.off, s.addr = c.off+shift, aligned
	if s.class == classTBss {
		return
	}
	c.off += shift
	c.addr = aligned + s.size
	if !s.in.NoBits {
		c.off += s.size
	}
}

func (c *cursor) alignTo(align uint64) {
	c.off = alignUp(c.off, align)
	c.addr = alignUp(c.addr, align)
}

func (c *cursor) alignToPage() {
	c.off = alignUp(c.off, pageSize)
	c.addr = alignUp(c.addr, pageSize)
}

func (l *linker) sectionsOf(classes ...class) []*section {
	var found []*section
	for _, s := range l.sections {
		for _, c := range classes {
			if s.class == c {
				found = append(found, s)
			}
		}
	}
	return found
}

func (l *linker) layout() {
	if l.dyn != nil {
		l.layoutDynamic()
		return
	}
	l.layoutStatic()
}

func (l *linker) layoutStatic() {
	text := l.sectionsOf(classText)
	rodata := l.sectionsOf(classRodata)
	writable := l.sectionsOf(classTData, classTBss, classData, classBss)
	tls := l.sectionsOf(classTData, classTBss)
	loadsWritable := len(l.sectionsOf(classTData, classData, classBss)) > 0

	count := 2
	for _, present := range []bool{len(text) > 0, len(rodata) > 0, loadsWritable, len(tls) > 0} {
		if present {
			count++
		}
	}
	headers := headerSize + uint64(count)*phdrSize
	l.segments = append(l.segments, segment{
		kind: elf.PT_LOAD, flags: elf.PF_R, addr: BaseAddress, fileSize: headers, memSize: headers, align: pageSize,
	})
	c := &cursor{off: headers, addr: BaseAddress + headers}
	c.alignToPage()
	if len(text) > 0 {
		l.segments = append(l.segments, l.loadSegment(c, text, elf.PF_R|elf.PF_X))
	}
	if len(rodata) > 0 {
		c.alignToPage()
		l.segments = append(l.segments, l.loadSegment(c, rodata, elf.PF_R))
	}
	if loadsWritable {
		l.segments = append(l.segments, l.writableSegment(c, writable))
	}
	if len(tls) > 0 {
		l.segments = append(l.segments, l.tlsSegment(tls))
	}
	l.segments = append(l.segments, segment{kind: elf.PT_GNU_STACK, flags: elf.PF_R | elf.PF_W, align: 16})
	l.fileEnd = c.off
}

func (l *linker) writableSegment(c *cursor, members []*section) segment {
	shared := *c
	if shared.addr%pageSize != 0 {
		shared.addr += pageSize
	}
	base := shared.addr
	seg := l.loadSegment(&shared, members, elf.PF_R|elf.PF_W)
	if !savesPage(base, seg) {
		*c = shared
		return seg
	}
	c.alignToPage()
	seg = l.loadSegment(c, members, elf.PF_R|elf.PF_W)
	return seg
}

func savesPage(base uint64, seg segment) bool {
	end := alignUp(seg.addr+seg.memSize, 8)
	first := (pageSize - base%pageSize) % pageSize
	last := end % pageSize
	return first != 0 && last != 0 && base/pageSize != end/pageSize && first+last <= pageSize
}

func (l *linker) loadSegment(c *cursor, members []*section, flags elf.ProgFlag) segment {
	for _, s := range members {
		c.place(s)
	}
	first := loadedFirst(members)
	last := members[len(members)-1]
	for _, s := range members {
		if s.class != classTBss {
			last = s
		}
	}
	seg := segment{kind: elf.PT_LOAD, flags: flags, off: first.off, addr: first.addr, align: pageSize}
	seg.memSize = last.end() - first.addr
	for _, s := range members {
		if !s.in.NoBits {
			seg.fileSize = s.end() - first.addr
		}
	}
	return seg
}

func loadedFirst(members []*section) *section {
	for _, s := range members {
		if s.class != classTBss {
			return s
		}
	}
	return members[0]
}

func (l *linker) tlsSegment(members []*section) segment {
	first := members[0]
	seg := segment{kind: elf.PT_TLS, flags: elf.PF_R, off: first.off, addr: first.addr, align: 1}
	for _, s := range members {
		seg.align = max(seg.align, s.align)
		seg.memSize = s.end() - first.addr
		if s.class == classTData {
			seg.fileSize = s.end() - first.addr
		}
	}
	return seg
}

func (s *section) header(name uint32) elf.Section64 {
	header := elf.Section64{
		Name: name, Type: uint32(s.kind()), Flags: uint64(s.flags()),
		Addr: s.addr, Off: s.off, Size: s.size, Addralign: s.align,
		Entsize: s.entsize, Info: s.info,
	}
	if s.linked != nil {
		header.Link = uint32(s.linked.index)
	}
	if s.infoSection != nil {
		header.Info = uint32(s.infoSection.index)
	}
	return header
}
