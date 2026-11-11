package asm

type scanner struct {
	text string
	pos  int
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

func isLetter(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

func isIdentStart(c byte) bool {
	return isLetter(c) || c == '_' || c == '.' || c == '$'
}

func isIdentChar(c byte) bool {
	return isIdentStart(c) || isDigit(c)
}

func isHexDigit(c byte) bool {
	return isDigit(c) || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

func (s *scanner) skipSpace() {
	for s.pos < len(s.text) && (s.text[s.pos] == ' ' || s.text[s.pos] == '\t') {
		s.pos++
	}
}

func (s *scanner) peek() byte {
	if s.pos < len(s.text) {
		return s.text[s.pos]
	}
	return 0
}

func (s *scanner) peekAt(offset int) byte {
	if s.pos+offset < len(s.text) {
		return s.text[s.pos+offset]
	}
	return 0
}

func (s *scanner) atEnd() bool {
	s.skipSpace()
	return s.pos >= len(s.text) || s.text[s.pos] == '#'
}

func (s *scanner) accept(c byte) bool {
	s.skipSpace()
	if s.pos < len(s.text) && s.text[s.pos] == c {
		s.pos++
		return true
	}
	return false
}

func (s *scanner) ident() string {
	start := s.pos
	for s.pos < len(s.text) && isIdentChar(s.text[s.pos]) {
		s.pos++
	}
	return s.text[start:s.pos]
}

func (s *scanner) rest() string {
	return s.text[s.pos:]
}

func (s *scanner) plainInt() (int64, bool) {
	start := s.pos
	negative := false
	if s.peek() == '-' {
		negative = true
		s.pos++
	}
	digits := s.pos
	var n uint64
	for s.pos < len(s.text) && isDigit(s.text[s.pos]) {
		n = n*10 + uint64(s.text[s.pos]-'0')
		s.pos++
	}
	count := s.pos - digits
	after := s.pos
	for after < len(s.text) && (s.text[after] == ' ' || s.text[after] == '\t') {
		after++
	}
	terminated := after >= len(s.text) || s.text[after] == ',' || s.text[after] == '#'
	plain := count > 0 && count < 19 && (s.text[digits] != '0' || count == 1) && terminated
	if !plain {
		s.pos = start
		return 0, false
	}
	if negative {
		return -int64(n), true
	}
	return int64(n), true
}
