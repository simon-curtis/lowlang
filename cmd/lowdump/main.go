package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

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

func dumpTokens(out io.Writer, tokens []lexer.Token, depth int) {
	indent := strings.Repeat(" ", depth)
	for _, t := range tokens {
		fmt.Fprintf(out, "%s%d:%d %d-%d %s", indent, t.Pos.Line, t.Pos.Col, t.Start, t.End, t.Kind)
		if t.Text != "" {
			fmt.Fprintf(out, " text=%s", lexer.Quote(t.Text))
		}
		if t.Kind == lexer.Char {
			fmt.Fprintf(out, " char=%d", t.Char)
		}
		if t.DocAt.Line != 0 {
			fmt.Fprintf(out, " doc=%d:%d %s", t.DocAt.Line, t.DocAt.Col, lexer.Quote(t.Doc))
		}
		fmt.Fprintln(out)
		for _, part := range t.Parts {
			if part.Tokens == nil {
				fmt.Fprintf(out, "%s  part text=%s\n", indent, lexer.Quote(part.Text))
				continue
			}
			fmt.Fprintf(out, "%s  part expr", indent)
			if part.SpecPos.Line != 0 {
				fmt.Fprintf(out, " spec=%s at %d:%d", lexer.Quote(part.Spec), part.SpecPos.Line, part.SpecPos.Col)
			}
			fmt.Fprintln(out)
			dumpTokens(out, part.Tokens, depth+4)
		}
	}
}
