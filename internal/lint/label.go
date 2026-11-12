package lint

import (
	"lowlang/internal/ast"
	"lowlang/internal/lexer"
)

const preferLoopLabel = "prefer-loop-label"

type relay struct {
	flag    *ast.Local
	value   bool
	keyword string
	label   string
}

func (l *linter) relays(b *ast.Block) {
	steps := steps(b)
	for i := 1; i < len(steps); i++ {
		l.relayAfter(steps[i-1], steps[i])
	}
}

func (l *linter) relayAfter(prev, next ast.Node) {
	label, body, ok := loopOf(prev)
	if !ok {
		return
	}
	r, ok := relayOf(next)
	if !ok {
		return
	}
	for _, pos := range r.exits(label, body) {
		l.suggestLabel(pos, r)
	}
}

func (l *linter) suggestLabel(pos lexer.Pos, r relay) {
	if r.label == "" {
		l.report(pos, preferLoopLabel,
			"%s relays this break to the enclosing loop; use a loop label: name: for … { … %s name }",
			r.flag.Name, r.keyword)
		return
	}
	l.report(pos, preferLoopLabel, "%s relays this break to loop %s; use %s %s",
		r.flag.Name, r.label, r.keyword, r.label)
}

func steps(b *ast.Block) []ast.Node {
	out := make([]ast.Node, 0, len(b.Stmts)+1)
	for _, s := range b.Stmts {
		out = append(out, unwrap(s))
	}
	if b.Tail != nil {
		out = append(out, unwrap(b.Tail))
	}
	return out
}

func unwrap(n ast.Node) ast.Node {
	if s, ok := n.(*ast.ExprStmt); ok {
		return s.X
	}
	return n
}

func loopOf(n ast.Node) (string, *ast.Block, bool) {
	switch n := n.(type) {
	case *ast.For:
		return n.Label, n.Body, true
	case *ast.ForIn:
		return n.Label, n.Body, true
	}
	return "", nil, false
}

func relayOf(n ast.Node) (relay, bool) {
	cond, ok := n.(*ast.If)
	if !ok || cond.Else != nil {
		return relay{}, false
	}
	flag, value, ok := flagTest(cond.Cond)
	if !ok {
		return relay{}, false
	}
	switch jump := jumpOf(cond.Then).(type) {
	case *ast.Break:
		return relay{flag: flag, value: value, keyword: "break", label: jump.Label}, true
	case *ast.Continue:
		return relay{flag: flag, value: value, keyword: "continue", label: jump.Label}, true
	}
	return relay{}, false
}

func flagTest(cond ast.Expr) (*ast.Local, bool, bool) {
	switch c := cond.(type) {
	case *ast.Ident:
		return c.Local, true, c.Local != nil
	case *ast.Unary:
		x, ok := c.X.(*ast.Ident)
		if !ok || c.Op != lexer.Not || x.Local == nil {
			return nil, false, false
		}
		return x.Local, false, true
	}
	return nil, false, false
}

func jumpOf(n ast.Node) ast.Node {
	block, ok := n.(*ast.Block)
	if !ok {
		return n
	}
	inner := steps(block)
	if len(inner) != 1 {
		return nil
	}
	return jumpOf(inner[0])
}

func (r relay) exits(label string, body *ast.Block) []lexer.Pos {
	var found []lexer.Pos
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.For, *ast.ForIn, *ast.Closure, *ast.Spawn, *ast.Defer:
			return false
		case *ast.Block:
			found = append(found, r.blockExits(label, n)...)
		}
		return true
	})
	return found
}

func (r relay) blockExits(label string, b *ast.Block) []lexer.Pos {
	var found []lexer.Pos
	set := false
	for _, step := range steps(b) {
		if sets, touches := r.assigns(step); touches {
			set = sets
			continue
		}
		if pos, ok := exit(step, label); ok && set {
			found = append(found, pos)
		}
	}
	return found
}

func (r relay) assigns(step ast.Node) (bool, bool) {
	a, ok := step.(*ast.Assign)
	if !ok {
		return false, false
	}
	target, ok := a.Target.(*ast.Ident)
	if !ok || target.Local != r.flag {
		return false, false
	}
	lit, ok := a.Value.(*ast.BoolLit)
	return a.Op == lexer.Assign && ok && lit.Value == r.value, true
}

func exit(step ast.Node, label string) (lexer.Pos, bool) {
	b, ok := jumpOf(step).(*ast.Break)
	if !ok || (b.Label != "" && b.Label != label) {
		return lexer.Pos{}, false
	}
	return b.Pos, true
}
