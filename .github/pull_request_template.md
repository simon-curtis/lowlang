## Change

## Checks

Tick every item. Tick an item that does not apply and say so after it.

- [ ] The fixture harness (`compiler/lowtest`) and the `compiler/tests/<name>` programs the change touches pass
- [ ] Each new construct has a `testdata/run` program, each rejected form a `testdata/fail` program

## Website

- [ ] `docs/design.md` describes the change in the chapter that owns it, and the std module table lists any new module
- [ ] Each new or changed `std/` item has a doc comment and an `examples/` program, so its `site/packages/` page is current
- [ ] `docs/examples/` and the folder `README.md` files list any new or changed example
- [ ] `editors/README.md` matches any editor or grammar change
- [ ] `site/content/` and `README.md` match any change to what the landing page or playground says
- [ ] `npm run build` in `site/` passes, and I read each changed page in `npm run dev`

The source at release N builds with release N-1, so the compiler uses a language feature only in the release after the one that adds it. Every green `build` on `main` that changes `compiler/` outside its tool and test directories, or `std/`, cuts a patch release, and the `release` workflow commits `bootstrap.txt` to `main` with that release and the SHA-256 of its `low-linux-amd64`, so the compiler can use a feature once the patch release that carries it has moved `bootstrap.txt`.
