#!/bin/bash
S=${S:?set S to the directory holding a5mlp and the bench binaries}
chase() { for i in 1 2 3 4 5; do taskset -c 5 $S/a5mlp h 1 16 | awk 'NR==2{print $3}'; done | tr '\n' ' '; echo; }
load() { for i in $(seq 40); do LOW_GCWORKERS=$1 taskset -c $2 $S/bin/shuffle >/dev/null; done; }
echo -n "solo: "; chase
for cfg in "1 0" "4 0-3" "4 0-3,6-9"; do
  set -- $cfg
  load $1 $2 & L=$!
  sleep 1
  echo -n "with shuffle w=$1 cpus=$2: "; chase
  wait $L
done
echo -n "solo: "; chase
