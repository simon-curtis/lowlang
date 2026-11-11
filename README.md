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

## Usage

Requires Go, and `as` and `ld` from binutils. Targets linux/amd64 and windows/amd64; the Windows target needs `clang` and `lld-link`.

```sh
go build -o low ./cmd/low
./low run testdata/run/fib.low
./low build testdata/run/fib.low -o fib
./low asm testdata/run/fib.low
./low build testdata/run/fib.low -target windows/amd64
./low lint app.low
./low dev app.low
./low run app/
```

A directory is a module; `import ui/text` loads `ui/text/` under the project root, which a `low.mod` file marks.

`low dev` runs the program and patches changed functions into it on every save, keeping its state.

The standard library is imported as `std/...`; [docs/design.md](docs/design.md) lists its modules.

## Benchmarks

```sh
go run ./cmd/bench
go run ./cmd/bench -runs 10 -only fib,nbody -langs low,go
```

Each `bench/<name>/` directory holds one program as `main.low`, `main.go` and `main.ts`. The runner builds them with `low build`, `go build` (standard library cached, `CGO_ENABLED=0`) and `bun build --compile`, fails a language whose output differs, then reports build time, binary size, median and minimum wall time, CPU time and peak RSS, with wall time relative to Go. `-low` uses a prebuilt compiler instead of building `./cmd/low`.

| Benchmark | Exercises |
|---|---|
| fib | recursive calls |
| sieve | `[]u8` indexing |
| nbody | `f64` arithmetic on a slice of structs |
| mandelbrot | `f64` loops |
| binarytrees | allocation and collection |
| maps | `i64` map insert, lookup, delete and iteration |
| strings | interpolation, `str` map keys, code point iteration |
| sort | generic quicksort with a closure comparator |
| channels | unbuffered ping-pong and a buffered worker pool |

## Website

`site/` is a static site styled with Tailwind. `npm run build` builds the compiler, renders `docs/design.md` and `editors/README.md` into `site/docs/`, highlights every Low snippet with hover text from `low lsp`, writes `style.css` from `tailwind.css` and copies the Lenis scroll library to `lenis.js`. The output is committed, so the directory serves as it is.

The LSP checks each snippet as a program: declarations stay at the top level and loose statements go inside `fn main()`. HTML comments directly above a block in a docs file add context that readers do not see. `<!-- prelude` holds Low code placed before the snippet. If the comment's last line opens a function, such as `fn main() ! ParseError {`, that line wraps the statements instead of `fn main()`. `<!-- prelude path/file.low` writes a file beside the snippet, for imported modules or embedded assets. The build lists each snippet that still fails to check.

```sh
cd site
npm install
npm run build
python3 ../assets/brand/build.py
```

`build.py` regenerates the icons, the social images and the web manifest from the mark; it needs `rsvg-convert`, `magick` and Noto Sans.

See [docs/design.md](docs/design.md) for the language design, the standard library, what is implemented and the roadmap.
