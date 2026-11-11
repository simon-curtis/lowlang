package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"lowlang/internal/ir"
)

const comparatorsProgram = `fn insertion(xs: []i64, less: fn(i64, i64) -> bool) {
    for i in 1..xs.len {
        var j = i
        for j > 0 && less(xs[j], xs[j - 1]) {
            let t = xs[j]
            xs[j] = xs[j - 1]
            xs[j - 1] = t
            j -= 1
        }
    }
}

fn main() {
    var a: []i64 = [5, 3, 9, 1, 7]
    var b: []i64 = [5, 3, 9, 1, 7]
    insertion(a, fn(x: i64, y: i64) -> bool { x < y })
    insertion(b, fn(x: i64, y: i64) -> bool { x > y })
    print(a[0])
    print(b[0])
}
`

func clonesOf(prog *ir.Program, name string) []*ir.Func {
	var clones []*ir.Func
	for _, f := range prog.Funcs {
		if strings.HasPrefix(f.Name, name+"..with") {
			clones = append(clones, f)
		}
	}
	return clones
}

func readFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "run", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func requireDirect(t *testing.T, prog *ir.Program, f *ir.Func) {
	t.Helper()
	if strings.Contains(f.String(), " call fn(") {
		t.Errorf("%s still calls through a function value:\n%s", f.Name, f)
	}
	if slices.ContainsFunc(asmLines(functionAsm(t, prog, f.Name)), indirectCall.MatchString) {
		t.Errorf("%s still makes an indirect call in its asm", f.Name)
	}
}

func TestTwoLambdasCloned(t *testing.T) {
	prog := analyseRelease(t, comparatorsProgram)
	clones := clonesOf(prog, "insertion")
	if len(clones) != 2 {
		t.Fatalf("got %d clones of insertion, want 2", len(clones))
	}
	for _, clone := range clones {
		requireDirect(t, prog, clone)
	}
	if hasFunction(prog, "insertion") {
		t.Error("the original insertion is still reachable")
	}
	main := function(t, prog, "main")
	for _, clone := range clones {
		if !strings.Contains(main.String(), fmt.Sprintf(" call %s ", clone.Name)) {
			t.Errorf("main does not call %s:\n%s", clone.Name, main)
		}
	}
}

func TestDevClonesNothing(t *testing.T) {
	prog := analyse(t, comparatorsProgram)
	if clones := clonesOf(prog, "insertion"); len(clones) != 0 {
		t.Errorf("a dev build cloned insertion %d times", len(clones))
	}
}

func TestCapturingClosureCloned(t *testing.T) {
	prog := analyseRelease(t, readFixture(t, "clone_closures.low"))
	clones := clonesOf(prog, "walk")
	if len(clones) != 3 {
		t.Fatalf("got %d clones of walk, want 3", len(clones))
	}
	captured := 0
	for _, clone := range clones {
		requireDirect(t, prog, clone)
		if clone.ParamWords == 5 {
			captured++
		}
	}
	if captured != 2 {
		t.Errorf("got %d clones that keep an environment word, want 2", captured)
	}
}

func TestRecursiveCloneCallsItself(t *testing.T) {
	prog := analyseRelease(t, readFixture(t, "clone_closures.low"))
	for _, clone := range clonesOf(prog, "walk") {
		self := fmt.Sprintf(" call %s ", clone.Name)
		if !strings.Contains(clone.String(), self) {
			t.Errorf("%s does not recurse into itself:\n%s", clone.Name, clone)
		}
		if strings.Contains(clone.String(), " call walk ") {
			t.Errorf("%s recurses into the original walk:\n%s", clone.Name, clone)
		}
	}
}

func TestUnknownFunctionValueKeepsOriginal(t *testing.T) {
	prog := analyseRelease(t, readFixture(t, "clone_closures.low"))
	if !hasFunction(prog, "twice") {
		t.Error("twice was removed although pick hands it function values that are not known")
	}
	if clones := clonesOf(prog, "twice"); len(clones) != 3 {
		t.Errorf("got %d clones of twice, want 3", len(clones))
	}
}

func TestCloneBudgetIsFourPerFunction(t *testing.T) {
	prog := analyseRelease(t, readFixture(t, "clone_budget_six.low"))
	clones := clonesOf(prog, "fold")
	if len(clones) != 4 {
		t.Fatalf("got %d clones of fold, want 4", len(clones))
	}
	if !hasFunction(prog, "fold") {
		t.Error("the original fold is gone although two call sites still use it")
	}
	main := function(t, prog, "main")
	if got := strings.Count(main.String(), " call fold "); got != 2 {
		t.Errorf("main calls the original fold %d times, want 2:\n%s", got, main)
	}
}

const sharedCodeProgram = `fn apply(xs: []i64, f: fn(i64) -> i64) -> i64 {
    var a = 0
    for x in xs {
        a += f(x)
    }
    var b = 0
    for x in xs {
        b += f(x) * 2
    }
    a + b
}

fn offset(xs: []i64, k: i64) -> i64 {
    apply(xs, fn(x: i64) -> i64 { x + k })
}

fn main() {
    let xs = [1, 2, 3]
    print(offset(xs, 1))
    print(offset(xs, 5))
}
`

func TestSameCodeWithDifferentEnvironmentsCloned(t *testing.T) {
	prog := analyseRelease(t, sharedCodeProgram)
	clones := clonesOf(prog, "apply")
	if len(clones) != 1 {
		t.Fatalf("got %d clones of apply, want 1", len(clones))
	}
	requireDirect(t, prog, clones[0])
	if clones[0].ParamWords != 4 {
		t.Errorf("clone has %d parameter words, want 4 with the environment kept", clones[0].ParamWords)
	}
	if hasFunction(prog, "apply") {
		t.Error("the original apply is still reachable")
	}
}
