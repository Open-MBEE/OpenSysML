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
unordered and took one by its scheduling rule. `ChoiceKind` names the twelve:

| Kind | Where it is noted | Alternatives, canonically |
|------|-------------------|---------------------------|
| `ChoiceTokenOrder` | `ActionExecutor.noteTokenOrder` | the tokens that could act in the step, by ID; the one stepped first is taken |
| `ChoiceDecisionBranch` | `ActionExecutor.noteDecisionBranches` | the successions whose guards hold, by declaration position |
| `choiceWriteOrder` | `stepWriteLedger.noteChoices` | the tokens that wrote one feature in one step; the write that stood is taken |
| `ChoiceTransition` | `StateExecutor.chooseTransition`; `pickBranch` (`state_route.go`) | the transitions one event enables out of one state, by declaration position; or the branches of a `choice` or `junction` pseudostate enabled when its guards are read, labelled `choice <name>` or `junction <name>` |
| `ChoiceRegionOrder` | `dispatchInOrder`, drawn on the front of `state_unit_front.go` (`on <event>`); `chooseDoAction` (`do round at t=…`); `fireJoinIncoming` (`join <name>`) | the next unit — a source's exit, a segment's effect, a target's entry — of each firing one occurrence selected across regions, by source in declaration order; the one performed first is taken, and the entries and exits a firing nests are drawn on its front — likewise the states whose do behaviors are due in one round, and the sources of the transitions into a join that fires |
| `choiceEntryOrder` | `enterRegionsInto` (`entering <state>`), `enterForkBranches` (`fork <name>`), a history's restore | the next entry unit of each region, branch or restored region of a composite state, in declaration order; the one performed first is taken. A unit performing no behavior rides with the performing unit beside it |
| `choiceEntryStep` | `StateExecutor.entryStep` (`entry at t=…`) | a free dispatch due at the instant, followed by the held entry cascades that can resume; the free dispatch is listed first and the selected alternative is taken |
| `choiceExitOrder` | `exitState` (`exiting <state>`) | the next exit unit of each region a state leaves, innermost first within a region, in declaration order; the one performed first is taken |
| `choiceStepOrder` | `StateExecutor.chooseStepOrder` | the move of a due do behavior and a dispatch due beside it; the one that goes first is taken |
| `choiceStatementOrder` | `stmtEngine.pickStatement` (`statements in <body>`) | the statements of one body that no succession or control structure orders and that may run next (`lower.StatementOrder`), by declaration position; the one run first is taken |
| `ChoiceDueOrder` | `Context.runDue` (`advance.go`); the checker's run, one executor holding the turn until it has no move at the instant | the executors due at one instant, in creation order; the one run first is taken |
| `choiceDispatchOrder` | `StateExecutor.nextEvent`, when the queue leaves several events unordered at its head | the events due at one instant the library does not order — time triggers with each other, a time trigger with a pool event of the same timestamp — labelled as the queue labels them; the one dispatched first is taken. A completion event goes before any of them and pool events keep their arrival order (`earlierFirstIncomingTransferSort`), so neither is a choice |

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
run. `Context.Notes`, `Choices` and `unevaluableGuards` read them back; the executors' `Notes` and
`NoteCount` let a caller driving a run call by call see what one call noted, which is how the REPL
ends `%step`, `%continue` and `%advance` with a count; `NotesSince` reads the notes past such a
mark without copying the ones before it.

Region order is drawn per dispatch, not per event: `dispatchInOrder` draws among the candidates
still active before each firing, since a reaction may leave a sibling's leaf, and both the
broadcast of a queued event and the polling of change triggers (`state_change_trigger.go`) go
through it. The choice is labelled by the occurrence dispatched, not by the trigger of whichever
region was drawn first, so the label is the same under every policy.

An entry-step choice is recorded as `entry at t=<instant>` when a state entry cascade has been
held by a non-default run-to-completion scope and a dispatch is due at the same clock instant.
The alternatives list the free dispatch first, when one exists, followed by the held entries
that can resume. A dispatch whose transition owner lies within a held scope is not offered as
free; tied events are filtered to the free alternatives, and a dispatch with no free alternative
does not appear. Fixed policies select the first alternative, so they dispatch before a held
entry when that dispatch is free. Exploration snapshots the entry boundary and enumerates each
alternative; replay parses the `entry at t=` location and applies the recorded pick. Once the
selected dispatch or held entry settles, ordinary transition, region-entry and exit choice
points continue as usual.

A `choice` pseudostate's branch is drawn on arrival: `resolveChoice` reads its guards once the
incoming segments' effects have run, so which branches are enabled can depend on those effects,
and with two or more enabled the draw is a `ChoiceTransition` at `choice <name>` that `explore`
enumerates and a seed replays (`TestExploreDynamicChoiceBranches`). A junction's guards are read
statically, when the transition is selected — against the data as it stands before the incoming
effect — and with two or more enabled the draw is likewise a `ChoiceTransition` at
`junction <name>`, made and recorded only as the transition fires (`settleDraws`), after the
region order among several candidates and after the transition's own guard is read again: a
candidate another region's reaction disarms draws nothing, and no branch guard is read again:
the route beyond each enabled branch — through any further junction — is settled with the
transition, so the branch drawn is taken along it though another region's effect since changed
what those guards read (`TestExploreStaticJunctionBranches`,
`TestExploreJunctionDrawnAsTransitionFires`, `state_junction_guards_read_once`,
`state_junction_beyond_a_draw_read_once`). A history's default transition through such a
junction draws and records the same way (`TestExploreHistoryDefaultThroughJunction`).

One thing that looks like an opening is determined and is never recorded. A composite state's
completion: once its do behavior and every one of its regions have
ended, its nil-trigger transitions are queued as completion events at the current instant, ordered
as a leaf's are, and the machine ends only when its own top-level regions are all at `done`
(`completeIfDone` → `scheduleCompletedComposites`).

## The standing of a choice point (`order_dependence.go`)

A default run (`reverse`, `declared`, `seed:<n>`) takes one linearization and reports its values,
and a value is a fact of the run only up to the partial order the library fixes (KerML 1.0 §7.4.9:
`HappensBefore` holds between the occurrences a succession joins, send before accept, and nothing
else). So every choice point is *classified* as it is recorded, from the lowered footprints
(`lower.Footprint`, the read/write/send/accept projection the checker's persistent-set reduction
already uses — `check_reduce.go`, `future_footprint.go`; there is no second conflict relation):

- `classifyChoice` takes one footprint per alternative and the relation the checker would use
  between them (`Context.relation` for executors of different objects), and asks
  `Footprint.DependentBy` pairwise over the footprints less control convergence (meeting at a join
  or both completing a performance changes no value). No dependent pair: the alternatives commute,
  the choice is `Independent`, and the run's values do not depend on it. A dependent pair: the
  choice is unresolved, and `Touches` names the places whose values the order decides
  (`touchMeeting`, `Relation.PlaceMeets`): a place both alternatives write, and, where a write of
  one meets a *read* of the other, what the reading alternative writes from that read — the
  statements of its body whose flow reads the place (`lower.Footprint.Flows`, one data flow per
  statement, `Footprint.WritesReading`), or every write of the alternative when the read is not a
  statement's (a guard's) or the move is described whole. The place read is not touched by the
  meeting: one write of it stands whichever order ran, so a mark on it would assert an openness
  exploration refutes (`action_order_sibling_read_of_racing_write`: `z` open, `x` bare). A pin is
  each performance's own, delivered to rather than raced for, and is neither touched nor a read
  that touches (`shared`). A pair that meets only on a channel or through a dynamic target
  (`Footprint.Dynamic`: a property read or a chained target the declarations do not decide) touches
  every write of both, conservatively.
- `classifyExclusive` is for alternatives of which one runs and the others do not — a decision's
  holding branches, the transitions one event enables, a pseudostate's branches: never independent,
  touching every write along any of them (`branchFutures`, `TransitionFootprints`).
- `classifyBy` classifies a statement order by `lower.StatementOrder.Dependent`, which also keeps
  the `then` successions written among the statements.

Per site, the footprint of an alternative is: a token, `standing` where it stands before the step
(`ChoiceTokenOrder`, replayed and explored steps included via `stepOrder.able`); a write, the
feature written (`choiceWriteOrder`); an executor due at the instant, its `executorFuture`
(`ChoiceDueOrder`, in `advance.go` and the checker's `drawOwner`); a pool occurrence, the
transitions out of the active states its trigger reacts to (`eventFootprint`,
`choiceDispatchOrder`); a due do move against a dispatch, `doStepFootprint` against
`dispatchFootprint` (`choiceStepOrder`, `choiceEntryStep`); the states whose do behaviors are due in one round,
`doStepFootprint` each; the sources of the segments pending into a join, each segment's own
footprint (`lower.JoinSegmentFootprints`: its trigger, guard and effects with the exits from its
source up to the join's owner — the owner's exit and the join's way out follow whichever fires
first, so they are left out); a region's queue on a unit front, what
its units may touch — the firing's transition, the region's entry (`regionEntryFootprint`: entering
and starting any state under it, its entry transitions' effects, the pseudostates it crosses) or the
exits of the states it leaves (`exitFootprint`) — carried on the `unitQueue` and drawn against on
`drain` (`ChoiceRegionOrder`, `choiceEntryOrder`, `choiceExitOrder`). A unit the graph does not name
is `unknownFootprint`, which conflicts with everything. A guard verdict or an invocation result
that depends on a body's statement orders (`choiceGuardOrder`, the `result of …` statement order)
is a choice among exclusive outcomes and stays unresolved.

What the classification feeds: `Context.OrderDependent(feature)` counts the unresolved choice
points touching a feature, `Context.UnresolvedChoices()` all of them, and
`OrderDependenceText(n)` is the marker every surface prints a touched value with —
`x = 1 (order-dependent; 2 choice points — explore shows the set)` — in the REPL
(`namedValue`, `formatFeatureValue`), the CLI (which reports through the REPL session), the trace
(`ChoicePoint.Standing`: `independent`, `order-dependent: x, y`, or `order-dependent`), the gRPC
surfaces (`order_dependent` on `FeatureValue`, `ExecuteActionResponse` and
`ExecuteStateResponse`; `independent` and `order_dependent` on a `choice-point` diagnostic; the
`order_dependence` capability) and the Python and Node clients. A value no unresolved choice touches
prints exactly as before. The classification is per choice point; what a value computed from a
touched one stands on is the next section's.

## What a value stands on (`order_marks.go`)

A statement that reads an order-dependent value and writes a third feature is not itself a
choice point, yet the third feature's value depends on the order as much as the first's. So the
run keeps, beside its notes, *marks*: per place written (`lower.Place`), the unresolved choice
points its current value stands on. `Context.OrderDependent(feature)` reads them — the choice
points marked on the name, alone or as the last segment of a written path, among those the run
still holds as unresolved — so the marker on every surface is the same typed count, and the
`choice-point` diagnostic's own `order_dependent` list stays what that choice touched. The marks
describe the value, not the feature's history, and follow one rule, derived from the lowered
footprints already carried (`lower.Footprint`, `lower.StatementOrder`,
`ActionGraph.BindingFootprints`, `lower.ExpressionFootprint`), never from a second dependence
tracker and never by re-reading declarations in the runtime:

- **Seed.** A choice point noted unresolved marks every place it touched (`ChoicePoint.touched`,
  the places behind `Touches`) with itself (`orderMarks.seed`, from `Context.noteFrom`): another
  order may have written them otherwise. A value reported under a name reads the marks of the
  places that name denotes as `choiceTouches` resolves them (`placeNamed`: a bare name is the
  behavior's own feature, `this.<name>` the performing object's, `<part>.<name>` a feature of the
  object a part of the behavior denotes, which no performer owns). A place is the feature written
  whatever path reaches it: `assign sensor.raw := 1` through the part `sensor` writes the place of
  `Sensor::raw` (`footprintBuilder.path`), the same place a later `sensor.raw` reads, so a chained
  feature stands open where a plain one would (`action_order_chained_write_of_racing_writers`,
  `action_order_chained_two_level_write`).
- **Propagate.** A move with footprint `fp` that writes `w` leaves `w` standing on what `fp` read
  — the union of the marks of every place in `fp.Reads` — together with the *control* the move
  runs under; a footprint the declarations do not decide (`fp.Dynamic`: a property read, a chained
  target, an invocation of a declared behavior) reads the order-dependent set entire,
  conservatively (`Context.propagateMarks`, `Context.standingOf`). An assignment propagates by its
  statement's footprint as it completes (`stmtEngine.statement`, `stmtHost.statementFootprint`); a
  performance's inputs stand on what they are bound from as it begins and its outputs, as it ends,
  on those and on what its body left on its own pins (`performances.markPerformance` over
  `BindingFootprints`: features, pin bindings, flows and the accepted payload, less the body's
  statements, which are recorded one by one). A feature valued by an expression is rewritten with
  what its expression read as it is derived, the behavior's own (`deriveBodyCell`) and an object's
  (`Instance.materializeDerived`: `sensor.scaled = raw * 10` stands where `raw` does) alike
  (`Context.deriveMarks` over `lower.ExpressionFootprint`). Reporting reads under the run that
  produced the values: `ActionExecutor.ResultsWithError` derives what a part's object holds under
  the executor's own run state (`enterRun`), since a derivation begun as a run of its own would
  install a fresh state and the marks of the run just ended would be gone before the surface read
  them — which is how a value reached through a part was once reported bare.
- **Clear.** Since the write *replaces* the marks of `w`, a write that reads nothing
  order-dependent, under no control, clears `w` from that point on: the mark describes the value.
  It replaces the marks of the one declaration it writes (`samePlace`): a same-named feature of
  another declaration may hold another value, so it keeps its marks, though a read of either
  stands on both (`Place.Conflicts`) while the declarations leave them apart.
- **Control.** A guard or condition that reads an order-dependent value makes everything the
  selected continuation writes depend on the order, though the guard drew nothing: a decision
  puts its token under the marks of every guard it read to decide — those that failed before the
  one it took, and all of them when it falls back to its unguarded succession
  (`ActionExecutor.guardsStanding`, `Token.underControl`) — until a node every succession out of
  the decision reaches (`commonReach`), where which branch was taken no longer decides the path;
  the tokens a fork spawns inherit the control of the token it split, and a join's token the union
  of what arrived (`split`, `synchronize`, `unionControl`), so a branch keeps its decision however
  it is shaped inside; an `if`, `while`, `until` or `for` puts its block or iterations under its
  condition's or collection's marks (`branchFrame.control`, `loopFrame.control`); a transition's
  firing — exit, effect, entry — runs under its guard's marks (`StateExecutor.taking`). A draw
  among exclusive alternatives is control too: a decision's branch, a transition chosen among
  several enabled, a statement an order picked (`stmtEngine.statementUnder`) run under the choice
  that drew them, so their writes stand on it rather than clearing its seed. Mere convergence adds
  nothing: a merge or join is where control from the decision ends, never a dependence of its own.
- **Transfer.** A `send` stamps its message with what its payload expression stood on where it
  was built (`Message.dependence`, in `Context.send`), and the `accept` that takes it writes its
  parameter standing on that and on the control at the accept (`Context.acceptMarks`, from
  `awaitSignal` and `bindAcceptPayload`): the transfer carries the dependence from the sender's
  side to the receiver's.
- **The clock.** A choice among the moves of one instant of the shared clock (a transition
  drawn among several, region, due, dispatch, entry, exit and step order) orders none of that
  instant's other writes after its alternatives, so a seed it left stands on the places it touched
  until the clock advances, whatever writes them meanwhile (`markSeed.clocked`); a write at a later
  instant clears as any other. Action-flow choices need no such hold: token and write order are
  noted after the step's writes, and a decision or statement order holds through control.

The Kernel Semantic Library is the basis: `Occurrences.kerml` defines `HappensBefore` and
`Performances.kerml` derives it from successions, so a statement after a join reads the value its
predecessors left and its result is a function of theirs — open exactly where theirs is;
`Transfers.kerml` orders a transfer's send before its accept, so what the accept binds is what the
sender computed. Nothing else orders two moves, which is why a seed stands until something ordered
after every alternative rewrites the place.

Snapshots, held images and a compound transition's rollback capture the marks beside the notes
(`runStateCapture`, `moveMark`, `imagedRun`), so a replay or an undone move leaves no mark of a
write it discarded. Reporting never changes the run: marks are read by the surfaces alone.

Approximations, all conservative where they err: a pin's place is its node's (`Place.Node`), so
the performances a repeated or re-entered node makes share the marks of their pins, and a body
reading a name its declarations do not resolve reads every value of that name; a seed held
for the clock's instant marks a same-instant rewrite from a literal that another order could not have changed;
the guards a route reads on its way through a junction or choice pseudostate are not control for
what follows them, and a calc or constraint body propagates nothing (the expression reading its
result stands on what it reads). A feature valued by an expression is computed as it is read,
and stands on what its lowered expression reads at that moment, through features so valued in
turn (`lower.ExpressionFootprint` follows a bound value's reads; `deriveMarks` rewrites the
feature's place as the derivation lands), so a write from it stands on those places as a write
from them directly would; a feature given an initial or default value is written once, as its
owner starts, and stands on what that write read.

## Repeated action-step performances

When a node declares an exact count greater than one, the arriving action token is split into
sibling tokens, one per performance. They use the existing `ChoiceTokenOrder` choice point: no
new scheduling policy is introduced, and declared, reverse, seeded, replayed and explored runs
continue to select among the same steppable tokens. Each token begins a fresh frame for the node,
so its local features are independent, while writes to features of the owning action remain
shared. Data flows, bindings and connections involving a repeated node's pins are refused.

The owner frame holds a per-node completion barrier. A repeated performance that finishes retires
its token without taking the node's outgoing successions or delivering its data flows. The last
one to finish releases the barrier and advances the node once, so every target follows every
source performance and the owner continues once. A nested flow runs separately inside each
performance's frame. Repetition identity and the live barrier are part of snapshots, held images
and the execution state key; otherwise replay and exploration could merge states with different
remaining performances.

A count of zero passes through without beginning a performance, trace events or data flows. It is
only order-safe at a flow boundary: a zero-count step between two real steps leaves their order
open and is refused. Any step count that is not fixed, or succession, guard, control-node or pin
interaction the multiplicity checker cannot support, is refused before the node runs. The
two-reading treatment of unwritten succession ends is documented in the
[semantic oracle](../../project/behavior-semantic-oracle.md#repeated-action-steps-and-shared-writes).

## Leaf action-body interleavings

A leaf action body — a node's initial feature values and its direct statements, with no flow of
its own — runs through the statement engine (`usageWork.perform`, `stmtEngine`). Its performance
encloses separate times the [semantic
oracle](../../project/behavior-semantic-oracle.md#a-leaf-bodys-start-shot-and-its-assignments-another-performance-may-run-between-them)
derives: the start shot, where `attribute t : Integer := c` snapshots `c`, and one subperformance
per statement, an `assign` writing when it ends. Another performance may run between any two.

- **Atomic units.** One initialization (every initial value of the body, read at its start
  shot) and one statement are each one move: an assignment reads its value and writes it with
  no boundary between, since the library places nothing between them. A loop iteration is one
  move; a step of a flow the body drives is one move unless its own body divides, as a token of
  the outer flow's does, so the repeated performances of a counted step in a loop or branch body
  and the branches of a fork there interleave as they do at the top level.
- **Scheduler boundaries.** Where a body divides, the run yields after its start shot and after
  each statement (`Context.bodyPerformed`, `Context.yieldBody`, `bodyPause.yielded`): the token
  goes back to the step's steppable tokens with its body frame held, and the next pick among
  them is an ordinary `ChoiceTokenOrder`. No new choice kind is added, so a witness, a replay
  file, a snapshot and a held image name a body boundary as they name any token move, and the
  body frame is resumed where it paused.
- **Where a body divides.** `lower.BodyDivides` lists a body's moves (its start shot, then each
  statement, a conditional's guard by itself and a loop counted twice) with their footprints
  (`Footprint`, the reduction's), and the footprints of every node of the outermost flow and its
  nested flows; the node's own, with its per-performance pins removed, when it may run beside
  itself. When the flow can hold two tokens at once and two or more of the body's moves are
  dependent on one of those footprints, the body divides. With at most one, every interleaving
  inside the body only reorders independent moves, so the body stays one move and the state
  space does not grow.
- **Guards.** An `if`'s guard is evaluated before its branch (`IfThenPerformance`), so a dividing
  body yields between them (`Context.guardPerformed`); the branch taken is kept in its
  `branchFrame` across the pause. A state's do body yields there only under one-move schedules,
  so `reverse`, `declared` and seeded runs keep their state traces.
- **Do bodies stating a flow.** A `for`, `while` or `if` node of a do body's stated flow
  yields after each iteration and branch statement, as the same statement in a statement-list
  body does (`bodyRun.nodesYield`); the flow counts the node's yield as a move
  (`ActionExecutor.yieldedIn`), so adding `then` takes no interleaving away. A token of a flow
  such a node drives moves only through that node's work, never as a move of the outer flow
  (`Token.drivenUnder`), so a replayed witness meets the same choices it recorded.
- **Callees in executors of their own.** An action a step is typed by, or a body performs, runs
  in its own `ActionExecutor`, outside the graph `BodyDivides` reads. When the body or flow
  driving it goes one move at a time, the callee's start shot is a boundary
  (`Context.startShotMove`), its own bodies divide where two of their moves may touch what they
  do not hold (`lower.BodySharesMoves`), and its flow pauses after each move when two of its
  moves may (`lower.FlowSharesMoves`, `Context.tokenStepBody`); the attributes and `in` parameters
  the callee's definition declares are its performance's own, not shared, while each write to an
  output reaches the caller's pin and its streaming flows as it is made, so outputs are shared. A pause inside a body-driven
  token propagates to the body driving it. A seeded run whose step holds another token pauses
  there when `drawYield` says so (`Context.drawsTokenSteps`), so seeds reach those outcomes too.
- **Executors on one clock.** Separate executors — object behaviors, state machines, actions
  started together — still interleave by whole turns: the executor the due order draws runs
  until it has no move at the instant (`Context.runDue`, `invocationRun.turn`). Their moves
  within one instant are not interleaved, which spec compliance records as approximate.
- **Ordered and unordered statements.** A succession inside a nested flow orders its steps, so
  no boundary reorders them; a boundary lets only another performance in. The direct statements
  of one body that no succession or control structure orders are unordered
  ([derivation](../../project/behavior-semantic-oracle.md#direct-statements-of-one-body-no-succession-orders-each-is-performed-in-which-order-is-open)).
  `lower.StatementOrder` lists, before each statement runs, those that may run next: a statement
  `then` links waits for the one before it, a local declaration, `return` or `perform` keeps its
  place, and two orders that only swap adjacent independent statements (by the reduction's
  footprints) are one. Two or more candidates are a `choiceStatementOrder`
  (`stmtEngine.pickStatement`); a compound statement already started yields at its inner
  boundaries so a dependent sibling may run between them. `declared` and `reverse` run
  a nested body's statements first to last as before, and an action definition's own statements
  in the token order each policy already gave them (`reverse` last to first), so their results
  do not move; `explore`, the checker, replay and seeds
  choose. A terminate action usage's implicit terminate is one of its body's statements in this
  sense. `explore` enumerates without partial-order reduction, so a body of many dependent
  unordered statements may exhaust its run budget, which it reports as incomplete.
- **Policies.** `explore`, replay and the checker step one move at a time and yield at every
  boundary of a dividing body. `reverse` and `declared` never yield, so a body runs whole as
  before and their results do not change. A seeded run whose step holds another token yields
  at a boundary when `scheduler.drawYield` — a hash of the seed, the token's ID and the
  boundary's ordinal — says so. That draw does not consume the generator the run's picks
  come from, so one seed still reproduces one run.
- **Checker and SMT.** The checker's moves are the executor's, so it reaches every interleaving
  exploration does, under its reduction and state merging; a paused body frame is part of the
  state it keys. The SMT encoding performs a body as one move and refuses a flow whose body
  divides (`Flow.checkBodyInterleaving`, `UnsupportedError` construct `body interleaving`), so
  it reports such a flow as not covered rather than encoding one order.

## Policies (`scheduler.go`)

A `SchedulePolicy` is parsed from one spelling and printed back to it:

| Spelling | Token order in a step | `pick` (branch, transition, region) | `pickDue` |
|----------|----------------------|--------------------------------------|-----------|
| `reverse` (default, zero value) | reverse spawn order | first in declaration order | last created |
| `declared` | spawn order | first in declaration order | first created |
| `seed:<n>` | shuffle of the tokens not parked; a dividing body yields where the seed's hash says | uniform draw | uniform draw |
| `replay:<file>` | the witness's `step n: …` line, then `reverse`'s pick alone | the witness's line, then `reverse` | the witness's line, then `reverse` |
| `explore[:runs=N,depth=D]` | the exploration's plan | the exploration's plan | the exploration's plan |

`reverse` is exactly what every run did before policies existed, so every `.expected.json` and
`.trace.golden` recorded before them still holds unchanged; that is the invariant the whole design
is built to keep. Under `reverse` and `declared` a region-order pick is declaration order and is
still reported — a fixed policy resolves the choice, it does not remove it — and so are the
entry-order and exit-order draws, one `choice` line per draw in the trace golden of a fixture
entering or leaving a state of two or more regions, its event order unchanged
([region-order scheduling](region-order-scheduling.md)). So is the step order: the fixed
policies run a round of the due do behaviors whole and dispatch after it, and
`StateExecutor.noteRoundStepOrder` records each due do move the round takes ahead of a dispatch
that would act as the `choiceStepOrder` the unit-at-a-time policies draw there, taken as the do
step, so a fixture with a do behavior beside an acting dispatch carries one `choice` line per such
move and its event order is unchanged.

`SchedulePolicy.start` begins the resolutions of one run as a `scheduler`. A seeded one carries a
`math/rand/v2` PCG the run consumes draw by draw, so the same seed replays the same run on every
platform; `scheduler.mark` saves and restores that state around a probe, so previewing does not
move the generator. `Context.SetSchedule` sets the policy runs started from then on draw under; a
run already under way keeps the one it started with, and `explore` is refused with
`ErrExploreUndriven` because it is not a policy one context runs under (below).
`Context.Reschedule` is the same change reaching the runs driven call by call too — the clock's
and those of the behaviors the objects run — each given a scheduler started under the new policy
where it stands, so a persistent session's turns from then on choose as a fresh run would; a
snapshot restores the schedulers the runs had along with their marks.

The scheduler lives in the run's `runState` beside the budget and the notes. A run driven call by
call — a REPL `%action` or `%state` session — owns its `executorRun.state`, installed for each
call by `beginExecutorRun` whatever ran in between, so a seeded debugging session draws from its
own generator and an interleaved run neither consumes its draws nor inherits its notes.

`replay:<file>` (`replay.go`) is the policy a witness is run again under: the file's choice lines
— `ChoiceTaken.String` spellings, one per line up to the first blank line, so a checker's witness
file with a trace body after its header serves as it stands — are followed one move at a time,
each having to name the step the run is at and pick among the alternatives it offers, and once
they are spent the run picks as `reverse` does, still one token a step — the witness was written
by a run stepping so, and a sweep letting every token act would leave another trace. A line the
run cannot follow, or one left over at the end, is recorded as the run goes and reported by
`Context.Unfollowed` as a `ReplayError` naming the move, its choice and what the run faced; the
run is never quietly turned into another linearization. A refused move changes nothing: a transition draw is refused before the dispatch
fires (`broadcastEvent`), and a `choice` pseudostate's, drawn only once the compound transition's
exits and incoming effects are made, or a join's, drawn one incoming segment at a time, undoes
that move whole — exits, effects, the do behaviors the exits abandoned, what was traced and noted
(`moveWhole` marks the move with a `moveMark`, kept or undone by the refusal; marks nest, a
`choice` on the way out of a join handing the do behaviors it ended to the join's). The model
checkers replay every `sat` witness under it before claiming a violation.

## Exploration (`explore.go`)

`explore` enumerates every linearization within a budget by replaying whole runs, each from a
fresh `Context`:

1. The first run records each choice point it reaches as an `exploreSlot` (a `pick` among `n`
   alternatives, or which of the tokens able to act a step tries next) and takes the first
   alternative at each.
2. `unexplored` leaves one prefix per slot the run owns with an untried alternative — the record
   up to that slot with the alternative advanced — and a later run replays the prefix and takes
   first alternatives past it. The queue (`explore_queue.go`) runs the prefixes in plan order:
   those departing from the first run at one choice before any departing at two, and among them
   the earliest choice varied first. So the first run's choice points are each varied once, from
   the first, before any is varied twice: a choice met early with a long tail of choices behind
   it is varied by the second run, not after every order of the tail, and a `runs` budget of one
   more than the first run's choice points varies each of them at least once.
3. A replay that does not meet the choice points its prefix planned — a different number of
   alternatives, or tokens able to act the plan did not find so — is `ErrExplorationDiverged`,
   and no outcome set is reported, since one could not be trusted.
4. Runs stop when every alternative within depth is tried, or when the `runs` budget is reached;
   a slot resolved past the `depth` budget takes its first alternative and is not the
   exploration's to vary, so a choice point deeper than `depth` is never varied however many
   runs remain. Either bound reached makes the `Exploration` incomplete, and
   `Exploration.Status` says which; nothing is silently truncated. The default budget is
   `DefaultExploreBudget`, 1024 runs and 64 choice points per run; a witness lists every choice
   point its run met, so its length sizes `depth`.
5. A construct the executors perform in one order without a choice point is a *coverage gap*
   (`CoverageGap`, `coverage_gap.go`): the run records it on its context (`Context.noteGap`),
   the queue folds the gaps of every run into `Exploration.Unenumerated`, and the checker folds
   them into `CheckReport.Unenumerated`. A gap makes the exploration incomplete as a budget does —
   `Exploration.Complete` is false while either stands — and `Status` names what was not
   enumerated (`incomplete after N runs; not enumerated: …`); `analysis` reports the result's
   standing as *observed*, never *proved*. No construct records a gap today: the one that did —
   the performances of a repeated step in a loop or `if` body, once run one move apiece — are
   sibling tokens of the body's flow (`splitRepeatedStep`), so their interleavings are token-order
   choice points like a top-level step's, and the table below finds every other order the
   executors take without a choice point fixed by the library or canonical. A construct found to
   perform an open order as one move is added as a `CoverageGap` and recorded where it runs. The
   REPL's `ExplorationVerdict.Unenumerated`, the wire `ExplorationStatus.unenumerated` and the
   Python and Node clients' `unenumerated` carry the gaps by identifier, and a conformance case
   states the gaps exploring it must record under `exploreGaps`.

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

A root executor creation failure, or a pure structural start check that fails before any root
model behavior runs, is a `SetupError`: it fails the exploration and produces no outcome. Errors
from entry behaviors, entry guards, state entry actions, do-behaviors, completion, or later model
behavior are runtime error outcomes, even when they occur during initialization;
`FailedLinearizations` counts the runs represented by error outcomes. A complete exploration
containing any such failure is observed, not proved. Action-step-multiplicity refusals occur when
the step is reached, so they are runtime error outcomes, never a `SetupError`.

Outcome probabilities describe only the model's weighted draws. A run records a trie of choice
prefixes; weighted nodes sum their children with normalized model weights, while scheduling
nodes take the minimum and maximum over their explored alternatives. An unexplored scheduling
alternative contributes zero to the minimum; an unexplored weighted alternative contributes
zero to its sum. The root vectors give each outcome's `ProbabilityRange`, computed in
O(depth × outcomes) vector space before outcomes are sorted. A probability is absent if no
committed run made a weighted pick, and an incomplete exploration has lower bounds only when it
did. Random-function draws still come from the model seed and are not enumerated, so these
probabilities are conditional on those draws. Checker `Violation.Mass` is a separate measure,
the path mass a check reports per violation.

## Orders taken without a choice point

Every order an executor takes is either a recorded choice above or one the library fixes, or one
with no observable alternative. The sorts, map iterations and sibling loops in the executors and
the lowering fall into these classes:

| Site | Order | Why it is not a choice |
|------|-------|------------------------|
| `clock.go` `Clock.Waits`, `action_executor.go` `visibleArmedWaits`, `tokenWaits` | clock waits by due instant | a later instant cannot occur before an earlier one (`Clocks.kerml`: `Clock::currentTime` is monotonic over `TimeInstant`s, and a wait ends at its instant); waits tied at one instant are drawn as `ChoiceDueOrder` and `choiceDispatchOrder` |
| `executor_common.go` `EventQueue.Events`, `completionsOf`, `Tied` | queued events by arrival (`Event.ID`) | the order events were put into the pool is observed, not drawn; events the library leaves unordered at one instant are offered as `choiceDispatchOrder`, and an acting dispatch against a due do move as `choiceStepOrder` |
| `action_executor.go` `tokensByID`, `waitingTokens`, `timeWaits`; `action_terminate.go` `dropTokensIn`, `beginPending` | tokens by `Token.ID` | an enumeration order for reporting and for stable alternative labels; which steppable token acts is `ChoiceTokenOrder`, and the tokens a termination drops all end in the same step (`Occurrences.kerml`: the suboccurrences of a terminated performance end with it), leaving no value an order could change |
| `action_choice.go` `stepWriteLedger.noteChoices`, `noteTokenOrder` | alternatives by token | the canonical labelling of the recorded `choiceWriteOrder` and `ChoiceTokenOrder` themselves |
| `action_executor.go` `setFrameFeatures`; `action_frame.go` `takeDeliveries` (pins), output names | feature or pin names | each name is a distinct feature, so the writes commute and the result is one however they are applied; the names also order error text |
| `action_executor.go` `synchronize` (indices descending) | slice deletion | mechanical: the consumed tokens are removed together within one step |
| `succession_order.go` `orderFeaturingInstances`, `orderFeatureValues` | instances by `Instance.ID` | the instances a `BehaviorOrder` relates are matched end to end by identity; the succession itself is the `HappensBefore` the order enforces (`Occurrences.kerml`), and which of several unrelated performances starts first is the executors' `ChoiceDueOrder` and `ChoiceTokenOrder` |
| `statements.go` `stmtListFrame.candidates` | runnable statements by index | the candidate set handed to the policy; which runs next is `choiceStatementOrder`, and `lower/statement_order.go` decides which pairs may reorder at all (a statement that keeps its place — a declaration read by name after it, a `return`, a performed reference — is ordered by the body's `then` successions or by its data dependence, KerML 1.0 §7.4.9 succession) |
| `lower/behavior_order.go` `BehaviorOrder` | successions outside a body | lowered as `HappensBefore` constraints the runtime enforces, never as an order it draws |
| `state_run_to_completion.go` `entryStep` | a held entry against a free dispatch | drawn as `choiceEntryStep` |
| `state_executor.go` `runDoRound` | the due do moves of a round against the dispatch after it | recorded as `choiceStepOrder` by `noteRoundStepOrder`, taken as the do step (above); the moves among the due do behaviors are `chooseDoAction`'s draw where two or more are due |
| `action_step_multiplicity.go` `splitRepeatedStep` | the `n` performances of a repeated step in a loop or `if` body, among themselves and against the steps outside the body | drawn as token order: the performances are sibling tokens of the body's flow, one repetition group, and the body yields between its moves (`lower.BodyDivides`), so `explore` reaches every interleaving a top-level `[n]` step admits |

## The conformance contract

Every element above has a test surface, documented for authors in
`internal/exec/runtime/testdata/conformance/README.md` and summarised in
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
