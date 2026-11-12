package ir

import (
	"lowlang/internal/ast"
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
	key := append([]*Value{table}, b.value(entry.Index, m.Key)...)
	if op, ok := arith[s.Op.Operator()]; ok {
		current := b.mapCall(prelude.MapGet, m, key...)
		old := b.present(current)
		value = []*Value{b.typed(op, m.Elem, old, value[0])}
	}
	args := append(key, value...)
	b.mapCall(prelude.MapSet, m, args...)
}

func (b *builder) present(optional []*Value) *Value {
	temp := b.newTemp()
	found := b.newBlock()
	done := b.newBlock()
	b.writeTemp(temp, []*Value{b.constant(0)})
	b.branch(optional[0], found, done)
	b.enter(found)
	b.writeTemp(temp, optional[1:2])
	b.jump(done)
	b.enter(done)
	return b.readTemp(temp, 1)[0]
}

func (b *builder) deleteEntry(e *ast.Call) {
	ref := e.Args[0].(*ast.Ref)
	m := ref.Type
	table := b.load(b.placeOf(ref.X), m)[0]
	args := b.withKey(table, e.Args[1], m)
	b.mapCall(prelude.MapDelete, m, args...)
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
		b.bind(s.ValueLocal, key)
	} else {
		b.bind(s.KeyLocal, key)
		b.bind(s.ValueLocal, b.mapCall(prelude.MapValue, m, table, at))
	}
	b.loopBody(loop, s.Body)
	from := b.emit(OpAdd, at, b.constant(1))
	b.writeTemp(cursor, b.mapCall(prelude.MapNext, m, table, from))
	b.closeLoop(loop)
}
