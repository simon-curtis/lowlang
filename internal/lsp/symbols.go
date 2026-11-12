package lsp

import (
	"fmt"
	"path/filepath"
	"strings"

	"lowlang/internal/ast"
	"lowlang/internal/diag"
)

func outline(file *ast.File, t text) []DocumentSymbol {
	symbols := []DocumentSymbol{}
	for _, e := range file.Externs {
		symbols = append(symbols, externSymbol(e, t))
	}
	for _, d := range file.Structs {
		symbols = append(symbols, structSymbol(d, t))
	}
	for _, d := range file.Enums {
		symbols = append(symbols, enumSymbol(d, t))
	}
	for f := range file.Declared() {
		if f.Extern == nil {
			symbols = append(symbols, funcSymbol(f, t))
		}
	}
	return symbols
}

func symbol(t text, pos diag.Pos, name, detail string, kind int) DocumentSymbol {
	span := t.span(pos, len(name))
	return DocumentSymbol{Name: name, Detail: detail, Kind: kind, Range: span, SelectionRange: span}
}

func funcSymbol(f *ast.Func, t text) DocumentSymbol {
	pos := t.nameAfter(f.Pos, f.Name)
	return symbol(t, pos, f.Name, f.Signature(), symbolFunction)
}

func externSymbol(e *ast.Extern, t text) DocumentSymbol {
	name := fmt.Sprintf("extern %q", e.Library)
	s := symbol(t, e.Pos, "extern", "", symbolNamespace)
	s.Name = name
	for _, f := range e.Funcs {
		s.Children = append(s.Children, funcSymbol(f, t))
	}
	return s
}

func structSymbol(d *ast.StructDecl, t text) DocumentSymbol {
	pos := t.nameAfter(d.Pos, d.Name)
	s := symbol(t, pos, d.Name, "struct", symbolStruct)
	for _, f := range d.Fields {
		detail := f.TypeName.String()
		s.Children = append(s.Children, symbol(t, f.Pos, f.Name, detail, symbolField))
	}
	return s
}

func enumSymbol(d *ast.EnumDecl, t text) DocumentSymbol {
	pos := t.nameAfter(d.Pos, d.Name)
	s := symbol(t, pos, d.Name, "enum", symbolEnum)
	for _, v := range d.Variants {
		detail := v.String()
		s.Children = append(s.Children, symbol(t, v.Pos, v.Name, detail, symbolEnumMember))
	}
	return s
}

type symbolSearch struct {
	query string
	seen  map[Location]bool
	found []SymbolInformation
}

func (w *symbolSearch) add(name string, kind int, loc Location, container string) {
	if !strings.Contains(strings.ToLower(name), w.query) || w.seen[loc] {
		return
	}
	w.seen[loc] = true
	w.found = append(w.found, SymbolInformation{Name: name, Kind: kind, Location: loc, ContainerName: container})
}

type symbolFile struct {
	uri  string
	text text
}

func (f symbolFile) at(pos diag.Pos, name string) Location {
	start := f.text.nameAfter(pos, name)
	return Location{URI: f.uri, Range: f.text.word(start)}
}

func (w *symbolSearch) file(f *ast.File, t text) {
	sf := symbolFile{uri: uriOf(f.Path), text: t}
	container := f.Module
	if container == "" {
		container = filepath.Base(f.Path)
	}
	for fn := range f.Declared() {
		w.add(fn.Name, symbolFunction, sf.at(fn.Pos, fn.Name), container)
	}
	for _, d := range f.Structs {
		w.add(d.Name, symbolStruct, sf.at(d.Pos, d.Name), container)
	}
	for _, d := range f.Enums {
		w.add(d.Name, symbolEnum, sf.at(d.Pos, d.Name), container)
		for _, v := range d.Variants {
			w.add(v.Name, symbolEnumMember, sf.at(v.Pos, v.Name), d.Name)
		}
	}
}

func (s *Server) workspaceSymbols(p WorkspaceSymbolParams) []SymbolInformation {
	w := &symbolSearch{query: strings.ToLower(p.Query), seen: map[Location]bool{}, found: []SymbolInformation{}}
	for _, state := range s.entries {
		for _, f := range state.program().files() {
			w.file(f, s.text(f.Path))
		}
	}
	return w.found
}
