package compiler

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lowlang/internal/amd64"
	"lowlang/internal/ast"
	"lowlang/internal/check"
	"lowlang/internal/diag"
	"lowlang/internal/ir"
	"lowlang/internal/parser"
	"lowlang/internal/target"
	"lowlang/internal/types"
)

type lengthPair struct {
	n, m    int
	filler  byte
	xAt     int
	yAt     int
	special byte
}

func (p lengthPair) call() string {
	return fmt.Sprintf("    collisions += trial(%d, %d, %d, %d, %d, %d)\n", p.n, p.m, p.filler, p.xAt, p.yAt, p.special)
}

func (p lengthPair) strings() (string, string) {
	build := func(length, at int) string {
		b := bytes.Repeat([]byte{p.filler}, length)
		if at >= 0 {
			b[at] = p.special
		}
		return string(b)
	}
	return build(p.n, p.xAt), build(p.m, p.yAt)
}

func blockCount(n int) int {
	if n <= 16 {
		return 0
	}
	return (n+15)/16 - 1
}

type window struct {
	blocks string
	a, b   uint64
}

func windowOf(s string) window {
	n := len(s)
	k := blockCount(n)
	w := window{blocks: s[:16*k]}
	switch {
	case n >= 16:
		w.a = binary.LittleEndian.Uint64([]byte(s[n-16:]))
		w.b = binary.LittleEndian.Uint64([]byte(s[n-8:]))
	case n >= 8:
		w.a = binary.LittleEndian.Uint64([]byte(s[:8]))
		w.b = binary.LittleEndian.Uint64([]byte(s[n-8:]))
	}
	return w
}

func lengthCancels(x, y string) bool {
	wx, wy := windowOf(x), windowOf(y)
	return len(x) != len(y) &&
		blockCount(len(x)) == blockCount(len(y)) &&
		wx.blocks == wy.blocks &&
		wx.a == wy.a &&
		wx.b^uint64(len(x)) == wy.b^uint64(len(y))
}

func lengthPairs() []lengthPair {
	var pairs []lengthPair
	for n := 8; n <= 16; n++ {
		for m := n + 1; m <= 16; m++ {
			d := byte(n ^ m)
			pairs = append(pairs, lengthPair{n: n, m: m, filler: '`', xAt: n - 8, yAt: n - 8, special: '`' ^ d})
		}
	}
	for blocks := 1; blocks <= 3; blocks++ {
		for n := 16*blocks + 8; n <= 16*blocks+16; n++ {
			for m := n + 1; m <= 16*blocks+16; m++ {
				d := byte(n ^ m)
				pairs = append(pairs, lengthPair{n: n, m: m, filler: '`', xAt: n - 8, yAt: -1, special: '`' ^ d})
			}
		}
	}
	return pairs
}

const hashProbe = `fn shaped(length: i64, filler: u8, at: i64, special: u8) -> str {
    var b: []u8 = []
    for i in 0..length {
        push(mut b, if i == at { special } else { filler })
    }
    __str_view(b)
}

fn trial(n: i64, m: i64, filler: u8, xAt: i64, yAt: i64, special: u8) -> i64 {
    if __hash(shaped(n, filler, xAt, special)) == __hash(shaped(m, filler, yAt, special)) {
        return 1
    }
    0
}

fn main() {
    var collisions = 0
%s    print(collisions)
}
`

func buildProbe(t *testing.T, source string) string {
	t.Helper()
	prelude, err := parsePrelude()
	if err != nil {
		t.Fatal(err)
	}
	probe, err := parser.ParseSource([]byte(source), diag.Start("probe.low"), true)
	if err != nil {
		t.Fatal(err)
	}
	files := []*ast.File{prelude, probe}
	if _, err := check.Check(files, stdFS, types.NewUniverse()); err != nil {
		t.Fatal(err)
	}
	host := target.Host()
	unit := &Unit{Platform: amd64.NewPlatform(host), Program: ir.Build(files, ir.Options{Inline: true})}
	binary := filepath.Join(t.TempDir(), host.Executable("probe"))
	asm := amd64.Generate(unit.Program, unit.Platform, nil).Asm
	if err := Link(context.Background(), asm, binary, unit); err != nil {
		t.Fatal(err)
	}
	return binary
}

func TestStringHashIgnoresLengthCancellation(t *testing.T) {
	pairs := lengthPairs()
	var calls strings.Builder
	for _, p := range pairs {
		x, y := p.strings()
		if !lengthCancels(x, y) {
			t.Fatalf("pair of lengths %d and %d does not cancel the length", p.n, p.m)
		}
		calls.WriteString(p.call())
	}
	first, second := pairs[0].strings()
	literal := fmt.Sprintf("a%s", strings.Repeat("`", 7))
	if first != literal || second != fmt.Sprintf("%s`", literal) {
		t.Fatalf("first pair is %q and %q, want the 8 and 9 byte pair", first, second)
	}
	binary := buildProbe(t, fmt.Sprintf(hashProbe, calls.String()))
	for run := range 8 {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		out, err := exec.CommandContext(ctx, binary).Output()
		cancel()
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.TrimSpace(string(out)); got != "0" {
			t.Errorf("run %d: %s of %d pairs hash the same under a fresh seed", run, got, len(pairs))
		}
	}
}
