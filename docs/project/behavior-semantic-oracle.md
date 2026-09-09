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
`internal/core/runtime/testdata/conformance/` and run through the same harness
(`go test -run 'TestExecutionConformance|TestExecutionTrace' ./internal/core/runtime`). Where the
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
hence happening during the state performance"). A feature with no declared multiplicity holds
exactly one value (KerML 1.0 §7.4.5, the assumed `1..1`, the rule the compliance map applies to
attributes), so a step such as `action left;` names one performance per performance of its owner
unless the step declares otherwise. Nothing in the library orders two steps no chain of
`HappensBefore` links connects.

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
| `Actions.sysml` `ControlAction` | `bind start = done` — "A ControlAction is instantaneous" | A control node adds no duration; its successor may start as soon as its predecessors end |
| `Actions.sysml` `ForkAction` | "Fork behavior results from requiring that the target multiplicity of all outgoing succession connectors be 1..1" | Each fork performance is followed by exactly one performance of every target |
| `Actions.sysml` `JoinAction` | "Join behavior results from requiring that the source multiplicity of all incoming succession connectors be 1..1" | Each join performance follows exactly one performance of every source, one per incoming succession |
| `Actions.sysml` `MergeAction`, `ControlPerformances.kerml` `MergePerformance` | "Incoming succession connectors to a MergeAction must have source multiplicity 0..1"; "For each instance of MergePerformance, the incomingHBLink is an instance of exactly one of the Successions, ordering the MergePerformance as happening after an instance of the source of that Succession" | A merge performance follows one source performance; a source a given merge performance was not reached from need not exist |
| `Actions.sysml` `DecisionAction`, `ControlPerformances.kerml` `DecisionPerformance` | "For each instance of DecisionPerformance, the outgoingHBLink is an instance of exactly one of the Successions, ordering the DecisionPerformance as happening before an instance of the target of that Succession" | Each decision performance is followed by a performance of the one target whose guard held |
| KerML 1.0 §7.4.5 | A feature with no declared multiplicity holds exactly one value | A plain step is one performance per performance of its owner, however many successions reach it |
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
outcome the derivation fixes.

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
first because its branch is shortest) is one admissible linearization.

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

Open: the order of `left` against any node of the `r` branch.

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

### A node reached over two successions is performed once, after both

Fixture: `action_node_with_two_incoming_successions_runs_once` (golden).

```
start → split ⇉ l1 ──┐
              ⇉ l2 ──┤→ both → done
```

Derived constraints:

- `both` is one performance of `converge` (KerML §7.4.5: the step declares no multiplicity).
- Each of `first l1 then both` and `first l2 then both` is a `HappensBefore` link whose
  `laterOccurrence` is that one performance, so it starts after both `l1` and `l2` have ended.
  This is the reading the pilot corpus states in prose for `engineStopped`, which five
  successions reach.

Open: the order of `l1` against `l2`.

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
trace must satisfy. The exact trace golden stays: it records the executor's scheduling (`right`
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
section. The executor evaluates every guard, takes the first declared, and records the choice
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
The first pick among three tokens and the next among the two left are two choice points of one
step, so a linearization is a sequence of two choices, not one choice among six.

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
this section. The partial order the library fixes among the nodes is stated as `.trace.order`
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
citing this section. The executor examines every transition out of the state for the event,
fires the first declared, and records the choice (`choice state idle on accept Go: transitions
1->low, 2->high (unordered; took 1->low)`); the golden pins that linearization. As for a decision,
the transitions after the first enabled one are read in a preview that is undone, and one whose
guard cannot be evaluated is not an alternative and does not fail the dispatch: it is recorded as
an informational `guard-unevaluable` note naming the state, the event and the transition
(`TestLaterGuardErrorIsNotAChoiceNorAFailure`; fixture `state_choice_unevaluable_transition`,
golden). The first transition read is the run's own, and its failure fails the dispatch as it
always has (`TestFirstTransitionFailureStillFailsTheRun`).

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

### Transitions in sibling regions enabled by one event: each fires, in which order is open

Fixture: `state_explore_region_order` (golden, explored).

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
stated as `outcomes` citing this section. The executor fires the selected transitions in region
declaration order under `declared`, `reverse` and `seed:<n>` alike — a tool-defined order it does
not report as a choice, so the trace under those policies carries no `choice` line for it — and
the golden pins that linearization (`a` first, `last = 2`). Only `explore` varies the order: it is
a choice point of the exploring run (`choice on accept Go: states a1, b1 react (unordered; took
b1 first)` in the witness of the second outcome), and exploration must reach both outcomes and no
other, in two runs. The existing fixtures `state_call_trigger_regions`,
`state_composite_region_depth_order`, `state_composite_region_deeper_first` and
`state_parallel_broadcast` pin the declaration-order linearization of this same shape as their
one expected outcome and, having no `outcomes`, are not explored by the harness; under `explore`
each reaches a second outcome.

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

Open: nothing that is observable; the loop is a chain.

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

Open: nothing observable.

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

Open: the interleaving of the two tokens at every node; which token takes which exit.

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

Open: nothing observable; the transition is a chain.

Fixed outcome: `x = 0`, `y = 0`, `z = 1`, final state `finished`. The executor agrees; the golden
shows the guard reading `x -> 5`, then `exit: active`, then `assign y` reading `x -> 0`, then
`enter: finished` with `assign z` reading `y -> 0`. The guard appears twice in the golden; that is
the tool detail noted above, not a second reading the library asks for.

## What the executor gets wrong

Nothing, at present: every derivation above is met and carries a golden. The table this section
held is empty and so omitted; `internal/core/runtime/testdata/conformance/known_failures.txt` is
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
