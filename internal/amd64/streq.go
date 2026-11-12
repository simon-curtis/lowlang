package amd64

import (
	"fmt"

	"lowlang/internal/ir"
)

const maxInlineLiteral = 16

type chunk struct {
	width  int
	offset int
}

func (s *selector) fuseLiteralEquality() {
	for _, mb := range s.f.blocks {
		for _, in := range mb.instrs {
			if in.isStringEquality() && !s.dead[in] {
				s.fuseLiteral(in)
			}
		}
	}
}

func (s *selector) fuseLiteral(in *instr) {
	for _, side := range []int{2, 0} {
		other := 2 - side
		pointer, length := in.args[other], in.args[other+1]
		if pointer.konst != nil || length.konst != nil {
			continue
		}
		text, ok := s.literalAt(in, side)
		if !ok {
			continue
		}
		literal := in.args[side]
		s.uses[literal]--
		s.uses[in.args[side+1]]--
		in.args = []*vreg{pointer, length}
		in.literal = &text
		if s.uses[literal] == 0 {
			s.kill(literal)
		}
		return
	}
}

func (s *selector) literalAt(in *instr, side int) (string, bool) {
	d := s.defs[in.args[side]]
	if d == nil || s.dead[d] || d.op != ir.OpString || len(d.value.Str) > maxInlineLiteral {
		return "", false
	}
	length := in.args[side+1].konst
	if length == nil || *length != int64(len(d.value.Str)) {
		return "", false
	}
	return d.value.Str, true
}

func literalChunks(n int) []chunk {
	switch {
	case n == 0:
		return nil
	case n > 8:
		return []chunk{{8, 0}, {8, n - 8}}
	case n == 8:
		return []chunk{{8, 0}}
	case n > 4:
		return []chunk{{4, 0}, {4, n - 4}}
	case n == 4:
		return []chunk{{4, 0}}
	case n == 3:
		return []chunk{{2, 0}, {1, 2}}
	}
	return []chunk{{n, 0}}
}

func (c chunk) value(text string) uint64 {
	var word uint64
	for i := 0; i < c.width; i++ {
		word |= uint64(text[c.offset+i]) << (8 * i)
	}
	return word
}

func (c chunk) operand(base string) string {
	width, narrowed := widths[8*c.width]
	if !narrowed {
		width = "qword"
	}
	if c.offset == 0 {
		return fmt.Sprintf("%s ptr [%s]", width, base)
	}
	return fmt.Sprintf("%s ptr [%s+%d]", width, base, c.offset)
}

func (g *Generator) literalEquality(fr *frame, in *instr) {
	text := *in.literal
	pointer, length := fr.loc(in.args[0]), fr.loc(in.args[1])
	g.emit("cmp %s, %d", length, len(text))
	done := g.newLabel()
	if chunks := literalChunks(len(text)); len(chunks) > 0 {
		g.emit("jne %s", done)
		if !isReg(pointer) {
			g.move("r10", pointer)
			pointer = "r10"
		}
		for i, c := range chunks {
			if i > 0 {
				g.emit("jne %s", done)
			}
			g.compareChunk(c, pointer, text)
		}
	}
	g.label(done)
	g.emit("sete al")
	g.emit("movzx eax, al")
}

func (g *Generator) compareChunk(c chunk, pointer, text string) {
	if c.width == 8 {
		g.emit("mov r11, %#x", c.value(text))
		g.emit("cmp %s, r11", c.operand(pointer))
		return
	}
	g.emit("cmp %s, %#x", c.operand(pointer), c.value(text))
}

func (g *Generator) stringEquality(fr *frame, in *instr) {
	moves := make([]pmove, len(in.args))
	for i, a := range in.args {
		moves[i] = pmove{dst: abi.args[i], src: fr.loc(a)}
	}
	g.parallel(moves)
	done := g.newLabel()
	g.emit("xor eax, eax")
	g.emit("cmp %s, %s", abi.args[1], abi.args[3])
	g.emit("jne %s", done)
	g.leaveVectors()
	g.emit("call rt_str_eq")
	g.safepointAt(fr, in, done)
}

func (in *instr) isStringEquality() bool {
	return in.op == ir.OpRuntime && in.value.Runtime.Conv == ir.CallStrEq
}
