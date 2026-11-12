package lsp

import (
	"strings"
	"testing"
)

const methodsSource = `struct Point {
    x: i64
    y: i64
}

struct Op {
    apply: fn(i64) -> i64
}

fn scale(p: Point, by: i64) -> Point {
    Point { x: p.x * by, y: p.y * by }
}

fn norm(p: Point) -> i64 {
    p.x + p.y
}

fn first[T](xs: []T) -> T {
    xs[0]
}

fn unrelated(n: i64) -> i64 {
    n
}

fn main() {
    let p = Point { x: 1, y: 2 }
    let pts = [p]
    let op = Op { apply: fn(n: i64) -> i64 { n } }
    print(p.norm() + op.apply(1) + pts.first().x)
}
`

func TestMethodCompletion(t *testing.T) {
	c := start(t)
	f := scratch(t, "methods.low", methodsSource)
	if got := c.open(f, f.src); len(got) != 0 {
		t.Fatalf("diagnostics = %+v", got)
	}
	cases := []struct {
		typed   string
		present []string
		absent  []string
	}{
		{"    p.", []string{"x", "y", "scale", "norm"}, []string{"unrelated", "first", "print"}},
		{"    pts.", []string{"len", "first", "push"}, []string{"norm", "x"}},
		{"    p.scale(2).", []string{"x", "y", "norm"}, []string{"len"}},
		{"    p.scale(2).norm().", nil, []string{"x", "norm"}},
	}
	for _, tc := range cases {
		items := c.completeTyping(f, "let op = ", tc.typed)
		expect(t, items, tc.present, tc.absent)
	}
}

func TestMethodSignatureAndNavigation(t *testing.T) {
	c := start(t)
	f := scratch(t, "methods.low", methodsSource)
	c.open(f, f.src)
	scale := c.signature(f, "let op = ", "    p.scale(")
	if scale == nil || activeParam(scale) != "by: i64" {
		t.Errorf("p.scale( help = %+v", scale)
	}
	apply := c.signature(f, "let op = ", "    op.apply(")
	if apply == nil || activeParam(apply) != "i64" {
		t.Errorf("op.apply( help = %+v", apply)
	}
	c.change(f, f.src)
	if got := c.hover(f, f.at(t, "print(p.norm()", 6)); !strings.Contains(got, "let p: Point") {
		t.Errorf("hover on receiver = %q", got)
	}
	loc := c.definition(f, f.at(t, "p.norm()", 2))
	if loc.Range.Start != f.at(t, "fn norm", 3) {
		t.Errorf("definition of norm = %+v", loc.Range.Start)
	}
}

const importedMethodsSource = `import std/http

fn host_of(u: http.Url) -> str {
    u.host
}

fn show(u: http.Url) -> str {
    u.authority()
}

fn main() {
    print(show(http.Url { scheme: "http", host: "a", port: 80, target: "/" }))
}
`

func TestImportedMethods(t *testing.T) {
	c := start(t)
	f := scratch(t, "imported.low", importedMethodsSource)
	if got := c.open(f, f.src); len(got) != 0 {
		t.Fatalf("diagnostics = %+v", got)
	}
	items := c.completeTyping(f, "fn show(u: http.Url) -> str {", "    u.")
	expect(t, items, []string{"host", "authority", "host_of"}, []string{"default_port", "parse_url"})
	help := c.signature(f, "fn show(u: http.Url) -> str {", "    u.authority(")
	if help == nil {
		t.Errorf("u.authority( help = %+v", help)
	}
}

const constrainedSource = `struct Point {
    x: i64
    y: i64
}

fn biggest[T: Ord](xs: []T) -> T {
    var best = xs[0]
    for x in xs {
        if x > best {
            best = x
        }
    }
    best
}

fn first[T](xs: []T) -> T {
    xs[0]
}

fn main() {
    let pts = [Point { x: 1, y: 2 }]
    let ints = [1, 2]
    print(pts.first().x + ints.biggest())
}
`

func TestConstrainedMethodCompletion(t *testing.T) {
	c := start(t)
	f := scratch(t, "constrained.low", constrainedSource)
	if got := c.open(f, f.src); len(got) != 0 {
		t.Fatalf("diagnostics = %+v", got)
	}
	points := c.completeTyping(f, "let ints = ", "    pts.")
	expect(t, points, []string{"first"}, []string{"biggest"})
	ints := c.completeTyping(f, "print(pts.first", "    ints.")
	expect(t, ints, []string{"first", "biggest"}, nil)
}

const localFuncSource = `struct Point {
    x: i64
}

fn main() {
    let p = Point { x: 1 }
    let show = fn(q: Point?) -> i64 { 0 }
    let other = fn(n: i64) -> i64 { n }
    print(p.show() + other(1))
}
`

func TestMethodCompletionOffersLocalsAcceptingTheReceiver(t *testing.T) {
	c := start(t)
	f := scratch(t, "localfn.low", localFuncSource)
	if got := c.open(f, f.src); len(got) != 0 {
		t.Fatalf("diagnostics = %+v", got)
	}
	items := c.completeTyping(f, "print(p.show", "    p.")
	expect(t, items, []string{"show"}, []string{"other"})
}
