# Low

Compiler for the Low language, written in Go. [docs/design.md](docs/design.md) is the language and compiler reference; each branch below names the section to read first.

## Language or syntax change

Read `## Syntax`. A change runs through `internal/lexer`, `internal/parser`, `internal/ast`, `internal/check`, then `internal/ir`. Each new construct gets a `testdata/run` program, and each rejected form a `testdata/fail` program. A builtin function, type or constant is one entry in `internal/check/builtins.go` (the only table of builtin names, parameters, `mut` markers, receivers and handlers) and one entry in `## Builtins`; `TestBuiltinsDocumented` in `internal/check` fails on a missing, stale or differently named one, or one whose parameters or `mut` markers differ. Keywords, contextual keywords and operators are the tables in `internal/lexer/token.go`. The editor grammars in `editors/` and `site/highlight.mjs` list the keywords, contextual keywords, types and constants; the grammars also list the builtin functions and the TextMate grammar the constraints. The lists move with those tables, and `go test ./editors/` fails when a list lacks or adds a name.

## Code generation or optimisation

Read `## Compiler`. `internal/ir` builds SSA, `internal/amd64` emits assembly, `internal/asm` assembles it and `internal/link` writes the ELF or PE executable, all in process. Compiling a program twice in one process must produce identical SSA and assembly; `cmd/low` tests enforce it.

## Standard library

Read `## Standard library`. Modules live in `internal/compiler/std/<name>/`, written in Low, with runnable programs that show how to use the module in `examples/`. `internal/compiler/prelude.low` carries the runtime and the generic map code.

## Tests

Read `## Tests`: it lists the fixture directories, the sidecar files that declare what each program needs, and what every test checks. `go test ./...` runs them all. `cmd/lowfuzz` finds compiler bugs with generated programs.

## Editor, LSP or lint

Read `editors/README.md`. `internal/lsp` serves `low lsp`; `internal/lint` backs `low lint`. The website's hovers and reference highlighting come from the LSP, so an LSP change shows up in `site/` after a rebuild.

## Docs or website

Read `README.md` `## Website` before editing a Low snippet in `docs/design.md` or anything in `site/`. `site/index.html`, `404.html`, `docs/`, `packages/` and `style.css` are generated; page content is in `site/content/`, and the origin, repository URL and output folders are in `site/config.mjs`. Edit the sources and run `npm run build` in `site/`.
