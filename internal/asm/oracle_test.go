package asm_test

import (
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"lowlang/internal/asm"
	"lowlang/internal/oracletool"
	"lowlang/internal/target"
)

const branchFlag = "-mbranches-within-32B-boundaries"

const maxMismatchLines = 3

type fileResult struct {
	entry      corpusEntry
	skipped    string
	parseError error
	asError    error
	mismatches []string
	sections   int
	size       int
}

type oracleRunner struct {
	mu      sync.Mutex
	largest string
	results []fileResult
}

func (r *oracleRunner) work(dir string, entry corpusEntry) {
	result := fileResult{entry: entry}
	source, err := entry.generate()
	if err != nil {
		firstLine, _, _ := strings.Cut(err.Error(), "\n")
		result.skipped = truncate(firstLine, 100)
		r.record(result, "")
		return
	}
	result.size = len(source)
	r.compareAll(dir, source, &result)
	r.record(result, source)
}

func (r *oracleRunner) compareAll(dir, source string, result *fileResult) {
	variants := [][]string{nil}
	if result.entry.platform == target.Linux {
		variants = [][]string{{branchFlag}, nil}
	}
	fed := asSource(source, result.entry.platform)
	for _, flags := range variants {
		opts := asm.Options{AlignBranches: len(flags) > 0}
		object, err := asm.Assemble(source, opts)
		if err != nil {
			result.parseError = err
			return
		}
		oracle, err := runAs(dir, fed, flags)
		if err != nil {
			result.asError = err
			return
		}
		c := newComparison(object, oracle, result.entry.platform)
		c.run()
		for _, m := range c.mismatches {
			result.mismatches = append(result.mismatches, fmt.Sprintf("as %v: %s", flags, m))
		}
		result.sections += c.checked
	}
}

func (r *oracleRunner) record(result fileResult, source string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.results = append(r.results, result)
	if len(source) > len(r.largest) {
		r.largest = source
	}
}

func TestOracle(t *testing.T) {
	if testing.Short() {
		t.Skip("oracle corpus is slow")
	}
	oracletool.Require(t, "as", oracletool.GNU)
	entries, err := corpusEntries()
	if err != nil {
		t.Fatal(err)
	}
	runner := &oracleRunner{}
	started := time.Now()
	runner.runAll(t, entries)
	wall := time.Since(started)
	runner.summarise(t, wall)
}

func (r *oracleRunner) runAll(t *testing.T, entries []corpusEntry) {
	jobs := make(chan corpusEntry)
	var wg sync.WaitGroup
	for range runtime.GOMAXPROCS(0) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			dir, err := os.MkdirTemp("", "asm-oracle-")
			if err != nil {
				t.Errorf("temp dir: %v", err)
				return
			}
			defer os.RemoveAll(dir)
			for entry := range jobs {
				r.work(dir, entry)
			}
		}()
	}
	for _, entry := range entries {
		jobs <- entry
	}
	close(jobs)
	wg.Wait()
}

func (r *oracleRunner) summarise(t *testing.T, wall time.Duration) {
	sort.Slice(r.results, func(i, j int) bool { return r.results[i].entry.label() < r.results[j].entry.label() })
	var skipped, parseErrors, asErrors, matched, mismatched, sections, totalBytes int
	skipReasons := map[string]int{}
	var details []string
	for _, res := range r.results {
		totalBytes += res.size
		switch {
		case res.skipped != "":
			skipped++
			skipReasons[fmt.Sprintf("%s: %s", res.entry.platform, res.skipped)]++
		case res.parseError != nil:
			parseErrors++
			details = append(details, fmt.Sprintf("%s: parse: %v", res.entry.label(), res.parseError))
		case res.asError != nil:
			asErrors++
			details = append(details, fmt.Sprintf("%s: oracle: %v", res.entry.label(), res.asError))
		case len(res.mismatches) > 0:
			mismatched++
			formatted := formatMismatches(res)
			details = append(details, formatted)
		default:
			matched++
		}
		sections += res.sections
	}
	compiled := len(r.results) - skipped
	t.Logf("corpus: %d entries, %d compiled, %d skipped (compile failures); %.0f MB of assembly; wall %v", len(r.results), compiled, skipped, float64(totalBytes)/1e6, wall.Round(time.Millisecond))
	for _, reason := range sortedKeys(skipReasons) {
		t.Logf("  skipped x%d  %s", skipReasons[reason], reason)
	}
	t.Logf("parse errors %d, oracle errors %d, files matching as exactly %d, files with mismatches %d", parseErrors, asErrors, matched, mismatched)
	t.Logf("sections compared %d", sections)
	for _, line := range details {
		t.Error(line)
	}
	r.timeLargest(t)
}

func sortedKeys(counts map[string]int) []string {
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func formatMismatches(res fileResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s: %d mismatches", res.entry.label(), len(res.mismatches))
	for i, m := range res.mismatches {
		if i == maxMismatchLines {
			break
		}
		fmt.Fprintf(&b, "\n    %s", m)
	}
	return b.String()
}

func (r *oracleRunner) timeLargest(t *testing.T) {
	for _, align := range []bool{false, true} {
		best, mean := timeAssemble(t, r.largest, asm.Options{AlignBranches: align})
		t.Logf("largest file: %.1f MB, AlignBranches %v, Assemble best %v, mean %v", float64(len(r.largest))/1e6, align, best.Round(time.Millisecond), mean.Round(time.Millisecond))
	}
}

func timeAssemble(t *testing.T, source string, opts asm.Options) (best, mean time.Duration) {
	const runs = 5
	var total time.Duration
	for range runs {
		runtime.GC()
		start := time.Now()
		if _, err := asm.Assemble(source, opts); err != nil {
			t.Errorf("largest file: %v", err)
			return 0, 0
		}
		elapsed := time.Since(start)
		total += elapsed
		if best == 0 || elapsed < best {
			best = elapsed
		}
	}
	return best, total / runs
}
