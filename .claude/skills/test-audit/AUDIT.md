# Audit

Prune low-value tests in coherent batches, one owner per batch. Judge each test by its assertions, not its name. Discovery is read-only: report the candidates before you edit anything.

## Candidate evidence

For each candidate, read:

- the whole test, and every program that reads it by name (`rg -w <name> compiler site`). A `compiler/tests` program that drives a fixture differently from `lowtest`, such as with a held-open stdin, is the keeper of what the fixture alone cannot show;
- the production owner and its callers;
- sibling fixtures and tests that might also cover the contract;
- the `## Tests` entry that describes it;
- `git log --follow` on the test, for the bug it was added for.

Then record:

- the name and path, and the failure it can detect;
- every production caller of the code it covers. Test tooling does not count: `compiler/tests`, `_test.low` files, `compiler/lowtest`, `compiler/lowfuzz`, `compiler/fuzz` and `compiler/oracletool`;
- the stronger proof that remains, or why no contract exists;
- what deleting it unlocks: a `pub` seam, a helper, a sidecar;
- the risk, and the command that validates the change.

A candidate with missing evidence stays. Prefer a few high-confidence candidates to many weak ones. Each deletion must leave a keeper; never add a replacement test that is coupled to the implementation.

## Edits

- Delete each seam the deletion orphans: a `pub` function, parameter, flag or helper that only the test used.
- Move a retained regression into its owner's fixture or table.
- Rewrite every line of `docs/design.md` `## Tests` that names or counts the removed test. Each `std/...` bullet lists what that module's test files cover.
- A removed identifier is gone when `git grep -n "<identifier>" -- . ':!*.git*'` returns zero hits. Report the count.

## Validation

Use the commands in `AGENTS.md` `## Commands`.

- Run every suite that owns or neighbours the changed contract, plus `git diff --check`.
- After you edit `std/`, rebuild `low` before running its tests, because `std/` is embedded in it.
- After you edit compiler source, check that `low2` and `low3` are identical.

## Report

- Causes found, and the categories and seams removed.
- Candidates kept, with the reason each one is a real contract.
- Commands run and their results.
- Test and production line counts from `git diff --numstat`, kept separate.
- Follow-ups.
