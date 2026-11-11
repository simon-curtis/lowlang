package reduce

import "lowlang/internal/fuzz/model"

type walker struct {
	target int
	seen   int
}

func counting() *walker {
	return &walker{target: -1}
}

func (w *walker) emit(apply func()) bool {
	hit := w.seen == w.target
	w.seen++
	if hit {
		apply()
	}
	return hit
}

func eachList(p *model.Program, fn func(*[]model.Stmt) bool) bool {
	for _, f := range p.Funcs {
		if listsIn(&f.Body, fn) {
			return true
		}
	}
	return listsIn(&p.Main, fn)
}

func listsIn(list *[]model.Stmt, fn func(*[]model.Stmt) bool) bool {
	if fn(list) {
		return true
	}
	for _, s := range *list {
		switch s := s.(type) {
		case *model.IfStmt:
			if listsIn(&s.Then, fn) || listsIn(&s.Else, fn) {
				return true
			}
		case *model.Range:
			if listsIn(&s.Body, fn) {
				return true
			}
		case *model.While:
			if listsIn(&s.Body, fn) {
				return true
			}
		}
	}
	return false
}

func eachSlot(p *model.Program, fn func(*model.Expr) bool) bool {
	for _, f := range p.Funcs {
		if stmtSlots(f.Body, fn) || slotsIn(&f.Tail, fn) {
			return true
		}
	}
	return stmtSlots(p.Main, fn)
}

func stmtSlots(stmts []model.Stmt, fn func(*model.Expr) bool) bool {
	for _, s := range stmts {
		if stmtSlot(s, fn) {
			return true
		}
	}
	return false
}

func stmtSlot(s model.Stmt, fn func(*model.Expr) bool) bool {
	switch s := s.(type) {
	case *model.Let:
		return slotsIn(&s.Init, fn)
	case *model.Assign:
		return slotsIn(&s.Value, fn)
	case *model.IfStmt:
		return slotsIn(&s.Cond, fn) || stmtSlots(s.Then, fn) || stmtSlots(s.Else, fn)
	case *model.Range:
		return slotsIn(&s.Lo, fn) || slotsIn(&s.Hi, fn) || stmtSlots(s.Body, fn)
	case *model.While:
		return slotsIn(&s.Cond, fn) || stmtSlots(s.Body, fn)
	case *model.Return:
		return slotsIn(&s.Value, fn)
	case *model.Print:
		return slotsIn(&s.Value, fn)
	}
	return false
}

func slotsIn(e *model.Expr, fn func(*model.Expr) bool) bool {
	if *e == nil {
		return false
	}
	if fn(e) {
		return true
	}
	switch x := (*e).(type) {
	case *model.Unary:
		return slotsIn(&x.X, fn)
	case *model.Binary:
		return slotsIn(&x.L, fn) || slotsIn(&x.R, fn)
	case *model.Convert:
		return slotsIn(&x.X, fn)
	case *model.IfExpr:
		return slotsIn(&x.Cond, fn) || slotsIn(&x.Then, fn) || slotsIn(&x.Else, fn)
	case *model.Math:
		return slotsIn(&x.X, fn)
	case *model.Call:
		for i := range x.Args {
			if slotsIn(&x.Args[i], fn) {
				return true
			}
		}
	}
	return false
}
