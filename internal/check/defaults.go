package check

import (
	"fmt"

	"lowlang/internal/ast"
	"lowlang/internal/types"
)

type fieldSource struct {
	decl     *ast.FieldDecl
	owner    string
	file     *ast.File
	bound    map[string]*types.Type
	clone    bool
	abstract bool
}

func (c *Checker) recordDefault(d *ast.StructDecl, st *types.Type, fd *ast.FieldDecl, f *types.Field) {
	if fd.Default == nil {
		return
	}
	c.fieldSources[f] = &fieldSource{
		decl:     fd,
		owner:    st.Name,
		file:     c.file,
		bound:    c.bound,
		clone:    d.TypeParams != nil,
		abstract: st.Abstract(),
	}
}

func (c *Checker) defaultOf(f *types.Field) ast.Expr {
	if e, ok := c.defaults[f]; ok {
		return e
	}
	src := c.fieldSources[f]
	if src == nil {
		return nil
	}
	if c.checkingDefault[f] {
		c.fail(src.decl.Default.Position(), "the default of %s.%s refers to itself", src.owner, f.Name)
	}
	c.checkingDefault[f] = true
	e := src.decl.Default
	if src.clone {
		e = ast.CloneExpr(e)
	}
	c.isolated(src, func() { c.expectType(e, f.Type) })
	c.defaults[f] = e
	return e
}

func (c *Checker) isolated(src *fieldSource, run func()) {
	fn, file, bound, scopes := c.fn, c.file, c.bound, c.scopes
	loops, deferring, closures, abstract := c.loops, c.deferring, c.closures, c.abstract
	c.fn = &ast.Func{Name: fmt.Sprintf("%s..%s", src.owner, src.decl.Name), File: src.file, RetType: types.Void}
	c.file, c.bound, c.scopes = src.file, src.bound, nil
	c.loops, c.deferring, c.closures, c.abstract = nil, false, nil, src.abstract
	c.pushScope()
	run()
	c.fn, c.file, c.bound, c.scopes = fn, file, bound, scopes
	c.loops, c.deferring, c.closures, c.abstract = loops, deferring, closures, abstract
}

func (c *Checker) checkDefaults(files []*ast.File) {
	for _, file := range files {
		c.file = file
		for _, d := range file.Structs {
			for _, f := range d.Type.Fields {
				c.defaultOf(f)
			}
		}
	}
}
