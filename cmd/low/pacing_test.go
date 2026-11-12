package main

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSparseSurvivorsDoNotTriggerExtraCollections(t *testing.T) {
	binary := hostBinary(t, filepath.Join("..", "..", "testdata", "run", "heap_sparse_pacing.low"))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	start := time.Now()
	out, err := exec.CommandContext(ctx, binary).Output()
	elapsed := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(out)); got != "30156250" {
		t.Fatalf("output %q", got)
	}
	if elapsed > 10*time.Second {
		t.Errorf("10M allocations with sparse survivors took %v", elapsed)
	}
}
