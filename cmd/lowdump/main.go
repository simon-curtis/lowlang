package main

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"lowlang/internal/diag"
	"lowlang/internal/lexer"
	"lowlang/internal/parser"
	"lowlang/internal/target"
)

const usage = `usage:
  lowdump tokens [-privileged] <file.low>
  lowdump ast [-privileged] <file.low>
  lowdump check [-windows] <file.low|dir>`

func main() {
	out := bufio.NewWriter(os.Stdout)
	err := run(out, os.Args[1:])
	out.Flush()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(out io.Writer, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("%s", usage)
	}
	privileged := args[1] == "-privileged"
	path := args[len(args)-1]
	if args[0] == "check" {
		platform := target.Linux
		if args[1] == "-windows" {
			platform = target.Windows
		}
		return dumpCheckCommand(out, path, platform)
	}
	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	switch args[0] {
	case "tokens":
		tokens, err := lexer.Lex(src, diag.Start(path), privileged)
		if err != nil {
			return err
		}
		dumpTokens(out, tokens, 0)
		return nil
	case "ast":
		f, err := parser.ParseSource(src, diag.Start(path), privileged)
		if err != nil {
			return err
		}
		astDumper{out}.file(f)
		return nil
	}
	return fmt.Errorf("%s", usage)
}
