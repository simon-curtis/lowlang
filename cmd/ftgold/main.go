package main

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

//go:embed probe/ftgold.c
var probeSource []byte

const usage = `usage:
  ftgold <golden file> <font file> <mode>...    write the digest of every glyph of the font at each size and phase
  ftgold -glyphs <font file> <mode> <size> <phase>    print each glyph's size, position and digest`

var modes = map[string]bool{"unhinted": true, "light": true}

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "ftgold: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) == 5 && args[0] == "-glyphs" {
		return glyphs(ctx, args[1:])
	}
	if len(args) < 3 {
		return errors.New(usage)
	}
	return golden(ctx, args[0], args[1], args[2:])
}

func glyphs(ctx context.Context, args []string) error {
	probe, cleanup, err := buildProbe(ctx)
	if err != nil {
		return err
	}
	defer cleanup()
	probeArgs := append([]string{"-glyphs"}, args...)
	cmd := exec.CommandContext(ctx, probe, probeArgs...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}

func golden(ctx context.Context, output, font string, wanted []string) error {
	for _, mode := range wanted {
		if !modes[mode] {
			return fmt.Errorf("unknown mode %q\n%s", mode, usage)
		}
	}
	probe, cleanup, err := buildProbe(ctx)
	if err != nil {
		return err
	}
	defer cleanup()
	cases := []string{font}
	for _, mode := range wanted {
		for _, size := range sizes() {
			for phase := range 4 {
				cases = append(cases, mode, strconv.Itoa(size), strconv.Itoa(phase))
			}
		}
	}
	var out bytes.Buffer
	cmd := exec.CommandContext(ctx, probe, cases...)
	cmd.Stdout, cmd.Stderr = &out, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("probe %s: %w", font, err)
	}
	slashed := filepath.ToSlash(font)
	contents := fmt.Sprintf("font %s\n%s", slashed, out.String())
	return os.WriteFile(output, []byte(contents), 0o644)
}

func sizes() []int {
	var all []int
	for px := 8; px <= 32; px++ {
		all = append(all, px*64)
	}
	fractional := []int{683, 853, 939, 1067, 1173, 48 * 64, 72 * 64}
	return append(all, fractional...)
}

func buildProbe(ctx context.Context) (string, func(), error) {
	dir, err := os.MkdirTemp("", "ftgold")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { os.RemoveAll(dir) }
	source := filepath.Join(dir, "ftgold.c")
	if err := os.WriteFile(source, probeSource, 0o644); err != nil {
		cleanup()
		return "", nil, err
	}
	flags, err := exec.CommandContext(ctx, "pkg-config", "--cflags", "--libs", "freetype2").Output()
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("pkg-config freetype2: %w", err)
	}
	binary := filepath.Join(dir, "ftgold")
	linkFlags := strings.Fields(string(flags))
	args := append([]string{"-O2", source, "-o", binary}, linkFlags...)
	cmd := exec.CommandContext(ctx, "cc", args...)
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
	if err := cmd.Run(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("cc: %w", err)
	}
	return binary, cleanup, nil
}
