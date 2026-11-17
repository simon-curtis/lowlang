---
name: test-audit
description: "Tests: gate every new or changed fixture, compiler/tests program or _test.low file; audit or prune existing tests; remove pub seams only tests call."
---

# Test audit

Every test proves one **contract** at its **owner**: the strongest boundary that can observe it. The gate below applies to every test you add or change. To audit existing tests, read [AUDIT.md](AUDIT.md). To prune a whole subsystem's tests, read [CAMPAIGN.md](CAMPAIGN.md) first.

## Owners

`docs/design.md` `## Tests` lists every suite and what each one checks. From strongest to narrowest:

1. A fixture in `testdata/run`, `fail`, `lint`, `test` or `windows`, or a std example. `lowtest` runs it through the `low` command line only, so the test survives any internal refactor. Program behaviour, diagnostics, lint warnings, `low test` output, memory and time limits, linking and the browser build all belong here.
2. A `compiler/tests/<name>` program. It owns contracts no fixture can observe: compiling twice gives the same output, the SSA and assembly the optimiser produces, the language server protocol, parity between tables and docs or grammars, and assembler and linker output compared with external tools.
3. A `std/<module>/*_test.low` file. It owns a module's private edge cases where a fixture cannot reach every input cheaply.

## Gate

Answer all four before you write the test. If any answer is missing, the test does not get written.

1. Which observable behaviour, invariant or independent contract does it protect?
2. Which credible regression turns it red?
3. Why does existing proof miss it? Search the fixtures and test programs first. Extend a sibling fixture, table row or helper before you add a file. A second layer earns a test only for a distinct risk, such as the browser back end, Windows, the dev profile or hot reload.
4. Does it need a `pub` function, parameter or flag that no production code calls? If so, prove the contract through the owner instead.

- A regression goes red on the defect and green after the fix. Prove it: build the compiler without the fix and run the test against it. If no baseline was available, say so.
- One owner proves the bug. Leave the other layers it passes through untested.
- A behaviour-preserving refactor leaves the test green.
- A `lowfuzz` reproduction is a regression like any other: give it a name that states its contract, or fold it into the fixture that already owns that contract.

## Junk

These patterns prove nothing:

- assertions that compare a value with itself, or expected values computed by the code under test;
- inventories copied by hand from the source. A parity test that reads both sides, such as `compiler/tests/check` or `compiler/tests/editors`, is a contract, not junk;
- source greps and checks on call shapes, where a fixture already proves the behaviour. A source check stays only when it survives renaming every identifier it reads;
- `pub` exports, parameters or flags that only tests use;
- programs with no assertion, such as a `testdata/run` program with no `.out` whose only proof is that it compiles and exits 0, when a sibling fixture already compiles the same construct;
- sidecars captured from the compiler under test and saved without checking them against the language's semantics;
- negative fixtures that fail at an unrelated guard. A `testdata/fail` program's `.err` must name the rule it targets;
- names that promise behaviour the inputs never exercise.

## Retain

Keep these even when they are static, slow or look like snapshots:

- language semantics, diagnostic text and lint warnings;
- determinism, and a rebuilt compiler building itself to the same bytes;
- ELF, PE and WebAssembly layout, and agreement with the pinned binutils and LLVM;
- optimiser output that `compiler/tests/compiler` checks;
- stack depth (`long_chains`), `.maxrss` and `.maxtime` limits;
- the `low test` protocol, the language server protocol, `build.low`, lock and version-range handling;
- the Windows and browser builds.

A failing test may be a compiler bug. Fix the compiler; do not delete the test.
