# Editor support

- Zed and Neovim highlight with the `tree-sitter-low` grammar; VS Code uses the TextMate grammar in `vscode/syntaxes`.
- All three start `low lsp`: diagnostics, hover, go-to-definition, references, rename, document and workspace symbols, completion, signature help, and inlay hints for inferred types. Rename and inlay hints answer only when the program compiles.
- `go test ./editors/` fails when a grammar, highlight query or `site/highlight.mjs` lacks or adds a keyword, builtin function, type, constant or constraint compared with the lexer and the checker.

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

`src/` is generated and committed because Zed and Neovim compile `src/parser.c` directly; regenerate it after every change to `grammar.js`. Zed and Neovim link to the shared `queries/highlights.scm`.

## VS Code

```sh
cd editors/vscode
npm install
npm run package
code --install-extension lowlang-0.1.0.vsix
```

The `lowlang.server.path` setting overrides the `low` binary. The `Low: Restart Language Server` command restarts it.

## Zed

Run `zed: install dev extension` and choose `editors/zed`. Zed builds the Rust extension and compiles the grammar at the commit named by `rev` in `extension.toml`, so `rev` must name a commit that contains the current `editors/tree-sitter-low/src`.

## Neovim

Requires Neovim 0.11 or later. With lazy.nvim:

```lua
{ dir = "~/path/to/lowlang/editors/nvim", build = "nvim -l build.lua" }
```

Without a plugin manager, add `editors/nvim` to `runtimepath` and run `nvim -l editors/nvim/build.lua` once. The build compiles the parser to `editors/nvim/parser/low.so` with `cc`.

The plugin enables the `low` language server unless `vim.g.low_lsp = false`. Indentation uses nvim-treesitter's `indentexpr` when nvim-treesitter is installed, and `cindent` otherwise.
