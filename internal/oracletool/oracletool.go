package oracletool

import (
	"os/exec"
	"slices"
	"strings"
	"testing"
)

const (
	GNU  = "2.47"
	LLVM = "23.1.4"

	BranchFlag = "-mbranches-within-32B-boundaries"
)

func Require(t testing.TB, tool, version string) {
	t.Helper()
	out, err := exec.Command(tool, "--version").Output()
	if err != nil {
		t.Skipf("%s is not available: %v", tool, err)
	}
	first, _, _ := strings.Cut(string(out), "\n")
	fields := strings.Fields(first)
	if !slices.Contains(fields, version) {
		t.Skipf("the %s oracle is pinned to version %s; found %q", tool, version, first)
	}
}
