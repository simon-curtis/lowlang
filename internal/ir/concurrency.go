package ir

import (
	"lowlang/internal/ast"
	"lowlang/internal/prelude"
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

func (b *builder) chanField(c *types.Type, name string) *types.Field {
	instance := types.InstanceName(prelude.ChanNew, []*types.Type{c.Elem})
	return b.preludeField(instance, name)
}

func (b *builder) refAddress(arg ast.Expr) *Value {
	r := arg.(*ast.Ref)
	return b.address(b.placeOf(r.X))
}

func (b *builder) receive(e *ast.Unary) []*Value {
	c := b.word(e.X)
	return b.chanNamed(prelude.ChanRecv, e.X.TypeOf(), c)
}

func (b *builder) send(s *ast.Send) {
	t := s.Chan.TypeOf()
	args := []*Value{b.word(s.Chan)}
	args = append(args, b.value(s.Value, t.Elem)...)
	b.chanNamed(prelude.ChanSend, t, args...)
}

func (b *builder) chanLoop(s *ast.ForIn) {
	t := s.Iter.TypeOf()
	c := b.word(s.Iter)
	loop := b.openLoop()
	received := b.chanNamed(prelude.ChanRecv, t, c)
	b.branch(received[0], loop.body, loop.exit)
	b.enter(loop.body)
	b.bind(s.ValueLocal, received[1:1+t.Elem.Size()])
	b.loopBody(loop, s.Body)
	b.closeLoop(loop)
}

func (b *builder) selectExpr(e *ast.Select) []*Value {
	arms := make([]*selectArm, len(e.Cases))
	var chans []*selectArm
	elseIndex := -1
	for i, sc := range e.Cases {
		a := &selectArm{sc: sc}
		arms[i] = a
		switch sc.Kind {
		case ast.RecvCase:
			a.chant = sc.Chan.TypeOf()
			a.ch = b.word(sc.Chan)
			a.box = b.slot(max(1, a.chant.Elem.Size()))
			chans = append(chans, a)
		case ast.SendCase:
			a.chant = sc.Chan.TypeOf()
			a.ch = b.word(sc.Chan)
			a.val = b.value(sc.Value, a.chant.Elem)
			chans = append(chans, a)
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
	b.callNamed(prelude.Wake, b.readTemp(outcome, 3)[2])
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

func (b *builder) lockChannels(chans []*selectArm) *Value {
	if len(chans) == 0 {
		return nil
	}
	locks := b.slot(len(chans))
	for i, a := range chans {
		lock := b.chanField(a.chant, prelude.ChanLock)
		word := a.ch
		if lock.Bytes != 0 {
			word = b.emit(OpAdd, a.ch, b.constant(int64(lock.Bytes)))
		}
		b.store(place{addr: locks, offset: 8 * i}, []*Value{word})
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
		probe = b.chanNamed(prelude.SelRecv, a.chant, a.ch, a.box)
	} else {
		args := append([]*Value{a.ch}, a.val...)
		probe = b.chanNamed(prelude.SelSend, a.chant, args...)
	}
	b.branch(b.emit(OpLt, probe[0], b.constant(0)), skip, hit)
	b.enter(hit)
	ok := b.emit(OpEq, probe[0], b.constant(1))
	b.writeTemp(outcome, []*Value{b.constant(int64(i)), ok, probe[1]})
	b.jump(skip)
	b.enter(skip)
}

func (b *builder) waitSelect(arms []*selectArm, locks *Value, n int, outcome int) {
	w := b.callNamed(prelude.Waiter)[0]
	for i, a := range arms {
		index := b.constant(int64(i))
		switch a.sc.Kind {
		case ast.RecvCase:
			b.chanNamed(prelude.SelWaitRecv, a.chant, a.ch, w, index, a.box)
		case ast.SendCase:
			a.box = b.slot(max(1, len(a.val)))
			b.store(place{addr: a.box}, a.val)
			b.chanNamed(prelude.SelWaitSend, a.chant, a.ch, w, index, a.box)
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
	indexField := b.preludeField(prelude.Waiter, prelude.WaiterIndex)
	okField := b.preludeField(prelude.Waiter, prelude.WaiterOK)
	index := b.load(place{addr: w, offset: indexField.Bytes}, indexField.Type)[0]
	ok := b.load(place{addr: w, offset: okField.Bytes}, okField.Type)[0]
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
			b.bind(a.sc.Local, b.chanNamed(prelude.SelValue, a.chant, ok, a.box))
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
