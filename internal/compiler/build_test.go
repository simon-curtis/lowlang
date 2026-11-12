package compiler

import (
	"context"
	"debug/elf"
	"os"
	"path/filepath"
	"testing"

	"lowlang/internal/amd64"
	"lowlang/internal/target"
)

func hotUnit(t *testing.T, platform target.OS, source string) *Unit {
	t.Helper()
	path := filepath.Join(t.TempDir(), "main.low")
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	unit, err := AnalyseDev(path, platform)
	if err != nil {
		t.Fatal(err)
	}
	return unit
}

const hotSource = "fn answer() -> i64 {\n    1\n}\n\nfn main() {\n    print(answer())\n}\n"

func TestHotBuildExposesThePatchProtocol(t *testing.T) {
	unit := hotUnit(t, target.Linux, hotSource)
	table := amd64.NewFuncTable(unit.Program)
	output := filepath.Join(t.TempDir(), "app")
	built, err := Build(context.Background(), unit, Hot(table), output)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{amd64.PatchListSymbol, amd64.PatchCountSymbol, "fn_table"} {
		if _, ok := built.Symbols[name]; !ok {
			t.Errorf("image has no %s", name)
		}
	}
	if built.Funcs["answer"] == "" {
		t.Errorf("no code recorded for answer in %v", built.Funcs)
	}
	file, err := elf.Open(output)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	symbols, err := file.Symbols()
	if err != nil {
		t.Fatal(err)
	}
	for _, sym := range symbols {
		if want, ok := built.Symbols[sym.Name]; ok && want != sym.Value {
			t.Errorf("%s at %#x in the file, %#x in the result", sym.Name, sym.Value, want)
		}
	}
}

func TestPatchLinksAtTheHotRegion(t *testing.T) {
	unit := hotUnit(t, target.Linux, hotSource)
	table := amd64.NewFuncTable(unit.Program)
	built, err := Build(context.Background(), unit, Hot(table), filepath.Join(t.TempDir(), "app"))
	if err != nil {
		t.Fatal(err)
	}
	const suffix = "_r1_0"
	patch, err := built.Patch(unit, table, []string{"answer"}, suffix, amd64.HotBase)
	if err != nil {
		t.Fatal(err)
	}
	if patch.Address < amd64.HotBase || patch.Address+uint64(len(patch.Code)) > amd64.HotBase+amd64.HotSize {
		t.Errorf("patch at %#x for %d bytes lies outside the hot region", patch.Address, len(patch.Code))
	}
	address, ok := patch.Symbols[amd64.Symbol("answer", suffix)]
	if !ok || address < patch.Address || address >= patch.Address+uint64(len(patch.Code)) {
		t.Errorf("patched symbol at %#x, ok %t, outside %#x+%d", address, ok, patch.Address, len(patch.Code))
	}
	entries := table.PatchEntries([]string{"answer"}, suffix, patch.Symbols)
	if len(entries) != 16 {
		t.Errorf("patch entry is %d bytes", len(entries))
	}
}

func TestHotBuildNeedsLinux(t *testing.T) {
	unit := hotUnit(t, target.Windows, hotSource)
	if _, err := Generate(unit, Hot(amd64.NewFuncTable(unit.Program))); err == nil {
		t.Fatal("generated hot code for windows")
	}
}

func TestBuildChoosesTheLinkerFromTheTarget(t *testing.T) {
	for _, platform := range []target.OS{target.Linux, target.Windows} {
		t.Run(platform.String(), func(t *testing.T) {
			unit := hotUnit(t, platform, hotSource)
			output := filepath.Join(t.TempDir(), platform.Executable("app"))
			if _, err := Build(context.Background(), unit, Release, output); err != nil {
				t.Fatal(err)
			}
			image, err := os.ReadFile(output)
			if err != nil {
				t.Fatal(err)
			}
			magic := map[target.OS]string{target.Linux: "\x7fELF", target.Windows: "MZ"}[platform]
			if string(image[:len(magic)]) != magic {
				t.Errorf("image starts with %q, want %q", image[:len(magic)], magic)
			}
		})
	}
}
