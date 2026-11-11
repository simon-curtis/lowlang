package gen

import (
	"lowlang/internal/fuzz/choice"
	"lowlang/internal/fuzz/model"
)

type stmtKind int

const (
	stmtLet stmtKind = iota
	stmtAssign
	stmtIf
	stmtRange
	stmtWhile
	stmtJump
	stmtReturn
	stmtPrint
)

var stmtWeights = []struct {
	kind   stmtKind
	weight int
}{
	{stmtLet, 25}, {stmtAssign, 20}, {stmtIf, 15}, {stmtRange, 8}, {stmtWhile, 6}, {stmtJump, 8}, {stmtReturn, 4}, {stmtPrint, 6},
}

func (g *Generator) block(nest, count int) []model.Stmt {
	saved := len(g.vars)
	stmts := make([]model.Stmt, 0, count)
	for range count {
		stmts = append(stmts, g.stmt(nest))
	}
	g.vars = g.vars[:saved]
	return stmts
}

func (g *Generator) pickStmt(nest int) stmtKind {
	for {
		total := 0
		for _, w := range stmtWeights {
			total += w.weight
		}
		n := g.src.Intn(total)
		for _, w := range stmtWeights {
			if n >= w.weight {
				n -= w.weight
				continue
			}
			if g.allowed(w.kind, nest) {
				return w.kind
			}
			break
		}
	}
}

func (g *Generator) allowed(kind stmtKind, nest int) bool {
	switch kind {
	case stmtAssign:
		return len(g.mutableVars()) > 0
	case stmtIf, stmtRange, stmtWhile:
		return nest < g.cfg.MaxNest
	case stmtJump:
		return len(g.loops) > 0
	}
	return true
}

func (g *Generator) stmt(nest int) model.Stmt {
	switch g.pickStmt(nest) {
	case stmtLet:
		return g.let()
	case stmtAssign:
		return g.assign()
	case stmtIf:
		return g.ifStmt(nest)
	case stmtRange:
		return g.rangeStmt(nest)
	case stmtWhile:
		return g.whileStmt(nest)
	case stmtJump:
		return g.jump()
	case stmtReturn:
		return &model.IfStmt{Cond: g.expr(model.Bool, 2), Then: []model.Stmt{&model.Return{Value: g.expr(g.result, 2)}}}
	}
	return &model.Print{Value: g.printable()}
}

func (g *Generator) printable() model.Expr {
	kind := g.pickKind(true)
	if kind.IsFloat() {
		return g.clamp(g.expr(kind, 2), model.I64)
	}
	return g.expr(kind, 2)
}

func (g *Generator) let() model.Stmt {
	kind := g.pickKind(true)
	init := g.expr(kind, g.cfg.MaxExpr)
	s := &model.Let{Name: g.fresh("v"), Kind: kind, Mutable: g.src.Bool(), Init: init}
	g.vars = append(g.vars, variable{name: s.Name, kind: kind, mutable: s.Mutable})
	return s
}

func (g *Generator) assign() model.Stmt {
	target := choice.Pick(g.src, g.mutableVars())
	op := g.assignOp(target.kind)
	value := g.operand(op, target.kind, g.cfg.MaxExpr)
	return &model.Assign{Name: target.name, Kind: target.kind, Op: op, Value: value}
}

func (g *Generator) assignOp(kind model.Kind) model.Op {
	switch {
	case kind == model.Bool:
		return model.Set
	case kind.IsFloat():
		return choice.Pick(g.src, []model.Op{model.Set, model.Add, model.Sub, model.Mul, model.Div})
	}
	return choice.Pick(g.src, []model.Op{
		model.Set, model.Add, model.Sub, model.Mul, model.Div, model.Rem, model.And, model.Or, model.Xor, model.Shl, model.Shr,
	})
}

func (g *Generator) operand(op model.Op, kind model.Kind, depth int) model.Expr {
	switch {
	case op == model.Div || op == model.Rem:
		return g.divisor(kind, depth)
	case op.IsShift():
		return g.shiftCount(depth)
	}
	return g.expr(kind, depth)
}

func (g *Generator) ifStmt(nest int) model.Stmt {
	s := &model.IfStmt{Cond: g.expr(model.Bool, 3), Then: g.block(nest+1, g.src.Range(1, 3))}
	if g.src.Bool() {
		s.Else = g.block(nest+1, g.src.Range(1, 3))
	}
	return s
}

func (g *Generator) bound(kind model.Kind) model.Expr {
	if g.src.Bool() {
		return &model.Lit{Kind: kind, Bits: uint64(g.src.Intn(8))}
	}
	return &model.Binary{Op: model.And, L: g.expr(kind, 2), R: &model.Lit{Kind: kind, Bits: 7}}
}

func (g *Generator) loopID() int {
	g.loopIDs++
	return g.loopIDs
}

func (g *Generator) rangeStmt(nest int) model.Stmt {
	kind := g.pickInt()
	s := &model.Range{ID: g.loopID(), Var: g.fresh("i"), Kind: kind, Lo: g.bound(kind), Hi: g.bound(kind)}
	saved := len(g.vars)
	g.vars = append(g.vars, variable{name: s.Var, kind: kind})
	g.loops = append(g.loops, s.ID)
	s.Body = g.block(nest+1, g.src.Range(1, 4))
	g.loops = g.loops[:len(g.loops)-1]
	g.vars = g.vars[:saved]
	return s
}

func (g *Generator) whileStmt(nest int) model.Stmt {
	s := &model.While{ID: g.loopID(), Counter: g.fresh("w"), Bound: g.src.Range(1, 6)}
	if g.src.Chance(60) {
		s.Cond = g.expr(model.Bool, 2)
	}
	g.loops = append(g.loops, s.ID)
	s.Body = g.block(nest+1, g.src.Range(1, 4))
	g.loops = g.loops[:len(g.loops)-1]
	return s
}

func (g *Generator) jump() model.Stmt {
	target := g.loops[len(g.loops)-1]
	if g.src.Chance(35) {
		target = choice.Pick(g.src, g.loops)
	}
	return &model.IfStmt{
		Cond: g.expr(model.Bool, 2),
		Then: []model.Stmt{&model.Jump{Target: target, Continue: g.src.Bool()}},
	}
}
