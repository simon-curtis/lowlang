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
- `x.f(args)` calls `f(x, args)`, so calls chain left to right: `"ab".repeat(3).first()`. `f` is any function callable by its plain name: a function in the current module, a function value in scope, a generic function or a builtin. A struct field of function type named `f` takes precedence, so `op.apply(v)` calls the stored function. When the first parameter is `mut`, `x` is passed as `mut` and must be a mutable place.
- `x |> f(args)` also calls `f(x, args)`, and `x |> f` calls `f(x)`. The right side is a call, a function name, a module function such as `text.repeat`, a closure or a function-valued field; any other expression is an error. `|>` has the lowest precedence of the binary operators and groups left to right, so `a + b |> f()` is `f(a + b)`. A trailing `?` applies to the call: `x |> parse()?` is `parse(x)?`. A `mut` first parameter receives `x` as `mut`, as with `x.f(args)`.
- A line that starts with `|>` or `.` continues the expression on the line before it.

### Control flow

```low
if n > 0 { print(n) } else { print(0) }

for i in 0..10 { print(i) }
for x in xs { print(x) }
for i, x in xs { print(i) }
for n > 0 { n -= 1 }
for { break }
```

- `for i in a..b` counts from `a` up to but not including `b`; the bounds are integers of one type.
- `for x in xs` visits each element of a slice, and `for i, x in xs` also binds its index as `i64`.
- Loop variables are immutable. `continue` moves to the next element.

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
- `==` and `!=` compare contents.

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

- `match` must cover every variant, or end with `_`. Arms after `_` and duplicate arms are errors.
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
- `x ?? fallback` gives the value inside `x`, or evaluates `fallback` when `x` is `none`. The fallback may itself be a `T?`, making the result a `T?`. `??` binds tighter than comparisons and looser than arithmetic, and groups to the right, so `a ?? b ?? 0` tries `a`, then `b`.
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
- `catch { arms }` matches the error exhaustively; `catch value` substitutes a fallback.
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

- `extern "name" { ... }` at the top level of a file names a C library and declares its functions, with no bodies. `"c"` is the C library itself. A trailing `...` makes a function variadic. Declarations are private to their module unless marked `pub`.
- A program that names no library is a static executable with no libc. A program that names any is a dynamic executable linked against libc and each named library.
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
- The project root is the nearest directory, starting at the entry, that holds a `low.mod` file; without one it is the entry's directory. `import ui/text` loads `<root>/ui/text/`.
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
  - `Ord` adds `<`, `<=`, `>` and `>=`, and is met by numbers.
  - `Hash` adds use as a map key, and is met by integers, `bool`, `str` and pointers.
  - `Number` adds arithmetic, numeric literals, conversion with `T(x)` and `print`.
  - `Integer` adds bitwise operators and shifts and implies `Hash`.
  - `Float` is met by `f64` and `f32`.
  - An unconstrained parameter can only be moved, stored and passed. Anything else a generic function needs, such as an ordering of its elements, is passed in as a function value.
- A generic body is checked once against its constraints, so a missing constraint is reported at the definition, not at a use.
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

The garbage collector is a non-moving mark-sweep collector in the runtime:

- The heap is a 16 GB reservation made with `mmap` and `MAP_NORESERVE`; pages are committed as they are touched.
- Each object has a one-word header holding its size in words, a mark bit, a free bit and a noscan bit. A bitmap records the first word of every object and free chunk, which resolves pointers into the middle of an object, such as a `mut` parameter aimed at a heap field.
- Allocation takes from exact-size free lists for objects under 64 words, then first-fit from a list of larger chunks, splitting them, then from the end of the heap.
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
- `push` and the formatting helpers call the runtime's `rt_grow` and the prelude; the buffer behind an interpolation lives in a frame slot passed as a `mut` argument.
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
- `let`, `var`, assignment, `if`/`else`, `for cond`, `for` with `break`/`continue`, `return`.
- Functions with parameters and return values, recursion, and `x.f(args)` calls and `x |> f(args)` pipes passing `x` as the first argument.
- Arithmetic, comparison, logical, bitwise and shift operators; numeric conversions; `sqrt`, `floor` and `exp`.
- `match`, `fail`, `?`, `catch`.
- Optionals `T?` with `none`, `some`, `??` and implicit wrapping.
- `mut` parameters.
- Pointers, `new`, and a garbage-collected heap.
- Range, slice and string loops; slice literals, indexing, slicing and `push`; string interpolation, concatenation and comparison.
- Map literals, lookups, entry assignment, `delete` and map loops.
- Builtins `print` for numbers, `bool` and `str`; `bytes` and `str`.
- Modules as directories, imports, and `pub` visibility.
- `embed` for compile-time file contents.
- `extern` blocks naming C libraries and declaring their functions, and dynamic linking against libc when any is named.
- C interop: System V calls, `extern struct`, `ptr` and `ptr[T]`, `view`, `load`, `store`, `cstr`, `str(p)`, `pin` and `unpin`.
- `defer`, and droppable structs with `drop` and move-only `using` bindings.
- Generic functions, structs and enums with built-in constraints, compiled per type; maps built on generic prelude code.
- Structural `==` and `!=` on structs and enums.
- Field defaults and trailing children blocks for struct literals and calls.
- Function values and closures capturing by reference; `extern fn` C function pointers and C callbacks.
- Green threads on an M:N scheduler with `spawn`, channels, `select`, `sleep`, descriptor waits through epoll, deadlock detection and stack overflow checks.
- Hot reload with `low dev`.
- Target: linux/amd64.

## Roadmap

1. GPU UI library with GPUI's renderer and platform layer and a signal-based state model, Linux first:
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
2. Own assembler and linker replacing `as` and `ld`, with static and dynamic ELF output.
3. Further targets: linux/arm64, darwin, windows; macOS (Metal) and Windows backends for the UI library.
