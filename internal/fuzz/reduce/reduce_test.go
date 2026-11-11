package reduce

import (
	"context"
	"strings"
	"testing"

	"lowlang/internal/fuzz/choice"
	"lowlang/internal/fuzz/gen"
	"lowlang/internal/fuzz/model"
)

func generated(t *testing.T, seed int64) *model.Program {
	t.Helper()
	return gen.Generate(choice.FromSeed(seed), gen.DefaultConfig)
}

func TestReduceKeepsTheTriggeringConstruct(t *testing.T) {
	keep := func(_ context.Context, p *model.Program) bool {
		return strings.Contains(p.String(), "<<")
	}
	var program *model.Program
	for seed := int64(0); program == nil; seed++ {
		if p := generated(t, seed); keep(context.Background(), p) {
			program = p
		}
	}
	original := program.String()
	r := &Reducer{Keep: keep, Workers: 4}
	reduced := r.Reduce(context.Background(), program)
	if !keep(context.Background(), reduced) {
		t.Fatal("reduced program lost the construct")
	}
	if len(reduced.String()) >= len(original)/4 {
		t.Errorf("reduced %d bytes to %d", len(original), len(reduced.String()))
	}
	if program.String() != original {
		t.Error("the input program was modified")
	}
	again := (&Reducer{Keep: keep, Workers: 4}).Reduce(context.Background(), reduced)
	if again.String() != reduced.String() {
		t.Errorf("not a fixed point:\n%s\n%s", reduced, again)
	}
}

func TestReducePreservesObservedOutput(t *testing.T) {
	program := generated(t, 3)
	outcome, err := model.Run(program, 2_000_000)
	if err != nil || outcome.Stdout == "" {
		t.Fatalf("seed 3 unusable: %v", err)
	}
	line, _, _ := strings.Cut(outcome.Stdout, "\n")
	keep := func(_ context.Context, p *model.Program) bool {
		got, err := model.Run(p, 2_000_000)
		return err == nil && strings.Contains(got.Stdout, line+"\n")
	}
	r := &Reducer{Keep: keep, Workers: 4}
	reduced := r.Reduce(context.Background(), program)
	if !keep(context.Background(), reduced) {
		t.Fatal("reduced program no longer prints the line")
	}
	if len(reduced.String()) >= len(program.String())/2 {
		t.Errorf("reduced %d bytes to %d", len(program.String()), len(reduced.String()))
	}
}

func TestReduceHonoursTestBudget(t *testing.T) {
	calls := 0
	keep := func(_ context.Context, p *model.Program) bool {
		calls++
		return true
	}
	r := &Reducer{Keep: keep, Workers: 1, MaxTests: 10}
	r.Reduce(context.Background(), generated(t, 1))
	if calls > 10 || r.Tests() > 10 {
		t.Errorf("%d predicate calls, %d tests", calls, r.Tests())
	}
}

func TestRemovedParametersLeaveCallsConsistent(t *testing.T) {
	keep := func(_ context.Context, p *model.Program) bool {
		return strings.Contains(p.String(), "fn f")
	}
	program := generated(t, 9)
	reduced := (&Reducer{Keep: keep, Workers: 4}).Reduce(context.Background(), program)
	arity := map[string]int{}
	for _, f := range reduced.Funcs {
		arity[f.Name] = len(f.Params)
	}
	eachSlot(reduced, func(e *model.Expr) bool {
		if c, ok := (*e).(*model.Call); ok && len(c.Args) != arity[c.Callee] {
			t.Errorf("call to %s has %d arguments, want %d", c.Callee, len(c.Args), arity[c.Callee])
		}
		return false
	})
	for _, f := range reduced.Funcs {
		if len(f.Params) != 0 {
			t.Errorf("%s kept %d unused parameters", f.Name, len(f.Params))
		}
	}
}

func TestReduceStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	keep := func(context.Context, *model.Program) bool { return true }
	program := generated(t, 1)
	reduced := (&Reducer{Keep: keep, Workers: 2}).Reduce(ctx, program)
	if reduced.String() != program.String() {
		t.Error("a cancelled reduction changed the program")
	}
}

func TestCloneIsDeep(t *testing.T) {
	program := generated(t, 5)
	before := program.String()
	clone := program.Clone()
	eachSlot(clone, func(e *model.Expr) bool {
		if lit, ok := (*e).(*model.Lit); ok && lit.Kind.IsInt() {
			lit.Bits ^= 0xFF
		}
		return false
	})
	if program.String() != before {
		t.Error("mutating the clone changed the original")
	}
	if clone.String() == before {
		t.Error("the clone was not mutated")
	}
}
