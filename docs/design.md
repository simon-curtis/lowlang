# LowLang Design

LowLang compiles to static native binaries with no libc dependency, has a tracing garbage collector and green threads with channels, and adds sum-type enums and typed error returns on top of a Go-sized core.

## Goals

- Compile speed and binary size comparable to Go.
- Runtime speed comparable to Go.
- Exhaustive enums with payloads.
- Errors as values, propagated with one character, impossible to ignore.
- A small language: one way to loop, one way to declare, no inheritance.

## Syntax

Newlines terminate statements. Blocks are expressions; the last expression is the value.

### Bindings

```low
let name = "lowlang"
var count = 0
count += 1
```

`let` is immutable, `var` is mutable. Types are inferred and may be written as `let x: i64 = 1`.

### Functions

```low
fn add(a: i64, b: i64) -> i64 {
    a + b
}
```

### Control flow

```low
if n > 0 { print(n) } else { print(0) }

for i in 0..10 { print(i) }
for n > 0 { n -= 1 }
for { break }
```

### Structs and methods

```low
struct Point {
    x: f64
    y: f64
}

impl Point {
    fn len(self) -> f64 { sqrt(self.x * self.x + self.y * self.y) }
}
```

### Enums

```low
enum Shape {
    Circle(radius: f64)
    Rect(w: f64, h: f64)
    Empty
}

fn area(s: Shape) -> f64 {
    match s {
        Circle(r) => 3.14159 * r * r
        Rect(w, h) => w * h
        Empty => 0.0
    }
}
```

`match` must be exhaustive. Variants are in scope inside `match` arms on their enum.

### Optionals

`T?` is either a value or `none`. `x ?? fallback` unwraps with a default.

### Errors

An error type is an enum. A function that can fail declares `-> T ! E`.

```low
enum ParseError {
    Empty
    BadDigit(at: i64)
}

fn parse(s: str) -> i64 ! ParseError {
    if s.len == 0 { fail Empty }
    ...
}

fn total(a: str, b: str) -> i64 ! ParseError {
    parse(a)? + parse(b)?
}

fn main() {
    let n = parse("42") catch e {
        Empty => 0
        BadDigit(at) => at
    }
}
```

- `fail v` returns the error `v`.
- Postfix `?` propagates the error to the caller. The caller's error type must accept it.
- `catch` handles errors inline with an exhaustive match.
- A fallible result cannot be discarded or used as a plain value without `?` or `catch`.

### Generics

```low
fn max[T: Ord](a: T, b: T) -> T {
    if a > b { a } else { b }
}
```

### Concurrency

```low
let results = chan[i64](16)
spawn worker(results)
let v = <-results
select {
    v = <-results => print(v)
    timeout(1s) => print("slow")
}
```

`spawn` starts a green thread on the runtime's M:N scheduler.

## Compiler

Written in Go. Pipeline:

| Stage | Package | Output |
|---|---|---|
| Lex | `internal/lexer` | tokens |
| Parse | `internal/parser` | `internal/ast` tree |
| Check | `internal/check` | types attached to the tree |
| Codegen | `internal/amd64` | x86-64 GNU assembly |
| Assemble and link | `as`, `ld` | static ELF binary |

The `low` command lives in `cmd/low`.

The runtime, `internal/amd64/runtime.asm`, is emitted with every program and talks to the kernel through raw syscalls.

Code generation is a stack machine: every expression lands in `rax`, intermediates are pushed, and all locals live in the frame. Arguments are pushed right to left and the result returns in `rax`. On fib(38) this runs at 1.5x Go's time, which the SSA IR and register allocator are meant to close.

## Tests

`go test ./...` compiles every `testdata/run/*.low`, runs it and compares stdout with `.out` and stderr with `.err`. Each `testdata/fail/*.low` must fail to compile with the message in its `.err`.

## Implemented

- Types: `i64`, `bool`, `str`.
- `let`, `var`, assignment, `if`/`else`, `for cond`, `for` with `break`/`continue`, `return`.
- Functions with parameters and return values, recursion.
- Arithmetic, comparison and logical operators.
- Builtins `print(i64)`, `print(str)`, `print(bool)`.
- Target: linux/amd64.

## Roadmap

1. SSA IR between check and codegen, with register allocation.
2. Structs, enums, `match`.
3. Error unions, `fail`, `?`, `catch`.
4. Heap allocation and tracing GC.
5. Slices, strings with length, maps.
6. Generics.
7. Green-thread scheduler, channels, `select`.
8. Own assembler and linker replacing `as` and `ld`.
9. Further targets: linux/arm64, darwin, windows.
