set -u
S=${S:?set S to the directory holding the stage and bench binaries}
R=$(dirname "$(realpath "$0")")
W=${W:?set W to the patched lowlang worktree}
O=${1:-$R/a4-out}/results.jsonl; mkdir -p $(dirname $O)
export SOURCE_DATE_EPOCH=1700000000
run() { tag=$1; w=$2; ratio=$3; shift 3; cmd="$*"; python3 $R/bench.py --runs 5 --cwd $W --out $O --tag "$tag " "off=LOW_GCWORKERS:$w::$cmd" "minor=LOW_GCWORKERS:$w,LOW_GCMINOR:200::$cmd" "ratio$ratio=LOW_GCWORKERS:$w,LOW_GCRATIO:$ratio::$cmd"; }
run "gcpause w1" 1 200 $S/bin/gcpause-m
run "gcpause w1 rss-match" 1 400 $S/bin/gcpause-m
run "shuffle w1" 1 200 $S/bin/shuffle-m
run "selfbuild w1" 1 200 $S/fm2 build compiler/low -o $S/tmp-out
run "lint w1" 1 200 $S/fm2 lint compiler/low/main.low
run "wasm w1" 1 200 $S/fm2 build -target browser/wasm32 -o $S/tmp-wasm compiler/low
run "lint-pkg w1" 1 200 $S/fm2 lint compiler/low
run "playground-wasm w1" 1 200 $S/fm2 build compiler/playground -target browser/wasm32 -o $S/tmp-wasm
run "selfbuild wdef" 0 200 $S/fm2 build compiler/low -o $S/tmp-out
