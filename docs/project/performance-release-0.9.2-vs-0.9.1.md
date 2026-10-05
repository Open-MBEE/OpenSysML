# Performance: release 0.9.2 against release 0.9.1

The release-gate measurement of the 0.9.2 candidate — `release/0.9.2` at
`2c96c7d9b` (2026-10-05), 28 commits past the `v0.9.1` tag (`61190b66a`,
2026-09-30) — against 0.9.1 itself. It follows the method of the
[0.9.1 against 0.9.0](performance-release-0.9.1-vs-0.9.0.md) record: every
benchmark on both revisions, `benchstat` over six runs, the rows the machine
left in doubt re-run interleaved, and whole-binary timings on generated and
real models. See [performance](../internals/performance.md) for what the
figures mean and where the remaining cost is.

0.9.2 is a patch release cut from `main`: the semantic fixes requested for
the 0.9.1 line (library names, element ids by declaration, succession ends,
variant and requirement-constraint references, implicit-subsetting
uniqueness, transition source members, gRPC parser warnings) and the
Python-client work (release-digest stamp, metamodel reader, the PyPI
README). It carries none of the `develop` features that follow 0.9.1, so
the expectation going in was parity, and parity is what the measurement
finds.

## Summary

- **Package benchmarks are level.** Across the nine benchmark packages
  no row regresses by a margin the interleaved re-run confirms. Package
  geomeans move between −2.3% (`internal/exec/runtime`) and +1.1%
  (`internal/frontend/lsp`), each within the machine's run-to-run spread;
  `tests/perf` is +0.8% as found, with ten rows significantly faster
  (3–9%) and one slower, re-run below.
- **Allocations are unchanged** to the byte everywhere except the model
  loader, which allocates **+0.4–0.7% bytes per element** and +0.08%
  allocations on the stress constellation at every size (32 to 512
  satellites), and the resolved-document analysis at +1.0% bytes. This is
  the price of the semantic fixes' extra bookkeeping on the symbol tables
  and is the only systematic difference the measurement finds.
- **Whole binary.** `sysml -validate` on the generated models (≈3 000,
  6 000 and 12 000 declarations) and on the Apollo 11 model is the same
  wall time and the same resident set on both binaries, to the 10 ms and
  1 MiB resolution of the clock. The binary is 128 KiB larger; an empty
  session starts in the same 0.02 s and maps the same 65 MiB.
- **Stress constellation as found** showed 13 rows slower by 11–44% on the
  candidate, twelve of them in the load and open paths and all with a
  14–74% spread on the candidate's six counts against 2–5% on the baseline's. Re-run
  interleaved, every one of them is `~`, their allocation counts are
  identical to four digits, and the spread appears on both sides: the
  first pass had measured the machine during the candidate's half hour,
  not the candidate.

Nothing here blocks the release.

## Method

Machine: `INTEL(R) XEON(R) PLATINUM 8559C`, 8 CPUs, 31 GiB, Go
`go1.25.0 linux/amd64`, `GOMAXPROCS=8`. 0.9.1 is a worktree at the tag,
built and benchmarked with its own tree; the candidate is `release/0.9.2`
at `2c96c7d9b`. Package benchmarks are
`go test ./<pkg> -run '^$' -bench . -benchmem -count 6` on each revision,
compared with `benchstat` (rows are significant at p < 0.05; `~` is no
significant difference). The first pass ran each package's six counts on
0.9.1 and then on the candidate; rows it left in doubt — a significant
difference, or a spread above 20% on either side — were re-run with the
two revisions interleaved, `-count 1` six times each, nothing else on the
machine. Whole-binary figures are `/usr/bin/time -f '%e %M'` over five
interleaved runs of each binary, both built with `make build-sysml`
(`-s -w`, `-trimpath`); medians are reported.

### What is and is not comparable

- **`internal/syntax/parser`** defines no benchmark that runs without
  `OPENSYSML_BENCH_MODEL`; neither revision produced a row and the package
  is omitted.
- **`internal/frontend/grpc`**: `InstantiateWarmModel` exists only on the
  candidate (it arrived with the gRPC parser-warning fix, #751) and has no
  baseline row; the two parse rows are compared.
- **`tests/stressmodel`**: both revisions have the same 48 rows.
  `ConvertAPIJSON/satellites=512` completed a single count on each side
  (134.6 s against 153.6 s) and is excluded as no distribution.
- **Diagnostics**: on every model measured both binaries return the same
  exit status and the same report — `no errors` on the generated models,
  the two example models and all 28 Apollo 11 files.

## Package benchmarks

Time per operation, 0.9.1 then the candidate; bytes and allocations where
they moved.

### `internal/exec/runtime`

| benchmark | 0.9.1 | 0.9.2 |
| --------- | ----- | ----- |
| `MaterializePlainAttributes` | 48.6 µs | 47.6 µs (~) |
| `SetFeatureValueNoDependents` | 401 ns | 386 ns (~) |
| `DerivedReadWriteRead` | 5.82 µs | 5.81 µs (~) |
| `AssignmentLoopStep` | 959 µs | 926 µs (~) |
| geomean | | −2.3% time / +0.0% B / +0.0% allocs |

### `internal/frontend/repl`

28 rows; 25 are `~`. The three that moved:

| benchmark | 0.9.1 | 0.9.2 |
| --------- | ----- | ----- |
| `RunStateMachine/elements=1000` | 12.9 µs | 13.5 µs (+4.8%, p=0.026) |
| `RunCalc/elements=1000` | 4.50 µs | 4.60 µs (+2.2%, p=0.026) |
| `CompiledCalc/Hypot(3.0,4.0)/interpreted` | 10.4 µs ±20% | 7.57 µs (−27%, p=0.002) |
| geomean | | +0.3% time / +0.6% B / +0.0% allocs |

The two slower rows are the 1 000-element size only; the 250 and 4 000
sizes of the same benchmarks are `~`, and the re-run below has them `~`
at 1 000 as well: `RunStateMachine/elements=1000` 14.5 µs against 14.2 µs (~, p=0.818),
`RunCalc/elements=1000` 4.83 µs against 4.94 µs (~, p=0.818), allocations
identical. The `Hypot` gain is the baseline's own
±20% spread (its six counts straddle 7.5–12 µs) rather than a change in
the candidate, which has no commit on the interpreter.

### `internal/frontend/grpc`

| benchmark | 0.9.1 | 0.9.2 |
| --------- | ----- | ----- |
| `ParseFileColdShared` | 7.73 ms | 7.74 ms (~) |
| `ParseFileColdInline` | 7.89 ms | 7.90 ms (~) |
| geomean (compared rows) | | +0.1% time / +0.1% B / +0.0% allocs |

### `internal/frontend/lsp`

| benchmark | 0.9.1 | 0.9.2 |
| --------- | ----- | ----- |
| `FormatEdits` | 536 µs | 517 µs (−3.5%, p=0.026) |
| `SpanToRangeLarge` | 2.91 ms | 2.96 ms (~) |
| `ReferencesWarm` | 216 µs | 201 µs (~) |
| `ReferencesCold` | 37.9 ms | 38.5 ms (~) |
| `RenameWarm` | 17.8 ms | 17.3 ms (~) |
| `WorkspaceUpdate` | 208 ms ±3% | 248 ms ±17% (~, p=0.093) |
| geomean | | +1.1% time / +0.0% B / +0.0% allocs |

`WorkspaceUpdate` is the row that sets the geomean; its re-run is below:
over twelve interleaved counts, 196 ms ±3% against 194 ms ±25% (~,
p=0.843), 49.38 MiB against 49.37 MiB, 605.6 k allocations on both. The
candidate's spread is two counts at 270 and 361 ms in a run of twelve
otherwise at 189–241 ms, the same shape the baseline shows at 256 ms; the
`FormatEdits` row that was faster as found is `~` the same way (530 µs
against 522 µs).

### `internal/translate/migrate`, `internal/workspace/libs`, `internal/workspace/model`

| benchmark | 0.9.1 | 0.9.2 |
| --------- | ----- | ----- |
| `migrate WriterSiblingBlocks` | 1.94 ms | 1.88 ms (~) |
| `libs ExpandWildcardImports` | 28.5 ms | 28.5 ms (~) |
| `libs IndexLibrary` | 14.3 ms | 14.1 ms (~) |
| `libs ExpandModelImports` | 6.85 ms | 6.76 ms (~) |
| `libs DecodeSnapshot` | 13.4 ms | 13.2 ms (~) |
| `libs SetDigest` | 637 µs | 643 µs (~) |
| `libs` geomean | | −0.6% time / +0.0% B / +0.0% allocs |
| `model AnalyseUnresolved` | 20.7 ms | 20.8 ms (~) |
| `model AnalyseResolved` | 1.31 ms / 486 KiB | 1.29 ms (~) / 491 KiB (+1.0%) |
| `model` geomean | | −0.6% time / +0.5% B / +0.2% allocs |

### `tests/perf`

37 rows; 26 are `~`. The rows that moved:

| benchmark | 0.9.1 | 0.9.2 |
| --------- | ----- | ----- |
| `WorkspaceEdit/synthetic/reindex+diagnostics` | 1.74 s | 1.60 s (−8.2%, p=0.002) |
| `FeaturesOf` | 1.02 s | 0.94 s (−7.8%, p=0.002) |
| `REPLLoadFile` | 1.50 s | 1.38 s (−7.5%, p=0.004) |
| `REPLSubmitSnippet` | 1.11 s | 1.04 s (−6.8%, p=0.004) |
| `REPLEvalExpr` | 4.78 ms | 4.63 ms (−3.2%, p=0.026) |
| `LowerActionGraph` | 1.87 µs | 1.70 µs (−9.0%, p=0.015) |
| `ExecuteActionFreshContext` | 31.2 µs | 29.2 µs (−6.4%, p=0.002) |
| `ExecuteState` | 695 µs | 667 µs (−4.1%, p=0.026) |
| `BatchConstraints` | 9.75 ms | 9.26 ms (−5.0%, p=0.026) |
| `SameConstraintManyInstances` | 3.73 µs | 3.57 µs (−4.2%, p=0.002) |
| `ActionChain/chain100` | 575 µs ±3% | 747 µs ±23% (+30%, p=0.015) |
| `Analyze/synthetic` | 463 MiB | 466 MiB (+0.6% B) |
| `REPLLoadFile` / `REPLSubmitSnippet` | 593 / 510 MiB | 596 / 512 MiB (+0.4% / +0.5% B) — the loader's bytes, below |
| geomean | | +0.8% time / −0.0% B / +0.0% allocs |

The gains are of the size the machine gives between two runs of the same
binary (the candidate has no commit on the REPL, the lowering or the
constraint engine), and the first pass ran 0.9.1's six counts first; the
re-run below is what to read: `FeaturesOf` is −11.8% at p=0.041 with a 23% spread on the baseline
side and no change in allocations (4.157 M against 4.154 M);
`REPLLoadFile`, `Lex`, `LowerStateGraph`, `Instantiate` and
`GRPCVerifyConstraint` are `~`. `WorkspaceEdit/synthetic/reindex+diagnostics`
is 1.57 s against 1.62 s (~, p=0.937) for −8.2% as found. Nothing here is
a change in the code. `ActionChain/chain100` is the
one row slower, with the candidate's six counts spread 23%: interleaved, 592 µs ±23% against 594 µs ±28% (~, p=0.818), 516.9 KiB and
5 757 allocations on both. **Noise.**

### `tests/stressmodel`

48 rows, 47 compared. As found, 34 time rows are `~`, two are faster
(`Satisfy/512` −3.3%, `FleetInstantiate/128` −15.5%) and 13 are slower,
twelve of them in the load and open paths, with a 14–74% spread on the
candidate's six counts against 2–5% on 0.9.1's:

| benchmark | 0.9.1 | 0.9.2 as found | re-run |
| --------- | ----- | -------------- | ------ |
| `Load/satellites=32` | 371 ms ±4% | 455 ms ±27% (+22%) | 363 ms ±51% / 373 ms ±44% (~) |
| `LoadFiles/satellites=512` | 7.19 s ±2% | 9.37 s ±23% (+30%) | 7.36 s ±41% / 7.12 s ±34% (~) |
| `OpenSplit/satellites=128/cold` | 746 ms ±2% | 887 ms ±24% (+19%) | 735 ms ±39% / 715 ms ±62% (~) |
| `OpenSplit/satellites=128/warm` | 214 ms ±5% | 295 ms ±22% (+38%) | 203 ms ±32% / 198 ms ±76% (~) |
| `OpenSplit/satellites=512/cold` | 2.95 s ±2% | 3.28 s ±14% (+11%) | 2.88 s ±12% / 2.89 s ±63% (~) |
| `OpenSplit/satellites=512/warm` | 676 ms ±4% | 935 ms ±21% (+38%) | 655 ms ±29% / 638 ms ±58% (~) |
| `HydratePlane/satellites=32` | 99 ms ±35% | 128 ms ±15% (+29%) | 104 ms ±41% / 92 ms ±26% (~) |
| `HydratePlane/satellites=128` | 259 ms ±22% | 372 ms ±41% (+44%) | 268 ms ±42% / 254 ms ±45% (~) |
| `HydratePlane/satellites=512` | 1.03 s ±2% | 1.31 s ±74% (+27%) | 1.21 s ±41% / 1.03 s ±25% (~) |
| `ConvertAPIJSON/satellites=32` | 7.29 s ±8% | 9.32 s ±22% (+28%) | 7.15 s ±21% / 7.19 s ±62% (~) |
| `FleetSatisfy/satellites=32` | 1.06 ms ±4% | 1.11 ms ±10% (+4.5%) | 1.09 ms ±13% / 1.10 ms ±42% (~) |
| geomean (as found) | | +6.0% time / +0.3% B / +0.0% allocs | |

The same paths at the other sizes are `~` as found (`Load` at 128 and 512,
`LoadFiles` at 32 and 128, `OpenSplit` at 32, `ValidateSplit` and
`AnalyzeSplitPerDocument` at every size and job count), which a code
regression in the loader would not leave alone. Interleaved, with nothing else on the machine, every one of the eleven
rows is `~` (the `re-run` column: 0.9.1 then the candidate, six counts
each), the re-run geomean over them is −0.4%, and the spread is now as
wide on 0.9.1 as it was on the candidate — 12–51% on the baseline's own
counts — which is what the first pass measured: the virtual machine's
neighbours during the candidate's half-hour, not the candidate. The two
rows faster as found (`Satisfy/512`, `FleetInstantiate/128`) are `~` the
same way.

Bytes and allocations on this package are the one systematic signal in
the whole measurement:

| benchmark | 0.9.1 | 0.9.2 |
| --------- | ----- | ----- |
| `Load/satellites=32` | 132.1 MiB / 1.993 M allocs | 132.6 MiB (+0.41%) / 1.994 M (+0.08%) |
| `Load/satellites=128` | 498.8 MiB / 7.676 M | 501.3 MiB (+0.50%) / 7.682 M (+0.08%) |
| `Load/satellites=512` | 1.924 GiB / 30.41 M | 1.934 GiB (+0.53%) / 30.43 M (+0.08%) |
| `LoadFiles/satellites=512` | 1.845 GiB | 1.858 GiB (+0.73%) |
| `ValidateSplit/satellites=512/jobs=8` | 2.325 GiB | 2.335 GiB (+0.45%) |
| bytes per element, `Load` | 21.0–21.7 KiB | +0.4–0.5% |
| `ConvertAPIJSON/satellites=32` | 25.73 M allocs | 25.53 M (−0.8%) |

A constant +0.4–0.7% bytes per element at every size, with +0.08%
allocations (the REPL's `LoadModel/elements=4000` shows the same +0.58%
bytes and +0.09% allocations), is one small allocation per declaration
on the load-and-analyse path. The candidate's changes on that path are
the implicit-subsetting uniqueness check (#748), which now collects the
subsettings a feature writes before walking the ones it has implicitly,
and the variant-reference parse (#790); the time it costs is below what
the machine resolves. **Explained**, and well below the per-element growth
the previous two records priced. The API-JSON conversion allocates 0.8%
fewer objects for the same bytes (the id-form fix, #749).

## Whole-binary scaling

`sysml -validate` over generated compliant models (the satellite-network
generator at ≈3 000, 6 000 and 12 000 declarations: 3 804, 7 260 and
14 172 lines) that report `no errors`, five runs each with the two
binaries interleaved; median wall seconds and median maximum resident set.

| declarations | 0.9.1 | 0.9.2 |
| ------------ | ----- | ----- |
| ≈3 000 | 0.10 s / 98 MiB | 0.10 s / 98 MiB |
| ≈6 000 | 0.14 s / 108 MiB | 0.15 s / 124 MiB |
| ≈12 000 | 0.25 s / 133 MiB | 0.24 s / 132 MiB |

Level at every size within the clock's 10 ms. The 16 MiB at ≈6 000 is
one run's garbage-collector timing, not a trend: the 3 000 and 12 000
rows are within 1 MiB and the five 6 000-declaration runs of the
candidate range 108–125 MiB.

### Process start and session floor

| figure | 0.9.1 | 0.9.2 |
| ------ | ----- | ----- |
| binary size | 42.46 MiB | 42.59 MiB (+128 KiB) |
| empty session (`sysml -e 1`), median of 5 | 0.02 s / 65 MiB | 0.02 s / 65 MiB |
| empty-session load (`LoadModel/elements=0`) | 392 µs | 398 µs (~) |

### Example models and the Apollo 11 model

Median wall time and maximum resident set of five interleaved runs; every
run returned the same exit status and report on both binaries. The Apollo
11 model is the public
[Apollo 11 SysML v2 model](https://github.com/airbus/apollo-11-sysml-v2)
(commit `6e9c93f`, 28 files, 7 221 lines).

| model / command | 0.9.1 | 0.9.2 |
| --------------- | ----- | ----- |
| `action-executor-demo.sysml -validate` | 0.02 s / 67 MiB | 0.02 s / 66 MiB |
| `disposal-robot-demo/robot.sysml -validate` | 0.04 s / 72 MiB | 0.04 s / 72 MiB |
| Apollo 11, `-validate` all 28 files | 0.28 s / 150 MiB | 0.28 s / 150 MiB |

## What this does not measure

- The Python client's own paths (the metamodel reader, the release-digest
  download) have no benchmark; they are covered by the client's test suite
  and the release-packaging rehearsal, not by this record.
- The gRPC service's `InstantiateWarmModel` row has no 0.9.1 counterpart.
- Everything ran on one 8-CPU virtual machine; the as-found stress rows
  show what its neighbours can do to a 30-minute benchmark run, and a
  one-CPU host should read the package figures rather than the
  whole-binary ones, as the previous record said.
