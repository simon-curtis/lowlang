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
		sym, ok := l.symbols[f.Symbol]
		if !ok {
			undefined[f.Symbol] = true
			continue
		}
		if err := l.applyFixup(image, s, f, sym.value); err != nil {
			return err
		}
	}
	return nil
}

func (l *linker) applyFixup(image []byte, s *section, f asm.Fixup, symbol uint64) error {
	width := fixupWidth(f.Kind)
	if width == 0 {
		return fmt.Errorf("link: %s relocation against %s in %s is not supported in a static program", f.Kind, f.Symbol, s.in.Name)
	}
	if f.Offset < 0 || uint64(f.Offset)+uint64(width) > s.size {
		return fmt.Errorf("link: %s relocation against %s lies outside %s", f.Kind, f.Symbol, s.in.Name)
	}
	place := s.addr + uint64(f.Offset)
	value := int64(symbol) + f.Addend
	if f.Kind == asm.PC32 || f.Kind == asm.PLT32 {
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

func fixupWidth(kind asm.FixupKind) int {
	switch kind {
	case asm.Abs64:
		return 8
	case asm.Abs32, asm.Abs32S, asm.PC32, asm.PLT32:
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
	case asm.Abs32S, asm.PC32, asm.PLT32:
		if value < math.MinInt32 || value > math.MaxInt32 {
			return fmt.Errorf("value %#x does not fit in 32 signed bits", value)
		}
	}
	return nil
}
