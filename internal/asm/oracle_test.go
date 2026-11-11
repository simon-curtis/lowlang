package asm_test

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"lowlang/internal/asm"
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
	text       int
	size       int
	elapsed    time.Duration
}

type oracleRunner struct {
	census  *census
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
	opts := asm.Options{AlignBranches: entry.platform == target.Linux}
	start := time.Now()
	object, err := asm.Assemble(source, opts)
	result.elapsed = time.Since(start)
	if err != nil {
		result.parseError = err
		r.record(result, source)
		return
	}
	counts := collectCensus(source, object)
	r.census.merge(counts)
	r.compareAll(dir, source, object, &result)
	r.record(result, source)
}

func (r *oracleRunner) compareAll(dir, source string, object *asm.Object, result *fileResult) {
	variants := [][]string{nil}
	if result.entry.platform == target.Linux {
		variants = [][]string{{branchFlag}, nil}
	}
	fed := asSource(source, result.entry.platform)
	for _, flags := range variants {
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
		result.sections, result.text = c.checked, c.text
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
	if _, err := exec.LookPath("as"); err != nil {
		t.Skip("GNU as is not on PATH")
	}
	entries, err := corpusEntries()
	if err != nil {
		t.Fatal(err)
	}
	runner := &oracleRunner{census: newCensus()}
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
	var skipped, parseErrors, asErrors, matched, mismatched, sections, text, totalBytes int
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
		text += res.text
	}
	compiled := len(r.results) - skipped
	t.Logf("corpus: %d entries, %d compiled, %d skipped (compile failures); %.0f MB of assembly; wall %v", len(r.results), compiled, skipped, float64(totalBytes)/1e6, wall.Round(time.Millisecond))
	for _, reason := range sortedKeys(skipReasons) {
		t.Logf("  skipped x%d  %s", skipReasons[reason], reason)
	}
	t.Logf("parse errors %d, oracle errors %d, files with all non-text sections exact %d, files with mismatches %d", parseErrors, asErrors, matched, mismatched)
	t.Logf("sections compared %d; .text sections not yet encoded %d", sections, text)
	for _, line := range details {
		t.Error(line)
	}
	r.timeLargest(t)
	t.Log(r.census.summary())
	r.writeCensus(t)
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
	const runs = 5
	var best, total time.Duration
	for range runs {
		start := time.Now()
		if _, err := asm.Assemble(r.largest, asm.Options{}); err != nil {
			t.Errorf("largest file: %v", err)
			return
		}
		elapsed := time.Since(start)
		total += elapsed
		if best == 0 || elapsed < best {
			best = elapsed
		}
	}
	t.Logf("largest file: %.1f MB, Assemble best %v, mean %v over %d serial runs", float64(len(r.largest))/1e6, best.Round(time.Millisecond), (total / runs).Round(time.Millisecond), runs)
}

func (r *oracleRunner) writeCensus(t *testing.T) {
	path := os.Getenv("LOW_ASM_CENSUS")
	if path == "" {
		return
	}
	report := []byte(r.census.summary())
	if err := os.WriteFile(path, report, 0o644); err != nil {
		t.Errorf("write census: %v", err)
	}
}
