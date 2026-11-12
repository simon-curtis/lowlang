package lsp

import (
	"errors"
	"fmt"

	"lowlang/internal/ast"
	"lowlang/internal/check"
	"lowlang/internal/diag"
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

type finder struct {
	server *Server
	program
	want  Location
	seen  map[Location]bool
	found []reference
}

func (f *finder) resolves(q *query) bool {
	tg, ok := q.resolve().target()
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

func shorthandValue(o *occurrence) (any, bool) {
	switch n := o.node.(type) {
	case *ast.FieldInit:
		return n.Value, n.Form == ast.FieldShorthand
	case *ast.FieldPattern:
		return n.Bind, n.Shorthand
	}
	return nil, false
}

func (f *finder) visit(x *index, o *occurrence) {
	q := &query{program: f.program, index: x, occ: o}
	shared, ok := shorthandValue(o)
	if !ok {
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
	value.node = shared
	if f.resolves(&query{program: f.program, index: x, occ: &value}) {
		f.add(x, o, valueSide)
	}
}

func (f *finder) search() []reference {
	for _, file := range f.files() {
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
	tg, ok := q.resolve().target()
	if !ok {
		return Location{}, nil, false
	}
	want := s.locate(tg)
	f := &finder{server: s, program: q.program, want: want, seen: map[Location]bool{}}
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

func nameError(name string) error {
	if err := check.DeclarationNameError(name); err != nil {
		return err
	}
	tokens, err := lexer.Lex([]byte(name), diag.Start(""), false)
	var lexed *diag.Error
	switch {
	case errors.As(err, &lexed):
		return errors.New(lexed.Msg)
	case err != nil || len(tokens) != 3 || tokens[0].Kind != lexer.Ident || tokens[0].Text != name:
		return fmt.Errorf("%q is not a valid name", name)
	}
	return nil
}

func (s *Server) renameError(p RenameParams) error {
	if err := nameError(p.NewName); err != nil {
		return err
	}
	if !s.stateOf(pathOf(p.TextDocument.URI)).fresh {
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
