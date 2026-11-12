package lsp

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"lowlang/internal/ast"
	"lowlang/internal/compiler"
	"lowlang/internal/lexer"
)

type side int

const (
	plain side = iota
	fieldSide
	valueSide
)

type reference struct {
	location Location
	name     string
	side     side
}

var identifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type finder struct {
	server *Server
	unit   *compiler.Unit
	want   Location
	seen   map[Location]bool
	found  []reference
}

func (f *finder) resolves(q *query) bool {
	tg, ok := q.definition()
	return ok && f.server.locate(tg) == f.want
}

func (f *finder) add(x *index, o *occurrence, s side) {
	loc := Location{URI: uriOf(x.file.Path), Range: x.text.word(o.pos)}
	if f.seen[loc] {
		return
	}
	f.seen[loc] = true
	f.found = append(f.found, reference{location: loc, name: o.name, side: s})
}

func (f *finder) visit(x *index, o *occurrence) {
	q := &query{unit: f.unit, index: x, occ: o}
	init, ok := o.node.(*ast.FieldInit)
	if !ok || init.Value == nil || init.Value.Position() != init.Pos {
		if f.resolves(q) {
			f.add(x, o, plain)
		}
		return
	}
	if f.resolves(q) {
		f.add(x, o, fieldSide)
		return
	}
	value := *o
	value.node = init.Value
	if f.resolves(&query{unit: f.unit, index: x, occ: &value}) {
		f.add(x, o, valueSide)
	}
}

func (f *finder) search() []reference {
	for _, file := range f.unit.Files {
		if file.Path == "" {
			continue
		}
		t := f.server.text(file.Path)
		x := newIndex(file, t)
		for i := range x.occurrences {
			f.visit(x, &x.occurrences[i])
		}
	}
	return f.found
}

func (s *Server) referencesTo(p TextDocumentPositionParams) (Location, []reference, bool) {
	q := s.query(p)
	if q == nil {
		return Location{}, nil, false
	}
	tg, ok := q.definition()
	if !ok {
		return Location{}, nil, false
	}
	want := s.locate(tg)
	f := &finder{server: s, unit: q.unit, want: want, seen: map[Location]bool{}}
	refs := f.search()
	return want, refs, true
}

func (s *Server) references(p ReferenceParams) any {
	at := TextDocumentPositionParams{TextDocument: p.TextDocument, Position: p.Position}
	declaration, refs, ok := s.referencesTo(at)
	if !ok {
		return nil
	}
	locations := []Location{}
	for _, r := range refs {
		if r.location == declaration && !p.Context.IncludeDeclaration {
			continue
		}
		locations = append(locations, r.location)
	}
	return locations
}

func (s *Server) renameError(p RenameParams) error {
	path := pathOf(p.TextDocument.URI)
	switch {
	case !identifier.MatchString(p.NewName):
		return fmt.Errorf("%q is not a valid name", p.NewName)
	case lexer.IsKeyword(p.NewName):
		return fmt.Errorf("%q is a keyword", p.NewName)
	case strings.HasPrefix(p.NewName, "__"):
		return fmt.Errorf("names starting with __ are reserved")
	case !s.fresh[s.entry(path)]:
		return errors.New("rename needs the program to compile; fix the errors or wait for the check to finish")
	}
	return nil
}

func (s *Server) rename(p RenameParams) (WorkspaceEdit, error) {
	if err := s.renameError(p); err != nil {
		return WorkspaceEdit{}, err
	}
	at := TextDocumentPositionParams{TextDocument: p.TextDocument, Position: p.Position}
	declaration, refs, ok := s.referencesTo(at)
	if !ok {
		return WorkspaceEdit{}, errors.New("nothing to rename here")
	}
	if !s.declares(declaration, refs) {
		return WorkspaceEdit{}, errors.New("only names declared in this program can be renamed")
	}
	changes := map[string][]TextEdit{}
	for _, r := range refs {
		edit := TextEdit{Range: r.location.Range, NewText: r.rewrite(p.NewName)}
		changes[r.location.URI] = append(changes[r.location.URI], edit)
	}
	return WorkspaceEdit{Changes: changes}, nil
}

func (s *Server) declares(declaration Location, refs []reference) bool {
	for _, r := range refs {
		if r.location != declaration {
			continue
		}
		t := s.text(pathOf(declaration.URI))
		line := t.line(declaration.Range.Start.Line)
		from := t.byteCol(declaration.Range.Start.Line, declaration.Range.Start.Character)
		to := t.byteCol(declaration.Range.End.Line, declaration.Range.End.Character)
		return line[from:to] == r.name
	}
	return false
}

func (r reference) rewrite(name string) string {
	switch r.side {
	case fieldSide:
		return fmt.Sprintf("%s: %s", name, r.name)
	case valueSide:
		return fmt.Sprintf("%s: %s", r.name, name)
	}
	return name
}
