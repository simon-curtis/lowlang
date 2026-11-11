package asm_test

import (
	"bytes"
	"debug/elf"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"lowlang/internal/target"
)

type normReloc struct {
	offset int64
	kind   string
	target string
	addend int64
}

func (r normReloc) String() string {
	return fmt.Sprintf("+%#x %s %s%+d", r.offset, r.kind, r.target, r.addend)
}

type oracleSection struct {
	name   string
	flags  string
	align  uint64
	noBits bool
	size   uint64
	data   []byte
	relocs []normReloc
}

type oracleSymbol struct {
	name    string
	section string
	value   uint64
	global  bool
}

type oracleObject struct {
	sections map[string]*oracleSection
	symbols  map[string]oracleSymbol
}

const (
	sectionUndefined = "*UND*"
	sectionAbsolute  = "*ABS*"
)

var relocKinds = map[elf.R_X86_64]string{
	elf.R_X86_64_64:            "Abs64",
	elf.R_X86_64_32:            "Abs32",
	elf.R_X86_64_32S:           "Abs32S",
	elf.R_X86_64_PC32:          "PC32",
	elf.R_X86_64_PLT32:         "PLT32",
	elf.R_X86_64_GOTPCREL:      "GOTPCREL",
	elf.R_X86_64_GOTPCRELX:     "GOTPCREL",
	elf.R_X86_64_REX_GOTPCRELX: "GOTPCREL",
}

func canonicalFlags(flags string) string {
	letters := strings.Split(flags, "")
	sort.Strings(letters)
	return strings.Join(letters, "")
}

func elfFlags(flags elf.SectionFlag) string {
	var b strings.Builder
	if flags&elf.SHF_ALLOC != 0 {
		b.WriteByte('a')
	}
	if flags&elf.SHF_WRITE != 0 {
		b.WriteByte('w')
	}
	if flags&elf.SHF_EXECINSTR != 0 {
		b.WriteByte('x')
	}
	if flags&elf.SHF_TLS != 0 {
		b.WriteByte('T')
	}
	return canonicalFlags(b.String())
}

func asSource(source string, platform target.OS) string {
	if platform != target.Windows {
		return source
	}
	source = strings.ReplaceAll(source, "@IMGREL", "")
	return strings.ReplaceAll(source, `,"dr"`, `,"a"`)
}

func runAs(dir, source string, flags []string) (*oracleObject, error) {
	sourcePath := filepath.Join(dir, "oracle.s")
	if err := os.WriteFile(sourcePath, []byte(source), 0o644); err != nil {
		return nil, fmt.Errorf("write source: %w", err)
	}
	objectPath := filepath.Join(dir, "oracle.o")
	args := append(append([]string{}, flags...), "-o", objectPath, sourcePath)
	out, err := exec.Command("as", args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("as %v: %w\n%s", flags, err, truncate(string(out), 600))
	}
	object, err := readObject(objectPath)
	os.Remove(sourcePath)
	os.Remove(objectPath)
	return object, err
}

func truncate(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	return text[:limit]
}

func readObject(path string) (*oracleObject, error) {
	file, err := elf.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open object: %w", err)
	}
	defer file.Close()
	symbols, err := file.Symbols()
	if err != nil {
		return nil, fmt.Errorf("read symbols: %w", err)
	}
	object := &oracleObject{sections: map[string]*oracleSection{}, symbols: map[string]oracleSymbol{}}
	for _, s := range file.Sections {
		if err := object.addSection(file, s); err != nil {
			return nil, err
		}
	}
	for _, s := range file.Sections {
		if s.Type == elf.SHT_RELA {
			if err := object.addRelocs(file, s, symbols); err != nil {
				return nil, err
			}
		}
	}
	for _, sym := range symbols {
		object.addSymbol(file, sym)
	}
	return object, nil
}

func (o *oracleObject) addSection(file *elf.File, s *elf.Section) error {
	switch s.Type {
	case elf.SHT_PROGBITS, elf.SHT_NOBITS:
	default:
		return nil
	}
	section := &oracleSection{
		name:   s.Name,
		flags:  elfFlags(s.Flags),
		align:  s.Addralign,
		noBits: s.Type == elf.SHT_NOBITS,
		size:   s.Size,
	}
	if !section.noBits {
		data, err := s.Data()
		if err != nil {
			return fmt.Errorf("read section %s: %w", s.Name, err)
		}
		section.data = data
	}
	o.sections[s.Name] = section
	return nil
}

func (o *oracleObject) addRelocs(file *elf.File, rela *elf.Section, symbols []elf.Symbol) error {
	targetSection := file.Sections[rela.Info]
	section := o.sections[targetSection.Name]
	if section == nil {
		return nil
	}
	data, err := rela.Data()
	if err != nil {
		return fmt.Errorf("read %s: %w", rela.Name, err)
	}
	for r := bytes.NewReader(data); r.Len() >= 24; {
		var entry struct {
			Offset uint64
			Info   uint64
			Addend int64
		}
		if err := binary.Read(r, binary.LittleEndian, &entry); err != nil {
			return fmt.Errorf("parse %s: %w", rela.Name, err)
		}
		section.relocs = append(section.relocs, o.normalise(file, symbols, entry.Offset, entry.Info, entry.Addend))
	}
	sort.SliceStable(section.relocs, func(i, j int) bool { return section.relocs[i].offset < section.relocs[j].offset })
	return nil
}

func (o *oracleObject) normalise(file *elf.File, symbols []elf.Symbol, offset, info uint64, addend int64) normReloc {
	kind, ok := relocKinds[elf.R_X86_64(info&0xffffffff)]
	if !ok {
		kind = elf.R_X86_64(info & 0xffffffff).String()
	}
	norm := normReloc{offset: int64(offset), kind: kind, addend: addend}
	index := int(info >> 32)
	if index == 0 {
		norm.target = "none"
		return norm
	}
	sym := symbols[index-1]
	bind := elf.ST_BIND(sym.Info)
	if elf.ST_TYPE(sym.Info) == elf.STT_SECTION {
		norm.target = fmt.Sprintf("sec:%s", file.Sections[sym.Section].Name)
		return norm
	}
	if bind == elf.STB_LOCAL && sym.Section != elf.SHN_UNDEF && sym.Section < elf.SHN_LORESERVE {
		norm.target = fmt.Sprintf("sec:%s", file.Sections[sym.Section].Name)
		norm.addend += int64(sym.Value)
		return norm
	}
	norm.target = fmt.Sprintf("sym:%s", sym.Name)
	return norm
}

func (o *oracleObject) addSymbol(file *elf.File, sym elf.Symbol) {
	switch elf.ST_TYPE(sym.Info) {
	case elf.STT_SECTION, elf.STT_FILE:
		return
	}
	if strings.HasPrefix(sym.Name, ".L") || sym.Name == "" {
		return
	}
	section := sectionUndefined
	switch {
	case sym.Section == elf.SHN_UNDEF:
	case sym.Section == elf.SHN_ABS:
		section = sectionAbsolute
	case sym.Section < elf.SHN_LORESERVE:
		section = file.Sections[sym.Section].Name
	}
	o.symbols[sym.Name] = oracleSymbol{
		name:    sym.Name,
		section: section,
		value:   sym.Value,
		global:  elf.ST_BIND(sym.Info) != elf.STB_LOCAL,
	}
}
