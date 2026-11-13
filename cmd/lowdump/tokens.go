package main

import (
	"fmt"
	"io"
	"strings"

	"lowlang/internal/lexer"
)

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
