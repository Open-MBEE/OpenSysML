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
No executor code was changed to build this corpus; that is the point of it.

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
(step 5) before `after` runs. The case is observable only because a node upstream is reached
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
arrivals collapse into (golden, step 3). A join differs only in what it awaits: every incoming
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
`x = 1`) and exists only so a change in that scheduling is noticed. The compliance row stays
approximate because the runtime still picks an order the specification does not, and reports no
conflict.

### A merge is re-entered on every traversal of a loop

Fixture: `action_merge_loop_reenters` (**known failure**), after the specification's
`ChargeBattery`.

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

Executor: `level = 50`, `passes = 1`, and `endCharging` is never performed. `stepMergeNode`
records the merge in `mergeVisited` on its first traversal and retires every later token that
reaches it, so the second arrival — the one carrying the loop — is discarded and the action
completes, without error, having never reached `endCharging` or `done`.

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

One of the six derivations is not met. It is listed in
`internal/core/runtime/testdata/conformance/known_failures.txt`, its expected outcome is the
derived one, and the compliance map cites it from the row it refutes.

| Case | Derived | Executor | Root |
|------|---------|----------|------|
| `action_merge_loop_reenters` | every arrival traverses the merge | the first traversal closes the merge for the run | `stepMergeNode` keys `mergeVisited` on the merge node alone |

When a fix lands, remove the entry from `known_failures.txt`, run `-update-traces` for the
case, and review the new golden against the derivation above before committing it.

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
