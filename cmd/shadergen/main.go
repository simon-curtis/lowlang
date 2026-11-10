package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"lowlang/internal/shaders"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: shadergen <shader directory>")
		os.Exit(2)
	}
	if err := run(context.Background(), os.Args[1]); err != nil {
		fmt.Fprintf(os.Stderr, "shadergen: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, dir string) error {
	stages, err := shaders.Stages(dir)
	if err != nil {
		return err
	}
	for _, stage := range stages {
		source := filepath.Join(dir, stage)
		output := fmt.Sprintf("%s.spv", source)
		cmd := exec.CommandContext(ctx, "glslc", "-O", "--target-env=vulkan1.2", source, "-o", output)
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("glslc %s: %w", stage, err)
		}
	}
	sum, err := shaders.Sum(dir)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, shaders.SumFile), []byte(sum), 0o644)
}
