# Performance: release 0.10.0 against release 0.9.2

The release-gate measurement of 0.10.0 — the `v0.10.0` tag at
`08ef39784` (2026-10-12), 2 001 commits past the `v0.9.2` tag
(`4226ad9a3`, 2026-10-05) — against 0.9.2 itself. It follows the method
of the [0.9.2 against 0.9.1](performance-release-0.9.2-vs-0.9.1.md)
record: every benchmark on both revisions, `benchstat` over six runs, the
rows the machine left in doubt re-run interleaved, and whole-binary
timings on generated and real models. See
[performance](../../internals/performance.md) for what the figures mean and
where the remaining cost is.

0.10.0 is a minor release carrying everything on `develop` since 0.9.2:
exact Rationals and unbounded Integers in the evaluator, the exploration
of every order of a body's unordered statements, executors that
interleave move by move, state machines whose change triggers survive a
rollback, actions that perform the steps they inherit, reflective
relationships and indexed connector ends in the semantic model,
incremental re-analysis in the gRPC service, views, code lenses and inlay
hints in the language server, repository and notebook commands in the
REPL, and the fixes of a 300-entry changelog. The expectation going in
was movement on the execution and analysis paths, and that is what the
measurement finds — it is not a parity record.

The release was measured twice on its way to the tag, and the tables
keep one earlier point where it helps: `82f0fac87`, the release merge
with `develop`'s six performance fixes and before the state-machine and
inherited-step work (`internal/exec/runtime`, `internal/ir/lower`)
landed. Where a row moved between `82f0fac87` and the tag the tables
carry its `82f0fac87` figure from the same interleaved run, so the cost
of that last work is read directly. The six fixes, by the change in the
history:

- a cached model's diagnostics are converted to protobuf once, not on
  every `ParseFile` that returns them (`70242d872`);
- the symbol index no longer deep-copies a document's re-export claims to
  decide whether a name's registration changed (`4b4b35121`);
- the binding check on a body write builds its description only when it
  refuses the write (`37b4a17c9`);
- the succession-cycle check is memoized while no behavior is held
  (`8bd97fcf6`);
- a compiled calc's scalars are packed into three words instead of being
  boxed per node (`4a1b937f2`);
- a symbol's qualified name is compared without being built (`367ca7e3a`).

A seventh landed with the inherited-step work itself: an action step's
symbol is found through its scope's declaration index rather than by
scanning the scope's members (`baea6252b`), which is what keeps the
lowering of a chain linear in its length below.

Every row that regresses is confirmed by the interleaved re-run and
attributed to the change responsible below; the fixes that remain open
are named in the summary, and nothing is fixed here.

## Summary

**Nothing here blocks the release.** 0.10.0 is slower than 0.9.2 on most
of the paths a model goes through, by a constant factor on each; nothing
is superlinear, nothing that fit 0.9.2's budgets stops fitting except the
one calc noted below, and the fleet's hydration is faster. Confirmed by
the interleaved re-run, in the order a model meets them:

- **Loading** is +15–18% allocations and +12–20% bytes per element on
  connector-dense models: `sysml -validate` +14–15% and +5–23 MiB at
  3 000–12 000 declarations, Apollo 11 +13% and +13 MiB; the package
  rows' spreads on this machine leave most of their time columns at `~`
  and the allocations are the reading. Each connector, binding and flow
  end is now owned, journaled, indexed and reflected. Structural; linear.
- **Action and state execution** is +40–66% per step with 1.2–1.5× the
  allocations: `AssignmentLoopStep` +66%, `ActionLoop` +42–56%,
  `StateLoop` +33–51%, `ExecuteState` +40%, `ExecuteActionFreshContext`
  +46%, `RunCalc` +7–10%. The exploration of a body's statement orders,
  the per-statement binding frames and the held control values are the
  semantics of the release; the state rows carry a further
  per-executor cost at the tag — `ExecuteState` 610 → 783 KiB,
  `StateLoop/count50` 605 → 802 KiB against `82f0fac87` — for the
  change-trigger bookkeeping that now survives a rollback. The per-step
  constant is where a 0.10.x pass should look next.
- **Lowering an action chain** costs 2.0× the bytes and 1.76× the
  allocations of 0.9.2 at every length (`LowerChain`, `82f0fac87`: +15%
  / `~`), `LowerActionGraph` +46% / +52%: the inherited steps,
  successions, pins, gates and assertions a specialization's graph now
  carries. Linear (56 allocations per step at 10 and at 1 000 steps);
  `ActionChain/chain1000` +52% bytes, `~` time. The same cost reaches
  `FleetInstantiate` (+13% allocations, +30% bytes, `82f0fac87`: +9.5%
  / `~`) through each satellite's state machine and action.
- **Checks with arguments**: `BatchConstraints` +22% allocations and
  +39% bytes, `SameConstraintManyInstances` 1 016 → 1 440 B per check
  (`82f0fac87`: `~`) — a constraint's `in` parameters are bound at check
  time and such a check no longer shares its verdict.
- **Workspace edits beside a large document** allocate 3.0× 0.9.2's
  bytes (`WorkspaceEditSmallDocBesideLarge` 7.95 → 23.7 MiB,
  `EditBeside/512` 2.41 → 6.66 MiB) for +26–56% time: the per-name
  snapshot of re-export claims across the large neighbor. The one row in
  this record with an evident fix still open.
- **The Annex A vehicle example now emits 134
  `Duplicate of imported member name` warnings** (0 in 0.9.2), and every
  path that returns them pays: `Analyze/vehicle` +79%,
  `GRPCParseFileUncached` +72% bytes, `ConnectParseFileHTTPCached`
  343 µs → 1.03 ms (the HTTP transport has no cache of its JSON). Whether
  the rule should fire on nested `::**` imports is a semantics question;
  if not, the group is one fix away from 0.9.2.
- **Exact numerics**: `Collatz(27)` +56–61% (an allocation per Integer
  multiply or divide), `Fib(25)/go` +125% and `Hypot/go` 15 → 64 ns (the
  Go target's values carry their number kind), `Instantiate` +7–29% per
  object (implicit ends and exact-number constructors). The price of the
  feature; `Fib(25)/interpreted` is 8% faster than 0.9.2.
- **Three things 0.10.0 cannot run**, for the release notes:
  `SumTo(1000000)` exceeds the default 10 000 000-step budget it fit in
  0.9.2 (compiled calcs now spend the interpreter's budget); the C target
  refuses calcs on Integers (unbounded against `int64`); the stress
  benchmark's per-document analysis trips the new decimal-rounding lint
  on its generated `Real` literals. And one thing neither can:
  the stress constellation's `PlaneResidency` row fails its own setup on
  both revisions.
- Faster: `HydratePlane` −19–33% allocations, −8–15% time;
  `ExecuteAction` −16% bytes. Binary 42.88 → 47.55 MiB; empty session
  65 → 69 MiB, start time unchanged.

## Method

Machine: `Intel(R) Xeon(R) Platinum 8375C CPU @ 2.90GHz`, 8 CPUs,
31 GiB, Go `go1.25.11 linux/amd64`, `GOMAXPROCS=8`. 0.9.2 is a worktree
at the `v0.9.2` tag, built and benchmarked with its own tree; 0.10.0 is a
worktree at the `v0.10.0` tag, built and benchmarked with its own;
`82f0fac87`, where it is shown, is a third worktree treated the same way.
Package benchmarks are
`go test ./<pkg> -run '^$' -bench . -benchmem -count 6` on each revision,
compared with `benchstat` (rows are significant at p < 0.05; `~` is no
significant difference). The first pass ran each package's six counts on
0.10.0 and then on 0.9.2; rows it left in doubt — a significant
difference, or a spread above 20% on either side — were re-run with the
revisions interleaved, `-count 1` six times each, nothing else on the
machine. Where the first pass left most of a package in doubt
(`internal/frontend/repl`, `tests/perf`, `tests/stressmodel`) the whole
package was re-run that way, and the re-run is the table shown. A
benchmark that fails on one revision (`b.Fatal`) prints `--- FAIL` and no
result line while the package's other benchmarks run on; `benchstat`
then has no row for it, and the failure is reported in that package's
section as a candidate-only, baseline-only or failed row. Nothing was
excluded from `-bench .`. Whole-binary figures are
`/usr/bin/time -f '%e %M'` over nine interleaved runs of each binary, all
built with `make build-sysml` (`-s -w`, `-trimpath`); medians are
reported. The pilot corpora the `vehicle` rows and the gRPC
warm-instantiation row read were fetched into every tree with
`scripts/download-pilot-corpora.sh`. Causes were read from
`git log v0.9.2..v0.10.0 -- <path>` and, where the history alone did not
name one, from `-cpuprofile` and `-memprofile` of the row on both
revisions.

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
  counterpart and are listed with their 0.10.0 figures, not compared.
- **Rows 0.10.0 cannot run.** In `internal/frontend/repl`,
  `CompiledCalc/SumTo(1000000)` fails on 0.10.0 for the interpreted and
  Go targets (`evaluation step limit exceeded (10000000 steps)`), and
  `Fib`, `SumTo` and `Collatz` for the C target are refused
  (`the interpreter's Integers are unbounded and a C program holds int64`);
  the five rows have 0.9.2 figures and no 0.10.0 figure, and are
  discussed with that package. In `tests/stressmodel`, the six
  `AnalyzeSplitPerDocument` rows fail on 0.10.0 because the benchmark
  asserts a clean analysis and the per-document registry now reports the
  decimal literals of the generated model as rounded by their `Real`
  features; they have 0.9.2 figures and no 0.10.0 figure.
- **Rows neither revision can run.** `tests/stressmodel`'s four
  `PlaneResidency` rows fail on both revisions in the benchmark's own
  setup (`ErrRecordStale`); no figure on either side.
- **`ConvertAPIJSON/satellites=512`** allocates 73 GiB per operation;
  0.9.2's first pass was killed for memory after two counts of it (as in
  the previous record), 0.10.0's completed, and the interleaved re-run
  produced the row on both revisions, from which it is compared.
- **Diagnostics**: on every model measured both binaries return the same
  exit status and the same report — `no errors` on the generated models,
  the two example models and all 28 Apollo 11 files. (0.10.0's
  decimal-rounding report is an opt-in lint on the command line; only the
  stress benchmark's registry turns it on.)
- **The machine.** This record's machine is slower than the previous
  records' (see [Re-runs](#re-runs)); every time here is from both
  revisions run on it, and no time is compared with an earlier record.

## Package benchmarks

Time per operation, 0.9.2 then 0.10.0, from the interleaved re-run
wherever a row was re-run (the first pass agrees in sign and roughly in
size on every row named here); bytes and allocations where they moved.
Where a row moved between `82f0fac87` — the release merge before the
state-machine and inherited-step work landed, measured in the same
interleaved runs — and the tag, the `82f0fac87` figure stands between
them. Allocation counts are deterministic to the unit on every revision
and are the reading that separates a change in the code from a change in
the machine.

### `internal/exec/runtime`

| benchmark | 0.9.2 | 0.10.0 | Δ time | Δ B/op | Δ allocs |
| --------- | ----- | ------ | ------ | ------ | -------- |
| `MaterializePlainAttributes` | 65.2 µs | 76.6 µs | +17.6% | +2.1% | 599 → 618 |
| `SetFeatureValueNoDependents` | 722 ns | 816 ns | +13% | ~ | 9 → 9 |
| `DerivedReadWriteRead` | 8.75 µs | 10.5 µs | +20% | ~ | 65 → 65 |
| `AssignmentLoopStep` | 1.38 ms | 2.30 ms | +66% | 522 → 818 KiB (+57%) | 4 924 → 7 329 (+49%) |

`AssignmentLoopStep` runs an action whose loop body assigns an attribute
on every iteration, and is the per-step cost of the release's execution
semantics in isolation: each iteration now explores the orders of its
body's statements, holds a binding frame per statement and keeps the
control values it held across the step (`71fda4cd5`, `807fcf591`,
`a4e1f6ef9`, `a0f77861d`). The lazy binding-check description
(`37b4a17c9`) already took 1 200 allocations off the step; the
2 405 that remain are the frames. `82f0fac87` and the tag agree on this
row to the allocation (7 330 against 7 329) — the state-machine work did
not touch it. `MaterializePlainAttributes` and `DerivedReadWriteRead` are
+18–20% time for 0–19 more allocations: the exact-number constructors
(`923ec81f0`) in the value path. `SetFeatureValueNoDependents` is +13%
with identical allocations, below the spread of its single counts.

Seven rows exist only on 0.10.0 and are listed, not compared (first
pass, this machine): `IntegerArithmeticLoop` 5.30 ms / 280 KiB / 9 071
allocs; `IntegerArithmeticLoopBeyondInt64` 5.72 ms / 514 KiB / 13 070;
`IntegerCollectionSum` 1.77 ms / 4.96 MiB / 45; `RationalDecimalLoop`
7.07 ms / 280 KiB / 9 071; `RationalHarmonicLoop` 1.96 ms / 179 KiB /
4 651; `RealDecimalLoop` 5.58 ms / 280 KiB / 9 070;
`BehaviorOrdersLibraryModel` 629 µs / 163 KiB / 1 922. The Integer loop
costs the same as the Real loop to the allocation while its values fit a
word, and 44% more allocations once they do not.

### `internal/frontend/repl`

| benchmark | 0.9.2 | 0.10.0 | Δ time | Δ B/op | Δ allocs |
| --------- | ----- | ------ | ------ | ------ | -------- |
| `LoadModel/elements=0` | 586 µs | 607 µs | ~ | +2.5% | +2.9% |
| `LoadModel/elements=250` | 40.6 ms | 42.8 ms | ~ | +2.3% | +3.6% |
| `LoadModel/elements=1000` | 161 ms | 172 ms | +7.0% | +2.3% | +3.8% |
| `LoadModel/elements=4000` | 710 ms | 774 ms | +9.0% | +2.8% | +3.8% |
| `RunStateMachine/elements=250…4000` | 20.9 µs | 22.5 µs | +6.7–8.2% | 15.23 → 15.93 KiB | 213 → 223 |
| `RunCalc/elements=250…4000` | 6.6–6.7 µs | 7.2–7.3 µs | +7.1–9.9% | 4.24 → 4.45 KiB | 76 → 79 |
| `Instantiate/elements=250` | 8.73 µs | 11.3 µs | +29% | +12.9% | 53 → 56 |
| `Instantiate/elements=1000` | 8.27 µs | 9.31 µs | +13% | +12.4% | 53 → 56 |
| `Instantiate/elements=4000` | 8.39 µs | 8.97 µs | +6.9% | +12.4% | 53 → 56 |
| `Diagnostics/attributes=50…800` | 155 µs – 1.91 ms | 145 µs – 1.99 ms | ~ | +0.1–1.3% | +0.3–2.6% |
| `CompiledCalc/Fib(25)/interpreted` | 11.1 ms | 10.2 ms | **−8.4%** | +18% | 489 → 574 |
| `CompiledCalc/Fib(25)/go` | 1.02 ms | 2.30 ms | +125% | 7 → 16 B | 0 |
| `CompiledCalc/Collatz(27)/interpreted` | 399 µs | 644 µs | +61% | 29.9 → 82.8 KiB (+177%) | 1 004 → 2 122 (+111%) |
| `CompiledCalc/Collatz(27)/go` | 7.31 µs | 11.4 µs | +56% | 0 | 0 |
| `CompiledCalc/Hypot(3.0,4.0)/interpreted` | 10.7 µs | 12.1 µs | +13% | +2.9% | 88 → 91 |
| `CompiledCalc/Hypot(3.0,4.0)/c` | 21.3 ns | 22.3 ns | +4.8% | 0 | 0 |
| `CompiledCalc/Hypot(3.0,4.0)/go` | 15.3 ns | 63.6 ns | +317% | 0 | 0 |

Five rows have a 0.9.2 figure and no 0.10.0 figure, as in the previous
measurement of the candidate: `CompiledCalc/SumTo(1000000)` for the
interpreted and Go targets fails with
`evaluation step limit exceeded (10000000 steps)` (0.9.2: 3.23 s and
866 µs) because a compiled calc now spends the interpreter's step budget,
and `Fib(25)`, `SumTo(1000000)` and `Collatz(27)` for the C target are
refused (`the interpreter's Integers are unbounded and a C program holds
int64`; 0.9.2: 286 µs, 540 µs, 1.23 µs). Both belong in the release
notes; neither is a performance regression.

The interpreted `Fib(25)` is the one row the six fixes turned around: the
three-word scalar packing (`4a1b937f2`) took it from 5.3× 0.9.2 to 8%
faster on this machine (20% on the earlier one; the spread is ±4% on
both sides). `Collatz(27)/interpreted` doubles its allocations: an
Integer multiply or divide allocates its exact result (`923ec81f0`,
`79629923b`), where 0.9.2 did `float64` arithmetic in place.
`Fib(25)/go` and `Hypot/go` are the Go target's values carrying their
number kind (`d6eafc870`): a tagged word per operation instead of a bare
`float64`, 15 → 64 ns on a two-operation calc. `Instantiate` is +12.4%
bytes per object for the implicit relationship ends (`fa8a7220c`,
`ca0e6cca0`), and `RunStateMachine` gained 6 allocations per run at the
tag — the change-trigger bookkeeping journaled per transition
(`bc1baced5`; `82f0fac87` had 217). `LoadModel` is the loader's +3.8%
allocations per element on a connector-light generated model (the
constellation below shows the connector-dense figure).

### `internal/frontend/grpc`

| benchmark | 0.9.2 | 0.10.0 | Δ time | Δ B/op | Δ allocs |
| --------- | ----- | ------ | ------ | ------ | -------- |
| `InstantiateWarmModel` | 200 µs | 291 µs | +46% | 83.4 → 107 KiB (+28%) | 1 555 → 1 862 (+20%) |
| `ParseFileColdShared` | 10.7 ms | 11.0 ms | ~ | +0.2% | +0.5% |
| `ParseFileColdInline` | 11.0 ms | 11.3 ms | ~ | +0.7% | +0.9% |

`InstantiateWarmModel` instantiates the vehicle example's parts over a
warm session: +20% allocations, the implicit relationship ends and the
exact-number attribute values (`fa8a7220c`, `923ec81f0`), unchanged from
`82f0fac87` (1 860). The cold parse rows are the parser and loader on a
small file and are within 1%.

### `internal/frontend/lsp`

| benchmark | 0.9.2 | 0.10.0 | Δ time | Δ B/op | Δ allocs |
| --------- | ----- | ------ | ------ | ------ | -------- |
| `FormatEdits` | 639 µs | 1.13 ms | +78% | 434 → 704 KiB (+62%) | 78 → 85 |
| `SpanToRangeLarge` | 3.54 ms | 3.44 ms | −3.0% | 0 | 0 |
| `ReferencesWarm` | 303 µs | 284 µs | ~ | ~ | ~ |
| `ReferencesCold` | 58.1 ms | 58.1 ms | ~ | +0.1% | ~ |
| `RenameWarm` | 25.9 ms | 25.6 ms | ~ | ~ | ~ |
| `WorkspaceUpdate` | 324 ms | 402 ms | +24% | 49.7 → 57.0 MiB (+15%) | 607k → 712k (+17%) |

`FormatEdits` formats `tests/codegen/testdata/compile_calcs.sysml`,
which grew from 597 to 930 lines on 0.10.0 with the new compiled
fixtures; the bytes move by the ratio of the file sizes (+62% against
+56% more lines) and the formatter itself has one commit in the interval
(the indexed connector ends, `63552cbc4`). Not a regression in the code.
`WorkspaceUpdate` edits a document in
a workspace of 128 and re-analyses: +17% allocations is the symbol
index's owned connector ends and the diagnostics of the re-analysis
(`00b8499a8`, `da8fba4a3`); the re-export claim fix (`4b4b35121`) took it
from +53% to +17%. Both unchanged from `82f0fac87` to the allocation.
References and rename are unchanged.

### `internal/translate/migrate`, `internal/workspace/libs`, `internal/workspace/model`

| benchmark | 0.9.2 | 0.10.0 | Δ time | Δ B/op | Δ allocs |
| --------- | ----- | ------ | ------ | ------ | -------- |
| migrate `WriterSiblingBlocks` | 2.23 ms | 3.03 ms | +36%¹ | ~ | 20 022 → 20 022 |
| libs `ExpandWildcardImports` | 42.3 ms | 73.6 ms | +74%¹ | +6.2% | +13.5% |
| libs `IndexLibrary` | 21.1 ms | 22.5 ms | +6.6% | +3.8% | +3.4% |
| libs `ExpandModelImports` | 10.7 ms | 15.8 ms | +48%¹ | +1.4% | +5.5% |
| libs `DecodeSnapshot` | 14.0 ms | 14.7 ms | ~ | 35.3 → 37.9 MiB (+7.2%) | 56.6k → 67.5k (+19%) |
| libs `SetDigest` | 772 µs | 996 µs | +29%¹ | +1.6% | +3.9% |
| model `AnalyseUnresolved` | 29.7 ms | 44.8 ms | +51%¹ | +0.8% | +0.9% |
| model `AnalyseResolved` | 1.72 ms | 1.88 ms | +9.8% | +8.7% | 5 049 → 5 970 (+18%) |

¹ First-pass figure (this machine, `-count 6` on each revision in turn,
not interleaved); the rows' allocations are unchanged or within 14%, and
their 20–40% single-count spreads on this machine leave the time column
unconfirmed. The interleaved rows (`IndexLibrary`, `DecodeSnapshot`,
`AnalyseResolved`) are the ones to read. `DecodeSnapshot` is the bundled
standard library's index decoded at start, 2.5 MiB and 10 800
allocations more per decode for the owned-relationship journal and the
reflected implicit relationships in the snapshot (`da8fba4a3`,
`fa8a7220c`); it is the session floor's +4 MiB below. `AnalyseResolved`
is +18% allocations for the connector ends the symbol table now owns and
indexes. `WriterSiblingBlocks` allocates identically on both sides.

### `tests/perf`

The harness's end-to-end rows: the vehicle example (Annex A) and a
synthetic 4 000-part model through lexing, parsing, indexing, analysis,
workspace edits, execution, constraints, the gRPC and HTTP transports.
Time per operation, 0.9.2, `82f0fac87` and 0.10.0, from the interleaved
re-run of the whole package; bytes and allocations against 0.9.2.

| benchmark | 0.9.2 | `82f0fac87` | 0.10.0 | Δ B/op | Δ allocs |
| --------- | ----- | ----------- | ------ | ------ | -------- |
| `Lex/synthetic`, `Lex/vehicle` | 9.85 ms, 370 µs | ~ | ~ | 0 | 0 |
| `Parse/synthetic`, `Parse/vehicle` | 118 ms, 3.20 ms | ~ | ~ | +0.0% | +0.0% |
| `IndexAddExpand/synthetic` | 83.4 ms | ~ | ~ | +6.4% | +7.6% |
| `IndexAddExpand/vehicle` | 42.7 ms | ~ | ~ | +1.9% | +6.5% |
| `IndexAddOnly/synthetic` | 70.9 ms | ~ | ~ | ~ | ~ |
| `IndexAddOnly/vehicle` | 1.54 ms | ~ | ~ | +4.3% | +3.8% |
| `Analyze/synthetic` | 1.89 s | ~ | ~ | +4.5% | +4.0% |
| `Analyze/vehicle` | 185 ms | 338 ms (+83%) | 330 ms (+79%) | 73.7 → 130 MiB (+76%) | 628k → 871k (+39%) |
| `WorkspaceEdit/synthetic` | 292 ms | 379 ms (+30%) | 461 ms (+58%)¹ | +10% | +14% |
| `WorkspaceEdit/synthetic/reindex+diagnostics` | 2.37 s | ~ | ~ | +5.3% | +5.7% |
| `WorkspaceEdit/vehicle` | 83.2 ms | 107 ms (+29%) | 106 ms (+28%) | +18% | +21% |
| `WorkspaceEdit/vehicle/reindex+diagnostics` | 253 ms | 352 ms (+39%) | 422 ms (+66%)¹ | 71.3 → 121 MiB (+70%) | +33% |
| `WorkspaceEditSmallDocBesideLarge` | 67.8 ms | 95.5 ms (+41%) | 106 ms (+56%) | 7.95 → 23.7 MiB (+198%) | 81.7k → 115k (+40%) |
| `FQNOf` | 2.00 ms | +18% | 2.45 ms (+23%) | +9.5% | 36 000 → 40 000 |
| `LookupQualified` | 1.97 ms | ~ | ~ | 0 | 0 |
| `FeaturesOf` | 1.41 s | +4.5% | ~ | ~ | ~ |
| `REPLLoadFile` | 2.16 s | +9.3% | 2.45 s (+14%) | +3.4% | +3.4% |
| `REPLSubmitSnippet` | 1.65 s | +13% | 1.93 s (+17%) | +6.6% | +4.4% |
| `REPLEvalExpr` | 5.37 ms | ~ | ~ | +0.0% | 101 → 104 |
| `LowerActionGraph` | 2.67 µs | ~ (2.91 KiB, 27) | 4.52 µs (+69%) | 2.63 → 3.84 KiB (+46%) | 25 → 38 (+52%) |
| `LowerStateGraph` | 13.0 µs | ~ | ~ | +1.7% | 146 → 149 |
| `ExecuteAction` | 98.6 µs | ~ | ~ | −16% | 193 → 244 (+26%) |
| `ExecuteActionFreshContext` | 48.2 µs | 67.0 µs (+39%) | 70.1 µs (+46%) | 26.4 → 35.9 KiB (+36%) | 268 → 338 (+26%) |
| `ExecuteState` | 1.13 ms | ~ (610 KiB, 11 620) | 1.58 ms (+40%) | 486 → 783 KiB (+61%) | 10 390 → 12 140 (+17%) |
| `BatchConstraints` | 18.5 ms | ~ (2.63 MiB, 56 720) | 23.2 ms (+26%) | 2.53 → 3.52 MiB (+39%) | 56.3k → 68.9k (+22%) |
| `SameConstraintManyInstances` | 5.39 µs | ~ (1 000 B, 29) | ~ | 1 016 → 1 440 B (+42%) | 29 → 35 (+21%) |
| `BatchSatisfy` | 1.76 s | ~ | ~ | +3.7% | +3.7% |
| `Instantiate` | 1.20 s | ~ | +6.4% | +4.6% | +4.0% |
| `GRPCParseFileCached` | 94.4 µs | ~ | ~ | +0.6% | 22 → 24 |
| `GRPCParseFileUncached` | 197 ms | ~ | ~¹ | 83.5 → 144 MiB (+72%) | 664k → 913k (+38%) |
| `GRPCEvaluate` | 13.0 µs | ~ | ~ | +12% | 110 → 116 |
| `GRPCVerifyConstraint` | 1.28 ms | ~ | ~ | +6.4% | +6.5% |
| `ConnectEvaluateHTTP` | 158 µs | ~ | ~ | +3.0% | 201 → 206 |
| `ConnectParseFileHTTPCached` | 343 µs | 1.03 ms (+201%) | 1.03 ms (+201%) | 352 → 462 KiB (+31%) | 166 → 1 017 (+513%) |
| `ActionLoop/for10` | 41.5 µs | +43% | 64.6 µs (+56%) | +39% | 204 → 263 (+29%) |
| `ActionLoop/for100` | 169 µs | +43% | 239 µs (+42%) | +40% | 477 → 626 (+31%) |
| `ActionLoop/for1000` | 1.42 ms | ~ | ~¹ | 279 → 411 KiB (+47%) | 3 181 → 4 231 (+33%) |
| `ActionChain/chain10` | 125 µs | ~ | ~ | +13% | 675 → 805 (+19%) |
| `ActionChain/chain100` | 857 µs | +26% | 1.17 ms (+37%) | 513 → 756 KiB (+48%) | 5 758 → 6 639 (+15%) |
| `ActionChain/chain1000` | 20.5 ms | ~ (7.08 MiB) | ~ (25.6 ms)¹ | 5.16 → 7.85 MiB (+52%) | 56.3k → 65.3k (+16%) |
| `StateLoop/count50` | 1.10 ms | ~ (605 KiB) | 1.66 ms (+51%) | 481 → 802 KiB (+67%) | +17% |
| `StateLoop/count500` | 10.6 ms | +28% | 14.0 ms (+33%) | 4.52 → 6.11 MiB (+35%) | +17% |
| `StateLoop/count5000` | 104 ms | ~ | ~¹ | 45.3 → 59.7 MiB (+32%) | 1.006M → 1.176M (+17%) |
| `LowerChain/chain10` | 12.0 µs | ~ (8.41 KiB, 78) | 23.5 µs (+96%) | 7.42 → 14.8 KiB (+99%) | 73 → 129 (+77%) |
| `LowerChain/chain100` | 209 µs | ~ (75.0 KiB, 471) | 324 µs (+55%) | 65.1 → 138 KiB (+112%) | 460 → 813 (+77%) |
| `LowerChain/chain1000` | 13.6 ms | ~ (1.06 MiB, 4 140) | ~ (14.7 ms)¹ | 922 → 1 857 KiB (+101%) | 4 118 → 7 227 (+76%) |

¹ Spread above 35% on one side after six interleaved rounds; the time
column of these rows is not confirmed and the bytes and allocations are
the reading. The first pass (not interleaved, this machine) agrees in sign
on each.

Eight rows exist only on 0.10.0 (the CSV/TSV/JSON value import; first
pass, this machine): `ImportValues/flat` 54.2 ms, 279 ms, 999 ms and
1.19 s for 100, 500 and 2 000 parts at three attributes and 2 000 at five
(131k, 615k, 2.43M and 3.67M allocations); `ImportValues/nested` 56.0 ms,
312 ms, 1.21 s and 1.88 s (175k, 829k, 3.28M and 4.57M). Linear in parts
times attributes, ≈400 allocations per imported value.

Reading the table by cause:

- **Lexing, parsing and indexing are unchanged** in time, with 4–8% more
  allocations in the index (the reflective relationships each declaration
  now registers, `00b8499a8`).
- **Analysis of the synthetic model** is `~` in time with +4.0%
  allocations (+3.3% on `82f0fac87`; the tag adds seven-tenths of a
  point with its semantic fixes to satisfy and verify relationships). **Analysis of the vehicle example is +79% with
  1.8 times the bytes** because the cost is not in the analysis but in
  its result: 0.10.0 reports 134 `Duplicate of imported member name`
  warnings on the Annex A vehicle example where 0.9.2 reported none (the
  resolver's `checkImportedNames`, `eb4b7583c`, which now sees the names
  a nested `::**` import brings alongside the package's own). Every path
  that returns the example's diagnostics pays to build and carry them:
  `WorkspaceEdit/vehicle/reindex+diagnostics` +70% bytes,
  `GRPCParseFileUncached` +72% bytes with 249 000 more allocations,
  `ConnectParseFileHTTPCached` 343 µs → 1.03 ms with 851 more
  allocations (the HTTP transport serializes the 134 warnings to JSON on
  every request; the gRPC transport's cached protobuf conversion,
  `70242d872`, is why `GRPCParseFileCached` is 24 allocations against
  0.9.2's 22 and `~` in time). Whether the rule should fire on the
  vehicle example's imports is a semantic question for the resolver's
  owners; **if it should not, every row in this group is one fix away
  from 0.9.2**, and if it should, the HTTP path wants the same cache the
  gRPC path got. A model with no such warnings does not pay it: the
  synthetic model's rows are the same code without them.
- **Workspace edits.** `WorkspaceEditSmallDocBesideLarge` — a one-line
  edit to a small document in the same workspace as the 4 000-part model
  — allocates 3.0 times 0.9.2's bytes and +40% objects for +56% time;
  `WorkspaceEdit/vehicle` +18% bytes, +21% allocations, +28% time. The
  re-export fix (`4b4b35121`) took the first from 3.9× to 3.0×; what
  remains is the re-registration itself: `symbols.registrationOf` still
  snapshots each name's re-export claims by reference (`4ca321508`),
  which is a slice per name of the large neighbor per edit, 15.7 MiB per
  edit of the small document. **Confirmed, and the one row in this
  record a 0.10.x pass should start with.** The synthetic edit's +58%
  time stands on ±34–48% spreads; its bytes (+10%) and allocations
  (+14%) are the stable reading. Nothing in this group moved between
  `82f0fac87` and the tag.
- **Execution is where the release's semantics cost**: `ActionLoop`
  +42–56%, `StateLoop` +33–51%, `ExecuteState` +40%,
  `ExecuteActionFreshContext` +46%, with 17–33% more allocations. The
  loop rows are the statement-order exploration and the per-statement
  binding frames described under `internal/exec/runtime`, which a loop
  body of one assignment pays on every iteration — the lazy
  binding-check description (`37b4a17c9`) already took `ActionLoop/for1000`
  from 6 220 to 4 231 allocations — and are unchanged from `82f0fac87`.
  **Confirmed; structural.** The state rows moved at the tag: `ExecuteState`
  from 610 to 783 KiB and `StateLoop/count50` from 605 to 802 KiB, with
  5% more allocations, while `count5000` moved 4% — a per-executor cost,
  not per step: the change-trigger bookkeeping journaled per transition
  and the change-condition read sets kept across a snapshot restore
  (`bc1baced5`, `3dfbdb7c6`, `91f9d384a`), which a machine entered once
  pays once. **Confirmed; the price of restoring trigger state correctly
  on rollback.** `ExecuteAction` (a single action, no loop) is `~` in time
  with 51 more allocations and fewer bytes.
- **Lowering an action chain costs twice the bytes and 1.76× the
  allocations at every length** (`LowerChain`, `LowerActionGraph` +46% /
  +52%), from `82f0fac87`'s +15% / `~`: the inherited action steps,
  successions, pins, gates and assertions a specialization's graph now
  carries from its general action (`896d35582`, `fa8f50125`, `9d25656eb`,
  `a5dbc9d3b`, `c8da68c1c`, `308d6a625`). The cost is linear — 10 →
  1 000 steps is 129 → 7 227 allocations, 56 per step at every length —
  because the step's symbol is found through the scope's declaration
  index (`baea6252b`); looked up by scanning the scope's members, as the
  work first did, the same row was 53.9 MiB at 1 000 steps against 7.85
  here. `ActionChain` (lowering plus execution) is +52% bytes and +16%
  allocations, `~` in time at 1 000 steps. **Confirmed; structural, and
  the per-step constant of lowering is the second place a 0.10.x pass
  should look.**
- **Constraints.** `BatchConstraints` is +22% allocations and +39% bytes
  at the tag where `82f0fac87` was `~` (the memoized succession-cycle
  check, `8bd97fcf6`, had cleared the release merge's +68%), and
  `SameConstraintManyInstances` 1 016 → 1 440 B per check, 29 → 35
  allocations: a constraint's or requirement's `in` parameters are now
  bound at check time (`645df3d19`) and a check that takes arguments no
  longer shares its verdict (`d070d92a6`) — six allocations per check for
  the argument frame. **Confirmed; the price of checks with arguments.**
  `BatchSatisfy` and `GRPCVerifyConstraint` are `~` with +3.7% and
  +6.5% allocations (the exact-number evaluation of the bodies);
  `GRPCEvaluate` six allocations more per call.
- **Instantiate**, the 4 000-part model's instantiation, is +6.4% in
  time with +4.0% allocations; the per-object cost is the REPL's
  `Instantiate` row.
- **REPL**: loading the 4 000-part file is +14% in time with +3.4%
  allocations; submitting a snippet to the loaded model +17% with +4.4%.
  `FQNOf` is +11% allocations, 4 000 more per 36 000 names.

## Stress constellation (`tests/stressmodel`)

The satellite-network generator at 32, 128 and 512 satellites (6 227,
24 167 and 95 927 elements) — connector-, binding- and flow-dense, with a
state machine and a `DownlinkPass` action per satellite — loaded, split
across six files, edited beside, validated, instantiated, hydrated and
converted. Time per operation, 0.9.2 and 0.10.0, from the interleaved
re-run of the whole package (six rounds; `82f0fac87` stands between them
where a row moved at the tag); bytes and allocations from the same run.

| benchmark | 0.9.2 | 0.10.0 | Δ time | Δ B/op | Δ allocs |
| --------- | ----- | ------ | ------ | ------ | -------- |
| `Load/32` | 679 ms | 775 ms | ~¹ | 133 → 160 MiB (+20%) | +16% |
| `Load/128` | 2.85 s | 2.55 s | ~¹ | 501 → 569 MiB (+13%) | +15% |
| `Load/512` | 9.47 s | 10.1 s | ~ | 1.94 → 2.17 GiB (+12%) | +15% |
| `Satisfy/32…512` | 6.55 ms – 130 ms | 7.52 ms – 152 ms | ~, ~, +17% | +6.2% | +15.7% |
| `FleetInstantiate/32` | 5.67 ms | 5.73 ms | ~ (+21% in the first re-run) | 5.27 → 6.88 MiB (+31%) | 12 720 → 14 350 (+13%) |
| `FleetInstantiate/128` | 21.3 ms | 23.4 ms | +9.8% | 24.1 → 31.4 MiB (+30%) | 48 370 → 54 600 (+13%) |
| `FleetInstantiate/512` | 84.1 ms | 83.5 ms | ~ | +15%¹ | 190.9k → 215.6k (+13%) |
| `FleetSatisfy/32…512` | 1.81 – 7.96 ms | 1.86 – 9.46 ms | ~, ~, +19%¹ | +4.7–5.1% | +12–13% |
| `EditBeside/32` | 1.84 ms | 1.76 ms | ~ | 296 → 559 KiB (+89%) | 3 442 → 4 536 (+32%) |
| `EditBeside/128` | 4.26 ms | 5.31 ms | +25%¹ | 720 KiB → 1.76 MiB (+150%) | 8 377 → 11 973 (+43%) |
| `EditBeside/512` | 18.6 ms | 23.6 ms | +26%¹ | 2.41 → 6.66 MiB (+177%) | 28 110 → 41 680 (+48%) |
| `ValidateSplit/32…512/jobs=1` | 0.81 s – 11.0 s | 0.89 s – 12.5 s | ~ | +12–17% | +15–16% |
| `ValidateSplit/32…512/jobs=8` | 0.38 s – 4.47 s | 0.37 s – 4.28 s | ~ | +12–16% | +15–16% |
| `LoadFiles/32…512` | 0.80 s – 10.5 s | 0.81 s – 11.7 s | ~, +12%, ~ | +14–19% | +18% |
| `EditImported/32…512` | 1.01 s – 10.5 s | 0.87 s – 11.4 s | ~, ~, +9.0% | +16–17% | +19% |
| `OpenSplit/32…512/cold` | 0.39 s – 4.39 s | 0.39 s – 4.38 s | ~ | +14–18% | +16–17% |
| `OpenSplit/32/warm` | 162 ms | 228 ms | +41%¹ | 65.6 → 79.7 MiB (+21%) | +22% |
| `OpenSplit/128/warm` | 357 ms | 440 ms | +23% | 167 → 209 MiB (+25%) | +24% |
| `OpenSplit/512/warm` | 1.04 s | 1.40 s | +34% | 613 → 891 MiB (+45%) | +25% |
| `HydratePlane/32` | 149 ms | 134 ms | ~ | 25.8 → 20.9 MiB (**−19%**) | **−19%** |
| `HydratePlane/128` | 356 ms | 328 ms | **−7.9%** | 76.6 → 57.5 MiB (**−25%**) | **−29%** |
| `HydratePlane/512` | 1.36 s | 1.16 s | **−15%** | 287 → 206 MiB (**−28%**) | **−33%** |
| `ConvertAPIJSON/32` | 9.00 s | 9.02 s | ~ | +0.7% | −1.5% |
| `ConvertAPIJSON/128` | 38.8 s | 38.6 s | ~ | +0.7% | −1.5% |
| `ConvertAPIJSON/512` | 169 s² | 171 s | ~ | 72.8 → 73.3 GiB | 402M → 396M |

¹ Spread above 20% on one side after six rounds; time unconfirmed, bytes
and allocations are the reading. ² One count on 0.9.2 (see below).

Twelve rows have a 0.9.2 figure and no 0.10.0 figure (`AnalyzeSplitPerDocument`,
six rows) or no figure on either revision (`PlaneResidency`, four rows at
32, 128, 512 and 1 600 satellites), discussed below.

- **The loader costs 15–18% more allocations and 12–20% more bytes per
  element** at every scale and through every entrance (`Load`,
  `LoadFiles`, `ValidateSplit`, `OpenSplit/cold`, `EditImported`), for
  9–12% more time where the spread lets the difference show and `~`
  where it does not (on this machine, most of the rows: the loader's
  single counts spread 15–55% on 0.9.2). Each connector, binding and flow
  end the constellation is made of is now owned, journaled, indexed and
  reflected (`00b8499a8`, `da8fba4a3`, `32890be28`, `63552cbc4`), and the
  extended declarations classified (`ca0e6cca0`). **Confirmed by the
  allocations at every size; structural, linear, and the cost of
  0.10.0's relationship model.** The whole-binary table below shows the
  same from outside the process, where the spread is small enough to
  show the time. Unchanged from `82f0fac87` to the allocation.
- **The satisfaction checks are +16% allocations and +6% bytes on the
  fleet**, `~` to +17% time: the exact-number evaluation of the
  assertions (`923ec81f0`, `79629923b`), 4 500 allocations per 96
  assertions, with the succession-cycle walk memoized (`8bd97fcf6`).
  **Confirmed; explained.**
- **Editing beside the constellation allocates 1.9–2.8 times the bytes
  of 0.9.2** and +32–48% objects, for `~` to +26% time; the warm
  `OpenSplit` (+21–45% bytes) and `WorkspaceUpdate` in the LSP table are
  the same cost. The per-name snapshot of claims by reference across the
  512-satellite neighbor (`4ca321508`) is what remains after the
  re-export fix (`4b4b35121`), 4.3 MiB per edit at 512. **Confirmed; the
  open fix.** `OpenSplit/512/warm` adds the warm cache's re-validation of
  the owned-relationship journal it now carries.
- **Instantiation of the fleet is +13% allocations and +30% bytes** at
  the tag, where `82f0fac87` was +9.5% and `~`: each satellite's state
  machine and action are lowered with their inherited steps and
  change-trigger bookkeeping (the `LowerChain` and `ExecuteState` costs
  above, per satellite); +9.8% time at 128, `~` at 32 and 512 (one
  earlier interleaved run had +21% at 32). **Confirmed; the same cause as
  the lowering rows.** `HydratePlane` is **19–33% fewer allocations and
  19–28% fewer bytes** than 0.9.2, 8–15% faster: the hydration of a
  plane's instances no longer rebuilds the implicit ends it can derive
  (`fa8a7220c`). The API-JSON conversion is unchanged in time with 1.5%
  fewer allocations.

`AnalyzeSplitPerDocument` fails on 0.10.0 at every size, as it did on
`82f0fac87`. The benchmark analyses each of the constellation's six files
in its own per-document registry and asserts a clean result; 0.10.0's
per-document analysis reports each decimal literal the generator writes
into a `Real`-typed attribute (`0.2 is rounded to the nearest Real,
0.20000000000000001, since a feature typed by Real holds a binary64
value; type the feature by Rational to keep the literal exact`,
`014fceb7b`), ten per file, and the assertion trips. The six rows are
baseline-only figures in this record (0.9.2: 656 ms / 159 ms, 2.27 s /
597 ms and 11.3 s / 2.50 s at one and eight jobs); the benchmark's
generator or its assertion needs the release's view of decimal literals
before the row can be compared again. The whole-workspace analysis the
other rows and `sysml -validate` do is not affected — the report is a
lint the per-document registry turns on — which is why `ValidateSplit`
and the whole-binary runs are clean on both.

`PlaneResidency` — the residency of a plane's interface record among the
constellation's documents — fails on **both** revisions at every size with
`workspace: the interface record was written among other documents:
plane000.sysml` (`ErrRecordStale`): the benchmark writes the record after
the plane's documents and the workspace refuses to open it as stale. It
never ran in 0.9.2's full pass, where the package was killed first (next
paragraph), and a focused run of the row on 0.9.2 fails the same way; it
is a defect of the benchmark's setup, not a difference between the
revisions. No figure on either side.

`ConvertAPIJSON/satellites=512` allocates 73 GiB per operation. On 0.9.2
the package's first pass completed two counts (169 s) before the process
was killed for memory on this 31 GiB machine (`signal: killed`, as in the
previous record); 0.10.0's first pass completed all six (169–235 s) and
the interleaved re-run produced the row on both revisions (`~`), so it is
compared here, from the re-run. The row is a memory test of the machine
as much as of the converter and is read with that in mind.

## Whole-binary scaling

`sysml -validate` over generated compliant models (the satellite-network
generator at ≈3 000, 6 000 and 12 000 declarations: 3 804, 7 260 and
14 172 lines) that report `no errors`, nine runs each with the three
binaries interleaved; median wall seconds and median maximum resident set,
every run exiting 0.

| declarations | 0.9.2 | `82f0fac87` | 0.10.0 |
| ------------ | ----- | ----------- | ------ |
| ≈3 000 | 0.13 s / 95 MiB | 0.16 s / 100 MiB | 0.15 s / 100 MiB |
| ≈6 000 | 0.20 s / 108 MiB | 0.24 s / 114 MiB | 0.23 s / 115 MiB |
| ≈12 000 | 0.36 s / 133 MiB | 0.42 s / 157 MiB | 0.41 s / 156 MiB |

Linear in the declarations on every binary, as before: doubling the model
doubles the time (0.15 → 0.23 → 0.41 s) and adds 15–41 MiB. 0.10.0 is
14–15% slower than 0.9.2 and holds 5–23 MiB more, growing with the model
— the loader's per-element cost of the stress constellation (+12–20%
bytes, +15–18% allocations) seen from outside the process. The tag and
`82f0fac87` agree within a sample: none of the state-machine work is on
the load path. The spreads were within ±0.01 s on 0.9.2 and `82f0fac87`;
0.10.0's first round of each model was its slowest (0.77, 1.54 and 2.46
s, a cold page cache for the freshly built binary) and the remaining
eight rounds agree with the medians.

### Process start and session floor

| measure | 0.9.2 | `82f0fac87` | 0.10.0 |
| ------- | ----- | ----------- | ------ |
| `sysml -e 1` wall time | 0.03 s | 0.02 s | 0.03 s |
| `sysml -e 1` max RSS (session floor) | 65 MiB | 69 MiB | 69 MiB |
| `bin/sysml` size (`-s -w`) | 42.88 MiB | 47.23 MiB | 47.55 MiB |

The floor is the standard library's bundled index decoded at start, and
rose 4 MiB with the snapshot's new contents (the owned-relationship
journal and the reflected implicit relationships, `DecodeSnapshot`
+7.2% bytes). The binary is 4.67 MiB larger than 0.9.2's: the new
frontends (views, code lenses, inlay hints, the REPL's repository and
notebook commands, `-list`), the exact-number evaluator, the value import,
the SysML v2 API client and, in the last 0.32 MiB, the inherited-step
lowering and the audit and migration fixes of the release's final
hotfixes. Start time is unchanged within the timer's resolution.

### Example models and the Apollo 11 model

| model | 0.9.2 | `82f0fac87` | 0.10.0 |
| ----- | ----- | ----------- | ------ |
| `examples/action-executor-demo.sysml` | 0.03 s / 66 MiB | 0.03 s / 70 MiB | 0.03 s / 71 MiB |
| `examples/disposal-robot-demo/robot.sysml` | 0.06 s / 72 MiB | 0.06 s / 77 MiB | 0.06 s / 77 MiB |
| Apollo 11 (28 files, `scripts/download-apollo11.sh`) | 0.55 s / 146 MiB | 0.61 s / 159 MiB | 0.62 s / 160 MiB |

The two small examples sit on the session floor and move with it. The
Apollo 11 model — the one real model in the set, connector- and
flow-dense like the stress constellation — validates in 0.62 s against
0.55 s (+13%) holding 13 MiB more. Every file validates with the same
report on all three binaries.

## Re-runs

The first pass on this machine left 114 of the 130 compared time rows
in doubt: 74 by a significant difference, 40 by a spread above 20% on one
side with no significant difference (the stress constellation's time rows
almost entirely, `Lex/synthetic` ±107%, `LookupQualified` ±111%,
`Fib(25)/interpreted` ±111% on 0.10.0). All were re-run interleaved —
`-count 1` of the row on 0.9.2, then on `82f0fac87`, then on 0.10.0, six
times, nothing else on the machine — and where most of a package was in
doubt (`internal/frontend/repl`, `tests/perf`, `tests/stressmodel`) the
whole package was re-run that way and the re-run is the table shown. The
re-run:

- **confirmed** every regression named in the summary, with the bytes
  and allocations identical to the first pass to the unit on every row,
  and the time deltas smaller than the first pass's on most
  (`WorkspaceUpdate` +127% → +24%, `FormatEdits` +117% → +78%,
  `AnalyseResolved` +53% → +9.8%, `InstantiateWarmModel` +74% → +46%,
  `LoadModel/4000` +35% → +9.0%, `IndexLibrary` +35% → +6.6%,
  `BatchConstraints` +123% → +26%, `ExecuteState` +73% → +40%,
  `LowerActionGraph` +118% → +69%);
- **dismissed** the first pass's time differences on `Instantiate`
  (first pass +12%, re-run +6.4%), `LowerStateGraph` (+29% → `~`),
  `REPLEvalExpr` (+35% → `~`), `GRPCVerifyConstraint` (+44% → `~`),
  `ActionChain/chain1000` and `LowerChain/chain1000` (`~` both ways),
  `IndexAddExpand` and `IndexAddOnly` (+31–39% → `~`),
  `RunStateMachine` (+13–30% → +7–8%), `Collatz(27)/interpreted`
  (+94% → +61%), `Hypot/interpreted` (+30% → +13%), `ReferencesWarm`
  (+15% → `~`) and, in the stress constellation, `Satisfy/512`
  (+17% in both, the one loader-side time row that held),
  `EditBeside/128` and `/512` (+25–26% on ±25–41% spreads, unconfirmed);
- **left unconfirmed** in time, with spreads above 35% on this machine
  after six rounds, `WorkspaceEdit/synthetic` and
  `/vehicle/reindex+diagnostics`, `GRPCParseFileUncached`,
  `ActionLoop/for1000`, `StateLoop/count5000`, the 1 000-step chain rows
  and the loader's stress rows — on each of them the bytes and
  allocations are the figure to read, and they are deterministic.

The machine this record was measured on is not the machine of the
previous records: it is 1.3–1.8× slower on the same code (`FormatEdits`
on 0.9.2: 512 µs there, 639 µs here; `LoadModel/4000` 458 ms against
710 ms), with the same allocations to the unit. Every time in this record
is from runs of both revisions on this machine, interleaved where it
mattered; no time is compared across machines. Readers comparing against
the 0.9.2 record's absolute figures should scale accordingly or, better,
read the allocation columns.

## What this does not measure

- The new surfaces with no 0.9.2 counterpart — the view renderers, the
  graphs export, the api-json loader, the browser engine, the REPL's
  repository and notebook commands, the Python and Node clients' new
  calls — have benchmarks only where listed above (`ImportValues`, the
  exact-numeric loops); the rest are covered by their test suites, not by
  this record.
- The compiled-calc C target is measured on `Hypot` only; the three
  Integer calcs it refuses have no 0.10.0 figure, so the C target's speed
  on 0.10.0 is read from one row.
- The stress constellation's per-document analysis
  (`AnalyzeSplitPerDocument`) has no 0.10.0 figure at any size, and its
  `PlaneResidency` row has no figure on either revision.
- Inherited action steps are measured only where the benchmarks'
  fixtures specialize an action (`LowerChain`, `FleetInstantiate`); a
  model whose actions redefine deep hierarchies of inherited steps has no
  row here.
- Everything ran on one 8-CPU virtual machine, slower than the previous
  records', with single-count spreads of 20–100% on many rows that the
  earlier machine held under 20%; the allocation columns are the
  machine-independent reading, and a one-CPU host should read the
  package figures rather than the whole-binary ones, as the previous
  records said.
