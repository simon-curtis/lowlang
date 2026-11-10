package amd64

import (
	"fmt"

	"lowlang/internal/ir"
)

type vreg struct {
	konst       *int64
	start       int
	end         int
	crossesCall bool
	hint        string
	copyOf      *vreg
	reg         string
	spill       int
}

type instr struct {
	op    ir.Op
	move  bool
	dst   *vreg
	args  []*vreg
	value *ir.Value
	call  *ir.Value
	pos   int
}

type mblock struct {
	label   string
	kind    ir.Kind
	instrs  []*instr
	succs   []*mblock
	control *vreg
	fused   *ir.Value
	compare []*vreg
	results []*vreg
	liveIn  map[*vreg]bool
	liveOut map[*vreg]bool
	start   int
	end     int
}

type mfunc struct {
	blocks []*mblock
	vregs  []*vreg
}

type lowering struct {
	fn     *mfunc
	vregs  map[*ir.Value]*vreg
	blocks map[*ir.Block]*mblock
	temps  map[*ir.Value]*vreg
}

func lower(f *ir.Func) *mfunc {
	splitCriticalEdges(f)
	l := &lowering{
		fn:     &mfunc{},
		vregs:  map[*ir.Value]*vreg{},
		blocks: map[*ir.Block]*mblock{},
		temps:  map[*ir.Value]*vreg{},
	}
	for _, b := range f.Blocks {
		mb := &mblock{label: fmt.Sprintf(".L%s_%d", f.Name, b.ID), kind: b.Kind}
		l.blocks[b] = mb
		l.fn.blocks = append(l.fn.blocks, mb)
	}
	for _, b := range f.Blocks {
		l.block(b)
	}
	for _, b := range f.Blocks {
		l.phiMoves(b)
	}
	l.fn.hint()
	return l.fn
}

func (f *mfunc) hint() {
	for _, mb := range f.blocks {
		for _, in := range mb.instrs {
			in.hint()
		}
	}
}

func (in *instr) hint() {
	switch {
	case in.move:
		in.dst.copyOf = in.args[0]
	case in.op == ir.OpCall || in.op == ir.OpRuntime:
		for i, a := range in.args[:min(len(in.args), len(argRegs))] {
			a.hint = argRegs[i]
		}
	}
}

func splitCriticalEdges(f *ir.Func) {
	next := 0
	for _, b := range f.Blocks {
		next = max(next, b.ID+1)
	}
	for _, b := range f.Blocks {
		if len(b.Preds) < 2 || !hasPhi(b) {
			continue
		}
		for j, pred := range b.Preds {
			if len(pred.Succs) < 2 {
				continue
			}
			mid := &ir.Block{ID: next, Kind: ir.Plain, Preds: []*ir.Block{pred}, Succs: []*ir.Block{b}}
			next++
			f.Blocks = append(f.Blocks, mid)
			for i, s := range pred.Succs {
				if s == b {
					pred.Succs[i] = mid
				}
			}
			b.Preds[j] = mid
		}
	}
}

func hasPhi(b *ir.Block) bool {
	return len(b.Values) > 0 && b.Values[0].Op == ir.OpPhi
}

func (l *lowering) vreg(v *ir.Value) *vreg {
	r, ok := l.vregs[v]
	if !ok {
		r = l.newVreg()
		if fitsImmediate(v) {
			r.konst = &v.Aux
		}
		l.vregs[v] = r
	}
	return r
}

func fitsImmediate(v *ir.Value) bool {
	return v.Op == ir.OpConst && v.Aux >= -1<<31 && v.Aux < 1<<31
}

func (l *lowering) immediate(v *ir.Value) bool {
	if !fitsImmediate(v) {
		return false
	}
	l.vreg(v)
	return true
}

func (l *lowering) newVreg() *vreg {
	r := &vreg{start: -1, spill: -1}
	l.fn.vregs = append(l.fn.vregs, r)
	return r
}

func (l *lowering) temp(phi *ir.Value) *vreg {
	r, ok := l.temps[phi]
	if !ok {
		r = l.newVreg()
		l.temps[phi] = r
	}
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
	var call *ir.Value
	for _, v := range b.Values {
		if v == mb.fused || l.immediate(v) {
			continue
		}
		in := &instr{op: v.Op, value: v}
		switch v.Op {
		case ir.OpPhi:
			in.move = true
			in.args = []*vreg{l.temp(v)}
		case ir.OpCall, ir.OpRuntime:
			call = v
		case ir.OpResult:
			in.call = call
		}
		if v.Op != ir.OpPhi && v.Op != ir.OpResult {
			for _, arg := range v.Args {
				in.args = append(in.args, l.vreg(arg))
			}
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

func (l *lowering) phiMoves(b *ir.Block) {
	for _, v := range b.Values {
		if v.Op != ir.OpPhi {
			break
		}
		for j, pred := range b.Preds {
			mp := l.blocks[pred]
			move := &instr{move: true, dst: l.temp(v), args: []*vreg{l.vreg(v.Args[j])}}
			mp.instrs = append(mp.instrs, move)
		}
	}
}
