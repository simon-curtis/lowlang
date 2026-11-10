package regex

import (
	"fmt"
	"slices"
	"unicode/utf8"
)

const (
	maxRepeat = 10000
	maxRune   = 0x10FFFF
)

type Error struct {
	At      int
	Message string
}

func (e *Error) Error() string {
	return fmt.Sprintf("at byte %d: %s", e.At, e.Message)
}

func invalid(at int, format string, args ...any) *Error {
	return &Error{At: at, Message: fmt.Sprintf(format, args...)}
}

type Range struct {
	Lo, Hi uint32
}

type Look int

const (
	TextStart Look = iota
	TextEnd
	EndOrFinalNewline
	LineStart
	LineEnd
	WordBoundary
	NotWordBoundary
)

type node interface{ node() }

type empty struct{}
type literal struct{ c uint32 }
type set struct{ ranges []Range }
type assert struct{ look Look }
type capture struct {
	index int
	body  node
}
type concat struct{ items []node }
type alternate struct{ branches []node }
type repeat struct {
	body     node
	min, max int
	greedy   bool
}

func (empty) node()     {}
func (literal) node()   {}
func (set) node()       {}
func (assert) node()    {}
func (capture) node()   {}
func (concat) node()    {}
func (alternate) node() {}
func (repeat) node()    {}

type flags struct {
	ignoreCase, multiline, dotAll bool
}

type parser struct {
	pattern string
	pos     int
	groups  int
}

type bounds struct {
	min, max, end int
}

func parse(pattern string) (node, int, error) {
	p := &parser{pattern: pattern}
	scope := flags{}
	root, err := p.alternation(&scope)
	if err != nil {
		return nil, 0, err
	}
	if p.pos < len(pattern) {
		return nil, 0, invalid(p.pos, "unbalanced parenthesis")
	}
	return root, p.groups, nil
}

func (p *parser) atEnd() bool {
	return p.pos >= len(p.pattern)
}

func (p *parser) peek() byte {
	return p.pattern[p.pos]
}

func (p *parser) decode(at int) (uint32, int) {
	r, size := utf8.DecodeRuneInString(p.pattern[at:])
	return uint32(r), size
}

func (p *parser) alternation(f *flags) (node, error) {
	first, err := p.concat(f)
	if err != nil {
		return nil, err
	}
	branches := []node{first}
	for !p.atEnd() && p.peek() == '|' {
		p.pos++
		branch, err := p.concat(f)
		if err != nil {
			return nil, err
		}
		branches = append(branches, branch)
	}
	if len(branches) == 1 {
		return first, nil
	}
	return alternate{branches}, nil
}

func (p *parser) concat(f *flags) (node, error) {
	var items []node
	for !p.atEnd() && p.peek() != '|' && p.peek() != ')' {
		atom, err := p.atom(f)
		if err != nil {
			return nil, err
		}
		if atom == nil {
			continue
		}
		item, err := p.repeat(atom)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	switch len(items) {
	case 0:
		return empty{}, nil
	case 1:
		return items[0], nil
	}
	return concat{items}, nil
}

func (p *parser) atom(f *flags) (node, error) {
	start := p.pos
	switch p.peek() {
	case '(':
		return p.group(f)
	case '[':
		return p.class(*f)
	case '\\':
		escaped, err := p.escape(false)
		if err != nil {
			return nil, err
		}
		return foldLiteral(escaped, *f), nil
	case '.':
		p.pos++
		return set{dot(*f)}, nil
	case '^':
		p.pos++
		if f.multiline {
			return assert{LineStart}, nil
		}
		return assert{TextStart}, nil
	case '$':
		p.pos++
		if f.multiline {
			return assert{LineEnd}, nil
		}
		return assert{EndOrFinalNewline}, nil
	case '*', '+', '?':
		return nil, invalid(start, "nothing to repeat")
	case '{':
		if _, ok := scanBounds(p.pattern, start); ok {
			return nil, invalid(start, "nothing to repeat")
		}
	}
	c, size := p.decode(start)
	p.pos += size
	return foldLiteral(literal{c}, *f), nil
}

func dot(f flags) []Range {
	if f.dotAll {
		return []Range{{0, maxRune}}
	}
	return []Range{{0, 9}, {11, maxRune}}
}

func foldLiteral(n node, f flags) node {
	lit, ok := n.(literal)
	if !f.ignoreCase || !ok {
		return n
	}
	folded := fold([]Range{{lit.c, lit.c}})
	if len(folded) == 1 {
		return n
	}
	return set{folded}
}

func (p *parser) group(f *flags) (node, error) {
	start := p.pos
	p.pos++
	if !p.atEnd() && p.peek() == '?' {
		return p.extension(f, start)
	}
	p.groups++
	index := p.groups
	inner := *f
	body, err := p.alternation(&inner)
	if err != nil {
		return nil, err
	}
	if err := p.closeGroup(start); err != nil {
		return nil, err
	}
	return capture{index, body}, nil
}

func (p *parser) closeGroup(start int) error {
	if p.atEnd() {
		return invalid(start, "missing ), unterminated subpattern")
	}
	p.pos++
	return nil
}

func (p *parser) extension(f *flags, start int) (node, error) {
	p.pos++
	if p.atEnd() {
		return nil, invalid(p.pos, "unexpected end of pattern")
	}
	c := p.peek()
	switch {
	case c == '=' || c == '!':
		return nil, invalid(start, "lookaround is not supported")
	case c == '<':
		next := p.pos + 1
		if next < len(p.pattern) && (p.pattern[next] == '=' || p.pattern[next] == '!') {
			return nil, invalid(start, "lookaround is not supported")
		}
		return nil, invalid(start, "named groups are not supported")
	case c == 'P':
		return nil, invalid(start, "named groups are not supported")
	case c != 'i' && c != 'm' && c != 's' && c != '-' && c != ':':
		_, size := p.decode(p.pos)
		return nil, invalid(start+1, "unknown extension %s", p.pattern[start+1:p.pos+size])
	}
	return p.flags(f, start)
}

func (p *parser) flags(f *flags, start int) (node, error) {
	scoped := *f
	enable := true
	flagged := false
	for {
		if p.atEnd() {
			return nil, invalid(p.pos, "missing -, : or )")
		}
		c := p.peek()
		p.pos++
		switch c {
		case 'i':
			scoped.ignoreCase = enable
			flagged = true
			continue
		case 'm':
			scoped.multiline = enable
			flagged = true
			continue
		case 's':
			scoped.dotAll = enable
			flagged = true
			continue
		case '-', ':', ')':
		default:
			return nil, invalid(p.pos-1, "unknown flag")
		}
		if !enable && !flagged {
			return nil, invalid(p.pos-1, "missing flag")
		}
		switch c {
		case '-':
			enable = false
			flagged = false
		case ')':
			*f = scoped
			return nil, nil
		default:
			body, err := p.alternation(&scoped)
			if err != nil {
				return nil, err
			}
			if err := p.closeGroup(start); err != nil {
				return nil, err
			}
			return body, nil
		}
	}
}

func (p *parser) repeat(atom node) (node, error) {
	if p.atEnd() {
		return atom, nil
	}
	at := p.pos
	low, high := 0, -1
	switch p.peek() {
	case '*':
		p.pos++
	case '+':
		low = 1
		p.pos++
	case '?':
		high = 1
		p.pos++
	case '{':
		b, ok := scanBounds(p.pattern, at)
		if !ok {
			return atom, nil
		}
		if err := checkBounds(at, b); err != nil {
			return nil, err
		}
		low, high = b.min, b.max
		p.pos = b.end
	default:
		return atom, nil
	}
	if _, ok := atom.(assert); ok {
		return nil, invalid(at, "nothing to repeat")
	}
	greedy := true
	if !p.atEnd() && p.peek() == '?' {
		greedy = false
		p.pos++
	}
	if p.startsRepeat() {
		return nil, invalid(p.pos, "multiple repeat")
	}
	return repeat{atom, low, high, greedy}, nil
}

func (p *parser) startsRepeat() bool {
	if p.atEnd() {
		return false
	}
	switch p.peek() {
	case '*', '+', '?':
		return true
	case '{':
		_, ok := scanBounds(p.pattern, p.pos)
		return ok
	}
	return false
}

func scanNumber(s string, i *int) int {
	n := -1
	for *i < len(s) && s[*i] >= '0' && s[*i] <= '9' {
		digit := int(s[*i] - '0')
		if n < 0 {
			n = digit
		} else {
			n = n*10 + digit
		}
		n = min(n, maxRepeat+1)
		*i++
	}
	return n
}

func scanBounds(s string, at int) (bounds, bool) {
	i := at + 1
	low := scanNumber(s, &i)
	high := low
	if i < len(s) && s[i] == ',' {
		i++
		high = scanNumber(s, &i)
	} else if low < 0 {
		return bounds{}, false
	}
	if i >= len(s) || s[i] != '}' {
		return bounds{}, false
	}
	return bounds{min: max(low, 0), max: high, end: i + 1}, true
}

func checkBounds(at int, b bounds) error {
	if b.min > maxRepeat || b.max > maxRepeat {
		return invalid(at+1, "repeat count too large")
	}
	if b.max >= 0 && b.min > b.max {
		return invalid(at+1, "min repeat greater than max repeat")
	}
	return nil
}

func (p *parser) escape(inClass bool) (node, error) {
	start := p.pos
	p.pos++
	if p.atEnd() {
		return nil, invalid(start, "bad escape (end of pattern)")
	}
	c := p.peek()
	p.pos++
	switch c {
	case 'd':
		return set{digits()}, nil
	case 'D':
		return set{negate(digits())}, nil
	case 'w':
		return set{word()}, nil
	case 'W':
		return set{negate(word())}, nil
	case 's':
		return set{spaces()}, nil
	case 'S':
		return set{negate(spaces())}, nil
	case 'n':
		return literal{10}, nil
	case 't':
		return literal{9}, nil
	case 'r':
		return literal{13}, nil
	case 'f':
		return literal{12}, nil
	case 'v':
		return literal{11}, nil
	case 'a':
		return literal{7}, nil
	case 'x':
		return p.hex(start, 2)
	case 'u':
		return p.hex(start, 4)
	case 'U':
		return p.hex(start, 8)
	case 'b':
		if inClass {
			return literal{8}, nil
		}
		return assert{WordBoundary}, nil
	}
	return p.escapeAssertion(start, c, inClass)
}

func (p *parser) escapeAssertion(start int, c byte, inClass bool) (node, error) {
	var look Look
	switch c {
	case 'B':
		look = NotWordBoundary
	case 'A':
		look = TextStart
	case 'Z', 'z':
		look = TextEnd
	default:
		return p.escapeLiteral(start, c)
	}
	if inClass {
		return nil, invalid(start, "bad escape %s", p.pattern[start:p.pos])
	}
	return assert{look}, nil
}

func (p *parser) escapeLiteral(start int, c byte) (node, error) {
	if c >= '0' && c <= '9' {
		return nil, invalid(start, "backreferences and octal escapes are not supported")
	}
	if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' {
		return nil, invalid(start, "bad escape %s", p.pattern[start:p.pos])
	}
	r, size := p.decode(start + 1)
	p.pos = start + 1 + size
	return literal{r}, nil
}

func (p *parser) hex(start, count int) (node, error) {
	var value uint32
	for range count {
		d := -1
		if !p.atEnd() {
			d = hexDigit(p.peek())
		}
		if d < 0 {
			return nil, invalid(start, "incomplete escape %s", p.pattern[start:p.pos])
		}
		value = value*16 + uint32(d)
		p.pos++
	}
	if value > maxRune {
		return nil, invalid(start, "bad escape %s", p.pattern[start:p.pos])
	}
	return literal{value}, nil
}

func hexDigit(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	}
	return -1
}

func (p *parser) class(f flags) (node, error) {
	start := p.pos
	p.pos++
	negated := false
	if !p.atEnd() && p.peek() == '^' {
		negated = true
		p.pos++
	}
	var ranges []Range
	first := true
	for {
		if p.atEnd() {
			return nil, invalid(start, "unterminated character set")
		}
		if p.peek() == ']' && !first {
			p.pos++
			break
		}
		first = false
		var err error
		if ranges, err = p.classItem(ranges); err != nil {
			return nil, err
		}
	}
	if f.ignoreCase {
		ranges = fold(ranges)
	}
	ranges = normalize(ranges)
	if negated {
		ranges = negate(ranges)
	}
	return set{ranges}, nil
}

func (p *parser) classItem(ranges []Range) ([]Range, error) {
	start := p.pos
	low, err := p.classAtom()
	if err != nil {
		return nil, err
	}
	dash := p.pos+1 < len(p.pattern) && p.peek() == '-' && p.pattern[p.pos+1] != ']'
	if !dash {
		return addAtom(ranges, low), nil
	}
	p.pos++
	high, err := p.classAtom()
	if err != nil {
		return nil, err
	}
	lo, loOK := low.(literal)
	hi, hiOK := high.(literal)
	if !loOK || !hiOK || hi.c < lo.c {
		return nil, invalid(start, "bad character range %s", p.pattern[start:p.pos])
	}
	return append(ranges, Range{lo.c, hi.c}), nil
}

func (p *parser) classAtom() (node, error) {
	if p.peek() == '\\' {
		return p.escape(true)
	}
	c, size := p.decode(p.pos)
	p.pos += size
	return literal{c}, nil
}

func addAtom(ranges []Range, atom node) []Range {
	switch a := atom.(type) {
	case literal:
		return append(ranges, Range{a.c, a.c})
	case set:
		return append(ranges, a.ranges...)
	}
	return ranges
}

func digits() []Range {
	return []Range{{'0', '9'}}
}

func word() []Range {
	return []Range{{'0', '9'}, {'A', 'Z'}, {'_', '_'}, {'a', 'z'}}
}

func spaces() []Range {
	return []Range{{9, 13}, {' ', ' '}}
}

func fold(ranges []Range) []Range {
	var out []Range
	for _, r := range ranges {
		out = append(out, r)
		out = addShifted(out, r, 'A', 'Z', 'a')
		out = addShifted(out, r, 'a', 'z', 'A')
	}
	return normalize(out)
}

func addShifted(out []Range, r Range, lo, hi, to uint32) []Range {
	from := max(r.Lo, lo)
	until := min(r.Hi, hi)
	if from > until {
		return out
	}
	return append(out, Range{from - lo + to, until - lo + to})
}

func normalize(ranges []Range) []Range {
	sorted := slices.Clone(ranges)
	slices.SortStableFunc(sorted, func(a, b Range) int { return int(a.Lo) - int(b.Lo) })
	var out []Range
	for _, r := range sorted {
		if n := len(out); n > 0 && r.Lo <= out[n-1].Hi+1 {
			out[n-1].Hi = max(out[n-1].Hi, r.Hi)
			continue
		}
		out = append(out, r)
	}
	return out
}

func negate(ranges []Range) []Range {
	var out []Range
	var next uint32
	for _, r := range ranges {
		if r.Lo > next {
			out = append(out, Range{next, r.Lo - 1})
		}
		next = r.Hi + 1
	}
	if next <= maxRune {
		out = append(out, Range{next, maxRune})
	}
	return out
}
