package compiler

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

type Overlay map[string][]byte

func (o Overlay) Open(name string) (fs.File, error) {
	if text, ok := o[filepath.Clean(name)]; ok {
		return &memoryFile{info: memoryInfo{name: filepath.Base(name), size: len(text)}, Reader: bytes.NewReader(text)}, nil
	}
	return os.Open(name)
}

func (o Overlay) ReadFile(name string) ([]byte, error) {
	if text, ok := o[filepath.Clean(name)]; ok {
		return text, nil
	}
	return os.ReadFile(name)
}

func (o Overlay) Stat(name string) (fs.FileInfo, error) {
	if text, ok := o[filepath.Clean(name)]; ok {
		return memoryInfo{name: filepath.Base(name), size: len(text)}, nil
	}
	return os.Stat(name)
}

func (o Overlay) ReadDir(name string) ([]fs.DirEntry, error) {
	dir := filepath.Clean(name)
	entries, err := os.ReadDir(dir)
	var held []fs.DirEntry
	for path, text := range o {
		if filepath.Dir(path) != dir {
			continue
		}
		info := memoryInfo{name: filepath.Base(path), size: len(text)}
		held = append(held, fs.FileInfoToDirEntry(info))
	}
	if err != nil && (len(held) == 0 || !errors.Is(err, fs.ErrNotExist)) {
		return nil, err
	}
	for _, entry := range held {
		entries = slices.DeleteFunc(entries, func(e fs.DirEntry) bool { return e.Name() == entry.Name() })
		entries = append(entries, entry)
	}
	slices.SortFunc(entries, func(a, b fs.DirEntry) int {
		return strings.Compare(a.Name(), b.Name())
	})
	return entries, nil
}

type memoryFile struct {
	*bytes.Reader
	info memoryInfo
}

func (f *memoryFile) Stat() (fs.FileInfo, error) {
	return f.info, nil
}

func (f *memoryFile) Close() error {
	return nil
}

type memoryInfo struct {
	name string
	size int
}

func (i memoryInfo) Name() string {
	return i.name
}

func (i memoryInfo) Size() int64 {
	return int64(i.size)
}

func (i memoryInfo) Mode() fs.FileMode {
	return 0o644
}

func (i memoryInfo) ModTime() time.Time {
	return time.Time{}
}

func (i memoryInfo) IsDir() bool {
	return false
}

func (i memoryInfo) Sys() any {
	return nil
}
