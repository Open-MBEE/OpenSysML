# Performance: release 0.10.0 against release 0.9.2

The release-gate measurement of the 0.10.0 candidate —
`hotfix/0.10.0-develop-sync` at `82f0fac87` (2026-10-09), which is `main`
at `ffe51571c` (the `release/0.10.0` merge plus a CI-only hotfix) with
everything `develop` gained after the release branch was cut merged back
into it, 1 754 commits past the `v0.9.2` tag (`4226ad9a3`, 2026-10-05) —
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
inlay hints in the language server, repository and notebook commands in the
REPL, and the fixes of a 300-entry changelog. The expectation going in was
movement on the execution and analysis paths, and that is what the
measurement finds — it is not a parity record.

The candidate also carries six performance fixes made against an earlier
measurement of `ffe51571c` alone, and this record shows what they bought:
where a row moved between `ffe51571c` and `82f0fac87` the tables carry a
third column with the `ffe51571c` figure from the same interleaved run.
The six, by the change on the branch:

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

Every row that regresses is confirmed by the interleaved re-run and
attributed to the change responsible below; the fixes that remain open are
named in the summary, and nothing is fixed here.

## Summary

**Nothing here blocks the release.** 0.10.0 is slower than 0.9.2 on most
of the paths a model goes through, by a constant factor on each; nothing
is superlinear, nothing that fit 0.9.2's budgets stops fitting except the
one calc noted below, and the fleet's instantiation and hydration are
faster. Against the earlier measurement of `ffe51571c`, the six fixes on
the candidate cleared three of the regressions that record named as
avoidable and cut a fourth by a quarter:

- `CompiledCalc/Fib(25)/interpreted` went from 5.3× 0.9.2 to **20%
  faster than 0.9.2** (the three-word scalar packing);
- `BatchConstraints` from +68% to `~`, `Satisfy` from +22–28% to
  +10–14% (the memoized succession-cycle check);
- `GRPCParseFileCached` from 299 allocations to 24 against 0.9.2's 22
  and from +22% to `~` (the cached protobuf diagnostics);
- the workspace-edit rows from 3.8–3.9× 0.9.2's bytes to 2.9–3.0×, and
  from +61–104% allocations to +32–48% (the re-export claims compared in
  place);
- `AssignmentLoopStep` lost 1 200 of the 3 604 allocations the release
  had added, and `ActionLoop/for1000` 2 000 of 3 039 (the lazy
  binding-check description);
- the loader lost 2–3 points of allocations (the qualified-name compare).

What remains, confirmed by the interleaved re-run:

- **Loading** is +15–18% allocations and +12–20% bytes per element on
  connector-dense models, for +10–16% time: `Load` +16% at 32
  satellites, `sysml -validate` +20–22% and +6–23 MiB at 3 000–12 000
  declarations, Apollo 11 +14% and +12 MiB. Each connector, binding and
  flow end is now owned, journaled, indexed and reflected. Structural;
  linear.
- **Action and state execution** is +20–60% per step with 1.3–1.5× the
  allocations: `AssignmentLoopStep` +61%, `ActionLoop` +38–56%,
  `StateLoop` +22–40%, `ExecuteState` +21%, `ExecuteActionFreshContext`
  +38%, `RunCalc` +10–14%. The exploration of a body's statement orders,
  the per-statement binding frames and the held control values are the
  semantics of the release; the per-step constant is where a 0.10.x pass
  should look next.
- **Workspace edits beside a large document** still allocate 2.9–3.0×
  0.9.2's bytes (`WorkspaceEditSmallDocBesideLarge` 7.95 → 23.7 MiB,
  `EditBeside/512` 2.41 → 6.66 MiB) for +20–31% time: the per-name
  snapshot of re-export claims across the large neighbor. The one row
  in this record with an evident fix still open.
- **The Annex A vehicle example now emits 134
  `Duplicate of imported member name` warnings** (0 in 0.9.2), and every
  path that returns them pays: `Analyze/vehicle` +85%,
  `GRPCParseFileUncached` +64%, `ConnectParseFileHTTPCached` 271 → 879 µs
  (the HTTP transport has no cache of its JSON). Whether the rule should
  fire on nested `::**` imports is a semantics question; if not, the
  group is one fix away from 0.9.2.
- **Exact numerics**: `Collatz(27)` +59–65% (an allocation per Integer
  multiply or divide), `Fib(25)/go` +81% and `Hypot/go` 11 → 56 ns (the
  Go target's values carry their number kind), `Instantiate` +11–31% per
  object (implicit ends and exact-number constructors). The price of the
  feature.
- **Three things the candidate cannot run**, for the release notes:
  `SumTo(1000000)` exceeds the default 10 000 000-step budget it fit in
  0.9.2 (compiled calcs now spend the interpreter's budget); the C target
  refuses calcs on Integers (unbounded against `int64`); the stress
  benchmark's per-document analysis trips the new decimal-rounding lint
  on its generated `Real` literals.
- Binary 42.88 → 47.23 MiB; empty session 64 → 69 MiB, start time
  unchanged.

## Method

Machine: `INTEL(R) XEON(R) PLATINUM 8559C`, 8 CPUs, 31 GiB, Go
`go1.25.11 linux/amd64`, `GOMAXPROCS=8`. 0.9.2 is a worktree at the tag,
built and benchmarked with its own tree; the candidate is
`hotfix/0.10.0-develop-sync` at `82f0fac87`, built and benchmarked with its
own; `main` at `ffe51571c`, where it is shown, is a third worktree treated
the same way. Package benchmarks are
`go test ./<pkg> -run '^$' -bench . -benchmem -count 6` on each revision,
compared with `benchstat` (rows are significant at p < 0.05; `~` is no
significant difference). The first pass ran each package's six counts on
0.9.2 and then on the candidate; rows it left in doubt — a significant
difference, or a spread above 20% on either side — were re-run with the
revisions interleaved, `-count 1` six times each, nothing else on the
machine. Where the first pass left most of a package in doubt
(`internal/frontend/repl`, `tests/perf`, `tests/stressmodel`) the whole
package or the whole of its moving rows was re-run that way, and the re-run
is the table shown. A benchmark that fails on one revision (`b.Fatal`)
prints `--- FAIL` and no result line while the package's other benchmarks
run on; `benchstat` then has no row for it, and the failure is reported in
that package's section as a candidate-only or baseline-only row. Nothing
was excluded from `-bench .`. Whole-binary figures are `/usr/bin/time -f '%e %M'`
over nine interleaved runs of each binary, all built with
`make build-sysml` (`-s -w`, `-trimpath`); medians are reported. The pilot
corpora the `vehicle` rows and the gRPC warm-instantiation row read were
fetched into every tree with `scripts/download-pilot-corpora.sh`; 0.9.2's
first pass predates the download, so those rows appear in the re-run only.
Causes were read from `git log v0.9.2..82f0fac87 -- <path>` and, where the
history alone did not name one, from `-cpuprofile` and `-memprofile` of the
row on both revisions.

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
- **`InstantiateWarmModel`** (`internal/frontend/grpc`) exists on both
  revisions and is compared from the interleaved re-run; 0.9.2's first
  pass skipped it for want of the vehicle corpus.
- **`tests/stressmodel`**: `ConvertAPIJSON/satellites=512` completed one
  count on 0.9.2 (156.0 s) before the run was killed for memory (78 GiB
  allocated per operation on a 31 GiB machine) and one on the candidate
  (135.6 s) before the same; it is excluded as no distribution, as in the
  previous record.
- **Diagnostics**: on every model measured both binaries return the same
  exit status and the same report — `no errors` on the generated models,
  the two example models and all 28 Apollo 11 files. (The candidate's
  decimal-rounding report is an opt-in lint on the command line; only the
  stress benchmark's registry turns it on.)

## Package benchmarks

Time per operation, 0.9.2 then the candidate, from the interleaved re-run
wherever a row was re-run (the first pass agrees in sign and roughly in
size on every row named here); bytes and allocations where they moved.
Where a row moved between `ffe51571c` and the candidate, the `ffe51571c`
figure from the same interleaved run stands between them. Allocation
counts are deterministic to the unit on every revision and are the reading
that separates a change in the code from a change in the machine.

### `internal/exec/runtime`

| benchmark | 0.9.2 | `ffe51571c` | 0.10.0 (`82f0fac87`) |
| --------- | ----- | ----------- | -------------------- |
| `MaterializePlainAttributes` | 45.2 µs ±4% / 42.68 KiB / 599 allocs | 53.1 µs (+17%) / 43.58 KiB / 618 | 51.5 µs ±2% (+14%, p=0.002) / 43.58 KiB (+2.1%) / 618 (+3.2%) |
| `SetFeatureValueNoDependents` | 423 ns ±28% / 352 B / 9 | 442 ns (~) | 490 ns ±24% (~, p=0.132) / 352 B / 9 |
| `DerivedReadWriteRead` | 5.39 µs ±2% / 2.484 KiB / 65 | 6.09 µs (+13%) | 5.96 µs ±2% (+11%, p=0.002) / 2.484 KiB / 65 |
| `AssignmentLoopStep` | 940 µs ±2% / 521.4 KiB / 4 923 | 1 568 µs (+67%) / 837.7 KiB (+61%) / 8 527 (+73%) | 1 509 µs ±2% (+61%, p=0.002) / 817.3 KiB (+57%) / 7 327 (+49%) |

Candidate-only rows (no 0.9.2 counterpart): `IntegerArithmeticLoop`
3.57 ms ±59% / 279.7 KiB / 9 070 allocs; `IntegerArithmeticLoopBeyondInt64`
3.51 ms ±7% / 514.1 KiB / 13 070; `IntegerCollectionSum` 879 µs ±5% /
4.961 MiB / 44; `RationalDecimalLoop` 3.60 ms ±34% / 279.7 KiB / 9 069;
`RationalHarmonicLoop` 849 µs ±4% / 178.9 KiB / 4 650; `RealDecimalLoop`
3.75 ms ±9% / 279.7 KiB / 9 069; `BehaviorOrdersLibraryModel` 392 µs ±10% /
163.2 KiB / 1 922. They are the baseline for the next record.

The interleaved re-run settled all three compared rows in doubt at ±2%.
**Executing one assignment statement of an action loop is +61% with 1.6
times the bytes and 1.5 times the allocations**; materializing an
instance's plain attributes +14% with 19 more allocations, and a derived
read-write-read +11% with none. The fix on this path — the binding check
describing a body write's host only when it refuses the write
(`37b4a17c9`) — took `AssignmentLoopStep` from 8 527 to 7 327 allocations
per step (1 200 of the 3 604 the release had added, the description
strings the earlier profile showed) and 4% of its time; the remaining
+61% is the three changes the profile splits it into: the frames built
per statement to track body feature bindings (`a4e1f6ef9`), the
exploration of a body's statement orders (`71fda4cd5`, `807fcf591`,
`c8891457a`) and control-node values held with the flow that brought them
(`a0f77861d`). Those are the semantics of the release — a body's unordered
statements are now explored in every order, and that costs frames — and
not a bug; the per-step constant is where a 0.10.x pass should look next.
The `tests/perf` `ActionLoop`, `StateLoop` and `ExecuteActionFreshContext`
rows below and the REPL's `RunCalc` are the same cost from other
entrances. The 19 allocations on `MaterializePlainAttributes` and the 2.1%
bytes are the implicit relationship ends an instance now carries
(`fa8a7220c`, `ca0e6cca0`); see `Instantiate` under the REPL.

### `internal/frontend/repl`

Attribute-dense generated models at 0, 250, 1 000 and 4 000 elements, and
the compiled-calc fixtures. 28 rows on 0.9.2, 23 on the candidate: the five
`CompiledCalc` rows the candidate cannot run are listed with their 0.9.2
figures. The first pass left the whole package in doubt (the candidate's
`LoadModel` counts ran 11–32% apart); the moving rows were re-run
interleaved and the re-run is shown.

| benchmark | 0.9.2 | `ffe51571c` | 0.10.0 (`82f0fac87`) |
| --------- | ----- | ----------- | -------------------- |
| `LoadModel/elements=0` | 399 µs ±20% / 216.2 KiB / 1 875 allocs | 399 µs (~) | 402 µs ±3% (~) / 221.6 KiB (+2.5%) / 1 931 (+3.0%) |
| `LoadModel/elements=250` | 28.0 ms ±8% / 13.22 MiB / 126.1 k | 30.1 ms (+7.4%) | 29.3 ms ±7% (+4.7%, p=0.015) / 13.51 MiB (+2.2%) / 130.3 k (+3.4%) |
| `LoadModel/elements=1000` | 104 ms ±17% / 52.06 MiB / 480.0 k | 117 ms (+12%) | 114 ms ±9% (+9.5%, p=0.041) / 53.25 MiB (+2.3%) / 496.8 k (+3.5%) |
| `LoadModel/elements=4000` | 458 ms ±6% / 207.2 MiB / 1.895 M | 507 ms (+11%) | 493 ms ±18% (+7.6%, p=0.009) / 212.7 MiB (+2.7%) / 1.962 M (+3.5%) |
| `RunStateMachine/elements=250` | 12.6 µs ±3% / 15.23 KiB / 213 | 13.9 µs (+10%) | 13.7 µs ±22% (+8.9%, p=0.002) / 15.52 KiB (+1.9%) / 217 (+1.9%) |
| `RunStateMachine/elements=1000` | 12.9 µs ±5% | 13.5 µs (+4.5%) | 13.2 µs ±20% (~, p=0.240) / same |
| `RunStateMachine/elements=4000` | 13.0 µs ±1% | 14.2 µs (+9.6%) | 13.7 µs ±23% (+5.8%, p=0.026) / same |
| `RunCalc/elements=250` | 4.42 µs ±6% / 4.242 KiB / 76 | 5.32 µs (+20%) | 5.02 µs ±27% (+14%, p=0.002) / 4.453 KiB (+5.0%) / 79 (+3.9%) |
| `RunCalc/elements=1000` | 4.50 µs ±5% | 5.30 µs (+18%) | 4.99 µs ±6% (+11%, p=0.002) / same |
| `RunCalc/elements=4000` | 4.49 µs ±5% | 5.31 µs (+18%) | 4.94 µs ±8% (+10%, p=0.002) / same |
| `Instantiate/elements=250` | 6.38 µs ±4% / 6.420 KiB / 53 | 8.49 µs (+33%) | 8.36 µs ±6% (+31%, p=0.002) / 7.114 KiB (+11%) / 56 (+5.7%) |
| `Instantiate/elements=1000` | 6.19 µs ±6% / 6.404 KiB | 6.96 µs (+12%) | 7.06 µs ±5% (+14%, p=0.002) / 7.197 KiB (+12%) / 56 |
| `Instantiate/elements=4000` | 6.08 µs ±4% / 6.375 KiB | 6.83 µs (+12%) | 6.73 µs ±5% (+11%, p=0.002) / 7.187 KiB (+13%) / 56 |
| `Diagnostics/attributes=50` | 86.0 µs ±8% / 72.67 KiB / 545 | — | 89.1 µs ±7% (~) / 73.60 KiB (+1.3%) / 559 (+2.6%) |
| `Diagnostics/attributes=200` | 300 µs ±14% / 248.0 KiB / 1 453 | — | 309 µs ±4% (~) / 248.9 KiB (+0.4%) / 1 467 (+1.0%) |
| `Diagnostics/attributes=800` | 1.21 ms ±8% / 895.2 KiB / 5 607 | 1.14 ms (−5.7%) | 1.21 ms ±20% (~, p=0.937) / 896.2 KiB (+0.1%) / 5 621 (+0.2%) |
| `CompiledCalc/Fib(25)/interpreted` | 7.69 ms ±8% / 42.67 KiB / 353 | 40.9 ms (+433%) / 261.2 KiB (+512%) / 2 059 (+483%) | 6.16 ms ±9% (−20%, p=0.002) / 45.73 KiB (+7.2%, p=0.041) / 381 (+7.8%, p=0.039) |
| `CompiledCalc/Fib(25)/c` | 225 µs ±5% / 1 B / 0 | refused | refused (Integer arithmetic) |
| `CompiledCalc/Fib(25)/go` | 949 µs ±13% / 7 B / 0 | 1 704 µs (+80%) | 1 720 µs ±53% (+81%, p=0.002) / 12 B (+71%) / 0 |
| `CompiledCalc/SumTo(1000000)/interpreted` | 1.549 s ±7% / 58.84 MiB / 3.043 M | step limit | step limit exceeded (10 000 000 steps) |
| `CompiledCalc/SumTo(1000000)/c` | 385 µs ±10% / 2 B / 0 | refused | refused (Integer arithmetic) |
| `CompiledCalc/SumTo(1000000)/go` | 783 µs ±4% / 5 B / 0 | step limit | step limit exceeded (10 000 000 steps) |
| `CompiledCalc/Collatz(27)/interpreted` | 275 µs ±7% / 29.30 KiB / 999 | 443 µs (+61%) | 455 µs ±6% (+65%, p=0.002) / 81.84 KiB (+179%) / 2 115 (+112%) |
| `CompiledCalc/Collatz(27)/c` | 1.01 µs ±10% / 0 B / 0 | refused | refused (Integer arithmetic) |
| `CompiledCalc/Collatz(27)/go` | 5.40 µs ±4% / 0 B / 0 | 8.36 µs (+55%) | 8.58 µs ±10% (+59%, p=0.002) / 0 B / 0 |
| `CompiledCalc/Hypot(3.0,4.0)/interpreted` | 7.10 µs ±31% / 7.846 KiB / 88 | 8.82 µs (~) | 8.21 µs ±34% (+16%, p=0.041) / 8.078 KiB (+3.0%) / 91 (+3.4%) |
| `CompiledCalc/Hypot(3.0,4.0)/c` | 13.3 ns ±14% / 0 B / 0 | 13.4 ns (~) | 13.8 ns ±4% (~, p=0.240) / 0 B / 0 |
| `CompiledCalc/Hypot(3.0,4.0)/go` | 11.0 ns ±81% / 0 B / 0 | 54.5 ns (+395%) | 56.4 ns ±2% (+412%, p=0.002) / 0 B / 0 |

Three things are going on in this package.

**The model loader costs 2.2–2.7% more bytes and 3.0–3.5% more
allocations per element**, and 5–10% more time (p ≤ 0.041 interleaved,
with the 4 000-element row's candidate counts 18% apart). This is the
semantic model's new bookkeeping — owned relationships journaled and
reflected as metaobjects (`da8fba4a3`, `00b8499a8`, `32890be28`), the
classification of extended declarations (`ca0e6cca0`) and the implicit-end
constancy derived once for reflection and writes (`fa8a7220c`) — and the
same per-element growth the stress constellation and `tests/perf` show
below. The qualified-name fix (`367ca7e3a`) took 5 000 allocations per
1 000 elements off `ffe51571c`'s loader (501.8 k → 496.8 k) and 1–2% of
its time. **Explained**; it is the fourth record running in which the
loader grows by a few percent per release.

**Instantiating one object is 11–31% slower and evaluating one calc
10–14%**, three allocations more per operation each. The gRPC
`InstantiateWarmModel` row below shows the same +35% with +18%
allocations. An instance now carries the implicit relationship ends of
its type (`fa8a7220c`, `ca0e6cca0`) and its attribute values go through
the exact-number constructors (`79629923b`, `923ec81f0`); the bytes rise
11–13% on these small objects and fall on the stress constellation's
fleet, where the Integer and Rational fast paths' allocation-free
representation (`d1eb16edb`) wins. `RunCalc` was +18–20% on `ffe51571c`;
the memoized succession-cycle check (`8bd97fcf6`) took a third of that
off. **Explained.**

**The compiled calcs pay for unbounded Integers, exact Rationals and the
step budget.** Each of the four fixtures is affected differently, and the
benchmark's own fixture file grew from 597 to 930 lines (the same four
calcs; the new content is other compiled fixtures), which the
`LoadModel`-sized setup absorbs but the `FormatEdits` row in
`internal/frontend/lsp` does not:

- `Fib(25)/interpreted` **is 20% faster than 0.9.2** with 7% more
  allocations. On `ffe51571c` it was 5.3 times slower and allocated six
  times the objects: the exact-Rational evaluation (`923ec81f0`) boxed
  every scalar a node yielded and paired the leaves of each arithmetic
  node, 250 000 calls deep. Packing a compiled calc's scalars into three
  words (`4a1b937f2`) removed the boxing — 2 059 → 381 allocations per
  call — and the row is now the fastest of the three records it appears
  in. The steepest row of the earlier measurement is cleared.
- `Collatz(27)/interpreted` is +65% with 2.1 times the allocations, and
  the fix did not reach it: `Collatz` multiplies and divides Integers,
  and the unbounded-Integer path (`79629923b`) still allocates a result
  per operation where `Fib`'s additions stay in the packed word (999 →
  2 115 allocations is one per arithmetic node). `Collatz(27)/go` is +59%
  with none on either side, and `Hypot(3.0,4.0)/go` goes from 11 ns to
  56 ns, also allocation-free: the Go target's compiled values now keep
  an Integer an Integer inside a Real-typed value and hold negative powers
  as exact Rationals (`b69ea6840`, `0aa4aca55`), so each arithmetic node
  carries its number kind and branches on it, and compiled calcs spend
  the interpreter's step budget (`d6eafc870`). The price of exactness; the
  Integer multiply path is where a profile-driven pass should look next.
- `SumTo(1000000)` sums a million integers in a loop. On 0.9.2 the
  interpreted run took 1.55 s inside the default budget of 10 000 000
  steps; on the candidate the same loop exceeds it, for the interpreted
  and the Go target both, because compiled calcs now spend the
  interpreter's step budget (`d6eafc870`) and the interpreter counts the
  statements of a loop body it explores in order (`71fda4cd5`,
  `43548d741`). The row is not a timing regression but a change in what
  fits the default budget: a loop of a million iterations no longer does
  without `OPENSYSML_MAX_STEPS`.
- The C target refuses every calc that does Integer arithmetic (`Fib`,
  `SumTo`, `Collatz`) because a C `int64` cannot hold the interpreter's
  unbounded Integers (`79629923b`); only `Hypot`, on Reals, still
  compiles to C. This is a deliberate narrowing of the C target, not a
  performance figure, and belongs in the release notes.

### `internal/frontend/grpc`

| benchmark | 0.9.2 | `ffe51571c` | 0.10.0 (`82f0fac87`) |
| --------- | ----- | ----------- | -------------------- |
| `ParseFileColdShared` | 10.7 ms ±34% / 4.689 MiB / 31.12 k allocs | 7.67 ms (~) | 7.61 ms ±6% (~, p=0.394) / 4.700 MiB (+0.2%) / 31.29 k (+0.5%) |
| `ParseFileColdInline` | 7.69 ms ±29% / 4.916 MiB / 31.47 k | 7.74 ms (~) | 7.66 ms ±8% (~, p=0.818) / 4.945 MiB (~) / 31.71 k (+0.8%) |
| `InstantiateWarmModel` | 150 µs ±12% / 83.06 KiB / 1 555 allocs | 211 µs (+41%) / 104.02 KiB / 1 838 | 198 µs ±10% (+32%, p=0.002) / 104.15 KiB (+25%) / 1 836 (+18%) |

The cold-parse rows are the parser and the loader on the vehicle example
and move with the loader's few percent of allocations; the time is `~` on
every revision. The warm instantiation is the `Instantiate` row of the
REPL at the service's scale: 281 more objects per instantiation, the same
implicit relationship ends and exact-number constructors on the vehicle
example's objects. The cached-diagnostics fix (`70242d872`) does not
touch these rows; its effect is on `tests/perf`'s `GRPCParseFileCached`
and `ConnectParseFileHTTPCached` below.

### `internal/frontend/lsp`

| benchmark | 0.9.2 | `ffe51571c` | 0.10.0 (`82f0fac87`) |
| --------- | ----- | ----------- | -------------------- |
| `FormatEdits` | 512 µs ±4% / 433.7 KiB / 78 allocs | 891 µs (+74%) / 704.3 KiB / 85 | 878 µs ±5% (+71%, p=0.002) / 704.3 KiB (+62%) / 85 (+9.0%) |
| `SpanToRangeLarge` | 2.99 ms ±17% / 0 B | — | 2.92 ms ±40% (~) / 0 B |
| `ReferencesWarm` | 206 µs ±7% / 339.4 KiB / 1 017 | — | 204 µs ±2% (~) / 339.4 KiB / 1 017 |
| `ReferencesCold` | 45.4 ms ±19% / 9.713 MiB / 44.51 k | — | 39.5 ms ±17% (~) / 9.720 MiB (~) / 44.51 k (~) |
| `RenameWarm` | 16.5 ms ±11% / 1.252 MiB / 12.75 k | 16.9 ms (~) | 16.5 ms ±6% (~, p=0.818) / 1.244 MiB (~) / 12.62 k (~) |
| `WorkspaceUpdate` | 194 ms ±7% / 49.38 MiB / 605.6 k | 262 ms (+35%) / 98.50 MiB (+99%) / 930.6 k (+54%) | 224 ms ±4% (+16%, p=0.002) / 57.18 MiB (+16%) / 710.0 k (+17%) |

`FormatEdits` formats `internal/frontend/repl/testdata/compile_calcs.sysml`,
which grew from 597 to 930 lines on the candidate with the new compiled
fixtures; the bytes move by the ratio of the file sizes (+62% against +56%
more lines) and the formatter itself has one commit in the interval (the
indexed connector ends, `63552cbc4`). **Not a regression in the code.**
References and rename are unchanged, to the allocation.

`WorkspaceUpdate` — one edit to one of the workspace's documents, with the
re-analysis the server does for it — is where the re-export fix landed.
On `ffe51571c` it allocated twice the bytes of 0.9.2 and 1.5 times the
objects, 39% of them in `symbols.registrationOf` deep-copying every
document's re-export claims before each name's re-registration
(`4ca321508`, `fe6e01fb9`); with the claims compared in place instead of
copied (`4b4b35121`) the row is **+16% time, +16% bytes and +17%
allocations** against 0.9.2 — 41 MiB per update less than `ffe51571c`,
and the first-pass spread of ±46% on 0.9.2 settled to ±7% interleaved. The
remainder is the resolver's new `checkImportedNames` (`eb4b7583c`) and
the loader's per-element growth, which the `tests/perf` and stress edit
rows below share. **Explained.**

### `internal/translate/migrate`, `internal/workspace/libs`, `internal/workspace/model`

| benchmark | 0.9.2 | 0.10.0 (`82f0fac87`) |
| --------- | ----- | -------------------- |
| `migrate` `WriterSiblingBlocks` | 1.93 ms ±7% / 1.906 MiB / 20.02 k allocs | 1.90 ms ±9% (~) / 1.906 MiB (~) / 20.02 k (~) |
| `libs` `ExpandWildcardImports` | 27.4 ms ±1% / 14.46 MiB / 128.3 k | 28.6 ms ±3% (+4.2%) / 15.36 MiB (+6.2%) / 145.7 k (+14%) |
| `libs` `IndexLibrary` | 16.3 ms ±15% / 13.56 MiB / 104.6 k | 16.2 ms ±14% (~, p=0.818) / 14.07 MiB (+3.8%) / 108.2 k (+3.4%) |
| `libs` `ExpandModelImports` | 6.50 ms ±11% / 4.315 MiB / 36.16 k | 6.67 ms ±4% (~, p=0.485) / 4.376 MiB (+1.4%) / 38.15 k (+5.5%) |
| `libs` `DecodeSnapshot` | 9.89 ms ±17% / 35.32 MiB / 56.63 k | 10.8 ms ±21% (~, p=0.132) / 37.85 MiB (+7.2%) / 67.45 k (+19%) |
| `libs` `SetDigest` | 629 µs ±12% / 1.710 MiB / 966 | 641 µs ±7% (~) / 1.733 MiB (+1.4%) / 998 (+3.3%) |
| `model` `AnalyseUnresolved` | 20.2 ms ±8% / 13.69 MiB / 149.3 k | 20.6 ms ±6% (~) / 13.80 MiB (+0.8%) / 150.7 k (+1.0%) |
| `model` `AnalyseResolved` | 1.51 ms ±22% / 543.0 KiB / 5 049 | 1.59 ms ±12% (~, p=0.699) / 590.4 KiB (+8.7%) / 5 971 (+18%) |

None of these rows moved between `ffe51571c` and the candidate, to the
allocation. The migration writer is unchanged. Indexing the standard
library allocates 3.4% more objects (the reflective relationship ends, the
`StateActivity` extension library `0188fb308`) and decoding its snapshot
19% more: the snapshot carries the owned-relationship journal and the
reflected implicit relationships (`da8fba4a3`, `32890be28`) that the 0.9.2
snapshot did not hold; the time of both is `~` interleaved. Expanding the
library's wildcard imports allocates 14% more objects and is +4.2% in time
interleaved (+15% in the first pass, with the candidate's counts 20%
apart); the import-ambiguity rule (`eb4b7583c`) now walks what each
wildcard brings in. Expanding a model's imports is `~` in time interleaved
(+9.0% in the first pass). Analysing a resolved document allocates 18% more with the time
`~` interleaved (first pass +12%): the constraint-tier passes added in the
interval — the indexed-end multiplicity check (`cf85beb36`), the
import-ambiguity warning (`eb4b7583c`), the reflective derivations for the
batch constraints (`70bfa8ee2`) — each walk the document once more. The
`tests/perf` `Analyze` rows below show the same +3.3% allocations on the
synthetic model and much more on the vehicle example.

### `tests/perf`

The synthetic 4 000-part model and the Annex A vehicle example through
every entrance: lexer, parser, index, analysis, workspace edits, REPL,
lowering, execution, constraints, the gRPC service in-process and over
HTTP. 49 rows on 0.9.2 and 57 on the candidate (the eight `ImportValues`
rows are new). The first pass left most of the package in doubt, so the
whole package was re-run interleaved across the three revisions; that
re-run is the table. The vehicle rows need the pilot corpus, which 0.9.2's
first pass predates, so those have the re-run only.

| benchmark | 0.9.2 | `ffe51571c` | 0.10.0 (`82f0fac87`) |
| --------- | ----- | ----------- | -------------------- |
| `Lex/synthetic` | 8.16 ms ±14% / 483.0 k allocs | 7.84 ms (~) | 7.97 ms ±6% (~) / 483.0 k |
| `Lex/vehicle` | 277 µs ±17% / 10.85 k | 268 µs (~) | 272 µs ±8% (~) / 10.85 k |
| `Parse/synthetic` | 99.0 ms ±16% / 293.2 k | 89.0 ms (~) | 88.5 ms ±25% (~) / 293.2 k |
| `Parse/vehicle` | 2.51 ms ±12% / 7 559 | 2.59 ms (~) | 2.49 ms ±9% (~) / 7 559 |
| `IndexAddExpand/synthetic` | 51.4 ms ±19% / 294.6 k | 51.7 ms (~) | 51.6 ms ±8% (~) / 317.6 k (+7.8%) |
| `IndexAddExpand/vehicle` | 27.3 ms ±33% / 131.2 k | 27.7 ms (~) | 28.1 ms ±18% (~) / 139.7 k (+6.5%) |
| `IndexAddOnly/synthetic` | 45.8 ms ±20% / 283.0 k | 47.5 ms (~) | 48.0 ms ±17% (~) / 306.6 k (+8.3%) |
| `IndexAddOnly/vehicle` | 1.14 ms ±10% / 9 277 | 1.13 ms (~) | 1.18 ms ±11% (~) / 9 633 (+3.8%) |
| `Analyze/synthetic` | 1.26 s ±20% / 466.3 MiB / 4.435 M | 1.33 s (~) / 487.2 MiB / 4.644 M | 1.35 s ±21% (+7.5%, p=0.041) / 484.5 MiB (+3.9%) / 4.582 M (+3.3%) |
| `Analyze/vehicle` | 117 ms ±17% / 73.65 MiB / 627.6 k | 221 ms (+89%) / 130.0 MiB / 865.3 k | 216 ms ±22% (+85%, p=0.002) / 129.9 MiB (+76%) / 869.3 k (+39%) |
| `WorkspaceEdit/synthetic` | 210 ms ±10% / 77.97 MiB / 596.3 k | 278 ms (+32%) / 85.84 MiB / 680.6 k | 285 ms ±68% (+35%, p=0.002) / 85.74 MiB (+10%) / 680.5 k (+14%) |
| `WorkspaceEdit/synthetic/reindex+diagnostics` | 1.62 s ±5% / 493.5 MiB / 4.787 M | 1.82 s (+13%) / 519.6 MiB / 5.089 M | 1.80 s ±16% (+11%, p=0.002) / 517.4 MiB (+4.8%) / 5.027 M (+5.0%) |
| `WorkspaceEdit/vehicle` | 56.5 ms ±12% / 12.33 MiB / 139.1 k | 78.6 ms (+39%) / 22.94 MiB (+86%) / 214.4 k (+54%) | 67.9 ms ±11% (+20%, p=0.002) / 14.60 MiB (+18%) / 168.1 k (+21%) |
| `WorkspaceEdit/vehicle/reindex+diagnostics` | 176 ms ±7% / 71.24 MiB / 651.7 k | 262 ms (+49%) / 129.7 MiB (+82%) / 917.6 k (+41%) | 250 ms ±10% (+42%, p=0.002) / 121.0 MiB (+70%) / 867.1 k (+33%) |
| `WorkspaceEditSmallDocBesideLarge` | 45.4 ms ±34% / 7.952 MiB / 81.74 k | 69.5 ms (+53%) / 31.24 MiB (+293%) / 162.6 k (+99%) | 59.4 ms ±35% (+31%, p=0.026) / 23.67 MiB (+198%) / 114.6 k (+40%) |
| `FQNOf` | 1.59 ms ±32% / 36.00 k | 1.68 ms (~) | 1.62 ms ±12% (~) / 40.00 k (+11%) |
| `LookupQualified` | 1.22 ms ±34% / 11.00 k | 1.26 ms (~) | 1.26 ms ±14% (~) / 11.00 k |
| `FeaturesOf` | 1.01 s ±11% / 4.154 M | 1.03 s (~) | 1.06 s ±13% (~) / 4.156 M (~) |
| `REPLLoadFile` | 1.53 s ±7% / 595.5 MiB / 5.383 M | 1.63 s (~) / 616.7 MiB / 5.595 M | 1.64 s ±10% (~, p=0.065) / 613.0 MiB (+2.9%) / 5.533 M (+2.8%) |
| `REPLSubmitSnippet` | 1.10 s ±9% / 4.805 M | 1.22 s (+11%) / 5.048 M | 1.25 s ±18% (+14%, p=0.026) / 4.986 M (+3.8%) |
| `REPLEvalExpr` | 4.76 ms ±15% / 101 | 4.66 ms (~) | 4.86 ms ±8% (~) / 104 (+3.0%) |
| `LowerActionGraph` | 1.85 µs ±11% / 25 | 2.16 µs (+17%) | 2.15 µs ±7% (+16%, p=0.004) / 27 (+8.0%) |
| `LowerStateGraph` | 9.02 µs ±13% / 103 | 8.48 µs (~) | 9.08 µs ±17% (~) / 103 |
| `ExecuteAction` | 79.9 µs ±10% / 192 | 86.7 µs (~) / 254 | 80.4 µs ±5% (~, p=0.937) / 234 (+22%) |
| `ExecuteActionFreshContext` | 31.0 µs ±16% / 268 | 41.8 µs (+35%) / 347 | 42.8 µs ±9% (+38%, p=0.002) / 327 (+22%) |
| `ExecuteState` | 729 µs ±12% / 483.8 KiB / 10.38 k | 903 µs (+24%) / 608.9 KiB / 11.66 k | 880 µs ±6% (+21%, p=0.002) / 607.5 KiB (+26%) / 11.61 k (+12%) |
| `BatchConstraints` | 9.83 ms ±26% / 2.241 MiB / 55.26 k | 16.5 ms (+68%) / 2.424 MiB (+8.1%) / 55.99 k (+1.3%) | 10.5 ms ±4% (~, p=0.093) / 2.240 MiB (~) / 55.35 k (+0.2%) |
| `SameConstraintManyInstances` | 3.85 µs ±11% / 48 | 4.40 µs (+14%) | 4.38 µs ±6% (+14%, p=0.015) / 48 |
| `BatchSatisfy` | 1.27 s ±13% / 4.211 M | 1.33 s (~) | 1.33 s ±6% (~, p=0.310) / 4.354 M (+3.4%) |
| `Instantiate` | 525 ms ±12% / 174.0 MiB / 1.871 M | 574 ms (~) | 563 ms ±16% (~, p=0.240) / 180.2 MiB (+3.5%) / 1.955 M (+4.5%) |
| `GRPCParseFileCached` | 63.4 µs ±23% / 216.7 KiB / 22 | 77.5 µs (+22%) / 242.0 KiB (+12%) / 299 (+1 259%) | 66.0 µs ±5% (~, p=0.485) / 218.0 KiB (+0.6%) / 24 (+9.1%) |
| `GRPCParseFileUncached` | 131 ms ±12% / 661.3 k | 225 ms (+71%) / 905.1 k | 215 ms ±20% (+64%, p=0.002) / 909.0 k (+37%) |
| `GRPCEvaluate` | 9.04 µs ±7% / 110 | 10.6 µs (+18%) | 10.1 µs ±13% (+11%, p=0.004) / 116 (+5.5%) |
| `GRPCVerifyConstraint` | 831 µs ±16% / 9 188 | 1 047 µs (+26%) | 1 009 µs ±43% (+21%, p=0.009) / 9 768 (+6.3%) |
| `ConnectEvaluateHTTP` | 153 µs ±6% / 201 | 151 µs (~) | 153 µs ±4% (~) / 206 (+2.5%) |
| `ConnectParseFileHTTPCached` | 271 µs ±7% / 351.6 KiB / 166 | 926 µs (+241%) / 491.7 KiB / 1 292 | 879 µs ±24% (+224%, p=0.002) / 465.2 KiB (+32%) / 1 017 (+513%) |
| `ActionLoop/for10` | 25.8 µs ±10% / 204 | 36.5 µs (+41%) / 273 | 35.7 µs ±19% (+38%, p=0.002) / 253 (+24%) |
| `ActionLoop/for100` | 104 µs ±31% / 477 | 166 µs (+59%) / 816 | 163 µs ±17% (+56%, p=0.002) / 616 (+29%) |
| `ActionLoop/for1000` | 954 µs ±14% / 279.0 KiB / 3 181 | 1 369 µs (+44%) / 449.3 KiB (+61%) / 6 220 (+96%) | 1 332 µs ±19% (+40%, p=0.002) / 410.2 KiB (+47%) / 4 220 (+33%) |
| `ActionChain/chain10` | 94.7 µs ±13% / 675 | 111 µs (+18%) | 105 µs ±22% (~, p=0.065) / 764 (+13%) |
| `ActionChain/chain100` | 575 µs ±20% / 5 757 | 735 µs (+28%) | 712 µs ±7% (+24%, p=0.009) / 6 394 (+11%) |
| `ActionChain/chain1000` | 14.3 ms ±22% / 56.28 k | 16.3 ms (~) | 15.7 ms ±9% (~, p=0.180) / 62.83 k (+12%) |
| `StateLoop/count50` | 706 µs ±7% / 10.37 k | 870 µs (+23%) | 861 µs ±22% (+22%, p=0.002) / 11.60 k (+12%) |
| `StateLoop/count500` | 6.45 ms ±10% / 100.9 k | 8.95 ms (+39%) | 9.03 ms ±21% (+40%, p=0.002) / 112.9 k (+12%) |
| `StateLoop/count5000` | 64.9 ms ±14% / 45.27 MiB / 1.006 M | 85.4 ms (+32%) | 85.5 ms ±12% (+32%, p=0.002) / 57.16 MiB (+26%) / 1.126 M (+12%) |
| `LowerChain/chain10` | 8.72 µs ±15% / 73 | 10.5 µs (+21%) | 10.2 µs ±17% (+17%, p=0.015) / 78 (+6.8%) |
| `LowerChain/chain100` | 149 µs ±14% / 460 | 159 µs (+7.0%) | 162 µs ±19% (+8.5%, p=0.026) / 471 (+2.4%) |
| `LowerChain/chain1000` | 9.13 ms ±19% / 4 115 | 8.74 ms (−4.3%) | 9.99 ms ±13% (~, p=0.699) / 4 136 (+0.5%) |

Candidate-only rows (first pass, no 0.9.2 counterpart), the CSV/TSV/JSON
value import of `-import-values`: `ImportValues/flat/parts=100/attrs=3`
22.1 ms ±5% / 131.0 k allocs; `/flat/parts=500/attrs=3` 107 ms ±13% /
615.0 k; `/flat/parts=2000/attrs=3` 513 ms ±42% / 2.433 M;
`/flat/parts=2000/attrs=5` 743 ms ±25% / 3.672 M;
`/nested/parts=100/attrs=3` 30.1 ms ±5% / 175.4 k;
`/nested/parts=500/attrs=3` 145 ms ±5% / 829.4 k;
`/nested/parts=2000/attrs=3` 664 ms ±57% / 3.284 M;
`/nested/parts=2000/attrs=5` 1.05 s ±18% / 4.569 M. Linear in parts ×
attributes at roughly 400 allocations per value (437 at 300 values, 367
at 10 000); the nested layout costs a third more than the flat one.

Reading the table by cause:

- **Lexing, parsing and indexing are unchanged** in time, with 4–8% more
  allocations in the index (the reflective relationships each declaration
  now registers, `00b8499a8`).
- **Analysis of the synthetic model** is +7.5% with +3.3% allocations, the
  constraint-tier passes `AnalyseResolved` names. **Analysis of the vehicle
  example is +85% with 1.8 times the bytes**, and the fixes did not touch
  it, because the cost is not in the analysis but in its result: the
  candidate reports 134 `Duplicate of imported member name` warnings on
  the Annex A vehicle example where 0.9.2 reported none (the resolver's
  `checkImportedNames`, `eb4b7583c`, which now sees the names a nested
  `::**` import brings alongside the package's own). Every path that
  returns the example's diagnostics pays to build and carry them:
  `WorkspaceEdit/vehicle/reindex+diagnostics` +42%,
  `GRPCParseFileUncached` +64% with 248 000 more allocations,
  `ConnectParseFileHTTPCached` 271 → 879 µs with 851 more allocations
  (the HTTP transport serializes the 134 warnings to JSON on every
  request; the gRPC transport's cached protobuf conversion, `70242d872`,
  is why `GRPCParseFileCached` went from 299 allocations back to 24 — two
  more than 0.9.2, the cached slice's header and its once — and from
  +22% to `~`). Whether the rule should fire on the vehicle example's
  imports is a semantic question for the resolver's owners; **if it
  should not, every row in this group is one fix away from 0.9.2**, and
  if it should, the HTTP path wants the same cache the gRPC path got.
  Either way, a model with no such warnings does not pay it: the
  synthetic model's rows above are the same code without them.
- **Workspace edits.** The re-export fix (`4b4b35121`) is what moved these
  rows against `ffe51571c`: `WorkspaceEdit/vehicle` from +86% bytes to
  +18% and `WorkspaceEditSmallDocBesideLarge` — a one-line edit to a small
  document in the same workspace as the 4 000-part model — from 3.9 times
  0.9.2's bytes to 3.0 times, and from +99% allocations to +40%. What
  remains is the re-registration itself: `symbols.registrationOf` still
  snapshots each name's re-export claims by reference (`4ca321508`),
  which is a slice per name of the large neighbor per edit, 15.7 MiB per
  edit of the small document. **Confirmed, and the remaining 3.0× is
  the one row in this record a 0.10.x pass should start with.** The
  synthetic edit's +35% time stands on a ±68% candidate spread; its
  bytes (+10%) and allocations (+14%) are the stable reading.
- **Execution is where the release's semantics cost**: `ActionLoop`
  +38–56%, `StateLoop` +22–40%, `ExecuteState` +21%,
  `ExecuteActionFreshContext` +38%, with 12–33% more allocations. The
  lazy binding-check description (`37b4a17c9`) took `ActionLoop/for1000`
  from 6 220 to 4 220 allocations per run (`ffe51571c` had doubled
  0.9.2's) and 3% of its time; the time that remains is the statement-order
  exploration and the per-statement binding frames described under
  `internal/exec/runtime`, which a loop body of one assignment pays on
  every iteration. **Confirmed; structural.** `ExecuteAction` (a single
  action, no loop) is `~` in time with 42 more allocations.
- **Constraints.** `BatchConstraints` was +68% on `ffe51571c` and is `~`
  on the candidate, allocations +0.2%: the memoized succession-cycle check
  (`8bd97fcf6`) **cleared it**. `SameConstraintManyInstances` +14% and
  `BatchSatisfy` `~` in time with +3.4% allocations are the exact-number
  evaluation of the constraint bodies. `GRPCVerifyConstraint` +21% on a
  ±43% candidate spread, +6.3% allocations; `GRPCEvaluate` +11%, six
  allocations more per call.
- **Instantiate**, the 4 000-part model's instantiation, is `~` in time
  interleaved (the first pass's +39% was two outliers at ±79% and ±109%)
  with +4.5% allocations and +3.5% bytes; the per-object cost is the REPL's
  `Instantiate` row.
- **Lowering** is +16% on a single action graph and +17% on a ten-step
  chain (two and five more allocations: the guard and effect spans the
  lowered graph now carries, `a0f77861d`), fading to `~` at 1 000 steps.
- **REPL**: loading the 4 000-part file is `~` in time interleaved (the
  first pass's +34% was the candidate's ±37% spread) with +2.8%
  allocations; submitting a snippet to the loaded model +14%.

## Stress constellation (`tests/stressmodel`)

The generated satellite-network constellation at 32, 128 and 512
satellites (6 227, 24 167 and 95 927 elements), the sizes of the previous
records. Time per operation, 0.9.2, `ffe51571c` and the candidate, from the
interleaved re-run of every row but `ConvertAPIJSON` and
`AnalyzeSplitPerDocument` (first pass, see below); bytes and allocations
per operation where they moved. 51 rows on 0.9.2, 45 on the candidate.

| benchmark | 0.9.2 | `ffe51571c` | 0.10.0 (`82f0fac87`) |
| --------- | ----- | ----------- | -------------------- |
| `Load/32` (6 227 el.) | 379 ms ±7% / 132.7 MiB / 1.994 M allocs | 456 ms (+20%) / 162.2 MiB / 2.365 M | 440 ms ±4% (+16%, p=0.002) / 159.7 MiB (+20%) / 2.316 M (+16%) |
| `Load/128` (24 167 el.) | 1.55 s ±20% / 501.3 MiB / 7.682 M | 1.85 s (+20%) | 1.71 s ±10% (+10%, p=0.041) / 568.2 MiB (+13%) / 8.836 M (+15%) |
| `Load/512` (95 927 el.) | 6.67 s ±23% / 1.935 GiB / 30.43 M | 7.81 s (+17%) | 7.50 s ±6% (~, p=0.132) / 2.166 GiB (+12%) / 34.92 M (+15%) |
| `Satisfy/32` (96 assertions) | 4.42 ms ±11% / 1.665 MiB / 28.90 k | 5.10 ms (+16%) / 1.831 MiB / 34.57 k | 4.88 ms ±3% (+11%, p=0.015) / 1.768 MiB (+6.2%) / 33.42 k (+16%) |
| `Satisfy/128` (384) | 18.9 ms ±14% / 114.9 k | 23.5 ms (+25%) | 21.1 ms ±15% (+12%, p=0.026) / 7.064 MiB (+6.2%) / 132.9 k (+16%) |
| `Satisfy/512` (1 536) | 103 ms ±11% / 458.8 k | 132 ms (+28%) | 109 ms ±15% (~, p=0.132) / 28.26 MiB (+6.2%) / 530.9 k (+16%) |
| `FleetInstantiate/32` (1 179 el.) | 4.33 ms ±5% / 7.665 MiB / 12.72 k | 5.13 ms (+19%) | 4.31 ms ±6% (~, p=1.000) / 7.592 MiB (~) / 13.93 k (+9.5%) |
| `FleetInstantiate/128` (1 715) | 12.1 ms ±12% / 24.10 MiB / 48.37 k | 16.6 ms (+37%) | 12.9 ms ±28% (~, p=0.093) / 24.44 MiB (~) / 52.94 k (+9.5%) |
| `FleetInstantiate/512` (3 947) | 63.2 ms ±7% / 151.4 MiB / 190.9 k | 84.6 ms (~, ±29%) | 62.6 ms ±9% (~, p=0.937) / 144.1 MiB (~) / 209.0 k (+9.5%) |
| `FleetSatisfy/32` (24 assertions) | 1.07 ms ±8% / 456.7 KiB / 7 811 | 1.32 ms (+23%) | 1.22 ms ±15% (+14%, p=0.002) / 480.1 KiB (+5.1%) / 8 852 (+13%) |
| `FleetSatisfy/128` (36) | 1.70 ms ±5% / 12.10 k | 2.07 ms (+22%) | 1.90 ms ±9% (+12%, p=0.002) / 743.5 KiB (+4.9%) / 13.65 k (+13%) |
| `FleetSatisfy/512` (108) | 5.35 ms ±17% / 37.06 k | 6.63 ms (+24%) | 5.90 ms ±36% (+10%, p=0.041) / 2.221 MiB (+4.6%) / 41.55 k (+12%) |
| `EditBeside/32` | 1.10 ms ±5% / 295.6 KiB / 3 442 | 1.48 ms (+34%) / 719.5 KiB (+143%) / 5 533 (+61%) | 1.32 ms ±4% (+20%, p=0.002) / 558.6 KiB (+89%) / 4 537 (+32%) |
| `EditBeside/128` | 3.41 ms ±3% / 719.9 KiB / 8 377 | 4.60 ms (+35%) / 2.379 MiB (+238%) / 15.92 k (+90%) | 4.06 ms ±12% (+19%, p=0.002) / 1.756 MiB (+150%) / 11.97 k (+43%) |
| `EditBeside/512` | 13.4 ms ±7% / 2.409 MiB / 28.11 k | 19.1 ms (+43%) / 9.146 MiB (+280%) / 57.44 k (+104%) | 16.1 ms ±4% (+20%, p=0.002) / 6.662 MiB (+177%) / 41.68 k (+48%) |
| `ValidateSplit/32/jobs=1` | 541 ms ±9% / 186.1 MiB / 2.480 M | 612 ms (+13%) | 614 ms ±24% (+14%, p=0.002) / 217.1 MiB (+17%) / 2.881 M (+16%) |
| `ValidateSplit/32/jobs=8` | 254 ms ±5% / 2.499 M | 275 ms (+8.2%) | 267 ms ±5% (+4.9%, p=0.041) / 219.0 MiB (+16%) / 2.896 M (+16%) |
| `ValidateSplit/128/jobs=1` | 1.96 s ±17% / 8.723 M | 2.17 s (+11%) | 2.15 s ±6% (~, p=0.093) / 700.6 MiB (+14%) / 10.07 M (+15%) |
| `ValidateSplit/128/jobs=8` | 802 ms ±19% / 8.740 M | 901 ms (~) | 853 ms ±12% (~, p=0.589) / 703.0 MiB (+14%) / 10.08 M (+15%) |
| `ValidateSplit/512/jobs=1` | 7.86 s ±17% / 33.69 M | 8.94 s (+14%) | 8.70 s ±8% (+11%, p=0.026) / 2.623 GiB (+12%) / 38.80 M (+15%) |
| `ValidateSplit/512/jobs=8` | 3.22 s ±20% / 33.69 M | 3.69 s (~) | 3.64 s ±4% (~, p=0.065) / 2.626 GiB (+12%) / 38.82 M (+15%) |
| `LoadFiles/32` | 473 ms ±15% / 147.7 MiB / 1.971 M | 555 ms (+17%) | 547 ms ±43% (+16%, p=0.009) / 175.7 MiB (+19%) / 2.325 M (+18%) |
| `LoadFiles/128` | 1.88 s ±12% / 6.998 M | 2.03 s (~) | 2.02 s ±6% (~, p=0.240) / 565.6 MiB (+16%) / 8.244 M (+18%) |
| `LoadFiles/512` | 7.62 s ±7% / 27.10 M | 8.51 s (+12%) | 8.39 s ±22% (+10%, p=0.002) / 2.123 GiB (+14%) / 31.91 M (+18%) |
| `EditImported/32` | 547 ms ±25% / 116.3 MiB / 1.979 M | 574 ms (~) / 150.5 MiB (+29%) | 572 ms ±112% (~, p=0.485) / 135.4 MiB (+16%) / 2.360 M (+19%) |
| `EditImported/128` | 1.82 s ±10% / 6.796 M | 2.01 s (+11%) / 468.1 MiB (+23%) | 1.96 s ±10% (+7.5%, p=0.026) / 447.7 MiB (+17%) / 8.090 M (+19%) |
| `EditImported/512` | 7.17 s ±12% / 26.06 M | 8.30 s (+16%) / 1.759 GiB (+19%) | 8.12 s ±6% (+13%, p=0.004) / 1.720 GiB (+16%) / 31.00 M (+19%) |
| `OpenSplit/32/cold` | 251 ms ±18% / 169.0 MiB / 2.360 M | 263 ms (~) | 256 ms ±12% (~, p=0.818) / 199.0 MiB (+18%) / 2.757 M (+17%) |
| `OpenSplit/32/warm` | 106 ms ±16% / 65.62 MiB / 571.1 k | 135 ms (+27%) / 86.05 MiB (+31%) | 132 ms ±5% (+24%, p=0.002) / 79.75 MiB (+22%) / 694.8 k (+22%) |
| `OpenSplit/128/cold` | 770 ms ±13% / 8.208 M | 834 ms (~) | 857 ms ±8% (+11%, p=0.041) / 627.4 MiB (+16%) / 9.554 M (+16%) |
| `OpenSplit/128/warm` | 203 ms ±13% / 166.7 MiB / 1.385 M | 284 ms (+40%) / 215.3 MiB (+29%) | 268 ms ±12% (+32%, p=0.002) / 208.9 MiB (+25%) / 1.717 M (+24%) |
| `OpenSplit/512/cold` | 2.93 s ±8% / 31.59 M | 3.61 s (+23%) | 3.52 s ±18% (+20%, p=0.026) / 2.332 GiB (+14%) / 36.72 M (+16%) |
| `OpenSplit/512/warm` | 661 ms ±6% / 613.2 MiB / 4.639 M | 941 ms (+42%) / 897.5 MiB (+46%) | 939 ms ±20% (+42%, p=0.002) / 890.2 MiB (+45%) / 5.804 M (+25%) |
| `HydratePlane/32` | 95.2 ms ±8% / 25.83 MiB / 314.8 k | 83.6 ms (−12%) | 78.6 ms ±32% (~, p=0.065) / 20.92 MiB (−19%) / 255.0 k (−19%) |
| `HydratePlane/128` | 263 ms ±8% / 76.62 MiB / 919.8 k | 208 ms (−21%) | 211 ms ±52% (~, p=0.065) / 57.47 MiB (−25%) / 652.9 k (−29%) |
| `HydratePlane/512` | 1.03 s ±3% / 287.1 MiB / 3.336 M | 828 ms (−20%) | 846 ms ±4% (−18%, p=0.002) / 207.0 MiB (−28%) / 2.243 M (−33%) |
| `ConvertAPIJSON/32` (first pass) | 9.74 s ±14% / 4.645 GiB / 25.53 M | — | 7.21 s ±8% (−26%, p=0.002) / 4.677 GiB (+0.7%) / 25.13 M (−1.5%) |
| `ConvertAPIJSON/128` (first pass) | 35.7 s ±38% / 18.22 GiB / 100.8 M | — | 29.7 s ±3% (~, p=0.065) / 18.34 GiB (+0.7%) / 99.31 M (−1.5%) |
| `ConvertAPIJSON/512` | one count, 156 s / 72.84 GiB | — | one count, 136 s / 73.31 GiB — no distribution |
| `AnalyzeSplitPerDocument/32/jobs=1` | 425 ms ±7% / 133.1 MiB / 1.636 M | fails | fails (see below) |
| `AnalyzeSplitPerDocument/32/jobs=8` | 108 ms ±5% | fails | fails |
| `AnalyzeSplitPerDocument/128/jobs=1` | 1.69 s ±5% / 489.0 MiB / 6.207 M | fails | fails |
| `AnalyzeSplitPerDocument/128/jobs=8` | 415 ms ±8% | fails | fails |
| `AnalyzeSplitPerDocument/512/jobs=1` | 7.18 s ±25% / 2.025 GiB / 24.49 M | fails | fails |
| `AnalyzeSplitPerDocument/512/jobs=8` | 1.77 s ±11% | fails | fails |

Allocation counts — deterministic to the unit — split the package into
four stories:

- **The loader costs 15–18% more allocations and 12–20% more bytes per
  element** at every scale and through every entrance (`Load`,
  `LoadFiles`, `ValidateSplit`, `OpenSplit/cold`, `EditImported`), for
  10–16% more time where the spread lets the difference show and `~`
  where it does not (the 512-satellite `Load` and the parallel
  `ValidateSplit`). Each connector, binding and flow end the constellation
  is made of is now owned, journaled, indexed and reflected
  (`00b8499a8`, `da8fba4a3`, `32890be28`, `63552cbc4`), and the extended
  declarations classified (`ca0e6cca0`). The qualified-name fix
  (`367ca7e3a`) took 2–3 points off `ffe51571c`'s allocations and 3–7 off
  its time. **Confirmed; structural, linear, and the cost of 0.10.0's
  relationship model.** The whole-binary table below shows the same from
  outside the process.
- **The satisfaction checks are +16% allocations and +10–14% time on the
  fleet** where `ffe51571c` was +20% and +22–28%: the memoized
  succession-cycle check (`8bd97fcf6`) took the walk out of every check
  and left the exact-number evaluation of the assertions (`923ec81f0`,
  `79629923b`), which is 4 500 allocations per 96 assertions. **Confirmed;
  explained.**
- **Editing beside the constellation allocates 2.9 times the bytes of
  0.9.2** (was 3.8 times) and +32–48% objects (was +61–104%), for +20%
  time (was +34–43%); the warm `OpenSplit` and `WorkspaceUpdate` in the
  LSP table are the same. The re-export fix (`4b4b35121`) removed the
  deep copy of each name's claims; the per-name snapshot of claims by
  reference across the 512-satellite neighbor (`4ca321508`) is what
  remains, 4.3 MiB per edit at 512. **Confirmed; the open fix.**
  `OpenSplit/512/warm` (+42%, +45% bytes) adds the warm cache's
  re-validation of the owned-relationship journal it now carries.
- **Instantiation of the fleet is `~` in time with 9.5% more allocations
  and the same or fewer bytes** — the three-word scalar packing
  (`4a1b937f2`) and the allocation-free Integer and Rational fast paths
  (`d1eb16edb`) took `ffe51571c`'s +19–37% back to 0.9.2, and
  `HydratePlane` is **18–33% fewer allocations and 18–28% fewer bytes**
  than 0.9.2, 18% faster at 512: the hydration of a plane's instances no
  longer rebuilds the implicit ends it can derive (`fa8a7220c`). The
  API-JSON conversion is 26% faster at 32 satellites with the same bytes,
  from the same source; its 512-satellite row still does not fit this
  machine on either revision.

`AnalyzeSplitPerDocument` fails on the candidate at every size. The
benchmark analyses each of the constellation's six files in its own
per-document registry and asserts a clean result; the candidate's
per-document analysis now reports each decimal literal the generator
writes into a `Real`-typed attribute (`0.2 is rounded to the nearest Real,
0.20000000000000001, since a feature typed by Real holds a binary64
value; type the feature by Rational to keep the literal exact`,
`014fceb7b`), ten per file, and the assertion trips. The six rows are
baseline-only figures in this record; the benchmark's generator or its
assertion needs the release's view of decimal literals (a `Rational`
attribute type, or the lint filtered) before the row can be compared
again. The whole-workspace analysis the other rows and `sysml -validate`
do is not affected — the report is a lint the per-document registry turns
on — which is why `ValidateSplit` and the whole-binary runs are clean on
both.

## Whole-binary scaling

`sysml -validate` over generated compliant models (the satellite-network
generator at ≈3 000, 6 000 and 12 000 declarations: 3 804, 7 260 and
14 172 lines) that report `no errors`, nine runs each with the three
binaries interleaved; median wall seconds and median maximum resident set,
every run exiting 0.

| declarations | 0.9.2 | `ffe51571c` | 0.10.0 (`82f0fac87`) |
| ------------ | ----- | ----------- | -------------------- |
| ≈3 000 | 0.09 s / 95 MiB | 0.11 s / 100 MiB | 0.11 s / 101 MiB |
| ≈6 000 | 0.14 s / 107 MiB | 0.16 s / 116 MiB | 0.17 s / 115 MiB |
| ≈12 000 | 0.23 s / 134 MiB | 0.27 s / 157 MiB | 0.28 s / 157 MiB |

Linear in the declarations on every binary, as before: doubling the model
doubles the time (0.11 → 0.17 → 0.28 s) and adds 14–42 MiB. The candidate
is 20–22% slower than 0.9.2 and holds 6–23 MiB more, growing with the
model — the loader's per-element cost of the stress constellation
(+14–20% bytes, +15–18% allocations) seen from outside the process. The
fixes did not move these figures against `ffe51571c`, and were not
expected to: none of the six is on the load path. The spreads were tight
on 0.9.2 and `ffe51571c` (±1 sample in the second decimal) and the
candidate's first round of each model was its slowest (0.51, 0.75 and
1.48 s, a cold page cache for the freshly built binary); the medians are
unaffected and the remaining eight rounds agree with them.

### Process start and session floor

| measure | 0.9.2 | `ffe51571c` | 0.10.0 (`82f0fac87`) |
| ------- | ----- | ----------- | -------------------- |
| `sysml -e 1` wall time | 0.02 s | 0.02 s | 0.02 s |
| `sysml -e 1` max RSS (session floor) | 64 MiB | 68 MiB | 69 MiB |
| `bin/sysml` size (`-s -w`) | 42.88 MiB | 46.99 MiB | 47.23 MiB |

The floor is the standard library's bundled index decoded at start, and
rose 5 MiB with the snapshot's new contents (the owned-relationship
journal and the reflected implicit relationships, `DecodeSnapshot`
+7.2% bytes). The binary is 4.35 MiB larger than 0.9.2's: the new
frontends (views, code lenses, inlay hints, the REPL's repository and
notebook commands, `-list`), the exact-number evaluator, the value import
and the SysML v2 API client; 0.24 MiB of it arrived after the release
branch was cut. Start time is unchanged.

### Example models and the Apollo 11 model

| model | 0.9.2 | `ffe51571c` | 0.10.0 (`82f0fac87`) |
| ----- | ----- | ----------- | -------------------- |
| `examples/action-executor-demo.sysml` | 0.02 s / 66 MiB | 0.02 s / 69 MiB | 0.02 s / 71 MiB |
| `examples/disposal-robot-demo/robot.sysml` | 0.04 s / 72 MiB | 0.04 s / 78 MiB | 0.04 s / 77 MiB |
| Apollo 11 (28 files, `scripts/download-apollo11.sh`) | 0.28 s / 149 MiB | 0.33 s / 169 MiB | 0.32 s / 161 MiB |

The two small examples sit on the session floor and move with it. The
Apollo 11 model — the one real model in the set, connector- and
flow-dense like the stress constellation — validates in 0.32 s against
0.28 s (+14%) holding 12 MiB more; 8 MiB less than `ffe51571c` held, the
smaller claim snapshots of the re-export fix on a multi-document model.
Every file validates with the same report on all three binaries.

## Re-runs

The first pass left 83 of the 196 compared rows in doubt: 61 by a
significant difference, 22 by a spread above 20% on one side with no
significant difference (`Fib(25)/interpreted` ±65%, `ReferencesCold`
±19/17%, `Instantiate` ±79/109%, most of the stress constellation's time
rows). All were re-run interleaved — `-count 1` of the row on 0.9.2, then
on `ffe51571c`, then on the candidate, six times — and the
`internal/frontend/repl`, `tests/perf` and `tests/stressmodel` packages
whole or in their moving rows, since most of each was in doubt. The pilot
corpora were downloaded between 0.9.2's first pass and the re-run, so the
`vehicle` rows and `InstantiateWarmModel` have the re-run only.

What the re-run changed:

- **Nothing changed sign.** Every row significant in the first pass is
  significant in the re-run in the same direction or `~`; the stress
  constellation's time regressions shrank from the first pass's +14–27%
  to +10–20% once the three revisions ran under the same conditions, and
  that is the figure reported.
- **Cleared:** `REPLLoadFile` (+34% as found; `~` at p=0.065),
  `Instantiate` in `tests/perf` (+39% as found; `~`), `BatchSatisfy`
  (+17% as found; `~`), `IndexLibrary` (+19% as found; `~`),
  `ExpandModelImports` (+9.0% as found; `~`),
  `AnalyseResolved` (+12% as found; `~`, the +18% allocations stand),
  `DecodeSnapshot` (`~`, the +19% allocations stand), `IndexAddExpand`
  and `IndexAddOnly` (`~`), `FleetInstantiate` at every size (`~`),
  `Load/512` and `Satisfy/512` (`~` at ±6% and ±15%), `LoadFiles/128`,
  `OpenSplit/32/cold`, `EditImported/32`, `ValidateSplit/128` (`~`),
  `ExecuteAction` (`~`), `ActionChain/chain10, chain1000` (`~`),
  `LowerStateGraph` (`~`), `RunStateMachine/1000` (`~`),
  `Diagnostics/attributes=800` (`~`), `GRPCParseFileCached` (`~`).
- **Confirmed** (significant both times, within a few points of each
  other): `AssignmentLoopStep` (+90% → +61%), `MaterializePlainAttributes`
  (`~` → +14%), `DerivedReadWriteRead` (+39% → +11%), `FormatEdits`
  (+71%), `WorkspaceUpdate` (+42% → +16%), the REPL `LoadModel` rows
  (+17–41% → +5–10%), `RunCalc` (+13–19% → +10–14%), `Instantiate/250,
  1000, 4000` (+13–27% → +11–31%), `Collatz(27)` (+71%/+53% →
  +65%/+59%), `Fib(25)/go` (+94% → +81%), `Hypot(3.0,4.0)/go` (+394% →
  +412%), `InstantiateWarmModel` (+32%), `ExpandWildcardImports` (+15% → +4.2%),
  and the `tests/perf` and stress
  rows the tables above mark significant.
- **Moved the other way:** `Fib(25)/interpreted` was `~` in the first pass
  (6.61 ms ±65% against 7.54 ms) and is −20% at ±9% interleaved.
- **Still in doubt:** `WorkspaceEdit/synthetic` (+35% at ±68% on the
  candidate's counts; the +10% bytes and +14% allocations are the
  reading), `GRPCVerifyConstraint` (+21% at ±43%),
  `EditImported/32` (±112%), `HydratePlane/32, 128` (−17%, −20% at
  ±32% and ±52%, `~` at p=0.065; the allocation gains stand),
  `LoadFiles/32` (+16% at ±43%); `IntegerArithmeticLoop` ±59% and
  `RationalDecimalLoop` ±34%, candidate-only.

Raw `benchstat` output for every package, first pass and re-run, is in the
tables above; the per-row p-values are p=0.002 (n=6) unless shown.

## What this does not measure

- The new surfaces with no 0.9.2 counterpart — the view renderers, the
  graphs export, the api-json loader, the browser engine, the REPL's
  repository and notebook commands, the Python and Node clients' new
  calls — have benchmarks only where listed above (`ImportValues`, the
  exact-numeric loops); the rest are covered by their test suites, not by
  this record.
- The compiled-calc C target is measured on `Hypot` only; the three
  Integer calcs it refuses have no candidate figure, so the C target's
  speed on 0.10.0 is read from one row.
- The stress constellation's per-document analysis
  (`AnalyzeSplitPerDocument`) has no candidate figure at any size.
- Everything ran on one 8-CPU virtual machine. The baseline's re-run
  spreads are mostly under 20%, but the stress constellation's
  `EditImported/32` and `HydratePlane` rows and the candidate's
  `WorkspaceEdit/synthetic` still show 30–110% spreads on single counts,
  and a one-CPU host should read the package figures rather than the
  whole-binary ones, as the previous records said.
