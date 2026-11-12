package check

import (
	"fmt"

	"lowlang/internal/ast"
	"lowlang/internal/diag"
	"lowlang/internal/types"
)

type dropSource struct {
	decl     *ast.StructDecl
	file     *ast.File
	bound    map[string]*types.Type
	clone    bool
	abstract bool
}

type containment struct {
	elem  *types.Type
	where string
	pos   diag.Pos
}

func (c *Checker) recordStruct(d *ast.StructDecl, st *types.Type) {
	c.structFiles[st] = c.file
	if d.Drop == nil {
		return
	}
	c.drops[st] = &dropSource{decl: d, file: c.file, bound: c.bound, clone: d.TypeParams != nil, abstract: st.Abstract()}
}

func (c *Checker) droppable(t *types.Type) bool {
	if t == nil || t.Kind != types.StructKind || !c.laid {
		return false
	}
	if known, ok := c.droppables[t]; ok {
		return known
	}
	c.droppables[t] = false
	result := c.drops[t] != nil
	for _, f := range t.Fields {
		if c.droppable(f.Type) {
			result = true
		}
	}
	c.droppables[t] = result
	return result
}

func (c *Checker) owns(t *types.Type) bool {
	if t == nil {
		return false
	}
	if t.Kind == types.ResultKind {
		return c.droppable(t.Ok)
	}
	return c.droppable(t)
}

func (c *Checker) contain(elem *types.Type, where string, pos diag.Pos) {
	if !c.laid {
		c.containments = append(c.containments, containment{elem, where, pos})
		return
	}
	if c.droppable(elem) {
		c.fail(pos, "cannot store %s in %s; it has drop", elem, where)
	}
}

func (c *Checker) contained(t *ast.TypeName, where string) *types.Type {
	elem := c.resolve(t)
	c.contain(elem, where, t.Pos)
	return elem
}

func (c *Checker) checkContainments() {
	for _, k := range c.containments {
		c.contain(k.elem, k.where, k.pos)
	}
	c.containments = nil
}

func (c *Checker) detached(file *ast.File, bound map[string]*types.Type, abstract bool, run func()) {
	fn, savedFile, savedBound, scopes := c.fn, c.file, c.bound, c.scopes
	loops, deferring, closures, savedAbstract, dropping := c.loops, c.deferring, c.closures, c.abstract, c.dropping
	c.file, c.bound, c.scopes = file, bound, nil
	c.loops, c.deferring, c.closures, c.abstract, c.dropping = nil, false, nil, abstract, false
	run()
	c.fn, c.file, c.bound, c.scopes = fn, savedFile, savedBound, scopes
	c.loops, c.deferring, c.closures, c.abstract, c.dropping = loops, deferring, closures, savedAbstract, dropping
}

func (c *Checker) dropFunc(t *types.Type) *ast.Func {
	if fn, ok := c.dropFuncs[t]; ok {
		return fn
	}
	src := c.drops[t]
	body := src.decl.Drop
	if src.clone {
		body = ast.CloneStmt(body)
	}
	fn := &ast.Func{Name: fmt.Sprintf("%s..drop", t.Name), File: src.file, Pos: src.decl.Pos, RetType: types.Void}
	fn.Body = &ast.Block{Stmts: []ast.Stmt{body}, Pos: src.decl.Pos}
	for _, f := range t.Fields {
		fn.Params = append(fn.Params, &ast.Param{Name: f.Name, Pos: src.decl.Pos})
		fn.ParamTypes = append(fn.ParamTypes, f.Type)
	}
	c.dropFuncs[t] = fn
	c.detached(src.file, src.bound, src.abstract, func() {
		c.dropping = true
		c.body(fn)
	})
	if !src.abstract {
		src.file.Funcs = append(src.file.Funcs, fn)
	}
	return fn
}

func (c *Checker) checkDrops(files []*ast.File) {
	for _, file := range files {
		for _, d := range file.Structs {
			if d.Drop != nil {
				c.dropFunc(d.Type)
			}
		}
	}
}

func typedIdent(local *ast.Local, pos diag.Pos) *ast.Ident {
	id := &ast.Ident{Name: local.Name, Local: local, Pos: pos}
	id.Type = local.Type
	return id
}

func typedCall(target *ast.Func, args []ast.Expr, pos diag.Pos) *ast.Call {
	call := &ast.Call{Fn: &ast.Ident{Name: target.Name, Pos: pos}, Args: args, Target: target, Pos: pos}
	call.Type = types.Void
	return call
}

func (c *Checker) releaseFunc(t *types.Type) *ast.Func {
	if fn, ok := c.releaseFuncs[t]; ok {
		return fn
	}
	file := c.structFiles[t]
	fn := &ast.Func{Name: fmt.Sprintf("%s..release", t.Name), File: file, RetType: types.Void}
	self := &ast.Local{Name: "self", Type: t, Param: true, Owner: fn}
	fn.Params = []*ast.Param{{Name: "self"}}
	fn.ParamTypes = []*types.Type{t}
	fn.ParamLocals = []*ast.Local{self}
	c.releaseFuncs[t] = fn
	field := func(f *types.Field) ast.Expr {
		sel := &ast.Selector{X: typedIdent(self, diag.Pos{}), Name: f.Name, Field: f}
		sel.Type = f.Type
		return sel
	}
	var stmts []ast.Stmt
	if c.drops[t] != nil {
		var args []ast.Expr
		for _, f := range t.Fields {
			args = append(args, field(f))
		}
		stmts = append(stmts, &ast.ExprStmt{X: typedCall(c.dropFunc(t), args, diag.Pos{})})
	}
	for _, f := range t.Fields {
		if c.droppable(f.Type) {
			release := c.releaseFunc(f.Type)
			stmts = append(stmts, &ast.ExprStmt{X: typedCall(release, []ast.Expr{field(f)}, diag.Pos{})})
		}
	}
	fn.Body = &ast.Block{Stmts: stmts}
	fn.Body.Type = types.Void
	if !t.Abstract() {
		file.Funcs = append(file.Funcs, fn)
	}
	return fn
}

func guarded(flag *ast.Local, call *ast.Call) ast.Stmt {
	then := &ast.Block{Stmts: []ast.Stmt{&ast.ExprStmt{X: call}}}
	then.Type = types.Void
	branch := &ast.If{Cond: typedIdent(flag, call.Pos), Then: then, Pos: call.Pos}
	branch.Type = types.Void
	return &ast.ExprStmt{X: branch}
}

func (c *Checker) usingBinding(s *ast.Let, t *types.Type) {
	switch {
	case c.deferring:
		c.fail(s.Pos, "using cannot appear in a deferred statement")
	case c.dropping:
		c.fail(s.Pos, "using cannot appear in a drop")
	case t.Abstract():
		return
	case !c.droppable(t):
		c.fail(s.Pos, "%s has no drop; bind it with let", t)
	}
	c.flags++
	flag := c.declare(fmt.Sprintf("__live%d", c.flags), types.Bool, true)
	s.Local.Flag, s.Flag = flag, flag
	call := typedCall(c.releaseFunc(t), []ast.Expr{typedIdent(s.Local, s.Pos)}, s.Pos)
	s.Release = guarded(flag, call)
}

func (c *Checker) droppableAssign(s *ast.Assign, t *types.Type) {
	if t.Abstract() || !c.droppable(t) {
		return
	}
	call := typedCall(c.releaseFunc(t), []ast.Expr{s.Target}, s.Pos)
	if id, ok := s.Target.(*ast.Ident); ok && id.Local != nil && id.Local.Flag != nil {
		s.Release = guarded(id.Local.Flag, call)
		s.Flag = id.Local.Flag
		return
	}
	s.Release = &ast.ExprStmt{X: call}
}
