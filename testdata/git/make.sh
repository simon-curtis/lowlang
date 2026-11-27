#!/bin/sh
# Writes the packs, refs and listings the std/git tests read, using git.
set -eu
out=$(cd "$(dirname "$0")" && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1
export GIT_AUTHOR_NAME=low GIT_AUTHOR_EMAIL=low@example.com GIT_COMMITTER_NAME=low GIT_COMMITTER_EMAIL=low@example.com
git init --quiet -b main "$work/r"
cd "$work/r"
commit() {
    export GIT_AUTHOR_DATE="$1 +0100" GIT_COMMITTER_DATE="$1 +0100"
    git add -A
    git commit --quiet -m "$2"
}
seq 1 400 | sed 's/^/line /' > a.txt
mkdir -p dir/sub
printf 'echo hi\n' > dir/run.sh && chmod +x dir/run.sh
printf 'nested\n' > dir/sub/n.txt
ln -s a.txt link
commit 1700000000 one
sed -i 's/^line 7$/line seven/' a.txt
commit 1700000100 two
sed -i 's/^line 200$/line two hundred/' a.txt
commit 1700000200 three
git tag -a v1 -m "version 1"
sed -i 's/^line 300$/line three hundred/' a.txt
commit 1700000300 four
git tag light
git checkout --quiet -b topic
printf 'topic\n' > t.txt
commit 1700000400 five
git checkout --quiet main
sed -i 's/^line 9$/line nine/' a.txt
commit 1700000500 six
git tag -a v2 -m "version 2"
git rev-list --objects v1 | git pack-objects --quiet --delta-base-offset --window=10 --stdout > "$out/base.pack"
printf 'v2\nlight\ntopic\n^v1\n' | git pack-objects --quiet --revs --thin --window=10 --stdout > "$out/thin.pack"
git pack-refs --all
cp .git/packed-refs "$out/packed-refs"
git show-ref -d > "$out/refs.txt"
git cat-file --batch-all-objects --batch-check > "$out/objects.txt"
for rev in v1 'v1^{}' 'v2^{tree}' light topic 'main^{commit}' 'heads/topic' 'tags/v2'; do
    printf '%s %s\n' "$rev" "$(git rev-parse "$rev")"
done > "$out/revs.txt"
