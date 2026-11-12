package ast

import (
	"testing"

	"lowlang/internal/lexer"
)

func TestParseWidth(t *testing.T) {
	cases := []struct {
		text  string
		width int
		ok    bool
	}{
		{"13", 13, true},
		{"-6", -6, true},
		{" 6 ", 6, true},
		{"", 0, false},
		{"0", 0, false},
		{"-0", 0, false},
		{"- 6", 0, false},
		{"abc", 0, false},
		{"1.5", 0, false},
		{"--2", 0, false},
		{"99999999999999999999", 0, false},
	}
	for _, tc := range cases {
		width, ok := ParseWidth(tc.text)
		if width != tc.width || ok != tc.ok {
			t.Errorf("ParseWidth(%q) = %d, %v; want %d, %v", tc.text, width, ok, tc.width, tc.ok)
		}
	}
}

func TestParseStandardFormat(t *testing.T) {
	cases := []struct {
		text      string
		kind      FormatKind
		precision int
		explicit  bool
	}{
		{"d", DecimalFormat, 0, false},
		{"d3", DecimalFormat, 3, true},
		{"x8", HexFormat, 8, true},
		{"X", UpperHexFormat, 0, false},
		{"b12", BinaryFormat, 12, true},
		{"f2", FixedFormat, 2, true},
		{"e0", ScientificFormat, 0, true},
		{"n", GroupedFormat, 0, false},
		{"f99", FixedFormat, 99, true},
	}
	for _, tc := range cases {
		f, ok := ParseFormat(tc.text, lexer.Pos{})
		if !ok || f.Kind != tc.kind || f.Precision != tc.precision || f.Explicit != tc.explicit {
			t.Errorf("ParseFormat(%q) = %+v, %v", tc.text, f, ok)
		}
	}
}

func TestParsePicture(t *testing.T) {
	cases := []struct {
		text    string
		minInt  int
		minFrac int
		maxFrac int
		group   bool
	}{
		{"##", 0, 0, 0, false},
		{"0", 1, 0, 0, false},
		{"#.##", 0, 0, 2, false},
		{"0.0#", 1, 1, 2, false},
		{"#,##0.00", 1, 2, 2, true},
		{"0,000", 4, 0, 0, true},
		{".0#", 0, 1, 2, false},
		{"#.", 0, 0, 0, false},
	}
	for _, tc := range cases {
		f, ok := ParseFormat(tc.text, lexer.Pos{})
		if !ok || f.Kind != PictureFormat || f.MinInt != tc.minInt || f.MinFrac != tc.minFrac || f.MaxFrac != tc.maxFrac || f.Group != tc.group {
			t.Errorf("ParseFormat(%q) = %+v, %v", tc.text, f, ok)
		}
	}
}

func TestRejectFormats(t *testing.T) {
	for _, text := range []string{"", "q2", "f100", "fx", "f-1", "d 3", " f2", "0.0.0", "#.#%", ",.", ",", ".", "f2 ", "x00000"} {
		if f, ok := ParseFormat(text, lexer.Pos{}); ok {
			t.Errorf("ParseFormat(%q) = %+v, want rejection", text, f)
		}
	}
}

func TestIntegerOnly(t *testing.T) {
	for text, want := range map[string]bool{"d": true, "x": true, "X": true, "b": true, "f": false, "e": false, "n": false, "0.0": false} {
		f, ok := ParseFormat(text, lexer.Pos{})
		if !ok || f.IntegerOnly() != want {
			t.Errorf("ParseFormat(%q).IntegerOnly() = %v, want %v", text, f.IntegerOnly(), want)
		}
	}
}
