package lsp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"lowlang/internal/compiler"
)

func scratch(t *testing.T, name, src string) fixture {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return fixture{path: path, uri: uriOf(path), src: src}
}

func (c *client) references(f fixture, pos Position, declaration bool) []Location {
	c.t.Helper()
	params := ReferenceParams{
		TextDocument: TextDocumentIdentifier{URI: f.uri},
		Position:     pos,
		Context:      ReferenceContext{IncludeDeclaration: declaration},
	}
	raw := c.request("textDocument/references", params)
	var locations []Location
	if err := json.Unmarshal(raw, &locations); err != nil {
		c.t.Fatal(err)
	}
	return locations
}

func (c *client) rename(f fixture, pos Position, name string) (WorkspaceEdit, *rpcError) {
	c.t.Helper()
	params := RenameParams{TextDocument: TextDocumentIdentifier{URI: f.uri}, Position: pos, NewName: name}
	raw, failure := c.call("textDocument/rename", params)
	if failure != nil {
		return WorkspaceEdit{}, failure
	}
	var edit WorkspaceEdit
	if err := json.Unmarshal(raw, &edit); err != nil {
		c.t.Fatal(err)
	}
	return edit, nil
}

func (c *client) renamed(f fixture, pos Position, name string) sources {
	c.t.Helper()
	edit, failure := c.rename(f, pos, name)
	if failure != nil {
		c.t.Fatalf("rename to %s failed: %s", name, failure.Message)
	}
	return applyEdit(c.t, edit)
}

func applyEdit(t *testing.T, edit WorkspaceEdit) sources {
	t.Helper()
	out := sources{}
	for uri, edits := range edit.Changes {
		path := pathOf(uri)
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		out[path] = []byte(applyText(string(src), edits))
	}
	return out
}

func applyText(src string, edits []TextEdit) string {
	t := newText([]byte(src))
	sorted := slices.Clone(edits)
	slices.SortFunc(sorted, func(a, b TextEdit) int {
		if a.Range.Start.Line != b.Range.Start.Line {
			return b.Range.Start.Line - a.Range.Start.Line
		}
		return b.Range.Start.Character - a.Range.Start.Character
	})
	lines := slices.Clone(t.lines)
	for _, e := range sorted {
		line := e.Range.Start.Line
		from := t.byteCol(line, e.Range.Start.Character)
		to := t.byteCol(line, e.Range.End.Character)
		lines[line] = lines[line][:from] + e.NewText + lines[line][to:]
	}
	return strings.Join(lines, "\n")
}

func compiles(t *testing.T, entry string, edited sources) {
	t.Helper()
	if _, err := compiler.Check(entry, edited.read); err != nil {
		t.Errorf("renamed program does not compile: %v", err)
	}
}

func TestReferences(t *testing.T) {
	c := start(t)
	f := load(t, "enums.low")
	c.open(f, f.src)
	at := f.at(t, "print(area(big))", 6)
	with := c.references(f, at, true)
	want := strings.Count(f.src, "area(")
	if len(with) != want {
		t.Errorf("references with declaration = %d, want %d", len(with), want)
	}
	without := c.references(f, at, false)
	if len(without) != want-1 {
		t.Errorf("references without declaration = %d, want %d", len(without), want-1)
	}
	declaration := f.at(t, "fn area(", 3)
	for _, loc := range without {
		if loc.Range.Start == declaration {
			t.Errorf("declaration included in %+v", without)
		}
	}
}

func TestRenameLocalLeavesOtherFunctions(t *testing.T) {
	c := start(t)
	f := load(t, "structs.low")
	c.open(f, f.src)
	edited := c.renamed(f, f.at(t, "print(r.label)", 6), "rect")
	src := string(edited[f.path])
	if !strings.Contains(src, "print(rect.label)") || !strings.Contains(src, "r.max.x - r.min.x") {
		t.Errorf("rename touched the wrong r:\n%s", src)
	}
	compiles(t, f.path, edited)
}

func TestRenameFieldExpandsShorthand(t *testing.T) {
	c := start(t)
	src := "struct Point {\n    x: i64\n    y: i64\n}\n\nfn make(x: i64, y: i64) -> Point {\n    Point { x, y }\n}\n\nfn main() {\n    let p = make(1, 2)\n    print(p.x)\n}\n"
	f := scratch(t, "shorthand.low", src)
	c.open(f, f.src)
	field := c.renamed(f, f.at(t, "    x: i64", 4), "px")
	if got := string(field[f.path]); !strings.Contains(got, "Point { px: x, y }") || !strings.Contains(got, "p.px") {
		t.Errorf("field rename:\n%s", got)
	}
	compiles(t, f.path, field)
	param := c.renamed(f, f.at(t, "fn make(x", 8), "a")
	if got := string(param[f.path]); !strings.Contains(got, "Point { x: a, y }") || !strings.Contains(got, "fn make(a: i64") {
		t.Errorf("parameter rename:\n%s", got)
	}
	compiles(t, f.path, param)
}

func TestInterpolationWithWidthAndFormat(t *testing.T) {
	c := start(t)
	src := "fn main() {\n    let ratio = 3.14159\n    print(\"{ratio,10:f2}|{ratio:e1}|{ratio, -8}|{ratio}\")\n}\n"
	f := scratch(t, "specs.low", src)
	c.open(f, f.src)
	for _, probe := range []string{"{ratio,10:f2}", "{ratio:e1}", "{ratio, -8}", "{ratio}"} {
		at := f.at(t, probe, 1)
		if got := c.hover(f, at); !strings.Contains(got, "f64") {
			t.Errorf("hover on ratio in %s = %q, want its type", probe, got)
		}
		if got := c.definition(f, at); got.Range.Start != f.at(t, "let ratio", 4) {
			t.Errorf("definition of ratio in %s = %+v", probe, got)
		}
	}
	references := c.references(f, f.at(t, "{ratio,10:f2}", 1), true)
	if len(references) != 5 {
		t.Errorf("references to ratio = %d, want 5", len(references))
	}
	edited := c.renamed(f, f.at(t, "let ratio", 4), "r")
	want := "print(\"{r,10:f2}|{r:e1}|{r, -8}|{r}\")"
	if got := string(edited[f.path]); !strings.Contains(got, want) {
		t.Errorf("rename left specs misplaced:\n%s", got)
	}
	compiles(t, f.path, edited)
}

func TestRenameBindingArm(t *testing.T) {
	c := start(t)
	src := `enum Oops {
    Bad(i64)
    Worse
}

fn risky(n: i64) -> i64 ! Oops {
    if n > 2 {
        fail Bad(n)
    }
    n
}

fn describe(e: Oops) -> i64 {
    match e {
        Bad(n) => n
        Worse => 0
    }
}

fn main() {
    print(risky(5) catch {
        problem => describe(problem)
    })
}
`
	f := scratch(t, "binding.low", src)
	c.open(f, f.src)
	edited := c.renamed(f, f.at(t, "problem =>", 0), "err")
	if got := string(edited[f.path]); !strings.Contains(got, "err => describe(err)") {
		t.Errorf("binding arm rename:\n%s", got)
	}
	compiles(t, f.path, edited)
}

func TestRenameAcrossModules(t *testing.T) {
	c := start(t)
	f := load(t, "modules", "main.low")
	c.open(f, f.src)
	entry := filepath.Dir(f.path)
	function := c.renamed(f, f.at(t, "geometry.circle(2)", 9), "round")
	if len(function) != 2 {
		t.Errorf("function rename edited %d files, want 2", len(function))
	}
	compiles(t, entry, function)
	variant := c.renamed(f, f.at(t, "Square(3)", 0), "Block")
	if got := string(variant[f.path]); strings.Contains(got, "Square") {
		t.Errorf("variant rename left Square:\n%s", got)
	}
	compiles(t, entry, variant)
}

func TestRenameRefusals(t *testing.T) {
	c := start(t)
	f := load(t, "modules", "main.low")
	c.open(f, f.src)
	local := f.at(t, "let c = ", 4)
	cases := []struct {
		at     Position
		name   string
		reason string
	}{
		{f.at(t, "print(geometry", 0), "show", "nothing to rename"},
		{f.at(t, "geometry.circle(2)", 0), "geo", "only names declared"},
		{local, "fn", "is a keyword"},
		{local, "2c", "not a valid name"},
		{local, "__c", "are reserved"},
	}
	for _, tc := range cases {
		_, failure := c.rename(f, tc.at, tc.name)
		if failure == nil || !strings.Contains(failure.Message, tc.reason) {
			t.Errorf("rename to %q = %+v, want failure %q", tc.name, failure, tc.reason)
		}
	}
	c.change(f, f.src+"\nfn broken() { let x: i64 = \"s\" }\n")
	_, failure := c.rename(f, local, "circle")
	if failure == nil || !strings.Contains(failure.Message, "needs the program to compile") {
		t.Errorf("rename on a broken program = %+v", failure)
	}
}
