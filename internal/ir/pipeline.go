package ir

func (p *Program) optimise(opts Options) {
	if !opts.Inline {
		return
	}
	p.specialiseConstantArguments()
	p.inline()
	p.Funcs = reachable(p.Main, p.Funcs)
}
