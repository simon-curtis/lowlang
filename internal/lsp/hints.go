package lsp

import (
	"fmt"
	"path/filepath"
	"strings"

	"lowlang/internal/ast"
	"lowlang/internal/diag"
)

func (s *Server) inlayHints(p InlayHintParams) []InlayHint {
	hints := []InlayHint{}
	path := pathOf(p.TextDocument.URI)
	file := s.checked(path)
	if file == nil || !s.fresh[s.entry(path)] {
		return hints
	}
	x := newIndex(file, s.text(path))
	for _, o := range x.occurrences {
		if !inferred(o) || o.local.Type == nil {
			continue
		}
		line := o.pos.Line - 1
		if line < p.Range.Start.Line || line > p.Range.End.Line {
			continue
		}
		end := o.pos
		end.Col += len(o.name)
		label := fmt.Sprintf(": %s", o.local.Type)
		hints = append(hints, InlayHint{Position: x.text.position(end), Label: label, Kind: hintType})
	}
	return hints
}

func inferred(o occurrence) bool {
	if o.local == nil {
		return false
	}
	switch n := o.node.(type) {
	case *ast.Let:
		return n.TypeName == nil
	case *ast.ForIn:
		return true
	}
	return false
}

func (s *Server) refreshHints() error {
	if !s.hintRefresh {
		return nil
	}
	s.requests++
	return s.stream.write(serverRequest{JSONRPC: "2.0", ID: s.requests, Method: "workspace/inlayHint/refresh"})
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
	for _, fn := range f.Funcs {
		if fn.Template == nil && fn.Closure == nil {
			w.add(fn.Name, symbolFunction, sf.at(fn.Pos, fn.Name), container)
		}
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
	for _, unit := range s.units {
		for _, f := range unit.Files {
			if f.Path != "" {
				w.file(f, s.text(f.Path))
			}
		}
	}
	return w.found
}
