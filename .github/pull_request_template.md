## Change

The source at release N builds with release N-1, so the compiler uses a language feature only in the release after the one that adds it. Every green `build` on `main` that changes `compiler/` outside its tool and test directories, or `std/`, cuts a patch release, and the `release` workflow commits `bootstrap.txt` to `main` with that release and the SHA-256 of its `low-linux-amd64`, so the compiler can use a feature once the patch release that carries it has moved `bootstrap.txt`.
