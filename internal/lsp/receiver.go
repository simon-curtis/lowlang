package lsp

import (
	"lowlang/internal/ast"
	"lowlang/internal/check"
	"lowlang/internal/types"
)

func (c *cursor) applySuffix(t *types.Type, suffix byte) *types.Type {
	if t == nil {
		return nil
	}
	switch {
	case suffix == '(' && t.Kind == types.FuncKind:
		return t.Ret
	case suffix == '[':
		return check.IndexType(c.unit.Types, t)
	case suffix == '?' && t.Kind == types.ResultKind:
		return t.Ok
	}
	return nil
}

func called(fn *ast.Func, suffixes []byte) (*types.Type, []byte) {
	if fn == nil || len(suffixes) == 0 || suffixes[0] != '(' {
		return nil, nil
	}
	return fn.RetType, suffixes[1:]
}

func (c *cursor) head(segments []segment) (*types.Type, []byte, []segment) {
	first := segments[0]
	if local := c.local(first.name); local != nil {
		return local.Type, first.suffixes, segments[1:]
	}
	if path, ok := c.file.Imported(first.name); ok && first.plain() && len(segments) > 1 {
		fn := c.function(path, segments[1].name, true)
		t, rest := called(fn, segments[1].suffixes)
		return t, rest, segments[2:]
	}
	fn := c.function(c.file.Module, first.name, false)
	t, rest := called(fn, first.suffixes)
	return t, rest, segments[1:]
}

func identical(a, b *types.Type) bool {
	return a != nil && b != nil && (a == b || a.String() == b.String())
}

func (c *cursor) method(t *types.Type, name string) *ast.Func {
	if fn := c.function(c.file.Module, name, false); fn != nil {
		return fn
	}
	if module, foreign := check.ReceiverModule(t, c.file.Module); foreign {
		return c.function(module, name, true)
	}
	return nil
}

func (c *cursor) moduleMethods(module string, t *types.Type, public bool) {
	for _, f := range c.moduleFiles(module) {
		for fn := range f.Declared() {
			if (fn.Pub || !public) && check.TakesFirst(fn, t) {
				c.add(fn.Name, completionMethod, fn.Signature())
			}
		}
	}
}

func (c *cursor) methods(t *types.Type) {
	c.moduleMethods(c.file.Module, t, false)
	if module, foreign := check.ReceiverModule(t, c.file.Module); foreign {
		c.moduleMethods(module, t, true)
	}
	for _, l := range c.scope() {
		if l.Type != nil && l.Type.Kind == types.FuncKind && len(l.Type.Params) > 0 && identical(l.Type.Params[0], t) {
			c.add(l.Name, completionMethod, l.Type.String())
		}
	}
	for _, b := range check.Builtins() {
		if b.Kind == check.FunctionBuiltin && b.Receives(t.Kind) {
			c.add(b.Name, completionMethod, "builtin")
		}
	}
}

func (c *cursor) typeOf(segments []segment) *types.Type {
	t, suffixes, rest := c.head(segments)
	for {
		for _, s := range suffixes {
			t = c.applySuffix(t, s)
		}
		if t == nil || len(rest) == 0 {
			return t
		}
		next := rest[0]
		if f := types.StructOf(t).Field(next.name); f != nil {
			t, suffixes, rest = f.Type, next.suffixes, rest[1:]
			continue
		}
		fn := c.method(t, next.name)
		if fn == nil || !check.TakesFirst(fn, t) {
			return nil
		}
		t, suffixes = called(fn, next.suffixes)
		rest = rest[1:]
	}
}
