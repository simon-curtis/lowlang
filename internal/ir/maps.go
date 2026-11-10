package ir

import (
	"lowlang/internal/ast"
	"lowlang/internal/types"
)

func entryWidth(m *types.Type) int {
	return 1 + m.Key.Size() + m.Elem.Size()
}

func (b *builder) key(e ast.Expr, m *types.Type) []*Value {
	words := b.value(e, m.Key)
	if len(words) == 1 {
		words = append(words, b.constant(0))
	}
	return words
}

func (b *builder) entryField(table, base *Value, offset int) place {
	entries := b.load(place{addr: table}, 1)[0]
	return place{addr: b.scaled(entries, base, 8), offset: 8 * offset}
}

func (b *builder) entryValue(m *types.Type, table, base *Value) place {
	return b.entryField(table, base, 1+m.Key.Size())
}

func (b *builder) mapLit(e *ast.MapLit) []*Value {
	m := e.Type
	text := int64(0)
	if m.Key == types.Str {
		text = 1
	}
	table := b.callNamed("__map_new", b.constant(int64(entryWidth(m))), b.constant(text))[0]
	for i, k := range e.Keys {
		key := b.key(k, m)
		value := b.value(e.Values[i], m.Elem)
		base := b.callNamed("__map_slot", table, key[0], key[1])[0]
		b.store(b.entryValue(m, table, base), value)
	}
	return []*Value{table}
}

func (b *builder) lookup(e *ast.Index) []*Value {
	m := e.X.TypeOf()
	table := b.word(e.X)
	key := b.key(e.Index, m)
	base := b.callNamed("__map_find", table, key[0], key[1])[0]
	temp := b.newTemp()
	found := b.newBlock()
	missing := b.newBlock()
	done := b.newBlock()
	b.branch(b.emit(OpLt, base, b.constant(0)), missing, found)
	b.enter(found)
	value := b.load(b.entryValue(m, table, base), m.Elem.Size())
	b.writeTemp(temp, append([]*Value{b.constant(1)}, value...))
	b.jump(done)
	b.enter(missing)
	b.writeTemp(temp, b.pad([]*Value{b.constant(0)}, e.Type.Size()))
	b.jump(done)
	b.enter(done)
	return b.readTemp(temp, e.Type.Size())
}

func (b *builder) assignEntry(s *ast.Assign) {
	entry := s.Entry
	m := entry.X.TypeOf()
	value := b.value(s.Value, m.Elem)
	table := b.word(entry.X)
	key := b.key(entry.Index, m)
	base := b.callNamed("__map_slot", table, key[0], key[1])[0]
	slot := b.entryValue(m, table, base)
	if op, ok := arith[s.Op]; ok {
		old := b.load(slot, 1)[0]
		value = []*Value{b.typed(op, m.Elem, old, value[0])}
	}
	b.store(slot, value)
}

func (b *builder) deleteEntry(e *ast.Call) {
	ref := e.Args[0].(*ast.Ref)
	table := b.load(b.place(ref.Place), 1)[0]
	key := b.key(e.Args[1], ref.Type)
	b.callNamed("__map_delete", table, key[0], key[1])
}

func (b *builder) mapLoop(s *ast.ForIn) {
	m := s.Iter.TypeOf()
	table := b.word(s.Iter)
	cursor := b.newTemp()
	b.writeTemp(cursor, b.callNamed("__map_next", table, b.constant(0)))
	header := b.newBlock()
	body := b.newBlock()
	next := b.newBlock()
	exit := b.newBlock()
	b.jump(header)
	b.cur = header
	base := b.readTemp(cursor, 1)[0]
	b.branch(b.emit(OpLt, base, b.constant(0)), exit, body)
	b.enter(body)
	key := b.load(b.entryField(table, base, 1), m.Key.Size())
	if s.KeyLocal == nil {
		b.bind(s.ValueLocal, key)
	} else {
		b.bind(s.KeyLocal, key)
		b.bind(s.ValueLocal, b.load(b.entryValue(m, table, base), m.Elem.Size()))
	}
	b.loopBody(s.Body, next, exit)
	b.enter(next)
	from := b.emit(OpAdd, base, b.constant(int64(entryWidth(m))))
	b.writeTemp(cursor, b.callNamed("__map_next", table, from))
	b.jump(header)
	b.seal(header)
	b.enter(exit)
}
