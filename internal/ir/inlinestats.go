package ir

type CallSiteReport struct {
	Direct   int
	Indirect int
	Reasons  map[string]int
	Cyclic   []string
}

func (p *Program) CallSites() CallSiteReport {
	in := &inliner{program: p, summaries: map[*Func]calleeSummary{}}
	in.callOrder()
	report := CallSiteReport{Reasons: map[string]int{}}
	for _, f := range p.Funcs {
		for _, b := range f.Blocks {
			for _, v := range b.Values {
				switch {
				case v.Op != OpCall:
				case v.Indirect || v.Target == nil:
					report.Indirect++
				default:
					report.Direct++
					in.classify(v.Target, report.Reasons)
				}
			}
		}
	}
	for _, f := range p.Funcs {
		if in.recursive[f] {
			report.Cyclic = append(report.Cyclic, f.Name)
		}
	}
	return report
}

func (in *inliner) classify(g *Func, reasons map[string]int) {
	found := false
	note := func(reason string) {
		reasons[reason]++
		found = true
	}
	if in.recursive[g] {
		note("recursive")
	}
	if g.Callback {
		note("callback")
	}
	if len(g.Blocks[0].Preds) > 0 {
		note("entry-loop")
	}
	cost, loops := 0, 0
	kinds := map[Op]bool{}
	for _, b := range g.Blocks {
		if b.Poll {
			loops++
		}
		for _, v := range b.Values {
			kinds[v.Op] = true
			if countsTowardCost(v.Op) {
				cost++
			}
		}
	}
	if kinds[OpVector] || kinds[OpVectorScalar] {
		note("vector")
	}
	if cost > maxInlineCost {
		note("cost")
	}
	if loops > maxInlineLoops {
		note("loops")
	}
	if !found {
		note("budget")
	}
}
