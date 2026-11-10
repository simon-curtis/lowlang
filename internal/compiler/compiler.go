package compiler

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"lowlang/internal/ast"
	"lowlang/internal/check"
	"lowlang/internal/ir"
	"lowlang/internal/lexer"
	"lowlang/internal/parser"
)

type Unit struct {
	Files   []*ast.File
	Program *ir.Program
	Sources []string
}

//go:embed prelude.low
var preludeSource []byte

func Analyse(path string) (*Unit, error) {
	prelude, err := parsePrelude()
	if err != nil {
		return nil, err
	}
	modules, err := load(path)
	if err != nil {
		return nil, err
	}
	files := append([]*ast.File{prelude}, modules.files...)
	embeds, err := check.Check(files)
	if err != nil {
		return nil, err
	}
	return &Unit{Files: files, Program: ir.Build(files), Sources: append(modules.sources, embeds...)}, nil
}

func parsePrelude() (*ast.File, error) {
	tokens, err := lexer.LexPrelude(preludeSource)
	if err != nil {
		return nil, err
	}
	return parser.ParsePrelude(tokens)
}

func Assemble(ctx context.Context, asm, object string) error {
	source := fmt.Sprintf("%s.s", object)
	if err := os.WriteFile(source, []byte(asm), 0o644); err != nil {
		return err
	}
	return tool(ctx, "as", "-o", object, source)
}

func Link(ctx context.Context, asm, output string, libraries []string) error {
	dir, err := os.MkdirTemp("", "low-build-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	object := filepath.Join(dir, "main.o")
	if err := Assemble(ctx, asm, object); err != nil {
		return err
	}
	if len(libraries) == 0 {
		return tool(ctx, "ld", "-static", "-o", output, object)
	}
	args, err := dynamicArgs(output, object, libraries)
	if err != nil {
		return err
	}
	return tool(ctx, "ld", args...)
}

const dynamicLinker = "/lib64/ld-linux-x86-64.so.2"

var startFileDirs = []string{"/usr/lib", "/usr/lib64", "/usr/lib/x86_64-linux-gnu", "/lib/x86_64-linux-gnu"}

func startFiles() (string, error) {
	for _, dir := range startFileDirs {
		if _, err := os.Stat(filepath.Join(dir, "crt1.o")); err == nil {
			return dir, nil
		}
	}
	return "", fmt.Errorf("cannot find the C start files (crt1.o) in %s; install the C library development files", strings.Join(startFileDirs, ", "))
}

func dynamicArgs(output, object string, libraries []string) ([]string, error) {
	dir, err := startFiles()
	if err != nil {
		return nil, err
	}
	args := []string{
		"-o", output, "-dynamic-linker", dynamicLinker, "-L", dir,
		filepath.Join(dir, "crt1.o"), filepath.Join(dir, "crti.o"), object,
	}
	for _, lib := range libraries {
		if lib != "c" {
			args = append(args, fmt.Sprintf("-l%s", lib))
		}
	}
	return append(args, "-lc", filepath.Join(dir, "crtn.o")), nil
}

func LinkAt(ctx context.Context, asm, output, against string, address uint64) error {
	object := fmt.Sprintf("%s.o", output)
	if err := Assemble(ctx, asm, object); err != nil {
		return err
	}
	script := fmt.Sprintf("%s.ld", output)
	layout := fmt.Sprintf("SECTIONS { . = %#x; .hot : { *(.text) *(.rodata) } }\n", address)
	if err := os.WriteFile(script, []byte(layout), 0o644); err != nil {
		return err
	}
	return tool(ctx, "ld", "-e", "0", "-T", script, "-R", against, "-o", output, object)
}

func tool(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s failed: %w\n%s", name, err, out)
	}
	return nil
}
