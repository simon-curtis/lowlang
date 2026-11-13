package main

import (
	"fmt"
	"slices"
	"strings"

	"lowlang/internal/ast"
	"lowlang/internal/builtin"
	"lowlang/internal/lexer"
	"lowlang/internal/types"
)

func typed(name string, t *types.Type) string {
	if t == nil {
		return ""
	}
	return fmt.Sprintf(" %s=%s", name, lexer.Quote(t.String()))
}

func typeList(name string, list []*types.Type) string {
	if len(list) == 0 {
		return ""
	}
	texts := make([]string, len(list))
	for i, t := range list {
		texts[i] = t.String()
	}
	return fmt.Sprintf(" %s=%s", name, lexer.Quote(strings.Join(texts, ", ")))
}

func localText(l *ast.Local) string {
	var b strings.Builder
	b.WriteString(l.Name)
	if l.Type != nil {
		fmt.Fprintf(&b, " %s", l.Type)
	}
	b.WriteString(flag("mutable", l.Mutable))
	b.WriteString(flag("param", l.Param))
	b.WriteString(flag("ref", l.Ref))
	b.WriteString(flag("addressed", l.Addressed))
	b.WriteString(flag("captured", l.Captured))
	b.WriteString(flag("assigned", l.Assigned))
	if l.Owner != nil {
		fmt.Fprintf(&b, " owner=%s", funcName(l.Owner))
	}
	if l.Flag != nil {
		fmt.Fprintf(&b, " flag=%s", l.Flag.Name)
	}
	return b.String()
}

func local(name string, l *ast.Local) string {
	if l == nil {
		return ""
	}
	return fmt.Sprintf(" %s=%s", name, lexer.Quote(localText(l)))
}

func funcName(f *ast.Func) string {
	var b strings.Builder
	if f.File != nil && f.File.Module != "" {
		fmt.Fprintf(&b, "%s.", f.File.Module)
	}
	b.WriteString(f.Name)
	if len(f.TypeArgs) > 0 {
		texts := make([]string, len(f.TypeArgs))
		for i, t := range f.TypeArgs {
			texts[i] = t.String()
		}
		fmt.Fprintf(&b, "[%s]", strings.Join(texts, ", "))
	}
	return b.String()
}

func funcRef(name string, f *ast.Func) string {
	if f == nil {
		return ""
	}
	return fmt.Sprintf(" %s=%s", name, lexer.Quote(funcName(f)))
}

func constRef(k *ast.Const) string {
	if k == nil {
		return ""
	}
	return fmt.Sprintf(" const=%s", k.Name)
}

func variantRef(v *types.Variant) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf(" variant=%s#%d", v.Name, v.Tag)
}

func fieldRef(name string, f *types.Field) string {
	if f == nil {
		return ""
	}
	return fmt.Sprintf(" %s=%s@%d", name, f.Name, f.Bytes)
}

func builtinRef(name string, id builtin.ID) string {
	if id == builtin.None {
		return ""
	}
	return fmt.Sprintf(" %s=%d", name, id)
}

func number(name string, n int64) string {
	if n == 0 {
		return ""
	}
	return fmt.Sprintf(" %s=%d", name, n)
}

func bound(types map[string]*types.Type) string {
	if len(types) == 0 {
		return ""
	}
	names := make([]string, 0, len(types))
	for name := range types {
		names = append(names, name)
	}
	slices.Sort(names)
	texts := make([]string, len(names))
	for i, name := range names {
		texts[i] = fmt.Sprintf("%s=%s", name, types[name])
	}
	return fmt.Sprintf(" bound=%s", lexer.Quote(strings.Join(texts, ", ")))
}

func (d astDumper) layout(depth int, t *types.Type) {
	if t == nil {
		return
	}
	d.line(depth, "", fmt.Sprintf("Layout %s bytes=%d align=%d size=%d", lexer.Quote(t.String()), t.Bytes, t.Align, t.Size()))
	for _, f := range t.Fields {
		d.line(depth+1, "", fmt.Sprintf("field %s@%d%s%s", f.Name, f.Bytes, flag("pub", f.Pub), typed("type", f.Type)))
	}
	for _, v := range t.Variants {
		offsets := make([]string, len(v.Offsets))
		for i, offset := range v.Offsets {
			offsets[i] = fmt.Sprintf("%d", offset)
		}
		head := fmt.Sprintf("variant %s#%d index=%d%s%s%s", v.Name, v.Tag, v.Index, typeList("fields", v.Fields), named("labels", strings.Join(v.Labels, ",")), named("offsets", strings.Join(offsets, ",")))
		d.line(depth+1, "", head)
	}
}

func (d astDumper) locals(depth int, label string, list []*ast.Local) {
	for _, l := range list {
		d.line(depth, label, localText(l))
	}
}
