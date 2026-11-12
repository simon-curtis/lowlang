package asm_test

import (
	"fmt"
	"path/filepath"
	"slices"
	"testing"

	"lowlang/internal/fixture/fixturetest"
	"lowlang/internal/oracletool"
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

func corpusEntries(t testing.TB) []corpusEntry {
	t.Helper()
	programs := slices.Concat(fixturetest.Programs(t, "run"), fixturetest.Programs(t, "windows"), fixturetest.Benchmarks(t))
	var entries []corpusEntry
	for _, program := range programs {
		rel, err := filepath.Rel(fixturetest.Root(), program.Path)
		if err != nil {
			t.Fatalf("relative path of %s: %v", program.Path, err)
		}
		for _, platform := range []target.OS{target.Linux, target.Windows} {
			entries = append(entries, corpusEntry{name: rel, path: program.Path, platform: platform})
		}
	}
	return entries
}

func (e corpusEntry) generate(dir string) (string, error) {
	generated, err := oracletool.Generate(dir, e.path, e.platform)
	return generated.Asm, err
}
