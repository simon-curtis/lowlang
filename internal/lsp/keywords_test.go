package lsp

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"lowlang/internal/compiler"
)

func TestBuiltinFuncsExist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "builtin.low")
	for _, name := range builtinFuncs {
		src := fmt.Sprintf("fn main() {\n    %s()\n}\n", name)
		docs := sources{path: []byte(src)}
		_, err := compiler.Check(path, docs.read)
		if err == nil || strings.Contains(err.Error(), "undefined function") {
			t.Errorf("%s is not a builtin: %v", name, err)
		}
	}
}

func TestBuiltinsDocumented(t *testing.T) {
	design, err := os.ReadFile(filepath.Join("..", "..", "docs", "design.md"))
	if err != nil {
		t.Fatal(err)
	}
	_, chapter, found := strings.Cut(string(design), "\n## Builtins\n")
	if !found {
		t.Fatal("docs/design.md has no ## Builtins chapter")
	}
	chapter, _, _ = strings.Cut(chapter, "\n## ")
	entry := regexp.MustCompile("(?m)^(?:### |\\| )`(\\w+)`")
	var documented []string
	for _, match := range entry.FindAllStringSubmatch(chapter, -1) {
		documented = append(documented, match[1])
	}
	builtins := slices.Concat(builtinFuncs, builtinTypes, builtinConstants, []string{"len"})
	for _, name := range builtins {
		if !slices.Contains(documented, name) {
			t.Errorf("builtin %s has no entry in ## Builtins", name)
		}
	}
	for _, name := range documented {
		if !slices.Contains(builtins, name) {
			t.Errorf("## Builtins documents %s, which is not a builtin", name)
		}
	}
}
