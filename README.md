<img src="assets/icon.svg" width="64" height="64" alt="">

# LowLang

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

Requires Go, and `as` and `ld` from binutils. Targets linux/amd64.

```sh
go build -o low ./cmd/low
./low run testdata/run/fib.low
./low build testdata/run/fib.low -o fib
./low asm testdata/run/fib.low
./low dev app.low
```

`low dev` runs the program and patches changed functions into it on every save, keeping its state.

See [docs/design.md](docs/design.md) for the language design, what is implemented and the roadmap.
