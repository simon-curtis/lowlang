package link

import (
	"fmt"

	"lowlang/internal/asm"
)

const (
	BaseAddress uint64 = 0x400000
	EntrySymbol        = "_start"
	FileSymbol         = "main.o"
)

func Static(obj *asm.Object) ([]byte, error) {
	l := newLinker(obj)
	l.collectSections()
	l.layout()
	l.resolveSymbols()
	entry, err := l.entry()
	if err != nil {
		return nil, err
	}
	tables := l.buildTables()
	image := make([]byte, tables.size)
	if err := l.applySections(image); err != nil {
		return nil, err
	}
	l.writeHeaders(image, entry, tables)
	l.writeTables(image, tables)
	return image, nil
}

func (l *linker) entry() (uint64, error) {
	sym, ok := l.symbols[EntrySymbol]
	if !ok {
		return 0, fmt.Errorf("link: entry symbol %s is not defined", EntrySymbol)
	}
	return sym.value, nil
}

type linker struct {
	obj      *asm.Object
	sections []*section
	byInput  map[int]*section
	segments []segment
	symbols  map[string]symbolValue
	fileEnd  uint64
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
