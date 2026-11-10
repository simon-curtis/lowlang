# LowLang Design

LowLang compiles to static native binaries with no libc dependency, has a tracing garbage collector and green threads with channels, and adds sum-type enums and typed error returns on top of a Go-sized core.

## Goals

- Compile speed and binary size comparable to Go.
- Runtime speed comparable to Go.
- Exhaustive enums with payloads.
- Errors as values, propagated with one character, impossible to ignore.
- A small language: one way to loop, one way to declare, data and functions with no methods or inheritance.

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

### Structs

```low
struct Point {
    x: f64
    y: f64
}

fn len(p: Point) -> f64 {
    sqrt(p.x * p.x + p.y * p.y)
}

fn shift(mut p: Point, dx: f64) {
    p.x += dx
}

var p = Point { x: 3.0, y: 4.0 }
p.x += 1.0
shift(mut p, 2.0)
let x = 1.0
let q = Point { x, y: 2.0 }
print(len(p) + len(q))
```

- Literals name every field, separated by commas or newlines; `x` alone means `x: x`.
- Structs are values: assignment and argument passing copy them.
- Fields are assigned through a `var` binding or a `mut` parameter, at any depth: `r.max.x = 1`. Other parameters are immutable.
- A struct literal cannot appear directly in an `if`, `for` or `match` condition; wrap it in parentheses.
- A struct cannot contain itself until heap allocation exists.
- Fields are laid out in declaration order, one word per scalar, with no padding.

### References

```low
fn swap(mut a: i64, mut b: i64) {
    let t = a
    a = b
    b = t
}

var x = 1
var y = 2
swap(mut x, mut y)
```

- A `mut` parameter refers to the caller's variable; reads and writes go through to it. The call site writes `mut` before the argument.
- A `mut` argument must be a `var` binding, a `mut` parameter, or a field of either.
- One call cannot pass overlapping places as `mut`: `swap(mut p.x, mut p.x)` is rejected, while `swap(mut p.x, mut p.y)` is allowed. Two `mut` parameters therefore never alias.
- References exist only as parameters. They cannot be stored, returned or bound with `let`, so they never outlive the variable they refer to.
- A `mut` parameter occupies one word holding the address of the place.

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

- `match` must cover every variant, or end with `_`. Arms after `_` and duplicate arms are errors.
- Patterns bind fields by position; `_` ignores a field, and a bare variant name ignores all of them.
- An arm body is an expression, or `break`, `continue`, `return` or `fail`.
- Variants are constructed as `Shape.Circle(2)`. Where the expected type is known (an annotated `let`, an argument, an assignment, a return, `fail`), the bare name works: `let s: Shape = Empty`.
- Variants with no payload in an enum where no variant has one are a single word. Otherwise an enum is a tag word followed by the largest variant's fields.
- An enum cannot contain itself until heap allocation exists.

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

fn validate(n: i64) ! ParseError {
    if n < 0 { fail BadDigit(n) }
}

fn main() ! ParseError {
    let n = parse("42") catch {
        Empty => 0
        BadDigit(at) => at
    }
    let m = parse("") catch -1
    validate(n)?
}
```

- `fail v` returns the error `v`; `v` is checked against the function's error type, so bare variant names work.
- Postfix `?` returns the error to the caller, whose error type must be the same enum.
- `catch { arms }` matches the error exhaustively; `catch value` substitutes a fallback.
- `fn f() ! E` fails or returns nothing.
- A result cannot be discarded, bound or passed on without `?` or `catch`, except by returning it from a function with the same result type.
- When `main` declares `! E` and returns an error, the program prints `error: <Variant>` to stderr and exits with status 1. Panics exit with status 2.
- A result is a tag word (0 ok, 1 error) followed by the larger of the ok value and the error enum.

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

Code generation is a stack machine. A one-word value lands in `rax`; a wider value (an enum with payload, a result) is pushed onto the stack with its first word lowest. All locals live in the frame, at offsets assigned by the checker. The generator tracks stack depth so `break` and `continue` can unwind partly built expressions.

Calls reserve stack space for a wide return value and the arguments, then evaluate arguments left to right into it. The callee finds argument 0 at `rbp+16` and writes a wide result just above the arguments; a one-word result returns in `rax`.

On fib(38) this runs at 1.5x Go's time, which the SSA IR and register allocator are meant to close.

## Tests

`go test ./...` compiles every `testdata/run/*.low`, runs it and compares stdout with `.out` and stderr with `.err`. Each `testdata/fail/*.low` must fail to compile with the message in its `.err`.

## Implemented

- Types: `i64`, `bool`, `str`, structs, enums with payloads, results `T ! E`.
- `let`, `var`, assignment, `if`/`else`, `for cond`, `for` with `break`/`continue`, `return`.
- Functions with parameters and return values, recursion.
- Arithmetic, comparison and logical operators.
- `match`, `fail`, `?`, `catch`.
- `mut` parameters.
- Builtins `print(i64)`, `print(str)`, `print(bool)`.
- Target: linux/amd64.

## Roadmap

1. SSA IR between check and codegen, with register allocation.
2. Heap allocation and tracing GC, with pointers for shared and recursive structs and enums.
3. Slices, strings with length, maps.
4. Optionals.
5. Generics.
6. Green-thread scheduler, channels, `select`.
7. Own assembler and linker replacing `as` and `ld`.
8. Further targets: linux/arm64, darwin, windows.
