# Recording the order of orthogonal regions

A design, not yet implemented: how the entry and exit of a composite state's regions, the steps
of the firings one occurrence selects across regions, and a do action's step against a dispatch
become choice points the schedule policy draws and `explore` enumerates — the runtime gap the
[alignment note](precise-semantics-alignment.md) lists under its findings as *the order in which
orthogonal regions are entered, exited and stepped is not a recorded choice point*, and the
[PSSM referee record](../../project/pssm-referee.md) measures on ten tests. It extends
[scheduling policies, choice points and exploration](scheduling.md), whose vocabulary it uses
throughout. What the runtime does today at each site is stated first, from the code, so the
change is a delta against the tree rather than a wish.

Every unit-level claim below about what PSSM admits was read off the referee's report for the
test (`go run ./cmd/pssm-referee -filter "<test>" -json`), not inferred from the specification;
the admitted sets are quoted where they decide a design point.

## What is decided here, and what is not

Decided by this note, with the argument for each:

- the **unit** a scheduling draw selects at each of the four sites, and that a firing is *not*
  atomic — its source exits, its segment effects and its target entries are separate units;
- the **choice kinds** that record the draws, their canonical alternative order, and the exact
  text of a `%trace` line, a witness line and an explore-table witness for each;
- what `declared`, `reverse`, `seed:<n>`, `replay:<file>`, `explore` and the model checker's
  `check` policy do at each site, and that the first two keep today's order at every one;
- how a refused replay line or checker move at a new site is undone, using the marks the
  `choice`- and join-refusals already use;
- the bound on choice points a fixture gains, so exploration stays within the default budget;
- the alignment row the firing-granularity decision needs, ready to paste.

Not decided here, and put to the maintainers in *Open decisions* at the end, because they are
project choices the specification does not make:

1. whether trace goldens recorded under the default policy may gain `choice` lines (the order of
   every trace stays; the lines are additive) — the one point on which this note cannot proceed
   alone, since the project reads a golden moving under `declared` as a changed default, and
   recording a choice under `declared` is exactly what the alignment note asks for;
2. two admitted traces of *Transition 017* that no compositional reading of the model produces,
   which the referee will report as still missing after the change.

## The sites, as the code stands

| Site | Today | Where |
|------|-------|-------|
| Entering a composite state's regions | each region in declaration order, whole: its start state's entry, then that state's own regions, then its do behavior registered | `state_region_entry.go:enterRegionsInto` → `enterRegion` |
| Entering a fork's branches | the first branch's effect, then the lazily deferred owner chain, then every region of the owner in declaration order, the later branches' effects in region order | `state_region_entry.go:enterForkBranches` |
| Exiting a composite state's regions | each region in declaration order, its active state innermost first, then the composite's own exit | `state_executor.go:exitState` |
| Firing the transitions one occurrence selects across regions | one whole firing at a time — source exits, segment effects, target entries — drawn among the candidates still active before each firing and recorded as a `ChoiceRegionOrder` | `state_executor.go:dispatchInOrder` → `chooseRegion` |
| A due do step against the dispatch at the head of the pool | the do round first (`runDoRound`, every due do action one step, ordered by `chooseDoAction`), then the change-trigger poll, then the dispatch | `state_executor.go:runStep` |

The model checker mirrors the last row: `check_moves.go:enabledMoves` offers the due do steps of
a round before it offers the dispatch, so a checked machine never dispatches while a do step is
due either. The other three sites lie inside one move of the checker (a dispatch or an entry),
where the checker resolves the choice points the move draws through its `picks`.

What the referee shows at each site. The suite numbers its states; here a state is named by
where it sits: `top` is the parallel state, `r1`/`r2` the substate the first/second region
holds (`r1'` the one it moves to), `r1.1` a state nested inside `r1`, `deep` the nested state
whose do activity is logged. Transition names are the suite's own, so
`go run ./cmd/pssm-referee -filter "<test>" -json` shows each trace with the suite's state
names in the same positions:

| Test | Admitted | Reached | Missing shape |
|------|----------|---------|---------------|
| Entering 010 | 3 | 1 | `r1(entry)` before, between or after `T2.1(effect)::r2(entry)` |
| Entering 011 | 6 | 1 | every interleaving of `T1.1(effect)::r1(entry)` with `T2.1(effect)::r2(entry)` |
| Fork 002 | 4 | 1 | `top(entry)` after the first of `T2.1(effect)`, `T2.2(effect)` in either order — never first |
| Junction 005 | 3 | 1 | `T1.3(effect)` before, between or after `T2.1(effect)::r2(entry)` |
| History 001-C | 12 | 1 | the two regions' restored entries and exits interleaved, twice over |
| History 002-B | 6 | 1 | `r1(exit)::r1'(entry)` interleaved with `r2(entry)::r2.1(exit)::T2.2.2(effect)::r2.2(entry)` |
| Exiting 001 | 3 | 1 | `r2(exit)` before, between or after `r1.1(exit)::r1(exit)` |
| Exiting 003 | 2 | 1 | `r1.1(exit)` and `r2.1(exit)` in either order |
| Transition 019 | 6 | 4 | both sources' exits before either segment's effect (the two reached beyond those are the join order, a separate row) |
| Behavior 003 A | 2 | 1 | `top(entry)` alone: the dispatch before the do activity's first step |
| Transition 017 | 8 | 1 | `deep(doActivity)` at any point among the completion effects, and two more (see *Open decisions*) |

In every row the reached traces are admitted; the failure is exploration reporting itself
complete after one linearization. The counts are also the tractability budget: the largest
admitted set is twelve.

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

The runtime's order at each site is one linearization of that partial order, so today's traces
conform; the referee's missing traces are the other linearizations. Recording a draw at each
point where two regions both have a next unit, and letting the policy take either, makes every
linearization of the partial order reachable and no other: that is the whole design, and the
admitted sets above are exactly the linearizations of the per-region chains they log (three of
one and two, six of two and two, twelve of the *History 001-C* pair, with the two exceptions of
*Transition 017* taken up at the end).

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
- one **do step** (`stepDoAction`: one action of a do behavior);
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
whole to a sibling's would have to be written somewhere; it is not.

The unit is the smallest thing a trace distinguishes, which is why nothing finer is drawn: two
entry behaviors of one state run in declaration order (`performEntry`), two effects of one
segment likewise, and the body of a do step is one action.

### The front

At each site the executor holds a **front**: one queue of units per region (or per fork branch,
or per firing), each queue in the library's order, and a loop that draws which queue advances
while two or more are non-empty. A queue's head may be *conditional*: a fork branch's next unit
is "enter the owner `top`" until a sibling branch enters it, at which point the unit is dropped
from every other queue — the shared ancestor is entered once, by whichever branch reaches it
first, which is what `lazyEntry` and `certainEntries` already implement one branch at a time,
and what *Fork 002*'s admitted set says (`top(entry)` follows the first branch effect, never
precedes both). A queue also grows as it runs: entering a state whose own regions are orthogonal
appends those regions' queues to the front (they become sibling queues, not sub-units), and a
start state's initial transition appends its effect and its target's entry.

With one non-empty queue there is no draw and nothing is recorded, as one executor alone due at
an instant is no choice today. With two or more, the executor draws once per unit, not once per
queue: a draw taken does not commit the region to run to the end of its queue, since the
alternatives PSSM admits (and the library) interleave at every unit.

## The choice kinds

Three kinds are added to `ChoiceKind`, and one existing kind draws at a finer grain:

| Kind | Site | `Where` | Alternatives, canonically | Taken |
|------|------|---------|---------------------------|-------|
| `ChoiceEntryOrder` (new) | region entry, fork branch entry, history restore | `entering <state>` — the composite whose regions are entered; `fork <name>` for a fork's branches | the regions (branches) whose queue is non-empty, in declaration order, each labelled by its next unit: `left(entry)`, `T2.1(effect)`, `region 'top/Region2'` for an unlogged unit | the queue advanced |
| `ChoiceExitOrder` (new) | region exit | `exiting <state>` | the regions whose queue is non-empty, in declaration order, each labelled by its next unit: `inner(exit)` | the queue advanced |
| `ChoiceRegionOrder` (existing) | firing units across regions | `on accept <event>` etc., as `dispatchInOrder` labels it today | the firings with a unit left, by source state in declaration order, each labelled by its next unit: `exit left`, `T1.2(effect)`, `enter right` | the firing advanced |
| `ChoiceStepOrder` (new) | a due do step against the dispatch at the head of the pool | `at t=<instant>` | `do <state>` per due do action in entry order, then `dispatch <event>` for the head of the pool (a change trigger risen is `dispatch change <condition>`) | the unit run |

Canonical order is today's order at every site — declaration order for regions, branches and
firings, the do steps before the dispatch — so the first alternative taken at every draw
reproduces the run the runtime makes now, unit for unit. `ChoiceRegionOrder` keeps its name and
its `Where` label so the witness and trace lines of the fixtures already recording it keep
their text where one firing runs to its end before the next begins, which under `declared` and
`reverse` is always.

Labels name the unit, not just the region, for two reasons: a witness names alternatives exactly
as the trace does (`exploreRun.describe`, `ChoicePoint.Describe`), and a unit label is what lets a reader of the trace
see the interleaving. Where a region's next unit logs nothing (a start state with no entry
behavior), the label is the region's name, so two alternatives are never spelled alike.

### `%trace`

`ChoicePoint.Describe` gains a case per kind, in the shape of the existing ones:

```
choice entering top: next left(entry), T2.1(effect) (unordered; took left(entry) first)
choice exiting top: next inner(exit), right(exit) (unordered; took inner(exit) first)
choice on accept Continue: next exit left, exit right (unordered; took exit left first)
choice at t=0.0: next do top, dispatch AnotherSignal (unordered; ran do top first)
```

`%step`, `%continue` and `%advance` count them as they count every choice
(`2 choice points; %trace on to see them`), and over gRPC and Connect each is the informational
`choice-point` diagnostic placed at the state or transition drawn.

### Witness lines and replay

`ChoiceTaken.String` writes the three order kinds as the region and due orders are written
today, `<where>: <took> first of <alternatives>`:

```
entering top: T2.1(effect) first of left(entry), T2.1(effect)
exiting top: right(exit) first of inner(exit), right(exit)
on accept Continue: exit right first of exit left, exit right
t=0.0: dispatch AnotherSignal first of do top, dispatch AnotherSignal
```

`parseOrderChoice` already keys the kind off the `where` prefix (`t=` for the due order, the
dispatch label for the dispatch order); it gains `entering ` and `exiting ` for the two new
order kinds, and tells `ChoiceStepOrder` from `ChoiceDueOrder` — both at `t=` — by the
alternatives' `do `/`dispatch ` prefixes, which an executor's name never carries (the due order
labels executors `action <name>` and `state machine <name>`). A witness with a line at a new
site is refused by a runtime without the change with the existing `ChoiceParseError`, so an old
binary never runs a new witness as another linearization.

Replay follows the lines one unit at a time. A line the run cannot follow at a new site is a
refusal, recorded and reported by `Context.Unfollowed` as today, and the move it lies in is
undone whole: every new site is drawn inside a move that `moveWhole` already marks (`moveMark`
records the trace, the notes, the do behaviors ended and the run's steps and elements, and
rolls them back) or can be — the dispatch a firing belongs to, the entry a fork or a history
restore makes. Region entry and exit units inside a firing roll back with the firing's mark,
which nests inside the dispatch's. The step-order draw is made before either unit runs, so a
refused line there undoes nothing, as a refused transition draw undoes nothing today. What a
mark must additionally capture for the new sites is the *front* itself — the queues as they
stood — since a rolled-back entry has to be re-entered from the same point; the region states
the units wrote are already in the mark's capture of the executor (`StateExecutor.capture`
clones `activeConfig`).

### Exploration and the checker

`explore` needs nothing new: each draw is an `exploreSlot` with as many alternatives as
non-empty queues, the walk of prefixes in `exploreQueue` enumerates them, and a replay meeting a
different number of alternatives at a slot is `ErrExplorationDiverged` as before. The run is
deterministic because a draw's alternatives are computed from the front alone, in declaration
order, and the front from the model and the draws before it. `ExploreWith` on any number of
jobs returns the same `Exploration` for the same reason it does today: the queue of prefixes is
in plan order and the result keys on outcomes, not on which job ran them
(`explore_queue_test.go` proves this on every conformance case with an admissible set, and the
new fixtures join that sweep by having one).

The checker (`check_moves.go`) gets the step-order site as a change to `enabledMoves`: the due
do steps and the dispatch are enabled *together* rather than the do round first, one move each,
which is the state-space reading of the same choice; the static partial-order reduction
(`check_reduce.go`) already footprints a do step and a dispatch separately, so two that commute
are explored once. The other three sites are choice points *within* a dispatch or entry move and
reach the checker through the script a move draws from (`checkScript`, `checkRun.choose`): a
move that draws past its script takes the first alternative and reports the draw, and the checker enumerates the
alternatives as it does a `choice` pseudostate's branch today. Every move stays one executor
acting one unit as `enabledMove` defines it; what grows is the pick sequence of a dispatch
entering several regions, bounded as the next section bounds it.

## Policies at the new sites

| Spelling | Entry, exit, firing-unit and step order |
|----------|------------------------------------------|
| `reverse` (default, zero value) | first alternative: declaration order, do steps before the dispatch — today's run |
| `declared` | first alternative — today's run |
| `seed:<n>` | uniform draw among the non-empty queues at every unit |
| `replay:<file>` | the witness's line, then `reverse` |
| `explore[:runs=N,depth=D]` | the exploration's plan |
| `check` (the checker's) | the scripted pick, first alternative past the script |

`reverse` and `declared` differ today only in token order within an action step and in the due
order of executors (`scheduler.choose` takes the last alternative of a `ChoiceDueOrder` under
`reverse`, the first under `declared`); every other kind is taken first in canonical order under
both. The new kinds are taken first under both, so both keep today's order, and the
invariant `scheduling.md` states — `reverse` is exactly what every run did before policies
existed — holds unit for unit: every `.expected.json`, every `.trace.order`, every state visit
and every output of every fixture is unchanged under both. What changes under both is that each
draw is *reported*, which is the first open decision.

A `.declared` or `.seed-1` golden pins the same fixture under the sweep policies
(`TestExecutionConformanceUnderPolicies`): under `seed:1` the new draws take the seed's next
values, so the sweep goldens of fixtures entering or leaving orthogonal states move in order as
well as in lines — those are the fixtures whose admissible set the oracle derives, and the
`outcomes` they list are what the sweep must stay within.

The same holds of every seeded run: a new draw consumes the generator, so from the first new site
on, a seed's run is a different run. A seed is a reproducible sample, not a pinned outcome
(`scheduling.md`: the same seed replays the same run on every platform, of one build), so nothing
documented promises otherwise; but the showcase README's `seed:7` and `seed:42` race at t=79 is
pinned by `TestSpacecraftShowcaseFrameCountAtLowPowerIsScheduleDependent`, and if either seed
comes to show the `reverse` outcome, the README and the test move to a seed that shows the other
one, with the reason stated in the change — the point of that passage is that some seed does,
not which.

## Tractability

A front of `k` queues holding `u_1 … u_k` units draws at most `Σu_i − u_last` times, where
`u_last` is the length of the queue left when every other is empty; with equal queues of length
`u`, at most `(k−1)·u` draws, and the number of linearizations is the multinomial
`(Σu_i)! / Π(u_i!)`. The fixtures and tests concerned:

| Case | Queues | Draws at most | Linearizations |
|------|--------|---------------|----------------|
| two regions, one logged unit each (*Exiting 003*, `state_parallel_standard`) | 1, 1 | 1 | 2 |
| two regions, effect and entry each (*Entering 011*) | 2, 2 | 2 | 6 |
| two firings, exit and effect each (*Transition 019*) | 2, 2 | 2 | 6 |
| *History 001-C*, two restores of two-unit regions | 2, 2 twice | 4 | 12 (the two restores compound) |
| a do step against three completion dispatches (*Transition 017*) | 1, 3 | 3 | 4 |
| the showcase's `modes` (two regions, one start state each), entered once and never left | 1, 1 | 1 | 2 |
| the showcase's do steps against its pool, per instant both are due | 1, 1 | 1 per such instant | 2 per such instant |

Every count is within `DefaultExploreBudget` (1024 runs, 64 draws per run) by two orders of
magnitude, and the referee runs the PSSM tests under that budget (`internal/pssm/run.go`,
`DefaultBudget`). A fixture that nests orthogonal states three deep with three regions each
would draw at most `(3−1)·3 + 3·(3−1)·1 = 12` times on entry and reach `9!/(3!)^3 = 1680`
linearizations — beyond the default `runs`, reported as an incomplete exploration, which is the
existing behavior for any model past the budget and the reason the budget is settable per
conformance case (`exploreBudget`). No fixture in the tree has that shape; the draws a fixture
makes are its trace golden's `choice` lines, one per draw, so growth is visible in review.

The trace's cost is one `choice` line per draw, 34–83 bytes each (`environment.md`).

## The alignment row

The firing-granularity decision is recorded as a row of the alignment note beside *Firing order
across orthogonal regions*, to be added with the implementation:

> **The units of a firing across regions.** PSSM §8.5.10 fires the transitions one occurrence
> selects in orthogonal regions "concurrently"; *Transition 019* (§9.3.3.12) admits both sources'
> exits before either segment's effect. *v2/KerML:* `TransitionPerformances.kerml` orders a
> firing's own units — `transitionLinkSource then effect`, `effect then
> transitionLink.laterOccurrence`, and `accept`/`guard then transitionLinkSource.exit` — and
> places no succession between the units of two performances in sibling regions; §7.18.1 has the
> regions performed concurrently. *Runtime:* a firing is not atomic: `dispatchInOrder` draws the
> next unit — a source's exit, a segment's effect, a target's entry — among the firings with a
> unit left, and records each draw as a `ChoiceRegionOrder` labelled by the unit; `declared` and
> `reverse` take the firings whole in declaration order, the run the runtime made before the
> draw was recorded. **agrees**: every trace PSSM admits here is a linearization of the library's
> partial order and every linearization of it is a run some policy produces; a finer grain (two
> effects of one segment, two entry behaviors of one state) is ordered by declaration in both.

The row on the do activity (*The do activity runs asynchronously, interleaved with the machine*)
and the two on entering and exiting regions drop their caveat and cite the new kinds; the row on
the join keeps its verdict — *Transition 019*'s extra traces are its: the suite pairs the join's
segment order with the effects' order (`T1.2` before `T2.2` has `T1.3` before `T2.3`), where the
runtime draws the join's order on its own, so every firing order reached brings a mirror the
suite does not admit.

## The test contract

Conformance fixtures under `internal/core/runtime/testdata/conformance/`, each with an
`outcomes` list citing the oracle, a default trace golden, and `.declared`/`.seed-1` goldens:

| Fixture | Pins |
|---------|------|
| `state_region_entry_order` | two regions with a logged entry and an initial-transition effect each; six outcomes |
| `state_region_entry_order_uneven` | one region with one unit, one with two; three outcomes (*Entering 010*'s shape) |
| `state_fork_branch_order` | a fork's two branches into one parallel state's regions; the shared owner entered once, four outcomes (*Fork 002*'s shape) |
| `state_region_entry_nested_front` | a region's start state itself parallel, so entering it appends queues to the front |
| `state_region_exit_order` | nested and sibling exits interleaved; three outcomes (*Exiting 001*'s shape) |
| `state_history_restore_order` | a deep history restoring two regions; entries and exits interleaved |
| `state_firing_units_interleaved` | two regions' transitions on one event, exit and effect each; six outcomes, joined afterwards so the join row's order stays visible |
| `state_do_step_or_dispatch` | a do action due and a signal at the head of the pool; two outcomes, one where the dispatch exits the state before the do step ran (*Behavior 003 A*'s shape) |
| `state_do_step_among_completions` | a do step against completion effects across regions |
| `state_junction_in_orthogonal_owner` (exists since the junction fix) | gains `outcomes`: the three *Junction 005* admits |

`explore_test.go` gains one test per kind — every outcome reached exactly once, the count of
draws asserted, determinism across runs — and `explore_queue_test.go`'s sweep on eight jobs
covers the fixtures by their `outcomes`. `internal/repl/explore_test.go` explores
`state_region_entry_order` and `state_do_step_or_dispatch` through the REPL and checks the table.
`replay_test.go` and `schedule_replay_test.go` refuse a witness line at each new site with the
run rolled back to the move's start; `robustness_test.go` adds the exploration of the
three-deep-three-wide model above reported incomplete within budget rather than hung, and a
replay whose entry-order line names a region the front does not hold.

The referee's expected movements, each to be verified by running the filter and adjudicated in
the record's movements table: *Entering 010*, *Entering 011*, *Fork 002*, *Junction 005*,
*History 001-C*, *History 002-B*, *Exiting 001*, *Exiting 003* and *Behavior 003 A*, `fail` →
`pass`; *Transition 019* stays `fail` on the join row alone, with its four missing traces reached and
as many extra as firing orders, the join's order drawn independently of theirs; *Transition 017* is the second open decision. The
baseline moves from 45 to 54 `pass` and from 15 to 6 `fail` if all nine move, and every count
is adjudicated in the same change.

## Open decisions

1. **Goldens recorded under the default gain `choice` lines.** A draw at a new site is recorded
   under every policy — *a fixed policy resolves the choice, it does not remove it*
   (`scheduling.md`) — so the trace golden of every fixture that enters or leaves a state with two
   or more regions, or whose regions react to one event, gains one `choice` line per draw: the 96
   fixtures with a `parallel` state or a fork and a trace golden are the upper bound. Not one
   order, output, state visit or `.expected.json` changes. The runtime showcase's spacecraft
   session, whose README checkpoint reads `500 choice points`, gains one draw entering `modes`
   (two regions, one unit each; the state is never left) and one at every instant a do step of
   `transmitData` or `recharge` is due while an occurrence waits in the pool — the count is what
   the regenerated checkpoint shows, and under `reverse` and `declared` the run itself, the
   t=79 race included, is unchanged (`TestSpacecraftShowcaseFrameCountAtLowPowerIsScheduleDependent`
   pins 49 frames and battery 41 under both). *Options:* (i) accept the
   additive movement, regenerate with `-update-traces` and gate the regeneration by a diff filter
   that admits only added `choice` lines, so the review can see that nothing else moved; (ii)
   record the new kinds only under `seed:<n>`, `replay` and `explore` — which breaks the rule that
   a witness of a seeded run replays under `replay:` from the same lines a default run traces,
   and leaves `%trace` under the default silent about a choice the run made. *Lean:* (i); the
   design is built for it. It is put here rather than taken because the project reads any golden
   moving under the default as a changed default, and that reading has to be reconciled with the
   alignment note's request before the goldens move.
2. **Two admitted traces of *Transition 017*.** Of its eight admitted traces, six are the do step
   `deep(doActivity)` placed among `T2.2(effect)`, `T3.1.2(effect)` and `T3.2(effect)` with
   `T3.1.2` before `T3.2` — the inner leaf's completion before its parent's — which the unit
   model reaches. Two have `T3.2(effect)` before `T3.1.2(effect)` and the do step after `T3.2`:
   `deep`'s completion transition fires before the transition its inner region's completion
   enables, and the do activity steps after its state was left. The suite's own comment on the
   test's state machine ("Expected execution sequence") has `T3.2` fire when the completion event
   `deep` generates is consumed, after the inner region's final state — the six, not the
   two — so the two registered traces contradict the test's stated intent. No reading of
   `StatePerformances.kerml` orders a parent's exit before its region's completion transition,
   and the runtime will not produce these, so the referee will report the test `fail` with two
   missing after the change. *Options:* (i) read the PSSM reference implementation's completion
   rule for a state entered with a do activity and a region to see whether the two are an
   artefact of it (a completion check made at entry before the region's initial transition has
   fired would produce them), and add an alignment row for it — *differs because v2 differs* if
   the library's successions exclude them, which maps the test to `differs-by-design` through a
   hand-reviewed row in `internal/pssm/rows.go`; (ii) leave the test `fail` citing the two traces
   with this analysis in the record. *Lean:* (i) before the record claims either.

## Implementation order

Each step keeps `go build`, `go vet` and `go test ./internal/core/runtime` green and can be
reviewed alone; the goldens move once, at step 6, after decision 1.

1. `ChoiceKind`: the three kinds, `Describe`, `ChoiceTaken.String`, `parseOrderChoice`, with
   unit tests on the round trip of every spelling above.
2. The front: a type holding per-region unit queues over the existing `regionEntry` and
   `lazyEntry`, `enterRegionsInto` and `enterForkBranches` rewritten to drain it under
   `ChoiceEntryOrder`, a history restore included; `moveMark` extended to the front and the
   region states. Fixtures `state_region_entry_order*`, `state_fork_branch_order`,
   `state_history_restore_order`; `Entering 010`, `Entering 011`, `Fork 002`, `Junction 005`,
   `History 001-C` and `History 002-B` verified with the referee's filter.
3. Exit: `exitState`'s region loop rewritten over a front of exit queues under
   `ChoiceExitOrder`; `state_region_exit_order`; `Exiting 001`, `Exiting 003`.
4. Firing units: `dispatchInOrder` drawing per unit over the firings' queues, the firing's own
   `moveMark` nested in the dispatch's; `state_firing_units_interleaved`; `Transition 019`
   reduced to the join row.
5. Step order: `runStep` and `enabledMoves` offering the due do steps and the dispatch together
   under `ChoiceStepOrder`, `runDoRound` kept for the order among several due do steps
   (`chooseDoAction`); `state_do_step_or_dispatch`, `state_do_step_among_completions`;
   `Behavior 003 A`, and `Transition 017` as decided.
6. Goldens, the showcase checkpoint, the guide's section on choice points and schedule policies,
   the alignment rows, the record's findings and movements tables, the spec-compliance rows, the
   baseline (`-update`), a changelog fragment.
