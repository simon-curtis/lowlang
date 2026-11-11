package ir

import "slices"

func (s *ssa) threadBranches() bool {
	var uses []int32
	threaded := false
	for _, b := range s.fn.Blocks {
		if !branchesOnPhiOfConstants(b) {
			continue
		}
		if uses == nil {
			uses = s.useCounts()
		}
		phi, negated, ok := branchOnPhi(b, uses)
		if ok && threadPredecessors(b, phi, negated) {
			threaded = true
		}
	}
	return threaded
}

func branchesOnPhiOfConstants(b *Block) bool {
	if b.Kind != If || len(b.Values) == 0 || len(b.Values) > 2 || b.Values[0].Op != OpPhi {
		return false
	}
	return slices.ContainsFunc(b.Values[0].Args, func(a *Value) bool { return a.Op == OpConst })
}

func (s *ssa) useCounts() []int32 {
	s.ws.uses = reset(s.ws.uses, s.fn.values+1)
	uses := s.ws.uses
	for _, b := range s.fn.Blocks {
		for _, v := range b.Values {
			for _, a := range v.Args {
				uses[a.ID]++
			}
		}
		if b.Control != nil {
			uses[b.Control.ID]++
		}
		for _, r := range b.Results {
			uses[r.ID]++
		}
	}
	return uses
}

func branchOnPhi(b *Block, uses []int32) (*Value, bool, bool) {
	if b.Kind != If || b.Succs[0] == b.Succs[1] || len(b.Values) == 0 || len(b.Values) > 2 {
		return nil, false, false
	}
	phi := b.Values[0]
	if phi.Op != OpPhi || uses[phi.ID] != 1 {
		return nil, false, false
	}
	if len(b.Values) == 1 {
		return phi, false, b.Control == phi
	}
	not := b.Values[1]
	ok := not.Op == OpNot && not.Args[0] == phi && b.Control == not && uses[not.ID] == 1
	return phi, true, ok
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
	b.Preds = slices.Delete(b.Preds, i, i+1)
	for _, v := range b.Values {
		if v.Op == OpPhi {
			v.Args = slices.Delete(v.Args, i, i+1)
		}
	}
}
