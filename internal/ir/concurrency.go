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

func (b *builder) chanCall(name string, c *types.Type, args ...*Value) []*Value {
	instance := types.InstanceName(name, []*types.Type{c.Elem})
	return b.callNamed(instance, args...)
}

func (b *builder) concurrencyBuiltin(e *ast.Call) ([]*Value, bool) {
	switch e.Builtin {
	case "chan":
		capacity := b.constant(0)
		if len(e.Args) == 1 {
			capacity = b.word(e.Args[0])
		}
		return b.chanCall("__chan_new", e.Type, capacity), true
	case "close":
		c := b.word(e.Args[0])
		b.chanCall("__chan_close", e.Args[0].TypeOf(), c)
	case "sleep":
		b.runtimeVoid("sleep", b.word(e.Args[0]))
	case "wait_readable":
		b.runtimeVoid("wait_fd", b.word(e.Args[0]), b.constant(1))
	case "wait_writable":
		b.runtimeVoid("wait_fd", b.word(e.Args[0]), b.constant(4))
	case "__spawn":
		f := b.value(e.Args[0], e.Args[0].TypeOf())
		b.runtimeVoid("spawn", f...)
	case "__current":
		g := b.runtime("current")
		g.Scalar = true
		return []*Value{g}, true
	case "__park":
		b.runtimeVoid("park")
	case "__context":
		return []*Value{b.runtime("context")}, true
	case "__set_context":
		b.runtimeVoid("set_context", b.word(e.Args[0]))
	case "__has_context":
		has := b.runtime("has_context")
		has.Scalar = true
		return []*Value{has}, true
	case "__ready":
		b.runtimeVoid("ready", b.word(e.Args[0]))
	case "__lock", "__unlock":
		b.runtimeVoid(e.Builtin[2:], b.refAddress(e.Args[0]))
	case "__cas":
		addr := b.refAddress(e.Args[0])
		old := b.word(e.Args[1])
		swapped := b.runtime("cas", addr, old, b.word(e.Args[2]))
		swapped.Scalar = true
		return []*Value{swapped}, true
	case "__panic":
		msg := b.value(e.Args[0], types.Str)
		b.runtimeVoid("panic_msg", msg...)
		b.cur.Kind = Unreachable
		b.dead()
	default:
		return nil, false
	}
	return nil, true
}

func (b *builder) refAddress(arg ast.Expr) *Value {
	r := arg.(*ast.Ref)
	return b.address(b.place(r.Place))
}

func (b *builder) receive(e *ast.Unary) []*Value {
	c := b.word(e.X)
	return b.chanCall("__chan_recv", e.X.TypeOf(), c)
}

func (b *builder) send(s *ast.Send) {
	t := s.Chan.TypeOf()
	args := []*Value{b.word(s.Chan)}
	args = append(args, b.value(s.Value, t.Elem)...)
	b.chanCall("__chan_send", t, args...)
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
	received := b.chanCall("__chan_recv", t, c)
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
	w := b.callNamed("__waiter")[0]
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
			a.box = b.chanCall("__box", a.chant)[0]
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
	done := b.newTemp()
	b.writeTemp(done, []*Value{b.constant(0)})
	for i, a := range arms {
		if a.ch != nil {
			b.trySelect(i, a, w, done)
		}
	}
	fired := b.newBlock()
	waiting := b.newBlock()
	finish := b.newBlock()
	b.branch(b.readTemp(done, 1)[0], fired, waiting)
	b.enter(fired)
	b.unlockChannels(locks, len(chans))
	b.callNamed("__sel_wake", w)
	b.jump(finish)
	b.enter(waiting)
	if elseIndex >= 0 {
		b.unlockChannels(locks, len(chans))
		b.store(place{addr: w, offset: 8}, []*Value{b.constant(int64(elseIndex))})
	} else {
		b.waitSelect(arms, w, locks, len(chans))
	}
	b.jump(finish)
	b.enter(finish)
	index := b.load(place{addr: w, offset: 8}, types.I64)[0]
	return b.selectArms(e, arms, w, index)
}

func (b *builder) lockChannels(chans []*Value) *Value {
	if len(chans) == 0 {
		return nil
	}
	locks := b.slot(len(chans))
	for i, ch := range chans {
		b.store(place{addr: locks, offset: 8 * i}, []*Value{ch})
	}
	b.runtimeVoid("lock_all", locks, b.constant(int64(len(chans))))
	return locks
}

func (b *builder) unlockChannels(locks *Value, n int) {
	if locks != nil {
		b.runtimeVoid("unlock_all", locks, b.constant(int64(n)))
	}
}

func (b *builder) trySelect(i int, a *selectArm, w *Value, done int) {
	try := b.newBlock()
	skip := b.newBlock()
	b.branch(b.readTemp(done, 1)[0], skip, try)
	b.enter(try)
	index := b.constant(int64(i))
	var result []*Value
	if a.sc.Kind == ast.RecvCase {
		result = b.chanCall("__sel_recv", a.chant, a.ch, w, index, a.box)
	} else {
		args := append([]*Value{a.ch, w, index}, a.val...)
		result = b.chanCall("__sel_send", a.chant, args...)
	}
	b.writeTemp(done, result)
	b.jump(skip)
	b.enter(skip)
}

func (b *builder) waitSelect(arms []*selectArm, w, locks *Value, n int) {
	for i, a := range arms {
		index := b.constant(int64(i))
		switch a.sc.Kind {
		case ast.RecvCase:
			b.chanCall("__sel_wait_recv", a.chant, a.ch, w, index, a.box)
		case ast.SendCase:
			args := append([]*Value{a.ch, w, index}, a.val...)
			b.chanCall("__sel_wait_send", a.chant, args...)
		}
	}
	b.unlockChannels(locks, n)
	for i, a := range arms {
		if a.ms != nil {
			a.timer = b.runtime("timer_add", a.ms, w, b.constant(int64(i)))
			a.timer.Scalar = true
		}
	}
	b.runtimeVoid("park")
	for _, a := range arms {
		switch {
		case a.sc.Kind == ast.RecvCase:
			b.chanCall("__sel_cancel_recv", a.chant, a.ch, w)
		case a.sc.Kind == ast.SendCase:
			b.chanCall("__sel_cancel_send", a.chant, a.ch, w)
		case a.timer != nil:
			b.runtimeVoid("timer_del", a.timer)
		}
	}
}

func (b *builder) selectArms(e *ast.Select, arms []*selectArm, w, index *Value) []*Value {
	temp := b.newTemp()
	done := b.newBlock()
	for i, a := range arms {
		body := b.newBlock()
		next := b.newBlock()
		match := b.emit(OpEq, index, b.constant(int64(i)))
		b.branch(match, body, next)
		b.enter(body)
		if a.sc.Local != nil {
			b.bind(a.sc.Local, b.chanCall("__sel_value", a.chant, w, a.box))
		}
		b.writeTemp(temp, b.value(a.sc.Body, e.Type))
		b.jump(done)
		b.enter(next)
	}
	b.cur.Kind = Unreachable
	b.enter(done)
	return b.readTemp(temp, e.Type.Size())
}
