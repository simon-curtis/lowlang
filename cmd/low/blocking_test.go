package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"testing"
	"time"

	"lowlang/internal/fixture/fixturetest"
)

func TestBlockedReadDoesNotStallCollection(t *testing.T) {
	fixturetest.RequireLinuxHost(t)
	program := fixturetest.RunProgram("std_sched_blocked_read")
	binary := hostBinary(t, program.Path)
	stdout := strings.Split(strings.TrimSuffix(program.Sidecars(t).Stdout, "\n"), "\n")
	data := []byte("xyz")
	cases := []struct {
		name    string
		release func(w io.WriteCloser) error
		reader  string
	}{
		{"eof", func(w io.WriteCloser) error { return w.Close() }, stdout[1]},
		{"data", func(w io.WriteCloser) error {
			_, err := w.Write(data)
			return err
		}, fmt.Sprintf("reader %d", len(data))},
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
			out, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			defer func() {
				stdin.Close()
				cmd.Wait()
			}()
			lines := bufio.NewScanner(out)
			if !lines.Scan() {
				t.Fatalf("no output while a reader was blocked: %v", lines.Err())
			}
			if got := lines.Text(); got != stdout[0] {
				t.Fatalf("first line %q, want %q", got, stdout[0])
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
