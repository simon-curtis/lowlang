# Editor support

- Zed and Neovim highlight with the `tree-sitter-low` grammar; VS Code uses the TextMate grammar in `vscode/syntaxes`.
- All three start `low lsp`. It provides diagnostics, hover, go-to-definition, references, rename, document and workspace symbols, completion, signature help, inlay hints for inferred types, and code actions on `?`, on statements outside a function and on `fn main`. Rename and inlay hints answer only when the program compiles. In a `_test` file they analyse the test build of its module. So they reach the module's private functions and report its test signature errors.
- At a `?`, the code actions offer `Catch the error` and `Ignore the error with or`. `Catch the error` rewrites it as `catch { e => ... }`. `Ignore the error with or` rewrites it as `or ...`. In an editor that accepts snippet edits (Zed, VS Code), each leaves the cursor where the handler goes. When the check fails because the enclosing function declares no error type, the code actions also offer `Add ! E to f`. It declares the operand's error type on that function and imports its module if needed.
- On a statement outside a function, the code actions offer `Move the statements into fn main`. It gathers every top-level statement, with the comments above it, into `fn main` at the first statement. It declares the error type the statements gave `main`, importing its module if needed. When that type comes from a check that has not finished on the current text, the action is left out. On the `fn main` line of a `main` that takes no parameters and returns nothing, in the entry module, the code actions offer `Move fn main to the top level`. It puts its body where the declaration was. It turns its `///` comment into `//`. It names the error enum on a first `fail` that leaves it out. Both are `refactor.rewrite` actions. The server also runs them as the `low.toggleMain` command of `workspace/executeCommand`, whose one argument is the document URI. The command applies whichever rewrite the file allows, wherever the cursor is. It fails when neither applies.
- `low run compiler/tests/editors` compares each list with the lexer and the checker. It fails when one lacks or adds a name. The tree-sitter grammar and highlight query list keywords, contextual keywords, builtin functions, types and constants. The TextMate grammar lists those and the constraints. `site/gen/highlight.low` lists keywords, contextual keywords, types and constants.

`low` must be on `PATH`:

```sh
./bootstrap.sh ~/.local/bin/low
```

## Grammar

```sh
cd editors/tree-sitter-low
npm install
npx tree-sitter generate
npx tree-sitter test
```

`src/` is committed because Zed and Neovim compile `src/parser.c` and `src/scanner.c` directly. `generate` writes every file in `src/` except `scanner.c`. `scanner.c` is the hand-written external scanner behind `externals: [$._newline]`. Regenerate after every change to `grammar.js`. `queries/highlights.scm` is the highlight query. `zed/languages/low/highlights.scm` and `nvim/queries/low/highlights.scm` are copies of it.

The repository holds no symlinks, because Windows checks them out as text. Files shared between editors are plain copies, listed in `compiler/tests/editors/main.low`. `low run compiler/tests/editors` fails when a copy differs from its source.

## VS Code

```sh
cd editors/vscode
npm install
npm run package
code --install-extension lowlang-0.1.0.vsix
```

The `lowlang.server.path` setting overrides the `low` binary. The `Low: Restart Language Server` command restarts it, and `Low: Toggle Top-Level Statements and fn main` runs `low.toggleMain` on the active file.

## Zed

Run `zed: install dev extension` and choose `editors/zed`. Zed builds the Rust extension and compiles the grammar at the commit named by `rev` in `extension.toml`. So `rev` must name a commit that contains the current `editors/tree-sitter-low/src`. `low run compiler/tests/editors` fails when `src/` differs from `rev`.

## Neovim

Requires Neovim 0.11 or later. With lazy.nvim:

```lua
{ dir = "~/path/to/lowlang/editors/nvim", build = "nvim -l build.lua" }
```

Without a plugin manager, add `editors/nvim` to `runtimepath` and run `nvim -l editors/nvim/build.lua` once. The build compiles the parser to `editors/nvim/parser/low.so` with `cc`.

The plugin enables the `low` language server unless `vim.g.low_lsp = false`. Indentation uses nvim-treesitter's `indentexpr` when nvim-treesitter is installed, and `cindent` otherwise. `:LowToggleMain` runs `low.toggleMain` on the current buffer.
