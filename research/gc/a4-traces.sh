set -u
S=${S:?set S to the directory holding the stage and bench binaries}
R=$(dirname "$(realpath "$0")")
T=$R/a4-out/traces
mkdir -p $T
export SOURCE_DATE_EPOCH=1700000000 LOW_GCTRACE=1
cd "${W:?set W to the patched lowlang worktree}"
one() { name=$1; workers=$2; shift 2
  LOW_GCWORKERS=$workers "$@" >/dev/null 2>$T/$name.off.trace; echo "$name off rc=$?"
  LOW_GCWORKERS=$workers LOW_GCMINOR=200 "$@" >/dev/null 2>$T/$name.minor.trace; echo "$name minor rc=$?"; }
one gcpause 1 $S/bin/gcpause-m
one shuffle 1 $S/bin/shuffle-m
one selfbuild 1 $S/fm2 build compiler/low -o $S/tmp-out
one lint-pkg 1 $S/fm2 lint compiler/low
one playground-wasm 1 $S/fm2 build compiler/playground -target browser/wasm32 -o $S/tmp-wasm
one selfbuild-wdef 0 $S/fm2 build compiler/low -o $S/tmp-out
$S/fm2 doc std >/dev/null 2>&1; echo "doc std rc=$?"
$S/bin/sort-d >/dev/null 2>&1; echo "sort rc=$?"
