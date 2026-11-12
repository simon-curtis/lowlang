package link

import (
	"debug/elf"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const libcName = "c"

var SystemDirs = []string{
	"/usr/lib",
	"/usr/lib64",
	"/usr/lib/x86_64-linux-gnu",
	"/lib/x86_64-linux-gnu",
	"/lib64",
	"/lib",
	"/usr/local/lib",
}

var runtimeFiles = map[string]string{
	"c": "libc.so.6",
	"m": "libm.so.6",
}

type Library struct {
	Name    string
	Soname  string
	Path    string
	symbols map[string]librarySymbol
}

type librarySymbol struct {
	kind    elf.SymType
	version string
}

type Resolver struct {
	Dirs []string
}

func (r Resolver) Resolve(names []string) ([]*Library, error) {
	ordered := linkOrder(names)
	libraries := make([]*Library, 0, len(ordered))
	for _, name := range ordered {
		library, err := r.open(name)
		if err != nil {
			return nil, err
		}
		libraries = append(libraries, library)
	}
	return libraries, nil
}

func linkOrder(names []string) []string {
	ordered := make([]string, 0, len(names)+1)
	for _, name := range names {
		if name != libcName && !slices.Contains(ordered, name) {
			ordered = append(ordered, name)
		}
	}
	return append(ordered, libcName)
}

func (r Resolver) open(name string) (*Library, error) {
	path, err := r.find(name)
	if err != nil {
		return nil, err
	}
	return loadLibrary(name, path)
}

func (r Resolver) find(name string) (string, error) {
	file, ok := runtimeFiles[name]
	if !ok {
		file = fmt.Sprintf("lib%s.so", name)
	}
	for _, dir := range r.Dirs {
		path := filepath.Join(dir, file)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("link: cannot find library %q: no %s in %s", name, file, strings.Join(r.Dirs, ", "))
}

func loadLibrary(name, path string) (*Library, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, fmt.Errorf("link: library %q: %w", name, err)
	}
	file, err := elf.Open(resolved)
	if err != nil {
		return nil, fmt.Errorf("link: library %q at %s is not a shared object: %w", name, resolved, err)
	}
	defer file.Close()
	if file.Type != elf.ET_DYN {
		return nil, fmt.Errorf("link: library %q at %s is not a shared object", name, resolved)
	}
	symbols, err := file.DynamicSymbols()
	if err != nil {
		return nil, fmt.Errorf("link: library %q at %s has no dynamic symbols: %w", name, resolved, err)
	}
	return &Library{
		Name:    name,
		Soname:  sonameOf(file, path),
		Path:    resolved,
		symbols: exportedSymbols(symbols),
	}, nil
}

func sonameOf(file *elf.File, path string) string {
	names, err := file.DynString(elf.DT_SONAME)
	if err != nil || len(names) == 0 {
		return filepath.Base(path)
	}
	return names[0]
}

func exportedSymbols(symbols []elf.Symbol) map[string]librarySymbol {
	exported := make(map[string]librarySymbol, len(symbols))
	for _, sym := range symbols {
		if !exportsDefault(sym) {
			continue
		}
		if _, taken := exported[sym.Name]; !taken {
			exported[sym.Name] = librarySymbol{kind: elf.ST_TYPE(sym.Info), version: sym.Version}
		}
	}
	return exported
}

func exportsDefault(sym elf.Symbol) bool {
	if sym.Section == elf.SHN_UNDEF || sym.Name == "" {
		return false
	}
	bind := elf.ST_BIND(sym.Info)
	if bind != elf.STB_GLOBAL && bind != elf.STB_WEAK {
		return false
	}
	if !sym.HasVersion {
		return true
	}
	return !sym.VersionIndex.IsHidden() && sym.VersionIndex.Index() != 0
}
