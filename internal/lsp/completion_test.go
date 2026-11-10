package lsp

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

type typing struct {
	text string
	at   Position
}

func (f fixture) typeAfter(t *testing.T, needle, typed string) typing {
	t.Helper()
	lines := strings.Split(f.src, "\n")
	for i, line := range lines {
		if strings.Contains(line, needle) {
			inserted := slices.Insert(lines, i+1, typed)
			return typing{text: strings.Join(inserted, "\n"), at: Position{Line: i + 1, Character: len(typed)}}
		}
	}
	t.Fatalf("%q not found in %s", needle, f.path)
	return typing{}
}

func (c *client) complete(f fixture, pos Position) map[string]CompletionItem {
	c.t.Helper()
	params := TextDocumentPositionParams{TextDocument: TextDocumentIdentifier{URI: f.uri}, Position: pos}
	raw := c.request("textDocument/completion", params)
	var list CompletionList
	if err := json.Unmarshal(raw, &list); err != nil {
		c.t.Fatal(err)
	}
	items := map[string]CompletionItem{}
	for _, item := range list.Items {
		items[item.Label] = item
	}
	return items
}

func (c *client) completeTyping(f fixture, needle, typed string) map[string]CompletionItem {
	c.t.Helper()
	edit := f.typeAfter(c.t, needle, typed)
	c.change(f, edit.text)
	return c.complete(f, edit.at)
}

func expect(t *testing.T, items map[string]CompletionItem, present, absent []string) {
	t.Helper()
	for _, name := range present {
		if _, ok := items[name]; !ok {
			t.Errorf("missing %s", name)
		}
	}
	for _, name := range absent {
		if _, ok := items[name]; ok {
			t.Errorf("unexpected %s", name)
		}
	}
}

func TestCompletionModuleMembers(t *testing.T) {
	c := start(t)
	f := load(t, "modules", "main.low")
	c.open(f, f.src)
	items := c.completeTyping(f, "fn main() {", "    geometry.")
	expect(t, items, []string{"circle", "shift", "Point", "Shape", "Circle"}, []string{"offset", "main", "print", "fn"})
	if got := items["circle"].Detail; got != "fn circle(r: i64) -> geometry.Circle" {
		t.Errorf("circle detail = %q", got)
	}
	variants := c.completeTyping(f, "fn main() {", "    geometry.Shape.")
	expect(t, variants, []string{"Square", "Dot"}, []string{"circle"})
}

func TestCompletionFields(t *testing.T) {
	c := start(t)
	f := load(t, "structs.low")
	c.open(f, f.src)
	direct := c.completeTyping(f, "print(r.label)", "    r.")
	expect(t, direct, []string{"min", "max", "label"}, []string{"x", "print"})
	if got := direct["label"].Detail; got != "str" {
		t.Errorf("label detail = %q", got)
	}
	chained := c.completeTyping(f, "print(r.label)", "    r.max.")
	expect(t, chained, []string{"x", "y"}, []string{"min"})
	partial := c.completeTyping(f, "print(r.label)", "    r.max.x")
	expect(t, partial, []string{"x", "y"}, nil)
}

func TestCompletionScope(t *testing.T) {
	c := start(t)
	f := load(t, "structs.low")
	c.open(f, f.src)
	items := c.completeTyping(f, "print(r.label)", "    sn")
	expect(t, items, []string{"p", "q", "r", "snapshot", "manhattan", "Point", "Shape", "Box", "print", "i64", "match", "none"}, []string{"n", "a", "b"})
	if got := items["r"].Detail; got != "Rect" {
		t.Errorf("r detail = %q", got)
	}
	if got := items["Box"].Detail; got != "Shape.Box(Rect)" {
		t.Errorf("Box detail = %q", got)
	}
}

func TestCompletionImportsAndHelpers(t *testing.T) {
	c := start(t)
	f := load(t, "modules", "main.low")
	c.open(f, f.src)
	items := c.completeTyping(f, "fn main() {", "    ")
	expect(t, items, []string{"geometry", "t", "helper", "main", "c", "q"}, []string{"circle", "offset"})
}

func TestCompletionWithoutCompile(t *testing.T) {
	c := start(t)
	f := load(t, "structs.low")
	c.open(f, "fn main() {\n    let x = \n}\n")
	items := c.complete(f, Position{Line: 1, Character: 12})
	expect(t, items, []string{"print", "let", "i64"}, nil)
}
