package lsp

import (
	"strings"
	"testing"
)

func TestDiagnosticsCarryTypedPositions(t *testing.T) {
	cases := []struct {
		name string
		src  string
		line int
		want string
	}{
		{"lex.low", "fn main() {\n    let s = \"open\n}\n", 1, "unterminated"},
		{"parse.low", "fn main() {\n    let = 1\n}\n", 1, "expected"},
		{"check.low", "fn main() {\n    let x: i64 = \"s\"\n}\n", 1, "i64"},
		{"module.low", "\n\nimport nowhere\n\nfn main() {\n}\n", 2, "cannot find module nowhere"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := start(t)
			f := scratch(t, tc.name, tc.src)
			got := c.open(f, f.src)
			if len(got) != 1 {
				t.Fatalf("diagnostics = %+v, want one", got)
			}
			if got[0].Range.Start.Line != tc.line {
				t.Errorf("line = %d, want %d (%s)", got[0].Range.Start.Line, tc.line, got[0].Message)
			}
			if !strings.Contains(got[0].Message, tc.want) {
				t.Errorf("message = %q, want it to mention %q", got[0].Message, tc.want)
			}
			if strings.HasPrefix(got[0].Message, f.path) {
				t.Errorf("message = %q still carries the position", got[0].Message)
			}
		})
	}
}

func TestMalformedNotificationIsLogged(t *testing.T) {
	c := start(t)
	c.notify("textDocument/didOpen", map[string]any{"textDocument": 5})
	c.request("shutdown", nil)
	if c.asked["window/logMessage"] != 1 {
		t.Errorf("window/logMessage sent %d times, want 1", c.asked["window/logMessage"])
	}
}

func TestMalformedRequestFails(t *testing.T) {
	c := start(t)
	_, failure := c.call("textDocument/hover", "position")
	if failure == nil || failure.Code != invalidParams {
		t.Errorf("failure = %+v, want invalid params", failure)
	}
}

func TestNameError(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{"count", ""},
		{"_count", ""},
		{"fn", "is a keyword"},
		{"2c", "not a valid name"},
		{"a b", "not a valid name"},
		{"", "not a valid name"},
		{"__c", "are reserved"},
	}
	for _, tc := range cases {
		err := nameError(tc.name)
		switch {
		case tc.want == "" && err != nil:
			t.Errorf("%q: %v", tc.name, err)
		case tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)):
			t.Errorf("%q: %v, want %q", tc.name, err, tc.want)
		}
	}
}

func TestHeaderName(t *testing.T) {
	cases := map[string]string{
		"fn main() {":              "main",
		"pub fn area(s: Shape) {":  "area",
		"    fn puts(s: str)":      "puts",
		"\tpub fn read(fd: i64)":   "read",
		"let f = fn(x: i64) { x }": "",
		"// fn main() {":           "",
		"    print(\"fn x\")":      "",
		"fn":                       "",
	}
	for line, want := range cases {
		if got := headerName(line); got != want {
			t.Errorf("headerName(%q) = %q, want %q", line, got, want)
		}
	}
}
