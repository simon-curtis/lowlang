package ir

import (
	"fmt"
	"slices"
)

type functionGroup struct {
	code  *Value
	sites []callSite
}

func (s *specialiser) cloneForFunctions() bool {
	sites, _ := s.program.directCalls()
	changed := map[*Func]bool{}
	funcs := make([]*Func, 0, len(s.program.Funcs))
	for _, g := range s.program.Funcs {
		funcs = append(funcs, g)
		if !s.eligible(g, sites[g]) {
			continue
		}
		clones, repointed := s.cloneForFunctionArguments(g, sites[g])
		funcs = append(funcs, clones...)
		for _, site := range repointed {
			changed[site.caller] = true
		}
	}
	s.program.Funcs = funcs
	simplifyAll(s.program, changed)
	return len(changed) > 0
}

func (s *specialiser) cloneForFunctionArguments(g *Func, sites []callSite) ([]*Func, []callSite) {
	cost := costOf(g)
	if cost > maxCloneSourceCost {
		return nil, nil
	}
	for param, env := range codeEnvPairs(g) {
		if !callsParameter(g, param) {
			continue
		}
		groups := groupByFunction(sites, param.Aux)
		if !worthCloning(groups, env) {
			continue
		}
		return s.cloneGroups(g, param, env, groups, cost)
	}
	return nil, nil
}

func (s *specialiser) cloneGroups(g *Func, param, env *Value, groups []functionGroup, cost int) ([]*Func, []callSite) {
	var clones []*Func
	var repointed []callSite
	allowed := max(0, min(len(groups), maxClonesPerFunction-s.clonesOf[g]))
	for _, group := range groups[:allowed] {
		if s.cloned+cost > maxClonedProgramCost {
			break
		}
		clone, sites := cloneForFunction(g, param, env, group)
		s.cloned += cost
		s.clonesOf[g]++
		clones = append(clones, clone)
		repointed = append(repointed, sites...)
	}
	return clones, repointed
}

func cloneForFunction(g *Func, param, env *Value, group functionGroup) (*Func, []callSite) {
	c := cloneFunc(g)
	clone := c.fn
	clone.Name = fmt.Sprintf("%s..with%d.%s", g.Name, param.Aux, group.code.Target.Name)
	dropped := []int64{param.Aux}
	becomeCopyOf(c.values[param], group.code)
	if capture := agreedConstant(nil, env, group.sites, isConstant); capture != nil {
		becomeCopyOf(c.values[env], capture)
		dropped = append(dropped, env.Aux)
	}
	sites := slices.Clone(group.sites)
	for _, call := range selfCalls(c, g, param, env) {
		sites = append(sites, callSite{call: call, caller: clone})
	}
	dropParameters(clone, sites, dropped)
	for _, site := range sites {
		site.call.Target = clone
	}
	simplify(clone)
	return clone, group.sites
}

func selfCalls(c *funcClone, g *Func, param, env *Value) []*Value {
	code, capture := c.values[param], c.values[env]
	var calls []*Value
	for _, b := range c.fn.Blocks {
		for _, v := range b.Values {
			if v.Op == OpCall && !v.Indirect() && v.Target == g && len(v.Args) > int(env.Aux) && v.Args[param.Aux] == code && v.Args[env.Aux] == capture {
				calls = append(calls, v)
			}
		}
	}
	return calls
}

func callsParameter(g *Func, param *Value) bool {
	for _, b := range g.Blocks {
		for _, v := range b.Values {
			switch {
			case v.Op != OpCall:
			case v.Indirect() && v.Args[0] == param:
				return true
			case !v.Indirect() && slices.Contains(v.Args, param):
				return true
			}
		}
	}
	return false
}

func groupByFunction(sites []callSite, word int64) []functionGroup {
	var groups []functionGroup
	for _, site := range sites {
		if int(word) >= len(site.call.Args) {
			continue
		}
		arg := site.call.Args[word]
		if !isFunction(arg) {
			continue
		}
		k := slices.IndexFunc(groups, func(g functionGroup) bool { return g.code.Target == arg.Target })
		if k < 0 {
			groups = append(groups, functionGroup{code: arg})
			k = len(groups) - 1
		}
		groups[k].sites = append(groups[k].sites, site)
	}
	return groups
}

func worthCloning(groups []functionGroup, env *Value) bool {
	switch len(groups) {
	case 0:
		return false
	case 1:
		return agreedConstant(nil, env, groups[0].sites, isConstant) == nil
	}
	return true
}
