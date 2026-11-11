package check

import "lowlang/internal/ast"

type readScan struct {
	skip   map[ast.Expr]bool
	reads  map[*ast.Ident]bool
	idents []*ast.Ident
	lets   []*ast.Let
}

func (c *Checker) markReadOnly(fn *ast.Func) {
	scan := &readScan{skip: map[ast.Expr]bool{}, reads: map[*ast.Ident]bool{}}
	ast.Inspect(fn.Body, scan.visit)
	uses := map[*ast.Local]int{}
	reads := map[*ast.Local]int{}
	for _, id := range scan.idents {
		if id.Local == nil {
			continue
		}
		uses[id.Local]++
		if scan.reads[id] {
			reads[id.Local]++
		}
	}
	for _, let := range scan.lets {
		if uses[let.Local] == reads[let.Local] {
			markLiteral(let.Value)
		}
	}
}

func (s *readScan) visit(n ast.Node) bool {
	switch n := n.(type) {
	case *ast.Ident:
		s.idents = append(s.idents, n)
	case *ast.Let:
		if n.Local != nil && isLiteral(n.Value) {
			s.lets = append(s.lets, n)
		}
	case *ast.Assign:
		s.write(n.Target)
	case *ast.Ref:
		s.write(n.X)
	case *ast.Index:
		if !s.skip[n] {
			s.read(n.X)
		}
	case *ast.Selector:
		if n.Length && !s.skip[n] {
			s.read(n.X)
		}
	case *ast.ForIn:
		s.read(n.Iter)
	}
	return true
}

func (s *readScan) write(e ast.Expr) {
	for e != nil {
		s.skip[e] = true
		switch x := e.(type) {
		case *ast.Selector:
			e = x.X
		case *ast.Index:
			e = x.X
		case *ast.Receiver:
			e = x.X
		default:
			return
		}
	}
}

func (s *readScan) read(x ast.Expr) {
	switch x := x.(type) {
	case *ast.Ident:
		s.reads[x] = true
	case *ast.SliceLit, *ast.MapLit:
		markLiteral(x)
	}
}

func isLiteral(e ast.Expr) bool {
	switch e.(type) {
	case *ast.SliceLit, *ast.MapLit:
		return true
	}
	return false
}

func markLiteral(e ast.Expr) {
	switch e := e.(type) {
	case *ast.SliceLit:
		e.ReadOnly = true
	case *ast.MapLit:
		e.ReadOnly = true
	}
}
