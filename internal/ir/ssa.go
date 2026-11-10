package ir

import "lowlang/internal/ast"

type varKey struct {
	local *ast.Local
	temp  int
	word  int
}

type ssa struct {
	fn         *Func
	defs       map[varKey]map[*Block]*Value
	sealed     map[*Block]bool
	incomplete map[*Block]map[varKey]*Value
}

func newSSA(fn *Func) ssa {
	return ssa{
		fn:         fn,
		defs:       map[varKey]map[*Block]*Value{},
		sealed:     map[*Block]bool{},
		incomplete: map[*Block]map[varKey]*Value{},
	}
}

func resolve(v *Value) *Value {
	for v.forward != nil {
		v = v.forward
	}
	return v
}

func (s *ssa) write(k varKey, b *Block, v *Value) {
	defs := s.defs[k]
	if defs == nil {
		defs = map[*Block]*Value{}
		s.defs[k] = defs
	}
	defs[b] = v
}

func (s *ssa) read(k varKey, b *Block) *Value {
	if v, ok := s.defs[k][b]; ok {
		return resolve(v)
	}
	return s.readRecursive(k, b)
}

func (s *ssa) readRecursive(k varKey, b *Block) *Value {
	var v *Value
	switch {
	case !s.sealed[b]:
		v = s.phi(b)
		if s.incomplete[b] == nil {
			s.incomplete[b] = map[varKey]*Value{}
		}
		s.incomplete[b][k] = v
	case len(b.Preds) == 0:
		v = s.prepend(b, OpUndef)
	case len(b.Preds) == 1:
		v = s.read(k, b.Preds[0])
	default:
		phi := s.phi(b)
		s.write(k, b, phi)
		v = s.addOperands(k, phi)
	}
	s.write(k, b, v)
	return v
}

func (s *ssa) phi(b *Block) *Value {
	return s.prepend(b, OpPhi)
}

func (s *ssa) prepend(b *Block, op Op) *Value {
	v := s.fn.newValue(b, op)
	b.Values = append([]*Value{v}, b.Values...)
	return v
}

func (s *ssa) addOperands(k varKey, phi *Value) *Value {
	for _, pred := range phi.Block.Preds {
		phi.Args = append(phi.Args, s.read(k, pred))
	}
	return s.trivial(phi)
}

func (s *ssa) trivial(phi *Value) *Value {
	var same *Value
	for _, arg := range phi.Args {
		arg = resolve(arg)
		if arg == same || arg == phi {
			continue
		}
		if same != nil {
			return phi
		}
		same = arg
	}
	if same == nil {
		same = s.prepend(phi.Block, OpUndef)
	}
	phi.forward = same
	return same
}

func (s *ssa) seal(b *Block) {
	for k, phi := range s.incomplete[b] {
		s.addOperands(k, phi)
	}
	delete(s.incomplete, b)
	s.sealed[b] = true
}

func (s *ssa) finalize() {
	s.prune()
	for s.removeTrivialPhis() {
	}
	s.countUses()
}

func (s *ssa) prune() {
	reachable := map[*Block]bool{}
	var visit func(b *Block)
	visit = func(b *Block) {
		if reachable[b] {
			return
		}
		reachable[b] = true
		for _, succ := range b.Succs {
			visit(succ)
		}
	}
	visit(s.fn.Blocks[0])
	var kept []*Block
	for _, b := range s.fn.Blocks {
		if reachable[b] {
			dropDeadPreds(b, reachable)
			kept = append(kept, b)
		}
	}
	s.fn.Blocks = kept
}

func dropDeadPreds(b *Block, reachable map[*Block]bool) {
	var live []int
	for i, p := range b.Preds {
		if reachable[p] {
			live = append(live, i)
		}
	}
	if len(live) == len(b.Preds) {
		return
	}
	preds := make([]*Block, len(live))
	for j, i := range live {
		preds[j] = b.Preds[i]
	}
	b.Preds = preds
	for _, v := range b.Values {
		if v.Op != OpPhi || v.forward != nil {
			continue
		}
		args := make([]*Value, len(live))
		for j, i := range live {
			args[j] = v.Args[i]
		}
		v.Args = args
	}
}

func (s *ssa) removeTrivialPhis() bool {
	changed := false
	for _, b := range s.fn.Blocks {
		for _, v := range b.Values {
			if v.Op == OpPhi && v.forward == nil && s.trivial(v) != v {
				changed = true
			}
		}
	}
	for _, b := range s.fn.Blocks {
		s.resolveBlock(b)
	}
	return changed
}

func (s *ssa) resolveBlock(b *Block) {
	var kept []*Value
	for _, v := range b.Values {
		if v.forward != nil {
			continue
		}
		for i, arg := range v.Args {
			v.Args[i] = resolve(arg)
		}
		kept = append(kept, v)
	}
	b.Values = kept
	if b.Control != nil {
		b.Control = resolve(b.Control)
	}
	for i, r := range b.Results {
		b.Results[i] = resolve(r)
	}
}

func (s *ssa) countUses() {
	for _, b := range s.fn.Blocks {
		for _, v := range b.Values {
			for _, arg := range v.Args {
				arg.Uses++
			}
		}
		if b.Control != nil {
			b.Control.Uses++
		}
		for _, r := range b.Results {
			r.Uses++
		}
	}
}
