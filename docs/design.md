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

fn counter() -> fn() -> i64 {
    var n = 0
    fn() -> i64 {
        n += 1
        n
    }
}

let twice = fn(x: i64) -> i64 { x * 2 }
let ops: [str: fn(i64) -> i64] = ["twice": twice, "negate": negate]
let parse: fn(str) -> i64 ! ParseError = parse_int
let square: fn(i64) -> i64 = fn(x) { x * x }

fn first(s: str) -> u8 {
    s[0]
}

"simon".first()
items.push(4)

let shouted = "hi"
    |> text.repeat(3)
    |> first
let n = input |> parse()?
```

- A function type is written `fn(i64, mut str) -> bool`, with `! E` for a result that can fail. Two function types are the same when their parameters, `mut` markers and results are.
- A named function is a value of its function type. `fn(params) -> R { body }` is a closure; parameter and result types may be left out when the expected function type supplies them, and a closure with no written result and no expected type returns nothing.
- A closure captures the variables it names by reference, as Go does: it reads and assigns the variable itself, and the variable lives as long as any closure holding it. Each loop iteration binds fresh variables, so closures made in different iterations see different ones. A `mut` parameter cannot be captured.
- A function value is called like a function. Values of function type can be stored in variables, fields, slices and maps, passed and returned; they cannot be compared or printed.
- `x.f(args)` calls `f(x, args)`, so calls chain left to right: `"ab".repeat(3).first()`. `f` is any function callable by its plain name: a function in the current module, a function value in scope, a generic function or a builtin. When no such name exists and `x` is a struct or enum, or a pointer to one, from another module, `f` is that module's public function, so `url.authority()` calls `http.authority(url)`. A struct field of function type named `f` takes precedence, so `op.apply(v)` calls the stored function. When the first parameter is `mut`, `x` is passed as `mut` and must be a mutable place.
- `x |> f(args)` also calls `f(x, args)`, and `x |> f` calls `f(x)`. The right side is a call, a function name, a module function such as `text.repeat`, a closure, or a struct field holding a function, so `x |> ops.scale` calls the stored function and never a function named `scale`; any other expression is an error. `|>` has the lowest precedence of the binary operators and groups left to right, so `a + b |> f()` is `f(a + b)`. A trailing `?` applies to the call: `x |> parse()?` is `parse(x)?`. A `mut` first parameter receives `x` as `mut`, as with `x.f(args)`.
- A line that starts with `|>` or `.` continues the expression on the line before it.

### Control flow

```low
if n > 0 { print(n) } else { print(0) }

for i in 0..10 { print(i) }
for x in xs { print(x) }
for i, x in xs { print(i) }
for n > 0 { n -= 1 }
for { break }

rows: for row, cells in grid {
    for col, cell in cells {
        if cell == want { break rows }
    }
}
```

- `for i in a..b` counts from `a` up to but not including `b`; the bounds are integers of one type.
- `for x in xs` visits each element of a slice, and `for i, x in xs` also binds its index as `i64`.
- Loop variables are immutable. `continue` moves to the next element.
- `break`, `continue`, `return` and `fail` are expressions of type never, so they can stand wherever a value is expected: after `??`, in an arm, or as a branch of `if`.
- A loop can be labelled with `name:` before `for`; `break name` and `continue name` then act on that loop from inside nested loops, running the deferred statements and drops of every block they leave. A label must be used, cannot repeat the label of an enclosing loop, and is not visible inside closures.

### Numbers

```low
let mask: u32 = 0xFF00
var flags: u8 = 0b0000_0101
let mode = 0o640
flags |= 1 << 4
let shifted = mask >> 8
let ratio: f32 = 0.5
let length = sqrt(3.0 * 3.0 + 4.0 * 4.0)
let whole = i64(floor(length))
let byte = u8(300)
```

- Integer types are `i64`, `i32`, `u64`, `u32`, `u16` and `u8`; float types are `f64` and `f32`.
- Integer literals are decimal, or hexadecimal, binary and octal with `0x`, `0b` and `0o`; `_` may separate digits.
- Integer arithmetic wraps at the type's width. Division by zero panics.
- An integer literal takes the type the context expects, including the other operand of a binary operator, and must fit it; otherwise it is `i64`. A literal above the `i64` range is only accepted as a `u64`. A float literal is `f64` unless an `f32` is expected. In an `if` expression with no expected type, an integer literal branch takes the type of the other branch. Literals are decimal, `0x` hexadecimal or `0b` binary, with `_` separators, and floats may have an exponent.
- Operands of a binary operator have the same type; conversions are explicit calls such as `u8(x)`, `f64(n)` and `i64(f)`. Integer conversions truncate or extend, float to integer truncates toward zero.
- `&`, `|`, `^`, `~`, `<<` and `>>` work on integers. `>>` is arithmetic on signed types and logical on unsigned ones. The shift count may be any integer type; a count of 64 or more, or a negative one, gives 0, or the sign fill for `>>` on a negative signed value.
- Binary operators bind as in Go: `*`, `/`, `%`, `<<`, `>>` and `&` tightest; then `+`, `-`, `|` and `^`; then `??`; then comparisons; then `&&`; then `||`. So `a - b >> 1` is `a - (b >> 1)` and `x | y - z` is `(x | y) - z`.
- `%` is integer only. Floats follow IEEE 754: division by zero gives an infinity, and comparisons with NaN are false except `!=`.
- `sqrt`, `floor` and `exp` take and return `f32` or `f64`.
- `print` accepts any integer type.

### Vectors

```low
import std/simd

fn find(data: []u8, c: u8) -> i64 {
    let needle: u8x32 = simd.splat(c)
    var i = 0
    for i + 32 <= data.len {
        let chunk: u8x32 = simd.read(data, i)
        let hits = simd.mask(chunk == needle)
        if hits != 0 {
            return i + simd.trailing_zeros(hits)
        }
        i += 32
    }
    -1
}
```

- Vector types are 128 or 256 bits of one lane type: `u8x16`, `u8x32`, `u16x8`, `u16x16`, `i32x4`, `i32x8`, `u32x4`, `u32x8`, `i64x2`, `i64x4`, `u64x2`, `u64x4`, `f32x4`, `f32x8`, `f64x2` and `f64x4`.
- `+`, `-`, `*`, `&`, `|`, `^`, `~` and unary `-` work lane by lane, `/` on float lanes, and `<<` and `>>` on integer lanes by a scalar count, arithmetic on signed lanes. Comparisons work lane by lane and give a vector of the same type whose lanes are all ones or all zeros; unsigned lanes compare unsigned. A scalar operand of the lane type is broadcast, so `chunk == '"'` compares every lane. Compound assignments work the same way.
- `u8x32(v)` reinterprets a vector of the same width.
- `std/simd` provides `read` and `write` between a vector and a slice at an element index, `read_str`, `splat`, `mask` (one bit per lane from its top bit), `any`, `blend`, `min`, `max`, `lane`, `with_lane`, `shuffle` and `shuffle16` (a byte table lookup within each 16-byte half, zero where the index has its top bit set), `trailing_zeros`, `count_ones` and `find_byte`. Reads and writes are bounds-checked.
- Vectors are values: they can be passed, returned, stored in fields and slices, and compared lane by lane, but not with `==` as a whole or printed.

### Slices

```low
let xs = [3, 1, 4]
var squares: []i64 = []
push(mut squares, 9)
let middle = xs[1..3]
xs[0] = 10
print(xs.len)
```

- `[]T` is a pointer, a length and a capacity over garbage-collected memory. Assigning or passing a slice shares its elements.
- `[a, b]` builds a slice; an empty `[]` needs an expected type.
- `xs[i]` reads or writes an element and panics when `i` is outside `0..xs.len`. Any integer type indexes.
- `xs[a..b]`, `xs[..b]` and `xs[a..]` share the elements in that range. The result's capacity equals its length, so `push` onto it always copies and never overwrites the parent.
- `push(mut xs, v)` appends, growing the capacity to twice plus four when full.
- `xs.len` is the length as `i64`.
- Elements of `u8`, `u16`, `u32`, `i32` and `f32` are packed at their natural width; other elements take a word each.

### Maps

```low
let ages = ["ada": 36, "alan": 41]
ages["grace"] = 85
print(ages["bob"] ?? -1)
var counts: [str: i64] = [:]
counts["a"] += 1
delete(mut counts, "a")
for name, age in ages { print("{name} {age}") }
for name in ages { print(name) }
```

- `[K: V]` maps keys of an integer type, `bool`, `str` or a pointer type to values of any type. A map is a pointer to a table on the garbage-collected heap, so assigning or passing it shares the table, and `m[k] = v` works through any binding.
- `[k: v, ...]` builds a map; `[:]` is an empty one and needs an expected type.
- `m[k]` returns `V?`. `m[k] = v` inserts or replaces. A compound assignment such as `m[k] += 1` starts from a zero value when `k` is missing.
- `delete(mut m, k)` removes a key; removing a missing key does nothing.
- `m.len` is the number of entries.
- `for k, v in m` and `for k in m` visit entries in insertion order. A replaced value keeps its key's position; a deleted and re-inserted key moves to the end. Inserting new keys while iterating may skip or repeat entries.

### Strings

```low
let name = "Ada"
let line = "{name} is {age}, ratio {score / 2.0}"
let full = name + " Lovelace"
for c in "héllo" { print(c) }
let first: u8 = name[0]
let copy = str(bytes(name))
```

- `str` is an immutable pointer and length over UTF-8 bytes.
- `s.len` is the length in bytes, `s[i]` reads a byte as `u8`, and `s[a..b]` shares a byte range.
- `for c in s` visits code points as `u32`, and `for i, c in s` also binds each code point's byte offset. Invalid UTF-8 yields U+FFFD for one byte.
- `"{expr}"` interpolates a number, `bool` or `str`; `{{` and `}}` are literal braces. `a + b` concatenates.
- Floats are formatted as the shortest decimal that reads back to the same value: positional for exponents from -6 to 20, otherwise `1.5e-7` or `1e+21`. Infinities print as `inf` and `-inf`, NaN as `NaN`.
- `'a'` is a code point of type `u32`, or `u8` where one is expected.
- `bytes(s)` copies a string into a `[]u8`; `str(b)` copies one back.
- `==` and `!=` compare contents; `<`, `<=`, `>` and `>=` compare bytes in order, which for UTF-8 is code point order.

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

struct Div {
    padding: f64 = 0.0
    align: Align = Start
    on_click: fn() = fn() {}
    children: []Element = []
}

let page = Div { padding: 8.0 } {
    Label(Text { value: "title" })
    Box(Div { align: Center } {
        button("ok")
        button("cancel")
    })
}
```

- Literals name their fields, separated by commas or newlines; `x` alone means `x: x`. A field declared with `= expr` may be left out, and every other field must be given.
- A default is any expression. It is checked once against the field's type, in the declaring module's scope, with no access to locals or other fields, and evaluated at every literal that leaves the field out, after the written fields. A field with a default can be left out of a literal in another module even when it is private.
- A brace block on the same line after a struct literal holds its children: the entries, one per line or separated by commas, become a slice literal for the struct's `children` field. After a call, the block becomes a slice literal passed as the last argument, so a function such as `fn button(label: str, children: []Element) -> Element` takes children the same way. A trailing block is not allowed where a struct literal is not, such as an `if` condition.
- Structs are values: assignment and argument passing copy them.
- `==` and `!=` compare structs field by field when every field can be compared: numbers, `bool`, `str`, pointers, channels, and structs and enums made of them. Each field uses its own `==`, so strings compare by content, floats by value, and pointers by identity.
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

- `match` must cover every variant, or end with `_` or a binding arm. A binding arm is a name starting with a lowercase letter that is not a variant, such as `other => describe(other)`: it matches anything and binds the whole subject. A capitalized name that is not a variant is an error, so a mistyped variant is never taken for a binding. Arms after `_` or a binding arm, and duplicate arms, are errors.
- An arm that starts with a binary operator tests the subject: `% 3 == 0 => ...` means `subject % 3 == 0`, with the subject on the left and ordinary precedence after it. The subject is evaluated once and arms are tried in order. Such tests work on a subject of any type, including enums (`== Empty`) and the error of a `catch`; a `match` that uses one, or whose subject is not an enum, must end with `_`. Arms are separated by newlines or commas.

```low
fn classify(n: i64) -> str {
    match n {
        % 15 == 0 => "FizzBuzz",
        % 3 == 0 => "Fizz",
        % 5 == 0 => "Buzz",
        _ => "number"
    }
}
```
- Patterns bind fields by position; `_` ignores a field, and a bare variant name ignores all of them.
- An arm body is an expression, or `break`, `continue`, `return` or `fail`.
- Variants are constructed as `Shape.Circle(2)`. Where the expected type is known (an annotated `let`, an argument, an assignment, a return, `fail`), the bare name works: `let s: Shape = Empty`.
- Variants with no payload in an enum where no variant has one are a single word. Otherwise an enum is a tag word followed by the largest variant's fields.
- An enum cannot contain itself by value; it can hold a `*` pointer to its own type.
- `==` and `!=` compare enums whose variant fields can all be compared: equal values have the same variant and equal fields. Optionals compare the same way, so `x == none` and `x == 5` work on an `i64?`.

### Optionals

```low
fn find(xs: []i64, target: i64) -> i64? {
    for i, x in xs {
        if x == target {
            return i
        }
    }
    none
}

let index = find(xs, 7) ?? -1
match find(xs, 4) {
    some(i) => print("at {i}")
    none => print("missing")
}

struct Node {
    value: i64
    next: *Node?
}
```

- `T?` holds either `some(value)` or `none`.
- A `T` is accepted wherever a `T?` is expected: in an annotated `let`, an assignment, an argument, a field, a return and either branch of an `if` or `match`. `none` and `some(v)` take their type from that expectation.
- `x ?? fallback` gives the value inside `x`, or evaluates `fallback` when `x` is `none`. The fallback may itself be a `T?`, making the result a `T?`. The fallback may leave instead, as in `let line = read_line() ?? break`. `??` binds tighter than comparisons and looser than arithmetic, and groups to the right, so `a ?? b ?? 0` tries `a`, then `b`.
- `match` unwraps an optional with `some(v)` and `none` arms and must cover both.
- `?` after a type binds to everything before it: `*Node?` is an optional pointer and `[]i64?` an optional slice.
- An optional is an enum with `none` as tag 0 and `some` as tag 1, so it takes one word more than its value.

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
- `catch { arms }` matches the error exhaustively, and a binding arm such as `e => report(e)` receives the whole error; `catch value` substitutes a fallback.
- `fn f() ! E` fails or returns nothing.
- A result cannot be discarded, bound or passed on without `?` or `catch`, except by returning it from a function with the same result type.
- When `main` declares `! E` and returns an error, the program prints `error: <Variant>` to stderr and exits with status 1. Panics exit with status 2.
- A result is a tag word (0 ok, 1 error) followed by the larger of the ok value and the error enum.

### Embedded data

```low
let shader = embed("shaders/quad.spv")
let font = embed("fonts/fallback.ttf")
print(shader.len)
```

- `embed("path")` is the contents of a file as a `str`, read at compile time. The path must be a string literal and is relative to the directory of the file that calls `embed`, so a module carries its own assets.
- The bytes live in the binary's read-only data, aligned to 16 bytes; nothing is copied at run time. Indexing, slicing, `.len` and `bytes(...)` work as on any string.
- A missing file is a compile error. `low dev` watches embedded files, and a change to one reloads the functions that embed it.

### C interop

```low
extern "c" {
    fn malloc(n: i64) -> ptr
    fn free(p: ptr)
    fn strtol(s: ptr[u8], mut end: ptr, base: i32) -> i64
    fn snprintf(buf: ptr[u8], n: i64, format: ptr[u8], ...) -> i32
    pub fn div(a: i32, b: i32) -> DivT
}

extern struct DivT {
    quot: i32
    rem: i32
}

using block = Block[u8] { data: malloc(64) }
let buf = block.data
snprintf(buf, 64, cstr("%d-%.2f"), 42, 3.14159)
print(str(buf))
var end: ptr = null
let n = strtol(cstr("123abc"), mut end, 10)
let pixels = view(buf, 64)
```

- `extern "name" { ... }` at the top level of a file names a C library and declares its functions, with no bodies. `"c"` is the C library itself. A trailing `...` makes a function variadic. `fn errno_location() -> ptr[i32] = "__errno_location"` gives a function a LowLang name different from its C symbol, for symbols that start with `__` or are LowLang keywords. Declarations are private to their module unless marked `pub`.
- A library is linked when code reachable from `main` calls or takes the address of one of the functions its `extern` block declares, so importing a module that declares C functions costs nothing until one is used. A program that uses no library is a static executable with no libc. A program that uses any is a dynamic executable linked against libc and each used library.
- C signatures use C types: integers, floats, `bool`, `ptr` and `ptr[T]`, and `extern struct`s by value. A `mut` parameter passes the address of a LowLang place for the duration of the call, as C# passes `ref`; C may read and write through it.
- `ptr[T]` is a nullable pointer to C memory and `ptr` is `void*`; `null` takes its type from context, and `ptr` converts to and from any `ptr[T]`. `==` and `!=` compare addresses.
- `view(p, n)` is a `[]T` over `n` elements at `p`, bounds-checked like any slice; `load(p)` and `store(p, v)` read and write one `T`.
- `cstr(s)` is a NUL-terminated copy of a string; `str(p)` copies a NUL-terminated `ptr[u8]` back.
- `extern struct` has C layout: fields in order at their natural alignment, size rounded to the largest alignment. Its fields must have C types. It is used like any struct and its fields are read and written at their own width.
- Memory may be passed to C for the duration of a call. Memory C keeps after the call must be pinned: `pin(x)` returns the address of a pointer, slice, map or string's data and keeps it alive until `unpin(p)`. Memory C allocates is never touched by the collector.
- `extern fn(ptr[i64], ptr[i64]) -> i32` is a C function pointer, one word, like C#'s `delegate* unmanaged`. Its parameters and result must have C types. It is a C type, so it can be a parameter, result or `extern struct` field.
- A top-level LowLang function becomes a C callback where an `extern fn` is expected, if it takes and returns only numbers, `bool`, `ptr` or `extern fn`, with no `mut` parameters. An `extern` function used as a value is its C address. Calling an `extern fn` value makes a C call.
- A closure reaches C through a `ptr` user-data argument: pin a pointer to the state it needs and pass a top-level callback that loads it.

```low
extern "c" {
    fn qsort(base: ptr, n: i64, size: i64, compare: extern fn(ptr[i64], ptr[i64]) -> i32)
}

fn ascending(a: ptr[i64], b: ptr[i64]) -> i32 {
    i32(load(a) - load(b))
}

qsort(data, n, 8, ascending)
```

### Defer

```low
let fd = open(path)?
defer close(fd)
defer {
    let summary = "closing {path}"
    log(summary)
}
```

- `defer` runs a statement when its enclosing block exits: at the end of the block, on `return`, `fail` or a propagated `?`, and when `break` or `continue` leaves it. Deferred statements of a block run last first.
- The statement runs at exit and sees variables as they are then. It cannot `return`, `fail`, use `?`, `break`, `continue` or contain another `defer`.

### Using

```low
struct File {
    fd: i32
    drop close(fd)
}

struct Conn {
    sock: i32
    buf: Block[u8]
    drop {
        flush(sock)
        close(sock)
    }
}

struct Block[T] {
    data: ptr[T]
    drop free(data)
}

using f = open_file("config.low")
using c = connect(addr)?
using var current = open_file("a")
current = open_file("b")
```

- `drop <stmt>` inside a struct body declares its cleanup, with the struct's fields in scope as borrowed values. Like `defer`, it takes one statement or a `{ }` block. A struct has at most one `drop`, and `using` cannot appear in it.
- A struct with a `drop` is droppable, and so is a struct with a droppable field, even without its own `drop`. Releasing a value runs its own `drop` first, then releases its droppable fields in declaration order. Generic instances are droppable per instance: `Pair[File, i64]` is, `Pair[i64, i64]` is not.
- `using x = expr` binds `x` and releases it when the enclosing block exits by any path: the end of the block, `return`, `fail`, `?`, `break` or `continue`, last first together with `defer`. `using var x = expr` can be reassigned; assignment releases the old value, and exit releases the value it holds then. `using` cannot appear inside `defer`, and binding a non-droppable value with it is an error.
- A droppable value cannot be bound with `let` or `var`: "File has drop; bind it with using". Results of droppable types are unwrapped with `?` or `catch` straight into a `using` binding.
- Droppable values are move-only:
  - `return x`, or a function body ending in `x`, moves a `using` binding out; the caller owns the value and must bind it with `using`.
  - A struct literal field or an assignment to a field of a droppable value moves the value into it, and the outer value releases it. A field is assigned through a `using var` binding or a `mut` parameter; the old value of the field is released first.
  - `using y = x` moves `x` into `y`.
  - Passing a value as an argument borrows it: the callee cannot release it, store it or return it. A value that has just been produced, such as a call result or a struct literal, must be bound with `using` before it is passed.
  - Any other copy is an error: binding it with `let`, storing it in a slice, map, channel, optional, pointer or enum variant, moving a field out of its struct, discarding a call result, or capturing it in a closure.
  - A struct with a droppable field is itself droppable, so it cannot be built with `new` or held by a pointer either, and `spawn` cannot take one as an argument: "spawn cannot take File; it has drop". Code that shares a resource across closures, pointers or green threads passes a copyable view of it and binds the owner where the resource has one home (see Resource ownership).
  - Using a binding after it was moved is an error, and so is using one that may have been moved on one branch or by an earlier loop iteration.
- Generic code is checked under these rules for each instance whose type arguments are droppable, so `fn same[T](x: T) -> T { x }` cannot be instantiated with `File`.
- Each `using` binding has a hidden flag that a move clears; the release at exit runs only while the flag is set, so a value moved on one branch is released exactly once.

### Modules

```low
import geometry
import t = ui/text

pub struct Circle {
    pub radius: f64
    cache: f64
}

pub fn area(c: Circle) -> f64 {
    3.14159 * c.radius * c.radius
}

fn main() {
    let c = geometry.circle(2.0)
    print(geometry.area(c))
    let s: geometry.Shape = Square(3)
    print(t.shout("hi"))
}
```

- A module is a directory: every `.low` file in it shares one namespace, so files of a module need no imports between them.
- The project root is the nearest directory, starting at the entry, that holds a `low.mod` file; without one it is the entry's directory. `import ui/text` loads `<root>/ui/text/`. A path starting with `std/` names a module of the standard library, which is compiled into `low`, so a project directory named `std` cannot be imported.
- `low build dir` builds the directory as the entry module. `low build file.low` builds that one file as the entry module, without the other files beside it.
- Imports come before declarations and apply to their file. A module is used through its last path segment, `text.shout(...)`, or through a name given with `import t = ui/text`. Types are written `geometry.Shape` and built with `geometry.Point { ... }`; a variant can be named `geometry.Shape.Dot` or, where the type is expected, `Dot`.
- Declarations and struct fields are private to their module unless marked `pub`. A `pub` enum's variants are public. A struct with private fields without defaults cannot be built with a literal outside its module.
- An import cycle is an error, and so is importing two modules under one name.

### Generics

```low
struct Stack[T] {
    items: []T
}

enum Tree[T: Ord] {
    Node(*Tree[T], T, *Tree[T])
    Leaf
}

fn max[T: Ord](a: T, b: T) -> T {
    if a > b { a } else { b }
}

fn sum[T: Number](xs: []T) -> T {
    var total: T = 0
    for x in xs {
        total += x
    }
    total
}

fn sort[T](mut xs: []T, less: fn(T, T) -> bool) { ... }

print(max(3, 9))
print(max[u8](200, 100))
var s = Stack[str] { items: [] }
let p = Pair { first: 1, second: "x" }
let t: Tree[i64] = Leaf
let pick: fn(i64, i64) -> i64 = max
```

- Functions, structs and enums take type parameters in brackets after their name. Generic types are written with arguments, `Stack[i64]` or `geometry.Labelled[str]`.
- A constraint lists what a type parameter supports, joined with `+`:
  - `Eq` allows `==` and `!=`, and is met by numbers, `bool`, `str`, pointers, channels, and structs and enums whose fields are all `Eq`.
  - `Ord` adds `<`, `<=`, `>` and `>=`, and is met by numbers and `str`.
  - `Hash` adds use as a map key, and is met by integers, `bool`, `str` and pointers.
  - `Number` adds arithmetic, numeric literals, conversion with `T(x)` and `print`.
  - `Integer` adds bitwise operators and shifts and implies `Hash`.
  - `Float` is met by `f64` and `f32`.
  - An unconstrained parameter can only be moved, stored and passed. Anything else a generic function needs, such as an ordering of its elements, is passed in as a function value.
- A generic body is checked once against its constraints, so a missing constraint is reported at the definition, not at a use.
- Each generic function's type parameters are its own, so two functions may both name a parameter `T`, and a function type mentioning one is distinct from the same text mentioning the other.
- Type arguments are inferred from the arguments, then from the expected type. They can be written explicitly as `max[u8](a, b)` or `Stack[str] { ... }`. A generic struct literal or enum variant takes its arguments from the expected type or from its field values. A generic function used as a value takes them from the expected function type.
- Each distinct set of type arguments compiles to its own copy, `max[i64]` and `max[f64]`, so generic code runs as fast as code written for one type.
- Values of different kinds that share behaviour, such as the elements of a UI tree, are structs of function values whose closures capture each kind's state.

### Concurrency

```low
fn worker(jobs: chan[i64], results: chan[i64]) {
    for j in jobs {
        results <- j * 2
    }
}

let jobs = chan[i64](16)
let results = chan[i64](16)
spawn worker(jobs, results)
jobs <- 21
close(jobs)
print((<-results) ?? 0)
let next = select {
    v = <-results => v ?? -1
    jobs <- 5 => 0
    after(100) => -2
    else => -3
}
sleep(10)
wait_readable(fd)
```

- `spawn f(args)` starts a green thread running the call. The function value and the arguments are evaluated before it starts; `mut` arguments are not allowed. A function that can fail must have its error handled inside the spawned call.
- `chan[T](n)` makes a channel holding up to `n` values; `chan[T]()` is unbuffered, so every send waits for a receiver. A channel is a reference: assigning or passing it shares it.
- `c <- v` sends, waiting while the buffer is full. `<-c` receives a `T?`, waiting while the channel is empty, and gives `none` once it is closed and drained. `close(c)` wakes every waiter; sending on or closing a closed channel panics.
- `for v in c` receives until the channel is closed and drained.
- `select` waits on several operations at once and runs the arm of the first that can proceed. Arms are `name = <-c`, `<-c`, `c <- v`, `after(ms)` and `else`. Channel and value expressions are evaluated first, in order; ready arms are taken in order; `else` runs when nothing is ready; without `else` the green thread waits, and `after(ms)` fires once `ms` milliseconds pass. `select` is an expression like `match`.
- `sleep(ms)` pauses the green thread. `wait_readable(fd)` and `wait_writable(fd)` wait until a file descriptor is ready, using epoll; one green thread may wait on a descriptor at a time.
- A program ends when `main` returns, whatever other green threads are doing. When every green thread is waiting and nothing can wake one, the program panics with `every green thread is blocked forever`. A green thread that exhausts its stack panics with `stack overflow`.

## Standard library

```low
import std/file
import std/http
import std/http/websocket
import std/io
import std/net

fn handle(req: http.Request) -> http.Response {
    if req.path == "/chat" {
        using ws = websocket.accept(req) catch {
            _ => return http.response(400, "")
        }
        let m = websocket.receive(ws) catch {
            _ => return http.upgraded()
        }
        websocket.send_text(ws, str(m.data)) catch {
            _ => {}
        }
        return http.upgraded()
    }
    http.response(200, file.read_text("index.html") catch "missing")
}

fn main() ! io.Error {
    let client = http.client().with_bearer_token("token").with_timeout(5000).build()
    let page = client.get("https://example.com/") catch {
        _ => return
    }
    using l = net.listen("", 8080)?
    http.serve(l, handle)?
}
```

`import std/<name>` loads a module of the standard library, which is compiled into `low`; the project root is not searched for it. Every module except `std/math` and `std/sql/sqlite` talks to the kernel through raw syscalls. `std/math` calls libm and `std/sql/sqlite` calls libsqlite3 through `extern` blocks, and a library is linked only when reachable code calls it, so a program stays static unless it calls one of their C functions.

| Module | Contents |
|---|---|
| `std/io` | `Error`, the error type shared by the I/O, codec and protocol modules; `Stream`, a copyable struct of `read`, `write` and `close` closures that views a resource; `Guard` and `Lease`, the shared state that makes a closed descriptor fail with `NotConnected` instead of reaching a reused descriptor (`guard`, `enter`, `retire`); a buffered `Reader` over a stream or, with `bytes_reader`, over a buffer, with `read_line`, `read_exact`, `read_some`, `read_byte`, `buffered` and `fill`; `write_all`, `read_all`, `buffer(n)`, `copy` and big- and little-endian byte helpers. |
| `std/file` | `open`, `create`, `append`, `update`, which return a droppable `File` whose `handle` is a copyable `FileHandle`, `read`, `write`, `write_all`, `seek`, `sync`, `truncate`, `close`, `info`, `stat`, `read_on`, `write_on`, `write_all_on`, `close_on` and `stream_on` on the `FileHandle`, `exists`, `read_file`, `read_text`, `write_file`, `write_text`, `remove`, `remove_dir`, `rename`, `mkdir`, `mkdir_all`, `list` and `stream`; `create_mode`, `write_file_mode`, `write_text_mode`, `mkdir_mode` and `mkdir_all_mode`, which take the permission bits (the others use `0o644` for files and `0o755` for directories, and all are filtered by the process umask), and `chmod`, which is not; `try_lock` and `unlock`, advisory `flock` locks that report a held lock as `false` instead of waiting; `remove_all`, which removes a tree without following symbolic links; and `temp_dir`, which creates a private directory under `/tmp`. `Info` also carries `regular`, `link`, `accessed`, `uid`, `gid`, `device` and `inode`. `lstat`, `is_dir`, `is_file`, `is_symlink`, `symlink`, `link`, `readlink`, `realpath`, `chown`, `lchown`, `set_times` and `disk_usage`; `read_dir_info`; `walk`, a sorted depth-first walk that uses `lstat`, never descends into symbolic links and lets its callback return `Continue`, `Skip` or `Stop`; `glob`, which matches each segment with `path.matches` and supports `**`; and `copy` and `copy_dir`, which keep file modes and recreate symbolic links. `temp_file(dir, pattern)` creates a uniquely named file with `O_EXCL` and mode `0600` and returns a droppable `TempFile` that closes and removes it unless `keep` is called, and `temp_dir_pattern` makes a `0700` directory the same way. |
| `std/file/watch` | inotify: `watch` and `watch_with` over paths with `Options` (`recursive`, `access`), a droppable `Watcher` with a copyable `WatcherHandle`, `add`, `remove`, `next`, which parks the green thread until an `Event` (`watched`, `name`, `path`, `kind`, `dir`, `cookie`) arrives, `events`, a `chan[Event]` fed by a forwarding green thread, and `stop`. `Kind` is `Created`, `Modified`, `Deleted`, `MovedFrom`, `MovedTo`, `Attrib`, `Closed`, `ClosedUnmodified`, `Opened`, `Accessed`, `SelfDeleted`, `SelfMoved` or `Overflow`. |
| `std/net` | IPv4 and IPv6 `Addr`, `parse_ip`, `format_ip`, `parse_addr`, `format`, `in_network`, which tests an address against a CIDR prefix, and `is_global`, which is false for the addresses IANA's special-purpose registries mark as not globally reachable and for multicast; the droppable `Listener`, `Conn` and `Socket`, each with a copyable `ListenerHandle`, `ConnHandle` and `SocketHandle` in its `handle` field, and `adopt` and `detach` between a `ConnHandle` and a `Conn`; TCP `listen`, `accept`, `accept_handle`, which returns the connection's view for a green thread to adopt, `connect`, `dial` and their `_within` forms with a connect timeout, `connect_through`, which binds the socket to a network interface first, and `connect_first`, which tries a list of addresses in order through an optional interface; `interface_index`; `read`, `write`, `write_all`, `close`, `shutdown_write`, `set_nodelay`, read and write timeouts and `stream`, with `read_on`, `write_on`, `write_all_on`, `close_on` and `stream_on` on the view; UDP `bind_udp`, `send_to`, `receive_from` and `close_socket`; `lookup`, which tries an IP literal, `/etc/hosts`, then A and AAAA queries to each `/etc/resolv.conf` nameserver. An `Addr` with an empty `ip` is a Unix address: `listen_unix`, `connect_unix`, `connect_unix_within` and `socketpair` return the same `Listener` and `Conn`, abstract names start with `@` or NUL, and a listener unlinks its pathname when closed; `peer_credentials`; `listen_with` and `bind_udp_with` take `ListenOptions` (`reuse_addr`, `reuse_port`, `backlog`); `set_keepalive`, `set_buffer_sizes`; UDP multicast through `join_group`, `leave_group`, `set_multicast_ttl`, `set_multicast_loop` and `set_multicast_interface`; and `Dialer`, a function value that makes a connection, with `direct_dialer`. |
| `std/socks` | SOCKS5 (RFC 1928, with RFC 1929 username and password) and SOCKS4a clients: `dial`, `dial4a`, `tunnel` and `tunnel4a` over an existing connection, and `dialer` and `dialer4a`, which return a `net.Dialer`. The proxy resolves the target name. |
| `std/pipe` | Pipelines: a `Pipe` that owns a `Reader` and a `Writer`, each droppable so that releasing it completes it, with copyable `ReaderHandle` and `WriterHandle` views for green threads (`adopt_reader`, `adopt_writer` and the `_on` operations), segmented buffers, backpressure, `Sequence` and `SequenceReader`, and adapters between pipes and streams. |
| `std/cancel` | `Token`: `token`, `child`, `cancel`, `cancelled`, `check`, `done`, a channel that closes on cancellation for `select`, `on_cancel`, which returns a droppable `Registration` that releases its callback, `release`, `on_cancel_handle` and `release_on` on the copyable `RegistrationHandle`, and `pause`, a sleep that ends early on cancellation. |
| `std/http` | `Request`, whose `token` is cancelled when the client goes away and when the handler returns and whose `remote` is the client's IP address for connections served by `serve`, `serve_tls` and `serve_conn`, `Response`, `Header`, header helpers, `parse_url`; `serve`, `serve_tls`, `serve_conn` and `serve_stream`, whose `_on` forms (`serve_on`, `serve_tls_on`, `serve_conn_on`) take the listener's or connection's view; a pooled client configured through `client()` and `with_*` builders, including `with_interface`, which binds every connection to a network interface, `with_resolver`, which replaces `net.lookup` for host names while TLS keeps the host name for SNI and certificate checks, and `with_cancel`, which makes a request fail with `Cancelled` once its token is cancelled, with per-request builders, `send`, `stream_to` and `download`; URL parsing, resolution and encoding; `connect`, which returns a droppable `Connection` that closes its stream when released, and `detach`, which hands the stream on. `with_proxy`, `with_http_proxy`, `with_https_proxy`, `with_no_proxy`, `with_proxy_from_environment` and `with_dialer` send requests through an `http://` proxy (absolute-form for `http` targets, `CONNECT` for `https` targets) or a `socks5://`, `socks5h://`, `socks4://` or `socks4a://` proxy; `no_proxy` follows Go's matching rules. `CookieJar` is a shared RFC 6265 jar (`cookie_jar`, `with_cookie_jar`, `jar_set`, `jar_header`, `jar_cookies`, `jar_clear`) that enforces the `Secure` rules and the `__Secure-` and `__Host-` prefixes and has no public-suffix check. A `Response` whose `stream` is a `chan[[]u8]` is written as it is produced, chunked on HTTP/1.1 and as DATA frames on HTTP/2, until the handler closes the channel. |
| `std/http/websocket` | `accept` and `dial` over `ws` and `wss`, which return a droppable `Socket` that sends a normal close when released, `receive`, `send_text`, `send_binary`, `ping` and `close`, with `_on` forms and `adopt` and `detach` on the `*SocketHandle`. |
| `std/http/sse` | Server-sent events. `Event` (`id`, `event`, `data`, `retry`), `format`, `comment` and `keepalive`; the server side `stream`, `stream_with`, `serve_events`, `serve_events_with`, `Emitter` and `last_event_id`; the client side `parser`, `feed`, `parse` and `listen`, which reads a `text/event-stream` response with any chunking and CR, LF or CRLF line ends. |
| `std/http/files` | `serve_file` and `serve_file_with` answer a request from a file: content type from the extension then sniffing, weak `ETag` and `Last-Modified` with `If-None-Match` and `If-Modified-Since`, a single `Range` as 206 or 416 with `If-Range`, `HEAD`, and a streamed body. |
| `std/web` | A router with groups, middleware and typed JSON endpoints, problem details (RFC 9457), an OpenAPI 3.1 document built from the types, cookie sessions, CSRF protection, API key, Basic and bearer authentication, CORS and response compression. `static` and `static_with` serve a directory, and `events` and `event_stream` serve server-sent events. |
| `std/tls` | TLS 1.2 and 1.3 clients and servers: `client`, `server` and `dial` return a droppable `Session` that sends `close_notify` and closes the stream beneath it when released, whose `handle` is the shared `*SessionHandle` holding `protocol`, `version`, `group`, `suite`, `cipher_suite` and `peer_certificates`; `client_handle` and `server_handle` return the view alone, and `read_on`, `write_on`, `close_on` and `stream_on` operate on it. |
| `std/serde` | The `Writer` and `Reader` protocols, `write[T]`, `read[T]` and `schema[T]`, which the compiler derives for every type, and `Field[T, F]`. |
| `std/json` | A reader and writer over UTF-8 bytes, the `Value` document model, `equal`, which compares numbers numerically and objects without regard to member order, and `serialize[T]` and `deserialize[T]` with naming policies. |
| `std/json/pointer` | RFC 6901 pointers: `escape`, `unescape`, `parse`, `format`, `get`, `has`, `set`, `add`, `replace`, `remove` and relative pointers. Edits return a new document. |
| `std/json/patch` | RFC 6902 `apply`, atomic and failing with the operation's index, `diff`, and RFC 7396 `merge_patch` and `merge_diff`. |
| `std/json/path` | RFC 9535 JSONPath: `compile`, `query`, `find`, `values` and `paths`, with slices, filters, `match`, `search`, `length`, `count` and `value`, normalised paths, and syntax errors at a byte offset. |
| `std/json/schema` | JSON Schema 2020-12: `compile` and `compile_with` check the schema, resolve `$ref`, `$anchor`, `$dynamicRef` and registered resources, and precompile patterns; `validate` returns `Violation`s with the instance pointer, the keyword pointer and a message; `unevaluatedProperties` and `unevaluatedItems` use annotations from the branches that passed; `format` annotates or asserts. |
| `std/cbor` | CBOR (RFC 8949): the `Value` model, a decoder with depth and length limits, a deterministic encoder, diagnostic notation, and `serialize[T]` and `deserialize[T]`. |
| `std/webauthn` | A WebAuthn relying party: registration and authentication options as JSON, registration and authentication verification for ES256, EdDSA and RS256 credentials, and COSE key parsing. |
| `std/sql` | The driver contract: `Connection`, the droppable owner that closes the driver connection when it is released; `ConnectionHandle`, the copyable view of closures that `std/orm` and closures use (`handle` makes it from an owner, `adopt` the other way); `Value`, `Rows`, `Error`, `Options` and dialect helpers. |
| `std/sql/postgres` | A PostgreSQL driver over protocol 3.0, plain or over TLS. |
| `std/sql/sqlserver` | A Microsoft SQL Server driver over TDS 7.4, plain or over TLS. |
| `std/sql/sqlite` | SQLite through libsqlite3, linked only when a program calls it: `open` and `open_with` (read-only, create, busy timeout), `exec` for scripts, `query`, `each` and `execute` with `?` or `?N` parameters over cached prepared statements, `last_insert_id`, `begin`, `begin_immediate`, `commit` and `rollback`, `backup` to a file through the online backup API, and `close` and `close_on` (idempotent; a closed `Database` fails with `sql.Error.Closed`), and `connection` and `connect`, which present a database as an `sql.Connection`: the one `connection` returns leaves closing to the `Database`, the one `connect` returns closes it. Calls block their OS thread. |
| `std/sql/pool` | A pool of connections made by an opener: `open` with a `Config` (`max`, `max_idle`, `idle_timeout`, `max_lifetime`, `wait_timeout`, `ping_after`, `ping`, `clock`), `acquire` and `acquire_until` with a cancel token, which park in first-in first-out order while every connection is in use, a droppable `Lease` that returns its connection on release, `with`, `discard`, `prune`, `stats` and `shutdown`. |
| `std/sql/migrate` | Ordered migrations recorded in a `schema_migrations` table with a SHA-256 checksum: `plan`, `with_table`, `status`, `up`, `up_to` and `down`, each migration in its own transaction under a lock, `from_text` and `load_dir` for `-- +up` and `-- +down` files named `NNN_name.sql`. |
| `std/toml` | TOML 1.0.0: a table tree, an encoder, and serde binding. |
| `std/yaml` | YAML 1.2.2: events, a node tree, an encoder, and serde binding. |
| `std/otel` | OpenTelemetry traces, metrics and logs with W3C propagation, OTLP/HTTP protobuf and JSON, OTLP/gRPC and console exporters, and the instrumentation used by `std/http`, `std/web`, `std/sql/postgres`, `std/sql/sqlserver` and `std/orm`. |
| `std/protobuf` | The protobuf wire format: an encoder with nested messages and packed fields, and a field decoder. |
| `std/xml` | XML 1.0 with namespaces: a pull reader, a tree, a writer, and serde binding. |
| `std/orm` | Typed queries over field references, row mapping, inserts, updates and deletes, tables created from types, and raw queries mapped onto structs, over an `sql.ConnectionHandle`. |
| `std/compress/deflate` | DEFLATE, zlib and gzip. |
| `std/compress/xz` | xz with LZMA2, and `.lzma`. |
| `std/compress/zstd` | Zstandard (RFC 8878). |
| `std/compress/lz4` | LZ4 frames. |
| `std/compress/brotli` | Brotli (RFC 7932). |
| `std/compress/bzip2` | `compress(data, level)`, `decompress(data, limit)`, a streaming `decode` and an `Encoder`. The decoder reads multiple streams and blocks and checks both CRCs; randomised blocks fail with `Unsupported`. |
| `std/compress/snappy` | Raw blocks (`compress_block`, `decompress_block`, `decoded_length`) and the framing format with masked CRC-32C (`compress`, `decompress`, `decode`, `Encoder`). |
| `std/hash` | CRC-32, CRC-32C, Adler-32, CRC-64 and xxHash32/64, in one-shot and incremental forms. FNV-1 and FNV-1a in 32 and 64 bits, SipHash-2-4 and SipHash-1-3 with a 16-byte key in 64- and 128-bit forms, MurmurHash3 (`murmur3_32`, `murmur3_128`), wyhash final 4.2, `jump` consistent hashing and `rendezvous` hashing. |
| `std/bits` | Dense bitsets. |
| `std/bits/roaring` | Roaring bitmaps with the portable serialization format. |
| `std/image` | Bitmaps, pixel operations and resizing, with PNG, JPEG, GIF, BMP, QOI and lossless WebP codecs. |
| `std/crypto` | `sha1` and `random`, which reads `getrandom`. |
| `std/crypto/hash` | `Algorithm` and cloneable `Digest`, SHA-256, SHA-384 and SHA-512, HMAC, HKDF and constant-time `equal`. |
| `std/crypto/sha3` | SHA3-256, SHA3-384, SHA3-512, SHAKE128 and SHAKE256. |
| `std/crypto/aead` | AES-128-GCM, AES-256-GCM, ChaCha20-Poly1305 and XChaCha20-Poly1305 (`xchacha20_poly1305`, 24-byte nonce) as `Aead` values, the AES block functions for 128-, 192- and 256-bit keys (`aes_key`, `encrypt_block_into`, `decrypt_block`, `decrypt_block_into`) and the ChaCha20 functions `chacha20_xor`, which takes the RFC 8439 block counter, and `hchacha20`. |
| `std/crypto/md5` | MD5 as a `hash.Algorithm`, so HMAC, HKDF and PBKDF2 work over it, and `sum`. For interop and checksums, not security. |
| `std/crypto/cipher` | AES in CBC, raw (`cbc_encrypt`, `cbc_decrypt`) or with PKCS#7 padding (`cbc_seal`, `cbc_open`, `pkcs7_pad`, `pkcs7_unpad`), in CTR with a 128-bit big-endian counter (`ctr`), and in XTS with ciphertext stealing (`xts`, `xts_encrypt`, `xts_decrypt`). There is no ECB, and CBC provides no integrity: use an `Aead`. |
| `std/crypto/x25519`, `std/crypto/ed25519` | X25519 key agreement (RFC 7748) and Ed25519 signatures (RFC 8032), over `std/crypto/curve25519`. |
| `std/crypto/ecdsa` | ECDH and ECDSA on P-256 and P-384, with RFC 6979 nonces and SEC1 private keys. |
| `std/crypto/rsa` | RSA PKCS#1 v1.5 and PSS signing and verification, and PKCS#1 keys. |
| `std/crypto/blake3` | BLAKE3: `sum`, `xof`, `keyed_hash`, `derive_key`, an incremental `Hasher` with `finalize` and seekable `finalize_at`, and `algorithm()`, a `hash.Algorithm` for HMAC. |
| `std/crypto/otp` | HOTP (RFC 4226) and TOTP (RFC 6238) over SHA-1, SHA-256 and SHA-512: `hotp`, `totp`, `verify_totp` with a window, `verify_hotp`, `generate_secret`, base32 secrets, and `otpauth://` `uri` and `parse_uri`. |
| `std/crypto/jose` | JSON Web Tokens: compact JWS with HS256/384/512, RS256/384/512, PS256/384/512, ES256, ES384 and EdDSA, JWK sets, and claim checks. |
| `std/crypto/mlkem` | ML-KEM-512, -768 and -1024 (FIPS 203). |
| `std/crypto/password` | Argon2id, scrypt (RFC 7914, with a memory limit checked before allocating) and PBKDF2 password hashes in PHC strings, bcrypt (`$2b$`; passwords over 72 bytes are rejected), `verify` for all of them and for `$2a$` and `$2y$` bcrypt strings, and BLAKE2b. |
| `std/crypto/x509` | DER, PEM, certificates, PKCS#8 private keys, the system roots, chain validation and TLS signature schemes. |
| `std/time` | `now`, in Unix seconds, `now_millis` and `now_nanos`, the wall clock at finer resolution, `monotonic`, in nanoseconds, `sleep_for`, which rounds up to whole milliseconds, `after`, `Timer` and `Ticker`, whose channels work in `select`, and `Stopwatch`. Calendar and zone types live in the submodules below, one module per type, so `d.plus(span.days(3))` resolves to `date.plus`. |
| `std/time/fault` | `Fault` (`OutOfRange`, `Invalid`, `Syntax(at)`, `Overflow`, `UnknownZone`, `Malformed`, `Unsupported`, `Skipped`, `Ambiguous`, `OffsetConflict`), the error every fallible time function returns, and `to_string`. |
| `std/time/duration` | `Duration`, an exact signed elapsed time with private seconds and nanoseconds, from `nanoseconds` to `hours`, `from_seconds_f64`, `plus`, `minus`, `times`, `divide`, `ratio`, `negate`, `abs`, the `try_` forms returning an optional, `round` and `truncate` with a `Rounding`, comparison, `to_string` (ISO 8601 `PT1H30M`), `friendly` (`1h 30m`) and `parse`, which reads both forms and fails at the offset of the first bad character. Overflow in the plain forms panics. |
| `std/time/timestamp` | `Timestamp`, a point on the UTC timeline from 0001-01-01 to 9999-12-31 with nanosecond precision: `now`, `from_unix` in seconds, milliseconds or nanoseconds, `plus` and `minus` by `Duration`, `until` and `since`, `round`, `truncate`, RFC 3339 `to_string` and `parse` (offset required, `:60` read as `:59`), and `to_http_date` and `parse_http_date` (IMF-fixdate, RFC 850 and asctime). |
| `std/time/clock` | `Clock`, a pair of closures for the wall and monotonic readings, with `system`, `fixed` and `manual`; a `Manual` clock moves with `advance` and `set`, so code that asks for the time is tested without sleeping. |
| `std/time/span` | `Span`, a calendar amount of years, months, weeks, days and time units with one sign, `Unit`, `Diff` (`largest`, `smallest`, `increment`, `mode`), `to_duration` and `from_duration`, ISO 8601 `to_string` (`P1Y2M3DT4H`), `friendly` and `parse`. |
| `std/time/rounding` | `Rounding` (`Trunc`, `Ceil`, `Floor`, `Expand` and the five `Half` modes) and the helpers the other time modules share. |
| `std/time/date` | `Date` with private fields and validated constructors (`of`, `clamped`, `from_epoch_days`, `from_ordinal`, `from_iso_week`, `from_weekday_in_month`), `Month`, `Weekday`, `IsoWeek`, `plus` and `minus` by `Span` with month-end clamping, `until` and `since` with a `Diff`, `next_weekday` and `on_or_next`, `range`, ISO 8601 `to_string` and `parse` (calendar, ordinal and week dates), and `format` and `parse_with` over a pattern. |
| `std/time/timeofday` | `TimeOfDay` to the nanosecond, `plus` that fails past midnight and `plus_wrapping` that wraps, `until`, `round`, `truncate`, `to_string`, `parse` and pattern `format` and `parse_with`. |
| `std/time/datetime` | `DateTime`, a civil date and time of day with no zone: `plus` by `Span` or `Duration`, `until` and `since` with a `Diff`, `round`, `truncate`, `to_string`, `parse` (an offset is rejected) and pattern `format` and `parse_with`. |
| `std/time/offset` | `Offset`, a UTC offset in whole seconds within ±25:59:59, with `utc`, `hours`, `hm`, `to_string`, `to_string_z` and `parse`. |
| `std/time/zone` | IANA zones: `Zone`, a shared value decoded once, from `named` (the host database, honouring `ZONEINFO`), `load_from` (any `fn(str) -> []u8?`), `local` (`TZ`, `/etc/localtime`, `/etc/timezone`, then UTC), `from_posix`, `decode`, `utc` and `fixed`; `info_at`, `candidates` (`One`, `Overlap`, `Gap` for a civil time), `next_transition`, `previous_transition` and `transitions`. |
| `std/time/tzdata` | An embedded copy of the database for hosts without one: `source`, for `zone.load_from`, `names` and `version`. The data is linked only when `source` or `names` is reachable. |
| `std/time/zoned` | `Zoned`, a `Timestamp` seen through a `Zone` whose offset is always the zone's own, built from a civil time with a `Disambiguation` (`Compatible`, `Earlier`, `Later`, `Reject`) or `strict`; `plus` by `Span` moves calendar units on civil dates and time units by elapsed time, `plus_duration` is exact, `until` and `since` with a `Diff`, `start_of_day`, `hours_in_day`, `series`, `round`, RFC 9557 `to_string` (`2026-11-11T10:00:00+00:00[Europe/London]`), `to_rfc3339`, `to_rfc2822`, `parse` with an `OffsetPolicy` for a written offset that disagrees with the zone, and pattern `format` and `parse_with`. |
| `std/time/pattern` | Compiled strftime and strptime patterns shared by the types above: `compile`, `Names` for other languages, `render` and `scan`; verbs `%Y %y %C %m %d %e %j %b %B %a %A %u %w %U %W %V %G %H %I %M %S %f %N %p %P %z %:z %::z %Z %s %F %T %R %D %% %n %t`, and the flags `-`, `_`, `0` and `^`. Parsing is strict, and every error carries a byte offset. |
| `std/time/humanize` | `ago`, `relative`, `in_words`, `compact` and `round_to` over a `Words` value; English by default. |
| `std/simd` | Vector reads and writes, `splat`, `mask`, `blend`, `min`, `max`, lane access, byte shuffles, bit counting and `find_byte`. |
| `std/text` | Integer parsing in any base from 2 to 36, correctly rounded float parsing, `parse_bool`, `radix`, `hex` and hex encoding, and `fixed(x, places)`, which rounds half to even on the exact binary value. `trim` and its start, end, prefix and suffix forms; `split` on a byte, `split_str` on a string (an empty separator splits into code points), `fields`, `lines` (dropping a final empty line and `\r` before `\n`), `chars` and `join`; `index`, `last_index`, `contains`, `count`, `starts_with` and `ends_with`; `repeat` and `replace`; ASCII `upper`, `lower` and case folding; `char_count`, `from_char`, `pad_left` and `pad_right` to a width in code points; `append` and `append_char` onto a `[]u8`; and comma-separated token lists. `cut` and `last_cut` return a `Cut` of `before`, `after` and `found`; `strip_prefix` and `strip_suffix` return `str?`; `split_n`, `title`, `quote` and `unquote` (backslash, `\x`, `\u` and `\U` escapes, failing with `QuoteError`), `edit_distance` over code points, `wrap` to a width, `is_ascii` and `reverse`. |
| `std/text/diff` | Myers line diff: `diff`, `lines`, `stats`, `distance`, `similarity`, GNU-compatible `unified` and `hunks`, `patch` and `patch_within` over a unified diff, and word and character level `inline`, `inline_chars` and `inline_markup`. |
| `std/text/table` | `Table` with per-column `Align`, width limits, truncation and wrapping, and `Border` styles `None`, `Ascii`, `Unicode`, `Markdown` and `Tabs`; widths use `unicode.text_width` and ignore ANSI sequences. `align` and `align_tabbed` align tab-separated cells like Go's `tabwriter`. |
| `std/text/naming` | `words`, `snake`, `kebab`, `dotted`, `screaming_snake`, `camel`, `pascal`, `title`, `sentence`, the matching `is_` predicates, and `slug` and `slug_with`, which transliterate Latin letters and cut at a word boundary. |
| `std/text/number` | `format_bytes` and `parse_bytes` in decimal and binary units, `grouped`, `ordinal`, `percent`, `compact`, `si`, English `words`, and `roman` with `parse_roman`; arithmetic is exact. |
| `std/builder` | A `Builder` over a growing byte buffer, appended in place through `mut`: `write`, `write_bytes`, `write_byte`, `write_char`, `write_i64`, `write_f64`, `write_line`, `write_repeat`, `write_join`, `len`, `capacity`, `reserve`, `truncate`, `reset`, `to_str` and `to_bytes`. |
| `std/regex` | Linear-time regular expressions in the RE2 style: `compile` and `compile_with` with `Flags`, `generated` for a pattern compiled into specialised code at build time, `is_match`, `find`, `full_match`, `find_all`, `replace_all` with a closure receiving each `Match`, `split` and `split_n`, and `start`, `end`, `text`, `group` and `span` on a match, with byte offsets. |
| `std/unicode` | Unicode 16.0.0 normalization: `nfd`, `nfkd`, `nfc` and `nfkc`; `combining_class`; full case folding with `casefold`; and `search_key`, which case-folds, applies NFKD and drops combining marks in one pass. `category`, `is_letter`, `is_upper`, `is_lower`, `is_digit`, `is_space`, `is_punct`, `is_identifier_start` and the other predicates; `to_upper`, `to_lower` and `to_title`, and `upper` and `lower` with SpecialCasing but without final-sigma or locale rules; `east_asian_width`, `display_width` and `text_width`; `script`; UAX #29 `graphemes`, `grapheme_count` and `grapheme_end`; and UTF-8 helpers (`decode_rune`, `encode_rune`, `utf8_valid`, `utf8_to_valid`). Category, width and identifier data are Unicode 16.0.0; script, grapheme break and case data come from 17.0.0 restricted to characters assigned in 16.0.0. The tables are generated by `cmd/tablegen` and linked only when a function that reads them is reachable. |
| `std/unicode/utf16`, `std/unicode/utf32` | Encoding and decoding of code points, `[]u16` and `[]u32`, and byte forms in either order with BOM handling, strict or lossy, with surrogate validation. |
| `std/charset` | `decode`, `decode_lossy`, `encode`, `encode_replace`, `canonical` (WHATWG labels) and `detect_bom` for UTF-8, UTF-16, UTF-32, ISO-8859-1, -2 and -15, windows-1250, -1251 and -1252, KOI8-R, IBM437 and IBM850. The CJK encodings are not provided. |
| `std/html` | An HTML tokenizer: a pull `Tokenizer` returning `StartTag`, `EndTag`, `Text`, `Comment`, `Doctype` and `End` tokens, `attribute` lookup and `unescape` for character references. |
| `std/math` | libm's trigonometric, hyperbolic, exponential and rounding functions, `pi`, `tau`, `e`, `infinity`, `nan`, `is_nan`, `is_inf`, generic `abs`, `min`, `max` and `clamp`, and integer `ipow` and `gcd`. |
| `std/slices` | `contains`, `index`, `clone`, `concat`, `reverse`, `map`, `filter`, `fold`, `any`, `all`, `sum`, `min` and `max` as optionals, `sorted`, and an unstable in-place `sort` and `sort_by`: a quicksort with median-of-three pivots that recurses into the smaller side and finishes runs of 16 or fewer with insertion sort. `sort_stable` and `sort_stable_by` are a top-down merge sort, and `sort_by_key` is stable. `reduce`, `count`, `find`, `find_index`, `first`, `last`, `equal`, `compare`, `starts_with`, `ends_with`, `unique` (adjacent duplicates), `flat_map`, `flatten`, `repeat`, `lower_bound`, `binary_search` (the first match) and `binary_search_by`; `take`, `skip`, `chunks` and `windows`, which share the source's elements; `partition` and `zip`, over `Pair[A, B]`; and the in-place `fill`, `swap`, `rotate`, `insert` and `remove`. Slices are not named types, so these are called with a pipe: `xs \|> slices.sort()`. |
| `std/maps` | `keys`, `values`, `sorted_keys`, `entries`, `from_pairs`, `clone`, `merge` (the right map wins), `filter`, `map_values`, `has`, `get_or`, `invert`, `count_by` and `group_by`. |
| `std/os` | `args` and `environment` from the argument and environment vectors the program started with, `env` as `str?`, `umask`, which sets the creation mask and returns the previous one, `pid` and `exit`. `getcwd`, `chdir`, `hostname`, `uid`, `gid`, `euid`, `egid`, `ppid`, `executable`, `uptime`, `cpu_count`, `lookup_user` and `user_name` from `/etc/passwd`; `set_env` and `unset_env`, which `env`, `environment`, `process.find` and the environment `process.run` passes on see; and `home_dir`, `cache_dir`, `config_dir`, `data_dir`, `state_dir` (XDG variables when absolute, otherwise under `$HOME`) and `temp_dir_path`. |
| `std/os/signal` | `Signal` (`Hangup`, `Interrupt`, `Quit`, `Abort`, `Kill`, `User1`, `User2`, `Pipe`, `Alarm`, `Terminate`, `Child`, `Continue`, `Stop`, `TerminalStop`, `WindowChange`) with `number`, `name`, `from_number` and `catchable`; `notify` returns a droppable `Watch` (copyable `WatchHandle`) whose `signals` channel delivers the caught signals, with `wait_for`, `wait_timeout`, `wait_any` and `stop`; `ignore`, `reset`, `send` and `raise`. `Kill` and `Stop` cannot be caught and fail with `Invalid`. |
| `std/process` | `Command` with arguments, an optional replacement environment, a working directory, `Inherit`, `Piped` or `Null` for each standard stream, and an optional new process group; `start`, which returns a droppable `Child` that kills and reaps an unwaited process and closes its pipes when released, `start_handle`, `adopt`, `wait`, `kill`, `wait_on` and `kill_on` on the shared `ChildHandle`, `run`, which feeds input and collects both outputs, `find`, which searches `PATH`, and `success`. |
| `std/cdp` | The Chrome DevTools Protocol: `launch` starts a headless Chromium with a disposable profile and connects to it and returns a droppable `Browser` that shuts it down when released, whose `handle` is the shared `*BrowserHandle` (`launch_handle`, `adopt`, `shutdown_on` and `open_page_on` take it), `call` sends a command to the browser or a session and `events` delivers everything else; `open_page` opens a page in its own browser context whose every request, including those of cross-site frames and workers, passes through `Handlers`; `navigate` and `shutdown`. |
| `std/path` | Slash-separated path manipulation: `clean` (lexical, resolving `.` and `..`), `join`, `base`, `dir`, `ext`, `stem` and `is_absolute`; `matches` for shell globs with `*`, `?`, `[...]` classes (ranges, `!` or `^` negation) and `\` escapes, where no wildcard crosses a `/`. |
| `std/sync` | `Mutex` with `lock`, `try_lock`, `unlock` and `locked`; `WaitGroup` with `add`, `done` and `wait`; `Once` with `run`; `RwLock`, first in first out and writer-fair, with `read_lock`, `read_unlock`, `write_lock`, `write_unlock`, `try_read_lock`, `try_write_lock`, `reading` and `writing`; `Semaphore` with `acquire`, `try_acquire`, `release` and `available`; `Cond` with `wait_cond`, `signal` and `broadcast`; `Barrier` with `wait_barrier`; `OnceValue[T]`; and `ErrorGroup[E]`, whose `go` blocks while `limit` tasks run and whose `join` returns the first error. All are built on channels, so a waiting green thread parks instead of spinning. |
| `std/retry` | `Policy` with attempts, base, multiplier, maximum delay, `Jitter` (`None`, `Full`, `Equal`, `Decorrelated`), a deadline, a seeded `Rng` and a cancel token; `delay`, a loop driver `start` with `next` and `at_least`, and `run` over a `Step` (`Done`, `Retry`, `RetryAfter`, `Fatal`) giving an `Outcome`. |
| `std/ratelimit` | `Bucket` (token bucket in exact nanosecond units) with `allow`, `allow_n`, `reserve`, `wait` and `set_rate`; `Window`, a sliding log; `Keyed`, a bounded per-key set of buckets with idle eviction. All take a `clock.Clock`. |
| `std/singleflight` | `Group[K, V]` with `do`, `do_shared`, `forget` and `in_flight`, which run one call per key while the others wait for its result, and `Expiring[V]`, a value refreshed at most once per interval. |
| `std/atomic` | `Int`, `Bool` and `Flag` built on compare-and-swap, with `load_int`, `store_int`, `add_int`, `swap_int`, `compare_and_swap_int` and `update_int`, and the same forms for `Bool`; every load is a locked operation, so a loop cannot hoist it. `Value[T]` replaces a heap pointer under a lock: `load_value`, `store_value`, `swap_value`, `update_value`, `snapshot_value` and `compare_and_swap_value`, which compares pointer identity. |
| `std/rand` | A seedable, non-cryptographic xoshiro256** generator: `seeded`, `rng` (all 256 bits of state from `std/crypto`), `next`, unbiased `below` and `between`, `float`, `chance`, `shuffle` and `pick`. `pcg32`, `pcg64` and `chacha` (seeded from 32 bytes, `chacha_rng` from entropy) produce an `Rng` that every function accepts, and `next32` draws 32 bits. Distributions: `normal`, `exponential`, `gamma`, `beta`, `poisson`, `binomial`, `geometric`, `zipf`, `weighted_pick`, `cumulative` with `pick_cumulative`, `sample` and `permutation`. Use `std/crypto` for secrets. |
| `std/uuid` | Random v4 and time-ordered v7 UUIDs, `format`, `parse` with the offset of the first bad character, `version`, `timestamp` for v7, `equal` and `nil`. |
| `std/csv` | RFC 4180 records with quoted fields, doubled quotes, embedded newlines and CRLF or LF line ends: `parse` and `parse_with` a separator, failing with the line of an unterminated or bare quote, and `format`, `format_with` and `append_record`, which quote only fields that need it. |
| `std/dotenv` | `.env` files: `parse` and `read` return a map from `NAME=value` lines, allowing `export`, blank lines, `#` comments (inline after whitespace in unquoted values), single quotes taken literally, and double quotes with `\n`, `\t` and `\r` escapes that may span lines; errors carry the line. |
| `std/flag` | Command-line flags from `Spec`s with help text, booleans and defaults: `--name value`, `--name=value`, single-dash forms and `--`; `get`, `enabled`, `number`, the remaining arguments, and `usage`. |
| `std/log` | Leveled line logging, `debug`, `info`, `warn` and `error`, to a descriptor (stderr by default) as `level prefix: message`, without timestamps so journald and other collectors add their own. `with_prefix` nests a prefix. A `Logger` also carries attributes (`str_attr`, `int_attr`, `float_attr`, `bool_attr`, `with_attrs`, and the `_with` level forms), a `Format` of `Text` (`key=value`, quoting what needs it) or `Json` (one object per line), a `Sink` (a descriptor, an `io.Stream`, a line closure or a `Record` handler), optional RFC 3339 timestamps, and a level from `parse_level` or the `LOG_LEVEL` variable. Each record is one `write`, so lines from green threads do not interleave. |
| `std/term` | `is_tty`, `size` and `set_size`; `enter_raw`, which returns a droppable `Raw` that restores the terminal when released; `Key`, `read_key` (a lone escape resolves after 50 ms) and `parse_keys`; ANSI helpers (`fg`, `bg` with 16, 256 and RGB colours, `bold`, `dim`, `underline`, cursor movement, `clear_screen`, the alternate screen, `strip` and `visible_width`); `colour_enabled`, which honours `NO_COLOR`, `CLICOLOR_FORCE` and `TERM=dumb`; and `open_pty`. |
| `std/mime` | Media types by file extension and back (`by_extension`, `extension`) for web, image, font, archive, audio, video and subtitle formats, and `sniff`, which recognises content by its magic bytes or its leading text and falls back to plain text or `application/octet-stream`. |
| `std/semver` | Semantic Versioning 2.0.0: `parse` (with an optional `v`, failing at the offset of the first bad part), `format`, and `compare` and `less` with prerelease precedence, ignoring build metadata. |
| `std/container/heap` | A binary `Heap[T]` ordered by a `less` function: `insert`, `pop`, `peek` and `size`. |
| `std/container/lru` | A least-recently-used `Cache[K, V]` of fixed capacity: `get` (which refreshes the entry), `put` (which returns the evicted key), `contains`, `size` and `keys` from most to least recent, all in constant time over index-linked entries. |
| `std/container/deque` | A ring-buffer `Deque[T]`: `push_front`, `push_back`, `pop_front`, `pop_back`, `front`, `back`, `at` and `size`, doubling when full. |
| `std/container/set` | `Set[T]` over a map, in insertion order: `set`, `from_slice`, `add` and `remove`, which return whether the set changed, `has`, `size`, `items`, `union`, `intersection`, `difference` and `is_subset`. |
| `std/container/ordered` | An AVL `Tree[K, V]` for ordered keys: `tree`, `put`, `get`, `remove`, `contains`, `size`, `height`, `is_valid`, `first`, `last`, `floor_entry`, `ceiling_entry`, `each`, `each_range`, `keys`, `values`, `entries` and the half-open `range(lo, hi)`. |
| `std/container/sorted` | `Sorted[T]`, an ordered set over `ordered.Tree`: `sorted`, `from_slice`, `add`, `remove`, `has`, `size`, `items`, `first`, `last`, `floor_item`, `ceiling_item` and `range`. |
| `std/archive` | The shared filesystem layer of `std/archive/tar` and `std/archive/zip`: names are cleaned and rejected when absolute, containing `..` or NUL, or passing through a symbolic link below the destination, and files are created exclusively without following links. |
| `std/archive/tar` | ustar, GNU and pax archives: a streaming `Reader` (`next`, `read_body`, `read_entry`, `list`) with limits on header, entry and total size, a `Writer` (`add_file`, `add_dir`, `add_symlink`, `add_hardlink`, `finish`) that uses pax records for long or non-ASCII names, gzip through `read_gzip` and `write_gzip`, and `extract`, `extract_gzip`, `create` and `create_gzip` over the filesystem, which strip setuid, setgid and sticky bits. |
| `std/archive/cpio` | newc, CRC and odc archives, detected per entry on read: a `Reader`, a `Writer` (`add_file`, `add_dir`, `add_symlink`, `add_links`, `finish`), and `extract` and `create` over the filesystem with the safe extraction of `std/archive`. Hard links store their data with the last link in newc and CRC. The binary format fails with `Unsupported`. |
| `std/testing` | `T`, passed to a test `fn(*T)`, with `log`, `error`, `fatal`, `skip`, `require` and `sub`; assertions `equal`, `not_equal`, `equal_slice`, `is_true`, `is_false`, `is_some`, `is_none`, `contains`, `contains_item`, `close_to`, `unexpected`, `must` and `expect_error`, which render values of any type; `table` and `table_of`; `run` and `run_with`, which run each test on its own green thread with a timeout, filter with `-run`, print with `-v` and exit with status 1 on failure; `bench`, `bench_for` and `blackhole`; `golden`, which rewrites its file when `UPDATE_GOLDEN` is set; and `check_property`, `check_shrunk` and `check_ints`, which print the seed and shrink integers. |
| `std/archive/zip` | Zip archives read through the central directory, with zip64, stored and deflate entries, CRC-32 checks and limits on entries, sizes and compression ratio, and written with `add_file`, `add_dir`, `add_symlink` and `finish`, which emits zip64 records when a count or size needs them; `extract` over the filesystem. Encrypted entries and other methods fail with `Unsupported`. |
| `std/base32` | RFC 4648 base32 with the standard and extended-hex alphabets: `encode`, `encode_unpadded`, `decode` (case-insensitive, padding optional, failing at the first bad character or on an impossible length), `encode_hex` and `decode_hex`. |
| `std/base58` | The Bitcoin alphabet (`encode`, `decode`), the Flickr alphabet (`encode_flickr`, `decode_flickr`) and Base58Check (`encode_check`, `decode_check`), failing at the first bad character. |
| `std/ascii85` | `encode`, `encode_adobe` (with `<~ ~>`), `decode`, and Z85 (`encode_z85`, `decode_z85`). |
| `std/msgpack` | MessagePack: the `Value` model with timestamps and extensions, a decoder with depth and length limits that reports the offset of an error, the smallest-form `encode`, and `serialize[T]` and `deserialize[T]`. |
| `std/bson` | BSON documents as `[]Field` with ObjectId, datetime, binary, regex, int32, int64, double and Decimal128 (as bytes): `decode`, `encode`, `get`, `diagnostic`, and `new_object_id` and `parse_object_id`. |
| `std/ini` | INI files with sections, `=` or `:`, `;` and `#` comments, quoted values, continuations and duplicate keys (last wins, or all with `collect`): `parse`, `get`, `get_all`, `sections`, `set`, `format`. |
| `std/properties` | Java `.properties`: `parse`, `get`, `set`, `format`, with `\u` escapes and surrogate pairs. |
| `std/big` | `Int` over 32-bit limbs: parsing and printing in bases 2 to 36, `add`, `sub`, `mul` (Karatsuba), `divmod`, `divmod_floor`, `divmod_euclid`, `pow`, `modpow` (Montgomery for odd moduli), `mod_inverse`, `gcd`, `lcm`, `isqrt`, shifts and bitwise operators with two's complement semantics, `is_probable_prime`, `next_prime`, `random_below`, `to_f64` and `from_bytes` and `to_bytes`. Division by zero is an error. Not constant time. |
| `std/big/rat` | Normalised rationals over `big.Int`: `ratio`, `parse` (`a/b`, decimals, exponents), the four operations, `inv`, `cmp`, `to_fixed`, `to_f64`. |
| `std/big/decimal` | A coefficient and an `i32` exponent: exact `add`, `sub` and `mul`, and `div` and `rescale` with `HalfEven`, `HalfUp`, `Down`, `Up`, `Floor` or `Ceiling` rounding to a target exponent. |
| `std/redis` | RESP2 and RESP3 over TCP or TLS: a droppable `Client` with a `ClientHandle`, `connect` with AUTH, SELECT and HELLO 3, `command`, typed helpers for strings, hashes, lists, sets, sorted sets and `publish`, pipelining, `transaction`, pub/sub through a channel usable in `select`, command timeouts, and limits on bulk size and nesting. |
| `std/smtp` | An SMTP client: `connect`, `connect_starttls` and `connect_tls`, EHLO extensions, AUTH PLAIN, LOGIN, XOAUTH2 and CRAM-MD5 (plaintext authentication is refused unless `insecure_auth`), `send` with per-recipient results, dot-stuffing, pipelining when advertised, and errors separating 4xx from 5xx. |
| `std/mail` | Message building: `Message` with text and HTML alternatives and attachments, RFC 2047 encoded words, quoted-printable, base64 wrapped at 76, RFC 5322 dates, and rejection of CR and LF in any header. |
| `std/template` | A text template engine over a `Data` model (`to_data[T]` through serde): `{{ expr }}`, `if`, `for`, `include`, `block` and `extends`, `let`, trim markers, filters (built in and registered), HTML autoescaping, depth and output limits, and errors with line and column. A `Set` is read-only while rendering, so green threads share it. |
| `std/multipart` | multipart parsing and writing (RFC 7578, RFC 2046): a streaming `Reader` with limits and a spill sink for large parts, RFC 2231 and 5987 parameters, `Writer`, `Form`, and `request_form` for an `http.Request`. |

| `std/base64` | `encode` and `decode`, and `encode_url` and `decode_url` for the URL-safe alphabet without padding (RFC 4648 §5), which reject padding, other alphabets and non-zero trailing bits. |
### Resource ownership

Every standard library resource that must be disposed of is a droppable owner bound with `using`. `defer` is left for work that is not disposal: unlocking a lock, signalling a channel, restoring ambient state and logging.

- An owner keeps the resource's name (`file.File`, `net.Conn`, `net.Listener`, `net.Socket`, `tls.Session`, `http.Connection`, `websocket.Socket`, `process.Child`, `cancel.Registration`, `pipe.Reader`, `pipe.Writer`, `sqlite.Database`, `sql.Connection`, `cdp.Browser`, `otel.Sdk`) and holds its view in a public `handle` field. Its `drop` closes through the handle.
- A view is named `<Owner>Handle`: `FileHandle`, `ConnHandle`, `ListenerHandle`, `SocketHandle`, `SessionHandle`, `ChildHandle`, `RegistrationHandle`, `ReaderHandle`, `WriterHandle`, `DatabaseHandle`, `ConnectionHandle`, `BrowserHandle` and `SdkHandle`; `SocketHandle` names the view in both `net` and `websocket`. It is a copyable struct, or a pointer to the shared state when that state is mutable, so closures capture it, slices, channels, optionals and pointer-held structs store it, and `spawn` takes it. A view never closes anything on its own.
- Operations a caller uses directly take the owner and borrow it: `net.read(c, buf)`. Operations that run after ownership has moved elsewhere (green threads, `io.Stream` closures, pools, callbacks) take the view and end in `_on`: `net.read_on(h, buf)`, `net.stream_on(h)`, `net.close_on(h)`. A function that returns a view without an owner ends in `_handle`: `net.accept_handle`, `tls.client_handle`, `cdp.launch_handle`. `adopt(h)` makes an owner from a view, and `detach(mut owner)` gives up an owner's responsibility so a function that opened a resource and wrapped it can return the wrapper.
- Closing is explicit and idempotent from the owner, the view and the stream, and the drop that follows an explicit close does nothing. The closed state lives in the shared view. Descriptor-based views share an `io.Guard`: every operation enters it, `retire` marks it closed and wakes blocked operations, and the last operation to leave closes the descriptors, so a view used after its owner closed fails with `NotConnected` and never reaches a reused descriptor. The SQL drivers fail with `sql.Error.Closed` and cdp with `cdp.Error.Closed`.
- `io.Stream` is a view whose `close` closes everything beneath it, so `tls.client(net.stream(c), config)` closes the connection when the session closes, and the connection's own drop later does nothing.
- A droppable cannot be stored in a pointer, slice, map, channel, optional or enum variant, captured by a closure or passed to `spawn`, so long-lived and shared state holds views. The HTTP client's idle connections, its HTTP/2 connections and the reaper's deadlines hold `ConnHandle` and `io.Stream` views and close them explicitly with the `_on` operations. A green thread that should own a resource receives its view and binds the owner itself: `http.serve` accepts with `net.accept_handle` and each spawned connection thread runs `using c = net.adopt(h)`. Pump threads that only use a resource take its view and leave it to its owner. A scope that opens a resource for a wrapper it returns binds it with `using var` and calls `detach`, as `tls.dial` does.
- A droppable `process.Child` that was not waited for is killed and reaped when it is released, and a `pipe.Reader` or `pipe.Writer` completes when it is released, so a peer never waits on an abandoned end.

### Networking

- Sockets are non-blocking. An operation that would block waits with `wait_readable` or `wait_writable`, so a green thread waiting on the network never holds an OS thread the collector must wait for. Every connection holds a duplicate descriptor for its write side, so one green thread can read while another writes. A timeout spawns a green thread that sleeps and shuts the socket down if the operation is still waiting; the waiting operation then fails with `TimedOut`. A connection counts the reads and writes in progress. Closing it shuts it down, which wakes any green thread still waiting while the descriptor stays registered, and the last operation to finish closes the descriptors, so a descriptor number is never reused under a waiting operation. Reads and writes after `close` fail with `NotConnected`, and closing twice does nothing.
- `serve` runs each connection on its own green thread.
- Cancellation is cooperative. A token runs its callbacks and closes its channel once, so code stops blocking work by closing what it waits on: `net.close` wakes a waiting read or write, `process.kill` ends a child, and `select` takes `<-cancel.done(t)`.
- An HTTP/1.1 handler without an `Upgrade` header runs on its own green thread while the connection's green thread reads ahead into the request reader. End of input or a read error cancels the request's token; the response is still written. Requests read ahead are served in order afterwards. An HTTP/2 stream's token is cancelled when the client resets it or the connection ends. Every request's token is cancelled once its handler returns.
- A client request with a token arms its deadline even without a timeout. Cancelling the token fires the deadline, which shuts down the connection the request is using and wakes it from waiting on a pooled connection or an HTTP/2 stream.
- A server speaks HTTP/1.1 and HTTP/2 on the same port. A connection that opens with the HTTP/2 preface is served as HTTP/2; any other is HTTP/1.1 with keep-alive, pipelining, chunked bodies and `Expect: 100-continue`. Over TLS, ALPN offers `h2` and `http/1.1` and the same detection applies. The `h2c` upgrade from HTTP/1.1, deprecated by RFC 9113, is not offered.
- HTTP/2 covers framing, HPACK with its dynamic table and Huffman decoding, stream multiplexing, flow control in both directions, SETTINGS, PING, RST_STREAM, GOAWAY and CONTINUATION, and the request validation of RFC 9113 section 8. A reader green thread feeds frames to the connection, each request's handler runs on its own green thread, and the connection selects over frames and finished responses, so every write happens on one green thread. The encoder uses the static table and literals without Huffman coding. It passes h2spec except the test sending a garbage preface, which the shared port reads as an HTTP/1.1 request. A handler's `Response.trailers` are sent as a final HEADERS frame over HTTP/2; HTTP/1.1 responses carry none.
- A handler that returns status 101 owns the connection: the server writes nothing more and closes it once the handler returns. `http.upgraded()` is that response, returned after a WebSocket session.
- `Request.connection` is the buffered reader of an HTTP/1.1 connection, which `websocket.accept` writes the handshake to and reads frames from. A WebSocket server requires masked client frames, answers pings, reassembles fragments, checks UTF-8 in text messages, limits messages to 16 MB and echoes a close frame.
- `http.client()` returns a `Builder[Client]` value whose `with_*` functions return updated copies, and `build` makes a `*Client` that any number of green threads share. `c.request(method, url)` copies the client's settings into a `Builder[Response]`, so headers, authentication, timeouts, redirects, retries, version and decompression can be overridden per request; the type parameter only selects whether `build` or `send` applies. Over HTTP/1.1 a request takes an idle keep-alive connection from a pool keyed by scheme, host and port, dials a new one while the host is under its limit, or queues for one; a sweeper green thread expires idle connections, and a pooled connection that fails before any response byte is retried once on a fresh one. Over TLS the client offers `h2` and `http/1.1` by ALPN and keeps one HTTP/2 connection per host: a reader green thread decodes frames into per-stream queues, request green threads wait on their stream, flow control credits windows as bodies are consumed, and SETTINGS, PING, RST_STREAM, GOAWAY and the peer's stream limit are honoured. One reaper green thread per client enforces whole-request deadlines by shutting down an expired HTTP/1.1 exchange, while HTTP/2 waits select on the deadline and reset the stream. Redirects switch 301, 302 and 303 to GET without a body, keep the method and body on 307 and 308, resolve relative locations and drop `Authorization` and `Cookie` across origins. Responses are decoded from gzip, deflate (zlib or raw), Brotli and Zstandard as they stream, chunked bodies are read and sent, and an optional RFC 6265 cookie jar is shared by the client. Retries apply to connection failures and, for idempotent methods, to transport errors and 429, 502, 503 and 504, with exponential backoff that respects `Retry-After`. Servers set `TCP_NODELAY` so separate HEADERS and DATA writes are not held back by delayed acknowledgements. `with_prior_knowledge(true)` speaks HTTP/2 to `http://` URLs without negotiation (h2c), keeping one connection per host as over TLS. `Response.trailers` holds the trailer fields of chunked HTTP/1.1 bodies and of HTTP/2 responses.
- Request and response bodies are limited to 64 MB.

### Processes

- `process.start` builds the path, arguments, environment and pipes before `fork`, so the child runs only straight-line syscalls: `setpgid` for a new group, `dup2` onto descriptors 0 to 2, `chdir` and `execve`. Every descriptor the standard library opens is close-on-exec, so the child inherits only its three streams. A child's pipes are streams over guarded descriptors, so closing one twice or using it after it closed fails with `NotConnected` and never reaches a reused descriptor.
- A failed `chdir` or `execve` writes its errno to a close-on-exec pipe, which the parent reads before returning; the read sees end of file once `execve` succeeds, and the parent reaps the child and fails with that errno otherwise.
- The parent's pipe ends are non-blocking and wait through the scheduler, as sockets do; the child's ends stay blocking. `wait` waits for the process's pidfd to become readable, then reaps it with `waitid`, and remembers the result. `kill` signals through the pidfd, or the whole group when the command asked for one.
- `signal.notify` installs a handler with `rt_sigaction` whose code is a short block of machine code in a mapped page: it writes the signal number to a non-blocking pipe and returns through `rt_sigreturn`. The runtime already gives every OS thread an alternate signal stack, so the handler runs on whichever thread receives the signal. A forwarding green thread reads the pipe with `wait_readable` and sends to the `signals` channel, which holds 16 and drops repeats while full. A later `notify` for the same signal replaces the handler; dropping a `Watch` restores the saved dispositions and unmaps the page. `raise` signals the calling thread, so its handler has run when `raise` returns, while `send` is process-directed and unordered against other threads.
- `watch.next` reads every event an inotify read returns and queues them. With `recursive` set, new and moved-in directories are watched before the batch that reports them is returned. `stop` wakes a parked `next` by adding and removing a watch on `/`, which makes the descriptor readable. Identical consecutive events the kernel has not yet delivered are merged by the kernel.

### Resilience

- `retry.delay` is the capped exponential backoff and `jittered` applies the chosen jitter to it. `next` calls the observer, waits through the policy's sleeper or `sleep`, and stops at the attempt limit, the deadline or a cancelled token.
- A `Bucket` keeps its balance as an integer in units of one token over `per` nanoseconds, so refills are exact and a wait is the ceiling of the deficit over the rate. `reserve` consumes and returns the wait; `wait` sleeps that long and refunds if cancelled.
- `Group.do` runs the function outside its lock, removes the key and stores the result before waking the waiters, and matches the key by call identity so a `forget` followed by a new call is not disturbed.

### Text

- `diff.unified` merges hunks whose gap is at most twice the context, omits a count of 1 in a hunk header and writes `\ No newline at end of file`, as GNU `diff -u` does; the edit script is minimal.
- `number.format_bytes` shows one decimal below 10 and rounds half to even on exact integers. `parse_bytes` reads `k` and `kb` as 1000 and `ki` and `kib` as 1024, and fails with `OutOfRange` on overflow or `TooPrecise` beyond 18 fraction digits. `si` rounds on the exact decimal expansion of the `f64`.
- `naming.words` splits at case, acronym and digit boundaries and keeps combining marks attached to their letter, so `HTTPServerID2` is `http server id 2`.

### Time

- Each concept is its own type: `Timestamp` is a point on the UTC timeline, `Date`, `TimeOfDay` and `DateTime` are civil values with no zone, `Zoned` is a timestamp seen through a zone, `Duration` is exact elapsed time and `Span` is a calendar amount. They never convert silently.
- Fields are private and constructors validate, so a `Date` cannot hold February 30. Months are 1 to 12 and `Month` and `Weekday` are enums. Nothing rolls over: `date.of(2026, 2, 30)` fails with `Invalid`, and the clamping forms are named (`clamped`, `plus_months`).
- Calendar arithmetic and elapsed arithmetic are separate. A `Duration` moves a `Timestamp` or `Zoned` by elapsed time. A `Span` moves a `Date`, `DateTime` or `Zoned` by calendar units: a month after January 31 is the last day of February, and a day after a spring-forward midnight is the next civil day although it is 23 hours long. `until` and `since` take a `Diff` and add whole years, then months, to the start without passing the end, then count the rest.
- Nothing reads the host zone except `zone.local`. A civil time in a daylight saving gap or overlap needs a `Disambiguation`: in an overlap `Compatible` and `Earlier` pick the first occurrence and `Later` the second; in a gap `Compatible` and `Later` move forward by the gap and `Earlier` back; `Reject` fails with `Skipped` or `Ambiguous`. `zoned.start_of_day` is the first instant of the civil day, correct where midnight does not exist.
- Zones come from TZif versions 1 to 4, using the 64-bit body from version 2, with the footer's POSIX TZ rule applying after the last transition. Before the first transition the first standard-time type applies. The rules cover `Jn`, zero-based `n` and `Mm.w.d` days and transition times from -167 to 167 hours, so negative daylight saving (Dublin, Casablanca), half-hour shifts (Lord Howe) and skipped days (Apia) work.
- Parsing is strict ISO 8601, RFC 3339 and RFC 9557: `T`, `t` or a space separates date and time, a fraction has 1 to 9 digits, `:60` reads as `:59`, and a civil type rejects an offset. A written offset that disagrees with a bracketed zone is `OffsetConflict` unless an `OffsetPolicy` says which wins. Every parse error is `Syntax(at)` with the byte offset of the first bad character, or a value error naming the field.
- The supported range is 0001-01-01 to 9999-12-31 for a `Timestamp`, years -9999 to 9999 for a `Date`. Leap seconds are not modelled. Plain arithmetic panics on overflow; the `try_` forms return an optional.
- Month and weekday names are English. `pattern.Names` and `humanize.Words` carry other languages.

### Browser automation

- `cdp.launch` finds `chromium`, `chromium-browser`, `google-chrome-stable` or `google-chrome` on `PATH` unless given an executable, creates a profile with `file.temp_dir`, and starts the browser headless in its own process group with `--remote-debugging-port=0`, standard input and output discarded and standard error piped. A green thread reads standard error until the `DevTools listening on` line, which gives the WebSocket address, and keeps draining it so the browser never blocks on a full pipe. `shutdown` closes the WebSocket, kills the process group, reaps the browser and removes the profile.
- Commands carry an id and, for a session, its `sessionId`; a pump green thread routes replies to the waiting call and sends every other message to the events channel. Sends are serialised, a call waits at most 30 seconds, and once the socket closes every waiting call fails with `Closed` and the events channel closes.
- `open_page` creates a browser context that is disposed on detach, denies downloads in it, and turns on auto-attach with `waitForDebuggerOnStart` and flat sessions, so every page, frame and worker target pauses on creation. The first page in the context is the opened page; any other page in it, such as a popup, is closed before it runs. Every other target in the context gets `Fetch.enable` for all URLs, `Network.enable`, auto-attach for its own children and, for pages, `Network.setBypassServiceWorker`, and only then resumes, so no request escapes interception. Each paused request is passed to `Handlers.request` on its own green thread and continued or failed with its verdict; responses and failed loads are reported with their URL.
- Cancellation is the caller's: `let h = b.handle` and `cancel.on_cancel(t, fn() { cdp.shutdown_on(h) })` end everything the browser was doing.

### TLS

- TLS 1.3 (RFC 8446) and TLS 1.2 (RFC 5246), client and server. `Config.min_version` and `Config.max_version`, written with `tls12()` and `tls13()`, bound the versions and default to 1.2 through 1.3. A client offers its range through `supported_versions`, and a server answers with the highest version both support; `Session.handle.version` holds the result. A server that supports 1.3 ends a 1.2 ServerHello random with the RFC 8446 downgrade sentinel, a client that offered 1.3 aborts with `illegal_parameter` when it sees either sentinel, and a server refuses `TLS_FALLBACK_SCSV` below its highest version with `inappropriate_fallback`.
- `Config.cipher_suites` lists suites in preference order, and a server takes the first of its own list that the client offers; `Session.handle.cipher_suite` holds the IANA code and `Session.handle.suite` the AEAD and hash. TLS 1.3 has TLS_AES_128_GCM_SHA256, TLS_CHACHA20_POLY1305_SHA256 and TLS_AES_256_GCM_SHA384. TLS 1.2 has ECDHE with ECDSA or RSA authentication and AES-128-GCM with SHA-256, ChaCha20-Poly1305 with SHA-256 or AES-256-GCM with SHA-384, AEAD only: GCM records carry the 64-bit sequence number as their explicit nonce, and ChaCha20-Poly1305 records use the RFC 7905 nonce.
- TLS 1.3 key exchange offers the hybrid post-quantum group X25519MLKEM768 first, then X25519, P-256 and P-384. A client sends key shares for X25519MLKEM768 and X25519; a server picks the first group of its own order that the client supports, answering with a HelloRetryRequest when the client sent no share for it, so a client that supports X25519MLKEM768 always gets it. TLS 1.2 uses the first of X25519, P-256 and P-384 in the server's order that the client lists, and a server with an ECDSA certificate also requires the client to list the certificate's curve (RFC 8422).
- Servers sign with ECDSA P-256 or P-384, RSA-PSS, RSA PKCS#1 v1.5 in TLS 1.2, or Ed25519, with keys loaded from PEM (`load_certificates`, `load_key`). TLS 1.2 ECDSA signature schemes name only the hash, so either curve signs with SHA-256 or SHA-384. Clients verify the chain against `Config.roots`, or the system roots when that is empty, check the host name against the certificate's subject alternative names, and verify CertificateVerify or ServerKeyExchange with any offered scheme the certificate allows and, in TLS 1.2, the suite's authentication.
- TLS 1.2 requires the extended master secret (RFC 7627) on both sides. A client requires the secure renegotiation indication (RFC 5746), a server echoes it when the client sends it, and both refuse renegotiation with a `no_renegotiation` warning.
- ALPN, SNI, KeyUpdate in TLS 1.3 and `close_notify` are supported. Session tickets are ignored, so there is no resumption or 0-RTT, and servers do not request client certificates; a client answers a CertificateRequest with an empty Certificate.
- Records hold at most 2^14 bytes of plaintext, and a peer's ciphertext may add 256 bytes in TLS 1.3 or 2048 in TLS 1.2. A record or handshake error sends its fatal alert, such as `bad_record_mac`, `record_overflow`, `unexpected_message` or `decode_error`, and leaves the session unusable.
- A session's read and write sides are independent, with a write lock, so it can be read and written by different green threads.

### Derived serialization

`std/serde` defines a `Writer` and a `Reader`, structs of closures through which values are written and read. The compiler derives, for every concrete type, a function that writes a value to a `Writer`, one that reads it from a `Reader`, and a `Schema` describing it:

- Booleans, integers, floats and strings map to their primitives; `[]u8` to binary; slices to sequences; maps to maps with keys written as text; optionals to null or the value; pointers to their target.
- A struct is a map from field names to values, and every field takes part, private ones included. Reading skips unknown keys, uses a field's default when its key is missing, treats a missing optional field as `none`, and fails with `Missing` otherwise.
- An enum variant without fields is its name. One with labelled fields is a single-key map from its name to a map of its fields, and one with positional fields maps its name to a sequence.
- A schema records each type's kind, name, fields with whether they are required, and variants. A struct or enum reached again through a pointer while it is being described becomes a `Ref` to its name.

`std/json`, `std/orm` and `std/web` are built on these. `T.field`, where `T` is a struct type, is a `serde.Field[T, F]` holding the field's name and index, so a function can take a field of a given struct with a given type.

### Data

- `json.Reader` tokenizes UTF-8 bytes like .NET's `Utf8JsonReader`, strictly per RFC 8259, with a depth limit of 64, returning strings without copying unless they contain escapes. Numbers are parsed as integers or correctly rounded floats.
- `json.serialize_with` and `json.deserialize_with` take `Options` for indentation, naming (`Unchanged`, `CamelCase`, `SnakeCase`, `PascalCase`), maximum depth and `omit_none`. Naming applies to field names and to variant names in both directions; a variant name read back is matched in PascalCase. With `omit_none`, a field whose value is `none` is left out instead of written as `null`. Binary data is base64; non-finite floats are written as `null`.
- `std/cbor` decodes RFC 8949 items into `Value`s: every major type, indefinite-length strings, arrays and maps, tags, simple values, and half, single and double floats, which become `f64`. Indefinite-length items arrive as their definite equivalents. Text must be valid UTF-8 and map keys must be distinct. Truncated input, reserved additional information, a stray break, indefinite integers or tags, mixed or nested string chunks, a two-byte simple value below 32 and data after the item fail with `Malformed`; nesting deeper than `Options.max_depth` or a string, array or map longer than `max_length` fails with `TooLarge`, and a declared length that exceeds the remaining input is `Malformed` before anything is allocated. `decode_first` returns the first item and its length so a caller can continue after it. `encode` writes the deterministic encoding of §4.2.1: shortest heads, definite lengths, map entries sorted by the bytes of their encoded keys, and floats in the shortest of half, single or double that keeps the value, with NaN as `f97e00`. `diagnostic` writes the notation of §8. `serialize` and `deserialize` bind types through serde with `[]u8` as byte strings, structs and variants as maps with text keys, and `none` as null; reading looks through tags and accepts integer map keys as their decimal text.
- `std/webauthn` produces options JSON in the shape py_webauthn's `options_to_json` writes, with base64url challenges, user ids and credential ids, `pubKeyCredParams` defaulting to EdDSA, ES256 and RS256, and attestation `none`. `verify_registration` takes the credential JSON a browser's `PublicKeyCredential` gives in base64url, with or without padding: it checks that `id` is the base64url of `rawId` and that `type` is `public-key`, that clientDataJSON has type `webauthn.create`, the expected challenge (compared in constant time) and the origin, then decodes the attestation object, checks the RP ID hash, user presence, user verification when required, and attested credential data, parses the COSE key, accepts only the `none` format with an empty statement, and rejects a backup state without backup eligibility. It returns the credential id, the COSE key bytes exactly as the authenticator wrote them, and the sign count. `verify_authentication` makes the same client data and authenticator data checks for `webauthn.get`, rejects a sign count that is not greater than the stored one unless both are zero, and verifies the signature over authenticatorData followed by SHA-256 of clientDataJSON with the stored key: ES256 as DER ECDSA on P-256, EdDSA as Ed25519 and RS256 as PKCS#1 v1.5 with SHA-256. Each failure is its own `Error` variant.
- `std/orm` builds queries from conditions over field references, such as `orm.filter(orm.from(users), orm.ge(User.age, 18))`, which are type-checked against the field. It renders them for Postgres or SQL Server, maps rows onto structs, stores nested values as JSON columns, creates tables from types with an identity key named `id`, and returns generated keys from inserts.
- `sql.Value` holds dates as days since 1970-01-01, times as microseconds since midnight, timestamps as microseconds since 1970-01-01 UTC, infinite dates and timestamps as the `i64` maximum and minimum, and UUIDs as 16 bytes. A connection serves one green thread at a time; a call made while another is running fails with `Busy`. `orm.transaction` rolls back on any error from its body and fails with that error.
- `std/sql/postgres` authenticates with SCRAM-SHA-256, MD5 or cleartext, uses the simple protocol for statements without parameters and the extended protocol with a 64-entry prepared statement cache otherwise, streams `each` in batches of 512 rows, reads results as text decoded by type OID, and keeps a connection usable after a server error. A timeout sends a CancelRequest and closes the connection. `connect_tls` upgrades with SSLRequest and `std/tls`.
- `std/sql/sqlserver` speaks TDS 7.4 with SQL authentication. Prelogin offers no encryption to `connect` and full encryption to `connect_tls`, which runs the `std/tls` handshake inside PRELOGIN packets and unwraps the packets the server keeps sending around TLS records. LOGIN7 announces UTF-8 support. Statements without parameters go as SQL batches; with parameters they go to `sp_executesql` as `@p1..@pN` declared as `bigint`, `float`, `bit`, `decimal(38, s)`, `nvarchar`, `varbinary`, `uniqueidentifier`, `date`, `time(7)`, `datetime2(7)` or `datetimeoffset(7)`, with NULL sent as `nvarchar(4000)`. Every request carries the transaction descriptor, and `begin`, `commit` and `rollback` use transaction manager requests. Results decode every fixed, nullable, decimal, money, date and time, GUID, character, binary, long-value, text, image, XML, `sql_variant` and user-defined type; GUIDs become RFC 4122 byte order and single-byte text decodes through its collation's UTF-8 flag or as Windows-1252. Row counts sum the DONE and DONEINPROC counts. `each` streams rows as they arrive, and stopping early sends an ATTENTION and reads until the server acknowledges it. A server error below severity 20 leaves the connection usable. A per-connection watchdog sends an ATTENTION and closes the stream when one read waits longer than the timeout, and the call fails with `Timeout`.
- `std/toml` parses into ordered tables and applies every rule of TOML 1.0.0, including the restrictions on redefining tables through headers, dotted keys, inline tables and arrays of tables; it agrees with Python's `tomllib` on the toml-test suite. Errors carry a line and column. `encode` writes plain values first and then `[table]` and `[[array]]` sections. `serialize` and `deserialize` bind types through serde, reading date-times as RFC 3339 text.
- `std/yaml` parses YAML 1.2.2 into ordered node trees and agrees with the yaml-test-suite on every case's event stream and JSON equivalent. A scanner modelled on libyaml tokenizes the whole stream, tracking indentation, implicit keys, JSON-like adjacent values and the YAML 1.2 tab rules, with 32-byte vector scans over plain scalars and `simd.find_byte_str` for comment and line ends; token text is a view into the source. A recursive-descent parser turns tokens into `Event`s with resolved tags, and a composer builds `Node`s carrying line, column, tag, anchor and style. Plain scalars resolve with the core schema; `!!str`, `!!int`, `!!float`, `!!bool`, `!!null` and `!!binary` are checked against their content; other tags stay on the node. Aliases share their anchored node. `Options.max_nodes` bounds the tree with every alias expanded, failing with `TooManyNodes`, and `max_depth` bounds nesting. Duplicate scalar keys are an error. `encode` writes block style, quotes a scalar only when it would read back differently, including YAML 1.1 booleans, numbers and dates, writes multi-line text as literal blocks, and anchors any node reached more than once. `serialize` and `deserialize` bind types through serde, with `[]u8` as `!!binary`.
- `std/otel` keeps the provider in the green thread's context slot as an immutable `Context` of the active span, baggage, a suppression flag and the SDK. `spawn` copies the slot, so a child green thread's spans parent to the spawning span, and `install` or `setup_from_env`, which return a droppable `Sdk` that shuts down when released, put the `SdkHandle` into the calling green thread's slot, reaching the green threads it spawns afterwards; programs install at the top of `main`. Every API call checks the slot first and returns a no-op value without allocating when no SDK is present. Timestamps and durations come from `time.now_nanos` and `time.monotonic`. Span and trace ids come from a `std/rand` generator behind a lock. Samplers are always on, always off, trace-id ratio on the low 63 bits, and parent-based. Batch processors for spans and logs queue to a bounded channel with a non-blocking send that counts drops, and a worker green thread exports when a batch fills, when the delay passes, and on flush or shutdown. Metric instruments aggregate per attribute set, keyed by a canonical encoding of the sorted attributes, in one stream per reader, capped at 2000 series plus an `otel.metric.overflow` series, and readers collect cumulatively or as deltas, periodically or on demand. Exporters encode OTLP once through a writer that emits protobuf through `std/protobuf` or the OTLP JSON mapping, and send with the context suppressed. OTLP/HTTP posts, optionally gzipped, retrying 429, 502, 503 and 504 with jittered exponential backoff and `Retry-After`. OTLP/gRPC makes unary `Export` calls on `opentelemetry.proto.collector.{trace,metrics,logs}.v1` over the pooled client's HTTP/2 connection, with h2c prior knowledge for `http://`: the message carries the 5-byte gRPC prefix, optionally gzipped under `grpc-encoding`, with `te: trailers` and `grpc-timeout`, and the result is `grpc-status` from the trailers or, in a trailers-only response, the headers; CANCELLED, DEADLINE_EXCEEDED, ABORTED, OUT_OF_RANGE, UNAVAILABLE and DATA_LOSS are retried, and RESOURCE_EXHAUSTED only when `grpc-status-details-bin` carries a RetryInfo, whose delay replaces the backoff. `setup_from_env` reads the standard `OTEL_*` variables; `OTEL_EXPORTER_OTLP_PROTOCOL=grpc` selects gRPC, defaulting to `http://localhost:4317`.
- `std/http` sits below `std/otel` in the import graph, so the hook runs through the context slot: `otel.Context` declares an `http.Ambient` as its first field, and `std/http` reads the slot as `*http.Ambient`, which relies on struct fields being laid out in declaration order. `install` sets the ambient `Observer` and `suppressed` clears it; `std/http` does nothing when it is absent. `http.serve` and `serve_tls` open a server span per request over HTTP/1.1 and HTTP/2, with the parent extracted from `traceparent` and `client.address` from `Request.remote`, and record `http.server.request.duration`; `std/web` adopts that span, adding `http.route` and renaming it to the method and route, and opens its own only when its handler runs outside `http.serve`. The client opens a CLIENT span named after the method for each attempt sent, redirects and retries included, each under the caller's span with `http.request.resend_count` from the second, injects `traceparent`, `tracestate` and `baggage` into that attempt, sets `url.full` without userinfo, ends the span when the body is delivered or the attempt fails, with `error.type` the status from 400 or the `http.Error` variant, and records `http.client.request.duration`. `std/sql/postgres` and `std/sql/sqlserver` wrap their connections with `instrument_sql`, and `std/orm` opens client spans with the operation and table, a nested database span merging into the outer one so an ORM query produces one span.
- `std/protobuf` writes and reads the protobuf wire format without code generation. An `Encoder` appends into a buffer that doubles as needed; signed integers sign-extend to ten bytes and `sint64` zigzag-encodes. `begin_message` returns a mark and `end_message` measures the body, shifts it right by the size of its length varint and writes the length, so nested messages need neither a second buffer nor padded lengths. Output matches `protoc --encode` byte for byte. A `Decoder` walks fields in order as number, wire type, integer value and a view of the payload, rejecting truncated or overlong varints, lengths past the end, field 0 and wire types 6 and 7 with `Malformed` and groups with `Unsupported`. The `as_*` readers check the wire type and the `unpack_*` readers accept packed and single encodings.
- `std/xml` reads XML 1.0 (fifth edition) with Namespaces from UTF-8, UTF-16 with a byte order mark, ISO-8859-1 or US-ASCII, converting to UTF-8 and normalising line endings first. `xml.Reader` is a pull reader: `next` returns one token and `position` its line and column. Names arrive with namespaces resolved and are interned in a per-reader table; text, CDATA, comments and attribute values are views into the input unless a reference or normalisation forces a copy, and 32-byte vector scans find the bytes that end them. The reader enforces well-formedness and the namespace constraints and agrees with libxml2 on what it accepts. The internal subset is parsed: general entities expand, including markup, with recursion detection, a nesting limit of 40 and a total of 1 MiB plus eight times the input, and attribute-list declarations supply defaults and normalise tokenised types. External entities are never fetched and parameter entities are not expanded. `xml.parse` builds an `Element` tree of owned strings with `attribute`, `child`, `children`, `text` and `find`. `xml.Writer` escapes text and attributes, splits CDATA around `]]>`, writes namespace declarations and indents element-only content. `serialize_value` and `deserialize` bind types through serde, mapping fields to attributes or child elements by local name, repeated elements to slices, and `xsi:nil` to `none`.
- Pipes block through channels. A writer flushing more than the pause threshold waits until the reader brings the unconsumed bytes below the resume threshold, or until the reader itself waits for more data, so a message larger than the thresholds cannot deadlock the pipe.
- `pool.open` stores `ConnectionHandle` views, so one connection serves a lease at a time and the pool, not the owner, closes it. Idle and lifetime limits are checked on acquire, release and `prune` against `clock.monotonic`, and a release hands the connection to the oldest waiter. A lease marks its connection broken on `Connect`, `Authentication`, `Closed`, `Timeout` or `Protocol` errors, and the pool closes it; a transaction left open at release is rolled back.
- `migrate.up` refuses any applied migration whose checksum differs (`Drift`), a pending version below the highest applied one (`OutOfOrder`) and duplicate or non-ascending versions. The lock is `BEGIN IMMEDIATE` on SQLite, `pg_advisory_lock` on PostgreSQL and `sp_getapplock` on SQL Server, and each migration re-checks inside its transaction that it is not yet recorded. In a text migration a statement ends at a line ending in `;`, and `-- +begin` to `-- +end` keeps a block with several `;` as one statement.

#### Unicode

- `std/unicode` reads tables generated by `cmd/tablegen unicode` from UnicodeData.txt, CaseFolding.txt and DerivedNormalizationProps.txt of Unicode 16.0.0, the version Python's `unicodedata` reports, and agrees with it on every code point. The tables are `embed`ded binary files: a two-level trie of 64-entry blocks, covering code points below U+2FA40 and giving 0 above, maps each code point to a 16-bit record id, and each record holds the canonical combining class and 16-bit offsets and lengths into a pool of `u32` code points for the full canonical decomposition, the full compatibility decomposition and the C and F case folding. Hangul syllables decompose and compose algorithmically. Decomposition is followed by a stable canonical reordering by combining class; composition binary-searches a sorted table of primary composites, leaving out Full_Composition_Exclusion. ASCII input returns at once, or lowercased by `casefold` and `search_key`.
- `search_key(s)` equals Python's `''.join(c for c in unicodedata.normalize('NFKD', s.casefold()) if not unicodedata.combining(c))`. It skips reordering, since every reordered character is dropped.

#### HTML

- `std/html` tokenizes UTF-8 HTML following the WHATWG tokenization states; it builds no tree and never fails. Tag and attribute names are lowercased in ASCII, attribute values may be double-quoted, single-quoted or unquoted, a repeated attribute keeps its first value, and a tag cut off by the end of input is dropped. A start tag for `script`, `style`, `xmp`, `iframe`, `noembed` or `noframes` switches to raw text, and one for `title` or `textarea` to escapable raw text, until the matching end tag in any case; `plaintext` takes the rest of the input. The script-data double-escape states are not implemented, and `noscript` is tokenized as when scripting is disabled.
- Comments end at `-->` or `--!>`, with `<!-->` and `<!--->` empty. `<?...>`, `<!...>` other than comments and doctypes, `<![CDATA[...]]>` and `</` followed by a non-letter become comments, and `</>` is skipped. `Doctype` carries the lowercased name and the optional public and system identifiers.
- Character references are decoded in text, escapable raw text and attribute values: named references by longest match against the full WHATWG table of 2,231 names generated by `cmd/tablegen entities` and `embed`ded, including the names valid without a semicolon, and decimal and hexadecimal references with or without a semicolon. Zero, surrogates and values above U+10FFFF become U+FFFD, and 0x80 to 0x9F map through Windows-1252. Inside an attribute, a match without a semicolon that is followed by `=` or a letter or digit stays literal. Text without references is a view into the input. The tokens match Python's `html.parser` except where it departs from WHATWG: duplicate attributes, processing instructions and CDATA as comments, raw text after `<script/>`, and control-character references.

### Regular expressions

- `std/regex` compiles a pattern once into a program for a Pike VM, a Thompson NFA simulation that carries capture slots per thread, so matching takes time linear in the input times the program size and never backtracks. A `Regex` is immutable; each call allocates its own thread queues, so one compiled value is shared freely between green threads.
- Syntax: literals; escapes `\n \t \r \f \v \a`, `\xhh`, `\uhhhh`, `\Uhhhhhhhh` and escaped punctuation; `.`; classes with ranges, negation and `\d \w \s \D \W \S` inside; `\d \w \s \D \W \S \b \B \A \Z \z`; `^` and `$`; capturing groups, `(?:...)`, inline flags `(?i)`, `(?m)`, `(?s)`, `(?-i)` and scoped `(?i:...)`; alternation; `*`, `+`, `?`, `{m}`, `{m,}`, `{,n}` and `{m,n}` with lazy forms. A `{` that does not form a valid count is a literal. Inline flags apply to the rest of the enclosing group.
- Input is UTF-8 matched by code point; offsets are bytes. `\d`, `\w`, `\s`, `\b` and case folding are ASCII. `.` excludes only `\n` unless `dot_all` is set. Without `multiline`, `$` matches at the end or before a final `\n`; with it, `^` and `$` match at every line boundary.
- Matching is leftmost-first, as in Python and RE2, and results agree with Python's `re` under `re.ASCII`. `find_all`, `replace_all` and `split` follow Python 3.7: an empty match is allowed right after a non-empty one, and after an empty match the next one may start at the same offset only if it is not empty. `split` does not include group text. `split_n` splits at most `max` times; a negative `max` splits everywhere.
- Repeat counts are limited to 10000 and programs to 100,000 instructions. Invalid patterns fail with `Invalid(at, message)`, at the byte offset Python reports. Backreferences, lookaround, named groups and possessive quantifiers are rejected.
- `regex.generated("pattern")` is a `Regex` built at compile time, like .NET's `[GeneratedRegex]`. The pattern must be a string literal; flags are written inline. An invalid pattern is a compile error at the literal, `invalid regex at byte N: message`, with the offset and message `compile` would fail with. The result is used through the same functions as a compiled `Regex` and matches identically.
- `internal/regex` holds a Go copy of the parser and assembler, held to the LowLang one by running the `std_regex_errors` fixture's patterns through it. The checker turns each distinct literal into LowLang functions named `__regex_` and the FNV-1a hash of the pattern, parsed with the prelude's privileges and added to `std/regex`, so they reach its private types. A `Regex` carries an `Engine` of thread-start, step and skip functions with the queue size; `compile` fills it with the Pike VM, and the generated engine keeps the VM's queues, priorities and capture slots but replaces interpretation with code per instruction: one function per instruction that starts a thread, follows a consumed character or joins two epsilon paths, each guarded by the queue's sparse set, with the remaining jumps, splits, saves and assertions inlined into it; characters and classes of up to four ranges compare inline in the step, which dispatches on the program counter by binary search, and larger classes get a binary-search function. When no empty match is possible from the start, a skip function moves past ASCII bytes that cannot begin a match before a thread starts. Programs over 2000 instructions are compiled at run time by `compile` instead, so call depth stays bounded.

### Compression

- DEFLATE, zlib and gzip decode stored, fixed and dynamic blocks with two-level lookup tables, check Adler-32, CRC-32, ISIZE and the gzip header CRC, and read concatenated gzip members. The encoder uses greedy matching at levels 1 to 3 and lazy matching at 4 to 9 over a hash chain on a 32 KB window, choosing dynamic, fixed or stored encoding per block; `flush` emits a sync flush so streamed output is decodable as it arrives.
- xz decodes multiple blocks and streams, the none, CRC32, CRC64 and SHA-256 checks, LZMA2 chunks with their resets, and the delta filter; other filters fail with `Unsupported`. The encoder writes LZMA2 with a hash-chain match finder and lazy parsing, a dictionary from 256 KiB to 64 MiB by level, and raw chunks where compression does not pay.
- `std/compress/zstd` and `std/compress/lz4` each offer `compress(data, level)`, `decompress(data, limit)`, a streaming `decode(reader, sink, limit)` that passes output to the sink one block at a time, and a streaming `Encoder` with `encode`, `flush` and `finish`. The zstd decoder handles every block and literal type, 1- and 4-stream Huffman literals, all four FSE table modes, repeat offsets, skippable and concatenated frames, and the content checksum; its history grows with real output up to the smaller of the window and the remaining limit, so large windows never allocate what the header claims up front. The zstd encoder finds matches with two hash chains keyed on 8-byte and 4- or 5-byte prefixes, with depth and lazy evaluation set by the level, Huffman-codes literals, picks predefined, RLE or custom FSE tables per block by estimated cost, and falls back to raw or RLE blocks. The LZ4 decoder accepts linked and independent blocks, block and content checksums, content sizes and skippable frames; the encoder writes linked 64 KB blocks with a content checksum. Dictionaries and LZ4 legacy frames fail with `Unsupported`.
- `std/compress/brotli` has the same API as zstd and LZ4. The decoder handles compressed, uncompressed and metadata meta-blocks, block switching for literals, commands and distances, all four literal context modes, context maps with run-length coding and inverse move-to-front, distance postfix and direct parameters, the static dictionary with all 121 transforms, and windows of 10 to 24 bits. Output collects in a ring buffer that starts at 64 KB and doubles up to the window, and reaches the sink at the end of each meta-block, when the ring wraps, and before each blocking read. The 122,784-byte dictionary is `embed`ded from `dictionary.bin`. The encoder writes uncompressed meta-blocks at level 0; levels 1 to 11 run hash chains of depth 2^(level-1) with lazy matching from level 4 and checks against the four recent distances and their neighbours, writing 256 KB meta-blocks with Huffman-coded commands and distances, and from level 5 model literals in UTF-8 context mode with up to 3·level-11 clustered trees. A block that would grow is stored. `flush` ends the meta-block and pads with an empty metadata block so everything sent decodes.
- `std/bits` holds a growable `Set` over `[]u64` words; counts use a SWAR popcount and scans (`next_set`, `next_clear`, `min`, `max`, `rank`, `select_at`, `members`) step a word at a time. `std/bits/roaring` holds `Roaring`, a set of `u32` split into 65536-value chunks keyed by the high 16 bits, each held as a sorted `[]u16` array of up to 4096 values, a 1024-word bitmap, or a list of runs. Arrays and bitmaps convert at 4096 values; ranges create runs, and unions and intersections of runs stay runs. Every container tracks its cardinality, so `cardinality` is constant time. Set operations come as new-set and in-place `_with` forms, working word by word on bitmaps and by galloping merge on arrays. `run_optimize` picks the smallest form per container by the Go library's rule, and `serialize` and `deserialize` implement the portable Roaring format byte for byte as Go `roaring/v2` writes it, rejecting any inconsistent length, ordering or cardinality with `Malformed`.
- `std/image` holds an `Image` of width, height, `Format` (`Gray8`, `GrayAlpha8`, `Rgb8`, `Rgba8`, `Gray16`, `Rgba16`, 16-bit samples big-endian), stride and pixel bytes. Pixels read and write as 8-bit `Rgba`, with operations converting through 16-bit intermediates; `get` outside the image is transparent black and `set` there does nothing. It converts formats, crops, flips, rotates, applies EXIF orientation, fills rectangles, composites straight alpha, premultiplies and unpremultiplies, and resizes separably on premultiplied values with nearest, bilinear, Catmull-Rom bicubic or Lanczos-3 filters. `decode` picks a codec by signature and rejects images over 2^26 pixels; `decode_limited` takes another limit. PNG decodes every colour type, bit depth and Adam7, checks every CRC, and encodes with per-row adaptive filtering. JPEG decodes baseline and progressive scans with restart markers, any sampling, grayscale and Adobe CMYK/YCCK with libjpeg's integer IDCT and fancy upsampling, so output matches libjpeg byte for byte, and encodes baseline 4:2:0 with the standard tables scaled by quality. GIF decodes 87a and 89a and composites every frame by disposal method in `decode_gif_frames`, and encodes one frame with a given or median-cut palette. BMP covers 1 to 32 bits, RLE, bitfields and every header version, and encodes 24 or 32 bits. QOI decodes and encodes. WebP decodes lossless VP8L with every transform, the colour cache and meta prefix codes; lossy VP8, alpha chunks and animation fail with `Unsupported`.
- Every codec has buffer functions, a streaming decoder that pulls from an `io.Reader`, pushes output to a sink and fails with `TooLarge` past a caller's limit, and a streaming encoder.

### Web

- `web.app()` holds routes and middleware. `get`, `post`, `put`, `patch`, `del` and `route` add handlers for paths with `:name` parameters and a trailing `*`; `group` makes a sub-application with a prefix and more middleware, and `mount` adds its routes back. An unknown path is a 404 and a known path with another method a 405 with `Allow`.
- `endpoint[Req, Res]` and `query_endpoint[Res]` read a JSON body into `Req`, call a handler returning `Res ! web.Problem`, and write the result as JSON or the problem as `application/problem+json`. Their schemas describe the operation in the OpenAPI document that `openapi` serves, along with path parameters and each route's security schemes.
- Middleware is a struct of a `run` closure and an optional security scheme. `api_key`, `basic` and `bearer` authenticate requests and set `Context.user`; `sessions` keeps a map in a cookie sealed with AES-256-GCM, with the cookie name as associated data and an expiry inside; `csrf` requires the session's token in `X-CSRF-Token` on unsafe methods; `require_session`, `cors`, `logger` and `compress` round them out; `compress` picks Zstandard, Brotli or gzip, in that order, from what `Accept-Encoding` allows.
- `jwt` verifies bearer tokens against a key set with `std/crypto/jose`. `oidc` does the same for an OpenID Connect issuer: it reads the discovery document and the JWKS, refreshes them hourly, and refreshes early, at most once a minute, when a token names a key it does not know. `oidc_login` adds a sign-in route that redirects to the issuer with `state`, `nonce` and a PKCE S256 challenge kept in the session, and a callback route that checks the state, exchanges the code, verifies the ID token and its nonce, and hands the claims to the application.
- A handler streams a body by returning a `Response` with `stream` set to a `chan[[]u8]` and closing it at the end. The server writes each chunk as it arrives, cancels `req.token` when the stream ends, the client disconnects or a write fails, and drops the stream for `HEAD` and bodiless statuses. Response compression skips streamed responses.
- `sse.stream` sets `text/event-stream`, `no-cache` and `x-accel-buffering: no`, flushes every event, sends a `: keepalive` comment every 15 seconds unless `keepalive` is 0, and stops when `req.token` is cancelled; `Emitter.send` returns false after the client has gone. `sse.format` removes CR and LF from `id` and `event` and NUL from `id`.
- `web.static` decodes each path segment and answers 400 for `..`, an embedded separator, NUL or a bad escape, and 404 for dotfiles, non-regular files and any file whose real path leaves the root. A directory without a trailing slash is redirected, a directory serves `index.html` or an optional listing, and `precompressed` picks `.br` then `.gz` by `Accept-Encoding`. A multi-range or malformed `Range` gives the whole file. Files stream in 64 KiB chunks.

## Compiler

Written in Go. Pipeline:

| Stage | Package | Output |
|---|---|---|
| Lex | `internal/lexer` | tokens |
| Parse | `internal/parser` | `internal/ast` tree |
| Check | `internal/check` | types attached to the tree |
| Build IR | `internal/ir` | SSA form, one value per machine word |
| Codegen | `internal/amd64` | x86-64 GNU assembly |
| Assemble and link | `as`, `ld` | static or dynamic ELF binary |

The `low` command lives in `cmd/low`; the pipeline it shares with hot reload lives in `internal/compiler`. `low ir` prints the SSA form and `low asm` the generated assembly.

`internal/compiler` finds the root, loads the entry module, then loads each imported directory once, breadth first, and rejects import cycles. Every file records its module path, and every source position its file, so errors name the file they come from. The checker keeps a scope of types and functions per module; the prelude belongs to the entry module, and its `__` names cannot clash with anything else. A function in module `ui/text` becomes the symbol `fn_ui.text.shout`, and types display with their module path, such as `geometry.Circle`.

The runtime, `internal/amd64/sched.asm` and `internal/amd64/runtime.asm`, is emitted with every program and talks to the kernel through raw syscalls. It ends the process with `exit_group`, so every thread stops.

### Scheduler

- Green threads run on OS threads, one per CPU in the process's affinity mask. Threads are started as work appears, with `clone` in a static program and `pthread_create` in a dynamic one. A thread whose green thread is in a C call does not count against the limit, so another starts while C blocks.
- Each OS thread keeps a 256-entry run queue; a full queue spills to a global queue. An idle thread takes from its own queue, the global queue every 32 schedules and whenever its own is empty, then expired timers and ready descriptors, then steals from other threads. With nothing to do it sleeps on a futex, or becomes the single poller that blocks in `epoll_wait` until the next timer.
- A green thread reserves 1 MB of stack with a guard page below it; pages are committed as they are touched, and finished green threads are reused. Its context is its saved registers on its own stack. A fault in the guard page panics with `stack overflow`; any other fault panics with `invalid memory access`.
- Per-thread state lives in a 24-byte `.tbss` block that sits directly below the thread pointer: the thread record at `fs:-24`, the running green thread at `fs:-16` and the preemption flag at `fs:-8`. A static program sets `fs` with `arch_prctl` and `CLONE_SETTLS`; a dynamic program uses the block glibc allocates for every thread. Generated code uses the fixed offsets, so hot reload patches need no TLS relocations.
- Every loop back-edge tests the preemption flag. A monitor thread sets it every 10 ms on a thread whose green thread has run since the last check, and the collector sets it everywhere; a set flag calls `rt_poll`, which saves every register and yields or stops for the collector. Code that holds a channel lock does not yield. Recursion without loops is not preempted.
- `park` and `ready` hand a green thread off with a per-thread lock and a wake token, so a wakeup that arrives before the green thread has finished parking is not lost.
- `ready` puts the green thread in its OS thread's `runnext` slot, so it runs as soon as the current one parks, without waking anyone. A green thread pushed out of that slot goes to the stealable run queue and calls `wakep`. `wakep` does nothing while an OS thread is already spinning; otherwise it wakes one idle thread, which counts as spinning until it finds work or gives up. A spinner that finds work while more is queued wakes a successor. The monitor thread moves a `runnext` stranded behind a long C call to the global queue.
- Timers are a locked array of deadlines; `sleep` and `select`'s `after` arms add to it, and the poller wakes through an eventfd when an earlier timer arrives. Descriptors are registered with `EPOLLONESHOT`.
- Channels and `select` are generic prelude code over `__Chan[T]`: a lock word, a ring buffer and ring queues of waiters that keep their capacity. A waiter is claimed with compare-and-swap before it is woken, so a `select` waiting on several channels and timers fires exactly once. An operation that completes at once allocates nothing; the waiter and value box are allocated only to block. Green threads to wake are collected under the lock and readied after it is released. `select` locks its channels in address order, tries each arm, and otherwise queues a waiter on every channel, parks, and removes the leftovers when woken.
- A channel lock word is 0 when free, 1 when held and 2 when held with sleepers. `rt_lock` spins briefly, then marks the word 2 and sleeps on a futex; while it sleeps its OS thread counts as stopped, so a collection can proceed. `rt_unlock` wakes one sleeper when the word was 2.
- C calls run on the OS thread's system stack (8 MB in a static program). Around a C call the generated code pushes the callee-saved registers on the green thread's stack, records where the green thread stopped, marks the thread as in C and switches stacks; on return it marks the thread running again, waiting if a collection is under way.
- A C callback runs on the green thread's stack. Its thunk marks the thread running, switches back to the green thread's stack below the C call's saved registers, and locks the green thread to the OS thread until it returns, because the C frames live on that OS thread's system stack: if the callback waits, the OS thread waits with it. Callbacks must arrive on the thread that made the C call.

### Linking

- A static program is linked with `ld -static` and starts at the generated `_start`, which initialises the heap and the scheduler, starts `main` on the first green thread and enters the scheduler on the process stack.
- A dynamic program is linked with `ld -dynamic-linker /lib64/ld-linux-x86-64.so.2` against glibc's `crt1.o`, `crti.o` and `crtn.o`, then `-l<name>` for each library and `-lc`. The start files are looked for in `/usr/lib`, `/usr/lib64`, `/usr/lib/x86_64-linux-gnu` and `/lib/x86_64-linux-gnu`. glibc's start-up code calls the generated C `main`, which does the same; when LowLang's `main` returns the program calls `exit(0)`, letting libc run its exit handlers. An error from `main` still prints and exits with status 1.

`internal/compiler/prelude.low` is LowLang source compiled ahead of every program: UTF-8 decoding, the string builder behind interpolation and concatenation, the generic map table, integer formatting, and float formatting with Burger and Dybvig's free-format algorithm over big integers of 32-bit limbs. Its names start with `__`, which user code cannot use, and four intrinsics are available only there: `__bits` gives a float's bit pattern, `__str_view` turns a `[]u8` into a `str` without copying, `__hash` hashes any `Hash` value, and `__zero` is the all-zero value of its expected type. Functions unreachable from `main` are dropped, so a program that formats no floats carries none of this code.

The standard library lives in `internal/compiler/std` and is embedded in `low` with `go:embed`. An import whose path is `std` or starts with `std/` is read from there, never from the project root. Its files are lexed like the prelude, so they may use `__` names and intrinsics, and they are left out of the sources `low dev` watches. `embed` in a standard library file reads from the embedded tree and writes the bytes once to `lowlang/embed/` under the user cache directory, named by their SHA-256, for the assembler's `.incbin`. These intrinsics serve it: `__argc()`, `__argv(i)` and `__envv(i)` read the argument count, argument `i` and environment entry `i` (`null` past the end) as NUL-terminated `ptr[u8]`s, which the start code saves from the initial stack in a static program and from `main`'s arguments in a dynamic one; `__syscall(n, args...)` makes Linux syscall `n` with up to six integer, `bool` or `ptr` arguments and returns `rax` as `i64`, a negative errno on failure; `__slice_ptr(x)` gives the data address of a slice or `str` as a `ptr[T]`; `__mulhi(a, b)` is the high 64 bits of the 128-bit product of two `u64`, an IR instruction that becomes one `mul`; `__make(n)` is a zeroed slice of `n` elements of the expected slice type, allocated through `rt_grow`; `__memmove(dst, src)` copies `min(dst.len, src.len)` elements with the runtime's overlap-safe `rt_memmove` and returns the count, taking a `str` source for a `[]u8` destination; `__clock_nanos(id)` returns `clock_gettime(id)` in nanoseconds, or 0 on failure, through `rt_clock_nanos`, which calls `__vdso_clock_gettime` when start-up found it from `AT_SYSINFO_EHDR` and the vDSO's dynamic symbol table (DT_HASH or DT_GNU_HASH) and makes the syscall otherwise; `__context()` reads the current green thread's context slot as the expected pointer type, `__set_context(p)` replaces it and `__has_context()` tells whether it is set; `spawn` copies the parent's slot into the child, and the collector marks every live green thread's slot; `__serialize`, `__deserialize` and `__schema` are the derivations below, allowed only in `std/serde`. `__syscall` lowers to a runtime-call instruction that the code generator emits inline: it moves the arguments into `rax`, `rdi`, `rsi`, `rdx`, `r10`, `r8` and `r9` with the parallel move and executes `syscall`. Being a call, it keeps no value in a caller-saved register across it, which covers the `rcx` and `r11` the kernel clobbers.

The garbage collector is a non-moving mark-sweep collector in the runtime:

- The heap is a 16 GB reservation made with `mmap` and `MAP_NORESERVE`; pages are committed as they are touched.
- Each object has a one-word header holding its size in words, a mark bit, a free bit and a noscan bit. A bitmap records the first word of every object and free chunk, which resolves pointers into the middle of an object, such as a `mut` parameter aimed at a heap field. Two summary bitmaps above it hold one bit per non-empty bitmap word and one per non-empty summary word, so finding the start behind an interior pointer skips empty spans 32 KB and 2 MB at a time instead of reading every bitmap word between the pointer and its object's header.
- Each OS thread allocates objects of up to 255 words from its own buffer of up to 4096 words without taking the heap lock: it bumps a pointer, writes the header and sets the object's start bit. Only the start-bitmap words at the buffer's unaligned edges are set atomically; the summary bits are tested first and set atomically only when clear. A thread whose buffer is full takes the heap lock and refills it from the first free chunk large enough, then the exact-size free lists from the largest down, then the end of the heap, zeroing the buffer in one pass. A collection turns every thread's unused buffer tail into a free chunk before marking, and an allocation that would leave a single word in a buffer takes that word too, so the sweep finds a header at every object boundary.
- Larger objects take the heap lock and allocate from exact-size free lists for objects under 64 words, then first-fit from a list of larger chunks, splitting them, then from the end of the heap.
- Collection runs when the words allocated since the last one exceed the words that survived it, with a 4 MB floor.
- An allocation whose type holds no pointers is noscan: `new` of a pointer-free value, and the backing arrays of slices whose elements hold none, such as `[]u8`, `[]i64` and `[]f32`. A noscan object is marked but its contents are never read. Other objects are scanned word by word.
- Allocation and pinning take a heap lock. A collection stops the world: it sets a stop flag and every thread's preemption flag, then waits until each other thread is stopped at a safepoint, idle in the scheduler or in a C call, and holds those threads there until it finishes. Threads waiting for a lock stop for the collector while they wait.
- Roots come from stack maps. At every call the code generator records the frame words that may hold a heap pointer at that point: the saved callee-saved registers, spill slots of values that may be pointers and are live across the call, and every frame slot. The maps form a table sorted by return address.
- Every green thread records where it stopped: the return address, stack pointer and frame pointer of the LowLang code that entered the runtime, and the lowest stack word holding registers it saved. The collector scans each live green thread's saved registers conservatively, then follows its `rbp` chain, looking up each frame's return address and scanning only its mapped words, and scans the green thread's first frame conservatively. Frames without a map, such as code patched in by hot reload or a loop interrupted by a preemption check, are scanned whole. A green thread that has not started is scanned whole.
- Values in callee-saved registers are found in the save slots of whichever later frame saved them, or among the registers saved when the green thread stopped.
- A callback thunk crosses C frames by segment. Its frame record holds the LowLang caller's frame pointer and return address, so the `rbp` chain leads from the thunk straight back to the caller, and its frame keeps the bounds of the C frames on the system stack and of the registers the C call saved. The thunk's call site has a map of count -1, which tells the collector to scan both regions conservatively before following the chain.
- Every candidate word, mapped or not, is checked against the heap before it is followed, so a map that names a word holding an integer is harmless.
- A `ptr` counts as a possible heap pointer, so a `cstr` copy or a pinned object stays alive while LowLang holds its address. `pin` records the address in a table of 65536 roots that every collection scans; `unpin` removes it.
- Sweeping coalesces adjacent dead objects into free chunks and returns a trailing free run to the end of the heap.

The derivations are generated as source. When the checker meets `__serialize(v, w)`, `__deserialize[T](r)` or `__schema[T]()` with a concrete type, it writes a function for that type in LowLang text, binding the type and each type it mentions to names like `__X0`, parses it with the prelude's privileges, marks it trusted so it may read and build private fields, and checks it like a generic instance. Calls inside it to the same intrinsics derive the types they reach, and each derived function is cached by type. Bound names resolve in type positions, struct literals and variant selectors. A field reference `T.field` is checked as `serde.Field[T, F]` and lowered to the field's name and index as constants.

### IR

- Generic templates never reach the IR. The checker keeps an untouched copy of each generic function's syntax tree, clones it for each distinct set of type arguments and checks the clone with those types bound, producing an ordinary function named like `max[i64]`; generic structs and enums become ordinary types named like `Stack[str]`. Instances are made on demand, checked from a work list, and limited to 1000 per template. Symbols escape every character outside `A-Za-z0-9_.` as `.` and its hex code, so `max[i64]` is `fn_max.5bi64.5d`.
- SSA is built directly from the checked tree with Braun et al.'s algorithm: variables are read and written per block, phis are created lazily and sealed once a block's predecessors are known, and trivial phis are removed.
- Every value is one 64-bit word. A struct, enum or result of n words is n SSA values, so aggregates live in registers with no frame temporaries.
- Narrow integers are kept sign-extended (`i32`) or zero-extended (`u32`, `u16`, `u8`) in their word; floats are kept as their bit pattern, `f32` zero-extended. Arithmetic, conversion and comparison values carry their type.
- A variable passed as `mut` lives in a frame slot and is accessed with loads and stores; every other variable is pure SSA.
- Integer division carries its zero and overflow checks inside the instruction.
- A call to a C function becomes a `ccall` carrying its System V AMD64 plan. Each argument word is classified: integers, `bool` and pointers take `rdi`, `rsi`, `rdx`, `rcx`, `r8` and `r9`; floats take `xmm0` to `xmm7`; an `extern struct` of at most 16 bytes is split into eightbytes that go to vector registers when every field in them is a float and to integer registers otherwise; a larger struct, or one that does not fit in the registers still free, goes on the stack. Variadic `f32` arguments are widened to `f64`, and `al` holds the number of vector registers used. A struct result of at most 16 bytes comes back in `rax`, `rdx`, `xmm0` and `xmm1` by class; a larger one is written to a buffer whose address goes in `rdi`. Narrow integer, `bool` and `f32` results are re-extended, and so are narrow places passed as `mut` once the call returns.
- Every frame keeps `rsp` 16-byte aligned at its calls, as C requires: frames are rounded to 16 bytes, and `_start` rounds the space it reserves for `main`'s result.
- An `extern struct` value is its C bytes, one word per eightbyte. Its locals and parameters live in frame slots; a field is loaded or stored at its own width, and a struct literal or a field of a computed value goes through a temporary slot. A struct whose size is not a multiple of 8 is stored and loaded up to its last byte, with 4-, 2- and 1-byte accesses for the final partial word, so writing a nested field or an element of a `view` never touches the bytes after it.
- `defer` keeps a stack of deferred statements per block and lowers them again at every exit from that block.
- A struct's `drop` becomes a function `T..drop` whose parameters are the struct's fields, and every droppable type gets a generated `T..release` that calls it and then the releases of its droppable fields. A `using` binding adds a deferred `if flag { T..release(x) }` to its block; a move reads the value and clears the flag. Moves are found by a flow-sensitive pass over each checked function body, which joins moved states across branches and runs a loop body a second time when an iteration moves a binding.
- `==` on a struct is the conjunction of its fields' comparisons. On an enum it compares tags, then branches on the tag to compare only that variant's fields, since the other payload words are undefined. An `extern struct` is stored in two frame slots and compared field by field at each field's offset and width, so padding is ignored.
- A LowLang function value is two words: a code address and an environment pointer. Code reached through a function value takes the environment as parameter word 0. A closure is checked in place and becomes a function named after its parent, such as `counter..c1`; its environment is a heap array holding a box pointer per captured variable, or 0 when it captures none. A captured variable lives in a heap box allocated when it is bound, and every access, in its own function and in closures, loads and stores through the box. A named function used as a value goes through an adapter, `name..value`, which drops the environment and calls it.
- A call through a function value moves the code address to `r10` and calls it, passing the environment and the arguments like any call. A call through an `extern fn` value is a `ccall` with the same System V plan as a named C function.
- A C callback is a thunk, `cb_name`, emitted once per program for every function used as one. It saves the C argument registers, re-extends narrow integers and `bool` from them and from the C stack, calls the function with LowLang's convention on the green thread's stack and moves a float result to `xmm0`.
- Every loop's back-edge block is marked; the code generator ends it with `cmp byte ptr fs:[-8], 0` and a branch to an out-of-line `call rt_poll`.
- `spawn f(a, b)` becomes a block that binds the callee and the arguments to fresh locals and passes a closure calling `f` with them to `rt_spawn`. Sends, receives, `close` and channel loops call the prelude's instances for the element type; `select` calls the prelude's per-arm helpers and the runtime's `rt_lock_all`, `rt_timer_add` and `rt_park`.
- `funcaddr` gives a function's code address, its thunk, or an `extern` function's address through the GOT. Functions reached only through `funcaddr` are kept.
- In dev builds every C function gets a global trampoline, `cfn_name: jmp name`, and calls go through it, so a patch can link against the running binary.
- An `embed` becomes an address and a length. The data is emitted with `.incbin` under a label named by the file's content hash, so a changed file changes the code of every function that embeds it.
- Every value records whether it may hold a heap pointer. Loads, parameters and call results take this from their type's word layout, where an enum word counts as a pointer if any variant puts one there; constants, typed arithmetic, string literals and frame addresses never do; untyped address arithmetic and anything unclassified count as pointers; a phi holds a pointer if any input may.
- A `str` is two words and a slice three. An element access computes `pointer + index * stride` after a `check` instruction, which compares unsigned and jumps to the index panic; packed elements load with `movzx`, `movsxd` or a 32-bit `mov` and store at their width.
- A map `[K: V]` is a pointer to the prelude's generic `__Map[K, V]`, whose `entries` slice holds `__Entry[K, V]` records of a live flag, the key and the value. An open-addressing index of entry numbers with linear probing finds them; deleted entries leave tombstones until the next rebuild, which compacts the entries in order and resizes the index to keep it at most three quarters full. Integer, `bool` and pointer keys hash their word; `str` keys hash their bytes with FNV-1a. Every concrete map type instantiates `__map_new`, `__map_get`, `__map_set`, `__map_delete`, `__map_len`, `__map_next`, `__map_key` and `__map_value` for its key and value types, and map literals, lookups, assignments, `delete`, `.len` and loops become calls to them. A compound assignment such as `m[k] += v` is a get, the operation on the value or zero, and a set.
- `push` and the formatting helpers call the runtime's `rt_grow` and the prelude; the buffer behind an interpolation lives in a frame slot passed as a `mut` argument. The prelude appends a whole string to a `[]u8` with `__extend`, which grows the buffer at most once and copies the bytes with the runtime's `rt_copy`, so concatenation, interpolation, `bytes` and `str(b)` copy in bulk. String ordering calls the prelude's `__str_cmp`.
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


### Vectors

- A vector local lives in a frame slot, like a variable passed as `mut`, and a vector expression evaluates to the address of a slot holding its result. A vector operation is one IR instruction, `vector`, taking the destination and operand addresses, or `vectorscalar` for `mask`. At calls, returns, fields and other aggregates a vector is its words, loaded from or stored to a slot. A `let` of a computed vector adopts the slot of the result instead of copying it.
- A function containing vector instructions is compiled twice. Its symbol is a stub that jumps to the AVX2 version when the CPU and the operating system support AVX2, and to the SSE2 version otherwise; `rt_cpu_init` checks `cpuid` and `xgetbv` at startup. `LOW_SIMD=sse2` in the environment selects the SSE2 versions, which the fixture runner uses to test both.
- The AVX2 version uses three-operand VEX instructions on `ymm` registers. A slot written once and read only by vector instructions later in the same block, with no call in between, is kept in `ymm8` to `ymm15` instead of memory. `vzeroupper` runs before every call and return of such a function.
- The SSE2 version splits 256-bit operations into two 128-bit halves and keeps every vector in memory.
- An operation without an instruction in the version being generated runs as a scalar loop over the lanes in a scratch area of the frame: `pshufb` on SSE2, 64-bit multiplication, minimum, maximum and comparison where unavailable, byte shifts and arithmetic 64-bit shifts. Unsigned comparisons flip the sign bit of both operands and compare signed.

## Hot reload

`low dev app.low` or `low dev app/` builds a dev binary, runs it, and watches every source file of every loaded module, every embedded file, and the file lists of their directories. On save:

- A file that fails to compile is reported and the running program is left alone.
- The program is restarted when the set of linked libraries changes, when a struct or enum layout changes, when an existing function's signature or a closure's captured variables change, or when `main` changes. A running frame of old code may still call any function, so it must keep the old calling convention and data layout; `main` never returns to pick up new code.
- Otherwise every function whose generated assembly differs from the running version is recompiled, including callers whose code changed only because a callee did, and every instance of an edited generic function. New functions are included. Heap, stack and all live values are kept.

Mechanism:

- Dev builds call every function through `fn_table`, a writable table of 4096 code addresses, with `call [rip+fn_table+8*i]`. Release builds call directly.
- In dev builds a function value's code word is the address of its `fn_table` entry and calls through it use `call [r10]`, so a stored function value or closure runs the newest code. Callback thunks call through the table too.
- At start the dev runtime installs a `SIGUSR1` handler and maps a 64 MB read-execute region at `0x40000000`, close enough to the binary for 32-bit relative references.
- A patch is assembled with only the changed functions and their strings, then linked at the next free address in that region against the running binary's symbols (`ld -R`). Patched functions get a fresh symbol suffix so they never collide with the originals.
- `low dev` writes the machine code into the region through `/proc/<pid>/mem`, writes `(index, address)` pairs and a count into a mailbox in the program, and sends `SIGUSR1`. The handler stores each address into `fn_table` with an aligned 8-byte write and clears the count, which `low dev` waits for.
- Frames already running keep executing their old code; the next call through the table enters the new code. Old code is never unmapped.

## UI groundwork

The UI library is staged under `ui/` until it moves into the standard library as `std/ui`.

- `ui/protocols` holds the Wayland protocol XML the library is built from: `wayland.xml` and `xdg-shell.xml`.
- `go run ./cmd/wlgen -o wayland.low ui/protocols/wayland.xml ui/protocols/xdg-shell.xml` generates LowLang bindings for libwayland-client. Every interface given in one run shares an `Interfaces` record of `wl_interface` tables that `interfaces()` builds in C memory; a reference to an interface that no given protocol declares is an error.
- A request becomes `interface_request(...)` over `wl_proxy_marshal_flags`, taking the `Interfaces` record when it creates an object of a known interface; `wl_registry_bind` takes the interface table and version. Destructor requests pass `WL_MARSHAL_FLAG_DESTROY`, and an interface without a `destroy` request gets one that calls `wl_proxy_destroy`. Objects are `ptr`, strings `ptr[u8]`, `fixed` an `i32`.
- Events become an `extern struct` listener of `extern fn` fields, each defaulting to a generated function that ignores the event, so a listener names only the events it handles; `interface_add_listener` copies it into C memory. Callbacks receive the listener's `data` pointer, usually a pinned LowLang object.
- Enum entries are functions such as `wl_shm_format_argb8888() -> u32`. Names that clash with LowLang keywords or builtins gain a trailing `_`.
- `ui/shaders` holds GLSL for instanced quads with per-corner radii, borders and clipping, blurred shadows, and monochrome glyph sprites sampled from an `R8` atlas. Instances are read from a storage buffer at set 0 binding 0 and the viewport size is a push constant. `go run ./cmd/shadergen ui/shaders` compiles each stage with `glslc` to a checked-in `.spv` and records the sources' SHA-256 sums in `sources.sum`.
- `ui/registry` holds the Vulkan registry, `vk.xml.gz` from Vulkan-Docs v1.4.363, and `vulkan.list`, which selects extensions, commands and extra types such as `pNext` structures. `go run ./cmd/vkgen -registry ui/registry/vk.xml.gz -select ui/registry/vulkan.list -o vulkan.low` generates LowLang bindings for the selected commands and every type they reach, skipping Vulkan SC variants. Enum values come from the core enums, every Vulkan feature level and the selected extensions.
- In the Vulkan bindings a handle is a one-field `extern struct`, `ptr` for dispatchable handles and `i64` for the others, so `VkBuffer {}` is the null handle and handles of different kinds do not mix. Structures and unions become `extern struct`s whose fields all have defaults: zero, `null`, the empty handle, or for `sType` the structure's own value, so a literal names only what it sets. A union takes the layout of its widest member. A fixed C array becomes numbered fields, `name_0` to `name_N`, with an accessor `Struct_name(s)` that returns them as a slice, or as a `str` for a character array.
- Vulkan types map to `u32` (`uint32_t`, `VkBool32`, `VkFlags`, 32-bit flag bits), `i32` (`int32_t`, enums), `i64` (`uint64_t`, `VkDeviceSize`, `VkFlags64`, `size_t`), `u16`, `u8`, `f32` and `f64`. Pointers to known types become `ptr[T]`; `void*`, function pointers and pointers to pointers become `ptr`. A command's single input or output through a pointer is a `mut` parameter; an array or optional pointer is a `ptr[T]`. Enum entries and API constants are functions such as `VK_FORMAT_R8G8B8A8_UNORM() -> i32`, and `vk_make_api_version` builds version numbers.

## Tests

`internal/wlgen` tests the generated signatures and checks, then builds a registry client from the vendored protocols and, when a Wayland compositor is running, runs it. `internal/vkgen` tests declaration parsing, extension enum values and the selection format, checks the shape of the generated bindings, and renders a rounded, bordered quad offscreen with the quad shaders through the bindings and dynamic rendering, comparing pixels; it skips without libvulkan or a Vulkan 1.3 device. `internal/shaders` checks that the SPIR-V matches the recorded source sums and validates it with `spirv-val` when installed.

`internal/hot` tests reload against running programs: state kept across a code swap, functions added, compile errors leaving the program running, heap data surviving a reload while the collector runs, stored function values, closures and callbacks running patched code, instances of an edited generic function patched,
 and restarts on layout, signature and capture changes.

`go test ./...` also compiles every `testdata/run/*.low` and every program directory in `testdata/run`, runs it with a one-minute limit and compares stdout with `.out` and stderr with `.err`, and checks peak memory against `.maxrss` in KB when present. A program with a `.needed` file must link exactly those shared libraries; every other program must be static with no interpreter. Each `testdata/fail/*.low` file or directory must fail to compile with the message in its `.err`, with file paths shown relative to the fixture.

## Implemented

- Types: `i64`, `i32`, `u32`, `u16`, `u8`, `f64`, `f32`, `bool`, `str`, slices `[]T`, maps `[K: V]`, structs, enums with payloads, results `T ! E`, pointers `*T`.
- `let`, `var`, assignment, `if`/`else`, `for cond`, `for` with `break`/`continue` and loop labels, `return`.
- Functions with parameters and return values, recursion, and `x.f(args)` calls and `x |> f(args)` pipes passing `x` as the first argument.
- Arithmetic, comparison, logical, bitwise and shift operators; numeric conversions; `sqrt`, `floor` and `exp`.
- `match`, `fail`, `?`, `catch`.
- Optionals `T?` with `none`, `some`, `??` and implicit wrapping.
- `mut` parameters.
- Pointers, `new`, and a garbage-collected heap.
- Range, slice and string loops; slice literals, indexing, slicing and `push`; string interpolation, concatenation, equality and ordering.
- Map literals, lookups, entry assignment, `delete` and map loops.
- Builtins `print` for numbers, `bool` and `str`; `bytes` and `str`.
- Modules as directories, imports, and `pub` visibility.
- A standard library of strings, number parsing and formatting, math, slices, files, standard input and output, the process environment and clocks.
- `embed` for compile-time file contents.
- `extern` blocks naming C libraries and declaring their functions, and dynamic linking against libc and the libraries whose functions reachable code uses.
- C interop: System V calls, `extern struct`, `ptr` and `ptr[T]`, `view`, `load`, `store`, `cstr`, `str(p)`, `pin` and `unpin`.
- `defer`, and droppable structs with `drop` and move-only `using` bindings.
- Generic functions, structs and enums with built-in constraints, compiled per type; maps built on generic prelude code.
- Structural `==` and `!=` on structs and enums.
- Field defaults and trailing children blocks for struct literals and calls.
- Function values and closures capturing by reference; `extern fn` C function pointers and C callbacks.
- Green threads on an M:N scheduler with `spawn`, channels, `select`, `sleep`, descriptor waits through epoll, deadlock detection and stack overflow checks.
- Derived serialization, deserialization and schemas for every type, and `T.field` references.
- Regular expressions generated at compile time from literal patterns with `regex.generated`.
- 128- and 256-bit vector types with lane-wise operators, compiled for AVX2 and SSE2 with dispatch at startup.
- Standard library: files, networking with DNS, pipelines, HTTP/1.1 and HTTP/2 clients and servers, WebSocket, TLS 1.2 and post-quantum TLS 1.3, JSON, the web framework with OpenAPI and authentication, the SQL driver contract, PostgreSQL and SQL Server drivers and the ORM, TOML, YAML and XML, protobuf, OpenTelemetry, DEFLATE/zlib/gzip, xz, Zstandard, LZ4 and Brotli compression, bitsets and Roaring bitmaps, image codecs, checksums, and cryptography: AES-256-GCM, ChaCha20-Poly1305, SHA-2, SHA-3, SHAKE, BLAKE2b, HMAC, HKDF, PBKDF2, Argon2id, X25519, ML-KEM-512/768/1024, Ed25519, ECDSA P-256/P-384, RSA, X.509 and JOSE.
- Hot reload with `low dev`.
- Targets: linux/amd64, and windows/amd64 with the Win64 C ABI and C libraries imported from DLLs (`"c"` and `"m"` resolve to msvcrt.dll); `low dev` runs on Linux only.

## Roadmap

1. Standard library, written in LowLang:
    1. Vector values in registers across blocks and calls, and an AVX-512 version.
    2. QUIC (RFC 9000, 9001 and 9002) over UDP with the TLS 1.3 handshake, loss recovery and congestion control; HTTP/3 (RFC 9114) with QPACK (RFC 9204).
    3. WebRTC data channels: ICE with STUN, DTLS, SCTP over DTLS and the data channel establishment protocol; media over SRTP.
    4. Cryptography:
        1. Hashing: BLAKE3, with keyed hashing, key derivation and extendable output.
        2. Random generation: a ChaCha20 CSPRNG seeded from `getrandom` for `crypto.random`, and AES-256-CTR-DRBG (NIST SP 800-90A).
        3. Post-quantum signatures: ML-DSA-44, ML-DSA-65 and ML-DSA-87 (FIPS 204), SLH-DSA (FIPS 205), and Falcon (FN-DSA).
        4. HPKE (RFC 9180) with the DHKEM(X25519) and X25519MLKEM768 KEMs, HKDF-SHA-256 and the AES-256-GCM and ChaCha20-Poly1305 AEADs.
        5. Shamir's secret sharing over GF(2^8).
        6. VOPRF (RFC 9497) over ristretto255 and P-256.
        7. Zero-knowledge proofs: Groth16 over BN254 and BLS12-381, Halo2 with KZG and IPA commitments, and STARKs over FRI.
        8. Homomorphic encryption: BFV, CKKS and TFHE.
2. GPU UI library with GPUI's renderer and platform layer and a signal-based state model, Linux first:
    1. A Wayland window through libwayland-client, with bindings generated from protocol XML; a Vulkan swapchain through `VK_KHR_wayland_surface`; a clear colour.
    2. A scene of GPU primitives (shadows, quads with rounded corners and borders, paths, underlines, monochrome, subpixel and polychrome sprites) drawn by GLSL shaders compiled to embedded SPIR-V. Primitives are sorted by draw order, batched by kind and atlas texture, and clipped by content masks.
    3. Text: font discovery with an embedded fallback font, OpenType parsing, rasterizing into a glyph atlas, shaping, line layout, Unicode grapheme and line-break segmentation from embedded Unicode tables.
    4. A retained element tree. Components run once at mount and build elements; only the closures that read a signal run again.
    5. `Signal[T]`, `Memo[T: Eq]` and effects. Reads are tracked through a current-observer scope on the UI thread. A memo whose new value is equal to the old one does not propagate.
    6. Reactive structure through explicit containers: a dynamic branch, and a keyed dynamic list.
    7. Incremental layout: a flexbox subset with dirty flags that propagate to ancestors and cached measurements, so a change re-lays out only the affected subtrees.
    8. Incremental paint: per-element scene fragments are reused, and only dirty elements re-emit primitives.
    9. Styles with partial overrides for hover and active states, driven by signals.
    10. Input: libxkbcommon keyboard, key repeat, hitboxes, mouse dispatch, focus and tab stops, actions and keymaps. Focus and hover state are signals.
    11. Foreground and background executors on the scheduler; a background task's result is written into a signal on the UI thread.
    12. Text input element whose state is held in signals, clipboard, IME (`text-input-v3`), fractional scale, `cursor-shape`, `xdg-decoration` and client-side decorations.
    13. Portals and AT-SPI over D-Bus, PNG images, SVG paths.
3. Own assembler and linker replacing `as` and `ld`, with static and dynamic ELF output.
4. Further targets: linux/arm64, darwin; macOS (Metal) and Windows backends for the UI library.
