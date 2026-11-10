# Editor support

`tree-sitter-low` is the grammar for `.low` files. Zed and Neovim use it directly; VS Code uses the TextMate grammar in `vscode/syntaxes`. All three start `low lsp`, which serves diagnostics, hover, go-to-definition, references, rename, document symbols and completion. Rename only runs when the program compiles. `low` must be on `PATH`:

```sh
go install ./cmd/low
```

`go test ./editors/` fails when the lexer has a keyword that a grammar or highlight query does not cover.

## Grammar

```sh
cd editors/tree-sitter-low
npm install
npx tree-sitter generate
npx tree-sitter test
```

`src/` is generated and committed, because Zed and Neovim compile `src/parser.c` directly. Regenerate after every change to `grammar.js`. `queries/highlights.scm` is shared: Zed and Neovim link to it.

## VS Code

```sh
cd editors/vscode
npm install
npm run package
code --install-extension lowlang-0.1.0.vsix
```

The `lowlang.server.path` setting overrides the `low` binary. The `LowLang: Restart Language Server` command restarts it.

## Zed

Run `zed: install dev extension` and choose `editors/zed`. Zed builds the Rust extension and compiles the grammar at the commit named by `rev` in `extension.toml`, so `rev` must name a commit that contains the current `editors/tree-sitter-low/src`.

## Neovim

Requires Neovim 0.11 or later. With lazy.nvim:

```lua
{ dir = "~/path/to/lowlang/editors/nvim", build = "nvim -l build.lua" }
```

Without a plugin manager, add `editors/nvim` to `runtimepath` and run `nvim -l editors/nvim/build.lua` once. The build compiles the parser to `editors/nvim/parser/low.so` with `cc`.

The plugin enables the `low` language server; set `vim.g.low_lsp = false` to stop that. Indentation uses nvim-treesitter's `indentexpr` when nvim-treesitter is installed, and `cindent` otherwise.
