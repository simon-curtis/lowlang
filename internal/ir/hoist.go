package ir

import (
	"lowlang/internal/ast"
	"lowlang/internal/lexer"
	"lowlang/internal/prelude"
)

const (
	sliceCopyBytes = 1024
	mapCopyEntries = 4
)

func constant(e ast.Expr) bool {
	switch e := e.(type) {
	case *ast.IntLit, *ast.FloatLit, *ast.BoolLit, *ast.StrLit:
		return true
	case *ast.Unary:
		return e.Op == lexer.Minus && constant(e.X)
	case *ast.Ident:
		return e.Null || e.Variant != nil
	case *ast.StructLit:
		if e.Type.CStruct() {
			return false
		}
		for _, f := range e.Fields {
			if !constant(f.Value) {
				return false
			}
		}
		return true
	}
	return false
}

func allConstant(list []ast.Expr) bool {
	for _, e := range list {
		if !constant(e) {
			return false
		}
	}
	return true
}

func (b *builder) cell() *Value {
	v := b.emit(OpCell)
	v.Aux = int64(*b.cells)
	v.Scalar = true
	*b.cells++
	return v
}

func (b *builder) template(build func() *Value) *Value {
	cell := b.cell()
	cached := b.emit(OpLoad, cell)
	temp := b.newTemp()
	b.writeTemp(temp, []*Value{cached})
	miss := b.newBlock()
	done := b.newBlock()
	b.branch(b.emit(OpEq, cached, b.constant(0)), miss, done)
	b.enter(miss)
	fresh := build()
	b.store(place{addr: cell}, []*Value{fresh})
	b.writeTemp(temp, []*Value{fresh})
	b.jump(done)
	b.enter(done)
	return b.readTemp(temp, 1)[0]
}

func (b *builder) hoistedSlice(e *ast.SliceLit) ([]*Value, bool) {
	elem := e.Type.Elem
	count := len(e.Elems)
	bytes := int64(count * elem.Stride())
	if !b.hoist || count == 0 || elem.CStruct() || isVector(elem) || !allConstant(e.Elems) {
		return nil, false
	}
	if !e.ReadOnly && bytes < sliceCopyBytes {
		return nil, false
	}
	template := b.template(func() *Value { return b.filledSlice(e)[0] })
	n := b.constant(int64(count))
	if e.ReadOnly {
		return sliceOf(template, n), true
	}
	size := b.constant(bytes)
	data := b.runtime(rtGrow, template, size, size, b.scan(elem))
	return sliceOf(data, n), true
}

func (b *builder) hoistedMap(e *ast.MapLit) ([]*Value, bool) {
	count := len(e.Keys)
	if !b.hoist || count == 0 || !allConstant(e.Keys) || !allConstant(e.Values) {
		return nil, false
	}
	if !e.ReadOnly && count < mapCopyEntries {
		return nil, false
	}
	template := b.template(func() *Value { return b.filledMap(e)[0] })
	if e.ReadOnly {
		return []*Value{template}, true
	}
	return b.mapCall(prelude.MapClone, e.Type, template), true
}
