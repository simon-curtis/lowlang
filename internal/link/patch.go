package link

import (
	"fmt"

	"lowlang/internal/asm"
)

type PatchImage struct {
	Address uint64
	Code    []byte
	Symbols map[string]uint64
}

func Patch(obj *asm.Object, base uint64, running map[string]uint64) (*PatchImage, error) {
	placed, err := placePatch(obj, base)
	if err != nil {
		return nil, err
	}
	if len(placed.sections) == 0 {
		return nil, fmt.Errorf("link: patch has no code")
	}
	address := placed.sections[0].addr
	end := placed.sections[len(placed.sections)-1].end()
	image := make([]byte, end-address)
	spaces := make([]fixupSpace, 0, len(placed.sections))
	for _, s := range placed.sections {
		copy(image[s.addr-address:], s.in.Data)
		spaces = append(spaces, fixupSpace{name: s.in.Name, addr: s.addr, offset: s.addr - address, size: s.size, fixups: s.in.Fixups})
	}
	format := &patchFormat{defined: placed.symbols, running: running}
	if err := relocate(format, image, spaces); err != nil {
		return nil, err
	}
	return &PatchImage{Address: address, Code: image, Symbols: placed.symbols}, nil
}

type placedPatch struct {
	sections []*section
	symbols  map[string]uint64
}

func placePatch(obj *asm.Object, base uint64) (*placedPatch, error) {
	l := newLinker(obj)
	l.collectSections()
	for _, s := range l.sections {
		if s.class != classText && s.class != classRodata {
			return nil, fmt.Errorf("link: patch section %s is neither code nor read-only data", s.in.Name)
		}
	}
	l.orderSections()
	address := alignUp(base, maxAlign(l.sections))
	c := &cursor{addr: address}
	for _, s := range l.sections {
		c.place(s)
	}
	symbols := make(map[string]uint64, len(obj.Symbols))
	for _, sym := range obj.Symbols {
		if value, ok := l.resolve(sym); ok {
			symbols[sym.Name] = value.value
		}
	}
	return &placedPatch{sections: l.sections, symbols: symbols}, nil
}

type patchFormat struct {
	defined map[string]uint64
	running map[string]uint64
}

func (p *patchFormat) supports(kind asm.FixupKind) bool {
	return !kind.ImageRelative() && !kind.GOT()
}

func (p *patchFormat) target(space *fixupSpace, f asm.Fixup) (uint64, bool, error) {
	if address, ok := p.defined[f.Symbol]; ok {
		return address, true, nil
	}
	address, ok := p.running[f.Symbol]
	return address, ok, nil
}
