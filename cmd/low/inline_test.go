package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"lowlang/internal/amd64"
	"lowlang/internal/compiler"
	"lowlang/internal/target"
)

type frameSize struct {
	name  string
	bytes int
}

var (
	functionLabel = regexp.MustCompile(`(?m)^(fn_[A-Za-z0-9_.]+):$`)
	frameReserve  = regexp.MustCompile(`sub rsp, (\d+)`)
)

func frameSizes(asm string) []frameSize {
	labels := functionLabel.FindAllStringSubmatchIndex(asm, -1)
	var sizes []frameSize
	for i, l := range labels {
		end := len(asm)
		if i+1 < len(labels) {
			end = labels[i+1][0]
		}
		body := asm[l[1]:end]
		if m := frameReserve.FindStringSubmatch(body); m != nil {
			bytes, _ := strconv.Atoi(m[1])
			sizes = append(sizes, frameSize{name: asm[l[2]:l[3]], bytes: bytes})
		}
	}
	return sizes
}

const slotCalleeProgram = `fn label(n: i64) -> str {
    "n={n}"
}

fn main() {
    print(label(1))
    print(label(2))
}
`

func TestInlinesCalleeWithSlot(t *testing.T) {
	prog := analyseRelease(t, slotCalleeProgram)
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
	prog := analyseRelease(t, interpolationProgram)
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
	prog := analyse(t, slotCalleeProgram)
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
	prog := analyseRelease(t, devirtualisedProgram)
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

func TestInlineReport(t *testing.T) {
	if os.Getenv("LOW_STATS") == "" {
		t.Skip("set LOW_STATS=1 to print the call site and frame report")
	}
	var paths []string
	std, err := filepath.Glob(filepath.Join("..", "..", "testdata", "run", "std_*.low"))
	if err != nil {
		t.Fatal(err)
	}
	paths = append(paths, std...)
	benches, err := filepath.Glob(filepath.Join("..", "..", "bench", "*", "main.low"))
	if err != nil {
		t.Fatal(err)
	}
	paths = append(paths, benches...)
	totals := map[string]int{}
	direct, indirect, programs, residual := 0, 0, 0, 0
	var largest, largestCyclic []frameSize
	for _, path := range paths {
		unit, err := compiler.Analyse(path, target.Linux)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		report := unit.Program.CallSites()
		direct += report.Direct
		indirect += report.Indirect
		for reason, n := range report.Reasons {
			totals[reason] += n
		}
		programs++
		cyclic := map[string]bool{}
		for _, name := range report.Cyclic {
			cyclic[amd64.Symbol(name, "")] = true
		}
		sizes := frameSizes(amd64.Generate(unit.Program, nil).Asm)
		for _, s := range sizes {
			entry := frameSize{name: fmt.Sprintf("%s %s", filepath.Base(path), s.name), bytes: s.bytes}
			largest = append(largest, entry)
			if cyclic[s.name] {
				largestCyclic = append(largestCyclic, entry)
			}
		}
		residual += len(unit.Program.Funcs)
	}
	byBytes := func(a, b frameSize) int { return b.bytes - a.bytes }
	slices.SortStableFunc(largest, byBytes)
	slices.SortStableFunc(largestCyclic, byBytes)
	reasons := make([]string, 0, len(totals))
	for reason := range totals {
		reasons = append(reasons, reason)
	}
	slices.Sort(reasons)
	var out strings.Builder
	fmt.Fprintf(&out, "programs=%d funcs=%d direct-sites=%d indirect-sites=%d\n", programs, residual, direct, indirect)
	for _, reason := range reasons {
		fmt.Fprintf(&out, "  %-10s %d\n", reason, totals[reason])
	}
	for _, s := range largest[:min(8, len(largest))] {
		fmt.Fprintf(&out, "  frame %6d %s\n", s.bytes, s.name)
	}
	for _, s := range largestCyclic[:min(8, len(largestCyclic))] {
		fmt.Fprintf(&out, "  cyclic frame %6d %s\n", s.bytes, s.name)
	}
	t.Log("\n" + out.String())
}
