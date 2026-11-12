<img src="assets/icon.svg" width="64" height="64" alt="Low">

# Low

A compiled language with Go's speed and footprint, plus enums, typed errors and expression-oriented syntax.

```low
fn fib(n: i64) -> i64 {
    if n < 2 { n } else { fib(n - 1) + fib(n - 2) }
}

fn main() {
    print(fib(30))
}
```

[low.simoncurtis.dev](https://low.simoncurtis.dev) has the overview and documentation. [docs/design.md](docs/design.md) is the full reference: syntax, standard library, compiler and roadmap.

## Usage

Building `low` needs Go. Compiling and linking a program needs only the `low` binary. `low dev` needs a Linux host with binutils (`as`, `ld`).

```sh
go build -o low ./cmd/low
./low run testdata/run/fib.low
./low build testdata/run/fib.low -o fib
./low build testdata/run/fib.low -target windows/amd64
./low asm testdata/run/fib.low
./low lint app.low
./low doc std
./low dev app.low
./low run app/
```

- `low` runs on Linux and on Windows. The target defaults to the host; `-target linux/amd64` or `-target windows/amd64` selects one.
- A directory is a module. `import ui/text` loads `ui/text/` under the project root, marked by a `low.mod` file.
- The standard library imports as `std/...`.
- `low dev` runs the program and patches changed functions into it on every save, keeping its state.
- On the Windows target, standard library modules that call the operating system (files, processes, sockets and everything built on them) panic with `system calls are not available on windows`. Language features and modules that only compute run unchanged.
- `go test ./...` expects a Linux host.

## Benchmarks

```sh
go run ./cmd/bench
go run ./cmd/bench -runs 10 -only fib,nbody -langs low,go
```

Each `bench/<name>/` holds one program as `main.low`, `main.go` and, for most, `main.ts`. The runner builds them with `low build`, `go build` (`CGO_ENABLED=0`, standard library cached) and `bun build --compile`, and fails a language whose output differs. It reports build time, binary size, median and minimum wall time, CPU time and peak RSS, with wall time relative to Go. `-low` uses a prebuilt compiler instead of building `./cmd/low`.

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

## Website

`site/` is a static site styled with Tailwind. Its output is committed, so the directory serves as it is. The `test` workflow runs `go vet` and `go test` on every push and pull request to `main`. When it passes on a push to `main`, the `site` workflow rebuilds the site and deploys it to Cloudflare Pages.

```sh
cd site
npm install
npm run build
python3 ../assets/brand/build.py
```

`npm run build`:

- builds the compiler and starts `low lsp`;
- renders `docs/design.md` and `editors/README.md` into `site/docs/`;
- renders the standard library from `low doc std` into `site/packages/`, one page per module, with each module's description from the table in `docs/design.md`;
- highlights every Low snippet with hover text and references from the LSP;
- writes `style.css` from `tailwind.css` and copies Lenis to `lenis.js`.

`build.py` regenerates the icons, social images and web manifest from the mark. It needs `rsvg-convert`, `magick` and Noto Sans.

### Checked snippets

The LSP checks each Low snippet in the docs as a program: declarations stay at the top level and loose statements go inside `fn main()`, and `///` lines stay with the declaration below them. A `...` standing for elided code checks as a diverging loop. The build lists every snippet that fails to check and then fails.

HTML comments directly above a block supply context readers do not see:

- `<!-- prelude` holds Low code placed before the snippet. If its last line opens a function, such as `fn main() ! ParseError {`, that line wraps the statements instead of `fn main()`.
- `<!-- prelude path/file.low` writes a file beside the snippet, for imported modules or embedded assets.
