package ir

import (
	"iter"
	"slices"
)

const (
	maxSpecialisationRounds = 4
	maxClonesPerFunction    = 4
	maxCloneSourceCost      = 150
	maxClonedProgramCost    = 4000
)

type callSite struct {
	call   *Value
	caller *Func
}

type specialiser struct {
	program  *Program
	cloned   int
	clonesOf map[*Func]int
	made     map[cloneKey]madeClone
	names    map[string]int
}

type cloneKey struct {
	source *Func
	word   int64
	code   *Func
}

type madeClone struct {
	fn      *Func
	dropped []int64
	capture *Value
}

func (p *Program) specialiseConstantArguments() {
	s := &specialiser{program: p, clonesOf: map[*Func]int{}, made: map[cloneKey]madeClone{}, names: map[string]int{}}
	for round := 0; round < maxSpecialisationRounds; round++ {
		constants := s.specialiseConstants()
		clones := s.cloneForFunctions()
		if !constants && !clones {
			return
		}
	}
}

func (s *specialiser) specialiseConstants() bool {
	sites, escaping := s.program.directCalls()
	changed := map[*Func]bool{}
	for _, g := range s.program.Funcs {
		if escaping[g] || !s.eligible(g, sites[g]) {
			continue
		}
		dropped := specialiseParams(g, sites[g])
		dropped = append(dropped, specialiseIntegers(g, sites[g])...)
		if len(dropped) == 0 {
			continue
		}
		dropParameters(g, sites[g], dropped)
		s.forget(g)
		changed[g] = true
		for _, site := range sites[g] {
			changed[site.caller] = true
		}
	}
	simplifyAll(s.program, changed)
	return len(changed) > 0
}

func (s *specialiser) forget(g *Func) {
	for key, made := range s.made {
		if key.source == g || made.fn == g {
			delete(s.made, key)
		}
	}
}

func (s *specialiser) eligible(g *Func, sites []callSite) bool {
	return len(sites) > 0 && g != s.program.Main && !g.Callback
}

func simplifyAll(p *Program, changed map[*Func]bool) {
	for _, f := range p.Funcs {
		if changed[f] {
			simplify(f)
		}
	}
}

func (p *Program) directCalls() (map[*Func][]callSite, map[*Func]bool) {
	sites := map[*Func][]callSite{}
	escaping := map[*Func]bool{}
	for _, f := range p.Funcs {
		for v := range f.directTargets() {
			if v.Op == OpCall {
				sites[v.Target] = append(sites[v.Target], callSite{call: v, caller: f})
			} else {
				escaping[v.Target] = true
			}
		}
	}
	return sites, escaping
}

func specialiseParams(g *Func, sites []callSite) []int64 {
	var dropped []int64
	var consumed *Value
	for param, env := range codeEnvPairs(g) {
		if param == consumed {
			continue
		}
		code := agreedConstant(g, param, sites, isFunction)
		if code == nil {
			continue
		}
		capture := agreedConstant(g, env, sites, isConstant)
		if capture == nil {
			continue
		}
		dropped = append(dropped, param.Aux, env.Aux)
		becomeCopyOf(param, code)
		becomeCopyOf(env, capture)
		consumed = env
	}
	return dropped
}

func codeEnvPairs(g *Func) iter.Seq2[*Value, *Value] {
	return func(yield func(*Value, *Value) bool) {
		params := parameters(g)
		for i := 0; i+1 < len(params); i++ {
			param, env := params[i], params[i+1]
			if env.Aux == param.Aux+1 && !yield(param, env) {
				return
			}
		}
	}
}

type agreement struct {
	param    *Value
	constant *Value
}

func specialiseIntegers(g *Func, sites []callSite) []int64 {
	var agreed []agreement
	for _, param := range parameters(g) {
		if constant := agreedConstant(g, param, sites, isInteger); constant != nil {
			agreed = append(agreed, agreement{param: param, constant: constant})
		}
	}
	if len(agreed) == 0 || !unlocksFolding(g, agreed) {
		return nil
	}
	dropped := make([]int64, 0, len(agreed))
	for _, a := range agreed {
		dropped = append(dropped, a.param.Aux)
		becomeCopyOf(a.param, a.constant)
	}
	return dropped
}

func isInteger(v *Value) bool {
	return v.Op == OpConst && v.FloatBits == 0
}

type shape struct {
	checks int
	blocks int
	values int
}

func shapeOf(f *Func) shape {
	s := shape{blocks: len(f.Blocks)}
	for _, b := range f.Blocks {
		for _, v := range b.Values {
			switch v.Op {
			case OpCheck:
				s.checks++
			case OpConst, OpParam, OpUndef:
			default:
				s.values++
			}
		}
	}
	return s
}

func (s shape) smallerThan(other shape) bool {
	return s.checks < other.checks || s.blocks < other.blocks || s.values < other.values
}

func unlocksFolding(g *Func, agreed []agreement) bool {
	baseline := cloneFunc(g)
	simplify(baseline.fn)
	trial := cloneFunc(g)
	for _, a := range agreed {
		substituted := trial.values[a.param]
		becomeCopyOf(substituted, a.constant)
	}
	simplify(trial.fn)
	before := shapeOf(baseline.fn)
	after := shapeOf(trial.fn)
	return after.smallerThan(before)
}

func simplify(f *Func) {
	s := newSSA(f)
	s.finalize()
}

func dropParameters(g *Func, sites []callSite, dropped []int64) {
	for _, param := range parameters(g) {
		param.Aux -= int64(countBelow(dropped, param.Aux))
	}
	for _, site := range sites {
		site.call.Args = withoutWords(site.call.Args, dropped)
	}
	g.ParamWords -= len(dropped)
}

func countBelow(words []int64, limit int64) int {
	n := 0
	for _, w := range words {
		if w < limit {
			n++
		}
	}
	return n
}

func withoutWords(args []*Value, dropped []int64) []*Value {
	kept := make([]*Value, 0, len(args))
	for i, a := range args {
		if !slices.Contains(dropped, int64(i)) {
			kept = append(kept, a)
		}
	}
	return kept
}

func parameters(g *Func) []*Value {
	var params []*Value
	for _, v := range g.Blocks[0].Values {
		if v.Op == OpParam {
			params = append(params, v)
		}
	}
	return params
}

func agreedConstant(self *Func, param *Value, sites []callSite, accept func(*Value) bool) *Value {
	var agreed *Value
	for _, site := range sites {
		if int(param.Aux) >= len(site.call.Args) {
			return nil
		}
		arg := site.call.Args[param.Aux]
		switch {
		case site.caller == self && arg.Op == OpParam && arg.Aux == param.Aux:
		case !accept(arg):
			return nil
		case agreed == nil:
			agreed = arg
		case !sameConstant(agreed, arg):
			return nil
		}
	}
	return agreed
}

func isFunction(v *Value) bool {
	return v.Op == OpFuncAddr && v.Target != nil && v.Aux == CodeAddr
}

func isConstant(v *Value) bool {
	return v.Op == OpConst
}

func sameConstant(a, b *Value) bool {
	return a.Op == b.Op && a.Aux == b.Aux && a.Target == b.Target && a.FloatBits == b.FloatBits && a.Str == b.Str
}

func becomeCopyOf(v, constant *Value) {
	v.Op = constant.Op
	v.Aux = constant.Aux
	v.Target = constant.Target
	v.Str = constant.Str
	v.FloatBits = constant.FloatBits
	v.Type = constant.Type
	v.Scalar = constant.Scalar
}
