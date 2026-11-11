package reduce

import (
	"math"
	"slices"

	"lowlang/internal/fuzz/model"
)

type pass func(p *model.Program, w *walker)

var passes = []pass{removeFuncs, removeParams, removeStmts, unwrapStmts, simplifyLoops, simplifyExprs, shrinkLiterals}

func removeFuncs(p *model.Program, w *walker) {
	called := map[string]bool{}
	eachSlot(p, func(e *model.Expr) bool {
		if c, ok := (*e).(*model.Call); ok {
			called[c.Callee] = true
		}
		return false
	})
	for i, f := range p.Funcs {
		if called[f.Name] {
			continue
		}
		if w.emit(func() { p.Funcs = slices.Delete(slices.Clone(p.Funcs), i, i+1) }) {
			return
		}
	}
}

func usesVariable(f *model.Func, name string) bool {
	used := false
	eachSlot(&model.Program{Funcs: []*model.Func{f}}, func(e *model.Expr) bool {
		if v, ok := (*e).(*model.Var); ok && v.Name == name {
			used = true
		}
		return used
	})
	return used
}

func dropParam(p *model.Program, f *model.Func, index int) {
	f.Params = slices.Delete(slices.Clone(f.Params), index, index+1)
	eachSlot(p, func(e *model.Expr) bool {
		if c, ok := (*e).(*model.Call); ok && c.Callee == f.Name {
			c.Args = slices.Delete(slices.Clone(c.Args), index, index+1)
		}
		return false
	})
}

func removeParams(p *model.Program, w *walker) {
	for _, f := range p.Funcs {
		for j, param := range f.Params {
			if usesVariable(f, param.Name) {
				continue
			}
			if w.emit(func() { dropParam(p, f, j) }) {
				return
			}
		}
	}
}

func removeStmts(p *model.Program, w *walker) {
	eachList(p, func(list *[]model.Stmt) bool {
		n := len(*list)
		for size := n; size >= 1; size /= 2 {
			for start := 0; start < n; start += size {
				end := min(start+size, n)
				if w.emit(func() { *list = slices.Delete(slices.Clone(*list), start, end) }) {
					return true
				}
			}
		}
		return false
	})
}

func splice(list *[]model.Stmt, i int, body []model.Stmt) {
	*list = slices.Concat((*list)[:i], body, (*list)[i+1:])
}

func unwrapStmts(p *model.Program, w *walker) {
	eachList(p, func(list *[]model.Stmt) bool {
		for i, s := range *list {
			if unwrapAt(list, i, s, w) {
				return true
			}
		}
		return false
	})
}

func unwrapAt(list *[]model.Stmt, i int, s model.Stmt, w *walker) bool {
	switch s := s.(type) {
	case *model.IfStmt:
		if w.emit(func() { splice(list, i, s.Then) }) {
			return true
		}
		if len(s.Else) == 0 {
			return false
		}
		return w.emit(func() { splice(list, i, s.Else) }) || w.emit(func() { s.Else = nil })
	case *model.Range:
		return w.emit(func() { splice(list, i, s.Body) })
	case *model.While:
		return w.emit(func() { splice(list, i, s.Body) })
	}
	return false
}

func simplifyLoops(p *model.Program, w *walker) {
	eachList(p, func(list *[]model.Stmt) bool {
		for _, s := range *list {
			loop, ok := s.(*model.While)
			if !ok {
				continue
			}
			if loop.Bound > 1 && w.emit(func() { loop.Bound = 1 }) {
				return true
			}
			if loop.Cond != nil && w.emit(func() { loop.Cond = nil }) {
				return true
			}
		}
		return false
	})
}

func zero(kind model.Kind) model.Expr {
	return &model.Lit{Kind: kind}
}

func one(kind model.Kind) model.Expr {
	if kind.IsFloat() {
		return &model.Lit{Kind: kind, Float: 1}
	}
	return &model.Lit{Kind: kind, Bits: 1}
}

func operands(e model.Expr) []model.Expr {
	switch e := e.(type) {
	case *model.Unary:
		return []model.Expr{e.X}
	case *model.Binary:
		return []model.Expr{e.L, e.R}
	case *model.Convert:
		return []model.Expr{e.X}
	case *model.IfExpr:
		return []model.Expr{e.Then, e.Else}
	case *model.Math:
		return []model.Expr{e.X}
	}
	return nil
}

func candidates(e model.Expr) []model.Expr {
	kind := e.Type()
	out := []model.Expr{zero(kind)}
	for _, operand := range operands(e) {
		if operand.Type() == kind {
			out = append(out, operand)
		}
	}
	return append(out, one(kind))
}

func simplifyExprs(p *model.Program, w *walker) {
	eachSlot(p, func(e *model.Expr) bool {
		if _, isLit := (*e).(*model.Lit); isLit {
			return false
		}
		for _, c := range candidates(*e) {
			if w.emit(func() { *e = c }) {
				return true
			}
		}
		return false
	})
}

func smaller(l *model.Lit) []model.Expr {
	var out []model.Expr
	add := func(bits uint64, f float64) {
		out = append(out, &model.Lit{Kind: l.Kind, Bits: bits, Float: f})
	}
	switch {
	case l.Kind == model.Bool:
		if l.Bits != 0 {
			add(0, 0)
		}
	case l.Kind.IsFloat():
		if l.Float != 0 || math.Signbit(l.Float) {
			add(0, 0)
		}
		if math.Abs(l.Float) > 1 {
			add(0, 1)
		}
		if t := math.Trunc(l.Float); t != l.Float && t != 0 {
			add(0, t)
		}
	default:
		if l.Bits != 0 {
			add(0, 0)
		}
		if l.Bits > 1 {
			add(1, 0)
			add(l.Bits>>1, 0)
		}
	}
	return out
}

func shrinkLiterals(p *model.Program, w *walker) {
	eachSlot(p, func(e *model.Expr) bool {
		lit, ok := (*e).(*model.Lit)
		if !ok {
			return false
		}
		for _, c := range smaller(lit) {
			if w.emit(func() { *e = c }) {
				return true
			}
		}
		return false
	})
}
