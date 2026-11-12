package ir

import (
	"math"
	"math/bits"
	"slices"

	"lowlang/internal/types"
)

type operand struct {
	id    int
	konst bool
	n     int64
	bits  int
}

type exprKey struct {
	op     Op
	aux    int64
	str    string
	target *Func
	ty     *types.Type
	from   *types.Type
	a      operand
	b      operand
}

type fact struct {
	smaller  operand
	larger   operand
	unsigned bool
	orEqual  bool
}

type optimizer struct {
	fn    *Func
	ws    *workspace
	doms  *Dominators[*Block]
	reps  map[exprKey]*Value
	added []exprKey
	facts []fact
	out   []*Value
}

func blockSuccs(b *Block) []*Block {
	return b.Succs
}

func blockPreds(b *Block) []*Block {
	return b.Preds
}

func newOptimizer(fn *Func, ws *workspace, doms *Dominators[*Block]) *optimizer {
	size := fn.values + 1
	ws.nonneg = reset(ws.nonneg, size)
	ws.defs = reset(ws.defs, size)
	ws.visiting = reset(ws.visiting, size)
	return &optimizer{
		fn:   fn,
		ws:   ws,
		doms: doms,
		reps: map[exprKey]*Value{},
	}
}

func (s *ssa) optimize() {
	o := newOptimizer(s.fn, s.ws, newDominators(s.fn))
	o.visit(0)
	for _, b := range s.fn.Blocks {
		s.resolveBlock(b)
	}
}

func (o *optimizer) visit(i int) {
	b := o.doms.Nodes[i]
	mark := len(o.added)
	edge, hasEdge := o.edgeFact(b)
	if hasEdge {
		o.facts = append(o.facts, edge)
	}
	o.out = nil
	o.block(b)
	b.Values = o.out
	for _, c := range o.doms.Children[i] {
		o.visit(c)
	}
	for _, k := range o.added[mark:] {
		delete(o.reps, k)
	}
	o.added = o.added[:mark]
	if hasEdge {
		o.facts = o.facts[:len(o.facts)-1]
	}
}

func (o *optimizer) block(b *Block) {
	for _, v := range b.Values {
		o.define(v)
		for i, a := range v.Args {
			v.Args[i] = resolve(a)
		}
		if v.Op == OpCheck {
			o.check(v)
			continue
		}
		o.fold(v)
		if v.forward != nil {
			continue
		}
		if cseOp(v.Op) && o.merge(v) {
			continue
		}
		o.out = append(o.out, v)
	}
}

func (o *optimizer) merge(v *Value) bool {
	key := keyOf(v)
	if rep, ok := o.reps[key]; ok {
		rep.Scalar = rep.Scalar && v.Scalar
		v.forward = rep
		return true
	}
	o.reps[key] = v
	o.added = append(o.added, key)
	return false
}

func (o *optimizer) check(v *Value) {
	if o.checked(v) || o.inRange(v.Args[0], v.Args[1], v.Block) {
		return
	}
	if o.tighten(v) && o.checked(v) {
		return
	}
	key := keyOf(v)
	o.reps[key] = v
	o.added = append(o.added, key)
	o.out = append(o.out, v)
}

func (o *optimizer) checked(v *Value) bool {
	_, ok := o.reps[keyOf(v)]
	return ok
}

func (o *optimizer) tighten(check *Value) bool {
	bound, found := o.boundBelow(ref(check.Args[0]), check.Args[1])
	if !found {
		return false
	}
	limit, defined := o.definition(bound.id)
	if bound.konst || !defined || limit == check.Args[1] {
		return false
	}
	check.Args[1] = limit
	return true
}

func cseOp(op Op) bool {
	switch op {
	case OpAdd, OpSub, OpMul, OpAnd, OpOr, OpXor, OpShl, OpShr, OpConvert, OpFuncAddr, OpHashSeed:
		return true
	}
	return false
}

func ref(v *Value) operand {
	if v.Op == OpConst {
		return operand{konst: true, n: v.Aux, bits: v.FloatBits}
	}
	return operand{id: v.ID}
}

func keyOf(v *Value) exprKey {
	k := exprKey{op: v.Op, aux: v.Aux, str: v.Str, target: v.Target, ty: v.Type, from: v.From}
	if len(v.Args) > 0 {
		k.a = ref(v.Args[0])
	}
	if len(v.Args) > 1 {
		k.b = ref(v.Args[1])
	}
	return k
}

func (o *optimizer) edgeFact(b *Block) (fact, bool) {
	if len(b.Preds) != 1 {
		return fact{}, false
	}
	p := b.Preds[0]
	if p.Kind != If || p.Control == nil || p.Succs[0] == p.Succs[1] {
		return fact{}, false
	}
	c := resolve(p.Control)
	if !c.Op.Compare() || c.Type == nil || !c.Type.Integer() {
		return fact{}, false
	}
	taken := p.Succs[0] == b
	x, y := ref(resolve(c.Args[0])), ref(resolve(c.Args[1]))
	unsigned := !c.Type.Signed
	switch {
	case taken && c.Op == OpLt, !taken && c.Op == OpGe:
		return fact{smaller: x, larger: y, unsigned: unsigned}, true
	case taken && c.Op == OpGt, !taken && c.Op == OpLe:
		return fact{smaller: y, larger: x, unsigned: unsigned}, true
	case !taken && c.Op == OpLt, taken && c.Op == OpGe:
		return fact{smaller: y, larger: x, unsigned: unsigned, orEqual: true}, true
	case !taken && c.Op == OpGt, taken && c.Op == OpLe:
		return fact{smaller: x, larger: y, unsigned: unsigned, orEqual: true}, true
	}
	return fact{}, false
}

func (o *optimizer) inRange(i, n *Value, at *Block) bool {
	if i.Op == OpConst && n.Op == OpConst {
		return uint64(i.Aux) < uint64(n.Aux)
	}
	if n.Op == OpConst && o.belowConstant(i, n.Aux, at) {
		return true
	}
	ri, rn := ref(i), ref(n)
	if slices.Contains(o.facts, fact{smaller: ri, larger: rn, unsigned: true}) {
		return true
	}
	if !o.nonNegative(i) {
		return false
	}
	if slices.Contains(o.facts, fact{smaller: ri, larger: rn}) {
		return true
	}
	_, found := o.boundBelow(ri, n)
	return found
}

func (o *optimizer) belowConstant(i *Value, n int64, at *Block) bool {
	if !o.nonNegative(i) {
		return false
	}
	bound, known := o.upper(i, at, rangeDepth)
	return known && bound < n
}

func (o *optimizer) boundBelow(i operand, n *Value) (operand, bool) {
	for _, f := range slices.Backward(o.facts) {
		if f.smaller == i && !f.unsigned && !f.orEqual && o.atLeast(n, f.larger, phiDepth) {
			return f.larger, true
		}
	}
	return operand{}, false
}

const phiDepth = 3

func (o *optimizer) atLeast(v *Value, bound operand, depth int) bool {
	v = resolve(v)
	switch {
	case ref(v) == bound:
		return true
	case v.Op == OpConst && bound.konst:
		return v.Aux >= bound.n
	case v.Op != OpPhi || depth == 0:
		return false
	}
	for k, arg := range v.Args {
		arg = resolve(arg)
		if arg != v && !o.atLeastFrom(arg, bound, v.Block.Preds[k], depth-1) {
			return false
		}
	}
	return true
}

func (o *optimizer) atLeastFrom(v *Value, bound operand, pred *Block, depth int) bool {
	if o.atLeast(v, bound, depth) {
		return true
	}
	i, reachable := o.doms.Index[pred]
	if !reachable {
		return false
	}
	want := ref(v)
	for {
		f, ok := o.edgeFact(o.doms.Nodes[i])
		if ok && !f.unsigned && f.smaller == bound && f.larger == want {
			return true
		}
		if i == 0 {
			return false
		}
		i = o.doms.Idom[i]
	}
}

func (o *optimizer) nonNegative(v *Value) bool {
	if known, ok := o.memoisedNonNegative(v); ok {
		return known
	}
	result := o.provesNonNegative(v)
	o.ws.nonneg = fit(o.ws.nonneg, v.ID+1)
	o.ws.nonneg[v.ID] = nonNegativeVerdict(result)
	return result
}

func nonNegativeVerdict(known bool) int8 {
	if known {
		return 1
	}
	return -1
}

func (o *optimizer) memoisedNonNegative(v *Value) (bool, bool) {
	if v.ID >= len(o.ws.nonneg) || o.ws.nonneg[v.ID] == 0 {
		return false, false
	}
	return o.ws.nonneg[v.ID] > 0, true
}

func (o *optimizer) define(v *Value) {
	o.ws.defs = fit(o.ws.defs, v.ID+1)
	o.ws.defs[v.ID] = v
}

func (o *optimizer) definition(id int) (*Value, bool) {
	if id >= len(o.ws.defs) || o.ws.defs[id] == nil {
		return nil, false
	}
	return o.ws.defs[id], true
}

func (o *optimizer) provesNonNegative(v *Value) bool {
	switch {
	case v.Op == OpConst:
		return v.Aux >= 0
	case unsignedNarrow(v.Type):
		return true
	case v.Op == OpPhi:
		return o.inductionNonNegative(v) || o.boundedBelowByFact(v)
	case v.Op == OpAdd:
		return o.increasesNonNegative(v) || o.boundedBelowByFact(v)
	case v.Op == OpMul:
		return o.productNonNegative(v) || o.boundedBelowByFact(v)
	}
	return o.boundedBelowByFact(v)
}

func unsignedNarrow(t *types.Type) bool {
	return t != nil && t.Integer() && !t.Signed && t.Bits > 0 && t.Bits < 64
}

func (o *optimizer) inductionNonNegative(phi *Value) bool {
	o.ws.visiting = fit(o.ws.visiting, phi.ID+1)
	if o.ws.visiting[phi.ID] {
		return true
	}
	o.ws.visiting[phi.ID] = true
	defer func() { o.ws.visiting[phi.ID] = false }()
	for _, a := range phi.Args {
		a = resolve(a)
		if a == phi || o.nonNegativeUnmemoised(a) {
			continue
		}
		return false
	}
	return true
}

func (o *optimizer) nonNegativeUnmemoised(v *Value) bool {
	if known, ok := o.memoisedNonNegative(v); ok {
		return known
	}
	return o.provesNonNegative(v)
}

func (o *optimizer) increasesNonNegative(add *Value) bool {
	base, step := resolve(add.Args[0]), resolve(add.Args[1])
	if base.Op == OpConst {
		base, step = step, base
	}
	if !signedOrUntyped(add.Type) || !o.nonNegativeUnmemoised(step) {
		return false
	}
	return o.nonNegativeUnmemoised(base) && o.cannotOverflow(add, base, step)
}

func signedOrUntyped(t *types.Type) bool {
	return t == nil || (t.Integer() && t.Signed && t.Bits > 0)
}

func maxOf(t *types.Type) int64 {
	if t == nil || t.Bits >= 64 {
		return math.MaxInt64
	}
	return 1<<(t.Bits-1) - 1
}

func (o *optimizer) cannotOverflow(add, base, step *Value) bool {
	if step.Op == OpConst && o.stepBelowBound(add, base, step.Aux) {
		return true
	}
	_, fits := o.sumUpper(add, rangeDepth)
	return fits
}

func (o *optimizer) stepBelowBound(add, base *Value, step int64) bool {
	if step == 0 {
		return true
	}
	limit := maxOf(add.Type)
	want := ref(base)
	for _, f := range o.factsAt(add.Block) {
		if !f.unsigned && !f.orEqual && f.smaller == want && bounded(f.larger, step, limit) {
			return true
		}
	}
	return false
}

func bounded(larger operand, step, limit int64) bool {
	if step == 1 {
		return true
	}
	return larger.konst && larger.n >= 0 && larger.n <= limit && step-1 <= limit-larger.n
}

func (o *optimizer) fold(v *Value) {
	switch v.Op {
	case OpAdd, OpSub, OpMul, OpAnd, OpOr, OpXor, OpShl, OpShr, OpDiv, OpRem:
		o.foldArithmetic(v)
	case OpNeg, OpCompl:
		if floatValue(v) {
			o.foldFloat(v)
		} else if integerOp(v) && v.Args[0].Op == OpConst {
			x := v.Args[0].Aux
			if v.Op == OpNeg {
				x = -x
			} else {
				x = ^x
			}
			becomeConstant(v, wrap(x, v.Type))
		}
	case OpNot:
		if v.Args[0].Op == OpConst {
			becomeConstant(v, v.Args[0].Aux^1)
		}
	case OpEq, OpNe, OpLt, OpLe, OpGt, OpGe:
		foldComparison(v)
	case OpConvert:
		foldConversion(v)
	case OpSqrt, OpFloor:
		o.foldFloat(v)
	case OpCall:
		callDirectly(v)
	}
}

func callDirectly(call *Value) {
	if !call.Indirect() {
		return
	}
	code := call.Args[0]
	if code.Op != OpFuncAddr || code.Aux != CodeAddr || code.Target == nil {
		return
	}
	call.Signature = nil
	call.Target = code.Target
	call.Args = call.Args[1:]
}

func (o *optimizer) foldArithmetic(v *Value) {
	switch {
	case floatValue(v):
		o.foldFloat(v)
	case integerOp(v):
		o.foldBinary(v)
	}
}

func foldComparison(v *Value) {
	x, y := v.Args[0], v.Args[1]
	if x.Op != OpConst || y.Op != OpConst || v.Type == nil || !v.Type.Integer() {
		return
	}
	less, equal := x.Aux < y.Aux, x.Aux == y.Aux
	if !v.Type.Signed {
		less = uint64(x.Aux) < uint64(y.Aux)
	}
	v.Type = nil
	if comparisonHolds(v.Op, less, equal) {
		becomeConstant(v, 1)
		return
	}
	becomeConstant(v, 0)
}

func comparisonHolds(op Op, less, equal bool) bool {
	switch op {
	case OpEq:
		return equal
	case OpNe:
		return !equal
	case OpLt:
		return less
	case OpLe:
		return less || equal
	case OpGt:
		return !less && !equal
	}
	return !less
}

func integerOp(v *Value) bool {
	t := v.Type
	if t == nil {
		return true
	}
	return t.Integer() && (t.Bits == 8 || t.Bits == 16 || t.Bits == 32 || t.Bits == 64)
}

func becomeConstant(v *Value, n int64) {
	v.Op = OpConst
	v.Args = nil
	v.Aux = n
}

func wrap(n int64, t *types.Type) int64 {
	if t == nil || t.Bits == 0 || t.Bits >= 64 {
		return n
	}
	shift := 64 - t.Bits
	if t.Signed {
		return n << shift >> shift
	}
	return int64(uint64(n) << shift >> shift)
}

func commutative(op Op) bool {
	return op == OpAdd || op == OpMul || op == OpAnd || op == OpOr || op == OpXor
}

func (o *optimizer) foldBinary(v *Value) {
	if commutative(v.Op) && v.Args[0].Op == OpConst && v.Args[1].Op != OpConst {
		v.Args[0], v.Args[1] = v.Args[1], v.Args[0]
	}
	x, y := v.Args[0], v.Args[1]
	if y.Op != OpConst {
		return
	}
	if x.Op == OpConst {
		if n, ok := evaluate(v.Op, v.Type, x.Aux, y.Aux); ok {
			becomeConstant(v, n)
		}
		return
	}
	o.simplify(v, x, y.Aux)
}

func (o *optimizer) simplify(v, x *Value, n int64) {
	switch {
	case n == 0 && (v.Op == OpAdd || v.Op == OpSub || v.Op == OpOr || v.Op == OpXor || v.Op == OpShl || v.Op == OpShr):
		forwardTo(v, x)
	case n == 1 && v.Op == OpMul:
		forwardTo(v, x)
	case n == 0 && (v.Op == OpMul || v.Op == OpAnd):
		becomeConstant(v, 0)
	case v.Op == OpMul && n > 1 && n&(n-1) == 0:
		o.mulToShift(v, x, n)
	case (v.Op == OpDiv || v.Op == OpRem) && dividesByConstant(v, n):
		o.divideByConstant(v, x, n)
	}
}

func forwardTo(v, to *Value) {
	to.Scalar = to.Scalar && v.Scalar
	v.forward = to
}

func (o *optimizer) mulToShift(v, x *Value, n int64) {
	count := o.constantFor(v, int64(bits.TrailingZeros64(uint64(n))))
	o.become(v, OpShl, x, count)
}

func evaluate(op Op, t *types.Type, x, y int64) (int64, bool) {
	var r int64
	switch op {
	case OpAdd:
		r = x + y
	case OpSub:
		r = x - y
	case OpMul:
		r = x * y
	case OpAnd:
		r = x & y
	case OpOr:
		r = x | y
	case OpXor:
		r = x ^ y
	case OpShl, OpShr:
		return shiftConstant(op, t, x, y)
	case OpDiv, OpRem:
		return divideConstant(op, t, x, y)
	}
	return wrap(r, t), true
}

func shiftConstant(op Op, t *types.Type, x, count int64) (int64, bool) {
	if t == nil {
		return 0, false
	}
	var r int64
	switch {
	case op == OpShl && uint64(count) >= 64:
		r = 0
	case op == OpShl:
		r = x << uint64(count)
	case uint64(count) >= 64 && t.Signed:
		r = x >> 63
	case uint64(count) >= 64:
		r = 0
	case t.Signed:
		r = x >> uint64(count)
	default:
		r = int64(uint64(x) >> uint64(count))
	}
	return wrap(r, t), true
}

func divideConstant(op Op, t *types.Type, x, y int64) (int64, bool) {
	if y == 0 || y == -1 {
		return 0, false
	}
	var r int64
	switch {
	case t != nil && !t.Signed && op == OpDiv:
		r = int64(uint64(x) / uint64(y))
	case t != nil && !t.Signed:
		r = int64(uint64(x) % uint64(y))
	case op == OpDiv:
		r = x / y
	default:
		r = x % y
	}
	return wrap(r, t), true
}

func (s *ssa) removeDeadValues() {
	live := make([]bool, s.fn.values+1)
	var work []*Value
	mark := func(v *Value) {
		if !live[v.ID] {
			live[v.ID] = true
			work = append(work, v)
		}
	}
	for _, b := range s.fn.Blocks {
		for _, v := range b.Values {
			if !discardable(v) {
				mark(v)
			}
		}
		if b.Control != nil {
			mark(b.Control)
		}
		for _, r := range b.Results {
			mark(r)
		}
	}
	for len(work) > 0 {
		v := work[len(work)-1]
		work = work[:len(work)-1]
		for _, a := range v.Args {
			mark(a)
		}
	}
	for _, b := range s.fn.Blocks {
		b.Values = slices.DeleteFunc(b.Values, func(v *Value) bool { return !live[v.ID] })
	}
}

func discardable(v *Value) bool {
	switch v.Op {
	case OpConst, OpString, OpEmbed, OpUndef, OpPhi, OpAdd, OpSub, OpMul, OpAnd, OpOr, OpXor, OpShl, OpShr,
		OpNeg, OpNot, OpCompl, OpConvert, OpSqrt, OpFloor, OpEq, OpNe, OpLt, OpLe, OpGt, OpGe, OpFuncAddr,
		OpMulHigh, OpTrailingZeros, OpHashSeed, OpSlot:
		return true
	case OpLoad:
		return v.Mem&MemTrap == 0
	case OpDiv, OpRem:
		return floatValue(v) || divisorCannotFault(v)
	}
	return false
}

func floatValue(v *Value) bool {
	return v.Type != nil && v.Type.Float()
}

func divisorCannotFault(v *Value) bool {
	d := v.Args[1]
	return d.Op == OpConst && d.Aux != 0 && d.Aux != -1
}

func (s *ssa) foldConstantBranches() bool {
	folded := false
	for _, b := range s.fn.Blocks {
		if b.Kind == If && b.Control != nil && b.Control.Op == OpConst {
			takeCase(b, branchIndex(b.Control.Aux != 0))
			folded = true
		}
		if b.Kind == Switch && b.Control.Op == OpConst {
			takeCase(b, b.Table.Index(b.Control.Aux))
			folded = true
		}
	}
	return folded
}

func takeCase(b *Block, taken int) {
	for i := range b.Succs {
		if i != taken {
			removeEdge(b, b.Succs[i], b.occurrence(i))
		}
	}
	b.Succs = []*Block{b.Succs[taken]}
	b.Kind = Plain
	b.Control = nil
	b.Table = nil
}

func branchIndex(condition bool) int {
	if condition {
		return 0
	}
	return 1
}

func (b *Block) occurrence(succ int) int {
	n := 0
	for _, s := range b.Succs[:succ] {
		if s == b.Succs[succ] {
			n++
		}
	}
	return n
}

func removeEdge(from, to *Block, occurrence int) {
	i := -1
	for ; occurrence >= 0; occurrence-- {
		next := slices.Index(to.Preds[i+1:], from)
		i += next + 1
	}
	to.removePred(i)
}

func (b *Block) removePred(i int) {
	b.Preds = slices.Delete(b.Preds, i, i+1)
	for _, v := range b.Values {
		if v.Op == OpPhi {
			v.Args = slices.Delete(v.Args, i, i+1)
		}
	}
}
