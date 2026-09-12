# Scheduling policies, choice points and exploration

How the runtime resolves what the Kernel Semantic Library leaves unordered, how it reports each
such resolution without changing the run, how a driver asks for another one, and how every one
within a bound is enumerated. The behavior a user sees is in
[the guide](../../guide/06-behavior.md#when-a-model-has-more-than-one-valid-run); which openings
are the library's and which are the tool's is derived case by case in
[the semantic oracle](../../project/behavior-semantic-oracle.md).

## The problem this answers

A succession is a `HappensBefore` link between whole occurrences (`Occurrences.kerml`); the
library orders nothing two such chains do not connect. Fork branches, the reactions of sibling
regions to one event, two enabled transitions out of one state, several holding guards of one
decision, and two performances due at one instant of the clock are all unordered, so one model
admits several complete outcomes and every one of them conforms. The executor runs exactly one
linearization. Until that linearization is named and reported, a conformance case can only pin it,
and a test that pins it cannot tell an admissible alternative from an executor bug.

The design separates four things that were one: the set of outcomes a case admits, the
linearization a run took, the points at which it had a choice, and the rule it chose by.

## Choice points (`choice.go`, `action_choice.go`)

A `ChoicePoint` is one point where an executor had several enabled alternatives the library leaves
unordered and took one by its scheduling rule. `ChoiceKind` names the six:

| Kind | Where it is noted | Alternatives, canonically |
|------|-------------------|---------------------------|
| `ChoiceTokenOrder` | `ActionExecutor.noteTokenOrder` | the tokens that could act in the step, by ID; the one stepped first is taken |
| `ChoiceDecisionBranch` | `ActionExecutor.noteDecisionBranches` | the successions whose guards hold, by declaration position |
| `ChoiceWriteOrder` | `stepWriteLedger.noteChoices` | the tokens that wrote one feature in one step; the write that stood is taken |
| `ChoiceTransition` | `StateExecutor.chooseTransition` | the transitions one event enables out of one state, by declaration position |
| `ChoiceRegionOrder` | `StateExecutor.chooseRegion`, drawn by `dispatchInOrder` | the states whose transitions one occurrence selected, by name in declaration order; the one fired first is taken |
| `ChoiceDueOrder` | `Context.runDue` (`advance.go`) | the executors due at one instant, in creation order; the one run first is taken |

Two rules hold at every site:

- **Recording a choice never alters the run.** The executor resolves the point exactly as the
  policy says and then describes what it resolved. To *know* a decision had a second holding
  guard, every guard is evaluated, not just up to the first that holds; a later guard that cannot
  be evaluated is recorded as an `UnevaluableGuard`, not a failure — a guard with no result is not
  true, so its succession is simply not selected, which is the library's reading. A first guard
  that errors still fails the run as it always did.
- **A choice is a fact about the run, not a fault in the model.** `ChoicePoint.Diagnostic()` is
  informational with code `choice-point`; `UnevaluableGuard.Diagnostic()` is informational with
  code `guard-unevaluable`. Both are `RunNote`s.

`Context.note` appends a note to the run's `runState.notes` and, when tracing, writes its
`String()` to the trace at the point it was made (`choice …`, `unevaluable guard …`). A probe —
a preview of what a run would do, bracketed by `beginProbe` — notes nothing, since it is not a
run. `Context.Notes`, `Choices` and `UnevaluableGuards` read them back; the executors' `Notes` and
`NoteCount` let a caller driving a run call by call see what one call noted, which is how the REPL
ends `%step`, `%continue` and `%advance` with a count.

Region order is drawn per dispatch, not per event: `dispatchInOrder` draws among the candidates
still active before each firing, since a reaction may leave a sibling's leaf, and both the
broadcast of a queued event and the polling of change triggers (`state_change_trigger.go`) go
through it. The choice is labelled by the occurrence dispatched, not by the trigger of whichever
region was drawn first, so the label is the same under every policy.

## Policies (`scheduler.go`)

A `SchedulePolicy` is parsed from one spelling and printed back to it:

| Spelling | Token order in a step | `pick` (branch, transition, region) | `pickDue` |
|----------|----------------------|--------------------------------------|-----------|
| `reverse` (default, zero value) | reverse spawn order | first in declaration order | last created |
| `declared` | spawn order | first in declaration order | first created |
| `seed:<n>` | shuffle of the tokens not parked | uniform draw | uniform draw |
| `replay:<file>` | the witness's `step n: …` line, then `reverse` | the witness's line, then `reverse` | the witness's line, then `reverse` |
| `explore[:runs=N,depth=D]` | the exploration's plan | the exploration's plan | the exploration's plan |

`reverse` is exactly what every run did before policies existed, so every `.expected.json` and
`.trace.golden` recorded before them still holds unchanged; that is the invariant the whole design
is built to keep. Under `reverse` and `declared` a region-order pick is declaration order and is
still reported — a fixed policy resolves the choice, it does not remove it.

`SchedulePolicy.start` begins the resolutions of one run as a `scheduler`. A seeded one carries a
`math/rand/v2` PCG the run consumes draw by draw, so the same seed replays the same run on every
platform; `scheduler.mark` saves and restores that state around a probe, so previewing does not
move the generator. `Context.SetSchedule` sets the policy runs started from then on draw under; a
run already under way keeps the one it started with, and `explore` is refused with
`ErrExploreUndriven` because it is not a policy one context runs under (below).

The scheduler lives in the run's `runState` beside the budget and the notes. A run driven call by
call — a REPL `%action` or `%state` session — owns its `executorRun.state`, installed for each
call by `beginExecutorRun` whatever ran in between, so a seeded debugging session draws from its
own generator and an interleaved run neither consumes its draws nor inherits its notes.

`replay:<file>` (`replay.go`) is the policy a witness is run again under: the file's choice lines
— `ChoiceTaken.String` spellings, one per line up to the first blank line, so a checker's witness
file with a trace body after its header serves as it stands — are followed one move at a time,
each having to name the step the run is at and pick among the alternatives it offers, and once
they are spent the run continues as `reverse`. A line the run cannot follow, or one left over at
the end, is recorded as the run goes and reported by `Context.Unfollowed` as a `ReplayError`
naming the move, its choice and what the run faced; the run is never quietly turned into another
linearization. The model checkers replay every `sat` witness under it before claiming a
violation.

## Exploration (`explore.go`)

`explore` enumerates every linearization within a budget by replaying whole runs, each from a
fresh `Context`:

1. The first run records each choice point it reaches as an `exploreSlot` (a `pick` among `n`
   alternatives, or which of the tokens able to act a step tries next) and takes the first
   alternative at each.
2. `nextPrefix` walks the record backwards to the last slot with an untried alternative, keeps
   the record up to it with that alternative advanced, and the next run replays that prefix and
   takes first alternatives past it. This is a depth-first walk of the choice tree.
3. A replay that does not meet the choice points its prefix planned — a different number of
   alternatives, or tokens able to act the plan did not find so — is `ErrExplorationDiverged`,
   and no outcome set is reported, since one could not be trusted.
4. Runs stop when every alternative within depth is tried, or when the `runs` budget is reached;
   a slot resolved past the `depth` budget takes its first alternative and is not the
   exploration's to vary. Either bound reached makes the `Exploration` incomplete, and
   `Exploration.Status` says which; nothing is silently truncated. The default budget is
   `DefaultExploreBudget`, 1024 runs and 64 choice points per run.

A fresh context per run is what makes replay sound: instances, identities, the message bus, the
clock, object behaviors, calc memoization and the notes of one run cannot leak into the next.
`Context.beginExploration` installs the exploration's run as the scheduler every choice draws
from; `scheduler.describe` attaches the `ChoicePoint` the run reports to the slot it just resolved,
so a witness names alternatives exactly as the trace does.

Outcomes are keyed by `Outcome.identity` — outputs, `finalState`, state visits, or the error —
and grouped: each distinct outcome reports how many linearizations reached it and the choices of
the first run that did, its witness. A step's pick among tokens that then did not act is no
alternative (the slot is narrowed, and a run that repeats one already made is not counted twice).
`Explore` is the one driver of this policy; the CLI's `-schedule explore`, the wire `schedule`
field and the Python client all reach it, and a REPL debugging session refuses it because it steps
one run and cannot replay from the start.

## The conformance contract

Every element above has a test surface, documented for authors in
`internal/core/runtime/testdata/conformance/README.md` and summarised in
[testing](../testing.md):

- A case whose model admits several results lists them under `outcomes` with an `admissible`
  citation into the semantic oracle; a case with one result states it as before. The observed run
  must match exactly one listed outcome.
- `TestExecutionConformance` runs every case under the default policy.
  `TestExecutionConformanceUnderPolicies` runs the whole suite under `declared` and `seed:1`: a
  case with no `schedule` pin was recorded under the default and must hold under any policy, so
  one that differs has been pinning a scheduling artefact and fails rather than being skipped.
  `OPENSYSML_SCHEDULE_SEEDS=<n>,<m>,…` widens that sweep to further seeds for a local or
  scheduled run; `seed:1` stays in every run so a sweep is never opted out of.
- A case with `outcomes` is also explored (`exploreConformanceCase`): exploration must reach every
  listed outcome, reach nothing unlisted, and complete within the case's `exploreBudget`. A case
  without `outcomes` is not explored, on the expectation that it has one reachable outcome; when
  exploring it shows more, the fix is to derive its admissible set in the oracle and list it, not
  to pin the policy. A `schedule` pin of `reverse` says the case's result is one linearization,
  kept only until its admissible set is derived or the bug it pins is fixed.
- `TestExecutionTrace` checks a case's `.trace.golden` under the default policy and, for a case
  with `outcomes`, a `<case>.<policy>.trace.golden` under each sweep policy; a `.trace.order`
  states the partial order — `a < b` per line — a trace must respect, checked beside the golden
  or instead of one, and `TestTraceOrderViolationFails` proves a violated constraint fails.
- `explore_test.go` covers the enumeration itself: every outcome reached once, determinism,
  the one-run case with no choice points, each budget's incompleteness, an error as an outcome,
  a decision in a loop, transition conflict, sibling region order under every policy, and due
  order.

## What this is not

This is bounded enumeration of linearizations, not model checking:
[bounded model checking](bounded-model-checking.md) proposes snapshots at choice points and
partial-order reduction so that only one representative of each equivalence class of
interleavings is run. Exploration runs every linearization within budget and reports the
distinct outcomes it reached; it answers "which outcomes can this model produce, and by what
choices" and reports honestly when the budget stopped it, which is the question the conformance
harness asks. The reduction, the deadlock and requirement analyses across schedules, and the
exploration of value domains remain that proposal's.
