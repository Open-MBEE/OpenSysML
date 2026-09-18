# Performance: release 0.8.0 against release 0.7.0

A release-gate measurement of `develop` (`f1dc180fe`, 2026-09-12, the
revision the 0.8.0 release is cut from) against `v0.7.0` (`e0fbfea5b`,
2026-09-09, on `main`), following the method of the
[0.6 against 0.4.2 record](performance-release-0.6-vs-0.4.2.md), the
[0.5 against 0.4.3 record](performance-release-0.5-vs-0.4.3.md), the
[September census](performance-census-2026-09.md) and the
[profile](performance-profile-2026-09.md). The question is the release one:
does the 0.8 line perform at least as well as 0.7.0, and where it does not,
why.

All figures were taken on one machine — `Intel Xeon Platinum 8559C`, 8 CPUs,
~31 GiB, Go 1.25.0 (`linux/amd64`) — the processor the two previous records
and the reference figures in `docs/internals/performance.md` were taken on.
The machine is a shared virtual one and its speed drifts by up to 30% over
an hour, so the two revisions were measured back to back, the rows that
drift left in doubt were re-measured interleaved (old, new, old, new, …),
and only the ratios between revisions carry.

Three columns appear throughout: **0.7.0**, **develop** (as found), and
**develop+fix** (with the changes this document ships). *Fixed* means the fix
is in the same change as this record; *explained* means the cause is known
and the cost is the intended price of a feature or rule that landed in the
interval, quantified. Nothing is left *open*.

## Method

- Both revisions built with `make build` into separate worktrees
  (`git worktree add ../opensysml-v0.7.0 v0.7.0`), so `bin/sysml` of each is
  its own binary. The 0.7.0 binary is 42.3 MiB, the 0.8 binary 45.2 MiB.
- Every package that declares a benchmark on both revisions —
  `internal/repl`, `internal/core/model`, `internal/grpc`,
  `internal/perfbench`, `internal/core/libs`, `internal/lsp`,
  `internal/core/runtime`, `internal/core/migrate` and `internal/core/parser`
  — run on both with `go test ./<pkg> -run '^$' -bench . -benchmem -count 6`
  and compared with `benchstat`. A movement is reported when `p ≤ 0.05` and
  the change exceeds about 5%; smaller significant movements are listed as
  noise. Where the fixes moved a row, the as-found and fixed figures are both
  given.
- Whole binary: `sysml -validate` over generated compliant models of 3 000,
  6 000 and 12 000 elements (parts with attributes, constraints and nested
  parts; calc definitions; state definitions), five runs each, the three
  binaries interleaved, wall time and maximum resident set from `getrusage`.
  Every binary reports `no errors` on all three. Process start as the mean
  of 50 `sysml -version` runs; the session floor as an empty `-e 1` session
  and as the `LoadModel/elements=0` benchmark. The `examples/` models present
  on both revisions were run through `-validate`, `-instantiate`, `-action`
  and `-state` on all three binaries, and the Apollo 11 model through
  `-validate`. Both revisions load the library from the embedded snapshot, so
  neither has an on-disk index cache to warm.
- The Python client's `client/python/scripts/bench_latency.py` and
  `bench_transports.py` were run against each revision's `sysml-grpc`, three
  times for the latency script; the transports script also measures the cold
  start of each transport.
- Regressions were profiled with `-cpuprofile`/`-memprofile`, bisected over
  the interval where the profile alone did not name the commit, and
  attributed with `git log -S -- <path>`.

### Workloads that are not comparable

- `internal/perfbench`'s `REPLLoadModel` runs on `develop` only: 0.7.0 fails
  the pilot model it loads with a dimension error (`cannot bind a value of
  dimension L^4·M^2·T^-5 to a feature typed by AccelerationValue`) that the
  0.8 line's unit arithmetic resolves. `ExpandModelImports` in
  `internal/core/libs` exists only on `develop` and is recorded as a
  reference figure. `internal/grpc`'s `InstantiateWarmModel` is added by this
  change, to pin the request path finding 1 fixes; its benchmark file was
  copied into each worktree so all three columns have it.
- `internal/lsp`'s `FormatEdits` formats a fixture that grew from 438 to 597
  lines between the revisions; the +60% the package comparison shows is the
  fixture, not the formatter (finding 9).
- `internal/core/libs`'s `DecodeSnapshot` decodes a snapshot that grew from
  3.58 MB to 3.64 MB and, in the package run, follows a benchmark 0.7.0 does
  not have; the two are separated in finding 8.
- `examples/phase-c-behavioral-bodies.sysml -state PhaseC::AutopilotMode`
  fails to start on all three binaries (`state behavior validateSensors
  performs no action`), so its figure is the time to that refusal.
- No committed baseline file was regenerated. The full suite, with the
  training and pilot corpora required, passes on `develop+fix`.

## Benchmarks: `internal/repl`

| figure | 0.7.0 | develop | develop+fix | develop+fix vs 0.7.0 |
| ------ | ----- | ------- | ----------- | -------------------- |
| load wall, 250 / 1000 / 4000 elements | 17.4 / 70.1 / 295 ms | 23.2 / 91.0 / 367 ms (+34% / +30% / +25%) | 19.7 / 75.7 / 322 ms | +13% / +8% / +9% |
| empty-session load wall | 198 µs | 205 µs | 205 µs (±21%) | +3% |
| state-machine start, 250 / 1000 / 4000 | 7.3 / 7.4 / 7.5 µs | 10.3 / 10.5 / 12.1 µs | 9.8 / 9.7 / 10.3 µs | +33% / +31% / +37% (2.5 µs) |
| calc, 250 / 1000 / 4000 | 1.74 / 1.73 / 1.75 µs | 4.11 / 4.15 / 4.33 µs | 3.58 / 3.77 / 3.65 µs | +105% / +118% / +109% (1.9 µs) |
| instantiate, 250 / 1000 / 4000 | 5.3 / 5.6 / 6.0 µs | 5.8 / 6.0 / 6.3 µs | 6.0 / 5.9 / 6.3 µs | +13% (p=0.03) / ~ / ~ |
| diagnostics, 50 / 200 / 800 attributes | 71.4 µs / 313 µs / 1.39 ms | 71.1 µs / 273 µs / 1.13 ms | 70.5 µs / 278 µs / 1.15 ms | ~ / −11% / −17% |
| CompiledCalc SumTo(1 000 000), interpreted / c / go | 1.53 s / 419 µs / 782 µs | 4.42 s (+189%) / 388 µs / 774 µs | 1.56 s / 387 µs / 771 µs | ~ / ~ / ~ |
| CompiledCalc Collatz(27), interpreted / c / go | 239 µs / 1.61 µs / 5.29 µs | 544 µs (+127%) / 1.00 µs / 5.26 µs | 258 µs / 1.00 µs / 5.28 µs | ~ (p=0.07) / −38% / ~ |
| CompiledCalc Hypot(3.0, 4.0), interpreted / c / go | 4.27 µs / 13.0 ns / 11.4 ns | 7.00 µs / 12.9 ns / 11.5 ns | 6.26 µs / 12.9 ns / 11.6 ns | +46% (2.0 µs) / ~ / ~ |
| CompiledCalc Fib(25), interpreted / c / go | 7.71 ms / 228 µs / 938 µs | 7.64 ms / 225 µs / 923 µs | 7.94 ms / 224 µs / 944 µs | ~ / ~ / ~ |
| geomean | | +24.8% | | +12.3% |

Interleaved re-measurement of the load rows on a quiet machine (three rounds
of `-count 3` on each revision, so `n=9`) gives `LoadModel` +7% (p=0.011) at
250 elements and within noise (p=0.67, p=0.34) at 1 000 and 4 000 elements
(finding 4 prices what remains); an interleaved rerun of the whole package
gives the same +34–39% and +116–120% on `RunStateMachine` and `RunCalc`
(finding 6). The compiled C and Go paths are unchanged, as the compiler is:
the interpreted `SumTo` and `Collatz` rows were the runtime write path of
finding 2, and are back at parity.

## Benchmarks: `internal/core/model` and `internal/grpc`

| benchmark | 0.7.0 | develop | develop+fix | develop+fix vs 0.7.0 |
| --------- | ----- | ------- | ----------- | -------------------- |
| `model` AnalyseUnresolved | 14.0 ms | 18.1 ms (+29%) | 17.2 ms | +23% |
| `model` AnalyseUnresolved bytes / allocs | 9.88 MiB / 134.5 k | 10.55 MiB / 147.8 k | 10.48 MiB / 138.5 k | +6% / +3% |
| `model` AnalyseResolved | 776 µs | 867 µs (+12%) | 762 µs (±22%) | ~ (p=0.39) |
| `model` AnalyseResolved bytes / allocs | 429 KiB / 4.06 k | 453 KiB / 4.41 k | 422 KiB / 3.92 k | −2% / −3% |
| `grpc` ParseFileColdShared | 8.98 ms | 6.85 ms | 6.78 ms | −25% |
| `grpc` ParseFileColdInline | 9.62 ms | 7.01 ms | 6.96 ms | −28% |
| `grpc` ParseFileCold* bytes / allocs | 5.0–5.2 MiB / 35 k | 4.2–4.4 MiB / 27 k | 4.2–4.4 MiB / 27 k | −15% / −22% |
| `grpc` InstantiateWarmModel (added here; measured on all three) | 710 µs, 163 KiB, 5.5 k allocs | 4.38 ms, 1.16 MiB, 15.0 k allocs (+518%) | 405 µs, 117 KiB, 3.2 k allocs | −43% / −29% / −42% |

`AnalyseUnresolved` analyses a document whose names do not resolve and is
the one row that reads worse than 0.7.0 after the fixes by more than the
load path does; finding 5 prices it. The gRPC cold parse is faster on both
`develop` columns: the wildcard-import expansion of the library packages was
reworked in the interval (`ExpandWildcardImports` −23% in `core/libs`,
below). `InstantiateWarmModel` is the request path a client repeats — an
`Instantiate` on a model the service already holds, over the published
vehicle example — and is finding 1.

## Benchmarks: packages without a 0.7.0 baseline

None: every package that declares a benchmark on `develop` declares it on
0.7.0 too, so the tables below compare. `internal/perfbench`, `count 6`,
`sec/op`; `~` is `p > 0.05`.

| benchmark | 0.7.0 | develop | develop+fix | develop+fix vs 0.7.0 |
| --------- | ----- | ------- | ----------- | -------------------- |
| Lex/synthetic / vehicle | 9.03 ms / 273 µs | 7.65 ms / 262 µs | 7.54 ms / 268 µs | ~ / ~ |
| Parse/synthetic / vehicle | 83.6 ms / 2.52 ms | 78.0 ms / 2.11 ms | 84.3 ms / 2.33 ms | ~ / ~ |
| IndexAddExpand/synthetic / vehicle | 49.3 ms / 31.3 ms | 38.3 ms / 23.4 ms | 44.0 ms / 25.9 ms | ~ / −17% |
| IndexAddOnly/synthetic / vehicle | 40.1 ms / 1.96 ms | 39.1 ms / 1.06 ms | 41.4 ms / 1.09 ms | ~ / −44% |
| Analyze/synthetic | 811 ms | 1.20 s (+48%) | 869 ms | +7% back to back; ~ (p=0.39) interleaved |
| Analyze/vehicle | 110 ms | 106 ms | 104 ms | ~ |
| WorkspaceEdit/synthetic (no diagnostics) | 158 ms | 165 ms | 163 ms | +3% |
| WorkspaceEdit/synthetic/reindex+diagnostics | 905 ms | 1.22 s (+35%) | 917 ms | ~ |
| WorkspaceEdit/vehicle / …/reindex+diagnostics | 46.3 / 138 ms | 38.9 / 122 ms | 39.3 / 116 ms | −15% / −16% |
| WorkspaceEditSmallDocBesideLarge | 293 ms | 452 ms (+54%) | 335 ms (±49%) | +14% |
| FQNOf / LookupQualified | 1.23 / 1.16 ms | 1.30 / 1.15 ms | 1.52 (±22%) / 1.24 ms | +24% (noise-bound; see below) / ~ |
| FeaturesOf | 1.43 s | 906 ms | 1.05 s (±60%) | ~ (p=0.07) |
| REPLLoadFile | 1.08 s | 1.38 s (+28%) | 1.12 s (±23%) | ~ (p=0.07) |
| REPLSubmitSnippet / REPLEvalExpr | 993 / 82.9 ms | 1.33 s / 79.0 ms | 1.04 s / 108 ms (±44%) | ~ / ~ |
| LowerActionGraph / LowerStateGraph | 1.43 / 6.91 µs | 2.49 (+74%) / 5.87 µs | 1.59 / 6.41 µs | ~ / ~ |
| LowerChain/chain10 / 100 / 1000 | 7.05 µs / 129 µs / 8.33 ms | 13.1 µs / 291 µs / 23.2 ms (+86% / +126% / +179%) | 6.99 µs / 134 µs / 8.46 ms | ~ / ~ / ~ |
| ExecuteAction / ExecuteActionFreshContext | 34.3 / 35.2 µs | 53.4 / 140 µs (+56% / +297%) | 37.4 / 38.9 µs | +9% / +11% |
| ActionLoop/for10 / 100 / 1000 | 30.7 / 103 / 875 µs | 47.6 / 259 / 2 376 µs (+55% / +151% / +172%) | 33.0 / 115 / 918 µs | ~ / +11% / +5% |
| ActionChain/chain10 / 100 / 1000 | 103 µs / 1.32 ms / 58.1 ms | 167 µs / 2.10 ms / 78.3 ms (+63% / +59% / +35%) | 98.8 µs / 1.33 ms / 57.4 ms | ~ / ~ / ~ |
| ExecuteState | 496 µs | 729 µs (+47%) | 577 µs (±19%) | +16% |
| StateLoop/count50 / 500 / 5000 | 478 µs / 4.27 ms / 41.7 ms | 705 µs / 6.69 ms / 66.2 ms (+48% / +57% / +59%) | 498 µs / 4.80 ms / 47.8 ms | +4% / +12% / +15% |
| BatchConstraints / SameConstraintManyInstances | 6.67 ms / 2.36 µs | 6.99 ms / 2.60 µs (+10%) | 7.44 ms / 2.56 µs | ~ / +8% |
| BatchSatisfy / Instantiate | 1.56 s / 1.09 s | 1.20 s / 409 ms | 1.28 s / 540 ms | −18% / −50% |
| GRPCParseFileCached / GRPCParseFileUncached | 57.2 µs / 117 ms | 61.6 µs (+8%) / 122 ms | 57.9 µs / 109 ms | ~ / −7% |
| GRPCEvaluate | 10.2 µs | 3.47 ms (+33 852%) | 8.06 µs | −21% |
| GRPCVerifyConstraint | 25.1 ms | 173 ms (+590%) | 2.56 ms | −90% |
| ConnectEvaluateHTTP / ConnectParseFileHTTPCached | 146 / 248 µs | 149 / 246 µs | 146 / 254 µs | ~ / ~ |
| geomean | | +39.6% | | −6.7% |

The `develop+fix` column of this package was taken while the machine was
noisier than during the other two (the ± figures show it), so its small
movements are read with the interleaved rerun (three rounds of `-count 2` on
each revision, `n=6`), which put `Analyze/synthetic` (p=0.24),
`WorkspaceEdit/synthetic/reindex+diagnostics` (p=0.13), `FQNOf` (p=0.13),
`ExecuteAction`, `ExecuteState` and `SameConstraintManyInstances` (p=0.07)
within noise of 0.7.0, `WorkspaceEditSmallDocBesideLarge` at −16% (p=0.04)
and `WorkspaceEdit/vehicle` at −9%, and `StateLoop/count5000` at +22%
(p=0.002; the state-machine cost finding 6 prices). `BatchSatisfy`
and `Instantiate` are faster on both `develop` columns: instantiation stopped
re-deriving the library features an object carries.

The other packages:

| package / benchmark | 0.7.0 | develop | develop+fix | develop+fix vs 0.7.0 |
| ------------------- | ----- | ------- | ----------- | -------------------- |
| `core/libs` ExpandWildcardImports / IndexLibrary / SetDigest | 32.3 ms / 14.0 ms / 799 µs | 24.9 ms / 13.8 ms / 559 µs | 25.0 ms / 13.7 ms / 558 µs | −23% / −3% / −30% |
| `core/libs` DecodeSnapshot, in the package run | 8.89 ms | 14.1 ms | 13.4 ms | +51% (finding 8) |
| `core/libs` DecodeSnapshot, run alone, bytes / allocs | 8.46 ms, 31.7 MiB, 66.2 k | 8.49 ms, 29.5 MiB, 45.8 k | 8.49 ms, 29.5 MiB, 45.8 k | ~ (p=0.59), −7%, −31% |
| `core/libs` ExpandModelImports | — | 5.78 ms | 5.77 ms | reference |
| `lsp` FormatEdits, each revision's own fixture | 304 µs (438 lines) | 492 µs (597 lines) | 487 µs | see finding 9 |
| `lsp` FormatEdits, 0.7.0 on the 597-line fixture | 483 µs | 492 µs | 487 µs | ~ (p=0.31) |
| `lsp` ReferencesWarm / ReferencesCold | 119 µs / 28.5 ms | 120 µs / 27.5 ms | 127 µs / 26.8 ms | +7% / −6% |
| `lsp` RenameWarm / WorkspaceUpdate | 53.3 / 288 ms | 58.9 (+10%) / 256 ms | 56.9 / 260 ms | +7% / −10% |
| `core/parser` ParseModel over `examples/pilot-corpora/sysml-examples` (99 files, 8 506 lines) | 11.0 ms | 11.2 ms | 11.2 ms | ~ |
| `core/runtime` MaterializePlainAttributes | 1.31 ms | 861 µs | 755 µs | −42% |
| `core/runtime` SetFeatureValueNoDependents | 163 ns | 1 477 ns (+807%) | 132 ns | −19% |
| `core/runtime` DerivedReadWriteRead | 2.65 µs | 6.81 µs (+157%) | 2.61 µs | ~ |
| `core/runtime` AssignmentLoopStep | 886 µs | 1.61 ms (+82%) | 852 µs | ~ |
| `core/migrate` WriterSiblingBlocks | 4.17 ms | 3.44 ms | 3.49 ms | ~ (p=0.07) |

## Whole-binary scaling

`sysml -validate` over generated compliant models that report `no errors`,
five runs each with the three binaries interleaved; median wall seconds and
median maximum resident set.

| elements | 0.7.0 | develop | develop+fix | develop+fix vs 0.7.0 |
| -------- | ----- | ------- | ----------- | -------------------- |
| 3 000 | 0.265 s / 130 MiB | 0.312 s / 130 MiB (+18%) | 0.286 s / 144 MiB | +8% / +11% |
| 6 000 | 0.508 s / 182 MiB | 0.612 s / 204 MiB (+20%) | 0.555 s / 199 MiB | +9% / +9% |
| 12 000 | 1.054 s / 322 MiB | 1.263 s / 329 MiB (+20%) | 1.105 s / 321 MiB | +5% / ±0% |

Scaling is linear on all three. As found, `develop` was a fifth slower at
every size, half of it the load-path fixes of finding 4 and half the new
validation rules of finding 7; with the fixes the slope is 5–9% above
0.7.0, all of it in those rules. The resident set is within a run's spread
of 0.7.0 — larger by 14 MiB at 3 000 and 6 000 elements, level at 12 000 —
and the difference at the small sizes is the 3 MiB the larger binary maps
plus the side tables the new passes keep (finding 10).

### Process start and session floor

| figure | 0.7.0 | develop | develop+fix |
| ------ | ----- | ------- | ----------- |
| `sysml -version`, mean of 50 | 3.61 ms | 4.01 ms | 4.09 ms |
| binary size | 42.3 MiB | 45.2 MiB | 45.2 MiB |
| empty session (`sysml -e 1`), median of 5 | 0.021 s / 58 MiB | 0.021 s / 56 MiB | 0.021 s / 56 MiB |
| empty-session load (`LoadModel/elements=0`) | 198 µs | 205 µs | 205 µs |

Start-up is 0.5 ms (+13%) slower per process and the session floor is
unchanged. The growth is the binary: `sysml` links the analysis framework
and its engines, the MOSA and DiagramLayout libraries, and the `ListEngines`
surface of the gRPC service, none of which 0.7.0 linked, so it maps 3 MiB
more and runs their package initialisers. This is the finding the two
previous records made for their intervals and is **explained** (finding 10).

### Example models

Median wall time and maximum resident set of three runs; every run returned
the same exit status on all three binaries.

| model / command | 0.7.0 | develop | develop+fix |
| --------------- | ----- | ------- | ----------- |
| `action-executor-demo.sysml -validate` | 0.027 s / 61 MiB | 0.027 s / 59 MiB | 0.026 s / 60 MiB |
| `action-executor-demo.sysml -action ActionExecutorDemo::sequential` | 0.026 s / 64 MiB | 0.026 s / 62 MiB | 0.025 s / 62 MiB |
| `orthogonal-regions-demo.sysml -state …::TrafficLight -advance 20` | 0.045 s / 72 MiB | 0.041 s / 68 MiB | 0.041 s / 68 MiB |
| `phase-c-behavioral-bodies.sysml -instantiate PhaseC::Vehicle` | 0.032 s / 63 MiB | 0.031 s / 62 MiB | 0.030 s / 62 MiB |
| `phase-c-behavioral-bodies.sysml -state PhaseC::AutopilotMode -advance 20` (refused on all three) | 0.029 s / 63 MiB | 0.031 s / 62 MiB | 0.028 s / 62 MiB |
| `disposal-robot-demo/robot.sysml -validate` | 0.050 s / 70 MiB | 0.045 s / 67 MiB | 0.044 s / 67 MiB |

At example scale the 0.8 binary is level with or a few milliseconds ahead of
0.7.0 on every command, and uses 1–4 MiB less: below 100 elements the run is
the library snapshot's decode and the process start, and the 8% the new
passes add to a 3 000-element validation is under a millisecond here.

#### The Apollo 11 model

The public [Apollo 11 SysML v2 model](https://github.com/airbus/apollo-11-sysml-v2)
(commit `6e9c93f`, 28 files, 7 221 lines) is the real model behind the
README's figures; see [performance](../internals/performance.md#a-real-model-apollo-11)
for what its run reports. Medians over eight warm runs:

| what | 0.7.0 | develop | develop+fix |
| ---- | ----- | ------- | ----------- |
| `sysml -validate` all 28 files | 0.423 s / 172 MiB | 0.397 s / 152 MiB | 0.381 s / 152 MiB |
| what the run reports | 37 warnings, no error | 4 warnings, no error | 4 warnings, no error |

The 0.8 line validates the model 10% faster and in 20 MiB less than 0.7.0,
as found and fixed alike. It also reports 33 fewer warnings: 0.7.0 read
`part` in a usage such as `part part : Foo;` as the usage's kind and warned
that the usage is unnamed, 33 times over this model; `ea4962d82` (*reject
prefix conflicts, bad escapes and misplaced body members the grammar
forbids*, 2026-09-11) parses it as the grammar does, so the warning is gone
and with it the resolution of a name that was never declared. The four
warnings both revisions report are the same four. The gain outweighs the
new passes' cost on this model because its imports of the quantity
libraries dominate the load, and the wildcard-import expansion is 23%
faster (`ExpandWildcardImports`).

### The Python client

`bench_latency.py` (20 part defs, 300 iterations, p50 milliseconds, the
best of three runs each) and `bench_transports.py` (300 iterations, p50 /
p95 milliseconds) against each revision's `sysml-grpc`, both on this
machine; only the rows the same script ran on both revisions are compared.

| operation | 0.7.0 | develop | develop+fix |
| --------- | ----- | ------- | ----------- |
| connect + first parse | 6.9 ms | 4.7 ms | 4.3 ms |
| `load_from_content`, cache hit / miss | 0.22 / 0.97 | 0.25 / 1.06 | 0.24 / 1.09 |
| `eval 2 + 2` | 0.22 | 0.22 | 0.21 |
| `convert` sysml → sysml / sysml → ttl / ttl → sysml | 0.47 / 2.51 / 6.47 | 0.53 / 2.54 / 7.40 | 0.52 / 2.48 / 6.42 |
| gRPC over TCP, small model: ParseFile / Evaluate / Query / Instantiate | 0.14 / 0.13 / 0.28 / 0.49 | 0.13 / 0.15 / 0.31 / 1.65 (+237%) | 0.15 / 0.16 / 0.33 / 0.34 |
| gRPC over TCP, large model: ParseFile / Evaluate / Query / Instantiate | 0.32 / 0.17 / 5.69 / 0.27 | 0.30 / 0.11 / 5.46 / 4.87 (+1 700%) | 0.32 / 0.13 / 5.30 / 0.17 |
| stdio protobuf, large model: Query / Instantiate | 4.74 / 0.15 | 4.88 / 4.62 | 4.82 / 0.11 |
| cold start to first RPC: gRPC TCP / Connect protobuf / Connect JSON / stdio protobuf / stdio JSON | 7.4 / 6.5 / 6.1 / 4.5 / 4.9 | 8.9 / 7.4 / 6.6 / 4.4 / 4.7 | 8.0 / 6.0 / 5.8 / 4.7 / 5.0 |

The client sees one movement: `Instantiate` on a held model, 3–18× slower
as found and 30–40% faster than 0.7.0 fixed, which is finding 1 through the
wire. Parse, evaluate, query and conversion are within a run's spread of
0.7.0 on all transports; the cold start is within the ±1.5 ms run-to-run
spread of a process spawn.

### Profile-guided optimisation

The binaries build against `cmd/sysml/default.pgo`, `cmd/sysml-grpc/default.pgo`
and `cmd/sysml-lsp/default.pgo`, last regenerated by `369a42174` (*regenerate
the PGO profile on the merged calc and stdlib work*, 2026-09-02) — before
the analysis framework, the runtime's split into a shared model and
run-derived state, the gRPC worker pool, and the validation passes added in
the interval. The profiles are therefore stale for the hot paths this record
measures: `analysis.Perform` and the registry dispatch, `passes.TypeCheckPass`
and `ControlNodeSuccessionPass`, the runtime's write-conformance path and
the `CachedModel` request path post-date them. Both revisions are built with
their own checked-in profile, so the comparison is fair; the profile was
not regenerated for this record, which measures the code, not the profile.
Regenerating it with `make pgo-profile` is part of the release procedure
and is where its effect belongs.

## Findings

Ordered by size. Each names the cause, the responsible change, the measured
cost, and its status.

### 1. The gRPC service built an analysis worker per request — *fixed*

`GRPCEvaluate` 10.2 µs → 3.47 ms (+33 852%), `GRPCVerifyConstraint` 25 ms →
173 ms (+590%), `InstantiateWarmModel` 710 µs → 4.38 ms (+518%) with seven
times the bytes, and through the Python client `Instantiate` 0.49 → 1.65 ms
on a small model and 0.27 → 4.87 ms on a large one. Commit `89c10fc21`
(*a resolver and semantic model per plan, a context per run*, 2026-09-10)
gave the analysis framework one `analysis.Worker` — a resolver and a
semantic model over the shared frozen index — per plan, and had the gRPC
service build one per *request* instead of serialising runtime requests on
one shared pair. Every `Evaluate`, `Instantiate`, `VerifyConstraint`,
`Sweep` and derived-context request then rebuilt the resolver and the
semantic side tables from nothing, so a 10 µs evaluation paid a 3.5 ms
construction, and the workers were retained until the model was evicted.

**Fixed**: `CachedModel` keeps an idle pool of workers. A request takes one
(building it only when the pool is empty), and its release trims the
resolver diagnostics the request added and returns the worker to the pool;
the release runs on the success, error and cancellation paths alike, and a
context derived from a request's context is given a no-op release so only
the owning request returns the worker. Two concurrent requests still get two
workers and share nothing that memoizes, which is what `89c10fc21` set out
to guarantee (`internal/grpc/cache_test.go`, `evaluate_retention_test.go`
and `budget_test.go` cover the isolation, the bounded retention and the
budget). `GRPCEvaluate` is 8.06 µs (−21% against 0.7.0), `GRPCVerifyConstraint`
2.56 ms (−90%: verification no longer re-resolves the model either),
`InstantiateWarmModel` 405 µs (−43%) in 117 KiB, and the client's
`Instantiate` 0.34 / 0.17 ms (−30% / −37%).

### 2. Every scalar write walked the type-difference closure — *fixed*

`SetFeatureValueNoDependents` 163 ns → 1 477 ns (+807%), `DerivedReadWriteRead`
+157%, `AssignmentLoopStep` +82%, `ExecuteActionFreshContext` +297%,
`ActionLoop/for1000` +172%, `StateLoop` +48–59%, and the REPL's interpreted
`SumTo` +189% and `Collatz` +127%. Bisected to `199af919b` (*judge a scalar's
type by one shared classification*, 2026-09-10): every write of a scalar to
a typed feature classifies the value against the feature's type, and the
classification asks `semantics.Model.excludes` whether a difference type
anywhere above the target subtracts the value's type — a breadth-first walk
over the target's supertypes, intersections and unions, on every write,
with a fresh visited map. The same commit and `9f80a56c5` (*judge open
operands by their declared type*, 2026-09-12) formatted the write's
description (`fmt.Sprintf("feature value %s.%s", …)`) before knowing whether
the write would be refused, and `ClockWait` formatted its holder and
description on every timed wait.

**Fixed**: whether a type's closure subtracts anything is memoized per type
(`Model.subtracting`), under the resolver's re-entrancy guard so a
provisional answer during a cyclic resolution is not pinned; a write's
description is a closure the refusal path calls (`checkTargetAs`,
`checkAdmits`), so a conformant write formats nothing; `ClockWait` carries
its holder and what it waits on and formats them when asked; the runtime
memoizes the library symbol a qualified name denotes, preallocates the type
list a scalar classification returns, and `comparisonValues` sets an operand
error's span only when there is an error. The diagnostics are byte-for-byte
those the eager code produced; the conformance tests
(`internal/core/runtime/conformance_test.go`, `value_conformance_test.go`,
the execution conformance suite) are unchanged. `SetFeatureValueNoDependents` is 132 ns (−19% against 0.7.0),
`DerivedReadWriteRead` and `AssignmentLoopStep` are at parity, `SumTo` and
`Collatz` are at parity, `ActionLoop/for1000` +5%, `ExecuteActionFreshContext`
+11%. What remains on the execution rows is finding 6.

### 3. Lowering computed every action's static footprint eagerly — *fixed*

`LowerChain/chain1000` 8.3 ms → 23.2 ms (+179%), `LowerActionGraph` +74%,
`ActionChain/chain1000` +35% (it lowers first). Commit `0ce5ae303` (*compute
static footprints for the model checker's independence relation*,
2026-09-11) computes, for every node of an `ActionGraph`, the features it
reads and writes, so the model checker can tell independent moves apart.
Lowering computed all of them as it built the graph — the model checker is
the only consumer, and the REPL, the executors and the gRPC service never
ask — and each footprint re-scanned the graph's declared features to
classify a name, so a 1 000-node chain scanned its feature list a thousand
times.

**Fixed**: `ActionGraph.Footprints()` computes the map once on first use
(`sync.Once`), and the footprints of a graph share one `declaredFeatures`
map built from the graph's feature list. The footprints are the same ones
(`footprint_test.go` compares them node by node); the model checker's
`c24d003a7` (*treat any two message moves as dependent*) is unaffected.
`LowerChain`, `LowerActionGraph` and `ActionChain` are at parity with 0.7.0
at every size.

### 4. Load path: memoization the interval's semantics stopped providing — *fixed*

`LoadModel` +25–34%, `Analyze/synthetic` +48%,
`WorkspaceEdit/synthetic/reindex+diagnostics` +35%,
`WorkspaceEditSmallDocBesideLarge` +54%, `REPLLoadFile` +28%,
`AnalyseResolved` +12% as found. The load profile against 0.7.0 showed no
single new cost: the passes of finding 7 ask the semantic model more
questions per element, and several of the queries they lean on were
answered from scratch on every call — on 0.7.0 too, where fewer callers
made it cheap:

- `ElementMetadataOf` re-ranked the index's documents per element, a map
  built and dropped on every call since `e4aaf2412` (*order elem.metadata
  by source position across inline and about forms*, 2026-09-10); the
  identity-metadata pass asks it for every element.
- `implicitBases` — the library base a declaration's kind implies — was not
  memoized; the OOSEM, type-check and conformance passes ask it per feature.
- Whether an operand is a measurement reference (a unit or scale named in
  an expression) was answered by `UnitTermOfExpr`, which builds the
  diagnosis of why a name is *not* a unit — and most operands probed are
  not; the type-check pass of `1eb0d53f0` probes every operand.
- `symbols.Scope` kept its name index and had no declaration index, so
  `memberDeclaring` scanned (and copied) a scope's members for each
  declaration it registered, and unqualified resolution copied a scope's
  anonymous members on every lookup.
- The did-you-mean suggestion table walked every registered qualified name
  and matched each by suffix (finding 5 has the rest of it).
- `analysis.Registry` sorted its engines by kind on every dispatch
  (`b9fe2cbb8`, *add the analysis framework contract and registry*,
  2026-09-10).

**Fixed**: the document ranks, `implicitBases` and the per-kind engine
lists (dropped on `Register`) are memoized, `implicitBases` under the
resolver's `Enter`/`Leave` guard so a cyclic resolution cannot pin a
provisional answer; the measurement-reference probe resolves the name and
asks whether it is a unit, building no diagnosis (`measurementRefNamed`);
a scope's name and declaration indexes are one atomically-replaced record,
and `ForEachAnonymousMember` visits without copying; the suggestion table
takes the index's registered names directly (`Index.Registered`) and
matches the qualified name exactly. Diagnostics are unchanged (the full
suite and both corpora pass). `LoadModel` is +7% at 250 elements and within
noise at 1 000 and 4 000 against 0.7.0 interleaved (+8–13% back to back), and
allocates fewer bytes at every size;
`Analyze/synthetic` and `WorkspaceEdit/synthetic/reindex+diagnostics` are
within noise of 0.7.0 interleaved, `AnalyseResolved` is at parity with 3%
fewer allocations, `WorkspaceEditSmallDocBesideLarge` +14%. What remains is
finding 7.

### 5. Did-you-mean suggestions over declared names — *explained*

After the fixes, `AnalyseUnresolved` is +23% (3.2 ms on 14.0) with +6%
bytes. The workload is a document whose names do not resolve, so every
diagnostic carries a suggestion; the suggestion table it builds covers,
since `3e593a901` (*suggest the quoted spelling of an unquoted declared
name*), `0b03e62e5` (*offer only reachable quoted members, rooted as
written, for calls too*) and `e5e72dd4e` (*quote a declared name holding
`::` whole in quoted-name hints*, all 2026-09-10), the quoted spellings of
declared names and the members reachable from the failing reference,
rooted as the reference wrote them, and answers each unresolved name with
the closest of them. The table is built once per analysis and is linear in
the model's names; on a document whose names resolve (`AnalyseResolved`,
`Diagnostics`, every load) it is not built at all — `Diagnostics` is
−11–17% and `AnalyseResolved` at parity. The cost is confined to the
diagnostics a user asked for and is the rule doing what it says; it is
**explained**.

### 6. Analysis-framework dispatch on every REPL calc and state run — *explained*

After the fixes, `RunCalc` is +105–118% (1.9 µs on 1.7) and
`RunStateMachine` +31–37% (2.5 µs on 7.4), constant in model size, and the
interpreted `Hypot(3.0, 4.0)` — a calc too short for its body to matter —
+46% (2.0 µs); `ExecuteAction` +9%, `ExecuteActionFreshContext` +11%. The
0.8 line runs every `%calc`, `%state` and `%action` through the analysis
framework (`b9fe2cbb8`, the contract and registry; `89c10fc21`, a worker
per plan and a context per run; `4a3fd5ed5`, *add %engines, %engine,
-engines, -engine and the standing line*, all 2026-09-10): the session
builds a plan, `Registry.AnswerWith` ranks the engines declaring its kind,
`analysis.Perform` runs the chosen one in a context of its own, and the
verdict is folded into the standing line. In the `RunCalc` profile that is
`Perform`, `AnswerWith`, `run`, `Session.calcVerdict` and `Session.evalCalc`,
27 more allocations per run (34 → 61); the evaluation itself is unchanged.
Two microseconds per command is what a user can select the engine for a
run and read its verdict with, and is **explained**; the interval's own
regression on these rows — the registry re-sorting per dispatch — is fixed
in finding 4.

### 7. New validation rules on the load path — *explained*

After the fixes the whole binary validates 3 000 / 6 000 / 12 000 elements
in +8% / +9% / +5% of 0.7.0's time and `LoadModel` is +7% at 250 elements,
within noise above that. The interval
added the warning-only MOSA conformance pass (`ee1a073ac`, 2026-09-11), the
DiagramLayout library and its constraint-tier validation (`95d98f373`,
`0737ab8c0`, 2026-09-12), non-conforming operator and invocation argument
warnings (`d68eb4a5f`, 2026-09-12), typing of bare feature references and
computed values (`1eb0d53f0`, 2026-09-11), and the control-node succession
rule's walk through inherited actions (`Model.ActionSuccessions`). In the
fixed 12 000-element profile the passes above name resolution are
`ControlNodeSuccessionPass.Run` with `ActionSuccessions` beneath it,
`TypeCheckPass.Run`, `IdentityMetadataPass.Run`, the feature-reference
walker and `ConstraintPass.Run`, each linear in the model and reading its
subjects once; parsing and name resolution are unchanged. The `ExecuteState`
+16%, `StateLoop` +12–15% and `SameConstraintManyInstances` +8% rows are the
same shape at the runtime: a write to a multi-valued feature now refuses a
duplicate (`2f6206c42`, *enforce uniqueness of multi-valued features*,
2026-09-10), and a body-local declaration is held to its declared type
(`dd5e1d7a2`, 2026-09-12), judgements the interval added and finding 2 made
free of their descriptions. These are the rules doing their work and are
**explained**.

### 8. `DecodeSnapshot` in the package run — *explained*

`core/libs` `DecodeSnapshot` reads +51% in the package comparison and ~
(p=0.59) when run alone, with 7% fewer bytes and 31% fewer allocations.
Two things changed: the embedded library snapshot grew from 3.58 to 3.64 MB
(the MOSA and DiagramLayout libraries), which the run-alone figure prices at
nothing measurable; and `develop` declares `ExpandModelImports`, which the
package run executes immediately before `DecodeSnapshot` and which leaves
the shared library base resident in the process, so the decoder's
allocations are collected against a 30 MiB larger live heap — 8.9 ms alone,
12.5–13.6 ms after `ExpandModelImports` on the same binary. The decode
itself is faster per byte and the binary's start (above) shows no such
movement; **explained**.

### 9. `FormatEdits` measures a larger fixture — *explained*

`lsp` `FormatEdits` +60% between the revisions' own runs. The fixture the
benchmark formats grew from 438 to 597 lines in the interval; the 0.7.0
formatter on the 597-line fixture takes 483 µs against 487 µs fixed
(p=0.31). The formatter has not moved. `RenameWarm` +7% and
`ReferencesWarm` +7% re-analyse the workspace and carry the passes of
finding 7. **Explained**.

### 10. Process start, resident set — *explained*

Start-up +0.5 ms (+13%) per process and +14 MiB resident at 3 000 and 6 000
elements, level at 12 000: the binary is 3 MiB larger for the analysis
framework and its engines, the two new libraries and the `ListEngines`
surface, and the new passes keep side tables (identity metadata, metadata
ranks, member sources, the memoizations of finding 4) whose fixed part
shows at the small sizes and is amortised by 12 000 elements. The empty
session is unchanged at 21 ms. **Explained**.

## Verdict

As found, `develop` was not on par with 0.7.0 anywhere a request or a run
touched the analysis framework or the runtime's write path: the gRPC
service rebuilt its resolver and semantic model on every request (`Evaluate`
340×, `VerifyConstraint` 7×, `Instantiate` 6× slower, and the Python client
saw it), every scalar write walked the type-difference closure (writes 9×,
action loops 2.7×, interpreted calcs 2–3× slower), lowering computed every
action footprint eagerly (chain lowering 2.8× slower), and the load path
had lost several memoizations (loading and validating a model 20–48%
slower). Four changes in the interval account for all of it, none of them
a rule's intended cost.

With the fixes in this change, `develop` serves `Evaluate` 21% and
`VerifyConstraint` 90% faster than 0.7.0 and instantiates on a held model
43% faster in 29% of the bytes; writes are 19% faster and action and chain
lowering and execution loops are at parity; the gRPC cold parse is 25–28%
faster, wildcard-import expansion 23%, instantiation and satisfaction of a
batch 18–50%, diagnostics 11–17%, and the Apollo 11 model validates 10%
faster in 20 MiB less. Against that, a whole-binary validation is 5–9%
slower and a REPL load 6–7%, all of it in the validation rules added since
0.7.0; a REPL calc or state run costs 2 µs more for going through the
analysis framework; unresolved-name diagnostics cost 23% more for their
richer suggestions; and process start is 0.5 ms slower for the larger
binary. Nothing is release-blocking. The PGO profiles predate every hot
path this record moved and are due for regeneration at the release cut.

## Reproducing

```bash
git fetch --tags
git worktree add ../opensysml-v0.7.0 v0.7.0
(cd ../opensysml-v0.7.0 && make build)
make build
./scripts/download-pilot-corpora.sh
for pkg in internal/repl internal/core/model internal/grpc internal/perfbench \
           internal/core/libs internal/lsp internal/core/runtime internal/core/migrate; do
  (cd ../opensysml-v0.7.0 && go test ./$pkg -run '^$' -bench . -benchmem -count 6) > old.$pkg.txt
  go test ./$pkg -run '^$' -bench . -benchmem -count 6 > new.$pkg.txt
  benchstat old.$pkg.txt new.$pkg.txt
done
export OPENSYSML_BENCH_MODEL=examples/pilot-corpora/sysml-examples
(cd ../opensysml-v0.7.0 && go test ./internal/core/parser -run '^$' -bench ParseModel -benchmem -count 6)
go test ./internal/core/parser -run '^$' -bench ParseModel -benchmem -count 6
# rows the machine's drift leaves in doubt: interleave the revisions
PB='Analyze|WorkspaceEdit|FQNOf|REPLLoadFile|ExecuteAction|ExecuteState|StateLoop|SameConstraint|ActionLoop'
for i in 1 2 3; do
  (cd ../opensysml-v0.7.0 && go test ./internal/perfbench -run '^$' -bench "$PB" -benchmem -count 2) >> old.i.txt
  go test ./internal/perfbench -run '^$' -bench "$PB" -benchmem -count 2 >> new.i.txt
  (cd ../opensysml-v0.7.0 && go test ./internal/repl -run '^$' -bench LoadModel -benchmem -count 3) >> old.load.txt
  go test ./internal/repl -run '^$' -bench LoadModel -benchmem -count 3 >> new.load.txt
done
benchstat old.i.txt new.i.txt
benchstat old.load.txt new.load.txt
go test ./internal/core/libs -run '^$' -bench DecodeSnapshot -benchmem -count 6   # alone
go test ./internal/core/runtime -run '^$' -bench SetFeatureValueNoDependents -cpuprofile sfv.cpu
go test ./internal/repl -run '^$' -bench 'RunCalc/elements=250' -cpuprofile calc.cpu
../opensysml-v0.7.0/bin/sysml -validate gen12000.sysml
bin/sysml -validate gen12000.sysml
for i in $(seq 50); do bin/sysml -version; done
bin/sysml-grpc -port 50123 -health-port 50124 -log-level error &
(cd client/python && python3 scripts/bench_latency.py --port 50123 --iterations 300)
(cd client/python && python3 scripts/bench_transports.py --iterations 300 --binary ../../bin/sysml-grpc)
```
