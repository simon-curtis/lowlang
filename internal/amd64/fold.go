package amd64

import (
	"math"

	"lowlang/internal/ir"
	"lowlang/internal/types"
)

func (f *mfunc) selectInstructions() {
	s := newSelector(f)
	s.negateCompares()
	s.foldAddresses()
	s.doubleFloats()
	s.foldLoads()
	s.fuseRotates()
	s.fuseLiteralEquality()
	s.sweep()
}

func (in *instr) writesMemory() bool {
	switch in.op {
	case ir.OpStore, ir.OpVector, ir.OpVectorScalar:
		return true
	}
	return in.op.IsCall()
}

func (s *selector) doubleFloats() {
	for _, mb := range s.f.blocks {
		for _, in := range mb.instrs {
			if in.op != ir.OpMul || !isFloat(in.value.Type) {
				continue
			}
			x, y := in.args[0], in.args[1]
			switch {
			case isTwo(y, in.value.Type):
			case isTwo(x, in.value.Type):
				x = y
			default:
				continue
			}
			s.uses[in.args[0]]--
			s.uses[in.args[1]]--
			s.uses[x] += 2
			in.op = ir.OpAdd
			in.args = []*vreg{x, x}
		}
	}
}

func isTwo(r *vreg, t *types.Type) bool {
	if t.Bits == 32 {
		return r.konst != nil && *r.konst == int64(math.Float32bits(2))
	}
	return r.pool != nil && uint64(*r.pool) == math.Float64bits(2)
}

func (s *selector) foldLoads() {
	for _, mb := range s.f.blocks {
		pending := map[*vreg]*instr{}
		for _, in := range mb.instrs {
			if s.dead[in] {
				continue
			}
			s.absorbLoad(in, pending)
			switch {
			case in.writesMemory():
				clear(pending)
			case in.op == ir.OpLoad && s.uses[in.dst] == 1:
				pending[in.dst] = in
			}
		}
		if mb.fused != nil && !mb.poll {
			s.absorbIntoBranch(mb, pending)
		}
	}
}

func (s *selector) take(load *instr, pending map[*vreg]*instr) *vreg {
	delete(pending, load.dst)
	s.dead[load] = true
	s.uses[load.dst]--
	return load.args[0]
}

func (s *selector) pendingLoad(r *vreg, pending map[*vreg]*instr) *instr {
	if load := pending[r]; load != nil && !s.dead[load] {
		return load
	}
	return nil
}

func (s *selector) absorbLoad(in *instr, pending map[*vreg]*instr) {
	if len(pending) == 0 || len(in.args) != 2 || in.memoryOperand() != nil {
		return
	}
	switch {
	case in.op == ir.OpCheck:
		in.args[0], in.args[1] = s.compareOperands(nil, in.args[0], in.args[1], pending)
	case in.op.Compare():
		s.absorbCompare(in, pending)
	case isFloat(in.value.Type) && in.op >= ir.OpAdd && in.op <= ir.OpDiv && in.op != ir.OpRem:
		s.absorbOperand(in, in.value.Type, in.op == ir.OpAdd || in.op == ir.OpMul, pending)
	case wordArithmetic(in.value.Type) && integerBinary(in.op):
		s.absorbOperand(in, in.value.Type, in.op != ir.OpSub, pending)
	}
}

func integerBinary(op ir.Op) bool {
	switch op {
	case ir.OpAdd, ir.OpSub, ir.OpMul, ir.OpAnd, ir.OpOr, ir.OpXor:
		return true
	}
	return false
}

func sameWidth(operation, loaded *types.Type) bool {
	if operation != nil && operation.Float() {
		return (operation.Bits == 32) == (loaded != nil && loaded.Bits == 32)
	}
	return loaded == nil || loaded.Bits == 64
}

func (s *selector) absorbOperand(in *instr, t *types.Type, commutative bool, pending map[*vreg]*instr) {
	second := s.pendingLoad(in.args[1], pending)
	first := s.pendingLoad(in.args[0], pending)
	switch {
	case second != nil && in.args[0] != in.args[1] && sameWidth(t, second.value.Type):
		in.args[1] = s.take(second, pending)
	case first != nil && commutative && in.args[0] != in.args[1] && sameWidth(t, first.value.Type):
		in.args[0] = in.args[1]
		in.args[1] = s.take(first, pending)
	}
}

func (s *selector) absorbCompare(in *instr, pending map[*vreg]*instr) {
	if isFloat(in.value.Type) {
		s.absorbFloatCompare(in, pending)
		return
	}
	in.args[0], in.args[1] = s.compareOperands(in.value.Type, in.args[0], in.args[1], pending)
}

func (s *selector) absorbFloatCompare(in *instr, pending map[*vreg]*instr) {
	_, ordered := orderedOperands(in.op, in.args[0], in.args[1])
	load := s.pendingLoad(ordered, pending)
	if load == nil || !sameWidth(in.value.Type, load.value.Type) || in.args[0] == in.args[1] {
		return
	}
	mem := s.take(load, pending)
	if ordered == in.args[1] {
		in.args[1] = mem
		return
	}
	in.args[0] = mem
}

func (s *selector) compareOperands(t *types.Type, a, b *vreg, pending map[*vreg]*instr) (*vreg, *vreg) {
	if a == b {
		return a, b
	}
	if load := s.pendingLoad(b, pending); load != nil && sameWidth(t, load.value.Type) {
		return a, s.take(load, pending)
	}
	load := s.pendingLoad(a, pending)
	if load == nil {
		return a, b
	}
	if sameWidth(t, load.value.Type) || narrowCompare(t, load.value.Type, b) {
		return s.take(load, pending), b
	}
	return a, b
}

func narrowCompare(operation, loaded *types.Type, other *vreg) bool {
	if loaded == nil || loaded.Bits == 64 || !loaded.Integer() || operation == nil || other.konst == nil {
		return false
	}
	if operation.Bits != loaded.Bits || operation.Signed != loaded.Signed {
		return false
	}
	return loaded.Holds(*other.konst)
}

func (s *selector) absorbIntoBranch(mb *mblock, pending map[*vreg]*instr) {
	t := mb.fused.Type
	if isFloat(t) {
		_, ordered := orderedOperands(mb.fused.Op, mb.compare[0], mb.compare[1])
		load := s.pendingLoad(ordered, pending)
		if load == nil || !sameWidth(t, load.value.Type) || mb.compare[0] == mb.compare[1] {
			return
		}
		mem := s.take(load, pending)
		mb.compare = []*vreg{mb.compare[0], mb.compare[1]}
		if ordered == mb.compare[1] {
			mb.compare[1] = mem
		} else {
			mb.compare[0] = mem
		}
		return
	}
	a, b := s.compareOperands(t, mb.compare[0], mb.compare[1], pending)
	mb.compare = []*vreg{a, b}
}
