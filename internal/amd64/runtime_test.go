package amd64

import (
	"fmt"
	"strings"
	"testing"

	"lowlang/internal/ir"
)

func definesLabel(source, label string) bool {
	return strings.Contains(source, fmt.Sprintf("\n%s:", label))
}

func TestRuntimeEntriesAreDefined(t *testing.T) {
	sources := map[string]string{
		"linux":   fmt.Sprintf("%s\n%s\n%s\n%s", runtime, scheduler, collector, linuxRuntime),
		"windows": fmt.Sprintf("%s\n%s\n%s\n%s", runtime, scheduler, collector, windowsRuntime),
	}
	for _, entry := range ir.RuntimeEntries() {
		symbols := []string{entry.Symbol}
		if entry.Conv == ir.CallGrow {
			symbols = []string{fmt.Sprintf("%s_scan", entry.Symbol), fmt.Sprintf("%s_noscan", entry.Symbol)}
		}
		for system, source := range sources {
			for _, symbol := range symbols {
				if !definesLabel(source, symbol) {
					t.Errorf("%s runtime does not define %s", system, symbol)
				}
			}
		}
	}
}
