package check

import (
	"maps"

	"lowlang/internal/ast"
	"lowlang/internal/builtin"
	"lowlang/internal/types"
)

type moveContext int

const (
	borrow moveContext = iota
	take
	discard
)

const (
	moved = 1
	maybe = 2
)

type moveState map[*ast.Local]int

type moveLoop struct {
	breaks    []moveState
	continues []moveState
}

type mover struct {
	c     *Checker
	ret   *types.Type
	state moveState
	dead  bool
	loops []*moveLoop
}

func (c *Checker) checkMoves(fn *ast.Func) {
	m := &mover{c: c, ret: fn.RetType, state: moveState{}}
	m.block(fn.Body, m.context(fn.RetType))
}

func (m *mover) context(t *types.Type) moveContext {
	switch {
	case t == types.Void:
		return discard
	case m.c.owns(t):
		return take
	}
	return borrow
}

func join(states ...moveState) moveState {
	out := moveState{}
	for _, s := range states {
		for l, v := range s {
			out[l] = max(out[l], v)
		}
	}
	for l, v := range out {
		for _, s := range states {
			if s[l] != v {
				out[l] = maybe
			}
		}
	}
	return out
}

func (m *mover) snapshot() moveState {
	return maps.Clone(m.state)
}

func (m *mover) branches(run ...func()) {
	start := m.snapshot()
	var ends []moveState
	for _, r := range run {
		m.openArm(start)
		r()
		ends = m.closeArm(ends)
	}
	m.merge(ends)
}

func (m *mover) openArm(start moveState) {
	m.state, m.dead = maps.Clone(start), false
}

func (m *mover) closeArm(ends []moveState) []moveState {
	if m.dead {
		return ends
	}
	return append(ends, m.state)
}

func (m *mover) merge(ends []moveState) {
	if len(ends) == 0 {
		m.dead = true
		return
	}
	m.state, m.dead = join(ends...), false
}

func (m *mover) block(b *ast.Block, ctx moveContext) {
	for _, s := range b.Stmts {
		if m.dead {
			return
		}
		m.stmt(s)
	}
	if b.Tail != nil && !m.dead {
		m.expr(b.Tail, ctx)
	}
}

func (m *mover) stmt(s ast.Stmt) {
	switch s := s.(type) {
	case *ast.Let:
		if s.Using {
			m.expr(s.Value, take)
			delete(m.state, s.Local)
			return
		}
		m.expr(s.Value, borrow)
	case *ast.Assign:
		m.assign(s)
	case *ast.ExprStmt:
		t := s.X.TypeOf()
		m.expr(s.X, discard)
		if t == types.Never {
			m.dead = true
		}
	case *ast.Return:
		if s.Value != nil {
			m.expr(s.Value, m.context(m.ret))
		}
		m.dead = true
	case *ast.Fail:
		m.expr(s.Value, borrow)
		m.dead = true
	case *ast.Break:
		loop := m.loops[len(m.loops)-1-s.Depth]
		loop.breaks = append(loop.breaks, m.snapshot())
		m.dead = true
	case *ast.Continue:
		loop := m.loops[len(m.loops)-1-s.Depth]
		loop.continues = append(loop.continues, m.snapshot())
		m.dead = true
	case *ast.For:
		m.loop(s.Cond == nil, func() {
			if s.Cond != nil {
				m.expr(s.Cond, borrow)
			}
		}, s.Body)
	case *ast.ForIn:
		m.expr(s.Iter, borrow)
		if s.End != nil {
			m.expr(s.End, borrow)
		}
		m.loop(false, func() {}, s.Body)
	case *ast.Defer:
		saved, dead := m.snapshot(), m.dead
		m.stmt(s.Body)
		m.state, m.dead = saved, dead
	case *ast.Spawn:
		m.block(s.Body, borrow)
	case *ast.Send:
		m.expr(s.Chan, borrow)
		m.expr(s.Value, borrow)
	}
}

func (m *mover) loop(forever bool, head func(), body *ast.Block) {
	entry := m.snapshot()
	var exit []moveState
	for pass := 0; pass < 2; pass++ {
		loop := &moveLoop{}
		m.loops = append(m.loops, loop)
		m.state, m.dead = maps.Clone(entry), false
		head()
		if !forever {
			exit = []moveState{m.snapshot()}
		}
		m.block(body, borrow)
		m.loops = m.loops[:len(m.loops)-1]
		back := loop.continues
		if !m.dead {
			back = append(back, m.state)
		}
		exit = append(exit, loop.breaks...)
		next := join(append([]moveState{entry}, back...)...)
		if maps.Equal(next, entry) {
			break
		}
		entry = next
	}
	if len(exit) == 0 {
		m.dead = true
		return
	}
	m.state, m.dead = join(exit...), false
}

func (m *mover) assign(s *ast.Assign) {
	if s.Release == nil {
		m.expr(s.Value, borrow)
		m.expr(s.Target, borrow)
		return
	}
	m.expr(s.Value, take)
	switch target := s.Target.(type) {
	case *ast.Ident:
		delete(m.state, target.Local)
	case *ast.Selector:
		root := m.root(target)
		if root == nil || (root.Flag == nil && !root.Ref) {
			m.c.fail(s.Pos, "a field with drop can only be assigned in a value bound with using")
		}
		m.expr(target.X, borrow)
	default:
		m.c.fail(s.Pos, "cannot store %s here; it has drop", s.Target.TypeOf())
	}
}

func (m *mover) root(e ast.Expr) *ast.Local {
	for {
		switch x := e.(type) {
		case *ast.Selector:
			e = x.X
		case *ast.Ident:
			return x.Local
		default:
			return nil
		}
	}
}

func branching(e ast.Expr) bool {
	switch e.(type) {
	case *ast.If, *ast.Match, *ast.Block, *ast.Select, *ast.Catch, *ast.Or, *ast.When, *ast.Region:
		return true
	}
	return false
}

func producer(e ast.Expr) bool {
	switch e := e.(type) {
	case *ast.Receiver:
		return producer(e.X)
	case *ast.Call:
		return e.Builtin == builtin.None
	case *ast.StructLit, *ast.Try, *ast.Catch, *ast.Or, *ast.When, *ast.If, *ast.Match, *ast.Block, *ast.Select, *ast.Region:
		return true
	}
	return false
}

func (m *mover) use(e *ast.Ident) {
	switch m.state[e.Local] {
	case moved:
		m.c.fail(e.Pos, "%s is used after it was moved", e.Name)
	case maybe:
		m.c.fail(e.Pos, "%s may have been moved", e.Name)
	}
}

func (m *mover) expr(e ast.Expr, ctx moveContext) {
	if ctx == discard && m.c.owns(e.TypeOf()) && producer(e) && !branching(e) {
		m.c.fail(e.Position(), "%s has drop; bind it with using", e.TypeOf())
	}
	owned := ctx == take && m.c.owns(e.TypeOf())
	switch e := e.(type) {
	case *ast.Ident:
		if e.Local == nil {
			return
		}
		if e.Local.Flag != nil {
			m.use(e)
			if owned {
				m.state[e.Local] = moved
				e.Moved = true
			}
			return
		}
		if owned {
			m.c.fail(e.Pos, "cannot move %s; it is borrowed", e.Name)
		}
	case *ast.Selector:
		m.expr(e.X, borrow)
		if owned {
			m.c.fail(e.Pos, "cannot move field %s out of its struct", e.Name)
		}
	case *ast.Call:
		m.call(e)
	case *ast.StructLit:
		for _, init := range e.Fields {
			m.expr(init.Value, m.context(init.Field.Type))
		}
	case *ast.Unary:
		m.expr(e.X, borrow)
	case *ast.Binary:
		m.binary(e)
	case *ast.Receiver:
		m.expr(e.X, ctx)
	case *ast.Ref:
		m.expr(e.X, borrow)
	case *ast.Index:
		m.expr(e.X, borrow)
		m.expr(e.Index, borrow)
	case *ast.Slicing:
		m.expr(e.X, borrow)
		for _, bound := range []ast.Expr{e.Lo, e.Hi} {
			if bound != nil {
				m.expr(bound, borrow)
			}
		}
	case *ast.SliceLit:
		for _, el := range e.Elems {
			m.expr(el, borrow)
		}
	case *ast.MapLit:
		for i := range e.Keys {
			m.expr(e.Keys[i], borrow)
			m.expr(e.Values[i], borrow)
		}
	case *ast.Interp:
		for _, part := range e.Parts {
			m.expr(part, borrow)
		}
	case *ast.Block:
		m.block(e, ctx)
	case *ast.Region:
		m.block(e.Body, ctx)
	case *ast.If:
		m.ifChain(e, ctx)
	case *ast.Match:
		m.expr(e.X, borrow)
		m.arms(e.Arms, ctx)
	case *ast.Try:
		m.expr(e.X, borrow)
	case *ast.Or:
		m.expr(e.X, borrow)
		m.branches(func() {}, func() { m.expr(e.Fallback, ctx) })
	case *ast.When:
		m.expr(e.Cond, borrow)
		m.branches(func() { m.expr(e.X, ctx) }, func() {})
	case *ast.Catch:
		m.expr(e.X, borrow)
		if e.Map != nil {
			m.branches(func() {}, func() { m.expr(e.Map, take) })
			return
		}
		m.branches(append([]func(){func() {}}, m.armRuns(e.Arms, ctx)...)...)
	case *ast.Select:
		var arms []func()
		for _, sc := range e.Cases {
			arms = append(arms, func() {
				for _, part := range []ast.Expr{sc.Chan, sc.Value} {
					if part != nil {
						m.expr(part, borrow)
					}
				}
				m.expr(sc.Body, ctx)
			})
		}
		m.branches(arms...)
	}
}

func (m *mover) binary(e *ast.Binary) {
	spine := ast.LeftSpine(e)
	m.expr(spine[len(spine)-1].X, borrow)
	for i := len(spine) - 1; i >= 0; i-- {
		m.expr(spine[i].Y, borrow)
	}
}

func (m *mover) ifChain(e *ast.If, ctx moveContext) {
	var ends []moveState
	for link := e; ; {
		m.expr(link.Cond, borrow)
		start := m.snapshot()
		m.openArm(start)
		m.block(link.Then, ctx)
		ends = m.closeArm(ends)
		m.openArm(start)
		next, chained := link.Else.(*ast.If)
		if !chained {
			if link.Else != nil {
				m.expr(link.Else, ctx)
			}
			ends = m.closeArm(ends)
			break
		}
		link = next
	}
	m.merge(ends)
}

func (m *mover) arms(arms []*ast.Arm, ctx moveContext) {
	m.branches(m.armRuns(arms, ctx)...)
}

func (m *mover) armRuns(arms []*ast.Arm, ctx moveContext) []func() {
	var run []func()
	for _, arm := range arms {
		run = append(run, func() {
			if arm.Pattern.Test != nil {
				m.expr(arm.Pattern.Test, borrow)
			}
			m.expr(arm.Body, ctx)
		})
	}
	return run
}

func (m *mover) call(e *ast.Call) {
	if _, direct := e.Fn.(*ast.Ident); !direct || e.Target == nil {
		m.expr(e.Fn, borrow)
	}
	for _, arg := range e.Args {
		if _, ref := arg.(*ast.Ref); !ref && m.c.owns(arg.TypeOf()) && producer(arg) {
			m.c.fail(arg.Position(), "%s has drop; bind it with using before passing it", arg.TypeOf())
		}
		m.expr(arg, borrow)
	}
}
