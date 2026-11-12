package link

import "debug/elf"

const (
	gotPltReserved uint64 = 24
	maxRelroPasses        = 8
	unwindAlign    uint64 = 8
)

func (l *linker) layoutDynamic() {
	header := l.sectionsOf(classInterp, classHash, classGnuHash, classDynsym, classDynstr, classVersym, classVerneed, classRelaDyn, classRelaPlt)
	text := l.sectionsOf(classPlt, classPltGot, classText)
	rodata := l.sectionsOf(classRodata)
	relro := l.sectionsOf(classTData, classTBss, classDynamic, classGot)
	writable := l.sectionsOf(classTData, classTBss, classDynamic, classGot, classGotPlt, classData, classBss)
	tls := l.sectionsOf(classTData, classTBss)

	count := 7
	for _, present := range []bool{len(text) > 0, len(rodata) > 0, len(tls) > 0} {
		if present {
			count++
		}
	}
	headers := headerSize + uint64(count)*phdrSize
	c := &cursor{off: headers, addr: BaseAddress + headers}

	loads := []segment{l.headerSegment(c, header)}
	if len(text) > 0 {
		c.alignToPage()
		loads = append(loads, l.loadSegment(c, text, elf.PF_R|elf.PF_X))
	}
	if len(rodata) > 0 {
		c.alignToPage()
		loads = append(loads, l.unwindPadded(c, l.loadSegment(c, rodata, elf.PF_R)))
	}
	rw := l.relroStart(*c, relro)
	loads = append(loads, l.loadSegment(&rw, writable, elf.PF_R|elf.PF_W))
	l.fileEnd = rw.off

	phdr := segment{
		kind: elf.PT_PHDR, flags: elf.PF_R, off: headerSize, addr: BaseAddress + headerSize,
		fileSize: uint64(count) * phdrSize, memSize: uint64(count) * phdrSize, align: 8,
	}
	l.segments = append(l.segments, phdr, l.interpSegment(header[0]))
	l.segments = append(l.segments, loads...)
	l.segments = append(l.segments, l.dynamicSegment())
	if len(tls) > 0 {
		l.segments = append(l.segments, l.tlsSegment(tls))
	}
	l.segments = append(l.segments, segment{kind: elf.PT_GNU_STACK, flags: elf.PF_R | elf.PF_W, align: 16})
	l.segments = append(l.segments, l.relroSegment(relro))
}

func (l *linker) unwindPadded(c *cursor, seg segment) segment {
	c.alignTo(unwindAlign)
	seg.fileSize = c.addr - seg.addr
	seg.memSize = seg.fileSize
	return seg
}

func (l *linker) headerSegment(c *cursor, members []*section) segment {
	seg := l.loadSegment(c, members, elf.PF_R)
	seg.off, seg.addr = 0, BaseAddress
	last := members[len(members)-1]
	seg.fileSize = last.end() - BaseAddress
	seg.memSize = seg.fileSize
	return seg
}

func (l *linker) interpSegment(interp *section) segment {
	return segment{
		kind: elf.PT_INTERP, flags: elf.PF_R, off: interp.off, addr: interp.addr,
		fileSize: interp.size, memSize: interp.size, align: 1,
	}
}

func (l *linker) dynamicSegment() segment {
	dynamic := l.dyn.sections.dynamic
	return segment{
		kind: elf.PT_DYNAMIC, flags: elf.PF_R | elf.PF_W, off: dynamic.off, addr: dynamic.addr,
		fileSize: dynamic.size, memSize: dynamic.size, align: dynamic.align,
	}
}

func (l *linker) relroSegment(relro []*section) segment {
	first := relro[0]
	size := l.relroEnd(relro) - first.addr
	return segment{
		kind: elf.PT_GNU_RELRO, flags: elf.PF_R, off: first.off, addr: first.addr,
		fileSize: size, memSize: size, align: 1,
	}
}

func (l *linker) relroEnd(relro []*section) uint64 {
	end := uint64(0)
	for _, s := range relro {
		if s.class != classTBss {
			end = s.end()
		}
	}
	gotPlt := l.dyn.sections.gotPlt
	return alignUp(end, gotPlt.align) + gotPltReserved
}

func (l *linker) relroStart(c cursor, relro []*section) cursor {
	floor := alignUp(c.addr, pageSize)
	for range maxRelroPasses {
		trial := c
		for _, s := range relro {
			trial.place(s)
		}
		gap := (pageSize - l.relroEnd(relro)%pageSize) % pageSize
		if gap == 0 {
			break
		}
		c.off += gap
		c.addr += gap
	}
	if c.addr-maxAlign(relro) < floor {
		c.addr += pageSize
	}
	return c
}

func maxAlign(members []*section) uint64 {
	largest := uint64(1)
	for _, s := range members {
		largest = max(largest, s.align)
	}
	return largest
}
