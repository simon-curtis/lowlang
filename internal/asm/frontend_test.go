package asm_test

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lowlang/internal/amd64"
	"lowlang/internal/asm"
	"lowlang/internal/oracletool"
	"lowlang/internal/target"
)

func assertMatchesAs(t *testing.T, source string) {
	t.Helper()
	oracletool.Require(t, "as", oracletool.GNU)
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

func TestTextSymbolDifference(t *testing.T) {
	assertMatchesAs(t, `
	.intel_syntax noprefix
	.text
	.globl entry
entry:
.Lt1:
	nop
	jmp .Lt3
	mov rax, 5
.Lt2:
	.p2align 4
.Lt3:
	ret
	.section .rodata
	.long 7, .Lt2 - .Lt1, .Lt3 - .Lt1
`)
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

func TestIncbinPaths(t *testing.T) {
	names := []string{
		`C:\Users\simon\blob.bin`,
		"caf\u00e9 \u00a0 \u65e5\u672c.bin",
		`say "hi".bin`,
		"tab\there.bin",
		`trailing\`,
	}
	for i, name := range names {
		t.Run(fmt.Sprintf("path%d", i), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), name)
			content := []byte{byte(i), 1, 2, 3}
			if err := os.WriteFile(path, content, 0o644); err != nil {
				t.Skipf("filesystem rejects %q: %v", name, err)
			}
			source := fmt.Sprintf("\t.section .rodata\nblob:\n\t.incbin %s\n\t.quad blob\n", amd64.QuoteString(path))
			object, err := asm.Assemble(source, asm.Options{})
			if err != nil {
				t.Fatal(err)
			}
			var got []byte
			for _, s := range object.Sections {
				if s.Name == ".rodata" {
					got = s.Data
				}
			}
			if !bytes.HasPrefix(got, content) {
				t.Errorf("included %v, want prefix %v", got, content)
			}
			assertMatchesAs(t, source)
		})
	}
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
