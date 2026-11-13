package compiler

import (
	"errors"
	"io/fs"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"lowlang/internal/ast"
	"lowlang/internal/check"
)

type embedder struct {
	disk  fs.FS
	std   fs.FS
	paths []string
	data  map[string][]byte
}

func (e *embedder) resolve(file *ast.File, name string) (check.Embedded, error) {
	if isStd(file.Module) {
		return e.stdFile(file, name)
	}
	return e.diskFile(file, name)
}

func (e *embedder) diskFile(file *ast.File, name string) (check.Embedded, error) {
	if !filepath.IsAbs(name) {
		name = filepath.Join(filepath.Dir(file.Path), name)
	}
	abs, err := filepath.Abs(name)
	if err != nil {
		return check.Embedded{}, err
	}
	data, err := fs.ReadFile(e.disk, abs)
	if err != nil {
		return check.Embedded{}, err
	}
	if !slices.Contains(e.paths, abs) {
		e.paths = append(e.paths, abs)
	}
	return e.hold(abs, data), nil
}

func (e *embedder) stdFile(file *ast.File, name string) (check.Embedded, error) {
	key := path.Join(path.Dir(file.Path), name)
	data, err := fs.ReadFile(e.std, key)
	if err != nil {
		return check.Embedded{}, err
	}
	return e.hold(key, data), nil
}

func (e *embedder) list(file *ast.File, name string) ([]string, error) {
	if isStd(file.Module) {
		return listFiles(e.std, path.Join(path.Dir(file.Path), name))
	}
	if !filepath.IsAbs(name) {
		name = filepath.Join(filepath.Dir(file.Path), name)
	}
	abs, err := filepath.Abs(name)
	if err != nil {
		return nil, err
	}
	return listFiles(e.disk, abs)
}

func listFiles(tree fs.FS, root string) ([]string, error) {
	info, err := fs.Stat(tree, root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, errors.New("not a directory")
	}
	var names []string
	err = fs.WalkDir(tree, root, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type().IsRegular() {
			inside := strings.TrimLeft(name[len(root):], `/\`)
			names = append(names, filepath.ToSlash(inside))
		}
		return nil
	})
	slices.Sort(names)
	return names, err
}

func (e *embedder) hold(key string, data []byte) check.Embedded {
	e.data[key] = data
	return check.Embedded{Path: key, Data: data}
}
