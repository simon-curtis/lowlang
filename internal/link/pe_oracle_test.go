package link_test

import (
	"bytes"
	"debug/pe"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"lowlang/internal/asm"
	"lowlang/internal/link"
	"lowlang/internal/target"
)

const (
	clangTarget  = "--target=x86_64-pc-windows-msvc"
	llvmReadobj  = "llvm-readobj"
	readobjFlags = "--file-headers --sections --coff-imports --coff-basereloc"
)

var lldArguments = []string{"/nologo", "/brepro", "/nodefaultlib", "/entry:start", "/subsystem:console", "/stack:8388608"}

func windowsCorpusPaths(t *testing.T) []string {
	matches, err := filepath.Glob(filepath.Join(repoRoot(), "testdata", "windows", "*.low"))
	if err != nil {
		t.Fatalf("glob windows corpus: %v", err)
	}
	var paths []string
	for _, match := range matches {
		rel, err := filepath.Rel(repoRoot(), match)
		if err != nil {
			t.Fatalf("relative path of %s: %v", match, err)
		}
		paths = append(paths, rel)
	}
	paths = append(paths, corpusPaths(t)...)
	sort.Strings(paths)
	return paths
}

func requireWindowsTools(t *testing.T) {
	for _, tool := range []string{"clang", "lld-link"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is not on PATH", tool)
		}
	}
}

func linkWithLLD(dir, source string) ([]byte, error) {
	sourcePath := filepath.Join(dir, "main.s")
	if err := os.WriteFile(sourcePath, []byte(source), 0o644); err != nil {
		return nil, fmt.Errorf("write source: %w", err)
	}
	object := filepath.Join(dir, "main.obj")
	if out, err := exec.Command("clang", clangTarget, "-c", "-o", object, sourcePath).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("clang: %w\n%s", err, out)
	}
	output := filepath.Join(dir, "lld.exe")
	arguments := append(append([]string{}, lldArguments...), fmt.Sprintf("/out:%s", output), object)
	if out, err := exec.Command("lld-link", arguments...).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("lld-link: %w\n%s", err, out)
	}
	return os.ReadFile(output)
}

func linkWithPE(source string) ([]byte, error) {
	object, err := asm.Assemble(source, asm.Options{ClangLayout: true})
	if err != nil {
		return nil, fmt.Errorf("assemble: %w", err)
	}
	return link.PE(object)
}

func describeWithReadobj(t *testing.T, dir, name string, image []byte) string {
	if _, err := exec.LookPath(llvmReadobj); err != nil {
		return ""
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, image, 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	arguments := append(strings.Fields(readobjFlags), path)
	out, err := exec.Command(llvmReadobj, arguments...).CombinedOutput()
	if err != nil {
		return fmt.Sprintf("%s failed: %v\n%s", llvmReadobj, err, out)
	}
	return strings.ReplaceAll(string(out), path, name)
}

func describeSectionDifference(got, want []byte) string {
	gotFile, err := pe.NewFile(bytes.NewReader(got))
	if err != nil {
		return fmt.Sprintf("parse linked image: %v", err)
	}
	wantFile, err := pe.NewFile(bytes.NewReader(want))
	if err != nil {
		return fmt.Sprintf("parse lld image: %v", err)
	}
	if len(gotFile.Sections) != len(wantFile.Sections) {
		return fmt.Sprintf("%d sections, want %d", len(gotFile.Sections), len(wantFile.Sections))
	}
	for i, wantSection := range wantFile.Sections {
		gotSection := gotFile.Sections[i]
		if gotSection.SectionHeader != wantSection.SectionHeader {
			return fmt.Sprintf("section %d header %+v, want %+v", i, gotSection.SectionHeader, wantSection.SectionHeader)
		}
		gotData, _ := gotSection.Data()
		wantData, _ := wantSection.Data()
		if !bytes.Equal(gotData, wantData) {
			return fmt.Sprintf("section %s differs at offset %d", wantSection.Name, firstDifference(gotData, wantData))
		}
	}
	return "sections are identical"
}

func firstDifferingLine(got, want string) string {
	gotLines, wantLines := strings.Split(got, "\n"), strings.Split(want, "\n")
	for i := range min(len(gotLines), len(wantLines)) {
		if gotLines[i] != wantLines[i] {
			return fmt.Sprintf("line %d: got %q, want %q", i+1, gotLines[i], wantLines[i])
		}
	}
	return fmt.Sprintf("line counts differ: got %d, want %d", len(gotLines), len(wantLines))
}

func TestPEMatchesLLD(t *testing.T) {
	requireWindowsTools(t)
	for _, path := range windowsCorpusPaths(t) {
		t.Run(path, func(t *testing.T) {
			source, _, err := generate(path, target.Windows)
			if err != nil {
				t.Skipf("compile: %v", err)
			}
			dir := t.TempDir()
			want, lldErr := linkWithLLD(dir, source)
			got, peErr := linkWithPE(source)
			if lldErr != nil {
				if peErr == nil {
					t.Fatalf("lld-link rejects the program but the PE writer accepts it: %v", lldErr)
				}
				return
			}
			if peErr != nil {
				t.Fatal(peErr)
			}
			if bytes.Equal(got, want) {
				return
			}
			t.Errorf("image differs from lld-link at byte %d (got %d bytes, want %d): %s", firstDifference(got, want), len(got), len(want), describeSectionDifference(got, want))
			gotText := describeWithReadobj(t, dir, "pe.exe", got)
			wantText := describeWithReadobj(t, dir, "pe.exe", want)
			if gotText != wantText {
				t.Errorf("%s: %s", llvmReadobj, firstDifferingLine(gotText, wantText))
			}
		})
	}
}
