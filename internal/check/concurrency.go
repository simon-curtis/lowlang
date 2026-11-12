package check

import (
	"fmt"

	"lowlang/internal/ast"
	"lowlang/internal/builtin"
	"lowlang/internal/types"
)

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
	e.Builtin = builtin.Chan
	e.Type = c.universe.Chan(c.contained(inst.Types[0], "a channel"))
}

func (c *Checker) send(s *ast.Send) {
	t := c.channel(s.Chan)
	c.expectType(s.Value, t.Elem)
}

func (c *Checker) spawn(s *ast.Spawn) {
	c.spawns++
	call := s.Call
	block := &ast.Block{Pos: s.Pos}
	var lets []*ast.Let
	bind := func(x ast.Expr, name string) ast.Expr {
		if _, ok := x.(*ast.Ref); ok {
			c.fail(x.Position(), "spawn cannot pass mut arguments")
		}
		let := &ast.Let{Name: name, Value: x, Pos: x.Position()}
		lets = append(lets, let)
		block.Stmts = append(block.Stmts, let)
		return &ast.Ident{Name: name, Pos: x.Position()}
	}
	if !c.namedFunc(call.Fn) {
		call.Fn = bind(call.Fn, fmt.Sprintf("__spawn%d_fn", c.spawns))
	}
	for i, arg := range call.Args {
		call.Args[i] = bind(arg, fmt.Sprintf("__spawn%d_%d", c.spawns, i))
	}
	closure := &ast.Closure{Body: &ast.Block{Tail: call, Pos: s.Pos}, Pos: s.Pos}
	start := &ast.ExprStmt{X: &ast.Call{Fn: &ast.Ident{Name: "__spawn", Pos: s.Pos}, Args: []ast.Expr{closure}, Pos: s.Pos}}
	block.Stmts = append(block.Stmts, start)
	s.Body = block
	c.pushScope()
	defer c.popScope()
	for _, let := range lets {
		if t := c.bind(let); c.droppable(t) {
			c.fail(let.Pos, "spawn cannot take %s; it has drop", t)
		}
	}
	c.stmt(start)
	block.Type = types.Void
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
				sc.Local = c.declare(sc.Name, c.universe.Optional(t.Elem), false)
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

func (c *Checker) lockWord(arg ast.Expr, name string) {
	r, ok := arg.(*ast.Ref)
	if !ok {
		c.fail(arg.Position(), "%s needs a mut i64 place", name)
	}
	if t := c.borrow(r); t != types.I64 {
		c.fail(r.Pos, "%s needs an i64, found %s", name, t)
	}
}
