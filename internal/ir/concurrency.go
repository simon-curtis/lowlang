package ir

import (
	"lowlang/internal/ast"
	"lowlang/internal/types"
)

type selectArm struct {
	sc    *ast.SelectCase
	chant *types.Type
	ch    *Value
	val   []*Value
	box   *Value
	ms    *Value
	timer *Value
}

func (b *builder) chanNamed(name string, c *types.Type, args ...*Value) []*Value {
	instance := types.InstanceName(name, []*types.Type{c.Elem})
	return b.callNamed(instance, args...)
}

func (b *builder) refAddress(arg ast.Expr) *Value {
	r := arg.(*ast.Ref)
	return b.address(b.placeOf(r.X))
}

func (b *builder) receive(e *ast.Unary) []*Value {
	c := b.word(e.X)
	return b.chanNamed("__chan_recv", e.X.TypeOf(), c)
}

func (b *builder) send(s *ast.Send) {
	t := s.Chan.TypeOf()
	args := []*Value{b.word(s.Chan)}
	args = append(args, b.value(s.Value, t.Elem)...)
	b.chanNamed("__chan_send", t, args...)
}

func (b *builder) chanLoop(s *ast.ForIn) {
	t := s.Iter.TypeOf()
	c := b.word(s.Iter)
	header := b.newBlock()
	body := b.newBlock()
	next := b.newBlock()
	exit := b.newBlock()
	b.jump(header)
	b.cur = header
	received := b.chanNamed("__chan_recv", t, c)
	b.branch(received[0], body, exit)
	b.enter(body)
	b.bind(s.ValueLocal, received[1:1+t.Elem.Size()])
	b.loopBody(s.Body, next, exit)
	b.enter(next)
	b.backEdge(header)
	b.seal(header)
	b.enter(exit)
}

func (b *builder) selectExpr(e *ast.Select) []*Value {
	arms := make([]*selectArm, len(e.Cases))
	var chans []*Value
	elseIndex := -1
	for i, sc := range e.Cases {
		a := &selectArm{sc: sc}
		arms[i] = a
		switch sc.Kind {
		case ast.RecvCase:
			a.chant = sc.Chan.TypeOf()
			a.ch = b.word(sc.Chan)
			a.box = b.slot(max(1, a.chant.Elem.Size()))
			chans = append(chans, a.ch)
		case ast.SendCase:
			a.chant = sc.Chan.TypeOf()
			a.ch = b.word(sc.Chan)
			a.val = b.value(sc.Value, a.chant.Elem)
			chans = append(chans, a.ch)
		case ast.AfterCase:
			a.ms = b.word(sc.Value)
		case ast.ElseCase:
			elseIndex = i
		}
	}
	locks := b.lockChannels(chans)
	outcome := b.newTemp()
	b.writeTemp(outcome, []*Value{b.constant(-1), b.constant(0), b.constant(0)})
	for i, a := range arms {
		if a.ch != nil {
			b.trySelect(i, a, outcome)
		}
	}
	fired := b.newBlock()
	waiting := b.newBlock()
	finish := b.newBlock()
	pending := b.emit(OpLt, b.readTemp(outcome, 1)[0], b.constant(0))
	b.branch(pending, waiting, fired)
	b.enter(fired)
	b.unlockChannels(locks, len(chans))
	b.callNamed("__wake", b.readTemp(outcome, 3)[2])
	b.jump(finish)
	b.enter(waiting)
	if elseIndex >= 0 {
		b.unlockChannels(locks, len(chans))
		b.writeTemp(outcome, []*Value{b.constant(int64(elseIndex))})
	} else {
		b.waitSelect(arms, locks, len(chans), outcome)
	}
	b.jump(finish)
	b.enter(finish)
	result := b.readTemp(outcome, 2)
	return b.selectArms(e, arms, result[0], result[1])
}

func (b *builder) lockChannels(chans []*Value) *Value {
	if len(chans) == 0 {
		return nil
	}
	locks := b.slot(len(chans))
	for i, ch := range chans {
		b.store(place{addr: locks, offset: 8 * i}, []*Value{ch})
	}
	b.runtime(rtLockAll, locks, b.constant(int64(len(chans))))
	return locks
}

func (b *builder) unlockChannels(locks *Value, n int) {
	if locks != nil {
		b.runtime(rtUnlockAll, locks, b.constant(int64(n)))
	}
}

func (b *builder) trySelect(i int, a *selectArm, outcome int) {
	try := b.newBlock()
	hit := b.newBlock()
	skip := b.newBlock()
	b.branch(b.emit(OpLt, b.readTemp(outcome, 1)[0], b.constant(0)), try, skip)
	b.enter(try)
	var probe []*Value
	if a.sc.Kind == ast.RecvCase {
		probe = b.chanNamed("__sel_recv", a.chant, a.ch, a.box)
	} else {
		args := append([]*Value{a.ch}, a.val...)
		probe = b.chanNamed("__sel_send", a.chant, args...)
	}
	b.branch(b.emit(OpLt, probe[0], b.constant(0)), skip, hit)
	b.enter(hit)
	ok := b.emit(OpEq, probe[0], b.constant(1))
	b.writeTemp(outcome, []*Value{b.constant(int64(i)), ok, probe[1]})
	b.jump(skip)
	b.enter(skip)
}

func (b *builder) waitSelect(arms []*selectArm, locks *Value, n int, outcome int) {
	w := b.callNamed("__waiter")[0]
	for i, a := range arms {
		index := b.constant(int64(i))
		switch a.sc.Kind {
		case ast.RecvCase:
			b.chanNamed("__sel_wait_recv", a.chant, a.ch, w, index, a.box)
		case ast.SendCase:
			a.box = b.slot(max(1, len(a.val)))
			b.store(place{addr: a.box}, a.val)
			b.chanNamed("__sel_wait_send", a.chant, a.ch, w, index, a.box)
		}
	}
	b.unlockChannels(locks, n)
	for i, a := range arms {
		if a.ms != nil {
			a.timer = b.runtime(rtTimerAdd, a.ms, w, b.constant(int64(i)))
		}
	}
	b.runtime(rtPark)
	for _, a := range arms {
		if a.timer != nil {
			b.runtime(rtTimerDel, a.timer)
		}
		if a.sc.Kind == ast.SendCase {
			b.clearSlot(a.box, len(a.val))
		}
	}
	index := b.load(place{addr: w, offset: 8}, types.I64)[0]
	ok := b.load(place{addr: w, offset: 16}, types.Bool)[0]
	b.writeTemp(outcome, []*Value{index, ok})
}

func (b *builder) selectArms(e *ast.Select, arms []*selectArm, index, ok *Value) []*Value {
	temp := b.newTemp()
	done := b.newBlock()
	for i, a := range arms {
		body := b.newBlock()
		next := b.newBlock()
		match := b.emit(OpEq, index, b.constant(int64(i)))
		b.branch(match, body, next)
		b.enter(body)
		if a.sc.Local != nil {
			b.bind(a.sc.Local, b.chanNamed("__sel_value", a.chant, ok, a.box))
		}
		if a.sc.Kind == ast.RecvCase {
			b.clearSlot(a.box, a.chant.Elem.Size())
		}
		b.writeTemp(temp, b.value(a.sc.Body, e.Type))
		b.jump(done)
		b.enter(next)
	}
	b.cur.Kind = Unreachable
	b.enter(done)
	return b.readTemp(temp, e.Type.Size())
}

func (b *builder) clearSlot(slot *Value, words int) {
	zeros := make([]*Value, max(1, words))
	for i := range zeros {
		zeros[i] = b.constant(0)
	}
	b.store(place{addr: slot}, zeros)
}
