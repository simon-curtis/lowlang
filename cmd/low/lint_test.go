package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lowlang/internal/compiler"
)

func TestLint(t *testing.T) {
	for _, path := range fixtures(t, "lint") {
		t.Run(filepath.Base(path), func(t *testing.T) {
			lintFixture(t, path)
		})
	}
}

func lintFixture(t *testing.T, path string) {
	want, _ := expected(t, path, ".warn")
	unit, err := compiler.Check(path, os.ReadFile)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, w := range unit.Warnings {
		got = append(got, relative(t, w.String(), path))
	}
	if text := strings.Join(got, "\n"); text != strings.TrimSpace(want) {
		t.Errorf("warnings:\n%s\nwant:\n%s", text, strings.TrimSpace(want))
	}
}
