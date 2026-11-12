package asm_test

import (
	"bytes"
	"debug/pe"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"lowlang/internal/asm"
)

const clangTarget = "--target=x86_64-pc-windows-msvc"

func clangText(t *testing.T, source string) []byte {
	t.Helper()
	dir := t.TempDir()
	input := filepath.Join(dir, "in.s")
	if err := os.WriteFile(input, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, "out.obj")
	if out, err := exec.Command("clang", clangTarget, "-c", "-o", output, input).CombinedOutput(); err != nil {
		t.Fatalf("clang: %v\n%s", err, out)
	}
	file, err := pe.Open(output)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	data, err := file.Section(".text").Data()
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func clangLayoutText(t *testing.T, source string) []byte {
	t.Helper()
	object, err := asm.Assemble(source, asm.Options{ClangLayout: true})
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	return object.Sections[0].Data
}

func requireClang(t *testing.T) {
	if _, err := exec.LookPath("clang"); err != nil {
		t.Skip("clang is not on PATH")
	}
}

func TestClangLayoutAlignmentPadding(t *testing.T) {
	requireClang(t)
	for _, power := range []int{3, 4, 5, 6} {
		for filler := 0; filler < 70; filler++ {
			source := fmt.Sprintf("\t.intel_syntax noprefix\n\t.text\n\t.zero %d\n\t.p2align %d\n\tret\n", filler, power)
			got, want := clangLayoutText(t, source), clangText(t, source)
			if !bytes.Equal(got, want) {
				t.Errorf("p2align %d after %d bytes: got % x, want % x", power, filler, got, want)
			}
		}
	}
}

func TestClangLayoutRelaxation(t *testing.T) {
	requireClang(t)
	for before := 100; before <= 124; before += 2 {
		for gap := 0; gap < 4; gap++ {
			source := fmt.Sprintf(`	.intel_syntax noprefix
	.text
top:
	jne far
	.zero %d
	jne far
	.p2align 5
	.zero %d
	jne top
	.zero %d
far:
	ret
`, before, 20+gap*3, 24+before/4)
			got, want := clangLayoutText(t, source), clangText(t, source)
			if !bytes.Equal(got, want) {
				t.Errorf("before %d gap %d: got % x, want % x", before, gap, got, want)
			}
		}
	}
}
