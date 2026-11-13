package lsp

import (
	"path/filepath"

	"lowlang/internal/compiler"
	platform "lowlang/internal/target"
)

type Definition struct {
	Path   string `json:"path"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
	Local  bool   `json:"local"`
}

type Name struct {
	Line       int         `json:"line"`
	Column     int         `json:"column"`
	Name       string      `json:"name"`
	Hover      string      `json:"hover,omitempty"`
	Doc        string      `json:"doc,omitempty"`
	Definition *Definition `json:"definition,omitempty"`
	First      *int        `json:"first,omitempty"`
}

type naming struct {
	server  *Server
	program program
	index   *index
	path    string
	names   []Name
	located map[int]Location
	primary map[Location]int
	shared  map[Location]int
}

func (n *naming) query(o *occurrence) *query {
	return &query{program: n.program, index: n.index, occ: o}
}

func (n *naming) describe(at int, o *occurrence) {
	r := n.query(o).resolve()
	name := Name{Line: o.pos.Line, Column: o.pos.Col, Name: o.name, Hover: r.hover(), Doc: r.doc()}
	if tg, ok := r.target(); ok {
		where := n.server.locate(tg)
		start := where.Range.Start
		local := filepath.Clean(tg.path) == n.path
		name.Definition = &Definition{Path: filepath.ToSlash(tg.path), Line: start.Line + 1, Column: n.server.text(tg.path).byteCol(start.Line, start.Character) + 1, Local: local}
		n.located[at] = where
		if _, seen := n.primary[where]; !seen {
			n.primary[where] = at
		}
	}
	n.names = append(n.names, name)
}

func (n *naming) share(at int, o *occurrence) {
	value, ok := shorthandValue(o)
	if !ok {
		return
	}
	other := *o
	other.node = value
	tg, ok := n.query(&other).resolve().target()
	if !ok {
		return
	}
	where := n.server.locate(tg)
	if _, seen := n.shared[where]; !seen && where != n.located[at] {
		n.shared[where] = at
	}
}

func (n *naming) group() {
	for at := range n.names {
		where, ok := n.located[at]
		if !ok {
			continue
		}
		first := n.primary[where]
		if other, ok := n.shared[where]; ok && other < first {
			first = other
		}
		n.names[at].First = &first
	}
}

func Names(path string, os platform.OS) ([]Name, error) {
	path = filepath.Clean(path)
	docs := compiler.Overlay{}
	unit, err := compiler.Check(path, os, docs)
	if err != nil {
		return nil, err
	}
	server := &Server{docs: docs}
	prog := program{unit: unit}
	file := prog.fileAt(path)
	if file == nil {
		return []Name{}, nil
	}
	n := &naming{
		server:  server,
		program: prog,
		index:   newIndex(file, server.text(path)),
		path:    path,
		names:   []Name{},
		located: map[int]Location{},
		primary: map[Location]int{},
		shared:  map[Location]int{},
	}
	for at := range n.index.occurrences {
		n.describe(at, &n.index.occurrences[at])
	}
	for at := range n.index.occurrences {
		n.share(at, &n.index.occurrences[at])
	}
	n.group()
	return n.names, nil
}
