set -u
S=${S:?set S to the directory holding the stage and bench binaries}
R=$(dirname "$(realpath "$0")")
O=${1:-$R/a6-out}/results.jsonl; mkdir -p $(dirname $O)
for b in gcpause shuffle; do
  python3 $R/bench.py --runs 5 --out $O --tag "$b wdef " "off=::$S/bin/$b" "minor=LOW_GCMINOR:200::$S/bin/$b" "ratio200=LOW_GCRATIO:200::$S/bin/$b" "ratio400=LOW_GCRATIO:400::$S/bin/$b" "ratio600=LOW_GCRATIO:600::$S/bin/$b"
done
