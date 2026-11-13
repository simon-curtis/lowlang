# Self-hosting plan

Port the compiler to Low stage by stage, using the Go compiler as a byte-exact oracle, then cut over at a tagged release and delete Go. This works because output is already deterministic and the assembler, linker and runtime need no external tools.

Scope: about 45k lines of Go source and 12k lines of Go tests in `cmd/` and `internal/`, plus `editors/*_test.go`, `bench/*/main.go`, `go.mod`, three workflows and `site/lsp.mjs`. The 5.7k lines of runtime `.asm`, the prelude and the std are already Go-free.

## Phase 0: close language gaps

Each gap is fixed in the Go compiler and tagged before the port depends on it. I inferred these from `docs/design.md` and import counts, not from a trial port, so the first task is a spike porting `internal/lexer` to confirm them.

| Gap | Why the compiler needs it | Proposed fix |
|---|---|---|
| Directory embedding | The binary carries `std/` (1,800 files); `embed` takes one literal path | A directory form of `embed` |
| Composite map keys | `Hash` covers only integers, `bool`, `str` and pointers; type interning keys on structure | Extend `Hash` to structs and enums of `Hash` fields |
| Version stamp | `-ldflags -X main.version` has no Low equivalent | `embed` a version file the release workflow writes |
| Recursion depth | Green threads have 1 MB stacks; parser, checker and IR builder recurse | Measure on the largest std module, then decide |
| `reflect` in `internal/ast/clone.go` | Generic deep clone | Hand-written clone per node |

No fix is needed for the 68 type switches (they become `match` on enums) or the 8 interfaces (they become structs of function values).

## Phase 1: test harness in Low

Port the fixture runner (`internal/fixture`, the run/fail/lint/windows checks in `cmd/low`) to a Low program that takes the compiler binary as an argument. It runs against the Go-built `low` first, so it is validated before any compiler code moves. Add dump commands for tokens and AST to the Go `low`, alongside the existing `low ir` and `low asm`.

## Phase 2: port, one checkpoint at a time

Each stage is done when its output equals the Go compiler's over every `testdata/run` program, std example and benchmark, on both targets.

1. `asm` and `link`: fed assembly from the Go `low asm`; executable bytes equal.
2. `lexer`, `parser`, `ast`: token and AST dumps equal.
3. `types`, `check`, `builtin`, `prelude`, `lint`: `testdata/fail` `.err` and `testdata/lint` `.warn` equal.
4. `ir`: `low ir` text equal.
5. `amd64`: `low asm` text equal.
6. `compiler` loader and the `low` driver: whole binaries equal.

Language and codegen changes freeze during this phase, or each lands twice.

## Phase 3: fixpoint

- Go `low` builds the Low compiler (stage 1). Stage 1 builds itself (stage 2). Stage 2 builds itself (stage 3).
- Gate: stage 2 and stage 3 are byte-identical, and stage 2 passes the Phase 1 harness.
- Gate: compile time and peak RSS on the benchmarks are within an agreed factor of the Go build.

## Phase 4: port the rest

- `lsp`, `hot`, `fuzz`, `cmd/lowfuzz`, `tablegen`, `vkgen`, `wlgen`, `shadergen`, `cmd/bench`, `peakrss` and `oracletool`.
- The remaining Go tests move onto `std/testing`: optimiser assertions, LSP protocol, hot reload, asm/link oracles, `TestBuiltinsDocumented`, editor grammar lists and UI tests.
- `internal/regex` is ported or folded into `std/regex`.

## Phase 5: cutover

1. Tag the last Go-built release. It is the root seed.
2. Add a bootstrap file naming the seed release and its SHA-256. CI downloads the seed, builds, and runs the fixpoint check.
3. Delete every `.go` file and `go.mod`. Move `internal/compiler/std` and the compiler modules to top-level directories. Rewrite the workflows, `site/lsp.mjs`, `README.md`, `AGENTS.md` and `docs/design.md` to current state.
4. Zero-hit search for `go.mod`, `*.go`, `go test`, `go build`, `go install`, `go vet`, `setup-go` and `lowlang/internal`.
5. Tag the first self-built release.

Standing rule afterwards: source at tag N must compile with release N-1, so the compiler cannot use a language feature until the release after it ships.

## Decisions for you

- **Go benchmark baselines (`bench/*/main.go`)**: "Runtime speed comparable to Go" is a stated goal and these measure it. I recommend deleting them and the Go half of `cmd/bench`, since keeping them keeps a Go toolchain dependency.
- **Freeze during Phase 2**: I recommend a full freeze on language and codegen changes; std and site work continue.
- **Seed distribution**: I recommend downloading from GitHub releases with a pinned hash, with no binaries in git.
