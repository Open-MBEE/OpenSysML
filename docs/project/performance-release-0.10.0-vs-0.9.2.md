# Performance: release 0.10.0 against release 0.9.2

The release-gate measurement of the 0.10.0 candidate — `main` at
`ffe51571c` (2026-10-08, the `release/0.10.0` merge plus a CI-only
hotfix), 1 676 commits past the `v0.9.2` tag (`4226ad9a3`, 2026-10-05) —
against 0.9.2 itself. It follows the method of the
[0.9.2 against 0.9.1](performance-release-0.9.2-vs-0.9.1.md) record: every
benchmark on both revisions, `benchstat` over six runs, the rows the machine
left in doubt re-run interleaved, and whole-binary timings on generated and
real models. See [performance](../internals/performance.md) for what the
figures mean and where the remaining cost is.

0.10.0 is a minor release carrying everything on `develop` since 0.9.2:
exact Rationals and unbounded Integers in the evaluator, the exploration of
every order of a body's unordered statements, executors that interleave move
by move, reflective relationships and indexed connector ends in the semantic
model, incremental re-analysis in the gRPC service, views, code lenses and
inlay hints in the language server, and the fixes of a 272-entry
changelog. The
expectation going in was movement on the execution and analysis paths, and
that is what the measurement finds — it is not a parity record. Every row
that regresses is confirmed by the interleaved re-run, attributed to the
change responsible below, and left for the follow-up work the summary
names; nothing is fixed here.

## Summary

**Nothing here blocks the release.** Every row scales as it did —
nothing superlinear arrived, no path stopped fitting in memory, and the
whole binary validates a 12 000-declaration model in 0.29 s against 0.25 s
and the Apollo 11 model in 0.32 s against 0.29 s, holding 19–26 MiB more.
But 0.10.0 is slower than 0.9.2 on most of what it measures, by constant
factors that are the price of its features in some places and avoidable in
others, and the follow-ups below should land in a 0.10.x patch rather than
wait for 0.11.

Confirmed by the interleaved re-run, with the change responsible:

- **Loading a model costs 16–20% more time, 14–22% more bytes and 17–19%
  more allocations per element** on the connector-dense stress
  constellation (2.5–4.6% on the REPL's attribute-dense fixture), and
  `sysml -validate` is 10–20% slower with 4–26 MiB more resident on
  3 000–12 000-declaration models. The connector, binding and flow ends
  are now owned, journaled, indexed and reflected as metaobjects
  (`00b8499a8`, `da8fba4a3`, `32890be28`, `63552cbc4`). Explained; the
  structural cost of the release.
- **Executing actions and state machines is 20–70% slower per step with
  1.3–2 times the allocations** (`AssignmentLoopStep` +70%, `ActionLoop`
  +36–52%, `StateLoop` +22–27%, `ExecuteActionFreshContext` +30%,
  `RunCalc` +20%). Three changes share it: a body's unordered statements
  are explored in every order (`71fda4cd5`, `807fcf591`), body feature
  bindings are tracked on every write (`a4e1f6ef9`), and control-node
  values are held with the flow that brought them (`a0f77861d`). The
  profile also shows the binding check building its `"action node …"`
  description string on every write that passes — a third of the new
  allocations — which is avoidable. **Follow-up.**
- **Checking a constraint is 16–68% slower** (`BatchConstraints` +68%,
  `Satisfy` +18–26%, `GRPCVerifyConstraint` +21%, `BatchSatisfy` +9.5%):
  ordering performances by outside successions (`72498ad90`) walks the
  succession graph on every check (`successionCycle`, 10% of the time)
  and the exact-arithmetic dispatch is on the evaluation path. The walk
  is memoizable. **Follow-up.**
- **Editing a document in a workspace allocates 2–4 times the bytes it
  did** (`WorkspaceUpdate` +99%, `WorkspaceEdit/vehicle` +86%,
  `WorkspaceEditSmallDocBesideLarge` +293%, `EditBeside` +143–280%),
  growing with the *other* documents in the workspace and +26–43% in time.
  The index snapshots each name's registration before a change so that a
  name registered again as it was is no change (`4ca321508`,
  `fe6e01fb9`); the snapshot copies the re-export claims of every symbol
  the name resolves to, and on an 84-byte edit beside a 96 000-element
  model that is 15–40% of all bytes allocated. **Follow-up.**
- **The Annex A vehicle example now reports 134 warnings**, one per
  member the model's recursive `::**` imports bring in twice
  (`eb4b7583c`, the import-ambiguity rule), where 0.9.2 reported none;
  both binaries still exit 0 with `no errors`. Every path that returns
  the model's diagnostics pays for them: `Analyze/vehicle` +64% time and
  +38% allocations (the suggestion tables the warnings build),
  `GRPCParseFileCached` 22 → 299 allocations (each diagnostic converted to
  protobuf per call), `ConnectParseFileHTTPCached` 3.5× (each serialised),
  `GRPCParseFileUncached` +69%. Whether the rule should fire on a
  recursive import of a namespace that is also reached through another is
  a semantics question for the release notes, not this record; the
  performance follows from the answer. **Follow-up.**
- **Interpreted and compiled calcs pay for exact numerics**:
  `Fib(25)/interpreted` 8.1 → 41 ms (+409%), `Collatz(27)/interpreted`
  +60%, `Collatz(27)/go` +56%, `Hypot(3.0,4.0)/go` 11.5 → 60 ns, and
  `SumTo(1000000)/go` 0.78 → 7.2 ms once its budget is raised. Unbounded
  Integers with an int64 fast path (`79629923b`), exact Rationals
  (`923ec81f0`) and the budget compiled calcs now spend (`d6eafc870`).
  The price of the feature; the interpreted `Fib` is the one row where
  it is steep enough to deserve a profile-driven pass.
- **Instantiating an object is 18–42% slower with 3–276 more
  allocations** (`Instantiate` +18–32%, `InstantiateWarmModel` +42%,
  `FleetInstantiate` +21–39%) and up to 16% fewer bytes: the implicit
  relationship ends an instance now carries (`fa8a7220c`, `ca0e6cca0`).
  Explained.
- `DecodeSnapshot` +19% allocations and `AnalyseResolved` +10%: the
  snapshot holds the owned-relationship journal; the analysis runs the
  three constraint-tier passes added in the interval. Explained.

Three rows the candidate cannot run, reported rather than compared:
`CompiledCalc/SumTo(1000000)` exceeds the default step budget that it fit
in 0.9.2 (a million-iteration loop now needs `OPENSYSML_MAX_STEPS`); the
C target refuses the three calcs that do Integer arithmetic, since a C
`int64` cannot hold an unbounded Integer; and the stress constellation's
`AnalyzeSplitPerDocument` fails because its registry turns on the new
decimal-rounding lint, which the generated model's `0.2` literals trip.
The first two belong in the release notes.

Faster: `HydratePlane` −9% time and −12–31% allocations (indexed
connector ends), `ConvertAPIJSON/32` −15%, `LookupQualified` −15%,
`FleetInstantiate` −7–16% bytes. Unchanged: lexing, parsing, formatting
(on the same input), the migration writer, `ExpandWildcardImports`,
`FeaturesOf`, `ReferencesWarm`, `RenameWarm`, `Diagnostics`,
`REPLEvalExpr`, the empty session's start time.

The binary is 47.00 MiB (+4.12 MiB) and an empty session maps 68 MiB
(+3 MiB).

## Method

Machine: `INTEL(R) XEON(R) PLATINUM 8559C`, 8 CPUs, 31 GiB, Go
`go1.25.11 linux/amd64`, `GOMAXPROCS=8`. 0.9.2 is a worktree at the tag,
built and benchmarked with its own tree; the candidate is `main` at
`ffe51571c`, built and benchmarked with its own. Package benchmarks are
`go test ./<pkg> -run '^$' -bench . -benchmem -count 6` on each revision,
compared with `benchstat` (rows are significant at p < 0.05; `~` is no
significant difference). The first pass ran each package's six counts on
0.9.2 and then on the candidate; rows it left in doubt — a significant
difference, or a spread above 20% on either side — were re-run with the
two revisions interleaved, `-count 1` six times each, nothing else on the
machine. Where the first pass left most of a package in doubt
(`tests/perf`, `tests/stressmodel`) the whole package was re-run that way,
and the re-run is the table shown. Whole-binary figures are
`/usr/bin/time -f '%e %M'` over five interleaved runs of each binary, both
built with `make build-sysml` (`-s -w`, `-trimpath`); medians are reported.
The pilot corpora the `vehicle` rows and the gRPC warm-instantiation row
read were fetched into both trees with `scripts/download-pilot-corpora.sh`
after the first pass, so those rows appear in the re-run only. Causes
were read from `git log v0.9.2..ffe51571c -- <path>` and, where the
history alone did not name one, from `-cpuprofile` and `-memprofile` of
the row on both revisions.

### What is and is not comparable

- **Ten packages define benchmarks on both revisions**; nine produce rows.
  `internal/syntax/parser` defines no benchmark that runs without
  `OPENSYSML_BENCH_MODEL`; neither revision produced a row and the package
  is omitted, as before.
- **Candidate-only rows.** `internal/exec/runtime` gains seven rows for the
  exact numerics and behavior ordering (`IntegerArithmeticLoop`,
  `IntegerArithmeticLoopBeyondInt64`, `IntegerCollectionSum`,
  `RationalDecimalLoop`, `RationalHarmonicLoop`, `RealDecimalLoop`,
  `BehaviorOrdersLibraryModel`); `tests/perf` gains the eight
  `ImportValues` rows of the CSV/TSV/JSON value import. They have no 0.9.2
  counterpart and are listed with their candidate figures, not compared.
- **Rows the candidate cannot run.** In `internal/frontend/repl`,
  `CompiledCalc/SumTo(1000000)` fails on the candidate for the interpreted
  and Go targets (`evaluation step limit exceeded (10000000 steps)`), and
  `Fib`, `SumTo` and `Collatz` for the C target are refused
  (`the interpreter's Integers are unbounded and a C program holds int64`);
  the five rows have 0.9.2 figures and no candidate figure, and are
  discussed with that package. In `tests/stressmodel`, the six
  `AnalyzeSplitPerDocument` rows fail on the candidate because the
  benchmark asserts a clean analysis and the per-document registry now
  reports the decimal literals of the generated model as rounded by their
  `Real` features; they have 0.9.2 figures and no candidate figure.
- **`InstantiateWarmModel`** (`internal/frontend/grpc`) now exists on both
  revisions and is compared; both first-pass runs skipped it for want of
  the vehicle corpus, so its figures are the sequential six-count run made
  after the download.
- **`tests/stressmodel`**: `ConvertAPIJSON/satellites=512` completed one
  count on 0.9.2 (156.0 s) before the run was killed for memory (78 GiB
  allocated per operation on a 31 GiB machine) and five on the candidate
  (118.7–121.9 s); it is excluded as no distribution, as in the previous
  record.
- **Diagnostics**: on every model measured both binaries return the same
  exit status and the same report — `no errors` on the generated models,
  the two example models and all 28 Apollo 11 files. (The candidate's
  decimal-rounding report is an opt-in lint on the command line; only the
  stress benchmark's registry turns it on.)

## Package benchmarks

Time per operation, 0.9.2 then the candidate, from the interleaved re-run
wherever a row was re-run (the first pass agrees in sign and roughly in
size on every row named here); bytes and allocations where they moved.
Allocation counts are deterministic to the unit on both revisions and are
the reading that separates a change in the code from a change in the
machine.

### `internal/exec/runtime`

| benchmark | 0.9.2 | 0.10.0 |
| --------- | ----- | ------ |
| `MaterializePlainAttributes` | 46.8 µs / 42.69 KiB / 599 allocs | 55.3 µs (+18%, p=0.002) / 43.58 KiB (+2.1%) / 618 (+3.2%) |
| `SetFeatureValueNoDependents` | 423 ns | 442 ns (~) |
| `DerivedReadWriteRead` | 5.65 µs ±16% | 6.53 µs ±2% (+16%, p=0.015); allocations identical |
| `AssignmentLoopStep` | 917 µs / 521.4 KiB / 4 923 allocs | 1 563 µs (+70%, p=0.002) / 837.7 KiB (+61%) / 8 527 (+73%) |
| `IntegerArithmeticLoop` | — | 3.18 ms |
| `IntegerArithmeticLoopBeyondInt64` | — | 3.40 ms |
| `IntegerCollectionSum` | — | 897 µs |
| `RationalDecimalLoop` | — | 3.53 ms |
| `RationalHarmonicLoop` | — | 844 µs |
| `RealDecimalLoop` | — | 4.00 ms ±37% |
| `BehaviorOrdersLibraryModel` | — | 328 µs |

As found the first two moved rows were +12% and +66%, and
`MaterializePlainAttributes` was `~` at ±18–21%; interleaved all three are
significant. `AssignmentLoopStep` — one assignment step of an action loop,
the figure the 0.9.1 and 0.9.2 records watched — allocates 3 600 more
objects per step. The profile of the equivalent
`tests/perf` row (`ActionLoop/for1000`, 3 181 → 6 220 allocations) names the
new allocators: `stmtEngine.evalIn` (31% of the objects; the frame chain
built per statement to track body feature bindings, `a4e1f6ef9`),
`actionStmtHost.describe` (17%; the `"action node " + name` string the
binding check receives on every write, built whether or not the check
fails), `stmtEngine.runWithOrder` (15%; the exploration of a body's
statement orders, `71fda4cd5`), and `EvalContext.heldValue` is 39% of
the CPU time (control-node values held with their flow, `a0f77861d`).
`MaterializePlainAttributes` and `DerivedReadWriteRead` carry the
implicit-end constancy (`fa8a7220c`) and the exact-number dispatch.

### `internal/frontend/repl`

28 rows on 0.9.2, 23 on the candidate (the five it cannot run are above);
of the 23, 8 are `~` as found and 15 moved. Interleaved:

| benchmark | 0.9.2 | 0.10.0 |
| --------- | ----- | ------ |
| `LoadModel/elements=0` | 405 µs ±39% / 216.2 KiB / 1 875 allocs | 415 µs (~) / 221.6 KiB (+2.5%) / 1 931 (+3.0%) |
| `LoadModel/elements=250` | 28.5 ms ±38% / 13.22 MiB | 29.0 ms (~) / 13.56 MiB (+2.6%) / allocs +4.4% |
| `LoadModel/elements=1000` | 107 ms ±36% / 52.08 MiB | 115 ms (~) / 53.48 MiB (+2.7%) / allocs +4.5% |
| `LoadModel/elements=4000` | 448 ms ±27% / 207.2 MiB / 1.895 M allocs | 494 ms (~, p=0.240) / 213.6 MiB (+3.1%) / 1.982 M (+4.6%) |
| `RunStateMachine/elements=250, 1000` | 13.1 µs / 15.23 KiB / 213 allocs | 13.7 µs, 13.6 µs (~) / 15.52 KiB (+1.9%) / 217 |
| `RunStateMachine/elements=4000` | 13.1 µs | 13.8 µs ±28% (+5.4%, p=0.009) |
| `RunCalc/elements=250, 1000, 4000` | 4.53–4.57 µs / 4.242 KiB / 76 allocs | 5.44–5.49 µs (+20%, p=0.002 at every size) / 4.453 KiB (+5.0%) / 79 |
| `Instantiate/elements=250` | 6.46 µs / 6.415 KiB / 53 allocs | 8.49 µs (+32%, p=0.002) / 7.105 KiB (+11%) / 56 |
| `Instantiate/elements=1000` | 6.02 µs | 7.09 µs (+18%, p=0.002) / +12% B |
| `Instantiate/elements=4000` | 6.02 µs ±36% | 6.85 µs (~, p=0.093) / +12% B |
| `Diagnostics/attributes=50, 200, 800` | 86.0 µs, 300 µs, 1.17 ms | (~) at every size; +0.1% B |
| `CompiledCalc/Fib(25)/interpreted` | 8.13 ms / 44.04 KiB / 368 allocs | 41.4 ms ±71% (+409%, p=0.002) / 269.6 KiB (+512%) / 2 125 (+477%) |
| `CompiledCalc/Fib(25)/go` | 959 µs ±94% / 6 allocs | 1.71 ms ±5% (~, p=0.065; +77% as found) / 12 allocs |
| `CompiledCalc/Fib(25)/c` | 229 µs | not compilable |
| `CompiledCalc/SumTo(1000000)/interpreted` | 1.55 s | step limit exceeded |
| `CompiledCalc/SumTo(1000000)/go` | 783 µs | step limit exceeded |
| `CompiledCalc/SumTo(1000000)/c` | 385 µs | not compilable |
| `CompiledCalc/Collatz(27)/interpreted` | 290 µs / 29.43 KiB / 1 000 allocs | 465 µs (+60%, p=0.002) / 81.91 KiB (+178%) / 2 115 (+112%) |
| `CompiledCalc/Collatz(27)/go` | 5.43 µs ±25% | 8.49 µs ±6% (+56%, p=0.002); 0 allocs on both |
| `CompiledCalc/Collatz(27)/c` | 1.00 µs | not compilable |
| `CompiledCalc/Hypot(3.0,4.0)/interpreted` | 7.30 µs ±38% / 88 allocs | 8.93 µs (~, p=0.065) / 91 allocs (+3.4%) |
| `CompiledCalc/Hypot(3.0,4.0)/go` | 11.5 ns | 60.3 ns ±27% (+423%, p=0.002); 0 allocs on both |
| `CompiledCalc/Hypot(3.0,4.0)/c` | 13.2 ns | 13.9 ns (+5.0%, p=0.026) |

Three things are going on in this package.

**The model loader costs 2.5–3.1% more bytes and 3–4.6% more
allocations per element**, with the time within the machine's spread
(+6–9% as found, `~` interleaved at every size). This is the semantic
model's new bookkeeping — owned relationships journaled and reflected as
metaobjects (`da8fba4a3`, `00b8499a8`, `32890be28`), the classification of
extended declarations (`ca0e6cca0`) and the implicit-end constancy derived
once for reflection and writes (`fa8a7220c`) — and the same per-element
growth the stress constellation and `tests/perf` show below. **Explained**;
it is the fourth record running in which the loader grows by a few percent
per release.

**Instantiating one object and evaluating one calc are 18–32% and 20%
slower**, three allocations more per operation each. The gRPC
`InstantiateWarmModel` row below shows the same +42% with +18%
allocations. An instance now carries the
implicit relationship ends of its type (`fa8a7220c`, `ca0e6cca0`) and
its attribute values go through the exact-number constructors
(`79629923b`, `923ec81f0`); the bytes rise 11–26% on these small objects
and fall on the stress constellation's fleet, where the Integer and
Rational fast paths' allocation-free representation (`d1eb16edb`) wins.

**The interpreted and compiled calcs pay for unbounded Integers, exact
Rationals and the step budget.** Each of the four fixtures is affected
differently, and the benchmark's own fixture file grew from 597 to 930
lines (the same four calcs; the new content is other compiled fixtures),
which the `LoadModel`-sized setup absorbs but the `FormatEdits` row in
`internal/frontend/lsp` does not:

- `Fib(25)/interpreted` is five times slower and allocates six times the
  bytes and 5.8 times the objects. The profile shows where the time went:
`compiledCalc.invoke`, `callNode`, `conditionalNode` and `leafPair`
(the interpreted target is the closure-compiled calc evaluator) and
`scalarCheck.held` at 11% flat — the exact-Rational evaluation
(`923ec81f0`) checks the kind of every scalar a node yields and pairs the
leaves of each arithmetic node, 250 000 calls deep. Allocation counts rose
5.8× with it (the `big`-free fast path still allocates a result per
call), and the candidate's ±71% spread is the collector. The price of
exactness, but the steepest row in the record and the one a
profile-driven pass should look at first.
- `Collatz(27)/interpreted` is +60% with 2.1 times the allocations;
  `Collatz(27)/go` is +56% with none on either side, and
  `Hypot(3.0,4.0)/go` goes from 11.5 ns to 60 ns, also allocation-free.
  The Go target's compiled values now
keep an Integer an Integer inside a Real-typed value and hold negative
powers as exact Rationals (`b69ea6840`, `0aa4aca55`), so each arithmetic
node carries its number kind and branches on it; `Hypot`'s four
operations went from 11.5 ns to 60 ns for it, still allocation-free, and
compiled calcs spend the interpreter's step budget (`d6eafc870`).
- `SumTo(1000000)` sums a million integers in a loop. On 0.9.2 the
  interpreted run took 1.55 s inside the default budget of 10 000 000
  steps; on the candidate the same loop exceeds it, for the interpreted
  and the Go target both, because compiled calcs now spend the
  interpreter's step budget (`d6eafc870`) and the interpreter counts the
  statements of a loop body it explores in order (`71fda4cd5`,
  `43548d741`). The row is not a timing regression but a change in what
  fits the default budget: a loop of a million iterations no longer does
  without `OPENSYSML_MAX_STEPS`.
- The C target refuses every calc that does Integer arithmetic
  (`Fib`, `SumTo`, `Collatz`) because a C `int64` cannot hold the
  interpreter's unbounded Integers (`79629923b`); only `Hypot`, on Reals,
  still compiles to C. This is a deliberate narrowing of the C target, not
  a performance figure, and belongs in the release notes.

### `internal/frontend/grpc`

| benchmark | 0.9.2 | 0.10.0 |
| --------- | ----- | ------ |
| `ParseFileColdShared` | 10.7 ms ±34% / 4.689 MiB / 31.12 k allocs | 7.67 ms ±21% (~) / 4.700 MiB (+0.2%) / 31.29 k (+0.5%) |
| `ParseFileColdInline` | 7.69 ms ±29% / 4.916 MiB / 31.47 k | 7.74 ms (~) / 4.932 MiB (~) / 31.69 k (+0.7%) |
| `InstantiateWarmModel` | 134 µs ±8% / 82.87 KiB / 1 555 allocs | 191 µs ±1% (+42%, p=0.002) / 104.13 KiB (+26%) / 1 831 (+18%) |

The cold-parse rows are the parser and the loader on the vehicle example
and move with the loader's few percent of allocations. The warm
instantiation is the `Instantiate` row of the REPL at the service's
scale: 276 more objects per instantiation. See the `Instantiate` cause under the
REPL: the same implicit relationship ends and exact-number constructors,
on the vehicle example's objects.

### `internal/frontend/lsp`

| benchmark | 0.9.2 | 0.10.0 |
| --------- | ----- | ------ |
| `FormatEdits` | 515 µs ±11% / 433.7 KiB / 78 allocs | 871 µs ±18% (+69%, p=0.002) / 704.3 KiB (+62%) / 85 |
| `SpanToRangeLarge` | 2.99 ms | 2.87 ms (~) |
| `ReferencesWarm` | 206 µs | 211 µs (~) |
| `ReferencesCold` | 45.4 ms ±19% | 37.9 ms (~, p=0.065) |
| `RenameWarm` | 20.0 ms ±76% | 18.9 ms ±28% (~; −14.5% as found) |
| `WorkspaceUpdate` | 198 ms ±100% / 49.39 MiB / 605.6 k allocs | 270 ms ±16% (~, p=0.065) / 98.46 MiB (+99%) / 930.6 k (+54%) |

`FormatEdits` formats `internal/frontend/repl/testdata/compile_calcs.sysml`,
which grew from 597 to 930 lines on the candidate with the new compiled
fixtures; the bytes move by the ratio of the file sizes (+62% against
+56% more lines) and the formatter itself has one commit in the interval
(the indexed connector ends, `63552cbc4`). **Not a regression in the
code.**

`WorkspaceUpdate` — one edit to one of the workspace's documents, with the
re-analysis the server does for it — allocates twice the bytes and 1.5
times the objects it did, and the time is within the machine's spread
only because the baseline's six counts ran 100% apart. The profile puts 39% of the bytes
(810 MB over the run) and 30% of the objects in one function new since
0.9.2, `symbols.registrationOf`, reached from `Index.noteBefore`: before
a name's registration changes, the index snapshots what a lookup of it
would read — the symbols under it, their re-export and hidden flags, and
a copy of every document's re-export claims and routes — so that a name
registered again exactly as it was does not count as a change
(`4ca321508`, `fe6e01fb9`). The snapshot is per name touched by the
re-registration, which on a workspace update is every name of the
document and of what imports it, and the claim maps are deep-copied. The
intent is the right one (fewer downstream invalidations); the copy is
avoidable. **Follow-up.** The rest of the growth is the resolver's new
`checkImportedNames` (`eb4b7583c`).

### `internal/translate/migrate`, `internal/workspace/libs`, `internal/workspace/model`

| benchmark | 0.9.2 | 0.10.0 |
| --------- | ----- | ------ |
| `migrate WriterSiblingBlocks` | 1.93 ms | 1.87 ms (~); bytes and allocations identical |
| `libs ExpandWildcardImports` | 26.8 ms | 27.8 ms (~) |
| `libs IndexLibrary` | 13.8 ms / 13.56 MiB / 104.6 k allocs | 14.1 ms (~; +5.1% as found) / 14.07 MiB (+3.8%) / 108.2 k (+3.4%) |
| `libs ExpandModelImports` | 6.36 ms | 6.50 ms (~) |
| `libs DecodeSnapshot` | 9.52 ms / 35.32 MiB / 56.63 k allocs | 9.98 ms (+4.9%, p=0.009) / 37.85 MiB (+7.2%) / 67.45 k (+19%) |
| `libs SetDigest` | 629 µs | 624 µs (~) |
| `model AnalyseUnresolved` | 20.2 ms | 19.9 ms (~) |
| `model AnalyseResolved` | 1.26 ms / 543.0 KiB / 5 049 allocs | 1.39 ms (+10%, p=0.002) / 590.4 KiB (+8.7%) / 5 971 (+18%) |

The migration writer is unchanged. Indexing the standard library
allocates 3.4% more objects (the reflective relationship ends, the
`StateActivity` extension library `0188fb308`), and decoding its snapshot
19% more: the snapshot carries the owned-relationship journal and the
reflected implicit relationships (`da8fba4a3`, `32890be28`) that the
0.9.2 snapshot did not hold. Analysing a resolved document is +10% with
18% more allocations: the constraint-tier passes added in the interval —
the indexed-end multiplicity check (`cf85beb36`), the import-ambiguity
warning (`eb4b7583c`), the reflective derivations for the batch
constraints (`70bfa8ee2`) — each walk the document once more. The
`tests/perf` `Analyze` rows below show the same +4.7% allocations on the
synthetic model and much more on the vehicle example.

### `tests/perf`

37 rows on 0.9.2 and 45 on the candidate as found (the eight `ImportValues`
rows are candidate-only); 48 and 56 once the vehicle corpus was present,
and the whole package was re-run interleaved. Of the 48 compared rows 20
are `~` in time; the rows that moved, and the rows whose allocations moved:

| benchmark | 0.9.2 | 0.10.0 |
| --------- | ----- | ------ |
| `Lex`, `Parse` (synthetic, vehicle) | | (~) time; bytes and allocations identical |
| `IndexAddExpand/synthetic` | 42.5 ms ±41% / 39.70 MiB / 290.8 k allocs | 44.9 ms (~) / 42.45 MiB (+6.9%) / 315.5 k (+8.5%) |
| `IndexAddOnly/synthetic` | 39.7 ms / 37.59 MiB / 281.2 k | 41.0 ms (~) / 40.34 MiB (+7.3%) / 305.9 k (+8.8%) |
| `IndexAddOnly/vehicle` | 1.07 ms / 1.091 MiB / 9 276 | 1.06 ms (~) / 1.138 MiB (+4.3%) / 9 632 (+3.8%) |
| `Analyze/synthetic` | 1.13 s ±17% / 466.1 MiB / 4.435 M | 1.24 s ±29% (+9.2%, p=0.026) / 487.1 MiB (+4.5%) / 4.644 M (+4.7%) |
| `Analyze/vehicle` | 118 ms ±20% / 73.64 MiB / 627.5 k | 193 ms ±44% (+64%, p=0.002) / 130.0 MiB (+77%) / 865.1 k (+38%) |
| `WorkspaceEdit/synthetic` | 204 ms ±74% / 77.98 MiB / 596.3 k | 256 ms ±42% (~) / 85.73 MiB (+9.9%) / 680.6 k (+14%) |
| `WorkspaceEdit/synthetic/reindex+diagnostics` | 1.52 s ±60% / 493.2 MiB / 4.787 M | 1.62 s (~) / 519.7 MiB (+5.4%) / 5.090 M (+6.3%) |
| `WorkspaceEdit/vehicle` | 52.0 ms / 12.33 MiB / 139.1 k | 66.0 ms (+27%, p=0.002) / 22.95 MiB (+86%) / 214.4 k (+54%) |
| `WorkspaceEdit/vehicle/reindex+diagnostics` | 167 ms / 71.25 MiB / 651.7 k | 223 ms ±27% (+34%, p=0.002) / 129.8 MiB (+82%) / 917.6 k (+41%) |
| `WorkspaceEditSmallDocBesideLarge` | 42.4 ms ±51% / 7.952 MiB / 81.74 k | 60.8 ms ±64% (+43%, p=0.041) / 31.24 MiB (+293%) / 162.6 k (+99%) |
| `FQNOf` | 1.37 ms ±23% / 36 000 symbols | 1.52 ms (~) / 40 000 symbols (+11%); 38.0 ns against 38.1 ns per symbol (~) |
| `LookupQualified` | 1.39 ms ±18% | 1.18 ms (−15%, p=0.041) |
| `FeaturesOf` | 931 ms ±55% | 945 ms (~); bytes and allocations identical |
| `REPLLoadFile` | 1.32 s ±26% / 595.5 MiB / 5.383 M | 1.48 s (~, p=0.065) / 616.5 MiB (+3.5%) / 5.595 M (+4.0%) |
| `REPLSubmitSnippet` | 1.03 s / 513.6 MiB / 4.841 M | 1.11 s (+7.6%, p=0.015) / 545.7 MiB (+6.3%) / 5.048 M (+4.3%) |
| `REPLEvalExpr` | 4.50 ms / 101 allocs | 4.46 ms (~) / 104 |
| `LowerActionGraph` | 1.73 µs / 2.625 KiB / 25 allocs | 2.01 µs (+16%, p=0.002) / 2.906 KiB (+11%) / 27 |
| `LowerStateGraph` | 8.30 µs | 8.08 µs (~); bytes −0.5% |
| `LowerChain/chain10, 100, 1000` | 8.44 µs, 145 µs, 8.94 ms | 9.53 µs (+13%), 150 µs (~), 8.55 ms (~); bytes +13–15% |
| `ExecuteAction` | 75.3 µs / 192 allocs | 78.9 µs (+4.9%, p=0.004) / 253 (+32%) |
| `ExecuteActionFreshContext` | 30.4 µs ±22% / 26.26 KiB / 268 | 39.5 µs ±4% (+30%, p=0.002) / 34.94 KiB (+33%) / 347 (+29%) |
| `ExecuteState` | 704 µs / 483.4 KiB / 10.38 k | 838 µs (+19%, p=0.002) / 608.2 KiB (+26%) / 11.66 k (+12%) |
| `ActionLoop/for10` | 25.0 µs / 20.85 KiB / 204 | 33.9 µs (+36%) / 28.37 KiB (+36%) / 273 (+34%) |
| `ActionLoop/for100` | 101 µs / 47.97 KiB / 477 | 152 µs (+50%) / 70.26 KiB (+46%) / 816 (+71%) |
| `ActionLoop/for1000` | 850 µs / 279.0 KiB / 3 181 | 1 290 µs (+52%) / 449.2 KiB (+61%) / 6 220 (+96%) |
| `ActionChain/chain10` | 88.6 µs / 675 allocs | 101 µs (+14%) / 784 (+16%) |
| `ActionChain/chain100` | 562 µs / 522.8 KiB / 5 757 | 696 µs (+24%) / 705.9 KiB (+35%) / 6 595 (+15%) |
| `ActionChain/chain1000` | 13.0 ms / 5.153 MiB / 56.28 k | 14.5 ms (+11%) / 7.077 MiB (+37%) / 64.77 k (+15%) |
| `StateLoop/count50, 500, 5000` | 653 µs, 6.23 ms, 61.6 ms | 798 µs (+22%), 7.92 ms (+27%), 78.4 ms (+27%); bytes +26%, allocs +12% at every size |
| `BatchConstraints` | 8.97 ms ±31% / 2.223 MiB / 55.19 k | 15.1 ms ±11% (+68%, p=0.002) / 2.387 MiB (+7.4%) / 55.86 k (+1.2%) |
| `SameConstraintManyInstances` | 3.56 µs / 1 016 B | 4.12 µs (+16%, p=0.002) / 1 000 B (−1.6%) |
| `BatchSatisfy` | 1.18 s / 417.0 MiB / 4.211 M | 1.29 s (+9.5%, p=0.009) / 427.2 MiB (+2.4%) / 4.357 M (+3.5%) |
| `Instantiate` | 508 ms ±102% / 173.9 MiB ±57% | 529 ms ±6% (~) / 180.2 MiB (~, p=0.065) |
| `GRPCParseFileCached` | 58.8 µs / 216.7 KiB / 22 allocs | 69.7 µs (+19%, p=0.002) / 242.1 KiB (+12%) / 299 (13.6×) |
| `GRPCParseFileUncached` | 124 ms ±31% / 81.41 MiB / 660.5 k | 210 ms ±5% (+69%, p=0.002) / 142.1 MiB (+74%) / 905.1 k (+37%) |
| `GRPCEvaluate` | 8.97 µs / 8.426 KiB / 110 | 9.75 µs (+8.7%, p=0.009) / 9.406 KiB (+12%) / 116 |
| `GRPCVerifyConstraint` | 795 µs / 510.6 KiB / 9 188 | 958 µs (+21%, p=0.002) / 541.8 KiB (+6.1%) / 9 768 (+6.3%) |
| `ConnectEvaluateHTTP` | 150 µs / 201 allocs | 153 µs (~) / 206 |
| `ConnectParseFileHTTPCached` | 251 µs / 352.7 KiB / 166 allocs | 882 µs (+252%, p=0.002) / 488.3 KiB (+38%) / 1 292 (7.8×) |
| `ImportValues/flat/parts=100, 500, 2000 (attrs=3)` | — | 21.3 ms, 104 ms, 548 ms; 300, 1 500, 6 000 values |
| `ImportValues/flat/parts=2000/attrs=5` | — | 844 ms ±37%; 10 000 values |
| `ImportValues/nested/parts=100, 500, 2000 (attrs=3)` | — | 29.4 ms, 141 ms, 620 ms |
| `ImportValues/nested/parts=2000/attrs=5` | — | 847 ms |
| geomean (compared rows) | | +18% time |

Four groups, each with a cause of its own:

1. **Execution.** Every row that runs an action or state machine is
   slower by a constant factor per step: an action loop iteration +36–52%
   with 1.3–2 times the allocations, a state-machine event +19–27% with
   +12% allocations, a succession chain +11–24%, a fresh action context
   +30%, and `ExecuteAction` +4.9% with 32% more allocations. The
   `internal/exec/runtime` rows above (`AssignmentLoopStep` +70%) and the
   REPL's `RunCalc` (+20%) are the same cost. The runtime profile (under
`internal/exec/runtime` above) splits the new work three ways: the
frames built per statement to track body feature bindings
(`a4e1f6ef9`), the exploration of a body's statement orders
(`71fda4cd5`, `807fcf591`, `c8891457a`), and control-node values held
with their flow (`a0f77861d`); the description string the binding check
builds on every passing write is the avoidable part. `LowerActionGraph`
+16% and `LowerChain/chain10` +13% are the lowering carrying the
succession orders the executor explores.
2. **Constraint checking.** `BatchConstraints` is +68% per check with
   +1.2% allocations, `SameConstraintManyInstances` +16% with fewer bytes,
   `GRPCVerifyConstraint` +21%, `BatchSatisfy` +9.5%. The profile of `BatchConstraints`
puts 10% of the candidate's CPU in `Context.successionCycle`, a function
that did not exist: ordering performances by outside successions
(`72498ad90`) walks the succession graph when a constraint's body is
evaluated, on every check, for a model whose successions do not change
between checks. The remainder is the exact-number dispatch on the
evaluation path (`923ec81f0`, `79629923b`) and the reflective
derivations the constraint passes read (`70bfa8ee2`). The walk is
memoizable per model. **Follow-up.**
3. **The gRPC parse path.** `ParseFile` on a document the service has
   already parsed allocated 22 objects on 0.9.2 and allocates 299 on the
   candidate (+19% time); through Connect it is 3.5 times slower, and an
   uncached parse is +69% time and +74% bytes. The service's `ParseSources`
   now names the documents an edit reached and re-analyzes only those
   (`fcc123b40`, `2d921fae9`, `af2604ba8`): the cached path that used to
   return a stored result now computes the affected set, and the uncached
   path carries the lineage bookkeeping. That is not where the allocations are,
though. The profile puts 70% of the cached path's objects in
`DiagnosticToProto` and `sourceSpanToProto`, called from
`modelDiagnostics`: the vehicle example now carries 134 warnings
(below), and each `ParseFile` converts every one of them to protobuf
again — the 22 → 299 allocations are 2 per warning plus the response.
The service reports the parser's and resolver's warnings as the
workspace does (`71d4bacea`), which it did not in 0.9.2; the cost is the
number of warnings times the number of calls, and would be gone with
either a cached conversion or no warnings on this model.
4. **Workspace edits and analysis.** Editing a document beside others
   allocates 1.9–3.9 times the bytes it did (`WorkspaceEdit/vehicle` +86%,
   `WorkspaceEditSmallDocBesideLarge` +293%; the LSP's `WorkspaceUpdate`
   +99% and the stress constellation's `EditBeside` +143–280% are the same
   path), and analysing the vehicle example — a model dense in connectors,
   bindings, flows and decimal literals — is +64% time and +38%
   allocations against +4.7% on the synthetic model. Two causes. The bytes are
`symbols.registrationOf` (see `WorkspaceUpdate` above): the stress
`EditBeside` profile puts 15% of the bytes and 13% of the objects there
on a 32-satellite constellation, and the share grows with the model
beside the edit (`EditBeside` +143% → +280% bytes from 32 to 512
satellites). The vehicle rows have a second cause: 0.9.2 validated the
Annex A `SimpleVehicleModel` with no diagnostic, and the candidate
reports 134 warnings, `Duplicate of imported member name '…'`, one per
member that `VehicleConfiguration_b::**` and
`VehicleConfiguration_b::PartsTree::**` both bring into the same
namespace (`eb4b7583c`, the import-ambiguity rule). The profile of
`Analyze/vehicle` shows the rule's `checkImportedNames` (21% of bytes)
and the `suggest.EditDistance` tables each diagnostic builds (12% of
objects) as the whole of the growth; the synthetic model, which has no
such imports, grows 4.7%. Whether a recursive import should warn about
a member it also reaches through a nested recursive import is for the
semantics to decide; until it does, every tool that analyses this model
pays 134 diagnostics' worth of work per analysis, and every gRPC client
receives them.

The import of values from CSV, TSV and JSON files (`ImportValues`) is new
in 0.10.0; 2 000 parts take 0.55–0.85 s flat and 0.62–0.85 s nested, which
is 90–280 µs per imported value. The first record to carry these rows,
they are the baseline for the next.

### `tests/stressmodel`

Generated satellite constellations at 32, 128 and 512 satellites (6 227,
24 167 and 95 927 elements), one document each; the first pass left every
time row in doubt (0.9.2's counts ran 10–60% apart), so the whole package
was re-run interleaved and the re-run is shown. 36 rows compared; six
`AnalyzeSplitPerDocument` rows are 0.9.2-only (above) and
`ConvertAPIJSON` has no distribution on either side.

| benchmark | 0.9.2 | 0.10.0 |
| --------- | ----- | ------ |
| `Load/satellites=32` | 346 ms / 132.7 MiB / 1.994 M allocs | 415 ms (+20%) / 162.1 MiB (+22%) / 2.365 M (+19%) |
| `Load/satellites=128` | 1.38 s / 501.3 MiB / 7.682 M | 1.60 s (+16%) / 579.1 MiB (+16%) / 9.040 M (+18%) |
| `Load/satellites=512` | 6.11 s / 1.934 GiB / 30.43 M | 7.12 s (+16%) / 2.208 GiB (+14%) / 35.74 M (+17%) |
| `Satisfy/satellites=32` (96 assertions) | 4.10 ms / 1.665 MiB / 28.90 k | 4.83 ms (+18%) / 1.831 MiB (+10%) / 34.57 k (+20%) |
| `Satisfy/satellites=128` (384) | 17.3 ms / 6.652 MiB | 20.7 ms (+20%) / 7.316 MiB (+10%) / +20% allocs |
| `Satisfy/satellites=512` (1 536) | 82.8 ms ±7% / 26.62 MiB | 104 ms ±7% (+26%) / 29.27 MiB (+10%) / +20% allocs |
| `FleetInstantiate/satellites=32` (1 179 elements) | 4.06 ms / 7.987 MiB / 12.72 k | 4.94 ms (+21%) / 7.417 MiB (−7.1%) / 13.96 k (+9.7%) |
| `FleetInstantiate/satellites=128` (1 715) | 11.4 ms / 24.10 MiB / 48.37 k | 14.6 ms (+29%) / 24.45 MiB (+1.4%) / 52.97 k (+9.5%) |
| `FleetInstantiate/satellites=512` (3 947) | 59.8 ms / 160.2 MiB ±7% / 190.9 k | 82.9 ms ±9% (+39%) / 134.1 MiB (−16%) / 209.0 k (+9.5%) |
| `FleetSatisfy/satellites=32, 128, 512` | 1.01 ms, 1.60 ms, 5.06 ms | 1.19 ms (+18%), 1.88 ms (+17%), 6.30 ms (+24%); +8.8–9.2% B, +17–18% allocs |
| `EditBeside/satellites=32` | 1.05 ms / 295.5 KiB / 3 442 | 1.32 ms (+26%) / 719.5 KiB (+143%) / 5 533 (+61%) |
| `EditBeside/satellites=128` | 3.21 ms / 719.9 KiB / 8 377 | 4.21 ms (+31%) / 2 435.7 KiB (+238%) / 15 922 (+90%) |
| `EditBeside/satellites=512` | 12.9 ms / 2.408 MiB / 28.11 k | 16.9 ms (+31%) / 9.147 MiB (+280%) / 57.44 k (+104%) |
| `ValidateSplit/satellites=32/files=6/jobs=1` | 508 ms / 186.1 MiB / 2.480 M | 572 ms (+13%) / 217.5 MiB (+17%) / 2.898 M (+17%) |
| `ValidateSplit/satellites=32/files=6/jobs=8` | 236 ms | 253 ms (+7.0%) / +17% B / +17% allocs |
| `ValidateSplit/satellites=128/files=6/jobs=1, 8` | 1.79 s, 737 ms | 2.04 s (+14%), 840 ms ±9% (+14%) / +15% B / +17% allocs |
| `ValidateSplit/satellites=512/files=6/jobs=1` | 7.39 s ±18% / 2.335 GiB / 33.69 M | 8.34 s ±2% (~, p=0.065) / 2.646 GiB (+13%) / 39.59 M (+18%) |
| `ValidateSplit/satellites=512/files=6/jobs=8` | 2.90 s ±18% | 3.48 s ±2% (+20%, p=0.004) / +13% B / +18% allocs |
| `LoadFiles/satellites=32/files=6` | 435 ms ±20% / 147.7 MiB / 1.971 M | 508 ms ±1% (~, p=0.065) / 176.8 MiB (+20%) / 2.365 M (+20%) |
| `LoadFiles/satellites=128, 512/files=6` | 1.62 s, 6.66 s ±10% | 1.85 s (+14%), 7.69 s (+16%) / +16–18% B / +21% allocs |
| `EditImported/satellites=32, 128, 512/files=6` | 473 ms ±43%, 1.60 s ±44%, 6.54 s ±36% | 532 ms ±2%, 1.82 s ±3%, 7.45 s ±4% (all ~) / +19–30% B / +22–25% allocs |
| `OpenSplit/satellites=32/files=6/cold` | 228 ms ±11% / 169.1 MiB / 2.362 M | 239 ms ±8% (+4.8%, p=0.041) / 199.9 MiB (+18%) / 2.777 M (+18%) |
| `OpenSplit/satellites=32/files=6/warm` | 96.6 ms ±13% / 65.62 MiB / 571.1 k | 122 ms ±39% (+27%) / 86.03 MiB (+31%) / 706.8 k (+24%) |
| `OpenSplit/satellites=128/files=6/cold` | 693 ms ±13% / 541.3 MiB | 804 ms ±6% (+16%, p=0.009) / 632.2 MiB (+17%) / +18% allocs |
| `OpenSplit/satellites=128/files=6/warm` | 180 ms ±7% / 166.6 MiB / 1.385 M | 250 ms ±45% (+39%) / 215.2 MiB (+29%) / 1.730 M (+25%) |
| `OpenSplit/satellites=512/files=6/cold` | 2.70 s ±5% / 2.042 GiB | 3.35 s ±17% (+24%, p=0.009) / 2.355 GiB (+15%) / +19% allocs |
| `OpenSplit/satellites=512/files=6/warm` | 586 ms ±14% / 613.4 MiB / 4.639 M | 849 ms ±113% (+45%) / 897.7 MiB (+46%) / 5.821 M (+25%) |
| `HydratePlane/satellites=32/files=6` | 85.5 ms ±9% / 25.80 MiB / 314.8 k | 77.7 ms ±10% (−9.1%, p=0.015) / 25.67 MiB (−0.5%) / 278.5 k (−12%) |
| `HydratePlane/satellites=128/files=6` | 224 ms ±23% / 76.61 MiB / 919.8 k | 202 ms ±19% (~) / 62.71 MiB (−18%) / 680.3 k (−26%) |
| `HydratePlane/satellites=512/files=6` | 932 ms ±3% / 287.2 MiB / 3.336 M | 767 ms ±64% (~, p=0.394; −37% as found) / 214.2 MiB (−25%) / 2.287 M (−31%) |
| `ConvertAPIJSON/satellites=32` (as found) | 7.86 s ±4% / 5.021 GiB / 40.91 M | 6.69 s ±4% (−15%) / 5.156 GiB (+2.7%) / 44.96 M (+9.9%) |
| `ConvertAPIJSON/satellites=128` (as found) | 31.9 s ±4% / 19.69 GiB / 157.8 M | 31.2 s ±2% (~) / 20.22 GiB (+2.7%) / 173.4 M (+9.9%) |
| `ConvertAPIJSON/satellites=512` | 156 s, one count, then killed | 119–122 s, five counts; no distribution on either side |
| `AnalyzeSplitPerDocument/satellites=32, 128, 512/files=6/jobs=1, 8` | 2.75 ms, 11.6 ms, 50.4 ms (jobs=1); 1.23 ms, 2.26 ms, 10.7 ms (jobs=8) | fails: the generated model's decimal literals are reported as rounded |
| geomean (36 compared rows) | 167 ms / 78.87 MiB / 911.8 k | 195 ms (+17%) / 95.95 MiB (+22%) / 1.081 M (+19%) |

Unless marked, every time row is p=0.002 over six interleaved counts;
every byte and allocation row is p=0.002 with 0% spread.

The constellation says in one place what the packages say severally:

- **Loading is +16–20% time, +14–22% bytes and +17–19% allocations per
  element**, at every size, in every path that loads (`Load`,
  `LoadFiles`, `ValidateSplit`, `OpenSplit/cold`, `EditImported`'s bytes).
  This is more than the REPL's `LoadModel` (+2.5–3.1% bytes) because the
  constellation is nothing but parts, ports, connections and flows between
  them, and it is the connector, binding and flow ends whose
  representation changed: their ends are owned by the connector and
  journaled as owned relationships (`00b8499a8`, `da8fba4a3`), inherited
  ends are replaced positionally, flows are paired by end, each end may
  carry an index (`63552cbc4`, `7514a97bf`), and every implicit
  relationship the resolver derives is reflected as a metaobject
  (`32890be28`). A model of 95 927 elements holds 5.3 million more objects
  for it and takes a second longer to load. **Explained**, and the largest
  single cost of the release.
- **Satisfying requirements is +18–26% time, +10% bytes and +20%
  allocations per assertion**, the same as `tests/perf`'s
  `GRPCVerifyConstraint` (+21%) and `BatchConstraints`; the fleet
  variant +17–24%. See the constraint-checking cause under `tests/perf`.
- **Instantiating a fleet is +21–39% time with 9.5% more allocations and
  up to 16% fewer bytes**: the same `Instantiate` cost as the REPL and
  gRPC rows (+18–42%), with the bytes going the other way because the
  instantiated objects' attribute storage is smaller (the
  allocation-free Integer and Rational fast paths, `d1eb16edb`).
- **Editing a small document beside the constellation is +26–31% time
  and 2.4–3.8 times the bytes**, growing with the size of the document
  beside it — the edited document is 84 bytes and does not change. The
  bytes scale with the *other* document: the workspace's re-analysis after
  an edit now re-derives something proportional to the whole workspace.
  See the workspace-edit cause under `tests/perf`.
- **Opening a workspace warm** (the documents already analysed once) is
  +27–45% with spreads of 39–113% on the candidate, the only rows whose
  spread the re-run did not settle; the bytes (+29–46%) are the loader's
  growth plus the edit path's.
- **Hydrating a plane is 9–37% faster with 12–31% fewer allocations** —
  the one improvement at scale, from the indexed connector ends: the
  hydrator finds an end by index rather than by scanning.
- **Converting to API JSON is 15% faster at 32 satellites** with 9.9% more
  allocations (`fcc123b40`'s affected-set bookkeeping applies here too),
  unchanged at 128; at 512 the first-pass run was killed on 0.9.2 after
  its second count (78 GiB allocated per operation) and the candidate's
  five counts of 119–122 s are reported for what they are.

## Whole-binary scaling

`sysml -validate` over generated compliant models (the satellite-network
generator at ≈3 000, 6 000 and 12 000 declarations: 3 804, 7 260 and
14 172 lines) that report `no errors`, five runs each with the two
binaries interleaved; median wall seconds and median maximum resident set.

| declarations | 0.9.2 | 0.10.0 |
| ------------ | ----- | ------ |
| ≈3 000 | 0.10 s / 97 MiB | 0.12 s / 101 MiB |
| ≈6 000 | 0.16 s / 109 MiB | 0.17 s / 115 MiB |
| ≈12 000 | 0.25 s / 132 MiB | 0.29 s / 158 MiB |

Loading and validating a model from the command line is 10–20% slower
and holds 4–26 MiB more at the end, growing with the model: the loader's
per-element bytes and the analysis passes above, seen whole. At 12 000
declarations that is 40 ms and 26 MiB; the figures scale linearly on both
binaries (0.10→0.25 s and 0.12→0.29 s from 3 000 to 12 000), so nothing
superlinear arrived with the release.

### Process start and session floor

| figure | 0.9.2 | 0.10.0 |
| ------ | ----- | ------ |
| binary size | 42.88 MiB (44 957 880 B) | 47.00 MiB (49 275 064 B; +4.12 MiB) |
| empty session (`sysml -e 1`), median of 5 | 0.02 s / 65 MiB | 0.02 s / 68 MiB |
| empty-session load (`LoadModel/elements=0`) | 405 µs / 216.2 KiB | 415 µs (~) / 221.6 KiB (+2.5%) |

The binary grows 4.12 MiB (+9.6%) with the release's new surfaces — the
views and their D2, Mermaid, DOT and PlantUML renderers, the graphs
export, the api-json loader, the value import, the browser engine's
shared code, the `big.Int` and `big.Rat` arithmetic — and the empty
session maps 3 MiB more for it; its start time is unchanged at the clock's
resolution.

### Example models and the Apollo 11 model

Median wall time and maximum resident set of five interleaved runs; every
run returned the same exit status and report on both binaries (the
candidate tree's copies of the two example files were given to both). The
Apollo 11 model is the public
[Apollo 11 SysML v2 model](https://github.com/airbus/apollo-11-sysml-v2)
(commit `6e9c93f`, 28 files, 7 221 lines).

| model / command | 0.9.2 | 0.10.0 |
| --------------- | ----- | ------ |
| `action-executor-demo.sysml -validate` | 0.02 s / 66 MiB | 0.03 s / 70 MiB |
| `disposal-robot-demo/robot.sysml -validate` | 0.04 s / 72 MiB | 0.04 s / 78 MiB |
| Apollo 11, `-validate` all 28 files | 0.29 s / 150 MiB | 0.32 s / 169 MiB |

The Apollo 11 model loads and validates in 0.32 s against 0.29 s (+10%,
three clock ticks) and holds 19 MiB more, in line with the generated
models of its size.

## Re-runs

The first pass left 71 of the 190 compared rows in doubt: 48 by a
significant difference, 23 by a spread above 20% on one side with no
significant difference (`WorkspaceUpdate` ±100%, `Fib(25)/go` ±94%,
`Instantiate` ±102%, most of the stress constellation's time rows). All
were re-run interleaved — `-count 1` of the row on 0.9.2, then on the
candidate, six times — and the `tests/perf` and `tests/stressmodel`
packages whole, since most of each was in doubt. The pilot corpora were
downloaded between the first pass and the re-run, so the `vehicle` rows
and `InstantiateWarmModel` have one sequential six-count run each
(reported above) and no first pass.

What the re-run changed:

- **Nothing changed sign.** Every row significant in the first pass is
  significant in the re-run in the same direction; the stress
  constellation's time regressions shrank from the first pass's +39–87%
  to +16–45% once the two revisions ran under the same conditions, and
  that is the figure reported. The first pass had run all of 0.9.2's
  counts first and the candidate's after; the re-run shows that about
  half of the first-pass difference on the long rows was the machine.
- **Cleared:** `RenameWarm` (−14.5% as found, `~`), `ReferencesCold`
  (`~`), `HydratePlane/128, 512` (−10%, −37% as found; `~` in time, the
  byte and allocation gains stand), `WorkspaceUpdate` (+36% as found;
  `~` in time at ±16%, the +99% bytes stand), `Fib(25)/go` (+77% as
  found; `~` at p=0.065, 1.71 ms ±5% against 959 µs ±94%), `Instantiate`
  in `tests/perf` (`~`), `EditImported` (`~` at every size), the REPL
  `LoadModel` rows (`~` in time at every size), `Hypot/interpreted`
  (`~`).
- **Confirmed** (significant both times, within a few points of each
  other): `AssignmentLoopStep` (+66% → +70%), `MaterializePlainAttributes`
  (`~` → +18%), `DerivedReadWriteRead` (+12% → +16%), `RunCalc` (+20%),
  `Instantiate/250, 1000` (+32%, +18%), `Fib(25)/interpreted` (+434% →
  +409%), `Collatz(27)/interpreted` (+68% → +60%), `Collatz(27)/go`
  (+56%), `Hypot(3.0,4.0)/go` (+398% → +423%), `FormatEdits` (+69%),
  `InstantiateWarmModel` (+42%), `DecodeSnapshot` (+4.9%),
  `AnalyseResolved` (+10%), and every `tests/perf` and stress row in the
  tables above.
- **Still in doubt:** `OpenSplit/*/warm` (±39–113% on the candidate's
  counts, every one of them above 0.9.2's); `RealDecimalLoop` ±37% and
  `ImportValues/flat/parts=2000/attrs=5` ±37%, candidate-only.

Raw `benchstat` output for every package, first pass and re-run, is in the
tables above; the per-row p-values are p=0.002 (n=6) unless shown.

## What this does not measure

- The new surfaces with no 0.9.2 counterpart — the view renderers, the
  graphs export, the api-json loader, the browser engine, the Python and
  Node clients' new calls — have benchmarks only where listed above
  (`ImportValues`, the exact-numeric loops); the rest are covered by their
  test suites, not by this record.
- The compiled-calc C target is measured on `Hypot` only; the three
  Integer calcs it refuses have no candidate figure, so the C target's
  speed on 0.10.0 is read from one row.
- Everything ran on one 8-CPU virtual machine. Its neighbours were
  quieter than during the 0.9.2 record (the baseline's re-run spreads are
  mostly under 20%), but the stress constellation's `Satisfy` and
  `EditBeside` rows still show 30–80% spreads on single counts, and a
  one-CPU host should read the package figures rather than the
  whole-binary ones, as the previous records said.
