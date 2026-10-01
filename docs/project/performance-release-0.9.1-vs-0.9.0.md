# Performance: release 0.9.1 against release 0.9.0

The release-gate measurement of the 0.9.1 candidate — `develop` at
`835f29fde` (2026-09-29), 1 196 commits past the `v0.9.0` tag
(`ee54ea03e`, 2026-09-24) — against 0.9.0 itself, and the state of the
candidate after the fixes this record made. It follows the method of the
[0.8.0 against 0.7.0](performance-release-0.8-vs-0.7.0.md) record: every
benchmark on both revisions, `benchstat` over six runs, the rows the machine
leaves in doubt re-run interleaved, whole-binary timings on generated and
real models, and each regression either fixed, explained as a rule's price,
or left named. See [performance](../internals/performance.md) for what the
figures mean and where the remaining cost is.

Three columns appear throughout: **0.9.0** is the tag, **develop** is the
candidate as found, **develop+fix** is the candidate with the six commits
of this change (`40af56ad1`, `e88448334`, `8ac7522c3`, `9e0047fe3`,
`f554cebad`, `16b3e69a8`). Where a table has two columns, the fixes do not touch that
path and the as-found figure stands.

## Summary

- **Runtime writes and loops.** As found, an assignment step in an action
  loop was 35% slower in 95% more allocations, a derived read-write-read
  124% slower, and the interpreted `SumTo` calc allocated 2.2× the bytes of
  0.9.0. Three changes account for it — the §7.6.3 effective-multiplicity
  rule, the nested-redefinition chains, and a per-statement `this` closure
  — and none of the cost is what the rules need. **Fixed**: the loop step
  and `SumTo` are at parity, the runtime package's geomean is +4% for +31.7%
  as found, and the derived read-write-read keeps +28% for the chain check
  the rule genuinely adds on every governed write.
- **Loading a model.** As found, loading the stress constellation allocated
  27% more per element and loading it from files was 25–31% slower. The
  new port-type-mismatch lint rebuilt every port type's feature list per
  reference. **Fixed**: bytes per element are +4%, `LoadFiles` is within
  noise at 32 satellites and +13% at 128, and the remainder is the two new
  lints and the interface records (priced below).
- **Satisfying requirements** was 44–51% slower per assertion and
  allocated 64% more, at every size. This is the shared-default and
  shared-verdict tracing that two 0.9.1 fixes require — refusing a verdict
  along a destroyed object and re-deriving clock-dependent checks per
  occurrence. Profiled, most of the allocation and a third of the time was
  recomputation around the tracing, not the tracing: the trace copied every
  path of a nested shared record per read, deduplicated reads through a key
  string built per read, spelled a dotted path per ancestor to ask whether a
  binding governs it, and every journal mark cloned the clock's waiter list.
  **Fixed**: the geomean is +11% time and +12% allocations for +46% / +64%
  as found, and bytes are 46% below 0.9.0. The rest — the per-read
  recording and the per-ancestor binding lookups — is the tracing's own
  bookkeeping, **priced** at +31% on the 32-satellite row and within noise
  at 128 and 512.
- **Whole binary.** The candidate validates the generated models in
  0.4–0.5× the wall time of 0.9.0 and the Apollo 11 model in 0.6×, with a
  smaller resident set at 6 000 elements and above, because the workspace
  analyses documents in parallel since `98f533591`. Example-scale commands
  are level. The binary is 2.7 MiB larger and an empty session maps 6 MiB
  more.
- **Elsewhere**: the gRPC cold parse is 8–9% slower. `VerifyConstraint`
  was 20–29% slower: `7bd51ece6` (implicit nested-usage subsetting) makes
  the instance graph carry every nested usage's subsetted collection, which
  the response serializes; the recomputation around it is **fixed** (bytes
  +12% for +19%) and the rest is the rule's output. The migration writer
  was 15–21% slower for a larger per-block buffer and is now **2× faster
  than 0.9.0** with a quarter of the allocations. The single-threaded
  analysis of a resolved document was +31%: the undeclared-signal lint
  walked the document twice by reflection — **fixed** to once, +14%
  remains for the lints themselves. The REPL's `EvalExpr` is 94% faster,
  action chains 44–77%, an LSP workspace update 22%, and a loaded model
  holds half the live heap it did.

Nothing here blocks the release; the residual costs are each a rule's
price, named with the commit that introduced it.

## Method

Machine: `INTEL(R) XEON(R) PLATINUM 8559C`, 8 CPUs, 31 GiB, Go
`go1.25.0 linux/amd64`, `GOMAXPROCS=8`. 0.9.0 is a worktree at the tag,
built and benchmarked with its own tree; `develop` is `835f29fde`; the
fixed candidate is this branch. Package benchmarks are
`go test ./<pkg> -run '^$' -bench . -benchmem -count 6` on each revision,
compared with `benchstat` (rows are significant at p < 0.05; `~` is no
significant difference). Rows the first pass left in doubt were re-run on
the fixed tree with the revisions interleaved. Whole-binary figures are
`/usr/bin/time -f '%e %M'` over three interleaved runs of each binary, all
three built with `make build-sysml` (`-s -w`, `-trimpath`); medians are
reported. CPU and heap profiles (`-cpuprofile`, `-memprofile`) were taken
on the rows that moved most and are what the findings cite.

### What is and is not comparable

- **`internal/syntax/parser`** defines no benchmark that runs without
  `OPENSYSML_BENCH_MODEL`; neither revision produced a row and the package
  is omitted.
- **`internal/frontend/grpc`** as found: `BenchmarkInstantiate` (warm)
  fails on `develop` because the service now bounds held objects at 10 000
  and the benchmark's loop exceeds it; `8ac7522c3` lifts the bound for the
  benchmark only. Its rows are excluded from the as-found comparison.
- **`tests/stressmodel`**: 0.9.0 has 15 of the candidate's 48 rows
  (`FleetInstantiate`, `FleetSatisfy`, `ValidateSplit`,
  `AnalyzeSplitPerDocument`, `OpenSplit`, `HydratePlane` and
  `ConvertAPIJSON` are new); the candidate's `ConvertAPIJSON/satellites=512`
  did not complete six runs and is excluded. Only the 15 common rows are
  compared.
- **Diagnostics**: on every model measured the three binaries return the
  same exit status. The generated models and `robot.sysml` report
  `no errors` on all three; the Apollo 11 model reports 4 warnings on 0.9.0
  and 2 on the candidate (the same two, plus two the candidate no longer
  raises), so the candidate does slightly less reporting work there.

## Package benchmarks

### `internal/exec/runtime`

| benchmark | 0.9.0 | develop | develop+fix |
| --------- | ----- | ------- | ----------- |
| `MaterializePlainAttributes` | 47.8 µs / 46.0 KiB | 52.1 µs (+9%) / 49.5 KiB (+8%) | 47.0 µs (−2%) / 42.7 KiB (−7%) |
| `SetFeatureValueNoDependents` | 424 ns | 387 ns (−9%) | 385 ns (−9%) |
| `DerivedReadWriteRead` | 4.21 µs / 2.44 KiB / 63 allocs | 9.43 µs (+124%) / 3.50 KiB (+44%) / 79 | 5.38 µs (+28%) / 2.48 KiB (+2%) / 65 |
| `AssignmentLoopStep` | 883 µs / 477 KiB / 4 419 allocs | 1 191 µs (+35%) / 758 KiB (+59%) / 8 632 (+95%) | 906 µs (~) / 521 KiB (+9%) / 4 923 (+11%) |
| geomean | | +31.7% / +25.2% B / +25.3% allocs | +4.0% / +0.8% B / +3.5% allocs |

### `internal/frontend/repl`

Rows that moved; `LoadModel` and `Diagnostics` are as found (the fixes do
not touch them).

| benchmark | 0.9.0 | develop | develop+fix |
| --------- | ----- | ------- | ----------- |
| `RunCalc/elements=250` | 3.85 µs / 66 allocs | 4.73 µs (+23%) / 76 | 4.36 µs (+13%) / 76 |
| `RunCalc/elements=1000` | 3.89 µs | 4.85 µs (+25%) | 4.35 µs (+12%) |
| `RunCalc/elements=4000` | 4.08 µs | 4.63 µs (+14%) | 4.39 µs (+8%) |
| `Instantiate/elements=250` | 5.71 µs / 7.1 KiB / 47 allocs | 7.04 µs (+23%) / 8.2 KiB (+15%) / 59 | 5.70 µs (~) / 6.4 KiB (−10%) / 53 |
| `Instantiate/elements=1000` | 5.96 µs | 7.20 µs (+21%) | 5.86 µs (~) |
| `Instantiate/elements=4000` | 6.27 µs | 7.17 µs (+14%) | 5.97 µs (−5%) |
| `CompiledCalc/SumTo(1000000)/interpreted` | 1.56 s / 58.1 MiB / 3.04 M allocs | 1.66 s (+7%) / 127.5 MiB (+119%) / 6.04 M (+99%) | 1.62 s (~) / 58.8 MiB (+1%) / 3.04 M (+0.1%) |
| `CompiledCalc/Collatz(27)/interpreted` | 262 µs / 28.1 KiB / 986 allocs | 286 µs (+9%) / 40.1 KiB (+43%) / 1 449 (+47%) | 275 µs (+5%) / 29.3 KiB (+5%) / 999 (+1%) |
| `CompiledCalc/Fib(25)/interpreted` | 7.70 ms | 7.90 ms (~) | 7.76 ms (~) |
| `LoadModel/elements=1000` | 108 ms / 45.1 MiB / 13.6 MiB live | 117 ms (+8%) / 50.7 MiB (+12%) / 6.9 MiB live (−49%) | as found |
| `LoadModel/elements=4000` | 180 MiB / 53.3 MiB live | 203 MiB (+12%) / 27.5 MiB live (−48%) | as found |
| `Diagnostics/attributes=50` | 58.4 KiB / 349 allocs | 72.6 KiB (+24%) / 543 (+56%) | as found |
| `RunStateMachine/elements=1000` | 14.6 µs | 12.8 µs (−12%) | as found |

### `tests/stressmodel`

The 15 rows both revisions have, at 32 and 128 satellites (the 512 rows
move by the same ratios as found: `Satisfy` +48%, `LoadFiles` +31%,
`EditImported` +30%, `Load` +9%; the fixed tree was re-run at 32 and 128
with four runs).

| benchmark | 0.9.0 | develop | develop+fix |
| --------- | ----- | ------- | ----------- |
| `Load/satellites=32` | 357 ms / 20.4 KiB per element | 409 ms (+15%) / 25.9 KiB (+27%) | 370 ms (~) / 21.2 KiB (+4%) |
| `Load/satellites=128` | 1.46 s / 19.8 KiB per element | 1.58 s (+8%) / 25.4 KiB (+28%) | 1.53 s (~) / 20.6 KiB (+4%) |
| `Load` live heap per element | 5.1 KiB / 4.6 KiB | 2.7 KiB / 2.3 KiB (−47% / −49%) | same |
| `Satisfy/satellites=32` (96 assertions) | 3.06 ms / 1.74 MiB / 25.9 k allocs | 4.63 ms (+51%) / 3.12 MiB (+80%) / 42.4 k (+64%) | 4.20 ms (+31%) / 1.66 MiB (−4%) / 28.9 k (+12%), finding 5 |
| `Satisfy/satellites=128` (384 assertions) | 14.4 ms / 10.7 MiB / 103 k allocs | 22.5 ms (+56%) / 16.2 MiB (+52%) / 169 k (+64%) | 17.8 ms (~) / 6.65 MiB (−38%) / 115 k (+12%), finding 5 |
| `LoadFiles/satellites=32` | 379 ms / 124 MiB | 476 ms (+25%) / 165 MiB (+33%) | 437 ms (~) / 136 MiB (+10%) |
| `LoadFiles/satellites=128` | 1.43 s / 404 MiB | 1.82 s (+28%) / 555 MiB (+37%) | 1.62 s (+13%) / 442 MiB (+9%) |
| `EditImported/satellites=32` | 484 ms / 108 MiB | 519 ms (+7%) / 134 MiB (+25%) | 491 ms (~) / 105 MiB (−2%) |
| `EditImported/satellites=128` | 1.44 s / 323 MiB | 1.80 s (+25%) / 454 MiB (+41%) | 1.64 s (+14%) / 340 MiB (+5%) |
| `EditBeside/satellites=32,128,512` | 1.04 / 3.20 / 12.9 ms | +3% / ~ / ~ | as found |

### `internal/frontend/grpc` and `tests/perf` (as found)

| benchmark | 0.9.0 | develop |
| --------- | ----- | ------- |
| `ParseFileColdShared` | 6.94 ms / 4.27 MiB | 7.55 ms (+9%) / 4.69 MiB (+10%) |
| `ParseFileColdInline` | 7.13 ms / 4.45 MiB | 7.71 ms (+8%) / 4.91 MiB (+11%) |
| `GRPCParseFileUncached` | 117 ms / 76.1 MiB | 132 ms (+13%) / 82.8 MiB (+9%) |
| `GRPCVerifyConstraint` | 687 µs / 456 KiB / 8.5 k allocs | 887 µs (+29%) / 542 KiB (+19%) / 9.5 k (+11%) |
| `GRPCEvaluate` | 7.8 KiB | 8.4 KiB (+9%), time ~ |
| `REPLEvalExpr` | 82.3 ms / 37.0 MiB / 293 k allocs | 4.53 ms (−94%) / 6.8 MiB (−82%) / 101 (−99.97%) |
| `REPLSubmitSnippet` | 1.26 s / 398 MiB | 0.96 s (−24%) / 461 MiB (+16%) |
| `ActionChain/chain100`, `chain1000` | 992 µs, 56.6 ms | 552 µs (−44%), 13.0 ms (−77%) |
| `ActionLoop/for1000` | 279 KiB / 3 175 allocs | 302 KiB (+9%) / 4 180 (+32%), time ~ |
| `IndexAddOnly/synthetic` | 47.0 ms | 38.5 ms (−18%) |
| `Analyze/synthetic` | 380 MiB | 450 MiB (+18%), time ~ |
| `WorkspaceEdit/synthetic/reindex+diagnostics` | 377 MiB | 443 MiB (+17%), time ~ |
| `WorkspaceEditSmallDocBesideLarge` | 6.1 MiB / 65.9 k allocs | 7.9 MiB (+31%) / 81.7 k (+24%), time ~ |
| `tests/perf` geomean | | −10.5% time / +3.0% B |

`ActionLoop`'s extra allocations are the loop-step finding below and are
gone on the fixed tree (`AssignmentLoopStep` is the same path); the
`tests/perf` package was not re-run in full after the fixes.

### `internal/frontend/lsp`, `internal/workspace/*`, `internal/translate/migrate` (as found)

| benchmark | 0.9.0 | develop |
| --------- | ----- | ------- |
| `lsp WorkspaceUpdate` | 301 ms / 66.4 MiB / 932 k allocs | 235 ms (−22%) / 49.7 MiB (−25%) / 606 k (−35%) |
| `lsp ReferencesCold` | 44.0 ms | 41.1 ms (−7%) |
| `lsp RenameWarm` | 1.41 MiB | 1.31 MiB (−7%), time ~ |
| `lsp` geomean | | −7.7% time |
| `libs ExpandModelImports` | 6.11 ms / 3.90 MiB | 6.63 ms (+8%) / 4.32 MiB (+11%) |
| `libs DecodeSnapshot` | 29.9 MiB / 46.4 k allocs | 35.3 MiB (+18%) / 56.6 k (+22%), time ~ |
| `libs IndexLibrary`, `ExpandWildcardImports` | 12.5 / 13.4 MiB | +9% / +8% B, time ~ |
| `model AnalyseResolved` | 1.26 ms / 486 KiB | 1.41 ms (+12%) / 534 KiB (+10%) |
| `model` geomean | | +8.3% time / +5.3% B |
| `migrate WriterSiblingBlocks` | 3.57 ms / 5.38 MiB | 4.32 ms (+21%) / 5.69 MiB (+6%) |

## Whole-binary scaling

`sysml -validate` over generated compliant models that report `no errors`,
three runs each with the three binaries interleaved; median wall seconds
and median maximum resident set.

| elements | 0.9.0 | develop | develop+fix |
| -------- | ----- | ------- | ----------- |
| 3 000 | 0.13 s / 89 MiB | 0.07 s / 92 MiB | 0.07 s / 93 MiB |
| 6 000 | 0.26 s / 116 MiB | 0.11 s / 99 MiB | 0.12 s / 100 MiB |
| 12 000 | 0.49 s / 154 MiB | 0.20 s / 127 MiB | 0.20 s / 127 MiB |

The candidate validates in 0.4–0.5× the wall time of 0.9.0 at every size
and in 17–27 MiB less from 6 000 elements up. The wall-time gain is the
workspace analysing documents in parallel (`ParallelFor` in
`internal/workspace/model`, from `98f533591`, *hydrate and demote interface
records, write them from every analysis*, 2026-09-27) on this 8-CPU
machine — single-threaded CPU per element is higher, as the package
benchmarks above show, and a one-CPU host should expect the package
figures rather than these. The 3 MiB more at 3 000 elements is the larger
binary and the new passes' tables.

### Process start and session floor

| figure | 0.9.0 | develop | develop+fix |
| ------ | ----- | ------- | ----------- |
| binary size | 39.7 MiB | 42.4 MiB | 42.4 MiB |
| empty session (`sysml -e 1`), median of 3 | 0.02 s / 58 MiB | 0.02 s / 65 MiB | 0.02 s / 64 MiB |
| empty-session load (`LoadModel/elements=0`) | 182 KiB / 1 621 allocs | 216 KiB (+19%) / 1 867 (+15%) | as found |

The session floor is 6 MiB higher and start-up is unchanged at this
resolution. The growth is the binary (2.7 MiB more of linked packages)
and the interface-record and lint tables an empty workspace now keeps.
**Explained**, as the previous records explained their intervals.

### Example models

Median wall time and maximum resident set of three runs; every run returned
the same exit status on all three binaries.

| model / command | 0.9.0 | develop | develop+fix |
| --------------- | ----- | ------- | ----------- |
| `action-executor-demo.sysml -validate` | 0.02 s / 61 MiB | 0.02 s / 65 MiB | 0.02 s / 66 MiB |
| `action-executor-demo.sysml -action ActionExecutorDemo::sequential` | 0.02 s / 64 MiB | 0.02 s / 70 MiB | 0.02 s / 70 MiB |
| `orthogonal-regions-demo.sysml -state …::TrafficLight -advance 20` | 0.04 s / 71 MiB | 0.05 s / 79 MiB | 0.05 s / 80 MiB |
| `phase-c-behavioral-bodies.sysml -instantiate PhaseC::Vehicle` | 0.03 s / 64 MiB | 0.02 s / 70 MiB | 0.03 s / 70 MiB |
| `phase-c-behavioral-bodies.sysml -state PhaseC::AutopilotMode -advance 20` | 0.03 s / 64 MiB | 0.02 s / 69 MiB | 0.02 s / 70 MiB |
| `disposal-robot-demo/robot.sysml -validate` | 0.05 s / 70 MiB | 0.04 s / 72 MiB | 0.04 s / 72 MiB |

At example scale the candidate is level with 0.9.0 within the 10 ms
resolution of the clock and maps 2–8 MiB more, which is the session floor
above.

### The Apollo 11 model

The public [Apollo 11 SysML v2 model](https://github.com/airbus/apollo-11-sysml-v2)
(commit `6e9c93f`, 28 files, 7 221 lines). Medians over three warm runs:

| what | 0.9.0 | develop | develop+fix |
| ---- | ----- | ------- | ----------- |
| `sysml -validate` all 28 files | 0.43 s / 156 MiB | 0.26 s / 150 MiB | 0.27 s / 150 MiB |
| what the run reports | 4 warnings, no error | 2 warnings, no error | 2 warnings, no error |

The candidate validates the model in 0.6× the time and 6 MiB less, for
the same reason as the generated models.

## Findings

### 1. Loop steps and interpreted calcs paid for the effective-multiplicity rule on every read — fixed

`AssignmentLoopStep` +35% and +95% allocations; `ActionLoop/for1000` +32%
allocations; `RunCalc` +14–25%; `Instantiate` +14–23%. Profiles of the loop
step put the new time under `EffectiveParameterRange`, which `145a1469b`
(*a parameter with no written multiplicity takes its §7.6.3 effective
multiplicity*, 2026-09-29) made every parameter binding consult, and which
walked the parameter's typing and redefinition closure on each call. The
rule is right; recomputing it per binding is not. `40af56ad1` memoizes the
range per parameter symbol on the semantic model, journaled and
invalidated with the other symbol-keyed caches. `e88448334` removes a
second per-write cost the profile showed beside it: the redefinition alias
map was built for every type on every write, including the great majority
of types that have no redefinition group at all. After both the loop step
is at parity with 0.9.0 (`~`, p = 0.24) and `Instantiate` is level or 5%
faster.

### 2. The nested-redefinition chains checked every write's ancestry — fixed, residual priced

`DerivedReadWriteRead` +124% time, +44% bytes. `7fbc9a5c1` (*apply nested
redefinition chains below composite features*, 2026-09-27) and `625ddacba`
(*apply nested chains to objects written into a governed feature*,
2026-09-28) make a write into a governed feature apply the chains its
owners declare. As found, every write asked every owner and classifier
whether it declares a nested chain and whether it hosts one, and both
answers were recomputed from the declarations each time. `40af56ad1`
memoizes both per type on the runtime model (`16b3e69a8` moves the
chain-host memo off `EffectiveFeature`, whose field count the self-model
asserts). The row is now +28% and +2%
bytes: what remains is the one lookup per write that decides whether a
chain applies, which is the rule's cost. It is 1.2 µs on this row and
does not show in any whole-binary or REPL figure.

### 3. Each statement re-bound the calc's `this` — fixed

`CompiledCalc/SumTo(1000000)/interpreted` allocated 127.5 MiB (+119%) in
6.04 M allocations (+99%) for 58 MiB and 3.04 M on 0.9.0 — three
allocations per loop iteration; `Collatz` +43% bytes. The heap profile
put them in the statement engine's evaluation context, which closed over
the host's occurrence-materialisation hook on every statement so a calc's
`this` could be materialised lazily. `9e0047fe3` binds the hook once per
occurrence and once per statement engine and hands the same function
value to each evaluation; the lazy materialisation and its error on a
calc with no occurrence (`TestInvokeCalcDefReadsThis`) are unchanged.
`SumTo` is back to 58.8 MiB and 3.04 M allocations, `Collatz` +5% bytes,
and the calc runs are within noise of 0.9.0.

### 4. The port-type-mismatch lint rebuilt every port's feature list — fixed, residual priced

`Load` +27% bytes per element, `LoadFiles` +25–31%, `EditImported`
+25–41% bytes. The heap profile of `LoadFiles/satellites=32` put the new
allocations in `semantics.(*Model).PortFeatures` and `ownedPortFeatures`,
reached from the port-type-mismatch lint of `598c848eb` (*Add
undeclared-signal and port-type-mismatch lints*, 2026-09-29), which asked
for each connected port's conjugated feature list per reference and got a
fresh slice each time. `f554cebad` memoizes `PortFeatures` per port type
on the semantic model, journaled like `conjSupers` beside it. Bytes per
element are +4% after it; `LoadFiles` is within noise at 32 satellites and
+13% at 128, `EditImported` −2% and +5% bytes. The residual time is the
two lints' walks — the undeclared-signal lint inspects every value in the
tree (`ast.inspectValue` in the profile) — and the interface records
`98f533591` writes from every analysis, which are the other new
allocators in the profile and are what the LSP and gRPC surfaces now
read. Both are the features' cost; neither was reshaped here.

### 5. Satisfaction traces every shared default and verdict — attributed, recomputation fixed, tracing priced

`Satisfy` +44–51% time and +64% allocations at 32, 128 and 512
satellites as found. `758c9c260` (*refuse shared defaults and verdicts
along a destroyed object*, 2026-09-25) and `afbdb615d` (*derive
clock-dependent defaults and checks per occurrence*, 2026-09-25) make a
requirement's verdict record the reads it depended on so it can be refused
or re-derived when an object on the path is destroyed or a clock moves.
Profiled on the 32-satellite row (`-cpuprofile`, `-memprofile`, `go tool
pprof -top -cum`), the tree before this change spends 11.9% of its CPU under
`Context.sharedPaths` and 6.7% under `Context.bindingDeclaredFor`, and its
three largest allocators are `Context.observeRead` (316 MB of the run's
heap), `Context.sharedPaths` (188 MB cumulative, in `strings.Builder`)
and `slices.Clone[clockWaiter]` under `Context.markJournal` (289 MB). Of
that, four things were recomputation, not the record the fixes need:

- `observeRead` copied every path of a nested shared record into the
  trace on each read of it. It now records the record once
  (`sharedRead.shared`) and `sharedPaths` expands it only when a verdict
  is being shared.
- `sharedPaths` deduplicated reads through a length-prefixed key string
  built per read (`pathKey`) and assembled the root-relative path twice.
  It now compares the paths as slices (`hasPath`) and assembles each
  once; the dotted-name property `pathKey` guarded is the same test on
  `hasPath`.
- `bindingDeclaredFor` spelled a dotted path at every ancestor of every
  read and looked each up in `bindingsForFeature`, whose per-type,
  per-path memo grows with the paths asked. A binding that involves a
  path has an end spelled exactly as that path, so its first segment is
  a feature the binding starts at: `Model.bindingRoots` memoizes that
  set per type (usually empty), and the path is spelled and looked up
  only where a binding starts at that feature.
- Every journal mark — one per bound transaction, so one per check —
  cloned the clock's waiter list, and every rollback cloned it back
  (the same in 0.9.0). The clock now replaces the list instead of editing
  it in place (`attach`, `detach`, `forgetFinished`), so a mark keeps
  the slice it saw. This is also why the fixed tree allocates 38–74%
  fewer bytes than 0.9.0 at 128 and 512 satellites: the clone was
  proportional to waiters × transactions.

Interleaved six-run comparison (`go test ./tests/stressmodel -run '^$'
-bench '^BenchmarkSatisfy/satellites=(32|128|512)/' -benchmem -count 1`
per round, six rounds, `benchstat`):

```
                                         │     v0.9.0      │           before this change         │               fixed                 │
                                         │     sec/op      │    sec/op      vs base               │    sec/op     vs base               │
Satisfy/satellites=32/assertions=96-8         3.203m ±  5%    4.822m ± 38%  +50.57% (p=0.002 n=6)   4.197m ±  7%  +31.05% (p=0.002 n=6)
Satisfy/satellites=128/assertions=384-8       15.32m ± 25%    22.16m ± 28%  +44.64% (p=0.002 n=6)   17.76m ±  4%        ~ (p=0.065 n=6)
Satisfy/satellites=512/assertions=1536-8     102.20m ±  8%   146.58m ± 11%  +43.43% (p=0.002 n=6)   91.96m ± 62%        ~ (p=0.093 n=6)
geomean                                       17.11m          25.02m        +46.18%                 19.00m        +10.99%

                                         │      B/op       │     B/op       vs base               │     B/op      vs base               │
Satisfy/satellites=32/assertions=96-8         1.735Mi ± 0%    3.118Mi ± 0%  +79.66% (p=0.002 n=6)   1.664Mi ± 0%   -4.14% (p=0.002 n=6)
Satisfy/satellites=128/assertions=384-8      10.653Mi ± 0%   16.206Mi ± 0%  +52.12% (p=0.002 n=6)   6.648Mi ± 0%  -37.59% (p=0.002 n=6)
Satisfy/satellites=512/assertions=1536-8     102.12Mi ± 0%   124.90Mi ± 0%  +22.31% (p=0.002 n=6)   26.60Mi ± 0%  -73.95% (p=0.002 n=6)
geomean                                       12.36Mi         18.48Mi       +49.52%                 6.651Mi       -46.19%

                                         │    allocs/op    │  allocs/op   vs base               │  allocs/op   vs base               │
Satisfy/satellites=32/assertions=96-8          25.91k ± 0%   42.37k ± 0%  +63.53% (p=0.002 n=6)   28.90k ± 0%  +11.54% (p=0.002 n=6)
Satisfy/satellites=128/assertions=384-8        102.9k ± 0%   168.8k ± 0%  +63.96% (p=0.002 n=6)   114.9k ± 0%  +11.58% (p=0.002 n=6)
Satisfy/satellites=512/assertions=1536-8       411.1k ± 0%   675.0k ± 0%  +64.21% (p=0.002 n=6)   458.8k ± 0%  +11.61% (p=0.002 n=6)
geomean                                        103.1k        169.0k       +63.90%                 115.1k       +11.58%
```

What remains is the tracing. On the fixed tree's 32-satellite profile
`Context.sharedPath` is 9.5% cumulative and `Context.bindingDeclaredFor`
6.1%, of which 0.86 s of 0.89 s is `bindingRootedAt`: one map lookup per
ancestor level per distinct read, for the object's type and each
classifier it was classified by. `observeRead` is 0.9% of CPU and 134 MB
(3%) of the heap, one 80-byte record per read. That is what the two fixes
ask for: every check records the paths it read so a later destruction or
clock move can find it, and on this benchmark no verdict is ever reused
(each satellite's checks run once per shape and input set), so the
recording buys nothing back. A per-`(instance, path)` memo of
`bindingDeclaredFor` would remove most of the remaining 0.3 ms per 96
assertions but is not safe as written — an object's classifiers change
under `classify`, and the memo would have to be invalidated with them — so
it is left named. The delayed-path-construction attempt recorded in the
previous version of this finding is superseded by the root prefilter
above, which does the same skipping with a measurement behind it.

### 6. Whole-binary validation is twice as fast, single-threaded analysis is dearer — the undeclared-signal lint fixed, the rest priced

Every whole-binary validation is 0.4–0.6× its 0.9.0 wall time while the
per-document analysis benchmarks are slower single-threaded
(`AnalyseResolved` +31% before this change, `ExpandModelImports`,
`GRPCParseFileUncached`, the gRPC cold parse +8–12%) and allocate 5–18%
more (`Analyze/synthetic`, `DecodeSnapshot`, `WorkspaceEdit`). The two
are the same interval seen from two sides: the workspace now runs its
analyses over `ParallelFor`, so a CLI run uses the machine, while each
analysis carries the new lints and the interface records. The
single-threaded cost is what a one-CPU host and the gRPC service's
per-request path will see.

Profiled (`go test ./internal/workspace/model -run '^$' -bench
AnalyseResolved -cpuprofile`), the tree before this change spends 19.8% of
`AnalyseResolved` under `UndeclaredSignalPass.Run`, all of it in
`kit.WalkScoped` → `ast.Inspect`, the reflective document walk: the lint
walked the document once to gather the signals it sends and once more to
check the triggers, and no other pass shares the walk. The pass now reads
`kit.ScopedNodes`, the walk's result kept on the pass context for the
document it analyzes (looked up untracked, so a gather does not come to
depend on the document), and both passes over it iterate the list. The
semantic queries the lint makes per trigger — `DirectSupertypes`,
`implicitBases`, the signal-union walk — were already memoized and
journaled on `semantics.Model`; none does per-symbol work a further
journaled cache would remove, so no new cache is added there.

```
                    │     v0.9.0     │          before this change         │               fixed                 │
                    │     sec/op     │    sec/op     vs base               │    sec/op     vs base               │
AnalyseUnresolved-8      19.45m ± 7%   19.66m ± 11%        ~ (p=0.589 n=6)   19.52m ±  2%        ~ (p=0.937 n=6)
AnalyseResolved-8        1.037m ± 5%   1.359m ±  6%  +30.97% (p=0.002 n=6)   1.178m ± 15%  +13.56% (p=0.002 n=6)
geomean                  4.492m        5.168m        +15.06%                 4.795m         +6.75%

                    │      B/op      │     B/op      vs base              │     B/op      vs base               │
AnalyseUnresolved-8     13.55Mi ± 0%   13.69Mi ± 0%  +1.05% (p=0.002 n=6)   13.69Mi ± 0%   +1.08% (p=0.002 n=6)
AnalyseResolved-8       486.0Ki ± 0%   533.6Ki ± 0%  +9.80% (p=0.002 n=6)   537.7Ki ± 0%  +10.63% (p=0.002 n=6)
```

The +14% and +11% bytes that remain on `AnalyseResolved` are the lints
running once each and the interface records; the live heap a loaded model
retains is half what it was (`LoadModel` live bytes −48%, `Load` live
bytes per element −47%), which is the same interface-record work paying
back on the resident set.

### 7. `GRPCVerifyConstraint` — attributed to implicit nested-usage subsetting, recomputation fixed, output priced

`GRPCVerifyConstraint` +29% time and +19% bytes as found (+23% and +19%
re-measured before this change). Bisected by running the benchmark
across the interval, one commit accounts for it: `7bd51ece6`
(*feat(semantics): implicit nested-usage subsetting and nested\*/owned\*
reflection*, 2026-09-27), against its parent over four runs each:

```
                       │   7bd51ece6^   │            7bd51ece6             │
GRPCVerifyConstraint-8   711.4µ ± ∞ ¹   856.6µ ± ∞ ¹  +20.41% (p=0.029 n=4)
                        472.1Ki ± ∞ ¹   550.6Ki ± ∞ ¹  +16.64% (p=0.029 n=4)
                         8.632k ± ∞ ¹    9.509k ± ∞ ¹  +10.16% (p=0.029 n=4)
```

The benchmark's timed loop is `Service.VerifyConstraint`, 89% of which is
`instanceGraphToProto` materializing every feature value of the subject
for the response. With the commit, every nested usage of the subject
subsets the corresponding usage of its owner's type, so materializing an
instance also materializes those collections: in the CPU profiles
`Context.materializeSubsettedCollections` goes from 4.0% of samples
(0.9.0) to 6.5% (before this change) and `Context.eachSubsetterOf` from 4.4% to
6.9%. The response carries the result — the fixed tree and the one before
it serialize byte-identical graphs, and both differ from 0.9.0 in exactly
the nested usages that now hold `values { instance_id }` — so the growth
in what is materialized is the rule's output, not recomputation.

Around it, three allocations per materialized collection were: the
declared-subsetting names recomputed per read (`declaredSubsettedNames`,
now memoized on `Model.declaredSubsetted` like `subsetted`), a
deduplication set made before knowing whether any instance is
contributed (`appendUniqueInstances`, now made on the first), and a
by-name map and a visited set made per reachability question
(`reachesSubsetted`, `reachesNames`, now a lookup over the owner's values
and a set made on the first edge). Interleaved six-run comparison:

```
                       │    v0.9.0     │      before this change       │            fixed              │
                       │    sec/op     │    sec/op     vs base         │    sec/op     vs base         │
GRPCVerifyConstraint-8    667.7µ ± 67%   868.9µ ± 10%  ~ (p=0.065 n=6)   807.1µ ± 11%  ~ (p=0.240 n=6)

                       │     B/op      │     B/op      vs base               │     B/op      vs base               │
GRPCVerifyConstraint-8    455.8Ki ± 0%   541.7Ki ± 0%  +18.83% (p=0.002 n=6)   510.6Ki ± 0%  +12.01% (p=0.002 n=6)

                       │   allocs/op   │  allocs/op   vs base               │  allocs/op   vs base              │
GRPCVerifyConstraint-8     8.534k ± 0%   9.486k ± 0%  +11.16% (p=0.002 n=6)   9.188k ± 0%  +7.66% (p=0.002 n=6)
```

(The 0.9.0 timing in this run has a 67% spread from one slow round; the
four-run pairs above and the previous record's six runs put it at
668–711 µs.) The remaining +12% bytes and +8% allocations are the
subsetted collections the response now holds, priced to `7bd51ece6`.

### 8. `WriterSiblingBlocks` — attributed, fixed

The migration writer's `WriterSiblingBlocks` was +21% time as found
(+15% re-measured here) and +5.7% bytes at the same 80 k allocations.
`writer.go` changed in one commit of the interval, `101f3b7cd` (*fix(view):
mark migration stand-ins, place shared and strip-drawn pins*,
2026-09-27), which adds a `standIns` list to the `buffer` each block
opens and a second marker check as it closes; against its parent over
four runs it is +4.7% time and +5.7% bytes for the larger buffer. No
other commit touched the writer or its benchmark, so the rest of the
delta has no commit to name and is superseded by the fix.

The profile puts 73% of the benchmark under `writer.trailed` →
`writer.aside` → `writer.line`, and all of its 5.4–5.7 MiB per operation
in `strings.Builder.WriteString` growing a fresh `buffer` per block
(80 k allocations, the same on both revisions). The writer now keeps the
buffers it closes and reuses them for the next block it opens, indents
from a table instead of `strings.Repeat` per line, and grows a line's
builder once for the indent, text and newline:

```
                      │    v0.9.0     │         before this change         │               fixed                │
                      │    sec/op     │   sec/op     vs base               │   sec/op     vs base               │
WriterSiblingBlocks-8    3.648m ± 7%   4.192m ± 6%  +14.91% (p=0.002 n=6)   1.771m ± 4%  -51.46% (p=0.002 n=6)

                      │     B/op      │     B/op      vs base              │     B/op      vs base               │
WriterSiblingBlocks-8   5.381Mi ± 0%   5.687Mi ± 0%  +5.67% (p=0.002 n=6)   1.906Mi ± 0%  -64.58% (p=0.002 n=6)

                      │   allocs/op   │  allocs/op   vs base           │  allocs/op   vs base               │
WriterSiblingBlocks-8    80.03k ± 0%   80.03k ± 0%  ~ (p=1.000 n=6) ¹   20.02k ± 0%  -74.99% (p=0.002 n=6)
```

The output is unchanged (the migration goldens and writer tests pass as
they are). Also left from the previous version of this finding, still
measured and not profiled: `RunStateMachine/elements=4000` +18% against
−12% at 1 000 (noise across the two sizes) and
`WorkspaceEditSmallDocBesideLarge` +31% bytes.

## Verification

On the fixed tree, and again on the tree with findings 5–8's fixes:
`gofmt -l .` is empty, `go vet ./...` and `make lint` (staticcheck, gosec)
pass, `make build` succeeds, and `go test -race ./...` passes. The corpus gates pass with both require variables set
(`OPENSYSML_REQUIRE_TRAINING_CORPUS=1 OPENSYSML_REQUIRE_PILOT_CORPORA=1
go test -count=1 ./tests/corpus -run 'TestTrainingExamples|TestPilotCorpora'`:
100/100 training files clean; the pilot ratchets at 56/58, 92/99 and
56/56, unchanged), as do the SMT-backed tests against `z3`
(`OPENSYSML_REQUIRE_SMT=1 OPENSYSML_SMT=/usr/bin/z3 go test -count=1
./internal/exec/solve ./internal/frontend/repl ./cmd/sysml`). No test or
expectation file was changed, other than the unit test of the removed
`pathKey` now asserting the same dotted-name property of `hasPath`; the
one gate a fix tripped
(`TestSelfModelInstanceLayerMatchesImplementation`, which counts
`runtime.EffectiveFeature`'s fields) was answered by moving the cache off
the struct (`16b3e69a8`), not by changing the model's count.

## Verdict

As found, `develop` was slower than 0.9.0 wherever a run wrote a feature
value in a loop, ran an interpreted calc, or loaded a model with ports:
a loop step 35% slower in twice the allocations, a derived read-write-read
2.2× slower, `SumTo` in 2.2× the bytes, a model load in 27% more bytes per
element and 25–31% longer from files. Four changes in the interval account
for it — the effective-multiplicity rule, the nested-redefinition chains,
a per-statement closure, and the port-type-mismatch lint — and in each
case the cost was recomputation, not the rule.

With the fixes in this change the runtime package is +4% against +32% as
found, loop steps, instantiation and interpreted calcs are at parity, and
the load path is +4% bytes per element with `LoadFiles` within noise at
32 satellites and +13% at 128. Satisfaction checking is +11% time and
+12% allocations geomean against +46% / +64% as found, in half the bytes
of 0.9.0, and keeps +31% on the 32-satellite row for the shared-verdict
tracing two 0.9.1 fixes require; a derived write keeps 28% for the chain
check; single-threaded analysis of a resolved document is +14% against
+31% for the lints and interface records; the gRPC `VerifyConstraint`
keeps +12% bytes for the nested-usage subsetting its response now carries;
and the migration writer is 2× faster than 0.9.0. Every whole-binary
validation is 0.4–0.6× its 0.9.0 wall time and the resident set is
smaller from 6 000 elements up. Nothing is release-blocking.

## Reproducing

```bash
git fetch --tags
git worktree add ../opensysml-v0.9.0 v0.9.0
(cd ../opensysml-v0.9.0 && make build-sysml)
make build-sysml
for pkg in internal/exec/runtime internal/frontend/grpc internal/frontend/lsp internal/frontend/repl \
           internal/translate/migrate internal/workspace/libs internal/workspace/model tests/perf tests/stressmodel; do
  (cd ../opensysml-v0.9.0 && go test ./$pkg -run '^$' -bench . -benchmem -count 6) > old.$pkg.txt
  go test ./$pkg -run '^$' -bench . -benchmem -count 6 > new.$pkg.txt
  benchstat old.$pkg.txt new.$pkg.txt
done
# the rows the fixes target, on the fixed tree
go test ./internal/exec/runtime -run '^$' -bench . -benchmem -count 6 > fix.runtime.txt
go test ./internal/frontend/repl -run '^$' -bench 'CompiledCalc|RunCalc|Instantiate' -benchmem -count 6 > fix.repl.txt
go test ./tests/stressmodel -run '^$' -bench '^Benchmark(Load|LoadFiles|Satisfy|EditImported)/satellites=(32|128)' -benchmem -count 4 > fix.stress.txt
benchstat old.internal/exec/runtime.txt new.internal/exec/runtime.txt fix.runtime.txt
# profiles behind findings 1–5
go test ./internal/exec/runtime -run '^$' -bench 'AssignmentLoopStep|DerivedReadWriteRead' -cpuprofile cpu.out -memprofile mem.out
go test ./tests/stressmodel -run '^$' -bench 'Satisfy/satellites=32|LoadFiles/satellites=32' -cpuprofile cpu.out -memprofile mem.out
# findings 5–8: v0.9.0, the tree before this change (git worktree add ../opensysml-before 17f8a80b5) and the fixed tree, one round per iteration
for i in 1 2 3 4 5 6; do for tree in ../opensysml-v0.9.0 ../opensysml-before .; do (cd $tree
  go test ./tests/stressmodel -run '^$' -bench '^BenchmarkSatisfy/satellites=(32|128|512)/' -benchmem -count 1 >> $OLDPWD/stress.$tree.txt
  go test ./tests/perf -run '^$' -bench '^BenchmarkGRPCVerifyConstraint$' -benchmem -count 1 >> $OLDPWD/grpc.$tree.txt
  go test ./internal/translate/migrate -run '^$' -bench '^BenchmarkWriterSiblingBlocks$' -benchmem -count 1 >> $OLDPWD/migrate.$tree.txt
  go test ./internal/workspace/model -run '^$' -bench '^BenchmarkAnalyse' -benchmem -count 1 >> $OLDPWD/model.$tree.txt
); done; done
benchstat stress.*.txt   # and grpc, migrate, model
# finding 7: the commit, against its parent
git worktree add ../opensysml-bisect 7bd51ece6^ && (cd ../opensysml-bisect && go test ./tests/perf -run '^$' -bench '^BenchmarkGRPCVerifyConstraint$' -benchmem -count 4)
# profiles behind findings 5–8
go test ./tests/stressmodel -run '^$' -bench '^BenchmarkSatisfy/satellites=32/' -cpuprofile cpu.out -memprofile mem.out
go test ./tests/perf -run '^$' -bench '^BenchmarkGRPCVerifyConstraint$' -cpuprofile cpu.out
go test ./internal/translate/migrate -run '^$' -bench '^BenchmarkWriterSiblingBlocks$' -cpuprofile cpu.out -memprofile mem.out
go test ./internal/workspace/model -run '^$' -bench '^BenchmarkAnalyseResolved$' -cpuprofile cpu.out
go tool pprof -top -cum <pkg>.test cpu.out
# whole binary: three interleaved runs of each command per binary
for i in 1 2 3; do for bin in ../opensysml-v0.9.0/bin/sysml bin/sysml; do
  /usr/bin/time -f '%e %M' $bin -validate gen12000.sysml
  /usr/bin/time -f '%e %M' $bin -validate $(find apollo-11-sysml-v2 -name '*.sysml' | sort)
done; done
```
