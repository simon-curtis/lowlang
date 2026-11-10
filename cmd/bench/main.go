package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

func main() {
	runs := flag.Int("runs", 5, "timed runs per program")
	only := flag.String("only", "", "comma-separated benchmarks to run")
	langs := flag.String("langs", "low,go,bun", "comma-separated languages to compare")
	low := flag.String("low", "", "prebuilt low compiler; built from ./cmd/low when empty")
	timeout := flag.Duration("timeout", 2*time.Minute, "limit for each build and run")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	cfg := config{
		runs:    *runs,
		only:    split(*only),
		langs:   split(*langs),
		low:     *low,
		timeout: *timeout,
	}
	err := run(ctx, cfg)
	stop()
	if err != nil {
		fmt.Fprintf(os.Stderr, "bench: %v\n", err)
		os.Exit(1)
	}
}

type config struct {
	runs    int
	only    []string
	langs   []string
	low     string
	timeout time.Duration
}

func split(list string) []string {
	var items []string
	for item := range strings.SplitSeq(list, ",") {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			items = append(items, trimmed)
		}
	}
	return items
}

func run(ctx context.Context, cfg config) error {
	if cfg.runs < 1 {
		return fmt.Errorf("-runs must be at least 1")
	}
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	benchmarks, err := discover(filepath.Join(root, "bench"), cfg.only)
	if err != nil {
		return err
	}
	work, err := os.MkdirTemp("", "low-bench-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	s := &suite{
		root:    root,
		work:    work,
		runs:    cfg.runs,
		timeout: cfg.timeout,
		log:     os.Stderr,
	}
	languages, err := s.prepare(ctx, cfg, benchmarks)
	if err != nil {
		return err
	}
	results := s.measure(ctx, benchmarks, languages)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	report(os.Stdout, benchmarks, languages, results)
	return nil
}

func discover(dir string, only []string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() && (len(only) == 0 || slices.Contains(only, e.Name())) {
			names = append(names, e.Name())
		}
	}
	for _, name := range only {
		if !slices.Contains(names, name) {
			return nil, fmt.Errorf("no benchmark %q in %s", name, dir)
		}
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("no benchmarks in %s", dir)
	}
	return names, nil
}

func (s *suite) prepare(ctx context.Context, cfg config, benchmarks []string) ([]language, error) {
	var languages []language
	for _, name := range cfg.langs {
		lang, err := s.language(ctx, name, cfg.low, benchmarks)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		languages = append(languages, lang)
	}
	return languages, nil
}

func (s *suite) language(ctx context.Context, name, low string, benchmarks []string) (language, error) {
	switch name {
	case "low":
		compiler, err := s.lowCompiler(ctx, low)
		if err != nil {
			return language{}, err
		}
		return language{name: name, source: "main.low", build: s.buildLow(compiler)}, nil
	case "go":
		if err := s.warmGo(ctx, benchmarks); err != nil {
			return language{}, err
		}
		return language{name: name, source: "main.go", build: s.buildGo}, nil
	case "bun":
		bun, err := exec.LookPath("bun")
		if err != nil {
			return language{}, err
		}
		return language{name: name, source: "main.ts", build: s.buildBun(bun)}, nil
	}
	return language{}, fmt.Errorf("unknown language; expected low, go or bun")
}
