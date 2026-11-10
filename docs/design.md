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

### Numbers

```low
let mask: u32 = 0xFF00
var flags: u8 = 0b0000_0101
flags |= 1 << 4
let shifted = mask >> 8
let ratio: f32 = 0.5
let length = sqrt(3.0 * 3.0 + 4.0 * 4.0)
let whole = i64(floor(length))
let byte = u8(300)
```

- Integer types are `i64`, `i32`, `u32`, `u16` and `u8`; float types are `f64` and `f32`.
- Integer arithmetic wraps at the type's width. Division by zero panics.
- An integer literal takes the type the context expects, including the other operand of a binary operator, and must fit it; otherwise it is `i64`. A float literal is `f64` unless an `f32` is expected. Literals are decimal, `0x` hexadecimal or `0b` binary, with `_` separators, and floats may have an exponent.
- Operands of a binary operator have the same type; conversions are explicit calls such as `u8(x)`, `f64(n)` and `i64(f)`. Integer conversions truncate or extend, float to integer truncates toward zero.
- `&`, `|`, `^`, `~`, `<<` and `>>` work on integers. `>>` is arithmetic on signed types and logical on unsigned ones. The shift count may be any integer type; a count of 64 or more, or a negative one, gives 0, or the sign fill for `>>` on a negative signed value.
- `%` is integer only. Floats follow IEEE 754: division by zero gives an infinity, and comparisons with NaN are false except `!=`.
- `sqrt`, `floor` and `exp` take and return `f32` or `f64`.
- `print` accepts any integer type.

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
- A struct cannot contain itself by value; it can hold a `*` pointer to its own type.
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

### Pointers

```low
enum List {
    Cons(i64, *List)
    Nil
}

var list = List.Nil
list = Cons(1, new(list))

let c = new(Counter { hits: 0 })
c.hits += 1
bump(mut *c)
let copy = *c
```

- `new(value)` copies a value onto the garbage-collected heap and returns a `*T`. Pointers are never null.
- `p.field` reads and writes through the pointer; `*p` is the whole value, readable, assignable and passable as `mut`.
- The object behind a pointer is mutable through any pointer to it, including one held by `let`.
- `==` and `!=` on pointers compare identity.
- `mut` arguments through pointers are not checked for overlap, since two pointers may refer to the same object.

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
- An enum cannot contain itself by value; it can hold a `*` pointer to its own type.

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
| Build IR | `internal/ir` | SSA form, one value per machine word |
| Codegen | `internal/amd64` | x86-64 GNU assembly |
| Assemble and link | `as`, `ld` | static ELF binary |

The `low` command lives in `cmd/low`; the pipeline it shares with hot reload lives in `internal/compiler`. `low ir` prints the SSA form and `low asm` the generated assembly.

The runtime, `internal/amd64/runtime.asm`, is emitted with every program and talks to the kernel through raw syscalls.

The garbage collector is a non-moving mark-sweep collector in the runtime:

- The heap is a 16 GB reservation made with `mmap` and `MAP_NORESERVE`; pages are committed as they are touched.
- Each object has a one-word header holding its size in words, a mark bit and a free bit. A bitmap records the first word of every object and free chunk, which resolves pointers into the middle of an object, such as a `mut` parameter aimed at a heap field.
- Allocation takes from exact-size free lists for objects under 64 words, then first-fit from a list of larger chunks, splitting them, then from the end of the heap.
- Collection runs when the words allocated since the last one exceed the words that survived it, with a 4 MB floor.
- Roots are found by scanning the whole stack conservatively; heap objects are scanned conservatively too. Any word holding a heap address keeps that object alive. Precise scanning needs pointer maps from the register allocator.
- Sweeping coalesces adjacent dead objects into free chunks and returns a trailing free run to the end of the heap.

### IR

- SSA is built directly from the checked tree with Braun et al.'s algorithm: variables are read and written per block, phis are created lazily and sealed once a block's predecessors are known, and trivial phis are removed.
- Every value is one 64-bit word. A struct, enum or result of n words is n SSA values, so aggregates live in registers with no frame temporaries.
- Narrow integers are kept sign-extended (`i32`) or zero-extended (`u32`, `u16`, `u8`) in their word; floats are kept as their bit pattern, `f32` zero-extended. Arithmetic, conversion and comparison values carry their type.
- A variable passed as `mut` lives in a frame slot and is accessed with loads and stores; every other variable is pure SSA.
- Integer division carries its zero and overflow checks inside the instruction.
- Unreachable blocks are pruned after construction.

### Code generation

- Critical edges are split and each phi becomes a copy into a temporary at the end of every predecessor and a copy out of it at the start of the block.
- A comparison used only by the branch that ends its block is fused into a `cmp` and conditional jump.
- Constants that fit in 32 bits are used as immediates and take no register.
- Results of narrow integer operations that can leave the type's range are re-extended with `movsxd`, `movzx` or a 32-bit `mov`. Unsigned comparisons and branches use the unsigned condition codes, and unsigned division uses `div`.
- Float operations load their operands into `xmm0` and `xmm1`, run the scalar SSE instruction (`addsd`, `ucomiss`, `cvttsd2si`, `sqrtsd`, `roundsd` and so on) and move the result back. Float comparisons are materialised rather than fused, so NaN checks use the parity flag.
- Variable shift counts go through `cl`, with `rcx` saved in `r11` around the shift; counts above 63 take a separate path.
- `exp` is a runtime routine: range reduction by `ln 2` split into high and low parts, a degree-13 polynomial, and scaling by two powers of two so subnormal results stay in range.
- Registers are assigned by linear scan over the hull of each value's live range. `rcx`, `rsi`, `rdi`, `r8` and `r9` hold values that are not live across a call; `rbx` and `r12` to `r15` hold values that are, and are saved in the prologue when used. Values that find no register are spilled to the frame. `rax`, `rdx`, `r10` and `r11` are scratch.
- A value used as a call argument prefers that argument's register, and a copy prefers its source's register.

Calls pass the first six argument words in `rdi`, `rsi`, `rdx`, `rcx`, `r8` and `r9` and the rest in an outgoing area at the bottom of the caller's frame, so `rsp` does not move inside a function. Up to two result words return in `rax` and `rdx`; wider results are written to the outgoing area after the stack arguments. Argument and parameter registers are filled with a parallel move that breaks cycles through `r11`.

On fib(38) this runs at 1.25x Go's time.

## Hot reload

`low dev app.low` builds a dev binary, runs it, and watches the source. On save:

- A file that fails to compile is reported and the running program is left alone.
- The program is restarted when a struct or enum layout changes, when an existing function's signature changes, or when `main` changes. A running frame of old code may still call any function, so it must keep the old calling convention and data layout; `main` never returns to pick up new code.
- Otherwise every function whose generated assembly differs from the running version is recompiled, including callers whose code changed only because a callee did. New functions are included. Heap, stack and all live values are kept.

Mechanism:

- Dev builds call every function through `fn_table`, a writable table of 4096 code addresses, with `call [rip+fn_table+8*i]`. Release builds call directly.
- At start the dev runtime installs a `SIGUSR1` handler and maps a 64 MB read-execute region at `0x40000000`, close enough to the binary for 32-bit relative references.
- A patch is assembled with only the changed functions and their strings, then linked at the next free address in that region against the running binary's symbols (`ld -R`). Patched functions get a fresh symbol suffix so they never collide with the originals.
- `low dev` writes the machine code into the region through `/proc/<pid>/mem`, writes `(index, address)` pairs and a count into a mailbox in the program, and sends `SIGUSR1`. The handler stores each address into `fn_table` with an aligned 8-byte write and clears the count, which `low dev` waits for.
- Frames already running keep executing their old code; the next call through the table enters the new code. Old code is never unmapped.

## Tests

`internal/hot` tests reload against running programs: state kept across a code swap, functions added, compile errors leaving the program running, heap data surviving a reload while the collector runs, and restarts on layout and signature changes.

`go test ./...` also compiles every `testdata/run/*.low`, runs it and compares stdout with `.out` and stderr with `.err`, and checks peak memory against `.maxrss` in KB when present. Each `testdata/fail/*.low` must fail to compile with the message in its `.err`.

## Implemented

- Types: `i64`, `i32`, `u32`, `u16`, `u8`, `f64`, `f32`, `bool`, `str`, structs, enums with payloads, results `T ! E`, pointers `*T`.
- `let`, `var`, assignment, `if`/`else`, `for cond`, `for` with `break`/`continue`, `return`.
- Functions with parameters and return values, recursion.
- Arithmetic, comparison, logical, bitwise and shift operators; numeric conversions; `sqrt`, `floor` and `exp`.
- `match`, `fail`, `?`, `catch`.
- `mut` parameters.
- Pointers, `new`, and a garbage-collected heap.
- Builtins `print(i64)`, `print(str)`, `print(bool)`.
- Hot reload with `low dev`.
- Target: linux/amd64.

## Roadmap

1. Slices; byte-level strings with length, UTF-8 iteration, a string builder and number formatting; maps.
2. Optionals.
3. Modules and visibility.
4. Pointer maps from the register allocator, for precise garbage collection; noscan allocations for pixel and atlas buffers.
5. Embedded data: Unicode tables, SPIR-V and a fallback font.
6. Dynamic ELF output: programs that use C libraries link libc and libvulkan dynamically with `ld -dynamic-linker /lib64/ld-linux-x86-64.so.2 -lc ...`. Programs that use none stay static with no libc.
7. C interop:
   - `extern fn` declarations.
   - System V ABI thunks: `rdi`, `rsi`, `rdx`, `rcx`, `r8`, `r9` and `xmm0` to `xmm7`, struct classification, 16-byte stack alignment.
   - C struct layout with natural alignment, alongside the one-word-per-scalar layout.
   - A nullable raw pointer type separate from `*T`.
   - C strings.
   - Unmanaged pointer-plus-length views over mmap'd and GPU-mapped memory.
   - Explicit release of file descriptors and mappings.
   - The garbage collector never frees memory held only by C.
8. Function values, and C-callable function pointers for callbacks.
9. Generics, with type erasure or dynamic dispatch for custom elements and views.
10. Green-thread scheduler, channels, `select`:
    - OS threads are created with `pthread_create` when libc is linked, leaving TLS (`fs`) to glibc.
    - C calls run on a large system stack rather than the green-thread stack.
    - A netpoller (epoll) waits on file descriptors, timerfds and channels together.
11. Builder or default-field syntax for element trees (`div`, `flex`, `padding`, `child` style).
12. GPU UI library modelled on Zed's GPUI, Linux first:
    1. A Wayland window through libwayland-client, with bindings generated from protocol XML; a Vulkan swapchain through `VK_KHR_wayland_surface`; a clear colour.
    2. A scene of GPU primitives (shadows, quads with rounded corners and borders, paths, underlines, monochrome, subpixel and polychrome sprites) drawn by GLSL shaders compiled to embedded SPIR-V. Primitives are sorted by draw order, batched by kind and atlas texture, and clipped by content masks.
    3. Text: font discovery, OpenType parsing, rasterizing into a glyph atlas, shaping, line layout, Unicode grapheme and line-break segmentation.
    4. Flexbox layout (a Taffy subset); an element tree with `request_layout`, `prepaint` and `paint` phases; styles with partial overrides for hover and active states.
    5. Input: libxkbcommon keyboard, key repeat, hitboxes, mouse dispatch, focus and tab stops, actions and keymaps.
    6. App and entity model with notify, observe and subscribe; foreground and background executors on the scheduler.
    7. Text input element, clipboard, IME (`text-input-v3`), fractional scale, `cursor-shape`, `xdg-decoration` and client-side decorations.
    8. Portals and AT-SPI over D-Bus, PNG images, SVG paths.
13. Own assembler and linker replacing `as` and `ld`, with static and dynamic ELF output.
14. Further targets: linux/arm64, darwin, windows; macOS (Metal) and Windows backends for the UI library.
