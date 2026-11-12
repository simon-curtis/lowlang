package link

import (
	"debug/elf"
	"encoding/binary"
	"math/bits"
	"slices"

	"lowlang/internal/asm"
)

const (
	pltEntrySize    uint64 = 16
	gotEntrySize    uint64 = 8
	relaSize        uint64 = 24
	dynEntrySize    uint64 = 16
	verneedSize     uint32 = 16
	vernauxSize     uint32 = 16
	spareDynEntries        = 5
	pltGotEntrySize uint64 = 8
	gnuHashHeader   uint64 = 16
	bloomWordBytes  uint64 = 8
	bloomWordBits          = 64
	bloomShift             = 6
	hashHeaderSize  uint64 = 8
	firstUnhashed          = 1
)

type dynTag struct {
	tag   elf.DynTag
	value uint64
}

func synthetic(name, flags string, c class, align uint64, size uint64) *section {
	in := &asm.Section{Name: name, Flags: flags, Align: int64(align), Data: make([]byte, size)}
	return &section{in: in, class: c, size: size, align: align}
}

func (d *dynamicLink) synthesize() []*section {
	s := &d.sections
	interpreter := append([]byte(Interpreter), 0)
	s.interp = synthetic(".interp", "a", classInterp, 1, uint64(len(interpreter)))
	copy(s.interp.in.Data, interpreter)

	s.dynstr = synthetic(".dynstr", "a", classDynstr, 1, uint64(len(d.strings.data)))
	s.dynstr.shType = elf.SHT_STRTAB
	copy(s.dynstr.in.Data, d.strings.data)

	s.dynsym = synthetic(".dynsym", "a", classDynsym, 8, symSize*uint64(len(d.symbols)+1))
	s.dynsym.shType, s.dynsym.entsize, s.dynsym.linked, s.dynsym.info = elf.SHT_DYNSYM, symSize, s.dynstr, firstUnhashed
	d.fillDynsym()

	buckets := bucketCount(len(d.symbols) + 1)
	s.hash = synthetic(".hash", "a", classHash, 8, hashHeaderSize+4*uint64(buckets+len(d.symbols)+1))
	s.hash.shType, s.hash.entsize, s.hash.linked = elf.SHT_HASH, 4, s.dynsym
	d.fillHash(buckets)

	s.gnuHash = synthetic(".gnu.hash", "a", classGnuHash, 8, d.gnu.size())
	s.gnuHash.shType, s.gnuHash.linked = elf.SHT_GNU_HASH, s.dynsym
	d.fillGnuHash()

	sections := []*section{s.interp, s.hash, s.gnuHash, s.dynsym, s.dynstr}
	sections = append(sections, d.synthesizeVersions()...)
	sections = append(sections, d.synthesizeRelocations()...)
	sections = append(sections, d.synthesizeGot()...)

	count := uint64(len(d.tagList()) + spareDynEntries)
	s.dynamic = synthetic(".dynamic", "aw", classDynamic, 8, count*dynEntrySize)
	s.dynamic.shType, s.dynamic.entsize, s.dynamic.linked = elf.SHT_DYNAMIC, dynEntrySize, s.dynstr
	return append(sections, s.dynamic)
}

func (d *dynamicLink) synthesizeVersions() []*section {
	if !d.versioned() {
		return nil
	}
	s := &d.sections
	s.versym = synthetic(".gnu.version", "a", classVersym, 2, 2*uint64(len(d.symbols)+1))
	s.versym.shType, s.versym.entsize, s.versym.linked = elf.SHT_GNU_VERSYM, 2, s.dynsym
	for i, sym := range d.symbols {
		binary.LittleEndian.PutUint16(s.versym.in.Data[2*(i+1):], sym.versionIndex)
	}
	data := d.verneedData()
	s.verneed = synthetic(".gnu.version_r", "a", classVerneed, 8, uint64(len(data)))
	s.verneed.shType, s.verneed.linked, s.verneed.info = elf.SHT_GNU_VERNEED, s.dynstr, uint32(len(d.needs))
	copy(s.verneed.in.Data, data)
	return []*section{s.versym, s.verneed}
}

func (d *dynamicLink) verneedData() []byte {
	var data []byte
	for i, need := range d.needs {
		next := uint32(0)
		if i < len(d.needs)-1 {
			next = verneedSize + vernauxSize*uint32(len(need.versions))
		}
		data = binary.LittleEndian.AppendUint16(data, 1)
		data = binary.LittleEndian.AppendUint16(data, uint16(len(need.versions)))
		data = binary.LittleEndian.AppendUint32(data, d.strings.addOnce(need.library.Soname))
		data = binary.LittleEndian.AppendUint32(data, verneedSize)
		data = binary.LittleEndian.AppendUint32(data, next)
		data = d.appendVernaux(data, need)
	}
	return data
}

func (d *dynamicLink) appendVernaux(data []byte, need libraryNeeds) []byte {
	for j, version := range need.versions {
		next := uint32(0)
		if j < len(need.versions)-1 {
			next = vernauxSize
		}
		data = binary.LittleEndian.AppendUint32(data, sysvHash(version.name))
		data = binary.LittleEndian.AppendUint16(data, 0)
		data = binary.LittleEndian.AppendUint16(data, version.index)
		data = binary.LittleEndian.AppendUint32(data, version.nameOffset)
		data = binary.LittleEndian.AppendUint32(data, next)
	}
	return data
}

func sysvHash(name string) uint32 {
	var hash uint32
	for i := range len(name) {
		hash = hash<<4 + uint32(name[i])
		high := hash & 0xf0000000
		hash ^= high >> 24
		hash &^= high
	}
	return hash
}

func (d *dynamicLink) synthesizeRelocations() []*section {
	s := &d.sections
	var sections []*section
	if count := d.importedSlots(); count > 0 {
		s.relaDyn = synthetic(".rela.dyn", "a", classRelaDyn, 8, relaSize*uint64(count))
		s.relaDyn.shType, s.relaDyn.entsize, s.relaDyn.linked = elf.SHT_RELA, relaSize, s.dynsym
		sections = append(sections, s.relaDyn)
	}
	if len(d.plts) > 0 {
		s.relaPlt = synthetic(".rela.plt", "a", classRelaPlt, 8, relaSize*uint64(len(d.plts)))
		s.relaPlt.shType, s.relaPlt.entsize, s.relaPlt.linked = elf.SHT_RELA, relaSize, s.dynsym
		s.relaPlt.shFlags = elf.SHF_INFO_LINK
		s.plt = synthetic(".plt", "ax", classPlt, 16, pltEntrySize*uint64(len(d.plts)+1))
		s.plt.entsize = pltEntrySize
		sections = append(sections, s.relaPlt, s.plt)
	}
	if len(d.pltGots) > 0 {
		s.pltGot = synthetic(".plt.got", "ax", classPltGot, 8, pltGotEntrySize*uint64(len(d.pltGots)))
		s.pltGot.entsize = pltGotEntrySize
		sections = append(sections, s.pltGot)
	}
	return sections
}

func (d *dynamicLink) importedSlots() int {
	count := 0
	for _, slot := range d.slots {
		if slot.imported != nil {
			count++
		}
	}
	return count
}

func (d *dynamicLink) synthesizeGot() []*section {
	s := &d.sections
	var sections []*section
	if len(d.slots) > 0 {
		s.got = synthetic(".got", "aw", classGot, 8, gotEntrySize*uint64(len(d.slots)))
		s.got.entsize = gotEntrySize
		sections = append(sections, s.got)
	}
	s.gotPlt = synthetic(".got.plt", "aw", classGotPlt, 8, gotPltReserved+gotEntrySize*uint64(len(d.plts)))
	s.gotPlt.entsize = gotEntrySize
	if s.relaPlt != nil {
		s.relaPlt.infoSection = s.gotPlt
	}
	return append(sections, s.gotPlt)
}

func (d *dynamicLink) fillDynsym() {
	data := d.sections.dynsym.in.Data
	for _, sym := range d.symbols {
		entry := data[symSize*uint64(sym.index):]
		binary.LittleEndian.PutUint32(entry, sym.nameOffset)
		entry[4] = elf.ST_INFO(elf.STB_GLOBAL, sym.kind)
	}
}

var hashBucketSizes = []int{1, 3, 17, 37, 67, 97, 131, 197, 263, 521, 1031, 2053, 4099, 8209, 16411, 32771, 65537, 131101, 262147}

func bucketCount(symbols int) int {
	best := hashBucketSizes[0]
	for i := 0; i+1 < len(hashBucketSizes); i++ {
		best = hashBucketSizes[i]
		if symbols < hashBucketSizes[i+1] {
			break
		}
	}
	return best
}

func (d *dynamicLink) hashInsertionOrder() []*dynamicSymbol {
	ordered := slices.Clone(d.hashed)
	for _, sym := range d.symbols {
		if !d.isHashed(sym) {
			ordered = append(ordered, sym)
		}
	}
	return ordered
}

func (d *dynamicLink) isHashed(sym *dynamicSymbol) bool {
	return d.pointer[sym.name] && d.pltSet[sym.name]
}

func (d *dynamicLink) fillHash(buckets int) {
	data := d.sections.hash.in.Data
	binary.LittleEndian.PutUint32(data[0:], uint32(buckets))
	binary.LittleEndian.PutUint32(data[4:], uint32(len(d.symbols)+1))
	table := data[hashHeaderSize:]
	chains := table[4*buckets:]
	for _, sym := range d.hashInsertionOrder() {
		slot := table[4*(sysvHash(sym.name)%uint32(buckets)):]
		binary.LittleEndian.PutUint32(chains[4*sym.index:], binary.LittleEndian.Uint32(slot))
		binary.LittleEndian.PutUint32(slot, sym.index)
	}
}

type gnuHashLayout struct {
	hashed      int
	buckets     uint32
	firstHashed uint32
	bloomWords  uint32
	shift       uint32
}

func newGnuHashLayout(hashed int, firstHashed uint32) gnuHashLayout {
	if hashed == 0 {
		return gnuHashLayout{buckets: 1, firstHashed: firstUnhashed, bloomWords: 1}
	}
	buckets := max(bucketCount(hashed), 2)
	return gnuHashLayout{
		hashed:      hashed,
		buckets:     uint32(buckets),
		firstHashed: firstHashed,
		bloomWords:  uint32(bloomBits(hashed) / bloomWordBits),
		shift:       bloomShift,
	}
}

func bloomBits(hashed int) int {
	ceilLog2 := bits.Len(uint(hashed - 1))
	log2 := ceilLog2 + 1
	switch {
	case log2 < 3:
		log2 = 5
	case (1<<(log2-2))&hashed != 0:
		log2 += 3
	default:
		log2 += 2
	}
	return 1 << max(log2, 6)
}

func (g gnuHashLayout) size() uint64 {
	return gnuHashHeader + bloomWordBytes*uint64(g.bloomWords) + 4*uint64(g.buckets) + 4*uint64(g.hashed)
}

func (g gnuHashLayout) bucket(name string) uint32 {
	return gnuHashOf(name) % g.buckets
}

func gnuHashOf(name string) uint32 {
	hash := uint32(5381)
	for i := range len(name) {
		hash = hash*33 + uint32(name[i])
	}
	return hash
}

func (d *dynamicLink) fillGnuHash() {
	g := d.gnu
	data := d.sections.gnuHash.in.Data
	binary.LittleEndian.PutUint32(data[0:], g.buckets)
	binary.LittleEndian.PutUint32(data[4:], g.firstHashed)
	binary.LittleEndian.PutUint32(data[8:], g.bloomWords)
	binary.LittleEndian.PutUint32(data[12:], g.shift)
	bloom := data[gnuHashHeader:]
	table := bloom[bloomWordBytes*uint64(g.bloomWords):]
	chain := table[4*g.buckets:]
	for i, sym := range d.hashed {
		hash := gnuHashOf(sym.name)
		word := bloom[bloomWordBytes*uint64((hash/bloomWordBits)%g.bloomWords):]
		mask := binary.LittleEndian.Uint64(word)
		mask |= 1<<(hash%bloomWordBits) | 1<<((hash>>g.shift)%bloomWordBits)
		binary.LittleEndian.PutUint64(word, mask)
		bucket := g.bucket(sym.name)
		if binary.LittleEndian.Uint32(table[4*bucket:]) == 0 {
			binary.LittleEndian.PutUint32(table[4*bucket:], sym.index)
		}
		last := i+1 == len(d.hashed) || g.bucket(d.hashed[i+1].name) != bucket
		value := hash &^ 1
		if last {
			value |= 1
		}
		binary.LittleEndian.PutUint32(chain[4*i:], value)
	}
}

func (l *linker) fillDynamic() {
	d := l.dyn
	d.fillGot(l)
	d.fillGotPlt()
	d.fillPlt()
	d.fillPltGot()
	d.fillRelocations()
	d.fillTags()
}

func (d *dynamicLink) fillGot(l *linker) {
	got := d.sections.got
	if got == nil {
		return
	}
	for i, slot := range d.slots {
		if slot.imported == nil {
			binary.LittleEndian.PutUint64(got.in.Data[gotEntrySize*uint64(i):], l.symbols[slot.name].value)
		}
	}
}

func (d *dynamicLink) fillGotPlt() {
	s := &d.sections
	data := s.gotPlt.in.Data
	binary.LittleEndian.PutUint64(data, s.dynamic.addr)
	for i := range d.plts {
		entry := s.plt.addr + pltEntrySize*uint64(i+1)
		binary.LittleEndian.PutUint64(data[gotPltReserved+gotEntrySize*uint64(i):], entry+6)
	}
}

func (d *dynamicLink) fillPlt() {
	s := &d.sections
	if s.plt == nil {
		return
	}
	data := s.plt.in.Data
	base := s.plt.addr
	got := s.gotPlt.addr
	copy(data, []byte{0xff, 0x35, 0, 0, 0, 0, 0xff, 0x25, 0, 0, 0, 0, 0x0f, 0x1f, 0x40, 0x00})
	putRel32(data[2:], got+8, base+6)
	putRel32(data[8:], got+16, base+12)
	for i := range d.plts {
		offset := pltEntrySize * uint64(i+1)
		entry := base + offset
		slot := got + gotPltReserved + gotEntrySize*uint64(i)
		code := data[offset:]
		copy(code, []byte{0xff, 0x25, 0, 0, 0, 0, 0x68, 0, 0, 0, 0, 0xe9, 0, 0, 0, 0})
		putRel32(code[2:], slot, entry+6)
		binary.LittleEndian.PutUint32(code[7:], uint32(i))
		putRel32(code[12:], base, entry+16)
	}
}

func (d *dynamicLink) fillPltGot() {
	section := d.sections.pltGot
	if section == nil {
		return
	}
	for i, sym := range d.pltGots {
		offset := pltGotEntrySize * uint64(i)
		code := section.in.Data[offset:]
		copy(code, []byte{0xff, 0x25, 0, 0, 0, 0, 0x66, 0x90})
		putRel32(code[2:], d.gotAddress(sym.name), section.addr+offset+6)
	}
}

func putRel32(field []byte, target, next uint64) {
	binary.LittleEndian.PutUint32(field, uint32(int64(target)-int64(next)))
}

func (d *dynamicLink) fillRelocations() {
	s := &d.sections
	next := 0
	for i, slot := range d.slots {
		if slot.imported == nil {
			continue
		}
		address := s.got.addr + gotEntrySize*uint64(i)
		putRela(s.relaDyn.in.Data[relaSize*uint64(next):], address, slot.imported.index, elf.R_X86_64_GLOB_DAT)
		next++
	}
	for i, sym := range d.plts {
		address := s.gotPlt.addr + gotPltReserved + gotEntrySize*uint64(i)
		putRela(s.relaPlt.in.Data[relaSize*uint64(i):], address, sym.index, elf.R_X86_64_JMP_SLOT)
	}
}

func putRela(field []byte, address uint64, symbol uint32, kind elf.R_X86_64) {
	binary.LittleEndian.PutUint64(field, address)
	binary.LittleEndian.PutUint64(field[8:], uint64(symbol)<<32|uint64(kind))
}

func (d *dynamicLink) tagList() []dynTag {
	s := &d.sections
	var tags []dynTag
	for _, offset := range d.neededOffsets {
		tags = append(tags, dynTag{elf.DT_NEEDED, uint64(offset)})
	}
	tags = append(tags,
		dynTag{elf.DT_HASH, s.hash.addr},
		dynTag{elf.DT_GNU_HASH, s.gnuHash.addr},
		dynTag{elf.DT_STRTAB, s.dynstr.addr},
		dynTag{elf.DT_SYMTAB, s.dynsym.addr},
		dynTag{elf.DT_STRSZ, s.dynstr.size},
		dynTag{elf.DT_SYMENT, symSize},
		dynTag{elf.DT_DEBUG, 0},
		dynTag{elf.DT_PLTGOT, s.gotPlt.addr},
	)
	if s.relaPlt != nil {
		tags = append(tags,
			dynTag{elf.DT_PLTRELSZ, s.relaPlt.size},
			dynTag{elf.DT_PLTREL, uint64(elf.DT_RELA)},
			dynTag{elf.DT_JMPREL, s.relaPlt.addr},
		)
	}
	if s.relaDyn != nil {
		tags = append(tags,
			dynTag{elf.DT_RELA, s.relaDyn.addr},
			dynTag{elf.DT_RELASZ, s.relaDyn.size},
			dynTag{elf.DT_RELAENT, relaSize},
		)
	}
	if s.verneed != nil {
		tags = append(tags,
			dynTag{elf.DT_VERNEED, s.verneed.addr},
			dynTag{elf.DT_VERNEEDNUM, uint64(len(d.needs))},
			dynTag{elf.DT_VERSYM, s.versym.addr},
		)
	}
	return append(tags, dynTag{elf.DT_NULL, 0})
}

func (d *dynamicLink) fillTags() {
	data := d.sections.dynamic.in.Data
	for i, entry := range d.tagList() {
		binary.LittleEndian.PutUint64(data[dynEntrySize*uint64(i):], uint64(entry.tag))
		binary.LittleEndian.PutUint64(data[dynEntrySize*uint64(i)+8:], entry.value)
	}
}
