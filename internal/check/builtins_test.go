package check_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"lowlang/internal/check"
	"lowlang/internal/compiler"
	"lowlang/internal/diag"
	"lowlang/internal/target"
)

func documentedBuiltins(t *testing.T) (names []string, signatures map[string][]string) {
	t.Helper()
	design, err := os.ReadFile(filepath.Join("..", "..", "docs", "design.md"))
	if err != nil {
		t.Fatal(err)
	}
	_, chapter, found := strings.Cut(string(design), "\n## Builtins\n")
	if !found {
		t.Fatal("docs/design.md has no ## Builtins chapter")
	}
	chapter, _, _ = strings.Cut(chapter, "\n## ")
	signatures = map[string][]string{}
	for _, section := range strings.Split(chapter, "\n### ") {
		heading, body, _ := strings.Cut(section, "\n")
		name := strings.Trim(heading, "`")
		if heading == name {
			continue
		}
		names = append(names, name)
		signature := regexp.MustCompile(fmt.Sprintf("`%s\\(([^)]*)\\)", name)).FindStringSubmatch(body)
		if signature != nil {
			signatures[name] = parameterNames(signature[1])
		}
	}
	row := regexp.MustCompile("(?m)^\\| `(\\w+)` \\|")
	for _, match := range row.FindAllStringSubmatch(chapter, -1) {
		names = append(names, match[1])
	}
	return names, signatures
}

func parameterNames(list string) []string {
	var names []string
	for _, param := range strings.Split(list, ",") {
		param = strings.TrimPrefix(strings.TrimSpace(param), "mut ")
		name, _, _ := strings.Cut(param, ":")
		if name = strings.TrimSpace(name); name != "" {
			names = append(names, name)
		}
	}
	return names
}

func TestBuiltinsDocumented(t *testing.T) {
	documented, signatures := documentedBuiltins(t)
	var registered []string
	for _, b := range check.Builtins() {
		if !b.Public() {
			continue
		}
		registered = append(registered, b.Name)
		if !slices.Contains(documented, b.Name) {
			t.Errorf("builtin %s has no entry in ## Builtins", b.Name)
		}
		if b.Kind != check.FunctionBuiltin {
			continue
		}
		params, ok := signatures[b.Name]
		if !ok {
			t.Errorf("## Builtins gives no signature for %s", b.Name)
		} else if !slices.Equal(params, b.Params) {
			t.Errorf("%s takes (%s) but ## Builtins documents (%s)", b.Name, strings.Join(b.Params, ", "), strings.Join(params, ", "))
		}
	}
	for _, name := range documented {
		if !slices.Contains(registered, name) {
			t.Errorf("## Builtins documents %s, which is not a builtin", name)
		}
	}
}

func TestBuiltinsAreComplete(t *testing.T) {
	seen := map[string]bool{}
	for _, b := range check.Builtins() {
		if seen[b.Name] {
			t.Errorf("%s is registered twice", b.Name)
		}
		seen[b.Name] = true
		if b.Kind == check.FunctionBuiltin && len(b.Params) == 0 && b.Public() {
			t.Errorf("%s names no parameters", b.Name)
		}
		if check.LookupBuiltin(b.Name) != b || !check.Reserved(b.Name) {
			t.Errorf("%s cannot be looked up", b.Name)
		}
	}
	for _, word := range []string{"fn", "match", "true"} {
		if !check.Reserved(word) {
			t.Errorf("%s is not reserved", word)
		}
	}
	if check.Reserved("frobnicate") {
		t.Error("frobnicate is reserved")
	}
}

func TestPublicBuiltinsRejectWrongArity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "builtin.low")
	for _, b := range check.Builtins() {
		if b.Kind != check.FunctionBuiltin || !b.Public() {
			continue
		}
		source := fmt.Sprintf("fn main() {\n    %s()\n}\n", b.Name)
		if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := compiler.Check(path, target.Linux, compiler.Overlay(nil))
		var failure *diag.Error
		if !errors.As(err, &failure) || !strings.Contains(failure.Msg, "takes") {
			t.Errorf("%s() reported %v", b.Name, err)
		}
	}
}
