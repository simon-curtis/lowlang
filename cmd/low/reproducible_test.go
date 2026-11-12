package main

import (
	"fmt"
	"strings"
	"testing"

	"lowlang/internal/compiler"
	"lowlang/internal/fixture/fixturetest"
	"lowlang/internal/target"
)

type determinismCase struct {
	name    string
	path    string
	profile compiler.Profile
}

type compiled struct {
	ir  string
	asm string
}

func determinismCases(t *testing.T) []determinismCase {
	t.Helper()
	var cases []determinismCase
	add := func(group string, program fixturetest.Program, platform target.OS, dev bool) {
		mode, profile := "release", compiler.Release(platform)
		if dev {
			mode, profile = "dev", compiler.Dev
		}
		name := fmt.Sprintf("%s/%s/%s/%s", group, program.Name, platform, mode)
		cases = append(cases, determinismCase{name: name, path: program.Path, profile: profile})
	}
	for _, program := range fixturetest.Programs(t, "run") {
		add("run", program, target.Linux, false)
	}
	for _, program := range fixturetest.Benchmarks(t) {
		add("bench", program, target.Linux, false)
		add("bench", program, target.Linux, true)
		add("bench", program, target.Windows, false)
	}
	for _, program := range fixturetest.Programs(t, "windows") {
		add("windows", program, target.Windows, false)
	}
	return cases
}

func compileOnce(c determinismCase) (compiled, error) {
	unit, err := compiler.Analyse(c.path, c.profile)
	if err != nil {
		return compiled{}, err
	}
	out, err := compiler.Generate(unit)
	if err != nil {
		return compiled{}, err
	}
	var ir strings.Builder
	for _, f := range unit.Program.Funcs {
		fmt.Fprint(&ir, f)
	}
	return compiled{ir: ir.String(), asm: out.Asm}, nil
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
