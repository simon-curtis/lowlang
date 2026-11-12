package compiler

import (
	"fmt"
	"strings"
	"testing"

	"lowlang/internal/ir"
)

const allocationProgram = `
fn join(a: str, b: str, c: str, d: str, e: str) -> str {
    a + b + c + d + e
}

fn label(n: i64, name: str) -> str {
    "{name}={n}:{n}"
}

fn status(code: i64) -> str {
    let m: [i64: str] = [200: "OK", 404: "Not Found"]
    m[code] ?? "?"
}

fn names(i: i64) -> str {
    let xs = ["alpha", "beta", "gamma"]
    xs[i % xs.len]
}

fn counter() -> fn() -> i64 {
    let base = 5
    fn() -> i64 { base + 1 }
}

fn bumper() -> fn() -> i64 {
    var n = 0
    fn() -> i64 {
        n += 1
        n
    }
}

fn unused() -> i64 {
    let xs = [1, 2, 3]
    xs[0]
}

fn main() {
    print(join("a", "b", "c", "d", "e"))
    print(label(1, "x"))
    print(status(200))
    print(names(1))
    print(counter()())
    print(bumper()())
}
`

func countInstructions(f *ir.Func, op string) int {
	return strings.Count(f.String(), fmt.Sprintf(" = %s ", op))
}

func TestConcatenationChainAllocatesOnce(t *testing.T) {
	prog := devProgram(t, allocationProgram)
	f := function(t, prog, "join")
	if grows := countInstructions(f, "runtime grow"); grows != 1 {
		t.Errorf("a + b + c + d + e performs %d allocations:\n%s", grows, f)
	}
}

func TestInterpolationAllocatesOnce(t *testing.T) {
	prog := devProgram(t, allocationProgram)
	f := function(t, prog, "label")
	if grows := countInstructions(f, "runtime grow"); grows != 1 {
		t.Errorf("interpolation performs %d allocations:\n%s", grows, f)
	}
}

func TestCapturedImmutableIsNotBoxed(t *testing.T) {
	prog := devProgram(t, allocationProgram)
	byValue := function(t, prog, "counter")
	if allocs := countInstructions(byValue, "runtime alloc"); allocs != 1 {
		t.Errorf("counter allocates %d objects, want the environment only:\n%s", allocs, byValue)
	}
	boxed := function(t, prog, "bumper")
	if allocs := countInstructions(boxed, "runtime alloc"); allocs != 2 {
		t.Errorf("bumper allocates %d objects, want a box and an environment:\n%s", allocs, boxed)
	}
}

func hoistedLiterals(prog *ir.Program) int {
	total := 0
	for _, f := range prog.Funcs {
		total += strings.Count(f.String(), " = cell ")
	}
	return total
}

func TestDevBuildDoesNotHoistLiterals(t *testing.T) {
	prog := devProgram(t, allocationProgram)
	if n := hoistedLiterals(prog); n != 0 {
		t.Errorf("dev build contains %d hoisted literals", n)
	}
	if prog.Cells != 0 {
		t.Errorf("dev build reserves %d cells", prog.Cells)
	}
}

func TestReleaseBuildHoistsConstantLiterals(t *testing.T) {
	prog := releaseProgram(t, allocationProgram)
	if n := hoistedLiterals(prog); n < 2 {
		t.Errorf("release build hoists %d literals, want the map and the slice", n)
	}
	if prog.Cells < 2 {
		t.Errorf("release build reserves %d cells", prog.Cells)
	}
}

func TestUnreachableFunctionsAreNotBuilt(t *testing.T) {
	prog := devProgram(t, allocationProgram)
	for _, f := range prog.Funcs {
		if f.Name == "unused" {
			t.Fatalf("unreachable function %s was built", f.Name)
		}
	}
}
