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

func TestPollSavesEveryFloatRegister(t *testing.T) {
	_, afterLabel, found := strings.Cut(scheduler, "\nrt_poll:\n")
	if !found {
		t.Fatal("scheduler does not define rt_poll")
	}
	body, _, found := strings.Cut(afterLabel, "\nrt_poll_run:")
	if !found {
		t.Fatal("scheduler does not define rt_poll_run after rt_poll")
	}
	for _, reg := range floatRegs {
		save := fmt.Sprintf(", %s\n", reg)
		restore := fmt.Sprintf("movq %s, [rsp", reg)
		if !strings.Contains(body, save) || !strings.Contains(body, restore) {
			t.Errorf("rt_poll does not save and restore %s", reg)
		}
	}
}
