# The PSSM test suite as an advisory referee

> **Labels.** Rows such as *SM7* or *SM15* are the rows of the semantic map in
> [precise-semantics alignment](../internals/design/precise-semantics-alignment.md), which
> defines each one; this record cites them so a test's verdict can be read against the row it
> reports on. Test names such as *Deferred 004 A* are the suite's own.

`cmd/pssm-referee` runs the OMG *Precise Semantics of UML State Machines* (PSSM) test suite,
translated by rule into SysML v2 textual notation, against this runtime, and files every test in
one of four buckets. It is advisory and opt-in: CI compares the committed **bucket counts**,
never a pass/fail verdict, and a movement in any count is adjudicated in the change that moves
it, exactly as the [pilot corpora](pilot-corpora.md) ratchet and the
[pilot execution referee](pilot-execution-referee.md) are.

**What a pass means.** A pass checks that the runtime reproduces UML behavior where the model
has a defensible SysML v2 mapping, provides a second opinion on the tool-choice rows, and is
never evidence of SysML v2 conformance. The suite is a UML artefact; where SysML v2 or the
Kernel Semantic Library say otherwise, the runtime follows them and the disagreement is
recorded as v2's, not as a failure. Used this way the suite is a second opinion on the ten
*differs, v2 silent* rows of the alignment note and a regression oracle for the rows on which
UML and v2 agree; it is never a conformance statement about SysML v2. The tool's `-h` opens with
that sentence.

## The suite, pinned

| | |
|---|---|
| Document | OMG `ptc/18-11-06`, *PSSM Test Suite*, version 1.0 — the machine-readable part of PSSM 1.0, published beside the specification at <https://www.omg.org/spec/PSSM/1.0> |
| File | `PSSM_TestSuite.xmi`, a UML 2.5 XMI file of 103 tests, each a UML class specializing the suite's abstract `SemanticTest` with a `Tester` that sends `Start` to a `Target` whose classifier behavior is the state machine under test |
| URL | `https://www.omg.org/spec/PSSM/20181101/PSSM_TestSuite.xmi` |
| SHA-256 | `c355b249c356774377a46b60345019d827af1ce417bde88e533aa5f39206ae07` |
| Pin | `scripts/pssm-pin.sh` (`PSSM_DOCUMENT`, `PSSM_VERSION`, `PSSM_URL`, `PSSM_SHA256`) |
| Download | `./scripts/download-pssm-suite.sh`, into the git-ignored `build/pssm/`, idempotently; a file whose digest is not the pinned one is discarded, and the referee refuses to read a suite whose digest is not the pinned one |

The suite's `href`s reach only the UML, fUML, Alf and PrimitiveTypes metamodels and profiles;
the PSSM syntax and semantics documents (`ptc/18-11-04`, `-05`) are not referenced, so only the
suite file is fetched.

**Licence.** PSSM's front matter grants a limited licence to use, copy and distribute the
specification, of which the suite is a machine-readable part, for informational use, without
modification and without commercial resale. This repository proceeds on the same reading the
OMG pilot corpora use: the suite is downloaded at a pinned URL and checksum into an ignored
directory at build time, translated **in memory**, and only bucket counts and per-test verdicts
are committed. No XMI, no excerpt and no derived `.sysml` model of a PSSM test is in the
repository; `-keep <dir>` writes the translated models for debugging only, into a directory the
caller names. This is open decision 6 of the alignment note, taken as its lean.

## Reading and classifying

`internal/pssm` reads the suite's UML subset — state machines, regions, vertices of every kind
(states, initial, final, junction, choice, fork, join, shallow and deep history, entry and exit
points, terminate), transitions with their kind, triggers, guards and effects, signal and call
events, the opaque and activity behaviors whose bodies are `trace("…")` calls, the `Tester` and
`Target` classes with the tester's stimulation sequence, and each test's expected trace or
traces. The reader is unit-tested against XMI fragments written for the purpose, never against
the suite; the pinned suite reads with zero diagnostics, and `TestSuiteClassification` pins the
per-area figures below when it is present.

Each test's state machine is classified by the UML constructs it uses, in the order of the
note's [construct-to-notation table](../internals/design/precise-semantics-alignment.md#which-uml-construct-maps-to-which-notation):

| Class | Meaning | Count |
|---|---|---:|
| **standard** | every construct has a spelling in standard SysML v2 notation | 34 |
| **extension** | spellable with this project's state-body extensions (`fork`, `join`, `junction`, `choice`, `history`, `defer`) | 31 |
| **not-expressible** | uses a construct with no spelling (entry and exit points, local and internal transitions, state-machine redefinition), a behavior shape the notation cannot bind, or a shape this project's lowerer refuses | 38 |

A test using any construct with no spelling or no translation is not expressible whatever else
it uses; otherwise the extensions win over standard. A terminate pseudostate is standard
notation (a terminate action usage, SysML v2 §7.18.3) and decides nothing; the three tests
that reach one were a class of their own, **terminate-gap**, while the runtime parsed and
lowered `terminate` without executing it (alignment finding 1, fixed), and are standard since.
The alignment note was first written with a hand count of 37 / 33 / 3 / 30; the classifier is
the record from now on, and the note's test-suite section carries its figures. Nine tests moved
from the hand count when the emitter was written, two of them moved back when the lowerer
learned to accept a fork-entered region, and a tenth moved when its failure was adjudicated;
each is listed with its reason in the note under
[Moves from the hand count](../internals/design/precise-semantics-alignment.md#moves-from-the-hand-count):

- **Entry, exit or do behaviors with parameters** that read the triggering event's data:
  *Event 017-B*, *Event 019-B*, *Event 019-C*, *Event 019-E*. The notation binds event data on
  the transition (`accept d : Data`), never on an `entry`, `exit` or `do` action.
- **An operation the tester calls and whose result it traces**: *Event 019-D*, *Event 019-E*,
  *Deferred 007*. The runtime's call events carry nothing back to the caller, and only the
  target's behaviors write the model's `log`.
- **A `trace(...)` in the tester's own behavior**: *Event 019-A* (and *019-D*, *019-E*).
- **A guard whose behavior acts on the model**: *Choice 005*, whose four guards each
  `trace("T1.n(guard)")` before returning, and whose admitted trace records the calls. A v2
  guard is a Boolean expression (`bool guard[*]` in `TransitionPerformances.kerml`, the effect a
  separate `step`), and UML 2.5.1 §14.5.11 itself calls a guard with a side effect ill formed;
  the translation keeps the guard's value and cannot reach the trace, so the emitter refuses
  it rather than run the test short (`classify.go:guardSideEffect`, `TestClassifyGuardSideEffect`,
  `TestEmitRejects`).
- **A fork into orthogonal regions that have no initial pseudostate**: *Fork 002*, *Join 001*
  — kept apart from the rest while the lowerer refused the shape, and translated since it
  accepts it, see [Findings about our own conformance](#findings-about-our-own-conformance).

## Translating

The emitter (`internal/pssm/emit.go`) produces one in-memory SysML v2 model per expressible
test, following the note's table and its worked example:

- The state machine becomes a state usage `M` in a package named for the test, with a `String`
  attribute `log` (and the target class's own attributes); every `trace("<segment>")` in an
  entry, exit, do or effect behavior becomes an append to `log`, so the final `log` is the
  `::`-joined trace PSSM compares.
- Regions become nested state bodies; an orthogonal state's regions become a `parallel` body's
  substates. Every state and pseudostate is named by its path as a bare identifier
  (`S1_S1_1`), since pseudostate declarations take no quoted name.
- The tester's `Start` and its follow-up sends become the run's queued events, in the tester's
  order, with a signal's scalar payload bound on the accepting transition's parameter. A guard
  on a choice or junction that reads the payload of the event that reached it is served by an
  attribute the triggered transition stores the payload in (UML 14.2.3.8.5).
- An initial transition whose target is a pseudostate starts the region in an empty helper
  state whose completion transition reaches that pseudostate, since a region cannot start in a
  pseudostate.
- A transition to a final state targets `done` and is declared in that final state's region.
- A terminate pseudostate is emitted as a terminate action usage declared in its region
  (`action S1_Terminate1 terminate;`), and a transition into it targets that usage by name
  (`then S1_Terminate1;`): SysML v2 §7.18.3's `accept Abort via commPort then stop; action
  stop terminate;`, the one spelling the notation gives a transition that ends the machine.
  The runtime ends the state-machine performance there — the transition's source is exited
  and its effect run, then nothing else is exited and running do behaviors are abandoned
  (`state_route.go:terminateAt`, `state_executor.go:terminateMachine`) — which is PSSM
  §9.4.13's rule for the pseudostate (SM38).

`TestEmitSuite` emits every expressible test of the pinned suite and asserts that the model
parses, validates and lowers with zero diagnostics; it fails on the first construct the emitter
cannot translate exactly rather than dropping it.

## Running and comparing

For each expressible test the referee parses the emitted model, resolves the state usage `M`,
and runs it through the runtime's shared state driver (`runtime.PerformState`, the
same entry point the execution-conformance harness uses) under the `explore` schedule policy,
which replays the run once per linearization of its choice points. The set of `log` values
reachable is compared with the test's set of admitted traces **as sets, in both directions**: a
reachable trace the suite does not admit is a failure naming that trace, an admitted trace the
runtime never reaches is a failure naming it. A run that ends in a typed runtime error, or that
exhausts the exploration budget, is a failure whose reason names the error. The exploration
budget is 4096 runs of 64 draws each, four times the `explore` policy's default: *Event 016 B*
enters and leaves orthogonal states nested two deep with each region firing, and the orders the
runtime draws among their entries, firings and exits reach 1152 linearizations. Each run is bounded
by the runtime's budgets and their environment overrides (`OPENSYSML_MAX_STEPS` and the others
the `sysml` command honors), except that the step budget defaults to 100 000 rather than the
runtime's ten million: a translated test that needs more is looping, and the smaller bound
reports the runaway in a second rather than minutes. `-jobs n` explores
`n` linearizations of one test at once; the report is byte-identical for any `n`, and
`TestRefereeDeterministic` asserts it.

### Buckets

| Bucket | Assigned when |
|---|---|
| `pass` | the test is expressible and the reachable set equals the admitted set |
| `fail` | the test is expressible and the sets differ, or the run errored or exhausted its budget; the reason names every extra and missing trace or the error |
| `not-expressible` | the classifier found a construct with no spelling or no translation; the reason names it |
| `differs-by-design` | the test would be a `fail`, **and** the committed table `internal/pssm/rows.go:TestRows` maps it to a note row whose verdict is *differs because v2 differs* |

A fifth bucket, `terminate-gap`, held the three tests that reach a terminate pseudostate
while the runtime parsed and lowered `terminate` without executing it. It is retired rather
than kept at zero: it named one construct's one missing execution, nothing else in the
suite's vocabulary can produce it, and a future suite that reaches a construct the runtime
accepts but cannot run is a `fail` naming the run's error, which is what the bucket stood in
for.

`differs-by-design` is never inferred from a failure: the table is written by hand from the
note, names the row, and is reviewed with every change to it. A test the table maps to a
*differs, v2 silent* row stays a `fail` when it fails and cites the row either way, so the bucket
lists say which failures are second opinions on a tool choice. Of the six *differs because v2
differs* rows, only SM15 (a do activity and the machine competing for one occurrence) is
reached by an expressible test; SM36 and SM37 (local and internal transitions) have no
spelling and their tests are not expressible, and A12, C6 and C9 are action and composite-structure
rows no state-machine test exercises. Of the ten *differs, v2 silent* rows, six are reached
by expressible tests and cited below: SM7, SM11, SM28 and SM30 by passes, since the rules the
note decided on them landed, and SM32 and SM34 by failures, the second opinions the suite
gives on those two tool choices; SM9 (when a completion guard is read) no failing test
reaches, and SM45 (destroying a performing object), C3 (multi-valued connector ends) and C8 (a
send with no receiver) are not state-machine rows and no test in the suite reaches them.

## Baseline

Recorded **2026-09-17** on develop commit **`c2bffb389`** with `terminate` executing
(alignment finding 1, SM38), the fork-entered-region fix
(finding 6), the active-ancestor fix, the completion-choice fix, the guard-side-effect
classification, the join incoming-effects fix, the junction branch-choice fix (finding 8) and
the segment-effect fix (finding 10) described below, and with every remaining failure
attributed, as
`docs/project/pssm-referee-baseline.json`; regenerate with `go run ./cmd/pssm-referee -update`,
check with `-check`. The counts are the gate; the rows are for whoever adjudicates a moved count.
The figures below are as measured when this record was last updated and are not the current
baseline — `go run ./cmd/pssm-referee` prints the current ones.

| Bucket | Tests |
|---|---:|
| `pass` | 46 |
| `fail` | 17 |
| `not-expressible` | 38 |
| `differs-by-design` | 2 |
| **Total** | **103** |

### Movements since the previous baseline

Three counts moved since the previous baseline (develop `265045be5` with the segment-effect
fix, 2026-09-16): the runtime executes `terminate` in every position the parser accepts
(alignment finding 1, SM38), so the three tests held in `terminate-gap` are translated and
run, the bucket is retired, and each lands where its traces put it. No other row's bucket,
reasons or reached set changed.

| Test | Finding | Movement | Adjudication |
|---|---|---|---|
| Terminate 003 | 1 (fixed) | `terminate-gap` → `pass` | Expected. A transition from `wait` on `Start`, effect `T2(effect)`, into `S1.Terminate1`, a terminate pseudostate inside the composite state `S1` whose entry is `S1(entry)`: reached `wait(exit)::T2(effect)::S1(entry)`, the one trace PSSM admits — the source is exited, the effect run, `S1` entered down to the pseudostate's owner, and the machine ends there with `S1`'s region never started (`state_terminate_entering_composite` is the conformance fixture of the shape) |
| Terminate 001 | 1 (fixed), then 9 | `terminate-gap` → `fail` | Expected in part. `S1` has two regions, `S1.1` in the first, `S2.1` in the second with a completion transition into `S1.Terminate1`. Reached `S1(entry)::S1.1(entry)::S2.1(entry)::S2.1(exit)`, which PSSM admits: the completion transition exits its source, and the machine ends with `S1.1` still active and never exited. The admitted trace not reached, `S1(entry)::S2.1(entry)::S1.1(entry)::S2.1(exit)`, enters the second region before the first — finding 9's region-entry site, the one *Entering 010* and *Junction 005* fail on; the termination itself is complete and correct in the trace reached. `fail` citing finding 9, as they do |
| Terminate 002 | 1 (fixed), then 9 | `terminate-gap` → `fail` | Expected in part. As *Terminate 001*, with `S1.1` given a do activity that traces `S1.1(doActivityPartI)` and then `S1.1(doActivityPartII)`. Reached `S1(entry)::S1.1(entry)::S2.1(entry)::S1.1(doActivityPartI)`, which PSSM admits: the do activity's first segment runs, the completion transition fires and the machine ends, aborting the do activity before its second segment — which no reached or admitted trace shows, the test's whole point. The four admitted traces not reached — `S1(entry)::S1.1(entry)::S2.1(entry)` and `S1(entry)::S1.1(entry)::S1.1(doActivityPartI)::S2.1(entry)`, and the two with `S2.1(entry)` before `S1.1(entry)` — vary the region-entry order and the do step's place against the dispatch of `S2.1`'s completion (before it, or not at all): finding 9's region-entry and do-step sites, the ones *Entering 010* and *Behavior 003 A* fail on. Note the run's trace ends on `S1.1(doActivityPartI)` with no `S2.1(exit)`: PSSM's expected traces for this test end there too, so the source's exit is not traced by the model. `fail` citing finding 9 |

### Movements before that

No count moved between the baseline of develop `46828f14f` (with the failures
attributed, 2026-09-16) and the one that followed it (develop `265045be5`, 2026-09-16). One reason did: the runtime fix of
[finding 10](#findings-about-our-own-conformance) — a segment leaving a junction or choice
declared inside a composite state runs its effect after that state's entry — took *Junction
005* off the inadmissible trace it reached and onto an admitted one, and the test stays `fail`
on what remains missing.

| Test | Finding | Movement | Adjudication |
|---|---|---|---|
| Junction 005 | 10, then 9 | `fail` → `fail`, reason changed | Expected in part. The reached trace is now `S1(entry)::T1.3(effect)::T2.1(effect)::S2.1(entry)::S1.2(exit)::S1(exit)`, one of the three PSSM admits — `S1(entry)` first, the fix's whole effect — where it was `T1.3(effect)::S1(entry)::…`, which PSSM does not admit. The two admitted traces still not reached, `S1(entry)::T2.1(effect)::S2.1(entry)::T1.3(effect)::…` and `S1(entry)::T2.1(effect)::T1.3(effect)::S2.1(entry)::…`, put the second region's initial-transition effect and entry before or around the first region's junction segment: the order in which `S1`'s two regions are entered, which the runtime fixes in declaration order and records no choice for — finding 9's region-entry site, the one *Entering 010* and *Entering 011* fail on. The test was expected to move to `pass` on finding 10 alone; it does not, because its admitted set interleaves the two regions, and its row now cites finding 9 |

### Movements before that

No count moved between the baseline of develop `bcc6b13e0` (with the junction
branch-choice fix, 2026-09-15) and the one that followed it (develop `46828f14f`, 2026-09-16).
The baseline file changed all the same: the adjudication of
the fourteen failures it left unattributed ([below](#fail-17)) added four tests to the
committed table — *Junction 004* and *Join003* on SM32, *Join001* and *Transition 019* on
SM34, both *differs, v2 silent* rows — so their rows now carry the row and its verdict, and
their reasons the *reports on* line. All four stay `fail`, as a test mapped to a tool-choice
row does; none was inferred from the run, each was read against its requirement, and the
SM32 and SM34 rows of the alignment note quote what each test shows.

### Movements before that

The baseline with the failures attributed followed one (develop `e6218449a` with the join
incoming-effects fix, 2026-09-15) that
counted 44 `pass` and 16 `fail`. One change moved it: the runtime fix of
[finding 8](#findings-about-our-own-conformance) — a junction with several enabled outgoing
branches is a choice point, as a choice with several is, where the runtime took the first in
declaration order. One test moved `fail` → `pass` and none the other way.

| Test | Row | Movement | Adjudication |
|---|---|---|---|
| Junction 003 | finding 8 | `fail` → `pass` | Runtime defect: the finding's case. `T1.3` and `T1.4` both leave `Junction1` unguarded, PSSM admits one trace per branch and demands neither; the run explored took `T1.3` only, so the `T1.4` path was never reached. Both are now drawn at the junction (`explore` enumerates the two runs), and the emitter had preserved both branches and their effects all along — a runtime defect, not a translation one |

### Movements before that

The baseline with the join incoming-effects fix followed one (develop `e6218449a` with the
completion-choice fix and the guard classification, 2026-09-15) that counted 43 `pass` and 17
`fail`. One change moved it: a join now fires every transition into it — each source exited, then that
segment's effect, in an order the scheduling policy draws — before the state owning the join is
exited and the outgoing segment's effect runs (runtime, `state_executor.go:fireJoinIncoming`; the
SM34 row of the alignment note records the rule). One test moved `fail` → `pass` and none the
other way.

| Test | Row | Movement | Adjudication |
|---|---|---|---|
| Join002 | SM34 | `fail` → `pass` | Runtime defect. `T1.2` and `T2.2` leave `S1`'s two regions for the join `Join1`; `T3` leaves it for `S2`. The run exited both regions and `S1`, ran only `T2.2`'s effect (the incoming segment that completed the join) and dropped `T1.2`'s — `S1(exit)::T2.2(effect)::T3(effect)::S2(entry)`, an execution neither UML nor SysML v2 admits: PSSM §8.5.7 (`JoinPseudostateActivation`) and requirement *Join 002* make the transitions into a join and the one out of it segments of one compound transition, every one of which is traversed, and the bundled library's `TransitionPerformances.kerml` orders each fired transition's effect after its own source's performance (`succession [1] transitionLinkSource then [*] effect`), never dropping one. Each incoming segment now fires whole — its source exited, then its effect — before `S1(exit)`, in either order; both admitted traces are reached (`T1.2(effect)::T2.2(effect)::…` under the default policy, the swapped order under `seed-1`). `state_join_runs_every_incoming_effect` pins the two orders as an admissible set with an oracle section |

The same change altered the recorded reason, without moving the bucket, of *Join001* (`T2.3(effect)`,
the incoming segment it dropped, now runs; what the run still gets wrong is the place of
`S1(exit)`: it exits `S1` after the last incoming segment's effect, where PSSM's
`exitSource` leaves the composite whose last region that segment empties before the
segment's effect) and of *Transition 019* (both `T1.3(effect)` and `T2.3(effect)` now run; the
two traces the suite does not admit are the ones where the join's segments fire in the order
opposite to the one the regions fired `Continue` in — PSSM fires each incoming segment on its own
source's completion, so their order follows the sources' order, where the runtime draws it
afresh). Their rows below quote the new reasons.

The baseline with the completion-choice fix and the guard classification followed one (develop
`e6218449a` with the active-ancestor fix, 2026-09-15) that counted 42
`pass`, 19 `fail` and 37 `not-expressible`. Two changes moved it: a runtime change (a state's
completion is one occurrence, so several enabled completion transitions out of one state are
one transition choice drawn when the completion is dispatched — SM19; `chooseCompletion`,
`state_explore_completion_choice` — where each used to be queued as its own event and the first
declared always fired) and a classifier rule (a guard whose behavior does more than return its
value has no translation). One test moved `fail` → `pass`, one `fail` → `not-expressible`, and
none the other way.

| Test | Row | Movement | Adjudication |
|---|---|---|---|
| Event 015 | SM19 | `fail` → `pass` | Expected: `S1.1`'s two completion transitions `T1.2` and `T1.3` are in conflict (§9.3.4.11) and either may fire; the run reached only the `T1.2` trace because the second completion event went stale once `S1.1` was left, and reported no choice, so `explore` had nothing to enumerate. Both traces are now reached, as two outcomes of one choice point |
| Choice 005 | the classifier | `fail` → `not-expressible` | A missing translation, not a defect of the runtime: the test's four guards each `trace("T1.n(guard)")` before returning their value, to show when a junction's and a choice's guards are read, and the admitted trace lists the four calls. The emitter carried each guard as its Boolean body alone and silently dropped the behavior, so the run reached `T2(effect)::S1(entry)::S1.1(entry)` — the admitted trace less the guard segments, the route itself right. A v2 guard is an expression with no spelling for an action, so the classifier now names the construct (*guard side effect*) and the emitter refuses it; see [Moves from the hand count](../internals/design/precise-semantics-alignment.md#moves-from-the-hand-count) |

### Movements before that

The baseline with the active-ancestor fix followed one (develop `f2193d764`, 2026-09-14) that
counted 41 `pass` and 20 `fail`. One change moved it: a transition from a substate into the composite state enclosing it no longer
re-enters that state (runtime, `state_executor.go:completeInto`; the SM35 row of the alignment
note records the rule). One test moved `fail` → `pass` and none the other way.

| Test | Row | Movement | Adjudication |
|---|---|---|---|
| Transition 011 C | SM35 | `fail` → `pass` | Runtime defect. `T1.3` leaves `S1.2` for `S1`, the composite enclosing it and already active. The run exited `S1.1` and `S1.2`, ran the effect, then started `S1`'s default substate again without exiting or entering `S1` — an entry sequence neither UML nor SysML v2 admits: UML's external transition either exits and re-enters `S1` or, as PSSM §8.5.8 reads it, does not enter a target that is already active and completes the source's region instead ("the RegionActivation owning the sourceVertexActivation completes"); SysML v2 §7.18.3 activates a target on the transition and nowhere restarts an active one. The runtime now follows PSSM: `S1`'s region is complete once `S1.2` is left, `S1` completes, `T2` fires and `S1(exit)` is logged, the admitted trace. `state_transition_into_active_ancestor`, `state_transition_into_active_parallel_ancestor` pin it |

The baseline of develop `f2193d764` followed one (develop `f4b844329`, 2026-09-14) that counted
18 `fail` and 39 `not-expressible`. One change moved it: the lowerer accepts an orthogonal region with no entry
transition when a fork's branch enters it ([finding 6](#findings-about-our-own-conformance)),
so the two tests the classifier filed under *lowerer refuses fork into a region without an
entry transition* translate and run. Both moved `not-expressible` → `fail`; nothing else moved,
and the eighteen other failures' reasons are byte-identical to the previous baseline's. Three
`not-expressible` tests that also carried the reason stay where they are on their other grounds:
*Transition 023* and *Standalone 002* drop it (their forks now enter their regions), *Entry 002 E*
keeps the renamed reason (its regions have neither an entry transition nor a fork branch).

| Test | Row | Movement | Adjudication |
|---|---|---|---|
| Fork 002 | finding 6, SM22 | `not-expressible` → `fail` | Expected: the fork's branches now enter `S1.1.1` and `S1.2.1` in `S1.1`'s two regions and the run reaches `T2(effect)::S1(entry)::T2.1(effect)::S1.1(entry)::T2.2(effect)`, one of the four traces PSSM admits. The three not reached are the other interleavings of the two branches' effects and entries; the branches are entered in the regions' declaration order and no choice point records the order, which is SM22's open decision — the same family as *Entering 010* and *Entering 011* |
| Join001 | finding 6 | `not-expressible` → `fail` | Expected to run, not to pass: the fork's branches enter `S2.1` and `S1.1` and the run reaches `S2.1(exit)::S1.1(exit)::S1(exit)::T2.4(effect)`, which PSSM does not admit — `T2.3(effect)`, the effect of the join's incoming transition from `S1.1`, never runs. The same drop as *Join002*'s missing `T1.2(effect)`: the join fires the outgoing transition after the sources' exits and runs only one incoming effect. A runtime defect of the join, not of the fork entry; it is adjudicated with *Join002* below |

The baseline of develop `f4b844329` followed one (develop `bb95cf226`, 2026-09-12) that counted
36 `pass` and 23 `fail`. Two changes moved it: the runtime fix of [finding 7](#findings-about-our-own-conformance) (a
transition into a history reads the record after its exits have run, and a history declared in
the machine's own body restores the machine's configuration) and a translation fix in the
emitter (an initial transition's effect used to be folded into the entry action of the state or
region it starts, so it ran before the state's own `entry` and ran again on every entry, a
history restore included; it is now the completion transition of an empty helper state the
initial transition enters, `emit_test.go:TestEmitInitialWithEffect`). Five tests moved `fail` →
`pass` and none the other way.

| Test | Row | Movement | Adjudication |
|---|---|---|---|
| History 001-A, History 002-D | finding 7 | `fail` → `pass` | Expected: the finding's two cases. `S1`'s self-transition now restores `S1.1.2`; its completion transition into its own shallow history finds no record (the owner's exit cleared it) and takes the default transition `T1.3` after `S1(entry)` |
| History 001-D | finding 7 | `fail` → `pass` | Expected: the test declares `DeepHistory1` in the machine's own region, restoring the machine's top-level configuration, which the runtime refused as a history outside any composite state; the machine's body is now a valid owner |
| History 002-A | finding 7 | `fail` → `pass` | Expected: `S1`'s self-transition `T3` into its shallow history read the record before `S1` was left, found none (`S1` had never been left) and performed a default entry through `S1.1`, re-running `S1.1(exit)::S1.2(entry)`; it now restores `S1.2`, the substate being left |
| History 001-B | finding 7 and the emitter | `fail` → `pass` | Expected: the default transition `T1.4` now runs after `S1(entry)` (finding 7's default entry from inside the owner) and the initial transition `T1.4`'s effect no longer repeats on the revisit (the emitter fix) |

The emitter fix also altered the recorded reason, without moving the bucket, of *Entering 010*,
*Entering 011*, *Junction 004* and *Junction 005* (the repeated `T1.1(effect)` / `T2.1(effect)`
of a region's initial transition is gone; *Junction 004* now reaches no trace at all — a run
error at the junction whose guards are all false, SM32's case — where it reached an inadmissible
one), and finding 7 the reason of *History 001-C* and *History 002-B* (the restore is now the
admitted one; what remains missing are the other interleavings of the two regions' entries and
exits). A later translation fix — the values a test's constructor writes on the new instance are
now the attributes' initial values (`emit_test.go:TestEmitFactoryInitializesAttributes`) — altered
the reason of *Join003* without moving it: `value` is now `15` as the test intends, and the run
fails at the join instead of at the guard's read of an unset feature. Their rows below quote the
new reasons.

The baseline of develop `bb95cf226` followed one (develop `fb034e817`, the same day) that
counted 25 `pass` and 34 `fail`. It was
recorded before the alignment note's four decided rules — SM7 deferral, SM11 composite
completion, SM28 empty history, SM30 dynamic choice guards — landed on `develop`, and of its
thirty-four failures eight cited one of those rows or SM32. Re-running on the commit that carries
the rules moved eleven tests from `fail` to `pass` and none the other way; every movement is
adjudicated here, and the two rows the movement adds to the committed table (*Deferred 003* →
SM7, *Event 016 A* → SM11) were reviewed by hand against the tests' requirements, not inferred
from the run.

| Test | Row | Movement | Adjudication |
|---|---|---|---|
| Deferred 004 A, Deferred 004 B | SM7 | `fail` → `pass` | Expected: the deferral now outranks the sibling region's transition |
| Final001 | SM11 | `fail` → `pass` | Expected: the composite's own completion transition `T1.2` now fires |
| History 002-C | SM28 | `fail` → `pass` | Expected: a shallow history with nothing recorded and no default transition now performs the owner's default entry instead of erroring |
| Choice 001, Choice 002 | SM30 | `fail` → `pass` | Expected: the choice's guards now read what the incoming effect wrote |
| History 001-A | was SM28 | `fail` → `fail`, row withdrawn | Expected to pass, did not. Its failure was never SM28's case — `S1` is left in `S1.1.2`, so there is a configuration to restore — and SM28's landing only turned the typed error into a wrong trace: the run performs a default entry (`…::T3(effect)::S1(entry)::S1.1(entry)::S1.1.1(entry)::S1.1.2(entry)`) where PSSM restores (`…::S1(entry)::S1.1(entry)::S1.1.2(entry)`). Attributed to [finding 7](#findings-about-our-own-conformance): the history record is read before the owner is left, so a self-transition of the owner into its own history sees no record |
| Event 016 A | SM11 | `fail` → `pass` | Consequence of SM11: `S1`'s body reaching `done` used to complete the machine, so the second `Continue` found nothing to trigger; `S1` now completes and its own triggered transition `T3` fires. Added to the table on SM11 |
| Deferred 003 | SM7 | `fail` → `pass` | Consequence of SM7 and SM11: `S1.1` defers `AnotherSignal` against the machine's transition out of `S1` (SM7), and `T1.1.2`'s `done` completed the whole machine before SM11. Added to the table on SM7 |
| Choice 003, Choice 004, Junction 001 | — | `fail` → `pass` | Not a decided rule: the previous `resolveRoute` settled a route through a choice or junction to its end state and never ran the effects of the segments leaving the pseudostate (`T4`, `T1.2`), so their `log` stayed short; the route that landed with SM30 carries every segment and runs their effects (`route.segments`, `state_pseudostate_chain_*` fixtures). A defect fix that rode along with the rule, not a rule choice |

The same segment-effect change altered the recorded reason, without moving the bucket, of
*Transition 017* (now reaches one of the eight admitted interleavings, `T3.2(effect)` included,
and still none of the other seven), *Join002* (`T3(effect)` now runs), *Junction 003* (the
`T1.3` path is now reached, the `T1.4` path still not), *Junction 004* and *Junction 005*
(`T1.3(effect)`, a segment out of a junction inside `S1`, now runs — before `S1(entry)`, where
*Junction 005*'s admitted traces have it after) and *History 001-B*
(`T1.4(effect)`, the history's default transition, now runs — likewise before the entry of the
state that owns the history, where PSSM runs it after). *History 002-D*'s reason changed from a
short trace to a budget exhaustion: with SM11 its `S1` now completes and fires `T3` into its own
history, and the history-record timing of finding 7 makes that re-enter `S1.1` without end. The
remaining failures' reasons are byte-identical to the previous baseline's.

### `pass` (46)

Behavior 001, Behavior 002, Behavior 003 B, Transition 001, Transition 007, Transition 011 C,
Transition 015, Transition 016, Transition 020, Transition 022, Event 001, Event 002, Event 008, Event 009,
Event 010, Event 015, Event 016 A (reports on SM11), Event 016 B, Event 017 A, Event 018, Entering 004,
Entering 005, Exiting 002, Exiting 005, Choice 001 and Choice 002 (report on SM30), Choice 003,
Choice 004, Final001 (reports on SM11), Deferred 001, Deferred 002, Deferred 003 (reports on
SM7), Deferred 004 A and Deferred 004 B (report on SM7), Deferred 005, Deferred 006 A (reports
on SM15: the runtime's rule and PSSM's agree on this variant), History 001-A, History 001-B,
History 001-D, History 002-A, History 002-C (reports on SM28), History 002-D, Join002, Junction 001,
Junction 003.

### `differs-by-design` (2)

| Test | Row | Admitted trace not reached |
|---|---|---|
| Deferred 006 B | SM15 | `S2(doActivityPartI)::S2(doActivityPartII)` — PSSM gives the deferred occurrence to the do activity alone; the runtime dispatches it to every scope |
| Deferred 006 C | SM15 | `S1.2(doActivity)::S1.1(doActivity)` — the same rule |

### `fail` (17)

Every failure is attributed. Five cite a *differs, v2 silent* row of the alignment note through
the committed table: the suite's second opinion on a tool choice, which the row records and the
test does not overturn. Twelve cite an open finding against this project — a gap of the runtime's,
recorded [below](#findings-about-our-own-conformance) and in the alignment note, that a change
of its own will close, moving the tests with it. None is a translation defect: each translated
model was read against the test's UML, and every construct the test uses reaches the run.

#### Citing a note row (5)

| Test | Row | What the run shows |
|---|---|---|
| Junction 002 | SM32 | run error: no outgoing guard of the junction holds; PSSM disables the compound transition and admits `T3(effect)` |
| Junction 004 | SM32 | run error: `junction S1_Junction1_2: no guard evaluated to true` — the junction on the default entry of `S1`'s second region, both of whose outgoing guards are false; PSSM's static evaluation disables the transition into `S1` as a whole, `S1` is never entered and the next occurrence fires `T3` from the source, admitting `T3(effect)`. The same rule as *Junction 002*'s, one region further in; the emitter carries both guards as `false` faithfully |
| Join003 | SM32 | run error: `join Join1: no guard evaluated to true` — the join's only outgoing transition is guarded `value < 10` with `value` `15`; the runtime fires the join's segments together once both sources are active (SM34) and fails resolving the way out. PSSM fires the first completion transition into the join alone (a segment may end at a join, whose completion is waited for) and disables the second, whose entering the join would need a way through, so `S1` stays active for `T5`; admits `T1.2(effect)::T5(effect)` and `T1.4(effect)::T5(effect)`. SM32's rule at a join's way out |
| Join001 | SM34 | reached `S1.1(exit)::T2.3(effect)::S2.1(exit)::T2.4(effect)::S1(exit)` and its mirror, `S1(exit)` after the last incoming segment's effect; PSSM admits `S1.1(exit)::T2.3(effect)::S2.1(exit)::S1(exit)::T2.4(effect)` and its mirror, `S1` left with the last source, before that segment's effect. The row records the runtime's place for the owner's exit — after every incoming effect, the library reading of the oracle's join section — and the test's own prose expected execution puts `S1(exit)` there too; its assertion does not |
| Transition 019 | SM34, finding 9 | reached `S1.1(exit)::T1.2(effect)::S2.1(exit)::T2.2(effect)::T2.3(effect)::T1.3(effect)` and its mirror, the join's segments in the order opposite to the regions' firing order: PSSM fires each completion transition into `Join1` when its source's completion is dispatched, so their order follows the sources', where the runtime holds them until the join is ready and draws the order (the row). The four admitted traces not reached put both regions' exits before either effect — the steps of two firings interleaved, finding 9's granularity — while the two that interleave each region's exit and effect are reached |

#### Citing a finding (12)

One line per test, from the baseline's `reasons`: what the run reached that the suite does not
admit (`—` when every reached trace is admitted and the failure is only a missing one), and
what PSSM admits that the run never reached. Where PSSM admits several interleavings one is
quoted and the number given. The full sets are in the baseline file.

| Test | Finding | Reached, not admitted | Admitted, not reached |
|---|---|---|---|
| Behavior 003 A | 9 (do step before dispatch) | — | `S1(entry)` (the machine's `AnotherSignal` transition dispatched before the do activity's first segment; `runStep` runs the do round before the dispatch) |
| Transition 017 | 9 (do step before dispatch) | — | `T2(effect)::S1(entry)::S3.1(doActivity)::T3.1.2(effect)::T2.2(effect)::T3.2(effect)` and 6 more interleavings of `S3.1(doActivity)`, `T2.2(effect)`, `T3.1.2(effect)` (the eighth admitted order is reached) |
| Entering 010 | 9 (region entry order) | — | `S1(entry)::T2.1(effect)::S1.1(entry)::S2.1(entry)` and `S1(entry)::T2.1(effect)::S2.1(entry)::S1.1(entry)` (the third admitted order is reached) |
| Entering 011 | 9 (region entry order) | — | `S1(entry)::T1.1(effect)::S1.1(entry)::T2.1(effect)::S1.2(entry)` and 4 more orders of the two regions' initial effects and entries (the sixth admitted order is reached) |
| Exiting 001 | 9 (region exit order) | — | `S1.1.1(exit)::S2.1(exit)::S1.1(exit)::S1(exit)` and `S2.1(exit)::S1.1.1(exit)::S1.1(exit)::S1(exit)` (the third admitted order is reached) |
| Exiting 003 | 9 (region exit order) | — | `S1.2.1(exit)::S1.1.1(exit)::S1.1(exit)::S1(exit)` (the other admitted order is reached) |
| Fork 002 | 9 (region entry order) | — | `T2(effect)::S1(entry)::T2.1(effect)::T2.2(effect)::S1.1(entry)` and 2 more orders of the two branches' effects and `S1.1(entry)` (the fourth admitted order is reached) |
| History 001-C | 9 (region entry and exit order) | — | `S1(entry)::S1.1(exit)::S1.2(entry)::S2.2(entry)::S2.2.1(exit)::S2.2.2(entry)::S1(exit)::S1(entry)::S1.1(exit)::S1.2(entry)::S2.2(entry)::S2.2.2(entry)::S1(exit)` and 10 more orders of the two regions' entries and exits (the twelfth admitted order, the one the PSSM text prints, is reached) |
| History 002-B | 9 (region entry and exit order) | — | `…::S1(exit)::T3(effect)::S1(entry)::S1.1(exit)::S1.2(entry)::S2.2(entry)::S2.2.1(exit)::T2.2.2(effect)::S2.2.2(entry)::S1(exit)` and 4 more orders of the two regions' entries and exits (the sixth admitted order is reached) |
| Junction 005 | 9 (region entry order) | — | `S1(entry)::T2.1(effect)::S2.1(entry)::T1.3(effect)::S1.2(exit)::S1(exit)` and `S1(entry)::T2.1(effect)::T1.3(effect)::S2.1(entry)::S1.2(exit)::S1(exit)` (the third admitted order, `T1.3(effect)` first after `S1(entry)`, is reached: the second region's initial effect and entry are admitted before or around the junction segment's effect) |
| Terminate 001 | 9 (region entry order) | — | `S1(entry)::S2.1(entry)::S1.1(entry)::S2.1(exit)` (the other admitted order is reached; the termination — source exited, `S1.1` left active and unexited, the machine ended — is in both) |
| Terminate 002 | 9 (region entry order, do step before dispatch) | — | `S1(entry)::S1.1(entry)::S2.1(entry)` and 3 more orders of the two regions' entries and the do activity's first segment, which PSSM admits before the terminating completion transition or not at all (the fifth admitted order is reached; the do activity's second segment is in none, aborted) |

Every reason in full — each extra trace, each missing trace, each error — is in the baseline
file's `reasons`.

### `not-expressible` (38)

By reason, as the classifier names them:

- **entry point, exit point** (no spelling): Entry 002 A, Entry 002 B, Entry 002 C, Entry 002 D,
  Entry 002 E, Entry 002 F, Exit 001, Exit 002, Exit 003, Exiting 004, Entering 009, Fork 001,
  Junction 006, Transition 011 B, Transition 011 D, Transition 011 E, Transition 023,
  TransitionExecutionAlgorithm, Standalone 001, Standalone 002.
- **local transition, internal transition** (no spelling, SM36 / SM37): Behavior 004,
  Transition 010, Transition 011 A, and among the above Transition 011 B, Transition 011 D,
  Entering 009, Entry 002 B, Entry 002 C, Entry 002 F, TransitionExecutionAlgorithm.
- **redefined state machine, extended region, redefined transition** (no spelling):
  Redefinition 001 to 006.
- **standalone state machine** (the machine under test is not a `Target`'s classifier
  behavior): Standalone 001, Standalone 002, Standalone 003.
- **behavior parameter, operation result, tester trace** (no translation): Event 017 B, Event
  019 A, Event 019 B, Event 019 C, Event 019 D, Event 019 E, Deferred 007, and among the above
  Entry 002 F, Standalone 002, Standalone 003.
- **lowerer refuses an orthogonal region with neither an entry transition nor a fork branch
  into it** (ours): Entry 002 E, which is not expressible on other grounds too. Fork 002 and
  Join 001, filed here while the lowerer refused every region without an entry transition,
  translate since finding 6 was fixed.
- **guard side effect** (no translation): Choice 005.

A test with several such constructs is listed under each; the baseline file names every
test's constructs in its `reasons`.

## Findings about our own conformance

The referee's classifier and runs surfaced five gaps that are this project's rather than SysML
v2's. Four — the first two found when the referee was added, the third by its exploration, the
fourth by attributing the failures that remained — are fixed, each in a change of its own; one,
found the same way, is open, and the tests that reach it stay `fail` citing it until a change
of its own closes it:

- **The lowerer refused a fork into orthogonal regions that have no initial pseudostate**
  (*Fork 002*, *Join 001*; alignment finding 6). UML lets a fork's outgoing transitions enter
  states inside a composite state's orthogonal regions directly, with no initial pseudostate in
  those regions; SysML v2 `parallel` regions can spell the shape and this project's `fork`
  extension can spell the fork. `lower.ToStateGraph` refused it ("region `<name>` has no initial
  state; write `entry; then <state>;` inside the region") because it required every region to
  name its own start even when a fork was the only way in. The two tests were filed
  `not-expressible` under the distinct reason *lowerer refuses fork into a region without an
  entry transition* so they were never confused with the constructs v2 has no spelling for.
  Fixed: `lower/fork_plan.go` reads each fork's branches into a `ForkPlan` — one target state
  per orthogonal region of one composite state — and the lowerer accepts a region a fork enters
  without an entry transition of its own, still refusing one with neither (the classifier's
  reason is now *lowerer refuses an orthogonal region with neither an entry transition nor a
  fork branch into it*, which only *Entry 002 E* still carries, and it reads a branch's region
  the way the lowerer does — the region of the fork's composite the target lies in, however
  deep); the runtime leaves the source configuration — every region of the composite when
  the fork is reached from inside them — runs the first branch's effect, enters the rest of the
  way down to the composite, then enters every region in declaration order, each branch's
  effect before its target (`state_executor.go:leaveForFork`,
  `state_region_entry.go:enterForkBranches`; `state_fork_enters_regions_without_initial`,
  `state_fork_in_composite_enters_parallel_substate`,
  `state_fork_omitted_region_declared_first` and `state_fork_from_within_owner_regions` with
  their trace goldens). The two tests
  translate and run; where each landed is in the movements table above.
- **A transition from a composite state into its own history pseudostate read the record
  before the state was left** (*History 001-A*, *History 002-D*; alignment finding 7). The
  configuration a history restores is recorded when its owner is exited
  (`exitState` → `recordChildHistory`, `recordRegionHistory`), but `resolveRoute` and
  `historyEntry` read it before the transition's exits ran, so a transition whose source was the
  owner itself saw the owner's *previous* exit, not the configuration it was leaving: *History
  001-A*'s self-transition found no record and performed a default entry where PSSM restores
  `S1.1.2`; *History 002-D*'s completion transition found the record `S1.1`'s exit left, which
  the owner's exit would have cleared, so the history's default transition was skipped and `S1.1`
  re-entered, whose completion fired the transition again until the step budget was exhausted.
  UML restores the most recent active configuration — the one being left. Fixed:
  `fireHistoryTransition` → `moveToHistory` runs the exits and effects before `historyEntry`
  reads the record, a default transition is taken from inside the owner once entered, and a
  history in the machine's own body restores the machine's configuration
  (`state_deep_history_self_transition`, `state_shallow_history_completion_default`,
  `state_machine_body_deep_history`). The two tests pass, and *History 001-B*, *001-D* and
  *002-A* with them; the movements table above adjudicates each.
- **A junction with several enabled outgoing branches took the first in declaration order**
  (*Junction 003*; alignment finding 8). `pseudostateBranch` stopped at the first junction guard
  that held, so where PSSM admits one trace per enabled branch — "one is chosen; the algorithm
  for making this selection is not defined" — and the referee explores every schedule to reach
  each, the run reached the `T1.3` path only and never `T1.4`'s. The project's own choice
  vertex already draws one of several enabled branches as a choice point (SM30), and KerML's
  `DecisionPerformance` ranks none of a branching point's successions; the junction differed
  from the choice in *when* its guards are read, never in how many may hold. Fixed:
  `state_route.go:enabledBranches` reads every outgoing guard at the junction's static instant
  and several enabled leave the route open at the junction, where `settleDraws` → `pickBranch`
  makes the transition choice point drawn by the schedule policy only as the transition fires,
  after the region order among several candidates and the transition's own guard read again
  (`state_junction_several_enabled_branches`, `state_junction_drawn_as_its_transition_fires`,
  `state_history_default_through_junction`, `explore_test.go:TestExploreStaticJunctionBranches`).
  The test passes; the movements table above adjudicates it.
- **The order in which orthogonal regions are entered, exited and stepped is not a recorded
  choice point** (*Entering 010*, *Entering 011*, *Exiting 001*, *Exiting 003*, *Fork 002*,
  *History 001-C*, *History 002-B*, *Transition 019*, *Behavior 003 A*, *Transition 017*,
  *Junction 005* since finding 10's fix, and *Terminate 001* and *Terminate 002* since the
  runtime executes `terminate`; alignment finding 9, open). Every trace the runtime
  reaches in these thirteen is one the suite
  admits; what fails is the admitted traces it never reaches, because four sites order what PSSM
  leaves concurrent and record no choice for `explore` to vary: the regions of a composite
  state and a fork's branches are entered in declaration order
  (`state_region_entry.go:enterRegionsInto`, `enterForkBranches`), exited in declaration order
  (`state_executor.go:exitState`), the transitions one occurrence selects fire one whole firing
  — source exit and effect — at a time (`dispatchInOrder`, SM21), and a do action due at the
  instant steps before the occurrence at the head of the pool is dispatched (`runStep` runs the
  do round first, SM13). Not a defect of behavior — SysML v2 §7.18.1 leaves parallel substates
  and a `do` sub-performance concurrent, so the runtime's one linearization per site is one v2
  admits — but a gap of exploration, and its fix is a scheduling design (each site stepped under
  a draw the policy makes and a `ChoiceRegionOrder` records, as `dispatchInOrder` and
  `runDoRound` already do among themselves, with the trace goldens of every fixture that enters
  or leaves an orthogonal state moving), so it is recorded rather than made here. The design is
  written — [recording the order of orthogonal regions](../internals/design/region-order-scheduling.md):
  the unit each site draws, the choice kinds and their trace and witness lines, what each policy
  does, the rollback of a refused replay, the budget, and the alignment row on firing
  granularity — and stops at two decisions for the maintainers: whether trace goldens recorded
  under the default policy may gain `choice` lines, and the two admitted traces of
  *Transition 017* that no reading of the model produces.
- **A segment leaving a junction inside a composite state ran its effect before the composite
  was entered** (*Junction 005*; alignment finding 10). The transition targets a junction in one
  region of the orthogonal `S1`, and `state_executor.go:moveTo` ran every effect of the route
  after the exits and before `enterBelow` entered the way down to the target, so the segment
  `T1.3`, declared in `S1`'s body, logged `T1.3(effect)` before `S1(entry)`; PSSM enters `S1`
  on the way to the junction and admits only orders with `S1(entry)` first, and v2 reads the
  segment as an `enclosedPerformance` of `S1`, after its `entry` — the reading finding 7's fix
  already applies to a history's default transition. A runtime defect. Fixed: a route's effects
  carry the state declaring the pseudostate each segment leaves (`state_route.go:routeEffect`,
  from the lowered `PseudostateOwner`), and `runEffects` enters the states down to that owner
  before running each (`enterAhead`; the move then finds them entered), for a junction as for a
  choice — whose guards are read once its own owner is entered (`enterOwnerOf`) and only the
  states every branch enters are in (`certainEntries`) — at every depth, in a region of a
  parallel state, and on a history's
  default transition (`state_junction_inside_composite`, `state_choice_inside_composite`,
  `state_choice_guard_reads_owner_entry`,
  `state_junction_inside_nested_composite`, `state_junction_inside_orthogonal_region`,
  `state_junction_then_choice_inside_composite`,
  `state_history_default_junction_inside_nested`, each with its trace golden; no golden on
  `develop` moved). *Junction 005* now reaches an admitted trace and stays `fail` on the two it
  still misses, which are finding 9's region-entry order; the movements table above
  adjudicates it.

### The twenty failures the first baseline left unadjudicated

The baseline of develop `bb95cf226` (36 `pass`, 23 `fail`) tabled twenty failures as
unadjudicated. Each has since been attributed to exactly one of a translation defect (the
emitter lost or misspelled a construct), a runtime defect (the extra or missing trace is
behavior UML and SysML v2 both forbid) or a missing alignment row (v2 legitimately differs, or
is silent where the project made a choice, and the note had no row or no text for it), in the
change that moved or attributed it; the movements tables above and the `fail` rows carry the
detail. By root cause:

| Root cause | Attribution | Tests | Where each landed |
|---|---|---|---|
| History record read before the owner is left | runtime defect, finding 7 (fixed) | History 001-B, History 001-D, History 002-A | `pass` |
| An initial transition's effect folded into the entry action it starts | translation defect, the emitter (fixed) | none on its own: it was *History 001-B*'s second defect, and altered the reasons of *Entering 010*, *Entering 011*, *Junction 004* and *Junction 005* | — |
| A join dropped every incoming effect but the last | runtime defect, SM34 (fixed) | Join002 | `pass` |
| A junction with several enabled branches took the first | runtime defect, finding 8 (fixed) | Junction 003 | `pass` |
| A transition into an active ancestor re-entered it | runtime defect, SM35 (fixed) | Transition 011 C | `pass` |
| Several completion transitions out of one state were not one choice | runtime defect, SM19 (fixed) | Event 015 | `pass` |
| A guard whose behavior acts on the model | translation defect, the classifier (a construct with no translation) | Choice 005 | `not-expressible` |
| A junction or join with no way through | missing alignment text, SM32 (*differs, v2 silent*) | Junction 004, Join003 | `fail`, citing SM32 |
| The order of a join's segments | missing alignment text, SM34 (*differs, v2 silent*) | Transition 019 (also finding 9) | `fail`, citing SM34 |
| Region entry, exit and do-step order is not a recorded choice | runtime gap, finding 9 (open) | Entering 010, Entering 011, Exiting 001, Exiting 003, History 001-C, History 002-B, Behavior 003 A, Transition 017 | `fail`, citing finding 9 |
| A junction segment's effect before its owner's entry | runtime defect, finding 10 (fixed) | Junction 005 | `fail`, citing finding 9 for the region-entry orders left missing once the segment's effect follows `S1(entry)` |

*Fork 002* and *Join001*, which finding 6's fix brought out of `not-expressible` after that
baseline, are attributed with them: *Fork 002* to finding 9 (region entry order) and *Join001*
to SM34 (where the owner is left), each read against its requirement. So are *Terminate 001*
and *Terminate 002*, which executing `terminate` brought out of `terminate-gap`: both to
finding 9 (region entry order; for *002* the do step's place too), the termination itself
reaching an admitted trace in each.

## Reproducing and CI

```sh
./scripts/download-pssm-suite.sh                  # once; verifies the pinned SHA-256
go run ./cmd/pssm-referee                         # summary and per-bucket rows
go run ./cmd/pssm-referee -json                   # the full report, byte-identical for any -jobs
go run ./cmd/pssm-referee -check                  # exit 1 unless the counts reproduce the baseline
go run ./cmd/pssm-referee -filter "Deferred 004"  # one test's rows
go run ./cmd/pssm-referee -keep build/pssm/models # write the translated models for debugging
OPENSYSML_REQUIRE_PSSM_SUITE=1 go test ./internal/pssm/...   # the classifier and emitter gates
```

CI downloads the suite and runs `-check` with `OPENSYSML_REQUIRE_PSSM_SUITE=1`; on a checkout
without the suite the referee and the gates report the absence and skip, unless that variable
is set, in which case they fail. `.agents/skills/testing-pssm-referee/SKILL.md` walks the
provisioning, the reproduction, the determinism and disagreement detectors, and the adversarial
paths (bad checksum, missing suite, a hand-broken translation).
