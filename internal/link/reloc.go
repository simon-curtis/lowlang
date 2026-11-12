package link

import (
	"encoding/binary"
	"fmt"
	"math"
	"sort"
	"strings"

	"lowlang/internal/asm"
)

func (l *linker) applySections(image []byte) error {
	undefined := map[string]bool{}
	for _, s := range l.sections {
		if s.in.NoBits {
			continue
		}
		copy(image[s.off:], s.in.Data)
		if err := l.applyFixups(image, s, undefined); err != nil {
			return err
		}
	}
	if len(undefined) == 0 {
		return nil
	}
	names := make([]string, 0, len(undefined))
	for name := range undefined {
		names = append(names, name)
	}
	sort.Strings(names)
	return fmt.Errorf("link: undefined symbol %s", strings.Join(names, ", "))
}

func (l *linker) applyFixups(image []byte, s *section, undefined map[string]bool) error {
	for _, f := range s.in.Fixups {
		target, ok, err := l.fixupTarget(s, f)
		if err != nil {
			return err
		}
		if !ok {
			undefined[f.Symbol] = true
			continue
		}
		if err := l.applyFixup(image, s, f, target); err != nil {
			return err
		}
	}
	return nil
}

func (l *linker) fixupTarget(s *section, f asm.Fixup) (uint64, bool, error) {
	if isGOTKind(f.Kind) {
		return l.gotTarget(s, f)
	}
	if sym, defined := l.symbols[f.Symbol]; defined {
		return sym.value, true, nil
	}
	if l.dyn == nil {
		return 0, false, nil
	}
	if address, planted := l.dyn.pltAddress(f.Symbol); planted && f.Kind == asm.PLT32 {
		return address, true, nil
	}
	if l.dyn.imports(f.Symbol) {
		return 0, false, fmt.Errorf("link: %s relocation against %s in %s needs a copy relocation or canonical PLT entry, which are not supported", f.Kind, f.Symbol, s.in.Name)
	}
	return 0, false, nil
}

func (l *linker) gotTarget(s *section, f asm.Fixup) (uint64, bool, error) {
	if l.dyn == nil {
		return 0, false, fmt.Errorf("link: %s relocation against %s in %s is not supported in a static program", f.Kind, f.Symbol, s.in.Name)
	}
	return l.dyn.gotAddress(f.Symbol), true, nil
}

func (l *linker) applyFixup(image []byte, s *section, f asm.Fixup, symbol uint64) error {
	width := fixupWidth(f.Kind)
	if width == 0 {
		return fmt.Errorf("link: %s relocation against %s in %s is not supported", f.Kind, f.Symbol, s.in.Name)
	}
	if f.Offset < 0 || uint64(f.Offset)+uint64(width) > s.size {
		return fmt.Errorf("link: %s relocation against %s lies outside %s", f.Kind, f.Symbol, s.in.Name)
	}
	place := s.addr + uint64(f.Offset)
	value := int64(symbol) + f.Addend
	if isPCRelative(f.Kind) {
		value -= int64(place)
	}
	if err := checkRange(f.Kind, value); err != nil {
		return fmt.Errorf("link: %s relocation against %s at %s+%#x: %w", f.Kind, f.Symbol, s.in.Name, f.Offset, err)
	}
	field := image[s.off+uint64(f.Offset):]
	if width == 8 {
		binary.LittleEndian.PutUint64(field, uint64(value))
		return nil
	}
	binary.LittleEndian.PutUint32(field, uint32(value))
	return nil
}

func isPCRelative(kind asm.FixupKind) bool {
	return kind == asm.PC32 || kind == asm.PLT32 || isGOTKind(kind)
}

func fixupWidth(kind asm.FixupKind) int {
	switch kind {
	case asm.Abs64:
		return 8
	case asm.Abs32, asm.Abs32S, asm.PC32, asm.PLT32, asm.GOTPCREL, asm.GOTPCRELX, asm.REXGOTPCRELX:
		return 4
	}
	return 0
}

func checkRange(kind asm.FixupKind, value int64) error {
	switch kind {
	case asm.Abs32:
		if value < 0 || value > math.MaxUint32 {
			return fmt.Errorf("value %#x does not fit in 32 unsigned bits", value)
		}
	case asm.Abs32S, asm.PC32, asm.PLT32, asm.GOTPCREL, asm.GOTPCRELX, asm.REXGOTPCRELX:
		if value < math.MinInt32 || value > math.MaxInt32 {
			return fmt.Errorf("value %#x does not fit in 32 signed bits", value)
		}
	}
	return nil
}
