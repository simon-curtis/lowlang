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

func typeParamNames(params []*ast.TypeParam) map[string]bool {
	names := map[string]bool{}
	for _, p := range params {
		names[p.Name] = true
	}
	return names
}

func (c *cursor) fits(file *ast.File, tn *ast.TypeName, t *types.Type, params map[string]bool) bool {
	switch {
	case tn == nil || t == nil:
		return false
	case tn.Name != "" && params[tn.Name] && len(tn.Args) == 0:
		return true
	case tn.Slice:
		return t.Kind == types.SliceKind && c.fits(file, tn.Elem, t.Elem, params)
	case tn.Map:
		return t.Kind == types.MapKind && c.fits(file, tn.Key, t.Key, params) && c.fits(file, tn.Elem, t.Elem, params)
	case tn.Pointer:
		return t.Kind == types.PointerKind && c.fits(file, tn.Elem, t.Elem, params)
	case tn.Optional:
		return t.Optional && c.fits(file, tn.Elem, t.Elem, params)
	}
	if decl := c.declType(file, tn); decl != nil {
		return sameType(decl, t)
	}
	return len(tn.Args) == 0 && t.String() == tn.String()
}

func (c *cursor) takes(fn *ast.Func, t *types.Type) bool {
	if len(fn.Params) == 0 {
		return false
	}
	if len(fn.TypeParams) > 0 {
		return c.fits(fn.File, fn.Params[0].TypeName, t, typeParamNames(fn.TypeParams))
	}
	return len(fn.ParamTypes) > 0 && identical(fn.ParamTypes[0], t)
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
			if (fn.Pub || !public) && c.takes(fn, t) {
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
		if fn == nil || !c.takes(fn, t) {
			return nil
		}
		t, suffixes = called(fn, next.suffixes)
		rest = rest[1:]
	}
}
