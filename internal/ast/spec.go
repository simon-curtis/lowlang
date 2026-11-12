package ast

import (
	"strconv"
	"strings"

	"lowlang/internal/lexer"
)

type FormatKind byte

const (
	DecimalFormat    FormatKind = 'd'
	HexFormat        FormatKind = 'x'
	UpperHexFormat   FormatKind = 'X'
	BinaryFormat     FormatKind = 'b'
	FixedFormat      FormatKind = 'f'
	ScientificFormat FormatKind = 'e'
	GroupedFormat    FormatKind = 'n'
	PictureFormat    FormatKind = 'p'
)

type Format struct {
	Kind      FormatKind
	Text      string
	Pos       lexer.Pos
	Precision int
	Explicit  bool
	MinInt    int
	MinFrac   int
	MaxFrac   int
	Group     bool
}

type Spec struct {
	Width    int
	WidthPos lexer.Pos
	Format   *Format
}

func (f *Format) IntegerOnly() bool {
	switch f.Kind {
	case DecimalFormat, HexFormat, UpperHexFormat, BinaryFormat:
		return true
	}
	return false
}

func ParseWidth(text string) (int, bool) {
	text = strings.TrimSpace(text)
	digits := strings.TrimPrefix(text, "-")
	if digits == "" || strings.Trim(digits, "0123456789") != "" {
		return 0, false
	}
	width, err := strconv.Atoi(text)
	if err != nil || width == 0 {
		return 0, false
	}
	return width, true
}

func ParseFormat(text string, pos lexer.Pos) (*Format, bool) {
	if text == "" {
		return nil, false
	}
	f := &Format{Text: text, Pos: pos}
	letter := text[0]
	ok := false
	if letter >= 'a' && letter <= 'z' || letter >= 'A' && letter <= 'Z' {
		ok = f.standard(text)
	} else {
		ok = f.picture(text)
	}
	if !ok {
		return nil, false
	}
	return f, true
}

func (f *Format) standard(text string) bool {
	kind := FormatKind(text[0])
	switch kind {
	case DecimalFormat, HexFormat, UpperHexFormat, BinaryFormat, FixedFormat, ScientificFormat, GroupedFormat:
	default:
		return false
	}
	f.Kind = kind
	rest := text[1:]
	if rest == "" {
		return true
	}
	if strings.Trim(rest, "0123456789") != "" || len(rest) > 4 {
		return false
	}
	precision, err := strconv.Atoi(rest)
	if err != nil || precision > 99 {
		return false
	}
	f.Precision, f.Explicit = precision, true
	return true
}

func (f *Format) picture(text string) bool {
	if strings.Trim(text, "0#,.") != "" || strings.Count(text, ".") > 1 {
		return false
	}
	whole, fraction, _ := strings.Cut(text, ".")
	if !strings.ContainsAny(text, "0#") {
		return false
	}
	f.Kind = PictureFormat
	f.MinInt = strings.Count(whole, "0")
	f.MinFrac = strings.Count(fraction, "0")
	f.MaxFrac = f.MinFrac + strings.Count(fraction, "#")
	f.Group = grouped(whole)
	return true
}

func grouped(whole string) bool {
	first := strings.IndexAny(whole, "0#")
	last := strings.LastIndexAny(whole, "0#")
	if first < 0 {
		return false
	}
	return strings.Contains(whole[first:last+1], ",")
}
