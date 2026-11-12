package corpus

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"lowlang/internal/fixture"
	"lowlang/internal/fuzz/model"
)

func program(value uint64) *model.Program {
	return &model.Program{Main: []model.Stmt{&model.Print{Value: &model.Lit{Kind: model.I64, Bits: value}}}}
}

func TestWriteProducesRunFixture(t *testing.T) {
	dir := t.TempDir()
	fixture := FromProgram(program(5), model.Outcome{Stdout: "5\n", Panic: "integer divide by zero"})
	path, err := fixture.Write(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(filepath.Base(path), "fuzz_") || filepath.Ext(path) != ".low" {
		t.Errorf("path %s", path)
	}
	want := map[string]string{".out": "5\n", ".err": "panic: integer divide by zero\n"}
	for ext, content := range want {
		got, err := os.ReadFile(strings.TrimSuffix(path, ".low") + ext)
		if err != nil || string(got) != content {
			t.Errorf("%s is %q, %v; want %q", ext, got, err, content)
		}
	}
}

func TestWrittenFixtureReadsBackThroughTheRunnerFormat(t *testing.T) {
	written := FromProgram(program(5), model.Outcome{Stdout: "5\n", Panic: "integer divide by zero"})
	path, err := written.Write(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	got, err := fixture.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, written.Sidecars) {
		t.Errorf("read %+v, wrote %+v", got, written.Sidecars)
	}
	if !got.Panics || !got.Windows() || got.SIMD {
		t.Errorf("run settings %+v", got)
	}
}

func TestWriteOmitsEmptyExpectations(t *testing.T) {
	dir := t.TempDir()
	path, err := FromProgram(program(1), model.Outcome{}).Write(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, ext := range []string{".out", ".err"} {
		if _, err := os.Stat(strings.TrimSuffix(path, ".low") + ext); !os.IsNotExist(err) {
			t.Errorf("%s exists: %v", ext, err)
		}
	}
}

func TestNameFollowsContent(t *testing.T) {
	a := FromProgram(program(1), model.Outcome{})
	b := FromProgram(program(1), model.Outcome{})
	c := FromProgram(program(2), model.Outcome{})
	if a.Name != b.Name || a.Name == c.Name {
		t.Errorf("names %s %s %s", a.Name, b.Name, c.Name)
	}
}

func TestWriteReportsUnwritableDirectory(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := FromProgram(program(1), model.Outcome{}).Write(filepath.Join(blocker, "sub")); err == nil {
		t.Error("expected an error")
	}
}
