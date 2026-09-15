# The PSSM test suite as an advisory referee

> **Labels.** Rows such as *SM7* or *SM15* are the rows of the semantic map in
> [precise-semantics alignment](../internals/design/precise-semantics-alignment.md), which
> defines each one; this record cites them so a test's verdict can be read against the row it
> reports on. Test names such as *Deferred 004 A* are the suite's own.

`cmd/pssm-referee` runs the OMG *Precise Semantics of UML State Machines* (PSSM) test suite,
translated by rule into SysML v2 textual notation, against this runtime, and files every test in
one of five buckets. It is advisory and opt-in: CI compares the committed **bucket counts**,
never a pass/fail verdict, and a movement in any count is adjudicated in the change that moves
it, exactly as the [pilot corpora](pilot-corpora.md) ratchet and the
[pilot execution referee](pilot-execution-referee.md) are.

**What a pass means.** A pass checks that the runtime reproduces UML behavior where the model
has a defensible SysML v2 mapping, provides a second opinion on the tool-choice rows, and is
never evidence of SysML v2 conformance. The suite is a UML artefact; where SysML v2 or the
Kernel Semantic Library say otherwise, the runtime follows them and the disagreement is
recorded as v2's, not as a failure. Used this way the suite is a second opinion on the eight
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
| **standard** | every construct has a spelling in standard SysML v2 notation | 31 |
| **extension** | spellable with this project's state-body extensions (`fork`, `join`, `junction`, `choice`, `history`, `defer`) | 30 |
| **terminate-gap** | spellable, but reaches `terminate`, which the runtime parses and lowers and does not yet execute (alignment finding 1) | 3 |
| **not-expressible** | uses a construct with no spelling (entry and exit points, local and internal transitions, state-machine redefinition), a behavior shape the notation cannot bind, or a shape this project's lowerer refuses | 39 |

A test using any construct with no spelling or no translation is not expressible whatever else
it uses; otherwise `terminate` wins over the extensions, and the extensions over standard. The
alignment note was first written with a hand count of 37 / 33 / 3 / 30; the classifier is the
record from now on, and the note's test-suite section carries its figures. Nine tests moved
from the hand count when the emitter was written; each is listed with its reason in the note
under [Moves from the hand count](../internals/design/precise-semantics-alignment.md#moves-from-the-hand-count):

- **Entry, exit or do behaviors with parameters** that read the triggering event's data:
  *Event 017-B*, *Event 019-B*, *Event 019-C*, *Event 019-E*. The notation binds event data on
  the transition (`accept d : Data`), never on an `entry`, `exit` or `do` action.
- **An operation the tester calls and whose result it traces**: *Event 019-D*, *Event 019-E*,
  *Deferred 007*. The runtime's call events carry nothing back to the caller, and only the
  target's behaviors write the model's `log`.
- **A `trace(...)` in the tester's own behavior**: *Event 019-A* (and *019-D*, *019-E*).
- **A fork into orthogonal regions that have no initial pseudostate**: *Fork 002*, *Join 001*
  — kept apart from the rest, see [Findings about our own conformance](#findings-about-our-own-conformance).

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
- `terminate` is emitted as written, so the three terminate tests are spellable models that
  the runtime refuses at run time.

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
exhausts the exploration budget, is a failure whose reason names the error. Each run is bounded
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
| `terminate-gap` | the classifier found `terminate`; the test is spellable and waits on the runtime executing it |
| `differs-by-design` | the test would be a `fail`, **and** the committed table `internal/pssm/rows.go:TestRows` maps it to a note row whose verdict is *differs because v2 differs* |

`differs-by-design` is never inferred from a failure: the table is written by hand from the
note, names the row, and is reviewed with every change to it. A test the table maps to a
*differs, v2 silent* row stays a `fail` when it fails and cites the row either way, so the bucket
lists say which failures are second opinions on a tool choice. Of the six *differs because v2
differs* rows, only SM15 (a do activity and the machine competing for one occurrence) is
reached by an expressible test; SM36 and SM37 (local and internal transitions) have no
spelling and their tests are not expressible, and A12, C6 and C9 are action and composite-structure
rows no state-machine test exercises. Of the eight *differs, v2 silent* rows, five are reached
by expressible tests and cited below: SM7, SM11, SM28 and SM30 by passes, since the rules the
note decided on them landed, and SM32 by a failure; SM45 (destroying a performing object), C3
(multi-valued connector ends) and C8 (a send with no receiver) are not state-machine rows and no
test in the suite reaches them.

## Baseline

Recorded **2026-09-15** on develop commit **`bcc6b13e0ab9fa3814f183c2e8f2cf33b68adb03`** with the history-record fix
(finding 7), the emitter's initial-effect fix and the active-ancestor fix described below, as
`docs/project/pssm-referee-baseline.json`; regenerate with `go run ./cmd/pssm-referee -update`,
check with `-check`. The counts are the gate; the rows are for whoever adjudicates a moved count.
The figures below are as measured when this record was last updated and are not the current
baseline — `go run ./cmd/pssm-referee` prints the current ones.

| Bucket | Tests |
|---|---:|
| `pass` | 42 |
| `fail` | 17 |
| `not-expressible` | 39 |
| `terminate-gap` | 3 |
| `differs-by-design` | 2 |
| **Total** | **103** |

### Movements since the previous baseline

The previous baseline (develop `f4b844329`, 2026-09-14) counted 41 `pass` and 18 `fail`. One
change moves it: a transition from a substate into the composite state enclosing it no longer
re-enters that state (runtime, `state_executor.go:completeInto`; the SM35 row of the alignment
note records the rule). One test moved `fail` → `pass` and none the other way.

| Test | Row | Movement | Adjudication |
|---|---|---|---|
| Transition 011 C | SM35 | `fail` → `pass` | Runtime defect. `T1.3` leaves `S1.2` for `S1`, the composite enclosing it and already active. The run exited `S1.1` and `S1.2`, ran the effect, then started `S1`'s default substate again without exiting or entering `S1` — an entry sequence neither UML nor SysML v2 admits: UML's external transition either exits and re-enters `S1` or, as PSSM §8.5.8 reads it, does not enter a target that is already active and completes the source's region instead ("the RegionActivation owning the sourceVertexActivation completes"); SysML v2 §7.18.3 activates a target on the transition and nowhere restarts an active one. The runtime now follows PSSM: `S1`'s region is complete once `S1.2` is left, `S1` completes, `T2` fires and `S1(exit)` is logged, the admitted trace. `state_transition_into_active_ancestor`, `state_transition_into_active_parallel_ancestor` pin it |

### Movements before that

The baseline of develop `f4b844329` followed one (develop `bb95cf226`, 2026-09-12) that
counted 36 `pass` and 23 `fail`. Two changes move it: the runtime fix of [finding 7](#findings-about-our-own-conformance) (a
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
exits). Their rows below quote the new reasons.

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

### `pass` (42)

Behavior 001, Behavior 002, Behavior 003 B, Transition 001, Transition 007, Transition 011 C,
Transition 015, Transition 016, Transition 020, Transition 022, Event 001, Event 002, Event 008, Event 009,
Event 010, Event 016 A (reports on SM11), Event 016 B, Event 017 A, Event 018, Entering 004,
Entering 005, Exiting 002, Exiting 005, Choice 001 and Choice 002 (report on SM30), Choice 003,
Choice 004, Final001 (reports on SM11), Deferred 001, Deferred 002, Deferred 003 (reports on
SM7), Deferred 004 A and Deferred 004 B (report on SM7), Deferred 005, Deferred 006 A (reports
on SM15: the runtime's rule and PSSM's agree on this variant), History 001-A, History 001-B,
History 001-D, History 002-A, History 002-C (reports on SM28), History 002-D, Junction 001.

### `differs-by-design` (2)

| Test | Row | Admitted trace not reached |
|---|---|---|
| Deferred 006 B | SM15 | `S2(doActivityPartI)::S2(doActivityPartII)` — PSSM gives the deferred occurrence to the do activity alone; the runtime dispatches it to every scope |
| Deferred 006 C | SM15 | `S1.2(doActivity)::S1.1(doActivity)` — the same rule |

### `terminate-gap` (3)

Terminate 001, Terminate 002, Terminate 003 — each reaches `S1.Terminate1`; they move to
`pass` or `fail` when the runtime executes `terminate` (alignment finding 1).

### `fail` (17)

One failure cites a note row through the committed table. The other sixteen are
**unadjudicated**: fails, not yet attributed to a translation defect, a runtime defect, or a
missing alignment row. The referee records them; it does not diagnose them, and none of them is
a finding against the runtime until someone adjudicates it.

#### Citing a note row (1)

| Test | Row | What the run shows |
|---|---|---|
| Junction 002 | SM32 | run error: no outgoing guard of the junction holds; PSSM disables the compound transition and admits `T3(effect)` |

#### Unadjudicated (16)

One line per test, from the baseline's `reasons`: what the run reached that the suite does not
admit (`—` when every reached trace is admitted and the failure is only a missing one), and
what PSSM admits that the run never reached. Where PSSM admits several interleavings one is
quoted and the number given; `∅` is the empty `log`. Enough to adjudicate without re-running the
suite; the full sets are in the baseline file.

| Test | Reached, not admitted | Admitted, not reached |
|---|---|---|
| Behavior 003 A | — | `S1(entry)` (the machine completing before the do activity's first segment) |
| Transition 017 | — | `T2(effect)::S1(entry)::S3.1(doActivity)::T3.1.2(effect)::T2.2(effect)::T3.2(effect)` and 6 more interleavings of `S3.1(doActivity)`, `T2.2(effect)`, `T3.1.2(effect)` (the eighth admitted order is reached) |
| Transition 019 | `S1.1(exit)::T1.2(effect)::S2.1(exit)::T2.2(effect)::T1.3(effect)` and the mirror ending `T2.3(effect)` | `S1.1(exit)::S2.1(exit)::T1.2(effect)::T2.2(effect)::T1.3(effect)::T2.3(effect)` and 5 more, all containing both `T1.3(effect)` and `T2.3(effect)` |
| Event 015 | — | `T1.3(effect)` |
| Entering 010 | — | `S1(entry)::T2.1(effect)::S1.1(entry)::S2.1(entry)` and `S1(entry)::T2.1(effect)::S2.1(entry)::S1.1(entry)` (the third admitted order is reached) |
| Entering 011 | — | `S1(entry)::T1.1(effect)::S1.1(entry)::T2.1(effect)::S1.2(entry)` and 4 more orders of the two regions' initial effects and entries (the sixth admitted order is reached) |
| Exiting 001 | — | `S1.1.1(exit)::S2.1(exit)::S1.1(exit)::S1(exit)` and `S2.1(exit)::S1.1.1(exit)::S1.1(exit)::S1(exit)` (the third admitted order is reached) |
| Exiting 003 | — | `S1.2.1(exit)::S1.1.1(exit)::S1.1(exit)::S1(exit)` (the other admitted order is reached) |
| Choice 005 | `T2(effect)::S1(entry)::S1.1(entry)` | `T1.2(guard)::T1.3(guard)::T2(effect)::S1(entry)::T1.4(guard)::T1.5(guard)::S1.1(entry)` |
| Join002 | `S1(exit)::T2.2(effect)::T3(effect)::S2(entry)` | `T1.2(effect)::T2.2(effect)::S1(exit)::T3(effect)::S2(entry)` and the order with the first two swapped (`T1.2(effect)` never runs, and `S1(exit)` precedes the effects of the join's incoming segments) |
| Join003 | run error: `join Join1: eval guard of transition Join1 -> S2: no value for feature value` | `T1.2(effect)::T5(effect)` and `T1.4(effect)::T5(effect)` |
| History 001-C | — | `S1(entry)::S1.1(exit)::S1.2(entry)::S2.2(entry)::S2.2.1(exit)::S2.2.2(entry)::S1(exit)::S1(entry)::S1.1(exit)::S1.2(entry)::S2.2(entry)::S2.2.2(entry)::S1(exit)` and 10 more orders of the two regions' entries and exits (the twelfth admitted order, the one the PSSM text prints, is reached) |
| History 002-B | — | `…::S1(exit)::T3(effect)::S1(entry)::S1.1(exit)::S1.2(entry)::S2.2(entry)::S2.2.1(exit)::T2.2.2(effect)::S2.2.2(entry)::S1(exit)` and 4 more orders of the two regions' entries and exits (the sixth admitted order is reached) |
| Junction 003 | — | `T1.4(effect)::T1.4.1(effect)::T1.8(effect)` (the other admitted path, through `T1.3`, is reached) |
| Junction 004 | run error: `junction S1_Junction1_2: no guard evaluated to true` (the junction in `S1`'s second region, both of whose outgoing guards are false) | `T3(effect)` |
| Junction 005 | `T1.3(effect)::S1(entry)::T2.1(effect)::S2.1(entry)::S1.2(exit)::S1(exit)` | `S1(entry)::T1.3(effect)::T2.1(effect)::S2.1(entry)::S1.2(exit)::S1(exit)` and 2 more orders of `T1.3(effect)`, `T2.1(effect)`, `S2.1(entry)`, all after `S1(entry)` |

Every reason in full — each extra trace, each missing trace, each error — is in the baseline
file's `reasons`.

### `not-expressible` (39)

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
- **lowerer refuses fork into a region without an entry transition** (ours): Fork 002 and
  Join 001 as the only construct; also present in Entry 002 E, Transition 023 and Standalone
  002, which are not expressible on other grounds too.

A test with several such constructs is listed under each; the baseline file names every
test's constructs in its `reasons`.

## Findings about our own conformance

The referee's classifier and runs surfaced two gaps that are this project's rather than SysML
v2's, recorded here as candidates when the referee was added; the second is fixed, the first
is not:

- **The lowerer refuses a fork into orthogonal regions that have no initial pseudostate**
  (*Fork 002*, *Join 001*; alignment finding 6). UML lets a fork's outgoing transitions enter
  states inside a composite state's orthogonal regions directly, with no initial pseudostate in
  those regions; SysML v2 `parallel` regions can spell the shape and this project's `fork`
  extension can spell the fork. `lower.ToStateGraph` refuses it ("region `<name>` has no initial
  state; write `entry; then <state>;` inside the region") because it requires every region to
  name its own start even when a fork is the only way in. The two tests are filed
  `not-expressible` under the distinct reason *lowerer refuses fork into a region without an
  entry transition* so they are never confused with the constructs v2 has no spelling for; a fix
  moves them into the expressible buckets and the count moves with them.
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

The sixteen unadjudicated `fail` rows are not findings yet. Each is still to be attributed
one by one — to a translation defect (the referee lost a construct), a runtime defect (the extra
trace shows behavior UML and v2 both forbid), or a missing alignment row (v2 legitimately
differs and the note has no row for it yet) — and the attribution belongs in the change that
moves the row.

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
