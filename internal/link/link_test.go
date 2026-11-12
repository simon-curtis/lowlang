package link_test

import (
	"bytes"
	"debug/elf"
	"encoding/binary"
	"math"
	"strings"
	"testing"

	"lowlang/internal/asm"
	"lowlang/internal/link"
)

const (
	textIndex = iota
	rodataIndex
	dataIndex
	bssIndex
	tbssIndex
)

type fixtureOptions struct {
	textFixups []asm.Fixup
	dataFixups []asm.Fixup
	symbols    []asm.Symbol
	bssSize    int64
}

func fixtureObject(options fixtureOptions) *asm.Object {
	bssSize := options.bssSize
	if bssSize == 0 {
		bssSize = 64
	}
	text := &asm.Section{Name: ".text", Flags: asm.FlagAlloc | asm.FlagExec, Align: 16, Data: make([]byte, 64), Fixups: options.textFixups}
	rodata := &asm.Section{Name: ".rodata", Flags: asm.FlagAlloc, Align: 8, Data: []byte{1, 2, 3, 4, 5, 6, 7, 8}}
	data := &asm.Section{Name: ".data", Flags: asm.FlagAlloc | asm.FlagWrite, Align: 8, Data: make([]byte, 32), Fixups: options.dataFixups}
	bss := &asm.Section{Name: ".bss", Flags: asm.FlagAlloc | asm.FlagWrite, Align: 8, NoBits: true, Size: bssSize}
	tbss := &asm.Section{Name: ".tbss", Flags: asm.FlagAlloc | asm.FlagWrite | asm.FlagTLS, Align: 8, NoBits: true, Size: 24}
	symbols := []asm.Symbol{
		{Name: link.EntrySymbol, Section: textIndex, Global: true},
		{Name: "text_label", Section: textIndex, Value: 8},
		{Name: "rodata_label", Section: rodataIndex, Value: 4},
		{Name: "data_label", Section: dataIndex, Value: 16, Global: true},
		{Name: "bss_label", Section: bssIndex, Value: 8},
		{Name: "tls_label", Section: tbssIndex, Value: 0},
		{Name: "constant", Section: asm.SectionAbsolute, Value: 0x1234},
		{Name: ".Lhidden", Section: textIndex, Value: 2},
	}
	symbols = append(symbols, options.symbols...)
	return &asm.Object{Sections: []*asm.Section{text, rodata, data, bss, tbss}, Symbols: symbols}
}

func mustLink(t *testing.T, obj *asm.Object) *elf.File {
	image, err := link.Static(obj)
	if err != nil {
		t.Fatalf("link: %v", err)
	}
	file, err := elf.NewFile(bytes.NewReader(image))
	if err != nil {
		t.Fatalf("parse linked image: %v", err)
	}
	return file
}

func sectionData(t *testing.T, file *elf.File, name string) []byte {
	section := file.Section(name)
	if section == nil {
		t.Fatalf("section %s is missing", name)
	}
	data, err := section.Data()
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return data
}

func symbolAddress(t *testing.T, file *elf.File, name string) uint64 {
	symbols, err := file.Symbols()
	if err != nil {
		t.Fatalf("read symbols: %v", err)
	}
	for _, sym := range symbols {
		if sym.Name == name {
			return sym.Value
		}
	}
	t.Fatalf("symbol %s is missing", name)
	return 0
}

func TestAbs64(t *testing.T) {
	obj := fixtureObject(fixtureOptions{dataFixups: []asm.Fixup{
		{Offset: 0, Kind: asm.Abs64, Symbol: "text_label", Addend: 3},
		{Offset: 8, Kind: asm.Abs64, Symbol: "constant", Addend: -1},
	}})
	file := mustLink(t, obj)
	data := sectionData(t, file, ".data")
	want := symbolAddress(t, file, "text_label") + 3
	if got := binary.LittleEndian.Uint64(data[0:]); got != want {
		t.Errorf("Abs64 against a label: got %#x, want %#x", got, want)
	}
	if got := binary.LittleEndian.Uint64(data[8:]); got != 0x1233 {
		t.Errorf("Abs64 against a constant: got %#x, want 0x1233", got)
	}
}

func TestAbs32(t *testing.T) {
	obj := fixtureObject(fixtureOptions{dataFixups: []asm.Fixup{
		{Offset: 0, Kind: asm.Abs32, Symbol: "rodata_label", Addend: 1},
	}})
	file := mustLink(t, obj)
	data := sectionData(t, file, ".data")
	want := symbolAddress(t, file, "rodata_label") + 1
	if got := uint64(binary.LittleEndian.Uint32(data)); got != want {
		t.Errorf("Abs32: got %#x, want %#x", got, want)
	}
}

func TestAbs32Overflow(t *testing.T) {
	cases := []struct {
		name  string
		value int64
		fails bool
	}{
		{"largest", math.MaxUint32, false},
		{"too large", math.MaxUint32 + 1, true},
		{"negative", -1, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			obj := fixtureObject(fixtureOptions{
				symbols:    []asm.Symbol{{Name: "wide", Section: asm.SectionAbsolute, Value: c.value}},
				dataFixups: []asm.Fixup{{Offset: 0, Kind: asm.Abs32, Symbol: "wide"}},
			})
			_, err := link.Static(obj)
			if (err != nil) != c.fails {
				t.Fatalf("error %v, want failure %v", err, c.fails)
			}
			if c.fails && !strings.Contains(err.Error(), "wide") {
				t.Errorf("error %q does not name the symbol", err)
			}
		})
	}
}

func TestAbs32S(t *testing.T) {
	cases := []struct {
		name  string
		value int64
		fails bool
	}{
		{"negative", -5, false},
		{"smallest", math.MinInt32, false},
		{"largest", math.MaxInt32, false},
		{"too large", math.MaxInt32 + 1, true},
		{"too small", math.MinInt32 - 1, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			obj := fixtureObject(fixtureOptions{
				symbols:    []asm.Symbol{{Name: "signed", Section: asm.SectionAbsolute, Value: c.value}},
				dataFixups: []asm.Fixup{{Offset: 4, Kind: asm.Abs32S, Symbol: "signed"}},
			})
			image, err := link.Static(obj)
			if (err != nil) != c.fails {
				t.Fatalf("error %v, want failure %v", err, c.fails)
			}
			if c.fails {
				return
			}
			file, err := elf.NewFile(bytes.NewReader(image))
			if err != nil {
				t.Fatal(err)
			}
			data := sectionData(t, file, ".data")
			if got := int64(int32(binary.LittleEndian.Uint32(data[4:]))); got != c.value {
				t.Errorf("Abs32S: got %d, want %d", got, c.value)
			}
		})
	}
}

func TestPC32AndPLT32(t *testing.T) {
	for _, kind := range []asm.FixupKind{asm.PC32, asm.PLT32} {
		t.Run(kind.String(), func(t *testing.T) {
			obj := fixtureObject(fixtureOptions{textFixups: []asm.Fixup{
				{Offset: 16, Kind: kind, Symbol: "data_label", Addend: -4},
				{Offset: 32, Kind: kind, Symbol: "text_label", Addend: -4},
			}})
			file := mustLink(t, obj)
			text := file.Section(".text")
			data := sectionData(t, file, ".text")
			forward := int32(binary.LittleEndian.Uint32(data[16:]))
			wantForward := int64(symbolAddress(t, file, "data_label")) - 4 - int64(text.Addr+16)
			if int64(forward) != wantForward {
				t.Errorf("forward displacement: got %d, want %d", forward, wantForward)
			}
			backward := int32(binary.LittleEndian.Uint32(data[32:]))
			wantBackward := int64(symbolAddress(t, file, "text_label")) - 4 - int64(text.Addr+32)
			if int64(backward) != wantBackward {
				t.Errorf("backward displacement: got %d, want %d", backward, wantBackward)
			}
		})
	}
}

func TestPC32Overflow(t *testing.T) {
	obj := fixtureObject(fixtureOptions{
		bssSize: 3 << 30,
		symbols: []asm.Symbol{{Name: "far", Section: bssIndex, Value: 3<<30 - 8}},
		textFixups: []asm.Fixup{
			{Offset: 0, Kind: asm.PC32, Symbol: "far", Addend: -4},
		},
	})
	_, err := link.Static(obj)
	if err == nil || !strings.Contains(err.Error(), "far") {
		t.Fatalf("error %v, want an overflow naming far", err)
	}
}

func TestUnsupportedFixup(t *testing.T) {
	for _, kind := range []asm.FixupKind{asm.GOTPCREL, asm.GOTPCRELX, asm.REXGOTPCRELX, asm.ImgRel32} {
		t.Run(kind.String(), func(t *testing.T) {
			obj := fixtureObject(fixtureOptions{textFixups: []asm.Fixup{{Offset: 0, Kind: kind, Symbol: "text_label"}}})
			_, err := link.Static(obj)
			if err == nil || !strings.Contains(err.Error(), kind.String()) {
				t.Fatalf("error %v, want one naming %s", err, kind)
			}
		})
	}
}

func TestUndefinedSymbols(t *testing.T) {
	obj := fixtureObject(fixtureOptions{
		symbols: []asm.Symbol{
			{Name: "zeta", Section: asm.SectionUndefined, Global: true},
			{Name: "alpha", Section: asm.SectionUndefined, Global: true},
		},
		textFixups: []asm.Fixup{
			{Offset: 0, Kind: asm.PC32, Symbol: "zeta", Addend: -4},
			{Offset: 8, Kind: asm.PC32, Symbol: "alpha", Addend: -4},
			{Offset: 16, Kind: asm.PC32, Symbol: "zeta", Addend: -4},
		},
	})
	_, err := link.Static(obj)
	if err == nil {
		t.Fatal("link succeeded with undefined symbols")
	}
	want := "link: undefined symbol alpha, zeta"
	if err.Error() != want {
		t.Errorf("error %q, want %q", err, want)
	}
}

func TestUnreferencedUndefinedSymbolIsAllowed(t *testing.T) {
	obj := fixtureObject(fixtureOptions{
		symbols: []asm.Symbol{{Name: "unused", Section: asm.SectionUndefined, Global: true}},
	})
	mustLink(t, obj)
}

func TestMissingEntry(t *testing.T) {
	obj := fixtureObject(fixtureOptions{})
	obj.Symbols[0].Name = "begin"
	_, err := link.Static(obj)
	if err == nil || !strings.Contains(err.Error(), link.EntrySymbol) {
		t.Fatalf("error %v, want one naming %s", err, link.EntrySymbol)
	}
}

func TestStructure(t *testing.T) {
	file := mustLink(t, fixtureObject(fixtureOptions{}))
	wantTypes := []elf.ProgType{elf.PT_LOAD, elf.PT_LOAD, elf.PT_LOAD, elf.PT_LOAD, elf.PT_TLS, elf.PT_GNU_STACK}
	wantFlags := []elf.ProgFlag{elf.PF_R, elf.PF_R | elf.PF_X, elf.PF_R, elf.PF_R | elf.PF_W, elf.PF_R, elf.PF_R | elf.PF_W}
	if len(file.Progs) != len(wantTypes) {
		t.Fatalf("got %d program headers, want %d", len(file.Progs), len(wantTypes))
	}
	for i, prog := range file.Progs {
		if prog.Type != wantTypes[i] || prog.Flags != wantFlags[i] {
			t.Errorf("program header %d: got %v %v, want %v %v", i, prog.Type, prog.Flags, wantTypes[i], wantFlags[i])
		}
		if prog.Type == elf.PT_LOAD && (prog.Off%0x1000 != prog.Vaddr%0x1000 || prog.Align != 0x1000) {
			t.Errorf("program header %d: offset %#x and address %#x are not congruent", i, prog.Off, prog.Vaddr)
		}
	}
	if file.Progs[0].Vaddr != link.BaseAddress {
		t.Errorf("image starts at %#x, want %#x", file.Progs[0].Vaddr, link.BaseAddress)
	}
	text := file.Section(".text")
	if file.Entry != text.Addr {
		t.Errorf("entry %#x, want the start of .text %#x", file.Entry, text.Addr)
	}
	names := []string{"", ".text", ".rodata", ".tbss", ".data", ".bss", ".symtab", ".strtab", ".shstrtab"}
	if len(file.Sections) != len(names) {
		t.Fatalf("got %d sections, want %d", len(file.Sections), len(names))
	}
	for i, name := range names {
		if file.Sections[i].Name != name {
			t.Errorf("section %d is %q, want %q", i, file.Sections[i].Name, name)
		}
	}
}

func TestSymbolTable(t *testing.T) {
	file := mustLink(t, fixtureObject(fixtureOptions{}))
	symbols, err := file.Symbols()
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]elf.Symbol{}
	for _, sym := range symbols {
		byName[sym.Name] = sym
	}
	if _, found := byName[".Lhidden"]; found {
		t.Error("a .L symbol reached the symbol table")
	}
	for _, name := range []string{"text_label", "rodata_label", "bss_label", "tls_label", "constant", "_start", "data_label", "_end", "_edata", "__bss_start"} {
		if _, found := byName[name]; !found {
			t.Errorf("symbol %s is missing", name)
		}
	}
	if sym := byName["constant"]; sym.Section != elf.SHN_ABS || sym.Value != 0x1234 {
		t.Errorf("constant: got section %v value %#x", sym.Section, sym.Value)
	}
	if sym := byName["tls_label"]; elf.ST_TYPE(sym.Info) != elf.STT_TLS || sym.Value != 0 {
		t.Errorf("tls_label: got type %v value %#x", elf.ST_TYPE(sym.Info), sym.Value)
	}
	if elf.ST_BIND(byName["text_label"].Info) != elf.STB_LOCAL || elf.ST_BIND(byName["_start"].Info) != elf.STB_GLOBAL {
		t.Error("symbol binding is wrong")
	}
	bss := file.Section(".bss")
	if got := byName["_end"].Value; got != bss.Addr+bss.Size {
		t.Errorf("_end is %#x, want %#x", got, bss.Addr+bss.Size)
	}
	if got := byName["__bss_start"].Value; got != bss.Addr {
		t.Errorf("__bss_start is %#x, want %#x", got, bss.Addr)
	}
}

func TestSectionAlignment(t *testing.T) {
	obj := fixtureObject(fixtureOptions{})
	obj.Sections[rodataIndex].Align = 64
	obj.Sections[dataIndex].Align = 128
	file := mustLink(t, obj)
	for _, section := range file.Sections {
		if section.Flags&elf.SHF_ALLOC == 0 || section.Addralign <= 1 {
			continue
		}
		if section.Addr%section.Addralign != 0 || section.Offset%section.Addralign != 0 {
			t.Errorf("%s at %#x/%#x is not aligned to %d", section.Name, section.Addr, section.Offset, section.Addralign)
		}
	}
}

func TestDeterministic(t *testing.T) {
	build := func() []byte {
		obj := fixtureObject(fixtureOptions{dataFixups: []asm.Fixup{{Offset: 0, Kind: asm.Abs64, Symbol: "text_label"}}})
		image, err := link.Static(obj)
		if err != nil {
			t.Fatal(err)
		}
		return image
	}
	if !bytes.Equal(build(), build()) {
		t.Error("two links of the same object differ")
	}
}

func TestEmptySectionsAreDropped(t *testing.T) {
	obj := fixtureObject(fixtureOptions{})
	obj.Sections[bssIndex].Size = 0
	obj.Sections[tbssIndex].Size = 0
	obj.Symbols = obj.Symbols[:4]
	file := mustLink(t, obj)
	for _, name := range []string{".bss", ".tbss"} {
		if file.Section(name) != nil {
			t.Errorf("empty section %s was kept", name)
		}
	}
	for _, prog := range file.Progs {
		if prog.Type == elf.PT_TLS {
			t.Error("a TLS segment was emitted without TLS sections")
		}
	}
}
