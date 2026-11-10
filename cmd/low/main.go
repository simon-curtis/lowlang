package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"

	"lowlang/internal/amd64"
	"lowlang/internal/check"
	"lowlang/internal/lexer"
	"lowlang/internal/parser"
)

const usage = `usage:
  low build <file.low> [-o output]
  low run <file.low>
  low asm <file.low>`

type usageError struct{}

func (usageError) Error() string {
	return usage
}

type exitError struct {
	code int
}

func (e exitError) Error() string {
	return fmt.Sprintf("exit status %d", e.code)
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	err := run(ctx, os.Args[1:])
	stop()
	var exit exitError
	switch {
	case err == nil:
	case errors.As(err, &exit):
		os.Exit(exit.code)
	default:
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) < 2 {
		return usageError{}
	}
	cmd, path := args[0], args[1]
	switch cmd {
	case "asm":
		asm, err := compile(path)
		if err != nil {
			return err
		}
		fmt.Print(asm)
		return nil
	case "build":
		output := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		if len(args) == 4 && args[2] == "-o" {
			output = args[3]
		} else if len(args) != 2 {
			return usageError{}
		}
		return build(ctx, path, output)
	case "run":
		return buildAndRun(ctx, path)
	}
	return usageError{}
}

func compile(path string) (string, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	tokens, err := lexer.Lex(src)
	if err != nil {
		return "", fmt.Errorf("%s:%w", path, err)
	}
	file, err := parser.Parse(tokens)
	if err != nil {
		return "", fmt.Errorf("%s:%w", path, err)
	}
	if err := check.Check(file); err != nil {
		return "", fmt.Errorf("%s:%w", path, err)
	}
	return amd64.Generate(file), nil
}

func build(ctx context.Context, path, output string) error {
	asm, err := compile(path)
	if err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "low-build-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	source := filepath.Join(dir, "main.s")
	object := filepath.Join(dir, "main.o")
	if err := os.WriteFile(source, []byte(asm), 0o644); err != nil {
		return err
	}
	if err := tool(ctx, "as", "-o", object, source); err != nil {
		return err
	}
	return tool(ctx, "ld", "-static", "-o", output, object)
}

func buildAndRun(ctx context.Context, path string) error {
	dir, err := os.MkdirTemp("", "low-run-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	binary := filepath.Join(dir, "main")
	if err := build(ctx, path, binary); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, binary)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	err = cmd.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exitError{code: exit.ExitCode()}
	}
	return err
}

func tool(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s failed: %w\n%s", name, err, out)
	}
	return nil
}
