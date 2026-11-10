package lsp

import (
	"encoding/json"
	"testing"
)

func (c *client) signature(f fixture, needle, typed string) *SignatureHelp {
	c.t.Helper()
	edit := f.typeAfter(c.t, needle, typed)
	c.change(f, edit.text)
	params := TextDocumentPositionParams{TextDocument: TextDocumentIdentifier{URI: f.uri}, Position: edit.at}
	raw := c.request("textDocument/signatureHelp", params)
	var help *SignatureHelp
	if err := json.Unmarshal(raw, &help); err != nil {
		c.t.Fatal(err)
	}
	return help
}

func activeParam(help *SignatureHelp) string {
	info := help.Signatures[0]
	span := info.Parameters[help.ActiveParameter].Label
	return info.Label[span[0]:span[1]]
}

func TestSignatureHelp(t *testing.T) {
	c := start(t)
	f := load(t, "structs.low")
	c.open(f, f.src)
	cases := []struct {
		typed string
		label string
		param string
	}{
		{"    add(", "fn add(a: Point, b: Point) -> Point", "a: Point"},
		{"    add(p, ", "fn add(a: Point, b: Point) -> Point", "b: Point"},
		{"    print(add(p, ", "fn add(a: Point, b: Point) -> Point", "b: Point"},
		{"    add(origin(), ", "fn add(a: Point, b: Point) -> Point", "b: Point"},
		{"    add(\"a,(b\", ", "fn add(a: Point, b: Point) -> Point", "b: Point"},
		{"    add(\n        p,\n        ", "fn add(a: Point, b: Point) -> Point", "b: Point"},
		{"    add([p, q], ", "fn add(a: Point, b: Point) -> Point", "b: Point"},
		{"    Box(", "Shape.Box(Rect)", "Rect"},
	}
	for _, tc := range cases {
		help := c.signature(f, "print(r.label)", tc.typed)
		if help == nil {
			t.Errorf("%q: no signature help", tc.typed)
			continue
		}
		if got := help.Signatures[0].Label; got != tc.label {
			t.Errorf("%q: label = %q, want %q", tc.typed, got, tc.label)
		}
		if got := activeParam(help); got != tc.param {
			t.Errorf("%q: active = %q, want %q", tc.typed, got, tc.param)
		}
	}
	for _, typed := range []string{"    add(p, q) ", "    unknown(", "    let x = 1"} {
		if help := c.signature(f, "print(r.label)", typed); help != nil {
			t.Errorf("%q: signature help = %+v, want none", typed, help)
		}
	}
}

func TestSignatureHelpAcrossModules(t *testing.T) {
	c := start(t)
	f := load(t, "modules", "main.low")
	c.open(f, f.src)
	help := c.signature(f, "fn main() {", "    geometry.circle(")
	if help == nil {
		t.Fatal("no signature help")
	}
	if got := help.Signatures[0].Label; got != "fn circle(r: i64) -> geometry.Circle" {
		t.Errorf("label = %q", got)
	}
	variant := c.signature(f, "fn main() {", "    geometry.Shape.Square(")
	if variant == nil || variant.Signatures[0].Label != "Shape.Square(i64)" {
		t.Errorf("variant help = %+v", variant)
	}
}
