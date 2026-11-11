package main

import (
	"path/filepath"
	"testing"

	"lowlang/internal/amd64"
	"lowlang/internal/compiler"
	"lowlang/internal/target"
)

func benchmarkPrograms(tb testing.TB) []string {
	tb.Helper()
	paths, err := filepath.Glob(filepath.Join("..", "..", "bench", "*", "main.low"))
	if err != nil {
		tb.Fatal(err)
	}
	if len(paths) == 0 {
		tb.Fatal("no benchmark programs")
	}
	return paths
}

func BenchmarkCompile(b *testing.B) {
	for _, path := range benchmarkPrograms(b) {
		b.Run(filepath.Base(filepath.Dir(path)), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				unit, err := compiler.Analyse(path, target.Linux)
				if err != nil {
					b.Fatal(err)
				}
				amd64.Generate(unit.Program, nil)
			}
		})
	}
}
