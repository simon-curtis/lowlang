package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func fixtures(t *testing.T, dir string) []string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("..", "..", "testdata", dir, "*.low"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatalf("no fixtures in testdata/%s", dir)
	}
	return paths
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
			runFixture(t, path)
		})
	}
}

func runFixture(t *testing.T, path string) {
	ctx := context.Background()
	binary := filepath.Join(t.TempDir(), "main")
	if err := build(ctx, path, binary); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, binary)
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	runErr := cmd.Run()

	wantOut, _ := expected(t, path, ".out")
	if got := stdout.String(); got != wantOut {
		t.Errorf("stdout:\n%s\nwant:\n%s", got, wantOut)
	}
	wantErr, panics := expected(t, path, ".err")
	if got := stderr.String(); got != wantErr {
		t.Errorf("stderr:\n%s\nwant:\n%s", got, wantErr)
	}
	if panics != (runErr != nil) {
		t.Errorf("exit error %v, expected panic %v", runErr, panics)
	}
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
	_, err := compile(path)
	if err == nil {
		t.Fatal("compiled without error")
	}
	got := strings.TrimPrefix(err.Error(), path+":")
	if got != strings.TrimSpace(want) {
		t.Errorf("error %q, want %q", got, strings.TrimSpace(want))
	}
}
