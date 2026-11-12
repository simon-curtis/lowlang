package compiler

import (
	"os"
	"path/filepath"
	"testing"

	"lowlang/internal/target"
)

func TestOverlayFilesJoinTheirDirectory(t *testing.T) {
	dir := t.TempDir()
	disk := filepath.Join(dir, "main.low")
	if err := os.WriteFile(disk, []byte("fn main() {\n    helper()\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	unsaved := filepath.Join(dir, "helper.low")
	overlay := Overlay{unsaved: []byte("fn helper() {\n}\n")}
	if _, err := Check(dir, target.Linux, overlay); err != nil {
		t.Fatalf("directory with an unsaved file: %v", err)
	}
	if _, err := Check(dir, target.Linux, Overlay(nil)); err == nil {
		t.Fatal("helper resolved without the unsaved file")
	}
}

func TestOverlayShadowsDisk(t *testing.T) {
	dir := t.TempDir()
	disk := filepath.Join(dir, "main.low")
	if err := os.WriteFile(disk, []byte("fn main() {\n    missing()\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Check(disk, target.Linux, Overlay(nil)); err == nil {
		t.Fatal("disk copy compiled")
	}
	overlay := Overlay{disk: []byte("fn main() {\n}\n")}
	if _, err := Check(disk, target.Linux, overlay); err != nil {
		t.Fatalf("overlay copy: %v", err)
	}
}

func TestOverlayEmbedsUnsavedFile(t *testing.T) {
	dir := t.TempDir()
	disk := filepath.Join(dir, "main.low")
	source := "fn main() {\n    print(embed(\"note.txt\"))\n}\n"
	if err := os.WriteFile(disk, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	note := filepath.Join(dir, "note.txt")
	unit, err := Check(disk, target.Linux, Overlay{note: []byte("held in memory")})
	if err != nil {
		t.Fatal(err)
	}
	if len(unit.Sources) != 2 || unit.Sources[1] != note {
		t.Fatalf("sources %v, want the program and %s", unit.Sources, note)
	}
	if _, err := Check(disk, target.Linux, Overlay(nil)); err == nil {
		t.Fatal("embed resolved without the unsaved file")
	}
}

func TestRootFindsModuleFile(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, found := Root(Overlay{}, nested); found {
		t.Fatal("found a root without low.mod")
	}
	overlay := Overlay{filepath.Join(dir, moduleFile): nil}
	root, found := Root(overlay, nested)
	if !found || root != dir {
		t.Fatalf("root %q, found %t; want %q", root, found, dir)
	}
}

func TestParseFileReadsThroughTheOverlay(t *testing.T) {
	name := filepath.Join(t.TempDir(), "held.low")
	file, err := ParseFile(Overlay{name: []byte("fn held() {\n}\n")}, name)
	if err != nil {
		t.Fatal(err)
	}
	if file.Path != name || len(file.Funcs) != 1 {
		t.Fatalf("parsed %q with %d functions", file.Path, len(file.Funcs))
	}
}
