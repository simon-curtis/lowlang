package lsp

import (
	"fmt"
	"strings"

	"lowlang/internal/ast"
	"lowlang/internal/types"
)

type referent interface {
	target() (target, bool)
	hover() string
	doc() string
}

type nothing struct{}

func (nothing) target() (target, bool) { return target{}, false }
func (nothing) hover() string          { return "" }
func (nothing) doc() string            { return "" }

type spelled struct {
	nothing
	text string
}

func (s spelled) hover() string { return s.text }

type valueRef struct {
	nothing
	name string
	t    *types.Type
}

func (v valueRef) hover() string {
	if v.t == nil {
		return ""
	}
	return fmt.Sprintf("%s: %s", v.name, v.t)
}

type described struct {
	referent
	text string
}

func (d described) hover() string { return d.text }

type own struct {
	referent
	at target
}

func (o own) target() (target, bool) { return o.at, true }

type funcRef struct {
	fn *ast.Func
}

func (r funcRef) target() (target, bool) {
	f := r.fn.Source()
	if f.File == nil || f.File.Path == "" {
		return target{}, false
	}
	return target{path: f.File.Path, pos: f.Pos, name: f.Name}, true
}

func (r funcRef) hover() string { return r.fn.Signature() }
func (r funcRef) doc() string   { return r.fn.Source().Doc }

type importRef struct {
	q    *query
	path string
}

func (r importRef) target() (target, bool) { return r.q.moduleTarget(r.path) }
func (r importRef) hover() string          { return fmt.Sprintf("import %s", r.path) }
func (r importRef) doc() string            { return "" }

type typeRef struct {
	decl *typeDecl
}

func (r typeRef) target() (target, bool) { return r.decl.target(), true }
func (r typeRef) hover() string          { return r.decl.text }
func (r typeRef) doc() string            { return r.decl.doc }

type localRef struct {
	q     *query
	local *ast.Local
}

func (r localRef) target() (target, bool) {
	d := r.q.index.declaration(r.local)
	if d == nil {
		return target{}, false
	}
	return target{path: r.q.file().Path, pos: d.pos}, true
}

func (r localRef) hover() string {
	l := r.local
	keyword := "let "
	switch {
	case r.using() && l.Mutable:
		keyword = "using var "
	case r.using():
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

func (r localRef) doc() string { return "" }

func (r localRef) using() bool {
	d := r.q.index.declaration(r.local)
	if d == nil {
		return false
	}
	let, ok := d.node.(*ast.Let)
	return ok && let.Using
}

type fieldRef struct {
	q     *query
	owner *types.Type
	name  string
	field *types.Field
}

func (r fieldRef) typed() *types.Field {
	if r.field != nil {
		return r.field
	}
	if r.owner == nil {
		return nil
	}
	return types.StructOf(r.owner).Field(r.name)
}

func (r fieldRef) decl() *ast.FieldDecl {
	d := r.q.structDecl(r.owner)
	if d == nil {
		return nil
	}
	for _, f := range d.Fields {
		if f.Name == r.name {
			return f
		}
	}
	return nil
}

func (r fieldRef) target() (target, bool) {
	f := r.decl()
	if f == nil {
		return target{}, false
	}
	return target{path: r.q.structDecl(r.owner).File.Path, pos: f.Pos}, true
}

func (r fieldRef) hover() string {
	f := r.typed()
	if f == nil {
		return ""
	}
	return fmt.Sprintf("%s: %s", r.name, f.Type)
}

func (r fieldRef) doc() string {
	f := r.decl()
	if f == nil {
		return ""
	}
	return f.Doc
}

type variantRef struct {
	q       *query
	enum    *types.Type
	name    string
	variant *types.Variant
}

func (r variantRef) enumVariant() (*types.Type, *types.Variant) {
	if r.variant != nil {
		return r.enum, r.variant
	}
	if r.enum == nil {
		return nil, nil
	}
	return r.enum, r.enum.Variant(r.name)
}

func (r variantRef) target() (target, bool) {
	d, v := r.q.variantDecl(r.enum, r.name)
	if d == nil {
		return target{}, false
	}
	return target{path: d.File.Path, pos: v.Pos}, true
}

func (r variantRef) hover() string {
	enum, v := r.enumVariant()
	if v == nil {
		return ""
	}
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

func (r variantRef) doc() string {
	_, v := r.q.variantDecl(r.enum, r.name)
	if v == nil {
		return ""
	}
	return v.Doc
}
