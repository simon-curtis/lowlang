package amd64

import (
	"fmt"
	"slices"

	"lowlang/internal/ir"
	"lowlang/internal/types"
)

const maxDisplacement = 1 << 30

type address struct {
	base   *vreg
	index  *vreg
	scale  int64
	disp   int64
	loaded *types.Type
}

func newAddress(base *vreg, disp int64, loaded *types.Type) *vreg {
	return &vreg{spill: -1, mem: &address{base: base, scale: 1, disp: disp, loaded: loaded}}
}

func (a *address) pieces() []*vreg {
	var pieces []*vreg
	if a.base != nil && a.base.frame == nil {
		pieces = append(pieces, a.base)
	}
	if a.index != nil {
		pieces = append(pieces, a.index)
	}
	return pieces
}

func (a *address) uses(r *vreg) bool {
	return a.base == r || a.index == r
}

func (a *address) renamed(from, to *vreg) *vreg {
	c := *a
	if c.base == from {
		c.base = to
	}
	if c.index == from {
		c.index = to
	}
	return &vreg{spill: -1, mem: &c}
}

func (fr *frame) piece(r *vreg, scratch string) string {
	loc := fr.loc(r)
	if isReg(loc) {
		return loc
	}
	return scratch
}

func (fr *frame) address(r *vreg) string {
	m := r.mem
	disp := m.disp
	base := "rbp"
	if m.base.frame != nil {
		disp += int64(fr.slot(int(m.base.frame.Aux)))
	} else {
		base = fr.piece(m.base, "r10")
	}
	text := base
	if m.index != nil {
		text = fmt.Sprintf("%s+%s*%d", base, fr.piece(m.index, "r11"), m.scale)
	}
	if disp != 0 {
		text = fmt.Sprintf("%s%+d", text, disp)
	}
	return fmt.Sprintf("[%s]", text)
}

func (g *Generator) reloadPiece(fr *frame, r *vreg, scratch string) {
	if loc := fr.loc(r); !isReg(loc) {
		g.move(scratch, loc)
	}
}

func (g *Generator) reloadPieces(fr *frame, r *vreg) {
	if r.mem == nil {
		return
	}
	if r.mem.base.frame == nil {
		g.reloadPiece(fr, r.mem.base, "r10")
	}
	if r.mem.index != nil {
		g.reloadPiece(fr, r.mem.index, "r11")
	}
}

func (r *vreg) narrowMemory() bool {
	return r.mem != nil && r.mem.loaded != nil && r.mem.loaded.Integer() && r.mem.loaded.Bits < 64
}

func (fr *frame) overwritesAddress(in *instr, dst string) bool {
	m := in.memoryOperand()
	if m == nil {
		return false
	}
	return slices.ContainsFunc(m.mem.pieces(), func(p *vreg) bool { return fr.loc(p) == dst })
}

func (in *instr) memoryOperand() *vreg {
	for _, a := range in.args {
		if a.mem != nil {
			return a
		}
	}
	return nil
}

type selector struct {
	f       *mfunc
	defs    map[*vreg]*instr
	uses    map[*vreg]int
	address map[*vreg]int
	dead    map[*instr]bool
}

func newSelector(f *mfunc) *selector {
	s := &selector{
		f:       f,
		defs:    map[*vreg]*instr{},
		uses:    map[*vreg]int{},
		address: map[*vreg]int{},
		dead:    map[*instr]bool{},
	}
	s.count()
	return s
}

func (s *selector) foldAddresses() {
	for _, mb := range s.f.blocks {
		for _, in := range mb.instrs {
			if in.op == ir.OpLoad || in.op == ir.OpStore {
				s.fold(in.args[0].mem)
			}
		}
	}
}

func (s *selector) count() {
	for _, mb := range s.f.blocks {
		for _, in := range mb.instrs {
			if in.dst != nil {
				s.defs[in.dst] = in
			}
			for _, a := range in.args {
				s.use(a)
			}
			if in.op == ir.OpLoad || in.op == ir.OpStore {
				s.address[in.args[0].mem.base]++
			}
		}
		for _, r := range mb.terminatorUses() {
			s.use(r)
		}
		for _, c := range mb.copies {
			s.use(c.src)
		}
	}
}

func (s *selector) use(r *vreg) {
	if r.mem != nil {
		for _, p := range r.mem.pieces() {
			s.uses[p]++
		}
		return
	}
	s.uses[r]++
}

func (s *selector) foldable(v *vreg) *instr {
	d := s.defs[v]
	if d == nil || v.fixed() || s.dead[d] || s.uses[v] != s.address[v] {
		return nil
	}
	if d.op != ir.OpAdd || !wordArithmetic(d.value.Type) {
		return nil
	}
	return d
}

func wordArithmetic(t *types.Type) bool {
	return t == nil || (t.Integer() && t.Bits == 64)
}

func (s *selector) fold(m *address) {
	for {
		d := s.foldable(m.base)
		if d == nil || !s.absorb(m, d) {
			return
		}
	}
}

func (s *selector) absorb(m *address, d *instr) bool {
	x, y := d.args[0], d.args[1]
	next := *m
	switch {
	case y.konst != nil:
		next.base, next.disp = x, m.disp+*y.konst
	case x.konst != nil:
		next.base, next.disp = y, m.disp+*x.konst
	case m.index != nil:
		return false
	default:
		if !s.split(&next, x, y) {
			return false
		}
	}
	if next.disp <= -maxDisplacement || next.disp >= maxDisplacement {
		return false
	}
	old := m.base
	s.address[old]--
	s.uses[old]--
	s.address[next.base]++
	s.uses[next.base]++
	if next.index != nil {
		s.uses[next.index]++
	}
	*m = next
	if s.uses[old] == 0 {
		s.kill(old)
	}
	return true
}

func (s *selector) split(m *address, x, y *vreg) bool {
	if index, scale := s.scaled(y); index != nil && x.frame == nil {
		m.base, m.index, m.scale = x, index, scale
		return true
	}
	if index, scale := s.scaled(x); index != nil && y.frame == nil {
		m.base, m.index, m.scale = y, index, scale
		return true
	}
	switch {
	case y.frame != nil && x.frame != nil:
		return false
	case y.frame != nil:
		x, y = y, x
	case x.fixed() || y.fixed():
		return false
	}
	m.base, m.index, m.scale = x, y, 1
	return true
}

func (s *selector) scaled(r *vreg) (*vreg, int64) {
	d := s.defs[r]
	if d == nil || s.dead[d] || s.uses[r] != 1 || r.fixed() || (d.op != ir.OpShl && d.op != ir.OpMul) || !wordArithmetic(d.value.Type) {
		return nil, 0
	}
	x, k := d.args[0], d.args[1].konst
	if k == nil || x.fixed() {
		return nil, 0
	}
	switch {
	case d.op == ir.OpShl && *k >= 0 && *k <= 3:
		return x, 1 << *k
	case d.op == ir.OpMul && (*k == 1 || *k == 2 || *k == 4 || *k == 8):
		return x, *k
	}
	return nil, 0
}

func (s *selector) kill(r *vreg) {
	d := s.defs[r]
	if d == nil || s.dead[d] || !discardable(d) {
		return
	}
	s.dead[d] = true
	for _, a := range d.args {
		if a.mem != nil || a.fixed() {
			continue
		}
		s.uses[a]--
		if s.uses[a] == 0 {
			s.kill(a)
		}
	}
}

func discardable(in *instr) bool {
	switch in.op {
	case ir.OpAdd, ir.OpShl, ir.OpMul:
		return wordArithmetic(in.value.Type)
	case ir.OpString:
		return true
	}
	return false
}

func (s *selector) sweep() {
	for _, mb := range s.f.blocks {
		kept := mb.instrs[:0]
		for _, in := range mb.instrs {
			if !s.dead[in] && !s.unusedConstant(in) {
				kept = append(kept, in)
			}
		}
		mb.instrs = kept
	}
}

func (s *selector) unusedConstant(in *instr) bool {
	return in.op == ir.OpConst && s.uses[in.dst] == 0
}
