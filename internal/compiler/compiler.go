package compiler

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"lowlang/internal/ast"
	"lowlang/internal/check"
	"lowlang/internal/ir"
	"lowlang/internal/lexer"
	"lowlang/internal/parser"
)

type Unit struct {
	File    *ast.File
	Program *ir.Program
}

//go:embed prelude.low
var preludeSource []byte

func Analyse(path string) (*Unit, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	prelude, err := parsePrelude()
	if err != nil {
		return nil, err
	}
	tokens, err := lexer.Lex(src)
	if err != nil {
		return nil, fmt.Errorf("%s:%w", path, err)
	}
	file, err := parser.Parse(tokens)
	if err != nil {
		return nil, fmt.Errorf("%s:%w", path, err)
	}
	file.Enums = append(prelude.Enums, file.Enums...)
	file.Structs = append(prelude.Structs, file.Structs...)
	file.Funcs = append(prelude.Funcs, file.Funcs...)
	if err := check.Check(file); err != nil {
		return nil, fmt.Errorf("%s:%w", path, err)
	}
	return &Unit{File: file, Program: ir.Build(file)}, nil
}

func parsePrelude() (*ast.File, error) {
	tokens, err := lexer.LexPrelude(preludeSource)
	if err != nil {
		return nil, fmt.Errorf("prelude.low:%w", err)
	}
	file, err := parser.ParsePrelude(tokens)
	if err != nil {
		return nil, fmt.Errorf("prelude.low:%w", err)
	}
	return file, nil
}

func Assemble(ctx context.Context, asm, object string) error {
	source := fmt.Sprintf("%s.s", object)
	if err := os.WriteFile(source, []byte(asm), 0o644); err != nil {
		return err
	}
	return tool(ctx, "as", "-o", object, source)
}

func Link(ctx context.Context, asm, output string) error {
	dir, err := os.MkdirTemp("", "low-build-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	object := filepath.Join(dir, "main.o")
	if err := Assemble(ctx, asm, object); err != nil {
		return err
	}
	return tool(ctx, "ld", "-static", "-o", output, object)
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
