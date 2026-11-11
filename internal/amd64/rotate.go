package amd64

import (
	"lowlang/internal/ir"
	"lowlang/internal/types"
)

func (s *selector) fuseRotates() {
	for _, mb := range s.f.blocks {
		for _, in := range mb.instrs {
			if in.op == ir.OpOr && !s.dead[in] && len(in.args) == 2 && rotatable(in.value.Type) {
				s.fuseRotate(in)
			}
		}
	}
}

func rotatable(t *types.Type) bool {
	return t != nil && t.Integer() && !t.Signed && (t.Bits == 32 || t.Bits == 64)
}

func (s *selector) fuseRotate(in *instr) {
	t := in.value.Type
	left, right := s.shiftOf(in.args[0], ir.OpShl, t), s.shiftOf(in.args[1], ir.OpShr, t)
	if left == nil || right == nil {
		left, right = s.shiftOf(in.args[1], ir.OpShl, t), s.shiftOf(in.args[0], ir.OpShr, t)
	}
	if left == nil || right == nil || left.args[0] != right.args[0] {
		return
	}
	amount := *left.args[1].konst
	if amount+*right.args[1].konst != int64(t.Bits) {
		return
	}
	x := left.args[0]
	s.dead[left], s.dead[right] = true, true
	s.uses[x]--
	in.args = []*vreg{x}
	in.rotate = amount
}

func (s *selector) shiftOf(r *vreg, op ir.Op, t *types.Type) *instr {
	d := s.defs[r]
	if d == nil || s.dead[d] || s.uses[r] != 1 || r.fixed() || d.op != op || d.value.Type != t {
		return nil
	}
	count := d.args[1].konst
	if count == nil || *count <= 0 || *count >= int64(t.Bits) || d.args[0].fixed() {
		return nil
	}
	return d
}

func (g *Generator) rotate(fr *frame, in *instr) {
	dst := fr.loc(in.dst)
	work := g.workRegister(dst)
	g.move(work, fr.loc(in.args[0]))
	operand := work
	if in.word32() {
		operand = dword(work)
	}
	g.emit("rol %s, %d", operand, in.rotate)
	g.move(dst, work)
}
