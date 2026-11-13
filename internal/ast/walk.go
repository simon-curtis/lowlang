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
	case *File:
		w.file(n)
	case *StructDecl:
		w.typeParams(n.TypeParams)
		for _, f := range n.Fields {
			w.node(f)
		}
		if n.Drop != nil {
			w.node(n.Drop)
		}
	case *FieldDecl:
		w.typeName(n.TypeName)
		w.expr(n.Default)
	case *EnumDecl:
		w.typeParams(n.TypeParams)
		for _, v := range n.Variants {
			w.node(v)
		}
	case *VariantDecl:
		w.typeNames(n.Fields)
	case *Const:
		w.typeName(n.TypeName)
		w.expr(n.Value)
	case *Func:
		w.typeParams(n.TypeParams)
		w.params(n.Params)
		w.typeName(n.Result)
		w.typeName(n.Error)
		w.block(n.Body)
	case *Param:
		w.typeName(n.TypeName)
	case *TypeName:
		w.typeName(n.Key)
		w.typeName(n.Elem)
		w.typeName(n.Result)
		w.typeName(n.Error)
		w.typeNames(n.Args)
		w.typeNames(n.Params)
	case *Pattern:
		w.expr(n.Test)
		for _, b := range n.Bindings {
			w.node(b)
		}
	case *SelectCase:
		w.expr(n.Chan)
		w.expr(n.Value)
		w.expr(n.Body)
	case *FieldInit:
		w.expr(n.Value)
	case *Block:
		for _, s := range n.Stmts {
			w.node(s)
		}
		w.expr(n.Tail)
	case *Let:
		w.pattern(n.Pattern)
		w.typeName(n.TypeName)
		w.expr(n.Value)
	case *StructPattern:
		for _, f := range n.Fields {
			w.node(f)
		}
	case *FieldPattern:
		w.pattern(n.Nested)
		if n.Bind != nil {
			w.node(n.Bind)
		}
	case *Assign:
		w.expr(n.Target)
		w.expr(n.Value)
	case *ExprStmt:
		w.expr(n.X)
	case *For:
		w.expr(n.Cond)
		w.block(n.Body)
	case *ForIn:
		w.pattern(n.Pattern)
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
		if n.Body != nil {
			w.block(n.Body)
		} else {
			w.node(n.Call)
		}
	case *Send:
		w.expr(n.Chan)
		w.expr(n.Value)
	case *Select:
		for _, c := range n.Cases {
			w.node(c)
		}
	case *Selector:
		w.expr(n.X)
	case *StructLit:
		w.typeNames(n.TypeArgs)
		for _, f := range n.Fields {
			w.node(f)
		}
	case *Ref:
		w.expr(n.X)
	case *Index:
		w.expr(n.X)
		w.expr(n.Index)
	case *Instance:
		w.expr(n.X)
		w.typeNames(n.Types)
		w.expr(n.Fallback)
	case *Closure:
		w.params(n.Params)
		w.typeName(n.Result)
		w.typeName(n.Error)
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
		if n.Map != nil {
			w.expr(n.Map)
		}
	case *Or:
		w.expr(n.X)
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

func (w walker) pattern(p *StructPattern) {
	if p != nil {
		w.node(p)
	}
}

func (w walker) block(b *Block) {
	if b != nil {
		w.node(b)
	}
}

func (w walker) arms(arms []*Arm) {
	for _, a := range arms {
		w.node(a.Pattern)
		w.expr(a.Body)
	}
}

func (w walker) file(f *File) {
	for _, imp := range f.Imports {
		w.node(imp)
	}
	for _, d := range f.Structs {
		w.node(d)
	}
	for _, d := range f.Enums {
		w.node(d)
	}
	for _, d := range f.Consts {
		w.node(d)
	}
	for fn := range f.Declared() {
		w.node(fn)
	}
}

func (w walker) params(params []*Param) {
	for _, p := range params {
		w.node(p)
	}
}

func (w walker) typeParams(params []*TypeParam) {
	for _, p := range params {
		w.node(p)
	}
}

func (w walker) typeName(t *TypeName) {
	if t != nil {
		w.node(t)
	}
}

func (w walker) typeNames(list []*TypeName) {
	for _, t := range list {
		w.typeName(t)
	}
}
