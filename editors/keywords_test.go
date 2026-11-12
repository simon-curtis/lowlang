package editors

import (
	"encoding/json"
	"os"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	"lowlang/internal/check"
	"lowlang/internal/lexer"
	"lowlang/internal/types"
)

type listing struct {
	path     string
	label    string
	category string
	words    func(text string) []string
}

var expected = map[string][]string{}

func init() {
	expected["keywords"] = lexer.Keywords()
	for _, b := range check.Builtins() {
		switch {
		case b.Kind == check.FunctionBuiltin && b.Public():
			expected["functions"] = append(expected["functions"], b.Name)
		case b.Kind == check.TypeBuiltin:
			expected["types"] = append(expected["types"], b.Name)
		case b.Kind == check.ConstantBuiltin:
			expected["constants"] = append(expected["constants"], b.Name)
		}
	}
	expected["constraints"] = types.ConstraintNames()
}

var listings = []listing{
	{"tree-sitter-low/grammar.js", "literals", "keywords", grammarKeywords},
	{"tree-sitter-low/queries/highlights.scm", "keywords", "keywords", queryKeywords},
	{"tree-sitter-low/queries/highlights.scm", "constants", "constants", queryConstants},
	{"tree-sitter-low/queries/highlights.scm", "builtin calls", "functions", queryList(`#any-of\? @function\.builtin ([^)]*)`)},
	{"tree-sitter-low/queries/highlights.scm", "plain calls", "functions", queryList(`#not-any-of\? @function\.call ([^)]*)`)},
	{"tree-sitter-low/queries/highlights.scm", "builtin types", "types", queryPattern(`#match\? @type\.builtin "\^\(([^)]*)\)\$"`)},
	{"tree-sitter-low/queries/highlights.scm", "plain types", "types", queryPattern(`#not-match\? @type "\^\(([^)]*)\)\$"`)},
	{"vscode/syntaxes/low.tmLanguage.json", "keywords", "keywords", scopes("keyword.control.", "storage.", "constant.language.boolean.")},
	{"vscode/syntaxes/low.tmLanguage.json", "constants", "constants", scopes("constant.language.")},
	{"vscode/syntaxes/low.tmLanguage.json", "types", "types", scopes("support.type.primitive.")},
	{"vscode/syntaxes/low.tmLanguage.json", "constraints", "constraints", scopes("support.type.constraint.")},
	{"vscode/syntaxes/low.tmLanguage.json", "builtin calls", "functions", captured("support.function.builtin.")},
	{"../site/highlight.mjs", "keywords", "keywords", siteKeywords},
	{"../site/highlight.mjs", "types", "types", moduleSet("types")},
	{"../site/highlight.mjs", "constants", "constants", moduleSet("constants")},
}

func quoted(text, quote string) []string {
	var words []string
	for _, m := range regexp.MustCompile(quote+`([A-Za-z_][A-Za-z0-9_]*)`+quote).FindAllStringSubmatch(text, -1) {
		words = append(words, m[1])
	}
	return words
}

func grammarKeywords(text string) []string {
	var words []string
	for _, m := range regexp.MustCompile(`(field\(|name: )?'([a-z_]+)'`).FindAllStringSubmatch(text, -1) {
		if m[1] == "" {
			words = append(words, m[2])
		}
	}
	return words
}

func withoutPredicates(text string) string {
	return regexp.MustCompile(`\(#[^)]*\)`).ReplaceAllString(text, "")
}

var impliedKeywords = map[string][]string{"(boolean)": {"true", "false"}, "(visibility)": {"pub"}}

func queryKeywords(text string) []string {
	words := quoted(withoutPredicates(text), `"`)
	for node, implied := range impliedKeywords {
		if strings.Contains(text, node) {
			words = append(words, implied...)
		}
	}
	return words
}

func queryConstants(text string) []string {
	var words []string
	if strings.Contains(text, "(boolean)") {
		words = append(words, "true", "false")
	}
	return slices.Concat(words, queryList(`#any-of\? @constant\.builtin ([^)]*)`)(text))
}

func queryList(predicate string) func(string) []string {
	pattern := regexp.MustCompile(predicate)
	return func(text string) []string {
		var words []string
		for _, m := range pattern.FindAllStringSubmatch(text, -1) {
			words = append(words, quoted(m[1], `"`)...)
		}
		return words
	}
}

func queryPattern(predicate string) func(string) []string {
	pattern := regexp.MustCompile(predicate)
	return func(text string) []string {
		var words []string
		for _, m := range pattern.FindAllStringSubmatch(text, -1) {
			words = append(words, strings.Split(m[1], "|")...)
		}
		return words
	}
}

func alternatives(match string) []string {
	if group := regexp.MustCompile(`\(([A-Za-z0-9_|]+)\)`).FindStringSubmatch(match); group != nil {
		return strings.Split(group[1], "|")
	}
	return quoted(match, `\\b`)
}

func rules(text string) []map[string]any {
	var root any
	if err := json.Unmarshal([]byte(text), &root); err != nil {
		panic(err)
	}
	var found []map[string]any
	var walk func(node any)
	walk = func(node any) {
		switch node := node.(type) {
		case map[string]any:
			found = append(found, node)
			for _, child := range node {
				walk(child)
			}
		case []any:
			for _, child := range node {
				walk(child)
			}
		}
	}
	walk(root)
	return found
}

func scopes(prefixes ...string) func(string) []string {
	return func(text string) []string {
		var words []string
		for _, rule := range rules(text) {
			name, named := rule["name"].(string)
			match, matches := rule["match"].(string)
			if named && matches && slices.ContainsFunc(prefixes, func(p string) bool { return strings.HasPrefix(name, p) }) {
				words = append(words, alternatives(match)...)
			}
		}
		return words
	}
}

func captured(prefix string) func(string) []string {
	return func(text string) []string {
		var words []string
		for _, rule := range rules(text) {
			captures, _ := rule["captures"].(map[string]any)
			first, _ := captures["1"].(map[string]any)
			name, _ := first["name"].(string)
			match, _ := rule["match"].(string)
			if strings.HasPrefix(name, prefix) {
				words = append(words, alternatives(match)...)
			}
		}
		return words
	}
}

func moduleSet(name string) func(string) []string {
	pattern := regexp.MustCompile(`const ` + name + ` = new Set\(\[([^\]]*)\]\)`)
	return func(text string) []string {
		return quoted(pattern.FindStringSubmatch(text)[1], `"`)
	}
}

func siteKeywords(text string) []string {
	var words []string
	for _, word := range moduleSet("constants")(text) {
		if lexer.IsKeyword(word) {
			words = append(words, word)
		}
	}
	return slices.Concat(moduleSet("keywords")(text), words)
}

func difference(have, want []string) []string {
	var out []string
	for _, word := range have {
		if !slices.Contains(want, word) && !slices.Contains(out, word) {
			out = append(out, word)
		}
	}
	sort.Strings(out)
	return out
}

func TestGrammarsMatchLanguageTables(t *testing.T) {
	for _, l := range listings {
		src, err := os.ReadFile(l.path)
		if err != nil {
			t.Fatal(err)
		}
		found := l.words(string(src))
		want := expected[l.category]
		if missing := difference(want, found); len(missing) > 0 {
			t.Errorf("%s %s lacks %s: %s", l.path, l.label, l.category, strings.Join(missing, ", "))
		}
		if extra := difference(found, want); len(extra) > 0 {
			t.Errorf("%s %s lists %s the language does not have: %s", l.path, l.label, l.category, strings.Join(extra, ", "))
		}
	}
}
