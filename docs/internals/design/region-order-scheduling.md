# Recording the order of orthogonal regions

How the entry and exit of a composite state's regions and the units of the firings one
occurrence selects across regions are choice points the schedule policy draws and `explore`
enumerates — the runtime gap the [alignment note](precise-semantics-alignment.md) lists under
its findings as *the order in which orthogonal regions are entered, exited and stepped is not a
recorded choice point*, and the [PSSM referee record](../../project/pssm-referee.md) measures.
The finding's four sites are implemented as this note describes: region entry (a composite
state's regions, a fork's branches, the regions a history restores), region exit, the units of
the firings across regions, and a due do step against the dispatch at the head of the pool —
the last at the grain of one token move of the do flow (see
[the do-step site](#the-do-step-site)). The gap found while
implementing the entry site, a completion's firing against the entry front (finding 11), turned
out on reading the admitted sets not to be one gap of the runtime, and its last section records
what it is instead.
The note extends [scheduling policies, choice points and exploration](scheduling.md), whose
vocabulary it uses throughout.

Every claim below about what PSSM admits was read off the referee's report for the test
(`go run -C tools ./cmd/pssm-referee -filter "<test>" -json`), not inferred from the specification; the
admitted sets are quoted where they decide a design point.

## The sites

| Site | Draw | Where |
|------|------|-------|
| Entering a composite state's regions | each region's units on a front under `ChoiceEntryOrder`; `declared` and `reverse` take declaration order | `state_region_entry.go:enterRegionsInto` → `enterRegions` → `performUnits` |
| Entering a fork's branches | one queue per branch — its effect, then the owner chain entered once by the first branch drawn, then its region — on a front under `ChoiceEntryOrder`; the owner's other regions join the front once the owner is entered | `state_region_entry.go:enterForkBranches` → `drawUnits` |
| Restoring a history | the restored regions' entries as the composite's regions above | `enterRegionsInto`, from the history's record |
| Exiting a composite state's regions | each region's exits, innermost first, on a front under `ChoiceExitOrder`; then the composite's own exit | `state_region_transition.go`, `state_executor.go:exitState` → `performUnits` |
| Firing the transitions one occurrence selects across regions | one queue per firing — source exit, effects, target entry — on a front under `ChoiceRegionOrder`, drawn one unit at a time | `state_executor.go:dispatchInOrder` → `openFront` |
| A due do step against the dispatch at the head of the pool | under `check`, `replay` and `explore` the machine runs one unit at a time: one token move of one due do behavior (drawn among the due ones by `chooseDoAction`), or the dispatch due when it would take its occurrence, drawn against the move under `ChoiceStepOrder`. The fixed policies run the whole round first (`runDoRound`, every due do action one sweep of its tokens), then the change-trigger poll, then the dispatch | `state_executor.go:oneUnit` → `stepDue`, `chooseStepOrder`; `runStep` for the fixed policies |

The model checker mirrors the last row: `check_moves.go:enabledMoves` offers the token moves of
the due do behaviors and the dispatch that acts together, one move each, as `oneUnit` draws
them, and the dispatch alone once no do behavior is due. Every run a fixed policy makes — the
whole round, then the dispatch — is one path of that enumeration, so a search that completes is
exhaustive ([bounded model checking](bounded-model-checking.md)). The other sites lie inside one move of the
checker (a dispatch or an entry), where the checker resolves the choice points the move draws
through its `picks`.

What the referee showed at each site before the change. The suite numbers its states; here a
state is named by where it sits: `top` is the parallel state, `r1`/`r2` the substate the
first/second region holds (`r1'` the one it moves to), `r1.1` a state nested inside `r1`, `deep`
the nested state whose do activity is logged. Transition names are the suite's own:

| Test | Admitted | Reached before | Missing shape | Now |
|------|----------|----------------|---------------|-----|
| Exiting 001 | 3 | 1 | `r2(exit)` before, between or after `r1.1(exit)::r1(exit)` | all reached, `pass` |
| Exiting 003 | 2 | 1 | `r1.1(exit)` and `r2.1(exit)` in either order | all reached, `pass` |
| Fork 002 | 4 | 1 | `top(entry)` after the first of `T2.1(effect)`, `T2.2(effect)` in either order — never first | all reached, `pass` |
| Terminate 001 | 2 | 1 | `r2(entry)` before `r1(entry)` | all reached, `pass` |
| Deferred 006 C | 2 | 1 | the two regions' do activities take one occurrence in either order | all reached, `pass` |
| Transition 019 | 6 | 4 | both sources' exits before either segment's effect | all reached; six unadmitted traces, the join's segment order (SM34), `fail` |
| Entering 010 | 3 | 1 | `r1(entry)` before, between or after `T2.1(effect)::r2(entry)` | finding 11: an initial transition's effect, a completion effect in v2 |
| Entering 011 | 6 | 1 | every interleaving of `T1.1(effect)::r1(entry)` with `T2.1(effect)::r2(entry)` | finding 11: as *Entering 010* |
| Junction 005 | 3 | 1 | `T1.3(effect)` before, between or after `T2.1(effect)::r2(entry)` | finding 11: as *Entering 010* |
| History 001-C | 12 | 1 | the two regions' completions and restored entries interleaved, twice over | finding 11: the pool's order, and the suite's defect |
| History 002-B | 6 | 1 | `r1(exit)::r1'(entry)` against `r2(exit)::r2'(entry)`, then `r2'.1(exit)::T2.2.2(effect)::r2'.2(entry)`, twice over | finding 11: the suite's defect |
| Behavior 003 A | 2 | 1 | `top(entry)` alone: the dispatch before the do activity's first step | the do-step site |
| Terminate 002 | 5 | 1 | `r2(entry)` before `r1(entry)`; the do activity's first segment against the terminating completion | one reached; the do-step site |
| Transition 017 | 8 | 1 | `deep(doActivity)` at any point among the completion effects, and two more (see *Transition 017*) | the do-step site, the pool's order (finding 11), and the suite's defect |

In every row the reached traces are admitted; the failure was exploration reporting itself
complete after one linearization.

## The unit model

### What the library orders

`StatePerformances.kerml` gives a `StatePerformance` three sub-performances in succession —
`entry then middle`, `middle then exit` — with `do` one of the `middle` steps, started before
the others start (`do.startShot then nonDoMiddle.startShot`) and otherwise concurrent with
them. That succession is on the do performance's *start*, not on its first action: a do
behavior that has begun and not yet reached its first logging action is exactly the state
*Behavior 003 A* admits when its dispatch leaves the state with nothing logged by the do
activity, so the do step against the dispatch is a free draw once the do has been registered
(`startDoAction`, at entry), which the runtime does before any occurrence is offered.
`TransitionPerformances.kerml` gives a `TransitionPerformance` two successions:
`transitionLinkSource then effect` and `effect then transitionLink.laterOccurrence` — the source
occurrence ends, then the effects, then the target occurrence begins; and a
`StateTransitionPerformance` adds `guard then transitionLinkSource.exit` and
`accept then transitionLinkSource.exit`. A succession is a `HappensBefore` between whole
occurrences; nothing in either file connects one region's chain to a sibling's, and SysML v2
§7.18.1 says only that parallel substates are "performed concurrently". So within one region the
library fixes a total order on the things a trace logs — a state's exit before the effect of the
transition leaving it, that effect before the target's entry, a nested state's exit before its
parent's — and across regions it fixes nothing.

The runtime's order at each site was one linearization of that partial order, so the traces it
reached conform; the referee's missing traces were the other linearizations. Recording a draw at
each point where two regions both have a next unit, and letting the policy take either, makes
every linearization of the partial order reachable and no other: that is the whole design, and
the admitted sets above are exactly the linearizations of the per-region chains they log (three
of one and two, six of two and two), with the two exceptions of *Transition 017* taken up below
and the five tests of finding 11, whose chains are not what the runtime's are.

### The unit

A **unit** is one performance the library orders against the others of its region and logs
separately:

- the **entry** of one state — its entry behaviors, `RecordStateEntry`, its activation
  (`activateState`), and the registration of its do behavior (`startDoAction`) — but not the
  entry of its own regions, which are units of their own;
- the **exit** of one state — `stopDoAction`, `RecordStateExit`, its exit behaviors, the
  history it records — but not the exit of the states inside its regions, which come before it
  as units of their own;
- the **effect** of one transition segment (`runEffects` over a `routeEffect`), the segment's
  owner entered ahead as the fix to the junction finding does;
- one **do step** (`stepDoAction`: one token move of a do behavior's flow — a body statement, an
  action of a do behavior given as actions, or one token of a nested perform's flow);
- one **dispatch** — the selection of an occurrence's transitions — which then decomposes into
  the exit, effect and entry units of the firings it selected.

A firing is therefore not atomic: its units are drawn one at a time against the units of the
sibling firings the same occurrence selected. This is the granularity decision, and the library
is what decides it — the successions above order a firing's own units and nothing orders them
against another region's — while *Transition 019*'s admitted set is the check: with atomic
firings the trace `r1(exit)::r2(exit)::T1.2(effect)::…` cannot occur, and PSSM admits it.
The alternative reading, that a firing is one `TransitionPerformance` and so one occurrence,
does not survive the library either: a `TransitionPerformance` *is* an occurrence, but its
`effect` sub-performances are occurrences too, and a `HappensBefore` from the performance as a
whole to a sibling's would have to be written somewhere; it is not. The alignment note records
the decision as its SM21 row.

The unit is the smallest thing a trace distinguishes, which is why nothing finer is drawn: two
entry behaviors of one state run in declaration order (`performEntry`), two effects of one
segment likewise, and a do step is one token move.

### Silent units

A unit that performs no behavior — the entry of a state with neither `entry` nor `do`, the exit
of one with neither `exit` nor `do`, the entry of a hidden owner that performs nothing — logs
nothing a sibling region could observe or be observed by, so the library's partial order does
not distinguish the linearizations that differ only in where it falls. Drawing it as an
alternative of its own would multiply linearizations without adding an observable one: *Event
016 B* (three firings across nested orthogonal regions whose exits and entries are all silent)
has some 320 000 unit-level linearizations and 1152 observable ones. So a silent unit **rides
with the performing unit beside it**: a queue whose head is silent is drawn as its next
performing unit, and `advance` runs the silent units up to and including that unit in one draw,
stopping early where a sibling's readiness changes so that `declared` keeps its sequence unit
for unit (`unitHead.silent`, `silentEntry`, `silentExit`, `unitFront.advance`). A queue holding
silent units only is drawn against a performing sibling as one alternative and runs to its end.
Exploration therefore counts linearizations of behaviors, not of bookkeeping, and the referee
runs the suite under `internal/pssm/run.go:DefaultBudget` — 4096 runs, past *Event 016 B*'s
1152 — where the runtime's default is 1024.

One entry that performs nothing is not silent: the entry of a state that completes as it is
entered — a completion transition out of it and nothing below to enter (`completesAtEntry`) —
generates a completion event, and the pool holds completion events in the order they were
generated (PSSM §8.5.9), so the order two such entries fall in is observed through the order
their completions dispatch. That entry is a drawn alternative under its own label (`entryHead`:
`l1(entry)` in `state_completion_pool_entry_order`), and its completion is queued as the unit is
performed; a state with a do behavior performs, so it is drawn already, and its completion still
waits for the behavior (`settleDoActions`). *Event 016 B*'s entries complete nothing, so its
count stands.

### The front

At each site the executor holds a **front** (`state_unit_front.go:unitFront`): one queue of
units per region (or per fork branch, or per firing), each queue a coroutine (`iter.Pull`)
running the region's existing entry, exit or firing code and yielding before every unit
(`unit`), and a loop (`drain`) that draws which queue advances while two or more have a unit
ready. A front lives within one move, so no snapshot ever holds one; the region's code is the
code that ran before the draw was recorded, with a yield at each unit boundary.

A queue's head is one of:

- a **unit** to perform, labelled as a PSSM trace spells it — `left(entry)`, `r(exit)`,
  `split->a(effect)`, `T2.1(effect)` — a state that shares its name with another region's
  qualified by its region (`stateName`, as `stateNames` qualifies a state visit);
- **shared** (`unitHead.shared`): a fork branch's next unit is "enter the owner" until a sibling
  branch enters it, at which point the head is dropped from every other queue — the shared
  ancestor is entered once, by whichever branch is drawn first, which is what *Fork 002*'s
  admitted set says (`top(entry)` follows the first branch effect, never precedes both);
- **waiting** (`until`): a queue parked until a condition holds — a fork branch waiting for the
  owner's entry a sibling is making, a firing waiting for its join's other segments — and not
  drawn until then;
- **void**: a firing disabled by a sibling's units (its source left, its guard now false) runs to
  its end performing nothing, last (`drainVoid`);
- **silent**, as above.

A queue also grows the front as it runs: entering a state whose own regions are orthogonal adds
those regions' queues to the front under way (`performUnits` → `spawnUnits`; the entering queue
parks until they are done where the site waits for them), so a nested parallel state's regions
are drawn against the outer regions as siblings, not as one unit — `state_region_entry_nested_front`'s
trace shows `next p(entry), q(entry), right(entry)` once `inner` is entered. A firing's units
open a front of their own only where two or more firings are selected; the front of a firing
accepts the entry and exit sites nested in it (`accepts`), so the regions a firing's target
enters are drawn under the firing's `ChoiceRegionOrder` at its `on <event>` label rather than
under a front of their own. A unit whose label the run learns only by performing it — the
first state a region's initial transition reaches — is drawn under the label of what is known
(`unitAhead`) and the draw prepays the unit's own.

With one queue ready there is no draw and nothing is recorded, as one executor alone due at an
instant is no choice. With two or more, the executor draws once per performing unit, not once
per queue: a draw taken does not commit the region to run to the end of its queue, since the
alternatives PSSM admits (and the library) interleave at every unit. A front every queue of
which waits on another is an error naming the site (`every region waits on another`); the
robustness cases of `robustness_region_order_test.go` cover it beside a replay line naming a
region the front does not hold, a firing-unit line naming a unit of no firing, an exit line
naming a state not being left, and a deep, wide front past the run budget reported incomplete
rather than hung.

## The choice kinds

Three kinds are added to `ChoiceKind`, and one existing kind draws at a finer grain:

| Kind | Site | `Where` | Alternatives, canonically | Taken |
|------|------|---------|---------------------------|-------|
| `ChoiceEntryOrder` | region entry, fork branch entry, history restore | `entering <state>` — the composite whose regions are entered; `fork <name>` for a fork's branches | the queues with a unit ready, in declaration order, each labelled by its next performing unit: `left(entry)`, `T2.1(effect)`, `split->a(effect)` | the queue advanced |
| `ChoiceExitOrder` | region exit | `exiting <state>` | the queues with a unit ready, in declaration order, each labelled by its next performing unit: `inner(exit)` | the queue advanced |
| `ChoiceRegionOrder` (existing) | firing units across regions, and the entries and exits nested in a firing | `on accept <event>`, `on change` — as `dispatchInOrder` labels the occurrence | the firings with a unit ready, by source state in declaration order, each labelled by its next unit: `l1(exit)`, `l1->l2(effect)`, `l2(entry)` | the firing advanced |
| `ChoiceStepOrder` | a due do step against the dispatch at the head of the pool | `at t=<instant>` | `do <state>` naming the due do behaviors in entry order, then `dispatch <event>` for the head of the pool once a message in flight is queued behind the events already there, so the message is named (`dispatch accept <signal>`) only when nothing is ahead of it (`dispatch change <condition>` for a change trigger risen, `dispatch` bare where two or more tied events whose dispatch acts leave the event to a draw of its own among them) | the unit run |

Canonical order is declaration order for regions, branches and firings — the order the runtime
took before the draws were recorded — so the first alternative taken at every draw reproduces
that run unit for unit. `ChoiceRegionOrder` keeps its name and its `Where` label, so the witness
and trace lines of the fixtures already recording it keep their text where one firing runs to
its end before the next begins, which under `declared` and `reverse` is always; its
`Describe` and `String` spell a firing-unit draw (`Where` beginning `on `) as `next …` and a
whole-firing draw among the states reacting as before.

Labels name the unit, not just the region, for two reasons: a witness names alternatives exactly
as the trace does (`exploreRun.describe`, `ChoicePoint.Describe`), and a unit label is what lets
a reader of the trace see the interleaving. Where two regions hold states of one name, the
label carries the region (`left.done(entry)`, `split->right.done(effect)`), so two alternatives
are never spelled alike.

### `%trace`

`ChoicePoint.Describe` has a case per kind, in the shape of the existing ones:

```
choice entering work: next left(entry), right(entry) (unordered; took left(entry) first)
choice fork split: next split->a(effect), split->b(effect) (unordered; took split->a(effect) first)
choice exiting work: next l(exit), r(exit) (unordered; took l(exit) first)
choice on accept Go: next l1(exit), r1(exit) (unordered; took l1(exit) first)
```

and, once the step-order site is implemented,

```
choice at t=0.0: next do top, dispatch AnotherSignal (unordered; ran do top first)
```

`%step`, `%continue` and `%advance` count them as they count every choice
(`2 choice points; %trace on to see them`), `%choices` lists them, and over gRPC and Connect each
is the informational `choice-point` diagnostic placed at the state or transition drawn. The
self-model's `choiceKindCount` counts nine kinds.

### Witness lines and replay

`ChoiceTaken.String` writes the order kinds as the region and due orders are written,
`<where>: <took> first of <alternatives>`:

```
entering work: right(entry) first of left(entry), right(entry)
fork split: split->b(effect) first of split->a(effect), split->b(effect)
exiting work: r(exit) first of l(exit), r(exit)
on accept Go: r1(exit) first of l1(exit), r1(exit)
```

`parseOrderChoice` keys the kind off the `where` prefix (`t=` for the due order, the dispatch
label for the dispatch order): `entering ` and `fork ` are `ChoiceEntryOrder`, `exiting ` is
`ChoiceExitOrder`, `on ` is `ChoiceRegionOrder`. The step-order line, at `t=` like the due
order, is to be told apart by its alternatives' `do `/`dispatch ` prefixes, which an executor's
name never carries (the due order labels executors `action <name>` and `state machine <name>`).
A witness with a line at a site the runtime does not draw is refused with the existing
`ChoiceParseError`, so an old binary never runs a newer witness as another linearization.

Replay follows the lines one unit at a time. A line the run cannot follow — naming a region the
front does not hold, a unit of no firing, a state not being left — is a refusal: the typed
`ReplayError` (`ErrReplayRefused`) naming the line and what was enabled, made at the draw before
any unit of the site runs, and the move it lies in is undone whole: every site is drawn
inside a move that `moveWhole` marks (`moveMark` records the trace, the notes, the do behaviors
ended and the run's steps and elements, and rolls them back; `StateExecutor.capture` clones the
configuration, so the region states the units wrote go back with it) — the dispatch a firing
belongs to (`dispatchInOrder` wraps a front of two or more firings in `moveWhole`), the
transition whose travel enters or leaves the regions (`travelResolving`), the join's incoming
firings. The front itself needs no capture: it lives within the move and is closed with it, and
a rolled-back move re-enters from the move's start. The machine's start is no move, and a
refusal there is made before either region's entry runs, so nothing is there to undo.

### Exploration and the checker

`explore` needed nothing new: each draw is an `exploreSlot` with as many alternatives as ready
queues, the walk of prefixes in `exploreQueue` enumerates them, and a replay meeting a different
number of alternatives at a slot is `ErrExplorationDiverged` as before. The run is deterministic
because a draw's alternatives are computed from the front alone, in declaration order, and the
front from the model and the draws before it. `ExploreWith` on any number of jobs returns the
same `Exploration` for the same reason it does elsewhere: the queue of prefixes is in plan order
and the result keys on outcomes, not on which job ran them (`explore_queue_test.go` proves this
on every conformance case with an admissible set, and the fixtures below join that sweep by
having one; the referee's `TestRefereeDeterministic` proves it on the suite for any `-jobs`).

The checker resolves the three sites through the script a move draws from (`checkScript`,
`checkRun.choose`): a move that draws past its script takes the first alternative and reports
the draw (`checkRun.drawn`), and the checker enumerates the alternatives as it does a `choice`
pseudostate's branch. The machine's start draws too — the order its regions are entered in — and
is no move, so `Check` begins the invocation once per way of resolving the start's draws
(`searchFrom`, from `picks` of `nil` on) and searches from every configuration a start reaches;
the start's draws are the first slots of every witness. Every move stays one executor acting one
unit as `enabledMove` defines it; what grows is the pick sequence of a dispatch entering several
regions. In the reduction corpus (`testdata/check/reduction_expected.txt`) the two join models of
a parallel state grew with the draws — `por_state_join_exit`, three regions, from 12 states and
14 moves to 72 and 84; `por_state_join_guard`, two regions, from 12 and 14 to 24 and 28 — the
start's entry orders and the join's exit orders now being configurations of their own. Their
reduced counts equal their unreduced ones after as before: neither model had a reduction to lose.

The step-order site reaches the checker as `enabledMoves` following `oneUnit`: the due do
behaviors' token moves and the dispatch enabled *together* rather than the do round first, one move each, which is the
state-space reading of the same choice — a dispatch move's `picks` open with its index past the
do moves, so the checker's script resolves the draw as the runtime's replay does; the static
partial-order reduction (`check_reduce.go`) footprints a do step and a dispatch separately, so
two that commute are explored once. No run is left out: a dispatch may cut a sweep after any
token move, and the fixed policies' whole round is the path that takes every do move first.

## Policies at the sites

| Spelling | Entry, exit and firing-unit order |
|----------|-----------------------------------|
| `reverse` (default, zero value) | first alternative: declaration order — the run the runtime made before the draw was recorded |
| `declared` | first alternative — the same run |
| `seed:<n>` | uniform draw among the ready queues at every unit |
| `replay:<file>` | the witness's line, then `reverse` |
| `explore[:runs=N,depth=D]` | the exploration's plan |
| `check` (the checker's) | the scripted pick, first alternative past the script |

`reverse` and `declared` differ only in token order within an action step and in the due order
of executors (`scheduler.choose` takes the last alternative of a `ChoiceDueOrder` under
`reverse`, the first under `declared`); every other kind is taken first in canonical order under
both. The order kinds are taken first under both, so both keep the order the runtime always
took, and the invariant `scheduling.md` states — `reverse` is exactly what every run did before
policies existed — holds unit for unit: every `.expected.json`, every `.trace.order`, every
state visit and every output of every fixture is unchanged under both. What changed under both
is that each draw is *reported*: a fixed policy resolves the choice, it does not remove it, so
the trace golden of a fixture that enters or leaves a state with two or more regions, or whose
regions react to one event, carries one `choice` line per draw — the recording the alignment
note asks for — and the event order of every such golden is the order it had, which is checked
by diffing the goldens with the `choice` lines filtered out.

Under `seed:<n>` the draws take the seed's next values, so a seeded run of a fixture entering or
leaving an orthogonal state is a different run from the one before the draws were recorded: a
seed is a reproducible sample, not a pinned outcome (`scheduling.md`: the same seed replays the
same run on every platform, of one build). `TestExecutionConformanceUnderPolicies` runs every
conformance case under `declared` and `seed:1`, so a case whose `stateVisits` or log pinned the
declaration order under `seed:1` was pinning a scheduling artefact: the cases *about* the order
state the exact set of linearizations the library admits as `outcomes`, and the cases whose
subject is something else pin `reverse` with the reason stated
(`internal/exec/runtime/testdata/conformance/README.md`). The runtime showcase's spacecraft
session gains one draw entering `modes` (two regions, one unit each; the state is never left),
its README checkpoints regenerated; under `reverse` and `declared` the run itself, the t=79 race
included, is unchanged, and `TestSpacecraftShowcaseFrameCountAtLowPowerIsScheduleDependent` pins
it.

## Tractability

A front of `k` queues holding `u_1 … u_k` performing units draws at most `Σu_i − u_last` times,
where `u_last` is the length of the queue left when every other is empty; with equal queues of
length `u`, at most `(k−1)·u` draws, and the number of linearizations is the multinomial
`(Σu_i)! / Π(u_i!)`. Silent units do not count. The fixtures and tests concerned:

| Case | Queues | Draws at most | Linearizations |
|------|--------|---------------|----------------|
| two regions, one logged unit each (*Exiting 003*, `state_parallel_standard`) | 1, 1 | 1 | 2 |
| two regions, effect and entry each (*Entering 011*, `state_region_entry_order`) | 2, 2 | 2 | 6 |
| two firings, exit and effect each (*Transition 019*, `state_firing_units_interleaved`) | 2, 2 | 2 | 6 |
| *History 001-C*, two restores of two-unit regions (`state_history_restore_order`: 8) | 2, 2 twice | 4 | 12 (the two restores compound) |
| *Event 016 B*, three firings across nested orthogonal regions | — | — | 1152 |
| the showcase's `modes` (two regions, one start state each), entered once and never left | 1, 1 | 1 | 2 |

Every count but *Event 016 B*'s is within `DefaultExploreBudget` (1024 runs, 64 draws per run)
by two orders of magnitude; the referee's `DefaultBudget` of 4096 runs holds *Event 016 B*. A
model that nests orthogonal states three deep with three regions each, every unit performing,
would reach `9!/(3!)^3 = 1680` linearizations — beyond the default `runs`, reported as an
incomplete exploration, which is the existing behavior for any model past the budget and the
reason the budget is settable per conformance case (`exploreBudget`) and on the policy
(`explore:runs=N`); `robustness_region_order_test.go` exercises a deeper and wider front than
that past the run budget. The draws a fixture makes are its trace golden's `choice` lines, one
per draw, so growth is visible in review. The trace's cost is one `choice` line per draw, 34–83
bytes each (`environment.md`).

## The alignment row

The firing-granularity decision is the alignment note's SM21 row, *Firing order across
orthogonal regions*: `TransitionPerformances.kerml` orders a firing's own units —
`transitionLinkSource then effect`, `effect then transitionLink.laterOccurrence`, and
`accept`/`guard then transitionLinkSource.exit` — and places no succession between the units of
two performances in sibling regions; §7.18.1 has the regions performed concurrently; so a firing
is not atomic, `dispatchInOrder` draws the next unit among the firings with a unit left and
records each draw as a `ChoiceRegionOrder` labelled by the unit, and `declared` and `reverse`
take the firings whole in declaration order. The rows on entering and exiting regions (SM22,
SM23) cite `ChoiceEntryOrder` and `ChoiceExitOrder`; the row on the do activity (SM13) keeps
its caveat until the step-order site lands; the row on the join (SM34) keeps its verdict —
*Transition 019*'s six unadmitted traces are its: the suite pairs the join's segment order with
the effects' order (`T1.2` before `T2.2` has `T1.3` before `T2.3`), where the runtime draws the
join's order on its own, so every firing order reached brings a mirror the suite does not admit.

## The test contract

Conformance fixtures under `internal/exec/runtime/testdata/conformance/`, each with an
`outcomes` list citing the oracle, a default trace golden, and `.declared`/`.seed-1` goldens:

| Fixture | Pins |
|---------|------|
| `state_region_entry_order` | two regions with a logged entry and an initial-transition effect each; six outcomes (*Entering 011*'s shape) |
| `state_region_entry_order_uneven` | one region with one unit, one with two; three outcomes (*Entering 010*'s shape) |
| `state_fork_branch_order` | a fork's two branches into one parallel state's regions; the shared owner entered once, four outcomes (*Fork 002*'s shape) |
| `state_region_entry_nested_front` | a region's start state itself parallel, so entering it adds queues to the front; six outcomes |
| `state_region_exit_order` | nested and sibling exits interleaved; three outcomes (*Exiting 001*'s shape) |
| `state_history_restore_order` | a deep history restoring two regions; entries and exits interleaved, eight outcomes |
| `state_firing_units_interleaved` | two regions' transitions on one event, exit and effect each; six outcomes, joined afterwards so the join row's order stays visible (*Transition 019*'s shape) |
| `state_do_step_or_dispatch` | a do action due and a signal at the head of the pool; two outcomes, one where the dispatch exits the state before the do step ran (*Behavior 003 A*'s shape) |
| `state_do_step_among_completions` | a region's do step against the other region's completion effects, in either order of the two completions; six outcomes |
| `state_do_step_cuts_typed_do` | a do behavior given as a two-step action against a signal; the dispatch cuts it at either step or waits: two outcomes |
| `state_do_step_cuts_nested_perform` | an inline do body performing that action between two assignments; the dispatch cuts at any of the four moves: four outcomes |
| `state_do_step_cuts_control_node_body` | an inline do body forking through a fork with a body of its own; the fork's body is a move the dispatch may fall before or after, then either branch or both: five outcomes, the fixed policies' `1001` among them |
| `state_do_action_loop_timed_exit` | a looping do forking to two timed branches against a timed exit; the fixed policies' `left = right = 1` among four outcomes, the check hitting no bound |

`robustness_region_do_step_test.go` holds the step-order site's failure modes: a witness naming
a state with no due do step, a dispatch not at the head of the pool or a draw at a unit offering
none, each refused with the run rolled back; an endless do body under `explore` ending in
`ErrDoStepLimitExceeded`. `robustness_do_step_token_grain_test.go` holds the token grain's: a
do flow that never rests against a queued dispatch ends each run at the dispatch or at the
do-step budget, under `explore` and `check` alike; a `step order` line naming a do body parked
at an `accept` is refused. `state_do_action_loop_timed_exit` states the exact `outcomes` set
the fixed policies and the checker reach between them, and its `check.expected.json` states the
four values of a search that hit no bound.

`explore_test.go` covers each kind — every outcome reached exactly once, the count of draws,
determinism across runs — and `explore_queue_test.go`'s sweep on eight jobs covers the fixtures
by their `outcomes`. `internal/frontend/repl/explore_test.go` explores a region-entry model through the
REPL and checks the table (`TestRunStateMachineExploresEveryRegionEntryOrder`). `replay_test.go`
refuses a witness line at each site with the run rolled back to the move's start;
`robustness_region_order_test.go` holds the feature's failure modes.

## Transition 017

Of its eight admitted traces, six are the do step `deep(doActivity)` placed among
`T2.2(effect)`, `T3.1.2(effect)` and `T3.2(effect)` with `T3.1.2` before `T3.2` — the inner
leaf's completion before its parent's — which the unit model reaches once the do-step site and
the pool's order (finding 11, below) are in. Two have `T3.2(effect)` before `T3.1.2(effect)` and the do step
after `T3.2`: `deep`'s completion transition fires before the transition its inner region's
completion enables, and the do activity steps after its state was left. The suite's own comment
on the test's state machine ("Expected execution sequence") has `T3.2` fire when the completion
event `deep` generates is consumed, after the inner region's final state — the six, not the
two — so the two registered traces contradict the test's stated intent, and no reading of
`StatePerformances.kerml` orders a parent's exit before its region's completion transition. The
runtime does not produce them; the test stays `fail` on those two, the defect recorded in
[`omg-issues.md`](../../project/omg-issues.md#pssm-transition-017-admits-a-parents-completion-before-its-regions)
as the suite's rather than corrected in the downloaded XMI, and no `differs-by-design` row is
added for it.

## The do-step site

A do behavior is concurrent with the machine (`StatePerformances.kerml`, above), so while a do
step is due and an occurrence at the head of the pool is due at the same instant, "dispatch it
now" against "keep moving the do flow" are both runs the library admits. Under `check`, `replay`
and `explore` the machine runs one unit at a time (`state_executor.go:oneUnit`): while a do
behavior is due, the draw under `ChoiceStepOrder` is one token move of one due do behavior
(`stepDue` → `stepDoAction`, the one among several drawn by `chooseDoAction`) or the dispatch,
and the draw is made again after every move until no do behavior is due — a dispatch may cut a
do flow after any token move. `declared`, `reverse` and `seed:<n>` run the whole round first
(`runStep` → `runDoRound`, every due do behavior one sweep of its tokens) and dispatch after
it, so no default trace moves; `check` and `explore` enumerate every cut, so
the exhaustive set is a superset of every fixed policy's outcome. *Behavior 003 A*,
*Transition 017*'s do step among the completion effects and the cuts of *Deferred 006 C*'s two
do activities are what it reaches.

A dispatch is drawn against a due do step only where it would **take** its occurrence — fire a
transition, or let a do behavior already parked at an `accept` go on (`dueDispatch`,
`eventActs`, previewed and rolled back). One that would defer or drop it is not: neither is a
performance's acceptance, and the due do step may be the `accept` that takes the occurrence, so
the occurrence waits for the round to close as under the fixed policies. Without that rule the
draw spends an occurrence a do behavior is one action from accepting, a run no policy of the
runtime's makes and none the library orders (`state_join_completion_segment_waits_for_do_behavior`'s
`Tick`, `state_join_completion_is_not_a_timers_expiry`'s timer). Events tied at the head are
previewed one by one: those that act are the dispatch's alternatives against the step — one named
by itself, two or more as the bare `dispatch` whose dispatch order is then drawn among them alone —
and one that would be dropped is not, so a guarded trigger tied with an unguarded one does not hide
the unguarded one's dispatch behind the step (`state_do_step_or_tied_dispatch`).

The grain is the token move. A do behavior's run under these engines steps its tokens
(`bodyRun.steps`): the action executor draws one eligible token (`drawOneMove`), moves it, and
pauses the run there (`bodyPause.tokenStep`), to be resumed by the next draw that picks the
behavior; a nested perform's flow inherits the stepping, so a token move inside a called action
is a move of the outer do step too, and a body parked at an `accept` offers no move until its
occurrence is dispatched. One move is the same thing for an inline do body, a do behavior given
as an action, a loop's iteration, or a nested perform (`state_do_step_cuts_typed_do`,
`state_do_step_cuts_nested_perform`, `state_do_action_loop_timed_exit`). A move that only routes
control — through the initial, a fork, join or merge with no body, over unguarded, unweighted
successions — is not drawn: it settles around the drawn move (`settleSilentMoves`), since no other
move observes where between two of them it falls; a control node with a body of its own performs
that body as the token passes, so it is a move like any other and the dispatch is drawn on either
side of it (`state_do_step_cuts_control_node_body`). A run whose do
flow never rests against a queued dispatch ends at the dispatch or at the do-step budget
(`robustness_do_step_token_grain_test.go`). Under the fixed policies a do step is one sweep
(`stepSubflowSweep`) and nothing pauses within it.

## Finding 11: a pending completion inside the entry front

Found while implementing the entry site. Every admitted trace *Entering 010*, *Entering 011*,
*Junction 005*, *History 001-C* and *History 002-B* still miss interleaves the firing of a
**completion transition** — the initial transition of a region, translated as a completion out
of a start state (`T2.1(effect)` in *Entering 010*), or in the History tests the exit of the
state a region enters by default and its successor's entry — with the entry units of the sibling
region *in the same step*. The runtime dispatches a completion as a run-to-completion step of
its own once the entry move has settled (the alignment note's SM9 and SM10), so no draw among
the entry units reaches an order in which the completion fires before the sibling's entry; the
suite's registered traces admit both. One trace of *Terminate 002* has the same shape with a do
behavior's first segment in place of the completion's firing, and three of *Transition 017*
have it between two regions' completions.

### The rule as first written

The fix as first designed: a state whose entry leaves it complete — no do behavior, no regions
still active, a completion transition enabled — **offers its completion's firing as a unit of its
region's queue** on the front that entered it, drawn against the sibling regions' remaining units
under `ChoiceRegionOrder`, the firing's own units — exit, effects, target entry — following as
units of that queue; `declared` and `reverse` take it last, so the default order is the step the
runtime dispatches today and no golden moves; the offer is made only when the completion is
enabled by the state's own entry, never by a sibling's unit still to come.

That rule is **not implemented**, and should not be: checked against the admitted sets read off
the referee, it reaches every trace of the five and, on three of them, traces PSSM refuses — and
no narrower rule reaches the five either. This section records what the sets show, so the
finding is not re-derived as one gap when it is three.

### What the admitted sets show

Each test's move is a set of per-region chains — the entry units of the move, and the completion
firings a chain's states generate as they are entered (a firing being its source's exit, its
effect and its target's entry, the target's entry possibly generating the next). The five tests
have these chains, read off the translated models (`-keep`) and their `log` statements, the
states named as the sites table names them (`r2'.1` a state nested inside `r2'`):

| Test | Region 1 | Region 2 | Admitted |
|---|---|---|---|
| Entering 010 | `r1(entry)` | start state (silent) → completion `T2.1(effect)::r2(entry)` | 3 |
| Entering 011 | start state → completion `T2.1(effect)::r1(entry)` | start state → completion `T1.1(effect)::r2(entry)` | 6 |
| Junction 005 | entered explicitly through its junction: `T1.3(effect)`, then `r1` (silent) | entered by default: start state → completion `T2.1(effect)::r2(entry)`; `r2` complete → completion out of `top` to the machine's final state, `r1(exit)::top(exit)` | 3 |
| History 001-C, first half | `r1` (silent) → completion `r1(exit)::r1'(entry)` | `r2` (silent) → completion `r2'(entry)`, `r2'.1` (silent) → completion `r2'.1(exit)::r2'.2(entry)` | 2 |
| History 001-C, second half (the deep history is region 2's) | entered by default: `r1` (silent) → completion `r1(exit)::r1'(entry)` | restored: `r2'(entry)`, `r2'.2(entry)` | 6 |
| History 002-B, first half | as *History 001-C* | `r2` (silent) → completion `r2(exit)::r2'(entry)`, `r2'.1` (silent) → completion `r2'.1(exit)::T2.2.2(effect)::r2'.2(entry)` | 2 |
| History 002-B, second half (the shallow history is region 2's) | as *History 001-C* | restored: `r2'(entry)`, then its initial `r2'.1` (silent) → completion `r2'.1(exit)::T2.2.2(effect)::r2'.2(entry)` | 3 |

The History tests admit the cross product of their halves, twelve for *001-C* and six for
*002-B*; *001-C*'s first half alone has ten linearizations, of which two are admitted.

Enumerating the traces each candidate rule reaches on these chains, against the admitted sets:

| Rule | Entering 010 | Entering 011 | Junction 005 | History 001-C first / second half | History 002-B first / second half |
|---|---|---|---|---|---|
| Today: completions dispatched after the move, pool in declaration order (SM9, SM10) | 1 of 3 | 1 of 6 | 1 of 3 | 1 of 2 / 1 of 6 | 1 of 2 / 1 of 3 |
| After the move, pool in the order the sources were entered (the entry draw) | 1 of 3 | 2 of 6 | 1 of 3 | **2 of 2** / 1 of 6 | 1 of 2, **1 extra** / 2 of 3 |
| The earliest pending completion offered whole against the move's remaining units, completions in order among themselves | 2 of 3 | 2 of 6 | 2 of 3, **1 extra** | 2 of 2, **1 extra** / 3 of 6 | 2 of 2, **1 extra** / **3 of 3** |
| As above, the firing's units drawn one at a time against the move's | **3 of 3** | 2 of 6 | 3 of 3, **2 extra** | 2 of 2, **1 extra** / **6 of 6** | 2 of 2, **1 extra** / 3 of 3, **1 extra** |
| The rule as first written: a completion's units are units of its region's queue, drawn against anything | **3 of 3** | **6 of 6** | 3 of 3, **2 extra** | 2 of 2, **8 extra** / **6 of 6** | 2 of 2, **19 extra** / 3 of 3, **12 extra** |
| Today's rule, with an initial transition's effect as an entry unit of its region | **3 of 3** | **6 of 6** | **3 of 3** | (no initial effect) | (no initial effect) |
| Today's rule, the translation folding an initial transition's effect into its target state's entry action, ahead of that state's own entry behavior | 0 of 3, **2 extra** | 2 of 6 | no spelling: the initial transition ends at a junction | (no initial effect) | (no initial effect) |

The last two rows are spellings of the translation, not rules of the runtime; the last was run
by emitting the three tests with the fold in place of the start state and exploring them against
the admitted sets as the referee does.

No rule reaches every set, and the reason is that the five tests want three different things.

**The Entering and Junction tests are about the initial transition, not about completion.**
In UML the effect of a region's initial transition is part of the region's *entry*: the default
entry rule (UML 2.5.1 §14.2.3.4.5, "State entry continues from an initial Pseudostate via its
outgoing Transition"; PSSM's region activation enters by firing it) runs it as the last step of
entering the region, and the tests admit it anywhere against the sibling region's entry units
— the entry-unit row of the table, every set reached exactly, shows that the referee's admitted
sets are the linearizations of the entry chains when that effect is an entry unit. SysML v2 cannot
spell it: an entry transition (`entry; then s;`, §7.18.3 `EntryTransitionMember`) is a
succession from the owner's `entry` to a substate carrying a guard at most
(`GuardedTargetSuccession`), no effect and no trigger, so the referee translates the effect as
the effect of an unguarded completion transition out of a behavior-less start state
(`emit.go:startTarget`). That changes its scheduling class: a v2 completion transition fires by
a step of its own after the move (SM9), and the runtime's one trace is the one PSSM would give a
UML model in which `T2.1` really were a completion transition.

The two other spellings a v2 model offers were tried and fail, for reasons that are not the
tests'. Folding the effect into the entry action of the *region* (the owner of the initial
transition) performs it on every entry of the region, a history restore included, where UML's
restore bypasses the initial transition — the emitter defect *History 001-B* exposed, in the
referee record's translation-defects table. Folding it into the entry action of the *target*
state, sequenced before that state's own entry behavior so that the trace keeps the order
effect-then-entry, has the same defect one state down, and the tests show it (the table's last
row). In `StatePerformances.kerml` a state's `entry` is one step of its `StatePerformance`
(`step entry[1]`, before `middle` and `exit`), performed whenever the state is, and a
`StateTransitionPerformance` is a performance of its own between `transitionLinkSource.exit`
and the target's `entry`; the fold moves a behavior from the transition into the state, so it
runs on every way into the state and not only by the initial transition. *Entering 010*'s
region 1 is that case: its initial transition `T1.1` has an effect, and its target `r1` is the
state the tester enters explicitly, bypassing the initial transition, so the fold runs
`T1.1(effect)` on that entry and every trace reached is one the test refuses (`0 of 3`, two
extra). Where the target is entered by the initial transition alone the fold is faithful by
accident of the model, not by the spelling — the special case the rules above forbid. It also
merges two behavior executions into one unit: UML performs the effect and the target's entry as
distinct behaviors, and PSSM interleaves each against the sibling region, so *Entering 011*'s
six admitted orders split `T1.1(effect)` from `r2(entry)` around the sibling's units in four,
which one entry action, one unit of the front, cannot do (`2 of 6`, no extra). And it has no
target at all when the initial transition ends at a pseudostate: *Junction 005*'s `T2.1` ends at
a junction whose way out a guard decides at firing time, `r2` or its sibling state, so there is
no one state's entry to fold into. What the fold leaves alone is the case it was to be checked
against: `r2`'s completion out of `top` stays a completion dispatched after the front, so the
fold would reach no refused trace there — but it cannot be spelled there. The start state's
completion stays the referee's spelling: it keeps the effect a behavior of its own, run once,
on the initial transition alone, at the cost of the step boundary.

*Junction 005* pins the difference from the other side: `r2`'s completion is a real one,
leaving `top` through its exit, and every admitted trace has it *after* `T1.3(effect)`, the
sibling's remaining entry unit — so a rule that fires a completion inside the entry front
reaches two traces PSSM refuses. The only rule that reaches all three sets exactly is the
entry-unit row, and it is not a scheduling rule of the runtime: it is a fact about the
translation. A runtime rule that fired *some* completions in the
entry front and not others would have to tell a start state's completion from `r2`'s by the
source having no behavior, which v2 does not distinguish (a state with no entry, do or exit has
an empty `StatePerformance` and a completion like any other) and PSSM contradicts (its completion
event is dispatched after the step whatever the source performs). That is a special case of the
translation's shape, not a semantics.

**The first halves of the History tests are about the pool's order.** Both regions enter a
silent state whose completion is enabled at once. PSSM's pool holds the two completion events in
the order they were generated — §8.5.9, "a new `CompletionEventOccurrence` is placed into the
(ordered) `eventPool` behind any `CompletionEventOccurrences` already in the pool" (SM10) — and
generates each as its source is entered, so the pool's order is the entry draw's; the runtime
used to queue them once the move had settled, leaf by leaf in region declaration order
(`scheduleTransitionEvents`), whichever order the front drew, so under `seed:<n>` and `explore`
the pool's order and the draw disagreed. It now queues each as its entry unit is performed
(`enterStateInto`; the last bullet below). *History 001-C* admits exactly the two orders the entry
draw gives (second row, `2 of 2`), and its own RTC table is one of them; the same site is what
*Transition 017*'s three finding-11 traces need, between `T2.2(effect)` and `T3.1.2(effect)`,
whose sources are entered silently. *History 002-B*'s first half, of the same shape — `r2`
has an exit action there and `T2.2.2` an effect, which only add labels — admits
`r1(exit)::r1'(entry)` first (its RTC table: step 4 dispatches `CE(r1)` before `CE(r2)`),
and then `r2(exit)::r2'(entry)` first followed by **`r2'.1(exit)::T2.2.2(effect)::r2'.2(entry)`
before `r1(exit)::r1'(entry)`**: `r2'.1`'s completion, generated when `T2.2`'s firing
entered it in the step after the entry, dispatched before `r1`'s, generated by the entry
itself — an order §8.5.9 excludes whichever region was entered first — while the order §8.5.9
does give when region 2 is entered first, `r2(exit)::r2'(entry)::r1(exit)::r1'(entry)::…`,
is the one *History 001-C* admits for the identical half and *History 002-B* does not. The two
tests contradict each other on structurally identical halves, and *002-B*'s registered
alternative contradicts the specification's pool.

**The second halves are about a completion fired inside the restore.** In both tests the history
pseudostate is region 2's, so the restore re-enters region 2 and region 1 takes its default
entry: `r1`, silent, whose completion `r1(exit)::r1'(entry)` is enabled at once. PSSM's
restore places that completion event in the pool and ends the step — §8.5.7.4, "If, after
this, the `StateActivation` is completed, a `CompletionEventOccurrence` is placed in the
`StateMachine` context's event pool", the regions "restored concurrently"; and the
specification's descriptions of the two tests (§9.3.15.4, §9.3.15.7, quoted in
[`omg-issues.md`](../../project/omg-issues.md#pssm-history-001-c-and-002-b-admit-a-completion-inside-the-restore-and-contradict-each-other))
say so of these very steps: in *001-C* the restore completes the step `T4` started and `r1`'s
completion event, when dispatched, fires `T1.2`, its RTC table at step 11 already in
`top[r1, r2'[r2'.2]]` with `CE(r1)` pending; in *002-B* the step `AnotherSignal` initiated
ends in `top[r1, r2'[r2'.1]]` and the next step fires `T1.2`. Under that reading
*001-C*'s second half has one trace, `r2'(entry)::r2'.2(entry)::r1(exit)::r1'(entry)`, the
one printed and the one the runtime reaches; *002-B*'s has two, the printed
`r2'(entry)::r1(exit)::r1'(entry)::r2'.1(exit)::T2.2.2(effect)::r2'.2(entry)` and its
first alternative with `r2'.1`'s completion dispatched before `r1`'s — the two orders of a
pool holding both after the restore, which the entry draw decides. Every other registered
alternative fires `r1`'s completion *inside* the restore, before or between region 2's
entries: five of *001-C*'s six, one of *002-B*'s three. Those are what the rule as first
written was designed to reach, and they are not one rule either: *001-C*'s five split the
firing around a restored entry (`r1(exit)::r2'(entry)::r1'(entry)`, three of them), *002-B*'s
one keeps it whole, and region 1 and the first restored unit are identical in the two tests —
they differ only in what follows `r2'(entry)` in region 2 (the deep restore enters `r2'.2`,
the shallow one runs `r2'`'s initial and fires `r2'.1`'s completion), which no rule drawn
from what a region has done so far can make the reason `r1`'s completion splits around
`r2'(entry)` in one test and not in the other. Both tests' in-move alternatives come from an
implementation that dispatches a default-entered state's completion while the sibling region's
restore is still under way, which the tests' own notes exclude, and they disagree with each
other on how far.

### Where this leaves the finding

There is no completion-scheduling rule the runtime can adopt that reaches the five admitted sets:
the Entering and Junction sets are reached only by a rule about the translation of an initial
transition's effect; the History sets are reached by no rule at all, since each test's halves
admit what the other's forbid. The rule as first written reaches every admitted trace of the
five and adds traces PSSM refuses: two to *Junction 005*, eight to *History 001-C*'s first half,
nineteen and twelve to *History 002-B*'s. Read against the specification's own text rather than
the registered alternatives, the History sets are smaller than registered and partly outside
what is registered: *001-C* has two traces (its two first halves, the printed second), both
registered; *002-B* has four (two first halves by the entry draw, two second halves by the pool's
order after the restore), two of them registered and two not, and four of its six registered
traces excluded. The rule that reaches exactly those — completions dispatched after the move,
the pool in the order the entries were drawn (the table's second row) — moves neither test to
`pass`. So finding 11 is not one gap of exploration with one fix; it decomposes as follows, each
part with its own home:

- **An initial transition's effect is a completion effect in v2.** A translation limit, and
  behind it a language difference: UML runs the effect as part of the region's entry, SysML v2
  has no place for it there. *Entering 010*, *Entering 011* and *Junction 005* stay `fail` under
  today's rule, and their reasons cite this difference rather than a missing site. Whether the
  difference is decided as a *differs because v2 differs* row — which would move the three to
  `differs-by-design` through `tools/referee/pssm/rows.go:TestRows` — is an adjudication of the
  alignment note, not of this design; the alignment note's item 11 records the reading and its
  open decision 8 holds the row, leaning against it.
- **The pool's order follows the entry draw** (implemented). A runtime item of the region-order
  design proper, and the one part of the finding that was a gap of the runtime: when the entries
  of two regions each generate a completion event, PSSM's pool holds them in the order the
  entries happened (§8.5.9), which the entry draw decides, while `scheduleTransitionEvents`
  queued them after the move in region declaration order whatever the draw was — SM10's *agrees*
  held under `declared`, where the two orders coincide, and not under `seed:<n>` or `explore`.
  The runtime now queues a state's completion as its entry unit is performed: `enterStateInto`
  is told when it enters the last state of an entry path (a region's initial or restored state,
  a fork branch's or a transition's target, the state a shared path ends at), and where that
  state completes at once (`completesAtEntry`: a completion transition out of it and nothing
  below to enter) it calls `scheduleCompletionTransitions` before returning, so the event IDs
  follow the draw; `scheduleTransitionEvents` keeps scheduling the time triggers of the settled
  configuration and its ancestors, a time trigger still counting from the state's entry, and
  never a completion. A state whose completion waits — a running do behavior
  (`settleDoActions`), a composite body reaching `done` (`completeIfDone`) — is untouched, and
  a region counts as complete at its own completion vertex, not at a nested composite's
  (`regionComplete`; `state_region_completes_at_own_done`). The front's consequence is the
  *silent units* section's rule: an entry that generates a completion event is drawn under its
  own label rather than riding with the neighboring performing unit, which is why *Transition
  017*'s and the History tests' entries, every one of them silent before, record an entry draw
  now, and why the speculative entry ahead of a choice's guards (`state_route.go`) stays silent —
  it never ends a path. `declared` queues what it queued and no default golden moved beyond
  gaining a `choice` line where a completing entry had ridden silently
  (`state_firing_units_interleaved`, `l2(entry)` against `r1(exit)`), while a `reverse` or
  `seed:<n>` run whose draw entered two completing states out of declaration order dispatches
  their completions in the draw's order (`state_completion_pool_entry_order`,
  `_history_order`, `_deep_history_order`, `_fork_order`, each pinning both orders under
  `check`; `TestRuntimeRobustnessCompletionOrder`). On the suite, as the enumeration above
  predicted and no more: no bucket moved; *History 001-C* reaches both first-half orders and
  still misses ten, *History 002-B* reaches one more admitted trace and the two §8.5.9 gives
  that the suite does not register, *Entering 011* its second, and *Transition 017*'s three
  finding-11 traces — `T3.1.2(effect)` before `T2.2(effect)` when region 3 is drawn first —
  are reached, leaving it the suite's two anomalous traces alone; the referee record's
  movements table adjudicates each.
- **The History pair contradicts itself and the specification.** Recorded in
  `docs/project/omg-issues.md` as a suite defect: two tests with identical halves register
  different admitted sets — *002-B* a dispatch order §8.5.9's pool cannot give and *001-C* the
  order it gives, *001-C* a split firing *002-B* forbids — and both register alternatives that
  dispatch a completion inside the restore step their own notes and RTC tables end first. The
  two stay `fail`, their reasons citing the defect; no runtime rule is chosen to reach either.
  The fix above brings the runtime to the sets the specification's text gives for both, which
  are not the registered ones, so neither test can reach `pass` against the downloaded XMI.
- **A do step against a sibling's entry unit.** *Terminate 002*'s trace
  `top(entry)::r1(entry)::r1(doActivityPartI)::r2(entry)` was filed under this finding by
  shape only. It is a do step, not a completion: the do behavior is started at `r1`'s entry
  (`startDoAction`), the library orders its steps after `entry` and against nothing in the
  sibling region, so a due do step of an entered state is a unit of its region's queue on the
  entry front. That is the do-step site's rule extended to the entry front — the do step is
  drawn against the sibling's remaining entry units as it is against the dispatch — and belongs
  with the pending completion above: both are units of a behavior already running inside the
  entry front, and the front (`state_unit_front.go`) is where they are drawn. The do-step site
  itself draws only once the entry move has settled, so the trace stays out of reach at token
  grain and *Terminate 002* keeps this as its one reason.

The exploration model is untouched: no new choice kind, no new unit, and every draw the front
recorded before is as it was; what the pool's fix added is a `ChoiceEntryOrder` draw where a
completing entry had ridden silently, under the entry unit's existing label.
