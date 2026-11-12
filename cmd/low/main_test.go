package main

import (
	"bytes"
	"context"
	"debug/elf"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"lowlang/internal/compiler"
	"lowlang/internal/fixture"
	"lowlang/internal/fixture/fixturetest"
	"lowlang/internal/ir"
	"lowlang/internal/peakrss"
	"lowlang/internal/target"
)

func hostBinary(t *testing.T, source string) string {
	t.Helper()
	host := target.Host()
	binary := filepath.Join(t.TempDir(), host.Executable("main"))
	if err := build(context.Background(), source, binary, host); err != nil {
		t.Fatal(err)
	}
	return binary
}

func TestRun(t *testing.T) {
	fixturetest.RequireLinuxHost(t)
	for _, program := range fixturetest.Programs(t, "run") {
		sidecars := program.Sidecars(t)
		t.Run(program.Name, func(t *testing.T) {
			if sidecars.MaxRSSKB == 0 && sidecars.MaxTime == 0 {
				t.Parallel()
			}
			runFixture(t, program, sidecars)
		})
	}
}

func runFixture(t *testing.T, program fixturetest.Program, sidecars fixture.Sidecars) {
	binary := filepath.Join(t.TempDir(), "main")
	buildLinux(t, program, sidecars, binary)
	environments := [][]string{nil}
	if sidecars.SIMD {
		environments = append(environments, []string{"LOW_SIMD=sse2"})
	}
	for _, env := range environments {
		runWithEnv(t, sidecars, binary, env)
	}
	checkLinking(t, sidecars, binary)
}

func buildLinux(t *testing.T, program fixturetest.Program, sidecars fixture.Sidecars, binary string) {
	t.Helper()
	unit, err := compiler.Analyse(program.Path, target.Linux)
	if err != nil {
		t.Fatal(err)
	}
	if got := hasVectorCode(unit.Program); got != sidecars.SIMD {
		t.Errorf("program has vector code %v, but its .tags file says simd %v", got, sidecars.SIMD)
	}
	if err := buildUnit(context.Background(), unit, binary); err != nil {
		t.Fatal(err)
	}
}

func hasVectorCode(program *ir.Program) bool {
	for _, f := range program.Funcs {
		for _, b := range f.Blocks {
			for _, v := range b.Values {
				if v.Op == ir.OpVector || v.Op == ir.OpVectorScalar {
					return true
				}
			}
		}
	}
	return false
}

func runWithEnv(t *testing.T, sidecars fixture.Sidecars, binary string, env []string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	var peak int64
	var elapsed time.Duration
	compareRun(t, sidecars, func(stdout, stderr io.Writer) error {
		start := time.Now()
		defer func() { elapsed = time.Since(start) }()
		newCmd := func() *exec.Cmd {
			cmd := exec.CommandContext(ctx, binary)
			cmd.Env = append(os.Environ(), env...)
			cmd.Stdout, cmd.Stderr = stdout, stderr
			return cmd
		}
		if sidecars.MaxRSSKB == 0 {
			return newCmd().Run()
		}
		result, err := peakrss.Run(newCmd)
		peak = result.PeakKB
		return err
	}, env)
	if sidecars.MaxRSSKB > 0 {
		t.Logf("peak memory %d KB, limit %d KB", peak, sidecars.MaxRSSKB)
		if peak > sidecars.MaxRSSKB {
			t.Errorf("peak memory %d KB exceeds %d KB", peak, sidecars.MaxRSSKB)
		}
	}
	if sidecars.MaxTime > 0 && elapsed > sidecars.MaxTime {
		t.Errorf("run took %v, limit %v", elapsed, sidecars.MaxTime)
	}
}

func TestRunWindows(t *testing.T) {
	launch := windowsLauncher(t)
	programs := slices.Concat(fixturetest.Programs(t, "run"), fixturetest.Programs(t, "windows"))
	for _, program := range programs {
		sidecars := program.Sidecars(t)
		if !sidecars.Windows() {
			continue
		}
		t.Run(program.Name, func(t *testing.T) {
			t.Parallel()
			runWindowsFixture(t, launch, program, sidecars)
		})
	}
}

type launcher func(ctx context.Context, binary string) *exec.Cmd

func windowsLauncher(t *testing.T) launcher {
	t.Helper()
	if target.Host() == target.Windows {
		return func(ctx context.Context, binary string) *exec.Cmd {
			return exec.CommandContext(ctx, binary)
		}
	}
	wine, err := exec.LookPath("wine")
	if err != nil {
		fixturetest.Unavailable(t, "wine is not installed")
	}
	env := append(os.Environ(), "WINEDEBUG=-all", "DISPLAY=", "WAYLAND_DISPLAY=")
	prefix := exec.Command(wine, "wineboot", "--init")
	prefix.Env = env
	if out, err := prefix.CombinedOutput(); err != nil {
		t.Fatalf("initialise the wine prefix: %v\n%s", err, out)
	}
	return func(ctx context.Context, binary string) *exec.Cmd {
		cmd := exec.CommandContext(ctx, wine, binary)
		cmd.Env = env
		return cmd
	}
}

func runWindowsFixture(t *testing.T, launch launcher, program fixturetest.Program, sidecars fixture.Sidecars) {
	binary := filepath.Join(t.TempDir(), target.Windows.Executable("main"))
	if err := build(context.Background(), program.Path, binary, target.Windows); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := launch(ctx, binary)
	compareRun(t, sidecars, func(stdout, stderr io.Writer) error {
		cmd.Stdout, cmd.Stderr = stdout, stderr
		return cmd.Run()
	}, nil)
}

func compareRun(t *testing.T, want fixture.Sidecars, run func(stdout, stderr io.Writer) error, env []string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	runErr := run(&stdout, &stderr)
	if got := stdout.String(); got != want.Stdout {
		t.Errorf("%v stdout:\n%s\nwant:\n%s", env, got, want.Stdout)
	}
	if got := stderr.String(); got != want.Stderr {
		t.Errorf("%v stderr:\n%s\nwant:\n%s", env, got, want.Stderr)
	}
	if want.Panics != (runErr != nil) {
		t.Errorf("%v exit error %v, expected panic %v", env, runErr, want.Panics)
	}
}

func checkLinking(t *testing.T, sidecars fixture.Sidecars, binary string) {
	f, err := elf.Open(binary)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	needed, err := f.ImportedLibraries()
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(needed)
	if len(sidecars.Libraries) == 0 {
		if len(needed) > 0 || slices.ContainsFunc(f.Progs, func(p *elf.Prog) bool { return p.Type == elf.PT_INTERP }) {
			t.Errorf("expected a static binary, found shared libraries %v", needed)
		}
		return
	}
	if !slices.Equal(needed, sidecars.Libraries) {
		t.Errorf("shared libraries:\n%s\nwant:\n%s", strings.Join(needed, "\n"), strings.Join(sidecars.Libraries, "\n"))
	}
}

func TestFail(t *testing.T) {
	for _, program := range fixturetest.Programs(t, "fail") {
		t.Run(program.Name, func(t *testing.T) {
			failFixture(t, program)
		})
	}
}

func failFixture(t *testing.T, program fixturetest.Program) {
	want, ok := program.Text(t, fixture.CompileErrorExt)
	if !ok {
		t.Fatal("missing .err fixture")
	}
	_, err := compiler.Analyse(program.Path, target.Linux)
	if err == nil {
		t.Fatal("compiled without error")
	}
	got := fixturetest.Relative(t, err.Error(), program.Path)
	if got != strings.TrimSpace(want) {
		t.Errorf("error %q, want %q", got, strings.TrimSpace(want))
	}
}
