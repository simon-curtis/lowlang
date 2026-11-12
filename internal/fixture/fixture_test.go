package fixture

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestWriteThenReadRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "program.low")
	want := Sidecars{
		Stdout:    "5\n",
		Stderr:    "panic: integer divide by zero\n",
		Panics:    true,
		MaxRSSKB:  16000,
		MaxTime:   1500 * time.Millisecond,
		Libraries: []string{"libc.so.6", "libm.so.6"},
		SIMD:      true,
		LinuxOnly: true,
	}
	if err := want.Write(path); err != nil {
		t.Fatal(err)
	}
	got, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("read %+v, want %+v", got, want)
	}
}

func TestWriteOmitsEmptySidecars(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "program.low")
	if err := (Sidecars{}).Write(path); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("wrote %d files for an empty fixture", len(entries))
	}
}

func TestReadWithoutSidecarsIsEmpty(t *testing.T) {
	got, err := Read(filepath.Join(t.TempDir(), "program.low"))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, Sidecars{}) {
		t.Errorf("read %+v", got)
	}
	if !got.Windows() {
		t.Error("a fixture without tags is not built for Windows")
	}
}

func TestReadRejectsUnknownTag(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "program.tags"), []byte("simd\nfast\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(filepath.Join(dir, "program.low")); err == nil {
		t.Error("an unknown tag was accepted")
	}
}

func TestReadRejectsBadLimits(t *testing.T) {
	for _, ext := range []string{MaxRSSExt, MaxTimeExt} {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "program"+ext), []byte("soon\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := Read(filepath.Join(dir, "program.low")); err == nil {
			t.Errorf("%s accepted an unparsable limit", ext)
		}
	}
}

func TestLibrariesExcludeWindows(t *testing.T) {
	if (Sidecars{Libraries: []string{"libm.so.6"}}).Windows() {
		t.Error("a program linking shared libraries is built for Windows")
	}
	if (Sidecars{LinuxOnly: true}).Windows() {
		t.Error("a linux-only program is built for Windows")
	}
}

func TestDirectoryFixtureSidecarsSitBesideIt(t *testing.T) {
	dir := t.TempDir()
	program := filepath.Join(dir, "modules")
	if err := os.Mkdir(program, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "modules.out"), []byte("ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Read(program)
	if err != nil {
		t.Fatal(err)
	}
	if got.Stdout != "ok\n" {
		t.Errorf("stdout %q", got.Stdout)
	}
}
