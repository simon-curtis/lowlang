package lsp

import (
	"lowlang/internal/ast"
	"lowlang/internal/diag"
	"lowlang/internal/types"
)

type target struct {
	path string
	pos  diag.Pos
	name string
}

type query struct {
	program
	index *index
	occ   *occurrence
}

func (q *query) callee(call *ast.Call) *query {
	occ := *q.occ
	occ.node = call.Fn
	return &query{program: q.program, index: q.index, occ: &occ}
}

func (q *query) file() *ast.File {
	return q.index.file
}

func (q *query) module(alias string) (string, bool) {
	if alias == "" {
		return q.file().Module, true
	}
	return q.file().Imported(alias)
}

func (q *query) declared(alias, name string) any {
	module, ok := q.module(alias)
	if !ok {
		return nil
	}
	return q.typeDecl(module, name)
}

func sameType(decl, t *types.Type) bool {
	return decl != nil && t != nil && (decl == t || decl == t.Origin)
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

func (q *query) variantDecl(t *types.Type, name string) (*ast.EnumDecl, *ast.VariantDecl) {
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

func (q *query) moduleTarget(path string) (target, bool) {
	files := q.moduleFiles(path)
	if len(files) == 0 {
		return target{}, false
	}
	return target{path: files[0].Path, pos: diag.Pos{Line: 1, Col: 1}}, true
}

func (q *query) resolve() referent {
	switch n := q.occ.node.(type) {
	case *ast.Call:
		return q.call(n)
	case *ast.Ident:
		return q.ident(n)
	case *ast.Selector:
		return q.selector(n)
	case *ast.StructLit:
		return q.named(n.Module, n.Name, nothing{})
	case *ast.TypeName:
		return q.named(n.Module, n.Name, spelled{text: n.String()})
	case *ast.FieldInit:
		return q.field(q.occ.parent, n.Name, n.Field)
	case qualifier:
		return q.imported(n.alias)
	case *ast.Pattern:
		return q.pattern(n)
	case *ast.Binding:
		return q.localRef(n.Local)
	case *ast.Import:
		return importRef{q: q, path: n.Path}
	}
	return q.declaration()
}

func (q *query) call(n *ast.Call) referent {
	switch {
	case n.Target != nil:
		return funcRef{fn: n.Target}
	case n.Variant != nil:
		return q.variantRef(n.TypeOf(), n.Variant.Name, n.Variant)
	}
	callee := q.callee(n).resolve()
	if text := builtinSignature(n); text != "" {
		return described{referent: callee, text: text}
	}
	return callee
}

func (q *query) ident(n *ast.Ident) referent {
	switch {
	case n.Local != nil:
		return q.localRef(n.Local)
	case n.Func != nil:
		return funcRef{fn: n.Func}
	case n.Variant != nil:
		return q.variantRef(n.TypeOf(), n.Name, n.Variant)
	}
	if decl := q.declared("", n.Name); decl != nil {
		return typeRef{q: q, decl: decl}
	}
	if path, ok := q.file().Imported(n.Name); ok {
		return importRef{q: q, path: path}
	}
	return valueRef{name: n.Name, t: n.TypeOf()}
}

func (q *query) selector(n *ast.Selector) referent {
	switch {
	case n.Func != nil:
		return funcRef{fn: n.Func}
	case n.Field != nil:
		return q.field(n.X.TypeOf(), n.Name, n.Field)
	case n.Variant != nil:
		return q.variantRef(n.TypeOf(), n.Name, n.Variant)
	}
	return valueRef{name: n.Name, t: n.TypeOf()}
}

func (q *query) pattern(n *ast.Pattern) referent {
	if n.Local != nil {
		return q.localRef(n.Local)
	}
	return q.variantRef(q.occ.parent, n.Name, n.Variant)
}

func (q *query) named(alias, name string, unresolved referent) referent {
	if decl := q.declared(alias, name); decl != nil {
		return typeRef{q: q, decl: decl}
	}
	return unresolved
}

func (q *query) imported(alias string) referent {
	if path, ok := q.file().Imported(alias); ok {
		return importRef{q: q, path: path}
	}
	return nothing{}
}

func (q *query) localRef(l *ast.Local) referent {
	if l == nil {
		return nothing{}
	}
	return localRef{q: q, local: l}
}

func (q *query) field(owner *types.Type, name string, field *types.Field) referent {
	return fieldRef{q: q, owner: owner, name: name, field: field}
}

func (q *query) variantRef(enum *types.Type, name string, variant *types.Variant) referent {
	return variantRef{q: q, enum: enum, name: name, variant: variant}
}

func (q *query) declaration() referent {
	self := target{path: q.file().Path, pos: q.occ.pos}
	switch n := q.occ.node.(type) {
	case *ast.Func:
		return own{at: self, referent: funcRef{fn: n}}
	case *ast.StructDecl:
		return own{at: self, referent: typeRef{q: q, decl: n}}
	case *ast.EnumDecl:
		return own{at: self, referent: typeRef{q: q, decl: n}}
	case *ast.FieldDecl:
		return own{at: self, referent: q.field(q.occ.parent, n.Name, nil)}
	case *ast.VariantDecl:
		return own{at: self, referent: q.variantRef(q.occ.parent, n.Name, nil)}
	case *ast.TypeParam:
		return own{at: self, referent: spelled{text: n.String()}}
	case *ast.Let, *ast.Param, *ast.ForIn, *ast.SelectCase:
		return own{at: self, referent: q.localRef(q.occ.local)}
	}
	return nothing{}
}
