package link

import (
	"debug/elf"
	"fmt"
	"sort"
	"strings"

	"lowlang/internal/asm"
)

const (
	Interpreter       = "/lib64/ld-linux-x86-64.so.2"
	unversionedIndex  = 0
	firstVersionIndex = 2
	boundBssStart     = "__bss_start"
	boundEdata        = "_edata"
	boundEnd          = "_end"
)

type dynamicSymbol struct {
	name         string
	library      *Library
	kind         elf.SymType
	version      string
	index        uint32
	nameOffset   uint32
	versionIndex uint16
}

type gotSlot struct {
	name     string
	imported *dynamicSymbol
}

type versionNeed struct {
	name       string
	index      uint16
	nameOffset uint32
}

type libraryNeeds struct {
	library  *Library
	versions []versionNeed
}

type dynamicSections struct {
	interp  *section
	hash    *section
	gnuHash *section
	dynsym  *section
	dynstr  *section
	versym  *section
	verneed *section
	relaDyn *section
	relaPlt *section
	plt     *section
	dynamic *section
	got     *section
	gotPlt  *section
	pltGot  *section
}

type dynamicLink struct {
	libraries     []*Library
	neededOffsets []uint32
	symbols       []*dynamicSymbol
	bySymbol      map[string]*dynamicSymbol
	slots         []*gotSlot
	slotIndex     map[string]int
	pltSet        map[string]bool
	plts          []*dynamicSymbol
	pltIndex      map[string]int
	pltGots       []*dynamicSymbol
	pltGotIndex   map[string]int
	pointer       map[string]bool
	hashed        []*dynamicSymbol
	gnu           gnuHashLayout
	wanted        map[string]bool
	wantedOrder   []string
	strings       *stringTable
	needs         []libraryNeeds
	sections      dynamicSections
}

func newDynamicLink(libraries []*Library) *dynamicLink {
	return &dynamicLink{
		libraries:   libraries,
		bySymbol:    map[string]*dynamicSymbol{},
		slotIndex:   map[string]int{},
		pltIndex:    map[string]int{},
		pltGotIndex: map[string]int{},
		pltSet:      map[string]bool{},
		pointer:     map[string]bool{},
		wanted:      map[string]bool{},
	}
}

func isGOTKind(kind asm.FixupKind) bool {
	return kind == asm.GOTPCREL || kind == asm.GOTPCRELX || kind == asm.REXGOTPCRELX
}

func (l *linker) definedNames() map[string]bool {
	defined := map[string]bool{boundBssStart: true, boundEdata: true, boundEnd: true}
	for _, sym := range l.obj.Symbols {
		if sym.Section != asm.SectionUndefined {
			defined[sym.Name] = true
		}
	}
	return defined
}

func (l *linker) addDynamicSections() error {
	d := l.dyn
	d.collectReferences(l)
	if err := d.bind(l); err != nil {
		return err
	}
	d.buildStrings()
	l.sections = append(l.sections, d.synthesize()...)
	return nil
}

func (d *dynamicLink) collectReferences(l *linker) {
	defined := l.definedNames()
	for i, in := range l.obj.Sections {
		if _, loaded := l.byInput[i]; !loaded {
			continue
		}
		for _, f := range in.Fixups {
			d.reference(f, defined[f.Symbol])
		}
	}
}

func (d *dynamicLink) reference(f asm.Fixup, local bool) {
	switch {
	case isGOTKind(f.Kind):
		d.addSlot(f.Symbol, local)
		if f.Kind != asm.GOTPCRELX {
			d.pointer[f.Symbol] = true
		}
	case f.Kind == asm.PLT32 && !local:
		d.addPlt(f.Symbol)
	}
}

func (d *dynamicLink) addSlot(name string, local bool) {
	if _, present := d.slotIndex[name]; present {
		return
	}
	d.slotIndex[name] = len(d.slots)
	d.slots = append(d.slots, &gotSlot{name: name})
	if !local {
		d.want(name)
	}
}

func (d *dynamicLink) addPlt(name string) {
	if d.pltSet[name] {
		return
	}
	d.pltSet[name] = true
	d.want(name)
}

func (d *dynamicLink) want(name string) {
	if d.wanted[name] {
		return
	}
	d.wanted[name] = true
	d.wantedOrder = append(d.wantedOrder, name)
}

func (d *dynamicLink) importOrder(l *linker) []string {
	ordered := make([]string, 0, len(d.wantedOrder))
	seen := map[string]bool{}
	for _, sym := range l.obj.Symbols {
		if d.wanted[sym.Name] && !seen[sym.Name] {
			seen[sym.Name] = true
			ordered = append(ordered, sym.Name)
		}
	}
	for _, name := range d.wantedOrder {
		if !seen[name] {
			ordered = append(ordered, name)
		}
	}
	return ordered
}

func (d *dynamicLink) bind(l *linker) error {
	var missing []string
	var resolved []*dynamicSymbol
	for _, name := range d.importOrder(l) {
		sym, found := d.lookup(name)
		if !found {
			missing = append(missing, name)
			continue
		}
		resolved = append(resolved, sym)
	}
	if len(missing) > 0 {
		return d.undefinedError(missing)
	}
	d.assignIndices(resolved)
	d.orderTables()
	return nil
}

func (d *dynamicLink) assignIndices(resolved []*dynamicSymbol) {
	var plain []*dynamicSymbol
	for _, sym := range resolved {
		if d.isHashed(sym) {
			d.hashed = append(d.hashed, sym)
		} else {
			plain = append(plain, sym)
		}
	}
	d.gnu = newGnuHashLayout(len(d.hashed), uint32(len(plain)+1))
	sort.SliceStable(d.hashed, func(a, b int) bool {
		return d.gnu.bucket(d.hashed[a].name) < d.gnu.bucket(d.hashed[b].name)
	})
	d.symbols = append(plain, d.hashed...)
	for i, sym := range d.symbols {
		sym.index = uint32(i + 1)
		d.bySymbol[sym.name] = sym
	}
}

func (d *dynamicLink) orderTables() {
	for _, sym := range d.symbols {
		if !d.pltSet[sym.name] {
			continue
		}
		if _, hasSlot := d.slotIndex[sym.name]; hasSlot {
			d.pltGotIndex[sym.name] = len(d.pltGots)
			d.pltGots = append(d.pltGots, sym)
			continue
		}
		d.pltIndex[sym.name] = len(d.plts)
		d.plts = append(d.plts, sym)
	}
	var locals, imported []*gotSlot
	for _, slot := range d.slots {
		slot.imported = d.bySymbol[slot.name]
		if slot.imported == nil {
			locals = append(locals, slot)
		} else {
			imported = append(imported, slot)
		}
	}
	sort.Slice(imported, func(a, b int) bool { return d.gotRank(imported[a]) < d.gotRank(imported[b]) })
	d.slots = append(locals, imported...)
	for i, slot := range d.slots {
		d.slotIndex[slot.name] = i
	}
}

func (d *dynamicLink) undefinedError(missing []string) error {
	sort.Strings(missing)
	sonames := make([]string, len(d.libraries))
	for i, library := range d.libraries {
		sonames[i] = library.Soname
	}
	return fmt.Errorf("link: undefined symbol %s (not exported by %s)", strings.Join(missing, ", "), strings.Join(sonames, ", "))
}

func (d *dynamicLink) lookup(name string) (*dynamicSymbol, bool) {
	for _, library := range d.libraries {
		sym, found := library.symbols[name]
		if found {
			return &dynamicSymbol{name: name, library: library, kind: importedKind(sym.kind), version: sym.version}, true
		}
	}
	return nil, false
}

func importedKind(kind elf.SymType) elf.SymType {
	switch kind {
	case elf.STT_FUNC, elf.STT_GNU_IFUNC:
		return elf.STT_FUNC
	case elf.STT_OBJECT, elf.STT_TLS:
		return kind
	}
	return elf.STT_NOTYPE
}

func (d *dynamicLink) imports(name string) bool {
	_, imported := d.bySymbol[name]
	return imported
}

func (d *dynamicLink) gotAddress(name string) uint64 {
	return d.sections.got.addr + 8*uint64(d.slotIndex[name])
}

func (d *dynamicLink) pltAddress(name string) (uint64, bool) {
	if index, present := d.pltIndex[name]; present {
		return d.sections.plt.addr + pltEntrySize*uint64(index+1), true
	}
	if index, present := d.pltGotIndex[name]; present {
		return d.sections.pltGot.addr + pltGotEntrySize*uint64(index), true
	}
	return 0, false
}

func (d *dynamicLink) buildStrings() {
	d.strings = newStringTable()
	for _, sym := range d.symbols {
		sym.nameOffset = d.strings.addOnce(sym.name)
	}
	for _, library := range d.libraries {
		d.neededOffsets = append(d.neededOffsets, d.strings.addOnce(library.Soname))
	}
	d.assignVersions()
}

type versionKey struct {
	library *Library
	name    string
}

func (d *dynamicLink) assignVersions() {
	next := uint16(firstVersionIndex)
	indices := map[versionKey]uint16{}
	byLibrary := map[*Library][]versionNeed{}
	for _, sym := range d.symbols {
		if sym.version == "" {
			sym.versionIndex = unversionedIndex
			continue
		}
		key := versionKey{sym.library, sym.version}
		index, assigned := indices[key]
		if !assigned {
			index = next
			next++
			indices[key] = index
			need := versionNeed{name: sym.version, index: index, nameOffset: d.strings.addOnce(sym.version)}
			byLibrary[sym.library] = append(byLibrary[sym.library], need)
		}
		sym.versionIndex = index
	}
	for _, library := range d.libraries {
		versions := byLibrary[library]
		if len(versions) == 0 {
			continue
		}
		sort.Slice(versions, func(a, b int) bool { return versions[a].index > versions[b].index })
		d.needs = append(d.needs, libraryNeeds{library: library, versions: versions})
	}
}

func (d *dynamicLink) versioned() bool {
	return len(d.needs) > 0
}

func (s *dynamicSymbol) versionedName() string {
	if s.version == "" {
		return s.name
	}
	return fmt.Sprintf("%s@%s", s.name, s.version)
}

func (d *dynamicLink) tableSymbols() []linkerSymbol {
	dynamic := d.sections.dynamic
	gotPlt := d.sections.gotPlt
	return []linkerSymbol{
		{"_DYNAMIC", symbolValue{value: dynamic.addr, section: dynamic, kind: elf.STT_OBJECT}},
		{"_GLOBAL_OFFSET_TABLE_", symbolValue{value: gotPlt.addr, section: gotPlt, kind: elf.STT_OBJECT}},
	}
}

func (d *dynamicLink) gotRank(slot *gotSlot) int64 {
	rank := int64(slot.imported.index)
	if d.isHashed(slot.imported) {
		rank -= int64(len(d.symbols)) + 1
	}
	return rank
}
