package asm_test

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"lowlang/internal/asm"
	"lowlang/internal/target"
)

type comparison struct {
	object     *asm.Object
	oracle     *oracleObject
	platform   target.OS
	byName     map[string]*asm.Section
	symbols    map[string]asm.Symbol
	mismatches []string
	checked    int
	text       int
}

func newComparison(object *asm.Object, oracle *oracleObject, platform target.OS) *comparison {
	c := &comparison{
		object:   object,
		oracle:   oracle,
		platform: platform,
		byName:   make(map[string]*asm.Section, len(object.Sections)),
		symbols:  make(map[string]asm.Symbol, len(object.Symbols)),
	}
	for _, s := range object.Sections {
		c.byName[s.Name] = s
	}
	for _, sym := range object.Symbols {
		c.symbols[sym.Name] = sym
	}
	return c
}

func (c *comparison) report(format string, args ...any) {
	c.mismatches = append(c.mismatches, fmt.Sprintf(format, args...))
}

func (c *comparison) run() {
	names := map[string]bool{}
	for name := range c.byName {
		names[name] = true
	}
	for name := range c.oracle.sections {
		names[name] = true
	}
	sorted := make([]string, 0, len(names))
	for name := range names {
		sorted = append(sorted, name)
	}
	sort.Strings(sorted)
	for _, name := range sorted {
		c.compareSection(name, c.byName[name], c.oracle.sections[name])
	}
	c.compareSymbols()
}

func (c *comparison) mineEmpty(s *asm.Section) bool {
	return s == nil || (len(s.Data) == 0 && s.Size == 0 && len(s.Items) == 0 && len(s.Fixups) == 0)
}

func (c *comparison) oracleEmpty(s *oracleSection) bool {
	return s == nil || (len(s.data) == 0 && s.size == 0)
}

func (c *comparison) mineFlags(s *asm.Section) string {
	flags := s.Flags
	if c.platform == target.Windows && strings.Contains(flags, "d") {
		flags = "a"
	}
	return canonicalFlags(flags)
}

func (c *comparison) compareSection(name string, mine *asm.Section, theirs *oracleSection) {
	if c.mineEmpty(mine) && c.oracleEmpty(theirs) {
		return
	}
	if mine == nil || theirs == nil {
		c.report("%s: present only in %s", name, map[bool]string{true: "as", false: "asm"}[mine == nil])
		return
	}
	c.checked++
	if got, want := c.mineFlags(mine), theirs.flags; got != want {
		c.report("%s: flags %q, as has %q", name, got, want)
	}
	if mine.NoBits != theirs.noBits {
		c.report("%s: nobits %v, as has %v", name, mine.NoBits, theirs.noBits)
	}
	if strings.Contains(theirs.flags, "x") && !c.object.TextEncoded {
		c.text++
		return
	}
	if uint64(mine.Align) != theirs.align {
		c.report("%s: align %d, as has %d", name, mine.Align, theirs.align)
	}
	if uint64(mine.Size) != theirs.size {
		c.report("%s: size %d, as has %d", name, mine.Size, theirs.size)
	}
	if !theirs.noBits {
		c.compareBytes(name, mine, theirs)
	}
	c.compareRelocs(name, mine, theirs)
}

func (c *comparison) compareBytes(name string, mine *asm.Section, theirs *oracleSection) {
	got, want := mine.Data, theirs.data
	if len(mine.Unresolved) > 0 {
		got, want = bytes.Clone(got), bytes.Clone(want)
		for _, span := range mine.Unresolved {
			clear(got[span.Offset : span.Offset+span.Size])
			clear(want[span.Offset : span.Offset+span.Size])
		}
	}
	if len(got) != len(want) {
		c.report("%s: %d bytes, as has %d", name, len(got), len(want))
		return
	}
	if bytes.Equal(got, want) {
		return
	}
	i := 0
	for got[i] == want[i] {
		i++
	}
	end := min(i+8, len(got))
	c.report("%s +%#x (%s): % x, as has % x", name, i, c.context(mine, int64(i)), got[i:end], want[i:end])
}

func (c *comparison) context(section *asm.Section, offset int64) string {
	index := -1
	for i, s := range c.object.Sections {
		if s == section {
			index = i
		}
	}
	best := ""
	var bestValue int64 = -1
	for _, sym := range c.object.Symbols {
		if sym.Section == index && sym.Value <= offset && sym.Value > bestValue {
			best, bestValue = sym.Name, sym.Value
		}
	}
	if best == "" {
		return "start"
	}
	return fmt.Sprintf("%s+%d", best, offset-bestValue)
}

func (c *comparison) isExec(name string) bool {
	s := c.oracle.sections[name]
	return s != nil && strings.Contains(s.flags, "x")
}

func (c *comparison) mineRelocs(section *asm.Section) ([]normReloc, []int) {
	relocs := make([]normReloc, 0, len(section.Fixups))
	lines := make([]int, 0, len(section.Fixups))
	for _, f := range section.Fixups {
		norm := c.normalise(f)
		relocs = append(relocs, norm)
		lines = append(lines, f.Line)
	}
	order := make([]int, len(relocs))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(i, j int) bool { return relocs[order[i]].offset < relocs[order[j]].offset })
	sortedRelocs := make([]normReloc, len(relocs))
	sortedLines := make([]int, len(relocs))
	for i, from := range order {
		sortedRelocs[i], sortedLines[i] = relocs[from], lines[from]
	}
	return sortedRelocs, sortedLines
}

func (c *comparison) normalise(f asm.Fixup) normReloc {
	norm := normReloc{offset: f.Offset, kind: f.Kind.String(), addend: f.Addend}
	sym, ok := c.symbols[f.Symbol]
	if !ok || sym.Section < 0 || sym.Global {
		norm.target = fmt.Sprintf("sym:%s", f.Symbol)
		return norm
	}
	section := c.object.Sections[sym.Section]
	norm.target = fmt.Sprintf("sec:%s", section.Name)
	norm.addend += sym.Value
	return norm
}

func (c *comparison) maskText(r normReloc) normReloc {
	if c.object.TextEncoded {
		return r
	}
	if name, ok := strings.CutPrefix(r.target, "sec:"); ok && c.isExec(name) {
		r.addend = 0
	}
	return r
}

func (c *comparison) compareRelocs(name string, mine *asm.Section, theirs *oracleSection) {
	got, lines := c.mineRelocs(mine)
	want := theirs.relocs
	if c.platform == target.Windows {
		want = c.windowsRelocs(want)
	}
	for i := 0; i < len(got) && i < len(want); i++ {
		g, w := c.maskText(got[i]), c.maskText(want[i])
		if g != w {
			c.report("%s reloc %d (line %d): %s, as has %s", name, i, lines[i], g, w)
			return
		}
	}
	if len(got) != len(want) {
		c.report("%s: %d relocations, as has %d", name, len(got), len(want))
	}
}

func (c *comparison) windowsRelocs(relocs []normReloc) []normReloc {
	out := make([]normReloc, len(relocs))
	for i, r := range relocs {
		if r.kind == "Abs32" {
			r.kind = "ImgRel32"
		}
		out[i] = r
	}
	return out
}

func (c *comparison) mineSectionName(sym asm.Symbol) string {
	switch sym.Section {
	case asm.SectionUndefined:
		return sectionUndefined
	case asm.SectionAbsolute:
		return sectionAbsolute
	}
	return c.object.Sections[sym.Section].Name
}

func (c *comparison) compareSymbols() {
	mine := make(map[string]asm.Symbol, len(c.symbols))
	for name, sym := range c.symbols {
		if !strings.HasPrefix(name, ".L") {
			mine[name] = sym
		}
	}
	for name, want := range c.oracle.symbols {
		got, ok := mine[name]
		if !ok {
			c.report("symbol %s: missing from asm", name)
			continue
		}
		c.compareSymbol(got, want)
	}
	for name := range mine {
		if _, ok := c.oracle.symbols[name]; !ok {
			c.report("symbol %s: not in as", name)
		}
	}
}

func (c *comparison) compareSymbol(got asm.Symbol, want oracleSymbol) {
	section := c.mineSectionName(got)
	if section != want.section {
		c.report("symbol %s: section %s, as has %s", got.Name, section, want.section)
		return
	}
	if got.Global != want.global {
		c.report("symbol %s: global %v, as has %v", got.Name, got.Global, want.global)
	}
	if c.isExec(section) && !c.object.TextEncoded {
		return
	}
	if section != sectionUndefined && uint64(got.Value) != want.value {
		c.report("symbol %s: value %#x, as has %#x", got.Name, uint64(got.Value), want.value)
	}
}
