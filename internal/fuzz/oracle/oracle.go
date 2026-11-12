package oracle

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strings"
	"time"

	"lowlang/internal/amd64"
	"lowlang/internal/compiler"
	"lowlang/internal/fuzz/model"
	"lowlang/internal/target"
)

const (
	ClassICE          = "ice"
	ClassReject       = "reject"
	ClassLink         = "link"
	ClassTimeout      = "timeout"
	ClassHarness      = "harness"
	ClassGenerator    = "generator"
	ClassReference    = "reference-mismatch"
	ClassDifferential = "release-dev-mismatch"
)

type Mode int

const (
	Release Mode = iota
	Dev
)

func (m Mode) String() string {
	if m == Dev {
		return "dev"
	}
	return "release"
}

type Finding struct {
	Class  string
	Detail string
	Source string
}

func (f Finding) Signature() string {
	detail := digits.ReplaceAllString(firstLine(f.Detail), "#")
	return fmt.Sprintf("%s:%s", f.Class, detail)
}

var digits = regexp.MustCompile(`\b\d+\b`)

type Result struct {
	Stdout string
	Stderr string
	Exit   int
	Signal string
}

func Expected(o model.Outcome) Result {
	return Result{Stdout: o.Stdout, Stderr: o.Stderr(), Exit: o.ExitCode()}
}

type Runner struct {
	Timeout time.Duration
}

func (r *Runner) Reproduces(signature string, budget int) func(context.Context, *model.Program) bool {
	return func(ctx context.Context, p *model.Program) bool {
		outcome, err := model.Run(p, budget)
		if err != nil {
			return false
		}
		for _, f := range r.Check(ctx, p.String(), outcome) {
			if f.Signature() == signature {
				return true
			}
		}
		return false
	}
}

type stageError struct {
	class  string
	detail string
}

func (e stageError) Error() string {
	return fmt.Sprintf("%s: %s", e.class, e.detail)
}

func (r *Runner) Check(ctx context.Context, source string, expected model.Outcome) []Finding {
	want := Expected(expected)
	var findings []Finding
	var ran []modeResult
	for _, mode := range []Mode{Release, Dev} {
		got, err := r.execute(ctx, source, mode)
		if err != nil {
			findings = append(findings, failure(mode, err, source))
			continue
		}
		ran = append(ran, modeResult{mode: mode, result: got})
	}
	if len(ran) == 2 {
		if diff := describe(ran[0].result, ran[1].result); diff != "" {
			findings = append(findings, Finding{Class: ClassDifferential, Detail: diff, Source: source})
		}
	}
	for _, m := range ran {
		if diff := describe(want, m.result); diff != "" {
			detail := fmt.Sprintf("%s: %s", m.mode, diff)
			findings = append(findings, Finding{Class: ClassReference, Detail: detail, Source: source})
		}
	}
	return findings
}

type modeResult struct {
	mode   Mode
	result Result
}

func failure(mode Mode, err error, source string) Finding {
	var stage stageError
	if errors.As(err, &stage) {
		return Finding{Class: stage.class, Detail: fmt.Sprintf("%s: %s", mode, stage.detail), Source: source}
	}
	return Finding{Class: ClassHarness, Detail: fmt.Sprintf("%s: %v", mode, err), Source: source}
}

func (r *Runner) execute(ctx context.Context, source string, mode Mode) (Result, error) {
	dir, err := os.MkdirTemp("", "lowfuzz-")
	if err != nil {
		return Result{}, err
	}
	defer os.RemoveAll(dir)
	binary := filepath.Join(dir, "main")
	if err := r.build(ctx, dir, binary, source, mode); err != nil {
		return Result{}, err
	}
	return r.run(ctx, binary)
}

func (r *Runner) build(ctx context.Context, dir, binary, source string, mode Mode) (err error) {
	path := filepath.Join(dir, "main.low")
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		return err
	}
	defer func() {
		if rec := recover(); rec != nil {
			err = stageError{class: ClassICE, detail: fmt.Sprintf("%v at %s", rec, topFrame(string(debug.Stack())))}
		}
	}()
	analyse := compiler.Analyse
	if mode == Dev {
		analyse = compiler.AnalyseDev
	}
	unit, err := analyse(path, target.Linux)
	if err != nil {
		return stageError{class: ClassReject, detail: strings.ReplaceAll(err.Error(), dir+"/", "")}
	}
	asm := amd64.Generate(unit.Program, unit.Platform, nil).Asm
	if err := compiler.Link(ctx, asm, binary, unit); err != nil {
		return stageError{class: ClassLink, detail: err.Error()}
	}
	return nil
}

func (r *Runner) run(ctx context.Context, binary string) (Result, error) {
	runCtx, cancel := context.WithTimeout(ctx, r.Timeout)
	defer cancel()
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(runCtx, binary)
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if runCtx.Err() != nil && ctx.Err() == nil {
		return Result{}, stageError{class: ClassTimeout, detail: fmt.Sprintf("no exit within %s", r.Timeout)}
	}
	result := Result{Stdout: stdout.String(), Stderr: stderr.String()}
	var exit *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exit):
		result.Exit = exit.ExitCode()
		if result.Exit < 0 {
			result.Signal = exit.ProcessState.String()
		}
	default:
		return Result{}, err
	}
	return result, nil
}

func describe(want, got Result) string {
	switch {
	case want.Signal != got.Signal:
		return fmt.Sprintf("signal %q, want %q", got.Signal, want.Signal)
	case want.Exit != got.Exit:
		return fmt.Sprintf("exit %d, want %d", got.Exit, want.Exit)
	case want.Stdout != got.Stdout:
		return fmt.Sprintf("stdout %s", firstDifference(want.Stdout, got.Stdout))
	case want.Stderr != got.Stderr:
		return fmt.Sprintf("stderr %q, want %q", got.Stderr, want.Stderr)
	}
	return ""
}

func firstDifference(want, got string) string {
	wantLines := strings.Split(want, "\n")
	gotLines := strings.Split(got, "\n")
	for i := range max(len(wantLines), len(gotLines)) {
		var w, g string
		if i < len(wantLines) {
			w = wantLines[i]
		}
		if i < len(gotLines) {
			g = gotLines[i]
		}
		if w != g {
			return fmt.Sprintf("line %d is %q, want %q", i+1, g, w)
		}
	}
	return "identical"
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}

func topFrame(stack string) string {
	for _, line := range strings.Split(stack, "\n") {
		if strings.HasPrefix(line, "lowlang/internal/") && !strings.HasPrefix(line, "lowlang/internal/fuzz/") {
			name, _, _ := strings.Cut(line, "(")
			return name
		}
	}
	return "unknown"
}
