# The PSSM test suite as an advisory referee

> **Labels.** Rows such as *SM7* or *SM15* are the rows of the semantic map in
> [precise-semantics alignment](../internals/design/precise-semantics-alignment.md), which
> defines each one; this record cites them so a test's verdict can be read against the row it
> reports on. Test names such as *Deferred 004 A* are the suite's own.

`tools/cmd/pssm-referee` runs the OMG *Precise Semantics of UML State Machines* (PSSM) test suite,
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

`tools/referee/pssm` reads the suite's UML subset — state machines, regions, vertices of every kind
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
| **standard** | every construct has a spelling in standard SysML v2 notation | 35 |
| **extension** | spellable with this project's state-body extensions (`fork`, `join`, `junction`, `choice`, `history`, `defer`) | 31 |
| **not-expressible** | uses a construct with no spelling (entry and exit points, local and internal transitions, state-machine redefinition), a behavior shape the translation does not spell, or a shape this project's lowerer refuses | 37 |

A test using any construct with no spelling or no translation is not expressible whatever else
it uses; otherwise the extensions win over standard. A terminate pseudostate is standard
notation (a terminate action usage, SysML v2 §7.18.3) and decides nothing; the three tests
that reach one were a class of their own, **terminate-gap**, while the runtime parsed and
lowered `terminate` without executing it (alignment finding 1, fixed), and are standard since.
The alignment note was first written with a hand count of 37 / 33 / 3 / 30; the classifier is
the record from now on, and the note's test-suite section carries its figures. Nine tests moved
from the hand count when the emitter was written, two of them moved back when the lowerer
learned to accept a fork-entered region, a third when the driver learned to perform the
tester's calls and traces in the tester's order, and a tenth moved when its failure was
adjudicated; each is listed with its reason in the note under
[Moves from the hand count](../internals/design/precise-semantics-alignment.md#moves-from-the-hand-count),
and the four constructs the translation rather than the notation stood in the way of are read
under [Behavior parameters, operation results, tester traces and standalone machines](../internals/design/precise-semantics-alignment.md#behavior-parameters-operation-results-tester-traces-and-standalone-machines):

- **Entry, exit or do behaviors with parameters** that read the triggering event's data:
  *Event 017-B*, *Event 019-B*, *Event 019-C*, *Event 019-E*, and among the tests not
  expressible on other grounds *Entry 002-F*, *Standalone 002*, *Standalone 003*. The notation
  binds event data on the transition (`accept d : Data`), never on an `entry`, `exit` or `do`
  action, and no spelling routing it from the one to the other is written.
- **An operation the tester calls and whose result it traces**: *Event 019-D*, *Event 019-E*,
  *Deferred 007*, *Standalone 003*. The runtime returns the outputs the triggered behaviors
  wrote to the caller (`StateExecutor.Call`, alignment row A14) and the driver traces them
  where the tester does; the emitter spells no effect, entry or exit that returns a value.
- **A `trace(...)` in the tester's own behavior** is translated: the driver performs the
  tester's steps in order and appends the trace to `log` once the call it follows has
  returned, so *Event 019-A* runs and passes. A trace embedding no call and not directly
  following one is refused, since the machine may still be running; no test is.
- **A standalone state machine** as the class under test is translated: the reader reads the
  machine as the `Target` whose `Machine` is itself, with its attributes, operations and
  constructor. *Standalone 001*, *002* and *003* stay not expressible on entry and exit points
  and on parameterised behaviors, their other reasons byte-identical
  (`TestSuiteNoTranslationReasons`).
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

The emitter (`tools/referee/pssm/emit.go`) produces one in-memory SysML v2 model per expressible
test, following the note's table and its worked example:

- The state machine becomes a state usage `M` in a package named for the test, with a `String`
  attribute `log` (and the target class's own attributes); every `trace("<segment>")` in an
  entry, exit, do or effect behavior becomes an append to `log`, so the final `log` is the
  `::`-joined trace PSSM compares.
- Regions become nested state bodies; an orthogonal state's regions become a `parallel` body's
  substates. Every state and pseudostate is named by its path as a bare identifier
  (`S1_S1_1`), since pseudostate declarations take no quoted name.
- The tester's stimulation is read once for the emitter and the driver
  (`tools/referee/pssm/stimulation.go`): `Start` and the follow-up sends, calls and traces in
  the tester's order, a signal's scalar payload bound on the accepting transition's parameter,
  a call's literal arguments typed by the operation's `in` parameters. A guard
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
and drives a state executor (`Context.CreateStateExecutorFor`, the executor the runtime's
shared state driver and the execution-conformance harness use) through the tester's steps in
the tester's order (`run.go:drive`): a send is queued, a call is `StateExecutor.Call` — the
call event queued, the machine run through the step dispatching it and no further, the
operation's outputs returned to the driver as PSSM §8.5.9 returns them to a synchronous caller — and a tester `trace(...)` is
evaluated over the suite's test library read into the model (`library.go`: `Concat`,
`ToString`, `formatParameterValue`) and appended to the target's `log` where the tester makes
it. The run is under the `explore` schedule policy,
which replays the run once per linearization of its choice points. The set of `log` values
reachable is compared with the test's set of admitted traces **as sets, in both directions**: a
reachable trace the suite does not admit is a failure naming that trace, an admitted trace the
runtime never reaches is a failure naming it. A run that ends in a typed runtime error, or that
exhausts the exploration budget, is a failure whose reason names the error. Each run is bounded
by the runtime's budgets and their environment overrides (`OPENSYSML_MAX_STEPS` and the others
the `sysml` command honors), except that the step budget defaults to 100 000 rather than the
runtime's ten million: a translated test that needs more is looping, and the smaller bound
reports the runaway in a second rather than minutes, and that the exploration runs 4096
linearizations rather than the runtime's default 1024 (`tools/referee/pssm/run.go:DefaultBudget`):
the most a test draws once region entry, exit and firing units are choice points is
*Event 016 B*'s 1152 linearizations (three firings across nested orthogonal regions), past the
default, and a test that exhausts the budget fails rather than reports what it reached.
`-jobs n` explores
`n` linearizations of one test at once; the report is byte-identical for any `n`, and
`TestRefereeDeterministic` asserts it.

### Buckets

| Bucket | Assigned when |
|---|---|
| `pass` | the test is expressible and the reachable set equals the admitted set |
| `fail` | the test is expressible and the sets differ, or the run errored or exhausted its budget; the reason names every extra and missing trace or the error |
| `not-expressible` | the classifier found a construct with no spelling or no translation; the reason names it |
| `differs-by-design` | the test would be a `fail`, **and** the committed table `tools/referee/pssm/rows.go:TestRows` maps it to a note row whose verdict is *differs because v2 differs* |

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

Recorded **2026-09-21** on develop commit **`2a6527359`** with the tester's calls and traces
driven in the tester's order and standalone machines read as targets, with completion events queued in the
order their sources are entered (the pool's order following the entry draw, finding 11's runtime
part), the order of orthogonal
regions drawn as choice points at finding 9's four sites (region entry, region exit,
the units of the firings one occurrence selects, a due do step against the dispatch), `terminate` executing
(alignment finding 1, SM38), the fork-entered-region fix
(finding 6), the active-ancestor fix, the completion-choice fix, the guard-side-effect
classification, the join incoming-effects fix, the junction branch-choice fix (finding 8) and
the segment-effect fix (finding 10) described below, and with every remaining failure
attributed, as
`docs/project/pssm-referee-baseline.json`; regenerate with `go run -C tools ./cmd/pssm-referee -update`,
check with `-check`. The counts are the gate; the rows are for whoever adjudicates a moved count.
The figures below are as measured when this record was last updated and are not the current
baseline — `go run -C tools ./cmd/pssm-referee` prints the current ones.

| Bucket | Tests |
|---|---:|
| `pass` | 52 |
| `fail` | 13 |
| `not-expressible` | 37 |
| `differs-by-design` | 1 |
| **Total** | **103** |

### Movements since the previous baseline

Two counts moved since the previous baseline (develop `b36c7c0f0`, 2026-09-19), `not-expressible`
38 → 37 and `pass` 51 → 52, and six reasons changed without moving a bucket. The driver
performs the tester's steps in the tester's order, a synchronous call returning the operation's
outputs after its run-to-completion step and a tester trace appended to `log` when the call it
embeds has returned (PSSM §8.5.9 `CallEventOccurrence`; the alignment note's A14 row and its
section [Behavior parameters, operation results, tester traces and standalone
machines](../internals/design/precise-semantics-alignment.md#behavior-parameters-operation-results-tester-traces-and-standalone-machines)),
and the reader reads a standalone state machine as the target class, so the classifier refuses
neither the tester's trace where the driver orders it nor the standalone kind. Every other
test's result and reason is byte-identical to the previous baseline's.

| Test | Construct | Movement | Adjudication |
|---|---|---|---|
| Event 019 A | tester trace (translated) | `not-expressible` → `pass` | Expected. The tester calls `this.testable.op()` while `S1` is active: `T2` fires on the call event, `S1`'s exit logs `S1(exit)` and `T2`'s effect `Call(op)`; the call returns once that step is done, the tester's `this.testable.trace("End")` appends `End`, and its `Continue` then fires `T3` out of `S2`, whose exit behavior logs `S2(entry)`. The one admitted trace `S1(exit)::Call(op)::End::S2(entry)` is reached and nothing else, in one run: no draw is involved, since the trace's place is fixed by the call's return |
| Event 019 D | tester trace (translated), operation result | `not-expressible` → `not-expressible`, reason changed | Expected. The tester's trace of `this.testable.op()`'s result is driven, so *tester trace* leaves the reason; *operation result T2* stays, since `T2`'s effect produces the value (`return "output"`) and the emitter spells no effect with a `return` parameter |
| Event 019 E | tester trace (translated), operation result, behavior parameter | `not-expressible` → `not-expressible`, reason changed | Expected. *tester trace* leaves the reason; the parameterised entry behaviors of `S1.1` and `S2.1.1` and the result they produce for `T2`'s operation stay |
| Standalone 001 | standalone machine (translated) | `not-expressible` → `not-expressible`, reason changed | Expected. *standalone state machine* leaves the reason; the machine's two exit points and entry point stay, the reason otherwise byte-identical |
| Standalone 002 | standalone machine (translated) | `not-expressible` → `not-expressible`, reason changed | Expected. *standalone state machine* leaves the reason; the exit point, entry point and `S2`'s parameterised behaviors stay, the reason otherwise byte-identical |
| Standalone 003 | standalone machine (translated), tester trace (translated), operation result, behavior parameter | `not-expressible` → `not-expressible`, reason changed | Expected. *standalone state machine* and *tester trace* leave the reason; the parameterised entry behaviors of `S1.1` and `S2.1.1`, which also produce `or`'s result, stay |
| Deferred 007 | tester trace (translated), operation result | `not-expressible` → `not-expressible`, reason changed | Expected. *tester trace* leaves the reason; *operation result T4* stays, since `T4`'s effect produces the value from the call's `in` parameter (`return T4_effect(p)`) |

Of the eight tests the four constructs held out of the run,
one moves; the seven that need a behavior with parameters or a returning behavior stay refused
on exactly those reasons until the emitter spells them.

### Movements before that

No count moved since the baseline before (develop `e823e6b82`, 2026-09-19), and four rows
did: the runtime queues a state's completion event as the state's entry unit is performed, so
the pool holds two regions' completions in the order the entry draw entered their sources
(§8.5.9; SM10, which now agrees under every policy), where it queued them once the move had
settled, in region declaration order whatever the draw was
([finding 11](#findings-about-our-own-conformance), the one part of it that was a gap of the
runtime). An entry that performs nothing but generates a completion event is a drawn alternative
now, its order observable through the pool's, so the tests whose completing states are entered
silently record an entry draw they did not before and `explore` runs more of them (*Transition
017* 12 → 18, *Transition 019* 24 → 80, *Fork 002* 12 → 20, *History 002-B* 32 → 48; *Event 016
B*'s entries complete nothing and it stays at 1152 under the 4096 budget). `declared` queues what
it queued, so no trace a fixed policy reached is lost; what moves is what `seed:<n>` and
`explore` reach. Every movement is the one the design note's enumeration predicted for this
rule — the row *after the move, pool in the order the sources were entered* of its candidate
table — and no bucket moves.

| Test | Finding | Movement | Adjudication |
|---|---|---|---|
| Transition 017 | 11 (the pool's order, fixed), suite defect | `fail` → `fail`, reason changed | Expected. The three admitted traces that dispatch `T3.1.2(effect)`, the completion of `S3.1`'s region, before `T2.2(effect)`, the completion of `S1`'s other region — `T2(effect)::S1(entry)::T3.1.2(effect)::T2.2(effect)::S3.1(doActivity)::T3.2(effect)`, `…::T3.1.2(effect)::S3.1(doActivity)::T2.2(effect)::…` and `…::S3.1(doActivity)::T3.1.2(effect)::T2.2(effect)::…` — are reached: region 3 drawn first at `entering S1` puts `S3.1.1`'s completion into the pool ahead of `S2.1`'s, and the do step falls before, between or after the two dispatches as before. Six of the eight admitted traces are reached; the two still missing fire `T3.2`, `S3.1`'s own completion transition, before `T3.1.2`, the completion out of its region, the suite's defect recorded in [`omg-issues.md`](omg-issues.md#pssm-transition-017-admits-a-parents-completion-before-its-regions). `fail` citing the defect alone; finding 11's part of the reason is closed |
| Entering 011 | 11 (an initial transition's effect, a completion effect in v2) | `fail` → `fail`, reason changed | Expected (the enumeration's `2 of 6`). Both regions' initial transitions have an effect, spelled as a completion out of a start state; the two start states' completions now dispatch in the order the entry draw entered them, so `S1(entry)::T1.1(effect)::S1.1(entry)::T2.1(effect)::S1.2(entry)` is reached beside `S1(entry)::T2.1(effect)::S1.2(entry)::T1.1(effect)::S1.1(entry)`. The four still missing interleave one region's effect with the other's entry, UML's initial-transition effect as an entry unit, which no v2 completion gives (finding 11, the alignment note's open decision 8); the reason stands on them |
| History 001-C | 11 (the pool's order, fixed; the suite's defect) | `fail` → `fail`, reason changed | Expected (the enumeration's `2 of 2` for the first half). The first entry of `S1` now dispatches `S2.1`'s completion before `S1.1`'s when region 2 is drawn first, so `S1(entry)::S2.2(entry)::S1.1(exit)::S1.2(entry)::…` is reached beside the trace the PSSM text prints. The ten still missing fire `S1.1(exit)::S1.2(entry)` inside the step that restores region 2, the suite's defect recorded in [`omg-issues.md`](omg-issues.md#pssm-history-001-c-and-002-b-admit-a-completion-inside-the-restore-and-contradict-each-other); the reason stands on them |
| History 002-B | 11 (the pool's order, fixed; the suite's defect) | `fail` → `fail`, reason changed | Expected (the enumeration's `1 of 2, 1 extra / 2 of 3`). After the restore, region 1's default entry of `S1.1` and region 2's restored `S2.2` with its initial `S2.2.1` each generate a completion, dispatched now in the order the entry draw entered them, so `…::T3(effect)::S1(entry)::S2.2(entry)::S2.2.1(exit)::T2.2.2(effect)::S2.2.2(entry)::S1.1(exit)::S1.2(entry)::S1(exit)`, the order the specification's own text gives, is reached beside the one reached before. The first entry of `S1` likewise dispatches `S2.1`'s completion first when region 2 is drawn first, `S1(entry)::S2.1(exit)::S2.2(entry)::S1.1(exit)::S1.2(entry)::…`, the order §8.5.9 gives and *History 001-C* admits for the identical half — two traces, one per restore order, which this test does not register. Two of the six admitted are reached and two unregistered ones; the four still missing — three that dispatch `S2.2.1`'s completion, generated a step later, before `S1.1`'s, and one that fires `T1.2` inside the restore — are the suite's defect, recorded in `omg-issues.md` as above; the reason stands on them and on the two unregistered traces |

*Transition 019* and *Fork 002* explore more runs (the entry of each completing target is a
draw) and reach the same sets, *Transition 019*'s six extra traces the join's segment order
(SM34) as before. *Entering 010*, *Junction 005* and *Terminate 002* (finding 11) did not move
and their reasons are byte-identical to the previous baseline's: the initial transition's
effect and a do step against a sibling's entry unit are not the pool's order.

### Movements before that

No count moved since the baseline before (develop `23a50b1d1`, 2026-09-18), and four rows
did: the runtime draws a due do step against the dispatch the machine would make at the same
instant, as a choice point under `check`, `replay` and `explore` (`ChoiceStepOrder`;
[finding 9](#findings-about-our-own-conformance), its fourth and last site). The fixed
policies keep running the do round to its end before they dispatch, so no trace the default
reached before is lost; what `explore` now reaches is the runs in which the machine dispatches
while a do behavior has a step due — before its first action, or between two. One test gains
the admitted trace it missed and moves to `pass`; one loses `pass` on a trace the suite does
not register, adjudicated below as the suite's; two stay `fail` with the do-step part of their
reason closed and the rest standing. The rows of finding 11 keep their reasons byte for byte.

| Test | Finding | Movement | Adjudication |
|---|---|---|---|
| Behavior 003 A | 9 (fixed) | `fail` → `pass` | Expected: the test the do-step site was designed for. `S1` has an entry behavior and a do activity whose first segment logs `S1(doActivityPartI)`; the tester's `AnotherSignal`, in the pool as `S1` is entered, fires the transition out of `S1`. The admitted trace not reached before, `S1(entry)` alone, dispatches `AnotherSignal` before the do activity's first segment: the step-order draw at `t=0.0` between `do S1` and `dispatch accept AnotherSignal` reaches it (`state_do_step_or_dispatch` is the conformance fixture of the shape). Both admitted traces reached, nothing else |
| Exiting 002 | 9 (fixed), suite defect | `pass` → `fail` | Expected once looked at, and the suite's defect rather than the runtime's. The model is *Behavior 003 A*'s — `S1` with a do activity whose first segment logs `S1(doActivityPartI)`, and the tester's `Continue` in the pool as `S1` is entered firing the transition out of `S1` — with an exit behavior logging `S1(exit)` in place of the entry behavior. The same draw reaches the same two runs, `S1(doActivityPartI)::S1(exit)` and `S1(exit)`; the suite registers the first alone, where *Behavior 003 A* registers both orders of the same segment against the same dispatch and *Terminate 002*'s own note says the segment "may be (invoked asynchronously) part of the trace". The test's point — the exit aborts the do activity, so `S1(doActivityPartII)` never logs — holds in both runs. Recorded in [`omg-issues.md`](omg-issues.md#pssm-exiting-002-registers-one-of-the-two-orders-the-suite-admits-elsewhere); the test stays `fail` on that trace alone, with no `differs-by-design` row |
| Terminate 002 | 9 (fixed), 11 | `fail` → `fail`, reason changed | Expected in part. The two admitted traces that dispatch `S2.1`'s terminating completion before the do activity's first segment, `S1(entry)::S1.1(entry)::S2.1(entry)` and `S1(entry)::S2.1(entry)::S1.1(entry)`, are reached: the step-order draw at `t=0.0` between `do S1.1` and the completion's dispatch, the do activity aborted before it logs. The one still missing, `S1(entry)::S1.1(entry)::S1.1(doActivityPartI)::S2.1(entry)`, runs the segment before the sibling region's entry — a step of a do behavior inside the entry front, finding 11's shape. `fail` citing finding 11 alone |
| Transition 017 | 9 (fixed), 11, suite defect | `fail` → `fail`, reason changed | Expected in part. Two more admitted traces are reached, `T2(effect)::S1(entry)::T2.2(effect)::S3.1(doActivity)::T3.1.2(effect)::T3.2(effect)` and `…::T2.2(effect)::T3.1.2(effect)::S3.1(doActivity)::T3.2(effect)`: `S3.1`'s do step drawn after one completion's dispatch, then after two, three of the eight now reached. The five still missing are the three that dispatch `T3.1.2`, the completion of `S3.1`'s region, before `T2.2`, the completion of `S1`'s other region — finding 11's shape, the completions dispatched in the order their sources were entered — and the two that fire `T3.2` before `T3.1.2`, the suite's defect recorded in [`omg-issues.md`](omg-issues.md#pssm-transition-017-admits-a-parents-completion-before-its-regions). `fail` citing finding 11 and the defect; the do-step part of the reason is closed |

*Deferred 006 C* explores more runs (the dispatch of `Continue` drawn against each region's do
step) and reaches the same two admitted traces and nothing else; *Deferred 006 A* and *Deferred
006 B* are unchanged: a dispatch that would defer its occurrence is not drawn ahead of a due do
step, so `S2`'s do activity, for which `S2` defers `AnotherSignal`, reaches its `accept` before
the signal is dispatched, as it did.
*Entering 010*, *Entering 011*, *Junction 005*, *History 001-C* and *History 002-B* (finding 11)
did not move and their reasons are byte-identical to the previous baseline's.

### Movements before that

Five counts moved since the previous baseline (develop `c2bffb389` with `terminate`
executing, 2026-09-17): the runtime draws the order in which a composite state's orthogonal
regions and a fork's branches are entered, the order in which orthogonal regions are exited,
and the order of the units — source exit, effect, target entry — of the firings one occurrence
selects across regions, as choice points the schedule policy resolves and `explore` varies
(`ChoiceEntryOrder`, `ChoiceExitOrder`, the per-unit `ChoiceRegionOrder`;
[finding 9](#findings-about-our-own-conformance), three of its four sites). The default
policy takes the order the runtime always took, so no trace it reached before is lost; what
`explore` now reaches is the other linearizations. Five tests gain every admitted trace they
missed and move to `pass`; two others stay `fail` with a changed reason; the six that depend on
the site not yet implemented — a due do step against the dispatch at the head of the pool —
or on the completion-firing interleaving of finding 11 keep their reasons byte for byte.

| Test | Finding | Movement | Adjudication |
|---|---|---|---|
| Exiting 001 | 9 (fixed in part) | `fail` → `pass` | Expected. `S1` has two regions, `S1.1` (with `S1.1.1` inside) in the first and `S2.1` in the second; leaving `S1` exits `S1.1.1`, then `S1.1`, and `S2.1`, the library ordering a nested state's exit before its parent's and nothing across the regions. The two admitted traces not reached before, `S1.1.1(exit)::S2.1(exit)::S1.1(exit)::S1(exit)` and `S2.1(exit)::S1.1.1(exit)::S1.1(exit)::S1(exit)`, are the other two linearizations of the chain `S1.1.1(exit)::S1.1(exit)` against `S2.1(exit)`, which the exit-order draw at `exiting S1` reaches (`state_region_exit_order` is the conformance fixture of the shape). All three admitted traces reached, nothing else |
| Exiting 003 | 9 (fixed in part) | `fail` → `pass` | Expected. `S1.1` is itself parallel inside `S1`, with `S1.1.1` and `S1.2.1` one in each of its regions: the exit-order draw at `exiting S1.1` reaches `S1.2.1(exit)::S1.1.1(exit)::S1.1(exit)::S1(exit)` beside the declared order. Both admitted traces reached, nothing else |
| Fork 002 | 9 (fixed in part) | `fail` → `pass` | Expected. `T2` ends at a fork whose branches `T2.1` and `T2.2` enter the two regions of `S1` directly, `T2.2`'s target `S1.1` logging its entry: after `T2(effect)`, `S1(entry)` is entered once by whichever branch is drawn first, then the branches' effects and `S1.1(entry)` interleave with `T2.2(effect)` before `S1.1(entry)` and nothing across the branches. The three admitted traces not reached before are the other linearizations of the chain `T2.2(effect)::S1.1(entry)` against `T2.1(effect)`, which the entry-order draw at `fork` reaches (`state_fork_branch_order`). All four admitted traces reached, nothing else |
| Terminate 001 | 1 (fixed), 9 (fixed in part) | `fail` → `pass` | Expected. The admitted trace not reached before, `S1(entry)::S2.1(entry)::S1.1(entry)::S2.1(exit)`, enters `S1`'s second region before its first: the entry-order draw at `entering S1` reaches it, and the termination — `S2.1` exited by its completion transition into the terminate pseudostate, `S1.1` left active and never exited, the machine ended — is the same in both. Both admitted traces reached, nothing else |
| Deferred 006 C | — (SM15 row), 9 (fixed in part) | `differs-by-design` → `pass` | Expected once looked at, and a correction of the previous adjudication. `S1.1` and `S1.2`, one in each region of `S1`, each have a do activity parked at `accept Continue`, `S1.1` deferring `Continue` as well; the runtime dispatches the one `Continue` to both do activities (SM15's rule, which the test still reports on), and each logs as it takes it. The trace not reached before, `S1.2(doActivity)::S1.1(doActivity)`, was filed under SM15 as "the same rule" as *Deferred 006 B*'s; it was not — both do activities take the occurrence under either rule, and what the runtime never varied was the *order* they take it in, which follows the order the do activities were started, the region-entry order. The entry-order draw at `entering S1` reaches it. The previous `differs-by-design` verdict was therefore misattributed: SM15 is the reason *Deferred 006 B* stays there, not this variant's. Both admitted traces reached, nothing else; the row keeps its SM15 mapping as *Deferred 006 A* does (a pass reporting on the row) |
| Transition 019 | SM34, 9 (fixed in part) | `fail` → `fail`, reason changed | Expected. The four admitted traces not reached before — both regions' exits before either effect, `S1.1(exit)::S2.1(exit)::T1.2(effect)::T2.2(effect)::…` and its kin, the steps of two firings interleaved — are reached: a firing is not atomic across regions, its exit and effect drawn one unit at a time against the sibling's (the granularity decision of the alignment note's SM21 row). Every one of the six admitted traces is reached. What the run reaches beyond them is six traces, not two: every admitted order of the exits and effects continued with the join's segments `T1.3` and `T2.3` in the order *opposite* to the regions' firing order, where PSSM fires each completion transition into `Join1` as its source's completion is dispatched and so follows the sources' order. That is SM34 alone — the runtime holds the join's segments until the join is ready and draws their order, the recorded tool choice — so the finding-9 part of the reason is closed and the test stays `fail` citing SM34 |
| Terminate 002 | 1 (fixed), 9 (fixed in part) | `fail` → `fail`, reason changed | Expected in part. Of the four admitted traces not reached before, the one whose difference was the region-entry order alone, `S1(entry)::S2.1(entry)::S1.1(entry)::S1.1(doActivityPartI)`, is reached through the entry-order draw. The three still missing move the do activity's first segment: `S1(entry)::S1.1(entry)::S2.1(entry)` and `S1(entry)::S2.1(entry)::S1.1(entry)` dispatch `S2.1`'s terminating completion before it — the do-step site, designed and not yet implemented — and `S1(entry)::S1.1(entry)::S1.1(doActivityPartI)::S2.1(entry)` runs it before the sibling region's entry, a step of a do behavior inside the entry front, the shape finding 11 records for a completion's firing. `fail` citing finding 9's do-step site and finding 11 |

*Behavior 003 A* (the do-step site), *Transition 017* (the do-step site and finding 11), and
*Entering 010*, *Entering 011*, *Junction 005*, *History 001-C* and *History 002-B* (finding 11)
did not move and their reasons are byte-identical to the previous baseline's.

Since that baseline, finding 11 has been adjudicated without a count moving or the baseline
being re-recorded: the referee's per-test reasons, which name traces, are unchanged, and the
rows of the failure table below for the seven tests that cited the finding — the five above,
*Terminate 002* and *Transition 017* — now carry the reasons the adjudication gives (an initial
transition's effect, the pool's order, the suite's defect, the do-step site), no longer a
runtime gap of the entry front.

### Movements before that

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
the fourteen failures it left unattributed ([below](#fail-13)) added four tests to the
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

### `pass` (52)

Behavior 001, Behavior 002, Behavior 003 A, Behavior 003 B, Transition 001, Transition 007, Transition 011 C,
Transition 015, Transition 016, Transition 020, Transition 022, Event 001, Event 002, Event 008, Event 009,
Event 010, Event 015, Event 016 A (reports on SM11), Event 016 B, Event 017 A, Event 018, Event 019 A, Entering 004,
Entering 005, Exiting 001, Exiting 003, Exiting 005, Fork 002, Choice 001 and Choice 002 (report on SM30), Choice 003,
Choice 004, Final001 (reports on SM11), Deferred 001, Deferred 002, Deferred 003 (reports on
SM7), Deferred 004 A and Deferred 004 B (report on SM7), Deferred 005, Deferred 006 A (reports
on SM15: the runtime's rule and PSSM's agree on this variant), Deferred 006 C (reports on SM15:
both do activities take the occurrence under either rule, and their order is drawn), History 001-A, History 001-B,
History 001-D, History 002-A, History 002-C (reports on SM28), History 002-D, Join002, Junction 001,
Junction 003, Terminate 001.

### `differs-by-design` (1)

| Test | Row | Admitted trace not reached |
|---|---|---|
| Deferred 006 B | SM15 | `S2(doActivityPartI)::S2(doActivityPartII)` — PSSM gives the deferred occurrence to the do activity alone; the runtime dispatches it to every scope |

### `fail` (13)

Every failure is attributed. Five cite a *differs, v2 silent* row of the alignment note through
the committed table: the suite's second opinion on a tool choice, which the row records and the
test does not overturn. Seven cite an open finding against this project — a gap of the runtime's,
recorded [below](#findings-about-our-own-conformance) and in the alignment note, that a change
of its own will close, moving the tests with it; one cites a defect of the suite's alone. None is a translation defect: each translated
model was read against the test's UML, and every construct the test uses reaches the run.

#### Citing a note row (5)

| Test | Row | What the run shows |
|---|---|---|
| Junction 002 | SM32 | run error: no outgoing guard of the junction holds; PSSM disables the compound transition and admits `T3(effect)` |
| Junction 004 | SM32 | run error: `junction S1_Junction1_2: no guard evaluated to true` — the junction on the default entry of `S1`'s second region, both of whose outgoing guards are false; PSSM's static evaluation disables the transition into `S1` as a whole, `S1` is never entered and the next occurrence fires `T3` from the source, admitting `T3(effect)`. The same rule as *Junction 002*'s, one region further in; the emitter carries both guards as `false` faithfully |
| Join003 | SM32 | run error: `join Join1: no guard evaluated to true` — the join's only outgoing transition is guarded `value < 10` with `value` `15`; the runtime fires the join's segments together once both sources are active (SM34) and fails resolving the way out. PSSM fires the first completion transition into the join alone (a segment may end at a join, whose completion is waited for) and disables the second, whose entering the join would need a way through, so `S1` stays active for `T5`; admits `T1.2(effect)::T5(effect)` and `T1.4(effect)::T5(effect)`. SM32's rule at a join's way out |
| Join001 | SM34 | reached `S1.1(exit)::T2.3(effect)::S2.1(exit)::T2.4(effect)::S1(exit)` and its mirror, `S1(exit)` after the last incoming segment's effect; PSSM admits `S1.1(exit)::T2.3(effect)::S2.1(exit)::S1(exit)::T2.4(effect)` and its mirror, `S1` left with the last source, before that segment's effect. The row records the runtime's place for the owner's exit — after every incoming effect, the library reading of the oracle's join section — and the test's own prose expected execution puts `S1(exit)` there too; its assertion does not |
| Transition 019 | SM34 | reached every one of the six admitted traces — the two regions' exits and effects in every order the library admits, a firing not atomic against its sibling's — and six more: each of those orders continued with the join's segments `T1.3` and `T2.3` in the order opposite to the regions' firing order, `S1.1(exit)::S2.1(exit)::T1.2(effect)::T2.2(effect)::T2.3(effect)::T1.3(effect)` among them. PSSM fires each completion transition into `Join1` when its source's completion is dispatched, so their order follows the sources'; the runtime holds them until the join is ready and draws the order (the row). Only the row remains |

#### Citing a finding (7)

One line per test, from the baseline's `reasons`: what the run reached that the suite does not
admit (`—` when every reached trace is admitted and the failure is only a missing one), and
what PSSM admits that the run never reached. Where PSSM admits several interleavings one is
quoted and the number given. The full sets are in the baseline file.

| Test | Finding | Reached, not admitted | Admitted, not reached |
|---|---|---|---|
| Transition 017 | suite defect (finding 11's part, the pool's order, fixed) | — | `T2(effect)::S1(entry)::T2.2(effect)::T3.2(effect)::S3.1(doActivity)::T3.1.2(effect)` and `T2(effect)::S1(entry)::T2.2(effect)::T3.2(effect)::T3.1.2(effect)::S3.1(doActivity)` (six of the eight admitted orders are reached: the do activity's segment before, between and after the two completions' dispatches, the completions `T2.2` and `T3.1.2` in either order — PSSM's pool holds them in the order their sources were entered (§8.5.9), which the entry draw at `entering S1` decides, and the runtime queues each as its source's entry unit is performed). The two missing fire `T3.2`, `S3.1`'s completion transition, before `T3.1.2`, the completion out of its own region, and run the do activity's segment after `S3.1` was left; they contradict the test's expected sequence and `StatePerformances.kerml` alike and are the suite's defect recorded in [`omg-issues.md`](omg-issues.md#pssm-transition-017-admits-a-parents-completion-before-its-regions) — no runtime reaches them, so the test stays `fail` on them |
| Entering 010 | 11 (an initial transition's effect, a completion effect in v2) | — | `S1(entry)::T2.1(effect)::S1.1(entry)::S2.1(entry)` and `S1(entry)::T2.1(effect)::S2.1(entry)::S1.1(entry)` (the third admitted order is reached; both run `T2.1(effect)`, the second region's initial transition's effect, before the first region's explicit target `S1.1` is entered. UML runs that effect as part of the region's default entry, so PSSM admits it anywhere against the sibling's entry units; SysML v2 has no place for an effect on an entry transition, so the referee spells it as the effect of a completion transition out of a start state, which the runtime dispatches as a step of its own after the entry, as it does every v2 completion. Folding the effect into the target's entry action instead is refused by this very test: region 1's initial transition `T1.1` has an effect too, and its target `S1.1` is the one the tester enters explicitly, so the fold runs `T1.1(effect)` on an entry that bypasses the initial transition — a trace in none of the three admitted; the design note's candidate table has the enumeration) |
| Entering 011 | 11 (an initial transition's effect, a completion effect in v2) | — | `S1(entry)::T1.1(effect)::T2.1(effect)::S1.1(entry)::S1.2(entry)` and 3 more orders of the two regions' initial effects and entries (two of the six admitted orders are reached, each region's initial effect and entry whole, in the order the entry draw entered the two start states; as *Entering 010*, with an initial effect in each region. The four missing interleave one region's effect with the other's entry. The fold into the target's entry action reaches the same two and no refused trace, and no more: one entry action is one unit, so `T1.1(effect)` cannot be split from `S1.1(entry)` around the sibling's units as the other four admitted orders split it) |
| History 001-C | 11 (the suite's defect; the pool's order, fixed) | — | `S1(entry)::S1.1(exit)::S1.2(entry)::S2.2(entry)::S2.2.1(exit)::S2.2.2(entry)::S1(exit)::S1(entry)::S1.1(exit)::S1.2(entry)::S2.2(entry)::S2.2.2(entry)::S1(exit)` and 9 more orders of the two regions' entries and exits (two of the twelve admitted orders are reached, the one the PSSM text prints and the one that dispatches `S2.1`'s completion before `S1.1`'s after the first entry of `S1` — the pool in the order the regions were entered, which the runtime's queue follows). The ten missing fire `S1.1(exit)::S1.2(entry)`, a completion the default entry of region 1 enables, *inside* the step that restores region 2, before or between `S2.2(entry)::S2.2.2(entry)`; the test's own note ends the restoring step first ("This completes the step started by the firing of `T4`. When dispatched, the completion event occurrence generated by `S1.1` triggers `T1.2`"), and six of the ten split the firing around a restored entry, which *History 002-B* forbids — the suite's defect recorded in [`omg-issues.md`](omg-issues.md#pssm-history-001-c-and-002-b-admit-a-completion-inside-the-restore-and-contradict-each-other) |
| History 002-B | 11 (the suite's defect; the pool's order, fixed) | `S1(entry)::S2.1(exit)::S2.2(entry)::S1.1(exit)::S1.2(entry)::…` (two traces, one per order of the restore's two completions) | `…::S1(exit)::T3(effect)::S1(entry)::S1.1(exit)::S1.2(entry)::S2.2(entry)::S2.2.1(exit)::T2.2.2(effect)::S2.2.2(entry)::S1(exit)` and 3 more orders of the two regions' entries and exits (two of the six admitted orders are reached, the one the PSSM text prints and the one that dispatches region 2's restored completion first after the restore, `S2.2(entry)::S2.2.1(exit)::T2.2.2(effect)::S2.2.2(entry)::S1.1(exit)::S1.2(entry)`, which the specification's text gives). The two reached and not admitted dispatch `S2.1`'s completion before `S1.1`'s after the first entry of `S1`, the order §8.5.9's pool gives when region 2 is entered first and the one *History 001-C* admits for the identical half. Three of the missing dispatch `S2.2.1`'s completion, generated by `T2.2`'s firing in the second step, before `S1.1`'s, generated by the first step's entry, which §8.5.9's pool cannot do in any entry order; one more fires `S1.1(exit)::S1.2(entry)` inside the step that restores region 2, which the test's own note ends first ("At the end of the RTC step … the state machine is in configuration `S1[S1.1, S2.2[S2.2.1]]`. The next step consists in the firing of `T1.2`"). The suite's defect, recorded in [`omg-issues.md`](omg-issues.md#pssm-history-001-c-and-002-b-admit-a-completion-inside-the-restore-and-contradict-each-other) |
| Junction 005 | 11 (an initial transition's effect, a completion effect in v2) | — | `S1(entry)::T2.1(effect)::S2.1(entry)::T1.3(effect)::S1.2(exit)::S1(exit)` and `S1(entry)::T2.1(effect)::T1.3(effect)::S2.1(entry)::S1.2(exit)::S1(exit)` (the third admitted order, `T1.3(effect)` first after `S1(entry)`, is reached: the second region's initial effect `T2.1` and its target's entry are admitted before or around the junction segment's effect, as *Entering 010*'s. `S2.1`'s completion out of `S1`, a real one, is admitted only after `T1.3(effect)` in all three, where the runtime dispatches it. The fold into the target's entry action has no target here: `T2.1` ends at a junction, whose way out a guard decides, so the effect has no one state's entry to join) |
| Terminate 002 | 11 (a do step inside the entry front) | — | `S1(entry)::S1.1(entry)::S1.1(doActivityPartI)::S2.1(entry)`, the do activity's first segment before the sibling region's entry (the four admitted orders with the segment after both entries or not at all are reached; the do activity's second segment is in none, aborted). The segment is a due do step of the entered `S1.1` drawn against the sibling's remaining entry unit — the do-step site's rule on the entry front, where that site draws it against the dispatch alone today |

Every reason in full — each extra trace, each missing trace, each error — is in the baseline
file's `reasons`.

### `not-expressible` (37)

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
- **behavior parameter** (no translation: the emitter spells no entry, exit or do behavior
  bound from the triggering event's data): Event 017 B, Event 019 B, Event 019 C, Event 019 E,
  Standalone 003, and among the above Entry 002 F, Standalone 002.
- **operation result** (no translation: the emitter spells no effect, entry or exit that
  returns a value; the runtime carries the result and the driver traces it): Event 019 D,
  Event 019 E, Deferred 007, Standalone 003.
- A standalone state machine is read as the target class, and a tester's `trace(...)` after a
  call is driven, so neither is a reason any longer; Event 019 A runs and passes.
- **lowerer refuses an orthogonal region with neither an entry transition nor a fork branch
  into it** (ours): Entry 002 E, which is not expressible on other grounds too. Fork 002 and
  Join 001, filed here while the lowerer refused every region without an entry transition,
  translate since finding 6 was fixed.
- **guard side effect** (no translation): Choice 005.

A test with several such constructs is listed under each; the baseline file names every
test's constructs in its `reasons`.

## Findings about our own conformance

The referee's classifier and runs surfaced six gaps that are this project's rather than SysML
v2's. Four — the first two found when the referee was added, the third by its exploration, the
fourth by attributing the failures that remained — are fixed, each in a change of its own; one
is fixed at three of its four sites, and one, found while fixing it, is open. The tests that
reach an open site stay `fail` citing it until a change of its own closes it:

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
- **The order in which orthogonal regions are entered, exited and stepped was not a recorded
  choice point** (alignment finding 9; fixed at three of its four sites, the fourth designed).
  Thirteen tests reached only traces the suite admits and failed on admitted traces they never
  reached, because four sites ordered what PSSM leaves concurrent and recorded no choice for
  `explore` to vary. Not a defect of behavior — SysML v2 §7.18.1 leaves parallel substates and a
  `do` sub-performance concurrent, so one linearization per site is one v2 admits — but a gap of
  exploration, closed by the scheduling design
  [recording the order of orthogonal regions](../internals/design/region-order-scheduling.md).
  Three sites are implemented: the regions of a composite state, a fork's branches and the
  regions a history restores are entered under a draw (`ChoiceEntryOrder`, `entering S1`,
  `fork`; `state_region_entry.go`), orthogonal regions are exited under a draw
  (`ChoiceExitOrder`, `exiting S1`; `state_executor.go:exitState`), and the firings one
  occurrence selects across regions run one unit — source exit, effect, target entry — at a
  time under a draw (`ChoiceRegionOrder`; `dispatchInOrder`), a firing not atomic against its
  sibling's (the alignment note's SM21 row records the decision, argued from the KerML
  successions, which order the units of one firing and nothing across two). Each region's units
  run as a coroutine on a front (`state_unit_front.go`) that draws which region's next
  behavior-performing unit runs while two or more have one, silent entries and exits riding with
  the performing unit next to them so that exploration counts linearizations of behaviors, not
  of bookkeeping; `declared` and `reverse` take declaration order, so no trace golden's event
  order moved under the default and the goldens that record a draw gained `choice` lines alone;
  seeds and `explore` reach the other orders; a witness names each draw and replay refuses one
  the run does not offer, rolling the move back. Moved: *Exiting 001*, *Exiting 003*,
  *Fork 002*, *Terminate 001* and *Deferred 006 C* to `pass`, and every admitted trace of
  *Transition 019* is reached, leaving SM34 alone (the movements tables above adjudicate each).
  The fourth site — a due do step against the dispatch the machine would make at the same
  instant — is drawn under `check`, `replay` and `explore` (`ChoiceStepOrder`, `state_executor.go:oneUnit`):
  one move is one action of a state's do behavior (`stepDoAction`) or the dispatch, "dispatch it
  now" against "keep moving the do flow"; the fixed policies finish the do round before they
  dispatch as they always did, so no default trace moved. A dispatch that would drop or defer its
  occurrence is not drawn ahead of a due do step — neither is an acceptance, and the do step may
  be the `accept` that takes it — so it waits for the round to close. Moved: *Behavior 003 A* to
  `pass`; *Terminate 002* and *Transition 017* reach every admitted trace of the do step's
  placement and stay `fail` on finding 11's (and, *Transition 017*, on the suite's two anomalous
  traces, recorded in
  [`omg-issues.md`](omg-issues.md#pssm-transition-017-admits-a-parents-completion-before-its-regions));
  *Exiting 002* reaches the order the suite registers for *Behavior 003 A* and not for it, the
  suite's defect. A round closes once each due do action has stepped, and the dispatch it owes
  goes before another opens, the fixed policies' rhythm; where a step left a token of its body
  standing the sweep-then-dispatch run of the fixed policies is still the checker's bounded
  verdict (`notEnumerated`), not yet a move of its own.
- **A completion transition's firing is not drawn against the entry front it completes in**
  (*Entering 010*, *Entering 011*, *Junction 005*, *History 001-C*, *History 002-B*; alignment
  finding 11, adjudicated). Found while implementing finding 9's entry site: every admitted
  trace these five still miss interleaves the firing of a completion transition — the initial
  transition of a region, translated as a completion out of a start state (`T2.1(effect)` in
  *Entering 010*), or `S1.1(exit)::S1.2(entry)` in the History tests — with the entry units of
  the sibling region *in the same step*, where the runtime dispatches a completion as a
  run-to-completion step of its own once the entry move has settled (SM9, SM10). The finding
  was first read as one gap of exploration, to be closed by offering the completion's firing as
  a unit of its region's queue on the front that entered it. Enumerated against the five
  admitted sets in the design's section
  [a pending completion inside the entry front](../internals/design/region-order-scheduling.md#finding-11-a-pending-completion-inside-the-entry-front),
  that rule reaches every admitted trace and, on three of the tests, traces PSSM refuses, and
  no narrower rule reaches the five: they want three different things. *Entering 010*,
  *Entering 011* and *Junction 005* want the effect of a region's **initial transition** run
  as part of the region's entry, as UML has it; SysML v2 has no place for an effect on an
  entry transition, so the referee spells it as a completion transition's effect, and the
  runtime dispatches it as it dispatches every v2 completion. The other spellings were run
  against the three (the design note's candidate table): folding the effect into the entry
  action of the region was the emitter defect *History 001-B* exposed, and folding it into the
  entry action of the state the initial transition targets, sequenced before that state's own
  entry behavior, reaches no admitted trace of *Entering 010* (region 1's initial effect then
  runs on the tester's explicit entry of `S1.1`, which bypasses the initial transition), two of
  *Entering 011*'s six (one entry action is one unit, so the effect cannot be split from the
  entry around the sibling's units) and has no target in *Junction 005* (the initial transition
  ends at a junction). *Junction 005* shows the class is right — `S2.1`'s real completion is
  admitted only after the sibling's remaining entry unit — so no runtime rule reaches the three
  without telling a start state's completion from a real one, which neither v2 nor PSSM does.
  Whether the difference takes a *differs because v2 differs* row is the alignment note's open
  decision 8; the three stay `fail` citing it. *History 001-C*'s first half wants the **pool's**
  **order** to follow the entry draw (§8.5.9: completion events dispatch in the order generated,
  which the order the regions were entered decides), where the runtime queued them after the
  move in declaration order (`scheduleTransitionEvents`) — the one part of the finding that was a
  gap of the runtime, a divergence under `reverse`, `seed:<n>` and `explore` alone. **Fixed**: a
  state's completion is queued as its entry unit is performed (`enterStateInto`), and an entry
  that performs nothing but generates a completion is a drawn alternative of the front rather
  than a silent unit, its order being observable through the pool's (the design note's
  *silent units* section; `state_completion_pool_entry_order` and its history and fork kin are
  the conformance fixtures). It moved no bucket, as the design's enumeration said: *History
  001-C* reaches both first-half orders, *History 002-B* one more admitted trace and the two
  §8.5.9 gives that the suite does not register, *Entering 011* its second, and *Transition
  017*'s reachable set is complete but for the suite's two anomalous traces (the movements
  table above). The rest of
  the History pair is the **suite's**
  **defect**, recorded in
  [`omg-issues.md`](omg-issues.md#pssm-history-001-c-and-002-b-admit-a-completion-inside-the-restore-and-contradict-each-other):
  both tests register a completion dispatched inside the step that restores the sibling region,
  which their own notes and RTC tables end first, and they contradict each other on the
  identical halves. The two stay `fail` citing the defect; no runtime rule is chosen to reach
  either, and against the specification's own text neither can pass on the downloaded XMI.
  *Terminate 002*'s one remaining trace has the shape with a do step in place of the completion's
  firing: a due do step of the entered `S1.1` drawn against the sibling's remaining entry unit,
  the do-step site's rule on the entry front, where that site draws it against the dispatch
  alone; the test stays `fail` citing this finding for it, and the design records it for that
  site's next change. No bucket moved under the adjudication or under the pool's fix.
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
  still misses, which interleave the second region's completion with the junction segment —
  finding 11; the movements table of that baseline adjudicates it.

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
| The order of a join's segments | missing alignment text, SM34 (*differs, v2 silent*) | Transition 019 (finding 9's part closed) | `fail`, citing SM34 |
| Region entry, exit and firing-unit order is not a recorded choice | runtime gap, finding 9 (fixed at these sites) | Exiting 001, Exiting 003 | `pass` |
| A do step against the dispatch at the head of the pool is not a recorded choice | runtime gap, finding 9 (fixed at this site, at do-action granularity) | Behavior 003 A | `pass` |
| A do activity's first segment registered as always before the next dispatch | suite defect (the suite admits both orders elsewhere) | Exiting 002 | `fail`, citing the defect |
| An initial transition's effect is a completion effect in v2 | language difference, finding 11 (adjudicated; the row is open decision 8 of the alignment note) | Entering 010, Entering 011 | `fail`, citing finding 11 |
| The pool's order does not follow the entry draw | runtime gap, finding 11 (fixed: completions queued as their sources are entered) | History 001-C (one trace, reached), Transition 017 (three, reached) | `fail`, citing the suite's defects for the rest |
| A completion dispatched inside the step that restores the sibling region | suite defect, finding 11 (recorded in `omg-issues.md`) | History 001-C, History 002-B | `fail`, citing finding 11 |
| A do step against a sibling's entry unit is not a recorded choice | runtime gap, finding 11 (the do-step site's rule on the entry front, recorded for that site's next change) | Terminate 002 | `fail`, citing finding 11 |
| A junction segment's effect before its owner's entry | runtime defect, finding 10 (fixed) | Junction 005 | `fail`, citing finding 11 for the initial transition's effect, admitted before or around the segment's effect once the segment's effect follows `S1(entry)` |

*Fork 002* and *Join001*, which finding 6's fix brought out of `not-expressible` after that
baseline, are attributed with them: *Fork 002* to finding 9 (region entry order, now `pass`)
and *Join001* to SM34 (where the owner is left), each read against its requirement. So are
*Terminate 001* and *Terminate 002*, which executing `terminate` brought out of
`terminate-gap`: *001* to finding 9's region-entry order (now `pass`), *002* to finding 11
(its do-step traces against the dispatch reached; the do step against the sibling's entry unit
remains), the termination itself reaching an admitted trace in each.

## Reproducing and CI

```sh
./scripts/download-pssm-suite.sh                  # once; verifies the pinned SHA-256
go run -C tools ./cmd/pssm-referee                         # summary and per-bucket rows
go run -C tools ./cmd/pssm-referee -json                   # the full report, byte-identical for any -jobs
go run -C tools ./cmd/pssm-referee -check                  # exit 1 unless the counts reproduce the baseline
go run -C tools ./cmd/pssm-referee -filter "Deferred 004"  # one test's rows
go run -C tools ./cmd/pssm-referee -keep build/pssm/models # write the translated models for debugging
OPENSYSML_REQUIRE_PSSM_SUITE=1 go test -C tools ./referee/pssm/...   # the classifier and emitter gates
```

CI downloads the suite and runs `-check` with `OPENSYSML_REQUIRE_PSSM_SUITE=1`; on a checkout
without the suite the referee and the gates report the absence and skip, unless that variable
is set, in which case they fail. `.agents/skills/testing-pssm-referee/SKILL.md` walks the
provisioning, the reproduction, the determinism and disagreement detectors, and the adversarial
paths (bad checksum, missing suite, a hand-broken translation).
