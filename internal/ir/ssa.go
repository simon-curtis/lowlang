package ir

import (
	"slices"

	"lowlang/internal/ast"
)

type varKey struct {
	local *ast.Local
	temp  int
	word  int
}

type ssa struct {
	fn *Func
	ws *workspace
}

type pendingPhi struct {
	key varKey
	phi *Value
}

func resolve(v *Value) *Value {
	for v.forward != nil {
		v = v.forward
	}
	return v
}

func (s *ssa) write(k varKey, b *Block, v *Value) {
	s.ws.variables[defKey{variable: k, block: b.ID}] = v
}

func (s *ssa) read(k varKey, b *Block) *Value {
	if v, ok := s.ws.variables[defKey{variable: k, block: b.ID}]; ok {
		return resolve(v)
	}
	return s.readRecursive(k, b)
}

func (s *ssa) readRecursive(k varKey, b *Block) *Value {
	var v *Value
	switch {
	case !s.ws.isSealed(b):
		v = s.phi(b)
		s.ws.addIncomplete(b, pendingPhi{key: k, phi: v})
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
	same, ok := soleOperand(phi)
	if !ok {
		return phi
	}
	if same == nil {
		same = s.prepend(phi.Block, OpUndef)
	}
	phi.forward = same
	return same
}

func soleOperand(phi *Value) (*Value, bool) {
	var same *Value
	for _, arg := range phi.Args {
		arg = resolve(arg)
		if arg == same || arg == phi {
			continue
		}
		if same != nil {
			return nil, false
		}
		same = arg
	}
	return same, true
}

func (s *ssa) seal(b *Block) {
	for i := 0; ; i++ {
		pending, ok := s.ws.takeIncomplete(b, i)
		if !ok {
			break
		}
		s.addOperands(pending.key, pending.phi)
	}
	s.ws.dropIncomplete(b)
	s.ws.markSealed(b)
}

func (s *ssa) finalize() {
	s.optimizeUntilStable()
	s.optimizeLoops()
	s.removeDeadValues()
	s.compactSlots()
	s.countUses()
	s.classifyPhis()
	s.classifyFloatPhis()
}

func (s *ssa) optimizeUntilStable() {
	for {
		s.prune()
		for s.removeTrivialPhis() {
		}
		promoted := s.promoteSlots()
		s.optimize()
		for s.removeTrivialPhis() {
		}
		folded := s.foldConstantBranches()
		threaded := s.threadBranches()
		merged := s.mergeBlocks()
		if !folded && !threaded && !merged && !promoted {
			return
		}
	}
}

func (s *ssa) classifyPhis() {
	var phis []*Value
	for _, b := range s.fn.Blocks {
		for _, v := range b.Values {
			if v.Op == OpPhi {
				v.Scalar = true
				phis = append(phis, v)
			}
			if v.Op == OpUndef || v.Op == OpSlot {
				v.Scalar = true
			}
		}
	}
	for changed := true; changed; {
		changed = false
		for _, phi := range phis {
			if phi.Scalar && slices.ContainsFunc(phi.Args, func(a *Value) bool { return !a.Scalar }) {
				phi.Scalar = false
				changed = true
			}
		}
	}
}

func (s *ssa) prune() {
	reachable := reset(s.ws.reach, s.fn.blockIDs)
	s.ws.reach = reachable
	var visit func(b *Block)
	visit = func(b *Block) {
		if reachable[b.ID] {
			return
		}
		reachable[b.ID] = true
		for _, succ := range b.Succs {
			visit(succ)
		}
	}
	visit(s.fn.Blocks[0])
	var kept []*Block
	for _, b := range s.fn.Blocks {
		if reachable[b.ID] {
			dropDeadPreds(b, reachable)
			kept = append(kept, b)
		}
	}
	s.fn.Blocks = kept
}

func dropDeadPreds(b *Block, reachable []bool) {
	var live []int
	for i, p := range b.Preds {
		if reachable[p.ID] {
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
			v.Uses = 0
		}
	}
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

type floatLattice []uint8

func (s *ssa) classifyFloatPhis() {
	var phis []*Value
	for _, b := range s.fn.Blocks {
		for _, v := range b.Values {
			if v.Op == OpPhi {
				phis = append(phis, v)
			}
		}
	}
	s.ws.lattice = reset(s.ws.lattice, s.fn.values+1)
	lattice := floatLattice(s.ws.lattice)
	for changed := true; changed; {
		changed = false
		for _, phi := range phis {
			if lattice.refine(phi) {
				changed = true
			}
		}
	}
	for _, phi := range phis {
		phi.FloatBits = lattice.bits(phi)
	}
}

func (l floatLattice) get(v *Value) (int, bool) {
	known := l[v.ID]
	return int(known) - 1, known != 0
}

func (l floatLattice) bits(v *Value) int {
	if bits, known := l.get(v); known {
		return bits
	}
	return 0
}

func (l floatLattice) refine(phi *Value) bool {
	bits, determined := l.join(phi)
	old, known := l.get(phi)
	if !determined || (known && old == bits) {
		return false
	}
	l[phi.ID] = uint8(bits + 1)
	return true
}

func (l floatLattice) join(phi *Value) (int, bool) {
	bits, determined := 0, false
	for _, a := range phi.Args {
		if a == phi || a.Op == OpUndef {
			continue
		}
		argBits, known := l.bitsOf(a)
		switch {
		case !known:
		case !determined:
			bits, determined = argBits, true
		case bits != argBits:
			return 0, true
		}
	}
	return bits, determined
}

func (l floatLattice) bitsOf(v *Value) (int, bool) {
	if v.Op != OpPhi {
		return v.FloatBits, true
	}
	return l.get(v)
}
