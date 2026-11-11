package asm_test

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"lowlang/internal/asm"
	"lowlang/internal/target"
)

func assertMatchesAs(t *testing.T, source string) {
	t.Helper()
	if _, err := exec.LookPath("as"); err != nil {
		t.Skip("GNU as is not on PATH")
	}
	object, err := asm.Assemble(source, asm.Options{})
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	dir := t.TempDir()
	oracle, err := runAs(dir, source, nil)
	if err != nil {
		t.Fatalf("oracle: %v", err)
	}
	c := newComparison(object, oracle, target.Linux)
	c.run()
	for _, m := range c.mismatches {
		t.Error(m)
	}
}

func TestConstantExpressions(t *testing.T) {
	assertMatchesAs(t, `
	.intel_syntax noprefix
	.set A, -5
	.set B, (1 << 4) | 3
	.set C, A * 3 + B
	.set D, 0x10 - 0b101 + 010
	.set E, D
	.set D, 7
	.section .rodata
	.p2align 3
v:
	.quad A, B, C, D, E, ~0, 100 / 7, 100 % 7, -A
	.quad 1 + 2 * 3, 2 * 3 + 1, 1 << 2 + 1, 6 | 1 + 1, 8 >> 1 - 1
	.long B >> 2, 7 & 3, 6 ^ 3, 2 | 8
	.short -1, 0xffff, 65535
	.byte 255, -128, 0x7f
	.quad 0xFFFFFFFFFFFFFFFF, -9223372036854775808, 9223372036854775807
	.long 4294967295, -2147483648
`)
}

func TestSymbolDifferences(t *testing.T) {
	assertMatchesAs(t, `
	.intel_syntax noprefix
	.section .rodata
	.p2align 2
.La:
	.long 1, 2, 3
.Lb:
	.long .Lb - .La, .Lend - .La, .Lc - .Lb + 4, .Lc - .La - 4
.Lc:
	.quad .La, .Lc + 16, ext + 8, .Lend
	.long ext2
.Lend:
	.text
.Lt1:
	nop
.Lt2:
	.section .data
	.long .Lt2 - .Lt1
`)
}

func TestUnresolvedTextDifference(t *testing.T) {
	object, err := asm.Assemble(`
	.intel_syntax noprefix
	.text
.Lt1:
	nop
.Lt2:
	.section .rodata
	.long 7, .Lt2 - .Lt1
`, asm.Options{})
	if err != nil {
		t.Fatal(err)
	}
	var rodata *asm.Section
	for _, s := range object.Sections {
		if s.Name == ".rodata" {
			rodata = s
		}
	}
	if rodata == nil || len(rodata.Unresolved) != 1 || rodata.Unresolved[0] != (asm.Span{Offset: 4, Size: 4}) {
		t.Fatalf("unresolved spans: %+v", rodata)
	}
}

func TestMacrosConditionalsAndLocalLabels(t *testing.T) {
	assertMatchesAs(t, `
	.intel_syntax noprefix
	.set MODE, 1
	.macro PAIR a, b
.Lpair_\@:
	.quad \a + \b, .Lpair_\@
	.endm
	.macro OUTER x
	PAIR \x, \x
	.endm
	.section .rodata
	PAIR 1, 2
	OUTER 5
.if MODE
	.quad 11
.if MODE - 1
	.quad 12
.else
	.quad 13
.endif
.else
	.quad 14
.if MODE
	.quad 15
.endif
.endif
1:
	.quad 1b, 2f
2:
	.quad 1b, 1f
1:
	.quad 1b
`)
}

func TestStringsFloatsAndFill(t *testing.T) {
	assertMatchesAs(t, `
	.intel_syntax noprefix
	.section .rodata
	.ascii "a\n\t\\\"\101\x41\r\b\f", "second"
	.asciz "zero # ; terminated"
	.p2align 3
	.double 1.5, -2.25e10, 3.0e-5, 0.1, 123456789.123456789, 1e308, 4.9e-324, 0.0, -0.0
	.space 5, 0xAA
	.zero 3
	.p2align 4
	.zero 2 * 8
	.data
	.p2align 2
	.byte 1
	.p2align 3
	.quad 9
	.bss
	.p2align 5
bss_a:
	.zero 10
	.p2align 3
bss_b:
	.zero 4
	.section .tbss,"awT",@nobits
	.p2align 3
tls_a:
	.zero 24
	.section .rodata
	.globl exported
exported:
	.quad exported, bss_b, tls_a
`)
}

func TestIncbin(t *testing.T) {
	path := filepath.Join(t.TempDir(), "blob.bin")
	if err := os.WriteFile(path, []byte{1, 2, 3, 4, 5}, 0o644); err != nil {
		t.Fatal(err)
	}
	assertMatchesAs(t, `
	.intel_syntax noprefix
	.section .rodata
	.p2align 4
blob:
	.incbin "`+path+`"
	.quad blob
`)
}

func TestIncbinMissingFile(t *testing.T) {
	_, err := asm.Assemble("\t.section .rodata\n\n\t.incbin \"/nonexistent/blob.bin\"\n", asm.Options{})
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected a wrapped not-exist error, got %v", err)
	}
	message := err.Error()
	if !strings.Contains(message, "line 3") {
		t.Fatalf("error does not name the line: %v", err)
	}
}

func TestErrors(t *testing.T) {
	cases := map[string]string{
		"\t.bogus 1\n":                         "unsupported directive",
		"\t.if 1\n":                            "missing .endif",
		"\t.macro M\n":                         "missing .endm",
		"\t.quad 1b\n":                         "no definition",
		"a:\na:\n":                             "already defined",
		"\t.section .bss\n\t.quad 1\n":         "nobits",
		"\tmov rax, [rax+rbx+rcx]\n":           "too many registers",
		"\t.quad 1 +\n":                        "unexpected",
		"\t.ascii \"open\n":                    "unterminated",
		"\t.set X, sym\n":                      "constant",
		"\t.section .data\n\tmov rax, rbx\n":   "non-executable",
		"\t.quad 5 / 0\n":                      "division by zero",
		"\t.intel_syntax prefix\n":             "noprefix",
		"\t.section .data\n\t.quad a, b - c\n": "undefined symbol",
	}
	for source, want := range cases {
		_, err := asm.Assemble(source, asm.Options{})
		if err == nil || !strings.Contains(fmt.Sprint(err), want) {
			t.Errorf("%q: error %v, want mention of %q", source, err, want)
		}
	}
}

func TestInstructionForms(t *testing.T) {
	object, err := asm.Assemble(`
	.intel_syntax noprefix
	.set M, 16
	.text
	lock cmpxchg qword ptr [rbx+M+8], rcx
	lea rax, [rip+sym+8]
	mov rax, qword ptr fs:[-24]
	mov eax, [rax+rcx*8-16]
	mov eax, [8*rcx+rdx]
	call qword ptr [rip+free@GOTPCREL]
	vmovdqu ymm0, ymmword ptr [rsi]
	rep stosq
	imul rax, rcx, 5*M
	jz 1f
1:
	call target_fn
	movzx eax, byte ptr [rdi+rsi]
	mov ah, [rsp]
`, asm.Options{})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"lock cmpxchg m64[r64+const], r64",
		"lea r64, m0[rip+sym+const]",
		"mov r64, fs:m64[const]",
		"mov r32, m0[r64+r64*8+const]",
		"mov r32, m0[r64+r64*8]",
		"call m64[rip+sym@GOTPCREL]",
		"vmovdqu ymm, m256[r64]",
		"rep stosq",
		"imul r64, r64, const",
		"jz sym",
		"call sym",
		"movzx r32, m8[r64+r64*1]",
		"mov r8h, m0[r64]",
	}
	var got []string
	for _, item := range object.Sections[0].Items {
		if item.Kind == asm.ItemInst {
			shape := instShape(item.Inst)
			got = append(got, shape)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("got %d instructions, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("instruction %d: %q, want %q", i, got[i], want[i])
		}
	}
	first := object.Sections[0].Items[0].Inst.Operands[0]
	if first.Base.String() != "rbx" || first.Value.Add != 24 || first.Size != 8 {
		t.Errorf("memory operand decoded as %+v", first)
	}
}
