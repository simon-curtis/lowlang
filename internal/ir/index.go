package ir

import (
	"lowlang/internal/ast"
	"lowlang/internal/types"
)

type indexGroup struct {
	index int64
	tags  []int
}

func (b *builder) variantIndex(e *ast.Selector) []*Value {
	t := e.X.TypeOf()
	tag := b.value(e.X, t)[0]
	if offset, ok := tagOffset(t); ok {
		return []*Value{b.offsetTag(tag, offset)}
	}
	return []*Value{b.indexOfTag(t, tag)}
}

func tagOffset(t *types.Type) (int64, bool) {
	offset := t.Variants[0].Index - int64(t.Variants[0].Tag)
	for _, v := range t.Variants[1:] {
		if v.Index-int64(v.Tag) != offset {
			return 0, false
		}
	}
	return offset, true
}

func (b *builder) offsetTag(tag *Value, offset int64) *Value {
	if offset == 0 {
		return tag
	}
	offsetValue := b.constant(offset)
	return b.typed(OpAdd, types.I64, tag, offsetValue)
}

func (b *builder) indexOfTag(t *types.Type, tag *Value) *Value {
	groups := indexGroups(t)
	fallback := widestGroup(groups)
	temp := b.newTemp()
	done := b.newBlock()
	targets := make([]*Block, len(groups))
	plan := &switchPlan{typ: types.I64}
	for i, g := range groups {
		targets[i] = b.newBlock()
		if i == fallback {
			continue
		}
		for _, tag := range g.tags {
			plan.cases = append(plan.cases, switchCase{key: int64(tag), target: targets[i]})
		}
	}
	plan.sortByKey()
	b.lowerSwitch(plan, tag, targets[fallback])
	for i, g := range groups {
		b.enter(targets[i])
		index := b.constant(g.index)
		b.writeTemp(temp, []*Value{index})
		b.jump(done)
	}
	b.enter(done)
	return b.readTemp(temp, 1)[0]
}

func indexGroups(t *types.Type) []indexGroup {
	var groups []indexGroup
	for _, v := range t.Variants {
		at := groupOf(groups, v.Index)
		if at < 0 {
			groups = append(groups, indexGroup{index: v.Index})
			at = len(groups) - 1
		}
		groups[at].tags = append(groups[at].tags, v.Tag)
	}
	return groups
}

func groupOf(groups []indexGroup, index int64) int {
	for i, g := range groups {
		if g.index == index {
			return i
		}
	}
	return -1
}

func widestGroup(groups []indexGroup) int {
	widest := 0
	for i, g := range groups {
		if len(g.tags) > len(groups[widest].tags) {
			widest = i
		}
	}
	return widest
}
