# Editor support

- Zed and Neovim highlight with the `tree-sitter-low` grammar; VS Code uses the TextMate grammar in `vscode/syntaxes`.
- All three start `low lsp`: diagnostics, hover, go-to-definition, references, rename, document and workspace symbols, completion, signature help, inlay hints for inferred types, and code actions on `?`. Rename and inlay hints answer only when the program compiles.
- At a `?`, the code actions offer `Catch the error`, which rewrites it as `catch { e => ... }`, and `Ignore the error with or`, which rewrites it as `or ...`; in an editor that accepts snippet edits (Zed, VS Code), each leaves the cursor where the handler goes. When the check fails because the enclosing function declares no error type, they also offer `Add ! E to f`, which declares the operand's error type on that function and imports its module if needed.
- `go test ./editors/` compares each list with the lexer and the checker and fails when one lacks or adds a name. The tree-sitter grammar and highlight query list keywords, contextual keywords, builtin functions, types and constants. The TextMate grammar lists those and the constraints. `site/gen/highlight.low` lists keywords, contextual keywords, types and constants.

`low` must be on `PATH`:

```sh
go install ./cmd/low
```

## Grammar

```sh
cd editors/tree-sitter-low
npm install
npx tree-sitter generate
npx tree-sitter test
```

`src/` is committed because Zed and Neovim compile `src/parser.c` and `src/scanner.c` directly. `generate` writes every file in `src/` except `scanner.c`, which is the hand-written external scanner behind `externals: [$._newline]`; regenerate after every change to `grammar.js`. `queries/highlights.scm` is the highlight query; `zed/languages/low/highlights.scm` and `nvim/queries/low/highlights.scm` are copies of it.

The repository holds no symlinks, because Windows checks them out as text. Files shared between editors are plain copies, listed in `copies` in `keywords_test.go`, and `go test ./editors/` fails when a copy differs from its source.

## VS Code

```sh
cd editors/vscode
npm install
npm run package
code --install-extension lowlang-0.1.0.vsix
```

The `lowlang.server.path` setting overrides the `low` binary. The `Low: Restart Language Server` command restarts it.

## Zed

Run `zed: install dev extension` and choose `editors/zed`. Zed builds the Rust extension and compiles the grammar at the commit named by `rev` in `extension.toml`, so `rev` must name a commit that contains the current `editors/tree-sitter-low/src`; `go test ./editors/` fails when `src/` differs from `rev`.

## Neovim

Requires Neovim 0.11 or later. With lazy.nvim:

```lua
{ dir = "~/path/to/lowlang/editors/nvim", build = "nvim -l build.lua" }
```

Without a plugin manager, add `editors/nvim` to `runtimepath` and run `nvim -l editors/nvim/build.lua` once. The build compiles the parser to `editors/nvim/parser/low.so` with `cc`.

The plugin enables the `low` language server unless `vim.g.low_lsp = false`. Indentation uses nvim-treesitter's `indentexpr` when nvim-treesitter is installed, and `cindent` otherwise.
