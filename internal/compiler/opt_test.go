package compiler

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"lowlang/internal/amd64"
	"lowlang/internal/fixture/fixturetest"
	"lowlang/internal/ir"
	"lowlang/internal/target"
)

func devProgram(t *testing.T, source string) *ir.Program {
	t.Helper()
	return programWith(t, source, AnalyseDev)
}

func releaseProgram(t *testing.T, source string) *ir.Program {
	t.Helper()
	return programWith(t, source, Analyse)
}

func programWith(t *testing.T, source string, analyser func(string, target.OS) (*Unit, error)) *ir.Program {
	t.Helper()
	path := filepath.Join(t.TempDir(), "main.low")
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	unit, err := analyser(path, target.Linux)
	if err != nil {
		t.Fatal(err)
	}
	return unit.Program
}

func function(t *testing.T, prog *ir.Program, name string) *ir.Func {
	t.Helper()
	for _, f := range prog.Funcs {
		if f.Name == name {
			return f
		}
	}
	t.Fatalf("no function %s", name)
	return nil
}

func countChecks(f *ir.Func) int {
	return strings.Count(f.String(), " = check [")
}

func functionAsm(t *testing.T, prog *ir.Program, name string) string {
	t.Helper()
	out, err := amd64.Generate(prog, amd64.NewPlatform(target.Linux), nil)
	if err != nil {
		t.Fatal(err)
	}
	asm := out.Asm
	label := amd64.Symbol(name, "") + ":\n"
	start := strings.Index(asm, label)
	if start < 0 {
		t.Fatalf("no label %s", label)
	}
	body := asm[start+len(label):]
	if end := regexp.MustCompile(`(?m)^fn_[A-Za-z0-9_.]+:$`).FindStringIndex(body); end != nil {
		body = body[:end[0]]
	}
	return body
}

const boundsProgram = `fn sum(xs: []i64) -> i64 {
    var t = 0
    for i in 0..xs.len {
        t += xs[i]
    }
    t
}

fn nested(xs: []i64) -> i64 {
    var t = 0
    for i in 0..xs.len {
        for j in 0..xs.len {
            t += xs[i] * xs[j]
        }
    }
    t
}

fn bump(xs: []i64, i: i64) {
    xs[i] += 1
    xs[i] += 1
}

fn grow() -> i64 {
    var xs: []i64 = [1]
    var t = 0
    var i = 0
    for i < xs.len {
        push(mut xs, i)
        t += xs[i]
        i += 1
        if i > 5 {
            return t
        }
    }
    t
}

fn main() {
    let xs = [1, 2, 3]
    print(sum(xs))
    print(nested(xs))
    bump(xs, 1)
    print(grow())
}
`

func TestBoundsChecksRemoved(t *testing.T) {
	prog := devProgram(t, boundsProgram)
	cases := []struct {
		name   string
		checks int
	}{
		{"sum", 0},
		{"nested", 0},
		{"bump", 1},
		{"grow", 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := function(t, prog, c.name)
			if got := countChecks(f); got != c.checks {
				t.Errorf("%d checks, want %d\n%s", got, c.checks, f)
			}
			panics := strings.Count(functionAsm(t, prog, c.name), "rt_panic_index")
			if (panics > 0) != (c.checks > 0) {
				t.Errorf("%d references to rt_panic_index with %d checks", panics, c.checks)
			}
		})
	}
}

func TestFloat32CallResultIsCleared(t *testing.T) {
	prog := devProgram(t, `extern "m" {
    fn sqrtf(x: f32) -> f32
}

fn root(x: f32) -> f32 {
    sqrtf(x)
}

fn main() {
    print(root(16.0))
}
`)
	body := functionAsm(t, prog, "root")
	_, after, found := strings.Cut(body, "call sqrtf")
	if !found {
		t.Fatalf("no call to sqrtf:\n%s", body)
	}
	cleared := regexp.MustCompile(`movd e[a-z0-9]+, xmm0|xorps (xmm\d+), xmm\d+\n\s+movss xmm\d+, xmm0`)
	if !cleared.MatchString(after) {
		t.Errorf("the f32 result keeps its upper bits:\n%s", after)
	}
}

const foldProgram = `fn constant_branch(x: i64) -> i64 {
    if 1 < 2 {
        x
    } else {
        x * 3 + 1
    }
}

fn dead_cycle(n: i64) -> i64 {
    var dead = 0
    for i in 0..n {
        dead += i
    }
    n
}

fn touch(mut xs: []i64) {
    xs[0] = 5
}

fn unused_slot_load() -> i64 {
    var xs: []i64 = [1]
    touch(mut xs)
    let unused = xs.len
    7
}

fn divisions(x: i64, y: u32, z: i32) -> i64 {
    x / 10 + x % 7 + x / -3 + x / 8 + x % 1024 + i64(y / 10) + i64(y % 3) + i64(z / 100) + i64(z % 16)
}

fn kept_divisions(x: i64, d: i64) -> i64 {
    x / d + x / -1 + x % 0
}

fn main() {
    print(constant_branch(2))
    print(dead_cycle(3))
    print(unused_slot_load())
    print(divisions(100, 100, 100))
    print(kept_divisions(100, 3))
}
`

func TestOptimiserRemovals(t *testing.T) {
	prog := devProgram(t, foldProgram)
	cases := []struct {
		name    string
		pattern string
		count   int
	}{
		{"constant_branch", "\n  if ", 0},
		{"constant_branch", " = mul", 0},
		{"dead_cycle", " = add", 1},
		{"dead_cycle", " = phi", 1},
		{"unused_slot_load", " = load", 0},
		{"divisions", " = div", 0},
		{"divisions", " = rem", 0},
		{"kept_divisions", " = div", 2},
		{"kept_divisions", " = rem", 1},
	}
	for _, c := range cases {
		t.Run(c.name+c.pattern, func(t *testing.T) {
			f := function(t, prog, c.name)
			if got := strings.Count(f.String(), c.pattern); got != c.count {
				t.Errorf("%d of %q, want %d\n%s", got, c.pattern, c.count, f)
			}
		})
	}
}

const layoutProgram = `fn fib(n: i64) -> i64 {
    if n < 2 { n } else { fib(n - 1) + fib(n - 2) }
}

fn sieve(limit: i64) -> i64 {
    var composite: []u8 = []
    for i in 0..limit {
        push(mut composite, u8(0))
    }
    var count = 0
    for i in 2..limit {
        if composite[i] == 0 {
            count += 1
            var j = i * i
            for j < limit {
                composite[j] = 1
                j += i
            }
        }
    }
    count
}

fn search(xs: []i64, key: i64) -> i64 {
    var found = -1
    for i in 0..xs.len {
        if xs[i] == key {
            found = i
            break
        }
        if xs[i] < 0 {
            continue
        }
    }
    found
}

fn main() {
    print(fib(10))
    print(sieve(100))
    print(search([1, 2, 3], 2))
}
`

var asmLabel = regexp.MustCompile(`^[.A-Za-z_][.A-Za-z0-9_]*:$`)

func asmLines(body string) []string {
	var lines []string
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return lines
}

func jumpsToNextLabel(lines []string) []string {
	var found []string
	for i := 0; i+1 < len(lines); i++ {
		target, isJump := strings.CutPrefix(lines[i], "jmp ")
		if isJump && lines[i+1] == target+":" {
			found = append(found, lines[i])
		}
	}
	return found
}

func jumpOnlyLabels(lines []string) []string {
	var found []string
	for i := 0; i+1 < len(lines); i++ {
		if asmLabel.MatchString(lines[i]) && strings.HasPrefix(lines[i+1], "jmp ") {
			found = append(found, lines[i])
		}
	}
	return found
}

func TestNoJumpToNextBlock(t *testing.T) {
	prog := devProgram(t, layoutProgram)
	for _, name := range []string{"fib", "sieve", "search", "main"} {
		t.Run(name, func(t *testing.T) {
			body := functionAsm(t, prog, name)
			if found := jumpsToNextLabel(asmLines(body)); len(found) > 0 {
				t.Errorf("jumps to the following label %v:\n%s", found, body)
			}
		})
	}
}

func TestNoJumpOnlyBlocks(t *testing.T) {
	prog := devProgram(t, layoutProgram)
	for _, name := range []string{"sieve", "search"} {
		t.Run(name, func(t *testing.T) {
			body := functionAsm(t, prog, name)
			if found := jumpOnlyLabels(asmLines(body)); len(found) > 0 {
				t.Errorf("jump-only blocks %v:\n%s", found, body)
			}
		})
	}
}

func TestSieveInnerLoopIsRotated(t *testing.T) {
	prog := devProgram(t, layoutProgram)
	lines := asmLines(functionAsm(t, prog, "sieve"))
	inner := -1
	for i, line := range lines {
		if strings.HasPrefix(line, "add ") && i+3 < len(lines) && lines[i+1] == "TLS_POLL" {
			inner = i
		}
	}
	if inner < 0 {
		t.Fatalf("no polled latch after an add:\n%s", strings.Join(lines, "\n"))
	}
	if !strings.HasPrefix(lines[inner+2], "jne ") || !asmLabel.MatchString(lines[inner+3]) {
		t.Fatalf("poll check is not followed by a resume label:\n%s", strings.Join(lines[inner:inner+4], "\n"))
	}
	rest := lines[inner+4:]
	if len(rest) < 2 || !strings.HasPrefix(rest[0], "cmp ") || !regexp.MustCompile(`^j(l|b|ge|ae|le|g|a|be) `).MatchString(rest[1]) {
		t.Errorf("latch does not end with a compare and conditional branch:\n%s", strings.Join(lines[inner:inner+7], "\n"))
	}
}

func TestFibBaseCaseSavesNothing(t *testing.T) {
	prog := devProgram(t, layoutProgram)
	body := functionAsm(t, prog, "fib")
	base, _, found := strings.Cut(body, "\tret\n")
	if !found {
		t.Fatalf("no return in fib:\n%s", body)
	}
	for _, reg := range []string{"rbx", "r12", "r13", "r14", "r15"} {
		if strings.Contains(base, reg) {
			t.Errorf("base case touches %s:\n%s", reg, base)
		}
	}
	if saves := strings.Count(body, "], rbx") + strings.Count(body, "], r12"); saves == 0 {
		t.Errorf("recursive path does not save callee-saved registers:\n%s", body)
	}
	for _, setup := range []string{"push rbp", "mov rbp, rsp", "sub rsp", "leave"} {
		if strings.Contains(base, setup) {
			t.Errorf("base case contains %q:\n%s", setup, base)
		}
	}
}

func fixtureSource(t *testing.T, name string) string {
	t.Helper()
	return fixturetest.RunProgram(name).Source(t)
}

func benchmarkSource(t *testing.T, name string) string {
	t.Helper()
	return fixturetest.BenchmarkProgram(name).Source(t)
}

var calleeSaved = regexp.MustCompile(`\b(rbx|r12|r13|r14|r15)\b`)
var calleeSavedStore = regexp.MustCompile(`^mov qword ptr \[rbp-\d+\], (rbx|r12|r13|r14|r15)$`)
var registerSpill = regexp.MustCompile(`^mov qword ptr \[rbp-\d+\], (rax|rcx|rdx|rsi|rdi|r8|r9|r10|r11)$`)

func lightPath(t *testing.T, body string) []string {
	t.Helper()
	lines := asmLines(body)
	for i, line := range lines {
		if calleeSavedStore.MatchString(line) {
			return lines[:i]
		}
	}
	t.Fatalf("no callee-saved register is saved:\n%s", body)
	return nil
}

func TestLightPathAvoidsCalleeSavedRegisters(t *testing.T) {
	for _, tc := range []struct {
		bench, function string
		spills          bool
	}{
		{"sort", "main", false},
		{"sort", "quicksort[i64]", true},
		{"mandelbrot", "main", false},
	} {
		t.Run(tc.bench+"/"+tc.function, func(t *testing.T) {
			prog := devProgram(t, benchmarkSource(t, tc.bench))
			light := lightPath(t, functionAsm(t, prog, tc.function))
			for _, line := range light {
				if calleeSaved.MatchString(line) {
					t.Errorf("light path uses a callee-saved register: %s", line)
				}
				if !tc.spills && registerSpill.MatchString(line) {
					t.Errorf("light path spills a register: %s", line)
				}
			}
		})
	}
}

func TestSortClosureIsFrameless(t *testing.T) {
	prog := devProgram(t, benchmarkSource(t, "sort"))
	body := functionAsm(t, prog, "main..c1")
	for _, setup := range []string{"push", "rbp", "rsp", "leave", "call"} {
		if strings.Contains(body, setup) {
			t.Errorf("closure contains %q:\n%s", setup, body)
		}
	}
	if !slices.Contains(asmLines(body), "ret") {
		t.Errorf("closure has no return:\n%s", body)
	}
}

var slotLoad = regexp.MustCompile(`^mov (r10|r11), qword ptr \[rbp-\d+\]$`)

func TestQuicksortHasNoSlotToSlotCopies(t *testing.T) {
	prog := devProgram(t, benchmarkSource(t, "sort"))
	lines := asmLines(functionAsm(t, prog, "quicksort[i64]"))
	for i := 0; i+1 < len(lines); i++ {
		if !slotLoad.MatchString(lines[i]) {
			continue
		}
		scratch := slotLoad.FindStringSubmatch(lines[i])[1]
		if strings.HasPrefix(lines[i+1], "mov qword ptr [rbp-") && strings.HasSuffix(lines[i+1], ", "+scratch) {
			t.Errorf("slot to slot copy: %s ; %s", lines[i], lines[i+1])
		}
	}
}

const joinProgram = `fn helper(x: i64) -> i64 {
    x * 3 + 1
}

fn small(a: i64, k: i64) -> i64 {
    var r = a
    if k > 0 {
        r = helper(r)
    }
    r + 7
}

fn large(a: i64, b: i64, k: i64) -> i64 {
    var r = a
    if k > 0 {
        r = helper(r)
    }
    let p = r * 3 + b
    let q = (p ^ r) + (b << 2)
    let s = (q - p) * (r + 5)
    let t = (s & 0xffff) + (q | 3) - (p >> 1)
    let u = t * 7 + s - q
    let v = (u ^ t) + (u & s) + (r | b)
    v - u + t * 2
}

fn main() {
    print(small(1, 1) + large(1, 2, 1))
}
`

func TestSmallJoinIsClonedOntoTheLightPath(t *testing.T) {
	prog := devProgram(t, joinProgram)
	lines := asmLines(functionAsm(t, prog, "small"))
	if strings.HasPrefix(lines[0], "push") {
		t.Errorf("small sets up a frame at entry:\n%s", strings.Join(lines, "\n"))
	}
}

func TestLargeJoinIsNotCloned(t *testing.T) {
	prog := devProgram(t, joinProgram)
	lines := asmLines(functionAsm(t, prog, "large"))
	if lines[0] != "push rbp" {
		t.Errorf("large does not set up its frame at entry:\n%s", strings.Join(lines, "\n"))
	}
	if cloned := strings.Count(strings.Join(lines, "\n"), "c:\n"); cloned > 0 {
		t.Errorf("large contains %d cloned blocks", cloned)
	}
}

var scaledElement = regexp.MustCompile(`^mov (r\w+), qword ptr \[(r\w+)\+(r\w+)\*8\]$`)

func TestQuicksortElementAccessUsesScaledIndex(t *testing.T) {
	prog := devProgram(t, benchmarkSource(t, "sort"))
	lines := asmLines(functionAsm(t, prog, "quicksort[i64]"))
	scaled := 0
	for _, line := range lines {
		if scaledElement.MatchString(line) {
			scaled++
		}
		if strings.HasPrefix(line, "shl ") && strings.HasSuffix(line, ", 3") {
			t.Errorf("element offset computed by shift: %s", line)
		}
	}
	if scaled == 0 {
		t.Errorf("no load through a scaled index:\n%s", strings.Join(lines, "\n"))
	}
}

func TestQuicksortPartitionIndexStaysInRegister(t *testing.T) {
	prog := devProgram(t, benchmarkSource(t, "sort"))
	lines := asmLines(functionAsm(t, prog, "quicksort[i64]"))
	for i, line := range lines {
		match := scaledElement.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		index := match[3]
		window := lines[i:min(i+16, len(lines))]
		if !slices.Contains(window, fmt.Sprintf("add %s, 1", index)) && !slices.Contains(window, fmt.Sprintf("sub %s, 1", index)) {
			continue
		}
		reload := fmt.Sprintf("mov %s, qword ptr [rbp-", index)
		store := fmt.Sprintf("], %s", index)
		for _, w := range window {
			if strings.HasPrefix(w, reload) || (strings.HasPrefix(w, "mov qword ptr [rbp-") && strings.HasSuffix(w, store)) {
				t.Errorf("loop index %s is accessed through the frame: %s", index, w)
			}
		}
		return
	}
	t.Fatalf("no partition loop with a stepped register index:\n%s", strings.Join(lines, "\n"))
}

var floatWithMemory = regexp.MustCompile(`^(add|sub|mul|div)sd xmm\d+, qword ptr \[r\w+\+r\w+\*\d`)

func TestNbodyInnerLoopFoldsLoadsIntoFloatOperations(t *testing.T) {
	prog := devProgram(t, benchmarkSource(t, "nbody"))
	lines := asmLines(functionAsm(t, prog, "advance"))
	folded := 0
	for _, line := range lines {
		if floatWithMemory.MatchString(line) {
			folded++
		}
	}
	if folded < 6 {
		t.Errorf("%d float operations take a memory operand:\n%s", folded, strings.Join(lines, "\n"))
	}
}

func TestMandelbrotDoublesWithoutMultiplying(t *testing.T) {
	prog := devProgram(t, benchmarkSource(t, "mandelbrot"))
	lines := asmLines(functionAsm(t, prog, "escapes"))
	doubled := false
	for _, line := range lines {
		if strings.HasPrefix(line, "mulsd ") && strings.Contains(line, "[rip+") {
			t.Errorf("multiplication by a constant: %s", line)
		}
		operands := strings.Split(strings.TrimPrefix(line, "addsd "), ", ")
		if strings.HasPrefix(line, "addsd ") && len(operands) == 2 && operands[0] == operands[1] {
			doubled = true
		}
	}
	if !doubled {
		t.Errorf("no register added to itself:\n%s", strings.Join(lines, "\n"))
	}
}

func markingLoop(t *testing.T, lines []string) []string {
	t.Helper()
	start := slices.IndexFunc(lines, func(line string) bool { return strings.HasPrefix(line, "imul ") })
	if start < 0 {
		t.Fatalf("no square before the marking loop:\n%s", strings.Join(lines, "\n"))
	}
	labels := map[string]int{}
	for i, line := range lines {
		if asmLabel.MatchString(line) {
			labels[strings.TrimSuffix(line, ":")] = i
		}
	}
	for i := start + 1; i < len(lines); i++ {
		match := jumpTarget.FindStringSubmatch(lines[i])
		if match == nil {
			continue
		}
		if head, ok := labels[match[1]]; ok && head > start && head < i {
			return lines[head : i+1]
		}
	}
	t.Fatalf("marking loop has no back edge:\n%s", strings.Join(lines, "\n"))
	return nil
}

func TestSieveSliceLivesInRegisters(t *testing.T) {
	prog := devProgram(t, benchmarkSource(t, "sieve"))
	f := function(t, prog, "sieve")
	if len(f.Slots) != 0 {
		t.Errorf("sieve keeps frame slots %v\n%s", f.Slots, f)
	}
	lines := asmLines(functionAsm(t, prog, "sieve"))
	for _, line := range markingLoop(t, lines) {
		if strings.Contains(line, "[rbp") {
			t.Errorf("marking loop reaches the frame: %s", line)
		}
	}
	for _, line := range lines {
		if strings.Contains(line, "[rbp") && !calleeSavedStore.MatchString(line) && !strings.HasPrefix(line, "mov rbx, qword ptr [rbp-") {
			t.Errorf("frame access outside the callee-saved registers: %s", line)
		}
	}
}

func TestSieveChecksOnlyTheMarkingIndex(t *testing.T) {
	prog := devProgram(t, benchmarkSource(t, "sieve"))
	f := function(t, prog, "sieve")
	if got := countChecks(f); got != 1 {
		t.Errorf("%d checks, want 1 for the square-started marking index\n%s", got, f)
	}
	text := f.String()
	param := regexp.MustCompile(`(v\d+) = param #0`).FindStringSubmatch(text)
	if param == nil || !regexp.MustCompile(fmt.Sprintf(` = check \[v\d+ %s\]`, param[1])).MatchString(text) {
		t.Errorf("the check is not against the limit the slice was filled to\n%s", f)
	}
	lines := asmLines(functionAsm(t, prog, "sieve"))
	loop := markingLoop(t, lines)
	if !slices.ContainsFunc(loop, func(line string) bool { return strings.Contains(line, "rt_panic_index") }) {
		t.Errorf("marking loop lost the check on an index that starts at a square:\n%s", strings.Join(loop, "\n"))
	}
	if outside := strings.Count(strings.Join(lines, "\n"), "rt_panic_index") - 1; outside != 0 {
		t.Errorf("%d other references to rt_panic_index", outside)
	}
}

var jumpTarget = regexp.MustCompile(`^j[a-z]+ (\.L[A-Za-z0-9_]+)$`)

func jumpsBackOver(lines []string, at int) bool {
	labels := map[string]int{}
	for i, line := range lines {
		if asmLabel.MatchString(line) {
			labels[strings.TrimSuffix(line, ":")] = i
		}
	}
	for i := at + 1; i < len(lines); i++ {
		match := jumpTarget.FindStringSubmatch(lines[i])
		if match == nil {
			continue
		}
		if target, ok := labels[match[1]]; ok && target < at {
			return true
		}
	}
	return false
}

func TestSieveFillGrowsOnceOutsideAnyLoop(t *testing.T) {
	prog := devProgram(t, benchmarkSource(t, "sieve"))
	lines := asmLines(functionAsm(t, prog, "sieve"))
	grows := 0
	for i, line := range lines {
		if line != "call rt_grow_scan" && line != "call rt_grow_noscan" {
			continue
		}
		grows++
		if jumpsBackOver(lines, i) {
			t.Errorf("the grow routine is called inside a loop:\n%s", strings.Join(lines, "\n"))
		}
	}
	if grows == 0 {
		t.Errorf("no allocation for the filled slice:\n%s", strings.Join(lines, "\n"))
	}
}

func TestPushedSliceTakesNoFrameSlot(t *testing.T) {
	prog := devProgram(t, `fn grow_in_place(n: i64) -> i64 {
    var xs: []i64 = []
    for i in 0..n {
        push(mut xs, i)
    }
    xs.len
}

fn append_two(mut xs: []i64) {
    push(mut xs, 1)
    push(mut xs, 2)
}

fn passed_on(n: i64) -> i64 {
    var xs: []i64 = []
    append_two(mut xs)
    xs.len + n
}

fn main() {
    print(grow_in_place(5))
    print(passed_on(5))
}
`)
	if slots := function(t, prog, "grow_in_place").Slots; len(slots) != 0 {
		t.Errorf("pushing to a local keeps frame slots %v", slots)
	}
	if slots := function(t, prog, "passed_on").Slots; len(slots) != 1 {
		t.Errorf("a slice passed as mut to a function has slots %v, want one", slots)
	}
}

func hasFunction(prog *ir.Program, name string) bool {
	return slices.ContainsFunc(prog.Funcs, func(f *ir.Func) bool { return f.Name == name })
}

func TestMandelbrotReleaseInlinesEscapes(t *testing.T) {
	prog := releaseProgram(t, benchmarkSource(t, "mandelbrot"))
	if hasFunction(prog, "escapes") {
		t.Errorf("escapes survives inlining")
	}
	body := functionAsm(t, prog, "main")
	if strings.Contains(body, "call fn_escapes") {
		t.Errorf("main calls escapes:\n%s", body)
	}
	dev := devProgram(t, benchmarkSource(t, "mandelbrot"))
	if !hasFunction(dev, "escapes") {
		t.Errorf("dev build inlined escapes")
	}
}

func TestNbodyPairLoopMultipliesOnlyPerBody(t *testing.T) {
	prog := releaseProgram(t, benchmarkSource(t, "nbody"))
	multiplies := 0
	for _, line := range asmLines(functionAsm(t, prog, "advance")) {
		if strings.HasPrefix(line, "imul ") {
			multiplies++
		}
	}
	if multiplies != 1 {
		t.Errorf("advance has %d multiplies, want the one outer body offset", multiplies)
	}
}

var indirectCall = regexp.MustCompile(`^call (r(ax|bx|cx|dx|si|di|bp|8|9|1[0-5])|qword ptr)`)

func TestSortReleaseCallsComparisonDirectly(t *testing.T) {
	prog := releaseProgram(t, benchmarkSource(t, "sort"))
	for _, line := range asmLines(functionAsm(t, prog, "quicksort[i64]")) {
		if indirectCall.MatchString(line) {
			t.Errorf("quicksort calls through a function value: %s", line)
		}
	}
	if hasFunction(prog, "main..c1") {
		t.Errorf("the comparison closure is still emitted")
	}
	dev := devProgram(t, benchmarkSource(t, "sort"))
	indirect := slices.ContainsFunc(asmLines(functionAsm(t, dev, "quicksort[i64]")), indirectCall.MatchString)
	if !indirect {
		t.Errorf("dev build specialised quicksort")
	}
}

func TestSieveReleaseMarkingLoopHasNoIndexCheck(t *testing.T) {
	prog := releaseProgram(t, benchmarkSource(t, "sieve"))
	f := function(t, prog, "sieve")
	if f.ParamWords != 0 {
		t.Errorf("sieve keeps %d parameter words although every call passes the same limit", f.ParamWords)
	}
	if got := countChecks(f); got != 0 {
		t.Errorf("%d checks remain in sieve\n%s", got, f)
	}
	lines := asmLines(functionAsm(t, prog, "sieve"))
	loop := markingLoop(t, lines)
	if slices.ContainsFunc(loop, func(line string) bool { return strings.Contains(line, "rt_panic_index") }) {
		t.Errorf("marking loop still checks its index:\n%s", strings.Join(loop, "\n"))
	}
	if refs := strings.Count(strings.Join(lines, "\n"), "rt_panic_index"); refs != 0 {
		t.Errorf("%d references to rt_panic_index in sieve", refs)
	}
}

func TestSieveMarkingLoopStartsOnAThirtyTwoByteBoundary(t *testing.T) {
	prog := releaseProgram(t, benchmarkSource(t, "sieve"))
	lines := asmLines(functionAsm(t, prog, "sieve"))
	loop := markingLoop(t, lines)
	back := jumpTarget.FindStringSubmatch(loop[len(loop)-1])
	if back == nil {
		t.Fatalf("marking loop does not end in a conditional jump:\n%s", strings.Join(loop, "\n"))
	}
	head := slices.Index(lines, fmt.Sprintf("%s:", back[1]))
	if head < 1 || lines[head-1] != ".p2align 5" {
		t.Errorf("marking loop head %s is not aligned:\n%s", back[1], strings.Join(lines, "\n"))
	}
}

func TestLightEqualArmStaysInLine(t *testing.T) {
	prog := releaseProgram(t, `fn weigh(a: []i64) -> i64 {
    if a.len == 12345 {
        return weigh(a[1..])
    }
    var s = 0
    for x in a {
        if x == 0 {
            s += 1
        } else {
            s += x * 3 - 1
        }
    }
    s
}

fn main() {
    print(weigh([0, 0, 4, 0]))
}
`)
	lines := asmLines(functionAsm(t, prog, "weigh"))
	for i, line := range lines {
		target, ok := strings.CutPrefix(line, "jmp ")
		if !ok || (i > 0 && lines[i-1] == "call rt_poll") {
			continue
		}
		if at := slices.Index(lines, fmt.Sprintf("%s:", target)); at >= 0 && at < i {
			t.Errorf("%s jumps back from an out-of-line arm:\n%s", line, strings.Join(lines, "\n"))
		}
	}
}

func TestSievePrimeArmIsOutOfLine(t *testing.T) {
	prog := releaseProgram(t, benchmarkSource(t, "sieve"))
	lines := asmLines(functionAsm(t, prog, "sieve"))
	branch := slices.IndexFunc(lines, func(line string) bool { return strings.HasPrefix(line, "je ") })
	if branch < 0 {
		t.Fatalf("no branch into the prime arm:\n%s", strings.Join(lines, "\n"))
	}
	label := fmt.Sprintf("%s:", strings.TrimPrefix(lines[branch], "je "))
	arm := slices.Index(lines, label)
	latch := slices.IndexFunc(lines, func(line string) bool { return strings.HasPrefix(line, "jl ") })
	if arm < branch || latch < branch || arm < latch {
		t.Errorf("prime arm at line %d does not follow the outer back edge at line %d:\n%s", arm, latch, strings.Join(lines, "\n"))
	}
}
