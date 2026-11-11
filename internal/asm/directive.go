package asm

import (
	"math"
	"os"
	"strconv"
	"strings"
)

func (a *assembler) directive(name string) {
	switch name {
	case ".quad":
		a.dataList(8)
	case ".long":
		a.dataList(4)
	case ".short":
		a.dataList(2)
	case ".byte":
		a.dataList(1)
	case ".zero", ".space":
		a.zeroFill()
	case ".p2align":
		a.alignDirective()
	case ".set":
		a.setSymbol()
	case ".globl", ".global":
		a.globals()
	case ".text", ".data", ".bss":
		a.section(name, "", false, false)
		a.expectEnd()
	case ".section":
		a.sectionDirective()
	case ".double":
		a.doubleList()
	case ".ascii":
		a.stringList(false)
	case ".asciz":
		a.stringList(true)
	case ".incbin":
		a.incbin()
	case ".intel_syntax":
		a.intelSyntax()
	case ".macro":
		a.defineMacro()
	default:
		if !a.conditional(name) {
			a.fail("unsupported directive %s", name)
		}
	}
}

func (a *assembler) expectEnd() {
	if !a.sc.atEnd() {
		a.fail("unexpected %q", a.sc.rest())
	}
}

func (a *assembler) intelSyntax() {
	rest := strings.TrimSpace(a.sc.rest())
	if rest != "noprefix" {
		a.fail(".intel_syntax supports only noprefix, got %q", rest)
	}
}

func (a *assembler) alignDirective() {
	s := &a.sc
	power := a.constExpr()
	var max int64
	if s.accept(',') {
		s.skipSpace()
		if s.peek() != ',' {
			a.fail(".p2align with a fill value is not supported")
		}
		s.pos++
		max = a.constExpr()
	}
	a.emitAlign(power, max)
	a.expectEnd()
}

func (a *assembler) markConstantData() {
	if a.cur.exec {
		a.cur.last = lastDirective
	}
}

func (a *assembler) dataList(size int) {
	s := &a.sc
	a.markConstantData()
	for {
		s.skipSpace()
		if n, ok := s.plainInt(); ok {
			a.emitConst(size, n)
		} else {
			v := a.parseExpr()
			a.emitValue(size, v)
		}
		if !s.accept(',') {
			break
		}
	}
	a.expectEnd()
}

func (a *assembler) zeroFill() {
	a.markConstantData()
	n := a.constExpr()
	fill := int64(0)
	if a.sc.accept(',') {
		fill = a.constExpr()
	}
	a.emitZero(n, byte(fill))
	a.expectEnd()
}

func (a *assembler) setSymbol() {
	s := &a.sc
	s.skipSpace()
	name := s.ident()
	if name == "" || !s.accept(',') {
		a.fail("malformed .set")
	}
	v := a.parseExpr()
	if !v.IsConst() {
		a.fail(".set %s needs a constant value", name)
	}
	a.defineConstant(name, v.Add)
	a.expectEnd()
}

func (a *assembler) globals() {
	s := &a.sc
	for {
		s.skipSpace()
		name := s.ident()
		if name == "" {
			a.fail(".globl needs a symbol")
		}
		a.lookup(name).global = true
		if !s.accept(',') {
			break
		}
	}
	a.expectEnd()
}

func (a *assembler) sectionDirective() {
	s := &a.sc
	s.skipSpace()
	name := s.ident()
	if name == "" {
		a.fail(".section needs a name")
	}
	flags, noBits, hasFlags := "", false, false
	if s.accept(',') {
		hasFlags = true
		flags = a.stringLiteral()
		for s.accept(',') {
			s.skipSpace()
			if s.peek() != '@' {
				a.fail("unexpected %q in .section", s.rest())
			}
			s.pos++
			typeName := s.ident()
			noBits = a.sectionType(typeName)
		}
	}
	a.section(name, flags, hasFlags, noBits)
	a.expectEnd()
}

func (a *assembler) sectionType(name string) bool {
	switch name {
	case "nobits":
		return true
	case "progbits":
		return false
	}
	a.fail("unsupported section type @%s", name)
	return false
}

func (a *assembler) doubleList() {
	s := &a.sc
	a.markConstantData()
	for {
		s.skipSpace()
		start := s.pos
		a.scanFloat()
		text := s.text[start:s.pos]
		f, err := strconv.ParseFloat(text, 64)
		if err != nil {
			a.failWrap(err, "bad floating point value %q", text)
		}
		a.emitConst(8, int64(math.Float64bits(f)))
		if !s.accept(',') {
			break
		}
	}
	a.expectEnd()
}

func (a *assembler) scanFloat() {
	s := &a.sc
	if c := s.peek(); c == '-' || c == '+' {
		s.pos++
	}
	for isDigit(s.peek()) || s.peek() == '.' {
		s.pos++
	}
	if c := s.peek(); c == 'e' || c == 'E' {
		s.pos++
		if c := s.peek(); c == '-' || c == '+' {
			s.pos++
		}
		for isDigit(s.peek()) {
			s.pos++
		}
	}
}

func (a *assembler) stringList(terminate bool) {
	s := &a.sc
	a.markConstantData()
	for {
		text := a.stringLiteral()
		a.emitString(text)
		if terminate {
			a.emitConst(1, 0)
		}
		if !s.accept(',') {
			break
		}
	}
	a.expectEnd()
}

func (a *assembler) stringLiteral() string {
	s := &a.sc
	s.skipSpace()
	if s.peek() != '"' {
		a.fail("expected a string, got %q", s.rest())
	}
	s.pos++
	start := s.pos
	for s.pos < len(s.text) && s.text[s.pos] != '"' && s.text[s.pos] != '\\' {
		s.pos++
	}
	if s.peek() == '"' {
		text := s.text[start:s.pos]
		s.pos++
		return text
	}
	return a.escapedString(start)
}

func (a *assembler) escapedString(start int) string {
	s := &a.sc
	var out strings.Builder
	out.WriteString(s.text[start:s.pos])
	for s.pos < len(s.text) {
		c := s.text[s.pos]
		s.pos++
		switch c {
		case '"':
			return out.String()
		case '\\':
			escaped := a.escapeByte()
			out.WriteByte(escaped)
		default:
			out.WriteByte(c)
		}
	}
	a.fail("unterminated string")
	return ""
}

func (a *assembler) escapeByte() byte {
	s := &a.sc
	if s.pos >= len(s.text) {
		a.fail("unterminated string")
	}
	c := s.text[s.pos]
	s.pos++
	switch c {
	case 'n':
		return '\n'
	case 't':
		return '\t'
	case 'r':
		return '\r'
	case 'b':
		return '\b'
	case 'f':
		return '\f'
	case '\\', '"':
		return c
	case 'x':
		return a.numericEscape(16, 2)
	}
	if c >= '0' && c <= '7' {
		s.pos--
		return a.numericEscape(8, 3)
	}
	a.fail("unknown escape \\%c", c)
	return 0
}

func (a *assembler) numericEscape(base uint64, maxDigits int) byte {
	s := &a.sc
	var n uint64
	for count := 0; count < maxDigits; count++ {
		d, ok := digitValue(s.peek())
		if !ok || d >= base {
			break
		}
		n = n*base + d
		s.pos++
	}
	return byte(n)
}

func (a *assembler) incbin() {
	a.markConstantData()
	path := a.stringLiteral()
	data, err := os.ReadFile(path)
	if err != nil {
		a.failWrap(err, "cannot read .incbin %q", path)
	}
	a.emitBytes(data)
	a.expectEnd()
}
