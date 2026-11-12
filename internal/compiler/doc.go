package compiler

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"slices"
	"strings"

	"lowlang/internal/ast"
)

type ModuleDoc struct {
	Path    string      `json:"path"`
	Imports []ImportDoc `json:"imports"`
	Items   []ItemDoc   `json:"items"`
}

type ImportDoc struct {
	Path  string `json:"path"`
	Alias string `json:"alias"`
}

type ItemDoc struct {
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	Receiver string `json:"receiver,omitempty"`
	Text     string `json:"text"`
	Doc      string `json:"doc,omitempty"`
	File     string `json:"file"`
	Line     int    `json:"line"`
}

type documenter struct {
	fsys  fs.FS
	parse func(name string) (*ast.File, error)
}

func Document(target string) ([]ModuleDoc, error) {
	if target == "std" {
		d := documenter{fsys: stdFS, parse: stdTree.parse}
		return d.walk("std")
	}
	disk := diskTree(Overlay(nil))
	d := documenter{fsys: os.DirFS(target), parse: func(name string) (*ast.File, error) {
		return disk.parse(path.Join(target, name))
	}}
	return d.walk(".")
}

func (d documenter) walk(base string) ([]ModuleDoc, error) {
	var modules []ModuleDoc
	err := fs.WalkDir(d.fsys, base, func(dir string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}
		module, found, err := d.module(dir)
		if err != nil {
			return err
		}
		if found {
			modules = append(modules, module)
		}
		return nil
	})
	return modules, err
}

func (d documenter) module(dir string) (ModuleDoc, bool, error) {
	entries, err := fs.ReadDir(d.fsys, dir)
	if err != nil {
		return ModuleDoc{}, false, err
	}
	var files []*ast.File
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".low") {
			continue
		}
		file, err := d.parse(path.Join(dir, entry.Name()))
		if err != nil {
			return ModuleDoc{}, false, err
		}
		files = append(files, file)
	}
	if len(files) == 0 {
		return ModuleDoc{}, false, nil
	}
	module := ModuleDoc{Path: dir, Imports: []ImportDoc{}, Items: []ItemDoc{}}
	if dir == "." {
		module.Path = ""
	}
	types := publicTypes(files)
	for _, file := range files {
		module.add(file, types)
	}
	slices.SortFunc(module.Imports, func(a, b ImportDoc) int { return strings.Compare(a.Path, b.Path) })
	module.Imports = slices.Compact(module.Imports)
	slices.SortStableFunc(module.Items, func(a, b ItemDoc) int { return strings.Compare(a.Name, b.Name) })
	return module, true, nil
}

func publicTypes(files []*ast.File) map[string]bool {
	types := map[string]bool{}
	for _, file := range files {
		for _, s := range file.Structs {
			types[s.Name] = s.Pub
		}
		for _, e := range file.Enums {
			types[e.Name] = e.Pub
		}
	}
	return types
}

func (m *ModuleDoc) add(file *ast.File, types map[string]bool) {
	name := path.Base(file.Path)
	for _, imp := range file.Imports {
		m.Imports = append(m.Imports, ImportDoc{Path: imp.Path, Alias: imp.Alias})
	}
	for _, s := range file.Structs {
		if s.Pub {
			m.Items = append(m.Items, ItemDoc{Kind: "struct", Name: s.Name, Text: structText(s), Doc: s.Doc, File: name, Line: s.Pos.Line})
		}
	}
	for _, e := range file.Enums {
		if e.Pub {
			m.Items = append(m.Items, ItemDoc{Kind: "enum", Name: e.Name, Text: enumText(e), Doc: e.Doc, File: name, Line: e.Pos.Line})
		}
	}
	for _, f := range file.Funcs {
		if f.Pub {
			m.Items = append(m.Items, ItemDoc{Kind: "fn", Name: f.Name, Receiver: receiver(f, types), Text: fmt.Sprintf("pub %s", f), Doc: f.Doc, File: name, Line: f.Pos.Line})
		}
	}
	for _, e := range file.Externs {
		for _, f := range e.Funcs {
			if f.Pub {
				m.Items = append(m.Items, ItemDoc{Kind: "fn", Name: f.Name, Text: fmt.Sprintf("pub %s", f), Doc: f.Doc, File: name, Line: f.Pos.Line})
			}
		}
	}
}

func receiver(f *ast.Func, types map[string]bool) string {
	if len(f.Params) == 0 {
		return ""
	}
	t := f.Params[0].TypeName
	if t.Pointer {
		t = t.Elem
	}
	if t.Module != "" || !types[t.Name] {
		return ""
	}
	return t.Name
}

func structText(s *ast.StructDecl) string {
	var b strings.Builder
	if s.C {
		b.WriteString("extern ")
	}
	b.WriteString(fmt.Sprintf("pub struct %s%s {\n", s.Name, ast.TypeParams(s.TypeParams)))
	hidden := false
	for _, f := range s.Fields {
		if !f.Pub {
			hidden = true
			continue
		}
		member(&b, f.Doc)
		b.WriteString(fmt.Sprintf("    pub %s: %s\n", f.Name, f.TypeName))
	}
	if hidden {
		b.WriteString("    ...\n")
	}
	b.WriteString("}")
	return b.String()
}

func enumText(e *ast.EnumDecl) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("pub enum %s%s {\n", e.Name, ast.TypeParams(e.TypeParams)))
	for _, v := range e.Variants {
		member(&b, v.Doc)
		b.WriteString(fmt.Sprintf("    %s\n", v))
	}
	b.WriteString("}")
	return b.String()
}

func member(b *strings.Builder, doc string) {
	if doc == "" {
		return
	}
	for line := range strings.Lines(doc) {
		b.WriteString(strings.TrimRight(fmt.Sprintf("    /// %s", line), " \n"))
		b.WriteString("\n")
	}
}
