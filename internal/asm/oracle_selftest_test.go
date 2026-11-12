package asm_test

import (
	"path/filepath"
	"testing"

	"lowlang/internal/oracletool"
	"lowlang/internal/target"
)

func TestOracleReadsObjects(t *testing.T) {
	oracletool.Require(t, "as", oracletool.GNU)
	path := filepath.Join("..", "..", "bench", "fib", "main.low")
	for _, platform := range []target.OS{target.Linux, target.Windows} {
		entry := corpusEntry{name: "bench/fib/main.low", path: path, platform: platform}
		dir := t.TempDir()
		source, err := entry.generate(dir)
		if err != nil {
			t.Fatalf("%s: %v", entry.label(), err)
		}
		oracle, err := runAs(dir, asSource(source, platform), oracletool.GNUOptions{})
		if err != nil {
			t.Fatalf("%s: %v", entry.label(), err)
		}
		checkOracleObject(t, entry, oracle)
	}
}

func checkOracleObject(t *testing.T, entry corpusEntry, oracle *oracleObject) {
	t.Helper()
	for _, name := range []string{".text", ".rodata", ".data", ".bss"} {
		section := oracle.sections[name]
		if section == nil || section.size == 0 {
			t.Errorf("%s: section %s is missing or empty", entry.label(), name)
		}
	}
	if len(oracle.symbols) == 0 {
		t.Errorf("%s: no symbols read", entry.label())
	}
	relocated := ".rodata"
	if entry.platform == target.Windows {
		relocated = ".idata$4"
	}
	section := oracle.sections[relocated]
	if section == nil || len(section.relocs) == 0 {
		t.Fatalf("%s: section %s has no relocations", entry.label(), relocated)
	}
	if kind := section.relocs[0].kind; kind != "Abs64" && kind != "Abs32" {
		t.Errorf("%s: unexpected relocation kind %s", entry.label(), kind)
	}
	if bss := oracle.sections[".bss"]; bss != nil && !bss.noBits {
		t.Errorf("%s: .bss should be nobits", entry.label())
	}
}
