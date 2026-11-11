package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

const (
	singleByteEntries = 128
	singleByteBase    = 0x80
)

func generateCharset(dir, output string) error {
	paths, err := filepath.Glob(filepath.Join(dir, "index-*.txt"))
	if err != nil {
		return err
	}
	slices.Sort(paths)
	var tables []byte
	var names strings.Builder
	for _, path := range paths {
		name := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(path), "index-"), ".txt")
		block, err := singleByteBlock(path)
		if err != nil {
			return err
		}
		tables = append(tables, block...)
		fmt.Fprintf(&names, "%s\n", name)
	}
	return writeSections(output, []section{
		{"tables.bin", tables},
		{"tables.txt", []byte(names.String())},
	})
}

func readIndex(path string) (map[int]rune, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	entries := map[int]rune{}
	for _, line := range strings.Split(string(data), "\n") {
		line, _, _ = strings.Cut(line, "#")
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if len(fields) < 2 {
			return nil, fmt.Errorf("%s: short line %q", filepath.Base(path), line)
		}
		pointer, err := strconv.Atoi(fields[0])
		if err != nil {
			return nil, err
		}
		code, err := strconv.ParseUint(strings.TrimPrefix(fields[1], "0x"), 16, 32)
		if err != nil {
			return nil, err
		}
		entries[pointer] = rune(code)
	}
	return entries, nil
}

func singleByteBlock(path string) ([]byte, error) {
	entries, err := readIndex(path)
	if err != nil {
		return nil, err
	}
	if len(entries) != singleByteEntries {
		return nil, fmt.Errorf("%s: %d entries, want %d", filepath.Base(path), len(entries), singleByteEntries)
	}
	type pair struct {
		code rune
		b    int
	}
	reverse := make([]pair, 0, singleByteEntries)
	var block []byte
	for p := range singleByteEntries {
		code, ok := entries[p]
		if !ok || code > 0xFFFF {
			return nil, fmt.Errorf("%s: pointer %d is missing or outside the BMP", filepath.Base(path), p)
		}
		block = appendU16(block, int(code))
		reverse = append(reverse, pair{code, p + singleByteBase})
	}
	slices.SortFunc(reverse, func(a, b pair) int { return int(a.code) - int(b.code) })
	for _, r := range reverse {
		block = appendU32(block, r.code<<8|rune(r.b))
	}
	return block, nil
}
