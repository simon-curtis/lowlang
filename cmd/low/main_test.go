package main

import (
	"bytes"
	"context"
	"debug/elf"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"lowlang/internal/compiler"
	"lowlang/internal/peakrss"
	"lowlang/internal/target"
)

func fixtures(t *testing.T, dir string) []string {
	t.Helper()
	base := filepath.Join("..", "..", "testdata", dir)
	entries, err := os.ReadDir(base)
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, e := range entries {
		if e.IsDir() || strings.HasSuffix(e.Name(), ".low") {
			paths = append(paths, filepath.Join(base, e.Name()))
		}
	}
	if len(paths) == 0 {
		t.Fatalf("no fixtures in testdata/%s", dir)
	}
	return paths
}

func relative(t *testing.T, message, path string) string {
	t.Helper()
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, prefix := range []string{abs + "/", path + "/", path + ":"} {
		message = strings.ReplaceAll(message, prefix, "")
	}
	return message
}

func expected(t *testing.T, path, ext string) (string, bool) {
	t.Helper()
	data, err := os.ReadFile(strings.TrimSuffix(path, ".low") + ext)
	if errors.Is(err, os.ErrNotExist) {
		return "", false
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(data), true
}

func TestRun(t *testing.T) {
	for _, path := range fixtures(t, "run") {
		t.Run(filepath.Base(path), func(t *testing.T) {
			if _, measured := expected(t, path, ".maxrss"); !measured {
				t.Parallel()
			}
			runFixture(t, path)
		})
	}
}

func runFixture(t *testing.T, path string) {
	binary := filepath.Join(t.TempDir(), "main")
	if err := build(context.Background(), path, binary, target.Linux); err != nil {
		t.Fatal(err)
	}
	environments := [][]string{nil}
	if name := filepath.Base(path); strings.HasPrefix(name, "simd") || strings.HasPrefix(name, "std_") {
		environments = append(environments, []string{"LOW_SIMD=sse2"})
	}
	for _, env := range environments {
		runWithEnv(t, path, binary, env)
	}
	checkLinking(t, path, binary)
}

func runWithEnv(t *testing.T, path, binary string, env []string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	newCmd := func(stdout, stderr io.Writer) *exec.Cmd {
		cmd := exec.CommandContext(ctx, binary)
		cmd.Env = append(os.Environ(), env...)
		cmd.Stdout, cmd.Stderr = stdout, stderr
		return cmd
	}
	limit, measured := memoryLimit(t, path)
	if !measured {
		compareRun(t, path, func(stdout, stderr io.Writer) error {
			return newCmd(stdout, stderr).Run()
		}, env)
		return
	}
	var peak int64
	compareRun(t, path, func(stdout, stderr io.Writer) error {
		result, err := peakrss.Run(func() *exec.Cmd { return newCmd(stdout, stderr) })
		peak = result.PeakKB
		return err
	}, env)
	t.Logf("peak memory %d KB, limit %d KB", peak, limit)
	if peak > limit {
		t.Errorf("peak memory %d KB exceeds %d KB", peak, limit)
	}
}

func TestRunWindows(t *testing.T) {
	wine, err := exec.LookPath("wine")
	if err != nil {
		t.Skip("wine is not installed")
	}
	paths := slices.DeleteFunc(fixtures(t, "run"), func(path string) bool {
		_, glibc := expected(t, path, ".needed")
		return glibc || strings.HasPrefix(filepath.Base(path), "std_")
	})
	for _, path := range append(paths, fixtures(t, "windows")...) {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			runWindowsFixture(t, wine, path)
		})
	}
}

func runWindowsFixture(t *testing.T, wine, path string) {
	binary := filepath.Join(t.TempDir(), "main.exe")
	if err := build(context.Background(), path, binary, target.Windows); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, wine, binary)
	cmd.Env = append(os.Environ(), "WINEDEBUG=-all", "DISPLAY=", "WAYLAND_DISPLAY=")
	compareRun(t, path, func(stdout, stderr io.Writer) error {
		cmd.Stdout, cmd.Stderr = stdout, stderr
		return cmd.Run()
	}, nil)
}

func compareRun(t *testing.T, path string, run func(stdout, stderr io.Writer) error, env []string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	runErr := run(&stdout, &stderr)

	wantOut, _ := expected(t, path, ".out")
	if got := stdout.String(); got != wantOut {
		t.Errorf("%v stdout:\n%s\nwant:\n%s", env, got, wantOut)
	}
	wantErr, panics := expected(t, path, ".err")
	if got := stderr.String(); got != wantErr {
		t.Errorf("%v stderr:\n%s\nwant:\n%s", env, got, wantErr)
	}
	if panics != (runErr != nil) {
		t.Errorf("%v exit error %v, expected panic %v", env, runErr, panics)
	}
}

func checkLinking(t *testing.T, path, binary string) {
	f, err := elf.Open(binary)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	needed, err := f.ImportedLibraries()
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(needed)
	want, ok := expected(t, path, ".needed")
	if !ok {
		if len(needed) > 0 || slices.ContainsFunc(f.Progs, func(p *elf.Prog) bool { return p.Type == elf.PT_INTERP }) {
			t.Errorf("expected a static binary, found shared libraries %v", needed)
		}
		return
	}
	if got := strings.Join(needed, "\n"); got != strings.TrimSpace(want) {
		t.Errorf("shared libraries:\n%s\nwant:\n%s", got, strings.TrimSpace(want))
	}
}

func memoryLimit(t *testing.T, path string) (int64, bool) {
	t.Helper()
	text, ok := expected(t, path, ".maxrss")
	if !ok {
		return 0, false
	}
	kb, err := strconv.ParseInt(strings.TrimSpace(text), 10, 64)
	if err != nil {
		t.Fatalf("invalid .maxrss: %v", err)
	}
	return kb, true
}

func TestFail(t *testing.T) {
	for _, path := range fixtures(t, "fail") {
		t.Run(filepath.Base(path), func(t *testing.T) {
			failFixture(t, path)
		})
	}
}

func failFixture(t *testing.T, path string) {
	want, ok := expected(t, path, ".err")
	if !ok {
		t.Fatal("missing .err fixture")
	}
	_, err := compiler.Analyse(path, target.Linux)
	if err == nil {
		t.Fatal("compiled without error")
	}
	got := relative(t, err.Error(), path)
	if got != strings.TrimSpace(want) {
		t.Errorf("error %q, want %q", got, strings.TrimSpace(want))
	}
}
