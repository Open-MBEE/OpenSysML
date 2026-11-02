# Performance: release 0.12.0 against release 0.11.0

The release-gate measurement of 0.12.0 against the `v0.11.0` tag on
`main` (`ddf4df00037d6ddbfb1fa9d743a4389f75c9e2dc`), in two passes. The
first pass measured the head of `develop` at
`1c6bb197227bfc4b8a8f33bda51c86093d56b933` (2026-10-30), which holds
#1699 (the export element-ID cost, merged at `101d2f267`); it found the
regressions of [Summary](#summary), each traced to its merge and fixed in
one of the seven pull requests of [Fixes](#fixes), which all merged into
`develop`. The second pass measured the release candidate itself —
`cc1d477ba9843ce8e36378a87f49ae4005f9a2b7`, `develop` at `ab88ea950`
(the merge of #1747, the last fix) with #1730 on top — with the same
full method, then re-probed the export rows on `5e0322e0c` and again on
`d01e4f49c` when #1760 and #1774 landed on them; it is
[The release candidate](#the-release-candidate), which also accounts
for every merge up to `develop` `d16ca71e0`, re-checking the runtime
rows at `1e34d9d95` when #1777 landed on them, the semantics rows at
`9a4ffcd83` when #1784 did, the export rows at `e022cbca0` when
#1783 did and at `86fa09ec9` when #1792 did, all of them at
`a37809a6c` when #1795 and #1791 did, the runtime and gRPC rows at
`df2bc173c` when #1778 did, and the load, derived and export rows at
`d16ca71e0` when #1797 and #1785 did. The
first pass stays in the record as the pre-fix measurement so that what
the fixes gave back is visible row by row. The record follows the
method of the [0.11.0 against 0.10.1](performance-release-0.11.0-vs-0.10.1.md)
record: every benchmark on both revisions, `benchstat` over six runs,
the rows the machine leaves in doubt re-run interleaved on a quiet
machine, whole-binary timings on generated and real models, process
start and session floor, and the `tests/stressmodel` constellation. See
[performance](../internals/performance.md) for what the figures mean.

The measurement began at `0c1fd6dbd03b54acc719cb46b0a630acb1a70a98`
(2026-10-29, 229 first-parent merges after `v0.11.0`), with #1699's head
`2350e3fae988c7ac3d5f9b7db1ea02617b08fa14` merged on top locally (merge
base `8e6bd003c1c229ce104fc66f7771344749d65d50`) because the maintainer
named it part of 0.12.0; #1699 then merged, with #1700, #1701 and #1705,
and every table of the first pass is taken on `1c6bb1972`, four
first-parent merges after the pinned head, as the pre-fix candidate.

## Three columns, and a fourth

0.11.0 was built with `go1.25.11`; `develop` builds with `go1.27.3`
(`go.mod` `toolchain`). The toolchain change dominates a naive timing
comparison, so three binaries are measured:

1. **0.11.0 as released** — the `v0.11.0` source built with `go1.25.11`,
   what a 0.11.0 user runs.
2. **0.11.0 rebuilt** — the same source built with `go1.27.3`.
3. **0.12.0 candidate, before the fixes** — `1c6bb1972` built with
   `go1.27.3`; "the candidate" in every section up to [Fixes](#fixes).
4. **0.12.0 release candidate** — `cc1d477ba` built with `go1.27.3`
   (export rows re-probed at `5e0322e0c` and `d01e4f49c`), in
   [The release candidate](#the-release-candidate) only.

A source regression or improvement is column 2 against column 3 (and,
for the release, column 2 against column 4): the same toolchain,
different source. What a user upgrading from 0.11.0 feels is column 1
against column 4, and column 1 against 3 in the first pass. Column 1 against column 2 is the
toolchain alone. Every statement below says which columns it rests on.
Allocations and bytes per operation are exact for a given binary, but
they are *not* toolchain-independent here: the same 0.11.0 source
allocates differently under `go1.27.3` than under `go1.25.11` on many
rows (the `WriteAPIJSON` rows +20–28 %, `ExecuteState` +21 %,
`ConvertAPIJSON/128` +23 %), so an allocation statement about the
source is also column 2 against column 3, never column 1 against 3.

## Method

Three detached worktrees, one per revision, built with `make build`,
with the pilot and training corpora copied in so the corpus-backed rows
run on every side. Every benchmark package is run with
`-run '^$' -bench . -benchmem -count 1`, six rounds, the three
configurations interleaved within each round; each round's output is
an append-only file and `benchstat` compares the six files per
configuration. `tests/stressmodel` runs the same way with
`ConvertAPIJSON/satellites=512` skipped, as the previous records did.
The machine is an 8-vCPU `Intel(R) Xeon(R) Processor` (31 GiB),
`GOMAXPROCS=8`, logged into every round's file; sec/op is comparable
only between the columns of this record, never with the earlier
records.

Rows known to fail from the earlier records, recorded before measuring
and failing identically on all three columns, so none is a regression:
the five `tests/codegen` `CompiledCalc` rows (the C target does not take
unbounded integers; `SumTo` at a million exceeds the step budget),
`tests/perf` `Analyze/vehicle` (unresolved `vehicle_b`, since #1062),
and `AnalyzeSplitPerDocument` and `PlaneResidency` in `tests/stressmodel`
(the Real-literal rounding diagnostics; `ErrRecordStale`). Those packages
exit non-zero on every revision; every other row of theirs is measured.

## Summary

Six rounds, 156 rows per round on 0.11.0 and 158 on the candidate, all
three columns on one host. Medians; a percentage is the candidate against
0.11.0 rebuilt (column 2 → 3) unless it says otherwise. Bytes and
allocations per operation are exact (`± 0 %` on every row below); the
sec/op of the runtime package came out with 50–290 % spreads on the
loaded machine and are decided by the quiet re-run of [Re-runs](#re-runs),
not here.

The candidate is faster than 0.11.0 nearly everywhere a user would
notice (column 1 → 3): loading the 1 000-element generated model 148 → 114 ms,
the 95 927-element stress model 9.09 → 8.73 s, a REPL snippet 1.44 → 1.05 s,
`ConvertAPIJSON` of 24 167 elements 57.9 → 49.9 s, `WriteAPIJSON` −10…−37 %,
action chains −35…−59 %, `LookupQualified` 150 → 43 ns per lookup with
no allocation. It allocates more on a handful of paths, each traced to the
merge that introduced it, and takes 31 MiB more resident memory to
validate the largest generated model. Those are the rows below.

### Source regressions (column 2 → 3)

| Row | Metric | 0.11.0 | 0.11.0 rebuilt | 0.12.0 candidate | 2 → 3 | Cause | Fix |
|---|---|---|---|---|---|---|---|
| repl `LoadModel/elements=0` | allocs/op | 2 022 | 2 022 | 2 155 | +6.6 % (B/op +8.3 %) | #1462, the duplicate-library check kept a list per library package name | `perf/duplicate-libraries-walk` → 2 038 (#1746) |
| repl `LoadModel/elements=1000` (and /250 +5.4 %, /4000 +5.0 %; +3.5–5 % B/op on every load-heavy row: stress `Load`, `LoadFiles`, `OpenSplit`, `ValidateSplit`, `EditImported`, `ImportValues`, `Analyze/synthetic`, `WorkspaceEdit`, `REPLSubmitSnippet`) | B/op | 55.39 Mi | 55.52 Mi | 58.31 Mi | +5.0 % | #1478 (bisected), the `typed by` notation pass swept every document's expressions | `perf/typed-by-sweep` → 55.8 Mi (#1744) |
| runtime `AssignmentLoopStep` | B/op | 899.7 Ki | 899.7 Ki | 948.0 Ki | +5.4 % | #1514 and #1461 grew the evaluation context from 152 to 184 bytes | `perf/eval-context-size` → 926 Ki (#1745) |
| runtime `DerivedReadWriteRead` | B/op · allocs/op | 2 688 · 67 | 2 688 · 67 | 3 328 · 71 | +23.8 % · +6.0 % | #1619 (bisected) marks what a derived value stands on, copying the footprint's reads; #1514 for 64 B | `perf/derive-marks-footprint` → 3 104 · 69 (#1747), with `perf/eval-context-size` (#1745) |
| perf `GRPCVerifyConstraint` | allocs/op | 14 390 | 14 090 | 16 300 | +15.7 % | #1508 and #1512, the binding walks | `perf/binding-walk-allocs` → 14 887 (#1742) |
| grpc `InstantiateWarmModel` | allocs/op | 1 990 | 1 960 | 2 170 | +10.5 % (B/op +6.3 %) | the binding walks, as above | the same → 1 996 (#1742) |
| perf `FeaturesOf` | B/op | 261.7 Mi | 261.2 Mi | 295.8 Mi | +13.3 % (sec/op −7 %) | #1626, the memo of related features recorded every empty relation | `perf/related-features-memo` → 271.2 Mi (#1743) |
| perf `Instantiate`, cold, pinned at two iterations | B/op | — | 221 MB | 252 MB | +14 % (allocs −4.6 %) | #1626 (bisected) | the same → 230 MB (#1743) |
| perf `BatchSatisfy` | B/op | 456.8 Mi | 454.9 Mi | 491.4 Mi | +8.0 % | #1626 (bisected) | the same → 475.5 Mi (#1743) |
| perf `ExecuteActionFreshContext` | allocs/op | 376 | 388 | 424 | +9.3 % | #1619 (bisected); the marks are the correctness cost — `perf/derive-marks-footprint` leaves this row at 424 over six rounds, so none of it is the copy | none |
| perf `ActionChain/chain10` | B/op | 200.6 Ki | 198.1 Ki | 222.9 Ki | +12.5 % (allocs −18 %) | #1473 (bisected), the performance-shape memo of an earlier fix: fewer, larger allocations | not a regression to fix: `chain100` and `chain1000` allocate 18–26 % less |
| whole binary, `-validate` of 12 086 generated elements | peak RSS | 161 MiB | 162 MiB | 193 MiB | +31 MiB | #1441 (bisected): the parse trees of a submission were held through its analysis for the roots an unresolved reference would be looked up by | `perf/release-preparse-trees` → 164 MiB (#1741) |

One regression was found already fixed: bisecting `LoadModel/elements=1000`
past #1478 lands on #1528 (+1.7 MB per load, the dependency search), which
#1624 removed from the path of a file that resolves; the candidate shows no
bytes under it.

### Toolchain alone (column 1 → 2)

`go1.27.3` makes the same 0.11.0 source faster on most rows (`LoadModel/4000`
693 → 552 ms, lsp `ReferencesWarm` 275 → 192 µs, `ExpandWildcardImports`
44 → 33 ms, `Parse/vehicle` 2.82 → 1.98 ms) and changes its allocation
counts: `WriteAPIJSON` +20–28 % allocations and up to +29 % bytes, `ExecuteState` and `StateLoop` +21 %,
`ConvertAPIJSON/128` +23 % allocations. None of those is a source change;
every allocation statement in this record is column 2 against 3.

## Package benchmarks

Column 2 → 3, medians of six. The runtime package's sec/op are omitted
(see [Re-runs](#re-runs)).

| Package · row | Metric | 0.11.0 rebuilt | 0.12.0 candidate | Δ |
|---|---|---|---|---|
| runtime `NewRootFrame` | sec/op · B/op · allocs | 10.6 µs · 12.5 Ki · 36 | 849 ns · 2.4 Ki · 21 | −92 % · −81 % · −42 % |
| runtime `IntegerCollectionSum` | sec/op | 1.11 ms | 855 µs | −23 % |
| grpc `ParseFileColdShared` | sec/op | 7.77 ms | 7.51 ms | −3 % |
| lsp `ReferencesWarm` | sec/op | 191.5 µs | 166.3 µs | −13 % |
| lsp `FormatEdits` | sec/op | 926 µs | 1.01 ms | +10 % (time only) |
| repl `Diagnostics/attributes=50` | sec/op | 114 µs | 94 µs | −17 % |
| repl `LoadModel/elements=1000` | sec/op | 130.2 ms | 114.4 ms | −12 % |
| repl `LoadModel/elements=4000` | sec/op | 552 ms | 510 ms | −8 % |
| repl `RunCalc/*` | B/op | 4.6 Ki | 4.8 Ki | +3.4 % |
| migrate `WriterSiblingBlocks` | sec/op | 2.15 ms | 2.45 ms | +14 % (time only) |
| libs `DecodeSnapshot` · `IndexLibrary` · `ExpandWildcardImports` · `SetDigest` | sec/op | 14.4 · 17.4 · 32.9 · 1.46 ms | 12.1 · 15.0 · 27.2 · 1.24 ms | −14…−17 % |
| model `AnalyseResolved` · `AnalyseUnresolved` | sec/op | 2.23 · 25.8 ms | 1.61 · 20.0 ms | −28 % · −23 % |
| export `WriteAPIJSON/*` (8 rows) | sec/op · allocs | | | −10…−37 % · −16…−26 % |
| perf `ActionChain/chain10` · `chain100` · `chain1000` | sec/op | 137 µs · 948 µs · 17.6 ms | 90 µs · 616 µs · 7.19 ms | −35 % · −35 % · −59 % |
| perf `ActionLoop/for10` · `for100` | sec/op · B/op | 54.7 µs · 35.2 Ki, 205 µs · 75.0 Ki | 29.8 µs · 21.1 Ki, 176 µs · 60.9 Ki | −45 % · −40 %, −14 % · −19 % |
| perf `ExecuteAction` | sec/op · allocs | 91.8 µs · 254 | 68.6 µs · 196 | −25 % · −23 % |
| perf `ExecuteState` · `StateLoop/*` | allocs | 14.97 k · 14.95 k…1.456 M | 12.49 k · 12.47 k…1.211 M | −17 % |
| perf `GRPCEvaluate` | sec/op · allocs | 12.4 µs · 125 | 10.2 µs · 129 | −18 % · +3 % |
| perf `GRPCParseFileUncached` | sec/op | 310 ms | 244 ms | −21 % |
| perf `GRPCVerifyConstraint` | sec/op | 1.86 ms | 2.05 ms | +10 % |
| perf `ImportValues/*` (8 rows) | sec/op · B/op | | | −8…−17 % · +3.6…+5.9 % |
| perf `IndexAddExpand/synthetic` | sec/op | 48.4 ms | 54.6 ms | +13 % (time only) |
| perf `LookupQualified` | ns per lookup · allocs | 149.7 · 7 | 42.6 · 0 | −72 % |
| perf `REPLLoadFile` | sec/op · B/op · allocs | 1.75 s · 621.9 Mi · 5.76 M | 1.53 s · 603.1 Mi · 5.48 M | −13 % · −3 % · −5 % |
| perf `REPLSubmitSnippet` | sec/op · B/op | 1.43 s · 564.2 Mi | 1.05 s · 588.2 Mi | −27 % · +4.3 % |
| perf `WorkspaceEdit/synthetic/reindex+diagnostics` | sec/op · B/op | 2.39 s · 513.5 Mi | 1.78 s · 536.4 Mi | −26 % · +4.5 % |
| perf `BatchConstraints` | sec/op · B/op | 13.4 ms · 3.37 Mi | 15.0 ms · 3.54 Mi | +12 % · +5 % |
| perf `Analyze/synthetic` | sec/op · B/op | 1.48 s · 490.8 Mi | 1.53 s · 514.0 Mi | +3 % · +4.7 % |

`Analyze/vehicle`, the five `CompiledCalc` rows and the two stress rows
named in [Method](#method) fail on every column and carry no figure.

## The stress constellation

`tests/stressmodel`, column 2 → 3, medians of six, with column 1 where it
reads differently. Every load-side row allocates 3.3–4.9 % more bytes
(#1478, above) and runs 4–10 % faster; the fleet rows are inside the spread.

| Row | 0.11.0 | 0.11.0 rebuilt | 0.12.0 candidate | 2 → 3 |
|---|---|---|---|---|
| `Load/satellites=512/elements=95927` | 9.09 s · 2 486 Mi | 9.22 s · 2 507 Mi | 8.73 s · 2 603 Mi | −5.3 % · +3.8 % |
| `Load/satellites=128` | 1.98 s | 2.17 s | 1.97 s | −9.5 % |
| `LoadFiles/satellites=512/files=6` | 10.33 s · 2 452 Mi | 10.16 s · 2 472 Mi | 10.62 s · 2 571 Mi | +4.6 % · +4.0 % |
| `OpenSplit/satellites=512/cold` · `/warm` | 5.27 s · 1.06 s | 4.57 s · 1.10 s | 4.29 s · 1.04 s | −6.2 % · −5.2 % |
| `OpenSplit/satellites=32/warm` | 134 ms | 170 ms | 133 ms | −22 % |
| `ValidateSplit/satellites=512/jobs=1` · `/jobs=8` | 11.63 s · 4.80 s | 10.66 s · 4.43 s | 10.03 s · 4.75 s | −5.9 % · +7.3 % |
| `EditImported/satellites=512/files=6` | 10.42 s · 2 016 Mi | 10.28 s · 2 021 Mi | 9.77 s · 2 120 Mi | −5.0 % · +4.9 % |
| `EditBeside/satellites=128` · `/512` | 4.59 · 16.7 ms | 4.08 · 15.8 ms | 4.46 · 16.5 ms | +9.3 % · +4.6 % (time only) |
| `HydratePlane/satellites=512` | 920 ms | 995 ms | 916 ms | −7.9 % |
| `FleetInstantiate/satellites=512/elements=3947` | 87.3 ms | 74.1 ms | 78.6 ms | +6.1 % (time only) |
| `FleetSatisfy/satellites=512/assertions=108` | 7.43 ms · 53.1 k | 7.72 ms · 49.9 k | 7.21 ms · 47.9 k | −6.7 % · −3.9 % allocs |
| `Satisfy/satellites=512/assertions=1536` | 150 ms · 646 k | 133 ms · 616 k | 141 ms · 590 k | +6.1 % · −4.2 % allocs |
| `ConvertAPIJSON/satellites=128/elements=24167` | 57.9 s · 129.8 M allocs | 56.4 s · 159.8 M | 49.9 s · 132.2 M | −11.5 % · −17.2 % |
| `ConvertAPIJSON/satellites=32` | 12.8 s | 12.1 s | 11.2 s | −7.3 % |

## Whole binary

`bin/sysml` from each configuration, nine interleaved runs each, medians
of wall time and peak resident set (`/usr/bin/time`), one binary at a time
on the measurement host. The validation reports of the Apollo 11 model are
byte-identical across the three binaries.

| Workload | 0.11.0 | 0.11.0 rebuilt | 0.12.0 candidate |
|---|---|---|---|
| binary size, `sysml` · `sysml-lsp` | 53.5 · 28.9 MB | 54.0 · 29.5 MB | 59.6 · 36.3 MB |
| process floor, `-e 1` | 0.02 s · 71 MiB | 0.02 s · 71 MiB | 0.02 s · 73 MiB |
| `examples/action-executor-demo.sysml` | 0.02 s · 73 MiB | 0.02 s · 72 MiB | 0.03 s · 75 MiB |
| `examples/disposal-robot-demo` | 0.04 s · 80 MiB | 0.04 s · 79 MiB | 0.04 s · 81 MiB |
| generated, 3 254 elements | 0.11 s · 104 MiB | 0.10 s · 103 MiB | 0.09 s · 106 MiB |
| generated, 6 198 elements | 0.16 s · 117 MiB | 0.15 s · 120 MiB | 0.15 s · 140 MiB |
| generated, 12 086 elements | 0.29 s · 161 MiB | 0.25 s · 162 MiB | 0.24 s · 193 MiB |
| Apollo 11, 28 files | 0.31 s · 168 MiB | 0.30 s · 167 MiB | 0.29 s · 180 MiB |

The floor is 2 MiB higher for a 5.6 MB larger binary; the time is the
same or better on every workload; the resident set grows with the model,
+31 MiB at 12 086 elements. That growth is not the allocation growth of
#1478 (a binary with `perf/typed-by-sweep` applied peaks at the same
193 MiB): the heap the process holds at exit is 57.6 MB on 0.11.0 and
79.7 MB on the candidate, and the 22 MB between them is the parse trees
of the loaded file, which #1441 keeps referenced through the analysis
(for the root namespaces an unresolved reference would be looked up by)
where 0.11.0 dropped them once the file was accepted. Under `GOGC=25`,
which keeps the resident set near the live heap, 0.11.0 rebuilt peaks at
146 MiB, the merge before #1441 at 147, #1441 at 165 and the candidate at
166 (medians of three cold runs each, binaries built as `make build`
builds them); under `GOMEMLIMIT=120MiB` the candidate takes 0.44 s to
0.11.0's 0.28 s, the collector working to hold the limit.
`perf/release-preparse-trees` lets the trees go and re-parses only when a
reference is left unresolved: 150 MiB at `GOGC=25`, 0.25 s under the
limit, with the load's time and allocations unchanged; every binary
completes under `ulimit -v 1200000` (the acceptance rows are in that PR).

### Export element IDs

0.11.0 does not take `elementId` as a query property, so #1699's rows are
develop before it (`0c1fd6dbd`) against the candidate, three runs each,
`sysml <model> probe.sysml -run-query "Probe::Ids root=SatelliteNetwork"`:

| Query over | develop before #1699 | 0.12.0 candidate |
|---|---|---|
| 6 198 elements, one query | 5.40 s · 2 242 MiB | 0.65 s · 210 MiB |
| 12 086 elements, one query | 10.69 s · 3 483 MiB | 1.14 s · 327 MiB |
| 12 086 elements, three queries | 29.83 s · 4 836 MiB | 1.25 s · 388 MiB |

## Re-runs

The six rounds ran while the machine also built and bisected, and the
runtime package's sec/op came out with 50–290 % spreads, so the rows whose
only move is in time were not decided by them. They were re-run
interleaved on the quiet machine once every other queue had drained:
six rounds, three columns, nothing else running (load 1.1–2.5, the
runner itself), the same "Intel(R) Xeon(R) Processor", 8 CPUs, `benchstat`
with its default 0.05 threshold. Spreads are 0–5 % unless marked. The
columns are the same three as above; bytes and allocations did not change
and are not repeated.

| Row | col 1 | col 2 | col 3 | 2 → 3 | 1 → 3 | Verdict |
|---|---|---|---|---|---|---|
| runtime `AssignmentLoopStep` | 1.634 ms | 1.528 ms | 1.658 ms | +8.5 % | +1.5 % | source regression confirmed (#1514/#1461, #1745) |
| runtime `DerivedReadWriteRead` | 6.360 µs | 5.929 µs | 6.860 µs | +15.7 % | +7.9 % | source regression confirmed (#1619, #1747) |
| runtime `IntegerArithmeticLoop` | 3.562 ms (±8 %) | 3.281 ms | 3.537 ms | +7.8 % | ~ (p=0.09) | source time regression, no bytes moved; the toolchain hides it from the user |
| runtime `RealDecimalLoop` | 3.844 ms | 3.439 ms | 3.635 ms | +5.7 % | −5.5 % | same |
| runtime `SetFeatureValueNoDependents` | 409.5 ns | 373.7 ns | 385.6 ns | +3.2 % | −5.8 % | same, small |
| grpc `InstantiateWarmModel` | 204.1 µs (±17 %) | 192.3 µs | 216.5 µs | +12.6 % | ~ (p=0.07) | source regression confirmed (#1508/#1512, #1742) |
| lsp `FormatEdits` | 864.1 µs | 807.0 µs | 841.1 µs | +4.2 % | −2.7 % | source time regression, no bytes moved, unattributed |
| repl `LoadModel/0` | 428.4 µs | 411.6 µs | 452.3 µs (±20 %) | +9.9 % | +5.6 % | confirmed (#1462, #1746) |
| repl `LoadModel/1000` | 118.8 ms | 109.7 ms | 105.1 ms | −4.2 % | −11.5 % | improvement stands |
| migrate `WriterSiblingBlocks` | 1.763 ms | 1.740 ms | 1.721 ms | −1.1 % | −2.4 % | cleared: the suite's +14 % was the machine |
| perf `IndexAddExpand/synthetic` | 42.94 ms | 38.48 ms | 38.66 ms | ~ | −10.0 % | cleared |
| perf `ExecuteActionFreshContext` | 49.35 µs | 46.72 µs | 49.58 µs | +6.1 % | ~ | confirmed (#1619, the marks) |
| perf `BatchConstraints` | 12.28 ms | 11.24 ms | 12.57 ms | +11.8 % | ~ (p=0.07) | source time regression confirmed, unattributed |
| perf `GRPCVerifyConstraint` | 1.392 ms | 1.316 ms | 1.635 ms | +24.3 % | +17.5 % | source regression confirmed (#1508/#1512, #1742) |
| stress `FleetInstantiate/32` | 4.921 ms | 4.834 ms | 4.847 ms | ~ | ~ | cleared |
| stress `FleetInstantiate/128` | 15.25 ms | 14.35 ms | 14.93 ms (±43 %) | +4.1 % | ~ | left in doubt: the candidate's spread is GC pacing |
| stress `HydratePlane/32` | 75.12 ms | 72.24 ms | 72.37 ms | ~ | −3.7 % | cleared |
| stress `EditBeside/128` | 3.889 ms | 3.816 ms | 3.804 ms (±48 %) | ~ | ~ | cleared, wide |
| stress `ValidateSplit/512/jobs=8` | 4.060 s | 3.923 s | 3.637 s | −7.3 % | −10.4 % | the suite's +7 % was the machine; the quiet host shows an improvement |
| stress `Satisfy/32`, `EditImported/32`, `FleetSatisfy/32` | | | | ~ | ~ / −4 % | cleared |

So of the ten time-only moves the loaded suite flagged, four were the
machine (`WriterSiblingBlocks`, `IndexAddExpand`, `HydratePlane/32`,
`EditBeside/128`, and `ValidateSplit/512` reversed sign), and five are
source: the three interpreter loops, `SetFeatureValueNoDependents` and
`FormatEdits`, all without a byte or an allocation moving, so they are
not in the fix list above — they are candidates for the same
evaluation-context cause as `AssignmentLoopStep` (the loops share its
path) and are stated here as a residual rather than attributed. On
every one of them the toolchain gain of column 1 → 2 is larger than the
source loss, so a user moving from 0.11.0 sees them flat or faster; the
rows where the user does see a regression are `GRPCVerifyConstraint`
(+17.5 %), `DerivedReadWriteRead` (+7.9 %), `LoadModel/0` (+5.6 %) and
`AssignmentLoopStep` (+1.5 %), each with a fix PR. The raw rounds are
`results/quiet/<column>-round<N>.txt` in the measurement tarball.

### The fixes on the quiet host

The fix branches were measured the same way (six quiet rounds,
interleaved with the candidate) on the rows above whose time they
could move. They recover allocations, not time: `perf/eval-context-size`
(#1745) leaves `AssignmentLoopStep` (1.651 → 1.646 ms), the two
interpreter loops, `SetFeatureValueNoDependents` and
`DerivedReadWriteRead` within noise; `perf/derive-marks-footprint`
(#1747) takes 1.2 % off `DerivedReadWriteRead` (6.772 → 6.693 µs) and
2 % off `IntegerArithmeticLoop`; `perf/binding-walk-allocs` (#1742)
leaves `InstantiateWarmModel`, `BatchConstraints` and
`ExecuteActionFreshContext` within noise and `GRPCVerifyConstraint` at
1.629 → 1.659 ms (+1.8 %, p=0.03). The time regressions of this table
therefore survive the fix PRs, which claim bytes and allocations only;
what they cost is the work #1508/#1512, #1514 and #1619 do, not the
allocations around it, and recovering it would mean changing what those
merges compute, which this record does not propose.

## Fixes

One pull request per cause, each against `develop` with its own
six-round A/B against the merge that introduced the cost; the branches
are named in the summary table. All seven merged into `develop` on
2026-10-30, in order: #1741 at `dbf32bc50`, #1742 `435991d22`, #1743
`2a7369580`, #1744 `5dd0724fd`, #1745 `68c80eec3`, #1746 `411daa342`,
#1747 `ab88ea950`. The merged head is probed against the pre-fix
candidate in [The cut point](#the-cut-point), and the release candidate
built on it is measured in full in
[The release candidate](#the-release-candidate).

- **#1741 `perf/release-preparse-trees`** (cause #1441): a submission's
  parse trees are let go once its files are accepted; the roots an
  unresolved reference is looked up by are found by parsing the pending
  sources once, on first need. `sysml -validate` on the 12 086-element
  generated model, medians of three cold runs, 0.11.0 rebuilt / #1441's
  parent / #1441 / candidate / fix: peak RSS at `GOGC=25` (≈ live heap)
  146 / 147 / 165 / 166 / **150 MiB**; at `GOGC=100` 162 / 163 / 176 /
  181 / 177 MiB (the pacer's spread is ±10 MiB there); wall under
  `GOMEMLIMIT=120MiB` 0.28 / 0.27 / 0.46 / 0.44 / **0.25 s**; all five
  complete under `ulimit -v 1200000`. The repl `LoadModel/*` rows are
  unchanged by it over six rounds.
- **#1742 `perf/binding-walk-allocs`** (causes #1508 and #1512): the
  binding walks allocate only where a binding can be found. Six rounds,
  candidate → fix: `GRPCVerifyConstraint` 16 301 → **14 887** allocs/op
  (0.11.0 rebuilt: 14 091; the 796 left are the owner and sibling reads
  those merges make correctly), 834 → 785 KiB/op; grpc
  `InstantiateWarmModel` 2 155 → **1 996** (0.11.0 rebuilt: 1 962);
  `Instantiate` allocs −12 %, `BatchSatisfy` allocs −6.5 % and B/op
  −2 %, `FeaturesOf` B/op −2 % (the rest of those bytes is #1626, the
  next fix). sec/op flat on this host.
- **#1743 `perf/related-features-memo`** (cause #1626): the memo of the
  features a feature's relationships name no longer records the
  features that declare no relationship of the kind asked for, which
  were most of its entries. Six rounds, #1626's parent / #1626 /
  candidate / fix: `FeaturesOf` 267.1 / 296.0 / 295.7 / **271.2 MiB**
  per op (0.11.0 rebuilt: 261.2); `BatchSatisfy` 461.0 / 491.2 / 491.2 /
  **475.5 MiB**; `Instantiate` pinned at two iterations 220.6 / 251.8 /
  251.8 / **229.8 MB**; allocs/op unchanged on every row, as is
  `GRPCVerifyConstraint` and every stress row. The `tests/perf` binary
  running `FeaturesOf` and `Instantiate`, three cold runs, candidate →
  fix: peak RSS 336 → **318 MiB**, wall 1.84 → 1.76 s, both complete
  under `ulimit -v 4000000`; the stress `FleetInstantiate/512` binary's
  peak RSS reads 187–193 vs 196–199 MiB with identical allocation
  counts and a B/op difference benchstat does not separate (p = 0.13).
- **#1744 `perf/typed-by-sweep`** (cause #1478): the parser counts a
  document's `typed by` spellings and the nonstandard-notation pass
  sweeps the document's expressions for one on a body-expression
  parameter only while the member walk has reported fewer than that.
  Six rounds, #1478's parent / #1478 / candidate / fix: repl
  `LoadModel/1000` 55.43 / 58.05 / 58.33 / **55.84 MiB** per op (0.11.0
  rebuilt: 55.52), `/250` 13.94 / 14.59 / 14.71 / **14.09 MiB**, `/4000`
  221.7 / 232.2 / 233.5 / **223.4 MiB**; `AnalyseResolved` 648.0 / 666.9
  / 667.9 / **649.8 KiB**; allocs/op unchanged (#1478 added bytes, not
  allocations). Peak RSS of `repl.test` running `LoadModel/1000` twenty
  times is 158–163 MiB on candidate and fix alike over three cold runs
  and under `ulimit -v 4000000`: the sweep's garbage was short-lived, so
  the fix recovers allocation volume, not resident memory. The other
  load-heavy rows listed under #1478 above were not re-measured on the
  branch.
- **#1745 `perf/eval-context-size`** (causes #1514 and #1461): the
  evaluation context, copied for every nested evaluation, notes the
  chain being evaluated by its last segment, its length and a pointer
  to the memoized declarations where #1514 carried two slice headers,
  and #1461's open-inputs flag shares a word with the behavior-body
  flag, so a context is 152 bytes again instead of 184. Six rounds,
  #1514's parent / #1514 / candidate / fix: `AssignmentLoopStep` 912.7 /
  974.1 / 947.9 / **925.7 KiB** per op (0.11.0 rebuilt: 899.7), allocs
  8 536 / 9 335 / 8 539 / 8 539; `DerivedReadWriteRead` 2 720 / 2 816 /
  3 328 / **3 264 B** (the rest is #1619, below); every other row of the
  package within ±0.2 %. The `runtime.test` binary running
  `AssignmentLoopStep` 300 times, three cold runs, candidate → fix: peak
  RSS 115–120 vs 113–120 MiB, wall 0.82 vs 0.82 s, both complete under
  `ulimit -v 4000000` (under `ulimit -v 2000000` both die in the Go
  runtime's arena reservation before loading a model); `sysml -validate`
  on the 12 086-element model 198 vs 187–200 MiB. The fix recovers
  allocation volume, not resident memory, and says so.
- **#1746 `perf/duplicate-libraries-walk`** (cause #1462): the
  duplicate-library check holds the first declaration of each library
  package name in a map and gives a name a list only when a second
  declaration is met. Six rounds, #1462's parent / #1462 / candidate /
  fix: repl `LoadModel/elements=0` 2 021 / 2 151 / 2 155 / **2 038**
  allocs/op (0.11.0 rebuilt: 2 022), 227.9 / 246.7 / 246.8 / **238.1
  KiB** per op; the `/250`, `/1000` and `/4000` rows are unchanged by it
  (their bytes are #1478, #1744). The `repl.test` binary running the
  empty load 200 times, three cold runs: peak RSS 111–113 vs 112–116
  MiB, wall 0.15 s on both, both complete under `ulimit -v 2000000` and
  `4000000`. The walk over every library document's root packages
  remains, so the load's time is not back to 0.11.0's.
- **#1747 `perf/derive-marks-footprint`** (cause #1619): `deriveMarks`
  cloned the memoized footprint's reads to stamp the object's feature on
  the reads reached through it, on every derivation of a value held
  through a part; the feature is now applied to the read as it is looked
  up, and the marks recorded are the same set. Six rounds, #1619's parent
  / #1619 / candidate / fix: `DerivedReadWriteRead` 2 816 / 3 328 /
  3 328 / **3 104 B** per op (0.11.0 rebuilt: 2 688), 67 / 71 / 71 /
  **69** allocs; `ExecuteActionFreshContext` 403.5 / 424.5 / 424.5 /
  424.0 allocs — its +21 are the marks themselves and stay; every other
  row of the package within ±0.2 %. The `runtime.test` binary running
  the derived read 20 000 times, three cold runs, candidate → fix: peak
  RSS 116–122 vs 113–120 MiB, wall 0.48 vs 0.47 s, both complete under
  `ulimit -v 4000000`. The remaining 288 B and two allocations per
  derivation are what #1619 records; the fix removes the copy, not the
  marks, and does not claim an RSS change.

## What this record does not measure

- In the original candidate measurement, anything after `d16ca71e0`, and
  #1771 by anything but its own pull request's benchstat
  ([After the measured heads](#after-the-measured-heads)). The separate
  [later develop comparison](#develop-after-the-candidate) below measures
  the three pinned revisions it names, not arbitrary newer heads.
- The merges between `1c6bb1972` and `ab88ea950` on their own: they are
  measured only inside the release candidate, never one at a time
  ([The cut point](#the-cut-point)).
- The `sysml-lsp` server under an editor, `sysml-grpc` over a network, the
  browser and WebAssembly builds, the Python and Node clients: the gRPC and
  LSP rows above are in-process.
- Wall time on another machine: every sec/op here is this host's, and the
  runtime package's sec/op only from the quiet re-run.
- The codegen rows (`tests/codegen` fails on every column, as recorded).
- Models with unresolved references, the case `perf/release-preparse-trees`
  makes parse twice; the generated and real models here all resolve.


## Reading the rows

Two rows of `tests/perf` cannot be read from the six-round table as the
others are, on every revision alike. `Instantiate` instantiates one
model per operation, and the first operation on a cold model is far
heavier than the rest (it fills the runtime's memos), so a round where
`testing` settled on one iteration reports some 100 MB/op more than a
round that ran two; the columns did not draw the same iteration counts,
and the figures below for that row come from runs pinned with
`-benchtime 2x`. `ExecuteAction` executes the same action on one model
and its bytes per operation grow linearly with the iteration count (about
10 B per earlier iteration, the same slope on 0.11.0 and on the
candidate), so its bytes are compared at equal iteration counts and its
allocations per operation, which do not drift, carry the comparison.

One cost was found and had already been removed: bisecting the bytes of
`LoadModel/elements=1000` past #1478 lands on #1528 (the REPL's search
beside a loaded file for the roots it leaves unresolved, +1.7 MB per
load), which #1624 took off the path of a file that resolves — the
candidate shows no bytes under that search.

## The cut point

The measurement began at `0c1fd6dbd`, and the candidate measured is
`1c6bb1972`, four first-parent merges later (#1699 at `101d2f267`, then
#1700, #1701, #1705). `develop` moved on while the suite ran. The
first-parent merges between `1c6bb1972` and the seven fix merges are
measured only by the probes of [the next section](#develop-after-the-fixes-landed),
on the rows of the right-hand column:

| Merge | Subject | Rows it could move |
|---|---|---|
| #1704, #1698, #1703, #1702, #1729, #1736 | Sonar-driven refactors, CI scripts | none expected; `go test` surfaces |
| #1706, #1707 | `internal/ir/view` | `internal/ir/view` and `ir/view/route` rows |
| #1732 | connector-end element IDs in the export | `WriteAPIJSON`, `ConvertAPIJSON`, the export probe |
| #1709 | reflective element attributes in semantics | `Lookup*`, `Analyze*`, `FeaturesOf`, the load rows |
| #1738 | membership element IDs; `-run-query -json` tables | `WriteAPIJSON`, the export probe, the REPL query rows |
| #1708, #1733, #1737, #1739 | subject labels, portion subparts, ECMA-427 identities, inherited connector bindings | the binding walks (`GRPCVerifyConstraint`, `InstantiateWarmModel`), `Lookup*`, the export probe |
| #1717, #1728, #1735, #1731 | docs audits, AGENTS note, dependabot | none |
| #1741–#1747 | the seven fixes of this record | the rows of [Fixes](#fixes) — measured below |

### Develop after the fixes landed

`ab88ea950`, the merge of #1747, is `develop` after the seven fixes and
every merge of the table above landed; the release candidate is this
head with #1730 on top ([The release candidate](#the-release-candidate)).
Two probes of it against the pre-fix candidate, on the same host as the
rest of this record, show what the fixes gave back before the full
measurement of the release candidate was taken.

First, against the candidate `1c6bb1972`, six interleaved rounds on the
rows the fixes claim. The host carried other work during this probe
(load 1.3–3.9), so only the exact columns are read from it; its times
are superseded by the quiet probe below. Column 2 is repeated for the
distance that remains.

| Row | 0.12.0 candidate | develop after the fixes | 0.11.0 rebuilt | fix |
|---|---|---|---|---|
| repl `LoadModel/0`, B · allocs | 246.8 Ki · 2 155 | 238.3 Ki · 2 038 | 227.9 Ki · 2 022 | #1746 |
| `LoadModel/1000`, B/op | 58.35 Mi | 55.82 Mi | 55.52 Mi | #1744 |
| `LoadModel/4000`, B/op | 233.5 Mi | 223.5 Mi | 222.3 Mi | #1744 |
| runtime `AssignmentLoopStep`, B/op | 948.3 Ki | 926.0 Ki | 899.7 Ki | #1745 |
| `DerivedReadWriteRead`, B · allocs | 3 328 · 71 | 3 040 · 69 | 2 688 · 67 | #1745, #1747 |
| perf `GRPCVerifyConstraint`, B · allocs | 814.6 Ki · 16 300 | 766.3 Ki · 14 890 | — · 14 090 | #1742 |
| `FeaturesOf`, B · allocs | 296.4 Mi · 2.190 M | 265.6 Mi · 2.105 M | 261.2 Mi · — | #1743, #1742 |
| `BatchSatisfy`, B · allocs | 491.2 Mi · 4.078 M | 465.1 Mi · 3.815 M | 455 Mi · — | #1743, #1742 |
| `Instantiate` (cold, 2 iterations), B · allocs | 240.1 Mi · 2.011 M | 209.7 Mi · 1.767 M | 210.8 Mi · — | #1743, #1742 |
| `ExecuteActionFreshContext`, B · allocs | 48.61 Ki · 424 | 47.93 Ki · 423 | — · 388 | #1747 (the marks remain) |
| `SetFeatureValueNoDependents`, the interpreter loops, allocs | | unchanged | | — |

Every row lands where its fix PR's A/B said it would, and the rows two
fixes touch (`FeaturesOf`, `BatchSatisfy`, `Instantiate`) land closer to
0.11.0 than either PR claimed alone: `Instantiate` is back at the
0.11.0 volume, `FeaturesOf` is 1.7 % above it, `BatchSatisfy` 2.2 %.
What remains above column 2 is the residual each PR names: the
26 Ki of `AssignmentLoopStep` and the 352 B · 2 allocs of
`DerivedReadWriteRead` that #1514 and #1619 compute, the 800 allocations
of `GRPCVerifyConstraint` that #1508/#1512 read, and the 16 allocations
of `LoadModel/0` that #1462's one-map walk still makes.

The whole-binary rows of #1741, re-measured on the merged head against
the candidate on the 12 086-element generated model (binaries built as
`make build` does, three cold runs each, same loaded host — compare
within the row, not with the quiet figures of
[Whole binary](#whole-binary)):

| `sysml -validate`, 12 086 elements | 0.12.0 candidate | develop after the fixes |
|---|---|---|
| peak RSS, `GOGC=25` (≈ live heap), three runs | 172 · 173 · 172 MiB | 153 · 153 · 154 MiB |
| peak RSS, `GOGC=100`, three runs | 185 · 186 · 199 MiB | 171 · 168 · 185 MiB |
| wall under `GOMEMLIMIT=120MiB` | 1.00 s | 0.57 s |
| `ulimit -v 1200000` | completes | completes |
| binary size | 59.65 MB | 59.71 MB |

The 19 MiB the merged head gives back at `GOGC=25` is #1741's parse
trees; the 7 MiB that remain above 0.11.0 rebuilt's 146 MiB (measured
quiet, so the comparison is approximate) this record does not attribute —
the binary is 5.7 MB larger and the merges of the table above hold
state of their own — and no fix here addresses them.

## The release candidate

### What was measured

The release candidate is `cc1d477ba9843ce8e36378a87f49ae4005f9a2b7`: the
head of #1730 (the Sonar-driven refactor of the export packages) after
`develop` at `ab88ea950` was merged into it, so its tree is `develop`
after the seven fixes plus #1730 and nothing else. Checked before the
run: `git merge-base cc1d477ba origin/develop` is `ab88ea950`, and
`git diff --stat ab88ea950 cc1d477ba` names four files —
`internal/translate/export/api_json_in.go`,
`internal/translate/export/api_json_structure.go`,
`internal/translate/export/rdf_out.go` and
`docs/project/api-json-differential-baseline.json` (the writer digest
re-recorded, no count moved). The seven fixes are in the candidate:
each is a first-parent merge of `develop` at `ab88ea950`, which the
candidate contains (its own first parent is #1730's branch; `develop`
is its second).

| Fix | Cause | Merge on `develop` | In the candidate |
|---|---|---|---|
| #1741 `perf/release-preparse-trees` | #1441 | `dbf32bc50` | yes |
| #1742 `perf/binding-walk-allocs` | #1508, #1512 | `435991d22` | yes |
| #1743 `perf/related-features-memo` | #1626 | `2a7369580` | yes |
| #1744 `perf/typed-by-sweep` | #1478 | `5dd0724fd` | yes |
| #1745 `perf/eval-context-size` | #1514, #1461 | `68c80eec3` | yes |
| #1746 `perf/duplicate-libraries-walk` | #1462 | `411daa342` | yes |
| #1747 `perf/derive-marks-footprint` | #1619 | `ab88ea950` | yes |

While the suite ran, #1730 merged into `develop` (`a71bd8a1a`) between
two Dependabot merges, #1748 (JaCoCo 0.8.16 in three `pom.xml`) and
#1750 (`action-gh-release` in `release-windows.yml`), and `develop`
became `2ccd77756`. The candidate is an ancestor of that head and
`git diff --stat cc1d477ba 2ccd77756 -- '*.go' go.mod go.sum` prints
nothing: the whole difference is `.github/workflows/release-windows.yml`,
`client/java/pom.xml`, `editors/mdk/pom.xml` and `editors/syson/pom.xml`
(5 lines). So the release candidate of this section is `develop`
`2ccd77756`, measured at `cc1d477ba`, whose Go sources are identical.

Columns 1 and 2 are the rounds of the first pass, reused: the same
binaries on the same VM (8 vCPUs, `Intel(R) Xeon(R) Processor`, 31 GiB,
checked into every round's file), not re-run. Column 4 is six rounds of
its own of every benchmark package and `tests/stressmodel`, the same
flags, run alone on that host (load 1.1–2.7 at the start of a round).
The candidate's worktree received the pilot and training corpora after
its second round, so the corpus-backed rows — `WriteAPIJSON`, the
`vehicle` rows of `tests/perf`, `GRPCParseFile*` and grpc
`InstantiateWarmModel`, which skips without the corpus — carry six
rounds of their own taken afterwards on the quiet host. The rows known
to fail ([Method](#method)) fail identically on the candidate.

### The regression rows, before and after the fixes

Medians over six rounds; bytes and allocations exact (`± 0 %`). The
percentage is the release candidate against 0.11.0 rebuilt (column
2 → 4); the pre-fix column is the first pass.

| Row | Metric | 0.11.0 rebuilt | before the fixes (`1c6bb1972`) | release candidate (`cc1d477ba`) | 2 → 4 | Fix |
|---|---|---|---|---|---|---|
| repl `LoadModel/elements=0` | allocs · B/op | 2 022 · 227.9 Ki | 2 155 · 246.8 Ki | 2 038 · 238.1 Ki | +0.8 % · +4.5 % | #1746 |
| repl `LoadModel/elements=1000` | B/op | 55.52 Mi | 58.31 Mi | 55.82 Mi | +0.5 % | #1744 |
| repl `LoadModel/elements=4000` | B/op | 222.3 Mi | 233.5 Mi | 223.5 Mi | +0.5 % | #1744 |
| runtime `AssignmentLoopStep` | B/op | 899.7 Ki | 948.0 Ki | 925.9 Ki | +2.9 % | #1745 |
| runtime `DerivedReadWriteRead` | B/op · allocs | 2 688 · 67 | 3 328 · 71 | 3 040 · 69 | +13.1 % · +3.0 % | #1745, #1747 |
| perf `GRPCVerifyConstraint` | allocs · B/op | 14 090 · 829.4 Ki | 16 300 · 814.6 Ki | 14 890 · 766.3 Ki | +5.6 % · −7.6 % | #1742 |
| grpc `InstantiateWarmModel` | allocs | 1 960 | 2 166 | 2 006 | +2.3 % | #1742 |
| perf `FeaturesOf` | B/op · allocs | 261.2 Mi · 2.228 M | 295.8 Mi · 2.190 M | 265.4 Mi · 2.105 M | +1.6 % · −5.5 % | #1743, #1742 |
| perf `BatchSatisfy` | B/op · allocs | 454.9 Mi · 4.202 M | 491.4 Mi · 4.078 M | 465.1 Mi · 3.815 M | +2.3 % · −9.2 % | #1743, #1742 |
| perf `ExecuteActionFreshContext` | allocs | 388 | 424 | 422 | +9.0 % | none: #1619's marks |
| perf `ActionChain/chain10` | B/op | 198.1 Ki | 222.9 Ki | 207.8 Ki (± 16 %, p=0.22) | ~ | — |
| whole binary, `-validate` of 12 086 elements | peak RSS | 161 MiB | 193 MiB | 171 MiB | +10 MiB | #1741 |

What remains above column 2 is what each fix's pull request named as
its residual, now measured on the release itself: the 26 Ki of
`AssignmentLoopStep` and the 352 B · 2 allocations of
`DerivedReadWriteRead` that #1514 and #1619 compute, the 800
allocations of `GRPCVerifyConstraint` and 46 of `InstantiateWarmModel`
that #1508/#1512 read, the 16 allocations and 10 Ki of `LoadModel/0`
that #1462's one-map walk still makes, the 4.2 Mi of `FeaturesOf` and
10 Mi of `BatchSatisfy` that #1626's memo holds for the features that
declare a relationship, and the 34 allocations of
`ExecuteActionFreshContext` that are #1619's marks. The load rows are
back at 0.11.0's volume (+0.5 %), and `FeaturesOf`, `BatchSatisfy` and
`GRPCVerifyConstraint` allocate fewer times than 0.11.0 did.

### Everything else in the six rounds

Beyond the table, these rows moved 2 % or more in bytes or allocations
between column 2 and the release candidate (p < 0.05, n=6). Down:
`NewRootFrame` −80 % B · −42 % allocs, `WriteAPIJSON` −15 % B and
−20…−26 % allocs, `REPLLoadFile` −6 % B · −5 % allocs, `ExecuteAction`
−23 % allocs, `ExecuteState` and `StateLoop` −16.5 % allocs,
`ActionLoop` −3…−41 % B, `ActionChain/chain100` and `chain1000` −19 %
and −27 % B with −11 % allocs, stress `ConvertAPIJSON` −6…−7 % B and
−16…−17 % allocs, `Satisfy` and `FleetSatisfy` −4 % allocs,
`FleetInstantiate/512` −10 % B (± 38 %). Up: repl `RunStateMachine`
+2.7 % B, `RunCalc` +2.0 % B, perf `GRPCEvaluate` +3.1 % B · +3.2 %
allocs, `SameConstraintManyInstances` +2.1 % B,
`ImportValues/flat/parts=2000/attrs=5` +2.0 % allocs — each already at
that size in the pre-fix candidate, below the threshold the first pass
traced, and not attributed here. `ExecuteAction`'s +30 % B/op is the
iteration-count artefact of [Reading the rows](#reading-the-rows); its
allocations are down. Every other row of every package is within 2 %
in bytes and allocations.

sec/op from these rounds alone (the host carried nothing else, but the
spreads are still 20–230 %) decides only the rows that move far:
`NewRootFrame` −92 %, `ActionLoop/for10` −42 %, `ActionChain` −29…−62 %;
stress `Satisfy/satellites=128` came out +22 % at ± 74 % and goes to
the quiet re-run with the rest of the time-only rows.

### Whole binary, release candidate

`bin/sysml` from each configuration (the release candidate's built from
`cc1d477ba` as `make build` builds it), nine interleaved runs each,
medians of wall time and peak resident set, one binary at a time, taken
twice: once straight after the suite (load 4.1) and once on the quiet
host (load 0.9); the quiet figures are below. The host was slower in
every column than in the first pass (0.11.0's 12 086-element validation
0.29 → 0.47 s, see [The quiet re-run](#the-quiet-re-run)), so wall time
is compared only within this table, and the pre-fix column repeats only
the resident sets of [Whole binary](#whole-binary), which the host's
speed does not move. The Apollo 11 validation report of the release
candidate is byte-identical to the pre-fix candidate's.

| Workload | 0.11.0 | 0.11.0 rebuilt | before the fixes, RSS | release candidate |
|---|---|---|---|---|
| binary size, `sysml` · `sysml-lsp` | 53.5 · 28.9 MB | 54.0 · 29.5 MB | 59.6 · 36.3 MB | 59.7 · 36.4 MB |
| process floor, `-e 1` | 0.04 s · 71 MiB | 0.04 s · 71 MiB | 73 MiB | 0.05 s · 74 MiB |
| `examples/action-executor-demo.sysml` | 0.05 s · 73 MiB | 0.04 s · 73 MiB | 75 MiB | 0.05 s · 75 MiB |
| `examples/disposal-robot-demo` | 0.07 s · 80 MiB | 0.07 s · 79 MiB | 81 MiB | 0.08 s · 82 MiB |
| generated, 3 254 elements | 0.17 s · 105 MiB | 0.19 s · 103 MiB | 106 MiB | 0.16 s · 106 MiB |
| generated, 6 198 elements | 0.29 s · 119 MiB | 0.30 s · 118 MiB | 140 MiB | 0.25 s · 129 MiB |
| generated, 12 086 elements | 0.47 s · 162 MiB | 0.43 s · 161 MiB | 193 MiB | 0.46 s · 171 MiB |
| Apollo 11, 28 files | 0.45 s · 170 MiB | 0.52 s · 170 MiB | 180 MiB | 0.54 s · 174 MiB |

The resident set the release takes over 0.11.0 is 3 MiB at the floor
(a 5.7 MB larger binary), 4 MiB on Apollo 11 and 10 MiB on the largest
generated model, where the pre-fix candidate took 31 — #1741 gave back
21 of those. Under `GOGC=25`, which keeps the resident set near the live
heap, the release candidate peaks at 152 MiB (medians of three cold
runs) to 0.11.0 rebuilt's 150 and the pre-fix candidate's 166; under
`GOMEMLIMIT=120MiB` it takes 0.33 s to 0.11.0 rebuilt's 0.39 (the
pre-fix candidate took 0.44 s to 0.28 on the faster host of the first
pass); both complete under `ulimit -v 1200000`. The
first `GOGC=25` run of each sequence (2.1–2.7 s, 225–232 MiB, three
sequences out of three) is the cold record-cache load described under
[The export rows](#the-export-rows-re-probed-on-5e0322e0c-and-d01e4f49c); the medians
are warm runs. The 10 MiB
that remain at the default `GOGC` this record does not attribute: it is
pacing over a heap that is larger by the binary and by the state the
merges since 0.11.0 hold, not the parse trees, and no fix here addresses
it.

### The export rows, re-probed on `5e0322e0c` and `d01e4f49c`

After the suite, #1760 (`feat(export): elementIds for the relationships a
head implies`) merged into `develop` at `5e0322e0c`. It is Go in the path
the export rows measure — `git diff --stat cc1d477ba 5e0322e0c -- '*.go'
go.mod go.sum` names `internal/translate/export/element_ids.go`,
`end_forms.go`, `rdf_normative.go`, `rdf_out.go`,
`internal/doc/queryexec/elementids.go` and two test files (565
insertions, 54 deletions) — so those rows were re-probed on a worktree
at `5e0322e0c`: six rounds interleaved with both 0.11.0 columns of
`tests/export` and the stress `ConvertAPIJSON` rows (load 1.4–2.1 at
the start of a round, the same steal as the afternoon's re-run), and
the whole-binary element-ID probe against the `cc1d477ba` binary. The
runtime, load and REPL rows stand as measured at `cc1d477ba`; nothing in
that diff is on their path. Then #1774 (`fix(export): read one
relationship id per feature-chain clause`) landed at `1d662609d` and
`develop` reached `d01e4f49c`; `git diff --stat b4d0a35c4 d01e4f49c --
'*.go' go.mod go.sum` names `internal/translate/export/element_ids.go`
(16 lines) and its identity test, `cmd/sysml/run_query_test.go`, and six
files of `client/opensysml` (the Go client, not the engine), so the same
probe was run a second time on a worktree at `d01e4f49c`, the same way.

| Row | Metric | 0.11.0 rebuilt | `cc1d477ba` (suite) | `5e0322e0c` | `d01e4f49c` | 2 → `d01e4f49c` |
|---|---|---|---|---|---|---|
| `WriteAPIJSON/ISQAtomicNuclear/standard` | B/op · allocs | 249.8 Mi · 1.866 M | 212.5 Mi · 1.388 M | 212.5 Mi · 1.388 M | 212.5 Mi · 1.388 M | −14.9 % · −25.6 % |
| `WriteAPIJSON/ISQAtomicNuclear/compact` | B/op · allocs | 193.2 Mi · 1.800 M | 164.8 Mi · 1.443 M | 164.8 Mi · 1.443 M | 164.8 Mi · 1.443 M | −14.7 % · −19.9 % |
| `WriteAPIJSON/SimpleVehicleModel/standard` · `/compact` | B/op | 384.4 · 271.0 Mi | 348.3 · 254.2 Mi | 348.3 · 254.2 Mi | 348.3 · 254.2 Mi | −9.4 % · −6.2 % |
| `WriteAPIJSON/then_after_members/standard` · `/compact` | allocs | 218.6 k · 209.7 k | 167.9 k · 172.7 k | 167.9 k · 172.7 k | 167.9 k · 172.7 k | −23.2 % · −17.6 % |
| `ConvertAPIJSON/satellites=32/elements=6227` | B/op · allocs | 7.310 Gi · 40.29 M | 6.824 Gi · 33.86 M | 6.863 Gi · 33.93 M | 6.865 Gi · 33.93 M | −6.1 % · −15.8 % (+0.6 % · +0.2 % over `cc1d477ba`, from #1760) |
| `ConvertAPIJSON/satellites=128/elements=24167` | B/op · allocs | 30.21 Gi · 159.8 M | 28.05 Gi · 132.2 M | 28.05 Gi · 132.2 M | 28.05 Gi · 132.2 M | −7.2 % · −17.3 % |

#1760's element IDs cost the writer nothing on the `WriteAPIJSON`
models (bytes and allocations equal to `cc1d477ba`'s in every sample)
and 40 MiB · 70 k allocations per conversion of the 6 227-element
constellation — the IDs of the relationships its heads imply — with the
24 167-element one unchanged. sec/op over these six rounds: every
`WriteAPIJSON` row `~` against column 2 except `ISQAtomicNuclear/compact`
(−26 %, p=0.009); against 0.11.0 as released, `ISQAtomicNuclear` −47 %
on both forms and `SimpleVehicleModel/standard` −24 %; `ConvertAPIJSON`
`~` against column 2 and −15 % at 128 satellites against column 1 — the
same host as the re-run above, so read the direction, not the size.

#1774 changes none of it: at `d01e4f49c` every `WriteAPIJSON` and
`ConvertAPIJSON` row allocates the same bytes and the same number of
times as at `5e0322e0c` (`~`, p ≥ 0.13, n=6 each), and its six rounds
ran at 0–4 % steal — the quietest of the day — so their sec/op is worth
a sentence: against column 2, `WriteAPIJSON/ISQAtomicNuclear` −35 % and
−27 %, `SimpleVehicleModel/standard` −27 %, `ConvertAPIJSON/32` −18 %
(p ≤ 0.04), the rest `~`; against 0.11.0 as released, five of the six
`WriteAPIJSON` rows −20…−45 % (p ≤ 0.04). That is the toolchain and
#1699's path together, what a 0.11.0 user sees.

The element-ID probe of [Export element IDs](#export-element-ids),
`cc1d477ba`'s binary against `5e0322e0c`'s, interleaved, three runs
each after one discarded warm-up per binary (the first run of a binary
after another binary's is a cold record-cache load, below), medians:

| Query over | `cc1d477ba` | `5e0322e0c` | `cc1d477ba` again | `d01e4f49c` |
|---|---|---|---|---|
| 6 198 elements, one query | 0.91 s · 226 MiB | 1.12 s · 230 MiB | 0.87 s · 222 MiB | 1.15 s · 241 MiB |
| 12 086 elements, one query | 3.36 s · 354 MiB | 3.68 s · 343 MiB | 2.20 s · 346 MiB | 1.60 s · 358 MiB |
| 12 086 elements, three queries | 1.98 s · 370 MiB | 2.07 s · 391 MiB | 1.47 s · 372 MiB | 1.74 s · 388 MiB |

The two pairs are two sequences, an hour apart; compare within a pair.
Resident memory is the row to read (the host's wall times are a day
slower than the first pass's, where this query took 1.14 s): #1760 and
#1774 together add 4–19 MiB to the query's peak, the IDs of the implied
relationships now emitted, and nothing to the time within the spread
of three. `-validate` of the same model, run in the second sequence
after a warm-up, peaks at 176 MiB on `cc1d477ba` and 177 on `d01e4f49c`
(0.11.0 rebuilt 162), so the growth is the query's, not the load's.
The one query at 12 086 elements peaking above the three in the first
pair is an artefact of the measurement, explained next.

Two whole-binary effects of the release candidate's binary came out of
these sequences, both of the library record cache
(`$XDG_CACHE_HOME/sysml-ls/libs`, keyed by document, content and format):
a run that finds its records cold takes 2.1–2.8 s and peaks at
225–300 MiB where the warm run takes 0.3–0.5 s and 150–176 MiB, and a
`-run-query` run leaves the next `-validate` of the same binary cold
(`cc1d477ba`: 0.43 → 0.26 s, then a query, then 2.29 s · 291 MiB, then
0.27 s again, reproduced three times). 0.11.0 rebuilt's first run after
hours idle was a cold load too (3.83 s), so the cost is the cache's,
not the release's; what a query does to the records that follow it is
this record's one open finding, filed for follow-up and not attributed
to any merge here. Every median above is a warm run.

### After the measured heads

`develop` moved while this section was measured. From `cc1d477ba`, in
first-parent order: #1748 and #1750 (poms, a workflow), #1730 itself
(`a71bd8a1a`), the client-parity merges #1751, #1753, #1755–#1758, #1761,
#1762, #1763, #1766, #1767, #1769, #1770 and #1764 (`client/**`,
`editors/vscode`, `docs/`, `overrides/`), #1760 (`5e0322e0c`, re-probed
above), and #1771 (`b4d0a35c4`, `refactor(runtime): clear the three
Sonar smells left on develop`). `git diff --stat 5e0322e0c b4d0a35c4 --
'*.go' go.mod go.sum` names two files: `internal/exec/runtime/invoke_calc.go`
(42 lines: a `constraintMemberBody` helper extracted from
`eachUnwrappedDeclMember`, which #1742 had just tuned) and
`internal/frontend/grpc/validate_test.go`. #1771 is the one runtime Go
change after the measured SHA, and its pull request carries the
measurement: six rounds, `develop` `8dc84706` against the branch, on the
three rows that walk the changed function — `AssignmentLoopStep`
926.2 → 926.3 Ki `~`, 8 543 → 8 545 allocs `~`; `InstantiateWarmModel`
2 046 → 2 045 allocs `~`; `GRPCVerifyConstraint` 766.3 Ki `~`, 14 890
allocs identical in every sample; sec/op `~` on all three. The record
cites that and does not re-measure the runtime rows for it.

After `b4d0a35c4` came #1765, #1759, #1768, #1772 and #1773 (the client
parity follow-ups: `client/**`, including six Go files of
`client/opensysml`, the Go client library, which no measured row runs)
and #1774 (`1d662609d`), whose 16 lines in
`internal/translate/export/element_ids.go` are covered by the second
export probe above; `develop` is `d01e4f49c`.

Then #1777 (`feat(engine): run state machines…`, `1e34d9d95`) landed on
the runtime: `git diff --stat d01e4f49c 1e34d9d95 -- '*.go' go.mod
go.sum` names `internal/exec/runtime/context.go` (+82),
`internal/frontend/engine/engine.go` and `json.go`,
`internal/frontend/grpc/run_options.go` (−76, the options moved to the
engine) and `service.go`, and two new `tests/wasm` tests — no file under
`internal/translate/export`, so the export probe above stands, and the
runtime rows can no longer stand by an empty diff. They were re-run:
six interleaved rounds, `cc1d477ba` against `1e34d9d95`, the runtime and
gRPC packages, repl `LoadModel`, and the `tests/perf` rows this record
reports, with the steal percentage logged per round, and the
`tests/perf` binary's peak resident set over three cold runs each.

Nothing moved. Bytes and allocations per operation are equal on every
exact row, `cc1d477ba` against `1e34d9d95` — `AssignmentLoopStep`
925.8 Ki · 8 540, `DerivedReadWriteRead` 2 969 B · 69, the interpreter
loops, `SetFeatureValueNoDependents` 416 B · 9,
`GRPCVerifyConstraint` 766.3 Ki · 14 890, `FeaturesOf` 265.4 Mi ·
2.103 M, `BatchSatisfy` 465 Mi · 3.815 M, `ExecuteActionFreshContext`
47.8 Ki · 423, `ExecuteState`, `ActionChain`, `LoadModel/0` 238.1 Ki ·
2 038 and `LoadModel/1000` 55.8 Mi · 538.0 k (the two `LoadModel` rows
differ in their last digit, −0.01 % and −0.00 %, p < 0.05, the kind of
difference `± 0 %` hides). `InstantiateWarmModel` is not exact on
either side (its allocation count varies with the warm model's state,
± 9–15 % in bytes and ± 4–6 % in count over six rounds) and came out
120 → 123 Ki and 2 009 → 2 029 allocations, p = 0.39 and 0.38 — a
difference measured and not resolved, inside the row's own spread, not
a claim of equality. sec/op is `~` on all of them, under 3–7 % steal
(p ≥ 0.24). The `tests/perf` binary (`FeaturesOf` and
`Instantiate`, two iterations) peaks at 366–406 MiB on `cc1d477ba` and
386–391 on `1e34d9d95` over three cold runs each, the same within the
spread. #1777 adds state to the runtime context that the measured paths
do not touch; the record names it as measured and neutral.

`develop` then took #1779 (`client/python`) and this record itself
(#1740) to `cf1652846`, with no Go change after `1e34d9d95`, and then
#1784 (`fix(semantics): Usage::mayTimeVary derives over the effective
isComposite`, `9a4ffcd83`), which is on the semantics path every load
and derivation runs: `git diff --stat cf1652846 9a4ffcd83 -- '*.go'
go.mod go.sum` names `internal/semantic/semantics/usage.go` (+46/−12),
`annotations.go` (+28), `internal/semantic/symbols/facts.go` (+3),
`internal/workspace/libs/interface.go` (+4/−1) and two test files. The
rows that exercise it were re-run the same way, `cc1d477ba` against
`9a4ffcd83`, six interleaved rounds with the steal percentage logged:
the runtime and gRPC packages, repl `LoadModel`, `internal/workspace/libs`
and `internal/workspace/model`, and the `tests/perf` load, analysis and
instantiation rows; then the whole-binary floor, examples, generated
models and Apollo 11, three runs each after a warm-up.

This one moved rows. Six rounds at 0.8–3.0 % steal, `cc1d477ba` against
`9a4ffcd83`, bytes and allocations exact: repl `LoadModel/elements=1000`
55.83 → 56.67 Mi (+1.5 %) and 538.0 k → 563.8 k allocations (+4.8 %),
`/250` and `/4000` the same +1.5 % · +4.7–4.8 %, `/0` unchanged (238.1 Ki ·
2 038, so the cost is per element loaded, not per load); perf
`REPLLoadFile` +1.8 % · +5.7 %, `Analyze/synthetic` +2.2 % · +6.4 %,
`ImportValues/flat` +1.3–2.6 % · +3.8–4.3 %, `ImportValues/nested`
+4.6–5.5 % · +11.0–13.1 %. Every other row is unchanged: the runtime
package (`DerivedReadWriteRead` 2 969 B · 69, `AssignmentLoopStep`),
`InstantiateWarmModel`, `FeaturesOf`, `GRPCVerifyConstraint`,
`ExpandWildcardImports`, `AnalyseResolved`, `AnalyzeRequirementGroup`,
the parser rows. sec/op is `~` on all of them (`LoadModel/1000`
114 → 138 ms at ± 49 %, p=0.13; `ImportValues/nested/2000` 771 →
1 004 ms, p=0.065; `REPLLoadFile` p=0.065) — the direction of the
bytes, unresolved in time by six rounds. The cause is #1784 itself,
the only change between the two heads: `Usage::mayTimeVary` now derives
over the effective `isComposite`, and the derivation runs once per
usage at load, allocating per usage; `/0` shows the standard library
alone does not pay it. This record names it and does not fix it: it
takes back about a third of what #1744 recovered on the load rows
(55.52 Mi on 0.11.0 rebuilt, 58.31 before the fixes, 55.83 at
`cc1d477ba`, 56.67 now) and puts the load allocation count above
0.11.0's for the first time since the fixes (533.1 k → 563.8 k, +5.8 %).
Whole binary, three runs each after a warm-up, `cc1d477ba` against
`9a4ffcd83`: floor 74 → 75 MiB, generated 12 086 elements 169 → 174 MiB
(0.27 → 0.29 s), 6 198 elements 132 → 129, Apollo 11 175 → 171 MiB
(0.35 → 0.34 s), examples within 2 MiB; the Apollo 11 report is
byte-identical. The resident set does not resolve the +1.5 % of bytes;
the allocation counts do.

After #1784, `develop` took #1781 (a workflow and a hygiene test) and
#1786 (`client/**`) to `fdc373e43`, and then #1783 (`fix(export): the RDF
export states a usage's isVariable and mayTimeVary as the metamodel
does`, `e022cbca0`), on the export path again: `git diff --stat
fdc373e43 e022cbca0 -- '*.go' go.mod go.sum` names
`internal/translate/export/api_json_absence.go` (+19/−2),
`api_json_booleans.go` (+6/−1), `rdf_out.go` (+18), `verbatim.go`
(+13/−1), `rdf_in_toolkit.go` (+4/−2) and four test files under
`tests/export` — nothing outside `internal/translate/export`. The export
rows were re-probed a third time, six interleaved rounds of
`tests/export` (the `WriteAPIJSON*` and RDF rows) and the stress
`ConvertAPIJSON` rows at `e022cbca0` against `d01e4f49c`, the previous
export measurement, then the whole-binary element-ID query, the RDF
export and the 12 086-element load against the `d01e4f49c` binary.

Six rounds at 0–2 % steal, `d01e4f49c` against `e022cbca0`, bytes and
allocations exact on every row, and every row moved by a little, in the
direction of the output: `WriteAPIJSON` writes +0.37–2.20 % more output
(`ISQAtomicNuclear/standard` 7.778 → 7.809 Mi, `then_after_members/compact`
338.6 → 346.1 Ki — the `isVariable` and `mayTimeVary` statements the fix
adds), for +0.06–0.57 % bytes (`SimpleVehicleModel/compact` 254.0 →
255.5 Mi) and +0.26–0.90 % allocations (`then_after_members/standard`
167.9 k → 169.4 k), all p = 0.002; stress `ConvertAPIJSON/32` 6.864 →
6.919 Gi (+0.79 %) and 33.93 → 34.34 M allocations (+1.19 %), `/128`
28.05 → 28.24 Gi (+0.68 %) and 132.2 → 133.8 M (+1.19 %), p = 0.002.
sec/op is `~` on all of them (`ConvertAPIJSON/128` 71.0 → 62.5 s at
± 21–31 %, p = 0.31; `WriteAPIJSON/SimpleVehicleModel/standard` 765 →
541 ms, p = 0.18). The cause is #1783 itself, the only change between
the two heads: each usage's export now states two more booleans and
their absence form, and each costs its string. Whole binary, three runs
each after a warm-up, `d01e4f49c` against `e022cbca0`: the element-ID
query on 6 198 elements 230–254 → 242–248 MiB (0.72–0.96 → 0.73–1.39 s),
on 12 086 elements 363–380 → 361–383 MiB (1.35–2.78 → 1.33–2.88 s),
three queries in one run 395–402 → 397–399 MiB; `-convert ttl` of the
12 086-element model 12.5–15.1 → 13.0–13.8 s and 4.29–4.61 → 4.31–4.92
GiB resident; `-validate` of the same model 165–173 → 173 MiB at 0.25 s
on both. Nothing a user sees moves beyond the run-to-run spread. One
residual this probe surfaced and does not attribute to #1783, since
both sides pay it: the Turtle export of a 12 086-element model holds
4.3–4.9 GiB resident for 13 s where `-validate` of the same model holds
173 MiB — the RDF writer's footprint, worth its own measurement.

After `e022cbca0`, `develop` took #1793 and #1794 (`ef85f6f70`), both
for one npm advisory: they move the `client/node`, `editors/vscode` and
`scripts/asciidoc` lockfiles and the AsciiDoc converter script, and
`git diff --stat e022cbca0 ef85f6f70 -- '*.go' go.mod go.sum` is empty,
so no measured row can have moved. Then #1792 (`fix(export): an
expression body's parameter states its derived isVariable and
mayTimeVary`, `86fa09ec9`) landed on the export path a fourth time: `git diff --stat e022cbca0 86fa09ec9 --
'*.go' go.mod go.sum` names `internal/translate/export/rdf_expr.go`
(+12/−6), `rdf_identity.go` (+25/−15), `rdf_out.go` (+1/−1) and two
test files under `tests/export` — nothing outside
`internal/translate/export`. The export rows were re-probed a fourth time with the
same method, six interleaved rounds of `tests/export` (the six
`WriteAPIJSON` rows, its only benchmarks) and the stress `ConvertAPIJSON`
rows at `86fa09ec9` against `e022cbca0`, then the whole-binary element-ID
query, the RDF export and the 12 086-element load against the `e022cbca0`
binary. No package row moves: every `WriteAPIJSON` row writes byte-identical output (`ISQAtomicNuclear/
standard` 7.809 Mi, `then_after_members/compact` 346.1 Ki on both), bytes
and allocations are `~` on all twelve `WriteAPIJSON` rows and both
`ConvertAPIJSON` rows (`/32` 6.918 → 6.919 Gi, p = 0.24; `/128` 28.24 Gi
and 133.8 M allocations on both, p = 0.82 and 0.94), and sec/op is `~`
everywhere at 0–1 % steal. The whole-binary `-convert ttl` of the
12 086-element model, which writes the statements the fix adds, three
runs each after a warm-up: 13.4–15.0 → 12.7–17.2 s and 4.33–4.69 → 4.34–4.96
GiB resident, inside the spread the previous probe showed for the same
binary pair; the element-ID query on 6 198 elements 240–251 → 228–242
MiB, on 12 086 elements 363–380 → 352–366 MiB, three queries in one run
395–404 → 394–402 MiB; `-validate` 167–172 → 172–175 MiB at 0.24–0.30 s
on both; both binaries are 59 728 032 bytes. #1792 costs nothing this
record can measure.

After `86fa09ec9`, `develop` took six merges to `a37809a6c`, two of
them Go. #1795 (`fix(export): usages a declaration implies state
isVariable/mayTimeVary`, `0f3ab935b`) is on the semantics path and the
export path at once: `git diff --stat 86fa09ec9 0f3ab935b -- '*.go'
go.mod go.sum` names `internal/semantic/semantics/usage.go` (+24/−2:
three `UsageMayTimeVaryUnder*` helpers and the `SubjectMember` case),
`internal/translate/export/behavior.go` (+8/−3), `rdf_normative.go`
(+8/−3), `rdf_out.go` (+29/−5) and two test files. #1791
(`fix(runtime): a false assumption makes a requirement hold`,
`075a34339`) is on the runtime path: `internal/exec/runtime/condition.go`
(+101/−56, `judgeConditions` replacing `evaluateConditions` for a
requirement-kind check), `context.go` (+7/−9), `satisfy.go` (+3/−5) and
five test files. #1787 (`editors/vscode`), #1788 (this record), #1752
(`.github/dependabot.yml`) and #1775 (`docs/community.md`,
`overrides/home.html`) touch no Go: `git diff --stat 86fa09ec9 a37809a6c
-- '*.go' go.mod go.sum` names only #1795's and #1791's fifteen files.
Rather than one probe per merge, every measured row set was re-run at
`a37809a6c` against `86fa09ec9` — the last measured head, and the one
the runtime rows can be compared against too, since no file under
`internal/exec` or `internal/frontend` changed between `1e34d9d95` and
`86fa09ec9` — so that a later head needs only a diff check: six
interleaved rounds of the runtime and gRPC packages, repl `LoadModel`,
`internal/workspace/libs` and `model`, and the `tests/perf` load,
analysis, instantiation and satisfaction rows, with the steal percentage
logged per round, then the whole-binary floor, examples, generated models
and Apollo 11 (three runs each after a warm-up); then six interleaved
rounds of `tests/export` and the stress `ConvertAPIJSON` rows, then the
whole-binary element-ID query, RDF export and 12 086-element load.

The load and runtime rows do not move. Six rounds at 0.6–3.7 % steal,
`86fa09ec9` against `a37809a6c`, bytes and allocations equal on every
exact row — `DerivedReadWriteRead` 2 969 B · 69,
`SetFeatureValueNoDependents` 416 B · 9, `BehaviorOrdersLibraryModel`
142.1 Ki · 1 736, `IntegerArithmeticLoop` 296.3 Ki · 9 078,
`GRPCVerifyConstraint` 766.3 Ki · 14 886 in every sample, `FeaturesOf`
2.103 M, `AnalyzeRequirementGroup` and the parser rows — and `~` on the
rest: `AssignmentLoopStep` 925.8 → 925.7 Ki · 8 540 → 8 539, repl
`LoadModel/1000` 56.69 → 56.71 Mi · 563.8 k on both (so #1795's new
`mayTimeVary` helpers, which only the export calls, add nothing at
load, and #1784's +4.8 % stands as reported), `/0`, `/250` and `/4000`
alike, `REPLLoadFile` 593.9 → 593.8 Mi · 5.789 M, `Analyze/synthetic`
504.5 → 504.4 Mi · 5.178 M, `ImportValues` equal in count on all eight
rows, the `libs` and `model` packages equal in count on every row;
`InstantiateWarmModel` 122.8 → 118.7 Ki · 2 026 → 2 001, p = 0.13 and
0.14, inside its ± 4–5 % spread. Two rows cross p < 0.05 in count:
`BatchSatisfy` 3 997.5 k → 3 998.6 k allocations (+0.03 %, p = 0.002;
476.8 → 477.0 Mi, p = 0.065) — one more allocation per assertion
(+1 015 on the row's 1 000 assertions), #1791's verdict struct, the
only runtime change between the heads, at 0.3 ppm of the row's bytes;
and `IndexAddOnly/synthetic` 305.0 k → 308.1 k (+1.0 %, p = 0.015) at
± 1–3 %, whose per-round counts overlap (304.2–307.3 k against
306.8–316.0 k) on an indexing path neither merge touches — noise, not
read. sec/op is `~` on every runtime, gRPC and perf row this record
reports (`AssignmentLoopStep` 2.23 → 1.90 ms at ± 13–27 %, p = 0.24;
`BatchSatisfy` 1.41 → 1.91 ms per assertion, p = 0.18); the rows that
cross the threshold do so on the base column's spread
(`LoadModel/1000` 163 → 117 ms at ± 53 %, `IntegerArithmeticLoop` at
± 138 %, `Analyze/synthetic` 1.39 → 1.78 s at ± 55 %) and are not read.
Whole binary, three runs each after a warm-up, `86fa09ec9` against
`a37809a6c`: floor 0.02–0.04 s and 73–74 MiB on both, generated 12 086
elements 0.23–0.26 s and 159–170 → 167–168 MiB, 6 198 elements 128 → 127–128
MiB, Apollo 11 0.29–0.46 → 0.29–0.46 s and 169–176 → 170–173 MiB with
a byte-identical report, examples within 1 MiB; the binary grows
59 728 032 → 59 736 224 bytes (+8 KiB).

The export rows move by a tenth of a percent, #1795's second side. Six
rounds at 0–1 % steal, `86fa09ec9` against `a37809a6c`, the `tests/export`
and stress `ConvertAPIJSON` rows (`satellites=512` skipped as before):
`WriteAPIJSON` output is byte-identical on the two ISQ fixtures and a
few bytes smaller on the other four (`then_after_members/compact`
346.1 → 346.0 Ki; a usage a declaration implies now states what its
absence statement used to), B/op `~` on four rows (p = 0.09–0.31) and
+0.12–0.15 % on `then_after_members`, allocations `~` on the ISQ rows
and +0.06–0.12 % on the other four (`SimpleVehicleModel/standard`
1.722 → 1.723 M), all at p = 0.002; `ConvertAPIJSON/32` 6.919 →
6.928 Gi (+0.12 %) · 34.34 → 34.37 M allocations (+0.11 %), `/128`
28.24 → 28.27 Gi (+0.12 %) · 133.8 → 134.0 M (+0.11 %), p = 0.002 each;
sec/op `~` on every row (`/128` 47.9 → 51.3 s at ± 15–33 %, p = 0.24).
The cost is the derivation itself — `UsageMayTimeVaryUnder*` walks the
owning type's conformance for each connector end, parameter and subject
the writer meets — a sixth of what #1783 added, and like it not
something this record changes. Whole binary, three runs each after a
warm-up: `-convert ttl` of 12 086 elements 12.1–15.8 → 13.6–14.1 s and
4.07–4.32 → 4.12–4.74 GiB resident (inside the 4.3–4.9 GiB the previous
pair showed), the element-ID query of 12 086 elements 1.5–4.2 → 1.3–3.1 s
and 332–379 → 339–382 MiB, `-validate` of the same model 0.25–0.32 →
0.28–0.35 s and 160–170 → 160–176 MiB.

#1778 followed (`feat(engine): trace events for action
execution`, `develop` `df2bc173c`) is Go on the runtime and gRPC paths —
`git diff --stat a37809a6c df2bc173c -- '*.go' go.mod go.sum` names
`internal/exec/runtime/statements.go` (+104/−54), `context.go` (+67/−7),
`trace.go` (+81/−1), `action_executor.go` (+56/−3), `action_choice.go`,
`action_frame.go`, `action_statements.go`, `action_subflow.go`,
`internal/frontend/grpc/service.go` (+82/−9), `run_options.go`,
`docquery.go`, `internal/frontend/engine/engine.go` (+73/−10),
`json.go`, `internal/frontend/combined/combined.go`, the MCP runtime
tools, `internal/doc/queryexec`, `api/proto/sysml.pb.go` and the
`client/opensysml` Go client, plus tests. Six interleaved rounds of the
runtime and gRPC packages and the `tests/perf` gRPC and satisfaction
rows at 0.4–3.3 % steal, `a37809a6c` against `df2bc173c`: bytes and
allocations equal in every sample on the exact rows
(`DerivedReadWriteRead`, `SetFeatureValueNoDependents`, `NewRootFrame`,
`MaterializePlainAttributes` 621, `BehaviorOrdersLibraryModel` 1 736,
`IntegerArithmeticLoopBeyondInt64` 13 078, `GRPCVerifyConstraint`
766.3 Ki · 14 886, `GRPCEvaluate` 129, `GRPCParseFileCached`) and `~`
on the rest — `AssignmentLoopStep` 925.8 → 925.7 Ki · 8 539 on both,
`IntegerArithmeticLoop` 296.3 Ki · 9 078 on both, `InstantiateWarmModel`
117.7 → 118.9 Ki · 1 995 → 2 002 (p = 0.09 and 0.10), `BatchSatisfy`
477.4 → 477.3 Mi · 4.002 M on both, `GRPCParseFileUncached` 182.4 Mi ·
1.018 M on both. The trace records #1778 adds are built only when a
trace is asked for, and no benchmark asks. sec/op is `~` on every row
but `BatchSatisfy` (1.80 → 1.40 ms per assertion at ± 18–23 %,
p = 0.015, the faster side the new one; not read). Whole binary, three
runs after a warm-up: floor 0.02–0.03 s and 73–75 MiB on both, the
action-executor example 0.02–0.04 s on both, 12 086 elements 0.24–0.25 s
and 168–176 → 161–168 MiB; the binary grows 59 736 224 → 59 768 992 bytes
(+32 KiB, the trace event types and their protobuf).

Two more merges followed, both Go. #1797 (`fix(semantics): a variant
usage has no owning type`, `develop` `06be28ed6`) is
`internal/semantic/semantics/usage.go` +4/−2 — an `IsVariant(sym)` guard
in `UsageMayTimeVary` and `UsageMayTimeVaryForOwningType`, on the
derivation #1784 added — plus `may_time_vary_test.go` and
`tests/export/usage_variable_test.go`. Six interleaved rounds of repl
`LoadModel`, the runtime derived rows, grpc `InstantiateWarmModel`, the
libs and model packages and the corpus-backed `tests/perf` rows at
1.3–3.3 % steal, `df2bc173c` against `d16ca71e0`: bytes and allocations
equal in every sample or `~` on every row — `LoadModel/1000`
56.71 → 56.69 Mi · 563.8 k on both (#1784's finding stands as reported),
`/0` 238.1 Ki · 2 038, `/250` 14.30 Mi · 145.8 k and `/4000` 226.9 Mi ·
2.235 M on both; `REPLLoadFile` 594.1 → 593.5 Mi · 5.789 M on both,
`Analyze/synthetic` 504.5 → 504.3 Mi · 5.178 M, `ImportValues/nested`
(2 000 parts, 5 attributes) 493.2 → 493.5 Mi · 6.369 M, `FeaturesOf`
265.3 → 265.2 Mi · 2.103 M, `DerivedReadWriteRead` 2.969 Ki · 69,
`MaterializePlainAttributes` 621, `GRPCVerifyConstraint` 766.3 Ki ·
14 886 on both, `InstantiateWarmModel` 119.3 → 120.5 Ki · 2 006 → 2 012
(p = 0.24 and 0.22, inside its ± 1–2 %). The guard is one field read
per usage on a path that already walked to the owner; it allocates
nothing. sec/op is `~` on every row but `BatchSatisfy` (1.81 → 1.58 ms
per assertion at ± 15–38 %, p = 0.041, the faster side the new one; not
read). Whole binary, three runs after a warm-up: floor 0.02–0.04 s and
75–76 MiB on both, 12 086 elements 0.27–0.37 → 0.26–0.34 s and 173–177
→ 172–177 MiB, the Apollo 11 model 0.32–0.69 → 0.32–0.49 s with its
report byte-identical; the binaries are equal in size (59 768 992 bytes).

#1785 (`refactor(export): split writtenRelationships into per-source
helpers`, `develop` `d16ca71e0`) is
`internal/translate/export/element_ids.go` +95/−48 alone: the RDF
writer's relationship-ID bookkeeping, one function, becomes
`writtenEndFeatureReferences`, `writtenDependencyAnnotation`,
`writtenClauseRelationships`, `writtenBoundEndReferences` and their
helpers (`writtenHeadClauses`, `clauseObjects`, `referenceFormOf`,
`referenceSubsettingID`) for Sonar's cognitive-complexity rule — the
same lookups and the same maps and slices, in the same order. Six
interleaved rounds of `tests/export` and the stress `ConvertAPIJSON`
rows (`satellites=512` skipped, as above), `df2bc173c` against
`d16ca71e0` at 0–1 % steal: `WriteAPIJSON` output byte-identical on the
ISQ and `then_after_members` fixtures (7.809 Mi, 3.211 Mi, 859.3 Ki and
346.0 Ki in every sample) and 8 bytes larger on `SimpleVehicleModel`
(14 484 246 → 14 484 254 standard, 4 386 802 → 4 386 810 compact). Those
8 bytes are #1797's, not #1785's: the two outputs, diffed, differ only in
the `isVariable` and `mayTimeVary` of the model's four variant usages,
`true` → `false` (one byte per boolean), and the same two rows lose
40–50 allocations per op (1.723 M and 1.787 M on both sides to four
digits, p = 0.002) — the owner walk the guard now skips. B/op is equal
to four digits on every `WriteAPIJSON` row (212.6, 164.9, 349.8 → 349.9,
255.5 → 255.4, 24.64 and 19.75 Mi) and allocations `~` on the other four;
`ConvertAPIJSON/32` 6.929 Gi · 34.37 M and `/128` 28.27 Gi · 134.0 M on
both (p = 0.18–0.94); sec/op `~` on every row (`/128` 56.4 → 49.9 s at
± 10–13 %, p = 0.18). #1785 leaves no trace in any row. Whole binary,
three runs after a warm-up: `-convert ttl` of 12 086 elements
13.9–15.1 → 14.3–15.7 s and 3.9–4.9 → 3.9–4.8 GiB, the element-ID query
of the same model 1.4–4.1 → 1.4–4.8 s and 347–384 → 346–361 MiB,
`-validate` 0.26–0.34 → 0.25–0.30 s and 168–176 → 166–171 MiB; the
binaries are equal in size.

So the candidate of this record is `develop` at `d16ca71e0`: its runtime,
load, REPL, LSP and whole-binary rows measured at `cc1d477ba` and
re-checked at `1e34d9d95`, `9a4ffcd83` and `a37809a6c` on the rows above,
at `df2bc173c` on the runtime and gRPC rows and at `d16ca71e0` on the
load and derived rows (#1771 measured in its own pull request), its
export rows at `e022cbca0` and re-checked at `86fa09ec9`, `a37809a6c`
and `d16ca71e0` (and at `d01e4f49c` and `5e0322e0c` before #1783 and
#1774), and #1793, #1794, #1787, #1788, #1752 and #1775 Go-neutral by
diff.


### The quiet re-run

The time-only rows of [Re-runs](#re-runs) were re-run for the release
candidate the same way: six rounds, three columns interleaved, eight
packages, nothing else running (load 1.1–1.7 at the start of every
round). The host did not hold still for it. Hypervisor steal ran at
3–8 % through the afternoon (`vmstat`), and 0.11.0 rebuilt — the same
binary that held `AssignmentLoopStep` at 1.52–1.56 ms over the six
rounds of the first quiet run — swung 1.69–3.54 ms over these six;
every column slowed alike and the spreads came out at 25–170 %. Bytes
and allocations are unaffected (exact, and equal to the suite's), but
those rounds decide no time row: `RealDecimalLoop` (−30 %, p=0.04),
lsp `FormatEdits` (−14 %, p=0.04) and `WriteAPIJSON/ISQAtomicNuclear/standard`
(−40 %, p=0.03) cross the threshold in the candidate's favour and
`IndexAddExpand/synthetic` against 0.11.0 as released (−40 %, p=0.01);
everything else — `AssignmentLoopStep`, `DerivedReadWriteRead`,
`IntegerArithmeticLoop`, `SetFeatureValueNoDependents`,
`InstantiateWarmModel`, `GRPCVerifyConstraint`, `BatchConstraints`,
`ExecuteActionFreshContext`, the stress rows — is `~`. The rows are in
the raw results; they are not read here.

A second, smaller re-run of the same rows (runtime, grpc, lsp
`FormatEdits`, repl `LoadModel`, the corpus-backed `tests/perf` rows,
with the steal percentage logged per round) was started once the
export probe finished and cancelled before its first round completed:
the release was waiting on this record. The time verdicts of
[Re-runs](#re-runs) — taken on the pre-fix candidate, on the quiet host
of the first pass — stand as the record's time verdicts, and the fixes'
own quiet A/B ([The fixes on the quiet host](#the-fixes-on-the-quiet-host))
is the evidence that the fixes moved bytes, not time.

Should anything that touches Go land between `d16ca71e0` and the
release branch after all, its rows are re-probed interleaved against
`cc1d477ba` and this section is amended, as the 0.11.0 record's was.


## Develop after the candidate

This is a new three-revision measurement, not an extrapolation from a pull
request. The preceding section says: “So the candidate of this record is
`develop` at `d16ca71e0`”. That is the second column, rather than the earlier
`b4d0a35c4`.

| Column | Revision |
|---|---|
| 0.11.0 rebuilt | `ddf4df00037d6ddbfb1fa9d743a4389f75c9e2dc` (`v0.11.0`) |
| Accounted develop | `d16ca71e0a56eb75022ed7e8df037737e2de8cc5` |
| Pinned later develop | `5edbe6686009ae84b198e94493e69d316ae46fb7` |

All three use Go 1.27.3, linux/amd64, the same Intel Xeon KVM host with eight
virtual CPUs and `GOMAXPROCS=8`. Each detached worktree was built with
`make build`; the pilot and training corpora were real copies, not symlinks.
Six rounds include all three configurations, interleaved, using
`-run '^$' -bench . -benchmem -count 1`. The long stress suite starts first;
its 512-satellite conversion is excluded as in Method. The known failing
rows in Method remain failures on all three revisions, not performance
observations. CPU model, toolchain, GOMAXPROCS and per-round `vmstat` are
recorded. There is no other local benchmark/build during a measurement,
but the hypervisor still steals CPU time (including samples around 2–3%).
Timing therefore needs confirmation; allocations are the stronger evidence.

Every metric significant between the second and third columns in either
the original suite or its six-round quiet confirmation appears below,
including improvements, output sizes, counts, custom per-element units and
movements that did not confirm. The three value columns are the original
six-round medians, never mixed with medians from a different benchmark
invocation. “Quiet” reports the accounted-to-pinned delta and benchstat
test from the confirmation; `~` means not significant there. A dash means
that metric was not emitted on that revision. Bytes are bytes (not MiB)
unless a table says otherwise. The last column distinguishes required
semantic cost, avoidable work and an unconfirmed observation; a significant
allocation change alone does not establish an avoidable regression.

### Every moved benchmark metric

#### `internal/exec/runtime`

| Benchmark | Metric | 0.11.0 rebuilt | Accounted develop | Pinned develop | Quiet confirmation | Cause and verdict |
|---|---|---:|---:|---:|---|---|
| `BehaviorOrdersLibraryModel` | B/op | 144,976 | 145,472 | 145,632 | +0.11%; p=0.002 n=6 | Eager expression-memo allocation; #1826 removes unused memo tables, with measured residuals below. |
| `BehaviorOrdersLibraryModel` | allocs/op | 1,727 | 1,736 | 1,738 | +0.12%; p=0.002 n=6 | Eager expression-memo allocation; #1826 removes unused memo tables, with measured residuals below. |
| `IntegerArithmeticNestedLoop` | sec/op | 0.00528667 | 0.00641081 | 0.00548265 | ~ | Timing not reliably reproduced as a slowdown on the shared host; no timing fix claimed. |

#### `internal/frontend/grpc`

| Benchmark | Metric | 0.11.0 rebuilt | Accounted develop | Pinned develop | Quiet confirmation | Cause and verdict |
|---|---|---:|---:|---:|---|---|
| `ParseFileColdShared` | B/op | 4,929,591.5 | 4,946,979 | 4,947,694 | ~ | Initial movement not significant in the quiet confirmation; no regression established. |
| `ParseFileColdShared` | allocs/op | 31,192 | 31,300.5 | 31,310.5 | +0.04%; p=0.009 n=6 | Larger semantic snapshot/model construction; required members plus the separately measured lazy-memo opportunity. |

#### `internal/frontend/lsp`

| Benchmark | Metric | 0.11.0 rebuilt | Accounted develop | Pinned develop | Quiet confirmation | Cause and verdict |
|---|---|---:|---:|---:|---|---|
| `ReferencesCold` | B/op | 142,437,336 | 142,558,502.5 | 145,783,155.5 | +2.30%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |

#### `internal/frontend/repl`

| Benchmark | Metric | 0.11.0 rebuilt | Accounted develop | Pinned develop | Quiet confirmation | Cause and verdict |
|---|---|---:|---:|---:|---|---|
| `Diagnostics/attributes=200` | B/diagnostic | 908.8 | 908.8 | 909.6 | +0.09%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `Diagnostics/attributes=200` | B/op | 363,519 | 363,521.5 | 363,841 | +0.09%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `Diagnostics/attributes=200` | allocs/op | 1,732 | 1,732 | 1,736 | +0.23%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `Diagnostics/attributes=50` | B/diagnostic | 1,037 | 1,037 | 1,040 | +0.29%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `Diagnostics/attributes=50` | B/op | 103,716 | 103,717 | 104,036.5 | +0.31%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `Diagnostics/attributes=50` | allocs/op | 670 | 670 | 674 | +0.60%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `Diagnostics/attributes=800` | B/diagnostic | 806.2 | 806.2 | 806.4 | +0.02%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `Diagnostics/attributes=800` | B/op | 1,289,845 | 1,289,855 | 1,290,177.5 | +0.02%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `Diagnostics/attributes=800` | allocs/op | 6,490 | 6,490 | 6,494 | +0.06%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `LoadModel/elements=0` | B/op | 233,433 | 244,033.5 | 244,513 | +0.20%; p=0.002 n=6 | Eager expression-memo allocation; #1826 removes unused memo tables, with measured residuals below. |
| `LoadModel/elements=0` | allocs/op | 2,022 | 2,038 | 2,044 | +0.29%; p=0.002 n=6 | Eager expression-memo allocation; #1826 removes unused memo tables, with measured residuals below. |
| `LoadModel/elements=1000` | B/element | 58,272.5 | 59,460.5 | 59,505 | ~ | Initial movement not significant in the quiet confirmation; no regression established. |
| `LoadModel/elements=1000` | B/op | 58,272,624 | 59,460,800.5 | 59,505,279 | ~ | Initial movement not significant in the quiet confirmation; no regression established. |
| `LoadModel/elements=1000` | allocs/op | 533,102 | 563,779 | 564,985 | +0.21%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `LoadModel/elements=250` | B/element | 58,559.5 | 59,988 | 60,054 | +0.11%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `LoadModel/elements=250` | B/op | 14,639,872 | 14,997,111 | 15,013,603 | +0.11%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `LoadModel/elements=250` | allocs/op | 137,891 | 145,763 | 146,069 | +0.21%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `LoadModel/elements=4000` | B/element | 58,292.5 | 59,488.5 | 59,536 | +0.12%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `LoadModel/elements=4000` | B/op | 233,170,360 | 237,952,424 | 238,143,860 | +0.12%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `LoadModel/elements=4000` | allocs/op | 2,113,421 | 2,235,321.5 | 2,240,125 | +0.22%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `RunCalc/elements=4000` | sec/op | 5.224e-06 | 5.9195e-06 | 4.632e-06 | ~ | Timing not reliably reproduced as a slowdown on the shared host; no timing fix claimed. |

#### `internal/workspace/libs`

| Benchmark | Metric | 0.11.0 rebuilt | Accounted develop | Pinned develop | Quiet confirmation | Cause and verdict |
|---|---|---:|---:|---:|---|---|
| `DecodeSnapshot` | B/op | 40,835,109.5 | 41,083,550 | 41,190,935.5 | +0.26%; p=0.002 n=6 | Larger semantic snapshot/model construction; required members plus the separately measured lazy-memo opportunity. |
| `DecodeSnapshot` | allocs/op | 72,057 | 72,090 | 72,093 | +0.00%; p=0.002 n=6 | Larger semantic snapshot/model construction; required members plus the separately measured lazy-memo opportunity. |
| `ExpandWildcardImports` | B/op | 16,103,843.5 | 16,104,524 | 16,104,539 | +0.00%; p=0.002 n=6 | Larger semantic snapshot/model construction; required members plus the separately measured lazy-memo opportunity. |
| `ExpandWildcardImports` | allocs/op | 145,704 | 145,724 | 145,725 | +0.00%; p=0.002 n=6 | Larger semantic snapshot/model construction; required members plus the separately measured lazy-memo opportunity. |
| `IndexLibrary` | B/op | 14,770,056.5 | 14,775,739 | 14,777,888.5 | +0.01%; p=0.002 n=6 | Larger semantic snapshot/model construction; required members plus the separately measured lazy-memo opportunity. |
| `IndexLibrary` | allocs/op | 108,116 | 108,190 | 108,207 | +0.02%; p=0.002 n=6 | Larger semantic snapshot/model construction; required members plus the separately measured lazy-memo opportunity. |

#### `internal/workspace/model`

| Benchmark | Metric | 0.11.0 rebuilt | Accounted develop | Pinned develop | Quiet confirmation | Cause and verdict |
|---|---|---:|---:|---:|---|---|
| `AnalyseResolved` | B/op | 664,547 | 665,426.5 | 666,483 | +0.16%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `AnalyseResolved` | allocs/op | 7,701 | 7,701 | 7,704 | +0.04%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `AnalyseResolved` | sec/op | 0.00184853 | 0.00168742 | 0.00156899 | ~ | Timing not reliably reproduced as a slowdown on the shared host; no timing fix claimed. |
| `AnalyseUnresolved` | B/op | 14,521,736 | 14,560,248.5 | 14,567,633 | +0.05%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `AnalyseUnresolved` | allocs/op | 151,263 | 151,903 | 152,057 | +0.10%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |

#### `tests/codegen`

| Benchmark | Metric | 0.11.0 rebuilt | Accounted develop | Pinned develop | Quiet confirmation | Cause and verdict |
|---|---|---:|---:|---:|---|---|
| `CompiledCalc/Fib(25)/go` | sec/op | 0.00184215 | 0.0020508 | 0.0017872 | ~ | Timing not reliably reproduced as a slowdown on the shared host; no timing fix claimed. |
| `CompiledCalc/Hypot(3.0,4.0)/interpreted` | B/op | 8,450.5 | 8,557 | 8,543.5 | ~ | Initial movement not significant in the quiet confirmation; no regression established. |

#### `tests/export`

| Benchmark | Metric | 0.11.0 rebuilt | Accounted develop | Pinned develop | Quiet confirmation | Cause and verdict |
|---|---|---:|---:|---:|---|---|
| `WriteAPIJSON/ISQAtomicNuclear/compact` | B/op | 202,600,375.5 | 172,903,605 | 173,436,091 | +0.31%; p=0.002 n=6 | Required extra serialized identities/properties; the scanner fix removes some ISQ allocation overhead, not the whole writer increase. |
| `WriteAPIJSON/ISQAtomicNuclear/compact` | allocs/op | 1,800,335.5 | 1,446,802.5 | 1,465,033.5 | +1.26%; p=0.002 n=6 | Required extra serialized identities/properties; the scanner fix removes some ISQ allocation overhead, not the whole writer increase. |
| `WriteAPIJSON/ISQAtomicNuclear/compact` | output-B | 3,335,945 | 3,366,806 | 3,376,236 | +0.28%; p=0.002 n=6 | Required additional semantic members/output; keep the corrected result. |
| `WriteAPIJSON/ISQAtomicNuclear/standard` | B/op | 261,963,284 | 222,958,111.5 | 223,513,690.5 | +0.25%; p=0.002 n=6 | Required extra serialized identities/properties; the scanner fix removes some ISQ allocation overhead, not the whole writer increase. |
| `WriteAPIJSON/ISQAtomicNuclear/standard` | allocs/op | 1,865,984.5 | 1,391,962 | 1,410,195.5 | +1.31%; p=0.002 n=6 | Required extra serialized identities/properties; the scanner fix removes some ISQ allocation overhead, not the whole writer increase. |
| `WriteAPIJSON/ISQAtomicNuclear/standard` | output-B | 8,155,901 | 8,188,286 | 8,198,556 | +0.13%; p=0.002 n=6 | Required additional semantic members/output; keep the corrected result. |
| `WriteAPIJSON/SimpleVehicleModel/compact` | B/op | 283,994,471.5 | 267,949,469 | 268,703,368.5 | +0.30%; p=0.002 n=6 | Required extra serialized identities/properties; the scanner fix removes some ISQ allocation overhead, not the whole writer increase. |
| `WriteAPIJSON/SimpleVehicleModel/compact` | allocs/op | 2,123,053 | 1,787,388 | 1,807,883 | +1.15%; p=0.002 n=6 | Required extra serialized identities/properties; the scanner fix removes some ISQ allocation overhead, not the whole writer increase. |
| `WriteAPIJSON/SimpleVehicleModel/compact` | output-B | 4,333,342 | 4,386,810 | 4,383,470 | -0.08%; p=0.002 n=6 | Required additional semantic members/output; keep the corrected result. |
| `WriteAPIJSON/SimpleVehicleModel/standard` | B/op | 403,162,140 | 366,537,822 | 367,537,505 | +0.26%; p=0.002 n=6 | Required extra serialized identities/properties; the scanner fix removes some ISQ allocation overhead, not the whole writer increase. |
| `WriteAPIJSON/SimpleVehicleModel/standard` | allocs/op | 2,200,094 | 1,723,092.5 | 1,743,583.5 | +1.19%; p=0.002 n=6 | Required extra serialized identities/properties; the scanner fix removes some ISQ allocation overhead, not the whole writer increase. |
| `WriteAPIJSON/SimpleVehicleModel/standard` | output-B | 14,431,120 | 14,484,254 | 14,470,202 | -0.10%; p=0.002 n=6 | Required additional semantic members/output; keep the corrected result. |
| `WriteAPIJSON/then_after_members/compact` | B/op | 23,210,201.5 | 20,710,441.5 | 20,688,777 | -0.10%; p=0.002 n=6 | Smaller allocation result retained; no regression or corrective change inferred. |
| `WriteAPIJSON/then_after_members/compact` | allocs/op | 209,705.5 | 174,467 | 176,506 | +1.17%; p=0.002 n=6 | Required extra serialized identities/properties; the scanner fix removes some ISQ allocation overhead, not the whole writer increase. |
| `WriteAPIJSON/then_after_members/compact` | output-B | 346,288 | 354,316 | 345,214 | -2.57%; p=0.002 n=6 | Required additional semantic members/output; keep the corrected result. |
| `WriteAPIJSON/then_after_members/standard` | B/op | 29,336,603.5 | 25,840,714 | 25,802,511.5 | -0.15%; p=0.002 n=6 | Smaller allocation result retained; no regression or corrective change inferred. |
| `WriteAPIJSON/then_after_members/standard` | allocs/op | 218,568.5 | 169,632 | 171,670.5 | +1.20%; p=0.002 n=6 | Required extra serialized identities/properties; the scanner fix removes some ISQ allocation overhead, not the whole writer increase. |
| `WriteAPIJSON/then_after_members/standard` | output-B | 869,946 | 879,885 | 866,269 | -1.55%; p=0.002 n=6 | Required additional semantic members/output; keep the corrected result. |

#### `tests/perf`

| Benchmark | Metric | 0.11.0 rebuilt | Accounted develop | Pinned develop | Quiet confirmation | Cause and verdict |
|---|---|---:|---:|---:|---|---|
| `Analyze/synthetic` | B/op | 514,661,448 | 529,089,728 | 532,988,024 | +0.69%; p=0.002 n=6 | Subject/role semantics plus avoidable member copying/model memos; partial fixes, not a whole-row recovery. |
| `Analyze/synthetic` | allocs/op | 4,800,719.5 | 5,178,280.5 | 5,289,276.5 | +2.14%; p=0.002 n=6 | Subject/role semantics plus avoidable member copying/model memos; partial fixes, not a whole-row recovery. |
| `AnalyzeRequirementGroup/members=16` | B/op | 486,986 | 487,577 | 487,993 | +0.09%; p=0.002 n=6 | Body copying plus model memos: avoidable portion addressed by #1824 and #1826; residual stated below. |
| `AnalyzeRequirementGroup/members=16` | allocs/op | 5,532 | 5,532 | 5,535 | +0.05%; p=0.002 n=6 | Body copying plus model memos: avoidable portion addressed by #1824 and #1826; residual stated below. |
| `AnalyzeRequirementGroup/members=32` | B/op | 792,362.5 | 793,465 | 794,137 | +0.08%; p=0.002 n=6 | Body copying plus model memos: avoidable portion addressed by #1824 and #1826; residual stated below. |
| `AnalyzeRequirementGroup/members=32` | allocs/op | 8,861 | 8,861 | 8,864 | +0.03%; p=0.002 n=6 | Body copying plus model memos: avoidable portion addressed by #1824 and #1826; residual stated below. |
| `AnalyzeRequirementGroup/members=64` | B/op | 1,424,209 | 1,426,329.5 | 1,427,644.5 | +0.09%; p=0.002 n=6 | Body copying plus model memos: avoidable portion addressed by #1824 and #1826; residual stated below. |
| `AnalyzeRequirementGroup/members=64` | allocs/op | 15,377 | 15,377 | 15,380 | +0.02%; p=0.002 n=6 | Body copying plus model memos: avoidable portion addressed by #1824 and #1826; residual stated below. |
| `AnalyzeRequirementGroup/members=8` | B/op | 311,849 | 312,183 | 312,471 | +0.09%; p=0.002 n=6 | Body copying plus model memos: avoidable portion addressed by #1824 and #1826; residual stated below. |
| `AnalyzeRequirementGroup/members=8` | allocs/op | 3,730 | 3,730 | 3,733 | +0.08%; p=0.002 n=6 | Body copying plus model memos: avoidable portion addressed by #1824 and #1826; residual stated below. |
| `BatchSatisfy` | B/op | 476,820,664 | 487,621,456 | 492,720,392 | +0.99%; p=0.002 n=6 | New satisfy subjects, role redefinitions and their semantic queries; required correctness cost. |
| `BatchSatisfy` | allocs/op | 4,201,755.5 | 3,815,494.5 | 3,959,610 | +3.59%; p=0.002 n=6 | New satisfy subjects, role redefinitions and their semantic queries; required correctness cost. |
| `ExecuteState` | B/op | 903,955 | 892,256 | 910,095.5 | ~ | Initial movement not significant in the quiet confirmation; no regression established. |
| `ExecuteState` | allocs/op | 14,965.5 | 12,494 | 12,492 | ~ | Initial movement not significant in the quiet confirmation; no regression established. |
| `FQNOf` | B/op | 735,040 | 735,040 | 743,040 | +1.09%; p=0.002 n=6 | More indexed expression/subject members; total work grows, not a new per-symbol lookup algorithm. |
| `FQNOf` | allocs/op | 40,000 | 40,000 | 41,000 | +2.50%; p=0.002 n=6 | More indexed expression/subject members; total work grows, not a new per-symbol lookup algorithm. |
| `FQNOf` | sec/op | 0.00152981 | 0.00169912 | 0.00189099 | -21.03%; p=0.009 n=6 | Timing not reliably reproduced as a slowdown on the shared host; no timing fix claimed. |
| `FQNOf` | sec/sym | 3.8245e-08 | 4.2475e-08 | 4.612e-08 | -22.95%; p=0.009 n=6 | Timing not reliably reproduced as a slowdown on the shared host; no timing fix claimed. |
| `FQNOf` | syms | 40,001 | 40,001 | 41,001 | +2.50%; p=0.002 n=6 | Required additional semantic members/output; keep the corrected result. |
| `GRPCParseFileUncached` | B/op | 190,090,620.5 | 190,267,051.5 | 203,528,038 | +5.91%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `ImportValues/flat/parts=100/attrs=3` | B/op | 13,190,096.5 | 13,650,022.5 | 13,660,661.5 | +0.08%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `ImportValues/flat/parts=100/attrs=3` | allocs/op | 163,938 | 172,901 | 173,115 | +0.12%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `ImportValues/flat/parts=2000/attrs=3` | B/op | 293,943,220 | 300,147,360 | 300,340,394 | +0.06%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `ImportValues/flat/parts=2000/attrs=3` | allocs/op | 3,082,074.5 | 3,254,498 | 3,258,512.5 | +0.12%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `ImportValues/flat/parts=2000/attrs=5` | B/op | 412,497,402 | 423,149,442 | 423,471,190 | +0.07%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `ImportValues/flat/parts=2000/attrs=5` | allocs/op | 4,613,727.5 | 4,906,220 | 4,910,230.5 | +0.08%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `ImportValues/flat/parts=500/attrs=3` | B/op | 70,188,405 | 71,720,178.5 | 71,769,019 | +0.07%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `ImportValues/flat/parts=500/attrs=3` | allocs/op | 777,675 | 821,043.5 | 822,057 | +0.12%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `ImportValues/nested/parts=100/attrs=3` | B/op | 18,179,009 | 19,270,408.5 | 19,284,225.5 | +0.07%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `ImportValues/nested/parts=100/attrs=3` | allocs/op | 219,810 | 249,959 | 250,373 | +0.17%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `ImportValues/nested/parts=2000/attrs=3` | B/op | 361,004,084 | 382,565,642 | 382,822,608 | +0.07%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `ImportValues/nested/parts=2000/attrs=3` | allocs/op | 4,167,427 | 4,763,835.5 | 4,771,855 | +0.17%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `ImportValues/nested/parts=2000/attrs=5` | allocs/op | 5,652,108.5 | 6,368,619 | 6,376,587.5 | +0.13%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `ImportValues/nested/parts=500/attrs=3` | B/op | 87,251,431.5 | 92,622,080 | 92,687,149 | +0.07%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `ImportValues/nested/parts=500/attrs=3` | allocs/op | 1,050,399 | 1,199,763.5 | 1,201,777 | +0.17%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `IndexAddExpand/synthetic` | B/op | 44,844,247 | 44,972,281.5 | 45,301,916.5 | +2.66%; p=0.015 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `IndexAddExpand/synthetic` | allocs/op | 317,285.5 | 318,335.5 | 322,729.5 | +3.68%; p=0.009 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `IndexAddExpand/vehicle` | B/op | 15,885,222.5 | 15,895,229 | 16,029,785.5 | +0.94%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `IndexAddExpand/vehicle` | allocs/op | 137,688 | 137,700 | 138,090 | +0.29%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `IndexAddOnly/synthetic` | allocs/op | 307,275.5 | 308,701 | 313,360.5 | +2.40%; p=0.015 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `IndexAddOnly/vehicle` | B/op | 1,193,189.5 | 1,193,254.5 | 1,367,512.5 | +14.61%; p=0.002 n=6 | Required conjugated-port registrations cross Go map capacity boundaries; no benchmark-specific reservation shipped. |
| `IndexAddOnly/vehicle` | allocs/op | 9,612.5 | 9,613 | 9,980.5 | +3.82%; p=0.002 n=6 | Required conjugated-port registrations cross Go map capacity boundaries; no benchmark-specific reservation shipped. |
| `Instantiate` | B/op | 215,406,876 | 320,848,240 | 218,254,200 | -32.00%; p=0.002 n=6 | Smaller allocation result retained; no regression or corrective change inferred. |
| `Instantiate` | allocs/op | 2,107,747.5 | 2,495,279 | 1,732,203.5 | -30.58%; p=0.002 n=6 | Smaller allocation result retained; no regression or corrective change inferred. |
| `Instantiate` | sec/inst | 0.000677159 | 0.00118005 | 0.000640353 | -58.10%; p=0.026 n=6 | Timing not reliably reproduced as a slowdown on the shared host; no timing fix claimed. |
| `Instantiate` | sec/op | 0.677159 | 1.18005 | 0.640353 | -58.10%; p=0.026 n=6 | Timing not reliably reproduced as a slowdown on the shared host; no timing fix claimed. |
| `REPLLoadFile` | B/op | 652,072,008 | 622,713,416 | 626,721,488 | +0.64%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `REPLLoadFile` | allocs/op | 5,758,271 | 5,789,380.5 | 5,901,390.5 | +1.93%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `REPLLoadFile` | sec/op | 1.53615 | 1.68219 | 1.53979 | -31.99%; p=0.041 n=6 | Timing not reliably reproduced as a slowdown on the shared host; no timing fix claimed. |
| `REPLSubmitSnippet` | B/op | 591,369,608 | 606,245,848 | 611,464,576 | +0.84%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `REPLSubmitSnippet` | allocs/op | 5,613,690.5 | 5,991,307 | 6,107,356.5 | +1.94%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `WorkspaceEdit/synthetic` | B/op | 90,906,081 | 91,156,482 | 91,754,430.5 | +0.43%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `WorkspaceEdit/synthetic` | allocs/op | 680,523.5 | 680,522 | 687,524.5 | +1.03%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `WorkspaceEdit/synthetic/reindex+diagnostics` | B/op | 538,079,520 | 552,345,480 | 569,459,152 | +2.95%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `WorkspaceEdit/synthetic/reindex+diagnostics` | allocs/op | 5,248,490.5 | 5,625,615.5 | 5,744,625.5 | +2.11%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `WorkspaceEdit/vehicle` | B/op | 15,240,223 | 15,231,887.5 | 15,285,616 | +0.48%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `WorkspaceEdit/vehicle` | allocs/op | 166,074.5 | 166,073.5 | 166,595 | +0.31%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `WorkspaceEdit/vehicle/reindex+diagnostics` | B/op | 153,895,739.5 | 152,962,798 | 164,395,061 | +7.50%; p=0.002 n=6 | Expanded conjugate namespace enumeration and element-set growth; required members retained (profile below). |
| `WorkspaceEdit/vehicle/reindex+diagnostics` | allocs/op | 921,411.5 | 909,707.5 | 914,749.5 | +0.55%; p=0.002 n=6 | Expanded conjugate namespace enumeration and element-set growth; required members retained (profile below). |
| `WorkspaceEditSmallDocBesideLarge` | B/op | 26,080,908.5 | 26,146,577 | 26,947,215 | +3.05%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `WorkspaceEditSmallDocBesideLarge` | allocs/op | 114,689 | 114,707 | 114,708.5 | ~ | Initial movement not significant in the quiet confirmation; no regression established. |

#### `tests/stressmodel`

| Benchmark | Metric | 0.11.0 rebuilt | Accounted develop | Pinned develop | Quiet confirmation | Cause and verdict |
|---|---|---:|---:|---:|---|---|
| `ConvertAPIJSON/satellites=128/elements=24167` | B/op | 32,437,800,128 | 30,359,675,952 | 30,379,125,248 | +0.07%; p=0.002 n=6 | Larger correct semantic/output graph plus avoidable address splitting; address fix and residual below. |
| `ConvertAPIJSON/satellites=128/elements=24167` | allocs/op | 159,810,293.5 | 133,957,186 | 134,478,051.5 | +0.39%; p=0.002 n=6 | Larger correct semantic/output graph plus avoidable address splitting; address fix and residual below. |
| `ConvertAPIJSON/satellites=32/elements=6227` | B/op | 7,848,764,948 | 7,438,934,836 | 7,445,502,600 | +0.11%; p=0.004 n=6 | Larger correct semantic/output graph plus avoidable address splitting; address fix and residual below. |
| `ConvertAPIJSON/satellites=32/elements=6227` | B/s | 11.8 MB/s | 12.42 MB/s | 11.62 MB/s | -7.35%; p=0.041 n=6 | Timing movement failed the additional six-round confirmation; host/GC-sensitive, not an established source regression. |
| `ConvertAPIJSON/satellites=32/elements=6227` | allocs/op | 40,294,864.5 | 34,373,302.5 | 34,507,432 | +0.39%; p=0.002 n=6 | Larger correct semantic/output graph plus avoidable address splitting; address fix and residual below. |
| `ConvertAPIJSON/satellites=32/elements=6227` | sec/op | 12.5992 | 11.9716 | 12.823 | +7.90%; p=0.041 n=6 | Timing movement failed the additional six-round confirmation; host/GC-sensitive, not an established source regression. |
| `EditBeside/satellites=128` | B/op | 1,922,511.5 | 1,926,680.5 | 1,992,493 | +3.36%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `EditBeside/satellites=32` | B/op | 598,379 | 599,521 | 613,691.5 | +2.33%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `EditBeside/satellites=512` | B/op | 7,342,103.5 | 7,364,479.5 | 7,574,767 | +2.91%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `EditImported/satellites=128/files=6` | B/op | 536,775,156 | 563,230,252 | 565,552,660 | +0.43%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `EditImported/satellites=128/files=6` | allocs/op | 8,673,192 | 9,475,181 | 9,520,836.5 | +0.48%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `EditImported/satellites=32/files=6` | B/op | 158,593,088 | 165,389,696 | 165,590,676 | +0.21%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `EditImported/satellites=32/files=6` | allocs/op | 2,498,021 | 2,702,123.5 | 2,713,843 | +0.43%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `EditImported/satellites=512/files=6` | B/op | 2,119,725,416 | 2,225,800,400 | 2,241,323,928 | +0.71%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `EditImported/satellites=512/files=6` | allocs/op | 33,369,928.5 | 36,563,577 | 36,744,915.5 | +0.50%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `FleetSatisfy/satellites=128/assertions=36` | B/op | 943,433.5 | 949,006.5 | 949,464.5 | +0.05%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `FleetSatisfy/satellites=128/assertions=36` | allocs/op | 16,202 | 15,595.5 | 15,636.5 | +0.26%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `FleetSatisfy/satellites=32/assertions=24` | B/op | 610,149.5 | 612,970.5 | 613,415.5 | +0.07%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `FleetSatisfy/satellites=32/assertions=24` | allocs/op | 10,460.5 | 10,060.5 | 10,091.5 | +0.30%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `FleetSatisfy/satellites=512/assertions=108` | allocs/op | 49,852 | 48,006 | 48,103.5 | +0.26%; p=0.013 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `HydratePlane/satellites=128/files=6` | B/op | 66,519,417.5 | 66,532,436.5 | 66,049,675 | -0.68%; p=0.009 n=6 | Smaller allocation result retained; no regression or corrective change inferred. |
| `HydratePlane/satellites=128/files=6` | allocs/op | 781,773 | 782,202 | 785,044.5 | +0.36%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `HydratePlane/satellites=32/files=6` | B/op | 23,359,729 | 23,399,837.5 | 23,144,916 | -0.92%; p=0.002 n=6 | Smaller allocation result retained; no regression or corrective change inferred. |
| `HydratePlane/satellites=32/files=6` | allocs/op | 286,502.5 | 286,935 | 287,641 | +0.25%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `HydratePlane/satellites=512/files=6` | B/op | 241,648,596 | 242,380,348 | 243,491,896 | +0.70%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `HydratePlane/satellites=512/files=6` | allocs/op | 2,761,708 | 2,762,163 | 2,773,614.5 | +0.42%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `Load/satellites=128/elements=24167` | B/element | 27,671 | 28,824.5 | 29,194 | +1.28%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `Load/satellites=128/elements=24167` | B/op | 668,728,696 | 696,600,140 | 705,526,936 | +1.28%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `Load/satellites=128/elements=24167` | allocs/op | 9,432,147.5 | 10,234,741.5 | 10,281,295.5 | +0.45%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `Load/satellites=128/elements=24167` | live-B/element | 2,534 | 2,550 | 2,559 | +0.35%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `Load/satellites=128/elements=24167` | live-B/op | 61,241,336 | 61,631,960 | 61,835,480 | +0.33%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `Load/satellites=32/elements=6227` | B/element | 28,805 | 29,951.5 | 30,289 | +1.13%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `Load/satellites=32/elements=6227` | B/op | 179,369,521 | 186,507,048 | 188,610,822 | +1.13%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `Load/satellites=32/elements=6227` | allocs/op | 2,441,050 | 2,645,750.5 | 2,657,641 | +0.45%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `Load/satellites=32/elements=6227` | live-B/element | 2,911 | 2,927 | 2,936 | +0.31%; p=0.013 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `Load/satellites=32/elements=6227` | live-B/op | 18,127,424 | 18,227,032 | 18,281,752 | +0.30%; p=0.035 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `Load/satellites=512/elements=95927` | B/element | 27,396 | 28,545.5 | 28,883 | +1.21%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `Load/satellites=512/elements=95927` | B/op | 2,628,010,464 | 2,738,268,240 | 2,770,637,416 | +1.21%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `Load/satellites=512/elements=95927` | allocs/op | 37,397,514 | 40,591,753.5 | 40,776,642.5 | +0.46%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `Load/satellites=512/elements=95927` | live-B/element | 2,447 | 2,463.5 | 2,472 | +0.37%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `Load/satellites=512/elements=95927` | live-B/op | 234,728,880 | 236,329,292 | 237,107,656 | +0.36%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `LoadFiles/satellites=128/files=6` | B/op | 662,117,000 | 689,614,780 | 694,538,244 | +0.82%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `LoadFiles/satellites=128/files=6` | allocs/op | 8,817,015.5 | 9,619,905 | 9,664,914.5 | +0.47%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `LoadFiles/satellites=32/files=6` | B/op | 195,497,042 | 202,344,536 | 203,556,956 | +0.51%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `LoadFiles/satellites=32/files=6` | allocs/op | 2,443,823 | 2,648,775 | 2,660,270 | +0.43%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `LoadFiles/satellites=512/files=6` | B/op | 2,592,073,912 | 2,699,569,232 | 2,719,518,208 | +0.73%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `LoadFiles/satellites=512/files=6` | allocs/op | 34,302,960.5 | 37,497,478 | 37,676,271 | +0.48%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `OpenSplit/satellites=128/files=6/cold` | B/op | 762,814,088 | 790,461,950 | 794,226,478 | +0.47%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `OpenSplit/satellites=128/files=6/cold` | allocs/op | 10,760,683 | 11,562,179.5 | 11,609,286 | +0.42%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `OpenSplit/satellites=128/files=6/warm` | B/op | 224,628,979.5 | 224,671,400 | 227,232,812.5 | +1.11%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `OpenSplit/satellites=128/files=6/warm` | allocs/op | 1,753,985.5 | 1,754,799 | 1,765,820.5 | +0.63%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `OpenSplit/satellites=32/files=6/cold` | B/op | 230,353,311.5 | 237,900,395 | 239,275,631.5 | +0.71%; p=0.009 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `OpenSplit/satellites=32/files=6/cold` | allocs/op | 3,036,824.5 | 3,243,647.5 | 3,258,036.5 | +0.48%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `OpenSplit/satellites=32/files=6/warm` | B/op | 85,058,659 | 85,074,995 | 85,690,840 | +0.74%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `OpenSplit/satellites=32/files=6/warm` | allocs/op | 704,922 | 705,736.5 | 708,631 | +0.41%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `OpenSplit/satellites=512/files=6/cold` | B/op | 2,952,724,244 | 3,064,140,468 | 3,079,950,608 | +0.49%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `OpenSplit/satellites=512/files=6/cold` | allocs/op | 41,632,122 | 44,828,144 | 45,019,497.5 | +0.43%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `OpenSplit/satellites=512/files=6/warm` | B/op | 954,020,708 | 954,538,856 | 963,760,444 | +0.98%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `OpenSplit/satellites=512/files=6/warm` | allocs/op | 5,948,334.5 | 5,949,162.5 | 5,992,695.5 | +0.73%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `Satisfy/satellites=128/assertions=384` | B/op | 9,168,248 | 9,225,624.5 | 9,228,964.5 | +0.04%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `Satisfy/satellites=128/assertions=384` | allocs/op | 154,277 | 148,134 | 148,525 | +0.26%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `Satisfy/satellites=32/assertions=96` | B/op | 2,293,856 | 2,308,193 | 2,309,227.5 | +0.04%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `Satisfy/satellites=32/assertions=96` | allocs/op | 38,761 | 37,225 | 37,328 | +0.28%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `Satisfy/satellites=512/assertions=1536` | B/op | 36,681,513 | 36,903,584 | 36,915,797.5 | +0.04%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `Satisfy/satellites=512/assertions=1536` | allocs/op | 616,401 | 591,849.5 | 593,383 | +0.26%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `ValidateSplit/satellites=128/files=6/jobs=1` | B/op | 841,064,616 | 870,030,532 | 874,388,112 | +0.49%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `ValidateSplit/satellites=128/files=6/jobs=1` | allocs/op | 11,273,023 | 12,077,396.5 | 12,125,500.5 | +0.40%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `ValidateSplit/satellites=128/files=6/jobs=8` | B/op | 842,792,832 | 872,266,312 | 876,287,938 | +0.45%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `ValidateSplit/satellites=128/files=6/jobs=8` | allocs/op | 11,287,384.5 | 12,092,145 | 12,138,598.5 | +0.38%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `ValidateSplit/satellites=128/files=6/jobs=8` | sec/op | 1.09857 | 1.05093 | 1.2163 | +26.59%; p=0.015 n=6 | Timing movement failed the additional six-round confirmation; host/GC-sensitive, not an established source regression. |
| `ValidateSplit/satellites=32/files=6/jobs=1` | B/op | 248,890,276 | 256,505,864 | 257,715,120 | +0.47%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `ValidateSplit/satellites=32/files=6/jobs=1` | allocs/op | 3,156,652 | 3,363,098.5 | 3,375,418 | +0.37%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `ValidateSplit/satellites=32/files=6/jobs=8` | B/op | 251,146,348 | 259,054,910 | 260,092,207.5 | +0.50%; p=0.041 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `ValidateSplit/satellites=32/files=6/jobs=8` | allocs/op | 3,172,753 | 3,381,803.5 | 3,392,698.5 | +0.33%; p=0.026 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `ValidateSplit/satellites=512/files=6/jobs=1` | B/op | 3,274,132,600 | 3,388,953,136 | 3,404,245,668 | +0.48%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `ValidateSplit/satellites=512/files=6/jobs=1` | allocs/op | 43,733,864 | 46,930,257.5 | 47,121,255.5 | +0.41%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `ValidateSplit/satellites=512/files=6/jobs=8` | B/op | 3,273,138,720 | 3,389,575,736 | 3,404,607,992 | +0.44%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |
| `ValidateSplit/satellites=512/files=6/jobs=8` | allocs/op | 43,734,166 | 46,930,358 | 47,121,296.5 | +0.41%; p=0.002 n=6 | Larger correct semantic/index graph and associated model queries; retain required objects, with the separate lazy-memo opportunity below. |

### Attribution and the limits of a capacity “fix”

Bisection walks `git rev-list --reverse --first-parent` between the
accounted and pinned heads, including squash/direct commits, and checks
both sides of the identified transition. A threshold identifies a moved
metric; it does not by itself decide whether the underlying work is wrong.

| Probe | First-parent transition | Observation and verdict |
|---|---|---|
| `index-vehicle` | [`664cf8970`](https://github.com/Open-MBEE/OpenSysML/commit/664cf8970443cd231a56ee5ef7683e0afccdd189) | New conjugated-port definitions add required names and cross the map capacity boundary. |
| `fqn` | [`bd2b6ae1b`](https://github.com/Open-MBEE/OpenSysML/commit/bd2b6ae1b574dc0e6f07a79a01baf79e3015cecd) | Expression/subject membership adds symbols to the timed enumeration. |
| `requirement-members` | [`8d67629b6`](https://github.com/Open-MBEE/OpenSysML/commit/8d67629b695096babc62b039f358bc450e29d858) | The 8-member allocation-count threshold finds eager model memos, not the body-copy root; do not conflate those causes. |
| `edit-beside` | [`664cf8970`](https://github.com/Open-MBEE/OpenSysML/commit/664cf8970443cd231a56ee5ef7683e0afccdd189) | New conjugate scopes and members are now indexed and enumerated during edits. |
| `api-json` | [`01cf854fd`](https://github.com/Open-MBEE/OpenSysML/commit/01cf854fd12a662e170eb5bf89ded85bd2de66cf) | Required serialized properties increase writer work; the address-split profile also includes untimed graph setup. |
| `empty-load` | [`7ce9252be`](https://github.com/Open-MBEE/OpenSysML/commit/7ce9252bee424ca75ea4350471b98e796b0890b8) | Unused expression memo maps are allocated at model construction; #1826 makes them lazy. |

The body-copy helper itself was introduced by the satisfy-subject change
in #1808 (`fa971e65c`). A zero-allocation test fails on the old helper and
passes when a declaration with no prepended subject borrows its body.
The 64-member profile attributes 116,352 bytes across 101 checks to that
copy. This is distinct from the 8-member allocation-count bisection above.

The vehicle index grows from 896 FQN entries and 888 reverse-name entries
to 923 and 915; the new conjugate scopes are real semantic content. In this
Go runtime the maps cross the 896-entry load boundary of a 1,024-slot
table. The two generic `layer.set` allocation profiles grow by about
9.48 and 5.26 MiB across 101 indexes (about 149 KiB per index), explaining
most of the approximately 174 KiB increase. Conjugate construction itself
also grows (about 1.5 MiB across the same profile). The raw Symbol struct
grows from 200 to 208 bytes; that alone is not proof of an 8-byte heap
increase per allocation because allocator size classes intervene.

The larger workspace vehicle row was profiled separately, not explained
away by the smaller index row: ten timed edits plus setup add about
90.95 MiB in `elementList.add` and 57.82 MiB flat in `appendSubtree` across
the profile. These are namespace traversal/deduplication sets over the
expanded conjugate hierarchy. Synthetic analysis and satisfy profiles add
`subjectRelationships`, `caseRoleRedefinitions` and `explicitRedefinitions`:
the subjects and their types/redefinitions are now represented rather than
omitted. The synthetic profile also contains the avoidable member-body copy.

No required symbols or output properties are removed to make a row smaller.
No capacity constant is fitted to the vehicle model. An upper-bound reserve
can retain oversized maps for duplicate FQNs, aliases and merged namespaces;
an exact count adds a traversal/deduplication structure. Neither is a proved
general improvement here. The index increase is classified as required
semantic registrations amplified by runtime capacity growth, not an
established avoidable index regression. This does not claim that the index
or namespace traversal is globally optimal.

### Timing confirmation

No source timing regression is established. The first quiet run appeared
to move split validation at 128 satellites/jobs=8 by +26.59% (1.1001 →
1.3925 s, p=0.015) and 32-satellite conversion by +7.90% (14.4737 →
15.6165 s, p=0.041). Six additional alternating focused rounds reverse
both directions: 1.549 → 1.425 s (p=1.000) and 16.84 → 14.69 s
(p=0.937), respectively. CPU profiles have the same map/GC hotspots; they
do not isolate a new CPU algorithm responsible for the first timing result.
The allocation changes persist. The timing result is host/GC-sensitive,
not a speed regression that an allocation fix can honestly claim to cure.

### The fixes, paired against their own baseline

Each result is six alternating-order paired rounds with the same toolchain
and GOMAXPROCS. These medians belong to these paired invocations, not to
the three-column table above. In particular, synthetic analysis and other
cache-sensitive rows must not borrow a byte total from a differently
warmed benchmark process to calculate a residual.

- [Borrow unchanged member bodies](https://github.com/Open-MBEE/OpenSysML/pull/1824),
  head `062792fe9ed37a25dd0ce8586d643a29579713df`, against pinned develop:
  the 64-member check falls from 1,427,643 to 1,426,489 B/op and from
  15,380 to 15,379 allocations. Accounted develop is 1,426,335.5 B/op:
  this recovers most of the increase, not all. The 8/16/32-member checks
  also lose one allocation and 127.5/255.5/510 bytes, respectively
  (all p=0.002). The body is never mutated; satisfy subjects still precede
  it. Existing diagnostics and fixtures stay unchanged. No timing win is
  claimed.
- [Scan positional addresses](https://github.com/Open-MBEE/OpenSysML/pull/1827),
  head `8c1ecc2f357d658480b0686178c7ad2ccf4b973a`: quote-aware detection shares the
  identity-segment cutter without materializing the segment slice. Empty,
  malformed, escaped, quoted-`@` and positional cases retain their answers;
  the focused allocation tests require zero allocations. The 32-satellite
  conversion, freshly paired against develop `bb5809b78`, falls from
  34,507,433.5 to 34,483,544 allocations (−23,889.5, −0.07%, p=0.002).
  This remains about +0.32% above the original accounted-develop result
  (34,373,317.5); that older number is context, not a paired recovery total.
  ISQ standard/compact writer modes lose about 1,541 allocations (−0.11%,
  p=0.002). Although graph construction precedes their timer, writer
  qualified-name evaluation calls `addressOf` and encoded-address detection
  inside it: the scanner is not wholly outside timed writer work.
  Other writer allocation rows do not
  significantly improve. In particular the
  complete writer increase is not recovered; its added correct properties
  still have to be serialized. Output byte counts are identical before
  and after this fix; API-JSON regeneration changes only writer
  `sourceDigest`, not a scorecard.
  Compact SimpleVehicleModel timing initially moved from 0.5022 to 0.5461
  s/op (+8.73%, p=0.004). Six additional isolated alternating pairs with
  the same binaries instead measured 0.5615 to 0.4978 s/op (p=0.093,
  not significant): the apparent slowdown reversed and did not reproduce.
  Both results are retained; no favorable timing delta is claimed as a win.
- [Lazy expression-memo initialization](https://github.com/Open-MBEE/OpenSysML/pull/1826),
  head `4975b4b9cabc27cfef15b1db1bd61f84a33018b4`, freshly paired against
  develop `ea9429f25` after #1817/#1823 merged. Empty load falls from
  244,785 to 244,273 B/op and from 2,048 to 2,039 allocations; the 64-member
  check falls from 1,427,691.5 to 1,427,546 B/op and from 15,381 to 15,378
  allocations. The 8/16/32-member checks also lose three allocations
  (all allocation p=0.002). This pair predates #1824's independent fix.
  `valueExpressions`, `expressionParts` and the new `bodySymbols` table
  stay nil until their first write, before journaling. Repeated reads
  preserve memoized pointer identity; document replacement invalidates
  the old body entry. Required expression objects remain. No timing or
  RSS recovery is claimed, nor a return of every row to accounted develop.

### Whole binary and controlled GC

Three cold processes per workload/revision, after an unreported warm-up,
use the existing whole-binary method. Generated model sizes are verified
as 3,254, 6,198 and 12,086 elements. Each cell is median elapsed seconds /
peak RSS MiB; output/verdict digests and exits were retained. As in
[Export element IDs](#export-element-ids), 0.11.0 does not expose
`elementId` as a query property, so those three cells are not applicable
rather than a comparable successful query with missing measurements.

| Workload | 0.11.0 rebuilt | Accounted develop | Pinned develop |
|---|---:|---:|---:|
| `action-executor` | 0.03 s / 73.38 MiB | 0.03 s / 75.75 MiB | 0.05 s / 76.25 MiB |
| `apollo` | 0.44 s / 167.50 MiB | 0.38 s / 171.25 MiB | 0.42 s / 172.88 MiB |
| `disposal-robot` | 0.05 s / 79.88 MiB | 0.07 s / 81.75 MiB | 0.06 s / 81.88 MiB |
| `generated-12086` | 0.32 s / 164.00 MiB | 0.28 s / 168.38 MiB | 0.33 s / 179.88 MiB |
| `generated-3254` | 0.12 s / 105.88 MiB | 0.13 s / 105.62 MiB | 0.15 s / 106.62 MiB |
| `generated-6198` | 0.17 s / 119.25 MiB | 0.17 s / 121.12 MiB | 0.17 s / 130.25 MiB |
| `ids-12086` | not measured | 2.33 s / 392.90 MiB | 3.11 s / 455.57 MiB |
| `ids-12086-three` | not measured | 2.7 s / 471.97 MiB | 3.01 s / 492.84 MiB |
| `ids-6198` | not measured | 1.16 s / 260.12 MiB | 1.4 s / 275.23 MiB |
| `process-floor` | 0.02 s / 71.88 MiB | 0.03 s / 74.12 MiB | 0.03 s / 74.12 MiB |
| `session-floor` | 0.04 s / 72.50 MiB | 0.03 s / 74.12 MiB | 0.03 s / 75.00 MiB |
| `ttl-12086` | 20.6 s / 4048.39 MiB | 17.1 s / 4230.09 MiB | 17.1 s / 4766.32 MiB |

All moved RSS rows were re-run in six alternating accounted/develop pairs;
none of the whole-binary wall-time rows confirmed a significant slowdown.
The significant RSS movements are listed in full, not just the largest:

| Workload | Accounted RSS MiB | Pinned RSS MiB | Quiet confirmation |
|---|---:|---:|---|
| `process-floor` | 74.19 | 74.56 | +0.51%; p=0.037 n=6 |
| `session-floor` | 74.19 | 75.12 | +1.26%; p=0.026 n=6 |
| `ids-6198` | 260.44 | 289.21 | +11.05%; p=0.015 n=6 |
| `ids-12086` | 397.84 | 433.29 | +8.91%; p=0.002 n=6 |
| `ids-12086-three` | 464.94 | 545.15 | +17.25%; p=0.002 n=6 |

Default-GC RSS is not retained semantic memory alone. Re-running the
12,086-element validation with controlled GC gives the following medians
of three runs (all completed successfully):

| Setting | 0.11.0 rebuilt RSS MiB | Accounted RSS MiB | Pinned RSS MiB |
|---|---:|---:|---:|
| `GOGC25` | 146.94 | 150.06 | 151.12 |
| `GOMEMLIMIT120MiB` | 142.00 | 144.29 | 144.56 |
| `address1200000KiB` | 163.50 | 173.88 | 169.62 |

The accounted-to-pinned validation RSS difference shrinks to about 1 MiB
at GOGC=25 and about 0.3 MiB with GOMEMLIMIT=120MiB. Address-space-limited
runs also complete. That supports a substantial GC/heap-headroom effect,
not a claim that all default-GC RSS growth is a retained leak. The larger
identity-query RSS changes remain workload-specific; these GC probes do
not prove their entire cause. Neither small allocation fix is presented
as recovering those RSS differences.

The identity queries were also re-run with GOGC=25: three alternating-order
cold pairs after a warm-up. Their RSS gap does **not** disappear:

| Workload | Accounted RSS MiB | Pinned RSS MiB |
|---|---:|---:|
| `ids-12086` | 328.86 | 366.47 |
| `ids-12086-three` | 348.75 | 384.47 |
| `ids-6198` | 222.00 | 241.12 |

Allocation profiles of the 12,086-element model and three identity queries
show 14,141 versus 14,340 result rows per query. Total sampled allocation
space falls from 766.48 to 641.01 MiB, while `identityIDsOf` grows by about
56.39 MiB cumulatively and `registerExpressionIDs` by 49.72 MiB; the
26.48 MiB increase under `registerExpressionPart` is nested inside those
totals, not additive. Required expression identities consume more space
even though other work allocates less. The end-of-command profile is not
a retained-model measurement: the CLI model is then unreachable. These
profiles do not establish a retained leak, and the validation-only GC
result cannot be generalized to identity queries. The workload-specific
peak-RSS residual is retained explicitly, not claimed recovered by the fixes.
