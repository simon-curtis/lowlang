package lsp

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

const constantsSource = `/// Bytes in a kilobyte.
pub const kib = 1 << 10
const mib = kib * kib
const name: str = "low"

fn size(n: i64) -> i64 {
    n * mib
}

fn pick(code: i64) -> str {
    match code {
        kib => "kib"
        _ => name
    }
}

fn main() {
    let kib = 2
    print(size(kib))
    print(pick(1024))
}
`

func TestConstantHover(t *testing.T) {
	c := start(t)
	f := scratch(t, "constants.low", constantsSource)
	if got := c.open(f, f.src); len(got) != 0 {
		t.Fatalf("diagnostics = %+v", got)
	}
	cases := []struct {
		at   Position
		want string
	}{
		{f.at(t, "n * mib", 4), "const mib: i64 = kib * kib"},
		{f.at(t, "const mib", 6), "const mib: i64 = kib * kib"},
		{f.at(t, "_ => name", 5), "const name: str = \"low\""},
		{f.at(t, "kib => ", 0), "const kib: i64 = 1 << 10\n```\n\nBytes in a kilobyte."},
		{f.at(t, "const mib", 12), "Bytes in a kilobyte."},
		{f.at(t, "print(size(kib))", 11), "let kib: i64"},
	}
	for _, tc := range cases {
		if got := c.hover(f, tc.at); !strings.Contains(got, tc.want) {
			t.Errorf("hover at %+v = %q, want %q", tc.at, got, tc.want)
		}
	}
	if got := c.hover(f, f.at(t, "print(size(kib))", 11)); strings.Contains(got, "const") {
		t.Errorf("hover on the local that hides a constant = %q", got)
	}
}

func TestConstantDefinition(t *testing.T) {
	c := start(t)
	f := scratch(t, "constants.low", constantsSource)
	c.open(f, f.src)
	cases := []struct {
		at   Position
		want Position
	}{
		{f.at(t, "n * mib", 4), f.at(t, "const mib", 6)},
		{f.at(t, "const mib", 6), f.at(t, "const mib", 6)},
		{f.at(t, "kib => ", 0), f.at(t, "pub const kib", 10)},
		{f.at(t, "const mib = kib", 12), f.at(t, "pub const kib", 10)},
		{f.at(t, "print(size(kib))", 11), f.at(t, "let kib", 4)},
	}
	for _, tc := range cases {
		loc := c.definition(f, tc.at)
		if loc.URI != f.uri || loc.Range.Start != tc.want {
			t.Errorf("definition at %+v = %s %+v, want %+v", tc.at, loc.URI, loc.Range.Start, tc.want)
		}
	}
}

func TestConstantReferences(t *testing.T) {
	c := start(t)
	f := scratch(t, "constants.low", constantsSource)
	c.open(f, f.src)
	at := f.at(t, "pub const kib", 10)
	if with := c.references(f, at, true); len(with) != 4 {
		t.Errorf("references with declaration = %d, want 4: %+v", len(with), with)
	}
	without := c.references(f, f.at(t, "kib => ", 0), false)
	if len(without) != 3 {
		t.Errorf("references without declaration = %d, want 3: %+v", len(without), without)
	}
	for _, loc := range without {
		if loc.Range.Start == at {
			t.Errorf("declaration included in %+v", without)
		}
	}
}

func TestRenameConstant(t *testing.T) {
	c := start(t)
	f := scratch(t, "constants.low", constantsSource)
	c.open(f, f.src)
	edited := c.renamed(f, f.at(t, "const mib = kib", 12), "kilo")
	got := string(edited[f.path])
	for _, want := range []string{"pub const kilo = 1 << 10", "const mib = kilo * kilo", "kilo => \"kib\"", "let kib = 2", "print(size(kib))"} {
		if !strings.Contains(got, want) {
			t.Errorf("rename lacks %q:\n%s", want, got)
		}
	}
	compiles(t, f.path, edited)
	local := c.renamed(f, f.at(t, "let kib", 4), "two")
	if got := string(local[f.path]); !strings.Contains(got, "let two = 2") || !strings.Contains(got, "print(size(two))") || !strings.Contains(got, "kib => \"kib\"") {
		t.Errorf("renaming the local touched the constant:\n%s", got)
	}
}

func TestConstantsAcrossModules(t *testing.T) {
	c := start(t)
	f := load(t, "consts", "main.low")
	if got := c.open(f, f.src); len(got) != 0 {
		t.Fatalf("diagnostics = %+v", got)
	}
	unbounded := f.at(t, "width < limits.unbounded", 15)
	hover := c.hover(f, unbounded)
	for _, want := range []string{"const unbounded: f32 = 1.0 / 0.0", "The width that stands for no limit."} {
		if !strings.Contains(hover, want) {
			t.Errorf("hover = %q, want %q", hover, want)
		}
	}
	loc := c.definition(f, unbounded)
	if filepath.Base(pathOf(loc.URI)) != "limits.low" || loc.Range.Start != (Position{Line: 1, Character: 10}) {
		t.Errorf("definition = %s %+v", loc.URI, loc.Range.Start)
	}
	if refs := c.references(f, unbounded, true); len(refs) != 3 {
		t.Errorf("references to unbounded = %d, want 3: %+v", len(refs), refs)
	}
	edited := c.renamed(f, f.at(t, "print(limits.mib)", 13), "megabyte")
	if len(edited) != 2 {
		t.Errorf("rename edited %d files, want 2", len(edited))
	}
	if got := string(edited[f.path]); !strings.Contains(got, "print(limits.megabyte)") {
		t.Errorf("rename left the use:\n%s", got)
	}
	compiles(t, filepath.Dir(f.path), edited)
}

func TestConstantSymbols(t *testing.T) {
	c := start(t)
	f := scratch(t, "constants.low", constantsSource)
	c.open(f, f.src)
	names := map[string]DocumentSymbol{}
	for _, s := range c.symbols(f) {
		names[s.Name] = s
	}
	for name, detail := range map[string]string{"kib": "const kib = 1 << 10", "mib": "const mib = kib * kib", "name": "const name: str = \"low\""} {
		s, ok := names[name]
		if !ok {
			t.Errorf("missing symbol %s", name)
			continue
		}
		if s.Kind != symbolConstant || s.Detail != detail {
			t.Errorf("%s = kind %d detail %q, want kind %d detail %q", name, s.Kind, s.Detail, symbolConstant, detail)
		}
	}
	raw := c.request("workspace/symbol", WorkspaceSymbolParams{Query: "ib"})
	var found []SymbolInformation
	if err := json.Unmarshal(raw, &found); err != nil {
		t.Fatal(err)
	}
	kinds := map[string]int{}
	for _, s := range found {
		kinds[s.Name] = s.Kind
	}
	if kinds["kib"] != symbolConstant || kinds["mib"] != symbolConstant || len(kinds) != 2 {
		t.Errorf("workspace symbols = %+v", found)
	}
}

func TestConstantCompletion(t *testing.T) {
	c := start(t)
	f := scratch(t, "constants.low", constantsSource)
	c.open(f, f.src)
	items := c.completeTyping(f, "let kib = 2", "    ")
	expect(t, items, []string{"kib", "mib", "name", "size"}, nil)
	if got := items["mib"]; got.Kind != completionConstant || got.Detail != "const mib: i64 = kib * kib" {
		t.Errorf("mib = %+v", got)
	}
	g := load(t, "consts", "main.low")
	c.open(g, g.src)
	members := c.completeTyping(g, "fn main() {", "    limits.")
	expect(t, members, []string{"unbounded", "kib", "mib", "greeting"}, []string{"name", "main"})
	if got := members["unbounded"]; got.Kind != completionConstant || got.Detail != "const unbounded: f32 = 1.0 / 0.0" {
		t.Errorf("unbounded = %+v", got)
	}
}
