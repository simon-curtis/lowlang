# Low GC experiments

Code: `runtime-instrumentation.patch` applies to lowlang main 377d6b2a and holds the instrumentation, A4, `bench/shuffle`, and the code and fixture parts of 2e63dc5e from `fix/issue-267-mark-workers`. To rerun, apply it in a fresh worktree, build the compiler stages with `SOURCE_DATE_EPOCH=1700000000`, and export `S` (the directory holding the stage binaries, bench binaries, `maxrss` and `a5mlp`) and `W` (the patched worktree) before running a script. Heap dumps are regenerated with `LOW_GCDUMP` (see Instrumentation), and the `traces/<name>.fine` inputs of `a1.py`, `sim.py` and `frontier*.py` with `LOW_GCTRACE=1 LOW_GCRATIO=5 LOW_GCFLOOR=256 <program> 2>traces/<name>.fine`; neither is kept.
Machine: i7-8700K, 6 cores / 12 threads, 1 NUMA node, 31 GB; load average 6 to 8 from other processes during runs.

## Instrumentation added to the runtime (research only)

- Env knobs: `LOW_GCRATIO` (percent of live, default 100), `LOW_GCFLOOR` (KB, default 4096), `LOW_GCWORKERS` (cap), `LOW_GCLIMIT` (KB of occupancy; collect when live + allocated reaches it, grow rule off), `LOW_GCHW=1` (headroom from the highest live seen, not current live), `LOW_GCDUMP=K` (for collections whose number mod K is 0 or 1, write heap, start bitmap and mark bitmap to fd 3 after marking, and the heap again after sweep and release).
- `LOW_GCTRACE` fields: `stop_us restart_us alloc_kb threshold_kb gs stack_words regions at_us exact_kb`. `exact_kb` counts words handed out minus unused buffer tails; `alloc_kb` (the trigger's counter) counts whole 128 KB buffers.
- Gates passed for the `LOW_GCDUMP` build (`fa9`): self-build fixpoint with defaults, under `LOW_GC=stress`, and with the dump armed. Gates passed for the policy build: self-build fixpoint (stage2 == stage3) with defaults, under `LOW_GC=stress`, `LOW_GCLIMIT` and `LOW_GCHW`; `lowtest run` 634/634 on the knobs build. Fixtures under `LOW_GC=stress` on the policy build: 630/634; the 4 failures are timeouts with output that is a correct prefix (signal 9 or `.maxtime`). The unpatched compiler under stress also fails `rt_gc_stress` and `std_html_adversarial`; `simd_registers` passed when run alone; `std_regex` was not rerun to completion.

## Noise

A/A (main compiler vs patched, defaults, 10 interleaved runs): medians within 3 to 5% wall. Differences under 5% are treated as noise.

## E1 Pause composition (selfbuild, 3 runs, 147 GCs)

pause 114 ms/run = mark 87 + sweep 25 + stop 0.3 + restart 1.6. Stop p99 114 us. The 1.6 ms gaps in the first single run did not reproduce. Null: time-to-safepoint is not a lever in Low.

## E2 Cost model (single-worker GCs)

mark_us = 0.18 x live_kb (binarytrees, gcpause). The fitted intercepts were unstable (-1784 to +166 us), so the fixed cost per GC is an assumption (alpha = 100 us); the E3 ratios move by at most 0.3 between alpha = 0, 100 and 500. Sweep 0.34 us per KB allocated in binarytrees. The sweep term sums to the same total under every schedule, so only GC count and live-at-GC differ between policies.

## E3 Trace-driven policy simulation

Live curves recorded with `LOW_GCRATIO=5 LOW_GCFLOOR=256` (a collection every 5% of live). The simulator models Low's trigger including the grow rule.
Validation against real runs at ratio 100/200/400: total allocation exact, GC count and marked bytes within about 25% (multi-threaded programs collect more in reality because the trigger counts whole buffers); one outlier (binarytrees r=400, grow rule).

GC work at Low's own peak occupancy (alpha=100 us, beta=0.18 us/KB):

| workload | Low default / oracle | limit-at-peak (ALAP) / oracle |
|---|---|---|
| binarytrees | 1.72 | 1.16 |
| gcpause | 1.02 | 1.00 |
| jsonpar | 2.51 | 1.40 |
| strings | 1.07 | 1.04 |
| selfbuild | 1.66 | 1.14 |
| maps | 4.65 | 1.31 |

Most of the gap between Low's ratio rule and the optimal schedule is closed by knowing the budget. Choosing when to collect within the budget is worth at most 1.3x except on sawtooth workloads (jsonpar: each parse builds about 7 MB then drops it; the oracle collects just after the drop, mean live 2.9 MB against 4.4 MB for ALAP).

## E4 Scheduler signals as live-set predictors (H: stack depth or green-thread count predicts troughs)

corr(live, gs) and corr(live, stack_words): jsonpar +0.11/+0.04, selfbuild -0.50/-0.39, binarytrees n/a/+0.12, gcpause n/a/+0.08, strings n/a/-0.82. Falsified.

## E5 Policies in the runtime (10 interleaved runs, medians)

LIM = `LOW_GCLIMIT` set to the default run's own peak occupancy (a budget learned from a previous run). HW = `LOW_GCHW=1`. RSS from `wait4` in a 1.6 MB static launcher (`maxrss.c`); an earlier harness folded its own 17.7 MB into `ru_maxrss`. Marked = sum of live_kb over collections. GC CPU = pause x workers.

| workload | policy | wall s | CPU s | RSS MB | GCs | marked MB | GC CPU ms | max pause ms |
|---|---|---|---|---|---|---|---|---|
| binarytrees | D | 1.30 | 1.29 | 49.7 | 135 | 2760 | 506 | 5.7 |
| | HW | 1.21 | 1.21 | 47.6 | 111 | 2250 | 418 | 5.3 |
| | LIM | 1.18 | 1.18 | 49.6 | 97 | 2090 | 393 | 5.3 |
| jsonpar | D | 0.472 | 1.85 | 15.1 | 328 | 1580 | 873 | 1.1 |
| | HW | 0.446 | 1.72 | 15.0 | 240 | 1120 | 707 | 1.9 |
| | LIM | 0.397 | 1.52 | 14.9 | 148 | 561 | 486 | 1.3 |
| selfbuild | D | 0.742 | 4.71 | 117 | 49 | 1330 | 1260 | 5.8 |
| | HW | 0.739 | 4.72 | 115 | 49 | 1340 | 1260 | 5.7 |
| | LIM | 0.692 | 4.23 | 122 | 16 | 704 | 648 | 9.8 |
| maps | D | 0.126 | 0.125 | 76 | 13 | 176 | 0.86 | 0.15 |
| | LIM | 0.123 | 0.122 | 79 | 2 | 68 | 0.40 | 0.26 |
| strings | D | 0.0723 | 0.0717 | 8.2 | 21 | 94 | 6.5 | 0.38 |
| | LIM | 0.0719 | 0.0714 | 10.1 | 17 | 82 | 6.2 | 0.50 |

LIM raises RSS by 4% on selfbuild and maps and by 24% on strings: the limit counts occupancy, while RSS also holds heap span lost to fragmentation, which a non-moving heap cannot give back. Selfbuild's heap span is 136 MB against 79 MB peak occupancy; the simulator's equal-peak comparison is in occupancy and excludes this. LIM also raises max pause on selfbuild (5.8 to 9.8 ms) because each collection sees more live data.

The trigger counts whole 128 KB buffers per thread, so real multi-threaded runs collected 15 to 28% more often than the exact-allocation model.

## E6 Mark scaling with workers (selfbuild GCs with live over 20 MB)

| workers | ns/KB marked | CPU ns/KB |
|---|---|---|
| 1 | 289 | 289 |
| 2 | 163 | 327 |
| 4 | 96 | 383 |
| 6 | 78 | 470 |
| 12 | 65 | 755 |

Confounded: 6 cores with SMT, so the knee at 6 cannot be attributed to memory bandwidth on this machine.

## E7 Where mark time goes (perf, one run per condition, single-threaded workloads)

gcpause, GC on minus GC off: +13.8 G cycles, +36.3 G instructions (IPC about 2.6), +1.4 M LLC load misses.
perf record with {cycles, instructions, LLC-load-misses} per function:

| workload | function | cycles % | instructions % | LLC misses % |
|---|---|---|---|---|
| gcpause | rt_mark_solo | 60.6 | 59.7 | 19.6 |
| gcpause | rt_find | 7.4 | 9.4 | 5.0 |
| binarytrees | rt_mark_solo | 31.9 | 44.1 | 22.9 |
| binarytrees | fn_check (mutator) | 19.8 | 11.0 | 62.4 |

On these two shapes marking retires instructions at or above the program's average rate and is not where the cache misses are. Both allocate their graphs in address order, the best case for hardware prefetch, so this does not show that marking is never memory-bound; it shows that on these workloads the cost is instruction count in the mark loop.

## Novel attempts

Each attempt, with prior-art queries, pre-registered prediction, falsifier, result and lesson, is in `ATTEMPTS.md`. Summary:

| attempt | idea | status |
|---|---|---|
| A0 | fork-snapshot marking | exists (D, GC FAQ); not attempted |
| A1 | abortable probe collections to find sawtooth troughs | falsified: a probe costs more than a trough saves |
| A2 to A2d | barrier-free generational marking: per-page content hash of last cycle's survivors, masked by the mark bitmap | no minor collector built; offline C projection against the runtime's mark time. Streaming over last cycle's survivors (hash or exact scan) detects old-to-young heap edges soundly on 11 workloads (regions unchecked) and flags 7x to 70x fewer pages than a page dirty bit. Projected minor mark 0.44 to 0.54 of a full mark on two compiler front-end runs that end in a compile error (lint of one file, wasm build of `compiler/low`), 0.89 on doc, no gain on sort; sweep is unchanged, so about 0.6x of a pause. Hashing is 1.2x to 2x cheaper than the exact scan at the price of stored hashes and collision risk |
| A4 | A2 built in the runtime: sticky mark bits, page hashes compared at each minor, dirty pages' old objects scanned as roots (`LOW_GCMINOR`) | falsified for the compiler: correct on the fixpoints and on every fixture that passes under stress alone, but a larger heap without minors matches or beats it on selfbuild, lint and the wasm build at equal or lower RSS, and single-threaded hashing makes default-worker selfbuild mark 3.1x slower. On the two stable-graph benchmarks (the development benchmark and the synthetic written for A2; no independent program), both marking with one worker because they never spawn (the regime issue 267's fix removes; untested with more workers), it beats memory-matched controls: gcpause mark 292 ms against 699 ms (wall 1.51 s against 1.92 s), shuffle 495 ms against 803 ms. The deciding term is the residual a minor still marks (young survivors and dirty pages): 1% of a full mark on gcpause, 21% to 24% on the compiler. With default workers the residual alone matches a parallel full mark, because A4 scans dirty pages serially (90% of the residual) |
| A3 | meshing pages chosen exactly from the mark bitmap | stopped at the ceiling: about 4% of selfbuild RSS; Low's sparse pages overlap in offset |
| A5 | what bounds Low's parallel mark (ceiling measurement) | for selfbuild, neither DRAM bandwidth nor work distribution on this 6-core machine. selfbuild's mark draws 79% to 85% of the channel's random-line traffic at 6 to 12 workers, but that traffic is mostly hardware prefetch: L3-miss stalls stay at 16% to 21% of mark cycles, and the efficiency lost from 1 to 6 workers spreads over memory stalls at every level. Work distribution is 5.9% of on-CPU worker time at 12. The latency-bound single-worker case (shuffle) is already at the single-thread miss-parallelism limit; A6 covers shuffle with more workers |
| A6 | A4 and A5 rerun with issue 267's fix (single-threaded programs recruit mark workers) | shuffle's latency-bound mark stops scaling at 4 workers while its stalled CPU doubles, but a co-running pointer chase shows the channel has headroom: the limit is per worker, not DRAM bandwidth, and the cache-miss counter does not transfer from the microbenchmark to the mark. gcpause's single chain cannot use the workers; in the same binary the fix makes its mark 16% slower and its longest pause 43% longer. Minor mode loses to a slightly larger heap on shuffle and wins on gcpause as it did with one worker |

## Prior art found

- Offline optimal GC schedule by DP: Hertz and Immerman, "Optimal Choice of When to Garbage Collect", TOPLAS 2019.
- Collecting at profiled low-live program points: Buytaert et al., GCH, HiPEAC 2005.
- Heap sizes from profiles of earlier runs: a search snippet attributes this to automatic heap sizing research, paper not identified (unverified). OpenJDK draft JEPs for automatic heap sizing (G1, Serial) adapt within a run only.
- High-water headroom (HW): one search found no runtime that uses it. It resembles the JVM's committed heap, which grows and shrinks lazily. Not established as new.
- Square-root rule and the memory/CPU exchange rate: MemBalancer (Kirisame et al., OOPSLA 2022).

## Untested

- FFI and cross-heap boundaries: no workload exercises them. Low keeps droppables off the heap and never traces C memory. The `browser/wasm32` module imports only `write`, `exit`, `clock`, `entropy`, `gc_stress`, `page_limit`, the scheduler's `switch` and `wait`, and math functions, so no JS object is ever referenced from the Low heap or the reverse: there is no two-heap boundary to measure until Low gains JS object interop.
- Memory bandwidth on many cores: needs a machine with more physical cores than this one.

## Scripts

`a2.c` (A2 shadow analysis over heap dumps), `a2join.py` (joins with runtime mark times; outputs in `a2*-out/`), `a1.py`, `bench.py` (interleaved runs), `a4-run.sh` (A4 arms; outputs in `a4-out/`, replication in `a4-rep/`), `a4-traces.sh` and `a4split.py` (per-collection split of minor cost), `a5mlp.c` (random-line throughput ceiling), `a5measure.py` and `a5events.py` (mark DRAM rate and stall attribution by worker count; outputs in `a5-out/`), `a6-run.sh` (A6 arms; outputs in `a6-out/`), `a6-cochase.sh` (channel occupancy by co-running pointer chase), `runtime-instrumentation.patch` (runtime changes, A4 included), `sim.py` (simulator, oracle, ALAP, Low rule), `frontier3.py`, `fit.py`, traces in `traces/`, raw runs in `results.jsonl`.
