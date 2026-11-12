package amd64

import (
	"fmt"

	"lowlang/internal/ir"
	"lowlang/internal/types"
)

type class int

const (
	integer class = iota
	vector
	stack
	byRef
)

type cArg struct {
	class class
	words int
	reg   string
	xmm   string
	slot  int
	copy  int
}

type cPlan struct {
	symbol     string
	args       []cArg
	results    []class
	memory     bool
	vectors    int
	stackWords int
	copyWords  int
	ret        *types.Type
}

func (p *cPlan) copyBase() int {
	if p.memory {
		return p.stackWords + p.ret.Size()
	}
	return p.stackWords
}

func (p *cPlan) outgoingWords() int {
	return p.copyBase() + p.copyWords
}

func (g *Generator) plan(sig *ir.CSig) *cPlan {
	plan, ok := g.plans[sig]
	if !ok {
		plan = g.platform.classify(sig)
		g.plans[sig] = plan
	}
	return plan
}

func eightbytes(t *types.Type) []class {
	if !t.CStruct() {
		if t.Float() {
			return []class{vector}
		}
		return []class{integer}
	}
	classes := make([]class, t.Size())
	for k := range classes {
		classes[k] = vector
	}
	var visit func(s *types.Type, base int)
	visit = func(s *types.Type, base int) {
		for _, f := range s.Fields {
			at := base + f.Bytes
			if f.Type.CStruct() {
				visit(f.Type, at)
				continue
			}
			if !f.Type.Float() {
				classes[at/8] = integer
			}
		}
	}
	visit(t, 0)
	return classes
}

type argRegisters struct {
	ints    int
	vectors int
}

func (r *argRegisters) assign(classes []class) ([]cArg, bool) {
	ints, vectors := r.ints, r.vectors
	args := make([]cArg, len(classes))
	for i, c := range classes {
		if c == integer {
			if ints == len(sysvIntRegs) {
				return nil, false
			}
			args[i] = cArg{class: integer, words: 1, reg: sysvIntRegs[ints], slot: -1}
			ints++
			continue
		}
		if vectors == sysvVectorRegs {
			return nil, false
		}
		args[i] = cArg{class: vector, words: 1, xmm: fmt.Sprintf("xmm%d", vectors), slot: -1}
		vectors++
	}
	r.ints, r.vectors = ints, vectors
	return args, true
}

func sysvPlan(sig *ir.CSig) *cPlan {
	plan := &cPlan{symbol: sig.Symbol, ret: sig.Ret}
	regs := &argRegisters{}
	if sig.Ret != types.Void {
		if sig.Ret.CStruct() && sig.Ret.Bytes > 16 {
			plan.memory = true
			regs.ints = 1
		} else {
			plan.results = eightbytes(sig.Ret)
		}
	}
	for i, t := range sig.Args {
		classes := []class{integer}
		if !sig.ByRef[i] {
			classes = eightbytes(t)
		}
		var assigned []cArg
		fits := false
		oversized := t.CStruct() && !sig.ByRef[i] && t.Bytes > 16
		if !oversized {
			assigned, fits = regs.assign(classes)
		}
		if !fits {
			for range classes {
				plan.args = append(plan.args, cArg{class: stack, words: 1, slot: plan.stackWords})
				plan.stackWords++
			}
			continue
		}
		plan.args = append(plan.args, assigned...)
	}
	plan.vectors = regs.vectors
	return plan
}

func win64Register(t *types.Type) bool {
	return !t.CStruct() || t.Bytes == 1 || t.Bytes == 2 || t.Bytes == 4 || t.Bytes == 8
}

func win64Class(t *types.Type) class {
	if !t.CStruct() && t.Float() {
		return vector
	}
	return integer
}

func win64Plan(sig *ir.CSig) *cPlan {
	plan := &cPlan{symbol: sig.Symbol, ret: sig.Ret}
	slots := 0
	if sig.Ret != types.Void {
		if win64Register(sig.Ret) {
			plan.results = []class{win64Class(sig.Ret)}
		} else {
			plan.memory = true
			slots = 1
		}
	}
	for i, t := range sig.Args {
		arg := cArg{class: integer, words: 1, slot: -1}
		switch {
		case sig.ByRef[i]:
		case !win64Register(t):
			arg = cArg{class: byRef, words: t.Size(), slot: -1, copy: plan.copyWords}
			plan.copyWords += t.Size()
		default:
			arg.class = win64Class(t)
		}
		if slots < len(win64IntRegs) {
			arg.reg = win64IntRegs[slots]
			if arg.class == vector {
				arg.xmm = fmt.Sprintf("xmm%d", slots)
			}
		} else {
			arg.slot = slots
		}
		plan.args = append(plan.args, arg)
		slots++
	}
	plan.stackWords = max(len(win64IntRegs), slots)
	return plan
}
