package compiler

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"

	"lowlang/internal/ast"
	"lowlang/internal/check"
)

type embedder struct {
	disk  fs.FS
	std   fs.FS
	paths []string
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
	return check.Embedded{Path: abs, Data: data}, nil
}

func (e *embedder) stdFile(file *ast.File, name string) (check.Embedded, error) {
	data, err := fs.ReadFile(e.std, path.Join(path.Dir(file.Path), name))
	if err != nil {
		return check.Embedded{}, err
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return check.Embedded{}, err
	}
	sum := sha256.Sum256(data)
	out := filepath.Join(cache, "lowlang", "embed", fmt.Sprintf("%x-%s", sum[:12], path.Base(name)))
	if err := materialize(out, data); err != nil {
		return check.Embedded{}, err
	}
	return check.Embedded{Path: out, Data: data}, nil
}

func materialize(out string, data []byte) error {
	if existing, err := os.ReadFile(out); err == nil && bytes.Equal(existing, data) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(out), "embed-*")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(temp.Name(), out)
}
