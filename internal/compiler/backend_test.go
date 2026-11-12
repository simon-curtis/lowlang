package compiler

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"lowlang/internal/amd64"
	"lowlang/internal/ir"
)

const backendProgram = `fn first_unused(xs: []i64) -> i64 {
    let unused = xs[0]
    xs.len
}

fn add32(a: u32, b: u32) -> u32 {
    a + b
}

fn mix32(a: u32, b: u32) -> u32 {
    (a * b + 7) << 3
}

fn widen(a: u32, b: u32) -> u64 {
    u64(a) + u64(b)
}

fn rotl32(x: u32) -> u32 {
    (x << 7) | (x >> 25)
}

fn rotr64(x: u64) -> u64 {
    (x >> 19) | (x << 45)
}

fn shifted_pair(x: u32) -> u32 {
    (x << 7) | (x >> 24)
}

fn is_get(s: str) -> bool {
    s == "GET"
}

fn is_empty(s: str) -> bool {
    "" == s
}

fn is_long(s: str) -> bool {
    s == "application/json2"
}

fn same(a: str, b: str) -> bool {
    a == b
}

fn collect(n: i64, offset: i64) -> i64 {
    var xs: []i64 = []
    var total = 0
    for i in 0..n {
        push(mut xs, total + offset + i)
        total += xs[i % xs.len]
    }
    total + xs.len
}

fn main() {
    var xs: []i64 = [1, 2, 3]
    print(first_unused(xs))
    print(i64(add32(4000000000, 500000000)))
    print(i64(mix32(123456, 7891)))
    print(widen(4000000000, 4000000000))
    print(i64(rotl32(2147483649)))
    print(i64(rotr64(1)))
    print(i64(shifted_pair(1)))
    print(is_get("GET"))
    print(is_empty(""))
    print(is_long("application/json2"))
    print(same("a", "b"))
    print(collect(10, 4))
}
`

func backendAsm(t *testing.T, name string) []string {
	t.Helper()
	prog := devProgram(t, backendProgram)
	return asmLines(functionAsm(t, prog, name))
}

func countPrefix(lines []string, prefix string) int {
	count := 0
	for _, line := range lines {
		if strings.HasPrefix(line, prefix) {
			count++
		}
	}
	return count
}

func TestUnusedLoadIsRemoved(t *testing.T) {
	prog := devProgram(t, backendProgram)
	for _, b := range function(t, prog, "first_unused").Blocks {
		for _, v := range b.Values {
			if v.Op == ir.OpLoad {
				t.Errorf("load of an unused element survives in %s", b)
			}
		}
	}
}

var dwordArithmetic = regexp.MustCompile(`^(add|sub|imul|shl|lea) (e[a-z]{2}|r\d+d),`)
var selfExtend = regexp.MustCompile(`^mov (e[a-z]{2}|r\d+d), (e[a-z]{2}|r\d+d)$`)

func TestUnsigned32BitArithmeticUsesDwordForms(t *testing.T) {
	for _, name := range []string{"add32", "mix32"} {
		lines := backendAsm(t, name)
		dword := 0
		for _, line := range lines {
			if selfExtend.MatchString(line) {
				t.Errorf("%s moves between 32-bit registers: %s", name, line)
			}
			if dwordArithmetic.MatchString(line) {
				dword++
			}
		}
		if dword == 0 {
			t.Errorf("%s has no 32-bit arithmetic:\n%s", name, strings.Join(lines, "\n"))
		}
	}
}

func TestUnsigned32BitAddUsesNoAddressSizePrefix(t *testing.T) {
	lines := backendAsm(t, "add32")
	if !slices.Contains(lines, "lea eax, [rdi+rsi]") {
		t.Errorf("add32 is not one lea with 64-bit address registers:\n%s", strings.Join(lines, "\n"))
	}
}

func TestWideningConversionNeedsNoNormalisation(t *testing.T) {
	lines := backendAsm(t, "widen")
	for _, line := range lines {
		if dwordArithmetic.MatchString(line) || selfExtend.MatchString(line) {
			t.Errorf("widening a u32 touches a 32-bit register: %s", line)
		}
	}
}

func TestShiftPairBecomesRotate(t *testing.T) {
	for _, test := range []struct {
		name  string
		width string
	}{{"rotl32", "rol e"}, {"rotr64", "rol r"}} {
		lines := backendAsm(t, test.name)
		if countPrefix(lines, test.width) != 1 {
			t.Errorf("%s does not use one %q:\n%s", test.name, test.width, strings.Join(lines, "\n"))
		}
		if countPrefix(lines, "shr ")+countPrefix(lines, "shl ") != 0 {
			t.Errorf("%s keeps its shifts:\n%s", test.name, strings.Join(lines, "\n"))
		}
	}
}

func TestShiftPairThatDoesNotCoverTheWidthIsNotARotate(t *testing.T) {
	lines := backendAsm(t, "shifted_pair")
	if countPrefix(lines, "rol ") != 0 {
		t.Errorf("shifts 7 and 24 of a 32-bit word are not a rotate:\n%s", strings.Join(lines, "\n"))
	}
}

func TestLiteralComparisonIsInline(t *testing.T) {
	for name, want := range map[string]string{
		"is_get":   "cmp word ptr [rdi], 0x4547",
		"is_empty": "cmp rsi, 0",
	} {
		lines := backendAsm(t, name)
		if !slices.Contains(lines, want) {
			t.Errorf("%s lacks %q:\n%s", name, want, strings.Join(lines, "\n"))
		}
	}
	for _, name := range []string{"is_get", "is_empty"} {
		if slices.Contains(backendAsm(t, name), "call rt_str_eq") {
			t.Errorf("%s calls rt_str_eq", name)
		}
	}
	if !slices.Contains(backendAsm(t, "is_long"), "call rt_str_eq") {
		t.Errorf("a literal longer than %d bytes is compared inline", amd64.MaxInlineLiteral)
	}
}

func TestStringComparisonChecksLengthsBeforeCalling(t *testing.T) {
	lines := backendAsm(t, "same")
	call := -1
	for i, line := range lines {
		if line == "call rt_str_eq" {
			call = i
		}
	}
	if call < 3 || lines[call-2] != "cmp rsi, rcx" || !strings.HasPrefix(lines[call-1], "jne ") {
		t.Errorf("the call is not guarded by a length comparison:\n%s", strings.Join(lines, "\n"))
	}
}

var growCall = regexp.MustCompile(`^call rt_grow_(scan|noscan)$`)

func TestGrowKeepsScalarsInCallerSavedRegisters(t *testing.T) {
	lines := backendAsm(t, "collect")
	grows := 0
	for i, line := range lines {
		if !growCall.MatchString(line) {
			continue
		}
		grows++
		for _, around := range lines[max(0, i-12):i] {
			if registerSpill.MatchString(around) {
				t.Errorf("a caller-saved register is spilled around the grow call: %s\n%s", around, strings.Join(lines, "\n"))
			}
		}
	}
	if grows != 1 {
		t.Errorf("%d grow calls:\n%s", grows, strings.Join(lines, "\n"))
	}
}

func TestCompressionLoopKeepsFewFrameOperands(t *testing.T) {
	prog := releaseProgram(t, fixtureSource(t, "ra_sha256_pressure"))
	lines := asmLines(functionAsm(t, prog, "digest"))
	frame := 0
	for _, line := range lines {
		if strings.Contains(line, "[rbp-") {
			frame++
		}
	}
	if frame > 120 {
		t.Errorf("%d instructions address the frame in digest:\n%s", frame, strings.Join(lines, "\n"))
	}
}
