package oracletool

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	branchFlag     = "-mbranches-within-32B-boundaries"
	noUsedNoteFlag = "-mx86-used-note=no"
	stackNote      = "\n.section .note.GNU-stack,\"\",@progbits\n"
	clangTarget    = "--target=x86_64-pc-windows-msvc"
	errorLimit     = 600
)

type GNUOptions struct {
	AlignBranches bool
	Linkable      bool
}

func AssembleGNU(dir, source string, options GNUOptions) (string, error) {
	text := source
	var args []string
	if options.AlignBranches {
		args = append(args, branchFlag)
	}
	if options.Linkable {
		text = fmt.Sprintf("%s%s", source, stackNote)
		args = append(args, noUsedNoteFlag)
	}
	sourcePath := filepath.Join(dir, "main.s")
	if err := os.WriteFile(sourcePath, []byte(text), 0o644); err != nil {
		return "", fmt.Errorf("write source: %w", err)
	}
	defer os.Remove(sourcePath)
	object := filepath.Join(dir, "main.o")
	args = append(args, "-o", object, sourcePath)
	if out, err := exec.Command("as", args...).CombinedOutput(); err != nil {
		return "", fmt.Errorf("as: %w\n%s", err, truncate(string(out)))
	}
	return object, nil
}

func AssembleClang(dir, source string) (string, error) {
	sourcePath := filepath.Join(dir, "main.s")
	if err := os.WriteFile(sourcePath, []byte(source), 0o644); err != nil {
		return "", fmt.Errorf("write source: %w", err)
	}
	defer os.Remove(sourcePath)
	object := filepath.Join(dir, "main.obj")
	if out, err := exec.Command("clang", clangTarget, "-c", "-o", object, sourcePath).CombinedOutput(); err != nil {
		return "", fmt.Errorf("clang: %w\n%s", err, truncate(string(out)))
	}
	return object, nil
}

func truncate(text string) string {
	if len(text) <= errorLimit {
		return text
	}
	return text[:errorLimit]
}
