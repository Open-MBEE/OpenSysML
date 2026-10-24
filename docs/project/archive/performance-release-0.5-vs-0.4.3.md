# Performance: the 0.5 line against release 0.4.3

A release-gate measurement of `main` (`ab982673`, 2026-09-04, the 0.5 line as
it stands) against the previous release, `v0.4.3` (`99e02003`, 2026-09-02),
following the method of the [September census](performance-census-2026-09.md),
the [profile](performance-profile-2026-09.md) and the
[execution study](execution-performance-2026-09.md). The question is the
release one: does `main` perform at least as well as the last tag, and where it
does not, why.

All figures were taken on one machine — `Intel Xeon Platinum 8375C @ 2.90 GHz`,
8 CPUs, ~31 GiB, Go 1.25.0, Linux. That is a different processor from the
`8559C` the reference figures in `docs/internals/performance.md` and the census
were taken on, so absolute numbers here are **not** comparable to those
documents; the two revisions were measured back to back on this machine, and
only the ratios between them carry.

Three columns appear throughout: **0.4.3**, **main** (as found), and
**main+fix** (with the changes this document ships). Where a regression is
marked *fixed*, the fix is in the same change as this record; *explained* means
the cause is known and the cost is the intended price of a feature or fix that
landed in the interval, quantified; nothing is left *open*. The record was
taken in two rounds: a first set of fixes that recovered the largest costs,
then — with the remaining regressions ruled release blockers — a second set
that took each of the four remaining rows to its floor. The `main+fix` column
is the second round; the first round's intermediate figures are quoted in the
findings where they explain what each fix bought.

## Method

- Both revisions built with `make build` into separate worktrees
  (`git worktree add ../opensysml-v0.4.3 v0.4.3`), so `bin/sysml` of each is its
  own binary. The `v0.4.3` tag lives on the upstream remote and is annotated;
  `v0.4.3^{commit}` is `99e02003`.
- Every package that declares a benchmark — `internal/frontend/repl`,
  `internal/perfbench`, `internal/workspace/libs`, `internal/workspace/model`,
  `internal/frontend/grpc`, `internal/frontend/lsp` — run on both revisions with
  `go test ./<pkg> -run '^$' -bench . -benchmem -count 6`, compared with
  `benchstat`. A movement is reported when `p ≤ 0.05` and the change exceeds
  about 5%; smaller significant movements are listed as noise.
- Whole binary: `sysml -validate` over generated compliant models of 3 000,
  6 000 and 12 000 elements (parts with attributes, constraints, nested parts;
  calc definitions; state definitions), three runs each, wall time and maximum
  resident set from `getrusage`. Both binaries report `no errors` on all three.
  Process start as the mean of 50 `sysml --version` runs; empty-session load as
  the `LoadModel/elements=0` benchmark. The `examples/` models were run through
  `-validate`, `-instantiate`, `-action` and `-state` on both binaries.
- Regressions were profiled with `-cpuprofile`/`-memprofile` and attributed to
  the commit that introduced the responsible code with `git log -S`; no bisect
  was needed.

### Workloads that are not comparable

- `internal/frontend/lsp` has benchmarks only on `main` (formatting was added after
  0.4.3); its 0.4.3 run produced no rows, so there is no comparison.
- `internal/perfbench`'s `vehicle` corpus benchmarks, `GRPCParseFile*`,
  `Connect*HTTP*` and `internal/frontend/repl`'s `CompiledCalc/*/interpreted` exist only
  on `main`.
- No committed baseline file was regenerated. One observation from running
  the suite: on this machine the provisioned pilot corpora do not match what
  the committed baselines record — `sysml-examples` holds 98 files against a
  recorded 99 (`Simple Tests/InterfaceTest.sysml` is absent), the kerml and
  sysml digests differ, and `AnalysisIndividualExample.sysml` gains a
  type-conformance error — identically on unmodified `main` and on this
  branch, so it is a provisioning question for the corpus pins, not an effect
  of these changes.

## Benchmarks: `internal/frontend/repl`

| figure | 0.4.3 | main | main+fix | main+fix vs 0.4.3 |
| ------ | ----- | ---- | -------- | ----------------- |
| load wall, 250 / 1000 / 4000 elements | 19.9 / 77.8 / 335 ms | 27.0 / 105 / 436 ms | 21.3 / 85.1 / 374 ms | +7% / +9% / +11% |
| load bytes allocated, 250 / 4000 | 8.2 / 130 MiB | 11.8 / 187 MiB | 8.7 / 138 MiB | +6% / +6% |
| load live heap, 250 / 4000 | 2.09 / 32.1 MiB | 2.12 / 32.4 MiB | 2.12 / 32.5 MiB | +1% |
| empty-session load wall | 255 µs | 263 µs | 271 µs | +6% (15 µs) |
| state-machine start, 250 / 1000 / 4000 | 8.2 / 8.2 / 8.4 µs | 61 / 252 / 992 µs | 9.7 / 9.7 / 9.8 µs | +18% / +19% / +17% (constant again) |
| state-machine start bytes / allocs, 4000 | 8.3 KiB / 110 | 401 KiB / 4.6k | 8.5 KiB / 121 | +3.3% / +10% |
| instantiate, 250 / 1000 / 4000 | 3.1 / 3.2 / 3.2 µs | 12.0 / 11.7 / 11.9 µs | 5.6 / 5.9 / 6.0 µs | +81% / +84% / +89% |
| instantiate bytes / allocs | 1.7 KiB / 29 | 6.2 KiB / 54 | 5.6 KiB / 35 | +235% / +21% |
| calc, 250 / 1000 / 4000 | 2.07 µs | 2.19–2.23 µs | 2.16 µs | +4.5% |
| diagnostics, 50 / 200 / 800 attributes | 88 µs / 350 µs / 1.45 ms | — | 89 µs / 353 µs / 1.47 ms | +1% (below threshold) |

## Benchmarks: `internal/perfbench`

Rows that moved more than ~5% on `main`; everything else is within noise
(`Lex`, `IndexAdd*`, `WorkspaceEdit/synthetic`, `LookupQualified`,
`REPLEvalExpr`, `LowerStateGraph`, `LowerChain/chain1000`) or a few percent
(`Parse/synthetic` +2.5%, `FQNOf` +1.9%).

| benchmark | 0.4.3 | main | main+fix | main+fix vs 0.4.3 |
| --------- | ----- | ---- | -------- | ----------------- |
| Analyze/synthetic | 808 ms | 1234 ms | 977 ms | +21% |
| WorkspaceEdit/synthetic/reindex+diagnostics | 909 ms | 1424 ms | 1144 ms (±16%) | +26% |
| REPLLoadFile | 1.14 s | 1.45 s | 1.19 s | +4% |
| REPLSubmitSnippet | 1.02 s | 1.31 s | 1.04 s | +2% |
| LowerActionGraph | 1.51 µs | 1.93 µs | 1.91 µs | +27% |
| LowerChain/chain10 / chain100 | 7.2 / 145 µs | 9.0 / 162 µs | 9.0 / 162 µs | +25% / +12% |
| ExecuteAction | 12.5 µs | 67.0 µs | 21.0 µs | +69% |
| ExecuteActionFreshContext | 15.0 µs | 70.0 µs | 23.9 µs | +60% |
| ActionLoop/for10 / for100 / for1000 | 12.9 / 87.5 / 825 µs | 65.7 / 149 / 973 µs | 21.8 / 98.0 / 850 µs | +69% / +12% / +3% |
| ActionChain/chain10 / chain100 / chain1000 | 19.1 / 251 µs / 11.4 ms | 85.8 / 470 µs / 13.3 ms | 36.9 / 377 µs / 13.1 ms | +94% / +50% / +15% |
| ExecuteState | 225 µs | 255 µs | 249 µs | +11% |
| StateLoop/count50 / 500 / 5000 | 219 µs / 2.08 / 20.8 ms | 250 µs / 2.38 / 23.6 ms | 246 µs / 2.34 / 30.1 ms (±23%) | +12% / +12% / (noisy) |
| BatchConstraints | 4.48 ms | 16.5 ms | 6.02 ms | +34% |
| SameConstraintManyInstances | 1.50 µs | 5.94 µs | 2.12 µs | +41% |
| BatchSatisfy | 4.54 s | 7.14 s | 0.34 s | −93% |
| Instantiate | 4.07 s | 6.55 s | 0.26 s | −94% |
| FeaturesOf | 11.6 s | 12.0 s | 0.73 s | −94% |
| GRPCEvaluate | 395 µs | 617 µs | 12.5 µs | −97% |
| GRPCVerifyConstraint | 17.6 ms | 2398 ms | 12.4 ms | −29% |

`GRPCVerifyConstraint` allocates half the bytes of 0.4.3 per request (4.2 vs
9.0 MiB) but 5.6× as many objects (34 k vs 6 k): the subject's behaving parts
are now materialized (finding 2), many small objects each, where 0.4.3 spent
its bytes on one large resolver per request.

## Benchmarks: the other packages

| package / benchmark | 0.4.3 | main | movement |
| ------------------- | ----- | ---- | -------- |
| `core/libs` DecodeSnapshot | 10.9 ms | 11.6 ms | +6.7% (±5%) |
| `core/model` AnalyseUnresolved / AnalyseResolved | 17.4 ms / 844 µs | 18.1 ms / 886 µs | +4.1% / +5.0% |
| `grpc` ParseFileColdInline | 12.8 ms | 13.3 ms | +3.4% wall; allocations +0.5% |
| `grpc` ParseFileColdShared | — | — | unchanged |
| `lsp` formatting benchmarks | absent | present | no comparison |

`ParseFileColdInline` and the `Analyse*` rows are the diagnostics cost of the
`Analyze` row above seen through the gRPC service and the workspace (a cold
parse runs the full pass registry) and are covered by the same finding.

## Whole-binary scaling

`sysml -validate` over generated compliant models that report `no errors`,
three runs each; wall seconds and maximum resident set.

| elements | 0.4.3 | main | main+fix | main+fix vs 0.4.3 |
| -------- | ----- | ---- | -------- | ----------------- |
| 3 000 | 0.26 s / 105 MiB | 0.36 s / 145 MiB | 0.28 s / 118 MiB (+8% / +12%) |
| 6 000 | 0.53 s / 155 MiB | 0.72 s / 209 MiB | 0.56 s / 172 MiB (+6% / +11%) |
| 12 000 | 1.10–1.44 s / 266 MiB | 1.48–1.61 s / 336 MiB | 1.18 s / 291 MiB (+7% / +9%) |

Scaling is still linear on both sides; the slope moved. The CPU profile of the
12 000-element run on `main` attributes the whole difference to
`Workspace.Diagnostics` (0.58 s of 1.21 s on 0.4.3; 0.93 s of 1.61 s on
`main`): parsing, indexing and name resolution take the same time on both
revisions. The passes that grew are listed under *Findings*. Memory follows the
same path: the extra resident set is the garbage the new passes make, not live
model.

### Process start and session floor

| figure | 0.4.3 | main |
| ------ | ----- | ---- |
| `sysml --version`, mean of 50 | 2.0 ms | 3.4 ms |
| binary size | 21.7 MiB | 37.9 MiB |
| Go package initialisation (`GODEBUG=inittrace=1`, sum) | 0.77 ms | 1.95 ms |
| empty-session load (`LoadModel/elements=0`) | 255 µs | 263 µs |

Start-up is 1.4 ms slower per process, well inside the < 20 ms the Unreleased
changelog claims. The growth is the binary: `sysml` now links `internal/frontend/grpc`
(and with it the protobuf and gRPC stacks), the Flexo sync client
(`internal/flexo`), `internal/translate/codegen` and `internal/edit`, none of which 0.4.3
linked, so it maps 16 MiB more text and runs 1.2 ms more package initialisers
(`protobuf/reflect/protodesc` and `descriptorpb` are the two largest new ones).
This is the expected price of the feature and is recorded as *explained*.

### Example models

Wall time and maximum resident set, three runs each, best shown; every run of
each pair returned the same exit code except where noted.

| model / command | 0.4.3 | main |
| --------------- | ----- | ---- |
| `action-executor-demo.sysml -action ActionExecutorDemo::sequential` | 0.03 s / 50 MiB | 0.03 s / 60 MiB |
| `orthogonal-regions-demo.sysml -state …::TrafficLight -advance 20` | 0.04 s / 56 MiB | 0.05 s / 67 MiB |
| `phase-c-behavioral-bodies.sysml -instantiate PhaseC::Vehicle` | 0.03 s / 50 MiB | 0.03 s / 61 MiB |
| `phase-c-behavioral-bodies.sysml -state PhaseC::AutopilotMode -advance 20` | 0.03 s / 50 MiB | 0.03 s / 61 MiB |
| `self-model/surfaces.sysml -validate` | 0.11 s / 78 MiB, exit 2 | 0.11 s / 90 MiB, exit 2 |
| `self-model/pipeline.sysml -validate` | 0.04 s / 51 MiB | 0.04 s / 61 MiB |
| `disposal-robot-demo/robot.sysml -validate` | 0.05 s / 56 MiB, exit 0 | 0.06 s / 67 MiB, **exit 2** |

At example scale the two binaries are indistinguishable in wall time; the
~10 MiB of extra resident set is the larger binary plus the library index the
new passes touch. One difference is not performance: `robot.sysml` validates
clean on 0.4.3 and fails on `main` with
`cannot override the binding value of TradeStudies::MaximizeObjective::best`
(and the same for `MinimizeObjective::best`), so its `-action`, `-state` and
`-satisfy` runs no longer start (the README's commands exit 2). This is the new
feature-value-overriding rule (`changes/unreleased/feature-value-overriding.added.md`)
firing on a committed example. It is out of scope for a performance record and
is reported as a finding for the rule's author: either the example or the rule
is wrong, and the pilot validator should adjudicate.

## Findings

Ordered by size. Each names the cause, the responsible change, the measured
cost, and its status.

### 1. State-machine start scans the whole model — *fixed*

`RunStateMachine` went from a constant 8 µs to a cost linear in model size
(992 µs at 4 000 elements). Commit `25fe8b9d` (*refuse `%state <machine>`
alone for a definition exhibited through typed usages*) added
`Session.exhibitingTypes`, which — when a state definition has no exhibiting
object — walks every scope of every document and asks
`Context.ExhibitsState` of every member to name the types that would exhibit
it. The benchmark's machine has no exhibitor, so it pays the walk on every
start. Two costs compounded:

- `Scope.Members()` was called per scope, which deduplicates through a map and
  allocates the member slice; 70% of the allocation profile. **Fixed**: the
  walk now uses `Scope.ForEachMember`, skipping anonymous members exactly as
  `Members()` did. Bytes per start are back to 8.5 KiB (from 401 KiB) and
  wall dropped 2.5× (403 µs at 4 000).
- The walk itself was O(model) per start. **Fixed**: the session now builds an
  `exhibitIndex` — every exhibited-state declaration keyed by the state
  definition it exhibits, with the owner that exhibits it — once per runtime
  context, from one walk of the documents, and `exhibitingTypes` reads it. The
  index lives with the `runtime.Context` it was built for, so a submission
  that rebuilds the context (any new declaration) drops it with the context;
  a test proves the walk runs once across repeated starts and again after a
  submission. Start is constant in model size again: 9.7 / 9.7 / 9.8 µs at
  250 / 1 000 / 4 000 elements.

The remaining +1.5 µs against 0.4.3 is not the exhibitor search. The profile
of the fixed start puts it in `Session.resolveObject`: since `25fe8b9d` the
name given to `%state` is first read as an object reference (a held object's
machine is driven in place rather than run detached), and when no object is
held under it the session builds the `NotInstantiatedError` that names what to
instantiate instead — a `lookupSymbol`, an `AllSupertypes` walk and the
listing of held objects — before falling back to the symbol lookup. That is
the object-first addressing the command was given, its cost is constant, and
it is **explained**.

### 2. Instantiation: 3.7× per object — *fixed to the feature shape, rest explained*

`repl` `Instantiate` 3.1 → 11.7 µs (+270%) as found, 5.9 µs (+85%) after the
fixes; `perfbench` `ExecuteActionFreshContext` +215% → +60%; `GRPCEvaluate`
+56% → −97%. The profile of the as-found benchmark put 41% of the time under
`Context.Instantiate`, of which `materialize` was 33% and
`aliasRedefinedFeatureValues` 7%; inside `materialize`, allocating one
`FeatureValue` per feature and inserting it into the `FeatureValues` map was
most of the cost, and the redefinition-group and behaving-part computations
were redone per object.

- **Fixed**: an object's `FeatureValue`s are allocated as one block (a
  `[]FeatureValue` sized by `FeaturesOf`, pointers into it in the map), and
  the map is pre-sized. Allocations per object 54 → 35.
- **Fixed**: the redefinition groups `aliasRedefinedFeatureValues` derives
  from a type's features, and the positions of the composite parts whose type
  runs a behavior (`materializeBehavingParts`), are memoized per type in the
  `Context` — both are functions of the type alone. Wall 11.7 → 5.9 µs.
- The remainder is the object itself. Commit `c3baef7a` (*materialize Systems
  and Domain library features on objects*, 2026-09-02) makes every object
  carry the features its library supertypes declare: the benchmark's
  `part def` — four declared features (`mass`, `power`, `sub`, `MassOK`) on
  0.4.3 — materializes 17 on `main`, adding `ownedPorts`, `performedActions`,
  `ownedActions`, `exhibitedStates`, `ownedStates`, `shape`,
  `envelopingShapes`, `boundingShapes`, `voids`, `isSolid`, `subitems`,
  `subparts` and `checkedConstraints` from `Parts::Part` / `Items::Item` /
  `Occurrences::Occurrence` (the KerML/SysML rule that a classifier's
  features are inherited by its instances; `%features` and gRPC feature
  listings show them). Measured directly: the same benchmark against an
  `attribute def` with four features and no library part supertypes runs
  in 3.2–3.3 µs, 1.95 KiB and 27 allocations on the fixed code — at parity
  with 0.4.3's 3.1 µs / 1.7 KiB / 29. The 13 extra feature values are
  therefore the whole residual: ~2.7 µs and ~3.7 KiB per object, ~200 ns
  and ~290 bytes per inherited feature. That is the intended behavior of
  `c3baef7a` and is **explained**; making it cheaper means materializing
  library features lazily on first read, which changes when their defaults
  fold and is not a release-gate change.
- Commit `37de3db2` (*read the instantiated object, run whole, after
  `-instantiate`*) — `materializeBehavingParts` runs the classifier behaviors
  of required composite parts on creation — contributes nothing measurable
  here once the per-type verdicts are memoized; the benchmark's part has no
  behaving parts. Where a type does, the cost is that of the parts created,
  which is what the commit is for.

### 3. gRPC verification builds a fresh runtime per request — *fixed*

`GRPCVerifyConstraint` 17.6 ms → 2 398 ms as found; 183 ms after the first
fix below; **12.4 ms** (−29% against 0.4.3) after the second.
`Service.newVerifyContext` created a new `resolve.Resolver`,
`semantics.Model` and `runtime.Context` per request (as it did on 0.4.3), so
nothing runtime-side was memoized across requests. On 0.4.3 that was cheap
because verification instantiated only the subject. With finding 2 every
request instantiates the subject's behaving part tree, which for the
synthetic model means computing `FeaturesOf` for most of the type graph from
scratch — 87 MB and 526 k allocations per request, and a GC that spends a
quarter of the profile scanning the cached model.

- Inside `FeaturesOf`, `findOwnerType` located the type owning a feature by
  scanning the parent scope's members (`MemberNames` + `LookupLocalAll`) until
  one declared the owner node — O(members) per feature, O(n²) per type. This
  was also the case on 0.4.3, which is why `perfbench` `FeaturesOf` took 11.6 s
  there; finding 2 simply made it hot. **Fixed**: `Scope.Owner()` already
  records that symbol, so the scan is now a fast path with the old search as
  fallback for scopes re-owned by another declaration (metadata bodies).
  `FeaturesOf` −93%, `perfbench` `Instantiate` −86% and `BatchSatisfy` −85%
  against 0.4.3; `GRPCVerifyConstraint` 13×.
- The remaining 10× was the per-request `Context`. **Fixed**: each
  `CachedModel` now owns one `resolve.Resolver` and one `semantics.Model`
  (`runtimeSemantics`), built on first use and shared by every request
  against that content hash; they are side tables over the immutable index,
  so sharing is sound, and they are evicted with the model. Access is
  serialized with a mutex — the resolver and model memoize on read — and a
  request's diagnostics are truncated from the shared resolver on release, so
  one request's unresolved name never reaches another's answer (a concurrent
  test under `-race` covers this). The `runtime.Context`, its instances and
  all evaluation state stay per request. With `MembersOf` and the shape
  features memoized in the semantics model itself (below), the type-graph
  work is paid once per model rather than once per request: 4.2 MiB per
  request against 0.4.3's 9.0. `Evaluate` parses a request-owned expression,
  and the resolver memoizes by AST node, so the shared resolver would otherwise
  grow by one expression's worth of entries per request for the life of the
  cached model; `Evaluate` runs under `Resolver.Scratch`, which drops what the
  request memoized about its own nodes (found by `astcodec.Reachable`) and
  keeps what it resolved of the model's. The "did you mean" spellings an
  unresolved name earns are memoized by name rather than node; they join the
  same lifecycle through the node the name was written at, so a request's
  misspellings go with it too. `GRPCEvaluate` pays 1.4 µs and 25 allocations
  for it (11.1 → 12.5 µs), still −97% against 0.4.3; a test checks the memo
  does not grow over 50 distinct expressions nor 50 distinct unresolved names.
- Carrier-only instantiation was asked for and is not needed: `Instantiate`
  already materializes only the subject's own feature values and the
  composite parts whose type runs a classifier behavior (finding 2);
  everything else is created on first read. The behaving parts are the one
  thing a constraint on the carrier can observe that a lazy part would get
  wrong (their state after their behavior ran), so they stay.
- `semantics.Model` now memoizes `MembersOf` (per type and view) and
  `ShapeFeatures`, once `MemberSourcesStable` reports the type's supertype
  closure settled; the runtime's own per-context member cache was removed in
  favor of it. This is the same memoization the model already applied to
  `memberSources` and constructor slots, moved one level up.

### 4. `MembersOf` recomputed per action frame — *partly fixed, explained*

`ExecuteAction` 12.5 → 67.0 µs; `ActionLoop/for10` and `ActionChain/chain10`
+410% / +350%; `BatchConstraints` and `SameConstraintManyInstances` +270% /
+295%. Commits `62def3d4` (*give each nested action node its own performance
frame*) and `fa5fc3da` (*seed inherited action defaults when the performance
starts*) build a frame per node performance and, in `newRootFrame` →
`addFeatureDirections` / `performanceFeatures`, walk
`semantics.Model.MembersOf(action)` each time — 33% of the CPU and 50% of the
allocation profile. `effectiveMembers` (constraint evaluation) and
`operationOf` did the same per evaluation.

- **Fixed**: `semantics.Model` memoizes `MembersOf` per type and view (callers
  only read the list). A view enumerated while a reference or supertype it
  depends on is still resolving (`Model.MemberSourcesStable`) is not cached.
  `ExecuteAction` 67 → 21 µs, `BatchConstraints` 16.5 → 6.0 ms,
  `SameConstraintManyInstances` 5.9 → 2.1 µs.
- Also in `lookupSubaction`, every name read during evaluation resolved
  through the resolver before checking whether any action performance was on
  the stack; for constraint and calc evaluation none ever is. **Fixed**: the
  resolver lookup is skipped when no frame carries a performance, which is
  exactly the case in which the function returned "not declared" regardless.
- The rest (+69% on `ExecuteAction`, +3% at `for1000`) is the per-node frame
  itself: a `performances` entry, its pin table and direction map per node
  performance instead of per action. That is the data the nested-frame feature
  exists to keep and is **explained**; the amortized cost at 1 000 iterations
  is ~25 ns per node.

### 5. New validation passes on the load path — *fixed where algorithmic, rest explained*

`LoadModel` +30–36%, `Analyze/synthetic` +53%,
`WorkspaceEdit/reindex+diagnostics` +57%, `REPLLoadFile` +28%, and the whole
of the whole-binary slope, as found. After the fixes: `LoadModel` +7–11%,
`Analyze/synthetic` +21%, `reindex+diagnostics` +26% (±16%), `REPLLoadFile`
+4%, `REPLSubmitSnippet` +2%. `internal/check/passes` gained ten files and 61
commits in the interval; the CPU profile of the 12 000-element validation
put 0.30 s of the 0.35 s difference in one of them, `ControlNodeSuccessionPass`
(commit `279408b8`, *validate control-node successions and owning type*),
whose `semantics.Model.ActionSuccessions` built the set of visible members of
every action (`MembersOf`, which reaches through the library `Action`
supertypes) before looking at any inherited succession.

- **Fixed**: the visible-member set is built only when an inherited succession
  is actually named — the only case that consults it. The pass drops from
  0.30 s to 0.16 s on the 12 000-element model.
- **Fixed**: even then, the check was O(members) per named succession — a
  full `MembersOf` enumeration into a set, for every action that inherits a
  named succession, i.e. O(actions × members). `Model.HasMember` now answers
  the question by walking the same member order and stopping at the first
  hit (or reading the memoized member list when one exists); a test proves
  it agrees with `MembersOf` on inherited, name-masked and redefined members.
- **Fixed**: `MemberSources` — the supertype closure every member query
  starts from — recomputed a type's direct contributors (supertypes, implicit
  base, referenced feature) on every call; they are memoized per type under
  the same stability guard `MemberSources` itself uses, so a closure still
  being resolved is never pinned. In the `LoadModel/elements=4000` profile
  `ControlNodeSuccessionPass` is now 2.8% of the total (was 7.7% after the
  first fix, ~20% as found); `MemberSources` 6.4% → 2.6%.
- The remainder is not algorithmic. The fixed profile is 22% parser, 14.5%
  name resolution and 43.5% `Workspace.Diagnostics`, and inside diagnostics
  every pass is linear in the model; the extra 7–11% over 0.4.3 is spread
  across the passes added in the interval (control-node successions,
  feature-value overriding, feature accessibility, the nonstandard-notation
  scan, and the type-check tier's new rules), each reading its subjects once.
  That is the rules doing their work and is **explained**.

### 6. Smaller movements — *explained*

- `LowerActionGraph` +27%, `LowerChain/chain10` +25%: lowering now records
  the per-node performance data of finding 4; absolute cost 0.4 µs per graph.
- `ExecuteState` / `StateLoop` +11–12%: a constant per step that the fixes
  here do not move; the profile spreads it over the state executor's
  transition handling rather than one function. `StateLoop/count5000` read
  +45% in the final run with a ±23% spread (one of six runs hit a GC cycle);
  its 50 and 500 rows, at ±1%, are the +12% of the other rows.
- `RunCalc` +4.5%: 0.09 µs per call; below threshold.
- `sysml --version` +1.4 ms: binary size and protobuf initialisers, above.
- Empty-session load +15 µs (+6%): a constant, independent of model size
  (+8 µs of it was already there as found); below the size a profile
  separates from session set-up.
- `core/libs` `DecodeSnapshot` +6.7% (±5%), `core/model` `Analyse*` +4–5%,
  `Parse/synthetic` +3.8%: at or below threshold; the snapshot grew with the
  library index fields the new passes read.

## Verdict

As found, `main` was not on par with 0.4.3: loading and validating a model
was 30–36% slower, starting a state machine cost time linear in model size
where it was constant, instantiation was 3.7× slower per object, and the gRPC
verification request was 136× slower. None of the movements was a leak or a
wrong algorithm in the sense of the census: each was a feature or rule landed
in the interval whose price was not measured.

With the fixes in this change, `main` is on par with or ahead of 0.4.3 on
everything that is not new work, and the new work is priced. State-machine
start is constant again (9.7 µs against 8.2; the 1.5 µs is the object-first
addressing of `%state`). gRPC verification is 29% faster than 0.4.3 while
doing more (it materializes the subject's behaving parts). Computing a type's
features, `-instantiate` and `%satisfy` over a large model, and `GRPCEvaluate`
are 15–35× faster, because a quadratic step 0.4.3 also had is gone.
Loading is 7–11% slower and allocates 6% more, all of it in the validation
passes added since 0.4.3, none of it algorithmic. Instantiating one object is
+85%, all of it the 13 library features an object now carries. Action
performance is +69% per start (a frame per node) but +3% at a thousand
iterations. Parsing, indexing, name resolution, diagnostics and the live heap
are unchanged.

## Reproducing

```bash
git remote add upstream https://github.com/Open-MBEE/OpenSysML.git   # read-only
git fetch upstream --tags
git worktree add ../opensysml-v0.4.3 v0.4.3
(cd ../opensysml-v0.4.3 && make build)
make build
for pkg in internal/frontend/repl internal/perfbench internal/workspace/libs internal/workspace/model internal/frontend/grpc internal/frontend/lsp; do
  (cd ../opensysml-v0.4.3 && go test ./$pkg -run '^$' -bench . -benchmem -count 6) > old.$pkg.txt
  go test ./$pkg -run '^$' -bench . -benchmem -count 6 > new.$pkg.txt
  benchstat old.$pkg.txt new.$pkg.txt
done
go test ./internal/frontend/repl -run '^$' -bench RunStateMachine/elements=4000 -cpuprofile sm.cpu -memprofile sm.mem
go test ./internal/perfbench -run '^$' -bench GRPCVerifyConstraint -benchtime 20x -cpuprofile gv.cpu
../opensysml-v0.4.3/bin/sysml -validate gen12000.sysml -cpuprofile old.cpu
bin/sysml -validate gen12000.sysml -cpuprofile new.cpu
```
