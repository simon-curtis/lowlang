package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"time"
)

type settings struct {
	seed        int64
	cases       int
	duration    time.Duration
	workers     int
	budget      int
	dir         string
	timeout     time.Duration
	panics      bool
	reduce      bool
	reduceTests int
	fixtures    string
}

func main() {
	var s settings
	flag.Int64Var(&s.seed, "seed", 0, "first seed; 0 picks one from the clock")
	flag.IntVar(&s.cases, "n", 0, "number of cases; 0 runs until stopped")
	flag.DurationVar(&s.duration, "time", 0, "stop after this long; 0 runs until stopped")
	flag.IntVar(&s.workers, "workers", max(1, runtime.NumCPU()/2), "parallel workers")
	flag.IntVar(&s.budget, "budget", 2_000_000, "reference interpreter step limit per program")
	flag.StringVar(&s.dir, "out", filepath.Join(os.TempDir(), "lowfuzz"), "directory for failing programs")
	flag.DurationVar(&s.timeout, "timeout", 10*time.Second, "run limit per compiled program")
	flag.BoolVar(&s.panics, "panics", false, "allow programs that divide by zero")
	flag.BoolVar(&s.reduce, "reduce", true, "minimise each new finding")
	flag.IntVar(&s.reduceTests, "reduce-tests", 600, "compiler runs allowed per reduction")
	flag.StringVar(&s.fixtures, "fixtures", "", "directory for run fixtures of minimised findings")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if s.duration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.duration)
		defer cancel()
	}
	if s.seed == 0 {
		s.seed = time.Now().UnixNano()
	}

	fuzzer := newFuzzer(s, os.Stdout)
	fmt.Printf("first seed %d, %d workers\n", s.seed, s.workers)
	fuzzer.run(ctx)
	fuzzer.report.summary()
	if fuzzer.report.failed() {
		os.Exit(1)
	}
}
