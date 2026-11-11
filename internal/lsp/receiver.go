package lsp

import (
	"fmt"
	"strings"

	"lowlang/internal/ast"
	"lowlang/internal/types"
)

type segment struct {
	name     string
	suffixes []byte
}

func (s segment) plain() bool {
	return len(s.suffixes) == 0
}

func skipGroup(src string, end int) (int, byte) {
	shut := src[end-1]
	open := byte('(')
	if shut == ']' {
		open = '['
	}
	depth := 0
	for i := end - 1; i >= 0; i-- {
		switch src[i] {
		case shut:
			depth++
		case open:
			depth--
			if depth == 0 {
				return i, open
			}
		}
	}
	return -1, open
}

func segmentsBefore(src string, end int) ([]segment, bool) {
	var segments []segment
	for {
		var suffixes []byte
		for end > 0 {
			switch src[end-1] {
			case ')', ']':
				start, open := skipGroup(src, end)
				if start < 0 {
					return nil, false
				}
				suffixes = append([]byte{open}, suffixes...)
				end = start
				continue
			case '?':
				suffixes = append([]byte{'?'}, suffixes...)
				end--
				continue
			}
			break
		}
		start := end
		for start > 0 && isWord(src[start-1]) {
			start--
		}
		if start == end {
			return nil, false
		}
		segments = append([]segment{{name: src[start:end], suffixes: suffixes}}, segments...)
		if start == 0 || src[start-1] != '.' {
			return segments, true
		}
		end = start - 1
	}
}

func applySuffix(t *types.Type, suffix byte) *types.Type {
	if t == nil {
		return nil
	}
	switch {
	case suffix == '(' && t.Kind == types.FuncKind:
		return t.Ret
	case suffix == '[' && t.Kind == types.SliceKind:
		return t.Elem
	case suffix == '[' && t.Kind == types.MapKind:
		return types.NewOptional(t.Elem)
	case suffix == '[' && t.Kind == types.StrKind:
		return types.U8
	case suffix == '?' && t.Kind == types.ResultKind:
		return t.Ok
	case suffix == '?' && t.Optional:
		return t.Elem
	}
	return nil
}

func (c *cursor) function(module, name string, public bool) *ast.Func {
	for _, f := range c.files(module) {
		for _, fn := range f.Funcs {
			if fn.Name == name && fn.Template == nil && fn.Closure == nil && (fn.Pub || !public) {
				return fn
			}
		}
		for _, e := range f.Externs {
			for _, fn := range e.Funcs {
				if fn.Name == name && (fn.Pub || !public) {
					return fn
				}
			}
		}
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

func fits(tn *ast.TypeName, t *types.Type, params map[string]bool) bool {
	switch {
	case tn == nil || t == nil:
		return false
	case tn.Name != "" && params[tn.Name] && len(tn.Args) == 0:
		return true
	case tn.Slice:
		return t.Kind == types.SliceKind && fits(tn.Elem, t.Elem, params)
	case tn.Map:
		return t.Kind == types.MapKind && fits(tn.Key, t.Key, params) && fits(tn.Elem, t.Elem, params)
	case tn.Pointer:
		return t.Kind == types.PointerKind && fits(tn.Elem, t.Elem, params)
	case tn.Optional:
		return t.Optional && fits(tn.Elem, t.Elem, params)
	case len(tn.Args) > 0:
		base, _, _ := strings.Cut(t.String(), "[")
		return base == tn.Name || strings.HasSuffix(base, fmt.Sprintf(".%s", tn.Name))
	}
	return t.String() == typeNameString(tn)
}

func takes(fn *ast.Func, t *types.Type) bool {
	if len(fn.Params) == 0 {
		return false
	}
	if len(fn.TypeParams) > 0 {
		return fits(fn.Params[0].TypeName, t, typeParamNames(fn.TypeParams))
	}
	return len(fn.ParamTypes) > 0 && identical(fn.ParamTypes[0], t)
}

func receiverModule(t *types.Type) string {
	if t.Kind == types.PointerKind {
		t = t.Elem
	}
	if t.Origin != nil {
		t = t.Origin
	}
	return t.Module
}

func (c *cursor) method(t *types.Type, name string) *ast.Func {
	if fn := c.function(c.file.Module, name, false); fn != nil {
		return fn
	}
	if m := receiverModule(t); m != c.file.Module {
		return c.function(m, name, true)
	}
	return nil
}

func (c *cursor) moduleMethods(module string, t *types.Type, public bool) {
	for _, f := range c.files(module) {
		for _, fn := range f.Funcs {
			if fn.Template == nil && fn.Closure == nil && (fn.Pub || !public) && takes(fn, t) {
				c.add(fn.Name, completionMethod, signature(fn))
			}
		}
		for _, e := range f.Externs {
			for _, fn := range e.Funcs {
				if (fn.Pub || !public) && takes(fn, t) {
					c.add(fn.Name, completionMethod, signature(fn))
				}
			}
		}
	}
}

func (c *cursor) methods(t *types.Type) {
	c.moduleMethods(c.file.Module, t, false)
	if m := receiverModule(t); m != c.file.Module {
		c.moduleMethods(m, t, true)
	}
	for _, l := range c.scope() {
		if l.Type != nil && l.Type.Kind == types.FuncKind && len(l.Type.Params) > 0 && identical(l.Type.Params[0], t) {
			c.add(l.Name, completionMethod, l.Type.String())
		}
	}
	switch t.Kind {
	case types.SliceKind:
		c.add("push", completionMethod, "builtin")
		c.add("pop", completionMethod, "builtin")
		c.add("truncate", completionMethod, "builtin")
	case types.MapKind:
		c.add("delete", completionMethod, "builtin")
	case types.ChanKind:
		c.add("close", completionMethod, "builtin")
	}
}

func (c *cursor) typeOf(segments []segment) *types.Type {
	t, suffixes, rest := c.head(segments)
	for {
		for _, s := range suffixes {
			t = applySuffix(t, s)
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
		if fn == nil || !takes(fn, t) {
			return nil
		}
		t, suffixes = called(fn, next.suffixes)
		rest = rest[1:]
	}
}
