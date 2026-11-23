# Attempts

Each attempt: idea, prior-art check, prediction, falsifier, method, result, lesson.

## A0 Fork-snapshot concurrent marking (rejected before building)

Idea: stop the world, fork, mark the copy-on-write snapshot in the child, sweep in the parent.
Prior art: exists. D's concurrent GC on Linux forks and marks in the child; the GC FAQ describes snapshot mark-sweep through copy-on-write.
Status: not attempted.

## A1 Abortable probe collections

Idea: a trigger cannot know the live size without marking, which is why collections land at peaks of a sawtooth. Make the measurement itself cheap and bounded: once the heap is past a fraction of its budget, start a collection with a mark budget B. If marking finishes within B, it was a trough and the collection completes. If the mark exceeds B, abort it (clear mark bits, sweep nothing) and resume the program; the attempt cost at most B. The policy pays a bounded tax to find troughs.
Prior art (queries: "abort garbage collection mark phase when live data exceeds budget, speculative trial collection"; "opportunistic garbage collection trigger at low live data points"; "CMS abortable preclean G1 concurrent mark abort reason"; "cancel or abandon garbage collection cycle if heap mostly live, cost-benefit predictor"). Adjacent: GCH cost function postpones collections (2005); a patent on feedback-based abortable collections aborts to meet a pause budget; CMS abortable preclean aborts on time; G1 aborts concurrent mark only for a full GC; Opportunistic ZGC (2026) triggers on idle CPU. None aborts a mark on a live-size budget to find troughs.
Ceiling (round one): on jsonpar, the best case, real LIM marked 561 MB against a simulated oracle of about 467 MB, so at most about 17% of GC CPU, about 5% of process CPU, before any probe tax. Other workloads have less.
Prediction: in simulation at Low's default peak budget, tuned on the first half of each trace and scored on the second half, A1 recovers at least half of the ALAP to oracle gap on jsonpar, and costs no more than 3% over ALAP on any workload.
Falsifier: recovers under a quarter of the gap on jsonpar, or costs more than 3% over ALAP on any workload, or the no-abort ablation does as well (meaning the abort adds nothing).
Gate: no runtime work unless the simulation passes and the ceiling is above measurement noise (it is not: 5% CPU against 3 to 5% noise).
Result: falsified. With a never-probe option in the grid, tuning on the first half picks never for every workload (A1 = ALAP); with probing forced on, it costs 1.5 to 2.9x ALAP. Trough width on jsonpar is about 22% of allocation, but a probe costs a fixed stop plus a partial mark (about 0.4 ms) against about 0.24 ms saved per collection, so blind probing loses.
Lesson: finding troughs needs a signal that costs far less than a stop; a bounded trial collection is too expensive at this heap size. Timing ideas need a near-free predictor, and the scheduler signals tested in round one (stack depth, green-thread count) are not one.

## A2 Content-hash dirty detection (barrier-free generational marking)

Idea: Low has no write barriers, and gcpause spends 70% of its time re-marking a structure that does not change. Detect which old pages changed by hashing them instead of by a barrier, page faults or kernel soft-dirty bits: after a collection, store a hash of every page holding live pointer objects; at the next collection, only pages whose hash differs can hold new pointers from old objects, so a minor collection scans those plus the roots and the new objects. Marking is instruction-bound, while hashing a page is a streaming read, so the hash may be cheaper than re-marking.
Prior art (3 queries: "detect modified pages by hashing page contents instead of write barrier", "checksum based dirty page detection generational remembered set without barrier", "Boehm GC virtual dirty bits SOFT_VDB MPROTECT_VDB PROC_VDB"):
- Sticky-mark-bit generational collection in a non-moving heap with VM dirty bits: Demers et al. 1990; Boehm GC's MPROTECT_VDB, PROC_VDB and SOFT_VDB modes. The generational part of A2 is not new.
- Boehm GC ships `checksums.c`, which checksums pages to check that the dirty-bit implementation reported every changed page. It is a debug check of a VDB, not a VDB. Closest prior art.
- Detlefs et al. (JVM 2002) name the barrier-free extreme, "scan the entire old generation", as the imprecise end of the remembered-set trade-off.
- Hosking 1993 compares dirty bits, cards and remembered sets; all need a barrier or VM traps.
Not found: content hashing as the production dirty detector, or masking the hash by the mark bitmap so that allocation into holes and writes to young objects do not dirty a page (VM dirty bits cannot make that distinction).
Cross-domain prior art (3 more queries: incremental checkpointing, DSM twins and diffs, persistent-store commit detection): page hashing to find changed memory is standard in hash-based incremental checkpointing (patent "Adaptive incremental checkpointing"); DSM twins and diffs compare against a saved copy; Magma (Squeak) compares shallow copies at commit; Chipmink (arXiv 2511.14162) diffs hashed object-graph pods. Claim scoped to: content hashing as a GC dirty detector, masked by the mark bitmap.
Method (shadow mode, no minor collector built): `LOW_GCDUMP=K` writes the heap, start bitmap and mark bitmap to fd 3 after marking, for collections whose number mod K is 0 or 1 (consecutive pairs). `a2.c` reads each pair; old set O = objects marked at the earlier collection (they cannot be freed before the later one). Arms, single-threaded C, min of 3:
- M: full trace reproducing the runtime's mark set (checked bit for bit; roots = marked objects with no marked referrer, then any unreached marked object).
- H: hash of every word of each non-leaf object in O, combined per 4 KB page by header address, compared to the hashes taken at the earlier collection. H2 hashes only words that fall in the heap or region range.
- S: direct scan of O's non-leaf objects for words resolving to objects not in O (the known barrier-free extreme).
Projected minor cost = H + M's per-word rate x (words of O objects on H-dirty pages + young live words). Root and region scanning is common to both and excluded. Soundness check: every page S finds must be H-dirty.
Deviation from protocol: gcpause (the development workload) was measured before the falsifiers below were written. gcpause, 8 pairs: no dirty pages, no young survivors; H costs 0.32 of M (143 vs 122 ns per KB scanned), S costs 2.8x H; projected minor 0.32x full. gcpause is the best case and is not counted toward the prediction.
Prediction (written before running any other workload): on binarytrees, selfbuild, maps, strings, jsonpar and a shuffled-graph workload: unsound pages = 0 everywhere; S costs at least 1.5x H on at least 4 of 6; projected minor at most 0.5x full on at least 2 of binarytrees, selfbuild, shuffled; the masked hash finds at least 2x fewer dirty pages than an unmasked whole-page hash on at least one workload.
Falsifiers: any unsound page; S within 1.2x of H on a majority (hashing adds nothing over the known scan); projected minor above 0.67x full on all of binarytrees, selfbuild and shuffled; masked and unmasked dirty fractions equal everywhere (the mask adds nothing over VM dirty bits).
Calibration: the C mark is 0.7x to 3.3x the runtime's own mark time, so the runtime's single-worker `mark_us` for the same collection (dump and trace from one run, `LOW_GCWORKERS=1`) is the denominator. `a2join.py` joins them; outputs in `a2-out/`.
Result (medians over consecutive pairs; proj = projected minor / runtime full mark):

| workload | pairs | live MB | H / mark | S / H | dirty H | dirty H2 | dirty S | dirty unmasked | unsound | proj (min to max) |
|---|---|---|---|---|---|---|---|---|---|---|
| gcpause (dev) | 8 | 114 | 0.27 | 2.3 | 0 | 0 | 0 | 0.0013 | 0 | 0.27 (0.20 to 0.28) |
| binarytrees | 14 | 20 | 1.38 | 1.4 | 0 | 0 | 0 | 0.0006 | 0 | 1.41 (0.83 to 2.70) |
| selfbuild | 6 | 22 | 0.39 | 2.7 | 0.018 | 0.018 | 0.018 | 0.47 | 0 | 0.66 (0.58 to 0.91) |
| shuffle | 9 | 71 | 0.18 | 3.4 | 0.16 | 0.16 | 0.16 | 0.18 | 0 | 0.41 (0.27 to 1.10) |
| jsonpar | 16 | 4.6 | 0.38 | 3.5 | 0.008 | 0.005 | 0.004 | 0.69 | 0 | 1.02 (0.65 to 1.82) |
| strings | 20 | 4.9 | 0.70 | 3.6 | 0.5 | 0 | 0 | 1.0 | 0 | 1.11 (0.64 to 1.61) |
| maps | 11 | 10 | n/a | 1.9 | 1 | 1 | 1 | 1 | 0 | 3.8 (no old pointer data; marking is nearly free) |

Against the pre-registration: unsound = 0 everywhere (pass); S at least 1.5x H on 5 of 6 (pass, binarytrees 1.42); projected minor at most 0.5x on 2 of binarytrees, selfbuild, shuffle (fail: shuffle 0.41 only, selfbuild 0.66, binarytrees 1.41); masked at least 2x fewer dirty pages than unmasked on one workload (pass: selfbuild 27x, jsonpar 84x). No falsifier fired. Verdict: the detector is sound and its precision claim holds; the speed claim holds on large pointer graphs (gcpause, shuffle), is modest on selfbuild and fails on binarytrees, jsonpar, strings and maps.
Lessons:
- The mask is where the novelty pays. Allocation into holes rewrites 47% of selfbuild's old pages and 69% of jsonpar's between collections; only 1.8% and 0.8% of them change inside old objects. Any page-granular detector the kernel or MMU offers sees the former.
- Hashing every word (H) and hashing only pointer-range words (H2) differ only where old objects hold changing scalars (strings: 0.5 vs 0); H2 matches the exact scan S almost everywhere.
- Per-object hashing loses to Low's mark on 3-word objects (binarytrees: 207 vs about 140 ns per KB): the per-object finalize and bitmap walk cost more than marking the object. Sawtooth programs (jsonpar) and young-dominated ones gain nothing because the young trace is the whole cost.

## A2b Run hashing (second try at A2's speed claim)

Idea: hash each maximal run of adjacent old non-leaf objects (split at 4 KB page boundaries of headers) as one stream, so finalization is paid per run, not per object.
Prediction (written before running): binarytrees H / mark falls below 0.6 and its projected minor below 0.75; shuffle and selfbuild projected minor improve by at least 20%; unsound stays 0.
Falsifier: binarytrees H / mark stays above 1.0, meaning the per-object bitmap walk and header read, not the hash, is the cost, and a hash cannot beat Low's mark on small objects.
Result: prediction failed, falsifier did not fire. binarytrees H / mark 1.38 to 0.91, projected minor 1.41 to 0.94; selfbuild 0.66 to 0.59 (10%); shuffle 0.41 to 0.39 (6%); gcpause 0.27 to 0.19. Unsound still 0. Outputs in `a2b-out/`.
Lesson: finalization was about a third of the per-object cost; the rest is the walk itself (bit scan over O, header read for size and leaf flag). Beating Low's mark on small objects needs a pass that does no per-object work.

## A2c Whole-page hashing for pages that were fully old

Idea: at the earlier collection, flag pages whose start bits equal their mark bits (no garbage, no free chunk starts). At the later collection those pages are hashed as raw 4 KB streams with no per-object work; other pages use run hashing. Sound because the raw hash covers every word of the page; allocation into a free tail that enters the page from the previous one, or scalar writes to leaf objects, can only add spurious dirty pages.
Prediction (written before running): binarytrees H / mark below 0.5 and projected minor below 0.6; no workload's projected minor gets worse by more than 5%; unsound 0.
Falsifier: binarytrees H / mark above 0.8, or spurious dirtying from leaf data raises any workload's projected minor by more than 10%.
Corrections made during A2c, each found by the soundness check or by the cost column:
1. The first run had unsound pages on selfbuild (11), shuffle (188), jsonpar (4) and strings (1). An object starting on a fully old page and spilling into the next page was covered by neither unit. Fixed by extending a page's raw hash to the end of its last object.
2. That extension also swallowed huge leaf strings (maps 99x the mark). It now extends only for non-leaf objects, since leaf objects hold no pointers.
3. strings hashed 1.9x the words, because whole-page mode also hashed pages holding only leaf objects. A page now qualifies only if it holds a non-leaf object; the flag is set at the earlier collection, where a sweep reads headers anyway.

Result (final variant, `a2c-out/`):

| workload | H / mark | S / H | dirty H | dirty unmasked | unsound | proj (min to max) |
|---|---|---|---|---|---|---|
| gcpause | 0.15 | 3.9 | 0 | 0.0013 | 0 | 0.15 (0.11 to 0.16) |
| binarytrees | 0.56 | 3.4 | 0 | 0.0006 | 0 | 0.61 (0.34 to 0.85) |
| selfbuild | 0.32 | 2.9 | 0.027 | 0.47 | 0 | 0.59 (0.52 to 0.94) |
| shuffle | 0.11 | 5.3 | 0.16 | 0.18 | 0 | 0.36 (0.23 to 0.89) |
| jsonpar | 0.35 | 3.7 | 0.010 | 0.69 | 0 | 0.97 (0.63 to 1.61) |
| strings | 0.67 | 3.3 | 0.5 | 1.0 | 0 | 1.08 (0.61 to 1.56) |
| maps | 6.8 | 1.0 | 1 | 1 | 0 | 7.1 (mark is about 0.1 ms; no gain possible) |

Against the A2c pre-registration: binarytrees H / mark 0.56 (predicted below 0.5: fail), projected minor 0.61 (predicted below 0.6: fail, narrowly); maps' projection rose from 3.8 to 7.1 (the 5% clause fails, though maps is a workload where minor collection is never worth choosing); unsound 0 after the fixes. The falsifier (H / mark above 0.8) did not fire.

Standing of A2 overall: masked content hashing is a sound barrier-free dirty detector on all seven workloads, 3 to 5x cheaper than the known barrier-free alternative (scanning old objects for young pointers). The mark bitmap mask removes 27x (selfbuild) to 69x (jsonpar) of the page changes a VM or kernel dirty bit would report. On workloads with stable pointer graphs the projected minor collection costs 0.15 to 0.61 of a full mark; on sawtooth and leaf-heavy workloads it costs about as much or more, so a real collector would choose per collection.
What the projection leaves out: root and region scanning (same in both); sweep (unchanged; it is about a quarter of a pause); floating garbage from old objects that die between full collections; storing and refreshing hashes for dirty and newly promoted pages; multiple workers (hashing splits by page with no synchronization); and the gap between C and a hand-written pass in the runtime.
Risk: a 64-bit hash collision on a changed page misses old-to-young pointers, and the consequence is a freed live object. The rate is about 2^-64 per dirty page per collection. A twin copy of old pointer pages removes the risk at the cost of memory.
Lessons:
- Correctness checks pay for themselves: the hybrid's first version was unsound, and only the exact-scan cross-check showed it.
- Cost lives in per-object work, not in bytes: hashing gets cheap only once it stops touching objects one at a time.

## A2d Controls and replication for A2 (pre-registered before running)

Three gaps in A2c:
- The control S was never tuned, while H got three rounds.
- The unmasked baseline was taken before sweep and heap release, which write to exactly the pages that hold garbage.
- Every speed prediction failed, and the final variant was tuned on the same seven workloads.

Scope of soundness: the check covers heap-to-heap edges only. Regions appear at 2 of selfbuild's 49 collections and nowhere else. A real minor collection must keep region hit bits sticky and rescan or hash region contents.
Changes:
- S2 is the control given H's structure. Pages fully old at the earlier collection are streamed word by word, the other pages object by object. Each word is rejected by a range check plus one byte lookup in a page map of pages covered by young objects, and only the survivors are resolved. The allocator could maintain the young-page map at buffer refill, so building it is not timed.
- The unmasked baseline is a raw hash of the heap dumped after `rt_heap_release` at the earlier collection (`LOW_GCDUMP` now writes a second, heap-only record there).
- `a2.c`'s H is frozen as the A2c variant. Held-out workloads not seen before:
  - `low doc std` (166 collections, median live 3.5 MB)
  - `low build -target browser/wasm32 compiler/low` (22 collections, up to 20 MB), which ends in a compile error: `compiler/low` imports `std/sys`, which `browser/wasm32` lacks
  - `low lint compiler/low/main.low` (28 collections, up to 47 MB), which ends in `undefined function: install_command` because it lints one file of a package
  - bench `sort` (5 collections, up to 16 MB)
  
  `alloc`, `allocpar`, `channels` and `hof` hold no live data and were dropped. doc, wasm and lint are the compiler binary, and wasm and lint run its front end up to an error, so the held-out evidence is one program family plus `sort`.

Predictions:
1. S2 costs at least 1.5x H on at least 4 of the 6 non-development workloads. If it does not, H's advantage over the known barrier-free scan came from tuning, and the finding is a page-filtered streaming scan, not hashing.
2. With the post-release baseline, the masked hash still finds at least 2x fewer dirty pages than unmasked on selfbuild and jsonpar.
3. Held out: unsound 0 on all four; projected minor at most 0.67 (runtime denominator) on at least 2 of 4.

Falsifiers:
- S2 within 1.2x of H on a majority.
- The masked to unmasked ratio below 2x on both selfbuild and jsonpar.
- Any unsound page held out, or projected minor above 0.67 on all four.
Result (`a2d-out/`, frozen H; proj under the runtime mark denominator, C-mark denominator in brackets):

| workload | S / H | S2 / H | dirty H | dirty unmasked (post-release) | unsound | proj (min to max) [C] |
|---|---|---|---|---|---|---|
| gcpause (dev) | 3.96 | 1.26 | 0 | 0.0013 | 0 | 0.13 (0.10 to 0.17) [0.23] |
| binarytrees | 3.44 | 1.21 | 0 | 0.0006 | 0 | 0.57 (0.31 to 0.82) [0.18] |
| selfbuild | 2.87 | 2.00 | 0.031 | 0.40 | 0 | 0.60 (0.53 to 1.02) [0.55] |
| shuffle | 4.98 | 1.24 | 0.16 | 0.18 | 0 | 0.35 (0.22 to 0.89) [0.34] |
| jsonpar | 3.82 | 1.37 | 0.0097 | 0.075 | 0 | 1.04 (0.66 to 1.99) [0.86] |
| strings | 3.50 | 1.18 | 0.5 | 1.0 | 0 | 1.12 (0.67 to 1.71) [0.65] |
| maps | 1.15 | 1.90 | 1 | 1 | 0 | 6.6 (no old pointer data) |
| doc (held out) | 2.54 | 1.79 | 0.0043 | 0.30 | 0 | 0.89 (0.60 to 2.53) [0.62] |
| wasm, front end to error (held out) | 3.02 | 2.05 | 0.017 | 0.21 | 0 | 0.54 (0.39 to 0.93) [0.52] |
| lint, front end to error (held out) | 3.00 | 1.98 | 0.023 | 0.15 | 0 | 0.44 (0.36 to 0.92) [0.49] |
| sort (held out) | 1.13 | 1.89 | 0 | 0 | 0 | 4.3 (leaf arrays only) |

Against the pre-registration:
1. S2 at least 1.5x H on at least 4 of 6: fail (2 of 6: selfbuild 2.0, maps 1.9; binarytrees 1.21, shuffle 1.24, jsonpar 1.37, strings 1.18). The falsifier (S2 within 1.2x on a majority) did not fire (1 of 6). On the held-out compiler workloads S2 is 1.8 to 2.05x H.
2. Masked at least 2x fewer dirty pages than unmasked on selfbuild and jsonpar, with the post-release baseline: pass, 13x and 7.7x. The earlier 27x and 69x were inflated by sweep and release writes.
3. Held out: unsound 0 on all four (pass); projected minor at most 0.67 on 2 of 4 (pass: lint 0.44, wasm 0.54; doc 0.89; sort no gain).

Standing of A2 after controls:
- Masked content hashing is a sound detector of old-to-young heap edges on eleven workloads.
- The mark-bitmap mask removes 7x to 70x of the page changes an unmasked, post-release page detector sees.
- On two held-out compiler front-end runs, a minor collection built on it projects to about half a full mark (0.44 to 0.54); A4 built it with a slower hash and measured 0.70 to 0.93 of a full mark per minor on the working versions of those workloads (a different denominator: A4's full mark covers a sticky set holding 17% to 26% dead objects), with young survivors and dirty pages alone costing 0.21 to 0.24; on development workloads with stable graphs, 0.13 to 0.60.
- Its speed advantage over the known barrier-free alternative, once that alternative gets the same page structure and a young-page filter, is 1.2x to 2x, not 3x to 5x. The scan needs no stored hashes and has no collision risk, so the choice between them is a trade, not a win.
- The speed claim was tuned across three variants on the development set; the held-out run is its only out-of-sample test.

Lessons:
- Give the control the same engineering as the treatment before reading the ratio; the untuned control overstated the hash's advantage by about 2.5x.
- Measure the baseline at the point where its real mechanism would reset; taking it before sweep overstated the mask's advantage by about 2x to 9x.
- The precision over page dirty bits belongs to any detector that reads only last cycle's survivors: the exact scan S flags no more pages than H on every workload. What A2 established is empirical: in Low, streaming over survivors is cheap enough that a barrier-free minor collection projects to about half a full mark on compiler workloads. Hashing is one of two ways to do it and adds 1.2x to 2x speed (an upper bound: S2 had one tuning round, H three) for stored hashes and collision risk.
- Scope of the projection: mark only (sweep unchanged, about a quarter of a pause, so 0.5x mark is about 0.6x pause); full collections are still needed to reclaim old garbage; heap-to-heap edges only.

## A3 Meshing a non-moving GC heap from the mark bitmap

Idea: Low's heap does not move objects, so pages that hold a few live objects stay resident (selfbuild: RSS 117 MB against 79 MB peak occupancy). Mesh merges two virtual pages whose live bytes do not overlap onto one physical frame, without moving objects. A collector knows every live word after marking, so it can pick mesh partners exactly at each collection, instead of relying on randomized allocation and probing as Mesh does.
Prior art (3 queries: "Mesh compaction without relocation garbage collector non-moving heap", "merge sparsely occupied pages same physical frame disjoint live objects mark bitmap conservative GC", "meshing garbage collected runtime Ruby Go JavaScript reduce RSS"):
- Mesh (Powers et al., PLDI 2019) and its predecessor Hound do this for malloc.
- A Hacker News comment on Mesh suggests it for non-compacting collectors.
- Index-Compact GC (Tong and Lau, APLAS 2010) uses two virtual spaces for compaction.
- No managed runtime found that meshes.
Novelty is limited to exact, bitmap-driven partner selection inside a collector.
Ceiling check first (from the existing dumps, before any prediction about savings): the fraction of resident pages holding live data that are sparse enough to mesh, and the gap between pages holding live data and live words / 512.
Ceiling result (`a3.c`, `a3-out/`; pairs of pages at most half full whose live words do not overlap, 64 random probes per page, means over dumps with live above 2 MB):

| workload | pages holding live data vs live words / 512 | pages meshing would free |
|---|---|---|
| selfbuild | 21.6% more | 5.9% |
| wasm, front end to error | 15.6% more | 3.2% |
| lint, front end to error | 13.6% more | 2.7% |
| doc | 46.2% more | 11.3% |
| jsonpar | 16.0% more | 1.2% |
| binarytrees, strings, shuffle | under 1% more | 0 to 0.3% |

Selfbuild's heap mapping at peak RSS: 88.6 MB resident against 60.8 MB maximum live. Estimated, combining the RSS run with the dump runs: about 17 MB of that is sparse live-holding pages and about 11 MB is free memory still resident. Meshing would return about 6% of the live-holding pages, about 4.6 MB, or 4% of process RSS.
Verdict: stopped at the ceiling, not built. The build gate (at least 10% of heap RSS) was set after seeing these numbers, so this is a judgement, not a pre-registered test.
Lesson: Low's sparse pages are not disjoint. Bump allocation in 128 KB buffers places survivors at correlated offsets, so half-empty pages collide. Mesh makes pages meshable by randomizing placement, which is Mesh's own contribution, not a GC-specific one. The bitmap's exactness does not help when the layout itself prevents meshing. The larger RSS terms in Low are live-holding sparse pages (about 17 MB) and resident free memory (about 11 MB), which is the release policy's territory.

## A4 Barrier-free sticky-mark minor collection, built in the runtime

Idea: build A2 for real.
- `LOW_GCMINOR=F` turns it on.
- Sweep keeps mark bits, so after any collection the marked set is last cycle's survivors.
- A collection is minor while live words stay below F% of the live size after the last full collection (floor: `LOW_GCFLOOR`).
- Minor collection:
  - hash each marked non-leaf object, XOR the hashes into the page of its header, and compare with the stored page hashes;
  - scan the marked objects on each changed page as roots, alongside the usual roots; marked objects stop the trace, so only young objects are traced;
  - XOR the hashes of new survivors into their pages;
  - keep region hit bits.
- Full collection: clear mark bits and region hits, mark everything, then rehash all marked objects.
- Hashing is single-threaded and per object, the simplest variant (A2's first), and is counted inside `mark_us`.
- Trace gains `minor`, `dirty` (pages) and `hash_us`.

Prediction (written before building):
- Self-build fixpoint holds with `LOW_GCMINOR=200`, with and without `LOW_GC=stress`.
- Fixtures under `LOW_GCMINOR=200 LOW_GC=stress` fail no case that passes under stress alone.
- With `LOW_GCWORKERS=1`, summed `mark_us` falls by at least 30% on at least 2 of selfbuild, lint and the wasm build, and on gcpause (dev).
- Max RSS rises by at most 15% on those.
- With default workers, summed `mark_us` falls on selfbuild by at least 10%.

Falsifiers:
- Any correctness failure absent from the baseline.
- Summed `mark_us` not lower on any of selfbuild, lint and wasm with one worker.
- RSS above +30% on any of them.

Correctness result:
- Self-build fixpoint: holds with defaults, with `LOW_GCMINOR=200` (34 of 41 collections minor), and with `LOW_GCMINOR=200 LOW_GC=stress` (54,219 of 54,235 minor). Pass.
- Fixtures under `LOW_GCMINOR=200 LOW_GC=stress`: 626 passed, 8 failed. 4 also fail under stress alone. The other 4 (locale_date_range, std_arrow, std_slhdsa, wrap_gc_trees) hit the 60 s limit. Prediction failed as written.
- Rerun of those 4 without a limit: all produce the expected output with minor on and off (wrap_gc_trees 103 s, std_arrow 135 s, std_slhdsa 187 s). The falsifier did not fire: they are slow, not wrong.
- Cause of the slowdown: dead old objects stay marked until a full collection, and a collection is minor while live words stay below 2x the larger of last full live and the 4 MB floor. In locale_date_range true live is about 300 KB, so up to 8 MB of dead old objects accumulate and every minor hashes all of them: summed mark time 0.9 s to 27.4 s, hashing 27.1 s of it.
- Regions: every live region's scan chunks are conservative roots of every collection, minor included, so region-to-heap edges are rescanned each time. Sticky hit bits only delay the return of quarantined chunks until a full collection. 4,600 stress minors ran with regions present.

Performance result (5 interleaved runs per arm, medians, summed `mark_us`; `a4-out/`). A third arm, minor off with a larger heap (`LOW_GCRATIO`), controls for minor mode's larger heap: dead old objects count as live until a full collection, so minor mode also collects less often.

| workload, workers | mark ms off | minor | larger heap | RSS MB off / minor / larger heap |
|---|---|---|---|---|
| gcpause, 1 | 2740 | 292 (hash 274) | 699 (ratio 400) | 146 / 241 / 229 |
| shuffle, 1 | 1630 | 495 (hash 288) | 803 (ratio 200) | 105 / 115 / 123 |
| selfbuild, 1 | 380 | 295 (hash 189) | 240 (ratio 200) | 114 / 156 / 131 |
| lint as registered, 1 | 118 | 89 (hash 50) | 90 | 69 / 78 / 67 |
| wasm as registered, 1 | 49 | 30 (hash 19) | 26 | 36 / 43 / 38 |
| `low lint compiler/low`, 1 | 310 | 195 (hash 122) | 191 | 92 / 102 / 101 |
| playground to wasm32, 1 | 269 | 222 (hash 144) | 183 | 185 / 199 / 194 |
| selfbuild, default | 82 | 252 (hash 190) | 53 | 116 / 159 / 132 |

Replication: `a4-run.sh` rerun from scratch into `a4-rep/` (5 runs every arm, gcpause memory-matched row included) reproduces every median within 5% (gcpause minor 293 ms, ratio-400 control 697 ms; selfbuild minor 292 ms, control 244 ms; default-worker selfbuild minor 253 ms, off 82 ms).

The lint and wasm workloads as registered (and as used in A2d) end in a compile error: `low lint compiler/low/main.low` stops at `undefined function: install_command`, and `compiler/low` cannot target `browser/wasm32` because it imports `std/sys`. They are the compiler front end up to an error. `low lint compiler/low` and `low build compiler/playground -target browser/wasm32` are the working versions.

Against the pre-registration:
- Mark time falls at least 30% with one worker on at least 2 of selfbuild, lint and wasm: fail. Selfbuild 22%, lint 24%, wasm 38% as registered; 37% and 17% for the working lint and wasm.
- gcpause at least 30%: pass, 89%.
- RSS rises at most 15%: fail. selfbuild +37%, wasm +19%, gcpause +65%.
- Default workers, selfbuild at least 10% lower: fail, 3.1x higher. Hashing is single-threaded (190 ms) against an 82 ms parallel full mark.
- Falsifiers: RSS above +30% fired on selfbuild. Mark time is lower on all three, and no output was wrong.

Verdict: falsified for the compiler. The larger-heap control matches or beats minor mode on every compiler workload at equal or lower RSS, so its gain there is heap sizing. On large pointer graphs it beats memory-matched controls: gcpause 292 ms against 699 ms of mark at 241 MB against 229 MB, wall 1.51 s against 1.92 s; shuffle 495 ms against 803 ms at 115 MB against 123 MB, wall 0.72 s against 1.04 s. gcpause max pause falls from 20.6 ms to 15.7 ms. Scope of the win: gcpause is the development benchmark and shuffle is the synthetic stable graph written for A2; no independent program shows it.

Where a minor's time goes (one traced run per arm, `a4split.py` over `a4-out/traces/`). Each minor's hash and residual mark (young survivors plus old objects on dirty pages) are expressed as a fraction of what a full mark of the same sticky set would cost at the off arm's mark ns per word:

| workload | mark ns/word (off) | hash / full | residual / full | sticky growth per minor | full collection reclaims of sticky set | A4 hash ns/word | A2d frozen H ns/word |
|---|---|---|---|---|---|---|---|
| gcpause | 1.16 | 0.38 | 0.01 | 4% | 0% | 0.47 | 0.20 |
| shuffle | 5.99 | 0.20 | 0.10 | 4% | none (grows) | 1.16 | 0.67 |
| selfbuild | 2.27 | 0.53 | 0.24 | 19% | 17% | 1.26 | 0.66 |
| `low lint compiler/low` | 2.71 | 0.49 | 0.21 | 13% | 21% | 1.28 | |
| playground to wasm32 | 1.55 | 0.69 | 0.24 | 21% | 20% | 1.02 | |
| selfbuild, default workers | 0.48 | 2.83 | 0.96 | 20% | 26% | 1.30 | |

Lessons:
- What decides the outcome is the residual: the young survivors and dirty-page objects a minor still marks. gcpause marks cheaply per word (1.16 ns) yet wins, because its residual is 1% of a full mark; the compiler's is 21% to 24%, with the sticky set growing 13% to 21% per minor. Marking cost per word does not explain the split.
- A4's hash is the first, per-object variant. Estimated per word it costs 1.9x to 2.3x more than A2c's frozen H, though the two are not like for like: A4's `hash_us` also covers the mark bitmap snapshot and hashing new survivors, and `a2.c` ran offline in C. With H's speed the compiler's minors would be roughly 0.5 of a full mark per collection instead of 0.70 to 0.93; whether that beats the larger-heap control is untested.
- Dead objects in the sticky set are a second-order cost: a full collection reclaims 17% to 26% of it on the compiler. Their larger effect is on heap sizing: they count as live, so minor mode collects less often, and that alone is what the larger-heap control reproduces.
- With default workers, selfbuild's residual costs 0.96 of a parallel full mark (1.84 ms against 1.69 ms per collection) and barely parallelizes (2.3 ms with one worker), while a full mark speeds up 4.8x. The residual tracks dirty pages, not young growth: correlation 0.96 (one worker) and 0.99 (default), about 1.2 us per dirty page in both, which is 74% and 90% of the residual. That is A4's serial `rt_gc_scan_dirty`, called once from the collecting thread, so this is a limit of A4 as built, not of barrier-free detection. Dirty pages are heavy-tailed: median 26 per minor, a few minors with thousands.
- The 4 MB floor in the minor test lets small heaps hold up to 8 MB of dead objects (the fixture timeouts).
- Every pre-registration needs an exit-0 check on each workload and a tuned heap-size control arm.

## A5 What bounds Low's parallel mark (ceiling measurement, pre-registered before measuring)

Prior-art queries:
- "parallel mark garbage collection scalability memory bandwidth bound many-core analysis": Boehm's scalability notes (shared bus), Auhagen, Bergstrom and Fluet on NUMA-local GC for Manticore, Gidra et al. and Suo et al. (EuroSys 2018) on HotSpot's parallel GC scaling.
- "garbage collector tracing memory-level parallelism prefetch FIFO mark loop cache misses": Boehm (ISMM 2000) prefetch on grey, Cher, Hosking and Vijaykumar (ASPLOS 2004) FIFO prefetch buffer, Garner et al. edge-order traversal.
- "heap traversal pointer chasing DRAM bandwidth utilization GC workers stall cycles measurement": MIT RAM-latency paper (APSYS 2013), Maas et al. hardware GC (ISCA 2018), near-memory GC.

Low's mark loop already has a 32-entry FIFO prefetch ring per worker (the Cher et al. design). No source found measures, for a collector with such a ring, the fraction of the machine's random-access line throughput its parallel mark reaches, so the answer decides what kind of mechanism is worth building.

Method:
- Machine ceiling: a C microbenchmark reading random 64-byte lines from a 1 GB buffer with k threads, each keeping M independent loads in flight (M from 1 to 32), and a sequential read stream. Reports lines per second.
- Mark rate: `perf record -e longest_lat_cache.miss` and `-e cycles` on gcpause (large stable graph) and selfbuild, with 1, 2, 4, 6 and 12 mark workers. L3 misses attributed to the runtime's mark symbols, divided by summed `mark_us`, give the mark's DRAM line rate. Cycles in work distribution symbols (`rt_gcw_acquire`, `rt_gcw_donate`, idle waits) against the mark loop give the distribution share.

Predictions:
1. At 12 workers, gcpause's mark reaches less than 50% of the machine's random-line throughput at 12 threads.
2. At 12 workers, at least 20% of GC worker cycles on selfbuild go to work distribution and waiting.
3. With one worker, gcpause's mark keeps fewer misses in flight than the ring allows: its line rate is below the microbenchmark's at M = 8 with one thread.

Falsifiers:
- gcpause's 12-worker mark at or above 80% of random-line throughput (the mark is bandwidth-bound here, and the mechanism must cut traffic).
- Work distribution under 5% of GC worker cycles on selfbuild at 12 workers.

Result (`a5mlp.c`, `a5measure.py`, `a5-out/`; L3 misses from `longest_lat_cache.miss`, attributed by symbol share of the event count):

Machine ceiling (1 GB buffer, huge pages; THP is set to always and a gcpause process holds 110 MB of huge pages mid-run). Each line is one 64-byte read; the counter column is counted misses per useful line, from runs of 40 M steps per thread with setup subtracted:

| threads | chains per thread | M useful lines/s | counter per line |
|---|---|---|---|
| 1 | 8 | 109 | 2.1 |
| 1 | 16 | 151 | 1.4 |
| 4 | 16 | 386 | 1.22 |
| 6 | 16 | 403 | 1.18 |
| 12 | 16 | 395 | 1.21 |

Random-line throughput saturates at about 395 M lines/s from 4 threads, about 475 M counted misses/s (30 GB/s of DRAM traffic); a 12-thread sequential read reaches 35 GB/s. Without huge pages, one thread reaches 28% less.

Mark (counted L3 misses in mark symbols per second of summed `mark_us`):

| workload | workers used | mark ms | M counted lines/s | share of saturated traffic | distribution share of on-CPU worker time | worker busy |
|---|---|---|---|---|---|---|
| gcpause | 1 (any setting) | 2,760 | 79 | 17% | 0 | 1.00 |
| selfbuild | 1 | 401 | 100 | 21% | 0 | 0.99 |
| selfbuild | 2 | 212 | 184 | 39% | 0.8% | 0.99 |
| selfbuild | 4 | 142 | 276 | 58% | 1.2% | 0.98 |
| selfbuild | 6 | 102 | 374 | 79% | 2.4% | 0.96 |
| selfbuild | 11.8 | 84 | 402 | 85% | 5.9% | 0.86 |

Counted misses per CPU microsecond of marking fall from 101 with one worker to 42 with twelve: each worker gets half the memory throughput once the channel fills, while mark CPU doubles (399 to 796 ms) for the same work.

gcpause marks with one worker whatever `LOW_GCWORKERS` says: the worker count is capped by the OS threads the program has (`rt_nm`), and gcpause never spawns. This is known: issue 267, with a fix on `fix/issue-267-mark-workers` (2e63dc5e) that starts OS threads up to the CPU count after a collection leaving 8 MB live. It also means A4's single-worker arms and Low's own bench programs run in the regime that fix removes.

Stall attribution (`a5measure.py` and `a5events.py`; `cycle_activity` events attributed to the mark symbols; `a5-out/stalls.txt`, `a5-out/stall-kinds.txt`):

| run | mark G cycles | L3-miss stall share | memory stall share (any level) |
|---|---|---|---|
| selfbuild, 1 worker | 0.99 to 1.10 | 0.16 | 0.39 |
| selfbuild, 6 workers on 6 distinct cores | 1.37 to 1.53 | 0.21 | 0.42 |
| selfbuild, 12 workers | 2.33 to 2.53 | 0.19 | 0.47 |
| shuffle, 1 worker | 4.20 | 0.88 | |
| gcpause, 1 worker | 6.76 | 0.03 | |

- From 1 to 6 workers on distinct cores, mark cycles grow by about 0.4 to 0.5 G. L3-miss stalls account for about a quarter of that, memory stalls at any level for about half, coherence hits on modified lines and memory-ordering clears for nothing measurable, and locked loads number about 9 M.
- selfbuild's DRAM traffic near the channel's limit is mostly hardware prefetch: the lines arrive without stalling the cores. A line rate near the ceiling is not, by itself, evidence that marking is bandwidth-bound.
- With 6 workers on 6 distinct cores the mark runs 3.8x to 4.0x faster than with one; going to 12 hyperthreads adds 15%.

Against the pre-registration:
1. gcpause's 12-worker mark below 50% of random-line throughput: passes as written (17%) but says nothing, because gcpause never uses a second worker. On selfbuild the counted rate is 85% of saturated random-line traffic and about 73% of the sequential read rate, whose counter was not calibrated, so the falsifier's threshold sits between the two readings; the stall attribution decides it instead, and it shows the mark is not stalled on DRAM.
2. At least 20% of worker CPU in work distribution on selfbuild at 12 workers: fail, 5.9% of on-CPU time, with workers off-CPU about 14% of the mark. The falsifier (under 5%) did not fire.
3. One-worker gcpause below the one-thread, 8-chain rate: passes as written (79 against about 230 counted lines/s) but says nothing, since gcpause's mark stalls on L3 misses for 3% of its cycles. On shuffle, which stalls for 88%, the single worker reaches 202 M counted lines/s against about 212 to 230 for one thread with 8 to 16 misses in flight: the 32-entry prefetch ring already gets close to the single-thread limit.

Verdict: on this machine Low's parallel mark is neither bandwidth-bound nor distribution-bound. It loses efficiency gradually to memory-hierarchy contention at every level, scales 3.8x to 4.0x on 6 cores, and its single-worker latency-bound case is already at the single-thread memory-level-parallelism limit. The many-core bandwidth question cannot be settled on a 6-core, 2-channel desktop; a machine with many cores per memory channel is needed to see whether the channel saturates first.

Lessons:
- Calibrate a cache-miss counter against a workload with known line counts before reading rates: at low load the counter records 2.1 misses per useful line (adjacent-line prefetch), at saturation 1.2.
- `perf record` ignored the requested period for the cache-miss event (it used 100,003); attribute by share of the event count, not by samples times period. Rare events (coherence hits) never reach that period, so their zero bounds them rather than measuring them.
- Traffic near the limit and being limited by traffic are different claims; stall attribution separates them.
- Check that the parallelism a workload is assumed to have is actually used, before predicting about it.
- A traffic-per-marked-byte figure from counted lines is an upper bound: selfbuild's 1.9 bytes per marked byte counts prefetch extras and divides by marked bytes that include leaf objects the mark never reads.

## A6 A4 and A5 rerun with the issue 267 fix (pre-registered before building)

Change: the worktree gains the runtime and fixture parts of 2e63dc5e (`fix/issue-267-mark-workers`): after a collection that leaves `PAR_MIN` live words, `rt_gcw_recruit` starts OS threads until min(CPUs, 16) are awake, so single-threaded programs mark with every worker. A4's minor collection is unchanged: its hashing and dirty-page scan stay single-threaded.

A6a, A4 under parallel marking (default workers; arms off, minor `LOW_GCMINOR=200`, and a larger heap without minors matched to minor mode's peak RSS):
- Prediction: minor mode's mark-time win on gcpause and shuffle shrinks from 9.4x and 3.3x (one worker) to under 2x, because full marks now run on up to 12 workers while hashing stays on one; on gcpause, minor mode's summed mark time is no lower than the memory-matched larger heap.
- Falsifier: minor mode still beats the memory-matched control by at least 2x on gcpause and shuffle.

A5b, bounds of a latency-bound parallel mark (shuffle and gcpause with 1, 2, 4, 6 and 12 workers; `a5measure.py`, stall attribution by symbol):
- Prediction 1: shuffle's mark, which stalls on L3 misses for 88% of its cycles with one worker, reaches at least 80% of the saturated random-line traffic (380 M counted lines/s) by 6 workers, and its speedup from 6 to 12 workers is under 1.2x.
- Prediction 2: shuffle's L3-miss stall share stays above 70% at every worker count.
- Prediction 3: gcpause's mark, which streams with 3% L3-miss stalls, scales at least 4x from 1 to 6 workers on distinct cores.
- Falsifiers: shuffle's mark below 50% of saturated traffic at 12 workers while its stall share stays above 70% (latency-bound but not filling the channel, so miss parallelism per worker, not bandwidth, is the limit); gcpause speedup from 1 to 6 workers below 2.5x.

Gates: self-build fixpoint, `gc_mark_threads` and the fixture suite under defaults.

Gates: self-build fixpoint holds; fixtures under defaults 635 passed, 0 failed (`gc_mark_threads` included). With 12 workers allowed, gcpause marks with all 12 in 154 of its 166 collections.

A6a result (`a6-run.sh`, `a6-out/`; default workers, 5 interleaved runs, medians):

| workload | arm | mark ms | RSS MB | wall s |
|---|---|---|---|---|
| gcpause | off | 3230 | 146 | 4.59 |
| gcpause | minor | 299 (hash 277) | 241 | 1.48 |
| gcpause | ratio 400 | 816 | 230 | 1.99 |
| gcpause | ratio 600 | 544 | 290 | 1.69 |
| shuffle | off | 725 | 112 | 0.93 |
| shuffle | minor | 444 (hash 287) | 115 | 0.66 |
| shuffle | ratio 200 | 359 | 131 | 0.58 |

Against the pre-registration:
- Minor mode's win under 2x on gcpause and shuffle: shuffle passes (3.3x to 1.6x, and a 16 MB larger heap beats it); gcpause fails, the win grows from 9.4x to 10.8x because its full mark got slower with more workers (below).
- gcpause minor no lower than the memory-matched heap: fail, 299 ms against 816 ms at 241 MB against 230 MB.
- Falsifier (still at least 2x over the memory-matched control on both): did not fire; shuffle is beaten by a slightly larger heap.

A5b result (`a6-out/a5b/`):

| workload | workers | mark ms | M counted lines/s | share of saturated traffic | L3-miss stall share | worker busy |
|---|---|---|---|---|---|---|
| shuffle | 1 | 1804 | 200 | 42% | 0.88 | 1.00 |
| shuffle | 2 | 1130 | 293 | 62% | 0.83 | 0.83 |
| shuffle | 4 | 826 | 358 | 75% | 0.86 | 0.67 |
| shuffle | 6 on distinct cores | 753 | 373 | 79% | 0.89 | 0.60 |
| shuffle | 12 | 762 | 353 | 74% | 0.92 | 0.52 |
| gcpause | 1 | 2926 | 75 | 16% | 0.03 | 1.00 |
| gcpause | 2 | 3356 | 67 | 14% | 0.03 | 0.51 |
| gcpause | 6 on distinct cores | 3370 | 67 | 14% | 0.02 | 0.17 |
| gcpause | 12 | 3407 | 67 | 14% | 0.02 | 0.09 |

Calibration checks run after the sweep:
- Counted lines are not useful lines for shuffle. The marked heap is 2.26 GB over 37 collections, which is 35 M to 85 M lines depending on how many lines each 40-byte node touches, while the counter attributes 362 M lines to the mark at 1 worker and 269 M at 12. Start- and mark-bitmap lookups (rt_find) and prefetch account for the rest, in a ratio that moves with worker count, so the "share of saturated traffic" column cannot be read as channel occupancy.
- Counter-free check (`a6-cochase.sh`, `a6-out/cochase.txt`): a 16-chain pointer chase on core 5 runs at 150 M lines/s alone, 137 M beside a 1-worker shuffle and 107 M to 121 M beside a 4-worker shuffle on cores 0 to 3. When 4 chase threads saturate the channel, each gets 98 M; a fifth thread on a channel already full would get about 80 M. The 4-worker mark leaves the channel headroom.
- Same binary without perf (`a6-out/same-binary.txt`, the research worktree, which is the issue 267 fix plus instrumentation, not the `fix/issue-267-mark-workers` branch itself): gcpause marks in 2830 ms with 1 worker, 3260 ms with 2, 3290 ms with default workers (16% slower than 1); wall 4.26 s against 4.69 s (10%); max pause 22.8 ms against 32.7 ms.

Against the pre-registration:
1. shuffle at least 80% of saturated traffic by 6 workers, and under 1.2x speedup from 6 to 12: as measured, the first part narrowly fails (76% unpinned, 79% on distinct cores) and the second passes (1.01x). The metric is uncalibrated for shuffle (above), so neither reading supports the bandwidth-bound hypothesis the prediction stood for.
2. shuffle's L3-miss stall share above 70% at every worker count: pass (0.83 to 0.92).
3. gcpause at least 4x from 1 to 6 workers: fail. It runs 0.87x, and the falsifier (below 2.5x) fired.
- shuffle falsifier (below 50% of saturated traffic at 12 workers with stalls above 70%): did not fire on its stated metric. The condition it was written to detect, a latency-bound mark that stops scaling without filling the channel, is what the co-run check shows.

Verdict:
- shuffle's mark time plateaus from 4 workers (826, 772, 762 ms at 4, 6, 12) while its stalled CPU doubles, but the channel is not full at 4 workers: a separate chase loses 24% of its rate, not the roughly 45% a saturated channel would cost it. The plateau is a per-worker limit, not DRAM bandwidth. Workers are off-CPU for 33% to 48% of the mark (busy 0.67 to 0.52), and the rest of their time is stalled on L3 misses; which of idle time and per-core miss capacity dominates is not yet measured. The many-core bandwidth question stays open on this machine.
- A chain-shaped heap cannot use the workers. gcpause is one singly linked list; one worker holds all the work (busy utilisation 1/workers). The parallel mark loop with atomic mark-bit sets spends 16% more mark cycles than the single-worker loop at 2 workers (8.06 G against 6.92 G), and idle workers raise that to 33% at 12 (9.21 G). In the same binary, recruiting workers makes gcpause's mark 16% slower, its wall 10% slower and its longest pause 43% longer. Siebert (ISMM 2008, "Limits of parallel marking garbage collection") describes the depth limit; the regression is specific to Low's fix.
- A4 under parallel marking: on shuffle a slightly larger heap beats minor mode. On gcpause minor mode wins as it did with one worker (292 ms against 699 ms for the memory-matched heap in A4), because the full mark cannot use workers either way; the parallel loop's 16% adds little to that gap.

Lessons:
- A cache-miss counter calibrated on one access pattern does not transfer to another: shuffle's counted lines per useful line were several times the microbenchmark's and changed with worker count. A co-runner with a known solo rate measures channel occupancy without a counter.
- A plateau with rising stalls is not by itself evidence of saturation; on this machine it came with a channel that had headroom.

Engineering follow-ups for Low (not novel; production collectors size GC worker counts dynamically, for example HotSpot's UseDynamicNumberOfGCThreads):
- Keep the mark loop in the single-worker form until a second worker takes work, so chain-shaped heaps pay nothing for recruited threads.
- Measure the idle share of shuffle's workers (steal attempts, sleep time per collection) to split the plateau between work starvation and per-core miss capacity.
