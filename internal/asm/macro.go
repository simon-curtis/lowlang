package asm

import (
	"strconv"
	"strings"
)

type macro struct {
	name   string
	params []string
	body   string
}

func (a *assembler) defineMacro() {
	s := &a.sc
	s.skipSpace()
	name := s.ident()
	if name == "" {
		a.fail(".macro needs a name")
	}
	fields := strings.FieldsFunc(s.rest(), func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t'
	})
	for _, field := range fields {
		if strings.ContainsAny(field, ":=") {
			a.fail("macro parameter %q uses an unsupported qualifier", field)
		}
	}
	a.macros[name] = &macro{name: name, params: fields, body: a.macroBody()}
}

func (a *assembler) macroBody() string {
	f := a.frame
	start := f.pos
	depth := 0
	for {
		lineStart := f.pos
		line, ok := f.next()
		if !ok {
			a.fail("missing .endm")
		}
		if f.main {
			a.line++
		}
		switch firstWord(line) {
		case ".macro":
			depth++
		case ".endm":
			if depth == 0 {
				return f.text[start:lineStart]
			}
			depth--
		}
	}
}

func firstWord(line string) string {
	line = strings.TrimLeft(line, " \t")
	end := strings.IndexAny(line, " \t")
	if end < 0 {
		return line
	}
	return line[:end]
}

func (a *assembler) expand(m *macro) {
	args := a.macroArgs()
	if len(args) > len(m.params) {
		a.fail("macro %s takes %d arguments, got %d", m.name, len(m.params), len(args))
	}
	if a.macroDepth >= maxMacroDepth {
		a.fail("macro %s nests too deeply", m.name)
	}
	a.macroRuns++
	text := m.substitute(args, a.macroRuns)
	a.macroDepth++
	a.runFrame(&frame{text: text})
	a.macroDepth--
}

func (a *assembler) macroArgs() []string {
	rest := strings.TrimSpace(a.sc.rest())
	if rest == "" {
		return nil
	}
	parts := strings.Split(rest, ",")
	for i, part := range parts {
		parts[i] = strings.TrimSpace(part)
	}
	return parts
}

func (m *macro) param(name string) (int, bool) {
	for i, p := range m.params {
		if p == name {
			return i, true
		}
	}
	return 0, false
}

func (m *macro) substitute(args []string, run int) string {
	body := m.body
	if strings.IndexByte(body, '\\') < 0 {
		return body
	}
	var out strings.Builder
	out.Grow(len(body) + 16)
	for len(body) > 0 {
		slash := strings.IndexByte(body, '\\')
		if slash < 0 {
			out.WriteString(body)
			break
		}
		out.WriteString(body[:slash])
		body = body[slash+1:]
		body = m.expandEscape(&out, body, args, run)
	}
	return out.String()
}

func (m *macro) expandEscape(out *strings.Builder, body string, args []string, run int) string {
	switch {
	case strings.HasPrefix(body, "@"):
		out.WriteString(strconv.Itoa(run))
		return body[1:]
	case strings.HasPrefix(body, "()"):
		return body[2:]
	}
	end := 0
	for end < len(body) && (isLetter(body[end]) || isDigit(body[end]) || body[end] == '_') {
		end++
	}
	index, ok := m.param(body[:end])
	if !ok {
		out.WriteByte('\\')
		return body
	}
	if index < len(args) {
		out.WriteString(args[index])
	}
	return body[end:]
}
