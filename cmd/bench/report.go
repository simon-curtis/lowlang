package main

import (
	"fmt"
	"io"
	"slices"
	"strings"
	"time"
)

func report(w io.Writer, benchmarks []string, languages []language, results map[key]*entry) {
	baseline := languages[0].name
	if slices.ContainsFunc(languages, func(l language) bool { return l.name == "go" }) {
		baseline = "go"
	}
	fmt.Fprintf(w, "| Benchmark | Language | Build | Binary | Median | Min | CPU | Peak RSS | vs %s |\n", baseline)
	fmt.Fprintln(w, "|---|---|--:|--:|--:|--:|--:|--:|--:|")
	var failures []string
	for _, bench := range benchmarks {
		base := results[key{bench, baseline}]
		for _, lang := range languages {
			e := results[key{bench, lang.name}]
			fmt.Fprintln(w, e.row(bench, lang.name, base))
			if e.err != nil {
				failures = append(failures, fmt.Sprintf("%s/%s: %v", bench, lang.name, e.err))
			}
		}
	}
	for _, f := range failures {
		fmt.Fprintf(w, "\n%s\n", f)
	}
}

func (e *entry) row(bench, lang string, base *entry) string {
	if e.err != nil {
		return fmt.Sprintf("| %s | %s | failed | | | | | | |", bench, lang)
	}
	walls := e.walls()
	cpus := e.column(func(s sample) time.Duration { return s.cpu })
	cells := []string{
		bench,
		lang,
		duration(e.build),
		bytesize(e.size),
		duration(median(walls)),
		duration(slices.Min(walls)),
		duration(median(cpus)),
		bytesize(e.peakRSS()),
		e.ratio(base),
	}
	return fmt.Sprintf("| %s |", strings.Join(cells, " | "))
}

func (e *entry) walls() []time.Duration {
	return e.column(func(s sample) time.Duration { return s.wall })
}

func (e *entry) column(pick func(sample) time.Duration) []time.Duration {
	values := make([]time.Duration, len(e.samples))
	for i, s := range e.samples {
		values[i] = pick(s)
	}
	return values
}

func (e *entry) peakRSS() int64 {
	var peak int64
	for _, s := range e.samples {
		peak = max(peak, s.rss)
	}
	return peak
}

func (e *entry) ratio(base *entry) string {
	if base == nil || base.err != nil {
		return "-"
	}
	mine := median(e.walls())
	theirs := median(base.walls())
	return fmt.Sprintf("%.2fx", float64(mine)/float64(theirs))
}

func median(values []time.Duration) time.Duration {
	sorted := slices.Clone(values)
	slices.Sort(sorted)
	mid := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[mid]
	}
	return (sorted[mid-1] + sorted[mid]) / 2
}

func duration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%.1f ms", float64(d)/float64(time.Millisecond))
	}
	return fmt.Sprintf("%.2f s", d.Seconds())
}

func bytesize(n int64) string {
	const mb = 1 << 20
	if n < mb {
		return fmt.Sprintf("%.0f KB", float64(n)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(n)/mb)
}
