package main

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"lowlang/internal/diag"
	"lowlang/internal/lexer"
	"lowlang/internal/parser"
)

const usage = `usage:
  lowdump tokens [-privileged] <file.low>
  lowdump ast [-privileged] <file.low>
  lowdump check [-windows] <file.low|dir>
  lowdump ir [-windows] [-unoptimised] <file.low|dir>
  lowdump asm [-windows] <file.low|dir>
  lowdump object [-clang] [-align-branches] <file.s>
  lowdump link [-windows] [-embed name=file]... [-lib name]... [-libdir dir]... <file.s> <output>
  lowdump patch [-embed name=file]... <file.s> <base> <running.txt>
  lowdump regex <patterns.txt>`

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
	flags := args[1 : len(args)-1]
	switch args[0] {
	case "link":
		return dumpLinkCommand(args[1:])
	case "patch":
		return dumpPatchCommand(out, args[1:])
	case "check":
		return dumpCheckCommand(out, path, platformOf(flags))
	case "ir":
		return dumpIRCommand(out, path, flags)
	case "asm":
		return dumpAsmCommand(out, path, flags)
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
	case "object":
		return dumpObjectCommand(out, src, args[1:len(args)-1])
	case "regex":
		dumpRegexCommand(out, src)
		return nil
	}
	return fmt.Errorf("%s", usage)
}
