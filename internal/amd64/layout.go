package amd64

import (
	"fmt"
	"slices"

	"lowlang/internal/ir"
)

const maxRotatedInstrs = 12

func (mb *mblock) rotatable(latch *mblock) bool {
	return mb != latch && mb.kind == ir.If && !mb.poll && len(mb.copies) == 0 && len(mb.instrs) <= maxRotatedInstrs
}

func (f *mfunc) rotateLoops() {
	rotated := 0
	for _, mb := range slices.Clone(f.blocks) {
		if mb.kind != ir.Plain || !mb.poll {
			continue
		}
		head := mb.succs[0]
		if !head.rotatable(mb) {
			continue
		}
		if len(head.instrs) > 0 {
			rotated++
			again := head.duplicate(fmt.Sprintf("%sr%d", head.label, rotated))
			mb.succs = []*mblock{again}
			f.blocks = append(f.blocks, again)
			continue
		}
		mb.kind = head.kind
		mb.fused = head.fused
		mb.compare = slices.Clone(head.compare)
		mb.control = head.control
		mb.succs = slices.Clone(head.succs)
	}
}

func (f *mfunc) layout() {
	visited := map[*mblock]bool{}
	var post []*mblock
	var visit func(mb *mblock)
	visit = func(mb *mblock) {
		visited[mb] = true
		for i := len(mb.succs) - 1; i >= 0; i-- {
			if s := mb.succs[i]; !visited[s] {
				visit(s)
			}
		}
		post = append(post, mb)
	}
	visit(f.blocks[0])
	slices.Reverse(post)
	f.blocks = post
	f.sinkRareArms()
}

func (mb *mblock) rareFirstArm() bool {
	if mb.kind != ir.If || mb.poll || mb.fused == nil || mb.fused.Op != ir.OpEq || isFloat(mb.fused.Type) {
		return false
	}
	return mb.fused.Args[1].Op == ir.OpConst && mb.succs[0] != mb.succs[1]
}

func (f *mfunc) sinkRareArms() {
	loops := f.loops()
	for _, mb := range slices.Clone(f.blocks) {
		if !mb.rareFirstArm() {
			continue
		}
		if body := innermostLoop(loops, mb); body != nil {
			f.sinkArm(mb, body)
		}
	}
}

func innermostLoop(loops []*ir.NaturalLoop[*mblock], mb *mblock) map[*mblock]bool {
	for _, l := range loops {
		if l.Body[mb] {
			return l.Body
		}
	}
	return nil
}

func (mb *mblock) heavy() bool {
	return mb.poll || mb.kind == ir.Ret || mb.kind == ir.Unreachable || mb.hasCall()
}

func (f *mfunc) sinkArm(mb *mblock, body map[*mblock]bool) {
	at := slices.Index(f.blocks, mb)
	join := slices.Index(f.blocks, mb.succs[1])
	if !body[mb.succs[1]] || join <= at+1 || f.blocks[at+1] != mb.succs[0] {
		return
	}
	last := join
	for i := join; i < len(f.blocks); i++ {
		if body[f.blocks[i]] {
			last = i
		}
	}
	arm := slices.Clone(f.blocks[at+1 : join])
	if !slices.ContainsFunc(arm, (*mblock).heavy) {
		return
	}
	rest := slices.Clone(f.blocks[join : last+1])
	copy(f.blocks[at+1:], rest)
	copy(f.blocks[at+1+len(rest):], arm)
}
