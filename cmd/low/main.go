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
	"lowlang/internal/compiler"
	"lowlang/internal/hot"
	"lowlang/internal/lsp"
)

const usage = `usage:
  low build <file.low> [-o output]
  low run <file.low>
  low asm <file.low>
  low ir <file.low>
  low dev <file.low>
  low lsp`

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
	if len(args) == 1 && args[0] == "lsp" {
		server := lsp.NewServer(os.Stdin, os.Stdout)
		return server.Run(ctx)
	}
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
	case "ir":
		unit, err := compiler.Analyse(path)
		if err != nil {
			return err
		}
		for _, f := range unit.Program.Funcs {
			fmt.Print(f)
		}
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
	case "dev":
		session, err := hot.New(path, os.Stdout, os.Stderr)
		if err != nil {
			return err
		}
		defer session.Close()
		return session.Run(ctx)
	}
	return usageError{}
}

func compile(path string) (string, error) {
	unit, err := compiler.Analyse(path)
	if err != nil {
		return "", err
	}
	return amd64.Generate(unit.Program, nil).Asm, nil
}

func build(ctx context.Context, path, output string) error {
	unit, err := compiler.Analyse(path)
	if err != nil {
		return err
	}
	asm := amd64.Generate(unit.Program, nil).Asm
	return compiler.Link(ctx, asm, output, unit.Program.Libraries)
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
