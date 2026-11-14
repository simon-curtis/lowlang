<div align="center">

<img src="assets/icon.svg" width="96" height="96" alt="Low">

# Low

A compiled language with Go's speed and footprint, plus enums, typed errors and expression-oriented syntax.

[![test](https://github.com/simon-curtis/lowlang/actions/workflows/test.yml/badge.svg)](https://github.com/simon-curtis/lowlang/actions/workflows/test.yml)
[![release](https://img.shields.io/github/v/release/simon-curtis/lowlang?color=111111)](https://github.com/simon-curtis/lowlang/releases/latest)
[![license](https://img.shields.io/github/license/simon-curtis/lowlang?color=111111)](LICENSE)

[Website](https://low.simoncurtis.dev) · [Documentation](https://low.simoncurtis.dev/docs/) · [Standard library](https://low.simoncurtis.dev/packages/) · [Releases](https://github.com/simon-curtis/lowlang/releases)

</div>

```sh
curl -fsSL https://low.simoncurtis.dev/install.sh | sh
```

```powershell
irm https://low.simoncurtis.dev/install.ps1 | iex
```

```low
fn fib(n: i64) -> i64 {
    if n < 2 { n } else { fib(n - 1) + fib(n - 2) }
}

fn main() {
    print(fib(30))
}
```

[low.simoncurtis.dev](https://low.simoncurtis.dev) has the overview and documentation. [docs/design.md](docs/design.md) is the full reference: syntax, standard library and compiler. The roadmap is the GitHub issues labelled `roadmap`, shown on the site's [roadmap page](https://low.simoncurtis.dev/docs/roadmap.html).

## Usage

`low` is written in Low. Building it from source needs a Linux host with `curl` and `sha256sum`: `./bootstrap.sh` builds the compiler with a published release and writes it to `./low`, or to the path given as its first argument. Compiling, linking and `low dev` need only the `low` binary; `low dev` needs a Linux host.

```sh
./bootstrap.sh
./low run testdata/run/fib.low
./low build testdata/run/fib.low -o fib
./low build testdata/run/fib.low -target windows/amd64
./low asm testdata/run/fib.low
./low lint app.low
./low doc std
./low names app.low
./low dev app.low
./low run app/
./low version
```

- `low` runs on Linux and on Windows. The target defaults to the host; `-target linux/amd64` or `-target windows/amd64` selects one.
- A directory is a module. `import ui/text` loads `ui/text/` under the project root, marked by a `low.mod` file.
- The standard library imports as `std/...`.
- `low dev` runs the program and patches changed functions into it on every save, keeping its state.
- On the Windows target the standard library calls Win32: files, processes, sockets, DNS, terminals and the environment work. Signals, file watches and pseudo-terminals fail with `Unsupported`; `docs/design.md` `## Compiler` lists the differences.
- The tests are Low programs: `compiler/lowtest` runs the fixtures and each `compiler/tests/<name>` runs with `low run`. `docs/design.md` `## Tests` says what they cover and which tools they use.

## Benchmarks

```sh
./low run compiler/bench
./low run compiler/bench -runs 10 -only fib,nbody
```

Each `bench/<name>/` holds one program as `main.low` and, for most, `main.ts`. The runner builds them with `low build` and `bun build --compile`, and fails a language whose output differs. It reports build time, binary size, median and minimum wall time, CPU time and peak RSS, with wall time relative to the first language. `-low` names the compiler to build with; the `low` on `PATH` is the default.

| Benchmark | Exercises |
|---|---|
| fib | recursive calls |
| sieve | `[]u8` indexing |
| nbody | `f64` arithmetic on a slice of structs |
| mandelbrot | `f64` loops |
| binarytrees | allocation and collection |
| alloc | short-lived heap allocations against a stack-allocated control |
| allocpar | heap allocation from parallel green threads |
| gcpause | collector pauses over chains, buffers and pairs |
| maps | `i64` map insert, lookup, delete and iteration |
| strings | interpolation, `str` map keys, code point iteration |
| sort | generic quicksort with a closure comparator |
| hof | function values passed to higher-order functions |
| jsonpar | JSON parsing and tree walking |
| channels | unbuffered ping-pong and a buffered worker pool |

## Releases

Pushing a `v*` tag runs the `test` workflow, then the `release` workflow builds `low` for `linux/amd64` and `windows/amd64` and publishes them as a GitHub release, `low-linux-amd64` and `low-windows-amd64.exe`. The workflow writes the tag into `compiler/low/version.txt` before it builds, so each binary prints the tag from `low version`; a build from source prints `dev`. `site/public/install.sh` and `site/public/install.ps1`, served by the website, download the latest release.

`bootstrap.txt` names the release that builds the compiler, `release v…`, and the SHA-256 of its Linux binary, `sha256 …`. `bootstrap.sh` downloads that release into `${XDG_CACHE_HOME:-$HOME/.cache}/lowlang/<release>/`, builds stage 1 with it, stage 2 with stage 1 and stage 3 with stage 2, fails unless stages 2 and 3 are byte-identical, and keeps stage 2. `LOW_SEED=<path>` uses a local binary instead of downloading one, and still checks its hash unless `LOW_SEED_UNCHECKED=1`.

The source at release N builds with release N-1, so the compiler uses a language feature only in the release after the one that adds it. After a release is published, `bootstrap.txt` moves to it with the SHA-256 of its `low-linux-amd64`.

## Website

`site/` is a static site. `site/gen` is a Low program that writes the pages, and Vite+ bundles them with their styles and scripts into `site/dist`, which is what Cloudflare Pages serves. The `test` workflow runs the fixtures and the `compiler/tests` programs on every push and pull request to `main`. The `site` workflow builds the site and deploys `site/dist` on every push to `main`, whether or not `test` passes. It also deploys when an issue is opened, edited, closed, reopened, deleted, labelled or unlabelled, so a roadmap change reaches the site.

```sh
cd site
npm install
npm run build
npm run dev
python3 ../assets/brand/build.py
npm install --prefix ../assets/brand/video
npm run render --prefix ../assets/brand/video
```

| Path | Holds |
|---|---|
| `site/gen/` | The generator, a Low module run with `low run site/gen` from the repository root. |
| `site/content/` | The hand-written bodies of the landing page and `404.html`. |
| `site/src/` | The stylesheet (Tailwind, in `style.css`), the scripts, the fonts and the inlined marks. |
| `site/public/` | Files copied to the site root unchanged: icons, the social image and video, the web manifest, `install.sh`, `install.ps1`, `_headers` and `_redirects`. |
| `site/index.html`, `404.html`, `docs/`, `packages/`, `search.json` | The generator's output, ignored by git. |
| `site/dist/` | The bundled site, ignored by git. |

`npm run build` runs the `site` task in `site/vite.config.ts`: `low run site/gen`, then `vp build`; the generator runs the `low` on `PATH`. `npm run dev` generates once and serves the pages, reloading styles and scripts as they change. `vp build` bundles every generated page and gives each stylesheet, script, font and the search index a content-hashed name under `assets/`; `site/public/_headers` lets browsers cache `assets/` without revalidating.

The generator:

- runs `low doc std` once for the standard library and `low names` once per Low snippet, several at a time. `low names` type-checks the snippet and prints every name in it with its hover text, its doc comment, its definition and the first name that refers to the same definition, so the generator needs no language server;
- renders `site/content/index.html` and `404.html` into `site/index.html` and `site/404.html`, wrapping each in the head, masthead, footer and scripts from `site/gen/layout.low`; the footer links the commit `git rev-parse HEAD` names, so a deployed page shows the commit it was built from; `site/gen/pages.low` lists each page's title and description, and `{{origin}}` and `{{repository}}` in the content expand to the URLs in `site/gen/config.low`; `{{examples}}` expands to the programs under `docs/examples/`, found recursively in path order and grouped by folder: each group is an `<h3>` with the title and first paragraph of the folder's `README.md`, then one collapsible entry per program with its highlighted source and a link to the file on GitHub, and a table row in the README that links the program's file name supplies the entry's description; every program is type-checked with the other snippets, with an empty file standing in for each `embed("...")` path;
- renders `docs/design.md` and `editors/README.md` into `site/docs/` with `std/markdown`, linking each code span that names a heading, such as `### Strings`, to that heading; a heading on the same page wins, and the build fails when the span names no heading or several;
- renders `site/docs/roadmap.html`, the last docs page, from the issues of the repository in `site/gen/config.low` that carry the `roadmap` label (state open or closed, in ascending number order) and each issue's sub-issues, recursively, in sub-issue order. It fetches them from the GitHub REST API with `std/http`, sends `Authorization: Bearer $GITHUB_TOKEN` when `GITHUB_TOKEN` is set and makes anonymous requests otherwise, and fails the build on a network error or a response that is not 2xx. Anonymous requests share GitHub's low rate limit, so set `GITHUB_TOKEN` for repeated builds. Each issue becomes a list item that links its title to the issue, renders its body as markdown and marks a closed issue `Done`;
- renders the standard library into `site/packages/`, one page per module, with each module's description from the table in `docs/design.md` and its examples, each with its expected output, above its types;
- highlights every Low snippet, including the examples in std doc comments and `examples` directories, with hover text and references from `low names`, and links each name to its definition: a declaration in the same snippet, an item in `site/packages/`, or the builtin's entry in the `## Builtins` chapter of `docs/design.md`;
- writes `site/search.json` as it renders: one entry per page with its title, section, description and opening text, and one per item on it. A docs or overview item is an `h2` to `h4` heading with its anchor and the text up to the next heading; a package item is a type, method, constant, function or example from `low doc std`, with its kind, a name qualified by the module or receiver (`msgpack.encode`, `Stream.read_all`), its signature and its doc text.

Every page carries a search palette, opened from the masthead's `Search` button, Ctrl K, ⌘ K or `/`. `site/src/search.js` fetches the index in the background when the pointer or focus reaches the button, or when the palette opens, and groups matches by page. Every query word must match. A word matched whole or at its start counts more than one matched inside a word, and a match in a name or title counts more than one in a description, a signature or the text, which counts more the more often it occurs. A name that equals or starts with the query ranks above the rest, and a page ranks by its own score or its best item, whichever is higher.

`build.py` regenerates the icons, social images and web manifest in `site/public/` and the mark in `site/src/` from the brand mark. It needs `rsvg-convert`, `magick` and Noto Sans.

`assets/brand/video/` renders `site/public/og-video.mp4`, the 12.5-second 1200×630 video that the landing page plays in its "Inside the compiler" panel through `site/src/film.js`, which plays it while it is on screen and shows the controls instead when the reader prefers reduced motion. Every page but `404.html` names it in `og:video` with `og:type` `video.other`, which Discord needs before it plays a linked video, and `og-image.png` is the still for platforms that do not play it; X shows the still, since its player card needs an approved domain. `render.mjs` compiles `hello.low` with `low` and hands the scene its source, `low ir` output, `fn_main` from `low asm`, the binary's size and its first 48 bytes. `scene.js` is a three.js scene that follows a spark through the compiler's stages on that program: the tokens drop in as keys, assemble into the syntax tree, take their types, fold into the SSA, give way to the assembly, and the ELF header's bytes become cubes that build the mark from `site/src/mark.svg`, beside which the name, the lead's first sentence from `site/content/index.html` and the address from `site/gen/config.low` come in. The ink, paper and `on-ink` syntax colours come from `site/src/style.css` and the fonts from `site/src/fonts/`. Each frame is a function of its time alone. `render.mjs` serves the repository on loopback, opens the scene in headless Chromium with SwiftShader, draws each frame at twice the size and pipes it to `ffmpeg`, which scales it down and encodes H.264 tagged BT.709 with the index at the front. It uses the `low` that `LOW` names or the one on `PATH`, finds `chromium`, `chromium-browser`, `google-chrome-stable` or `google-chrome` on `PATH` unless `CHROMIUM` names a browser, and needs `ffmpeg`. `--draft` renders at the target size and 15 frames a second, `--crf` sets the quality (26 by default), `--still 1.5,6` writes those moments as PNG files instead of a video, and a path argument names the output file, which `--still` requires.

### Checked snippets

`low names` checks each Low snippet as a program: every `low` fence in the docs and in a std doc comment, and every `data-lang="low"` block in `site/content/`. Declarations stay at the top level and loose statements go inside `fn main()`, and `///` lines stay with the declaration below them. A `...` standing for elided code checks as a diverging loop. A std example that does not import its own module checks with that import added. The build lists every snippet that fails to check and then fails. It fails when a prefix in `site/gen/categories.low` names no module, and when a std module has no category.

HTML comments directly above a `low` fence in `docs/design.md` or `editors/README.md` supply context readers do not see. Blocks in `site/content/` do not read them.

- `<!-- prelude` holds Low code placed before the snippet. If its last line opens a function, such as `fn main() ! ParseError {`, that line wraps the statements instead of `fn main()`.
- `<!-- prelude path/file.low` writes a file beside the snippet, for imported modules or embedded assets.

## License

[MIT](LICENSE)
