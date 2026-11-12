package ast

import (
	"testing"

	"lowlang/internal/diag"
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
		width, ok := parseWidth(tc.text)
		if width != tc.width || ok != tc.ok {
			t.Errorf("parseWidth(%q) = %d, %v; want %d, %v", tc.text, width, ok, tc.width, tc.ok)
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
		f, ok := parseFormat(tc.text, diag.Pos{})
		if !ok || f.Kind != tc.kind || f.Precision != tc.precision || f.Explicit != tc.explicit {
			t.Errorf("parseFormat(%q) = %+v, %v", tc.text, f, ok)
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
		f, ok := parseFormat(tc.text, diag.Pos{})
		if !ok || f.Kind != PictureFormat || f.MinInt != tc.minInt || f.MinFrac != tc.minFrac || f.MaxFrac != tc.maxFrac || f.Group != tc.group {
			t.Errorf("parseFormat(%q) = %+v, %v", tc.text, f, ok)
		}
	}
}

func TestRejectFormats(t *testing.T) {
	for _, text := range []string{"", "q2", "f100", "fx", "f-1", "d 3", " f2", "0.0.0", "#.#%", ",.", ",", ".", "f2 ", "x00000"} {
		if f, ok := parseFormat(text, diag.Pos{}); ok {
			t.Errorf("parseFormat(%q) = %+v, want rejection", text, f)
		}
	}
}

func TestParseSpec(t *testing.T) {
	at := diag.Pos{File: "spec", Line: 3, Col: 10}
	cases := []struct {
		text      string
		width     int
		widthCol  int
		format    string
		formatCol int
	}{
		{text: ":x", format: "x", formatCol: 11},
		{text: ",8", width: 8, widthCol: 11},
		{text: ",-8:#,##0", width: -8, widthCol: 11, format: "#,##0", formatCol: 14},
		{text: ",  4:f2", width: 4, widthCol: 13, format: "f2", formatCol: 15},
	}
	for _, tc := range cases {
		spec, failure := ParseSpec(tc.text, at)
		if failure != nil {
			t.Errorf("ParseSpec(%q) failed: %v", tc.text, failure)
			continue
		}
		if spec.Width != tc.width || (tc.width != 0 && spec.WidthPos.Col != tc.widthCol) {
			t.Errorf("ParseSpec(%q) width %d at %d, want %d at %d", tc.text, spec.Width, spec.WidthPos.Col, tc.width, tc.widthCol)
		}
		if tc.format == "" {
			if spec.Format != nil {
				t.Errorf("ParseSpec(%q) has format %+v", tc.text, spec.Format)
			}
			continue
		}
		if spec.Format == nil || spec.Format.Text != tc.format || spec.Format.Pos.Col != tc.formatCol {
			t.Errorf("ParseSpec(%q) format %+v, want %q at %d", tc.text, spec.Format, tc.format, tc.formatCol)
		}
	}
}

func TestParseSpecRejects(t *testing.T) {
	at := diag.Pos{File: "spec", Line: 1, Col: 5}
	cases := map[string]string{
		",":    `invalid width ""`,
		",0":   `invalid width "0"`,
		",a:x": `invalid width "a"`,
		":":    `invalid format ""`,
		",3:q": `invalid format "q"`,
	}
	for text, want := range cases {
		_, failure := ParseSpec(text, at)
		if failure == nil || failure.Msg != want {
			t.Errorf("ParseSpec(%q) = %v, want %s", text, failure, want)
		}
	}
}

func TestIntegerOnly(t *testing.T) {
	for text, want := range map[string]bool{"d": true, "x": true, "X": true, "b": true, "f": false, "e": false, "n": false, "0.0": false} {
		f, ok := parseFormat(text, diag.Pos{})
		if !ok || f.IntegerOnly() != want {
			t.Errorf("parseFormat(%q).IntegerOnly() = %v, want %v", text, f.IntegerOnly(), want)
		}
	}
}
