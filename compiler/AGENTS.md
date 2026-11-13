# Low compiler in Low

`compiler/` is the Low port of the Go compiler in `internal/` and `cmd/`. [docs/self-hosting/plan.md](../docs/self-hosting/plan.md) orders the work. The Go compiler is the oracle: a ported stage is done when its output equals the Go stage's output byte for byte.

## Layout

- `compiler/low.mod` marks the project root, so `import lexer` loads `compiler/lexer/`.
- One Go package `internal/<name>` becomes one module `compiler/<name>/`, keeping the Go file names with a `.low` extension.
- `compiler/lowdump/` and `cmd/lowdump/` are the same dump tool in Low and in Go. Each stage adds one command to both, in its own file, printing that stage's output as text or writing it as bytes.
- `compiler/low/` is the `low` command: `build`, `run`, `asm`, `ir`, `lint`, `doc`, `dev`, `lsp` and `version`. `version` prints `compiler/low/version.txt`. `dev` runs `compiler/hot` and `lsp` runs `compiler/lsp`; `compiler/compiler/overlay.low` holds the language server's unsaved text and is passed down as state.
- `compiler/parallel/` runs a job over indices on one green thread per CPU. `ir/reach.low` builds and finalises the reachable functions with it and `amd64/codegen.low` generates each function with it; both merge the results in the sequential order (the discovery order for the hoisted-literal cells, the function order for assembly, data labels in first-seen order and stack maps), so the output does not depend on scheduling. `compiler.build` passes `release` to `amd64.generate`, which drops each function's SSA once its assembly exists, so a unit cannot be generated twice after a build. `low` pins a 128 MB untouched byte buffer for the length of a `build`, `run`, `asm` or `ir` command: the collector runs when the allocation since the last collection exceeds the live words, so the buffer spaces collections out without adding resident memory.
- `compiler/lowfuzz/`, `compiler/tablegen/`, `compiler/vkgen/`, `compiler/wlgen/`, `compiler/shadergen/` and `compiler/bench/` are the Low tools of the same names in `cmd/`. `lowfuzz` builds each case in a child process through its hidden `__build` command. `bench` compares `low` with `bun`.
- `compiler/lowtest/` is the fixture harness. `/tmp/low build compiler/lowtest -o /tmp/lowtest`, then `/tmp/lowtest <low binary> [run|examples|fail|lint|windows]... [-filter text]` from the repository root checks any `low` binary against the sidecar files, using only its command line. `compiler/peakrss` and `lowtest` sample `.maxrss` from `/proc` every millisecond.
- `compiler/tests/<name>/` holds the `std/testing` program for the Go tests of `internal/<name>` (`low` for `cmd/low`, `ui` for `internal/uitest`). Run one with `/tmp/low run compiler/tests/<name>` from the repository root. The `asm` and `link` oracle tests skip unless the binutils and LLVM versions `compiler/oracletool` pins are installed.

## Build and compare

```
$ go build -o /tmp/low ./cmd/low
$ go build -o /tmp/lowdump-go ./cmd/lowdump
$ /tmp/low build compiler/lowdump -o /tmp/lowdump-low
$ /tmp/lowdump-go tokens file.low > a; /tmp/lowdump-low tokens file.low > b; cmp a b
```

The commands are `tokens`, `ast`, `check`, `ir`, `asm`, `object`, `link`, `patch` and `regex`; run either tool without arguments for their flags. `ir` prints the SSA of every function, `asm` the assembly of the whole program, `object` the assembled sections, symbols and relocations of a `.s` file, `link` writes the executable for a `.s` file, `patch` prints the hot patch for a `.s` file against a base executable, and `regex` prints the matcher generated for each pattern in a file. `check` loads a whole program and prints every file's AST with the types, locals, targets and layouts the checker resolved; `cmd/lowdump/ast.go` and `compiler/lowdump/ast.low` print the same lines, so a field the checker sets is compared as soon as the Low checker sets it. Both compilers embed `internal/compiler/std` and `internal/compiler/prelude.low`: the Low `compiler` module calls `embed_dir` on the std tree once per compilation, holds the map in the `Std` value it passes to the loader and the embedder, and takes the prelude with `embed`.

Compare stdout, stderr and the exit status over every `.low` file under `testdata/`, `internal/compiler/` and `bench/`, including the files that must fail.

The Low `low` builds itself: `/tmp/low build compiler/low -o /tmp/low1`, then `/tmp/low1 build compiler/low -o /tmp/low2` and `/tmp/low2 build compiler/low -o /tmp/low3` give three byte-identical executables, and each builds every program to the same bytes as the Go `low`.

## Porting rules

- Keep the Go structure: the same functions in the same order with the same names in `snake_case`, so a later change to the Go source maps onto the Low source line by line.
- Low has no module-level variables. A Go table becomes a function that returns it, or a `match`.
- Low has no methods or overloading and one function namespace per module. `l.peek(0)` calls `peek(l, 0)`. Two Go methods with one name on different receivers need different names.
- State that Go mutates through a pointer receiver is a struct built with `new` and passed as `*T`.
- An error is the enum `diag.Error`. A Go function that returns `error` returns `! diag.Error`, and the callers use `?`. Go code that panics with `diag.Fail` and recovers with `diag.Try` fails with `diag.Error` and propagates the same way.
- A Go interface with a type switch over its implementations is an enum with one variant per implementation. A Go interface called through its methods is a struct of function values.
- A Go `map` keyed by a struct is keyed by a `str` or integer built from the same fields.
- Source text is a `str`. `s[a..b]` shares the bytes, so token and name text is sliced, never copied.
- Output must not depend on map hash order. Low maps iterate in insertion order, which is deterministic; where Go sorts keys, sort the same way.
- A Go nil pointer is `*T?`, and a nil interface value is `Expr?`. `[]*T?` is an optional slice, not a slice of optionals: parallel Go slices with nil holes become one slice of a struct with optional fields, as `ast.InterpPart` does.
- Low has no module-level state, so the Go package-level type singletons live on `types.Universe`, which every stage receives. Compare them by kind (`is_void`, `is_never`) or through the universe (`u.i64`).
- A struct copy is `new(*p)`. `ast/clone.low` copies each node that way and then replaces its children.
- A local cannot share the name of a function it calls, and a function cannot be named after a keyword or a type.
- An enum converts to an integer only through `.index`, which needs `= n` on every variant.
- `diag.quote` and `diag.quote_char` format as Go's `%q`. `lexer.quote` is the dump format.
- The Low compiler runs on a 1 MB green thread stack, so no pass recurses once per link of a left-deep binary chain or an `else if` chain. `ast.left_spine` and `ast.else_chain` list the links outermost first; a pass visits them in a loop with open and close steps that keep the Go order of work (`ir.binary`, `ir.if_expr`, `check.binary`, `check.if_expr`), and the walkers, printers and clones iterate the same lists. The same holds for the SSA reader (`ir.read_recursive`) and the optimiser's dominator walk (`ir.visit`), which keep their own frames. `testdata/run/long_chains.low` and `testdata/fail/long_chain_error.low` hold chains of 20,000 terms.
- Assignments are statements: a `match` arm that assigns takes a `{ }` block.
- `"{{"` and `"}}"` are literal braces in a string.
