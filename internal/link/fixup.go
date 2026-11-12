package link

import (
	"encoding/binary"
	"fmt"
	"sort"
	"strings"

	"lowlang/internal/asm"
)

type fixupSpace struct {
	name   string
	addr   uint64
	offset uint64
	size   uint64
	fixups []asm.Fixup
}

type fixupFormat interface {
	supports(kind asm.FixupKind) bool
	target(space *fixupSpace, f asm.Fixup) (address uint64, defined bool, err error)
}

func relocate(format fixupFormat, image []byte, spaces []fixupSpace) error {
	undefined := map[string]bool{}
	for i := range spaces {
		space := &spaces[i]
		for _, f := range space.fixups {
			address, defined, err := format.target(space, f)
			if err != nil {
				return err
			}
			if !defined {
				undefined[f.Symbol] = true
				continue
			}
			if err := applyFixup(format, image, space, f, address); err != nil {
				return err
			}
		}
	}
	return undefinedSymbolsError(undefined)
}

func undefinedSymbolsError(undefined map[string]bool) error {
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

func applyFixup(format fixupFormat, image []byte, space *fixupSpace, f asm.Fixup, address uint64) error {
	width := f.Kind.Width()
	if width == 0 || !format.supports(f.Kind) {
		return fmt.Errorf("link: %s relocation against %s in %s is not supported", f.Kind, f.Symbol, space.name)
	}
	if f.Offset < 0 || uint64(f.Offset)+uint64(width) > space.size {
		return fmt.Errorf("link: %s relocation against %s lies outside %s", f.Kind, f.Symbol, space.name)
	}
	place := space.addr + uint64(f.Offset)
	value := int64(address) + f.Addend
	if f.Kind.PCRelative() {
		value -= int64(place)
	}
	if !f.Kind.Fits(value) {
		return fmt.Errorf("link: %s relocation against %s at %s+%#x: %w", f.Kind, f.Symbol, space.name, f.Offset, rangeError(f.Kind, value))
	}
	field := image[space.offset+uint64(f.Offset):]
	if width == 8 {
		binary.LittleEndian.PutUint64(field, uint64(value))
		return nil
	}
	binary.LittleEndian.PutUint32(field, uint32(value))
	return nil
}

func rangeError(kind asm.FixupKind, value int64) error {
	signedness := "unsigned"
	if kind.Signed() {
		signedness = "signed"
	}
	return fmt.Errorf("value %#x does not fit in 32 %s bits", value, signedness)
}

func entryAddress(symbol string, lookup func(name string) (uint64, bool)) (uint64, error) {
	address, defined := lookup(symbol)
	if !defined {
		return 0, fmt.Errorf("link: entry symbol %s is not defined", symbol)
	}
	return address, nil
}
