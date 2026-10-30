# Performance: release 0.12.0 against release 0.11.0

The release-gate measurement of the 0.12.0 candidate — the head of
`develop` at `1c6bb197227bfc4b8a8f33bda51c86093d56b933` (2026-10-30),
which holds #1699 (the export element-ID cost, merged at `101d2f267`)
— measured against the `v0.11.0` tag on `main`
(`ddf4df00037d6ddbfb1fa9d743a4389f75c9e2dc`). The record follows the
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
and every table below is taken on `1c6bb1972`, four first-parent merges
after the pinned head, as the candidate.

## Three columns

0.11.0 was built with `go1.25.11`; `develop` builds with `go1.27.3`
(`go.mod` `toolchain`). The toolchain change dominates a naive timing
comparison, so three binaries are measured:

1. **0.11.0 as released** — the `v0.11.0` source built with `go1.25.11`,
   what a 0.11.0 user runs.
2. **0.11.0 rebuilt** — the same source built with `go1.27.3`.
3. **0.12.0 candidate** — `1c6bb1972` built with `go1.27.3`.

A source regression or improvement is column 2 against column 3: the
same toolchain, different source. What a user upgrading from 0.11.0
feels is column 1 against column 3. Column 1 against column 2 is the
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
six-round A/B against the merge that introduced the cost, listed here
as they open; the branches are named in the summary table.

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

- The merges after `1c6bb1972` ([The cut point](#the-cut-point)).
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
#1700, #1701, #1705). `develop` moved on while the suite ran; the
first-parent merges between `1c6bb1972` and `54e590985` are **not
measured** by any table here:

| Merge | Subject | Rows it could move |
|---|---|---|
| #1704, #1698, #1703, #1702 | Sonar-driven refactors | none expected; `go test` surfaces |
| #1706, #1707 | `internal/ir/view` | `internal/ir/view` and `ir/view/route` rows |
| #1732 | connector-end element IDs in the export | `WriteAPIJSON`, `ConvertAPIJSON`, the export probe |
| #1709 | reflective element attributes in semantics | `Lookup*`, `Analyze*`, `FeaturesOf`, the load rows |

When the maintainer names the release head, the rows in the right-hand
column are re-probed interleaved against `1c6bb1972` and this section
is amended, as the 0.11.0 record's was.
