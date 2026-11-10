package lsp

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

func (c *client) hints(f fixture) []InlayHint {
	c.t.Helper()
	whole := Range{End: Position{Line: strings.Count(f.src, "\n") + 1}}
	params := InlayHintParams{TextDocument: TextDocumentIdentifier{URI: f.uri}, Range: whole}
	raw := c.request("textDocument/inlayHint", params)
	var hints []InlayHint
	if err := json.Unmarshal(raw, &hints); err != nil {
		c.t.Fatal(err)
	}
	return hints
}

func hintAfter(f fixture, hints []InlayHint, line string) (string, bool) {
	lines := strings.Split(f.src, "\n")
	for _, h := range hints {
		if lines[h.Position.Line] == line {
			return h.Label, true
		}
	}
	return "", false
}

func TestInlayHints(t *testing.T) {
	c := start(t)
	f := load(t, "structs.low")
	c.open(f, f.src)
	hints := c.hints(f)
	cases := []struct {
		line  string
		label string
	}{
		{"    let p = Point { x: 3, y: -4 }", ": Point"},
		{"    let q = add(p, Point { x: 1, y: 1 })", ": Point"},
		{"    var r = Rect {", ": Rect"},
	}
	for _, tc := range cases {
		got, ok := hintAfter(f, hints, tc.line)
		if !ok || got != tc.label {
			t.Errorf("hint on %q = %q, want %q", tc.line, got, tc.label)
		}
	}
	for _, h := range hints {
		line := strings.Split(f.src, "\n")[h.Position.Line]
		name := strings.Fields(line)[1]
		after := strings.Index(line, fmt.Sprintf(" %s", name)) + 1 + len(name)
		if h.Position.Character != after {
			t.Errorf("hint %q not placed after %s in %q", h.Label, name, line)
		}
	}
	c.change(f, f.src+"\nfn broken() { let x: i64 = \"s\" }\n")
	if stale := c.hints(f); len(stale) != 0 {
		t.Errorf("hints while the program does not compile = %+v", stale)
	}
}

func TestInlayHintRefresh(t *testing.T) {
	refreshing := InitializeParams{Capabilities: ClientCapabilities{Workspace: WorkspaceClient{InlayHint: InlayHintClient{RefreshSupport: true}}}}
	c := startWith(t, refreshing)
	f := load(t, "structs.low")
	c.open(f, f.src)
	c.symbols(f)
	if c.asked["workspace/inlayHint/refresh"] == 0 {
		t.Error("no refresh after a successful compile")
	}
	quiet := start(t)
	quiet.open(f, f.src)
	quiet.symbols(f)
	if len(quiet.asked) != 0 {
		t.Errorf("server requests to a client without refresh support: %v", quiet.asked)
	}
}

func TestInlayHintsSkipWrittenTypes(t *testing.T) {
	c := start(t)
	f := scratch(t, "typed.low", "fn main() {\n    let a: i64 = 1\n    let b = 2\n    for i in 0..3 {\n        print(i + a + b)\n    }\n}\n")
	c.open(f, f.src)
	hints := c.hints(f)
	if _, ok := hintAfter(f, hints, "    let a: i64 = 1"); ok {
		t.Error("hint on a let with a written type")
	}
	if got, _ := hintAfter(f, hints, "    let b = 2"); got != ": i64" {
		t.Errorf("hint on b = %q", got)
	}
	if got, _ := hintAfter(f, hints, "    for i in 0..3 {"); got != ": i64" {
		t.Errorf("hint on loop variable = %q", got)
	}
}

func TestWorkspaceSymbols(t *testing.T) {
	c := start(t)
	f := load(t, "modules", "main.low")
	c.open(f, f.src)
	raw := c.request("workspace/symbol", WorkspaceSymbolParams{Query: "sh"})
	var symbols []SymbolInformation
	if err := json.Unmarshal(raw, &symbols); err != nil {
		t.Fatal(err)
	}
	found := map[string]SymbolInformation{}
	for _, s := range symbols {
		found[s.Name] = s
	}
	for _, name := range []string{"shift", "Shape", "shout"} {
		if _, ok := found[name]; !ok {
			t.Errorf("missing %s in %+v", name, symbols)
		}
	}
	if _, ok := found["circle"]; ok {
		t.Error("circle does not match sh")
	}
	shift := found["shift"]
	src, err := os.ReadFile(pathOf(shift.Location.URI))
	if err != nil {
		t.Fatal(err)
	}
	line := strings.Split(string(src), "\n")[shift.Location.Range.Start.Line]
	if !strings.Contains(line, "fn shift") || shift.ContainerName != "geometry" {
		t.Errorf("shift at %q in %s", line, shift.ContainerName)
	}
}
