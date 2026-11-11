package asm

import "strings"

const maxMacroDepth = 64

type assembler struct {
	opts          Options
	syms          map[string]*symbol
	symOrder      []*symbol
	sections      map[string]*secBuilder
	secOrder      []*secBuilder
	cur           *secBuilder
	macros        map[string]*macro
	macroRuns     int
	macroDepth    int
	conds         []cond
	numericLabels map[int]int
	pending       []pendingDifference
	frame         *frame
	sc            scanner
	line          int
}

type cond struct {
	active       bool
	taken        bool
	parentActive bool
}

type frame struct {
	text string
	pos  int
	main bool
}

func newAssembler(opts Options) *assembler {
	a := &assembler{
		opts:          opts,
		syms:          make(map[string]*symbol, 1024),
		sections:      make(map[string]*secBuilder),
		macros:        make(map[string]*macro),
		numericLabels: make(map[int]int),
	}
	a.section(".text", "", false, false)
	a.section(".data", "", false, false)
	a.section(".bss", "", false, false)
	a.cur = a.sections[".text"]
	return a
}

func (f *frame) next() (string, bool) {
	if f.pos >= len(f.text) {
		return "", false
	}
	rest := f.text[f.pos:]
	end := strings.IndexByte(rest, '\n')
	line := rest
	if end < 0 {
		f.pos = len(f.text)
	} else {
		line = rest[:end]
		f.pos += end + 1
	}
	if n := len(line); n > 0 && line[n-1] == '\r' {
		line = line[:n-1]
	}
	return line, true
}

func (a *assembler) runFrame(f *frame) {
	saved := a.frame
	a.frame = f
	for {
		line, ok := f.next()
		if !ok {
			break
		}
		if f.main {
			a.line++
		}
		a.processLine(line)
	}
	a.frame = saved
}

func (a *assembler) processLine(line string) {
	a.sc = scanner{text: line}
	if !a.active() {
		a.skipLine()
		return
	}
	for a.statement() {
	}
}

func (a *assembler) skipLine() {
	s := &a.sc
	s.skipSpace()
	if s.peek() == '.' {
		name := s.ident()
		a.conditional(name)
	}
}

func (a *assembler) statement() bool {
	s := &a.sc
	if s.atEnd() {
		return false
	}
	if isDigit(s.peek()) {
		return a.numericLabel()
	}
	if !isIdentStart(s.peek()) {
		a.fail("unexpected %q", s.rest())
	}
	name := s.ident()
	if s.peek() == ':' {
		s.pos++
		a.defineLabel(name)
		return true
	}
	switch {
	case name[0] == '.':
		a.directive(name)
	case a.macros[name] != nil:
		a.expand(a.macros[name])
	default:
		a.instruction(name)
	}
	return false
}

func (a *assembler) numericLabel() bool {
	s := &a.sc
	start := s.pos
	for isDigit(s.peek()) {
		s.pos++
	}
	digits := s.text[start:s.pos]
	if s.peek() != ':' {
		a.fail("unexpected %q", s.text[start:])
	}
	s.pos++
	n := 0
	for _, d := range []byte(digits) {
		n = n*10 + int(d-'0')
	}
	a.defineNumericLabel(n)
	return true
}

func (a *assembler) active() bool {
	return len(a.conds) == 0 || a.conds[len(a.conds)-1].active
}

func (a *assembler) conditional(name string) bool {
	switch name {
	case ".if":
		parent := a.active()
		c := cond{parentActive: parent}
		if parent {
			c.active = a.constExpr() != 0
			c.taken = c.active
		}
		a.conds = append(a.conds, c)
	case ".else":
		top := a.topCond()
		top.active = top.parentActive && !top.taken
		top.taken = top.taken || top.active
	case ".endif":
		a.topCond()
		a.conds = a.conds[:len(a.conds)-1]
	default:
		return false
	}
	return true
}

func (a *assembler) topCond() *cond {
	if len(a.conds) == 0 {
		a.fail("conditional directive without .if")
	}
	return &a.conds[len(a.conds)-1]
}
