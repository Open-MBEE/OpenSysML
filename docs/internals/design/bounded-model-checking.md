# Bounded model checking of behaviors

A design for exploring every admissible interleaving of an action or state machine, up to a
bound, and reporting the outcomes the specification leaves open, the deadlocks a run can reach,
and the requirements an interleaving can violate. This note fixes the data shapes, the reduction
rule, the bounds and the user surface so the work can be reviewed before code is written, and so
each stage can be judged complete on its own.

What is implemented of it is the `explore` scheduling policy
([scheduling](scheduling.md#exploration-explorego)): every linearization within a budget of runs
and choice points, each replayed from a fresh context, with the distinct outcomes, their
linearization counts and one witness per outcome reported, and an honest `incomplete` when the
budget stopped it. It runs every linearization rather than one per equivalence class, and it
reports outcomes, not deadlocks or requirement violations; the snapshots, the partial-order
reduction and those analyses remain this proposal's.

## The problem this answers

The Kernel Semantic Library states a **partial order** between the performances of a behavior. A
succession is a `HappensBefore` link that orders one whole occurrence before another
(`Occurrences.kerml`); nothing orders two steps no chain of such links connects. Fork branches,
orthogonal regions, the `do` behaviors of concurrently active states, and two nodes reached by
independent chains are unordered, and the library states no conflict rule for two unordered
writes of one feature. One model therefore admits many executions, and every one of them
conforms.

The executor runs exactly one, chosen by the run's scheduling policy
([scheduling](scheduling.md)) and reported at each choice point. Under the default `reverse` the
choice is fixed and documented — tokens are stepped in descending index order within a step, a
fork appends its branch tokens in succession-declaration order, the first holding guard, the first
enabled transition and the first of the regions an event selected fire — while `seed:<n>` draws
every one of those, transition and region order included, from a generator the seed replays;
only the order regions are entered and exited in on a composite transition stays declaration
order under every policy, since it is no choice point. [The semantic
oracle](../../project/behavior-semantic-oracle.md) separates what the library fixes from what
that scheduling chose. The compliance map marks the rows where the runtime picks an
order the specification does not as approximate. The `explore` policy answers which outcomes the
admissible executions reach, by running each of them within a budget; what no surface offers is
the question a safety case asks at scale: *does any admissible execution violate this
requirement, deadlock, or end in a state the model did not intend?* Enumerating every
linearization answers it only for behaviors small enough to enumerate, and a race the scheduling
happens to resolve the intended way is invisible to a single run.

The pinned OMG pilot evaluates expressions and executes neither actions nor state machines
([pilot execution referee](../../project/pilot-execution-referee.md)), so there is no reference
to compare an exploration against; the reference is the library text, as the oracle reads it.

## Scope

In scope, in stage order (see [Stages](#stages)):

1. **Actions**: token flow over a lowered `ActionGraph`, including nested action nodes,
   fork/join/merge/decision, body statements, and `accept` of messages posted within the checked
   behavior (a `send` on one branch, an `accept` on another).
2. **State machines**: transition selection and firing over a lowered `StateGraph`, orthogonal
   regions, `do` behavior interleaving, deferred events, and the order in which events posted
   at one instant are dispatched.
3. **Time**: time triggers and change events, explored as the choice of which of several events
   due at the same instant is dispatched first. Time itself is not a choice: the event queue is
   ordered by timestamp, and only ties are branching points.

Out of scope, and stated as such in the report where they apply:

- **Data nondeterminism.** Inputs, `in` parameters and unbound features are fixed at the values
  the caller gave; the checker explores scheduling, not the value domain. Value-domain questions
  are the SMT layer's (`internal/core/solve`), which reasons about constraints and requirements
  over free variables and has no notion of a behavior's state. The two are complementary and stay
  separate here; [SMT bounded model checking](smt-model-checking.md) is the design that gives the
  solver that notion, with this engine as its referee.
- **Unbounded state spaces.** A merge-loop, a `do` behavior that never completes or a time
  trigger that re-arms itself produces infinitely many states. The checker is *bounded*: it
  reports what it found within the bound and never claims more.
- **Behavior across objects.** Stage 1 and 2 check one behavior, performed by one object. Signals
  routed to other objects' running machines are recorded as effects, not followed. Following them
  is a later extension, with its own note.
- **Liveness.** Only reachability properties: a requirement violated in some reachable state, a
  deadlock, a divergent outcome. "Every run eventually reaches `done`" is not asked.

## What is explored

A **state** of the exploration is the whole mutable condition of a run at a point where the
scheduler has a choice:

| Component | Where it lives today | What the checker must capture |
|-----------|----------------------|-------------------------------|
| Token list | `ActionExecutor.tokens` (`[]Token`: id, location, wait, frame, paused body) | The multiset of (location, frame, wait) — the id is scheduling detail and is dropped from the canonical form |
| Performance frames | `actionFrame` tree under `performances.root`: `data`, `locals`, `live`, `subactions`, `pending`, `nested`, `ended`, `began` | Every frame reachable from a token or from the root, with its held values and delivery queues |
| Merge and breakpoint bookkeeping | each token's `Via` and `moved` (the succession it arrived over and the sweep that moved it), `ActionExecutor.sweep`, `firedBreakpoints` | The arrivals (they decide whether a merge admits a token); breakpoints are disabled during exploration and not captured |
| Object values | `Context.instances` → `Instance.FeatureValues`, connector ends, classifiers | Every object the behavior reads or writes, the performing object included |
| Messages in flight | `Context.messages` (the message bus `send` posts to and `accept` consumes from, oldest first) | The bus contents in arrival order |
| State configuration | `StateExecutor.activeConfig`, `stateStack`, `history`, `stateAttrs`, `stateData` | All of it |
| Event queue | `StateExecutor.eventQueue` (a heap ordered by timestamp, completion first, then arrival), `deferred`, `timerScheduled`, `changeFired`, `changeWaits` | The queue as a sequence in dispatch order; the latches |
| `do` behaviors | `StateExecutor.doActions` (in state-entry order, one action per round) | The pending statements of each |
| Virtual time | `Context.clock` (`now` and the waiters on it), shared by every executor of the context | Captured, not explored |

Not captured: the memo tables (`calcShapes`, `writeTargets`, `invocationTargets`, literal
caches, compiled calc closures, effective features), the lowered graphs, the symbol tables.
These are functions of the model, not of the run, and are shared by every explored state. The
`Context` is therefore split into the **model-derived** part, `runtime.Model`, that is shared
and the **run-derived** part, `runtime.Context`, that is a state (`instances`, `created`,
`lives`, `occurrences`, `variantObjects`, `selectedVariants`, the runs' `calcUsageRuns`,
`activations`, the id sequence, the bus, the clock, the scheduler, the trace). A `Model` is
built once per analysis worker — it memoizes into plain maps as the resolver does, so it is
shared exactly as far as the resolver is — and `NewContext(model, maxSteps)` allocates the
run-derived part alone. This split was the single largest change the checker needed, and the
executor benefits from it on its own: a fresh run no longer rebuilds what the model fixed.

The `Context` already journaled feature-value writes and object-identity changes while a probe
or transaction was under way (`beginJournal`, `journalWrites`, `journalUndos`), and rolled them
back on failure. That was an undo log for one component of the state. The checker generalizes it
rather than adding a second mechanism: a **snapshot** (`Context.Snapshot`, and the executors'
`Snapshot` that add their own state) is a journal mark taken between steps, extended to cover the
token list, the frame tree, the message bus and the state-executor fields above, and **restore**
(`Snapshot.Restore`) is a rollback to that mark, repeatable until `Release`; `beginJournal` and
`beginProbe` take the same mark and roll back the same way. Restoring
puts values back into the maps and frames the run already holds, so every object keeps its
identity and every alias into it stays valid; a usage re-instantiated since denotes again the
object it denoted at the mark; an open calc usage evaluation keeps the outputs it had worked out
at the mark and no more; what the run made after the mark is abandoned and the identities
it took are handed out again — never one another context sharing the identity sequence (a REPL
re-analysis adopts the replaced context's, `AdoptIdentities`) took since, whether or not that
context is still around, since identities are monotone across contexts and no part of the
canonical state. The sequence remembers of each context only how far it took, so a replaced
context is collected. An undo log suits depth-first exploration — only
the path from the root to the current state is live, and backtracking one move undoes one move's
writes — and avoids copying the object graph at every choice point. A snapshot is taken between
steps, never from inside one (`ErrSnapshotMidRun`), and cannot capture a body paused
mid-statement — a token or a `do` behavior suspended in its coroutine on a wait
(`ErrSnapshotPausedBody`); stage 3's `do` interleaving is where those waits become explicit
state.

### The atomic step

The executor's `Step()` under a fixed policy is a tool artifact: it steps every token once, in a
fixed order, and the
[oracle](../../project/behavior-semantic-oracle.md#what-the-library-fixes-and-what-a-trace-adds)
already warns that a `step N:` line is a boundary the library does not define. The checker must
not explore interleavings *inside* that artifact, nor interleavings finer than the library
admits. The `explore` policy already takes the unit below for actions: one of its steps is one
token advancing one node, so a branch of several nodes can be overtaken between any two of them
(`action_explore_write_between_branch_nodes` is the case a lockstep step would have missed).

The unit the library defines is a **performance**: a node's body runs "completely before" its
successors start (`HappensBefore`), and two unordered performances may overlap arbitrarily in
time. Two consequences:

- The atomic unit of exploration is **one token advancing one node**: run the node's body to
  completion, follow its enabled successions, and stop. This is what `stepToken(i)` does today
  for a leaf node; for a node with a subflow it is one step of that subflow, which the frame tree
  already makes a distinct token.
- Because two unordered performances *may* overlap, a body with several statements is not
  atomic against a concurrent body under the library's reading: `left { x := 1; y := x }` and
  `right { x := 2 }` admit `y = 2`. The checker takes the coarser reading — one body, one atomic
  step — and says so in the report. Statement-level interleaving multiplies the state space by
  the product of body lengths for no property a systems model states; the coarse reading is
  also the one the executor implements, so the checker's outcomes are a superset of the
  executor's rather than of a finer semantics it does not have. A later stage may add a
  `-granularity statement` mode if a property needs it.

For a state machine the atomic unit is **one dispatch**: take one event off the queue, select
the transitions it enables in the active configuration, fire them, run entries and effects, and
stop. A `do` round is one atomic unit per do behavior.

### The choice points

At each state the checker enumerates the **enabled moves**:

- **Action**: every token whose node can advance now. A token at a plain node always can. A
  token at a join that has not collected cannot. A token whose body is paused can when the
  body would go on if resumed (`Token.resumable`): at a breakpoint always, on the clock once
  the performed action's wait has ended — so a performed action and a sibling accept due at
  the same instant are two moves, either first. A token parked at an `accept` (`Wait != nil`)
  can when its wait is answered in the current state: for a message
  accept, when `Context.messages` holds a message `acceptMatch` would take; for a time or change
  trigger, when `triggerHolds`. The executor already retries every parked token on every step
  and clears the wait only when the match succeeds (`stepNestedAction`), so the checker asks the
  same two questions without taking the message — a readiness probe that reads the bus and the
  guard and mutates nothing, evaluated under `beginProbe` where the guard could have effects.
  A probe that fails — `acceptMatch` leaves a routing error in its error slot because the `via`
  port does not resolve, or `triggerHolds` returns one — also makes the token a move, as
  `HasPendingSignal` counts that case as pending today: taking the move raises the typed
  runtime error, which the report lists as a violation with its witness, not as a deadlock. A
  parked token whose wait is neither answered nor failed is not a move; a state in which no
  token has a move and some token is parked is a deadlock, as `ErrAcceptDeadlock` reports it
  today. A token at a decision has one move per succession whose guard holds — the library
  fixes exactly one (`DecisionPerformance::outgoingHBLink [1]`), so with guards evaluated
  against the current state there is normally one; overlapping guards are a model defect the
  executor reports as a `ChoiceDecisionBranch` today, taking the first in declaration order
  under `reverse` and `declared` and a seeded draw under `seed:<n>`.
- **State machine**: with one event at the head of the queue there is one move. Several events
  due at the same instant are one move each; the executor dispatches them in arrival order,
  the checker explores every order, so a state that reacts differently to `A` then `B` than to
  `B` then `A` is found. Completion events keep their precedence over pool events at the same
  instant (`eventHeap.Less`), because that is a library-derived rule
  (a state that has completed leaves before it reacts), not a tool choice.
- **`do` behaviors**: one move per active do behavior with pending statements.
- **Regions**: one event enabling transitions in several orthogonal regions is one dispatch
  under run-to-completion. Selection happens once, against the state before any reaction fires:
  `selectTransitions` evaluates every candidate's guards over the active configuration and then
  the selected transitions fire, so a reaction's effect cannot enable or disable another
  region's transition for the same event — that write is seen by the *next* dispatch's
  selection, which the captured state carries. The checker keeps this reading, and what it
  explores inside a dispatch is the order in which the selected reactions run, which the
  library leaves open and the executor draws through its scheduling policy today
  (`ChoiceRegionOrder`: declaration order under `reverse` and `declared`, a seeded draw under
  `seed:<n>`, every order under `explore`). Order matters in two ways. Two reactions that write
  shared data, or one that writes what another's effect reads, reach different states in
  different orders. And a reaction that leaves a composite state deactivates every leaf under
  it, so a sibling region's candidate whose leaf was left is dropped when its turn comes
  (`dispatchInOrder` checks `isActive` before each firing, not at selection): a region-local transition out of a parallel composite fires alone in one order
  and after its sibling's reaction in the other. The active configuration is therefore part of
  a reaction's footprint — it reads the activity of its own leaf and writes the activity of
  every state it exits or enters — and the dispatch is split by the independence relation
  below: the selected reactions are partitioned into groups whose footprints intersect, each
  group of size `k` contributes `k!` orders, and the dispatch has one move per combination;
  groups of size one contribute nothing, so a machine whose regions keep to their own data and
  their own states has exactly one move, as today. Each order applies the executor's fire-time
  checks in that order, so a candidate an earlier reaction left is dropped exactly as the
  executor drops it, and runs to completion before the next event is taken, so the
  run-to-completion boundary is kept. A witness records the region order it took and the
  candidates that order dropped, and the replay scheduler honours it. Without this split the
  checker could not claim to find a divergence two regions produce, and the verdict would have
  to exclude it.
- **Choice pseudostates**: a compound transition through a `choice` has one move per branch
  whose guard holds *after* the effects of the segments into the choice have run — the guards
  are read against the state those effects leave (`state_route.go:resolveChoice`), so a branch
  an incoming effect enables is a move and one it disables is not. Several enabled is the
  executor's `ChoiceTransition` at `choice <name>` today, the first in declaration order under
  `reverse` and `declared`, a seeded draw under `seed:<n>`, every branch under `explore`. A
  junction contributes no move: its branch is settled before the transition fires, from the
  state the dispatch starts in, and is part of the transition's enabledness (a junction with no
  enabled branch means the transition is not enabled).
- **Not moves**: deferral is determined by the configuration — a state that defers the
  occurrence holds it back from every transition not nested in it, and the occurrence is either
  consumed by a nested transition or deferred (`deferralOutranks`) — and a composite state's
  completion is a completion event queued at the current instant as a leaf's is, ordered by the
  same `eventHeap` rule. Both are read from the state, not drawn.

### The properties

A run is a **violation** when, in any reached state:

1. A **requirement** or **constraint** the user named evaluates `false` — the same
   `-requirement`/`-constraint`/`-satisfy` verdict the CLI reports today for one run, evaluated
   over the checked object at each state. Properties are evaluated only at states where no
   body is mid-run (there are none, by the atomicity rule), so a transient value inside one
   performance is never a false violation.
2. A **deadlock**: no enabled move and the behavior is not complete (`ErrActionDeadlock`,
   `ErrAcceptDeadlock` as the executor reports them, but now for every schedule, not one).
3. A **typed runtime error** a body raises (`ErrBindingConflict`, an unbound feature, a
   budget exhaustion is *not* one — see bounds).

A run is **divergent** when two complete executions end with different values of a feature the
user named (or, by default, of any feature of the checked behavior or object). Divergence is not
a violation — the library admits it — but it is what the semantic oracle calls the tool-defined
outcome, and a model that depends on it has a defect the modeler should see. The report lists
each divergent feature with the set of final values and one witness schedule per value.

## Partial-order reduction

Exploring every interleaving of `n` concurrent tokens over `k` steps each is `(nk)!/(k!)^n`
schedules. Almost all of them are equivalent: two moves that touch disjoint data reach the same
state in either order. Partial-order reduction explores one representative of each equivalence
class.

### The independence relation

Two enabled moves `a` and `b` are **independent** in a state when executing `a` then `b` reaches
the same state as `b` then `a`, and neither disables the other. The checker approximates this
statically and conservatively from the lowered IR: it computes, per node, a **footprint**

```
footprint(node) = {
  reads:    features the body's expressions read, by resolved declaration,
            and features the guards of the node's outgoing successions read
  writes:   Assign.Target / AssignTarget.Steps[last], bind ends, out pins delivered
  sends:    Send targets (port or receiver), by resolved feature
  accepts:  Accept.SignalType / ViaPort, and the features an accept's trigger
            condition reads
  control:  the join/merge nodes the token's successions reach; for a transition,
            the states it exits and enters (written) and the leaf it fires from (read)
}
```

The footprint covers the whole atomic move, not the body alone. Advancing a token evaluates
the guards of its outgoing successions (`enabledSuccessions`) and, for a parked token, the
trigger condition that answers its wait (`triggerHolds`), so a feature either of those reads
is a read of the move: a branch that writes `ready` and a branch whose succession is guarded
`[ready]` are dependent, since their order decides which successor is taken. For a state
machine the same rule covers a transition's guard and effect, a state's entry and exit
actions, and the change condition a change trigger polls; its `control` entry makes a
transition that leaves a composite dependent on every sibling candidate under it, since
firing it decides whether they still fire.

and declares `a` and `b` **dependent** when any of these hold:

- `writes(a) ∩ (reads(b) ∪ writes(b)) ≠ ∅`, or symmetrically — a data race.
- `sends(a)` may deliver what `accepts(b)` waits for — a message the order of moves can make
  available or not.
- `control(a) ∩ control(b) ≠ ∅` — both tokens converge on one join or merge, whose behavior
  depends on arrival count and order (`stepJoinNode`, `stepMergeNode`).
- Either footprint contains a **dynamic** target the static analysis cannot resolve: a chained
  assignment whose `Base` is an expression (`assign pick.target.mark := …` where `pick` is an
  object-valued pin), a `send` through a `via` path routed by connections, a feature read
  through a variant selection. Such a move is dependent on every other move. This is the
  soundness clause: the reduction never assumes independence it cannot prove.

Two moves in different `actionFrame`s that read and write only their own frame's `data` are
independent by construction; this is the common case for fork branches that compute into their
own pins and meet at a join, and it is what makes the reduction effective on real models.

Footprints are computed once per node when the graph is lowered, in `internal/core/lower`, and
stored beside `Bodies` as `Footprints map[ast.Node]Footprint`. The lowering layer already
resolves every name a statement uses (`Assign.Scope`, `Send.TargetSym`, `AssignTarget.Steps`);
the footprint is a projection of what it has, not a new analysis. That keeps the executor's
invariant that the runtime consumes lowered IR and re-derives nothing from `symbol.Decl`.

### The algorithm

The first checker uses **persistent sets** with a sleep set, the classic static reduction: at
each state, pick one enabled move `a` and grow a set from it that is closed under "any move
dependent on a member is a member". Explore only that set. The sleep
set prunes moves already explored from an equivalent predecessor. With a conservative
independence relation this is sound: every deadlock and every reachable state up to equivalence
is found.

A later stage upgrades to **dynamic partial-order reduction** (Flanagan–Godefroid): explore one
schedule to the bound, then walk it backwards; wherever two moves were actually dependent at
run time (their concrete footprints, not the static ones, intersected), add the later move as a
backtrack point at the earlier state. Dynamic footprints are exact where the static ones were
"dynamic target, dependent on everything", so DPOR recovers the reduction the static clause
gave up. The concrete footprint is recorded by instrumenting the frame's `data` reads and
writes, `Instance.FeatureValues` reads and writes, and the message queues, during the
exploration run only — the ordinary executor path is untouched.

### Visited states

Reduction alone re-explores a state reached by two inequivalent paths. The checker keeps a
**visited set** keyed by a canonical hash of the state: token multiset sorted by (node identity,
frame path), frames in root-first order with values rendered through the same canonical
formatter the trace recorder uses (`RecordActionStep` sorts by id for the same reason), objects
by materialization path rather than by `Instance.ID`, the event queue in dispatch order. Ids
handed out by `idSequence` and `nextTokenID` are excluded: two states that differ only in the
numbers the run happened to assign are one state.

## Bounds

The checker stops a branch when any of these is reached, records which, and never reports
"verified" without stating them:

| Bound | Default | Flag | What it limits |
|-------|---------|------|----------------|
| Depth | 10 000 moves | `-check-depth N` | Moves along one schedule; a merge-loop or re-arming timer is cut here |
| States | 1 000 000 | `-check-states N` | Distinct states visited across the whole exploration |
| Time | 60 s wall clock | `-check-timeout D` | The exploration as a whole |
| Executor budgets | as today | `OPENSYSML_MAX_ACTION_STEPS` etc. | Per schedule, unchanged; hitting one is a bound, not an error |

A branch cut by a bound is reported as **incomplete**, distinctly from a deadlock or a violation.
The overall verdict is one of:

- `no violation within bounds` — every schedule explored ended complete or was cut by a bound;
  the bounds hit are listed.
- `no violation, exhaustive` — every schedule ended complete and no bound was hit. This is the
  only verdict that is a proof, and it is a proof relative to the atomicity rule and the
  properties given.
- `violation` — with the property, the state, and a witness schedule.
- `divergent` — no violation, but a named feature ends differently on different schedules.

## Witness schedules and replay

A schedule is a sequence of moves: `(token-or-event identity, node-or-transition)`. The checker
writes a witness in the trace golden format the executor already produces (`.trace.golden`
lines: `step N: token@node`, body statements, `assign x <- v`), so a witness reads like any trace
and is reviewed against the [oracle](../../project/behavior-semantic-oracle.md) the same way.

Replay: the executor gains a **scheduler seam** — the one place `Step()` decides which token to
advance (the `for i := len(tokenIndices) - 1; …` loop) and the state executor's event dispatch
decides which of several same-instant events to take first. Today the policy is fixed; behind an interface

```go
type Scheduler interface {
    // Next picks the enabled move to make from those offered, in a fixed
    // presentation order; the default returns the executor's current pick.
    Next(enabled []Move) Move
}
```

the default scheduler reproduces today's traces bit for bit (the golden trace suite proves it),
a `replay` scheduler follows a witness, and the checker's DFS is itself a scheduler that
snapshots before each `Next`. A seeded-random scheduler falls out for free and is worth wiring
into the conformance harness on its own: running the corpus under several seeds distinguishes
goldens that pin a library-fixed order from goldens that pin a tool choice, which is exactly the
distinction the compliance map's approximate rows make by hand today.

The seam does not change what a step is, when errors surface, or what a trace records; the
constructor-succeeds/`initialize()`-errors contract and the error-timing tests are unaffected.

## User surface

Command line, alongside the check flags in [the CLI reference](../../reference/cli.md):

```
sysml model.sysml -instantiate Fleet::truck \
    -check-action "Fleet::Truck::dispatch truck" \
    -requirement Fleet::NeverOverloaded \
    -check-diverge load
```

| Flag | Meaning |
|------|---------|
| `-check-action "<name> [object]"` | Explore the schedules of an action, as `-action` runs one |
| `-check-state "<name> [object]"` | Explore the schedules of a state machine, as `-state` runs one, for the `-advance` horizon |
| `-check-diverge <feature>` | Report divergence of this feature; repeatable; absent, every feature of the behavior and the object |
| `-check-depth`, `-check-states`, `-check-timeout` | The bounds |
| `-check-witness <dir>` | Write each violation's and each divergent value's witness schedule as a trace file |

The properties are the existing `-requirement`, `-constraint` and `-satisfy` flags: with a
`-check-*` flag present they are evaluated at every explored state rather than once. `-json`
applies, with `verdict`, `bounds_hit`, `states`, `violations[]`, `divergent[]` and the witness
paths.

REPL: `%check-action`, `%check-state` with the same arguments, and `%replay <witness>` which
installs the replay scheduler and then behaves as `%step`/`%continue` do, so a witness can be
walked with breakpoints in the debugger. The LSP surfaces nothing in the first stages; a code action
"check this action" is a natural later addition.

Exit status follows the CLI's existing convention: a violation is a failed check.

## Test contract

Written before the code, as the behavioral four-layer contract asks:

1. **Determinism of the default seam.** Every existing `.trace.golden` and `.expected.json`
   passes unchanged with the default scheduler. This is the regression gate for the refactoring.
2. **Snapshot/restore round trip.** For every conformance case, snapshot at every step of the
   default run, restore, run to completion, and compare against the unsnapshotted run: same
   trace, same values, same errors. A separate test restores to a snapshot *twice* and runs both
   to completion to prove snapshots are not consumed.
3. **Oracle cases.** Each case in the semantic oracle names the orderings it leaves open and the
   outcome it fixes. The checker over `action_join_waits_for_slowest_branch` must report no
   divergence and `arrived = 3` on every schedule; over `action_fork_branches_write_one_feature`
   it must report `x` divergent with exactly `{1, 2}` and `leftRan`, `rightRan` not divergent.
   These expectations are derived from the library text, not from the checker, and go in a
   `.check.expected.json` beside each case.
4. **Reduction soundness.** For a corpus of small models, compare the set of final states the
   reduced exploration reaches with the unreduced one (a `-check-no-por` flag exists for this
   test only). They must be equal. Include models exercising every dependence clause: shared
   write, a write to a feature another branch's succession guard reads, a write to a feature
   a parked trigger's condition reads, send/accept pairing, join convergence, two orthogonal
   regions writing one feature on one event, one region's effect writing a feature another
   region's guard on the same event reads (every order selects the same set and differs only
   in the effects), a region-local transition leaving a parallel composite while a sibling
   region's candidate is selected for the same event (one order fires both, the other drops the
   sibling), and a dynamic target that must fall back to "dependent on everything".
5. **Reduction effectiveness.** Pin the state count for the corpus so a change that silently
   weakens the reduction is noticed; this is a ratchet like the corpus gates, adjudicated on
   every movement.
6. **Bounds.** A merge-loop model must report `incomplete` with the depth bound named, never
   hang and never claim exhaustiveness. A timeout test uses a short `-check-timeout`.
7. **Witness replay.** Every witness the checker writes for the corpus replays to the state it
   claims, through the replay scheduler, and the replayed trace equals the witness.
8. **Robustness.** The failure modes in `robustness_test.go` (deadlocks, unbound parameters,
   dangling successions) are reported by the checker as violations with a witness, not as
   exploration errors. An `accept` whose matching message needs a `via` port that does not
   resolve is reported as that routing error, on the schedule that reaches it, not as a
   deadlock.

## Stages

Each stage leaves `main` green, ships behind its own flag, and is useful on its own.

1. **Scheduler seam and run/model split of `Context`.** No user-visible change. Default scheduler
   reproduces every golden. Seeded-random scheduler wired into the conformance harness as an
   opt-in sweep. Test layers 1 and 2. This is the refactoring stage; it is the largest and the
   one whose review matters most, because everything after it depends on the state being
   capturable in one place. *Implemented:* the seam and the `explore` and `seed:<n>` policies
   ([scheduling](scheduling.md)), with `seed:1` in every conformance run and
   `OPENSYSML_SCHEDULE_SEEDS` widening the sweep on demand; `runtime.Model` holding the
   model-derived part once per worker and `runtime.Context` the run-derived part, with
   `analysis.Worker` owning the model and `Model.NewContext` building only a run; and
   `Snapshot`/`Restore`/`Release` over the journal capturing the context's objects, lifetimes,
   variants, occurrences, bus, clock, id sequence, activation and run counters, trace, run
   ledgers and scheduler state, the action executor's tokens, frame tree, merge and breakpoint
   bookkeeping and step counters, and the state executor's configuration, stack, history, state
   values, event queue, deferred events, timers, change triggers, `do` progress and virtual
   time — everything layer 2 needs, which the round-trip and restore-twice tests over every
   conformance case prove. Not captured: a body coroutine paused mid-statement
   (`ErrSnapshotPausedBody`), which the eight conformance cases whose default run pauses one
   pin. Stages 2 and 3 add no capture for the executors' present state; they add the choice
   points (one token, one dispatch), the canonical form over a snapshot, and, for `do`
   interleaving, the explicit representation of a body's wait that removes the paused-body
   limit.
2. **Actions, static reduction.** Snapshot/restore for the action executor's state; DFS with
   persistent sets and a visited set; footprints in the lowering layer; `-check-action`,
   `-check-diverge`, bounds, witnesses in trace format; `%check-action`, `%replay`. Test layers
   3–8 for actions.
3. **State machines and time ties.** Snapshot/restore for the state executor; dispatch-order
   choice points; `do` interleaving; `-check-state`. Test layers 3–8 for states.
4. **Dynamic reduction.** Concrete footprint instrumentation; DPOR backtrack points; the
   effectiveness ratchet moves and is re-adjudicated. Statement-level granularity is considered
   here, if a property has asked for it.
5. **Across objects.** Following signals to other objects' running machines, whose queues and
   configurations join the explored state. Its own note.

Stages 1 and 2 are the minimum that answers the safety-case question for actions. Stage 3 is
where most systems models live (a state machine per component) and should follow directly.

## What this does not change

- The executor's semantics. The default run is the same run; the checker explores the schedules
  the library admits and the executor could have taken. Where the executor is wrong against the
  library — the three known failures the oracle records — the checker is equally wrong, and
  fixing them is upstream of this work, not part of it.
- The compliance map's honesty. A row marked approximate because the runtime picks an order the
  specification does not stays approximate; the checker makes the openness *visible*, it does
  not make the pick faithful. The row gains a pointer to the divergence report.
- The SMT layer. Scheduling and value nondeterminism remain separate questions with separate
  tools here; composing them is [its own note](smt-model-checking.md).

Under the [analysis framework](analysis-framework.md) this checker is the `check` engine,
selected by `-engine check`; its verdicts are *bounded* within the state and depth budgets it
names and *witnessed* for a violation it replays, and `explore` beside it under `-engine all`
is its referee.

## Alternatives considered

- **A race detector only.** Record, during the single default run, the unordered pairs of
  moves whose footprints intersect, and warn. Linear cost, no snapshot machinery, and it finds
  most under-synchronized models. It is not this design's competitor but its first deliverable:
  the footprints of stage 2 make it a one-file addition, and it should ship as an opt-in
  warning before the checker does. What it cannot do is answer
  "does any schedule violate R" — it reports that a race exists, not what the race can cause —
  and that is the question the safety case asks.
- **Translate to an existing model checker** (Promela/SPIN, TLA+, UPPAAL for time). Attractive for
  the algorithms, but the translation would have to re-implement the executor's semantics — every
  frame rule, pin binding, delivery queue and classifier-behavior binding — in another language
  and keep the two in step, which is the drift this project's architecture forbids for its own
  IR. Exploring the executor itself keeps one semantics.
- **Exploring `Step()` as the unit.** Simplest to implement, since `Step()` exists; wrong, because
  it interleaves at a boundary the library does not define and would report divergences that are
  artifacts of the stepper. Rejected in favor of the performance as the unit.
- **Symbolic execution.** Would answer data and scheduling nondeterminism together. The value
  domain includes quantities with units, collections, strings and object graphs; a symbolic
  state over those is a much larger project than either the SMT layer or this checker, and
  neither is a prerequisite for it. The [SMT bounded model checking](smt-model-checking.md) note
  takes the narrower route: a bounded transition relation over the solver's already-translatable
  subset, rather than a symbolic executor.
