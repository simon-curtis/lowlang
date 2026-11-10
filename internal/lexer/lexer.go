package lexer

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

type Error struct {
	Pos Pos
	Msg string
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Pos, e.Msg)
}

type Lexer struct {
	src     []byte
	file    string
	off     int
	line    int
	col     int
	last    Kind
	tokens  []Token
	prelude bool
}

func Lex(src []byte, file string) ([]Token, error) {
	return LexAt(src, Pos{File: file, Line: 1, Col: 1}, false)
}

func LexPrelude(src []byte) ([]Token, error) {
	return LexAt(src, Pos{File: "prelude.low", Line: 1, Col: 1}, true)
}

func LexAt(src []byte, at Pos, prelude bool) ([]Token, error) {
	l := &Lexer{src: src, file: at.File, line: at.Line, col: at.Col, last: Newline, prelude: prelude}
	if err := l.run(); err != nil {
		return nil, err
	}
	return l.tokens, nil
}

func (l *Lexer) run() error {
	for {
		l.skipSpace()
		if l.off >= len(l.src) {
			l.emitNewline(l.pos())
			l.emit(Token{Kind: EOF, Pos: l.pos()})
			return nil
		}
		if err := l.next(); err != nil {
			return err
		}
	}
}

func (l *Lexer) pos() Pos {
	return Pos{File: l.file, Line: l.line, Col: l.col}
}

func (l *Lexer) peek(ahead int) byte {
	if l.off+ahead >= len(l.src) {
		return 0
	}
	return l.src[l.off+ahead]
}

func (l *Lexer) advance() byte {
	c := l.src[l.off]
	l.off++
	if c == '\n' {
		l.line++
		l.col = 1
	} else {
		l.col++
	}
	return c
}

func (l *Lexer) emit(t Token) {
	l.tokens = append(l.tokens, t)
	l.last = t.Kind
}

func (l *Lexer) emitNewline(p Pos) {
	switch l.last {
	case Ident, Int, Float, Char, String, True, False, Break, Continue, Return, RParen, RBrace, RBracket, Question:
		l.emit(Token{Kind: Newline, Pos: p})
	}
}

func (l *Lexer) continued() bool {
	at := l.off
	for at < len(l.src) {
		switch c := l.src[at]; {
		case c == ' ' || c == '\t' || c == '\r' || c == '\n':
			at++
		case c == '/' && at+1 < len(l.src) && l.src[at+1] == '/':
			for at < len(l.src) && l.src[at] != '\n' {
				at++
			}
		case c == '|':
			return at+1 < len(l.src) && l.src[at+1] == '>'
		case c == '.':
			return at+1 >= len(l.src) || l.src[at+1] != '.'
		default:
			return false
		}
	}
	return false
}

func (l *Lexer) skipSpace() {
	for l.off < len(l.src) {
		switch c := l.peek(0); {
		case c == ' ' || c == '\t' || c == '\r':
			l.advance()
		case c == '/' && l.peek(1) == '/':
			for l.off < len(l.src) && l.peek(0) != '\n' {
				l.advance()
			}
		default:
			return
		}
	}
}

func (l *Lexer) next() error {
	start := l.pos()
	c := l.peek(0)
	switch {
	case c == '\n':
		if !l.continued() {
			l.emitNewline(start)
		}
		l.advance()
		return nil
	case isLetter(c):
		return l.ident(start)
	case isDigit(c):
		l.number(start)
		return nil
	case c == '"':
		return l.string(start)
	case c == '\'':
		return l.char(start)
	}
	return l.operator(start)
}

func (l *Lexer) ident(start Pos) error {
	from := l.off
	for l.off < len(l.src) && (isLetter(l.peek(0)) || isDigit(l.peek(0))) {
		l.advance()
	}
	text := string(l.src[from:l.off])
	if strings.HasPrefix(text, "__") && !l.prelude {
		return &Error{Pos: start, Msg: "names starting with __ are reserved"}
	}
	kind, ok := keywords[text]
	if !ok {
		kind = Ident
	}
	l.emit(Token{Kind: kind, Text: text, Pos: start})
	return nil
}

func (l *Lexer) number(start Pos) {
	from := l.off
	kind := Int
	if l.peek(0) == '0' && (l.peek(1) == 'x' || l.peek(1) == 'b' || l.peek(1) == 'o') {
		l.advance()
		l.advance()
		l.digits(isHexDigit)
		l.emitNumber(kind, from, start)
		return
	}
	l.digits(isDigit)
	if l.peek(0) == '.' && isDigit(l.peek(1)) {
		kind = Float
		l.advance()
		l.digits(isDigit)
	}
	if e := l.peek(0); e == 'e' || e == 'E' {
		kind = Float
		l.advance()
		if s := l.peek(0); s == '+' || s == '-' {
			l.advance()
		}
		l.digits(isDigit)
	}
	l.emitNumber(kind, from, start)
}

func (l *Lexer) digits(accept func(byte) bool) {
	for l.off < len(l.src) && (accept(l.peek(0)) || l.peek(0) == '_') {
		l.advance()
	}
}

func (l *Lexer) emitNumber(kind Kind, from int, start Pos) {
	text := strings.ReplaceAll(string(l.src[from:l.off]), "_", "")
	l.emit(Token{Kind: kind, Text: text, Pos: start})
}

var escapes = map[byte]byte{'n': '\n', 't': '\t', 'r': '\r', '0': 0, '\\': '\\', '"': '"', '\'': '\''}

func (l *Lexer) unterminated() bool {
	return l.off >= len(l.src) || l.peek(0) == '\n'
}

func (l *Lexer) escape(b *strings.Builder) error {
	pos := l.pos()
	l.advance()
	if l.unterminated() {
		return &Error{Pos: pos, Msg: "unterminated escape sequence"}
	}
	e, ok := escapes[l.advance()]
	if !ok {
		return &Error{Pos: pos, Msg: "unknown escape sequence"}
	}
	b.WriteByte(e)
	return nil
}

func (l *Lexer) string(start Pos) error {
	l.advance()
	var parts []Part
	var b strings.Builder
	for {
		if l.unterminated() {
			return &Error{Pos: start, Msg: "unterminated string"}
		}
		c, next := l.peek(0), l.peek(1)
		switch {
		case c == '"':
			l.advance()
			return l.emitString(start, parts, b.String())
		case (c == '{' && next == '{') || (c == '}' && next == '}'):
			l.advance()
			b.WriteByte(l.advance())
		case c == '}':
			return &Error{Pos: l.pos(), Msg: "unmatched } in string; write }}"}
		case c == '{':
			parts = append(parts, Part{Text: b.String()})
			b.Reset()
			part, err := l.interpolation()
			if err != nil {
				return err
			}
			parts = append(parts, part)
		case c == '\\':
			if err := l.escape(&b); err != nil {
				return err
			}
		default:
			b.WriteByte(l.advance())
		}
	}
}

func (l *Lexer) emitString(start Pos, parts []Part, tail string) error {
	if parts == nil {
		l.emit(Token{Kind: String, Text: tail, Pos: start})
		return nil
	}
	parts = append(parts, Part{Text: tail})
	l.emit(Token{Kind: String, Parts: parts, Pos: start})
	return nil
}

func (l *Lexer) interpolation() (Part, error) {
	open := l.pos()
	l.advance()
	pos := l.pos()
	from := l.off
	depth := 0
	for {
		if l.unterminated() {
			return Part{}, &Error{Pos: open, Msg: "unterminated interpolation"}
		}
		switch l.peek(0) {
		case '{':
			depth++
		case '}':
			if depth == 0 {
				text := string(l.src[from:l.off])
				l.advance()
				if strings.TrimSpace(text) == "" {
					return Part{}, &Error{Pos: open, Msg: "empty interpolation"}
				}
				return Part{Text: text, Expr: true, Pos: pos}, nil
			}
			depth--
		case '"', '\'':
			if err := l.skipQuoted(open); err != nil {
				return Part{}, err
			}
			continue
		}
		l.advance()
	}
}

func (l *Lexer) skipQuoted(open Pos) error {
	quote := l.advance()
	for {
		if l.unterminated() {
			return &Error{Pos: open, Msg: "unterminated interpolation"}
		}
		c := l.advance()
		if c == quote {
			return nil
		}
		if c == '\\' && !l.unterminated() {
			l.advance()
		}
	}
}

func (l *Lexer) char(start Pos) error {
	l.advance()
	var b strings.Builder
	switch {
	case l.unterminated() || l.peek(0) == '\'':
		return &Error{Pos: start, Msg: "empty character literal"}
	case l.peek(0) == '\\':
		if err := l.escape(&b); err != nil {
			return err
		}
	default:
		r, size := utf8.DecodeRune(l.src[l.off:])
		if r == utf8.RuneError {
			return &Error{Pos: start, Msg: "invalid UTF-8 in character literal"}
		}
		for range size {
			b.WriteByte(l.advance())
		}
	}
	if l.unterminated() || l.peek(0) != '\'' {
		return &Error{Pos: start, Msg: "character literal must hold one character"}
	}
	l.advance()
	r, _ := utf8.DecodeRuneInString(b.String())
	l.emit(Token{Kind: Char, Text: strconv.Itoa(int(r)), Pos: start})
	return nil
}

var operators = []struct {
	text string
	kind Kind
}{
	{"...", Ellipsis}, {"<<=", ShlAssign}, {">>=", ShrAssign},
	{"??", Coalesce}, {"<-", LArrow}, {"|>", PipeArrow}, {"->", Arrow}, {"=>", FatArrow}, {"%=", PercentAssign}, {"&=", AmpAssign}, {"|=", PipeAssign}, {"^=", CaretAssign},
	{"<<", Shl}, {">>", Shr}, {"+=", PlusAssign}, {"-=", MinusAssign}, {"*=", StarAssign}, {"/=", SlashAssign},
	{"&&", AndAnd}, {"||", OrOr}, {"==", Eq}, {"!=", NotEq}, {"<=", LtEq}, {">=", GtEq},
	{"..", DotDot}, {"(", LParen}, {")", RParen}, {"[", LBracket}, {"]", RBracket}, {"{", LBrace}, {"}", RBrace}, {",", Comma}, {":", Colon}, {".", Dot}, {"?", Question},
	{"=", Assign}, {"+", Plus}, {"-", Minus}, {"*", Star}, {"/", Slash}, {"%", Percent},
	{"!", Not}, {"<", Lt}, {">", Gt}, {"&", Amp}, {"|", Pipe}, {"^", Caret}, {"~", Tilde},
}

func (l *Lexer) operator(start Pos) error {
	rest := l.src[l.off:]
	for _, op := range operators {
		if !strings.HasPrefix(string(rest[:min(len(rest), len(op.text))]), op.text) {
			continue
		}
		for range op.text {
			l.advance()
		}
		l.emit(Token{Kind: op.kind, Text: op.text, Pos: start})
		return nil
	}
	return &Error{Pos: start, Msg: fmt.Sprintf("unexpected character %q", l.peek(0))}
}

func isLetter(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isHexDigit(c byte) bool {
	return isDigit(c) || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}
