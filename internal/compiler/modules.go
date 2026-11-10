package compiler

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"lowlang/internal/ast"
	"lowlang/internal/lexer"
	"lowlang/internal/parser"
)

type edge struct {
	to  string
	pos lexer.Pos
}

type loader struct {
	root    string
	files   []*ast.File
	sources []string
	edges   map[string][]edge
	pending []edge
}

func load(path string) (*loader, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	dir, entry := filepath.Dir(path), []string{path}
	if info.IsDir() {
		dir = path
		if entry, err = sources(path); err != nil {
			return nil, err
		}
		if len(entry) == 0 {
			return nil, fmt.Errorf("%s: no .low files", path)
		}
	}
	l := &loader{root: root(dir), edges: map[string][]edge{}}
	if err := l.module("", entry); err != nil {
		return nil, err
	}
	for len(l.pending) > 0 {
		next := l.pending[0]
		l.pending = l.pending[1:]
		if err := l.imported(next); err != nil {
			return nil, err
		}
	}
	return l, l.cycles()
}

func root(dir string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return dir
	}
	for d := abs; ; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, "low.mod")); err == nil {
			return d
		}
		if filepath.Dir(d) == d {
			return dir
		}
	}
}

func sources(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".low") {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	return files, nil
}

func (l *loader) imported(e edge) error {
	if _, done := l.edges[e.to]; done {
		return nil
	}
	dir := filepath.Join(l.root, filepath.FromSlash(e.to))
	files, err := sources(dir)
	if errors.Is(err, os.ErrNotExist) || (err == nil && len(files) == 0) {
		return fmt.Errorf("%s: cannot find module %s in %s", e.pos, e.to, dir)
	}
	if err != nil {
		return err
	}
	return l.module(e.to, files)
}

func (l *loader) module(path string, files []string) error {
	l.edges[path] = []edge{}
	for _, name := range files {
		file, err := parseFile(name)
		if err != nil {
			return err
		}
		file.Module = path
		l.files = append(l.files, file)
		l.sources = append(l.sources, name)
		if err := l.link(file); err != nil {
			return err
		}
	}
	return nil
}

func (l *loader) link(file *ast.File) error {
	seen := map[string]bool{}
	for _, imp := range file.Imports {
		if seen[imp.Alias] {
			return fmt.Errorf("%s: module name %s imported twice", imp.Pos, imp.Alias)
		}
		seen[imp.Alias] = true
		e := edge{to: imp.Path, pos: imp.Pos}
		l.edges[file.Module] = append(l.edges[file.Module], e)
		l.pending = append(l.pending, e)
	}
	return nil
}

func parseFile(name string) (*ast.File, error) {
	src, err := os.ReadFile(name)
	if err != nil {
		return nil, err
	}
	tokens, err := lexer.Lex(src, name)
	if err != nil {
		return nil, err
	}
	file, err := parser.Parse(tokens)
	if err != nil {
		return nil, err
	}
	file.Path = name
	return file, nil
}

func (l *loader) cycles() error {
	state := map[string]int{}
	var stack []string
	var visit func(module string) error
	visit = func(module string) error {
		state[module] = 1
		stack = append(stack, module)
		for _, e := range l.edges[module] {
			switch state[e.to] {
			case 1:
				cycle := append(slices.Clone(stack[slices.Index(stack, e.to):]), e.to)
				return fmt.Errorf("%s: import cycle: %s", e.pos, strings.Join(display(cycle), " -> "))
			case 0:
				if err := visit(e.to); err != nil {
					return err
				}
			}
		}
		stack = stack[:len(stack)-1]
		state[module] = 2
		return nil
	}
	return visit("")
}

func display(modules []string) []string {
	out := make([]string, len(modules))
	for i, m := range modules {
		out[i] = m
		if m == "" {
			out[i] = "main"
		}
	}
	return out
}
