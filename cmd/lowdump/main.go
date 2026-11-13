package main

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"lowlang/internal/diag"
	"lowlang/internal/lexer"
)

const usage = `usage:
  lowdump tokens [-privileged] <file.low>`

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
	}
	return fmt.Errorf("%s", usage)
}
