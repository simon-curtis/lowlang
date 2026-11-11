package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"lowlang/internal/amd64"
	"lowlang/internal/compiler"
	"lowlang/internal/target"
)

type determinismCase struct {
	name     string
	path     string
	platform target.OS
	dev      bool
}

type compiled struct {
	ir  string
	asm string
}

func determinismCases(t *testing.T) []determinismCase {
	t.Helper()
	var cases []determinismCase
	add := func(path string, platform target.OS, dev bool) {
		mode := "release"
		if dev {
			mode = "dev"
		}
		name := fmt.Sprintf("%s/%s/%s", filepath.Base(filepath.Dir(path)), filepath.Base(path), mode)
		if platform == target.Windows {
			name = fmt.Sprintf("%s/windows", name)
		}
		cases = append(cases, determinismCase{name: name, path: path, platform: platform, dev: dev})
	}
	benchmarks, err := filepath.Glob(filepath.Join("..", "..", "bench", "*", "main.low"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range benchmarks {
		add(path, target.Linux, false)
		add(path, target.Linux, true)
		add(path, target.Windows, false)
	}
	for _, path := range fixtures(t, "run") {
		name := filepath.Base(path)
		if strings.HasPrefix(name, "std_") && name != "std_otel_export.low" {
			continue
		}
		add(path, target.Linux, false)
	}
	add(filepath.Join("..", "..", "testdata", "run", "std_otel_export.low"), target.Linux, true)
	add(filepath.Join("..", "..", "testdata", "run", "std_otel_export.low"), target.Windows, false)
	for _, path := range fixtures(t, "windows") {
		add(path, target.Windows, false)
	}
	return cases
}

func compileOnce(c determinismCase) (compiled, error) {
	analyse := compiler.Analyse
	if c.dev {
		analyse = compiler.AnalyseDev
	}
	unit, err := analyse(c.path, c.platform)
	if err != nil {
		return compiled{}, err
	}
	var table map[string]int
	var ir strings.Builder
	if c.dev {
		table = map[string]int{}
	}
	for i, f := range unit.Program.Funcs {
		fmt.Fprint(&ir, f)
		if c.dev {
			table[f.Name] = i
		}
	}
	return compiled{ir: ir.String(), asm: amd64.Generate(unit.Program, table).Asm}, nil
}

func firstDifference(a, b string) string {
	left, right := strings.Split(a, "\n"), strings.Split(b, "\n")
	for i := 0; i < min(len(left), len(right)); i++ {
		if left[i] != right[i] {
			return fmt.Sprintf("line %d: %q != %q", i+1, left[i], right[i])
		}
	}
	return fmt.Sprintf("lengths differ: %d != %d lines", len(left), len(right))
}

func TestCompileIsDeterministic(t *testing.T) {
	for _, c := range determinismCases(t) {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			first, err := compileOnce(c)
			if err != nil {
				t.Fatal(err)
			}
			second, err := compileOnce(c)
			if err != nil {
				t.Fatal(err)
			}
			if first.ir != second.ir {
				t.Errorf("ir differs between compiles: %s", firstDifference(first.ir, second.ir))
			}
			if first.asm != second.asm {
				t.Errorf("assembly differs between compiles: %s", firstDifference(first.asm, second.asm))
			}
		})
	}
}
