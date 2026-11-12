package ir

import (
	"lowlang/internal/ast"
	"lowlang/internal/types"
)

func (b *builder) pop(e *ast.Call) []*Value {
	ref := e.Args[0].(*ast.Ref)
	elem := ref.Type.Elem
	header := b.placeOf(ref.X)
	slice := b.load(header, types.Str)
	temp := b.newTemp()
	some := b.newBlock()
	empty := b.newBlock()
	done := b.newBlock()
	b.branch(b.emit(OpEq, slice[1], b.constant(0)), empty, some)
	b.enter(some)
	last := b.count(OpSub, slice[1], b.constant(1))
	addr := b.scaled(slice[0], last, elem.Stride())
	value := b.load(place{addr: addr, scalar: scalarOf(elem), tail: elem.Tail()}, elem)
	b.store(header.at(1), []*Value{last})
	b.writeTemp(temp, append([]*Value{b.constant(1)}, value...))
	b.jump(done)
	b.enter(empty)
	b.writeTemp(temp, b.pad([]*Value{b.constant(0)}, 1+elem.Size()))
	b.jump(done)
	b.enter(done)
	return b.readTemp(temp, 1+elem.Size())
}

func (b *builder) truncate(e *ast.Call) {
	ref := e.Args[0].(*ast.Ref)
	header := b.placeOf(ref.X)
	length := b.word(e.Args[1])
	slice := b.load(header, types.Str)
	b.check(length, b.count(OpAdd, slice[1], b.constant(1)))
	b.store(header.at(1), []*Value{length})
}
