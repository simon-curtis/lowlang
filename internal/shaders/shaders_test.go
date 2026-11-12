package shaders

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

var dir = filepath.Join("..", "..", "ui", "shaders")

func TestBinariesMatchSources(t *testing.T) {
	want, err := Sum(dir)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dir, SumFile))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("shader sources or SPIR-V changed without a record; run go run ./cmd/shadergen ui/shaders\nrecorded:\n%s\ncurrent:\n%s", got, want)
	}
}

func TestBinariesValidate(t *testing.T) {
	if _, err := exec.LookPath("spirv-val"); err != nil {
		t.Skip("spirv-val is not installed")
	}
	stages, err := Stages(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, stage := range stages {
		binary := filepath.Join(dir, Binary(stage))
		out, err := exec.CommandContext(context.Background(), "spirv-val", "--target-env", TargetEnv, binary).CombinedOutput()
		if err != nil {
			t.Errorf("%s: %v\n%s", binary, err, out)
		}
	}
}
