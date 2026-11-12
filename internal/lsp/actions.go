package lsp

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"lowlang/internal/ast"
	"lowlang/internal/check"
	"lowlang/internal/compiler"
	"lowlang/internal/diag"
	"lowlang/internal/types"
)

const defaultIndent = "    "

var snippetEscaper = strings.NewReplacer(`\`, `\\`, `$`, `\$`, `}`, `\}`)

type failure struct {
	err  *check.Unpropagated
	docs compiler.Overlay
}

func newFailure(r compiled) failure {
	f := failure{docs: r.job.docs}
	errors.As(r.err, &f.err)
	return f
}

type owner struct {
	name   string
	result *ast.TypeName
	err    *ast.TypeName
	body   *ast.Block
	inline bool
}

func (o owner) subject() string {
	if o.name == "" {
		return "the closure"
	}
	return o.name
}

type trySite struct {
	try   *ast.Try
	owner owner
}

type actionContext struct {
	uri      string
	version  int
	snippets bool
	file     *ast.File
	text     text
	site     trySite
	result   *types.Type
}

func (s *Server) codeActions(p CodeActionParams) []CodeAction {
	actions := []CodeAction{}
	path := pathOf(p.TextDocument.URI)
	file := s.parsed(path)
	if file == nil {
		return actions
	}
	t := s.text(path)
	start := p.Range.Start
	col := t.byteCol(start.Line, start.Character) + 1
	site, ok := nearestTry(trySites(file), start.Line+1, col)
	if !ok {
		return actions
	}
	result := s.unpropagated(path, site.try.Pos)
	version, open := s.versions[path]
	a := actionContext{
		uri:      p.TextDocument.URI,
		version:  version,
		snippets: s.snippetEdits && open,
		file:     file,
		text:     t,
		site:     site,
		result:   result,
	}
	if lift, ok := a.lift(); ok {
		actions = append(actions, lift)
	}
	return append(actions, a.catch())
}

func (s *Server) unpropagated(path string, pos diag.Pos) *types.Type {
	f := s.stateOf(path).failure
	if f.err == nil {
		return nil
	}
	at := f.err.Failure.Pos
	if filepath.Clean(at.File) != path || at.Line != pos.Line || at.Col != pos.Col {
		return nil
	}
	compiled, err := f.docs.ReadFile(path)
	if err != nil {
		return nil
	}
	current, err := s.docs.ReadFile(path)
	if err != nil || !bytes.Equal(compiled, current) {
		return nil
	}
	return f.err.Result
}

func trySites(file *ast.File) []trySite {
	var sites []trySite
	for _, f := range file.Funcs {
		if f.Body != nil {
			sites = collectTries(sites, owner{name: f.Name, result: f.Result, err: f.Error, body: f.Body})
		}
	}
	return sites
}

func collectTries(sites []trySite, o owner) []trySite {
	ast.Inspect(o.body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.Try:
			sites = append(sites, trySite{try: n, owner: o})
		case *ast.Closure:
			sites = collectTries(sites, owner{result: n.Result, err: n.Error, body: n.Body, inline: n.Inline})
			return false
		}
		return true
	})
	return sites
}

func nearestTry(sites []trySite, line, col int) (trySite, bool) {
	onLine := slices.DeleteFunc(sites, func(s trySite) bool { return s.try.Pos.Line != line })
	if len(onLine) == 0 {
		return trySite{}, false
	}
	slices.SortFunc(onLine, func(a, b trySite) int { return a.try.Pos.Col - b.try.Pos.Col })
	for _, s := range onLine {
		if s.try.Pos.Col >= col {
			return s, true
		}
	}
	return onLine[len(onLine)-1], true
}

func (a actionContext) lift() (CodeAction, bool) {
	if a.result == nil || a.site.owner.err != nil || a.site.owner.inline {
		return CodeAction{}, false
	}
	name, edits, ok := a.spell(a.result.Err.Name)
	if !ok {
		return CodeAction{}, false
	}
	body := a.site.owner.body.Pos
	clause := fmt.Sprintf("! %s ", name)
	if before := a.text.line(body.Line - 1)[:body.Col-1]; !strings.HasSuffix(before, " ") {
		clause = fmt.Sprintf(" %s", clause)
	}
	at := a.text.position(body)
	edits = append(edits, TextEdit{Range: Range{Start: at, End: at}, NewText: clause})
	return CodeAction{
		Title:       fmt.Sprintf("Add ! %s to %s", name, a.site.owner.subject()),
		Kind:        "quickfix",
		IsPreferred: true,
		Edit:        WorkspaceEdit{Changes: map[string][]TextEdit{a.uri: edits}},
	}, true
}

func (a actionContext) spell(qualified string) (string, []TextEdit, bool) {
	if strings.Contains(qualified, "[") {
		return "", nil, false
	}
	dot := strings.LastIndex(qualified, ".")
	if dot < 0 {
		return qualified, nil, true
	}
	module, name := qualified[:dot], qualified[dot+1:]
	if module == a.file.Module {
		return name, nil, true
	}
	for _, imp := range a.file.Imports {
		if imp.Path == module {
			return fmt.Sprintf("%s.%s", imp.Alias, name), nil, true
		}
	}
	alias := module[strings.LastIndex(module, "/")+1:]
	if _, taken := a.file.Imported(alias); taken {
		return "", nil, false
	}
	return fmt.Sprintf("%s.%s", alias, name), []TextEdit{a.importEdit(module)}, true
}

func (a actionContext) importEdit(module string) TextEdit {
	line := fmt.Sprintf("import %s\n", module)
	if n := len(a.file.Imports); n > 0 {
		at := Position{Line: a.file.Imports[n-1].Pos.Line}
		return TextEdit{Range: Range{Start: at, End: at}, NewText: line}
	}
	return TextEdit{NewText: fmt.Sprintf("%s\n", line)}
}

func (a actionContext) catch() CodeAction {
	action := CodeAction{Title: "Catch the error", Kind: "quickfix", Edit: a.catchText()}
	if a.snippets {
		action.Edit = a.catchSnippet()
	}
	return action
}

func (a actionContext) catchText() WorkspaceEdit {
	pos := a.site.try.Pos
	indent := leadingSpace(a.text.line(pos.Line - 1))
	inner := fmt.Sprintf("%s%s", indent, a.indentUnit())
	var b strings.Builder
	fmt.Fprintf(&b, " catch {\n")
	fmt.Fprintf(&b, "%se => %s\n", inner, a.armBody())
	fmt.Fprintf(&b, "%s}", indent)
	edit := TextEdit{Range: a.text.span(pos, 1), NewText: b.String()}
	return WorkspaceEdit{Changes: map[string][]TextEdit{a.uri: {edit}}}
}

func (a actionContext) catchSnippet() WorkspaceEdit {
	pos := a.site.try.Pos
	before := a.text.line(pos.Line - 1)[:pos.Col-1]
	indent := leadingSpace(before)
	var b strings.Builder
	fmt.Fprintf(&b, "%s catch {\n", snippetEscaper.Replace(before))
	fmt.Fprintf(&b, "%s%se => $0\n", indent, a.indentUnit())
	fmt.Fprintf(&b, "%s\\}", indent)
	snippet := StringValue{Kind: "snippet", Value: b.String()}
	start := diag.Pos{File: pos.File, Line: pos.Line, Col: 1}
	edit := SnippetTextEdit{Range: a.text.span(start, len(before)+1), Snippet: snippet}
	document := VersionedTextDocumentIdentifier{URI: a.uri, Version: a.version}
	return WorkspaceEdit{DocumentChanges: []TextDocumentEdit{{TextDocument: document, Edits: []SnippetTextEdit{edit}}}}
}

func (a actionContext) armBody() string {
	if a.result != nil && a.result.Ok == types.Void {
		return "{}"
	}
	if a.site.owner.result == nil {
		return "return"
	}
	return "{}"
}

func (a actionContext) indentUnit() string {
	body := a.site.owner.body.Pos
	outer := leadingSpace(a.text.line(body.Line - 1))
	for n := body.Line; n < len(a.text.lines); n++ {
		line := a.text.line(n)
		if strings.TrimSpace(line) == "" {
			continue
		}
		inner := leadingSpace(line)
		if len(inner) > len(outer) && strings.HasPrefix(inner, outer) {
			return inner[len(outer):]
		}
		return defaultIndent
	}
	return defaultIndent
}

func leadingSpace(line string) string {
	return line[:len(line)-len(strings.TrimLeft(line, " \t"))]
}
