package main

import (
	"strings"
	"testing"
)

const promoteRotProgram = `fn rot(mut a: u32, mut b: u32) {
    a = a + b
    b = (b << 7) | (b >> 25)
    b = b ^ a
}

fn core(n: i64) -> u32 {
    var x: u32 = 1
    var y: u32 = 2
    for i in 0..n {
        rot(mut x, mut y)
    }
    x + y
}

fn main() {
    print(core(10))
}
`

func TestInlinedMutCalleeLosesSlot(t *testing.T) {
	prog := analyseRelease(t, promoteRotProgram)
	main := function(t, prog, "main")
	if len(main.Slots) != 0 {
		t.Fatalf("slots remain: %v", main.Slots)
	}
	text := main.String()
	for _, op := range []string{" = load ", " = store ", " = slot "} {
		if strings.Contains(text, op) {
			t.Fatalf("main still contains%s:\n%s", op, text)
		}
	}
	if asm := functionAsm(t, prog, "main"); strings.Contains(asm, "[rbp") {
		t.Fatalf("main addresses the frame:\n%s", asm)
	}
}

func TestEscapingSlotStays(t *testing.T) {
	prog := analyse(t, promoteRotProgram)
	core := function(t, prog, "core")
	if len(core.Slots) != 2 {
		t.Fatalf("want 2 slots passed to rot, got %v", core.Slots)
	}
	if !strings.Contains(core.String(), " = load ") {
		t.Fatalf("loads after the call were removed:\n%s", core)
	}
}

const promoteBranchProgram = `fn pick(mut a: i64, mut b: i64, flag: bool) {
    if flag {
        a = b + 1
    } else {
        b = a + 2
    }
}

fn main() {
    var x = 1
    var y = 5
    for i in 0..4 {
        pick(mut x, mut y, i % 2 == 0)
    }
    print(x)
    print(y)
}
`

func TestConditionalStoresBecomePhis(t *testing.T) {
	prog := analyseRelease(t, promoteBranchProgram)
	main := function(t, prog, "main")
	if len(main.Slots) != 0 {
		t.Fatalf("slots remain: %v\n%s", main.Slots, main)
	}
	if !strings.Contains(main.String(), " = phi ") {
		t.Fatalf("expected phis:\n%s", main)
	}
}

const promoteCStructProgram = `extern struct Pair {
    a: u8 = 0
    b: u16 = 0
    c: u32 = 0
}

fn bump(mut x: u8) {
    x += 1
}

fn main() {
    var p = Pair { a: 1, b: 2, c: 3 }
    bump(mut p.a)
    print("{p.a} {p.b} {p.c}")
}
`

func TestCStructSlotStays(t *testing.T) {
	prog := analyseRelease(t, promoteCStructProgram)
	main := function(t, prog, "main")
	if len(main.Slots) == 0 {
		t.Fatalf("extern struct slot was promoted:\n%s", main)
	}
	if !strings.Contains(main.String(), " = load ") {
		t.Fatalf("extern struct field loads were removed:\n%s", main)
	}
}

const promoteSelectProgram = `fn main() {
    let quiet = chan[i64]()
    let polled = select {
        <-quiet => 1
        else => 2
    }
    print(polled)
}
`

func TestSelectBoxStays(t *testing.T) {
	prog := analyseRelease(t, promoteSelectProgram)
	main := function(t, prog, "main")
	if len(main.Slots) == 0 {
		t.Fatalf("select box slot was promoted:\n%s", main)
	}
}

const promoteWideProgram = `struct Pair {
    a: i64
    b: i64
}

fn swap(mut p: Pair) {
    let t = p.a
    p.a = p.b
    p.b = t
}

fn main() {
    var p = Pair { a: 1, b: 2 }
    for i in 0..3 {
        swap(mut p)
    }
    print(p.a)
    print(p.b)
}
`

func TestMultiWordSlotPromoted(t *testing.T) {
	prog := analyseRelease(t, promoteWideProgram)
	main := function(t, prog, "main")
	if len(main.Slots) != 0 {
		t.Fatalf("slots remain: %v\n%s", main.Slots, main)
	}
}

const promoteInlinedRawProgram = `fn peek(p: ptr[i64]) {
    load(p)
}

fn main() {
    let p: ptr[i64] = null
    print("before")
    peek(p)
    print("after")
}
`

func TestInlinedRawLoadKeepsTrap(t *testing.T) {
	prog := analyseRelease(t, promoteInlinedRawProgram)
	main := function(t, prog, "main")
	if !strings.Contains(main.String(), " = load ") {
		t.Fatalf("inlined unused load was discarded:\n%s", main)
	}
}
