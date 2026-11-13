package ir

import "slices"

const (
	maxInlineCost    = 48
	maxInlineLoops   = 1
	freeInlineCost   = 6
	maxCallerGrowth  = 240
	minProgramGrowth = 6000
	maxProgramGrowth = 40000
	programShare     = 5
	loopReserveShare = 4
	maxInlineRounds  = 4
)

type calleeSummary struct {
	cost     int
	eligible bool
}

type inliner struct {
	program   *Program
	recursive map[*Func]bool
	summaries map[*Func]calleeSummary
	budget    int
	reserve   int
	scratch   inlineScratch
}

func (p *Program) inline() {
	budget := programBudget(p)
	in := &inliner{program: p, summaries: map[*Func]calleeSummary{}, budget: budget, reserve: budget / loopReserveShare}
	order := in.callOrder()
	for _, f := range order {
		in.inlineInto(f)
	}
}

func programBudget(p *Program) int {
	total := 0
	for _, f := range p.Funcs {
		total += costOf(f)
	}
	return min(maxProgramGrowth, max(minProgramGrowth, total/programShare))
}

func costOf(f *Func) int {
	cost := 0
	for _, b := range f.Blocks {
		for _, v := range b.Values {
			if countsTowardCost(v.Op) {
				cost++
			}
		}
	}
	return cost
}

func directCallees(f *Func) []*Func {
	var callees []*Func
	for v := range f.directTargets() {
		if v.Op == OpCall && !slices.Contains(callees, v.Target) {
			callees = append(callees, v.Target)
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
}

func (in *inliner) callOrder() []*Func {
	g := &callGraph{
		index:     map[*Func]int{},
		low:       map[*Func]int{},
		onStack:   map[*Func]bool{},
		recursive: map[*Func]bool{},
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
	if in.recursive[g] || g.Callback || len(g.Blocks[0].Preds) > 0 {
		return calleeSummary{}
	}
	cost, loops := 0, 0
	for _, b := range g.Blocks {
		if b.Poll {
			loops++
		}
		for _, v := range b.Values {
			if countsTowardCost(v.Op) {
				cost++
			}
		}
	}
	return calleeSummary{cost: cost, eligible: cost <= maxInlineCost && loops <= maxInlineLoops}
}

func countsTowardCost(op Op) bool {
	switch op {
	case OpConst, OpParam, OpResult, OpUndef:
		return false
	}
	return true
}

type inlineCandidate struct {
	call  *Value
	depth int
	cost  int
}

func (in *inliner) inlineInto(f *Func) {
	grown := 0
	for round := 0; round < maxInlineRounds; round++ {
		if !in.expandRound(f, &grown) {
			return
		}
		simplify(f)
	}
}

func (in *inliner) expandRound(f *Func, grown *int) bool {
	candidates := in.candidates(f)
	if len(candidates) == 0 {
		return false
	}
	results := resultsOf(f, candidates)
	expanded := false
	for _, c := range candidates {
		free := c.cost <= freeInlineCost
		if !free && !in.admits(c, *grown) {
			continue
		}
		newInlineSite(f, c.call, c.call.Target, &in.scratch).expand(results[c.call])
		expanded = true
		if !free {
			*grown += c.cost
			in.budget -= c.cost
		}
	}
	return expanded
}

func (in *inliner) admits(c inlineCandidate, grown int) bool {
	available := in.budget
	if c.depth == 0 {
		available -= in.reserve
	}
	return grown+c.cost <= maxCallerGrowth && c.cost <= available
}

func (in *inliner) candidates(f *Func) []inlineCandidate {
	var candidates []inlineCandidate
	var depths []int
	for _, b := range f.Blocks {
		for _, v := range b.Values {
			if v.Op != OpCall || v.Target == nil || v.Target == f {
				continue
			}
			summary := in.summarise(v.Target)
			if !summary.eligible || len(v.Args) != v.Target.ParamWords {
				continue
			}
			if depths == nil {
				depths = loopDepths(f)
			}
			candidates = append(candidates, inlineCandidate{call: v, depth: depths[b.ID], cost: summary.cost})
		}
	}
	slices.SortFunc(candidates, func(a, b inlineCandidate) int {
		switch {
		case a.depth != b.depth:
			return b.depth - a.depth
		case a.cost != b.cost:
			return a.cost - b.cost
		}
		return a.call.ID - b.call.ID
	})
	return candidates
}

func loopDepths(f *Func) []int {
	depths := make([]int, f.blockIDs)
	if !slices.ContainsFunc(f.Blocks, func(b *Block) bool { return b.Poll }) {
		return depths
	}
	for _, l := range findLoops(newDominators(f)) {
		for _, b := range l.order {
			depths[b.ID]++
		}
	}
	return depths
}

func resultsOf(f *Func, candidates []inlineCandidate) map[*Value][]*Value {
	results := make(map[*Value][]*Value, len(candidates))
	for _, c := range candidates {
		results[c.call] = make([]*Value, c.call.Target.ResultWords())
	}
	for _, b := range f.Blocks {
		for _, v := range b.Values {
			if v.Op != OpResult {
				continue
			}
			if words, ok := results[v.Args[0]]; ok {
				words[v.Aux] = v
			}
		}
	}
	return results
}
