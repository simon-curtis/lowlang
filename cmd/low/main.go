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
	"runtime/debug"
	"strings"

	"lowlang/internal/compiler"
	"lowlang/internal/hot"
	"lowlang/internal/lsp"
	"lowlang/internal/target"
)

var usage = fmt.Sprintf(`usage:
  low build <file.low> [-o output] [-target %[1]s]
  low run <file.low>
  low asm <file.low> [-target %[1]s]
  low ir <file.low> [-target %[1]s]
  low lint <file.low> [-target %[1]s]
  low doc <dir|std> [-target %[1]s]
  low names <file.low> [-target %[1]s]
  low dev <file.low>
  low lsp
  low version`, target.Choices())

var version string

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
	if len(args) == 1 {
		switch args[0] {
		case "lsp":
			server := lsp.NewServer(os.Stdin, os.Stdout)
			return server.Run(ctx)
		case "version":
			fmt.Println(buildVersion())
			return nil
		}
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
	case "lint", "doc", "names":
		opts, err := parseOptions(args[2:], false)
		if err != nil {
			return err
		}
		switch cmd {
		case "lint":
			return lintCommand(path, opts.platform)
		case "names":
			return namesCommand(path, opts.platform)
		}
		return docCommand(path, opts.platform)
	case "run", "dev":
		if len(args) != 2 {
			return usageError{}
		}
		if cmd == "run" {
			return buildAndRun(ctx, path)
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

func buildVersion() string {
	if version != "" {
		return version
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}
	settings := make(map[string]string, len(info.Settings))
	for _, setting := range info.Settings {
		settings[setting.Key] = setting.Value
	}
	revision := settings["vcs.revision"]
	if revision == "" {
		return "dev"
	}
	revision = revision[:min(len(revision), 12)]
	if settings["vcs.modified"] == "true" {
		return fmt.Sprintf("dev+%s.dirty", revision)
	}
	return fmt.Sprintf("dev+%s", revision)
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

func lintCommand(path string, platform target.OS) error {
	unit, err := compiler.Check(path, platform, compiler.Overlay(nil))
	if err != nil {
		return err
	}
	report(unit)
	if len(unit.Warnings) > 0 {
		return exitError{code: 1}
	}
	return nil
}

func docCommand(dir string, platform target.OS) error {
	modules, err := compiler.Document(dir, platform)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(modules)
}

func namesCommand(path string, platform target.OS) error {
	names, err := lsp.Names(path, platform)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(names)
}

func report(unit *compiler.Unit) {
	for _, w := range unit.Warnings {
		fmt.Fprintln(os.Stderr, w)
	}
}

func analyse(path string, platform target.OS) (*compiler.Unit, error) {
	unit, err := compiler.Analyse(path, compiler.Release(platform))
	if err != nil {
		return nil, err
	}
	report(unit)
	return unit, nil
}

func compileCommand(ctx context.Context, cmd, path string, opts options) error {
	unit, err := analyse(path, opts.platform)
	if err != nil {
		return err
	}
	switch cmd {
	case "ir":
		for _, f := range unit.Program.Funcs {
			fmt.Print(f)
		}
		return nil
	case "asm":
		out, err := compiler.Generate(unit)
		if err != nil {
			return err
		}
		fmt.Print(out.Asm)
		return nil
	}
	output := opts.output
	if output == "" {
		output = defaultOutput(path, opts.platform)
	}
	_, err = compiler.Build(ctx, unit, output)
	return err
}

func defaultOutput(path string, platform target.OS) string {
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return platform.Executable(name)
}

func build(ctx context.Context, path, output string, platform target.OS) error {
	unit, err := analyse(path, platform)
	if err != nil {
		return err
	}
	_, err = compiler.Build(ctx, unit, output)
	return err
}

func buildAndRun(ctx context.Context, path string) error {
	dir, err := os.MkdirTemp("", "low-run-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	binary := filepath.Join(dir, target.Host().Executable("main"))
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
