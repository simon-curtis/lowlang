package ast

import (
	"fmt"
	"strings"
)

func (t *TypeName) String() string {
	switch {
	case t == nil:
		return ""
	case t.Optional:
		return fmt.Sprintf("%s?", t.Elem)
	case t.Pointer:
		return fmt.Sprintf("*%s", t.Elem)
	case t.Slice:
		return fmt.Sprintf("[]%s", t.Elem)
	case t.Map:
		return fmt.Sprintf("[%s: %s]", t.Key, t.Elem)
	case t.Raw && t.Elem == nil:
		return "ptr"
	case t.Raw:
		return fmt.Sprintf("ptr[%s]", t.Elem)
	case t.Func:
		return t.funcString()
	}
	name := t.Name
	if t.Module != "" {
		name = fmt.Sprintf("%s.%s", t.Module, t.Name)
	}
	if len(t.Args) > 0 {
		name = fmt.Sprintf("%s[%s]", name, TypeNames(t.Args))
	}
	return name
}

func (t *TypeName) funcString() string {
	params := make([]string, len(t.Params))
	for i, p := range t.Params {
		params[i] = p.String()
		if i < len(t.Muts) && t.Muts[i] {
			params[i] = fmt.Sprintf("mut %s", params[i])
		}
	}
	prefix := "fn"
	if t.CFunc {
		prefix = "extern fn"
	}
	return fmt.Sprintf("%s(%s)%s", prefix, strings.Join(params, ", "), Results(t.Result, t.Error))
}

func TypeNames(list []*TypeName) string {
	texts := make([]string, len(list))
	for i, t := range list {
		texts[i] = t.String()
	}
	return strings.Join(texts, ", ")
}

func Results(result, errName *TypeName) string {
	var b strings.Builder
	if result != nil {
		b.WriteString(" -> ")
		b.WriteString(result.String())
	}
	if errName != nil {
		b.WriteString(" ! ")
		b.WriteString(errName.String())
	}
	return b.String()
}

func (p *TypeParam) String() string {
	if len(p.Constraints) == 0 {
		return p.Name
	}
	return fmt.Sprintf("%s: %s", p.Name, strings.Join(p.Constraints, " + "))
}

func TypeParams(params []*TypeParam) string {
	if len(params) == 0 {
		return ""
	}
	texts := make([]string, len(params))
	for i, p := range params {
		texts[i] = p.String()
	}
	return fmt.Sprintf("[%s]", strings.Join(texts, ", "))
}

func (v *VariantDecl) String() string {
	if len(v.Fields) == 0 {
		return v.Name
	}
	return fmt.Sprintf("%s(%s)", v.Name, TypeNames(v.Fields))
}

func (p *Param) String() string {
	if p.Mut {
		return fmt.Sprintf("mut %s: %s", p.Name, p.TypeName)
	}
	return fmt.Sprintf("%s: %s", p.Name, p.TypeName)
}

func (f *Func) String() string {
	params := make([]string, 0, len(f.Params)+1)
	for _, p := range f.Params {
		params = append(params, p.String())
	}
	if f.Variadic {
		params = append(params, "...")
	}
	var b strings.Builder
	if f.Extern != nil {
		b.WriteString("extern ")
	}
	b.WriteString("fn ")
	b.WriteString(f.Name)
	b.WriteString(TypeParams(f.TypeParams))
	b.WriteString("(")
	b.WriteString(strings.Join(params, ", "))
	b.WriteString(")")
	b.WriteString(Results(f.Result, f.Error))
	return b.String()
}
