package asm

import (
	"fmt"
	"strconv"
)

const gotSymbol = "_GLOBAL_OFFSET_TABLE_"

const (
	rankAdditive = 1
	rankBitwise  = 2
	rankMultiply = 3
)

func binaryRank(op string) int {
	switch op {
	case "*", "/", "%", "<<", ">>":
		return rankMultiply
	case "|", "&", "^":
		return rankBitwise
	case "+", "-":
		return rankAdditive
	}
	return 0
}

func (a *assembler) peekOperator() string {
	s := &a.sc
	s.skipSpace()
	c := s.peek()
	switch c {
	case '*', '/', '%', '|', '&', '^', '+', '-':
		return s.text[s.pos : s.pos+1]
	case '<', '>':
		if s.peekAt(1) == c {
			return s.text[s.pos : s.pos+2]
		}
	}
	return ""
}

func (a *assembler) parseExpr() Value {
	return a.parseRank(rankAdditive)
}

func (a *assembler) parseRank(minRank int) Value {
	left := a.parseUnary()
	for {
		op := a.peekOperator()
		rank := binaryRank(op)
		if rank == 0 || rank < minRank {
			return left
		}
		a.sc.pos += len(op)
		right := a.parseRank(rank + 1)
		left = a.binary(op, left, right)
	}
}

func (a *assembler) binary(op string, l, r Value) Value {
	switch op {
	case "+":
		return a.combine(l, r)
	case "-":
		negated := r.negated()
		return a.combine(l, negated)
	}
	if !l.IsConst() || !r.IsConst() {
		a.fail("operator %s needs constant operands", op)
	}
	x, y := l.Add, r.Add
	switch op {
	case "*":
		return Value{Add: x * y}
	case "/":
		if y == 0 {
			a.fail("division by zero")
		}
		return Value{Add: x / y}
	case "%":
		if y == 0 {
			a.fail("division by zero")
		}
		return Value{Add: x % y}
	case "<<":
		return Value{Add: x << uint64(y)}
	case ">>":
		return Value{Add: x >> uint64(y)}
	case "|":
		return Value{Add: x | y}
	case "&":
		return Value{Add: x & y}
	}
	return Value{Add: x ^ y}
}

func (a *assembler) combine(l, r Value) Value {
	sum, ok := l.plus(r)
	if !ok {
		a.fail("expression has too many symbols")
	}
	return sum
}

func (a *assembler) parseUnary() Value {
	s := &a.sc
	s.skipSpace()
	switch s.peek() {
	case '-':
		s.pos++
		return a.parseUnary().negated()
	case '+':
		s.pos++
		return a.parseUnary()
	case '~':
		s.pos++
		v := a.parseUnary()
		if !v.IsConst() {
			a.fail("operator ~ needs a constant operand")
		}
		return Value{Add: ^v.Add}
	case '(':
		s.pos++
		v := a.parseExpr()
		if !s.accept(')') {
			a.fail("missing )")
		}
		return v
	}
	return a.parsePrimary()
}

func (a *assembler) parsePrimary() Value {
	s := &a.sc
	c := s.peek()
	switch {
	case isDigit(c):
		return a.parseNumber()
	case isIdentStart(c):
		name := s.ident()
		return a.symbolValue(name)
	}
	a.fail("unexpected %q in expression", s.rest())
	return Value{}
}

func (a *assembler) symbolValue(name string) Value {
	sym := a.lookup(name)
	if sym.state == symAbsolute {
		return Value{Add: sym.value}
	}
	v := Value{Sym: name}
	s := &a.sc
	if s.peek() == '@' {
		s.pos++
		suffix := s.ident()
		v.Reloc = a.relocSuffix(suffix)
	}
	return v
}

func (a *assembler) relocSuffix(name string) Reloc {
	switch name {
	case "GOTPCREL":
		a.lookup(gotSymbol)
		return RelocGOTPCREL
	case "IMGREL":
		return RelocImgRel
	case "PLT":
		return RelocPLT
	}
	a.fail("unknown relocation suffix @%s", name)
	return RelocNone
}

func (a *assembler) parseNumber() Value {
	s := &a.sc
	start := s.pos
	if s.peek() == '0' && (s.peekAt(1) == 'x' || s.peekAt(1) == 'X') && isHexDigit(s.peekAt(2)) {
		s.pos += 2
		return Value{Add: a.digits(16)}
	}
	if s.peek() == '0' && (s.peekAt(1) == 'b' || s.peekAt(1) == 'B') && (s.peekAt(2) == '0' || s.peekAt(2) == '1') {
		s.pos += 2
		return Value{Add: a.digits(2)}
	}
	for isDigit(s.peek()) {
		s.pos++
	}
	digits := s.text[start:s.pos]
	if next := s.peek(); (next == 'b' || next == 'f') && !isIdentChar(s.peekAt(1)) {
		s.pos++
		return a.localLabelRef(digits, next == 'f')
	}
	if len(digits) > 1 && digits[0] == '0' {
		n, err := strconv.ParseUint(digits, 8, 64)
		if err != nil {
			a.failWrap(err, "bad octal number %q", digits)
		}
		return Value{Add: int64(n)}
	}
	n, err := strconv.ParseUint(digits, 10, 64)
	if err != nil {
		a.failWrap(err, "bad number %q", digits)
	}
	return Value{Add: int64(n)}
}

func (a *assembler) digits(base uint64) int64 {
	s := &a.sc
	var n uint64
	count := 0
	for {
		d, ok := digitValue(s.peek())
		if !ok || d >= base {
			break
		}
		n = n*base + d
		s.pos++
		count++
	}
	if count == 0 {
		a.fail("missing digits")
	}
	return int64(n)
}

func digitValue(c byte) (uint64, bool) {
	switch {
	case isDigit(c):
		return uint64(c - '0'), true
	case c >= 'a' && c <= 'f':
		return uint64(c-'a') + 10, true
	case c >= 'A' && c <= 'F':
		return uint64(c-'A') + 10, true
	}
	return 0, false
}

func (a *assembler) localLabelRef(digits string, forward bool) Value {
	n, err := strconv.Atoi(digits)
	if err != nil {
		a.failWrap(err, "bad local label %q", digits)
	}
	count := a.numericLabels[n]
	if forward {
		count++
	} else if count == 0 {
		a.fail("local label %db has no definition", n)
	}
	name := fmt.Sprintf(".L%d\x02%d", n, count)
	a.lookup(name)
	return Value{Sym: name}
}

func (a *assembler) constExpr() int64 {
	v := a.parseExpr()
	if !v.IsConst() {
		a.fail("expression is not a constant")
	}
	return v.Add
}
