package main

import (
	"context"
	"encoding/json"
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
	"lowlang/internal/target"
)

const usage = `usage:
  low build <file.low> [-o output] [-target linux/amd64|windows/amd64]
  low run <file.low>
  low asm <file.low> [-target linux/amd64|windows/amd64]
  low ir <file.low> [-target linux/amd64|windows/amd64]
  low lint <file.low>
  low doc <dir|std>
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
	case "asm", "ir", "build":
		opts, err := parseOptions(args[2:], cmd == "build")
		if err != nil {
			return err
		}
		return compileCommand(ctx, cmd, path, opts)
	case "lint":
		if len(args) != 2 {
			return usageError{}
		}
		return lintCommand(path)
	case "doc":
		if len(args) != 2 {
			return usageError{}
		}
		return docCommand(path)
	case "run":
		if len(args) != 2 {
			return usageError{}
		}
		return buildAndRun(ctx, path)
	case "dev":
		if len(args) != 2 {
			return usageError{}
		}
		session, err := hot.New(path, os.Stdout, os.Stderr)
		if err != nil {
			return err
		}
		defer session.Close()
		return session.Run(ctx)
	}
	return usageError{}
}

type options struct {
	output   string
	platform target.OS
}

func parseOptions(args []string, output bool) (options, error) {
	opts := options{platform: target.Host()}
	for len(args) > 0 {
		if len(args) < 2 {
			return opts, usageError{}
		}
		switch {
		case args[0] == "-o" && output:
			opts.output = args[1]
		case args[0] == "-target":
			platform, err := target.Parse(args[1])
			if err != nil {
				return opts, err
			}
			opts.platform = platform
		default:
			return opts, usageError{}
		}
		args = args[2:]
	}
	return opts, nil
}

func lintCommand(path string) error {
	unit, err := compiler.Check(path, os.ReadFile)
	if err != nil {
		return err
	}
	report(unit)
	if len(unit.Warnings) > 0 {
		return exitError{code: 1}
	}
	return nil
}

func docCommand(target string) error {
	modules, err := compiler.Document(target)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(modules)
}

func report(unit *compiler.Unit) {
	for _, w := range unit.Warnings {
		fmt.Fprintln(os.Stderr, w)
	}
}

func compileCommand(ctx context.Context, cmd, path string, opts options) error {
	unit, err := compiler.Analyse(path, opts.platform)
	if err != nil {
		return err
	}
	report(unit)
	switch cmd {
	case "ir":
		for _, f := range unit.Program.Funcs {
			fmt.Print(f)
		}
		return nil
	case "asm":
		fmt.Print(amd64.Generate(unit.Program, nil).Asm)
		return nil
	}
	output := opts.output
	if output == "" {
		output = defaultOutput(path, opts.platform)
	}
	return link(ctx, unit, output)
}

func defaultOutput(path string, platform target.OS) string {
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if platform == target.Windows {
		return fmt.Sprintf("%s.exe", name)
	}
	return name
}

func link(ctx context.Context, unit *compiler.Unit, output string) error {
	asm := amd64.Generate(unit.Program, nil).Asm
	return compiler.Link(ctx, asm, output, unit.Program)
}

func build(ctx context.Context, path, output string, platform target.OS) error {
	unit, err := compiler.Analyse(path, platform)
	if err != nil {
		return err
	}
	report(unit)
	return link(ctx, unit, output)
}

func buildAndRun(ctx context.Context, path string) error {
	dir, err := os.MkdirTemp("", "low-run-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	binary := filepath.Join(dir, defaultOutput("main", target.Host()))
	if err := build(ctx, path, binary, target.Host()); err != nil {
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
