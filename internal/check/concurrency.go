package check

import (
	"fmt"

	"lowlang/internal/ast"
	"lowlang/internal/lexer"
	"lowlang/internal/types"
)

var chanFuncs = []string{
	"__chan_new", "__chan_send", "__chan_recv", "__chan_close", "__box",
	"__sel_recv", "__sel_send", "__sel_wait_recv", "__sel_wait_send", "__sel_value",
}

var softBuiltins = map[string]bool{"close": true, "sleep": true, "wait_readable": true, "wait_writable": true}

func (c *Checker) chanOf(elem *types.Type) *types.Type {
	if t, ok := c.chans[elem]; ok {
		return t
	}
	t := types.NewChan(elem)
	c.chans[elem] = t
	c.chanOps(t)
	return t
}

func (c *Checker) chanOps(t *types.Type) {
	if t.Abstract() {
		return
	}
	if !c.funcsReady {
		c.waitingChans = append(c.waitingChans, t)
		return
	}
	args := []*types.Type{t.Elem}
	for _, name := range chanFuncs {
		c.instance(c.scope("").funcs[name], args, lexer.Pos{})
	}
}

func (c *Checker) channel(e ast.Expr) *types.Type {
	t := c.plain(e, nil)
	if t.Kind != types.ChanKind {
		c.fail(e.Position(), "expected a channel, found %s", t)
	}
	return t
}

func (c *Checker) makeChan(e *ast.Call, inst *ast.Instance) {
	if len(inst.Types) != 1 {
		c.fail(inst.Pos, "chan takes 1 type argument, found %d", len(inst.Types))
	}
	if len(e.Args) > 1 {
		c.fail(e.Pos, "chan takes at most 1 argument, found %d", len(e.Args))
	}
	if len(e.Args) == 1 {
		c.integer(e.Args[0])
	}
	e.Builtin = "chan"
	e.Type = c.chanOf(c.contained(inst.Types[0], "a channel"))
}

func (c *Checker) send(s *ast.Send) {
	t := c.channel(s.Chan)
	c.expectType(s.Value, t.Elem)
}

func (c *Checker) spawn(s *ast.Spawn) {
	c.spawns++
	call := s.Call
	block := &ast.Block{Pos: s.Pos}
	bind := func(x ast.Expr, name string) ast.Expr {
		if _, ok := x.(*ast.Ref); ok {
			c.fail(x.Position(), "spawn cannot pass mut arguments")
		}
		block.Stmts = append(block.Stmts, &ast.Let{Name: name, Spawned: true, Value: x, Pos: x.Position()})
		return &ast.Ident{Name: name, Pos: x.Position()}
	}
	if !c.namedFunc(call.Fn) {
		call.Fn = bind(call.Fn, fmt.Sprintf("__spawn%d_fn", c.spawns))
	}
	for i, arg := range call.Args {
		call.Args[i] = bind(arg, fmt.Sprintf("__spawn%d_%d", c.spawns, i))
	}
	closure := &ast.Closure{Body: &ast.Block{Tail: call, Pos: s.Pos}, Pos: s.Pos}
	start := &ast.Call{Fn: &ast.Ident{Name: "__spawn", Pos: s.Pos}, Args: []ast.Expr{closure}, Pos: s.Pos}
	block.Stmts = append(block.Stmts, &ast.ExprStmt{X: start})
	s.Body = block
	c.block(block, nil)
}

func (c *Checker) namedFunc(x ast.Expr) bool {
	switch x := x.(type) {
	case *ast.Ident:
		return c.find(x.Name) == nil
	case *ast.Selector:
		id, ok := x.X.(*ast.Ident)
		return ok && c.isModule(id.Name)
	case *ast.Instance:
		return c.namedFunc(x.X)
	}
	return false
}

func (c *Checker) selectExpr(e *ast.Select, want *types.Type) {
	result := types.Never
	elses := 0
	for _, sc := range e.Cases {
		c.pushScope()
		switch sc.Kind {
		case ast.RecvCase:
			t := c.channel(sc.Chan)
			if sc.Name != "" {
				sc.Local = c.declare(sc.Name, c.optional(t.Elem), false)
			}
		case ast.SendCase:
			t := c.channel(sc.Chan)
			c.expectType(sc.Value, t.Elem)
		case ast.AfterCase:
			c.integer(sc.Value)
		case ast.ElseCase:
			elses++
			if elses > 1 {
				c.fail(sc.Pos, "select has more than one else")
			}
		}
		result = unify(result, c.expr(sc.Body, want))
		c.popScope()
	}
	e.Type = result
}

func (c *Checker) concurrencyBuiltin(e *ast.Call, name string) bool {
	if softBuiltins[name] && c.local().funcs[name] != nil {
		return false
	}
	switch name {
	case "close":
		c.single(e, name)
		c.channel(e.Args[0])
		e.Type = types.Void
	case "sleep":
		c.single(e, name)
		c.integer(e.Args[0])
		e.Type = types.Void
	case "wait_readable", "wait_writable":
		c.single(e, name)
		c.integer(e.Args[0])
		e.Type = types.Void
	case "__spawn":
		c.single(e, name)
		t := c.plain(e.Args[0], nil)
		if t.Kind != types.FuncKind || t.C || len(t.Params) > 0 {
			c.fail(e.Pos, "__spawn takes a function value with no parameters")
		}
		e.Type = types.Void
	case "__current", "__park":
		if len(e.Args) != 0 {
			c.fail(e.Pos, "%s takes no arguments", name)
		}
		e.Type = types.Void
		if name == "__current" {
			e.Type = c.raw(nil)
		}
	case "__ready":
		c.single(e, name)
		c.expectType(e.Args[0], c.raw(nil))
		e.Type = types.Void
	case "__lock", "__unlock":
		c.single(e, name)
		c.lockWord(e.Args[0], name)
		e.Type = types.Void
	case "__cas":
		if len(e.Args) != 3 {
			c.fail(e.Pos, "__cas takes 3 arguments")
		}
		c.lockWord(e.Args[0], name)
		c.expectType(e.Args[1], types.I64)
		c.expectType(e.Args[2], types.I64)
		e.Type = types.Bool
	case "__panic":
		c.single(e, name)
		c.expectType(e.Args[0], types.Str)
		e.Type = types.Never
	default:
		return false
	}
	e.Builtin = name
	return true
}

func (c *Checker) lockWord(arg ast.Expr, name string) {
	r, ok := arg.(*ast.Ref)
	if !ok {
		c.fail(arg.Position(), "%s needs a mut i64 place", name)
	}
	if t := c.borrow(r); t != types.I64 {
		c.fail(r.Pos, "%s needs an i64, found %s", name, t)
	}
}
