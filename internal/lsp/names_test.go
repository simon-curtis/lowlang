package lsp

import (
	"strings"
	"testing"

	platform "lowlang/internal/target"
)

const namesProgram = `import std/slices

struct Point {
    x: i64
    y: i64
}

/// Adds the coordinates.
fn total(p: Point) -> i64 {
    p.x + p.y
}

fn main() {
    let x = 1
    let p = Point { x, y: 2 }
    print(total(p))
    print(slices.sum([x, 2]))
}
`

func namesOf(t *testing.T, src string) []Name {
	t.Helper()
	f := scratch(t, "main.low", src)
	names, err := Names(f.path, platform.Host())
	if err != nil {
		t.Fatal(err)
	}
	return names
}

func nameAt(t *testing.T, names []Name, line, column int) (int, Name) {
	t.Helper()
	for at, n := range names {
		if n.Line == line && n.Column <= column && column <= n.Column+len(n.Name) {
			return at, n
		}
	}
	t.Fatalf("no name at %d:%d", line, column)
	return 0, Name{}
}

func TestNamesDescribeEveryOccurrence(t *testing.T) {
	names := namesOf(t, namesProgram)

	_, call := nameAt(t, names, 16, 11)
	if call.Name != "total" || call.Hover != "fn total(p: Point) -> i64" || call.Doc != "Adds the coordinates." {
		t.Fatalf("call to total described as %+v", call)
	}
	if call.Definition == nil || !call.Definition.Local || call.Definition.Line != 9 || call.Definition.Column != 4 {
		t.Fatalf("call to total defined at %+v", call.Definition)
	}

	_, imported := nameAt(t, names, 17, 18)
	if imported.Definition == nil || imported.Definition.Local || !strings.HasSuffix(imported.Definition.Path, "std/slices/slices.low") {
		t.Fatalf("slices.sum defined at %+v", imported.Definition)
	}

	_, builtin := nameAt(t, names, 16, 5)
	if builtin.Definition != nil || builtin.First != nil {
		t.Fatalf("print has a definition: %+v", builtin)
	}
}

func TestNamesGroupReferences(t *testing.T) {
	names := namesOf(t, namesProgram)

	declared, declaration := nameAt(t, names, 9, 4)
	_, call := nameAt(t, names, 16, 11)
	if declaration.First == nil || call.First == nil || *declaration.First != *call.First || *call.First > declared {
		t.Fatalf("total declaration and call are not one group: %v %v", declaration.First, call.First)
	}

	_, local := nameAt(t, names, 14, 9)
	_, shorthand := nameAt(t, names, 15, 21)
	_, field := nameAt(t, names, 4, 5)
	_, use := nameAt(t, names, 17, 23)
	if local.First == nil || use.First == nil || *local.First != *use.First {
		t.Fatalf("local x and its use are not one group: %v %v", local.First, use.First)
	}
	if shorthand.First == nil || field.First == nil || *shorthand.First != *field.First {
		t.Fatalf("shorthand x and field x are not one group: %v %v", shorthand.First, field.First)
	}
}

func TestNamesFailOnACompileError(t *testing.T) {
	f := scratch(t, "main.low", "fn main() {\n    missing()\n}\n")
	if _, err := Names(f.path, platform.Host()); err == nil {
		t.Fatal("a program that does not check produced names")
	}
}
