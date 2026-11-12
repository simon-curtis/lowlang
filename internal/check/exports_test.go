package check_test

import (
	"os"
	"path/filepath"
	"testing"

	"lowlang/internal/ast"
	"lowlang/internal/check"
	"lowlang/internal/compiler"
	"lowlang/internal/target"
	"lowlang/internal/types"
)

const takesFirstSource = `struct Point {
    x: i64
}

fn biggest[T: Ord](xs: []T) -> i64 {
    xs.len
}

fn lengths[T](xs: []T) -> i64 {
    xs.len
}

fn keyed[K](m: [str: K]) -> i64 {
    m.len
}

fn plain(xs: []i64) -> i64 {
    xs.len
}

fn main() {
}
`

func TestTakesFirstFollowsGenericInference(t *testing.T) {
	path := filepath.Join(t.TempDir(), "main.low")
	if err := os.WriteFile(path, []byte(takesFirstSource), 0o644); err != nil {
		t.Fatal(err)
	}
	unit, err := compiler.Check(path, target.Linux, compiler.Overlay(nil))
	if err != nil {
		t.Fatal(err)
	}
	funcs := map[string]*ast.Func{}
	var point *types.Type
	for _, file := range unit.Files {
		for fn := range file.Declared() {
			funcs[fn.Name] = fn
		}
		for _, s := range file.Structs {
			if s.Name == "Point" {
				point = s.Type
			}
		}
	}
	points := unit.Types.Slice(point)
	numbers := unit.Types.Slice(types.I64)
	byName := unit.Types.Map(types.Str, point)
	byNumber := unit.Types.Map(types.I64, point)
	cases := []struct {
		fn   string
		from *types.Type
		want bool
	}{
		{"biggest", points, false},
		{"biggest", numbers, true},
		{"biggest", point, false},
		{"lengths", points, true},
		{"lengths", point, false},
		{"keyed", byName, true},
		{"keyed", byNumber, false},
		{"plain", numbers, true},
		{"plain", points, false},
	}
	for _, c := range cases {
		if got := check.TakesFirst(funcs[c.fn], c.from); got != c.want {
			t.Errorf("TakesFirst(%s, %s) = %t, want %t", c.fn, c.from, got, c.want)
		}
	}
}
