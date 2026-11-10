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
			typedLines := strings.Split(typed, "\n")
			inserted := slices.Insert(lines, i+1, typedLines...)
			last := len(typedLines) - 1
			at := Position{Line: i + 1 + last, Character: len(typedLines[last])}
			return typing{text: strings.Join(inserted, "\n"), at: at}
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

func TestCompletionAfterCallsAndIndexing(t *testing.T) {
	c := start(t)
	f := load(t, "structs.low")
	c.open(f, f.src)
	for _, typed := range []string{
		"    origin().",
		"    corner(r).",
		"    add(origin(), Point { x: 1, y: (2) }).",
		"    r.max.",
		"    corner(r).x",
	} {
		items := c.completeTyping(f, "print(r.label)", typed)
		expect(t, items, []string{"x", "y"}, []string{"label", "print"})
	}
	shapes := scratch(t, "points.low", "struct Point {\n    x: i64\n    y: i64\n}\n\nenum Problem {\n    Bad\n}\n\nfn find(n: i64) -> Point ! Problem {\n    if n < 0 {\n        fail Bad\n    }\n    Point { x: n, y: n }\n}\n\nfn main() ! Problem {\n    let pts = [Point { x: 1, y: 2 }]\n    let pick = fn(i: i64) -> Point { pts[i] }\n    print(pts.len)\n}\n")
	c.open(shapes, shapes.src)
	for _, typed := range []string{"    pts[0].", "    find(1)?.", "    pick(0)."} {
		items := c.completeTyping(shapes, "print(pts.len)", typed)
		expect(t, items, []string{"x", "y"}, []string{"print"})
	}
	if items := c.completeTyping(shapes, "print(pts.len)", "    find(1)."); len(items) != 0 {
		t.Errorf("fields offered on an unhandled result: %+v", items)
	}
}

func TestCompletionAfterModuleCall(t *testing.T) {
	c := start(t)
	f := load(t, "modules", "main.low")
	c.open(f, f.src)
	items := c.completeTyping(f, "fn main() {", "    geometry.shift(p).")
	expect(t, items, []string{"x", "y"}, []string{"circle"})
	private := c.completeTyping(f, "fn main() {", "    geometry.circle(2).")
	expect(t, private, nil, []string{"radius"})
}

func TestCompiledLine(t *testing.T) {
	compiled := newText([]byte("a\nb\nc\nd"))
	cases := []struct {
		now  string
		line int
		want int
	}{
		{"a\nb\nc\nd", 2, 2},
		{"a\nx\nb\nc\nd", 0, 0},
		{"a\nx\nb\nc\nd", 1, 0},
		{"a\nx\nb\nc\nd", 3, 2},
		{"a\nx\ny\nz\nd", 2, 0},
		{"a\nx\ny\nz\nd", 4, 3},
		{"a\nd", 1, 3},
		{"x\nb\nc\nd", 0, -1},
	}
	for _, tc := range cases {
		c := &cursor{text: newText([]byte(tc.now)), compiled: compiled, line: tc.line}
		if got := c.compiledLine(); got != tc.want {
			t.Errorf("%q line %d -> %d, want %d", tc.now, tc.line, got, tc.want)
		}
	}
}

func TestCompletionScope(t *testing.T) {
	c := start(t)
	f := load(t, "structs.low")
	c.open(f, f.src)
	items := c.completeTyping(f, "print(r.label)", "    sn")
	expect(t, items, []string{"p", "q", "r", "manhattan", "Point", "Shape", "Box", "print", "i64", "match", "none"}, []string{"n", "a", "b", "snapshot", "m"})
	later := c.completeTyping(f, "let snapshot = r", "    sn")
	expect(t, later, []string{"r", "snapshot"}, []string{"m"})
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
	expect(t, items, []string{"geometry", "t", "helper", "main"}, []string{"circle", "offset", "c", "q"})
	later := c.completeTyping(f, "q.x += 10", "    ")
	expect(t, later, []string{"c", "s", "p", "q"}, []string{"l", "m", "typed"})
}

func TestCompletionWithoutCompile(t *testing.T) {
	c := start(t)
	f := load(t, "structs.low")
	c.open(f, "fn main() {\n    let x = \n}\n")
	items := c.complete(f, Position{Line: 1, Character: 12})
	expect(t, items, []string{"print", "let", "i64"}, nil)
}
