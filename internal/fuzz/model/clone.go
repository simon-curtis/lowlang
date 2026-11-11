package model

func (p *Program) Clone() *Program {
	funcs := make([]*Func, len(p.Funcs))
	for i, f := range p.Funcs {
		funcs[i] = f.Clone()
	}
	return &Program{Funcs: funcs, Main: cloneStmts(p.Main)}
}

func (f *Func) Clone() *Func {
	c := *f
	c.Params = append([]Param(nil), f.Params...)
	c.Body = cloneStmts(f.Body)
	c.Tail = cloneExpr(f.Tail)
	return &c
}

func cloneStmts(stmts []Stmt) []Stmt {
	if stmts == nil {
		return nil
	}
	out := make([]Stmt, len(stmts))
	for i, s := range stmts {
		out[i] = cloneStmt(s)
	}
	return out
}

func cloneStmt(s Stmt) Stmt {
	switch s := s.(type) {
	case *Let:
		c := *s
		c.Init = cloneExpr(s.Init)
		return &c
	case *Assign:
		c := *s
		c.Value = cloneExpr(s.Value)
		return &c
	case *IfStmt:
		return &IfStmt{Cond: cloneExpr(s.Cond), Then: cloneStmts(s.Then), Else: cloneStmts(s.Else)}
	case *Range:
		c := *s
		c.Lo, c.Hi, c.Body = cloneExpr(s.Lo), cloneExpr(s.Hi), cloneStmts(s.Body)
		return &c
	case *While:
		c := *s
		c.Cond, c.Body = cloneExpr(s.Cond), cloneStmts(s.Body)
		return &c
	case *Jump:
		c := *s
		return &c
	case *Return:
		return &Return{Value: cloneExpr(s.Value)}
	case *Print:
		return &Print{Value: cloneExpr(s.Value)}
	}
	panic("unknown statement")
}

func cloneExpr(e Expr) Expr {
	switch e := e.(type) {
	case nil:
		return nil
	case *Lit:
		c := *e
		return &c
	case *Var:
		c := *e
		return &c
	case *Unary:
		return &Unary{Op: e.Op, X: cloneExpr(e.X)}
	case *Binary:
		return &Binary{Op: e.Op, L: cloneExpr(e.L), R: cloneExpr(e.R)}
	case *Convert:
		return &Convert{To: e.To, X: cloneExpr(e.X)}
	case *IfExpr:
		return &IfExpr{Cond: cloneExpr(e.Cond), Then: cloneExpr(e.Then), Else: cloneExpr(e.Else)}
	case *Call:
		args := make([]Expr, len(e.Args))
		for i, a := range e.Args {
			args[i] = cloneExpr(a)
		}
		return &Call{Callee: e.Callee, Result: e.Result, Args: args}
	case *Math:
		return &Math{Op: e.Op, X: cloneExpr(e.X)}
	}
	panic("unknown expression")
}
