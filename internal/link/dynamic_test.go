package link_test

import (
	"bytes"
	"debug/elf"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lowlang/internal/asm"
	"lowlang/internal/link"
)

type relocation struct {
	offset uint64
	symbol string
	kind   elf.R_X86_64
}

func systemResolver() link.Resolver {
	return link.Resolver{Dirs: link.SystemDirs}
}

func systemLibraries(t *testing.T, names ...string) []*link.Library {
	libraries, err := systemResolver().Resolve(names)
	if err != nil {
		t.Skipf("system libraries are unavailable: %v", err)
	}
	return libraries
}

func importedSymbols(names ...string) []asm.Symbol {
	symbols := make([]asm.Symbol, len(names))
	for i, name := range names {
		symbols[i] = asm.Symbol{Name: name, Section: asm.SectionUndefined, Global: true}
	}
	return symbols
}

func mustLinkDynamic(t *testing.T, obj *asm.Object) *elf.File {
	libraries := systemLibraries(t)
	image, err := link.Dynamic(obj, libraries)
	if err != nil {
		t.Fatalf("link: %v", err)
	}
	file, err := elf.NewFile(bytes.NewReader(image.Bytes))
	if err != nil {
		t.Fatalf("parse linked image: %v", err)
	}
	return file
}

func relocations(t *testing.T, file *elf.File, name string) []relocation {
	data := sectionData(t, file, name)
	symbols, err := file.DynamicSymbols()
	if err != nil {
		t.Fatalf("read dynamic symbols: %v", err)
	}
	var found []relocation
	for at := 0; at+24 <= len(data); at += 24 {
		info := binary.LittleEndian.Uint64(data[at+8:])
		found = append(found, relocation{
			offset: binary.LittleEndian.Uint64(data[at:]),
			symbol: symbols[info>>32-1].Name,
			kind:   elf.R_X86_64(info & 0xffffffff),
		})
	}
	return found
}

func slotOf(t *testing.T, file *elf.File, symbol string) uint64 {
	for _, r := range relocations(t, file, ".rela.dyn") {
		if r.symbol == symbol {
			return r.offset
		}
	}
	t.Fatalf("no GLOB_DAT relocation for %s", symbol)
	return 0
}

func displacement(t *testing.T, file *elf.File, offset int) int64 {
	text := sectionData(t, file, ".text")
	return int64(int32(binary.LittleEndian.Uint32(text[offset:])))
}

func resolvedTarget(t *testing.T, file *elf.File, offset int) uint64 {
	text := file.Section(".text")
	return uint64(int64(text.Addr) + int64(offset) + 4 + displacement(t, file, offset))
}

func within(section *elf.Section, address uint64) bool {
	return section != nil && address >= section.Addr && address < section.Addr+section.Size
}

func TestDynamicGotAndPltRelocations(t *testing.T) {
	obj := fixtureObject(fixtureOptions{
		symbols: importedSymbols("malloc", "free", "qsort"),
		textFixups: []asm.Fixup{
			{Offset: 0, Kind: asm.PLT32, Symbol: "malloc", Addend: -4},
			{Offset: 8, Kind: asm.REXGOTPCRELX, Symbol: "malloc", Addend: -4},
			{Offset: 16, Kind: asm.REXGOTPCRELX, Symbol: "free", Addend: -4},
			{Offset: 24, Kind: asm.GOTPCRELX, Symbol: "free", Addend: -4},
			{Offset: 32, Kind: asm.GOTPCREL, Symbol: "text_label", Addend: -4},
			{Offset: 40, Kind: asm.PLT32, Symbol: "qsort", Addend: -4},
		},
	})
	file := mustLinkDynamic(t, obj)

	for _, symbol := range []string{"malloc", "free"} {
		slotOf(t, file, symbol)
	}
	for _, r := range relocations(t, file, ".rela.dyn") {
		if r.kind != elf.R_X86_64_GLOB_DAT {
			t.Errorf("%s: .rela.dyn relocation %v, want GLOB_DAT", r.symbol, r.kind)
		}
	}
	plt := relocations(t, file, ".rela.plt")
	if len(plt) != 1 || plt[0].symbol != "qsort" || plt[0].kind != elf.R_X86_64_JMP_SLOT {
		t.Fatalf(".rela.plt = %+v, want one JUMP_SLOT for qsort", plt)
	}

	got := file.Section(".got")
	mallocSlot, freeSlot := slotOf(t, file, "malloc"), slotOf(t, file, "free")
	if resolvedTarget(t, file, 8) != mallocSlot {
		t.Errorf("malloc GOT reference resolves to %#x, want slot %#x", resolvedTarget(t, file, 8), mallocSlot)
	}
	if resolvedTarget(t, file, 16) != freeSlot || resolvedTarget(t, file, 24) != freeSlot {
		t.Errorf("free GOT references resolve to %#x and %#x, want shared slot %#x", resolvedTarget(t, file, 16), resolvedTarget(t, file, 24), freeSlot)
	}
	if !within(got, mallocSlot) || !within(got, freeSlot) {
		t.Errorf("slots %#x and %#x are outside .got %#x+%#x", mallocSlot, freeSlot, got.Addr, got.Size)
	}

	localSlot := resolvedTarget(t, file, 32)
	if !within(got, localSlot) {
		t.Fatalf("local slot %#x is outside .got", localSlot)
	}
	slots := sectionData(t, file, ".got")
	held := binary.LittleEndian.Uint64(slots[localSlot-got.Addr:])
	if want := symbolAddress(t, file, "text_label"); held != want {
		t.Errorf("local GOT slot holds %#x, want the address of text_label %#x", held, want)
	}

	if target := resolvedTarget(t, file, 0); !within(file.Section(".plt.got"), target) {
		t.Errorf("PLT32 to a symbol with a GOT slot resolves to %#x, want an entry in .plt.got", target)
	}
	target := resolvedTarget(t, file, 40)
	if !within(file.Section(".plt"), target) || target == file.Section(".plt").Addr {
		t.Errorf("PLT32 to qsort resolves to %#x, want a lazy entry in .plt past the header", target)
	}
}

func TestDynamicPltSlotHoldsLazyEntry(t *testing.T) {
	obj := fixtureObject(fixtureOptions{
		symbols:    importedSymbols("qsort"),
		textFixups: []asm.Fixup{{Offset: 0, Kind: asm.PLT32, Symbol: "qsort", Addend: -4}},
	})
	file := mustLinkDynamic(t, obj)
	plt := relocations(t, file, ".rela.plt")
	if len(plt) != 1 {
		t.Fatalf(".rela.plt has %d entries, want 1", len(plt))
	}
	gotPlt := file.Section(".got.plt")
	data := sectionData(t, file, ".got.plt")
	held := binary.LittleEndian.Uint64(data[plt[0].offset-gotPlt.Addr:])
	entry := resolvedTarget(t, file, 0)
	if held != entry+6 {
		t.Errorf("JUMP_SLOT slot holds %#x, want the lazy stub %#x", held, entry+6)
	}
}

func TestDynamicUndefinedSymbolNamesLibraries(t *testing.T) {
	obj := fixtureObject(fixtureOptions{
		symbols:    importedSymbols("no_such_function_anywhere"),
		textFixups: []asm.Fixup{{Offset: 0, Kind: asm.PLT32, Symbol: "no_such_function_anywhere", Addend: -4}},
	})
	_, err := link.Dynamic(obj, systemLibraries(t))
	if err == nil {
		t.Fatal("link succeeded with an unresolvable symbol")
	}
	for _, want := range []string{"no_such_function_anywhere", "libc.so.6"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %s", err, want)
		}
	}
}

func TestDynamicRejectsCopyRelocation(t *testing.T) {
	obj := fixtureObject(fixtureOptions{
		symbols: importedSymbols("environ"),
		textFixups: []asm.Fixup{
			{Offset: 0, Kind: asm.REXGOTPCRELX, Symbol: "environ", Addend: -4},
			{Offset: 8, Kind: asm.PC32, Symbol: "environ", Addend: -4},
		},
	})
	_, err := link.Dynamic(obj, systemLibraries(t))
	if err == nil || !strings.Contains(err.Error(), "environ") || !strings.Contains(err.Error(), "copy relocation") {
		t.Fatalf("error %v, want one rejecting a copy relocation against environ", err)
	}
}

func TestDynamicRecordsLibrariesInLinkOrder(t *testing.T) {
	obj := fixtureObject(fixtureOptions{})
	libraries := systemLibraries(t, "m", "c")
	image, err := link.Dynamic(obj, libraries)
	if err != nil {
		t.Fatal(err)
	}
	file, err := elf.NewFile(bytes.NewReader(image.Bytes))
	if err != nil {
		t.Fatal(err)
	}
	needed, err := file.ImportedLibraries()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(needed, " "), "libm.so.6 libc.so.6"; got != want {
		t.Errorf("DT_NEEDED = %q, want %q", got, want)
	}
}

func TestResolveMapsRuntimeLibrariesAndPutsLibcLast(t *testing.T) {
	libraries := systemLibraries(t, "c", "m", "m")
	sonames := make([]string, len(libraries))
	for i, library := range libraries {
		sonames[i] = library.Soname
	}
	if got, want := strings.Join(sonames, " "), "libm.so.6 libc.so.6"; got != want {
		t.Errorf("sonames = %q, want %q", got, want)
	}
}

func TestResolveReadsSonameThroughSymlink(t *testing.T) {
	target := systemLibraries(t, "m")[0].Path
	dir := t.TempDir()
	if err := os.Symlink(target, filepath.Join(dir, "libfoo.so")); err != nil {
		t.Fatal(err)
	}
	resolver := link.Resolver{Dirs: append([]string{dir}, link.SystemDirs...)}
	libraries, err := resolver.Resolve([]string{"foo"})
	if err != nil {
		t.Fatal(err)
	}
	if libraries[0].Soname != "libm.so.6" {
		t.Errorf("soname = %q, want libm.so.6", libraries[0].Soname)
	}
	if libraries[0].Path != target {
		t.Errorf("path = %q, want the symlink target %q", libraries[0].Path, target)
	}
}

func TestResolveMissingLibrary(t *testing.T) {
	_, err := systemResolver().Resolve([]string{"definitely_not_installed"})
	if err == nil {
		t.Fatal("resolved a library that does not exist")
	}
	for _, want := range []string{"definitely_not_installed", "libdefinitely_not_installed.so"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %s", err, want)
		}
	}
}

func TestResolveRejectsNonSharedObject(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "libbad.so"), []byte("not an elf file"), 0o644); err != nil {
		t.Fatal(err)
	}
	resolver := link.Resolver{Dirs: []string{dir}}
	_, err := resolver.Resolve([]string{"bad"})
	if err == nil || !strings.Contains(err.Error(), "not a shared object") {
		t.Fatalf("error %v, want one rejecting a non shared object", err)
	}
}
