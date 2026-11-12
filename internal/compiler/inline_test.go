package compiler

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

const slotCalleeProgram = `fn label(n: i64) -> str {
    "n={n}"
}

fn main() {
    print(label(1))
    print(label(2))
}
`

func TestInlinesCalleeWithSlot(t *testing.T) {
	prog := releaseProgram(t, slotCalleeProgram)
	if hasFunction(prog, "label") {
		t.Error("label is still reachable")
	}
	main := function(t, prog, "main")
	if strings.Contains(main.String(), " call label ") {
		t.Errorf("main still calls label:\n%s", main)
	}
	if !strings.Contains(main.String(), "slots=[3 3]") {
		t.Errorf("main did not take label's frame slots:\n%s", main)
	}
}

const interpolationProgram = `fn wrap(s: str) -> str {
    "[{s}]"
}

fn main() {
    print(wrap("a"))
    print(wrap("bc"))
}
`

func TestAppendStrInlinedAndPromoted(t *testing.T) {
	prog := releaseProgram(t, interpolationProgram)
	main := function(t, prog, "main")
	if strings.Contains(main.String(), " call __append_str ") {
		t.Errorf("main still calls __append_str:\n%s", main)
	}
	if strings.Contains(functionAsm(t, prog, "main"), "call fn___append_str") {
		t.Errorf("main's asm still calls fn___append_str")
	}
	if len(main.Slots) != 0 {
		t.Errorf("main keeps frame slots %v:\n%s", main.Slots, main)
	}
}

func TestDevInlinesNothing(t *testing.T) {
	prog := devProgram(t, slotCalleeProgram)
	main := function(t, prog, "main")
	if !strings.Contains(main.String(), " call label ") {
		t.Errorf("a dev build inlined label:\n%s", main)
	}
	if len(main.Slots) != 0 {
		t.Errorf("main has slots %v", main.Slots)
	}
}

const devirtualisedProgram = `fn double(x: i64) -> i64 {
    x * 2
}

fn negate(x: i64) -> i64 {
    0 - x
}

fn run(f: fn(i64) -> i64, v: i64) -> i64 {
    f(v) + 1
}

fn main() {
    print(run(double, 20))
    print(run(negate, 20))
}
`

func TestDevirtualisedSiteInlined(t *testing.T) {
	prog := releaseProgram(t, devirtualisedProgram)
	main := function(t, prog, "main")
	for _, name := range []string{"run", "double", "negate", "double..value", "negate..value"} {
		if strings.Contains(main.String(), fmt.Sprintf(" call %s ", name)) {
			t.Errorf("main still calls %s:\n%s", name, main)
		}
	}
	if slices.ContainsFunc(asmLines(functionAsm(t, prog, "main")), indirectCall.MatchString) {
		t.Errorf("main still makes an indirect call:\n%s", functionAsm(t, prog, "main"))
	}
}
