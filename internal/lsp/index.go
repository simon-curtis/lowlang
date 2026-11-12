package lsp

import (
	"lowlang/internal/ast"
	"lowlang/internal/lexer"
	"lowlang/internal/types"
)

type moduleRef struct {
	alias string
}

type occurrence struct {
	pos    lexer.Pos
	name   string
	node   any
	local  *ast.Local
	parent *types.Type
}

type index struct {
	file        *ast.File
	text        text
	occurrences []occurrence
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

func newIndex(file *ast.File, t text) *index {
	x := &index{file: file, text: t}
	x.declarations()
	return x
}

func (x *index) add(o occurrence) {
	x.occurrences = append(x.occurrences, o)
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
	if l == nil {
		return nil
	}
	for i := range x.occurrences {
		if x.occurrences[i].local == l {
			return &x.occurrences[i]
		}
	}
	return nil
}

func (x *index) declarations() {
	for _, imp := range x.file.Imports {
		x.add(occurrence{pos: x.text.nameAfter(imp.Pos, imp.Path), name: imp.Path, node: imp})
	}
	for _, d := range x.file.Structs {
		x.add(occurrence{pos: x.text.nameAfter(d.Pos, d.Name), name: d.Name, node: d})
		x.typeParams(d.TypeParams)
		for _, f := range d.Fields {
			x.add(occurrence{pos: f.Pos, name: f.Name, node: f})
			x.typeName(f.TypeName)
			x.expr(f.Default)
		}
		x.stmt(d.Drop)
	}
	for _, d := range x.file.Enums {
		x.add(occurrence{pos: x.text.nameAfter(d.Pos, d.Name), name: d.Name, node: d})
		x.typeParams(d.TypeParams)
		for _, v := range d.Variants {
			x.add(occurrence{pos: v.Pos, name: v.Name, node: v})
			for _, t := range v.Fields {
				x.typeName(t)
			}
		}
	}
	for _, e := range x.file.Externs {
		for _, f := range e.Funcs {
			x.fn(f)
		}
	}
	for _, f := range x.file.Funcs {
		if f.Template == nil && f.Closure == nil {
			x.fn(f)
		}
	}
}

func (x *index) typeParams(params []*ast.TypeParam) {
	for _, p := range params {
		x.add(occurrence{pos: p.Pos, name: p.Name, node: p})
	}
}

func (x *index) fn(f *ast.Func) {
	x.add(occurrence{pos: x.text.nameAfter(f.Pos, f.Name), name: f.Name, node: f})
	x.typeParams(f.TypeParams)
	x.params(f.Params, f.ParamLocals)
	x.typeName(f.Result)
	x.typeName(f.Error)
	if f.Body != nil {
		x.expr(f.Body)
	}
}

func (x *index) params(params []*ast.Param, locals []*ast.Local) {
	for i, p := range params {
		var local *ast.Local
		if i < len(locals) {
			local = locals[i]
		}
		x.add(occurrence{pos: p.Pos, name: p.Name, node: p, local: local})
		x.typeName(p.TypeName)
	}
}

func (x *index) typeName(t *ast.TypeName) {
	if t == nil {
		return
	}
	if t.Name != "" {
		pos := t.Pos
		if t.Module != "" {
			x.add(occurrence{pos: pos, name: t.Module, node: moduleRef{alias: t.Module}})
			pos.Col += len(t.Module) + 1
		}
		x.add(occurrence{pos: pos, name: t.Name, node: t})
	}
	x.typeName(t.Key)
	x.typeName(t.Elem)
	x.typeName(t.Result)
	x.typeName(t.Error)
	for _, a := range t.Args {
		x.typeName(a)
	}
	for _, p := range t.Params {
		x.typeName(p)
	}
}

func (x *index) stmt(s ast.Stmt) {
	switch s := s.(type) {
	case *ast.Let:
		x.add(occurrence{pos: x.text.nameAfter(s.Pos, s.Name), name: s.Name, node: s, local: s.Local})
		x.typeName(s.TypeName)
		x.expr(s.Value)
	case *ast.Assign:
		x.expr(s.Target)
		x.expr(s.Value)
	case *ast.ExprStmt:
		x.expr(s.X)
	case *ast.For:
		x.expr(s.Cond)
		x.expr(s.Body)
	case *ast.Return:
		x.expr(s.Value)
	case *ast.Fail:
		x.expr(s.Value)
	case *ast.Defer:
		x.stmt(s.Body)
	case *ast.Spawn:
		x.expr(s.Call)
	case *ast.Send:
		x.expr(s.Chan)
		x.expr(s.Value)
	case *ast.ForIn:
		x.forIn(s)
	}
}

func (x *index) forIn(s *ast.ForIn) {
	pos := s.Pos
	if s.Key != "" {
		pos = x.text.nameAfter(pos, s.Key)
		x.add(occurrence{pos: pos, name: s.Key, node: s, local: s.KeyLocal})
		pos.Col += len(s.Key)
	}
	x.add(occurrence{pos: x.text.nameAfter(pos, s.Value), name: s.Value, node: s, local: s.ValueLocal})
	x.expr(s.Iter)
	x.expr(s.End)
	x.expr(s.Body)
}

func (x *index) exprs(list []ast.Expr) {
	for _, e := range list {
		x.expr(e)
	}
}

func (x *index) expr(e ast.Expr) {
	switch e := e.(type) {
	case *ast.Block:
		if e == nil {
			return
		}
		for _, s := range e.Stmts {
			x.stmt(s)
		}
		x.expr(e.Tail)
	case *ast.Ident:
		x.add(occurrence{pos: e.Pos, name: e.Name, node: e})
	case *ast.Selector:
		x.expr(e.X)
		x.add(occurrence{pos: e.Pos, name: e.Name, node: e})
	case *ast.StructLit:
		x.structLit(e)
	case *ast.Ref:
		x.expr(e.X)
	case *ast.Receiver:
		x.expr(e.X)
	case *ast.Index:
		x.expr(e.X)
		x.expr(e.Index)
	case *ast.Instance:
		x.expr(e.X)
		for _, t := range e.Types {
			x.typeName(t)
		}
	case *ast.Closure:
		x.closure(e)
	case *ast.Slicing:
		x.expr(e.X)
		x.expr(e.Lo)
		x.expr(e.Hi)
	case *ast.SliceLit:
		x.exprs(e.Elems)
	case *ast.Interp:
		x.exprs(e.Parts)
	case *ast.MapLit:
		x.exprs(e.Keys)
		x.exprs(e.Values)
	case *ast.Unary:
		x.expr(e.X)
	case *ast.Binary:
		x.expr(e.X)
		x.expr(e.Y)
	case *ast.Call:
		x.callee(e)
		x.expr(e.Fn)
		x.exprs(e.Args)
	case *ast.If:
		x.expr(e.Cond)
		x.expr(e.Then)
		x.expr(e.Else)
	case *ast.Match:
		x.expr(e.X)
		x.arms(e.Arms, e.X.TypeOf())
	case *ast.Select:
		x.selectCases(e.Cases)
	case *ast.Try:
		x.expr(e.X)
	case *ast.Catch:
		x.catch(e)
	}
}

func (x *index) callee(e *ast.Call) {
	switch fn := e.Fn.(type) {
	case *ast.Ident:
		x.add(occurrence{pos: fn.Pos, name: fn.Name, node: e})
	case *ast.Selector:
		x.add(occurrence{pos: fn.Pos, name: fn.Name, node: e})
	}
}

func (x *index) structLit(e *ast.StructLit) {
	x.add(occurrence{pos: e.Pos, name: e.Name, node: e})
	for _, t := range e.TypeArgs {
		x.typeName(t)
	}
	for _, f := range e.Fields {
		x.add(occurrence{pos: f.Pos, name: f.Name, node: f, parent: e.Type})
		if f.Value.Position() != f.Pos {
			x.expr(f.Value)
		}
	}
}

func (x *index) closure(e *ast.Closure) {
	var locals []*ast.Local
	if e.Func != nil {
		locals = e.Func.ParamLocals
	}
	x.params(e.Params, locals)
	x.typeName(e.Result)
	x.typeName(e.Error)
	x.expr(e.Body)
}

func (x *index) catch(e *ast.Catch) {
	x.expr(e.X)
	errType := e.X.TypeOf()
	if errType != nil && errType.Kind == types.ResultKind {
		errType = errType.Err
	}
	x.arms(e.Arms, errType)
	x.expr(e.Fallback)
}

func (x *index) arms(arms []*ast.Arm, scrutinee *types.Type) {
	for _, a := range arms {
		if a.Pattern.Test != nil {
			x.expr(a.Pattern.Test)
			x.expr(a.Body)
			continue
		}
		if a.Pattern.Local != nil {
			x.add(occurrence{pos: a.Pattern.Pos, name: a.Pattern.Name, node: a.Pattern, local: a.Pattern.Local})
			x.expr(a.Body)
			continue
		}
		x.add(occurrence{pos: a.Pattern.Pos, name: a.Pattern.Name, node: a.Pattern, parent: scrutinee})
		for _, b := range a.Pattern.Bindings {
			x.add(occurrence{pos: b.Pos, name: b.Name, node: b, local: b.Local})
		}
		x.expr(a.Body)
	}
}

func (x *index) selectCases(cases []*ast.SelectCase) {
	for _, c := range cases {
		if c.Name != "" {
			x.add(occurrence{pos: c.Pos, name: c.Name, node: c, local: c.Local})
		}
		x.expr(c.Chan)
		x.expr(c.Value)
		x.expr(c.Body)
	}
}
