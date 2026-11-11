package ir

import "slices"

const (
	maxInlineCost    = 48
	maxInlineLoops   = 1
	maxCallerGrowth  = 240
	maxProgramGrowth = 6000
)

type calleeSummary struct {
	cost     int
	eligible bool
}

type inliner struct {
	program   *Program
	recursive map[*Func]bool
	summaries map[*Func]calleeSummary
	grown     int
}

func (p *Program) inline() {
	in := &inliner{program: p, summaries: map[*Func]calleeSummary{}}
	order := in.callOrder()
	for _, f := range order {
		in.inlineInto(f)
	}
}

func directCallees(f *Func) []*Func {
	var callees []*Func
	for _, b := range f.Blocks {
		for _, v := range b.Values {
			if v.Op == OpCall && !v.Indirect && v.Target != nil && !slices.Contains(callees, v.Target) {
				callees = append(callees, v.Target)
			}
		}
	}
	return callees
}

type callGraph struct {
	index     map[*Func]int
	low       map[*Func]int
	onStack   map[*Func]bool
	stack     []*Func
	next      int
	order     []*Func
	recursive map[*Func]bool
	known     map[*Func]bool
}

func (in *inliner) callOrder() []*Func {
	g := &callGraph{
		index:     map[*Func]int{},
		low:       map[*Func]int{},
		onStack:   map[*Func]bool{},
		recursive: map[*Func]bool{},
		known:     map[*Func]bool{},
	}
	for _, f := range in.program.Funcs {
		g.known[f] = true
	}
	for _, f := range in.program.Funcs {
		if _, seen := g.index[f]; !seen {
			g.connect(f)
		}
	}
	in.recursive = g.recursive
	return g.order
}

func (g *callGraph) connect(f *Func) {
	g.index[f] = g.next
	g.low[f] = g.next
	g.next++
	g.stack = append(g.stack, f)
	g.onStack[f] = true
	for _, c := range directCallees(f) {
		if !g.known[c] {
			continue
		}
		if c == f {
			g.recursive[f] = true
		}
		if _, seen := g.index[c]; !seen {
			g.connect(c)
			g.low[f] = min(g.low[f], g.low[c])
		} else if g.onStack[c] {
			g.low[f] = min(g.low[f], g.index[c])
		}
	}
	if g.low[f] == g.index[f] {
		g.pop(f)
	}
}

func (g *callGraph) pop(root *Func) {
	var members []*Func
	for {
		top := g.stack[len(g.stack)-1]
		g.stack = g.stack[:len(g.stack)-1]
		g.onStack[top] = false
		members = append(members, top)
		if top == root {
			break
		}
	}
	if len(members) > 1 {
		for _, m := range members {
			g.recursive[m] = true
		}
	}
	g.order = append(g.order, members...)
}

func (in *inliner) summarise(g *Func) calleeSummary {
	if s, ok := in.summaries[g]; ok {
		return s
	}
	s := in.measure(g)
	in.summaries[g] = s
	return s
}

func (in *inliner) measure(g *Func) calleeSummary {
	if in.recursive[g] || g.Callback || len(g.Slots) > 0 || len(g.Blocks[0].Preds) > 0 {
		return calleeSummary{}
	}
	cost, loops := 0, 0
	for _, b := range g.Blocks {
		if b.Poll {
			loops++
		}
		for _, v := range b.Values {
			if !inlinableOp(v.Op) {
				return calleeSummary{}
			}
			if countsTowardCost(v.Op) {
				cost++
			}
		}
	}
	return calleeSummary{cost: cost, eligible: cost <= maxInlineCost && loops <= maxInlineLoops}
}

func inlinableOp(op Op) bool {
	switch op {
	case OpSlot, OpCCall, OpRuntime, OpVector, OpVectorScalar:
		return false
	}
	return true
}

func countsTowardCost(op Op) bool {
	switch op {
	case OpConst, OpParam, OpResult, OpUndef:
		return false
	}
	return true
}

func (in *inliner) inlineInto(f *Func) {
	sites := in.callSites(f)
	if len(sites) == 0 {
		return
	}
	grown := 0
	changed := false
	for _, call := range sites {
		callee := call.Target
		summary := in.summarise(callee)
		if !summary.eligible || len(call.Args) != callee.ParamWords || grown+summary.cost > maxCallerGrowth || in.grown+summary.cost > maxProgramGrowth {
			continue
		}
		newInlineSite(f, call, callee).expand()
		grown += summary.cost
		in.grown += summary.cost
		changed = true
	}
	if changed {
		simplify(f)
	}
}

func (in *inliner) callSites(f *Func) []*Value {
	var sites []*Value
	for _, b := range f.Blocks {
		for _, v := range b.Values {
			if v.Op == OpCall && !v.Indirect && v.Target != nil && v.Target != f && in.summarise(v.Target).eligible {
				sites = append(sites, v)
			}
		}
	}
	return sites
}
