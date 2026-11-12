package ir

func (p *Program) splitCriticalEdges() {
	for _, f := range p.Funcs {
		f.splitCriticalEdges()
	}
}

func (f *Func) splitCriticalEdges() {
	for _, b := range f.Blocks {
		if len(b.Preds) < 2 || !hasPhi(b) {
			continue
		}
		for j, pred := range b.Preds {
			if len(pred.Succs) < 2 {
				continue
			}
			mid := f.newBlock()
			mid.Kind = Plain
			mid.Preds = []*Block{pred}
			mid.Succs = []*Block{b}
			for i, s := range pred.Succs {
				if s == b {
					pred.Succs[i] = mid
				}
			}
			b.Preds[j] = mid
		}
	}
}

func hasPhi(b *Block) bool {
	return len(b.Values) > 0 && b.Values[0].Op == OpPhi
}
