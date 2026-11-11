package asm_test

import (
	"fmt"
	"strings"
	"testing"

	"lowlang/internal/asm"
)

func nops(n int) string {
	return strings.Repeat("nop\n", n)
}

func nopBytes(n int) string {
	return strings.Repeat("90", n)
}

func TestAlignBranchesPadding(t *testing.T) {
	boundary := asm.Options{AlignBranches: true}
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			"jcc crossing the boundary is preceded by a nop",
			nops(30) + "jne t\nt:\nret",
			expectBytes(nopBytes(30), "6690 7500", ret),
		},
		{
			"jmp crossing the boundary",
			nops(30) + "jmp t\nt:\nret",
			expectBytes(nopBytes(30), "6690 eb00", ret),
		},
		{
			"near jcc crossing the boundary",
			nops(30) + "jne t\n.zero 130\nt:\nret",
			expectBytes(nopBytes(30), "6690 0f85 82000000", 130, ret),
		},
		{
			"a label before the branch stays before the padding",
			nops(30) + "l:\njne l\nret",
			expectBytes(nopBytes(30), "6690 75fc", ret),
		},
		{
			"call is not padded",
			nops(30) + "call t\nt:\nret",
			expectBytes(nopBytes(30), "e800000000", ret),
		},
		{
			"indirect jmp is not padded",
			nops(30) + "jmp rax\nret",
			expectBytes(nopBytes(30), "ffe0", ret),
		},
		{
			"cmp fused with jne is padded together",
			nops(30) + "cmp rax, rbx\njne t\nt:\nret",
			expectBytes(nopBytes(30), "6690 4839d8 7500", ret),
		},
		{
			"inc fused with jne",
			nops(30) + "inc rax\njne t\nt:\nret",
			expectBytes(nopBytes(30), "6690 48ffc0 7500", ret),
		},
		{
			"test fused with jle",
			nops(30) + "test rax, rax\njle t\nt:\nret",
			expectBytes(nopBytes(30), "6690 4885c0 7e00", ret),
		},
		{
			"alignment between the pair breaks the fusion",
			nops(30) + "and eax, 1\n.p2align 3\njne t\nt:\nret",
			expectBytes(nopBytes(30), "83e001 0f1f8000000000 7500", ret),
		},
		{
			"cs prefixes widen earlier instructions",
			strings.Repeat("mov rax, rbx\n", 9) + nops(3) + "jne t\nt:\nret",
			expectBytes("2e2e 4889d8", repeatHex("4889d8", 8), nopBytes(3), "7500", ret),
		},
		{
			"an existing segment prefix is repeated",
			strings.Repeat("mov rax, fs:[rbx]\n", 7) + nops(2) + "jne t\nt:\nret",
			expectBytes("6464 64488b03", repeatHex("64488b03", 6), nopBytes(2), "7500", ret),
		},
	}
	for _, c := range cases {
		checkText(t, c.name, c.source, boundary, c.want)
	}
}

func TestAlignBranchesNoPaddingWithoutFlag(t *testing.T) {
	source := nops(30) + "jne t\nt:\nret"
	checkText(t, "no flag", source, asm.Options{}, expectBytes(nopBytes(30), "7500", ret))
}

func TestAlignBranchesRaisesSectionAlignment(t *testing.T) {
	text := fmt.Sprintf(".intel_syntax noprefix\n.text\n%s", "jne t\nt:\nret\n")
	object, err := asm.Assemble(text, asm.Options{AlignBranches: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := object.Sections[0].Align; got != 32 {
		t.Errorf("section alignment %d, want 32", got)
	}
	plain, err := asm.Assemble(text, asm.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got := plain.Sections[0].Align; got != 1 {
		t.Errorf("section alignment %d, want 1", got)
	}
}
