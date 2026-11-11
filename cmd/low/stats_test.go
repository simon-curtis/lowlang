package main

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"lowlang/internal/compiler"
	"lowlang/internal/ir"
	"lowlang/internal/target"
)

func statsPrograms(t *testing.T) []string {
	t.Helper()
	paths := benchmarkPrograms(t)
	for _, path := range fixtures(t, "run") {
		if strings.HasPrefix(filepath.Base(path), "std_") {
			paths = append(paths, path)
		}
	}
	return paths
}

func programName(path string) string {
	name := strings.TrimSuffix(filepath.Base(path), ".low")
	if name == "main" {
		return filepath.Base(filepath.Dir(path))
	}
	return name
}

func TestMemoryStats(t *testing.T) {
	if os.Getenv("LOW_STATS") != "1" {
		t.Skip("set LOW_STATS=1 to print memory statistics")
	}
	var table strings.Builder
	fmt.Fprintf(&table, "%-28s %7s %7s %7s %6s %6s %6s %6s\n",
		"program", "loads", "redund", "r+calls", "slots", "promo", "chk", "alloc")
	var total ir.Stats
	direct := 0
	reasons := map[string]int{}
	for _, path := range statsPrograms(t) {
		unit, err := compiler.Analyse(path, target.Linux)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		s := unit.Program.MemoryStats()
		total = total.Add(s)
		sites := unit.Program.CallSites()
		direct += sites.Direct
		for reason, n := range sites.Reasons {
			reasons[reason] += n
		}
		fmt.Fprintf(&table, "%-28s %7d %7d %7d %6d %6d %6d %6d\n",
			programName(path), s.Loads, s.RedundantLoads, s.RedundantAcrossCalls, s.Slots, s.PromotableSlots,
			s.ChecksAffine+s.ChecksLoadedLimit+s.ChecksOther,
			s.AllocsOneWord+s.AllocsTwoToFourWords+s.AllocsFiveToThirtyTwo+s.AllocsLarger+s.AllocsVariable)
	}
	var calls strings.Builder
	fmt.Fprintf(&calls, "direct calls %d, not inlined by reason:", direct)
	for _, reason := range slices.Sorted(maps.Keys(reasons)) {
		fmt.Fprintf(&calls, " %s %d", reason, reasons[reason])
	}
	t.Logf("\n%s\ntotal\n%s%s", table.String(), total, calls.String())
}
