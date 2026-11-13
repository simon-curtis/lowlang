package uitest

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lowlang/internal/fixture/fixturetest"
)

const (
	goldenPattern  = "testdata/freetype/*.gold"
	goldenFontLine = "font "
	goldenTimeout  = 10 * time.Minute
)

type goldenFile struct {
	name  string
	font  string
	lines []string
}

func readGolden(t *testing.T, path string) goldenFile {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	trimmed := strings.TrimSpace(string(content))
	lines := strings.Split(trimmed, "\n")
	if len(lines) < 3 || !strings.HasPrefix(lines[0], goldenFontLine) {
		t.Fatalf("%s: the first line must be %q followed by a repository path", path, goldenFontLine)
	}
	relative := strings.TrimPrefix(lines[0], goldenFontLine)
	root := fixturetest.Root()
	font := filepath.Join(root, relative)
	return goldenFile{name: filepath.Base(path), font: font, lines: lines[1:]}
}

func (g goldenFile) arguments(t *testing.T) []string {
	t.Helper()
	arguments := []string{g.font}
	for _, line := range g.lines[1:] {
		fields := strings.Fields(line)
		if len(fields) != 4 {
			t.Fatalf("%s: bad case line %q", g.name, line)
		}
		arguments = append(arguments, fields[:3]...)
	}
	return arguments
}

func differingLines(want, got []string) []string {
	var differences []string
	for i := 0; i < max(len(want), len(got)); i++ {
		var expected, actual string
		if i < len(want) {
			expected = want[i]
		}
		if i < len(got) {
			actual = got[i]
		}
		if expected != actual {
			differences = append(differences, fmt.Sprintf("line %d\n  want %q\n  got  %q", i+2, expected, actual))
		}
	}
	return differences
}

func TestFreeTypeGolden(t *testing.T) {
	fixturetest.RequireLinuxHost(t)
	paths, err := filepath.Glob(goldenPattern)
	if err != nil || len(paths) == 0 {
		t.Fatalf("no golden files match %s", goldenPattern)
	}
	binary := buildProgram(t, "freetypetest")
	for _, path := range paths {
		golden := readGolden(t, path)
		t.Run(golden.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), goldenTimeout)
			defer cancel()
			arguments := golden.arguments(t)
			stdout, err := exec.CommandContext(ctx, binary, arguments...).Output()
			if err != nil {
				t.Fatalf("%v\n%s", err, stdout)
			}
			output := strings.TrimSpace(string(stdout))
			got := strings.Split(output, "\n")
			differences := differingLines(golden.lines, got)
			if len(differences) == 0 {
				return
			}
			listed := strings.Join(differences, "\n")
			t.Errorf("%d of %d lines differ from %s:\n%s\nrun `go run ./cmd/ftgold -glyphs <font> <mode> <size> <phase>` and `freetypetest -glyphs <font> <mode> <size> <phase>` and compare the per-glyph lines to find the glyph",
				len(differences), len(golden.lines), golden.name, listed)
		})
	}
}
