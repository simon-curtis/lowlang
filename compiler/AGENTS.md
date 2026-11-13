# Low compiler in Low

`compiler/` is the Low port of the Go compiler in `internal/` and `cmd/`. [docs/self-hosting/plan.md](../docs/self-hosting/plan.md) orders the work. The Go compiler is the oracle: a ported stage is done when its output equals the Go stage's output byte for byte.

## Layout

- `compiler/low.mod` marks the project root, so `import lexer` loads `compiler/lexer/`.
- One Go package `internal/<name>` becomes one module `compiler/<name>/`, keeping the Go file names with a `.low` extension.
- `compiler/lowdump/` and `cmd/lowdump/` are the same dump tool in Low and in Go. Each stage adds one command to both, in its own file, printing that stage's output as text or writing it as bytes.

## Build and compare

```
$ go build -o /tmp/low ./cmd/low
$ go build -o /tmp/lowdump-go ./cmd/lowdump
$ /tmp/low build compiler/lowdump -o /tmp/lowdump-low
$ /tmp/lowdump-go tokens file.low > a; /tmp/lowdump-low tokens file.low > b; cmp a b
```

The commands are `tokens`, `ast`, `check`, `ir`, `object`, `link`, `patch` and `regex`; run either tool without arguments for their flags. `ir` prints the SSA of every function, `object` the assembled sections, symbols and relocations of a `.s` file, `link` writes the executable for a `.s` file, `patch` prints the hot patch for a `.s` file against a base executable, and `regex` prints the matcher generated for each pattern in a file. `check` loads a whole program and prints every file's AST with the types, locals, targets and layouts the checker resolved; `cmd/lowdump/ast.go` and `compiler/lowdump/ast.low` print the same lines, so a field the checker sets is compared as soon as the Low checker sets it. The Go compiler embeds `internal/compiler/std` and `internal/compiler/prelude.low`; the Low `compiler` module reads them from disk, from the directory in `LOW_STD` or else `internal/compiler/std` relative to the working directory, with `prelude.low` beside it.

Compare stdout, stderr and the exit status over every `.low` file under `testdata/`, `internal/compiler/` and `bench/`, including the files that must fail.

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
- Assignments are statements: a `match` arm that assigns takes a `{ }` block.
- `"{{"` and `"}}"` are literal braces in a string.
