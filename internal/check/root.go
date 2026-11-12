package check

import (
	"lowlang/internal/ast"
	"lowlang/internal/lexer"
	"lowlang/internal/types"
)

type root struct {
	local  *ast.Local
	offset int
}

type borrowed struct {
	root
	bytes int
}

func (a borrowed) overlaps(b borrowed) bool {
	return a.local != nil && a.local == b.local &&
		a.offset < b.offset+b.bytes &&
		b.offset < a.offset+a.bytes
}

func rootOf(e ast.Expr) (root, bool) {
	switch e := e.(type) {
	case *ast.Receiver:
		return rootOf(e.X)
	case *ast.Ident:
		if e.Local != nil {
			return root{local: e.Local}, true
		}
	case *ast.Selector:
		if e.Field == nil {
			return root{}, false
		}
		if e.X.TypeOf().Kind == types.PointerKind {
			return root{}, true
		}
		base, ok := rootOf(e.X)
		if !ok {
			return root{}, false
		}
		return root{local: base.local, offset: base.offset + e.Field.Bytes}, true
	case *ast.Index:
		if e.X.TypeOf().Kind == types.SliceKind {
			return root{}, true
		}
	case *ast.Unary:
		if e.Op == lexer.Star {
			return root{}, true
		}
	}
	return root{}, false
}
