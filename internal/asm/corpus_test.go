package asm_test

import (
	"fmt"
	"path/filepath"
	"sort"

	"lowlang/internal/amd64"
	"lowlang/internal/compiler"
	"lowlang/internal/target"
)

type corpusEntry struct {
	name     string
	path     string
	platform target.OS
}

func (e corpusEntry) label() string {
	return fmt.Sprintf("%s [%s]", e.name, e.platform)
}

func corpusEntries() ([]corpusEntry, error) {
	root := filepath.Join("..", "..")
	var paths []string
	for _, pattern := range []string{"testdata/run/*.low", "testdata/windows/*.low", "bench/*/main.low"} {
		full := filepath.Join(root, pattern)
		matches, err := filepath.Glob(full)
		if err != nil {
			return nil, fmt.Errorf("glob %s: %w", pattern, err)
		}
		paths = append(paths, matches...)
	}
	sort.Strings(paths)
	var entries []corpusEntry
	for _, path := range paths {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil, fmt.Errorf("relative path of %s: %w", path, err)
		}
		for _, platform := range []target.OS{target.Linux, target.Windows} {
			entries = append(entries, corpusEntry{name: rel, path: path, platform: platform})
		}
	}
	return entries, nil
}

func (e corpusEntry) generate() (source string, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("compiler panic: %v", r)
		}
	}()
	unit, err := compiler.Analyse(e.path, e.platform)
	if err != nil {
		return "", err
	}
	return amd64.Generate(unit.Program, nil).Asm, nil
}
