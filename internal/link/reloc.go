package link

import (
	"fmt"

	"lowlang/internal/asm"
)

func (l *linker) applySections(image []byte) error {
	spaces := make([]fixupSpace, 0, len(l.sections))
	for _, s := range l.sections {
		if s.in.NoBits {
			continue
		}
		copy(image[s.off:], s.in.Data)
		spaces = append(spaces, fixupSpace{name: s.in.Name, addr: s.addr, offset: s.off, size: s.size, fixups: s.in.Fixups})
	}
	return relocate(l, image, spaces)
}

func (l *linker) imageBase() uint64 {
	return 0
}

func (l *linker) supports(kind asm.FixupKind) bool {
	return !kind.ImageRelative()
}

func (l *linker) target(space *fixupSpace, f asm.Fixup) (uint64, bool, error) {
	if f.Kind.GOT() {
		return l.gotTarget(space, f)
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
		return 0, false, fmt.Errorf("link: %s relocation against %s in %s needs a copy relocation or canonical PLT entry, which are not supported", f.Kind, f.Symbol, space.name)
	}
	return 0, false, nil
}

func (l *linker) gotTarget(space *fixupSpace, f asm.Fixup) (uint64, bool, error) {
	if l.dyn == nil {
		return 0, false, fmt.Errorf("link: %s relocation against %s in %s is not supported in a static program", f.Kind, f.Symbol, space.name)
	}
	return l.dyn.gotAddress(f.Symbol), true, nil
}
