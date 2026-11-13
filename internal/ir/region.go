package ir

import "lowlang/internal/ast"

func (b *builder) region(e *ast.Region) []*Value {
	handle := b.runtime(rtRegionEnter)
	words := b.block(e.Body)
	b.runtime(rtRegionLeave, handle)
	if e.Copy != nil {
		words = b.duplicate(e.Copy, words)
	}
	b.runtime(rtRegionFree, handle)
	return words
}

func (b *builder) duplicate(fn *ast.Func, words []*Value) []*Value {
	path := b.emptySlice()
	args := append([]*Value{path}, words...)
	return b.invoke(b.funcs[fn], args)
}
