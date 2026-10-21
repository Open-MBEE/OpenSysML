# Performance: release 0.11.0 against release 0.10.1

The release-gate measurement of the 0.11.0 candidate — the head of
`develop` at `d8b60ed62c0740722e041c59f752cc5c0c4cdddf` (2026-10-20), from
which `release/0.11.0` is to be cut, with the head of #1350
(`1ceae965cd07f8b9afb849da3b217d9018d9881f`, the memoization of a
symbol's implied generals) merged on top as `df3341e65` — measured against
the `v0.10.1` tag on `main` (`1c45b6e1749fc8adc9b82d4af22fbe180b56a0e2`,
2026-10-15). It follows the method of the
[0.10.1 against 0.10.0](performance-release-0.10.1-vs-0.10.0.md) record
and of the [0.10.0 against 0.9.2](archive/performance-release-0.10.0-vs-0.9.2.md)
record before it: every benchmark on both revisions, `benchstat` over six
runs, the rows the machine leaves in doubt re-run interleaved on a quiet
machine, and whole-binary timings on generated and real models. See
[performance](../internals/performance.md) for what the figures mean and
where the remaining cost is.

0.11.0 is a minor release: 227 merges to `develop` since
the 0.10.1 tag, among them the hiding of colliding imports (#1062),
objects typed by every type they inherit through subsetting (#1276), the
subsetting of a usage's base features (#1319), transitive order
dependence in the executor (#1136), constancy conformance (#1298), the
redefinition closure in the executor's root frame (#1057) and the API
JSON element form (#1253). **The expectation going in was not parity**,
and the measurement does not find it: parsing, lexing, indexing,
formatting, the migrator, the library index, the lowering of graphs and
the whole binary are at parity to the allocation or within the machine's
noise, while name resolution over wildcard imports, analysis, the
constraint and instantiation paths, the executor and the language
server's cross-file queries moved, some of them far. This record says by
how much, why, and which part of it is fixed.

The measurement was taken on the candidate as it stands and, where a row
moved there, on the candidate with every fix below merged locally
(`5bcaa4265`), so the tables show three columns: 0.10.1, the 0.11.0
candidate, and the candidate with the fixes. Each fix is its own pull
request against `develop`, with a test that fails before it and a
benchmark A/B against the merge that introduced the cost. As this record
is finished, #1347, #1348, #1351, #1352 and #1353 have merged to
`develop` (`a662c2bce`) and #1349 is open against it, so the third
column is the head 0.11.0 is cut from, less #1349. The fixes, by the
change in the history:

- the execution runtime computes an expression's footprint once per
  model instead of on every loop pass, guard and send (`internal/exec/runtime`,
  #1347; the cost came with #1136);
- name resolution enumerates a namespace's imported members without
  building each member's qualified name, candidate slice and route
  conditions (`internal/semantic/resolve`, #1348; the cost came with
  #1062);
- the constancy check conforms a usage to a named library type by the
  element the name resolves to instead of the qualified-name string of
  every supertype (`internal/semantic/semantics`, #1349, stacked on
  #1348; the cost came with #1298);
- the execution runtime computes a feature's effective types once per
  model instead of on every object made as it (`internal/exec/runtime`,
  #1351; the cost came with #1276);
- the REPL counts a diagnostic's UTF-16 column without encoding the line
  (`internal/frontend/repl`, #1352; the cost came with #1205);
- the semantic model computes a feature's redefinition closure once per
  symbol and reads an implicit parameter's position from the memoized
  parameter list (`internal/semantic/semantics`, #1353; the cost came
  with #1057's use of the closure in the root frame);
- and, already in the candidate, the semantic model computes a symbol's
  implied generals once per symbol (`internal/semantic/semantics`,
  #1350; the cost came with #1319 and was what made the bare `develop`
  head unmeasurable — see the summary).

## Summary

**Nothing here blocks the release on time, and the fixes return the
executor to 0.10.1 to the allocation; what remains after them is the
inherent cost of four correctness changes and one design cost that is
named, not fixed.** Confirmed by the interleaved quiet-machine re-run
(six rounds, 0.10.1 | candidate | candidate + fixes, allocations exact):

- **The executor's loop rows** (`internal/exec/runtime`): two allocations
  per iteration on every arithmetic loop (`IntegerArithmeticLoop`
  9 072 → 11 073 allocs/op, 280 → 483 KiB; `RationalDecimalLoop`,
  `RealDecimalLoop` and `IntegerArithmeticLoopBeyondInt64` the same;
  `IntegerArithmeticNestedLoop` 26.08k → 30.08k; `RationalHarmonicLoop`
  4 652 → 5 052; `AssignmentLoopStep` 7 328 → 7 529) and +9–18% time.
  #1136 (transitive order dependence) made the statement engine rebuild
  the loop condition's footprint on every pass. #1347 restores every row
  to 0.10.1's count (9 072, 26.08k, 4 652, 7 328) and the time to within
  +5–6% on two rows and the noise on the rest.
- **The root frame** (`NewRootFrame` 36 → 55 allocs/op, 11.2 → 18.4 µs):
  since #1057 the root frame asks for every held feature's transitive
  redefinition closure, which was rebuilt per call. #1353 takes it back
  to 36 allocations and 10.8 µs.
- **The language server's cross-file queries** (`internal/frontend/lsp`):
  `ReferencesCold` 44.6k → 6.28M allocs/op, 9.8 MiB → 1 010 MiB,
  44 ms → 1 207 ms; `RenameWarm` 13.7k → 12.41M, 1.35 MiB → 1 668 MiB,
  20 ms → 2 004 ms. #1062 (hidden colliding imports) made the
  enumeration of a wildcard import's members build, per member, its
  qualified name, a one-candidate slice and its route conditions. #1348
  removes that (1.94M and 3.74M allocations, −69% and −70%) but the rows
  still allocate ~1 GiB and take ~1–1.7 s: each query re-enumerates
  every wildcard import's members per document (references) or per
  occurrence (rename), because the per-edge enumeration memo is cleared
  when the outermost enumeration returns and #1062 made each enumeration
  expensive. Keeping what an import brings for the life of the resolver
  (a memo per namespace and member selection, journaled like the
  collisions memo) is the follow-up fix, branch
  `perf/memoize-namespace-children`, which was not measured for this
  record; **until it lands this is the one confirmed regression 0.11.0
  ships**. `ReferencesWarm`,
  `WorkspaceUpdate`, `FormatEdits` and `SpanToRangeLarge` are unchanged
  to the allocation.
- **Analysis and the import graph** (`ImportValues/*` +59–72%
  allocations, `Analyze/synthetic` 4.38M → 5.85M, `WorkspaceEdit/synthetic
  reindex+diagnostics` 4.83M → 6.29M, the REPL's `LoadModel` +27–29%,
  the workspace model's `AnalyseResolved` 5.6k → 10.6k): three causes
  stacked. #1298 (constancy conformance) built the qualified name of a
  usage's every supertype four times per usage (+29% on `ImportValues`,
  +10% on `Analyze`); #1349 restores those exactly (`ImportValues/flat/100`
  135.5k → 175.2k → 135.5k against the pre-#1298 merge). #1276 and #1319
  type each feature by every type it inherits through subsetting and
  subset a usage's base features, which is more semantic work per usage
  by design; with #1350 in the candidate and #1351 applied the rows end
  +15–21% over 0.10.1 (`ImportValues` +15–21%, `Analyze/synthetic` +15%,
  `AnalyseResolved` +89%, the REPL's `LoadModel` +11–12%), and that
  remainder is carried as the inherent cost of the two changes.
- **Instantiation and constraint checking** (`Instantiate`
  1.94M → 4.73M allocs/op, `FeaturesOf` 4.13M → 6.73M, `BatchSatisfy`
  4.30M → 6.81M, `BatchConstraints` 68.1k → 84.6k,
  `SameConstraintManyInstances` 35 → 47, the stress constellation's
  `Satisfy` +63% and `FleetSatisfy` +72–78%): #1276 made the runtime
  re-resolve a feature's declared types on every object made as it.
  #1351 memoizes them per feature and recovers about 60% of the growth
  (`Instantiate` 4.03M, `FeaturesOf` 5.16M, `BatchSatisfy` 5.66M,
  `BatchConstraints` 72.5k, `SameConstraintManyInstances` 38; against
  the pre-#1276 merge, `Satisfy/32` 33.8k → 54.5k → 41.1k); the rest is
  the type-set work #1276 and #1319 add, carried as inherent.
- **The REPL's diagnostics** (`Diagnostics` 561 → 691 allocs/op at 50
  attributes, +38–46% bytes): #1205 (CI formats) encoded the line prefix
  to UTF-16 per diagnostic to read its length and copied the snippet's
  source twice; #1352 removes that (645; 1 → 0 allocations per column).
  The other +50 came with #1161 (project interchange dependencies) and
  is in the supertype walk the diagnostic's formatting makes; it is
  carried.
- **The API JSON conversion** (`ConvertAPIJSON` 25.26M → 29.20M
  allocs/op and 4.69 → 5.49 GiB at 32 satellites, +16% / +22% at 128):
  #1253 states `isComposite` on every feature in the element form. The
  output is larger; inherent.
- **The vehicle rows** (`WorkspaceEdit/vehicle reindex+diagnostics`
  858k → 1 206k allocs/op, `GRPCParseFileUncached` 901k → 1 331k, and
  `Analyze/vehicle`, which fails on the candidate): since #1062 the
  Annex A `SimpleVehicleModel` reports 20 unresolved references
  (`vehicle_b`, `providePower`, `cost`, `ignitionCmdPort`, …) that
  0.10.1 resolved by binding the first match of a colliding import; the
  merge moved the pilot ratchet for that file 134 → 154 and the
  benchmark asserts an error-free analysis, so the row cannot be
  measured on 0.11.0. The two rows that still run pay the suggestion
  path of those diagnostics and the enumeration cost above; #1348 takes
  them to 924k and 1 014k (pre-#1062 877k and 941k).

- **The executor's statements** (`ActionLoop/for1000` 4 231 → 6 266
  allocs/op, `ActionChain/chain1000` 65.0k → 80.8k and 17.0 → 31.0 ms,
  `StateLoop/count5000` 1.176M → 1.381M, `ExecuteState` 12.13k → 14.20k,
  `ExecuteAction` 245 → 299, `ExecuteActionFreshContext` 340 → 403, and
  the bytes of `MaterializePlainAttributes` 43.6 → 51.2 KiB and
  `SetFeatureValueNoDependents` 352 → 416 B): all bisect to #1136, which
  beside the loop footprints #1347 fixed has three more per-execution
  costs on the statement path. An assignment's footprint is stamped with
  the performance's own places on every execution, cloning the
  statement's read and write places each time (two allocations per
  executed statement; `placesOf`, 2 007 of the 2 035 new objects per
  `ActionLoop/for1000` operation). A body's statement order was memoized
  per executor, not per model, so a chain of a thousand actions rebuilt
  every body's order on every run (all of `ActionChain/chain1000`'s
  growth) and a state's entry behavior rebuilt its order on every entry
  (half of `StateLoop`'s and `ExecuteState`'s). And taking a transition
  made two closures (`taking`/`underControl`; the other half of the
  state rows). The first two are fixed by
  `perf/stamp-statement-footprint-once` (#1355), the third by
  `perf/take-transition-without-closures` (#1356); their failing-before
  tests put one more loop pass at 6.00 → 4.00 allocations, one more
  chained action at 84.58 → 70.54, one more state event at 133.50 →
  126.00 (#1355) → 120.00 (both), and taking a transition at 2 → 0.
  #1355's six-round A/B (pre-#1136 d5e2317de | develop | #1355,
  allocs/op) puts `ActionLoop/for1000` at 4 238 | 6 256 | 4 241,
  `ActionChain/chain1000` 65.97k | 80.77k | 66.42k, `ExecuteAction`
  252 | 289 | 254, `StateLoop/count5000` 1.191M | 1.336M | 1.261M and
  `ExecuteState` 12.29k | 13.75k | 13.00k; #1356 takes the state rows'
  remainder, and the last object per event is #1276's declared-types
  lookup. `ExecuteActionFreshContext` (347 | 394 | 376) keeps what a
  fresh context's first run computes for its model. The bytes of
  `MaterializePlainAttributes` and `SetFeatureValueNoDependents` are
  #1136's but none of these three costs (both fixes leave them), and are
  carried. Both fixes were written after the tables above and are not in
  the third column.
- **Three small rows with their cause named and carried**:
  `GRPCVerifyConstraint` 9 786 → 10 770 and the REPL's `Instantiate`
  56 → 59 are #1276's (the wider type set; #1351 takes the first to
  10 690); `ConnectParseFileHTTPCached` 1 017 → 1 151 is #1062's
  collision bookkeeping on the parse path (the same +0.6% the gRPC parse
  rows show); the gRPC `InstantiateWarmModel` 1 833 → 2 238 (2 006 with
  the fixes) is #1325's API JSON writer provenance on the instantiate
  reply; the REPL's `RunStateMachine` 224 → 228 is #1315's.

What the quiet re-run cleared: `Parse/synthetic` and `Parse/vehicle`
(+16–19% time in the loaded rounds with identical allocations; eight
isolated rounds: 99.7 → 103.8 ms, p = 0.13, and 2.65 → 2.80 ms,
p = 0.065 — the machine), `WorkspaceUpdate`, `FormatEdits`,
`IndexAddExpand`, `FQNOf`, `LookupQualified`, `REPLEvalExpr`,
`LowerChain`, `GRPCEvaluate`, `ConnectEvaluateHTTP`, `FleetInstantiate`'s
time and the stress constellation's `HydratePlane` and `EditBeside` time.

What improved: `BehaviorOrdersLibraryModel` 1 927 → 1 721 allocs/op
(−11%, −15% bytes, −20% time with the fixes), the REPL's `RunCalc`
79 → 74, `IndexAddExpand/vehicle` and `WorkspaceEdit/vehicle` −1%
allocations with the fixes.

Known from the earlier records and unchanged: `AnalyzeSplitPerDocument`
and `PlaneResidency` in `tests/stressmodel` fail on both revisions and
are not measured.

## Method

Three detached worktrees, one per column, each built with `make build`
(`go1.25.11 linux/amd64`), with the pilot and training corpora linked in
so the corpus-backed rows (`*/vehicle`, `ConnectEvaluateHTTP`,
`GRPCParseFile*`, `InstantiateWarmModel`) run on every side. Every
benchmark package is run with `-run '^$' -bench . -benchmem -count 1`,
0.10.1 then the candidate, six rounds in a row, so the two revisions see
the same machine state round by round; each round's output is an
append-only file and `benchstat` compares the six files per revision.
`tests/stressmodel` runs the same way with `ConvertAPIJSON/satellites=512`
skipped, as the previous records did. The first-round findings were
attributed while the later rounds ran — a first-parent bisect over the
227 merges by allocation count, then a profile of the first bad merge —
so the time column of those rounds is loaded and is not what the tables
show; the times come from the quiet re-run below.

The machine is an 8-vCPU `Intel(R) Xeon(R) Processor` (GenuineIntel, one
thread per core, 320 MiB L3, 31 GiB), `GOMAXPROCS=8`. It is not the host
of the 0.10.1 record, so sec/op is comparable only between the columns
of this record, never with the earlier records; allocations and bytes
are exact and comparable across hosts.

The candidate moved once while the first six rounds ran: `develop`
advanced from `b60a18c20` to `d8b60ed62` with #1319, whose first two
rounds showed `tests/stressmodel`'s `Load`/`LoadFiles`/`ValidateSplit`/
`OpenSplit`/`EditImported` at +200–245% allocations (`Load/512` 44.3M →
143.0M, 14.1 → 28.6 s), `Satisfy`/`FleetSatisfy` at +350–395% and
`tests/perf`'s `Instantiate` at 3.75M → 36.5M (1.36 → 7.2 s): the
symbol's implied generals were re-derived on every feature-type query.
#1350 memoizes them, was already open, and is what the release needs
before it can be measured, so the run was restarted on `d8b60ed62` with
#1350's head merged locally and that is the candidate of every table.
With it the rows return to the `b60a18c20` level or better on time
(`Instantiate` 1.43 s, `ImportValues/nested/2000` −27%, `AnalyseResolved`
−42%, `NewRootFrame` back to 55 allocations) while a residual allocation
cost of #1319 remains and is accounted for above (`Instantiate` 4.73M
against 3.75M, `AnalyseResolved` 10.6k against 5.6k). No `release/0.11.0`
branch exists on `origin` as this is written and `develop` has not moved
since.

### What is and is not comparable

- **Allocations and bytes per operation** are deterministic on every row
  but `RenameWarm` (±9% on 0.10.1) and the gRPC `InstantiateWarmModel`
  (±1–3%) and agree to the unit between the main run and the quiet
  re-run; a movement there is the code's.
- **Time** is the quiet re-run's: six rounds of the three binaries
  interleaved with nothing else on the machine, over
  `internal/exec/runtime`, `internal/frontend/{repl,lsp,grpc}`,
  `internal/workspace/model`, `tests/perf` and the 32-satellite rows of
  `tests/stressmodel`; `Parse/*` alone for eight rounds. The 128- and
  512-satellite stress rows carry the main run's time with that caveat.
- **`tests/codegen`** has benchmarks only on the candidate
  (`CompiledCalc/*`, added with #1226); it is reported but has no 0.10.1
  column.
- **`Analyze/vehicle`** has no candidate column: the benchmark fails
  (see the summary).

## Package benchmarks

Allocations and bytes are from the six main rounds (exact); times are the
quiet re-run's medians. Columns: 0.10.1 | 0.11.0 candidate | candidate +
fixes.

### `internal/exec/runtime`

| benchmark | time (µs) | allocs/op | B/op |
| --------- | --------- | --------- | ---- |
| `IntegerArithmeticLoop` | 3 745 \| 4 229 (+13%) \| 3 957 (+6%) | 9 072 \| 11 073 \| 9 073 | 280 \| 483 \| 296 KiB |
| `IntegerArithmeticLoopBeyondInt64` | 3 984 \| 4 704 (+18%) \| 4 305 (~) | 13.07k \| 15.07k \| 13.07k | 514 \| 718 \| 530 KiB |
| `IntegerArithmeticNestedLoop` | 5 300 \| 5 999 (+13%) \| 5 684 (~) | 26.08k \| 30.08k \| 26.08k | 915 \| 1 335 \| 1 023 KiB |
| `RationalDecimalLoop` | 4 126 \| 4 498 (+9%) \| 4 331 (+5%) | 9 070 \| 11 072 \| 9 072 | 280 \| 483 \| 296 KiB |
| `RationalHarmonicLoop` | 1 019 \| 1 047 (~) \| 1 019 (~) | 4 652 \| 5 052 \| 4 652 | 179 \| 220 \| 182 KiB |
| `RealDecimalLoop` | 4 080 \| 4 617 (+13%) \| 4 164 (~) | 9 070 \| 11 072 \| 9 071 | 280 \| 483 \| 296 KiB |
| `AssignmentLoopStep` | 1 667 \| 1 722 (~) \| 1 694 (~) | 7 328 \| 7 529 \| 7 328 | 827 \| 849 \| 830 KiB |
| `IntegerCollectionSum` | 910 \| 949 (~) \| 932 (~) | 44 \| 44 \| 44 | 4.96 MiB on all |
| `NewRootFrame` | 11.17 \| 18.38 (+65%) \| 10.82 (~) | 36 \| 55 \| 36 | 12.48 \| 13.79 \| 12.54 KiB |
| `DerivedReadWriteRead` | 6.63 \| 7.22 (~) \| 6.69 (~) | 65 \| 73 \| 67 | 2.48 \| 2.67 \| 2.63 KiB |
| `MaterializePlainAttributes` | 58.0 \| 61.6 (~) \| 60.9 (~) | 618 \| 621 \| 621 | 43.6 \| 51.2 \| 51.2 KiB |
| `SetFeatureValueNoDependents` | 0.446 \| 0.465 (~) \| 0.461 (~) | 9 \| 9 \| 9 | 352 \| 416 \| 416 B |
| `BehaviorOrdersLibraryModel` | 359 \| 300 (~) \| 288 (−20%) | 1 927 \| 1 721 \| 1 723 | 164 \| 140 \| 140 KiB |

Package geomean: time +9.9% on the candidate, +1.2% with the fixes;
allocations +11.6% and −0.6%; bytes +25% and +4.4%.

The loop rows and the root frame are the subject of #1347 and #1353 and
return to 0.10.1's count. `DerivedReadWriteRead` 65 → 73 bisects to
#1276; #1351 recovers four of the eight and the other two are the wider
type set each read consults (inherent). `MaterializePlainAttributes`
(+3 allocations, +7.6 KiB) and `SetFeatureValueNoDependents` (+64 B at
nine allocations) bisect to #1136 as well but are none of its three
per-execution costs — #1355 and #1356 leave them — and are carried.

### `internal/frontend/repl`

| benchmark | time | allocs/op | B/op |
| --------- | ---- | --------- | ---- |
| `LoadModel/elements=250` | 1.96 \| 1.98 \| 1.98 ms | 125.8k \| 159.9k \| 139.5k | 14.09 \| 15.02 \| 14.44 MiB |
| `LoadModel/elements=1000` | 7.52 \| 7.57 \| 7.61 ms | 482.5k \| 620.5k \| 538.7k | 55.8 \| 59.5 \| 57.3 MiB |
| `LoadModel/elements=4000` | 29.9 \| 30.2 \| 30.2 ms | 1.909M \| 2.462M \| 2.135M | 223 \| 238 \| 229 MiB |
| `Diagnostics/attributes=50` | 93.1 \| 105.3 (+13%) \| 104.4 (~) µs | 561 \| 691 \| 645 | 73.7 \| 102.0 \| 99.8 KiB |
| `Diagnostics/attributes=200` | 317 \| 368 (+16%) \| 352 (~) µs | 1 469 \| 1 903 \| 1 707 | 249 \| 363 \| 354 KiB |
| `Diagnostics/attributes=800` | 1.22 \| 1.41 (+16%) \| 1.48 (+22%) ms | 5 623 \| 7 261 \| 6 465 | 896 \| 1 296 \| 1 258 KiB |
| `Instantiate/elements=*` | 8.1–8.7 \| 8.6–9.4 \| 8.6–9.4 µs | 56 \| 59 \| 59 | 7.3 \| 8.4 \| 8.4 KiB |
| `RunStateMachine/elements=*` | 14.4–15.1 \| 14.7–15.7 \| 14.3–14.7 µs | 224 \| 228 \| 228 | 16.4 \| 16.7 \| 16.7 KiB |
| `RunCalc/elements=*` | 5.0–5.3 \| 5.0–5.3 \| 5.0–5.1 µs | 79 \| 74 \| 74 | 4.56 \| 4.54 \| 4.54 KiB |
| `LoadModel/elements=0` | 38.5 \| 38.6 \| 38.6 µs | 1 938 \| 1 974 \| 1 980 | 228 \| 230 \| 231 KiB |

`LoadModel`'s +27–29% allocations are the analysis cost above (#1298,
fixed by #1349, and #1276/#1319's remainder); its time moves under 1%
because the load is dominated by the library index. `Diagnostics` is
#1205 (fixed by #1352) and #1161; the bytes per diagnostic go
755 → 1 044 → 1 022 at 50 attributes. `Instantiate` 56 → 59 is #1276's and
`RunStateMachine` 224 → 228 is #1315's; both are carried.

### `internal/frontend/grpc`

| benchmark | time | allocs/op | B/op |
| --------- | ---- | --------- | ---- |
| `InstantiateWarmModel` | 193 \| 266 (+38%) \| 220 (+14%) µs | 1 833 \| 2 238 \| 2 006 | 104 \| 133 \| 122 KiB |
| `ParseFileColdShared` | 7.81 \| 7.88 \| 8.11 ms | 31.39k \| 31.58k \| 31.57k | 4.72 \| 4.73 \| 4.73 MiB |
| `ParseFileColdInline` | 8.14 \| 8.16 \| 8.12 ms | 31.81k \| 32.01k \| 32.00k | 4.97 \| 4.98 \| 4.98 MiB |

`InstantiateWarmModel` is the instantiation path: #1351 recovers 232 of
its 405 allocations and the rest is #1325's writer provenance on the
reply, carried. The two parse rows are +0.6% allocations (the resolver's
collision bookkeeping, #1062) and at parity on time.

### `internal/frontend/lsp`

| benchmark | time | allocs/op | B/op |
| --------- | ---- | --------- | ---- |
| `ReferencesCold` | 44.1 \| 1 207 (+2 636%) \| 1 028 (+2 230%) ms | 44.64k \| 6 279k \| 1 944k | 9.79 \| 1 010 \| 950 MiB |
| `RenameWarm` | 20.5 \| 2 004 (+9 684%) \| 1 685 (+8 124%) ms | 13.73k \| 12 410k \| 3 740k | 1.35 \| 1 668 \| 1 549 MiB |
| `ReferencesWarm` | 200 \| 205 \| 203 µs | 1 017 on all | 339 KiB on all |
| `WorkspaceUpdate` | 244 \| 281 (~) \| 254 (~) ms | 710.3k \| 710.3k \| 710.0k | 57.2 MiB on all |
| `FormatEdits` | 940 \| 924 \| 919 µs | 85 on all | 704 KiB on all |
| `SpanToRangeLarge` | 2.98 \| 2.91 \| 2.86 ms | 0 | 0 |

The two cold rows are #1062's enumeration (fixed in part by #1348) and
the per-query re-enumeration the summary names as carried: with the fix
the allocations fall 3.2× but the bytes only 6–7%, because what remains
is the enumerated member sets themselves, rebuilt per document or per
occurrence. The warm rows and the workspace update are at parity to the
allocation.

### `internal/translate/migrate`, `internal/workspace/libs`, `internal/workspace/model`, `tests/codegen`

| benchmark | time | allocs/op | B/op |
| --------- | ---- | --------- | ---- |
| migrate `WriterSiblingBlocks` | 1.93 \| 1.95 ms | 20.02k \| 20.02k | 1.906 MiB on both |
| libs `ExpandWildcardImports` | 32.6 \| 30.9 ms | 145.7k \| 145.7k | 15.36 MiB on both |
| libs `IndexLibrary` | 16.9 \| 15.4 ms | 108.2k \| 108.3k | 14.07 \| 14.09 MiB |
| libs `ExpandModelImports` | 7.00 \| 6.70 ms | 38.15k \| 38.15k | 4.376 MiB on both |
| libs `DecodeSnapshot` | 16.4 \| 14.9 ms | 67.45k \| 67.64k | 37.85 \| 38.47 MiB |
| libs `SetDigest` | 1.32 \| 1.36 ms | 1 004 \| 1 029 | 1.737 \| 1.775 MiB |
| model `AnalyseResolved` | 1.37 \| 2.32 (+69%) \| 2.29 (+67%) ms | 5 597 \| 10 552 \| 10 554 | 585 \| 713 \| 713 KiB |
| model `AnalyseUnresolved` | 20.4 \| 21.4 \| 21.8 ms | 150.4k \| 151.1k \| 151.1k | 13.79 \| 13.84 \| 13.84 MiB |

The migrator and the library index are at parity (the library's
+0.1–2.5% allocations are the bundled index's new entries: the snapshot
is 1.6% larger). `AnalyseResolved` is #1319's residual with #1350
(5.6k → 17.1k on the bare head, 10.6k with it) and none of the fixes
touches it: it is the base-feature subsetting's work on a resolved
model, carried as inherent. `tests/codegen`'s `CompiledCalc/*` rows
(`Fib(25)`, `Collatz(27)`, `Hypot` interpreted, Go and C) exist only on
the candidate: 375, 2 344 and 86 allocs/op interpreted, 0 compiled.

### `tests/perf`

| benchmark | time | allocs/op | B/op |
| --------- | ---- | --------- | ---- |
| `ImportValues/flat/parts=100/attrs=3` | 25.3 \| 25.3 \| 25.3 ms | 131.2k \| 208.9k \| 157.2k | 11.0 \| 13.8 \| 12.8 MiB |
| `ImportValues/flat/parts=500/attrs=3` | 132 \| 132 \| 132 ms | 616.4k \| 1 001k \| 742.4k | 61.8 \| 73.3 \| 68.2 MiB |
| `ImportValues/flat/parts=2000/attrs=3` | 0.53 \| 0.53 \| 0.53 s | 2.439M \| 3.972M \| 2.940M | 260 \| 307 \| 286 MiB |
| `ImportValues/flat/parts=2000/attrs=5` | 0.88 \| 0.88 \| 0.88 s | 3.682M \| 6.144M \| 4.424M | 366 \| 438 \| 404 MiB |
| `ImportValues/nested/parts=100/attrs=3` | 27.5 \| 27.5 \| 27.5 ms | 175.5k \| 294.9k \| 202.7k | 15.5 \| 19.3 \| 17.5 MiB |
| `ImportValues/nested/parts=500/attrs=3` | 142 \| 142 \| 142 ms | 830.7k \| 1 424k \| 963.6k | 76.9 \| 92.6 \| 83.5 MiB |
| `ImportValues/nested/parts=2000/attrs=3` | 0.58 \| 0.58 \| 0.58 s | 3.290M \| 5.659M \| 3.819M | 319 \| 382 \| 346 MiB |
| `ImportValues/nested/parts=2000/attrs=5` | 0.96 \| 0.96 \| 0.96 s | 4.578M \| 7.796M \| 5.252M | 439 \| 525 \| 475 MiB |
| `Lex/synthetic` | 7.70 \| 7.85 \| 7.80 ms | 0 | 0 |
| `Lex/vehicle` | 283 \| 283 \| 271 µs | 0 | 16 B |
| `Parse/synthetic` | 101 \| 101 \| 95.6 ms | 293.2k on all | 39.6 MiB on all |
| `Parse/vehicle` | 2.44 \| 2.79 (~, 8 rounds) \| 2.59 ms | 7 560 on all | 1.063 MiB on all |
| `IndexAddExpand/synthetic` | 48.8 \| 53.5 \| 50.8 ms | 316.7k \| 318.3k \| 317.3k | 44.7 \| 44.9 \| 44.8 MiB |
| `IndexAddExpand/vehicle` | 27.5 \| 28.3 \| 26.7 ms | 139.7k \| 139.7k \| 137.7k | 16.0 \| 16.0 \| 15.9 MiB |
| `IndexAddOnly/synthetic` | 45.4 \| 47.9 \| 45.5 ms | 306.6k \| 307.7k \| 307.3k | 42.4 \| 42.6 \| 42.5 MiB |
| `IndexAddOnly/vehicle` | 1.13 \| 1.14 \| 1.10 ms | 9 633 on all | 1.19 MiB on all |
| `Analyze/synthetic` | 1.38 \| 1.68 (+22%) \| 1.58 (+15%) s | 4.382M \| 5.847M \| 5.055M | 509 \| 550 \| 530 MiB |
| `Analyze/vehicle` | 207 ms \| fails \| fails | 856.6k \| — \| — | 136 MiB \| — \| — |
| `WorkspaceEdit/synthetic` | 284 \| 280 \| 283 ms | 680.5k on all | 90.0 MiB on all |
| `WorkspaceEdit/synthetic/reindex+diagnostics` | 1.80 \| 2.13 (+18%) \| 2.09 (+16%) s | 4.828M \| 6.293M \| 5.501M | 541 \| 577 \| 556 MiB |
| `WorkspaceEdit/vehicle` | 59.6 \| 62.4 \| 62.1 ms | 168.1k \| 168.1k \| 166.1k | 15.3 \| 15.3 \| 15.2 MiB |
| `WorkspaceEdit/vehicle/reindex+diagnostics` | 235 \| 285 (+21%) \| 278 (+18%) ms | 857.7k \| 1 206k \| 923.7k | 127 \| 182 \| 160 MiB |
| `WorkspaceEditSmallDocBesideLarge` | 53.6 \| 52.4 \| 55.6 ms | 114.6k \| 122.7k \| 114.6k | 24.8 \| 25.4 \| 25.3 MiB |
| `FQNOf` | 1.71 \| 1.66 \| 1.68 ms | 40.0k on all | 735 KiB on all |
| `LookupQualified` | 1.16 \| 1.36 \| 1.18 ms | 0 | 0 |
| `FeaturesOf` | 1.11 \| 1.49 \| 1.30 s | 4.130M \| 6.730M \| 5.160M | 357 \| 433 \| 390 MiB |
| `REPLLoadFile` | 1.70 \| 2.07 \| 1.84 s | 5.333M \| 6.799M \| 6.007M | 644 \| 684 \| 665 MiB |
| `REPLSubmitSnippet` | 1.35 \| 1.49 (+11%) \| 1.49 (+11%) s | 4.695M \| 6.829M \| 6.023M | 570 \| 625 \| 606 MiB |
| `REPLEvalExpr` | 5.55 \| 5.68 \| 5.48 ms | 106 on all | 7.14 MiB on all |
| `LowerActionGraph` | 3.49 \| 3.31 \| 3.24 µs | 38 on all | 3.93 \| 4.01 \| 4.01 KiB |
| `LowerStateGraph` | 9.20 \| 9.13 \| 9.29 µs | 149 \| 152 \| 152 | 12.4 \| 12.8 \| 12.8 KiB |
| `LowerChain/chain1000` | 10.3 \| 10.4 \| 9.95 ms | 7 223 \| 7 225 \| 7 224 | 1.90 MiB on all |
| `ExecuteAction` | 86.4 \| 101 (+17%) \| 96.0 (+11%) µs | 245 \| 299 \| 289 | 200 \| 184 \| 191 KiB |
| `ExecuteActionFreshContext` | 47.8 \| 59.9 (+25%) \| 58.3 (+22%) µs | 340 \| 403 \| 393 | 40.8 \| 47.1 \| 48.6 KiB |
| `ExecuteState` | 1.12 \| 1.28 (+14%) \| 1.19 (+6%) ms | 12.13k \| 14.20k \| 13.74k | 865 \| 950 \| 974 KiB |
| `BatchConstraints` | 12.6 \| 14.6 (+16%) \| 13.3 (~) ms | 68.08k \| 84.64k \| 72.51k | 3.37 \| 3.71 \| 3.58 MiB |
| `SameConstraintManyInstances` | 5.42 \| 6.20 (+15%) \| 5.39 (~) µs | 35 \| 47 \| 38 | 1 440 \| 1 600 \| 1 528 B |
| `BatchSatisfy` | 1.46 \| 1.94 (+32%) \| 1.68 (+15%) s | 4.301M \| 6.811M \| 5.658M | 454 \| 535 \| 504 MiB |
| `Instantiate` | 0.66 \| 1.28 (+93%) \| 1.15 (+73%) s | 1.937M \| 4.730M \| 4.030M | 194 \| 349 \| 332 MiB |
| `GRPCParseFileCached` | 172 \| 181 \| 186 µs | 24 on all | 223 KiB on all |
| `GRPCParseFileUncached` | 224 \| 274 (+23%) \| 265 (+18%) ms | 900.9k \| 1 331k \| 1 014k | 149 \| 213 \| 189 MiB |
| `GRPCEvaluate` | 10.7 \| 10.6 \| 11.2 µs | 117 on all | 9.85 \| 9.88 \| 9.91 KiB |
| `GRPCVerifyConstraint` | 0.99 \| 1.23 (+24%) \| 1.20 (+22%) ms | 9 786 \| 10 770 \| 10 690 | 556 \| 629 \| 628 KiB |
| `ConnectEvaluateHTTP` | 176 \| 173 \| 177 µs | 207 on all | 22.9 \| 23.0 \| 22.8 KiB |
| `ConnectParseFileHTTPCached` | 1.04 \| 1.12 (+7%) \| 1.15 (+10%) ms | 1 017 \| 1 151 \| 1 151 | 476 \| 507 \| 504 KiB |
| `ActionLoop/for10` | 39.5 \| 51.2 (+30%) \| 45.3 (+15%) µs | 264 \| 318 \| 308 | 33.2 \| 39.1 \| 39.0 KiB |
| `ActionLoop/for100` | 173 \| 199 (+15%) \| 193 (+11%) µs | 627 \| 861 \| 851 | 72.5 \| 97.1 \| 97.0 KiB |
| `ActionLoop/for1000` | 1.43 \| 1.64 (+15%) \| 1.75 (+22%) ms | 4 231 \| 6 266 \| 6 256 | 425 \| 637 \| 637 KiB |
| `ActionChain/chain10` | 130 \| 160 (+24%) \| 162 (+25%) µs | 803 \| 978 \| 968 | 198 \| 213 \| 212 KiB |
| `ActionChain/chain100` | 0.85 \| 1.18 (+38%) \| 1.16 (+36%) ms | 6 632 \| 8 166 \| 8 155 | 794 \| 918 \| 918 KiB |
| `ActionChain/chain1000` | 17.0 \| 31.0 (+82%) \| 31.2 (+83%) ms | 64.96k \| 80.75k \| 80.68k | 8.34 \| 9.83 \| 9.84 MiB |
| `StateLoop/count50` | 1.04 \| 1.17 (+13%) \| 1.14 (~) ms | 12.11k \| 14.18k \| 13.72k | 923 \| 1 003 \| 1 007 KiB |
| `StateLoop/count500` | 8.72 \| 10.5 \| 10.0 ms | 117.9k \| 138.5k \| 133.9k | 6.57 \| 7.66 \| 7.56 MiB |
| `StateLoop/count5000` | 85.6 \| 97.5 (~) \| 96.5 (+13%) ms | 1.176M \| 1.381M \| 1.336M | 63.3 \| 74.7 \| 73.5 MiB |

Package geomean: allocations +19.5% on the candidate, +15.8% with the
fixes; bytes +11.0% and +8.3%. (`ImportValues`' time is the
benchmark's fixed per-value budget and does not move.)

The rows group by cause as in the summary: the import graph and analysis
(#1298 → #1349, #1276/#1319 carried), instantiation and constraints
(#1276 → #1351, remainder carried), the vehicle rows and
`GRPCParseFileUncached` (#1062 → #1348, remainder carried), and the
executor's statement rows (`ActionLoop`, `ActionChain`, `StateLoop`,
`ExecuteState`, `ExecuteAction*`: #1136's per-execution stamping, order
rebuilding and transition closures, fixed after the tables by #1355 and
#1356), and the three small carried rows. `ActionChain/chain1000` is the
one row whose time moved more than its allocations (+82% against +24%):
each rebuilt order is few objects but much work, and it moves the same
with the six fixes applied.

## Stress constellation (`tests/stressmodel`)

Six rounds on both revisions, allocations exact; the 32-satellite rows'
times and third column are from the quiet re-run (six rounds, three
binaries interleaved), the 128- and 512-satellite times from the main
run. `ConvertAPIJSON/satellites=512` was skipped as before.

| benchmark | time (s) | allocs/op | B/op |
| --------- | -------- | --------- | ---- |
| `Load/satellites=32` | 0.548 \| 0.666 (+22%) \| 0.575 (~) | 2.228M \| 3.293M \| 2.382M | 165 \| 206 MiB |
| `Load/satellites=128` | 1.93 \| 2.61 (~) | 8.491M \| 12.78M | 585 \| 774 MiB |
| `Load/satellites=512` | 9.15 \| 11.35 (+24%) | 33.55M \| 50.74M | 2.13 \| 2.84 GiB |
| `LoadFiles/satellites=32` | 0.634 \| 0.846 (~) \| 0.690 (~) | 2.234M \| 3.296M \| 2.385M | 181 \| 220 MiB |
| `LoadFiles/satellites=128` | 2.57 \| 3.12 (~) | 7.885M \| 12.17M | 581 \| 760 MiB |
| `LoadFiles/satellites=512` | 9.79 \| 13.41 (+37%) | 30.48M \| 47.65M | 2.08 \| 2.77 GiB |
| `ValidateSplit/satellites=32/jobs=1` | 0.794 \| 0.997 (+26%) \| 0.840 (~) | 2.780M \| 4.011M \| 3.099M | 225 \| 280 MiB |
| `ValidateSplit/satellites=32/jobs=8` | 0.290 \| 0.431 (+49%) \| 0.387 (+33%) | 2.796M \| 4.026M \| 3.114M | 227 \| 283 MiB |
| `ValidateSplit/satellites=128/jobs=1` | 2.70 \| 3.80 (~) | 9.683M \| 14.58M | 723 \| 964 MiB |
| `ValidateSplit/satellites=512/jobs=1` | 10.8 \| 13.5 (~) | 37.29M \| 56.86M | 2.58 \| 3.51 GiB |
| `OpenSplit/satellites=32/cold` | 0.264 \| 0.447 (+69%) \| 0.374 (+42%) | 2.655M \| 3.889M \| 2.974M | 206 \| 262 MiB |
| `OpenSplit/satellites=32/warm` | 0.138 \| 0.151 (~) \| 0.143 (~) | 696.3k \| 723.6k \| 723.6k | 83.8 \| 84.5 MiB |
| `OpenSplit/satellites=512/cold` | 3.95 \| 5.35 (~) | 35.20M \| 54.76M | 2.29 \| 3.21 GiB |
| `OpenSplit/satellites=512/warm` | 1.11 \| 1.05 (~) | 5.806M \| 6.143M | 934 \| 940 MiB |
| `EditImported/satellites=32` | 0.607 \| 0.790 (+30%) \| 0.741 (+22%) | 2.268M \| 3.347M \| 2.435M | 139 \| 186 MiB |
| `EditImported/satellites=512` | 9.45 \| 11.9 (~) | 29.56M \| 46.71M | 1.67 \| 2.39 GiB |
| `EditBeside/satellites=32` (ms) | 1.48 \| 1.50 (~) \| 1.48 (~) | 4 541 \| 4 961 \| 4 629 | 572 \| 604 KiB |
| `EditBeside/satellites=512` (ms) | 16.3 \| 19.7 (~) | 41.69k \| 47.03k | 6.99 \| 7.41 MiB |
| `Satisfy/satellites=32` (ms) | 5.78 \| 7.22 (+25%) \| 5.93 (~) | 33.42k \| 54.48k \| 41.07k | 1.85 \| 2.30 MiB |
| `Satisfy/satellites=512` (ms) | 117 \| 153 (~) | 530.9k \| 867.8k | 29.6 \| 36.8 MiB |
| `FleetSatisfy/satellites=32` (ms) | 1.41 \| 2.06 (+46%) \| 1.54 (~) | 8 852 \| 15 180 \| 11.26k | 492 \| 621 KiB |
| `FleetSatisfy/satellites=512` (ms) | 6.13 \| 8.51 (~) | 41.55k \| 74.12k | 2.33 \| 2.98 MiB |
| `FleetInstantiate/satellites=32` (ms) | 4.99 \| 6.18 (~) \| 5.15 (~) | 14.39k \| 16.69k \| 15.45k | 9.58 \| 8.83 MiB |
| `FleetInstantiate/satellites=512` (ms) | 77.7 \| 74.8 (~) | 216.1k \| 248.7k | 177 \| 155 MiB |
| `HydratePlane/satellites=32` (ms) | 87.6 \| 87.7 (~) \| 88.3 (~) | 253.0k \| 278.3k \| 278.2k | 21.7 \| 22.5 MiB |
| `HydratePlane/satellites=512` (ms) | 847 \| 957 (~) | 2.231M \| 2.648M | 217 \| 229 MiB |
| `ConvertAPIJSON/satellites=32` | 8.21 \| 8.32 (~) \| 9.35 (~) | 25.26M \| 29.20M \| 29.20M | 4.69 \| 5.49 GiB |
| `ConvertAPIJSON/satellites=128` | 32.3 \| 37.5 (+16%) | 99.04M \| 114.7M | 18.3 \| 22.3 GiB |

The load, validation and edit rows carry the analysis cost (+44–58%
allocations, +21–42% bytes, growing with the constellation): `LoadFiles/32`
2.239M → 2.882M bisects to #1298 and #1349 restores it (2.239M, and
`EditImported/32` 2.296M); the rest is #1276 and #1319 with #1350. The
`Satisfy` and `FleetSatisfy` rows are #1276 (`Satisfy/32` 33.8k → 42.7k
at that merge, then #1319's share) and #1351 recovers 60% of them. The
warm `OpenSplit` rows (+4–6%) and `HydratePlane` (+10–19%) move with the
model's type sets. `ConvertAPIJSON` is #1253's larger element form:
+16% allocations and +17–22% bytes on both sizes; its time is not
significant on the quiet machine at 32 satellites (8.21 against 8.32 s,
spreads ±19–35%) and the loaded +16% at 128 is read with that in mind.
`FleetInstantiate` allocates 15% more but no more bytes.

On the quiet machine the fixes return `Load`, `LoadFiles`,
`ValidateSplit/jobs=1`, `Satisfy`, `FleetSatisfy` and the warm rows to
0.10.1's time (not significant, p ≥ 0.13) with +7–12% allocations left;
`EditImported` (+22%), `OpenSplit/cold` (+42%) and `ValidateSplit/jobs=8`
(+33%) stay slower with the fixes, and are the rows where the analysis
remainder is paid per file on a cold index or across eight workers at
once.
`AnalyzeSplitPerDocument` and `PlaneResidency` fail on both revisions, as
in the earlier records.

## Whole-binary scaling

`sysml -validate` over generated compliant models (the satellite-network
generator at 3 254, 6 198 and 12 086 declarations) that report no
errors, nine runs each with the three binaries interleaved; median wall
seconds of the eight warm runs and median maximum resident set, every
run exiting 0; the first run of each binary on each model is excluded
by the odd count as the cold run, though none was slow here (0.11–0.12 s
on the smallest model, 0.34–0.36 s on Apollo 11): all three binaries
had been run before the timing pass, so there was no cold round.

| declarations | 0.10.1 | 0.11.0 candidate | candidate + fixes |
| ------------ | ------ | ---------------- | ----------------- |
| 3 254 | 0.155 s / 100 MiB | 0.150 s / 103 MiB | 0.185 s / 104 MiB |
| 6 198 | 0.190 s / 115 MiB | 0.180 s / 117 MiB | 0.200 s / 118 MiB |
| 12 086 | 0.310 s / 158 MiB | 0.325 s / 160 MiB | 0.330 s / 160 MiB |

Linear in the declarations on all three binaries, as before, and at
parity on time: the differences are one to three ticks of the 10 ms
timer inside the 0.06–0.11 s spread of the warm runs (0.10.1's own runs
on the smallest model span 0.13–0.19 s). The resident set is 2–3 MiB
higher on the candidate at every size: the larger binary and the
analysis bytes above, which at these sizes are a few megabytes.

### Process start and session floor

| measure | 0.10.1 | 0.11.0 candidate | candidate + fixes |
| ------- | ------ | ---------------- | ----------------- |
| `sysml -e 1` wall time | 0.02 s | 0.03 s | 0.03 s |
| `sysml -e 1` max RSS (session floor) | 69 MiB | 70 MiB | 71 MiB |
| `bin/sysml` size | 47.60 MiB | 49.20 MiB | 49.21 MiB |
| `bin/sysml-lsp` size | 26.00 MiB | 27.16 MiB | 27.18 MiB |

The floor is the standard library's bundled index decoded at start, 1–2
MiB higher; the binaries are 1.6 MiB and 1.2 MiB larger (227 merges
of code, among them the API JSON element form, the CI formats
and the document backends). The start time's one tick is at the timer's
resolution.

### Example models and the Apollo 11 model

| model | 0.10.1 | 0.11.0 candidate | candidate + fixes |
| ----- | ------ | ---------------- | ----------------- |
| `examples/action-executor-demo.sysml` | 0.03 s / 70 MiB | 0.03 s / 72 MiB | 0.03 s / 73 MiB |
| `examples/disposal-robot-demo/robot.sysml` | 0.05 s / 77 MiB | 0.05 s / 79 MiB | 0.05 s / 79 MiB |
| Apollo 11 (28 files, `scripts/download-apollo11.sh`) | 0.350 s / 162 MiB | 0.350 s / 168 MiB | 0.345 s / 168 MiB |

The two small examples sit on the session floor and move with it; the
Apollo 11 model validates in the same time on all three binaries holding
6 MiB more, and every file validates with the same report on all three
(three `incommensurable-operands` warnings, no error). Everything in
this section that a user of the binary would notice is the resident set:
+1–6 MiB, which is the bundled index and the wider type sets, not a
regression in time.

## Re-runs

Three quiet passes were made after the six main rounds, each with
nothing else on the machine. The first re-ran `Parse/synthetic` and
`Parse/vehicle` alone, eight rounds of 0.10.1 and the candidate
alternating. The second re-ran `internal/exec/runtime`,
`internal/frontend/repl`, `internal/frontend/lsp`,
`internal/frontend/grpc`, `internal/workspace/model` and `tests/perf`
whole, six rounds of 0.10.1, the candidate and the candidate with the
fixes alternating; the third re-ran the 32-satellite rows of
`tests/stressmodel` the same way. Every time in the tables above comes
from these passes except the 128- and 512-satellite stress rows.

What the loaded rounds flagged that the quiet passes cleared: the two
`Parse` rows (+16–19% time with allocations identical to the unit: eight
quiet rounds put them at p = 0.13 and p = 0.065 with spreads of ±5–31%);
`WorkspaceUpdate` (+15% loaded, not significant quiet); `AnalyseUnresolved`;
`FleetInstantiate`, `HydratePlane` and `EditBeside`'s time; and the
stress rows' time generally, whose loaded spreads (±23–374% on the
concurrent rows) exceeded their differences. What the quiet passes
confirmed: every executor loop row and the root frame (p = 0.002 on
each, returned by the fixes), the language server's two cold rows, the
analysis and instantiation rows, `ConvertAPIJSON`'s time with its bytes,
and the lowered-graph executor rows. Allocation counts agreed between
the main run and every quiet pass on every row whose count is
deterministic: the machine moves time, not allocations.

## What this does not measure

- **The language server's per-query cost after #1348.** The ~1 GiB the
  two cold rows still allocate is the per-query re-enumeration named in
  the summary; its fix (`perf/memoize-namespace-children`) follows this
  record and is not measured here.
- **The executor fixes.** `perf/stamp-statement-footprint-once` (#1355)
  and `perf/take-transition-without-closures` (#1356) were written after
  the six-round tables; their A/Bs against the parent of #1136 are in
  their own pull requests, not here.
- **Concurrent analysis.** `ValidateSplit/jobs=8` and `OpenSplit` run on
  8 CPUs; their loaded spreads are the machine's, and only the quiet
  32-satellite rows' time is stated.
- **The bare `develop` head.** `d8b60ed62` without #1350 is measured only
  far enough (two rounds) to state what #1350 restores; it is not a
  release candidate.
