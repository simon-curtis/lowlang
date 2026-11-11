package asm_test

import (
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"lowlang/internal/asm"
)

const (
	nop11 = "66662e0f1f840000000000"
	ret   = "c3"
)

func expectBytes(parts ...any) string {
	var b strings.Builder
	for _, p := range parts {
		switch v := p.(type) {
		case string:
			b.WriteString(strings.ReplaceAll(v, " ", ""))
		case int:
			b.WriteString(strings.Repeat("00", v))
		}
	}
	return b.String()
}

func repeatHex(piece string, n int) string {
	return strings.Repeat(piece, n)
}

func checkText(t *testing.T, name, source string, opts asm.Options, want string) {
	t.Helper()
	got := hex.EncodeToString(assembleText(t, source, opts))
	if got != want {
		t.Errorf("%s: got %s, want %s", name, got, want)
	}
}

func TestRelaxationBoundaries(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{"forward short at the limit", "jne c\n.zero 127\nc:\nret", expectBytes("757f", 127, ret)},
		{"forward near past the limit", "jne c\n.zero 128\nc:\nret", expectBytes("0f85 80000000", 128, ret)},
		{"backward short at the limit", "top:\n.zero 126\njmp top", expectBytes(126, "eb80")},
		{"backward near past the limit", "top:\n.zero 127\njmp top", expectBytes(127, "e9 7cffffff")},
		{"two short jumps", "jne c\n.zero 126\nc:\njne d\n.zero 126\nd:\nret", expectBytes("757e", 126, "757e", 126, ret)},
		{"backward pair", "top:\n.zero 120\njmp top\njmp top", expectBytes(120, "eb86 eb84")},
	}
	for _, c := range cases {
		checkText(t, c.name, c.source, asm.Options{}, c.want)
	}
}

func TestRelaxationChain(t *testing.T) {
	const source = "jmp far1\n.zero %d\njmp far2\nfar1:\n.zero %d\nfar2:\nret"
	fits := fmt.Sprintf(source, 124, 100)
	checkText(t, "inner jump stays short", fits, asm.Options{}, expectBytes("eb7e", 124, "eb64", 100, ret))
	grows := fmt.Sprintf(source, 124, 130)
	checkText(t, "inner growth forces the outer jump", grows, asm.Options{}, expectBytes("e9 81000000", 124, "e9 82000000", 130, ret))
}

func TestRelaxationThroughAlignment(t *testing.T) {
	const source = "jne end\n.zero %d\n.p2align 4\njmp end\n.zero 100\nend:\nret"
	six := fmt.Sprintf(source, 116)
	checkText(t, "six byte pad", six, asm.Options{}, expectBytes("0f85 e0000000", 116, "90 0f1f4400 00", "eb64", 100, ret))
	four := fmt.Sprintf(source, 118)
	checkText(t, "four byte pad", four, asm.Options{}, expectBytes("0f85 e0000000", 118, "90 0f1f00", "eb64", 100, ret))
}

func TestUnresolvedJumpsKeepRelocations(t *testing.T) {
	object, err := asm.Assemble(".intel_syntax noprefix\n.text\njne ext\njmp ext\ncall ext\nret\n", asm.Options{})
	if err != nil {
		t.Fatal(err)
	}
	text := object.Sections[0]
	want := expectBytes("0f85 00000000", "e9 00000000", "e8 00000000", ret)
	if got := hex.EncodeToString(text.Data); got != want {
		t.Errorf("got %s, want %s", got, want)
	}
	if len(text.Fixups) != 3 {
		t.Fatalf("got %d relocations, want 3", len(text.Fixups))
	}
	offsets := []int64{2, 7, 12}
	for i, f := range text.Fixups {
		if f.Offset != offsets[i] || f.Kind != asm.PLT32 || f.Addend != -4 {
			t.Errorf("relocation %d: %+v", i, f)
		}
	}
}

func TestAlignPaddingLengths(t *testing.T) {
	padding := map[int]string{
		1:  "0f1f4000" + nop11,
		2:  "0f1f00" + nop11,
		3:  "6690" + nop11,
		4:  "90" + nop11,
		5:  nop11,
		6:  "662e0f1f840000000000",
		7:  "660f1f840000000000",
		8:  "0f1f840000000000",
		9:  "0f1f8000000000",
		10: "660f1f440000",
		11: "0f1f440000",
		12: "0f1f4000",
		13: "0f1f00",
		14: "6690",
		15: "90",
	}
	for n := 1; n <= 15; n++ {
		source := fmt.Sprintf("%s.p2align 4\nret", strings.Repeat("ret\n", n))
		want := expectBytes(repeatHex(ret, n), padding[n], ret)
		checkText(t, fmt.Sprintf("%d bytes before the boundary", n), source, asm.Options{}, want)
	}
}

func TestAlignJumpsOverLongPadding(t *testing.T) {
	short := expectBytes("c3 eb7d 0f1f4000", repeatHex(nop11, 11), ret)
	checkText(t, "short jump over 127 bytes", "ret\n.p2align 7\nret", asm.Options{}, short)
	eighty := fmt.Sprintf("%s.p2align 7\nret", strings.Repeat("ret\n", 40))
	over := expectBytes(repeatHex(ret, 40), "eb56 660f1f840000000000", repeatHex(nop11, 7), ret)
	checkText(t, "jump over eight patterns", eighty, asm.Options{}, over)
	near := expectBytes("c3 e9fa010000", repeatHex(nop11, 46), ret)
	checkText(t, "near jump over 511 bytes", "ret\n.p2align 9\nret", asm.Options{}, near)
}

func TestAlignMaxSkip(t *testing.T) {
	before := strings.Repeat("ret\n", 13)
	skipped := fmt.Sprintf("%s.p2align 4,,2\nret", before)
	checkText(t, "skip exceeds the limit", skipped, asm.Options{}, expectBytes(repeatHex(ret, 14)))
	padded := fmt.Sprintf("%s.p2align 4,,3\nret", before)
	checkText(t, "skip equals the limit", padded, asm.Options{}, expectBytes(repeatHex(ret, 13), "0f1f00", ret))
}
