package link_test

import (
	"bytes"
	"debug/pe"
	"encoding/binary"
	"strings"
	"testing"

	"lowlang/internal/asm"
	"lowlang/internal/link"
)

const (
	peTextIndex = iota
	peRodataIndex
	peDataIndex
	peBssIndex
)

const (
	peImageBase    = 0x140000000
	peDirImport    = 1
	peDirBaseReloc = 5
	peDirDebug     = 6
	peDirIAT       = 12
)

type peOptions struct {
	textFixups   []asm.Fixup
	rodataFixups []asm.Fixup
	dataFixups   []asm.Fixup
	extra        []*asm.Section
	symbols      []asm.Symbol
	omitEntry    bool
}

func peObject(options peOptions) *asm.Object {
	text := &asm.Section{Name: ".text", Flags: asm.FlagAlloc | asm.FlagExec, Align: 16, Data: make([]byte, 64), Fixups: options.textFixups}
	rodata := &asm.Section{Name: ".rodata", Flags: asm.FlagAlloc, Align: 8, Data: make([]byte, 32), Fixups: options.rodataFixups}
	data := &asm.Section{Name: ".data", Flags: asm.FlagAlloc | asm.FlagWrite, Align: 8, Data: make([]byte, 24), Fixups: options.dataFixups}
	bss := &asm.Section{Name: ".bss", Flags: asm.FlagAlloc | asm.FlagWrite, Align: 8, NoBits: true, Size: 5000}
	sections := append([]*asm.Section{text, rodata, data, bss}, options.extra...)
	symbols := []asm.Symbol{
		{Name: "text_label", Section: peTextIndex, Value: 8},
		{Name: "rodata_label", Section: peRodataIndex, Value: 4},
		{Name: "data_label", Section: peDataIndex, Value: 16},
		{Name: "bss_label", Section: peBssIndex, Value: 8},
		{Name: "constant", Section: asm.SectionAbsolute, Value: 0x1234},
	}
	if !options.omitEntry {
		symbols = append(symbols, asm.Symbol{Name: "start", Section: peTextIndex, Global: true})
	}
	symbols = append(symbols, options.symbols...)
	return &asm.Object{Sections: sections, Symbols: symbols}
}

func linkPE(t *testing.T, obj *asm.Object) ([]byte, *pe.File) {
	t.Helper()
	image, err := link.PE(obj)
	if err != nil {
		t.Fatalf("link: %v", err)
	}
	file, err := pe.NewFile(bytes.NewReader(image))
	if err != nil {
		t.Fatalf("parse image: %v", err)
	}
	return image, file
}

func peSection(t *testing.T, file *pe.File, name string) *pe.Section {
	t.Helper()
	for _, s := range file.Sections {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("no section %s", name)
	return nil
}

func peHeader(t *testing.T, file *pe.File) *pe.OptionalHeader64 {
	t.Helper()
	header, ok := file.OptionalHeader.(*pe.OptionalHeader64)
	if !ok {
		t.Fatalf("optional header is %T", file.OptionalHeader)
	}
	return header
}

func peSectionNames(file *pe.File) []string {
	var names []string
	for _, s := range file.Sections {
		names = append(names, s.Name)
	}
	return names
}

func TestPEHeaders(t *testing.T) {
	image, file := linkPE(t, peObject(peOptions{}))
	header := peHeader(t, file)
	if file.Machine != pe.IMAGE_FILE_MACHINE_AMD64 || file.Characteristics != 0x22 {
		t.Errorf("machine %#x characteristics %#x", file.Machine, file.Characteristics)
	}
	if header.ImageBase != peImageBase || header.SectionAlignment != 0x1000 || header.FileAlignment != 0x200 {
		t.Errorf("image base %#x section alignment %#x file alignment %#x", header.ImageBase, header.SectionAlignment, header.FileAlignment)
	}
	if header.Subsystem != pe.IMAGE_SUBSYSTEM_WINDOWS_CUI || header.DllCharacteristics != 0x8160 {
		t.Errorf("subsystem %d dll characteristics %#x", header.Subsystem, header.DllCharacteristics)
	}
	if header.SizeOfStackReserve != 8<<20 || header.SizeOfStackCommit != 0x1000 || header.SizeOfHeapReserve != 1<<20 || header.SizeOfHeapCommit != 0x1000 {
		t.Errorf("stack %d/%d heap %d/%d", header.SizeOfStackReserve, header.SizeOfStackCommit, header.SizeOfHeapReserve, header.SizeOfHeapCommit)
	}
	if header.MajorOperatingSystemVersion != 6 || header.MajorSubsystemVersion != 6 || header.MajorLinkerVersion != 14 {
		t.Errorf("versions os %d subsystem %d linker %d", header.MajorOperatingSystemVersion, header.MajorSubsystemVersion, header.MajorLinkerVersion)
	}
	text := peSection(t, file, ".text")
	if header.AddressOfEntryPoint != text.VirtualAddress || header.BaseOfCode != text.VirtualAddress {
		t.Errorf("entry %#x base of code %#x, text at %#x", header.AddressOfEntryPoint, header.BaseOfCode, text.VirtualAddress)
	}
	if got := uint32(len(image)); header.SizeOfHeaders%0x200 != 0 || got%0x200 != 0 {
		t.Errorf("headers %#x or file %#x not file aligned", header.SizeOfHeaders, got)
	}
	last := file.Sections[len(file.Sections)-1]
	if want := (last.VirtualAddress + last.VirtualSize + 0xFFF) &^ 0xFFF; header.SizeOfImage != want {
		t.Errorf("size of image %#x, want %#x", header.SizeOfImage, want)
	}
}

func TestPESectionLayout(t *testing.T) {
	image, file := linkPE(t, peObject(peOptions{}))
	want := []string{".text", ".rdata", ".data", ".rodata"}
	if got := peSectionNames(file); strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("sections %v, want %v", got, want)
	}
	text := peSection(t, file, ".text")
	if text.Characteristics != 0x60000020 || text.VirtualAddress != 0x1000 || text.VirtualSize != 64 {
		t.Errorf("text %+v", text.SectionHeader)
	}
	if tail := image[text.Offset+text.VirtualSize : text.Offset+text.Size]; !bytes.Equal(tail, bytes.Repeat([]byte{0xCC}, len(tail))) {
		t.Errorf("text padding is not int3")
	}
	data := peSection(t, file, ".data")
	if data.Characteristics != 0xC0000040 {
		t.Errorf("data characteristics %#x", data.Characteristics)
	}
	if data.VirtualSize != 24+5000 || data.Size != 0x200 {
		t.Errorf("data virtual %d raw %d: bss must follow data in the same section without file bytes", data.VirtualSize, data.Size)
	}
	rodata := peSection(t, file, ".rodata")
	if rodata.Characteristics != 0xC0000040 {
		t.Errorf("rodata characteristics %#x", rodata.Characteristics)
	}
	for i := 1; i < len(file.Sections); i++ {
		previous, current := file.Sections[i-1], file.Sections[i]
		if current.VirtualAddress != (previous.VirtualAddress+previous.VirtualSize+0xFFF)&^0xFFF {
			t.Errorf("%s at %#x does not follow %s", current.Name, current.VirtualAddress, previous.Name)
		}
		if current.Offset != previous.Offset+previous.Size {
			t.Errorf("%s at file offset %#x does not follow %s", current.Name, current.Offset, previous.Name)
		}
	}
}

func TestPEOrdersSectionsByName(t *testing.T) {
	extra := []*asm.Section{
		{Name: ".zeta", Flags: asm.FlagReadOnly, Align: 1, Data: []byte{1}},
		{Name: ".alpha", Flags: asm.FlagReadOnly, Align: 1, Data: []byte{2}},
	}
	_, file := linkPE(t, peObject(peOptions{extra: extra}))
	want := []string{".text", ".rdata", ".data", ".alpha", ".rodata", ".zeta"}
	if got := peSectionNames(file); strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("sections %v, want %v", got, want)
	}
	if alpha := peSection(t, file, ".alpha"); alpha.Characteristics != 0x40000040 {
		t.Errorf("read-only data characteristics %#x", alpha.Characteristics)
	}
}

func rva(file *pe.File, name string, offset uint32) uint32 {
	for _, s := range file.Sections {
		if s.Name == name {
			return s.VirtualAddress + offset
		}
	}
	return 0
}

func readAt(image []byte, file *pe.File, name string, offset int) []byte {
	for _, s := range file.Sections {
		if s.Name == name {
			return image[int(s.Offset)+offset:]
		}
	}
	return nil
}

func TestPEFixups(t *testing.T) {
	obj := peObject(peOptions{
		textFixups: []asm.Fixup{
			{Offset: 4, Kind: asm.PC32, Symbol: "data_label", Addend: -4},
			{Offset: 12, Kind: asm.PLT32, Symbol: "text_label", Addend: -4},
			{Offset: 20, Kind: asm.ImgRel32, Symbol: "rodata_label", Addend: 8},
		},
		rodataFixups: []asm.Fixup{{Offset: 8, Kind: asm.Abs64, Symbol: "text_label", Addend: 3}},
		dataFixups:   []asm.Fixup{{Offset: 0, Kind: asm.Abs64, Symbol: "constant", Addend: 1}},
	})
	image, file := linkPE(t, obj)
	text := readAt(image, file, ".text", 0)
	textRVA := rva(file, ".text", 0)
	dataLabel := rva(file, ".data", 16)
	if got, want := binary.LittleEndian.Uint32(text[4:]), dataLabel-(textRVA+4)-4; got != want {
		t.Errorf("PC32 got %#x, want %#x", got, want)
	}
	if got, want := binary.LittleEndian.Uint32(text[12:]), textRVA+8-(textRVA+12)-4; got != want {
		t.Errorf("PLT32 got %#x, want %#x", got, want)
	}
	if got, want := binary.LittleEndian.Uint32(text[20:]), rva(file, ".rodata", 4)+8; got != want {
		t.Errorf("ImgRel32 got %#x, want %#x", got, want)
	}
	rodata := readAt(image, file, ".rodata", 0)
	if got, want := binary.LittleEndian.Uint64(rodata[8:]), uint64(peImageBase)+uint64(textRVA)+8+3; got != want {
		t.Errorf("Abs64 got %#x, want %#x", got, want)
	}
	data := readAt(image, file, ".data", 0)
	if got := binary.LittleEndian.Uint64(data); got != 0x1235 {
		t.Errorf("Abs64 against an absolute symbol got %#x, want 0x1235", got)
	}
}

func TestPEBaseRelocations(t *testing.T) {
	obj := peObject(peOptions{
		rodataFixups: []asm.Fixup{
			{Offset: 16, Kind: asm.Abs64, Symbol: "text_label"},
			{Offset: 0, Kind: asm.Abs64, Symbol: "data_label"},
			{Offset: 8, Kind: asm.Abs64, Symbol: "constant"},
		},
		dataFixups: []asm.Fixup{{Offset: 0, Kind: asm.Abs64, Symbol: "text_label"}},
	})
	image, file := linkPE(t, obj)
	reloc := peSection(t, file, ".reloc")
	if reloc.Characteristics != 0x42000040 || reloc != file.Sections[len(file.Sections)-1] {
		t.Errorf(".reloc characteristics %#x, last section %v", reloc.Characteristics, reloc == file.Sections[len(file.Sections)-1])
	}
	header := peHeader(t, file)
	directory := header.DataDirectory[peDirBaseReloc]
	if directory.VirtualAddress != reloc.VirtualAddress || directory.Size != reloc.VirtualSize {
		t.Errorf("base relocation directory %+v, section at %#x size %#x", directory, reloc.VirtualAddress, reloc.VirtualSize)
	}
	data := image[reloc.Offset : reloc.Offset+reloc.VirtualSize]
	type block struct {
		page    uint32
		entries []uint16
	}
	var blocks []block
	for len(data) > 0 {
		size := binary.LittleEndian.Uint32(data[4:])
		b := block{page: binary.LittleEndian.Uint32(data)}
		for i := 8; i < int(size); i += 2 {
			b.entries = append(b.entries, binary.LittleEndian.Uint16(data[i:]))
		}
		blocks = append(blocks, b)
		data = data[size:]
	}
	dataPage := rva(file, ".data", 0) &^ 0xFFF
	rodataPage := rva(file, ".rodata", 0) &^ 0xFFF
	if len(blocks) != 2 || blocks[0].page != dataPage || blocks[1].page != rodataPage {
		t.Fatalf("blocks %+v, want pages %#x and %#x", blocks, dataPage, rodataPage)
	}
	if want := []uint16{10<<12 | uint16(rva(file, ".data", 0)&0xFFF), 0}; len(blocks[0].entries) != 2 || blocks[0].entries[0] != want[0] || blocks[0].entries[1] != 0 {
		t.Errorf("first block entries %#x, want %#x padded with an absolute entry", blocks[0].entries, want)
	}
	rodataBase := rva(file, ".rodata", 0)
	want := []uint16{10<<12 | uint16(rodataBase&0xFFF), 10<<12 | uint16((rodataBase+16)&0xFFF)}
	if len(blocks[1].entries) != 2 || blocks[1].entries[0] != want[0] || blocks[1].entries[1] != want[1] {
		t.Errorf("second block entries %#x, want %#x", blocks[1].entries, want)
	}
}

func TestPEOmitsRelocationsWithoutAbsoluteAddresses(t *testing.T) {
	_, file := linkPE(t, peObject(peOptions{
		textFixups: []asm.Fixup{{Offset: 4, Kind: asm.PC32, Symbol: "data_label", Addend: -4}},
	}))
	for _, s := range file.Sections {
		if s.Name == ".reloc" {
			t.Fatalf("unexpected .reloc section")
		}
	}
	if directory := peHeader(t, file).DataDirectory[peDirBaseReloc]; directory.VirtualAddress != 0 || directory.Size != 0 {
		t.Errorf("base relocation directory %+v", directory)
	}
}

func importSections() []*asm.Section {
	return []*asm.Section{
		{Name: ".idata$2", Flags: asm.FlagReadOnly, Align: 1, Data: make([]byte, 40)},
		{Name: ".idata$3", Flags: asm.FlagReadOnly, Align: 1, Data: make([]byte, 20)},
		{Name: ".idata$4", Flags: asm.FlagReadOnly, Align: 8, Data: make([]byte, 24)},
		{Name: ".idata$5", Flags: asm.FlagReadOnly, Align: 8, Data: make([]byte, 24)},
		{Name: ".idata$6", Flags: asm.FlagReadOnly, Align: 2, Data: make([]byte, 14)},
		{Name: ".idata$7", Flags: asm.FlagReadOnly, Align: 1, Data: make([]byte, 13)},
	}
}

func TestPEImportDirectories(t *testing.T) {
	image, file := linkPE(t, peObject(peOptions{extra: importSections()}))
	header := peHeader(t, file)
	rdata := peSection(t, file, ".rdata")
	if got := header.DataDirectory[peDirDebug]; got.VirtualAddress != rdata.VirtualAddress || got.Size != 28 {
		t.Errorf("debug directory %+v", got)
	}
	imports := header.DataDirectory[peDirImport]
	if imports.VirtualAddress != rdata.VirtualAddress+28 || imports.Size != 40+20 {
		t.Errorf("import directory %+v", imports)
	}
	iat := header.DataDirectory[peDirIAT]
	if iat.Size != 24 || iat.VirtualAddress%8 != 0 {
		t.Errorf("import address table %+v", iat)
	}
	if want := rdata.VirtualAddress + 28 + 40 + 20 + 20; want+(8-want%8)%8 > iat.VirtualAddress {
		t.Errorf("import address table at %#x precedes the lookup table, which starts at %#x", iat.VirtualAddress, want)
	}
	entry := image[rdata.Offset:]
	if repro := binary.LittleEndian.Uint32(entry[12:]); repro != 16 {
		t.Errorf("debug entry type %d, want 16 (reproducible build)", repro)
	}
}

func TestPEStampsContentHash(t *testing.T) {
	first, firstFile := linkPE(t, peObject(peOptions{extra: importSections()}))
	second, _ := linkPE(t, peObject(peOptions{extra: importSections()}))
	if !bytes.Equal(first, second) {
		t.Fatal("linking twice gives different images")
	}
	_, otherFile := linkPE(t, peObject(peOptions{extra: importSections(), dataFixups: []asm.Fixup{{Offset: 0, Kind: asm.Abs64, Symbol: "text_label"}}}))
	if firstFile.TimeDateStamp == 0 || firstFile.TimeDateStamp == otherFile.TimeDateStamp {
		t.Errorf("timestamps %#x and %#x do not follow the content", firstFile.TimeDateStamp, otherFile.TimeDateStamp)
	}
	rdata := peSection(t, firstFile, ".rdata")
	if stamp := binary.LittleEndian.Uint32(first[rdata.Offset+4:]); stamp != firstFile.TimeDateStamp {
		t.Errorf("debug entry stamp %#x, header stamp %#x", stamp, firstFile.TimeDateStamp)
	}
}

func TestPERejections(t *testing.T) {
	cases := []struct {
		name    string
		options peOptions
		want    string
	}{
		{"undefined symbol", peOptions{textFixups: []asm.Fixup{{Offset: 0, Kind: asm.PC32, Symbol: "missing_b"}, {Offset: 8, Kind: asm.PLT32, Symbol: "missing_a"}}}, "undefined symbol missing_a, missing_b"},
		{"missing entry", peOptions{omitEntry: true}, "entry symbol start is not defined"},
		{"absolute 32-bit", peOptions{dataFixups: []asm.Fixup{{Offset: 0, Kind: asm.Abs32, Symbol: "text_label"}}}, "Abs32 relocation against text_label in .data is not supported"},
		{"signed absolute 32-bit", peOptions{dataFixups: []asm.Fixup{{Offset: 0, Kind: asm.Abs32S, Symbol: "text_label"}}}, "Abs32S relocation"},
		{"global offset table", peOptions{textFixups: []asm.Fixup{{Offset: 0, Kind: asm.GOTPCREL, Symbol: "text_label"}}}, "GOTPCREL relocation"},
		{"relocation outside", peOptions{textFixups: []asm.Fixup{{Offset: 62, Kind: asm.PC32, Symbol: "text_label"}}}, "lies outside .text"},
		{"rip-relative range to an absolute symbol", peOptions{textFixups: []asm.Fixup{{Offset: 0, Kind: asm.PC32, Symbol: "constant"}}}, "does not fit in 32 signed bits"},
		{"image relative range", peOptions{textFixups: []asm.Fixup{{Offset: 0, Kind: asm.ImgRel32, Symbol: "text_label", Addend: -1 << 20}}}, "does not fit in 32 unsigned bits"},
		{"thread local", peOptions{extra: []*asm.Section{{Name: ".tbss", Flags: asm.FlagAlloc | asm.FlagWrite | asm.FlagTLS, NoBits: true, Size: 8, Align: 8}}}, "thread-local section .tbss"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := link.PE(peObject(c.options))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("error %v, want mention of %q", err, c.want)
			}
		})
	}
}
