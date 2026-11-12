package lsp

import (
	"path/filepath"

	"lowlang/internal/ast"
	"lowlang/internal/compiler"
	"lowlang/internal/diag"
	"lowlang/internal/types"
)

type program struct {
	unit *compiler.Unit
}

func (p program) files() []*ast.File {
	var files []*ast.File
	if p.unit == nil {
		return files
	}
	for _, f := range p.unit.Files {
		if f.Path != "" {
			files = append(files, f)
		}
	}
	return files
}

func (p program) moduleFiles(module string) []*ast.File {
	var files []*ast.File
	for _, f := range p.files() {
		if f.Module == module {
			files = append(files, f)
		}
	}
	return files
}

func (p program) fileAt(path string) *ast.File {
	for _, f := range p.files() {
		if filepath.Clean(f.Path) == path {
			return f
		}
	}
	return nil
}

type typeDecl struct {
	name string
	doc  string
	file *ast.File
	pos  diag.Pos
	text string
	typ  *types.Type
	enum *ast.EnumDecl
}

func structTypeDecl(d *ast.StructDecl) *typeDecl {
	return &typeDecl{name: d.Name, doc: d.Doc, file: d.File, pos: d.Pos, text: d.String(), typ: d.Type}
}

func enumTypeDecl(d *ast.EnumDecl) *typeDecl {
	return &typeDecl{name: d.Name, doc: d.Doc, file: d.File, pos: d.Pos, text: d.String(), typ: d.Type, enum: d}
}

func (d *typeDecl) target() target {
	return target{path: d.file.Path, pos: d.pos, name: d.name}
}

func (p program) typeDecl(module, name string) *typeDecl {
	for _, f := range p.moduleFiles(module) {
		for _, d := range f.Structs {
			if d.Name == name {
				return structTypeDecl(d)
			}
		}
		for _, d := range f.Enums {
			if d.Name == name {
				return enumTypeDecl(d)
			}
		}
	}
	return nil
}

func (p program) enumDecl(module, name string) *ast.EnumDecl {
	decl := p.typeDecl(module, name)
	if decl == nil {
		return nil
	}
	return decl.enum
}

func (p program) function(module, name string, public bool) *ast.Func {
	for _, f := range p.moduleFiles(module) {
		for fn := range f.Declared() {
			if fn.Name == name && (fn.Pub || !public) {
				return fn
			}
		}
	}
	return nil
}

func (p program) declType(file *ast.File, tn *ast.TypeName) *types.Type {
	module := file.Module
	if tn.Module != "" {
		path, ok := file.Imported(tn.Module)
		if !ok {
			return nil
		}
		module = path
	}
	decl := p.typeDecl(module, tn.Name)
	if decl == nil {
		return nil
	}
	return decl.typ
}
