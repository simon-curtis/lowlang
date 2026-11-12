package link

import (
	"lowlang/internal/amd64"
	"lowlang/internal/asm"
)

const (
	BaseAddress uint64 = 0x400000
	EntrySymbol        = amd64.ELFEntrySymbol
	FileSymbol         = "main.o"
)

func Static(obj *asm.Object) ([]byte, error) {
	l := newLinker(obj)
	return l.link()
}

func Dynamic(obj *asm.Object, libraries []*Library) ([]byte, error) {
	l := newLinker(obj)
	l.dyn = newDynamicLink(libraries)
	return l.link()
}

func (l *linker) link() ([]byte, error) {
	l.collectSections()
	if l.dyn != nil {
		if err := l.addDynamicSections(); err != nil {
			return nil, err
		}
	}
	l.orderSections()
	l.layout()
	l.resolveSymbols()
	entry, err := l.entry()
	if err != nil {
		return nil, err
	}
	if l.dyn != nil {
		l.fillDynamic()
	}
	tables, err := l.buildTables()
	if err != nil {
		return nil, err
	}
	image := make([]byte, tables.size)
	if err := l.applySections(image); err != nil {
		return nil, err
	}
	if err := l.writeHeaders(image, entry, tables); err != nil {
		return nil, err
	}
	if err := l.writeTables(image, tables); err != nil {
		return nil, err
	}
	return image, nil
}

func (l *linker) entry() (uint64, error) {
	return entryAddress(EntrySymbol, l.addressOf)
}

func (l *linker) addressOf(name string) (uint64, bool) {
	sym, ok := l.symbols[name]
	return sym.value, ok
}

type linker struct {
	obj      *asm.Object
	sections []*section
	byInput  map[int]*section
	segments []segment
	symbols  map[string]symbolValue
	fileEnd  uint64
	dyn      *dynamicLink
}

func newLinker(obj *asm.Object) *linker {
	return &linker{
		obj:     obj,
		byInput: map[int]*section{},
		symbols: map[string]symbolValue{},
	}
}

func alignUp(value, align uint64) uint64 {
	if align <= 1 {
		return value
	}
	return (value + align - 1) / align * align
}
