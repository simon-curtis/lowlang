package gen

import (
	"fmt"
	"math"

	"lowlang/internal/fuzz/choice"
	"lowlang/internal/fuzz/model"
)

var intOps = []model.Op{model.Add, model.Sub, model.Mul, model.And, model.Or, model.Xor}

var floatOps = []model.Op{model.Add, model.Sub, model.Mul, model.Div}

var compareOps = []model.Op{model.Eq, model.Ne, model.Lt, model.Le, model.Gt, model.Ge}

var floatValues = []float64{0, 0.5, 1, 1.5, 2, 3, 10, 0.1, 0.3, 100.25, 1000, 65536, 12345.678, 1e9, 1e-6, 16777217}

func (g *Generator) expr(kind model.Kind, depth int) model.Expr {
	if depth <= 0 || g.src.Chance(15) {
		return g.leaf(kind)
	}
	switch {
	case kind == model.Bool:
		return g.boolExpr(depth)
	case kind.IsFloat():
		return g.floatExpr(kind, depth)
	}
	return g.intExpr(kind, depth)
}

func (g *Generator) leaf(kind model.Kind) model.Expr {
	if candidates := g.varsOf(kind, false); len(candidates) > 0 && g.src.Chance(65) {
		v := choice.Pick(g.src, candidates)
		return &model.Var{Name: v.name, Kind: v.kind}
	}
	return g.literal(kind)
}

func (g *Generator) literal(kind model.Kind) model.Expr {
	switch {
	case kind == model.Bool:
		return &model.Lit{Kind: model.Bool, Bits: uint64(g.src.Intn(2))}
	case kind.IsFloat():
		return g.floatLit(kind)
	}
	return g.intLit(kind)
}

func (g *Generator) floatLit(kind model.Kind) model.Expr {
	v := choice.Pick(g.src, floatValues)
	if g.src.Chance(30) {
		v = -v
	}
	if v == 0 && g.src.Chance(30) {
		v = math.Copysign(0, -1)
	}
	if kind == model.F32 {
		v = float64(float32(v))
	}
	return &model.Lit{Kind: kind, Float: v}
}

func boundaries(kind model.Kind) []uint64 {
	mask := kind.Mask()
	half := mask >> 1
	return []uint64{0, 1, 2, mask, mask - 1, half, half + 1, half - 1, 0x5555555555555555 & mask, 0xAAAAAAAAAAAAAAAA & mask}
}

func (g *Generator) intLit(kind model.Kind) model.Expr {
	var bits uint64
	switch n := g.src.Intn(10); {
	case n < 4:
		bits = uint64(g.src.Intn(12))
		if kind.Signed() && g.src.Chance(30) {
			bits = -bits
		}
	case n < 7:
		bits = choice.Pick(g.src, boundaries(kind))
	default:
		bits = g.src.Uint64()
	}
	return &model.Lit{Kind: kind, Bits: kind.Wrap(bits)}
}

func (g *Generator) intExpr(kind model.Kind, depth int) model.Expr {
	switch n := g.src.Intn(20); {
	case n < 8:
		op := choice.Pick(g.src, intOps)
		return &model.Binary{Op: op, L: g.expr(kind, depth-1), R: g.expr(kind, depth-1)}
	case n < 10:
		op := choice.Pick(g.src, []model.Op{model.Div, model.Rem})
		return &model.Binary{Op: op, L: g.expr(kind, depth-1), R: g.divisor(kind, depth-1)}
	case n < 13:
		op := choice.Pick(g.src, []model.Op{model.Shl, model.Shr})
		return &model.Binary{Op: op, L: g.expr(kind, depth-1), R: g.shiftCount(depth - 1)}
	case n < 14:
		return &model.Unary{Op: model.Neg, X: g.expr(kind, depth-1)}
	case n < 15:
		return &model.Unary{Op: model.Compl, X: g.expr(kind, depth-1)}
	case n < 17:
		return g.convert(kind, depth-1)
	case n < 18:
		return g.ifExpr(kind, depth-1)
	}
	return g.callOr(kind, depth-1)
}

func (g *Generator) floatExpr(kind model.Kind, depth int) model.Expr {
	switch n := g.src.Intn(14); {
	case n < 6:
		op := choice.Pick(g.src, floatOps)
		return &model.Binary{Op: op, L: g.expr(kind, depth-1), R: g.expr(kind, depth-1)}
	case n < 7:
		return &model.Unary{Op: model.Neg, X: g.expr(kind, depth-1)}
	case n < 9:
		op := choice.Pick(g.src, []model.MathOp{model.Sqrt, model.Floor})
		return &model.Math{Op: op, X: g.expr(kind, depth-1)}
	case n < 11:
		return g.convert(kind, depth-1)
	case n < 12:
		return g.ifExpr(kind, depth-1)
	}
	return g.callOr(kind, depth-1)
}

func (g *Generator) boolExpr(depth int) model.Expr {
	switch n := g.src.Intn(10); {
	case n < 5:
		kind := g.pickNumeric()
		op := choice.Pick(g.src, compareOps)
		return &model.Binary{Op: op, L: g.expr(kind, depth-1), R: g.expr(kind, depth-1)}
	case n < 7:
		op := choice.Pick(g.src, []model.Op{model.LogAnd, model.LogOr})
		return &model.Binary{Op: op, L: g.expr(model.Bool, depth-1), R: g.expr(model.Bool, depth-1)}
	case n < 8:
		return &model.Unary{Op: model.Not, X: g.expr(model.Bool, depth-1)}
	case n < 9:
		return g.ifExpr(model.Bool, depth-1)
	}
	return g.callOr(model.Bool, depth-1)
}

func (g *Generator) ifExpr(kind model.Kind, depth int) model.Expr {
	return &model.IfExpr{Cond: g.expr(model.Bool, depth), Then: g.expr(kind, depth), Else: g.expr(kind, depth)}
}

func (g *Generator) callOr(kind model.Kind, depth int) model.Expr {
	var callees []*model.Func
	for _, f := range g.funcs {
		if f.Result == kind {
			callees = append(callees, f)
		}
	}
	if len(callees) == 0 || g.src.Chance(40) {
		return g.leaf(kind)
	}
	f := choice.Pick(g.src, callees)
	args := make([]model.Expr, len(f.Params))
	for i, p := range f.Params {
		args[i] = g.expr(p.Kind, depth)
	}
	return &model.Call{Callee: f.Name, Result: kind, Args: args}
}

func (g *Generator) divisor(kind model.Kind, depth int) model.Expr {
	raw := g.expr(kind, depth)
	if kind.IsFloat() || (g.cfg.Panics && g.src.Chance(15)) {
		return raw
	}
	return &model.Binary{Op: model.Or, L: raw, R: &model.Lit{Kind: kind, Bits: 1}}
}

func (g *Generator) shiftCount(depth int) model.Expr {
	kind := g.pickInt()
	if g.src.Bool() {
		return &model.Lit{Kind: kind, Bits: kind.Wrap(uint64(g.src.Intn(kind.Bits() + 3)))}
	}
	return g.expr(kind, depth)
}

func (g *Generator) convert(to model.Kind, depth int) model.Expr {
	from := g.pickNumeric()
	x := g.expr(from, depth)
	if from.IsFloat() && to.IsInt() {
		return g.clamp(x, to)
	}
	return &model.Convert{To: to, X: x}
}

func (g *Generator) clamp(x model.Expr, to model.Kind) model.Expr {
	from := x.Type()
	name := fmt.Sprintf("clamp_%s_%s", from, to)
	if _, ok := g.helpers[name]; !ok {
		g.helpers[name] = clampHelper(name, from, to)
	}
	return &model.Call{Callee: name, Result: to, Args: []model.Expr{x}}
}

func clampHelper(name string, from, to model.Kind) *model.Func {
	limit := math.Exp2(float64(to.Bits()))
	low := -1.0
	if to.Signed() {
		limit = math.Exp2(float64(to.Bits() - 1))
		low = -limit
	}
	x := &model.Var{Name: "x", Kind: from}
	inRange := &model.Binary{
		Op: model.LogAnd,
		L:  &model.Binary{Op: model.Gt, L: x, R: floatBound(from, low)},
		R:  &model.Binary{Op: model.Lt, L: x, R: floatBound(from, limit)},
	}
	return &model.Func{
		Name:   name,
		Params: []model.Param{{Name: "x", Kind: from}},
		Result: to,
		Tail:   &model.IfExpr{Cond: inRange, Then: &model.Convert{To: to, X: x}, Else: &model.Lit{Kind: to}},
	}
}

func floatBound(kind model.Kind, v float64) model.Expr {
	if kind == model.F32 {
		v = float64(float32(v))
	}
	return &model.Lit{Kind: kind, Float: v}
}
