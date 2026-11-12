package ir

import "slices"

func (s *ssa) threadBranches() bool {
	threaded := false
	counted := false
	for _, b := range s.fn.Blocks {
		phi, not, ok := phiBranch(b)
		if !ok {
			continue
		}
		if !counted {
			s.countUses()
			counted = true
		}
		if phi.Uses != 1 || (not != nil && not.Uses != 1) {
			continue
		}
		if threadPredecessors(b, phi, not != nil) {
			threaded = true
		}
	}
	return threaded
}

func phiBranch(b *Block) (*Value, *Value, bool) {
	if b.Kind != If || b.Succs[0] == b.Succs[1] || len(b.Values) == 0 || len(b.Values) > 2 {
		return nil, nil, false
	}
	phi := b.Values[0]
	if phi.Op != OpPhi || !slices.ContainsFunc(phi.Args, func(a *Value) bool { return a.Op == OpConst }) {
		return nil, nil, false
	}
	if len(b.Values) == 1 {
		return phi, nil, b.Control == phi
	}
	not := b.Values[1]
	return phi, not, not.Op == OpNot && not.Args[0] == phi && b.Control == not
}

func threadPredecessors(b *Block, phi *Value, negated bool) bool {
	threaded := false
	for i := len(b.Preds) - 1; i >= 0; i-- {
		arg := phi.Args[i]
		if arg.Op != OpConst {
			continue
		}
		taken := (arg.Aux != 0) != negated
		target := b.Succs[1]
		if taken {
			target = b.Succs[0]
		}
		if canThread(b, b.Preds[i], target) {
			redirect(b, i, target)
			threaded = true
		}
	}
	return threaded
}

func canThread(b, pred, target *Block) bool {
	if pred == b || target == b || slices.Contains(target.Preds, pred) {
		return false
	}
	return len(pred.Succs) == 1 || pred.Succs[0] != pred.Succs[1]
}

func redirect(b *Block, i int, target *Block) {
	pred := b.Preds[i]
	at := slices.Index(target.Preds, b)
	for _, v := range target.Values {
		if v.Op == OpPhi {
			v.Args = append(v.Args, v.Args[at])
		}
	}
	target.Preds = append(target.Preds, pred)
	pred.Succs[slices.Index(pred.Succs, b)] = target
	b.removePred(i)
}
