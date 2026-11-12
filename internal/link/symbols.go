package link

import (
	"debug/elf"
	"strings"

	"lowlang/internal/asm"
)

type symbolValue struct {
	value     uint64
	section   *section
	absolute  bool
	undefined bool
	kind      elf.SymType
}

type linkerSymbol struct {
	name  string
	value symbolValue
}

func (l *linker) resolveSymbols() {
	for _, sym := range l.obj.Symbols {
		value, ok := l.resolve(sym)
		if ok {
			l.symbols[sym.Name] = value
		}
	}
	for _, defined := range l.boundarySymbols() {
		if _, taken := l.symbols[defined.name]; !taken {
			l.symbols[defined.name] = defined.value
		}
	}
}

func (l *linker) resolve(sym asm.Symbol) (symbolValue, bool) {
	switch sym.Section {
	case asm.SectionUndefined:
		return symbolValue{}, false
	case asm.SectionAbsolute:
		return symbolValue{value: uint64(sym.Value), absolute: true}, true
	}
	s, ok := l.byInput[sym.Section]
	if !ok {
		return symbolValue{}, false
	}
	return symbolValue{value: s.addr + uint64(sym.Value), section: s}, true
}

func (l *linker) boundarySymbols() []linkerSymbol {
	var lastData, lastLoaded, firstBss *section
	for _, s := range l.sections {
		if s.class == classTBss {
			continue
		}
		lastLoaded = s
		if s.class == classBss && firstBss == nil {
			firstBss = s
		}
		if s.class != classBss {
			lastData = s
		}
	}
	if lastLoaded == nil {
		return nil
	}
	edata := endOf(lastData)
	bssStart := edata
	if firstBss != nil {
		bssStart = symbolValue{value: firstBss.addr, section: firstBss}
	}
	return []linkerSymbol{
		{boundBssStart, bssStart},
		{boundEdata, edata},
		{boundEnd, endOf(lastLoaded)},
	}
}

func endOf(s *section) symbolValue {
	return symbolValue{value: s.end(), section: s}
}

func (l *linker) tlsStart() uint64 {
	for _, seg := range l.segments {
		if seg.kind == elf.PT_TLS {
			return seg.addr
		}
	}
	return 0
}

func (v symbolValue) isTLS() bool {
	return v.section != nil && (v.section.class == classTData || v.section.class == classTBss)
}

func (l *linker) symbolIndexOf(v symbolValue) uint16 {
	if v.undefined {
		return uint16(elf.SHN_UNDEF)
	}
	if v.absolute || v.section == nil {
		return uint16(elf.SHN_ABS)
	}
	return uint16(v.section.index)
}

func (l *linker) emittedValue(v symbolValue) uint64 {
	if v.isTLS() {
		return v.value - l.tlsStart()
	}
	return v.value
}

func (l *linker) emittedSymbols() (locals, globals []linkerSymbol) {
	for _, sym := range l.obj.Symbols {
		if strings.HasPrefix(sym.Name, ".L") {
			continue
		}
		value, ok := l.resolve(sym)
		if !ok {
			globals = l.appendImported(globals, sym.Name)
			continue
		}
		entry := linkerSymbol{name: sym.Name, value: value}
		if sym.Global {
			globals = append(globals, entry)
		} else {
			locals = append(locals, entry)
		}
	}
	own := l.objectDefined()
	for _, defined := range l.boundarySymbols() {
		if own[defined.name] {
			continue
		}
		globals = append(globals, defined)
	}
	if l.dyn != nil {
		locals = append(locals, l.dyn.tableSymbols()...)
	}
	return locals, globals
}

func (l *linker) appendImported(globals []linkerSymbol, name string) []linkerSymbol {
	if l.dyn == nil {
		return globals
	}
	imported, ok := l.dyn.bySymbol[name]
	if !ok {
		return globals
	}
	entry := linkerSymbol{name: imported.versionedName(), value: symbolValue{undefined: true, kind: imported.kind}}
	return append(globals, entry)
}

func (l *linker) objectDefined() map[string]bool {
	defined := map[string]bool{}
	for _, sym := range l.obj.Symbols {
		if sym.Section != asm.SectionUndefined {
			defined[sym.Name] = true
		}
	}
	return defined
}

func (l *linker) definedNames() map[string]bool {
	defined := l.objectDefined()
	for _, boundary := range l.boundarySymbols() {
		defined[boundary.name] = true
	}
	return defined
}

func (l *linker) addresses() map[string]uint64 {
	out := make(map[string]uint64, len(l.symbols))
	for name, value := range l.symbols {
		if !strings.HasPrefix(name, ".L") {
			out[name] = l.emittedValue(value)
		}
	}
	if l.dyn != nil {
		for _, table := range l.dyn.tableSymbols() {
			out[table.name] = l.emittedValue(table.value)
		}
	}
	return out
}
