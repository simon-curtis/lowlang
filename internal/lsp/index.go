package lsp

import (
	"lowlang/internal/ast"
	"lowlang/internal/diag"
	"lowlang/internal/types"
)

type qualifier struct {
	alias string
}

type occurrence struct {
	pos    diag.Pos
	name   string
	node   any
	local  *ast.Local
	parent *types.Type
}

type index struct {
	file         *ast.File
	text         text
	occurrences  []occurrence
	declarations map[*ast.Local]int
}

type indexed struct {
	path  string
	file  *ast.File
	src   string
	index *index
}

func (i indexed) matches(path string, file *ast.File, src []byte) bool {
	return i.index != nil && i.path == path && i.file == file && i.src == string(src)
}

type indexer struct {
	index     *index
	locals    map[*ast.Param]*ast.Local
	parents   map[ast.Node]*types.Type
	shorthand map[ast.Expr]bool

	shorthandBinding map[*ast.Binding]bool
}

func newIndex(file *ast.File, t text) *index {
	x := &index{file: file, text: t, declarations: map[*ast.Local]int{}}
	b := &indexer{
		index:     x,
		locals:    map[*ast.Param]*ast.Local{},
		parents:   map[ast.Node]*types.Type{},
		shorthand: map[ast.Expr]bool{},

		shorthandBinding: map[*ast.Binding]bool{},
	}
	ast.Inspect(file, b.visit)
	for i, o := range x.occurrences {
		if o.local == nil {
			continue
		}
		if _, seen := x.declarations[o.local]; !seen {
			x.declarations[o.local] = i
		}
	}
	return x
}

func (x *index) at(line, byteCol int) *occurrence {
	for i := range x.occurrences {
		o := &x.occurrences[i]
		start := o.pos.Col - 1
		if o.pos.Line-1 == line && byteCol >= start && byteCol <= start+len(o.name) {
			return o
		}
	}
	return nil
}

func (x *index) declaration(l *ast.Local) *occurrence {
	i, ok := x.declarations[l]
	if !ok {
		return nil
	}
	return &x.occurrences[i]
}

func (b *indexer) add(o occurrence) {
	b.index.occurrences = append(b.index.occurrences, o)
}

func (b *indexer) declared(at diag.Pos, name string, node ast.Node, local *ast.Local) {
	pos := b.index.text.nameAfter(at, name)
	b.add(occurrence{pos: pos, name: name, node: node, local: local})
}

func (b *indexer) bindParams(params []*ast.Param, locals []*ast.Local) {
	for i, p := range params {
		if i < len(locals) {
			b.locals[p] = locals[i]
		}
	}
}

func (b *indexer) visit(n ast.Node) bool {
	switch n := n.(type) {
	case *ast.Import:
		b.declared(n.Pos, n.Path, n, nil)
	case *ast.StructDecl:
		b.declared(n.Pos, n.Name, n, nil)
		for _, f := range n.Fields {
			b.parents[f] = n.Type
		}
	case *ast.EnumDecl:
		b.declared(n.Pos, n.Name, n, nil)
		for _, v := range n.Variants {
			b.parents[v] = n.Type
		}
	case *ast.FieldDecl:
		b.add(occurrence{pos: n.Pos, name: n.Name, node: n, parent: b.parents[n]})
	case *ast.VariantDecl:
		b.add(occurrence{pos: n.Pos, name: n.Name, node: n, parent: b.parents[n]})
	case *ast.TypeParam:
		b.add(occurrence{pos: n.Pos, name: n.Name, node: n})
	case *ast.Func:
		b.declared(n.Pos, n.Name, n, nil)
		b.bindParams(n.Params, n.ParamLocals)
	case *ast.Closure:
		if n.Func != nil {
			b.bindParams(n.Params, n.Func.ParamLocals)
		}
	case *ast.Param:
		b.add(occurrence{pos: n.Pos, name: n.Name, node: n, local: b.locals[n]})
	case *ast.TypeName:
		b.typeName(n)
	case *ast.Let:
		if n.Pattern == nil {
			b.declared(n.Pos, n.Name, n, n.Local)
		}
	case *ast.ForIn:
		b.forIn(n)
	case *ast.StructPattern:
		for _, f := range n.Fields {
			b.parents[f] = n.Type
		}
	case *ast.FieldPattern:
		b.fieldPattern(n)
	case *ast.Ident:
		if !b.shorthand[n] {
			b.add(occurrence{pos: n.Pos, name: n.Name, node: n})
		}
	case *ast.Selector:
		b.add(occurrence{pos: n.Pos, name: n.Name, node: n})
	case *ast.Call:
		b.callee(n)
	case *ast.StructLit:
		b.structLit(n)
	case *ast.FieldInit:
		if n.Form != ast.FieldImplied {
			b.add(occurrence{pos: n.Pos, name: n.Name, node: n, parent: b.parents[n]})
		}
	case *ast.Match:
		b.patterns(n.Arms, n.X.TypeOf())
	case *ast.Catch:
		b.patterns(n.Arms, errorOf(n.X.TypeOf()))
	case *ast.Pattern:
		b.pattern(n)
	case *ast.Binding:
		if !b.shorthandBinding[n] {
			b.add(occurrence{pos: n.Pos, name: n.Name, node: n, local: n.Local})
		}
	case *ast.SelectCase:
		if n.Name != "" {
			b.add(occurrence{pos: n.Pos, name: n.Name, node: n, local: n.Local})
		}
	}
	return true
}

func (b *indexer) typeName(t *ast.TypeName) {
	if t.Name == "" {
		return
	}
	pos := t.Pos
	if t.Module != "" {
		b.add(occurrence{pos: pos, name: t.Module, node: qualifier{alias: t.Module}})
		pos.Col += len(t.Module) + 1
	}
	b.add(occurrence{pos: pos, name: t.Name, node: t})
}

func (b *indexer) forIn(s *ast.ForIn) {
	pos := s.Pos
	if s.Key != "" {
		pos = b.index.text.nameAfter(pos, s.Key)
		b.add(occurrence{pos: pos, name: s.Key, node: s, local: s.KeyLocal})
		pos.Col += len(s.Key)
	}
	if s.Pattern == nil {
		b.declared(pos, s.Value, s, s.ValueLocal)
	}
}

func (b *indexer) fieldPattern(f *ast.FieldPattern) {
	o := occurrence{pos: f.Pos, name: f.Name, node: f, parent: b.parents[f]}
	if f.Shorthand {
		b.shorthandBinding[f.Bind] = true
		o.local = f.Bind.Local
	}
	b.add(o)
}

func (b *indexer) callee(e *ast.Call) {
	switch fn := e.Fn.(type) {
	case *ast.Ident:
		b.add(occurrence{pos: fn.Pos, name: fn.Name, node: e})
	case *ast.Selector:
		b.add(occurrence{pos: fn.Pos, name: fn.Name, node: e})
	}
}

func (b *indexer) structLit(e *ast.StructLit) {
	b.add(occurrence{pos: e.Pos, name: e.Name, node: e})
	for _, f := range e.Fields {
		b.parents[f] = e.Type
		if f.Form == ast.FieldShorthand {
			b.shorthand[f.Value] = true
		}
	}
}

func errorOf(t *types.Type) *types.Type {
	if t != nil && t.Kind == types.ResultKind {
		return t.Err
	}
	return t
}

func (b *indexer) patterns(arms []*ast.Arm, scrutinee *types.Type) {
	for _, a := range arms {
		b.parents[a.Pattern] = scrutinee
	}
}

func (b *indexer) pattern(p *ast.Pattern) {
	switch {
	case p.Test != nil:
	case p.Local != nil:
		b.add(occurrence{pos: p.Pos, name: p.Name, node: p, local: p.Local})
	default:
		b.add(occurrence{pos: p.Pos, name: p.Name, node: p, parent: b.parents[p]})
	}
}
