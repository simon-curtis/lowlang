package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type incoming struct {
	ID     *int            `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
	Result json.RawMessage `json:"result"`
	Error  *rpcError       `json:"error"`
}

type outgoing struct {
	JSONRPC string `json:"jsonrpc"`
	ID      *int   `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

type client struct {
	t           *testing.T
	to          *stream
	messages    chan incoming
	diagnostics map[string][]Diagnostic
	publishes   map[string]int
	asked       map[string]int
	next        int
	done        chan error
}

func start(t *testing.T) *client {
	t.Helper()
	return startWith(t, InitializeParams{})
}

func startWith(t *testing.T, init InitializeParams) *client {
	t.Helper()
	serverIn, clientOut := io.Pipe()
	clientIn, serverOut := io.Pipe()
	server := NewServer(serverIn, serverOut)
	c := &client{
		t:           t,
		to:          &stream{out: clientOut},
		messages:    make(chan incoming, 64),
		diagnostics: map[string][]Diagnostic{},
		publishes:   map[string]int{},
		asked:       map[string]int{},
		done:        make(chan error, 1),
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		c.done <- server.Run(ctx)
		serverOut.Close()
	}()
	go c.receive(clientIn)
	t.Cleanup(func() {
		cancel()
		clientOut.Close()
	})
	c.request("initialize", init)
	c.notify("initialized", map[string]any{})
	return c
}

func (c *client) receive(r io.Reader) {
	defer close(c.messages)
	s := newStream(r, io.Discard)
	for {
		body, err := s.body()
		if err != nil {
			return
		}
		var m incoming
		if err := json.Unmarshal(body, &m); err != nil {
			return
		}
		c.messages <- m
	}
}

func (c *client) send(m outgoing) {
	c.t.Helper()
	m.JSONRPC = "2.0"
	if err := c.to.write(m); err != nil {
		c.t.Fatal(err)
	}
}

func (c *client) notify(method string, params any) {
	c.t.Helper()
	c.send(outgoing{Method: method, Params: params})
}

func (c *client) request(method string, params any) json.RawMessage {
	c.t.Helper()
	result, failure := c.call(method, params)
	if failure != nil {
		c.t.Fatalf("%s failed: %s", method, failure.Message)
	}
	return result
}

func (c *client) call(method string, params any) (json.RawMessage, *rpcError) {
	c.t.Helper()
	c.next++
	id := c.next
	c.send(outgoing{ID: &id, Method: method, Params: params})
	timeout := time.After(10 * time.Second)
	for {
		select {
		case m, ok := <-c.messages:
			if !ok {
				c.t.Fatalf("connection closed waiting for %s", method)
			}
			if m.Method != "" {
				c.record(m)
				continue
			}
			if m.ID == nil || *m.ID != id {
				continue
			}
			return m.Result, m.Error
		case <-timeout:
			c.t.Fatalf("timed out waiting for %s", method)
		}
	}
}

func (c *client) record(m incoming) {
	c.t.Helper()
	if m.Method != "textDocument/publishDiagnostics" {
		c.asked[m.Method]++
		return
	}
	var p PublishDiagnosticsParams
	if err := json.Unmarshal(m.Params, &p); err != nil {
		c.t.Fatal(err)
	}
	c.diagnostics[p.URI] = p.Diagnostics
	c.publishes[p.URI]++
}

func (c *client) await(uri string, after int) []Diagnostic {
	c.t.Helper()
	timeout := time.After(10 * time.Second)
	for c.publishes[uri] <= after {
		select {
		case m, ok := <-c.messages:
			if !ok {
				c.t.Fatalf("connection closed waiting for diagnostics on %s", uri)
			}
			if m.Method != "" {
				c.record(m)
			}
		case <-timeout:
			c.t.Fatalf("timed out waiting for diagnostics on %s", uri)
		}
	}
	return c.diagnostics[uri]
}

func (c *client) change(f fixture, text string) []Diagnostic {
	c.t.Helper()
	before := c.publishes[f.uri]
	change := DidChangeParams{TextDocument: TextDocumentIdentifier{URI: f.uri}, ContentChanges: []ContentChange{{Text: text}}}
	c.notify("textDocument/didChange", change)
	return c.await(f.uri, before)
}

type fixture struct {
	path string
	uri  string
	src  string
}

func load(t *testing.T, parts ...string) fixture {
	t.Helper()
	rel := filepath.Join(append([]string{"..", "..", "testdata", "run"}, parts...)...)
	path, err := filepath.Abs(rel)
	if err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return fixture{path: path, uri: uriOf(path), src: string(src)}
}

func (f fixture) at(t *testing.T, needle string, offset int) Position {
	t.Helper()
	for i, line := range strings.Split(f.src, "\n") {
		if col := strings.Index(line, needle); col >= 0 {
			return Position{Line: i, Character: col + offset}
		}
	}
	t.Fatalf("%q not found in %s", needle, f.path)
	return Position{}
}

func (c *client) open(f fixture, text string) []Diagnostic {
	c.t.Helper()
	before := c.publishes[f.uri]
	c.notify("textDocument/didOpen", DidOpenParams{TextDocument: TextDocumentItem{URI: f.uri, Text: text}})
	return c.await(f.uri, before)
}

func (c *client) symbols(f fixture) []DocumentSymbol {
	c.t.Helper()
	raw := c.request("textDocument/documentSymbol", DocumentParams{TextDocument: TextDocumentIdentifier{URI: f.uri}})
	var symbols []DocumentSymbol
	if err := json.Unmarshal(raw, &symbols); err != nil {
		c.t.Fatal(err)
	}
	return symbols
}

func (c *client) hover(f fixture, pos Position) string {
	c.t.Helper()
	params := TextDocumentPositionParams{TextDocument: TextDocumentIdentifier{URI: f.uri}, Position: pos}
	raw := c.request("textDocument/hover", params)
	var h *Hover
	if err := json.Unmarshal(raw, &h); err != nil {
		c.t.Fatal(err)
	}
	if h == nil {
		return ""
	}
	return h.Contents.Value
}

func (c *client) definition(f fixture, pos Position) Location {
	c.t.Helper()
	params := TextDocumentPositionParams{TextDocument: TextDocumentIdentifier{URI: f.uri}, Position: pos}
	raw := c.request("textDocument/definition", params)
	var loc *Location
	if err := json.Unmarshal(raw, &loc); err != nil {
		c.t.Fatal(err)
	}
	if loc == nil {
		c.t.Fatalf("no definition at %+v", pos)
	}
	return *loc
}

func TestDiagnosticsFollowOverlay(t *testing.T) {
	c := start(t)
	f := load(t, "enums.low")
	broken := f.src + "\nfn broken() {\n    let x: i64 = \"s\"\n}\n"
	got := c.open(f, broken)
	if len(got) != 1 {
		t.Fatalf("diagnostics = %+v, want one", got)
	}
	wantLine := strings.Count(f.src, "\n") + 2
	if got[0].Range.Start.Line != wantLine {
		t.Errorf("diagnostic line = %d, want %d (%s)", got[0].Range.Start.Line, wantLine, got[0].Message)
	}
	if got := c.change(f, f.src); len(got) != 0 {
		t.Errorf("diagnostics after fix = %+v, want none", got)
	}
}

func TestRapidEditsPublishOnlyTheLatest(t *testing.T) {
	c := start(t)
	f := load(t, "enums.low")
	c.open(f, f.src)
	before := c.publishes[f.uri]
	for i := range 5 {
		broken := fmt.Sprintf("%s\nfn broken%d() {\n    let x: i64 = \"s\"\n}\n", f.src, i)
		change := DidChangeParams{TextDocument: TextDocumentIdentifier{URI: f.uri}, ContentChanges: []ContentChange{{Text: broken}}}
		c.notify("textDocument/didChange", change)
	}
	if got := c.change(f, f.src); len(got) != 0 {
		t.Errorf("diagnostics = %+v, want none from the superseded edits", got)
	}
	if published := c.publishes[f.uri] - before; published != 1 {
		t.Errorf("published %d times for six edits, want 1", published)
	}
}

func TestHoverDoesNotWaitForPendingCompile(t *testing.T) {
	c := start(t)
	f := load(t, "enums.low")
	c.open(f, f.src)
	before := c.publishes[f.uri]
	change := DidChangeParams{TextDocument: TextDocumentIdentifier{URI: f.uri}, ContentChanges: []ContentChange{{Text: f.src}}}
	c.notify("textDocument/didChange", change)
	if got := c.hover(f, f.at(t, "print(area(big))", 11)); !strings.Contains(got, "let big: Shape") {
		t.Errorf("hover during pending compile = %q", got)
	}
	if c.publishes[f.uri] != before {
		t.Errorf("hover waited for the debounced compile")
	}
	c.await(f.uri, before)
}

func TestHover(t *testing.T) {
	c := start(t)
	f := load(t, "enums.low")
	c.open(f, f.src)
	cases := []struct {
		at   Position
		want string
	}{
		{f.at(t, "print(area(big))", 11), "let big: Shape"},
		{f.at(t, "print(area(big))", 6), "fn area(s: Shape) -> i64"},
		{f.at(t, "Circle(r) =>", 0), "Shape.Circle(i64)"},
		{f.at(t, "var s: Shape", 7), "enum Shape {"},
	}
	for _, tc := range cases {
		if got := c.hover(f, tc.at); !strings.Contains(got, tc.want) {
			t.Errorf("hover at %+v = %q, want %q", tc.at, got, tc.want)
		}
	}
}

func TestHoverShowsDocComments(t *testing.T) {
	c := start(t)
	f := load(t, "doc_comments.low")
	c.open(f, f.src)
	cases := []struct {
		at   Position
		want string
	}{
		{f.at(t, "match side(", 6), "fn side(p: Point) -> Side\n```\n\nClassifies a point.\n\n```low\nside(Point { x: -1, y: 0 })\n```"},
		{f.at(t, "side(Point { x: -3", 5), "A point on the plane."},
		{f.at(t, "Left =>", 0), "Left of the y axis."},
		{f.at(t, "pub x: i64", 4), "Distance from the y axis."},
	}
	for _, tc := range cases {
		if got := c.hover(f, tc.at); !strings.Contains(got, tc.want) {
			t.Errorf("hover at %+v = %q, want %q", tc.at, got, tc.want)
		}
	}
}

func TestHoverOnBuiltins(t *testing.T) {
	c := start(t)
	f := load(t, "maps.low")
	c.open(f, f.src)
	cases := []struct {
		at   Position
		want string
	}{
		{f.at(t, "print(ages.len)", 0), "fn print(value: i64)"},
		{f.at(t, `print("{k}={v}")`, 0), "fn print(value: str)"},
		{f.at(t, `delete(mut shared, "alan")`, 0), "fn delete(mut map: [str: i64], key: str)"},
	}
	for _, tc := range cases {
		if got := c.hover(f, tc.at); !strings.Contains(got, tc.want) {
			t.Errorf("hover at %+v = %q, want %q", tc.at, got, tc.want)
		}
	}
}

func TestDefinition(t *testing.T) {
	c := start(t)
	f := load(t, "enums.low")
	c.open(f, f.src)
	cases := []struct {
		at   Position
		want Position
	}{
		{f.at(t, "print(area(big))", 6), f.at(t, "fn area(", 3)},
		{f.at(t, "print(area(big))", 11), f.at(t, "let big", 4)},
		{f.at(t, "Circle(r) =>", 0), f.at(t, "Circle(radius", 0)},
		{f.at(t, "Circle(r) =>", 7), f.at(t, "Circle(r) =>", 7)},
		{f.at(t, "fn area(s: Shape)", 11), f.at(t, "enum Shape", 5)},
	}
	for _, tc := range cases {
		loc := c.definition(f, tc.at)
		if loc.URI != f.uri || loc.Range.Start != tc.want {
			t.Errorf("definition at %+v = %s %+v, want %+v", tc.at, loc.URI, loc.Range.Start, tc.want)
		}
	}
}

func TestDefinitionAcrossModules(t *testing.T) {
	c := start(t)
	f := load(t, "modules", "main.low")
	c.open(f, f.src)
	if got := c.diagnostics[f.uri]; len(got) != 0 {
		t.Fatalf("diagnostics = %+v", got)
	}
	loc := c.definition(f, f.at(t, "geometry.circle(2)", 9))
	path := pathOf(loc.URI)
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	line := strings.Split(string(src), "\n")[loc.Range.Start.Line]
	if filepath.Base(filepath.Dir(path)) != "geometry" || !strings.Contains(line, "fn circle") {
		t.Errorf("definition = %s:%d %q", path, loc.Range.Start.Line, line)
	}
	module := c.definition(f, f.at(t, "import geometry", 7))
	if filepath.Base(filepath.Dir(pathOf(module.URI))) != "geometry" {
		t.Errorf("import definition = %s", module.URI)
	}
}

func TestDocumentSymbols(t *testing.T) {
	c := start(t)
	f := load(t, "enums.low")
	symbols := c.symbols(f)
	names := map[string]DocumentSymbol{}
	for _, s := range symbols {
		names[s.Name] = s
	}
	for _, want := range []string{"Shape", "Color", "Wrapper", "area", "main"} {
		if _, ok := names[want]; !ok {
			t.Errorf("missing symbol %s in %+v", want, symbols)
		}
	}
	shape := names["Shape"]
	if len(shape.Children) != 3 || shape.Children[0].Name != "Circle" {
		t.Errorf("Shape children = %+v", shape.Children)
	}
	if got := names["area"].Detail; got != "fn area(s: Shape) -> i64" {
		t.Errorf("area detail = %q", got)
	}
}

func TestShutdown(t *testing.T) {
	c := start(t)
	c.request("shutdown", nil)
	c.notify("exit", nil)
	select {
	case err := <-c.done:
		if err != nil {
			t.Fatalf("Run = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not exit")
	}
}

func TestEveryOccurrenceAnswers(t *testing.T) {
	entries, err := filepath.Glob(filepath.Join("..", "..", "testdata", "run", "*.low"))
	if err != nil {
		t.Fatal(err)
	}
	modules, err := filepath.Glob(filepath.Join("..", "..", "testdata", "run", "modules", "*", "*.low"))
	if err != nil {
		t.Fatal(err)
	}
	c := start(t)
	for _, entry := range append(entries, modules...) {
		rel, err := filepath.Rel(filepath.Join("..", "..", "testdata", "run"), entry)
		if err != nil {
			t.Fatal(err)
		}
		f := load(t, rel)
		c.open(f, f.src)
		for line, text := range strings.Split(f.src, "\n") {
			for col := range len(text) {
				if !isWord(text[col]) || (col > 0 && isWord(text[col-1])) {
					continue
				}
				pos := Position{Line: line, Character: col}
				c.hover(f, pos)
				params := TextDocumentPositionParams{TextDocument: TextDocumentIdentifier{URI: f.uri}, Position: pos}
				c.request("textDocument/definition", params)
			}
		}
	}
}
