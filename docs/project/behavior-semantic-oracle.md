# A semantic oracle for action and state execution

The pinned OMG pilot evaluates expressions but executes neither actions nor state machines
([pilot execution referee](pilot-execution-referee.md)), so every behavioral row in
[spec compliance](spec-compliance.md) is refereed by this repository's own conformance models,
trace goldens and robustness cases — all of which were recorded from the executor they check.
This record adds a referee that is not the executor: for each rule below, a minimal model whose
expected outcome and ordering are **derived by hand from the bundled library text** (Kernel
Semantic Library `Occurrences.kerml`, `Performances.kerml`, `ControlPerformances.kerml`,
`StatePerformances.kerml`, `TransitionPerformances.kerml`; Systems Library `Actions.sysml`), with
the sentence that justifies each ordering constraint cited, and the orderings the library leaves
open named as open.

The oracle cases live beside the other conformance cases under
`internal/exec/runtime/testdata/conformance/` and run through the same harness
(`go test -run 'TestExecutionConformance|TestExecutionTrace' ./internal/exec/runtime`). Where the
executor meets the derived expectation, the case also carries a `.trace.golden` that
regression-locks the executor's linearization. Where it does not, the derived expectation is kept
in the `.expected.json`, the case is listed in `known_failures.txt` so the harness reports rather
than runs it, and the gap is recorded in [What the executor gets wrong](#what-the-executor-gets-wrong).
No executor code was changed to build this corpus; that is the point of it. The one gap the
corpus found — a merge closed after its first traversal — was fixed afterwards against the
derivation, not the other way round.

## What the library fixes, and what a trace adds

The library states a **partial order** between performances. A succession is a connector typed
by `HappensBefore`, which "asserts that the earlierOccurrence is completely separated in time …
with the earlierOccurrence happening completely before the laterOccurrence"
(`Occurrences.kerml`, `assoc all HappensBefore`). The steps of a behavior are its
`enclosedPerformances`, happening during it (`Performances.kerml`, `Performance::enclosedPerformances`;
`StatePerformances.kerml`, "all steps are implicitly considered to be enclosedPerformances, and
hence happening during the state performance"). The default `1..1` multiplicity for a feature
(KerML 1.0 §7.4.5) is the rule for feature values, not an inherited count for an action node:
`Actions.sysml` declares `Action.subactions` as `Action[0..*]`, and each action-node usage's own
declared multiplicity determines how many performances it names, whether the usage is reached by a
succession or starts concurrently as an unordered subaction. An omitted multiplicity remains one
performance; a fixed finite `[n]` or `[n..n]` names `n` performances, including none for `[0]`. A
non-fixed or unevaluable count cannot be run by this fixed-count executor. Nothing in the library
orders two steps no chain of `HappensBefore` links connects.

A `.trace.golden` records one **linearization** of that partial order plus tool-defined
scheduling detail the library says nothing about:

- Tokens are stepped in descending index order within a step; a fork appends its branch tokens in
  succession-declaration order, so the branch declared *last* is stepped *first* in every step.
- A node's body runs in the step that moves its token on, so a body's statements appear in the
  golden between the `step N:` line that shows the token at the node and the `step N+1:` line.
- A node several successions reach (a join, or a plain node) performs with a fresh token appended
  to the token list once every succession has delivered, and the step that fired it may go on to
  step that token (and tokens the removal shifted) again; a `step N:` line is therefore the
  executor's step boundary, not a unit the library defines.
- Under the `explore` policy a step is one token advancing one node, so a `choice` line is one
  pick among the tokens able to act at that moment and the next step picks again among those left
  and any the move enabled; a linearization is the sequence of those picks, and a branch of
  several nodes may be overtaken by a concurrent one between any two of them. The fixed policies
  move every steppable token once per step, so their `step N:` lines group moves that `explore`
  spreads over consecutive steps; a body's statements run without interruption under both.
- A state machine records a transition's guard evaluation, exit, effect and entry as they run, and
  evaluates a guard once to select the transition and once again to fire it; the second
  evaluation is a tool detail with no observable effect, since a guard is an expression.
- A `choice` line marks a point where the executor picked among alternatives the library leaves
  unordered — several steppable tokens in one step, several holding decision guards, several
  enabled transitions out of one state for one event, or two tokens writing one feature in one
  step — and names the alternative it took. Everything after a `choice` line is one linearization
  among the ones that line admits.

When a golden is reviewed against this record, the question is whether the golden's
linearization is *one of* the linearizations the derivation admits and whether every outcome the
derivation fixes is met — not whether the golden is the only correct trace.

## Library text relied on

| Where | Text | Used for |
|-------|------|----------|
| `Occurrences.kerml` `HappensBefore` | "the earlierOccurrence happening completely before the laterOccurrence … no snapshot of the earlierOccurrence happens at the same time as any snapshot of the laterOccurrence" | Every succession orders the whole source performance (its body included) before the whole target performance |
| `Performances.kerml` `Performance::enclosedPerformances`, `subperformances` | `step enclosedPerformances: Performance[0..*] subsets performances, timeEnclosedOccurrences` — "timeEnclosedOccurrences of this Performance that are also Performances"; `composite step subperformances: Performance[0..*] subsets enclosedPerformances, suboccurrences` — "enclosedPerformances that are composite" | A composite step's performances start no earlier and end no later than the performance owning them, whether or not a succession orders them |
| `Occurrences.kerml` `Occurrence::timeEnclosedOccurrences` | "Occurrences that start no earlier than and end no later than this occurrence" | The owner's performance ends only after every subperformance has; its own successors follow them all |
| `Actions.sysml` `Action::subactions` | `action subactions: Action[0..*] :> actions, subperformances` — "The subperformances of this Action that are Actions" | Every composite action usage of an action (a `send`, `accept`, `assign`, `if`, `while` or `for` among them) is one of its subperformances; a `ref` action usage is not composite and is not one |
| KerML 1.0 §7.3.2, §8.3.3.1.10 `Type::inheritedMembership` | `derived var feature inheritedMembership : Membership[0..*] ordered subsets membership` | A type's effective memberships include its owned memberships and those inherited from its generals; action steps, successions, flows and statements therefore carry into a specialization |
| KerML 1.0 §7.3.4.3, §8.3.3.3 `FeatureTyping` | Feature typing is a specialization | A body-stating typed usage inherits the action content of its type; its own redefinitions mask inherited members |
| SysML v2 §7.17 `PerformActionUsage` | A perform usage reference-subsets the action it performs | A body-stating perform usage merges the referenced action's members into its one lowered flow |
| `Systems Library/States.sysml` `StateAction` | `entry action entryAction :>> 'entry'; do action doAction : Action :>> 'do'; exit action exitAction : Action :>> 'exit'` | Entry, do and exit action usages are behavior members whose body-stating typed usages specialize the action they perform |
| KerML 1.0 §8.3.3.1 `Type::multiplicity` | "If there is no such ownedMember, then the cardinality of this Type is constrained by all the Multiplicity constraints applicable to any direct supertypes" | A step that declares no multiplicity takes the multiplicity of its redefined feature; subsetting alone does not transfer that multiplicity |
| `Actions.sysml` `ControlAction` | `bind start = done` — "A ControlAction is instantaneous" | A control node adds no duration; its successor may start as soon as its predecessors end |
| `Actions.sysml` `ForkAction` | "Fork behavior results from requiring that the target multiplicity of all outgoing succession connectors be 1..1" | Each fork performance is followed by exactly one performance of every target |
| `Actions.sysml` `JoinAction` | "Join behavior results from requiring that the source multiplicity of all incoming succession connectors be 1..1" | Each join performance follows exactly one performance of every source, one per incoming succession |
| `Actions.sysml` `MergeAction`, `ControlPerformances.kerml` `MergePerformance` | "Incoming succession connectors to a MergeAction must have source multiplicity 0..1"; "For each instance of MergePerformance, the incomingHBLink is an instance of exactly one of the Successions, ordering the MergePerformance as happening after an instance of the source of that Succession" | A merge performance follows one source performance; a source a given merge performance was not reached from need not exist |
| `Actions.sysml` `DecisionAction`, `ControlPerformances.kerml` `DecisionPerformance` | "For each instance of DecisionPerformance, the outgoingHBLink is an instance of exactly one of the Successions, ordering the DecisionPerformance as happening before an instance of the target of that Succession" | Each decision performance is followed by a performance of the one target whose guard held |
| `Stochastic.sysml` `Probability` | "A seeded run draws a branch by these weights, an unseeded run takes the most probable, and a weighted branch whose guard does not hold is left out of the draw, the others' weights renormalized." | These weights belong to model draws; they do not assign probabilities to unresolved scheduling choices |
| KerML 1.0 §7.4.5 | A feature with no declared multiplicity holds exactly one value | This constrains feature values, not action-node usage counts; a plain step with no multiplicity is one performance per performance of its owner, however many successions reach it |
| `Performances.kerml` `Performance::enclosedPerformances`; `Actions.sysml` `Action.subactions` | `subactions : Action[0..*]`; `subperformances` | A node usage's declared multiplicity counts its enclosed performances, including unordered concurrent starts; each repeated performance owns a fresh node frame while writing the shared owner features |
| OMG issue [KERML-29](https://issues.omg.org/issues/KERML-29) | Deferred; the multiplicity of succession ends is unresolved | The execution checker approximates an unwritten end as either unconstrained `[0..*]` or exact-one `[1..1]` and accepts only when both readings force and admit the endpoint counts |
| `StatePerformances.kerml` `StatePerformance` | `succession [1] entry then [*] middle; succession [*] middle then [1] exit` | Entry first, exit last, within a state performance |
| `StatePerformances.kerml` `StateTransitionPerformance` | `succession all [*] acceptable then [*] guard; succession [*] guard then [1] transitionLinkSource.exit` | The guard is evaluated after the trigger and before the source state's exit |
| `TransitionPerformances.kerml` `TransitionPerformance` | `binding transitionLink.earlierOccurrence = transitionLinkSource; succession [1] transitionLinkSource then [*] effect; succession [*] effect then [1] transitionLink.laterOccurrence; succession all [*] guard then [*] effect` | The effect runs after the source state performance has ended (its exit included) and before the target state performance starts (its entry included) |
| `Transfers.kerml` `SendPerformance` | `feature sentTransfer: MessageTransfer [1] subsets sender.outgoingTransfersFromSelf`; `succession self then sentTransfer` | A send is followed by exactly one transfer carrying its payload |
| `Transfers.kerml` `AcceptPerformance` | `feature acceptedTransfer: MessageTransfer[1] subsets receiver.incomingTransfersToSelf`; `succession acceptedTransfer then self.endShot`; `binding payload = acceptedTransfer.payload` | An accept ends after exactly one transfer to its receiver and yields that transfer's payload; nothing pairs a particular accept with a particular transfer |
| `Actions.sysml` `DecisionTransitionAction`, `TransitionPerformances.kerml` `NonStateTransitionPerformance`, `TPCGuardConstraint` | "the base type of TransitionUsages used as conditional successions in action models"; `in feature transitionLinkSource: Performance[1]`, `feature transitionLink: HappensBefore[0..1]`, `succession [1] transitionLinkSource then [1] Performance::self`, `connector all guardConstraint: TPCGuardConstraint[*] from [0..1] transitionLink to [*] guard` (`constrainedHBLink` / `constrainedGuard`, `inv { allTrue(constrainedGuard()) }`) | A guarded succession in an action is a transition performance that happens after its complete source performance — a merge's body included — and whose guard constrains the `HappensBefore` link to the successor: a false guard leaves the link out, not the source performance |

The SysML v2 specification's own control-node example (`ChargeBattery`, §7.17.3, reproduced in
the training corpus as `17. Control/Decision Example.sysml`) and the pilot validation corpus
(`3a-Function-based Behavior-1.sysml`: "A merge node is necessary to prevent a loop of
successions from being unsatisfiable"; "The performance of the actions on the left cannot
continue once there is a performance of 'engineStopped'") are used only to confirm that the
readings above are the ones the specification's authors rely on, not as library text.

## Cases

Each case names its fixture (the `.sysml` and `.expected.json`, plus a `.trace.golden` where the
executor meets the derivation), the constraints derived, the orderings left open, and the
outcome the derivation fixes. An openness that is observable is encoded in the fixture: the
`.expected.json` lists the admissible set as `outcomes` and cites the section below as
`admissible`, and a `.trace.order` file states the partial order the library does fix as
`earlier < later` constraints on the trace. The harness checks a run against exactly one member,
then explores the case (`exploreConformanceCase`, budget `exploreBudget`, default 1024 runs and 64
choice points) and fails on a listed outcome no linearization reaches, on a reached outcome the
list omits, and on a budget hit. An openness that is not observable is pinned by a fixture with one
expected outcome; such a fixture has no `outcomes` and the harness does not explore it, so the
`explore` figures quoted for it below come from running the fixture under the `explore` policy,
not from the suite.

### Exploration separates scheduler choices from model weights

Fixture: `action_explore_mixed_scheduler_weighted_probability` (conformance case).

```
start → fork ─┬─ a: x := 1 ─┐
              └─ b: x := 2 ─┴─ join → decide
                                      x == 1 → weighted draw: y := 1 (0.3) | y := 2 (0.7)
                                      x == 2 → y := 0
```

Derived admissible outcomes:

- `ForkAction` requires one performance of each outgoing succession target, but does not order
  the branches. The writes race, so the scheduler may leave `x = 1` or `x = 2` at the join.
- `DecisionAction` / `DecisionPerformance` requires exactly one outgoing succession for each
  decision performance. The `x == 1` route reaches the weighted decision; its two weights sum
  to one, so the model gives `y = 1` probability `0.3` and `y = 2` probability `0.7`. The
  `x == 2` route instead completes with `y = 0`.
- Scheduler choices have no probability. Over schedulers, `{x=1,y=1}` therefore has range
  `[0, 0.3]`, `{x=1,y=2}` has `[0, 0.7]`, and `{x=2,y=0}` has `[0, 1]`: each minimum is zero
  because a scheduler can choose the other write last, while each maximum is the model's
  weighted probability when the `x = 1` route is selected or certainty when `x = 2` is selected.
  This is the min/max at scheduling nodes and weighted sum at weighted nodes, not a uniform
  distribution over linearizations.

The conformance schema has no error member in `outcomes`; runtime-error outcomes are not
expressible by this expectation format and are rejected as unexpected.

### A join follows one performance of every source, however long each branch takes

Fixture: `action_join_waits_for_slowest_branch` (golden).

```
start → split ⇉ a ──────────────┐
              ⇉ b1 → b2 ────────┤→ sync → after → done
              ⇉ c1 → c2 → c3 ───┘
```

Derived constraints:

- `split` is followed by exactly one performance each of `a`, `b1`, `c1` (ForkAction, target
  multiplicity 1..1).
- `sync` follows exactly one performance each of `a`, `b2`, `c3` (JoinAction, source multiplicity
  1..1), and `b2` follows `b1`, `c3` follows `c2` follows `c1` (HappensBefore).
- `after` follows `sync` (HappensBefore), so it runs after all three `arrived := arrived + 1`
  writes have ended.

Open: the relative order of `a`, `b2` and `c3` — and of every node on one branch against every
node on another. The golden's order (the `c` branch stepped first within each step, `a` finishing
first because its branch is shortest) is one admissible linearization. Nothing observable depends
on it: every interleaving increments `arrived` three times before `after` reads it, so the fixture
pins the one outcome and has no `outcomes`. Under `explore` the interleavings of one, two and
three nodes on three branches are `6! / (1! 2! 3!) = 60` linearizations reaching that one outcome
(60 runs, 1 outcome, complete), each a sequence of token choices (`step 3: 2@a first of 2@a, 3@b1,
4@c1; step 4: 3@b1 first of 3@b1, 4@c1; …`).

Fixed outcome: `arrived = 3`, `seen = 3`. The executor agrees; the golden shows `after` reading
`arrived -> 3` and every branch's write preceding it.

### A join counts one token per incoming succession, not the tokens parked at it

Fixture: `action_join_one_token_per_incoming_succession` (golden).

```
start → split ⇉ l1 ──┐
              ⇉ l2 ──┤→ left ────────────────┐
              ⇉ r1 → r2 → r3 → right ────────┤→ sync → after → done
```

Derived constraints:

- `left` is one performance (KerML §7.4.5), and it follows both `l1` and `l2` (HappensBefore over
  each succession into it).
- `sync` follows exactly one performance of `left` and exactly one of `right` (JoinAction, source
  multiplicity 1..1 on each of its two incoming successions).
- `right` writes `log := log * 10 + 1`; `after` follows `sync` and writes `log := log * 10 + 2`.

Open: the order of `left` against any node of the `r` branch. Nothing observable depends on it,
so the fixture pins the one outcome without `outcomes`; under `explore` the seventy
interleavings — `l1` and `l2` in either order then `left`, three events woven into the four of
the `r` branch, `2 × C(7, 3)` — all reach it (70 runs, 1 outcome, complete).

Fixed outcome: `log = 12` — `right` writes before `after`, whatever the interleaving, because
`after` cannot start before `sync`, and `sync` cannot start before `right` has ended.

Executor: `log = 12`, no deadlock. Every token records the succession it travelled
(`Token.Via`, the lowered `ActionEdge`), and `ActionExecutor.synchronize` holds a token at a
node until one token has arrived over *each* succession into it; the golden shows the two
`left` arrivals collapse into one token at `sync` (step 4), which waits there for `right`
(step 6) before `after` runs (step 7). The case is observable only because a node upstream is reached
over two successions, so it shares a mechanism with the next case; it stays a separate case
because it pins a distinct rule — a join fires on *which* successions delivered, not on how
many tokens arrived. `action_join_same_succession_twice` pins the converse: two tokens over
one succession into a join satisfy that succession once, and the second waits for the join's
next firing (`log = 1212`).

### A node without multiplicity reached over two successions is performed once, after both

Fixture: `action_node_with_two_incoming_successions_runs_once` (golden).

```
start → split ⇉ l1 ──┐
              ⇉ l2 ──┤→ both → done
```

Derived constraints:

- `both` declares no multiplicity, so it is one performance of `converge`; this example says
  nothing about a step usage that explicitly declares a count.
- Each of `first l1 then both` and `first l2 then both` is a `HappensBefore` link whose
  `laterOccurrence` is that one performance, so it starts after both `l1` and `l2` have ended.
  This is the reading the pilot corpus states in prose for `engineStopped`, which five
  successions reach.

Open: the order of `l1` against `l2`. It is not observable — `both` performs once either way —
so the fixture pins the one outcome without `outcomes`; under `explore` the two orders both reach
it (2 runs, 1 outcome, complete).

Fixed outcome: `hits = 1`.

Executor: `hits = 1`. The same `synchronize` gate holds a token at a plain node until each
succession into it has delivered, so `both` is performed once by the one token the two
arrivals collapse into (golden: both arrivals held at `both` after step 3, its one
performance in step 4). A join differs only in what it awaits: every incoming
succession (source multiplicity 1..1), so a join one of whose sources no token can reach
deadlocks (`robustness_test.go:deadlock_join_starvation`, `:deadlock_join_same_succession_twice`);
a plain node awaits a succession only once it has delivered or while some token of the
activation, other than one held at the node, can still reach its source without passing through
the node — the branch a decision did not take, or a loop back over the node itself, imposes no
`HappensBefore` on this performance. `action_node_converges_after_decision` (golden) pins the
first half: `converge` behind a decision's two branches performs once for the branch taken and
does not deadlock on the other; `action_node_loop_back_reperforms` (golden) the second: `bump`,
reached from `start` and from the decision after it, performs once per pass.
`action_nested_node_two_successions_per_performance` pins that the count is per performance of
the owning action: two performances of an action holding such a node each perform it once; and
`action_node_concurrent_performances` and `action_node_concurrent_nested_bindings` (goldens)
that a flow-owning node reached from both branches of a fork is likewise one performance,
holding at each pin the one delivery the flow into that pin carried.

### A specialized action performs its inherited steps; a redefining step keeps the redefined step's members

Fixtures: `inherited_action_steps_p0`, `_p2`, `_p3`, `_usage_typed`,
`_usage_typed_with_body`, `_multi_level`, `_diamond`, `_narrow`, `_keep`,
`_redefine_two_levels`, `_direct_assign`, `_direct_send`, `_typed_node`, `_perform`,
`_state_entry`, `_state_do`, `_state_exit`, `_redefined_typed_step`; the ordering-sensitive
cases carry trace goldens. The body-stating cases are `_usage_typed_with_body`,
`_nested_typed_usage_with_body`, `_state_entry_typed_body`, `_state_do_typed_body`,
`_state_exit_typed_body`, `_state_entry_perform_body` and
`_classifier_perform_typed_body`; state and classifier observations use explicit `inout` pins
bound to `hits`.

Derived constraints:

1. KerML 1.0 §7.3.2 and §8.3.3.1.10 `Type::inheritedMembership` derives a type's inherited
   memberships from its generals. Redefinition is a Subsetting that is a Specialization
   (§7.3.4.5), and FeatureTyping is also a Specialization, so a specialized action definition
   inherits action nodes, statements, successions and flows, and performing a typed action usage
   performs the action definition's body.
2. The pilot's `TypeAdapter.getInheritedMemberships` passes inherited memberships through
   `removeRedefinedFeatures`: a member disappears only when an owned or nearer inherited feature
   redefines it, directly or indirectly. An inherited succession therefore maps its endpoint to
   the redefining node, and inherited start/done markers are the specialized flow's own markers.
3. For `Keep::a`, its own `assign c := c + 10` does not redefine `Base::a`'s unnamed assignment
   `assign c := c + 1`. The pilot's `FeatureAdapter.addRedefinitions` /
   `FeatureAdapter.getRelevantFeatures` add implicit redefinitions for end features,
   constructor-result features and parameters by position; `ActionUsageAdapter.getRelevantFeatures`
   does so for state- and transition-action members, not arbitrary assignments. Both statements
   remain members of `Keep::a`, and each performance applies both. Keep owns no multiplicity, so
   the redefined `Base::a[3]` governs it: `3 × (1 + 10) = 33`. The two additions are unordered
   within each leaf performance, but their order does not change the result.
4. `Narrow::a[2]` owns its count, so it performs twice and inherits Base's `+1` body:
   `2 × 1 = 2`. In the two-level case, `Mid::a[2]` governs `Leaf::a`, whose own `+10` statement
   and inherited Base `+1` statement run twice: `2 × (10 + 1) = 22`.
5. Base's `first start then a` and `then done` constrain the specialized graph too. They target
   the replacement for `a`, never a duplicate inherited node. Nearest-first generalization makes
   each inherited body run once through a diamond.
6. A feature's type includes the types of features it redefines, so `Narrow2::a` inherits `Foo`
   as its performed action. Its two performances update their shared `hits` feature twice; the
   repeated node's frame is not reported as an output.

The fixed outcomes are P0 `c = 1`, P2 `c = 1, z = 0`, P3 `c = 101`, typed usage `u1.c = 1`,
typed usage with its own `b` body `c = 101`, multi-level `c = 11`, diamond `c = 111`,
Narrow `c = 2`, Keep `c = 33`, two-level redefinition `c = 22`, direct assignment `c = 5`,
direct send `number = 5`, typed node `x.c = 1`, `perform` node `p.c = 1`, and one inherited
step each for state entry, do and exit. Body-stating typed state entry/do/exit each add the
inherited step and own member once (`hits = 11`); the `entry perform` form has the same outcome,
the empty-`Bump` entry body leaves `c = 1`, and the part-level typed classifier body leaves
`hits = 101`. `Narrow2` performs `a` twice as `Foo` and reports only the outer `hits = 2`.
Subsetting alone does not carry multiplicity: only the own-or-
redefined source used by `GoverningMultiplicityOf` does.

Known refusals are cyclic specialization (`action specializes itself`), a missing redefined action
step (`redefined action step not found`), a succession reaching a non-action replacement
(`redefining feature is not an action step`), and an inherited succession mapping to multiple
replacements (`inherited succession reaches more than one redefining step`).

### Body-stating typed usages and mixed state behaviors

7. KerML 1.0 §7.3.4.3 / §8.3.3.3 makes FeatureTyping a Specialization, so a typed usage inherits
   its type's memberships as well as stating its own. The pilot's `TypeAdapter.getInheritedMemberships`
   and `removeRedefinedFeatures` preserve the typed action's steps, successions, flows and statements,
   dropping only inherited members its own body redefines. Thus `action y : B1 { action b { … } }`
   performs B1's ordered `a` and its own unordered `b` in one flow. This applies to top-level typed
   usages and nested action nodes. A nested typed node with a real body is lowered with typing
   included and marked in the graph so the runtime does not also invoke that type; a node whose body
   only binds features or pins retains its invocation path. The nested and top-level fixtures each
   leave `c = 101`, not `102`.
8. `Systems Library/States.sysml` `StateAction` declares the entry, do and exit behavior features.
   The pilot's `ActionUsageAdapter.getRelevantFeatures` and `getRedefinedFeature` identify those
   state behavior usages as redefinitions of `entryAction`, `doAction` and `exitAction`. A body-stating
   typed entry/do/exit usage therefore specializes the performed action's body: its own members and
   inherited ordered flow run once as a single started flow. SysML v2 §7.17 `PerformActionUsage`
   gives the `perform action p : Bump { … }` form the same result because its ReferenceSubsetting
   is a Subsetting, hence a Specialization. The same rule applies to a part-level classifier
   `perform` body. The inherited action's step and the own action body each add once in the state
   fixtures; the chosen trace order is a linearization, while the relative order of unordered
   members is not fixed by the library.
   In `action_invoked_node_body_writes_output`, the usage's own `assign y := y + 1` is an
   unordered subaction beside Scale's inherited `start → scaling → done` flow. The inherited
   `scaling` step sets `y` to 30; the assignment leaves 31 if it runs after `scaling`, while
   `scaling` leaves 30 if the assignment runs first. The following `check` reads that final value,
   so the oracle admits exactly `{30, 31}`.
   A typed node whose own statements read the callee's outputs is order-dependent under this merge
   rule: an order before the producing step reads the unset output and fails. `state_block_flow_typed_node`
   therefore reads `scaled.y` after the node. `action_invoked_node_body_writes_output` and
   `state_block_flow_typed_node_body_writes_output` carry the `{30, 31}` sets.

### Repeated action steps and shared writes

Fixtures: `action_step_multiplicity_exact`, `_reverse`, `_explore`, `_range`, `_named_bound`,
`_zero`, `_local_frames`, `_nested`, `_nested_state_entry`, `_perform`, `_ordering` and
`_order_target_only`, `_unordered_start`, `_unordered_start_reverse`,
`_unordered_beside_ordered`, `_unordered_zero`,
`_unordered_nested`, `_unordered_unbounded`, `_unordered_unaddressable`, `_unordered_outgoing`,
`_unordered_loop_body` and `state_step_multiplicity_unordered_do_body`;
`action_step_multiplicity_shared_writers` states the open outcome set.

Derived constraints:

- A missing own multiplicity inherits the first declared multiplicity of a redefined feature;
  where neither declares one, the action-node usage is one performance. An own or inherited exact
  finite count `[n]`, `[n..n]`, or named exact bound that evaluates to `n` performs `n` times,
  including unordered subactions that start concurrently without an incoming succession; `[0]`
  performs no body, trace event, flow or data transfer. Other ranges and bounds the model cannot
  evaluate do not identify a fixed number and are refused.
- An action usage in a loop or conditional block flow is performed once per pass. Repetition in
  those statement-engine flows is out of scope: exact counts other than `[1]`, including `[0]`,
  are refused with `action-step-multiplicity-unsupported` rather than being expanded.
- `Occurrences.kerml` `HappensBefore` orders whole source performances before whole target
  performances. A repeated node therefore needs every incident edge to admit and force its
  complete count. The accepted fixtures use explicitly written end multiplicities: `[1] p` to
  `[*] a[3]`, a written target `[3] a[3]`, `[*] a[3]` to `[1] q`, and `[2] a[2]` to `[3] b[3]`.
  A start source and done target constrain no repeated endpoint; guards, control-node adjacency,
  pins of repeated nodes and external reads of their features exceed the supported subset.
- KerML leaves succession-end defaults unresolved ([OMG KERML-29](https://issues.omg.org/issues/KERML-29),
  deferred). The checker evaluates unwritten ends both as unconstrained `[0..*]` and as `[1..1]`,
  accepting only an edge whose counts are forced and admitted under each reading. If a reading
  forces an end that excludes its endpoint count, the result is `action-step-order-unsatisfiable`;
  otherwise an edge not established by both readings is `action-step-order-open`. This is
  deliberately approximate by refusal, not a new claim about KerML defaults.
- A plain `then` (no written end multiplicities) between a repeated step and any step other than
  the action's `start`/`done` is refused with `action-step-order-unsatisfiable`: under the
  `[1..1]` reading the end excludes the step's count, and under the unconstrained reading the order
  is left open. Write the ends explicitly (`succession first [1] p then [*] a;`, `then [3] a;`,
  `succession first [*] a then [1] q;`) to state the library's fan-out/fan-in pattern.
- In `action_step_multiplicity_shared_writers`, each of three `a` frames has its own `l`, which
  snapshots the shared `c` when that frame begins; its `w` writes `l + 1` back to that same `c`.
  If all three snapshots happen before any write, all writes store `1`; a snapshot after one or two
  completed writes can lead to a final value of `2` or `3`. There are at most three writes, so the
  hand-derived final set is exactly `{c = 1, c = 2, c = 3}`.

Open: the order among sibling repeated performances is not established by their count. The
runtime represents them as sibling tokens, each with an independent performance frame; their
owner-frame writes share the same feature space. The last completion is a barrier: only after
every performance at that node finishes can the token carry its succession and data flows on.
Exploration therefore finds each admitted shared-write result without merging states that differ
in the live repetition set.

Fixed outcome: `action_step_multiplicity_exact` has `c = 3` under declared, reverse and explore
schedules; explore reaches one distinct outcome. In `action_step_multiplicity_zero`, `[0]` leaves
the initial `c` unchanged and its `q` successor still runs, setting `c = 7`. The local-frame
fixture reaches `c = 3`: each fresh `l` starts at zero, becomes one, and contributes one to the
shared `c`.

### Concurrent branches writing one feature: the value is open, the writes are not

Fixture: `action_fork_branches_write_one_feature` (golden).

```
start → split ⇉ left  { x := 1; leftRan := true  } ─┐
              ⇉ right { x := 2; rightRan := true } ─┤→ sync → done
```

Derived constraints:

- `left` and `right` are each performed exactly once (ForkAction), so `leftRan` and `rightRan`
  both end `true`.
- `sync` follows both (JoinAction), so both writes of `x` have ended before the action ends and
  `x` is `1` or `2`, never `0`.

Open: the order of `left` against `right`, and so which write of `x` stands. The library gives
no `HappensBefore` link between them and no conflict rule; either final value is admissible.

Pinned outcome: the admissible set `{x = 1, x = 2}`, with `leftRan = true` and `rightRan = true`
in both, which the case states as `outcomes` citing this section and the harness checks the run
against — a run must match exactly one member. The partial order the library does fix is stated
as `.trace.order` constraints (`split < left`, `split < right`, `left < sync`, `right < sync`) the
trace must satisfy. Exploration reaches both outcomes and no other, one linearization each
(2 runs, 2 outcomes, complete). The exact trace golden stays: it records the executor's scheduling (`right`
is the branch declared last, so its token is stepped first and `left` writes last, giving
`x = 1`) and exists only so a change in that scheduling is noticed. The executor reports the
conflict as a choice point (`choice step 3: writes x := 1 by token 2, x := 2 by token 3
(unordered; x := 1 by token 2 stood)`), so the trace and the diagnostics name the value that
stood as a tool decision rather than passing it off as the model's answer. Companion fixtures:
`action_choice_same_step_write_conflict` (golden), the same shape over a `String` feature;
`action_choice_chained_write_conflict` (golden), the same shape writing one object's feature
through a feature chain (`s.reading`) from both branches; and
`action_choice_performer_write_conflict` (golden), a performed action's branches both writing
the part performing it. A destination is the object written and the feature written, whatever
name reached it, so two chains reaching one object are one conflict
(`TestWriteConflictOnOneObjectThroughTwoChains`), a feature and one redefining it are one
destination under either name (`TestAliasWritesAreOneDestination`), and two writes of equal
value are still one: the library orders the writes no more when they agree. The choice is
recorded once the step is complete, one per destination, listing the last write of every token
that wrote it and the one that stood: three branches are one choice of three
(`TestThreeWritersAreOneChoice`), and a token writing twice contributes only its last write, the
one that would stand had it gone last (`TestRepeatedWritesByOneTokenListItsLast`).

### A decision with several holding guards: exactly one branch follows, which one is open

Fixture: `action_choice_decision_overlapping_guards` (golden).

```
start → select ─ if level > 50 → warn  { handler := 1 } → done
               ─ if level > 70 → alarm { handler := 2 } → done
```

Derived constraints:

- `select` is followed by a performance of exactly one target (DecisionPerformance: "the
  outgoingHBLink is an instance of exactly one of the Successions"), so `handler` ends `1` or
  `2`, never `0`.
- Each guard is a `TPCGuardConstraint` on its own link; a false guard leaves that link out. With
  `level = 75` both guards are true, so neither link is excluded by its guard.

Open: which of the two admissible links the decision performance takes. The library says only
that it is exactly one of them; nothing ranks `warn` against `alarm`.

Pinned outcome: the admissible set `{handler = 1, handler = 2}`, stated as `outcomes` citing this
section; exploration reaches each once (2 runs, 2 outcomes, complete). The executor evaluates
every guard, takes the first declared, and records the choice
(`choice step 2: decision select branches 1->warn, 2->alarm hold (unordered; took 1->warn)`);
the golden pins that linearization. Reporting never changes the run: the guards after the first
holding one are read in a preview that is undone — what evaluating them costs, writes or starts
is restored, and nothing they do is traced — so the run spends and does what first-match did
(`TestLaterGuardIsProbedWithoutCost`). A guard read that way that cannot be evaluated is not an
alternative and not an error: the library's `TPCGuardConstraint` is `inv { allTrue(constrainedGuard()) }`,
an expression with no result is not true, so the link it guards is simply not selected, and the
library defines no failure for it. The executor records the guard as an informational
`guard-unevaluable` note (`unevaluable guard step 2: decision select branch 2->alarm: division by
zero (not selected)`) so the tool's reading is visible without changing the run
(`TestLaterGuardErrorIsNotAChoiceNorAFailure`; fixture `action_choice_unevaluable_guard`, golden).
The first guard read is the run's own, not a preview, and its failure fails the run as it always
has. A decision whose guards are all false remains an execution error
(`TestRuntimeRobustness/decision_all_guards_false`), as the library then admits no outgoing link.

### Three concurrent writers of one feature: six orders, three values

Fixture: `action_explore_three_writers` (golden, explored).

```
start → split ⇉ a { x := 1; aRan := true } ─┐
              ⇉ b { x := 2; bRan := true } ─┤→ sync → done
              ⇉ c { x := 3; cRan := true } ─┘
```

Derived constraints:

- `a`, `b` and `c` are each performed exactly once (ForkAction), so `aRan`, `bRan` and `cRan`
  all end `true`.
- `sync` follows all three (JoinAction), so every write of `x` has ended before the action ends
  and `x` is `1`, `2` or `3`, never `0`.

Open: the order of the three branches. The library gives no `HappensBefore` link among them, so
every one of the `3! = 6` orders is a valid linearization. The value of `x` is the last write, and
each branch is last in exactly two of the six orders, so the six linearizations reach exactly
three outcomes, `{x = 1, x = 2, x = 3}`, two linearizations each.

Pinned outcome: that admissible set, with the three `…Ran` flags `true` in every member, stated
as `outcomes` citing this section; `.trace.order` states the partial order the library does fix
(`split < a`, `split < b`, `split < c`, `a < sync`, `b < sync`, `c < sync`). The exact golden
records the default schedule (`c` is declared last, so its token is stepped first and `a` writes
last, giving `x = 1`). Exploration is what makes the set checkable: `explore` replays the run
along every choice sequence and must reach each of the three outcomes and no other, in six runs.
The first pick among three tokens and the next among the two left are two choice points in
consecutive steps, so a linearization is a sequence of two choices, not one choice among six.

### Subactions no succession orders: each is performed during the owner, in which order is open

Fixtures: `action_unordered_subactions_write_conflict` (golden, explored), with
`action_unordered_subactions`, `action_unordered_beside_first`, `action_unordered_nested_subactions`,
`action_unordered_statements`, `action_unordered_send_accept`, `action_unordered_reference_not_performed`,
`action_unordered_accept_holds_owner`, `action_unordered_send_to_receiver`,
`action_body_flow_unordered_statement` and `state_do_body_unordered_statement` (each one outcome).

```
race { a { c := 1 }   b { c := 2 } }        -- no succession, no `first`
```

Derived constraints:

- `a` and `b` are composite action usages of `race`, so each is one of its `subactions`
  (`Actions.sysml`), hence a `subperformance` and an `enclosedPerformance` of it
  (`Performances.kerml`): each is performed exactly once per performance of `race` (KerML 1.0
  §7.4.5), starting no earlier and ending no later than it (`timeEnclosedOccurrences`).
- `race` therefore ends only after both have; a succession out of `race`, its `done` and the
  reading of its outputs come after both writes. A `first`-rooted flow beside them
  (`action_unordered_beside_first`) is one more part of the same performance, and the owner ends
  after it and after them.
- A statement written among the action's members (`send`, `accept`, `assign`, `if`, `while`,
  `for`) is an action usage like `a` and is performed the same way; an accept among them holds
  the owner open until its transfer arrives (`AcceptPerformance`), and with none to arrive the
  owner never ends (`action_unordered_accept_holds_owner`, an accept deadlock).
- A `ref` action usage is referential, not composite, so it is no `subperformance` and is not
  performed (`action_unordered_reference_not_performed`); so is a `perform`, an event occurrence
  usage, which is referential (`validateEventOccurrenceUsageIsReference`), and an abstract usage.
- The same holds for the flow a nested action node or a state's entry, do or exit body states
  (`action_unordered_nested_subactions`, `state_do_body_unordered_statement`).

Open: the order of `a` against `b`. No `HappensBefore` links them, so both linearizations are
valid; with `c := c + 1` and `c := c + 10` they agree on `c = 11` (`action_unordered_subactions`),
with `c := 1` and `c := 2` the write that stands is open.

Pinned outcome: the admissible set `{c = 1, c = 2}`, stated as `outcomes` citing this section;
exploration reaches both and no other, one linearization each. The executor performs each such
subaction as a token started with the owner's performance (`ActionGraph.Concurrent`, lowered by
`StartFlow`), so its interleavings are the same choice points fork branches are. The exact golden
records the default schedule.

Not covered: a nested action node whose members are only statements, and a loop, branch or
behavior body stating no flow, run their statements and nodes in declaration order, as they did
before; the library orders them no more than it orders `a` and `b`, so that order is the
executor's choice, recorded in [spec compliance](spec-compliance.md).

### A write between two nodes of a concurrent branch: three orders, three outcomes

Fixture: `action_explore_write_between_branch_nodes` (golden, explored).

```
start → split ⇉ left1 { x := 1 } → left2 { y := x } ─┐
              ⇉ right { x := 2 } ────────────────────┤→ sync → done
```

Derived constraints:

- `left1`, `left2` and `right` are each performed exactly once (ForkAction, and `left2` follows
  `left1` by HappensBefore), and `sync` follows `left2` and `right` (JoinAction), so every write
  has ended before the action ends.
- `left2` reads `x` after `left1`'s write has ended, so `y` is `1` or `2`, never `0`.

Open: the order of `right` against `left1` and against `left2`. The library gives no `HappensBefore`
link from either to `right`, so `right` may end before `left1` starts, start after `left1` ends and
end before `left2` starts, or start after `left2` ends: three linearizations, no more, since
`left2` cannot precede `left1`. Each leaves a different pair of values: `right, left1, left2`
gives `x = 1, y = 1`; `left1, right, left2` gives `x = 2, y = 2`; `left1, left2, right` gives
`x = 2, y = 1`.

Pinned outcome: that admissible set of three, stated as `outcomes` citing this section;
`.trace.order` states the partial order the library does fix (`split < left1`, `left1 < left2`,
`split < right`, `left2 < sync`, `right < sync`). The exact golden records the default schedule
(`right` is declared last, so its token is stepped first: `right, left1, left2`, giving `x = 1,
y = 1`). Exploration must reach each of the three outcomes and no other, in three runs (3 runs,
3 outcomes, complete). The case is what distinguishes exploring one move at a time from exploring
the order of one lockstep step: had every steppable token moved once per step, `left1` and `right`
would both have moved in the step after the fork whichever went first, `left2` could never have
run before `right`, and the exploration would have reported two outcomes complete, missing
`x = 2, y = 1`. No fixed policy takes it: each moves `left1` and `right` in the step after the fork,
in one order or the other, before `left2` can run, so `declared` (and `seed:1`) give `x = 2, y = 2`
and `reverse` gives `x = 1, y = 1`.

### Two writers of one feature before a long tail of closed choices: two values

Fixture: `action_explore_early_race_long_tail` (explored, checked).

```
start → split ⇉ a { x := 1 } → p1 { p := 1 } → p2 { p := 2 } ─┐
              ⇉ b { x := 2 } → q1 { q := 1 } → q2 { q := 2 } ─┤→ sync → done
```

Derived constraints:

- `a` and `b` are each performed exactly once (ForkAction) and `sync` follows both branches
  (JoinAction), so `x` is `1` or `2`, never `0`, at the end.
- `p1` HappensBefore `p2` and `q1` HappensBefore `q2`, each branch writing its own feature, so
  `p` and `q` both end `2` whatever the interleaving.

Open: the order of `a` against `b`, and the interleaving of the two branches — the library links
neither. The last write of `x` stands, so the two orders of the writes are two outcomes; the
`C(6, 3) = 20` interleavings of the branches split ten and ten by which write comes last, so the
twenty linearizations reach exactly two outcomes, `{x = 1, x = 2}`, ten each.

Pinned outcome: that admissible set, stated as `outcomes` citing this section, and a `check`
divergent over `x` alone. The case is what distinguishes an exploration's plan order: a run meets
the one open choice first and closed ones after it, so a walk taking the deepest untried
alternative first spends the ten orders under one write order before it varies the write order,
and a budget under eleven runs tables one value; a walk varying every choice of the first run
once before any twice tables both by the second run. `TestExploreVariesEveryChoiceOfTheFirstRunFirst`
pins that order; the harness explores the case to its complete table of two.

### A performed action and a sibling accept due at one instant: which resumes first is open

Fixture: `action_explore_performed_and_accept_due_together` (golden, explored).

```
action def Sleeper { start → nap accept after 2 [s] → rested }
start → split ⇉ performed : Sleeper → writeOne { x := 1 } ─┐
              ⇉ direct accept after 2 [s] → writeTwo { x := 2 } ┤→ sync → done
```

Derived constraints:

- `performed` performs `Sleeper` as a suboccurrence, so `Sleeper`'s `localClock` is `wake`'s
  (`Occurrences.kerml`, "The localClock of a suboccurrence defaults to the localClock of its
  containing occurrence"): `nap` and `direct` are `TriggerAfter(2, …)` against one clock, each
  fixed at `0 + 2` when reached (`Triggers.kerml`), and each ends when that clock reads 2
  (`TimeSignal::signalCondition`).
- `writeOne` follows `performed`, which ends after `nap` (HappensBefore, and a performed action
  ends after its last step); `writeTwo` follows `direct`; `sync` follows both writes
  (JoinAction). Each write is performed exactly once, so `x` is `1` or `2`, never `0`.

Open: the order of `performed`'s resumption against `direct`'s, and so of `writeOne` against
`writeTwo`. Both accepts end at the same reading of the one clock, no `HappensBefore` chain
connects a step of one branch to a step of the other, and `timeOrderingConstraint`
(`Clocks.kerml`, `TimeOf`) orders only occurrences already ordered by `HappensBefore`. Two chains of
two moves each — resume `performed` then `writeOne`, take `direct` then `writeTwo` — interleave six
ways; the three that end with `writeOne` give `x = 1`, the three that end with `writeTwo` give
`x = 2`.

Pinned outcome: the admissible set `{x = 1, x = 2}`, stated as `outcomes` citing this section;
`.trace.order` states the partial order the library does fix (`split < performed`,
`performed < writeOne`, `split < direct`, `direct < writeTwo`, both writes and `sync` before
`done`). Exploration must reach both outcomes and no other, in six runs (6 runs, 2 outcomes,
complete). The case is what distinguishes a paused body from a parked token in the explored
move set: `performed`'s token is paused inside `Sleeper` when the clock reaches 2, and
`direct`'s is parked at its accept. Had the executor resumed paused bodies only after every
ordinary token of a step had acted, as the fixed policies sweep, then under `explore` — where a
step ends with the first move — `direct` would always have run first, `writeOne` would always have
been last, and the exploration would have reported `x = 1` alone, complete. The fixed policies
sweep ordinary tokens before paused bodies, so each takes `direct` first, then steps the two
writes in its own order in the next step: `reverse` (and `seed:1`) writes `x := 2` then `x := 1`,
giving `x = 1`; `declared` writes them the other way round, giving `x = 2`.

### Dispatch during an entry the model does not run to completion

Fixture: `state_run_to_completion_false_self_signal`,
`state_run_to_completion_scope_sibling_region`, and
`state_run_to_completion_scope_parent_transition`, and
`state_run_to_completion_scope_qualified_source`, and
`state_run_to_completion_terminate_during_held_entry` (goldens, explored).

An entry that does not run to completion and a dispatch due at the same instant
may proceed in either order. Dispatching first exits the entered composite
before its unfinished entry reaches the nested state; completing the entry first
visits that nested state before the dispatch exits the composite. Both traces
are valid linearizations of the same instant.

### A decision inside a loop: every pass is its own open choice

Fixture: `action_explore_decision_in_loop` (golden, explored).

```
start → again → pick ─ if passes < 2  → left  { lefts++;  passes++ } → again
                     ─ if passes < 2  → right { rights++; passes++ } → again
                     ─ if passes >= 2 → done
```

Derived constraints:

- `pick` is followed by a performance of exactly one target on each pass (DecisionPerformance), so
  each pass adds one to `passes` and one to exactly one of `lefts` and `rights`.
- On the first two passes `passes < 2` holds and `passes >= 2` does not, so `done` is not
  selectable and one of `left`, `right` is; on the third `passes = 2`, only `done` is
  selectable, and the loop ends. Every run ends with `passes = 2` and `lefts + rights = 2`.

Open: which of `left` and `right` follows `pick` on each of the two passes. Each pass is its own
decision performance, constrained by nothing the earlier pass did, so the four sequences
`left,left`, `left,right`, `right,left`, `right,right` are all valid linearizations. Two of them
agree on the tallies, so they reach three outcomes: `{lefts = 2, lefts = 1 ∧ rights = 1,
rights = 2}`.

Pinned outcome: that admissible set, stated as `outcomes` citing this section. The default takes
the first declared branch on every pass, so the golden records `left` twice. Exploration must
reach all three outcomes and no other in four runs, the third pass never being a choice point:
a decision whose guards leave one link selectable is not a choice, however many links it has.

### Two accepts of one type racing for two sends: each takes one message, which one is open

Fixture: `action_choice_shared_message_accept` (golden).

```
start → split ⇉ sendOne { send 1 } → sendTwo { send 2 } ─┐
              ⇉ left  accept x : Integer                 ─┤→ sync → recorder { a := x; b := y } → done
              ⇉ right accept y : Integer                 ─┘
```

Derived constraints:

- `sendOne`, `sendTwo`, `left` and `right` are each performed exactly once (ForkAction; a plain
  step is one performance), and `sendOne` ends before `sendTwo` starts (`HappensBefore`).
- Each send is followed by exactly one `MessageTransfer` carrying its payload
  (`SendPerformance::sentTransfer`, `succession self then sentTransfer`), so two transfers exist,
  one carrying `1` and one carrying `2`, and the first is sent before the second.
- Each accept ends after exactly one transfer to `this` and yields that transfer's payload
  (`AcceptPerformance::acceptedTransfer`, `succession acceptedTransfer then self.endShot`,
  `binding payload = acceptedTransfer.payload`), so `x` and `y` each end as `1` or `2`, never `0`.
- `sync` follows `sendTwo`, `left` and `right` (JoinAction), so both accepts have ended, and both
  sends, before `recorder` reads `x` and `y`.

Open: which transfer each accept takes. The transfers' `HappensBefore` links order each send
before the accept that takes its transfer and nothing else: no link orders `left` against `right`,
and `acceptedTransfer` names *a* transfer to the receiver, not the one a particular send made. A
transfer is one link object with one `payload`; the reading this record relies on is that a
transfer is accepted once — the two accepts take the two transfers, one each — which is the
reading under which a second accept parked at a receiver waits for a second message rather than
re-reading the first. Under it either pairing is admissible: `left` takes `1` and `right` `2`, or
the reverse.

Pinned outcome: the admissible set `{a = 2 ∧ b = 1, a = 1 ∧ b = 2}`, stated as `outcomes` citing
this section; exploration reaches each twice (4 runs, 2 outcomes, complete): once the first
message is in flight, `sendTwo` and both accepts are able to act, and an accept picked first takes
the message while the other must wait for `sendTwo`, whereas `sendTwo` picked first leaves both
messages to the two accepts and the accept picked next takes the older. The partial order the
library fixes among the nodes is stated as `.trace.order`
constraints (`split < sendOne`, `split < left`, `split < right`, `sendOne < sendTwo`,
`sync < recorder`, `recorder < done`); the join's predecessors are not stated as constraints on
`sync` because a token parks at a join before the join performs, so the entry first mentioning
`sync` may precede the last branch's arrival. The exact trace golden records the default
scheduling: the accepts are stepped before the sender, both park, and when the first message is
in flight the accept declared last is stepped first and takes it (`choice step 4: tokens
2@sendTwo, 3@left, 4@right (unordered; took 4@right first)`), leaving `2` to `left` — `a = 2`,
`b = 1`. Under `declared` and `seed:1` (`.declared.trace.golden`, `.seed-1.trace.golden`) the
sender is stepped first and the accept declared first takes the message it just sent, in two
successive steps (`choice step 3: tokens 2@sendOne, 3@left (unordered; took 2@sendOne first)`,
then `2@sendTwo, 4@right`), so `left` takes `1` and `right` takes `2` — `a = 1`, `b = 2`. That
linearization reaches two choice points where the default reaches one: the reporting rule (every
choice point a run reaches is reported) applied to a different run, not a difference in what the
model admits.

### Two accepts addressed by two sends completing in either order: each payload is fixed, the one that stands is open

Fixture: `w7d_send_via_port_to_receiver` (golden).

```
start → sender { send 42 via senderPort to receiver; send 7 via senderPort to sibling }
      → split ⇉ receiver accept value : Integer via receiverPort { receiverGot := value } ─┐
              ⇉ sibling  accept value : Integer via receiverPort { siblingGot := value  } ─┤→ sync → done
```

Derived constraints:

- `sender`, `receiver` and `sibling` are each performed exactly once (a plain step is one
  performance; ForkAction), and both sends end before `split` starts (`HappensBefore`).
- Each send is followed by exactly one `MessageTransfer` carrying its payload to the receiver it
  names (`SendPerformance::sentTransfer`, `succession self then sentTransfer`), so `receiver`'s
  transfer carries `42` and `sibling`'s carries `7`; neither accept can take the other's, so
  `receiverGot` ends `42` and `siblingGot` ends `7` in every run.
- Each accept ends after its transfer and yields that transfer's payload
  (`AcceptPerformance::acceptedTransfer`, `binding payload = acceptedTransfer.payload`). Both
  declare `accept value : Integer`, and an accept's payload is bound in the enclosing action body
  (the visibility the `accept_payload_*` cases pin), so the two accepts write one feature,
  `route`'s `value`.
- `sync` follows both accepts (JoinAction), so both writes of `value` have ended before the
  action ends and `value` is `42` or `7`, never unset.

Open: the order of `receiver` against `sibling`. Each transfer's `HappensBefore` link orders its
own send before the accept that takes it and nothing else; no link orders the two accepts, and
the library has no conflict rule for two performances writing one feature, so the write that
stands is the one whose accept completes last — `value = 42` when `sibling` completes first,
`value = 7` when `receiver` does.

Pinned outcome: the admissible set `{value = 42, value = 7}`, with `receiverGot = 42` and
`siblingGot = 7` in both, stated as `outcomes` citing this section; `.trace.order` states the
partial order the library does fix (`sender < split`, `split < receiver`, `split < sibling`,
`sync < done`). The exact golden records the default schedule: `sibling` is declared last, so its
token is stepped first and `receiver`'s write stands (`choice step 4: writes value := 42 by token
2, value := 7 by token 3 (unordered; value := 42 by token 2 stood)`), giving `value = 42`.
Exploration reaches both outcomes in two runs. That the payload of a nested accept is reported
as a feature of the enclosing action at all is the tool's reporting, not the library's; this
record derives only that, given that reporting, both values are admissible.

### Two transitions out of one state enabled by one event: exactly one fires, which one is open

Fixture: `state_choice_transition_conflict` (golden).

```
idle ─ accept Go if level > 5 → low  { route := 1 }
     ─ accept Go if level > 7 → high { route := 2 }
```

Derived constraints:

- A `StateTransitionPerformance` follows its trigger and its guard, and its
  `transitionLinkSource.exit` follows the guard (`StatePerformances.kerml`); the source performance
  `idle` ends once, so at most one transition out of it fires for one Go.
- Both guards hold for `level = 8`, so both transitions are enabled by the one event; the
  machine ends in `low` or `high`, never still in `idle`.

Open: which enabled transition fires. UML orders a transition on a descendant state before one
on its ancestor (the case `state_choice_ancestor_priority_not_reported` pins that rule, and the
executor does not report it as a choice); between two transitions on the *same* state nothing
in the library or the specification ranks them. The choice is the firing transition's: the
regions of a parallel state that select the same transition out of it make one choice, reported
once (`state_choice_shared_ancestor_regions`), and a composite state's transitions that lose to a
nested one were never chosen among, so nothing about them is reported
(`state_choice_ancestor_outranked_not_reported`).

Pinned outcome: the admissible set `{route = 1 in low, route = 2 in high}`, stated as `outcomes`
citing this section; exploration reaches each once (2 runs, 2 outcomes, complete), as it does for
the companion fixtures `state_choice_shared_ancestor_regions` and
`state_choice_change_transition_conflict` (2 runs, 2 outcomes each); `state_explore_transition_conflict`
states the same set for two transitions converging on one target, told apart by `side` and the
visit list. The executor examines every transition out of the state for the event,
fires the first declared, and records the choice (`choice state idle on accept Go: transitions
1->low, 2->high (unordered; took 1->low)`); the golden pins that linearization. As for a decision,
the transitions after the first enabled one are read in a preview that is undone, and one whose
guard cannot be evaluated is not an alternative and does not fail the dispatch: it is recorded as
an informational `guard-unevaluable` note naming the state, the event and the transition
(`TestLaterGuardErrorIsNotAChoiceNorAFailure`; fixture `state_choice_unevaluable_transition`,
golden). The first transition read is the run's own, and its failure fails the dispatch as it
always has (`TestFirstTransitionFailureStillFailsTheRun`). A state's completion is one occurrence
too: several unguarded completion transitions out of one state are the same choice, drawn when
the completion is dispatched, and the completion fires exactly one of them
(`state_explore_completion_choice`, 2 runs, 2 outcomes, complete).

The choice is recorded when the transition fires, not when it is selected: a transition selected
on the event reads its guard once more as it fires, and one another region's reaction has
meanwhile disabled does not fire and reports nothing
(`TestNotesOfATransitionBlockedBeforeFiringAreDropped`), while one whose effect then fails was
the run's choice all the same (`TestNotesOfATransitionFailingInItsEffectAreKept`). A change
occurrence is an event like a signal: two `accept when` transitions out of one state whose
conditions rise on one write are enabled by one occurrence, and the same rule applies — the
first declared fires, the rise is consumed for the others, and the choice is recorded
(`choice state watching on change: transitions 1->cool, 2->hot (unordered; took 1->cool)`;
fixture `state_choice_change_transition_conflict`, golden; `TestChangeTransitionChoice`,
`TestChangeTransitionChoiceUnderHierarchyAndRegions` for the nested-wins and parallel-region
shapes).

### Two branches of a choice enabled by the data the incoming effect wrote: exactly one is taken, which one is open

Fixture: `state_choice_dynamic_conflict` (golden, explored).

```
idle ─ accept Go { level := 8 } → pick ─ if level > 5 → low  { route := 1 }
                                       ─ if level > 7 → high { route := 2 }
```

Derived constraints:

- A choice is a `DecisionPerformance` reached through the segment into it: the segment's
  effect happens before what the segment leads to (`TransitionPerformances.kerml`,
  `succession [*] effect then [1] transitionLink.laterOccurrence`), so the guards the decision
  reads are read against `level = 8`, not against the `level = 0` the machine held when Go
  arrived. UML says the same in words: a choice vertex's guards are evaluated dynamically, after
  the incoming transition's behavior has run, where a junction's are evaluated statically with
  the compound transition's enabledness (UML 2.5.1 §14.2.3.7, `choice` and `junction`).
- `DecisionPerformance::outgoingHBLink: HappensBefore[1]` (`ControlPerformances.kerml`): exactly one
  branch follows, so the machine ends in `low` or `high`, never at `pick` and never in both.
- Both guards hold for `level = 8`, so both branches are enabled once the effect has run; had the
  guards been read before it, neither would hold and the transition would have no branch.

Open: which enabled branch is taken. Nothing in the library or the specification ranks two
branches of one choice whose guards both hold.

Pinned outcome: the admissible set `{route = 1 in low, route = 2 in high}`, stated as `outcomes`
citing this section; exploration reaches each once (2 runs, 2 outcomes, complete). The executor
exits `idle`, runs the incoming effect, reads the branches in declaration order, takes the first
enabled one and records the choice at the choice vertex (`choice choice pick: transitions 1->low,
2->high (unordered; took 1->low)`); the golden pins that linearization, `seed:1` the other one. As
for a transition conflict, branches after the first enabled one are read in a preview that is
undone. A choice with no enabled branch and no unguarded one fails the run at that instant with a
typed error naming the choice (`TestRuntimeRobustness/state_choice_without_an_enabled_branch`).

### Two branches of a junction enabled when the transition is selected: exactly one is taken, which one is open

Fixture: `state_junction_several_enabled_branches` (golden, explored).

```
idle ─ accept Go → split ─ { route := 1 } → left
                         ─ { route := 2 } → right
```

Derived constraints:

- A junction is a `DecisionPerformance` like a choice; what differs is when its guards are read.
  UML reads a junction's guards statically, with the enabledness of the compound transition, before
  any of its effects run (UML 2.5.1 §14.2.3.7, `junction`; §14.2.3.8.1, "compound transition"),
  so the two branches here, both unguarded, are both enabled when Go is dispatched from `idle`.
- `DecisionPerformance::outgoingHBLink: HappensBefore[1]` (`ControlPerformances.kerml`): exactly one
  branch follows, so the machine ends in `left` or `right`, never at `split` and never in both,
  and exactly one of the two branch effects runs (`TransitionPerformances.kerml`,
  `succession [1] transitionLinkSource then [*] effect`, per segment taken).

Open: which enabled branch is taken. Nothing in the library ranks two branches of one junction
whose guards both hold, and UML says the same: when several outgoing transitions of a junction are
enabled, "one is chosen; the algorithm for making this selection is not defined" (PSSM 1.0
`Junction003`, restating UML 2.5.1 §14.2.3.9.1 on conflicting transitions).

Pinned outcome: the admissible set `{route = 1 in left, route = 2 in right}`, stated as `outcomes`
citing this section; exploration reaches each once (2 runs, 2 outcomes, complete). The executor
reads the branches when it selects the transition out of `idle`, takes the first enabled one and
records the choice at the junction as the transition fires, before `idle` is exited (`choice
junction split: transitions 1->left, 2->right (unordered; took 1->left)`); the golden pins that
linearization, `seed:1` the other one.
As at a choice, branches after the first enabled one are read in a preview that is undone. If no
branch has a way through, the compound transition is unenabled and the occurrence is handled as
unmatched; a junction reached only past a choice remains a run error. The distinction and its
fixtures are pinned in the no-way-through section above.

### A junction with no way through: the transition is not enabled, and the occurrence is handled as unmatched

Fixture: `state_junction_no_way_through_unmatched` (trace golden),
`state_junction_no_way_through_other_transition_fires`,
`state_junction_no_way_through_deferred`, `state_junction_dead_branch_not_drawn`,
`state_completion_no_way_through_dropped`, `state_history_default_no_way_through`,
`state_history_self_transition_default_no_way_through_restores`, and
`state_join_no_way_out_disables_last_segment` (explored outcomes).

Derived constraints:

- UML 2.5.1 §14.2.3.7 (junction) and §14.2.3.8.1 (compound transition) are the extension's
  reference for static route availability. The library declares
  `feature outgoingHBLink: HappensBefore[1]` (`ControlPerformances.kerml`), but does not define
  a state-machine junction's enablement or unmatched-event behavior.
- The runtime checks a route before selecting its transition, only as far as the first choice.
  A junction with no way through, or a join whose completing occurrence has no route out, leaves
  the compound transition unenabled; `errNoWayThrough` is the only route error that disables it.
  The occurrence can then select another enabled transition, remain deferred, or be discarded as
  unmatched. A completion with no way through is dropped.
- A branch that reaches a later junction with no way through is removed from the current
  junction's drawable branches; if one branch remains, it is followed without a draw. Cycles,
  unevaluable guards and binding failures remain run errors. A route stays open at the first
  choice: choices resolve dynamically on arrival and retain `ErrChoiceWithoutBranch`; a dead
  junction reached beyond a choice remains a run error.
- A history default with no recorded history is statically checked only when the source is
  outside the history owner. A transition from the owner or one of its descendants can exit the
  owner, record history, then restore it instead of being disabled based on a record that does
  not exist yet.

Open: when more than one ordinary transition can take an occurrence after a dead route is
disabled, the existing transition-selection policy decides among them; this rule adds no new
choice point for the unavailable route.

Pinned outcome: `state_junction_no_way_through_unmatched` leaves the machine in `s2`, logs
`T3`, and reports the original `Start` as unmatched. In
`state_junction_no_way_through_other_transition_fires`, the other `Start` transition fires; in
`state_junction_no_way_through_deferred`, the occurrence remains deferred. The dead branch in
`state_junction_dead_branch_not_drawn` is not drawn when the other route remains available.
`state_completion_no_way_through_dropped` keeps the source state active until a later signal;
the outside-owner history default is disabled, while the owner self-transition fixture restores
its recorded history. The join fixture ends in `S3` with either `T1.2 T5` or `T1.4 T5`.

### A junction with two branches enabled in a region another region's reaction may disarm: drawn only as its transition fires

Fixture: `state_junction_drawn_as_its_transition_fires` (golden, explored).

```
work ─┬─ a: a1 ─ accept Go { armed := false } → a2
      └─ b: b1 ─ accept Go [armed] → split ─ { route := 1 } → left
                                            ─ { route := 2 } → right
```

Derived constraints:

- One Go selects a transition in each region of `work`; the two fire in an open order (the
  section before). Region b's guard `armed` and the junction's branches are read when the
  transitions are selected, before either fires, and both branches hold.
- Region a's effect disarms `armed`. Fired first, it leaves b's transition unenabled when its turn
  comes, so b stays in `b1`, `route` stays 0 and nothing follows the junction: a compound transition
  whose guard no longer holds does not fire (UML 2.5.1 §14.2.3.9.1). Fired second, b's transition
  takes the junction and exactly one branch (`DecisionPerformance::outgoingHBLink:
  HappensBefore[1]`), so the machine ends in `left` or `right`.

Open: the region order, and, when b fires, which enabled branch it takes.

Pinned outcome: the admissible set `{a2+b1 with route = 0, a2+left with route = 1, a2+right with
route = 2}`, stated as `outcomes` citing this section; exploration reaches each once (3 runs, 3
outcomes, complete). The branch is drawn only as b's transition fires, after the region order is
drawn: a witness reads `on accept Go: b1 first of a1, b1; junction split -> 2->right`, in that
order, and the run in which a fires first draws nothing at the junction, so replaying its
witness meets no draw it does not list. The golden pins the a-first linearization, and so does the
`seed:1` golden, whose draw falls the same way; the b-first runs are exploration's.

### A junction's guards are read once, as its incoming transition is selected: a branch enabled then is taken though another region's effect since made its guard unevaluable

Fixture: `state_junction_guards_read_once` (golden, explored).

```
work ─┬─ a: a1 ─ accept Go { d := 0 } → a2
      └─ b: b1 ─ accept Go → split ─ [v > 0]     { route := 1 } → left
                                    ─ [v / d > 1] { route := 2 } → right
```

Derived constraints:

- A junction's guards are static: they are read when the compound transition through it is
  selected, before any effect of the step runs (UML 2.5.1 §14.2.3.7, `junction`; §14.2.3.8.1,
  "compound transition"). With `v = 4` and `d = 2` both branches hold when Go is dispatched, so b's
  transition is enabled through either.
- Region a's effect zeroes `d`. Fired first, it leaves `v / d` unevaluable, but the guard is not
  read again: the enabled set b's transition was selected with stands, and the branch drawn from
  it is taken. Exactly one branch follows (`DecisionPerformance::outgoingHBLink:
  HappensBefore[1]`), so the machine ends in `left` or `right`, never fails the run.

Open: the region order, and which enabled branch b takes.

Pinned outcome: the admissible set `{a2+left with route = 1, a2+right with route = 2}`, stated as
`outcomes` citing this section; exploration reaches each once per region order (4 runs, 2
outcomes, complete), the a-first run through the second branch among them. The golden pins the
a-first linearization through the first branch, and the `seed:1` golden, its draws falling the same
way, the same one; the b-first runs are exploration's.

### Every junction guard on a route is read once, as its transition is selected: a junction beyond a draw takes the branch enabled then though another region's effect since changed what its guards read

Fixture: `state_junction_beyond_a_draw_read_once` (golden, explored).

```
work ─┬─ a: a1 ─ accept Go { d := 0 } → a2
      └─ b: b1 ─ accept Go → split ─ { route := 1 } → again ─ [4 / d > 1] { route += 10 } → left
                                    ─ { route := 2 } → again ─ [d == 0]    { route += 20 } → right
```

Derived constraints:

- A compound transition's junction guards are static, on every junction of the route: they are
  read when the transition is selected, before any effect of the step runs (UML 2.5.1 §14.2.3.7,
  `junction`; §14.2.3.8.1, "compound transition"). With `d = 2`, `split` has both branches enabled
  and `again`, beyond either, has its first branch enabled and its second not; b's transition is
  enabled through `split`'s two branches, each on to `left`.
- Region a's effect zeroes `d`. Fired first, it would make `4 / d` unevaluable and `d == 0` hold,
  but `again`'s guards are not read again once `split` is drawn: the route beyond each of `split`'s
  branches was settled with its transition, so `again` takes the branch enabled then, to `left`,
  and `route` is 11 or 12 — never 21 or 22, and the run never fails.

Open: the region order, and which of `split`'s enabled branches b takes.

Pinned outcome: the admissible set `{a2+left with route = 11, a2+left with route = 12}`, stated as
`outcomes` citing this section; exploration reaches each once per region order (4 runs, 2
outcomes, complete), the a-first runs among them. The golden pins the a-first linearization
through the first branch, and the `seed:1` golden, its draws falling the same way, the same one;
the b-first runs are exploration's.

### A history without a record takes its default transition through a junction with two branches enabled: exactly one is taken, which one is open

Fixture: `state_history_default_through_junction` (golden, explored).

```
idle ─ accept Go → work.resume ─ (default) → split ─ { route := 1 } → w1
                                                    ─ { route := 2 } → w2
```

Derived constraints:

- `work` has never been left when Go arrives, so its history holds no record and the default
  transition out of it is taken (UML 2.5.1 §14.2.3.7, `shallowHistory`), after `work` is entered.
- The default transition ends at a junction; both branches hold, exactly one follows
  (`DecisionPerformance::outgoingHBLink: HappensBefore[1]`), so the machine ends in `w1` or `w2`
  with the one branch effect run.

Open: which enabled branch is taken, as at any junction.

Pinned outcome: the admissible set `{route = 1 in w1, route = 2 in w2}`, stated as `outcomes`
citing this section; exploration reaches each once (2 runs, 2 outcomes, complete). The draw is
recorded as a `ChoiceTransition` at `junction split` when the default route is taken, so it
appears among the run's notes and choices and in the trace like a junction reached from a
transition, and a seed replays it. The golden pins the first branch, `seed:1` the other one.

### Transitions in sibling regions enabled by one event: each fires, in which order is open

Fixtures: `state_explore_region_order` (golden, explored), `state_firing_units_interleaved`
(golden, explored).

```
work parallel { a: a1 ─ accept Go { last := 1 } → a2
                b: b1 ─ accept Go { last := 2 } → b2 }
```

Derived constraints:

- The regions of a parallel state are concurrent substate performances of it; the one Go is
  offered to both, and each region's transition has its own source, so neither outranks the other
  (the nested-wins rule of the previous section ranks a substate's transition against its
  *enclosing* state's, never one region's against a sibling's) and both fire.
- Each `StateTransitionPerformance` is ordered only against its own trigger, guard,
  `transitionLinkSource.exit` and target entry (`StatePerformances.kerml`,
  `TransitionPerformances.kerml`); no link joins one region's transition to the other's. UML says
  the same of the set of transitions selected for one event: the order in which they fire is not
  defined (UML 2.5.1 §14.2.3.9.4).
- Both effects write `last`, so the value that stands is the last write: `last = 2` when `a`'s
  transition fires first, `last = 1` when `b`'s does; the machine ends in `a2+b2` either way.

Open: which region's transition fires first. The two orders reach two outcomes, told apart by
`last` and by the order `a2` and `b2` are visited in.

Pinned outcome: the admissible set `{last = 2 visiting a2 then b2, last = 1 visiting b2 then a2}`,
stated as `outcomes` citing this section. The order is a choice point under every policy, drawn one
unit at a time — a firing's source exit, its effect and its target entry are its units, and the
draw is among the firings with a unit left — and reported as `choice on accept Go: next a1(exit),
b1(exit) (unordered; took a1(exit) first)`: `declared` and `reverse` take the firings whole in region
declaration order — a tool-defined order — and the default golden pins that linearization (`a`
first, `last = 2`); `seed:<n>` draws each unit, the `seed:1` golden's draws falling on the same
order; `explore` varies every draw (`took b1(exit) first` in the witness of the second
outcome) and must reach both outcomes and no other. Here the finer grain reaches no third outcome,
since each region logs one write. `state_firing_units_interleaved` logs each source's exit and each
effect, so the grain shows: the units of one firing keep their order (`transitionLinkSource then
effect`, `TransitionPerformances.kerml`), no succession joins them to the other firing's, and the
six linearizations of two chains of two — a source's exit falling between the other firing's exit
and effect among them — are the admissible set, each followed by the join's segment. The fixtures
`state_call_trigger_regions`, `state_composite_region_depth_order`,
`state_composite_region_deeper_first` and `state_parallel_broadcast` are this same shape and list
both orders as `outcomes` citing this section, the default golden of each pinning the
declaration-order linearization. `state_change_region_order` is the shape with a change
occurrence in place of the signal — one write of `temp` raises `temp > 20` in both regions at once
— and lists the same two outcomes: a change occurrence is an event like any other, so the poll
that dispatches it draws the region order the same way (`choice on change: next a1(exit), b1(exit)
(unordered; took a1(exit) first)`), and no order between the two raised conditions is derivable from the
library either.

### Regions of a parallel state entered on one occurrence: each is entered, in which order is open

Fixtures: `state_region_entry_order` (golden, explored), `state_region_entry_order_uneven` (golden,
explored), `state_fork_branch_order` (golden, explored), `state_region_entry_nested_front` (golden,
explored), `state_history_restore_order` (golden, explored).

```
idle ─ accept Go → work parallel { left:  { entry { log += "left(entry) " } ; entry; then l { entry { log += "l(entry) " } } }
                                   right: { entry { log += "right(entry) " } ; entry; then r { entry { log += "r(entry) " } } } }
```

Derived constraints:

- Entering a parallel state starts one substate performance per region, and those are concurrent
  (SysML v2 §7.18.1: parallel substates are "performed concurrently"). Within a region the library
  fixes the order the trace logs: the region's own entry precedes the entry of the state its
  initial transition reaches (`StatePerformances.kerml` `StatePerformance`: `succession [1] entry
  then [*] middle`, the substates being `middle` steps), so `left(entry) < l(entry)` and
  `right(entry) < r(entry)`.
- No succession joins a step of one region's chain to a step of the other's, and the owner's entry
  precedes both chains (the regions are its `middle`), so the library leaves the two chains
  unordered against each other.
- Every entry appends to `log`, so `log` records the interleaving.

Open: the interleaving of the two chains. Two chains of two have six linearizations.

Pinned outcome: the admissible set of those six values of `log`, stated as `outcomes` citing this
section. The order is a choice point under every policy, drawn one unit at a time among the regions
with an entry left and reported as `choice entering work: next left(entry), right(entry) (unordered;
took left(entry) first)`; `declared` and `reverse` take the regions whole in declaration order — a
tool-defined order — and the default golden pins that linearization; `seed:<n>` draws each unit;
`explore` varies every draw and must reach all six and no other. `state_region_entry_order_uneven`
is the shape with one chain of one (`left` logs its entry, `l` nothing) and one of two: three
linearizations. `state_fork_branch_order` reaches the two regions through a fork instead of the
owner's initial transitions: each branch is a chain of its segment's effect then its target's entry,
the owner's entry is one performance (`Actions.sysml` `ForkAction`, one performance of every target,
of one `work`) performed by whichever branch is drawn to it first and preceding both targets, so the
four linearizations of `{T1.1(effect), T1.2(effect)}` around `work(entry)` are the set.
`state_region_entry_nested_front` makes one region's start state itself parallel: its two regions'
entries join the front the sibling region is drawn from, three chains of one, six linearizations.
`state_history_restore_order` restores two regions through a deep history: the restore enters the
recorded states as a front drawn the same way, and the fixture's eight outcomes are its two entry
orders on the first occurrence, two exit orders on leaving (the next section) and two restore
orders.

### Regions of a parallel state left on one occurrence: each is exited, in which order is open

Fixtures: `state_region_exit_order` (golden, explored), `state_history_restore_order` (golden,
explored).

```
work parallel { left:  l { exit { log += "l(exit) " } }
                right: outer { exit { log += "outer(exit) " } ; r { exit { log += "r(exit) " } } } }
  exit { log += "work(exit) " }
work ─ accept Go → rest
```

Derived constraints:

- A transition leaving `work` ends every active substate performance before `work`'s own exit
  (`StatePerformances.kerml` `StatePerformance`: `succession [*] middle then [1] exit`), and a
  nested state's exit precedes its parent's by the same succession one level down: `r(exit) <
  outer(exit)`, and both of `l(exit)` and `outer(exit)` before `work(exit)`.
- The two regions are concurrent substate performances; no succession joins `l`'s exit to `r`'s or
  `outer`'s, so the chain of one and the chain of two are unordered against each other.

Open: the interleaving of the two chains. A chain of one and a chain of two have three
linearizations.

Pinned outcome: the admissible set of those three values of `log`, each ending in `work(exit)`,
stated as `outcomes` citing this section. The order is a choice point under every policy, drawn one
unit at a time among the regions with an exit left and reported as `choice exiting work: next
l(exit), r(exit) (unordered; took l(exit) first)`; `declared` and `reverse` leave the regions whole
in declaration order — a tool-defined order — and the default golden pins that linearization;
`seed:<n>` draws each unit; `explore` varies every draw and must reach all three and no other.

### Two time events due at one instant: each dispatches, in which order is open

Fixture: `state_explore_time_trigger_tie` (golden, explored).

```
work parallel { a: a1 ─ accept after 2 [s] { last := 1 } → a2
                b: b1 ─ accept after 2 [s] { last := 2 } → b2 }
```

Derived constraints:

- The regions of `work` are concurrent substate performances of it, sharing its `localClock`
  (`Occurrences.kerml`, "The localClock of a suboccurrence defaults to the localClock of its
  containing occurrence"); each region arms `TriggerAfter(2, …)` on entering its first state at
  `t=0` (`Triggers.kerml`), and each ends when that clock reads 2 (`TimeSignal::signalCondition`).
  The two are two acceptable events, one per transition, each firing its own region's transition.
- `timeOrderingConstraint` (`Clocks.kerml`, `TimeOf`) orders only occurrences already ordered by
  `HappensBefore`, and no `HappensBefore` chain joins one region's transition to the other's; the
  pool order (`earlierFirstIncomingTransferSort`, `Occurrences.kerml`) ranks transfers, which a
  time signal is not. Neither dispatch precedes the other.
- Both effects write `last`, so the value that stands is the last write: `last = 2` when `a`'s
  timer dispatches first, `last = 1` when `b`'s does; the machine ends in `a2+b2` either way.

Open: which time event dispatches first. The two orders reach two outcomes, told apart by `last`
and by the order `a2` and `b2` are visited in.

Pinned outcome: the admissible set `{last = 2 visiting a2 then b2, last = 1 visiting b2 then a2}`,
stated as `outcomes` citing this section. The order is a dispatch-order choice point under every
policy, reported as `choice events at t=2.0: time a1 1->a2, time b1 1->b2 (unordered; dispatched
time a1 1->a2 first)`: `declared` and `reverse` dispatch the earlier armed first — a tool-defined
order — and the default golden pins that linearization (`a` first, `last = 2`); `seed:<n>` draws
the order; `explore` varies it and must reach both outcomes and no other, in two runs. A time event
due together with a signal already in the pool at the same instant is the same choice; two pool
events are not, their arrival order being library-derived (`earlierFirstIncomingTransferSort`).
A completion event precedes both, never drawn.

### Do behaviors of sibling regions active at one instant: each proceeds, in which order is open

Fixtures: `state_concurrent_do` (golden, explored), `state_anonymous_do_atomic` (golden, explored),
`state_concurrent_inline_do_bodies` (golden, explored), `state_concurrent_do_action_bodies_timed`
(golden, explored).

```
Interleave parallel { left:  lwork { do { seq := seq*10+1; seq := seq*10+2; seq := seq*10+3 } }
                      right: rwork { do { seq := seq*10+4; seq := seq*10+5; seq := seq*10+6 } } }
```

Derived constraints:

- A state's do behavior is a performance nested in the state performance, after its entry and
  before its exit (`StatePerformances.kerml` `StatePerformance`: `succession [1] entry then [*]
  middle; succession [*] middle then [1] exit`), so each region's do behavior runs while its state
  is active and the statements of one body keep their declared order (`1 < 2 < 3`, `4 < 5 < 6`).
- The regions of a parallel state are concurrent substate performances; no succession joins a step
  of one region's do behavior to a step of the other's, so the library orders nothing between them.
  The executor shares the machine one action at a time — each behavior with an action due performs
  one before any performs its next — and which of the due behaviors acts first in a round is a
  tool-defined order.
- Every statement writes `seq`, so the digits record the interleaving: in `state_concurrent_do`,
  each region's start state completes as it is entered, and the pool dispatches the two completions
  in the order they were generated (PSSM §8.5.9) — the order the entry draw entered the two start
  states — so the region entered first enters its working state one step before the other (its
  first digit is alone in its round), the next two rounds each have both due, and the other's last
  statement is alone again (its last digit last).

Open: which region's start state is entered first (the entry draw, whose two orders the pool
follows), and which region's do behavior acts first in each round both are due in. Two entry
orders of two rounds of two orders reach eight values of `seq`.

Pinned outcome: the admissible set `{124356, 142356, 124536, 142536, 415263, 451263, 415623,
451623}`, stated as `outcomes` citing this section. The entry order is the choice point of
[regions entered on one occurrence](#regions-of-a-parallel-state-entered-on-one-occurrence-each-is-entered-in-which-order-is-open),
reported as `choice entering Interleave: next lstart(entry), rstart(entry) (unordered; took
lstart(entry) first)` — an entry that performs nothing but generates a completion event is drawn,
its place in the pool being observable — and the round's order is a choice point under every
policy, reported as `choice do round at t=0.0: states lwork, rwork react (unordered; took lwork
first)`: `declared` and `reverse` take region declaration order at both and the default golden
pins that linearization (`124356`); `seed:<n>` draws both; `explore` varies both and must reach
all eight values and no other. `state_anonymous_do_atomic` is the same machine with each body
written as `do action { … }` rather than the braced `do { … }`; the two spellings are one
anonymous inline action of three statements, and an inline body yields after each statement,
so both interleave and reach the same eight values, not `123456`. `state_concurrent_inline_do_bodies` writes the left body as a `for`
loop over 1..2 followed by a statement and the right one as a statement followed by an `if` block
of two: an iteration and a statement of a nested block are each one step, so the same eight values
and no other are reached.
`state_concurrent_do_action_bodies_timed` is the shape with action bodies
that wait on the clock: both behaviors pause at an `accept after 2 [s]` and are due again in the
round at `t=2.0`, where the order of the two counts is open (`1324` entering order, `3124` the
other), while the counts at `t=4.0` and `t=5.0` are alone in their rounds.

### Transitions into a join: each exits its source and runs its effect before the owner is left, in which order is open

Fixture: `state_join_runs_every_incoming_effect` (golden, explored),
`state_join_segment_fires_on_own_signal` (trace golden), and
`state_join_segments_arrive_together_on_one_occurrence` and
`state_join_segment_not_chosen_does_not_arrive` (golden, explored).

```
Outer { exit { log += "outer(exit) " }
        Work parallel { left:  l1 ─ accept X: exit + effect → sync
                        right: r1 ─ accept Y: exit + effect → sync }
        sync join ─ effect → Rest { entry { log += "rest(entry)" } } }
```

Derived constraints:

- The library has no join among states (`fork` and `join` are action nodes, `Actions.sysml`
  `ForkAction` / `JoinAction`); the state-body form follows UML 2.5.1 §14.2.3.8.1 and PSSM
  §8.5.7 `JoinPseudostateActivation`. Each incoming segment is its own
  `StateTransitionPerformance`, with its own `feature trigger: MessageTransfer[*];` and
  `private succession [1] transitionLinkSource then [*] effect;`
  (`TransitionPerformances.kerml`): the source exits before that segment's effect, and a later
  trigger can fire another segment independently. A completion segment fires on its own
  completion; it is not combined with another completion event.
- The library does not state that one segment's occurrence waits for another, or provide a
  succession connecting the segments across regions. The independent-occurrence join rule is
  therefore UML's extension reference, not a claim supplied by the KerML library: arrived
  segments plus those enabled by the current occurrence and chosen by their regions in that
  dispatch complete the join only when all incoming segments are covered. A not-yet-arrived
  segment fires only when its source is active, its trigger
  takes the occurrence, its guard holds, and its region's dispatch chose that transition; a
  competing or nested transition chosen by the region is not displaced by a sibling's join firing.
- At selection, probes count only the candidate segment plus prior arrivals, and check the way
  out only if they complete the join. In signal/change dispatches, same-occurrence peers count
  only when their regions chose them; firing checks completion against those peers, and a dead
  route fires none. Timer expiries are separate occurrences even when due at the same instant;
  a dead completing segment is not enabled, leaving its timer group's alternatives available.
  `state_join_peer_not_chosen_does_not_block_arrival`,
  `state_join_time_segments_expire_together` and
  `state_join_dead_timer_join_keeps_group_alternative` pin these distinctions.
- A substate's steps are enclosed in its owner's state performance and are "and hence happening
  during the state performance" (`StatePerformances.kerml`); the library orders the owner's
  middle steps before its exit with `private succession [*] middle then [1] exit;`. Thus the
  runtime leaves `Work` and then `Outer` after the last incoming segment effect, before the
  outgoing segment's effect and `Rest`'s entry. PSSM instead leaves the owner before the final
  segment effect; the runtime's owner-exit ordering is the extension's reading recorded under
  SM34.

Open: which segment fires first when one occurrence enables several; the library orders neither
segment against its sibling.

Pinned outcome: `state_join_runs_every_incoming_effect` pins both incoming-effect orders, each
ending with `outer(exit) sync(effect) rest(entry)`; exploration reaches exactly both. In
`state_join_segment_fires_on_own_signal`, signal X logs its source exit and effect, then waits
without running Y's segment; signal Y logs its source exit and effect, followed by the owner exit
and outgoing effect. `state_join_segments_arrive_together_on_one_occurrence` shows one Go
occurrence firing both enabled segments in either `join sync` order, recording both arrivals;
Finish then fires the last segment, leaves the owner and runs the outgoing effect. Each segment's
source exit and effect are one ordered unit, while arrivals are retained until the incoming set
is complete.
`state_join_segment_not_chosen_does_not_arrive` shows that Go fires A's segment while the middle
region takes its nested `b1` transition; Finish records C, and the later Go fires B's segment and
completes the join. `state_join_peer_not_chosen_does_not_block_arrival` shows that an unchosen
peer cannot make a dead way out disable A when A would arrive alone. The region's first dispatch
does not also fire the unchosen B segment.

### Same-instant timer expiries are separate join occurrences

Each queued timer expiry is its own occurrence, even when several timers are due at the same
instant. Selection checks a segment with only the arrivals already recorded: an incomplete
segment arrives on its own expiry, while a later expiry completes the join. If that completion
has no way out, its segment is disabled and the other transitions in its source's timer group
remain available. If B's expiry is dispatched before A's, B's group can choose either its
incomplete join segment or its alternative. `state_join_time_segments_expire_together` pins the
separate arrivals; `state_join_dead_timer_join_keeps_group_alternative` pins the dead-exit
alternative and its admissible outcomes.

### A merge is re-entered on every traversal of a loop

Fixture: `action_merge_loop_reenters` (golden), after the specification's `ChargeBattery`.

```
start → continueCharging(merge) → monitor → decide ─ level < 100 ─→ addCharge ─┐
             ↑                                     └ level >= 100 → endCharging → done
             └────────────────────────────────────────────────────────────────────┘
```

Derived constraints:

- Each `decide` performance is followed by a performance of the one target whose guard holds
  (DecisionPerformance: `outgoingHBLink` "is an instance of exactly one of the Successions,
  ordering the DecisionPerformance as happening before an instance of the target").
- `then continueCharging` after `addCharge` is a `HappensBefore` link from that `addCharge`
  performance to a `continueCharging` performance, so each `addCharge` is followed by a merge
  performance, which is followed by a `monitor` performance and a `decide` performance.
- Each merge performance follows exactly one source performance — `start` for the first, the
  latest `addCharge` for the others — and a merge's incoming successions have source multiplicity
  0..1, so the merge reached from `start` needs no `addCharge` before it (MergeAction,
  MergePerformance).
- `monitor` increments `passes`; `addCharge` adds `50` to `level`, which starts at `0`.

Open: nothing that is observable; the loop is a chain. The fixture pins the one outcome without
`outcomes`; under `explore` the run reaches no choice point (1 run, 1 outcome, complete).

Fixed outcome: `monitor` runs with `level` at `0`, `50` and `100`; the third `decide` selects
`endCharging`; `level = 100`, `passes = 3`. This is the reading the specification's example and
the pilot corpus's comment ("a merge node is necessary to prevent a loop of successions from
being unsatisfiable") take for granted: the merge exists so the loop can be re-entered.

Executor: `level = 100`, `passes = 3`. `stepMergeNode` keeps no record of earlier traversals:
each arriving token is one `MergePerformance`, its body runs, then the guard on the merge's one
outgoing succession is read and the token is forwarded or retired, so the token carrying the loop
re-enters `continueCharging` as often as `decide` sends it back. The golden shows `monitor` reading `level -> 0`, `-> 50`
and `-> 100`, the third `decide` evaluating `level >= 100 -> true`, then `endCharging` and
`done`. A merge is the one node several successions reach that does *not* synchronize (the
previous two cases): `ActionExecutor.synchronizes` exempts `MergeNode`, since
`MergePerformance` follows *one* source performance while a plain node or join follows one per
succession. Loop termination is the guard's and the step budget's job, not the merge's:
`robustness_test.go:unguarded_loop_through_a_merge` runs `start → m(merge) → a → m` with no
exit to `ErrActionStepLimitExceeded`, both under `RunToCompletion` and after stepping it.

### A merge counts one performance per pass of a loop

Fixture: `action_merge_loop_three_passes` (golden).

```
start → again(merge) → work → decide ─ count < 3 ─┐      again { merged := merged + 1 }
          ↑                            └ count >= 3 → done   work  { count := count + 1 }
          └───────────────────────────────────────┘
```

Derived constraints:

- The same chain as above: each `decide` is followed by the one target whose guard holds
  (DecisionPerformance); each `decide → again` succession is a `HappensBefore` link to a merge
  performance, followed by a `work` performance and a `decide` performance.
- A merge performance is one performance with its own steps (`MergeAction` is an `Action`;
  `Actions.sysml` `Action::merges : MergeAction[0..*]`), so its body runs once per merge
  performance, i.e. once per arrival.

Open: nothing observable. The fixture pins the one outcome without `outcomes`; under `explore`
the run reaches no choice point (1 run, 1 outcome, complete).

Fixed outcome: `count = 3`, `merged = 3` — three passes of `work`, three merge performances.
The executor agrees; the golden shows `again`'s body and `work`'s body alternating three times
before `count >= 3 -> true`.

### A merge fed by a fork branch and by a loop passes every arrival

Fixture: `action_merge_fork_branch_and_loop` (golden).

```
start → split ⇉ gate(merge) → work → more(decide) ─ passes < 3 ─┐   gate { merged := merged + 1 }
              ⇉ prep → ↑                            └ passes >= 3 → done   work { worked := worked + 1 }
                       └────────────────────────────────────────┘   more { passes := passes + 1 }
```

Derived constraints:

- `split` is followed by exactly one performance of `gate` and one of `prep` (ForkAction, target
  multiplicity 1..1); `prep` is followed by a `gate` performance (HappensBefore). Each is its own
  merge performance following its own one source (MergePerformance, source multiplicity 0..1),
  and neither waits for the other: two tokens are downstream of `gate`.
- Each `gate` performance is followed by a `work` performance and a `more` performance; each
  `more` is followed by the one target whose guard holds (DecisionPerformance), and `more → gate`
  is a `HappensBefore` link to a further merge performance.
- `more`'s body is a step of the decision performance, so it ends before the guard on the
  outgoing succession is read (HappensBefore orders the whole source performance before its
  target). Each `more` performance therefore reads a `passes` it has itself just incremented.

Open: the interleaving of the two tokens at every node; which token takes which exit. The outcome
does not depend on it, so the fixture pins the one outcome without `outcomes`; under `explore`
every interleaving — a choice between the two tokens at each step until one of them is done, the
two threads dividing the four passes as `3 + 1`, `2 + 2` or `1 + 3` — reaches it. There are
8526 of them, more than the default budget of 1024 runs, so `explore` alone reports
`incomplete: runs budget 1024 hit after 1024 runs` with the one outcome tabled, and
`explore:runs=10000` completes (8526 runs, 1 outcome, complete).

Fixed outcome: `passes = 4`, `merged = 4`, `worked = 4`. Whatever the interleaving, `passes`
takes the values 1, 2, 3, 4 one `more` performance at a time, the two that read 1 and 2 select
`gate` and the two that read 3 and 4 select `done`, so the merge is reached twice from the fork
(once directly, once through `prep`) and twice from the loop, and `work` follows each of the
four. The executor agrees; the golden shows the direct token at `gate` in step 2 while the
other is still at `prep`, the two loop re-entries at steps 5 and 6, and `done` reached twice.
Had `passes` been counted in `work` instead, the outcome would depend on the interleaving
(one token's `more` may read the other's write), which is why the count is where it is.

### A merge performs on every arrival; the guard prunes the outgoing link, not the merge

Fixtures: `f63_merge_body_runs_on_traversal` (golden) and `action_merge_body_flips_own_guard`
(golden).

```
start → split ⇉ gate(merge) ─ if ready ─→ tail → done   gate { mergeRuns := mergeRuns + 1 }
              ⇉ slow → slower → ↑                        slower { ready := true }
                                                         tail { passed := mergeRuns }

start → count(merge) ─ if arrivals < 3 ─→ work ─┐        count { arrivals := arrivals + 1 }
          ↑                                     │        work { continued := continued + 1 }
          └─────────────────────────────────────┘
```

Derived constraints:

- Each arrival at `gate` is its own merge performance (MergePerformance, `incomingHBLink[1]`),
  and a merge performance is an `Action` with its own steps (`Action::merges : MergeAction[0..*]`),
  so `gate`'s body runs once per arrival: `mergeRuns` reaches 2 in the first model, `arrivals`
  counts every entry to `count` in the second.
- `succession first gate if ready then tail` is a `DecisionTransitionAction` — "the base type of
  TransitionUsages used as conditional successions in action models" (`Actions.sysml`) — hence a
  `NonStateTransitionPerformance` whose `transitionLinkSource: Performance[1]` is the `gate`
  performance (`binding transitionLink.earlierOccurrence = transitionLinkSource`) and which
  happens after it: `succession [1] transitionLinkSource then [1] Performance::self`
  (`TransitionPerformances.kerml`). The guard is a step of that later performance, so it reads
  the state the complete merge performance left, the merge body's write included.
- The guard constrains the link, not the source: `TPCGuardConstraint` ties `constrainedHBLink`
  (`transitionLink: HappensBefore[0..1]`) to `constrainedGuard` with `allTrue(constrainedGuard())`,
  so a false guard means the `HappensBefore` link to `tail` (or `work`) does not exist. The
  merge performance it would have left exists regardless — nothing in the library makes a
  performance conditional on its outgoing links.

Open: in the first model, the interleaving of the direct arrival with `slow → slower`; the
outcome does not depend on it, since `ready` is written before the second arrival either way.
Both fixtures pin one outcome without `outcomes`; under `explore` the first reaches it by every
interleaving of the direct arrival with the `slow → slower` branch (22 runs, 1 outcome, complete) —
when `slower` runs before the direct arrival reaches `gate`, that arrival reads `ready = true` and
goes on to `tail` too, and the two tokens' moves through `gate`, `tail` and `done` interleave — and
the second, a chain, reaches no choice point (1 run, 1 outcome, complete).

Fixed outcome, first model: `ready = true`, `mergeRuns = 2`, `passed = 2` — the direct arrival
performs `gate` and reads `ready = false`, so no link to `tail` follows it; the second arrival
performs `gate` again, reads `ready = true`, and `tail` reads the two merge performances.
Second model: `arrivals = 3`, `continued = 2` — the third `count` performance's own increment is
what its guard reads, so `work` does not follow it and the action ends with no token. Had the
guard been read before the body, `work` would have followed the third arrival (`continued = 3`),
which is the observable the second fixture pins.

Executor: both agree. `stepMergeNode` runs the body, then evaluates the outgoing succession's
guard and retires the token when it is false; the goldens show `assign mergeRuns` before each
`eval feature ready`, `tail` reading `mergeRuns -> 2`, and in the second model `arrivals < 3 ->
false` read straight after the increment that made it so, followed by `no active tokens`.

### A transition's guard, the source's exit, the effect and the target's entry, in that order

Fixture: `state_transition_guard_exit_effect_entry_order` (golden).

```
active { exit { x := 0 } } ── accept Go if x > 0 do y := x ──→ finished { entry { z := y + 1 } }
x = 5 initially
```

Derived constraints:

- The guard is evaluated after the trigger and before the source's exit
  (`StateTransitionPerformance`: `acceptable then guard`, `guard then transitionLinkSource.exit`),
  so it reads `x = 5` and holds.
- The exit ends the source state performance (`StatePerformance`: `middle then exit`), and the
  effect follows the source performance (`TransitionPerformance`:
  `transitionLinkSource then effect`), so the effect reads the exit's `x = 0`.
- The target state performance follows the effect (`effect then transitionLink.laterOccurrence`),
  and its entry is its first step (`entry then middle`), so the entry reads the effect's `y = 0`.

Open: nothing observable; the transition is a chain. The fixture pins the one outcome without
`outcomes`; under `explore` the run reaches no choice point (1 run, 1 outcome, complete).

Fixed outcome: `x = 0`, `y = 0`, `z = 1`, final state `finished`. The executor agrees; the golden
shows the guard reading `x -> 5`, then `exit: active`, then `assign y` reading `x -> 0`, then
`enter: finished` with `assign z` reading `y -> 0`. The guard appears twice in the golden; that is
the tool detail noted above, not a second reading the library asks for.

### An action token and a state transition due at one instant: which runs first is open

Fixture: `clock_action_state_due_together` (golden).

```
part beacon : Beacon   exhibit state blinking { dark ─ accept after 5 [s] → shining { lit := true } }
action watcher         start → arm { armed := beacon.lit == false } → wait accept after 5 [s]
                             → look { sawLit := beacon.lit } → done
```

Derived constraints:

- Every occurrence's `localClock` defaults to the `universalClock`, and a suboccurrence's to its
  container's (`Occurrences.kerml`, `feature localClock : Clock[1] default universalClock`;
  "The localClock of a suboccurrence defaults to the localClock of its containing occurrence"), so
  the action and the machine time their accepts against one clock, whose `currentTime` "advances
  monotonically" (`Clocks.kerml`, `Clock`).
- `accept after d` is `TriggerAfter(d, receiver, clock)`, which "returns … TriggerAt(clock.currentTime
  + delay, …)" (`Triggers.kerml`): the instant is fixed when the accept is reached, `0 + 5` for
  both here, since `arm` reads `beacon.lit` at instant 0, materializing the beacon and starting
  its machine before the action's own wait is set.
- Each accept ends after its `TimeSignal`, whose condition is "the currentTime of the signalClock
  being equal to the signalTime" (`Triggers.kerml`, `TimeSignal::signalCondition`;
  `AcceptPerformance`, `succession acceptedTransfer then self.endShot`), so neither `look` nor the
  entry of `shining` happens before the clock reads 5, and `arm` reads `lit` still `false`.

Open: the order of `look` and the entry of `shining`. Each follows its own accept, and the two
accepts end at the same reading of the one clock; no `HappensBefore` chain connects a step of the
action to a step of the machine, and `timeOrderingConstraint` (`Clocks.kerml`, `TimeOf`) orders
only occurrences already ordered by `HappensBefore`. `look` therefore reads `lit` either before or
after `shining`'s entry writes it.

Pinned outcome: the admissible set `{armed ∧ sawLit, armed ∧ ¬sawLit}`, stated as `outcomes`
citing this section. The executor draws the order of executors due at one instant through the
scheduling policy and records it as a `due order` choice naming the executors in the order they
were created: under the default policy the last created runs first, as the token order is
reversed — the beacon's machine, created when `arm` materialized the beacon, runs before the
action, and `look` reads `lit = true` (`choice at t=5.0: due action watcher, state machine
blinking of object #1 (unordered; ran state machine blinking of object #1 first)`); under
`declared` the action, created first, runs first and reads `lit = false`; `seed:1` draws one of
the two (`.declared.trace.golden`, `.seed-1.trace.golden`). One executor alone due at an instant
is not a choice and is not reported.

### A do step and a dispatch due at one instant: which goes first is open

Fixtures: `state_do_step_or_dispatch` (golden, explored), `state_do_step_among_completions`
(golden, explored), `state_do_step_or_tied_dispatch` (golden, explored),
`state_do_step_cuts_typed_do` (golden, explored), `state_do_step_cuts_nested_perform` (golden,
explored), `state_do_step_cuts_control_node_body` (golden, explored, checked),
`state_do_action_loop_timed_exit` (explored, checked).

```
state Machine { attribute log : String = "";
                entry; then top;
                state top { do action work { first start; then action mark assign log := log + "did "; then done; } }
                transition first top accept Stop do assign log := log + "stop " then idle;
                state idle; }
```

Derived constraints:

- A state's do behavior starts before the state's other middle steps start and is otherwise
  concurrent with them (`StatePerformances.kerml` `StatePerformance`, `succession do.startShot
  then nonDoMiddle.startShot`); the succession is on the do performance's start, not on its first
  action, so a do behavior that has begun and not yet performed its first action is a state the
  library admits while the machine dispatches.
- A transition's accept precedes its source's exit (`TransitionPerformances.kerml`
  `StateTransitionPerformance`, `accept then transitionLinkSource.exit`), and the exit ends the
  do behavior with the state (`succession [*] middle then [1] exit`): a dispatch that leaves the
  state cuts the do behavior off wherever it stands.
- No `HappensBefore` chain connects an action of the do behavior to the dispatch of an occurrence
  in the machine's pool, so the library orders nothing between the two.
- An occurrence no performance accepts is not a step of any performance: dropping it, or holding
  it deferred, moves nothing in the `StatePerformance`, so there is nothing to order against the
  do behavior's action — and the do behavior's next action may be the `accept` that takes it. The
  draw is between the do step and a dispatch that *takes* its occurrence: fires a transition, or
  lets a do behavior already parked at an `accept` go on.

Open: whether the do behavior's next action or the dispatch goes first, at every instant both are
due. In the fixture `Stop` is in the pool as `top` is entered, so `log` ends `did stop ` or
`stop `.

Pinned outcome: the admissible set `{did stop , stop }`, stated as `outcomes` citing this section.
Under `check`, `replay` and `explore` the order is a choice point reported as `choice at t=0.0:
next do top, dispatch accept Stop (unordered; took do top first)`: one move is one token move of
a state's do behavior — a statement of an inline body, a step of a do behavior given as an
action, a token inside a nested perform — drawn against the dispatch the machine would make now
(`do <state>` naming the due states, then `dispatch <event>`), and the draw is made again after
every move while a do behavior is due, so the dispatch may cut the flow anywhere or wait for it
to rest. A dispatch that would drop its occurrence is not drawn ahead
of a due do step; it waits until no do move is due, as under the fixed policies, so an occurrence a
do behavior is about to accept — `Tick` in `state_join_completion_segment_waits_for_do_behavior`,
`b1`'s timer in `state_join_completion_is_not_a_timers_expiry` — is not lost to the draw, and
those fixtures keep their admissible sets. `declared`, `reverse` and `seed:<n>` run the whole do
round — every due do behavior, each steppable token once — and dispatch after it, so their traces
record no such choice and end `did stop `; `explore` must reach both outcomes and no other, and
the fixed policies' run is always among the runs `check` tables. `state_do_step_cuts_typed_do`
makes the do behavior a typed action of two steps whose `inout` writes back as it ends: the
dispatch cuts it at either step (`count = 100`) or takes it after it ended (`111`).
`state_do_step_cuts_nested_perform` performs that action from an inline do body between two
assignments: `1000` (cut before the first), `1001` (after it, or inside the perform, whose
write-back is lost), `1012` (after the perform), `1112` (after the body ended).
`state_do_step_cuts_control_node_body` forks the do flow through a fork with a body of its own
(`fork split { assign count := count + 1; }`): a control node's body is performed by the token
passing through it, so it is a move the dispatch may fall before (`1000`) or after (`1001`, the
fixed policies' run, whose sweep moves each token once and so ends at the fork), then after
either branch (`1011`, `1101`) or both (`1111`) — five outcomes, exact under `check`. A control
node with no body only routes control, and where between two moves it falls no other move
observes, so it is not drawn.
`state_do_action_loop_timed_exit` loops a forked do flow through timed waits against a timed
exit due at the same instant: the exit may cut the flow before either branch writes, after one,
or after both — the fixed policies' `left = right = 1` — four outcomes, exact under `check`.
`state_do_step_among_completions` is the shape with two regions'
completion effects for the dispatch: a region's do step and the other region's completion are
each drawn at every instant both are due, and the order among the completions themselves is the
entry draw's, which the pool follows (§8.5.9) — the do step falls before, between or after the
two effects in either of their orders, six outcomes. `state_do_step_or_tied_dispatch` ties two time triggers at the instant the
do step is due, one guarded on what the step writes: each tied event is previewed on its own, so
the unguarded trigger alone is drawn against the step (`choice at t=2.0: next do top, dispatch
time top 2->idle`) and the guarded one, which the dispatch would drop before the step, waits for
the round to close, where the two are a dispatch order; `log` ends `did one `, `did two ` or
`two `, and `explore` reaches the three and no other. Were the tied events judged together, the
dropped one would hide the acting one behind the step and `two ` would be lost.

### A do step and a sibling region's entry due inside one entry: which goes first is open

Fixtures: `state_do_step_before_sibling_entry` (golden, explored, checked),
`state_do_step_before_sibling_entries` (golden, explored, checked),
`state_do_step_before_nested_entries` (golden, explored, checked),
`state_do_step_nested_before_outer_entry` (golden, explored, checked),
`state_do_step_before_fork_branch` (golden, explored, checked),
`state_do_step_before_history_restore` (golden, explored, checked),
`state_do_step_typed_before_sibling_entry` (golden, explored, checked),
`state_do_step_cut_by_sibling_completion` (golden, explored, checked),
`state_do_step_cut_by_sibling_terminate` (golden, explored, checked).

```
idle ─ accept Go → work parallel { left:  { entry; then l1 { do { log += "did " } } }
                                   right: { entry; then r1 { entry { log += "r1(entry) " } } } }
```

Derived constraints:

- Entering `work` starts one performance per region, concurrent with each other (SysML v2
  §7.18.1); within `left`, `l1`'s entry precedes its do behavior's start
  (`StatePerformances.kerml` `StatePerformance`, `succession [1] entry then [*] middle`, the do
  behavior a `middle` step whose start precedes the other middle steps' starts), so `l1(entry) <
  did`.
- No succession joins a step of `left`'s chain to a step of `right`'s (the previous section's
  derivation for the entries), and the do behavior's actions are steps of `left`'s chain: the
  library orders `did` after `l1`'s entry and against nothing in `right`. That `r1`'s entry is
  another unit of the same entry occurrence orders nothing — the do behavior has started and its
  next action is due as any other due action is.
- Every write appends to `log`, so `log` records the interleaving.

Open: whether the do behavior's next action or the sibling's remaining entry goes first, at every
draw of the entry front where both are left. The fixture's `log` ends `did r1(entry) ` or
`r1(entry) did `.

Pinned outcome: the admissible set `{did r1(entry) , r1(entry) did }`, stated as `outcomes` citing
this section. The step is a unit of its region's queue on the entry front: once the queue has
performed its entries — the entry unit that started the do behavior and, below a composite, its
substates' — each due token move of that behavior is drawn against
the sibling regions' remaining entry units under the front's own draw — the `entering <owner>`
choice, its alternative labeled `do <state>` beside the entries — for as long as a sibling has a
unit left; when none has, the remaining moves fall to the do-step site of the previous section,
drawn against the dispatch after the entry move settles. `declared`, `reverse` and `seed:<n>`
never take the alternative: they run the entries whole, as before, and the do round after the
move settles, so their traces record no such draw and end `r1(entry) did `; `check`, `replay`
and `explore` draw it at every unit and reach both outcomes and no other.
`state_do_step_before_sibling_entries` leaves two sibling regions' entries, `m1` and `r1`: the
step falls before, between or after them in either of their orders, six outcomes.
`state_do_step_before_nested_entries` makes the sibling's start state parallel: its regions'
entries `a1`, `b1` are units of the same front and the step is drawn against each while one is
left, six outcomes; `state_do_step_nested_before_outer_entry` puts the do behavior in that nested
state instead, drawn against the outer sibling's `l1` as against its own sibling's `b1`, six
outcomes. `state_do_step_before_fork_branch` reaches the regions through a fork: the target one
branch enters starts its do behavior, and the step is drawn against the other branch's effect and
its target's entry, three outcomes. `state_do_step_before_history_restore` restores two regions
through a deep history, one of them into a state with a do behavior: the restore is a front drawn
the same way, so the step falls before or after the other region's restored entry, on top of the
first occurrence's firing — where the step falls before the other region's entry, after it, or
not at all, the `Pause` already in the pool cutting it (the previous section's draw) — and the
exit's two orders: twelve `log` values.
`state_do_step_typed_before_sibling_entry` gives the do behavior as a typed `action def` of two
steps with an `inout` written back as it ends: each step is one move, drawn against the sibling's
entry while it is left, and the sibling's write lands before the write-back, which overwrites
it, or after both steps: two `count` values. `state_do_step_cut_by_sibling_completion` completes the
sibling's state into a transition that leaves the parallel state: the do behavior's two steps
are drawn against the sibling's entry and then, as the previous section has it, against the
completion's dispatch that cuts them off, six outcomes; `state_do_step_cut_by_sibling_terminate`
is PSSM *Terminate 002*'s shape, the sibling completing into a terminate that ends the machine,
and the do activity's first segment falls before the sibling's entry, after it, or never, its
second — beyond an accept the terminate leaves unfed — never; with the two entry orders, five
outcomes, PSSM's five admitted traces.

### A composite's own do step and its substates' entries due inside its entry: which goes first is open

Fixtures: `state_do_step_before_own_substate_entries` (golden, explored, checked),
`state_do_step_before_own_body_entry` (golden, explored, checked),
`state_do_step_way_down_before_fork_branch` (golden, explored, checked),
`state_do_step_machine_before_top_entries` (golden, explored, checked).

```
idle ─ accept Go → work parallel { do { log += "did " }
                                   left:  { entry; then l1 { entry { log += "l1(entry) " } } }
                                   right: { entry; then r1 { entry { log += "r1(entry) " } } } }
```

Derived constraints:

- `work`'s entry precedes its do behavior's start and its substates' entries alike
  (`StatePerformances.kerml` `StatePerformance`, `succession [1] entry then [*] middle`: the do
  behavior and the nested `StatePerformance`s are both `middle` steps), and PSSM §8.5.5 has the
  do activity start after the entry behavior and run concurrently with what follows it — so
  `work(entry) < did` and `work(entry) < l1(entry)`, `work(entry) < r1(entry)`.
- No succession orders the do behavior's actions against the nested performances' entries: they
  are concurrent `middle` steps of one `StatePerformance`, as the previous section has the
  regions' chains concurrent with each other.
- Every write appends to `log`, so `log` records the interleaving.

Open: whether the do behavior's next action or a remaining substate entry goes first, at every
draw where both are left. The fixture's `log` is `did ` before, between or after `l1(entry) ` and
`r1(entry) ` in either of their orders.

Pinned outcome: the six interleavings, stated as `outcomes` citing this section. The composite's
do behavior begins as its own entry unit ends, before its regions are entered, and its due token
move is drawn on the front entering them beside the regions' queues — the same `entering work`
choice, the alternative labeled `do work` — for as long as a region has a unit left.
`declared`, `reverse` and `seed:<n>` never take the alternative and end `l1(entry) r1(entry) did `
(`reverse`: `r1(entry) l1(entry) did `); `check`, `replay` and `explore` reach the six and no other.
`state_do_step_before_own_body_entry` gives the composite a serial body two states deep, whose
entries no front orders: each entry on the way down is drawn against the step at its own
`entering <owner>` choice, `did ` falling before `w1(entry) `, between it and `w2(entry) `, or
after both, three outcomes. `state_do_step_way_down_before_fork_branch` reaches the substates
through a fork: the first branch's way down enters the composite and starts its do behavior,
which is drawn against the branches' remaining target entries, six outcomes.
`state_do_step_machine_before_top_entries` is the same shape at the machine, whose do behavior
begins before its top regions are entered: six outcomes.

### A succession outside a behavior body orders the performances it relates, wherever they run

Fixtures: `namespace_succession_chain_ends` (golden), `type_succession_performed_actions`
(golden), `namespace_succession_qualified_ends`, `namespace_succession_explicit_start_violated`,
`namespace_succession_requirement_end`.

```
package D {
    part def Bot { perform action m { … n := n + 1 … }  perform action g { … n := n * 10 … } }
    part b : Bot;
    first b.g then b.m;                  -- owned by the package
}
part def Robot { perform action move; perform action grip; first grip then move; }   -- owned by the part def
first r::move then r::grip;              -- package-owned, ends named by qualified name
```

Derived constraints:

- A succession is a Connector typed by `HappensBefore`: KerML 1.0 §7.4.6.4 gives a succession
  with no explicit subsetting "a default subsetting to the feature happensBeforeLinks … it will
  implicitly have the type HappensBefore", and §8.3.4.5.4 `checkSuccessionSpecialization` (semantics, §8.4.4.6.3)
  requires it. SysML v2 §8.4.9.4 carries this to `SuccessionAsUsage`, "asserting that the Occurrence
  identified by its first end happens temporally before the one identified by its second end".
  `Occurrences.kerml` `HappensBefore` makes that "completely before": no snapshot of the earlier
  occurrence is at the same time as any snapshot of the later one, so the earlier one *ends*
  before the later one *starts*. Nothing in this depends on where the succession is owned.
- The links of a connector relate values of its related features in the context of each instance
  of its featuring type. KerML §8.3.4.5.3 `checkConnectorTypeFeaturing`: "Each relatedFeature of
  a Connector must have each featuringType of the Connector as a direct or indirect featuringType
  (where a Feature with no featuringType is treated as if the Classifier Base::Anything was its
  featuringType)".
- A succession owned by a type (`first grip then move` in `Robot`) has that type as its featuring
  type, so it constrains every `Robot`: on each, its `grip` performance ends before its `move`
  performance starts.
- A succession owned by a package has no owning type. KerML §8.3.4.5.3
  `deriveConnectorDefaultFeaturingType` makes its `defaultFeaturingType` "the innermost common
  direct or indirect featuringType of the relatedFeatures", and Table 11 note 2 (§8.4.4.1), with
  the prose of §8.4.4.6.1, lets an implied TypeFeaturing to that type be added "only if the Connector
  has no explicit owningType or ownedTypeFeaturings, and the defaultFeaturingType of the
  Connector is not null". So:
  - `first r::move then r::grip` relates `Robot::move` and `Robot::grip` (a qualified name names
    the member, not `r`'s value of it), whose innermost common featuring type is `Robot`: the
    succession is featured by `Robot` and constrains every `Robot`, not only `r`.
  - `first part1::action1 then requirement1` relates `part1::action1`, featured by `part1`, and
    `requirement1`, featured by nothing and so (per `isFeaturedWithin`, §8.3.3.3.4) within every
    type; the succession is featured by `part1`.
  - `first b.g then b.m` relates two feature chains whose first chaining feature `b` is a package
    member; their featuring type is `Base::Anything`, and the one link relates the `g` and `m`
    performances of the object `b` denotes.
  - Ends with no common featuring type (`first p1::a then p2::b` with `p1`, `p2` two package-level
    parts) leave `defaultFeaturingType` null, no implied TypeFeaturing may be added, and
    `checkConnectorTypeFeaturing` fails: the model is ill-formed.
- An object's performed actions and exhibited states are its `Parts::performedActions` and
  `exhibitedStates` (`ref action performedActions: Action[0..*] :> actions,
  enactedPerformances`). These are the occurrences a succession between them orders.
- When an end has more than one performance per featuring instance, or none, how many links the
  succession requires is the multiplicity of its ends, which is unresolved where unwritten
  (OMG issue [KERML-29](https://issues.omg.org/issues/KERML-29), deferred). The rule this record
  already applies to action steps applies here too: an unwritten end is read both as `[0..*]` and
  as `[1..1]`. The order is enforced only when both readings force it: exactly one performance of
  each end per featuring instance. Otherwise it is refused as open.

Where the specification is silent: neither KerML nor SysML defines an executor. A succession is a
necessary condition on a run's occurrences. Nothing in either specification says when an
object's performed actions start, or whether a tool must realize a succession or only check it.
`Parts::performedActions` is a referential `[0..*]` feature and fixes no start. UML, fUML and
PSSM say nothing about successions between the classifier behaviors of distinct objects either,
and are advisory only.

Policy this implementation takes where the specification is silent:

- **Realized where the tool chooses the start.** The executor itself starts the behaviors an
  object's type performs or exhibits, when the object is materialized. Where it chooses the start
  time, it must choose a conforming schedule rather than produce a violation. A behavior that is
  the later end of an applicable succession is held, not started, until every earlier-end
  performance in the same featuring instance has ended. Holding constrains the scheduler and
  fixes no single order. Performances no succession relates keep every interleaving the executor
  admitted before, and `-schedule explore` and the check still enumerate them. A held behavior,
  and what it waits for, is part of the run's state, so a snapshot, a held image and the checked
  state key all carry it.
- **Checked where the model fixes the start.** An explicit `perform x.beh.start`, or an
  `-action` performance by an object of a behavior its type declares, starts when the model says.
  If that start would put an earlier-end performance that has not ended before, or overlapping,
  a later-end performance in the same featuring instance, the run fails with a typed
  `succession-order-violated` error. It is never silently reordered.
- **Typed errors for what no schedule satisfies.** A cycle of successions among the behaviors of
  one featuring instance (`first a then b; first b then a;`) has no conforming run. The run fails
  with `succession-order-cycle`, and validation warns with the same code.
- **Reported when it constrains nothing.** A succession one of whose ends is a behavior an object
  performs, but which orders nothing in a run, is reported with `succession-orders-nothing` and
  the reason. It is never dropped silently:
  - its other end is not a behavior an object performs, such as `requirement1`, whose
    evaluations the runtime checks as verdicts and does not place in the run's occurrence order;
  - an end's performance is absent when the other starts, so whether one is required is open
    under KERML-29;
  - an end has more than one performance in one featuring instance, so the pairing is open.

  Validation reports the reasons it can see statically, with the same code. The run reports
  them as notes. A succession neither of whose ends is such a behavior gets neither, because
  execution has nothing to order. Examples are the successions between events, flows and
  messages of an interaction, and those the library's `Flows::Message` declares between
  `sourceEvent`, `self` and `targetEvent`. They are constraints on occurrences the run does not
  enact as object behaviors.

Pinned outcome: in `namespace_succession_chain_ends`, `b.n = 1`: `g` (`n := n * 10` over 0)
ends before `m` (`n := n + 1`) starts. Before this, the run started `m` first and ended with
`n = 10`. In `type_succession_performed_actions` and `namespace_succession_qualified_ends`,
every `Robot` performs `grip` before `move`, including a `Robot` the succession does not name.
In `namespace_succession_explicit_start_violated`, the explicit start of the later end before
the earlier end has ended fails with `succession-order-violated`.

Not covered: a requirement, constraint or calculation end. Its evaluation is a model-level
verdict, not an occurrence of the run, so the succession is reported as ordering nothing. Also
not covered: the package-level `connect q::a to q::b;` form. Validation rejects it with
`Must be an accessible feature (use dot notation for nesting)`, although
`deriveConnectorDefaultFeaturingType` makes it well-formed. That is a separate validation gap,
and this change does not touch it.

## What the executor gets wrong

Nothing, at present: every derivation above is met and carries a golden. The table this section
held is empty and so omitted; `internal/exec/runtime/testdata/conformance/known_failures.txt` is
kept with only its header comments, because the harness reads it and because it is where the
next unmet derivation goes (see [Adding a case](#adding-a-case)).

The one entry it held, `action_merge_loop_reenters`, was met by removing the executor's
first-traversal record from `stepMergeNode` so that a merge passes every arriving token; the
case's expected outcome was not touched. The same change moved the merge's body ahead of its
outgoing guard, as every other node kind already had it, which is the one existing expectation
that moved: `f63_merge_body_runs_on_traversal` went from `mergeRuns = 1`, `passed = 1` (an
arrival whose guard was false skipped the body) to `mergeRuns = 2`, `passed = 2`, per the
derivation above. When a new gap is found, list the case there, record it
in a table here (case, derived, executor, root), and when the fix lands remove the entry, run
`-update-traces` for the case, and review the new golden against its derivation before
committing it.

## Adding a case

1. Write the smallest model that makes the rule observable through feature values, not only
   through trace order — a value the harness can compare is what the `.expected.json` holds,
   and what survives a change in tool-defined scheduling.
2. Derive the constraints and the outcome from the library text *before* running the executor,
   and record them here with the sentence relied on. Say which orderings are open.
3. Put the derived outcome in the `.expected.json` with `"trace": true`. Run the conformance
   test. If it passes, run `-update-traces` for the case and check that the golden is one of the
   admissible linearizations. If it fails, add the case to `known_failures.txt` with a one-line
   reason and add it to the table above; do not adjust the expectation to the executor.
4. Cite the case from the compliance row it refers to, leaving the row's status as the executor
   earns it — a golden that pins an approximation does not make the approximation faithful.
