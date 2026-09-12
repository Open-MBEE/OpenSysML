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
exhausts the exploration budget, is a failure whose reason names the error. `-jobs n` explores
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
and cited by the failures below (SM7, SM11, SM28, SM30, SM32); SM45 (destroying a performing
object), C3 (multi-valued connector ends) and C8 (a send with no receiver) are not state-machine
rows and no test in the suite reaches them.

## Baseline

Recorded **2026-09-12** on develop commit **`fb034e81736e226593b3da93dd6dc41aa03abd2b`**, as
`docs/project/pssm-referee-baseline.json`; regenerate with `go run ./cmd/pssm-referee -update`,
check with `-check`. The counts are the gate; the rows are for whoever adjudicates a moved count.
The figures below are as measured when this record was last updated and are not the current
baseline — `go run ./cmd/pssm-referee` prints the current ones.

| Bucket | Tests |
|---|---:|
| `pass` | 25 |
| `fail` | 34 |
| `not-expressible` | 39 |
| `terminate-gap` | 3 |
| `differs-by-design` | 2 |
| **Total** | **103** |

These counts record the runtime as it stands on that commit. The alignment note's four decided
rules (SM7 deferral, SM11 composite completion, SM28 empty history, SM30 dynamic choice guards)
are being implemented separately; when they land, the tests below that cite those rows are
expected to move and the counts are re-adjudicated then, not predicted here.

### `pass` (25)

Behavior 001, Behavior 002, Behavior 003 B, Transition 001, Transition 007, Transition 015,
Transition 016, Transition 020, Transition 022, Event 001, Event 002, Event 008, Event 009,
Event 010, Event 016 B, Event 017 A, Event 018, Entering 004, Entering 005, Exiting 002,
Exiting 005, Deferred 001, Deferred 002, Deferred 005, Deferred 006 A (reports on SM15: the
runtime's rule and PSSM's agree on this variant).

### `differs-by-design` (2)

| Test | Row | Admitted trace not reached |
|---|---|---|
| Deferred 006 B | SM15 | `S2(doActivityPartI)::S2(doActivityPartII)` — PSSM gives the deferred occurrence to the do activity alone; the runtime dispatches it to every scope |
| Deferred 006 C | SM15 | `S1.2(doActivity)::S1.1(doActivity)` — the same rule |

### `terminate-gap` (3)

Terminate 001, Terminate 002, Terminate 003 — each reaches `S1.Terminate1`; they move to
`pass` or `fail` when the runtime executes `terminate` (alignment finding 1).

### `fail` (34)

The rows are cited where the committed table maps the test; the other failures are on
constructs UML and v2 agree on, and each is a candidate runtime or translation defect to
adjudicate — the referee records them, it does not diagnose them.

| Test | Row | What the run shows |
|---|---|---|
| Deferred 004 A | SM7 | reached `S2.1(exit)::T2.2(effect)::S1.1(exit)::T1.2(effect)`: the sibling region's transition takes the occurrence `S1` defers, and `S1(exit)::T4(effect)` never follows; PSSM admits `S1.1(exit)::T1.2(effect)::S2.1(exit)::T2.2(effect)::S1(exit)::T4(effect)` |
| Deferred 004 B | SM7 | the same shape one level deeper: reached `S2.1(exit)::T2.2(effect)::S1.1.1(exit)::S1.1(exit)::T1.1.2(effect)`, admitted `S1.1.1(exit)::T1.1.2(effect)::S1.1(exit)::S2.1(exit)::T2.2(effect)::S1(exit)` |
| Final001 | SM11 | reached `S1.1.1(exit)::S1.1(exit)::T1.1.2(effect)::S2.1(exit)` without `T1.2(effect)`, the composite state's own completion transition; PSSM admits `S1.1.1(exit)::T1.1.2(effect)::S1.1(exit)::T1.2(effect)::S2.1(exit)` |
| History 001-A | SM28 | run error: the deep history has no default transition and `S1` no recorded configuration; PSSM enters the region's initial pseudostate |
| History 002-C | SM28 | the same, shallow history |
| Choice 001 | SM30 | reached an empty `log`; PSSM admits `T4(effect)` four times over, the choice's guards reading what the incoming effects wrote |
| Choice 002 | SM30 | reached an empty `log`; PSSM admits `T3(effect)`, `T4(effect)` or `T5(effect)` |
| Junction 002 | SM32 | run error: no outgoing guard of the junction holds; PSSM disables the compound transition and admits `T3(effect)` |
| Behavior 003 A | — | reached `S1(entry)::S1(doActivityPartI)` only; PSSM also admits `S1(entry)`, the machine completing before the do activity's first segment |
| Transition 011 C | — | after `T1.3(effect)` the run re-enters and re-exits `S1.1` and `S1.2` instead of exiting `S1`; PSSM admits `…::T1.3(effect)::S1(exit)` |
| Transition 017 | — | one trace reached, stopping before `T3.2(effect)`; none of the eight admitted interleavings of the do activity and the two regions' effects |
| Transition 019 | — | two traces reached, each lacking the second region's `T1.3(effect)` or `T2.3(effect)`; none of the six admitted |
| Event 015 | — | one of the two admitted traces reached; `T1.3(effect)` is not |
| Event 016 A | — | reached `T1.2(effect)`; PSSM admits `T1.2(effect)::T3(effect)` |
| Entering 010 | — | reached one trace in which `T1.1(effect)::S1.1(entry)` occurs twice; none of the three admitted region-entry orders |
| Entering 011 | — | reached one trace in which `T1.1(effect)` and `T2.1(effect)` occur twice; none of the six admitted |
| Exiting 001 | — | one of the three admitted exit orders of nested orthogonal regions reached; the other two never |
| Exiting 003 | — | one of the two admitted exit orders reached; the other never |
| Choice 003, Choice 004 | — | reached an empty `log`; PSSM admits `T4(effect)` |
| Choice 005 | — | reached `T2(effect)::S1(entry)::S1.1(entry)` with none of the guards' own segments (`T1.2(guard)`, `T1.3(guard)`, …) PSSM admits |
| Join002 | — | reached `S1(exit)::T2.2(effect)::S2(entry)`; PSSM admits `T1.2(effect)` and `T2.2(effect)` in either order, then `S1(exit)::T3(effect)::S2(entry)` |
| Join003 | — | run error: the guard out of the join reads `value`, which the carried payload does not supply |
| Deferred 003 | — | reached `S1.1.1(exit)::S1.1(exit)::T1.1.2(effect)` and stops; PSSM admits `S1.1.1(exit)::T1.1.2(effect)::S1.1(exit)::T1.2(effect)::S1.2(exit)::T1.3(effect)` — the effect runs after the exits and the deferred occurrences are then dispatched |
| History 001-B | — | the reached trace lacks `T1.4(effect)` after the first `S1(entry)` |
| History 001-C | — | one trace reached, stopping after the restore at `S2.2(entry)::S2.2.2(entry)` without `S1.1(exit)::S1.2(entry)::…::S1(exit)`; none of the twelve admitted |
| History 001-D | — | run error: `DeepHistory1` is declared outside the composite state it restores |
| History 002-A | — | after `T3(effect)::S1(entry)` the run re-runs `S1.1(exit)::S1.2(entry)` that PSSM's shallow-history restore skips |
| History 002-B | — | after `T3(effect)::S1(entry)` the run re-runs `S2.1(exit)::S2.2(entry)` and stops before `S1(exit)`; none of the six admitted |
| History 002-D | — | reached `…::S1(exit)::S1(exit)::T1.2(effect)` and stops; PSSM admits `…::S1(exit)::T1.2(effect)::S1(exit)::T3(effect)::S1(entry)::T1.3(effect)::S1.2(entry)::S1(exit)` |
| Junction 001 | — | reached `S1(entry)::T1.1(effect)::S1(exit)`; PSSM admits `T1.2(effect)` between them |
| Junction 003 | — | reached `T1.6(effect)` alone; PSSM admits two chains of effects ending in `T1.6(effect)` or `T1.8(effect)` |
| Junction 004 | — | reached `S1(entry)::T1.1(effect)::S1.1(entry)::T2.1(effect)::T1.1(effect)::S1.2(exit)`; PSSM admits `T3(effect)` |
| Junction 005 | — | reached the same trace as *Junction 004*, the junction's guard reading the payload before the effect writes it; none of the three admitted |

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

The referee's classifier and runs surfaced one gap that is this project's rather than SysML
v2's, recorded here as a candidate and **not fixed** in the change that added the referee:

- **The lowerer refuses a fork into orthogonal regions that have no initial pseudostate**
  (*Fork 002*, *Join 001*; alignment finding 7). UML lets a fork's outgoing transitions enter
  states inside a composite state's orthogonal regions directly, with no initial pseudostate in
  those regions; SysML v2 `parallel` regions can spell the shape and this project's `fork`
  extension can spell the fork. `lower.ToStateGraph` refuses it ("region `<name>` has no initial
  state; write `entry; then <state>;` inside the region") because it requires every region to
  name its own start even when a fork is the only way in. The two tests are filed
  `not-expressible` under the distinct reason *lowerer refuses fork into a region without an
  entry transition* so they are never confused with the constructs v2 has no spelling for; a fix
  moves them into the expressible buckets and the count moves with them.

The `fail` rows that cite no note row are candidates of the same kind, still to be adjudicated
one by one: a run whose extra trace shows the runtime doing something UML and v2 both forbid is
a runtime finding, one whose translation lost a construct is a referee finding, and the
adjudication belongs in the change that moves the row.

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
