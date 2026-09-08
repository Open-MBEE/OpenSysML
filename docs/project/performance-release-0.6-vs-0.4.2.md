# Performance: release 0.6.0 against release 0.4.2

A release-gate measurement of `main` (`30f103bb`, 2026-09-07, tagged `v0.6.0`)
against `v0.4.2` (`6bebe3ea`, 2026-08-31), following the method of the
[0.5 against 0.4.3 record](performance-release-0.5-vs-0.4.3.md), the
[September census](performance-census-2026-09.md) and the
[profile](performance-profile-2026-09.md). The question is the release one:
does the 0.6 line perform at least as well as 0.4.2, and where it does not,
why.

All figures were taken on one machine — `Intel Xeon Platinum 8559C`, 8 CPUs,
~31 GiB, Go 1.25.0, Linux — the processor the reference figures in
`docs/internals/performance.md` were taken on, but a different one from the
0.5 record, so numbers here are not comparable to that document. The two
revisions were measured back to back and only the ratios between them carry.

Three columns appear throughout: **0.4.2**, **main** (as found), and
**main+fix** (with the changes this document ships). *Fixed* means the fix is
in the same change as this record; *explained* means the cause is known and
the cost is the intended price of a feature or rule that landed in the
interval, quantified. Nothing is left *open*.

## Method

- Both revisions built with `make build` into separate worktrees
  (`git worktree add ../opensysml-v0.4.2 v0.4.2`), so `bin/sysml` of each is
  its own binary.
- Every package that declares a benchmark on both revisions —
  `internal/repl`, `internal/core/model`, `internal/grpc` — run on both with
  `go test ./<pkg> -run '^$' -bench . -benchmem -count 6` and compared with
  `benchstat`. A movement is reported when `p ≤ 0.05` and the change exceeds
  about 5%; smaller significant movements are listed as noise. Packages whose
  benchmarks exist only on `main` were run there once, at `main+fix`, and are
  recorded without a comparison.
- Whole binary: `sysml -validate` over generated compliant models of 3 000,
  6 000 and 12 000 elements (parts with attributes, constraints and nested
  parts; calc definitions; state definitions), three runs each, wall time and
  maximum resident set from `getrusage`. Every binary reports `no errors` on
  all three. Process start as the mean of 50 `sysml --version` runs;
  empty-session load as the `LoadModel/elements=0` benchmark. The `examples/`
  models present on both revisions were run through `-validate`,
  `-instantiate`, `-action` and `-state` on both binaries. The 0.4.2 binary
  was run with its on-disk library index cache warm — the state a user's
  second run is in; the cold figure is given where it matters.
- Regressions were profiled with `-cpuprofile`/`-memprofile` and attributed to
  the commit that introduced the responsible code with `git log -S`.

### Workloads that are not comparable

- `internal/perfbench`, `internal/core/libs`, `internal/lsp`,
  `internal/core/parser`, `internal/core/runtime` and `internal/core/migrate`
  declare benchmarks only on `main`; 0.4.2 has none of them. They are recorded
  below as reference figures for the next comparison. Where a fix here moved
  one of them, the as-found and fixed figures are both given.
- `internal/repl`'s `RunStateMachine`, `RunCalc` and `Instantiate` exist on
  both revisions but the runtime they time was rebuilt between 0.4.2 and 0.5
  (the 0.5 record's findings 1–3 cover the part after 0.4.3); they are
  reported, and the movements are those fixes, not this one's.
- No committed baseline file was regenerated. The full suite, with the
  training and pilot corpora required, passes on `main+fix`.

## Benchmarks: `internal/repl`

| figure | 0.4.2 | main | main+fix | main+fix vs 0.4.2 |
| ------ | ----- | ---- | -------- | ----------------- |
| load wall, 250 / 1000 / 4000 elements | 16.7 / 65.0 / 274 ms | 22.6 / 86.2 / 347 ms | 17.3 / 68.8 / 297 ms (±17%) | +4% / +6% (p=0.07) / +8% |
| load bytes allocated, 250 / 4000 | 10.1 / 160 MiB | 13.7 / 215 MiB | 9.4 / 147 MiB | −7% / −8% |
| load allocations, 250 / 4000 | 82 k / 1.25 M | 129 k / 2.00 M | 98 k / 1.50 M | +19% / +19% |
| load live heap, 250 / 4000 | 2.15 / 32.8 MiB | 2.14 / 32.9 MiB | 2.14 / 32.9 MiB | ±0% |
| empty-session load wall | 170 µs | 220 µs (±18%) | 200 µs (±8%) | +18% (30 µs) |
| state-machine start, 250 / 1000 / 4000 | 13.6 / 36.7 / 220 µs | 7.2 / 8.1 / 6.8 µs | 7.1 / 7.0 / 7.4 µs | −48% / −81% / −97% |
| calc, 250 / 1000 / 4000 | 11.5 / 35.4 / 228 µs | 1.44 / 1.53 / 1.51 µs | 1.62 / 1.58 / 1.54 µs | −86% / −96% / −99% |
| instantiate, 250 / 1000 / 4000 | 10.4 / 32.6 / 226 µs | 4.9 / 5.2 / 5.3 µs | 6.3 / 5.2 / 5.6 µs (±17%) | −39% / −84% / −98% |
| instantiate bytes / allocs | 2.9 KiB / 39 | 6.6 KiB / 35 | 6.5 KiB / 35 | +127% / −10% |
| diagnostics, 50 / 200 / 800 attributes | 76.6 µs / 314 µs / 1.23 ms | — | 70.2 µs / 277 µs / 1.17 ms | −8% / −12% / −5% |

The three runtime rows are constant in model size on `main` where 0.4.2 paid
a cost linear in it: state-machine start no longer scans the model (the 0.5
record's finding 1), and `FeaturesOf` no longer locates a feature's owner by
walking the whole type graph (its finding 3). The +128% bytes per
instantiated object are the library features an object now carries (its
finding 2). None of this is touched here.

## Benchmarks: `internal/core/model` and `internal/grpc`

| benchmark | 0.4.2 | main | main+fix | main+fix vs 0.4.2 |
| --------- | ----- | ---- | -------- | ----------------- |
| `model` AnalyseUnresolved | 14.5 ms | 14.2 ms | 14.3 ms (±14%) | ~ (p=0.49) |
| `model` AnalyseResolved | 631 µs | 828 µs | 805 µs | +28% |
| `model` AnalyseResolved bytes / allocs | 370 KiB / 3.1 k | 460 KiB / 4.2 k | 429 KiB / 4.1 k | +16% / +30% |
| `grpc` ParseFileColdShared | 11.5 ms | 9.0 ms | — | −22% |
| `grpc` ParseFileColdInline | 12.8 ms | 9.1 ms | — | −29% |
| `grpc` ParseFileCold* bytes | 5.2 / 6.0 MiB | 5.0 / 5.2 MiB | — | −4% / −12% |

`AnalyseResolved` is a 20-attribute `part def` analysed with the library
resolved — 0.8 ms of which half is the pass registry — and is the one row
that still reads worse than 0.4.2 after the fixes; finding 3 prices it. The
gRPC cold parse is faster because the bundled library is decoded from an
embedded snapshot rather than parsed (`a27daf33`, *load the bundled library
from an embedded snapshot*, 2026-09-02).

## Benchmarks: packages without a 0.4.2 baseline

`internal/perfbench` at `main+fix`, `count 6`. The rows the fixes here moved
show the as-found figure first.

| benchmark | main → main+fix |
| --------- | --------------- |
| Analyze/synthetic | 1.13 s → 0.91 s (−20%) |
| Analyze/vehicle | 117 ms → 111 ms (−5%) |
| WorkspaceEdit/synthetic/reindex+diagnostics | 1.15 s → 1.02 s (−11%) |
| WorkspaceEdit/vehicle/reindex+diagnostics | 140 ms → 127 ms (−10%) |
| REPLLoadFile | 1.24 s → 1.12 s (−10%) |
| REPLSubmitSnippet | 1.15 s → 1.01 s (−12%) |
| GRPCParseFileUncached | 124 ms → 117 ms (−6%) |
| Lex/synthetic / vehicle | 7.6 ms / 262 µs |
| Parse/synthetic / vehicle | 83.5 ms / 2.25 ms |
| IndexAddExpand/synthetic / vehicle | 45.0 ms / 32.3 ms |
| IndexAddOnly/synthetic / vehicle | 46.5 ms / 1.17 ms |
| WorkspaceEdit/synthetic / vehicle (no diagnostics) | 174 ms / 47.8 ms |
| WorkspaceEditSmallDocBesideLarge | 303 ms |
| FQNOf / LookupQualified | 1.18 ms / 1.11 ms |
| FeaturesOf | 1.31 s |
| REPLEvalExpr | 79.9 ms |
| LowerActionGraph / LowerStateGraph | 1.38 µs / 5.39 µs |
| LowerChain/chain10 / 100 / 1000 | 7.1 µs / 128 µs / 8.4 ms |
| ExecuteAction / ExecuteActionFreshContext | 18.0 µs / 20.6 µs |
| ActionLoop/for10 / 100 / 1000 | 18.0 / 81.8 / 717 µs |
| ActionChain/chain10 / 100 / 1000 | 30.3 µs / 299 µs / 10.6 ms |
| ExecuteState | 336 µs |
| StateLoop/count50 / 500 / 5000 | 326 µs / 2.95 ms / 29.3 ms |
| BatchConstraints / SameConstraintManyInstances | 5.29 ms / 1.93 µs |
| BatchSatisfy / Instantiate | 1.46 s / 1.03 s |
| GRPCParseFileCached / GRPCEvaluate / GRPCVerifyConstraint | 58.1 µs / 9.59 µs / 18.1 ms |
| ConnectEvaluateHTTP / ConnectParseFileHTTPCached | 157 µs / 360 µs |

The other packages, at `main+fix`:

| package / benchmark | main+fix |
| ------------------- | -------- |
| `core/libs` ExpandWildcardImports / IndexLibrary / DecodeSnapshot / SetDigest | 33.1 ms / 14.0 ms / 9.1 ms / 963 µs |
| `lsp` FormatEdits / ReferencesWarm / ReferencesCold / RenameWarm / WorkspaceUpdate | 293 µs / 133 µs / 29.1 ms / 56.2 ms / 308 ms |
| `core/parser` ParseModel over `examples/pilot-corpora/sysml-examples` (99 files, 8 504 lines) | 11.1 ms |
| `core/runtime` MaterializePlainAttributes / SetFeatureValueNoDependents / DerivedReadWriteRead / AssignmentLoopStep | 1.30 ms / 164 ns / 2.46 µs / 738 µs |
| `core/migrate` WriterSiblingBlocks | 3.52 ms |

## Whole-binary scaling

`sysml -validate` over generated compliant models that report `no errors`,
three runs each; wall seconds and maximum resident set.

| elements | 0.4.2 | main | main+fix | main+fix vs 0.4.2 |
| -------- | ----- | ---- | -------- | ----------------- |
| 3 000 | 0.31–0.35 s / 108–111 MiB | 0.28–0.29 s / 148–151 MiB | 0.23 s / 115–130 MiB | −30% / +6–17% |
| 6 000 | 0.52–0.55 s / 150–162 MiB | 0.55–0.59 s / 205–218 MiB | 0.45–0.46 s / 168–184 MiB | −15% / +12% |
| 12 000 | 0.95–0.96 s / 246–260 MiB | 1.08–1.11 s / 323–334 MiB | 0.94–1.00 s / 271–286 MiB | ±0% / +10% |

Scaling is linear on both sides. The fixed binary starts ahead of 0.4.2 —
it does not load the library's index from disk at start (finding 4) — and the validation
slope closes the gap by 12 000 elements, where the two are level. The
resident set is 25 MiB larger at every size, which is the size difference of
the two binaries (below); the live heap of the loaded model is unchanged
(`LoadModel` live-B/op above), and the garbage the passes make per element
went down with the fixes (`LoadModel` B/op, −8%).

### Process start and session floor

| figure | 0.4.2 | main+fix |
| ------ | ----- | -------- |
| `sysml --version`, mean of 50 | 2.1 ms | 3.7 ms |
| binary size | 15.8 MiB | 39.2 MiB |
| Go package initialisation (`GODEBUG=inittrace=1`, sum) | 0.64 ms | 1.98 ms |
| empty-session load (`LoadModel/elements=0`) | 170 µs | 200 µs |

Start-up is 1.6 ms slower per process. The growth is the binary: `sysml` now
links `internal/grpc` and with it the protobuf and gRPC stacks, the Flexo
sync client, `internal/codegen` and `internal/edit`, none of which 0.4.2
linked, so it maps 23 MiB more and runs 1.3 ms more package initialisers.
This is the same finding the 0.5 record made against 0.4.3 and is
**explained**.

### Example models

Wall time and maximum resident set, best of three; every run returned exit 0
on both binaries.

| model / command | 0.4.2 | main+fix |
| --------------- | ----- | -------- |
| `action-executor-demo.sysml -action ActionExecutorDemo::sequential` | 0.11 s / 61 MiB | 0.02 s / 61 MiB |
| `action-executor-demo.sysml -validate` | 0.11 s / 58 MiB | 0.02 s / 61 MiB |
| `orthogonal-regions-demo.sysml -state …::TrafficLight -advance 20` | 0.12 s / 62 MiB | 0.04 s / 72 MiB |
| `phase-c-behavioral-bodies.sysml -instantiate PhaseC::Vehicle` | 0.11 s / 57 MiB | 0.03 s / 64 MiB |
| `phase-c-behavioral-bodies.sysml -state PhaseC::AutopilotMode -advance 20` | 0.11 s / 56 MiB | 0.02 s / 63 MiB |
| `disposal-robot-demo/robot.sysml -validate` | 0.14 s / 65 MiB | 0.04 s / 69 MiB |

At example scale the 0.6 binary is 3–5× faster end to end: 0.4.2 spent
~90 ms of every run decoding the bundled library's index from its on-disk
cache (`~/.cache/sysml-ls/libs`, 96 record files) — and 0.26 s when that
cache is cold and the library is parsed — where the embedded snapshot
decodes in 9 ms (finding 4). The `robot.sysml` example, which the
0.5 record found failing to validate on the 0.5 line, validates clean on
both binaries here.

## Findings

Ordered by size. Each names the cause, the responsible change, the measured
cost, and its status.

### 1. Inherited-name conflicts merged every base's members per declaration — *fixed*

`LoadModel` +27–36% wall and +57–60% allocations as found; the inherited-name
conflict pass was 12% of the 4 000-element load profile against 1% on 0.4.2,
and a third of every byte allocated. Commit `1c851321` (*inherit a subsetted
library feature's own members in the inherited-name rule*, 2026-09-06)
extended the pass to the members of every type a declaration passes through
on the way to its library bases. The pass already memoized each library
base's members by name, but for every checked declaration it merged those
maps — hundreds of names from `Parts::Part` and its supertypes — plus the
passed-through types' own members into a fresh `map[string][]candidate`,
so the whole visible member set of the library was copied once per part,
attribute, action and state in the model.

**Fixed**: the contributors are kept as they are memoized — one map per
library base, one per passed-through type — and looked up by name rather
than merged. A declaration's own names are checked by asking each
contributor; the names two contributors both supply are found by walking
every contributor but the largest (a name only the largest supplies is
reached once and cannot conflict) and counting the others; both answers are
the ones the merged map gave, in the same contributor order, and the
diagnostics are unchanged (`TestW9C*` in `internal/core/passes/w9c_rules_test.go` covers the library-base,
user-supertype, inherited-through-feature, specializing-own-name and
inherited-short-name cases, warning and silent alike). The pass is 2.9% of the fixed load profile;
`LoadModel` allocates 8% fewer bytes than 0.4.2 and runs +4–8% instead of
+27–36%; `Analyze/synthetic` −20%, `REPLLoadFile` −10%.

### 2. OOSEM classification conformed the same type once per feature — *fixed*

The OOSEM method pass (`a88b7749`, *add OOSEM viewpoints, views and method
checks*, 2026-09-05) classifies every declaration by whether one of its
types conforms to an OOSEM definition. A feature's kind was memoized, but
the classification of its *type* was not, so an `Integer` attribute typed
fifty times ran the twelve `Conforms` walks against the OOSEM definitions
fifty times. **Fixed**: the type's classification is memoized alongside the
feature's. On `AnalyseResolved`, whose twenty attributes share one type, the
pass went from 9.0% to 6.4% of the profile and `kindOfType` from 3.3% to
0.3%; on the 4 000-element load, whose types are many, the pass is 4.4%
either way. What remains of it is `FeatureTypeSet` per feature (finding 3).

### 3. New validation passes on the load path — *explained*

After the fixes, `LoadModel` is +4% / +6% / +8% at 250 / 1 000 / 4 000
elements against 0.4.2, `AnalyseResolved` +28% (174 µs on 631), and the
validation slope of the whole binary is level at 12 000 elements.
`internal/core/passes` gained sixteen rule files in the interval: annotation
ownership, conjugation, control nodes, end features and end multiplicities,
enumeration bodies, feature declarations, feature-value overriding, identity
metadata, invocations, OOSEM methods, return parameters, send actions, and
the type-check tier's operator, relationship-end and trigger rules. In the
fixed 4 000-element load profile the passes other than name resolution take
28% of the time against 14% on 0.4.2, and no single one dominates: identity
metadata 4.4%, OOSEM 4.4%, constraint and type checking 3.2% each, inherited
names 2.9%, control-node successions 1.6%, and the rest under 1.5%. Parsing
(20%) and name resolution (14%) are unchanged.

Two of the new passes share one cost worth naming. `identity.Build`
(`701a75ef`, *IdentityMetadata library, identity side table, and validation
pass*, 2026-09-01) computes identity metadata for every symbol in the model
and every annotation target — 3.9% of the load profile, 6.7% of
`AnalyseResolved`. The OOSEM pass asks `semantics.Model.FeatureTypeSet` of
every feature, and that in turn asks `implicitBases` — the library base a
declaration's kind implies unless its declared generalizations already reach
it — which is not memoized: the declared-generalization walk is 4.7% of the
fixed load profile and 10.9% of `AnalyseResolved`, from 2% and 4.5% on
0.4.2, because a pass now asks it for every feature. Each of these is linear
in the model and reads its subjects once; memoizing `implicitBases` would
need the same re-entrancy guard `DirectSupertypes` carries (a cyclic
generalization must not pin a partial answer) and is the one further
optimization this measurement identifies. At the figures above it is a few
percent of load and is not a release-gate change. The +28% on
`AnalyseResolved` is these passes on a 0.8 ms workload where the pass
registry's fixed costs are half the total; the same passes are the +8% on a
4 000-element load and 0% on a 12 000-element validation. This is the rules
doing their work and is **explained**.

### 4. Library loaded from an embedded snapshot — *improvement, explained*

Commit `a27daf33` (2026-09-02) replaced loading the bundled library at
start — parsed on a cold cache, decoded from ~100 per-file gob records in
`~/.cache/sysml-ls/libs` on a warm one — with decoding one embedded snapshot
(`DecodeSnapshot`, 9 ms in `core/libs`). It is why every `examples/` run is
3–5× faster than on 0.4.2 with 0.4.2's cache warm, why `ParseFileCold*` is
−22% / −29%, and why the whole-binary validation of 3 000 elements is −30%. The +18% (30 µs) on the empty-session
load is unrelated to it — the session floor is measured with the library
already loaded — and is the constant set-up of the new passes' side tables;
below the size a profile separates from session set-up.

### 5. Smaller movements — *explained*

- `LoadModel` allocations +19% after the fixes, while bytes are −8%: the
  new passes' side tables are many small objects (identity infos, kind
  memos, per-name lookups) where the code they replaced allocated a few
  large maps. Live heap is unchanged.
- Whole-binary resident set +10–12% at 6 000 and 12 000 elements: the
  25 MiB the larger binary maps, as under *Process start*. Garbage per
  element went down.
- `Diagnostics` −5–12%: diagnostics over a document with unresolved names,
  an improvement below the size this record investigates.
- `Instantiate` bytes +128%: the library features an object carries since
  `c3baef7a`, priced in the 0.5 record's finding 2.

## Verdict

As found, `main` was not on par with 0.4.2 on the load path: loading and
validating a model was 27–36% slower and allocated 35% more, and every
diagnostics-bearing benchmark in `perfbench` carried the same cost. One
pass — the inherited-name conflict rule, after it was extended on 2026-09-06
— accounted for most of it, by merging the library's visible member set once
per declaration. The runtime, gRPC and library-loading paths were already
well ahead of 0.4.2.

With the fixes in this change, `main` validates a 3 000-element model 30%
faster than 0.4.2, a 6 000-element model 15% faster, and a 12 000-element
model in the same time, allocating 8% fewer bytes per element; every
`examples/` command runs 3–5× faster end to end; the gRPC cold parse is
22–29% faster; instantiation, calc and state-machine start are constant in
model size where 0.4.2 was linear. `LoadModel` is 4–8% slower and
`AnalyseResolved` 28% slower on the benchmarks' workloads, all of it in the
sixteen validation rules added since 0.4.2, none of it algorithmic, and the
one further memoization worth making is named. Process start is 1.6 ms
slower and the resident set 25 MiB larger, both the binary's size. Parsing,
name resolution, diagnostics and the live heap are unchanged.

## Reproducing

```bash
git fetch --tags
git worktree add ../opensysml-v0.4.2 v0.4.2
(cd ../opensysml-v0.4.2 && make build)
make build
for pkg in internal/repl internal/core/model internal/grpc; do
  (cd ../opensysml-v0.4.2 && go test ./$pkg -run '^$' -bench . -benchmem -count 6) > old.$pkg.txt
  go test ./$pkg -run '^$' -bench . -benchmem -count 6 > new.$pkg.txt
  benchstat old.$pkg.txt new.$pkg.txt
done
for pkg in internal/perfbench internal/core/libs internal/lsp internal/core/runtime internal/core/migrate; do
  go test ./$pkg -run '^$' -bench . -benchmem -count 6 > new.$pkg.txt
done
OPENSYSML_BENCH_MODEL=examples/pilot-corpora/sysml-examples go test ./internal/core/parser -run '^$' -bench ParseModel -benchmem -count 6
go test ./internal/repl -run '^$' -bench 'LoadModel/elements=4000' -benchtime 10x -cpuprofile load.cpu -memprofile load.mem
go test ./internal/core/model -run '^$' -bench AnalyseResolved -cpuprofile ar.cpu
../opensysml-v0.4.2/bin/sysml -validate gen12000.sysml
bin/sysml -validate gen12000.sysml
```
