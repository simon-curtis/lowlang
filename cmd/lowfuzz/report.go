package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sync"
)

type report struct {
	mu      sync.Mutex
	out     io.Writer
	dir     string
	cases   int
	skipped int
	seen    map[string]int
}

func newReport(out io.Writer, dir string) *report {
	return &report{out: out, dir: dir, seen: map[string]int{}}
}

func (r *report) caseRan() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cases++
}

func (r *report) caseSkipped() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.skipped++
}

func (r *report) claim(signature string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seen[signature]++
	return r.seen[signature] == 1
}

func (r *report) announce(format string, args ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	fmt.Fprintf(r.out, format, args...)
}

func (r *report) save(seed int64, class, source string) (string, error) {
	if err := os.MkdirAll(r.dir, 0o755); err != nil {
		return "", fmt.Errorf("create %s: %w", r.dir, err)
	}
	path := filepath.Join(r.dir, fmt.Sprintf("%s-%d.low", class, seed))
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		return "", fmt.Errorf("write %s: %w", path, err)
	}
	return path, nil
}

func (r *report) summary() {
	r.mu.Lock()
	defer r.mu.Unlock()
	fmt.Fprintf(r.out, "%d cases, %d skipped, %d distinct findings\n", r.cases, r.skipped, len(r.seen))
	sigs := make([]string, 0, len(r.seen))
	for sig := range r.seen {
		sigs = append(sigs, sig)
	}
	slices.Sort(sigs)
	for _, sig := range sigs {
		fmt.Fprintf(r.out, "  %4d  %s\n", r.seen[sig], sig)
	}
}

func (r *report) failed() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.seen) > 0
}
