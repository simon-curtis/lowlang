package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"lowlang/internal/peakrss"
)

type suite struct {
	root    string
	work    string
	runs    int
	timeout time.Duration
	log     io.Writer
}

type language struct {
	name   string
	source string
	build  func(ctx context.Context, source, output string) error
}

type sample struct {
	wall time.Duration
	cpu  time.Duration
	rss  int64
}

type entry struct {
	build   time.Duration
	size    int64
	samples []sample
	err     error
}

type key struct {
	bench string
	lang  string
}

func (s *suite) goEnv() []string {
	cache := filepath.Join(s.work, "gocache")
	return append(os.Environ(), fmt.Sprintf("GOCACHE=%s", cache), "CGO_ENABLED=0")
}

func (s *suite) command(ctx context.Context, dir string, env []string, name string, args ...string) error {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %w\n%s", name, strings.Join(args, " "), err, bytes.TrimSpace(out))
	}
	return nil
}

func (s *suite) lowCompiler(ctx context.Context, prebuilt string) (string, error) {
	if prebuilt != "" {
		return filepath.Abs(prebuilt)
	}
	compiler := filepath.Join(s.work, "low")
	fmt.Fprintln(s.log, "building low compiler")
	err := s.command(ctx, s.root, os.Environ(), "go", "build", "-o", compiler, "./cmd/low")
	return compiler, err
}

func (s *suite) buildLow(compiler string) func(context.Context, string, string) error {
	return func(ctx context.Context, source, output string) error {
		return s.command(ctx, s.work, os.Environ(), compiler, "build", source, "-o", output)
	}
}

func (s *suite) warmGo(ctx context.Context, benchmarks []string) error {
	fmt.Fprintln(s.log, "caching Go standard library")
	args := []string{"list", "-deps", "-f", "{{if .Standard}}{{.ImportPath}}{{end}}"}
	for _, name := range benchmarks {
		args = append(args, s.goPackage(name))
	}
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	list := exec.CommandContext(ctx, "go", args...)
	list.Dir = s.root
	list.Env = s.goEnv()
	out, err := list.Output()
	if err != nil {
		return fmt.Errorf("go list: %w", err)
	}
	build := append([]string{"build"}, strings.Fields(string(out))...)
	return s.command(ctx, s.root, s.goEnv(), "go", build...)
}

func (s *suite) goPackage(bench string) string {
	return fmt.Sprintf("./%s", filepath.ToSlash(filepath.Join("bench", bench)))
}

func (s *suite) buildGo(ctx context.Context, source, output string) error {
	pkg := filepath.Dir(source)
	return s.command(ctx, s.root, s.goEnv(), "go", "build", "-o", output, pkg)
}

func (s *suite) buildBun(bun string) func(context.Context, string, string) error {
	return func(ctx context.Context, source, output string) error {
		return s.command(ctx, s.work, os.Environ(), bun, "build", "--compile", "--minify", source, "--outfile", output)
	}
}

func (s *suite) measure(ctx context.Context, benchmarks []string, languages []language) map[key]*entry {
	results := map[key]*entry{}
	for _, bench := range benchmarks {
		if ctx.Err() != nil {
			break
		}
		s.measureBenchmark(ctx, bench, languages, results)
	}
	return results
}

func (s *suite) measureBenchmark(ctx context.Context, bench string, languages []language, results map[key]*entry) {
	binaries := map[string]string{}
	for _, lang := range languages {
		e, binary := s.compile(ctx, bench, lang)
		results[key{bench, lang.name}] = e
		binaries[lang.name] = binary
	}
	s.verify(ctx, bench, languages, results, binaries)
	for run := range s.runs {
		fmt.Fprintf(s.log, "%s: run %d/%d\n", bench, run+1, s.runs)
		s.sampleAll(ctx, bench, languages, results, binaries)
	}
}

func (s *suite) compile(ctx context.Context, bench string, lang language) (*entry, string) {
	source := filepath.Join(s.root, "bench", bench, lang.source)
	if _, err := os.Stat(source); err != nil {
		return &entry{err: err}, ""
	}
	binary := filepath.Join(s.work, fmt.Sprintf("%s-%s", bench, lang.name))
	fmt.Fprintf(s.log, "%s: building %s\n", bench, lang.name)
	start := time.Now()
	err := lang.build(ctx, source, binary)
	elapsed := time.Since(start)
	if err != nil {
		return &entry{err: err}, ""
	}
	info, err := os.Stat(binary)
	if err != nil {
		return &entry{err: err}, ""
	}
	return &entry{build: elapsed, size: info.Size()}, binary
}

func (s *suite) verify(ctx context.Context, bench string, languages []language, results map[key]*entry, binaries map[string]string) {
	var reference []byte
	var referenceLang string
	for _, lang := range languages {
		e := results[key{bench, lang.name}]
		if e.err != nil {
			continue
		}
		var out bytes.Buffer
		if _, err := s.execute(ctx, binaries[lang.name], &out); err != nil {
			e.err = err
			continue
		}
		if referenceLang == "" {
			reference, referenceLang = out.Bytes(), lang.name
			continue
		}
		if !bytes.Equal(out.Bytes(), reference) {
			e.err = fmt.Errorf("output differs from %s:\n%s", referenceLang, out.Bytes())
		}
	}
}

func (s *suite) sampleAll(ctx context.Context, bench string, languages []language, results map[key]*entry, binaries map[string]string) {
	for _, lang := range languages {
		e := results[key{bench, lang.name}]
		if e.err != nil {
			continue
		}
		sm, err := s.execute(ctx, binaries[lang.name], io.Discard)
		if err != nil {
			e.err = err
			continue
		}
		e.samples = append(e.samples, sm)
	}
}

func (s *suite) execute(ctx context.Context, binary string, stdout io.Writer) (sample, error) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	var stderr bytes.Buffer
	start := time.Now()
	result, err := peakrss.Run(func() *exec.Cmd {
		cmd := exec.CommandContext(ctx, binary)
		cmd.Dir = s.work
		cmd.Stdout = stdout
		cmd.Stderr = &stderr
		return cmd
	})
	wall := time.Since(start)
	if err != nil {
		return sample{}, fmt.Errorf("%s: %w\n%s", filepath.Base(binary), err, bytes.TrimSpace(stderr.Bytes()))
	}
	state := result.Cmd.ProcessState
	cpu := state.UserTime() + state.SystemTime()
	return sample{wall: wall, cpu: cpu, rss: result.PeakKB * 1024}, nil
}
