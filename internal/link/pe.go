package link

import (
	"debug/pe"
	"fmt"
	"sort"
	"strings"

	"lowlang/internal/asm"
)

const (
	peImageBase      uint64 = 0x140000000
	peSectionAlign   uint64 = pageSize
	peFileAlign      uint64 = 0x200
	peEntrySymbol           = "start"
	peStackReserve   uint64 = 8 << 20
	peStackCommit    uint64 = pageSize
	peHeapReserve    uint64 = 1 << 20
	peHeapCommit     uint64 = pageSize
	peCodeFill              = 0xCC
	peDebugTypeRepro        = 16
)

type peDebugEntry struct {
	Characteristics  uint32
	TimeDateStamp    uint32
	MajorVersion     uint16
	MinorVersion     uint16
	Type             uint32
	SizeOfData       uint32
	AddressOfRawData uint32
	PointerToRawData uint32
}

type peImportDescriptor struct {
	OriginalFirstThunk uint32
	TimeDateStamp      uint32
	ForwarderChain     uint32
	Name               uint32
	FirstThunk         uint32
}

var (
	peDebugEntrySize   = structSize(peDebugEntry{})
	peNullImportLength = structSize(peImportDescriptor{})
)

const (
	peReadOnlyData uint32 = pe.IMAGE_SCN_CNT_INITIALIZED_DATA | pe.IMAGE_SCN_MEM_READ
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
	return s.chars&pe.IMAGE_SCN_CNT_CODE != 0
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
	s.chars &^= pe.IMAGE_SCN_CNT_INITIALIZED_DATA | pe.IMAGE_SCN_CNT_UNINITIALIZED_DATA
	switch {
	case s.isCode():
		return
	case initialized > 0:
		s.chars |= pe.IMAGE_SCN_CNT_INITIALIZED_DATA
	default:
		s.chars |= pe.IMAGE_SCN_CNT_UNINITIALIZED_DATA
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
	if err := l.addSyntheticChunks(); err != nil {
		return nil, err
	}
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
	if err := l.writeHeaders(image, entry, 0); err != nil {
		return nil, err
	}
	if err := l.stamp(image, entry); err != nil {
		return nil, err
	}
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
		if in.Flags.Has(asm.FlagTLS) {
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
	if in.Flags.Has(asm.FlagExec) {
		return pe.IMAGE_SCN_CNT_CODE | pe.IMAGE_SCN_MEM_EXECUTE | pe.IMAGE_SCN_MEM_READ
	}
	chars := uint32(pe.IMAGE_SCN_MEM_READ)
	if in.Flags.Has(asm.FlagWrite) || !in.Flags.Has(asm.FlagReadOnly) {
		chars |= pe.IMAGE_SCN_MEM_WRITE
	}
	if in.NoBits {
		return chars | pe.IMAGE_SCN_CNT_UNINITIALIZED_DATA
	}
	return chars | pe.IMAGE_SCN_CNT_INITIALIZED_DATA
}

func (l *peLinker) debugEntry(timestamp uint32) ([]byte, error) {
	w := &structWriter{}
	w.add(peDebugEntry{TimeDateStamp: timestamp, Type: peDebugTypeRepro})
	return w.bytes()
}

func (l *peLinker) addSyntheticChunks() error {
	debug, err := l.debugEntry(0)
	if err != nil {
		return err
	}
	l.debug = &peChunk{name: rdataName, data: debug, size: peDebugEntrySize, align: 4, chars: peReadOnlyData}
	l.chunks = append(l.chunks, l.debug)
	if len(l.chunksWithPrefix(importGroup)) == 0 {
		return nil
	}
	null := &peChunk{name: importPrefix, data: make([]byte, peNullImportLength), size: peNullImportLength, align: 1, chars: peReadOnlyData}
	l.chunks = append(l.chunks, null)
	return nil
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
	return c.chars&^pe.IMAGE_SCN_CNT_UNINITIALIZED_DATA | pe.IMAGE_SCN_CNT_INITIALIZED_DATA
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
	return entryAddress(peEntrySymbol, l.entryRVA)
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
