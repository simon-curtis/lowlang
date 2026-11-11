package ast

func Inspect(n Node, visit func(Node) bool) {
	w := walker{visit: visit}
	w.node(n)
}

type walker struct {
	visit func(Node) bool
}

func (w walker) node(n Node) {
	if n == nil || !w.visit(n) {
		return
	}
	switch n := n.(type) {
	case *Block:
		for _, s := range n.Stmts {
			w.node(s)
		}
		w.expr(n.Tail)
	case *Let:
		w.expr(n.Value)
	case *Assign:
		w.expr(n.Target)
		w.expr(n.Value)
	case *ExprStmt:
		w.expr(n.X)
	case *For:
		w.expr(n.Cond)
		w.block(n.Body)
	case *ForIn:
		w.expr(n.Iter)
		w.expr(n.End)
		w.block(n.Body)
	case *Return:
		w.expr(n.Value)
	case *Fail:
		w.expr(n.Value)
	case *Defer:
		if n.Body != nil {
			w.node(n.Body)
		}
	case *Spawn:
		if n.Call != nil {
			w.node(n.Call)
		}
		w.block(n.Body)
	case *Send:
		w.expr(n.Chan)
		w.expr(n.Value)
	case *Select:
		for _, c := range n.Cases {
			w.expr(c.Chan)
			w.expr(c.Value)
			w.expr(c.Body)
		}
	case *Selector:
		w.expr(n.X)
	case *StructLit:
		for _, f := range n.Fields {
			w.expr(f.Value)
		}
	case *Ref:
		w.expr(n.X)
	case *Index:
		w.expr(n.X)
		w.expr(n.Index)
	case *Instance:
		w.expr(n.X)
		w.expr(n.Fallback)
	case *Closure:
		w.block(n.Body)
	case *Slicing:
		w.expr(n.X)
		w.expr(n.Lo)
		w.expr(n.Hi)
	case *SliceLit:
		w.exprs(n.Elems)
	case *MapLit:
		w.exprs(n.Keys)
		w.exprs(n.Values)
	case *Interp:
		w.exprs(n.Parts)
	case *Unary:
		w.expr(n.X)
	case *Binary:
		w.expr(n.X)
		w.expr(n.Y)
	case *Call:
		w.expr(n.Fn)
		w.exprs(n.Args)
	case *If:
		w.expr(n.Cond)
		w.block(n.Then)
		w.expr(n.Else)
	case *Match:
		w.expr(n.X)
		w.arms(n.Arms)
	case *Try:
		w.expr(n.X)
	case *Catch:
		w.expr(n.X)
		w.arms(n.Arms)
		w.expr(n.Fallback)
	case *Receiver:
		w.expr(n.X)
	}
}

func (w walker) expr(e Expr) {
	if e != nil {
		w.node(e)
	}
}

func (w walker) exprs(list []Expr) {
	for _, e := range list {
		w.expr(e)
	}
}

func (w walker) block(b *Block) {
	if b != nil {
		w.node(b)
	}
}

func (w walker) arms(arms []*Arm) {
	for _, a := range arms {
		if a.Pattern.Test != nil {
			w.expr(a.Pattern.Test)
		}
		w.expr(a.Body)
	}
}
