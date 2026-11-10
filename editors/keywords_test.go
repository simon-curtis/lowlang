package editors

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"lowlang/internal/lexer"
)

type grammarFile struct {
	path    string
	pattern string
	nodes   map[string]string
}

var keywordNodes = map[string]string{
	"true":  "(boolean)",
	"false": "(boolean)",
	"pub":   "(visibility)",
}

var grammarFiles = []grammarFile{
	{path: "tree-sitter-low/grammar.js", pattern: `'%s'`},
	{path: "tree-sitter-low/queries/highlights.scm", pattern: `"%s"`, nodes: keywordNodes},
	{path: "vscode/syntaxes/low.tmLanguage.json", pattern: `[(|]%s[|)]|\\\\b%s\\\\b`},
}

func keywords() []string {
	var words []string
	for kind := lexer.Fn; kind < lexer.LParen; kind++ {
		words = append(words, kind.String())
	}
	return words
}

func (g grammarFile) covers(text, word string) bool {
	if node, ok := g.nodes[word]; ok {
		return strings.Contains(text, node)
	}
	expr := strings.ReplaceAll(g.pattern, "%s", regexp.QuoteMeta(word))
	return regexp.MustCompile(expr).MatchString(text)
}

func (g grammarFile) missing(words []string) ([]string, error) {
	src, err := os.ReadFile(g.path)
	if err != nil {
		return nil, err
	}
	text := string(src)
	var absent []string
	for _, word := range words {
		if !g.covers(text, word) {
			absent = append(absent, word)
		}
	}
	return absent, nil
}

func TestGrammarsCoverEveryKeyword(t *testing.T) {
	words := keywords()
	for _, g := range grammarFiles {
		absent, err := g.missing(words)
		if err != nil {
			t.Fatal(err)
		}
		if len(absent) > 0 {
			t.Errorf("%s is missing keywords: %s", g.path, strings.Join(absent, ", "))
		}
	}
}
