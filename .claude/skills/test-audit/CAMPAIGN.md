# Campaign

A campaign prunes the whole test surface of one subsystem on one branch, for example `compiler/lsp`, `compiler/amd64` or `std/sql`. [SKILL.md](SKILL.md) and [AUDIT.md](AUDIT.md) apply to every lane. Each step ends on its completion criterion. Do not start the next step until it is met.

## 1. Baseline

Pin a `main` SHA. Record the subsystem's test and support line counts, and the pass or fail state of every fixture, test program and `_test.low` file in scope. List failures separately, because each one may be a compiler bug.

Done when every in-scope test has a recorded result.

## 2. Lanes

Split the surface into **lanes** along production owner boundaries, not file names. For the compiler, the lanes might be lexer, parser, check, ir, amd64, asm, link, wasm, native, lsp and lint. Include the subsystem's fixtures under `testdata/`, its `compiler/tests/<name>` program, its `_test.low` files and its std examples.

Done when every in-scope test belongs to exactly one lane.

## 3. Ledger

Give each lane to its own read-only agent. The agent reads every test in the lane in full, including table rows and sidecars, along with the production owners, callers and history. It marks each test or table row in a written **ledger**:

- `R`: retain. Name the contract and the bug it catches.
- `F`: retain the contract but repair the assertion, such as a `.err` that matches an unrelated guard.
- `C`: consolidate. Name the owner that absorbs the assertion.
- `D`: delete. Name the proof that remains, or say why no contract exists.

Done when every test and row in the lane has a mark and an evidence line.

## 4. Layer plan

Treat the ledger as input, not as the edit list. Make a second read-only pass that looks for a redundant **layer**, such as a `compiler/tests` case that replays what a fixture already proves through the command line. Name the **keeper** for each contract, and correct any ledger errors you find.

Done when each lane names the tests it deletes, the keeper for each contract, the assertions to carry into keepers, and the `pub` seams the deletions unlock.

## 5. Cutover

Edit one lane at a time. Changes to shared code (`compiler/fixture`, `compiler/lowtest`, `std/testing`) go through one agent. Each lane also removes the seams it unlocks, and its edits to `docs/design.md` `## Tests`. Commit each lane on its own.

Done when every lane plan is applied and each lane's keepers pass.

## 6. Preservation review

Independent reviewers, one per lane group, compare the deleted coverage with the keepers. They look for contracts that lost their only proof, and for new assertions that can never fail.

For each restored contract, mutate the production owner once and confirm the keeper goes red. Then restore the source byte for byte.

Done when every gap is restored, or rejected with source evidence, and every restored contract has caught its mutation.

## 7. Compiler defects

A baseline failure that survives into a keeper is a bug. Fix it in its owner as a separate commit. Show that the keeper is red with the fix reverted and green with it applied, using the same compiler build. Record unrelated defects as follow-ups.

Done when each fix has a red control and a green candidate.

## 8. Hand off

Squash any reversals on the branch, then rebase onto `main`. When `main` changed a test the campaign deleted, keep the deletion and port the new contract into the keeper. Confirm that every regression `main` added still has an owner. Report as [AUDIT.md](AUDIT.md) describes, adding:

- the baseline and final line counts, with production counted separately;
- the lanes, deleted layers and keepers;
- the preservation gaps found, and the mutations that caught them;
- the defects fixed, with their control and candidate runs.
