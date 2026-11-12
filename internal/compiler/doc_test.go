package compiler

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lowlang/internal/ast"
	"lowlang/internal/diag"
	"lowlang/internal/target"
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
	modules, err := Document(dir, target.Linux)
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
	modules, err := Document("std", target.Linux)
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

func TestStdTargetsShareAPI(t *testing.T) {
	linux := documentedAPI(t, target.Linux)
	windows := documentedAPI(t, target.Windows)
	for key, item := range linux {
		other, ok := windows[key]
		switch {
		case !ok:
			t.Errorf("%s is missing on windows", key)
		case other != item:
			t.Errorf("%s differs between targets:\nlinux:   %q\nwindows: %q", key, item.Text+" "+item.Doc, other.Text+" "+other.Doc)
		}
	}
	for key := range windows {
		if _, ok := linux[key]; !ok {
			t.Errorf("%s is missing on linux", key)
		}
	}
}

func documentedAPI(t *testing.T, platform target.OS) map[string]ItemDoc {
	t.Helper()
	modules, err := Document("std", platform)
	if err != nil {
		t.Fatal(err)
	}
	api := map[string]ItemDoc{}
	for _, module := range modules {
		for _, item := range module.Items {
			item.File, item.Line = "", 0
			api[fmt.Sprintf("%s %s %s.%s", module.Path, item.Kind, item.Receiver, item.Name)] = item
		}
	}
	return api
}

type undocumented struct {
	missing   []string
	malformed []string
}

func (u *undocumented) check(doc string, pos diag.Pos, name string) {
	if doc == "" {
		u.missing = append(u.missing, fmt.Sprintf("%s: %s", pos, name))
		return
	}
	for line := range strings.SplitSeq(doc, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "///") {
			u.malformed = append(u.malformed, fmt.Sprintf("%s: %s", pos, name))
			return
		}
	}
}

func (u *undocumented) file(f *ast.File) {
	for _, s := range f.Structs {
		if !s.Pub {
			continue
		}
		u.check(s.Doc, s.Pos, s.Name)
		for _, field := range s.Fields {
			if field.Pub {
				u.check(field.Doc, field.Pos, fmt.Sprintf("%s.%s", s.Name, field.Name))
			}
		}
	}
	for _, e := range f.Enums {
		if !e.Pub {
			continue
		}
		u.check(e.Doc, e.Pos, e.Name)
		for _, v := range e.Variants {
			u.check(v.Doc, v.Pos, fmt.Sprintf("%s.%s", e.Name, v.Name))
		}
	}
	for _, fn := range f.Funcs {
		if fn.Pub {
			u.check(fn.Doc, fn.Pos, fn.Name)
		}
	}
	for _, e := range f.Externs {
		for _, fn := range e.Funcs {
			if fn.Pub {
				u.check(fn.Doc, fn.Pos, fn.Name)
			}
		}
	}
}

func TestStdDocumented(t *testing.T) {
	u := &undocumented{}
	err := fs.WalkDir(stdFS, "std", func(name string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(name, ".low") {
			return err
		}
		f, err := stdTree.parse(name)
		if err != nil {
			return err
		}
		u.file(f)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(u.missing) > 0 {
		t.Errorf("%d public std items have no /// doc:\n%s", len(u.missing), strings.Join(u.missing, "\n"))
	}
	if len(u.malformed) > 0 {
		t.Errorf("%d public std docs have a line starting with a second ///:\n%s", len(u.malformed), strings.Join(u.malformed, "\n"))
	}
}
