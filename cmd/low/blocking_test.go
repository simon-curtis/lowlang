package main

import (
	"bufio"
	"context"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lowlang/internal/target"
)

func TestReadiedGreenThreadRunsWhileProducerComputes(t *testing.T) {
	source := filepath.Join("..", "..", "testdata", "run", "sched_poll_reply.low")
	binary := filepath.Join(t.TempDir(), "main")
	if err := build(context.Background(), source, binary, target.Linux); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	start := time.Now()
	out, err := exec.CommandContext(ctx, binary).Output()
	elapsed := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(out)); got != "total 89700" {
		t.Fatalf("output %q", got)
	}
	if elapsed > time.Second {
		t.Errorf("300 request and reply rounds took %v", elapsed)
	}
}

func TestBlockedReadDoesNotStallCollection(t *testing.T) {
	source := filepath.Join("..", "..", "testdata", "run", "std_sched_blocked_read.low")
	binary := filepath.Join(t.TempDir(), "main")
	if err := build(context.Background(), source, binary, target.Linux); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name    string
		release func(w io.WriteCloser) error
		reader  string
	}{
		{"eof", func(w io.WriteCloser) error { return w.Close() }, "reader 0"},
		{"data", func(w io.WriteCloser) error {
			_, err := w.Write([]byte("xyz"))
			return err
		}, "reader 3"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, binary)
			stdin, err := cmd.StdinPipe()
			if err != nil {
				t.Fatal(err)
			}
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			start := time.Now()
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			defer func() {
				stdin.Close()
				cmd.Wait()
			}()
			lines := bufio.NewScanner(stdout)
			if !lines.Scan() {
				t.Fatalf("no output: %v", lines.Err())
			}
			elapsed := time.Since(start)
			if got := lines.Text(); got != "allocated 7000000" {
				t.Fatalf("first line %q", got)
			}
			if elapsed > 3*time.Second {
				t.Errorf("allocation took %v while a reader was blocked", elapsed)
			}
			if err := c.release(stdin); err != nil {
				t.Fatal(err)
			}
			if !lines.Scan() {
				t.Fatalf("reader never woke: %v", lines.Err())
			}
			if got := lines.Text(); got != c.reader {
				t.Errorf("second line %q, want %q", got, c.reader)
			}
		})
	}
}
