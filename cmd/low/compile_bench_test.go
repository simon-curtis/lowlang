package main

import (
	"testing"

	"lowlang/internal/amd64"
	"lowlang/internal/compiler"
	"lowlang/internal/fixture/fixturetest"
	"lowlang/internal/target"
)

func BenchmarkCompile(b *testing.B) {
	for _, program := range fixturetest.Benchmarks(b) {
		b.Run(program.Name, func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				unit, err := compiler.Analyse(program.Path, target.Linux)
				if err != nil {
					b.Fatal(err)
				}
				amd64.Generate(unit.Program, unit.Platform, nil)
			}
		})
	}
}
