package main

import (
	"strings"
	"testing"

	"lowlang/internal/compiler"
	"lowlang/internal/fixture"
	"lowlang/internal/fixture/fixturetest"
	"lowlang/internal/target"
)

func TestLint(t *testing.T) {
	for _, program := range fixturetest.Programs(t, "lint") {
		t.Run(program.Name, func(t *testing.T) {
			lintFixture(t, program)
		})
	}
}

func lintFixture(t *testing.T, program fixturetest.Program) {
	want, _ := program.Text(t, fixture.WarningsExt)
	unit, err := compiler.Check(program.Path, target.Linux, compiler.Overlay(nil))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, w := range unit.Warnings {
		got = append(got, fixturetest.Relative(t, w.String(), program.Path))
	}
	if text := strings.Join(got, "\n"); text != strings.TrimSpace(want) {
		t.Errorf("warnings:\n%s\nwant:\n%s", text, strings.TrimSpace(want))
	}
}
