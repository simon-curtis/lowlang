# Low Design

Low is a small compiled language, about the size of Go's core. It has a tracing garbage collector, green threads with channels, enums that carry data and typed errors. Programs build to static native binaries that do not depend on libc.

| Command | Does |
| --- | --- |
| [`low run`](#low-run) | Compiles and runs a program. |
| [`low test`](#low-test) | Runs the tests of the named modules. |
| [`low build`](#low-build) | Writes an executable or a WebAssembly module. |
| [`low install`](#low-install) | Builds programs and places them in `LOW_BIN`. |
| [`low update`](#low-update) | Moves every dependency to the newest version its range accepts. |
| [`low publish`](#low-publish) | Adds a tagged version of a package to the package index. |
| [`low yank`](#low-yank) | Marks a published version as yanked. |
| [`low dev`](#low-dev) | Runs a program and patches changed functions into it on save. |
| [`low ir`](#low-ir-and-low-asm) | Prints the optimised SSA form. |
| [`low asm`](#low-ir-and-low-asm) | Prints the generated assembly. |
| [`low lint`](#low-lint) | Type-checks a program or a library module and prints its warnings. |
| [`low doc`](#low-doc) | Prints the public API of modules as JSON. |
| [`low names`](#low-names) | Prints every name in a file as JSON. |
| [`low explain`](#low-explain) | Prints the meaning of a diagnostic code. |
| [`low prof`](#low-prof) | Runs a program and reports where its cycles go. |
| [`low abbench`](#low-abbench) | Compares two builds of a program on alternating runs. |
| [`low protogen`](#low-protogen) | Generates Low modules from `.proto` files. |
| [`low lsp`](#low-lsp) | Starts the language server that the editor extensions use. |
| [`low version`](#low-version) | Prints the release. |
| [`low help`](#low-help) | Prints the commands, or the options of one command. |

A directory is a module. A `build.low` file marks the project root and names its dependencies, and `import ui/text` loads `ui/text/` under it. The standard library is imported as `std/...`. The editor extensions need `low` on `PATH`.

Build from source on Linux with `curl` and `sha256sum`: `bootstrap.sh` downloads the release that `bootstrap.txt` names, checks its SHA-256, builds the compiler with it three times and writes the result to its first argument.

```
$ git clone https://github.com/simon-curtis/lowlang
$ cd lowlang
$ ./bootstrap.sh ~/.local/bin/low
```

## Commands

`low help` prints the commands. `low help <command>` and `low <command> --help` print the options of one command.

### Command lines

Command lines follow the POSIX utility syntax guidelines with GNU long options.

| Form | Meaning |
| --- | --- |
| `-o path`, `-opath` | A short option is a dash and a letter. It takes the rest of its argument or the next one as its value, so `-o=path` has the value `=path`. |
| `-vj4` | Short switches combine. |
| `--name value`, `--name=value` | A long option. |
| `--list` | A switch takes no value, so `--list=false` is an error. |
| `--` | Ends the options. |
| `-` | A lone dash is an operand. |

- An argument that does not parse as options is reported whole, as `unknown option -vx`.
- A value that is given and empty is a value, so `--target=` and `-c ''` are errors for an unknown target or configuration.
- Options and operands may be interleaved for every command except `low run`, `low prof` and `low abbench`. For those three, options end at the program or command, and later arguments belong to it.

Some options are shared between commands.

| Option | Short form | Taken by |
| --- | --- | --- |
| `--target` | | `build`, `run`, `install`, `asm`, `ir`, `lint`, `doc` and `names` |
| `--config` | `-c` | `build`, `run`, `install`, `asm` and `ir` |
| `--output` | `-o` | |
| `--help` | `-h` | |
| `--version` | `-V` | `low` itself |
| `--verbose` | `-v` | `low test` |
| `--jobs` | `-j` | `low test` |

Help goes to standard output with status 0, in colour when `term.colour_enabled(1)` holds. `low`, `low -h`, `low --help` and `low help` print the commands; `low help <command>` and `low <command> --help` print the options of one command.

| Exit status | When |
| --- | --- |
| 0 | The command succeeded, or printed help. |
| 1 | Every failure that is not a mistake on the command line. |
| 2 | A mistake on the command line (an unknown command or option, a missing or invalid value, a wrong number of operands, `--subsystem` for a target that is not `windows/amd64`). |

A mistake on the command line prints three lines to standard error: one naming the mistake, the command's `Usage:` line and a `Try 'low <command> --help'` line. An unknown command prints its line and `Try 'low --help'`.

One table in `compiler/low/commands.low` holds each command's name, summary, operands, description and options. The parser, both help pages and the usage line all read it.

### low run

Compiles and runs a file or a module directory, `.` by default.

```
$ low run -c debug app/ -v
```

- Options come before the program. Every argument after the program goes to the program, so the `-v` above goes to `app`.
- With none named, a directory that holds no `.low` file runs the one program its project declares (see [Projects](#projects)).
- `--target browser/wasm32` runs it as WebAssembly under `node`.
- `-c`, `--trace`, `--json` and `--max-errors` work as they do for [`low build`](#low-build). `--json` covers the compilation, not the program's output.

### low test

Builds the tests of the named modules and runs each test in its own process, in parallel, with a timeout, filters and a text or JSON report.

### low build

Writes a native binary, or a WebAssembly module for `browser/wasm32`, from a file or a module directory, `.` by default. Options may come before or after the program.

| Option | Meaning |
| --- | --- |
| `-o`, `--output` | Names the output. |
| `--target` | Picks `linux/amd64`, `windows/amd64` or `browser/wasm32`. Defaults to the host. |
| `-c debug`, `--config debug` | Builds without optimisation. |
| `-c release` | Optimises. This is the default. |
| `--subsystem console\|gui` | Picks the PE subsystem of a Windows executable. |
| `--trace file.json` | Writes a Chrome trace of the compilation. |
| `--map file` | Writes the address of every symbol. |
| `--json` | Writes each error and warning to standard error as one JSON object per line (see [JSON diagnostics](#json-diagnostics)). |
| `--max-errors n` | Stops after `n` errors, 50 by default (see [Compile errors](#compile-errors)). |

**Which program is built.** With no program named, the current directory is built when it holds `.low` files. Otherwise the one program the project declares is built, an error naming them when there are several.

**Where the output goes.** Without `-o`, or with an `--output` that names a directory or ends in a separator, which creates it, the output is named after the file or directory and written to that directory, the current one by default, so `low build` in `app/` writes `app`, or `app.exe` on Windows.

**`--subsystem`** overrides the project's `gui` declaration. Without it, the subsystem is `gui` for a program the project's `build.low` declares with `gui`, and `console` otherwise. The option is an error for another target.

**`--trace file.json`** writes the compilation's stages, each parallel job's thread and the process's CPU and memory as a Chrome trace. It prints a stage table and a thread usage matrix to standard error. `low run`, `low asm` and `low ir` take it too.

**`--map file`** writes every symbol outside `.L` with its address in the image at its preferred base. Each symbol gets one `hex-address name` line, sorted by address. Absolute constants carry their value, and negative ones carry it as `-hex`. The executable is the same with or without `--map`.

### low install

Builds each named program and places its executable in `LOW_BIN` or `~/.low/bin`. The executable is named as `low build` names it. It prints `installed <path>` on standard output for each.

- `--target` names the host and `-c` picks the configuration, as for `low build`. `--subsystem` is a usage error.
- With no program named, it builds `.` when that directory holds `.low` files. Otherwise it builds every program the project declares (see [Projects](#projects)), each named after its directory.
- An executable that cannot be replaced is first renamed to `<name>.old`, and the next install removes that file. A running executable on Windows, for example, cannot be replaced.

### low update

Resolves every version-control dependency again to the newest tag its ranges accept. It places each in the module cache and rewrites `build.lock` with each version, commit and content hash.

### low publish

```
$ low publish <source> <version>
```

Adds a tagged version of the package at `https://<source>` to the package index and commits it there (see [Projects](#projects)).

- The tag must be pushed.
- Its tree must resolve its dependencies and type-check.
- A version the index lists is refused.

### low yank

```
$ low yank [--undo] <source> <version>
```

Marks a version in the package index as yanked, or with `--undo` as available again. It changes only the flag of its line and commits the change.

### low dev

Runs the program and patches changed functions into it on save. Changing `main`, a type's layout or a function's signature restarts it. [Hot reload](#hot-reload) describes how.

### low ir and low asm

`low ir` prints the optimised SSA form. `low asm` prints the generated assembly. Both take `--json` and `--max-errors` as [`low build`](#low-build) does.

### low lint

Type-checks a program or a library module and prints its warnings. `--json` writes them as [JSON diagnostics](#json-diagnostics), and `--max-errors` works as it does for [`low build`](#low-build).

### low doc

Prints the public API of every module under a directory, of the standard library or of every indexed package version as JSON.

### low names

Prints every name in a file as JSON, with its hover text, definition and reference group.

### low explain

Prints the severity, meaning and example of a diagnostic code, the word in brackets that ends an error or warning. `low explain break-outside` prints the `break-outside` row of the table in [Diagnostic codes](#diagnostic-codes).

### low prof

Runs a program and reports where its cycles go, by category, by function and by function with its callees. Options come before the program and every argument after it goes to the program.

| Option | Meaning |
| --- | --- |
| `--folded` | Writes the stacks for flame graph tools. |
| `--callers` | Lists the call chains that reach one function. |
| `--event` | On Linux, picks what is sampled: `cycles`, `instructions`, `cache-misses`, `branch-misses`, `cpu-clock`, `faults` or `switches`. |
| `--map` | On Windows, the map that `low build --map` wrote. A Windows program needs it. |

On Linux, every process the program starts is sampled with it, and each is named from its own symbol table. A Windows program is sampled by cycles.

A program named without a directory is found through `PATH`, as `std/process` finds it.

### low abbench

Runs one command with each of two programs, `-a` and `-b`, on alternating runs. Options come before the command.

It reports the median and spread of each side, and the median of the paired `b/a` ratios with a 95% bootstrap interval. It reports these for wall time, CPU time, cycles, instructions and peak memory. Peak memory is private on Windows and resident on Linux. It leaves out a metric the host does not count.

| In the command or options | Meaning |
| --- | --- |
| `{exe}` | The program of each side. |
| `{side}` | `a` or `b`. |
| `--same` | Requires a file both sides write to be identical. |
| `--affinity` | Restricts the CPUs. |

### low protogen

Generates one Low module for each `.proto` file named and each file it imports with `std/protobuf/proto`. It writes them under the output directory, `a/b.proto` as `a/b/b.low`. A failure to read, generate or write exits with status 1.

| Option | Meaning |
| --- | --- |
| `-o`, `--output` | The output directory. |
| `-I`, `--include` | The comma-separated directories that imports are resolved against, `.` by default. |
| `--module` | The import path of the output directory when it is not the project root. |

### low lsp

Starts the language server that the editor extensions use. [Editors](#editors) covers the extensions.

### low version

Prints the release.

| Build | Prints |
| --- | --- |
| A release | The release. |
| From source | `dev+` and the first 12 digits of the commit `std/buildinfo` reports, then ` ~ ` and the build time in UTC as `YYYY/MM/DD HH:MM`: `dev+0e21bead73d7 ~ 2026/11/18 16:01`. |
| Outside a git checkout | `dev`. |

### low help

Prints the commands, or with a command name the options of that command.

## Goals

- Compile speed and binary size comparable to Go.
- Runtime speed comparable to Go.
- Exhaustive enums with payloads.
- Errors as values, propagated with one character, impossible to ignore.
- A small language: one way to loop, one way to declare, data and functions with no methods or inheritance.

## Syntax

Newlines terminate statements. Blocks are expressions. The last expression is the value.

### Bindings

A binding gives a value a name. `let` is immutable, `var` is mutable.

```low
let name = "lowlang"
var count = 0
count += 1
```

Types are inferred and may be written as `let x: i64 = 1`.

#### Struct patterns

A struct pattern binds several fields of one value at once.

<!-- prelude
struct Point {
    x: i64
    y: i64
}
struct Rect {
    min: Point
    max: Point
    label: str
}
let cells = [Point { x: 1, y: 2 }]
let rect = Rect { min: Point { x: 0, y: 0 }, max: Point { x: 4, y: 3 }, label: "box" }
-->
```low
let { x, y } = rect.max
var { x: min_x, y: min_y } = cells[0]
var { x: max_x, y: max_y } = cells[0]
let { min: { x: left }, max, label } = rect
```

| Pattern | Binds |
| --- | --- |
| `{ x, y } = value` | Each listed field to a local of the same name. |
| `{ x: name }` | Field `x` to `name`, as in a struct literal. |
| `{ min: { x, y }, max }` | A field that is a struct nests a pattern. |

Commas or newlines separate entries. A pattern lists only the fields it wants.

`var` makes every bound name mutable and `let` makes them immutable. A pattern takes no type annotation, and `using` binds one name. `{` after `let` or `var` always starts a pattern.

The value is evaluated once, then each field is read from it.

These are errors:

- a field named twice
- two entries binding one name
- an empty `{}`
- a field the struct lacks
- a value that is not a struct; a pointer counts as not a struct

A struct with `drop` cannot be destructured.

### Module constants

A constant is a named value the compiler works out at compile time. `const name: T = value` declares a constant at the top level of a module.

```low
/// The width that stands for no limit.
pub const unbounded: f32 = 1.0 / 0.0
const kib = 1 << 10
const mib = kib * kib
const greeting = "hello, " + name
const name = "low"
const ok = 200

fn status(code: i64) -> str {
    match code {
        ok => "ok"
        404 => "not found"
        _ => "other"
    }
}
```

A constant is a number, a `bool` or a `str`. Without a type, it takes the type `let` would infer. An integer literal is `i64` and a float literal is `f64`.

Its value is built from literals, other constants, unary and binary operators and numeric conversions such as `u8(x)`. The compiler evaluates it at compile time, with the wrapping, shifts, rounding and conversions of the same expression at run time.

These are errors:

- division by zero
- a constant that depends on itself
- a float out of the range of `u64` converted to `u64`
- a constant that is assigned or passed as `mut`
- `const` inside a function body

Constants of a module may be declared in any order and in any of its files. Each use is the folded value.

`pub` exports a constant, read as `module.name`. A constant cannot share its name with a function, type or builtin of its module. A local of the same name hides it.

In a `match` arm, a constant's name, or `module.name`, compares the subject with the constant instead of binding a new name.

### Functions

A function is declared with `fn`. Functions are also values: they can be stored, passed, returned and written in place as closures.

<!-- prelude
import std/slices
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
let words = ["low", "level"]
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
let cube = fn(x: i64) x * x * x
let lengths = words |> slices.map(fn(w) w.len)

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

#### Function types

A function type is written `fn(i64, mut str) -> bool`, with `! E` for a result that can fail. Two function types are the same when their parameters, `mut` markers and results are.

Values of function type can be stored in variables, fields, slices and maps, passed and returned, and called like functions. They cannot be compared or printed.

#### Closures

A named function is a value of its function type. A closure is a function written as an expression.

| Form | Meaning |
| --- | --- |
| `fn(params) -> R { body }` | A closure. |
| `fn(params) expr` | A closure whose body is one expression. That expression extends as far as an expression can. |

A closure that writes its result or error type takes a braced body, so `!` after the parameters always starts an error type.

A closure's parameter and result types may be omitted when the expected function type supplies them.

The compiler checks an argument to a generic function after the arguments that are not closures. Its expected type therefore takes the type arguments they bind. `slices.map(words, fn(w) w.len)` gives `w` the element type of `words`.

The result is the body's type, or the type of the first `return`'s value, in two cases:

- when the expected type leaves the result open, as `U` in `slices.map`
- when there is no expected type and the body is one expression

`?` and `fail` need a written result. A braced closure with no written result and no expected type returns nothing.

#### Captured variables

A closure captures the variables it names by reference, as Go does. It reads and assigns the variable itself. The variable lives as long as any closure holding it.

- Each loop iteration binds fresh variables, so closures made in different iterations see different ones.
- A `mut` parameter cannot be captured.

If a variable is never assigned after its binding, never passed as `mut` and never modified by `push`, `pop`, `truncate` or `delete`, it cannot be told apart from a copy, so the closure holds its value directly.

#### Method call syntax

`x.f(args)` calls `f(x, args)`, so calls chain left to right: `"ab".repeat(3).first()`.

The compiler looks for `f` in this order:

1. A struct field of function type named `f` takes precedence, so `op.apply(v)` calls the stored function.
2. `f` is any function callable by its plain name: a function in the current module, a function value in scope, a generic function or a builtin. A local variable or parameter named `f` that is not a function value is skipped. So `fn sized(width: f32) -> Box { box().width(width) }` calls the function `width`.
3. When no such name exists and `x` is a struct or enum, or a pointer to one, from another module, `f` is that module's public function. `url.authority()` calls `http.authority(url)`.
4. Otherwise `f` is the public function of that name in an imported module. Its first parameter accepts `x`. If that parameter is generic, `x`'s type meets its constraints.

Two examples of the last step: `words.map(fn(w) w.len).sum()` calls `slices.sum(slices.map(words, ...))`, and `"banana".count("an")` calls `text.count`. When more than one imported module has such a function, the call is an error naming them.

When the first parameter is `mut`, `x` is passed as `mut` and must be a mutable place.

#### The pipe operator

`x |> f(args)` calls `f(x, args)`, and `x |> f` calls `f(x)`.

The right side is a call, a function name, a module function such as `text.repeat`, a closure, or a struct field holding a function. Any other expression is an error. `x |> ops.scale` calls the stored function, never a function named `scale`.

- `|>` has the lowest precedence of the binary operators and groups left to right. `a + b |> f()` is `f(a + b)`.
- A trailing `?`, `catch` or `or` applies to the call: `x |> parse()?` is `parse(x)?`.
- A `mut` first parameter receives `x` as `mut`, as with `x.f(args)`.

A line that starts with `|>` or `.` continues the expression on the line before it.

### Top-level statements

Statements may stand at the top level of a file, among its declarations. In source order they form the body of the program's `main`, which takes no parameters.

```low
import std/text

enum Problem {
    Negative(i64)
}

fn checked(n: i64) -> i64 ! Problem {
    if n < 0 { fail Problem.Negative(n) }
    n
}

let words = ["low", "level"]
for w in words {
    print(w)
}
let n = checked(text.parse_int("12") or 0)?
print(n)
```

Only the entry module may have them, in one of its files. That module cannot also declare `fn main`. Under `low test` the tested project root may have them too. Its tests can call that `main`.

A top-level `let` or `var` is a local of `main`, so the module's functions cannot see it. A value the functions share is a `const` or a function.

Each top-level statement ends at a newline. `fn` followed by `(` starts a closure expression. Any other `fn` starts a declaration. `pub` and `///` apply only to declarations.

A final expression statement is the tail of `main`, as in any block, and its value is discarded.

Neither the statements nor a drop or field default can call or name `main`.

#### The error type of main

The first `?`, `fail` or `catch M` among the statements, outside closures, gives `main` its error type. Every later one must use the same enum. Without one, `main` returns nothing.

That first `fail` or `catch M` has no error type to check against, so it names the enum, as in `fail Problem.Negative(n)`.

### Control flow

`if` chooses between branches and `for` loops.

<!-- prelude
var n = 3
let xs = [1, 2, 3]
let grid = [[1, 2], [3, 4]]
let want = 4
struct Point {
    x: i64
    y: i64
}
let points = [Point { x: 1, y: 2 }]
-->
```low
if n > 0 { print(n) } else { print(0) }

for i in 0..10 { print(i) }
for x in xs { print(x) }
for i, x in xs { print(i) }
for { x, y } in points { print(x + y) }
for i, { x } in points { print(i + x) }
for n > 0 { n -= 1 }
for { break }

rows: for row, cells in grid {
    for col, cell in cells {
        if cell == want { break rows }
    }
}
```

| Loop | Does |
| --- | --- |
| `for i in a..b` | Counts from `a` up to but not including `b`. The bounds are integers of one type. |
| `for x in xs` | Visits each element of a slice. |
| `for i, x in xs` | Also binds the index as `i64`. |
| `for { x, y } in points` | Destructures each element with a struct pattern, as in `### Bindings`. |
| `for i, { x, y } in points` | Binds the index and destructures the element. |

Loop variables are immutable, and so are the names a pattern binds. In `for`, `{` starts a pattern only when its closing `}` is followed by `in`, so `for { break }` is still an infinite loop.

`break`, `continue`, `return` and `fail` are expressions of type never, so they can stand wherever a value is expected: after `??`, in an arm, or as a branch of `if`.

#### Loop labels

`name:` before `for` labels a loop. `break name` and `continue name` act on that loop from inside nested loops. They run the deferred statements and drops of every block they leave.

- A label must be used.
- It cannot repeat the label of an enclosing loop.
- It is not visible inside closures.

### Numbers

Low has six integer types and two float types. Conversions between them are always written out.

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

| Kind | Types |
| --- | --- |
| Integer | `i64`, `i32`, `u64`, `u32`, `u16` and `u8` |
| Float | `f64` and `f32` |

#### Number literals

Integer literals are decimal, or hexadecimal, binary and octal with `0x`, `0b` and `0o`. Float literals may have an exponent. In any number literal `_` may separate two digits, as in `1_000_000.0` or `0xFF_FF`. An `_` anywhere else is an error.

An integer literal takes the type the context expects, including the other operand of a binary operator, and must fit it; otherwise it is `i64`. A literal above the `i64` range is accepted only as a `u64`. In an `if` expression with no expected type, an integer literal branch takes the type of the other branch.

A float literal is `f64` unless an `f32` is expected.

#### Arithmetic and conversions

Operands of a binary operator have the same type. Conversions are explicit calls such as `u8(x)`, `f64(n)` and `i64(f)`. Integer conversions truncate or extend. A float to integer conversion truncates toward zero.

| | Integers | Floats |
| --- | --- | --- |
| Arithmetic | Wraps at the type's width. | Follows IEEE 754. |
| Division by zero | Panics. | Gives an infinity. |

`%` is integer only. Comparisons with NaN are false except `!=`.

`sqrt`, `floor` and `exp` take and return `f32` or `f64`. `print` accepts any integer or float type.

#### Bit operators

`&`, `|`, `^`, `~`, `<<` and `>>` work on integers. `>>` is arithmetic on signed types and logical on unsigned ones.

The shift count may be any integer type. A count of 64 or more, or a negative one, gives 0, or the sign fill for `>>` on a negative signed value.

#### Operator precedence

Binary operators bind as in Go, from tightest to loosest: `*`, `/`, `%`, `<<`, `>>`, `&`; then `+`, `-`, `|`, `^`; then `??`; then comparisons; then `&&`; then `||`.

So `a - b >> 1` is `a - (b >> 1)` and `x | y - z` is `(x | y) - z`.

### Vectors

A vector holds several numbers of one lane type, and its operators work on every lane at once.

```low
import std/bits
import std/simd

fn find(data: u8[], c: u8) -> i64 {
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

Vector types are 128, 256 or 512 bits of one lane type.

| Lane type | 128 bits | 256 bits | 512 bits |
| --- | --- | --- | --- |
| `u8` | `u8x16` | `u8x32` | `u8x64` |
| `u16` | `u16x8` | `u16x16` | `u16x32` |
| `i32` | `i32x4` | `i32x8` | `i32x16` |
| `u32` | `u32x4` | `u32x8` | `u32x16` |
| `i64` | `i64x2` | `i64x4` | `i64x8` |
| `u64` | `u64x2` | `u64x4` | `u64x8` |
| `f32` | `f32x4` | `f32x8` | `f32x16` |
| `f64` | `f64x2` | `f64x4` | `f64x8` |

Every type works on every CPU. Where the AVX-512 version is not selected, a 512-bit vector runs as two 256-bit or four 128-bit halves.

Vectors are values. They can be passed, returned, stored in fields and slices, and compared lane by lane. They cannot be compared with `==` as a whole. They print as `<1, 2, 3, 4>`.

#### Vector operators

`+`, `-`, `*`, `&`, `|`, `^`, `~` and unary `-` work lane by lane. `/` works on float lanes.

`<<` and `>>` work on integer lanes by a scalar count, arithmetic on signed lanes, and follow the scalar rule on every lane type. A count of the lane width or more, or a negative one, gives 0, or the sign fill for `>>` on a negative signed lane.

Compound assignments work the same way.

Comparisons work lane by lane and give a vector of the same type whose lanes are all ones or all zeros. Unsigned lanes compare unsigned.

A scalar operand of the lane type is broadcast, so `chunk == '"'` compares every lane.

`u8x32(v)` reinterprets a vector of the same width.

#### The simd module

`std/simd` provides the functions that work on vectors. Reads and writes are bounds-checked.

| Function | Does |
| --- | --- |
| `read` and `write` | Move between a vector and a slice at an element index. |
| `mask` | One bit per lane from its top bit. |
| `any` and `all` | Whether some or every lane has its top bit set. |
| `shuffle16`, `shuffle` and `shuffle64` | A byte table lookup within each 16-byte quarter of a `u8x16`, `u8x32` or `u8x64`, zero where the index has its top bit set. |

It also provides `read_str`, `splat`, `blend`, `min`, `max`, `lane`, `with_lane`, `find_byte` and `find_byte_str`.

### Slices

A slice is a growable sequence of values of one type. `T[]` is a pointer, a length and a capacity over garbage-collected memory. Assigning or passing a slice shares its elements.

```low
let xs = [3, 1, 4]
var squares: i64[] = []
squares.push(9)
let middle = xs[1..3]
xs[0] = 10
print(xs.len)
```

#### Slice types

`[]` after a type applies to everything before it, as `?` does.

| Type | Is |
| --- | --- |
| `*Node[]` | A slice of pointers. |
| `u8[][]` | A slice of slices. |
| `fn() -> i64[]` | A function returning a slice. |

`[]T` is the prefix spelling of `T[]`. The compiler's own source uses it, because the release in `bootstrap.txt` reads only that form.

Elements of `u8`, `u16`, `u32`, `i32` and `f32` are packed at their natural width; other elements take a word each.

#### Slice operations

| Form | Does |
| --- | --- |
| `[a, b]` | Builds a slice. |
| `[]` | An empty slice. It needs an expected type. |
| `xs[i]` | Reads or writes an element and panics when `i` is outside `0..xs.len`. Any integer type indexes. |
| `xs.len` | The length as `i64`. |
| `xs[a..b]`, `xs[..b]` and `xs[a..]` | Share the elements in that range. |
| `xs.push(v)` | Appends, growing the capacity to twice plus four when full. |
| `xs.pop()` | Removes the last element and returns it as `T?`, or `none` when `xs` is empty. |
| `xs.truncate(n)` | Sets the length to `n` and panics unless `0 <= n <= xs.len`. |

Slice builtins are called as methods: `xs.push(v)`, not `push(mut xs, v)`. A function in the program named `pop` or `truncate` takes precedence over the builtin.

In an `if` expression with no expected type, a branch that is an empty `[]` takes the type of the other branch.

#### Slice capacity

A range such as `xs[a..b]` gives a result whose capacity equals its length, so `push` onto it always copies and never overwrites the parent.

`pop` and `truncate` keep the capacity, so a later `push` reuses the freed space.

- They change only the binding they are given. A slice that shares its elements with another binding sees a later `push` overwrite the freed elements, as after any `push` that finds spare capacity.
- A reslice such as `xs = xs[..n]` is the form that copies on the next `push`.
- Elements past the length stay in the array, and keep what they point to alive, until a `push` overwrites them.

#### Constant slice and map literals

A slice literal or map literal whose elements are all constants is built once per program, at its first evaluation. It is reused.

| Use of the literal | Gets |
| --- | --- |
| Bound with `let`, when the binding is only indexed, iterated or asked for `.len` | The literal in place. |
| Indexed, iterated or measured directly | The literal in place. |
| Any other use | A copy. |

Any other use gets a copy because `push`, a slice of it, an argument, a return value, a field or an assignment through it would otherwise mutate the shared value.

A slice literal under 1024 bytes and a map literal under 4 entries are rebuilt for those uses, since copying them costs more than building them.

### Maps

A map looks a value up by its key. `[K: V]` maps keys of an integer type, `bool`, `str` or a pointer type to values of any type.

```low
let ages = ["ada": 36, "alan": 41]
ages["grace"] = 85
print(ages["bob"] ?? -1)
var counts: [str: i64] = [:]
counts["a"] += 1
counts.delete("a")
for name, age in ages { print("{name} {age}") }
for name in ages { print(name) }
for at, name, age in ages { print("{at} {name} {age}") }
```

A map is a pointer to a table on the garbage-collected heap, so assigning or passing it shares the table. `m[k] = v` works through any binding.

| Form | Does |
| --- | --- |
| `[k: v, ...]` | Builds a map. |
| `[:]` | An empty map. It needs an expected type. |
| `m[k]` | Returns `V?`. |
| `m[k] = v` | Inserts or replaces. |
| `m[k] += 1` | A compound assignment starts from a zero value when `k` is missing. |
| `m.delete(k)` | Removes a key; removing a missing key does nothing. |
| `m.len` | The number of entries. |
| `for k, v in m` and `for k in m` | Visit entries in insertion order. |
| `for i, k, v in m` | Also binds `i`, an `i64` that counts the entries visited from 0, so a deleted entry leaves no gap. Only a map loop binds three names. |

For `[:]`, an `if` branch takes that type from the other branch, as an empty `[]` does.

#### Map iteration order

Iteration follows insertion order:

- An overwritten value keeps its key's position.
- A deleted and re-inserted key moves to the end.
- Inserting new keys while iterating may skip or repeat entries.

#### Constant map literals

A map literal of constants is built once per program in a release build. A use that only indexes, iterates or measures it shares the built table. Any other use gets a copy from the prelude's `__map_clone`. That function copies the entry array and the index.

A literal of fewer than 4 entries is rebuilt for those uses, because the copy costs as much as the build.

#### Map layout

A map is an array of entries in insertion order plus an open-addressed index of one word per slot.

| Slot | Means |
| --- | --- |
| 0 | Empty. |
| -1 | Its entry was deleted. |
| Otherwise | The upper 32 bits of the key's 63-bit hash above the entry number plus one. |

A probe skips empty slots and slots of other keys without reading the entries. The map code reads the low bits of the hash as the position and the upper 32 bits as the tag.

Iteration reads the entries, never the index, so the hash function never shows in the order. No program output depends on a hash value.

#### Hash seed

Hashes are seeded per process, so no key set can be built to collide without the seed. Start-up fills `rt_hash_seed` with 128 random bits and forces the second word odd.

| System | Source of the bits |
| --- | --- |
| Linux | The 16 bytes at `AT_RANDOM`, or `getrandom` when the auxiliary vector has none, or the fallback when that fails. |
| Windows | `ProcessPrng`, with the same fallback. |

The fallback derives each word through its own SplitMix64 finalizer.

| Word | Comes from |
| --- | --- |
| `s0` | One RDTSC read, the stack address and one RDRAND word. |
| `s1` | A second RDTSC read taken after a `cpuid`, a rotated stack address and a second RDRAND word. |

RDRAND contributes only when CPUID reports it and the instruction succeeds within 10 tries.

#### Hash function

In the hash function, `s0` and `s1` are the two seed words. `mum(a, b)` is the high 64 bits of the 128-bit product of `a` and `b` xor its low 64 bits. `fold(h)` is `x = h ^ h >> 32` followed by `(x ^ x >> 16) >> 1`, which keeps 63 bits.

An integer, `bool` or pointer key `k` hashes to `fold(mum(k ^ s0, s1))`.

A `str` is hashed in these steps:

1. Every `str` starts from `acc = mum(s1 ^ length, s0 | 1)`, so the length reaches the hash only through a product with seed words.
2. A string longer than 16 bytes folds every 16-byte block while more than 16 bytes remain. Each fold computes `acc = mum(w0 ^ s0, w1 ^ acc)`. The hash then reads its last 16 bytes as the two words.
3. A `str` of 16 bytes or fewer reads two words `a` and `b`, as the table below sets out.
4. A `str` hashes to `fold(mum(a ^ s0, b ^ acc))`.

| Length in bytes | `a` and `b` |
| --- | --- |
| 8 to 16 | The first and last eight. |
| 4 to 7 | The first and last four packed into `a` with `b` zero. |
| 1 to 3 | The first, middle and last byte. |
| The empty string | Zero. |

Two strings of different lengths whose input words differ by a chosen xor do not collide under every seed, because cancelling the length term needs `mum(s1 ^ n, s0 | 1) ^ mum(s1 ^ n', s0 | 1)`, which depends on the seed. Collisions between distinct strings remain possible with probability about 2^-63 per pair. The hash gives no protection once `s0` and `s1` are known.

### Strings

A `str` is an immutable pointer and length over UTF-8 bytes. Strings are built with interpolation and `+`, and read by byte or by code point.

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

| Expression | Gives |
| --- | --- |
| `s.len` | The length in bytes. |
| `s[i]` | One byte, as `u8`. |
| `s[a..b]` | A byte range that shares the bytes. |
| `for c in s` | Each code point as `u32`. Invalid UTF-8 yields U+FFFD for one byte. |
| `for i, c in s` | Each code point with its byte offset. Invalid UTF-8 yields U+FFFD for one byte. |
| `'a'` | A code point of type `u32`, or `u8` where one is expected. |
| `bytes(s)` | A copy of the string as a `u8[]`. |
| `str(b)` | A copy of the bytes as a string. |

#### String concatenation and comparison

`a + b` concatenates, and `s += b` appends to a place. A chain such as `a + b + c + d`, with any interpolations inside it, is one operation that allocates the result once.

`==` and `!=` compare contents. `<`, `<=`, `>` and `>=` compare bytes in order, which for UTF-8 is code point order.

Equality checks the lengths, then the pointers, then compares eight bytes at a time with an overlapping final load.

#### String interpolation

`"{expr}"` interpolates a value of any type. `{{` and `}}` are literal braces. `print(expr)` writes the same text followed by a newline.

`"{expr,width:format}"` aligns and formats the value, as in `"hello {name,13} your age is {age:##}"`, `"{price,10:f2}"`, `"{id:x8}"` and `"{n,-6}|"`. Both parts are optional. `{x,}` and `{x:}` are errors.

The three parts are found in this order:

1. The expression ends at the first `,` or `:` outside brackets, which is where the width starts.
2. The width ends at the next `:` or at the closing brace.
3. The format is all the text up to the closing brace, so it may contain `,` and `.`.

**Width.** A width is a non-zero decimal integer with an optional `-` and optional surrounding spaces. A positive width right-aligns and a negative one left-aligns, padding with spaces to that many code points. A longer text is never cut. A width applies to every type, including `str` and the default text of a struct.

**Format.** A format has no spaces and is checked at compile time. It applies to integers and floats. It is either a standard format, one letter with an optional precision of 0 to 99, or a picture. The compiler lowers it to prelude calls with constant parameters, so no format text is parsed at run time.

#### Standard formats

`N` is the precision.

| Format | Types | Output |
|---|---|---|
| `dN` | integers | decimal with at least `N` digits, zero-padded after the sign: `{-5:d3}` is `-005` |
| `xN`, `XN` | integers | lower or upper case hex with at least `N` digits. A negative value is its two's complement at the width of its type: `{i32(-1):x}` is `ffffffff`, `{-1:x}` is `ffffffffffffffff` |
| `bN` | integers | binary, with the rules of `x` |
| `fN` | integers, floats | fixed point with `N` decimals, 2 by default: `{3.14159:f3}` is `3.142`, `{7:f1}` is `7.0` |
| `eN` | integers, floats | one digit, then `N` decimals (6 by default) and an exponent written like the default float text: `{12345.678:e2}` is `1.23e+4` |
| `nN` | integers, floats | like `f` with a `,` between every three integer digits; `N` is 0 for an integer and 2 for a float by default: `{1234567:n}` is `1,234,567` |

#### Picture formats

A picture is a format of only `0`, `#`, `,` and `.`, with at least one `0` or `#` and at most one `.`.

**Integer part.** Each `0` is a digit that is always written and each `#` is a placeholder. A `,` between two placeholders groups the integer digits in threes. With no `0` in the integer part, a zero integer part is omitted.

**Fraction part.** The value is rounded to the number of `0`s plus `#`s. Trailing zeros are then dropped down to the number of `0`s. The `.` is dropped when no decimals remain.

| Interpolation | Text |
| --- | --- |
| `{36:##}` | `36` |
| `{0:##}` | empty |
| `{0.5:#.##}` | `.5` |
| `{0.5:0.##}` | `0.5` |
| `{2.5:0.##}` | `2.5` |
| `{2:0.##}` | `2` |
| `{3.14159:0.00}` | `3.14` |
| `{1234567.891:#,##0.00}` | `1,234,567.89` |
| `{5:000}` | `005` |

#### Rounding in formats

Rounding is correct for the exact binary value of a float. It breaks ties to even, as glibc's `printf` does.

| Interpolation | Text |
| --- | --- |
| `{0.125:f2}` | `0.12` |
| `{0.375:f2}` | `0.38` |
| `{2.675:f2}` | `2.67`, because 2.675 lies just below the tie. |

- `{1e300:f2}` writes every digit.
- An `f32` formats its own exact value.
- Integers are exact up to the whole range of `u64` and `i64`.
- A negative value that rounds to zero keeps its sign, as in `-0.00`.
- `NaN`, `inf` and `-inf` ignore the format, and the width still applies.

#### Format errors

A format on a value of a type other than an integer or float is an error: `cannot format a value of type str with "f2"`. These are errors too:

- `d`, `x`, `X` or `b` on a float
- an unknown letter
- a precision above 99
- a picture with other characters, two `.` or no `0` or `#`

#### String escapes

String and character literals take the escapes `\n`, `\t`, `\r`, `\0`, `\\`, `\"` and `\'`. Four more name a value by its hex digits, which may be upper or lower case.

| Escape | Digits | Value |
| --- | --- | --- |
| `\xhh` | 2 | One byte. |
| `\uhhhh` | Exactly 4 | One code point. |
| `\Uhhhhhhhh` | Exactly 8 | One code point. |
| `\u{h...}` | 1 to 6 | One code point. |

**`\xhh`.** In a string it may make bytes that are not UTF-8, as `"\xff"` does. In a character literal it is the code point `0` to `0xff`.

**Code points.** `\uhhhh`, `\Uhhhhhhhh` and `\u{h...}` are one code point. In a string it is written as its UTF-8 bytes, so `"é"` is `"\u00e9"` and `"\u{e9}"`. `"😀"` is `"\U0001F600"` and `"\u{1F600}"`. A surrogate (`D800` to `DFFF`) or a value above `10FFFF` is an error.

**Braced form.** `\uhhhh` and `\Uhhhhhhhh` take exactly 4 and 8 digits. `\u{h...}` takes 1 to 6, so `'\u{2028}'` is the code point 8232 and `"\u{0}"` is one NUL byte. The `{` after `\u` belongs to the escape and never starts an interpolation, so `"{x}\u{2028}{y}"` interpolates twice. These are errors: an empty `\u{}`, a `\u{` with no closing `}` before the end of the literal or a non-hex character, and more than 6 digits.

**In the library.** `text.unquote` reads the fixed-width spellings, and `std/regex` takes them in a pattern. `text.quote` writes `\xhh` for control characters and bytes that are not UTF-8.

#### Default text

Every type has a default text, generated by the compiler with no annotation or constraint.

| Type | Text |
| --- | --- |
| Integer, float | The formats above. |
| `bool` | `true`/`false`. |
| `str` | Inserted unchanged when it is itself printed or interpolated. Inside another value it is quoted with `"`, with `"`, `\`, newline and tab escaped. |
| Struct | `Point{x: 1, y: 2}`. It shows its name without module or type arguments, as in `Stack{items: [1]}`, and every field, private ones included. |
| Enum variant | Its name, `Leaf`; with positional fields `Node(a, b)`; with labelled fields `Move{x: 1, y: 2}`. |
| Optional | `none` or its value. |
| Slice | `[1, 2, 3]`. |
| `u8[]` | `0x` and lowercase hex digits, `0x6869`. |
| Map | `{k: v, k2: v2}` in iteration order with keys rendered like any nested value. |
| Vector | `<1, 2, 3, 4>`. |
| Pointer | Its target. A pointer met again while it is being rendered is `<cycle>`, so shared pointers render in full each time and only a cycle is cut. |
| Function, channel | Its type, `fn(i64) -> i64` or `chan[str]`. |
| `ptr[T]` | Its type and address, `ptr[u8](0x7ffd1c2a)`, and a null one `ptr[u8](0x0)`. |

Floats are formatted as the shortest decimal that reads back to the same value. The text is positional for exponents from -6 to 20, and otherwise `1.5e-7` or `1e+21`. Infinities print as `inf` and `-inf`, NaN as `NaN`.

### Structs

A struct groups named fields. Structs are values: assignment and argument passing copy them.

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
    children: Element[] = []
}

let page = Div { padding: 8.0 } {
    Label(Text { value: "title" })
    Box(Div { align: Center } {
        button("ok")
        button("cancel")
    })
}
```

#### Struct declarations

A declaration's fields are separated by commas or newlines. Fields are laid out in declaration order, one word per scalar, with no padding.

A struct cannot contain itself by value. It can hold a `*` pointer to its own type.

**Defaults.** A default is any expression. It is checked once against the field's type, in the declaring module's scope, with no access to locals or other fields. It is evaluated at every literal that leaves the field out, after the written fields. A field with a default can be left out of a literal in another module even when it is private.

**Tags.** A field may carry a tag. A tag is a string literal without interpolation, after the field's type and before its default: `id: i32 "2,sint" = 0`. The checker keeps the text and does nothing else with it. `serde.schema` reports it in `Property.tag`, so a serialization format that needs more than the type holds, such as a field number, reads it there.

#### Struct literals

Literals name their fields, separated by commas or newlines. `x` alone means `x: x`. A field declared with `= expr` may be left out. Every other field must be given.

A struct literal cannot appear directly in an `if`, `for` or `match` condition. Wrap it in parentheses.

#### Trailing blocks

A brace block on the same line after a struct literal holds its children. The entries, one per line or separated by commas, become a slice literal for the struct's `children` field.

A brace block after a call becomes a slice literal passed as the last argument, so `fn button(label: str, children: Element[]) -> Element` takes children the same way.

A trailing block is not allowed where a struct literal is not.

#### Struct fields and equality

Fields are assigned through a `var` binding or a `mut` parameter, at any depth: `r.max.x = 1`. Other parameters are immutable.

`==` and `!=` compare structs field by field when every field can be compared: numbers, `bool`, `str`, pointers, channels, and structs and enums made of them. Each field uses its own `==`, so strings compare by content, floats by value and pointers by identity.

### References

A `mut` parameter refers to the caller's variable. Reads and writes go through to it. The call site writes `mut` before the argument.

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

A `mut` argument must be a `var` binding, a `mut` parameter, a field or element of either, or a place through a dereferenced pointer.

One call cannot pass overlapping places as `mut`. `swap(mut p.x, mut p.x)` is rejected. `swap(mut p.x, mut p.y)` is allowed. Two `mut` parameters therefore never alias.

References exist only as parameters. They cannot be stored, returned or bound with `let`, so they never outlive the variable they refer to.

#### How a reference is passed

A `mut` parameter occupies one word holding the address of the place.

A place narrower than a word is copied into a word-sized frame slot before the call. Examples are an element of a packed slice like `u8[]` and a narrow `extern struct` field. The place is copied back at its own width after the call. The callee's word-sized reads and writes therefore never touch the neighbouring bytes.

Code in the callee that reads the same element through another route, such as a pointer to the slice, sees the value from before the call until the callee returns.

### Pointers

`new(value)` copies a value onto the garbage-collected heap and returns a `*T`. Pointers are never null.

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

| Expression | Meaning |
| --- | --- |
| `p.field` | Reads and writes through the pointer. |
| `*p` | The whole value, readable, assignable and passable as `mut`. |

- The object behind a pointer is mutable through any pointer to it, including one held by `let`.
- `==` and `!=` on pointers compare identity.
- `mut` arguments through pointers are not checked for overlap, since two pointers may refer to the same object.

### Regions

`region { ... }` is an expression with the type and value of its block. While the block runs, every allocation of the green thread goes to a fresh region of memory: `new`, the growth of slices and maps, strings, interpolation, closure environments, derived code and the standard library. The region is freed when the block ends, so the garbage collector never sees that memory.

```low
struct Report {
    lines: str[]
    total: i64
}

fn summarise(rows: str[]) -> Report {
    region {
        var lines: str[] = []
        var total = 0
        for row in rows {
            let upper = new(row.len)
            lines.push("{row}: {upper}")
            total += *upper
        }
        Report { lines, total }
    }
}
```

`region` is a contextual keyword. It is an ordinary identifier except where an expression begins and `{` follows, outside the head of an `if`, `for` or `match`.

Regions nest. An allocation goes to the innermost region, and a nested region's value is copied to the region around it.

#### The value of a region

The value of the block is deep-copied to the allocator that was current outside the block. This happens before the region is freed, so the result owns its memory.

| In the result | Copied as |
| --- | --- |
| Numbers and `bool` | Values. |
| `str` | A copy. |
| Slices | Their elements, in order. |
| Maps | Their entries, in insertion order. |
| Structs, enums and optionals | Each field. |
| `*T` | A new object each time it occurs, so two pointers to one object in the block become two objects in the result. |

A pointer that leads back to itself panics with `region: cycle through *T`. A result without pointers, strings, slices or maps is not copied.

The result type cannot contain a function, closure, channel, `ptr`, extern struct, droppable struct or error result. The checker rejects it with the offending type named.

#### Leaving a region

`return`, `break`, `continue`, `fail`, `?`, `or return`, `or break` and `catch` mappings cannot leave the block. A loop or closure inside the block may use them for itself.

A panic ends the process without freeing regions.

#### Memory that outlives a region

A pointer, slice or string that points into the region and is stored outside it dangles once the block ends. Storing it outside means keeping it in a variable, a heap object or a channel.

| Build | Reading the dangling value |
| --- | --- |
| `-c debug` | Panics with `region: use of freed memory` until a later region on the same OS thread reuses that memory. After that it reads the later region's data. |
| Release | Gives the freed region's data or the later region's data, because a release build keeps the first chunks of a freed region committed for the next region on the thread. It panics only once the chunk has gone back to the operating system. |

#### Regions, threads and the collector

`spawn` inside a region starts a green thread whose allocations go to the garbage-collected heap. The call's closure environment is copied to the heap, but the values it holds that point into the region are freed with the region.

An allocation in a region does not count toward the collector's trigger, so allocation in regions alone does not start a collection.

### Enums

An enum is a set of variants, each of which may carry fields. A `match` takes a value apart by variant.

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

Variants are separated by commas or newlines. They are constructed as `Shape.Circle(2)`. Where the expected type is known, the bare name works, as in `let s: Shape = Empty`. The expected type is known in an annotated `let`, an argument, an assignment, a return and `fail`.

An enum cannot contain itself by value; it can hold a `*` pointer to its own type.

#### Match arms

`match` must cover every variant, or end with `_` or a binding arm. Arms after `_` or a binding arm, and duplicate arms, are errors. Arms are separated by newlines or commas.

An arm body is an expression, or `break`, `continue`, `return` or `fail`.

| Arm | Starts with | Matches |
| --- | --- | --- |
| Variant | A variant name | That variant. Patterns bind fields by position; `_` ignores a field, and a bare variant name ignores all of them. |
| Binding | A name starting with a lowercase letter that is not a variant, such as `other => describe(other)` | Anything. It binds the whole subject. |
| Test | A literal (`4`, `-1`, `'x'`, `"a"`, `true`), a parenthesised expression, `!` or `~`, or a name followed by `.` (`Mode.Fast`, `limits.max`) | A subject equal to the expression. |
| Operator | A binary operator | A subject for which the test holds. |

A capitalized name that is not a variant is an error, so a mistyped variant is never taken for a binding.

**Test arms.** A test arm is an expression compared with `==`. `4 => ...` means `== 4 => ...` and `(base + 1) => ...` means `== base + 1 => ...`. A `match` with a test arm must end with `_`. A bare name is a variant or a binding, so write `(limit) => ...` to compare against a variable.

**Operator arms.** An arm that starts with a binary operator tests the subject. `% 3 == 0 => ...` means `subject % 3 == 0`. The subject is on the left, and ordinary precedence applies after it.

- The subject is evaluated once and arms are tried in order.
- Such tests work on a subject of any type, including enums (`== Empty`) and the error of a `catch`.
- A `match` that uses one, or whose subject is not an enum, must end with `_`.

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

#### Variant indexes

`= n` after a variant gives it an index.

```low
enum Colour {
    Black = 0,
    Red = 1,
    Indexed(n: u8) = -1,
    Rgb(r: u8, g: u8, b: u8) = -1,
}

let index = Colour.Red.index
```

- The index is an integer literal that fits in an `i64`, with an optional leading `-`.
- When one variant has an index, every variant must have one.
- Variants may share an index.

`e.index` is the `i64` index of the variant `e` holds. It is an error on an enum whose variants have no indexes, and that includes an optional.

The index is a value the variant carries, not its tag: layout, `==` and `match` are the same with or without indexes.

A field of a variant takes a tag as a struct field does: `Sensor(id: i32 "sint") = 1`. `serde.schema` reports it in `Property.tag`.

#### Enum equality

`==` and `!=` compare enums whose variant fields can all be compared. Equal values have the same variant and equal fields.

Optionals compare the same way, so `x == none` and `x == 5` work on an `i64?`. `x == none` and `x != none` also work on every optional, whatever its payload. They test only whether a value is present. Comparing two optionals still needs a payload that can be compared.

#### Enum layout and code generation

An enum whose variants all have no payload is a single word. Otherwise an enum is a tag word followed by the largest variant's fields.

`e.index` compiles to the tag plus a constant when every variant's index is its tag plus that constant. Otherwise it compiles to a switch on the tag, as for a `match`. The default of that switch is the index shared by the most variants.

Code generation for a `match` depends on its arms. The rules below apply when the arms before the last are all distinct variants of one enum, or all distinct integer or character literals of one type.

| Arms | Compiles to |
| --- | --- |
| At least 12, with values that span at most 4096 and at most three times the arm count | A `switch` block. The backend emits a bounds check and an indirect jump through a table of 32-bit offsets. |
| 48 or more, without that density | A binary search over the values. |
| Fewer | The chain of comparisons. |

Duplicate values, test arms and arms with other expressions always keep the chain. A `switch` on a constant folds to its target.

### Optionals

`T?` holds either `some(value)` or `none`.

<!-- prelude
let xs = [1, 7, 4]
-->
```low
fn find(xs: i64[], target: i64) -> i64? {
    for i, x in xs {
        if x == target {
            return i
        }
    }
    none
}

fn first(xs: i64[]) -> i64? {
    xs[0] when xs.len > 0
}

let index = find(xs, 7) ?? -1
let head = xs[0] when xs.len > 0 else 0
match find(xs, 4) {
    some(i) => print("at {i}")
    none => print("missing")
}

struct Node {
    value: i64
    next: *Node?
}
```

A `T` is accepted wherever a `T?` is expected: an annotated `let`, an assignment, an argument, a field, a return and either branch of an `if` or `match`. `none` and `some(v)` take their type from that expectation.

| Form | Gives |
| --- | --- |
| `x ?? fallback` | The value inside `x`, or `fallback` when `x` is `none`. |
| `value when cond` | `value` when the `bool` `cond` is true and `none` otherwise. |
| `value when cond else fallback` | `value` when `cond` is true and `fallback` otherwise. |
| `match` with `some(v)` and `none` arms | The unwrapped value in the `some(v)` arm. It must cover both. |

An optional is an enum with `none` as tag 0 and `some` as tag 1, so it takes one word more than its value.

#### The ?? operator

`x ?? fallback` gives the value inside `x`, or evaluates `fallback` when `x` is `none`.

- The fallback may itself be a `T?`, making the result a `T?`.
- The fallback may leave instead, as in `let line = read_line() ?? break`.
- An expected type `T` reaches `x` as `T?`, so in `let n: u8 = (200 when ready) ?? 0` the literal is a `u8`.
- `??` binds tighter than comparisons and looser than arithmetic. It groups to the right: `a ?? b ?? 0` tries `a`, then `b`.

#### The when expression

`value when cond` gives `value` when the `bool` `cond` is true and `none` otherwise. `cond` is evaluated first, and `value` only when `cond` is true.

The result is a `T?` for a `value` of type `T` or `T?`, so `find(xs, 7) when xs.len > 0` is an `i64?`. A `value` of no type, such as a call that returns nothing, is an error.

`when` binds looser than every binary operator, `|>` included, and does not chain: `a + b when c && d` is `(a + b) when (c && d)`, and a second `when` needs parentheses.

`return v when c` returns `v when c`, and an expression-bodied closure `fn(n) n * 2 when n > 1` has the whole `when` as its body.

`when` is a contextual keyword: it is an ordinary identifier everywhere except directly after an expression on the same line.

#### when with else

`value when cond else fallback` gives `value` when `cond` is true and `fallback` otherwise, as `(value when cond) ?? fallback` does.

- `fallback` is checked against the type of `value`, so `spans(tab) when ready else []` is a slice of the type `spans` returns, and an empty `[]` or `[:]` as `value` takes the type of `fallback`.
- `fallback` is evaluated only when `cond` is false or `value` is a `none`.
- `fallback` may be a `T?`, which makes the result a `T?`. It may leave, as in `let n = next() when more else break`.
- `else` binds as loosely as `when`, and a second `when` in `fallback` needs parentheses.

#### Optional types

`?` after a type binds to everything before it. Parentheses group a type.

| Type | Is |
| --- | --- |
| `*Node?` | An optional pointer. |
| `i64[]?` | An optional slice. |
| `*Node[]?` | An optional slice of pointers. |
| `(fn() -> i64)[]` | A slice of functions. |
| `*(i64[])` | A pointer to a slice. |
| `(str?)?` | An optional optional. |

`str??` means the same as `(str?)?`. At the end of a line, where `??` would continue the expression, it is an error and the parenthesised form is needed.

### Embedded data

`embed("path")` is the contents of a file as a `str`, read at compile time. `embed_dir("path")` gives the regular files under a directory as a `[str: str]`.

<!-- prelude shaders/quad.spv
-->
<!-- prelude fonts/fallback.ttf
-->
```low
let shader = embed("shaders/quad.spv")
let font = embed("fonts/fallback.ttf")
print(shader.len)
```

The path must be a string literal. It is relative to the directory of the file that calls `embed`, so a module carries its own assets.

The bytes live in the binary's read-only data, aligned to 16 bytes; nothing is copied at run time. Indexing, slicing, `.len` and `bytes(...)` work as on any string.

| Case | Result |
| --- | --- |
| A missing file | A compile error. |
| A path outside the tree of the fetched dependency whose module calls `embed` | A compile error (see `### Projects`). |

`low dev` watches embedded files. A change to one reloads the functions that embed it.

`embed_dir("path")` embeds every regular file under a directory, however deep. It gives a `[str: str]` from each file's path inside the directory to its contents, with `/` between segments. The entries are in byte order of those paths. Each call builds a new map. The contents are the same read-only bytes `embed` gives.

### C interop

An `extern` block declares the functions of a C library. Pointers, C structs and callbacks have their own types.

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

#### Declaring C functions

`extern "name" { ... }` at the top level of a file names a C library and declares its functions without bodies. `"c"` is the C library itself.

- A trailing `...` makes a function variadic.
- Declarations are private to their module unless marked `pub`.
- `fn errno_location() -> ptr[i32] = "__errno_location"` gives a function a Low name different from its C symbol, for symbols that start with `__` or are Low keywords.

C signatures use C types: integers, floats, `bool`, `ptr`, `ptr[T]`, and `extern struct`s by value.

A `mut` parameter passes the address of a Low place for the duration of the call, as C# passes `ref`. C may read and write through it.

#### Linking C libraries

A library is linked when code reachable from `main` calls or takes the address of one of its declared functions. Importing a module that declares C functions therefore costs nothing until one is used.

| Program | Executable |
| --- | --- |
| Uses no library | A static executable with no libc. |
| Uses any library | A dynamic executable linked against libc and each used library. |

#### C pointers

`ptr[T]` is a nullable pointer to C memory. `ptr` is `void*` and converts to and from any `ptr[T]`. `null` takes its type from context. `==` and `!=` compare addresses.

| Builtin | Does |
| --- | --- |
| `ptr(n)` | Makes an address from an `i64` or `u64`. |
| `i64(p)`, `u64(p)` | Give a `ptr`'s address as a number. |
| `view(p, n)` | A `T[]` over `n` elements at `p`, bounds-checked like any slice. |
| `load(p)`, `store(p, v)` | Read and write one `T`. |
| `cstr(s)` | A NUL-terminated copy of a string. |
| `str(p)` | Copies a NUL-terminated `ptr[u8]` back. |

`ptr(n)`, `i64(p)` and `u64(p)` are for C values that carry either an address or a number, such as a Win32 `LPARAM`.

#### C struct layout

`extern struct` has C layout. Fields are in order at their natural alignment, and the size is rounded to the largest alignment. Its fields must have C types. It is used like any struct, and its fields are read and written at their own width.

#### Memory shared with C

Memory may be passed to C for the duration of a call. Memory C keeps after the call must be pinned. `pin(x)` returns the address of a pointer, slice, map or string's data and keeps it alive until `unpin(p)`.

Memory C allocates is never touched by the collector.

#### C callbacks

`extern fn(ptr[i64], ptr[i64]) -> i32` is a C function pointer. It is one word, like C#'s `delegate* unmanaged`. Its parameters and result must have C types. It is a C type, so it can be a parameter, a result or an `extern struct` field.

- A top-level Low function becomes a C callback where an `extern fn` is expected, if it takes and returns only numbers, `bool`, `ptr` or `extern fn`, with no `mut` parameters.
- An `extern` function used as a value is its C address.
- Calling an `extern fn` value makes a C call.

A closure reaches C through a `ptr` user-data argument: pin a pointer to the state it needs and pass a top-level callback that loads it.

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

`defer` runs a statement when its enclosing block exits.

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

The block exits, and the statement runs, in each of these cases:

- at the end of the block
- on `return`, `fail` or a propagated `?`
- when `break` or `continue` leaves it

Deferred statements of a block run last first. The statement sees variables as they are at exit.

The statement cannot `return`, `fail`, use `?`, `break`, `continue` or contain another `defer`.

### Using

A struct with a `drop` owns a resource, and `using` binds such a value so that it is released when its block exits.

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

#### Declaring a drop

`drop <stmt>` inside a struct body declares its cleanup, with the struct's fields in scope as borrowed values. Like `defer`, it takes one statement or a `{ }` block. A struct has at most one `drop`, and `using` cannot appear in it.

A struct with a `drop` is droppable, and so is a struct with a droppable field, even without its own `drop`. Generic instances are droppable per instance: `Pair[File, i64]` is, `Pair[i64, i64]` is not.

Releasing a value runs its own `drop` first, then releases its droppable fields in declaration order.

#### Binding with using

| Form | Meaning |
| --- | --- |
| `using x = expr` | Binds `x` and releases it when the enclosing block exits by any path: the end of the block, `return`, `fail`, `?`, `break` or `continue`. |
| `using var x = expr` | A binding that can be reassigned. Assignment releases the old value, and exit releases the value held then. |
| `using y = x` | Moves `x` into `y`. |

Releases and `defer`s run together, last first.

- `using` cannot appear inside `defer`.
- Binding a non-droppable value with `using` is an error.
- A droppable value cannot be bound with `let` or `var`: "File has drop; bind it with using".
- Results of droppable types are unwrapped with `?`, `catch` or `or` straight into a `using` binding.

Each `using` binding has a hidden flag that a move clears. The release at exit runs only while the flag is set, so a value moved on one branch is released exactly once.

#### Moving a droppable value

Droppable values are move-only. A value moves, or is borrowed, in these places:

| Place | What happens |
| --- | --- |
| `return x`, or a function body ending in `x` | Moves a `using` binding out. The caller owns the value and must bind it with `using`. |
| A struct literal field, or an assignment to a field of a droppable value | Moves the value into it, and the outer value releases it. |
| `using y = x` | Moves `x` into `y`. |
| An argument | Borrows the value: the callee cannot release it, store it or return it. |

A field is assigned through a `using var` binding or a `mut` parameter, and the field's old value is released first.

A freshly produced value, such as a call result or a struct literal, must be bound with `using` before it is passed.

Using a binding after it was moved is an error, as is using one that may have been moved on one branch or by an earlier loop iteration.

#### Copies that are errors

Any other copy is an error:

- binding it with `let`
- storing it in a slice, map, channel, optional, pointer or enum variant
- moving a field out of its struct
- discarding a call result
- capturing it in a closure

A struct with a droppable field is itself droppable, so it cannot be built with `new` or held by a pointer, and `spawn` cannot take one as an argument: "spawn cannot take File; it has drop".

Code that shares a resource across closures, pointers or green threads passes a copyable view of it. It binds the owner where the resource has one home (see Resource ownership).

Generic code is checked under these rules for each instance whose type arguments are droppable, so `fn same[T](x: T) -> T { x }` cannot be instantiated with `File`.

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

A module is a directory. Every `.low` file in it shares one namespace, so files of a module need no imports between them.

#### Imports and visibility

Imports come before declarations and top-level statements and apply to their file.

| Written | Means |
| --- | --- |
| `import ui/text` | Loads `ui/text/` under the project's modules directory, which is the root unless the `build.low` declares `modules`. |
| `text.shout(...)` | A module is used through its last path segment. |
| `import t = ui/text` | The module is used through the name given. |
| `import github.com/acme/ui` | A path segment may hold `.` and `-` between identifiers, written without spaces. |
| `import colors = color-names` | When the last segment is not an identifier, the import needs a name. |
| `geometry.Shape` | A type of another module. |
| `geometry.Point { ... }` | Builds a struct of another module. |
| `geometry.Shape.Dot` | Names a variant. Where the type is expected, `Dot` is enough. |

A path starting with `std/` names a module of the standard library, which is compiled into `low`, so a project directory named `std` cannot be imported.

Declarations and struct fields are private to their module unless marked `pub`. A `pub` enum's variants are public. A struct with private fields without defaults cannot be built with a literal outside its module.

Two things are errors:

- An import cycle, except through the imports of test files.
- Importing two modules under one name.

#### The project root and the entry module

The project root is the nearest directory, starting at the entry, that holds a `build.low` or a `low.mod` file. Without one it is the entry's directory. A `build.low` names the project's dependencies, programs and modules directory, as [Projects](#projects) describes. An empty `low.mod` marks a root with none.

| Command | Entry module |
| --- | --- |
| `low build dir` | The directory. |
| `low build file.low` | That one file, without the other files beside it. |

#### Files for one target

A file's name can limit it to one operating system or one architecture.

| File name ends in | Belongs to the module only when building for |
| --- | --- |
| `_linux.low`, `_windows.low` or `_browser.low` | That operating system. |
| `_amd64.low` or `_wasm32.low` | That architecture. |

The standard library uses this to give `std/sys` and the operating-system parts of `std/os`, `std/process`, `std/net` and `std/crypto/x509` one implementation per target. A test requires every target to declare the same public items with the same doc comments. The test compares the browser on the modules that do not import `std/sys`, directly or through another module.

#### Example programs

A directory named `examples` inside a module holds that module's examples. Each `.low` file in it is a whole program that imports the module, with its expected output in a `.out` file of the same name.

An `examples` directory is never a module. Importing a path with an `examples` segment is an error.

#### Test files

A file whose name, after removing `.low` or its operating system or architecture suffix, ends in `_test` is a test file: `split_test.low`, `split_test_linux.low` and `split_test_wasm32.low`.

A test file belongs to its module only in a test build of that module. Every other build leaves it out, so `low build`, `low run`, `low dev`, `low asm`, `low ir`, `low lint`, `low doc` and `low names`, and every module that a tested module imports, never read it. The language server reads a test file only for its own analysis of that file.

A test is a top-level `fn test_<name>(t: *testing.T)` in a test file. Here `testing` is the name the file imports `std/testing` under. Inside `std/testing` the parameter type is `*T`.

- A `test_` function in a test file is a compile error at the function if it has any other shape, with other parameters, type parameters, a result or error type, or nothing after the prefix.
- A `test_` function in any other file is an ordinary function.
- Tests run in the order of their file names, then of their declarations.
- A test's name is its function name without `test_`.

The imports of test files and of the generated harness are left out of the import cycle check. A test file may import `std/testing` although `std/testing` imports the tested module, as `std/text` is tested. A test file may import a module that imports the tested module.

#### Running tests

`low test [path]... [options]` builds the tests of each named module into a program and runs every test in its own process. Paths and options come in any order. `--` ends the options.

The tested module keeps its import path, its directory below the project's modules directory (`src/lib` is `lib` when the `build.low` declares `modules("src")`), so a module that the test code imports is loaded once and a test reaches the module's private items. The project root loads as a module named `__root`, so its `main` is an ordinary function that tests can call.

Two kinds of path are an error: a directory outside the modules directory that is not the project root, and the modules directory itself. The message is `<path>: outside the modules directory <dir>`.

#### Test paths

A path names one module, a tree of modules or one test.

| Path | Tests |
| --- | --- |
| `dir` | The module in that directory. |
| `std/<module>` | A standard library module. |
| `dir/...` | Every module at or below `dir` that has a test file, in sorted path order. |
| `std/...`, `std/<dir>/...` | The standard library modules with a test file. |
| `...` | A bare `...` is `./...`. |
| `path/name_test.low:LINE` | The module of that file with the one test whose function is the last in the file to start at or before `LINE`. |
| No path | `./...`. |

- The walk of `dir/...` leaves out directories named `testdata` or `examples` and directories whose name starts with `.` or `_`, below `dir`.
- For `path/name_test.low:LINE`, the test is found by comparing cleaned absolute paths with the `file` and `line` of `-list`. A line before the first test is a usage error.
- A module named by a path that has no test file prints `?    <module> [no test files]` and does not fail.
- A path may be repeated and each occurrence is built and run on its own.

If a `dir`, `dir/...` or `path/name_test.low:LINE` names nothing below the current directory, it names the same path below the modules directory of the project that holds the current directory. It keeps the label it was written with. From the root of a project whose `build.low` declares `modules("src")`, `low test lib` tests `src/lib` and reports it as `lib`. A path that exists below the current directory and a `std/` path are never looked up there.

#### Test options

| Option | Does |
| --- | --- |
| `--run text` | Runs the top-level tests for which `testing.selected(text, name)` holds, which compares each `/` separated level of the name by substring, and passes the filter to the child so that it filters subtests. |
| `--skip text` | Leaves out the top-level tests for which `testing.selected(text, name)` holds. Subtests are not skipped. |
| `--list` | Builds, then prints `<module> <test> <file>:<line>` for each selected test. `file` is relative to the current directory when it is inside it. It runs nothing. |
| `-v`, `--verbose` | Shows the output of every test and passes `-v` to the child. The `--- PASS: <name>` line of a replayed test ends with ` (cached)`. |
| `--notime` | Leaves every duration out, and passes `-notime` to the child. |
| `--count n` | Runs each selected test `n` times (default `1`), each in its own process. |
| `--shuffle seed` | Starts the tests in an order shuffled by `rand.seeded(seed)` and prints `shuffle seed <seed>` before anything else. The report order does not change. |
| `--failfast` | Starts no more tests after one failed, crashed or timed out. A test never started counts as `not run`. |
| `--json` | Writes one JSON object per line instead of the text report. |
| `--timeout ms` | Kills a test's process after `ms` milliseconds (default `30000`, `0` for no limit) and reports `timed out after <ms>ms`. The child always gets `-timeout 0`, so only the parent reports a timeout. |
| `-j n`, `--jobs n` | Runs at most `n` processes at once (default `os.cpu_count()`). |
| `--update` | Passes `-update` to the child, which rewrites golden files. |
| `--nocache` | Builds every program and runs every test instead of reusing cached programs and replaying cached passes, and stores nothing. |
| `--max-errors n` | Stops checking a module's build after `n` errors, 50 by default. |

#### How a test run builds and runs

A module's test program is built in process with the release profile for the host, into one temporary directory, one module after another. The temporary directory is removed when the command ends.

- A module that fails to build prints its error to standard error and counts as a build failure. The other modules are still built and run.
- Warnings print as `low build` prints them.
- There is no `--target`, because a program for another operating system cannot run on the host.

`low test` runs each program with `-list` to learn its tests. Every selected test then runs through a pool of `--jobs` green threads, shared by all modules. A test runs as `<program> -one <name> -timeout 0`, plus `-run`, `-v`, `-notime` and `-update` when given.

| Part of the child | Set to |
| --- | --- |
| Standard input | `/dev/null`. |
| Standard output and standard error | Piped and captured. |
| Working directory | The module's directory (the current directory for a standard library module), so a test reads files beside its module as in Go. |

The child leads its own process group. The parent alone measures the timeout. It kills the group when the timeout expires, and kills the group again once the child has exited. A grandchild holding the pipes therefore cannot keep the parent waiting. A grandchild that leaves the group is not reached.

SIGINT and SIGTERM kill every running test's group, remove the temporary directory and end `low test` with status 128 plus the signal number, so an interrupt leaves no test process behind. A test process also ends when `low test` is killed, because a grouped child ends with its parent. A host without signal support has no such handling.

#### Test outcomes and exit status

The child's exit status gives the outcome of a test.

| Child ends with | Outcome |
| --- | --- |
| `0` | Passed. |
| `1` | Failed. |
| `3` | Skipped (`std/testing`'s `skipped_status`). |
| Any other status or a signal | A crash. |
| A kill by the timeout | A timeout. |

A test that `--failfast` stopped before it started is not run.

The exit status of `low test` follows from the outcomes.

| Status | When |
| --- | --- |
| `2` | A usage error or any build failure. A usage error prints a message to standard error. |
| `1` | Otherwise, when any test failed, crashed, timed out or did not run. |
| `0` | In every other case. |

#### Test slots shared across runs

Every `low test` on the machine shares two pools of slots.

| Pool | Slots | Held by |
| --- | --- | --- |
| `test` | One per CPU. | Each run of a test program, `-list` included, from just before the child starts until it has exited, so the wait does not count against `--timeout`. |
| `build` | One per four CPUs and at least one. | A module's build. |

- A replayed test and a reused program take no slot.
- `--jobs` still bounds one run.
- A test that `--failfast` stopped while it waited for its slot is not run.
- One green thread of a run polls for a slot at a time. It polls every 10 ms at first and every 50 ms after a few tries.

Slot `n` of a pool is an exclusive `flock` (`LockFileEx` on Windows) on `<cache>/slots/<pool>/<n>`. The system releases the lock when the holder ends, so a killed run frees its slots. Runs under different cache roots do not share slots. Without a cache root there are no slots.

A test that runs `low test` itself waits for a slot while it holds one, so such tests never finish when more of them run at once than there are slots.

#### The test result cache

A passing test is replayed from the cache when its inputs are unchanged. A replayed test reports a pass with the stored output and elapsed time, counts as cached and starts no process.

The cache is off in these cases:

- under `--nocache`, `--update` and `--count` above `1`
- when `compiler.Disk.cache` (`LOW_CACHE`, or the platform default that holds fetched dependencies) is empty

The cache's entries live in `<cache>/test/<key[0..2]>/<key>`. The directory is created on the first store. A cache that cannot be created, read or written leaves the tests running as without it.

**The key.** The key is the SHA-256 of these parts:

- a format line
- the SHA-256 of the built test program
- the test name
- the directory the child runs in (the current directory for a standard library module)
- the flags passed to the child
- `--timeout`
- every environment variable whose name starts with `LOW_`, except `LOW_CACHE`, sorted by name

The runtime reads some of those variables without `std/os`, so they are part of the key. A program built from the same sources is byte-identical whatever the temporary directory, `SOURCE_DATE_EPOCH` or module order, so the program's hash identifies it. The `-list` output has its own entry, keyed by the program's hash. A module whose program is unchanged is therefore not run for `-list` either.

**Recording inputs.** A test that is not replayed runs with `-inputs <file>` in the temporary directory, so `std/testing` makes `sys.record_inputs` log the files, directories, links, programs and environment variables the test reads. A name is resolved against the child's directory. A name inside `low test`'s own temporary directory is left out. Each (operation, path) pair is kept once in first-seen order.

**Storing a pass.** After a pass, `low test` stores the output, the elapsed time and the fingerprints of the inputs. It stores them only when all of these hold:

1. The log is complete and well formed.
2. The log has no `exec`, `chdir` or `redirect` line.
3. Every path named by an `open` or `list` line that exists as a regular file or a directory has a modification time at least two whole seconds before the clock reading taken just before the child started, because a file changed during the run would be fingerprinted in a state the test never saw.

A failure, a crash, a timeout and a skip are never stored. An entry is written to a temporary file in its directory and moved over its name, so several `low test` processes can share the cache. `lowtest` passes `--nocache`, so the `testdata/test` sidecars compare the output of a run that executes every test.

**What is not an input.** Reads made inside a C library or a Windows API outside `std/sys` are not logged, except the database file `std/sql/sqlite` opens: the Windows certificate store `std/crypto/x509` reads, the DNS settings of `std/net` on Windows, the user locale of `std/ui/dwrite` and what the Wayland and xkb libraries read. A network peer, a pipe, the clock and random numbers are not inputs either, so a test that depends on them replays while its logged inputs are unchanged.

**Fingerprints.** A fingerprint is computed from the file system and the environment when it is stored and again before each replay. A test is replayed only when every stored fingerprint equals the current one.

| Operation | Fingerprint |
| --- | --- |
| `open` | `file <SHA-256 of the content>` for a regular file, `dir` and the sorted entry names for a directory, `other` for anything else, `absent` or `error <kind>`. |
| `stat` | `file <size>`, `dir`, `other` or `absent`. |
| `list` | `entries` and the sorted names, or `absent`. |
| `readlink` | `link <target>` or `absent`. |
| `getenv` | `unset` or `set <value>`. |
| `environ` | The SHA-256 of the sorted `name=value` pairs. |

#### The test program cache

A module's test program is reused when nothing it was built from has changed. The program cache is off under `--nocache` and when `compiler.Disk.cache` is empty.

A program's entry is `<cache>/test/programs/<key[0..2]>/<key>`. The key is the SHA-256 of a format line, the target, the module's directory (its name for a standard library module) and the current directory.

| In the entry | Holds |
| --- | --- |
| The program | The last program built for that key, named by its SHA-256. |
| `manifest` | The SHA-256 of the `low` executable that built it, the program's hash, the warnings the build printed and a fingerprint of each path the build passed to `compiler.Disk`. |
| `place` | The module's directory. |

The manifest's fingerprints are `read` as `open`, `stat`, `list`, and `real` as `real <resolved path>` or `absent`. Each fingerprint is taken from what the call returned, so a file edited during the build is fingerprinted as the build saw it. The shared libraries the Linux linker reads are not fingerprinted.

The running `low` is hashed once and remembered in `<cache>/test/programs/compiler` with its path, size and modification time, written only once the executable is two seconds old, so a later run with the same executable reads the hash instead of computing it.

**Reuse.** A program is reused when the running `low` hashes the same and every fingerprint equals the current one. It is hard-linked into the temporary directory, or copied where a link fails. Its warnings print again, and its `build` event has `cached` set.

**Storing.** A build that takes the `std/buildinfo` stamp, or that sees one path give two answers, is not stored. A store places the program under a temporary name and renames it. It writes the manifest the same way and removes the entry's other programs, so the cache keeps one program per module and current directory. The first store of a run removes every entry whose directory is missing, so a deleted checkout or worktree leaves no programs behind.

#### The text test report

The text report is deterministic: modules in path order and tests in source order, each module printed once all its tests have finished. `--notime` leaves the times out.

**Each test.** With `-v` or `--verbose` every test shows its output.

| Test | Shows |
| --- | --- |
| Failed | The child's standard output, which holds its `--- FAIL: <name>` header and notes, then its standard error indented by four spaces, then `    rerun: low test <module> --run <name>`. |
| Crashed or timed out | Its own header, `--- FAIL: <name> (crashed: exit status <n>)`, `(crashed: signal <name>)` or `(timed out after <ms>ms)`, then the child's standard output and standard error indented by four spaces, then the rerun line. |

**Each module.** Each module then has one line.

| Line | For |
| --- | --- |
| `ok   <module> <counts> (<time>)` | A module that none of the lines below describes. |
| `FAIL <module> <counts> (<time>)` | A module with a failed, crashed, timed out or not run test. |
| `FAIL <module> [build failed]` | A module that did not build. |
| `?    <module> [no test files]` | A module without test files. |

The time is the span from the module's first test start to its last test end. `<module>` is the path as written, or the path relative to the current directory for a `...` expansion.

**The last line.** The last line is `PASS` or `FAIL`, two spaces, the counts and the total time, such as `FAIL  2 failed, 1 crashed, 1 timed out, 1 build failed, 40 passed, 3 skipped, 5 not run (1.20s)`. Counts that are zero are left out except `passed`. `passed` reads `<n> passed (<m> cached)` when `m` of the passed tests were replayed from the cache, in a module line as in the last line.

#### The JSON test report

`--json` writes these events, in the order of the text report, each on one line, with `elapsed_ns` left out under `--notime`:

| `event` | Members |
| --- | --- |
| `shuffle` | `seed`, written first when `--shuffle` is given. |
| `diagnostic` | One per error of a module whose build failed, before its `build` event, as in [JSON diagnostics](#json-diagnostics) with `module` added. |
| `build` | `module`, `ok`, `cached` (`true` when the program was reused), and `error` when the build or the listing failed. Not written for a module with no test files. |
| `test` | `module`, `test`, `file`, `line`, `result` (`pass`, `fail`, `skip`, `crash`, `timeout` or `not_run`), `cached` (`true` for a test replayed from the cache), `detail` for a crash or timeout, `output` (the child's standard output then standard error) and `elapsed_ns`. |
| `module` | `module`, `result` (`ok`, `fail`, `build_failed` or `no_test_files`), the counts `failed`, `crashed`, `timed_out`, `build_failed`, `passed`, `skipped`, `not_run` and `cached`, and `elapsed_ns`. |
| `summary` | `result` (`pass` or `fail`), the same counts and `elapsed_ns`. |
| `list` | `module`, `test`, `file` and `line`, written instead of the others by `--list`. |

#### JSON diagnostics

`--json` on `low build`, `low run`, `low asm`, `low ir` and `low lint` writes each error and warning to standard error as one JSON object on its own line, sorted by file, line and column, in place of the text lines. A build that fails exits with status 1. `low test --json` writes the same objects as `diagnostic` events in its report.

| Member | Value |
| --- | --- |
| `event` | `diagnostic`. |
| `code` | The code, a row of [Diagnostic codes](#diagnostic-codes). |
| `severity` | `error` or `warning`. |
| `file`, `line`, `column` | Where the diagnostic starts. Lines and columns count from 1, and a column counts bytes. |
| `end_line`, `end_column` | The position just past the token at the start: a name, a string literal or one character. |
| `message` | The text the text line shows between the position and the code. |
| `related` | Other places the diagnostic names, each with `label`, `file`, `line` and `column`. |
| `fixes` | Each fix's `title`, `safe` (`true` when `low fix` applies it) and `edits`, each edit with `file`, `line`, `column`, `end_line`, `end_column` and the replacement `text`. |

#### Test files in the language server

The language server analyses an open test file with the test build of its module over the editor's unsaved text, so diagnostics, hover, go-to-definition, references, rename, completion, signature help, inlay hints and symbols answer in it as in any file, and reach the module's private items and the modules it imports. The two generated files have no path, so no answer names them.

The build's diagnostics are published on the open test files of that directory.

| Diagnostic | Appears |
| --- | --- |
| An error in a test file | At its position. |
| An error in any other file | On the first line of an open test file, prefixed with that file's name, line and column. |
| A warning | Only for test files. |

The ordinary program's diagnostics for ordinary files do not change.

- A change to an ordinary file rebuilds every open test directory of its project.
- A change to a test file rebuilds its own directory.
- A test file under a standard library checkout is analysed as an ordinary directory of the project, not as the embedded module.

#### Lint warnings

`low lint file.low` type-checks the program, or a library module whose entry directory has no `main`. It prints warnings for the program's own modules, not the standard library. It exits with status 1 if there were any.

- `--target` picks the operating system whose files are checked and defaults to the host.
- `low build`, `low run`, `low dev`, `low asm` and `low ir` report `missing function main` for a library module whose entry directory has no `main`.
- `low build`, `low run`, `low asm` and `low ir` print the same warnings, except those only `low lint` applies, to stderr and continue. The language server reports those as warnings.

| Lint | Flags |
| --- | --- |
| `prefer-match` | An `if` chain or a run of guard clauses that compares one subject, which a `match` would say. |
| `use-std` | A function whose body repeats a public function of the standard library. |
| `prefer-loop-label` | A boolean flag set at a `break` out of an inner loop and tested right after that loop. |
| `one-letter-name` | A name of one letter where it is bound. Only `low lint` applies it. |
| `receiver-call` | A call to a function that changes its first argument, written with that argument in the parentheses. Only `low lint` applies it. |
| `method-call` | `push`, `pop`, `truncate` and `delete` called as `push(mut x, v)` rather than as `x.push(v)`. |
| `large-struct` | A struct of at least 64 words. Only `low lint` applies it. |
| `prefer-const` | A function that a constant could stand in for. |

#### Lint: prefer-match

`prefer-match` flags two shapes. In both, every condition compares one side-effect-free subject with `==`, `!=`, `<`, `<=`, `>` or `>=`. The subject is a variable, field, index or dereference. It is compared against an integer, character or string literal, a constant, an enum variant or another side-effect-free operand, with the subject on either side.

The shapes are:

- an `if` / `else if` / `else` chain with at least two conditions;
- at least two consecutive guard clauses, `if` statements without `else` whose block is only `return value`, that end a function body before its tail value or final `return`. Earlier guards on another subject are left out.

When both sides of a comparison are operands, the subject is the one every condition shares, taken from the left of the first condition when both are shared. A comparison of an operand with itself, such as the `x != x` test for `nan`, is not counted.

It suggests `match subject { 4 => ..., < 0x80 => ..., (limit) => ..., > high => ..., _ => ... }`: an `==` test against a constant as the constant alone, an `==` test against another operand in parentheses, and any other as an operator arm.

#### Lint: use-std

`use-std` flags a function whose body repeats a public function of the standard library. It suggests calling that function, through the file's import of that module when there is one.

Both bodies are reduced to a list of tests and results, with the parameters numbered by position:

- Guard clauses, an `if` / `else if` / `else` chain and a `match` with test and operator arms give the same list.
- `>` and `>=` are turned into `<` and `<=` with the operands swapped.
- The operands of `==` and `!=` are put in order.
- The reduction accepts only literals, parameters, unary and binary operators, and needs at least one test.

The parameter and result types must match those of the standard function. A type parameter matches any type that is the same at each of its uses.

`prefer-match` does not report on a function that `use-std` reports. A module is not compared with its own functions.

The standard functions come from `compiler/lint/std_index.low`. `compiler/tablegen stdindex std compiler/lint` writes it from every public function in `std/` whose parameter and result types are type parameters, numbers, `bool` or `str`. `std_index_matches_generator` in `compiler/tests/tablegen` fails when the committed index differs from the generator.

#### Lint: prefer-loop-label

`prefer-loop-label` flags a `break` out of an inner loop that sets a boolean flag to a literal. It does so when the statement right after that loop is `if flag { break }` or `if flag { continue }` (or `if !flag` for a flag set to `false`), with no `else`.

It suggests labelling the loop that `if` acts on and jumping to it with `break name` or `continue name`, reusing the label the `if` already names. Breaks inside a nested loop, closure, `spawn` or `defer` are not counted.

#### Lint: one-letter-name

`one-letter-name` flags a name of one letter where it is bound. Only `low lint` applies it.

| Bound by | Reported at |
| --- | --- |
| A parameter of a function or closure | The name. |
| A `let` or `var` | Its start. |
| The key or value of a `for … in` | Its start. A `for` whose key and value are both one letter gets one warning that names both. |
| A name a `match` or `catch` arm binds | The name. |
| A name a destructuring pattern gives a field (`{ x: w }`) | The name. |
| The value a `select` case receives | Its start. |

`_` is not flagged, and neither is a type parameter, a struct field or the shorthand `{ x }` that binds a field under its own name.

#### Lint: receiver-call

`receiver-call` flags a call that names a function of its own module, `send(stream, item)`, when that function changes its first argument. It suggests `stream.send(item)`. Only `low lint` applies it.

A function changes a parameter when its body does one of these:

- assigns to it or through it
- passes the parameter or something reached through it as a `mut` argument
- passes a pointer reached through it to a function that changes that parameter, to a function value or to a function without a body

Building an enum variant around the pointer changes nothing, and neither does passing it to a builtin.

These calls are not flagged:

- a call through a module name
- a call to a function without a body
- a call already written on its first argument
- a call whose first argument is a struct with a field of the function's name

#### Lint: large-struct

`large-struct` flags a struct of at least 64 words. It is reported at its declaration with its size, since every copy of it moves every word.

It suggests splitting it into smaller structs or holding its larger parts behind a pointer. An `extern struct` and a generic struct are not flagged.

#### Lint: prefer-const

`prefer-const` flags a function that has no parameters or type parameters and returns a number, `bool` or `str`. The function must never be used as a value, and its body must be one expression that a constant can hold: literals, constants, operators and numeric conversions.

It suggests `const name: T = ...`, with the `()` dropped where the function is called.

#### low doc output

`low doc dir` prints, as JSON, every module under the directory. `low doc std` documents the standard library. The website builds its package pages from it with `--target linux/amd64`. `--target` picks the operating system whose files are documented and defaults to the host.

| Each module has | With |
| --- | --- |
| Its imports | Each with the name it is used through. |
| Its public structs, enums, constants and functions | Each with its declaration as written, its file and its line. |
| Its examples | Their name, source and expected output. |

A struct shows its public fields, and `...` stands for its private ones. A function whose first parameter is a public type of its module, or a pointer to one, names that type as its receiver.

#### low doc index output

`low doc index` prints one JSON array. It holds a `source` and its `versions` for every package of the package index that `LOW_INDEX` names, in the order of the sources. An unset or absent `LOW_INDEX`, or an index without packages, prints `[]`.

Each version has its `tag`, `commit` and `hash` as the index holds them, `yanked`, and the `modules` of its tree as `low doc dir` prints them. Versions are listed newest first, and a yanked version is listed like any other.

A source is a file of the index with a directory above it, so a file at the top of the checkout is not a package.

Each version's tree comes from the module cache. It is fetched and checked against the index's hash when it is not there yet. A failure to fetch or parse a version fails the command.

The modules are the ones an import reaches. These are the directories that hold `.low` files for `--target` other than tests, skipping `testdata`, `examples` and directories that start with `.` or `_`, inside the `modules` directory of the tree's `build.low`.

A module's `path` is the path it is imported by: the source for the modules directory itself, and the source, `/` and the module's directory below it for a module below it.

#### low names output

`low names file.low` type-checks the program and prints one JSON array of every name the file mentions. The website build highlights and links every snippet from it.

| Key | Holds |
| --- | --- |
| `line`, `column` | The line and byte `column`, both counted from 1. |
| `name` | The name. |
| `hover`, `doc` | The `hover` text and `doc` the language server shows for it. |
| `definition` | `path`, `line`, `column`, and `local` when the definition is in the same file. |
| `first` | The array index of the first name that resolves to the same definition. |

A key is absent when the name has no such fact.

Names with the same `first` are one reference group. A field written in shorthand, as `x` in `Point { x }`, groups with the field. The group of the local it reads starts at the shorthand when that comes first.

A program that does not check prints the error and exits with status 1. `--target` picks the operating system whose files are checked. It defaults to the host.

### Projects

```low
import std/build

build.title("Test App")
build.version("0.3.0")
build.dep("github.com/acme/ui", "^0.4")
build.local("../shared")
build.modules("src")
build.program("tool")
build.gui("src/app")
build.icon("assets/app.ico")
build.manifest("assets/app.manifest")
build.file_info("CompanyName", "Example Ltd")
build.file_info("ProductName", "Test App")
build.pre_build("npm", ["install"])
build.post_build("npm", ["run", "clean"])
```

A `build.low` at the project root describes the project: its name and version, its dependencies, where its modules live, the programs it builds and the commands to run around a build.

The compiler reads `build.low` from its source and never runs it. It therefore holds only `import std/build` and top-level calls of `std/build` functions. Their arguments are string literals, or slices of string literals for a hook's arguments. Anything else is an error at its position.

| Declaration | Declares | Times |
| --- | --- | --- |
| `title(name)` | The project's name. | At most once |
| `version(number)` | The project's semantic version. | At most once |
| `dep(source, range)` | A dependency fetched from a `git` repository. | Any |
| `local(dir)` | A dependency in a directory on disk. | Any |
| `modules(dir)` | The directory that holds the project's own modules. | At most once |
| `program(dir)` | A console program. | Any |
| `gui(dir)` | A windowed program. | Any |
| `icon(file)` | The icon of the Windows executables. | At most once |
| `manifest(file)` | The application manifest of the Windows executables. | At most once |
| `file_info(key, value)` | One string of the Windows version information. | Once per key |
| `pre_build(program, args)` | A command to run before a build. | Any |
| `post_build(program, args)` | A command to run after a build. | Any |

Warnings are printed for the project's own modules only.

#### Dependencies

`dep(source, range)` names a dependency kept in a `git` repository at `https://<source>`. Its versions are the repository's tags that parse as semantic versions, with or without a leading `v`.

The source names directories in the module cache. Its `/`-separated segments are therefore not empty, `.` or `..`, and it holds no `\`, `:` or `@`.

| Range | Accepts |
| --- | --- |
| `^1.2` | The versions up to the next change of the first non-zero part. |
| `~1.2.3` | The versions up to the next minor version, or the next major version for `~1`. |
| `=1.2.3` | That version alone. |
| `1.2` | The same as `^1.2`. |

A pre-release version is accepted only by an `=` range that names it.

`local(dir)` names a dependency in a directory, relative to the `build.low` that declares it; `dir` may name that directory's `build.low`. It has no version and is never fetched.

A dependency's hooks never run.

#### Importing a dependency

A dependency's modules are imported under its name: the source for a `dep`, the directory's last segment for a `local`.

| Import | Loads |
| --- | --- |
| `import github.com/acme/ui` | The dependency's modules directory, which is its root unless its `build.low` declares `modules`. |
| `import github.com/acme/ui/text` | Its `text/` module. |
| `import text`, inside a dependency | Its own `text/`, so two projects may each hold a module of one name. |

- A project imports only its own modules and those of the dependencies its `build.low` names.
- A module path that is both one of the project's directories and a dependency is an error.
- One name given to two dependency directories is an error.
- Of a dependency's declarations only `modules` is used. Its `program` and `gui` are checked and ignored.

#### The modules directory

`modules(dir)` names the directory under the project root, `.` by default, that holds the project's own modules, so that a project with its code under `src/` imports `lib` and not `src/lib`. It is declared at most once and has the shape of a `program` directory.

It changes where an import is placed and what `low test` calls a module, and nothing else:

- The project root stays the nearest `build.low` or `low.mod`.
- Hooks, `build.lock`, `dep` and `local` stay relative to the `build.low`.

A `low.mod` or `build.low` directly inside the modules directory would take over as the root of everything below it. Loading the project, or the programs it declares, is therefore an error: `<dir>/low.mod marks another project root inside the modules directory`. A build of a path that starts below such a marker finds that marker as its root and reads no declaration of the outer project.

#### Programs

`program(dir)` declares a console program in the directory `dir`, and `gui(dir)` declares a windowed program there. The directory is relative to the `build.low`. Either may repeat.

- `dir` is not empty, absolute, rooted or drive-lettered, such as `/x`, a backslash first, or `C:/x`. It has no `..` segment. `\` counts as `/`.
- A directory named by two declarations is an error.
- A declared directory must exist when a program of the project is built, run or installed.
- A dependency's declarations are checked and otherwise ignored.

With no program named, the commands pick from the declared programs:

| Command | Builds |
| --- | --- |
| `low install` | Every declared program. |
| `low build`, `low run` | The one declared. |

`low build` and `low run` do this only in a directory that holds no `.low` file and whose project declares programs. Any directory of the project qualifies, not only the root. `low run` then takes no program arguments. A directory with `.low` files builds as it did without a declaration.

`low build`, `low run` and `low install` link a program the `build.low` declares with `gui` with the GUI subsystem for `windows/amd64`. This holds whichever way its directory is named, as a directory or as a file in it. `--subsystem` on `low build` overrides the declaration. The other targets ignore it. `low test` never reads it, so a test program is a console program.

#### Windows resources

Every program of the project that is linked for `windows/amd64` carries what `icon`, `manifest`, `version` and `file_info` declare, console or windowed. `low test` and a dependency's declarations add no resources.

**Icon.** `icon(file)` names a Windows `.ico` file relative to the `build.low`. It is written as a `program` directory is, and is declared at most once. Every program of the project that `low build`, `low run` and `low install` link for `windows/amd64` carries its images as the executable's icon. Explorer, a shortcut and a pinned taskbar button show that icon. A window's own icon is set apart from it, through `std/ui`.

**Manifest.** `manifest(file)` names the XML application manifest of the Windows executables. It is relative to the `build.low` and declared at most once, like `icon`. An application asks in it for per-monitor DPI awareness, long path awareness, the common controls, the UTF-8 code page and an execution level. The executable carries it as the resource `RT_MANIFEST` number 1, so it needs no sidecar `.manifest` file. The file is written as it is, and must be well-formed XML whose root element is `assembly`.

**Version information.** `version(number)` and `file_info(key, value)` give the executables their version information. The file properties dialog and installers read it. A project that declares neither `version` nor `file_info` has no version information.

- `version` sets the strings `FileVersion` and `ProductVersion` to the version in its canonical form, `1.4.0` or `1.4.0-rc.1`.
- `file_info` sets the string `key` to `value`. Keys include `CompanyName`, `ProductName`, `FileDescription`, `InternalName`, `OriginalFilename` and `LegalCopyright`. It may repeat for different keys but not for one key.
- A `file_info` of `FileVersion` or `ProductVersion` overrides the string `version` gave, so it can carry a fourth number.
- The numeric file and product version of the fixed part come from the leading dotted numbers of `FileVersion` and `ProductVersion`, up to four and zero when absent, so `1.4.0-rc.1` is `1.4.0.0`. Each of those numbers is at most 65535.

**Errors.** For any target, each of these is an error at the declaration:

- a declared file that is missing or is a directory
- an icon file that is not an icon file, holds no image or names an image outside itself
- a manifest that is not well-formed XML with an `assembly` root
- version information that is too large for its block or has a version number above 65535

#### Build hooks

`pre_build(program, args)` and `post_build(program, args)` run a program in the project root, with the streams of `low`. They run before and after `low build`, `low run` and `low install` compile the program, in the order declared.

A non-zero exit stops the command with `pre_build <program> exited with status <n>`. `low test` and the other commands run no hooks.

#### Resolving versions

`low build`, `low run` and `low test` resolve the version-control dependencies of the project and of every dependency's `build.low`, local or fetched.

- Each source gets the newest version that every range naming it accepts.
- If `build.lock` holds a version they all accept, that version is kept instead.
- `low update [dir]` resolves again to the newest accepted versions and rewrites `build.lock`.

The other commands and the language server read the trees in the cache through `build.lock`. They report a dependency that `build.lock` does not list, or whose tree is not in the cache, at its `dep`.

#### build.lock

`build.lock` holds one line per dependency, `source version commit hash`, sorted by source. It is written when the resolution changes.

`hash` is `h1:` followed by the standard base64 of the SHA-256 of a listing of the dependency's tree. The listing has one line per regular file, sorted by path. Each line is made of the file's SHA-256 in lower-case hexadecimal, two spaces, the path relative to the tree root with `/` separators, and a newline. This is the `h1` hash of Go's `dirhash`.

A published version never changes, and two checks enforce it:

- A tree written to the cache fails with an error if its hash differs from the one `build.lock` holds for that version. The error names the source, the version, and the expected and actual hashes.
- When a resolution lists a source's tags and a tag that `build.lock` holds points at another commit, it fails with an error. That error names the source, the tag, and the old and new commits.

The hash is checked when a tree is written to the cache, not on each use.

#### The module cache

Fetched dependencies live in a module cache shared by every project of the user.

| Location | Used |
| --- | --- |
| `LOW_CACHE` | When the environment variable is set. |
| `$XDG_CACHE_HOME/low` or `$HOME/.cache/low` | Otherwise, on Linux. |
| `%LOCALAPPDATA%\low` | Otherwise, on Windows. |

| Path | Holds |
| --- | --- |
| `<cache>/git/<source>` | A bare mirror of the repository. |
| `<cache>/deps/<source>@<commit>` | The tree of one commit. |

A tree is written from `git archive` with line endings untouched and placed by a rename, so a tree is either complete or absent and projects that fetch at once share it. A tree already in the cache is reused.

A tree written to the cache holds only regular files and directories. Fetching a tag whose archive holds a symbolic link, a hard link or a device fails with an error that names the entry, so the hash covers everything the tree contains.

In a module of a fetched dependency, `embed` and `embed_dir` fail with an error when the file's absolute path, or its real path once symbolic links resolve, is outside the dependency's tree. This covers `..` segments, absolute paths and links. A `local` dependency of a fetched dependency is confined to the same tree. The root project and the `local` dependencies of the root project are not confined.

#### The package index

The package index is a git repository of plain files. `LOW_INDEX` names a checkout of it. With `LOW_INDEX` unset, or without a file for a source, the source resolves from the repository's tags.

A package is named by its source, `github.com/acme/ui`. Its file is `<index>/<source>`, so no segment of a source may start with `.`. Each line is `tag commit hash`, with `yanked` after the hash for a yanked version.

| Field | Holds |
| --- | --- |
| `tag` | The tag, spelled as the repository spells it. |
| `commit` | The commit the tag names. |
| `hash` | The `h1:` hash of that commit's tree as `build.lock` holds it. |

Lines are appended in the order of publication and never edited, except for the flag. The checkout is read as it stands, with no network, and kept current with `git pull`.

A source that the index lists takes its versions, commits and hashes from the index instead of from `git ls-remote`, so only an indexed version is chosen. A fetched tree is checked against the index's hash on first use, not only against `build.lock`. A locked version that the index lists with another commit or hash fails as a moved tag does.

#### Publishing

`low publish <source> <version>` adds a version to the index:

1. It reads the tags of the repository at `https://<source>` and takes the one whose version is `<version>`, so the indexed commit is one dependents can fetch.
2. It extracts that commit's tree to a scratch directory, as dependencies are fetched, and hashes it.
3. It resolves the tree's own dependencies there.
4. It requires that the version in its `build.low`, when declared, equals the tag's.
5. It type-checks every module of the tree, the directories that hold `.low` files other than tests. It skips `testdata`, `examples` and directories that start with `.` or `_`, and it fails when there is none.
6. It appends the line to the index and commits that file with the message `publish <source> <tag>`.

A version the index lists under any spelling of its tag is refused, so a tag that moved after publication changes nothing. A failure leaves the index checkout as it was. Pushing the index is left to git.

#### Yanking

`low yank <source> <version>` rewrites only the flag of the line of that version. `--undo` clears it. The change is committed as `yank <source> <tag>` or `unyank <source> <tag>`.

Yanking withdraws a version from new resolutions and never deletes a line, so the history of the index lists every version ever published.

- A resolution skips a yanked version when it chooses one, `low update` included.
- An error that finds only yanked versions in range names them.
- A version `build.lock` holds that the index has yanked still resolves, builds and keeps its tree. `low build`, `low run` and `low install` print `warning: <source> <version> is yanked from the package index` once on standard error for it.

### Doc comments

A line starting with `///` documents the declaration, struct field or enum variant directly below it.

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

Consecutive `///` lines form one doc, written in Markdown. One space after `///` is dropped.

A `///` line anywhere else is an error. `////` starts an ordinary comment.

| Tool | Use of a doc |
| --- | --- |
| The language server | Shows it under the signature on hover. |
| `low doc` | Prints it. |
| The website | Renders it on the package pages. |

### Generics

Functions, structs and enums take type parameters in brackets after their name. A constraint on a parameter says what the code may do with a value of that type.

<!-- prelude
struct Pair[A, B] {
    first: A
    second: B
}
-->
```low
struct Stack[T] {
    items: T[]
}

enum Tree[T: Ord] {
    Node(*Tree[T], T, *Tree[T])
    Leaf
}

fn max[T: Ord](a: T, b: T) -> T {
    if a > b { a } else { b }
}

fn sum[T: Number](xs: T[]) -> T {
    var total: T = 0
    for x in xs {
        total += x
    }
    total
}

fn sort[T](mut xs: T[], less: fn(T, T) -> bool) { ... }

print(max(3, 9))
print(max[u8](200, 100))
var s = Stack[str] { items: [] }
let p = Pair { first: 1, second: "x" }
let t: Tree[i64] = Leaf
let pick: fn(i64, i64) -> i64 = max
```

Generic types are written with arguments: `Stack[i64]`, `geometry.Labelled[str]`.

#### Type constraints

A constraint lists what a type parameter supports, joined with `+`.

| Constraint | Allows | Met by |
| --- | --- | --- |
| `Eq` | `==` and `!=`. | Numbers, `bool`, `str`, pointers, channels, and structs and enums whose fields are all `Eq`. |
| `Ord` | Adds `<`, `<=`, `>` and `>=`. | Numbers and `str`. |
| `Hash` | Adds use as a map key. | Integers, `bool`, `str` and pointers. |
| `Number` | Adds arithmetic, numeric literals, conversion with `T(x)` and the `f`, `e` and `n` formats and pictures of interpolation. | |
| `Integer` | Adds bitwise operators and shifts and the `d`, `x`, `X` and `b` formats. It implies `Hash`. | |
| `Float` | | `f64` and `f32`. |

An unconstrained parameter can only be moved, stored, passed, printed and interpolated, with a width but no format, each instance rendering its own type. Anything else a generic function needs, such as an ordering of its elements, is passed in as a function value.

A generic body is checked once against its constraints, so a missing constraint is reported at the definition, not at a use.

Each generic function's type parameters are its own. Two functions may both name a parameter `T`, and a function type mentioning one is distinct from the same text mentioning the other.

#### Type arguments

Type arguments are inferred from the arguments, then from the expected type. They can be written explicitly: `max[u8](a, b)`, `Stack[str] { ... }`.

| Generic use | Takes its arguments from |
| --- | --- |
| A struct literal or enum variant | The expected type or its field values. |
| A function used as a value | The expected function type. |

Each distinct set of type arguments compiles to its own copy, `max[i64]` and `max[f64]`, so generic code runs as fast as code written for one type.

Values of different kinds that share behaviour, such as the elements of a UI tree, are structs of function values whose closures capture each kind's state.

### Concurrency

`spawn` starts a green thread, and green threads pass values to each other over channels.

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

#### Green threads

`spawn f(args)` starts a green thread running the call.

- The function value and the arguments are evaluated before it starts.
- `mut` arguments are not allowed.
- A function that can fail must have its error handled inside the spawned call.

A program ends when `main` returns, whatever other green threads are doing.

| Condition | Panic |
| --- | --- |
| Every green thread is waiting and nothing can wake one | `every green thread is blocked forever` |
| A green thread exhausts its stack | `stack overflow` |

#### Channels

A channel is a reference: assigning or passing it shares it.

| Form | Does |
| --- | --- |
| `chan[T](n)` | Makes a channel holding up to `n` values. |
| `chan[T]()` | Makes an unbuffered channel, so every send waits for a receiver. |
| `c <- v` | Sends, waiting while the buffer is full. |
| `<-c` | Receives a `T?`, waiting while the channel is empty. It gives `none` once the channel is closed and drained. |
| `close(c)` | Wakes every waiter. |
| `for v in c` | Receives until the channel is closed and drained. |

Sending on or closing a closed channel panics.

#### Select

`select` waits on several operations at once and runs the arm of the first that can proceed. It is an expression like `match`.

| Arm | Runs when |
| --- | --- |
| `name = <-c`, `<-c` | The receive can proceed. |
| `c <- v` | The send can proceed. |
| `after(ms)` | `ms` milliseconds pass. |
| `else` | Nothing is ready. Without `else` the green thread waits. |

Channel and value expressions are evaluated first, in order. Ready arms are taken in order.

`after` is a contextual keyword. It is an ordinary identifier everywhere except at the start of a `select` arm followed by `(`.

#### Sleeping and waiting on descriptors

`sleep(ms)` pauses the green thread.

`wait_readable(fd)` and `wait_writable(fd)` wait until a file descriptor is ready, using epoll on Linux. On Windows `fd` is a waitable handle and both wait until it is signalled. One green thread may wait on a descriptor at a time.
## Errors

An error type is an enum. A function that can fail declares `-> T ! E`.

```low
enum ParseError {
    Empty
    BadDigit(at: i64)
}

fn parse(s: str) -> i64 ! ParseError {
    if s.len == 0 { fail Empty }
    var n = 0
    for i, c in s {
        if c < '0' || c > '9' { fail BadDigit(i) }
        n = n * 10 + i64(c - '0')
    }
    n
}

fn total(a: str, b: str) -> i64 ! ParseError {
    parse(a)? + parse(b)?
}

fn validate(n: i64) ! ParseError {
    if n < 0 { fail BadDigit(n) }
}

enum ConfigError {
    Port(ParseError)
    Missing
}

fn describe(e: ParseError) -> ConfigError {
    Port(e)
}

fn port(s: str) -> i64 ! ConfigError {
    let n = parse(s) catch Port
    let m = s |> parse() catch describe
    let k = parse(s) or fail Missing
    n + m + k
}

fn main() ! ParseError {
    let n = parse("42") catch {
        Empty => 0
        BadDigit(at) => at
    }
    let m = parse("") or -1
    let o: i64? = parse("") or none
    validate(n)?
    print("{n} {m} {o ?? 0}")
}
```

| Form | Does |
| --- | --- |
| `fail v` | Returns the error `v`. |
| Postfix `?` | Returns the error to the caller. |
| `catch { arms }` | Matches the error. |
| `catch M` | Maps the error and returns it to the caller. |
| `or value` | Discards the error and gives `value`. |

`fn f() ! E` fails or returns nothing.

A result cannot be discarded, bound or passed on without `?`, `catch` or `or`, except by returning it from a function with the same result type.

### Raising and passing on an error

`fail v` returns the error `v`. `v` is checked against the function's error type, so bare variant names work.

Postfix `?` returns the error to the caller, whose error type must be the same enum.

### Handling an error

`catch` is for when the error matters.

- `catch { arms }` matches the error exhaustively. A binding arm such as `e => report(e)` receives the whole error.
- `catch M` returns the error to the caller after mapping it. The mapping uses `M`, a variant or a function written as a name or a qualified name.

`parse(s) catch Port` is `parse(s) catch { e => fail Port(e) }`. `M(e)` must have the caller's error type, so `M` is a one-field variant of that enum or a function value of type `fn(E) -> F`. The mapping runs before the deferred statements.

`or value` is for when the error does not matter. It discards the error and evaluates `value` in its place. The value may leave instead, as in `or fail Missing`, `or return none` or `or break`.

Where an optional `T?` is expected, `or` and `catch { arms }` produce one, so the fallback or an arm may be `none`.

### Errors from main

When `main` declares `! E`, or top-level statements give it one, and returns an error, the program prints `error: <Variant>` to stderr and exits with status 1. Panics exit with status 2, as does `panic(message)`.

### Result layout

A result is a tag word (0 ok, 1 error) followed by the larger of the ok value and the error enum.

### Compile errors

The checker reports every error it finds in one run, up to 50, sorted by file, line and column. A syntax error, an import error or an error in a declaration's signature, fields or variants stops the build alone.

- An error inside a function ends the check of its statement. The checker goes on with the next statement, so two mistakes in one function are both reported. Checking the function stops at its end: a function with an error gets no return check, and the move and read-only passes leave it out.
- A `let` whose statement failed still binds its names, as poisoned locals. A later use of a poisoned name fails quietly, so one root mistake gives one error, not one per use.
- A block with a failed statement fails quietly too, after its tail is checked, so its type never reaches the expression around it.
- `--max-errors n` stops after `n` errors. `--max-errors 1` stops at the first error.
- Instances of a generic function whose body failed are not checked, and an error repeated with the same position, code and message is reported once.

### Diagnostic codes

Every compile error and warning carries a code, printed after the message in brackets: `main.low:2:5: break outside of a loop [break-outside]`. The code names one kind of mistake and keeps its name when the message is reworded, so a tool can act on it without reading the message. `low explain <code>` prints its row.

`compiler/diag/codes.low` holds the same table. `compiler/tests/check` fails when the two tables differ, when the compiler emits a code with no row, and when no part of the compiler emits a row's code.

| Code | Severity | Meaning | Example |
| --- | --- | --- | --- |
| `ambiguous` | error | A method call matches functions in more than one imported module. | `xs.first()` where two imported modules declare `first` for `xs`'s type |
| `argument-type` | error | A builtin is given an argument of a type it does not take. | `str(42)` |
| `arity` | error | A call, variant or pattern has the wrong number of arguments or fields. | `fn f(a: i64) {}` then `f(1, 2)` |
| `bind-type` | error | A binding is given a value that cannot be stored, such as `void`. | `let x = print("a")` |
| `bound-twice` | error | A destructuring pattern binds one name twice. | `let { x: v, y: v } = point`, where `point` has fields `x` and `y` |
| `break-outside` | error | `break` or `continue` appears outside a loop. | `fn main() { break }` |
| `builtin-name` | error | A function or constant takes the name of a builtin. | `fn print(x: i64) {}` |
| `c-callback` | error | A function passed to C as a callback has a parameter or result C cannot pass. | `extern` callback taking a `str` |
| `capture` | error | A closure captures a `mut` parameter. | `fn f(mut n: i64) { let g = fn() { n += 1 } }` |
| `catch-form` | error | `catch` is followed by something other than arms or a mapper. | `parse(s) catch 0` |
| `char-literal` | error | A character literal is empty, invalid UTF-8 or holds more than one character. | `'ab'` |
| `closure-result` | error | A closure that uses `?` or `fail` does not declare its result. | `import std/text` then `let f = fn(s: str) text.parse_int(s)?` |
| `const-cycle` | error | A constant's value refers to itself. | `const a = a + 1` |
| `const-in-function` | error | `const` appears inside a function body. | `fn main() { const n = 1 }` |
| `const-pattern` | error | A match arm names a constant and binds fields. | `limit(x) => x` where `limit` is a constant |
| `const-type` | error | A constant's value is not a number, bool or str. | `const xs = [1, 2]` |
| `constraint` | error | A type argument does not satisfy the constraint on its parameter. | `max[bool](a, b)` where `max` needs `Ordered` |
| `conversion` | error | A value cannot be converted to the requested type. | `i64("a")` |
| `default-cycle` | error | A field default refers to itself. | `struct S { a: i64 = S {}.a }` |
| `defer` | error | A deferred statement nests `defer`, leaves its scope or returns. | `defer { return }` |
| `dependency` | error | A `build.low` dependency is missing, unfetched or clashes with another. | `build.local("vendor/shapes")` in `build.low` with no `vendor/shapes` directory |
| `derive` | error | Derived serialization cannot be generated for a type or field. | `json.encode(f)` where `f` is a function value |
| `destructure` | error | A destructuring pattern is applied to a value that is not a struct. | `let { x } = 1` |
| `division-by-zero` | error | A constant expression divides by zero. | `const n = 1 / 0` |
| `doc-comment` | error | A `///` comment is not followed by a declaration, field or variant. | `/// note` at the end of a file |
| `drop-binding` | error | A value with `drop` is bound with `let`, or a value without `drop` with `using`. | `let f = file.open(p)?` where the file has `drop` |
| `duplicate-arm` | error | A match has two arms for one variant. | `A => 1` written twice in one `match` |
| `duplicate-field` | error | A struct literal or pattern names one field twice. | `struct P { x: i64 }` then `let p = P { x: 1, x: 2 }` |
| `duplicate-import` | error | Two imports in one file use the same name. | `import std/text` then `import text = std/path` |
| `embed` | error | `embed` or `embed_dir` is given something other than an existing path literal. | `embed("missing.txt")` |
| `empty-interpolation` | error | A string holds `{}` with nothing inside. | `"a {} b"` |
| `empty-pattern` | error | A destructuring pattern has no fields. | `struct P { x: i64 }` then `let {} = P { x: 1 }` |
| `empty-type` | error | A struct has no fields or an enum has no variants. | `struct Empty {}` |
| `error-type` | error | An error type is not an enum. | `fn f() ! i64 {}` |
| `escape` | error | An escape sequence in a string or character literal is malformed. | `"\q"` |
| `extern-name` | error | An `extern` library or C symbol name is invalid. | `extern "lib c" { }` |
| `extern-type` | error | An `extern` declaration uses a type C cannot pass. | `extern "c" { fn puts(s: str) -> i32 }` |
| `field-tag` | error | A field tag is not a plain string literal. | `struct P { x: i64 "{1}" }` |
| `float-literal` | error | A float literal cannot be read or does not fit `f64`. | `1e999` |
| `format-spec` | error | A format spec in an interpolation is malformed. | `"{x:q}"` |
| `format-type` | error | A value cannot be formatted with the given spec. | `"{true:x}"` |
| `has-drop` | error | A value with `drop` is copied where it would be dropped twice: captured, spawned, stored or destructured. | `spawn worker(conn)` where `conn` has `drop` |
| `immutable` | error | A `let`, `const` or non-`mut` parameter is assigned or passed as `mut`. | `let n = 1` then `n = 2` |
| `import-cycle` | error | Modules import each other in a cycle. | `a` imports `b` and `b` imports `a` |
| `import-examples` | error | An import names a directory under `examples`. | `import examples/hello` |
| `import-order` | error | An import follows a declaration. | `fn f() {}` then `import std/text` |
| `import-path` | error | An import path or name is not written as identifiers. | `import color-names` |
| `infer-type` | error | A type cannot be inferred and needs an annotation. | `let xs = []` |
| `infinite-size` | error | A struct or enum contains itself by value. | `struct Node { next: Node }` |
| `instance-limit` | error | A generic is instantiated with too many distinct type arguments. | a generic function called at thousands of types |
| `label-shadow` | error | A loop label repeats the label of an enclosing loop. | `outer: for { outer: for { } }` |
| `label-target` | error | A label is not followed by a `for` loop. | `outer: print(1)` |
| `large-struct` | warning | A struct is large enough that every copy is costly. | a struct of 64 or more words |
| `loop-bindings` | error | A `for` loop binds more names than its iterable gives. | `for a, b in 0..3 {}` |
| `main-signature` | error | `main` takes parameters or returns a value. | `fn main(x: i64) {}` |
| `main-use` | error | A function, drop or field default uses what the statements outside a function produce. | `fn f() { main() }` beside top-level statements |
| `manifest` | error | A `build.low` holds something its reader does not accept. | `version("one")` in `build.low` |
| `map-key` | error | A map key type is not an integer, bool, str or pointer. | `let m: [f64: i64] = [:]` |
| `match-pattern` | error | A match arm's pattern does not fit the subject. | `enum E { A }` then `match E.A { _(x) => 1 }` |
| `match-subject` | error | The match subject is used outside a match arm. | a value arm written outside `match` |
| `match-type` | error | A `match` subject has no value. | `match print("a") { _ => 1 }` |
| `method-call` | warning | `push`, `pop`, `truncate` or `delete` is called as a function on a `mut` argument. | `var xs = [1]` then `push(mut xs, 2)` |
| `missing-arms` | error | A `match` does not cover every variant. | `match c { Red => 1 }` on a three-colour enum |
| `missing-fields` | error | A struct literal leaves out fields that have no default. | `struct P { x: i64 }` then `let p = P {}` |
| `missing-main` | error | A program has no `main` and no statements outside a function. | `fn f() {}` as the whole program |
| `missing-mut` | error | An argument to a `mut` parameter is not written with `mut`. | `xs.push(1)` written as `push(xs, 1)` |
| `missing-return` | error | `return` without a value in a function that returns one. | `fn f() -> i64 { return }` |
| `module-shadowed` | error | A local hides a module of the same name. | `import std/time` then `fn stamp(time: i64) -> i64 { time.now_nanos() }` |
| `move-borrowed` | error | A value is moved while it is borrowed. | moving `buf` while a slice of it is alive |
| `move-field` | error | A field with `drop` is moved out of its struct. | `using f = owner.file` where `owner` is bound with `using` and `file` has `drop` |
| `mut-aliased` | error | One variable is passed as `mut` twice in one call. | `fn swap(mut a: i64, mut b: i64) {}` then `var n = 1` then `swap(mut n, mut n)` |
| `mut-position` | error | `mut` appears where it is not an argument to a `mut` parameter. | `var n = 1` then `print(mut n)` |
| `name-clash` | error | A function or constant has the name of a type or function. | `fn i64() {}` |
| `no-error-type` | error | `fail`, `?` or a `catch` mapper is used in a function without an error type. | `import std/text` then `fn main() { text.parse_int("1")? }` |
| `not-assignable` | error | An assignment target or `mut` argument is not a place. | `fn f() -> i64 { 1 }` then `f() = 2` |
| `not-callable` | error | A value that is not a function is called. | `let n = 1` then `n()` |
| `not-constant` | error | A constant uses something other than literals, constants, operators and numeric conversions. | `fn three() -> i64 { 3 }` then `const x = three()` |
| `not-generic` | error | Type arguments are given to something that is not generic. | `let x: i64[str] = 1` |
| `not-indexable` | error | A value that cannot be indexed or sliced is indexed. | `let n = 1` then `print(n[0])` |
| `not-iterable` | error | A `for` loop iterates over a value that is not a range, slice, str, map or channel. | `for x in true {}` |
| `not-pointer` | error | A value that is not a pointer is dereferenced. | `*1` |
| `not-result` | error | `?`, `catch` or `or` is applied to a value that cannot fail. | `1?` |
| `not-struct` | error | A struct literal names a type that is not a struct. | `i64 { a: 1 }` |
| `number-separator` | error | A `_` in a number is not between two digits. | `1__000` |
| `one-letter-name` | warning | A binding has a one-letter name. | `let n = 1` |
| `operand-type` | error | An operator is applied to a type it does not support. | `print(true + true)` |
| `optional-optional` | error | `??` ends a line, which continues the expression. | `let x: i64? = none ??` with the right side on the next line |
| `overflow` | error | A literal or constant does not fit its type. | `let b: u8 = 256` |
| `pattern-annotation` | error | A destructuring pattern has a type annotation. | `struct P { x: i64 }` then `let { x }: P = P { x: 1 }` |
| `pipe-target` | error | The right side of `\|>` is not a function or a call. | `x \|> 1` |
| `prefer-const` | warning | A function returns the same value on every call. | `fn limit() -> i64 { 10 }` |
| `prefer-loop-label` | warning | A flag relays a `break` to an enclosing loop. | `done = true` then `break`, tested by the outer loop |
| `prefer-match` | warning | An `if` chain compares one subject against several values. | `let c = 1` then `if c == 1 { print(1) } else if c == 2 { print(2) } else { print(3) }` |
| `private` | error | A private name of another module is used. | `import std/text` then `text.first_in_range("a", 0, 1)` |
| `propagate-type` | error | `?` passes an error of one enum from a function whose error type is another. | `parse(s)?` in a function declared `! ReadError` |
| `pub-extern` | error | An `extern` block is marked `pub`. | `pub extern "c" {}` |
| `receiver-call` | warning | A function that changes its first argument is not called on that argument. | `fn grow(mut xs: i64[]) { xs.push(1) }` then `var xs = [1]` then `grow(mut xs)` |
| `redeclared` | error | A name is declared twice in one scope: a type, function, constant, field, variant, parameter or drop. | `fn f() {}` then `fn f() {}` |
| `regex` | error | A generated regex call is not given one valid pattern literal. | `import std/regex` then `let re = regex.generated("(a")` |
| `region-exit` | error | `return`, `break`, `continue` or `?` leaves a region. | `region { return 1 }` |
| `region-result` | error | A region returns a value that holds region memory it cannot copy out. | `let f = region { fn() -> i64 { 1 } }` |
| `reserved-name` | error | A name starts with `__`. | `fn __f() {}` |
| `return-type` | error | A function body produces a type other than its declared result. | `fn f() -> i64 { "a" }` |
| `return-value` | error | `return` has a value in a function that returns nothing. | `fn f() { return 1 }` |
| `select-else` | error | A `select` has more than one `else`. | two `else` arms in one `select` |
| `spawn-call` | error | `spawn` is not followed by a function call. | `spawn 1` |
| `spawn-mut` | error | `spawn` is given a `mut` argument. | `spawn fill(mut xs)` |
| `target-unsupported` | error | The target lacks what the program uses: system calls, `std/sys` or a C library. | `__syscall(1)` built for `browser/wasm32` |
| `test-signature` | error | A test function is not declared `fn test_name(t: *testing.T)`. | `fn test_add() {}` in a `_test.low` file |
| `top-level` | error | Statements outside a function are in the wrong place or beside `fn main`. | top-level statements in a file that also declares `fn main` |
| `type-arguments` | error | Type arguments are not followed by a call. | `id[i64]` used as a value |
| `type-arity` | error | A generic type or function is given the wrong number of type arguments. | `let c = chan[i64, str](1)` |
| `type-as-value` | error | A generic type is used as a value. | `struct Box[T] { v: T }` then `let b = Box.v` |
| `type-mismatch` | error | A value has a type other than the one expected. | `let n: i64 = "a"` |
| `unexpected-character` | error | The source holds a character no token starts with. | `1 $ 2` |
| `unexpected-token` | error | The parser expected one token and found another. | `fn main( {}` |
| `unhandled-error` | error | A result that can fail is used without `?`, `catch` or `or`. | `parse(s)` as a statement |
| `unknown-constraint` | error | A type parameter names a constraint that does not exist. | `fn f[T: Sortable](x: T) {}` |
| `unknown-field` | error | A struct has no field of the given name. | `struct P { x: i64 }` then `print(P { x: 1 }.z)` |
| `unknown-label` | error | `break` or `continue` names a label no enclosing loop has. | `for i in 0..3 { break outer }` |
| `unknown-module` | error | An import or qualified name refers to a module that does not exist. | `import std/nope` |
| `unknown-name` | error | A name is not declared. | `print(y)` |
| `unknown-type` | error | A type is not declared. | `fn f(x: Nope) {}` |
| `unknown-variant` | error | An enum has no variant of the given name. | `enum Color { Red }` then `print(Color.Purple)` |
| `unmatched-brace` | error | A `}` in a string has no `{` before it. | `"a } b"` |
| `unreachable-arm` | error | A match arm follows an arm that matches everything. | `_ => 0` followed by `A => 1` in one `match` |
| `unterminated-interpolation` | error | A `{` in a string is not closed. | `"a {1 + 2"` |
| `unterminated-string` | error | A string literal is not closed on its line. | `"abc` |
| `unused-label` | error | A loop label is never used. | `outer: for i in 0..3 { print(i) }` |
| `use-after-move` | error | A value with `drop` is used after it was moved, or may have been. | `take(f)` twice, where `f` has `drop` |
| `use-std` | warning | A function repeats one in the standard library. | a hand-written `trim` that matches `text.trim` |
| `using-pattern` | error | `using` is followed by a destructuring pattern. | `using { x } = p` |
| `using-position` | error | `using` appears in a deferred statement or a drop. | `defer { using f = open() }` where `open` returns a value with `drop` |
| `variadic-position` | error | `...` is not the last parameter of an `extern` function. | `extern "c" { fn printf(..., fmt: ptr) -> i32 }` |
| `variadic-value` | error | A variadic `extern` function is used as a value. | `extern "c" { fn printf(fmt: ptr, ...) -> i32 }` then `let f = printf` |
| `variant-index` | error | Variant indexes are missing on some variants, or `.index` is used without them. | `enum E { A = 1, B }` |

## Builtins

Every module sees these names without an import. A function in the program named `pop`, `truncate`, `close`, `sleep`, `wait_readable` or `wait_writable` takes precedence over the builtin. `compiler/check/builtins.low` is the only table of builtin names, parameters, `mut` markers, receivers and handlers. `compiler/tests/check` requires an entry here for each public one.

### `bytes`

`bytes(s: str) -> u8[]` copies a string's bytes into a new slice.

### `close`

`close(c: chan[T])` closes a channel and wakes every waiter. Sending on or closing a closed channel panics.

### `cstr`

`cstr(s: str) -> ptr[u8]` is a NUL-terminated copy of a string, for passing to C.

### `delete`

`delete(mut m: [K: V], key: K)` removes a key from a map. Removing a missing key does nothing.

### `embed`

`embed(path: str) -> str` is the contents of a file, read at compile time. `path` is a string literal relative to the directory of the file that calls `embed`.

### `embed_dir`

`embed_dir(path: str) -> [str: str]` is the contents of every regular file under a directory, read at compile time. The map is keyed by the file's path inside the directory. `path` is a string literal relative to the directory of the file that calls `embed_dir`.

### `exp`

`exp(x: f64) -> f64` is e raised to the power `x`. It takes and returns `f32` too.

### `floor`

`floor(x: f64) -> f64` rounds `x` down to a whole number. It takes and returns `f32` too.

### `index`

`e.index` is an `i64`. It is the index declared with `= n` on the variant an enum value holds. The enum's variants must have indexes; see `### Enums`.

### `len`

`xs.len` is an `i64`: the number of elements in a slice, the number of entries in a map, or the number of bytes in a `str`.

### `load`

`load(p: ptr[T]) -> T` reads one `T` from C memory. A null pointer traps.

### `new`

`new(value: T) -> *T` copies a value onto the garbage-collected heap and returns a pointer to it. A droppable value cannot go on the heap.

### `panic`

`panic(message: str)` writes `panic: ` and the message to standard error and exits with status 2. It never returns, so it can stand wherever a value is expected.

### `pin`

`pin(value) -> ptr` returns the address of a pointer, slice, map or string's data and keeps it alive until `unpin`. Memory C keeps after a call must be pinned.

### `pop`

`pop(mut xs: T[]) -> T?` removes the last element and returns it, or `none` when `xs` is empty. The capacity stays.

### `print`

`print(value)` writes a value of any type to standard output, followed by a newline. The text is the default rendering described under `### Strings`.

### `push`

`push(mut xs: T[], value: T)` appends `value`, growing the capacity to twice plus four when full.

### `sleep`

`sleep(ms)` pauses the green thread for `ms` milliseconds. `ms` is any integer type.

### `sqrt`

`sqrt(x: f64) -> f64` is the square root of `x`. It takes and returns `f32` too.

### `store`

`store(p: ptr[T], value: T)` writes one `T` to C memory.

### `truncate`

`truncate(mut xs: T[], n)` sets the length to `n` and keeps the capacity. `n` is any integer type. It panics unless `0 <= n <= xs.len`.

### `unpin`

`unpin(p: ptr)` releases an address returned by `pin`.

### `view`

`view(p: ptr[T], n) -> T[]` is a slice over `n` elements at `p`, bounds-checked like any slice. `n` is any integer type.

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
| `str` | An immutable pointer and length over UTF-8 bytes. `str(b)` copies a `u8[]` into a string, and `str(p)` copies a NUL-terminated `ptr[u8]`. |
| `ptr` | `ptr[T]` is a nullable pointer to C memory. `ptr` alone is `void*` and converts to and from any `ptr[T]`. |
| `chan` | `chan[T]` is a channel of `T` values. `chan[T](n)` makes a channel holding up to `n` values. `chan[T]()` makes an unbuffered one. |
| `u8x16` | 16 lanes of `u8`, 128 bits. |
| `u8x32` | 32 lanes of `u8`, 256 bits. |
| `u8x64` | 64 lanes of `u8`, 512 bits. |
| `u16x8` | 8 lanes of `u16`, 128 bits. |
| `u16x16` | 16 lanes of `u16`, 256 bits. |
| `u16x32` | 32 lanes of `u16`, 512 bits. |
| `i32x4` | 4 lanes of `i32`, 128 bits. |
| `i32x8` | 8 lanes of `i32`, 256 bits. |
| `i32x16` | 16 lanes of `i32`, 512 bits. |
| `u32x4` | 4 lanes of `u32`, 128 bits. |
| `u32x8` | 8 lanes of `u32`, 256 bits. |
| `u32x16` | 16 lanes of `u32`, 512 bits. |
| `i64x2` | 2 lanes of `i64`, 128 bits. |
| `i64x4` | 4 lanes of `i64`, 256 bits. |
| `i64x8` | 8 lanes of `i64`, 512 bits. |
| `u64x2` | 2 lanes of `u64`, 128 bits. |
| `u64x4` | 4 lanes of `u64`, 256 bits. |
| `u64x8` | 8 lanes of `u64`, 512 bits. |
| `f32x4` | 4 lanes of `f32`, 128 bits. |
| `f32x8` | 8 lanes of `f32`, 256 bits. |
| `f32x16` | 16 lanes of `f32`, 512 bits. |
| `f64x2` | 2 lanes of `f64`, 128 bits. |
| `f64x4` | 4 lanes of `f64`, 256 bits. |
| `f64x8` | 8 lanes of `f64`, 512 bits. |

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
        using ws = websocket.accept(req) or return http.response(400, "")
        let m = websocket.receive(ws) or return http.upgraded()
        websocket.send_text(ws, str(m.data)) or {}
        return http.upgraded()
    }
    http.response(200, file.read_text("index.html") or "missing")
}

fn main() ! io.Error {
    let client = http.client().with_bearer_token("token").with_timeout(5000).build()
    let page = client.get("https://example.com/") or return
    using l = net.listen("", 8080)?
    http.serve(l, handle)?
}
```

`import std/<name>` loads a standard library module. The library is compiled into `low`, and the project root is not searched for it.

Every module except `std/math` and `std/sql/sqlite` talks to the kernel through raw syscalls. `std/math` calls libm and `std/sql/sqlite` calls libsqlite3 through `extern` blocks. `std/math/complex` and `std/math/fft` reach libm through `std/math`. A library is linked only when reachable code calls it, so a program stays static unless it calls one of their C functions.

### Standard library doc comments

Every public struct, enum, function, struct field and enum variant carries a doc comment, and `StdDocumented` in `compiler/tests/compiler` fails on any that does not. A doc is written for a caller reading it on hover:

- The first sentence is the summary: a verb phrase for a function (`Returns the ...`, `Parses ...`), a noun phrase for a type, field or variant.
- Later paragraphs carry the contract the signature cannot: which error variant is returned and when, whether the call blocks, what dropping or closing releases, limits, defaults and edge cases such as empty input or end of input.
- Identifiers and literals are in backticks. A `low` example block appears only where usage is not obvious from the signature, and it type-checks. It stays a few lines long. A program that shows how to use the module goes in the module's `examples` directory, where it runs.
- A doc states the current behaviour as read from the code.

### Module table

One sentence per module. Each name links to the module's page, which holds the rest.

| Module | Contents |
|---|---|
| `std/sys` | The operating system layer: Linux system calls, and kernel32, ws2_32 and the other Win32 libraries on Windows. |
| `std/io` | `Error`, the error type shared by the I/O, codec and protocol modules. |
| `std/file` | Opening: `open`, `create`, `append` and `update` return a droppable `File` whose `handle` is a copyable `FileHandle`. |
| `std/file/watch` | inotify. |
| `std/net` | Addresses: IPv4 and IPv6 `Addr`, `parse_ip`, `format_ip`, `parse_addr`, `format`, `in_network` (tests an address against a CIDR prefix) and `is_global` (false for addresses the IANA special-purpose registries mark as not globally reachable, and for multicast). |
| `std/net/sntp` | An SNTP client (RFC 4330). |
| `std/net/punycode` | Punycode (RFC 3492): `encode` writes a string as ASCII digits without the `xn--` prefix. |
| `std/net/idna` | Internationalised domain names by UTS #46 (Unicode 16.0.0): `to_ascii(domain, Options)` returns the `xn--` form and fails with an `Error` that names the failed check. |
| `std/net/publicsuffix` | The Mozilla Public Suffix List, embedded: `public_suffix` returns a `Suffix` with its name and its `Section` (`Icann`, `Private` or `Unlisted`). |
| `std/quic` | QUIC version 1 (RFC 9000, 9001 and 9002) over UDP. |
| `std/quic/wire` | QUIC encoding with no I/O. |
| `std/quic/recovery` | RFC 9002 loss detection and congestion control with no I/O: `recovery` makes a `Recovery` that `on_sent`, `on_ack`, `on_timeout` and `discard_space` drive per packet number `Space`. |
| `std/net/dns` | DNS messages (RFC 1035, 3596, 2782, 6891, 8659) with no I/O. |
| `std/socks` | SOCKS5 (RFC 1928, with RFC 1929 username and password) and SOCKS4a clients: `dial`, `dial4a`, `tunnel` and `tunnel4a` over an existing connection, and `dialer` and `dialer4a`, which return a `net.Dialer`. |
| `std/ssh` | SSH 2.0 clients and servers. |
| `std/ssh/wire` | The data types of RFC 4251 section 5. |
| `std/ssh/key` | OpenSSH key formats. |
| `std/ssh/agent` | An ssh-agent client over a Unix socket, or on Windows a named pipe. |
| `std/ssh/sftp` | SFTP version 3. |
| `std/mdns` | Multicast DNS (RFC 6762) for the `.local` names of WebRTC host candidates. |
| `std/stun` | STUN (RFC 8489) with no dependency on a socket in its codec. |
| `std/ice` | The ICE agent of RFC 8445 without a socket or a clock. |
| `std/turn` | The TURN client of RFC 8656 over UDP, TCP or TLS, without a socket or a clock, which `std/ice` uses for relayed candidates. |
| `std/ice/session` | An `ice` agent on UDP sockets: `open` binds one socket per interface address and starts a reader green thread per socket and one that runs the agent. |
| `std/dtls` | DTLS 1.3 (RFC 9147) and DTLS 1.2 (RFC 6347) with the WebRTC profile of RFC 8827, without a socket or a clock. |
| `std/dtls/session` | A `dtls` connection run by a green thread over a `Transport` of a send function and a channel of datagrams. |
| `std/rtp` | RTP and RTCP packets (RFC 3550) with no I/O. |
| `std/srtp` | SRTP and SRTCP (RFC 3711) with the profiles `profile_aes128_cm_sha1_80`, `profile_aes128_cm_sha1_32`, `profile_aead_aes_128_gcm` and `profile_aead_aes_256_gcm` (RFC 7714). |
| `std/sctp` | SCTP (RFC 4960) with partial reliability (RFC 3758) and stream reset (RFC 6525), without a socket or a clock, for data channels over DTLS. |
| `std/sctp/session` | An `sctp` association run by a green thread over a `Transport` of a send function and a channel of packets. |
| `std/datachannel` | WebRTC data channels (RFC 8831) and their establishment protocol (RFC 8832) over `std/sctp/session`. |
| `std/sdp` | Session descriptions (RFC 8866) with the attributes of JSEP (RFC 8829, RFC 9429). |
| `std/webrtc` | The WebRTC peer connection over `std/ice/session`, `std/dtls/session`, `std/srtp` and `std/datachannel`. |
| `std/pipe` | Pipelines. |
| `std/cancel` | `Token` with `token`, `child`, `cancel`, `cancelled`, `check` and `done` (a channel that closes on cancellation, for `select`). |
| `std/header` | The header primitives below `std/http`, `std/multipart` and `std/mail`. |
| `std/http` | Server types: `Request`, `Response`, header helpers over `header.Header` and `parse_url`. |
| `std/grpc` | gRPC (the HTTP/2 protocol of grpc.io) over `std/http`: `Channel`, `Server`, `Code`, `Status`, `Metadata`, deadlines, compression, `grpc.health.v1` and the support for the services `std/protobuf/proto` generates. |
| `std/http/websocket` | `accept` (the `101` goes through `http.upgrade`) and `dial` over `ws` and `wss` return a droppable `Socket` that sends a normal close when released. |
| `std/http/sse` | Server-sent events. |
| `std/http/files` | `serve_file` and `serve_file_with` answer a request from a file. |
| `std/http/huffman` | The static Huffman code of RFC 7541 appendix B that HPACK and QPACK share: `decode`, `encode` and `encoded_size`. |
| `std/http/qpack` | QPACK (RFC 9204) with no I/O. |
| `std/web` | A router with groups, middleware and typed JSON endpoints, problem details (RFC 9457), an OpenAPI 3.1 document built from the types, cookie sessions, CSRF protection, API key, Basic and bearer authentication, CORS and response compression. |
| `std/tls` | TLS 1.2 and 1.3 clients and servers. |
| `std/serde` | The `Writer` and `Reader` protocols, `write[T]`, `read[T]` and `schema[T]` (which the compiler derives for every type), and `Field[T, F]`. |
| `std/json` | A reader and writer over UTF-8 bytes, the `Value` document model, `equal`, and `serialize[T]` and `deserialize[T]` with naming policies. |
| `std/json/pointer` | RFC 6901 pointers: `escape`, `unescape`, `parse`, `format`, `get`, `has`, `set`, `add`, `replace`, `remove` and relative pointers. |
| `std/json/patch` | RFC 6902 `apply` (atomic, failing with the operation's index) and `diff`; RFC 7396 `merge_patch` and `merge_diff`. |
| `std/json/path` | RFC 9535 JSONPath: `compile`, `query`, `find`, `values` and `paths`, with slices, filters, `match`, `search`, `length`, `count` and `value`, normalised paths, and syntax errors at a byte offset. |
| `std/json/schema` | JSON Schema 2020-12. |
| `std/cbor` | CBOR (RFC 8949): the `Value` model, a decoder with depth and length limits, a deterministic encoder, diagnostic notation, and `serialize[T]` and `deserialize[T]`. |
| `std/webauthn` | A WebAuthn relying party: registration and authentication options as JSON, registration and authentication verification for ES256, EdDSA and RS256 credentials, and COSE key parsing. |
| `std/sql` | The driver contract: `Connection` is the droppable owner that closes the driver connection when released. |
| `std/sql/driver` | What the drivers share. |
| `std/sql/postgres` | A PostgreSQL driver over protocol 3.0, plain or over TLS. |
| `std/sql/sqlserver` | A Microsoft SQL Server driver over TDS 7.4, plain or over TLS, with SQL or integrated authentication, read-intent logins (`Config.read_only`) and named instances: `connect`, `connect_tls`, `connect_with` and `connect_stream`. |
| `std/sql/mysql` | A MySQL and MariaDB driver over the client/server protocol, plain or over TLS. |
| `std/sql/sqlite` | SQLite through libsqlite3, linked only when a program calls it. |
| `std/sql/pool` | A pool of connections made by an opener. |
| `std/sql/migrate` | Ordered migrations recorded in a `schema_migrations` table with a SHA-256 checksum: `plan`, `with_table`, `status`, `up`, `up_to` and `down`, each migration in its own transaction under a lock. |
| `std/toml` | TOML 1.0.0: a table tree, an encoder, and serde binding. |
| `std/yaml` | YAML 1.2.2: events, a node tree, an encoder, and serde binding. |
| `std/otel` | OpenTelemetry traces, metrics and logs with W3C propagation, OTLP/HTTP protobuf and JSON, OTLP/gRPC and console exporters, and the instrumentation used by `std/http`, `std/web`, `std/sql/driver` and `std/orm`. |
| `std/protobuf` | The protobuf wire format: an encoder with nested messages and packed fields, a field decoder, and `serialize[T]` and `deserialize[T]` for structs whose field tags carry protobuf field numbers. |
| `std/protobuf/proto` | A `.proto` parser for proto3 and proto2, an import loader over a caller-supplied reader, and a generator that writes Low structs and enums with the field tags `std/protobuf` reads. |
| `std/xml` | XML 1.0 with namespaces: a pull reader, a tree, a writer, and serde binding. |
| `std/dbus` | The D-Bus wire format and a client and service over Unix sockets. |
| `std/portal` | A client of the XDG desktop portals over `std/dbus`: file chooser dialogs, `OpenURI`, the settings portal's colour scheme, accent colour and animation preference, and notifications. |
| `std/atspi` | The AT-SPI2 accessibility bridge over `std/dbus`: an application's accessible tree exported on the accessibility bus, with the events a screen reader follows. |
| `std/orm` | Typed queries over field references, row mapping, inserts, updates and deletes, tables created from types, and raw queries mapped onto structs, over an `sql.ConnectionHandle`. |
| `std/compress/deflate` | DEFLATE, zlib and gzip. |
| `std/compress/xz` | xz with LZMA2, and `.lzma`. |
| `std/compress/zstd` | Zstandard (RFC 8878). |
| `std/compress/lz4` | LZ4 frames and raw blocks (`block_compress`, `block_decompress`). |
| `std/compress/brotli` | Brotli (RFC 7932). |
| `std/compress/bzip2` | `compress(data, level)`, `decompress(data, limit)`, a streaming `decode` and an `Encoder`. |
| `std/compress/snappy` | Raw blocks (`compress_block`, `decompress_block`, `decoded_length`) and the framing format with masked CRC-32C (`compress`, `decompress`, `decode`, `Encoder`). |
| `std/compress/codec` | The parts the compressors share. |
| `std/hash` | One-shot and incremental forms of CRC-32, CRC-32C, Adler-32, CRC-64 and xxHash32/64. |
| `std/bits` | Word operations (`popcount`, `trailing_zeros` on the processor bit scan, `leading_zeros`, `bit_length`, `select_in_word` and the 32 and 64-bit rotations `rotl32`, `rotr32`, `rotl64` and `rotr64`) and dense bitsets. |
| `std/bits/roaring` | Roaring bitmaps with the portable serialization format. |
| `std/image` | Bitmaps, pixel operations and resizing, with PNG, JPEG, GIF, BMP, QOI and lossless WebP codecs. |
| `std/crypto` | `random` is a ChaCha20 generator keyed with 32 bytes from the operating system. |
| `std/crypto/limb` | Fixed-width limb arithmetic the big-integer code shares. |
| `std/crypto/drbg` | AES-256 CTR_DRBG (NIST SP 800-90A Rev. 1 section 10.2.1). |
| `std/crypto/hash` | `Algorithm` and cloneable `Digest`, SHA-1, SHA-224, SHA-256, SHA-384, SHA-512, SHA-512/224 and SHA-512/256, HMAC (`hmac`, and `hmac_keyed` for the keyed inner and outer digests that PBKDF2 clones), HKDF and constant-time `equal`. |
| `std/crypto/sha3` | SHA3-224, SHA3-256, SHA3-384, SHA3-512, SHAKE128 and SHAKE256, with incremental `absorb` and `squeeze` for the SHAKE sponges. |
| `std/crypto/aead` | `Aead` values for AES-128-GCM, AES-256-GCM, ChaCha20-Poly1305 and XChaCha20-Poly1305 (`xchacha20_poly1305`, 24-byte nonce). |
| `std/crypto/md5` | MD5 as a `hash.Algorithm`, so HMAC, HKDF and PBKDF2 work over it, and `sum`. |
| `std/crypto/cipher` | AES in CBC, raw (`cbc_encrypt`, `cbc_decrypt`) or with PKCS#7 padding (`cbc_seal`, `cbc_open`, `pkcs7_pad`, `pkcs7_unpad`); in CTR with a 128-bit big-endian counter (`ctr`); and in XTS with ciphertext stealing (`xts`, `xts_encrypt`, `xts_decrypt`). |
| `std/crypto/x25519`, `std/crypto/ed25519` | X25519 key agreement (RFC 7748) and Ed25519 signatures (RFC 8032), over `std/crypto/curve25519`, which also owns the scalar `clamp`, and `std/crypto/edwards25519`. |
| `std/crypto/edwards25519` | The Edwards curve behind Ed25519 and ristretto255. |
| `std/crypto/ristretto255` | The prime-order group of RFC 9496. |
| `std/crypto/ecdsa` | ECDH and ECDSA on P-256 and P-384, over `std/crypto/nistec`, with RFC 6979 nonces and SEC1 private keys. |
| `std/crypto/nistec` | The NIST prime curves P-256 and P-384 that `std/crypto/ecdsa`, `std/crypto/hashtocurve` and `std/crypto/voprf` share. |
| `std/crypto/hashtocurve` | RFC 9380 hashing: `expand_message_xmd` over any `hash.Algorithm`, `hash_to_field` with `L = 48`, and the constant-time `P256_XMD:SHA-256_SSWU_RO_` suite (`map_to_curve_p256`, `hash_to_curve_p256`). |
| `std/crypto/pairing` | Pairing-friendly curves of embedding degree 12 for zero-knowledge proofs and BLS signatures. |
| `std/crypto/pasta` | The Pasta curves Pallas and Vesta, `y^2 = x^3 + 5`, that form a cycle: the scalar field of one is the base field of the other. |
| `std/crypto/bn254` | The BN254 precompiles of Ethereum, EIP-196 and EIP-197, over `std/crypto/pairing`: `ec_add`, `ec_mul` and `ec_pairing`. |
| `std/crypto/bls12381` | BLS12-381 encodings and hashing over `std/crypto/pairing`. |
| `std/zk/groth16` | Groth16 (Groth 2016) over `std/crypto/pairing`, for BN254 and BLS12-381. |
| `std/zk/goldilocks` | The Goldilocks field `p = 2^64 - 2^32 + 1` with generator 7 and two-adicity 32, and its cubic extension `F[x] / (x^3 - x - 1)`, the one Winterfell uses. |
| `std/zk/stark` | A STARK over FRI for the Goldilocks field with the cubic extension for every challenge. |
| `std/zk/airs` | Two `Air`s with their traces. |
| `std/zk/halo2` | Halo2 as `halo2_proofs` 0.3.0 defines it (the PLONKish arithmetisation with the inner product argument, no trusted setup), over Vesta. |
| `std/crypto/voprf` | The oblivious PRF of RFC 9497 in its three modes (`Mode`: `Oprf`, `Voprf`, `Poprf`) for `ristretto255-SHA512` and `P256-SHA256` (`Suite`). |
| `std/crypto/rsa` | RSA PKCS#1 v1.5 and PSS signing and verification, RSAES-OAEP encryption and decryption, and PKCS#1 keys and SubjectPublicKeyInfo. |
| `std/crypto/blake3` | BLAKE3: `sum`, `xof`, `keyed_hash`, `derive_key`, an incremental `Hasher` with `finalize` and seekable `finalize_at`, and `algorithm()`, a `hash.Algorithm` for HMAC. |
| `std/crypto/blake2b` | BLAKE2b (RFC 7693). |
| `std/crypto/otp` | HOTP (RFC 4226) and TOTP (RFC 6238) over SHA-1, SHA-256 and SHA-512: `hotp`, `totp`, `verify_totp` with a window, `verify_hotp`, `generate_secret`, base32 secrets, and `otpauth://` `uri` and `parse_uri`. |
| `std/crypto/shamir` | Shamir secret sharing over GF(2^8) with the AES reduction polynomial `0x11b`: `split` cuts a secret into `n` `Share`s (x coordinates `1..=n`, one y byte per secret byte), any `k` of which recover it. |
| `std/crypto/mlkem` | ML-KEM-512, -768 and -1024 (FIPS 203). |
| `std/crypto/hpke` | Hybrid public key encryption (RFC 9180) with HKDF-SHA-256 and the AEADs AES-256-GCM, ChaCha20-Poly1305 and export-only. |
| `std/crypto/prehash` | The `Hash` enum of the functions FIPS 204 and FIPS 205 accept for pre-hash signing, with `digest` and `oid`. |
| `std/crypto/mldsa` | ML-DSA-44, -65 and -87 (FIPS 204). |
| `std/crypto/slhdsa` | All twelve SLH-DSA parameter sets (FIPS 205): SHA2 and SHAKE, security levels 128, 192 and 256, `s` (small signatures, slow signing) and `f` (fast signing). |
| `std/crypto/falcon` | Falcon-512 and Falcon-1024 key generation, signing and signature verification as specified in the NIST round 3 submission (Falcon specification version 1.2). |
| `std/crypto/fhe/ring` | The residue number system polynomial ring that the homomorphic encryption modules under `std/crypto/fhe` share. |
| `std/crypto/fhe/rlwe` | The ring learning-with-errors keys that `std/crypto/fhe/bfv` and `std/crypto/fhe/ckks` share. |
| `std/crypto/fhe/bfv` | The Brakerski/Fan-Vercauteren scheme over `std/crypto/fhe/rlwe`. |
| `std/crypto/fhe/ckks` | The Cheon-Kim-Kim-Song scheme for approximate arithmetic on complex numbers, over `std/crypto/fhe/rlwe`. |
| `std/crypto/fhe/tfhe` | Gate bootstrapping of Chillotti, Gama, Georgieva and Izabachene (CGGI 2016, 2019). |
| `std/crypto/jose` | JSON Web Tokens: compact JWS with HS256/384/512, RS256/384/512, PS256/384/512, ES256, ES384 and EdDSA, JWK sets, and claim checks. |
| `std/crypto/password` | Password hashes in PHC strings: Argon2id, scrypt (RFC 7914, with a memory limit checked before allocating) and PBKDF2. bcrypt (`$2b$`; passwords over 72 bytes are rejected), and `bcrypt_pbkdf`, the OpenBSD key derivation of OpenSSH private keys. |
| `std/crypto/sasl` | The client side of SCRAM (RFC 5802, RFC 7677) over any `hash.Algorithm`. |
| `std/crypto/der` | A DER cursor and the ASN.1 values the other crypto modules share. |
| `std/crypto/x509` | PEM (`pem_blocks`, `pem_certificates`, `pem_private_key`), certificates, PKCS#8 private keys, the system roots, chain validation and the TLS signature scheme table (`signature_schemes`, `signing_schemes`, `authentication`, `verify_signature`, `sign`). |
| `std/acme` | An ACME client (RFC 8555). |
| `std/crypto/x509/fetch` | The HTTP transport for revocation data, over `std/http`. |
| `std/time` | `now` (Unix seconds), `now_millis` and `now_nanos` (the wall clock at finer resolution), `monotonic` (nanoseconds), `sleep_for` (rounds up to whole milliseconds), `after`, `Timer` and `Ticker` (their channels work in `select`), and `Stopwatch`. |
| `std/time/fault` | `Fault` (`OutOfRange`, `Invalid`, `Syntax(at)`, `Overflow`, `UnknownZone`, `Malformed`, `Unsupported`, `Skipped`, `Ambiguous`, `OffsetConflict`), the error every fallible time function returns, and `to_string`. |
| `std/time/duration` | `Duration`, an exact signed elapsed time with private seconds and nanoseconds. |
| `std/time/timestamp` | `Timestamp`, a point on the UTC timeline from 0001-01-01 to 9999-12-31 with nanosecond precision. |
| `std/time/clock` | `Clock`, closures for the wall reading, the monotonic reading and a cancellable `sleep`, with `system`, `fixed` and `manual`. |
| `std/time/span` | `Span`, a calendar amount of years, months, weeks, days and time units with one sign. |
| `std/time/rounding` | `Rounding` (`Trunc`, `Ceil`, `Floor`, `Expand` and the five `Half` modes) and the helpers the other time modules share. |
| `std/time/calendar` | The proleptic Gregorian calendar the date types share: `leap_year`, `days_in_month`, `days_from_civil` and `civil_from_days` from the Unix epoch, `weekday_index`, and the English month and weekday names, full and short. |
| `std/time/scan` | `Scan`, the read position the time parsers share, with `more`, `at_digit`, `at_byte`, `accept`, `expect`, `digits`, `take_whole` and `take_fraction`, and the fraction text the formatters share. |
| `std/time/date` | `Date` with private fields and validated constructors (`of`, `clamped`, `from_epoch_days`, `from_ordinal`, `from_iso_week`, `from_weekday_in_month`). |
| `std/time/timeofday` | `TimeOfDay` to the nanosecond. |
| `std/time/datetime` | `DateTime`, a civil date and time of day with no zone: `plus` by `Span` or `Duration`, `until` and `since` with a `Diff`, `round`, `truncate`, `to_string`, `parse` (an offset is rejected), and pattern `format` and `parse_with`. |
| `std/time/offset` | `Offset`, a UTC offset in whole seconds within ±25:59:59, with `utc`, `hours`, `hm`, `to_string`, `to_string_z`, `parse`, and `read`, which reads an offset out of the middle of a string and reports where it ended. |
| `std/time/zone` | IANA zones. |
| `std/time/tzdata` | An embedded copy of the database for hosts without one: `source` (for `zone.load_from`), `names` and `version`. |
| `std/time/zoned` | `Zoned`, a `Timestamp` seen through a `Zone` whose offset is always the zone's own. |
| `std/time/pattern` | Compiled strftime and strptime patterns shared by the types above: `compile`, `Names` for other languages, `render` and `scan`. |
| `std/time/humanize` | `ago`, `relative`, `in_words`, `compact` and `round_to` over a `Words` value; English by default. |
| `std/locale` | CLDR 48 locale identifiers. |
| `std/locale/data` | Generated by `compiler/tablegen cldr` and `compiler/tablegen collate`. |
| `std/locale/decimal` | `Decimal`, an exact sign, digit string and exponent with `from_int`, `from_f64` (shortest round-trip), `parse`, `shift`, `strip_zeros`, `pad`, and the rounding every formatter shares. |
| `std/locale/plural` | CLDR plural rules. |
| `std/locale/numfmt` | Numbers by locale: `format`, `format_int` and `format_f64` under `Options`. |
| `std/locale/datefmt` | Gregorian dates and times by locale; other calendars are not provided. |
| `std/locale/list` | `format` of a list as `And`, `Or` or `Unit` at `Wide`, `Short` and `Narrow` widths. |
| `std/locale/reltime` | `format` and `format_int` of a relative amount in seconds, minutes, hours, days, weeks, months, quarters or years, `Numeric` always or auto (`yesterday`), at `Long`, `Short` and `Narrow` widths, with the plural category chosen by the locale. |
| `std/locale/collate` | UTS #10 collation. |
| `std/simd` | Vector reads and writes, `splat`, `mask`, `blend`, `min`, `max`, lane access, byte shuffles and `find_byte`. |
| `std/text` | Parsing and formatting. |
| `std/text/diff` | Myers line diff: `diff`, `lines`, `stats`, `distance`, `similarity`, GNU-compatible `unified` and `hunks`, `patch` and `patch_within` over a unified diff, and word and character level `inline`, `inline_chars` and `inline_markup`. |
| `std/text/table` | `Table` with per-column `Align`, width limits, truncation and wrapping, and `Border` styles `None`, `Ascii`, `Unicode`, `Markdown` and `Tabs`. |
| `std/text/naming` | `words`, `snake`, `kebab`, `dotted`, `screaming_snake`, `camel`, `pascal`, `title`, `sentence`, the matching `is_` predicates, and `slug` and `slug_with` (transliterate Latin letters and cut at a word boundary). |
| `std/text/number` | `format_bytes` and `parse_bytes` in decimal and binary units, `grouped`, `ordinal`, `percent`, `compact`, `si`, English `words`, and `roman` with `parse_roman`. |
| `std/uri` | RFC 3986 percent coding and references. |
| `std/builder` | A `Builder` over a growing byte buffer, appended in place through `mut`. |
| `std/regex` | Linear-time regular expressions in the RE2 style. |
| `std/unicode` | Normalization (Unicode 16.0.0): `nfd`, `nfkd`, `nfc` and `nfkc`; `combining_class`; full case folding with `casefold`; `search_key`, which case-folds, applies NFKD and drops combining marks in one pass. |
| `std/unicode/utf16`, `std/unicode/utf32` | Encoding and decoding of code points, `u16[]` and `u32[]`, and byte forms in either order with BOM handling, strict or lossy, with surrogate validation. `join` combines a surrogate pair, `units` reads a byte form into code units, and `decode_escape` in `utf16` decodes a `\uXXXX` escape with its trailing pair. Their byte forms take `unicode.Order`. |
| `std/charset` | `decode`, `decode_lossy`, `encode`, `encode_replace`, `canonical` (WHATWG labels) and `detect_bom` for UTF-8, UTF-16, UTF-32, ISO-8859-1, -2 and -15, windows-874, windows-1250 to -1258, KOI8-R, IBM437, IBM850, Shift_JIS, EUC-KR, Big5, GBK and gb18030, and `windows_1252` for the code point of one windows-1252 byte. |
| `std/html` | An HTML parser following the WHATWG standard. |
| `std/markdown` | GitHub Flavored Markdown. |
| `std/math` | libm's trigonometric, hyperbolic, exponential and rounding functions, `pi`, `tau`, `e`, `infinity`, `nan`, `is_nan`, `is_inf`, generic `abs`, `min`, `max`, `clamp` and `compare`, and integer `ipow`, `gcd`, `floor_div` and `floor_mod`. |
| `std/math/stats` | Statistics over `f64[]` that fail with `Error` (`Empty`, `TooShort`, `Mismatch`, `NotFinite`, `Range`, `Bins`, `Edges`, `ZeroVariance`) instead of returning `nan`. |
| `std/math/complex` | `Complex` of two `f64` with `complex`, `add`, `sub`, `mul`, `div` (Smith's method), `neg`, `scale`, `conj`, `abs` (`hypot`), `arg`, `polar` and `rect`, `cexp`, `clog`, `csqrt`, `cpow`, `pow_int`, `format` and `format_fixed`. |
| `std/math/matrix` | A dense row-major `Matrix` of `f64`: `zeros`, `identity`, `from_data`, `from_rows`, `clone`, `get`, `set`, `transpose`, `add`, `sub`, `scale`, `multiply` and `multiply_vector`. |
| `std/math/fft` | Discrete Fourier transforms of `Complex[]` of any length: `forward`, `inverse` (scaled by `1 / n`), and for real signals `forward_real` and `inverse_real`, which use the `n / 2 + 1` bins of `numpy.fft.rfft`. |
| `std/slices` | Queries and transforms: `contains`, `index`, `clone`, `concat`, `reverse`, `map`, `filter`, `fold`, `scan`, `any`, `all`, `sum`, `min` and `max` (as optionals), `sorted`. |
| `std/maps` | `keys`, `values`, `sorted_keys`, `entries`, `from_pairs`, `clone`, `merge` (the right map wins), `filter`, `map_values`, `has`, `get_or`, `invert`, `count_by` and `group_by`. |
| `std/os` | Process state: `args` and `environment` (from the argument and environment vectors the program started with), `env` as `str?`, `umask` (sets the creation mask and returns the previous one), `pid` and `exit`. |
| `std/stdio` | `stdin`, `stdout` and `stderr` as `io.Stream`s, and `exit`. |
| `std/os/signal` | `Signal`, the one signal vocabulary of `std/process`, with `number`, `name`, `from_number` and `catchable`. |
| `std/process` | `Command` with arguments, an optional replacement environment, a working directory, `Inherit`, `Piped` or `Null` for each standard stream, an optional new process group, and `windowless`. |
| `std/perf` | Measures a child process through the Linux kernel's performance events. |
| `std/build` | The declarations of a project's `build.low` (see [Projects](#projects)): `title`, `version`, `dep(source, range)`, `local(dir)`, `modules(dir)`, `program(dir)`, `gui(dir)`, `pre_build(program, args)` and `post_build(program, args)`. |
| `std/buildinfo` | What the compiler knew when it built the program. |
| `std/cdp` | The Chrome DevTools Protocol. |
| `std/filepath` | The operating system's paths, as `std/path` handles slash-separated ones: `clean`, `join`, `base`, `dir`, `ext`, `stem` and `is_absolute`, plus `volume` and `is_separator`. |
| `std/path` | Slash-separated path manipulation for module paths, URLs and other paths that are not the operating system's: `clean` (lexical, resolving `.` and `..`), `join`, `base`, `dir`, `ext`, `stem` and `is_absolute`. |
| `std/sync` | `Mutex` with `lock`, `try_lock`, `unlock` and `locked`. |
| `std/retry` | `Policy` with attempts, base, multiplier, maximum delay, `Jitter` (`None`, `Full`, `Equal`, `Decorrelated`), a deadline, a seeded `Rng` and a cancel token. |
| `std/ratelimit` | `Bucket` (token bucket in exact nanosecond units) with `allow`, `allow_n`, `reserve`, `wait` and `set_rate`. |
| `std/singleflight` | `Group[K, V]` with `do`, `do_shared`, `forget` and `in_flight`, which run one call per key while the others wait for its result. |
| `std/atomic` | `Int`, `Bool` and `Flag` built on compare-and-swap, with `load_int`, `store_int`, `add_int`, `swap_int`, `compare_and_swap_int` and `update_int`, and the same forms for `Bool`. |
| `std/rand` | A seedable, non-cryptographic xoshiro256** generator: `seeded`, `rng` (all 256 bits of state from `std/crypto`), `next`, unbiased `below` and `between`, `float`, `chance`, `shuffle` and `pick`. |
| `std/uuid` | Random v4 and time-ordered v7 UUIDs, `format`, `parse` (reports the offset of the first bad character), `version`, `timestamp` for v7, `equal` and `nil`. |
| `std/csv` | RFC 4180 records with quoted fields, doubled quotes, embedded newlines and CRLF or LF line ends. |
| `std/dotenv` | `.env` files. |
| `std/flag` | Command-line flags from `Spec`s with help text, booleans and defaults: `--name value`, `--name=value`, single-dash forms and `--`. |
| `std/log` | Leveled line logging (`debug`, `info`, `warn`, `error`) to a descriptor (stderr by default) as `level prefix: message`. |
| `std/log/syslog` | RFC 5424 syslog. |
| `std/log/journald` | The native protocol of systemd's journal. |
| `std/term` | `is_tty`, `size` and `set_size`. |
| `std/mime` | Media types by file extension and back (`by_extension`, `extension`) for web, image, font, archive, audio, video and subtitle formats. |
| `std/semver` | Semantic Versioning 2.0.0: `parse` (with an optional `v`, failing at the offset of the first bad part), `format`, and `compare` and `less` with prerelease precedence, ignoring build metadata. |
| `std/container/heap` | A binary `Heap[T]` ordered by a `less` function: `insert`, `pop`, `peek` and `size`. |
| `std/container/lru` | A least-recently-used `Cache[K, V]` of fixed capacity: `get` (refreshes the entry), `put` (returns the evicted key), `contains`, `size` and `keys` from most to least recent. |
| `std/container/deque` | A ring-buffer `Deque[T]`: `push_front`, `push_back`, `pop_front`, `pop_back`, `front`, `back`, `at` and `size`, doubling when full. |
| `std/container/set` | `Set[T]` over a map, in insertion order: `set`, `from_slice`, `add` and `remove` (both return whether the set changed), `has`, `size`, `items`, `union`, `intersection`, `difference` and `is_subset`. |
| `std/container/ordered` | An AVL `Tree[K, V]` for ordered keys: `tree`, `put`, `get`, `remove`, `contains`, `size`, `height`, `is_valid`, `first`, `last`, `floor_entry`, `ceiling_entry`, `each`, `each_range`, `keys`, `values`, `entries` and the half-open `range(lo, hi)`. |
| `std/container/sorted` | `Sorted[T]`, an ordered set over `ordered.Tree`: `sorted`, `from_slice`, `add`, `remove`, `has`, `size`, `items`, `first`, `last`, `floor_item`, `ceiling_item` and `range`. |
| `std/archive` | The filesystem layer shared by `std/archive/tar`, `std/archive/cpio` and `std/archive/zip`, built on `std/file` and `std/sys`. |
| `std/archive/tar` | ustar, GNU and pax archives. |
| `std/archive/cpio` | newc, CRC and odc archives, detected per entry on read: a `Reader`, a `Writer` (`add_file`, `add_dir`, `add_symlink`, `add_links`, `finish`), and `extract` and `create` over the filesystem with the safe extraction of `std/archive`. |
| `std/archive/zip` | Zip archives read through the central directory, with zip64, stored and deflate entries, CRC-32 checks and limits on entries, sizes and compression ratio. |
| `std/testing` | `T`, passed to a test `fn(*T)`, with `log`, `error`, `fatal`, `skip`, `require` and `sub`. |
| `std/base32` | RFC 4648 base32 with the standard and extended-hex alphabets: `encode`, `encode_unpadded`, `decode` (case-insensitive, padding optional, failing at the first bad character or on an impossible length), `encode_hex` and `decode_hex`. |
| `std/base58` | The Bitcoin alphabet (`encode`, `decode`), the Flickr alphabet (`encode_flickr`, `decode_flickr`) and Base58Check (`encode_check`, `decode_check`), failing at the first bad character. |
| `std/ascii85` | `encode`, `encode_adobe` (with `<~ ~>`), `decode`, and Z85 (`encode_z85`, `decode_z85`). |
| `std/msgpack` | MessagePack: the `Value` model with timestamps and extensions, a decoder with depth and length limits that reports the offset of an error, the smallest-form `encode`, and `serialize[T]` and `deserialize[T]`. |
| `std/arrow` | Apache Arrow arrays and the IPC stream and file formats. |
| `std/avro` | Apache Avro. |
| `std/parquet` | Apache Parquet files. |
| `std/bson` | BSON documents as `Field[]` with ObjectId, datetime, binary, regex, int32, int64, double and Decimal128 (as bytes): `decode`, `encode`, `get`, `diagnostic`, `new_object_id` and `parse_object_id`. |
| `std/mongo` | A MongoDB driver over `OP_MSG` with `std/bson` documents, plain or over TLS. |
| `std/ini` | INI files with sections, `=` or `:`, `;` and `#` comments, quoted values, continuations and duplicate keys (last wins, or all with `collect`): `parse`, `get`, `get_all`, `sections`, `set`, `format`. |
| `std/properties` | Java `.properties`: `parse`, `get`, `set`, `format`, with `\u` escapes and surrogate pairs. |
| `std/big` | `Int` over 32-bit limbs. |
| `std/big/rat` | Normalised rationals over `big.Int`: `ratio`, `parse` (`a/b`, decimals, exponents), the four operations, `inv`, `cmp`, `to_fixed`, `to_f64`. |
| `std/big/decimal` | A coefficient and an `i32` exponent: exact `add`, `sub` and `mul`, and `div` and `rescale` with `HalfEven`, `HalfUp`, `Down`, `Up`, `Floor` or `Ceiling` rounding to a target exponent. |
| `std/redis` | RESP2 and RESP3 over TCP or TLS. |
| `std/kafka` | A Kafka client over TCP or TLS for brokers of Kafka 2.7 or later, written against the flexible versions of each API. |
| `std/nats` | A NATS client over TCP or TLS. |
| `std/mqtt` | An MQTT 3.1.1 and 5.0 client over TCP or TLS. |
| `std/amqp` | An AMQP 0-9-1 client over TCP or TLS for RabbitMQ and other 0-9-1 brokers. |
| `std/smtp` | An SMTP client. |
| `std/mail` | Message building: `Message` with text and HTML alternatives and attachments, RFC 2047 encoded words, quoted-printable, base64 wrapped at 76, RFC 5322 dates, and rejection of CR and LF in any header. |
| `std/template` | A text template engine over a `Data` model (`to_data[T]` through serde). |
| `std/multipart` | multipart parsing and writing (RFC 7578, RFC 2046). |
| `std/base64` | One encoder, `encode_with(data, alphabet, padded)`, and one decoder, `decode_with(s, alphabet, padding)`, over an alphabet (`standard`, `url_safe`) and a `Padding` rule (`Required`, `Forbidden`, `Optional`). |

### Resource ownership

Every standard library resource that must be disposed of is a droppable owner bound with `using`. `defer` is for work that is not disposal: unlocking a lock, signalling a channel, restoring ambient state and logging.

#### Owners and views

An owner keeps the resource's name (`file.File`, `net.Conn`, `net.Listener`, `net.Socket`, `dbus.Connection`, `tls.Session`, `http.Connection`, `websocket.Socket`, `process.Child`, `cancel.Registration`, `pipe.Reader`, `pipe.Writer`, `sqlite.Database`, `sql.Connection`, `cdp.Browser`, `otel.Sdk`) and holds its view in a public `handle` field. Its `drop` closes through the handle.

A view is named `<Owner>Handle`: `FileHandle`, `ConnHandle`, `ListenerHandle`, `SocketHandle`, `SessionHandle`, `ChildHandle`, `RegistrationHandle`, `ReaderHandle`, `WriterHandle`, `DatabaseHandle`, `ConnectionHandle`, `BrowserHandle` and `SdkHandle`. `SocketHandle` names the view in both `net` and `websocket`.

A view is a copyable struct, or a pointer to the shared state when that state is mutable, so closures capture it, slices, channels, optionals and pointer-held structs store it, and `spawn` takes it. A view never closes anything on its own.

#### Owner and view operations

| Operation | Rule |
| --- | --- |
| Operations a caller uses directly | Take the owner and borrow it: `net.read(c, buf)`. |
| Operations that run after ownership has moved elsewhere (green threads, `io.Stream` closures, pools, callbacks) | Take the view and end in `_on`: `net.read_on(h, buf)`, `net.stream_on(h)`, `net.close_on(h)`. |
| A function that returns a view without an owner | Ends in `_handle`: `net.accept_handle`, `tls.client_handle`, `cdp.launch_handle`. |
| `adopt(h)` | Makes an owner from a view. |
| `detach(mut owner)` | Gives up an owner's responsibility so a function that opened a resource and wrapped it can return the wrapper. |

#### Closing owners, views and streams

Closing is explicit and idempotent from the owner, the view and the stream. The drop after an explicit close does nothing. The closed state lives in the shared view.

Descriptor-based views share an `io.Guard`. Every operation enters it, and `retire` marks it closed and wakes blocked operations. Once the wake has returned, the last operation to leave closes the descriptors.

A view used after its owner closed fails with `NotConnected` and never reaches a reused descriptor. The SQL drivers fail with `sql.Error.Closed` and cdp with `cdp.Error.Closed`.

#### The io.Stream view

`io.Stream` is a view whose `close` closes everything beneath it. In `tls.client(net.stream(c), config)`, closing the session closes the connection, and the connection's own drop does nothing.

#### Droppables and long-lived state

A droppable cannot be stored in a pointer, slice, map, channel, optional or enum variant. It cannot be captured by a closure or passed to `spawn`. Long-lived and shared state therefore holds views.

- The HTTP client's idle connections, its HTTP/2 connections and the reaper's deadlines hold `ConnHandle` and `io.Stream` views and close them explicitly with the `_on` operations.
- A green thread that should own a resource receives its view and binds the owner itself: `http.serve` accepts with `net.accept_handle`, and each spawned connection thread runs `using c = net.adopt(h)`.
- Pump threads that only use a resource take its view and leave it to its owner.
- A scope that opens a resource for a wrapper it returns binds it with `using var` and calls `detach`, as `tls.dial` does.

#### Release of mappings, children and pipes

| Type | When released |
| --- | --- |
| `file.Mapping` | Unmaps its region and has no view. Its `data` slice is valid while the `Mapping` is held. A copy used after the release faults. |
| `process.Child` | A `process.Child` that was not waited for is killed and reaped. |
| `pipe.Reader` and `pipe.Writer` | A `pipe.Reader` or `pipe.Writer` completes, so a peer never waits on an abandoned end. |

### Networking

#### Non-blocking sockets

Sockets are non-blocking. An operation that would block waits with `sys.wait_socket`, so a green thread waiting on the network never holds an OS thread the collector must wait for. Every connection writes through a second descriptor from `sys.write_view`, so one green thread can wait to read while another waits to write.

#### Serving connections

`serve` runs each connection on its own green thread.

#### Socket timeouts

A timeout spawns a green thread that sleeps and shuts the socket down if the operation is still waiting. It enters the guard first, so a socket already closed is left alone. The waiting operation then fails with `TimedOut`.

#### Closing connections and sockets with operations in progress

A connection counts the reads and writes in progress. Closing it shuts it down. That wakes any green thread still waiting while the descriptor stays registered. The last operation to finish closes the descriptors, so a descriptor number is never reused under a waiting operation. Reads and writes after `close` fail with `NotConnected`, and closing twice does nothing.

A UDP receive that waits re-enters the socket's guard after each wake, so closing the socket from another green thread fails the waiting receive with `NotConnected`.

#### Resolver lookup order

A resolver query tries a host in this order:

1. IP literal.
2. Hosts file.
3. `localhost`.
4. DNS over each name of the search list.

#### Resolver DNS attempts

Every attempt opens a new UDP socket, so the kernel picks the source port. Every attempt writes a query with a random id and an EDNS(0) payload of 1232. Only a reply from the server's address and port with that id, the query's question and QR set counts. Anything else is ignored until the attempt's timeout.

Servers are tried in order (rotated with `rotate`) for `attempts` rounds. A and AAAA are asked for each candidate name, and the search continues to the next candidate only on NODATA.

| Case | Handling |
| --- | --- |
| A truncated reply | Repeats the query over TCP. |
| An NXDOMAIN reply that is authoritative or has recursion available | Ends the search. |
| An NXDOMAIN reply that is neither of those | Moves to the next server. |
| A `FORMERR` reply to an EDNS query | Is repeated without EDNS. |
| `use-vc` | Sends every query over TCP. |
| CNAME chains | Are followed within a reply and across queries, up to eight links. A loop fails `Io(Malformed)`. |

#### Resolver cancellation

A resolver with a token registers a callback that closes the socket or connection of the attempt in progress, and the lookup then fails with `Io(Aborted)`.

#### Cooperative cancellation

Cancellation is cooperative. A token runs its callbacks and closes its channel once. Code therefore stops blocking work by closing what it waits on:

| Call | Effect |
| --- | --- |
| `net.close` | Wakes a waiting read or write. |
| `process.kill` | Ends a child. |
| `select` | Takes `<-cancel.done(t)`. |

#### Request tokens in the HTTP server

| Request | Handling |
| --- | --- |
| An HTTP/1.1 handler without an `Upgrade` header | Runs on the connection's green thread while one spawned watcher reads ahead into the request reader. End of input or a read error cancels the request's token, and the response is still written. Requests read ahead are served in order afterwards. |
| An HTTP/2 or HTTP/3 stream | The stream's token is cancelled when the client resets it or the connection ends. |
| Every request | The request's token is cancelled once its handler returns. |

#### Client request tokens and deadlines

A client request with a token arms its deadline even without a timeout. Cancelling the token fires the deadline. The deadline shuts down the connection the request is using and wakes the request from waiting on a pooled connection or an HTTP/2 stream.

One reaper green thread per client enforces whole-request deadlines by shutting down an expired HTTP/1.1 exchange. HTTP/2 and HTTP/3 waits select on the deadline and reset the stream.

#### HTTP/1.1 and HTTP/2 on one server port

A server speaks HTTP/1.1 and HTTP/2 on the same port.

| Connection | Served as |
| --- | --- |
| A connection that opens with the HTTP/2 preface | HTTP/2. |
| Any other connection | HTTP/1.1 with keep-alive, pipelining, chunked bodies and `Expect: 100-continue`. |

Over TLS, ALPN offers `h2` and `http/1.1` and the same detection applies. The `h2c` upgrade from HTTP/1.1, deprecated by RFC 9113, is not offered.

#### HTTP/2 coverage

HTTP/2 covers:

- framing
- HPACK with its dynamic table and Huffman decoding (`std/http/huffman`)
- stream multiplexing
- flow control in both directions
- SETTINGS, PING, RST_STREAM, GOAWAY and CONTINUATION
- the request validation of RFC 9113 section 8

#### HTTP/2 threads, encoder and h2spec

A reader green thread feeds frames to the connection, each request's handler runs on its own green thread, and the connection selects over frames and finished responses, so every write happens on one green thread.

The encoder uses the static table and literals without Huffman coding. It passes h2spec except the test sending a garbage preface, which the shared port reads as an HTTP/1.1 request.

#### Response trailers

A handler's `Response.trailers` are sent as a final HEADERS frame over HTTP/2; HTTP/1.1 responses carry none. `Response.trailers` holds the trailer fields of chunked HTTP/1.1 bodies and of HTTP/2 and HTTP/3 responses.

#### HTTP/3 server

HTTP/3 (RFC 9114) is served by `serve_quic` and `serve_quic_on` over a `quic.Listener` whose ALPN is `h3`. A connection that negotiated another protocol is closed with `H3_GENERAL_PROTOCOL_ERROR`.

##### HTTP/3 server control and QPACK streams

Each connection opens a control stream that carries SETTINGS and a QPACK decoder stream, and the encoder is static-only so no encoder stream is opened.

| SETTINGS setting | Value |
| --- | --- |
| QPACK table capacity | 4096 |
| Blocked streams | 16 |
| Field section size | 65536 |

A green thread per peer unidirectional stream reads the control, QPACK encoder and QPACK decoder streams, at most 16 at a time.

| Peer stream or frame | Result |
| --- | --- |
| A duplicate or a push stream | `H3_STREAM_CREATION_ERROR` |
| A closed critical stream | `H3_CLOSED_CRITICAL_STREAM` |
| A first control frame that is not SETTINGS | `H3_MISSING_SETTINGS` |
| An unknown stream type | Is stopped and ignored. |

##### HTTP/3 server request streams

A green thread per request stream reads HEADERS, DATA and an optional trailing HEADERS, runs the handler and writes the response. A connection runs at most the listener's `max_streams` of them at a time. It does not accept the next request stream until one has finished, so a client that stops its streams cannot push the handler count past that.

##### HTTP/3 server QPACK decoder

One `qpack.Decoder` per connection sits behind a mutex. A section that needs table entries the encoder stream has not delivered waits for them, bounded by the life of the connection.

Every `decode`, `cancel_stream` and `on_encoder_stream` writes the decoder stream bytes it owes, taking and writing them under one lock, so the instructions reach the peer in the order they were produced. A write to the decoder stream that fails because the peer sent STOP_SENDING closes the connection with `H3_CLOSED_CRITICAL_STREAM`.

##### HTTP/3 server frame size limits

Frame sizes are checked before anything is read or allocated:

| Read | Limit |
| --- | --- |
| A HEADERS frame | Over 64 KiB is `H3_EXCESSIVE_LOAD`. |
| A control frame | Over 16 KiB is `H3_EXCESSIVE_LOAD`. |
| Unknown frame types | Are read and discarded in chunks of at most 4 KiB. |
| DATA | Is read in chunks of at most 16 KiB, each chunk no larger than what is left of the frame. |
| A body | Over 64 MiB resets the stream with `H3_EXCESSIVE_LOAD`. |

No buffer is sized from a declared length, since a field section is read into a buffer that grows with the bytes that arrive.

##### HTTP/3 server invalid frames and requests

| Case | Result |
| --- | --- |
| A frame out of order, such as DATA before HEADERS or HTTP/2-only frame types | `H3_FRAME_UNEXPECTED` |
| A request with an uppercase field name, a connection-specific field, a missing or repeated pseudo-header, a pseudo-header in the trailers or a `content-length` that differs from the DATA total | Resets and stops its stream with `H3_MESSAGE_ERROR`. The handler is not called. |

A field section over the size limit is answered with `431`.

##### HTTP/3 server responses

A response is:

1. HEADERS.
2. DATA.
3. An optional trailing HEADERS for `Response.trailers`.
4. FIN.

A streaming `Response.stream` sends one DATA frame per chunk.

##### HTTP/3 server request token

`Request.token` is cancelled:

- when the response is complete
- when a write of the response fails because the client stopped or reset the stream
- when the connection ends

A stop or reset that arrives while the handler runs is not noticed before the response is written. A client that resets its request stream before it is complete has its response reset with `H3_REQUEST_CANCELLED`.

##### HTTP/3 server unsupported features

Server push, extended CONNECT and HTTP datagrams are not supported.

#### Handlers that return status 101

A handler that returns status 101 owns the connection: the server writes nothing more and closes it once the handler returns.

| Function | Effect |
| --- | --- |
| `http.upgrade(req, headers)` | Writes the `101` and returns the connection's reader. |
| `http.upgraded()` | The response returned after a WebSocket session. |

#### WebSocket server

`Request.connection` is the buffered reader of an HTTP/1.1 connection. `websocket.accept` writes the handshake to it and reads frames from it.

A WebSocket server:

- requires masked client frames
- answers pings
- reassembles fragments
- checks UTF-8 in text messages
- limits messages to 16 MB
- echoes a close frame

#### HTTP client builder

| Call | Effect |
| --- | --- |
| `http.client()` | Returns a `Builder[Client]` value whose `with_*` functions return updated copies. |
| `build` | Makes a `*Client` that any number of green threads share. |
| `c.request(method, url)` | Copies the client's settings into a `Builder[Response]`, so headers, authentication, timeouts, redirects, retries, version and decompression can be overridden per request. |

The type parameter only selects whether `build` or `send` applies.

#### HTTP/1.1 client connections

A request takes an idle keep-alive connection from a pool keyed by scheme, host and port, dials a new one while the host is under its limit, or queues for one. A sweeper green thread expires idle connections. A pooled connection that fails before any response byte is retried once on a fresh one.

#### HTTP/2 client connections

Over TLS the client offers `h2` and `http/1.1` by ALPN and keeps one HTTP/2 connection per host.

- A reader green thread decodes frames into per-stream queues.
- Request green threads wait on their stream.
- Flow control credits windows as bodies are consumed.
- SETTINGS, PING, RST_STREAM, GOAWAY and the peer's stream limit are honoured.

`with_prior_knowledge(true)` speaks HTTP/2 to `http://` URLs without negotiation (h2c), keeping one connection per host as over TLS.

#### HTTP/3 client

##### HTTP/3 client transport selection

`with_version(Http3)` sends `https` requests over QUIC and keeps `http` requests, requests through a proxy and requests of a client with a dialer on HTTP/1.1:

| Request | Transport |
| --- | --- |
| An `https` request | QUIC |
| An `http` request | Kept on HTTP/1.1. |
| A request through a proxy | Kept on HTTP/1.1. |
| A request of a client with a dialer | Kept on HTTP/1.1. |

There is no fallback: a QUIC connection that cannot be made fails the request with `Connect`, `Tls` or `Timeout`.

##### HTTP/3 client connection

The host holds one connection, shared by concurrent requests, next to its HTTP/1.1 and HTTP/2 state. The connection is replaced once it has ended or the server sent GOAWAY. `close_idle` and the sweeper close it when idle.

| Setting | Effect |
| --- | --- |
| The TLS options of the client | Apply with ALPN `h3` and the URL host as server name. |
| `with_resolver` | Supplies the addresses, which are tried in turn when one times out or is refused. |
| `with_connect_timeout` | Becomes `quic.Config.handshake_timeout`. |
| `with_interface` | Fails with `Connect(Unsupported)`. |

##### HTTP/3 client handshake

The handshake runs on its own green thread, so a cancelled token or the request's deadline abandons the wait at once. The deadline of the request that dials never becomes the handshake timeout, because the connection outlives that request: a server whose HANDSHAKE_DONE arrives after the deadline still leaves a connection the next request reuses.

##### HTTP/3 client control and QPACK streams

The client shares the connection code of the server: it opens the control stream with SETTINGS and the QPACK decoder stream. It reads the server's control, QPACK encoder and decoder streams under the same limits: at most 16 at a time, with duplicates and unknown critical streams rejected. It decodes field sections through one mutex-guarded `qpack.Decoder` that waits for encoder stream instructions.

| Case | Result |
| --- | --- |
| A push stream | `H3_ID_ERROR` because the client never sends MAX_PUSH_ID. |
| A MAX_PUSH_ID from the server | `H3_FRAME_UNEXPECTED` |

QUIC is configured with no server-initiated bidirectional streams.

##### HTTP/3 client request

A request:

1. Opens a bidirectional stream.
2. Writes HEADERS (`:method`, `:scheme`, `:authority`, `:path`, then the lowercased request headers without connection-specific ones, `host` and a `te` other than `trailers`).
3. Writes DATA frames for the body, streamed from a body reader in 16 KiB frames.
4. Writes FIN.

##### HTTP/3 client response

The response is:

1. Up to 16 interim `1xx` HEADERS that are discarded.
2. A final HEADERS checked by `valid_h3_response`.
3. DATA frames read as the body stream.
4. An optional trailing HEADERS checked by `valid_h3_trailers` that fills `Response.trailers`.

`Response.version` is `Http3`.

##### HTTP/3 client malformed responses

A malformed response is:

- an invalid section
- a `content-length` that differs from the DATA total
- DATA on a bodiless response
- too many interim responses

| Case | Result |
| --- | --- |
| A malformed response | Resets and stops the stream with `H3_MESSAGE_ERROR` and fails as `Protocol(Malformed)`. |
| A frame out of order | Closes the connection with `H3_FRAME_UNEXPECTED`. |
| A section over 64 KiB | Resets the stream with `H3_EXCESSIVE_LOAD`. |

##### HTTP/3 client cancellation and stream setup

One green thread per request watches the deadline and the token. When either fires, it resets and stops that stream with `H3_REQUEST_CANCELLED` and wakes a blocked QPACK wait. The connection stays open.

Opening a stream also races the deadline, because it parks at the peer's stream limit. So does opening the control and QPACK decoder streams right after the handshake, which parks when the server allows fewer than two unidirectional streams. That setup runs on its own green thread under the deadline, the token and the connect timeout (10 seconds when none is set). It closes the connection when one of them fires, so the host's dial flag is always cleared.

##### HTTP/3 client stale requests

These count as stale:

- a stream the server reset before the response head
- a stream the server refused with GOAWAY
- a request on a connection that ended
- a request that finds the connection draining

A replayable request is sent once more on a new connection, as for HTTP/2.

##### HTTP/3 client features above the transport

Decompression, redirects, cookies, retries and the 64 MB body limit sit above the transport and apply as for HTTP/2.

#### Client redirects

| Redirect | Handling |
| --- | --- |
| 301, 302 and 303 | Switch to GET without a body. |
| 307 and 308 | Keep the method and body. |
| Relative locations | Are resolved. |
| Across origins | `Authorization` and `Cookie` are dropped. |

#### Response decoding and cookies

Responses are decoded from gzip, deflate (zlib or raw), Brotli and Zstandard as they stream, and chunked bodies are read and sent. An optional RFC 6265 cookie jar is shared by the client.

#### Client retries

Retries apply to:

- connection failures
- for idempotent methods, transport errors and 429, 502, 503 and 504

Retries apply through `std/retry` with exponential backoff, where `Retry-After` raises the planned wait.

#### TCP_NODELAY and body limit

- Servers set `TCP_NODELAY` so separate HEADERS and DATA writes are not held back by delayed acknowledgements.
- Request and response bodies are limited to 64 MB.

#### HTTP/2 duplex server and call API

The duplex server and the client's call API (`std/http/call.low`) are the HTTP/2 surface `std/grpc` stands on.

##### Duplex server handlers

A duplex handler runs when the request headers arrive and reads the body through `Request.upload`.

- `receive_body` returns the next chunk, and `none` at the end of the request or once the request token is cancelled.
- The window of received data is returned to the peer as chunks are taken, so a handler that stops reading stalls the sender.
- `Response.finish` supplies the trailers after the last chunk of `stream`.
- A reset from the peer or a closed connection cancels `Request.token`.

##### Duplex connection drain

Closing the drain channel of a duplex connection:

1. Sends `GOAWAY`, refuses later streams and lets the streams in progress finish.
2. Reads and discards frames until the peer closes it or one second passes.
3. Only then closes the connection.

The peer therefore reads the `GOAWAY` before the close. A socket on Windows closed with unread input resets the connection, and the peer loses the data it has not read.

##### Client call API functions

| Function | Effect |
| --- | --- |
| `open_call` | Returns a `*Call` once the request headers are sent. |
| `call_send` | Writes one DATA frame and waits for the peer's window. |
| `call_receive` | Returns `Head`, `Data`, `Trailers`, `Reset(code)`, `Lost` or `End`, and it returns window as events are taken. |
| `call_close(call, complete)` | Releases the stream, resetting it with `CANCEL` when it is not complete. |

##### Client call API cancellation

Cancelling the token of `open_call` resets the stream with `CANCEL` at once. From then on `call_send` and `call_receive` fail with `TimedOut`, without sending data or returning events that arrived before.

### gRPC

#### gRPC transports and messages

`std/grpc` speaks the gRPC protocol over:

| Transport | Targets |
| --- | --- |
| HTTP/2 prior knowledge | `host:port` and `http://` targets |
| TLS with ALPN `h2` | `https://` targets |

Messages are `std/protobuf` structs; `*_bytes` functions carry raw message bytes.

#### gRPC errors

Every function that can fail returns `grpc.Error`, an enum with the one variant `Failed(status)`. `status_of(e)` returns its `Status`, and `error(code, message)` builds one.

#### gRPC status codes

| Name | What it is |
| --- | --- |
| `Code` | The 17 gRPC codes with their numbers. |
| `code_name` | Gives the upper-case name. |
| `Status` | Holds the code, the message, the `grpc-status-details-bin` bytes and the trailers that came with it. |
| `encode_message` and `decode_message` | `grpc-message` is percent-encoded on the wire by them, and a message that does not decode is kept as received. |

##### gRPC status from an HTTP status

`code_of_http` maps an HTTP status without gRPC trailers:

| HTTP status | Code |
| --- | --- |
| 400 | `Internal` |
| 401 | `Unauthenticated` |
| 403 | `PermissionDenied` |
| 404 | `Unimplemented` |
| 429, 502, 503 and 504 | `Unavailable` |
| Any other | `Unknown` |

##### gRPC status from an RST_STREAM code

`status_of_reset` maps an `RST_STREAM` code:

| RST_STREAM code | Code |
| --- | --- |
| `REFUSED_STREAM` | `Unavailable` |
| `CANCEL` | `Cancelled` |
| `ENHANCE_YOUR_CALM` | `ResourceExhausted` |
| `INADEQUATE_SECURITY` | `PermissionDenied` |
| Every other code | `Internal` |

#### gRPC message frames

A frame is a compressed-flag byte, a four-byte big-endian length and the message. `MessageReader` reassembles frames from arbitrary chunks.

#### gRPC timeouts

`grpc-timeout` is at most eight digits with one of `H M S m u n`. `encode_timeout` picks the finest unit that fits in eight digits, rounding up. `parse_timeout` rejects anything else.

#### gRPC compression and message size

Compression is `Identity`, `Gzip` or `Deflate`, negotiated by `grpc-encoding` and `grpc-accept-encoding`.

| Case | Result |
| --- | --- |
| A request in an encoding the server does not have | Fails with `Unimplemented` and the server's accepted list in `grpc-accept-encoding`. |
| A message longer than `max_receive` after decompression (4 MiB by default) | Fails with `ResourceExhausted`. |
| A message longer than `max_send` before it is sent | Fails with `ResourceExhausted`. |

#### gRPC metadata

`Metadata` is an ordered multi-map of lower-case keys.

| Function | Values |
| --- | --- |
| `add` and `set` | Take ASCII values. |
| `add_bin` and the `-bin` keys | Carry bytes, base64 on the wire. |

These fail with a `MetadataError`:

- keys outside lower-case letters, digits, `-`, `_` and `.`
- keys that start with `grpc-` or `:` or are `content-type`, `te`, `content-length`, `host`, `connection`, `keep-alive`, `transfer-encoding`, `upgrade` or `proxy-connection`
- an ASCII value outside printable ASCII
- `add` with a `-bin` key
- `add_bin` with another key

Both sides send metadata as headers first and trailers last.

#### gRPC channels

A `Channel` made by `dial`:

- connects on the first call
- shares one HTTP/2 connection per server between all calls and green threads
- queues calls beyond the server's stream limit
- reconnects when the connection ends

##### gRPC call functions and streams

| Function | Effect |
| --- | --- |
| `unary` and `unary_bytes` | Return a `Reply`. |
| `server_streaming`, `client_streaming` and `bidi_streaming` | Return a `Stream`. |
| `receive` | Reads the `Stream`. |
| `send` and `close_send` | Write the `Stream`. `send` blocks while the stream's window is closed. |
| `finish` | Finishes the `Stream`. |
| `close_stream` | Releases the `Stream`. |

##### gRPC call deadlines and cancellation

| Option | Effect |
| --- | --- |
| `CallOptions.timeout` | The whole call's deadline sent as `grpc-timeout`. A call past it fails with `DeadlineExceeded` and resets the stream with `CANCEL`. |
| `CallOptions.token` and `cancel_stream` | Cancel the call with `Cancelled` and reset the stream at once. A `receive` after the cancel fails with `Cancelled` without returning messages that arrived before, unless the server had already ended the call. |

##### gRPC call retries

A unary call that fails with `Unavailable` before any response header arrived is retried under `ChannelOptions.retry`, and `retry: false` turns that off. No other failure is retried.

#### gRPC server methods

A `Server` is a table from `/package.Service/Method` to a handler.

| Function | Registers |
| --- | --- |
| `handle_unary`, `handle_server_streaming`, `handle_client_streaming` and `handle_bidi_streaming` | One with message types. |
| `handle_bytes` | One over raw bytes. |

Methods are registered before serving starts.

##### gRPC handler context

A handler receives a `*Context` with the method, the request metadata, `peer`, `token`, `response_headers` and `response_trailers`.

| Function | Effect |
| --- | --- |
| `read` and `write` | Used to read and write. |
| `flush_headers` | Sends the headers early. |
| `remaining(ctx)` | What is left of the client's deadline. |
| `outgoing(ctx)` | The `CallOptions` for a call made on behalf of the handler, with that deadline and `ctx.token`. |

A cancelled or expired call therefore stops its downstream calls.

##### gRPC server call errors

| Case | Result |
| --- | --- |
| A response that fails before any message | Is a Trailers-Only response. |
| A handler that returns an error | Ends the call with its status. |
| `Cancelled` from a call whose deadline passed | Becomes `DeadlineExceeded`. |
| A method that is not registered | Fails with `Unimplemented`. |
| An HTTP/1 request | Fails with `505`. |

##### gRPC serving and shutdown

`serve`, `serve_on`, `serve_tls` and `serve_tls_on` accept connections until `shutdown(s, grace)` closes the listeners, sends `GOAWAY` and waits for the calls to end. After `grace` milliseconds the remaining calls are cancelled with `Unavailable`.

#### gRPC health service

| Function | Effect |
| --- | --- |
| `health()` | The state of `grpc.health.v1.Health`. |
| `register_health` | Serves it. |
| `set_health` and `set_all_not_serving` | Change it. |
| `check_health` and `watch_health` | The client calls. |

| Call | Result |
| --- | --- |
| `Check` of a service never set | Fails with `NotFound`. |
| `Watch` | Sends the current state, `ServiceUnknown` for a service never set, and each change after it. |

#### gRPC features not supported

There is no server reflection, no interceptor chain, no load balancing across addresses, no service config, no hedging, and no conversion of a handler panic into `Internal`.

### Processes

This section covers `process.start`, `perf.held`, the parent's pipes, `wait` and `kill`, `signal.notify` and `watch.next`.

#### Starting a process

`process.start` builds the path, arguments, environment and pipes before `fork`. The child therefore runs only straight-line syscalls: `setpgid` for a new group, `dup2` onto descriptors 0 to 2, `chdir` and `execve`.

Every descriptor the standard library opens is close-on-exec, so the child inherits only its three streams. A child's pipes are streams over guarded descriptors, so closing one twice or using it after it closed fails with `NotConnected` and never reaches a reused descriptor.

A failed `chdir` or `execve` writes its errno to a close-on-exec pipe, which the parent reads before returning. The read sees end of file once `execve` succeeds; otherwise the parent reaps the child and fails with that errno.

#### Process pipes, waiting and killing

The parent's pipe ends are non-blocking and wait through the scheduler, as sockets do. The child's ends stay blocking. On Windows the parent's ends are overlapped pipes, as `## Compiler` describes.

| Function | Behaviour |
| --- | --- |
| `wait` | Waits for the process's pidfd to become readable, then reaps it with `waitid` and remembers the result. |
| `kill` | Signals through the pidfd, or the whole group when the command asked for one. |

#### Held perf sampling

`perf.held` runs the program as `/bin/sh -c 'kill -STOP $$; exec "$0" "$@"'`. The process is therefore single-threaded when it stops and keeps its process id through the `exec`.

Events are opened disabled with `enable_on_exec` and `inherit`. This starts them at the program's first instruction and extends them to every thread and process it starts.

A `Sampler` opens one event for each processor in `/sys/devices/system/cpu/online`. It does so because the kernel gives an inherited event a buffer only when it is bound to a processor. It maps 128 pages for each, halving down to 8 when the locked memory allowance refuses.

#### Perf samples

A sample carries the user call chain the kernel reads through the frame pointers. A routine that keeps no frame, such as `rt_memmove`, therefore appears under its caller's caller.

With a fixed period the sample records the processor in place of the period. This is because the kernel samples every software event when both a period and the period field are asked for. `drain` gives each such sample the period as its weight.

#### Perf usage and CPU affinity

| Function | Behaviour |
| --- | --- |
| `usage` | Is `waitid` with `WNOWAIT` and `WNOHANG`, whose `rusage` covers the child and the descendants it waited for and leaves the child for `process.wait` to reap. |
| `restrict_cpus` | Calls `sched_setaffinity` on every thread in `/proc/self/task`, twice. A thread started during the first pass is therefore covered. |

#### Signal handlers

`signal.notify` installs a handler with `rt_sigaction`, or on Windows registers a console control handler with `SetConsoleCtrlHandler`. The handler is a short block of machine code in a mapped page. It writes the signal number to a non-blocking pipe and returns through `rt_sigreturn`.

The Windows handler writes through an overlapped `WriteFile`. `handler_code` writes the address of that `WriteFile` into the code.

#### Windows console control events

The Windows handler maps console events to signals:

| Console event | Signal |
| --- | --- |
| Ctrl+C | `Interrupt` |
| Ctrl+Break | `Quit` |
| A closed console and logoff | `Hangup` |
| Shutdown | `Terminate` |

The Windows handler runs on a thread the system starts. It returns `1` only for the events of the signals some `notify` asked for and `0` for the rest. An event no watch asked for therefore reaches the default handler and ends the process, as Ctrl+Break does in a program that watches only `Interrupt`.

#### Signal forwarding

A `Watch` forwards only the signals it asked for. On Windows `notify` of any other signal fails with `Unsupported`.

The runtime gives every OS thread an alternate signal stack, so the handler runs on whichever thread receives the signal. A forwarding green thread reads the pipe with `wait_readable` and sends to the `signals` channel, which holds 16 and drops repeats while full.

#### Replacing and dropping a signal watch

Another `notify` for the same signal replaces the handler. Dropping a `Watch` restores the saved dispositions and unmaps the page, except that a handler another `notify` has replaced is silenced and its page stays mapped.

#### Raising and sending signals

| Function | Behaviour |
| --- | --- |
| `raise` | Signals the calling thread, so its handler has run when `raise` returns. |
| `send` | Is process-directed and unordered against other threads. |

#### Inotify watching

`watch.next` reads every event an inotify read returns and queues them. With `recursive` set, new and moved-in directories are watched before the batch that reports them is returned.

`stop` wakes a parked `next` by adding and removing a watch on `/`, which makes the descriptor readable. The kernel merges identical consecutive events it has not yet delivered.

### Resilience

This section covers `retry.delay`, `Bucket` and `Group.do`.

#### Retry backoff

| Function | Behaviour |
| --- | --- |
| `retry.delay` | Is the capped exponential backoff. |
| `jittered` | Applies the chosen jitter to it. |
| `reserve` | Plans the wait and stops at the attempt limit, the deadline or a cancelled token. |
| `commit` | Calls the observer, waits through the clock's `sleep`, which a cancelled token ends early, and starts the next attempt. |
| `next` | Is `reserve` then `commit`. |

#### Token bucket

A `Bucket` keeps its balance as an integer in units of one token over `per` nanoseconds. Refills are therefore exact, and a wait is the ceiling of the deficit over the rate.

| Function | Behaviour |
| --- | --- |
| `reserve` | Consumes and returns the wait. |
| `wait` | Sleeps that long and refunds if cancelled. |

#### Call group

`Group.do` runs the function outside its lock, removes the key and stores the result before waking the waiters. It matches the key by call identity. A `forget` followed by a new call is therefore not disturbed.

### Numerics

This section covers `std/math/stats`, `std/math/complex`, `std/math/matrix` and `std/math/fft`.

#### Numerics modules

`std/math/stats`, `std/math/complex`, `std/math/matrix` and `std/math/fft` are written in Low and use no operating system service. Only `complex` and `fft` reach libm, through `std/math`, for `hypot`, `atan2`, `log`, `cos` and `sin`.

A program that uses only `stats` or `matrix` therefore stays static. All four build for `browser/wasm32`.

#### Statistics

`stats` rejects a `nan` or infinite input, and a `nan` or infinite result such as an overflowing sum, with `NotFinite`. A statistic is therefore never a silent `nan`.

| Function | Behaviour |
| --- | --- |
| `variance` and `covariance` | Sum deviations from the mean with compensation and subtract the squared sum of the deviations. A large common offset therefore does not cost digits. |
| `quantile` | Sorts a copy and takes position `q * (n - 1)`. |
| `quantiles` | Sorts once for several. |
| `histogram` | Takes its range from the data, widened by 0.5 on each side when every value is equal. |
| `histogram_edges` | Ignores values outside the edges and finds the bin by binary search. |

#### Complex numbers

| Function | Behaviour |
| --- | --- |
| `complex.div` | Scales by the larger part of the divisor. A zero divisor divides each part of the dividend by zero. |
| `csqrt` | Returns the root with a non-negative real part, and the sign of the imaginary part of a negative real argument follows the sign bit of the imaginary part, so the branch cut agrees with C99 `csqrt`. |

#### Matrix errors and singular matrices

`matrix` panics with a `matrix:` message when shapes do not fit, as `std/slices` panics on misuse. `fft.inverse_real` does the same with an `fft:` message.

| Function | Behaviour |
| --- | --- |
| `lu` | Fails with `Singular` only when the best pivot is exactly zero. This is the criterion of LAPACK. A matrix that is singular up to rounding therefore factorises. |
| `determinant` | Of a singular matrix is `0`. |
| `least_squares` | Fails with `Singular` when a diagonal entry of `R` is at most `max(rows, cols) * epsilon` times the largest one. This is because rank deficiency from rounding shows as a small entry, not a zero. |

#### QR factorisation

`qr` takes `alpha = -sign(a[k][k]) * norm` for each reflection and leaves a column that is already zero below the diagonal alone. This gives the signs of LAPACK and `numpy.linalg.qr`.

#### FFT algorithms

`fft` computes twiddle factors with `cos` and `sin`, not by recurrence.

Bluestein's algorithm reduces `k * k` modulo `2 * n` before forming the chirp angle. It convolves with a power-of-two transform of length at least `2 * n - 1`.

`inverse` is the conjugate of the forward transform of the conjugate, divided by `n`.

#### Numerics fixtures

The fixtures `testdata/run/std_math_stats`, `std_math_complex`, `std_math_matrix` and `std_math_fft` print every value rounded to six digits and normalise `-0.000000`, so the last bits of a libm do not change the output. Their `.out` files come from `numpy`, `cmath`, `math.fsum` and exact `fractions` arithmetic.

### Text

This section covers `diff.unified`, `number.format_bytes`, `parse_bytes`, `si` and `naming.words`.

#### Unified diffs

`diff.unified` merges hunks whose gap is at most twice the context, omits a count of 1 in a hunk header and writes `\ No newline at end of file`, as GNU `diff -u` does. The edit script is minimal.

#### Byte sizes

| Function | Behaviour |
| --- | --- |
| `number.format_bytes` | Shows one decimal below 10 and rounds half to even on exact integers. |
| `parse_bytes` | Reads `k` and `kb` as 1000 and `ki` and `kib` as 1024. It fails with `OutOfRange` on overflow or `TooPrecise` beyond 18 fraction digits. |
| `si` | Rounds on the exact decimal expansion of the `f64`. |

#### Splitting names into words

`naming.words` splits at case, acronym and digit boundaries and keeps combining marks attached to their letter, so `HTTPServerID2` is `http server id 2`.

### Time

This section covers the time types, calendar and elapsed arithmetic, zones, parsing, the supported range and the names of months and weekdays.

#### Time types

Each concept is its own type:

| Type | Is |
| --- | --- |
| `Timestamp` | A point on the UTC timeline. |
| `Date`, `TimeOfDay` and `DateTime` | Civil values with no zone. |
| `Zoned` | A timestamp seen through a zone. |
| `Duration` | Exact elapsed time. |
| `Span` | A calendar amount. |

Types never convert silently.

#### Time validation

Fields are private and constructors validate, so a `Date` cannot hold February 30. Months are 1 to 12, and `Month` and `Weekday` are enums.

Nothing rolls over: `date.of(2026, 2, 30)` fails with `Invalid`, and the clamping forms are named (`clamped`, `plus_months`).

#### Calendar and elapsed arithmetic

Calendar arithmetic and elapsed arithmetic are separate.

| Type | Moves |
| --- | --- |
| `Duration` | A `Timestamp` or `Zoned` by elapsed time. |
| `Span` | A `Date`, `DateTime` or `Zoned` by calendar units. |

A month after January 31 is the last day of February. A day after a spring-forward midnight is the next civil day, although it is 23 hours long.

`until` and `since` take a `Diff`, add whole years, then months, to the start without passing the end, then count the rest.

#### Time zones and disambiguation

Only `zone.local` reads the host zone. A civil time in a daylight saving gap or overlap needs a `Disambiguation`.

| Disambiguation | In an overlap | In a gap |
| --- | --- | --- |
| `Compatible` | Picks the first occurrence. | Moves forward by the gap. |
| `Earlier` | Picks the first occurrence. | Moves back. |
| `Later` | Picks the second occurrence. | Moves forward by the gap. |

`Reject` fails with `Skipped` or `Ambiguous`. `zoned.start_of_day` is the first instant of the civil day, correct where midnight does not exist.

#### TZif zone data

Zones come from TZif versions 1 to 4, using the 64-bit body from version 2.

| Time | Applies |
| --- | --- |
| After the last transition | The footer's POSIX TZ rule. |
| Before the first transition | The first standard-time type. |

The rules cover `Jn`, zero-based `n` and `Mm.w.d` days and transition times from -167 to 167 hours. Negative daylight saving (Dublin, Casablanca), half-hour shifts (Lord Howe) and skipped days (Apia) therefore work.

#### Time parsing

Parsing is strict ISO 8601, RFC 3339 and RFC 9557:

- `T`, `t` or a space separates date and time.
- A fraction has 1 to 9 digits.
- `:60` reads as `:59`.
- A civil type rejects an offset.

A written offset that disagrees with a bracketed zone is `OffsetConflict` unless an `OffsetPolicy` says which wins. Every parse error is `Syntax(at)` with the byte offset of the first bad character, or a value error naming the field.

#### Time range and overflow

| Type | Supported range |
| --- | --- |
| `Timestamp` | 0001-01-01 to 9999-12-31 |
| `Date` | Years -9999 to 9999 |

Leap seconds are not modelled. Plain arithmetic panics on overflow. The `try_` forms return an optional.

#### Time names and locales

Month and weekday names are English. `pattern.Names` and `humanize.Words` carry other languages. `datefmt.names`, `pattern.compile_in` and `humanize.ago_in` fill them from CLDR for a `locale.Locale`.

### Browser automation

This section covers `cdp.launch`, the commands and messages of the connection, `open_page` and cancellation.

#### Launching a browser

`cdp.launch` finds `chromium`, `chromium-browser`, `google-chrome-stable` or `google-chrome` on `PATH` unless given an executable. It creates a profile with `file.temp_dir`. It starts the browser headless in its own process group with `--remote-debugging-port=0`. Standard input and output are discarded and standard error is piped.

A green thread reads standard error until the `DevTools listening on` line, which gives the WebSocket address. That thread keeps draining standard error so the browser never blocks on a full pipe.

#### Browser launch failures

Another green thread waits with `process.wait_exit_on` for the browser to end, so a browser that dies before the line fails the launch at once, as `Launch(Aborted)`, even while a helper it started, such as the crash reporter, keeps standard error open.

Standard error closing with the browser still running gives `Launch(Eof)`. The startup deadline gives `Launch(TimedOut)`.

#### Browser shutdown

`shutdown` closes the WebSocket, kills the process group, waits for the thread that waits for the browser to end, reaps the browser and removes the profile.

#### Browser commands and messages

Commands carry an id and, for a session, its `sessionId`. A pump green thread routes replies to the waiting call. It sends every other message to each subscription. Sends are serialised.

| Condition | Result |
| --- | --- |
| A message that is not JSON | Fails every waiting call with `Protocol`. |
| A call waits | At most 30 seconds. |
| The socket closes | Every waiting call fails with `Closed` and the subscription channels close. |

#### Browser page contexts and targets

`open_page` creates a browser context that is disposed on detach. It denies downloads in that context. It turns on auto-attach with `waitForDebuggerOnStart` and flat sessions, so every page, frame and worker target pauses on creation.

Each page subscribes to the events. It acts only on events of its own context and the sessions attached in it. The first page in the context is the opened page. Any other page in the context, such as a popup, is closed before it runs.

Every other target in the context gets `Fetch.enable` for all URLs, `Network.enable`, auto-attach for its own children and, for pages, `Network.setBypassServiceWorker`, and only then resumes, so no request escapes interception. A target that cannot be prepared is closed. If it is the opened page, `open_page` fails.

`close_page` ends the subscription and disposes the context.

#### Browser request interception

Each paused request is passed to `Handlers.request` on its own green thread. The request is continued or failed with the verdict. Responses and failed loads are reported with their URL.

#### Browser cancellation

Cancellation is the caller's: `let h = b.handle` and `cancel.on_cancel(t, fn() { cdp.shutdown_on(h) })` end everything the browser was doing.

### TLS

This section covers TLS 1.3 (RFC 8446) and TLS 1.2 (RFC 5246), client and server: versions, suites, key exchange, signatures, certificate status, QUIC mode and server credentials.

#### TLS versions

TLS 1.3 (RFC 8446) and TLS 1.2 (RFC 5246), client and server. `Config.min_version` and `Config.max_version` bound the versions. They are written with `tls12` and `tls13`. They default to 1.2 through 1.3.

A client offers its range through `supported_versions`. A server answers with the highest version both support. `Session.handle.version` holds the result.

#### TLS downgrade protection

| Peer | Behaviour |
| --- | --- |
| A server that supports 1.3 | Ends a 1.2 ServerHello random with the RFC 8446 downgrade sentinel. |
| A client that offered 1.3 | Aborts with `illegal_parameter` when it sees either sentinel. |
| A server | Refuses `TLS_FALLBACK_SCSV` below its highest version with `inappropriate_fallback`. |

#### TLS cipher suite selection

| Name | Meaning |
| --- | --- |
| `Config.cipher_suites` | Lists suites in preference order. |
| `Session.handle.cipher_suite` | The IANA code. |
| `Session.handle.suite` | The AEAD and hash. |

A server takes the first of its own list that the client offers.

#### TLS suites by version

| Version | Suites |
| --- | --- |
| TLS 1.3 | TLS_AES_128_GCM_SHA256, TLS_CHACHA20_POLY1305_SHA256 and TLS_AES_256_GCM_SHA384. |
| TLS 1.2 | ECDHE with ECDSA or RSA authentication and AES-128-GCM with SHA-256, ChaCha20-Poly1305 with SHA-256 or AES-256-GCM with SHA-384, AEAD only. |

| Record | Nonce |
| --- | --- |
| GCM records | Carry the 64-bit sequence number as their explicit nonce. |
| ChaCha20-Poly1305 records | Use the RFC 7905 nonce. |

#### TLS 1.3 key exchange

TLS 1.3 key exchange offers the hybrid post-quantum group X25519MLKEM768 first, then X25519, P-256 and P-384. A client sends key shares for X25519MLKEM768 and X25519.

A server picks the first group of its own order that the client supports. It answers with a HelloRetryRequest when the client sent no share for that group. So a client that supports X25519MLKEM768 always gets it.

#### TLS 1.2 key exchange

TLS 1.2 uses the first of X25519, P-256 and P-384 in the server's order that the client lists. In TLS 1.2 a server with an ECDSA certificate also requires the client to list the certificate's curve (RFC 8422).

#### TLS server signatures

Servers sign with ECDSA P-256 or P-384, RSA-PSS, RSA PKCS#1 v1.5 in TLS 1.2, or Ed25519. Keys are loaded from PEM (`x509.pem_certificates`, `x509.pem_private_key`).

#### TLS signature scheme codes

`std/crypto/x509` owns the one table of signature scheme codes. The client offer, the codes a key signs with and the key exchange authentication of a code all come from it.

TLS 1.2 ECDSA signature schemes name only the hash, so either curve signs with SHA-256 or SHA-384 (`x509.SignatureRules.Tls12`). In TLS 1.3 a code names its curve.

#### TLS client verification of the server

Clients verify the following:

| Check | Against |
| --- | --- |
| The chain | `Config.roots`, or the system roots when that is empty. |
| The host name | The certificate's subject alternative names. |
| CertificateVerify or ServerKeyExchange | Any offered scheme the certificate allows and, in TLS 1.2, the suite's authentication. |

#### TLS 1.2 handshake requirements

TLS 1.2 requires the extended master secret (RFC 7627) on both sides. A client requires the secure renegotiation indication (RFC 5746). A server echoes it when the client sends it. Both refuse renegotiation with a `no_renegotiation` warning.

#### TLS supported features and client certificates

- ALPN, SNI, KeyUpdate in TLS 1.3 and `close_notify` are supported.
- Session tickets are ignored, so there is no resumption or 0-RTT.
- Servers do not request client certificates. A client answers a CertificateRequest with an empty Certificate.

#### TLS record limits and alerts

Records hold at most 2^14 bytes of plaintext, and a peer's ciphertext may add 256 bytes in TLS 1.3 or 2048 in TLS 1.2.

A record or handshake error sends its fatal alert, such as `bad_record_mac`, `record_overflow`, `unexpected_message` or `decode_error`, and leaves the session unusable.

#### TLS session concurrency

A session's read and write sides are independent, with a write lock, so different green threads can read and write it.

#### TLS in QUIC mode

QUIC mode (`quic_client`, `quic_server`) runs the TLS 1.3 handshake without records.

- Handshake messages go to the transport per level. The traffic secrets are handed over as the schedule derives them.
- It offers TLS 1.3 only and carries the transport parameters in extension 0x39.
- It requires ALPN and an empty `legacy_session_id`.
- It rejects handshake data at the wrong level and bytes left over when a key changes.
- It caps a handshake message at 64 KiB.

#### TLS certificate status requests

A client with `Config.revocation.mode` other than `Off` sends the `status_request` extension (RFC 6066 section 8, `ocsp` with empty responder and request extensions). It checks what the server staples as Certificate status describes. `status_request_v2` is not offered or accepted.

#### TLS stapling by a server

| Server | Behaviour |
| --- | --- |
| A TLS 1.2 server | Answers with an empty `status_request` in its ServerHello and a `CertificateStatus` message after its Certificate. |
| A TLS 1.3 server | Puts the response in the `status_request` extension of the certificate entry it belongs to. |

A server staples only when the client asked. It staples from `Config.staples`: each entry in TLS 1.3, the first entry alone in TLS 1.2.

#### TLS revocation alerts

| Condition | Alert |
| --- | --- |
| A revoked certificate | `certificate_revoked` |
| Any other failure of the chain, which includes a status that cannot be established | `bad_certificate` |

#### TLS server credentials hook

A server with `Config.credentials` calls its `choose` once per connection. It passes the server name and ALPN protocols of the first ClientHello. The server presents the `Credentials` that `choose` returns instead of `Config.certificates`, `Config.key` and `Config.staples`.

| Condition | Result |
| --- | --- |
| A selected `Credentials.protocol` the client did not offer | Fails the handshake after a `no_application_protocol` alert. |
| Credentials without a certificate or a key | Refuse the handshake with `unrecognized_name`. |

`std/acme` uses this hook to serve a certificate that renews and to answer TLS-ALPN-01.

### Certificate status

This section covers revocation checking in `x509.verify_chain`: its modes, status sources, must-staple, times, OCSP requests and responses, CRLs and fetching.

#### Revocation modes

`x509.verify_chain` takes a `Revocation` with a `mode`, a list of stapled `staples` and an optional `fetcher`.

| Mode | Behaviour |
| --- | --- |
| `Off` | The default. Checks nothing and does not require must-staple certificates to be stapled. |
| `SoftFail` and `HardFail` | Check every certificate of the path below the trust anchor against the next certificate of the path. They skip a certificate with the `id-pkix-ocsp-nocheck` extension. The anchor is never checked. |

#### Certificate status sources

A certificate's status comes from the first source that gives an answer: the stapled responses, then, with a `fetcher`, an OCSP request to each of its `ocsp_urls`, then each of its `crl_urls`.

A response answers for the certificate its `CertID` names, whatever its place in the list.

#### Certificate status outcomes

`Revoked` fails the check in every mode, with `Refused`.

A status cannot be established because no source answered, the response is `Unknown`, unusable, unsigned by an acceptable responder or out of date, or the fetch failed. Such a status passes under `SoftFail` and fails with `Invalid` under `HardFail`. An `Unknown` OCSP answer falls through to the next source.

#### Must-staple certificates

A certificate with the TLS feature extension `status_request` (must-staple, RFC 7633) needs a stapled `Good` response whenever the mode is not `Off`. A response from the fetcher does not satisfy it, and without one the check fails with `Invalid` in both modes.

#### Certificate status times

Times are checked against the `now` the caller passes.

| A list or response counts as | Up to |
| --- | --- |
| Issued | `clock_skew` (300 seconds) after `now` |
| Current | `clock_skew` after its `nextUpdate` |

A CRL and a single OCSP response without `nextUpdate` are not accepted.

#### OCSP requests and nonces

`ocsp_request` writes an unsigned request with a SHA-1 or SHA-256 `CertID`. For a nonce of 1 to 32 bytes, it writes the RFC 8954 nonce extension. A fetched request carries a random 16 byte nonce.

| Response | `ocsp_status` |
| --- | --- |
| A response whose nonce differs | Rejects it. |
| A response with no nonce | Accepts it, as responders that serve precomputed responses do. |
| A stapled response | Ignores its nonce. |

#### OCSP response signers

`ocsp_status` accepts a response signed by the issuer of the certificate, named by name or by key hash. It accepts a response signed by a certificate carried in the response that the issuer issued, that is valid at `now`, has no unhandled critical extension and has `id-kp-OCSPSigning`.

It recomputes the `CertID` with the hash the response names (SHA-1, SHA-256, SHA-384 or SHA-512) and verifies the signature with the algorithms of `check_signature`.

#### CRL parsing

`parse_crl` reads v1 and v2 lists as strict DER, including every revoked entry. It keeps the entries encoded: `crl_lookup` scans them for one serial number and builds a value only for the match.

A list that is indirect, limited to some reasons or to attribute certificates, a delta list, or that has another critical extension or critical entry extension, fails to parse with `Unsupported`, since it would be incomplete for the certificates it is used for.

#### CRL verification

`verify_crl` needs:

- the list's issuer name to equal the issuer certificate's subject
- `cRLSign` in its key usage when it has one
- the authority key identifier of the list to equal the subject key identifier of the issuer when both exist
- a valid signature

`crl_status` also needs the list to cover the certificate: an issuing distribution point may be limited to user or CA certificates, or name a distribution point that must appear in the certificate's `crl_urls`.

#### Certificate status fetching

Fetching is a pair of function values, `Fetcher.ocsp` and `Fetcher.crl`, so tests and callers with their own transport substitute them. `std/crypto/x509/fetch` implements them over `std/http`.

Nothing caches a response or a list. A caller that wants a cache wraps the functions.

### ACME

`acme.connect` fetches the directory (an `https` URL) of RFC 8555 and returns a `Client` that any number of green threads may share.

#### ACME signed requests and nonces

Every signed request is a flattened JWS (`jose.sign_flattened`) with `url` and `nonce` in its protected header. The header has `kid` set to the account URL, or an embedded `jwk` for `new_account` and `revoke_with_key`. A request that reads a resource is POST-as-GET with an empty payload.

The client keeps the `Replay-Nonce` values the server sends in a pool. It fetches one from `newNonce` when the pool is empty. A `badNonce` answer makes the client repeat the request with a fresh nonce, up to `Options.nonce_retries` times. `rejected_nonces` counts those answers.

#### ACME account keys

An account key signs with one of these:

| Signs with | Key | Source |
| --- | --- | --- |
| ES256 | P-256 | `x509.generate_private_key` makes it |
| ES384 | P-384 | `x509.generate_private_key` makes it |
| EdDSA | Ed25519 | `x509.generate_private_key` makes it |
| RS256 | RSA | `x509.pem_private_key` loads an RSA key |

#### ACME errors

An error answer is a `Problem`. Its `kind` is the suffix of the `urn:ietf:params:acme:error:` type (`Other` for any other type or a body that is not a problem document). It has `type_uri`, `detail`, `status`, `instance` and the `Subproblem` list of a `compound` answer.

`Error` separates `Http`, `Server`, `Protocol` (an answer the protocol does not allow), `Crypto`, `Rejected` (an authorization became `invalid`), `OrderFailed`, `Timeout`, `Cancelled`, `Unsupported`, `Solver` and `Store`. `describe` writes any of them as one line.

#### ACME account functions

| Function | What it does |
| --- | --- |
| `new_account` | Creates an account or returns the one the server holds for the key. It does so with contacts, terms-of-service agreement and an external account binding (`Binding`, an HMAC SHA-256 key of at least 32 bytes). |
| `find_account` | Never creates one and fails with kind `AccountDoesNotExist`. |
| `change_key` | Signs the rollover with both keys. |
| `deactivate_account` | Ends the account. |

#### ACME orders

`new_order` takes `Identifier`s from `identifier(name)`, which makes an `ip` identifier (RFC 8738) of an IP address in its canonical text and a `dns` identifier of anything else, with an optional `profile` and the `replaces` identifier of RFC 9773.

#### ACME polling and chain download

`wait_authorization` and `wait_order` poll on `Options.clock`. They wait for `Retry-After` (a number of seconds or an HTTP date) or `Options.poll_interval`. They fail with:

| Error | When |
| --- | --- |
| `Timeout` | After `Options.poll_timeout`. |
| `Cancelled` | When `Options.token` is cancelled. |

`download_chain` reads the PEM chain and lists the `alternate` links of the response in `Chain.alternates`.

#### ACME obtain

`obtain` runs a whole order with `Solvers`. For each authorization that is not `valid`, it takes the first of `Solvers.methods` that the server offers and a solver exists for. For that authorization it:

1. Puts the response in place.
2. Answers the challenge.
3. Waits for the authorization.
4. Takes the response away again, also when it fails.

The TXT record of `dns-persist-01` stays. A wildcard name needs `dns-01`, or `dns-persist-01` with `PersistentDns.wildcard`.

#### ACME certificate requests and revocation

| Function | What it does |
| --- | --- |
| `csr_for` | Writes the PKCS#10 request. It uses the first DNS name as its subject when that is at most 64 bytes. It puts every name in the subject alternative name. |
| `revoke` | Signs with the account. |
| `revoke_with_key` | Signs with the certificate's own key. |

#### ACME solvers

| Solver | What it does |
| --- | --- |
| `http01_handler` | Answers `GET` and `HEAD` of `/.well-known/acme-challenge/<token>` from a `Challenges` table with the key authorization (404 for an unknown token). It passes every other request on. |
| `tls_alpn_source` | Presents the certificate `put_certificate` holds for the server name to a ClientHello that offers `acme-tls/1`. It passes every other ClientHello to the source it wraps. |
| `tls_alpn_credentials` | Writes that certificate. It is valid from an hour before `now` for 7 days. It carries the critical `id-pe-acmeIdentifier` extension holding the SHA-256 digest of the key authorization. For an IP identifier the SNI name is the `in-addr.arpa` or `ip6.arpa` name of the address. |
| `DnsRecords` | A pair of functions that publish and remove the TXT record `dns01_name` and `dns01_value` compute. |

The module never contacts a DNS server.

#### ACME DNS-PERSIST-01 record

DNS-PERSIST-01 (draft-ietf-acme-dns-persist) proves a domain with a TXT record at `_validation-persist.<domain>` that stays in place between orders. The record holds an issuer domain name of the CA, `accounturi`, and optionally `policy=wildcard` and `persistUntil=<Unix seconds>`. It uses the issue-value syntax of RFC 8659.

| Part | What it holds |
| --- | --- |
| Issuer | The issuer is `PersistentDns.issuer` or the first of `Challenge.issuers`, which the client reads from `issuerDomainNames` or `issuer-domain-names` and requires to hold 1 to 10 names. |
| `accounturi` | The account URL. For a server whose directory has `accountHashPrefix`, it is the URI of `account_hash_uri`. That URI is the prefix, `sha-256/` and the Base64url SHA-256 digest of one length byte, the lower case domain, the account key's JWK thumbprint and the account URL, which changes when the account key does. |

#### ACME DNS-PERSIST-01 in obtain

`obtain` uses it when `Solvers.persistent` is set and the server offers the challenge for a `dns` identifier. It calls `PersistentDns.publish` with the record name and text. It leaves the record in place afterwards.

A wildcard authorization uses the challenge only with `PersistentDns.wildcard`. A `persist_until` that has passed fails with `Unsupported` before anything is published.

#### ACME DNS-PERSIST-01 parsing

`parse_dns_persist` reads a record as a server does:

- It reads names without regard to case.
- It ignores unknown parameters.
- It refuses a repeated `accounturi`, `policy` or `persistUntil`.

#### ACME renewal identifiers

| Function | What it does |
| --- | --- |
| `renewal_id` | Builds the certificate identifier of RFC 9773 from the authority key identifier and the serial number (`Certificate.authority_key_id`, `Certificate.serial`). |
| `renewal_info` | Returns the suggested `RenewalWindow` with the `Retry-After` of the answer. |

#### ACME manager issuance

`manager` keeps one certificate per entry of `ManagerConfig.names` obtained and renewed.

It reads `account.pem` from the `Store`. It finds the account for that key or, when the server does not know it, creates one with the same key, and writes a new key there.

- A certificate saved as `<names joined with +>.pem` (with `*` and `:` written `_`) is used again when its key matches its leaf and its names cover the entry. Anything else counts as absent.
- Each issuance is retried under `ManagerConfig.backoff` with `std/retry`.
- Only the first attempt of a renewal carries `replaces`.
- A certificate is saved and served only after its leaf is checked against the key and the names.
- A run that gives up leaves the certificate alone for `failure_pause`. `renew_now` clears that pause.

#### ACME manager renewal timing

A renewal is due at `renew_fraction` of the lifetime. When the directory has `renewalInfo` and `use_ari` is set, it is due instead at a random time inside the window the server suggests.

- The window is asked for again after its `Retry-After` or `check_interval`.
- A failed request keeps the earlier window.
- A window that ends before the certificate's validity begins is ignored.

| Name | What it does |
| --- | --- |
| `run_once` | Makes one pass and returns the wait until the next. |
| `start` | Runs passes on a green thread. |
| `stop` | Ends that thread. |
| `ManagerConfig.token` | Ends that thread when it is cancelled. |

Every issuance reports an `Event` (`Obtained` with the reason `initial`, `scheduled`, `suggested` or `requested`, or `Failed`) to `ManagerConfig.observer`.

#### ACME manager certificate source

`source` returns the `tls.CredentialSource` that presents the current certificate of the entry whose name covers the SNI name (exact match without regard to case, or `*.` for one label).

| Case | Result |
| --- | --- |
| An empty name | Gets the first certificate held. |
| An unknown name or nothing held yet | Refuses the handshake. |

It also answers TLS-ALPN-01 when `Solvers.challenges` is set. A renewed certificate reaches connections that start after the swap.

#### ACME stores

| Store | What it does |
| --- | --- |
| `memory_store` | Keeps the bytes in a map. |
| `directory_store` | Creates its directory with mode `0o700`. It writes each file to a temporary file with mode `0o600` in that directory, flushes it and moves it onto the name. It refuses a name that is empty, starts with `.` or contains a path separator with `Invalid`. |

#### ACME unsupported features

Not supported:

- Pre-authorization.
- Listing the orders of an account.
- Generating RSA keys.
- For DNS-PERSIST-01, an IP identifier.
- For DNS-PERSIST-01, a record at an ancestor of the domain (`ancestorDomain`).

### SSH

The transport follows RFC 4253 with the binary packet protocol, strict key exchange (`kex-strict-c-v00@openssh.com`, `kex-strict-s-v00@openssh.com`) and `ext-info` (RFC 8308).

#### SSH algorithms

| Kind | Algorithms |
| --- | --- |
| Key exchanges | `mlkem768x25519-sha256`, `curve25519-sha256` (and its `@libssh.org` name), `ecdh-sha2-nistp256`, `ecdh-sha2-nistp384`, `diffie-hellman-group16-sha512` and `diffie-hellman-group14-sha256`. |
| Host keys | `ssh-ed25519`, `ecdsa-sha2-nistp256`, `ecdsa-sha2-nistp384`, `rsa-sha2-256` and `rsa-sha2-512`. `ssh-rsa` with SHA-1 is not offered. |
| Ciphers | `chacha20-poly1305@openssh.com`, `aes256-gcm@openssh.com`, `aes128-gcm@openssh.com`, `aes256-ctr` and `aes128-ctr`. |
| MACs | The CTR ciphers use `hmac-sha2-512-etm@openssh.com` or `hmac-sha2-256-etm@openssh.com`. |
| Compression | `none`. |

`Algorithms` restricts each list. `default_kex`, `default_host_keys`, `default_ciphers` and `default_macs` return the defaults in preference order.

#### SSH packet limits and strict key exchange

- A packet carries at most 262144 bytes, and a larger length fails with `TooLarge` before anything is read.
- MACs and tags are compared in constant time.
- Sequence numbers run for the life of the connection.
- Under strict key exchange a message other than key exchange messages before `NEWKEYS`, or a first packet that is not `KEXINIT`, fails with `StrictKex`.
- Under strict key exchange, sequence numbers reset at each `NEWKEYS`.

#### SSH rekeying

A key exchange starts after `rekey_bytes` or `rekey_seconds` under one set of keys, or when the peer starts one.

| Option | Default |
| --- | --- |
| `rekey_bytes` | 1 GiB |
| `rekey_seconds` | One hour |

`key_exchanges` counts them. The `diffie-hellman` groups use `rsa.mod_pow`, so the private exponent never selects a branch or a table index.

#### SSH host key verification

A client verifies the server's host key through `verify_host_key`, which `accept_any_host_key`, `key.known_hosts_verifier` and `key.trust_on_first_use` make. The signature over the exchange hash is checked after it. A server signs with the host key whose algorithm the client offers.

#### SSH user authentication

User authentication (RFC 4252, RFC 4256) tries the `AuthMethod`s in order: `NoAuth`, `Password`, `PublicKey` and `KeyboardInteractive`.

`PublicKey` sends a query, then a signature over the session identifier. Its `Signer` may be a certified key or come from `std/ssh/agent`. A client that exhausts its methods fails with `AuthFailed`.

#### SSH server authentication

The server calls `ServerAuth` callbacks.

- It can require several methods in sequence (`required`, answered with partial success).
- It shows a banner.
- It ends the connection with `Disconnected` after `max_attempts` failures (default 6). Queries and `none` requests do not count as failures.

#### SSH handshake timeout

`ClientConfig.handshake_timeout` (milliseconds, 0 or less for none) bounds the TCP connect of `dial` and the version exchange, key exchange and authentication of `dial` and `client` together. `dial` gives the handshake what the connect left.

- The bound is a timer green thread. It closes the stream when it fires.
- It stops once authentication succeeds, so later channel IO is not bound by it.
- `dial_through` bounds only the handshake.

#### SSH handshake failures

A failure of the transport in a stage is `Handshake(phase, cause)`. Such a failure is a timeout, an end of stream or a stream error. `Phase` is `Connect`, `VersionExchange`, `KeyExchange` or `Authentication`. `cause` is `TimedOut` for the timeout.

The failures the protocol itself reports keep their variants, which name their stage:

| Variant | Stage |
| --- | --- |
| `BadVersion` | The version exchange. |
| `NoCommonAlgorithm` | The key exchange. |
| `KeyExchange` | The key exchange. |
| `BadSignature` | The key exchange. |
| `StrictKex` | The key exchange. |
| `HostKeyRejected` | The host key. |
| `AuthFailed` | Authentication. |

#### SSH channels

Channels are multiplexed with a 2 MiB window per channel, adjusted after half is consumed.

| Topic | Behaviour |
| --- | --- |
| Writes | A write waits while the peer's window is empty, so a slow reader stops the writer and the sender never overruns it. |
| Open channels | At most 1024 channels are open. |
| `close_write_on` | Sends EOF. |
| Closing | A channel closes once both sides have sent `CLOSE`. |
| Extended data | Extended data type 1 is stderr. |

#### SSH server channel handlers

A server handler called for each `Incoming` calls `accept` or `reject`. By default every channel is refused.

#### SSH session exit and exec

A session client reads `wait_exit_on` for the `exit-status` or `exit-signal` the server sent. `exec` collects stdout, stderr and the exit status.

#### SSH forwarding

| Function | What it does |
| --- | --- |
| `listen_local` | Binds a local port and opens a `direct-tcpip` channel for each connection. |
| `listen_remote` | Sends `tcpip-forward`. |
| `accept_forward` | Returns each `forwarded-tcpip` channel with the origin address. |
| `direct_streamlocal` | Reaches a Unix socket. |
| `serve_direct_tcpip` and `forward_global` | Give a server the matching handlers. |

#### SSH key parsing and certificates

`key.parse_private_key` reads unencrypted and `aes256-ctr` plus bcrypt encrypted `openssh-key-v1` keys, with the check integers verified before the keys are returned, and PEM keys. A wrong passphrase is `WrongPassphrase`.

A certificate checks:

- The signature of its authority over the certificate body.
- Its validity window.
- Its type.
- The principal that logs in.

#### SFTP request pipelining

SFTP version 3 requests are pipelined.

| Topic | Behaviour |
| --- | --- |
| Reads | Run ahead by 8 requests. |
| Writes | Are sent without waiting for each reply, with a failure reported by a later write, `flush` or `close`. |
| Sizes | Come from `limits@openssh.com` when the server sends it, capped at 65536. |

#### SFTP server path resolution

- The server resolves every path from its root. `..` stops at the root.
- Symbolic links are expanded in the process. An absolute link target is taken relative to the root. More than 40 links fail.
- A failing `lstat` other than `NotFound` ends the request.
- The root itself cannot be removed, renamed or replaced.
- Each operation resolves its path again, so the server does not protect against a directory replaced by a link while a request runs.

#### SFTP paths on Windows

- On Windows a backslash separates components in client paths and link targets.
- On Windows a drive prefix of a link target is dropped before the target is taken relative to the root.
- On Windows a component is `NotFound` if it has a character Windows rejects (`<>:"|?*`, control characters), a trailing dot or space, or a device name (`CON`, `NUL`, `COM1` and the others).

#### SSH agent connections

`std/ssh/agent` keeps one `io.Stream` per `Session`. `dial` makes it, in `agent_linux.low` (a Unix socket through `std/net`) or in `agent_windows.low`.

- The Windows `dial` opens a path that starts with `\\.\pipe\` through `sys.open_pipe` and `sys.stream_on`, so reads and writes park the green thread.
- It sends any other path to the Unix socket as on Linux.

| Function | What it does |
| --- | --- |
| `connect_env` | Uses `SSH_AUTH_SOCK`. When that is unset or empty, it uses the OpenSSH agent service pipe `\\.\pipe\openssh-ssh-agent` on Windows and `NoAgent` elsewhere. |
| `connect_pageant` | Finds the window of class `Pageant`. |
| `connect_any` | Tries `SSH_AUTH_SOCK` when it is set, then on Windows the pipe and Pageant, and fails with the first failure. |

#### SSH Pageant exchange

Each request:

1. Creates a file mapping named `PageantRequest` and sixteen hex digits.
2. Copies the framed request into the mapping.
3. Sends `WM_COPYDATA` with `dwData` `0x804e50ba` and the mapping's name through `SendMessageTimeoutW` (a minute, abort if hung).
4. Reads the framed reply back from the mapping.

The mapping has the default security descriptor and holds at most 262144 bytes.

`agent/pageant.low` wraps that exchange in an `io.Stream`. A write collects one frame and calls the exchange. Reads return the reply.

#### SSH agent tests

- `agent/pageant_test.low` runs the framing against a fake exchange on Linux. No test exercises Pageant itself.
- `testdata/windows/ssh_agent_pipe.low` serves a fake agent on a named pipe from a child process.
- `std/ssh/agent/agent_test_windows.low` checks the default pipe, a missing pipe and a missing Pageant window.

#### SSH test platforms

- The OpenSSH interoperation tests and the permission-bit tests of `std/ssh/sftp` run on Linux only (`interop_test_linux.low`, `sftp_test_linux.low`).
- `sftp_test_windows.low` holds the Windows path and link containment tests.

#### SSH interoperation with OpenSSH

Interoperation is checked against OpenSSH 10.6p1. The `ssh`, `sftp` and `scp` clients run against the server and the client runs against an unprivileged `sshd`, over every cipher, MAC, key exchange and host key above.

### QUIC

An endpoint is one UDP socket and one reader green thread.

#### QUIC datagram routing

A datagram over 1500 bytes is dropped. The rest are routed by destination connection ID, at most 4096 routes.

#### QUIC connection loop

Each connection is a loop green thread that owns all of its state. The reader, the TLS handshake green thread and user calls reach it through channels. Every send to it also waits on a channel that closes when the loop exits, so nothing blocks on a connection that has ended.

#### QUIC Version Negotiation and incomplete handshakes

A server answers an unsupported version with Version Negotiation only for datagrams of at least 1200 bytes.

It holds at most 256 connections whose handshake is incomplete, at most 8 of them per client IP address. An Initial beyond either limit is dropped. From 128 of them, or always when `Config.retry` is set, a new Initial gets a Retry.

#### QUIC Retry token

The token of a Retry is an HMAC-SHA256 over:

- The client address.
- An expiry 10 seconds ahead.
- The original connection ID.
- The source connection ID of the Retry.

The token is compared in constant time. The token is accepted only on an Initial whose destination connection ID is that source connection ID, so one token starts one connection: later Initials with that ID are routed to it.

#### QUIC address validation limit

Before the client address is validated the server sends at most three times the bytes received.

#### QUIC client Retry handling

A client accepts one Retry. It ignores Retry and Version Negotiation once it has processed another packet.

#### QUIC stateless reset

A short-header datagram of at least 42 bytes for no connection gets a smaller stateless reset. Its token is an HMAC of the connection ID.

#### QUIC handshake buffers and keys

The handshake runs `tls.quic_client` or `tls.quic_server` on its own green thread.

| Item | Behaviour |
| --- | --- |
| Received CRYPTO data | Reassembled to at most 64 KiB in at most 64 pieces per level. Beyond that the connection fails with `CRYPTO_BUFFER_EXCEEDED`. |
| Packets waiting for keys not yet installed | At most 16 packets wait. |
| CRYPTO data that arrives after the handshake | Discarded, such as session tickets. |
| Initial keys | Discarded when Handshake packets flow. |
| Handshake keys | Discarded when the handshake is confirmed. |

#### QUIC handshake timeout

The handshake timeout runs until the handshake is confirmed. For a client that is the arrival of HANDSHAKE_DONE. So a client whose server abandoned the handshake after the client finished its side ends with `TimedOut` within that timeout.

#### QUIC transport parameter checks

Each side checks the peer's transport parameters:

- The connection IDs they name, including a Retry's, must match the ones seen on the wire.
- A client must not send the parameters only a server sends.

#### QUIC acknowledgements

Packets carry ACKs for at most 32 received ranges, sent within 5 ms or at once after two ack-eliciting packets.

#### QUIC loss recovery

Loss recovery is `std/quic/recovery`.

- The sender skips a packet number every 128 to 384 packets. An ACK of a skipped or unsent number is `PROTOCOL_VIOLATION`.
- Only the first 256 ranges of an ACK frame are processed.
- Lost frames are sent again from what the packet carried, so stream data, flow control updates and control frames are rebuilt rather than copied.

#### QUIC key updates and AEAD limits

1-RTT keys are updated after 2^23 packets or when `update_keys` asks, and a peer's key update is followed. Every change of the sending keys sends a PING, so the acknowledgement that allows the next update arrives even when there is no data to send.

A connection closes with `AEAD_LIMIT_REACHED` once more packets fail authentication than the integrity limit of its AEAD. The limit is 2^52, or 2^36 for ChaCha20-Poly1305.

#### QUIC connection IDs

| Item | Behaviour |
| --- | --- |
| Issued IDs | The endpoint issues 8-byte IDs. |
| Peer's IDs | The endpoint stores at most 2 of the peer's. |
| RETIRE_CONNECTION_ID frames owed to the peer | Capped at 8. Beyond that the connection fails with `CONNECTION_ID_LIMIT_ERROR`. |
| PATH_CHALLENGE | Only the latest is answered. |

Connection migration is not supported.

#### QUIC stream flow control

Flow control applies per stream and per connection in both directions, and the receive windows are credited as the application reads.

- Data past the flow control limit, a changed final size and an offset plus length over 2^62-1 fail the connection with the matching error.
- `write` parks while the stream's 1 MiB send buffer or the peer's credit is exhausted.

#### QUIC stream receive buffer

A stream reassembles into one receive buffer.

- The buffer spans the read position to the highest offset received.
- It grows by doubling, never past the flow control window.
- Each byte is copied once into place.
- Bytes that were already received are not overwritten by a retransmission.
- A sorted list of received ranges describes the buffer. The list holds at most 256 ranges, and beyond that the connection fails with `PROTOCOL_VIOLATION`.

#### QUIC stream counts

A peer opens at most the advertised number of streams of each type. At most 4096 are tracked. The allowance of a peer-opened stream returns (MAX_STREAMS) only when the stream is complete and the application has accepted it. So streams the application never accepts stay bounded by the advertised number.

#### QUIC closing a connection

| Function | What it does |
| --- | --- |
| `close_on` | Sends CONNECTION_CLOSE. The connection stays closing for three probe timeouts, answering with the same frame. While a stream holds data the peer has not acknowledged, `close_on` holds the CONNECTION_CLOSE back for at most 5 seconds and never past the idle timeout. |
| `close_listener` | Asks every connection to close before it waits for any, so it takes as long as the slowest one, not the sum. |

A CONNECTION_CLOSE from the peer drains it. The idle timeout, the handshake timeout and a stateless reset end it. A reason phrase is truncated to 1024 bytes and never interpreted.

#### QUIC unsupported features

Not supported:

- 0-RTT.
- Session resumption.
- Connection migration.
- Datagrams (RFC 9221).
- Versions other than 1.

### STARK security

`stark.security` returns three levels for an `Air` and `Params`.

#### STARK security levels

| Level | What it is |
| --- | --- |
| `conjectured` | Is `min(191, queries * log2(blowup))` plus the grinding bits once the queries alone give 80 bits, capped at 128 by the 256-bit Blake3 hashes of the Merkle trees. It assumes the proximity gap conjecture for Reed-Solomon codes up to capacity. |
| `proven_unique` | Rests on the proximity gap theorems of eprint 2024/1553. It is for unique decoding. |
| `proven_list` | Rests on the proximity gap theorems of eprint 2024/1553. It is for the list decoding that the Johnson bound allows (the best proximity parameter from 3 up). |

Both `proven_unique` and `proven_list` use the cubic extension of 192 bits, a random linear combination of the constraints and of the DEEP terms, and folding by 2.

#### STARK security parameter sets

The formulas are Winterfell 0.13.1's `ConjecturedSecurity` and `ProvenSecurity`. `std_zk_stark` compares six parameter sets with its output.

| `blowup` | `queries` | `grinding_bits` | conjectured | proven unique | proven list |
| --- | --- | --- | --- | --- | --- |
| 4 | 100 | 20 | 128 | 87 | 118 |
| 8 | 40 | 16 | 128 | 49 | 75 |
| 8 | 64 | 20 | 128 | 73 | 114 |
| 16 | 64 | 20 | 128 | 78 | 128 |
| 16 | 100 | 24 | 128 | 115 | 128 |
| 32 | 80 | 20 | 128 | 96 | 128 |
| 64 | 40 | 16 | 128 | 55 | 127 |

#### STARK security and trace length

The levels are for a trace of 65536 rows. A longer trace lowers the proven ones. `zero_knowledge` pads the trace, which `security` takes into account.

#### STARK statements and proof binding

A statement is the `Air` with its boundary values. A changed public input or a changed constraint is another statement and its proof is rejected. A proof is bound to its `Params` through the header and the transcript, so a verifier that fixes its own `Params` rejects a proof made for weaker ones.

#### STARK trace hiding

`std/zk/stark` does not hide anything about the trace without `zero_knowledge`. Its hiding with it is the trace randomisation that `std/zk/stark` in the table above describes.

### WebRTC

#### STUN

`std/stun` implements RFC 8489 as a codec plus a client and a server over UDP.

##### STUN message parsing and encoding

A message is parsed only when:

- its datagram length matches the header
- the top two bits of the type are clear
- the magic cookie matches
- every attribute fits

A parsed message keeps its bytes, so the integrity and fingerprint checks hash exactly what arrived, whatever padding the sender used. Encoding pads attribute values with zero bytes, or with `Message.padding`.

##### STUN integrity and fingerprint checks

`check_integrity` hashes the message up to the attribute, with the length field rewritten to end after it. So attributes that follow, FINGERPRINT included, do not matter. It compares in constant time.

- MESSAGE-INTEGRITY-SHA256 values of 16 to 32 bytes in steps of four are accepted. `seal` writes all 32.
- `check_fingerprint` requires the attribute to be last.

##### STUN credentials

Long-term keys are `MD5(username:realm:password)` or the SHA-256 form without SASLprep. A caller whose password changes under SASLprep prepares it first. Short-term credentials use the password bytes as the key.

##### STUN client requests

`request` retransmits on the RFC schedule, the first send at 0 and then after 500, 1000, 2000, 4000, 8000 and 16000 ms. It fails with `TimedOut` 8000 ms after the seventh send.

It ignores datagrams:

- from another address
- with another transaction ID
- that do not parse
- when a key is given, without a valid MESSAGE-INTEGRITY

Datagrams reach it through a reader green thread and a channel of 64. A datagram that arrives when the channel is full is dropped.

##### STUN server responses

`respond` answers Binding requests only:

| Received | Answer |
| --- | --- |
| An indication, a response or a datagram with a wrong FINGERPRINT | Nothing |
| Another method | 400 |
| An unknown attribute below `0x8000` | 420 |
| With `authenticate`, a request without a known USERNAME and valid MESSAGE-INTEGRITY | 400 or 401 |

#### ICE

`std/ice` is a state machine. It holds the checklist, the credentials and the timers.

##### ICE agent time and transmits

Every function takes the time as a count of milliseconds, so the same code runs against a socket and against an in-memory network. The checks use the `std/stun` codec.

The agent sends no datagram itself. It queues `Transmit` values instead. A `Transmit` names the bound address of the socket to send from. With a `Tcp` or `Tls` transport, it names the host address whose connection to a TURN server carries the message.

##### ICE candidate priorities

Candidates have the priorities of RFC 8445 section 5.1.2:

| Candidate | Priority |
| --- | --- |
| Host | 126 |
| Peer reflexive | 110 |
| Server reflexive | 100 |
| Relayed | 0 |

The local preference ranks IPv6 above IPv4 and earlier addresses above later ones. Pair priorities come from section 6.1.2.3. A foundation is the CRC-32 of the type, base IP, server IP and transport, in eight hexadecimal digits.

##### ICE candidate parsing

`parse` takes the value of an `a=candidate` line, with or without the `a=` and `candidate:` prefixes. It skips extension attributes. It fails with `Unsupported` for TCP candidates and for host names, including the `.local` names of mDNS.

##### ICE candidate gathering

Gathering reports each host candidate of `Config.hosts`. It sends a Binding request from each to every `stun_servers` entry of the same family. The answer becomes a server-reflexive candidate whose related address is the host candidate.

Each `turn_servers` entry of the same family gets a `std/turn` client on each host socket. Over TCP or TLS it gets one on a connection for each host address. Its allocation becomes a relayed candidate whose related address is the server-reflexive address the server reports.

Gathering completes once every allocation has succeeded or failed.

##### ICE relayed candidates over UDP, TCP and TLS

The relayed side is UDP whatever the transport to the server. A relayed candidate's foundation includes that transport. Its local preference ranks a candidate through UDP above one through TCP, and one through TCP above one through TLS.

##### ICE relay connection loss

`relay_lost` fails the allocation of a connection that closed or could not be made. That settles the allocation for gathering. Its relayed candidate's checks and consent then fail as they time out. The next `gather` opens a new connection.

##### ICE gathering while a pair is selected

A `gather` while a pair is selected keeps the allocation whose relayed address is the selected pair's base, and reports its relayed candidate again. It does so because a second allocation on the same socket or connection would have the same 5-tuple.

The `gather` releases and makes again the other allocations. The selected pair so carries data through an ICE restart. Once the new checklist selects a pair, the kept allocation is released unless that pair uses it.

##### ICE relay-only policy

`relay_only` is the W3C `relay` transport policy. With it, the agent announces relayed candidates alone and sends no Binding request to a STUN server.

##### ICE relayed traffic

A check or a datagram sent from a relayed candidate goes to the server inside a Send indication or, once its channel is bound, ChannelData. What the server relays back is unwrapped. A STUN message is given to the agent, and anything else is queued for `poll_delivery` with the peer's address.

`is_relay_server` tells a caller which datagrams the agent must see.

##### ICE local addresses

`local_addresses` takes the addresses of the interfaces `net.interfaces` reports as up. It leaves out link-local addresses and, unless asked, loopback addresses.

##### ICE checklist pruning

The checklist is pruned as RFC 8445 section 6.1.2.4 says. Of two pairs with the same remote candidate and the same local base, only the higher priority stays. Pairs start frozen or waiting by foundation.

##### ICE check scheduling

- The checks run one per `pacing` milliseconds, from the triggered queue first and then by priority.
- A check is sent `transmissions` times on the schedule of `stun.waits`.
- A request that arrives for a pair whose check is in flight leaves that check running instead of cancelling it, because two agents that start together would otherwise cancel each other without end.

##### ICE incoming requests

A request must carry:

- a USERNAME that starts with the local username fragment
- a valid MESSAGE-INTEGRITY under the local password

When a FINGERPRINT is present, it must be valid.

| Request | Answer |
| --- | --- |
| A missing USERNAME | 400 |
| A wrong USERNAME or a bad MESSAGE-INTEGRITY | 401 |
| An unknown attribute below `0x8000` | 420 |
| A datagram that does not parse or has a wrong FINGERPRINT | Nothing |

##### ICE role conflicts

The tie-breakers settle a role conflict as RFC 8445 section 7.3.1.1 says. The answer is 487 when the receiver keeps its role.

##### ICE peer-reflexive candidates

A source address that no remote candidate has becomes a peer-reflexive remote candidate with the priority the request states. A candidate signalled later with the same address replaces it with its own type and priority, as libwebrtc does.

A successful response that reports an unknown mapped address creates a peer-reflexive local candidate whose base is the sending socket.

##### ICE nomination

Nomination is regular.

| Role | Nomination |
| --- | --- |
| Controlling agent | It sends a check with USE-CANDIDATE on the best valid pair as soon as one exists. The pair is selected when that check succeeds. |
| Controlled agent | It selects a valid pair when a USE-CANDIDATE request has arrived for it. |

##### ICE-lite agents

A remote that is ICE-lite makes the agent controlling. An ICE-lite agent:

- gathers host candidates only
- sends no checks
- treats each authenticated request as a valid pair
- selects the pair whose request carries USE-CANDIDATE

##### ICE consent checks

After selection a full agent sends a consent check on the selected pair every `consent_interval` milliseconds, randomised between 80 and 120 percent (RFC 7675). With `consent` off, it sends a Binding indication after `keepalive_interval` milliseconds without any send.

An ICE-lite agent counts the authenticated requests it receives.

##### ICE connection states

| State | Condition |
| --- | --- |
| `Disconnected` | No consent response has arrived for `disconnected_after` milliseconds. The state returns to `Completed` when one arrives. |
| `Failed` | After `consent_timeout`. |
| `Failed` | Every pair has failed after `end_of_remote_candidates`. |
| `Failed` | No pair is selected `failure_timeout` milliseconds after the remote credentials arrive. |

##### ICE restart

`restart` gives the agent new credentials and drops the remote credentials, candidates and pairs. The caller gathers again and sets the remote's new credentials. A different remote username fragment does the same on the other side.

The selected pair, with its consent state, stays in use until the new checklist nominates one. Its remote address stays known, so the datagrams that are not STUN still arrive from it.

##### ICE session threads

`std/ice/session` reads every socket in its own green thread and hands the datagrams to the one green thread that owns the agent. The commands go through a channel to the same green thread. `events` and `incoming` hold 256 entries and drop what does not fit.

##### ICE session datagram routing

| Datagram | Goes |
| --- | --- |
| A first byte of 0 to 3 | To the agent. |
| Another first byte | On `incoming` only when its source is a known remote address, including the selected pair's remote. |
| From a TURN server | To the agent whatever its first byte. |

A DTLS or RTP datagram from a stranger is therefore dropped (RFC 7983). The data a TURN server relays arrives on `incoming` with the peer as its source. `send` on a relayed pair goes through the agent.

##### ICE session TCP and TLS TURN connections

- The first message for a TCP or TLS TURN server opens a connection in its own green thread.
- That thread connects and finishes the TLS handshake within `connect_timeout`.
- It verifies the certificate against `Options.tls` and the server's `server_name` or, when that is empty, its IP address.
- It splits what it reads with a `turn.Framer` and hands each message to the agent without dropping it.
- A second green thread writes the queued messages in order.

##### ICE session relay connection loss and close

The agent hears `relay_lost` when the connection:

- closes
- fails to connect or verify
- sends bytes that do not frame, including a partial message at the end

A notice from a connection the session already replaced is ignored. Closing the session writes the queued Refresh of lifetime 0 before the connections close.

#### TURN

`std/turn` is a state machine like `std/ice`. Every function takes the time as a count of milliseconds.

##### TURN client requests

The client queues datagrams for `poll_transmit`. Requests use long-term credentials with the MD5 key.

| Event | Result |
| --- | --- |
| The first Allocate | It goes unsigned. |
| A 401 answer | It gives the realm and nonce, and the request is sent again signed. |
| A 438 answer | It replaces the nonce, and the request is sent again, at most twice. |
| A 401 to a signed request | It fails the allocation. |
| Any other error | It fails the allocation. |
| A request unanswered after `transmissions` sends on the `stun.waits` schedule | It fails the allocation. |

Responses are checked for MESSAGE-INTEGRITY and matched by transaction ID.

##### TURN over TCP and TLS

- Over TCP and TLS each request is sent once and fails after `stream_timeout`, 39.5 seconds (Ti, RFC 8489 section 6.2.2), and ChannelData is padded to four bytes.

##### TURN requested transport

REQUESTED-TRANSPORT is UDP on every transport.

##### TURN stream framing

A `Framer` takes a STUN message as its header and stated length and ChannelData as its header and length rounded up to four.

- It fails with `Malformed` on a first byte that is neither, or on a STUN length that is not a multiple of four.
- It keeps a partial message for the next read. `partial` reports what is left when the stream ends.

##### TURN connection loss

The server deletes an allocation whose connection closes, because the allocation belongs to the connection's 5-tuple. So `connection_lost` fails it, and the client does not reconnect: a new connection needs a new client and gives a new relayed address.

##### TURN URIs

`parse_uri` takes `turn:` with `?transport=udp` or `tcp`, and `turns:` (TLS over TCP), with the default ports 3478 and 5349. It refuses `turns:` over UDP, which is DTLS.

##### TURN permissions and channels

1. `send_to` asks for a permission for the peer's IP address with CreatePermission.
2. It holds up to 16 datagrams per address until the permission is granted.
3. Then it sends Send indications and binds a channel from 0x4000 upwards with ChannelBind.
4. After that, data goes as ChannelData, padded to four bytes.

Data from the server is accepted only from a permitted address.

##### TURN refreshes and close

Permissions are refreshed every 4 minutes, channels every 9 and the allocation a minute before its lifetime ends. `close` sends a Refresh with lifetime 0.

##### TURN client tests

`std/turn/client_test.low` runs the client against a TURN server written in the test. It covers:

- the 401 challenge
- a stale nonce
- a wrong password
- data held for a permission and then moved to a channel both ways
- Data indications
- the refresh and release
- an Allocate that is never answered

Over a stream it covers:

- framing at every split point and a byte at a time
- coalesced and padded messages
- a partial message
- a request sent once and timed out after Ti
- a lost connection

##### TURN relay tests with an in-test server

`std/ice/session/stream_relay_test.low` runs a TURN server written in the test. The server has a TCP and a TLS listener, with a certificate from `x509.create_self_signed`. It relays over UDP and writes each batch of messages in two pieces.

An agent with `relay_only` connects through it over each transport to an agent with host candidates. A wrong TLS name, an untrusted certificate, a refused connection and a response cut off at close each end gathering without a candidate. A connection the server closes after the allocation is replaced by the next `gather`.

The server also listens on UDP. Over UDP, TCP and TLS the relay-only agent restarts ICE and gathers again while data flows both ways over the relayed pair, before and after the new pair is selected. This keeps the relayed address.

##### TURN relay tests against pion

`std/ice/session/relay_test_linux.low` builds `testdata/turn/pion`, a pion/turn 5.1.3 server on 127.0.0.1 with UDP, TCP and TLS listeners.

- It connects an agent with `relay_only` through each listener to an agent with host candidates, with data both ways over the relayed pair.
- It restarts ICE with data flowing, as in the in-test server's restart.
- It skips when `go` or `timeout` is not installed or the pion modules cannot be downloaded.

##### Short stream frames in pion/turn

pion/turn 5.1.3 holds a stream frame shorter than 9 bytes until more bytes arrive. So the test sends more than four bytes of ChannelData.

#### mDNS

`std/mdns` answers and asks for the addresses of `.local` names only. It has no service discovery, probing, announcement or conflict resolution.

##### mDNS names

Names are random UUIDs, as browsers make them for host candidates, so no two hosts claim one.

##### mDNS queries

A query asks one question, for `A` or `AAAA` records, because Chromium's responder ignores a query with more than one.

##### mDNS resolve

`resolve` binds port 5353, shared with the host's other mDNS sockets, and joins the group. It does so because Chromium answers a one-shot query only to the group and never to the querier's port.

When port 5353 cannot be shared, `resolve` queries from a free port and hears only the unicast answers a one-shot query gets. It sends both queries again every second until an answer names an address or the wait ends.

##### mDNS responder answers

`respond` answers a query as follows:

| Query from | Answer |
| --- | --- |
| A port other than 5353 | It answers as a one-shot query (RFC 6762 section 6.7). The answer goes to the querier with its ID, its questions and a time to live of 10 seconds. |
| Port 5353 | It answers with ID 0, the cache-flush bit and a time to live of 120 seconds. That answer goes to the group unless every question set the unicast-response bit. |

##### mDNS responder interfaces

The responder replies through every multicast interface with an IPv4 address, as Chromium does, so a querier listening on any of them hears it.

##### mDNS tests

- `std/mdns/mdns_test.low` checks the codec against a response captured from Chromium 158, with a documentation address in place of its own.
- It runs a responder and `resolve`'s query loop on loopback sockets.

##### mDNS checks

- By hand, `resolve` resolved Chromium's and Firefox's host candidate names.
- `avahi-resolve` 0.9-rc5 resolved a name `responder` published.
- Python sockets on port 5353 got both the one-shot unicast and the multicast answer from it, once through each multicast interface.

#### DTLS

`std/dtls` implements the client and the server of DTLS 1.3 and DTLS 1.2.

##### DTLS suites and key exchange

| Version | Suites |
| --- | --- |
| DTLS 1.3 | `0x1301` (AES-128-GCM), `0x1302` (AES-256-GCM) and `0x1303` (ChaCha20-Poly1305). |
| DTLS 1.2 | The ECDHE-ECDSA suites `0xC02B` (AES-128-GCM), `0xC02C` (AES-256-GCM) and `0xCCA9` (ChaCha20-Poly1305). |

Both use X25519 and P-256 and ECDSA P-256 or P-384 certificates.

`std/dtls` reuses the `std/tls` key schedules, key exchange and signature code, with the DTLS 1.3 labels under the prefix `dtls13`. `std/dtls` adds the record layers, the handshake transport and the certificate policy of WebRTC.

##### DTLS 1.2 extended master secret and renegotiation

In DTLS 1.2 the extended master secret is required and the `renegotiation_info` extension is the empty one. A ClientHello or HelloRequest sent after a DTLS 1.2 handshake finishes gets the warning alert `no_renegotiation` and changes nothing.

##### DTLS version negotiation

The version is negotiated through `supported_versions` (RFC 8446 section 4.2.1).

- A client whose `max_version` is `dtls13` offers DTLS 1.3 with its 1.3 suites and a key share for its first group. It offers DTLS 1.2 beside it unless `min_version` is `dtls13`.
- The `legacy_version` field is always `0xFEFD`.
- A server picks DTLS 1.3 when both allow it, and DTLS 1.2 otherwise.
- Peers with no version in common fail with `protocol_version`.

##### DTLS downgrade protection

A server that allows DTLS 1.3 and negotiates DTLS 1.2 ends its random with `DOWNGRD` and 1. A client that offered DTLS 1.3 fails a DTLS 1.2 ServerHello that carries it (RFC 8446 section 4.1.3). So stripping `supported_versions` from a hello is detected.

##### DTLS 1.2 record format

A DTLS 1.2 record is `epoch`, a 48-bit sequence number and a body.

| Epoch | Holds |
| --- | --- |
| 0 | Plaintext. |
| 1 | Protected. It starts at the ChangeCipherSpec. |

##### DTLS 1.2 record protection

| Suite | AEAD nonce |
| --- | --- |
| A GCM suite | The 4-byte implicit part and the 8 bytes of epoch and sequence number sent in the record. |
| ChaCha20-Poly1305 | The write IV XORed with the epoch and sequence number (RFC 7905). |

The additional data names the epoch, the sequence number, the type, the version and the plaintext length.

##### DTLS 1.2 replay window

The receiver keeps a window of 64 sequence numbers for epoch 1. It drops a record that is outside the window or repeated. The window moves only after the record authenticates, so a forged record cannot push it.

Epoch 0 has no window because nothing authenticates it: handshake messages are deduplicated by `message_seq`.

##### DTLS 1.2 dropped records

- An epoch-0 alert or application record is dropped once the keys of epoch 1 exist.
- A record that does not parse, authenticate or fit the epoch is dropped without an alert (RFC 6347 section 4.1.2.7).

##### DTLS 1.3 record header

A DTLS 1.3 record is sent in the unified header of RFC 9147 section 4. The first byte carries the low two bits of the epoch. A 16-bit sequence number and a length follow. The sequence number is encrypted with the mask that AES-ECB or ChaCha20 under the `sn` key makes from the first 16 bytes of the ciphertext.

##### DTLS 1.3 record receiving

The receiver also reads the 8-bit and the lengthless forms. It takes the full sequence number closest to the next one expected. It drops a record with a connection ID, because connection IDs (RFC 9146) are not implemented.

##### DTLS 1.3 record protection

| Part | Value |
| --- | --- |
| Nonce | The write IV XORed with the 64-bit sequence number. |
| Additional data | The header with the sequence number unencrypted. |
| Inner plaintext | The content, its type and zero padding. |

##### DTLS 1.3 epochs

| Epoch | Carries |
| --- | --- |
| 0 | Plaintext. |
| 2 | The handshake. |
| 3 | The application data. |

Each KeyUpdate moves one direction to the next epoch. A reader is kept for each of the four values of the epoch's low bits. Each reader has its own window of 64, so a late record of the previous epoch is still read.

##### DTLS 1.3 plaintext records

Once DTLS 1.3 is negotiated, a plaintext record is read only as a ClientHello or ServerHello, or as an alert while no keys exist.

##### DTLS handshake fragmentation

Handshake messages carry `message_seq`, a fragment offset and a fragment length. A message larger than `Config.mtu` is split across records and datagrams. The receiver reassembles the next eight message numbers, in any arrival order and with duplicates, up to 65536 bytes per message.

##### DTLS handshake transcripts

| Version | Transcript |
| --- | --- |
| DTLS 1.2 | The Finished hash covers each message as one unfragmented message with its DTLS header. It leaves out the first ClientHello and the HelloVerifyRequest. |
| DTLS 1.3 | The transcript covers each message in its TLS 1.3 form, without `message_seq` and the fragment fields. After a HelloRetryRequest it starts with the `message_hash` of the first ClientHello. |

##### DTLS server cookies

A server with `Config.cookies` keeps no state for a ClientHello without a valid cookie. That means no reassembly buffer, no transcript and no timer.

| Version | Answer |
| --- | --- |
| DTLS 1.2 | It answers with a HelloVerifyRequest whose cookie is an HMAC-SHA-256 over the client's random, session id, cipher suites and compression methods under a random per-connection secret. |
| DTLS 1.3 | It answers with a HelloRetryRequest. Its cookie holds the chosen suite and group, whether a key share is asked for and the hash of the first ClientHello, under an HMAC-SHA-256 with that secret. |

The server rebuilds the HelloRetryRequest and the transcript from the cookie of the second ClientHello.

##### DTLS 1.3 HelloRetryRequest without cookies

A DTLS 1.3 server without cookies sends a HelloRetryRequest only when the client has no key share for the group the server takes.

##### DTLS invalid ClientHello datagrams

A server answers a datagram that is not a valid ClientHello with nothing.

##### DTLS 1.2 flight retransmission

A DTLS 1.2 flight is retransmitted as a whole after `retransmit_initial` milliseconds, doubling to `retransmit_max`. Each message keeps its original `message_seq` and gets new record sequence numbers. The handshake fails after `handshake_timeout`.

##### DTLS 1.2 repeated flights

A server that has finished and sees a message of the client's last flight again sends its last flight once more. A client never answers a repeated server flight, so two peers cannot keep each other sending.

##### DTLS 1.3 acknowledgements

DTLS 1.3 acknowledges records with ACK (RFC 9147 section 7). Each sent fragment is remembered by its record number, so an ACK marks those bytes received and the next retransmission, on the same timer as DTLS 1.2, sends only what is left, in new records.

- The peer's next flight acknowledges a flight as a whole.
- Application data from the server acknowledges the client's last flight.

##### DTLS 1.3 when an ACK is sent

A receiver sends an ACK of the handshake records it has read when it holds a message beyond the next one it expects. It sends one when the client's last flight completes, and for a NewSessionTicket or KeyUpdate. A server that has finished and sees the client's last flight again acknowledges it again.

##### DTLS 1.3 ACK contents and epoch

- An ACK lists only the newest records that fit `Config.mtu`.
- An ACK is sent in the newest epoch the sender writes, so the server acknowledges the client's last flight at epoch 3.
- An ACK lists only records the receiver opened. A record of an epoch whose keys the receiver does not hold yet is dropped unacknowledged and comes back in the peer's retransmission.

##### DTLS 1.3 retransmission until acknowledged

The client's last flight and a KeyUpdate are retransmitted until acknowledged. They are given up after `handshake_timeout` without failing the connection.

##### DTLS 1.3 key update

`update_keys` sends a KeyUpdate, optionally asking the peer to update too. It fails with:

| Error | Condition |
| --- | --- |
| `NotConnected` | Before the handshake finishes. |
| `Unsupported` | Over DTLS 1.2. |
| `WouldBlock` | While a flight is unacknowledged. |

The sender writes with the new keys once the KeyUpdate is acknowledged (RFC 9147 section 8). The receiver reads the next epoch at once and answers a request with its own KeyUpdate.

##### DTLS certificate exchange

WebRTC peers authenticate with self-signed certificates, so both roles send a Certificate, the server sends a CertificateRequest, and the client sends a CertificateVerify.

##### DTLS peer certificate checks

The peer certificate is accepted when its digest equals one of `Config.fingerprints`, as the remote SDP states, compared in constant time, and is refused otherwise unless `unverified` is set. Validity dates and the issuer are not checked (RFC 8827 section 6.5).

##### DTLS identity key and signature algorithms

The identity key is ECDSA on P-256 or P-384. The handshake offers and accepts `ecdsa_secp256r1_sha256` and `ecdsa_secp384r1_sha384` only.

##### DTLS SRTP negotiation

`use_srtp` (RFC 5764) is offered by a client and answered by a server only when `Config.srtp_profiles` is not empty. In DTLS 1.3 the answer is in EncryptedExtensions. The server picks the first of its own profiles that the client offered. When none is shared, the extension is left out and the handshake continues.

##### DTLS keying material export

Over DTLS 1.2 `export_keying_material` is the TLS 1.2 PRF over the label, the client random and the server random, with the length-prefixed context appended for `export_keying_material_context`. Over DTLS 1.3 it is the TLS 1.3 exporter over the exporter master secret, with an empty context for `export_keying_material`.

`"EXTRACTOR-dtls_srtp"` with 60 bytes gives the SRTP keys and salts of both directions.

##### DTLS writes and closing

- A write is one record in one datagram and fails with `TooLarge` when it exceeds `Config.mtu`.
- `close` sends `close_notify`.
- A received `close_notify` closes the connection.
- A fatal alert fails it.

##### DTLS session

`std/dtls/session` runs a `Conn` in one green thread that waits on commands, datagrams and the retransmission timer together. `Handle.data` and `Handle.events` drop what does not fit. A `Transport` has no socket of its own, so the same code runs over `std/ice/session`'s `send` and `incoming`.

##### DTLS 1.3 tests with OpenSSL records

`std/dtls/conn13_test.low` opens records captured from OpenSSL 4.1.0 in each DTLS 1.3 suite. It seals them back to the same bytes. It checks the exporter against OpenSSL's `EXTRACTOR-dtls_srtp` output under the logged secrets.

##### DTLS OpenSSL interop test

`std/dtls/session/interop_test_linux.low` runs `openssl s_client` against a `std/dtls/session` server and a client against `openssl s_server`, with `SRTP_AES128_CM_SHA1_80`.

##### DTLS 1.2 interop cases

Over `-dtls1_2`, with the HelloVerifyRequest cookie and a Low peer that offers both versions, it covers:

- `0xC02B` with X25519
- `0xC02C` with P-256
- `0xCCA9` with X25519
- `0xC02B` with P-256 and a 300-byte `Config.mtu`

That `Config.mtu` fragments the certificate flight.

##### DTLS 1.3 interop cases

Over `-dtls1_3` it covers:

- `0x1301` with X25519
- `0x1302` with P-256
- `0x1303` with X25519
- `0x1301` with P-256 at a 300-byte `Config.mtu`
- a HelloRetryRequest for a key share
- `-dtls` choosing DTLS 1.3

Each DTLS 1.3 case also runs a KeyUpdate that asks the peer to update and sends data after it.

##### DTLS interop dropped handshake datagrams

One case per version at a 300-byte `Config.mtu` drops handshake datagrams the Low peer sends.

| Version | Drops |
| --- | --- |
| DTLS 1.2 | The third and the fourth. |
| DTLS 1.3 | The second, which from the Low server is the ServerHello, so `s_client` meets the encrypted part of the flight before the retransmitted ServerHello. |

##### DTLS interop OpenSSL 4.1.0 blocking behaviour

The Low server sends a line before it reads one, because of how `openssl s_client` 4.1.0 behaves in blocking mode after a DTLS 1.3 handshake in which the ServerHello arrives after the encrypted records that follow it.

That client finishes the handshake inside `SSL_read` and waits there for server data. It reads no standard input until some arrives. `openssl s_server -num_tickets 0` stalls the same client with the same datagram order, and an `openssl s_server` that sends its default session tickets does not.

##### DTLS interop case checks

Each case requires the fingerprint check, the negotiated version, suite and profile, application data and the 60 bytes of `EXTRACTOR-dtls_srtp` keying material to match.

##### DTLS interop downgrade cases

Two more cases strip `supported_versions` and the key share from the ClientHello between the peers.

- The Low client fails on the downgrade sentinel of `openssl s_server`.
- `openssl s_client` reports an inappropriate fallback against the Low server.

##### DTLS interop skips

- The DTLS 1.3 cases skip when `openssl` has no `-dtls1_3`.
- The test skips when `openssl` or `timeout` is not installed.

#### RTP

`std/rtp` parses a datagram as exactly one RTP packet or one compound RTCP packet.

The payload and extension values share the datagram's bytes.

##### RTP packet parse errors

`parse` fails with `Malformed` when:

- the version is not 2
- the contributing sources or the extension block run past the end
- the padding count is 0 or larger than the payload

##### RTP extension parsing

| Case | Result of `parse` |
| --- | --- |
| A one-byte extension element with identifier 15 | Ends the block. |
| Zero bytes between elements | Skipped. |
| An extension block of another profile | One `Extension` with identifier 0 that holds the whole block. |

##### RTP packet marshalling

`marshal` writes the one-byte form when every element has an identifier of 1 to 14 and 1 to 16 bytes, and the two-byte form otherwise. It does so unless `extension_profile` names one. It pads the block to whole words.

It fails with `Invalid` for more than 15 contributing sources or a field wider than its wire field. Padding is written as zero bytes with the count in the last.

##### RTCP parsing

An RTCP packet whose type `parse_rtcp` reads becomes its struct:

| Packet | Parsed as |
| --- | --- |
| Sender and receiver reports | Their struct, with their report blocks and the signed 24-bit cumulative loss. |
| SDES | Chunks of items. |
| Goodbyes | Their struct, with an optional reason. |
| Generic NACK (transport feedback format 1) | Expanded into the list of lost sequence numbers. |
| Picture Loss Indication (payload feedback format 1 with no control information) | Its struct. |
| Other feedback formats, REMB and transport-wide congestion control among them | Keep their control information unparsed in `FeedbackMessage`. |

Other packet types become `RawPacket`.

##### RTCP marshalling

`marshal_rtcp` packs a NACK's sequence numbers into packet identifiers with a 16-bit bitmask of the next 16. It ends each SDES chunk with a zero byte padded to a word. It never adds padding.

##### RTP packet classification

`classify` follows RFC 7983 and RFC 5761.

| First byte | Classified as |
| --- | --- |
| 0 to 3 | STUN |
| 20 to 63 | DTLS |
| 128 to 191 | RTCP when the second byte is 192 to 223, and RTP otherwise. |

#### SRTP

`std/srtp` implements RFC 3711 and RFC 7714.

##### SRTP profiles

| Standard | Cipher | Tag |
| --- | --- | --- |
| RFC 3711 | AES-128 counter mode | HMAC-SHA1 tags of 80 or 32 bits |
| RFC 7714 | AES-128-GCM and AES-256-GCM | A 128-bit tag |

MKI is not supported. SRTCP always carries an 80-bit tag in the counter mode profiles.

##### SRTP key derivation

Session keys come from the AES counter mode key derivation with a key derivation rate of 0, for the GCM profiles too, with a 12-byte master salt.

##### SRTP context state

A `Context` serves one direction. Per synchronization source, it keeps the highest index seen and a 64-packet replay window, for RTP and RTCP apart.

##### SRTP index estimation

The RTP index is the 32-bit rollover counter above the sequence number. It is estimated from the highest index as RFC 3711 appendix A says. The sender estimates it the same way, so a retransmission within half the sequence space of the newest packet keeps its index.

##### SRTP index exhaustion

| Function | Fails with `Exhausted` once |
| --- | --- |
| `protect` | The rollover counter would pass 2^32 - 1. |
| `protect_rtcp` | The 31-bit SRTCP index would pass 2^31 - 1. That index starts at 0 for each source. |

##### SRTP unprotect checks

`unprotect` and `unprotect_rtcp` check the replay window, then the tag, and only then decrypt and move the window, so a forged or replayed packet changes nothing. A packet more than 64 behind the newest of its source, or already received, fails with `Replay`.

`unprotect_rtcp` reads the E bit and returns an unencrypted SRTCP packet after checking its tag.

##### SRTP keys from DTLS

`from_dtls` splits the keying material that `export_keying_material(c, "EXTRACTOR-dtls_srtp", keying_material_size(profile))` returns into the client key, the server key, the client salt and the server salt (RFC 5764 section 4.2). The DTLS client sends with the client keys.

The profile numbers are those of `use_srtp`, so `dtls.srtp_profile` passes straight to `from_dtls`.

##### SRTP vector tests

`std/srtp/vectors_test.low` checks:

- the key derivation and keystream of RFC 3711 appendix B
- the RTP and RTCP vectors of RFC 7714 sections 16 and 17
- packets that libsrtp sealed for all four profiles across the rollover from sequence 65535 to 0, in both directions

##### SRTP libsrtp interoperability test

`std/srtp/interop_test_linux.low` runs `testdata/srtp/gst_peer.py`, which drives GStreamer's `srtpenc` and `srtpdec` (libsrtp). For each profile it opens twelve packets from sequence 65530 and a sender report that libsrtp protected, and libsrtp must open every packet and the report that `std/srtp` protects.

It skips when `python3`, `timeout` or GStreamer's srtp elements for Python are not installed.

#### SCTP

`std/sctp` is a state machine like `std/ice` and `std/dtls`. Every function takes the time as a count of milliseconds.

##### SCTP association queueing

The association queues packets for `poll_transmit`. The same code runs over a DTLS connection and over an in-memory network.

##### SCTP packet format and checksum

Packets carry the common header of RFC 4960 with a CRC-32c checksum. The checksum is written little-endian and checked before anything else. A packet with a wrong checksum, a wrong port or a wrong verification tag is dropped without a reply. Chunks are padded to four bytes. Parameters and chunks that do not parse end the packet.

##### SCTP handshake

The handshake is the four-way exchange. The INIT and the COOKIE ECHO are retransmitted after `rto_initial`, doubling to `rto_max`. The handshake fails after 8 transmissions.

Both sides announce RE-CONFIG and FORWARD-TSN support in the supported-extensions and forward-TSN-supported parameters.

##### SCTP state cookie

The server keeps no state between INIT and COOKIE ECHO: the state cookie holds these, under an HMAC-SHA-256 with a random per-association key:

- both tags
- both first TSNs
- the peer's window and stream count and the extensions it announced

A cookie whose MAC does not match is ignored.

##### SCTP message fragmentation

A message is cut into DATA chunks of at most `Config.mtu` minus the 12-byte header and 16-byte chunk header, rounded down to a multiple of four. `Config.max_message_size` bounds the whole message. `send` fails with `TooLarge` above it.

The TSN is a 32-bit serial number kept as a 64-bit count by comparison with the cumulative TSN, so a stream of any length survives wrap-around.

##### SCTP receiver

The receiver reassembles fragments by TSN.

| Message | Delivered |
| --- | --- |
| Ordered | In stream sequence order. |
| Unordered | As soon as its fragments are complete. |

The receiver buffers at most `Config.window` bytes and discards DATA beyond it. Its SACK advertises the room left.

##### SCTP SACK contents and timing

A SACK carries:

- the cumulative TSN
- the gap blocks within 65535 of it
- up to 16 duplicate TSNs

It is sent for every second packet, at once when a gap or a duplicate exists, and otherwise after `sack_delay`.

##### SCTP congestion window

The sender keeps the congestion window at 4 packets at the start.

| Phase | Window growth |
| --- | --- |
| Slow start | By the acknowledged bytes. |
| Congestion avoidance | By one packet per window. |

It bounds the flight by the smaller of the congestion window and the peer's window (RFC 4960 section 7).

##### SCTP loss recovery

| Event | Sender action |
| --- | --- |
| The third SACK that reports a gap before a chunk | Retransmits it at once. It halves the window once per window of data. |
| A timeout | Retransmits everything outstanding, sets the window to one packet and doubles the timeout. |

##### SCTP round-trip time and timeout

The round-trip time uses chunks that were sent once, and the timeout stays within `rto_min` and `rto_max`.

##### SCTP partial reliability

`Delivery` states whether a message is ordered and its limits, `max_retransmits` and `lifetime`.

When the peer announced FORWARD-TSN:

- a message past a limit is abandoned
- the sender advances its peer's cumulative TSN with a FORWARD-TSN chunk that names the abandoned ordered streams and sequence numbers
- the receiver drops the fragments it holds below it and delivers what follows

Without the extension every message is reliable.

##### SCTP stream reset request

`reset_stream` queues the stream and sends an outgoing SSN reset request (RFC 6525) once everything sent before it has been sent, so the peer sees the reset after all earlier data.

The peer answers with the result, resets its inbound sequence number for the stream and reports `StreamReset`. The sender restarts its own sequence numbers at 0 when the answer arrives. A request is retransmitted after the retransmission timeout.

##### SCTP stream sequence numbers during a reset

A stream sequence number is assigned when the first fragment of a message is sent. A message queued for a stream with a request in flight waits for the answer, so it carries the number 0 the peer expects.

##### SCTP incoming stream reset

An incoming reset request from the peer makes the association reset the same streams in its own outgoing direction. A request with an unexpected sequence number is answered with the result the RFC defines for it.

##### SCTP heartbeat

A heartbeat goes out every `heartbeat_interval` milliseconds once the answer to the last one arrived. A heartbeat that is still unanswered when the next is due counts as a timeout.

##### SCTP shutdown sequence

`shutdown`:

1. stops accepting messages
2. sends what is queued
3. sends SHUTDOWN

It reports `Closed` when SHUTDOWN ACK arrives and it has sent SHUTDOWN COMPLETE (RFC 9260 section 9.2).

The cumulative TSN in a SHUTDOWN acknowledges data as a SACK's does, so the receiver answers with SHUTDOWN ACK once what it sent is acknowledged. The receiver reports `Closed` when SHUTDOWN COMPLETE arrives.

##### SCTP shutdowns that cross

A side that sent SHUTDOWN answers each packet with DATA with another. If it receives SHUTDOWN in return, because both sides shut down at once, it answers with SHUTDOWN ACK and closes on the peer's SHUTDOWN ACK.

##### SCTP shutdown retransmission

SHUTDOWN and SHUTDOWN ACK are retransmitted on the retransmission timeout until `max_retransmissions` consecutive timeouts fail the association.

A closed association answers a SHUTDOWN ACK that carries its tag with a SHUTDOWN COMPLETE with the T bit set, so the peer closes when the first SHUTDOWN COMPLETE was lost.

##### SCTP abort and failure

| Cause | Reports |
| --- | --- |
| `abort`, which sends ABORT | `Failed` |
| A peer ABORT | `Failed` |
| A handshake that does not finish | `Failed` |
| `max_retransmissions` consecutive timeouts | `Failed` |

##### SCTP session threading and queues

`std/sctp/session` runs an `Assoc` in one green thread that waits on commands, packets and the next timeout together. A `Transport` has no socket, so the same code runs over `std/dtls/session`'s `send` and `data`.

| Queue | Holds | When it does not fit |
| --- | --- | --- |
| `Handle.messages` | 256 messages | The session stops reading packets while they wait, which closes the receiver window. |
| `Handle.events` | 64 | Drops what does not fit. |

##### SCTP association test

`std/sctp/assoc_test.low` drives two associations over a simulated path with loss, reordering and duplication. It checks:

- the handshake and a retransmitted INIT
- fragmentation
- ordered, unordered and partially reliable delivery
- retransmission on a lossy path
- congestion window growth
- the receiver window
- stream reset
- heartbeat
- shutdown
- shutdowns that cross
- shutdown on a lossy path and through the cumulative TSN of a SHUTDOWN
- abort
- that a packet with a flipped bit is dropped

#### Data channels

`std/datachannel` runs DCEP and the channels of one SCTP association in one green thread, which waits on commands, the association's messages and its events together. The association is a `std/sctp/session` over the `Transport` the caller gives, so the same code runs over `std/dtls/session` and over an in-memory path.

##### Data channel stream identifiers

`Config.client` of the association names the DTLS role (RFC 8832 section 6).

| DTLS role | Opens channels on |
| --- | --- |
| Client | Even stream identifiers, each taking the lowest unused one |
| Server | Odd ones, each taking the lowest unused one |

A channel cannot use identifier 65535.

##### Rejected DATA_CHANNEL_OPEN

A DATA_CHANNEL_OPEN that arrives on a stream of the receiver's own parity, on a stream that has a channel, or that does not parse resets that stream and creates nothing.

##### Data channel options

`Options` map onto the OPEN message.

| Option | Effect |
| --- | --- |
| `max_lifetime` | Selects the timed type with the lifetime in milliseconds. |
| `max_retransmits` | Selects the retransmit type with the count. |
| Neither | Selects the reliable type. |
| An unordered channel | Sets the high bit of the type. |
| Both limits at once | Fails with `Invalid`. |

##### Data channel OPEN and acknowledgement

The OPEN is sent ordered and reliable with PPID 50. The channel is usable at once, because data may follow the OPEN before the acknowledgement arrives.

The receiver answers with DATA_CHANNEL_ACK and puts the channel on `Handle.incoming`. When 64 channels wait there, the new channel is closed. A received ACK is recognised by its first byte, because `pion/datachannel` pads it to four bytes.

##### Negotiated data channels

A negotiated channel sends and expects no OPEN. Both sides open it with the same `id`. This fails with `Invalid` outside 0 to 65534 or when a channel already uses it.

##### Data channel message types

| Message | PPID |
| --- | --- |
| Text | 51 |
| Binary | 53 |

An empty message has no SCTP form, so it is sent as one zero byte with PPID 56 for text or 57 for binary. It is received as an empty `Message`.

##### Data channel messages before OPEN

A message on a stream whose OPEN has not arrived waits for it. An unordered channel can produce this case. At most 64 messages wait per stream.

##### Data channel delivery

Each message is sent with the channel's `sctp.Delivery`. So a channel with `ordered: false` or a limit uses unordered delivery and partial reliability. These apply only when the peer announced FORWARD-TSN.

##### Closing a data channel

1. `close_channel` resets the stream once the messages queued before it were sent.
2. The peer sees the reset, closes its channel and resets its own outgoing direction.
3. The channel ends, closing `Channel.messages`, when that answer arrives.

Sends on a closing channel fail with `NotConnected`. The identifier can be used again, with sequence numbers restarting at 0. An association that closes or fails ends every channel.

##### Data channel receive queue

`Channel.messages` holds 256 messages. When a channel's queue is full the router keeps up to 256 messages in arrival order and stops reading the association, which closes the receive window, so a channel that is not read delays the others.

##### Data channel fixture test

`testdata/run/std_datachannel.low` runs two peers over an in-memory path with no loss and with every third packet lost. It checks:

- identifiers
- labels
- reliability parameters
- text and binary messages up to 20000 bytes
- empty messages
- negotiated channels
- closing and identifier reuse

##### Data channel interoperability with pion

The module was also run against `pion/datachannel` over UDP in both roles, with:

- reliable and partially reliable unordered channels
- text and binary messages up to 40000 bytes and empty messages in both directions
- a channel closed by the Low side

#### SDP

`std/sdp` reads a description line by line, with CRLF or LF line ends.

##### SDP parse errors

It fails with `Line` at the first line that:

- is empty
- has no `type=` prefix
- has a type RFC 8866 does not define
- appears twice where one is allowed
- does not parse

##### SDP required lines

| Line | Status |
| --- | --- |
| `v=0` | Required. |
| `o=` | Required. |
| `s=` | Required. |
| `t=` | Not required, and `format` writes `t=0 0` for a session without one. |

##### SDP line order

The lines of a section may come in any order, and a `m=` line starts a media description. `format` writes the order of RFC 8866 section 5, so the text of a description in that order, which browsers and pion send, formats back to the same bytes.

##### SDP attribute syntax

An `Attribute` is a name and an optional value, split at the first colon.

| Attribute | Value |
| --- | --- |
| `a=rtcp-mux` | No value. |
| `a=msid-semantic: WMS` | The value ` WMS`. |

`get`, `has` and `all` look an attribute up.

##### SDP typed accessors

The typed accessors fail with `Attribute` naming the attribute whose value does not parse. They return `none` or an empty list when the attribute is absent.

##### SDP session and media description lookup

`ice_ufrag`, `ice_pwd`, `ice_options`, `fingerprints`, `setup`, `direction` and `end_of_candidates` read the media description and then the session, because JSEP allows each on either. Chrome and pion put the credentials on every media description, and Firefox puts the fingerprint on the session.

##### SDP fingerprints and candidates

| Function | Returns the attribute values for |
| --- | --- |
| `fingerprints` | `std/dtls`'s `parse_fingerprint` |
| `candidates` | `std/ice`'s `parse` |

So a candidate `std/ice` does not read, such as a TCP candidate, is the caller's to skip.

##### SDP data channel description

`is_data_channel` is an `application` media description over `DTLS/SCTP` with the format `webrtc-datachannel` (RFC 8841). The media description's `sctp_port` and `max_message_size` are attributes.

##### SDP captured descriptions

`testdata/sdp/` holds offers and an answer captured from three sources.

| Source | Files |
| --- | --- |
| First: Chromium 158 | `chromium_offer.sdp`, with host candidates, and `chromium_answer.sdp`, its answer to the pion offer. |
| Second: Firefox 160 | `firefox_offer.sdp`, with mDNS and TCP candidates, `a=extmap:2/recvonly` and an `a=fmtp:101 0-15`. |
| Third: pion/webrtc 4.2.27 | `pion_offer.sdp`. |

Each comes from a data channel, an audio and a video transceiver. Documentation addresses stand in for the interface addresses, and nothing else is edited.

##### SDP captured description tests

`std/sdp/sdp_test.low` requires each to format back to its own bytes. `std/sdp/webrtc_test.low` reads the WebRTC attributes of each.

#### Peer connection

`std/webrtc` is one green thread that owns the description state, the SRTP contexts and the track table. Each layer below it runs in its own session. Every call is a command to that thread, so no two green threads touch the SRTP state.

##### Peer connection layers

| Layer | Behaviour |
| --- | --- |
| The ICE session | Owns the sockets. |
| The DTLS session | Runs over the ICE session's `send` and the datagrams the peer gives it. |
| The data channel peer | Runs over the DTLS session's `send` and `data`. |

##### ICE sockets and agent role

The first `create_offer` or `create_answer` opens the ICE sockets, because the agent's role is fixed when it is made. The first offerer is controlling and the first answerer is controlled. Later exchanges keep the roles.

##### Host candidates in descriptions

A description holds the host candidates gathered when it was made, in its first section that is not rejected. Every local candidate, those included, also arrives as `Event.Candidate` for trickle.

##### Remote candidates skipped

`add_remote_candidate` and the description's own candidates skip:

- TCP candidates
- host names outside `.local`
- component 2

A candidate value seen before is ignored.

##### mDNS host candidates from browsers

Browsers send a `.local` candidate for their host addresses. `mdns.resolve` resolves it in a green thread of its own, and the candidate is added with the address it gets. It is dropped when no answer comes within 5 seconds.

`end_of_remote_candidates` and `a=end-of-candidates` reach ICE once no name is still being resolved, so ICE does not fail a pair list that is still growing.

##### Peer mDNS responder

With `Config.mdns` the peer opens an `mdns` responder with the ICE sockets and names each host address with `mdns.random_name`, so its descriptions and `Event.Candidate` carry the name. With `Config.mdns` the related address of a server-reflexive or relayed candidate is `0.0.0.0` port 0.

##### TURN servers and relay only

`Config.ice` carries `turn_servers` and `relay_only` to the agent, so a peer gathers relayed candidates, or uses them alone.

##### Peer connection bundled transport

Every section shares one transport (`a=group:BUNDLE`, RFC 9143, as browsers send with `max-bundle`). Every section multiplexes RTCP with RTP (`a=rtcp-mux`). The ICE credentials, the fingerprint and `setup` are read from the first section that is not rejected.

##### DTLS role negotiation

Every offer says `actpass`. In the first exchange the answerer is the DTLS client unless the offer said `active`, as RFC 8842 recommends. In the first exchange the offerer takes the role opposite the answer's `active` or `passive`.

One flag then drives the DTLS role, the SCTP role that picks even or odd stream identifiers and the SRTP key direction. Later exchanges keep the roles whatever their `setup` says. An answer states the role the peer has.

##### Peer connection certificate fingerprints

The DTLS connection accepts only a certificate whose digest is one of the remote description's fingerprints. DTLS refuses renegotiation, so a later description whose fingerprints are not among them is refused with `Invalid`.

##### Peer connection datagram routing

The peer splits the datagrams of known remote addresses that are not STUN by their first byte (RFC 7983, `rtp.classify`).

| Datagram | Goes to |
| --- | --- |
| DTLS | The DTLS session through a channel of 256 made when the peer opens, so a ClientHello that arrives before the DTLS session exists waits. |
| RTP and RTCP | SRTP. |

##### DTLS session start and SRTP keys

The DTLS session starts when ICE selects a pair. When the handshake finishes and `use_srtp` agreed a profile, the peer derives the SRTP contexts from `EXTRACTOR-dtls_srtp` (RFC 5764). The offered profiles are `Config.srtp_profiles`, AES-128-GCM first.

##### SCTP in the peer connection

SCTP runs when both descriptions have a data channel section (RFC 8841).

| Setting | Value |
| --- | --- |
| Port | The remote `a=sctp-port`, 5000 when absent. |
| Packet size | `Config.mtu` less the 37 bytes a DTLS record with an AES-GCM suite adds. |
| Message bound | The remote `a=max-message-size`, which is 65536 when absent and no bound when 0. |

##### Peer connection state

| State | When |
| --- | --- |
| `Connected` | Once DTLS is up and, with a data channel section, SCTP too. |
| `Disconnected` | Follows ICE consent. |
| `Failed` | Follows a failure of any layer. |

##### Media section order

The peer keeps its media sections in m-line order. They are the W3C transceivers that `transceivers` reports. A section keeps its `mid` and its place in every later description. A rejected or stopped section stays at port 0 and is not reused.

##### Peer connection offer section order

An offer lists:

1. the negotiated sections
2. a new section for each local track without one
3. a data channel section when `Config.data_channels` is set and no exchange has had one

A data channel section is offered at most once.

A description whose sections do not start with the negotiated `mid`s in their order is refused with `Invalid`.

##### Local track sections in an offer

A local track's section offers its codecs, the direction it asks for, `a=msid` and `a=ssrc` with the track's synchronization source. The direction is `sendrecv` until `set_direction` changes it or `remove_track` takes the send part away. A section without a track does not offer to send.

##### Answering a media section

An answer gives each new audio or video section the first local track of its kind without a section that shares one of its codecs. It keeps the offered codecs that share a name and clock rate (and channel count when both state it) with the track's, at the offered payload types, which `codecs` then returns. A section without a track is answered with every offered codec.

##### Media section send and receive

| A section | When |
| --- | --- |
| Sends | It has a track, its direction sends and the remote receives. |
| Receives | Its direction receives and the remote sends. |

A section a remote offer made is therefore `recvonly` until `add_track` gives it a track.

##### Peer connection rejected sections

These sections are rejected with port 0:

- sections of another type
- a second data channel section
- sections over another protocol

##### Remote track matching

A section the remote sends on has a `RemoteTrack`.

| Section | RTP matching |
| --- | --- |
| It has an `a=ssrc` | The peer matches its RTP by the synchronization source of the section's first `a=ssrc`. |
| It has none | The peer matches by payload type and binds the first source that arrives. |

##### Remote track changes

A later description that names another source on the section closes the old track's `packets` and announces a new `RemoteTrack`. SRTP keeps a rollover counter and replay window per source, so the new source needs no new keys. A description in which the remote stops sending, or that rejects or stops the section, closes the track's `packets`.

##### Data channel section in a later exchange

A data channel section accepted in a later exchange starts SCTP over the DTLS connection that is up, and `create_data_channel` waits for that association to connect.

##### Stopping a data channel section

A stopped or rejected data channel section aborts an association that has not connected. It shuts a connected one down. Each side sends what its channels queued, the channels deliver it, and they end on both sides when the association reports `Closed`.

A shutdown that has not finished 5 seconds later aborts the association. An abort that reaches the side that stopped the section, or that has an offer outstanding, does not fail the peer.

##### Sending RTP and RTCP

| Function | Does |
| --- | --- |
| `send_rtp` | Replaces the packet's synchronization source with the track's and protects it. |
| `send_rtcp` | Sends one compound SRTCP packet. |

`send_rtp` fails with `NotConnected` for a removed track and for one whose section is stopped, and neither checks the negotiated direction.

##### Receiving RTP and RTCP

The peer unprotects and parses received RTP and puts it on the track's `packets`. RTCP goes to `Handle.rtcp`. Both channels hold 256 and drop what does not fit.

##### Media left to the caller

Codecs, jitter buffers, retransmission and congestion control are the caller's.

##### Peer connection signalling state

The signalling state (`signaling`) is `Stable` or `HaveLocalOffer`.

| Call | Effect |
| --- | --- |
| `create_offer` | Moves to `HaveLocalOffer`. |
| `apply_answer` and `rollback` | Move back. |
| `create_answer` | Applies the remote offer and makes the answer at once. It needs `Stable`. |

Provisional answers and sections that are not bundled are not supported.

##### Offer glare and rollback

When offers cross (glare), `create_answer` fails with `WrongState` on both sides. The side that yields calls `rollback`. `rollback` takes the `mid`s back from the sections its offer added and drops a data channel section it added. The yielding side answers, and offers its own changes once stable, as the W3C perfect negotiation pattern does.

##### Peer connection certificate

The certificate is a new P-256 identity made by `open`.

##### ICE restart offer

`restart_ice` makes the next offer restart ICE (RFC 8445 section 9). The agent takes new credentials, forgets the remote credentials, candidates and pairs and gathers again. The peer forgets the candidate values it has seen, because the regathered candidates repeat them.

##### ICE restart from new remote credentials

An offer whose credentials differ from the last remote ones makes the answerer restart as well. An answer with new credentials replaces the remote ones and the checklist.

##### Selected pair during an ICE restart

The selected pair carries DTLS, SRTP and SCTP until the new checks nominate a pair, which `Event.Selected` reports. Until then each side answers consent checks under the other's old credentials with 401. This is harmless while that takes less than the 10 seconds after which consent turns `Disconnected`.

##### Rollback after an ICE restart

A `rollback` keeps the new credentials, because the agent cannot take the old ones back. The next answer therefore carries them, and the remote restarts its checklist as it does for any new credentials.

##### Relayed pair through an ICE restart

A relayed selected pair keeps its TURN allocation through the restart.

##### Relayed ICE restart test

`std/webrtc/relay_test_linux.low` runs the pion/turn server of `std/ice/session/relay_test_linux.low`. It restarts ICE from a `relay_only` peer through that server over UDP and TCP, with data channel messages both ways and audio during the restart and after the new `Event.Selected`. It skips as that test does.

##### Peer connection loopback test

`testdata/run/std_webrtc.low` runs two peers in one process over loopback sockets. The offer and the answer go through `sdp.format` and `sdp.parse`. A data channel is opened each way with text and a 20000-byte binary message. Each side's audio track carries SRTP to the other across the sequence number wrap, with an RTCP picture loss indication back.

##### Loopback test renegotiations and checks

Later exchanges run with the data channel open throughout:

1. They add a video track whose packets arrive under its new source.
2. They turn the first answerer's audio to `recvonly` in an offer it makes, which ends that track at the first offerer while the DTLS roles stay.
3. They remove the video track, which ends it at the other side and stops its sends.
4. They cross two offers, one of them an ICE restart. The restarting side rolls back and answers the other's offer, which stops the video section.
5. They restart ICE from the first answerer.
6. Last, they stop the data channel section, which delivers a 60000-byte message the answerer sent just before answering and then ends the channel within 2 seconds, while audio still flows.

During that restart, before the answer, RTP and data channel messages reach the restarting side. Afterwards both sides report a new `Event.Selected` and media and messages still flow. Neither side ever reports `Disconnected` or `Failed`. Each description's `mid`s, kinds and directions are printed and compared.

##### Pion interoperability test

`std/webrtc/interop_test_linux.low` builds `testdata/webrtc/pion`, a pion/webrtc 4.2.27 peer that echoes data channel messages and RTP. It runs that peer with Low offering and with pion offering.

- Each side opens a data channel.
- Text and a 30000-byte binary message are echoed.
- Five SRTP packets come back on pion's track.
- Then Low adds a video track and offers it, and five VP8 packets on the new source come back.
- Pion adds a video track and offers it, and its packets arrive on the `RemoteTrack` that announces it.
- The side that made the first offer restarts ICE, with a data channel echo while Low's restart offer awaits its answer, or, because pion stops sending until its own restart finds a pair, as soon as Low has answered pion's, new credentials in both descriptions, a new `Event.Selected`, and echoes of text and RTP after it.

##### Pion interoperability third run

A third run has pion offer its audio track without a data channel section, so the first answer has none.

1. Low offers one, which starts SCTP over the DTLS connection that is up and carries a channel echo.
2. Low stops it, which ends the channel within 3 seconds through pion's answer to the SHUTDOWN.

##### Pion interoperability test skips

The test skips when `go` or `timeout` is not installed or the pion modules cannot be downloaded.

##### Peer connection browser runs

The module was also run by hand against headless Chromium 158 and Firefox 160 in both offer directions.

- The runs used the browsers' `.local` host candidates and, with `Config.mdns`, the peer's own.
- Each run had a data channel each way with text and a 60000-byte binary echo, and 50 SRTP packets that each browser counted in `getStats`.
- Every run used DTLS 1.3 and `TLS_AES_128_GCM_SHA256`, with AES-128-GCM or AES-128 counter mode SRTP as the browser chose.

##### Peer connection browser run with unresolved names

In a run where the peer left the browser's names unresolved, both browsers still connected and reported a remote `host` candidate. They had therefore resolved the peer's names through its responder.

##### Peer connection browser renegotiation run

A by-hand renegotiation run against both browsers had the browser offer first.

1. In a second offer the browser restarted ICE and turned its video section to `recvonly`.
2. In a third offer the peer added a video track to that section and restarted ICE.

The data channel carried the signalling notes while each restart was outstanding and echoed after it. `Event.Selected` reported the new pair. Each browser reported its selected pair under the newest credentials and counted 50 audio and 50 video packets in `getStats`.

##### Peer connection mDNS test

`std/webrtc/mdns_test_linux.low` connects two peers with `Config.mdns` over loopback and requires each to select a remote `host` candidate, which only a resolved name gives. It skips when the host cannot share port 5353 or multicast does not come back to it.

### Halo2 security

`std/zk/halo2` is a Halo2 proof system as `halo2_proofs` 0.3.0 defines it.

#### Halo2 soundness and zero knowledge

Because the system is as `halo2_proofs` 0.3.0 defines it, a proof is sound under the discrete logarithm assumption on Vesta and Pallas (about 127 bits of group security) and the random oracle model for Blake2b, and zero knowledge for an honest prover that draws its randomness from `crypto.random`.

#### Halo2 parameters

There is no trusted setup.

| Function | Does |
| --- | --- |
| `params` | Derives every generator by hash to curve. |
| `keygen` | Is deterministic. |
| `read_params` | Reads the `2^k` generators from a file without recomputing them. |

Parameters from a source that is not trusted must therefore be compared with `params(c, k)`. With other generators the argument binds nothing.

#### Halo2 hidden values

What is hidden: the advice columns, the permutation and lookup products, and the quotient pieces.

| Hidden value | Hidden by |
| --- | --- |
| The advice columns | Their commitments carry a random blind, and the advice columns' last `blinding_factors + 1` rows hold random values. |
| The permutation and lookup products | They end in random rows. |
| The quotient pieces | They carry blinds and are opened together with a random polynomial. |

Instance columns and fixed columns are public.

`blinding_factors` is the larger of 3 and the largest number of queries of one advice column, plus 2.

The evaluations a proof reveals therefore leave the committed polynomials uniformly random. This is the argument of `halo2_proofs`. No simulator is built or checked here.

#### Halo2 timing and memory

What is not constant time:

- `std/crypto/pasta` branches on points and scalar bits in `add`, `double`, `mul` and `msm` and in `field_sqrt`. `prove` commits to and multiplies by the witness through them. It sorts the compressed lookup input by value and compares witness values, so its running time depends on the private witness, and it must not run where a local timing observer is in scope.

`verify` handles public data only. `prove` does not wipe the witness or the blinds from memory.

#### Differences from halo2_proofs

| Function | Here | In `halo2_proofs` |
| --- | --- | --- |
| `prove` | Checks the gates, the equalities and the lookups. It fails with `Unsatisfied`. | It makes a proof that does not verify. |
| `verify` | It rejects trailing bytes with `Malformed`. | It leaves them unread. |

### Derived serialization

`std/serde` defines a `Writer` and a `Reader`. They are structs of closures through which values are written and read.

For every concrete type the compiler derives a function that writes a value to a `Writer`, a function that reads it from a `Reader`, and a `Schema` describing it.

#### Derived serialization of primitives, collections and pointers

| Type | Maps to |
| --- | --- |
| Booleans, integers, floats and strings | Their primitives. |
| `u8[]` | Binary. |
| Slices | Sequences. |
| Maps | Maps with keys written as text. |
| Optionals | Null or the value. |
| Pointers | Their target, a shared pointer in full each time. A pointer reached again while its target is being written is a cycle and panics. |

#### Derived struct serialization

A struct is a map from field names to values. Every field takes part, private ones included.

A reader opens it with `begin_map` and steps through it with `next_field`. A format may spell `next_field` differently from `next_key`, the step through the entries of a map.

Reading skips unknown fields, uses a field's default when it is missing and treats a missing optional field as `none`. Otherwise it fails with `Missing`.

#### Derived enum serialization

| Enum variant | Written as |
| --- | --- |
| Without fields | Its name. |
| With labelled fields | A single-key map from its name to a map of its fields. |
| With positional fields | It maps its name to a sequence. |

#### Serde schemas

A schema records each type's kind, name, fields and variants.

| Schema entry | Records |
| --- | --- |
| Each field | Whether the field is required and its tag. |
| Each variant | Its index, which is the position when the enum declares no indexes. |

A struct or enum reached again through a pointer while it is being described becomes a `Ref` to its name.

#### Serde reader positions

A `Reader` reports `at` in an `Error` as its `position`. The unit of `position` is the format's:

| Unit of `position` | Format |
| --- | --- |
| A byte offset | JSON |
| A line | YAML |
| The count of values read | CBOR, MessagePack and TOML |
| A column index | `std/orm` |
| `0` | XML |

#### Serde trees

CBOR, MessagePack, TOML, YAML, `std/avro` and `std/template` build a `serde.Tree` from a value and read a value from one. The variant shape, the text of a map key and the range of a `u64` are therefore defined once, in `std/serde`.

#### Modules and struct fields

`std/json`, `std/orm` and `std/web` are built on these. `T.field`, where `T` is a struct type, is a `serde.Field[T, F]` that holds the field's name and index. A function can therefore take a field of a given struct with a given type.

### Data

#### JSON reader

`json.Reader` tokenizes UTF-8 bytes like .NET's `Utf8JsonReader`, strictly per RFC 8259, with a depth limit of 64. It returns strings without copying unless they contain escapes. Numbers are parsed as integers or correctly rounded floats.

#### JSON serialization

`json.serialize_with` and `json.deserialize_with` take `Options` for indentation, naming (`Unchanged`, `CamelCase`, `SnakeCase`, `PascalCase`), maximum depth and `omit_none`.

- Naming applies to struct field names and to variant names in both directions. It does not apply to the keys of a map. A variant name read back is matched in PascalCase.
- With `omit_none`, a field whose value is `none` is left out instead of written as `null`.
- Binary data is base64. Non-finite floats are written as `null`.

#### CBOR decoding

`std/cbor` decodes RFC 8949 items into `Value`s: every major type, indefinite-length strings, arrays and maps, tags, simple values, and half, single and double floats, which become `f64`. Indefinite-length items arrive as their definite equivalents. Text must be valid UTF-8 and map keys must be distinct.

`decode_first` returns the first item and its length so a caller can continue after it.

#### CBOR decoding errors

| Case | Fails with |
| --- | --- |
| Truncated input | `Malformed` |
| Reserved additional information | `Malformed` |
| A stray break | `Malformed` |
| Indefinite integers or tags | `Malformed` |
| Mixed or nested string chunks | `Malformed` |
| A two-byte simple value below 32 | `Malformed` |
| Data after the item | `Malformed` |
| A declared length that exceeds the remaining input (checked before anything is allocated) | `Malformed` |
| Nesting deeper than `Options.max_depth` | `TooLarge` |
| A string, array or map longer than `max_length` | `TooLarge` |

#### CBOR encoding

`encode` writes the deterministic encoding of §4.2.1:

- shortest heads
- definite lengths
- map entries sorted by the bytes of their encoded keys
- floats in the shortest of half, single or double that keeps the value, with NaN as `f97e00`

`diagnostic` writes the notation of §8.

#### CBOR serde binding

`serialize` and `deserialize` bind types through serde:

| Type | Bound as |
| --- | --- |
| `u8[]` | Byte strings |
| Structs and variants | Maps with text keys |
| `none` | Null |

Reading looks through tags and accepts integer map keys as their decimal text.

#### WebAuthn options

`std/webauthn` produces options JSON in the shape py_webauthn's `options_to_json` writes. Challenges, user ids and credential ids are base64url. `pubKeyCredParams` defaults to `supported_algorithms()` of the COSE parser (EdDSA, ES256 and RS256), and attestation is `none`.

#### WebAuthn registration verification

`verify_registration` takes the credential JSON a browser's `PublicKeyCredential` gives in base64url, with or without padding.

- It checks that `id` is the base64url of `rawId`, that `type` is `public-key`, that clientDataJSON has type `webauthn.create`, the expected challenge (compared in constant time) and the origin.
- It then decodes the attestation object and checks the RP ID hash, user presence, user verification when required, and attested credential data.
- It parses the COSE key, accepts only the `none` format with an empty statement, and rejects a backup state without backup eligibility.
- It returns the credential id, the COSE key bytes exactly as the authenticator wrote them, and the sign count.

#### WebAuthn authentication verification

`verify_authentication` makes the same client data and authenticator data checks for `webauthn.get`. It rejects a sign count that is not greater than the stored one unless both are zero.

It verifies the signature over authenticatorData followed by SHA-256 of clientDataJSON with the stored key:

| Algorithm | Signature |
| --- | --- |
| ES256 | DER ECDSA on P-256 |
| EdDSA | Ed25519 |
| RS256 | PKCS#1 v1.5 with SHA-256 |

#### WebAuthn errors

Each failure is its own `Error` variant.

#### ORM queries

`std/orm` builds queries from conditions over field references, such as `orm.filter(orm.from(users), orm.ge(User.age, 18))`, and type-checks them against the field. It renders them for the connection's `Dialect`, maps rows onto structs, stores nested values as JSON columns and creates tables from types with an identity key named `id`.

#### ORM generated keys

`std/orm` returns generated keys from inserts:

| Database | Through |
| --- | --- |
| PostgreSQL and MariaDB | `RETURNING` |
| SQL Server | `OUTPUT` |
| MySQL | `LAST_INSERT_ID()` |

#### SQL value representation

`sql.Value` holds:

| Value | Held as |
| --- | --- |
| Dates | Days since 1970-01-01 |
| Times | Microseconds since midnight |
| Timestamps | Microseconds since 1970-01-01 UTC |
| Infinite dates and timestamps | The `i64` maximum and minimum |
| UUIDs | 16 bytes |

#### SQL connections and transactions

A connection serves one green thread at a time. A call made while another is running fails with `Busy`.

`orm.transaction` rolls back on any error from its body. It fails with that error, or with the rollback's error when the rollback fails too.

#### PostgreSQL protocol and authentication

- `std/sql/postgres` authenticates with SCRAM-SHA-256, MD5 or cleartext.
- It uses the simple protocol for statements without parameters. Otherwise it uses the extended protocol with a 64-entry prepared statement cache.
- It streams `each` in batches of 512 rows, reads results as text decoded by type OID, and keeps a connection usable after a server error.
- `connect_tls` upgrades with SSLRequest and `std/tls`.

#### PostgreSQL parameter types

A `Null` or `Text` parameter is declared with OID 0, so the server infers its type. Every other `Value` is a typed parameter. The driver declares it with OID 0 as well, sends Parse, Describe statement and Sync for the unnamed statement, and reads the ParameterDescription.

| Typed parameter | Declared with |
| --- | --- |
| One that the server inferred as anything but `text` or `unknown` | The inferred type, so `Int(21)` binds into the `int4` of `twice($1)` and a `Float` into a `real`. |
| One the server could not infer (`SELECT $1`, both sides of `$1 < $2`) | Its own type (`Int` as `int8`, `Float` as `float8`, `Decimal` as `numeric`, and so on). |

The statement is parsed again with those OIDs. Bind sends every value in text format, so the server parses the text into whichever type it chose.

This round trip happens once per statement text and value types, on the call that first sees them. The later calls reuse the declared OIDs and the prepared statement cache. A statement without a typed parameter keeps its single round trip.

#### PostgreSQL cancellation and timeouts

- `cancel` sends a CancelRequest over a new connection once the request is sent, and keeps the connection.
- The call fails with `Cancelled` when the server stops it with SQLSTATE `57014`. `each` also stops between batches.
- A timeout sends a CancelRequest and closes the connection.

#### PostgreSQL server messages

Every field of an ErrorResponse fills the `Diagnostic`.

An ErrorResponse or NoticeResponse reaches `on_notice` as a `Message`. A NotificationResponse reaches it as a `Notification`. Both arrive in order with the rows, while a call reads the connection. A ParameterStatus after startup reaches `on_notice` as a `Setting`.

#### PostgreSQL statement streaming and describe

| Function | Behaviour |
| --- | --- |
| `stream` | Sends a statement without parameters through the simple protocol, so every result set of a string of statements arrives in order. |
| `stream_prepared` | Always sends Parse, Bind, Describe portal and Execute, with or without parameters, so the server refuses text holding more than one command with SQLSTATE `42601`. |
| `describe` | Sends Parse and Describe statement for the unnamed statement and reads the ParameterDescription and the RowDescription or NoData. It leaves the prepared statement cache as it is. |

#### PostgreSQL column metadata

| `Column` field | Taken from |
| --- | --- |
| `table` | The table OID of its RowDescription field |
| `number` | The attribute number of its RowDescription field, and `number` is negative for a system column |
| `precision`, `scale` and `length` | The RowDescription type modifier: `numeric(p, s)`, the length of `varchar` and `bpchar` (-1 without one) and the fractional digits of `time`, `timetz`, `timestamp`, `timestamptz` and `interval` (6 when undeclared) |

#### PostgreSQL command tags

A CommandComplete tag gives `Done`:

| Command | `Done` gets |
| --- | --- |
| SELECT, INSERT, UPDATE, DELETE, MERGE, COPY, FETCH and MOVE | A count |
| Every other command | `none` |

#### PostgreSQL server text

With `Options.server_text` every non-null value is `Text` of the server's own text, and the startup message leaves out `DateStyle` so the server's setting stands.

#### SQL Server login

`std/sql/sqlserver` speaks TDS 7.4 with SQL authentication, or on Windows with integrated authentication when `connect_with` gets `Config.integrated`. In that case LOGIN7 carries an SSPI Negotiate token for `MSSQLSvc/host:port`, and the server's SSPI tokens are answered until the login completes.

`Config.read_only` sets the read-only intent bit of the LOGIN7 type flags. LOGIN7 announces UTF-8 support.

#### SQL Server host names

| Host | Resolved as |
| --- | --- |
| `host\instance` with no port | Through the SQL Server Browser on UDP port 1434 |
| `.` and `(local)` | They name `localhost` |

#### SQL Server encryption

| Call | Prelogin offers |
| --- | --- |
| `connect` | No encryption |
| `connect_tls` | Full encryption |

`connect_tls` runs the `std/tls` handshake inside PRELOGIN packets and unwraps the packets the server keeps sending around TLS records.

#### SQL Server statements and transactions

Statements without parameters go as SQL batches. With parameters they go to `sp_executesql` as `@p1..@pN` declared as `bigint`, `float`, `bit`, `decimal(38, s)`, `nvarchar`, `varbinary`, `uniqueidentifier`, `date`, `time(7)`, `datetime2(7)` or `datetimeoffset(7)`, with NULL sent as `nvarchar(4000)`.

Every request carries the transaction descriptor. `begin`, `commit` and `rollback` use transaction manager requests.

#### SQL Server result decoding

Results decode every fixed, nullable, decimal, money, date and time, GUID, character, binary, long-value, text, image, XML, `sql_variant` and user-defined type. GUIDs become RFC 4122 byte order.

#### SQL Server text collations

`char`, `varchar` and `text` decode as UTF-8 when the collation's UTF-8 flag is set and otherwise through `std/charset` in the collation's code page.

| Collation | Code page picked by |
| --- | --- |
| SQL collation | Its sort id |
| Windows collation | Its LCID, by locale where a language spans two code pages and by language otherwise |

Text that is all ASCII is returned as it stands. Other text whose collation has no code page, or that the code page does not map, fails with `Conversion`.

#### SQL Server column metadata

A `Column` takes `precision`, `scale` and `length` from COLMETADATA.

| `Column` field | Holds |
| --- | --- |
| `length` | The declared size, in characters for `nchar` and `nvarchar` and in bytes for the rest, and -1 for `max`, `xml`, `text`, `ntext` and `image` |
| `scale` | Also the fractional-second digits of `time`, `datetime2` and `datetimeoffset` |

#### SQL Server server text

With `Options.server_text`, `time`, `datetime2` and `datetimeoffset` (and `sql_variant` holding them) are `Text` as `yyyy-mm-dd hh:mm:ss` with exactly `scale` fractional digits. A `datetimeoffset` is in its local time followed by ` +hh:mm`.

#### SQL Server row counts and result sets

Row counts sum the DONE and DONEINPROC counts that carry the count bit.

In `stream`, DONE and DONEINPROC end a result set with their count, or `none` without the count bit. DONEPROC ends none.

#### SQL Server streaming, cancellation and timeouts

- `each` streams rows as they arrive. Stopping early sends an ATTENTION and reads until the server acknowledges it.
- A server error below severity 20 leaves the connection usable.
- A per-connection watchdog sends an ATTENTION and closes the stream when one read waits longer than the timeout. The call fails with `Timeout`.
- `cancel` sends an ATTENTION once the request is sent. The call fails with `Cancelled` when the server acknowledges it.
- Finishing a response and sending an ATTENTION take one lock, so no ATTENTION follows a finished response.

#### SQL Server diagnostics and settings

Every ERROR and INFO token becomes a `Diagnostic` with the number as its code, the state, the class, the server, the procedure and the line.

| Class | `Diagnostic` kind |
| --- | --- |
| 10 and below | `Info` |
| 11 to 19 | `Failure` |
| 20 and above | `Fatal` |

Each one reaches `on_notice`. The ERROR tokens of a call make one `Server` error in order. An ENVCHANGE of the database or language after the login reaches `on_notice` as a `Setting` named `database` or `language`.

#### MySQL protocol

`std/sql/mysql` speaks the MySQL client/server protocol, version 10 greeting and protocol 4.1. The greeting tells MySQL from MariaDB, and the connection carries `sql.mysql()` or `sql.mariadb()`.

#### MySQL packets

Packets are a three-byte length and a sequence number. A payload of `0xFFFFFF` bytes or more is split, with an empty packet closing an exact multiple. The sequence numbers are checked on every packet.

#### MySQL login and TLS

The login negotiates protocol 4.1, `DEPRECATE_EOF`, session tracking, multiple statements and result sets, and `utf8mb4`. On MariaDB it asks for extended column metadata, which names `JSON` columns.

`connect_tls` sends the SSL request and runs the `std/tls` handshake before the login. It fails with `Unsupported` when the server offers no TLS. A login after `connect_stream` uses the stream as given.

#### MySQL authentication plugins

Authentication follows the plugin the server names, and an auth switch changes it: `mysql_native_password`, `caching_sha2_password`, `sha256_password`, MariaDB's `client_ed25519` and `mysql_clear_password`.

`caching_sha2_password` answers the fast path, and the full path sends the password in the clear over TLS and RSA-OAEP (SHA-1) encrypted under the server's public key over a plain connection, which the driver asks for with a one-byte request and which seals the NUL-terminated password XORed with the nonce. `sha256_password` does the same.

- `mysql_clear_password` is refused without TLS.
- An empty password sends an empty response.
- Any other plugin fails with `Unsupported`.

#### MySQL statements

| Statement | Goes |
| --- | --- |
| Without parameters | As `COM_QUERY` and reads text rows. |
| With parameters | Through `COM_STMT_PREPARE` and `COM_STMT_EXECUTE`, reads binary rows, and keeps a cache of 64 prepared statements. |

The cache closes the oldest when full. A text or byte parameter of 1 MiB or more is sent in 1 MiB `COM_STMT_SEND_LONG_DATA` chunks.

#### MySQL result sets

`stream` passes every result set of a statement in order, each ending with `Done` and the number of its rows. A statement without a result set gives `Done` with its affected rows. `query` returns the last set.

#### MySQL column metadata

| `Column` field | Value |
| --- | --- |
| `scale` | The decimals byte of its definition (also the fractional-second digits of temporal types) |
| `precision` | The display length of a decimal |
| `length` | In bytes for `char`, `varchar`, `binary` and `varbinary` |

#### MySQL server text

`Options.server_text` has no effect, and no `Setting` is sent.

#### MySQL errors and unsupported requests

An ERR packet becomes a `Diagnostic` whose code is the SQLSTATE and whose message ends with the error number. A `LOCAL INFILE` request from the server is refused with `Unsupported`. The connection stays usable.

#### MySQL value decoding

Values decode every column type:

- integers with their signedness, with `BIGINT UNSIGNED` as `Decimal`
- `DECIMAL` and `NEWDECIMAL` as `Decimal`
- floats
- `DATE`
- `TIME` (negative and past 24 hours)
- `DATETIME`
- `TIMESTAMP`
- `YEAR`
- `BIT` as bytes
- `ENUM` and `SET` as text
- `JSON` as text
- binary and character columns told apart by character set
- geometry as bytes

After the login the session selects the UTC time zone.

#### MySQL connection state and transactions

`State` reports `server`, `mariadb`, `connection_id`, `schema` and `in_transaction`, from the greeting, the status flags and the session-tracking packets. It carries `ping` and `reset`, which send `COM_PING` and `COM_RESET_CONNECTION`.

`begin_with` takes an `Isolation` and a read-only flag and starts the transaction; `savepoint`, `rollback_to` and `release` manage nested ones.

#### MySQL cancellation and timeouts

- A timeout opens a second connection with the same options, runs `KILL QUERY` for the thread id, and closes the first. The call fails with `Timeout`.
- `cancel` runs the same `KILL QUERY` once the statement is sent and keeps the connection. The call fails with `Cancelled` when the server reports error 1317.
- A malformed packet abandons the connection with `Protocol`.
- Dropping the connection sends `COM_QUIT`.

#### MySQL implicit commits

MySQL commits implicitly before and after a DDL statement, so a migration that creates or alters a table cannot be rolled back with the rest of its transaction. The connection reports `in_transaction` from the server's flags, so a caller can see it.

#### MongoDB wire protocol

`std/mongo` speaks `OP_MSG` and `OP_COMPRESSED` over TCP, or over `std/tls` when the connection string or `Options.tls` asks for it.

#### MongoDB messages

A message carries a kind 0 body and kind 1 document sequences. `insert`, `update` and `delete` send their documents as sequences, batched by the server's `maxWriteBatchSize` and `maxMessageSizeBytes`.

- A reply's CRC-32C is verified when the flag is set.
- A reply that sets `moreToCome` or an unknown flag bit, a frame over 96 MiB and a frame that does not parse fail with `Protocol`.
- A request sets `moreToCome` only for an unacknowledged write (`w: 0`), which gets no reply.

#### MongoDB compression

`OP_COMPRESSED` carries snappy, zlib or zstd, chosen from `compressors` against the server's list. The handshake and authentication commands are never compressed.

#### MongoDB handshake and authentication

The handshake sends `hello` with `helloOk`, the client metadata and `saslSupportedMechs`.

Authentication takes SCRAM-SHA-256 or SCRAM-SHA-1 through `std/crypto/sasl`, the strongest the server lists unless `authMechanism` names one, with `skipEmptyExchange`.

| Mechanism | Secret |
| --- | --- |
| SCRAM-SHA-1 | The hex of `MD5("user:mongo:password")` |
| SCRAM-SHA-256 | The password as given |

- A non-ASCII password fails with `Unsupported` because the driver has no SASLprep.
- A refused login fails with `Authentication`.
- `MONGODB-X509`, `tlsCertificateKeyFile` and `mongodb+srv://` fail with `Unsupported`.

#### MongoDB server monitoring

Every server has a monitor green thread with its own connection. The monitor polls `hello` every `heartbeat` milliseconds. It can be woken to recheck sooner, but never more often than every 500.

#### MongoDB topology

The topology is unknown, single, a replica set without or with a primary, or sharded. A primary is stale when its `setVersion` and `electionId` are behind the newest seen. Members come from `hosts`, `passives` and `arbiters`. A member whose `me` differs from the address used is dropped.

#### MongoDB server selection

A selection filters by read preference mode, tag sets and `maxStalenessSeconds`, keeps the servers within `local_threshold` of the fastest and fails with `Selection` after `server_selection_timeout`.

`$readPreference` goes out for any mode but `primary` on a replica set or sharded topology, and on a single member that is neither a standalone nor a mongos, where `primary` is sent as `primaryPreferred`.

#### MongoDB connection pools

Each server has a pool of up to `max_pool_size` connections.

| Setting or event | Effect |
| --- | --- |
| `max_idle_time` | An idle connection closes after it. |
| `wait_queue_timeout` | A caller waits up to it for one. |
| A network failure | It closes the idle connections of that server and starts a new generation. |

#### MongoDB sessions

- Every command runs in a session, an implicit one drawn from a pool of server sessions (`lsid` as UUID binary subtype 4) unless the call names one.
- `endSessions` goes out when the client closes.
- `$clusterTime` from every reply is carried on every request.
- A causal `Session` adds `afterClusterTime` from its `operationTime` to reads.

#### MongoDB retries

A read, and a write that carries a `txnNumber` on a server that supports it, is retried once on a freshly selected server after a network failure or a retryable error. `update_many` and `delete_many` are not retried.

#### MongoDB error mapping

| Server reply | Becomes |
| --- | --- |
| A server error | `Server` with its `errorLabels` |
| A `writeErrors` entry | `Write` with the position in the batch |
| A `writeConcernError` | `WriteConcern` |

#### MongoDB transactions

A transaction sends `startTransaction`, `autocommit: false` and the next `txnNumber` with its first command and the session's read concern with `afterClusterTime`. It runs on one primary.

- `commit_transaction` retries once and then asks for a majority write concern.
- `with_transaction` reruns its body for up to 120 seconds on `TransientTransactionError` and `UnknownTransactionCommitResult`.
- A transaction on a server without replica set support fails with `Unsupported`.

#### MongoDB unsupported features

Not implemented:

- DNS seed lists
- client certificates
- SASLprep
- streaming `hello` and exhaust cursors (the monitor polls, and `exhaustAllowed` is only encoded and decoded)
- tailable cursors
- pipeline-style updates
- aggregates over a database
- sharded transaction pinning and `recoveryToken`
- load-balanced mode
- a minimum pool size
- typed documents through serde

#### TOML parsing and encoding

`std/toml` parses into ordered tables and applies every rule of TOML 1.0.0, including the restrictions on redefining tables through headers, dotted keys, inline tables and arrays of tables. It agrees with Python's `tomllib` on the toml-test suite. Errors carry a line and column.

`encode` writes plain values first and then `[table]` and `[[array]]` sections. `serialize` and `deserialize` bind types through serde, reading date-times as RFC 3339 text.

#### YAML parsing

`std/yaml` parses YAML 1.2.2 into ordered node trees and agrees with the yaml-test-suite on every case's event stream and JSON equivalent.

#### YAML scanner, parser and composer

A scanner modelled on libyaml tokenizes the whole stream, tracking indentation, implicit keys, JSON-like adjacent values and the YAML 1.2 tab rules, with 32-byte vector scans over plain scalars and `simd.find_byte_str` for comment and line ends. Token text is a view into the source.

A recursive-descent parser turns tokens into `Event`s with resolved tags, and a composer builds `Node`s carrying line, column, tag, anchor and style.

#### YAML scalars, aliases and limits

Plain scalars resolve with the core schema. `!!str`, `!!int`, `!!float`, `!!bool`, `!!null` and `!!binary` are checked against their content; other tags stay on the node. Aliases share their anchored node.

| Limit | Effect |
| --- | --- |
| `Options.max_nodes` | Bounds the tree with every alias expanded, failing with `TooManyNodes`. |
| `max_depth` | Bounds nesting. |

Duplicate scalar keys are an error.

#### YAML encoding and serde

`encode`:

- writes block style
- quotes a scalar only when it would read back differently (including YAML 1.1 booleans, numbers and dates)
- writes multi-line text as literal blocks
- anchors any node reached more than once

`serialize` and `deserialize` bind types through serde, with `u8[]` as `!!binary`.

#### OpenTelemetry context

`std/otel` keeps the provider in the green thread's context slot as an immutable `Context` of the active span, baggage, a suppression flag and the SDK.

`spawn` copies the slot, so a child green thread's spans parent to the spawning span.

#### OpenTelemetry installation

`install` and `setup_from_env` return a droppable `Sdk` that shuts down when released. They put the `SdkHandle` into the calling green thread's slot, which reaches the green threads it spawns afterwards. Programs install at the top of `main`.

Every API call checks the slot first and returns a no-op value without allocating when no SDK is present.

#### OpenTelemetry timestamps, ids and samplers

| Item | Comes from |
| --- | --- |
| Timestamps and durations | `time.now_nanos` and `time.monotonic` |
| Span and trace ids | A `std/rand` generator behind a lock |

Samplers are always on, always off, trace-id ratio on the low 63 bits, and parent-based.

#### OpenTelemetry batch processors

Batch processors for spans and logs queue to a bounded channel with a non-blocking send that counts drops. A worker green thread exports:

- when a batch fills
- when the delay passes
- on flush or shutdown

Periodic metric readers use the same control worker for flush and shutdown.

#### OpenTelemetry metric instruments

Metric instruments aggregate per attribute set, keyed by a canonical encoding of the sorted attributes, in one stream per reader, capped at 2000 series plus an `otel.metric.overflow` series. Readers collect cumulatively or as deltas, periodically or on demand.

#### OpenTelemetry exporters

Exporters encode OTLP once through a writer that emits protobuf through `std/protobuf` or the OTLP JSON mapping, and send with the context suppressed.

Processors and metric readers call exporters through `quietly`, so no exporter has to suppress its own telemetry.

#### OTLP/HTTP exporter

OTLP/HTTP posts, optionally gzipped. It retries 429, 502, 503 and 504 through `std/retry` with equal-jitter exponential backoff, where `Retry-After` raises the wait.

#### OTLP/gRPC exporter

OTLP/gRPC makes unary `Export` calls on `opentelemetry.proto.collector.{trace,metrics,logs}.v1` through a `std/grpc` `Channel` dialled to the endpoint:

| Endpoint | Channel |
| --- | --- |
| `http://` | h2c |
| `https://` | TLS |

The calls are gzipped under `grpc-encoding` when the exporter's `gzip` is set. They carry the exporter's headers as metadata and its timeout as the call deadline.

#### OTLP/gRPC retries

The channel's own retry is off. The exporter's retry policy decides.

| Status | Retried |
| --- | --- |
| CANCELLED, DEADLINE_EXCEEDED, ABORTED, OUT_OF_RANGE, UNAVAILABLE and DATA_LOSS | Retried. |
| RESOURCE_EXHAUSTED | Retried only when `grpc-status-details-bin` carries a RetryInfo, whose delay replaces the backoff. |

#### OpenTelemetry environment setup

`setup_from_env` reads the standard `OTEL_*` variables. `OTEL_EXPORTER_OTLP_PROTOCOL=grpc` selects gRPC, defaulting to `http://localhost:4317`.

#### HTTP tracing hook

`std/http` sits below `std/otel` in the import graph, so the hook runs through the context slot. `otel.Context` declares an `http.Ambient` as its first field. `std/http` reads the slot as `*http.Ambient`, which relies on struct fields being laid out in declaration order.

One constructor builds every `Context` and derives the ambient `Observer` from it. The observer is set exactly when an SDK is installed and the context is not suppressed. `std/http` does nothing when it is absent.

#### HTTP server spans

`http.serve` and `serve_tls` open a server span per request over HTTP/1.1 and HTTP/2, with the parent extracted from `traceparent` and `client.address` from `Request.remote`, and record `http.server.request.duration`.

`std/web` adopts that span, adds `http.route` and renames it to the method and route. It opens its own span only when its handler runs outside `http.serve`.

#### HTTP client spans

- The client opens a CLIENT span named after the method for each attempt sent, redirects and retries included.
- Each span is under the caller's span, with `http.request.resend_count` from the second attempt.
- The client injects `traceparent`, `tracestate` and `baggage` into that attempt, sets `url.full` without userinfo, and ends the span when the body is delivered or the attempt fails, with `error.type` the status from 400 or the `http.Error` variant.
- It records `http.client.request.duration`.

#### SQL driver tracing

`std/sql/driver` wraps every connection that `std/sql/postgres`, `std/sql/sqlserver`, `std/sql/mysql` and `std/sql/sqlite` return with `instrument_sql`. It reads the `db.system.name` from the connection's `Dialect`.

`std/orm` opens client spans with the operation and table. A nested database span merges into the outer one, so an ORM query produces one span.

#### Protobuf encoder

`std/protobuf` writes and reads the protobuf wire format without code generation. An `Encoder` appends into a buffer that doubles as needed. Signed integers sign-extend to ten bytes and `sint64` zigzag-encodes.

`begin_message` returns a mark. `end_message` measures the body, shifts it right by the size of its length varint and writes the length. Nested messages therefore need neither a second buffer nor padded lengths. Output matches `protoc --encode` byte for byte.

#### Protobuf decoder

A `Decoder` walks fields in order. It yields each field as number, wire type, integer value and a view of the payload.

| Input | Rejected with |
| --- | --- |
| Truncated or overlong varints | `Malformed` |
| Lengths past the end | `Malformed` |
| Field 0 | `Malformed` |
| Wire types 6 and 7 | `Malformed` |
| Groups | `Unsupported` |

The `as_*` readers check the wire type and the `unpack_*` readers accept packed and single encodings.

#### Protobuf serde binding

`serialize[T]` and `deserialize[T]` take a struct whose fields carry their protobuf numbers as tags (see Structs). They run through `std/serde`. Encoding builds a `serde.Tree` with `serde.to_tree` and writes it by the schema. Decoding turns the wire bytes into a tree by the schema and reads it with `serde.read`.

#### Protobuf field tags

A tag is a comma-separated list of the field number, `sint`, `fixed`, `unpacked`, `required`, `key=sint`, `key=fixed`, `value=sint` and `value=fixed`.

#### Protobuf type mapping

| Type | Protobuf form |
| --- | --- |
| Integers | The `int32`, `int64`, `uint32` or `uint64` encoding by width and sign |
| A field of an enum with payload variants | Tagged `oneof`. Each variant's `= n` index is the field number of its member, and a variant's payload tag has no number. |
| An enum of payload-free variants | A protobuf enum of the variants' indexes |
| A struct | An embedded message |
| A slice | Repeated, packed unless tagged `unpacked` |
| A map | A repeated entry message |
| A pointer field such as `*Node?` | Carries a recursive message |

#### Protobuf writing rules

| Field | Written |
| --- | --- |
| A singular field that holds its default | Not written unless it declares a default of its own |
| An optional field | Whenever it holds a value |
| A non-optional embedded message | Always |

#### Protobuf decoding rules

- Decoding gives an absent field its declared default, `none` for an optional one and zero otherwise.
- It takes the last value of a repeated singular field and merges repeated occurrences of an embedded message.
- It collects repeated fields packed or not and takes the last member of a `oneof`.
- It skips unknown fields, fields of the wrong wire type and enum values no variant has.
- It checks strings for UTF-8.

#### Protobuf serde errors

| Error | Raised for |
| --- | --- |
| `Syntax` | Malformed input, at a byte offset |
| `TooDeep` | More than 100 levels of messages |
| `Mismatch` | A `T` that is not a struct |
| `Missing` | An absent `required` field |

#### Protobuf schema parsing

`std/protobuf/proto` reads `.proto` text with `parse(path, source)`. It accepts `syntax`, `package`, `import` with `public` and `weak`, `option`, `message`, nested messages and enums, `enum` with `allow_alias`, `oneof`, `map`, `repeated`, `optional`, proto2 `required` and `default`, `reserved`, `extensions`, `extend`, and `service` with `rpc` and `stream` on either side.

It keeps options as written. The exceptions are:

- `default` and `json_name` are fields of `Field`.
- `packed` and `deprecated` change what the generator writes.

#### Protobuf schema rejections

`parse` rejects:

- a proto2 `group`
- an `edition`
- proto3 files that use `required`, `default`, extension ranges or a first enum value other than zero
- bad or duplicate field numbers and names
- reserved numbers and names
- wrong map keys
- empty oneofs
- nesting past 32 levels

Each rejection is one `proto.Error` variant with the file, line and column. `proto.message` formats it.

#### Protobuf file loading

`read_files(paths, roots, read)` loads each path and its imports through `read`. `read` returns the text of a file or fails with `io.Error.NotFound`. The loader tries the roots in order and rejects import cycles. It lists the files with every file after the files it imports.

The loader touches no file system itself, so a program supplies `file.read_text` and a test supplies a map.

#### Protobuf name resolution

The loader gives each type its full name and replaces every type name with the full name of the message or enum it refers to. A name resolves by matching the whole written name against the innermost enclosing message and then each scope outwards through the package, then absolutely. It resolves only through the file's own definitions, its imports and the `import public` closure of those.

#### Protobuf module generation

`generate(set, options)` writes one Low module per file, `a/b.proto` as `a/b/b.low`. `Options.module` is the import path of the output directory.

| Protobuf | Generated |
| --- | --- |
| A message | A `pub struct` |
| An enum or oneof | A `pub enum` |
| A singular message field | Optional |
| A singular field or oneof member in a cycle of messages | `*T?` |
| A map | `[K: V]` with its key and value encodings in the tag |
| A proto2 field with a default | Keeps it as the field's default |
| A message without fields | One `bool` field `empty` on field number 536870911 |
| A type from another file | Reached through an import of that file's module |

The output is a function of the files alone. Extensions and other options are left out.

#### Protobuf generated names

- Nested types are named by joining the names of the messages around them: `Outer.Inner` is `OuterInner`, and the oneof `choice` of `Scalars` is `ScalarsChoice`.
- A name that is already taken gets a number after it.
- Field names stay when they are `snake_case`. Other field names go through `std/text/naming`.
- A keyword gains a trailing `_`.
- An enum value loses the enum name as a prefix when a name remains.

#### Protobuf generated services

A `service` with methods becomes, for service `Greeter`, a `GreeterClient` made by `greeter_client(channel)`. The client has one function per method named after it in `snake_case` (a name another service of the file already took gains a number), whose call shape follows the `stream` keywords and whose types are the request and response messages.

The service becomes a `GreeterHandlers` struct with one function-valued field per method. It becomes `register_greeter(server, handlers)`, which registers each method as `/package.Greeter/Method`.

A service with no methods gets only the client.

#### Protobuf generation command

`low protogen -I dir,dir --module path -o outdir file.proto...` reads the files against the roots and writes the modules under `outdir`, imported files included. The generator is the library `compiler/protogen`, which `compiler/low/protogen.low` calls.

#### XML input

`std/xml` reads XML 1.0 (fifth edition) with Namespaces. The input may be:

- UTF-8
- UTF-16 with a byte order mark
- any encoding `std/charset` knows by its declared label

The reader converts it to UTF-8 and normalises line endings first.

#### XML pull reader

`xml.Reader` is a pull reader: `next` returns one token and `position` its line and column. Names arrive with namespaces resolved and are interned in a per-reader table. Text, CDATA, comments and attribute values are views into the input unless a reference or normalisation forces a copy. 32-byte vector scans find the bytes that end them.

The reader enforces well-formedness and the namespace constraints. It agrees with libxml2 on what it accepts.

#### XML internal subset and entities

The internal subset is parsed. General entities expand, including markup. Expansion has recursion detection, a nesting limit of 40 and a total of 1 MiB plus eight times the input.

Attribute-list declarations supply defaults and normalise tokenised types. External entities are never fetched and parameter entities are not expanded.

#### XML tree and writer

`xml.parse` builds an `Element` tree of owned strings with `attribute`, `child`, `children`, `text` and `find`.

`xml.Writer` escapes text and attributes and splits CDATA around `]]>`. It writes namespace declarations and indents element-only content.

#### XML serde binding

`serialize_value` and `deserialize` bind types through serde. They map:

- fields to attributes or child elements by local name
- repeated elements to slices
- `xsi:nil` to `none`

#### Pipe blocking

Pipes block through channels. A writer flushing more than the pause threshold waits until the reader brings the unconsumed bytes below the resume threshold, or until the reader itself waits for more data. A message larger than the thresholds therefore cannot deadlock the pipe.

#### Connection pool

- `pool.open` stores `ConnectionHandle` views. One connection therefore serves a lease at a time, and the pool, not the owner, closes it.
- The pool checks idle and lifetime limits on acquire, release and `prune` against `clock.monotonic`.
- A release hands the connection to the oldest waiter.
- A lease marks its connection broken on `Connect`, `Authentication`, `Closed`, `Timeout` or `Protocol` errors. The pool closes it.
- A transaction left open at release is rolled back.

#### Migration checks

- `migrate.up` refuses any applied migration whose checksum differs (`Drift`).
- It refuses a pending version below the highest applied one (`OutOfOrder`).
- It refuses duplicate or non-ascending versions.

#### Migration locks

The lock is:

| Database | Lock |
| --- | --- |
| SQLite | `BEGIN IMMEDIATE` |
| PostgreSQL | `pg_advisory_lock` |
| SQL Server | `sp_getapplock` |
| MySQL and MariaDB | `GET_LOCK` |

Each migration re-checks inside its transaction that it is not yet recorded.

#### Migration failures

A migration that fails reports its own error, or the rollback's error when the rollback fails too. A lock that cannot be released fails with `Sql` unless a migration error is reported first.

#### Text migrations

In a text migration a statement ends at a line ending in `;`, and `-- +begin` to `-- +end` keeps a block with several `;` as one statement.

#### Unicode

`std/unicode` reads tables generated by `compiler/tablegen`. The normalization and case folding tables are of Unicode 16.0.0, and the word, sentence and line break tables of Unicode 17.0.0.

##### Unicode normalization tables

`std/unicode` reads tables generated by `compiler/tablegen unicode` from UnicodeData.txt, CaseFolding.txt and DerivedNormalizationProps.txt of Unicode 16.0.0. This is the version Python's `unicodedata` reports, and `std/unicode` agrees with it on every code point.

##### Unicode layout files

`compiler/tablegen unicode` and `compiler/tablegen properties` also write `normalization_layout.low` and `property_layout.low`: constants for the record sizes, field offsets, trie shifts and masks, Hangul constants and packed property bit fields, and one `<table>_table()` accessor per `.bin` file that `embed`s it. The module sources read these instead of literals.

`layout_files_match_generator` in `compiler/tests/tablegen` fails when a committed layout file differs from the generator.

##### Case folding orbits for regular expressions

`compiler/tablegen classes` writes the simple case folding orbits and the word set that `std/regex` embeds, as `### Regular expressions` describes.

##### Normalization table format

The tables are `embed`ded binary files. A two-level trie of 64-entry blocks covers code points below U+2FA40 and gives 0 above. It maps each code point to a 16-bit record id.

Each record holds the canonical combining class and 16-bit offsets and lengths into a pool of `u32` code points. The offsets and lengths cover:

- the full canonical decomposition
- the full compatibility decomposition
- the C and F case folding

##### Normalization algorithms

Hangul syllables decompose and compose algorithmically. Decomposition is followed by a stable canonical reordering by combining class.

Composition binary-searches a sorted table of primary composites. The table leaves out Full_Composition_Exclusion. ASCII input returns at once, or lowercased by `casefold` and `search_key`.

##### Segmentation tables

Word, sentence and line break data live in their own tries, `word_*`, `sentence_*` and `line_*` `.bin` files of an index, 32-entry blocks and records. `compiler/tablegen segmentation` writes them from WordBreakProperty.txt, SentenceBreakProperty.txt, LineBreak.txt, EastAsianWidth.txt, emoji-data.txt and UnicodeData.txt of Unicode 17.0.0. `segmentation_layout.low` holds the shifts, masks, flag bits and class constants.

| Record | Holds |
| --- | --- |
| Word record | The Word_Break class and the Extended_Pictographic bit. |
| Line record | The Line_Break class after the LB1 resolution (AI, SG and XX to AL, SA to CM for Mn and Mc and to AL otherwise, CJ to NS), the East Asian Width F, W or H bit, the Extended_Pictographic and Cn bit, and the Pi and Pf bits of quotation marks. |

| Trie | Cost |
| --- | --- |
| Word | 45,877 bytes |
| Sentence | 47,343 bytes |
| Line | 43,546 bytes |

Each is linked only when `words`, `sentences` or `line_breaks` is reachable.

##### Grapheme, word and sentence scanning

`word_end`, `sentence_end` and `grapheme_end` scan forward from a segment start with a small state record and the next code point. They look ahead only where a rule needs it (WB6, WB7b, WB12, SB8). WB4 and SB5 fold Extend, Format and ZWJ into the character before them.

##### Line break scanning

`line_breaks` scans the whole string once, because UAX #14 rules (LB15a, LB19a) read the character before a break position and a restart at an earlier break would change the answer. It implements LB1 to LB31 of Unicode 17.0.0 with no tailoring.

| Rule | Handling |
| --- | --- |
| LB9 and LB10 | Fold or replace CM and ZWJ. |
| LB15a to LB15d, LB20a, LB28a with U+25CC and LB30b | Use the record bits. |
| LB3 | The break after the last character is mandatory. |
| LB4 and LB5 | The breaks after BK, LF, NL and a CR not followed by LF are mandatory. |

##### Segmentation conformance

The default rules pass every line of WordBreakTest.txt, SentenceBreakTest.txt and LineBreakTest.txt of 17.0.0.

##### Search keys

`search_key(s)` equals Python's `''.join(c for c in unicodedata.normalize('NFKD', s.casefold()) if not unicodedata.combining(c))`. It skips reordering, since every reordered character is dropped.

#### Hostnames

##### Punycode

`std/net/punycode` implements RFC 3492 with 32-bit overflow checks. Its encoder and decoder agree with every sample of section 7.1 and with Python's `punycode` codec on 300 random strings.

##### IDNA processing

`std/net/idna` follows UTS #46 revision 33 for Unicode 16.0.0. It:

- maps
- normalises to NFC through `std/unicode`
- breaks at the label separators
- decodes `xn--` labels
- then validates each label against the criteria `V1` to `V7` and `U1`

##### IDNA options

| Option | Effect |
| --- | --- |
| `check_bidi` | Applies the RFC 5893 Bidi rule when set. |
| `check_joiners` | Applies the RFC 5892 ContextJ rule for U+200C and U+200D when set. |
| `use_std3_ascii_rules` | Restricts ASCII code points to letters, digits and `-`. |

`to_ascii` encodes the labels with Punycode and applies the length limits when `verify_dns_length` is set.

The mapping table of this version has no `disallowed_STD3` entries. ContextO and `IgnoreInvalidPunycode` are not provided.

##### IDNA tables

`compiler/tablegen idna <ucd directory> <output directory>` generates the tables from IdnaMappingTable.txt of `Public/idna/16.0.0`, and DerivedBidiClass.txt, DerivedJoiningType.txt and UnicodeData.txt of Unicode 16.0.0. It writes `idna_layout.low` and these files:

| File | Holds |
| --- | --- |
| `idna_mapping.bin` | Sorted ranges with a status and a pool offset, adjacent equal ranges merged. |
| `idna_pool.bin` | The mapped UTF-8 text, shared. |
| `idna_classes.bin` | Sorted ranges holding the Bidi class, the joining type and the mark flag. |

Lookups binary-search the ranges.

##### IDNA conformance

The conformance fixture runs every line of IdnaTestV2.txt for 16.0.0 through `to_unicode` and through `to_ascii` with nontransitional and transitional processing. 6,387 of 6,387 lines agree on the text and on the failed check.

The two lines whose source holds an unpaired surrogate are not run, because a `str` cannot hold one.

##### Public suffix trie

`std/net/publicsuffix` stores the rules as a trie of reversed labels in UTF-8. `compiler/tablegen publicsuffix <public_suffix_list.dat> <output directory>` writes it into `suffix_nodes.bin`, `suffix_labels.bin`, `suffix_version.bin` and `suffix_layout.low`.

Each node records:

- whether a rule ends there
- whether a wildcard rule `*.` hangs from it
- whether an exception rule `!` ends there

and for each whether the rule is in the private section.

##### Public suffix matching

- The longest rule wins.
- An exception rule wins over any other and drops its leftmost label.
- A name no rule matches takes its last label by the default rule `*`.

##### Embedded public suffix list

The embedded list is version 2026-11-14_09-32-49_UTC, commit c3a53eed2cc49353c5fb5c30536186795d5c451b of github.com/publicsuffix/list. `compiler/tablegen publicsuffix public_suffix_list.dat std/net/publicsuffix` regenerates it from a downloaded `public_suffix_list.dat`.

The fixtures replay the list's `tests/test_psl.txt` and check every rule of the list against the tables.

##### Public suffix label matching

| Label | Handling |
| --- | --- |
| A label that is ASCII and does not begin with `xn--` | Matched as its lowercase form without UTS #46 processing, so `_dmarc` is accepted. |
| Any other label | Processed with `check_hyphens`, `use_std3_ascii_rules` and `verify_dns_length` off. |
| An empty label or a rejected label | Gives `none`. |

##### URL host conversion

`http.parse_url` converts a host with non-ASCII bytes with `to_ascii`. It runs with `check_hyphens`, `use_std3_ascii_rules` and `verify_dns_length` off, as the WHATWG URL standard does. It fails with `Invalid` when the conversion fails.

`Url.host` is therefore ASCII for the `host` header, the TLS server name, the resolver, `net.dial` and the proxies.

##### Cookie Domain attributes

A `Domain` attribute is converted the same way. One that is a public suffix is accepted only when it equals the request host. This makes the cookie host-only (RFC 6265bis section 5.7). A request to an IP literal skips the check.

##### Linking of hostname tables

The tables are linked into a program only when it calls a function that reads them.

| Program | Links |
| --- | --- |
| A program that serves HTTP without the client | None. |
| A program that uses the client | The mapping tables of `std/net/idna`, the tables of `std/net/publicsuffix` and the normalisation tables of `std/unicode`. |

#### Locales

`std/locale` and its sub-modules read CLDR 48.0. The tables are generated, and a program pays for the tables it reaches.

##### Locale data sources

`std/locale` and its sub-modules read CLDR 48.0 (the `common` directory of `cldr-common-48.0.zip`) and, for collation, `common/uca/allkeys_CLDR.txt`, `common/uca/FractionalUCA.txt` and the Unicode 17.0.0 `UnicodeData.txt`. The tables are generated:

| Command | Writes |
| --- | --- |
| `compiler/tablegen cldr <cldr common> std/locale/data` | The locale blobs, the supplemental tables, the plural tables and `layout.low`. |
| `compiler/tablegen collate <cldr common> <ucd directory> std/locale/data` | The collation tables. |

A different CLDR release is a regeneration of both over that release.

##### Locale blobs

`std/locale/data/bin` holds one binary blob per CLDR locale file, 621 in all. `locales.txt` lists the 1122 identifiers that resolve to data.

A blob is a list of numbered groups (`layout.low` names them):

- decimal and percent patterns and symbols
- compact patterns
- currency names
- month, weekday, quarter, day-period and era names
- date and time patterns
- available formats
- interval formats
- zone formats
- relative-time patterns
- list patterns

##### Locale blob inheritance

A formatter walks `Locale.blobs` from the most specific blob to the root and takes the first one that has the entry. Inheritance therefore follows the CLDR parent chain.

##### Locale table linking

A program pays for the tables it reaches.

| Call | Reaches |
| --- | --- |
| `locale.parse` | Every blob through `data.blob_by_id`. |
| `locale.parse_with` with `data.chain_<id>()` | The blobs of one chain. |

##### Linked program sizes

These are sizes of the linked program on Linux amd64.

| Program | Bytes on Linux amd64 |
| --- | --- |
| `print("hello")` | 40,952 |
| A program that calls only the English forms of `std/text/number`, `std/time/humanize`, `std/time/datetime` and `std/time/pattern` | 117,808. That is 4,760 bytes more than the same program with the locale-taking functions deleted. The program links no locale table. |

| Locales | Program | Bytes on Linux amd64 |
| --- | --- | --- |
| One locale (`de`, through `parse_with`) | `number.grouped_in` | 491,208 |
| One locale (`de`, through `parse_with`) | `datefmt.format` | 426,120 |
| One locale (`de`, through `parse_with`) | `collate.root` | 1,224,136 |
| One locale (`de`, through `parse_with`) | `collate.for_locale` for `sv` | 1,638,808 |
| All locales (`locale.parse`) | `number.grouped_in` | 4,263,816 |
| All locales (`locale.parse`) | A program that uses `datefmt.format`, `numfmt.format_int` and `collate.for_locale` | 5,723,888 |

The root collation table is linked only when a collator is built. The 191 KB of tailoring rules is linked only when `collate.for_locale` is reached.

##### Locale plural rules

Plural rules come from `plurals.xml`, `ordinals.xml` and `pluralRanges.xml` as tables that `plural.cardinal`, `plural.ordinal` and `plural.range` evaluate.

`locale_plural` checks every `@integer` and `@decimal` sample of every rule of both files. `locale_plural_range` checks the range category of 91 locale tags against `Intl.PluralRules.selectRange`.

##### Locale number formatting

Number formatting follows ECMA-402 `Intl.NumberFormat` over the CLDR patterns:

- grouping with the minimum grouping digits of the locale
- numbering systems from `-u-nu`
- rounding by fraction digits, significant digits, increment and the nine rounding modes on the exact decimal
- sign display
- compact notation
- currency symbols, narrow symbols, codes and plural names with accounting and cash digits

Parsing localized numbers is not provided.

##### Calendars in date formatting

Date formatting is Gregorian only. The `ca` key is read and does not change the output. The data of other calendars is not generated.

##### Date patterns and names

`datefmt.format` prints the standard patterns of the four lengths, or the pattern the CLDR distance rules pick from the available formats for a skeleton, with the `j` skeleton letter resolved by the hour-cycle preference of the locale and `hc` overriding it. Month, weekday, era, quarter and day-period names come from the locale.

##### Zone fields and date ranges

A zone field prints the localized GMT offset form (`GMT+01:00`, `GMT+1`). The zone names of CLDR are not generated. `format_range` follows the interval formats of the locale.

##### Locale strftime layouts

`std/time/pattern` accepts `%c`, `%x` and `%X` through `compile_in` and `datefmt.layouts`. Both turn the standard date, time and date-time patterns of a locale into strftime layouts with ASCII digits.

A standard pattern with a field no strftime verb prints, as in `la` and `zh-Hant`, fails with `NoLayout`.

##### Collation algorithm

Collation implements UTS #10 over the CLDR root order (`allkeys_CLDR.txt`):

- 32-bit weights with implicit weights for unassigned and Han code points
- contractions, expansions and canonical reordering
- non-ignorable and shifted variable weighting
- strengths primary to identical
- backwards secondary weights
- `caseFirst` upper and lower
- numeric ordering of ASCII digit runs
- `sort_key` and `compare`

##### Tailored collators

A tailored collator compiles the CLDR rules of the locale when it is built:

- `&` resets (with `[before n]` and `[last regular]`)
- `<` through `<<<<`
- `=`
- expansions
- `[reorder]`
- `[suppressContractions]`
- the `caseFirst` and `alternate` settings of the collation

The collations CLDR 48 defines without an `alt` attribute are generated for 113 locale files. The locale's own tag or its `-u-co-` type selects one (`phonebook`, `traditional`, `search`, `eor` and the rest).

##### Collation limits

Absent:

- `zh` and `zh-Hant`, which use the root order (the rules of `zh.xml` are not generated)
- collations with an `alt` attribute
- the rules that use a prefix context (`|`), a logical position other than `[last regular]` (`[first ...]` and the other `[last ...]`), or `[import]`

Not implemented:

- `caseLevel`
- `optimize`
- a `maxVariable` other than punctuation
- numeric ordering of non-ASCII digits

`caseFirst` is decided per collation element, with no separate class for mixed-case contractions.

##### Collation conformance

`locale_collate` runs `CollationTest_CLDR_NON_IGNORABLE_SHORT.txt` and `CollationTest_CLDR_SHIFTED_SHORT.txt` (CLDR 48 UCA conformance files) through the root collator and compares each line's sort key and the order of adjacent lines. The full files (not embedded, 18 MB and 22 MB) pass at 206,298 of 206,298 and 227,809 of 227,809 sort keys and 206,297 of 206,297 and 227,808 of 227,808 adjacent orders.

##### Locale matrices compared with Intl

The other tests compare the module with Node 24.19.0 `Intl` (ICU 78.3, CLDR 48.0, Unicode 17.0) over matrices. Scripts kept outside the repository generate the matrices, and the matrices are embedded as TSV files:

| Test | Rows | Tags | Covers |
| --- | --- | --- | --- |
| `locale_number` | 10,509 | 86 | Decimal, percent, scientific, compact, currency, accounting, sign display, grouping, rounding. |
| `locale_numbers` | 3,791 | 48 | `grouped_in`, `percent_in`, `compact_in`, `currency_in`. |
| `locale_date` | 6,846 | 81 | |
| `locale_date_range` | 8,836 | 30 | |
| `locale_list` | 8,640 | 80 | |
| `locale_reltime` | 7,218 | 81 | |
| `locale_humanize` | 5,400 | 40 | |
| `locale_pattern` | 765 | 51 | `%c`, `%x`, `%X`. |
| `locale_collate_intl` | 39,217 | 66 | Tags including `-u-co-phonebk` and `-u-co-trad`, and the strength, alternate, case-first, numeric and backwards-secondary options. |

Every embedded row passes.

##### Rows left out of the locale matrices

Rows left out of the matrices because the module and `Intl` differ:

- 1 `fr` compact-long row (it needs the exact-count plural key `count="1"` that `numfmt` does not read)
- in `locale_date`: 79 `ko` rows containing the day-period names `오전` and `오후` and 18 `zh-Hant` rows with a date and a time
- in `locale_date_range`: 44 interval rows (`bg` and `vi` `jm` across midnight, `ja` `h12` across midnight)
- 32 collation rows, those of `fr-CA` with `ð` or `đ` and those of `da` with `caseFirst: lower` and `Aa`

##### Interval format coverage

Interval formats are matrix-checked for date-only skeletons and for time-only skeletons within one day. Skeletons that combine a date and a time across different dates are covered by the interval rules and not by a matrix.

##### Locale features not provided

Not provided:

- other calendars
- time zone display names
- CLDR units and unit formatting
- `humanize.in_words` and `humanize.compact` for other languages
- ordinal suffixes by locale
- number parsing
- the collation features listed above

#### HTML

`std/html` tokenizes UTF-8 HTML following the WHATWG tokenization states and never fails.

##### HTML tokenizer rules

- Tag and attribute names are lowercased in ASCII.
- Attribute values may be double-quoted, single-quoted or unquoted.
- A repeated attribute keeps its first value.
- A tag cut off by the end of input is dropped.

##### Raw text and script data

| Start tag | Switches to |
| --- | --- |
| `script`, `style`, `xmp`, `iframe`, `noembed` or `noframes` | Raw text, until the matching end tag in any case. |
| `title` or `textarea` | Escapable raw text, until the matching end tag in any case. |
| `plaintext` | Takes the rest of the input. |

Script data follows the escaped and double-escaped states of the standard. `<script><!--<script></script>--></script>` is therefore one element. `noscript` is tokenized as when scripting is disabled.

##### Comments, instructions and doctypes

Comments end at `-->` or `--!>`, with `<!-->` and `<!--->` empty. `<?target data?>` is an `Instruction` token when the target starts with a letter or `_` and holds letters, digits, `-` and `_`.

These become comments:

- `<?xml ...>`
- `<?xml-stylesheet ...>`
- any other `<?...>`
- `<!...>` other than comments and doctypes
- `<![CDATA[...]]>`
- `</` followed by a non-letter

`</>` is skipped. `Doctype` carries the lowercased name, the optional public and system identifiers and `force_quirks`.

##### HTML character references

Character references are decoded in text, escapable raw text and attribute values.

| Reference | Handling |
| --- | --- |
| Named references | Match the longest name against the full WHATWG table of 2,231 names, including the names valid without a semicolon. |
| Decimal and hexadecimal references | Work with or without a semicolon. |
| Zero, surrogates and values above U+10FFFF | Become U+FFFD. |
| 0x80 to 0x9F | Map through Windows-1252. |
| Inside an attribute, a match without a semicolon that is followed by `=` or a letter or digit | Stays literal. |

`compiler/tablegen entities` generates the table and also writes `entity_layout.low` with the index entry layout, the name length limit and the table accessors. Text without references is a view into the input.

##### Tree construction

`parse` runs the tree construction algorithm of the standard over the tokenizer, which the builder drives:

- every insertion mode
- the stack of open elements
- the list of active formatting elements with the adoption agency algorithm
- foster parenting for table content
- `template` contents
- `frameset`
- the HTML integration points
- the adjusted tag and attribute names of SVG and MathML
- the quirks mode decision from the doctype

Scripting is disabled: `noscript` content is parsed as elements.

##### Fragment parsing

| Function | Parses |
| --- | --- |
| `parse_fragment(context, input)` | Applies the fragment case with a context element. |
| `parse_body(input)` | Is the fragment case with `body`. |

##### Parse results

The result `Parsed` holds `root` (the document, or a fragment node), `quirks` and `errors`. `errors` holds the parse errors of the tree builder in the order found. The tokenizer's recoveries are not listed. The `Document` node carries the quirks mode.

##### Selected content of a select element

Once parsing ends, a `select` element that is not `multiple` and has a display size of 1 gets its first `selectedcontent` descendant filled with a deep copy of the children of its selected option.

The selected option is the last option with `selected`, or else the first option that is not disabled.

##### Parse time and memory

`parse`, `parse_fragment`, `parse_body`, `sanitize` and `clean` take time and memory linear in the input for every input. Nesting 100,000 levels deep therefore takes milliseconds.

##### Stack of open elements

The stack of open elements keeps, for every entry, pointers to the nearest entry at or below it that is:

- a scope boundary of each kind (default, list item, button, table)
- a special element
- a list-item scan stop
- a reset-insertion-mode stop
- an HTML-namespace element

It keeps a chain per element name. As a result "has an element in scope", the `li`, `dd` and `dt` scans, "any other end tag", foster parenting and the reset of the insertion mode do not walk the stack.

##### Active formatting elements

The list of active formatting elements is a linked list with a marker segment per `template` or applet-like boundary and a queue of at most three entries per name and attribute set. The Noah's Ark clause is therefore a lookup.

##### Adoption agency bounds

The adoption agency algorithm keeps the standard's bounds of 8 outer and 3 inner iterations. It uses an entry pointer in place of the bookmark index.

##### Text and attributes while parsing

Text is appended to a byte buffer that is written to its node once. Attributes are deduplicated through a map.

##### Reconstruction of formatting elements

The one superlinear case in the standard is the reconstruction of formatting elements, which can create a new element for every formatting entry at every `p` start tag. A parse creates at most four times the input length in reconstructed elements. After that the entries are dropped instead of recreated.

##### Maximum nesting depth

`ParseOptions.max_depth` bounds the element nesting the way Blink and WebKit do. The default is 512 (`default_max_depth`), and 0 or less means no limit.

##### Nodes past the depth limit

While more elements than `max_depth` are open, a new element, comment or instruction becomes a child of the parent of the current node. Every node past the limit is therefore a child of the element at the limit. The stack of open elements keeps growing, so end tags still match their start tags.

Text, nodes foster parented out of a table, `template` content and nodes the adoption agency algorithm moves are placed as the standard says. A text node can sit one level below the limit.

Misnested formatting markup can still build a tree deeper than the limit, as it does in browsers. A program that walks a parsed tree therefore must not recurse once per level.

##### Sanitizing and the depth limit

`sanitize` always parses with the default limit and parses its own output again. Its result is therefore never deeper than the limit allows.

##### Parse options and the depth fixture

`parse_with` and `parse_fragment_with` take the options. `max_depth: 0` gives the standard's tree. `testdata/run/std_html_depth` holds the element count and depth of twelve nesting shapes. The default tree of each equals Chromium's.

##### Node structure

A `Node` has a `data` of `Document`, `Fragment`, `Doctype`, `Element`, `Text`, `Comment` or `Instruction`, and first, last, previous and next links. Insertion and removal therefore take constant time.

An `Element` has:

- a `Namespace` (`Html`, `Svg` or `MathMl`)
- a name
- its attributes in source order (each with an optional namespace, prefix, name and value)
- for `template`, a content fragment

##### Node functions

| Functions | Do |
| --- | --- |
| `create_element`, `create_text`, `create_comment`, `create_doctype`, `create_instruction`, `create_document` and `create_fragment` | Make detached nodes. |
| `append_child` and `insert_before` | Fail with `HierarchyRequest` or `NotFound` and change nothing when they fail. |
| `remove` | Detaches a node. |
| `clone` | Copies a node or a subtree. |
| `contains`, `children`, `parent`, `first_child`, `last_child`, `previous_sibling`, `next_sibling`, `attr`, `has_attr`, `set_attr`, `remove_attr`, `local_name`, `is_html`, `template_contents` and `text_content` | Read and edit it. |

##### Iterative tree walks

Walks, `clone`, serialization, selector matching and `clean` are iterative. A tree 300,000 levels deep therefore does not exhaust the stack.

##### HTML serialization

`inner_html` and `outer_html` serialize as section 13.3 of the standard.

| Where | Escapes |
| --- | --- |
| Text and attribute values | `&` and U+00A0. |
| Attributes | Also `"`. |
| Text | Also `<` and `>`. |

| Content | Written |
| --- | --- |
| Void elements of the HTML namespace | With no end tag. |
| The children of `style`, `script`, `xmp`, `iframe`, `noembed`, `noframes` and `plaintext` | Raw. |
| The content of a `template` | Inside it. |

##### Reparsable serialization

A `pre`, `textarea` or `listing` element does not get an added line feed. A leading line feed in its content is therefore lost when the text is parsed again. `reparsable_html` writes one more line feed after those start tags when the content begins with one. Parsing its result gives the same tree.

##### Selector compilation

`compile(source)` turns a selector list into a `Selector` once. `matches`, `closest`, `query` and `query_all` use it many times.

##### Supported selectors

`compile` supports:

- type and universal selectors
- `.class`
- `#id`
- attribute selectors with `=`, `~=`, `|=`, `^=`, `$=`, `*=` and the `i` and `s` flags
- the descendant, `>`, `+` and `~` combinators
- selector lists
- `:not()`, `:is()`, `:where()`
- `:has()` with a relative selector list
- `:first-child`, `:last-child`, `:only-child`
- `:nth-child()` and `:nth-last-child()` with an optional `of` selector list
- the `-of-type` forms
- `:empty`, `:root`, `:checked`, `:disabled` and `:enabled`

##### Selector errors

A source it cannot compile fails with `SelectorError.Invalid(at, message)`, where `at` is a byte offset. These are errors:

- namespace prefixes
- pseudo-elements
- other pseudo-classes (`:hover`, `:focus`, `:lang()`, `:dir()`, `:scope`, `:required` and the rest)
- `||`
- `&`
- a `:has()` inside `:has()`
- unterminated constructs
- more than 1000 compound selectors in one complex selector
- more than 32 nested functional pseudo-classes

| Selector | Empty or invalid argument |
| --- | --- |
| `:is()` and `:where()` | Dropped. |
| `:not()`, `:has()` and `nth-child` with `of` | Rejected. |

##### Selector matching

Matching runs right to left with backtracking. `:has()` tests candidates below or after the subject.

##### Selector case sensitivity

| Match | Case |
| --- | --- |
| Type and attribute names on an HTML element | Without regard to case. |
| Type and attribute names on SVG and MathML elements | Exactly. |
| Attribute values | Exactly except for the attributes HTML lists as case-insensitive (`type`, `lang`, `dir`, `rel`, `method` and the like). The `i` and `s` flags override that. |
| `.class` and `#id` in a document in quirks mode | Without regard to ASCII case. |
| Attribute selectors in a document in quirks mode | Still compare exactly. |

##### Pseudo-class matching

| Pseudo-class | Matches |
| --- | --- |
| `:empty` | An element with no child element and no text. Whitespace therefore counts as content. |
| `:checked` | Reads the `checked` attribute of a checkbox or radio `input` and the `selected` attribute of an `option`. |
| `:root` | The child of the document. |

##### Selector queries

`query` and `query_all` search the descendants of the node in document order, skip the content of `template` elements and never return the node itself. `closest` starts at the node.

##### Sanitize policy

`sanitize(input, policy)`:

- parses `input` as the content of a `body`
- removes what a `Policy` does not allow
- serializes the rest

A `Policy` lists:

- the allowed HTML elements
- the attributes allowed on every element and on each named element
- the allowed URL schemes
- whether a URL without a scheme is allowed
- the elements dropped with their content
- the allowed SVG and MathML elements

##### Sanitizing elements

| Element | Result |
| --- | --- |
| An HTML element that is not listed | Unwrapped, so its children take its place, unless it is in the drop list. |
| A foreign element that is not listed | Dropped with its content. |

These are always removed:

- comments, instructions and doctypes
- the elements `script`, `style`, `noscript`, `template`, `iframe`, `object`, `embed`, `applet`, `noembed`, `noframes`, `plaintext`, `xmp`, `frame`, `frameset`, `base`, `link` and `meta`
- the SVG elements `script`, `style`, `foreignObject`, `use`, `image`, `feImage`, `iframe` and the animation elements
- the MathML elements `annotation-xml` and `maction`

##### Sanitizing attributes

An attribute stays only when:

- it is allowed
- its name does not start with `on`
- it is not `srcdoc`

##### Sanitizing URL attributes

The attributes that hold a URL (`href`, `src`, `srcset`, `action`, `formaction`, `poster`, `cite`, `xlink:href` and the older `background`, `data`, `longdesc`, `manifest`, `usemap`, `codebase`, `classid`, `profile`, `icon`, `lowsrc`, `dynsrc`, `imagesrcset` and `ping`) are checked as a browser reads them.

- Tabs and line breaks anywhere and control characters and spaces at either end are ignored.
- A scheme is what precedes a `:` that comes before any `/`, `?` or `#`.
- Every candidate of a `srcset` and every URL of a `ping` must pass.

##### Sanitize output check

`sanitize` serializes with `reparsable_html`. The result is checked before it is returned: parsing it and serializing the tree gives the same text and cleaning that tree removes nothing. When the check fails, the output is cleaned again. When five rounds do not settle it, the result is the empty string. No input therefore yields markup that a second parse reads differently.

##### Default policy and clean

`default_policy` allows headings, paragraphs, lists, tables, quotations, inline text elements, links and images with `class`, `dir`, `lang` and `title`, the `http`, `https` and `mailto` schemes and relative URLs, and no SVG or MathML. `clean(root, policy)` does the same to a tree in place.

##### HTML fixtures

| Fixture | Checks |
| --- | --- |
| `testdata/run/std_html_tree` | Runs the html5lib tree-construction files of the web-platform-tests copy of the html5lib suite and passes every case except the ones that need scripting enabled. |
| `testdata/run/std_html_serialize` | Compares `inner_html` and `outer_html` with parse5. |
| `testdata/run/std_html_select` | Compares the selector results and rejections with jsdom and soupsieve. |
| `testdata/run/std_html_quirks` | Holds the mode of 41 doctypes, equal to parse5's, and the class and id counts with and without a doctype, equal to jsdom's. |
| `testdata/run/std_html_adversarial` | Parses, cleans and sanitizes 25 hostile shapes (nested `div`, `b`, `a`, `table`, `svg`, `math`, `template`, `li`, `dd`, `option` and `button`, stray end tags, repeated and distinct attributes, one-character text tokens and character references) at sizes where a quadratic parser takes seconds. Its counts for the unlimited tree equal parse5's. |
| `testdata/run/std_html_edit` | Compares the tree editing results and errors with jsdom, and `clean` with hand-derived counts. |
| `testdata/run/std_html_sanitize` | Compares `sanitize` with a reference sanitizer written over parse5 for URL obfuscations, mutation-XSS payloads and generated markup. It checks that the output holds no `script` element, event handler attribute or `javascript:` URL. It checks that the output is a fixed point of parse and serialize. |

#### Markdown

`std/markdown` parses blocks with the CommonMark line algorithm and GFM extensions. `render` writes the HTML `marked` writes with `gfm` on.

##### Markdown block parsing

`std/markdown` parses blocks with the CommonMark line algorithm: a stack of open containers, lazy continuation lines, and a list that is loose when a blank line separates its items or two of its blocks.

##### GFM extensions

GFM adds tables, strikethrough, task list items and extended autolinks. The autolinks are `http://`, `https://`, `ftp://`, `www.` and bare email addresses, with `marked`'s trailing punctuation rules.

##### Markdown tables

- A table needs a delimiter row of the same width as the header.
- It ends at a blank line or a line that starts another block.
- Body rows are padded or cut to the header width.

##### Markdown inline parsing

Inline parsing scans delimiter runs into a chain. It resolves emphasis, strong and strikethrough with the CommonMark flanking and rule-of-three tests, so the work is linear in practice. Links and images use a bracket stack.

Inline content is parsed once per heading, paragraph or table cell. This happens after every reference definition in the document is known.

##### Markdown escapes and entities

Escapes and entities stay in the tree as `Escape` and `Entity` nodes, so a renderer can tell `\*` from `*`. Numeric references decode. Named references pass through to the HTML unchanged, as `marked` does.

##### Markdown plain text

`Heading` and `Cell` carry the raw source text next to the parsed content. `plain_text` renders spans without emphasis, link or image markup. Images give their alt text and character references are decoded. Inline HTML stays as written and a break is a line ending.

##### Markdown HTML rendering

`render` writes the HTML `marked` writes with `gfm` on. It writes:

- `<br>` and `<hr>` without a slash
- a `<tbody>` with no line ending before its first row
- tight list items without `<p>`
- a `language-` class from the first word of the info string
- `cleanUrl` for hrefs

A raw HTML block is written as parsed, so a block followed by a blank line is not followed by a line ending.

##### Markdown renderer hooks

Each hook of a `Renderer` receives the renderer. `render` and `render_inline` take any renderer, so a hook can nest the rest of the output. The default hooks are public, and a custom hook can call them.

##### Markdown agreement with marked

The output equals `marked` on `docs/design.md`, `editors/README.md` and the doc comments of the standard library. Paragraphs keep their leading and trailing whitespace as `marked` does, except at the end of a list.

Known departures are in unusual input:

- an empty list item that follows a paragraph or a setext underline
- a bare URL directly followed by `[`, a backtick or a backslash, which `marked` takes into the link
- a paragraph that ends in a whitespace-only line

### D-Bus

`std/dbus` does not use `std/serde`: its model is name-keyed structs, with no 16-bit integer, signature typing, variant, dict entry or descriptor, and derived code cannot produce the tuple structs of D-Bus.

#### D-Bus values

Values are the dynamic `Value` enum. Its variants are `Byte`, `Boolean`, `Int16`, `UInt16`, `Int32`, `UInt32`, `Int64`, `UInt64`, `Double`, `String`, `ObjectPath`, `Signature`, `UnixFd`, `Bytes` for `ay`, `Array`, `Struct`, `Variant` and `DictEntry`.

- `Int16` carries an `i64` in -32768..32767 because Low has no 16-bit integer.
- `Array` holds the element signature so an empty array keeps its type.

| Functions | Do |
| --- | --- |
| `variant`, `strings`, `dict` and `dict_of` | Build values. |
| `bool_of`, `int_of`, `uint_of`, `double_of`, `string_of`, `fd_of`, `bytes_of`, `array_of`, `fields_of` and `unwrap` | Read them, failing with `Marshal` on a different variant. |

#### D-Bus signatures and names

`Type` is a parsed signature.

| Function | Behaviour |
| --- | --- |
| `parse_signature` | Accepts at most 255 bytes, array nesting of 32, struct nesting of 32 and 64 nesting levels in all. It rejects empty structs, a dict entry outside an array and a non-basic dict key. |
| `parse_type` | Takes one complete type. |
| `parse_element` | Takes one array element type. |
| `valid_object_path`, `valid_interface`, `valid_member`, `valid_error_name` and `valid_bus_name` | Follow the specification's name rules. Those rules include the 255-byte limit and the unique-name form. |

#### D-Bus value encoding

`encode(value, order)` returns an `Encoded`, which holds bytes and descriptor numbers. `decode` reads either byte order.

- Alignment is relative to the start of the message.
- Array lengths exclude their padding.
- Strings and signatures must be UTF-8 without NUL.
- Booleans are 0 or 1 and padding bytes are zero.
- An array over 64 MiB fails with `TooLarge`.
- A variant nests at most 64 deep.

#### D-Bus messages

`Message` carries the type, the flags, the serial, the header fields and the body. The flags are `no_reply_expected`, `no_auto_start` and `allow_interactive_authorization`.

| Function | Behaviour |
| --- | --- |
| `encode_message` | Writes the header `yyyyuua(yv)` padded to 8 bytes and checks the fields each type requires. |
| `message_length` | Reads the fixed header from 16 bytes and returns the total length, failing with `TooLarge` over 128 MiB. |
| `message_fd_count` | Returns the `UNIX_FDS` field. |
| `decode_message` | Takes the message with the descriptors that accompanied it. |
| `method_call`, `signal`, `method_return` and `error_reply` | Build messages. |
| `remote_error` | Turns an error message into `Remote(name, message)`. |

#### D-Bus addresses

`parse_addresses` splits an address on `;` into `Address` values, each a transport and a key map.

| Function | Behaviour |
| --- | --- |
| `address_guid` | Reads the `guid` key. |
| `address_text` | Writes an address back. |
| `escape_address_value` and `unescape_address_value` | Apply the `%xx` escaping. |
| `socket_path` | Resolves `unix:path=`, `unix:abstract=` and `unix:runtime=yes`, where the last means `$XDG_RUNTIME_DIR/bus`. |
| `session_addresses` and `system_addresses` | Read `DBUS_SESSION_BUS_ADDRESS` and `DBUS_SYSTEM_BUS_ADDRESS`. They fall back to `unix:path=$XDG_RUNTIME_DIR/bus` and `unix:path=/run/dbus/system_bus_socket`. |

#### D-Bus authentication

Authentication is the SASL exchange of the specification. A client:

1. Sends the NUL byte.
2. Sends `AUTH EXTERNAL` with the hexadecimal uid.
3. Falls back to `ANONYMOUS` after a `REJECTED`.
4. Sends `NEGOTIATE_UNIX_FD` and `BEGIN`.

`ServerAuth` configures the other side with `guid`, `uid`, `anonymous` and `unix_fds`. EXTERNAL is accepted when the uid the client states equals `uid` and the peer credentials of the socket report the same uid. `uid` is the effective uid of the process by default. Lines are at most 16384 bytes.

#### D-Bus connections

| Function | Behaviour |
| --- | --- |
| `connect_session`, `connect_system` and `connect_addresses` | Open a `Connection`. |
| `connect_over` | Takes an open `net.ConnHandle` and says whether to send `Hello`. |
| `accept_over` | The server side. |

`Connection` is the owner of a `ConnectionHandle`. The handle shares one reader green thread, which matches replies to calls by serial. That thread offers every other message to the filters added with `add_filter`, and it ends when the socket closes.

#### D-Bus calls and descriptors

`call`, `call_with` and `send` take serial numbers from the connection. `call_with` takes a `CallOptions` with a timeout and a `cancel.Token`.

| Pending calls fail with | When |
| --- | --- |
| `Closed` | The connection ends. |
| `Timeout` or `Cancelled` | Their option fires. |

A method return carries the descriptors that came with its message. Descriptors travel with a sent message through `SCM_RIGHTS` on one write.

#### D-Bus bus daemon and standard interfaces

| Function | Behaviour |
| --- | --- |
| `request_name`, `release_name` and `name_has_owner` | Use the bus daemon. |
| `add_match`, `remove_match` and `subscribe` | Register a `MatchRule`. |
| `rule_text` | Writes the rule. |
| `rule_matches` | Evaluates it locally. |
| `Subscription` | Delivers the matching messages on a channel. |
| `get_property`, `set_property`, `get_all`, `ping`, `machine_id`, `introspect_xml` and `introspect` | Call the standard interfaces. |
| `introspect` | Parses the XML with `std/xml` into a `NodeInfo`. |

#### D-Bus services

| Function | Behaviour |
| --- | --- |
| `serve` | Makes a `Service`, a filter that answers every method call that arrives. It answers with `UnknownObject` for a path nothing is exported at. |
| `export` | Takes an `InterfaceSpec`, which holds methods, properties and signals. |
| `emit_changed` | Sends `PropertiesChanged`. |

Each call runs on its own green thread. `Introspectable`, `Properties` and `Peer` are answered from the specifications.

#### D-Bus service errors

- A handler fails with `Remote(name, message)` to reply with that error.
- Other errors reply `org.freedesktop.DBus.Error.Failed` or `InvalidArgs`.
- An unknown path, interface, method or property replies with the matching standard error.

#### D-Bus operations on a connection handle

Every operation has a form that takes the `*ConnectionHandle` and ends in `_on`. It is for threads the connection has been moved away from, as `### Resource ownership` describes.

#### D-Bus tests

| Checked | Comes from |
| --- | --- |
| Marshalling vectors | `jeepney`, cross-checked against GLib `GVariant`. |
| Signatures and addresses | libdbus. |
| Names | GLib. |

The protocol fixtures run with no daemon, over a socket pair. `std/dbus/daemon_test_linux.low` starts a private `dbus-daemon`. It runs a Low service against `busctl`, `gdbus` and `dbus-send`, and a Low client against a Gio service. Each test skips when a tool is missing.

### Desktop portals

`std/portal` talks to `org.freedesktop.portal.Desktop` on the session bus.

#### Portal connections

| Function | Behaviour |
| --- | --- |
| `connect()` | Returns a `Portal` that owns its connection. |
| `over(handle)` | Returns a `Portal` that borrows a `*dbus.ConnectionHandle`. |
| `connect_within(ms)` | Limits the calls that do not wait on the user, and the limit is 25 seconds otherwise. |

A `Portal` is bound with `using`, and several green threads may use it at once.

#### Portal request pattern

A call that waits on the user follows the portal's request pattern in `request(p, ask, token, stop)`.

The request object path is `/org/freedesktop/portal/desktop/request/<sender>/<token>`. It is computed from the connection's unique name and the handle token. The unique name drops its leading colon and writes dots as underscores.

1. The `Response` signal on that path is subscribed to before the call.
2. The call returns the handle, and a handle other than the computed one moves the subscription.
3. The call waits for `Response`, for the connection to close, or for `stop`.

`Response` carries a code and an `a{sv}` of results. Cancelling `stop` sends `Request.Close` and fails with `Cancelled`. An empty handle token takes `low1`, `low2` and on, per `Portal`.

#### Portal file chooser

`open_file(p, dialog)` and `save_file(p, dialog)` take a `FileDialog`. A `FileDialog` holds a parent window string, title, accept label, `modal`, `multiple`, `directory`, `filters`, `current_filter`, `current_name`, `current_folder`, `current_file`, token and cancel token. A `FileFilter` has a name, glob patterns and MIME types.

Both functions return the chosen `file://` URIs and the selected filter. They return `none` when the user closed the dialog. Response code 2 fails with `Remote("org.freedesktop.portal.Error.Failed", ...)`.

#### Portal parent window

`parent_window` is the portal's string: `x11:<id in hex>`, `wayland:<exported handle>`, or empty for none. `std/ui` windows do not export a Wayland handle, so they pass it empty.

`std/portal` does not import `std/ui`, and `parent_window` is a string the caller builds. A window of `std/ui` on Wayland offers only its `wl_surface` through `native_window`. The portal needs `wayland:` and a handle from `zxdg_exporter_v2`. The Wayland client in `std/ui/platform` does not speak `zxdg_exporter_v2`. A dialog from a `std/ui` program therefore passes an empty `parent_window` and is not attached to the window.

#### Portal open URI and settings

| Function | Behaviour |
| --- | --- |
| `open_uri(p, uri, request)` | Returns `false` when the user closed the application chooser. |
| `read_setting`, `color_scheme`, `accent_color` and `animations_enabled` | Read `org.freedesktop.portal.Settings.Read` and unwrap its variants. |
| `color_scheme` | Returns `NoPreference`, `Dark` or `Light`. |
| `accent_color` | Returns `none` when a channel is outside 0 to 1. |
| `watch_settings(p, namespace)` and `setting_change` | Follow `SettingChanged`. |

#### Portal notifications

`add_notification`, `remove_notification`, `watch_actions` and `action_invoked` cover the notification portal with a title, body, themed icon name, priority, default action and buttons.

#### Portal call functions

Each call has a `..._call` function that returns its `dbus.Message` without sending it.

#### Portal tests

- `std/portal/portal_test.low` compares the marshalled bodies of those calls with `testdata/portal/*.hex`.
- `testdata/portal/generate.py` writes those files with `jeepney`.
- `portal_test_linux.low` runs a Low `FileChooser` service on a private `dbus-daemon`. It tests the request pattern, dismissal, failure and cancellation without a dialog. It reads the colour scheme from the session's portal when one runs. Each skips when its tool or portal is missing.

### Accessibility

`std/atspi` exports one application's accessible tree on the accessibility bus.

#### AT-SPI bridge connection

| Function | Behaviour |
| --- | --- |
| `bus_address()` | Asks `org.a11y.Bus.GetAddress` on the session bus. |
| `connect(application, notify)` | Connects to that address and exports the objects. It registers the application with the registry through `org.a11y.atspi.Socket.Embed` on `org.a11y.atspi.Registry`. |
| `serve(h, application, notify)` | Exports on a connection the caller made, which is how the tests run the bridge over a socket pair. |
| `close(b)` | Ends a connection `connect` made. |

A `Bridge` is a copyable handle.

#### AT-SPI object tree

The tree is flat: every node is a child of the window.

| Object | Path | Interfaces |
| --- | --- | --- |
| The application object | `/org/a11y/atspi/accessible/root` | `Accessible` and `Application` |
| One window object | `/org/a11y/atspi/accessible/window` | `Accessible` and `Component`, role frame |
| One object per `Node` | `/org/a11y/atspi/accessible/e<n>` | `Accessible` and `Component`. `Action` with one action named `click` when it is `actionable`. `Text` when `text` is set. |

A `Node` carries a `key`, a `name`, a `Role`, bounds in window coordinates, `visible`, `focused`, `focusable`, `actionable`, `editable` and `text`.

#### AT-SPI roles

`Role` holds the AT-SPI numbers of `atspi-constants.h`:

| Role | Number |
| --- | --- |
| check box | 7 |
| dialog | 16 |
| frame | 23 |
| image | 27 |
| label | 29 |
| list | 31 |
| list item | 32 |
| menu | 33 |
| menu item | 35 |
| page tab | 37 |
| panel | 39 |
| push button | 43 |
| application | 75 |
| entry | 79 |
| heading | 83 |
| link | 88 |

#### AT-SPI state bits

The state bits follow the same header.

| Object | State bits |
| --- | --- |
| A visible node | `enabled`, `sensitive`, `visible` and `showing` |
| The node itself supplies | `focusable`, `focused`, `editable` and `single-line` |
| A check box | `checkable` |
| The window | `active` |

#### AT-SPI updates and signals

`update(b, title, active, nodes)` replaces the tree. A key keeps its object path for the life of the bridge. An object appears for a new key and goes away for a key the update leaves out. An update equal to the last one does nothing.

The signals go out as the tree changes, on `org.a11y.atspi.Event.Object`:

| Signal | Sent |
| --- | --- |
| `ChildrenChanged` | With `add` or `remove` and the index on the window. |
| `StateChanged` | For `focused`, `showing`, `visible` and `active`. |
| `PropertyChange` | For `accessible-name`. |

Plus `org.a11y.atspi.Event.Focus.Focus` for a node that gains focus and `org.a11y.atspi.Event.Window.Activate` and `Deactivate`.

#### AT-SPI queued actions

What assistive technology asks for is queued, not run.

| Request | Queues |
| --- | --- |
| `DoAction` | `Click(key)` |
| `GrabFocus` | `Focus(key)` |

`notify` runs on the connection's green thread, and the application's own thread calls `take_actions(b)`.

#### AT-SPI text offsets

`Text` serves characters, not bytes, so offsets and counts are in code points.

#### AT-SPI in std/ui programs

`std/ui` connects the two.

| Function | Behaviour |
| --- | --- |
| `on_frame(cx, f)` | Runs `f` after each frame. |
| `enable_accessibility(cx, application) -> Accessibility ! Error` | Connects a bridge, publishes `elements(cx)`, `window_title(cx)` and `window_focused(cx)` after every frame and runs the queued actions on the context's green thread before the next frame, with `click_element` and `set_focus`. |

| Element | Exported as |
| --- | --- |
| A button, link, check box, list item, tab or menu item | `actionable`. |
| A field | `focusable` and `editable`. |
| A text run, heading or field | Serves its text, but a `masked` field's text is empty. |
| A text run inside an actionable element | Left out because the element's name already holds it. |

Releasing the `Accessibility` closes the connection. It fails with `Unsupported` when the session has no accessibility bus and on Windows, where UI Automation is not implemented.

#### AT-SPI missing features

Not done:

- The tree is flat and has no `Cache` object, so a client that asks for the cache falls back to calling each object.
- A `text_area`'s text is not exported because `ElementInfo.text` is empty for it.
- `Selection`, `Value`, `EditableText` and the caret are absent.
- The extents are window coordinates because a Wayland client does not know where its window is.

#### AT-SPI tests

- `std/atspi/atspi_test_linux.low` walks the tree through a client on a socket pair and checks the events, the actions and the registration.
- `daemon_test_linux.low` starts a private `dbus-daemon` with a stand-in registry, registers the bridge with it and reads the tree back with `gdbus`, and skips without `dbus-daemon` or `gdbus`.
- `std/ui/accessibility_test_linux.low` draws a form in a headless context, reads the exported tree through a client on a socket pair and presses and focuses its elements.

#### AT-SPI read through libatspi

A window opened with `ui.open` and `enable_accessibility` was read from outside through libatspi against the session's `at-spi2-registryd`.

- The read covered the application, frame, heading, button, entry and label with their names, roles, extents and text.
- It covered `DoAction` on the button, `GrabFocus` on the entry, and the tree after the click.

Orca has not been run against it.

### Regular expressions

`std/regex` compiles a pattern once into a program for a Pike VM. A Pike VM is a Thompson NFA simulation that carries capture slots per thread. Matching therefore takes time linear in the input times the program size and never backtracks.

#### Regex machine pools

A `Regex` keeps two pools of idle machines.

| Pool | Serves |
| --- | --- |
| The pool with no capture slots | `is_match` |
| The pool with capture slots | `find`, `full_match` and the scans |

Each machine owns its thread queues, capture slots and scratch. A call takes an idle machine under a lock. It builds one when none is idle, and it puts the machine back when it finishes. A compiled value is therefore shared freely between green threads, and a callback that matches with the same `Regex` during `replace_all` gets its own machine. A `Match` copies its slots out of the machine.

#### Regex syntax

The syntax is:

- literals
- escapes `\n \t \r \f \v \a`, `\xhh`, `\uhhhh`, `\Uhhhhhhhh` and escaped punctuation
- `.`
- classes with ranges, negation and `\d \w \s \D \W \S` inside
- `\d \w \s \D \W \S \b \B \A \Z \z`
- `^` and `$`
- capturing groups, `(?:...)`, inline flags `(?i)`, `(?m)`, `(?s)`, `(?-i)` and scoped `(?i:...)`
- alternation
- `*`, `+`, `?`, `{m}`, `{m,}`, `{,n}` and `{m,n}` with lazy forms

A `{` that does not form a valid count is a literal. Inline flags apply to the rest of the enclosing group.

#### Regex input encoding

Input is UTF-8 matched by code point. Offsets are bytes. `\d` and `\s` are ASCII: `[0-9]` and `[\t-\r ]`.

#### Regex word characters

`\w` is the UTS #18 word set: Alphabetic, Mark (`Mn`, `Mc`, `Me`), Decimal_Number (`Nd`), Connector_Punctuation (`Pc`) and Join_Control (U+200C and U+200D). `\W` is its complement.

`\b` and `\B` compare the word-ness of the code point before the position with that of the code point at it, so a boundary is a change in word-ness. A byte that is not valid UTF-8 is not a word character.

#### Regex case folding

`(?i)` folds by Unicode simple case folding (`CaseFolding.txt` status C and S). A literal, a class member or every code point of a range matches all members of its orbit. So under `(?i)` these match:

| Pattern | Matches |
| --- | --- |
| `k` | `K` and U+212A KELVIN SIGN |
| `s` | U+017F |
| `σ` | `ς` and `Σ` |
| `ß` | U+1E9E |
| `ǆ` | `ǅ` and `Ǆ` |
| `(?i)[a-z]` | Also U+017F and U+212A |

A class folds before it is negated, so `(?i)[^k]` matches none of the three. `\d`, `\s`, `\w` and their negations are not folded.

#### Regex dot and line terminators

- `.` excludes only `\n` unless `dot_all` is set, so it matches `\r`. `[^\r\n]` is a character outside a line terminator.
- Without `multiline`, `^` matches only at the start of the input and `$` matches at the end or before a final `\n`.
- With `multiline`, a line ends at `\n`, at a `\r` not followed by `\n`, or at `\r\n`: `^` matches at the start, after `\n` and after a `\r` not followed by `\n`, and `$` matches at the end, before `\r` and before a `\n` not preceded by `\r`, so neither matches between the `\r` and the `\n` of one terminator.

#### Regex Unicode tables

The word set and the fold orbits are `word_ranges.bin` and `fold_orbits.bin` in `std/regex`. `compiler/tablegen classes <ucd directory> std/regex` writes them from `UnicodeData.txt`, `DerivedCoreProperties.txt`, `PropList.txt` and `CaseFolding.txt` of Unicode 16.0.0, the version of the category data in `std/unicode`.

| File | Holds |
| --- | --- |
| `word_ranges.bin` | 796 sorted `lo`, `hi` pairs of little-endian `u32` (6,368 bytes). |
| `fold_orbits.bin` | 2,938 `code`, `next` pairs sorted by `code` (23,504 bytes). `next` is the next larger member of the orbit, and the largest member points to the smallest. |
| `class_layout.low` | Holds the entry sizes and the `embed` accessors. `compiler/tablegen classes` writes it. |

Folding a range binary-searches the first orbit member inside it and follows each orbit, so a range as wide as the code space costs one pass over the table. `compiler/regex` embeds the same two files, so a generated pattern and `compile` build identical classes.

#### Regex ASCII fast paths

Bytes below U+0080 never reach the tables.

| Function | Behaviour |
| --- | --- |
| `in_class` | Scans the ranges that start below U+0080 before it binary-searches the rest. |
| `word_before` and `word_at` | Test an ASCII byte directly. |
| A non-ASCII word test | Decodes the code point and binary-searches `word_ranges.bin`. For the code point before the position, it steps back at most three continuation bytes. |

#### Regex leftmost-first matching

Matching is leftmost-first, as in Python and RE2. Results for ASCII patterns and ASCII text without `\r` agree with Python's `re` under `re.ASCII`.

#### Regex empty matches and splitting

`find_all`, `replace_all` and `split` follow Python 3.7.

- An empty match is allowed right after a non-empty one.
- After an empty match, the next one may start at the same offset only if it is not empty.
- `split` does not include group text.
- `split_n` splits at most `max` times, and a negative `max` splits everywhere.

#### Regex limits and rejected patterns

| Limit | Value |
| --- | --- |
| Repeat counts | 10000 |
| Programs | 100,000 instructions |

Invalid patterns fail with `Invalid(at, message)`, at the byte offset Python reports. Backreferences, lookaround, named groups and possessive quantifiers are rejected.

#### Regex generated patterns

`regex.generated("pattern")` is a `Regex` built at compile time, like .NET's `[GeneratedRegex]`.

- The pattern must be a string literal, and flags are written inline.
- An invalid pattern is a compile error at the literal, `invalid regex at byte N: message`. The offset and message are the ones `compile` would fail with.
- The result is used through the same functions as a compiled `Regex`, and it matches identically.

#### Regex generated pattern checker

`compiler/regex` holds the parser and assembler the checker runs on a literal pattern. It is held to the `std/regex` one by running the `std_regex_errors` fixture's patterns through it.

The checker turns each distinct literal into Low functions named `__regex_` and the FNV-1a hash of the pattern. It parses them with the prelude's privileges and adds them to `std/regex`, so they reach its private types.

#### Regex generated engine

A `Regex` carries an `Engine` of thread-start, step and skip functions with the queue size. `compile` fills it with the Pike VM. The generated engine keeps the VM's queues, priorities and capture slots but replaces interpretation with code per instruction:

- One function per instruction that starts a thread, follows a consumed character or joins two epsilon paths, each guarded by the queue's sparse set. The remaining jumps, splits, saves and assertions are inlined into it.
- Characters and classes of up to four ranges compare inline in the step, which dispatches on the program counter by binary search.
- Larger classes get a function that tests the ranges below U+0080 first and binary-searches the rest. There is one such function per distinct set of ranges, shared by every instruction that tests it.
- `^` and `$` under `multiline` call `line_start` and `line_end`, and `\b` and `\B` call `word_before` and `word_at`, the functions the interpreter uses.
- When no empty match is possible from the start, a skip function moves past ASCII bytes that cannot begin a match before a thread starts.
- Programs over 2000 instructions are compiled at run time by `compile` instead, so call depth stays bounded.

### Compression

`std/compress/codec` holds what the codecs share. Deflate, brotli, WebP lossless, zstd, LZ4, snappy, xz and bzip2 build on it where their formats agree.

#### Shared codec pieces

Each encoder is a core that writes into a `Buffer`.

| Name | Behaviour |
| --- | --- |
| `Buffer` | A growable byte buffer with an LSB bit packer, and every write to it is infallible. |
| `Encoder` | The shared `Encoder` runs the core's `feed`, `sync` and `close` steps, drains the buffer to its sink and rejects every call after `finish` with `Invalid`, so one-shot `compress` has no error to handle. |
| `BitReader` | Reads LSB-first bits from a slice or an `io.Reader` and fails with `Malformed` on short input. |
| `Tables` | Builds two-level lookup tables for canonical Huffman codes. |
| `Builder` | Computes length-limited code lengths. |
| `Chain` and `History` | `Chain` is a hash-chain match finder, and `History` is the sliding window that zstd and LZ4 index with it. |
| `decompress` | Runs the one-shot decode of a codec under an output limit. |
| `copy_match` | Copies an LZ77 match. |
| `read_varint` and `write_varint` | Code LEB128 integers, which `std/protobuf` and xz also use. |

#### DEFLATE, zlib and gzip codecs

DEFLATE, zlib and gzip decode stored, fixed and dynamic blocks with two-level lookup tables. They check Adler-32, CRC-32, ISIZE and the gzip header CRC, and they read concatenated gzip members.

The encoder uses greedy matching at levels 1 to 3 and lazy matching at 4 to 9, over a hash chain on a 32 KB window. It chooses dynamic, fixed or stored encoding per block. `flush` emits a sync flush so streamed output is decodable as it arrives.

#### xz codec

xz decodes:

- multiple blocks and streams
- the none, CRC32, CRC64 and SHA-256 checks
- LZMA2 chunks with their resets
- the delta filter

Other filters fail with `Unsupported`.

The encoder writes LZMA2 with a hash-chain match finder and lazy parsing. The dictionary runs from 256 KiB to 64 MiB by level. The encoder writes raw chunks where compression does not pay.

#### zstd and LZ4 functions

`std/compress/zstd` and `std/compress/lz4` each offer:

- `compress(data, level)` and `decompress(data, limit)`.
- A streaming `decode(reader, sink, limit)` that passes output to the sink one block at a time.
- A streaming `Encoder` with `encode`, `flush` and `finish`.

#### zstd decoder

The zstd decoder handles:

- every block and literal type
- 1- and 4-stream Huffman literals
- all four FSE table modes
- repeat offsets
- skippable and concatenated frames
- the content checksum

Its history grows with real output up to the smaller of the window and the remaining limit. Large windows therefore never allocate what the header claims up front.

#### zstd encoder

The zstd encoder finds matches with two hash chains keyed on 8-byte and 4- or 5-byte prefixes. The level sets the depth and lazy evaluation. It Huffman-codes literals and picks predefined, RLE or custom FSE tables per block by estimated cost. It falls back to raw or RLE blocks.

#### LZ4 decoder and encoder

The LZ4 decoder accepts linked and independent blocks, block and content checksums, content sizes and skippable frames. The encoder writes linked 64 KB blocks with a content checksum.

#### zstd and LZ4 unsupported inputs

Dictionaries and LZ4 frames with magic number `0x184C2102` fail with `Unsupported`.

#### Brotli codec

`std/compress/brotli` has the same API as zstd and LZ4.

#### Brotli decoder

The decoder handles:

- compressed, uncompressed and metadata meta-blocks
- block switching for literals, commands and distances, and all four literal context modes
- context maps with run-length coding and inverse move-to-front, and distance postfix and direct parameters
- the static dictionary with all 121 transforms, and windows of 10 to 24 bits

Output collects in a ring buffer that starts at 64 KB and doubles up to the window. The output reaches the sink at the end of each meta-block, when the ring wraps, and before each blocking read. The 122,784-byte dictionary is `embed`ded from `dictionary.bin`.

#### Brotli encoder

| Levels | Encoder |
| --- | --- |
| 0 | Writes uncompressed meta-blocks. |
| 1 to 11 | Run hash chains of depth 2^(level-1). They use lazy matching from level 4 and check against the four recent distances and their neighbours. They write 256 KB meta-blocks with Huffman-coded commands and distances. |
| From level 5 | They model literals in UTF-8 context mode with up to 3·level-11 clustered trees. |

A block that would grow is stored. `flush` ends the meta-block and pads with an empty metadata block so everything sent decodes.

#### Bit sets

`std/bits` holds a growable `Set` over `u64[]` words. Counts use a SWAR popcount. Scans step a word at a time. The scans are `next_set`, `next_clear`, `min`, `max`, `rank`, `select_at` and `members`.

#### Roaring bitmap containers

`std/bits/roaring` holds `Roaring`, a set of `u32` split into 65536-value chunks keyed by the high 16 bits. Each chunk is a sorted `u16[]` array of up to 4096 values, a 1024-word bitmap, or a list of runs.

- Arrays and bitmaps convert at 4096 values.
- Ranges create runs, and unions and intersections of runs stay runs.
- Every container tracks its cardinality, so `cardinality` is constant time.

#### Roaring set operations and run_optimize

Set operations come as new-set and in-place `_with` forms.

| Container | Set operations work |
| --- | --- |
| Bitmaps | Word by word. |
| Arrays | By galloping merge. |

`run_optimize` picks the smallest form per container by the Go library's rule.

#### Roaring serialization

`serialize` and `deserialize` implement the portable Roaring format byte for byte as Go `roaring/v2` writes it. They reject any inconsistent length, ordering or cardinality with `Malformed`.

#### Image type and formats

`std/image` holds an `Image` of width, height, `Format`, stride and pixel bytes. `Format` is one of `Gray8`, `GrayAlpha8`, `Rgb8`, `Rgba8`, `Gray16` and `Rgba16`, and 16-bit samples are big-endian.

#### Image pixels and operations

Pixels read and write as 8-bit `Rgba`, with operations converting through 16-bit intermediates. `get` outside the image is transparent black, and `set` there does nothing.

Operations convert formats, crop, flip, rotate, apply EXIF orientation, fill rectangles, composite straight alpha, and premultiply and unpremultiply. They resize separably on premultiplied values with nearest, bilinear, Catmull-Rom bicubic or Lanczos-3 filters.

#### Image decoding

| Function | Behaviour |
| --- | --- |
| `decode` | Picks a codec by signature and rejects images over 2^26 pixels. |
| `decode_limited` | Takes another limit. |

#### PNG codec

PNG decodes every colour type, bit depth and Adam7, checks every CRC, and encodes with per-row adaptive filtering. The result is:

| Image | Result |
| --- | --- |
| 16-bit gray without transparency | `Gray16` |
| Other 16-bit images | `Rgba16` |
| Gray | `Gray8` or `GrayAlpha8` |
| Colour with alpha or `tRNS` | `Rgba8` |
| Otherwise | `Rgb8` |

`decode_png` decodes only PNG and fails with `Unsupported` on any other signature. `gAMA`, `cHRM`, `sRGB` and `iCCP` chunks are not read, so samples are returned as stored. The PngSuite in `testdata/ui_png` checks the result against ImageMagick's.

#### JPEG codec

JPEG decodes baseline and progressive scans with restart markers, any sampling, grayscale and Adobe CMYK/YCCK. It uses libjpeg's integer IDCT and fancy upsampling, so output matches libjpeg byte for byte. It encodes baseline 4:2:0 with the standard tables scaled by quality.

#### GIF, BMP, QOI and WebP codecs

| Format | Behaviour |
| --- | --- |
| GIF | Decodes 87a and 89a and composites every frame by disposal method in `decode_gif_frames`. It encodes one frame with a given or median-cut palette. |
| BMP | Covers 1 to 32 bits, RLE, bitfields and every header version, and encodes 24 or 32 bits. |
| QOI | Decodes and encodes. The result is `Rgb8` or `Rgba8` by channel count. |
| WebP | Decodes lossless VP8L with every transform, the colour cache and meta prefix codes. Lossy VP8, alpha chunks and animation fail with `Unsupported`. |

#### Buffer and streaming functions of every codec

Every codec has buffer functions, a streaming decoder and a streaming encoder. The streaming decoder pulls from an `io.Reader`, pushes output to a sink and fails with `TooLarge` past a caller's limit.

### Web

`web.app()` holds routes and middleware.

#### Web routes

| Function | Behaviour |
| --- | --- |
| `get`, `post`, `put`, `patch`, `del` and `route` | Add handlers for paths with `:name` parameters and a trailing `*`. |
| `group` | Makes a sub-application with a prefix and more middleware that registers its routes on the same router. |

| Request | Answer |
| --- | --- |
| An unknown path | A 404. |
| A known path with another method | Runs the middleware of the first route with that path, then answers 405 with `Allow`, so `cors` answers a preflight for any route. |

#### Web typed endpoints

`endpoint[Req, Res]` and `query_endpoint[Res]` read a JSON body into `Req` and call a handler returning `Res ! web.Problem`. They write the result as JSON, or the problem as `application/problem+json`. Their schemas describe the operation in the OpenAPI document that `openapi` serves, along with path parameters and each route's security schemes.

#### Web middleware

Middleware is a struct of a `run` closure and an optional security scheme.

| Middleware | Behaviour |
| --- | --- |
| `api_key`, `basic` and `bearer` | Authenticate requests and set `Context.user`. |
| `sessions` | Keeps a map in a cookie sealed with AES-256-GCM, with the cookie name as associated data and an expiry inside. |
| `csrf` | Requires the session's token in `X-CSRF-Token` on unsafe methods. |
| `require_session`, `cors` (with the request headers it allows), `logger` and `compress` | Complete the set. |
| `compress` | Picks Zstandard, Brotli or gzip, in that order, from what `Accept-Encoding` allows. |

#### Web JWT and OpenID Connect verification

`jwt` verifies bearer tokens against a key set with `std/crypto/jose`. `oidc` does the same for an OpenID Connect issuer. It reads the discovery document and the JWKS and refreshes them hourly. It refreshes early, at most once a minute, when a token names a key it does not know.

#### Web OpenID Connect login routes

`oidc_login` adds two routes.

| Route | Behaviour |
| --- | --- |
| Sign-in | Redirects to the issuer with `state`, `nonce` and a PKCE S256 challenge kept in the session. |
| Callback | Checks the state, exchanges the code and verifies the ID token and its nonce. It hands the claims to the application. |

#### Web streaming responses

A handler streams a body by returning a `Response` with `stream` set to a `chan[u8[]]` and closing it at the end. The server writes each chunk as it arrives. It cancels `req.token` when the stream ends, the client disconnects or a write fails. It drops the stream for `HEAD` and bodiless statuses. Response compression skips streamed responses.

#### Web server-sent events

| Function | Behaviour |
| --- | --- |
| `sse.stream` | Sets `text/event-stream`, `no-cache` and `x-accel-buffering: no`. It flushes every event and sends a `: keepalive` comment every 15 seconds unless `keepalive` is 0. It stops when `req.token` is cancelled. |
| `Emitter.send` | Returns false after the client has gone. |
| `sse.format` | Removes CR and LF from `id` and `event` and NUL from `id`. |

#### Web static files

`web.static` decodes each path segment.

| Case | Result |
| --- | --- |
| `..`, an embedded separator, NUL or a bad escape | Answers 400. |
| Dotfiles, non-regular files and any file whose real path leaves the root | Answers 404. |
| A directory without a trailing slash | Is redirected. |
| A directory | Serves `index.html` or an optional listing. |
| `precompressed` | Picks `.br` then `.gz` by `Accept-Encoding`. |
| A multi-range or malformed `Range` | Gives the whole file. |
| Files | Stream in 64 KiB chunks. |
## Compiler

The compiler is written in Low, in `compiler/`. Pipeline:

| Stage | Module | Output |
|---|---|---|
| Lex | `compiler/lexer` | tokens |
| Parse | `compiler/parser` | `compiler/ast` tree |
| Check | `compiler/check` | types attached to the tree |
| Build IR | `compiler/ir` | SSA form, one value per machine word |
| Codegen | `compiler/amd64` | x86-64 GNU assembly |
| Codegen | `compiler/wasm` | WebAssembly module, for `browser/wasm32` |
| Assemble | `compiler/asm` | object: sections, symbols and fixups |
| Link | `compiler/link` | ELF or PE executable |

Building a program and running `low dev` need no external tool.

- `low` builds and runs on Linux and on Windows. A target is an operating system and an architecture: `linux/amd64`, `windows/amd64` and `browser/wasm32`. The default target is the host. `--target` selects one. The native targets are the amd64 ones, and the compiler assembles and links them in process. `browser/wasm32` goes from the SSA straight to a WebAssembly module, as `### WebAssembly` describes. `low asm` rejects it.
- On the Windows target, `std/sys` is built from its `_windows.low` files. These call kernel32, ws2_32, iphlpapi, shell32 and crypt32. Files, processes, sockets, DNS, terminals, the environment and the system root certificates therefore work there. `std/sql/sqlserver` calls secur32 for integrated authentication. `__syscall` is a compile error on the Windows target. The checker fails at the call with `windows/amd64 has no system calls, so __syscall cannot be called`. A standard library file that calls it and has no `_windows.low` counterpart therefore stops the build instead of panicking at run time.
  - A descriptor is a handle. Descriptors 0 to 2 name the standard handles. A socket's descriptor packs the socket in its low 32 bits and a manual-reset event in its high 32 bits. Sockets are overlapped and non-blocking. `wait_socket` issues an overlapped `IOCTL_AFD_POLL` request for the socket's read or write conditions with the descriptor's event. It parks on the event until the request completes. That is at once when the socket is already ready. `write_view` duplicates the socket with `WSADuplicateSocketW` and gives the copy its own event. A reader and a writer of one connection therefore wait on separate requests and events. `shutdown` cancels the socket's pending requests with `CancelIoEx`. This wakes the waiters on every descriptor of the socket. Closing the descriptor first reads and discards unread input, then closes both. It does so because Windows answers a close with unread input with a reset, which discards data the peer has not yet read.
  - Pipes made by `pipe(true)` and `pipe_for_child` are overlapped named pipes. Each is made by `CreateNamedPipeW` with `FILE_FLAG_OVERLAPPED` and a unique `\\.\pipe\low.<pid>.<random>` name, and has a `CreateFileW` client end. Their descriptor packs the handle in the low 31 bits, a tag in bit 31 and a manual-reset event in the high 32 bits, so it never collides with a socket's descriptor, whose bit 31 is clear. A read or write issues an overlapped `ReadFile` or `WriteFile` with the descriptor's event. When the call is pending, it parks on the event with `wait_readable`. It then collects the count with `GetOverlappedResult`. `ERROR_BROKEN_PIPE` ends a read with `0` and fails a write with `BrokenPipe`. Closing the descriptor through a view made by `wrap` cancels the pending operations with `CancelIoEx`. The cancelled operations fail with `NotConnected`. The guard then closes the handle and the event once the last operation has returned. Suppose a read or write has entered its lease but not yet issued its call when the descriptor is closed. That operation is not woken, and the descriptor stays open until it completes by itself. `open_pipe` opens an existing named pipe with `CreateFileW` and `FILE_FLAG_OVERLAPPED`, `SECURITY_SQOS_PRESENT` and `SECURITY_IDENTIFICATION`. It retries `ERROR_PIPE_BUSY` every 20 milliseconds for a second. It returns the same tagged descriptor. `pipe_for_child` makes the parent's end overlapped and the child's end synchronous, because the child's runtime calls `ReadFile` and `WriteFile` without an `OVERLAPPED`. `pipe(false)` stays an anonymous `CreatePipe` pair. A read from any other pipe or a file holds an OS thread until it returns. `wait_input` parks on a console handle until it is signalled and otherwise sleeps a millisecond between tries. `listen_pipe(path, first)` creates one instance with `CreateNamedPipeW` as a duplex, overlapped, byte pipe. The pipe refuses remote clients (`PIPE_REJECT_REMOTE_CLIENTS`) and has unlimited instances. When `first` is set, it adds `FILE_FLAG_FIRST_PIPE_INSTANCE`, so another process that already owns the name makes it fail with `Denied`. Its security attributes carry a security descriptor. The descriptor's only access control entry allows `GENERIC_ALL` to the user SID that `GetTokenInformation(TokenUser)` reads from the process token, built with `InitializeAcl`, `AddAccessAllowedAce`, `InitializeSecurityDescriptor` and `SetSecurityDescriptorDacl`. `accept_pipe` enters the view's guard and issues an overlapped `ConnectNamedPipe` with the descriptor's event. It parks on the event when the call is pending. It takes `ERROR_PIPE_CONNECTED` as a connection that already happened. It fails with `NotConnected` when closing the view cancels it.
  - `process.start` calls `CreateProcessW` with the command line quoted for the Microsoft C runtime, a UTF-16 environment block, and inheritable copies of the child's three streams passed through `PROC_THREAD_ATTRIBUTE_HANDLE_LIST`. The child therefore inherits nothing else. The process handle stands in for the pidfd. `wait` parks the green thread on it with `wait_readable`, which registers the handle with `RegisterWaitForSingleObject`. It then reads `GetExitCodeProcess`. `kill` calls `TerminateProcess`. A command with `group` is created with `CREATE_SUSPENDED | CREATE_NEW_PROCESS_GROUP`, assigned to a new job object with `AssignProcessToJobObject` and resumed, so every process it starts joins the job. `kill` on such a command calls `TerminateJobObject`, which ends every process of the job with status 1. The job handle closes when `wait` reaps the child. The job has `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`. Closing the handle therefore ends every process still in the job. This includes the system closing it when the parent exits for any reason. A parent that is itself in a job needs Windows 8 or Windows Server 2012. These nest jobs. `process.find` splits `PATH` on `;` and tries each `PATHEXT` extension.
  - Environment names compare without ASCII case. `os.home_dir`, `cache_dir`, `config_dir`, `data_dir`, `state_dir` and `temp_dir` read `USERPROFILE`, `LOCALAPPDATA`, `APPDATA`, `TMP` and `TEMP`. `net.system_config` takes up to three DNS servers and the primary domain from `GetNetworkParams`. `net.lookup` reads the hosts file under the system directory before it asks them through the same resolver as Linux. `x509.system_roots` reads the `ROOT` certificate store.
  - A refused connection to a loopback address fails at once rather than after the SYN retransmissions Windows would otherwise make.
  - Ignoring, resetting and raising signals, catching any signal other than Ctrl+C, Ctrl+Break, console close, logoff and shutdown, inotify watches, pseudo-terminals, setting a terminal's size, `chown`, peer credentials, binding to a device and `SO_REUSEPORT` fail with `Unsupported`. User and group IDs are -1 and the umask is 0.
  - Returning from a C call can wait out a collection. The wait keeps the thread's last-error value, so `GetLastError` after an extern call reads that call's error.
- The `low` command lives in `compiler/low`. `compiler.analyse` takes a path, a profile, an `Overlay` and an `ir.Runner` and returns a checked, lowered unit. `native.build` is the one build entry. For a native target it derives the `amd64.Platform` from the unit's target. It generates the assembly for the unit, assembles it with the unit's embedded files, links it with the linker its target uses and writes the executable. For `browser/wasm32` it writes the module `wasm.generate` returns. `native.analyse` is `compiler.analyse` with the native overlay and runner. `native.generate`, `native.patch` and `native.assembler` are the other entries of `compiler/native`. `low build`, `low run`, hot reload, the fuzz oracle and the Vulkan and Wayland generator tests all call them. Only the `dev` profile produces a hot-reloadable build.
- `low ir` prints the SSA form and `low asm` the generated assembly.
- Compilation is deterministic: the same source and target always produce the same SSA form, assembly and binary. No pass depends on map iteration order. Where order matters, the compiler walks slices or stacks.
- `parser.parse_source` is the one entry from source text to tree, for user files, the prelude, the standard library and generated code. `lexer.scan` returns a `lexer.Tokens`. It holds one pointer-free `lexer.Lexeme` per token over the source text. A lexeme carries the kind, byte offsets, line, column, character value and an index into a side table. Decoded strings, interpolation parts and doc comments live in the side table. The token array therefore holds no pointer, and the collector never scans it. The parser reads each token as a `lexer.Token` value that `lexer.token` builds from the lexeme and slices from the source. `lexer.lex` returns those values as a slice for the language server. The lexer lexes each interpolated `{}` expression into its own `lexer.Tokens` when it lexes the string. It keeps the `,width:format` suffix as text that `ast.parse_spec` parses. A `diag.Pos` is two words. The first is the `diag.Source` of its file, which every position one lexer makes shares. The second is the line and column packed into one word, read through `pos_file`, `pos_line` and `pos_col`.
- A lexer, parser or checker error is a `diag.Error` holding the position of the fault, and `low lsp` reads the position from it.
- `types.Universe` interns the pointer, slice, optional, raw pointer, channel, map, result and function types, and owns the built-in type names and the constraint names. `check.check` creates it with the checker's callback for new types and returns it. The checker lays out and instantiates types in three phases: declaring, laid out, and functions ready.

Modules:

- `compiler/parser` gathers a file's top-level statements into a `main` marked `top_level`, which `ast.declared` leaves out, so the language server lists no symbol for it and the walkers visit only its body. `compiler/check` checks that `main` before every other function, sets its error type at the first `?`, `fail` or `catch M`, and until then rejects a use of it from the statements, a drop or a field default.
- `compiler/compiler` finds the root, loads the entry module, then loads each imported directory once, breadth first, and rejects import cycles. Before it walks a run of pending imports it lists every module the run names and reads and parses their files through the `Overlay`'s `ir.Runner`, which also finds each literal path a file outside `std/` names in an `embed("...")` call; the checker's embedder reads those files through the runner before checking starts; the walk then takes each tree from that cache in the same order a sequential load would parse it, so the files and the first error do not depend on scheduling. It rejects top-level statements in any module but the entry module and the tested root, in two files of one module, and beside `fn main`. It leaves out the test files of every module except the one `low test` tests. For that module, `compiler/compiler/tests.low` loads the tested module under its own name with its test files, finds the test functions, and parses two generated files as privileged source: `__tests.low`, added to the tested module, whose `__tests()` lists each test with its name, function, file and line, and `__main.low`, the only file of the entry module, whose `main` runs that list with `std/testing`; neither has a path, so the language server never lists them. `compiler.check_tests_in` checks that build for the language server. `low test` builds each module's program with the release profile into a temporary directory and runs the program once per test, as `### Modules` describes. `compiler/low/test*.low` is the command: `test_options` lists the test options and reads their values, `test_select` expands the paths, `test_build` builds and lists a module and selects its tests, `test_run` supervises the child processes, `test_cache` keys, fingerprints, stores and replays results, `test_programs` records what a build reads through a wrapped `compiler.Disk` and stores and reuses programs, `test_slots` takes the machine's build and test slots, and `test_report` writes the text and JSON reports. `compiler.std_test_modules` lists the embedded standard library modules that have a test file. One loader reads the program, its imports, `embed` files and the embedded standard library, which is the map `embed_dir("../../std")` returns. Files outside the embedded library are read through a `compiler.Overlay`, which holds the `ir.Runner` the loader parses with and a `compiler.Disk` (a struct of function values that read a file, stat a path, list a directory, resolve a path's symbolic links and give the working directory, and say whether a path can start with a drive letter) and puts unsaved editor text over it for the language server, which shares the loader's `root` lookup and `parse_file`. `compiler/native` supplies the `Disk` over `std/file` and `std/sys`. `compiler/check` does no file IO.
- `compiler/compiler`, `compiler/ir` and the modules they import reach no operating system module: `std/sys`, `std/os`, `std/file`, `parallel`, `amd64`, `asm` and `link` stay out of their import closure, and a test in `compiler/tests/compiler` loads `compiler/compiler` and fails when one appears. The environment comes in as state. `ir.Runner` is a struct holding `each`, which runs a job for each index of a range, and `stage`, which `ir.mark` calls with the name of the compiler stage that starts; `ir.sequential` runs the indices in order and ignores stages, `native.runner` runs them on one green thread per CPU, and `native.traced` runs them as `native.runner` does while recording into a `parallel.Trace` each stage's start and each job's thread, start and end. `ir.build` builds the reachable functions in rounds over a frontier, each round through the runner and into a result slice indexed by position, and merges the results in discovery order, so the SSA does not depend on the runner. The functions of a round are lowered and finalised in at most 128 consecutive batches (`ir.batches`), each function inside its own `region` whose value is its `ir.Body`: the function's values, blocks, C signatures, jump tables and indirect-call signatures as flat records that name values, blocks and functions by index and types by id (`types.type_of_id`), with every string in one byte pool, so the body holds no pointer and the collector never scans it. Each function keeps its `ir.Body` in `Func.body`, and `ir.Program.all` lists every function by the index a body names it with. Specialisation, the call graph (each function's call targets found in parallel, then merged in function order into call sites listed by callee index), the inliner's summaries and candidates, critical edge splitting and the symbol scans read and edit the records; a rewrite that needs the pointer graph (simplifying a function, expanding inlined calls, finding loops, trying a constant substitution) runs on a copy that `ir.inflate_body` rebuilds and `ir.flatten_body` turns back into records, inside one `region` per function, so the copy's memory is freed and reused by the next function while it is still in the cache. `compiler.lower` then rebuilds every function's pointer graph on the heap with `ir.materialise`; `compiler.lower_flat`, which the `low` command uses, leaves the bodies flat, seals the universe (`types.seal`) and then gives the unit's syntax trees to `ir.lower_files`, which empties each file once it has listed the declarations and externs and drops the type names of their signatures, so the checker the universe's type hook held is garbage while the program is lowered, each tree is garbage once its functions are lowered, and a type created during lowering or later panics, and `amd64` rebuilds each function inside its code generation `region` with `ir.with_blocks`, while `wasm.generate` and `low ir` call `ir.materialise` first. `compiler/native` composes the runner, the `Disk`, the back end and the executable writer; the root programs (`compiler/low`, `compiler/hot`, the fuzz and generator programs and the tests) call it, and `compiler/lsp` takes its `Overlay` from the root that starts it. `native.Error` wraps `compiler.Error` and adds the back end, assembler and linker errors.
- `low build --trace file.json` compiles through `native.traced`. The stages are `load`, `check`, `lint`, `ir.build`, `specialise`, `inline`, `ir.finish`, `materialise`, `codegen`, which generates and assembles each function, `codegen.merge`, `layout`, `link` and `write`, with `assemble` after `codegen.merge` when `low asm` or the text path assembles one listing, `wasm` for `browser/wasm32`, and `hooks.before` and `hooks.after` around a project's build hooks. A green thread samples the process every 10 milliseconds while the trace runs: its user and kernel CPU time and its private and resident memory, from `GetProcessTimes` and `K32GetProcessMemoryInfo` on Windows and from `/proc/self/stat` and `/proc/self/statm` on Linux, where the CPU time counts in 10-millisecond ticks. `compiler/timeline` reads the trace. `timeline.chrome` writes the Chrome Trace Event JSON that Perfetto and `chrome://tracing` open: one row of stages, one row for the main thread with its serial work and each fan-out, one row per worker thread with each job labelled by its index, and counter tracks of the cores in use, split into user and kernel, and of the memory. `timeline.summary` writes, per stage, the wall time, the serial time, the fan-outs, the jobs, the longest job, the average number of lanes inside a job, the average number of cores the process used, the kernel share of that CPU time, the share of the fan-outs' lanes kept busy and the peak private memory. A lane inside a job counts as busy while it waits, so lanes above cores means blocked work. A matrix follows, with time across and the main thread and each worker down, every cell shaded by how much of it that thread spent inside work, under a ruler of stage letters, with a `cpu` row shaded by the share of the CPUs in use and a `memory` row shaded by the share of the peak private memory.
- `compiler/playground` is the compiler built for `browser/wasm32`. It reads one program from standard input as `main.low` through a `Disk` that holds no files, builds it with `ir.sequential` and the release profile, and writes the module to standard output. Warnings and errors go to standard error as `low build` writes them, and an error exits with status 1. It carries the whole embedded library, so a program the browser cannot build is rejected with the same message as natively.
- Every file records its module path and every source position its file, so errors name the file they come from.
- The checker keeps a scope of types and functions per module. The prelude belongs to the entry module. Its `__` names cannot clash with anything else. Every module sees the prelude's generic `__` functions and types. `std/container/deque` is therefore a wrapper over the prelude's `__Queue`, the ring buffer behind channel wait queues.
- A function in module `ui/text` becomes the symbol `fn_ui.text.shout`. Types display with their module path, such as `geometry.Circle`.

The runtime is `runtime.asm`, `sched.asm` and `gc.asm`, plus `linux.asm` or `windows.asm` for the target OS. Every target shares `runtime.asm`, `sched.asm` and `gc.asm`. The compiler emits the runtime with every program. A Linux program talks to the kernel through raw syscalls and ends the process with `exit_group`, so every thread stops. A dynamically linked Linux program ends with `exit`. A Windows program calls kernel32 through the `WIN` macro and ends with `ExitProcess`.

The runtime's data layout, thread-local access and Windows stack-bounds switching are defined once, in `layout.asm` and the target's `linux_header.asm` or `windows_header.asm`, as constants and macros such as `G_SCANLO`, `TLS_GET`, `TEB_ENTER_OS` and `MSG_BEGIN`. Generated code and patches use those names and macros. `generate` and `generate_patch` both prepend the headers. The code generator passes `RT_DYNAMIC` and the hot-reload constants (`HOT_BASE`, `HOT_SIZE`, `PATCH_BYTES`, `PATCH_ENTRY`, `PATCH_SIGNAL`) to the assembly as `.set` lines. It defines `HOT_INIT` as the call to `rt_hot_init` in a hot build and as nothing otherwise. It defines `VALUE_CODE dst, word` as the load of a function value's code address. That address is the word itself in a plain build and the `fn_table` entry it points to in a hot build. `rt_spawn` reads the function it starts through it. The Windows import table lists every function the Windows runtime calls through `WIN`, taken from `windows.asm`, plus the program's `extern` imports.

The contracts between the stages have one owner each:

- `compiler/builtin` numbers every builtin. The checker records the number on each call and `compiler/ir` lowers it through a table indexed by that number.
- `compiler/ir/runtime.low` lists every runtime routine generated code calls through a `runtime` instruction. For each it gives the symbol, the register convention (plain, growth or string equality) and whether the routine returns a scalar or nothing. A system call is its own `syscall` instruction.
- `compiler/prelude` names every prelude function the compiler calls or the checker's generated source calls. It names the fields `compiler/ir` reads from the struct a function returns (`__waiter` and `__chan_new`). It names which functions a map or channel type instantiates. The prelude is validated when it loads, so a missing or reshaped entry is an error that names it. `compiler/ir` reads field offsets from the checked types.
- `compiler/amd64` owns the target. A `Platform` value, `Sysv` for Linux and `Win64` for Windows, supplies the runtime and header text, the C argument classification, the registers a C callback saves, the entry symbol, how a system call is made, stack probing and the dynamic-library shims. `PROGRAM_ENTRY` and `PROGRAM_EXIT` in each header are the program's entry and exit. `amd64` exports the entry symbols, the hot-patch list and count symbols and the `.idata` section names that `compiler/link` and `compiler/hot` read.
- `compiler/amd64/convention.low` holds the Low register convention: argument, result, caller-saved, callee-saved, system call and growth registers, and the frame offsets derived from them.

### Scheduler

- Green threads run on OS threads, one per CPU in the process's affinity mask. Threads start as work appears, with `clone` in a static program and `pthread_create` in a dynamic one. A thread whose green thread is in a C call does not count against the limit, so another starts while C blocks.
- Run queues:
  - Each OS thread keeps a 256-entry run queue. A full queue spills to a global queue. `spawn` queues on the global queue when its own thread already holds 16.
  - A thread takes work in this order: expired timers; the global queue every 61st schedule; its own `runnext` and queue; a batch from the global queue (its length divided by the thread count, plus one, at most 128); ready descriptors; stealing.
  - A thief takes half of the victim's queue into its own. It reads the thread count once per round and visits only the threads below it. A thread that starts during a round is therefore first visited in the next. It tries the victim's `runnext` only in the last of 4 rounds. It takes it at once when the victim is in a C call or stopped, or when the running victim's `runnext` sequence has stayed unchanged for 2,000 timestamp-counter ticks. A victim is stopped when it is asleep on a runtime lock or held by a collection.
  - At most half of the running threads spin looking for work. With nothing to do, a thread sleeps on a futex or becomes the single poller that blocks in `epoll_wait` until the next timer.
  - Every 32nd schedule on an OS thread moves `runnext` to the tail of its run queue and takes the queue's head, so green threads that keep readying each other cannot starve the rest of the queue.
- A green thread reserves 1 MB of stack with a guard page below it. Pages are committed as they are touched, and finished green threads are reused. A function frame of a page or more is reserved one page at a time, touching each page. So is the area the program entry reserves for a `main` result returned on the stack. A frame therefore cannot step over the guard page. On Windows a page is committed only when the guard page above it is touched. Its context is its saved registers on its own stack. A stack that cannot be mapped panics with `out of memory`. On Linux a guard page that cannot be protected panics with `stack guard page: out of memory`. This happens, for instance, when the process is at `vm.max_map_count`. A fault in the guard page panics with `stack overflow`. Any other fault panics with `invalid memory access`, after a `fault:` line on stderr. That line gives the address accessed, the instruction's address, the stack pointer and the lowest address of the green thread's stack, each as 16 hexadecimal digits.
- Per-thread state lives in a 24-byte `.tbss` block directly below the thread pointer: the thread record at `fs:-24`, the running green thread at `fs:-16` and the preemption flag at `fs:-8`. A static program sets `fs` with `arch_prctl` and `CLONE_SETTLS`. A dynamic program uses the block glibc allocates for every thread. Generated code uses the fixed offsets, so hot reload patches need no TLS relocations.
- Preemption:
  - Every loop back-edge tests the preemption flag.
  - A monitor thread sets the flag every 10 ms on a thread whose green thread has run since the last check, and the collector sets it everywhere.
  - A set flag calls `rt_poll`, which saves every general register and `xmm2` to `xmm15` and yields or stops for the collector.
  - Code that holds a channel lock does not yield. Recursion without loops is not preempted.
- `park` and `ready` hand a green thread off with a per-thread lock and a wake token, so a wakeup that arrives before the green thread has finished parking is not lost.
- Wakeups:
  - `ready` puts the green thread in its OS thread's `runnext` slot, so it runs as soon as the current one parks. It bumps a per-thread `runnext` sequence. A green thread pushed out of that slot goes to the stealable run queue.
  - `ready` then calls `wakep`. `wakep` acts only when its OS thread has not woken anyone in the last 100,000 timestamp-counter ticks or has not scheduled since it last did. A pair of green threads that hand work to each other therefore stays on one OS thread, while a sender that keeps computing after `ready` always has an idle thread woken to steal its `runnext` within the 2,000-tick delay.
  - `wakep` does nothing while an OS thread is already spinning. Otherwise it wakes one idle thread, or starts a new one below the CPU limit. The woken thread counts as spinning until it finds work or gives up. A spinner that finds work while more is queued wakes a successor.
  - `spawn` always wakes a thread without the interval.
  - On Windows, where waking a thread is a system call that costs more, an OS thread that finds no work keeps checking the run queues as a spinner for up to 1,000,000 timestamp-counter ticks before it parks. It skips the spin while a descriptor wait is pending, stops spinning when a collection stops the world, and reruns the scheduler loop between checks while a timer is armed, so timers still fire. Work queued in that window is taken without a wake, and `wakep` sees the spinner and wakes no one.
  - The monitor thread moves the `runnext` and wakes a thread for the queue of an OS thread that sits in a C call or a blocking system call.
- In-call threads:
  - An OS thread in a C call, or in a system call that can block, is marked in-call. The collector treats that as stopped. `rt_world_stop` does not wait for it. On return from the call, the thread parks in `rt_m_running` while a collection is in progress.
  - The code generator emits `__syscall` inline only when its number is a constant on a short list of calls that never block on external events: `close`, `lseek`, memory mapping, signals, `pipe`, `dup`, socket creation and configuration, process creation and exit, time, identity, limits, `epoll_ctl`, `eventfd` and `getrandom`.
  - Every other number, and every non-constant number, calls `rt_sysblock`. It saves the argument registers on the green thread's scanned stack, marks the thread in-call, makes the call and leaves the in-call state through the same path as a C call.
  - The monitor counts in-call threads that have not scheduled since its previous visit, and `wakep` does not count them against the CPU limit, so work queued behind one finds a running thread.
  - A green thread blocked in `read` on a pipe therefore does not delay a collection started by another.
- Timers are a locked array of deadlines. `sleep` and `select`'s `after` arms add to it. The poller wakes through an eventfd when an earlier timer arrives. Descriptors are registered with `EPOLLONESHOT`.
- On Windows the poller waits on an event and on a high-resolution waitable timer (`CREATE_WAITABLE_TIMER_HIGH_RESOLUTION`, Windows 10 1803) set to the earliest deadline. The monitor thread waits on a second such timer. A deadline therefore lands within about a millisecond rather than on the 15.6 ms system tick. Where the timer cannot be created, the poller waits on the event with a millisecond timeout and the monitor calls `Sleep`. Both round up to the tick.
- Channels and `select` are generic prelude code over `__Chan[T]`. A `__Chan[T]` holds a lock word, a ring buffer and ring queues of waiters that keep their capacity.
  - A waiter is claimed with compare-and-swap before it is woken, so a `select` waiting on several channels and timers fires exactly once. Queued entries whose waiter has been claimed are skipped and dropped lazily.
  - An operation that completes at once allocates nothing. A buffered send or receive that needs no waiter runs inline in `__chan_send` and `__chan_recv`.
  - An operation that must wait allocates the waiter and value box under the lock it already holds. It queues them, releases the lock and parks. Each blocking send or receive therefore takes the lock once. Green threads to wake are readied after the lock is released.
  - `select` locks its channels in address order. It tries each arm with a probe that returns the green thread to wake, using a stack slot as the receive box. A ready arm allocates nothing.
  - When no arm is ready and there is no `else`, `select` allocates one waiter, queues it on every channel with the stack slots as boxes, releases the locks, adds the `after` timers and parks. When woken it deletes the timers and clears the slots.
- Channel lock word: 0 when free, 1 when held, 2 when held with sleepers.
  - `rt_lock` takes a free lock with one compare-and-swap.
  - A contender that finds the lock held makes 4 polling rounds. Each round is a `pause` loop that waits 8,000 timestamp-counter ticks, doubling every round, before it reads the word. Contenders therefore do not bounce the cache line that holds the channel's fields. The contender checks the stop flag each round and takes the lock with a compare-and-swap when it reads 0.
  - After the rounds it yields its OS thread once, then exchanges 2 into the word. Reading 0 means it holds the lock. Otherwise it waits on a futex until woken and exchanges again. While it sleeps, its OS thread counts as stopped, so a collection can proceed.
  - `rt_unlock` exchanges 0 into the word and wakes one sleeper only when the old value was 2.
- C calls run on the OS thread's system stack (8 MB in a static program). Around a C call the generated code pushes the callee-saved registers on the green thread's stack, records where the green thread stopped, marks the thread as in C and switches stacks. On return it marks the thread running again, waiting if a collection is under way.
- A C callback runs on the green thread's stack. Its thunk marks the thread running, switches back to the green thread's stack below the C call's saved registers, and locks the green thread to the OS thread until it returns. This is because the C frames live on that OS thread's system stack. If the callback waits, the OS thread waits with it. Callbacks must arrive on the thread that made the C call.
- A green thread locked to its OS thread, by a callback or by `sync.lock_thread`, keeps a lock count. When it parks, its OS thread is marked in-call, hands its `runnext` to the global queue, wakes another thread if work is queued or a timer or descriptor wait has no poller, and sleeps on the green thread's status word. `ready` wakes that OS thread instead of queuing the green thread. The OS thread marks itself running before resuming it.

### Linking

The in-process assembler and linker reproduce GNU `as` and `ld` and `clang` and `lld-link` output byte for byte. `compiler/asm` matches the object bytes of `as` for Linux and `clang` for Windows. `compiler/link` matches the segments, sections, symbols and loaded bytes of `ld` for ELF and the whole image of `lld-link` for PE. Layout choices that look arbitrary, such as `clang`'s padding and jump sizes or the XXH3 timestamp, are there to match. The oracle tests in `compiler/tests/asm` and `compiler/tests/link` check this against the pinned tool versions.

- A static program is assembled by `compiler/asm` and linked in process by `compiler/link`. `compiler/link` writes the executable without running `as` or `ld`. The program starts at the generated `_start`. `asm.assemble_pieces` clears each piece of the slice it is given once the piece is assembled. The ELF linker relocates each section's bytes in place. It returns a `link.Image` of parts (headers, sections, tables) at file offsets. `native.build` writes the parts in order with zeros in the gaps, so the executable is never copied into one buffer. `link.contents` joins the parts for callers that want the bytes.
- `asm.new_env` assembles the environment text once into an `asm.Env`. The environment text is the `.set` constants and macros every program shares. The `asm.Env` holds the instruction tables, the macros and the constants and is never written afterwards. `asm.assemble` builds its own `Env` for any text. `asm.assemble_chunk` assembles one function's text against an `Env` with origin 0. The text starts with `.p2align 5` and stays in `.text`. Every branch to a symbol outside the text is written `{disp32}`, which fixes it at the rel32 form, as GNU `as` and `llvm-mc` treat the same prefix. The result is an `asm.Chunk` of bytes, fixups, symbols and the offsets of the labels it was asked for. An assembler keeps the encoding of each instruction line that named no symbol, keyed by the line's text from the mnemonic on. A later line with the same text in an executable section takes those bytes without being parsed. `asm.assemble_pieces` takes `Text` and `Block(Chunk)` pieces in order and lays the chunks out as the text would be. A chunk adds an alignment fragment, its bytes as one fixed fragment, its symbols in first-mention order and its fixups. Relaxation of the rest therefore runs over the few fragments left. A chunk records whether it was assembled after data. The merge fails when the preceding bytes are not the same kind, because the padding of a function's first instruction depends on it.
- `compiler/asm` encodes the vector instructions in VEX form and in EVEX form (AVX-512 with the `F`, `BW`, `DQ` and `VL` extensions). EVEX takes `xmm`, `ymm` and `zmm` registers 0 to 31 and the opmask registers `k0` to `k7`. An operand carries `{k1}` merge masking (the first operand only, never `k0`). It carries `{z}` zeroing (a register with a mask). On the memory operand of a form that allows it, it carries `{1toN}` element broadcast, where `N` times the element size fills the vector. An 8-bit displacement counts units of the form's tuple size. That size is the vector length, the element size under a broadcast, or the element or 128-bit size of a broadcast source or a shift count. These are encoded in EVEX: the opmask moves, logic and tests (`kmov`, `kand`, `kandn`, `kor`, `kxnor`, `kxor`, `knot`, `kortest` and `ktest`, each in `b`, `w`, `d` and `q`), the lane arithmetic, minimum and maximum, logic and `vpternlog`, shifts by an `xmm` count, compares into a mask register (`vpcmp*`, `vcmpps`, `vcmppd`), `vpblendm`, the `vmovdqa32` to `vmovdqu64` moves, the `vpmov*2m` and `vpmovm2*` conversions and the `vpbroadcast` forms. A mnemonic that has both encodings assembles as VEX unless an operand is a `zmm` register, an `xmm` or `ymm` register from 16 up, masked or broadcast. A register from 16 up, a mask or a broadcast on a form without EVEX is an error. The vector code generator emits EVEX instructions only in its AVX-512 version (see Vectors).
- `asm.Options.independent_chunks` makes a 32-byte alignment the origin of the relaxation heuristic's stretch, so a function lays out the same wherever it lands. `native.build` sets it for every unit, so the text path and the chunk path give identical images. `native.assembler`, which the oracle tests use, leaves it off and keeps `as`'s section-wide stretch.
- `link.pe` writes a Windows image that the loader may relocate, with `DYNAMIC_BASE` and a `.reloc` section. `link.pe_fixed` writes the image of a dev build. It keeps the preferred base, strips relocations, clears `DYNAMIC_BASE` and returns the address of every symbol with the image.
  - The image is based at `0x400000`, below the hot-reload region. It has the layout of `ld -static` with separate code: a read-only header segment, then read-execute `.text`, read-only `.rodata`, and read-write `.data` and `.bss`. Each is page-aligned in memory and congruent with its file offset. `.tbss` has a `PT_TLS` segment. The stack is non-executable.
  - Symbols resolve across the sections of the one object. `PC32` and `PLT32` resolve alike. `Abs32` and `Abs32S` fail when the value does not fit. An undefined symbol fails the link and names the symbol.
  - The file carries section headers and a symbol table of the object's non-`.L` symbols plus `__bss_start`, `_edata` and `_end`, so `debug/elf`, `readelf` and `gdb` read it, and `low dev` reads it to link patches against it. `_start` initialises the heap and the scheduler, starts `main` on the first green thread and enters the scheduler on the process stack.
- A program that declares C libraries is assembled and linked in process too, by `link.Dynamic`, as a non-PIE executable that asks `/lib64/ld-linux-x86-64.so.2` to load its libraries. No C start files are linked.
  - The layout follows `ld`: `PT_PHDR`, `PT_INTERP`, a read-only `PT_LOAD` holding the headers and the dynamic tables, a read-execute `PT_LOAD` for `.plt`, `.plt.got` and `.text`, a read-only `PT_LOAD` for `.rodata`, a read-write `PT_LOAD` for `.dynamic`, `.got`, `.got.plt`, `.data` and `.bss`, then `PT_DYNAMIC`, `PT_TLS`, `PT_GNU_STACK` and `PT_GNU_RELRO` over everything the loader may write before it starts the program.
  - Each library's `-l<name>` is the first `lib<name>.so` in `/usr/lib`, `/usr/lib64`, `/usr/lib/x86_64-linux-gnu`, `/lib/x86_64-linux-gnu`, `/lib64`, `/lib` and `/usr/local/lib`, with symlinks followed. `DT_NEEDED` carries the library's `DT_SONAME`. `c` and `m` name `libc.so.6` and `libm.so.6`. Libraries appear in declaration order, then libc. A library that is missing, or is not a shared object, fails the link and names the library.
  - Every symbol a program calls or takes the address of is checked against the libraries' dynamic symbols, and one that no library exports fails the link. `.dynsym` records each import with its `GLIBC_*` version in `.gnu.version` and `.gnu.version_r`. Both `.hash` and `.gnu.hash` are written.
  - `PLT32` calls go through a lazy `.plt` entry with a `JUMP_SLOT`, or through a `.plt.got` entry when the symbol also has a GOT slot. `GOTPCREL`, `GOTPCRELX` and `REX_GOTPCRELX` resolve to a `.got` slot with one `GLOB_DAT` for each imported symbol. A slot for a symbol the program defines holds its address. A direct reference to an imported data symbol would need a copy relocation and fails the link.
  - The generated `_start` calls `__libc_start_main(main, argc, argv, 0, 0, rtld_fini, stack_end)` with no init or fini functions. That is all glibc needs: stdio buffers flush and `atexit` handlers run when the program exits. `atexit` and `at_quick_exit` exist only in `libc_nonshared.a`, so a program that declares them gets shims that tail-call `__cxa_atexit` and `__cxa_at_quick_exit`.
  - glibc's start-up code calls the generated C `main`, which does the same work as `_start`.
  - When Low's `main` returns, the program calls `exit(0)`, letting libc run its exit handlers. An error from `main` prints and exits with status 1.
- A Windows program is assembled in process with the padding and jump sizes of `clang`. `link.PE` links it into a PE32+ executable that starts at `start`. No import library is read.
  - The subsystem is `IMAGE_SUBSYSTEM_WINDOWS_CUI` unless `low build --subsystem gui` or the project's `gui` declaration of the program asks for `IMAGE_SUBSYSTEM_WINDOWS_GUI`. `IMAGE_SUBSYSTEM_WINDOWS_GUI` opens no console window. `low test` builds its harness for the console subsystem. Nothing else in the image differs. `print` writes through `GetStdHandle` and ignores a failed write. A GUI program therefore prints to a redirected standard output and discards the text when it has none. A program that is both windowed and a command-line tool therefore cannot be one executable. This is because under the GUI subsystem redirected standard streams work, output to an interactive terminal is discarded, and a shell does not wait for the program to exit.
  - The image is based at `0x140000000` with 4 KiB section alignment and 512-byte file alignment. It reserves 8 MB of stack and 1 MB of heap, and sets `DYNAMICBASE`, `HIGH_ENTROPY_VA` and `NX_COMPAT`.
  - Sections merge by the name before `$`, `.bss` joins `.data` after its initialised bytes, and `.idata` joins `.rdata`. The order is `.text`, `.rdata`, `.data`, the rest by name, then `.reloc`. Code padding is `int3`. `.rodata` is writable.
  - `.rdata` starts with a debug directory holding one `REPRO` entry. The header timestamp and that entry carry the low 32 bits of the XXH3-64 hash of the image taken with both zero. A program therefore always links to the same bytes.
  - `link.PeOptions.resources` holds what the image carries. That is the bytes of an `.ico` file, the bytes of an XML manifest and the version information strings, which the project's `icon`, `manifest`, `version` and `file_info` declarations fill. With any of them the image has a read-only `.rsrc` section, ordered with the other named sections, and the resource directory covers it. Resource types are in ascending order. Every resource is in language `0x409`, 8-byte aligned. The section holds these resources in this order. First, one `RT_ICON` resource per image of the icon file, numbered from 1 in the file's order. Second, one `RT_GROUP_ICON` resource numbered 1. Its entries are the file's directory entries with the image's number in place of its file offset. Third, one `RT_VERSION` resource numbered 1. Fourth, one `RT_MANIFEST` resource numbered 1. The version resource is a `VS_VERSIONINFO` block, the one `llvm-rc` writes for the same strings. It holds a `VS_FIXEDFILEINFO` with the file and product version from the leading numbers of the `FileVersion` and `ProductVersion` strings, the application file type and the `VOS_NT_WINDOWS32` operating system. It holds a `StringFileInfo` table `040904B0` with the strings in UTF-16 in the given order. It holds a `VarFileInfo` translation of English (United States) and Unicode. The block must fit in 65535 bytes. A key is not empty or repeated. `link.check_resources` reports what `pe` would. Without any resource the image has no `.rsrc` section.
  - The code generator writes the import tables as `.idata$2` to `.idata$7` with `__imp_` labels. The import directory covers the `.idata$2` descriptors and a null descriptor the linker adds after them. The code generator's `.idata$3` terminator follows that null descriptor, as `lld-link` lays out the same object. The import address table directory covers `.idata$5`.
  - `extern "c"` imports from `msvcrt.dll`. `extern "m"` imports from the UCRT maths set `api-ms-win-crt-math-l1-1-0.dll`. That set has the C99 functions such as `round` and `cbrt` that `msvcrt.dll` lacks. Any other `extern "name"` imports from `name.dll`.
  - Every `Abs64` fixup gets a `DIR64` entry in `.reloc`, one block for each 4 KiB page, so the loader can rebase the image. `ImgRel32` resolves to an RVA. `PC32` and `PLT32` resolve to rip-relative values. 32-bit absolute, GOT and thread-local references fail the link. So does an undefined symbol, which the error names.
  - `.incbin` paths are written with octal escapes for every byte outside printable ASCII, and `\` and `"` escaped. Drive letters, backslashes and non-ASCII names therefore assemble unchanged.

Prelude:

- `compiler/compiler/prelude/prelude.low` is Low source compiled ahead of every program. It holds:
  - UTF-8 decoding.
  - The string builder behind interpolation and concatenation.
  - The generic map table.
  - Integer formatting.
  - Float formatting by unrounded scaling against a table of 128-bit powers of ten from 1e-348 to 1e347, which finds the shortest digits that read back to the same value without allocating.
  - Formats and widths for interpolation. A float's exact decimal expansion is its mantissa times 2^e. It is built in base 10^9 limbs by repeated multiplication by 2 for a positive `e` and by 5 for a negative one. The decimal point is moved by `-e` places. An integer's digits are its magnitude. Rounding to the requested decimals or significant digits then works on those digits. It uses the exact remainder to break ties to even. So `f`, `e`, `n` and pictures need no approximation. NaN and infinities take the default text.
- `compiler/tablegen pow10` generates the table twice. One copy is the `__pow10_hi` and `__pow10_lo` functions the prelude holds. The other is `std/text/pow10.bin`, which `std/text` embeds for float parsing. It embeds the file because standard library modules see only the prelude's generic functions and types, not its other functions.
- Prelude names start with `__`, which user code cannot use. Four intrinsics are privileged names that the prelude and the standard library can use:
  - `__bits` gives a float's bit pattern.
  - `__str_view` turns a `u8[]` into a `str` without copying.
  - `__hash` hashes any `Hash` value.
  - `__zero` is the all-zero value of its expected type.
- A function's IR is built only when `main` reaches it through a call or a function address. So a program that formats no floats neither builds nor carries any of this code. Reachable functions are built in call order and emitted in declaration order.

Standard library:

- It lives in `std/` and is embedded in `low` with `embed_dir`.
- An import whose path is `std` or starts with `std/` is read from there, never from the project root.
- Its files are lexed like the prelude, so they may use `__` names and intrinsics, and they are left out of the sources `low dev` watches.
- `embed` in a standard library file reads from the embedded tree. The unit holds the bytes of each standard embedded file and each file the overlay holds unsaved, by path. It holds the content hash of every other embedded file. The assembler resolves `.incbin` from the held bytes first. Otherwise it reads the file in 64 KiB pieces straight into the section, failing when the hash of the appended bytes differs from the one the unit holds. WebAssembly generation reads each other file again through the overlay with the same check. The assembler creates `.rodata` with room for the bytes of every embedded file and a quarter more, so appending the files does not copy the section as it grows.

Standard library intrinsics:

- `__argc()`, `__argv(i)` and `__envv(i)` read the argument count, argument `i` and environment entry `i` (`null` past the end) as NUL-terminated `ptr[u8]`s. The start code saves them from the initial stack in a static program and from `main`'s arguments in a dynamic one.
- `__syscall(n, args...)` makes Linux syscall `n` with up to six integer, `bool` or `ptr` arguments. It returns `rax` as `i64`, a negative errno on failure. It is available only on `linux/amd64`, and the checker rejects a call on any other target.
  - It lowers to a `syscall` instruction. The code generator emits it inline for a constant number on the non-blocking list, and otherwise as a call to `rt_sysblock` (see the Scheduler section).
  - When the code generator emits `syscall` inline, it moves the arguments into `rax`, `rdi`, `rsi`, `rdx`, `r10`, `r8` and `r9` with the parallel move and executes `syscall`.
  - It is a call, so it keeps no value in a caller-saved register across it. That covers the `rcx` and `r11` the kernel clobbers.
- `__slice_ptr(x)` gives the data address of a slice or `str` as a `ptr[T]`.
- `__mulhi(a, b)` is the high 64 bits of the 128-bit product of two `u64`. It is an IR instruction that becomes one `mul`.
- `__hash_seed(i)` is word `i`, a literal 0 or 1, of the process's 128-bit hash seed. It is an IR instruction that becomes one load from `rt_hash_seed`.
- `__static_block()` is the address, as a `ptr`, of a 1024-byte block of process-wide memory. The block starts zeroed and belongs to the module that calls it. `__static_block()` is an IR instruction that carries the module name. It becomes the address of `rt_static_block.` followed by the module path with `/` written as `.`. On amd64 that is a zeroed `.bss` symbol. On `browser/wasm32` it is a region of the WebAssembly memory below the hoisted-literal cells. There is one block per module that calls it, in sorted module order, so two modules never share a block. Every green thread and OS thread sees the block of a module. The collector does not scan it, so it holds numbers and bytes, never pointers. Only the standard library can call it. `std/crypto` keeps the state of `random` in its block, guarded by a `__lock` word inside it.
- `__extend_by(mut b, n)` grows a `u8[]` by `n` bytes with at most one allocation. It returns the index of the first new byte, whose contents the caller overwrites.
- `__make(n)` is a zeroed slice of `n` elements of the expected slice type, allocated through the runtime's grow routine.
- `__memmove(dst, src)` copies `min(dst.len, src.len)` elements with the runtime's overlap-safe `rt_memmove` and returns the count. It takes a `str` source for a `u8[]` destination. `rt_memmove` loads up to 64 bytes into registers before storing them and uses `rep movsb` beyond that.
- `__clock_nanos(id)` returns `clock_gettime(id)` in nanoseconds, or 0 on failure, through `rt_clock_nanos`. `rt_clock_nanos` calls `__vdso_clock_gettime` when start-up found it from `AT_SYSINFO_EHDR` and the vDSO's dynamic symbol table (DT_HASH or DT_GNU_HASH). Otherwise `rt_clock_nanos` makes the syscall.
- `__context()` reads the current green thread's context slot as the expected pointer type. `__set_context(p)` replaces it. `__has_context()` tells whether it is set. `spawn` copies the parent's slot into the child, and the collector marks every live green thread's slot.
- `__os_error()` reads the current green thread's operating system error slot, an `i64` that starts at 0. `__set_os_error(code)` replaces it. `io.errno` and the Windows error mapping store each error number they turn into an `io.Error`. This lets a named variant, which folds several numbers, be paired with the number the system returned. A spawned green thread starts with 0. `std/os` reads the slot through `error_code`. It returns the number of `Os(code)`, or the stored number when it maps to the variant and `none` otherwise.
- `__serialize`, `__deserialize` and `__schema` are the derivations described below, allowed only in `std/serde`.

The garbage collector is a non-moving mark-sweep collector in the runtime:

- The runtime reserves 80 GiB with `mmap` and `MAP_NORESERVE`. The heap is the first 16 GiB and its pages are committed as they are touched. Region memory is the 64 GiB after it.
- Object layout:
  - Each object has a one-word header. Bits 3 and up hold the object's size in words, header included. So the word is the size in bytes, with its low three bits used as flags. Bit 0 is always zero, bit 1 marks a free chunk and bit 2 marks an object with no pointers to scan.
  - A bitmap records the first word of every object and free chunk. This resolves pointers into the middle of an object, such as a `mut` parameter aimed at a heap field.
  - Two summary bitmaps above it hold one bit per non-empty bitmap word and one per non-empty summary word. Finding the start behind an interior pointer skips empty spans 32 KB and 2 MB at a time. It does not read every bitmap word between the pointer and its object's header.
  - A second bitmap of the same shape holds the mark bit of each object. It is zero outside a collection.
  - The start bitmap, the mark bitmap, the first summary bitmap, the mark stacks, the region tables, the thread stacks and the per-thread structures are touched sparsely or in small steps, so they are excluded from transparent huge pages on Linux. One touched byte would otherwise make a 2 MB page resident.
- Allocation buffers:
  - Each OS thread allocates objects of up to 255 words from its own buffer of up to 16384 words without taking the heap lock. It bumps a pointer, writes the header and sets the object's start bit.
  - Only the start-bitmap words at the buffer's unaligned edges are set atomically, with `lock or`. The other words are updated with a plain `or`. The summary bits are tested first and set atomically only when clear. Each thread remembers the last bitmap word whose summary bits it set, so allocation skips the summary update while it stays in that word. The memory is reset whenever the thread takes a new buffer.
  - A thread whose buffer is full takes the heap lock and refills the buffer. It tries these sources in order: the smallest free chunk large enough in the size-class bins below, the exact-size free lists from the largest down, then the end of the heap. After releasing the lock, it zeroes the buffer's dirty pages in one pass. On Linux it first maps the buffer's pages with one `madvise(MADV_POPULATE_WRITE)` when any of them is clean. This way a buffer carved from returned memory does not fault once per page.
  - A collection turns every thread's unused buffer tail into a free chunk before marking. An allocation that would leave a single word in a buffer takes that word too, so the sweep finds a header at every object boundary.
- Free chunks:
  - Free chunks of 64 words or more sit in 256 size-class bins, four per power of two. A 256-bit bitmap marks the non-empty ones. Bins 0 to 127 hold chunks whose pages may be non-zero (dirty). Bins 128 to 255 hold chunks known to be zero (clean).
  - An allocation scans up to eight chunks of its own bin for the smallest that fits. It then takes the first chunk of the lowest non-empty bin above it. It prefers a dirty bin to a clean one of the same class. It splits off the remainder.
  - Chunks under 64 words sit in exact-size free lists that only buffer refills read.
  - Objects over 255 words take the heap lock and allocate from the bins, then from the end of the heap, and are zeroed after the lock is released.
- Dirty pages:
  - A bitmap with one bit per 4096-byte heap page records which pages may hold non-zero words. Pages never handed out are zero from the operating system and are clean.
  - An allocation zeroes only the dirty pages of its range and then marks the whole range dirty. So a large zeroed object that is never written costs no resident memory.
  - Every free chunk of 256 KB or more whose interior pages are all clean is filed in a clean bin.
- Returning memory:
  - After a collection restarts the world, and before the heap end moves for an allocation, a walk of the dirty bins returns the whole interior pages of every dirty free chunk of at least 1 MB to the operating system on Linux with `madvise(MADV_DONTNEED)`, clears their dirty bits and files the chunk in a clean bin. While the last collection's live words are under half the heap, the floor is 256 KB. So a heap that has shrunk to a fraction of its size does not keep its free fragments resident.
  - The walk runs on the thread that collected, so returning memory is not part of the pause. It takes one chunk out of its bin under the heap lock. It releases the lock for the `madvise` and takes it again to file the chunk. So other threads allocate while pages are returned. The walk stops once another collection has run. The thread of that collection walks the bins that collection rebuilt. The chunk in flight is recorded in its thread's `M_RETURNING`, and every collection sets its mark bit before marking, so a collection that runs during the `madvise` keeps it out of the free lists it rebuilds.
  - The walk starts at the first bin that can hold such a chunk and visits dirty bins only, so a chunk is advised once and not again until it is reused and freed.
  - Resident memory holds the live objects and the chunks too small to return, rather than also the garbage the last collection freed.
  - On Windows the pages stay committed and chunks stay dirty. The heap and its bitmaps are committed in steps of 16 MB as the heap end moves, under the heap lock, so a growing heap makes few `VirtualAlloc` calls while other threads wait for the lock.
- Collection runs when the words allocated since the last one exceed the words that survived it, with a 4 MB floor. An allocation the free chunks cannot hold moves the heap end. A thread's allocation buffer counts by the size of the chunk it receives, so refilling from small free chunks after a sweep counts no more than the memory handed out. Region allocation does not count.
- Regions (`region { ... }`, see `### Regions`) allocate outside the heap:
  - Each green thread holds a pointer to its current region, or 0. The allocation fast path does not test it: while a region is current, the OS thread's allocation buffer has its end moved to `M_ALLOC_HELD` and zeroed, so the buffer's bounds check fails and the allocation reaches the slow path, which tests the region first. `rt_region_enter` and `rt_resume` hold the buffer when the green thread that runs has a region, and `rt_region_leave` (back to no region) and `rt_resume` give it back otherwise. A collection retires the buffer from whichever field holds its end. A region's allocations take the heap lock only to take a chunk from the pool or the range. Entering and freeing a region register and unregister it under a lock of the region list alone, so they do not wait for the heap lock.
  - A region is a descriptor and a list of chunks. A chunk is a power-of-two multiple of 64 KiB in the region range, aligned to its size. The first chunk of a region is 64 KiB and later ones 1 MiB, small enough that a region's resident memory follows what it writes rather than whole huge pages; an object over 32 KB gets a chunk of its own of the smallest size that holds it. The 64 KiB units skipped to align a chunk go to the pool of 64 KiB chunks. Scan and noscan objects bump through separate chunks, with the header format of heap objects, so the descriptor sits at the start of the first noscan chunk.
  - A chunk records in `CH_DIRTY` the highest address its earlier uses wrote; the memory above it is zero, and a newly committed chunk is clean. Each bump stream's end (`R_SCAN_END`, `R_NOSCAN_END`) is a zero frontier rather than the chunk's end: an allocation that crosses it takes the slow path, which zeroes the next 4 KiB (`REGION_ZERO_STEP`) or up to the object's end when that is further, never past the mark, and moves the frontier to the chunk's end once it passes the mark. Allocation then writes the header and nothing else, and memory is zeroed just before it is used, while its lines are about to be written anyway. An object with a chunk of its own is zeroed up to the mark when the chunk is taken. Linux commits a chunk with `mprotect(PROT_READ|PROT_WRITE)` and frees it with `madvise(MADV_DONTNEED)` followed by `mprotect(PROT_NONE)`; Windows uses `MEM_COMMIT` and `MEM_DECOMMIT` inside a range reserved with `MEM_RESERVE`.
  - `rt_region_enter` creates the region and makes it current, `rt_region_leave` makes its parent current while the region stays registered, and `rt_region_free` unregisters it and releases each chunk. `rt_region_suspend` returns the current region and makes none current, and `rt_region_resume` makes the region it is given current again.
  - Each OS thread keeps up to 32 freed 64 KiB chunks and up to 12 freed 1 MiB chunks, linked through a table indexed by 64 KiB unit, and takes a chunk of either size from its own list before the pool, without the heap lock. Freeing a chunk raises its mark to its last object and leaves its memory as it is; a kept chunk stays committed, so its reuse makes no system call. An OS thread that parks with kept chunks waits at most 5 ms (`REGION_KEEP_IDLE_NS`) before decommitting them onto a pending list, which the next collection appends to the quarantine list, so the kept chunks of idle threads are not resident while the rest of the program runs. A `-c debug` build sets `RT_REGION_GUARD`, which also protects a kept chunk: Linux with `mprotect(PROT_NONE)`, so its pages stay resident and reuse costs one `mprotect`, and Windows by decommitting it. Any other freed chunk is decommitted and its range appended to a quarantine list, so memory a finished region used is not resident while later stages of the program run. The system calls run outside the heap lock.
  - Every live region's scan chunks, from the first object to the bump pointer, are conservative roots of a collection. Noscan chunks are never read. A candidate word that points into region memory is not followed; it sets a bit for its 64 KiB unit in a hit bitmap that each collection clears.
  - A quarantined range returns to the per-size pool of chunks after a collection in which no candidate word pointed into it, and a chunk is taken from the pool of its size before the range's bump pointer moves. When the range is exhausted, a chunk request collects once and retries, then panics with `region: address space exhausted`.
  - The 64 GiB range holds 65536 chunks of 1 MiB. A program that needs 3.3 GB of live region memory at once uses about 5% of it, and freed ranges are reused after one collection.
  - A fault on a quarantined chunk, or on a kept chunk in a `-c debug` build, panics with `region: use of freed memory`. The panic comes from the signal handler on Linux and a vectored exception handler on Windows.
  - `spawn` inside a region copies the call's environment from region memory to the heap before the green thread starts, and the new green thread has no current region.
  - Regions hold no state the hot-reload patches touch: a patch changes function code through `fn_table`, so an open region stays open.
- `LOW_GCTRACE=1` prints one line per collection to stderr: `gc N pause_us= mark_us= sweep_us= live_kb= heap_kb= slow_allocs= refills= workers= return_us=`. The line comes from timestamps around the stop, mark, sweep, restart and the return of free pages after it, from per-thread counters of buffer refills and from the number of threads that marked, the collecting thread included.
- `LOW_GC=off` never collects. `LOW_GC=stress` collects after every 8192 words allocated.
- An allocation whose type holds no pointers is noscan. This covers `new` of a pointer-free value, and the backing arrays of slices whose elements hold none, such as `u8[]`, `i64[]` and `f32[]`. A noscan object is marked, but its contents are never read. Other objects are scanned word by word.
- Allocation and pinning take a heap lock. A collection stops the world. It sets a stop flag and every thread's preemption flag. It then waits until each other thread is stopped at a safepoint, idle in the scheduler or in a C call. It holds those threads there until it finishes. Threads waiting for a lock stop for the collector while they wait.
- Marking and sweeping run on every thread the collector holds in its stop path. These are the threads stopped at a safepoint, waiting for a lock or returning from a C call, and the idle threads parked in the scheduler, which the collector wakes. At most one worker per CPU, and at most 16, take part. The collecting thread is one of them. Threads in a C call or a blocking system call do not join and do not delay the collection. Some collections start OS threads after they restart the world. This happens when a collection leaves at least 2 M words (16 MB) live and its mark showed work for more workers. The mark showed such work when a worker took a batch from the shared pool, or when a mark that ran alone held at least 32 entries on its stack at one of the checks made every 4096 marked objects. The collection starts threads until the threads outside a C call number one per CPU, at most 16. They wait idle in the scheduler, so the next collection wakes them as workers. A heap that is one linked list shows neither, so a program that holds one keeps the OS threads it has. The collector publishes a phase by incrementing the stop word and waking its waiters. It does not restart the world until every worker has left the phase.
  - When the previous collection left under 256 K words (2 MB) live, or no other OS thread exists yet, the collecting thread marks and sweeps alone without waking any thread, and sets mark bits with a plain `bts`.
  - A parallel mark makes the next collections mark and sweep alone when it leaves at least 16 MB live and no worker takes a batch from the shared pool. A heap that is one linked list is such a case. Every 64th collection uses two workers and returns to full parallel marking if one of them takes a batch, and the first collection after one that leaves twice the live words of the mark that made the switch marks with every worker.
  - Each worker has its own mark stack and prefetch ring in a slot of the mark reservation. The mark bit of an object and the hit bit of a region unit are set with `lock bts`. Only the thread that sets a bit pushes the object and adds its size to its own live count. The counts are summed when marking ends.
  - A worker that finds its stack empty takes a batch from a shared pool under a lock. A worker that sees an idle worker and an empty pool moves half of its stack, oldest entries first, to the pool. An idle worker spins with `pause` for 4096 rounds and then sleeps on a futex word. A donation and the end of marking bump that word before waking its sleepers. Marking ends when every worker is idle and the pool is empty, decided under the pool lock.
  - Goroutine stacks and regions are claimed by atomic index. Pins and `rt_cells` are scanned by the collecting thread.
  - Sweeping splits the heap bitmap into ranges of 8192 bitmap words (4 MB of heap), claimed by atomic index. Each range builds its own bins and small lists with their tails. It records its first start if that object is dead, its first marked object and the start of a free run left open at its end. The collector then walks the ranges in address order. It completes the runs that cross a range boundary and prepends each range's lists. So the free lists are the ones a sequential sweep builds, whatever the thread timing.
- Roots come from stack maps.
  - At every call the code generator records the frame words that may hold a heap pointer at that point: the saved callee-saved registers, spill slots of values that may be pointers and are live across the call, and every frame slot. The frame slots are contiguous, so a map lists them as one range of frame words that the collector scans with `rt_scan_range`. The maps form a table sorted by return address, and call sites whose maps are identical share one copy.
  - A value passed to a C call that may be a pointer is live until the call returns, so a collection during the call keeps memory whose last use is that argument.
  - The words of `rt_cells`, one per hoisted literal, are scanned as roots in every collection.
- Green-thread stacks:
  - Every green thread records where it stopped: the return address, stack pointer and frame pointer of the Low code that entered the runtime, and the lowest stack word holding registers it saved.
  - The collector scans each live green thread's saved registers conservatively. It then follows the thread's `rbp` chain, looking up each frame's return address and scanning only its mapped words. It scans the green thread's first frame conservatively.
  - Frames without a map, such as code patched in by hot reload or a loop interrupted by a preemption check, are scanned whole. So is the frame of their caller, whose outgoing argument area holds the stack-passed parameters of the unmapped frame, before the caller's map is read. A green thread that has not started is scanned whole.
  - Values in callee-saved registers are found in the save slots of whichever later frame saved them, or among the registers saved when the green thread stopped.
- A callback thunk crosses C frames by segment. Its frame record holds the Low caller's frame pointer and return address, so the `rbp` chain leads from the thunk straight back to the caller. Its frame keeps the bounds of the C frames on the system stack and of the registers the C call saved. The thunk's call site has a map of count -1, which tells the collector to scan both regions conservatively before following the chain.
- Every candidate word, mapped or not, is checked against the heap before it is followed, so a map that names a word holding an integer is harmless.
- A `ptr` counts as a possible heap pointer, so a `cstr` copy or a pinned object stays alive while Low holds its address. `pin` records the address in a table of 65536 roots that every collection scans. `unpin` removes it.
- Marking resolves a candidate word first by testing the start bit of the word before it. That word is the header when the candidate is an object's first field address. Otherwise marking falls back to the summary walk. Candidates are prefetched through a ring of pending headers before they are marked, so cache misses overlap. Marking sets the object's bit in the mark bitmap and adds its size to the live word count. Newly marked noscan objects are not pushed on the mark stack.
- Sweeping walks the start bitmap, jumping over empty words with the summary bitmaps. So its cost follows the number of objects and not the address span of the heap.
  - It rebuilds the free lists and bins from empty.
  - Within a word, the start bits without a mark bit are dead and the rest are live. Sweeping jumps between the two kinds of run to coalesce adjacent dead objects into free chunks.
  - A chunk keeps only its first start bit, and the word's mark bits are cleared once it is done.
  - A trailing free run returns to the end of the heap.

The derivations are generated as source:

- When the checker meets `__serialize(v, w)`, `__deserialize[T](r)` or `__schema[T]()` with a concrete type, or a `print` or interpolation of a type other than a number, `bool` or `str`, it writes a function for that type in Low text. It binds the type and each type it mentions to names like `__X0`.
- It parses the text with the prelude's privileges. It marks the text trusted so it may read and build private fields. It checks the text like a generic instance.
- Calls inside it to the same intrinsics derive the types they reach. Each derived function is cached by type.
- The serialization of a type is `fn(w: Writer, mut path: i64[], v: T)`. `path` holds the addresses of the pointers being written. Each is pushed before a pointer's target is written and popped after it. A pointer already on `path` panics with `serde: cycle through *T`. `__serialize(v, w)` starts with an empty path. The derived code recurses through `__serialize_into(w, mut path, x)`.
- The rendering of a type is `fn(mut b: u8[], mut path: i64[], v: T)`, which appends to `b` through the prelude's `__append_*` functions. `path` holds the addresses of the pointers being rendered. Each is pushed before a pointer's target is rendered and popped after it. A `print` or interpolation creates an empty buffer and path, calls the function and uses the buffer as the text. An interpolation appends into the buffer it is composing. The same `__append_*` functions render a number, `bool` or `str`, so `print` has one text path. It builds the text and writes it with `rt_print_str`. `__render(v)` gives the text of any value with a `str` quoted, for `std/testing`'s `format`.
- The copy of a region's result is derived the same way when the type holds pointers, strings, slices or maps: `fn(mut path: i64[], v: T) -> T`. It copies each value to the allocator that is current when it runs. That is the one outside the region. `path` holds the addresses of the pointers being copied, as for the rendering. A pointer already on it panics with `region: cycle through *T`. The private builtin `__copy_into(mut path, v)` calls the derived function for the type of `v`. Types it cannot copy are errors at the `region`.
- Bound names resolve in type positions, struct literals and variant selectors.
- A field reference `T.field` is checked as `serde.Field[T, F]` and lowered to the field's name and index as constants.

### IR

- Generic templates never reach the IR.
  - The checker keeps an untouched copy of each generic function's syntax tree, clones it for each distinct set of type arguments and checks the clone with those types bound. The result is an ordinary function named like `max[i64]`. Generic structs and enums become ordinary types named like `Stack[str]`.
  - Instances are made on demand, checked from a work list, and limited to 1000 per template.
  - Symbols escape every character outside `A-Za-z0-9_.` as `.` and its hex code, so `max[i64]` is `fn_max.5bi64.5d`.
- SSA is built directly from the checked tree with Braun et al.'s algorithm: variables are read and written per block, phis are created lazily and sealed once a block's predecessors are known, and trivial phis are removed.
- Every value is one 64-bit word. A struct, enum or result of n words is n SSA values.
- A parameter or result type of at least `ir.by_reference_words` (64) words that is neither a vector nor holds a C struct passes by reference. The parameter is one pointer word to memory the callee only reads, and the result is written through a pointer the caller passes as the last parameter word. Callbacks and `main` keep the word convention. The caller passes the memory an argument already lives in when its words are one unchanged run read from a by-reference parameter, a call's result or an immutable local, and otherwise stores the words into a fresh frame slot. An immutable local of such a type lives in memory the same way, and a run of at least 8 words read from one such place is copied with one `rt_memmove`.
- Narrow integers are kept sign-extended (`i32`) or zero-extended (`u32`, `u16`, `u8`) in their word. Floats are kept as their bit pattern, `f32` zero-extended. Arithmetic, conversion and comparison values carry their type.
- Frame slots and SSA variables:
  - A variable passed as `mut` to a function, captured by a closure, or of a C or vector type lives in a frame slot (or a heap box when captured) and is accessed with loads and stores. Every other variable is pure SSA.
  - `push`, `__extend`, `__extend_by` and `delete` take their `mut` operand in place. On an SSA variable they read and write its words (a slice's pointer, length and capacity) as SSA values, so a slice that is only pushed to, indexed and passed by value stays in registers.
  - Lengths and capacities these builtins compute are typed integers and never count as pointers.
- Integer division carries its zero and overflow checks inside the instruction. A constant divisor other than 0 and -1 needs neither check.
- A call to a C function becomes a `ccall` carrying its C signature: the symbol, the type of each argument, which arguments are `mut` places, and the result type. `compiler/amd64` classifies the signature for the target. On Linux the System V AMD64 rules classify each argument word:
  - Integers, `bool` and pointers take `rdi`, `rsi`, `rdx`, `rcx`, `r8` and `r9`.
  - Floats take `xmm0` to `xmm7`.
  - An `extern struct` of at most 16 bytes is split into eightbytes that go to vector registers when every field in them is a float and to integer registers otherwise.
  - A larger struct, or one that does not fit in the registers still free, goes on the stack.
  - Variadic `f32` arguments are widened to `f64`, and `al` holds the number of vector registers used.
  - A struct result of at most 16 bytes comes back in `rax`, `rdx`, `xmm0` and `xmm1` by class. A larger one is written to a buffer whose address goes in `rdi`.
  - Narrow integer, `bool` and `f32` results are re-extended, and so are narrow places passed as `mut` once the call returns.
- On Windows the Win64 rules apply. Every argument takes one of four slots, `rcx`, `rdx`, `r8` and `r9`, and a float also takes the matching `xmm0` to `xmm3`; a variadic float is passed in both. The slots after the fourth go on the stack above 32 bytes of shadow space. An `extern struct` of 1, 2, 4 or 8 bytes passes in its slot. Any other struct is copied into the caller's outgoing area and passed by address, and a struct result of another size is written to a buffer whose address goes in the first slot. Results come back in `rax` or `xmm0`.
- Every frame keeps `rsp` 16-byte aligned at its calls, as C requires: frames are rounded to 16 bytes, and `_start` rounds the space it reserves for `main`'s result.
- An `extern struct` value is its C bytes, one word per eightbyte. Its locals and parameters live in frame slots. A field is loaded or stored at its own width. A struct literal whose fields are all constants is its constant words; any other struct literal, and a field of a computed value, goes through a temporary slot. A struct whose size is not a multiple of 8 is stored and loaded up to its last byte, with 4-, 2- and 1-byte accesses for the final partial word, so writing a nested field or an element of a `view` never touches the bytes after it.
- `defer` keeps a stack of deferred statements per block and lowers them again at every exit from that block.
- Drop and `using`:
  - A struct's `drop` becomes a function `T..drop` whose parameters are the struct's fields.
  - Every droppable type gets a generated `T..release` that calls `T..drop` and then the releases of its droppable fields.
  - A `using` binding adds a deferred `if flag { T..release(x) }` to its block. A move reads the value and clears the flag.
  - A flow-sensitive pass over each checked function body finds moves. It joins moved states across branches and runs a loop body a second time when an iteration moves a binding.
- `==` on a struct is the conjunction of its fields' comparisons. On an enum it compares tags, then branches on the tag to compare only that variant's fields, since the other payload words are undefined. An `extern struct` is stored in two frame slots and compared field by field at each field's offset and width, so padding is ignored.
- Function values and closures:
  - A Low function value is two words: a code address and an environment pointer. Code reached through a function value takes the environment as parameter word 0.
  - A closure is checked in place and becomes a function named after its parent, such as `counter..c1`. Its environment is a heap array, or 0 when it captures none. Environment words follow the order of the closure's captures.
  - A captured variable lives in a heap box allocated when it is bound if it is assigned after its binding, passed as `mut`, a C struct or a vector. Every access, in its own function and in closures, loads and stores through the box. The environment holds the box pointer in one word.
  - Any other captured variable stays an SSA variable of its function and the environment holds its words inline. The closure reads them into SSA variables on entry.
  - `spawn` binds its callee and arguments to such variables, so a spawn allocates the environment and nothing else.
  - A named function used as a value goes through an adapter, `name..value`, which drops the environment and calls it.
- A call through a function value moves the code address to `r10` and calls it, passing the environment and the arguments like any call. A call through an `extern fn` value is a `ccall` with the same plan as a named C function.
- A C callback is a thunk, `cb_name`, emitted once per program for every function used as one. It saves the C argument registers. It re-extends narrow integers and `bool` from them and from the C stack. It calls the function with Low's convention on the green thread's stack and moves a float result to `xmm0`. On Windows it also saves and restores `xmm6` to `xmm15`, which C treats as callee-saved.
- Every loop's back-edge block is marked. The code generator ends it with the poll check described under Code generation.
- `spawn f(a, b)` becomes a block that binds the callee and the arguments to fresh locals and passes a closure calling `f` with them to `rt_spawn`.
- `region { body }` becomes `rt_region_enter`, the body, `rt_region_leave`, a call of the derived copy function on the body's value when the checker attached one, and `rt_region_free`. Nothing in the body leaves the block, so each of the four runs exactly once.
- Sends, receives, `close` and channel loops call the prelude's instances for the element type. `select` calls the prelude's per-arm helpers and the runtime's `rt_lock_all`, `rt_timer_add` and `rt_park`.
- `funcaddr` gives a function's code address, its thunk, or an `extern` function's address through the GOT. Functions reached only through `funcaddr` are kept.
- In dev builds every C function gets a global trampoline, `cfn_name: jmp name`, and calls go through it, so a patch can link against the running binary.
- An `embed` becomes an address and a length. The data is emitted with `.incbin` under a label named by the file's content hash. So a changed file changes the code of every function that embeds it. Embedded files follow every other `.rodata` datum. So the strings an `embed_dir` map is keyed by sit together, and reading them does not fault in the pages of every file. A loop over a read-only table fills an `embed_dir` map. The table has one row per file, holding the key's address and length and the file's address and length. So the function that builds the map stays a few blocks however many files it embeds.
- Every value records whether it may hold a heap pointer.
  - Loads, parameters and call results take this from their type's word layout, where an enum word counts as a pointer if any variant puts one there.
  - Constants, typed arithmetic, string literals and frame addresses never hold pointers.
  - Untyped address arithmetic and anything unclassified count as pointers.
  - A phi holds a pointer if any input may.
- A `str` is two words and a slice three. An element access computes `pointer + index * stride` after a `check` instruction. The `check` compares unsigned and jumps to the index panic. Packed elements load with `movzx`, `movsxd` or a 32-bit `mov` and store at their width.
- Maps:
  - A map `[K: V]` is a pointer to the prelude's generic `__Map[K, V]`, whose `entries` slice holds `__Entry[K, V]` records of a live flag, the key and the value.
  - An open-addressing index of entry numbers with linear probing finds them. Deleted entries leave tombstones until the next rebuild. The rebuild compacts the entries in order and resizes the index to keep it at most three quarters full.
  - Integer, `bool` and pointer keys hash their word and `str` keys their bytes, both with the seeded multiply of the Maps section.
  - Every concrete map type instantiates `__map_new`, `__map_get`, `__map_set`, `__map_delete`, `__map_len`, `__map_next`, `__map_key` and `__map_value` for its key and value types. Map literals, lookups, assignments, `delete`, `.len` and loops become calls to them.
  - A compound assignment such as `m[k] += v` is a get, the operation on the value or zero, and a set.
- `push` and the formatting helpers call the runtime's grow routine and the prelude. The buffer behind an interpolation lives in a frame slot passed as a `mut` argument. A `push` that finds the slice full grows it to twice the capacity plus 4.
- Counted push loops:
  - A counted loop `for i in a..b` whose whole body is `xs.push(v)` becomes one reservation instead of a push per iteration. `xs` must be an SSA variable. `v` must be made only of literals, other variables, field reads, conversions, unary operators and non-trapping arithmetic or comparison on numbers.
  - The trip count is `b - a` as an unsigned word when `a < b`. The loop is skipped otherwise. A trip count above 2^40 bytes of elements calls the grow routine with an impossible size, so the program stops with the out-of-memory panic.
  - The slice grows once to exactly its length plus the trip count, or not at all when the capacity already suffices, and its length becomes that sum. When `xs` is known to be empty the growth is unconditional.
  - A value that is zero in every word (a zero constant or a conversion of one) needs no stores in a freshly grown buffer, since the runtime returns zeroed memory. Otherwise, and in the no-growth case for a zero value, the elements are written with a counted loop.
  - A value that calls a function, can trap or reads the slice itself keeps the push loop.
- String building:
  - The prelude appends a whole string to a `u8[]` with `__extend`. `__extend` grows the buffer at most once and copies the bytes with the runtime's `rt_memmove`. So concatenation, interpolation, `bytes` and `str(b)` copy in bulk.
  - An integer is formatted by counting its digits against powers of ten, growing the buffer once with `__extend_by` and writing the digits from the end.
  - A concatenation chain and an interpolation, including interpolations and chains nested inside either, are flattened into one list of pieces. Adjacent literal pieces merge.
  - Every piece is evaluated first, in order. The result buffer is allocated once with the grow routine. Its size is the sum of the literal bytes, the lengths of the `str` pieces and the widest rendering of each number (20 for an integer, 25 for a float, 5 for a `bool`). No rendering exceeds that width. The longest float is a negative value with exponent -6 and 17 significant digits, `-0.0000012345678901234567`.
  - A chain with no number pieces copies each piece with `rt_memmove` at a running offset. With number pieces, the buffer lives in a frame slot. `str` pieces are copied at its length, and numbers go through the prelude's `__append_*`.
  - A piece with a width adds the width's size to the estimate. It is appended into the buffer and then padded in place by `__align`. `__align` counts code points from the byte offset where the piece started. A literal piece with a width is kept as a piece rather than merged into the text around it.
  - A piece with a format calls `__append_decimal` (`d`), `__append_radix` (`x`, `X`, `b`, which masks a signed value to its type's width), or `__append_int_layout` or `__append_float_layout` (`f`, `e`, `n` and pictures). The call passes the format's digits, decimals, grouping and mode as constants. An `f32` is widened to `f64` first, which is exact.
  - A piece with a format or a derived rendering can write more than its estimate. A composition containing one appends its literal and `str` pieces with `__append_str`, which grows the buffer when needed, instead of copying at a running offset.
  - String ordering calls the prelude's `__str_cmp`.
- Unreachable blocks are pruned after construction.

Frame-slot promotion:

- A frame slot is promoted to SSA values by the optimiser when its address is used only by word-sized loads and stores at word offsets inside it, and is never passed to a call, stored, compared with another address or reached through arithmetic with a variable. Its stores disappear, each load becomes the value stored on every path that reaches it, and phis join the paths. A slot read before any store reads an undefined value.
- An `rt_memmove` of a constant whole number of words between two slots counts as word loads and stores: when either slot is promotable the copy becomes one load and store per word first.
- A call returning by reference whose result slot is used only by the call and by one copy of the whole result into another frame slot, or into the function's own result pointer, writes its result there directly, when no argument of the call points into that memory and nothing reads it between the call and the copy.
- A slot of a C or vector type, a `select` box, a lock array and a slot with more than one address value stay in memory.
- When code generation reads a function back from its flat body, a run of at least 8 stores, to consecutive words of one place, of words loaded from consecutive words of another becomes one `rt_memmove`, when nothing can have changed the source since the loads. That holds for a by-reference parameter; for a frame slot outside any loop that one store, copy or call result writes, before the loads, and that is otherwise only read or passed by reference; and for loads in the run's own block when every write between them and the run goes to a frame slot or parameter that cannot overlap the source. Two frame slots never overlap, a frame slot never overlaps a parameter, and a function's result pointer never overlaps one of its by-reference parameters.
- After inlining, a `mut` argument whose callee is inlined therefore lives in registers.
- Loads and stores carry the flags raw (a narrow or tail access of a C struct, or `load` and `store`) and trap (`load`, which is never discarded).

Optimiser:

- An optimiser runs over each function's SSA before phis are classified.
  - It keeps each value's type, float width and may-hold-a-pointer flag. A value that replaces another holds a pointer if either did.
  - A pass leaves the control-flow graph alone, so phi inputs and predecessor lists stay valid while it runs.
  - Between passes a branch on a constant becomes a jump to the taken successor. Blocks left unreachable are pruned together with their predecessor entries and phi inputs. Trivial phis are removed. The pass runs again until no branch folds.
- The optimiser builds a dominator tree and walks it from the entry, so each value sees only facts established by its dominators. `ir.immediate_dominators` computes the tree with the semi-NCA algorithm over a depth-first numbering. It finds semidominators by Lengauer and Tarjan's path-compressed forest. It then finds each immediate dominator by climbing the partial tree from the block's depth-first parent. Its cost follows the edges, not the depth of the tree. So a block with thousands of predecessors along a chain stays cheap. The code generator numbers machine blocks the same way and calls it too. Predecessors that the entry cannot reach are ignored.
- Constant folding and simplification:
  - Copies are propagated, and integer operations, comparisons and `not` on constants are folded at their own type. `i32` results are sign-extended, and `u8`, `u16` and `u32` results are zero-extended, exactly as the generated code would leave them.
  - Division and remainder of constants by zero or by -1 are never folded, so the runtime panic and the overflow case stay in the instruction.
  - Adding, subtracting, or-ing, xor-ing or shifting by zero, multiplying by one, and multiplying or and-ing by zero reduce to an operand or a constant. Multiplication by a power of two becomes a shift.
- Division and remainder by a constant other than 0 and -1 become arithmetic on the 64-bit word. This is exact for narrow operands because they are kept extended.
  - A divisor of 1 is the operand or zero.
  - Unsigned powers of two are a shift or a mask.
  - Signed powers of two add `(x >> 63) >>> (64 - k)` before the arithmetic shift so the result truncates toward zero. A negative divisor negates the quotient.
  - Any other divisor multiplies by a magic number with the high-multiply instruction, signed (`imul`) or unsigned (`mul`). When the magic number needs 65 bits, it adds the operand back or halves and re-adds it. It shifts, and for signed values adds the dividend's sign bit.
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
  - A variable stepping by one cannot overflow below a bound it is compared against. Any other step cannot overflow when the sum of the upper bounds of the variable and the step stays within the type's range.
  - An upper bound for a value comes from a constant, from the width of a narrow unsigned type, or from a dominating signed comparison against a value that has an upper bound. Strict comparisons subtract one. For a sum or product of two non-negative values, the bound comes from their upper bounds when the result stays within the type's range. Only comparisons whose block dominates the use contribute.
  - A check `[i, n]` with a constant `n` is removed when `i` is non-negative and its upper bound is below `n`.
  - Branch facts also record `i <= n` on the false edge of `i < n` and the true edge of `i >= n`. These never remove a check and never prove a step cannot overflow.
  - A length that is a phi satisfies `n >= L` when every input satisfies it on its incoming edge. An input satisfies it by being `L`, by being a constant at least `L`, by being a phi that does, or by being a value that a fact on the input's dominators bounds below by `L`. When `i < L` is known and `n >= L` holds, a non-negative `i` removes the check. Otherwise the check compares against `L` instead of `n`, which keeps the same panics.
  - A slice filled by a counted `push` loop from 0 to `L` has length `L` when `L > 0` and 0 otherwise. Indexing it by a counter below `L` therefore needs no check.
- Dead-value removal:
  - Values nothing reads are removed, together with groups of values that only read each other, such as a phi and the add feeding it.
  - Only these operations are removed: constants, strings, embeds, undefined values, phis, add, subtract, multiply, and, or, xor, shifts, negation, `not`, complement, conversions, `sqrt`, `floor`, comparisons, function addresses, high multiplies, trailing-zero counts, loads, float division, and integer division or remainder by a constant other than 0 and -1.
  - Every other instruction stays, including calls, stores, checks, vector instructions, frame slots and divisions that can panic.
  - An unread load through a reference, a slice element or a frame slot is removed. This is safe because `*T` values are never null and an index is checked before its element is loaded. A `load` through a `ptr[T]` stays, so a null raw pointer traps even when the value is unused.
- Float arithmetic, negation, `sqrt`, `floor` and conversions between integers, `f32` and `f64` are folded on constants with the result the SSE instruction produces.
  - `f32` operations round to single precision.
  - A float to integer conversion of a NaN or an out-of-range value gives the minimum 64-bit value.
  - Unsigned 64-bit conversions are left to the instruction.
  - A result that is NaN is never folded, so the sign and payload of a NaN come from the hardware.
- Loop-invariant code motion:
  - It works on natural loops, innermost first. It finds them from back edges whose header dominates the latch. It works in loops with a single outside predecessor that ends in a plain jump. That predecessor is the preheader.
  - Only values whose operands are defined outside the loop or already hoisted move: multiply, and, or, xor, shifts, negation, `not`, complement, conversions, `sqrt`, `floor`, function addresses, high multiplies, trailing-zero counts, float division, and add, subtract or shift that is not the constant-offset address arithmetic an addressing mode absorbs.
  - Loads, checks, calls, stores and integer division stay where they are. A loop that runs zero times therefore executes nothing it would not have executed.
  - A constant a hoisted value reads is copied into the preheader, because the code generator defines a constant that is not an immediate at its first use.
- Strength reduction replaces `i * c` by an add induction variable when `i` is a basic induction variable of a loop with a constant step. It replaces `i * c + d` derived from it the same way. In the preheader, the new variable starts at the start value multiplied by `c`. On the back edge it steps by `step * c`. The arithmetic is 64-bit wrapping, so the result equals the product for every start, step and trip count, including overflow and negative steps.
- Devirtualisation and specialisation:
  - A call through a function value whose code address is a function address becomes a direct call that keeps the environment argument.
  - Before inlining, a function loses its function-address and environment parameter words when it meets all of these conditions. It is never called through a function value. It is not `main` or a callback. It receives the same function address and the same constant environment from every direct call site. The parameters become the constants, the arguments leave every call site, and the function and its callers are simplified again. Calls from the function to itself that pass its own parameters unchanged count as agreeing. This runs for up to four rounds, so a function value handed down through several calls reaches the call that uses it.
  - A function of at most 150 values that calls a function-value parameter or hands it to another call is cloned per function address when its direct call sites pass two or more different function addresses for that parameter. It is cloned when its direct call sites pass one function address with different environments.
    - Each clone takes the call sites of one function address and is named `name..withN.target`, where `N` is the first parameter word.
    - The clone has that word as the constant. It drops the environment word too when every one of those call sites passes the same constant. It is simplified, so its call through the function value becomes direct.
    - Calls from the clone to the original that pass the parameters unchanged call the clone. The original stays for the remaining call sites.
    - A function gets at most 4 clones, and clones add at most 4000 values to the program.
  - The same conditions let an integer parameter become a constant when it receives one and the same constant from every direct call site. This happens only when it pays. The function is simplified as a copy with the constants substituted and as a copy without. The substitution is kept when the copy with it has fewer checks, blocks or values besides constants and parameters.
- Inlining:
  - Release builds inline direct calls bottom-up over the strongly connected components of the call graph, one level at a time. A component's level is one more than the highest level among the components it calls. The functions of a level find their sites and expand them in parallel. The program budget is charged between those steps in the level's discovery order, so the output does not depend on scheduling.
  - A callee is eligible when it:
    - is not part of a cycle;
    - is not a callback;
    - costs at most 48, or at most 150 when the site is its only reference. A callee's cost counts its values besides constants, parameters, results, undefined values and loads read only by its returns, less one for every two of its parameter and result words, and is at least 1 when it counts any value;
    - has at most one loop and an entry block no jump returns to;
    - uses no vector instruction.
  - Frame slots, `ccall`s and runtime calls are cloned with the body, and the callee's slots are appended to the caller's.
  - A function's references are its call sites and address uses in the program, plus one for `main` and each runtime root. They are counted before inlining, and an inlined site gives its reference to the references the callee's body holds. The original of a callee inlined at its only reference becomes unreachable and is dropped. After inlining, the program keeps the functions reachable from its roots and empties the body of every other one.
  - A callee that costs at most 14, or one inlined at its only reference, is free. Any other site charges its cost to the caller, which grows by at most 240, and to the program, whose budget is a fifth of its value count before inlining, at least 6000 and at most 40000. The last quarter of the program budget is kept for call sites inside loops.
  - A caller inlines up to four rounds. Each round takes its sites inside the deepest loops first, then the smaller callees, then the earlier values; splits the call's block; clones the callee's blocks with parameters bound to the arguments; forwards a single return's values or joins several returns in phis; and simplifies the caller. Calls that an inlined body or a devirtualised call exposes are inlined in the next round.
  - Loop back edges keep their marks, and checks and divisions keep their panics.
  - Dev builds do not inline, specialise or hoist literals, so hot reload patches see every function and every literal's data.
  - A `compiler.Profile` chooses the target, whether the IR is optimised and whether code generation is hot-reloadable. `release(target)` optimises and generates plain code. `dev` skips the optimiser and generates `fn_table` code.
- Literal hoisting:
  - A release build hoists a non-empty slice or map literal whose elements are all constants: integer, float, `bool` and `str` literals, negated numbers, `none`, enum variants without payload and non-C struct literals of constants.
  - The checker marks a literal read-only when it is the operand of an index, a `for` or `.len`, or is bound by a `let` whose every use is such an operand. An assignment, a `mut` argument or any other use anywhere in the target path of an element or field makes the binding non-read-only.
  - Each hoisted literal owns a word of `rt_cells`. The code loads the word. When it is 0, the code builds the literal as usual and stores the pointer. So the first evaluation builds the literal, and a racing thread at most builds a second equal copy. The miss path calls `rt_region_suspend` before it builds the literal. It calls `rt_region_resume` after it stores the pointer. The literal is therefore on the heap even when the first evaluation is inside a `region` block.
  - A read-only literal uses the cell's pointer, as a slice of its length for a slice.
  - Any other use copies it. A slice is copied with one grow call that moves the template's bytes. A map is copied with the prelude's `__map_clone`, which copies the entry array and the index. In that case, a literal below the copy thresholds of the Slices and Maps sections is built each time.
- Block merging and jump threading:
  - A block that ends in a jump to a block with no other predecessor and no phis is merged into it. The pass rewrites the successors' predecessor lists to name the absorbing blocks once, after every merge.
  - A predecessor that passes a constant skips a block that branches on a phi, or on its negation, and holds nothing else but constants that only feed its successors' phis. The predecessor jumps to the successor the constant selects. It takes a copy of each constant that successor's phis read from the block. Use counts follow each moved edge. The threaded predecessors leave the block together once all are visited. Blocks are visited successors first, so a chain of such blocks is threaded in one pass.
  - The phi that joins `a && b` takes `false` from the edge where `a` is false, and the phi that joins `a || b` takes `true` from the edge where `a` is true, so a condition built from them threads into branches on its operands.
  - The backend threads a block the same way when a predecessor copies a constant, or a value its own predecessor branched on, into the register the block branches on.

### Code generation

- `generate` produces the functions on one green thread per CPU. Unless the build is hot-reloadable, the functions run in at most 128 consecutive batches (`ir.batches`). For `low asm`, each batch runs inside one `region` block. Its value is the assembly text, data chunks and stack maps of the batch's functions. Outside a hot-reloadable build, each function is generated and assembled inside its own `region` block. That block's value is the function's `asm.Chunk`, data chunks and stack maps, so its text never leaves the region. Inside it, generation runs in a nested `region` whose value is the function's text, data chunks and stack maps. Assembly runs in another nested `region` whose value is the chunk. The memory of instruction selection and register allocation is therefore freed before assembly starts. That memory is freed while it is still in the cache. It does not count toward the collector's trigger, and a collection scans only the regions in progress. A function assembled after data cannot know so until the function before it is generated. So the first function of each batch is assembled as if after code and keeps its text. It is assembled again after the batches finish when the last function of the batch before ends in data. The stack maps become a `.rodata` chunk built from the maps' return labels. The chunks merge in function order through `asm.assemble_pieces`, together with the runtime, data and start text. A hot-reloadable build generates and assembles sequentially outside regions as text, because the function table keeps the symbol names it is given.
- Phis and layout:
  - Critical edges are split and each phi becomes a parallel copy at the end of every predecessor. The copies are emitted with cycles broken through `r11`.
  - An argument whose live range ends at that copy prefers the phi's register, and the phi prefers the registers of its arguments, so a loop-carried value usually needs no move.
- Block layout:
  - Blocks are laid out in reverse post-order, visiting the false successor first so the true successor follows its branch.
  - Inside a loop, the true arm of an integer `==` comparison against a constant is treated as rare when it holds a loop, a call, a return or an unreachable end. The blocks between the branch and its false successor move after the last block of the innermost loop, so the common path falls through to the back edge. A light arm stays in line, because the extra taken branches would cost more than the arm when it is the common path.
  - A jump or branch to an empty block goes to that block's final target. A jump to the next block in layout is omitted. A conditional branch whose true target is next is inverted. A block holding phi copies is never empty.
- Loop latches:
  - When a loop's header holds only a fused compare and branch, its latch ends with that compare and a conditional branch to the body instead of a jump to the header. The compare and the branch come after the poll check when the block has one.
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
  - A block that only branches on a boolean phi is threaded. A predecessor that passes a constant jumps to the successor the constant selects. A predecessor that passes the condition of its own branch jumps to the matching successor. Any other predecessor branches on the value it passes.
  - Evaluation order and short-circuiting of `&&` and `||` operands are unchanged.
  - A branch on the result of a `not` or a comparison defined in its own block uses that operand directly, so the `not` and the materialised boolean are not emitted.
- Spilling:
  - A value's spill weight is the sum, over its uses and definitions, of 8 raised to the block's loop depth, capped at depth 4. Loop depth counts the natural loops containing the block. They are found with the dominator tree the optimiser also uses. A cycle with more than one entry is not a loop.
  - When no register is free the allocator spills the conflicting value with the lowest spill cost, breaking ties by the later end of its live range.
  - The allocator runs up to three times per function. The spill cost is set to the weight, to the weight divided by the length of the live range and to the weight divided by its square root. It keeps the assignment whose spilled values have the lowest total weight plus 2 for each callee-saved register it uses. The weight alone wins ties. A run whose total is 0, or that never compared two spill costs, is kept, and the runs after it are skipped, because they would assign the same registers.
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
  - The pair `(x << k) | (x >> (w - k))` is one `rol` when it meets all of these conditions. Both shifts act on one `u32` or `u64` value `x` of width `w`. `k` is a constant between 1 and `w - 1`. The shifts are read only by the `or`. They appear in either order. No other combination is a rotate.
- `exp` is a runtime routine: range reduction by `ln 2` split into high and low parts, a degree-13 polynomial, and scaling by two powers of two so subnormal results stay in range.
- A comparison of a `str` against a literal of up to 16 bytes is inline. It is a length compare, then one compare of the bytes at the pointer against an immediate of 1, 2, 4 or 8 bytes, or two when the length is none of these. It ends in `sete`. The literal's address and length are never materialised, and the comparison is not a call. Any other `str` equality moves the arguments into the argument registers. It returns 0 without calling `rt_str_eq` when the lengths differ, and calls it otherwise.
- Register allocation:
  - Registers are assigned by linear scan over each value's live segments. A value therefore does not occupy a register where it is dead, and a register's holes hold other values. Each machine register value has a number. Block liveness is the list of live values in number order. It is found per value by walking back from each block that uses it before defining it, until a block that defines it. Its cost is therefore the size of the live sets. The instruction selector counts uses and definitions in slices indexed by them. The allocator keeps one list of assigned values per register. The three spill-cost orders stop at the first allocation that costs nothing.
  - `rcx`, `rsi`, `rdi`, `r8` and `r9` hold values that are not live across a call. `rbx` and `r12` to `r15` hold values that are, and are saved in the function's frame when used. Values that find no register are spilled to the frame. `rax`, `rdx`, `r10` and `r11` are scratch.
  - A growth call is `rt_grow_scan` or `rt_grow_noscan`, according to whether the new buffer may hold pointers. It takes the old pointer, the used bytes and the new size in `rax`, `rdx` and `r10`. It returns the new pointer in `rax`. It preserves every other general register, so a value that cannot hold a pointer stays in `rcx`, `rsi`, `rdi`, `r8` or `r9` across it. A value that may hold a pointer is treated as live across a call: the routine saves the preserved registers below the scanned part of the stack, where the collector does not read them. The routine records the caller's return address and frame pointer like any runtime entry. The old buffer stays reachable through the words it keeps in the scanned region.
  - Float register values are allocated from `xmm2` to `xmm15` by the same scan. No `xmm` register survives a call, so a float live across a call is spilled. Spilled floats are not in the stack maps. A function that uses vectors gets no float registers, so its float operations go through `xmm0` and `xmm1`.
- Register preferences:
  - A value used as a call argument prefers that argument's register, a parameter prefers the register it arrives in, and a copy and its source prefer each other's registers.
  - The result of a two-operand add, subtract, multiply, and, or, xor, shift or float divide prefers the register of its first operand. For a commutative operation it prefers the register of its second, so the instruction can work in place. Where it cannot share, it avoids the register of the operand it still has to read.
- Wrapped saves:
  - A function whose calls all lie below one block saves the callee-saved registers it uses at the head of that block instead of in the prologue. That block is the nearest common dominator of its call blocks. The function restores the registers only on returns reached through it.
  - The paths that skip the block use no callee-saved register and have no calls or polls. They are at most 8 instructions, copies included, where they rejoin the shared tail. The shared tail is duplicated for them.
  - A value defined on a skipping path and live into the block is copied at the end of that path into a fresh value, and the block's head copies it into the original. The original lives only below the block, so a loop counter initialised on the light path stays in a caller-saved register.
  - A value defined in the region below the block and live into it that crosses a call is given a fresh value there. A stack parameter's fresh value keeps the parameter's arrival slot as its home.
  - Save slots keep their frame positions, and the stack maps name every save slot at every safepoint.
  - A vector function is not wrapped. The block and the tail are found with the same dominator tree that the optimiser uses.
- Frame modes: a function has one of three.
  - A full frame is built in the prologue.
  - A function with a wrapped region builds its frame, `push rbp`, `mov rbp, rsp` and `sub rsp`, at the head of the block that holds the saves. The paths that skip the block return with a bare `ret`. This needs every skipping block to be free of anything that reads or writes the frame: no spilled value, frame address, slot access, parameter beyond the sixth, poll, panic or return wider than two words.
  - A function with no calls, polls, panicking checks, frame slots or spilled values and no callee-saved register has no frame at all and contains no safepoint.
- A frame slot's address is not held in a register: a load or store through it addresses `[rbp+offset]` directly and any other use computes the address with `lea` into a temporary where it is needed. A spilled value copied from another value whose live range it does not overlap shares that value's slot, so splitting a value at the wrapped block adds no slot-to-slot copy. Any other spilled value takes the first slot whose earlier occupants are all dead before it is defined, so a frame holds as many spill slots as values live at once rather than one per spilled value. A parameter passed on the stack that gets no register is read and written where the caller put it, above `rbp`, and the stack maps name that word while it holds a pointer.
- A stack parameter or a stack result word whose spill weight is at most 2 takes no register: its uses read its arrival slot or the outgoing area. A stack result word that no call can overwrite before its last use stays in the outgoing area.
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
- The poll check is the `TLS_POLL` macro, which compares the thread's preemption flag with 0, and a `jne` to an out-of-line stub. The stub calls `rt_poll` and jumps back to a label placed directly after the `jne`. On Linux `TLS_POLL` is `cmp byte ptr fs:[-8], 0`; on Windows it reads the flag through `rt_tls_slot` and `gs`.
- Alignment:
  - Every function and every C callback thunk starts on a 32-byte boundary.
  - A program for Linux is assembled with branch alignment, which pads so that no jump, and no fused compare and jump, crosses or ends at a 32-byte boundary. Hot patches are assembled without it.
  - A loop that is a single back-edge block, ends in its own conditional jump and has at most four instructions and phi copies besides the poll, if any, and the compare starts on a 32-byte boundary, so the whole loop sits in one window and needs no padding inside.

Calling convention:

- The first six argument words go in `rdi`, `rsi`, `rdx`, `rcx`, `r8` and `r9`. The rest go in an outgoing area at the bottom of the caller's frame, so `rsp` does not move inside a function.
- Up to two result words return in `rax` and `rdx`. Wider results are written to the outgoing area after the stack arguments. A result passed by reference returns no words.
- Argument and parameter registers, general and `xmm`, are filled with a parallel move that breaks cycles through `r11`.

### Vectors

- A vector local lives in a frame slot, like a variable passed as `mut`, and a vector expression evaluates to the address of a slot holding its result.
- A vector operation is one IR instruction, `vector`, taking the destination and operand addresses, or `vectorscalar` for `mask`. `any` and `all` are a `mask` compared with 0 or with every lane bit set.
- At calls, returns, fields and other aggregates a vector is its words, loaded from or stored to a slot. A `let` of a computed vector adopts the slot of the result instead of copying it.
- A function containing vector instructions is compiled three times. Its symbol is a stub that reads the byte `rt_simd_tier` and jumps to the AVX-512 version (2), the AVX2 version (1) or the SSE2 version (0). `rt_cpu_init` sets the byte at startup from `cpuid` and `xgetbv`. It sets 1 when the CPU and the operating system support AVX2. It sets an AVX-512 capability flag when `cpuid` leaf 7 lists AVX512F, AVX512DQ, AVX512BW and AVX512VL and `xgetbv` shows the SSE, AVX, opmask, `zmm` upper-half and `zmm16` to `zmm31` state enabled.
- `LOW_SIMD=sse2` in the environment selects the SSE2 versions, and `LOW_SIMD=avx512` selects the AVX-512 versions. The fixture runner uses both to test every tier. The AVX-512 versions are never chosen without `LOW_SIMD=avx512`. The request is ignored on a CPU or operating system without the AVX-512 state, which keeps the AVX2 or SSE2 choice. Windows reads no environment, so it runs AVX2 or SSE2. Automatic selection of AVX-512 on a capable CPU is off until the tier has run on hardware.
- The AVX-512 version is the AVX2 version on `xmm`, `ymm` and `zmm` registers 0 to 15, one register for a vector of any width, with these operations in EVEX form:
  - 64-bit multiplication (`vpmullq`), minimum and maximum of 64-bit lanes, and arithmetic 64-bit shifts (`vpsraq`, which fills with the sign for a count above 63).
  - Every integer comparison except equality and signed greater-than, and every comparison of a 512-bit vector, as a `vpcmp` or `vcmpps`/`vcmppd` into `k1` followed by `vpmovm2*` back to lane masks. Unsigned comparisons need no sign-bit bias.
  - `mask` as `vpmov*2m` into `k1` and a `kmov` of the lane count's width. A comparison directly followed in its block by a `mask` of its result writes `k1`, and the `mask` reads it. The lane masks are written only when the result has another use. So `simd.any(a == b)` and `simd.all(a == b)` are a `vpcmp`, a `kmov` and a scalar comparison.
  - Byte multiplication and byte shifts through 16-bit lanes: `vpmovzxbw` widens each operand, `vpmullw`, `vpsllw` or `vpsrlw` computes, and `vpmovwb` keeps the low bytes, so a count of 8 or more gives 0. A `u8x64` is widened as two 256-bit halves (`vextracti64x4`, `vinserti64x4`).
  - `select` as `vpternlogq`, `~` as `vpternlogd`, a splat as a `vpbroadcast` from a general register, a bitwise operation on a 512-bit vector as `vpandq`, `vporq`, `vpxorq` or `vpandnq`, a float negation of one as `vpxord` or `vpxorq` with a broadcast sign constant, and moves of one as `vmovdqu64` and `vmovdqa64`.
  - The tier uses AVX512F, AVX512DQ, AVX512BW and AVX512VL instructions and nothing from other extensions.
- The AVX2 version uses three-operand VEX instructions on `ymm` registers and splits a 512-bit operation into two 256-bit halves. An addition, subtraction, multiplication, minimum, maximum, floating-point division, bitwise operation, byte shuffle or integer equality is one instruction on the allocated registers. Its second operand may be the slot in memory. A splat broadcasts into the destination register. `vzeroupper` runs before every call and return of a vector function.
- A vector slot is a register slot when all its uses are vector instructions, vector loads and stores through the slot, and lane addresses (the slot plus an index). The register allocator treats it as one value whose home is the frame slot. It gives the value one of registers 8 to 15, as `xmm`, `ymm` or `zmm` by the slot's width, with the same linear scan as the other values. A 512-bit slot is a register slot only in the AVX-512 version. A register slot that gets no register is read from and written to its home. Any other vector slot stays in memory. Slots are 8-byte aligned and every vector load and store is the unaligned form.
- A register slot is live in its register across blocks and loops. A pass after lowering inserts the transfers. It inserts a spill (`vmovdqu`, or `vmovdqu64` for a `zmm` register, to the home) before a word load from the slot and before a word store that writes only part of it. It inserts a fill (`vmovdqu` from the home) before the next read through the register. It inserts a spill of every live register slot before a call, runtime call, C call or a syscall that is not emitted inline, with a fill after it where the value is read again. A run of word stores that writes the whole slot needs no spill. A block ends with every live-out register slot in its register. No `ymm` or `zmm` register survives a call.
- The poll stub of a block saves the register slots live out of the block to their homes, runs `vzeroupper`, calls `rt_poll`, loads them back and jumps to the block's continuation.
- The SSE2 version splits 256-bit operations into two 128-bit halves and 512-bit operations into four, and keeps every vector in memory.
- An operation without an instruction in the version being generated runs as a scalar loop over the lanes in a scratch area of the frame. These operations are `pshufb` on SSE2, 64-bit multiplication, minimum, maximum and comparison where unavailable, byte multiplication, byte shifts and arithmetic 64-bit shifts. Unsigned comparisons flip the sign bit of both operands and compare signed. The AVX-512 version has an instruction or a short sequence for each of these, so it runs no scalar loop. The scalar loop of a shift clamps the count, read as unsigned, to 63 before the lane loop, so a negative count or one above 63 gives the same lanes as a count of 63.

### WebAssembly

- `compiler/wasm` turns each SSA function into a WebAssembly function whose parameters and results are its words as `i64`. Engines refuse a function type of more than 1000 parameters or results. So a function whose parameters or results come to more than 256 words passes that side through linear memory. The rule reads only the word counts of the function type, so every direct and indirect call site and the callee agree. Parameters above the limit are one `i64` pointer to the words, which the caller stores in an area of its frame and the callee loads into locals before anything else runs. Results above the limit are a leading `i64` pointer to a caller-owned slot in the same area, which the callee fills at `return` and the caller loads from after the call. The area is part of the caller's zeroed frame, so the collector's scan of the stack sees any pointer it holds. The callee's pointer parameters get their root words as usual. Every SSA value is an `i64` local. Values whose live ranges do not overlap share one, coloured along the dominator tree.
- Blocks nest as `block`, `loop` and `if` following the dominator tree, as in Ramsey's "Beyond Relooper". They are built with an explicit task stack. A graph is reducible when every edge to a block that is not later in reverse post-order goes to a block that dominates its source. `ir.is_reducible` in `compiler/ir/dominators.low` is that test. The back end fails the build with `irreducible control flow in <function>` when it is false. The lowering has no node splitting. `compiler/tests/ir` requires every function to be reducible after optimisation (see Checks). Phi copies run at the end of the predecessor, which is always a jump because the IR splits every other edge into a block with phis. A switch is a `br_table`.
- Results match the amd64 back end: narrow integers are sign-extended or masked, division by zero panics, a shift by more than 63 fills, and a float converted to an integer out of range or NaN gives the minimum `i64`.
- There is one linear memory. A Low address is the WebAssembly address plus 64 KiB, so a null pointer and small offsets from it lie outside the memory and trap. From the bottom: the 16 bytes of hash seeds, one 1024-byte static block for each module that calls `__static_block`, the hoisted-literal cells, the strings and embedded files, an 8 MiB stack for frame slots that grows down from its top through the global `sp`, then the heap. The heap starts at the next 64 KiB boundary and is managed by the runtime described below. A prologue that would move `sp` below the stack panics with `stack overflow`. A `region` block allocates on the collected heap. Memory in a region is therefore reclaimed when it becomes unreachable, not at the end of the block, and a use after the block does not fault.
- A memory scan cannot see a local, so a frame also holds root words. A call that can collect is a direct or indirect call, `rt_alloc`, `rt_grow` or a scheduler routine, which allocates thread records and switches stacks. Before such a call, each local whose value is live across the call and not scalar is stored to its root word. The prologue stores the parameters that are not scalar. A function with no such call has no root words. Objects do not move, so nothing is reloaded after the call.
- String equality, printing, panics, `memmove`, the high half of a 64-bit multiplication and the float-to-integer conversions are WebAssembly functions the generator writes into every module.
- A `vector` instruction is straight-line scalar code over its lanes in `compiler/wasm/vector.low`, with no SIMD instructions. Each lane is loaded from the operand addresses at its width, computed as an `i64`, `f32` or `f64` and stored to the destination before the next lane is read. A destination that is also an operand is therefore safe. `shuffle` builds its result in locals before storing it. `copy` is `memory.copy`. Bitwise operations and `select` run on 8-byte words. `vectorscalar` (`mask`) ors the top bit of each lane into the result.
  - Lanes match the amd64 versions: integer lanes wrap, comparisons give all ones or all zeros, and a float `min` or `max` returns its second operand when either is NaN or both are zero.
  - A shift count at or above the lane width, or negative, gives zero, or the sign fill for a signed right shift.
- `compiler/compiler/prelude/browser.low` is the runtime of a target without an operating system. The compiler parses it with the prelude's privileges and joins it to every program. A function named `__rt_*` in privileged source is a root beside `main`. The back end calls `__rt_alloc`, `__rt_grow`, `__rt_pin`, `__rt_unpin`, `__rt_lock`, `__rt_unlock`, `__rt_cas`, `__rt_context`, `__rt_set_context`, `__rt_has_context`, `__rt_lock_thread`, `__rt_unlock_thread`, `__rt_lock_all`, `__rt_unlock_all`, `__rt_spawn`, `__rt_current`, `__rt_park`, `__rt_ready`, `__rt_sleep`, `__rt_timer_add` and `__rt_timer_del` by name for the runtime value of the same symbol, with the words `compiler/ir/runtime.low` gives it. The thread entry calls `__rt_finish`. A held lock panics `every green thread is blocked forever`, because channel code never parks while it holds one.
- A runtime function uses no `new`, slice, map, run-time string, closure or interpolation, and has no stack slot, so it has no frame. It keeps addresses in `i64` so that no local of it is a root. It allocates only by calling `__rt_alloc` or `__rt_grow`. It stores the object it gets in a word the collector reads before it allocates again. `compiler/tests/wasm` checks that no `__rt_` function has a stack slot, contains `rt_alloc` or `rt_grow`, or calls a function outside the `__rt_` names.
- The runtime reaches the machine through `extern "wasm"`, accepted only in privileged source and lowered inline with no import. All of them take and return `i64`, and an address is a Low address:
  - `stack_pointer()`, `stack_top()`, `cells_start()`, `cells_end()`, `heap_start()` and `memory_end()` give the shadow stack bounds, the hoisted-literal cells, the heap's first address and the end of the memory.
  - `memory_grow(pages)` grows the memory by 64 KiB pages and returns the previous page count, or -1 when the host's page limit or the address space refuses.
  - `fill(at, byte, n)` and `copy(to, from, n)` are `memory.fill` and `memory.copy`.
  - `stress_words()` is the host's stress interval, or 0.
  - `stack_floor()` gives the lowest address a frame may use, and `set_stack(sp, top, floor)` sets `sp`, `top` and `floor` together.
  - `clock(id)` is `low.clock`.
  - `switch(from, to)` and `wait(ms)` call the host imports of the same names. They exist only in a module that imports them, and trap otherwise.
- The heap is a table of 65536 page entries followed by the pages the allocator hands out. The table is set up on the first allocation in the first pages of the heap. An entry names the page's kind (free, small, large head, large tail or runtime table), whether its objects hold pointers, and the allocated and mark bits of a large object.
  - An object of up to 16 KiB comes from a page of one size class and one scan kind: 16, 32, 48, 64, 96, 128, 192, 256, 384, 512, 768, 1024, 1536, 2048, 3072, 4096, 8192 or 16384 bytes. The page starts with an allocated bitmap and a mark bitmap. The free objects of a (class, scan kind) are linked through their first word.
  - A larger object takes a run of whole pages, found first-fit. A tail entry names its head.
  - When no run fits, the memory grows. When it cannot, the runtime collects once and retries, then panics `out of memory`.
  - Returned memory is zeroed, at least one word and 8-byte aligned. `rt_grow` returns a block of the new size holding the first `used` bytes of the old one, which is a root while the new block is allocated.
- A collection runs when the bytes allocated since the last one exceed the bytes that survived it, with a floor of 4 MiB. It is conservative and non-moving:
  - The roots are every word of the shadow stack from `sp` to its top, of the hoisted-literal cells and of the pin table, plus the context word and the scheduler's state. Every green thread that is not running is a root with its stack from its saved `sp` to its top. A word that points anywhere inside an allocated object, including past its first byte, marks it.
  - Marking pushes objects that hold pointers on a stack kept in pages from the page allocator, which doubles when full. An object that holds none is marked and not scanned.
  - Sweeping rebuilds the free lists, returns empty small pages and unmarked large runs to the free pages and clears the marks. `unpin` removes one occurrence of the word.
- The host asks for stress with `host.stress`. `host.mjs` sets it from `LOW_GC=stress` when run by `node`, and the module reads it through `low.gc_stress()`. The runtime then collects every 8192 words allocated, or every eighth of the live words when that is more. Sweep fills each freed object after its link word and each freed large run with `0xa5`, so a use of an unrooted object reads a pattern instead of stale data. `LOW_WASM_MAX_PAGES=n` in the same environment sets `host.maxPages`, which caps the memory at `n` pages above its initial size. `memory_grow` consults `low.page_limit()`.
- The module imports `low.write(fd, ptr, len)`, `low.exit(status)`, `low.clock(id)`, `low.entropy(ptr, len)`, `low.gc_stress()` and `low.page_limit()`, plus `m.exp` and each `f64` function an `extern "m"` block declares, which must be one of the functions `wasm.host_math` lists and `host.mjs` provides. `low.clock(id)` returns nanoseconds as `i64` from the monotonic clock for id 1 and from the wall clock since the Unix epoch otherwise. A module of a program that uses the scheduler routines also imports `low.switch(from, to)` and `low.wait(ms)`, and exports `run_thread`. It exports `memory` and `_start`. `_start` fills the hash seeds and calls `main`. It exits with status 1 and `error: Variant` when `main` fails.
- `extern "low"` in standard library source imports `low.<symbol>` from the host. A pointer parameter (`ptr`, `ptr[T]` or a reference argument) is an `i32` holding the WebAssembly address. Another integer parameter is an `i32` wrapped from the value. An `f64` stays `f64`. A result is nothing, an integer or an `f64`. An integer result is extended to `i64` by the result type's signedness. `std/crypto` declares `entropy(buf: ptr, n: i64)` this way. `std/stdio` declares `read(fd, buf, n)`, which returns the count read, `write(fd, buf, n)` and `halt(status)`.
- A target without an operating system (`target.has_os`) rejects, while the loader reads the program, anything that reaches one:
  - An import of `std/sys` fails at the import with `browser/wasm32 has no operating system, so std/sys cannot be imported: main -> std/os -> std/sys`. The chain is the shortest from `main`, because the loader is breadth first and records each module's first importer.
  - An `extern` block whose library is not `m` fails at the block with `browser/wasm32 has no library "c"; a browser program can declare only extern "m"`. Libraries `low` and `wasm` are allowed only in privileged source, the standard library and the runtime.
  - A call to `__syscall` fails at the call with `browser/wasm32 has no system calls, so __syscall cannot be called`. The checker decides it for every target without Linux system calls, `windows/amd64` included.
- `wait_readable`, `wait_writable` and calls through C function pointers fail the build with a `browser/wasm32:` message naming them.
- Green threads run on one host thread. A thread switches away when it parks, sleeps or finishes. A loop that never does so is preempted at its back-edges.
  - A module that uses the scheduler routines has a mutable `i64` global, the budget, which starts at 1000. Each block that closes a loop (`b.poll`) decrements the budget at its end, before the phi copies. At 0 the block resets the budget and calls `__rt_yield`. Other modules have neither the global nor the polls. `__rt_` functions never poll.
  - The poll is a collecting point. `colors.low` records, keyed by the block, the pointer locals busy at the end of the block (the values live out of it and its control value). Before the call, the poll stores them to the frame's root words, as a call does.
  - `__rt_yield` returns at once while the held-lock count is not 0. The held-lock count is a word of the heap's table page. `__rt_lock`, `__rt_unlock`, `__rt_lock_thread` and `__rt_unlock_thread` keep it, and through them `__rt_lock_all` and `__rt_unlock_all` do. Otherwise `__rt_yield` runs the expired timers. It returns when the run queue is empty. If the queue is not empty, it marks the thread runnable, queues it and switches to the head of the queue as `park` does.
  - A thread is a heap record holding its state (runnable, running, parked or dead), a wake token, the closure's code and env words, its stack block, its top and saved `sp` and its context word. `spawn` copies the spawner's context into the record and queues it without running it. The main thread's record is made on the first scheduler call and uses the static stack.
  - The scheduler's state hangs from a word of the heap's table page: the table of live threads, the FIFO run queue, the timers, the running thread and the free stacks.
  - `ready` queues a parked thread and sets the wake token of any other. `park` consumes a set token and returns, and otherwise marks the thread parked and switches away.
  - Switching away picks a thread in a loop. It runs the expired timers and takes the head of the run queue. With an empty queue it waits in the host with `low.wait` until the earliest timer or, with no timer, panics `every green thread is blocked forever`. When the pick is the thread itself, it continues. Otherwise it saves its `sp` and context, makes the pick current and calls `low.switch(from, to)`. Once resumed, it restores `sp`, `top` and `floor` with `set_stack` before any function with a frame runs.
  - A timer is a deadline, the thread to ready, a waiter, the arm index and an id. Firing one with a waiter claims the waiter's `claimed` word from 0 to 1, stores the arm index and 0 in the waiter and readies the thread. A timer with no waiter readies the thread directly. A claimed waiter's timer is dropped. `sleep` adds a timer without a waiter and parks. Timers use the monotonic clock.
  - A thread's stack is a 256 KiB block from `__rt_alloc`, of the kind that holds no pointers. The collector scans it from the thread's saved `sp` to its top. The block is taken when the thread first runs, from the free list when it holds one. The floor is at the base of the block, so overflowing it panics `stack overflow`. The exported `run_thread(g)` sets `sp`, `top` and `floor` from the record. It calls the closure through the table with its env as the only argument. It calls `__rt_finish`, which marks the thread dead, removes it from the table, picks the next thread as above and then frees the stack. `run_thread(g)` returns the thread to run next. The stack stays referenced while the pick runs because a collection there still runs on it.
- `compiler/wasm/host.mjs` is the host. `run(program, host)` takes the bytes of a module or a compiled `WebAssembly.Module`, instantiates it with the imports above, runs it and returns its exit status. `host.read(fd, bytes)` answers `low.read` with the count read, 0 at the end of the input, or -1 when the host has no input. `low.halt` ends the run as `low.exit` does. A trap prints `panic: invalid memory access` or `panic: stack overflow` and returns 2. The host uses WebAssembly JavaScript Promise Integration only for a module that imports `low.switch`. Any other module runs `_start` synchronously, because a promising call runs on a secondary stack of under 1 MB. For a module that imports it, `low.switch` and `low.wait` are `WebAssembly.Suspending` imports, and `_start` and `run_thread` are called through `WebAssembly.promising`:
  - `switch(from, to)` stores a promise as the resumer of `from` and schedules `go(to)` with `queueMicrotask`. It never runs WebAssembly inside the import. `go(g)` resolves the stored resumer of `g` when there is one. Otherwise it calls `run_thread(g)` and then `go` on the thread it returns.
  - `wait(ms)` returns a promise that `setTimeout` resolves.
  - `_start` returning ends the run with status 0. A rejection of `_start` or of any `run_thread` call ends it the way a trap does: `low.exit` gives its status and any other error prints `panic:` and its message and gives 2. The run clears its pending timeouts and settles once.
  - Without `WebAssembly.Suspending`, a module that imports `low.switch` prints `panic: green threads need WebAssembly JavaScript Promise Integration` and exits with 2. A module that does not import it needs no Promise Integration.

  Run by `node` with a module path, it sets `--experimental-wasm-jspi` through `v8.setFlagsFromString` when `WebAssembly.Suspending` is missing, reads input from the process's descriptors when the module asks and runs the module on a worker with a 256 MB stack. The worker's exit without a status prints `panic:` and gives 2. So a program whose threads all wait for a wakeup that never comes cannot exit with status 0. `low run --target browser/wasm32` writes the script beside the module and runs it that way. `compiler/tests/wasm` builds run fixtures for the host and for the browser, once normally and once under stress. It checks that both print the same and exit with the same status. The fixtures include channel, `select`, `sleep` and `std/sync` programs, spinning threads that only back-edge preemption lets run, and the deadlock panic. A test also runs a threaded module with `WebAssembly.Suspending` deleted and checks the panic above. It also runs a program that allocates 2 GiB in total with at most 1 MiB live under a 64 MiB page limit.

## Hot reload

`low dev app.low` or `low dev app/` builds a dev binary for the host and runs it. It watches every source file of every loaded module, every embedded file, and the file lists of their directories. The watcher polls the content of those files every 100 ms on every host. On save:

- A file that fails to compile is reported and the running program is left alone.
- The program restarts when any of these changes:
  - the set of linked libraries;
  - the set of called C functions;
  - the set of modules that call `__static_block`;
  - a struct or enum layout;
  - an existing function's signature;
  - a closure's captured variables;
  - `main`;
  - more functions than `fn_table` has entries.

  A running frame of old code may still call any function, so it must keep the old calling convention and data layout. `main` never returns to pick up new code.
- Otherwise every function whose generated assembly differs from the running version is recompiled. This includes callers whose code changed only because a callee did, every instance of an edited generic function, and new functions. The program keeps its heap, stack, open regions and all live values.

Mechanism:

- Dev builds call every function through `fn_table`, a writable table of code addresses. The call is `call [rip+fn_table+8*i]`. The build sizes the table to the first power of two that is at least 4096 and at least twice the number of functions. Release builds call directly.
- In dev builds a function value's code word is the address of its `fn_table` entry. Calls through it use `call [r10]`, so a stored function value or closure runs the newest code. Callback thunks call through the table too.
- `compiler/amd64` owns this protocol. It covers `generate` with a `FuncTable` and `generate_patch`, the `fn_table` size, the patch signal and entry size, the mailbox and handler symbol names, and `hot_base`, `hot_size` and the hot runtime text for each `amd64.Platform`.
- At start, before the collector reserves the heap, the dev runtime reserves a 64 MB read-execute region. The region is close enough to the binary for 32-bit relative references. On Linux the runtime installs a `SIGUSR1` handler and maps the region at `0x40000000`. On Windows `rt_hot_init` calls `VirtualAlloc` for the region at `0x180000000` and panics when the address is taken. `link.pe_fixed` links a dev build for Windows at the image base `0x140000000`, with relocations stripped and no `DYNAMIC_BASE`. The image and the region therefore stay a fixed distance apart.
- `Built.Patch` assembles and links a patch in process, at the next free address in that region. It resolves its references against the symbols of the running binary that the linker returned with its image. A patch holds only the changed functions and their strings. Patched functions get a fresh symbol suffix so they never collide with the originals.
- `low dev` writes the machine code into the region. It writes the `(index, address)` pairs and count into a mailbox in the program. It then runs the program's `rt_hot_handler`. The handler stores each address into `fn_table` with an aligned 8-byte write and clears the count. `low dev` waits for the count to clear. On Linux the host writes through `/proc/<pid>/mem` and sends `SIGUSR1`. On Windows it opens the process with `OpenProcess` and writes with `WriteProcessMemory`. On Windows it flushes the instruction cache and starts a thread at `rt_hot_handler` with `CreateRemoteThread`.
- Frames already running keep executing their old code; the next call through the table enters the new code. Old code is never unmapped.
- On Windows, once the program has cleared the count, `low dev` posts the registered window message `LowHotReload` to every top-level window of the program. A `std/ui/platform` window answers it with a `Resized` of its current size, so a `std/ui` window draws a frame with the patched code. On Linux a window draws patched code at its next frame.
- `low dev` starts the program as a process group, which is a job object on Windows. `Ctrl+C` is caught through `signal.notify`, which stops the session, kills the program's group and removes the build directory.

## UI

`std/ui` holds the modules that draw a window's user interface with Vulkan. They use Wayland on Linux and Win32 on Windows. The modules cover windows and input, a flexbox layout engine, fonts and text, a scene of GPU primitives, and a renderer that draws a scene into an offscreen target or a window. On top of them sits an element tree with signals, hit testing and focus, which a program runs as an application.

### UI modules

| Module | Contents |
|---|---|
| `std/ui` | `Element` and its builders, `Signal`, `Memo`, `Effect`, `Task`, `Context`, `App` and `run`. |
| `std/ui/geometry` | `Point`, `Size`, `Rect`, `Edges`, `Corners` and `Color`, all in `f32`. |
| `std/ui/layout` | The flexbox engine over a tree of styled nodes. |
| `std/ui/font` | TrueType and OpenType CFF parsing, glyph outlines, the rasteriser and the embedded fallback font. |
| `std/ui/fontconfig` | The installed fonts on Linux: the fontconfig configuration, the face index and its cache, family matching and per-code-point fallback. |
| `std/ui/dwrite` | DirectWrite shaping, rasterisation and rendering parameters on Windows, and a stub that fails with `Unavailable` elsewhere. |
| `std/ui/svg` | An SVG document of strokes and fills, parsed and rasterised to a coverage mask. |
| `std/ui/text` | Shaping, line breaking, measurement, the glyph atlas and icon masks. |
| `std/ui/scene` | Quads, shadows, underlines, sprites and filled or stroked vector paths with their clip rectangles and draw order, and batching by kind and atlas. |
| `std/ui/gpu` | The Vulkan renderer, its shaders, offscreen targets and swapchains, and the generated Vulkan bindings. |
| `std/ui/platform` | Windows, input events and the waiting loop: Wayland, xkbcommon and the generated Wayland bindings on Linux, user32 and kernel32 on Windows. |
| `std/ui/islands` | A dark look and the widgets drawn in it, built on `std/ui`: a `Theme` of colour tokens made from a `Scheme` accent, twelve schemes, spacing and type sizes, islands, buttons, tabs, menus and a menu bar, popovers, dialogs and form cards, tooltips, a drawn title bar, pickers, completion lists, code blocks with a gutter, a scrolling data grid (`sheet_view`), glow and grain backdrops and the Lucide icons it draws. It is platform independent. |

- `std/ui/geometry`, `std/ui/layout`, `std/ui/font`, `std/ui/fontconfig`, `std/ui/dwrite`, `std/ui/svg`, `std/ui/text`, `std/ui/scene` and, apart from `open` and `run`, `std/ui` only compute and read files, so they run on Linux and Windows with no library besides `dwrite.dll` on Windows. `std/ui/fontconfig` reads the fontconfig configuration and the font files with `std/file`. It does not link libfontconfig. All of them but `std/ui` also build for `browser/wasm32`. There `std/ui/dwrite` is the stub and `fontconfig.environment` returns `none`.
- `std/ui/gpu` and `std/ui/platform` hold one file per target for what differs: `surface_linux.low` and `surface_windows.low`, `wayland_linux.low` and `win32_windows.low`. These sit behind one API (see Modules under Syntax). `std/ui/platform/window_browser.low` imports `std/sys`. The loader therefore rejects a `browser/wasm32` build of `std/ui/platform` and of every module that imports it, `std/ui` among them, before checking it.
- The generated bindings are private to the module that uses them. `std/ui/gpu/vulkan.low` holds the types and constants, and `vulkan_linux.low` and `vulkan_windows.low` the commands, linked from `libvulkan.so` and `vulkan-1.dll`. `std/ui/platform/wayland_linux.low` holds the Wayland bindings.
- A Linux program that opens a window links `libvulkan.so.1`, `libwayland-client.so.0`, `libxkbcommon.so.0` and libc. A Windows one imports `vulkan-1.dll`, `user32.dll`, `kernel32.dll` and `dwmapi.dll`.

### Generators

- `compiler/wlgen/protocols` holds the Wayland protocol XML: `wayland.xml`, `xdg-shell.xml`, `cursor-shape-v1.xml`, `text-input-unstable-v3.xml`, `viewporter.xml`, `fractional-scale-v1.xml` and `xdg-decoration-unstable-v1.xml`. `low run compiler/wlgen -o std/ui/platform/wayland_linux.low compiler/wlgen/protocols/wayland.xml compiler/wlgen/protocols/xdg-shell.xml compiler/wlgen/protocols/cursor-shape-v1.xml compiler/wlgen/protocols/text-input-unstable-v3.xml compiler/wlgen/protocols/viewporter.xml compiler/wlgen/protocols/fractional-scale-v1.xml compiler/wlgen/protocols/xdg-decoration-unstable-v1.xml` generates the Wayland bindings.
  - Every interface given in one run shares an `Interfaces` record of `wl_interface` tables that `interfaces()` builds in C memory. The record keeps every block it allocates, listener copies included. `release_interfaces(ifs)` frees them once the connection is closed. An interface that a request or event creates must be declared by a given protocol, or generation fails. An object argument of an undeclared interface, such as the `zwp_tablet_tool_v2` that `cursor-shape-v1.xml` names, gets a null entry in its message type table.
  - A request becomes `interface_request(...)` over `wl_proxy_marshal_flags`, taking the `Interfaces` record when it creates an object of a known interface. `wl_registry_bind` takes the interface table and version.
  - Destructor requests pass `WL_MARSHAL_FLAG_DESTROY`, and an interface without a `destroy` request gets one that calls `wl_proxy_destroy`.
  - Objects are `ptr`, strings are `ptr[u8]` and `fixed` is an `i32`.
  - Events become an `extern struct` listener of `extern fn` fields. Each field defaults to a generated function that ignores the event, so a listener names only the events it handles. `interface_add_listener(ifs, proxy, listener, data)` copies it into the record's C memory. Callbacks receive the listener's `data` pointer, which is a pinned Low object.
  - Enum entries are constants such as `wl_shm_format_argb8888: u32`. Names that clash with Low keywords or builtins gain a trailing `_`.
- `compiler/vkgen/registry` holds the Vulkan registry, `vk.xml.gz` from Vulkan-Docs v1.4.363, and `vulkan.list`. The list selects extensions, commands, extra types such as `pNext` structures, and the library each target loads.
  - `low run compiler/vkgen -registry compiler/vkgen/registry/vk.xml.gz -select compiler/vkgen/registry/vulkan.list -o std/ui/gpu` writes `vulkan.low` and one `vulkan_<os>.low` per target. A `command` line that ends with a target name, such as `command vkCreateWaylandSurfaceKHR linux`, goes only to that target's file. `library windows vulkan-1` names the library its `extern` block links.
  - Commands and every type they reach are generated, skipping Vulkan SC variants. Enum values come from the core enums, every Vulkan feature level and the selected extensions.
  - A handle is a one-field `extern struct`. The field is `ptr` for dispatchable handles and `i64` for the others, so `VkBuffer {}` is the null handle and handles of different kinds do not mix.
  - Structures and unions become `extern struct`s. Every field has a default: zero, `null`, the empty handle, or for `sType` the structure's own value. A literal therefore names only what it sets. A union takes the layout of its widest member.
  - A fixed C array becomes numbered fields, `name_0` to `name_N`. An accessor `Struct_name(s)` returns them as a slice, or as a `str` for a character array.
  - Vulkan types map to `u32` (`uint32_t`, `VkBool32`, `VkFlags`, 32-bit flag bits), `i32` (`int32_t`, enums), `i64` (`uint64_t`, `VkDeviceSize`, `VkFlags64`, `size_t`), `u16`, `u8`, `f32` and `f64`. Platform types such as `HWND` and `wl_display*` are `ptr`.
  - Pointers to known types become `ptr[T]`. `void*`, function pointers and pointers to pointers become `ptr`. A command's single input or output through a pointer is a `mut` parameter. An array or optional pointer is a `ptr[T]`.
  - Enum entries and API constants are constants such as `VK_FORMAT_R8G8B8A8_UNORM: i32`, and `vk_make_api_version` builds version numbers.
- Both generators emit declarations without `pub`. `compiler/tests/vkgen` and `compiler/tests/wlgen` fail when the checked-in files differ from what the generators produce.
- `compiler/protogen` is the library behind `low protogen`, the driver of `std/protobuf/proto`. `low protogen -I testdata/run/std_protobuf_generated/protos -o testdata/run/std_protobuf_generated all.proto rich.proto items.proto kinds.proto` writes the modules that `testdata/run/std_protobuf_generated` compiles and compares with the generator's output. `testdata/run/std_grpc_generated` holds the generated client and server code of two services. Its program serves and calls these services over `std/grpc` and compares the code with the generator's output. `compiler/tests/low` runs `low protogen` on those protos and requires the files it writes to equal the checked-in modules. `compiler/tests/protogen` checks that `proto.keywords` lists the lexer's keywords and builds the generated code of a proto whose names are keywords.
- `std/ui/gpu/shaders` holds the GLSL and its SPIR-V. `low run compiler/shadergen std/ui/gpu/shaders` compiles each stage with `glslc` to a `.spv` and records the SHA-256 sums of the sources and the `.spv` files in `shaders.sum`.

### Platform

`std/ui/platform` gives both targets one API:

- `open(Options) -> Window ! Error` creates a window of a logical size with a title. `Window` is droppable. Releasing it destroys the window and closes the connection. A window belongs to the green thread that opened it. With `Options.hidden` the window is created and sized as usual but stays invisible until `show(w)`. `show(w)` does nothing to a window that is already visible.
- `next(w, timeout_ms) -> Event[] ! Error` returns the events that arrived, waiting until at least one arrives, a `Waker` fires or the timeout passes. A negative timeout waits without limit. The first `next` after `open` returns a `Resized`.
- `waker(w) -> Waker` is a copyable handle. `wake(k)` makes the waiting `next`, or the next one, return a `Woken`. Each `wake` gives one `Woken`.
- `native(w) -> Native` gives the handles Vulkan needs to make a surface: the `wl_display` and `wl_surface` on Linux, the `HINSTANCE` and `HWND` on Windows.
- `Error` is `NoDisplay(detail)` when there is no compositor or no desktop. It is `Unsupported(detail)` when the windowing system lacks something the window needs. It is `Failed(call, code)` with the call's error code, or `Disconnected`. It is `BadIcon(detail)` when an icon frame is not a PNG of the size it states or the system cannot make an icon from it.
- `Event` is `Resized(width, height, scale)` with `f32` logical pixels and scale, `CloseRequested`, `PointerMoved(x, y)`, `PointerLeft`, `PointerDown(ButtonEvent)`, `PointerUp(ButtonEvent)`, `Scrolled(dx, dy, modifiers)`, `Key(KeyEvent)`, `Preedit(text, cursor)`, `Commit(text)`, `Maximized(bool)`, `FocusChanged(bool)`, `FileDragEntered(x, y, paths)`, `FileDragMoved(x, y)`, `FileDropped(x, y)`, `FileDragExited` and `Woken`. Pointer positions are logical pixels from the top left of the window. `Maximized` arrives only when the maximised state changes.
- A `ButtonEvent` carries the `Button`, the position, the `Modifiers` and `clicks`. The `Button` is `Left`, `Right`, `Middle`, `Navigate(Back)` and `Navigate(Forward)` for the side buttons, as gpui's `MouseButton`, and `Other(code)`. `clicks` is how many presses of that button in a row the press makes. The platform counts them as gpui's `ClickState` counts them (`platform/windows/window.rs:1013-1083`). A press of the same button within the double-click time of the last press, and within the double-click distance of it on each axis, counts on. Any other press starts again at one. A release carries the count of its press. On Windows the time is `GetDoubleClickTime`, the distance `SM_CXDOUBLECLK` and `SM_CYDOUBLECLK` device pixels, read at each press, and the press time `GetMessageTime`; Shift and Control come from the message's key flags and Alt and Super from `GetKeyState`. On Wayland the time is 400 milliseconds and the distance 5 logical pixels, as gpui's `DOUBLE_CLICK_INTERVAL` and `DOUBLE_CLICK_DISTANCE`, measured from the button event's timestamp, and `BTN_SIDE` and `BTN_EXTRA` are back and forward.
- `clipboard(w) -> Clipboard` is a copyable handle to the system clipboard through the window. `write_clipboard(c, text) ! Error` replaces the clipboard's contents with `text`. `read_clipboard(c) -> str? ! Error` returns its text, or `none` when it holds none.
- `set_text_input(w, caret: Caret?)` turns the system's input method on, with its candidate window beside `caret` (logical pixels from the top left of the content, so it can follow the text being edited), or off for `none`. It starts off, and setting it again moves the caret. While it is on, the input method's composition arrives as `Preedit(text, cursor)`. Here `cursor` is a `TextRange` of byte offsets into `text`, or `none`, and empty text ends the composition. The result arrives as `Commit(text)`, which replaces the composition. Neither arrives while it is off. Keys the input method handles give no `Key` event. Other keys still give `Key` events with their text.
- `set_cursor(w, Cursor)` sets the pointer's image while it is over the window's content. A window starts with `Cursor.Arrow`; `Hidden` shows no image.
- `Options.titlebar` is a `TitlebarOptions`, as gpui's `TitlebarOptions` is (`platform.rs:1248`). Its `appears_transparent` hides the system title bar so the program draws its own. It defaults to `false`. The platform draws no caption buttons. `decoration_height(w)` is the height in logical pixels at the top of the content that the program must draw as a title bar. It is zero where the system draws one or the program draws its own, and 32 on Wayland while the compositor gives no server-side decorations. A window opens `Options.height` plus that height tall. Its `Resized` size includes that height, and a `Resized` follows a change of it.
- `Options.owner` is a `WindowHandle?`, the copyable view that `Window.handle` holds. It is a handle because a `Window` is droppable and cannot sit in an optional. Opening fails with `Unsupported` when the owner has closed or belongs to another `Display`. `Options.min_width` and `Options.min_height` are the smallest content size in logical pixels, zero for no limit. `Options.placement` is a `Placement`. `System` leaves the position to the system. `CentreOnOwner` centres on the owner and keeps the window inside the work area of the owner's monitor. `CentreOnScreen` centres on the work area of the monitor that holds the first window of the window's `Display` that is still open. With no owner, `CentreOnOwner` centres on that window. The first window of a display is centred on the primary monitor's work area. The size that is centred is the size after the minimum. `Options.taskbar` is `true` by default and gives the window its own taskbar button. `Options.activate` is `true` by default. `false` shows the window with `SW_SHOWNOACTIVATE`, so it takes neither the keyboard focus nor the foreground. `Options.border` is a `u32?`, the colour of the window's one-pixel border as `0xRRGGBB`, and `none` keeps the system's; it is a plain integer because `platform` imports no geometry. `Options.icon` is an `IconFrame[]?`, the frames of the window's icon. `none` keeps the system's. An `IconFrame` is a square picture at one pixel size. It has `size`, its width and height in pixels, and `png`, the bytes of a PNG file as a `str`. That is what `embed` returns, so a program writes `IconFrame { size: 32, png: embed("icons/32.png") }` for each of its sizes. `set_icon(w, frames) ! Error` changes the icon while the window is open and `set_title(w, title)` changes the title. The Wayland backend ignores `owner`, `min_width`, `min_height`, `placement`, `taskbar`, `activate`, `border` and `icon`.
- `set_hit_test(w, f)` sets `f(x, y) -> WindowControlArea?`, which answers what lies at a point of the content in logical pixels. `WindowControlArea` is `Drag`, `Close`, `Max` or `Min`, as gpui's is (`window.rs:479`). The window asks `f` where the system asks for a hit test, so the program marks the parts of its own title bar. A window with no `f` has no areas. On Wayland a window that draws its own frame asks `f` on each press of the left button: a `Drag` area starts `xdg_toplevel.move` with the press serial, a second press within the click interval toggles the maximised state, and `Close`, `Max` and `Min` act when the press and the release both land on the same area; none of those presses or releases reaches the program as a pointer event, and a press elsewhere on a `Drag` area sends none either.
- `prefers_reduced_motion() -> bool` needs no window. On Windows it reports `SPI_GETCLIENTAREAANIMATION` turned off. On Linux it reports the GNOME `enable-animations` setting turned off, read with `portal.animations_enabled` from `std/portal` with a 200 millisecond limit. It reports `false` where the system does not say.
- `controls(w) -> Controls` is a copyable handle for code that does not own the window. `is_maximized(c)` reports the maximised state. `minimize(c)` minimises the window. `toggle_maximize(c)` maximises it or restores it, and a `Maximized` event follows.
- A `KeyEvent` carries a `Key`, the text it produces with the modifiers held, the `Modifiers` (`shift`, `control`, `alt`, `super`), whether it is a press or a release, and whether it repeats. The `Key` is `Named(NamedKey)` for Enter, Escape, Tab, Backspace, Delete, the arrows, Home, End, Page Up, Page Down, Insert, Pause, Break, Menu and F1 to F12, or `Character(str)`, the key's text with at most Shift applied. Control+A is `Character("a")` with empty text. Modifier keys, and keys with neither a name nor text, give no event.
- On Wayland the scale is the surface's preferred scale. It is a multiple of 1/120 such as 1.25 where the compositor offers `wp_fractional_scale_manager_v1` and `wp_viewporter`. Otherwise it is the whole number of `wl_surface.preferred_buffer_scale`. On Windows the scale is the window's DPI divided by 96, which is 1.25 at 125 percent and 2.25 at 216 DPI. A window's logical size is its client size in device pixels divided by the scale, so it is fractional where the scale does not divide the pixel size.
- A `Gpu` made with `gpu.open_for` draws into the window, so it is released before the `Window`.
- A `Display` is the connection a program's windows share, and `std/ui` opens every window of an application in one. `Display` and `Window` are droppable. Each therefore has a public copyable `handle` and an `_on` form of every operation, as `std/file` and `std/net` have. `DisplayHandle` is the view of a `Display`. Once the display is released, its `open_in_on` and `wait_on` fail with `Unsupported`, and a waker from `waker_of_on` does nothing. `open_in(d, options)` opens a window in `d`, and releasing that window leaves `d` open. A window still open when its display is released is closed with it, and its `next` fails with `Unsupported`. `open(options)` makes a display, opens one window in it and gives the window the display, so releasing the window closes both. `wait(d, timeout_ms) ! Error` returns when a window of `d` has events for `next`, when a wake of `waker_of(d)` or of a window's `waker` has arrived, or after the timeout. A negative timeout waits without limit, and the call takes no event out of any window. A wake of `waker_of(d)` gives no `Woken` to any window, so it ends `wait` whether or not any window is open. A wake that arrives while no one waits ends the next `wait`. Wakes that arrive together end one.

Wayland:

- `open_display` connects to the compositor and binds `wl_compositor`, `xdg_wm_base` and `wl_seat` from the registry. It also binds `wl_data_device_manager` (up to version 3), `wp_cursor_shape_manager_v1`, `zwp_text_input_manager_v3`, `wp_viewporter`, `wp_fractional_scale_manager_v1` and `zxdg_decoration_manager_v1` when the compositor offers them. So every window of a display shares one connection, one seat, one keymap and one clipboard. `open_in` makes a surface with an `xdg_toplevel` and returns once the first `configure` has been acknowledged. It fails with `Unsupported` when none arrives within five seconds. The surface has no buffer until the first `present`, which is when the compositor maps it. `Options.hidden` and `show` therefore change nothing. Without server-side decorations the program draws the window's frame. See the decoration and scale items below.
- Listener callbacks run during `wl_display_dispatch_pending` on the green thread that called `next` or `wait`. They append to a window's event queue, a pinned Low object. The surface, `xdg_surface` and `xdg_toplevel` listeners hold their window. The registry, seat, pointer, keyboard, data device and text input listeners hold the display, which keeps the window whose surface each device's last `enter` named. Pointer events go to the pointer's window, keys and key repeat to the keyboard's, and composition to the text input's. An `enter` for a surface that is no window of the display clears that focus. An event with no focus is dropped. Closing a window clears every focus it held. A failure inside a callback is kept on the display and returned by the next `next` or `wait`.
- `xdg_wm_base.ping` is answered inside the callback. `xdg_toplevel.close` becomes `CloseRequested`. A `configure` with a non-zero size becomes `Resized`. The scale follows `wp_fractional_scale_v1.preferred_scale`, or `wl_surface.preferred_buffer_scale` without the fractional scale globals, and a change of it sends `Resized`. A `configure` also carries the toplevel's states, and a change of whether they hold `maximized` becomes `Maximized` when the `configure` is applied. `minimize(c)` is `xdg_toplevel.set_minimized`, and `toggle_maximize(c)` is `set_maximized` or `unset_maximized`. Each is flushed at once.
- With `wp_fractional_scale_manager_v1` and `wp_viewporter` each surface gets a `wp_fractional_scale_v1` and a `wp_viewport`. The scale is held in 120ths from `preferred_scale`. The surface's buffer scale stays 1, and `preferred_buffer_scale` is ignored. Every `configure` sets the viewport's destination to the logical size, so the buffer `std/ui` draws at the size times the scale, rounded, fills the surface. Without either global the whole preferred buffer scale is the scale and `wl_surface.set_buffer_scale` is set to it. Both objects are destroyed before the surface.
- With `zxdg_decoration_manager_v1` each toplevel asks for server-side decorations, or for client-side ones when `TitlebarOptions.appears_transparent` is set. It follows the `configure` mode the compositor answers. The program draws the frame when the mode is not server-side, including when the compositor offers no manager: `decoration_height` is then 32 unless the window draws its own title bar, the first `configure` sizes the surface to `Options.height` plus it, and a change of the mode sends `Resized`. A window that draws its own frame and is not maximised also has resize borders of five logical pixels inside its edges. A press there starts `xdg_toplevel.resize` with the matching edge, and the pointer shows the matching resize cursor.
- `set_cursor` keeps the cursor in the window. While the pointer is over the surface, it shows the cursor through `wp_cursor_shape_device_v1.set_shape` with the serial of the last pointer `enter`. Every `Cursor` is one shape: `Arrow` is `default`, `Text` `text`, `Pointer` `pointer`, `Crosshair` `crosshair`, `Grab` `grab`, `Grabbing` `grabbing`, `NotAllowed` `not_allowed`, `Move` `move`, `Wait` `wait`, `Progress` `progress`, `Help` `help`, `ResizeLeftRight` `ew_resize`, `ResizeUpDown` `ns_resize`, `ResizeUpLeftDownRight` `nwse_resize` and `ResizeUpRightDownLeft` `nesw_resize`. `Hidden` is `wl_pointer.set_cursor` with no surface. The cursor starts as `Arrow`. The current one is shown on every `enter`, so a cursor set while the pointer is outside takes effect on the next one. Without `wp_cursor_shape_manager_v1` only `Hidden` takes effect.
- The clipboard is the `wl_data_device` selection of the seat. `write_clipboard` makes a `wl_data_source` that offers `text/plain;charset=utf-8` and `text/plain`. It sets the source as the selection with the serial of the latest key, button or keyboard `enter`, and keeps the text. Each `send` event writes the text to the descriptor it carries and closes it. `cancelled` drops the source. `read_clipboard` returns the kept text while the window owns the selection. Otherwise it asks the selection offer to `receive` into a pipe. The offer prefers `text/plain;charset=utf-8` over `text/plain`. The read continues until end of input, stopping after a second without data. It gives `none` when the text is empty, when the selection offers neither type, or when the window has not received one, which a compositor sends only to the focused client. `write_clipboard` fails with `Unsupported` without `wl_data_device_manager`, and both fail with `Disconnected` once the window is closed.
- `set_text_input` uses `zwp_text_input_v3`. The caret is kept. While the surface has text-input focus (`enter`), a caret enables the input method with the normal content type and its cursor rectangle in a `commit`, and `none` disables it. `preedit_string` and `commit_string` fill pending state that `done` turns into events. `Commit` comes first when there is committed text. `Preedit` follows when the composition is not empty or ended one, with the cursor range from `cursor_begin` and `cursor_end`, or `none` when either is negative. Text input focus lost, or turned off with a composition showing, gives `Preedit("", none)`. A `done` while text input is off gives no event. Without `zwp_text_input_manager_v3` text input does nothing.
- Files dragged from another program are not supported: the data device's drag offers are not handled, so no `FileDrag*` event is sent. A drag started by `on_drag` inside the window works.
- The keymap arrives as a file descriptor, which is read with `std/sys` and compiled with `xkb_keymap_new_from_string`. Keys are translated with `xkb_state_key_get_one_sym` and `xkb_state_key_get_utf8`, and modifiers follow `xkb_state_update_mask`. A pointer button event and a `Scrolled` event carry the modifiers the last `wl_keyboard.modifiers` reported. Control with Pause gives the `Break` keysym, which is `Break`, and the `Menu` keysym is `Menu`. libxkbcommon is bound by hand in `xkb_linux.low`.
- Key repeat follows the compositor's `repeat_info`: a held key that produces text or is a named key repeats from a timer inside `next` and `wait`.
- `next` and `wait` wait on the connection's file descriptor without blocking an OS thread. A green thread started by `open_display` waits with `wait_readable(fd)` each time a wait arms it, and reports readiness on a channel. The wait prepares the read. It then `select`s on that channel, the display's bell and an `after` arm for the timeout, the next key repeat or 60 seconds. It cancels the read when anything else wins. Each `Waker`, a window's or the display's, is a channel of one slot that `wake` fills. `wake` then rings the bell, a channel of one slot that the display's wakers share. Both sends give up when the slot is full, so a wake from any green thread never blocks. A ring of the bell, and every dispatch, moves each full window slot into a `Woken` on that window, and the display's slot into a wake that `wait` counts. So `next` on one window keeps waiting through a wake of another. Only one green thread may wait on a descriptor, so a flush that would block retries every millisecond for up to five seconds.
- A broken or reset connection gives `Disconnected`. The generated bindings' C memory is freed when the display is released.

Win32:

- On Windows a `Display` holds what belongs to the connection and not to one window. `open_display` locks the green thread to its OS thread with `sync.lock_thread`, because a window's messages reach only the thread that created it. It sets per-monitor DPI awareness, registers the `LowDisplay` class of its message-only window, calls `OleInitialize` and creates the message-only window (`HWND_MESSAGE`). Each window registers the `LowWindow` class before it is created. Releasing the display destroys the message-only window, attempts `UnregisterClassW` for both classes, calls `OleUninitialize` and unlocks the thread once. The unregistration fails and is ignored while a window of the class exists, so releasing one display never stops another from opening a window. `wait` dispatches the thread's messages with `PeekMessageW`, counts a window's fault as an event for `next` and waits in `MsgWaitForMultipleObjectsEx`. `waker_of(d)` posts `WM_APP` to the message-only window, whose procedure counts the wake on the display. `next` pumps through the window's display. Each pump clears the pending key of every window of the display.
- `open` creates the window with `CreateWindowExW` and converts its title to UTF-16. A creation error of 0 or access denied gives `NoDisplay`. A window opened with `hidden` is cloaked with `DwmSetWindowAttribute(hwnd, DWMWA_CLOAK, TRUE)` before `ShowWindow`. The system therefore keeps a sized and composed window that no one sees, and a Vulkan swapchain made for it works as for any window. `show` clears the attribute. If cloaking fails, `open` leaves `ShowWindow` to `show`. A failed uncloak is returned by the next `next`. `DwmGetWindowAttribute` with `DWMWA_CLOAKED` reports the cloak as `DWM_CLOAKED_APP`. A window opened with `Options.border` is given `DwmSetWindowAttribute(hwnd, DWMWA_BORDER_COLOR, &colorref, 4)` right after `CreateWindowExW` returns. `colorref` is the `0xRRGGBB` colour with its red and blue bytes swapped into `0x00BBGGRR`. Windows 11 draws it as the window's border. Windows 10 has no border colour and fails the call. `open` ignores the result either way. Every frame given to `Options.icon` or `set_icon` is checked first, on every platform, in `icon.low`. `size` must be positive. `png` must be at least 24 bytes that start with the eight-byte PNG signature and a chunk named `IHDR`. The width and the height in that header must both equal `size`. The first frame that fails these checks fails the call with `BadIcon(detail)` naming its index, and changes nothing. On Windows `set_icon` takes `GetSystemMetricsForDpi(SM_CXICON, dpi)` and `GetSystemMetricsForDpi(SM_CXSMICON, dpi)` for the window's `GetDpiForWindow`. For each it picks the smallest frame at least that size, the first of equals, or the biggest frame when none is (`pick_icon_frame`). It makes an `HICON` of each picked frame with `CreateIconFromResourceEx(png, len, TRUE, 0x00030000, size, size, 0)`, which accepts PNG data. When that call returns null, which it does for a PNG whose body is damaged, `set_icon` destroys any icon it made, fails with `BadIcon` and leaves the window's icons alone. Otherwise it sends `WM_SETICON` with `ICON_BIG` and then `ICON_SMALL`, and destroys the two icons the window had. The window therefore never has an icon that is destroyed. The window keeps its frames and the index it picked for each size. `WM_DPICHANGED` picks again and replaces the icons only when an index changes, ignoring a failure. No frames send `WM_SETICON` with a null icon for both sizes, which restores the system's icon. `Options.icon` is applied right after `CreateWindowExW` returns and its failure is ignored, as the border's is. The window destroys the icons it owns when it closes, whether by `close_on`, by being released or by the system destroying it with its owner. `set_icon` fails with `Unsupported` once it has closed. The Wayland backend checks the frames and ignores them.
- The window procedure is a Low callback. It reaches the window's state through `GWLP_USERDATA`, which `WM_NCCREATE` sets from `lpCreateParams`. It appends events for `WM_SIZE`, `WM_DPICHANGED`, `WM_CLOSE`, `WM_MOUSEMOVE`, `WM_MOUSELEAVE`, the button messages, `WM_MOUSEWHEEL`, `WM_MOUSEHWHEEL`, `WM_KEYDOWN`, `WM_KEYUP`, `WM_SYSKEYDOWN`, `WM_SYSKEYUP`, `WM_CHAR`, `WM_SYSCHAR`, `WM_SETFOCUS` and `WM_KILLFOCUS`. A failed call inside it is kept and returned by the next `next`.
- `open` queues a `Resized` for the first `next`, and a `WM_SIZE` that repeats the last size or minimises the window gives none. A `WM_SIZE` queues `Maximized` when `IsZoomed` differs from the state kept.
- A window with an owner is created with the owner's `HWND` as `hWndParent` and `WS_OVERLAPPEDWINDOW`. This makes an owned top-level window. It stays above its owner, minimises with it and is destroyed with it. No window ever calls `EnableWindow` on its owner. `taskbar: true` sets `WS_EX_APPWINDOW`, so an owned window has its own taskbar button. `taskbar: false` sets `WS_EX_TOOLWINDOW` on a window with no owner and nothing on an owned one. The extended style is kept on the window and passed to every `AdjustWindowRectExForDpi`, because a tool window's caption is smaller. An owner that has closed fails `open_in` with `Unsupported`. When the window is placed, it is created at the top left of the target work area. It therefore starts on the monitor it will be centred on and reads that monitor's DPI. `fit_window` then sets its size and the centred position in one `SetWindowPos`, clamped into the work area with the left and top edge first. The owner's `GetWindowRect` gives the centre for `CentreOnOwner` unless the owner is minimised, which centres on the work area.
- `WM_GETMINMAXINFO` sets `ptMinTrackSize` for each of `min_width` and `min_height` that is above zero. The value is the minimum content size times the window's scale, plus the frame from `AdjustWindowRectExForDpi` for the dpi of that scale, or plus `frame_insets` with `appears_transparent`. A change of scale from `WM_DPICHANGED` takes effect at the next message. `set_title` calls `SetWindowTextW`, and a failure is returned by the next `next`. `WM_NCDESTROY` clears the window's handle, so a window that the system destroys with its owner is released without a `DestroyWindow` call, and its anchor is unpinned.
- With `appears_transparent`, the window is created with the usual overlapped style and `fit_window` adds the frame insets to the requested size, so the content is the requested size, calling `SetWindowPos` with `SWP_FRAMECHANGED` instead of `AdjustWindowRectExForDpi`. `WM_NCCALCSIZE` insets the client area as gpui's `handle_calc_client_size` does (`events.rs:699-718`, insets from `get_client_area_insets`, `1494-1534`). A restored window loses the resize frame, `GetSystemMetricsForDpi(SM_CXSIZEFRAME)` or `SM_CYSIZEFRAME` plus `SM_CXPADDEDBORDER`, from its left, right and bottom, and nothing from its top on Windows 10 or one device pixel on Windows 11 (build 22000 or later, read once with `RtlGetVersion`), the one row the system paints as the top border at every scale, so the resize borders lie in the invisible frame outside the content. A maximised window loses the frame on all four sides, the part of it the system hangs off the monitor. A window with the system title bar leaves `WM_NCCALCSIZE` to `DefWindowProcW`.
- `WM_NCHITTEST` converts the screen point with `ScreenToClient` and answers in this order, as gpui's `handle_hit_test_msg` does (`events.rs:856-890`). First comes the area `set_hit_test` gives: `Drag` as `HTCAPTION`, `Close` as `HTCLOSE`, `Max` as `HTMAXBUTTON` and `Min` as `HTMINBUTTON`. Then comes `DefWindowProcW`, whose resize border answers (`HTLEFT`, `HTTOPLEFT` and the like, or `HTNOWHERE`) are returned as they are. Then comes `HTTOP` within `SM_CYSIZEFRAME` of the top of the content when the window is not maximised. Last comes `HTCLIENT`. An area therefore takes priority over the resize borders that lie in the content, as it does in gpui. `HTMAXBUTTON` is what makes Windows 11 show its snap layouts over the `Max` area. A window with the system title bar returns the area where one answers and otherwise what `DefWindowProcW` returns.
- `WM_NCLBUTTONDOWN` and `WM_NCLBUTTONDBLCLK` over `HTCLOSE`, `HTMAXBUTTON` and `HTMINBUTTON` are taken and remembered when the window has no system title bar, or when the last hit test answered that code from an area. `WM_NCLBUTTONUP` over the same code runs the action, `WM_CLOSE` posted, `toggle_maximize` or `minimize` through `ShowWindowAsync`, as gpui's handlers do (`events.rs:63`, `949-998`, `1000-1065`). A release over another code, or none pressed, runs nothing. The press is forgotten by `WM_LBUTTONUP`, `WM_NCMOUSELEAVE`, `WM_CAPTURECHANGED` and the release itself, so a release elsewhere followed by an unpaired `WM_NCLBUTTONUP` closes nothing. Other messages go to `DefWindowProcW`, so `HTCAPTION` drags and double-clicks as the system does, and the system's own caption buttons of a titled window keep their default handling. With the title bar hidden, `WM_NCMOUSEMOVE` gives `PointerMoved` in content coordinates, requesting leave tracking with `TME_LEAVE | TME_NONCLIENT` where `WM_MOUSEMOVE` requests `TME_LEAVE`, and `WM_NCMOUSELEAVE` gives `PointerLeft`, and also reaches `DefWindowProcW` when the window has its system title bar; the caption buttons themselves give no pointer event for a click. A `WM_SIZE` that minimises the window leaves the maximised state as it was.
- The window requests pointer leave with `TrackMouseEvent` on the first move. `PointerLeft` comes only when it was requested. A press captures the pointer until every button is up. The extra buttons are `Other(4)` and `Other(5)`. One wheel notch scrolls 48 logical pixels. A `Scrolled` event takes its Shift and Control from the wheel message's key flags and its Alt and Super from `GetKeyState`, as a button event does.
- Virtual-key codes map to `Key`, with `ToUnicode` giving the character with only Shift applied. Control with Pause arrives as `VK_CANCEL`, which is `Break`, and `VK_APPS` is `Menu`. The text of a press comes from the `WM_CHAR` or `WM_SYSCHAR` that follows it in the same drain, with surrogate pairs joined.
- `next` drains the thread's message queue with `PeekMessageW`. `PeekMessageW` dispatches the messages of every window on the thread to its own window's queue. When nothing is pending, `next` waits in `MsgWaitForMultipleObjectsEx` with the timeout. The OS thread is in a C call while it waits, so other green threads keep running. A `Waker` of a window posts `WM_APP` to the window, and `WM_NCDESTROY` and releasing the window detach it. A window destroyed by the system clears its handle, so releasing it destroys nothing twice.
- The clipboard holds `CF_UNICODETEXT`. `write_clipboard` converts the text to UTF-16, with a surrogate pair for each code point above U+FFFF and a terminating zero, into a `GMEM_MOVEABLE` block. It opens the clipboard with the window as its owner, trying every 5 ms up to 20 times before failing with `Failed("OpenClipboard", code)`. It empties the clipboard and gives the block to `SetClipboardData`, which then owns it. The block is freed when any step fails, and the clipboard is closed on every path. A text that holds a zero ends there. `read_clipboard` converts the block back to UTF-8 up to its first zero within `GlobalSize`. A clipboard with no `CF_UNICODETEXT`, which Windows also synthesises from `CF_TEXT`, gives `none`. A released window gives `Disconnected`.
- Text input is off by default. `open` detaches the input context with `ImmAssociateContextEx(hwnd, null, 0)`. `set_text_input` with a caret attaches the default context with `IACE_DEFAULT`. `none` detaches it again and, when a composition was showing, queues `Preedit("", none)`. The caret is placed in device pixels each time it is set, and at `WM_IME_STARTCOMPOSITION`, `WM_SETFOCUS` and a DPI change. `ImmSetCompositionWindow` puts the composition at the caret's bottom left (`CFS_POINT`), and `ImmSetCandidateWindow` keeps the candidate window off the caret's rectangle (`CFS_EXCLUDE`).
  - `WM_IME_STARTCOMPOSITION`, `WM_IME_COMPOSITION` and `WM_IME_ENDCOMPOSITION` are handled without `DefWindowProcW`, so the system draws no composition window and does not turn the result into `WM_IME_CHAR` and `WM_CHAR`. `WM_IME_NOTIFY` still reaches it, so the candidate window shows. All three are ignored while text input is off.
  - `GCS_RESULTSTR` becomes `Commit`, which also clears the composition. `GCS_COMPSTR` becomes `Preedit`, with a collapsed `TextRange` at the `GCS_CURSORPOS` offset converted from UTF-16 units to UTF-8 bytes, or `none` when the input method gives no position. An empty composition string and `WM_IME_ENDCOMPOSITION` give `Preedit("", none)` once.
  - `WM_CHAR` never gives a `Commit`: its text joins the press before it as `KeyEvent.text`, with or without text input. A key the input method handles arrives as `VK_PROCESSKEY`, which gives no `Key` event, so its `WM_CHAR` has no press to join and is dropped.
- `set_cursor` maps each `Cursor` to a stock cursor from `LoadCursorW`: `Arrow` to `IDC_ARROW`, `Text` to `IDC_IBEAM`, `Pointer`, `Grab` and `Grabbing` to `IDC_HAND` (Windows has no open or closed hand), `Crosshair` to `IDC_CROSS`, `NotAllowed` to `IDC_NO`, `Move` to `IDC_SIZEALL`, `Wait` to `IDC_WAIT`, `Progress` to `IDC_APPSTARTING`, `Help` to `IDC_HELP`, `ResizeLeftRight` to `IDC_SIZEWE`, `ResizeUpDown` to `IDC_SIZENS`, `ResizeUpLeftDownRight` to `IDC_SIZENWSE` and `ResizeUpRightDownLeft` to `IDC_SIZENESW`. `Hidden` is `SetCursor(null)`.
  - `WM_SETCURSOR` applies the window's cursor and returns `TRUE` when the hit test is the client area. It leaves the others to `DefWindowProcW`. `set_cursor` also calls `SetCursor` at once while pointer-leave tracking is active, which is from the first `WM_MOUSEMOVE` until `WM_MOUSELEAVE`. Otherwise the next `WM_SETCURSOR` applies it.
- Files dragged from the shell arrive through an `IDropTarget`. A new window registers it with `RegisterDragDrop` (`platform/windows/window.rs:1237-1249`) after its display's `OleInitialize` (`platform/windows/platform.rs:93`). Releasing the window revokes it with `RevokeDragDrop` (`window.rs:510`) before its display's `OleUninitialize`. The target is a Low object whose vtable holds callbacks, written as the `IDropTarget` of `window.rs:880-1010` is. The window's state is reached through the object. Its `QueryInterface` answers `IUnknown` and `IDropTarget`, comparing all 16 bytes of the interface id. It returns `E_POINTER` for a null out pointer. `DragEnter` asks the data object for `CF_HDROP` with `QueryGetData` and `GetData`, reads the paths with `DragQueryFileW`, releases the medium with `ReleaseStgMedium` and gives `FileDragEntered`. `DragOver` gives `FileDragMoved`, `Drop` gives `FileDropped` and `DragLeave` gives `FileDragExited`. The point each call receives is in screen pixels, which `ScreenToClient` and the window's scale turn into logical pixels from the top left of the window. Each call is also forwarded to an `IDropTargetHelper` made with `CoCreateInstance` of `CLSID_DragDropHelper` (`platform.rs:143`, `window.rs:931-934`, `948-951`, `971`, `990-993`) and released on revoke. The shell's drag image therefore shows over the window. A window whose helper cannot be made works without the image. A drag with files answers the first of `DROPEFFECT_COPY`, `DROPEFFECT_MOVE` and `DROPEFFECT_LINK` that the source allows in the `*pdweffect` it passes in, where gpui always answers copy. With none allowed, or with no files, the drag answers `DROPEFFECT_NONE`, and a drag without files gives no event. A drag whose mouse capture is lost without a release (`WM_CAPTURECHANGED`) is not ended. The platform gives no event for it, and the next release ends it.

### Renderer

- `gpu.open() -> Gpu ! Error` creates a Vulkan 1.3 instance and device on the first physical device that offers dynamic rendering, synchronization2 and a graphics queue. `gpu.open_for(Surface) -> Gpu ! Error` also needs `VK_KHR_swapchain` and a queue that presents to the window. It owns the `VkSurfaceKHR` it makes from the `Surface`'s display and window handles. `gpu.adapter(Gpu) -> str` names the physical device it chose. `Gpu` is droppable. It waits for the device to go idle and destroys everything it made. A `Target` made from it is droppable too and must be released first. `using` does this when it is bound after the `Gpu`.
- A `Gpu` is one `Device` with one `Presenter`. `gpu.open_device(first: Surface) -> Device ! Error` opens the first Vulkan 1.3 device that offers `VK_KHR_swapchain` and a graphics queue that presents to `first`. It then destroys the probe surface it made, so the `Device` keeps no window. `gpu.presenter(d, Surface) -> Presenter ! Error` makes the `VkSurfaceKHR` and swapchain state of one window. It fails with `NoDevice` when the queue family of `d` cannot present to that surface. `gpu.present_to(p, scene, mut atlases)` draws into the next image of `p`'s swapchain on the `Device` that made `p`. A `Device` shares one frame ring, one set of pipelines keyed by surface format and one texture per atlas between its presenters and targets. Rows uploaded by `present_to` with one presenter are therefore sampled by the next draw with any other. A `Device` keeps its live presenters and targets. Releasing a `Presenter` waits for the device to go idle and destroys its swapchain and surface. It must come before its window is released. A window takes one `Presenter`. On Windows the `present_to` of a second one fails with `VK_ERROR_NATIVE_WINDOW_IN_USE_KHR`. Releasing the `Device` first destroys every presenter's swapchain and surface and every target's image. So a later `Presenter` or `Target` release does nothing and `present_to` with a `Presenter` fails with `NoDevice`. Releasing a `Gpu` releases its presenter and then its device, and every `Target` still alive with it. A `Target` drawn to or read through a different `Gpu` or `Device` fails with `WrongDevice`.
- `gpu.Error` is `NoDevice`, `Vulkan(call, result)` for a failed call and its `VkResult`, `NoMemoryType(allowed, flags)` when no memory type matches, `InvalidSize(width, height)` for a target smaller than one pixel, and `NoSurface` for `present` on a `Gpu` without a window. `describe(e)` gives it as text.
- A `Gpu` or `Device` keeps one texture for each `scene.Atlas` of the list it was last given, which mirrors that atlas. It makes a texture again when its atlas changes size and drops the textures past the end of the list. A sprite names the atlas it samples by its index in the list. A sprite whose index is negative or not in the list fails the draw with `InvalidTexture` before anything is written. Each atlas uploads its own dirty rows, and `render` and `present` clean every atlas of the list once the frame is submitted.
- It holds seven pipelines, one per primitive kind, in the order of `scene.kinds()`, and an eighth that composites rasterised paths, sharing one descriptor set layout (the instance storage buffer at binding 0, an atlas texture and its linear, edge-clamped sampler at binding 1) and one pipeline layout whose push constants are the target size in pixels at offset 0 and the scene's `TextParams` at offset 16, 48 bytes in all. The subpixel pipeline needs dual-source blending, which `supports_subpixel(g)` reports. Without it the subpixel kind draws with the sprite shader. There are two frames in flight. Each owns a command buffer, a fence, its semaphores, one host-visible instance buffer per kind that grows by doubling, and one more for the path sprites. Each buffer has a descriptor set for every atlas texture the kind samples, and one for the kinds that sample none. The path sprites' set binds the resolved path image at binding 1. The sets come from a list of descriptor pools that grows by one pool when the last is full.
- Drawing a scene writes each kind's instances into its buffer in draw order. It uploads each atlas's dirty rows through a staging buffer. It records one `vkCmdDraw` of six vertices per batch, with `firstInstance` at the batch's first instance and the descriptor set of the batch's texture bound. The draws sit inside one dynamic rendering pass that clears to the scene's background. A batch of paths ends that pass, rasterises its triangles and composites them, then resumes the pass with the target's contents loaded. The `Paths` buffer holds the vertices of every path, three to a triangle. The batch is drawn with `vkCmdDraw` over its range of them into a four-sample `R8G8B8A8_UNORM` image. The render area of that image is the batch's clipped bounds rounded out to whole pixels. The samples are averaged into a single-sample image, which a barrier makes readable. The main pass then copies that image into the target through one `vkCmdDraw` of six vertices per path sprite. Each path sprite is a rectangle that is read with `texelFetch` at the fragment's pixel. When every path of the batch has one draw order, the sprites are the paths' own clipped bounds, which are disjoint. Otherwise there is one sprite over their union, so a pixel several paths cover is composited once. Each `Device` makes the two path images on the first batch. It grows them when a batch needs more, to a multiple of 256 pixels on each side, after waiting for the device to go idle. It keeps them until it is released, so a path drawn into a target or window of any size reuses them.
- `target(g, width, height)` makes an offscreen `Target`, an `R8G8B8A8_UNORM` image. `render(g, t, scene, mut atlases)` submits a frame and returns, and `read(g, t)` waits for the queue to go idle and returns the pixels as RGBA bytes.
- `present(g, scene, mut atlases)` draws into the next image of the `Gpu`'s swapchain and queues it. `present_to` does the same for one `Presenter` of a `Device`. Presents run one after another on one queue, each waiting up to thirty seconds to acquire an image, so a window that stops consuming frames stalls the others. A draw that fails after its image is acquired submits an empty batch that waits on the acquire semaphore and marks the swapchain for rebuilding, so the frame slot can be reused. The swapchain has a `B8G8R8A8_UNORM` or `R8G8B8A8_UNORM` format and FIFO presentation. It is made on the first call at the scene's size in device pixels, or at the surface's current extent when the surface fixes one. It is made again when that size changes, or when acquiring or presenting reports `VK_ERROR_OUT_OF_DATE_KHR` or `VK_SUBOPTIMAL_KHR`. Each frame in flight has its own acquire semaphore and each image its own render-finished semaphore. A scene of no pixels, or a minimised window, presents nothing.
- Colours are sRGB-encoded and blended in that space with premultiplied-free `SRC_ALPHA, ONE_MINUS_SRC_ALPHA` blending. The path pipelines write premultiplied colour and blend with `ONE, ONE_MINUS_SRC_ALPHA`, both into the rasterised image and from the resolved image into the target, as gpui's path pipelines do.
- C structures that Vulkan reads through a pointer are pinned heap values kept by a droppable `Pins` list for the duration of the call or the frame.
- The shaders:
  - Quads have per-corner radii and per-edge border widths. The fragment shader reads the pixel centre from `gl_FragCoord`. It returns the background where no border or corner reaches it. Otherwise it computes the signed distance to the outer edge and an approximate one to the inner edge. The inner edge is a circle where the two border widths of a corner agree and a quarter ellipse where they differ. It blends the border colour over the background with the straight-alpha `over` operator, mixes by the inner distance, and multiplies the alpha by the outer distance covering one pixel of anti-aliasing.
  - Shadows are a rounded rectangle blurred by a Gaussian of the given sigma. The blur is integrated in closed form along x with a polynomial approximation of the error function, and sampled four times along y. A shadow is drawn over the rectangle grown by three sigma. A shadow without blur is the rounded rectangle's signed distance covering one pixel.
  - Underlines are the rectangle of their bounds in one colour. A wavy one is drawn as gpui's `underline_fragment` does (`shaders.hlsl:1108-1130`). With the height of the bounds as the unit, the stroke follows `sin(x * frequency) * amplitude` about the middle row. Here `frequency` is `PI * 2 * thickness / height` and `amplitude` is `thickness * 0.8 / height`. The alpha is `saturate(0.5 - max(-d_bottom, d_top))` for the distances to the stroke's two borders. These distances are measured along the normal of the curve and converted to pixels.
  - Sprites sample the first byte of the `R8G8B8A8` atlas as coverage for a monochrome glyph tinted with the sprite's colour. When the scene's `TextParams` are not all zero the coverage first goes through DirectWrite's gamma-ratio correction with the grey contrast, as the reference renderer does for grey glyphs.
  - Subpixel sprites sample the first three bytes as one coverage per colour stripe, in blue to red order when `TextParams.bgr` is set. The fragment shader returns the colour with the correction applied per stripe, using the subpixel contrast and the colour's own brightness. It returns this as the second blend source. The blend is `SRC1_COLOR, ONE_MINUS_SRC1_COLOR` over the target's red, green and blue channels.
  - Polychrome sprites sample all four bytes of the `R8G8B8A8` atlas as a colour with straight alpha, with the sampler's linear filtering, as gpui's `polychrome_sprite_fragment` does (`shaders.hlsl:1210-1270`). Unlike gpui's, the shader samples the mip level the sprite names in `level`. It reads the two whole levels either side with `textureLod` and blends them by the fraction. At each level it clamps the coordinate to half a texel inside the sprite's tile, or to the tile's centre when the tile is narrower than a texel, so linear filtering reads the tile's own edge and never a neighbour's. The sprite and subpixel shaders read level 0 with `textureLod`, so an icon rasterised at twice its size is not drawn from a mip level the sampler would pick. The fragment shader optionally replaces the colour with its luminance (`0.2126, 0.7152, 0.0722`). It multiplies the alpha by the sprite's opacity and by `saturate(0.5 - quad_sdf)` over the sprite's rounded bounds. The blend is the same `SRC_ALPHA, ONE_MINUS_SRC_ALPHA` as for the other sprites and gpui's polychrome blend state (`directx_renderer.rs:1512-1527`). gpui's decoded images are straight alpha too (`img.rs:678-712`).
  - Paths are rasterised from triangles. Each vertex carries a position, the curve coordinates `st`, the path's colour and a clip rectangle. The clip rectangle is the path's bounds cut to the clip it was pushed under. A vertex is 48 bytes in the renderer and 16 in the scene. A solid triangle has `st` of `(0, 1)`, and every pixel inside it is covered. A curve triangle has `st` of `(0, 0)`, `(0.5, 0)` and `(1, 1)`. For it the fragment shader computes `f = s * s - t`, the screen-space gradient of `f` with `dFdx` and `dFdy`, and the alpha `saturate(0.5 - f / length(gradient))`. This is the Loop-Blinn coverage of gpui's `path_rasterization_fragment` (`shaders.wgsl`). A triangle whose `s` changes by less than `0.001` per pixel is solid. The fragment shader outputs the colour multiplied by that alpha and by the path's own alpha, premultiplied. It gives zero alpha to a fragment whose pixel centre is outside the vertex's clip rectangle. It receives that rectangle as four distances to the sides, interpolated across the triangle. The path sprite fragment shader returns the resolved texel at `gl_FragCoord`.
  - Every other instance carries a clip rectangle, and fragments outside it are discarded by alpha.

### Scene

- A `Scene` holds `Quad`, `Shadow`, `Path`, `Underline`, `Sprite`, `SubpixelSprite` and `PolychromeSprite` instances in device pixels. Each has its clip rectangle and its draw order, and each sprite has the index of the atlas it samples. A `Scene` holds the `TextParams` the glyph shaders read: four gamma coefficients, the grey and subpixel contrasts, and the stripe order. All zero is the identity correction.
- `push_clip` and `pop_clip` keep a stack of clip rectangles. Each primitive takes the intersection of the stack.
- A primitive's draw order is one more than the greatest draw order among the earlier primitives whose bounds, cut to the clip, it overlaps. The draw order is 1 when it overlaps none. A primitive whose cut bounds are empty is dropped. Touching edges do not overlap. A shadow's bounds are those before blurring. A grid of 64-pixel cells holds the placed bounds, so a push tests only the primitives in the cells it covers. A quad with a transparent background is split into the four strips around its interior. The interior is the quad inset by one pixel and by the larger of each border width and corner radius, taking the larger of the two bands that leave the corners out. Each strip is placed on its own, so a primitive pushed inside the border draws below it. A quad whose interior is empty stays whole.
- Each list is kept sorted by draw order, a primitive going after those of equal draw order. `grouped(s)` returns the scene with each sprite list sorted by draw order and then by atlas. It uses one stable sort that takes `O(n log n)` time however the atlases interleave and moves nothing when the lists are already in that order, so sprites of one draw order and one atlas keep the order pushed where gpui breaks the tie by tile. Sprites of one draw order never overlap, so gathering them by atlas changes no pixel. The renderer draws `grouped(s)`. `push_sprite`, `push_subpixel_sprite` and `push_polychrome_sprite` sample atlas 0. `push_sprite_from`, `push_subpixel_sprite_from` and `push_polychrome_sprite_from` take the atlas index. `batches` merges the lists into runs of one kind and, for sprites, one atlas. At each step it selects the lowest draw order and, among equal ones, the kind that comes first in `kinds()`. That order is shadows, quads, paths, underlines, sprites, subpixel sprites, polychrome sprites, which is the order of gpui's `PrimitiveKind`. A `Batch` carries its kind, its first instance, its count and its `texture`. The `texture` is the atlas its sprites sample, and it is 0 for every other kind. `Kind.index` is the position in `kinds()`, and the renderer keeps one pipeline and instance buffer per index and one descriptor set per index and texture. A `scene.Atlas` holds four bytes per pixel. `write` fills the first byte with coverage, `write_stripes` the first three and `write_colors` all four. `extend_edges` repeats a tile's last column and row out to a larger block. An atlas holds mip levels above its pixels once `add_levels` gives it some, each half the size of the one below, and the renderer gives its texture as many. `reduce` rebuilds them over a block whose corners are multiples of `2^count`. Each texel is the mean of the two by two below it, its colour weighted by alpha. A change to rows `top` to `bottom` of the pixels uploads rows `top >> k` to `bottom` divided by `2^k`, rounded up, of level `k`. A `PolychromeSprite` carries its bounds, clip, atlas tile, four corner radii, a grayscale flag, an opacity and the mip level it samples, 80 bytes. An `Underline` carries its bounds, clip, colour, a thickness and a wavy flag as a float, 64 bytes. `push_underline` places it by its bounds as any other primitive.
- A `Path` is a list of `PathVertex`, three to a triangle. A `PathVertex` is a position and the curve coordinates `st`, 16 bytes. A `Path` has the colour it fills with, the bounds that hold its start and every vertex, the clip `push_path` gives it and its draw order. `path(start, color)` starts one. Its methods follow gpui's `Path`. `move_to` starts a contour. `line_to` adds the triangle from the contour's first point to the last point and the new one once a contour has two edges. `curve_to(to, ctrl)` adds a curve triangle from the current point through the control point to `to`, after the same fan triangle. `push_triangle` adds one. A `line_to` straight after a `move_to` adds a triangle with two equal corners, as gpui's does. `push_path` places a path by its bounds, as every primitive is placed. A path whose bounds are empty once clipped is dropped. No fill rule exists: a path covers the union of its triangles. This is the interior of a convex or fan-visible outline, and not that of a concave or self-intersecting one. A translucent colour darkens a pixel once for every triangle that covers it. `path_copies(s, b)` gives the rectangles of a batch of paths that the renderer composites, per path when the batch has one draw order and as one union otherwise.
- `stroke_polyline(points, width, color)` builds a path for a line of `width` device pixels. The path is a strip of quads with butt caps. A corner shares one mitred vertex pair between the two segments. It is bevelled when the mitre is longer than four half-widths. Where a segment is shorter than the reach of its corners, the corner uses overlapping quads, so the stroke has no gap or spike. A translucent stroke is darker where they overlap. `stroke_quad` and `stroke_cubic` stroke the polyline from `flatten_quad` and `flatten_cubic`. The points from those two functions are within a tenth of a pixel of the curve. Round and square caps, joins other than mitre and bevel, and dashes are not drawn.

### Layout

- `tree(scale)` makes a `layout.Tree` for a display of `scale` device pixels per logical pixel. The tree holds nodes in a slice. `node(mut t, style, children)` and `leaf(mut t, style, measure)` add one and return its index. `compute(mut t, root, window)` places every node in a window of `window` logical pixels. `rect(t, node)` is its border box in logical pixels relative to the window, and `content(t, node)` is its content box.
- The engine is the flexbox algorithm of taffy 0.15 with the pixel snapping of gpui-fast 0.1.12 (`taffy.rs` in that crate):
  - A node's authored lengths are multiplied by the scale and rounded to whole device pixels, a tie toward zero. The lengths are sizes, minimums, maximums, `basis`, margins, insets, padding and gaps. A border width is `0` when zero and otherwise at least one device pixel. A percentage is kept as a fraction.
  - A leaf's measure function is called with logical widths and its answer is multiplied by the scale and rounded up to whole device pixels.
  - The tree is laid out in device pixels. The origin and far corner of each node's border box are summed unrounded from the root and rounded to whole device pixels with a tie toward zero. `rect` divides them by the scale, so edges that meet in the window round alike.
  - A root with an `Auto` width or height is as large as the window on that axis, rounded as a length is. Where the window is `unbounded`, it is as large as its content.
- A `Style` has:
  - `display` (`Flex`, or `None` for a node that takes no space), `position` (`Relative`, or `Absolute` with an `Inset` of one `Dimension` per side);
  - `direction` (`Row` or `Column`), `wrap` (`Single` or `Multi`), `justify` (`Start`, `End`, `Center`, `SpaceBetween`, `SpaceAround`, `SpaceEvenly`), `align_items` (`Start`, `End`, `Center`, `Stretch`), `align_content` (`Start`, `End`, `Center`, `Stretch`, `SpaceBetween`, `SpaceAround`, `SpaceEvenly`) and `align_self` (those or `Auto`, which follows `align_items`);
  - `grow`, `shrink` and `basis`; `width`, `height` and their minimums and maximums as `Auto`, `Px(f32)` or `Percent(f32)`, where `Percent(50.0)` is half of the parent's inner size, or of its padding box for an absolute node;
  - `margin` as a `Margin` of one `Dimension` per side, where `Auto` absorbs free space and a negative length pulls the node over its neighbours; `padding` and `border` edges in pixels; `column_gap` and `row_gap`;
  - `overflow_x` and `overflow_y`, each `Visible`, `Hidden` or `Scroll`.
- Sizes, minimums, maximums and `basis` are border-box lengths, and a box is never smaller than its padding and border.
- An `Auto` minimum along the parent's main axis is the automatic minimum of a flex item. It is the smaller of its specified size and its content's narrowest size, never above its maximum. It is zero when the node's overflow on that axis is `Hidden` or `Scroll`. The narrowest width of a leaf is its measure function's answer for an `unbounded` width, so text does not wrap when it is asked for its narrowest size. The narrowest width of a container is the sum or maximum of its children's. A node's narrowest height is its height at the width it is laid out at. An `Auto` minimum on the cross axis is zero.
- The algorithm follows the CSS flexbox steps as taffy 0.15 orders them, including lines, `align_content`, auto margins, relative insets and the cache of measured sizes. It leaves out block and grid display, right-to-left and reversed directions, `wrap-reverse`, baseline alignment, `align-content: balance`, aspect ratios, sizing keywords and scrollbar gutters.
- Absolute children are placed against the padding box of their parent by their insets and do not take part in the flow. One with no inset on an axis is placed on it by the parent's `justify` or `align_items`. Every node is a containing block, so an absolute child is laid out by its immediate parent.
- A leaf's measure function takes the width available to it and returns its size. The width is `unbounded` when unconstrained, and `is_bounded` tells the two cases apart.
- A node with `display: None` and every node under it get an empty rectangle at the top left of the node's parent.
- `testdata/ui_layout/` holds trees and the rectangles taffy 0.15 gives them under gpui-fast's rounding at scales 1, 1.25, 1.5 and 1.75 in a 640 by 480 window. `std/ui/layout/oracle_test.low` lays out each tree with `layout` and requires every rectangle to equal the stored one. A tree with a reversed wrapping container is left out. `testdata/ui_layout/README.md` describes the file format.

### Text

- `font.parse(data) -> Font ! Error` reads a TrueType font with `glyf` outlines, or an OpenType font with the `OTTO` signature and `CFF ` outlines. It reads the `head`, `hhea`, `maxp`, `cmap` and `hmtx` tables, either the `loca` and `glyf` tables or the `CFF ` table, and the optional `kern` table. It uses a `cmap` subtable of format 12 or 4, preferring a Unicode one that covers every plane. It uses the first horizontal `kern` subtable of format 0. A `maxp` of version 0.5 is read.
  - `parse_face(data, index)` reads font `index` of a TrueType or OpenType collection, which has the `ttcf` signature. The table offsets of a collection count from the start of the file. For a single font it reads font 0. It fails with `Malformed("header")` for an index the file does not hold. `parse(data)` is `parse_face(data, 0)`. `face_count(data)` is the number of fonts a collection holds. It is 1 for a single font, and 0 for anything else or a collection whose offsets run past the data.
  - It fails with `Unsupported(table)` for CFF2 outlines and for `CFF ` outlines in a font without the `OTTO` signature. It fails with `Malformed(table)` for a table directory entry, `loca` offset or `CFF ` structure that runs past the data or disagrees with itself.
  - The parser checks a `CFF ` table in full when it parses the font. The checks cover the header, the Name, Top DICT, String and Global Subr INDEXes, the Top DICT's `CharStrings`, `Private`, `FontMatrix`, `ROS`, `FDArray` and `FDSelect` operators, each Private DICT with its `Subrs` INDEX, and the `FDSelect` of format 0 or 3. An `FDArray` holds at most 256 font dictionaries, the most an `FDSelect` can name. The `CharStrings` INDEX must hold as many charstrings as `maxp` has glyphs. The `CharstringType` must be 2. A CID-keyed font takes each glyph's Private DICT, local subroutines and `FontMatrix` from the font dictionary its `FDSelect` names. A `FontMatrix` in both the Top DICT and a font dictionary is composed. The matrix maps the charstring's units to `units_per_em`, so the default `0.001` leaves a 1000-unit font unscaled.
  - Glyph data and lookup tables are read when used, and a malformed one gives glyph 0, an empty outline or a zero metric.
  - `Font` metrics (`units_per_em`, `ascent`, `descent`, `line_gap`, `glyph_count`) are `i64` font units with y up. `scale(f, px)` converts them to pixels, and `glyph_index`, `advance` and `kern` look up one glyph or pair.
- `Font` also carries `underline_position`, `underline_thickness`, `strikeout_position` and `strikeout_thickness` in font units. They come from the `post` table (offsets 8 and 10) and the `OS/2` table (offsets 28 and 26). When a table is missing or short, the values are a tenth of an em below the baseline, a third of an em above it and a twentieth of an em thick. A position is the distance of the stroke's top from the baseline, negative below it.
- `font.describe(data, index) -> Description ! Error` reads what a font index needs without parsing the outlines. The families are the typographic family (name ID 16), then the family (ID 1), from Windows Unicode, Unicode or Mac Roman records, Windows US English first. The style is ID 17, else ID 2. The `OS/2` weight and width class are 400 and 5 when the font has no `OS/2` table. `italic` comes from `fsSelection` bit 0 or the `head` `macStyle` italic bit. `oblique` comes from `fsSelection` bit 9. `drawable` (a `glyf` with `loca`, or a `CFF ` table, besides `cmap`, `head`, `hhea`, `hmtx` and `maxp`). It reads the code points the `cmap` maps to a glyph other than 0 as sorted `Range`s, and the `fvar` named instances. An instance's weight is its `wght` coordinate rounded, else the font's weight. An instance is italic when its `ital` coordinate is at least 0.5. Without an `ital` axis, it is italic when the font is italic or the instance's name has the word Italic. An instance is oblique when its `slnt` coordinate is not 0. Without a `slnt` axis, it is oblique when the font is oblique or the name has the word Oblique. `covers(runs, code_point)` searches the ranges.
- `font.fallback()` is Inter Variable 4.1 at its default instance (weight 400, optical size 14), embedded under the SIL Open Font License in `std/ui/font/LICENSE`. `font.fallback_file()` returns the file. The parser reads the default outlines and ignores the variation tables.
- `outline(f, glyph)` gives a glyph's contours of on- and off-curve points. Composite glyphs are expanded through their offsets and scales, at most 6 deep and 64 components in all. `rasterize(f, glyph, px)` gives a `Bitmap` of coverage bytes with its offset from the pen. `rasterize_slanted(f, glyph, px, slant)` shears the outline right by `slant` times its height first.
  - A `glyf` contour is quadratic. A `CFF ` glyph runs its Type 2 charstring. The charstring draws every path operator. It takes the hint, mask and width operators without drawing them. It calls local and global subroutines with their bias. It evaluates the arithmetic and storage operators. The glyph gives cubic contours (`Contour.cubic`). The first point is on the curve, each later on-curve point is a line, and each two off-curve points and the on-curve point after them are one cubic curve.
  - A charstring with more than 48 values on the stack, subroutines nested deeper than 10, a subroutine that does not exist, more than 200000 operations, a `seac` form of `endchar`, no `endchar` or any other malformed operator gives an outline with no contours.
- The rasteriser scales the outline to pixels. It flattens each quadratic curve into lines whose count grows with the curve's size. It flattens each cubic curve by halving it until the larger second difference of its control points is within 4/3 of 0.05 pixel, so the lines stay within 0.05 pixel of the curve. It accumulates each line's signed area and cover into an `f32` buffer. A running sum along each row gives the coverage, clamped to 0..1 and stored as one byte per pixel.
- `std/ui/fontconfig` finds the installed fonts on Linux without libfontconfig. `environment()` gives the configuration file (`FONTCONFIG_FILE`, else `fonts.conf` in `FONTCONFIG_PATH` or `/etc/fonts`) and the XDG directories. `read_config(env)` reads it with `std/xml`.
  - Only `<dir>`, `<include>` and `<alias>` are read. An `<include>` of a directory reads the files in it whose names start with a digit and end in `.conf`, in byte order. `ignore_missing` is not needed, because a missing or unreadable file, or one that is not XML, is skipped. Each file is read once and includes nest at most 64 deep. `~` is `env.home`. `prefix="xdg"` puts a `<dir>` under `env.data_home` and an `<include>` under `env.config_home`. Another relative `<include>` is under the including file's directory, and so is a `<dir>` with `prefix="relative"`. Any other relative `<dir>` is under `env.working_dir`. The `fonts` directories of `env.data_home` and each of `env.data_dirs` follow the configured ones.
  - `open(env)` walks the directories. It follows symbolic links, reads a directory reached twice once, and goes at most 32 deep. It indexes every font of every `.ttf`, `.otf`, `.ttc` and `.otc` file that `font.describe` reads, has a family and is `drawable`. A variable font gives one face per named instance, read as the default instance because the parser ignores the variation tables. The faces are cached in `low/fontconfig.cache` under `env.cache_home`, one record per directory keyed by its modification time. A directory whose time is unchanged is not read again. The cache is written to a temporary file that then takes its place. A damaged or foreign cache is ignored. On a machine with 934 installed faces the first `open` takes about 1.1 s and a cached one about 16 ms.
  - `empty()` is an index with no installed faces. `add(mut ix, data)` adds the faces of a file in memory, which every match tries before the installed ones. `families(ix, family)` expands a family through the aliases in the order the configuration gave them: `prefer` before it, `accept` after it and `default` at the end. It compares names without ASCII case or spaces. `find(ix, family, weight, slant)` takes the first family in that list with a face. It chooses among that family's faces as CSS font matching does. It matches width first (normal, else narrower, else wider). It then matches slant (italic falls back to oblique then upright, oblique to italic then upright, upright to oblique then italic). It matches weight last (a weight from 400 to 500 tries heavier faces up to 500, then lighter, then heavier; below 400 lighter then heavier; above 500 heavier then lighter). `fallback(ix, family, weight, slant, code_point)` does the same over the faces whose `cmap` has the code point, through the families of `family` and then of `sans-serif`, else within the family of the first face that has it, added faces first. `read_face(face)` parses a face. `std/ui/fontconfig/examples/matching.low` matches against the embedded Inter Variable.
- `text.System` holds the fonts, in order of preference, the glyph atlases and the caches. The atlases are `atlases`, which the renderer is given. A sprite names its atlas by index. `system(fonts)` makes one. `default_system()` makes one with only the fallback font. `system_with_atlas(fonts, size)` makes one with a single smaller atlas that never grows or gains a second. A code point is drawn with the first font that maps it, else with the first font's missing glyph.
- Shaping maps each code point to one glyph and places glyphs by their advances plus the `kern` pair adjustment when both come from one font. There are no ligatures, no `GPOS`, no bidirectional reordering and no complex scripts.
- `text.wrap(mut sys, s, style, width) -> Layout` breaks lines after spaces and at `\n`, and breaks a word longer than the width between glyphs. Spaces at the end of a line hang past it. A `Style` has the em size, the family, the weight, the `FontStyle` (`Normal`, `Oblique` or `Italic`) and the line height. With the built-in fonts every line has the first font's ascent, descent and line gap, centred in the line height when there is one. The family and weight are ignored unless the system uses the installed fonts on Linux, described below. The embedded Inter Variable has no italic face, so a style other than `Normal` rasterises each glyph with its outline sheared by 0.2126 of its height (`font.rasterize_slanted`), keeping the advances. The sheared glyphs are packed apart from the upright ones. A `Line` has `width` and `end`. `width` is the end of its last glyph that is not a space. `end` is the pen after its last glyph, where the next glyph would go. A glyph's `x` and `end` place a caret. `measure` gives a layout's size. Layouts of the same string, size and width are kept until `end_frame(mut sys)`, and `cached(sys)` counts them.
- `Style.decoration` is `None`, `Underline` or `Strikethrough`. A `Layout` carries it with a `Stroke`, which has `top` above the baseline and `thickness`, both in layout pixels. The `Stroke` comes from the metrics of the layout's first font, the installed face on Linux. When the layout is native, it comes from `dwrite.strokes`, DirectWrite's `DWRITE_FONT_METRICS` of the face. `paint` pushes one `scene.Underline` per line with an edge snapped to device pixels and a height of at least one device pixel. DirectWrite's own underline and strikethrough callbacks are not used. The layout sets no decoration, so `renderer_decoration` is never called. `set_subpixel(mut sys, subpixel)` switches an installed-fonts system between subpixel and grey glyphs and forgets every packed glyph.
- `use_system_fonts(mut sys, subpixel)` switches a system to the installed fonts through `std/ui/dwrite` and returns whether it could. A system that could not keeps the built-in fonts. It adds the embedded Inter Variable, the default family, so the default family draws whether or not it is installed. With the installed fonts, `wrap` finds the face of the style's family nearest its weight and `FontStyle`, else the default family. It gives `CreateTextFormat` the matching `DWRITE_FONT_STYLE`, so that a family with no italic or oblique face is drawn with DirectWrite's synthesised oblique. It shapes each paragraph as one line with ligatures and contextual alternates. Lines are `line_height` apart, or the face's ascent plus descent when that is zero, with the ascent and descent centred in the line. Lines break where gpui's `compute_wrap_boundaries` breaks them. A break is allowed between two characters by the rules of its `line_wrapper`: inside a word, after a space, around CJK text, never before closing punctuation. A line breaks when its width past the last break exceeds the width. A word wider than the width breaks between glyphs. The width of a laid-out text is the smaller of the width it wraps at and its unwrapped width, with trailing spaces counted. A `Style` with `wrap` false never breaks. With `truncate` `End`, `Start` or `Middle` and a width, a line wider than the width is cut one character at a time as gpui's `LineWrapper::truncate_line` cuts it. Trailing spaces and punctuation are dropped, and U+2026 marks the cut. `align` `Center` or `Right` moves each line within the width the text was laid out at, by the line's advance with its trailing spaces. A `Glyph` carries the UTF-16 unit where its cluster starts, so a break and a cut map glyphs back to characters. `add_font(mut sys, data)` adds the font file in `data` (a `str`, as `embed` gives it) to a system with the installed fonts through an in-memory DirectWrite font collection. A family the file holds is picked before an installed one of the same name. A context's `add_font(cx, data)` does the same for its text. On Linux, where there is no DirectWrite, `use_system_fonts` keeps the built-in fonts and the built-in rasteriser and returns whether `fontconfig.environment` gives an environment. `subpixel` is ignored. A layout with a family uses the face `fontconfig.find` chooses for its family, weight and `FontStyle` ahead of the built-in fonts. Its metrics, and the `Stroke` of an underline or strikethrough, are that face's. A layout with no family, or one no face matches, uses the built-in fonts. A code point no font of the layout maps, other than a control character, is drawn with the face `fontconfig.fallback` picks, `sans-serif` standing for no family. A family with no face of the `FontStyle` asked for gives the face `fontconfig.find` falls back to, so an installed oblique face answers `Italic` and the other way round. A family with only upright faces draws its upright face sheared by 0.2126 as the built-in fonts do. A face whose `fontconfig.Face.slant` is not 0 is drawn unsheared, glyph by glyph, so a built-in glyph in the same layout still shears. The installed faces are read through `fontconfig.open` on the first layout whose family no added face matches or whose code point no font has. The face found for each family, weight and slant, and for each code point, is kept. `add_font` adds the file to the index with `fontconfig.add`, so its families come before the installed ones. `params(sys)` returns the `TextParams` for the scene. They are the identity for the built-in fonts. For the installed ones they are the gamma coefficients, contrasts and stripe order that DirectWrite's rendering parameters give.
- `paint(mut sys, mut scene, layout, origin, color, scale)` pushes one sprite per visible glyph. Built-in glyph origins are snapped to whole device pixels. An installed font's glyph origin is split into whole device pixels, a quarter-pixel step across and a whole pixel down, each rounded with ties toward zero and carried into the whole part. The glyph is rasterised at that step, with `GetRecommendedRenderingMode` picking the mode. It is pushed as a subpixel sprite, or as a sprite when `subpixel` is off.
- `add_image(mut sys, width, height, rgba)` keeps a picture of red, green, blue and alpha bytes with straight alpha and returns an `Image`. A picture larger than the first atlas, up to 4096 pixels, makes the next `end_frame` grow it to hold it. A larger one is left out. `paint_image(mut sys, mut scene, image, fit, bounds, corners, grayscale, opacity, scale)` follows gpui's `Window::paint_image` (`window.rs:5121-5215`). `fit_bounds` places the picture in `bounds` by `ObjectFit::get_bounds` (`style.rs:42-139`). The visible part is the overlap of the two, snapped with `geometry.snap`. The tile is cut to the visible part by gpui's rounded ratios when it is smaller than the placed picture. The corner radii are limited to half the shorter visible side before they are scaled. A picture is packed into the atlas once, in a block that starts on a multiple of `2^levels` pixels. `levels` is the number of halvings, up to six, that keep its shorter side at least four texels. The block is rounded up to that multiple by repeating the last column and row (`scene.extend_edges`), and `scene.reduce` builds its mip levels. Every texel at those levels then averages only the picture, and the shader's clamp keeps a magnified sprite's edge from blending with transparent black or a neighbouring picture. The sprite's `level` is `log2` of the larger ratio of tile pixels to device pixels along a side, clamped to the picture's levels, and 0 when the picture is drawn at its own size or larger. So a picture drawn at half its size averages each two by two square instead of aliasing. gpui's atlas has no mip levels, and its tiles are packed without a border.
- `paint_icon(mut sys, mut scene, source, bounds, color, scale)` pushes one sprite that draws the SVG document `source` in `color`. `bounds`, in logical pixels, is snapped to device pixels with `geometry.snap`. `svg.render` rasterises the document at the snapped width times two, with the height from the document's proportions, into one atlas tile per source and size. The sprite is that tile's size halved and rounded up, centred on the snapped bounds with its origin rounded to a whole pixel, so the linear sampler averages each pixel's 2 by 2 texels. A document that does not parse, or has a snapped size under one device pixel, draws nothing.
- `svg.parse(source) -> Icon ! Error` reads `path`, `line`, `polyline`, `polygon`, `rect`, `circle` and `ellipse` under `svg` and `g`, with `viewBox`, `width` and `height`, and the inherited `fill`, `stroke`, `stroke-width`, `stroke-linecap` and `stroke-linejoin`. Shapes are built as `usvg` builds them. Path data is read in double precision as absolute coordinates and cast to single precision per point, with the reflection rules of `S` and `T`, an implicit move after a close, and quadratic curves kept quadratic. An elliptical arc, and the corners of a rounded rectangle and the sides of a circle or ellipse, become cubic curves by `kurbo`'s method. This method is the centre parameterisation of the SVG specification and a split into `ceil(max(3.999999, (1.1163 * r / 0.1) ^ (1/6)) * |sweep| / 2 pi)` equal cubics with arm length `4/3 * tan(step / 4)`. An arc whose radius is under `1e-5` or whose ends coincide is a line. A rectangle without a positive size, a circle or ellipse without a positive radius, a polyline or polygon with fewer than two points, and a path with fewer than two segments are dropped. A `transform`, `fill-rule="evenodd"`, any other element, and a stroke whose cap or join is not round fail with `Unsupported`. `svg.render(icon, width) -> Mask` computes the pixmap size and scale in single precision as gpui does for `resvg` (`scale = width / document width`, pixmap size `trunc(document size * scale)`, transform `scale(size / document size)`). Each shape is filled and then stroked in document order onto one coverage buffer. The fill is a port of tiny-skia's anti-aliased path filler. The filler supersamples four sub-scanlines and four sub-pixels per pixel, clips and chops curves into edges in fixed point, and composites source-over. The stroke is a port of tiny-skia's path stroker with round caps and joins, called with `res_scale = max(|sx|, |sy|)` of the transform, whose outline the filler then draws. The path is transformed point by point in single precision as `x * sx + tx`. A stroke one device pixel wide or thinner is drawn as a filled outline, where tiny-skia draws a modulated hairline. Colours and opacities are ignored because only coverage is kept. The cosine, arc cosine and power the stroker needs are the single-precision `cosf`, `acosf` and `powf` of the C library on Linux and Windows, and the double-precision functions rounded to single precision for `browser/wasm32`, which no test compares with `resvg`.
- An atlas is a `R8G8B8A8` image packed in shelves. A `text.System` starts with one of 1024 by 1024. A glyph is rasterised at its pixel size, rounded to 1/64 pixel, the first time it is drawn, with a one-pixel border. It goes into the open atlas, the newest one. A glyph, icon or picture larger than the first atlas, or one that does not fit in the room it has left, is left out together with everything packed after it in the frame. `end_frame` then replaces the first atlas with one twice as large, as often as needed up to 4096 by 4096, the smallest `maxImageDimension2D` Vulkan allows. The next frame packs its own text and pictures, `frame` marking the context due so that frame is drawn. Once the first atlas is 4096 by 4096 and full, `paint` adds an atlas of that size and packs into it, so what the frame drew stays drawn, up to eight atlases in all. The sprites name their atlas. `scene.batches` splits a sprite run where the atlas changes. The renderer binds each batch's texture. A tile that needs more than 4096 pixels across or down with its border, or a picture whose block does, is left out on its own, because 4096 is the largest image every Vulkan device accepts. In gpui, such a sprite gets a texture of its own (`min_size.max(&DEFAULT_ATLAS_SIZE)`). Each 4096 by 4096 atlas holds 64 MiB of pixels in memory and the same again in its texture, and a third more of each once it holds a picture's mip levels. The frame that first draws from it stages up to 64 MiB more. Eight atlases therefore cost up to 512 MiB of each. When the eight are full, `end_frame` clears the first atlas and drops the others. The frame after packs from the start. `system_with_atlas` keeps one atlas that neither grows nor gains another, and clears it when it fills.

### Elements

`std/ui` builds an interface from values and draws it with the modules above. For a window, a program imports `std/ui` and nothing else.

- An `Element` is a box or a text run. `div()` and `text(s)` make one. Each builder returns a changed copy, so an element is built by chaining. `children()` takes a trailing block of elements. The builders are `row`, `column`, `gap`, `padding`, `padding_xy`, `padding_top`, `padding_right`, `padding_bottom`, `padding_left`, `margin`, `margin_xy`, `margin_top`, `margin_right`, `margin_bottom`, `margin_left`, `margin_auto`, `margin_x_auto`, `margin_y_auto`, `margin_top_auto`, `margin_right_auto`, `margin_bottom_auto`, `margin_left_auto`, `size`, `width`, `height`, `width_percent`, `height_percent`, `w_full`, `h_full`, `size_full`, `min_width`, `min_height`, `max_width`, `max_height`, `grow`, `shrink`, `flex_1`, `flex_auto`, `flex_initial`, `flex_none`, `basis`, `basis_percent`, `flex_wrap`, `absolute`, `relative`, `top`, `right`, `bottom`, `left`, `inset`, `overflow_hidden`, `overflow_x_hidden`, `overflow_y_hidden`, `overflow_scroll`, `overflow_x_scroll`, `overflow_y_scroll`, `items_start`, `items_center`, `items_end`, `justify_center`, `justify_end`, `justify_between`, `background`, `border`, `border_top`, `border_right`, `border_bottom`, `border_left`, `border_color`, `rounded`, `shadow`, `opacity`, `invisible`, `visible`, `group`, `group_hover`, `group_active`, `text_color`, `text_size`, `font_style`, `italic`, `text_decoration`, `underline`, `strikethrough`, `nowrap`, `wrapping`, `ellipsis`, `ellipsis_start`, `ellipsis_middle`, `truncate`, `text_left`, `text_center`, `text_right`, `placeholder`, `placeholder_color`, `selection_color`, `id`, `track_scroll`, `window_control_area`, `child` and `children`. They set a `layout.Style` and a `Visual`, the colours, corners, shadows and text style, which are what `layout` and `scene` take. The handler builders `on_drag`, `on_drag_move`, `on_drop`, `can_drop` and `drag_over` add a listener or a condition and are described under hit testing and focus. `opacity` multiplies by its factor the alpha of each colour an element and its children draw, and the opacity of an image. The colours are the background, border, shadow, text, caret, selection and an icon's colour. Factors multiply down the tree, as gpui's `with_element_opacity` and `element_opacity` do (`window.rs:4287-4301`, `4380`; `div.rs:2627`). The opacity is applied once per primitive and not to a group, so overlapping children show through one another. `flex_1` sets `grow` and `shrink` to one and a basis of zero percent, as gpui's `flex_1` does (`styled.rs:181-187`), so in a parent without a definite main size it takes the content size. `width_percent` and `height_percent` set the width or height to a percentage of the parent's inner size on that axis, where `50.0` is half. `w_full`, `h_full` and `size_full` are `100.0` for the width, the height or both, as gpui's `relative(1.)` sets them.
- `nowrap` stops a text from breaking. `ellipsis`, `ellipsis_start` and `ellipsis_middle` cut a line wider than its box with U+2026 at the end, the start or the middle. `truncate` is `overflow_hidden`, `nowrap` and `ellipsis`. `text_left`, `text_center` and `text_right` align lines in the box. Each is inherited with the text style.
- `text_color`, `text_size`, `font_family`, `font_weight`, `font_style` and `line_height` are inherited by an element's children. A text run is black at one rem in the default family at weight 400 and upright until they are set. `font_style(e, uitext.FontStyle)` takes `Normal`, `Oblique` or `Italic`. `italic(e)` is `font_style` with `Italic`. `text_decoration(e, uitext.Decoration)` takes `None`, `Underline` or `Strikethrough`. It is inherited with the text style. `underline(e)` and `strikethrough(e)` are `text_decoration` with those. A text draws one `scene.Underline` per line in its colour. The underline runs from the line's first glyph to its last that is not a space. It is at least one device pixel thick and is positioned from the font's metrics (see Text). A text's line height is the size times 1.618034 until `line_height` sets it. Either is rounded to a whole device pixel. A text run's width is rounded up to a whole logical pixel.
- A `Signal[T]` is a value that belongs to the application, not to one window. `signal(cx, v)` makes one with any window's context. `get(s)` reads it. `set(s, v)` and `update(s, f)` change it, run the effects that read it and make a frame due in every window of the application whose frames are not tracked. Copies of a signal share the value. The tree is a function of its signals. A program therefore holds its state in signals and returns a new tree from the function it gives `run`.
- Reads are tracked on the application's green thread through the running observer, which is an effect, a memo or a frame. `get(s)` and `read(m)` inside one make it depend on what they read. `untracked(cx, f)` reads without that. `memo(cx, f) -> Memo[T]`, for a `T` that is `Eq`, computes `f` at once and keeps the result. `read(m)` recomputes it first when a signal or memo it read has changed. A memo that computes a value equal to the one it holds changes nothing downstream, so the effects, memos and tracked frames that read it do not run again. `effect(cx, f) -> Effect` runs `f` at once and again each time something it read changes. A change marks the memos below it to be checked and queues the effects. An effect runs after every memo it reads has been brought up to date. Queued effects run outer before inner and otherwise in the order they were made. So each runs once per change with consistent values, as in a diamond where one signal reaches an effect through two memos. A `set` inside an effect, and every `set` inside `batch(cx, f)`, queues effects that run once the outermost effect or batch returns. An effect or memo belongs to the effect, memo or frame that was running when it was made, or else to the window of `cx`. It ends when its owner runs again or ends, when its window closes, or when `dispose(e)` ends an effect. `on_cleanup(cx, f)` runs `f` before the running effect or memo runs again or ends, or when the window closes outside any. An ended memo keeps its last value.
- `set_tracked_frames(cx, true)` makes a frame of the window of `cx` due only when a signal or memo the last frame read has changed, a memo cutting off an equal value, or the window's own state changes: its size, the pointer, the focus, a scroll, a timer, an animation, or a call that changes every window, such as `set_menu_style` or `set_text_smoothing`. Frames are untracked by default, because a tree may read state held outside signals, such as a struct a handler changes and a posted closure fills, and only an untracked frame is drawn again for every signal change and drained post.
- A `Context` is the user interface of one window. It holds what the pointer and the keyboard are on, and its frames. The signals, the text system, the keymap, the clipboard, the posted closures and the clock belong to the application's shared state, which every window's context reaches. A signal change or a drained post makes each untracked window due by advancing a generation count. A change that every window draws, such as `set_menu_style`, advances an epoch that makes tracked windows due as well. It is a handle: copies share the state. `headless(background)` makes one with no window and its own shared state. `frame(cx, render, size, scale)` returns the scene of the tree that `render` returns. `dispatch(cx, event)` applies a `platform.Event`. So a program or a test drives an interface with no window and no GPU. `draw(cx, g, t, scene)` draws that scene into an offscreen target. `window_size(cx)` returns the client size in logical pixels that the frame being built is laid out in. `scale(cx)` returns the device pixels per logical pixel of the last `Resized` or `frame`, 1 before either, so a program can make an image at the device-pixel size it is drawn at from the first tree `run` builds. `frame_due` reports whether a frame is due. `closed(cx)` reports whether the window has closed or `quit` was called. `quit(cx)` ends `run` whichever window it is called with.
- `open_window(cx, o, render) -> Window ! Error` opens a window beside the application's others. `close_window(w)`, `is_open(w)`, `window_context(w)` and `main_window(cx)` close it, report it, return its context and return the first window. `activate_window(w)` brings a window to the front through `platform.activate`. On Windows that call restores a minimised window with `ShowWindow(SW_RESTORE)` and then calls `SetForegroundWindow`. A headless or closed window has no system window, so it does nothing. `WindowOptions.owner` names the window a new one belongs to. `closed` runs with a `CloseReason` once the window has closed. `close_window` closes the windows a window owns first. Only a context from `headless` opens windows, each with its own state and the application's shared state, driven with `frame` and `dispatch` on `window_context(w)`; a context from `open` and a call made while a frame is being built fail with `Platform(Unsupported(detail))`.
- `set_title(cx, title)` changes the title of the window of a context through `platform.set_title_on`. `window_title(cx)` returns it, or `Options.title` or `WindowOptions.title` until then. A headless or closed window has no system window, so only `window_title` changes. `native_window(cx) -> platform.Native?` returns the window's `platform.native_on` handles, for a call that needs an owner handle such as a file dialog. It returns `none` for a headless or closed window. `window_focused(cx)` is `true` between a `FocusChanged(true)` event and the next `FocusChanged(false)`. `on_focus_changed(cx, f)` adds a handler that runs on the context's green thread with the new state each time it changes. A frame is due after that. A window's icon is `Options.icon`, `WindowOptions.icon` and `set_window_icon`. `set_icon(cx, frames) ! Error` is `set_window_icon` for the window of a context, doing nothing once it has closed.
- `open(Options) -> App ! Error` opens a window, a `Gpu` that draws into it and a `Context` whose waker is the window's. `run(app, render) ! Error` loops until the window is closed. It waits for events and dispatches them. When a frame is due, it builds the tree, lays it out, paints it and presents it. `open` opens the window with `platform.Options.hidden`, so it is invisible while the GPU and the fonts load. `run` calls `platform.show` once, right after the first `present` returns without error, whether or not that call had anything to present. A window with no frame to draw, because its content area is empty, is shown 500 milliseconds after `run` starts. The loop waits at most that long. A first `present` that fails ends `run` with the error, and the window stays hidden. `app.cx` is the context, so `app.cx.signal(0)` makes a signal before `run`. `Options` has a title, a size, a background, a `border` colour and a `titlebar`. The `appears_transparent` field of `titlebar` hides the system title bar. `border` is a `geometry.Color?`. `open` converts it with `geometry.rgb_of`, which ignores alpha, to the `0xRRGGBB` of `platform.Options.border`. `WindowOptions.border` is the same for `open_window`, while a headless context ignores it. `Options.icon` and `WindowOptions.icon` are `IconFrame[]?` of `std/ui`, the same `size` and `png` as `platform.IconFrame`, that `open` and `open_window` convert and give the platform. `set_window_icon(w, frames) ! Error` changes a window's icon through `platform.set_icon_on`. It fails with `Platform(BadIcon(detail))` for a frame that is not a PNG of its `size`. It does nothing for a headless or closed window. `std/ui` draws no caption buttons itself, so a program that hides it draws its own title bar and its minimise, maximise and close buttons from elements marked with `window_control_area`. `open` gives the platform a hit test that answers from the areas of the last frame. On Windows `open` switches the context's text to the installed fonts. They are drawn as subpixel text when the GPU can blend it and the system uses ClearType. A headless context keeps the built-in font. `set_text_smoothing(cx, mode)` changes that choice afterwards. `TextSmoothing.System` is the choice `open` made. `Subpixel` is subpixel when the GPU can blend it. `Greyscale` is one coverage value per pixel. The call clears the cached glyphs through `uitext.set_subpixel` and makes every window due for a frame. A headless context draws built-in glyphs whatever the mode.
- `elements(cx) -> ElementInfo[]` lists what the last frame drew, in paint order, for a script, a test or an assistive-technology bridge. It lists every element with an id, a handler, a `label` or a `role`, and every text run, field, `text_area`, picture, icon, `canvas` and list, including the rows of a list in view. An `ElementInfo` carries the `key`, the `id`, the `label`, the `role`, the `text`, the `bounds`, `visible` and `focused`. The `text` is what a text run or a field shows. It is empty for a `masked` field and every other element, so a script reads an element's text without learning a password. The `bounds` are the border box, unclipped, as `bounds_of` returns. `label(e, name)` and `role(e, r)` set the name and the `Role` (`Generic`, `Button`, `Link`, `Heading`, `Text`, `TextInput`, `Image`, `Checkbox`, `List`, `ListItem`, `Tab`, `Menu`, `MenuItem`, `Dialog`). They make `e` recorded even with no id or handler. Without them the label is the text run's text, or for an element with a click handler or focus the text runs inside it joined by spaces, and the role follows the element's kind, `Image` for a picture, an icon or a `canvas`, `Button` for a click handler and else `Generic`. `visible` is `false` for an element that is `invisible` or inside one. It is `false` for one clipped away entirely, and for one under an element painted after it that takes the pointer at the centre of its visible part and is not inside it, as an overlay is. There is no `enabled`, because no element has a disabled state.
- `on_frame(cx, f)` adds a handler that runs on the context's green thread after each frame, with that frame's `elements` and `hitboxes` in place. `enable_accessibility(cx, application) -> Accessibility ! Error` uses it to publish the frame to the AT-SPI bus, as `### Accessibility` describes. The returned `Accessibility` withdraws the application when released.
- `element_at(cx, point) -> str?` returns the key of the topmost element that takes the pointer at `point`, the one a click there reaches. That is an element with a handler, focus or scrolling, or a list. A text run therefore answers with the element that holds it. `click_element(cx, key) -> bool` moves the pointer to the centre of the visible part of that element and presses and releases the left button there through `dispatch`, so what is on top gets the click. It returns `false`, sending nothing, when the last frame drew no such element or none of it is in view. A script on another green thread runs these through `post`.
- `serve_debug(cx, DebugOptions) -> DebugServer ! io.Error` lets a script in another process reach a running window's elements. It is off until an application calls it. A `DebugServer` stops listening, removes the socket file and closes its connections when it is released. On Linux the transport is a Unix socket, made with mode 0600 at `DebugOptions.path` or `debug_path(pid)` (`$XDG_RUNTIME_DIR/low-ui-<pid>.sock`, else `/tmp/low-ui-<uid>-<pid>.sock`). On Windows it is a named pipe (`\\.\pipe\low-ui-<pid>`) made with `sys.listen_pipe`. The server drops a Linux peer whose `peer_credentials` user id is not the server's before it reads a byte. On Windows the pipe's access list names only the current user and refuses remote clients. A socket file that no process answers on is replaced. One that answers fails with `AddrInUse`. The wire format is one JSON object per line over `std/json`. A request `{"id": 1, "method": "elements", "params": {}}` is answered by `{"id": 1, "result": ...}` or `{"id": 1, "error": "..."}`. A line may be up to 8 MiB. A line that is not JSON, has no method or names an unknown method is answered with an error, and the connection stays open. The methods are `elements`, `element_at`, `click`, `type` and `text`. `elements` answers with an array of `key`, `id`, `label`, `role`, `text`, `bounds` with `x`, `y`, `width` and `height`, `visible` and `focused`. `element_at` takes `x` and `y` and answers `{"key": ...}` or `null`. `click` takes `key` and answers whether `click_element` sent the click. `type` takes `text` and an optional `key` that is focused first when the frame drew it. It answers whether an element was focused to receive a `Commit` event. `text` takes `key` and answers the element's `ElementInfo.text`, so a `masked` field answers an empty string. Each request runs through `post` on the context's green thread. Its reply is computed after the frame that follows the post, with that frame's elements in place, so a request sees what the requests before it did. A request whose context draws no frame within `DebugOptions.timeout_ms` (10 seconds) is answered with an error. `debug_connect(path) -> DebugClient ! DebugError` is the matching client. `debug_elements`, `debug_element_at`, `debug_click`, `debug_type`, `debug_type_into` and `debug_text` send one request each and take one at a time. A failed connection is `DebugError.Io`, an error answer is `Remote` and a reply outside the protocol is `Protocol`. `std/ui/debug_server_test.low` serves a headless context. It drives the context from a client on another green thread over a real socket or pipe, pumping the context with `Woken` events and `frame` as `run` does. The Windows pipe is compiled but not run by any test.
- `handle(cx)` returns a copyable `Handle`. `post(h, f)` queues a closure that runs on the context's green thread before the next frame and wakes the window. Another green thread therefore changes signals through it.
- `background_spawn(cx, work, done) -> Task` runs `work` on a new green thread and posts `done` with its result to the context, as gpui's `background_spawn` hands its result back to the foreground. `spawn_into(s, work) -> Task` does the same and sets the signal `s` to the result. A `Task` is a handle read and changed only on the context's green thread. `cancel(t)` drops the result when it arrives. `finished(t)` is `true` once the result has arrived, whether or not it was dropped. The work itself always runs to its end.
- A frame runs in order: the posted closures, `render`, building a layout tree from the elements (a text run is a leaf whose measure wraps the text), `layout.compute`, recording the hitboxes, hit testing, then painting. Painting pushes, per element in tree order, its shadows, its quad and its text sprites, so a child draws over its parent. An element with `display: None` and its children take no space and draw nothing.
- Painting turns logical rectangles into device pixels with `geometry.snap`, `geometry.cover` and `geometry.snap_stroke`. A quad has each edge rounded to the nearest device pixel, a tie going toward zero. Each border width is rounded the same way but never under one device pixel unless it is zero. Corner radii are limited to half the shorter side of the border box in logical pixels, as `Corners::clamp_radii_for_quad_size` does (`geometry.rs:2479`, called by `Style::paint` at `style.rs:707-710`), and then scaled. An element paints as `Style::paint` does. It paints its shadows, then its background in a quad with no border, then its children, then its border in a quad with a transparent background. So the border covers whatever a child draws under it. A shadow takes the smallest device-pixel rectangle that holds its box moved by its offset and grown by its spread, keeps the element's corner radii, and reads `blur` as the standard deviation of the Gaussian. A clip is the smallest device-pixel rectangle that holds its box.

- A rem is 16 logical pixels until `set_rem_size(cx, size)` changes it. `rems(cx, count)` is `count * rem_size(cx)`, which the builders then take as logical pixels. So a 13 pixel rem makes `rems(cx, 0.5)` 6.5 pixels, and layout rounds that to device pixels like any authored length.

Hit testing and focus:

- An element has a hitbox when it has a click, key, mouse or scroll handler, a `hover`, `active`, `group_hover`, `group_active` or `focus` style, a `cursor`, a `group` name, a `window_control_area`, a tooltip, menu or popover, is `focusable`, or is made with `occlude` or `block_mouse_except_scroll`. A frame records one hitbox per such element, in tree order, as the rectangle of its border box in logical pixels. It keeps for each the hitbox of the nearest ancestor that has one.
- An element's key is its `id` when it has one, else its position in the tree, such as `"0/2/1"`. Focus and the press a click needs are kept by key from one frame to the next. So an element keeps focus when the tree moves it only if it has an id.
- The pointer is over every hitbox that contains it, so the elements of a nested path are all hovered, until an `occlude`d hitbox. The hitboxes are walked from the last recorded to the first. One made with `occlude` stops the walk after itself, so nothing under it is hovered, pressed, clicked or scrolled. One made with `block_mouse_except_scroll` stops hover and clicks the same way but lets the wheel reach what is under it, as gpui's `hit_test` (`window.rs:775-795`) and `should_handle_scroll` (`window.rs:514`). `on_mouse_down_out` asks whether the pointer is inside an element's rectangle, as gpui's `contains`. `on_mouse_up_out` asks whether the element is hovered. `hover(e, f)` restyles `e` with `f` while the pointer is over it or over an element inside it. `active(e, f)` does so from a press of the primary button over it until the release. `focus(e, f)` does so while it is focused. The conditions apply in the order the builders were called. A change of what is hovered or focused makes a frame due.
- `group(e, name)` names `e` a group. A group always has a hitbox (`div.rs:1697`). `group_hover(e, name, f)` applies `f` to `e` while the pointer is over the hitbox of the innermost group called `name` that holds `e`. That means over the group's clipped bounds and not only over `e`. `group_active(e, name, f)` applies it from a press of the primary button over that group until the release (`div.rs:2330-2348`, `2497-2504`, `2548-2552`). Groups are looked up from the elements inside them, nearest first, as gpui's `GroupHitboxes` stack does (`div.rs:2876-2900`). A group is pushed after its own style is computed, so an element does not find its own group. A name with no enclosing group never applies. A frame resolves each name to a hitbox during the walk that records the hitboxes. A group under an `occlude`d hitbox is not hovered, so its `group_hover` styles do not apply there. The group conditions apply in builder order with `hover`, `active` and `focus`. A change of which groups are hovered makes a frame due like any other hover.
- `invisible(e)` and `visible(e)` set `Visual.visible`. `with_visible(v, b)` sets it in a style function, and `with_opacity(v, f)` sets the opacity there. A hidden element keeps its place in the layout and its hitbox, and paints nothing: as `Interactivity::paint` returns before painting the element, its children, its listeners and its cursor when `style.visibility` is `Hidden` (`div.rs:1809-1811`, set by `visibility_style_methods!` at `styled.rs:26`, `style.rs:296-298`), so neither it nor an element inside it draws, runs a click, key or wheel handler, shows a cursor or takes focus, and its hitboxes take no part in hit testing once the frame that hides it is painted, while the styles of that frame still see the hover the hitboxes had before it. The visibility is the one the conditions leave, so `group_hover` can make a hidden element visible, as a row's delete button that appears while the pointer is over the row.
- `on_click(e, f)` runs `f` with a `ClickEvent` when the primary button is pressed and released over `e`. A click goes to the innermost element with a click handler that both the press and the release were over, and to no other. A press elsewhere, a release elsewhere and any other button give no click, as gpui's `Interactivity` records a pending press only for `MouseButton::Left` (`div.rs:2133-2144`). The event carries the release's position, modifiers and click count. So a double click runs the handler twice, the second time with `click_count` two. A handler tells a single from a double or triple click by the count, as gpui's `ClickEvent::click_count` does. A click from Enter or Space has `keyboard` set, a count of one and the bottom left corner of the element as its position, as gpui's `KeyboardClickEvent` (`interactive.rs:180-194`).
- `on_mouse_down(e, button, f)`, `on_any_mouse_down(e, f)` and `on_mouse_up(e, button, f)` run `f` with a `MouseDownEvent` or `MouseUpEvent` (button, position, modifiers, click count) for a press or release of that button, or any button, over `e`. `on_mouse_move(e, f)` runs `f` with a `MouseMoveEvent` (position, the last pressed button still held, and the modifiers of the last button or key event) for each move over `e`. A press or release that also moves the pointer gives no move. Each goes to the elements under the pointer from the innermost outward, every one of them, until a handler calls `stop_propagation(cx)`, as gpui's bubble phase does. `on_mouse_down_out(e, f)` and `on_mouse_up_out(e, button, f)` run for a press or release that is not over `e`, before the handlers of the elements under the pointer, as gpui's capture-phase `on_mouse_down_out` does. A menu closes on a click elsewhere through them. A press of the primary button focuses before its handlers run, so a handler that moves focus keeps it.
- `on_drag(e, kind, value, preview)` starts a drag when the primary button is pressed over `e` and the pointer then moves more than two logical pixels from the press, as gpui's `DRAG_THRESHOLD` and the check in `div.rs:2159` (`elements/div.rs:47`). The press that began it gives no click on `e` or on any element around it. The innermost element under the press with an `on_drag` starts it. The drag carries a `DragPayload` of `kind` and `value`. `preview(value)` builds the element drawn over everything else. Its top left is at the pointer minus the distance the pointer had from the top left of `e` at the move that crossed the threshold, as `div.rs:2163` takes the offset there and not at the press, and `window.rs:2055` draws the preview where the tooltip would be. An element has one `on_drag`; with several the first applies. While the drag is active the pointer shows the `cursor` of `e`, or the arrow when `e` sets none, whatever is under it (`div.rs:2082`, `2538`). A preview is laid out with no limit on its width or height, where gpui uses `AvailableSpace::min_size()` (`window.rs:2058`). `layout.compute` has no minimum-content measure for a root, so its text does not wrap. A preview is drawn with its ids, handlers, hover, active and focus styles, scrolling and fields stripped, so it takes no part in hit testing or focus and records no hitbox.
- gpui picks the handlers a drag reaches by the Rust type of its value. Low has no runtime type identity. So a drag carries a `kind` string the application chooses and a `value` string. `on_drag_move`, `on_drop` and `drag_over` take the `kind` they accept. An application keeps larger values in its own state and drags a key to them. The drag of files from another program has the kind `external_paths` and its `paths`, as gpui's `ExternalPaths`.
- `on_drag_move(e, kind, f)` runs `f` with a `DragMoveEvent` (position, the border box of `e`, modifiers, payload) for every move of the pointer while a drag of `kind` is active, wherever the pointer is, as gpui's `on_drag_move`. The move that starts a drag does not run it. A handler that calls `stop_propagation(cx)` stops the move reaching the others. `on_drop(e, kind, f)` runs `f` with a `DropEvent` (position, modifiers, payload) when a drag of `kind` is released over `e`, whether or not it started there. The release is offered to every element the pointer is over, from the last recorded to the first, as each drop listener runs in bubble order when its hitbox is hovered (`div.rs:2095`, `window.rs:3705`). So an element under a sibling that overlaps it without occluding it still takes it, as `drag_over` decides it does. The first with an `on_drop` of that kind takes the drag before `can_drop` is asked, as `div.rs:2100-2113` does. So a refusal runs nothing and offers it to no other element. An element that occludes `e` takes the pointer away from it. The drop is registered after the element's `on_mouse_up` (`div.rs:2089`, `2047`). So in bubble order it runs before the element's own mouse up handlers. An accepted one stops propagation, which skips them and those of every element further out. `can_drop(e, f)` refuses the payloads `f` returns `false` for, for `on_drop` and for `drag_over`. An element has one predicate, and with several the last applies, as each gpui call replaces the one before. `drag_over(e, kind, f)` restyles `e` with `f` while a drag of `kind` that `can_drop` accepts is over it or over an element inside it.
- While a drag is active no element is hovered or shown as active. None is clicked and no tooltip shows or is started, as `div.rs:2498` suppresses hover while dragging and `window.rs:2055` prepares the preview where it would prepare the tooltip, so `std/ui` hides the tooltip during a drag. Any release of any button ends the drag (`window.rs:3716-3724`), whether or not an element took it. Escape does not, as in gpui. `dragging(cx)` reports whether one is active. `drag_payload(cx)` returns what it carries. `cancel_drag(cx)` ends it without a drop, as `stop_active_drag` (`app.rs:1949`). gpui's `group_drag_over` has no counterpart because `std/ui` has no groups.
- A drag of files from another program arrives as the platform events `FileDragEntered(x, y, paths)`, `FileDragMoved(x, y)`, `FileDropped(x, y)` and `FileDragExited`, which `dispatch` turns into the mouse events gpui turns them into (`window.rs:3620-3660`). Entering makes a drag of kind `external_paths` and a move of the primary button held. A move moves the pointer. Dropping is a release of the primary button with a click count of one. Leaving ends the drag with no drop, takes the pointer out of the window and refreshes hover and tooltips as `PointerLeft` does. A `FileDragMoved` to the position the pointer already has gives no move, as `PointerMoved` does not. Such a drag has no preview, because the system draws one.
- `masked(e)` makes a field draw a bullet (U+2022) for each grapheme of the text it shows, preedit included, while the text stays the field's value. A masked field refuses copy and cut, which are consumed. It treats the whole text as one word, so Control with an arrow key or Backspace moves or deletes one grapheme, and a double click selects all of it. Selection, deletion, paste and the input method work as in any field.
- A field selects the word under the pointer on a press with a click count of two, by `unicode.word_end` segments, and the whole text on a count of three; a drag after either does not extend it.
- A press focuses the innermost `focusable` element under the pointer, or its nearest focusable ancestor. A press on none removes focus. `set_focus(cx, key)`, `blur(cx)` and `focused(cx)` change and read it. Tab and Shift+Tab move focus to the next and previous `focusable` element in tree order, wrapping at the ends, or to the first and last when none is focused.
- `on_key(e, f)` runs `f` for each key press, repeat and release while `e` or an element inside it is focused. A press reaches it only when no key binding took the press. A key goes to the focused element and then outward until a handler returns `true`. Tab and Enter and Space act only when no handler took the key. Enter and Space click the focused element when it has a click handler.
- `dispatch` tests against the hitboxes of the last frame, so the pointer is compared with what was drawn.

Title bar:

- When `platform.decoration_height_on` of a window's handle is above zero, `frame` places the tree `render` returns under a title bar of that height. The bar shows the window's title, truncated, and caption buttons for `Min`, `Max` and `Close`. Each button is marked with its `WindowControlArea`, and the whole bar is a `Drag` area. The tree sits in the rest of the window. The window's size includes the bar, so `Options.height` is still the height of the tree. A window with no bar, a headless context and a window with `appears_transparent` draw none.
- `window_control_area(e, area)` marks `e` with a `platform.WindowControlArea`, as gpui's `window_control_area` does (`div.rs:581`, `1004`) and records its hitbox as `insert_window_control_hitbox` does (`window.rs:3324`, called from `div.rs:1849`). `window_control_area_at(cx, x, y) -> platform.WindowControlArea?` answers from the hitboxes of the last frame, by a rule of `std/ui`'s own. It starts at the last hitbox in tree order that contains the point and walks outward through its ancestors. The first element marked with an area answers with it. An element that has a handler, a text field or is `focusable` answers `none` when it is met first. So a menu button on a `Drag` bar is a button and not part of the caption, and the window answers the system `HTCLIENT` there so the button gets its clicks as ordinary pointer events. gpui answers from the first window-control hitbox, in the order they were recorded, whose id lies among the hitboxes under the last mouse move (`window.rs:1133-1146`). gpui delivers a click on a button over a `Drag` bar through non-client mouse dispatch (`events.rs:949-990`). It gives `none` where no marked element lies, and before the first frame. Areas clip like hitboxes do. `open` makes it the platform's `set_hit_test`.
- A `Close`, `Max` or `Min` area acts on a press and release by the system, not through `on_click`. A program that wants a click handler on one adds the handler and the area to different elements.
- `is_maximized(cx)` follows the platform's `Maximized` events. `dispatch` of one makes a frame due only when the state changed, so a title bar can restyle for the maximised window. `minimize(cx)` and `toggle_maximize(cx)` run the window's commands. A headless context has no window, so `toggle_maximize` flips the state it keeps and makes a frame due, and `minimize` does nothing.

Key bindings and actions:

- An action is a name, such as `"editor.save"`. `on_action(e, name, f)` runs `f` when the action is dispatched while `e` or an element inside it is focused. `capture_action(e, name, f)` runs `f` before any `on_action` handler, as gpui's `on_action` and `capture_action` (`elements/div.rs:842-866`). The dispatch path is the focused element and the elements around it, or the root element when none is focused, as gpui's `focus_node_id_in_rendered_frame` falls back to the root (`window.rs:3982-3990`). Capture handlers run from the outermost element inward and let the action continue unless one calls `stop_propagation`. `on_action` handlers then run from the focused element outward. The first stops the action unless it calls `propagate(cx)`, as gpui's `dispatch_action_on_node` sets `propagate_event` to false before each bubble handler (`window.rs:3992-4080`). `dispatch_action(cx, name)` dispatches an action from code and returns whether a handler stopped it.
- `key_binding(keystrokes, action, context)` reads a binding as gpui's `KeyBinding::load`. Keystrokes are separated by spaces. Each is modifiers and a key joined by `-` (`ctrl`, `alt`, `shift`, `super`, also written `cmd` or `win`, and `secondary`, which is `ctrl`). A single upper case letter means that letter with `shift`, and `ctrl--` means Control with the minus key, as `Keystroke::parse` (`platform/keystroke.rs:120-200`). The context is a predicate over key contexts. It joins identifiers and `key == value` tests with `&&`, `||`, `!`, `!=`, `>` and parentheses. It has gpui's precedences and its meanings for `!` (no context on the path matches) and `>` (a context inside another), as `KeyBindingContextPredicate` (`keymap/context.rs:180-470`). Text that cannot be read fails with `KeymapError`. `bind_keys(cx, bindings)` adds bindings and `clear_key_bindings(cx)` removes them all. An empty action name unbinds its keystrokes where its predicate holds, as gpui's `NoAction`.
- `key_context(e, context)` adds identifiers and `key = value` pairs, such as `"Editor mode = full"`, to the contexts of the path, as gpui's `key_context` (`elements/div.rs:658`). An element with a key context or an action handler takes part in dispatch.
- A key press goes to the keymap before any `on_key` handler or text field. The bindings whose predicate matches the path's contexts and whose keystrokes match the presses so far are ordered by how deep in the path their predicate matched and then by which was added later, and a binding that is a prefix of a longer one makes the presses wait, as gpui's `Keymap::bindings_for_input` (`keymap.rs:142-206`). A matched binding dispatches its action. When no handler stops it, the next binding is tried, and then the press reaches `on_key` handlers and the focused field as if it were unbound. A chord such as `ctrl-k ctrl-o` holds its first keystroke and types nothing until the second arrives. A press that ends no binding replays the longest bound prefix and then the rest as ordinary presses, a chord left waiting one second replays its keystrokes the same way, and moving focus drops a waiting chord, as gpui's `dispatch_key`, `replay_prefix` and `flush_dispatch` (`key_dispatch.rs:465-548`, `window.rs:3800-3830`). Releases never match. `pending_keystrokes(cx)` returns the keystrokes of a waiting chord.
- A key matches a keystroke by the text it produces. A letter typed with Shift matches `shift-k`. A character that needs Shift, such as `!`, matches a binding that names the character with or without `shift`, as gpui's `should_match` on Windows (`platform/keystroke.rs:82-115`). The keyboard reports the produced character, so `shift-1` matches nothing. A binding names `!`. Space is `space`. The named keys are `enter`, `escape`, `tab`, `backspace`, `delete`, `left`, `right`, `up`, `down`, `home`, `end`, `pageup`, `pagedown`, `insert`, `pause`, `break`, `menu` and `f1` to `f12`. So Control with Pause matches `ctrl-break` and the context-menu key matches `menu`.
- `bindings_for_action(cx, name)` returns the bindings that would dispatch `name` from the focused element, leaving out those another binding of the same keystrokes takes over there. `bindings_for_action_in(cx, name, key)` does so for any element, as gpui's `bindings_for_action_in` lets a menu show the shortcuts of what was focused before it opened. `keystroke_text_for(cx, name)` returns the keystrokes of the last of them, such as `ctrl-k ctrl-o`, or the action's name when none. `keystroke_text` and `binding_text` write keystrokes in gpui's modifier order, `ctrl`, `alt`, `super`, `shift`. `is_action_available(cx, name)` and `available_actions(cx)` report the actions the focused path handles.
- A context has a clock in milliseconds that timers such as the chord timeout and the tooltip delay run on. `run` reads the monotonic clock and waits for events at most until the next timer. A context from `headless` starts at zero and moves only when `advance_clock(cx, ms)` moves it. That also runs the timers of every open window that fall due.
- `now(cx)` returns that clock, held at the time the frame started while a frame is being built. `after(cx, ms, f)` runs `f` once on the context's green thread `ms` milliseconds from now. `every(cx, ms, f)` runs it every `ms` milliseconds, at least one. Each returns an id for `cancel_timer(cx, id)`, which `f` may call on its own id. The ticks of `every` keep their phase. A clock that moves past several at once runs `f` once and waits for the tick after it. A timer that ran makes a frame due. `animate(cx, f) -> i64` calls `f(now)` at the start of each frame, before `render`, until it returns `false`. It keeps a frame due while any animation runs, so a value computed from `now` follows the clock whatever else changes. It returns an id for `cancel_timer`. All of them run on the clock `advance_clock` moves, so a test steps an animation with `advance_clock` and `frame`.
- `prefers_reduced_motion(cx)` returns whether the system asks for no animation, read from `platform.prefers_reduced_motion()` at most once a second. On Windows it reads `SPI_GETCLIENTAREAANIMATION`. On Linux it reads the GNOME `enable-animations` setting with `portal.animations_enabled` from `std/portal` on the session bus, with a 200 millisecond limit, which is `false` when there is no portal or it fails. A context from `headless` reads `false`. `set_reduced_motion(cx, on)` makes the answer `on` in every window of the application. `none` hands it back to the system.

Overlays, tooltips, menus and dialogs:

- `deferred(e)` draws `e` after the rest of the tree, outside every clip of its ancestors, and records its hitboxes after theirs, so it is on top and takes the pointer first, as gpui's `deferred` and `defer_draw` (`elements/deferred.rs:7-60`, `window.rs:2157`, `2756`). It is laid out in its place in the tree and keeps its ancestors for hover, actions and focus. `priority(e, n)` orders deferred elements. A higher priority draws later. Equal priorities draw in tree order. A deferred element inside another draws after it, as `with_priority`.
- `anchored()` is an absolutely positioned box that places its children at a point and keeps them inside the window, as gpui's `anchored` (`elements/anchored.rs:122-230`). `anchor(e, corner)` names the `Corner` of the box that goes at the point, top left by default. `position(e, p)` sets the point. The point is in window coordinates in the default `PositionMode.Window` and measured from the box's own place in `Local`. Without a position the point is the box's own place. `attach_to(e, key, corner)` takes the point from a corner of the element with that key, in the frame being built or else the last one. `offset(e, p)` moves the box after the point is chosen. The default `FitMode.SwitchAnchor` moves a box that would cross the window's right or bottom edge to the other side of the point on that axis when that fits, then pushes it back inside. `snap_to_window` only pushes it back. `snap_to_window_with_margin(e, edges)` pushes it back to the margin from the edge it crossed. The flip is measured from the point without the offset. The final shift is rounded to whole logical pixels, as gpui does. `bounds_of(cx, key)` returns the rectangle an element with that key was laid out at in the last frame, for any element with an id or a hitbox.
- `occlude(e)` and `block_mouse_except_scroll(e)` give `e` a hitbox that blocks the pointer, as above.
- `tooltip(e, build)` shows the element `build` returns 500 milliseconds after the pointer comes to rest over `e`, as gpui's `TOOLTIP_SHOW_DELAY` (`elements/div.rs:48`, `2650-2860`). It is placed one pixel right of and below the pointer where the pointer was when it appeared. One that would cross the right or bottom edge goes to the other side of the pointer, or to zero when that side has no room, as gpui's `prepaint_tooltip` (`window.rs:2088-2156`). It is drawn over everything else and hides when the pointer leaves `e`, a button is pressed or the wheel turns. `hoverable_tooltip(e, build)` keeps it through presses and wheels and while the pointer is over it. It hides the tooltip 500 milliseconds after the pointer has left both `e` and the tooltip, as `HOVERABLE_TOOLTIP_HIDE_DELAY`. A tooltip whose owner leaves the tree hides.
- `context_menu(e, id, items)` opens a menu at the pointer when the secondary button is pressed over `e` or an element inside it, kept 8 pixels inside the window. `dropdown_menu(e, id, items)` opens one under `e`, its left edge on the left edge of `e`, when `e` is clicked. It closes the menu when `e` is clicked again. With no room below it opens above `e`, and with no room on the right it ends at the right edge of `e`. `items` is called each frame the menu is open and returns `MenuItem`s. `menu_entry(label, f)` runs `f`. `menu_action(label, action)` dispatches `action` to the element focused before the menu opened and shows its key binding there, as gpui's `bindings_for_action_in`. `menu_separator()` draws a line. `submenu(label, items)` opens a menu to the right of its row, or to the left of it when there is no room on the right. `disabled(item)` greys an item out and skips it. An action no element on that path handles is disabled. A menu takes focus while it is open. Up and Down move the highlight past separators and disabled items, wrapping. Home and End go to the ends. Enter and Space choose. Right opens a submenu and Left closes it. Escape closes the menu ahead of any key binding. Moving the pointer over an item highlights it and over a submenu's row opens it. Choosing closes the menu and gives focus back before the item runs.
- `popover(e, id, content)` opens the element `content` returns where a dropdown menu would open when `e` is clicked. A second click closes it. A menu or popover closes on a press outside it and its owner. That press then reaches what is under the pointer. Escape closes the topmost, after the focused element and its parents inside the layer have had it through their `on_key` handlers. So an element inside a dialog or popover that handles Escape keeps the layer open. A menu or popover whose owner leaves the tree closes.
- `open_dialog(cx, id, content)` shows `content` as given, centred on a translucent black backdrop. The backdrop covers the window and occludes it. `open_dialog` moves focus to the first focusable element inside. While a dialog is open, Tab, Shift+Tab and presses move focus only among its elements. While a dialog is open, a press outside its content does nothing. While a dialog is open, Escape closes the dialog and gives focus back to the element focused before it opened, unless the focused element or one of its parents handles Escape with `on_key`. Menus and popovers opened inside a dialog work as elsewhere.
- `open_dialog_with(cx, id, backdrop, content, closed)` opens a dialog as `open_dialog` does, with `backdrop` as the colour over the window. `closed` runs once after the dialog closes by any route: Escape, `close_overlay` on it or on a layer under it, or a layer under it whose owner left the tree. Focus has been given back by then. `closed` runs as a closure posted with `post` does, before the next frame, never inside layout or prepaint. Opening an id that is open closes the first dialog and notifies it.
- `close_overlay(cx, id)` closes a menu, popover or dialog and those opened after it. `overlay_open(cx, id)` reports whether it is open. Menus, popovers, dialogs and tooltips are laid out each frame in trees of their own after the deferred elements, menus and popovers at their natural size and a dialog against the window, and drawn after them in the order they opened. A popover or dialog draws only the element its `content` returns, so its background, border, corners, padding and shadow are the application's. The backdrop of a dialog is the one thing `std/ui` colours. Menus draw the application's `MenuStyle`. `set_menu_style(cx, style)` sets, for every window, the panel's `background`, `border`, `radius` and `shadow` and the `text`, `disabled`, `hint`, `highlight` and `separator` colours from the next frame on. `MenuStyle {}` is the light panel menus draw until then.

Scrolling and clipping:

- `overflow_hidden`, `overflow_x_hidden` and `overflow_y_hidden` clip an element's children to its padding box on the axes named. `overflow_scroll`, `overflow_x_scroll` and `overflow_y_scroll` clip them and scroll them with the wheel. A scrolling element has a hitbox and so a key; give it an `id` to keep its offset when the tree moves it.
- Children that would not fit a scrolling element's main axis give up space like any flex item, down to their automatic minimum. A child that must keep its size in a scrolling column or row therefore takes `shrink(0.0)`.
- The scroll range of an axis is how far the children's far edge lies beyond the padding box's inner edge, never below zero. The offset is kept per key. It shifts the children and their hitboxes, and each frame clamps it to the range. `scroll_to(cx, key, point)` sets it and `scroll_offset(cx, key)` reads it.
- `scroll_metrics(cx, key) -> ScrollMetrics?` returns what the last frame recorded for the scrolling element under `key`. `viewport` is the size of its box inside border and padding. `content` is the viewport grown by the scroll range. So `content` less `viewport` is the farthest `scroll_offset` on each axis, and an axis with nothing to scroll has equal sizes. A scrolling element, a `uniform_list` and a `list` record it while they are prepainted. A `text_area` records it while it is painted. A tree being built reads the frame before. A `text_area`'s content is every line at its line height by the widest line in view, so its content width changes as lines of other widths scroll in.
- `scrollbar(e, ScrollbarOptions)` draws a bar over the padding box of a scrolling element, `e`, on each axis that scrolls and overflows. The bar has a groove in `track`, `width` thick, and a thumb in `thumb`. The thumb's length is `min_thumb` at least and otherwise the track's length times viewport over content. Its place follows the element's offset. A second bar leaves the corner between them empty. A press on the thumb drags it, keeping the point grabbed under the pointer, even once the pointer leaves the bar and the element. A press on the groove scrolls one viewport toward the press. Neither reaches the element's click handlers, and the wheel scrolls as before. The thumb is `hot` under the pointer and while dragged. It is drawn `inset` in from each long side of the bar, or `hot_inset` while hot, with the whole width of the bar still taking the pointer. `margin` keeps the groove and the thumb's travel short of each end of the edge. `autohide` draws the bar only while the pointer is over the element or a drag is on. A `list` is dragged with `list_scroll_by`. A `text_area` draws both bars without an overflow style, and a press on one neither moves its caret nor gives it focus. The bar of an element that does not scroll draws nothing.
- A clip applies to painting and to hit testing, so a child scrolled out of view is not hovered, pressed or scrolled. The hitbox of a clipped element is its rectangle intersected with its ancestors' clips.
- A `Scrolled` event goes to the hitboxes under the pointer from the innermost outward. Each runs its `on_scroll_wheel` handler with a `ScrollEvent` of the pointer position, the distances as the wheel reported them and the `Modifiers` held. The first scrolling element that moves stops the event. One already at the end of its range passes it on. A vertical-only scroll (`dx` zero) made with Shift held and no Control, Alt or Super scrolls the elements horizontally by `dy`, so Shift with the wheel turning down moves the same way as a wheel turning right; a container with no horizontal range to move passes it on like any other scroll at the end of its range, and a `list` or `uniform_list`, which scrolls only vertically, passes it on.

Virtual lists:

- `uniform_list(id, count, render)` and `list(state, render)` are scrolling elements that build only the rows in view. Each has `overflow_y: Scroll`, no size of its own, no children, and a hitbox, so `height`, `flex_1` and the like size it and its clip, hitbox and wheel routing are those of `overflow_y_scroll`. They are in `std/ui/list.low`. `Element` carries the render closure and the handle or state in `rows`.
- The tree is laid out in one pass and a list needs its own size before it can render rows, so the rows are laid out after it. The prepaint walk reaches a list with its content box known. It asks `render` for the rows in range. It lays out each row as its own `layout.Tree` root. That layout is `layout.compute` with the content width and, for `list` and for the measured item of `uniform_list`, an `unbounded` height. A `uniform_list` row gets the item height. The walk prepaints each row as a child of the list. The row is shifted to its position and clipped to the list's content box, so the list's padding and border stay still while the rows scroll. The list's hitbox is the row's parent. The row trees stay in the context until painting draws them inside the list's clip, in the order they were prepainted. `Walk` carries the inherited text style and the scale for this, since a row is built where only the walk is known. A row without an `id` has the key `"<list key>/<index>"`, so hover, press, focus and a field's caret stay with the row as it scrolls. Paint draws a list's rows and skips the children of the list element, as prepaint does. Rows are kept per list element and frame, by the index of the list's hitbox, so two lists with one `id` each draw their own rows. A row's laid out height is never below zero.
- A `uniform_list` follows `uniform_list.rs`. A frame calls `render(0, 1)` and lays that row out at the content width, which gives the height of every item (`measure_item`, `uniform_list.rs:650-672`, which lays it out at the minimum content width; `layout` has no minimum-content root). Then it calls `render(start, end)` for the visible range `floor(offset / height)` to `min(ceil((offset + viewport) / height), count)` (`uniform_list.rs:450-457`). The scroll range is `count * height` less the content box's height. The offset is kept under the list's `id` in the same map `scroll_offset` and `scroll_to` use. A row sits at `index * height` less the offset. The wheel scrolls it like `overflow_y_scroll`. The visible range and the row positions are computed in `f64` from the index and the offset. The offset itself is an `f32` as in gpui, so its step doubles at each power of two past 16777216 pixels. A list with no items or a zero-high item draws nothing. A list with no items leaves its stored offset alone.
- `uniform_list_scroll_handle(cx)` makes a `UniformListScrollHandle`, a handle whose copies share state. `track_scroll(e, h)` attaches it to a list. `scroll_to_item(h, index, strategy)` sets a request that the next frame applies in prepaint as `uniform_list.rs:393-448` does. The stored offset is first kept within `0` and the scroll range, as `uniform_list.rs:378-384` does. Nothing moves when the item is fully in view. Otherwise the offset is the item's top for `ScrollStrategy.Top`, the item centred in the viewport for `Center` and the item's bottom at the viewport's bottom for `Bottom`, kept within `0` and the scroll range. gpui 0.2.2 has no `Nearest`. `scroll_to_item_strict` applies the strategy even when the item is in view. The `_with_offset` forms shorten the viewport by whole items from the top for `Top` and `Center` and from the bottom for `Bottom`. `logical_scroll_top_index(h)` returns the requested item while a request is pending and else the first item at the offset. `visible_range(h)` returns the range `render` was asked for. `item_height(h)` returns the measured height. `is_scrollable(h)` reports whether the items are taller than the viewport.
- `list` follows `list.rs`. `list_state(cx, count, alignment, overdraw)` makes a `ListState`. It is a handle that keeps the item count, each item's measured height and the scroll position. The scroll position is a `ListOffset` of an item index and an offset into it, so the position stays on its item when heights above it change. `ListAlignment.Top` starts at the first item and `Bottom` rests on the last. A `Bottom` list shorter than its viewport sits at the bottom of it (`list.rs:539-551`, `list.rs:700-717`). A frame follows `layout_items` (`list.rs:604-779`). From the scroll position it lays out items forward until the viewport and `overdraw` are filled. It lays items out above when they do not fill the viewport and moves the position up. Then it measures items in the leading overdraw. Only the items in the viewport are drawn. An item outside it is laid out once and its height kept. A row that has been measured is laid out again at each frame in which it is drawn. Items that have not been laid out count as zero high (`list.rs:1117-1123`), so the scroll range covers the items laid out and a list whose rows exactly fill the viewport with no `overdraw` has nothing to scroll. A change of the content width drops every height (`list.rs:1025-1038`).
- The wheel on a `list` is `StateInner::scroll` (`list.rs:487-537`). It moves the position from the one the last frame resolved by the wheel's distance within `0` and the heights laid out less the viewport. A `Bottom` list that reaches the end returns to resting on the last item. Events are dropped between `list_reset` and the next frame. `list_scroll_to(s, offset)`, `list_scroll_by(s, distance)` and `scroll_to_reveal_item(s, index)` set the position as `list.rs:348-389` do. `logical_scroll_top(s)` reads it. `bounds_for_item(s, index)` returns where a laid out row was drawn at the last frame, unlike gpui, which computes it from the logical scroll top. `item_count(s)` returns the count. `list_reset(s, count)` and `list_splice(s, start, end, count)` replace items with ones not laid out and keep the position on its item as `list.rs:244-307` do. They carry the `list_` prefix because `reset`, `scroll_to` and `scroll_by` are taken in `std/ui`. The heights are an `f32` slice and a Fenwick tree of `f64` sums, so an item's offset and the item at an offset are found in logarithmic time. A splice or reset rebuilds the sums in linear time, where gpui uses a `SumTree`.
- Not implemented from gpui: `ListState::measure_all`, the scroll handler, the scrollbar offsets, keeping a focused row that has scrolled out of view (`splice_focusable`), autoscroll requests from a row, `ListSizingBehavior::Infer`, `uniform_list` decorations, `y_flipped` and horizontal scrolling of `uniform_list`.

Canvases:

- `canvas(paint)` is a box that draws vector shapes at its place in the tree. `paint` is a `fn(Painter)`. `std/ui` calls it while it paints the frame, after layout, with a `Painter` whose `width` and `height` are the size of the canvas's content box. The shapes draw above the canvas's background, in the order `paint` makes them, and under its children and border. A canvas has no size of its own. The builders of any box size, pad, border, fade and position it. Its children, laid out in the content box, are how a chart carries text. `paint` does not run for a canvas that is `invisible` or inside an `invisible` element.
- Every point, rectangle, radius and stroke width a `Painter` method takes is in logical pixels, and a point is measured from the top left of the content box. Painting moves the point by the box's place, which includes the scroll offsets of the canvas's parents. It multiplies the point and the widths by the window scale. It multiplies each colour's alpha by the opacity of the canvas and its parents. It pushes the shapes inside a clip of the canvas's padding box, which narrows the clips of its parents. A shape whose faded colour is transparent is left out of the scene. A canvas that itself scrolls moves its children and not its shapes. Padding is therefore room for a stroke that runs along an edge of the content box.
- `fill_rect(p, r, color)` and `fill_circle(p, center, radius, color)` push quads. The rectangle has its edges rounded to device pixels as a background's are. The circle is a square with corner radii of `radius`. `fill_triangle(p, a, b, c, color)` and `fill_polygon(p, points, color)` push a `scene.Path` that fans from the first point. So they fill a convex outline, or one where the straight line from the first point to every other point stays inside, and no other. `fill_area(p, points, baseline, color)` fills between the line through `points` and the horizontal line at `baseline` as two triangles a segment, or one on each side of the baseline for a segment that crosses it, which is the shape of an area chart and one a fan cannot fill. Its triangles do not overlap while the points run one way along x.
- `stroke_line(p, a, b, width, color)`, `stroke_polyline(p, points, width, color)`, `stroke_quadratic(p, a, ctrl, b, width, color)` and `stroke_cubic(p, a, c1, c2, b, width, color)` push the path of `scene.stroke_polyline`. The curves run along `quadratic_points(p, a, ctrl, b)` and `cubic_points(p, a, c1, c2, b)`. These return the points of the flattened curve in the canvas's coordinates, within a tenth of a device pixel of it. They serve a fill or an area with a curved side. The strokes keep the limits of the scene's. These are butt caps, mitred or bevelled corners, no dashes, and triangles that overlap inside a turn sharper than its segments are long, where a translucent colour is darker. Fewer points than a shape needs, or a width of zero or less, draw nothing.
- A canvas takes part in hit testing as the box it is: `on_click`, `on_mouse_move`, `hover`, `cursor` and the other handlers work on it and give it a hitbox. Events carry window positions, so `canvas_point(cx, key, position) -> Point?` returns `position` measured from the top left of the content box of the canvas under `key` as the last frame placed it. These are the coordinates `paint` draws in. It returns `none` when the last frame placed no canvas under that key. Prepaint records the content box of every canvas by its key, hidden ones included.
- The options few elements set are held behind one pointer in the `Element`, `extras`. A builder replaces it with a changed copy. The options are the paint function, the `scrollbar` options, the label and role, a field's placeholder, colours and masking, an image and its fit, a list's rows, a text area's lines, tab size and blank marks, the key context, the drag behaviour and the overlay placement. An `Element` is copied at each level of the tree while it is built and while `build` turns it into the layout tree, so its size sets how deep a tree fits the stack, and an option few elements use belongs behind that pointer. Prepaint and paint walk the tree through a stack of steps on the heap, so their stack use does not grow with depth; `testdata/run/ui_deep_tree` draws a tree 200 rows deep in one frame.

Text fields, the cursor and the clipboard:

- `image(picture)` is a leaf that draws `picture`, a value of `add_image(cx, width, height, rgba)`, over its border box through `text.paint_image`, with the element's corner radii. A width or height that is not set follows gpui's `Img::request_layout` (`img.rs:277-400`). If both are unset, they take the picture's pixel size as logical pixels. If one is unset, it follows the other's logical size in the picture's proportions. `object_fit` takes a `text.Fit` (`Contain` by default, as `img.rs:136-145`). `grayscale` draws luminance.
- `add_png(cx, data)` decodes a PNG with `std/image`, converts it to `Rgba8` with straight alpha and keeps it as `add_image` does. `add_png_file(cx, path)` reads the file first. Every colour type, bit depth and interlacing is accepted. Palette and `tRNS` become alpha, and 16-bit samples round to 8 bits. Gamma and colour profile chunks are not applied. Both fail with the `io.Error` of `std/file` and `image.decode_png`. `testdata/ui_png` holds the PngSuite (Willem van Schaik, free to use). `suite.golden` lists each file's size and the CRC of its 8-bit pixels, equal to ImageMagick's for every decodable file. The 14 corrupt `x*` files fail with `Malformed` or `Unsupported`.
- `icon(source)` is a leaf 16 by 16 logical pixels that draws the SVG document `source` through `text.paint_icon` over its border box in the inherited text colour; `size`, `width` and `height` change the box.
- `field(s)` is a single-line text field showing `s`. It is a leaf 200 logical pixels wide that is `focusable` and shows the text cursor. The box builders and `text_color` and `text_size` change it. It keeps no text. `on_change(e, f)` runs `f` with the text after each edit. The application stores the text in a signal that the next frame passes to `field` again. A field whose application ignores `f` shows the same text. `field_signal(text)` is a `field` showing `get(text)` whose `on_change` sets `text`. A field with an `id` keeps its caret, selection and horizontal scroll from one frame to the next wherever the tree puts it.
- The caret and the anchor of the selection are byte offsets, kept per key and clamped to the text each frame. `field_selection(cx, key)` returns them as a `FieldSelection` as of the last event. As `get` does, it makes the running effect, memo or tracked frame depend on the field's selection and input method composition. These change after each `dispatch`, each frame and each setter that moved them, and run the effects that read them. `set_field_selection(cx, key, anchor, caret)` selects between two offsets. `select_field_text(cx, key)` selects all of the text with the caret at its end, as `text_area_selection` and `set_text_area_selection` do for a text area. A setter keeps its request on the field's state, which it creates when the key has none. It resolves the request when the field is next painted, against the text it shows then. So a call made in the event that first adds the field to the tree, or together with a change to the text, applies to the next frame. An offset past the end of the text moves to the end and an offset inside a grapheme cluster to the nearest cluster boundary. A setter asks for a new frame and scrolls the field to show the caret even when it is not focused. It does not move focus, and focus does not change the selection. So `set_focus` and a setter work in either order. A key, a click, a drag or an input method event that arrives before that frame resolves the request against the text of the last frame first. Left and Right move by extended grapheme cluster, and by word segment (`unicode.word_end`, skipping spaces) with Control. Home and End move to the ends. Shift keeps the anchor and so selects. An arrow without Shift collapses a selection to its near edge. Backspace and Delete remove the selection, else a cluster, or a word with Control. Control or Super with A selects all, C copies, X cuts and V pastes.
- Typed text, a paste and an input method's `Commit` replace the selection and drop every code point below space and DEL, so a field holds one line. The text of a key press is inserted from the `Key` event alone. An input method's `Preedit` is shown at the caret, underlined, in place of the selection. It is not part of the text until a `Commit`. Key releases, shortcuts and keys that make no text insert nothing. Tab, Enter and the other keys fall through to focus movement and click handling, so Enter runs the field's `on_click`.
- A press places the caret at the nearest cluster boundary of the pointer, and a drag with the primary button held extends the selection from there. The caret is a one device pixel quad drawn when the field is focused. The selection is a translucent quad behind the text. Both are clipped to the field's content box, and the text scrolls to keep the caret inside it.
- `text_input(cx) -> Caret?` is where the focused field's caret was in the last frame, in logical pixels from the top left of the window, and it is `none` when no field is focused. `run` hands it to `platform.set_text_input` after each frame that changed it. That call turns the input method on for a field and off otherwise.
- `cursor(e, c)` sets the shape shown over `e` and over its children that set none. `pointer_cursor(cx)` returns the shape of the innermost element under the pointer that sets one, else `Arrow`. It returns `Arrow` over a `scrollbar` and while its thumb is dragged, whatever the element under the bar sets. `run` hands it to `platform.set_cursor` when it changes.
- `clipboard(cx) -> str?` and `set_clipboard(cx, s)` read and write the clipboard. A context with no window keeps the text in memory. `open` makes them the window's system clipboard. They fall back to the last text written when the system clipboard cannot be read.
- `read_only(e)` marks a `field` or a `text_area` as showing text the user may select and copy but not change. On any other element it does nothing. A read-only element keeps focus by click and by Tab, selection by pointer (click, drag, double and triple click) and by keyboard (Shift with the arrows, Home and End, Control variants, select all), copy, caret movement, scrolling, and the drawn caret and selection. It refuses every path that ends in a change of text. These are typed characters, Backspace and Delete with or without Control, cut and paste, an input method's `Preedit` and `Commit`, and in a text area also Enter, Tab, undo, redo and the built-in handlers of `text_area.backspace`, `delete`, `delete_word_left`, `delete_word_right`, `new_line`, `tab`, `cut`, `paste`, `undo` and `redo`. A refused key changes neither the text, the caret nor the selection, and `on_change` and `LineSource.edit` are never called. A preedit already showing when an element becomes read-only is dropped at the next frame. A refused key is not consumed. Its action is left unhandled by the area, so `on_action` handlers of the elements around it still run. Then the key goes to `on_key` handlers outward from the focused element and on to focus movement and click handling, as a key the element does not use does. Tab therefore moves focus on from a read-only text area and Enter runs its `on_click`. `is_action_available` and `available_actions` leave the refused actions out for a focused read-only text area. `text_input(cx)` is `none` while a read-only element is focused, so the platform opens no composition for it.

Text areas:

- `text_area(source)` is a multi-line editor over a `LineSource`, a struct of function values the application provides: `line_count() -> i64`, `line_text(i) -> str` (a line without its line break), `edit(TextEdit)`, and the optional `edit_all(TextEdit[])`, `line_runs(i) -> TextRun[]`, `line_fill(i) -> Color?`, `line_marks(i) -> TextMark[]`, `line_hidden(i) -> bool` and `hidden_before(i) -> i64`. `line_source(line_count, line_text, edit)` leaves the optional functions empty, with `line_hidden` returning `false` and `hidden_before` returning `0`. The application owns the document. The functions read the live document and not a snapshot, since keys, clicks and an input method's text reach the area between frames. A `TextEdit` replaces the text from `start` up to `end` with `text`. Each of `start` and `end` is a `TextPosition` of a line and a byte offset in it. The line breaks in `text` are `\n`. The area works out where its caret lands from the replacement and takes the removed text before it calls `edit`. `edit` applies exactly the change it is given. After each call the area reads back the lines the edit touches and the line count. When the document differs from what the edit describes, as when the application ignores or alters it, the area drops its undo and redo history and keeps its caret within the text the document holds. Before it reads a position, whether for a key, a click, a drag, an input method's text or `set_text_area_selection`, the area moves the caret and the anchor into the current document: the line to the last line, and the byte offset to the line's length and down to a grapheme boundary. Because of this, an edit the application made to the document between frames never leaves a position outside it. A count below one is read as one empty line.
- The element is a leaf 300 by 200 logical pixels, `focusable`, with the text cursor and the key context `TextArea`. The box builders and `text_color`, `text_size`, `font_family`, `font_weight`, `font_style` and `line_height` change it. A `key_context` added to it extends `TextArea` and does not replace it. It keeps per key, and so by `id` where the tree moves it, the caret and the anchor as `TextPosition`s, the preferred column of vertical moves as a distance in layout pixels, the scroll position, the preedit and the undo and redo history. `text_area_selection`, `set_text_area_selection`, `text_area_selections`, `set_text_area_selections`, `text_area_top_line` and `scroll_text_area_to_line` read and set the selection or selections and the first line in view. `text_area_top_shift(cx, key)` returns the pixels of the first line scrolled out of view above the top. With `text_area_top_line`, it lets a gutter beside the area follow it between whole lines. `reset_text_area(cx, key)` clears the caret, the selections, the history, the preedit and the scroll position kept under a key, as for an area just added to the tree. It is for the application that replaces the document. `placeholder(e, s)` and `placeholder_color(e, c)` apply to a text area as to a field. The placeholder is drawn from the start of the content box while the document is one line with no text and no composition is showing. It is never part of the document. `selection_color(e, c)` sets the colour a field or a text area paints its selection in. Without it the colour is blue at an alpha of 0.4. `text_area_apply(cx, key, edits, after)` makes edits of the application's own, such as toggling a comment or indenting a selection, one undoable step. The edits name positions in the document as it is, may come in any order and must not overlap. They reach the source last position first as one `edit_all` call or one `edit` call each, with the history, the read-back check and the selection restored by undo from before the step as for several selections. The selection becomes `after`, and the step is its own group like a paste. An invalid step (outside the document, overlapping, all empty) or a `read_only` or absent area leaves everything as it was. The edits and `after` keep the document's line numbers, so a step may edit the lines of a fold. `text_area_point_of(cx, key, position)` returns the top left, in window coordinates, of the cell of a position as the last frame drew it. It returns `none` when the position lies outside the content box or on a hidden line. A line below a fold is at the row it is drawn in. `text_area_position_at(cx, key, point)` returns the position a click at a window point selects, or `none` when the area was not drawn. Neither changes the area, so a completion popup can anchor to any offset and the application can build double-click and alt-drag selections of its own. `reveal_text_area_selection(cx, key, selection)` makes the next frame scroll to the caret of `selection` on both axes in place of the primary selection's. It is for the application that adds a selection with `set_text_area_selections` and wants that one in view. It moves no selection and applies to one frame. A key, click or drag before that frame drops it. `text_area_selection` and `text_area_selections` are tracked reads like `field_selection`: they depend on the area's selections and input method composition.
- The editing keys are actions, as in gpui's `examples/input.rs`, which declares `actions!` for each key and binds them with `KeyBinding::new` (`input.rs:13-35`, `681-693`). `text_area_bindings()` returns the default bindings in the context `TextArea`. The application passes them to `bind_keys`, so a later binding of the same keystrokes wins and an empty action unbinds. The actions are `text_area.left`, `right`, `up`, `down`, `word_left`, `word_right`, `line_start`, `line_end`, `document_start`, `document_end`, `page_up`, `page_down` and the `select_` form of each, `backspace`, `delete`, `delete_word_left`, `delete_word_right`, `new_line`, `tab`, `select_all`, `copy`, `cut`, `paste`, `undo`, `redo`, `add_cursor_above`, `add_cursor_below`, `select_next_occurrence`, `select_all_occurrences` and `collapse_selections`. Dispatching one runs the `capture_action` handlers of the elements on the path first, from the outside in. Then it runs the area's own `on_action` handlers, then its built-in handler, which stops the action. So the focused area has the first claim, as the handlers of gpui's example are registered on the focused element. An application overrides an action on the area by handling it there. It keeps the default by calling `propagate`. An element around the area intercepts an editing action with `capture_action`, which stops the built-in handler when it calls `stop_propagation`. An `on_action` handler on an ancestor does not run for an action the area handles. `is_action_available` and `available_actions` list them for a focused area. Typed text is not an action: a key that makes text and is bound to nothing inserts that text at the caret, as the field does, with every code point below space except Tab dropped.
- Left and Right move by extended grapheme cluster, and by word with Control, and cross from the end of a line to the start of the next. Up and Down keep the column the caret had when the vertical run began, as the distance in layout pixels from the left edge. They land on the grapheme boundary nearest it, so a short line in between does not lose the column. Up on the first line goes to its start and Down on the last to its end. Home and End go to the start and end of the line. Control with them goes to the start and end of the document. Page Up and Page Down move the caret and the view by the lines in view less one. Shift extends the selection from the anchor. An arrow without Shift collapses a selection to its near edge, as the field does. Backspace and Delete join lines at a line's start and end. Control makes them remove a word. A selection of several lines is removed by typing, Backspace, Delete, cut or paste.
- Tab inserts a tab character through `text_area.tab`, as a code editor does. An application that wants Tab to move focus binds `tab` to the empty action; Shift+Tab is not bound, so it moves focus back. Enter inserts `\n` through `text_area.new_line`. Escape is bound to `text_area.collapse_selections`, which the area handles only while it holds several selections, so with one the key bubbles to the application as before.
- `tab_size(e, n)` sets the tab stops of a text area. A `\t` advances to the next multiple of `n` columns, counted in code points from the start of the line with earlier tabs expanded. The line is shaped with each tab as the spaces that fill that span. `n` below `1` is `1`, which is also the default and keeps a tab one column wide. The caret, a click or drag (which lands on the nearer edge of a tab's span), the selection rectangles, the preferred column of Up and Down and the widest line all read the same expanded columns. The text, `copy`, the edit offsets and every byte offset of the `LineSource` are unchanged.
- `blank_marks(e, color, caret_line)` draws `·` over each space and `→` at the first column of each tab, in `color` and on the caret's line in `caret_line`. A no-break space and every other `White_Space` code point that is not a line break gets a `·`. The marks are drawn after the line's text from glyphs laid out in the area's own style. A `·` is centred in its blank's cell and a `→` starts at the left of the tab's first column. They are drawing only, so text, `copy`, columns and widths do not change, and only the lines in view are visited. Without it nothing is laid out or drawn for blanks.
- A press places the caret on the line under the pointer and the nearest grapheme boundary on it. The caret goes to the end of the document when the pointer is below the last line or the content box, and to the start when it is above the box. This is as gpui's `index_for_mouse_position` returns the end of the content for a position below the bounds and the start for one above (`input.rs:166`). A press with Shift moves only the caret, as gpui's `on_mouse_down` calls `select_to` when Shift is held (`input.rs:97-108`, `184-195`). A press with a click count of two selects the word under the pointer and one of three selects the line with its break, or to the end of the last line. A drag after a single press extends the selection, also with the pointer outside the area, and a drag after a double or triple click does not. The `selection_reversed` flag of gpui's example is the order of the area's anchor and caret.
- Copy writes the selected text with `\n` between lines. Paste inserts the clipboard text. It turns `\r\n` and `\r` into `\n` and drops every code point below space except Tab and `\n`. gpui's single-line example replaces newlines with spaces (`input.rs:131-136`).
- Undo and redo apply inverse edits through `source.edit`. gpui has no text history, so the grouping is this module's own. An undo step is a group of edits and restores the selection, or every selection, from before it, and redo restores the ones from after it. Typed characters and Tabs in a row, Backspaces in a row and Deletes in a row each form one group. A group stops growing 300 milliseconds after its last edit on the context's clock, when the caret moves by key, click or `set_text_area_selection`, or when the selection is not empty at the edit. Enter, paste, cut, a commit and removing a selection are each a group, and so is any key that edits at several selections at once. A new edit drops the redo steps. The history keeps 1,000 groups. An edit the application makes to the document itself is not in the history. An undo or redo step whose positions do not lie in the document, or whose edit the application does not apply as given, stops there and drops the history.
- A text area holds one or more selections, each an anchor and a caret, kept in document order and never overlapping. The first is the primary: the view scrolls to it, the input method's preedit and `text_area_selection` follow it, and `set_text_area_selection` replaces all of them with one. `text_area_selections(cx, key)` returns every selection in document order and `set_text_area_selections(cx, key, selections)` replaces them. An empty slice changes nothing, and the positions are fitted to the document as `set_text_area_selection` fits one. Selections that overlap merge. An empty selection touching another merges into it. Two non-empty selections that only touch stay apart. A merged selection keeps the direction of whichever reached further. Every caret is drawn and every selection painted. A frame finds the ones in view by binary search, so its cost follows the lines in view and the selections among them and not the document or the selection count. A single selection takes the same paint path with no allocation. Movement keys move every caret, each keeping its own preferred column, and carets that meet merge. Typed text, an input method's `Commit`, Enter, Tab, Backspace, Delete, the word deletions, cut and paste apply at every selection as one step: one undo group that restores the whole set of selections from before it on undo and from after it on redo. Copy joins the selections' texts with `\n`. Paste splits the text at `\n` and gives one line to each selection when there are as many lines as selections, and otherwise inserts the whole text at each. A step of several edits reaches the source as one `edit_all(edits)` call when the source provides it, and as one `edit` call per element otherwise. The edits are ordered last position first by `start` and then `end`, so applying them in order never moves a position a later one names. Every position is in the document as it was before the step. No two edits overlap, as edits that would overlap, such as a word deletion from two carets in one word, are merged into their union first. An insertion at the start of a deletion comes after the deletion. Undo and redo replay a group through `edit`, one change at a time. After the batch the area reads back the line count and each edit's text as for a single edit and drops its history on a difference. `text_area.add_cursor_above` and `text_area.add_cursor_below` (Control+Alt+Up and Down) add a caret one line above or below the caret of each selection, at its preferred column. `text_area.select_next_occurrence` (Control+D) selects the word at the caret when the primary selection is empty. Otherwise it adds the next match of the newest selection's text after it, wrapping round the document, comparing case-sensitively, skipping a match that overlaps a selection and doing nothing when every match is selected. `text_area.select_all_occurrences` (Control+Shift+D) selects every match. A press with Control adds a caret and a double or triple click with Control adds the word or line, and dragging after it extends the added selection. A drag with Alt makes a column selection. It has one selection per line between the pressed and the pointer's line, each between the two columns read from that line's own shaped text. So `tab_size` stops are honoured, and a line shorter than the column gets a caret at its end. With Control and Alt the column selections are added to the existing ones. A plain press collapses to one selection. A drag with the middle button makes the same column selection as a drag with Alt, from the press to the pointer, with Control keeping the existing selections as Control and Alt does. A middle press without movement leaves one caret at the press, takes focus as a left press does and pastes nothing. The drag keeps following the pointer outside the area, as the platform keeps the pointer captured for any held button on Windows. It ends at the release of the middle button. A middle press does not stop propagation, so `on_mouse_down` handlers for `Button.Middle` on the area and the elements around it still run.
- An input method's `Preedit` is shown underlined at the caret, and `Commit` inserts at the caret. A preedit that starts while the selection is not empty first removes the selection through `source.edit` as a history group of its own, so no hidden text is drawn and the lines after a multi-line selection move up. A commit with a selection still present replaces it in one group. The preedit itself is not in the history until the commit. The platform delivers the preedit's range in bytes of the preedit, and the area uses it as the field does. gpui's `impl EntityInputHandler` (`input.rs:261`) counts UTF-16 units over the whole document, and its `text_for_range`, `bounds_for_range` and `character_index_for_point` (`input.rs:262`, `352`, `373`) have no counterpart here.
- The view is a top line, as an `i64`, and the pixels of it scrolled off, below one line, so a document of a million lines keeps full precision where a pixel offset in `f32` would not. The wheel scrolls both axes and, like a scrolling box, hands the event on when it cannot move. After any caret move or edit the next frame scrolls the least that brings the caret into view on both axes, as `uniform_list`'s `scroll_to_item` does for a row (`uniform_list.rs:395-440`). A wheel turn or `scroll_text_area_to_line` afterwards is left alone. The horizontal range is the widest line among those in view, so it follows the lines the view shows and not the whole document. Lines do not wrap.
- `line_hidden(i)` folds a line. The line is not read for drawing and takes no height. The caret and the vertical, page, horizontal and column-selection moves skip it. A click or drag lands on the line drawn at that row. `Ctrl+Home`, `Ctrl+End` and a click above or below the lines go to the first and last lines drawn. Positions, `TextEdit`s, selections, the undo history, `text_area_top_line` and `scroll_text_area_to_line` all keep the document's line numbers. The wheel, the page keys, the scroll bar and the reveal of the caret count only the lines drawn, so the scroll range is the number of lines drawn at the line height. `hidden_before(i)` is how many of the lines before line `i` are hidden, `hidden_before(line_count())` being the total. `hidden_before(i)` gives the scroll range, the thumb offset, `text_area_visible_row` and the line a bar drag lands on without a pass over the document. The area calls `line_hidden` only while it reports a hidden line, and then about lines near the view and the caret. So a source that hides nothing is never asked and costs what it did without the feature. A top line that becomes hidden moves to the nearest line drawn above it, or below when none is. A selection across a hidden range keeps covering it, so copying it copies the folded text. An edit inside a hidden range is the application's to refuse. `text_area_visible_row(cx, key, line)` returns how many lines drawn lie between the first line in view and `line`, or `none` for a hidden line or one out of view. So a gutter numbers each row by walking `line_hidden` from `text_area_top_line`. It places each row that many rows below the top, less `text_area_top_shift`.
- A frame reads from the source only the lines that intersect the view, which is the visible range of `uniform_list` (`uniform_list.rs:450-457`) at a fixed line height, the text size times 1.618034 rounded to a device pixel until `line_height` sets it; nothing is read for the lines outside it, the caret's line included when it is out of view. A click, a key or a drag reads the lines it needs, a few at most apart from copying or deleting a selection.
- A line is drawn from its runs, the plain gaps between them in the element's text style. Each run is shaped on its own, in the element's colour unless the run sets one and in its `weight` when the text system has a face for it. The pieces sit one after another, so the caret, the selection and a click all read the same positions. `line_fill` fills the full width of a line's row behind its selection and text, across the area's padding on each side. A line's gutter is not supported. A `TextRun` may carry a `background`, a colour drawn from its first byte to its last at the height of the line, behind its text. A `TextMark` is a stretch of one line, a `start` and `end` byte offset, a `color` and a `MarkStyle`. The application returns it from `line_marks` for a search match, a diagnostic or another participant's caret and selection. The area asks for the marks of the lines drawn only, so never for a hidden line, and not for the line that holds the input method's preedit. `Fill` draws the colour behind the text. `Underline` draws a straight `scene.Underline` one logical pixel thick at the foot of the line. `Squiggle` draws a wavy `scene.Underline` of the same thickness and three times as high at the foot of it. Both are over the text. `Bar` draws a one-pixel bar the height of the line at `end`. A stretch with no bytes draws that bar at its offset in every style. The underlines are in device pixels, snapped as a quad is, and the clip of the content box cuts them. Offsets are moved to the line's end and down to a grapheme boundary, an `end` before `start` is read as `start`, and marks are drawn in the order given. From the bottom up a line is its `line_fill`, run backgrounds, `Fill` marks, the selection, the text, the other marks and the caret.
- A focused area draws these things, clipped to its content box: the fill of each line in view, a translucent selection rectangle on each line the selection touches, the text of each piece, and the caret. The caret is a quad one device pixel wide and a line high, as the field does. The rectangle of a line the selection continues past is extended by the width of a space, so an empty line in the selection shows. A composition is underlined one device pixel high. A tab is shaped as a space, byte for byte. Offsets therefore stay aligned, and the font's missing-glyph box is not drawn. `text_input(cx)` reports the caret's place in logical pixels from the top left of the window. It does so while the caret is inside the content box on both axes. Otherwise it reports `none`, as for an unfocused area.

### UI tests

- `std/ui/dwrite/dwrite_test_windows.low` and `std/ui/text/text_test_windows.low` test `std/ui/dwrite` and the installed-font half of `std/ui/text`. They run on Windows against the fonts that ship with it.
- `std/ui/fontconfig/fontconfig_test.low`, `fontconfig_test_linux.low` and `std/ui/text/text_test_linux.low` test `std/ui/fontconfig` and the installed-font half of `std/ui/text` on Linux. The tests build fonts, collections, variable fonts and `fonts.conf` files in a temporary directory, so they do not depend on the machine's fonts. They check these things: include order and prefixes, symbolic links and loops, the cache and its invalidation, the CSS order of width, slant and weight, aliases, code point fallback, and the face, metrics, weight, underline and strikethrough strokes, and sheared or unsheared italic and oblique a layout gets. One test compares `find` with `fc-match` for `sans-serif`, `serif` and `monospace` on the machine's own fonts. It skips without fontconfig.
- `std/ui/svg/svg_test.low`, `std/ui/svg/svg_mask_test.low` and `std/ui/text/text_test.low` test `std/ui/svg` and the built-in-font half of `std/ui/text`. They check parsing, errors, the verbs `usvg` builds for quadratic curves, circles and closed subpaths, masks equal to the ones `resvg` renders for a stroked line, curves and arcs, and a filled rounded rectangle with strokes, and the sprites `paint_icon` pushes.
- `testdata/run/ui_*` programs test `std/ui/layout`, `std/ui/font` and `std/ui/text`. They need no library, so they run on Linux and, through the `windows` suite, on Windows. They check layouts against hand-computed rectangles, glyph metrics and rasterised coverage of the fallback font, line breaking and atlas packing. `testdata/run/ui_font_cff` parses name-keyed and CID-keyed OpenType CFF fonts built with fontTools, checks their outlines and advances against the ones fontTools' `RecordingPen` gives, probes each Type 2 operator and limit through fonts it assembles, rejects damaged CFF tables, and rasterises curved glyphs. `std/ui/geometry/geometry_test.low` and `std/ui/scene/scene_test.low` check rectangle and colour arithmetic, clipping, draw order, scene batching by atlas and the glyph atlases' dirty rows, tiles, clearing and added textures.
- `std/ui/*_test.low` runs with `low test std/ui`. It drives a `Context` with no window and no GPU. It checks the scene a frame returns against hand-computed rectangles and colours. It dispatches pointer and key events to check hover, click, focus and key bubbling. `std/ui/decoration_test.low` checks the title bar `frame` draws above the tree when the window asks for a decoration height. `std/ui/input_state_test.low` checks that effects and memos depend on `field_selection` and `text_area_selections` and on a `field_signal`. `std/ui/titlebar_test.low` checks which area lies under a point for nested areas, handlers, fields, clipping and overlap. It checks that `Maximized` events and `toggle_maximize` change `is_maximized` and make a frame due. It checks that `minimize` runs the window's command. `std/ui/examples/title_bar.low` runs the same headless. `std/ui/window_test.low` opens several headless windows on one application. It checks that a signal change or a drained post makes each due. It checks that the keymap and the clock are shared while chords and timers stay per window. It checks that a task delivers through a closed window's signal. It checks how `close_window` and `open_window` behave. `std/ui/task_test.low` holds a task's work at a channel. This checks that its result reaches a signal only on the context's green thread, after the work returns, and never once the task is cancelled.
- `compiler/tests/ui` builds Low programs that import the `std/ui` modules and runs them:
  - offscreen renders of every primitive kind, clipping, draw order, frames in flight, instance buffer growth, atlas updates and sprites that sample different atlases in both orders and interleaved by draw order, compared by pixels; an element tree driven through hover, press, focus, click and scrolling; and the `parity` and `overlays` scenes. The pixels of those last two scenes at the spots the tests name must equal the ones gpui-fast 0.1.12 draws for the same element trees. The second scene covers `deferred`, `priority` and `anchored` with each fit mode. These tests build for the host, so they run on Linux and on Windows. They skip without libvulkan or a Vulkan 1.3 device;
  - the `virtual_lists` scene, a `uniform_list` scrolled to an item with `Center` and a `list` scrolled into an item, each cut by a clipping box. Its pixels must equal the ones gpui-fast 0.1.12 draws for the same element tree;
  - the `group_hover` scene, two groups with one under the pointer and a child outside any group that names it. Its pixels must equal the ones gpui-fast 0.1.12 draws for the same element tree with the desktop pointer moved onto the group;
  - on a Windows host, the `placeholder` scene, two empty fields drawn through DirectWrite. Their box, border and background must equal the ones gpui-fast 0.1.12 draws for the same element tree. Their placeholder ink must lie inside the content box in its colour. The test skips without a display or a device;
  - on a Windows host, the `text_area` scene, a focused text area with a line fill, a selection across two lines and the caret drawn through DirectWrite. Its fill, selection rectangles and caret must equal the ones gpui-fast 0.1.12 draws for the same element tree. Its text ink must lie inside its rows. The test skips without a display or a device;
  - the Wayland listener callbacks and key translation, driven directly by a program built from the platform's sources, which also covers text input composition, fractional and whole scale preferences, decoration modes with the title bar height and resize edges they give, a frame's close button, every cursor shape, clipboard offers and sends through pipes, reading a pipe to its end or its timeout, and pointer and keyboard events reaching the window of a display whose surface the device entered and none after an `enter` on another surface or once the window lets go. Key translation skips without libxkbcommon;
  - `app`, the example program: a window with a heading, a hoverable, pressable and clickable button, a focusable panel that shows the last key pressed, a text field and signal-driven text. Run by hand, it is the interface to try. Run with `--smoke`, it focuses the button and the field through `set_focus`, activates the button with Enter, types into the field, quits and prints the signals' values. The test compares those values. It runs on the host, so it needs a Vulkan device and, on Linux, a compositor. It skips without them. The Windows build is checked to import `vulkan-1.dll`, `user32.dll`, `kernel32.dll` and `dwmapi.dll`;
  - a Wayland window that opens with `hidden`, which `show` leaves as it is, opens again after closing, writes to and reads back the clipboard, and presents frames through `open_for` and `present`, and `wayland_wait`, which opens two windows in one `Display` twice over. It checks that `wait` runs to its timeout with nothing pending. It checks that one wait takes the wakes of `waker_of` that arrived, also from another green thread, and that they give no window a `Woken`. It checks that a wake of the second window's `waker` ends `wait` and gives that window alone a `Woken`. It checks that `next` on the first window runs to its timeout through it. It checks that a wake ends `wait` after a window has closed and with none open. It checks that once the display is released, a window still open in it fails `next` and the display fails `wait_on` and `open_in_on`. `wayland_apptool` opens a second `std/ui` window from a post. It checks that the second window draws a frame, that the main window draws a signal set from a post, and that closing the second window runs its `closed`. These skip without a running compositor, and `wayland_apptool` without a Vulkan 1.3 device;
  - `win32_twopresent` and `wayland_twopresent`, two platform windows drawn by one `Device` through two `Presenter`s with a different scene each. Each window must show its own scene after a glyph atlas is uploaded through either window, after forty presents alternating between the windows and once the first window's `Presenter` is released. The Win32 fixture reads each window's pixels with `PrintWindow`. A Wayland window cannot be read back, so its fixture checks that the presents succeed. `std/ui/gpu/device_test.low` checks the atlas sampling in both directions. It draws into a `Target` of the same `Device` and reads it back. A presenter refused for a surface the device was not opened for is a failure, not a skip. They skip without a Vulkan 1.3 device that presents to the first window, and the Wayland one without a running compositor;
  - on a Windows host, the Win32 platform run with posted and sent messages standing in for input, on windows disabled so the desktop's own pointer does not reach them. Each window is opened with `activate: false` and driven only by messages posted to its handle, never by synthesised input, so they take no focus from the person running them: opening, which checks the window is neither the foreground nor the active window. `win32_hidden` opens a window with `hidden`. It checks with `DwmGetWindowAttribute` that `DWMWA_CLOAKED` reports `DWM_CLOAKED_APP` until `show` and nothing after it. It checks that a second `show` and a `show` on a window opened without `hidden` change nothing. It checks that the client area and the `Resized` event are the requested size while cloaked. `win32_border`, which on Windows 11 or later opens windows with `border: some(0x444444)`, `some(0x445566)` and `none`, makes each topmost without activating it and reads the screen along the edges of its `DWMWA_EXTENDED_FRAME_BOUNDS` with `GetPixel`, because `DwmGetWindowAttribute` has no getter for `DWMWA_BORDER_COLOR`, to check that the frame draws the colour with red and blue in place and that a window with no border colour draws neither. `win32_icon` opens windows with PNG frames made by `std/image`. It reads each icon back with `WM_GETICON` and measures its bitmap with `GetIconInfo` and `GetObjectW`. This checks that the large and the small icon are the frames that cover `SM_CXICON` and `SM_CXSMICON` at the window's scale. It checks that a replacement, a refusal (not a PNG, another size than stated, a PNG cut after its header, a bad frame after a good one) and no frames each leave the icons they should. It checks that the replaced icons and the icons of a closed window are destroyed. `win32_appicon` does the same through `ui.open`, `ui.open_window` and `ui.set_window_icon`, pairs of windows. `win32_classonce` opens windows in one `Display` and closes them in either order. It checks with `GetClassInfoExW` that the window class and the message-only window's class stay registered until the display closes. It checks that a display can open windows after another display or a `platform.open` window has been released. `win32_handles` holds a `DisplayHandle` in a closure and checks that after the display is released `open_in_on` and `wait_on` fail with `Unsupported`. `win32_wait` checks that a message posted to the second window of a display and a wake of `waker_of`, also from another green thread, each end `wait`. It checks that a wake ends `wait` after a window has closed and with none open. It checks that one wait takes all the wakes that arrived. It checks that `wait` runs to its timeout less one clock tick. It checks that key events for two windows keep their text apart. `win32_owned` checks that `GetWindow(GW_OWNER)` returns the owner. It checks that the owner stays enabled. It checks that `WS_EX_APPWINDOW` is set for `taskbar: true` on an owned window and not for a window with no owner, which has neither it nor `WS_EX_TOOLWINDOW`. It checks that `WS_EX_TOOLWINDOW` is set for `taskbar: false` without an owner. It checks that an owner from another `Display` fails with `Unsupported`. It checks that `WM_GETMINMAXINFO` reports the minimum at 100 and 150 percent with the frame from `AdjustWindowRectExForDpi`, and that it answers as `DefWindowProcW` does with no minimum. It checks that a window smaller than its minimum is centred at the minimum. It checks that a centred window lies within a pixel of its owner's centre and inside the work area, and that it is clamped into the work area when the owner sits in a corner. It checks that `set_title` changes the window text. It checks that a window the system destroys with its owner has a cleared handle. Such a window returns one `CloseRequested` from `next` and then fails with `Unsupported`. It reports no `DestroyWindow` failure when released. It has its anchor and drop target pins free, which it shows by filling the runtime's pin table (`pin_capacity`) and releasing the owner or destroying it 40 times. `win32_windowhandles` holds a `WindowHandle` in a closure and checks `detach`, `adopt`, an idempotent `close_on` and `Unsupported` from a stale handle, pointer, keys, resizing, the clipboard, input method composition, cursors, waking, dropped files, which queries the registered `IDropTarget` for its interfaces, calls it with a data object holding `CF_HDROP` that checks the format description it is asked for, with sources that allow one effect, with a point in negative screen coordinates and with a data object holding no files, and checks that the target is gone once the window closes, and the title bar: `win32_titlebar` sets a hit test on a platform window. It checks that the client area is the window less the frame insets and the requested size. It checks the hit codes of the areas, the eight borders in the invisible frame and the top edge, a close click, a press and release on different buttons, a press released over the content followed by an unpaired non-client release, and double clicks on a caption button and on the drag area. It checks `ShowWindow` maximising and restoring with the client area kept inside the monitor's work area, `toggle_maximize`, the minimise button, and minimising a maximised window, which sends no `Maximized(false)`. `win32_titled` opens a window with the system title bar. It checks that its hit test falls through to the system except where an area answers, that a press and release on an area-answered close code closes it, and that `WM_NCMOUSELEAVE` gives `PointerLeft`. `win32_apptitlebar` opens a `std/ui` window, disabled and found by a title that holds the process id, with the title bar hidden. It checks that its areas answer through `WM_NCHITTEST`, that a click on the maximise area maximises it, and that `toggle_maximize` restores it. The `Maximized` event is shown by the first and last. `win32_appreveal` opens a `std/ui` window and checks that `open` leaves it cloaked and that it is uncloaked after the first frame is presented. It checks that a window of no area, which never has a frame to present, is still cloaked 200 milliseconds after `run` starts and uncloaked a second after. The title bar cases need a desktop. `win32_apptitlebar` and `win32_appreveal` also need a Vulkan 1.3 device.
  - The clipboard program saves every format on the clipboard and puts them back when it ends. It skips when one is a graphics handle it cannot copy. The input method program composes through `ImmSetCompositionStringW` and sends the `WM_IME_*` messages itself when the host's input method sends none. The cursor program compares `GetCursor` with the stock cursor after `set_cursor` and after a sent `WM_SETCURSOR`.
- `compiler/tests/vkgen` and `compiler/tests/wlgen` check that the checked-in bindings match their generators. Both find their libraries through `link.system_dirs`.
  - `compiler/tests/wlgen` tests the generated signatures and checks, then builds a registry client from the vendored protocols and, when a Wayland compositor is running, runs it.
  - `compiler/tests/vkgen` tests declaration parsing, extension enum values and the selection format, and checks the shape of the generated bindings. It renders a rounded, bordered quad offscreen with the quad shaders through the bindings and dynamic rendering, comparing pixels. It skips without libvulkan or a Vulkan 1.3 device.
- `compiler/tests/shaders` checks that the sources and the SPIR-V match the sums recorded in `shaders.sum` and validates the SPIR-V with `spirv-val` when installed.

## Tests

The tests are Low programs. `compiler/lowtest` is the fixture harness. Build it with `low build compiler/lowtest -o lowtest`. Then run `lowtest <low binary> [run|examples|fail|lint|test|windows|browser]... [-filter text]` from the repository root. It checks any `low` binary against the sidecar files using only its command line. Each `compiler/tests/<name>` is a `std/testing` program for `compiler/<name>`, run with `low run compiler/tests/<name>`. `low` tests `compiler/low`, `editors` tests the editor grammars and `ui` tests the `std/ui` modules. A test that needs a tool outside `low` skips when the tool is missing. The `run` and `examples` suites skip on a Windows host. The `windows` suite skips on a Linux host. So the workflow runs the `windows` and `test` suites, `low test std/...` and the Win32 tests on a Windows runner. That runner uses a `low.exe` and `lowtest.exe` built for Windows. The `windows` suite prints the processor and the enabled XState feature mask of its host. It runs a program whose expected stderr comes back empty once more, and the report holds the exit status of both runs. So a lost panic message on a runner can be told from a host that always loses it. The `compiler/tests/asm` and `compiler/tests/link` oracle tests need the pinned binutils and LLVM below. The workflow does not install them, so these tests run by hand.

### Fixtures

A fixture program is a `.low` file or a directory with a `main.low`. Its expectations sit beside it in sidecar files named after it: `name.low` and `name/` both take `name.out`. `compiler/fixture` reads and writes the sidecars. `compiler/fuzz/corpus` writes fixtures through it, and `compiler/lowtest` lists programs for the harness.

- `testdata/run` holds programs that run on Linux:
  - `.out` is the expected stdout, empty when absent.
  - `.err` is the expected stderr. A program has one exactly when it exits with a panic status. A `fault:` line is left out of the comparison and printed with the failure when the program fails.
  - `.maxrss` is a limit on peak memory in KB.
  - `.maxtime` is a limit on the wall-clock time of one run, as a duration such as `1s`. Only programs whose point is speed have one. A program with a `.maxtime` or `.maxrss` runs on its own, not alongside the others.
  - `.needed` lists the shared libraries the binary must link, one per line in sorted order. Without it the binary must be static with no interpreter. A program with one has no Windows build.
  - `.tags` declares what the runner cannot learn from the program. `simd` marks a program that compiles to vector code and runs a second time with `LOW_SIMD=sse2`, and a third time with `LOW_SIMD=avx512` on a host whose `/proc/cpuinfo` lists `avx512f`, `avx512vl`, `avx512bw` and `avx512dq` (the `run` suite prints whether it does). `linux-only` marks a program that uses operating system services the Windows standard library lacks, such as signals, pseudo-terminals, inotify, Unix sockets, file modes or `/proc`. It marks a program that relies on a Linux kernel guarantee, such as appends from concurrent threads landing whole. Such a program has no Windows build. `debug` builds the program with `-c debug`. The runner builds each program with `--map` and fails when `simd` and the build it runs disagree: a program has vector code when the map lists a function's `.avx512`, `.avx2` and `.sse2` versions beside its stub.
- `testdata/windows` holds programs that only run on Windows, with `.out`, `.err` and `.maxtime`.
- `std/<module>/examples` holds the module's examples, with `.out`. An example with no Windows build has a `.tags` of `linux-only`. The runner does not check `simd` for them.
- `testdata/fail` holds programs that must fail to compile with the errors in `.err`, one per line, each ending with its diagnostic code in brackets. `two_independent_errors` reports two errors from one file, and `root_error_no_cascade` one error for a root mistake its later statements use. File paths in the message are shown relative to the fixture. `.target` names the target to build for, such as `browser/wasm32`. `fixture.read_target` reads it. Without it the program builds for `linux/amd64`.
- `testdata/lint` holds programs and library modules without a `main`. The `low lint` warnings of each must equal `.warn`, one per line with relative paths. A program without a `.warn` file has none.
- `testdata/test` holds directories whose tests run with `low test`:
  - `.module` names the path to test, joined to the directory's path from the repository root. The path is a submodule of a project, `...` for every module below the directory, or `name_test.low:LINE`. Without it the directory itself is tested.
  - `.args` lists extra arguments for `low test`, one per line, after the path and `--notime`.
  - `.out` and `.err` are the expected stdout and stderr, empty when absent, with the absolute path of the directory removed. Module names are paths from the repository root. The repository root is the working directory of `lowtest`.
  - `.status` is the expected exit status, `0` when absent.
- `bench/<name>/main.low` are the benchmark programs. The compile benchmark and the determinism test use them.

### Checks

- The `run` and `examples` suites compile every `testdata/run` program and std example for Linux. They run each one with a one-minute limit. They compare stdout with `.out` and stderr with `.err`. They apply `.maxrss`, `.maxtime`, `.needed` and `.tags`. These suites skip on a Windows host.
- The `windows` suite builds every `testdata/run` program and std example that has a Windows build, and every `testdata/windows` program, for Windows. It compares stdout, stderr and `.maxtime` the same way. It runs a program with a `.maxtime` or `.maxrss` on its own. It runs only on a Windows host.
- The `fail` and `lint` suites check every `testdata/fail` and `testdata/lint` program against its sidecar.
- The `test` suite runs `low test --notime --nocache` on every `testdata/test` directory. It compares stdout, stderr and the exit status with the directory's sidecars. It runs on both hosts. A test build cannot be expressed in a `testdata/fail` program, so the rejected test shapes are fixtures here. The fixtures also cover these cases: a crashing test, a test that times out, skipped tests, `--run` on a subtest, `--skip`, `--failfast`, `--count`, `--shuffle`, `--json` on a passing module and on one that fails to build, `--list`, selection by `file:line`, a test file for each operating system that defines the same test, a `...` walk that leaves out `testdata`, `examples`, dot and underscore directories while another module fails to build, a module with no test files and a test that reads a file beside its module. `testdata/run/test_files_left_out` holds a test file that does not compile, which shows that `low build` and `low run` leave test files out.
- `low test std/...` runs the unit tests in the `_test.low` files of the standard library modules, built against the standard library embedded in `low`. Each file tests one module through its private code, so it uses the module's names without an import. Each file covers one area of its module:
  - `std/testing`: `selected` for each `/` level of a filter, `settings_from` for every flag and each flag error, the failure message of each assertion that returns, and `sub`, `table` and `table_of` against a `T` the test builds.
  - `std/path`: `clean`, `join`, `base`, `dir`, `ext`, `stem`, `is_absolute` and `matches` over empty paths, `.`, `..`, repeated and trailing separators and the root.
  - `std/filepath`: every function against `std/path` on paths without a volume, with `/` turned into the target's separator, and on Windows `volume`, `is_absolute`, `clean`, `join`, `base` and `dir` over drives, drive-relative paths, shares and device paths.
  - `std/text`: splitting, `cut`, trimming, searching, `replace`, integer parsing and formatting in bases 2 to 36, `quote` and `unquote`, `fixed`, padding, `position`, `wrap`, `append`, `append_char`, `char_len`, `title`, `edit_distance`, `has_token` and `lower`, including empty input, multi-byte text and bytes that are not UTF-8.
  - `std/crypto/mldsa`: the number-theoretic transform against schoolbook multiplication, the rounding and hint functions against their definitions, and hint decoding for a well-formed encoding and for each malformed one (count above the limit, decreasing or repeated indices, decreasing counts, nonzero padding).
  - `std/crypto/slhdsa`: `base_2b` over nibbles, twelve-bit groups and groups that cross bytes, the WOTS+ checksum digits, the compressed address layout, `chain` composition, the WOTS+, XMSS and FORS signatures recovering their public keys, the hypertree rejecting a changed message, leaf or tree index, and the signature size of every parameter set.
  - `std/crypto/falcon`: the negacyclic transform against schoolbook multiplication for degrees 2 to 1024, the compressed signature and 14-bit public key codecs against an encoder written in the test, each non-canonical encoding (`-0`, a coefficient beyond 2047, trailing bits, a truncated or short buffer, a public key coefficient of 12289 or more, a wrong length), the trimmed private key coefficients and their excluded lowest value, the point hash and the strict norm bound.
  - `std/zk/groth16`: the testdata fixture `std_zk_groth16` verifies, for BN254 and BLS12-381 and for a circuit with one public signal and one with three public signals and a bit decomposition, the `proof.json`, `verification_key.json` and `public.json` that snarkjs 0.7.6 made with circom 2.2.3 from the two `.circom` sources beside the vectors, each with one contribution to a throwaway powers-of-tau and phase 2 ceremony. The fixture reads the circom `.r1cs` and `.wtns` of the same circuits, proves them with a `setup` of its own and verifies the result. It lists the rejections: a changed public input, a changed `C`, a point off the curve, the group order as an input, a file or JSON for the other curve and every proper prefix of a file. `snarkjs groth16 verify` accepts the proofs and keys that `setup` and `prove` make for the same circuits. `snarkjs wtns check` accepts the files `write_r1cs` and `write_witness` write. That run needs snarkjs and is not part of the suite. The unit tests cover the refusals of `prove` for an assignment of the wrong length and for a key made for another system.
  - `std/zk/goldilocks`, `std/zk/stark` and `std/zk/airs`: the fixture `std_zk_stark` compares the field, the extension and the transforms with Winterfell 0.13.1 (`winter-math`, `winter-fri` and `winter-air`), whose Rust program and output are beside it in `winterfell/`. The comparison covers 100 sums, differences and products and 9 inverses over edge values, seven powers of the generator, the 32 roots of unity of order 2 to `2^32`, 36 extension products, 6 extension inverses and Frobenius images, a 32-point transform, a 128-point coset evaluation with blowup 4 and an extension coset evaluation with blowup 8, and six `security` results. The unit test `fold_matches_winterfell` compares one FRI fold of 32 extension values at the offset 7 with `apply_drp`. The fixture also proves and verifies the two example computations with and without `zero_knowledge`. For each of them it rejects a wrong public input and a changed trace value, trace Merkle path and root, composition value, path and root, out-of-domain value, FRI layer value, path and root, remainder and nonce. It reads every proper prefix of a proof as `Malformed`, and a proof with one more byte. It flips one bit of every byte of a proof and accepts none. It rejects a proof, and reads its bytes as `Mismatch`, under another `blowup`, `queries`, `grinding_bits` or `zero_knowledge` and for a trace of another length. The unit tests check that a prover is rejected when it skips the satisfiability check and proves a trace that breaks a constraint. They check that a number at or above the modulus is `Malformed`. They check that `zero_knowledge` proofs of one statement have different roots and the others the same. They check the Merkle batched openings. The `std/zk/goldilocks` tests cover the batch inverses. `cargo run --release > vectors.txt` in `winterfell/` regenerates the Winterfell values.
  - `std/zk/halo2`, `std/crypto/pasta` and `std/crypto/blake2b`: the fixture `std_zk_halo2` compares them with `halo2_proofs` 0.3.0, `pasta_curves` and `blake2b_simd`, whose Rust program, vectors and output are beside it in `halo2/`. It checks field elements, sums, differences, products, negations, squares, inverses, square roots, powers, 64-byte reductions, non-canonical encodings, the root of unity and `delta` of both Pasta fields. It checks the generator, multiples, sums, doubles, negations, encodings that decode and ones that do not, coordinates and the hash to curve for the `Halo2-Parameters` domain and another of both curves. It checks 27 Blake2b-512 digests with three personalisations and the 32-byte digest of `abc`. It checks the cube root of unity of both curves. It checks the pinned description, the fixed commitments and the permutation commitments of a circuit with three advice columns, four fixed columns, an instance column, two gates, a lookup and four equality constraints (`k = 4`, degree 5). It accepts the proof `halo2_proofs` made for that circuit. A second circuit is written the way `halo2_proofs` circuits are. It is a chip with five advice columns, a constants column, a lookup table column loaded with `assign_table`, a multiplication, an addition and a squaring gate behind simple selectors and a range lookup behind a complex selector. It is laid out in four regions, two of which take a constant, with the results tied to two public inputs (`k = 4`, degree 5). Its pinned description shows the multiplication and addition selectors sharing a column. The fixture checks its pinned description, fixed commitments and permutation commitments. It accepts the proof `halo2_proofs` made for two circuits with different witnesses and public inputs. It proves the two circuits. It refuses a witness outside the table and a public input that differs in one circuit. It rejects the public inputs swapped, one or three instance sets for two proofs, a changed public input, proper prefixes, a trailing byte and a bit flipped in each 32-byte word of one proof or the other. `prove` with a seeded random function makes the proofs `halo2/low_proof.hex` and `halo2/low_regions_proof.hex`. Running the Rust program (`cargo run` in `halo2/`) verifies both with `halo2_proofs` and fails if either is rejected. The lines `low_proof_verdict` and `regions_low_proof_verdict` in `halo2/vectors.txt` record the verdict with the Blake2b-256 digest of the proof judged, which the fixture compares with its own proofs. The fixture also rejects, for the proofs of both programs, each commitment, each evaluation, each opening evaluation, the quotient commitment, the inner product commitment and value, and both points of every inner product round swapped for another valid value; a changed public input, no instance column, too many public inputs and another number of columns; the proper prefixes that end at the start of each 32-byte element, one byte into it and one byte before its end; a trailing byte; and a bit flipped in the first and the last byte of each element of one proof and in an inner byte of each element of the other. The unit tests cover the transcript against the reference, the transforms of the evaluation domain, the parameter file, the queries of a constraint system, keygen refusals, including a simple selector in a sum, beside another or in a lookup, the placement of regions, constants and tables, the combination of selectors, and small circuits. These circuits prove and verify, one of them twice in one proof. Their prover refuses a witness that breaks a gate, an equality or an instance value.
  - `std/crypto/fhe/ring`: Barrett reduction of 128-bit inputs, products, Shoup products and inverses against `std/big` for 61-, 60-, 40-, 17- and 1-bit moduli and edge inputs; Miller-Rabin against `std/big` on every integer below 3000, strong pseudoprimes and Carmichael numbers; the shape of `ntt_primes`; the transform root against the smallest primitive root found by search; the transform against schoolbook negacyclic multiplication for degrees 2 to 1024; exact basis extension, `round(t * x / D)` for decryption and for the scaling step of the product, and division by the last prime on coefficients and on transforms, each against `std/big` on random values and on 0, 1, -1 and values near `Q/2`; automorphisms preserving products, composing and agreeing between coefficients and transforms; the seeded stream against the ChaCha20 block function; the frequencies of ternary coefficients and the mean, variance and cut of the Gaussians; `turn` at exact angles and under angle addition; and the logarithm against `exp`.
  - `std/crypto/fhe/rlwe`: the primes `chain_primes` picks against the primes SEAL 4.1 picks for three parameter sets, key switching at every level giving a phase within 12 bits of `d * s'` under the target key and not under another, and the phase of a public encryption of zero within 12 bits.
  - `std/crypto/fhe/bfv`: batching against SHA-256 digests of the plaintexts SEAL 4.1's `BatchEncoder` makes for three parameter sets. Slot-wise `add`, `sub`, `negate`, `add_plain`, `multiply_plain`, `multiply`, `multiply_relinearize`, row rotations both ways and the row swap on random values. At the default parameters, the fresh budget is at least the bound `log2(Q) - log2(t * (N + 4) / 2) - 1`, each product costs at most `log2(t) + log2(N) + 2` bits, each budget is within 2 bits of SEAL's and exactly five squarings decrypt. Switching down to every level keeps the plaintext. A wrong secret key, a tampered ciphertext and a product relinearized with another key's relinearization key leave no budget. A missing Galois key fails. Ciphertexts at different levels refuse to add. Parameters above the standard's bound, of an unlisted degree, with a special prime smaller than a ciphertext prime or a plaintext modulus too large are rejected, and encoding is refused without batching.
  - `std/crypto/fhe/ckks`: the encoder against the plaintext SEAL 4.1's `CKKSEncoder` makes at degree 16 and against the SHA-256 digest of its plaintext at degree 8192, and the primes against SEAL's. At the default parameters, the precision of a fresh ciphertext, a rescaled square and a fourth power is each above a bound of 12 times the error derived from the parameters and within 1 bit of SEAL's. At the default parameters, the precision of rotations both ways and conjugation is above the bound with the key-switching error and 3 bits above SEAL's. SEAL's figures are means over repeated runs. `add`, `sub`, `negate`, `add_plain`, rotation and a rescaled `multiply_plain` at every level after `drop_to`. Three-part products that decrypt, add and relinearize. A wrong secret key and another key's relinearization key leave less than 1 bit. Each error variant from the operation or parameters that cause it.
  - `std/crypto/fhe/tfhe`: the default parameters against the reference library's set and each `Error` variant from the parameters that cause it. Torus products against the schoolbook product at degrees 2 to 1024 with extreme coefficients. Gadget digits are balanced and sum to the input rounded to the nearest multiple of the last digit. The measured deviation of the noise sampler at `2^-15` and `2^-25`. The noise of fresh ciphertexts, of the external product by a key bit of 0 and of 1, of the key switch and of a blind rotation is each within 10 to 25 percent of the variance the formulas predict. Decryption under a wrong secret key is right for about half the bits. Every gate on every input, on fresh ciphertexts and on ciphertexts of an earlier gate, with the phase before bootstrapping and the output phase at least 1/16 of a turn from the decision boundary, and `mux` for both selectors. Gates under another key's cloud key give both right and wrong bits. A 4-bit adder built from the gates sums 15 and 1 and 11 and 7.
- The `browser` suite builds every `testdata/run` program and std example for `browser/wasm32`. It runs each one with `low run --target browser/wasm32` and compares stdout, stderr and the exit status with the same sidecars. It ignores compiler warnings on stderr. It does not apply `.maxtime`, `.needed` and `.tags`. `.maxrss` becomes `LOW_WASM_MAX_PAGES`, the limit divided by 64 KB. This does not hold for the programs `fixture.browser_page_environment` names (`heap_lazy_zero`, `region_reuse`, `rt_heap_release` and `ssa_fill_memory`). Their native peak stays below the size of their data because pages never touched are never committed, while the browser runtime zeroes every page it hands out. The suite skips entirely when `node` is not on `PATH`. With `LOW_GC=stress` in its environment, the suite runs the programs under the stress option of `### WebAssembly`: it builds each module with `LOW_GC` removed, because the compiler's own collector would also stress, and runs it with `node` and the `host.mjs` embedded in the harness. Its summary line counts three groups apart from the passes and failures:
  - Skipped: programs the loader rejects with a `browser/wasm32 has no` message because they reach `std/sys` or a C library, and `region_freed`, whose use after a region faults only where a region unmaps its memory.
  - Unsupported by the back end: programs whose build fails with a `browser/wasm32:` message, such as `wait_readable` and `wait_writable`.
  - Any other build failure is a failure.
- The time zone programs read zone rules from the embedded `std/time/tzdata`, because host databases differ by release and by build. Debian's includes `backzone`. The programs read the host database only for the behaviour of `zone.named` and `zone.local`, through zones whose rules every build agrees on.
- The `std/cdp` programs print `launch failed` when no browser starts.
- `compiler/tests/low` compiles each program twice in one process and requires identical SSA form and assembly. It does this for every `testdata/run` program for Linux release, every benchmark for Linux release, Linux dev and Windows, and every `testdata/windows` program for Windows. It does the same for the `browser/wasm32` module. It compiles every `testdata/run` program and std example the browser target builds, and `compiler/playground`, twice for the release profile. They must give identical module bytes. A program the loader or the back end rejects for the browser target is skipped. It builds `ptr_unused_null_load` and `overflow`. It requires the first to print one `fault:` line with a zero address, a non-zero instruction address and a stack pointer inside the green thread's stack before `panic: invalid memory access`. It requires the second to print only `panic: stack overflow`. It also builds a program with `low build` and installs it with `low install`, each from inside the program's directory and by its path. It runs the results and checks the path each install prints. In projects of its own it checks that a `gui` declaration gives a program the GUI subsystem under `low build`, `low run` and `low install`, and a sibling `program` the console subsystem, that `--subsystem` overrides the declaration, that the `icon`, `manifest`, `version` and `file_info` declarations give every program of the project, console or windowed, the resources they name under `low build` and `low install`, and give a program of another project none, that `low test` builds a console harness, that `low build`, `low run` and `low install` with no program take the declared ones, that a project with a modules directory builds, installs and tests by import path from the root and from inside the directory, and that a dependency's `program` and `gui` change nothing in the dependent. The tests that run `low` share one `low` that the first of them builds. One runs `low test` on a small project in a temporary directory with its own `LOW_CACHE`. It shows that passing tests are replayed until the file, the environment variable or the directory listing they read changes. It shows that a failing test and a test that starts a process are never replayed. It shows that `--nocache` replays nothing. It publishes a package from a local `git` repository to an index with `low publish`. It checks that a second publish is refused. It checks that `low build` of a project that depends on it prints nothing, warns once after `low yank` and is quiet again after `low yank --undo`.
- `compiler/tests/ir` requires `ir.is_reducible` of every function's control-flow graph after optimisation, with the release profile. It does this over `compiler/low`, every benchmark and every `testdata/run` program. It names each irreducible function by program. It also checks that predicate on small graphs with one and with two entries into a cycle. It checks that a callee over the size limit is inlined when its call is its only reference and kept when it has two.
- `compiler/tests/compiler` compiles inline programs, benchmarks and fixtures. It checks what the optimiser and code generator did with them: inlining, specialisation, bounds checks, register use, string comparison, switch lowering and allocation. It also checks the project root, the `build.low` reader, where imports and tested modules are placed under a modules directory, and version ranges. The reader check includes the `program`, `gui` and `modules` declarations.
- `compiler/tests/fetch` publishes tagged versions to local `git` repositories, served in place of `https://` through `GIT_CONFIG_*`. It checks that resolution picks the newest accepted tag, keeps a locked version, moves with `update` and names every range when none is accepted. With a package index checkout it checks that `publish` records the tag, commit and hash, refuses a listed version, a tree that does not build, an unpushed tag and a declared version that differs, that `yank` and its undo change only the flag, that resolution takes versions and hashes from the index, skips yanked versions, names them when only they match, builds a locked yanked version with a warning, and fails on an index hash that differs from the fetched tree or from `build.lock`. With versions published it checks that `low doc index` lists every version newest first with the yanked ones marked, names modules by their import path, leaves out `testdata`, `_` directories and the files at the top of the index, honours the `modules` directory of `build.low`, and lists nothing for an unset, absent or empty index. It skips without `git`.
- `compiler/tests/check` requires each public builtin in `compiler/check/builtins.low` to have an entry in `## Builtins` with the same name, parameters and `mut` markers, and each entry to name a builtin.
- `compiler/tests/lsp` starts the server over its protocol and checks diagnostics, hover, go-to-definition, references, rename, completion, signature help, inlay hints and symbols. One test opens each `testdata/run` fixture of at most 5,000 bytes in turn, requests hover and definition at every word, and closes it before opening the next. Further tests open a module's test file. They check that it answers every request through the test build of its module, that edits to the module's other files reach it, and that no answer names a generated file.
- `low test site/gen` renders the pages of indexed packages from modules written in the test, with no network. It checks that every version and module has a page. It checks that the landing page marks yanked versions and names the newest version that is not yanked as the default. It checks that signatures link the version's other modules and the standard library. It checks that search lists the package and the modules of its default version only. It checks that the packages index lists packages only when the index has some.
- `compiler/tests/editors` compares the keyword, contextual keyword, builtin function, type, constant and constraint lists of the editor grammars and `site/gen/highlight.low` with the lexer and the checker. It requires the copied editor files to match their sources. It requires the Zed `rev` to name a commit whose `editors/tree-sitter-low/src` equals the current one. The rev check skips outside a git checkout.
- The UI tests are listed under UI tests.
- `compiler/tests/hot` tests reload against running programs on Linux and on Windows hosts:
  - state kept across a code swap;
  - functions added;
  - compile errors leaving the program running;
  - heap data surviving a reload while the collector runs;
  - stored function values, closures, callbacks and spawned threads running patched code;
  - a program with more functions than the smallest `fn_table`;
  - instances of an edited generic function patched;
  - restarts on layout, signature and capture changes.
- `compiler/lowfuzz` generates well-typed, terminating programs from a choice stream (`compiler/fuzz/gen`). It predicts their output with a reference interpreter (`compiler/fuzz/model`). It builds each program for the host with the release profile and with the dev profile. The profiles give the same optimisation and hot-reload code generation as `low build` and `low dev`. It runs both builds and reports any of these (`compiler/fuzz/oracle`):
  - compiler panic;
  - rejected program;
  - link failure;
  - timeout;
  - release/dev difference;
  - difference from the prediction;
  - browser difference: when `node` is on `PATH` and `-browser` is not false, the oracle also builds each case for `browser/wasm32` through the same hidden `__build` command. It runs the module under `node` with the `host.mjs` that `wasm.host_script` returns. It compares the module's stdout and exit status with the native release run. The browser side is skipped, with one notice, when `node` is missing.

  The generator covers every integer width, `f32` and `f64`, shifts, conversions, `if` expressions, functions, and `for` loops with labelled `break` and `continue`. Float values are observed through clamped integer conversions and NaN comparisons.
- `compiler/fuzz/reduce` minimises each new signature. It removes functions, parameters and statements, unwraps `if` and loop bodies, replaces expressions with literals or their operands, and shrinks literals, keeping only candidates that still produce the signature. `-reduce-tests` bounds the compiler runs.
  - The minimised program is saved under `-out`, one per distinct signature.
  - With `-fixtures` it is also written there as a `testdata/run` fixture. The fixture is a `.low` with the predicted `.out` and `.err`, named by content hash. It fails until the bug is fixed.
- `compiler/tests/asm` assembles programs with `as` or `clang` and requires the same object bytes. `programs/evex.s` holds every EVEX and opmask form with high registers, masks, broadcasts and displacements either side of the tuple-size boundaries. `evex_encodings` and `evex_errors` check bytes and diagnostics without `as`. Every program carries the same runtime assembly, so the corpus is every `testdata/windows` program, two benchmarks and the `testdata/run` programs that together contain the most distinct instruction forms (mnemonic, operand classes, addressing forms, directives), each for Linux and Windows. `LOW_ASM_FULL=1` covers every `testdata/run` and `testdata/windows` program and every benchmark. The same program also links every corpus program, `compiler/low` and the programs that link C libraries through the text path and through the chunk path for Linux and Windows. It requires identical images. The corpus tests report in order while their work runs ahead. When one starts, the jobs of as many following tests as a quarter of the CPU count start too. `compiler/tests/link` links programs with `ld` or `lld-link` and requires the same segments, sections, symbols and loaded bytes for ELF and the same bytes for PE. `LOW_LINK_FULL=1` covers every `testdata/run` program and benchmark. It also reads the `.rsrc` section of PE images back: the resource tree and the icon, version and manifest resources in it, the version block against the bytes `llvm-rc` writes for the same strings, the numbers of the fixed version and the resources `pe` rejects.
  - The padding and relaxation choices these tests compare differ between tool releases, so `compiler/oracletool` pins GNU binutils 2.47 and LLVM 23.1.4. The tests skip when a tool is missing or reports another version.
