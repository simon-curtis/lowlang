package amd64

import (
	"fmt"
	"strings"
	"testing"
)

func TestHotRuntimeDefinesPatchSymbols(t *testing.T) {
	for _, symbol := range []string{PatchListSymbol, PatchCountSymbol} {
		global := fmt.Sprintf("\t.globl %s\n%s:\n", symbol, symbol)
		if !strings.Contains(hotRuntime, global) {
			t.Errorf("hot runtime does not define %s", symbol)
		}
	}
}

func TestEntryMacroTakesTheExportedSymbol(t *testing.T) {
	for name, header := range map[string]string{"linux": linuxHeader, "windows": windowsHeader} {
		if !strings.Contains(header, ".macro PROGRAM_ENTRY entry\n") {
			t.Errorf("%s header: PROGRAM_ENTRY does not take the entry symbol", name)
		}
	}
}
