# Low

Compiler for the Low language, written in Go. [docs/design.md](docs/design.md) is the language and compiler reference; each branch below names the section to read first.

## Language or syntax change

Read `## Syntax`. A change runs through `internal/lexer`, `internal/parser`, `internal/ast`, `internal/check`, then `internal/ir`. Each new construct gets a `testdata/run` program, and each rejected form a `testdata/fail` program. The editor grammars in `editors/` and `site/highlight.mjs` list the keywords and move with the lexer; `go test ./editors/` catches a missed keyword.

## Code generation or optimisation

Read `## Compiler`. `internal/ir` builds SSA, `internal/amd64` emits assembly. Compiling a program twice in one process must produce identical SSA and assembly; `cmd/low` tests enforce it.

## Standard library

Read `## Standard library`. Modules live in `internal/compiler/std/<name>/`, written in Low. `internal/compiler/prelude.low` carries the runtime and the generic map code.

## Tests

Read `## Tests`. `go test ./...` runs every `testdata/run` program against its `.out`, `.err`, `.maxrss` and `.needed` files, and expects every `testdata/fail` program to fail with its `.err`. `cmd/lowfuzz` finds compiler bugs with generated programs.

## Editor, LSP or lint

Read `editors/README.md`. `internal/lsp` serves `low lsp`; `internal/lint` backs `low lint`. The website's hovers and reference highlighting come from the LSP, so an LSP change shows up in `site/` after a rebuild.

## Docs or website

Read `README.md` `## Website` before editing a Low snippet in `docs/design.md` or anything in `site/`. `site/docs/`, `style.css` and the snippets in `site/index.html` are generated; edit their sources and run `npm run build` in `site/`.
