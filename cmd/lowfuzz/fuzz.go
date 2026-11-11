package main

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"

	"lowlang/internal/fuzz/choice"
	"lowlang/internal/fuzz/corpus"
	"lowlang/internal/fuzz/gen"
	"lowlang/internal/fuzz/model"
	"lowlang/internal/fuzz/oracle"
	"lowlang/internal/fuzz/reduce"
)

type fuzzer struct {
	cfg         gen.Config
	runner      *oracle.Runner
	report      *report
	budget      int
	workers     int
	reduce      bool
	reduceTests int
	fixtures    string
}

func (f *fuzzer) run(ctx context.Context, s settings) {
	var next atomic.Int64
	var wg sync.WaitGroup
	for range s.workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ctx.Err() == nil {
				n := next.Add(1) - 1
				if s.cases > 0 && n >= int64(s.cases) {
					return
				}
				f.runCase(ctx, s.seed+n)
			}
		}()
	}
	wg.Wait()
}

func (f *fuzzer) runCase(ctx context.Context, seed int64) {
	program := gen.Generate(choice.FromSeed(seed), f.cfg)
	outcome, err := model.Run(program, f.budget)
	switch {
	case errors.Is(err, model.ErrBudget):
		f.report.caseSkipped()
		return
	case err != nil:
		finding := oracle.Finding{Class: oracle.ClassGenerator, Detail: err.Error(), Source: program.String()}
		f.handle(ctx, seed, program, finding)
		return
	}
	findings := f.runner.Check(ctx, program.String(), outcome)
	if ctx.Err() != nil {
		return
	}
	f.report.caseRan()
	for _, finding := range findings {
		if f.report.claim(finding.Signature()) {
			f.handle(ctx, seed, program, finding)
		}
	}
}

func (f *fuzzer) handle(ctx context.Context, seed int64, program *model.Program, finding oracle.Finding) {
	signature := finding.Signature()
	reduced := program
	if f.reduce && finding.Class != oracle.ClassGenerator && finding.Class != oracle.ClassHarness {
		reducer := &reduce.Reducer{
			Keep:     f.runner.Reproduces(signature, f.budget),
			Workers:  f.workers,
			MaxTests: f.reduceTests,
		}
		reduced = reducer.Reduce(ctx, program)
	}
	path := f.report.save(seed, finding.Class, reduced.String())
	f.report.announce("seed %d: %s\n  %d -> %d bytes, %s\n", seed, signature, len(program.String()), len(reduced.String()), path)
	f.writeFixture(reduced)
}

func (f *fuzzer) writeFixture(program *model.Program) {
	if f.fixtures == "" {
		return
	}
	outcome, err := model.Run(program, f.budget)
	if err != nil {
		return
	}
	path, err := corpus.FromProgram(program, outcome).Write(f.fixtures)
	if err != nil {
		f.report.announce("  fixture: %v\n", err)
		return
	}
	f.report.announce("  fixture %s\n", path)
}
