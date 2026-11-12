package compiler

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"lowlang/internal/ast"
	"lowlang/internal/diag"
	"lowlang/internal/parser"
	"lowlang/internal/target"
)

const moduleFile = "low.mod"

//go:embed std
var stdFS embed.FS

func isStd(module string) bool {
	return module == "std" || strings.HasPrefix(module, "std/")
}

type tree struct {
	files      fs.FS
	join       func(elem ...string) string
	privileged bool
}

var stdTree = tree{files: stdFS, join: path.Join, privileged: true}

func diskTree(files fs.FS) tree {
	return tree{files: files, join: filepath.Join}
}

func (t tree) sources(dir string, platform target.OS) ([]string, error) {
	entries, err := fs.ReadDir(t.files, dir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if builds(e, platform) {
			files = append(files, t.join(dir, e.Name()))
		}
	}
	return files, nil
}

func builds(entry fs.DirEntry, platform target.OS) bool {
	name := entry.Name()
	return !entry.IsDir() && strings.HasSuffix(name, ".low") && platform.Builds(name)
}

func (t tree) parse(name string) (*ast.File, error) {
	src, err := fs.ReadFile(t.files, name)
	if err != nil {
		return nil, err
	}
	file, err := parseSource(src, name, t.privileged)
	if err != nil {
		return nil, err
	}
	file.Path = name
	return file, nil
}

func parseSource(src []byte, name string, privileged bool) (*ast.File, error) {
	return parser.ParseSource(src, diag.Start(name), privileged)
}

func ParseFile(files fs.FS, name string) (*ast.File, error) {
	return diskTree(files).parse(name)
}

func Root(files fs.FS, dir string) (string, bool) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", false
	}
	for d := abs; ; d = filepath.Dir(d) {
		if _, err := fs.Stat(files, filepath.Join(d, moduleFile)); err == nil {
			return d, true
		}
		if filepath.Dir(d) == d {
			return "", false
		}
	}
}

type edge struct {
	to  string
	pos diag.Pos
}

type loader struct {
	root     string
	platform target.OS
	disk     tree
	std      tree
	files    []*ast.File
	sources  []string
	edges    map[string][]edge
	pending  []edge
}

func load(entryPath string, platform target.OS, disk fs.FS) (*loader, error) {
	l := &loader{platform: platform, disk: diskTree(disk), std: stdTree, edges: map[string][]edge{}}
	info, err := fs.Stat(disk, entryPath)
	if err != nil {
		return nil, err
	}
	dir, entry := filepath.Dir(entryPath), []string{entryPath}
	if info.IsDir() {
		dir = entryPath
		if entry, err = l.disk.sources(entryPath, platform); err != nil {
			return nil, err
		}
		if len(entry) == 0 {
			return nil, fmt.Errorf("%s: no .low files", entryPath)
		}
	}
	l.root = dir
	if root, found := Root(disk, dir); found {
		l.root = root
	}
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

func (l *loader) userFiles() []*ast.File {
	return slices.DeleteFunc(slices.Clone(l.files), func(f *ast.File) bool { return isStd(f.Module) })
}

func (l *loader) treeOf(module string) tree {
	if isStd(module) {
		return l.std
	}
	return l.disk
}

func (l *loader) imported(e edge) error {
	if _, done := l.edges[e.to]; done {
		return nil
	}
	source := l.treeOf(e.to)
	dir, where := e.to, "the standard library"
	if !source.privileged {
		dir = filepath.Join(l.root, filepath.FromSlash(e.to))
		where = dir
	}
	files, err := source.sources(dir, l.platform)
	if errors.Is(err, fs.ErrNotExist) || (err == nil && len(files) == 0) {
		return diag.Errorf(e.pos, "cannot find module %s in %s", e.to, where)
	}
	if err != nil {
		return err
	}
	return l.module(e.to, files)
}

func (l *loader) module(module string, files []string) error {
	l.edges[module] = []edge{}
	source := l.treeOf(module)
	for _, name := range files {
		file, err := source.parse(name)
		if err != nil {
			return err
		}
		file.Module = module
		l.files = append(l.files, file)
		if !source.privileged {
			l.sources = append(l.sources, name)
		}
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
			return diag.Errorf(imp.Pos, "module name %s imported twice", imp.Alias)
		}
		seen[imp.Alias] = true
		e := edge{to: imp.Path, pos: imp.Pos}
		l.edges[file.Module] = append(l.edges[file.Module], e)
		l.pending = append(l.pending, e)
	}
	return nil
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
				return diag.Errorf(e.pos, "import cycle: %s", strings.Join(display(cycle), " -> "))
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
