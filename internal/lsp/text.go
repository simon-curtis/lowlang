package lsp

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"lowlang/internal/lexer"
)

type text struct {
	lines []string
}

func newText(src []byte) text {
	return text{lines: strings.Split(string(src), "\n")}
}

func (t text) line(n int) string {
	if n < 0 || n >= len(t.lines) {
		return ""
	}
	return t.lines[n]
}

func (t text) utf16(line, byteCol int) int {
	src := t.line(line)
	byteCol = min(max(byteCol, 0), len(src))
	units := 0
	for _, r := range src[:byteCol] {
		units += utf16Len(r)
	}
	return units
}

func (t text) byteCol(line, utf16Col int) int {
	src := t.line(line)
	units := 0
	for i, r := range src {
		if units >= utf16Col {
			return i
		}
		units += utf16Len(r)
	}
	return len(src)
}

func utf16Len(r rune) int {
	if r >= 0x10000 {
		return 2
	}
	return 1
}

func (t text) position(p lexer.Pos) Position {
	line := p.Line - 1
	return Position{Line: line, Character: t.utf16(line, p.Col-1)}
}

func (t text) span(p lexer.Pos, length int) Range {
	end := lexer.Pos{File: p.File, Line: p.Line, Col: p.Col + length}
	return Range{Start: t.position(p), End: t.position(end)}
}

func (t text) word(p lexer.Pos) Range {
	src := t.line(p.Line - 1)
	start := min(max(p.Col-1, 0), len(src))
	end := start
	for end < len(src) && isWord(src[end]) {
		end++
	}
	if end == start {
		end = min(start+1, len(src))
	}
	return t.span(p, max(end-start, 1))
}

func (t text) nameAfter(p lexer.Pos, name string) lexer.Pos {
	src := t.line(p.Line - 1)
	from := min(max(p.Col-1, 0), len(src))
	for {
		i := strings.Index(src[from:], name)
		if i < 0 {
			return p
		}
		at := from + i
		end := at + len(name)
		before := at == 0 || !isWord(src[at-1])
		after := end == len(src) || !isWord(src[end])
		if before && after {
			return lexer.Pos{File: p.File, Line: p.Line, Col: at + 1}
		}
		from = at + 1
	}
}

func isWord(c byte) bool {
	return c == '_' || c >= utf8.RuneSelf || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

func pathOf(uri string) string {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "file" {
		return uri
	}
	p := u.Path
	if u.Host != "" && u.Host != "localhost" {
		p = fmt.Sprintf("//%s%s", u.Host, p)
	}
	rooted := strings.TrimPrefix(p, "/")
	if drive := filepath.VolumeName(rooted); len(drive) == 2 {
		p = fmt.Sprintf("%s%s", strings.ToUpper(drive), rooted[2:])
	}
	return filepath.Clean(filepath.FromSlash(p))
}

func uriOf(path string) string {
	volume := filepath.VolumeName(path)
	p := filepath.ToSlash(path)
	u := url.URL{Scheme: "file", Path: p}
	switch {
	case strings.HasPrefix(volume, `\\`):
		host, rest, _ := strings.Cut(strings.TrimPrefix(p, "//"), "/")
		u.Host, u.Path = host, fmt.Sprintf("/%s", rest)
	case volume != "":
		u.Path = fmt.Sprintf("/%s", p)
	}
	return u.String()
}
