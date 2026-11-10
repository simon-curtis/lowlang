package regex

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lowlang/internal/lexer"
)

func fixturePatterns(t *testing.T, path string) []string {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := lexer.Lex(src, path)
	if err != nil {
		t.Fatal(err)
	}
	var patterns []string
	for i := 0; i+2 < len(tokens); i++ {
		call := tokens[i].Kind == lexer.Ident && strings.HasPrefix(tokens[i].Text, "case_")
		if call && tokens[i+2].Kind == lexer.String {
			patterns = append(patterns, tokens[i+2].Text)
		}
	}
	return patterns
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
	patterns := fixturePatterns(t, base+".low")
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

func TestQuote(t *testing.T) {
	got := quote("\"\\{}\n\r\x00\xff")
	want := `"\"\\{{}}\n\r\0` + "\xff" + `"`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
