package amd64

import (
	"fmt"
	"slices"

	"lowlang/internal/ir"
)

type vreg struct {
	konst             *int64
	pool              *int64
	mem               *address
	frame             *ir.Value
	pinned            string
	float             bool
	light             bool
	ranges            []segment
	crossesCall       bool
	crossesPreserving bool
	scalar            bool
	hint              string
	sources           []*vreg
	sinks             []*vreg
	copyPeers         []*vreg
	avoid             *vreg
	reg               string
	spill             int
	weight            int
	length            int
}

type instr struct {
	op      ir.Op
	dst     *vreg
	args    []*vreg
	value   *ir.Value
	call    *ir.Value
	pos     int
	negated bool
	rotate  int64
	literal *string
}

type phiCopy struct {
	dst *vreg
	src *vreg
}

type mblock struct {
	label   string
	kind    ir.Kind
	poll    bool
	instrs  []*instr
	copies  []*phiCopy
	heads   []*phiCopy
	light   bool
	succs   []*mblock
	control *vreg
	table   *ir.JumpTable
	fused   *ir.Value
	compare []*vreg
	results []*vreg
	steps   []step
	liveIn  map[*vreg]bool
	liveOut map[*vreg]bool
	start   int
	copyPos int
	end     int
}

type mfunc struct {
	blocks []*mblock
	vregs  []*vreg
	guard  *mblock
}

type lowering struct {
	fn     *mfunc
	vregs  map[*ir.Value]*vreg
	blocks map[*ir.Block]*mblock
	scalar bool
}

func lower(f *ir.Func, prefix string, scalar, wrap bool) *mfunc {
	l := &lowering{
		fn:     &mfunc{},
		vregs:  map[*ir.Value]*vreg{},
		blocks: map[*ir.Block]*mblock{},
		scalar: scalar,
	}
	for _, b := range f.Blocks {
		mb := &mblock{label: fmt.Sprintf(".L%s_%d", prefix, b.ID), kind: b.Kind, poll: b.Poll}
		l.blocks[b] = mb
		l.fn.blocks = append(l.fn.blocks, mb)
	}
	for _, b := range f.Blocks {
		l.block(b)
	}
	for _, b := range f.Blocks {
		l.phiCopies(b)
	}
	if scalar {
		l.fn.simplifyBranches()
		l.fn.selectInstructions()
	}
	l.fn.duplicateReturns()
	if scalar {
		l.fn.rematerializeFrameAddresses()
	}
	l.fn.rotateLoops()
	l.fn.layout()
	if wrap {
		l.fn.shrinkWrap()
	}
	l.fn.hint()
	l.fn.settleFloats()
	l.fn.pinResults()
	return l.fn
}

func (f *mfunc) hint() {
	for _, mb := range f.blocks {
		for _, in := range mb.instrs {
			in.hint()
		}
		for _, c := range slices.Concat(mb.heads, mb.copies) {
			c.hint()
		}
	}
}

func (c *phiCopy) hint() {
	if c.src.fixed() {
		return
	}
	c.dst.sources = append(c.dst.sources, c.src)
	c.src.sinks = append(c.src.sinks, c.dst)
	c.dst.copyPeers = append(c.dst.copyPeers, c.src)
	c.src.copyPeers = append(c.src.copyPeers, c.dst)
}

func (in *instr) hint() {
	switch {
	case in.isParam():
		if k := int(in.value.Aux); k < len(abi.args) {
			in.dst.hint = abi.args[k]
		}
	case in.rotate != 0:
		in.dst.sources = []*vreg{in.args[0]}
	case in.twoOperand():
		in.hintOperands()
	case in.preservesRegisters(), in.literal != nil:
	case in.op == ir.OpCall || in.op == ir.OpRuntime || in.op == ir.OpSyscall:
		args, regs := in.args, abi.args
		if in.value.Indirect() {
			args = args[1:]
		}
		if in.op == ir.OpSyscall {
			regs = abi.syscall
		}
		for i, a := range args[:min(len(args), len(regs))] {
			a.hint = regs[i]
		}
	}
}

func (in *instr) twoOperand() bool {
	switch in.op {
	case ir.OpAdd, ir.OpSub, ir.OpMul, ir.OpAnd, ir.OpOr, ir.OpXor, ir.OpShl, ir.OpShr:
		return in.rotate == 0
	case ir.OpDiv:
		return isFloat(in.value.Type)
	}
	return false
}

func (in *instr) hintOperands() {
	first, second := in.args[0], in.args[1]
	in.dst.sources = []*vreg{first}
	switch in.op {
	case ir.OpAdd, ir.OpMul, ir.OpAnd, ir.OpOr, ir.OpXor:
		in.dst.sources = append(in.dst.sources, second)
	default:
		in.dst.avoid = second
	}
}

func (l *lowering) vreg(v *ir.Value) *vreg {
	r, ok := l.vregs[v]
	if !ok {
		r = l.newVreg()
		r.scalar = v.Scalar
		switch {
		case l.scalar && v.Op == ir.OpSlot:
			r.frame = v
		case fitsImmediate(v):
			r.konst = &v.Aux
		case l.scalar && v.Op == ir.OpConst && v.FloatBits > 0:
			r.pool = &v.Aux
			r.float = true
		default:
			r.float = l.scalar && floatable(v)
		}
		l.vregs[v] = r
	}
	return r
}

func floatable(v *ir.Value) bool {
	if v.FloatBits == 0 {
		return false
	}
	switch v.Op {
	case ir.OpAdd, ir.OpSub, ir.OpMul, ir.OpDiv, ir.OpNeg, ir.OpSqrt, ir.OpFloor, ir.OpLoad, ir.OpParam, ir.OpResult, ir.OpPhi:
		return true
	case ir.OpConvert:
		return v.Type.Float()
	}
	return false
}

func (in *instr) takesFloat(k int) bool {
	switch in.op {
	case ir.OpAdd, ir.OpSub, ir.OpMul, ir.OpDiv, ir.OpNeg, ir.OpEq, ir.OpNe, ir.OpLt, ir.OpLe, ir.OpGt, ir.OpGe:
		return isFloat(in.value.Type)
	case ir.OpConvert:
		return in.value.From.Float()
	case ir.OpSqrt, ir.OpFloor, ir.OpCall, ir.OpCCall, ir.OpRuntime, ir.OpSyscall:
		return true
	case ir.OpStore:
		return k == 1
	}
	return false
}

func (f *mfunc) settleFloats() {
	for _, mb := range f.blocks {
		for _, in := range mb.instrs {
			for k, a := range in.args {
				if !in.takesFloat(k) {
					a.float = false
				}
			}
		}
		mb.restrictFloats()
	}
	f.dropIdleFloats()
	for _, r := range f.vregs {
		if r.pool != nil && !r.float {
			r.pool = nil
		}
		if r.float {
			r.scalar = true
		}
	}
	for _, mb := range f.blocks {
		mb.instrs = slices.DeleteFunc(mb.instrs, func(in *instr) bool {
			return in.op == ir.OpConst && in.dst.pool != nil
		})
	}
}

func (in *instr) computesFloat() bool {
	switch in.op {
	case ir.OpAdd, ir.OpSub, ir.OpMul, ir.OpDiv, ir.OpNeg, ir.OpEq, ir.OpNe, ir.OpLt, ir.OpLe, ir.OpGt, ir.OpGe:
		return isFloat(in.value.Type)
	case ir.OpConvert:
		return in.value.From.Float() || in.value.Type.Float()
	}
	return in.op == ir.OpSqrt || in.op == ir.OpFloor
}

func (f *mfunc) dropIdleFloats() {
	busy := map[*vreg]bool{}
	for _, mb := range f.blocks {
		for _, in := range mb.instrs {
			if in.computesFloat() {
				busy[in.dst] = true
				for _, a := range in.args {
					busy[a] = true
				}
			}
		}
		if mb.fused != nil && isFloat(mb.fused.Type) {
			for _, r := range mb.compare {
				busy[r] = true
			}
		}
	}
	for changed := true; changed; {
		changed = false
		for _, mb := range f.blocks {
			for _, c := range slices.Concat(mb.heads, mb.copies) {
				if c.dst.float && c.src.float && busy[c.dst] != busy[c.src] {
					busy[c.dst], busy[c.src] = true, true
					changed = true
				}
			}
		}
	}
	for _, r := range f.vregs {
		if r.float && !busy[r] {
			r.float = false
		}
	}
}

func (mb *mblock) restrictFloats() {
	if mb.control != nil {
		mb.control.float = false
	}
	if mb.fused != nil && isFloat(mb.fused.Type) {
		return
	}
	for _, r := range mb.compare {
		r.float = false
	}
}

func fitsImmediate(v *ir.Value) bool {
	switch v.Op {
	case ir.OpConst:
		return v.Aux >= -1<<31 && v.Aux < 1<<31
	case ir.OpUndef:
		return true
	}
	return false
}

func (l *lowering) immediate(v *ir.Value) bool {
	if !fitsImmediate(v) {
		return false
	}
	l.vreg(v)
	return true
}

func (l *lowering) newVreg() *vreg {
	return l.fn.newVreg()
}

func (f *mfunc) newVreg() *vreg {
	r := &vreg{spill: -1}
	f.vregs = append(f.vregs, r)
	return r
}

func fusible(b *ir.Block) *ir.Value {
	if b.Kind != ir.If || len(b.Values) == 0 {
		return nil
	}
	last := b.Values[len(b.Values)-1]
	if last != b.Control || !last.Op.Compare() || last.Uses != 1 {
		return nil
	}
	return last
}

func (l *lowering) block(b *ir.Block) {
	mb := l.blocks[b]
	mb.fused = fusible(b)
	mb.table = b.Table
	var call *ir.Value
	for _, v := range b.Values {
		if v == mb.fused || v.Op == ir.OpPhi || l.immediate(v) || (v.Op == ir.OpParam && v.Uses == 0) {
			continue
		}
		in := &instr{op: v.Op, value: v}
		switch {
		case v.Op.IsCall():
			call = v
		case v.Op == ir.OpResult:
			in.call = call
		}
		if v.Op != ir.OpResult {
			for _, arg := range v.Args {
				in.args = append(in.args, l.vreg(arg))
			}
		}
		if v.Op == ir.OpLoad || v.Op == ir.OpStore {
			in.args[0] = newAddress(in.args[0], v.Aux, v.Type)
		}
		if v.Op.HasResult() {
			in.dst = l.vreg(v)
		}
		mb.instrs = append(mb.instrs, in)
	}
	for _, s := range b.Succs {
		mb.succs = append(mb.succs, l.blocks[s])
	}
	switch {
	case mb.fused != nil:
		mb.compare = []*vreg{l.vreg(mb.fused.Args[0]), l.vreg(mb.fused.Args[1])}
	case b.Control != nil:
		mb.control = l.vreg(b.Control)
	}
	for _, r := range b.Results {
		mb.results = append(mb.results, l.vreg(r))
	}
}

func (l *lowering) phiCopies(b *ir.Block) {
	for _, v := range b.Values {
		if v.Op != ir.OpPhi {
			break
		}
		for j, pred := range b.Preds {
			mp := l.blocks[pred]
			mp.copies = append(mp.copies, &phiCopy{dst: l.vreg(v), src: l.vreg(v.Args[j])})
		}
	}
}
