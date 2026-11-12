package regex

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixturePatterns(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
}

func fixtureOutcomes(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var outcomes []string
	for line := range strings.Lines(string(data)) {
		if strings.HasPrefix(line, "  ") {
			outcomes = append(outcomes, strings.TrimSpace(line))
		}
	}
	return outcomes
}

func TestCompileAgreesWithStdErrors(t *testing.T) {
	base := filepath.Join("..", "..", "testdata", "run", "std_regex_errors")
	patterns := fixturePatterns(t, filepath.Join(base, "patterns.txt"))
	outcomes := fixtureOutcomes(t, base+".out")
	if len(patterns) == 0 || len(patterns) != len(outcomes) {
		t.Fatalf("%d patterns, %d outcomes", len(patterns), len(outcomes))
	}
	for i, pattern := range patterns {
		want := outcomes[i]
		_, err := Compile(pattern)
		var got string
		var invalid *Error
		switch {
		case errors.As(err, &invalid):
			got = fmt.Sprintf("error at %d: %s", invalid.At, invalid.Message)
		case err != nil:
			t.Fatalf("%q: unexpected error %v", pattern, err)
		}
		if strings.HasPrefix(want, "error at ") != (got != "") || got != "" && got != want {
			t.Errorf("%q: got %q, want %q", pattern, got, want)
		}
	}
}

func TestGenerateFallsBackForLargePrograms(t *testing.T) {
	prog, err := Compile("a{2001}")
	if err != nil {
		t.Fatal(err)
	}
	source := Generate("__r", "a{2001}", prog)
	if !strings.Contains(source, `compile("a{{2001}}")`) {
		t.Errorf("large program is not compiled at run time:\n%s", source)
	}
}
