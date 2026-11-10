package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

const (
	blockShift   = 6
	blockSize    = 1 << blockShift
	hangulBase   = 0xAC00
	hangulCount  = 11172
	hangulLBase  = 0x1100
	hangulVBase  = 0x1161
	hangulTBase  = 0x11A7
	hangulTCount = 28
	hangulNCount = 21 * hangulTCount
)

type character struct {
	class      int
	compatible bool
	mapping    []rune
}

type database struct {
	chars     map[rune]character
	folds     map[rune][]rune
	excluded  map[rune]bool
	canonical map[rune][]rune
	compat    map[rune][]rune
}

type record struct {
	class     int
	canonical string
	compat    string
	fold      string
}

func generateUnicode(dir, output string) error {
	db := database{
		chars:     map[rune]character{},
		folds:     map[rune][]rune{},
		excluded:  map[rune]bool{},
		canonical: map[rune][]rune{},
		compat:    map[rune][]rune{},
	}
	if err := eachLine(filepath.Join(dir, "UnicodeData.txt"), db.readCharacter); err != nil {
		return err
	}
	if err := eachLine(filepath.Join(dir, "CaseFolding.txt"), db.readFolding); err != nil {
		return err
	}
	if err := eachLine(filepath.Join(dir, "DerivedNormalizationProps.txt"), db.readExclusion); err != nil {
		return err
	}
	for c := range db.chars {
		db.canonical[c] = db.decompose(c, false)
		db.compat[c] = db.decompose(c, true)
	}
	sections, err := db.tables()
	if err != nil {
		return err
	}
	return writeSections(output, sections)
}

func eachLine(path string, handle func(fields []string) error) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line, _, _ := strings.Cut(scanner.Text(), "#")
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Split(line, ";")
		for i, f := range fields {
			fields[i] = strings.TrimSpace(f)
		}
		if err := handle(fields); err != nil {
			return fmt.Errorf("%s: %w", filepath.Base(path), err)
		}
	}
	return scanner.Err()
}

func parseCode(s string) (rune, error) {
	v, err := strconv.ParseUint(s, 16, 32)
	return rune(v), err
}

func parseCodes(s string) ([]rune, error) {
	var out []rune
	for _, field := range strings.Fields(s) {
		c, err := parseCode(field)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

func (db *database) readCharacter(fields []string) error {
	if len(fields) < 6 {
		return fmt.Errorf("short UnicodeData line %v", fields)
	}
	if strings.HasSuffix(fields[1], "First>") || strings.HasSuffix(fields[1], "Last>") {
		return nil
	}
	c, err := parseCode(fields[0])
	if err != nil {
		return err
	}
	class, err := strconv.Atoi(fields[3])
	if err != nil {
		return err
	}
	decomposition := fields[5]
	compatible := strings.HasPrefix(decomposition, "<")
	if compatible {
		_, decomposition, _ = strings.Cut(decomposition, ">")
	}
	mapping, err := parseCodes(decomposition)
	if err != nil {
		return err
	}
	db.chars[c] = character{class: class, compatible: compatible, mapping: mapping}
	return nil
}

func (db *database) readFolding(fields []string) error {
	if fields[1] != "C" && fields[1] != "F" {
		return nil
	}
	c, err := parseCode(fields[0])
	if err != nil {
		return err
	}
	mapping, err := parseCodes(fields[2])
	if err != nil {
		return err
	}
	db.folds[c] = mapping
	return nil
}

func (db *database) readExclusion(fields []string) error {
	if fields[1] != "Full_Composition_Exclusion" {
		return nil
	}
	first, last, ranged := strings.Cut(fields[0], "..")
	if !ranged {
		last = first
	}
	from, err := parseCode(first)
	if err != nil {
		return err
	}
	to, err := parseCode(last)
	if err != nil {
		return err
	}
	for c := from; c <= to; c++ {
		db.excluded[c] = true
	}
	return nil
}

func (db *database) decompose(c rune, compat bool) []rune {
	if c >= hangulBase && c < hangulBase+hangulCount {
		index := c - hangulBase
		out := []rune{hangulLBase + index/hangulNCount, hangulVBase + index%hangulNCount/hangulTCount}
		if t := index % hangulTCount; t != 0 {
			out = append(out, hangulTBase+t)
		}
		return out
	}
	ch := db.chars[c]
	if len(ch.mapping) == 0 || (ch.compatible && !compat) {
		return []rune{c}
	}
	var out []rune
	for _, part := range ch.mapping {
		out = append(out, db.decompose(part, compat)...)
	}
	return out
}

func (db *database) recordOf(c rune, pool *sequencePool) (record, error) {
	r := record{class: db.chars[c].class}
	var err error
	if r.canonical, err = pool.mapping(c, db.canonical[c]); err != nil {
		return r, err
	}
	if r.compat, err = pool.mapping(c, db.compat[c]); err != nil {
		return r, err
	}
	if fold, ok := db.folds[c]; ok {
		r.fold, err = pool.add(fold)
	}
	return r, err
}

type sequencePool struct {
	data    []byte
	offsets map[string]int
}

func (p *sequencePool) mapping(c rune, seq []rune) (string, error) {
	if len(seq) == 0 || (len(seq) == 1 && seq[0] == c) {
		return "", nil
	}
	return p.add(seq)
}

func (p *sequencePool) add(seq []rune) (string, error) {
	key := string(seq)
	if _, ok := p.offsets[key]; ok {
		return key, nil
	}
	offset := len(p.data) / 4
	if offset+len(seq) > 0xFFFF || len(seq) > 0xFF {
		return "", fmt.Errorf("sequence pool overflows 16-bit offsets")
	}
	p.offsets[key] = offset
	for _, c := range seq {
		p.data = appendU32(p.data, c)
	}
	return key, nil
}

func (p *sequencePool) appendReference(out []byte, key string) []byte {
	if key == "" {
		return appendU16(out, 0)
	}
	return appendU16(out, p.offsets[key])
}

func (db *database) tables() ([]section, error) {
	pool := &sequencePool{offsets: map[string]int{}}
	records := []record{{}}
	ids := map[record]int{{}: 0}
	var limit rune
	perCode := map[rune]int{}
	codes := make([]rune, 0, len(db.chars))
	for c := range db.chars {
		codes = append(codes, c)
	}
	slices.Sort(codes)
	for _, c := range codes {
		r, err := db.recordOf(c, pool)
		if err != nil {
			return nil, err
		}
		id, ok := ids[r]
		if !ok {
			id = len(records)
			ids[r] = id
			records = append(records, r)
		}
		if id != 0 {
			perCode[c] = id
			limit = max(limit, c+1)
		}
	}
	if len(records) > 0xFFFF {
		return nil, fmt.Errorf("%d records overflow 16-bit ids", len(records))
	}
	stage1, stage2, err := buildTrie(perCode, limit)
	if err != nil {
		return nil, err
	}
	var recordBytes []byte
	for _, r := range records {
		recordBytes = append(recordBytes, byte(r.class), byte(len([]rune(r.canonical))), byte(len([]rune(r.compat))), byte(len([]rune(r.fold))))
		recordBytes = pool.appendReference(recordBytes, r.canonical)
		recordBytes = pool.appendReference(recordBytes, r.compat)
		recordBytes = pool.appendReference(recordBytes, r.fold)
	}
	return []section{
		{"stage1.bin", stage1},
		{"stage2.bin", stage2},
		{"records.bin", recordBytes},
		{"mappings.bin", pool.data},
		{"compositions.bin", db.compositions()},
	}, nil
}

func buildTrie(perCode map[rune]int, limit rune) ([]byte, []byte, error) {
	blocks := int(limit+blockSize-1) / blockSize
	var stage1, stage2 []byte
	seen := map[string]int{}
	for b := range blocks {
		var block []byte
		for i := range blockSize {
			block = appendU16(block, perCode[rune(b*blockSize+i)])
		}
		index, ok := seen[string(block)]
		if !ok {
			index = len(stage2) / (2 * blockSize)
			seen[string(block)] = index
			stage2 = append(stage2, block...)
		}
		if index > 0xFFFF {
			return nil, nil, fmt.Errorf("trie block index overflows 16 bits")
		}
		stage1 = appendU16(stage1, index)
	}
	return stage1, stage2, nil
}

type composition struct {
	first, second, composite rune
}

func (db *database) compositions() []byte {
	var pairs []composition
	for c, ch := range db.chars {
		if ch.compatible || len(ch.mapping) != 2 || db.excluded[c] {
			continue
		}
		pairs = append(pairs, composition{ch.mapping[0], ch.mapping[1], c})
	}
	slices.SortFunc(pairs, func(a, b composition) int {
		if a.first != b.first {
			return int(a.first - b.first)
		}
		return int(a.second - b.second)
	})
	var out []byte
	for _, p := range pairs {
		out = appendU32(out, p.first)
		out = appendU32(out, p.second)
		out = appendU32(out, p.composite)
	}
	return out
}
