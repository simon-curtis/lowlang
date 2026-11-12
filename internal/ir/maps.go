package ir

import (
	"lowlang/internal/ast"
	"lowlang/internal/lexer"
	"lowlang/internal/prelude"
	"lowlang/internal/types"
)

func (b *builder) mapCall(name string, m *types.Type, args ...*Value) []*Value {
	instance := types.InstanceName(name, []*types.Type{m.Key, m.Elem})
	return b.callNamed(instance, args...)
}

func (b *builder) withKey(table *Value, key ast.Expr, m *types.Type, rest ...*Value) []*Value {
	args := []*Value{table}
	args = append(args, b.value(key, m.Key)...)
	return append(args, rest...)
}

func (b *builder) mapLit(e *ast.MapLit) []*Value {
	if words, ok := b.hoistedMap(e); ok {
		return words
	}
	return b.filledMap(e)
}

func (b *builder) filledMap(e *ast.MapLit) []*Value {
	m := e.Type
	table := b.mapCall(prelude.MapNew, m)[0]
	for i, k := range e.Keys {
		value := b.value(e.Values[i], m.Elem)
		args := b.withKey(table, k, m, value...)
		b.mapCall(prelude.MapSet, m, args...)
	}
	return []*Value{table}
}

func (b *builder) lookup(e *ast.Index) []*Value {
	m := e.X.TypeOf()
	args := b.withKey(b.word(e.X), e.Index, m)
	return b.mapCall(prelude.MapGet, m, args...)
}

func (b *builder) assignEntry(s *ast.Assign) {
	entry := s.Entry
	m := entry.X.TypeOf()
	value := b.value(s.Value, m.Elem)
	table := b.word(entry.X)
	key := b.withKey(table, entry.Index, m)
	if op := s.Op.Operator(); op != lexer.Assign {
		current := b.mapCall(prelude.MapGet, m, key...)
		old := b.present(current, m.Elem.Size())
		value = b.compound(op, m.Elem, old, value)
	}
	args := append(key, value...)
	b.mapCall(prelude.MapSet, m, args...)
}

func (b *builder) present(optional []*Value, words int) []*Value {
	temp := b.newTemp()
	found := b.newBlock()
	done := b.newBlock()
	zero := make([]*Value, words)
	for i := range zero {
		zero[i] = b.constant(0)
	}
	b.writeTemp(temp, zero)
	b.branch(optional[0], found, done)
	b.enter(found)
	b.writeTemp(temp, optional[1:1+words])
	b.jump(done)
	b.enter(done)
	return b.readTemp(temp, words)
}

func (b *builder) deleteEntry(e *ast.Call) []*Value {
	ref := e.Args[0].(*ast.Ref)
	m := ref.Type
	table := b.load(b.placeOf(ref.X), m)[0]
	args := b.withKey(table, e.Args[1], m)
	b.mapCall(prelude.MapDelete, m, args...)
	return nil
}

func (b *builder) mapLoop(s *ast.ForIn) {
	m := s.Iter.TypeOf()
	table := b.word(s.Iter)
	cursor := b.newTemp()
	b.writeTemp(cursor, b.mapCall(prelude.MapNext, m, table, b.constant(0)))
	loop := b.openLoop()
	at := b.readTemp(cursor, 1)[0]
	b.branch(b.emit(OpLt, at, b.constant(0)), loop.exit, loop.body)
	b.enter(loop.body)
	key := b.mapCall(prelude.MapKey, m, table, at)
	if s.KeyLocal == nil {
		b.bindLoopValue(s, key)
	} else {
		b.bind(s.KeyLocal, key)
		b.bindLoopValue(s, b.mapCall(prelude.MapValue, m, table, at))
	}
	b.loopBody(loop, s.Body)
	from := b.emit(OpAdd, at, b.constant(1))
	b.writeTemp(cursor, b.mapCall(prelude.MapNext, m, table, from))
	b.closeLoop(loop)
}
