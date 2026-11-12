package fixturetest

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"lowlang/internal/fixture"
	"lowlang/internal/link"
	"lowlang/internal/target"
)

type Program struct {
	Name string
	Path string
}

func Root() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..")
}

func Programs(t testing.TB, dir string) []Program {
	t.Helper()
	base := filepath.Join(Root(), "testdata", dir)
	entries, err := os.ReadDir(base)
	if err != nil {
		t.Fatal(err)
	}
	var programs []Program
	for _, e := range entries {
		if e.IsDir() || strings.HasSuffix(e.Name(), fixture.SourceExt) {
			programs = append(programs, Program{Name: fixture.Base(e.Name()), Path: filepath.Join(base, e.Name())})
		}
	}
	if len(programs) == 0 {
		t.Fatalf("no programs in testdata/%s", dir)
	}
	return programs
}

func Benchmarks(t testing.TB) []Program {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(Root(), "bench", "*", "main.low"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("no benchmark programs")
	}
	programs := make([]Program, len(paths))
	for i, path := range paths {
		programs[i] = Program{Name: filepath.Base(filepath.Dir(path)), Path: path}
	}
	return programs
}

func (p Program) Sidecars(t testing.TB) fixture.Sidecars {
	t.Helper()
	sidecars, err := fixture.Read(p.Path)
	if err != nil {
		t.Fatal(err)
	}
	return sidecars
}

func (p Program) Text(t testing.TB, ext string) (string, bool) {
	t.Helper()
	text, ok, err := fixture.Text(p.Path, ext)
	if err != nil {
		t.Fatal(err)
	}
	return text, ok
}

func RunProgram(name string) Program {
	return Program{Name: name, Path: filepath.Join(Root(), "testdata", "run", fmt.Sprintf("%s.low", name))}
}

func BenchmarkProgram(name string) Program {
	return Program{Name: name, Path: filepath.Join(Root(), "bench", name, "main.low")}
}

func (p Program) Source(t testing.TB) string {
	t.Helper()
	source, err := os.ReadFile(p.Path)
	if err != nil {
		t.Fatal(err)
	}
	return string(source)
}

func RequireLinuxHost(t testing.TB) {
	t.Helper()
	if target.Host() != target.Linux {
		t.Skip("runs Linux binaries, which need a Linux host")
	}
}

func RequireLibrary(t testing.TB, name string) {
	t.Helper()
	resolver := link.Resolver{Dirs: link.SystemDirs}
	if _, err := resolver.Resolve([]string{name}); err != nil {
		t.Skipf("lib%s is not available: %v", name, err)
	}
}

func Unavailable(t testing.TB, format string, args ...any) {
	t.Helper()
	if os.Getenv("CI") == "true" {
		t.Fatalf(format, args...)
	}
	t.Skipf(format, args...)
}

func Relative(t testing.TB, message, path string) string {
	t.Helper()
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	separator := string(filepath.Separator)
	for _, prefix := range []string{abs + separator, path + separator, path + ":"} {
		message = strings.ReplaceAll(message, prefix, "")
	}
	return filepath.ToSlash(message)
}
