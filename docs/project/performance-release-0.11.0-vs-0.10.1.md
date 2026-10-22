# Performance: release 0.11.0 against release 0.10.1

The release-gate measurement of the 0.11.0 candidate — the head of
`develop` at `5db0f74158f1ecd43509d90141e5055cb0c3e539` (2026-10-21), from
which `release/0.11.0` is to be cut — measured against the `v0.10.1` tag
on `main` (`1c45b6e1749fc8adc9b82d4af22fbe180b56a0e2`, 2026-10-15). It
follows the method of the
[0.10.1 against 0.10.0](performance-release-0.10.1-vs-0.10.0.md) record
and of the [0.10.0 against 0.9.2](archive/performance-release-0.10.0-vs-0.9.2.md)
record before it: every benchmark on both revisions, `benchstat` over six
runs, the rows the machine leaves in doubt re-run interleaved on a quiet
machine, and whole-binary timings on generated and real models. See
[performance](../internals/performance.md) for what the figures mean and
where the remaining cost is.

This record supersedes the one first merged for 0.11.0, which measured
an earlier candidate (`develop` at `d8b60ed62` with #1350 merged on top)
and three columns; between that measurement and this one every fix it
named merged and `develop` advanced by 23 more merges, so the candidate
here is `develop` itself and the tables have one candidate column, with
a third column only on the rows the two fixes written for this record
touch.

0.11.0 is a minor release: 256 merges to `develop` since the 0.10.1
tag, among them the hiding of colliding imports (#1062), objects typed
by every type they inherit through subsetting (#1276), the subsetting of
a usage's base features (#1319), transitive order dependence in the
executor (#1136), constancy conformance (#1298), the redefinition
closure in the executor's root frame (#1057), the API JSON element form
(#1253), nested performances counted among an item's `ownedPerformances`
(#1310) and a nested requirement's subject bound to its enclosing part
(#1312). **The expectation going in was not parity**, and the
measurement does not find it: parsing, lexing, indexing, formatting, the
migrator, the library index, the lowering of graphs and the whole binary
are at parity to the allocation or within the machine's noise; the
executor is back at 0.10.1 to the allocation after the fixes the earlier
candidate motivated (#1347, #1351, #1353, #1355, #1356); name resolution
over wildcard imports, analysis and the instantiation and constraint
paths still cost more, and the language server's two cross-file queries
cost much more. This record says by how much, why, and which part of it
is fixed.

The nine fixes the earlier candidate motivated are all in the candidate
(#1347, #1348, #1349, #1350, #1351, #1352, #1353, #1355, #1356, #1358;
their A/Bs against the merges that introduced each cost are in their
pull requests). Two more were written for this measurement, against the
costs of two merges that landed after the earlier candidate, and are the
third column where it appears:

- the semantic model decides once per member whether it restates a frame
  root or a coordinate frame's featuring reference, instead of walking its
  redefinition chain and building every feature's qualified name each
  time a type's shape is computed (`internal/semantic/semantics`, #1368;
  the cost came with #1310);
- the runtime reads the member a declaration registered from its scope's
  declaration index instead of collecting the scope's members and
  scanning them for each member of each instantiated type
  (`internal/exec/runtime`, #1369; the cost came with #1312).

## Method

Two detached worktrees, one per revision, each built with `make build`
(`go1.25.11 linux/amd64`), with the pilot and training corpora linked in
so the corpus-backed rows (`*/vehicle`, `ConnectEvaluateHTTP`,
`GRPCParseFile*`, `InstantiateWarmModel`) run on both sides. Every
benchmark package is run with `-run '^$' -bench . -benchmem -count 1`,
six rounds per revision; each round's output is an append-only file and
`benchstat` compares the six files per revision. `tests/stressmodel` runs
the same way with `ConvertAPIJSON/satellites=512` skipped, as the
previous records did. The 0.10.1 rounds are the ones taken for the
earlier candidate (the tag did not move); the candidate's six rounds
were taken on `5db0f7415` after it, alone on the machine. The 0.10.1
rounds ran while that candidate's findings were bisected beside them, so
their time column is loaded and is not what the tables show; the times
come from the quiet re-run below.

The machine is an 8-vCPU `Intel(R) Xeon(R) Processor` (GenuineIntel, one
thread per core, 320 MiB L3, 31 GiB), `GOMAXPROCS=8`. It is not the host
of the 0.10.1 record, so sec/op is comparable only between the columns
of this record, never with the earlier records; allocations and bytes
are exact and comparable across hosts.

The candidate moved three times under this measurement. The first
candidate, `d8b60ed62` with #1350 merged locally, is the one the earlier
record measured; its first bare rounds showed #1319 re-deriving a
symbol's implied generals on every feature-type query (`Load/512` 44.3M →
143.0M allocations, `Instantiate` 3.75M → 36.5M), which #1350 removed,
and the fixes that record motivated were written against it. `develop`
then advanced to `c5eb6cc30` (#1354, the base feature a declaration's
kind implies memoized) and, while those rounds ran, to `5db0f7415` with
the record itself, every one of its fixes and seventeen other merges; the
rounds were restarted there and that head is the candidate of every
table. No `release/0.11.0` branch exists on `origin` as this is written.

### What is and is not comparable

- **Allocations and bytes per operation** are deterministic on every row
  but `RenameWarm` (±9% on 0.10.1), the gRPC `InstantiateWarmModel`
  (±1–3%) and `Analyze/synthetic` (±2% between passes) and agree to the unit between the main run and the quiet
  re-run; a movement there is the code's.
- **Time** is the quiet re-run's: six rounds of the two binaries
  interleaved with nothing else on the machine, over
  `internal/exec/runtime`, `internal/frontend/{repl,lsp,grpc}`,
  `internal/workspace/model`, the rows of `tests/perf` whose loaded time
  moved and the 32-satellite rows of `tests/stressmodel`. The 128- and
  512-satellite stress rows and the `tests/perf` rows at parity carry the
  main run's time with that caveat; `Parse/*` carry the earlier
  candidate's eight isolated rounds, which cleared them.
- **The third column** (candidate with #1368 and #1369) exists only on
  the rows those fixes move and comes from its own six interleaved quiet
  rounds (candidate | #1368 | #1369 | both).
- **`tests/codegen`** has benchmarks only on the candidate
  (`CompiledCalc/*`, added with #1226); it is reported but has no 0.10.1
  column, and five of its rows fail on the candidate by design of the
  benchmark (the C target does not take unbounded integers; `SumTo` at a
  million exceeds the interpreter's step budget).
- **`Analyze/vehicle`** has no candidate column: the benchmark fails
  (see the summary).

## Summary

**Nothing here blocks the release on time; the executor is at 0.10.1 to
the allocation; what remains is the inherent cost of six correctness
changes, one design cost that is named, and two costs fixed in the two
pull requests this record adds.** Allocations are exact (six rounds);
times are the quiet re-run's.

- **The executor** (`internal/exec/runtime`): every loop row, the root
  frame and the statement rows are at 0.10.1's count — `IntegerArithmeticLoop`
  9 072, `IntegerArithmeticNestedLoop` 26.08k, `RationalHarmonicLoop`
  4 652, `AssignmentLoopStep` 7 328, `NewRootFrame` 36, `ActionLoop/for1000`
  4 231 → 4 241, `ActionChain/chain1000` 64.98k → 66.30k, `StateLoop/count5000`
  1.176M → 1.201M (+2%), `ExecuteState` 12.13k → 12.39k — after #1347,
  #1353, #1355 and #1356, which the earlier candidate motivated (that
  candidate had them at +22% to +65%). What is left is #1276's
  declared-types lookup, one object per event (`DerivedReadWriteRead`
  65 → 67, `ExecuteAction` 245 → 254, `ExecuteActionFreshContext`
  340 → 377), and the bytes of `MaterializePlainAttributes` (43.6 → 51.1
  KiB) and `SetFeatureValueNoDependents` (352 → 416 B), which are #1136's
  wider footprint records; both carried. `BehaviorOrdersLibraryModel`
  improved, 1 927 → 1 736 (−10%, −14% bytes).
- **The language server's cross-file queries** (`internal/frontend/lsp`):
  `ReferencesCold` 44.6k → 124.7k allocs/op, 9.8 → 135 MiB, 43 → 255 ms
  (loaded; the quiet figure is in the table); `RenameWarm` 13.5k → 4.9k
  allocations and 1.37 MiB → 957 KiB, 21 → 57 ms. The earlier candidate
  had these at 6.28M / 1 010 MiB and 12.41M / 1 668 MiB: #1062 (hidden
  colliding imports) made the enumeration of a wildcard import's members
  build, per member, its qualified name, a one-candidate slice and its
  route conditions, and re-enumerated every import per document or per
  rename occurrence. #1348 removed the per-member work and #1358 keeps an
  import's members for the resolver's life (journaled with the import's
  document), which is why `RenameWarm` now allocates less than 0.10.1.
  `ReferencesCold` still allocates 2.8× and takes longer: the first
  enumeration of each import is #1062's, by design (every colliding
  import's members must be enumerated to know what is hidden), and the
  kept results are built once per query. **This is the one row 0.11.0
  ships above 0.10.1 by more than its correctness changes explain in
  work**, and it is a design cost, named: a references query over a
  cold workspace enumerates every wildcard import once. `ReferencesWarm`,
  `WorkspaceUpdate`, `FormatEdits` and `SpanToRangeLarge` are unchanged
  to the allocation.
- **Analysis and the import graph** (`ImportValues/*` +12–18%
  allocations, `Analyze/synthetic` 4.38M → 4.77M (+9%),
  `WorkspaceEdit/synthetic reindex+diagnostics` 4.83M → 5.21M, the REPL's
  `LoadModel` +4–7%, `REPLLoadFile` 5.33M → 5.72M, `REPLSubmitSnippet`
  4.70M → 5.45M, the workspace model's `AnalyseResolved` 5.6k → 7.6k): the
  earlier candidate had these at +59–72%, +33%, +30%, +27–29% and +89%;
  #1349 (the conformance by the element a name resolves to), #1350,
  #1351 and #1354 took back what #1298, #1319 and #1276 had added
  avoidably. The remainder is #1276 and #1319 typing each feature by
  every type it inherits through subsetting and subsetting a usage's base
  features — more semantic work per usage by design — and is carried as
  the inherent cost of the two changes.
- **Instantiation and constraint checking** (`Instantiate` 2.51M → 6.34M
  allocs/op, `FeaturesOf` 4.13M → 6.45M, `BatchSatisfy` 4.30M → 8.30M,
  `GRPCVerifyConstraint` 9 786 → 14 668, `SameConstraintManyInstances`
  35 → 38, the stress constellation's `Satisfy` +26% and `FleetSatisfy`
  +30–33%): the earlier candidate, with #1351, had `Instantiate` at 4.03M
  and the rest of its growth attributed to #1276; this candidate is
  worse, and the first-parent probe over the 23 merges since puts the
  difference at two: #1310 (`Instantiate` 3.82M → 4.94M) and #1312
  (4.94M → 6.34M). Neither is the change's own work. #1310 made each
  type's shape computation ask, for every library member, whether it
  restates a frame root, and that walk built every feature's qualified
  name on every shape — once per type, a thousand types in the
  benchmark; #1312 looks up a nested requirement's redefining default
  for each member of each instantiated type by collecting the scope's
  members and scanning them. #1368 memoizes the first per symbol and
  #1369 reads the second from the scope's declaration index; six quiet
  rounds (candidate | #1368 | #1369 | both) put `Instantiate` at 6.34M |
  4.22M | 5.01M | 2.22M, `BatchSatisfy` at 8.30M | 5.67M | 6.97M | 4.34M
  and `FeaturesOf` at 6.45M | 3.68M | 5.03M | 2.26M — with both,
  `Instantiate` and `FeaturesOf` are below 0.10.1 (2.51M, 4.13M: the
  memo also serves the shapes 0.10.1 computed per type) and `BatchSatisfy`
  is at parity (4.30M). The stress constellation's `Satisfy/32` 33.4k →
  42.0k → 40.0k and `FleetSatisfy/32` 8.85k → 11.50k → 11.00k keep +20–24%
  after #1369 (#1368 does not reach them: the plane's types are few):
  that remainder is #1276's and #1319's wider type set, which the earlier
  candidate measured at the same level (`Satisfy/32` 41.1k with #1351),
  and is carried.
  `GRPCVerifyConstraint`'s +4 900 is #1310's own work — a verification
  now begins a run for each nested performance it counts
  (`beginRun`/`resolveBindings`/`beginJournal` per performance) — and is
  carried as inherent.
- **The REPL's diagnostics** (`Diagnostics` 561 → 657 allocs/op at 50
  attributes, +37–42% bytes): #1205 (CI formats) encoded the line prefix
  to UTF-16 per diagnostic to read its length and copied the snippet's
  source twice; #1352 removed that (the earlier candidate had 691). The
  remaining +96 per 50 attributes is the supertype walk the diagnostic's
  formatting makes since #1161 (project interchange dependencies) over
  #1276's wider type set; carried.
- **The API JSON conversion** (`ConvertAPIJSON` 25.26M → 29.20M
  allocs/op and 4.69 → 5.49 GiB at 32 satellites, +16% / +22% at 128,
  +16–17% time): #1253 states `isComposite` on every feature in the
  element form. The output is larger; inherent.
- **The stress constellation's load and analysis rows**
  (`Load`/`LoadFiles`/`EditImported` +3.5–5.7% allocations,
  `ValidateSplit` +5.6–7.9%, `OpenSplit/cold` +6–8%, `HydratePlane`
  +9–17%): the earlier candidate had `Load/512` at +200% before #1350
  and +11% after; #1354 took it to this. The remainder is #1319's
  subsetting of a usage's base features over a constellation of
  thousands of usages; carried.
- **The vehicle rows** (`WorkspaceEdit/vehicle reindex+diagnostics`
  858k → 876k allocs/op, `GRPCParseFileUncached` 903k → 983k, and
  `Analyze/vehicle`, which fails on the candidate): since #1062 the
  Annex A `SimpleVehicleModel` reports 20 unresolved references
  (`vehicle_b`, `providePower`, `cost`, `ignitionCmdPort`, …) that
  0.10.1 resolved by binding the first match of a colliding import; the
  merge moved the pilot ratchet for that file 134 → 154 and the
  benchmark asserts an error-free analysis, so the row cannot be
  measured on 0.11.0. The two rows that still run pay the suggestion
  path of those diagnostics (the earlier candidate had them at 1 206k
  and 1 331k before #1348 and #1358).
- **The library index** (`DecodeSnapshot` 67.6k → 72.1k allocs/op,
  +2.4% bytes): #1357 records each library symbol's implied generals in
  the bundled snapshot (3 950 367 → 3 974 247 bytes) so the resolver
  reads them instead of re-deriving them; more recorded data, decoded at
  every start; inherent and 0.1 ms.
- **Small rows with their cause named and carried**: the REPL's
  `Instantiate` 56 → 59 is #1276's (the wider type set);
  `ConnectParseFileHTTPCached` 1 017 → 1 151 is #1062's collision
  bookkeeping on the parse path; the gRPC `InstantiateWarmModel`
  1 835 → 2 015 is #1325's API JSON writer provenance on the instantiate
  reply; the REPL's `RunStateMachine` 224 → 228 is #1315's;
  `SetDigest` 1 004 → 1 029 and `LowerStateGraph` 149 → 152 are within
  the changes' own work.

What improved: `BehaviorOrdersLibraryModel` 1 927 → 1 736 allocs/op, the
REPL's `RunCalc` 79 → 74, `RenameWarm` 13.5k → 4.9k, `IndexAddExpand/vehicle`
and `WorkspaceEdit/vehicle` −1%, and with #1368 and #1369 `FeaturesOf`
4.13M → 2.26M and `Instantiate` 2.51M → 2.22M.

Known from the earlier records and unchanged: `AnalyzeSplitPerDocument`
and `PlaneResidency` in `tests/stressmodel` fail on both revisions and
are not measured.

## Package benchmarks

Allocations and bytes are from the six main rounds (exact); times are the
quiet re-run's medians where that pass covered the row and the main
run's otherwise (marked †). Columns: 0.10.1 → 0.11.0 candidate, with the
`benchstat` verdict in parentheses (`~` is not significant at p < 0.05
over six rounds); a third figure, where present, is the candidate with
#1368 and #1369.

### `internal/exec/runtime`

| benchmark | time | allocs/op | B/op |
| --------- | ---- | --------- | ---- |
| `MaterializePlainAttributes` | 56.84µ → 67.44µ (~) | 618.0 → 621.0 (+0%) | 43.60Ki → 51.14Ki (+17%) |
| `SetFeatureValueNoDependents` | 438.7n → 496.1n (~) | 9.000 → 9.000 (~) | 352.0 → 416.0 (+18%) |
| `DerivedReadWriteRead` | 6.582µ → 7.918µ (+20%) | 65.00 → 67.00 (+3%) | 2.484Ki → 2.625Ki (+6%) |
| `IntegerArithmeticLoop` | 3.758m → 4.449m (~) | 9.072k → 9.072k (~) | 279.9Ki → 295.5Ki (+6%) |
| `IntegerArithmeticLoopBeyondInt64` | 4.003m → 4.096m (~) | 13.07k → 13.07k (~) | 514.3Ki → 529.9Ki (+3%) |
| `IntegerArithmeticNestedLoop` | 5.365m → 6.072m (~) | 26.08k → 26.08k (~) | 915.0Ki → 1022.5Ki (+12%) |
| `BehaviorOrdersLibraryModel` | 352.9µ → 344.4µ (~) | 1.927k → 1.736k (-10%) | 164.4Ki → 141.5Ki (-14%) |
| `RationalDecimalLoop` | 4.117m → 4.775m (+16%) | 9.070k → 9.070k (~) | 279.9Ki → 295.5Ki (+6%) |
| `RationalHarmonicLoop` | 988.2µ → 1015.7µ (~) | 4.652k → 4.651k (~) | 179.0Ki → 182.2Ki (+2%) |
| `RealDecimalLoop` | 3.986m → 4.066m (+2%) | 9.070k → 9.070k (~) | 279.9Ki → 295.5Ki (+6%) |

Every loop row, `NewRootFrame` and the lowered-graph rows are at 0.10.1's
count; the bytes of the loop rows (+3–12%) and of `MaterializePlainAttributes`
and `SetFeatureValueNoDependents` are #1136's footprint records and
#1276's wider type set, carried. Rows not listed are unchanged to the
allocation and not significant on time.

### `internal/frontend/repl`

| benchmark | time | allocs/op | B/op |
| --------- | ---- | --------- | ---- |
| `LoadModel/elements=0` | 442.3µ → 512.3µ (+16%) | 1.938k → 2.019k (+4%) | 222.4Ki → 228.0Ki (+3%) |
| `LoadModel/elements=250` | 30.17m → 33.52m (+11%) | 125.8k → 133.8k (+6%) | 13.44Mi → 13.78Mi (+3%) |
| `LoadModel/elements=1000` | 129.3m → 149.5m (~) | 482.5k → 516.8k (+7%) | 53.14Mi → 54.84Mi (+3%) |
| `LoadModel/elements=4000` | 539.7m → 640.3m (+19%) | 1.909M → 2.049M (+7%) | 212.6Mi → 219.5Mi (+3%) |
| `RunStateMachine/elements=250` | 13.94µ → 15.44µ (~) | 224.0 → 228.0 (+2%) | 15.98Ki → 16.31Ki (+2%) |
| `RunStateMachine/elements=1000` | 14.61µ → 14.38µ (~) | 224.0 → 228.0 (+2%) | 15.98Ki → 16.31Ki (+2%) |
| `RunStateMachine/elements=4000` | 14.13µ → 16.41µ (~) | 224.0 → 228.0 (+2%) | 15.98Ki → 16.31Ki (+2%) |
| `RunCalc/elements=250` | 5.000µ → 4.937µ (~) | 79.00 → 74.00 (-6%) | 4.453Ki → 4.438Ki (-0%) |
| `RunCalc/elements=1000` | 5.225µ → 4.956µ (~) | 79.00 → 74.00 (-6%) | 4.453Ki → 4.438Ki (-0%) |
| `RunCalc/elements=4000` | 5.099µ → 4.883µ (~) | 79.00 → 74.00 (-6%) | 4.453Ki → 4.438Ki (-0%) |
| `Instantiate/elements=250` | 8.635µ → 9.272µ (+7%) | 56.00 → 59.00 (+5%) | 7.121Ki → 8.189Ki (+15%) |
| `Instantiate/elements=1000` | 7.905µ → 8.638µ (+9%) | 56.00 → 59.00 (+5%) | 7.168Ki → 8.127Ki (+13%) |
| `Instantiate/elements=4000` | 8.122µ → 8.498µ (~) | 56.00 → 59.00 (+5%) | 7.184Ki → 8.157Ki (+14%) |
| `Diagnostics/attributes=50` | 91.52µ → 103.56µ (~) | 561.0 → 657.0 (+17%) | 73.69Ki → 100.63Ki (+37%) |
| `Diagnostics/attributes=200` | 317.4µ → 354.7µ (~) | 1.469k → 1.719k (+17%) | 249.0Ki → 354.3Ki (+42%) |
| `Diagnostics/attributes=800` | 1.269m → 1.372m (~) | 5.623k → 6.477k (+15%) | 896.3Ki → 1259.0Ki (+40%) |

`LoadModel` carries #1276/#1319 (+4–7%), `Diagnostics` the formatting
walk named in the summary, `Instantiate` the wider type set,
`RunStateMachine` #1315's; `RunCalc` improved. The seven other rows
(`Eval*`, `Complete*`, `Render*`, …) are unchanged to the allocation.

### `internal/frontend/grpc`

| benchmark | time | allocs/op | B/op |
| --------- | ---- | --------- | ---- |
| `InstantiateWarmModel` | 193.4µ → 222.5µ (+15%) | 1.835k → 2.015k (+10%) | 104.4Ki → 123.1Ki (+18%) |

`InstantiateWarmModel` is #1325's writer provenance on the reply; the
parse rows are at parity (−0.4–0.6% allocations, the collision
bookkeeping of #1062 being offset by #1348).

### `internal/frontend/lsp`

| benchmark | time | allocs/op | B/op |
| --------- | ---- | --------- | ---- |
| `ReferencesCold` | 44.47m → 190.60m (+329%) | 44.64k → 124.74k (+179%) | 9.786Mi → 134.799Mi (+1277%) |
| `RenameWarm` | 21.91m → 49.10m (+124%) | 13.533k → 4.901k (-64%) | 1370.1Ki → 956.9Ki (-30%) |

See the summary for the two cold rows. `RenameWarm`'s 0.10.1 count varies
±9% between rounds (its mean is shown); the candidate's does not.

### `internal/translate/migrate`, `internal/workspace/libs`, `internal/workspace/model`, `tests/codegen`

| benchmark | time | allocs/op | B/op |
| --------- | ---- | --------- | ---- |
| benchmark | time | allocs/op | B/op |
| --------- | ---- | --------- | ---- |
| `DecodeSnapshot` | 16.42m → 16.35m (~) † | 67.45k → 72.06k (+7%) | 37.85Mi → 38.76Mi (+2%) |
| `SetDigest` | 1.322m → 1.144m (~) † | 1.004k → 1.029k (+2%) | 1.737Mi → 1.775Mi (+2%) |
| benchmark | time | allocs/op | B/op |
| --------- | ---- | --------- | ---- |
| `AnalyseResolved` | 1.376m → 1.753m (+27%) | 5.597k → 7.600k (+36%) | 584.7Ki → 646.0Ki (+10%) |

The migrator and the library index are at parity (`DecodeSnapshot` and
`SetDigest` carry #1357's recorded implied generals); `AnalyseResolved`
carries #1276/#1319 (+36% allocations, the earlier candidate +89%).
`tests/codegen` (`CompiledCalc/*`, candidate only) runs seven of its
twelve rows: `Fib(25)` interpreted 6.8 ms and compiled to Go 1.8 ms,
`Collatz(27)` 1.16 ms and 12 µs; the C target of each is not compilable
(it holds `int64`, the interpreter's integers are unbounded) and
`SumTo(1000000)` exceeds the interpreter's step budget; no 0.10.1 column
exists.

### `tests/perf`

| benchmark | time | allocs/op | B/op |
| --------- | ---- | --------- | ---- |
| `ImportValues/flat/parts=100/attrs=3` | 22.71m → 27.19m (+20%) | 131.2k → 153.1k (+17%) | 10.51Mi → 12.24Mi (+16%) |
| `ImportValues/flat/parts=500/attrs=3` | 108.0m → 138.1m (+28%) | 616.4k → 724.0k (+17%) | 58.96Mi → 65.54Mi (+11%) |
| `ImportValues/flat/parts=2000/attrs=3` | 499.3m → 634.7m (+27%) | 2.439M → 2.868M (+18%) | 248.0Mi → 274.8Mi (+11%) |
| `ImportValues/flat/parts=2000/attrs=5` | 745.2m → 893.9m (+20%) | 3.682M → 4.328M (+18%) | 348.7Mi → 387.1Mi (+11%) |
| `ImportValues/nested/parts=100/attrs=3` | 30.89m → 37.29m (~) | 175.5k → 198.3k (+13%) | 14.81Mi → 16.80Mi (+13%) |
| `ImportValues/nested/parts=500/attrs=3` | 148.9m → 181.4m (+22%) | 830.7k → 943.7k (+14%) | 73.35Mi → 80.31Mi (+9%) |
| `ImportValues/nested/parts=2000/attrs=3` | 700.6m → 781.6m (+12%) | 3.290M → 3.741M (+14%) | 304.4Mi → 332.7Mi (+9%) |
| `ImportValues/nested/parts=2000/attrs=5` | 1.041 → 1.100 (~) | 4.578M → 5.150M (+12%) | 418.6Mi → 455.8Mi (+9%) |
| `IndexAddExpand/vehicle` | 28.00m → 32.54m (~) † | 139.7k → 137.7k (-1%) | 15.24Mi → 15.15Mi (-1%) |
| `Analyze/synthetic` | 1.515 → 1.807 (+19%) † | 4.382M → 4.766M (+9%) | 485.6Mi → 504.3Mi (+4%) |
| `WorkspaceEdit/synthetic/reindex+diagnostics` | 1.981 → 2.082 (~) | 4.828M → 5.214M (+8%) | 515.6Mi → 528.1Mi (+2%) |
| `WorkspaceEdit/vehicle` | 70.57m → 59.45m (~) | 168.1k → 166.1k (-1%) | 14.59Mi → 14.50Mi (-1%) |
| `WorkspaceEdit/vehicle/reindex+diagnostics` | 237.0m → 276.2m (~) | 857.7k → 876.4k (+2%) | 120.8Mi → 141.6Mi (+17%) |
| `WorkspaceEditSmallDocBesideLarge` | 51.41m → 51.80m (~) | 114.6k → 114.7k (+0%) | 23.67Mi → 24.14Mi (+2%) |
| `FeaturesOf` | 1.065 → 1.223 (+15%) → 668.0m | 4.130M → 6.450M (+56%) → 2.263M | 340.1Mi → 412.9Mi (+21%) → 261.0Mi |
| `REPLLoadFile` | 1.632 → 1.753 (+7%) | 5.333M → 5.717M (+7%) | 613.8Mi → 632.8Mi (+3%) |
| `REPLSubmitSnippet` | 1.182 → 1.377 (+17%) | 4.695M → 5.446M (+16%) | 543.0Mi → 575.9Mi (+6%) |
| `LowerActionGraph` | 3.372µ → 3.179µ (~) † | 38.00 → 38.00 (~) | 3.836Ki → 3.914Ki (+2%) |
| `LowerStateGraph` | 10.096µ → 9.313µ (~) † | 149.0 → 152.0 (+2%) | 12.12Ki → 12.45Ki (+3%) |
| `ExecuteAction` | 82.83µ → 91.12µ (+10%) | 245.0 → 254.0 (+4%) | 177.9Ki → 193.3Ki (~) |
| `ExecuteActionFreshContext` | 46.70µ → 55.99µ (+20%) | 340.5 → 377.0 (+11%) | 39.93Ki → 47.72Ki (+20%) |
| `ExecuteState` | 1.181m → 1.315m (~) † | 12.13k → 12.39k (+2%) | 843.5Ki → 846.7Ki (~) |
| `SameConstraintManyInstances` | 5.248µ → 5.315µ (~) | 35.00 → 38.00 (+9%) | 1.406Ki → 1.492Ki (+6%) |
| `BatchSatisfy` | 1.448 → 1.996 (+38%) → 1.418 | 4.301M → 8.301M (+93%) → 4.336M | 432.6Mi → 601.6Mi (+39%) → 455.9Mi |
| `Instantiate` | 683.2m → 1416.3m (+107%) → 727.9m | 2.509M → 6.338M (+153%) → 2.219M | 235.0Mi → 425.4Mi (+81%) → 206.7Mi |
| `GRPCParseFileUncached` | 227.7m → 247.8m (~) | 902.7k → 983.2k (+9%) | 143.0Mi → 176.9Mi (+24%) |
| `GRPCEvaluate` | 10.48µ → 10.95µ (~) | 117.0 → 118.0 (+1%) | 9.465Ki → 9.708Ki (+3%) |
| `GRPCVerifyConstraint` | 1.026m → 1.651m (+61%) → 1.559m | 9.786k → 14.668k (+50%) → 14.66k | 543.4Ki → 854.3Ki (+57%) → 854.2Ki |
| `ConnectParseFileHTTPCached` | 1.015m → 1.229m (+21%) | 1.017k → 1.151k (+13%) | 464.2Ki → 493.5Ki (+6%) |
| `ActionLoop/for10` | 41.12µ → 49.05µ (~) † | 264.0 → 273.0 (+3%) | 32.41Ki → 35.40Ki (+9%) |
| `ActionLoop/for100` | 169.3µ → 192.3µ (~) † | 627.0 → 636.0 (+1%) | 70.79Ki → 75.18Ki (+6%) |
| `ActionLoop/for1000` | 1.493m → 1.787m (~) † | 4.231k → 4.241k (+0%) | 414.7Ki → 433.2Ki (+4%) |
| `ActionChain/chain10` | 118.8µ → 137.2µ (+16%) | 803.0 → 823.0 (+2%) | 193.1Ki → 200.6Ki (~) |
| `ActionChain/chain100` | 818.9µ → 916.6µ (+12%) | 6.632k → 6.744k (+2%) | 774.9Ki → 831.7Ki (+7%) |
| `ActionChain/chain1000` | 16.92m → 18.42m (+9%) | 64.98k → 66.30k (+2%) | 7.958Mi → 8.655Mi (+9%) |
| `StateLoop/count50` | 1.013m → 1.141m (+13%) | 12.11k → 12.37k (+2%) | 877.5Ki → 889.2Ki (~) |
| `StateLoop/count500` | 8.816m → 9.297m (+5%) | 117.9k → 120.4k (+2%) | 6.272Mi → 6.369Mi (+2%) |
| `StateLoop/count5000` | 83.92m → 92.31m (+10%) | 1.176M → 1.201M (+2%) | 60.34Mi → 61.40Mi (+2%) |

Rows at parity to the allocation and not significant on time are
omitted: the lexer and parser rows (`Lex*`, `Parse*` — eight isolated
rounds, p = 0.13 and 0.065), `Index*`, `FQNOf`, `LookupQualified`,
`Format*`, `Lower*`, `REPLEvalExpr`, `GRPCEvaluate`, `ConnectEvaluateHTTP`
and the executor rows #1347/#1355/#1356 returned. The instantiation and
constraint rows have the third column; the import-graph rows carry
#1276/#1319.

## Stress constellation (`tests/stressmodel`)

| benchmark | time | allocs/op | B/op |
| --------- | ---- | --------- | ---- |
| `Load/satellites=32/elements=6227` | 478.5m → 562.3m (+18%) | 2.228M → 2.306M (+4%) | 157.0Mi → 165.1Mi (+5%) |
| `Load/satellites=128/elements=24167` | 1.928 → 2.281 (~) † | 8.491M → 8.896M (+5%) | 558.2Mi → 613.6Mi (+10%) |
| `Load/satellites=512/elements=95927` | 9.149 → 9.828 (~) † | 33.55M → 35.26M (+5%) | 2.126Gi → 2.355Gi (+11%) |
| `Satisfy/satellites=32/assertions=96` | 5.249m → 6.184m (+18%) → 6.151m | 33.42k → 42.03k (+26%) → 40.04k | 1.768Mi → 2.229Mi (+26%) → 2.201Mi |
| `Satisfy/satellites=128/assertions=384` | 25.16m → 28.79m (~) → 28.61m † | 132.9k → 167.3k (+26%) → 159.4k | 7.064Mi → 8.911Mi (+26%) → 8.797Mi |
| `Satisfy/satellites=512/assertions=1536` | 116.5m → 139.9m (~) † | 530.9k → 668.6k (+26%) | 28.26Mi → 35.65Mi (+26%) |
| `FleetInstantiate/satellites=32/elements=1179` | 4.629m → 4.972m (+7%) → 5.051m | 14.39k → 15.45k (+7%) → 15.29k | 9.136Mi → 8.711Mi (~) → 7.471Mi |
| `FleetInstantiate/satellites=128/elements=1715` | 16.17m → 16.89m (~) → 17.46m † | 54.74k → 58.59k (+7%) → 57.95k | 31.38Mi → 32.15Mi (+2%) → 32.12Mi |
| `FleetInstantiate/satellites=512/elements=3947` | 77.73m → 89.79m (~) † | 216.1k → 231.3k (+7%) | 169.2Mi → 184.6Mi (~) |
| `FleetSatisfy/satellites=32/assertions=24` | 1.243m → 1.651m (+33%) → 1.507m | 8.852k → 11.496k (+30%) → 11.00k | 480.2Ki → 608.7Ki (+27%) → 601.4Ki |
| `FleetSatisfy/satellites=128/assertions=36` | 2.006m → 2.947m (~) → 2.503m † | 13.65k → 17.87k (+31%) → 17.12k | 743.6Ki → 941.6Ki (+27%) → 930.8Ki |
| `FleetSatisfy/satellites=512/assertions=108` | 6.126m → 7.491m (~) † | 41.55k → 55.30k (+33%) | 2.222Mi → 2.825Mi (+27%) |
| `EditBeside/satellites=32` | 1.374m → 1.508m (~) † | 4.541k → 4.630k (+2%) | 558.7Ki → 576.4Ki (+3%) |
| `EditBeside/satellites=128` | 4.193m → 4.607m (+10%) † | 11.98k → 12.07k (+1%) | 1.756Mi → 1.804Mi (+3%) |
| `EditBeside/satellites=512` | 16.30m → 16.50m (~) † | 41.69k → 41.78k (+0%) | 6.663Mi → 6.831Mi (+3%) |
| `ValidateSplit/satellites=32/files=6/jobs=1` | 667.5m → 763.9m (~) | 2.780M → 2.934M (+6%) | 214.5Mi → 229.2Mi (+7%) |
| `ValidateSplit/satellites=32/files=6/jobs=8` | 287.8m → 367.9m (+28%) | 2.796M → 2.949M (~) | 216.5Mi → 231.1Mi (+7%) |
| `ValidateSplit/satellites=128/files=6/jobs=1` | 2.700 → 3.389 (~) † | 9.683M → 10.396M (+7%) | 689.4Mi → 771.1Mi (+12%) |
| `ValidateSplit/satellites=128/files=6/jobs=8` | 1.036 → 1.354 (~) † | 9.692M → 10.411M (+7%) | 690.2Mi → 772.7Mi (+12%) |
| `ValidateSplit/satellites=512/files=6/jobs=1` | 10.84 → 13.62 (~) † | 37.29M → 40.24M (+8%) | 2.579Gi → 2.929Gi (+14%) |
| `ValidateSplit/satellites=512/files=6/jobs=8` | 4.085 → 5.663 (~) † | 37.30M → 40.24M (+8%) | 2.580Gi → 2.930Gi (+14%) |
| `LoadFiles/satellites=32/files=6` | 592.5m → 644.8m (+9%) | 2.234M → 2.309M (+3%) | 172.9Mi → 180.5Mi (+4%) |
| `LoadFiles/satellites=128/files=6` | 2.569 → 2.691 (~) † | 7.885M → 8.281M (+5%) | 554.5Mi → 608.1Mi (+10%) |
| `LoadFiles/satellites=512/files=6` | 9.790 → 11.430 (~) † | 30.48M → 32.16M (+6%) | 2.080Gi → 2.321Gi (+12%) |
| `EditImported/satellites=32/files=6` | 633.2m → 697.3m (~) † | 2.268M → 2.363M (+4%) | 132.4Mi → 146.5Mi (+11%) |
| `EditImported/satellites=128/files=6` | 2.248 → 2.550 (~) † | 7.727M → 8.137M (+5%) | 436.0Mi → 495.2Mi (+14%) |
| `EditImported/satellites=512/files=6` | 9.453 → 11.878 (~) † | 29.56M → 31.23M (+6%) | 1.675Gi → 1.911Gi (+14%) |
| `OpenSplit/satellites=32/files=6/cold` | 283.5m → 346.8m (~) | 2.655M → 2.814M (+6%) | 196.3Mi → 211.7Mi (+8%) |
| `OpenSplit/satellites=32/files=6/warm` | 130.9m → 137.8m (~) | 696.3k → 709.4k (+2%) | 79.96Mi → 81.16Mi (+2%) |
| `OpenSplit/satellites=128/files=6/cold` | 1.078 → 1.323 (~) † | 9.166M → 9.881M (+8%) | 615.0Mi → 697.7Mi (+13%) |
| `OpenSplit/satellites=128/files=6/warm` | 300.2m → 344.3m (~) † | 1.719M → 1.769M (+3%) | 208.9Mi → 214.5Mi (+3%) |
| `OpenSplit/satellites=512/files=6/cold` | 3.946 → 5.194 (~) † | 35.20M → 38.14M (+8%) | 2.287Gi → 2.636Gi (+15%) |
| `OpenSplit/satellites=512/files=6/warm` | 1.107 → 1.037 (~) † | 5.806M → 6.006M (+3%) | 890.3Mi → 910.8Mi (+2%) |
| `HydratePlane/satellites=32/files=6` | 89.63m → 82.26m (~) | 253.0k → 276.6k (+9%) | 20.72Mi → 21.97Mi (+6%) |
| `HydratePlane/satellites=128/files=6` | 242.6m → 249.2m (~) † | 648.8k → 742.1k (+14%) | 57.24Mi → 62.10Mi (+8%) |
| `HydratePlane/satellites=512/files=6` | 847.4m → 943.8m (~) † | 2.231M → 2.603M (+17%) | 206.7Mi → 225.2Mi (+9%) |
| `ConvertAPIJSON/satellites=32/elements=6227` | 7.751 → 8.174 (~) | 25.26M → 29.20M (+16%) | 4.688Gi → 5.490Gi (+17%) |
| `ConvertAPIJSON/satellites=128/elements=24167` | 32.33 → 37.89 (+17%) † | 99.04M → 114.66M (+16%) | 18.33Gi → 22.34Gi (+22%) |

The load and analysis rows carry #1319 (+3.5–8%), `Satisfy`/`FleetSatisfy`
the #1310/#1312 costs with the third column from the fixes' quiet rounds,
`HydratePlane` #1319 over the plane's usages, `ConvertAPIJSON` #1253's
larger element form. `EditBeside`, `OpenSplit/warm` and `FleetInstantiate`
are within +2–7%; `AnalyzeSplitPerDocument` and `PlaneResidency` fail on
both revisions and are not measured. The 128- and 512-satellite rows
carry the main run's time (†).

## Whole-binary scaling

`sysml -validate` over generated compliant models (the satellite-network
generator at 3 254, 6 198 and 12 086 declarations) that report no
errors, nine runs each with the two binaries interleaved on the quiet
machine; median wall seconds of the eight warm runs and median maximum
resident set, every run exiting 0; the first run of each binary on each
model is excluded by the odd count as the cold run.

| declarations | 0.10.1 | 0.11.0 candidate |
| ------------ | ------ | ---------------- |
| 3 254 | 0.12 s / 101 MiB | 0.12 s / 104 MiB |
| 6 198 | 0.17 s / 114 MiB | 0.17 s / 118 MiB |
| 12 086 | 0.31 s / 158 MiB | 0.31 s / 162 MiB |

Linear in the declarations on both binaries, as before, and at parity
on time to the 10 ms timer at every size (the warm runs of each binary
on each model span 0.11–0.13 s, 0.17–0.18 s and 0.30–0.35 s). The
resident set is 3–4 MiB higher on the candidate at every size: the
larger binary, the recorded implied generals in the snapshot and the
analysis bytes above, which at these sizes are a few megabytes. The cold
first runs were 0.11–0.32 s on both binaries: both had been run before
the timing pass, so there was no cold round.

### Process start and session floor

| measure | 0.10.1 | 0.11.0 candidate |
| ------- | ------ | ---------------- |
| `sysml -e 1` wall time | 0.02 s | 0.02 s |
| `sysml -e 1` max RSS (session floor) | 69 MiB | 71 MiB |
| `bin/sysml` size | 47.60 MiB | 49.32 MiB |
| `bin/sysml-lsp` size | 26.00 MiB | 27.28 MiB |

The floor is the standard library's bundled index decoded at start; the
binaries are 1.7 MiB and 1.3 MiB larger (256 merges of code, among them
the API JSON element form, the CI formats, the document backends and the
recorded implied generals in the snapshot).

### Example models and the Apollo 11 model

| model | 0.10.1 | 0.11.0 candidate |
| ----- | ------ | ---------------- |
| `examples/action-executor-demo.sysml` | 0.03 s / 70 MiB | 0.03 s / 73 MiB |
| `examples/disposal-robot-demo/robot.sysml` | 0.06 s / 77 MiB | 0.05 s / 80 MiB |
| Apollo 11 (28 files, `scripts/download-apollo11.sh`) | 0.35 s / 162 MiB | 0.35 s / 169 MiB |

The two small examples sit on the session floor and move with it; every
file of the Apollo 11 model validates with the same report on both
binaries (three `incommensurable-operands` warnings, no error).
Everything in this section that a user of the binary would notice is the
resident set, which is the bundled index and the wider type sets, not a
regression in time.

## Re-runs

Two quiet passes were made for this candidate, each with nothing else on
the machine. The first ran the rows #1310 and #1312 moved six rounds
interleaved over the candidate, #1368, #1369 and both together
(`tests/perf`'s `Instantiate`, `BatchSatisfy`, `FeaturesOf`,
`GRPCVerifyConstraint`, `SameConstraintManyInstances`,
`ExecuteActionFreshContext`, `Analyze/synthetic`; the 32- and
128-satellite `Satisfy`, `FleetSatisfy` and `FleetInstantiate`; the REPL's
`Instantiate`) and is the third column. The second ran `internal/exec/runtime`,
`internal/frontend/{repl,lsp,grpc}` and `internal/workspace/model` whole,
the `tests/perf` rows whose loaded time had moved and the 32-satellite
stress rows, six rounds of 0.10.1 and the candidate alternating, and is
every time in the tables not marked †. The earlier candidate's eight
isolated `Parse/*` rounds stand (allocations identical to the unit on
this candidate too).

What the quiet pass confirmed: the language server's two cold rows
(`ReferencesCold` 44 → 191 ms, `RenameWarm` 22 → 49 ms, p = 0.002), the
analysis rows (`AnalyseResolved` 1.38 → 1.75 ms, `ImportValues` +12–28%,
`REPLSubmitSnippet` +17%), the instantiation and constraint rows
(`Instantiate` 0.68 → 1.42 s, `BatchSatisfy` 1.45 → 2.00 s,
`GRPCVerifyConstraint` 1.03 → 1.65 ms — the three the fixes' own rounds
then return to 0.73 s, 1.42 s and parity), the REPL's `LoadModel`
(+11–19%), `ConnectParseFileHTTPCached` (+21%) and the 32-satellite
`Load`, `Satisfy` and `FleetSatisfy` with their allocations. What it
cleared: `AnalyseUnresolved`, `WorkspaceUpdate`, `FormatEdits`,
`ReferencesWarm`, the gRPC parse rows, `NewRootFrame`, `Analyze/synthetic`
(p = 0.18), `HydratePlane`, `EditBeside` and `ConvertAPIJSON/32`'s time
(p = 0.09; its 128-satellite row carries the main run's +17% with its
+16% allocations).

What it leaves in doubt, stated rather than resolved: four executor rows
whose allocation counts are identical on both revisions moved on time at
p = 0.009–0.026 — `AssignmentLoopStep` 1.70 → 2.11 ms, `IntegerCollectionSum`
0.87 → 1.08 ms, `RationalDecimalLoop` 4.12 → 4.78 ms, `DerivedReadWriteRead`
6.6 → 7.9 µs — while the other loop rows with the same bytes (`IntegerArithmeticLoop`,
`RealDecimalLoop` +2%, `IntegerArithmeticNestedLoop`) did not; the
executor statement rows (`ActionChain` +9–16%, `StateLoop` +5–13%,
`ExecuteAction` +10%) moved likewise on counts within +2% of 0.10.1. The
earlier candidate's quiet pass, with the same executor code after its
fixes, had these rows within +5–6% or the noise. The bytes those rows do
carry (+5.6% on the loops, #1136's footprint records) are the one
difference in their work; the rest is the machine or the collector, and
this record carries them as time unexplained by allocation rather than
as a regression attributed to a change.

## What this does not measure

- **The two fixes' effect beyond their rows.** #1368 and #1369 were
  written after the six main rounds; their six quiet rounds cover the
  rows the probe showed moving, not every package, and the record's
  single candidate column is `develop` without them.
- **Concurrent analysis.** `ValidateSplit/jobs=8` and `OpenSplit` run on
  8 CPUs; their loaded spreads are the machine's, and only the quiet
  32-satellite rows' time is stated.
- **The intermediate heads.** `d8b60ed62`, `a662c2bce` and `c5eb6cc30`
  are measured only as far as the earlier record and the probes above
  state; none is a release candidate.
