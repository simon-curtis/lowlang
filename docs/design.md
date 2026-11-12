# Low Design

Low compiles to static native binaries with no libc dependency, has a tracing garbage collector and green threads with channels, and adds sum-type enums and typed error returns on top of a Go-sized core.

| Command | Does |
| --- | --- |
| `low run` | Compiles and runs a file or a module directory. |
| `low build` | Writes a native binary. `-o` names it and `-target` picks `linux/amd64` or `windows/amd64` and defaults to the host. |
| `low dev` | Runs the program and patches changed functions into it on save. Changing `main`, a type's layout or a function's signature restarts it. Needs a Linux host. |
| `low ir` | Prints the optimised SSA form. |
| `low asm` | Prints the generated assembly. |
| `low lint` | Type-checks the program and prints its warnings. |
| `low doc` | Prints the public API of every module under a directory as JSON. |
| `low lsp` | Starts the language server that the editor extensions use. |
| `low version` | Prints the release, or `dev+` and the commit for a build from source. |

A directory is a module. A `low.mod` file marks the project root, and `import ui/text` loads `ui/text/` under it. The standard library is imported as `std/...`, and the editor extensions need `low` on `PATH`.

Build from source with Go 1.27.3:

```
$ git clone https://github.com/simon-curtis/lowlang
$ cd lowlang
$ go install ./cmd/low
```

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

- `let` is immutable, `var` is mutable.
- Types are inferred and may be written as `let x: i64 = 1`.

### Functions

<!-- prelude
import std/text
enum ParseError {
    Bad
}
fn negate(x: i64) -> i64 {
    -x
}
fn parse_int(s: str) -> i64 ! ParseError {
    0
}
var items = [1, 2, 3]
let input = "42"
fn main() ! ParseError {
-->
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
- A named function is a value of its function type. `fn(params) -> R { body }` is a closure. Parameter and result types may be omitted when the expected function type supplies them; a closure with no written result and no expected type returns nothing.
- Values of function type can be stored in variables, fields, slices and maps, passed and returned, and called like functions. They cannot be compared or printed.
- A closure captures the variables it names by reference, as Go does: it reads and assigns the variable itself, which lives as long as any closure holding it.
  - Each loop iteration binds fresh variables, so closures made in different iterations see different ones.
  - A `mut` parameter cannot be captured.
  - A variable that is never assigned after its binding, never passed as `mut` and never modified by `push`, `pop`, `truncate` or `delete` cannot be told apart from a copy, so the closure holds its value directly.
- `x.f(args)` calls `f(x, args)`, so calls chain left to right: `"ab".repeat(3).first()`.
  - `f` is any function callable by its plain name: a function in the current module, a function value in scope, a generic function or a builtin.
  - When no such name exists and `x` is a struct or enum, or a pointer to one, from another module, `f` is that module's public function: `url.authority()` calls `http.authority(url)`.
  - A struct field of function type named `f` takes precedence, so `op.apply(v)` calls the stored function.
  - When the first parameter is `mut`, `x` is passed as `mut` and must be a mutable place.
- `x |> f(args)` calls `f(x, args)`, and `x |> f` calls `f(x)`.
  - The right side is a call, a function name, a module function such as `text.repeat`, a closure, or a struct field holding a function; any other expression is an error. `x |> ops.scale` calls the stored function, never a function named `scale`.
  - `|>` has the lowest precedence of the binary operators and groups left to right: `a + b |> f()` is `f(a + b)`.
  - A trailing `?` applies to the call: `x |> parse()?` is `parse(x)?`.
  - A `mut` first parameter receives `x` as `mut`, as with `x.f(args)`.
- A line that starts with `|>` or `.` continues the expression on the line before it.

### Control flow

<!-- prelude
var n = 3
let xs = [1, 2, 3]
let grid = [[1, 2], [3, 4]]
let want = 4
-->
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
- `for x in xs` visits each element of a slice; `for i, x in xs` also binds the index as `i64`.
- Loop variables are immutable.
- `break`, `continue`, `return` and `fail` are expressions of type never, so they can stand wherever a value is expected: after `??`, in an arm, or as a branch of `if`.
- `name:` before `for` labels a loop. `break name` and `continue name` act on that loop from inside nested loops and run the deferred statements and drops of every block they leave. A label must be used, cannot repeat the label of an enclosing loop, and is not visible inside closures.

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
- Integer literals are decimal, or hexadecimal, binary and octal with `0x`, `0b` and `0o`; `_` may separate digits. Float literals may have an exponent.
- An integer literal takes the type the context expects, including the other operand of a binary operator, and must fit it; otherwise it is `i64`. A literal above the `i64` range is accepted only as a `u64`. In an `if` expression with no expected type, an integer literal branch takes the type of the other branch.
- A float literal is `f64` unless an `f32` is expected.
- Integer arithmetic wraps at the type's width. Division by zero panics.
- Operands of a binary operator have the same type. Conversions are explicit calls such as `u8(x)`, `f64(n)` and `i64(f)`: integer conversions truncate or extend, float to integer truncates toward zero.
- `&`, `|`, `^`, `~`, `<<` and `>>` work on integers. `>>` is arithmetic on signed types and logical on unsigned ones. The shift count may be any integer type; a count of 64 or more, or a negative one, gives 0, or the sign fill for `>>` on a negative signed value.
- Binary operators bind as in Go, from tightest to loosest: `*`, `/`, `%`, `<<`, `>>`, `&`; then `+`, `-`, `|`, `^`; then `??`; then comparisons; then `&&`; then `||`. So `a - b >> 1` is `a - (b >> 1)` and `x | y - z` is `(x | y) - z`.
- `%` is integer only. Floats follow IEEE 754: division by zero gives an infinity, and comparisons with NaN are false except `!=`.
- `sqrt`, `floor` and `exp` take and return `f32` or `f64`.
- `print` accepts any integer or float type.

### Vectors

```low
import std/bits
import std/simd

fn find(data: []u8, c: u8) -> i64 {
    let needle: u8x32 = simd.splat(c)
    var i = 0
    for i + 32 <= data.len {
        let chunk: u8x32 = simd.read(data, i)
        let hits = simd.mask(chunk == needle)
        if hits != 0 {
            return i + bits.trailing_zeros(hits)
        }
        i += 32
    }
    -1
}
```

- Vector types are 128 or 256 bits of one lane type: `u8x16`, `u8x32`, `u16x8`, `u16x16`, `i32x4`, `i32x8`, `u32x4`, `u32x8`, `i64x2`, `i64x4`, `u64x2`, `u64x4`, `f32x4`, `f32x8`, `f64x2` and `f64x4`.
- `+`, `-`, `*`, `&`, `|`, `^`, `~` and unary `-` work lane by lane. `/` works on float lanes. `<<` and `>>` work on integer lanes by a scalar count, arithmetic on signed lanes. Compound assignments work the same way.
- Comparisons work lane by lane and give a vector of the same type whose lanes are all ones or all zeros; unsigned lanes compare unsigned.
- A scalar operand of the lane type is broadcast, so `chunk == '"'` compares every lane.
- `u8x32(v)` reinterprets a vector of the same width.
- `std/simd` provides `read` and `write` between a vector and a slice at an element index, `read_str`, `splat`, `mask` (one bit per lane from its top bit), `any`, `blend`, `min`, `max`, `lane`, `with_lane`, `shuffle` and `shuffle16` (a byte table lookup within each 16-byte half, zero where the index has its top bit set), `find_byte` and `find_byte_str`. Reads and writes are bounds-checked.
- Vectors are values: they can be passed, returned, stored in fields and slices, and compared lane by lane. They cannot be compared with `==` as a whole. They print as `<1, 2, 3, 4>`.

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
- `xs.len` is the length as `i64`.
- `xs[a..b]`, `xs[..b]` and `xs[a..]` share the elements in that range. The result's capacity equals its length, so `push` onto it always copies and never overwrites the parent.
- `push(mut xs, v)` appends, growing the capacity to twice plus four when full.
- `pop(mut xs)` removes the last element and returns it as `T?`, or `none` when `xs` is empty.
- `truncate(mut xs, n)` sets the length to `n` and panics unless `0 <= n <= xs.len`.
- `pop` and `truncate` keep the capacity, so a later `push` reuses the freed space.
  - They change only the binding they are given. A slice that shares its elements with another binding sees a later `push` overwrite the freed elements, as after any `push` that finds spare capacity. A reslice such as `xs = xs[..n]` is the form that copies on the next `push`.
  - Elements past the length stay in the array, and keep what they point to alive, until a `push` overwrites them.
- A function in the program named `pop` or `truncate` takes precedence over the builtin.
- A slice literal or map literal whose elements are all constants is built once per program, at its first evaluation, and reused.
  - A literal is used in place when it is bound with `let` and the binding is only indexed, iterated or asked for `.len`, or when it is indexed, iterated or measured directly.
  - Any other use gets a copy, because `push`, a slice of it, an argument, a return value, a field or an assignment through it would otherwise mutate the shared value.
  - A slice literal under 1024 bytes and a map literal under 4 entries are rebuilt for those uses, since copying them costs more than building them.
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

- `[K: V]` maps keys of an integer type, `bool`, `str` or a pointer type to values of any type.
- A map is a pointer to a table on the garbage-collected heap, so assigning or passing it shares the table, and `m[k] = v` works through any binding.
- `[k: v, ...]` builds a map; `[:]` is an empty one and needs an expected type.
- `m[k]` returns `V?`. `m[k] = v` inserts or replaces. A compound assignment such as `m[k] += 1` starts from a zero value when `k` is missing.
- `delete(mut m, k)` removes a key; removing a missing key does nothing.
- `m.len` is the number of entries.
- `for k, v in m` and `for k in m` visit entries in insertion order. An overwritten value keeps its key's position; a deleted and re-inserted key moves to the end. Inserting new keys while iterating may skip or repeat entries.
- A map literal of constants is built once per program in a release build. A use that only indexes, iterates or measures it shares the built table; any other use gets a copy from the prelude's `__map_clone`, which copies the entry array and the index. A literal of fewer than 4 entries is rebuilt for those uses, because the copy costs as much as the build.
- A map is an array of entries in insertion order plus an open-addressed index of one word per slot.
  - A slot is 0 when empty, -1 when its entry was deleted, and otherwise holds the upper 32 bits of the key's 63-bit hash above the entry number plus one. A probe skips empty slots and slots of other keys without reading the entries.
  - Iteration reads the entries, never the index, so the hash function never shows in the order. No program output depends on a hash value.
  - The map code reads the low bits of the hash as the position and the upper 32 bits as the tag.
- Hashes are seeded per process, so no key set can be built to collide without the seed. Start-up fills `rt_hash_seed` with 128 random bits and forces the second word odd.
  - On Linux the bits are the 16 bytes at `AT_RANDOM`, or `getrandom` when the auxiliary vector has none, or the fallback when that fails. On Windows they come from `ProcessPrng`, with the same fallback.
  - The fallback derives each word through its own SplitMix64 finalizer: `s0` from one RDTSC read, the stack address and one RDRAND word, and `s1` from a second RDTSC read taken after a `cpuid`, a rotated stack address and a second RDRAND word. RDRAND contributes only when CPUID reports it and the instruction succeeds within 10 tries.
- Hash function, with `s0` and `s1` the two seed words and `mum(a, b)` the high 64 bits of the 128-bit product of `a` and `b` xor its low 64 bits:
  - An integer, `bool` or pointer key `k` hashes to `fold(mum(k ^ s0, s1))`.
  - A `str` of 16 bytes or fewer reads two words `a` and `b`. 8 to 16 bytes give the first and last eight. 4 to 7 give the first and last four packed into `a` with `b` zero. 1 to 3 give the first, middle and last byte. The empty string gives zero.
  - Every `str` starts from `acc = mum(s1 ^ length, s0 | 1)`, so the length reaches the hash only through a product with seed words.
  - A longer string folds every 16-byte block while more than 16 bytes remain, computing `acc = mum(w0 ^ s0, w1 ^ acc)`, and then reads its last 16 bytes as the two words.
  - A `str` hashes to `fold(mum(a ^ s0, b ^ acc))`. `fold(h)` is `x = h ^ h >> 32` followed by `(x ^ x >> 16) >> 1`, which keeps 63 bits.
- Two strings of different lengths whose input words differ by a chosen xor do not collide under every seed, because cancelling the length term needs `mum(s1 ^ n, s0 | 1) ^ mum(s1 ^ n', s0 | 1)`, which depends on the seed. Collisions between distinct strings remain possible with probability about 2^-63 per pair, and the hash gives no protection once `s0` and `s1` are known.

### Strings

<!-- prelude
let age = 36
let score = 9.0
-->
```low
let name = "Ada"
let line = "{name} is {age}, ratio {score / 2.0}"
let row = "{name,-8}|{age,4:d3}|{score,8:f2}|{age:x4}|{score:#,##0.0#}"
let full = name + " Lovelace"
for c in "héllo" { print(c) }
let first: u8 = name[0]
let copy = str(bytes(name))
```

- `str` is an immutable pointer and length over UTF-8 bytes.
- `s.len` is the length in bytes, `s[i]` reads a byte as `u8`, and `s[a..b]` shares a byte range.
- `for c in s` visits code points as `u32`; `for i, c in s` also binds each code point's byte offset. Invalid UTF-8 yields U+FFFD for one byte.
- `"{expr}"` interpolates a value of any type; `{{` and `}}` are literal braces. `print(expr)` writes the same text followed by a newline.
- `"{expr,width:format}"` aligns and formats the value, as in `"hello {name,13} your age is {age:##}"`, `"{price,10:f2}"`, `"{id:x8}"` and `"{n,-6}|"`. Both parts are optional, and `{x,}` and `{x:}` are errors.
  - The expression ends at the first `,` or `:` outside brackets, which is where the width starts; the width ends at the next `:` or at the closing brace; the format is all the text up to the closing brace, so it may contain `,` and `.`.
  - A width is a non-zero decimal integer with an optional `-` and optional surrounding spaces. A positive width right-aligns and a negative one left-aligns, padding with spaces to that many code points. A longer text is never cut. A width applies to every type, including `str` and the default text of a struct.
  - A format has no spaces and is checked at compile time. It applies to integers and floats, and is either a standard format, one letter with an optional precision of 0 to 99, or a picture. The compiler lowers it to prelude calls with constant parameters, so no format text is parsed at run time.
- Standard formats, where `N` is the precision:

  | Format | Types | Output |
  |---|---|---|
  | `dN` | integers | decimal with at least `N` digits, zero-padded after the sign: `{-5:d3}` is `-005` |
  | `xN`, `XN` | integers | lower or upper case hex with at least `N` digits. A negative value is its two's complement at the width of its type: `{i32(-1):x}` is `ffffffff`, `{-1:x}` is `ffffffffffffffff` |
  | `bN` | integers | binary, with the rules of `x` |
  | `fN` | integers, floats | fixed point with `N` decimals, 2 by default: `{3.14159:f3}` is `3.142`, `{7:f1}` is `7.0` |
  | `eN` | integers, floats | one digit, then `N` decimals (6 by default) and an exponent written like the default float text: `{12345.678:e2}` is `1.23e+4` |
  | `nN` | integers, floats | like `f` with a `,` between every three integer digits; `N` is 0 for an integer and 2 for a float by default: `{1234567:n}` is `1,234,567` |

- A picture is a format of only `0`, `#`, `,` and `.`, with at least one `0` or `#` and at most one `.`.
  - In the integer part each `0` is a digit that is always written and each `#` is a placeholder. A `,` between two placeholders groups the integer digits in threes. With no `0` in the integer part a zero integer part is omitted: `{36:##}` is `36`, `{0:##}` is empty and `{0.5:#.##}` is `.5`, while `{0.5:0.##}` is `0.5`.
  - In the fraction part the value is rounded to the number of `0`s plus `#`s, then trailing zeros are dropped down to the number of `0`s. The `.` is dropped when no decimals remain: `{2.5:0.##}` is `2.5`, `{2:0.##}` is `2`, `{3.14159:0.00}` is `3.14`, `{1234567.891:#,##0.00}` is `1,234,567.89` and `{5:000}` is `005`.
- Rounding is correct for the exact binary value of a float and breaks ties to even, as glibc's `printf` does: `{0.125:f2}` is `0.12`, `{0.375:f2}` is `0.38`, `{2.675:f2}` is `2.67` because 2.675 lies just below the tie, and `{1e300:f2}` writes every digit. An `f32` formats its own exact value and integers are exact up to the whole range of `u64` and `i64`. A negative value that rounds to zero keeps its sign, as in `-0.00`. `NaN`, `inf` and `-inf` ignore the format, and the width still applies.
- A format on a value of a type other than an integer or float is an error: `cannot format a value of type str with "f2"`. So is `d`, `x`, `X` or `b` on a float, an unknown letter, a precision above 99 and a picture with other characters, two `.` or no `0` or `#`.
- `a + b` concatenates. A chain such as `a + b + c + d`, with any interpolations inside it, is one operation that allocates the result once.
- Floats are formatted as the shortest decimal that reads back to the same value: positional for exponents from -6 to 20, otherwise `1.5e-7` or `1e+21`. Infinities print as `inf` and `-inf`, NaN as `NaN`.
- Every type has a default text, generated by the compiler with no annotation or constraint:
  - Integers, floats and `bool` use the formats above and `true`/`false`. A `str` that is itself printed or interpolated is inserted unchanged; a `str` inside another value is quoted with `"`, with `"`, `\`, newline and tab escaped.
  - A struct is `Point{x: 1, y: 2}`: its name without module or type arguments (`Stack{items: [1]}`) and every field, private ones included.
  - An enum variant is its name, `Leaf`; with positional fields `Node(a, b)`; with labelled fields `Move{x: 1, y: 2}`. An optional is `none` or its value.
  - A slice is `[1, 2, 3]`, a map `{k: v, k2: v2}` in iteration order with keys rendered like any nested value, and a vector `<1, 2, 3, 4>`. A `[]u8` is `0x` and lowercase hex digits, `0x6869`.
  - A pointer renders its target. A pointer met again while it is being rendered is `<cycle>`, so shared pointers render in full each time and only a cycle is cut.
  - A function or channel is its type, `fn(i64) -> i64` or `chan[str]`. A `ptr[T]` is its type and address, `ptr[u8](0x7ffd1c2a)`, and a null one `ptr[u8](0x0)`.
- `'a'` is a code point of type `u32`, or `u8` where one is expected.
- `bytes(s)` copies a string into a `[]u8`; `str(b)` copies one back.
- `==` and `!=` compare contents. `<`, `<=`, `>` and `>=` compare bytes in order, which for UTF-8 is code point order.
- Equality checks the lengths, then the pointers, then compares eight bytes at a time with an overlapping final load.

### Structs

<!-- prelude
enum Align {
    Start
    Center
}
struct Text {
    value: str
}
enum Element {
    Label(Text)
    Box(Div)
    Button(str)
}
fn button(label: str) -> Element {
    Button(label)
}
-->
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

- Literals name their fields, separated by commas or newlines; `x` alone means `x: x`. A field declared with `= expr` may be left out; every other field must be given.
- A default is any expression. It is checked once against the field's type, in the declaring module's scope, with no access to locals or other fields. It is evaluated at every literal that leaves the field out, after the written fields. A field with a default can be left out of a literal in another module even when it is private.
- A brace block on the same line after a struct literal holds its children: the entries, one per line or separated by commas, become a slice literal for the struct's `children` field.
- A brace block after a call becomes a slice literal passed as the last argument, so `fn button(label: str, children: []Element) -> Element` takes children the same way.
- A trailing block is not allowed where a struct literal is not.
- A struct literal cannot appear directly in an `if`, `for` or `match` condition; wrap it in parentheses.
- Structs are values: assignment and argument passing copy them.
- `==` and `!=` compare structs field by field when every field can be compared: numbers, `bool`, `str`, pointers, channels, and structs and enums made of them. Each field uses its own `==`, so strings compare by content, floats by value and pointers by identity.
- Fields are assigned through a `var` binding or a `mut` parameter, at any depth: `r.max.x = 1`. Other parameters are immutable.
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
- A `mut` argument must be a `var` binding, a `mut` parameter, a field or element of either, or a place through a dereferenced pointer.
- One call cannot pass overlapping places as `mut`: `swap(mut p.x, mut p.x)` is rejected, `swap(mut p.x, mut p.y)` is allowed. Two `mut` parameters therefore never alias.
- References exist only as parameters. They cannot be stored, returned or bound with `let`, so they never outlive the variable they refer to.
- A `mut` parameter occupies one word holding the address of the place.
- A place narrower than a word, such as an element of a packed slice like `[]u8` or a narrow `extern struct` field, is copied into a word-sized frame slot before the call and copied back at its own width after it, so the callee's word-sized reads and writes never touch the neighbouring bytes. Code in the callee that reads the same element through another route, such as a pointer to the slice, sees the value from before the call until the callee returns.

### Pointers

<!-- prelude
struct Counter {
    hits: i64
}
fn bump(mut c: Counter) {
    c.hits += 1
}
-->
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

- `match` must cover every variant, or end with `_` or a binding arm. Arms after `_` or a binding arm, and duplicate arms, are errors.
  - A binding arm is a name starting with a lowercase letter that is not a variant, such as `other => describe(other)`. It matches anything and binds the whole subject.
  - A capitalized name that is not a variant is an error, so a mistyped variant is never taken for a binding.
- An arm that starts with a literal (`4`, `-1`, `'x'`, `"a"`, `true`), a parenthesised expression, `!` or `~`, or a name followed by `.` (`Mode.Fast`, `limits.max`) is a test arm: an expression compared with `==`. `4 => ...` means `== 4 => ...` and `(base + 1) => ...` means `== base + 1 => ...`. A `match` with a test arm must end with `_`. A bare name is a variant or a binding, so write `(limit) => ...` to compare against a variable.
- An arm that starts with a binary operator tests the subject: `% 3 == 0 => ...` means `subject % 3 == 0`, with the subject on the left and ordinary precedence after it.
  - The subject is evaluated once and arms are tried in order.
  - Such tests work on a subject of any type, including enums (`== Empty`) and the error of a `catch`.
  - A `match` that uses one, or whose subject is not an enum, must end with `_`.
  - Arms are separated by newlines or commas.

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
- Code generation for a `match` depends on its arms. It applies when the arms before the last are all distinct variants of one enum, or all distinct integer or character literals of one type:
  - With at least 12 such arms whose values span at most 4096 and at most three times the arm count, it compiles to a `switch` block: the backend emits a bounds check and an indirect jump through a table of 32-bit offsets.
  - Without that density, 48 or more arms compile to a binary search over the values.
  - Fewer arms keep the chain of comparisons.
  - Duplicate values, test arms and arms with other expressions always keep the chain.
  - A `switch` on a constant folds to its target.
- An enum whose variants all have no payload is a single word. Otherwise an enum is a tag word followed by the largest variant's fields.
- An enum cannot contain itself by value; it can hold a `*` pointer to its own type.
- `==` and `!=` compare enums whose variant fields can all be compared: equal values have the same variant and equal fields. Optionals compare the same way, so `x == none` and `x == 5` work on an `i64?`.

### Optionals

<!-- prelude
let xs = [1, 7, 4]
-->
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
- A `T` is accepted wherever a `T?` is expected: an annotated `let`, an assignment, an argument, a field, a return and either branch of an `if` or `match`. `none` and `some(v)` take their type from that expectation.
- `x ?? fallback` gives the value inside `x`, or evaluates `fallback` when `x` is `none`.
  - The fallback may itself be a `T?`, making the result a `T?`.
  - The fallback may leave instead, as in `let line = read_line() ?? break`.
  - `??` binds tighter than comparisons and looser than arithmetic, and groups to the right: `a ?? b ?? 0` tries `a`, then `b`.
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
- `catch { arms }` matches the error exhaustively; a binding arm such as `e => report(e)` receives the whole error. `catch value` substitutes a fallback.
- `fn f() ! E` fails or returns nothing.
- A result cannot be discarded, bound or passed on without `?` or `catch`, except by returning it from a function with the same result type.
- When `main` declares `! E` and returns an error, the program prints `error: <Variant>` to stderr and exits with status 1. Panics exit with status 2.
- A result is a tag word (0 ok, 1 error) followed by the larger of the ok value and the error enum.

### Embedded data

<!-- prelude shaders/quad.spv
-->
<!-- prelude fonts/fallback.ttf
-->
```low
let shader = embed("shaders/quad.spv")
let font = embed("fonts/fallback.ttf")
print(shader.len)
```

- `embed("path")` is the contents of a file as a `str`, read at compile time. The path must be a string literal and is relative to the directory of the file that calls `embed`, so a module carries its own assets.
- The bytes live in the binary's read-only data, aligned to 16 bytes; nothing is copied at run time. Indexing, slicing, `.len` and `bytes(...)` work as on any string.
- A missing file is a compile error. `low dev` watches embedded files, and a change to one reloads the functions that embed it.
### C interop

<!-- prelude
struct Block[T] {
    data: ptr[T]
    drop free(data)
}
-->
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

- `extern "name" { ... }` at the top level of a file names a C library and declares its functions without bodies. `"c"` is the C library itself.
- A trailing `...` makes a function variadic.
- `fn errno_location() -> ptr[i32] = "__errno_location"` gives a function a Low name different from its C symbol, for symbols that start with `__` or are Low keywords.
- Declarations are private to their module unless marked `pub`.
- A library is linked when code reachable from `main` calls or takes the address of one of its declared functions, so importing a module that declares C functions costs nothing until one is used.
- A program that uses no library is a static executable with no libc. A program that uses any is a dynamic executable linked against libc and each used library.
- C signatures use C types: integers, floats, `bool`, `ptr`, `ptr[T]`, and `extern struct`s by value.
- A `mut` parameter passes the address of a Low place for the duration of the call, as C# passes `ref`. C may read and write through it.
- `ptr[T]` is a nullable pointer to C memory. `ptr` is `void*` and converts to and from any `ptr[T]`. `null` takes its type from context. `==` and `!=` compare addresses.
- `view(p, n)` is a `[]T` over `n` elements at `p`, bounds-checked like any slice. `load(p)` and `store(p, v)` read and write one `T`.
- `cstr(s)` is a NUL-terminated copy of a string. `str(p)` copies a NUL-terminated `ptr[u8]` back.
- `extern struct` has C layout: fields in order at their natural alignment, size rounded to the largest alignment. Its fields must have C types. It is used like any struct, and its fields are read and written at their own width.
- Memory may be passed to C for the duration of a call. Memory C keeps after the call must be pinned: `pin(x)` returns the address of a pointer, slice, map or string's data and keeps it alive until `unpin(p)`.
- Memory C allocates is never touched by the collector.
- `extern fn(ptr[i64], ptr[i64]) -> i32` is a C function pointer, one word, like C#'s `delegate* unmanaged`. Its parameters and result must have C types. It is a C type, so it can be a parameter, a result or an `extern struct` field.
- A top-level Low function becomes a C callback where an `extern fn` is expected, if it takes and returns only numbers, `bool`, `ptr` or `extern fn`, with no `mut` parameters.
- An `extern` function used as a value is its C address. Calling an `extern fn` value makes a C call.
- A closure reaches C through a `ptr` user-data argument: pin a pointer to the state it needs and pass a top-level callback that loads it.

<!-- prelude
extern "c" {
    fn malloc(n: i64) -> ptr
}
let n = 3
let data = malloc(n * 8)
-->
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

<!-- prelude
enum IoError {
    Closed
}
fn open(path: str) -> i32 ! IoError {
    3
}
fn close(fd: i32) {
}
fn log(line: str) {
}
let path = "config.low"
fn main() ! IoError {
-->
```low
let fd = open(path)?
defer close(fd)
defer {
    let summary = "closing {path}"
    log(summary)
}
```

- `defer` runs a statement when its enclosing block exits: at the end of the block, on `return`, `fail` or a propagated `?`, and when `break` or `continue` leaves it.
- Deferred statements of a block run last first.
- The statement sees variables as they are at exit.
- It cannot `return`, `fail`, use `?`, `break`, `continue` or contain another `defer`.

### Using

<!-- prelude
enum IoError {
    Closed
}
extern "c" {
    fn free(p: ptr)
}
fn close(fd: i32) {
}
fn flush(sock: i32) {
}
fn open_file(path: str) -> File {
    File { fd: 3 }
}
fn connect(addr: str) -> Conn ! IoError {
    Conn { sock: 4, buf: Block[u8] { data: null } }
}
let addr = "localhost:8080"
fn main() ! IoError {
-->
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

- `drop <stmt>` inside a struct body declares its cleanup, with the struct's fields in scope as borrowed values. Like `defer`, it takes one statement or a `{ }` block.
- A struct has at most one `drop`, and `using` cannot appear in it.
- A struct with a `drop` is droppable, and so is a struct with a droppable field, even without its own `drop`. Generic instances are droppable per instance: `Pair[File, i64]` is, `Pair[i64, i64]` is not.
- Releasing a value runs its own `drop` first, then releases its droppable fields in declaration order.
- `using x = expr` binds `x` and releases it when the enclosing block exits by any path: the end of the block, `return`, `fail`, `?`, `break` or `continue`. Releases and `defer`s run together, last first.
- `using var x = expr` can be reassigned. Assignment releases the old value, and exit releases the value held then.
- `using` cannot appear inside `defer`. Binding a non-droppable value with `using` is an error.
- A droppable value cannot be bound with `let` or `var`: "File has drop; bind it with using". Results of droppable types are unwrapped with `?` or `catch` straight into a `using` binding.
- Droppable values are move-only:
  - `return x`, or a function body ending in `x`, moves a `using` binding out. The caller owns the value and must bind it with `using`.
  - A struct literal field or an assignment to a field of a droppable value moves the value into it, and the outer value releases it. A field is assigned through a `using var` binding or a `mut` parameter, and the field's old value is released first.
  - `using y = x` moves `x` into `y`.
  - Passing a value as an argument borrows it: the callee cannot release it, store it or return it. A freshly produced value, such as a call result or a struct literal, must be bound with `using` before it is passed.
  - Any other copy is an error: binding it with `let`, storing it in a slice, map, channel, optional, pointer or enum variant, moving a field out of its struct, discarding a call result, or capturing it in a closure.
  - A struct with a droppable field is itself droppable, so it cannot be built with `new` or held by a pointer, and `spawn` cannot take one as an argument: "spawn cannot take File; it has drop". Code that shares a resource across closures, pointers or green threads passes a copyable view of it and binds the owner where the resource has one home (see Resource ownership).
  - Using a binding after it was moved is an error, as is using one that may have been moved on one branch or by an earlier loop iteration.
- Generic code is checked under these rules for each instance whose type arguments are droppable, so `fn same[T](x: T) -> T { x }` cannot be instantiated with `File`.
- Each `using` binding has a hidden flag that a move clears. The release at exit runs only while the flag is set, so a value moved on one branch is released exactly once.

### Modules

<!-- prelude geometry/geometry.low
pub struct Circle {
    pub radius: f64
}
pub enum Shape {
    Square(i64)
}
pub fn circle(radius: f64) -> Circle {
    Circle { radius }
}
pub fn area(c: Circle) -> f64 {
    3.14159 * c.radius * c.radius
}
-->
<!-- prelude ui/text/text.low
pub fn shout(s: str) -> str {
    s
}
-->
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

- A module is a directory. Every `.low` file in it shares one namespace, so files of a module need no imports between them.
- A file whose name ends in `_linux.low` or `_windows.low` belongs to the module only when building for that operating system. The standard library uses this to give `std/sys` and the operating-system parts of `std/os`, `std/process`, `std/net` and `std/crypto/x509` one implementation per target; a test requires every target to declare the same public items with the same doc comments.
- The project root is the nearest directory, starting at the entry, that holds a `low.mod` file. Without one it is the entry's directory.
- `import ui/text` loads `<root>/ui/text/`. A path starting with `std/` names a module of the standard library, which is compiled into `low`, so a project directory named `std` cannot be imported.
- `low build dir` builds the directory as the entry module. `low build file.low` builds that one file as the entry module, without the other files beside it.
- `low lint file.low` type-checks the program, prints warnings for the program's own modules (not the standard library), and exits with status 1 if there were any. `-target` picks the operating system whose files are checked and defaults to the host. `low build`, `low run`, `low asm` and `low ir` print the same warnings to stderr and continue. The language server reports them as warnings.
- `low doc dir` prints, as JSON, every module under the directory with its imports, each with the name it is used through, and its public structs, enums and functions, each with its declaration as text, its file and its line. A struct shows its public fields, and `...` stands for its private ones. A function whose first parameter is a public type of its module, or a pointer to one, names that type as its receiver. `low doc std` documents the standard library, and the website builds its package pages from it with `-target linux/amd64`. `-target` picks the operating system whose files are documented and defaults to the host.
- `prefer-match` flags an `if` / `else if` / `else` chain with at least two conditions in which every condition is `==` between one side-effect-free subject (a variable, field, index or dereference) and a non-negative integer, character or string literal or an enum variant. It suggests `match subject { 4 => ..., _ => ... }`.
- `prefer-loop-label` flags a `break` out of an inner loop that sets a boolean flag to a literal when the statement right after that loop is `if flag { break }` or `if flag { continue }` (or `if !flag` for a flag set to `false`), with no `else`. It suggests labelling the loop that `if` acts on and jumping to it with `break name` or `continue name`, reusing the label the `if` already names. Breaks inside a nested loop, closure, `spawn` or `defer` are not counted.
- Imports come before declarations and apply to their file.
- A module is used through its last path segment, `text.shout(...)`, or through a name given with `import t = ui/text`.
- Types are written `geometry.Shape` and built with `geometry.Point { ... }`. A variant can be named `geometry.Shape.Dot` or, where the type is expected, `Dot`.
- Declarations and struct fields are private to their module unless marked `pub`. A `pub` enum's variants are public.
- A struct with private fields without defaults cannot be built with a literal outside its module.
- An import cycle is an error, as is importing two modules under one name.

### Doc comments

```low
/// A point on the plane.
pub struct Point {
    /// Distance from the y axis.
    pub x: i64
    pub y: i64
}

/// Returns the point mirrored in the y axis.
pub fn mirror(p: Point) -> Point {
    return Point { x: -p.x, y: p.y }
}
```

- A line starting with `///` documents the declaration, struct field or enum variant directly below it. Consecutive `///` lines form one doc, written in Markdown; one space after `///` is dropped.
- A `///` line anywhere else is an error. `////` starts an ordinary comment.
- The language server shows a doc under the signature on hover, `low doc` prints it, and the website renders it on the package pages.

### Generics

<!-- prelude
struct Pair[A, B] {
    first: A
    second: B
}
-->
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

- Functions, structs and enums take type parameters in brackets after their name. Generic types are written with arguments: `Stack[i64]`, `geometry.Labelled[str]`.
- A constraint lists what a type parameter supports, joined with `+`:
  - `Eq` allows `==` and `!=`. It is met by numbers, `bool`, `str`, pointers, channels, and structs and enums whose fields are all `Eq`.
  - `Ord` adds `<`, `<=`, `>` and `>=`. It is met by numbers and `str`.
  - `Hash` adds use as a map key. It is met by integers, `bool`, `str` and pointers.
  - `Number` adds arithmetic, numeric literals, conversion with `T(x)` and the `f`, `e` and `n` formats and pictures of interpolation.
  - `Integer` adds bitwise operators and shifts and the `d`, `x`, `X` and `b` formats, and implies `Hash`.
  - `Float` is met by `f64` and `f32`.
  - An unconstrained parameter can only be moved, stored, passed, printed and interpolated, with a width but no format, each instance rendering its own type. Anything else a generic function needs, such as an ordering of its elements, is passed in as a function value.
- A generic body is checked once against its constraints, so a missing constraint is reported at the definition, not at a use.
- Each generic function's type parameters are its own. Two functions may both name a parameter `T`, and a function type mentioning one is distinct from the same text mentioning the other.
- Type arguments are inferred from the arguments, then from the expected type. They can be written explicitly: `max[u8](a, b)`, `Stack[str] { ... }`.
- A generic struct literal or enum variant takes its arguments from the expected type or from its field values. A generic function used as a value takes them from the expected function type.
- Each distinct set of type arguments compiles to its own copy, `max[i64]` and `max[f64]`, so generic code runs as fast as code written for one type.
- Values of different kinds that share behaviour, such as the elements of a UI tree, are structs of function values whose closures capture each kind's state.

### Concurrency

<!-- prelude
let fd: i32 = 0
-->
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

- `spawn f(args)` starts a green thread running the call. The function value and the arguments are evaluated before it starts. `mut` arguments are not allowed. A function that can fail must have its error handled inside the spawned call.
- `chan[T](n)` makes a channel holding up to `n` values. `chan[T]()` is unbuffered, so every send waits for a receiver. A channel is a reference: assigning or passing it shares it.
- `c <- v` sends, waiting while the buffer is full.
- `<-c` receives a `T?`, waiting while the channel is empty. It gives `none` once the channel is closed and drained.
- `close(c)` wakes every waiter. Sending on or closing a closed channel panics.
- `for v in c` receives until the channel is closed and drained.
- `select` waits on several operations at once and runs the arm of the first that can proceed. It is an expression like `match`.
  - Arms are `name = <-c`, `<-c`, `c <- v`, `after(ms)` and `else`.
  - Channel and value expressions are evaluated first, in order.
  - Ready arms are taken in order.
  - `else` runs when nothing is ready. Without `else` the green thread waits.
  - `after(ms)` fires once `ms` milliseconds pass.
- `sleep(ms)` pauses the green thread.
- `wait_readable(fd)` and `wait_writable(fd)` wait until a file descriptor is ready, using epoll on Linux. On Windows `fd` is a waitable handle and both wait until it is signalled. One green thread may wait on a descriptor at a time.
- A program ends when `main` returns, whatever other green threads are doing.
- When every green thread is waiting and nothing can wake one, the program panics with `every green thread is blocked forever`. A green thread that exhausts its stack panics with `stack overflow`.

## Builtins

Every module sees these names without an import. A function in the program named `pop`, `truncate`, `close`, `sleep`, `wait_readable` or `wait_writable` takes precedence over the builtin.

### `bytes`

`bytes(s: str) -> []u8` copies a string's bytes into a new slice.

### `close`

`close(c: chan[T])` closes a channel and wakes every waiter. Sending on or closing a closed channel panics.

### `cstr`

`cstr(s: str) -> ptr[u8]` is a NUL-terminated copy of a string, for passing to C.

### `delete`

`delete(mut m: [K: V], key: K)` removes a key from a map. Removing a missing key does nothing.

### `embed`

`embed(path: str) -> str` is the contents of a file, read at compile time. `path` is a string literal relative to the directory of the file that calls `embed`.

### `exp`

`exp(x: f64) -> f64` is e raised to the power `x`. It takes and returns `f32` too.

### `floor`

`floor(x: f64) -> f64` rounds `x` down to a whole number. It takes and returns `f32` too.

### `len`

`xs.len` is an `i64`: the number of elements in a slice, the number of entries in a map, or the number of bytes in a `str`.

### `load`

`load(p: ptr[T]) -> T` reads one `T` from C memory. A null pointer traps.

### `new`

`new(value: T) -> *T` copies a value onto the garbage-collected heap and returns a pointer to it. A droppable value cannot go on the heap.

### `pin`

`pin(value) -> ptr` returns the address of a pointer, slice, map or string's data and keeps it alive until `unpin`. Memory C keeps after a call must be pinned.

### `pop`

`pop(mut xs: []T) -> T?` removes the last element and returns it, or `none` when `xs` is empty. The capacity stays.

### `print`

`print(value)` writes a value of any type to standard output, followed by a newline. The text is the default rendering described under `### Strings`.

### `push`

`push(mut xs: []T, value: T)` appends `value`, growing the capacity to twice plus four when full.

### `sleep`

`sleep(ms)` pauses the green thread for `ms` milliseconds. `ms` is any integer type.

### `sqrt`

`sqrt(x: f64) -> f64` is the square root of `x`. It takes and returns `f32` too.

### `store`

`store(p: ptr[T], value: T)` writes one `T` to C memory.

### `truncate`

`truncate(mut xs: []T, n)` sets the length to `n` and keeps the capacity. `n` is any integer type. It panics unless `0 <= n <= xs.len`.

### `unpin`

`unpin(p: ptr)` releases an address returned by `pin`.

### `view`

`view(p: ptr[T], n) -> []T` is a slice over `n` elements at `p`, bounds-checked like any slice. `n` is any integer type.

### `wait_readable`

`wait_readable(fd)` waits until a file descriptor is ready to read, using epoll. On Windows `fd` is a waitable handle and the wait ends when it is signalled. `fd` is any integer type. One green thread may wait on a descriptor at a time.

### `wait_writable`

`wait_writable(fd)` waits until a file descriptor is ready to write, using epoll. On Windows `fd` is a waitable handle and the wait ends when it is signalled. `fd` is any integer type. One green thread may wait on a descriptor at a time.

### Types

Calling a numeric type converts a number to it, as in `u8(x)`, `f64(n)` and `i64(f)`.

| Type | Values |
|---|---|
| `i64` | Signed 64-bit integers. An integer literal with no expected type is an `i64`. |
| `i32` | Signed 32-bit integers. |
| `u64` | Unsigned 64-bit integers. |
| `u32` | Unsigned 32-bit integers. |
| `u16` | Unsigned 16-bit integers. |
| `u8` | Unsigned 8-bit integers, and the bytes of a `str`. |
| `f64` | 64-bit IEEE 754 floats. A float literal is an `f64` unless an `f32` is expected. |
| `f32` | 32-bit IEEE 754 floats. |
| `bool` | `true` and `false`. |
| `str` | An immutable pointer and length over UTF-8 bytes. `str(b)` copies a `[]u8` into a string, and `str(p)` copies a NUL-terminated `ptr[u8]`. |
| `ptr` | `ptr[T]` is a nullable pointer to C memory. `ptr` alone is `void*` and converts to and from any `ptr[T]`. |
| `chan` | `chan[T]` is a channel of `T` values. `chan[T](n)` makes one holding up to `n` values, and `chan[T]()` an unbuffered one. |
| `u8x16` | 16 lanes of `u8`, 128 bits. |
| `u8x32` | 32 lanes of `u8`, 256 bits. |
| `u16x8` | 8 lanes of `u16`, 128 bits. |
| `u16x16` | 16 lanes of `u16`, 256 bits. |
| `i32x4` | 4 lanes of `i32`, 128 bits. |
| `i32x8` | 8 lanes of `i32`, 256 bits. |
| `u32x4` | 4 lanes of `u32`, 128 bits. |
| `u32x8` | 8 lanes of `u32`, 256 bits. |
| `i64x2` | 2 lanes of `i64`, 128 bits. |
| `i64x4` | 4 lanes of `i64`, 256 bits. |
| `u64x2` | 2 lanes of `u64`, 128 bits. |
| `u64x4` | 4 lanes of `u64`, 256 bits. |
| `f32x4` | 4 lanes of `f32`, 128 bits. |
| `f32x8` | 8 lanes of `f32`, 256 bits. |
| `f64x2` | 2 lanes of `f64`, 128 bits. |
| `f64x4` | 4 lanes of `f64`, 256 bits. |

### Constants

| Constant | Value |
|---|---|
| `true` | The true `bool`. |
| `false` | The false `bool`. |
| `none` | The empty optional, typed by its context. |
| `null` | The null `ptr`, typed by its context. |

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

`import std/<name>` loads a standard library module. The library is compiled into `low`, and the project root is not searched for it.

Every module except `std/math` and `std/sql/sqlite` talks to the kernel through raw syscalls. `std/math` calls libm and `std/sql/sqlite` calls libsqlite3 through `extern` blocks. A library is linked only when reachable code calls it, so a program stays static unless it calls one of their C functions.

Every public struct, enum, function, struct field and enum variant carries a doc comment, and `TestStdDocumented` in `internal/compiler` fails on any that does not. A doc is written for a caller reading it on hover:

- The first sentence is the summary: a verb phrase for a function (`Returns the ...`, `Parses ...`), a noun phrase for a type, field or variant.
- Later paragraphs carry the contract the signature cannot: which error variant is returned and when, whether the call blocks, what dropping or closing releases, limits, defaults and edge cases such as empty input or end of input.
- Identifiers and literals are in backticks. A `low` example block appears only where usage is not obvious from the signature, and it type-checks.
- A doc states the current behaviour as read from the code.

| Module | Contents |
|---|---|
| `std/sys` | The operating system layer: Linux system calls, and kernel32, ws2_32 and the other Win32 libraries on Windows. It is the only standard library module besides the prelude that calls `__syscall`, and its Windows files call none. It owns every syscall number, flag, errno and structure layout and exposes typed wrappers that repeat an interrupted call and fail with an `io.Error`, so callers match `io.Error` variants. Descriptors: `FdHandle`, a copyable view whose closing is coordinated by an `io.Guard`, and the droppable `Fd` that owns one; `read`, `write`, `write_all`, `stream` and the `_on` forms on a view park the green thread while a non-blocking descriptor is not ready; `pipe`, `duplicate`, `set_nonblocking`, `try_read`, `try_write` and `wait_input`. Files: `open` with an `Open` mode, `stat`, `fstat`, `access`, `chmod`, `chown`, `set_times`, `unlink`, `rmdir`, `mkdir`, `rename`, `symlink`, `link`, `readlink`, `statfs`, `lock`, `seek`, `fsync`, `ftruncate`, `list_dir` and `random_bytes`. Processes: `fork_exec`, `reap`, `kill`, signal dispositions and handlers, identity, working directory, `arguments`, `environment`, `executable_path`, memory mapping and `exit_process`. Sockets: `socket`, `socketpair`, `bind`, `listen`, `accept`, `connect`, `finish_connect`, `recv`, `send`, `wait_socket`, `set_option`, `get_option`, `interface_index` and multicast membership. Terminals: `termios`, `raw_mode`, window size and `open_pty`. `inotify_init`, `inotify_add` and `inotify_remove`. |
| `std/io` | `Error`, the error type shared by the I/O, codec and protocol modules. `Stream`, a copyable struct of `read`, `write` and `close` closures that views a resource. `Guard` and `Lease` (`guard`, `enter`, `retire`), the shared state that makes a closed descriptor fail with `NotConnected` instead of reaching a reused descriptor. A buffered `Reader` over a stream or, with `bytes_reader`, over a buffer, with `read_line`, `read_exact`, `read_up_to`, `read_some`, `read_byte`, `buffered` and `fill`. `write_all`, `read_all`, `buffer(n)`, `copy`, and the fixed-width integer codec in both byte orders: reads (`be16`, `be24`, `be32`, `be64` and the `le` forms as `i64`, `u16_be`, `u32_be`, `u64_be` and the `_le` forms as unsigned types, `uint_be` and `uint_le` for any width from 1 to 8 bytes as `u64`, and `u16_le_str`, `u32_le_str` and `u64_le_str` over a `str`), appends (`put_be16` through `put_be64`, `put_le16` through `put_le64`, and `put_be` and `put_le` for any width up to 8 bytes) and in-place stores (`set_be16` through `set_be64`, `set_le16` through `set_le64`). |
| `std/file` | Opening: `open`, `create`, `append` and `update` return a droppable `File` whose `handle` is a copyable `FileHandle`. On a `File`: `read`, `write`, `write_all`, `seek`, `sync`, `truncate`, `close`, `info` and `stat`. On a `FileHandle`: `read_on`, `write_on`, `write_all_on`, `close_on` and `stream_on`. Whole files and directories: `exists`, `read_file`, `read_text`, `write_file`, `write_text`, `remove`, `remove_dir`, `rename`, `mkdir`, `mkdir_all`, `list` and `stream`. `read_file` sizes its buffer from the file's size; a file that reports no size, such as one under `/proc`, is read into a 64 KB buffer that doubles when full. Permission bits: `create_mode`, `write_file_mode`, `write_text_mode`, `mkdir_mode` and `mkdir_all_mode` take them, the other creators use `0o644` for files and `0o755` for directories, all are filtered by the process umask, and `chmod` is not. Locks: `try_lock` and `unlock` are advisory `flock` locks that report a held lock as `false` instead of waiting. `remove_all` removes a tree without following symbolic links. `temp_dir` creates a private directory under `os.temp_dir_path()`, which is `TMPDIR` when set and `/tmp` otherwise; `temp_file` and `temp_dir_pattern` use it when `dir` is empty and all three draw the random name suffix from `getrandom`. `child_path` joins a directory and an entry name. `Info` also carries `regular`, `link`, `accessed`, `uid`, `gid`, `device` and `inode`. Further functions: `lstat`, `is_dir`, `is_file`, `is_symlink`, `symlink`, `link`, `readlink`, `realpath`, `chown`, `lchown`, `set_times`, `disk_usage` and `read_dir_info`. `walk` is a sorted depth-first walk that uses `lstat`, never descends into symbolic links and lets its callback return `Continue`, `Skip` or `Stop`. `glob` matches each segment with `path.matches` and supports `**`. `copy` and `copy_dir` keep file modes and recreate symbolic links. `temp_file(dir, pattern)` creates a uniquely named file with `O_EXCL` and mode `0600` and returns a droppable `TempFile` that closes and removes it unless `keep` is called. `temp_dir_pattern` makes a `0700` directory the same way. |
| `std/file/watch` | inotify. `watch` and `watch_with` take paths and `Options` (`recursive`, `access`) and return a droppable `Watcher` with a copyable `WatcherHandle`. On a watcher: `add`, `remove`, `next`, `events` and `stop`. `next` parks the green thread until an `Event` (`watched`, `name`, `path`, `kind`, `dir`, `cookie`) arrives. `events` is a `chan[Event]` fed by a forwarding green thread. `Kind` is `Created`, `Modified`, `Deleted`, `MovedFrom`, `MovedTo`, `Attrib`, `Closed`, `ClosedUnmodified`, `Opened`, `Accessed`, `SelfDeleted`, `SelfMoved` or `Overflow`. |
| `std/net` | Addresses: IPv4 and IPv6 `Addr`, `parse_ip`, `format_ip`, `parse_addr`, `format`, `in_network` (tests an address against a CIDR prefix) and `is_global` (false for addresses the IANA special-purpose registries mark as not globally reachable, and for multicast). Types: the droppable `Listener`, `Conn` and `Socket`, each with a copyable `ListenerHandle`, `ConnHandle` or `SocketHandle` in its `handle` field; `adopt` and `detach` convert between a `ConnHandle` and a `Conn`. TCP: `listen`, `accept`, `accept_handle` (returns the connection's view for a green thread to adopt), `connect`, `dial` and their `_within` forms with a connect timeout, `connect_through` (binds the socket to a network interface first), `connect_first` (tries a list of addresses in order through an optional interface) and `interface_index`. Connection I/O: `read`, `write`, `write_all`, `close`, `shutdown_write`, `set_nodelay`, read and write timeouts and `stream`, with `read_on`, `write_on`, `write_all_on`, `close_on` and `stream_on` on the view. UDP: `bind_udp`, `send_to`, `receive_from` and `close_socket`. `lookup` tries an IP literal, then `/etc/hosts`, then A and AAAA queries to each `/etc/resolv.conf` nameserver. An `Addr` with an empty `ip` is a Unix address: `listen_unix`, `connect_unix`, `connect_unix_within` and `socketpair` return the same `Listener` and `Conn`, abstract names start with `@` or NUL, and a listener unlinks its pathname when closed. `peer_credentials` returns the peer's credentials. `listen_with` and `bind_udp_with` take `ListenOptions` (`reuse_addr`, `reuse_port`, `backlog`). Socket options: `set_keepalive` and `set_buffer_sizes`. UDP multicast: `join_group`, `leave_group`, `set_multicast_ttl`, `set_multicast_loop` and `set_multicast_interface`. `Dialer` is a function value that makes a connection; `direct_dialer` is the plain one. |
| `std/socks` | SOCKS5 (RFC 1928, with RFC 1929 username and password) and SOCKS4a clients: `dial`, `dial4a`, `tunnel` and `tunnel4a` over an existing connection, and `dialer` and `dialer4a`, which return a `net.Dialer`. The proxy resolves the target name. |
| `std/pipe` | Pipelines. A `Pipe` owns a `Reader` and a `Writer`, each droppable so that releasing it completes it, with copyable `ReaderHandle` and `WriterHandle` views for green threads (`adopt_reader`, `adopt_writer` and the `_on` operations). Also segmented buffers, backpressure, `Sequence` and `SequenceReader`, and adapters between pipes and streams. |
| `std/cancel` | `Token` with `token`, `child`, `cancel`, `cancelled`, `check` and `done` (a channel that closes on cancellation, for `select`). `on_cancel` returns a droppable `Registration` that releases its callback; `release`, `on_cancel_handle` and `release_on` work on the copyable `RegistrationHandle`. `pause` is a sleep that ends early on cancellation. |
| `std/header` | The header primitives below `std/http`, `std/multipart` and `std/mail`: `Header`, `get`, `has`, `add`, `set`, `token`, `token_byte`, `valid_value`, `valid`, `quote`, and `parse_value` with `param`, which resolve RFC 2231 and 5987 parameters. |
| `std/http` | Server types: `Request`, `Response`, header helpers over `header.Header` and `parse_url`. `Request.token` is cancelled when the client goes away and when the handler returns. `Request.remote` is the client's IP address for connections served by `serve`, `serve_tls` and `serve_conn`. Serving: `serve`, `serve_tls`, `serve_conn` and `serve_stream`; the `_on` forms (`serve_on`, `serve_tls_on`, `serve_conn_on`) take the listener's or connection's view. Client: a pooled client configured through `client()` and `with_*` builders, per-request builders, `send`, `stream_to` and `download`. `with_interface` binds every connection to a network interface. `with_resolver` substitutes for `net.lookup` on host names; TLS keeps the host name for SNI and certificate checks. `with_cancel` makes a request fail with `Io(Aborted)` once its token is cancelled. URLs: `parse_url`, `resolve` and `with_query_pair` build on `std/uri`. `connect` returns a droppable `Connection` that closes its stream when released; `detach` hands the stream on. Proxies: `with_proxy`, `with_http_proxy`, `with_https_proxy`, `with_no_proxy`, `with_proxy_from_environment` and `with_dialer` send requests through an `http://` proxy (absolute-form for `http` targets, `CONNECT` for `https` targets) or a `socks5://`, `socks5h://`, `socks4://` or `socks4a://` proxy. `no_proxy` follows Go's matching rules. `accepts_coding`, `negotiate_coding` and `encode_body` are the one content-coding table, which the client's decompression and `std/web` compression and static files share. `Cookie` is the one cookie model: the jar stores it, `request_cookies` reads the `cookie` headers into it and `set_cookie` and `format_set_cookie` write it, and `std/web` sessions use it. `CookieJar` is a shared RFC 6265 jar (`cookie_jar`, `with_cookie_jar`, `jar_set`, `jar_header`, `jar_cookies`, `jar_clear`) that enforces the `Secure` rules and the `__Secure-` and `__Host-` prefixes and has no public-suffix check. A `Response` whose `stream` is a `chan[[]u8]` is written as it is produced, chunked on HTTP/1.1 and as DATA frames on HTTP/2, until the handler closes the channel. |
| `std/http/websocket` | `accept` (the `101` goes through `http.upgrade`) and `dial` over `ws` and `wss` return a droppable `Socket` that sends a normal close when released. Operations: `receive`, `send_text`, `send_binary`, `ping` and `close`, with `_on` forms, and `adopt` and `detach` on the `*SocketHandle`. |
| `std/http/sse` | Server-sent events. Shared: `Event` (`id`, `event`, `data`, `retry`), `format`, `comment` and `keepalive`. Server: `stream`, `stream_with`, `serve_events`, `serve_events_with`, `Emitter` and `last_event_id`. Client: `parser`, `feed`, `parse` and `listen`, which reads a `text/event-stream` response with any chunking and CR, LF or CRLF line ends. |
| `std/http/files` | `serve_file` and `serve_file_with` answer a request from a file. Content type comes from the extension, then sniffing. Weak `ETag` and `Last-Modified` work with `If-None-Match` and `If-Modified-Since`. A single `Range` is answered as 206 or 416, with `If-Range`. `HEAD` is supported and the body is streamed. |
| `std/web` | A router with groups, middleware and typed JSON endpoints, problem details (RFC 9457), an OpenAPI 3.1 document built from the types, cookie sessions, CSRF protection, API key, Basic and bearer authentication, CORS and response compression. `static` and `static_with` serve a directory. `events` and `event_stream` serve server-sent events. |
| `std/tls` | TLS 1.2 and 1.3 clients and servers. `client`, `server` and `dial` return a droppable `Session` that sends `close_notify` and closes the stream beneath it when released. Its `handle` is the shared `*SessionHandle` holding `protocol`, `version`, `group`, `suite`, `cipher_suite` and `peer_certificates`. `client_handle` and `server_handle` return the view alone, and `read_on`, `write_on`, `close_on` and `stream_on` operate on it. |
| `std/serde` | The `Writer` and `Reader` protocols, `write[T]`, `read[T]` and `schema[T]` (which the compiler derives for every type), and `Field[T, F]`. The helpers the derived code calls (`in_range`, `integer_in`, `key_integer`, `key_unsigned`, `key_boolean`) and that readers share (`parse_integer`, `parse_unsigned`). `Tree`, `Node` and `Member` are the value shape that CBOR, MessagePack, TOML, YAML and template data share: `to_tree` builds one from any value, `tree_reader` reads a `T` from one, and `key_text` and `unsigned_tree` hold the key text and the `u64` range rule. |
| `std/json` | A reader and writer over UTF-8 bytes, the `Value` document model with member builders (`text_member`, `int_member`, `bool_member`, `base64url_member`, `object_member`, `array_member`) and typed getters (`field`, `field_text`, `field_int`, `text_of`, `int_of`), `equal` (compares numbers numerically and objects without regard to member order), and `serialize[T]` and `deserialize[T]` with naming policies. |
| `std/json/pointer` | RFC 6901 pointers: `escape`, `unescape`, `parse`, `format`, `get`, `has`, `set`, `add`, `replace`, `remove` and relative pointers. Edits return a new document. |
| `std/json/patch` | RFC 6902 `apply` (atomic, failing with the operation's index) and `diff`; RFC 7396 `merge_patch` and `merge_diff`. |
| `std/json/path` | RFC 9535 JSONPath: `compile`, `query`, `find`, `values` and `paths`, with slices, filters, `match`, `search`, `length`, `count` and `value`, normalised paths, and syntax errors at a byte offset. |
| `std/json/schema` | JSON Schema 2020-12. `compile` and `compile_with` check the schema, resolve `$ref`, `$anchor`, `$dynamicRef` and registered resources, and precompile patterns. `validate` returns `Violation`s with the instance pointer, the keyword pointer and a message. `unevaluatedProperties` and `unevaluatedItems` use annotations from the branches that passed. `format` annotates or asserts. |
| `std/cbor` | CBOR (RFC 8949): the `Value` model, a decoder with depth and length limits, a deterministic encoder, diagnostic notation, and `serialize[T]` and `deserialize[T]`. |
| `std/webauthn` | A WebAuthn relying party: registration and authentication options as JSON, registration and authentication verification for ES256, EdDSA and RS256 credentials, and COSE key parsing. |
| `std/sql` | The driver contract: `Connection`, the droppable owner that closes the driver connection when released; `ConnectionHandle`, the copyable view of closures that `std/orm` and closures use (`handle` makes it from an owner, `adopt` the other way); `Value`, `Rows`, `Error`, `Options` and `Dialect`. A `Dialect` is a record that owns everything that differs between databases: the OpenTelemetry system name, default port, placeholder prefix, identifier quotes, generated-key and paging syntax, column types, the conditional `CREATE TABLE`, the session lock statements and the write-transaction statement. `postgres()`, `sqlserver()` and `sqlite()` build the three; `std/orm` and `std/sql/migrate` read the one a connection carries. `rolled_back` rolls a transaction back after an error and reports the rollback's own error when it fails. |
| `std/sql/driver` | What the drivers share. A `Session` holds the busy and closed state, and `guarded` fails a call with `Busy` or `Closed` and abandons the connection on an error the driver calls fatal. `guard` applies that to every operation of a `ConnectionHandle`, `establish` runs the login under it, `collect`, `ignore_columns` and `ignore_row` are the row callbacks, `dial` and `port_of` resolve the dialect's default port, and `traced` wraps a handle with `otel.instrument_sql`. `connection` guards, traces and adopts a driver's handle, so `std/sql/postgres`, `std/sql/sqlserver` and `std/sql/sqlite` do not import `std/otel`. |
| `std/sql/postgres` | A PostgreSQL driver over protocol 3.0, plain or over TLS. |
| `std/sql/sqlserver` | A Microsoft SQL Server driver over TDS 7.4, plain or over TLS. |
| `std/sql/sqlite` | SQLite through libsqlite3, linked only when a program calls it. `open` and `open_with` (read-only, create, busy timeout). `exec` runs scripts. `query`, `each` and `execute` take `?` or `?N` parameters over cached prepared statements. Also `last_insert_id`, `begin`, `begin_immediate`, `commit` and `rollback`, and `backup` to a file through the online backup API. `close` and `close_on` are idempotent; a closed `Database` fails with `sql.Error.Closed`. `connection` and `connect` present a database as an `sql.Connection`: the one `connection` returns leaves closing to the `Database`, the one `connect` returns closes it. Calls block their OS thread. |
| `std/sql/pool` | A pool of connections made by an opener. `open` takes a `Config` (`max`, `max_idle`, `idle_timeout`, `max_lifetime`, `wait_timeout`, `ping_after`, `ping`, `clock`). `acquire` and `acquire_until` (with a cancel token) park in first-in first-out order while every connection is in use. A droppable `Lease` returns its connection on release. Also `with`, `discard`, `prune`, `stats` and `shutdown`. |
| `std/sql/migrate` | Ordered migrations recorded in a `schema_migrations` table with a SHA-256 checksum: `plan`, `with_table`, `status`, `up`, `up_to` and `down`, each migration in its own transaction under a lock. `from_text` and `load_dir` read `-- +up` and `-- +down` files named `NNN_name.sql`. |
| `std/toml` | TOML 1.0.0: a table tree, an encoder, and serde binding. |
| `std/yaml` | YAML 1.2.2: events, a node tree, an encoder, and serde binding. |
| `std/otel` | OpenTelemetry traces, metrics and logs with W3C propagation, OTLP/HTTP protobuf and JSON, OTLP/gRPC and console exporters, and the instrumentation used by `std/http`, `std/web`, `std/sql/driver` and `std/orm`. |
| `std/protobuf` | The protobuf wire format: an encoder with nested messages and packed fields, and a field decoder. |
| `std/xml` | XML 1.0 with namespaces: a pull reader, a tree, a writer, and serde binding. |
| `std/orm` | Typed queries over field references, row mapping, inserts, updates and deletes, tables created from types, and raw queries mapped onto structs, over an `sql.ConnectionHandle`. |
| `std/compress/deflate` | DEFLATE, zlib and gzip. |
| `std/compress/xz` | xz with LZMA2, and `.lzma`. |
| `std/compress/zstd` | Zstandard (RFC 8878). |
| `std/compress/lz4` | LZ4 frames. |
| `std/compress/brotli` | Brotli (RFC 7932). |
| `std/compress/bzip2` | `compress(data, level)`, `decompress(data, limit)`, a streaming `decode` and an `Encoder`. The decoder reads multiple streams and blocks and checks both CRCs. Randomised blocks fail with `Unsupported`. |
| `std/compress/snappy` | Raw blocks (`compress_block`, `decompress_block`, `decoded_length`) and the framing format with masked CRC-32C (`compress`, `decompress`, `decode`, `Encoder`). |
| `std/compress/codec` | The parts the compressors share: a growable `Buffer` that packs bits, a `BitReader`, two-level Huffman decode `Tables`, a length-limited Huffman `Builder`, hash-chain `Chain` and `walk`, `copy_match`, and LEB128 varints. |
| `std/hash` | One-shot and incremental forms of CRC-32, CRC-32C, Adler-32, CRC-64 and xxHash32/64. Also FNV-1 and FNV-1a in 32 and 64 bits; SipHash-2-4 and SipHash-1-3 with a 16-byte key in 64- and 128-bit forms; MurmurHash3 (`murmur3_32`, `murmur3_128`); wyhash final 4.2; `jump` consistent hashing; and `rendezvous` hashing. |
| `std/bits` | Word operations (`popcount`, `trailing_zeros` on the processor bit scan, `leading_zeros`, `bit_length`, `select_in_word` and the 32 and 64-bit rotations `rotl32`, `rotr32`, `rotl64` and `rotr64`) and dense bitsets. `std/hash`, `std/rand`, `std/text`, the JSON, XML and YAML readers and the compressors use these for rotations, bit counts and bit lengths. |
| `std/bits/roaring` | Roaring bitmaps with the portable serialization format. |
| `std/image` | Bitmaps, pixel operations and resizing, with PNG, JPEG, GIF, BMP, QOI and lossless WebP codecs. |
| `std/crypto` | `random`, which reads `getrandom` and panics when the generator fails. |
| `std/crypto/hash` | `Algorithm` and cloneable `Digest`, SHA-1, SHA-256, SHA-384 and SHA-512, HMAC (`hmac`, and `hmac_keyed` for the keyed inner and outer digests that PBKDF2 clones), HKDF and constant-time `equal`. |
| `std/crypto/sha3` | SHA3-256, SHA3-384, SHA3-512, SHAKE128 and SHAKE256. |
| `std/crypto/aead` | `Aead` values for AES-128-GCM, AES-256-GCM, ChaCha20-Poly1305 and XChaCha20-Poly1305 (`xchacha20_poly1305`, 24-byte nonce). The AES block functions for 128-, 192- and 256-bit keys (`aes_key`, `encrypt_block_into`, `decrypt_block`, `decrypt_block_into`). The ChaCha20 functions `chacha20_xor` (takes the RFC 8439 block counter) and `hchacha20`. |
| `std/crypto/md5` | MD5 as a `hash.Algorithm`, so HMAC, HKDF and PBKDF2 work over it, and `sum`. For interop and checksums, not security. |
| `std/crypto/cipher` | AES in CBC, raw (`cbc_encrypt`, `cbc_decrypt`) or with PKCS#7 padding (`cbc_seal`, `cbc_open`, `pkcs7_pad`, `pkcs7_unpad`); in CTR with a 128-bit big-endian counter (`ctr`); and in XTS with ciphertext stealing (`xts`, `xts_encrypt`, `xts_decrypt`). There is no ECB, and CBC provides no integrity: use an `Aead`. |
| `std/crypto/x25519`, `std/crypto/ed25519` | X25519 key agreement (RFC 7748) and Ed25519 signatures (RFC 8032), over `std/crypto/curve25519`. |
| `std/crypto/ecdsa` | ECDH and ECDSA on P-256 and P-384, with RFC 6979 nonces and SEC1 private keys. It owns the curve sizes (`scalar_size`), the named-curve lookup (`curve_by_oid`) and the uncompressed point encoding (`uncompressed`, `coordinates`) that `std/crypto/x509`, `std/crypto/jose` and `std/webauthn` use. |
| `std/crypto/rsa` | RSA PKCS#1 v1.5 and PSS signing and verification, and PKCS#1 keys. `integer` strips the leading zeros that key integers from JWK and COSE carry. |
| `std/crypto/blake3` | BLAKE3: `sum`, `xof`, `keyed_hash`, `derive_key`, an incremental `Hasher` with `finalize` and seekable `finalize_at`, and `algorithm()`, a `hash.Algorithm` for HMAC. |
| `std/crypto/otp` | HOTP (RFC 4226) and TOTP (RFC 6238) over SHA-1, SHA-256 and SHA-512: `hotp`, `totp`, `verify_totp` with a window, `verify_hotp`, `generate_secret`, base32 secrets, and `otpauth://` `uri` and `parse_uri`. |
| `std/crypto/mlkem` | ML-KEM-512, -768 and -1024 (FIPS 203). |
| `std/crypto/jose` | JSON Web Tokens: compact JWS with HS256/384/512, RS256/384/512, PS256/384/512, ES256, ES384 and EdDSA, JWK sets, and claim checks. `asymmetric_algorithms` lists the public key algorithms and `token_kid` reads the key id from a token header. |
| `std/crypto/password` | Password hashes in PHC strings: Argon2id, scrypt (RFC 7914, with a memory limit checked before allocating) and PBKDF2. bcrypt (`$2b$`; passwords over 72 bytes are rejected). `verify` handles all of them and `$2a$` and `$2y$` bcrypt strings. BLAKE2b. |
| `std/crypto/der` | A DER cursor and the ASN.1 values the other crypto modules share. Reading: `reader`, `read`, `peek`, `done`, `expect`, `enter`, `finish`, `whole`, with `unsigned`, `small_int`, `boolean`, `bits`, `named_bits`, `oid` and `time` for the primitive types; it rejects indefinite and non-minimal lengths. Writing: `encode`, `integer` and `sequence`. `prime256v1` and `secp384r1` return the named-curve OIDs that `std/crypto/ecdsa` and `std/crypto/x509` share. `std/crypto/x509`, `std/crypto/rsa` and `std/crypto/ecdsa` build on it. |
| `std/crypto/x509` | PEM (`pem_blocks`, `pem_certificates`, `pem_private_key`), certificates, PKCS#8 private keys, the system roots, chain validation and the TLS signature scheme table (`signature_schemes`, `signing_schemes`, `authentication`, `verify_signature`, `sign`). |
| `std/time` | `now` (Unix seconds), `now_millis` and `now_nanos` (the wall clock at finer resolution), `monotonic` (nanoseconds), `sleep_for` (rounds up to whole milliseconds), `after`, `Timer` and `Ticker` (their channels work in `select`), and `Stopwatch`. Calendar and zone types live in the submodules below, one module per type, so `d.plus(span.days(3))` resolves to `date.plus`. |
| `std/time/fault` | `Fault` (`OutOfRange`, `Invalid`, `Syntax(at)`, `Overflow`, `UnknownZone`, `Malformed`, `Unsupported`, `Skipped`, `Ambiguous`, `OffsetConflict`), the error every fallible time function returns, and `to_string`. |
| `std/time/duration` | `Duration`, an exact signed elapsed time with private seconds and nanoseconds. Constructors from `nanoseconds` to `hours`, and `from_seconds_f64`. Arithmetic: `plus`, `minus`, `times`, `divide`, `ratio`, `negate`, `abs`, with `try_` forms returning an optional. Plain forms panic on overflow. Also `round` and `truncate` with a `Rounding`, comparison, `to_string` (ISO 8601 `PT1H30M`), `friendly` (`1h 30m`) and `parse`, which reads both forms and fails at the offset of the first bad character. |
| `std/time/timestamp` | `Timestamp`, a point on the UTC timeline from 0001-01-01 to 9999-12-31 with nanosecond precision: `now`, `from_unix` in seconds, milliseconds or nanoseconds, `plus` and `minus` by `Duration`, `until` and `since`, `round`, `truncate`, RFC 3339 `to_string` and `parse` (offset required, `:60` read as `:59`), and `to_http_date` and `parse_http_date` (IMF-fixdate, RFC 850 and asctime). |
| `std/time/clock` | `Clock`, a pair of closures for the wall and monotonic readings, with `system`, `fixed` and `manual`. A `Manual` clock moves with `advance` and `set`, so code that asks for the time is tested without sleeping. |
| `std/time/span` | `Span`, a calendar amount of years, months, weeks, days and time units with one sign. `Unit`, `Diff` (`largest`, `smallest`, `increment`, `mode`), `to_duration` and `from_duration`, ISO 8601 `to_string` (`P1Y2M3DT4H`), `friendly` and `parse`. |
| `std/time/rounding` | `Rounding` (`Trunc`, `Ceil`, `Floor`, `Expand` and the five `Half` modes) and the helpers the other time modules share. |
| `std/time/date` | `Date` with private fields and validated constructors (`of`, `clamped`, `from_epoch_days`, `from_ordinal`, `from_iso_week`, `from_weekday_in_month`). `Month`, `Weekday` and `IsoWeek`. `plus` and `minus` by `Span` with month-end clamping, `until` and `since` with a `Diff`, `next_weekday` and `on_or_next`, and `range`. ISO 8601 `to_string` and `parse` (calendar, ordinal and week dates). `format` and `parse_with` over a pattern. |
| `std/time/timeofday` | `TimeOfDay` to the nanosecond. `plus` fails past midnight and `plus_wrapping` wraps. Also `until`, `round`, `truncate`, `to_string`, `parse`, and pattern `format` and `parse_with`. |
| `std/time/datetime` | `DateTime`, a civil date and time of day with no zone: `plus` by `Span` or `Duration`, `until` and `since` with a `Diff`, `round`, `truncate`, `to_string`, `parse` (an offset is rejected), and pattern `format` and `parse_with`. |
| `std/time/offset` | `Offset`, a UTC offset in whole seconds within ±25:59:59, with `utc`, `hours`, `hm`, `to_string`, `to_string_z` and `parse`. |
| `std/time/zone` | IANA zones. `Zone` is a shared value decoded once, from `named` (the host database, honouring `ZONEINFO`), `load_from` (any `fn(str) -> []u8?`), `local` (`TZ`, `/etc/localtime`, `/etc/timezone`, then UTC), `from_posix`, `decode`, `utc` or `fixed`. Queries: `info_at`, `candidates` (`One`, `Overlap`, `Gap` for a civil time), `next_transition`, `previous_transition` and `transitions`. |
| `std/time/tzdata` | An embedded copy of the database for hosts without one: `source` (for `zone.load_from`), `names` and `version`. The data is linked only when `source` or `names` is reachable. |
| `std/time/zoned` | `Zoned`, a `Timestamp` seen through a `Zone` whose offset is always the zone's own. Built from a civil time with a `Disambiguation` (`Compatible`, `Earlier`, `Later`, `Reject`) or `strict`. `plus` by `Span` moves calendar units on civil dates and time units by elapsed time; `plus_duration` is exact. Also `until` and `since` with a `Diff`, `start_of_day`, `hours_in_day`, `series` and `round`. RFC 9557 `to_string` (`2026-11-11T10:00:00+00:00[Europe/London]`), `to_rfc3339`, `to_rfc2822`, and `parse` with an `OffsetPolicy` for a written offset that disagrees with the zone. Pattern `format` and `parse_with`. |
| `std/time/pattern` | Compiled strftime and strptime patterns shared by the types above: `compile`, `Names` for other languages, `render` and `scan`. Verbs: `%Y %y %C %m %d %e %j %b %B %a %A %u %w %U %W %V %G %H %I %M %S %f %N %p %P %z %:z %::z %Z %s %F %T %R %D %% %n %t`. Flags: `-`, `_`, `0` and `^`. Parsing is strict, and every error carries a byte offset. |
| `std/time/humanize` | `ago`, `relative`, `in_words`, `compact` and `round_to` over a `Words` value; English by default. |
| `std/simd` | Vector reads and writes, `splat`, `mask`, `blend`, `min`, `max`, lane access, byte shuffles and `find_byte`. |
| `std/text` | Parsing and formatting: integer parsing in any base from 2 to 36; correctly rounded float parsing, which allocates only to expand a decimal when digits beyond the 19th decide the rounding; `parse_bool`, `radix`, `hex`, hex encoding and `hex_digit`, the value of one hex digit byte; `fixed(x, places)`, which rounds half to even on the exact binary value. Trimming and splitting: `trim` and its start, end, prefix and suffix forms; `split` on a byte; `split_str` on a string (an empty separator splits into code points); `fields`; `lines` (drops a final empty line and `\r` before `\n`); `chars`; `join`. Searching: `index`, `last_index`, `contains`, `count`, `starts_with`, `ends_with`. Rewriting: `repeat`, `replace`, ASCII `upper` and `lower` (return their argument when no byte changes), and case-insensitive `equal_fold` and `starts_with_fold` (allocate nothing). Widths: `char_count`, `from_char`, and `pad_left` and `pad_right` to a width in code points. `append` and `append_char` onto a `[]u8`; `char_len` and `put_char` encode one code point into a buffer, and are the only UTF-8 encoder. Comma-separated token lists. `cut` and `last_cut` return a `Cut` of `before`, `after` and `found`. `strip_prefix` and `strip_suffix` return `str?`. Also `split_n`, `title`, `quote` and `unquote` (backslash, `\x`, `\u` and `\U` escapes, failing with `QuoteError`), `edit_distance` over code points, `wrap` to a width, `is_ascii` and `reverse`. `parse_u64` and `parse_uint` parse unsigned digits, `digit_char` writes one digit, and `position(s, at)` turns a byte offset into a `Position` of line and column counted from 1. `float32_bits` and `float32_from_bits` convert between an `f32` and its bits, `float_decimal` writes a float in the decimal form that TOML and YAML share, `float_diagnostic` in the form of CBOR's diagnostic notation, and `quote_ascii` writes a string as a double-quoted ASCII literal. |
| `std/text/diff` | Myers line diff: `diff`, `lines`, `stats`, `distance`, `similarity`, GNU-compatible `unified` and `hunks`, `patch` and `patch_within` over a unified diff, and word and character level `inline`, `inline_chars` and `inline_markup`. |
| `std/text/table` | `Table` with per-column `Align`, width limits, truncation and wrapping, and `Border` styles `None`, `Ascii`, `Unicode`, `Markdown` and `Tabs`. Widths use `unicode.text_width` and ignore ANSI sequences. `align` and `align_tabbed` align tab-separated cells like Go's `tabwriter`. |
| `std/text/naming` | `words`, `snake`, `kebab`, `dotted`, `screaming_snake`, `camel`, `pascal`, `title`, `sentence`, the matching `is_` predicates, and `slug` and `slug_with` (transliterate Latin letters and cut at a word boundary). |
| `std/text/number` | `format_bytes` and `parse_bytes` in decimal and binary units, `grouped`, `ordinal`, `percent`, `compact`, `si`, English `words`, and `roman` with `parse_roman`. Arithmetic is exact. |
| `std/uri` | RFC 3986 percent coding and references. `escape` leaves the unreserved bytes, `escape_form` writes a space as `+`, `escape_target` encodes only the bytes a request target cannot hold and keeps existing escapes, and `escape_where` takes the set to keep. `unescape` returns `str?` and is `none` on a bad escape; `unescape_lossy` leaves a bad escape in place and `unescape_form_lossy` also reads `+` as a space. `encode_form` writes a list of `Field`s as `application/x-www-form-urlencoded`, and `parse_query` reads one into a `[str: str]` where the last value of a name wins. `scheme_end`, `without_fragment`, `remove_dots` and `resolve` follow RFC 3986 section 5, working on strings. |
| `std/builder` | A `Builder` over a growing byte buffer, appended in place through `mut`: `write`, `write_bytes`, `write_byte`, `write_char`, `write_i64`, `write_f64`, `write_line`, `write_repeat`, `write_join`, `len`, `capacity`, `reserve`, `truncate`, `reset`, `to_str` and `to_bytes`. |
| `std/regex` | Linear-time regular expressions in the RE2 style: `compile` and `compile_with` with `Flags`, `generated` (a pattern compiled into specialised code at build time), `is_match`, `find`, `full_match`, `find_all`, `replace_all` (with a closure receiving each `Match`), `split` and `split_n`. A match offers `start`, `end`, `text`, `group` and `span`, with byte offsets. |
| `std/unicode` | Normalization (Unicode 16.0.0): `nfd`, `nfkd`, `nfc` and `nfkc`; `combining_class`; full case folding with `casefold`; `search_key`, which case-folds, applies NFKD and drops combining marks in one pass. Properties: `category`, `is_letter`, `is_upper`, `is_lower`, `is_digit`, `is_space`, `is_punct`, `is_identifier_start` and the other predicates; `east_asian_width`, `display_width` and `text_width`; `script`. Case mapping: `to_upper`, `to_lower` and `to_title`, and `upper` and `lower` with SpecialCasing but without final-sigma or locale rules. Segmentation: UAX #29 `graphemes`, `grapheme_count` and `grapheme_end`. UTF-8 helpers: `decode_rune`, `encode_rune`, `utf8_valid`, `utf8_to_valid`. Category, width and identifier data are Unicode 16.0.0; script, grapheme break and case data come from 17.0.0 restricted to characters assigned in 16.0.0. The tables are generated by `cmd/tablegen` and linked only when a function that reads them is reachable. |
| `std/unicode/utf16`, `std/unicode/utf32` | Encoding and decoding of code points, `[]u16` and `[]u32`, and byte forms in either order with BOM handling, strict or lossy, with surrogate validation. `join` combines a surrogate pair and `units` reads a byte form into code units. |
| `std/charset` | `decode`, `decode_lossy`, `encode`, `encode_replace`, `canonical` (WHATWG labels) and `detect_bom` for UTF-8, UTF-16, UTF-32, ISO-8859-1, -2 and -15, windows-1250, -1251 and -1252, KOI8-R, IBM437 and IBM850, and `windows_1252` for the code point of one windows-1252 byte. The CJK encodings are not provided. |
| `std/html` | An HTML tokenizer: a pull `Tokenizer` returning `StartTag`, `EndTag`, `Text`, `Comment`, `Doctype` and `End` tokens, `attribute` lookup, `unescape` for character references and `escape` for text and attribute values. |
| `std/math` | libm's trigonometric, hyperbolic, exponential and rounding functions, `pi`, `tau`, `e`, `infinity`, `nan`, `is_nan`, `is_inf`, generic `abs`, `min`, `max`, `clamp` and `compare`, and integer `ipow`, `gcd`, `floor_div` and `floor_mod`. |
| `std/slices` | Queries and transforms: `contains`, `index`, `clone`, `concat`, `reverse`, `map`, `filter`, `fold`, `any`, `all`, `sum`, `min` and `max` (as optionals), `sorted`. Sorting: `sort` and `sort_by` are unstable and in place, a quicksort with median-of-three pivots that recurses into the smaller side and finishes runs of 16 or fewer with insertion sort. `sort_stable` and `sort_stable_by` are a top-down merge sort; `sort_by_key` is stable. Also `reduce`, `count`, `find`, `find_index`, `first`, `last`, `equal`, `compare`, `starts_with`, `ends_with`, `unique` (adjacent duplicates), `flat_map`, `flatten`, `repeat`, `lower_bound`, `binary_search` (the first match) and `binary_search_by`. `take`, `skip`, `chunks` and `windows` share the source's elements. `partition` and `zip` work over `Pair[A, B]`. `fill`, `swap`, `rotate`, `insert` and `remove` are in place. Slices are not named types, so these are called with a pipe: `xs \|> slices.sort()`. |
| `std/maps` | `keys`, `values`, `sorted_keys`, `entries`, `from_pairs`, `clone`, `merge` (the right map wins), `filter`, `map_values`, `has`, `get_or`, `invert`, `count_by` and `group_by`. |
| `std/os` | Process state: `args` and `environment` (from the argument and environment vectors the program started with), `env` as `str?`, `umask` (sets the creation mask and returns the previous one), `pid` and `exit`. Queries: `getcwd`, `chdir`, `hostname`, `uid`, `gid`, `euid`, `egid`, `ppid`, `executable`, `uptime`, `cpu_count`, and `lookup_user` and `user_name` from `/etc/passwd`. `set_env` and `unset_env` are seen by `env`, `environment`, `process.find` and the environment `process.run` passes on. Directories: `home_dir`, `cache_dir`, `config_dir`, `data_dir` and `state_dir` (XDG variables when absolute, otherwise under `$HOME`), and `temp_dir_path`. |
| `std/os/signal` | `Signal` (`Hangup`, `Interrupt`, `Quit`, `IllegalInstruction`, `Trap`, `Abort`, `BusError`, `FloatingPoint`, `Kill`, `User1`, `SegmentationFault`, `User2`, `Pipe`, `Alarm`, `Terminate`, `Child`, `Continue`, `Stop`, `TerminalStop`, `TerminalInput`, `TerminalOutput`, `UrgentData`, `CpuLimit`, `FileSizeLimit`, `VirtualAlarm`, `ProfilingAlarm`, `WindowChange`, `IoReady`, `PowerFailure`, `BadSystemCall` and `Other(number)` for any other number), the one signal vocabulary of `std/process`, with `number`, `name`, `from_number` and `catchable`. `notify` returns a droppable `Watch` (copyable `WatchHandle`) whose `signals` channel delivers the caught signals, with `wait_for`, `wait_timeout`, `wait_any` and `stop`. Also `ignore`, `reset`, `send` and `raise`. `Kill` and `Stop` cannot be caught and fail with `Invalid`. |
| `std/process` | `Command` with arguments, an optional replacement environment, a working directory, `Inherit`, `Piped` or `Null` for each standard stream, and an optional new process group. `start` returns a droppable `Child` that kills and reaps an unwaited process and closes its pipes when released. `start_handle`, `adopt`, `wait`, `kill` (which takes a `std/os/signal` `Signal`; an `Exit` is `Exited(code)` or `Signaled(signal)` with the same type), and `wait_on`, `wait_exit_on` (waits without reaping, so the process group can still be signalled) and `kill_on` on the shared `ChildHandle`. `run` feeds input and collects both outputs. `find` searches `PATH`. `success`. |
| `std/cdp` | The Chrome DevTools Protocol. `launch` starts a headless Chromium with a disposable profile, connects to it and returns a droppable `Browser` that shuts it down when released. Its `handle` is the shared `*BrowserHandle`; `launch_handle`, `adopt`, `shutdown_on` and `open_page_on` take it. `call` sends a command to the browser or a session, and `events` delivers everything else. `open_page` opens a page in its own browser context whose every request, including those of cross-site frames and workers, passes through `Handlers`. Also `navigate` and `shutdown`. |
| `std/path` | Slash-separated path manipulation: `clean` (lexical, resolving `.` and `..`), `join`, `base`, `dir`, `ext`, `stem` and `is_absolute`. `matches` matches shell globs with `*`, `?`, `[...]` classes (ranges, `!` or `^` negation) and `\` escapes; no wildcard crosses a `/`. |
| `std/sync` | `Mutex` with `lock`, `try_lock`, `unlock` and `locked`. `WaitGroup` with `add`, `done` and `wait`. `Once` with `run`. `RwLock`, first in first out and writer-fair, with `read_lock`, `read_unlock`, `write_lock`, `write_unlock`, `try_read_lock`, `try_write_lock`, `reading` and `writing`. `Semaphore` with `acquire`, `try_acquire`, `release` and `available`. `Cond` with `wait_cond`, `signal` and `broadcast`. `Barrier` with `wait_barrier`. `OnceValue[T]`. `ErrorGroup[E]`, whose `go` blocks while `limit` tasks run and whose `join` returns the first error. All are built on channels, so a waiting green thread parks instead of spinning. |
| `std/retry` | `Policy` with attempts, base, multiplier, maximum delay, `Jitter` (`None`, `Full`, `Equal`, `Decorrelated`), a deadline, a seeded `Rng` and a cancel token. `delay`; a loop driver `start` with `next` (`reserve` then `commit`) and `at_least`; `run` over a `Step` (`Done`, `Retry`, `RetryAfter`, `Fatal`) giving an `Outcome`. |
| `std/ratelimit` | `Bucket` (token bucket in exact nanosecond units) with `allow`, `allow_n`, `reserve`, `wait` and `set_rate`. `Window`, a sliding log. `Keyed`, a bounded per-key set of buckets with idle eviction. All take a `clock.Clock`. |
| `std/singleflight` | `Group[K, V]` with `do`, `do_shared`, `forget` and `in_flight`, which run one call per key while the others wait for its result. `Expiring[V]`, a value refreshed at most once per interval. |
| `std/atomic` | `Int`, `Bool` and `Flag` built on compare-and-swap, with `load_int`, `store_int`, `add_int`, `swap_int`, `compare_and_swap_int` and `update_int`, and the same forms for `Bool`. Every load is a locked operation, so a loop cannot hoist it. `Value[T]` swaps a heap pointer under a lock: `load_value`, `store_value`, `swap_value`, `update_value`, `snapshot_value` and `compare_and_swap_value` (compares pointer identity). |
| `std/rand` | A seedable, non-cryptographic xoshiro256** generator: `seeded`, `rng` (all 256 bits of state from `std/crypto`), `next`, unbiased `below` and `between`, `float`, `chance`, `shuffle` and `pick`. `pcg32`, `pcg64` and `chacha` (seeded from 32 bytes; `chacha_rng` seeds from entropy) produce an `Rng` that every function accepts, and `next32` draws 32 bits. Distributions: `normal`, `exponential`, `gamma`, `beta`, `poisson`, `binomial`, `geometric`, `zipf`, `weighted_pick`, `cumulative` with `pick_cumulative`, `sample` and `permutation`. Use `std/crypto` for secrets. |
| `std/uuid` | Random v4 and time-ordered v7 UUIDs, `format`, `parse` (reports the offset of the first bad character), `version`, `timestamp` for v7, `equal` and `nil`. |
| `std/csv` | RFC 4180 records with quoted fields, doubled quotes, embedded newlines and CRLF or LF line ends. `parse` and `parse_with` a separator fail with the line of an unterminated or bare quote. `format`, `format_with` and `append_record` quote only fields that need it. |
| `std/dotenv` | `.env` files. `parse` and `read` return a map from `NAME=value` lines. They allow `export`, blank lines, `#` comments (inline after whitespace in unquoted values), single quotes taken literally, and double quotes with `\n`, `\t` and `\r` escapes that may span lines. Errors carry the line. |
| `std/flag` | Command-line flags from `Spec`s with help text, booleans and defaults: `--name value`, `--name=value`, single-dash forms and `--`. `get`, `enabled`, `number`, the remaining arguments, and `usage`. |
| `std/log` | Leveled line logging (`debug`, `info`, `warn`, `error`) to a descriptor (stderr by default) as `level prefix: message`, without timestamps so journald and other collectors add their own. `with_prefix` nests a prefix. A `Logger` also carries attributes (`str_attr`, `int_attr`, `float_attr`, `bool_attr`, `with_attrs`, and the `_with` level forms), a `Format` of `Text` (`key=value`, quoting what needs it) or `Json` (one object per line), a `Sink` (a descriptor, an `io.Stream`, a line closure or a `Record` handler), optional RFC 3339 timestamps, and a level from `parse_level` or the `LOG_LEVEL` variable. Each record is one `write`, so lines from green threads do not interleave. A failed write is passed to `Logger.on_error` (set with `with_error_handler`), which ignores it by default. |
| `std/term` | `is_tty`, `size` and `set_size`. `enter_raw` returns a droppable `Raw` that restores the terminal when released. `Key`, `read_key` (a lone escape resolves after 50 ms) and `parse_keys`. ANSI helpers: `fg`, `bg` with 16, 256 and RGB colours, `bold`, `dim`, `underline`, cursor movement, `clear_screen`, the alternate screen, `strip` and `visible_width`. `colour_enabled` honours `NO_COLOR`, `CLICOLOR_FORCE` and `TERM=dumb`. `open_pty`. |
| `std/mime` | Media types by file extension and back (`by_extension`, `extension`) for web, image, font, archive, audio, video and subtitle formats. `sniff` recognises content by its magic bytes or its leading text and falls back to plain text or `application/octet-stream`. |
| `std/semver` | Semantic Versioning 2.0.0: `parse` (with an optional `v`, failing at the offset of the first bad part), `format`, and `compare` and `less` with prerelease precedence, ignoring build metadata. |
| `std/container/heap` | A binary `Heap[T]` ordered by a `less` function: `insert`, `pop`, `peek` and `size`. |
| `std/container/lru` | A least-recently-used `Cache[K, V]` of fixed capacity: `get` (refreshes the entry), `put` (returns the evicted key), `contains`, `size` and `keys` from most to least recent. All run in constant time over index-linked entries. |
| `std/container/deque` | A ring-buffer `Deque[T]`: `push_front`, `push_back`, `pop_front`, `pop_back`, `front`, `back`, `at` and `size`, doubling when full. |
| `std/container/set` | `Set[T]` over a map, in insertion order: `set`, `from_slice`, `add` and `remove` (both return whether the set changed), `has`, `size`, `items`, `union`, `intersection`, `difference` and `is_subset`. |
| `std/container/ordered` | An AVL `Tree[K, V]` for ordered keys: `tree`, `put`, `get`, `remove`, `contains`, `size`, `height`, `is_valid`, `first`, `last`, `floor_entry`, `ceiling_entry`, `each`, `each_range`, `keys`, `values`, `entries` and the half-open `range(lo, hi)`. |
| `std/container/sorted` | `Sorted[T]`, an ordered set over `ordered.Tree`: `sorted`, `from_slice`, `add`, `remove`, `has`, `size`, `items`, `first`, `last`, `floor_item`, `ceiling_item` and `range`. |
| `std/archive` | The filesystem layer shared by `std/archive/tar`, `std/archive/cpio` and `std/archive/zip`, built on `std/file` and `std/sys`. Names are cleaned and rejected when absolute, containing `..` or NUL, or passing through a symbolic link below the destination. Files are created exclusively without following links. |
| `std/archive/tar` | ustar, GNU and pax archives. A streaming `Reader` (`next`, `read_body`, `read_entry`, `list`) with limits on header, entry and total size. A `Writer` (`add_file`, `add_dir`, `add_symlink`, `add_hardlink`, `finish`) that uses pax records for long or non-ASCII names. gzip through `read_gzip` and `write_gzip`. `extract`, `extract_gzip`, `create` and `create_gzip` over the filesystem strip setuid, setgid and sticky bits. |
| `std/archive/cpio` | newc, CRC and odc archives, detected per entry on read: a `Reader`, a `Writer` (`add_file`, `add_dir`, `add_symlink`, `add_links`, `finish`), and `extract` and `create` over the filesystem with the safe extraction of `std/archive`. Hard links store their data with the last link in newc and CRC. The binary format fails with `Unsupported`. |
| `std/archive/zip` | Zip archives read through the central directory, with zip64, stored and deflate entries, CRC-32 checks and limits on entries, sizes and compression ratio. Written with `add_file`, `add_dir`, `add_symlink` and `finish`, which emits zip64 records when a count or size needs them. `extract` works over the filesystem. Encrypted entries and other methods fail with `Unsupported`. |
| `std/testing` | `T`, passed to a test `fn(*T)`, with `log`, `error`, `fatal`, `skip`, `require` and `sub`. Assertions `equal`, `not_equal`, `equal_slice`, `is_true`, `is_false`, `is_some`, `is_none`, `contains`, `contains_item`, `close_to`, `unexpected`, `must` and `expect_error` render values of any type. `table` and `table_of`. `run` and `run_with` run each test on its own green thread with a timeout, filter with `-run`, print with `-v` and exit with status 1 on failure. `bench`, `bench_for` and `blackhole`. `golden` rewrites its file when `UPDATE_GOLDEN` is set. `check_property`, `check_shrunk` and `check_ints` print the seed and shrink integers. |
| `std/base32` | RFC 4648 base32 with the standard and extended-hex alphabets: `encode`, `encode_unpadded`, `decode` (case-insensitive, padding optional, failing at the first bad character or on an impossible length), `encode_hex` and `decode_hex`. |
| `std/base58` | The Bitcoin alphabet (`encode`, `decode`), the Flickr alphabet (`encode_flickr`, `decode_flickr`) and Base58Check (`encode_check`, `decode_check`), failing at the first bad character. |
| `std/ascii85` | `encode`, `encode_adobe` (with `<~ ~>`), `decode`, and Z85 (`encode_z85`, `decode_z85`). |
| `std/msgpack` | MessagePack: the `Value` model with timestamps and extensions, a decoder with depth and length limits that reports the offset of an error, the smallest-form `encode`, and `serialize[T]` and `deserialize[T]`. |
| `std/bson` | BSON documents as `[]Field` with ObjectId, datetime, binary, regex, int32, int64, double and Decimal128 (as bytes): `decode`, `encode`, `get`, `diagnostic`, `new_object_id` and `parse_object_id`. |
| `std/ini` | INI files with sections, `=` or `:`, `;` and `#` comments, quoted values, continuations and duplicate keys (last wins, or all with `collect`): `parse`, `get`, `get_all`, `sections`, `set`, `format`. |
| `std/properties` | Java `.properties`: `parse`, `get`, `set`, `format`, with `\u` escapes and surrogate pairs. |
| `std/big` | `Int` over 32-bit limbs. Parsing and printing in bases 2 to 36 (printing divides the limbs, packed into 64-bit words, by the largest power of the base below 2^64 with a precomputed reciprocal). `add`, `sub`, `mul` (Karatsuba), `divmod`, `divmod_floor`, `divmod_euclid`, `pow`, `modpow` (Montgomery for odd moduli), `mod_inverse`, `gcd`, `lcm`, `isqrt`. Shifts and bitwise operators with two's complement semantics. `is_probable_prime`, `next_prime`, `random_below`, `to_f64`, `from_bytes` and `to_bytes`. Division by zero is an error. Not constant time. |
| `std/big/rat` | Normalised rationals over `big.Int`: `ratio`, `parse` (`a/b`, decimals, exponents), the four operations, `inv`, `cmp`, `to_fixed`, `to_f64`. |
| `std/big/decimal` | A coefficient and an `i32` exponent: exact `add`, `sub` and `mul`, and `div` and `rescale` with `HalfEven`, `HalfUp`, `Down`, `Up`, `Floor` or `Ceiling` rounding to a target exponent. |
| `std/redis` | RESP2 and RESP3 over TCP or TLS: a droppable `Client` with a `ClientHandle`, `connect` with AUTH, SELECT and HELLO 3, `command`, typed helpers for strings, hashes, lists, sets, sorted sets and `publish`, pipelining, `transaction`, pub/sub through a channel usable in `select`, command timeouts, and limits on bulk size and nesting. |
| `std/smtp` | An SMTP client: `connect`, `connect_starttls` and `connect_tls`, EHLO extensions, AUTH PLAIN, LOGIN, XOAUTH2 and CRAM-MD5 (plaintext authentication is refused unless `insecure_auth`), `send` with per-recipient results, dot-stuffing, pipelining when advertised, and errors separating 4xx from 5xx. |
| `std/mail` | Message building: `Message` with text and HTML alternatives and attachments, RFC 2047 encoded words, quoted-printable, base64 wrapped at 76, RFC 5322 dates, and rejection of CR and LF in any header. |
| `std/template` | A text template engine over a `Data` model (`to_data[T]` through serde): `{{ expr }}`, `if`, `for`, `include`, `block` and `extends`, `let`, trim markers, filters (built in and registered), HTML autoescaping, depth and output limits, and errors with line and column. A `Set` is read-only while rendering, so green threads share it. |
| `std/multipart` | multipart parsing and writing (RFC 7578, RFC 2046): a streaming `Reader` with limits and a spill sink for large parts, RFC 2231 and 5987 parameters, `Writer` and `Form`; `std/http` reads and writes them with `multipart_reader`, `multipart_form`, `set_multipart` and `with_multipart`. |
| `std/base64` | One encoder, `encode_with(data, alphabet, padded)`, and one decoder, `decode_with(s, alphabet, padding)`, over an alphabet (`standard`, `url_safe`) and a `Padding` rule (`Required`, `Forbidden`, `Optional`). `encode`, `encode_unpadded` and `encode_url`, and `decode`, `decode_unpadded` and `decode_url`, fix the alphabet and padding. Every decoder rejects non-zero trailing bits and a byte outside the alphabet. |

### Resource ownership

Every standard library resource that must be disposed of is a droppable owner bound with `using`. `defer` is for work that is not disposal: unlocking a lock, signalling a channel, restoring ambient state and logging.

- An owner keeps the resource's name (`file.File`, `net.Conn`, `net.Listener`, `net.Socket`, `tls.Session`, `http.Connection`, `websocket.Socket`, `process.Child`, `cancel.Registration`, `pipe.Reader`, `pipe.Writer`, `sqlite.Database`, `sql.Connection`, `cdp.Browser`, `otel.Sdk`) and holds its view in a public `handle` field. Its `drop` closes through the handle.
- A view is named `<Owner>Handle`: `FileHandle`, `ConnHandle`, `ListenerHandle`, `SocketHandle`, `SessionHandle`, `ChildHandle`, `RegistrationHandle`, `ReaderHandle`, `WriterHandle`, `DatabaseHandle`, `ConnectionHandle`, `BrowserHandle` and `SdkHandle`. `SocketHandle` names the view in both `net` and `websocket`. A view is a copyable struct, or a pointer to the shared state when that state is mutable, so closures capture it, slices, channels, optionals and pointer-held structs store it, and `spawn` takes it. A view never closes anything on its own.
- Operations a caller uses directly take the owner and borrow it: `net.read(c, buf)`. Operations that run after ownership has moved elsewhere (green threads, `io.Stream` closures, pools, callbacks) take the view and end in `_on`: `net.read_on(h, buf)`, `net.stream_on(h)`, `net.close_on(h)`. A function that returns a view without an owner ends in `_handle`: `net.accept_handle`, `tls.client_handle`, `cdp.launch_handle`. `adopt(h)` makes an owner from a view. `detach(mut owner)` gives up an owner's responsibility so a function that opened a resource and wrapped it can return the wrapper.
- Closing is explicit and idempotent from the owner, the view and the stream, and the drop after an explicit close does nothing. The closed state lives in the shared view. Descriptor-based views share an `io.Guard`: every operation enters it, `retire` marks it closed and wakes blocked operations, and the last operation to leave closes the descriptors. A view used after its owner closed fails with `NotConnected` and never reaches a reused descriptor. The SQL drivers fail with `sql.Error.Closed` and cdp with `cdp.Error.Closed`.
- `io.Stream` is a view whose `close` closes everything beneath it. In `tls.client(net.stream(c), config)`, closing the session closes the connection, and the connection's own drop does nothing.
- A droppable cannot be stored in a pointer, slice, map, channel, optional or enum variant, captured by a closure or passed to `spawn`, so long-lived and shared state holds views. The HTTP client's idle connections, its HTTP/2 connections and the reaper's deadlines hold `ConnHandle` and `io.Stream` views and close them explicitly with the `_on` operations. A green thread that should own a resource receives its view and binds the owner itself: `http.serve` accepts with `net.accept_handle`, and each spawned connection thread runs `using c = net.adopt(h)`. Pump threads that only use a resource take its view and leave it to its owner. A scope that opens a resource for a wrapper it returns binds it with `using var` and calls `detach`, as `tls.dial` does.
- A `process.Child` that was not waited for is killed and reaped when released. A `pipe.Reader` or `pipe.Writer` completes when released, so a peer never waits on an abandoned end.

### Networking

- Sockets are non-blocking. An operation that would block waits with `sys.wait_socket`, so a green thread waiting on the network never holds an OS thread the collector must wait for. Every connection holds a duplicate descriptor for its write side, so one green thread can read while another writes.
- A timeout spawns a green thread that sleeps and shuts the socket down if the operation is still waiting. The waiting operation then fails with `TimedOut`.
- A connection counts the reads and writes in progress. Closing it shuts it down, which wakes any green thread still waiting while the descriptor stays registered. The last operation to finish closes the descriptors, so a descriptor number is never reused under a waiting operation. Reads and writes after `close` fail with `NotConnected`, and closing twice does nothing.
- `serve` runs each connection on its own green thread.
- Cancellation is cooperative. A token runs its callbacks and closes its channel once, so code stops blocking work by closing what it waits on: `net.close` wakes a waiting read or write, `process.kill` ends a child, and `select` takes `<-cancel.done(t)`.
- An HTTP/1.1 handler without an `Upgrade` header runs on the connection's green thread while one spawned watcher reads ahead into the request reader. End of input or a read error cancels the request's token, and the response is still written. Requests read ahead are served in order afterwards. An HTTP/2 stream's token is cancelled when the client resets it or the connection ends. Every request's token is cancelled once its handler returns.
- A client request with a token arms its deadline even without a timeout. Cancelling the token fires the deadline, which shuts down the connection the request is using and wakes the request from waiting on a pooled connection or an HTTP/2 stream.
- A server speaks HTTP/1.1 and HTTP/2 on the same port. A connection that opens with the HTTP/2 preface is served as HTTP/2; any other is HTTP/1.1 with keep-alive, pipelining, chunked bodies and `Expect: 100-continue`. Over TLS, ALPN offers `h2` and `http/1.1` and the same detection applies. The `h2c` upgrade from HTTP/1.1, deprecated by RFC 9113, is not offered.
- HTTP/2 covers framing, HPACK with its dynamic table and Huffman decoding, stream multiplexing, flow control in both directions, SETTINGS, PING, RST_STREAM, GOAWAY and CONTINUATION, and the request validation of RFC 9113 section 8. A reader green thread feeds frames to the connection, each request's handler runs on its own green thread, and the connection selects over frames and finished responses, so every write happens on one green thread. The encoder uses the static table and literals without Huffman coding. It passes h2spec except the test sending a garbage preface, which the shared port reads as an HTTP/1.1 request. A handler's `Response.trailers` are sent as a final HEADERS frame over HTTP/2; HTTP/1.1 responses carry none.
- A handler that returns status 101 owns the connection: the server writes nothing more and closes it once the handler returns. `http.upgrade(req, headers)` writes the `101` and returns the connection's reader, and `http.upgraded()` is the response returned after a WebSocket session.
- `Request.connection` is the buffered reader of an HTTP/1.1 connection. `websocket.accept` writes the handshake to it and reads frames from it. A WebSocket server requires masked client frames, answers pings, reassembles fragments, checks UTF-8 in text messages, limits messages to 16 MB and echoes a close frame.
- `http.client()` returns a `Builder[Client]` value whose `with_*` functions return updated copies. `build` makes a `*Client` that any number of green threads share. `c.request(method, url)` copies the client's settings into a `Builder[Response]`, so headers, authentication, timeouts, redirects, retries, version and decompression can be overridden per request. The type parameter only selects whether `build` or `send` applies.
- HTTP/1.1 client connections: a request takes an idle keep-alive connection from a pool keyed by scheme, host and port, dials a new one while the host is under its limit, or queues for one. A sweeper green thread expires idle connections. A pooled connection that fails before any response byte is retried once on a fresh one.
- HTTP/2 client connections: over TLS the client offers `h2` and `http/1.1` by ALPN and keeps one HTTP/2 connection per host. A reader green thread decodes frames into per-stream queues, request green threads wait on their stream, flow control credits windows as bodies are consumed, and SETTINGS, PING, RST_STREAM, GOAWAY and the peer's stream limit are honoured. `with_prior_knowledge(true)` speaks HTTP/2 to `http://` URLs without negotiation (h2c), keeping one connection per host as over TLS.
- One reaper green thread per client enforces whole-request deadlines by shutting down an expired HTTP/1.1 exchange. HTTP/2 waits select on the deadline and reset the stream.
- Redirects switch 301, 302 and 303 to GET without a body, keep the method and body on 307 and 308, resolve relative locations and drop `Authorization` and `Cookie` across origins.
- Responses are decoded from gzip, deflate (zlib or raw), Brotli and Zstandard as they stream, and chunked bodies are read and sent. An optional RFC 6265 cookie jar is shared by the client.
- Retries apply to connection failures and, for idempotent methods, to transport errors and 429, 502, 503 and 504, through `std/retry` with exponential backoff, where `Retry-After` raises the planned wait.
- Servers set `TCP_NODELAY` so separate HEADERS and DATA writes are not held back by delayed acknowledgements.
- `Response.trailers` holds the trailer fields of chunked HTTP/1.1 bodies and of HTTP/2 responses.
- Request and response bodies are limited to 64 MB.

### Processes

- `process.start` builds the path, arguments, environment and pipes before `fork`, so the child runs only straight-line syscalls: `setpgid` for a new group, `dup2` onto descriptors 0 to 2, `chdir` and `execve`. Every descriptor the standard library opens is close-on-exec, so the child inherits only its three streams. A child's pipes are streams over guarded descriptors, so closing one twice or using it after it closed fails with `NotConnected` and never reaches a reused descriptor.
- A failed `chdir` or `execve` writes its errno to a close-on-exec pipe, which the parent reads before returning. The read sees end of file once `execve` succeeds; otherwise the parent reaps the child and fails with that errno.
- The parent's pipe ends are non-blocking and wait through the scheduler, as sockets do; the child's ends stay blocking. `wait` waits for the process's pidfd to become readable, then reaps it with `waitid` and remembers the result. `kill` signals through the pidfd, or the whole group when the command asked for one.
- `signal.notify` installs a handler with `rt_sigaction`. The handler is a short block of machine code in a mapped page: it writes the signal number to a non-blocking pipe and returns through `rt_sigreturn`. The runtime gives every OS thread an alternate signal stack, so the handler runs on whichever thread receives the signal. A forwarding green thread reads the pipe with `wait_readable` and sends to the `signals` channel, which holds 16 and drops repeats while full. Another `notify` for the same signal replaces the handler. Dropping a `Watch` restores the saved dispositions and unmaps the page, except that a handler another `notify` has replaced is silenced and its page stays mapped. `raise` signals the calling thread, so its handler has run when `raise` returns; `send` is process-directed and unordered against other threads.
- `watch.next` reads every event an inotify read returns and queues them. With `recursive` set, new and moved-in directories are watched before the batch that reports them is returned. `stop` wakes a parked `next` by adding and removing a watch on `/`, which makes the descriptor readable. The kernel merges identical consecutive events it has not yet delivered.

### Resilience

- `retry.delay` is the capped exponential backoff and `jittered` applies the chosen jitter to it. `reserve` plans the wait and stops at the attempt limit, the deadline or a cancelled token; `commit` calls the observer, waits through the policy's sleeper or `sleep` and starts the next attempt; `next` is `reserve` then `commit`.
- A `Bucket` keeps its balance as an integer in units of one token over `per` nanoseconds, so refills are exact and a wait is the ceiling of the deficit over the rate. `reserve` consumes and returns the wait. `wait` sleeps that long and refunds if cancelled.
- `Group.do` runs the function outside its lock, removes the key and stores the result before waking the waiters. It matches the key by call identity, so a `forget` followed by a new call is not disturbed.

### Text

- `diff.unified` merges hunks whose gap is at most twice the context, omits a count of 1 in a hunk header and writes `\ No newline at end of file`, as GNU `diff -u` does. The edit script is minimal.
- `number.format_bytes` shows one decimal below 10 and rounds half to even on exact integers. `parse_bytes` reads `k` and `kb` as 1000 and `ki` and `kib` as 1024, and fails with `OutOfRange` on overflow or `TooPrecise` beyond 18 fraction digits. `si` rounds on the exact decimal expansion of the `f64`.
- `naming.words` splits at case, acronym and digit boundaries and keeps combining marks attached to their letter, so `HTTPServerID2` is `http server id 2`.

### Time

- Each concept is its own type: `Timestamp` is a point on the UTC timeline; `Date`, `TimeOfDay` and `DateTime` are civil values with no zone; `Zoned` is a timestamp seen through a zone; `Duration` is exact elapsed time; `Span` is a calendar amount. Types never convert silently.
- Fields are private and constructors validate, so a `Date` cannot hold February 30. Months are 1 to 12, and `Month` and `Weekday` are enums. Nothing rolls over: `date.of(2026, 2, 30)` fails with `Invalid`, and the clamping forms are named (`clamped`, `plus_months`).
- Calendar arithmetic and elapsed arithmetic are separate. A `Duration` moves a `Timestamp` or `Zoned` by elapsed time. A `Span` moves a `Date`, `DateTime` or `Zoned` by calendar units: a month after January 31 is the last day of February, and a day after a spring-forward midnight is the next civil day although it is 23 hours long. `until` and `since` take a `Diff`, add whole years, then months, to the start without passing the end, then count the rest.
- Only `zone.local` reads the host zone. A civil time in a daylight saving gap or overlap needs a `Disambiguation`. In an overlap, `Compatible` and `Earlier` pick the first occurrence and `Later` the second. In a gap, `Compatible` and `Later` move forward by the gap and `Earlier` back. `Reject` fails with `Skipped` or `Ambiguous`. `zoned.start_of_day` is the first instant of the civil day, correct where midnight does not exist.
- Zones come from TZif versions 1 to 4, using the 64-bit body from version 2, with the footer's POSIX TZ rule applying after the last transition. Before the first transition the first standard-time type applies. The rules cover `Jn`, zero-based `n` and `Mm.w.d` days and transition times from -167 to 167 hours, so negative daylight saving (Dublin, Casablanca), half-hour shifts (Lord Howe) and skipped days (Apia) work.
- Parsing is strict ISO 8601, RFC 3339 and RFC 9557: `T`, `t` or a space separates date and time, a fraction has 1 to 9 digits, `:60` reads as `:59`, and a civil type rejects an offset. A written offset that disagrees with a bracketed zone is `OffsetConflict` unless an `OffsetPolicy` says which wins. Every parse error is `Syntax(at)` with the byte offset of the first bad character, or a value error naming the field.
- The supported range is 0001-01-01 to 9999-12-31 for a `Timestamp` and years -9999 to 9999 for a `Date`. Leap seconds are not modelled. Plain arithmetic panics on overflow; the `try_` forms return an optional.
- Month and weekday names are English. `pattern.Names` and `humanize.Words` carry other languages.

### Browser automation

- `cdp.launch` finds `chromium`, `chromium-browser`, `google-chrome-stable` or `google-chrome` on `PATH` unless given an executable, and creates a profile with `file.temp_dir`. It starts the browser headless in its own process group with `--remote-debugging-port=0`, standard input and output discarded and standard error piped. A green thread reads standard error until the `DevTools listening on` line, which gives the WebSocket address, and keeps draining it so the browser never blocks on a full pipe. Another waits with `process.wait_exit_on` for the browser to end, so a browser that dies before the line fails the launch at once even while a helper it started, such as the crash reporter, keeps standard error open. `shutdown` closes the WebSocket, kills the process group, waits for that thread, reaps the browser and removes the profile.
- Commands carry an id and, for a session, its `sessionId`. A pump green thread routes replies to the waiting call and sends every other message to the events channel. Sends are serialised and a call waits at most 30 seconds. Once the socket closes, every waiting call fails with `Closed` and the events channel closes.
- `open_page` creates a browser context that is disposed on detach, denies downloads in it, and turns on auto-attach with `waitForDebuggerOnStart` and flat sessions, so every page, frame and worker target pauses on creation. The first page in the context is the opened page; any other page in it, such as a popup, is closed before it runs. Every other target in the context gets `Fetch.enable` for all URLs, `Network.enable`, auto-attach for its own children and, for pages, `Network.setBypassServiceWorker`, and only then resumes, so no request escapes interception. Each paused request is passed to `Handlers.request` on its own green thread and continued or failed with its verdict. Responses and failed loads are reported with their URL.
- Cancellation is the caller's: `let h = b.handle` and `cancel.on_cancel(t, fn() { cdp.shutdown_on(h) })` end everything the browser was doing.

### TLS

- TLS 1.3 (RFC 8446) and TLS 1.2 (RFC 5246), client and server. `Config.min_version` and `Config.max_version`, written with `tls12()` and `tls13()`, bound the versions and default to 1.2 through 1.3. A client offers its range through `supported_versions`, and a server answers with the highest version both support. `Session.handle.version` holds the result.
- Downgrade protection: a server that supports 1.3 ends a 1.2 ServerHello random with the RFC 8446 downgrade sentinel; a client that offered 1.3 aborts with `illegal_parameter` when it sees either sentinel; a server refuses `TLS_FALLBACK_SCSV` below its highest version with `inappropriate_fallback`.
- `Config.cipher_suites` lists suites in preference order, and a server takes the first of its own list that the client offers. `Session.handle.cipher_suite` holds the IANA code and `Session.handle.suite` the AEAD and hash.
- TLS 1.3 has TLS_AES_128_GCM_SHA256, TLS_CHACHA20_POLY1305_SHA256 and TLS_AES_256_GCM_SHA384. TLS 1.2 has ECDHE with ECDSA or RSA authentication and AES-128-GCM with SHA-256, ChaCha20-Poly1305 with SHA-256 or AES-256-GCM with SHA-384, AEAD only: GCM records carry the 64-bit sequence number as their explicit nonce, and ChaCha20-Poly1305 records use the RFC 7905 nonce.
- TLS 1.3 key exchange offers the hybrid post-quantum group X25519MLKEM768 first, then X25519, P-256 and P-384. A client sends key shares for X25519MLKEM768 and X25519. A server picks the first group of its own order that the client supports, answering with a HelloRetryRequest when the client sent no share for it, so a client that supports X25519MLKEM768 always gets it. TLS 1.2 uses the first of X25519, P-256 and P-384 in the server's order that the client lists, and a server with an ECDSA certificate also requires the client to list the certificate's curve (RFC 8422).
- Servers sign with ECDSA P-256 or P-384, RSA-PSS, RSA PKCS#1 v1.5 in TLS 1.2, or Ed25519, with keys loaded from PEM (`x509.pem_certificates`, `x509.pem_private_key`). `std/crypto/x509` owns the one table of signature scheme codes: the client offer, the codes a key signs with and the key exchange authentication of a code all come from it. TLS 1.2 ECDSA signature schemes name only the hash, so either curve signs with SHA-256 or SHA-384 (`x509.SignatureRules.Tls12`); in TLS 1.3 a code names its curve. Clients verify the chain against `Config.roots`, or the system roots when that is empty, and check the host name against the certificate's subject alternative names. They verify CertificateVerify or ServerKeyExchange with any offered scheme the certificate allows and, in TLS 1.2, the suite's authentication.
- TLS 1.2 requires the extended master secret (RFC 7627) on both sides. A client requires the secure renegotiation indication (RFC 5746), a server echoes it when the client sends it, and both refuse renegotiation with a `no_renegotiation` warning.
- ALPN, SNI, KeyUpdate in TLS 1.3 and `close_notify` are supported. Session tickets are ignored, so there is no resumption or 0-RTT. Servers do not request client certificates; a client answers a CertificateRequest with an empty Certificate.
- Records hold at most 2^14 bytes of plaintext, and a peer's ciphertext may add 256 bytes in TLS 1.3 or 2048 in TLS 1.2. A record or handshake error sends its fatal alert, such as `bad_record_mac`, `record_overflow`, `unexpected_message` or `decode_error`, and leaves the session unusable.
- A session's read and write sides are independent, with a write lock, so different green threads can read and write it.

### Derived serialization

`std/serde` defines a `Writer` and a `Reader`, structs of closures through which values are written and read. For every concrete type the compiler derives a function that writes a value to a `Writer`, a function that reads it from a `Reader`, and a `Schema` describing it:

- Booleans, integers, floats and strings map to their primitives; `[]u8` to binary; slices to sequences; maps to maps with keys written as text; optionals to null or the value; pointers to their target, a shared pointer in full each time. A pointer reached again while its target is being written is a cycle and panics.
- A struct is a map from field names to values, and every field takes part, private ones included. A reader opens it with `begin_map` and steps through it with `next_field`, which a format may spell differently from `next_key`, the step through the entries of a map. Reading skips unknown fields, uses a field's default when it is missing, treats a missing optional field as `none`, and fails with `Missing` otherwise.
- An enum variant without fields is its name. One with labelled fields is a single-key map from its name to a map of its fields, and one with positional fields maps its name to a sequence.
- A schema records each type's kind, name, fields with whether they are required, and variants. A struct or enum reached again through a pointer while it is being described becomes a `Ref` to its name.

A `Reader` reports `at` in an `Error` as its `position`, whose unit is the format's: a byte offset for JSON, a line for YAML, the count of values read for CBOR, MessagePack and TOML, a column index for `std/orm` and `0` for XML. CBOR, MessagePack, TOML, YAML and `std/template` build a `serde.Tree` from a value and read a value from one, so the variant shape, the text of a map key and the range of a `u64` are defined once, in `std/serde`.

`std/json`, `std/orm` and `std/web` are built on these. `T.field`, where `T` is a struct type, is a `serde.Field[T, F]` holding the field's name and index, so a function can take a field of a given struct with a given type.

### Data

- `json.Reader` tokenizes UTF-8 bytes like .NET's `Utf8JsonReader`, strictly per RFC 8259, with a depth limit of 64. It returns strings without copying unless they contain escapes. Numbers are parsed as integers or correctly rounded floats.
- `json.serialize_with` and `json.deserialize_with` take `Options` for indentation, naming (`Unchanged`, `CamelCase`, `SnakeCase`, `PascalCase`), maximum depth and `omit_none`. Naming applies to struct field names and to variant names in both directions, and not to the keys of a map; a variant name read back is matched in PascalCase. With `omit_none`, a field whose value is `none` is left out instead of written as `null`. Binary data is base64; non-finite floats are written as `null`.
- `std/cbor` decodes RFC 8949 items into `Value`s: every major type, indefinite-length strings, arrays and maps, tags, simple values, and half, single and double floats, which become `f64`. Indefinite-length items arrive as their definite equivalents. Text must be valid UTF-8 and map keys must be distinct. These fail with `Malformed`: truncated input, reserved additional information, a stray break, indefinite integers or tags, mixed or nested string chunks, a two-byte simple value below 32, data after the item, and a declared length that exceeds the remaining input (checked before anything is allocated). Nesting deeper than `Options.max_depth`, or a string, array or map longer than `max_length`, fails with `TooLarge`. `decode_first` returns the first item and its length so a caller can continue after it. `encode` writes the deterministic encoding of §4.2.1: shortest heads, definite lengths, map entries sorted by the bytes of their encoded keys, and floats in the shortest of half, single or double that keeps the value, with NaN as `f97e00`. `diagnostic` writes the notation of §8. `serialize` and `deserialize` bind types through serde with `[]u8` as byte strings, structs and variants as maps with text keys, and `none` as null; reading looks through tags and accepts integer map keys as their decimal text.
- `std/webauthn` produces options JSON in the shape py_webauthn's `options_to_json` writes, with base64url challenges, user ids and credential ids, `pubKeyCredParams` defaulting to `supported_algorithms()` of the COSE parser (EdDSA, ES256 and RS256), and attestation `none`.
  - `verify_registration` takes the credential JSON a browser's `PublicKeyCredential` gives in base64url, with or without padding. It checks that `id` is the base64url of `rawId`, that `type` is `public-key`, that clientDataJSON has type `webauthn.create`, the expected challenge (compared in constant time) and the origin. It then decodes the attestation object and checks the RP ID hash, user presence, user verification when required, and attested credential data. It parses the COSE key, accepts only the `none` format with an empty statement, and rejects a backup state without backup eligibility. It returns the credential id, the COSE key bytes exactly as the authenticator wrote them, and the sign count.
  - `verify_authentication` makes the same client data and authenticator data checks for `webauthn.get`. It rejects a sign count that is not greater than the stored one unless both are zero. It verifies the signature over authenticatorData followed by SHA-256 of clientDataJSON with the stored key: ES256 as DER ECDSA on P-256, EdDSA as Ed25519 and RS256 as PKCS#1 v1.5 with SHA-256.
  - Each failure is its own `Error` variant.
- `std/orm` builds queries from conditions over field references, such as `orm.filter(orm.from(users), orm.ge(User.age, 18))`, and type-checks them against the field. It renders them for the connection's `Dialect`, maps rows onto structs, stores nested values as JSON columns, creates tables from types with an identity key named `id`, and returns generated keys from inserts.
- `sql.Value` holds dates as days since 1970-01-01, times as microseconds since midnight, timestamps as microseconds since 1970-01-01 UTC, infinite dates and timestamps as the `i64` maximum and minimum, and UUIDs as 16 bytes. A connection serves one green thread at a time; a call made while another is running fails with `Busy`. `orm.transaction` rolls back on any error from its body and fails with that error, or with the rollback's error when the rollback fails too.
- `std/sql/postgres` authenticates with SCRAM-SHA-256, MD5 or cleartext. It uses the simple protocol for statements without parameters and the extended protocol with a 64-entry prepared statement cache otherwise, streams `each` in batches of 512 rows, reads results as text decoded by type OID, and keeps a connection usable after a server error. A timeout sends a CancelRequest and closes the connection. `connect_tls` upgrades with SSLRequest and `std/tls`.
- `std/sql/sqlserver` speaks TDS 7.4 with SQL authentication.
  - Prelogin offers no encryption to `connect` and full encryption to `connect_tls`. `connect_tls` runs the `std/tls` handshake inside PRELOGIN packets and unwraps the packets the server keeps sending around TLS records. LOGIN7 announces UTF-8 support.
  - Statements without parameters go as SQL batches. With parameters they go to `sp_executesql` as `@p1..@pN` declared as `bigint`, `float`, `bit`, `decimal(38, s)`, `nvarchar`, `varbinary`, `uniqueidentifier`, `date`, `time(7)`, `datetime2(7)` or `datetimeoffset(7)`, with NULL sent as `nvarchar(4000)`. Every request carries the transaction descriptor, and `begin`, `commit` and `rollback` use transaction manager requests.
  - Results decode every fixed, nullable, decimal, money, date and time, GUID, character, binary, long-value, text, image, XML, `sql_variant` and user-defined type. GUIDs become RFC 4122 byte order. Single-byte text decodes through its collation's UTF-8 flag or as Windows-1252. Row counts sum the DONE and DONEINPROC counts.
  - `each` streams rows as they arrive, and stopping early sends an ATTENTION and reads until the server acknowledges it. A server error below severity 20 leaves the connection usable. A per-connection watchdog sends an ATTENTION and closes the stream when one read waits longer than the timeout, and the call fails with `Timeout`.
- `std/toml` parses into ordered tables and applies every rule of TOML 1.0.0, including the restrictions on redefining tables through headers, dotted keys, inline tables and arrays of tables. It agrees with Python's `tomllib` on the toml-test suite. Errors carry a line and column. `encode` writes plain values first and then `[table]` and `[[array]]` sections. `serialize` and `deserialize` bind types through serde, reading date-times as RFC 3339 text.
- `std/yaml` parses YAML 1.2.2 into ordered node trees and agrees with the yaml-test-suite on every case's event stream and JSON equivalent.
  - A scanner modelled on libyaml tokenizes the whole stream, tracking indentation, implicit keys, JSON-like adjacent values and the YAML 1.2 tab rules, with 32-byte vector scans over plain scalars and `simd.find_byte_str` for comment and line ends. Token text is a view into the source.
  - A recursive-descent parser turns tokens into `Event`s with resolved tags, and a composer builds `Node`s carrying line, column, tag, anchor and style.
  - Plain scalars resolve with the core schema. `!!str`, `!!int`, `!!float`, `!!bool`, `!!null` and `!!binary` are checked against their content; other tags stay on the node. Aliases share their anchored node. `Options.max_nodes` bounds the tree with every alias expanded, failing with `TooManyNodes`, and `max_depth` bounds nesting. Duplicate scalar keys are an error.
  - `encode` writes block style, quotes a scalar only when it would read back differently (including YAML 1.1 booleans, numbers and dates), writes multi-line text as literal blocks, and anchors any node reached more than once. `serialize` and `deserialize` bind types through serde, with `[]u8` as `!!binary`.
- `std/otel` keeps the provider in the green thread's context slot as an immutable `Context` of the active span, baggage, a suppression flag and the SDK.
  - `spawn` copies the slot, so a child green thread's spans parent to the spawning span. `install` and `setup_from_env` return a droppable `Sdk` that shuts down when released and put the `SdkHandle` into the calling green thread's slot, reaching the green threads it spawns afterwards. Programs install at the top of `main`. Every API call checks the slot first and returns a no-op value without allocating when no SDK is present.
  - Timestamps and durations come from `time.now_nanos` and `time.monotonic`. Span and trace ids come from a `std/rand` generator behind a lock. Samplers are always on, always off, trace-id ratio on the low 63 bits, and parent-based.
  - Batch processors for spans and logs queue to a bounded channel with a non-blocking send that counts drops. A worker green thread exports when a batch fills, when the delay passes, and on flush or shutdown.
  - Metric instruments aggregate per attribute set, keyed by a canonical encoding of the sorted attributes, in one stream per reader, capped at 2000 series plus an `otel.metric.overflow` series. Readers collect cumulatively or as deltas, periodically or on demand.
  - Exporters encode OTLP once through a writer that emits protobuf through `std/protobuf` or the OTLP JSON mapping, and send with the context suppressed.
  - OTLP/HTTP posts, optionally gzipped, retrying 429, 502, 503 and 504 through `std/retry` with equal-jitter exponential backoff, where `Retry-After` raises the wait.
  - OTLP/gRPC makes unary `Export` calls on `opentelemetry.proto.collector.{trace,metrics,logs}.v1` over the pooled client's HTTP/2 connection, with h2c prior knowledge for `http://`. The message carries the 5-byte gRPC prefix, optionally gzipped under `grpc-encoding`, with `te: trailers` and `grpc-timeout`. The result is `grpc-status` from the trailers or, in a trailers-only response, the headers. CANCELLED, DEADLINE_EXCEEDED, ABORTED, OUT_OF_RANGE, UNAVAILABLE and DATA_LOSS are retried. RESOURCE_EXHAUSTED is retried only when `grpc-status-details-bin` carries a RetryInfo, whose delay replaces the backoff.
  - `setup_from_env` reads the standard `OTEL_*` variables. `OTEL_EXPORTER_OTLP_PROTOCOL=grpc` selects gRPC, defaulting to `http://localhost:4317`.
- `std/http` sits below `std/otel` in the import graph, so the hook runs through the context slot. `otel.Context` declares an `http.Ambient` as its first field, and `std/http` reads the slot as `*http.Ambient`, which relies on struct fields being laid out in declaration order. One constructor builds every `Context` and derives the ambient `Observer` from it: the observer is set exactly when an SDK is installed and the context is not suppressed, and `std/http` does nothing when it is absent. Processors and metric readers call exporters through `quietly`, so no exporter has to suppress its own telemetry.
  - `http.serve` and `serve_tls` open a server span per request over HTTP/1.1 and HTTP/2, with the parent extracted from `traceparent` and `client.address` from `Request.remote`, and record `http.server.request.duration`. `std/web` adopts that span, adds `http.route` and renames it to the method and route; it opens its own span only when its handler runs outside `http.serve`.
  - The client opens a CLIENT span named after the method for each attempt sent, redirects and retries included. Each span is under the caller's span, with `http.request.resend_count` from the second attempt. The client injects `traceparent`, `tracestate` and `baggage` into that attempt, sets `url.full` without userinfo, and ends the span when the body is delivered or the attempt fails, with `error.type` the status from 400 or the `http.Error` variant. It records `http.client.request.duration`.
  - `std/sql/driver` wraps every connection that `std/sql/postgres`, `std/sql/sqlserver` and `std/sql/sqlite` return with `instrument_sql`, reading the `db.system.name` from the connection's `Dialect`. `std/orm` opens client spans with the operation and table; a nested database span merges into the outer one, so an ORM query produces one span.
- `std/protobuf` writes and reads the protobuf wire format without code generation. An `Encoder` appends into a buffer that doubles as needed. Signed integers sign-extend to ten bytes and `sint64` zigzag-encodes. `begin_message` returns a mark and `end_message` measures the body, shifts it right by the size of its length varint and writes the length, so nested messages need neither a second buffer nor padded lengths. Output matches `protoc --encode` byte for byte. A `Decoder` walks fields in order as number, wire type, integer value and a view of the payload. It rejects truncated or overlong varints, lengths past the end, field 0 and wire types 6 and 7 with `Malformed`, and groups with `Unsupported`. The `as_*` readers check the wire type and the `unpack_*` readers accept packed and single encodings.
- `std/xml` reads XML 1.0 (fifth edition) with Namespaces from UTF-8, UTF-16 with a byte order mark, or any encoding `std/charset` knows by its declared label, converting to UTF-8 and normalising line endings first.
  - `xml.Reader` is a pull reader: `next` returns one token and `position` its line and column. Names arrive with namespaces resolved and are interned in a per-reader table. Text, CDATA, comments and attribute values are views into the input unless a reference or normalisation forces a copy, and 32-byte vector scans find the bytes that end them. The reader enforces well-formedness and the namespace constraints and agrees with libxml2 on what it accepts.
  - The internal subset is parsed. General entities expand, including markup, with recursion detection, a nesting limit of 40 and a total of 1 MiB plus eight times the input. Attribute-list declarations supply defaults and normalise tokenised types. External entities are never fetched and parameter entities are not expanded.
  - `xml.parse` builds an `Element` tree of owned strings with `attribute`, `child`, `children`, `text` and `find`. `xml.Writer` escapes text and attributes, splits CDATA around `]]>`, writes namespace declarations and indents element-only content. `serialize_value` and `deserialize` bind types through serde, mapping fields to attributes or child elements by local name, repeated elements to slices, and `xsi:nil` to `none`.
- Pipes block through channels. A writer flushing more than the pause threshold waits until the reader brings the unconsumed bytes below the resume threshold, or until the reader itself waits for more data, so a message larger than the thresholds cannot deadlock the pipe.
- `pool.open` stores `ConnectionHandle` views, so one connection serves a lease at a time and the pool, not the owner, closes it. Idle and lifetime limits are checked on acquire, release and `prune` against `clock.monotonic`, and a release hands the connection to the oldest waiter. A lease marks its connection broken on `Connect`, `Authentication`, `Closed`, `Timeout` or `Protocol` errors, and the pool closes it. A transaction left open at release is rolled back.
- `migrate.up` refuses any applied migration whose checksum differs (`Drift`), a pending version below the highest applied one (`OutOfOrder`) and duplicate or non-ascending versions. The lock is `BEGIN IMMEDIATE` on SQLite, `pg_advisory_lock` on PostgreSQL and `sp_getapplock` on SQL Server, and each migration re-checks inside its transaction that it is not yet recorded. A migration that fails reports its own error, or the rollback's error when the rollback fails too; a lock that cannot be released fails with `Sql` unless a migration error is reported first. In a text migration a statement ends at a line ending in `;`, and `-- +begin` to `-- +end` keeps a block with several `;` as one statement.

#### Unicode

- `std/unicode` reads tables generated by `cmd/tablegen unicode` from UnicodeData.txt, CaseFolding.txt and DerivedNormalizationProps.txt of Unicode 16.0.0, the version Python's `unicodedata` reports, and agrees with it on every code point. `cmd/tablegen unicode` and `cmd/tablegen properties` also write `normalization_layout.low` and `property_layout.low`: zero-argument functions for the record sizes, field offsets, trie shifts and masks, Hangul constants and packed property bit fields, and one `<table>_table()` accessor per `.bin` file that `embed`s it. The module sources read these instead of literals, and `TestLayoutFilesMatchGenerator` fails when a committed layout file differs from the generator.
- The tables are `embed`ded binary files. A two-level trie of 64-entry blocks covers code points below U+2FA40 and gives 0 above. It maps each code point to a 16-bit record id. Each record holds the canonical combining class and 16-bit offsets and lengths into a pool of `u32` code points for the full canonical decomposition, the full compatibility decomposition and the C and F case folding.
- Hangul syllables decompose and compose algorithmically. Decomposition is followed by a stable canonical reordering by combining class. Composition binary-searches a sorted table of primary composites, leaving out Full_Composition_Exclusion. ASCII input returns at once, or lowercased by `casefold` and `search_key`.
- `search_key(s)` equals Python's `''.join(c for c in unicodedata.normalize('NFKD', s.casefold()) if not unicodedata.combining(c))`. It skips reordering, since every reordered character is dropped.

#### HTML

- `std/html` tokenizes UTF-8 HTML following the WHATWG tokenization states. It builds no tree and never fails. Tag and attribute names are lowercased in ASCII, attribute values may be double-quoted, single-quoted or unquoted, a repeated attribute keeps its first value, and a tag cut off by the end of input is dropped.
- A start tag for `script`, `style`, `xmp`, `iframe`, `noembed` or `noframes` switches to raw text, and one for `title` or `textarea` to escapable raw text, until the matching end tag in any case. `plaintext` takes the rest of the input. The script-data double-escape states are not implemented, and `noscript` is tokenized as when scripting is disabled.
- Comments end at `-->` or `--!>`, with `<!-->` and `<!--->` empty. `<?...>`, `<!...>` other than comments and doctypes, `<![CDATA[...]]>` and `</` followed by a non-letter become comments, and `</>` is skipped. `Doctype` carries the lowercased name and the optional public and system identifiers.
- Character references are decoded in text, escapable raw text and attribute values. Named references match the longest name against the full WHATWG table of 2,231 names, including the names valid without a semicolon. `cmd/tablegen entities` generates the table and also writes `entity_layout.low` with the index entry layout, the name length limit and the table accessors. Decimal and hexadecimal references work with or without a semicolon. Zero, surrogates and values above U+10FFFF become U+FFFD, and 0x80 to 0x9F map through Windows-1252. Inside an attribute, a match without a semicolon that is followed by `=` or a letter or digit stays literal. Text without references is a view into the input.
- The tokens match Python's `html.parser` except where it departs from WHATWG: duplicate attributes, processing instructions and CDATA as comments, raw text after `<script/>`, and control-character references.

### Regular expressions

- `std/regex` compiles a pattern once into a program for a Pike VM, a Thompson NFA simulation that carries capture slots per thread, so matching takes time linear in the input times the program size and never backtracks.
- A `Regex` keeps two pools of idle machines: one without capture slots for `is_match`, one with them for `find`, `full_match` and the scans. Each machine owns its thread queues, capture slots and scratch. A call takes an idle machine under a lock, builds one when none is idle, and puts it back when it finishes. A compiled value is therefore shared freely between green threads, and a callback that matches with the same `Regex` during `replace_all` gets its own machine. A `Match` copies its slots out of the machine.
- Syntax: literals; escapes `\n \t \r \f \v \a`, `\xhh`, `\uhhhh`, `\Uhhhhhhhh` and escaped punctuation; `.`; classes with ranges, negation and `\d \w \s \D \W \S` inside; `\d \w \s \D \W \S \b \B \A \Z \z`; `^` and `$`; capturing groups, `(?:...)`, inline flags `(?i)`, `(?m)`, `(?s)`, `(?-i)` and scoped `(?i:...)`; alternation; `*`, `+`, `?`, `{m}`, `{m,}`, `{,n}` and `{m,n}` with lazy forms. A `{` that does not form a valid count is a literal. Inline flags apply to the rest of the enclosing group.
- Input is UTF-8 matched by code point; offsets are bytes. `\d`, `\w`, `\s`, `\b` and case folding are ASCII. `.` excludes only `\n` unless `dot_all` is set. Without `multiline`, `$` matches at the end or before a final `\n`; with it, `^` and `$` match at every line boundary.
- Matching is leftmost-first, as in Python and RE2, and results agree with Python's `re` under `re.ASCII`. `find_all`, `replace_all` and `split` follow Python 3.7: an empty match is allowed right after a non-empty one, and after an empty match the next one may start at the same offset only if it is not empty. `split` does not include group text. `split_n` splits at most `max` times; a negative `max` splits everywhere.
- Repeat counts are limited to 10000 and programs to 100,000 instructions. Invalid patterns fail with `Invalid(at, message)`, at the byte offset Python reports. Backreferences, lookaround, named groups and possessive quantifiers are rejected.
- `regex.generated("pattern")` is a `Regex` built at compile time, like .NET's `[GeneratedRegex]`. The pattern must be a string literal; flags are written inline. An invalid pattern is a compile error at the literal, `invalid regex at byte N: message`, with the offset and message `compile` would fail with. The result is used through the same functions as a compiled `Regex` and matches identically.
- `internal/regex` holds a Go copy of the parser and assembler, held to the Low one by running the `std_regex_errors` fixture's patterns through it. The checker turns each distinct literal into Low functions named `__regex_` and the FNV-1a hash of the pattern, parsed with the prelude's privileges and added to `std/regex`, so they reach its private types.
- A `Regex` carries an `Engine` of thread-start, step and skip functions with the queue size. `compile` fills it with the Pike VM. The generated engine keeps the VM's queues, priorities and capture slots but replaces interpretation with code per instruction:
  - One function per instruction that starts a thread, follows a consumed character or joins two epsilon paths, each guarded by the queue's sparse set. The remaining jumps, splits, saves and assertions are inlined into it.
  - Characters and classes of up to four ranges compare inline in the step, which dispatches on the program counter by binary search. Larger classes get a binary-search function.
  - When no empty match is possible from the start, a skip function moves past ASCII bytes that cannot begin a match before a thread starts.
  - Programs over 2000 instructions are compiled at run time by `compile` instead, so call depth stays bounded.

### Compression

- `std/compress/codec` holds what the codecs share. `Buffer` is a growable byte buffer with an LSB bit packer, and every write to it is infallible. Each encoder is a core that writes into a `Buffer`, with an `Encoder` wrapper that drains the buffer to its sink, so one-shot `compress` has no error to handle. `BitReader` reads LSB-first bits from a slice or an `io.Reader` and fails with `Malformed` on short input. `Tables` builds two-level lookup tables for canonical Huffman codes, and `Builder` computes length-limited code lengths. `Chain` is a hash-chain match finder, `copy_match` copies an LZ77 match, and `read_varint` and `write_varint` code LEB128 integers, which `std/protobuf` and xz also use. Deflate, brotli, WebP lossless, zstd, LZ4, snappy, xz and bzip2 build on it where their formats agree.
- DEFLATE, zlib and gzip decode stored, fixed and dynamic blocks with two-level lookup tables, check Adler-32, CRC-32, ISIZE and the gzip header CRC, and read concatenated gzip members. The encoder uses greedy matching at levels 1 to 3 and lazy matching at 4 to 9 over a hash chain on a 32 KB window, choosing dynamic, fixed or stored encoding per block. `flush` emits a sync flush so streamed output is decodable as it arrives.
- xz decodes multiple blocks and streams, the none, CRC32, CRC64 and SHA-256 checks, LZMA2 chunks with their resets, and the delta filter; other filters fail with `Unsupported`. The encoder writes LZMA2 with a hash-chain match finder and lazy parsing, a dictionary from 256 KiB to 64 MiB by level, and raw chunks where compression does not pay.
- `std/compress/zstd` and `std/compress/lz4` each offer `compress(data, level)`, `decompress(data, limit)`, a streaming `decode(reader, sink, limit)` that passes output to the sink one block at a time, and a streaming `Encoder` with `encode`, `flush` and `finish`.
  - The zstd decoder handles every block and literal type, 1- and 4-stream Huffman literals, all four FSE table modes, repeat offsets, skippable and concatenated frames, and the content checksum. Its history grows with real output up to the smaller of the window and the remaining limit, so large windows never allocate what the header claims up front.
  - The zstd encoder finds matches with two hash chains keyed on 8-byte and 4- or 5-byte prefixes, with depth and lazy evaluation set by the level. It Huffman-codes literals, picks predefined, RLE or custom FSE tables per block by estimated cost, and falls back to raw or RLE blocks.
  - The LZ4 decoder accepts linked and independent blocks, block and content checksums, content sizes and skippable frames. The encoder writes linked 64 KB blocks with a content checksum.
  - Dictionaries and LZ4 frames with magic number `0x184C2102` fail with `Unsupported`.
- `std/compress/brotli` has the same API as zstd and LZ4.
  - The decoder handles compressed, uncompressed and metadata meta-blocks, block switching for literals, commands and distances, all four literal context modes, context maps with run-length coding and inverse move-to-front, distance postfix and direct parameters, the static dictionary with all 121 transforms, and windows of 10 to 24 bits. Output collects in a ring buffer that starts at 64 KB and doubles up to the window, and reaches the sink at the end of each meta-block, when the ring wraps, and before each blocking read. The 122,784-byte dictionary is `embed`ded from `dictionary.bin`.
  - The encoder writes uncompressed meta-blocks at level 0. Levels 1 to 11 run hash chains of depth 2^(level-1) with lazy matching from level 4 and checks against the four recent distances and their neighbours, writing 256 KB meta-blocks with Huffman-coded commands and distances. From level 5 they model literals in UTF-8 context mode with up to 3·level-11 clustered trees. A block that would grow is stored. `flush` ends the meta-block and pads with an empty metadata block so everything sent decodes.
- `std/bits` holds a growable `Set` over `[]u64` words. Counts use a SWAR popcount, and scans (`next_set`, `next_clear`, `min`, `max`, `rank`, `select_at`, `members`) step a word at a time.
- `std/bits/roaring` holds `Roaring`, a set of `u32` split into 65536-value chunks keyed by the high 16 bits. Each chunk is a sorted `[]u16` array of up to 4096 values, a 1024-word bitmap, or a list of runs. Arrays and bitmaps convert at 4096 values; ranges create runs, and unions and intersections of runs stay runs. Every container tracks its cardinality, so `cardinality` is constant time. Set operations come as new-set and in-place `_with` forms, working word by word on bitmaps and by galloping merge on arrays. `run_optimize` picks the smallest form per container by the Go library's rule. `serialize` and `deserialize` implement the portable Roaring format byte for byte as Go `roaring/v2` writes it, rejecting any inconsistent length, ordering or cardinality with `Malformed`.
- `std/image` holds an `Image` of width, height, `Format` (`Gray8`, `GrayAlpha8`, `Rgb8`, `Rgba8`, `Gray16`, `Rgba16`, 16-bit samples big-endian), stride and pixel bytes.
  - Pixels read and write as 8-bit `Rgba`, with operations converting through 16-bit intermediates. `get` outside the image is transparent black and `set` there does nothing. Operations convert formats, crop, flip, rotate, apply EXIF orientation, fill rectangles, composite straight alpha, premultiply and unpremultiply, and resize separably on premultiplied values with nearest, bilinear, Catmull-Rom bicubic or Lanczos-3 filters.
  - `decode` picks a codec by signature and rejects images over 2^26 pixels; `decode_limited` takes another limit.
  - PNG decodes every colour type, bit depth and Adam7, checks every CRC, and encodes with per-row adaptive filtering. The result is `Gray16` for 16-bit gray without transparency, `Rgba16` for other 16-bit images, `Gray8` or `GrayAlpha8` for gray, `Rgba8` for colour with alpha or `tRNS`, and `Rgb8` otherwise.
  - JPEG decodes baseline and progressive scans with restart markers, any sampling, grayscale and Adobe CMYK/YCCK, with libjpeg's integer IDCT and fancy upsampling, so output matches libjpeg byte for byte. It encodes baseline 4:2:0 with the standard tables scaled by quality.
  - GIF decodes 87a and 89a and composites every frame by disposal method in `decode_gif_frames`. It encodes one frame with a given or median-cut palette.
  - BMP covers 1 to 32 bits, RLE, bitfields and every header version, and encodes 24 or 32 bits.
  - QOI decodes and encodes; the result is `Rgb8` or `Rgba8` by channel count.
  - WebP decodes lossless VP8L with every transform, the colour cache and meta prefix codes. Lossy VP8, alpha chunks and animation fail with `Unsupported`.
- Every codec has buffer functions, a streaming decoder that pulls from an `io.Reader`, pushes output to a sink and fails with `TooLarge` past a caller's limit, and a streaming encoder.

### Web

- `web.app()` holds routes and middleware. `get`, `post`, `put`, `patch`, `del` and `route` add handlers for paths with `:name` parameters and a trailing `*`. `group` makes a sub-application with a prefix and more middleware, and `mount` adds its routes back. An unknown path is a 404 and a known path with another method a 405 with `Allow`.
- `endpoint[Req, Res]` and `query_endpoint[Res]` read a JSON body into `Req`, call a handler returning `Res ! web.Problem`, and write the result as JSON or the problem as `application/problem+json`. Their schemas describe the operation in the OpenAPI document that `openapi` serves, along with path parameters and each route's security schemes.
- Middleware is a struct of a `run` closure and an optional security scheme. `api_key`, `basic` and `bearer` authenticate requests and set `Context.user`. `sessions` keeps a map in a cookie sealed with AES-256-GCM, with the cookie name as associated data and an expiry inside. `csrf` requires the session's token in `X-CSRF-Token` on unsafe methods. `require_session`, `cors`, `logger` and `compress` complete the set; `compress` picks Zstandard, Brotli or gzip, in that order, from what `Accept-Encoding` allows.
- `jwt` verifies bearer tokens against a key set with `std/crypto/jose`. `oidc` does the same for an OpenID Connect issuer: it reads the discovery document and the JWKS, refreshes them hourly, and refreshes early, at most once a minute, when a token names a key it does not know. `oidc_login` adds a sign-in route that redirects to the issuer with `state`, `nonce` and a PKCE S256 challenge kept in the session, and a callback route that checks the state, exchanges the code, verifies the ID token and its nonce, and hands the claims to the application.
- A handler streams a body by returning a `Response` with `stream` set to a `chan[[]u8]` and closing it at the end. The server writes each chunk as it arrives, cancels `req.token` when the stream ends, the client disconnects or a write fails, and drops the stream for `HEAD` and bodiless statuses. Response compression skips streamed responses.
- `sse.stream` sets `text/event-stream`, `no-cache` and `x-accel-buffering: no`, flushes every event, sends a `: keepalive` comment every 15 seconds unless `keepalive` is 0, and stops when `req.token` is cancelled. `Emitter.send` returns false after the client has gone. `sse.format` removes CR and LF from `id` and `event` and NUL from `id`.
- `web.static` decodes each path segment and answers 400 for `..`, an embedded separator, NUL or a bad escape, and 404 for dotfiles, non-regular files and any file whose real path leaves the root. A directory without a trailing slash is redirected, and a directory serves `index.html` or an optional listing. `precompressed` picks `.br` then `.gz` by `Accept-Encoding`. A multi-range or malformed `Range` gives the whole file. Files stream in 64 KiB chunks.
## Compiler

The compiler is written in Go. Pipeline:

| Stage | Package | Output |
|---|---|---|
| Lex | `internal/lexer` | tokens |
| Parse | `internal/parser` | `internal/ast` tree |
| Check | `internal/check` | types attached to the tree |
| Build IR | `internal/ir` | SSA form, one value per machine word |
| Codegen | `internal/amd64` | x86-64 GNU assembly |
| Assemble | `internal/asm` | object: sections, symbols and fixups |
| Link | `internal/link` | ELF or PE executable |

Building a program and running `low dev` need no external tool.

- `low` builds and runs on Linux and on Windows. The default target is the host; `-target linux/amd64` or `-target windows/amd64` selects one. Every target is assembled and linked in process.
- On the Windows target, `std/sys` is built from its `_windows.low` files, which call kernel32, ws2_32, iphlpapi, shell32 and crypt32, so files, processes, sockets, DNS, terminals, the environment and the system root certificates work there. `__syscall` itself compiles to a call that panics with `system calls are not available on windows`.
  - A descriptor is a handle. Descriptors 0 to 2 name the standard handles. A socket's descriptor packs the socket in its low 32 bits and a manual-reset event in its high 32 bits; `WSAEventSelect` ties the event to the socket's network events, `wait_socket` checks readiness with `WSAPoll` and parks on the event, and `shutdown` signals it to wake waiters. Closing the descriptor reads and discards unread input first, because Windows answers a close with unread input by a reset that discards data the peer has not yet read, then closes both.
  - Only sockets are non-blocking. A read from a pipe or file holds an OS thread until it returns; `wait_input` parks on a console handle until it is signalled and otherwise sleeps a millisecond between tries.
  - `process.start` calls `CreateProcessW` with the command line quoted for the Microsoft C runtime, a UTF-16 environment block, and inheritable copies of the child's three streams passed through `PROC_THREAD_ATTRIBUTE_HANDLE_LIST`, so the child inherits nothing else. The process handle stands in for the pidfd, `wait` waits on it and reads `GetExitCodeProcess`, and `kill` calls `TerminateProcess`. `process.find` splits `PATH` on `;` and tries each `PATHEXT` extension.
  - Environment names compare without ASCII case. `os.home_dir`, `cache_dir`, `config_dir`, `data_dir`, `state_dir` and `temp_dir` read `USERPROFILE`, `LOCALAPPDATA`, `APPDATA`, `TMP` and `TEMP`. `net.lookup` reads the hosts file under the system directory and asks the adapters' IPv4 DNS servers from `GetNetworkParams`. `x509.system_roots` reads the `ROOT` certificate store.
  - A refused connection to a loopback address fails at once rather than after the SYN retransmissions Windows would otherwise make.
  - Signals, inotify watches, pseudo-terminals, setting a terminal's size, `chown`, peer credentials, binding to a device and `SO_REUSEPORT` fail with `Unsupported`. User and group IDs are -1 and the umask is 0.
  - Returning from a C call can wait out a collection; the wait keeps the thread's last-error value, so `GetLastError` after an extern call reads that call's error.
- The `low` command lives in `cmd/low`. `compiler.Build` is the one build entry: it generates the assembly for the unit's target, assembles it, links it with the linker that target uses and writes the executable. `low build`, `low run`, hot reload, the fuzz oracle and the Vulkan and Wayland generator tests all call it. Hot reload is the only caller that passes the hot mode value.
- `low ir` prints the SSA form and `low asm` the generated assembly.
- Compilation is deterministic: the same source and target always produce the same SSA form, assembly and binary. No pass depends on map iteration order; where order matters the compiler walks slices or stacks.
- `parser.ParseSource` is the one entry from source text to tree, for user files, the prelude, the standard library and generated code. The lexer lexes each interpolated `{}` expression into tokens when it lexes the string, and keeps the `,width:format` suffix as text that `ast.ParseSpec` parses.
- A lexer, parser or checker error is a `*diag.Error` holding the position of the fault; `low lsp` reads it with `errors.As`.
- `types.Universe` interns the pointer, slice, optional, raw pointer, channel, map, result and function types, and owns the built-in type names and the constraint names. The checker lays out and instantiates types in three phases: declaring, laid out, and functions ready.

Modules:

- `internal/compiler` finds the root, loads the entry module, then loads each imported directory once, breadth first, and rejects import cycles. One loader reads the program, its imports, `embed` files and the embedded standard library through `fs.FS`. `compiler.Overlay` puts unsaved editor text over the disk for the language server, which shares the loader's `Root` lookup and `ParseFile`. `internal/check` does no file IO.
- Every file records its module path and every source position its file, so errors name the file they come from.
- The checker keeps a scope of types and functions per module. The prelude belongs to the entry module, and its `__` names cannot clash with anything else. Every module sees the prelude's generic `__` functions and types, so `std/container/deque` is a wrapper over the prelude's `__Queue`, the ring buffer behind channel wait queues.
- A function in module `ui/text` becomes the symbol `fn_ui.text.shout`. Types display with their module path, such as `geometry.Circle`.

The runtime, `internal/amd64/sched.asm` and `internal/amd64/runtime.asm`, is emitted with every program and talks to the kernel through raw syscalls. It ends the process with `exit_group`, so every thread stops.

The runtime's data layout, thread-local access and Windows stack-bounds switching are defined once, in `layout.asm` and the target's `linux_header.asm` or `windows_header.asm`, as constants and macros such as `G_SCANLO`, `TLS_GET`, `TEB_ENTER_OS` and `MSG_BEGIN`. Generated code and patches use those names and macros, and `Generate` and `GeneratePatch` both prepend the headers. Go passes `RT_DYNAMIC` and the hot-reload constants (`HOT_BASE`, `HOT_SIZE`, `PATCH_BYTES`, `PATCH_ENTRY`, `PATCH_SIGNAL`) to the assembly as `.set` lines, and defines `HOT_INIT` as the call to `rt_hot_init` in a hot build and as nothing otherwise. The Windows import table lists every function the Windows runtime calls through `WIN`, taken from `windows.asm`, plus the program's `extern` imports.

The contracts between the stages have one owner each:

- `internal/builtin` numbers every builtin. The checker records the number on each call and `internal/ir` lowers it through a table indexed by that number.
- `internal/ir/runtime.go` lists every runtime routine generated code calls: its symbol, its register convention (plain, growth, system call or string equality), whether it returns a scalar or nothing, and whether it allocates. A system call is its own `syscall` instruction; the others are `runtime` instructions.
- `internal/prelude` names every prelude function and struct the compiler uses, the layout of each struct, and which functions a map or channel type instantiates. The prelude is validated when it loads, so a missing or reshaped entry is an error that names it. `internal/ir` reads field offsets from the checked types.
- `internal/amd64` owns the target. A `Platform` value, `sysv` for Linux and `win64` for Windows, supplies the runtime and header text, the C argument classification, the registers a C callback saves, the entry symbol, how a system call is made, stack probing and the dynamic-library shims. `PROGRAM_ENTRY` and `PROGRAM_EXIT` in each header are the program's entry and exit. `amd64` exports the entry symbols, the hot-patch list and count symbols and the `.idata` section names that `internal/link` and `internal/hot` read.
- One `convention` value in `internal/amd64` holds the Low register convention: argument, result, caller-saved, callee-saved, system call and growth registers, and the frame offsets derived from them.

### Scheduler

- Green threads run on OS threads, one per CPU in the process's affinity mask. Threads start as work appears, with `clone` in a static program and `pthread_create` in a dynamic one. A thread whose green thread is in a C call does not count against the limit, so another starts while C blocks.
- Run queues:
  - Each OS thread keeps a 256-entry run queue. A full queue spills to a global queue, and `spawn` queues on the global queue when its own thread already holds 16.
  - A thread takes work in this order: expired timers; the global queue every 61st schedule; its own `runnext` and queue; a batch from the global queue (its length divided by the thread count, plus one, at most 128); ready descriptors; stealing.
  - A thief takes half of the victim's queue into its own. It tries the victim's `runnext` only in the last of 4 rounds, and takes it at once when the victim is in a C call or stopped (asleep on a runtime lock or held by a collection), or when the running victim's `runnext` sequence has stayed unchanged for 2,000 timestamp-counter ticks.
  - At most half of the running threads spin looking for work. With nothing to do, a thread sleeps on a futex or becomes the single poller that blocks in `epoll_wait` until the next timer.
  - Every 32nd schedule on an OS thread moves `runnext` to the tail of its run queue and takes the queue's head, so green threads that keep readying each other cannot starve the rest of the queue.
- A green thread reserves 1 MB of stack with a guard page below it. Pages are committed as they are touched, and finished green threads are reused. Its context is its saved registers on its own stack. A fault in the guard page panics with `stack overflow`; any other fault panics with `invalid memory access`.
- Per-thread state lives in a 24-byte `.tbss` block directly below the thread pointer: the thread record at `fs:-24`, the running green thread at `fs:-16` and the preemption flag at `fs:-8`. A static program sets `fs` with `arch_prctl` and `CLONE_SETTLS`; a dynamic program uses the block glibc allocates for every thread. Generated code uses the fixed offsets, so hot reload patches need no TLS relocations.
- Preemption:
  - Every loop back-edge tests the preemption flag.
  - A monitor thread sets the flag every 10 ms on a thread whose green thread has run since the last check, and the collector sets it everywhere.
  - A set flag calls `rt_poll`, which saves every general register and `xmm2` to `xmm15` and yields or stops for the collector.
  - Code that holds a channel lock does not yield. Recursion without loops is not preempted.
- `park` and `ready` hand a green thread off with a per-thread lock and a wake token, so a wakeup that arrives before the green thread has finished parking is not lost.
- Wakeups:
  - `ready` puts the green thread in its OS thread's `runnext` slot, so it runs as soon as the current one parks, and bumps a per-thread `runnext` sequence. A green thread pushed out of that slot goes to the stealable run queue.
  - `ready` then calls `wakep`, which acts only when its OS thread has not woken anyone in the last 100,000 timestamp-counter ticks or has not scheduled since it last did. A pair of green threads that hand work to each other therefore stays on one OS thread, while a sender that keeps computing after `ready` always has an idle thread woken to steal its `runnext` within the 2,000-tick delay.
  - `wakep` does nothing while an OS thread is already spinning. Otherwise it wakes one idle thread, or starts a new one below the CPU limit, and the woken thread counts as spinning until it finds work or gives up. A spinner that finds work while more is queued wakes a successor.
  - `spawn` always wakes a thread without the interval.
  - The monitor thread moves the `runnext` and wakes a thread for the queue of an OS thread that sits in a C call or a blocking system call.
- In-call threads:
  - An OS thread in a C call, or in a system call that can block, is marked in-call, which the collector treats as stopped. `rt_world_stop` does not wait for it, and on return from the call the thread parks in `rt_m_running` while a collection is in progress.
  - The code generator emits `__syscall` inline only when its number is a constant on a short list of calls that never block on external events: `close`, `lseek`, memory mapping, signals, `pipe`, `dup`, socket creation and configuration, process creation and exit, time, identity, limits, `epoll_ctl`, `eventfd` and `getrandom`.
  - Every other number, and every non-constant number, calls `rt_sysblock`. It saves the argument registers on the green thread's scanned stack, marks the thread in-call, makes the call and leaves the in-call state through the same path as a C call.
  - The monitor counts in-call threads that have not scheduled since its previous visit, and `wakep` does not count them against the CPU limit, so work queued behind one finds a running thread.
  - A green thread blocked in `read` on a pipe therefore does not delay a collection started by another.
- Timers are a locked array of deadlines. `sleep` and `select`'s `after` arms add to it, and the poller wakes through an eventfd when an earlier timer arrives. Descriptors are registered with `EPOLLONESHOT`.
- Channels and `select` are generic prelude code over `__Chan[T]`: a lock word, a ring buffer and ring queues of waiters that keep their capacity.
  - A waiter is claimed with compare-and-swap before it is woken, so a `select` waiting on several channels and timers fires exactly once. Queued entries whose waiter has been claimed are skipped and dropped lazily.
  - An operation that completes at once allocates nothing. A buffered send or receive that needs no waiter runs inline in `__chan_send` and `__chan_recv`.
  - An operation that must wait allocates the waiter and value box under the lock it already holds, queues them, releases the lock and parks, so each blocking send or receive takes the lock once. Green threads to wake are readied after the lock is released.
  - `select` locks its channels in address order and tries each arm with a probe that returns the green thread to wake, using a stack slot as the receive box. A ready arm allocates nothing.
  - When no arm is ready and there is no `else`, `select` allocates one waiter, queues it on every channel with the stack slots as boxes, releases the locks, adds the `after` timers and parks. When woken it deletes the timers and clears the slots.
- Channel lock word: 0 when free, 1 when held, 2 when held with sleepers.
  - `rt_lock` takes a free lock with one compare-and-swap.
  - A contender that finds the lock held makes 4 polling rounds. Each round is a `pause` loop that waits 8,000 timestamp-counter ticks, doubling every round, before it reads the word, so contenders do not bounce the cache line that holds the channel's fields. The contender checks the stop flag each round and takes the lock with a compare-and-swap when it reads 0.
  - After the rounds it yields its OS thread once, then exchanges 2 into the word. Reading 0 means it holds the lock; otherwise it waits on a futex until woken and exchanges again. While it sleeps its OS thread counts as stopped, so a collection can proceed.
  - `rt_unlock` exchanges 0 into the word and wakes one sleeper only when the old value was 2.
- C calls run on the OS thread's system stack (8 MB in a static program). Around a C call the generated code pushes the callee-saved registers on the green thread's stack, records where the green thread stopped, marks the thread as in C and switches stacks. On return it marks the thread running again, waiting if a collection is under way.
- A C callback runs on the green thread's stack. Its thunk marks the thread running, switches back to the green thread's stack below the C call's saved registers, and locks the green thread to the OS thread until it returns, because the C frames live on that OS thread's system stack: if the callback waits, the OS thread waits with it. Callbacks must arrive on the thread that made the C call.

### Linking

The in-process assembler and linker reproduce GNU `as` and `ld` and `clang` and `lld-link` output byte for byte: `internal/asm` matches the object bytes of `as` for Linux and `clang` for Windows, and `internal/link` matches the segments, sections, symbols and loaded bytes of `ld` for ELF and the whole image of `lld-link` for PE. Layout choices that look arbitrary, such as `clang`'s padding and jump sizes or the XXH3 timestamp, are there to match. The oracle tests in `internal/asm` and `internal/link` check this against the pinned tool versions.

- A static program is assembled by `internal/asm` and linked in process by `internal/link`, which writes the executable without running `as` or `ld`. It starts at the generated `_start`.
  - The image is based at `0x400000`, below the hot-reload region, and has the layout of `ld -static` with separate code: a read-only header segment, then read-execute `.text`, read-only `.rodata`, and read-write `.data` and `.bss`, each page-aligned in memory and congruent with its file offset. `.tbss` has a `PT_TLS` segment and the stack is non-executable.
  - Symbols resolve across the sections of the one object. `PC32` and `PLT32` resolve alike, and `Abs32` and `Abs32S` fail when the value does not fit. An undefined symbol fails the link and names the symbol.
  - The file carries section headers and a symbol table of the object's non-`.L` symbols plus `__bss_start`, `_edata` and `_end`, so `debug/elf`, `readelf` and `gdb` read it, and `low dev` reads it to link patches against it. `_start` initialises the heap and the scheduler, starts `main` on the first green thread and enters the scheduler on the process stack.
- A program that declares C libraries is assembled and linked in process too, by `link.Dynamic`, as a non-PIE executable that asks `/lib64/ld-linux-x86-64.so.2` to load its libraries. No C start files are linked.
  - The layout follows `ld`: `PT_PHDR`, `PT_INTERP`, a read-only `PT_LOAD` holding the headers and the dynamic tables, a read-execute `PT_LOAD` for `.plt`, `.plt.got` and `.text`, a read-only `PT_LOAD` for `.rodata`, a read-write `PT_LOAD` for `.dynamic`, `.got`, `.got.plt`, `.data` and `.bss`, then `PT_DYNAMIC`, `PT_TLS`, `PT_GNU_STACK` and `PT_GNU_RELRO` over everything the loader may write before it starts the program.
  - Each library's `-l<name>` is the first `lib<name>.so` in `/usr/lib`, `/usr/lib64`, `/usr/lib/x86_64-linux-gnu`, `/lib/x86_64-linux-gnu`, `/lib64`, `/lib` and `/usr/local/lib`, with symlinks followed. `DT_NEEDED` carries the library's `DT_SONAME`. `c` and `m` name `libc.so.6` and `libm.so.6`. Libraries appear in declaration order, then libc. A library that is missing, or is not a shared object, fails the link and names the library.
  - Every symbol a program calls or takes the address of is checked against the libraries' dynamic symbols, and one that no library exports fails the link. `.dynsym` records each import with its `GLIBC_*` version in `.gnu.version` and `.gnu.version_r`, and both `.hash` and `.gnu.hash` are written.
  - `PLT32` calls go through a lazy `.plt` entry with a `JUMP_SLOT`, or through a `.plt.got` entry when the symbol also has a GOT slot. `GOTPCREL`, `GOTPCRELX` and `REX_GOTPCRELX` resolve to a `.got` slot with one `GLOB_DAT` for each imported symbol, and a slot for a symbol the program defines holds its address. A direct reference to an imported data symbol would need a copy relocation and fails the link.
  - The generated `_start` calls `__libc_start_main(main, argc, argv, 0, 0, rtld_fini, stack_end)` with no init or fini functions, which is all glibc needs: stdio buffers flush and `atexit` handlers run when the program exits. `atexit` and `at_quick_exit` exist only in `libc_nonshared.a`, so a program that declares them gets shims that tail-call `__cxa_atexit` and `__cxa_at_quick_exit`.
  - glibc's start-up code calls the generated C `main`, which does the same work as `_start`.
  - When Low's `main` returns, the program calls `exit(0)`, letting libc run its exit handlers. An error from `main` prints and exits with status 1.
- A Windows program is assembled in process with the padding and jump sizes of `clang`, and linked by `link.PE` into a PE32+ console executable that starts at `start`. No import library is read.
  - The image is based at `0x140000000` with 4 KiB section alignment and 512-byte file alignment. It reserves 8 MB of stack and 1 MB of heap, and sets `DYNAMICBASE`, `HIGH_ENTROPY_VA` and `NX_COMPAT`.
  - Sections merge by the name before `$`, `.bss` joins `.data` after its initialised bytes, and `.idata` joins `.rdata`. The order is `.text`, `.rdata`, `.data`, the rest by name, then `.reloc`. Code padding is `int3`. `.rodata` is writable.
  - `.rdata` starts with a debug directory holding one `REPRO` entry. The header timestamp and that entry carry the low 32 bits of the XXH3-64 hash of the image taken with both zero, so a program always links to the same bytes.
  - The code generator writes the import tables as `.idata$2` to `.idata$7` with `__imp_` labels. The import directory covers the `.idata$2` descriptors and a null descriptor the linker adds after them, which is followed by the code generator's `.idata$3` terminator, as `lld-link` lays out the same object, and the import address table directory covers `.idata$5`.
  - `extern "c"` imports from `msvcrt.dll`, `extern "m"` from the UCRT maths set `api-ms-win-crt-math-l1-1-0.dll`, which has the C99 functions such as `round` and `cbrt` that `msvcrt.dll` lacks, and any other `extern "name"` from `name.dll`.
  - Every `Abs64` fixup gets a `DIR64` entry in `.reloc`, one block for each 4 KiB page, so the loader can rebase the image. `ImgRel32` resolves to an RVA, and `PC32` and `PLT32` to rip-relative values. 32-bit absolute, GOT and thread-local references fail the link, and so does an undefined symbol, which the error names.
  - `.incbin` paths are written with octal escapes for every byte outside printable ASCII, and `\` and `"` escaped, so drive letters, backslashes and non-ASCII names assemble unchanged.

Prelude:

- `internal/compiler/prelude.low` is Low source compiled ahead of every program. It holds:
  - UTF-8 decoding.
  - The string builder behind interpolation and concatenation.
  - The generic map table.
  - Integer formatting.
  - Float formatting by unrounded scaling against a table of 128-bit powers of ten from 1e-348 to 1e347, which finds the shortest digits that read back to the same value without allocating.
  - Formats and widths for interpolation. A float's exact decimal expansion is its mantissa times 2^e, built in base 10^9 limbs by repeated multiplication by 2 for a positive `e` and by 5 for a negative one, with the decimal point moved by `-e` places. An integer's digits are its magnitude. Rounding to the requested decimals or significant digits then works on those digits, using the exact remainder to break ties to even, so `f`, `e`, `n` and pictures need no approximation. NaN and infinities take the default text.
- `cmd/tablegen pow10` generates the table twice: as the `__pow10_hi` and `__pow10_lo` functions the prelude holds, and as `std/text/pow10.bin`, which `std/text` embeds for float parsing because standard library files cannot call prelude functions.
- Prelude names start with `__`, which user code cannot use. Four intrinsics are available only in the prelude:
  - `__bits` gives a float's bit pattern.
  - `__str_view` turns a `[]u8` into a `str` without copying.
  - `__hash` hashes any `Hash` value.
  - `__zero` is the all-zero value of its expected type.
- A function's IR is built only when `main` reaches it through a call or a function address, so a program that formats no floats neither builds nor carries any of this code. Reachable functions are built in call order and emitted in declaration order.

Standard library:

- It lives in `internal/compiler/std` and is embedded in `low` with `go:embed`.
- An import whose path is `std` or starts with `std/` is read from there, never from the project root.
- Its files are lexed like the prelude, so they may use `__` names and intrinsics, and they are left out of the sources `low dev` watches.
- `embed` in a standard library file reads from the embedded tree and writes the bytes once to `lowlang/embed/` under the user cache directory, named by their SHA-256, for the assembler's `.incbin`.

Standard library intrinsics:

- `__argc()`, `__argv(i)` and `__envv(i)` read the argument count, argument `i` and environment entry `i` (`null` past the end) as NUL-terminated `ptr[u8]`s. The start code saves them from the initial stack in a static program and from `main`'s arguments in a dynamic one.
- `__syscall(n, args...)` makes Linux syscall `n` with up to six integer, `bool` or `ptr` arguments and returns `rax` as `i64`, a negative errno on failure.
  - It lowers to a `syscall` instruction, which the code generator emits inline for a constant number on the non-blocking list and as a call to `rt_sysblock` otherwise (see the Scheduler section).
  - The code generator emits `syscall` inline: it moves the arguments into `rax`, `rdi`, `rsi`, `rdx`, `r10`, `r8` and `r9` with the parallel move and executes `syscall`.
  - Being a call, it keeps no value in a caller-saved register across it, which covers the `rcx` and `r11` the kernel clobbers.
- `__slice_ptr(x)` gives the data address of a slice or `str` as a `ptr[T]`.
- `__mulhi(a, b)` is the high 64 bits of the 128-bit product of two `u64`. It is an IR instruction that becomes one `mul`.
- `__hash_seed(i)` is word `i`, a literal 0 or 1, of the process's 128-bit hash seed. It is an IR instruction that becomes one load from `rt_hash_seed`.
- `__extend_by(mut b, n)` grows a `[]u8` by `n` bytes with at most one allocation and returns the index of the first new byte, whose contents the caller overwrites.
- `__make(n)` is a zeroed slice of `n` elements of the expected slice type, allocated through the runtime's grow routine.
- `__memmove(dst, src)` copies `min(dst.len, src.len)` elements with the runtime's overlap-safe `rt_memmove` and returns the count. It takes a `str` source for a `[]u8` destination. `rt_memmove` loads up to 64 bytes into registers before storing them and uses `rep movsb` beyond that.
- `__clock_nanos(id)` returns `clock_gettime(id)` in nanoseconds, or 0 on failure, through `rt_clock_nanos`. `rt_clock_nanos` calls `__vdso_clock_gettime` when start-up found it from `AT_SYSINFO_EHDR` and the vDSO's dynamic symbol table (DT_HASH or DT_GNU_HASH), and makes the syscall otherwise.
- `__context()` reads the current green thread's context slot as the expected pointer type, `__set_context(p)` replaces it and `__has_context()` tells whether it is set. `spawn` copies the parent's slot into the child, and the collector marks every live green thread's slot.
- `__serialize`, `__deserialize` and `__schema` are the derivations described below, allowed only in `std/serde`.

The garbage collector is a non-moving mark-sweep collector in the runtime:

- The heap is a 16 GB reservation made with `mmap` and `MAP_NORESERVE`. Pages are committed as they are touched.
- Object layout:
  - Each object has a one-word header. Bits 3 and up hold the object's size in words, header included, so the word is the size in bytes with its low three bits used as flags: bit 0 is always zero, bit 1 marks a free chunk and bit 2 marks an object with no pointers to scan.
  - A bitmap records the first word of every object and free chunk, which resolves pointers into the middle of an object, such as a `mut` parameter aimed at a heap field.
  - Two summary bitmaps above it hold one bit per non-empty bitmap word and one per non-empty summary word. Finding the start behind an interior pointer skips empty spans 32 KB and 2 MB at a time instead of reading every bitmap word between the pointer and its object's header.
  - A second bitmap of the same shape holds the mark bit of each object. It is zero outside a collection.
  - The start bitmap, the mark bitmap, the first summary bitmap and the mark stack are touched sparsely or in small steps, so they are excluded from transparent huge pages on Linux; one touched byte would otherwise make a 2 MB page resident.
- Allocation buffers:
  - Each OS thread allocates objects of up to 255 words from its own buffer of up to 4096 words without taking the heap lock: it bumps a pointer, writes the header and sets the object's start bit.
  - Only the start-bitmap words at the buffer's unaligned edges are set atomically, with `lock or`; the other words are updated with a plain `or`. The summary bits are tested first and set atomically only when clear, and each thread remembers the last bitmap word whose summary bits it set, so allocation skips the summary update while it stays in that word. The memory is reset whenever the thread takes a new buffer.
  - A thread whose buffer is full takes the heap lock and refills it from, in order: the smallest free chunk large enough in the size-class bins below, the exact-size free lists from the largest down, then the end of the heap. It zeroes the buffer's dirty pages in one pass after releasing the lock.
  - A collection turns every thread's unused buffer tail into a free chunk before marking, and an allocation that would leave a single word in a buffer takes that word too, so the sweep finds a header at every object boundary.
- Free chunks:
  - Free chunks of 64 words or more sit in 256 size-class bins, four per power of two, and a 256-bit bitmap marks the non-empty ones. Bins 0 to 127 hold chunks whose pages may be non-zero (dirty) and bins 128 to 255 hold chunks known to be zero (clean).
  - An allocation scans up to eight chunks of its own bin for the smallest that fits, then takes the first chunk of the lowest non-empty bin above it, preferring a dirty bin to a clean one of the same class, and splits off the remainder.
  - Chunks under 64 words sit in exact-size free lists that only buffer refills read.
  - Objects over 255 words take the heap lock and allocate from the bins, then from the end of the heap, and are zeroed after the lock is released.
- Dirty pages:
  - A bitmap with one bit per 4096-byte heap page records which pages may hold non-zero words. Pages never handed out are zero from the operating system and are clean.
  - An allocation zeroes only the dirty pages of its range and then marks the whole range dirty, so a large zeroed object that is never written costs no resident memory.
  - Every free chunk of 1 MB or more whose interior pages are all clean is filed in a clean bin.
- Returning memory:
  - Before the heap end moves for an allocation, every dirty free chunk of at least 1 MB returns its whole interior pages to the operating system on Linux with `madvise(MADV_DONTNEED)`, clears their dirty bits and moves to a clean bin.
  - The walk starts at the first bin that can hold such a chunk and visits dirty bins only, so a chunk is advised once and not again until it is reused and freed.
  - Resident memory holds the live objects and the new one rather than also the chunks that were too small.
  - On Windows the pages stay committed and chunks stay dirty.
- Collection runs when the words allocated since the last one exceed the words that survived it, with a 4 MB floor. A thread's allocation buffer counts by the size of the chunk it receives, so refilling from small free chunks after a sweep counts no more than the memory handed out.
- `LOW_GCTRACE=1` prints one line per collection to stderr: `gc N pause_us= mark_us= sweep_us= live_kb= heap_kb= slow_allocs= refills=`, from timestamps around the stop, mark, sweep and restart and from per-thread counters of buffer refills.
- `LOW_GC=off` never collects. `LOW_GC=stress` collects after every 8192 words allocated.
- An allocation whose type holds no pointers is noscan: `new` of a pointer-free value, and the backing arrays of slices whose elements hold none, such as `[]u8`, `[]i64` and `[]f32`. A noscan object is marked but its contents are never read. Other objects are scanned word by word.
- Allocation and pinning take a heap lock. A collection stops the world: it sets a stop flag and every thread's preemption flag, then waits until each other thread is stopped at a safepoint, idle in the scheduler or in a C call, and holds those threads there until it finishes. Threads waiting for a lock stop for the collector while they wait.
- Roots come from stack maps.
  - At every call the code generator records the frame words that may hold a heap pointer at that point: the saved callee-saved registers, spill slots of values that may be pointers and are live across the call, and every frame slot. The maps form a table sorted by return address.
  - The words of `rt_cells`, one per hoisted literal, are scanned as roots in every collection.
- Green-thread stacks:
  - Every green thread records where it stopped: the return address, stack pointer and frame pointer of the Low code that entered the runtime, and the lowest stack word holding registers it saved.
  - The collector scans each live green thread's saved registers conservatively, then follows its `rbp` chain, looking up each frame's return address and scanning only its mapped words, and scans the green thread's first frame conservatively.
  - Frames without a map, such as code patched in by hot reload or a loop interrupted by a preemption check, are scanned whole. A green thread that has not started is scanned whole.
  - Values in callee-saved registers are found in the save slots of whichever later frame saved them, or among the registers saved when the green thread stopped.
- A callback thunk crosses C frames by segment. Its frame record holds the Low caller's frame pointer and return address, so the `rbp` chain leads from the thunk straight back to the caller. Its frame keeps the bounds of the C frames on the system stack and of the registers the C call saved. The thunk's call site has a map of count -1, which tells the collector to scan both regions conservatively before following the chain.
- Every candidate word, mapped or not, is checked against the heap before it is followed, so a map that names a word holding an integer is harmless.
- A `ptr` counts as a possible heap pointer, so a `cstr` copy or a pinned object stays alive while Low holds its address. `pin` records the address in a table of 65536 roots that every collection scans; `unpin` removes it.
- Marking resolves a candidate word first by testing the start bit of the word before it, which is the header when the candidate is an object's first field address, and falls back to the summary walk otherwise. Candidates are prefetched through a ring of pending headers before they are marked, so cache misses overlap. Marking sets the object's bit in the mark bitmap and adds its size to the live word count. Newly marked noscan objects are not pushed on the mark stack.
- Sweeping walks the start bitmap, jumping over empty words with the summary bitmaps, so its cost follows the number of objects and not the address span of the heap.
  - It rebuilds the free lists and bins from empty.
  - Within a word, the start bits without a mark bit are dead and the rest are live. Sweeping jumps between the two kinds of run to coalesce adjacent dead objects into free chunks.
  - A chunk keeps only its first start bit, and the word's mark bits are cleared once it is done.
  - A trailing free run returns to the end of the heap.

The derivations are generated as source:

- When the checker meets `__serialize(v, w)`, `__deserialize[T](r)` or `__schema[T]()` with a concrete type, or a `print` or interpolation of a type other than a number, `bool` or `str`, it writes a function for that type in Low text, binding the type and each type it mentions to names like `__X0`.
- It parses the text with the prelude's privileges, marks it trusted so it may read and build private fields, and checks it like a generic instance.
- Calls inside it to the same intrinsics derive the types they reach. Each derived function is cached by type.
- The serialization of a type is `fn(w: Writer, mut path: []i64, v: T)`. `path` holds the addresses of the pointers being written, pushed before a pointer's target is written and popped after it; a pointer already on it panics with `serde: cycle through *T`. `__serialize(v, w)` starts with an empty path, and the derived code recurses through `__serialize_into(w, mut path, x)`.
- The rendering of a type is `fn(mut b: []u8, mut path: []i64, v: T)`, which appends to `b` through the prelude's `__append_*` functions. `path` holds the addresses of the pointers being rendered, pushed before a pointer's target is rendered and popped after it. A `print` or interpolation creates an empty buffer and path, calls it, and uses the buffer as the text; an interpolation appends into the buffer it is composing. A number, `bool` or `str` is rendered by the same `__append_*` functions, so `print` has one text path: it builds the text and writes it with `rt_print_str`. `__render(v)` gives the text of any value with a `str` quoted, for `std/testing`'s `format`.
- Bound names resolve in type positions, struct literals and variant selectors.
- A field reference `T.field` is checked as `serde.Field[T, F]` and lowered to the field's name and index as constants.

### IR

- Generic templates never reach the IR.
  - The checker keeps an untouched copy of each generic function's syntax tree, clones it for each distinct set of type arguments and checks the clone with those types bound. The result is an ordinary function named like `max[i64]`; generic structs and enums become ordinary types named like `Stack[str]`.
  - Instances are made on demand, checked from a work list, and limited to 1000 per template.
  - Symbols escape every character outside `A-Za-z0-9_.` as `.` and its hex code, so `max[i64]` is `fn_max.5bi64.5d`.
- SSA is built directly from the checked tree with Braun et al.'s algorithm: variables are read and written per block, phis are created lazily and sealed once a block's predecessors are known, and trivial phis are removed.
- Every value is one 64-bit word. A struct, enum or result of n words is n SSA values, so aggregates live in registers with no frame temporaries.
- Narrow integers are kept sign-extended (`i32`) or zero-extended (`u32`, `u16`, `u8`) in their word. Floats are kept as their bit pattern, `f32` zero-extended. Arithmetic, conversion and comparison values carry their type.
- Frame slots and SSA variables:
  - A variable passed as `mut` to a function, captured by a closure, or of a C or vector type lives in a frame slot (or a heap box when captured) and is accessed with loads and stores. Every other variable is pure SSA.
  - `push`, `__extend`, `__extend_by` and `delete` take their `mut` operand in place. On an SSA variable they read and write its words (a slice's pointer, length and capacity) as SSA values, so a slice that is only pushed to, indexed and passed by value stays in registers.
  - Lengths and capacities these builtins compute are typed integers and never count as pointers.
- Integer division carries its zero and overflow checks inside the instruction. A constant divisor other than 0 and -1 needs neither check.
- A call to a C function becomes a `ccall` carrying its C signature: the symbol, the type of each argument, which arguments are `mut` places, and the result type. `internal/amd64` classifies the signature for the target. On Linux the System V AMD64 rules classify each argument word:
  - Integers, `bool` and pointers take `rdi`, `rsi`, `rdx`, `rcx`, `r8` and `r9`.
  - Floats take `xmm0` to `xmm7`.
  - An `extern struct` of at most 16 bytes is split into eightbytes that go to vector registers when every field in them is a float and to integer registers otherwise.
  - A larger struct, or one that does not fit in the registers still free, goes on the stack.
  - Variadic `f32` arguments are widened to `f64`, and `al` holds the number of vector registers used.
  - A struct result of at most 16 bytes comes back in `rax`, `rdx`, `xmm0` and `xmm1` by class. A larger one is written to a buffer whose address goes in `rdi`.
  - Narrow integer, `bool` and `f32` results are re-extended, and so are narrow places passed as `mut` once the call returns.
- On Windows the Win64 rules apply. Every argument takes one of four slots, `rcx`, `rdx`, `r8` and `r9`, and a float also takes the matching `xmm0` to `xmm3`; a variadic float is passed in both. The slots after the fourth go on the stack above 32 bytes of shadow space. An `extern struct` of 1, 2, 4 or 8 bytes passes in its slot. Any other struct is copied into the caller's outgoing area and passed by address, and a struct result of another size is written to a buffer whose address goes in the first slot. Results come back in `rax` or `xmm0`.
- Every frame keeps `rsp` 16-byte aligned at its calls, as C requires: frames are rounded to 16 bytes, and `_start` rounds the space it reserves for `main`'s result.
- An `extern struct` value is its C bytes, one word per eightbyte. Its locals and parameters live in frame slots. A field is loaded or stored at its own width, and a struct literal or a field of a computed value goes through a temporary slot. A struct whose size is not a multiple of 8 is stored and loaded up to its last byte, with 4-, 2- and 1-byte accesses for the final partial word, so writing a nested field or an element of a `view` never touches the bytes after it.
- `defer` keeps a stack of deferred statements per block and lowers them again at every exit from that block.
- Drop and `using`:
  - A struct's `drop` becomes a function `T..drop` whose parameters are the struct's fields.
  - Every droppable type gets a generated `T..release` that calls `T..drop` and then the releases of its droppable fields.
  - A `using` binding adds a deferred `if flag { T..release(x) }` to its block. A move reads the value and clears the flag.
  - Moves are found by a flow-sensitive pass over each checked function body, which joins moved states across branches and runs a loop body a second time when an iteration moves a binding.
- `==` on a struct is the conjunction of its fields' comparisons. On an enum it compares tags, then branches on the tag to compare only that variant's fields, since the other payload words are undefined. An `extern struct` is stored in two frame slots and compared field by field at each field's offset and width, so padding is ignored.
- Function values and closures:
  - A Low function value is two words: a code address and an environment pointer. Code reached through a function value takes the environment as parameter word 0.
  - A closure is checked in place and becomes a function named after its parent, such as `counter..c1`. Its environment is a heap array, or 0 when it captures none. Environment words follow the order of the closure's captures.
  - A captured variable that is assigned after its binding, passed as `mut`, a C struct or a vector lives in a heap box allocated when it is bound. Every access, in its own function and in closures, loads and stores through the box, and the environment holds the box pointer in one word.
  - Any other captured variable stays an SSA variable of its function and the environment holds its words inline. The closure reads them into SSA variables on entry.
  - `spawn` binds its callee and arguments to such variables, so a spawn allocates the environment and nothing else.
  - A named function used as a value goes through an adapter, `name..value`, which drops the environment and calls it.
- A call through a function value moves the code address to `r10` and calls it, passing the environment and the arguments like any call. A call through an `extern fn` value is a `ccall` with the same plan as a named C function.
- A C callback is a thunk, `cb_name`, emitted once per program for every function used as one. It saves the C argument registers, re-extends narrow integers and `bool` from them and from the C stack, calls the function with Low's convention on the green thread's stack and moves a float result to `xmm0`. On Windows it also saves and restores `xmm6` to `xmm15`, which C treats as callee-saved.
- Every loop's back-edge block is marked. The code generator ends it with `cmp byte ptr fs:[-8], 0` and a branch to an out-of-line `call rt_poll`.
- `spawn f(a, b)` becomes a block that binds the callee and the arguments to fresh locals and passes a closure calling `f` with them to `rt_spawn`.
- Sends, receives, `close` and channel loops call the prelude's instances for the element type. `select` calls the prelude's per-arm helpers and the runtime's `rt_lock_all`, `rt_timer_add` and `rt_park`.
- `funcaddr` gives a function's code address, its thunk, or an `extern` function's address through the GOT. Functions reached only through `funcaddr` are kept.
- In dev builds every C function gets a global trampoline, `cfn_name: jmp name`, and calls go through it, so a patch can link against the running binary.
- An `embed` becomes an address and a length. The data is emitted with `.incbin` under a label named by the file's content hash, so a changed file changes the code of every function that embeds it.
- Every value records whether it may hold a heap pointer.
  - Loads, parameters and call results take this from their type's word layout, where an enum word counts as a pointer if any variant puts one there.
  - Constants, typed arithmetic, string literals and frame addresses never hold pointers.
  - Untyped address arithmetic and anything unclassified count as pointers.
  - A phi holds a pointer if any input may.
- A `str` is two words and a slice three. An element access computes `pointer + index * stride` after a `check` instruction, which compares unsigned and jumps to the index panic. Packed elements load with `movzx`, `movsxd` or a 32-bit `mov` and store at their width.
- Maps:
  - A map `[K: V]` is a pointer to the prelude's generic `__Map[K, V]`, whose `entries` slice holds `__Entry[K, V]` records of a live flag, the key and the value.
  - An open-addressing index of entry numbers with linear probing finds them. Deleted entries leave tombstones until the next rebuild, which compacts the entries in order and resizes the index to keep it at most three quarters full.
  - Integer, `bool` and pointer keys hash their word and `str` keys their bytes, both with the seeded multiply of the Maps section.
  - Every concrete map type instantiates `__map_new`, `__map_get`, `__map_set`, `__map_delete`, `__map_len`, `__map_next`, `__map_key` and `__map_value` for its key and value types. Map literals, lookups, assignments, `delete`, `.len` and loops become calls to them.
  - A compound assignment such as `m[k] += v` is a get, the operation on the value or zero, and a set.
- `push` and the formatting helpers call the runtime's grow routine and the prelude. The buffer behind an interpolation lives in a frame slot passed as a `mut` argument. A `push` that finds the slice full grows it to twice the capacity plus 4.
- Counted push loops:
  - A counted loop `for i in a..b` whose whole body is `push(mut xs, v)` becomes one reservation instead of a push per iteration. `xs` must be an SSA variable. `v` must be made only of literals, other variables, field reads, conversions, unary operators and non-trapping arithmetic or comparison on numbers.
  - The trip count is `b - a` as an unsigned word when `a < b`; the loop is skipped otherwise. A trip count above 2^40 bytes of elements calls the grow routine with an impossible size so the program stops with the out-of-memory panic.
  - The slice grows once to exactly its length plus the trip count, or not at all when the capacity already suffices, and its length becomes that sum. When `xs` is known to be empty the growth is unconditional.
  - A value that is zero in every word (a zero constant or a conversion of one) needs no stores in a freshly grown buffer, since the runtime returns zeroed memory. Otherwise, and in the no-growth case for a zero value, the elements are written with a counted loop.
  - A value that calls a function, can trap or reads the slice itself keeps the push loop.
- String building:
  - The prelude appends a whole string to a `[]u8` with `__extend`, which grows the buffer at most once and copies the bytes with the runtime's `rt_memmove`, so concatenation, interpolation, `bytes` and `str(b)` copy in bulk.
  - An integer is formatted by counting its digits against powers of ten, growing the buffer once with `__extend_by` and writing the digits from the end.
  - A concatenation chain and an interpolation, including interpolations and chains nested inside either, are flattened into one list of pieces. Adjacent literal pieces merge.
  - Every piece is evaluated first, in order. The result buffer is allocated once with the grow routine at the sum of the literal bytes, the lengths of the `str` pieces and the widest rendering of each number (20 for an integer, 25 for a float, 5 for a `bool`), which no rendering exceeds: the longest float is a negative value with exponent -6 and 17 significant digits, `-0.0000012345678901234567`.
  - A chain with no number pieces copies each piece with `rt_memmove` at a running offset. With number pieces the buffer lives in a frame slot, `str` pieces are copied at its length and numbers go through the prelude's `__append_*`.
  - A piece with a width adds the width's size to the estimate, is appended into the buffer and then padded in place by `__align`, which counts code points from the byte offset where the piece started. A literal piece with a width is kept as a piece rather than merged into the text around it.
  - A piece with a format calls `__append_decimal` (`d`), `__append_radix` (`x`, `X`, `b`, which masks a signed value to its type's width), or `__append_int_layout` or `__append_float_layout` (`f`, `e`, `n` and pictures) with the format's digits, decimals, grouping and mode as constants. An `f32` is widened to `f64` first, which is exact.
  - A piece with a format or a derived rendering can write more than its estimate. A composition containing one appends its literal and `str` pieces with `__append_str`, which grows the buffer when needed, instead of copying at a running offset.
  - String ordering calls the prelude's `__str_cmp`.
- Unreachable blocks are pruned after construction.

Frame-slot promotion:

- A frame slot is promoted to SSA values by the optimiser when its address is used only by word-sized loads and stores at word offsets inside it, and is never passed to a call, stored, compared with another address or reached through arithmetic with a variable. Its stores disappear, each load becomes the value stored on every path that reaches it, and phis join the paths. A slot read before any store reads an undefined value.
- A slot of a C or vector type, a `select` box, a lock array and a slot with more than one address value stay in memory.
- After inlining, a `mut` argument whose callee is inlined therefore lives in registers.
- Loads and stores carry the flags raw (a narrow or tail access of a C struct, or `load` and `store`) and trap (`load`, which is never discarded).

Optimiser:

- An optimiser runs over each function's SSA before phis are classified.
  - It keeps each value's type, float width and may-hold-a-pointer flag. A value that replaces another holds a pointer if either did.
  - A pass leaves the control-flow graph alone, so phi inputs and predecessor lists stay valid while it runs.
  - Between passes a branch on a constant becomes a jump to the taken successor, blocks left unreachable are pruned together with their predecessor entries and phi inputs, and trivial phis are removed. The pass runs again until no branch folds.
- The optimiser builds a dominator tree with the Cooper-Harvey-Kennedy algorithm and walks it from the entry, so each value sees only facts established by its dominators. The tree is generic over its node type and the code generator builds it over machine blocks. Predecessors that the entry cannot reach are ignored.
- Constant folding and simplification:
  - Copies are propagated, and integer operations, comparisons and `not` on constants are folded at their own type: `i32` results are sign-extended and `u8`, `u16` and `u32` results zero-extended exactly as the generated code would leave them.
  - Division and remainder of constants by zero or by -1 are never folded, so the runtime panic and the overflow case stay in the instruction.
  - Adding, subtracting, or-ing, xor-ing or shifting by zero, multiplying by one, and multiplying or and-ing by zero reduce to an operand or a constant. Multiplication by a power of two becomes a shift.
- Division and remainder by a constant other than 0 and -1 become arithmetic on the 64-bit word, which is exact for narrow operands because they are kept extended.
  - A divisor of 1 is the operand or zero.
  - Unsigned powers of two are a shift or a mask.
  - Signed powers of two add `(x >> 63) >>> (64 - k)` before the arithmetic shift so the result truncates toward zero. A negative divisor negates the quotient.
  - Any other divisor multiplies by a magic number with the high-multiply instruction, signed (`imul`) or unsigned (`mul`), adds the operand back or halves and re-adds it when the magic number needs 65 bits, shifts, and for signed values adds the dividend's sign bit.
  - A remainder is the dividend minus the quotient times the divisor.
  - Signed division by the minimum 64-bit value stays an instruction with no checks.
- Common subexpressions are removed within the dominator tree for add, subtract, multiply, and, or, xor, shifts, conversions, function addresses and `check`. Comparisons, constants and loads are not merged: a load may observe a store, and a comparison that stays single-use can fuse into its branch.
- Bounds-check elimination:
  - A `check [i, n]` is removed when an identical check dominates it, or when `i < n` is known on the path and `i` is known to be non-negative.
  - A branch on a single-predecessor block's comparison of integers establishes `i < n` on the true edge of `i < n`, on the true edge of `n > i`, on the false edge of `i >= n` and on the false edge of `n <= i`.
  - `i` is non-negative when it is one of:
    - a non-negative constant;
    - an unsigned value;
    - a value a dominating signed comparison bounds below by a constant of at least 0 (or by -1 strictly);
    - a product of non-negative values that cannot overflow;
    - an induction variable that starts non-negative and only grows by a non-negative step without overflowing.
  - A variable stepping by one cannot overflow below a bound it is compared against. Any other step cannot overflow when the upper bounds of the variable and the step, added, stay within the type's range.
  - An upper bound is derived for a value from a constant, from the width of a narrow unsigned type, from a dominating signed comparison against a value that has an upper bound (strict comparisons subtract one), and for a sum or product of two non-negative values from their upper bounds when the result stays within the type's range. Only comparisons whose block dominates the use contribute.
  - A check `[i, n]` with a constant `n` is removed when `i` is non-negative and its upper bound is below `n`.
  - Branch facts also record `i <= n` on the false edge of `i < n` and the true edge of `i >= n`. These never remove a check and never prove a step cannot overflow.
  - A length that is a phi satisfies `n >= L` when every input satisfies it on its incoming edge, by being `L`, a constant at least `L`, a phi that does, or a value that a fact on the input's dominators bounds below by `L`. When `i < L` is known and `n >= L` holds, `i` non-negative removes the check, and otherwise the check compares against `L` instead of `n`, which keeps the same panics.
  - A slice filled by a counted `push` loop from 0 to `L` has length `L` when `L > 0` and 0 otherwise, so indexing it by a counter below `L` needs no check.
- Dead-value removal:
  - Values nothing reads are removed, together with groups of values that only read each other, such as a phi and the add feeding it.
  - Only these operations are removed: constants, strings, embeds, undefined values, phis, add, subtract, multiply, and, or, xor, shifts, negation, `not`, complement, conversions, `sqrt`, `floor`, comparisons, function addresses, high multiplies, trailing-zero counts, loads, float division, and integer division or remainder by a constant other than 0 and -1.
  - Every other instruction stays, including calls, stores, checks, vector instructions, frame slots and divisions that can panic.
  - An unread load through a reference, a slice element or a frame slot is removed, since `*T` values are never null and an index is checked before its element is loaded. A `load` through a `ptr[T]` stays, so a null raw pointer traps even when the value is unused.
- Float arithmetic, negation, `sqrt`, `floor` and conversions between integers, `f32` and `f64` are folded on constants with the result the SSE instruction produces.
  - `f32` operations round to single precision.
  - A float to integer conversion of a NaN or an out-of-range value gives the minimum 64-bit value.
  - Unsigned 64-bit conversions are left to the instruction.
  - A result that is NaN is never folded, so the sign and payload of a NaN come from the hardware.
- Loop-invariant code motion:
  - It works on natural loops found from back edges whose header dominates the latch, innermost first, in loops with a single outside predecessor that ends in a plain jump. That predecessor is the preheader.
  - Only values whose operands are defined outside the loop or already hoisted move: multiply, and, or, xor, shifts, negation, `not`, complement, conversions, `sqrt`, `floor`, function addresses, high multiplies, trailing-zero counts, float division, and add, subtract or shift that is not the constant-offset address arithmetic an addressing mode absorbs.
  - Loads, checks, calls, stores and integer division stay where they are, so a loop that runs zero times executes nothing it would not have executed.
  - A constant a hoisted value reads is copied into the preheader, because the code generator defines a constant that is not an immediate at its first use.
- Strength reduction replaces `i * c` by an add induction variable when `i` is a basic induction variable of a loop with a constant step, and replaces `i * c + d` derived from it the same way. The new variable starts at the start value multiplied by `c` in the preheader and steps by `step * c` on the back edge, with 64-bit wrapping arithmetic, so the result equals the product for every start, step and trip count, including overflow and negative steps.
- Devirtualisation and specialisation:
  - A call through a function value whose code address is a function address becomes a direct call that keeps the environment argument.
  - Before inlining, a function loses its function-address and environment parameter words when it is never called through a function value, is not `main` or a callback, and receives the same function address and the same constant environment from every direct call site. The parameters become the constants, the arguments leave every call site, and the function and its callers are simplified again. Calls from the function to itself that pass its own parameters unchanged count as agreeing. This runs for up to four rounds, so a function value handed down through several calls reaches the call that uses it.
  - A function of at most 150 values that calls a function-value parameter, or hands it to another call, is cloned per function address when its direct call sites pass two or more different function addresses for that parameter, or one function address with different environments.
    - Each clone takes the call sites of one function address and is named `name..withN.target`, where `N` is the first parameter word.
    - The clone has that word as the constant, drops the environment word too when every one of those call sites passes the same constant, and is simplified, so its call through the function value becomes direct.
    - Calls from the clone to the original that pass the parameters unchanged call the clone. The original stays for the remaining call sites.
    - A function gets at most 4 clones, and clones add at most 4000 values to the program.
  - The same conditions let an integer parameter that receives one and the same constant from every direct call site become that constant, but only when it pays: the function is simplified as a copy with the constants substituted and as a copy without, and the substitution is kept when the copy with it has fewer checks, blocks or values besides constants and parameters.
- Inlining:
  - Release builds inline direct calls bottom-up over the strongly connected components of the call graph.
  - A callee is eligible when it:
    - is not part of a cycle;
    - is not a callback;
    - has at most 48 values besides constants, parameters and results;
    - has at most one loop and an entry block no jump returns to;
    - uses no vector instruction.
  - Frame slots, `ccall`s and runtime calls are cloned with the body, and the callee's slots are appended to the caller's.
  - A callee of at most 6 values is free. Any other site is charged to the caller, which grows by at most 240 values, and to the program, whose budget is a fifth of its value count before inlining, at least 6000 and at most 40000. The last quarter of the program budget is kept for call sites inside loops.
  - A caller inlines up to four rounds. Each round takes its sites inside the deepest loops first, then the smaller callees, then the earlier values; splits the call's block; clones the callee's blocks with parameters bound to the arguments; forwards a single return's values or joins several returns in phis; and simplifies the caller. Calls that an inlined body or a devirtualised call exposes are inlined in the next round.
  - Loop back edges keep their marks, and checks and divisions keep their panics.
  - Dev builds do not inline, specialise or hoist literals, so hot reload patches see every function and every literal's data.
- Literal hoisting:
  - A release build hoists a non-empty slice or map literal whose elements are all constants: integer, float, `bool` and `str` literals, negated numbers, `none`, enum variants without payload and non-C struct literals of constants.
  - The checker marks a literal read-only when it is the operand of an index, a `for` or `.len`, or is bound by a `let` whose every use is such an operand. An assignment, a `mut` argument or any other use anywhere in the target path of an element or field makes the binding non-read-only.
  - Each hoisted literal owns a word of `rt_cells`. The code loads the word, and when it is 0 builds the literal as usual and stores the pointer, so the first evaluation builds it and a racing thread at most builds a second equal copy.
  - A read-only literal uses the cell's pointer, as a slice of its length for a slice.
  - Any other use copies it: a slice with one grow call that moves the template's bytes, a map with the prelude's `__map_clone`, which copies the entry array and the index. A literal below the copy thresholds of the Slices and Maps sections is built each time in that case.
- Block merging and jump threading:
  - A block that ends in a jump to a block with no other predecessor and no phis is merged into it.
  - A block that holds a phi and at most one more value and branches on that phi, or on its negation, is skipped by a predecessor that passes a constant: the predecessor jumps to the successor the constant selects.

### Code generation

- Phis and layout:
  - Critical edges are split and each phi becomes a parallel copy at the end of every predecessor, emitted with cycles broken through `r11`.
  - An argument whose live range ends at that copy prefers the phi's register, and the phi prefers the registers of its arguments, so a loop-carried value usually needs no move.
- Block layout:
  - Blocks are laid out in reverse post-order, visiting the false successor first so the true successor follows its branch.
  - Inside a loop, the true arm of an integer `==` comparison against a constant is treated as rare when it holds a loop, a call, a return or an unreachable end. The blocks between the branch and its false successor move after the last block of the innermost loop, so the common path falls through to the back edge. A light arm stays in line, because the extra taken branches would cost more than the arm when it is the common path.
  - A jump or branch to an empty block goes to that block's final target, a jump to the next block in layout is omitted, and a conditional branch whose true target is next is inverted. A block holding phi copies is never empty.
- Loop latches:
  - A loop's latch whose header holds only a fused compare and branch ends with that compare and a conditional branch to the body, after the poll check when the block has one, instead of a jump to the header.
  - A header with up to 12 instructions, calls included, is copied after the latch instead, so the back edge tests the condition without a jump and the first test still runs at the original header.
- A comparison used only by the branch that ends its block is fused into a `cmp` and conditional jump.
- Addressing:
  - A load or store addresses `[base + index*scale + disp]` at every width.
  - A 64-bit `add` read only by addresses folds into them, as does a `shl` or `mul` by 1, 2, 4 or 8 feeding the index. A constant operand becomes the displacement. Other strides keep one multiply and fold the add and the displacement.
  - The folded base and index are uses of the load or store, so they stay live to it, across blocks where the definition dominates. A base or index in a spilled slot is reloaded into `r10` or `r11` immediately before the instruction. An instruction with a memory operand takes any other temporary from `rax`.
- Load folding:
  - A load is folded into its only consumer as a memory operand when both are in one block and no store, call, runtime call, vector operation or poll lies between them.
  - Integer arithmetic, logic, bounds checks and compares fold a 64-bit load.
  - A narrow load is never folded into a 64-bit operation. It folds only into a compare of the same type against a constant the type holds, as `cmp` of a sized memory operand with an immediate.
  - Scalar SSE arithmetic and compares fold a load of the operation's own width, `f64` or `f32`.
- A float multiplication by 2.0 is an addition of the operand to itself, which is exact for every input including NaN, infinities and signed zeros. Other float constants keep their multiply.
- Boolean branches:
  - A branch on `not x` swaps its successors instead of computing the complement, which is exact for float comparisons as well.
  - A comparison read only by `not` is emitted with the inverted condition.
  - A block that only branches on a boolean phi is threaded: a predecessor that passes a constant jumps to the successor the constant selects, a predecessor that passes the condition of its own branch jumps to the matching successor, and any other predecessor branches on the value it passes.
  - Evaluation order and short-circuiting of `&&` and `||` operands are unchanged.
  - A branch on the result of a `not` or a comparison defined in its own block uses that operand directly, so the `not` and the materialised boolean are not emitted.
- Spilling:
  - A value's spill weight is the sum over its uses and definitions of 8 raised to the block's loop depth, capped at depth 4.
  - When no register is free the allocator spills the conflicting value with the lowest spill cost, breaking ties by the later end of its live range.
  - The allocator runs three times per function, with the spill cost set to the weight, to the weight divided by the length of the live range and to the weight divided by its square root. It keeps the assignment whose spilled values have the lowest total weight plus 2 for each callee-saved register it uses. The weight alone wins ties.
- Constants that fit in 32 bits are used as immediates and take no register. An undefined value used as an operand or stored to memory is the immediate 0, so a variant's unused payload words never hold stale pointers.
- Narrow integer results:
  - Results of narrow integer operations that can leave the type's range are re-extended with `movsxd`, `movzx` or a 32-bit `mov`.
  - A `u32` add, subtract, multiply, negation, complement and constant left shift uses the 32-bit form of the instruction, whose write zero-extends and needs no re-extension.
  - A conversion to an integer type that holds every value of the source type is a register copy. A right shift of an extended value needs no re-extension.
  - Unsigned comparisons and branches use the unsigned condition codes, and unsigned division uses `div`.
  - A division by a constant other than 0 and -1 that reaches the code generator, which is signed division by the minimum 64-bit value, emits no zero or -1 test.
- Float registers:
  - Float values live in `xmm2` to `xmm15`; `xmm0` and `xmm1` are scratch.
  - A value is a float register value when it is produced or consumed by float arithmetic, a float comparison, a conversion, `sqrt` or `floor`. A float that only passes through moves, calls, stores and phis stays in a general register.
  - Scalar SSE instructions (`addsd`, `subss`, `sqrtsd`, `roundsd`, `cvttsd2si` and so on) work on registers directly, with a spilled operand as a memory operand.
  - An `f32` in an `xmm` register keeps bits 32 to 63 zero, so `movq` moves it to a general register or the frame unchanged. `movq` is used only where a float meets an integer word: call, return and parameter words, stores and loads, bit casts, and C argument registers.
- A float constant that does not fit in 32 bits is a `.rodata` quadword, shared by value and used as a memory operand. Zero is `xorps`.
- A float comparison used only by the branch that ends its block is fused into `ucomisd` or `ucomiss` and a conditional jump. `<` and `<=` swap their operands and use `ja` and `jae`, so an unordered result is false. `==` and `!=` also test the parity flag.
- Shifts and rotates:
  - Variable shift counts go through `cl`, with `rcx` saved in `r11` around the shift. Counts above 63 take a separate path.
  - The pair `(x << k) | (x >> (w - k))` is one `rol` when it meets all of these: both shifts act on one `u32` or `u64` value `x` of width `w`, `k` is a constant between 1 and `w - 1`, the shifts are read only by the `or`, and they appear in either order. No other combination is a rotate.
- `exp` is a runtime routine: range reduction by `ln 2` split into high and low parts, a degree-13 polynomial, and scaling by two powers of two so subnormal results stay in range.
- A comparison of a `str` against a literal of up to 16 bytes is inline: a length compare, then one compare of the bytes at the pointer against an immediate of 1, 2, 4 or 8 bytes, or two when the length is none of these, ending in `sete`. The literal's address and length are never materialised and the comparison is not a call. Any other `str` equality moves the arguments into the argument registers, returns 0 without calling `rt_str_eq` when the lengths differ and calls it otherwise.
- Register allocation:
  - Registers are assigned by linear scan over each value's live segments, so a value does not occupy a register where it is dead and a register's holes hold other values.
  - `rcx`, `rsi`, `rdi`, `r8` and `r9` hold values that are not live across a call. `rbx` and `r12` to `r15` hold values that are, and are saved in the function's frame when used. Values that find no register are spilled to the frame. `rax`, `rdx`, `r10` and `r11` are scratch.
  - A growth call, `rt_grow_scan` or `rt_grow_noscan` according to whether the new buffer may hold pointers, takes the old pointer, the used bytes and the new size in `rax`, `rdx` and `r10` and returns the new pointer in `rax`. It preserves every other general register, so a value that cannot hold a pointer stays in `rcx`, `rsi`, `rdi`, `r8` or `r9` across it. A value that may hold a pointer is treated as live across a call: the routine saves the preserved registers below the scanned part of the stack, where the collector does not read them. The routine records the caller's return address and frame pointer like any runtime entry, and the old buffer stays reachable through the words it keeps in the scanned region.
  - Float register values are allocated from `xmm2` to `xmm15` by the same scan. No `xmm` register survives a call, so a float live across a call is spilled, and spilled floats are not in the stack maps. A function that uses vectors gets no float registers, so its float operations go through `xmm0` and `xmm1`.
- Register preferences:
  - A value used as a call argument prefers that argument's register, a parameter prefers the register it arrives in, and a copy and its source prefer each other's registers.
  - The result of a two-operand add, subtract, multiply, and, or, xor, shift or float divide prefers the register of its first operand, and for a commutative operation of its second, so the instruction can work in place. Where it cannot share, it avoids the register of the operand it still has to read.
- Wrapped saves:
  - A function whose calls all lie below one block, the nearest common dominator of its call blocks, saves the callee-saved registers it uses at the head of that block instead of in the prologue and restores them only on returns reached through it.
  - The paths that skip the block use no callee-saved register, have no calls or polls and are at most 8 instructions, copies included, where they rejoin the shared tail, which is duplicated for them.
  - A value defined on a skipping path and live into the block is copied at the end of that path into a fresh value, and the block's head copies it into the original. The original lives only below the block, so a loop counter initialised on the light path stays in a caller-saved register.
  - A value defined in the region below the block and live into it that crosses a call is given a fresh value there.
  - Save slots keep their frame positions, and the stack maps name every save slot at every safepoint.
  - A vector function is not wrapped. The block and the tail are found with the same dominator tree that the optimiser uses.
- Frame modes: a function has one of three.
  - A full frame is built in the prologue.
  - A function with a wrapped region builds its frame, `push rbp`, `mov rbp, rsp` and `sub rsp`, at the head of the block that holds the saves, and the paths that skip the block return with a bare `ret`. This needs every skipping block to be free of anything that reads or writes the frame: no spilled value, frame address, slot access, parameter beyond the sixth, poll, panic or return wider than two words.
  - A function with no calls, polls, panicking checks, frame slots or spilled values and no callee-saved register has no frame at all and contains no safepoint.
- A frame slot's address is not held in a register: a load or store through it addresses `[rbp+offset]` directly and any other use computes the address with `lea` into a temporary where it is needed. A spilled value copied from another value whose live range it does not overlap shares that value's slot, so splitting a value at the wrapped block adds no slot-to-slot copy.
- A call result in `rax` or `rdx` that is read once, by the instruction directly after the call, stays there and is not moved. The immediate consumers are integer arithmetic and logic, a store of the value, call arguments, the return, a branch condition and a phi copy. A result word nothing reads is not moved at all.
- A block that only returns, with at most four predecessors that end in a jump, is copied into them, so each returns without a jump.
- A return restores with `leave`, or `pop rbp` in a frame of size 0. The last instruction of a returning block writes `rax` directly when it defines the first result and nothing else reads it. Up to two results are placed with a parallel move.
- Instruction selection:
  - An add or subtract of a constant or a register into a different register is `lea`.
  - A multiply by a constant is the three-operand `imul`.
  - A commutative operation takes whichever operand already sits in the destination.
  - A compare with 0 is `test`. A compare of a memory operand with an immediate reads the memory directly.
  - The constant 0 is `xor r32, r32` where the flags are dead.
  - A 32-bit add or subtract into a different register is a `lea` with a 32-bit destination and 64-bit address registers.
  - A move that undoes the move before it is dropped.
- The poll check is `cmp byte ptr fs:[-8], 0` and `jne` to an out-of-line stub that calls `rt_poll` and jumps back to a label placed directly after the `jne`.
- Alignment:
  - Every function and every C callback thunk starts on a 32-byte boundary.
  - A program for Linux is assembled with branch alignment, which pads so that no jump, and no fused compare and jump, crosses or ends at a 32-byte boundary. Hot patches are assembled without it.
  - A loop that is a single back-edge block, ends in its own conditional jump and has at most four instructions and phi copies besides the poll, if any, and the compare starts on a 32-byte boundary, so the whole loop sits in one window and needs no padding inside.

Calling convention:

- The first six argument words go in `rdi`, `rsi`, `rdx`, `rcx`, `r8` and `r9`. The rest go in an outgoing area at the bottom of the caller's frame, so `rsp` does not move inside a function.
- Up to two result words return in `rax` and `rdx`. Wider results are written to the outgoing area after the stack arguments.
- Argument and parameter registers, general and `xmm`, are filled with a parallel move that breaks cycles through `r11`.

### Vectors

- A vector local lives in a frame slot, like a variable passed as `mut`, and a vector expression evaluates to the address of a slot holding its result.
- A vector operation is one IR instruction, `vector`, taking the destination and operand addresses, or `vectorscalar` for `mask`.
- At calls, returns, fields and other aggregates a vector is its words, loaded from or stored to a slot. A `let` of a computed vector adopts the slot of the result instead of copying it.
- A function containing vector instructions is compiled twice. Its symbol is a stub that jumps to the AVX2 version when the CPU and the operating system support AVX2, and to the SSE2 version otherwise. `rt_cpu_init` checks `cpuid` and `xgetbv` at startup.
- `LOW_SIMD=sse2` in the environment selects the SSE2 versions, which the fixture runner uses to test both.
- The AVX2 version uses three-operand VEX instructions on `ymm` registers. A slot written once and read only by vector instructions later in the same block, with no call in between, is kept in `ymm8` to `ymm15` instead of memory. `vzeroupper` runs before every call and return of such a function.
- The SSE2 version splits 256-bit operations into two 128-bit halves and keeps every vector in memory.
- An operation without an instruction in the version being generated runs as a scalar loop over the lanes in a scratch area of the frame. These operations are `pshufb` on SSE2, 64-bit multiplication, minimum, maximum and comparison where unavailable, byte shifts and arithmetic 64-bit shifts. Unsigned comparisons flip the sign bit of both operands and compare signed.

## Hot reload

`low dev app.low` or `low dev app/` needs a Linux host. It builds a dev binary, runs it, and watches every source file of every loaded module, every embedded file, and the file lists of their directories. On save:

- A file that fails to compile is reported and the running program is left alone.
- The program restarts when any of these changes:
  - the set of linked libraries;
  - a struct or enum layout;
  - an existing function's signature;
  - a closure's captured variables;
  - `main`.

  A running frame of old code may still call any function, so it must keep the old calling convention and data layout. `main` never returns to pick up new code.
- Otherwise every function whose generated assembly differs from the running version is recompiled, including callers whose code changed only because a callee did, every instance of an edited generic function, and new functions. Heap, stack and all live values are kept.

Mechanism:

- Dev builds call every function through `fn_table`, a writable table of 4096 code addresses, with `call [rip+fn_table+8*i]`. Release builds call directly.
- In dev builds a function value's code word is the address of its `fn_table` entry and calls through it use `call [r10]`, so a stored function value or closure runs the newest code. Callback thunks call through the table too.
- `internal/amd64` owns this protocol: `GenerateHot` and `GeneratePatch`, the `fn_table` size, the patch signal and entry size, the mailbox symbol names and the region's base and size.
- At start the dev runtime installs a `SIGUSR1` handler and maps a 64 MB read-execute region at `0x40000000`, close enough to the binary for 32-bit relative references.
- A patch is assembled and linked in process, by `Built.Patch`, at the next free address in that region, resolving its references against the symbols of the running binary that `compiler.Build` returned. It holds only the changed functions and their strings. Patched functions get a fresh symbol suffix so they never collide with the originals.
- `low dev` writes the machine code into the region through `/proc/<pid>/mem`, writes `(index, address)` pairs and a count into a mailbox in the program, and sends `SIGUSR1`. The handler stores each address into `fn_table` with an aligned 8-byte write and clears the count, which `low dev` waits for.
- Frames already running keep executing their old code; the next call through the table enters the new code. Old code is never unmapped.

## UI groundwork

The UI library is staged under `ui/` until it moves into the standard library as `std/ui`.

- `ui/protocols` holds the Wayland protocol XML the library is built from: `wayland.xml` and `xdg-shell.xml`.
- `go run ./cmd/wlgen -o wayland.low ui/protocols/wayland.xml ui/protocols/xdg-shell.xml` generates Low bindings for libwayland-client.
  - Every interface given in one run shares an `Interfaces` record of `wl_interface` tables that `interfaces()` builds in C memory. A reference to an interface that no given protocol declares is an error.
  - A request becomes `interface_request(...)` over `wl_proxy_marshal_flags`, taking the `Interfaces` record when it creates an object of a known interface. `wl_registry_bind` takes the interface table and version.
  - Destructor requests pass `WL_MARSHAL_FLAG_DESTROY`, and an interface without a `destroy` request gets one that calls `wl_proxy_destroy`.
  - Objects are `ptr`, strings `ptr[u8]`, `fixed` an `i32`.
  - Events become an `extern struct` listener of `extern fn` fields, each defaulting to a generated function that ignores the event, so a listener names only the events it handles. `interface_add_listener` copies it into C memory. Callbacks receive the listener's `data` pointer, usually a pinned Low object.
  - Enum entries are functions such as `wl_shm_format_argb8888() -> u32`. Names that clash with Low keywords or builtins gain a trailing `_`.
- `ui/shaders` holds GLSL for instanced quads with per-corner radii, borders and clipping, blurred shadows, and monochrome glyph sprites sampled from an `R8` atlas. Instances are read from a storage buffer at set 0 binding 0 and the viewport size is a push constant. `go run ./cmd/shadergen ui/shaders` compiles each stage with `glslc` to a checked-in `.spv` and records the SHA-256 sums of the sources and the `.spv` files in `shaders.sum`.
- `ui/registry` holds the Vulkan registry, `vk.xml.gz` from Vulkan-Docs v1.4.363, and `vulkan.list`, which selects extensions, commands and extra types such as `pNext` structures.
  - `go run ./cmd/vkgen -registry ui/registry/vk.xml.gz -select ui/registry/vulkan.list -o vulkan.low` generates Low bindings for the selected commands and every type they reach, skipping Vulkan SC variants. Enum values come from the core enums, every Vulkan feature level and the selected extensions.
  - A handle is a one-field `extern struct`, `ptr` for dispatchable handles and `i64` for the others, so `VkBuffer {}` is the null handle and handles of different kinds do not mix.
  - Structures and unions become `extern struct`s whose fields all have defaults: zero, `null`, the empty handle, or for `sType` the structure's own value, so a literal names only what it sets. A union takes the layout of its widest member.
  - A fixed C array becomes numbered fields, `name_0` to `name_N`, with an accessor `Struct_name(s)` that returns them as a slice, or as a `str` for a character array.
  - Vulkan types map to `u32` (`uint32_t`, `VkBool32`, `VkFlags`, 32-bit flag bits), `i32` (`int32_t`, enums), `i64` (`uint64_t`, `VkDeviceSize`, `VkFlags64`, `size_t`), `u16`, `u8`, `f32` and `f64`.
  - Pointers to known types become `ptr[T]`; `void*`, function pointers and pointers to pointers become `ptr`. A command's single input or output through a pointer is a `mut` parameter; an array or optional pointer is a `ptr[T]`.
  - Enum entries and API constants are functions such as `VK_FORMAT_R8G8B8A8_UNORM() -> i32`, and `vk_make_api_version` builds version numbers.

## Tests

`go test ./...` runs every test whose tools are installed. A test that needs a tool outside the Go toolchain skips when the tool is missing, except `TestRunWindows`, which fails under `CI=true` when `wine` is missing. The workflow installs `wine` for it and sets `CI=true`. The `internal/asm` and `internal/link` oracle tests need the pinned binutils and LLVM below, which the workflow does not install, so they run by hand.

### Fixtures

A fixture program is a `.low` file or a directory with a `main.low`. Its expectations sit beside it in sidecar files named after it: `name.low` and `name/` both take `name.out`. `internal/fixture` reads and writes the sidecars; `internal/fuzz/corpus` writes fixtures through it and `internal/fixture/fixturetest` lists programs for the tests.

- `testdata/run` holds programs that run on Linux:
  - `.out` is the expected stdout, empty when absent.
  - `.err` is the expected stderr. A program has one exactly when it exits with a panic status.
  - `.maxrss` is a limit on peak memory in KB.
  - `.maxtime` is a limit on the wall-clock time of one run, as a Go duration such as `1s`. Only programs whose point is speed have one. A program with a `.maxtime` or `.maxrss` runs on its own, not alongside the others.
  - `.needed` lists the shared libraries the binary must link, one per line in sorted order. Without it the binary must be static with no interpreter. A program with one has no Windows build.
  - `.tags` declares what the runner cannot learn from the program: `simd` marks a program that compiles to vector code and runs a second time with `LOW_SIMD=sse2`, and `linux-only` marks a program that uses operating system services the Windows standard library lacks, such as signals, pseudo-terminals, inotify, Unix sockets, file modes or `/proc`, and has no Windows build. The runner fails when `simd` and the compiled code disagree.
- `testdata/windows` holds programs that only run on Windows, with `.out` and `.err`.
- `testdata/fail` holds programs that must fail to compile with the message in `.err`, with file paths shown relative to the fixture.
- `testdata/lint` holds programs whose `low lint` warnings must equal `.warn`, one per line with relative paths. A program without a `.warn` file has none.
- `bench/<name>/main.low` are the benchmark programs; the compile benchmark and the determinism test use them.

### Checks

- `go test ./...` compiles every `testdata/run` program for Linux, runs it with a one-minute limit, compares stdout with `.out` and stderr with `.err`, and applies `.maxrss`, `.maxtime`, `.needed` and `.tags`. These tests skip on a Windows host, as do the `internal/hot` tests.
- `TestRunWindows` builds every `testdata/run` program that has a Windows build, and every `testdata/windows` program, for Windows and compares the same way. It runs them directly on a Windows host and under `wine` on Linux.
- Every `testdata/fail` and `testdata/lint` program is checked against its sidecar.
- The time zone programs read zone rules from the embedded `std/time/tzdata`, because host databases differ by release and by build (Debian's includes `backzone`). They read the host database only for the behaviour of `zone.named` and `zone.local`, through zones whose rules every build agrees on.
- The `std/cdp` programs print `launch failed` when no browser starts. CI turns off Ubuntu's AppArmor restriction on unprivileged user namespaces so Chromium can create its sandbox.
- `cmd/low` compiles each program twice in one process and requires identical SSA form and assembly: every `testdata/run` program for Linux release, every benchmark for Linux release, Linux dev and Windows, and every `testdata/windows` program for Windows.
- `internal/compiler` compiles inline programs, benchmarks and fixtures and checks what the optimiser and code generator did with them: inlining, specialisation, bounds checks, register use, string comparison, switch lowering and allocation.
- `internal/wlgen` tests the generated signatures and checks, then builds a registry client from the vendored protocols and, when a Wayland compositor is running, runs it.
- `internal/vkgen` tests declaration parsing, extension enum values and the selection format, checks the shape of the generated bindings, and renders a rounded, bordered quad offscreen with the quad shaders through the bindings and dynamic rendering, comparing pixels. It skips without libvulkan or a Vulkan 1.3 device. Both generators find their libraries through `link.SystemDirs`.
- `internal/shaders` checks that the sources and the SPIR-V match the sums recorded in `shaders.sum` and validates the SPIR-V with `spirv-val` when installed.
- `internal/hot` tests reload against running programs:
  - state kept across a code swap;
  - functions added;
  - compile errors leaving the program running;
  - heap data surviving a reload while the collector runs;
  - stored function values, closures and callbacks running patched code;
  - instances of an edited generic function patched;
  - restarts on layout, signature and capture changes.
- `cmd/lowfuzz` generates well-typed, terminating programs from a choice stream (`internal/fuzz/gen`), predicts their output with a reference interpreter (`internal/fuzz/model`), compiles each for release and dev, runs both, and reports any of these (`internal/fuzz/oracle`):
  - compiler panic;
  - rejected program;
  - link failure;
  - timeout;
  - release/dev difference;
  - difference from the prediction.

  The generator covers every integer width, `f32` and `f64`, shifts, conversions, `if` expressions, functions, and `for` loops with labelled `break` and `continue`. Float values are observed through clamped integer conversions and NaN comparisons.
- `internal/fuzz/reduce` minimises each new signature. It removes functions, parameters and statements, unwraps `if` and loop bodies, replaces expressions with literals or their operands, and shrinks literals, keeping only candidates that still produce the signature. `-reduce-tests` bounds the compiler runs.
  - The minimised program is saved under `-out`, one per distinct signature.
  - With `-fixtures` it is also written there as a `testdata/run` fixture (`.low` with the predicted `.out` and `.err`, named by content hash) that fails until the bug is fixed.
- `internal/asm` tests assemble every program with `as` or `clang` and require the same object bytes. `internal/link` tests link programs with `ld` or `lld-link` and require the same segments, sections, symbols and loaded bytes for ELF and the same bytes for PE. `LOW_LINK_FULL=1` covers every `testdata/run` program and benchmark.
  - The padding and relaxation choices these tests compare differ between tool releases, so `internal/oracletool` pins GNU binutils 2.47 and LLVM 23.1.4. The tests skip when a tool is missing or reports another version.

## Roadmap

1. Standard library, written in Low:
    1. Vector values in registers across blocks and calls, and an AVX-512 version.
    2. QUIC (RFC 9000, 9001 and 9002) over UDP with the TLS 1.3 handshake, loss recovery and congestion control; HTTP/3 (RFC 9114) with QPACK (RFC 9204).
    3. WebRTC data channels: ICE with STUN, DTLS, SCTP over DTLS and the data channel establishment protocol; media over SRTP.
    4. Cryptography:
        1. Random generation: a ChaCha20 CSPRNG seeded from `getrandom` for `crypto.random`, and AES-256-CTR-DRBG (NIST SP 800-90A).
        2. Post-quantum signatures: ML-DSA-44, ML-DSA-65 and ML-DSA-87 (FIPS 204), SLH-DSA (FIPS 205), and Falcon (FN-DSA).
        3. HPKE (RFC 9180) with the DHKEM(X25519) and X25519MLKEM768 KEMs, HKDF-SHA-256 and the AES-256-GCM and ChaCha20-Poly1305 AEADs.
        4. Shamir's secret sharing over GF(2^8).
        5. VOPRF (RFC 9497) over ristretto255 and P-256.
        6. Zero-knowledge proofs: Groth16 over BN254 and BLS12-381, Halo2 with KZG and IPA commitments, and STARKs over FRI.
        7. Homomorphic encryption: BFV, CKKS and TFHE.
        8. Certificate status: OCSP (RFC 6960) and CRL checks in `std/crypto/x509` chain validation, and OCSP stapling in `std/tls`.
        9. ACME (RFC 8555): account keys, orders, HTTP-01 and TLS-ALPN-01 challenges, and renewal before expiry, feeding certificates to `std/tls` servers.
    5. DNS: search domains and `ndots` from `/etc/resolv.conf`, TCP fallback on truncated replies, CNAME chasing, and MX, TXT, SRV, CAA and PTR lookups over a message encoder and decoder.
    6. Hostnames: IDNA (UTS #46) with Punycode (RFC 3492), and an embedded public suffix list that the HTTP client's cookie jar checks domains against.
    7. gRPC: unary and streaming clients and servers over the HTTP/2 stack with status codes, deadlines, metadata and compression; `std/otel` exports through it.
    8. `std/protobuf`: `serialize[T]` and `deserialize[T]` through serde, and a `.proto` parser that generates Low types.
    9. Database drivers: MySQL and MariaDB over the client/server protocol, plain or over TLS; MongoDB over `OP_MSG` with `std/bson`.
    10. Messaging clients: MQTT 3.1.1 and 5, AMQP 0-9-1, Kafka and NATS.
    11. SSH: the transport (RFC 4253), user authentication with keys and passwords, the connection protocol with sessions and port forwarding, clients and servers, and SFTP.
    12. D-Bus: the wire format over Unix sockets, the session and system buses, method calls, signals and properties.
    13. System logging and time: syslog (RFC 5424) and the journald native protocol as `std/log` destinations; an SNTP client (RFC 4330).
    14. `std/html`: a WHATWG tree builder, CSS selector queries and an allow-list sanitizer.
    15. Markdown: CommonMark with GitHub tables, task lists, strikethrough and autolinks, rendered to HTML.
    16. Columnar and row formats: Avro with schema resolution and object container files; Parquet and Arrow IPC readers and writers.
    17. `std/unicode`: grapheme cluster, word and sentence segmentation (UAX #29) and line breaking (UAX #14) from embedded Unicode tables.
    18. Locales: CLDR plural rules, number, currency, date and time formatting, and collation (UTS #10); `std/time/humanize` and `std/text/number` take a locale.
    19. Numerics: statistics (mean, median, variance, standard deviation, percentiles, histograms), complex numbers, dense matrices with LU and QR decomposition and linear solves, and FFT.
    20. `std/file` memory-mapped files with `msync` and `madvise`; `std/os` user and group lookup, `getrlimit` and `setrlimit`, and cgroup v2 CPU and memory limits.
2. GPU UI library with GPUI's renderer and platform layer and a signal-based state model, Linux first:
    1. A Wayland window through libwayland-client, with bindings generated from protocol XML; a Vulkan swapchain through `VK_KHR_wayland_surface`; a clear colour.
    2. A scene of GPU primitives (shadows, quads with rounded corners and borders, paths, underlines, monochrome, subpixel and polychrome sprites) drawn by GLSL shaders compiled to embedded SPIR-V. Primitives are sorted by draw order, batched by kind and atlas texture, and clipped by content masks.
    3. Text: font discovery with an embedded fallback font, OpenType parsing, rasterizing into a glyph atlas, shaping, and line layout over `std/unicode` segmentation.
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
3. A wasm32 target and a browser playground that runs snippets:
    1. Targets named by OS and architecture, with `wasm32` beside `amd64`.
    2. A WebAssembly back end lowered from the IR: structured control flow rebuilt from the IR's blocks, locals in place of registers, a linear-memory heap with the size-class bins, and a module written directly without an external assembler or linker.
    3. A browser host: `print` and the clock as JavaScript imports; modules that need the OS (files, sockets, processes, the UI library) are rejected at compile time for this target.
    4. The compiler built for `js/wasm`, so the page compiles and runs a snippet without a server.
    5. A Run button on every documentation snippet, and an editable playground page on the site.
4. Standard library and tools on Windows:
    1. `__syscall` rejected at compile time on the Windows target, so a module without a Windows implementation fails the build.
    2. Overlapped reads and writes for pipes and files, so they park the green thread instead of holding an OS thread, and job objects for process groups.
    3. Console control events for `std/os/signal`, `ReadDirectoryChangesW` for watches, and pseudo-consoles for `open_pty`.
    4. `low dev` on a Windows host.
5. A package registry:
    1. Dependencies declared in `low.mod` with a version each, a `low.sum` lockfile holding exact versions and content hashes, and a module cache that imports resolve to.
    2. Packages published from tagged git repositories, each version immutable once indexed, and yanking in place of deletion.
    3. A package page on the website for every indexed version, generated with `low doc` as the standard library pages are.
