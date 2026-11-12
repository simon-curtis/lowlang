package link_test

import (
	"bytes"
	"context"
	"debug/elf"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"lowlang/internal/asm"
	"lowlang/internal/link"
	"lowlang/internal/oracletool"
	"lowlang/internal/target"
)

const noUnwindSection = ".eh_frame"

var loaderPath = link.Interpreter

func dynamicPrograms(t *testing.T) []string {
	matches, err := filepath.Glob(filepath.Join(repoRoot(), "testdata/run/*.needed"))
	if err != nil {
		t.Fatalf("glob programs: %v", err)
	}
	paths := make([]string, 0, len(matches))
	for _, match := range matches {
		rel, err := filepath.Rel(repoRoot(), strings.TrimSuffix(match, ".needed")+".low")
		if err != nil {
			t.Fatalf("relative path of %s: %v", match, err)
		}
		paths = append(paths, rel)
	}
	slices.Sort(paths)
	return paths
}

func linkDynamicWithLd(dir, source string, libraries []string) (string, error) {
	sourcePath := filepath.Join(dir, "main.s")
	text := fmt.Sprintf("%s%s", source, stackNote)
	if err := os.WriteFile(sourcePath, []byte(text), 0o644); err != nil {
		return "", fmt.Errorf("write source: %w", err)
	}
	object := filepath.Join(dir, "main.o")
	if out, err := exec.Command("as", oracletool.BranchFlag, noUsedNoteFlag, "-o", object, sourcePath).CombinedOutput(); err != nil {
		return "", fmt.Errorf("as: %w\n%s", err, out)
	}
	output := filepath.Join(dir, "main.ld")
	args := []string{"-o", output, "-dynamic-linker", loaderPath, object}
	for _, name := range libraries {
		if name != "c" {
			args = append(args, fmt.Sprintf("-l%s", name))
		}
	}
	args = append(args, "-lc")
	if out, err := exec.Command("ld", args...).CombinedOutput(); err != nil {
		return "", fmt.Errorf("ld: %w\n%s", err, out)
	}
	return output, nil
}

func importOrder(path string) ([]string, error) {
	file, err := elf.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	symbols, err := file.DynamicSymbols()
	if err != nil {
		return nil, err
	}
	names := make([]string, len(symbols))
	for i, sym := range symbols {
		names[i] = sym.Name
	}
	return names, nil
}

func orderImports(obj *asm.Object, names []string) {
	rank := map[string]int{}
	for i, name := range names {
		rank[name] = i
	}
	var slots []int
	var moved []asm.Symbol
	for i, sym := range obj.Symbols {
		if _, ranked := rank[sym.Name]; ranked && sym.Section == asm.SectionUndefined {
			slots = append(slots, i)
			moved = append(moved, sym)
		}
	}
	slices.SortFunc(moved, func(a, b asm.Symbol) int { return rank[a.Name] - rank[b.Name] })
	for i, slot := range slots {
		obj.Symbols[slot] = moved[i]
	}
}

func linkDynamicInProcess(dir, source string, libraries []string, order []string) (string, error) {
	object, err := asm.Assemble(source, asm.Options{AlignBranches: true})
	if err != nil {
		return "", fmt.Errorf("assemble: %w", err)
	}
	if order != nil {
		orderImports(object, order)
	}
	resolver := link.Resolver{Dirs: link.SystemDirs}
	resolved, err := resolver.Resolve(libraries)
	if err != nil {
		return "", err
	}
	image, err := link.Dynamic(object, resolved)
	if err != nil {
		return "", err
	}
	output := filepath.Join(dir, "main.low")
	if err := os.WriteFile(output, image, 0o755); err != nil {
		return "", err
	}
	return output, nil
}

func TestDynamicMatchesGNULd(t *testing.T) {
	requireTools(t)
	for _, path := range dynamicPrograms(t) {
		t.Run(path, func(t *testing.T) {
			source, libraries, err := generate(path, target.Linux)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			dir := t.TempDir()
			want, err := linkDynamicWithLd(dir, source, libraries)
			if err != nil {
				t.Fatal(err)
			}
			order, err := importOrder(want)
			if err != nil {
				t.Fatal(err)
			}
			got, err := linkDynamicInProcess(dir, source, libraries, order)
			if err != nil {
				t.Fatal(err)
			}
			compareDynamicExecutables(t, got, want)
		})
	}
}

type dynamicComparison struct {
	t         *testing.T
	got       *elf.File
	want      *elf.File
	gotBytes  []byte
	wantBytes []byte
}

func compareDynamicExecutables(t *testing.T, gotPath, wantPath string) {
	got, err := elf.Open(gotPath)
	if err != nil {
		t.Fatalf("open linked output: %v", err)
	}
	defer got.Close()
	want, err := elf.Open(wantPath)
	if err != nil {
		t.Fatalf("open ld output: %v", err)
	}
	defer want.Close()
	c := &dynamicComparison{t: t, got: got, want: want, gotBytes: readFile(t, gotPath), wantBytes: readFile(t, wantPath)}
	c.compareHeader()
	c.comparePrograms()
	c.compareSections()
	c.compareTags()
	c.compareImports()
	c.compareContents()
	c.compareSymbols()
}

func (c *dynamicComparison) compareHeader() {
	if c.got.Type != c.want.Type || c.got.Machine != c.want.Machine || c.got.Entry != c.want.Entry {
		c.t.Errorf("header: got type %v machine %v entry %#x, want type %v machine %v entry %#x",
			c.got.Type, c.got.Machine, c.got.Entry, c.want.Type, c.want.Machine, c.want.Entry)
	}
}

func (c *dynamicComparison) comparePrograms() {
	if len(c.got.Progs) != len(c.want.Progs) {
		c.t.Errorf("program headers: got %d, want %d", len(c.got.Progs), len(c.want.Progs))
		return
	}
	headerLoad := true
	for i, want := range c.want.Progs {
		got := c.got.Progs[i]
		g, w := got.ProgHeader, want.ProgHeader
		if w.Type == elf.PT_LOAD && headerLoad {
			headerLoad = false
			g.Filesz, g.Memsz, w.Filesz, w.Memsz = 0, 0, 0, 0
		}
		if g != w {
			c.t.Errorf("program header %d: got %+v, want %+v", i, got.ProgHeader, want.ProgHeader)
		}
	}
}

func withoutUnwind(sections []*elf.Section) []*elf.Section {
	kept := make([]*elf.Section, 0, len(sections))
	for _, s := range sections {
		if s.Name != noUnwindSection {
			kept = append(kept, s)
		}
	}
	return kept
}

func (c *dynamicComparison) linkName(file *elf.File, index uint32) string {
	if int(index) >= len(file.Sections) {
		return ""
	}
	return file.Sections[index].Name
}

func (c *dynamicComparison) compareSections() {
	got, want := withoutUnwind(c.got.Sections), withoutUnwind(c.want.Sections)
	if len(got) != len(want) {
		c.t.Errorf("sections: got %d, want %d", len(got), len(want))
		return
	}
	for i, w := range want {
		g := got[i]
		c.compareSection(g, w)
	}
}

func (c *dynamicComparison) compareSection(got, want *elf.Section) {
	if got.Name != want.Name || got.Type != want.Type || got.Flags != want.Flags || got.Addralign != want.Addralign || got.Entsize != want.Entsize {
		c.t.Errorf("section %s: got %+v, want %+v", want.Name, got.SectionHeader, want.SectionHeader)
		return
	}
	if c.linkName(c.got, got.Link) != c.linkName(c.want, want.Link) {
		c.t.Errorf("section %s: links to %s, want %s", want.Name, c.linkName(c.got, got.Link), c.linkName(c.want, want.Link))
	}
	if want.Flags&elf.SHF_INFO_LINK != 0 {
		if c.linkName(c.got, got.Info) != c.linkName(c.want, want.Info) {
			c.t.Errorf("section %s: info names %s, want %s", want.Name, c.linkName(c.got, got.Info), c.linkName(c.want, want.Info))
		}
	}
	if want.Flags&elf.SHF_ALLOC == 0 {
		return
	}
	if want.Name != ".dynstr" && got.Size != want.Size {
		c.t.Errorf("section %s: size %#x, want %#x", want.Name, got.Size, want.Size)
	}
	if c.inHeaderSegment(want) {
		return
	}
	if got.Addr != want.Addr || got.Offset != want.Offset {
		c.t.Errorf("section %s: at %#x/%#x, want %#x/%#x", want.Name, got.Addr, got.Offset, want.Addr, want.Offset)
	}
}

func (c *dynamicComparison) inHeaderSegment(s *elf.Section) bool {
	for _, p := range c.want.Progs {
		if p.Type == elf.PT_LOAD {
			return s.Addr >= p.Vaddr && s.Addr < p.Vaddr+p.Memsz && s.Addr < 0x401000
		}
	}
	return false
}

type tagEntry struct {
	tag   elf.DynTag
	value uint64
}

func readTags(t *testing.T, file *elf.File) []tagEntry {
	data, err := file.Section(".dynamic").Data()
	if err != nil {
		t.Fatalf("read .dynamic: %v", err)
	}
	var tags []tagEntry
	for at := 0; at+16 <= len(data); at += 16 {
		tag := elf.DynTag(binary.LittleEndian.Uint64(data[at:]))
		value := binary.LittleEndian.Uint64(data[at+8:])
		tags = append(tags, tagEntry{tag, value})
		if tag == elf.DT_NULL {
			break
		}
	}
	return tags
}

func (c *dynamicComparison) compareTags() {
	got, want := readTags(c.t, c.got), readTags(c.t, c.want)
	if len(got) != len(want) {
		c.t.Errorf("dynamic tags: got %d, want %d", len(got), len(want))
		return
	}
	exact := map[elf.DynTag]bool{
		elf.DT_PLTGOT: true, elf.DT_DEBUG: true, elf.DT_SYMENT: true, elf.DT_PLTREL: true,
		elf.DT_PLTRELSZ: true, elf.DT_RELASZ: true, elf.DT_RELAENT: true, elf.DT_VERNEEDNUM: true,
	}
	for i, w := range want {
		g := got[i]
		if g.tag != w.tag || exact[w.tag] && g.value != w.value {
			c.t.Errorf("dynamic tag %d: got %v %#x, want %v %#x", i, g.tag, g.value, w.tag, w.value)
		}
	}
	gotNeeded, _ := c.got.ImportedLibraries()
	wantNeeded, _ := c.want.ImportedLibraries()
	if !slices.Equal(gotNeeded, wantNeeded) {
		c.t.Errorf("needed libraries: got %v, want %v", gotNeeded, wantNeeded)
	}
}

func (c *dynamicComparison) compareImports() {
	got, err := c.got.DynamicSymbols()
	if err != nil {
		c.t.Fatalf("read dynamic symbols: %v", err)
	}
	want, err := c.want.DynamicSymbols()
	if err != nil {
		c.t.Fatalf("read ld dynamic symbols: %v", err)
	}
	if len(got) != len(want) {
		c.t.Errorf("dynamic symbols: got %d, want %d", len(got), len(want))
		return
	}
	for i, w := range want {
		g := got[i]
		if g.Name != w.Name || g.Info != w.Info || g.Section != w.Section || g.Version != w.Version || g.Library != w.Library {
			c.t.Errorf("dynamic symbol %d: got %+v, want %+v", i, g, w)
		}
	}
}

func (c *dynamicComparison) compareContents() {
	for _, name := range []string{".hash", ".gnu.hash", ".rela.dyn", ".rela.plt", ".plt", ".plt.got", ".text", ".rodata", ".got", ".got.plt", ".data"} {
		want := c.want.Section(name)
		got := c.got.Section(name)
		if want == nil || got == nil {
			if want != got {
				c.t.Errorf("section %s present in only one output", name)
			}
			continue
		}
		wantData, err := want.Data()
		if err != nil {
			c.t.Fatalf("read ld %s: %v", name, err)
		}
		gotData, err := got.Data()
		if err != nil {
			c.t.Fatalf("read %s: %v", name, err)
		}
		if !bytes.Equal(gotData, wantData) {
			c.t.Errorf("section %s differs from ld at byte %d", name, firstDifference(gotData, wantData))
		}
	}
}

type dynamicSymbolView struct {
	value   uint64
	info    byte
	other   byte
	section string
}

func (c *dynamicComparison) viewSymbols(file *elf.File) map[string]dynamicSymbolView {
	symbols, err := file.Symbols()
	if err != nil {
		c.t.Fatalf("read symbols: %v", err)
	}
	views := make(map[string]dynamicSymbolView, len(symbols))
	for _, sym := range symbols {
		if elf.ST_TYPE(sym.Info) == elf.STT_FILE {
			continue
		}
		view := dynamicSymbolView{value: sym.Value, info: sym.Info, other: sym.Other}
		switch {
		case sym.Section == elf.SHN_ABS:
			view.section = "*ABS*"
		case sym.Section < elf.SHN_LORESERVE:
			view.section = file.Sections[sym.Section].Name
		}
		views[sym.Name] = view
	}
	return views
}

func (c *dynamicComparison) compareSymbols() {
	got, want := c.viewSymbols(c.got), c.viewSymbols(c.want)
	if len(got) != len(want) {
		c.t.Errorf("symbols: got %d, want %d", len(got), len(want))
	}
	var problems []string
	for name, w := range want {
		g, ok := got[name]
		if !ok {
			problems = append(problems, fmt.Sprintf("%s: missing", name))
			continue
		}
		if g != w {
			problems = append(problems, fmt.Sprintf("%s: got %+v, want %+v", name, g, w))
		}
	}
	slices.Sort(problems)
	for i, problem := range problems {
		if i == 5 {
			c.t.Errorf("... %d more symbol mismatches", len(problems)-5)
			break
		}
		c.t.Error(problem)
	}
}

func TestDynamicProgramsLoad(t *testing.T) {
	for _, tool := range []string{"ldd"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is not on PATH", tool)
		}
	}
	if _, err := os.Stat(loaderPath); err != nil {
		t.Skipf("no dynamic loader at %s", loaderPath)
	}
	for _, path := range dynamicPrograms(t) {
		t.Run(path, func(t *testing.T) {
			source, libraries, err := generate(path, target.Linux)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			dir := t.TempDir()
			binary, err := linkDynamicInProcess(dir, source, libraries, nil)
			if err != nil {
				t.Fatal(err)
			}
			checkLibrariesResolve(t, binary)
			checkEveryImportBinds(t, binary)
		})
	}
}

func checkLibrariesResolve(t *testing.T, binary string) {
	out, err := exec.Command("ldd", binary).CombinedOutput()
	if err != nil {
		t.Fatalf("ldd: %v\n%s", err, out)
	}
	if strings.Contains(string(out), "not found") {
		t.Errorf("ldd reports a missing library:\n%s", out)
	}
}

func checkEveryImportBinds(t *testing.T, binary string) {
	imports, err := importOrder(binary)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary)
	cmd.Env = append(os.Environ(), "LD_BIND_NOW=1", "LD_DEBUG=bindings")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Run()
	bound := boundSymbols(stderr.String(), binary)
	for _, name := range imports {
		if name != "" && !bound[name] {
			t.Errorf("imported symbol %s was not bound by the loader", name)
		}
	}
}

func boundSymbols(log, binary string) map[string]bool {
	prefix := fmt.Sprintf("binding file %s [", binary)
	bound := map[string]bool{}
	for _, line := range strings.Split(log, "\n") {
		_, rest, found := strings.Cut(line, prefix)
		if !found {
			continue
		}
		_, quoted, found := strings.Cut(rest, "symbol `")
		if !found {
			continue
		}
		name, _, found := strings.Cut(quoted, "'")
		if found {
			bound[name] = true
		}
	}
	return bound
}
