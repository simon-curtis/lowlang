package lsp

import (
	"slices"

	"lowlang/internal/lexer"
)

func keywords() []string {
	var words []string
	for kind := lexer.Fn; kind < lexer.LParen; kind++ {
		words = append(words, kind.String())
	}
	return words
}

func keyword(name string) bool {
	return slices.Contains(keywords(), name)
}
