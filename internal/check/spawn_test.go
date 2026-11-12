package check_test

import (
	"os"
	"path/filepath"
	"testing"

	"lowlang/internal/ast"
	"lowlang/internal/compiler"
	"lowlang/internal/target"
)

const spawnSource = `fn work(n: i64) {
}

fn main() {
    spawn work(7)
}
`

func TestSpawnedCallIsVisitedOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "main.low")
	if err := os.WriteFile(path, []byte(spawnSource), 0o644); err != nil {
		t.Fatal(err)
	}
	unit, err := compiler.Check(path, target.Linux, compiler.Overlay(nil))
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	for _, file := range unit.Files {
		for fn := range file.Declared() {
			if fn.Name != "main" {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				if call, ok := n.(*ast.Call); ok && named(call, "work") {
					calls++
				}
				return true
			})
		}
	}
	if calls != 1 {
		t.Errorf("the spawned call was visited %d times, want 1", calls)
	}
}

func named(call *ast.Call, name string) bool {
	id, ok := call.Fn.(*ast.Ident)
	return ok && id.Name == name
}
