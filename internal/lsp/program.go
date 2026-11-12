package lsp

import (
	"path/filepath"

	"lowlang/internal/ast"
	"lowlang/internal/compiler"
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

func (p program) typeDecl(module, name string) any {
	for _, f := range p.moduleFiles(module) {
		for _, d := range f.Structs {
			if d.Name == name {
				return d
			}
		}
		for _, d := range f.Enums {
			if d.Name == name {
				return d
			}
		}
	}
	return nil
}

func (p program) enumDecl(module, name string) *ast.EnumDecl {
	decl, _ := p.typeDecl(module, name).(*ast.EnumDecl)
	return decl
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
	switch d := p.typeDecl(module, tn.Name).(type) {
	case *ast.StructDecl:
		return d.Type
	case *ast.EnumDecl:
		return d.Type
	}
	return nil
}
