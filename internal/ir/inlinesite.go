package ir

import "slices"

type inlineScratch struct {
	values []*Value
	blocks []*Block
}

func (c *inlineScratch) reset(callee *Func) {
	c.values = slices.Grow(c.values[:0], callee.values+1)[:callee.values+1]
	clear(c.values)
	c.blocks = slices.Grow(c.blocks[:0], callee.blockIDs)[:callee.blockIDs]
	clear(c.blocks)
}

type inlineSite struct {
	caller   *Func
	call     *Value
	callee   *Func
	head     *Block
	tail     *Block
	values   []*Value
	blocks   []*Block
	slotBase int
	returns  []*Block
}

func newInlineSite(caller *Func, call *Value, callee *Func, scratch *inlineScratch) *inlineSite {
	scratch.reset(callee)
	return &inlineSite{
		caller: caller,
		call:   call,
		callee: callee,
		head:   call.Block,
		values: scratch.values,
		blocks: scratch.blocks,
	}
}

func (s *inlineSite) expand(results []*Value) {
	s.slotBase = len(s.caller.Slots)
	s.caller.Slots = append(s.caller.Slots, s.callee.Slots...)
	s.splitHead()
	s.cloneCallee()
	s.connect()
	s.bindResults(results)
}

func (s *inlineSite) splitHead() {
	head := s.head
	position := slices.Index(head.Values, s.call)
	s.tail = s.caller.newBlock()
	tail := s.tail
	tail.Kind = head.Kind
	tail.Control = head.Control
	tail.Table = head.Table
	tail.Results = head.Results
	tail.Poll = head.Poll
	tail.Succs = head.Succs
	tail.Values = slices.Clone(head.Values[position+1:])
	for _, v := range tail.Values {
		v.Block = tail
	}
	for _, succ := range tail.Succs {
		for i, p := range succ.Preds {
			if p == head {
				succ.Preds[i] = tail
			}
		}
	}
	head.Values = head.Values[:position]
	head.Kind = Plain
	head.Control = nil
	head.Table = nil
	head.Results = nil
	head.Poll = false
	head.Succs = nil
}

func (s *inlineSite) cloneCallee() {
	for _, b := range s.callee.Blocks {
		s.blocks[b.ID] = s.caller.newBlock()
	}
	for _, b := range s.callee.Blocks {
		s.cloneValues(b)
	}
	for _, b := range s.callee.Blocks {
		s.cloneBlock(b)
	}
}

func (s *inlineSite) cloneValues(b *Block) {
	clone := s.blocks[b.ID]
	for _, v := range b.Values {
		if v.Op == OpParam {
			s.values[v.ID] = s.call.Args[v.Aux]
			continue
		}
		c := s.caller.newValue(clone, v.Op)
		c.Aux = v.Aux
		if v.Op == OpSlot {
			c.Aux += int64(s.slotBase)
		}
		c.Str = v.Str
		c.Runtime = v.Runtime
		c.Target = v.Target
		c.Sig = v.Sig
		c.Type = v.Type
		c.From = v.From
		c.Scalar = v.Scalar
		c.FloatBits = v.FloatBits
		c.Indirect = v.Indirect
		c.Mem = v.Mem
		s.values[v.ID] = c
		clone.Values = append(clone.Values, c)
	}
}

func (s *inlineSite) mapValue(v *Value) *Value {
	return s.values[resolve(v).ID]
}

func (s *inlineSite) cloneBlock(b *Block) {
	clone := s.blocks[b.ID]
	for _, v := range b.Values {
		if v.Op == OpParam {
			continue
		}
		c := s.values[v.ID]
		c.Args = make([]*Value, len(v.Args))
		for i, a := range v.Args {
			c.Args[i] = s.mapValue(a)
		}
	}
	clone.Kind = b.Kind
	clone.Table = b.Table
	clone.Poll = b.Poll
	if b.Control != nil {
		clone.Control = s.mapValue(b.Control)
	}
	for _, p := range b.Preds {
		clone.Preds = append(clone.Preds, s.blocks[p.ID])
	}
	for _, succ := range b.Succs {
		clone.Succs = append(clone.Succs, s.blocks[succ.ID])
	}
	if b.Kind == Ret {
		s.makeReturn(b, clone)
	}
}

func (s *inlineSite) makeReturn(b, clone *Block) {
	clone.Kind = Plain
	clone.Succs = []*Block{s.tail}
	for _, r := range b.Results {
		clone.Results = append(clone.Results, s.mapValue(r))
	}
	s.returns = append(s.returns, clone)
}

func (s *inlineSite) connect() {
	entry := s.blocks[s.callee.Blocks[0].ID]
	entry.Preds = []*Block{s.head}
	s.head.Succs = []*Block{entry}
	s.tail.Preds = slices.Clone(s.returns)
}

func (s *inlineSite) bindResults(results []*Value) {
	for k, r := range results {
		if r != nil {
			forwardTo(r, s.returnedWord(k))
		}
	}
	for _, b := range s.returns {
		b.Results = nil
	}
}

func (s *inlineSite) returnedWord(k int) *Value {
	if len(s.returns) == 1 {
		return s.returns[0].Results[k]
	}
	phi := s.caller.newValue(s.tail, OpPhi)
	for _, b := range s.returns {
		phi.Args = append(phi.Args, b.Results[k])
	}
	s.tail.Values = append([]*Value{phi}, s.tail.Values...)
	return phi
}
