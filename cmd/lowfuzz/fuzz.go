package main

import (
	"context"
	"errors"
	"fmt"
	"io"
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
	settings
	cfg    gen.Config
	runner *oracle.Runner
	report *report
}

func newFuzzer(s settings, out io.Writer) *fuzzer {
	cfg := gen.DefaultConfig
	cfg.Panics = s.panics
	return &fuzzer{
		settings: s,
		cfg:      cfg,
		runner:   &oracle.Runner{Timeout: s.timeout},
		report:   newReport(out, s.dir),
	}
}

func (f *fuzzer) run(ctx context.Context) {
	var next atomic.Int64
	var wg sync.WaitGroup
	for range f.workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ctx.Err() == nil {
				n := next.Add(1) - 1
				if f.cases > 0 && n >= int64(f.cases) {
					return
				}
				f.runCase(ctx, f.seed+n)
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
		f.claimAll(ctx, seed, program, []oracle.Finding{finding})
		return
	}
	findings := f.runner.Check(ctx, program.String(), outcome)
	if ctx.Err() != nil {
		return
	}
	f.report.caseRan()
	f.claimAll(ctx, seed, program, findings)
}

func (f *fuzzer) claimAll(ctx context.Context, seed int64, program *model.Program, findings []oracle.Finding) {
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
	location := f.saved(seed, finding.Class, reduced)
	f.report.announce("seed %d: %s\n  %d -> %d bytes, %s\n", seed, signature, len(program.String()), len(reduced.String()), location)
	f.writeFixture(reduced)
}

func (f *fuzzer) saved(seed int64, class string, program *model.Program) string {
	path, err := f.report.save(seed, class, program.String())
	if err != nil {
		return fmt.Sprintf("not saved: %v", err)
	}
	return path
}

func (f *fuzzer) writeFixture(program *model.Program) {
	if f.fixtures == "" {
		return
	}
	outcome, err := model.Run(program, f.budget)
	if err != nil {
		f.report.announce("  fixture: %v\n", err)
		return
	}
	path, err := corpus.FromProgram(program, outcome).Write(f.fixtures)
	if err != nil {
		f.report.announce("  fixture: %v\n", err)
		return
	}
	f.report.announce("  fixture %s\n", path)
}
