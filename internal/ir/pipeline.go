package ir

import "slices"

func (p *Program) optimise() {
	p.specialiseConstantArguments()
	p.inline()
	reachable := p.reachableFromMain()
	p.keepReachable(reachable)
}

func (p *Program) reachableFromMain() map[*Func]bool {
	seen := map[*Func]bool{}
	var visit func(f *Func)
	visit = func(f *Func) {
		if seen[f] {
			return
		}
		seen[f] = true
		for v := range f.directTargets() {
			visit(v.Target)
		}
	}
	visit(p.Main)
	return seen
}

func (p *Program) keepReachable(reachable map[*Func]bool) {
	p.Funcs = slices.DeleteFunc(p.Funcs, func(f *Func) bool { return !reachable[f] })
}
