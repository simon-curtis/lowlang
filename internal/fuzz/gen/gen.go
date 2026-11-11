package gen

import (
	"fmt"
	"maps"
	"slices"

	"lowlang/internal/fuzz/choice"
	"lowlang/internal/fuzz/model"
)

type Config struct {
	Funcs    int
	MaxExpr  int
	MaxBlock int
	MaxNest  int
	Panics   bool
}

var DefaultConfig = Config{Funcs: 6, MaxExpr: 4, MaxBlock: 6, MaxNest: 3, Panics: false}

type variable struct {
	name    string
	kind    model.Kind
	mutable bool
}

type Generator struct {
	src     *choice.Source
	cfg     Config
	funcs   []*model.Func
	helpers map[string]*model.Func
	vars    []variable
	loops   []int
	names   int
	loopIDs int
	result  model.Kind
}

func Generate(src *choice.Source, cfg Config) *model.Program {
	g := &Generator{src: src, cfg: cfg, helpers: map[string]*model.Func{}}
	count := src.Range(1, cfg.Funcs)
	for i := range count {
		g.function(i)
	}
	main := g.mainBlock()
	funcs := make([]*model.Func, 0, len(g.helpers)+len(g.funcs))
	for _, name := range slices.Sorted(maps.Keys(g.helpers)) {
		funcs = append(funcs, g.helpers[name])
	}
	funcs = append(funcs, g.funcs...)
	return &model.Program{Funcs: funcs, Main: main}
}

func (g *Generator) function(index int) {
	g.vars = nil
	g.loops = nil
	g.result = g.pickKind(true)
	params := make([]model.Param, g.src.Range(0, 4))
	for i := range params {
		params[i] = model.Param{Name: fmt.Sprintf("p%d", i), Kind: g.pickKind(true)}
		g.vars = append(g.vars, variable{name: params[i].Name, kind: params[i].Kind})
	}
	body := g.block(0, g.src.Range(1, g.cfg.MaxBlock))
	tail := g.expr(g.result, g.cfg.MaxExpr)
	g.funcs = append(g.funcs, &model.Func{
		Name:   fmt.Sprintf("f%d", index),
		Params: params,
		Result: g.result,
		Body:   body,
		Tail:   tail,
	})
}

func (g *Generator) mainBlock() []model.Stmt {
	var stmts []model.Stmt
	for _, f := range g.funcs {
		for range g.src.Range(1, 3) {
			args := make([]model.Expr, len(f.Params))
			for i, p := range f.Params {
				args[i] = g.literal(p.Kind)
			}
			call := &model.Call{Callee: f.Name, Result: f.Result, Args: args}
			stmts = append(stmts, g.observe(call)...)
		}
	}
	return stmts
}

func (g *Generator) observe(call *model.Call) []model.Stmt {
	if !call.Result.IsFloat() {
		return []model.Stmt{&model.Print{Value: call}}
	}
	return []model.Stmt{
		&model.Print{Value: g.clamp(call, model.I64)},
		&model.Print{Value: &model.Binary{Op: model.Ne, L: call, R: call}},
	}
}

func (g *Generator) pickKind(withBool bool) model.Kind {
	weights := []struct {
		kind   model.Kind
		weight int
	}{
		{model.I64, 6}, {model.I32, 4}, {model.U64, 4}, {model.U32, 4}, {model.U16, 2}, {model.U8, 4},
		{model.F64, 2}, {model.F32, 2},
	}
	if withBool {
		weights = append(weights, struct {
			kind   model.Kind
			weight int
		}{model.Bool, 2})
	}
	total := 0
	for _, w := range weights {
		total += w.weight
	}
	n := g.src.Intn(total)
	for _, w := range weights {
		if n < w.weight {
			return w.kind
		}
		n -= w.weight
	}
	return model.I64
}

func (g *Generator) pickInt() model.Kind {
	return choice.Pick(g.src, model.IntKinds)
}

func (g *Generator) pickNumeric() model.Kind {
	for {
		if k := g.pickKind(false); k != model.Bool {
			return k
		}
	}
}

func (g *Generator) fresh(prefix string) string {
	g.names++
	return fmt.Sprintf("%s_%d", prefix, g.names)
}

func (g *Generator) varsOf(kind model.Kind, mutableOnly bool) []variable {
	var found []variable
	for _, v := range g.vars {
		if v.kind == kind && (!mutableOnly || v.mutable) {
			found = append(found, v)
		}
	}
	return found
}

func (g *Generator) mutableVars() []variable {
	var found []variable
	for _, v := range g.vars {
		if v.mutable {
			found = append(found, v)
		}
	}
	return found
}
