package lsp

import (
	"fmt"
	"path/filepath"
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
