# Performance: release 0.10.1 against release 0.10.0

The release-gate measurement of 0.10.1 — the head of `hotfix/0.10.1` at
`5e9d673df`, which already carries the first of the five performance fixes
below, with the other four applied on top —
measured against the `v0.10.0` tag (`08ef39784`, 2026-10-12). It follows
the method of the [0.9.2 against 0.9.1](archive/performance-release-0.9.2-vs-0.9.1.md)
record and of the 0.10.0 against 0.9.2 record that preceded it (written
on `develop`, where the 0.10.0 release was measured; it reaches this
branch's line with the release's back-merge): every
benchmark on both revisions, `benchstat` over six runs, the rows the
machine left in doubt re-run interleaved, and whole-binary timings on
generated and real models. See [performance](../internals/performance.md)
for what the figures mean and where the remaining cost is.

0.10.1 is a patch release: about fifteen fixes cherry-picked from
`develop` onto `main` — the frame feature-identity rule in the executor
(a name read in a body denotes the feature it names, not a same-named
binding an enclosing frame happens to hold), redefinitions that take
their short name from the feature they redefine, the migrator's and
renderer's fixes, the mechanical SonarCloud clean-up — and no feature.
**The expectation going in was parity**, and on the paths a model goes
through that is what the measurement finds: parsing, indexing, analysis,
the workspace, the language server, the gRPC service, the REPL, the
migrator and the whole binary are unchanged to the allocation or within
the machine's noise. What moved was the executor, and this record says
by how much and why.

The measurement was taken twice. The first pass, on the hotfix head as
it stood before any fix (`50933ba4e`), found five regressions that were
not the inherent cost of a correctness fix; each was fixed in its own
pull request against `hotfix/0.10.1` and the candidate was then measured
again with all five applied. The tables show 0.10.0, the unfixed head
where a row moved there, and the final candidate; the fixes, by the
change in the history:

- name resolution takes the specialization chain of a scope's owner from
  the semantic model's memoized closure instead of re-walking the
  generalization graph on every lookup that consults inherited imports
  (`internal/semantic/resolve`, #1155);
- the execution runtime resolves an unqualified name read from a scope
  once per model instead of asking the resolver on every read
  (`internal/exec/runtime`, #1167);
- a name read no longer copies every frame it probes: the frame probes
  take the frame by pointer (`internal/exec/runtime`, #1168);
- the distinguishability check builds the names a membership binds as a
  fixed pair instead of a slice that escaped once per member
  (`internal/semantic/resolve`, #1172);
- an action execution's root frame sizes its held-feature index for every
  feature it takes instead of growing it entry by entry
  (`internal/exec/runtime`, #1173).

Every row that regresses is confirmed by the interleaved re-run and
attributed below; what remains after the fixes is named, with its cause,
in the summary.

## Summary

**Nothing here blocks the release, and the patch is at parity everywhere
but the executor.** Confirmed by the interleaved re-runs:

- **Parsing, indexing, analysis, the workspace, the migrator**
  (`internal/workspace/libs`, `internal/workspace/model`,
  `internal/translate/migrate`, the `Lex`/`Parse`/`Index*`/`Analyze`/
  `WorkspaceEdit` rows of `tests/perf`, the `Load`/`LoadFiles`/
  `ValidateSplit`/`OpenSplit`/`HydratePlane`/`ConvertAPIJSON` rows of the
  stress constellation): identical to the allocation, within 0.4% of
  it, or — the analysis rows — 1–6% fewer allocations than 0.10.0,
  because the resolver walk #1155 removed was one 0.10.0 paid too; no
  significant time difference. The one analysis row that regressed,
  `WorkspaceEditSmallDocBesideLarge` (+7% allocations on the unfixed
  head), is #1172's and is back at 0.10.0's count.
- **The language server, the gRPC service, the REPL**: no row
  significant on time in the interleaved run; `ReferencesCold` carries
  +1.4% allocations (the derived-short-name journal of the redefinition
  short-name fix), everything else within 0.5%.
- **The executor** is where the patch is not free. The frame
  feature-identity rule (#1058) makes every name read in a body resolve
  its name to a feature before a frame answers it. On the unfixed head
  that cost +23–37% time on every loop row of `internal/exec/runtime`
  (`IntegerArithmeticLoop` +29%, `AssignmentLoopStep` +37%), +17%
  allocations on `ExecuteState`/`StateLoop`, +62% on `BatchConstraints`
  and +6% on the stress constellation's `Satisfy`/`FleetSatisfy`. Four
  parts of that were avoidable and are fixed (#1155, #1167, #1168, and
  #1173 for the root frame); what remains is the rule's own two-phase
  read, +7–17% time on the loop rows of `internal/exec/runtime`
  (`AssignmentLoopStep` +10.6%, `IntegerArithmeticLoop` +14.9%,
  `RealDecimalLoop` +15.9%, `RationalDecimalLoop` +7.3%) with
  allocations within one or two per operation of 0.10.0 — a memoized
  lookup and a frame probe per name read, and nothing on the rows that
  read no names.
- **Whole binary**: `sysml -validate` on the generated 3 000–12 000
  declaration models, the two example models and the 28 Apollo 11 files
  is at parity on the median (0.10–0.11 / 0.16 / 0.27–0.29 s; Apollo
  0.31–0.33 s) and on the resident set to the MiB; the session floor is
  69 MiB and 0.02 s on both.

## Method

Machine: `INTEL(R) XEON(R) PLATINUM 8559C`, 8 CPUs, 31 GiB, Go
`go1.25.11 linux/amd64`, `GOMAXPROCS=8`; one machine for every figure
here (no rebuild mid-run). 0.10.0 is a worktree at the `v0.10.0` tag,
built and benchmarked with its own tree; the unfixed head `50933ba4e`
and the final candidate (the `hotfix/0.10.1` head `5e9d673df`, which
includes #1155, with the branches of #1167, #1168, #1172 and #1173 merged
at `3823af21e`, `bc660758b`, `dfd6c5a6a` and `ca39121f5`) are worktrees
treated the same way. The record was measured and written before those
four pull requests merged, so that it measures the release as it ships;
it is to be read with them in `hotfix/0.10.1`, and the unfixed-head
column shows what each of them changed.
Package benchmarks are
`go test ./<pkg> -run '^$' -bench . -benchmem -count 6` on each
revision, compared with `benchstat` (rows are significant at p < 0.05;
`~` is no significant difference). The first pass ran each package's six
counts on the candidate and then on 0.10.0; rows it left in doubt — a
significant difference, or a spread above 20% on either side — were
re-run with the revisions interleaved, `-count 1` six times each,
nothing else on the machine (`internal/exec/runtime`,
`internal/frontend/lsp`, `tests/perf` and `tests/stressmodel` whole).
The final candidate was measured the same way, six counts of every
package and the executor and `tests/perf` rows interleaved with 0.10.0.
A benchmark that fails on one revision (`b.Fatal`) prints `--- FAIL`
and no result line while the package's other benchmarks run on;
`benchstat` then has no row for it, and the failure is reported in that
package's section. Nothing was excluded from `-bench .` except the one
row noted under comparability. Whole-binary figures are
`/usr/bin/time -f '%e %M'` over nine interleaved runs of each binary,
all built with `make build` (`-s -w`, `-trimpath`); medians are
reported. The four corpora the benchmarks and gates read were fetched
into every tree with the `scripts/download-*.sh` scripts. Causes were
read from `git log v0.10.0..hotfix/0.10.1 -- <path>` and, where the
history alone did not name one, from `-memprofile` of the row on both
revisions (`go tool pprof -diff_base`).

### What is and is not comparable

- **Ten packages define benchmarks on both revisions**; nine produce
  rows. `internal/syntax/parser` defines no benchmark that runs without
  `OPENSYSML_BENCH_MODEL`; neither revision produced a row and the
  package is omitted, as before.
- **Rows neither revision can run.** In `tests/stressmodel` the six
  `AnalyzeSplitPerDocument` rows fail on both revisions (the benchmark
  asserts a clean analysis and the per-document registry reports the
  generated model's decimal literals as rounded by their `Real`
  features, as on 0.10.0 in the previous record) and the four
  `PlaneResidency` rows fail in the benchmark's own setup
  (`ErrRecordStale`); no figure on either side.
- **`ConvertAPIJSON/satellites=512`** allocates 73 GiB per operation.
  The interleaved stress run produced one complete count on 0.10.0 and
  three on the unfixed head before the kernel killed the benchmark for
  memory; those agree (129.6 against 129.2 s, 396.3M against 396.4M
  allocations) and the row is listed, not compared. The final
  candidate's stress pass skipped it (`-skip`) so that its remaining
  rows would run.
- **New rows.** `internal/exec/runtime` gains `IntegerArithmeticNestedLoop`
  (#1168) and `NewRootFrame` (#1173) on the final candidate only; they
  are listed with their figures, not compared.
- **Diagnostics**: on every model measured both binaries return the same
  exit status and the same report — `no errors` on the generated models,
  the two example models and all 28 Apollo 11 files.
- **The machine.** Every figure is from one host, and no time is compared
  with an earlier record; allocations and bytes are exact and comparable
  across records.

## Package benchmarks

Time per operation, 0.10.0 then the final candidate, from the interleaved
re-run wherever a row was re-run (the first pass agrees in sign and
roughly in size on every row named here); bytes and allocations where
they moved, with the unfixed head's figure between them where a fix
moved the row back. Allocation counts are deterministic to the unit on
every revision and are the reading that separates a change in the code
from a change in the machine; a time difference with identical
allocations is read against the row's spread.

### `internal/exec/runtime`

Time per operation from the final interleaved run (six rounds, 0.10.0 and
the final candidate alternating); the unfixed head's time difference is
from its own interleaved run against 0.10.0. Allocations: 0.10.0, the
unfixed head `50933ba4e`, the final candidate.

| benchmark | 0.10.0 | 0.10.1 | Δ time | Δ time, unfixed head | allocs/op |
| --------- | ------ | ------ | ------ | -------------------- | --------- |
| `MaterializePlainAttributes` | 54.1 µs | 55.3 µs | ~ | ~ | 618 → 618 → 618 |
| `SetFeatureValueNoDependents` | 428 ns | 432 ns | ~ | ~ | 9 → 9 → 9 |
| `DerivedReadWriteRead` | 6.25 µs | 6.38 µs | ~ | ~ | 65 → 65 → 65 |
| `AssignmentLoopStep` | 1.44 ms | 1.59 ms | +10.6% | +37.1% | 7 327 → 9 530 → 7 328 |
| `IntegerArithmeticLoop` | 3.25 ms | 3.73 ms | +14.9% | +28.5% | 9 070 → 9 072 → 9 071 |
| `IntegerArithmeticLoopBeyondInt64` | 3.31 ms | 3.86 ms | +16.9% | +29.6% | 13 070 → 13 072 → 13 072 |
| `IntegerCollectionSum` | 839 µs | 848 µs | ~ | ~ | 44 → 44 → 44 |
| `BehaviorOrdersLibraryModel` | 330 µs | 335 µs | +1.7% | ~ | 1 922 → 1 927 → 1 927 |
| `RationalDecimalLoop` | 3.71 ms | 3.99 ms | +7.3% | +23.0% | 9 069 → 9 071 → 9 070 |
| `RationalHarmonicLoop` | 848 µs | 945 µs | ~ (p = 0.065) | +25.7% | 4 650 → 4 652 → 4 651 |
| `RealDecimalLoop` | 3.47 ms | 4.03 ms | +15.9% | +25.3% | 9 069 → 9 071 → 9 070 |

Bytes per operation are within 0.1% on every row but `AssignmentLoopStep`
(817 → 827 KiB, +1.2%: the name memo's entries) and
`BehaviorOrdersLibraryModel` (+0.6%). Two rows exist only on the final
candidate and are listed, not compared: `IntegerArithmeticNestedLoop`
4.94 ms / 915 KiB / 26 080 allocations (#1168) and `NewRootFrame` 9.79 µs /
12.5 KiB / 36 allocations (#1173; 43 allocations and 16.2 KiB on the
unfixed head).

The loop rows are the per-step cost of the executor in isolation, and the
patch's one semantic change to the executor lands on every step. The
frame feature-identity rule (#1058) makes a name read in a body denote the
feature the name resolves to, not a same-named binding an enclosing frame
happens to hold: before a frame can answer a read, the read resolves its
single-part name through the resolver and asks each frame whether it
holds that feature. On the unfixed head that was a full resolver walk per
read — and, for every read that reaches a scope's inherited imports, a
breadth-first re-walk of the owner's generalization graph allocating a
map and a slice (`AssignmentLoopStep` +2 203 allocations, +30%) — plus a
copy of every frame probed, because the probes took the frame by value.
Three fixes take the avoidable part out: the resolver reads the
specialization chain from the semantic model's memoized closure (#1155:
back to 7 529 allocations), the runtime `Model` memoizes the resolver's
answer per (scope, name, exclusion) so a run resolves each name once
(#1167: 7 328, one above 0.10.0 — the memo entry itself), and the probes
take the frame by pointer (#1168). What remains on the final candidate is
the rule's own two-phase read — a memo lookup and a frame probe per name
read, with no allocation — at +7–17% of a loop step
(`AssignmentLoopStep` +10.6%, `IntegerArithmeticLoop` +14.9%,
`RealDecimalLoop` +15.9%, `RationalDecimalLoop` +7.3%, geomean +7.6%
over the package). It is the cost of a correctness fix and is not treated
as a regression to remove: the alternative, answering a read from the
innermost frame that holds the name, is the behaviour the rule corrects.
The materialization, derived-value and collection rows, which read no
names in a loop body, are at parity to the allocation and within their
spreads on time.

### `internal/frontend/repl`, `internal/frontend/grpc`

No row of either package is significant on time in the first pass's six
counts beyond its own spread (the REPL's geomean is −7%, the gRPC
service's −9%, both inside the ±10–30% spreads of their small rows), and
allocations are identical on every row but these: `REPLEvalExpr`
104 → 106, `ExecuteAction` 244 → 252 → 245 (final), `GRPCEvaluate`
116 → 117, `ConnectEvaluateHTTP` 206 → 207 — the one or two resolutions
the frame feature-identity rule adds to an expression evaluation, and
for `ExecuteAction` the root frame's held-feature index (#1173). The
gRPC `Instantiate` rows hold the same bytes and allocations to the unit.

### `internal/frontend/lsp`

Time per operation from the interleaved run of 0.10.0 against the unfixed
head (six rounds; the package did not move between the unfixed head and
the final candidate on any row the fixes touch, and the final candidate's
six counts of it are first-pass figures); allocations: 0.10.0, the
unfixed head, the final candidate.

| benchmark | 0.10.0 | `50933ba4e` | Δ time | allocs/op |
| --------- | ------ | ----------- | ------ | --------- |
| `FormatEdits` | 968 µs | 938 µs | ~ | 85 → 85 → 85 |
| `SpanToRangeLarge` | 3.03 ms | 3.02 ms | ~ | 0 → 0 → 0 |
| `ReferencesWarm` | 228 µs | 235 µs | ~ | 1 017 → 1 017 → 1 017 |
| `ReferencesCold` | 43.5 ms | 51.5 ms | ~ (±81% / ±14%) | 44 510 → 45 130 → 44 640 |
| `RenameWarm` | 20.4 ms | 22.9 ms | ~ (±64% / ±15%) | 12.8k → 15.4k → 13.5k (±64% / ±11% / ±6%) |
| `WorkspaceUpdate` | 284 ms | 291 ms | ~ | 710.6k → 710.6k → 710.3k |

The first pass showed `ReferencesCold` and `RenameWarm` +15% on time;
interleaved, no row of the package is significant on time (the geomean
is +5% inside spreads of ±6–20%). Allocations moved on one row:
`ReferencesCold` 44.51k → 45.13k (+1.4%) on the unfixed head and
44.65k (+0.3%) on the final candidate — the derived-short-name lookups of
the redefinition short-name fix, which `checkDistinguishability` now
performs for every member of a scope with imports (the part of it that
allocated per member is #1172). Everything else is identical to the
allocation.

### `internal/translate/migrate`, `internal/workspace/libs`, `internal/workspace/model`

At parity. `internal/workspace/libs` (20 rows: the bundled standard
library's decode, index and lookups) is identical to the byte and the
allocation on every row, geomean +0.02% on time. `internal/workspace/model`
(10 rows) is within +0.2% allocations (the derived-short-name journal:
+0.23% geomean) and +1.3% time, inside its spreads.
`internal/translate/migrate` (8 rows) is identical to the allocation,
no significant time difference.

### `tests/perf`

Time per operation from the final interleaved run (six rounds, 0.10.0 and
the final candidate alternating); allocations: 0.10.0, the unfixed head,
the final candidate. Rows not listed are identical to the allocation and
not significant on time on both passes.

| benchmark | 0.10.0 | 0.10.1 | Δ time | allocs/op |
| --------- | ------ | ------ | ------ | --------- |
| `Parse/synthetic` | 85.5 ms | 100 ms | +17.0% | 293.2k → 293.2k → 293.2k |
| `Parse/vehicle` | 2.25 ms | 2.50 ms | +11.4% | 7 559 → 7 560 → 7 560 |
| `ImportValues/*` (8 rows) | — | — | ~ | +0.30–0.41% → +0.11–0.27% |
| `Analyze/synthetic` | 1.54 s | 1.52 s | ~ | 4.613M → 4.622M → 4.382M (−5.0%) |
| `Analyze/vehicle` | 217 ms | 212 ms | ~ | 869.7k → 868.9k → 856.6k (−1.5%) |
| `WorkspaceEdit/synthetic/reindex+diagnostics` | — | — | ~ | 5.058M → 5.068M → 4.828M (−4.6%) |
| `WorkspaceEdit/vehicle/reindex+diagnostics` | — | — | ~ | 867.7k → 869.2k → 857.7k (−1.2%) |
| `WorkspaceEditSmallDocBesideLarge` | — | — | ~ | 114.5k → 122.6k → 114.6k |
| `FeaturesOf` | — | — | ~ | 4.156M → 4.158M → 4.130M (−0.6%) |
| `REPLLoadFile` | — | — | ~ | 5.564M → 5.573M → 5.333M (−4.1%) |
| `REPLSubmitSnippet` | — | — | ~ | 5.017M → 5.036M → 4.695M (−6.4%) |
| `REPLEvalExpr` | — | — | ~ | 104 → 106 → 106 |
| `ExecuteAction` | — | — | ~ | 244 → 252 → 245 |
| `ExecuteActionFreshContext` | — | — | ~ | 337 → 345 → 340 (35.7 → 42.6 → 39.8 KiB) |
| `ExecuteState` | 1.20 ms | — | ~ (unfixed head +32.6%) | 12.12k → 14.22k → 12.13k |
| `BatchConstraints` | 13.6 ms | — | ~ (unfixed head +76%, ±53% / ±91%) | 67.5k → 109.7k → 68.0k |
| `SameConstraintManyInstances` | 5.36 µs | — | ~ (unfixed head +99%) | 35 → 65 → 35 |
| `BatchSatisfy` | — | — | ~ | 4.367M → 4.382M → 4.301M (−1.5%) |
| `GRPCEvaluate` | — | — | ~ | 116 → 117 → 117 |
| `GRPCVerifyConstraint` | 953 µs | 1.02 ms | +6.9% (±7% / ±25%) | 9 783 → 9 816 → 9 786 |
| `ConnectEvaluateHTTP` | — | — | ~ | 206 → 207 → 207 |
| `ActionLoop/for10` | 36.2 µs | 40.3 µs | +11.3% | 263 → 271 → 264 (28.9 → 35.7 → 32.4 KiB) |
| `ActionLoop/for100` | 151 µs | 159 µs | ~ | 626 → 634 → 627 |
| `ActionLoop/for1000` | — | — | ~ | 4 230 → 4 239 → 4 231 |
| `ActionChain/chain10` | — | — | ~ | 805 → 880 → 803 |
| `ActionChain/chain100` | 789 µs | 850 µs | +7.8% | 6 637 → 7 340 → 6 632 |
| `ActionChain/chain1000` | 15.9 ms | 16.4 ms | +3.1% | 64.9k → 72.1k → 64.9k |
| `StateLoop/count50` | 1.23 ms | — | ~ (unfixed head +25.9%) | 12.11k → 14.19k → 12.11k |
| `StateLoop/count500` | 9.23 ms | — | ~ (unfixed head +50.1%) | 117.9k → 138.5k → 117.9k |
| `StateLoop/count5000` | 95.8 ms | — | ~ (unfixed head +39.6%) | 1.176M → 1.382M → 1.176M (59.7 → 64.2 → 60.4 MiB) |
| `Instantiate` | 543 ms | 643 ms | +18.4% (±33% / ±101%) | identical |
| `FQNOf` | 1.76 ms | 1.66 ms | −5.9% (±32% / ±5%) | identical |

Three things are in this table.

The **executor rows** are the frame feature-identity rule again, at the
scale of an action or state machine rather than a loop step. On the
unfixed head `ExecuteState` and the three `StateLoop` rows carried +17%
allocations and +26–50% time, `BatchConstraints` +62% allocations
(`SameConstraintManyInstances` 35 → 65) and `ActionChain` +9–11%: the
resolver's generalization re-walk on every name read (#1155) and the
walk itself (#1167). On the final candidate every one of them is back to
0.10.0's allocation count or within 0.8% of it, and none is significant
on time. What the rule costs here is `ExecuteAction` +1 allocation and
`ExecuteActionFreshContext` +3 allocations / +4 KiB per fresh execution —
the root frame's held-feature index, 61 entries for a plain action
definition, which #1173 sizes up front (it was +8 allocations and +7 KiB
grown entry by entry) — and `ActionLoop/for10` +11% on a 36 µs row whose
fixed cost is that index. `REPLEvalExpr` 104 → 106, `GRPCEvaluate`
116 → 117 and `ConnectEvaluateHTTP` 206 → 207 are the one or two
resolutions the rule adds to an expression evaluation.

The **analysis rows** improved. `Analyze`, `WorkspaceEdit`, `FeaturesOf`,
`REPLLoadFile`, `REPLSubmitSnippet` and `BatchSatisfy` are 0.6–6.4%
fewer allocations than 0.10.0: #1155 replaced a breadth-first walk of the
generalization graph that 0.10.0's resolver also performed, on every
lookup that consulted inherited imports, with the semantic model's
memoized closure; analysis of the synthetic model pays it on every
inherited lookup and saves 231 000 allocations. `WorkspaceEditSmallDocBesideLarge`
(+7% on the unfixed head) is #1172's: the redefinition short-name fix
made `memberNames` a resolver method and its two-element slice escaped
once per member on every distinguishability check; as a fixed pair it is
back to 0.10.0's count to within 0.1%. The `ImportValues` rows carry
+0.1–0.3% allocations — the derived-short-name journal, one entry per
redefinition — and are not significant on time.

The **`Parse` rows** moved on time in the final interleaved run
(`Parse/synthetic` +17%, `Parse/vehicle` +11%, p = 0.002) with bytes and
allocations identical to the unit, after +72% in the first pass (where
the candidate's counts followed 0.10.0's) and no significant difference
in the first interleaved run (+7% at ±12% / ±7%). The patch does touch the
lexer and parser — multi-line string literals, trailing commas in
sequences, multiplicity after a pre-name specialization, KerML-only
keywords as SysML names — none of which adds work to a token that does
not use them. An eight-round interleaved run of the two rows alone,
nothing else running, finds no difference (`Parse/synthetic` 121 against
132 ms, p = 0.57; `Parse/vehicle` 2.96 against 3.45 ms, p = 0.16; spreads
±20–30% on both sides). The row is read as the machine: three runs, three
answers, and allocations that did not move.

`Instantiate` (+18% at ±33% / ±101%) and `FQNOf` (−6% at ±32%) are the
machine: their allocations are identical and their spreads exceed the
difference. `GRPCVerifyConstraint` +6.9% at ±25% is the same reading.

## Stress constellation (`tests/stressmodel`)

The satellite-network generator at 32, 128 and 512 satellites (6 227,
24 167 and 95 927 elements), loaded whole and split six ways, analysed,
edited beside and within, hydrated, satisfied and converted.

Allocations per operation: 0.10.0, the unfixed head, the final candidate.
Time: no row of the package is significant between 0.10.0 and the unfixed
head in the interleaved six rounds except `LoadFiles/satellites=32`
(+16% at ±26% / ±12%, allocations identical — the machine), and no row
of `Satisfy`, `FleetSatisfy` or `EditBeside` is significant between
0.10.0 and the final candidate in the three interleaved rounds; the final
candidate's other rows are first-pass figures.

| benchmark | satellites | allocs/op | Δ allocs, final |
| --------- | ---------- | --------- | --------------- |
| `Load` | 32 / 128 / 512 | 2.316M → 2.319M → 2.228M; 8.837M → 8.852M → 8.491M; 34.92M → 34.99M → 33.55M | −3.8% / −3.9% / −3.9% |
| `LoadFiles` | 32 / 128 / 512 | 2.325M → 2.326M → 2.234M; 8.244M → 8.247M → 7.885M; 31.91M → 31.92M → 30.48M | −3.9% / −4.4% / −4.5% |
| `ValidateSplit/jobs=1` | 32 / 128 / 512 | 2.881M → 2.880M → 2.780M; 10.07M → 10.07M → 9.683M; 38.80M → 38.81M → 37.29M | −3.5% / −3.8% / −3.9% |
| `ValidateSplit/jobs=8` | 512 | 38.82M → 38.83M → 37.31M | −3.9% |
| `EditImported` | 32 / 128 / 512 | 2.360M → 2.360M → 2.268M; 8.090M → 8.089M → 7.727M; 31.00M → 31.00M → 29.56M | −3.9% / −4.5% / −4.7% |
| `OpenSplit/cold` | 32 | 2.762M → 2.756M → 2.658M | −3.8% |
| `OpenSplit/warm` | 32 / 128 / 512 | 694.8k → 696.3k → 696.3k; 1.717M → 1.719M → 1.719M; 5.804M → 5.806M → 5.806M | +0.2% / +0.1% / +0.02% |
| `HydratePlane` | 32 / 128 / 512 | 255.0k → 253.9k → 253.0k; 652.9k → 652.6k → 648.8k; 2.243M → 2.246M → 2.231M | −0.8% / −0.6% / −0.5% |
| `EditBeside` | 32 / 128 / 512 | 4 536 → 4 707 → 4 541; 11.97k → 12.64k → 11.98k; 41.68k → 44.31k → 41.69k | +0.1% / +0.04% / +0.01% |
| `FleetInstantiate` | 32 / 128 / 512 | 14.35k → 14.39k → 14.39k; 54.60k → 54.74k → 54.74k; 215.6k → 216.1k → 216.1k | +0.28% / +0.25% / +0.24% |
| `Satisfy` | 32 / 128 / 512 | 33.42k → 35.34k → 33.42k; 132.9k → 140.6k → 132.9k; 530.9k → 561.6k → 530.9k | 0 |
| `FleetSatisfy` | 32 / 128 / 512 | 8 852 → 9 332 → 8 852; 13.65k → 14.37k → 13.65k; 41.56k → 43.72k → 41.54k | 0 |
| `ConvertAPIJSON` | 32 / 128 | 25.33M → 25.33M → 25.07M; 99.31M → 99.33M → 99.04M | −1.0% / −0.3% |

The constellation says the same as `tests/perf`, at scale. **Loading,
splitting, validating, editing within and hydrating** a 96 000-element
model allocate 3.5–4.7% less on the final candidate than on 0.10.0 — the
resolver's generalization re-walk that #1155 removed was paid by 0.10.0
too, on every inherited lookup of every analysis, and a `Load` of the
512-satellite model saves 1.4 million allocations. Live bytes per element
(2.53 KiB) and bytes per element (24.7 KiB, +0.2% on the unfixed head for
the derived-short-name journal) are unchanged. **`Satisfy` and
`FleetSatisfy`** carried +5.2–5.8% allocations on the unfixed head: the
constraint bodies read names, and each read re-walked the generalization
graph (#1155 brought it to +0.6%) and asked the resolver afresh (#1167
brings it to 0.10.0's count exactly, on all six rows). **`EditBeside`**
carried +3.8–6.3%: the `memberNames` slice of #1172, escaping once per
member of the edited document's imports on every distinguishability check;
fixed, the row is +5 allocations per operation at every size — the
derived-short-name journal entries of the small document's redefinitions.
**`FleetInstantiate`** +0.24–0.28% and the warm `OpenSplit` +0.02–0.2%
are the same journal, one entry per redefinition, and did not move with
the fixes. `ConvertAPIJSON` at 32 and 128 satellites is at parity (−0.3
to −1%), unchanged in bytes (18.3 GiB at 128).

`AnalyzeSplitPerDocument` (six rows) and `PlaneResidency` (four rows)
fail on both revisions as described under comparability;
`ConvertAPIJSON/satellites=512` is listed from the counts that survived
(129.6 s / 73 GiB / 396.3M allocations on 0.10.0; 129.2 s / 396.4M on
the unfixed head) and was skipped on the final candidate.

## Whole-binary scaling

`sysml -validate` over generated compliant models (the satellite-network
generator at ≈3 000, 6 000 and 12 000 declarations) that report
`no errors`, nine runs each with the two binaries interleaved; median
wall seconds of the eight warm runs and median maximum resident set,
every run exiting 0. The unfixed head was measured the same way against
0.10.0 in an earlier run on the same machine and agreed with it to the
timer tick and the MiB (0.12 against 0.12 s, 0.20 against 0.18 s, 0.34
against 0.30 s; 101/119/157 MiB on both).

| declarations | 0.10.0 | 0.10.1 (final candidate) |
| ------------ | ------ | ------------------ |
| ≈3 000 | 0.10 s / 101 MiB | 0.11 s / 102 MiB |
| ≈6 000 | 0.16 s / 114 MiB | 0.16 s / 115 MiB |
| ≈12 000 | 0.27 s / 157 MiB | 0.29 s / 158 MiB |

Linear in the declarations on both binaries, as before, and at parity:
the differences are one or two timer ticks inside the 0.02–0.05 s spread
of the warm runs, and the resident set agrees to within 1 MiB. The first
run of each model on the freshly built 0.10.1 binary was its slowest
(0.45, 0.83 and 1.58 s: the cold index cache) and is excluded from the
medians by the odd count; 0.10.0's binary had been run before and had no
cold round.

### Process start and session floor

| measure | 0.10.0 | 0.10.1 (final candidate) |
| ------- | ------ | ------------------ |
| `sysml -e 1` wall time | 0.02 s | 0.02 s |
| `sysml -e 1` max RSS (session floor) | 69 MiB | 69 MiB |
| `bin/sysml` size | 47.55 MiB | 47.61 MiB |

The floor is the standard library's bundled index decoded at start and
is unchanged; the binary is 56 KiB larger (the patch's code).

### Example models and the Apollo 11 model

| model | 0.10.0 | 0.10.1 (final candidate) |
| ----- | ------ | ------------------ |
| `examples/action-executor-demo.sysml` | 0.02 s / 70 MiB | 0.02 s / 71 MiB |
| `examples/disposal-robot-demo/robot.sysml` | 0.04 s / 77 MiB | 0.04 s / 78 MiB |
| Apollo 11 (28 files, `airbus/apollo-11-sysml-v2` at `6e9c93f`) | 0.31 s / 162 MiB | 0.33 s / 163 MiB |

The two small examples sit on the session floor and move with it; the
Apollo 11 model — the one real model in the set — validates in the same
time (0.33 against 0.31 s, inside the 0.31–0.36 s spread of 0.10.0's own
runs) holding 1 MiB more. Every file validates with the same report on
both binaries.

## Re-runs

Four interleaved runs were made, each with nothing else on the machine.
The first re-ran `internal/exec/runtime`, `internal/frontend/lsp`,
`tests/perf` and `tests/stressmodel` whole, six rounds of 0.10.0 and the
unfixed head alternating (and three rounds of the #1155 branch on the
rows it addressed); the second re-ran `internal/exec/runtime` and
`tests/perf` whole, six rounds of 0.10.0 and the final candidate; the
third re-ran the three stress rows that had moved, three rounds of each;
the fourth re-ran the two `Parse` rows, eight rounds. The sixth 0.10.0
count of the second run's `tests/perf` was taken after a reboot of the
same machine (its first attempt was lost to a VM restart); allocations
agree with the five before it to the unit.

What the first pass flagged that the interleaved run cleared: the
language server's `ReferencesCold` and `RenameWarm` (+15% time in the
first pass; not significant interleaved, spreads ±14–81%);
`Analyze/synthetic` and `Analyze/vehicle` (+12% / +22% on the unfixed
head's six counts; not significant against the final candidate, and
−1.5 to −5% allocations); `Parse/synthetic` (+72% in the first pass,
where the candidate's six counts ran after 0.10.0's; see the `tests/perf`
section); `Instantiate`, `FQNOf`, `GRPCVerifyConstraint`,
`LoadFiles/satellites=32` and `BatchConstraints`' time, whose spreads
(±25–101%) exceed their differences while their allocations are
identical. What the interleaved run confirmed: every executor loop row
(+23–37% time on the unfixed head, +7–17% on the final candidate,
p = 0.002 on each), the allocation movements of `AssignmentLoopStep`,
`ExecuteState`, `StateLoop`, `BatchConstraints`, `ActionChain`,
`WorkspaceEditSmallDocBesideLarge`, `ExecuteActionFreshContext`,
`Satisfy`, `FleetSatisfy` and `EditBeside` — each deterministic to the
unit on every count, and each the subject of a fix above — and the
analysis rows' improvement. Allocation counts agreed between the first
pass and every re-run on every row whose count is deterministic — all but
`RenameWarm` (±6–64%), `BatchConstraints` (±2% on the unfixed head) and
`ExecuteAction` (one count of six differed by one): the machine moves
time, not allocations.

## What this does not measure

- **The executor outside loops.** The runtime rows are loop bodies and
  materializations; a model that executes a few steps pays the frame
  feature-identity rule's resolution once per name read and the root
  frame's index once per execution — visible in `ExecuteAction` (+1
  allocation on the final candidate) and `ExecuteActionFreshContext`
  (+3 allocations, +4 KiB) and nowhere else.
- **Concurrent analysis.** `ValidateSplit/jobs=8` and `OpenSplit` run
  on 8 CPUs; their spreads (±23–374% on the unfixed head's six rounds)
  are the machine's, not the code's, and no time difference in them is
  significant.
- **Memory held at rest** beyond the `sysml -e 1` floor and the
  `-validate` resident sets above; a long-running language-server or
  gRPC session is not measured.
- **The stress constellation above 512 satellites** (`PlaneResidency`'s
  1 600-satellite row fails in setup on both revisions), and
  `ConvertAPIJSON/satellites=512` on the final candidate (skipped).
- **Other machines.** Every time here is from one host; allocations
  and bytes transfer, times do not.
