# Recording the order of orthogonal regions

How the entry and exit of a composite state's regions and the units of the firings one
occurrence selects across regions are choice points the schedule policy draws and `explore`
enumerates — the runtime gap the [alignment note](precise-semantics-alignment.md) lists under
its findings as *the order in which orthogonal regions are entered, exited and stepped is not a
recorded choice point*, and the [PSSM referee record](../../project/pssm-referee.md) measures.
Three of the finding's four sites are implemented as this note describes: region entry (a
composite state's regions, a fork's branches, the regions a history restores), region exit, and
the units of the firings across regions. The fourth — a due do step against the dispatch at the
head of the pool — is designed here and not yet implemented; so is the fix to the gap found
while implementing the entry site, a completion's firing against the entry front (finding 11).
The note extends [scheduling policies, choice points and exploration](scheduling.md), whose
vocabulary it uses throughout.

Every claim below about what PSSM admits was read off the referee's report for the test
(`go run ./cmd/pssm-referee -filter "<test>" -json`), not inferred from the specification; the
admitted sets are quoted where they decide a design point.

## The sites

| Site | Draw | Where |
|------|------|-------|
| Entering a composite state's regions | each region's units on a front under `ChoiceEntryOrder`; `declared` and `reverse` take declaration order | `state_region_entry.go:enterRegionsInto` → `enterRegions` → `performUnits` |
| Entering a fork's branches | one queue per branch — its effect, then the owner chain entered once by the first branch drawn, then its region — on a front under `ChoiceEntryOrder`; the owner's other regions join the front once the owner is entered | `state_region_entry.go:enterForkBranches` → `drawUnits` |
| Restoring a history | the restored regions' entries as the composite's regions above | `enterRegionsInto`, from the history's record |
| Exiting a composite state's regions | each region's exits, innermost first, on a front under `ChoiceExitOrder`; then the composite's own exit | `state_region_transition.go`, `state_executor.go:exitState` → `performUnits` |
| Firing the transitions one occurrence selects across regions | one queue per firing — source exit, effects, target entry — on a front under `ChoiceRegionOrder`, drawn one unit at a time | `state_executor.go:dispatchInOrder` → `openFront` |
| A due do step against the dispatch at the head of the pool | **not yet implemented**: the do round first (`runDoRound`, every due do action one step, ordered by `chooseDoAction`), then the change-trigger poll, then the dispatch | `state_executor.go:runStep` |

The model checker mirrors the last row: `check_moves.go:enabledMoves` offers the due do steps of
a round before it offers the dispatch, so a checked machine never dispatches while a do step is
due either, and `check` reports a search that met that state as within bounds rather than
exhaustive (`CheckReport.NotEnumerated`, *do round before dispatch*;
[bounded model checking](bounded-model-checking.md)). The other sites lie inside one move of the
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
| Entering 010 | 3 | 1 | `r1(entry)` before, between or after `T2.1(effect)::r2(entry)` | finding 11 |
| Entering 011 | 6 | 1 | every interleaving of `T1.1(effect)::r1(entry)` with `T2.1(effect)::r2(entry)` | finding 11 |
| Junction 005 | 3 | 1 | `T1.3(effect)` before, between or after `T2.1(effect)::r2(entry)` | finding 11 |
| History 001-C | 12 | 1 | the two regions' restored entries and exits interleaved, twice over | finding 11 |
| History 002-B | 6 | 1 | `r1(exit)::r1'(entry)` interleaved with `r2(entry)::r2.1(exit)::T2.2.2(effect)::r2.2(entry)` | finding 11 |
| Behavior 003 A | 2 | 1 | `top(entry)` alone: the dispatch before the do activity's first step | the do-step site |
| Terminate 002 | 5 | 1 | `r2(entry)` before `r1(entry)`; the do activity's first segment against the terminating completion | one reached; the do-step site and finding 11 |
| Transition 017 | 8 | 1 | `deep(doActivity)` at any point among the completion effects, and two more (see *Transition 017*) | the do-step site, finding 11, and the suite's defect |

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
of one and two, six of two and two, twelve of the *History 001-C* pair, with the two exceptions
of *Transition 017* taken up below).

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
- one **do step** (`stepDoAction`: one action of a do behavior; the site not yet drawn);
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
segment likewise, and the body of a do step is one action.

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

Two kinds are added to `ChoiceKind`, one existing kind draws at a finer grain, and one more is
designed:

| Kind | Site | `Where` | Alternatives, canonically | Taken |
|------|------|---------|---------------------------|-------|
| `ChoiceEntryOrder` | region entry, fork branch entry, history restore | `entering <state>` — the composite whose regions are entered; `fork <name>` for a fork's branches | the queues with a unit ready, in declaration order, each labelled by its next performing unit: `left(entry)`, `T2.1(effect)`, `split->a(effect)` | the queue advanced |
| `ChoiceExitOrder` | region exit | `exiting <state>` | the queues with a unit ready, in declaration order, each labelled by its next performing unit: `inner(exit)` | the queue advanced |
| `ChoiceRegionOrder` (existing) | firing units across regions, and the entries and exits nested in a firing | `on accept <event>`, `on change` — as `dispatchInOrder` labels the occurrence | the firings with a unit ready, by source state in declaration order, each labelled by its next unit: `l1(exit)`, `l1->l2(effect)`, `l2(entry)` | the firing advanced |
| `ChoiceStepOrder` (designed, not implemented) | a due do step against the dispatch at the head of the pool | `at t=<instant>` | `do <state>` per due do action in entry order, then `dispatch <event>` for the head of the pool (a change trigger risen is `dispatch change <condition>`) | the unit run |

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
regions.

The step-order site is to reach the checker as a change to `enabledMoves`: the due do steps and
the dispatch enabled *together* rather than the do round first, one move each, which is the
state-space reading of the same choice; the static partial-order reduction (`check_reduce.go`)
already footprints a do step and a dispatch separately, so two that commute are explored once.
That change is what retires the *do round before dispatch* bound of `CheckReport.NotEnumerated`.

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
(`internal/core/runtime/testdata/conformance/README.md`). The runtime showcase's spacecraft
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

Conformance fixtures under `internal/core/runtime/testdata/conformance/`, each with an
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

To be added with the step-order site: `state_do_step_or_dispatch` (a do action due and a signal
at the head of the pool; two outcomes, one where the dispatch exits the state before the do
step ran — *Behavior 003 A*'s shape) and `state_do_step_among_completions` (a do step against
completion effects across regions), and `state_do_action_loop_timed_exit` gains the exact
`outcomes` set the fixed policies and the checker reach between them.

`explore_test.go` covers each kind — every outcome reached exactly once, the count of draws,
determinism across runs — and `explore_queue_test.go`'s sweep on eight jobs covers the fixtures
by their `outcomes`. `internal/repl/explore_test.go` explores a region-entry model through the
REPL and checks the table (`TestRunStateMachineExploresEveryRegionEntryOrder`). `replay_test.go`
refuses a witness line at each site with the run rolled back to the move's start;
`robustness_region_order_test.go` holds the feature's failure modes.

## Transition 017

Of its eight admitted traces, six are the do step `deep(doActivity)` placed among
`T2.2(effect)`, `T3.1.2(effect)` and `T3.2(effect)` with `T3.1.2` before `T3.2` — the inner
leaf's completion before its parent's — which the unit model reaches once the step-order site
and finding 11's fix are in. Two have `T3.2(effect)` before `T3.1.2(effect)` and the do step
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

## The do-step site, designed

At each **token move** of a due do flow while an occurrence at the head of the pool is due at
the same instant, the choice is "dispatch it now" against "keep moving the do flow" — finer
than a round: under `check`, `replay` and `explore` a do behavior's flow is stepped one token
move at a time and the machine may dispatch after each; under the fixed policies a do step is
one sweep (each token once) and the dispatch follows the sweep. A do behavior is concurrent
with the machine (`StatePerformances.kerml`, above), so a dispatch may cut a sweep anywhere,
and both are runs the library admits. `declared`, `reverse` and `seed:<n>` keep finishing the
sweep before they dispatch, so no default trace moves; `check` and `explore` enumerate both, so
the exhaustive set becomes a superset of every fixed policy's outcome, and the bounded verdict
`check` gives today at that state (*do round before dispatch*) is retired with its report
field once no other left-out interleaving remains. `runDoRound` keeps the order among several
due do steps (`chooseDoAction`). *Behavior 003 A*, *Terminate 002*'s terminating-completion
traces and *Transition 017*'s do step are what it reaches.

## Finding 11: a pending completion inside the entry front

Found while implementing the entry site. Every admitted trace *Entering 010*, *Entering 011*,
*Junction 005*, *History 001-C* and *History 002-B* still miss interleaves the firing of a
**completion transition** — the initial transition of a region, translated as a completion out
of a start state (`T2.1(effect)` in *Entering 010*), or the restored state's exit and its
successor's entry in the History tests — with the entry units of the sibling region *in the same step*. The runtime dispatches a
completion as a run-to-completion step of its own once the entry move has settled (the
alignment note's SM9 and SM10), so no draw among the entry units reaches an order in which the
completion fires before the sibling's entry; PSSM, which fires the completion as soon as its
source is complete while the region's entry is still under way, admits both. One trace of
*Terminate 002* has the same shape with a do behavior's first segment in place of the
completion's firing, and three of *Transition 017* have it between two regions' completions.

The fix, so a follow-up can implement it without re-deriving it: a state whose entry leaves it
complete — no do behavior, no regions still active, a completion transition enabled — **offers
its completion's firing as a unit of its region's queue** on the front that entered it, drawn
against the sibling regions' remaining units under `ChoiceRegionOrder` (the front of a firing
already accepts nested sites), the firing's own units — exit, effects, target entry — following
as units of that queue. `declared` and `reverse` take it **last**: the completion's queue is
ready only once every other queue is done, so the default order — the step the runtime
dispatches today, after the entry has settled — is unchanged and no trace golden moves under
the default; `seed:<n>` and `explore` draw it against the siblings. A completion so drawn is
the one step the runtime would have dispatched next, so the run-to-completion accounting
records it as dispatched within the move rather than as a step of its own, and a refused
replay line at it rolls back with the entry move as any other unit. What it must not do is fire
a completion whose source is completed by a sibling's unit still to come (a join's, a region's
final state reached by the sibling): the offer is made only when the completion is enabled by
the state's own entry.
