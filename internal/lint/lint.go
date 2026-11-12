package lint

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"lowlang/internal/ast"
	"lowlang/internal/diag"
)

type Warning struct {
	Pos  diag.Pos
	Rule string
	Msg  string
}

func (w Warning) String() string {
	return fmt.Sprintf("%s: warning: %s [%s]", w.Pos, w.Msg, w.Rule)
}

func Run(files []*ast.File) []Warning {
	l := &linter{reported: map[diag.Pos]bool{}, chained: map[*ast.If]bool{}}
	for _, file := range files {
		l.file(file)
	}
	slices.SortStableFunc(l.warnings, compare)
	return l.warnings
}

func compare(a, b Warning) int {
	if c := cmp.Compare(a.Pos.File, b.Pos.File); c != 0 {
		return c
	}
	if c := cmp.Compare(a.Pos.Line, b.Pos.Line); c != 0 {
		return c
	}
	return cmp.Compare(a.Pos.Col, b.Pos.Col)
}

type linter struct {
	warnings []Warning
	reported map[diag.Pos]bool
	chained  map[*ast.If]bool
}

func (l *linter) file(file *ast.File) {
	for _, fn := range file.Funcs {
		if fn.Body != nil && !generated(fn) {
			ast.Inspect(fn.Body, l.visit)
		}
	}
	for _, decl := range file.Structs {
		if decl.Drop != nil {
			ast.Inspect(decl.Drop, l.visit)
		}
	}
}

func generated(fn *ast.Func) bool {
	return strings.HasPrefix(fn.Pos.File, "<")
}

func (l *linter) visit(n ast.Node) bool {
	switch n := n.(type) {
	case *ast.If:
		l.ifChain(n)
	case *ast.Block:
		l.relays(n)
	}
	return true
}

func (l *linter) report(pos diag.Pos, rule, format string, args ...any) {
	if l.reported[pos] {
		return
	}
	l.reported[pos] = true
	l.warnings = append(l.warnings, Warning{Pos: pos, Rule: rule, Msg: fmt.Sprintf(format, args...)})
}
