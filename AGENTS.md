# Low

Compiler for the Low language, written in Low. [docs/design.md](docs/design.md) is the language and compiler reference; each branch below names the section to read first.

## Layout

- `compiler/` is the compiler. `compiler/low.mod` marks the project root, so `import lexer` loads `compiler/lexer/`. One directory is one module.
- `std/` is the standard library, one directory per module, embedded in `low`. `compiler/compiler/prelude/prelude.low` is the runtime and generic map code compiled ahead of every program, `compiler/compiler/prelude/browser.low` is the allocator, collector and lock routines written in Low and joined to every program for a target without an operating system, and `compiler/amd64/runtime/*.asm` is the runtime assembly.
- `testdata/` holds the fixture programs, `bench/` the benchmark programs, `docs/` the design reference and examples, `editors/` the editor grammars and `site/` the website.
- `bootstrap.txt` names the release that builds the compiler and the SHA-256 of its Linux binary. `bootstrap.sh` downloads it, builds the compiler three times and checks the last two builds are byte-identical.

### Compiler modules

- `lexer`, `parser` and `ast` turn source into a tree; `check`, `types`, `builtin` and `prelude` attach types to it; `ir` builds and optimises SSA; `amd64` emits assembly; `asm` assembles it and `link` writes the ELF or PE executable, all in process. `wasm` writes the WebAssembly module for `browser/wasm32` straight from the SSA, and `wasm/host.mjs` runs it. `target` names the targets and `diag` holds the error enum and its formatting.
- `compiler/compiler` loads a program and its imports, embeds `std/` once per compilation and drives the front end; it and `ir` import no operating system module (`compiler/tests/compiler` checks the import closure). The file access (`compiler.Disk`, held by `compiler/compiler/overlay.low` with the language server's unsaved text) and the job runner (`ir.Runner`) are passed in as state. `compiler/native/` is the native driver: it implements the `Disk` over `std/file`, the runner over `parallel`, and generates, assembles and links with `amd64`, `asm` and `link`, so roots call `native.analyse`, `native.build` and `native.overlay`, and `compiler/lsp` takes its `Overlay` from its root. `compiler/low/` is the `low` command: `build`, `run`, `asm`, `ir`, `lint`, `doc`, `names`, `dev`, `lsp` and `version`. `version` prints `compiler/low/version.txt`. `dev` runs `compiler/hot`, `lsp` runs `compiler/lsp`, and `compiler/lint` backs `lint`.
- `compiler/parallel/` runs a job over indices on one green thread per CPU. `ir/reach.low` builds and finalises the reachable functions in rounds through the `ir.Runner` it is given, `native.runner` being `parallel.each`, and `amd64/codegen.low` generates each function inside a `region` block whose value is the function's assembly text, data chunks and stack maps, so the allocation of instruction selection and register allocation never reaches the collector; both merge the results in the sequential order (the discovery order for the hoisted-literal cells, the function order for assembly, data labels in first-seen order and stack maps), so the output does not depend on scheduling. `native.build` passes `release` to `amd64.generate`, which drops each function's SSA once its assembly exists, so a unit cannot be generated twice after a build. The release in `bootstrap.txt` builds the first stage with a runtime that keeps a hoisted literal first built inside a region in region memory, so code a region reaches in the compiler must not hoist a literal, and `fuse_literal` in `amd64/streq.low` loops without a slice literal.
- `compiler/playground/` is the compiler built for `browser/wasm32`: it reads one program on standard input through `std/stdio`, compiles it with `ir.sequential` and a `Disk` that holds no files, and writes the module to standard output.
- `compiler/lowfuzz/` finds compiler bugs with generated programs, building each case in a child process through its hidden `__build` command; `compiler/fuzz/` holds its generator, model, oracle and reducer. `compiler/tablegen/`, `compiler/vkgen/`, `compiler/wlgen/` and `compiler/shadergen/` generate checked-in tables and bindings. `compiler/bench/` compares `low` with `bun`.
- `compiler/lowtest/` is the fixture harness. `compiler/peakrss` and `lowtest` sample `.maxrss` from `/proc` every millisecond.
- `compiler/tests/<name>/` holds the `std/testing` program that tests `compiler/<name>/`; `low` tests `compiler/low`, `editors` the editor grammars and `ui` the `std/ui` modules. The `asm` and `link` oracle tests skip unless the binutils and LLVM versions `compiler/oracletool` pins are installed.

## Commands

```
$ ./bootstrap.sh /tmp/low
$ /tmp/low build compiler/low -o /tmp/low2
$ /tmp/low2 build compiler/low -o /tmp/low3
$ /tmp/low build compiler/lowtest -o /tmp/lowtest
$ /tmp/lowtest /tmp/low2 run fail lint examples browser
$ /tmp/low2 run compiler/tests/check
```

`bootstrap.sh` builds the compiler with the release in `bootstrap.txt` and writes the result to its first argument, `./low` by default. A rebuilt compiler builds itself to the same bytes, so `low2` and `low3` are identical. `lowtest <low binary> [run|examples|fail|lint|windows|browser]... [-filter text]` checks any `low` binary against the sidecar files, using only its command line; `windows` runs only on a Windows host, with a `low.exe` and `lowtest.exe` built by `-target windows/amd64`. A `compiler/tests/<name>` program runs from the repository root. Run `low ir` and `low asm` on a program to read its SSA and assembly.

Sweeps, snapshots and test runs write gigabytes. Where `/tmp` is a `tmpfs` it is held in memory, so keep that work on disk and point temporary files there: `TMPDIR=<disk dir>`. Run sweeps with at most 8 jobs at once.

Source at release N compiles with release N-1, so the compiler cannot use a language feature until the release after it ships. After a release is published, `bootstrap.txt` moves to it.

## Language notes for compiler code

- Low has no module-level variables. A table is a function that returns it, or a `match`.
- There is one function namespace per module, and `l.peek(0)` calls `peek(l, 0)`. Two functions with one name on different receivers need different names.
- State a function mutates is a struct built with `new` and passed as `*T`.
- An error is the enum `diag.Error`. A function that can fail returns `! diag.Error` and its callers use `?`.
- A variant set is an enum with one variant per case; a set of behaviours called through one value is a struct of function values.
- A map is keyed by a `str` or an integer built from the key's fields.
- Source text is a `str`. `s[a..b]` shares the bytes, so token and name text is sliced, never copied.
- Output must not depend on map hash order. Low maps iterate in insertion order, which is deterministic; sort keys where a defined order is needed.
- An absent pointer is `*T?`, and an absent tree value is `Expr?`. `[]*T?` is an optional slice, not a slice of optionals: parallel slices with holes become one slice of a struct with optional fields, as `ast.InterpPart` does.
- The type singletons live on `types.Universe`, which every stage receives. Compare them by kind (`is_void`, `is_never`) or through the universe (`u.i64`).
- A struct copy is `new(*p)`. `ast/clone.low` copies each node that way and then replaces its children.
- A local cannot share the name of a function it calls, and a function cannot be named after a keyword or a type.
- An enum converts to an integer only through `.index`, which needs `= n` on every variant.
- `diag.quote` and `diag.quote_char` format a string with escapes and quotes. `lexer.quote` is the dump format.
- The compiler runs on a 1 MB green thread stack, so no pass recurses once per link of a left-deep binary chain or an `else if` chain. `ast.left_spine` and `ast.else_chain` list the links outermost first; a pass visits them in a loop with open and close steps (`ir.binary`, `ir.if_expr`, `check.binary`, `check.if_expr`), and the walkers, printers and clones iterate the same lists. The same holds for the SSA reader (`ir.read_recursive`) and the optimiser's dominator walk (`ir.visit`), which keep their own frames. `testdata/run/long_chains.low` and `testdata/fail/long_chain_error.low` hold chains of 20,000 terms.
- Assignments are statements: a `match` arm that assigns takes a `{ }` block.
- `"{{"` and `"}}"` are literal braces in a string.

## Language or syntax change

Read `## Syntax`. A change runs through `compiler/lexer`, `compiler/parser`, `compiler/ast`, `compiler/check`, then `compiler/ir`. Each new construct gets a `testdata/run` program, and each rejected form a `testdata/fail` program. A builtin function, type or constant is one entry in `compiler/check/builtins.low` (the only table of builtin names, parameters, `mut` markers, receivers and handlers) and one entry in `## Builtins`; `compiler/tests/check` fails on a missing, stale or differently named one, or one whose parameters or `mut` markers differ. Keywords, contextual keywords and operators are the tables in `compiler/lexer`. The editor grammars in `editors/` and `site/gen/highlight.low` list the keywords, contextual keywords, types and constants; the grammars also list the builtin functions and the TextMate grammar the constraints. The lists move with those tables, and `compiler/tests/editors` fails when a list lacks or adds a name.

## Code generation or optimisation

Read `## Compiler`. `compiler/ir` builds SSA, `compiler/amd64` emits assembly, `compiler/asm` assembles it and `compiler/link` writes the ELF or PE executable, all in process; `compiler/wasm` writes WebAssembly and `compiler/tests/wasm` compares it with the host under `node`. Compiling a program twice in one process must produce identical SSA and assembly; `compiler/tests/low` enforces it.

## Standard library

Read `## Standard library`. Modules live in `std/<name>/`, written in Low, with runnable programs that show how to use the module in `examples/`. `compiler/compiler/prelude/prelude.low` carries the runtime and the generic map code.

## Tests

Read `## Tests`: it lists the fixture directories, the sidecar files that declare what each program needs, and what every test checks. `lowtest` runs the fixtures and each `compiler/tests/<name>` runs with `low run`. `compiler/lowfuzz` finds compiler bugs with generated programs.

## Editor, LSP or lint

Read `editors/README.md`. `compiler/lsp` serves `low lsp`; `compiler/lint` backs `low lint`. The website's hovers and reference highlighting come from `low names`, so a change to it shows up in `site/` after a rebuild.

## Docs or website

Read `README.md` `## Website` before editing a Low snippet in `docs/design.md` or anything in `site/`. `site/index.html`, `404.html`, `search.json`, `docs/`, `packages/` and `playground/` are written by the Low program in `site/gen/` and ignored by git, `site/public/playground.wasm` is `compiler/playground` built for `browser/wasm32`, and `site/dist/` is the bundle Vite+ makes of them. Page content is in `site/content/`, styles and scripts are in `site/src/`, and the origin and repository URL are in `site/gen/config.low`. Edit the sources and run `npm run build` in `site/` with a `low` built from this tree on `PATH`: the generator builds each snippet for `browser/wasm32`, and the bootstrap release has no WebAssembly back end.
