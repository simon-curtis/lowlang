package ast

import (
	"fmt"
	"strings"

	"lowlang/internal/types"
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
	return strings.Join(typeTexts(list), ", ")
}

func typeTexts(list []*TypeName) []string {
	texts := make([]string, len(list))
	for i, t := range list {
		texts[i] = t.String()
	}
	return texts
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
	head, params, tail := f.parts(false)
	return fmt.Sprintf("%s%s%s", head, strings.Join(params, ", "), tail)
}

func (f *Func) Signature() string {
	head, params, tail := f.SignatureParts()
	return fmt.Sprintf("%s%s%s", head, strings.Join(params, ", "), tail)
}

func (f *Func) SignatureParts() (string, []string, string) {
	return f.Source().parts(true)
}

func (f *Func) parts(resolved bool) (string, []string, string) {
	params := make([]string, 0, len(f.Params)+1)
	for i, p := range f.Params {
		text := p.String()
		if resolved && i < len(f.ParamTypes) {
			text = types.ParamText(f.ParamTypes[i], p.Mut)
			text = fmt.Sprintf("%s: %s", p.Name, text)
		}
		params = append(params, text)
	}
	if f.Variadic {
		params = append(params, "...")
	}
	var b strings.Builder
	if f.Extern != nil {
		b.WriteString("extern ")
	}
	fmt.Fprintf(&b, "fn %s%s(", f.Name, TypeParams(f.TypeParams))
	tail := Results(f.Result, f.Error)
	if resolved && f.RetType != nil {
		tail = types.ReturnText(f.RetType)
	}
	return b.String(), params, fmt.Sprintf(")%s", tail)
}

func (k *Const) String() string {
	if t := k.typeText(); t != "" {
		return fmt.Sprintf("const %s: %s = %s", k.Name, t, k.Text)
	}
	return fmt.Sprintf("const %s = %s", k.Name, k.Text)
}

func (k *Const) typeText() string {
	if k.Folded != nil {
		return k.Folded.TypeOf().String()
	}
	return k.TypeName.String()
}

func (d *StructDecl) String() string {
	return d.render(false)
}

func (d *StructDecl) Interface() string {
	return d.render(true)
}

func (d *StructDecl) render(public bool) string {
	var b strings.Builder
	if d.C {
		b.WriteString("extern ")
	}
	if public {
		b.WriteString("pub ")
	}
	fmt.Fprintf(&b, "struct %s%s {\n", d.Name, TypeParams(d.TypeParams))
	hidden := false
	for _, f := range d.Fields {
		switch {
		case public && !f.Pub:
			hidden = true
		case public:
			writeDoc(&b, f.Doc)
			fmt.Fprintf(&b, "    pub %s: %s\n", f.Name, f.TypeName)
		default:
			fmt.Fprintf(&b, "    %s: %s\n", f.Name, f.TypeName)
		}
	}
	if hidden {
		b.WriteString("    ...\n")
	}
	b.WriteString("}")
	return b.String()
}

func (d *EnumDecl) String() string {
	return d.render(false)
}

func (d *EnumDecl) Interface() string {
	return d.render(true)
}

func (d *EnumDecl) render(public bool) string {
	var b strings.Builder
	if public {
		b.WriteString("pub ")
	}
	fmt.Fprintf(&b, "enum %s%s {\n", d.Name, TypeParams(d.TypeParams))
	for _, v := range d.Variants {
		if public {
			writeDoc(&b, v.Doc)
		}
		fmt.Fprintf(&b, "    %s\n", v)
	}
	b.WriteString("}")
	return b.String()
}

func writeDoc(b *strings.Builder, doc string) {
	for line := range strings.Lines(doc) {
		fmt.Fprintf(b, "%s\n", strings.TrimRight(fmt.Sprintf("    /// %s", line), " \n"))
	}
}

func (v *VariantDecl) Params() []string {
	return typeTexts(v.Fields)
}
