package lsp

import (
	"fmt"
	"strings"

	"lowlang/internal/ast"
	"lowlang/internal/compiler"
	"lowlang/internal/lexer"
	"lowlang/internal/types"
)

type target struct {
	path string
	pos  lexer.Pos
	name string
}

type query struct {
	unit  *compiler.Unit
	index *index
	occ   *occurrence
}

func (q *query) callee(call *ast.Call) *query {
	occ := *q.occ
	occ.node = call.Fn
	return &query{unit: q.unit, index: q.index, occ: &occ}
}

func (q *query) file() *ast.File {
	return q.index.file
}

func (q *query) files() []*ast.File {
	var files []*ast.File
	for _, f := range q.unit.Files {
		if f.Path != "" {
			files = append(files, f)
		}
	}
	return files
}

func (q *query) module(alias string) (string, bool) {
	if alias == "" {
		return q.file().Module, true
	}
	return q.file().Imported(alias)
}

func (q *query) typeDecl(alias, name string) any {
	module, ok := q.module(alias)
	if !ok {
		return nil
	}
	for _, f := range q.files() {
		if f.Module != module {
			continue
		}
		for _, d := range f.Structs {
			if d.Name == name {
				return d
			}
		}
		for _, d := range f.Enums {
			if d.Name == name {
				return d
			}
		}
	}
	return nil
}

func sameType(decl, t *types.Type) bool {
	if decl == nil || t == nil {
		return false
	}
	base, _, _ := strings.Cut(t.Name, "[")
	return decl == t || decl == t.Origin || (decl.Name == base && decl.Module == t.Module)
}

func (q *query) structDecl(t *types.Type) *ast.StructDecl {
	if t == nil {
		return nil
	}
	s := types.StructOf(t)
	for _, f := range q.files() {
		for _, d := range f.Structs {
			if sameType(d.Type, s) {
				return d
			}
		}
	}
	return nil
}

func (q *query) variant(t *types.Type, name string) (*ast.EnumDecl, *ast.VariantDecl) {
	var fallback *ast.EnumDecl
	var fallbackVariant *ast.VariantDecl
	for _, f := range q.files() {
		for _, d := range f.Enums {
			for _, v := range d.Variants {
				if v.Name != name {
					continue
				}
				if sameType(d.Type, t) {
					return d, v
				}
				if fallback == nil {
					fallback, fallbackVariant = d, v
				}
			}
		}
	}
	return fallback, fallbackVariant
}

func (q *query) moduleFile(path string) *ast.File {
	for _, f := range q.files() {
		if f.Module == path {
			return f
		}
	}
	return nil
}

func (q *query) moduleTarget(path string) (target, bool) {
	f := q.moduleFile(path)
	if f == nil {
		return target{}, false
	}
	return target{path: f.Path, pos: lexer.Pos{Line: 1, Col: 1}}, true
}

func funcTarget(f *ast.Func) (target, bool) {
	if f.Template != nil {
		f = f.Template
	}
	if f.File == nil || f.File.Path == "" {
		return target{}, false
	}
	return target{path: f.File.Path, pos: f.Pos, name: f.Name}, true
}

func declTarget(decl any) (target, bool) {
	switch d := decl.(type) {
	case *ast.StructDecl:
		return target{path: d.File.Path, pos: d.Pos, name: d.Name}, true
	case *ast.EnumDecl:
		return target{path: d.File.Path, pos: d.Pos, name: d.Name}, true
	}
	return target{}, false
}

func (q *query) localTarget(l *ast.Local) (target, bool) {
	d := q.index.declaration(l)
	if d == nil {
		return target{}, false
	}
	return target{path: q.file().Path, pos: d.pos}, true
}

func (q *query) fieldTarget(t *types.Type, name string) (target, bool) {
	d := q.structDecl(t)
	if d == nil {
		return target{}, false
	}
	for _, f := range d.Fields {
		if f.Name == name {
			return target{path: d.File.Path, pos: f.Pos}, true
		}
	}
	return target{}, false
}

func (q *query) variantTarget(t *types.Type, name string) (target, bool) {
	d, v := q.variant(t, name)
	if d == nil {
		return target{}, false
	}
	return target{path: d.File.Path, pos: v.Pos}, true
}

func (q *query) nameTarget(name string) (target, bool) {
	if decl := q.typeDecl("", name); decl != nil {
		return declTarget(decl)
	}
	if path, ok := q.file().Imported(name); ok {
		return q.moduleTarget(path)
	}
	return target{}, false
}

func (q *query) definition() (target, bool) {
	self := target{path: q.file().Path, pos: q.occ.pos}
	switch n := q.occ.node.(type) {
	case *ast.Call:
		switch {
		case n.Target != nil:
			return funcTarget(n.Target)
		case n.Variant != nil:
			return q.variantTarget(n.TypeOf(), n.Variant.Name)
		}
		return q.callee(n).definition()
	case *ast.Ident:
		switch {
		case n.Local != nil:
			return q.localTarget(n.Local)
		case n.Func != nil:
			return funcTarget(n.Func)
		case n.Variant != nil:
			return q.variantTarget(n.TypeOf(), n.Name)
		}
		return q.nameTarget(n.Name)
	case *ast.Selector:
		switch {
		case n.Func != nil:
			return funcTarget(n.Func)
		case n.Field != nil:
			return q.fieldTarget(n.X.TypeOf(), n.Name)
		case n.Variant != nil:
			return q.variantTarget(n.TypeOf(), n.Name)
		}
	case *ast.StructLit:
		return declTarget(q.typeDecl(n.Module, n.Name))
	case *ast.FieldInit:
		return q.fieldTarget(q.occ.parent, n.Name)
	case *ast.TypeName:
		return declTarget(q.typeDecl(n.Module, n.Name))
	case moduleRef:
		if path, ok := q.file().Imported(n.alias); ok {
			return q.moduleTarget(path)
		}
	case *ast.Pattern:
		if n.Local != nil {
			return q.localTarget(n.Local)
		}
		return q.variantTarget(q.occ.parent, n.Name)
	case *ast.Binding:
		return q.localTarget(n.Local)
	case *ast.Import:
		return q.moduleTarget(n.Path)
	case *ast.Func, *ast.Let, *ast.Param, *ast.ForIn, *ast.SelectCase, *ast.StructDecl, *ast.EnumDecl, *ast.FieldDecl, *ast.VariantDecl, *ast.TypeParam:
		return self, true
	}
	return target{}, false
}

func (q *query) hover() string {
	switch n := q.occ.node.(type) {
	case *ast.Call:
		switch {
		case n.Target != nil:
			return signature(n.Target)
		case n.Variant != nil:
			return variantText(n.TypeOf(), n.Variant)
		}
		if text := builtinSignature(n); text != "" {
			return text
		}
		return q.callee(n).hover()
	case *ast.Ident:
		return q.identHover(n)
	case *ast.Selector:
		switch {
		case n.Func != nil:
			return signature(n.Func)
		case n.Field != nil:
			return fmt.Sprintf("%s: %s", n.Name, n.Field.Type)
		case n.Variant != nil:
			return variantText(n.TypeOf(), n.Variant)
		}
		return typed(n.Name, n.TypeOf())
	case *ast.StructLit:
		return declText(q.typeDecl(n.Module, n.Name))
	case *ast.FieldInit:
		if n.Field != nil {
			return fmt.Sprintf("%s: %s", n.Name, n.Field.Type)
		}
	case *ast.TypeName:
		if decl := q.typeDecl(n.Module, n.Name); decl != nil {
			return declText(decl)
		}
		return typeNameString(n)
	case moduleRef:
		if path, ok := q.file().Imported(n.alias); ok {
			return fmt.Sprintf("import %s", path)
		}
	case *ast.Import:
		return fmt.Sprintf("import %s", n.Path)
	case *ast.Pattern:
		if n.Local != nil {
			return q.localText(n.Local)
		}
		if n.Variant != nil {
			return variantText(q.occ.parent, n.Variant)
		}
	case *ast.Func:
		return signature(n)
	case *ast.StructDecl, *ast.EnumDecl:
		return declText(n)
	case *ast.FieldDecl:
		return fmt.Sprintf("%s: %s", n.Name, typeNameString(n.TypeName))
	case *ast.VariantDecl:
		return variantDeclText(n)
	case *ast.TypeParam:
		return typeParamText(n)
	}
	if q.occ.local != nil {
		return q.localText(q.occ.local)
	}
	return ""
}

func (q *query) identHover(n *ast.Ident) string {
	switch {
	case n.Local != nil:
		return q.localText(n.Local)
	case n.Func != nil:
		return signature(n.Func)
	case n.Variant != nil:
		return variantText(n.TypeOf(), n.Variant)
	}
	if decl := q.typeDecl("", n.Name); decl != nil {
		return declText(decl)
	}
	if path, ok := q.file().Imported(n.Name); ok {
		return fmt.Sprintf("import %s", path)
	}
	return typed(n.Name, n.TypeOf())
}

func typed(name string, t *types.Type) string {
	if t == nil {
		return ""
	}
	return fmt.Sprintf("%s: %s", name, t)
}

func (q *query) using(l *ast.Local) bool {
	d := q.index.declaration(l)
	if d == nil {
		return false
	}
	let, ok := d.node.(*ast.Let)
	return ok && let.Using
}

func (q *query) localText(l *ast.Local) string {
	keyword := "let "
	switch {
	case q.using(l) && l.Mutable:
		keyword = "using var "
	case q.using(l):
		keyword = "using "
	case l.Param && l.Ref:
		keyword = "mut "
	case l.Param:
		keyword = ""
	case l.Mutable:
		keyword = "var "
	}
	if l.Type == nil {
		return fmt.Sprintf("%s%s", keyword, l.Name)
	}
	return fmt.Sprintf("%s%s: %s", keyword, l.Name, l.Type)
}

func variantText(enum *types.Type, v *types.Variant) string {
	fields := make([]string, len(v.Fields))
	for i, f := range v.Fields {
		fields[i] = f.String()
	}
	name := v.Name
	if enum != nil && enum.Kind == types.EnumKind {
		name = fmt.Sprintf("%s.%s", enum.Name, v.Name)
	}
	if len(fields) == 0 {
		return name
	}
	return fmt.Sprintf("%s(%s)", name, strings.Join(fields, ", "))
}

func variantDeclText(v *ast.VariantDecl) string {
	if len(v.Fields) == 0 {
		return v.Name
	}
	fields := make([]string, len(v.Fields))
	for i, f := range v.Fields {
		fields[i] = typeNameString(f)
	}
	return fmt.Sprintf("%s(%s)", v.Name, strings.Join(fields, ", "))
}

func typeParamText(p *ast.TypeParam) string {
	if len(p.Constraints) == 0 {
		return p.Name
	}
	return fmt.Sprintf("%s: %s", p.Name, strings.Join(p.Constraints, " + "))
}

func typeParamsText(params []*ast.TypeParam) string {
	if len(params) == 0 {
		return ""
	}
	texts := make([]string, len(params))
	for i, p := range params {
		texts[i] = typeParamText(p)
	}
	return fmt.Sprintf("[%s]", strings.Join(texts, ", "))
}

func signature(f *ast.Func) string {
	head, params, tail := signatureParts(f)
	return fmt.Sprintf("%s%s%s", head, strings.Join(params, ", "), tail)
}

func signatureParts(f *ast.Func) (string, []string, string) {
	if f.Template != nil {
		f = f.Template
	}
	params := make([]string, 0, len(f.Params)+1)
	for i, p := range f.Params {
		t := typeNameString(p.TypeName)
		if i < len(f.ParamTypes) {
			t = f.ParamTypes[i].String()
		}
		prefix := ""
		if p.Mut {
			prefix = "mut "
		}
		params = append(params, fmt.Sprintf("%s%s: %s", prefix, p.Name, t))
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
	b.WriteString(typeParamsText(f.TypeParams))
	b.WriteString("(")
	tail := fmt.Sprintf(")%s", resultText(f))
	return b.String(), params, tail
}

func resultText(f *ast.Func) string {
	t := f.RetType
	if t == nil {
		return resultNames(f.Result, f.Error)
	}
	switch {
	case t == types.Void:
		return ""
	case t.Kind == types.ResultKind && t.Ok == types.Void:
		return fmt.Sprintf(" ! %s", t.Err)
	case t.Kind == types.ResultKind:
		return fmt.Sprintf(" -> %s ! %s", t.Ok, t.Err)
	}
	return fmt.Sprintf(" -> %s", t)
}

func resultNames(result, errName *ast.TypeName) string {
	var b strings.Builder
	if result != nil {
		b.WriteString(" -> ")
		b.WriteString(typeNameString(result))
	}
	if errName != nil {
		b.WriteString(" ! ")
		b.WriteString(typeNameString(errName))
	}
	return b.String()
}

func declText(decl any) string {
	var b strings.Builder
	switch d := decl.(type) {
	case *ast.StructDecl:
		if d.C {
			b.WriteString("extern ")
		}
		b.WriteString(fmt.Sprintf("struct %s%s {\n", d.Name, typeParamsText(d.TypeParams)))
		for _, f := range d.Fields {
			b.WriteString(fmt.Sprintf("    %s: %s\n", f.Name, typeNameString(f.TypeName)))
		}
		b.WriteString("}")
	case *ast.EnumDecl:
		b.WriteString(fmt.Sprintf("enum %s%s {\n", d.Name, typeParamsText(d.TypeParams)))
		for _, v := range d.Variants {
			b.WriteString(fmt.Sprintf("    %s\n", variantDeclText(v)))
		}
		b.WriteString("}")
	}
	return b.String()
}

func typeNamesString(list []*ast.TypeName) string {
	texts := make([]string, len(list))
	for i, t := range list {
		texts[i] = typeNameString(t)
	}
	return strings.Join(texts, ", ")
}

func typeNameString(t *ast.TypeName) string {
	switch {
	case t == nil:
		return ""
	case t.Optional:
		return fmt.Sprintf("%s?", typeNameString(t.Elem))
	case t.Pointer:
		return fmt.Sprintf("*%s", typeNameString(t.Elem))
	case t.Slice:
		return fmt.Sprintf("[]%s", typeNameString(t.Elem))
	case t.Map:
		return fmt.Sprintf("[%s: %s]", typeNameString(t.Key), typeNameString(t.Elem))
	case t.Raw && t.Elem == nil:
		return "ptr"
	case t.Raw:
		return fmt.Sprintf("ptr[%s]", typeNameString(t.Elem))
	case t.Func:
		return funcTypeString(t)
	}
	name := t.Name
	if t.Module != "" {
		name = fmt.Sprintf("%s.%s", t.Module, t.Name)
	}
	if len(t.Args) > 0 {
		name = fmt.Sprintf("%s[%s]", name, typeNamesString(t.Args))
	}
	return name
}

func funcTypeString(t *ast.TypeName) string {
	params := make([]string, len(t.Params))
	for i, p := range t.Params {
		params[i] = typeNameString(p)
		if i < len(t.Muts) && t.Muts[i] {
			params[i] = fmt.Sprintf("mut %s", params[i])
		}
	}
	prefix := "fn"
	if t.CFunc {
		prefix = "extern fn"
	}
	results := resultNames(t.Result, t.Error)
	return fmt.Sprintf("%s(%s)%s", prefix, strings.Join(params, ", "), results)
}
