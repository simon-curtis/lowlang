package lsp

import (
	"strings"
	"testing"
)

func TestHoverOnUsingBindings(t *testing.T) {
	c := start(t)
	f := load(t, "using.low")
	c.open(f, f.src)
	cases := []struct {
		at   Position
		want string
	}{
		{f.at(t, "using log = open", 6), "using log: File"},
		{f.at(t, "using b = a", 10), "using a: File"},
		{f.at(t, "Conn { id, log }", 11), "log: File"},
		{f.at(t, "fn connect(id: i64)", 11), "id: i64"},
	}
	for _, tc := range cases {
		if got := c.hover(f, tc.at); !strings.Contains(got, tc.want) {
			t.Errorf("hover at %+v = %q, want %q", tc.at, got, tc.want)
		}
	}
}

func TestDropBodiesAndTestArmsAreIndexed(t *testing.T) {
	c := start(t)
	f := scratch(t, "drops.low", "struct File {\n    fd: i64\n    label: str = make_label(3)\n    drop release(fd)\n}\n\nfn release(fd: i64) {\n    print(fd)\n}\n\nfn make_label(n: i64) -> str {\n    \"file\"\n}\n\nfn limit(scale: i64) -> i64 {\n    scale * 2\n}\n\nfn band(n: i64) -> str {\n    match n {\n        < limit(5) => \"low\",\n        _ => \"high\"\n    }\n}\n\nfn main() {\n    using f = File { fd: 3 }\n    print(band(f.fd))\n}\n")
	if got := c.open(f, f.src); len(got) != 0 {
		t.Fatalf("diagnostics = %+v", got)
	}
	cases := []struct {
		at   Position
		want string
	}{
		{f.at(t, "drop release(fd)", 5), "fn release(fd: i64)"},
		{f.at(t, "= make_label(3)", 2), "fn make_label(n: i64) -> str"},
		{f.at(t, "< limit(5)", 2), "fn limit(scale: i64) -> i64"},
	}
	for _, tc := range cases {
		if got := c.hover(f, tc.at); !strings.Contains(got, tc.want) {
			t.Errorf("hover at %+v = %q, want %q", tc.at, got, tc.want)
		}
	}
	loc := c.definition(f, f.at(t, "drop release(fd)", 5))
	if loc.Range.Start != f.at(t, "fn release", 3) {
		t.Errorf("definition of release from drop = %+v", loc.Range.Start)
	}
}
