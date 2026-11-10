package lsp

import (
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

func (c *cursor) typeOf(segments []segment) *types.Type {
	t, suffixes, rest := c.head(segments)
	for {
		for _, s := range suffixes {
			t = applySuffix(t, s)
		}
		if t == nil || len(rest) == 0 {
			return t
		}
		f := types.StructOf(t).Field(rest[0].name)
		if f == nil {
			return nil
		}
		t, suffixes, rest = f.Type, rest[0].suffixes, rest[1:]
	}
}
