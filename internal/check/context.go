package check

import (
	"lowlang/internal/ast"
	"lowlang/internal/types"
)

type funcContext struct {
	fn        *ast.Func
	file      *ast.File
	bound     map[string]*types.Type
	scopes    []map[string]*ast.Local
	loops     []*loop
	closures  []*ast.Closure
	abstract  bool
	deferring bool
	dropping  bool
}

func (c *Checker) enter(next funcContext, run func()) {
	saved := c.funcContext
	c.funcContext = next
	run()
	c.funcContext = saved
}

func (c *Checker) within(file *ast.File, bound map[string]*types.Type, run func()) {
	next := c.funcContext
	next.file, next.bound = file, bound
	c.enter(next, run)
}

type declSource struct {
	file     *ast.File
	bound    map[string]*types.Type
	clone    bool
	abstract bool
}

func (c *Checker) sourceOf(st *types.Type, generic bool) declSource {
	return declSource{file: c.file, bound: c.bound, clone: generic, abstract: st.Abstract()}
}

func (s declSource) context() funcContext {
	return funcContext{file: s.file, bound: s.bound, abstract: s.abstract}
}
