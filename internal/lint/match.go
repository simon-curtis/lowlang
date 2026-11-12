package lint

import (
	"fmt"
	"strconv"
	"strings"

	"lowlang/internal/ast"
	"lowlang/internal/diag"
	"lowlang/internal/lexer"
)

const preferMatch = "prefer-match"

const minConstants = 2

func (l *linter) ifChain(head *ast.If) {
	if l.chained[head] {
		return
	}
	links, tail := l.chain(head)
	if tail == nil || len(links) < minConstants {
		return
	}
	var subject string
	arms := make([]string, 0, len(links))
	for i, link := range links {
		test, ok := compared(link.Cond)
		if !ok || (i > 0 && test.subject != subject) {
			return
		}
		subject = test.subject
		arms = append(arms, test.constant)
	}
	l.suggestMatch(head.Pos, subject, arms)
}

func (l *linter) chain(head *ast.If) ([]*ast.If, ast.Expr) {
	links := []*ast.If{head}
	for cur := head; ; {
		next, ok := cur.Else.(*ast.If)
		if !ok {
			return links, cur.Else
		}
		l.chained[next] = true
		links = append(links, next)
		cur = next
	}
}

func (l *linter) suggestMatch(pos diag.Pos, subject string, constants []string) {
	cases := make([]string, len(constants))
	for i, c := range constants {
		cases[i] = fmt.Sprintf("%s => …", c)
	}
	l.report(pos, preferMatch,
		"this if chain tests %s against %s; use match %s { %s, _ => … }",
		subject, strings.Join(constants, ", "), subject, strings.Join(cases, ", "))
}

type comparison struct {
	subject  string
	constant string
}

func compared(cond ast.Expr) (comparison, bool) {
	b, ok := cond.(*ast.Binary)
	if !ok || b.Op != lexer.Eq {
		return comparison{}, false
	}
	value, other := b.Y, b.X
	constant, isConstant := constantOf(value)
	if !isConstant {
		value, other = b.X, b.Y
		if constant, isConstant = constantOf(value); !isConstant {
			return comparison{}, false
		}
	}
	if _, both := constantOf(other); both {
		return comparison{}, false
	}
	subject, ok := subjectOf(other)
	if !ok {
		return comparison{}, false
	}
	return comparison{subject: subject, constant: constant}, true
}

func constantOf(e ast.Expr) (string, bool) {
	switch e := e.(type) {
	case *ast.IntLit:
		if e.Char {
			return strconv.QuoteRune(rune(e.Value)), true
		}
		return strconv.FormatInt(e.Value, 10), true
	case *ast.StrLit:
		return strconv.Quote(e.Value), true
	case *ast.Ident:
		return e.Name, e.Variant != nil
	case *ast.Selector:
		return e.Name, e.Variant != nil
	}
	return "", false
}

func subjectOf(e ast.Expr) (string, bool) {
	switch e := e.(type) {
	case *ast.Ident:
		return e.Name, e.Variant == nil && !e.Null
	case *ast.IntLit:
		return strconv.FormatInt(e.Value, 10), true
	case *ast.Selector:
		x, ok := subjectOf(e.X)
		return fmt.Sprintf("%s.%s", x, e.Name), ok && e.Variant == nil
	case *ast.Index:
		x, ok := subjectOf(e.X)
		if !ok {
			return "", false
		}
		i, ok := subjectOf(e.Index)
		return fmt.Sprintf("%s[%s]", x, i), ok
	case *ast.Unary:
		if e.Op != lexer.Star {
			return "", false
		}
		x, ok := subjectOf(e.X)
		return fmt.Sprintf("*%s", x), ok
	}
	return "", false
}
