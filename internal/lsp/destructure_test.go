package lsp

import (
	"strings"
	"testing"
)

const destructureSource = `struct Point {
    x: i64
    y: i64
}

struct Rect {
    min: Point
    max: Point
}

fn main() {
    let r = Rect { min: Point { x: 1, y: 2 }, max: Point { x: 3, y: 4 } }
    let { min: { x, y }, max: top } = r
    print(x + y + top.x)
    for { x: px } in [r.min, r.max] {
        print(px)
    }
}
`

func TestDestructureHoverAndDefinition(t *testing.T) {
	c := start(t)
	f := scratch(t, "destructure.low", destructureSource)
	c.open(f, f.src)
	if got := c.hover(f, f.at(t, "x + y", 0)); !strings.Contains(got, "x: i64") {
		t.Errorf("hover on a shorthand-bound use = %q", got)
	}
	if got := c.hover(f, f.at(t, "top.x", 0)); !strings.Contains(got, "top: Point") {
		t.Errorf("hover on a bound name = %q", got)
	}
	if got := c.hover(f, f.at(t, "min: {", 0)); !strings.Contains(got, "min: Point") {
		t.Errorf("hover on a field key = %q", got)
	}
	if got := c.definition(f, f.at(t, "x + y", 0)); got.Range.Start != f.at(t, "{ x, y }", 2) {
		t.Errorf("definition of x = %+v", got)
	}
	if got := c.definition(f, f.at(t, "top.x", 0)); got.Range.Start != f.at(t, "top }", 0) {
		t.Errorf("definition of top = %+v", got)
	}
	if got := c.definition(f, f.at(t, "min: {", 0)); got.Range.Start != f.at(t, "    min: Point", 4) {
		t.Errorf("definition of the field key min = %+v", got)
	}
}

func TestDestructureReferences(t *testing.T) {
	c := start(t)
	f := scratch(t, "destructure.low", destructureSource)
	c.open(f, f.src)
	locals := c.references(f, f.at(t, "x + y", 0), true)
	if len(locals) != 2 {
		t.Errorf("references to the local x = %d, want 2: %+v", len(locals), locals)
	}
	fields := c.references(f, f.at(t, "    x: i64", 4), true)
	if len(fields) < 4 {
		t.Errorf("references to the field x = %d, want at least 4: %+v", len(fields), fields)
	}
}

func TestRenameLocalInShorthandPattern(t *testing.T) {
	c := start(t)
	f := scratch(t, "destructure.low", destructureSource)
	c.open(f, f.src)
	edited := c.renamed(f, f.at(t, "x + y", 0), "left")
	got := string(edited[f.path])
	if !strings.Contains(got, "min: { x: left, y }") || !strings.Contains(got, "left + y + top.x") {
		t.Errorf("local rename:\n%s", got)
	}
	compiles(t, f.path, edited)
}

func TestRenameFieldInShorthandPattern(t *testing.T) {
	c := start(t)
	f := scratch(t, "destructure.low", destructureSource)
	c.open(f, f.src)
	edited := c.renamed(f, f.at(t, "    x: i64", 4), "column")
	got := string(edited[f.path])
	if !strings.Contains(got, "min: { column: x, y }") || !strings.Contains(got, "for { column: px }") || !strings.Contains(got, "Point { column: 1") {
		t.Errorf("field rename:\n%s", got)
	}
	compiles(t, f.path, edited)
}

func TestRenameBoundNameInPattern(t *testing.T) {
	c := start(t)
	f := scratch(t, "destructure.low", destructureSource)
	c.open(f, f.src)
	edited := c.renamed(f, f.at(t, "top.x", 0), "upper")
	got := string(edited[f.path])
	if !strings.Contains(got, "max: upper }") || !strings.Contains(got, "upper.x") {
		t.Errorf("bound name rename:\n%s", got)
	}
	compiles(t, f.path, edited)
}
