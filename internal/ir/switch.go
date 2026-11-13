package ir

import (
	"slices"

	"lowlang/internal/ast"
	"lowlang/internal/lexer"
	"lowlang/internal/types"
)

const (
	linearSwitchMax  = 3
	minTableCases    = 12
	minTreeCases     = 48
	maxTableSpan     = 1 << 12
	maxTableSparsity = 3
)

type switchCase struct {
	key    int64
	target *Block
}

type switchPlan struct {
	typ   *types.Type
	cases []switchCase
}

func switchOf(arms []*ast.Arm, targets []*Block) (*switchPlan, bool) {
	keyed := arms[:len(arms)-1]
	if len(keyed) < minTableCases {
		return nil, false
	}
	plan := &switchPlan{cases: make([]switchCase, 0, len(keyed))}
	for i, arm := range keyed {
		key, typ, ok := armKey(arm.Pattern)
		if !ok || (i > 0 && typ != plan.typ) {
			return nil, false
		}
		plan.typ = typ
		plan.cases = append(plan.cases, switchCase{key: key, target: targets[i]})
	}
	plan.sortByKey()
	for i := 1; i < len(plan.cases); i++ {
		if plan.cases[i].key == plan.cases[i-1].key {
			return nil, false
		}
	}
	if !plan.dense() && len(plan.cases) < minTreeCases {
		return nil, false
	}
	return plan, true
}

func (p *switchPlan) sortByKey() {
	slices.SortFunc(p.cases, func(a, b switchCase) int {
		switch {
		case a.key < b.key:
			return -1
		case a.key > b.key:
			return 1
		}
		return 0
	})
}

func armKey(p *ast.Pattern) (int64, *types.Type, bool) {
	if p.Test == nil {
		if p.Variant == nil {
			return 0, nil, false
		}
		return int64(p.Variant.Tag), types.I64, true
	}
	test, ok := p.Test.(*ast.Binary)
	if !ok || test.Op != lexer.Eq {
		return 0, nil, false
	}
	if _, subject := test.X.(*ast.Subject); !subject {
		return 0, nil, false
	}
	lit, ok := ast.Literal(test.Y).(*ast.IntLit)
	typ := test.X.TypeOf()
	if !ok || typ == nil || typ.Kind != types.IntKind || !typ.Holds(lit.Value) {
		return 0, nil, false
	}
	return lit.Value, typ, true
}

func (p *switchPlan) span() uint64 {
	return uint64(p.cases[len(p.cases)-1].key) - uint64(p.cases[0].key) + 1
}

func (p *switchPlan) dense() bool {
	count := uint64(len(p.cases))
	span := p.span()
	return count >= minTableCases && span <= maxTableSpan && span <= maxTableSparsity*count
}

func (b *builder) lowerSwitch(plan *switchPlan, subject *Value, fallback *Block) {
	if plan.dense() {
		b.jumpTable(plan, subject, fallback)
		return
	}
	b.search(plan, subject, plan.cases, fallback)
}

func (b *builder) jumpTable(plan *switchPlan, subject *Value, fallback *Block) {
	low := plan.cases[0].key
	table := &JumpTable{Low: low, Cases: make([]int, plan.span())}
	b.cur.Kind = Switch
	b.cur.Control = subject
	b.cur.Table = table
	link(b.cur, fallback)
	for i, c := range plan.cases {
		table.Cases[uint64(c.key)-uint64(low)] = i + 1
		link(b.cur, c.target)
	}
}

func (b *builder) search(plan *switchPlan, subject *Value, cases []switchCase, fallback *Block) {
	if len(cases) <= linearSwitchMax {
		b.probe(plan, subject, cases, fallback)
		return
	}
	mid := len(cases) / 2
	below := b.newBlock()
	atLeast := b.newBlock()
	pivot := b.constant(cases[mid].key)
	b.branch(b.typed(OpLt, plan.typ, subject, pivot), below, atLeast)
	b.enter(below)
	b.search(plan, subject, cases[:mid], fallback)
	b.enter(atLeast)
	b.search(plan, subject, cases[mid:], fallback)
}

func (b *builder) probe(plan *switchPlan, subject *Value, cases []switchCase, fallback *Block) {
	for i, c := range cases {
		same := b.typed(OpEq, plan.typ, subject, b.constant(c.key))
		if i == len(cases)-1 {
			b.branch(same, c.target, fallback)
			return
		}
		next := b.newBlock()
		b.branch(same, c.target, next)
		b.enter(next)
	}
}
