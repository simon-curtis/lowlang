package main

import (
	"encoding/binary"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

const (
	codePoints     = 0x110000
	propsShift     = 7
	caseShift      = 7
	propsBlockMask = 1<<propsShift - 1
	caseBlockMask  = 1<<caseShift - 1
	maxRecordID    = 0xFFFF
	foldFrom       = 0x40000
	foldTo         = 0xE0000
	foldGap        = foldTo - foldFrom
)

const (
	caseUpper = iota
	caseLower
	caseTitle
	caseFields
)

const (
	propsRecordSize        = wordSize
	caseFieldWidth         = wordSize
	caseRecordSize         = caseFields * caseFieldWidth
	specialCodeOffset      = 0
	specialReferenceOffset = specialCodeOffset + wordSize
	specialLengthOffset    = specialReferenceOffset + u16Width
	specialFieldStride     = 2 * u16Width
	specialSize            = specialReferenceOffset + caseFields*specialFieldStride
	specialPoolUnit        = wordSize
)

var propertyTables = []string{
	"props_index.bin", "props_blocks.bin", "props_records.bin",
	"case_index.bin", "case_blocks.bin", "case_records.bin",
	"special.bin", "special_pool.bin",
	"breaks_index.bin", "breaks_blocks.bin", "breaks_records.bin",
}

const propertyLayoutFile = "property_layout.low"

var categoryNames = []string{
	"Cn", "Lu", "Ll", "Lt", "Lm", "Lo", "Mn", "Mc", "Me", "Nd", "Nl", "No", "Pc", "Pd", "Ps",
	"Pe", "Pi", "Pf", "Po", "Sm", "Sc", "Sk", "So", "Zs", "Zl", "Zp", "Cc", "Cf", "Cs", "Co",
}

var widthNames = []string{"N", "A", "F", "H", "Na", "W"}

var scriptNames = []string{
	"Unknown", "Common", "Inherited", "Latin", "Greek", "Cyrillic", "Armenian", "Hebrew", "Arabic",
	"Syriac", "Thaana", "Devanagari", "Bengali", "Gurmukhi", "Gujarati", "Tamil", "Telugu", "Kannada",
	"Malayalam", "Sinhala", "Thai", "Lao", "Tibetan", "Myanmar", "Georgian", "Hangul", "Ethiopic",
	"Khmer", "Mongolian", "Hiragana", "Katakana", "Bopomofo", "Han",
}

var breakNames = []string{
	"Other", "CR", "LF", "Control", "Extend", "ZWJ", "Regional_Indicator", "Prepend", "SpacingMark",
	"L", "V", "T", "LV", "LVT",
}

var conjunctNames = []string{"None", "Consonant", "Linker", "Extend"}

const (
	categoryMask     = 0x1F
	widthMask        = 7
	scriptMask       = 63
	breakMask        = 15
	conjunctMask     = 3
	categoryShift    = 0
	widthShift       = 5
	scriptShift      = 8
	breakShift       = 14
	conjunctShift    = 18
	pictographShift  = 20
	spaceShift       = 21
	xidStartShift    = 22
	xidContinueShift = 23
	pictographFlag   = 1 << pictographShift
	spaceFlag        = 1 << spaceShift
	xidStartFlag     = 1 << xidStartShift
	xidContinueFlag  = 1 << xidContinueShift
)

type caseDeltas struct {
	upper, lower, title int32
}

type casing struct {
	lower, upper, title []rune
}

type propertyDatabase struct {
	rangeStart rune
	props      []uint32
	deltas     []caseDeltas
	special    map[rune]casing
	breaks     breakClasses
}

func generateProperties(dir, output string) error {
	db := propertyDatabase{
		props:   make([]uint32, codePoints),
		deltas:  make([]caseDeltas, codePoints),
		special: map[rune]casing{},
		breaks:  newBreakClasses(),
	}
	steps := []struct {
		file   string
		handle func(first, last rune, fields []string) error
	}{
		{"UnicodeData.txt", db.readUnicodeData},
		{"EastAsianWidth.txt", db.readWidth},
		{"Scripts.txt", db.readScript},
		{"GraphemeBreakProperty.txt", db.readBreak},
		{"emoji-data.txt", db.readPictographic},
		{"PropList.txt", db.readSpace},
		{"DerivedCoreProperties.txt", db.readDerived},
		{"LineBreak.txt", db.readLineBreak},
		{"WordBreakProperty.txt", db.readWordBreak},
	}
	for _, step := range steps {
		if err := eachRange(filepath.Join(dir, step.file), step.handle); err != nil {
			return err
		}
	}
	if err := eachLine(filepath.Join(dir, "SpecialCasing.txt"), db.readSpecial); err != nil {
		return err
	}
	sections, err := db.tables()
	if err != nil {
		return err
	}
	return writeSections(output, sections)
}

func eachRange(path string, handle func(first, last rune, fields []string) error) error {
	return eachLine(path, func(fields []string) error {
		first, rest, ranged := strings.Cut(fields[0], "..")
		last := first
		if ranged {
			last = rest
		}
		from, err := parseCode(first)
		if err != nil {
			return err
		}
		to, err := parseCode(last)
		if err != nil {
			return err
		}
		return handle(from, to, fields[1:])
	})
}

func indexOf(names []string, name string) (int, error) {
	at := slices.Index(names, name)
	if at < 0 {
		return 0, fmt.Errorf("unknown value %q", name)
	}
	return at, nil
}

func (db *propertyDatabase) setField(first, last rune, mask uint32, shift uint, value uint32) {
	for c := first; c <= last; c++ {
		db.props[c] = db.props[c]&^(mask<<shift) | value<<shift
	}
}

func (db *propertyDatabase) setFlag(first, last rune, flag uint32) {
	for c := first; c <= last; c++ {
		db.props[c] |= flag
	}
}

func parseDelta(field string, c rune) (int32, error) {
	if field == "" {
		return 0, nil
	}
	to, err := parseCode(field)
	return int32(to - c), err
}

func (db *propertyDatabase) readUnicodeData(first, last rune, fields []string) error {
	if len(fields) < 14 {
		return fmt.Errorf("short UnicodeData line for %04X", first)
	}
	switch {
	case strings.HasSuffix(fields[0], "First>"):
		db.rangeStart = first
		return nil
	case strings.HasSuffix(fields[0], "Last>"):
		first = db.rangeStart
	}
	category, err := indexOf(categoryNames, fields[1])
	if err != nil {
		return err
	}
	db.setField(first, last, categoryMask, categoryShift, uint32(category))
	upper, err := parseDelta(fields[11], first)
	if err != nil {
		return err
	}
	lower, err := parseDelta(fields[12], first)
	if err != nil {
		return err
	}
	title, err := parseDelta(fields[13], first)
	if err != nil {
		return err
	}
	db.deltas[first] = caseDeltas{upper, lower, title}
	return nil
}

func (db *propertyDatabase) readWidth(first, last rune, fields []string) error {
	width, err := indexOf(widthNames, fields[0])
	if err != nil {
		return err
	}
	db.setField(first, last, widthMask, widthShift, uint32(width))
	return nil
}

func (db *propertyDatabase) readScript(first, last rune, fields []string) error {
	script := slices.Index(scriptNames, fields[0])
	if script < 0 {
		script = len(scriptNames)
	}
	db.setField(first, last, scriptMask, scriptShift, uint32(script))
	return nil
}

func (db *propertyDatabase) readBreak(first, last rune, fields []string) error {
	kind, err := indexOf(breakNames, fields[0])
	if err != nil {
		return err
	}
	db.setField(first, last, breakMask, breakShift, uint32(kind))
	return nil
}

func (db *propertyDatabase) readPictographic(first, last rune, fields []string) error {
	if fields[0] == "Extended_Pictographic" {
		db.setFlag(first, last, pictographFlag)
	}
	return nil
}

func (db *propertyDatabase) readSpace(first, last rune, fields []string) error {
	if fields[0] == "White_Space" {
		db.setFlag(first, last, spaceFlag)
	}
	return nil
}

func (db *propertyDatabase) readDerived(first, last rune, fields []string) error {
	switch fields[0] {
	case "XID_Start":
		db.setFlag(first, last, xidStartFlag)
	case "XID_Continue":
		db.setFlag(first, last, xidContinueFlag)
	case "InCB":
		kind, err := indexOf(conjunctNames, fields[1])
		if err != nil {
			return err
		}
		db.setField(first, last, conjunctMask, conjunctShift, uint32(kind))
	}
	return nil
}

func (db *propertyDatabase) readSpecial(fields []string) error {
	if len(fields) != 5 || fields[4] != "" {
		return nil
	}
	c, err := parseCode(fields[0])
	if err != nil {
		return err
	}
	var entry casing
	for i, target := range []*[]rune{&entry.lower, &entry.title, &entry.upper} {
		if *target, err = parseCodes(fields[1+i]); err != nil {
			return err
		}
	}
	db.special[c] = entry
	return nil
}

type trie struct {
	index   []byte
	blocks  []byte
	records []byte
}

func foldPlanes[R comparable](perCode []R) ([]R, error) {
	var zero R
	for _, v := range perCode[foldFrom:foldTo] {
		if v != zero {
			return nil, fmt.Errorf("code points between U+%X and U+%X are not all default", foldFrom, foldTo)
		}
	}
	folded := slices.Clone(perCode[:foldFrom])
	return append(folded, perCode[foldTo:]...), nil
}

func buildRecordTrie[R comparable](all []R, shift uint, encode func(R) []byte) (trie, error) {
	var out trie
	perCode, err := foldPlanes(all)
	if err != nil {
		return out, err
	}
	var zero R
	limit := len(perCode)
	for limit > 0 && perCode[limit-1] == zero {
		limit--
	}
	size := 1 << shift
	records := []R{zero}
	ids := map[R]int{zero: 0}
	seen := map[string]int{}
	var stage2 [][]byte
	for start := 0; start < limit; start += size {
		block := make([]byte, 0, size*2)
		for i := range size {
			var r R
			if start+i < limit {
				r = perCode[start+i]
			}
			id, ok := ids[r]
			if !ok {
				id = len(records)
				ids[r] = id
				records = append(records, r)
			}
			if id > maxRecordID {
				return out, fmt.Errorf("record ids overflow 16 bits")
			}
			block = appendU16(block, id)
		}
		index, ok := seen[string(block)]
		if !ok {
			index = len(stage2)
			seen[string(block)] = index
			stage2 = append(stage2, block)
		}
		if index > maxRecordID {
			return out, fmt.Errorf("block index overflows 16 bits")
		}
		out.index = appendU16(out.index, index)
	}
	for _, block := range stage2 {
		out.blocks = append(out.blocks, block...)
	}
	for _, r := range records {
		out.records = append(out.records, encode(r)...)
	}
	return out, nil
}

func (db *propertyDatabase) specialTable() (entries, pooled []byte, err error) {
	codes := make([]rune, 0, len(db.special))
	for c := range db.special {
		codes = append(codes, c)
	}
	slices.Sort(codes)
	pool := &sequencePool{offsets: map[string]int{}}
	entries = make([]byte, 0, len(codes)*specialSize)
	for _, c := range codes {
		encoded, err := pool.encodeSpecial(c, db.special[c])
		if err != nil {
			return nil, nil, err
		}
		entries = append(entries, encoded...)
	}
	return entries, pool.data, nil
}

func (p *sequencePool) encodeSpecial(c rune, entry casing) ([]byte, error) {
	out := make([]byte, specialSize)
	binary.LittleEndian.PutUint32(out[specialCodeOffset:], uint32(c))
	sequences := [caseFields][]rune{caseUpper: entry.upper, caseLower: entry.lower, caseTitle: entry.title}
	for field, seq := range sequences {
		key, err := p.add(seq)
		if err != nil {
			return nil, err
		}
		reference := specialReferenceOffset + field*specialFieldStride
		copy(out[reference:], p.appendReference(nil, key))
		binary.LittleEndian.PutUint16(out[specialLengthOffset+field*specialFieldStride:], uint16(len(seq)))
	}
	return out, nil
}

func (db *propertyDatabase) tables() ([]section, error) {
	props, err := buildRecordTrie(db.props, propsShift, func(v uint32) []byte {
		return appendU32(nil, rune(v))
	})
	if err != nil {
		return nil, err
	}
	deltas, err := buildRecordTrie(db.deltas, caseShift, func(d caseDeltas) []byte {
		fields := [caseFields]int32{caseUpper: d.upper, caseLower: d.lower, caseTitle: d.title}
		out := make([]byte, 0, caseRecordSize)
		for _, delta := range fields {
			out = appendU32(out, rune(delta))
		}
		return out
	})
	if err != nil {
		return nil, err
	}
	special, specialPool, err := db.specialTable()
	if err != nil {
		return nil, err
	}
	breaks, err := db.breakTrie()
	if err != nil {
		return nil, err
	}
	sections := namedSections(propertyTables,
		props.index, props.blocks, props.records,
		deltas.index, deltas.blocks, deltas.records,
		special, specialPool,
		breaks.index, breaks.blocks, breaks.records)
	return append(sections, section{propertyLayoutFile, []byte(propertyLayoutSource())}), nil
}

func propertyLayoutSource() string {
	constants := []layoutConstant{
		lowConstant("props_block_shift", propsShift, lowInt),
		lowConstant("props_block_mask", propsBlockMask, lowWord),
		lowConstant("props_record_size", propsRecordSize, lowInt),
		lowConstant("case_block_shift", caseShift, lowInt),
		lowConstant("case_block_mask", caseBlockMask, lowWord),
		lowConstant("case_record_size", caseRecordSize, lowInt),
		lowConstant("case_field_width", caseFieldWidth, lowInt),
		lowConstant("case_upper_field", caseUpper, lowInt),
		lowConstant("case_lower_field", caseLower, lowInt),
		lowConstant("case_title_field", caseTitle, lowInt),
		lowConstant("special_size", specialSize, lowInt),
		lowConstant("special_code_offset", specialCodeOffset, lowInt),
		lowConstant("special_reference_offset", specialReferenceOffset, lowInt),
		lowConstant("special_length_offset", specialLengthOffset, lowInt),
		lowConstant("special_field_stride", specialFieldStride, lowInt),
		lowConstant("special_pool_unit", specialPoolUnit, lowInt),
		lowConstant("fold_from", foldFrom, lowCodePoint),
		lowConstant("fold_to", foldTo, lowCodePoint),
		lowConstant("fold_gap", foldGap, lowCodePoint),
		lowConstant("property_category_shift", categoryShift, lowInt),
		lowConstant("property_category_mask", categoryMask, lowWord),
		lowConstant("property_width_shift", widthShift, lowInt),
		lowConstant("property_width_mask", widthMask, lowWord),
		lowConstant("property_script_shift", scriptShift, lowInt),
		lowConstant("property_script_mask", scriptMask, lowWord),
		lowConstant("property_break_shift", breakShift, lowInt),
		lowConstant("property_break_mask", breakMask, lowWord),
		lowConstant("property_conjunct_shift", conjunctShift, lowInt),
		lowConstant("property_conjunct_mask", conjunctMask, lowWord),
		lowConstant("property_pictographic_shift", pictographShift, lowInt),
		lowConstant("property_space_shift", spaceShift, lowInt),
		lowConstant("property_identifier_start_shift", xidStartShift, lowInt),
		lowConstant("property_identifier_continue_shift", xidContinueShift, lowInt),
	}
	constants = append(constants, classConstants("category_", categoryNames, lowWord)...)
	constants = append(constants, classConstants("width_", widthNames, lowWord)...)
	constants = append(constants, classConstants("break_", breakNames, lowInt)...)
	constants = append(constants, classConstants("conjunct_", conjunctNames, lowInt)...)
	constants = append(constants, breakLayoutConstants()...)
	return layoutSource(constants, propertyTables)
}
