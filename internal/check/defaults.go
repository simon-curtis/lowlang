package check

import (
	"fmt"

	"lowlang/internal/ast"
	"lowlang/internal/types"
)

type fieldSource struct {
	declSource
	decl  *ast.FieldDecl
	owner string
}

func (c *Checker) recordDefault(d *ast.StructDecl, st *types.Type, fd *ast.FieldDecl, f *types.Field) {
	if fd.Default == nil {
		return
	}
	c.fieldSources[f] = &fieldSource{declSource: c.sourceOf(st, d.TypeParams != nil), decl: fd, owner: st.Name}
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
	next := src.context()
	next.fn = &ast.Func{Name: fmt.Sprintf("%s..%s", src.owner, src.decl.Name), File: src.file, RetType: types.Void}
	c.enter(next, func() {
		c.pushScope()
		c.expectType(e, f.Type)
	})
	c.defaults[f] = e
	return e
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
