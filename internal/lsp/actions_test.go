package lsp

import (
	"encoding/json"
	"testing"

	"lowlang/internal/compiler"
)

const openFile = `import std/file

fn main() {
  file.open("data.txt")?
}
`

const parseTwice = `enum ParseError {
    Empty
}

fn parse(s: str) -> i64 ! ParseError {
    if s.len == 0 { fail Empty }
    1
}

fn twice(s: str) -> i64 {
    parse(s)? * 2
}

fn main() ! ParseError {
    print(twice("4")?)
}
`

func (c *client) codeActions(f fixture, pos Position) map[string]CodeAction {
	c.t.Helper()
	params := CodeActionParams{TextDocument: TextDocumentIdentifier{URI: f.uri}, Range: Range{Start: pos, End: pos}}
	raw := c.request("textDocument/codeAction", params)
	var list []CodeAction
	if err := json.Unmarshal(raw, &list); err != nil {
		c.t.Fatal(err)
	}
	actions := map[string]CodeAction{}
	for _, a := range list {
		actions[a.Title] = a
	}
	return actions
}

func applied(t *testing.T, f fixture, action CodeAction) string {
	t.Helper()
	return string(applyEdit(t, action.Edit)[f.path])
}

func TestLiftImportsTheErrorModule(t *testing.T) {
	c := start(t)
	f := scratch(t, "main.low", openFile)
	c.open(f, f.src)
	actions := c.codeActions(f, f.at(t, "file.open", 0))
	lift, ok := actions["Add ! io.Error to main"]
	if !ok {
		t.Fatalf("no lift action among %v", actions)
	}
	want := `import std/file
import std/io

fn main() ! io.Error {
  file.open("data.txt")?
}
`
	got := applied(t, f, lift)
	if got != want {
		t.Fatalf("lift produced\n%s\nwant\n%s", got, want)
	}
	compiles(t, f.path, compiler.Overlay{f.path: []byte(got)})
}

func TestCatchWrapsTheTry(t *testing.T) {
	c := start(t)
	f := scratch(t, "main.low", openFile)
	c.open(f, f.src)
	catch, ok := c.codeActions(f, f.at(t, "?", 0))["Catch the error"]
	if !ok {
		t.Fatal("no catch action")
	}
	want := `import std/file

fn main() {
  file.open("data.txt") catch {
    e => return
  }
}
`
	got := applied(t, f, catch)
	if got != want {
		t.Fatalf("catch produced\n%s\nwant\n%s", got, want)
	}
	compiles(t, f.path, compiler.Overlay{f.path: []byte(got)})
}

func TestCatchPlacesTheCursorInTheArm(t *testing.T) {
	snippets := ClientCapabilities{Workspace: WorkspaceClient{WorkspaceEdit: WorkspaceEditClient{SnippetEditSupport: true}}}
	c := startWith(t, InitializeParams{Capabilities: snippets})
	f := scratch(t, "main.low", openFile)
	c.open(f, f.src)
	before := c.publishes[f.uri]
	change := DidChangeParams{TextDocument: VersionedTextDocumentIdentifier{URI: f.uri, Version: 7}, ContentChanges: []ContentChange{{Text: f.src}}}
	c.notify("textDocument/didChange", change)
	c.await(f.uri, before)
	catch, ok := c.codeActions(f, f.at(t, "?", 0))["Catch the error"]
	if !ok {
		t.Fatal("no catch action")
	}
	changes := catch.Edit.DocumentChanges
	if len(changes) != 1 || changes[0].TextDocument.Version != 7 || len(changes[0].Edits) != 1 {
		t.Fatalf("want one snippet edit at version 7, got %+v", catch.Edit)
	}
	edit := changes[0].Edits[0]
	if want := "  file.open(\"data.txt\") catch {\n    e => $0\n  \\}"; edit.Snippet.Value != want {
		t.Fatalf("snippet %q, want %q", edit.Snippet.Value, want)
	}
	try := f.at(t, "?", 1)
	if want := (Range{Start: Position{Line: try.Line}, End: try}); edit.Range != want {
		t.Fatalf("snippet replaces %+v, want %+v", edit.Range, want)
	}
}

func TestLiftAfterResultType(t *testing.T) {
	c := start(t)
	f := scratch(t, "main.low", parseTwice)
	c.open(f, f.src)
	lift, ok := c.codeActions(f, f.at(t, "parse(s)?", 0))["Add ! ParseError to twice"]
	if !ok {
		t.Fatal("no lift action")
	}
	got := applied(t, f, lift)
	compiles(t, f.path, compiler.Overlay{f.path: []byte(got)})
}

func TestLiftNeedsAnUndeclaredErrorType(t *testing.T) {
	c := start(t)
	f := scratch(t, "main.low", parseTwice)
	c.open(f, f.src)
	actions := c.codeActions(f, f.at(t, `twice("4")?`, 0))
	if len(actions) != 1 {
		t.Fatalf("want only the catch action, got %v", actions)
	}
	if _, ok := actions["Catch the error"]; !ok {
		t.Fatalf("want the catch action, got %v", actions)
	}
}

func TestNoActionsAwayFromTry(t *testing.T) {
	c := start(t)
	f := scratch(t, "main.low", openFile)
	c.open(f, f.src)
	if actions := c.codeActions(f, f.at(t, "fn main", 0)); len(actions) != 0 {
		t.Fatalf("want no actions, got %v", actions)
	}
}
