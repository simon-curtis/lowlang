package lexer

import (
	"fmt"
	"strings"
)

type Error struct {
	Pos Pos
	Msg string
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Pos, e.Msg)
}

type Lexer struct {
	src    []byte
	off    int
	line   int
	col    int
	last   Kind
	tokens []Token
}

func Lex(src []byte) ([]Token, error) {
	l := &Lexer{src: src, line: 1, col: 1, last: Newline}
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
	return Pos{Line: l.line, Col: l.col}
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
	case Ident, Int, String, True, False, Break, Continue, Return, RParen, RBrace:
		l.emit(Token{Kind: Newline, Pos: p})
	}
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
		l.emitNewline(start)
		l.advance()
		return nil
	case isLetter(c):
		l.ident(start)
		return nil
	case isDigit(c):
		l.number(start)
		return nil
	case c == '"':
		return l.string(start)
	}
	return l.operator(start)
}

func (l *Lexer) ident(start Pos) {
	from := l.off
	for l.off < len(l.src) && (isLetter(l.peek(0)) || isDigit(l.peek(0))) {
		l.advance()
	}
	text := string(l.src[from:l.off])
	kind, ok := keywords[text]
	if !ok {
		kind = Ident
	}
	l.emit(Token{Kind: kind, Text: text, Pos: start})
}

func (l *Lexer) number(start Pos) {
	from := l.off
	for l.off < len(l.src) && (isDigit(l.peek(0)) || l.peek(0) == '_') {
		l.advance()
	}
	text := strings.ReplaceAll(string(l.src[from:l.off]), "_", "")
	l.emit(Token{Kind: Int, Text: text, Pos: start})
}

var escapes = map[byte]byte{'n': '\n', 't': '\t', 'r': '\r', '0': 0, '\\': '\\', '"': '"'}

func (l *Lexer) string(start Pos) error {
	l.advance()
	var b strings.Builder
	for {
		if l.off >= len(l.src) || l.peek(0) == '\n' {
			return &Error{Pos: start, Msg: "unterminated string"}
		}
		c := l.advance()
		if c == '"' {
			break
		}
		if c != '\\' {
			b.WriteByte(c)
			continue
		}
		escPos := l.pos()
		if l.off >= len(l.src) {
			return &Error{Pos: start, Msg: "unterminated string"}
		}
		e, ok := escapes[l.advance()]
		if !ok {
			return &Error{Pos: escPos, Msg: "unknown escape sequence"}
		}
		b.WriteByte(e)
	}
	l.emit(Token{Kind: String, Text: b.String(), Pos: start})
	return nil
}

var operators = []struct {
	text string
	kind Kind
}{
	{"->", Arrow}, {"+=", PlusAssign}, {"-=", MinusAssign}, {"*=", StarAssign}, {"/=", SlashAssign},
	{"&&", AndAnd}, {"||", OrOr}, {"==", Eq}, {"!=", NotEq}, {"<=", LtEq}, {">=", GtEq},
	{"(", LParen}, {")", RParen}, {"{", LBrace}, {"}", RBrace}, {",", Comma}, {":", Colon},
	{"=", Assign}, {"+", Plus}, {"-", Minus}, {"*", Star}, {"/", Slash}, {"%", Percent},
	{"!", Not}, {"<", Lt}, {">", Gt},
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

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}
