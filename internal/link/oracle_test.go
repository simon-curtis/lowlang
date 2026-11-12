package link_test

import (
	"bytes"
	"debug/elf"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"

	"lowlang/internal/amd64"
	"lowlang/internal/asm"
	"lowlang/internal/compiler"
	"lowlang/internal/link"
	"lowlang/internal/oracletool"
	"lowlang/internal/target"
)

const (
	noUsedNoteFlag = "-mx86-used-note=no"
	headerBytes    = 64
	stackNote      = "\n.section .note.GNU-stack,\"\",@progbits\n"
)

var sampleCorpus = []string{
	"testdata/run/chaining.low",
	"testdata/run/callbacks.low",
	"testdata/run/cf_loops.low",
	"testdata/run/cf_spawn_loop.low",
	"testdata/run/chan_fairness.low",
	"testdata/run/std_jose.low",
	"testdata/run/std_tzdata.low",
	"testdata/run/std_unicode_graphemes.low",
	"testdata/run/std_http_proxy.low",
	"bench/binarytrees/main.low",
	"bench/nbody/main.low",
	"bench/jsonpar/main.low",
	"bench/maps/main.low",
}

func repoRoot() string {
	return filepath.Join("..", "..")
}

func corpusPaths(t *testing.T) []string {
	if os.Getenv("LOW_LINK_FULL") == "" {
		return sampleCorpus
	}
	var paths []string
	for _, pattern := range []string{"testdata/run/*.low", "bench/*/main.low"} {
		matches, err := filepath.Glob(filepath.Join(repoRoot(), pattern))
		if err != nil {
			t.Fatalf("glob %s: %v", pattern, err)
		}
		for _, match := range matches {
			rel, err := filepath.Rel(repoRoot(), match)
			if err != nil {
				t.Fatalf("relative path of %s: %v", match, err)
			}
			paths = append(paths, rel)
		}
	}
	sort.Strings(paths)
	return paths
}

func generate(path string, platform target.OS) (source string, libraries []string, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("compiler panic: %v", r)
		}
	}()
	unit, err := compiler.Analyse(filepath.Join(repoRoot(), path), platform)
	if err != nil {
		return "", nil, err
	}
	return amd64.Generate(unit.Program, unit.Platform, nil).Asm, unit.Program.Libraries(), nil
}

func linkWithLd(dir, source string) (string, error) {
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
	if out, err := exec.Command("ld", "-static", "-o", output, object).CombinedOutput(); err != nil {
		return "", fmt.Errorf("ld: %w\n%s", err, out)
	}
	return output, nil
}

func linkInProcess(dir, source string) (string, error) {
	object, err := asm.Assemble(source, asm.Options{AlignBranches: true})
	if err != nil {
		return "", fmt.Errorf("assemble: %w", err)
	}
	image, err := link.Static(object)
	if err != nil {
		return "", err
	}
	output := filepath.Join(dir, "main.low")
	if err := os.WriteFile(output, image, 0o755); err != nil {
		return "", err
	}
	return output, nil
}

func requireTools(t *testing.T) {
	for _, tool := range []string{"as", "ld"} {
		oracletool.Require(t, tool, oracletool.GNU)
	}
}

func TestMatchesGNULd(t *testing.T) {
	requireTools(t)
	for _, path := range corpusPaths(t) {
		t.Run(path, func(t *testing.T) {
			source, libraries, err := generate(path, target.Linux)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			if len(libraries) > 0 {
				t.Skip("program links C libraries")
			}
			dir := t.TempDir()
			want, err := linkWithLd(dir, source)
			if err != nil {
				t.Fatal(err)
			}
			got, err := linkInProcess(dir, source)
			if err != nil {
				t.Fatal(err)
			}
			compareExecutables(t, got, want)
		})
	}
}

func compareExecutables(t *testing.T, gotPath, wantPath string) {
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
	gotBytes := readFile(t, gotPath)
	wantBytes := readFile(t, wantPath)
	c := &executableComparison{t: t, got: got, want: want, gotBytes: gotBytes, wantBytes: wantBytes}
	c.compareHeader()
	c.comparePrograms()
	c.compareSections()
	c.compareSymbols()
	c.compareLoadedBytes()
}

func readFile(t *testing.T, path string) []byte {
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

type executableComparison struct {
	t         *testing.T
	got       *elf.File
	want      *elf.File
	gotBytes  []byte
	wantBytes []byte
}

func (c *executableComparison) compareHeader() {
	if c.got.Type != c.want.Type || c.got.Machine != c.want.Machine || c.got.Entry != c.want.Entry {
		c.t.Errorf("header: got type %v machine %v entry %#x, want type %v machine %v entry %#x",
			c.got.Type, c.got.Machine, c.got.Entry, c.want.Type, c.want.Machine, c.want.Entry)
	}
}

func (c *executableComparison) comparePrograms() {
	if len(c.got.Progs) != len(c.want.Progs) {
		c.t.Errorf("program headers: got %d, want %d", len(c.got.Progs), len(c.want.Progs))
		return
	}
	for i, want := range c.want.Progs {
		got := c.got.Progs[i]
		if got.ProgHeader != want.ProgHeader {
			c.t.Errorf("program header %d: got %+v, want %+v", i, got.ProgHeader, want.ProgHeader)
		}
	}
}

func (c *executableComparison) compareSections() {
	if len(c.got.Sections) != len(c.want.Sections) {
		c.t.Errorf("sections: got %d, want %d", len(c.got.Sections), len(c.want.Sections))
		return
	}
	for i, want := range c.want.Sections {
		got := c.got.Sections[i]
		g, w := got.SectionHeader, want.SectionHeader
		if w.Flags&elf.SHF_ALLOC == 0 {
			g.Offset, g.Size, g.FileSize = 0, 0, 0
			w.Offset, w.Size, w.FileSize = 0, 0, 0
		}
		if g != w {
			c.t.Errorf("section %d: got %+v, want %+v", i, got.SectionHeader, want.SectionHeader)
		}
	}
}

type symbolView struct {
	value   uint64
	info    byte
	other   byte
	section string
}

func (c *executableComparison) symbolViews(file *elf.File) (map[string]symbolView, int) {
	symbols, err := file.Symbols()
	if err != nil {
		c.t.Fatalf("read symbols: %v", err)
	}
	views := make(map[string]symbolView, len(symbols))
	firstGlobal := 0
	for i, sym := range symbols {
		if firstGlobal == 0 && elf.ST_BIND(sym.Info) != elf.STB_LOCAL {
			firstGlobal = i + 1
		}
		view := symbolView{value: sym.Value, info: sym.Info, other: sym.Other}
		switch {
		case sym.Section == elf.SHN_ABS:
			view.section = "*ABS*"
		case sym.Section < elf.SHN_LORESERVE:
			view.section = file.Sections[sym.Section].Name
		}
		views[sym.Name] = view
	}
	return views, firstGlobal
}

func (c *executableComparison) compareSymbols() {
	got, gotFirst := c.symbolViews(c.got)
	want, wantFirst := c.symbolViews(c.want)
	if gotFirst != wantFirst {
		c.t.Errorf("first global symbol index: got %d, want %d", gotFirst, wantFirst)
	}
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
	sort.Strings(problems)
	for i, problem := range problems {
		if i == 5 {
			c.t.Errorf("... %d more symbol mismatches", len(problems)-5)
			break
		}
		c.t.Error(problem)
	}
}

func (c *executableComparison) compareLoadedBytes() {
	for i, want := range c.want.Progs {
		if want.Type != elf.PT_LOAD || i >= len(c.got.Progs) {
			continue
		}
		got := c.got.Progs[i]
		skip := uint64(0)
		if want.Off == 0 {
			skip = headerBytes
		}
		gotData := c.gotBytes[got.Off+skip : got.Off+got.Filesz]
		wantData := c.wantBytes[want.Off+skip : want.Off+want.Filesz]
		if !bytes.Equal(gotData, wantData) {
			c.t.Errorf("segment %d (%v): loaded bytes differ at %d", i, want.Flags, firstDifference(gotData, wantData))
		}
	}
}

func firstDifference(a, b []byte) int {
	for i := range min(len(a), len(b)) {
		if a[i] != b[i] {
			return i
		}
	}
	return min(len(a), len(b))
}
