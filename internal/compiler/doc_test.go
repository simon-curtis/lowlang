package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDocumentProject(t *testing.T) {
	dir := t.TempDir()
	shapes := filepath.Join(dir, "shapes")
	if err := os.Mkdir(shapes, 0o755); err != nil {
		t.Fatal(err)
	}
	source := `import std/text

/// A box.
pub struct Box {
    /// Across.
    pub width: i64
    secret: i64
}

pub enum Shape {
    Dot
    Square(i64)
}

pub fn area(b: *Box) -> i64 {
    return b.width
}

pub fn unit() -> Box {
    return Box { width: 1, secret: 0 }
}

fn hidden() {}
`
	if err := os.WriteFile(filepath.Join(shapes, "box.low"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	modules, err := Document(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(modules) != 1 || modules[0].Path != "shapes" {
		t.Fatalf("modules = %+v", modules)
	}
	module := modules[0]
	if len(module.Imports) != 1 || module.Imports[0] != (ImportDoc{Path: "std/text", Alias: "text"}) {
		t.Errorf("imports = %v", module.Imports)
	}
	want := []ItemDoc{
		{Kind: "struct", Name: "Box", Text: "pub struct Box {\n    /// Across.\n    pub width: i64\n    ...\n}", Doc: "A box.", File: "box.low", Line: 4},
		{Kind: "enum", Name: "Shape", Text: "pub enum Shape {\n    Dot\n    Square(i64)\n}", File: "box.low", Line: 10},
		{Kind: "fn", Name: "area", Receiver: "Box", Text: "pub fn area(b: *Box) -> i64", File: "box.low", Line: 15},
		{Kind: "fn", Name: "unit", Text: "pub fn unit() -> Box", File: "box.low", Line: 19},
	}
	if len(module.Items) != len(want) {
		t.Fatalf("items = %+v", module.Items)
	}
	for i, item := range module.Items {
		if item != want[i] {
			t.Errorf("item %d = %+v, want %+v", i, item, want[i])
		}
	}
}

func TestDocumentStd(t *testing.T) {
	modules, err := Document("std")
	if err != nil {
		t.Fatal(err)
	}
	for _, module := range modules {
		if module.Path == "std/io" && len(module.Items) > 0 {
			return
		}
	}
	t.Fatal("std/io has no documented items")
}
