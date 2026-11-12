package ir

import (
	"fmt"

	"lowlang/internal/ast"
	"lowlang/internal/lexer"
	"lowlang/internal/types"
)

type location struct {
	local  *ast.Local
	ptr    ast.Expr
	index  *ast.Index
	offset int
	width  *types.Type
	tail   int
}

func fieldWidth(e *ast.Selector) *types.Type {
	if types.StructOf(e.X.TypeOf()).C {
		return e.Field.Type.Width()
	}
	return nil
}

func locate(e ast.Expr) (*location, bool) {
	switch e := e.(type) {
	case *ast.Receiver:
		return locate(e.X)
	case *ast.Ident:
		if e.Local != nil {
			return &location{local: e.Local}, true
		}
	case *ast.Selector:
		if e.Field == nil {
			return nil, false
		}
		width := fieldWidth(e)
		if e.X.TypeOf().Kind == types.PointerKind {
			return &location{ptr: e.X, offset: e.Field.Bytes, width: width, tail: e.Field.Type.Tail()}, true
		}
		base, ok := locate(e.X)
		if !ok {
			return nil, false
		}
		return &location{local: base.local, ptr: base.ptr, index: base.index, offset: base.offset + e.Field.Bytes, width: width, tail: e.Field.Type.Tail()}, true
	case *ast.Index:
		if e.X.TypeOf().Kind == types.SliceKind {
			return &location{index: e, tail: e.Type.Tail()}, true
		}
	case *ast.Unary:
		if e.Op == lexer.Star {
			return &location{ptr: e.X, tail: e.Type.Tail()}, true
		}
	}
	return nil, false
}

func mustLocate(e ast.Expr) *location {
	loc, ok := locate(e)
	if !ok {
		panic(fmt.Sprintf("ir: %T is not a place", e))
	}
	return loc
}

func (b *builder) placeOf(e ast.Expr) place {
	return b.place(mustLocate(e))
}

func paramSize(f *ast.Func, i int) int {
	if f.Params[i].Mut {
		return 1
	}
	return f.ParamTypes[i].Size()
}

func Boxed(l *ast.Local) bool {
	if !l.Captured {
		return false
	}
	return l.Assigned || l.Addressed || l.Type.Kind == types.VectorKind || l.Type.CStruct()
}
