package ir

import "math"

const rangeDepth = 4

func (o *optimizer) factsAt(at *Block) []fact {
	i, reachable := o.doms.Index[at]
	if !reachable {
		return nil
	}
	var facts []fact
	for {
		if f, ok := o.edgeFact(o.doms.Nodes[i]); ok {
			facts = append(facts, f)
		}
		if i == 0 {
			return facts
		}
		i = o.doms.Idom[i]
	}
}

func (o *optimizer) upper(v *Value, at *Block, depth int) (int64, bool) {
	v = resolve(v)
	bound, known := o.operationUpper(v, depth)
	if v.Op == OpConst {
		return bound, known
	}
	for _, f := range o.factsAt(at) {
		if f.unsigned || f.smaller != ref(v) {
			continue
		}
		limit, ok := o.operandUpper(f.larger, at, depth)
		if !ok || (!f.orEqual && limit == math.MinInt64) {
			continue
		}
		if !f.orEqual {
			limit--
		}
		if !known || limit < bound {
			bound, known = limit, true
		}
	}
	return bound, known
}

func (o *optimizer) operandUpper(larger operand, at *Block, depth int) (int64, bool) {
	if larger.konst {
		return larger.n, true
	}
	def, defined := o.defs[larger.id]
	if !defined || depth == 0 {
		return 0, false
	}
	return o.upper(def, at, depth-1)
}

func (o *optimizer) operationUpper(v *Value, depth int) (int64, bool) {
	switch {
	case v.Op == OpConst:
		return v.Aux, true
	case unsignedNarrow(v.Type):
		return 1<<v.Type.Bits - 1, true
	case depth == 0 || !signedOrUntyped(v.Type):
		return 0, false
	case v.Op == OpAdd:
		return o.sumUpper(v, depth)
	case v.Op == OpMul:
		return o.productUpper(v, depth)
	}
	return 0, false
}

func (o *optimizer) operandUppers(v *Value, depth int) (x, y int64, ok bool) {
	a, b := resolve(v.Args[0]), resolve(v.Args[1])
	if !o.nonNegativeUnmemoised(a) || !o.nonNegativeUnmemoised(b) {
		return 0, 0, false
	}
	x, knownX := o.upper(a, v.Block, depth-1)
	y, knownY := o.upper(b, v.Block, depth-1)
	return x, y, knownX && knownY && x >= 0 && y >= 0
}

func (o *optimizer) sumUpper(v *Value, depth int) (int64, bool) {
	x, y, ok := o.operandUppers(v, depth)
	if !ok || x > maxOf(v.Type)-y {
		return 0, false
	}
	return x + y, true
}

func (o *optimizer) productUpper(v *Value, depth int) (int64, bool) {
	x, y, ok := o.operandUppers(v, depth)
	if !ok || (y != 0 && x > maxOf(v.Type)/y) {
		return 0, false
	}
	return x * y, true
}

func (o *optimizer) boundedBelowByFact(v *Value) bool {
	want := ref(v)
	for _, f := range o.factsAt(v.Block) {
		if f.unsigned || f.larger != want || !f.smaller.konst {
			continue
		}
		if f.smaller.n >= 0 || (!f.orEqual && f.smaller.n == -1) {
			return true
		}
	}
	return false
}

func (o *optimizer) productNonNegative(v *Value) bool {
	_, ok := o.operationUpper(v, rangeDepth)
	return ok
}
