package asm_test

import (
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"lowlang/internal/asm"
)

type encodingCase struct {
	source string
	hex    string
}

func assembleText(t *testing.T, source string, opts asm.Options) []byte {
	t.Helper()
	text := fmt.Sprintf(".intel_syntax noprefix\n.text\n%s\n", source)
	object, err := asm.Assemble(text, opts)
	if err != nil {
		t.Fatalf("assemble %q: %v", source, err)
	}
	for _, s := range object.Sections {
		if s.Name == ".text" {
			return s.Data
		}
	}
	t.Fatal("no .text section")
	return nil
}

func checkEncodings(t *testing.T, cases []encodingCase) {
	t.Helper()
	for _, c := range cases {
		got := hex.EncodeToString(assembleText(t, c.source, asm.Options{}))
		want := strings.ReplaceAll(c.hex, " ", "")
		if got != want {
			t.Errorf("%s: got %s, want %s", c.source, got, want)
		}
	}
}

func TestMemoryBaseEncodings(t *testing.T) {
	checkEncodings(t, []encodingCase{
		{"mov rax, [rbp]", "48 8b 45 00"},
		{"mov rax, [rbp+8]", "48 8b 45 08"},
		{"mov rax, [rbp+1000]", "48 8b 85 e8 03 00 00"},
		{"mov rax, [r13]", "49 8b 45 00"},
		{"mov rax, [r13+8]", "49 8b 45 08"},
		{"mov rax, [r13+rcx*4]", "49 8b 44 8d 00"},
		{"mov rax, [rsp]", "48 8b 04 24"},
		{"mov rax, [rsp+8]", "48 8b 44 24 08"},
		{"mov rax, [rsp+400]", "48 8b 84 24 90 01 00 00"},
		{"mov rax, [r12]", "49 8b 04 24"},
		{"mov rax, [r12+8]", "49 8b 44 24 08"},
		{"mov rax, [r12+rax*2]", "49 8b 04 44"},
		{"mov rax, [rbp+rcx]", "48 8b 44 0d 00"},
		{"mov rax, [r13+r12*8-16]", "4b 8b 44 e5 f0"},
		{"mov r12, [rsp+r13]", "4e 8b 24 2c"},
		{"lea rax, [rax+rax*8]", "48 8d 04 c0"},
		{"mov eax, [rcx*4+16]", "8b 04 8d 10 00 00 00"},
		{"movzx eax, byte ptr [r13]", "41 0f b6 45 00"},
	})
}

func TestRegisterAndImmediateEncodings(t *testing.T) {
	checkEncodings(t, []encodingCase{
		{"mov al, sil", "40 88 f0"},
		{"mov ah, bl", "88 dc"},
		{"mov dword ptr [rbp-4], 5", "c7 45 fc 05 00 00 00"},
		{"mov word ptr [rsp], 7", "66 c7 04 24 07 00"},
		{"add rax, 128", "48 05 80 00 00 00"},
		{"add rax, 127", "48 83 c0 7f"},
		{"add eax, -129", "05 7f ff ff ff"},
		{"xchg rcx, rdx", "48 87 d1"},
		{"xchg [rax], rcx", "48 87 08"},
		{"push r12", "41 54"},
		{"pop rbp", "5d"},
		{"imul rax, rbx, 300", "48 69 c3 2c 01 00 00"},
		{"shl rax, 1", "48 d1 e0"},
		{"shl r9d, cl", "41 d3 e1"},
		{"hlt", "f4"},
	})
}

func TestVexEncodings(t *testing.T) {
	checkEncodings(t, []encodingCase{
		{"vaddps xmm0, xmm1, xmm2", "c5 f0 58 c2"},
		{"vaddps ymm8, ymm1, ymm2", "c5 74 58 c2"},
		{"vaddps xmm0, xmm1, [rax]", "c5 f0 58 00"},
		{"vaddps xmm0, xmm1, [r8]", "c4 c1 70 58 00"},
		{"vaddps xmm0, xmm1, [rax+r9]", "c4 a1 70 58 04 08"},
		{"vmovdqu ymm0, [rbp-32]", "c5 fe 6f 45 e0"},
		{"vpxor xmm0, xmm0, xmm0", "c5 f9 ef c0"},
		{"vmovq xmm0, rax", "c4 e1 f9 6e c0"},
		{"vpshufb xmm1, xmm2, xmm3", "c4 e2 69 00 cb"},
		{"vzeroupper", "c5 f8 77"},
		{"vpbroadcastb ymm0, xmm1", "c4 e2 7d 78 c1"},
	})
}
