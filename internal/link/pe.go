package link

import (
	"encoding/binary"
	"fmt"
	"sort"
	"strings"

	"lowlang/internal/asm"
)

const (
	peImageBase       uint64 = 0x140000000
	peSectionAlign    uint64 = 0x1000
	peFileAlign       uint64 = 0x200
	peEntrySymbol            = "start"
	peStackReserve    uint64 = 8 << 20
	peStackCommit     uint64 = 0x1000
	peHeapReserve     uint64 = 1 << 20
	peHeapCommit      uint64 = 0x1000
	peNullImportLen   uint64 = 20
	peDebugEntryLen   uint64 = 28
	peCodeFill               = 0xCC
	peDebugTypeRepro         = 16
	peDebugTypeOffset        = 12
	peTimestampOffset        = 4
)

const (
	scnCode        uint32 = 0x00000020
	scnInit        uint32 = 0x00000040
	scnUninit      uint32 = 0x00000080
	scnDiscardable uint32 = 0x02000000
	scnExec        uint32 = 0x20000000
	scnRead        uint32 = 0x40000000
	scnWrite       uint32 = 0x80000000
)

const (
	importPrefix  = ".idata$2"
	iatPrefix     = ".idata$5"
	importGroup   = ".idata"
	relocName     = ".reloc"
	rdataName     = ".rdata"
	textName      = ".text"
	dataName      = ".data"
	groupSplitter = "$"
)

var peMergedNames = map[string]string{
	".bss":   dataName,
	".idata": rdataName,
}

var peSectionPriority = map[string]int{textName: 0, rdataName: 1, dataName: 2}

type peChunk struct {
	in      *asm.Section
	name    string
	data    []byte
	size    uint64
	align   uint64
	chars   uint32
	uninit  bool
	section *peSection
	offset  uint64
}

func (c *peChunk) rva() uint64 {
	return c.section.rva + c.offset
}

func (c *peChunk) fileOffset() uint64 {
	return c.section.rawOffset + c.offset
}

func (c *peChunk) sortKey() (bool, string) {
	if strings.Contains(c.name, groupSplitter) {
		return c.uninit, c.name
	}
	return c.uninit, ""
}

type peSection struct {
	name        string
	chars       uint32
	chunks      []*peChunk
	rva         uint64
	virtualSize uint64
	rawOffset   uint64
	rawSize     uint64
}

func (s *peSection) isCode() bool {
	return s.chars&scnCode != 0
}

func (s *peSection) arrange() {
	sort.SliceStable(s.chunks, func(a, b int) bool {
		leftUninit, leftName := s.chunks[a].sortKey()
		rightUninit, rightName := s.chunks[b].sortKey()
		if leftUninit != rightUninit {
			return rightUninit
		}
		return leftName < rightName
	})
	var offset, initialized uint64
	for _, c := range s.chunks {
		offset = alignUp(offset, c.align)
		c.offset = offset
		offset += c.size
		if !c.uninit {
			initialized = offset
		}
	}
	s.virtualSize = offset
	s.rawSize = alignUp(initialized, peFileAlign)
	s.chars &^= scnInit | scnUninit
	switch {
	case s.isCode():
		return
	case initialized > 0:
		s.chars |= scnInit
	default:
		s.chars |= scnUninit
	}
}

func (s *peSection) place(rva, offset uint64) {
	s.rva = rva
	if s.rawSize == 0 {
		return
	}
	s.rawOffset = offset
}

type peSite struct {
	chunk  *peChunk
	offset uint64
}

type peLinker struct {
	obj        *asm.Object
	symbols    map[string]asm.Symbol
	chunkOf    map[int]*peChunk
	chunks     []*peChunk
	sections   []*peSection
	debug      *peChunk
	sites      []peSite
	headerSize uint64
	imageSize  uint64
	fileSize   uint64
}

func PE(obj *asm.Object) ([]byte, error) {
	l := newPELinker(obj)
	if err := l.collect(); err != nil {
		return nil, err
	}
	l.addSyntheticChunks()
	l.group()
	l.findBaseRelocations()
	l.layout()
	l.addRelocations()
	entry, err := l.entry()
	if err != nil {
		return nil, err
	}
	image := make([]byte, l.fileSize)
	l.writeSections(image)
	if err := l.applyFixups(image); err != nil {
		return nil, err
	}
	l.writeHeaders(image, entry)
	l.stamp(image)
	return image, nil
}

func newPELinker(obj *asm.Object) *peLinker {
	symbols := make(map[string]asm.Symbol, len(obj.Symbols))
	for _, sym := range obj.Symbols {
		symbols[sym.Name] = sym
	}
	return &peLinker{obj: obj, symbols: symbols, chunkOf: map[int]*peChunk{}}
}

func (l *peLinker) collect() error {
	for i, in := range l.obj.Sections {
		size := sizeOf(in)
		if size == 0 {
			continue
		}
		if strings.Contains(in.Flags, "T") {
			return fmt.Errorf("link: thread-local section %s is not supported in a PE image", in.Name)
		}
		c := &peChunk{
			in: in, name: in.Name, size: size, align: uint64(max(in.Align, 1)),
			chars: peCharacteristics(in), uninit: in.NoBits,
		}
		if !in.NoBits {
			c.data = in.Data
		}
		l.chunkOf[i] = c
		l.chunks = append(l.chunks, c)
	}
	return nil
}

func peCharacteristics(in *asm.Section) uint32 {
	flags := in.Flags
	if strings.Contains(flags, "x") {
		return scnCode | scnExec | scnRead
	}
	chars := scnRead
	if strings.Contains(flags, "w") || !strings.Contains(flags, "r") {
		chars |= scnWrite
	}
	if in.NoBits {
		return chars | scnUninit
	}
	return chars | scnInit
}

func (l *peLinker) addSyntheticChunks() {
	l.debug = &peChunk{name: rdataName, data: make([]byte, peDebugEntryLen), size: peDebugEntryLen, align: 4, chars: scnInit | scnRead}
	binary.LittleEndian.PutUint32(l.debug.data[peDebugTypeOffset:], peDebugTypeRepro)
	l.chunks = append(l.chunks, l.debug)
	if len(l.chunksWithPrefix(importGroup)) == 0 {
		return
	}
	null := &peChunk{name: importPrefix, data: make([]byte, peNullImportLen), size: peNullImportLen, align: 1, chars: scnInit | scnRead}
	l.chunks = append(l.chunks, null)
}

func (l *peLinker) chunksWithPrefix(prefix string) []*peChunk {
	var found []*peChunk
	for _, c := range l.chunks {
		if strings.HasPrefix(c.name, prefix) {
			found = append(found, c)
		}
	}
	return found
}

func outputName(name string) string {
	base, _, _ := strings.Cut(name, groupSplitter)
	if merged, ok := peMergedNames[base]; ok {
		return merged
	}
	return base
}

func (c *peChunk) outputChars() uint32 {
	return c.chars&^scnUninit | scnInit
}

type peSectionKey struct {
	name  string
	chars uint32
}

func (l *peLinker) group() {
	byKey := map[peSectionKey]*peSection{}
	for _, c := range l.chunks {
		key := peSectionKey{name: outputName(c.name), chars: c.outputChars()}
		s, ok := byKey[key]
		if !ok {
			s = &peSection{name: key.name, chars: key.chars}
			byKey[key] = s
			l.sections = append(l.sections, s)
		}
		c.section = s
		s.chunks = append(s.chunks, c)
	}
	sort.SliceStable(l.sections, func(a, b int) bool {
		left, right := l.sections[a], l.sections[b]
		leftRank, leftKnown := peSectionPriority[left.name]
		rightRank, rightKnown := peSectionPriority[right.name]
		switch {
		case leftKnown && rightKnown:
			return leftRank < rightRank
		case leftKnown != rightKnown:
			return leftKnown
		}
		return left.name < right.name
	})
	for _, s := range l.sections {
		s.arrange()
	}
}

func (l *peLinker) layout() {
	count := uint64(len(l.sections))
	if len(l.sites) > 0 {
		count++
	}
	preamble := uint64(len(peDOSStub)) + peSignatureSize + peFileHeaderSize + peOptionalHeaderSize
	l.headerSize = alignUp(preamble+count*peSectionHeaderSize, peFileAlign)
	rva := alignUp(l.headerSize, peSectionAlign)
	offset := l.headerSize
	for _, s := range l.sections {
		s.place(rva, offset)
		rva = alignUp(rva+s.virtualSize, peSectionAlign)
		offset += s.rawSize
	}
	l.imageSize = rva
	l.fileSize = offset
}

func (l *peLinker) entry() (uint64, error) {
	target, ok := l.target(peEntrySymbol)
	if !ok || target.absolute {
		return 0, fmt.Errorf("link: entry symbol %s is not defined", peEntrySymbol)
	}
	return uint64(target.value), nil
}

func (l *peLinker) writeSections(image []byte) {
	for _, s := range l.sections {
		if s.isCode() {
			s.fillCode(image)
		}
		for _, c := range s.chunks {
			start := c.fileOffset()
			copy(image[start:], c.data)
		}
	}
}

func (s *peSection) fillCode(image []byte) {
	span := image[s.rawOffset : s.rawOffset+s.rawSize]
	for i := range span {
		span[i] = peCodeFill
	}
}
