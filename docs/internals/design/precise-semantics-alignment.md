# Alignment with the UML precise-semantics specifications

Whether, and how far, the runtime should align with the OMG precise-semantics family that
SysML v1 execution tools ran under: **fUML** (the executable subset of UML activities and
classes), **PSCS** (composite structures: parts, ports, connectors) and **PSSM** (state
machines, which ships a conformance test suite of UML models with expected traces). The runtime's
position is stated in [the architecture](../architecture.md) (the runtime section's *Spec Alignment* paragraph): the SysML v2
metamodel and the bundled KerML semantic library govern execution, UML 2.5.1 is the fallback only
where v2 has no production and the library no performance, and the runtime is not a UML or fUML
activity engine. This note tests that position against the three specifications clause by
clause, counts where a port could change behavior, assesses the PSSM test suite as a referee, and
recommends what to do. It changes nothing; implementation, if any, is a later decision.

Written for a maintainer who has read neither the precise-semantics specifications nor the state
executor: each row quotes or paraphrases the clause, then the library, then the code, then gives
a verdict. Every runtime claim names a file and function and the test or fixture that pins it;
every specification claim names a clause read in the pinned document.

## Sources, pinned

### The precise-semantics specifications

| Specification | Version, OMG document | Read | Machine-readable artifacts |
|---|---|---|---|
| **PSSM** — Precise Semantics of UML State Machines | 1.0, `formal/19-05-01`, <https://www.omg.org/spec/PSSM/1.0> | Clause 2 (conformance), Clause 6 (scope of the subset), §8.5.4 state-machine configuration, §8.5.5 state activations, §8.5.6 do-activity, §8.5.7 pseudostates, §8.5.8 transition activations, §8.5.9 event pool and deferral, §8.5.10 run-to-completion, Clause 9 (test suite: 9.1–9.3 architecture, 9.4 test cases) | `ptc/18-11-04` syntax (`PSSM_Syntax.xmi`), `ptc/18-11-05` semantics (`PSSM_Semantics.xmi`), `ptc/18-11-06` test suite (`PSSM_TestSuite.xmi`), all linked from the specification page |
| **fUML** — Semantics of a Foundational Subset for Executable UML Models | 1.5, `formal/21-03-01`, <https://www.omg.org/spec/FUML/1.5> | §2.3 conformance and semantic variation points, §7.3 the excluded features (`TimeEvent`, `ChangeEvent`, interruptible regions), §8.8.1 common behaviors (execution, object activation, event pool, `GetNextEventStrategy`), §8.9.1 activities (tokens, offers, node activations), §8.9.2 actions (`ChoiceStrategy`, `FirstChoiceStrategy`, decision, fork, join, merge, expansion regions, structured nodes, accept-event, send-signal, call, start-object-behavior, destroy) | `ptc/20-05-12`, `ptc/20-05-13`, `ptc/20-05-14` |
| **PSCS** — Precise Semantics of UML Composite Structures | 1.2, `formal/19-02-01`, <https://www.omg.org/spec/PSCS/1.2> | §8.1 overview, §8.4 structured classifiers and `CS_RequestPropagationStrategy` / `CS_DefaultRequestPropagationStrategy`, §8.6 `CS_DefaultConstructStrategy`, `CS_Object` (`sendIn`/`sendOut`/`dispatchIn`/`dispatchOut`), `CS_Link`, delegation and assembly connectors, destruction of parts and links | `ptc/18-08-04` through `ptc/18-08-09` |

Versions and document numbers are those the OMG specification pages listed at the time of
writing; the PDFs and the XMI files were fetched from those pages and read, not recalled. The
spec-numbering convention here is the document's own (PSSM `8.5.9`, fUML `8.8.1`).

### The specifications the runtime already follows

| Specification | Version, OMG document | Clauses this note relies on |
|---|---|---|
| **SysML v2** | 2.0, `formal/26-03-02`, <https://www.omg.org/spec/SysML/2.0> | §7.18.1–§7.18.3 states and transitions (entry/do/exit, `parallel`, transition steps, `done`, a transition to `terminate`); §8.4.13.4 control nodes (`ForkNode`, `JoinNode`, `MergeNode`, `DecisionNode`); §8.4.13.5–§8.4.13.6 send and accept actions; §8.4.13.8 terminate action |
| **KerML** | 1.0, `formal/26-03-01`, <https://www.omg.org/spec/KerML/1.0> | §9.2.11.1 `StatePerformances`, §9.2.12 `TransitionPerformances`, §9.2.5 `Occurrences` (`isDispatch`, `dispatchScope`, `isRunToCompletion`, `runToCompletionScope`, `incomingTransferSort`), §9.2.10 `ControlPerformances`, §9.2.13 `Clocks` |
| **Bundled semantic library** | the pilot implementation's library snapshot shipped under `internal/workspace/libs/stdlib` (EPL-2.0, see its `NOTICE`) | `Kernel Semantic Library/Occurrences.kerml`, `StatePerformances.kerml`, `TransitionPerformances.kerml`, `ControlPerformances.kerml`, `Clocks.kerml` — the text the runtime executes against, quoted where it differs in wording from the specification PDF |

The record of which pilot release and which OMG documents the conformance work pins is
[spec-compliance](../../project/spec-compliance.md); this note does not re-pin them.

### The PSSM test suite

- **Where.** Published beside the specification as `ptc/18-11-06`, a UML 2.5 XMI file
  (`PSSM_TestSuite.xmi`), linked from <https://www.omg.org/spec/PSSM/1.0>. Clause 9 of the PDF
  describes it and reproduces the model of every test with its expected trace.
- **Licence.** The specification's front matter grants a limited licence to use, copy and
  distribute the specification (of which the suite is a machine-readable part) subject to
  retaining its notices, for informational use, without modification and without commercial
  resale or transfer. The suite is **not vendored, copied or committed** to this repository, and
  no fixture here is derived from it; the two illustrative translations in this note are written
  by hand from the PDF's figures and are examples in prose, not fixtures.
- **Form.** Clause 9.1 describes the suite as itself a fUML/PSCS/PSSM-conformant model. Each
  test is a UML class specializing the abstract `SemanticTest` (its `SemanticTestSuite` owner
  registers the concrete tests), with a `Tester` object that sends a `Start` signal to a
  `Target` object whose classifier behavior is the state machine under test; every entry, exit,
  effect and do-activity body calls a `trace(String)` operation that appends a segment to a
  `TraceBuilder`, and the test compares the resulting `::`-separated string
  (`S(entry)::S(doActivity)::T(effect)`, in the PDF's own notation) with the expected trace(s).
  A test may declare several expected traces where PSSM admits alternative valid orderings
  (orthogonal regions, do-activity interleaving); Clause 9.3 describes each test with the
  requirement it covers, its model, its expected result and the run-to-completion steps that
  produce it, and Clause 9.4 maps the requirements of Clause 8 to the tests covering them. Clause
  9.1 states the design: each test verifies one requirement extracted from UML Clause 14 "as
  formally interpreted according to the semantic model defined in Clause 8".
- **Count.** Traversing `PSSM_TestSuite.xmi` for concrete `uml:Class` elements whose direct
  generalization is `SemanticTest` gives **103** tests, grouped by the package they live in:

  | Area | Tests | Area | Tests |
  |---|---|---|---|
  | Behavior | 5 | Terminate | 3 |
  | Transition | 15 | Final | 1 |
  | Event | 16 | History | 8 |
  | Entering | 5 | Deferred | 10 |
  | Exiting | 5 | Redefinition | 6 |
  | Entry (entry points) | 6 | Standalone (a state machine that is itself the active test target, not a classifier behavior) | 3 |
  | Exit (exit points) | 3 | Junction | 6 |
  | Choice | 5 | Other (UML Figure 14.2, the transition execution algorithm example) | 1 |
  | Fork | 2 | | |
  | Join | 3 | **Total** | **103** |

  Of the 103, 67 declare one expected trace, 22 declare two, 5 three, and the remaining 9 declare
  between four and 84 alternatives (the largest are the orthogonal-region and do-activity
  interleaving tests). The XMI's packages are the PDF's test categories; the PDF's coverage
  clause (9.4) groups the entry-point and exit-point categories under "Encapsulated".

## The semantic map

Each row: what the precise-semantics specification says; what SysML v2 and the KerML library say;
what the runtime does today, with the file and function and the test or fixture pinning it; one
of four verdicts.

- **agrees** — PSSM (or fUML, PSCS) and the runtime's v2-grounded behavior coincide.
- **differs because v2 differs** — the v2 library or notation specifies otherwise; both texts
  are quoted.
- **differs, v2 silent** — v2 gives no rule; the runtime's behavior is a tool choice and PSSM
  offers one.
- **gap** — a concept the runtime has no behavior for at all.

Test names are conformance fixtures under `internal/exec/runtime/testdata/conformance/` (run by
`TestExecutionConformance`; a `.trace.golden` beside one is compared by `TestExecutionTrace`),
subtests of `TestRuntimeRobustness` in `internal/exec/runtime/robustness_test.go`, or unit tests
in the runtime package. File paths are relative to `internal/exec/runtime/` unless stated.

### State machines (PSSM)

Rows are numbered SM1–SM45 so the count at the end of the map can be checked against them.

#### The event pool and the run-to-completion step

**SM1. One event pool per active object; reception is decoupled from dispatch.**
PSSM §8.4 builds on fUML §8.8.1: an active object has an object activation whose event pool
receives event occurrences asynchronously; one occurrence is dispatched at a time; an occurrence
no accepter matches is lost. PSSM redefines the pool as `SM_ObjectActivation` so that it can also
hold `CompletionEventOccurrence`s and a separate `deferredEventPool` (§8.4). *v2/KerML:*
`Occurrences.kerml` gives every occurrence `incomingTransfersToSelf` and an
`incomingTransferSort` that "determines which transfer to accept when multiple are available and
which of the unaccepted transfers are never to be accepted (dispatched)"; `StatePerformances.kerml`
gives a `StatePerformance` `acceptable`, `accepted [0..1]` and `deferrable`. *Runtime:* each
`StateExecutor` owns an `eventQueue` (a heap, `executor_common.go:eventHeap`) fed by
`SendSignal` and by the object's message bus, and `state_executor.go:runStep` dequeues one
occurrence per step; an occurrence no active state accepts or defers is dropped
(`state_undeferred_event`, `TestUndeferredEventIsDroppedWhereNoTransitionHandlesIt` in
`state_deferred_test.go`). Delivery from a sibling behavior to a waiting machine:
`classifier_behavior_test.go:TestStateDoBehaviorAwaitingAMessageIsWokenByASibling`. **agrees.**

**SM2. The run-to-completion step.** PSSM §8.5.4: the state-machine configuration "changes during a
run-to-completion step initiated by an event occurrence" and does not evolve between steps; a step
ends when every transition it triggered has been traversed and every entry behavior it invoked
has completed (UML 14.2.3.4.2, cited there). *v2/KerML:* `Occurrences.kerml`
`isRunToCompletion: Boolean[1] default true` — "determines whether transition performances might
happen during state entry performances within the run to completion scope" — with the
`StatePerformances.kerml` invariant `isRunToCompletion implies
allSubtransitionPerformances(runToCompletionScope)->forAll{in tp; includes(tp.successors, entry) |
includes(tp.predecessors, entry)}`: under the default, no transition of the scope overlaps a
state entry. *Runtime:* `runStep` dispatches one occurrence and fires the transitions it selects to
their end, entry behaviors included, before the next occurrence is looked at
(`state_executor.go:enterStateInto` runs the entry body synchronously); the resulting completion
transitions are *queued* (SM9) and dispatched by a later step, never inside the entry.
`state_composite_orthogonal_exit` and its trace golden pin one whole step; `state_completion_done`
pins that the completion step follows the entering step. The lowered graph carries effective `isRunToCompletion` and `runToCompletionScope` values;
`state_run_to_completion.go:holdEntry` and `entryStep` split entry cascades at scoped boundaries,
while free dispatch alternatives remain available at the same instant. The default retains one
whole-machine step, and the scoped fixtures cover both admissible orders. **agrees.**

**SM3. One occurrence, one dispatch, possibly several transitions.** PSSM §8.5.2 (`select`) builds
"the set of transitions that can be fired using the proposed event occurrence"; UML 14.2.3.9.4
fires the maximal set of non-conflicting enabled transitions for one occurrence, one per
orthogonal region at most. *v2/KerML:* `Occurrences.kerml` has `isDispatch: Boolean[1] default
false` — "determines whether transfers to the dispatch scope might be accepted more than once" —
but `Performances.kerml` redefines `isDispatch default true` and `dispatchScope default
thisPerformance` for every performance, so a state performance's dispatch scope is itself and the
`StatePerformance` invariant ranges over its own nested state performances: when one of them has
accepted the same transfer this performance finds acceptable, this performance must exit first
(`includes(thatSP.exit.startShot.successors, oSP.exit.startShot)`), or the nested one must have
accepted an earlier-sorted transfer or hold this one as `deferrable`. Two states in sibling
regions are each their own scope with no nested state performances, so nothing in the library
stops both from accepting one transfer. *Runtime:* `state_executor.go:broadcastEvent` selects one
candidate per active leaf (`selectTransitions`), removes those a nested transition outranks
(`losesToNestedTransition`), and fires the survivors one at a time (`dispatchInOrder`); a
composite and one of its substates never both take one occurrence (the nested one wins, SM18),
which satisfies the invariant, and sibling regions each take it, which the invariant leaves free.
The runtime does not read a redefinition of `isDispatch` or `dispatchScope`.
`state_parallel_broadcast`, `state_explore_region_order`. **agrees.**

#### Event dispatch order and deferred events

**SM4. Dispatch order within the pool.** fUML §8.8.1 makes the dispatching strategy a variation point
with FIFO as the default; PSSM §8.4 orders completion events before every other occurrence in the
pool (SM10) and released deferred events after the completion events but before the rest (SM6).
*v2/KerML:* `incomingTransferSort` defaults to `earlierFirstIncomingTransferSort` — "`t1First =
includes(t1.endShot.successors, t2.endShot)`", the transfer that ended earlier is accepted first.
*Runtime:* `executor_common.go:eventHeap.Less` orders by timestamp, then completion before ordinary
at one timestamp, then by arrival ID; because a run drains the current instant before the clock
moves (`runCounting`, SM41), ties on the timestamp are the rule and the order is completion first,
then arrival. `TestDeferredEventsKeepTheirArrivalOrder`, `TestRecalledEventPrecedesLaterArrivals`
(`state_deferred_test.go`), `signal_injection_test.go:TestRunToCompletionTakesAPendingSignalBeforeALaterTimer`.
No fixture on `develop` isolates the completion-before-signal tie; `eventHeap.Less` is the
whole rule and [bounded model checking](bounded-model-checking.md) records it as the queue's
invariant. **agrees.**

**SM5. Deferring an occurrence.** PSSM §8.5.5 ("StateActivation and deferred events"): a state
activation defers an occurrence when some active state declares a `deferrableTrigger` matching it
*and* "there is no Transition with a higher priority and able to react to the EventOccurrence in
the active StateMachineConfiguration"; the occurrence is moved to the `deferredEventPool` and
returns to the regular pool when the deferring state leaves the configuration. *v2/KerML:* SysML v2
§7.18 has no production for deferral (the grammar page of this project records `defer` as an
extension); the library has `StatePerformance::deferrable: Transfer[0..*] subsets acceptable` —
transfers that "can be considered for acceptance more than once" — and the `isDispatch`
invariant's `includes(oSP.deferrable, accableT)` clause, and says nothing about priority between
a deferral and a transition. *Runtime:* `state_executor.go:dispatchEvent` defers only when no
transition consumed the occurrence and nothing resumed on it (`!consumed && len(resumed) == 0 &&
e.defersEvent(&event)`); `defersEvent` checks the active states and their ancestors for a matching
`Deferred` trigger; `recallDeferredEvents` returns an occurrence to the queue once no active state
defers it. `state_deferred_event`, `TestDeferredEventIsDeliveredAfterLeavingTheDeferringState`,
`TestCompositeStateDefersForItsSubstates`, `TestDeferralSpansOrthogonalRegions`,
`TestEventBlockedByAGuardIsStillDeferred` (a guard that fails does not consume, so the occurrence
is deferred). **agrees** on the mechanism;
the priority rule is SM7.

**SM6. Order of released deferred events.** PSSM §8.4: released occurrences "are placed in that pool
after all existing CompletionEventOccurrences, but before any other EventOccurrence already in the
pool, in the order in which the DeferredEventOccurrences had in the deferredEventPool". *v2/KerML:*
silent beyond `earlierFirstIncomingTransferSort`, which ranks a recalled transfer by its original
end, i.e. ahead of later arrivals. *Runtime:* `recallDeferredEvents` re-stamps a recalled
occurrence with the current clock time but keeps its arrival ID, so `eventHeap.Less` places it
behind completion events at that instant and ahead of every later arrival, in its original order.
`TestRecalledEventPrecedesLaterArrivals`, `TestDeferredEventsKeepTheirArrivalOrder`. **agrees.**

**SM7. Deferral against a transition elsewhere in the configuration.** PSSM §8.5.5 condition 2
above, read with §8.5.2 ("transition priorities, which are relative to the level of nesting of
their source states"): only a transition of *higher* priority than the deferring state — one
whose source is that state or nested in it — overrides the deferral. *Deferred 003* (§9.3.16.4)
shows the override: a composite state defers `Continue`, its substate has a transition on
`Continue`, and "this transition has priority over the deferring constraint added by [the
composite] since it is more deeply nested". *Deferred 004-A* (§9.3.16.5) shows the other direction
at equal depth: with one region's state deferring `Continue` and the sibling region's state holding
a transition on `Continue`, the occurrence "is deferred by" the former and the sibling's transition
fires only once the deferred occurrence is released. *Deferred 004-B* (§9.3.16.6) shows it with
the deferring state deeper than the sibling region's transition.
*v2/KerML:* silent — no notation, and `deferrable` carries no rank against `accepted` of another
state. *Runtime:* `selectTransitions` walks each active leaf's parent chain to the innermost state
with an enabled transition without consulting `Deferred` on the way, and `dispatchEvent` tests
`defersEvent` only when nothing consumed the occurrence; so any enabled transition anywhere in the
configuration — in an enclosing state or in a sibling region — consumes the occurrence and nothing
is deferred. The sibling-region case is pinned by
`TestEventConsumedByAnotherRegionIsNotDeferred` ("deferral only retains what the active
configuration leaves unhandled"), whose outcome is the opposite of *Deferred 004-A*'s; the
enclosing-state case has no fixture on `develop` and follows from the two functions cited. The
nested-transition override agrees with *Deferred 003*. **differs, v2 silent.**
*Decided:* PSSM's rule. A state of the active configuration that defers the dispatched
occurrence holds it back from every enabled transition except one sourced by that state or by a
state nested in it; with several deferring states (one per orthogonal region, say), *each* must
be overridden by such a transition for the occurrence to fire at all, otherwise it is deferred
whole — PSSM's condition applied per deferring state, as the reference implementation's
`isDeferred` does; a do behavior's `accept` still takes the occurrence ahead of the deferral
(SM15). The deferral is determined, not a choice point. A held occurrence is released, with its
arrival ID and the current clock time, to whatever configuration the exit of the deferring state
leaves (SM6). Implemented in `state_executor.go:deferralOutranks` (from `selectTransitions`,
which `dispatchEvent` and `Decide` share, over `deferringStates` and `encloses`); pinned
by `state_deferral_outranks_sibling_region` (*Deferred 004-A*),
`state_deferral_outranks_enclosing_state`, `state_deferral_nested_override` (*Deferred 003*),
`state_deferral_nested_outranks_sibling_region` (*Deferred 004-B*),
`TestDeferralOutranksASiblingRegionsTransition`, `TestDeferralOutranksAnEnclosingStatesTransition`,
`TestTransitionNestedInTheDeferringStateOverridesDeferral`,
`TestDeferralInEachRegionMustBeOverriddenForTheEventToFire` and
`TestDeferredEventIsDeliveredAfterLeavingTheDeferringState` in `state_deferred_test.go`.

#### Completion events and completion transitions

**SM8. When a state completes.** PSSM §8.5.5: a state generates a completion event when its entry
behavior has finished, its do activity (if any) has finished, and, for a composite state, every
region has reached a final state. *v2/KerML:* SysML v2 §7.18.3 knows no completion event: a
transition usage with no accepter is triggered when its guard holds "during a performance of its
source", with the do action interrupted if still running; KerML orders a transition only against
its own source's `exit`. The runtime's rule is therefore UML's, adopted where v2 leaves the
instant open. *Runtime:* a completion transition is a `lower.Transition` with a nil trigger;
`state_executor.go:scheduleCompletionTransitions` skips a state whose do action is still running
(`hasRunningDoAction`), and `settleDoActions` schedules it when the do action ends; a composite
completes only when every region has (`state_completion_all_regions`,
`state_parallel_completion`, `TestOrthogonalMachineCompletesOnlyWhenEveryRegionDoes`,
`TestNestedOrthogonalRegionsCompleteOnlyWhenEveryRegionDoes` in `state_completion_test.go`;
`TestCompletionWaitsForTheDoBehavior` in `state_do_activity_test.go`). **agrees.**

**SM9. Completion as an occurrence in the pool, dispatched by its own step.** PSSM §8.5.9: the
completion event is a `CompletionEventOccurrence` placed in the pool and dispatched by a
subsequent RTC step; it is generated whether or not a completion transition exists, its guards
are evaluated when it is dispatched, and it is "lost" when no transition is enabled then
(*Deferred 004-A*'s note: "the completion event is lost since [the completed substate] has no
completion transition"). *v2/KerML:* §7.18.3 "a transition usage can only be triggered during a
performance of its source" and, with no accepter, "if the guard expression evaluates to true";
§8.4.13.3 `StateTransitionAction`: "the guard ... is evaluated during the performance of its
source StateAction" — not at a fixed instant of it. The library has no completion occurrence.
*Runtime:* `scheduleCompletionTransitions` runs when a leaf is entered (`scheduleFromLeaf`) and
again when its do action ends (`settleDoActions`); it pushes one `EventTime` event with a nil
trigger at the current clock time *per completion transition whose guard holds then*, and
nothing when none holds; `dispatchEvent` fires it in a later `runStep`, re-evaluating the guard
(`passesGuard` in `fireTransition`). Where nothing changes between completion and dispatch the two
traces are identical — a lost completion event and an unscheduled one look alike, and the
re-evaluation drops an event whose guard turned false. `state_completion_absent` pins the
no-transition case, `state_completion_through_pseudostate` a completion routed through a
junction, `state_completion_done` the step boundary. The residual case is a guard false at
completion and true before the dispatch step — a sibling region's completion transition,
dispatched first, writing the machine's data: PSSM fires it (the guard is read at dispatch), the
v2 text admits it (the guard held during the source performance), the runtime never scheduled it.
No fixture on `develop` reaches the case; it follows from `scheduleCompletionTransitions` and
`eventHeap.Less`. **differs, v2 silent** — PSSM fixes the instant (the guard is read when the
completion occurrence is dispatched); the v2 text does not. §7.18.3 states only necessary
conditions — a transition usage "can only be triggered during a performance of its source" and
fires "if the guard expression evaluates to true" — and the library fixes an *ordering* but no
sampling instant: `TransitionPerformances.kerml`'s `StateTransitionPerformance` orders
`acceptable` before `guard` (`succession all [*] acceptable then [*] guard`), `guard` before the
source's `exit`, and the transition performance after the source's `nonDoMiddle` steps
(`succession [*] transitionLinkSource.nonDoMiddle then [1] Performance::self`, over
`StatePerformances.kerml`'s `nonDoMiddle = middle->excluding(do)`), and says nothing about when
within that window a guard is read. Reading the guard once, at completion, is a tool choice the
text neither prescribes nor forbids, as is PSSM's reading at dispatch; the runtime keeps its own,
and the row records the one observable difference above.

**SM10. Priority of completion events, and their scope.** PSSM §8.5.9: completion events are
dispatched before any other occurrence in the pool, in the order generated; a completion event
is bound to the state activation that produced it, and once that state has left the configuration
the occurrence can trigger nothing. *v2/KerML:* silent (no completion occurrence exists in the
library). *Runtime:* `eventHeap.Less` puts completion events first at one timestamp, by ID among
themselves; `dispatchEvent` drops an `EventTime` event whose source "was left before this event
came up" (`inActiveConfiguration`). So a current-time `EventTime` completion transition *is*
equivalent to PSSM's activation-scoped completion event in priority and in scope, with SM9's
sampling instant as the one proviso and one more: the runtime's stamp is the clock instant, so a completion scheduled
at `t` ranks behind an ordinary occurrence still queued with a stamp earlier than `t`; such an
occurrence can only exist if the instant it arrived was left undrained, which `runCounting` does
not do. `state_time_trigger_restarts_on_re_entry` pins the withdrawal of a left state's timer,
which is what makes a stale `EventTime` event rare; the drop itself has no fixture and rests on
`dispatchEvent`. **agrees**, under every policy, on the order among completion events too: the
runtime queues a state's completion as its entry unit is performed (`enterStateInto` →
`scheduleCompletionTransitions`, for a state that completes at once — `completesAtEntry`), so
the IDs follow the entry draw (SM22) that decided which state was entered first, as PSSM's pool
holds completion events in the order generated; `scheduleTransitionEvents` schedules the settled
configuration's time triggers alone. An entry that performs nothing but generates a completion
is a drawn alternative of the entry front, its order being observable through the pool's
(`state_completion_pool_entry_order` and its history and fork kin pin both orders under
`check`; `TestRuntimeRobustnessCompletionOrder`). Before the change the completions were queued
once the move had settled, leaf by leaf in region declaration order, agreeing under `declared`
alone; finding 11 below records it.

**SM11. What the completion of a composite state completes.** PSSM §8.5.5 and requirement *Final 001*
(§9.4.14): a transition to a final state "represents the completion of the behaviors of the
Region containing the FinalState"; when every region of a composite state has completed, the
*composite state* generates a completion event, its completion transition (if enabled) fires,
and the state machine goes on — the machine itself ends only when its own top-level regions
reach a final state or a terminate is reached. *v2/KerML:* SysML v2 §7.18.3: "a transition to
`done` indicates that the source state is the final state of the containing state performance,
though the containing state does not necessarily terminate immediately"; `States.sysml` binds
`done` to `StatePerformance::endShot` of the state whose body names it. The text ends the
containing state and says nothing about the state that encloses *it*. *Runtime:*
`state_executor.go:completeIfDone` → `machineComplete` walks outward from the completed region:
when a composite state's regions have all reached `done`, the enclosing region counts as complete
too, and so on up to the machine, which exits (`exitMachine`) and reports `StateCompleted`;
`scheduleFromLeaf` schedules completion transitions for active *leaves* only, so a nil-trigger
transition out of a composite state is never queued — a composite state's completion cannot fire
one. `state_completion_nested_regions` pins the machine completing on the last nested `done`
("The orthogonal regions of a composite state complete like the machine's own");
`state_entry_transition_nested_done` pins the same through an entry into regions already at
`done`. The spec-compliance record states the rule as adopted. Under PSSM the model
`state outer { … transition first a when Go then done; } transition first outer then next;`
enters `next`; here it ends the machine and `next` is unreachable. **differs, v2 silent** — and
see *Findings*, since the v2 sentence is at least uneasy with it.
*Decided:* the library's reading, which PSSM's rule coincides with. `States.sysml` declares
`ref state done: StateAction :>> Action::done, StatePerformance::endShot;`, so `then done`
written in a composite's body reaches the `endShot` of *that composite's* performance and ends
that composite only; the enclosing body's `transition first outer then next` is a
`TransitionPerformance` (`TransitionPerformances.kerml`) whose `transitionLinkSource` is `outer`
and whose `effect` and `transitionLink.laterOccurrence` — the entry of `next` — follow the
source's performance (`succession [1] transitionLinkSource then [*] effect`, then
`transitionLink.laterOccurrence`), a firing the runtime never performed. Ending the whole machine
at the composite's `done` had no basis in either file; it read `done` as the machine's `endShot`.
So a composite state whose do behavior has ended and whose every region has reached its `done` is
*complete*: its nil-trigger transitions are scheduled exactly as a leaf's are —
`scheduleCompletionTransitions` over the composite, guards read then (SM9), one event per enabled
transition at the current instant, ordered as any completion event (SM10) — and the machine ends
only when its own top-level regions are all at `done`. What the two library files leave open is
the composite with *no* enabled completion transition: neither names what its owner does once a
substate's `endShot` has passed with no transition to follow it. The runtime's rule, a tool
choice, is that it stays completed *and active*, its triggered transitions still firing and the
machine still running: under PSSM §8.5.9 the lost completion event ends nothing, and under v2
§7.18.3 the state performance has ended without its containing region reaching `done`, so
neither text ends the machine, and nothing propagates outward. `done` written among a declared
state's members lowers to that state's own completion vertex
(`lower/state_graph.go:completionOwner`). Implemented in
`state_executor.go:completeIfDone` → `scheduleCompletedComposites`/`completedComposite`,
`machineComplete` over `TopRegions`, `stateComplete`, `settleDoActions`; pinned by
`state_outer_completion_to_next`, `state_composite_completion_then_machine_done` (*Final 001*'s
shape), `state_composite_completion_nested`, `state_composite_completion_inside_region`,
`state_completion_nested_regions` and `state_entry_transition_nested_done` (rewritten with a
completion transition out of the composite), their `_stay_active` siblings (no completion
transition: machine running), `TestCompositeCompletionQueuesItsTransitionsLikeALeaf`,
`TestCompositeCompletesOnceItsDoBehaviorAndItsBodyHaveBothEnded` and
`TestCompletedCompositeWithoutCompletionTransitionStaysActive` in `state_completion_test.go`.

#### Entry, do and exit

**SM12. Order of entering a state.** PSSM §8.5.5 (`enter`): the entry behavior executes
synchronously, then the do activity is *started* (not awaited), then, for a composite state, the
regions are entered (requirement *Entering 002*, §9.4.5: the do activity "commences execution
immediately after the entry Behavior is executed" and "executes concurrently with any subsequent
Behaviors associated with entering the State, such as the entry Behaviors of substates";
*Entering 008*: each region of an orthogonal state is entered). *v2/KerML:* SysML v2 §7.18.1: "an entry action starts
when the state is activated; a do action starts after the entry action completes and continues
while the state is active"; `StatePerformances.kerml` `succession entry then do` and
`succession entry then middle`, with substates in `middle`. *Runtime:*
`state_executor.go:enterStateInto` runs the entry body to its end, then `startDoAction` for the
state, then `enterRegionsInto` (each region in declaration order to its initial leaf, entry
behaviors along the way); a substate's do action is likewise started by its own entry, before
its own body. `state_parallel_entry_behavior`, `state_nested_parallel_entry_exit_behavior`,
`state_do_action_declaration_order`, `state_entry_exit_action_successions`. The fixed policies
leave the trace as PSSM leaves it, "entry, region entries" in that order, the do action's steps
after the move; the one-move engines draw each due step of the composite's own do action
against its substates' entries as well (SM13, the do step on the entry front:
`state_do_step_before_own_substate_entries`, `state_do_step_before_own_body_entry`), the order
PSSM admits with the do activity running asynchronously. **agrees.**

**SM13. The do activity runs asynchronously, interleaved with the machine.** PSSM §8.5.6: the do
activity executes on a `DoActivityContextObject` of its own, "asynchronously" to the state
machine's RTC steps, reading and writing the context object's features through delegation
(`getFeature`, `setFeature`, `send`); its natural completion generates a completion event for the
state. *v2/KerML:* §7.18.1 "continues while the state is active"; the library's
`StatePerformance::do` is a sub-performance concurrent with `middle`. *Runtime:* a do action is
an `ActionExecutor` registered in `e.doActions`; `runStep` runs one *do round* — `runDoRound`
steps each due do action one step — before it dispatches an occurrence, so a do body advances
one node per step of its machine and may `accept` a message from the shared bus; the do body
reads and assigns the machine's attributes directly (`state_do_action_typed_inout_writes_back`,
`state_nested_parallel_region_owner_behavior`); its end schedules the state's completion
transitions (`settleDoActions`, SM8). `state_concurrent_do` records four admissible
interleavings of two do actions under the scheduling policies; `TestCompletionWaitsForTheDoBehavior`.
The step granularity (one action node per machine step) is a tool choice PSSM does not make
either — its do activity runs in the fUML "as if concurrent" sense — so no trace admissible
here is inadmissible there. The converse holds under `check`, `replay` and `explore`, where a
due do step and the dispatch at the head of the pool are drawn against each other per token move
(finding 9's fourth site, `ChoiceStepOrder`), and a due do step of an entered state against the
sibling regions' remaining entry units inside the entry front (the same site's rule on the
front, under the front's own draw; the fixed policies alone run the whole round before
they dispatch, one path of that enumeration — see
[recording the order of orthogonal regions](region-order-scheduling.md)). **agrees.**

**SM14. Order of leaving a state.** PSSM §8.5.5 (`exit`) and requirements *Exiting 001–003*, *005*
(§9.4.6): exit "commences with the innermost State"; a running do activity "is aborted before the
exit Behavior commences"; each region of an orthogonal state is exited, then the state's exit
behavior runs as "the final step"; the state is then removed from the configuration. *v2/KerML:*
§7.18.3 "1. If the source state has a do action that is still being performed, that is
interrupted. 2. Then, if the source state has an exit action, that is performed."; `exit`
follows `nonDoMiddle` in the library. *Runtime:* `state_executor.go:exitState` withdraws the
state's timers, records history (SM26), exits each region's active state in declaration order
(recursively), stops the do action (`stopDoAction`, which also releases a token parked at an
`accept`), then runs the exit body. `state_composite_exit_order`,
`state_composite_nested_regions_exit_once`, `state_do_action_signal_accept_cancelled_on_exit`,
`state_do_action_timed_accept_cancelled_on_exit`, `TestDoBehaviorIsCancelledWhenItsStateIsExited`.
**agrees.**

**SM15. A do activity and the machine competing for one occurrence.** PSSM §8.5.6 ("doActivity
accepter registration"): a do activity's `accept` is registered as an event accepter of the
*state machine's* object activation, so the do activity "will compete with the executing
StateMachine that invoked it to accept EventOccurrences dispatched from the same eventPool", and
fUML §8.8.2.11 `ObjectActivation::dispatchNextEvent` dispatches an occurrence "to exactly one of
those waiting accepters", chosen by the `ChoiceStrategy`; two rules give the do activity the
occurrence when the machine would defer it. *v2/KerML:* `Performances.kerml` gives every
performance its own `dispatchScope` (SM3), so a do sub-performance and the enclosing state
performance are separate scopes and nothing in `StatePerformances.kerml` says one transfer may
reach only one of them. *Runtime:* `broadcastEvent` computes the selected transitions first, then
`doBehaviorsTaking` lets a do action parked at an `accept` for the message take it as long as no
selected transition leaves that do action's state: a transition out of the state wins over its
own do action (`state_do_body_accept_yields_to_a_transition`), a transition between the state's
substates and the state's do action both go on
(`state_do_body_accept_goes_on_across_a_substate_transition`), and a do action in one region
and a transition in a sibling region are "dispatched once to both"
(`state_do_body_accept_shares_the_dispatch_with_a_region`; all `TestRuntimeRobustness`
subtests). PSSM gives one occurrence to one accepter; the runtime gives it to every dispatch scope
that accepts it, which is the KerML reading. **differs because v2 differs.**

**SM16. Exit, effect, entry of one transition, and when the guard is read.** PSSM §8.5.8: a
transition fires by exiting its source (up to the least common ancestor), running its effect,
then entering the target chain; the guard was evaluated during selection, before any exit.
*v2/KerML:* §7.18.3 steps 1–5 quoted above; `TransitionPerformances.kerml` `succession all guard
then effect`, `StatePerformances.kerml` `succession guard then transitionLinkSource.exit`.
*Runtime:* `passesGuard` runs in `selectTransitions`, then `fireFrom` → `exitState`, effect
(`state_region_transition.go:runEffect`), `enterStateInto`. `state_transition_guard_exit_effect_entry_order` pins
the four in that order with a guard that reads state the effect changes. **agrees.**

#### Transition selection, priority and conflicts

**SM17. What enables a transition.** PSSM §8.5.8 (`isTriggered`, `evaluateGuard`,
`canFireOn`): a transition is enabled for an occurrence when one of its triggers matches it and
its guard evaluates to true, the guard being evaluated with the occurrence's data bound to the
trigger's parameters; a guard that evaluates to nothing is not true. *v2/KerML:* §7.18.3 items 1–3
(source active, guard true, accepter accepts); the library's `TPCGuardConstraint`. *Runtime:*
`selectTransitions` → `matchesEvent` (`triggerMatches`) and `passesGuard` with `bindTriggerArguments` binding the
payload for the guard's duration; a guard with no result (`state_choice_unevaluable_transition`:
a division by zero) is "not true, so that transition is not selected", and reported as a
`guard-unevaluable` note rather than a failure. `state_transition_accept_payload`,
`state_call_trigger_guard`, `state_entry_transition_guard_first`. The guard is a step of the
`TransitionPerformance` it guards, performed after the trigger has been accepted
(`TransitionPerformances.kerml`: `bool guard[*] subsets enclosedPerformances`, `succession all
[*] trigger then [*] guard`), so it reads the payload by the transition's name as the exits and
effects of the *Read the leaving transition's payload* row below do — `if raise.l > 5`, `if
raise.d.level > 5` — and a compound transition is one performance, so a guard on a segment out
of a choice or junction reads the accepting segment's payload by that segment's name
(`evalTransitionStep`, `stepFiring`: the candidate transition's own firing for a plain guard, the
compound firing `resolveRoute` holds for a segment's; `transitionPayload` for the read). A guard
naming a transition not being taken reads null, and comparing it is the operator's type error
rather than false: the read resolves, so it is not "no result", and a null is no Boolean.
`state_choice_guard_reads_accepting_segment`, `state_junction_guard_reads_call_argument`,
`state_guard_reads_own_payload_member`, `state_guard_names_transition_not_taken`,
`state_guard_reads_deferred_payload`. **agrees.**

**SM18. Innermost first.** PSSM §8.5.2: "transition priorities, which are relative to the level
of nesting of their source states" — a transition from a substate outranks one from its
enclosing state for the same occurrence. *v2/KerML:* the `StatePerformance` `isDispatch`
invariant (SM3) requires the enclosing performance to exit only after a nested one that accepted
the same transfer, which is the same precedence expressed as an ordering. *Runtime:*
`selectTransitions` walks from each active leaf outward and stops at the first state with an
enabled transition; `losesToNestedTransition` removes an enclosing candidate when a nested one
was selected. `state_composite_inner_priority` (substate's transition wins),
`state_composite_outer_transition` (the composite's fires when the substate has none). **agrees.**

**SM19. Two enabled transitions out of one state.** PSSM §8.5.2: among conflicting transitions
of equal priority one is chosen "nondeterministically (using the ChoiceStrategy mechanism from
fUML)"; the test suite lists the outcomes as alternative traces (*Event 010*, §9.3.4.6: "T2 and
T3 are in conflict, but it is not possible to anticipate which one will be chosen"). *v2/KerML:* `StatePerformance::accepted [0..1]`
— one transfer accepted, and one `transitionLink` per performance; which of two enabled
transitions establishes it is unspecified (the project's semantic-oracle record says the same).
*Runtime:* `chooseTransition` draws one by the scheduling policy — declaration order under the
default — and records a `ChoiceTransition` when more than one was enabled;
`explore` enumerates the set; a state's completion is one occurrence too, so several completion
transitions out of one state are one `ChoiceTransition` drawn by `chooseCompletion` when the
completion is dispatched (*Event 015*, §9.3.4.11: "T1.2 and T1.3 are in conflict", either
fires). `state_choice_transition_conflict`, `state_explore_transition_conflict`,
`state_explore_completion_choice`. **agrees**: the runtime's choice is one PSSM admits and is
recorded as a choice.

**SM20. Selection is against the configuration the occurrence was dequeued for.** PSSM §8.5.10:
the RTC step selects its transitions against the active configuration at the start of the step;
a state the step enters does not react to the same occurrence. *v2/KerML:* `isRunToCompletion`
(SM2). *Runtime:* `broadcastEvent` computes `selectTransitions` and `chooseTransitions` before
any candidate fires; a candidate whose source was left by an earlier firing of the same dispatch
is skipped by `dispatchInOrder` ("still active"); each region takes at most one transition and
a transition shared by regions fires once. `state_transition_cross_region` (a cross-region
succession "cannot re-enable itself"), `state_transition_sibling_region`,
`TestEventConsumedByAnotherRegionIsNotDeferred`. **agrees.**

#### Orthogonal regions and the recorded choice points

Six choice points are recorded by `scheduling.md`. Three belong to the state executor and are
adjudicated here — `ChoiceTransition` (SM19), `ChoiceRegionOrder` (SM21, and SM24 for the do
round) and `ChoiceDueOrder` (SM42); the other three — `ChoiceTokenOrder`, `ChoiceDecisionBranch`,
`ChoiceWriteOrder` — belong to the action executor and are the fUML rows A5 and A7.

**SM21. Firing order across orthogonal regions.** PSSM §8.5.10 fires the selected transitions
"concurrently"; the test suite therefore lists every interleaving of the regions' effects as an
admissible trace (*Transition 011-D*, §9.3.3.7: the second region's substate exited before the
first's as the alternative;
*Transition 019*, §9.3.3.12: five interleavings of two regions' exits and effects).
*v2/KerML:* silent — the library orders a transition against its own source and nothing else, and
`state_explore_region_order`'s header records "the order ... is open". *Runtime:* `dispatchInOrder`
runs the selected firings as the queues of one front (`state_unit_front.go`), each firing's
units — its source's exit, its effects, its target's entry — in the library's order, and draws
the next unit among the firings with a unit ready from the scheduling policy, recording each
draw as a `ChoiceRegionOrder` labelled `on <event>` with the units as its alternatives
(`l1(exit)`, `l1->l2(effect)`); a unit may disable a sibling firing (its source left), which
then performs nothing. `declared` and `reverse` take the firings whole in declaration order, so
the run every fixture made before the draws were recorded is the run they make.
`state_parallel_broadcast`, `state_explore_region_order`, `state_composite_region_depth_order` (a
`.declared` and a `.seed-1` golden beside the default), `state_firing_units_interleaved` (two
regions' firings of exit and effect each: six outcomes, the linearizations of two chains of two).
*Decision:* a firing is **not atomic** across regions. `TransitionPerformances.kerml` orders a
firing's own units — `transitionLinkSource then effect`, `effect then
transitionLink.laterOccurrence`, `accept`/`guard then transitionLinkSource.exit` — and places no
succession between the units of two performances in sibling regions; §7.18.1 has the regions
"performed concurrently"; so the unit is the grain, and *Transition 019* (both sources' exits
before either segment's effect) is the check: every interleaving of the two firings' units is
a run `explore` reaches, and every one PSSM admits. **agrees.**

**SM22. Entering the regions of a composite state.** PSSM §8.5.5 enters regions concurrently
(*Entering 004*, §9.3.5.2, enters two regions from one transition; *Transition 011-D*'s alternative
trace is the interleaving PSSM admits for the exits).
*v2/KerML:* silent on order among `parallel` substates; §7.18.1 says only that they are performed
concurrently. *Runtime:* `enterRegionsInto` enters the regions as the queues of one front,
each region's entries a unit at a time, and draws which region's next unit runs from the
scheduling policy, recording each draw as a `ChoiceEntryOrder` labelled `entering <state>`
(`state_region_entry_order`, `state_region_entry_order_uneven`, `state_region_entry_nested_front`,
`state_parallel_standard`, `state_typed_region_order`, `state_parallel_entry_behavior`);
`enterForkBranches` draws a fork's branches the same way under `fork <name>`, the shared owner
entered once by the branch drawn first (`state_fork_branch_order`: *Fork 002*'s four traces),
and a history restores its regions through the same front (`state_history_restore_order`).
A unit that performs no behavior rides with the performing unit beside it, so exploration
counts linearizations of behaviors. `declared` and `reverse` take declaration order, the order
the runtime always took. **agrees.** The firing of a completion a region's entry enables is not a
unit of the front, and should not be — *Entering 010*, *Entering 011* — finding 11 below.

**SM23. Exiting the regions of a composite state.** PSSM §8.5.5 exits regions concurrently as
well (*Exiting 003*, §9.3.6.4, exits nested orthogonal regions). *Runtime:* `exitState` exits the
regions' active states as the queues of one front, each region's exits innermost first, drawing
which region's next exit runs and recording each draw as a `ChoiceExitOrder` labelled
`exiting <state>`; the composite's own exit follows every region's
(`state_region_exit_order`: *Exiting 001*'s three traces; `state_composite_orthogonal_exit` and
its trace golden, `state_composite_nested_regions_exit_once`). `declared` and `reverse` take
declaration order. **agrees.**

**SM24. Order of do activities in one round.** PSSM: do activities are independent asynchronous
executions; the *Behavior* and *Deferred* tests with do activities list up to 84 interleavings.
*Runtime:* `runDoRound` steps every due do action once per round, in an order `chooseDoAction`
draws from the policy and records as a `ChoiceRegionOrder` labelled `do round at t=…`;
`state_concurrent_do` and `TestChangeWatchOrderChoice` (`scheduler_test.go`) pin the draw.
**agrees.**

**SM25. A transition across regions and the common-ancestor rule.** PSSM §8.5.8: a transition
exits the states between its source and the least common ancestor of source and target, and
enters the states from there down to the target; a transition into a *sibling region's* vertex
from inside another region is not well formed in UML (it would require leaving one region while
the composite stays active), so PSSM has no rule for it. *v2/KerML:* the library orders
`guard then transitionLinkSource.exit` only, so a succession into a sibling region exits its
source and nothing else (the header of `state_transition_cross_region`). *Runtime:*
`state_region_transition.go:resolveRoute` computes the exit path up to the common ancestor
(`getLCA`, `entryPlan`, `enterOutside`) and the entry chain down; a cross-region target exits
the source only and takes the target region's active state over
(`state_transition_cross_region`, `state_transition_cross_region_third_region`,
`state_transition_leave_composite_substate_region`). The common-ancestor part **agrees**; the
cross-region succession is a v2 construct PSSM has no counterpart for and is not counted.

#### History

**SM26. Shallow history.** PSSM requirement *History 005* (§9.4.15): a shallow history restores "the most
recent active substate of its containing Region, but not the substates of that substate", with
the full entry semantics. *v2/KerML:* no notation and no library element; `history` is this
project's extension with UML as its reference (`pseudostates.md`). *Runtime:*
`exitState` records the region's active state in `recordRegionHistory`;
`fireHistoryTransition` → `moveToHistory` exits the source configuration first, reads the
record once the exits have run (finding 7), re-enters the owner and restores that substate,
which for a shallow history is entered through its own initial transition
(`state_shallow_history`, `state_history_revisit`,
`state_shallow_history_completion_default`: a completion transition of the owner into its own
shallow history). A history declared in the machine's own body restores the machine's
top-level configuration (`state_machine_body_deep_history`). **agrees.**

**SM27. Deep history.** PSSM requirement *History 001* (§9.4.15): "the full state configuration of the most recent
visit ... including execution of all entry Behaviors encountered along the way", outermost
first (*History 004*). *Runtime:* `historyRecord` keeps the recorded child per region
recursively; `fireHistoryTransition` with a deep history restores the chain through
`enterStateInto` along the recorded path, running each entry body (`state_deep_history`,
`state_deep_history_region_composite`, `state_deep_history_self_transition`: the owner's
self-transition restores the configuration it is leaving). **agrees.**

**SM28. History with nothing to restore.** PSSM requirements *History 002–003* (§9.4.15): with no prior visit, or a
region that "had reached its FinalState", the history's outgoing transition (the default history
transition) is taken; with no such transition "standard default entry of the Region is performed"
(the region's initial transition). *Runtime:* `fireHistoryTransition` takes the history's own
outgoing transition when no configuration is recorded; with neither it fails the run with "no
recorded configuration" (`robustness_test.go:history_without_record_or_default`), where PSSM
enters the region's initial state. A region that reached `done` is not a case here, because that
completes the machine (SM11); a region left with no active state is forgotten
(`forgetRegionHistory`). **differs, v2 silent.**
*Decided:* PSSM's rule. A shallow or deep history with no recorded configuration takes its own
outgoing transition when it has one, else performs the owning state's default entry — the same
path an ordinary transition into the composite takes, through its `entry` transition and on down;
an owner that declares no entry transition either is the typed `ErrHistoryWithoutEntry`, raised
at the transition and naming the history and its owner. Since SM11's decision, a composite whose
region reached `done` and then left it is a case here too: the record is forgotten, so the next
history entry is a default entry. Implemented in `state_executor.go:historyEntry` (via
`hasDefaultEntry`, from `fireHistoryTransition`) and `recordChildHistory`/`recordRegionHistory`
(a body or region left at `done` leaves no record); pinned by
`state_history_empty_default_entry` (*History 002*), `state_deep_history_empty_default_entry`
(*History 003*), `state_history_after_completion_default_entry`,
`state_history_after_completion_default_transition` (the history's own default transition is
preferred over the entry after a completion too) and
`history_test.go:TestHistoryOverACompletedConfigurationIsADefaultEntry` (a completed region
beside a running one, and a body left at `done`) and
`robustness_test.go:history_without_record_default_or_entry`.

#### Choice and junction

**SM29. Junction: guards read before the step.** PSSM requirement *Junction 001* (§9.4.11): a junction's
outgoing guards "are evaluated before any compound transition containing this Pseudostate is
executed" — statically, as part of deciding whether the incoming transition is enabled — and
*Junction 003*: with several true, one is chosen, algorithm undefined. *v2/KerML:* no junction
in v2; the project's `junction` is a UML-referenced extension (`pseudostates.md`, "a static
conditional branch"). *Runtime:* `state_route.go:resolveRoute` → `followOut` resolves the
route through a junction *before* the incoming transition fires, so the guards read the data as
it stands before the incoming effect; every outgoing guard is read (`enabledBranches`), one
enabled branch is taken, and several enabled leave the route open at the junction (`junctionDraw`)
for the transition choice point drawn there only as the transition fires (`settleDraws` →
`pickBranch`: after the region order among several candidates and the transition's own guard
read again, `ChoiceTaken` drawn by the schedule policy and enumerated by `explore`; a candidate
another region's reaction disarms draws nothing; no branch guard is read again — the route
beyond each enabled branch, through any further junction, is settled with the transition — so a
branch enabled at selection is taken along it though another region's effect since changed what
its guards read), the unguarded branches being the default when no guard holds.
`state_junction_pseudostate`, `state_completion_through_pseudostate`,
`state_junction_several_enabled_branches`, `state_junction_drawn_as_its_transition_fires`,
`state_junction_guards_read_once`, `state_junction_beyond_a_draw_read_once`,
`state_history_default_through_junction` (+ trace goldens,
every branch under `explore`),
`explore_test.go:TestExploreStaticJunctionBranches`, `TestExploreJunctionDrawnAsTransitionFires`,
`TestExploreHistoryDefaultThroughJunction`. **agrees.** SM30's decision moved the
static resolution to `state_route.go` and left junctions static; *Finding 8* made several
enabled branches a choice point where the runtime had taken the first in declaration order.

**SM30. Choice: guards read on arrival.** PSSM requirement *Choice 001* (§9.4.10): the guards "are evaluated
dynamically, when the compound transition traversal reaches this Pseudostate" — after the
incoming segment's effect has run — and *Choice 002*: with several true, one is chosen.
*v2/KerML:* no choice vertex in v2 (a `decide` is an action-body construct, not a state-body
one); `pseudostates.md` names the choice "a dynamic conditional branch whose outgoing guards are
evaluated when the choice is entered". *Runtime:* `transientPseudostate` returns true for both
kinds and `resolveRoute` resolves a choice exactly as a junction: the branch is picked in
`pseudostateBranch` when the route is resolved, before the incoming transition's effect runs, and
the code comment states that the two are "indistinguishable for a guard over state data". So
`transition first idle do assign x := 1 then pick; transition first pick if x == 1 then seen;`
takes the unguarded branch, not `seen`, where PSSM would take `seen`. `state_choice_pseudostate`
pins choice routing on data the incoming effect does not touch, so the difference has no fixture
on `develop`; it follows from the two functions and the comment. **differs, v2 silent** — and
a *Finding*, since the project's own design note says otherwise.
*Decided:* PSSM's rule, and the project's own note. A route is resolved before firing only up
to the first *choice*: junctions along it are resolved statically as before (SM29, SM32
unchanged). Firing then exits the states every branch of the choice leaves (`travel` →
`certainExits` over the targets `reachable` past it), runs the segments' effects up to the
choice, and only then reads the choice's guards against the data as it now stands; one enabled
branch is taken — several enabled is the existing transition choice point, recorded as
`ChoiceTaken` at the choice and enumerated by `explore`, an unguarded branch is the else branch
— and the route goes on: a junction reached next is read statically at that point, a further
choice lazily again on arrival. No enabled branch is the typed `ErrChoiceWithoutBranch` at that
instant, naming the choice (SM31). Implemented in `state_route.go:resolveRoute` (stops at the
first choice), `followOut` (junctions read statically), `travel`, `resolveChoice`, `settleDraws`,
`state_executor.go:fireTransition`; pinned by
`state_choice_after_incoming_effect` (*Choice 001*: `assign x := 1 then pick; … if x == 1 then
seen` reaches `seen`), `state_choice_dynamic_conflict`, `state_pseudostate_chain_junction_choice`,
`state_pseudostate_chain_choice_junction`, `state_pseudostate_chain_choice_choice`,
`TestExploreDynamicChoiceBranches` and `robustness_test.go:state_choice_without_an_enabled_branch`;
`state_choice_pseudostate` and the other `state_choice_*` cases keep their outcomes.

**SM31. Choice with no guard true.** PSSM requirement *Choice 003* (§9.4.10): "the model is considered ill formed".
*Runtime:* `state_route.go:resolveChoice` fails the run at the instant the choice is reached,
after the incoming segment's effect, with the typed `ErrChoiceWithoutBranch` naming the choice
(`robustness_test.go:state_choice_without_an_enabled_branch`); a junction with no guard true
fails as SM32 describes (`region_pseudostate_without_satisfied_guard`). **agrees.**

**SM32. Junction or join with no path through.** PSSM requirement *Junction 002* (§9.4.11): when no outgoing guard holds, "the
entire compound transition is disabled even though its Triggers are enabled" — the incoming
transition is not selected, and the occurrence is deferred or lost like any other unhandled one.
*Runtime:* the incoming transition is selected on its own trigger and guard; the failure to route
surfaces as a "no guard evaluated to true" run failure when the route is resolved, not as a
disabled transition (`region_pseudostate_without_satisfied_guard` uses a junction).
The same holds wherever on the compound transition the junction with no way through lies.
*Junction 004* (§9.4.11) puts it on the default entry of a sibling region: the transition
targets a junction inside one region of an orthogonal state, and the other region's initial
transition leads to a junction both of whose guards are false; PSSM's static evaluation takes
the default entry in and disables the whole transition — the state is never entered, its `entry`
never logged, and the next occurrence fires from the source — where the runtime enters the state
and fails at the second region's junction. *Join 003* (§9.4.12) puts it at a join's way out: the
join's only outgoing transition is guarded false, so PSSM fires the first completion transition
into the join on its own (a segment may end at a join, whose completion is waited for) and
disables the second, whose entering the join would need a way through; the owner stays active
for the next occurrence. The runtime fires nothing at the first completion (SM34: the segments
fire together, once every source is active) and fails at the second, resolving the route out of
the join. **differs, v2 silent.** Unchanged by the decisions below: a junction's guards stay
static, and the runtime's rule here stays the project's. *Choice 005* traces the same reach
from the other side — its junction on the composite's default entry is read before `T2(effect)`
and the composite's entry — and is refused on its acting guards before the order is reached; see
[A guard whose behavior acts on the model](#a-guard-whose-behavior-acts-on-the-model).

#### Fork and join

**SM33. Fork.** PSSM §8.5.7 (`ForkPseudostateActivation`) and requirement *Entering 010* (§9.4.5, the
forked regions "are entered explicitly and the others by default"): the parent is entered under the
common-ancestor rule, then every outgoing transition fires "without any guard evaluation, since
UML does not allow Transitions outgoing a fork Pseudostate to have guards", each into a different
region of one orthogonal state. *v2/KerML:* no state-body fork in v2 (`fork` is an action node);
the extension follows UML. *Runtime:* `fireForkTransition` → `forkPlan` enters the target
composite with each branch's target as that region's initial configuration; `planFork` refuses a
guarded branch ("outgoing transitions cannot be guarded"), a triggered one ("cannot have
triggers", `fork_branch_with_a_trigger`), a branch outside an orthogonal region and two branches
into one region (`robustness_test.go:fork_branches_share_region`).
`state_fork_join_pseudostate`. **agrees.**

**SM34. Join.** PSSM requirement *Join 001* (§9.4.12): "all incoming Transitions have to complete before execution
can continue through an outgoing Transition"; the join fires when every source state is active
and the occurrence enables all incoming segments. Requirement *Join 002* (§9.4.12) and §8.5.7
(`JoinPseudostateActivation`): the incoming transitions and the outgoing one are segments of one
compound transition, so every incoming segment fires — its source exited, then its effect — in
either order, before the state owning the join is exited and the outgoing effect runs (the test's
expected execution: the two incoming effects in parallel, the owner's exit, then the outgoing
effect). *v2/KerML:* no state-body join in v2 (`join` is an action node, as SM33 says of `fork`);
the extension follows UML, and the library's `transitionLinkSource then effect` and "happening
during the state performance" fix each segment's exit-then-effect and place both segments before
the owner's exit
([oracle](../../project/behavior-semantic-oracle.md#transitions-into-a-join-each-exits-its-source-and-runs-its-effect-before-the-owner-is-left-in-which-order-is-open)).
*Runtime:* `fireJoinTransition` fires only when `joinSynchronized` finds every other segment
into the join enabled by the occurrence being dispatched — its source active, its guard holding,
its trigger matching the same signal, call, timer expiry or change rise — whichever path fires
the segment: a signal or call dispatch, a timer, a completion or a change poll
(`state_join_waits_for_every_segment_enabled`, `state_join_segment_trigger_unmatched`,
`state_join_time_segment_needs_same_occurrence`,
`state_join_time_segment_unsynchronized_reads_no_route` — the route out of the join is
resolved only once the join is ready, so an expiry that holds it reads no guard beyond it —
`state_join_time_segments_expire_together`, `state_join_time_segments_expire_apart` — each timer
is its own occurrence, so a time-triggered segment is enabled while its own timer is due: two
expiries at one instant fire the join, whichever is dispatched first, and expiries at different
instants never do —
`state_join_change_segments_rise_together`,
`state_join_change_segment_rises_alone` — one condition rising is one occurrence, so a later
rise of the other segment's condition does not fire the join);
`fireJoinIncoming` then fires the incoming segments one at a time, drawing the next from the
scheduling policy as a `ChoiceRegionOrder` labelled `join <name>` (declaration order by default),
each with the arguments its own trigger takes from the occurrence bound (`fireJoinSegment`,
`state_join_segment_reads_its_payload`), before the owner is exited and the outgoing segment
followed. The owner and the region of it each segment leaves are the lowerer's `JoinPlan`, found
by the same ancestor walk that places the segments one per region, so a segment whose source
is nested below a region's state exits its wrappers up to the region and the owner is left
(`state_join_from_nested_states`, `join_from_nested_states_wrapper_exit_that_fails`), a segment
whose source is a composite state with an active substate is enabled through that substate and
exits it first (`state_join_from_composite_sources`, `TestJoinFromActiveCompositeSourcesFires`,
`join_from_composite_source_substate_exit_that_fails`), the region recorded being the owner's own
— the machine's top-level one, for a join of the machine's regions — however deep the source lies,
so an orthogonal state between them is exited once, with the segment (`state_join_of_machine_regions_from_nested_source`,
`join_of_machine_regions_nested_source_owner_exit_that_fails`, `lower/join_check_test.go`), and a
sibling segment's guard the firing occurrence has read fail is the step's error
(`join_time_segment_sibling_guard_that_fails`); a replay refused at a later draw undoes the segments already
fired with the rest of the move (`TestReplayRefusedJoinDrawChangesNothing`); a join with a single incoming
branch is refused (`join_with_one_incoming_branch`), as is one two of whose incoming transitions
leave the same region — UML 2.5.1 §14.2.3.5 Pseudostates has a join target "two or more
Transitions originating from Vertices in different orthogonal Regions", so every segment fires
when the join does and none is an alternative to another (`lower/join_check.go`). `state_fork_join_pseudostate`,
`state_join_runs_every_incoming_effect` (both orders as `outcomes`, explored). On the shape —
every segment exits its source and runs its effect, the outgoing effect follows the last — the
two agree; on two orders within it they part. *Where the owner is left.* The runtime leaves the
state owning the regions after every incoming effect (`fireJoinIncoming`, then the outgoing
segment's exits), the reading the oracle section derives from the library: each segment is
declared in the owner's body and is an `enclosedPerformance` of it, ended before the owner's
`exit`. *Join 001*'s prose expected execution reads the same way — each segment's source exit
and effect, in parallel, then the owner's exit, then the join — but the traces its assertion
admits put the owner's exit between the last source's exit and that segment's effect
(`T2.3(effect)` or `T2.4(effect)` closes the log): the owner is left with the last source — the exits of the
segment that completes the join climb to the compound transition's common ancestor, and its
effect runs after them. *When the segments fire.* PSSM §8.5.7 fires each incoming transition as
its own occurrence is dispatched — a completion transition into the join when its source's
completion event is dequeued, the join's activation counting the segments that have arrived — so
in *Transition 019* (§9.3.3.12) the two completion transitions into `Join1` fire in the order the
regions completed, `T1.3(effect)` after `T1.2(effect)`'s region and `T2.3(effect)` after the
other's, and the suite admits no trace with the join's segments in the opposite order; the
runtime holds every segment until the join is ready and then draws their order afresh, so it
reaches those two traces too. On neither order does v2 speak — the join is not a v2 state
construct, and the library derivation reaches the owner's exit only through this project's
reading of a join segment as a substate's transition — so the project's rule is the oracle
section's: the segments fire together, in an open order, and the owner is left after the last.
**differs, v2 silent.** Adopting PSSM's orders would leave the owner between the last source's
exit and its effect, and tie a completion-fired segment's place to its source's completion,
which SM19's completion choice already draws.

#### Transition kinds: external, local, internal

**SM35. External transitions, self-transitions included.** PSSM §8.5.8: an external transition
exits its source (a self-transition exits and re-enters it, running exit and entry behaviors;
a transition from a composite state to one of its own substates likewise exits the composite).
*v2/KerML:* §7.18.3 "the triggering of a transition usage from its source state usage to its
target state usage deactivates the source state and activates the target state" — every
transition deactivates its source; there is no kind attribute. *Runtime:* every transition is
external: `state_composite_self_transition` ("exits the active substates innermost-first and the
composite, runs the effect, then re-enters"), `state_composite_self_transition_in_region`,
`state_composite_to_substate` ("is also external: the composite is exited and re-entered around
the effect"). The one target an external transition does not enter is one that is already
active: PSSM §8.5.8 (`ExternalTransitionActivation`) enters the target only if it "can be
entered" and otherwise, for a composite target, "the RegionActivation owning the
sourceVertexActivation completes" — a transition from a substate into the composite state
enclosing it exits the substate, runs its effect and leaves the region complete, so a composite
with no other region completes and its completion transition fires (requirement *Transition
011-C*, §9.3.3.7: the admitted trace ends with the transition's effect and the composite's
exit, with no second entry of the composite). §7.18.3 orders the
source's exit and the effect and says nothing of a target already active; the runtime follows
PSSM: `moveTo` → `completeInto` when the exit boundary is the target itself
(`state_transition_into_active_ancestor`, `state_transition_into_active_parallel_ancestor`).
**agrees.**

**SM36. Local transitions.** PSSM §8.5.8 / UML 14.2.3.8.1: a local transition whose source is a
composite state and whose target is inside it does not exit the source; only the substates
between are exited and entered. *v2/KerML:* the §7.18.3 sentence above — the source is
deactivated by every triggered transition — leaves no room for a transition that keeps its
source active, and `StateTransitionPerformance` always sequences `transitionLinkSource.exit`.
*Runtime:* none; a composite-to-substate transition is external (SM35).
**differs because v2 differs.**

**SM37. Internal transitions.** PSSM §8.5.8: an internal transition runs its effect without
exiting or entering its source, so no exit or entry behavior runs and the do activity continues.
*v2/KerML:* the same §7.18.3 sentence; a self-transition to the same state is the nearest
spelling and deactivates the source (SM35). *Runtime:* none. **differs because v2 differs.**
(An idiom that reaches the effect without leaving the state — a do action with an `accept` — is
SM13/SM15, a different construct.)

#### Terminate

**SM38. Terminate.** PSSM requirements *Terminate 001–002* (§9.4.13): entering a terminate
pseudostate means "the execution of the StateMachine is terminated immediately. The StateMachine
does not exit any States nor does it perform any exit Behaviors"; running do activities "are
automatically aborted", and entering it "is equivalent to invoking a DestroyObjectAction".
*v2/KerML:* SysML v2 §7.17.10 defines the terminate action, and §7.18.3 shows `accept Abort via
commPort then stop; action stop terminate;` as the way "to immediately terminate the containing
state performance"; `Performances.kerml` `TerminatePerformance`. *Runtime:* lowering carries
every `terminate` as a `lower.Effect` of kind `EffectTerminate` with the terminated occurrence
as an evaluable target (`TerminateContaining`, `TerminateEnclosing`, `TerminateNode`,
`TerminateOccurrence`), and a state graph lists its terminate action usages with the composite
state each is declared in (`lower.StateGraph.Terminates`, `TerminateOwner`). A `terminate` in
an action or state-behavior body ends the containing performance, or the occurrence it names,
at that statement (`action_terminate.go:performances.terminate`,
`occurrence_terminate.go:terminateOccurrence`): the outputs assigned so far stay, the tokens
held anywhere in the ended action are dropped, and a state's entry, do or exit behavior ends
while the state stays active and the machine keeps dispatching. A transition whose target is a
terminate action usage ends the state-machine performance where it arrives
(`state_route.go:terminateAt`, `state_executor.go:terminateMachine`): the transition's source
is exited and its effect run, the states down to the usage's owner are entered, then nothing
else is exited, running do behaviors are abandoned (`abandonMachine`), and the outcome is
`Terminated` rather than a final state. Only a calculation still refuses it, as a side effect
(`robustness_test.go:calc_terminate_is_rejected`). PSSM's rules for the pseudostate — "does
not exit any States", do activities "automatically aborted" — are these; the source's exit is
in PSSM's own expected traces (the last step of *Terminate 001*'s admitted trace is the exit of
the state whose completion transition reaches the pseudostate). **agrees**.

*The braced block as the unit a `terminate` ends.* SysML.xtext reads `entry { … }`, `do { … }`,
`exit { … }` and a transition's `do { … }` as one `ActionUsage` with an `ActionBody`, so a
`terminate;` written in one names the performance of the whole block (`Performances.kerml`
`TerminatePerformance`). The parser takes that reading: a braced block is one anonymous action
usage whose body is the block's statements, the tree `entry action { … }` produces
(`parser/behavior.go:parseBracedActionUsage`, reached from `parseStateSubactionBlock` and
`parseTransitionEffect`; the usage's `Keyword` is empty where none was written, which is how
the formatter and the converters keep the spelling). Everything downstream sees one action:
the block is a namespace of its own whose declarations are local to it, lowering gives it one
`lower.StateBehavior` whose `Body` is one `Block` (`lower/state_behavior.go:lowerStateBehavior`),
the runtime runs it as it runs any inline body — one statement per do round, so orthogonal
regions still interleave statement by statement and a do body still resumes between statements
(`state_anonymous_do_atomic`, `state_concurrent_do`, `state_concurrent_inline_do_bodies`) — and a
`terminate;` in it resolves to the block's own performance, ending the block and nothing beside
it (conformance `state_terminate_braced_entry_do_exit_effect`, `state_terminate_braced_do_after_accept`,
`state_terminate_braced_entry_among_named`, `state_terminate_braced_inherited_by_two_usages`,
`state_terminate_braced_do_in_one_region`, `state_braced_block_local_attribute`). The one
grouping left is a transition's own body, `then t { … }`, which is the transition usage's
`ActionBody` rather than an `EffectBehaviorUsage` and still lowers one behavior per statement:
`lower.StateBehavior.Block` names the `TransitionMember` those steps share, and a `terminate`
in one of them ends the steps after it (`state_statements.go:StateExecutor.endedBefore`,
conformance `state_terminate_transition_body`).

#### Time and change events against the simulation clock

PSSM restricts a transition's triggers to `CallEvent`s and `SignalEvent`s
(`pssm_transition_triggers`, §7.5.2) and inherits fUML §2.3's exclusion of `TimeEvent` and
`ChangeEvent`, which "imply a background infrastructure, such as a model of time or a mechanism
for monitoring for change". The four rows below therefore cannot disagree with PSSM; they carry
**agrees** in the sense that no PSSM rule constrains them, and the count says how many rows
agree by exclusion.

**SM39. Time triggers.** *v2/KerML:* §7.18.3 `accept after <duration>` / `accept at <instant>`,
with `Clocks::Clock` and `TimeInstantValue`; the project reads the duration from the state's
entry. *Runtime:* `scheduleTimeTransitions` queues an `EventTime` event at `clock.now +
duration` when the state is entered (a composite's timer counts from entering the composite,
`state_composite_outer_time_trigger`); `exitState` withdraws the timer; re-entry restarts it
(`state_time_trigger_restarts_on_re_entry`); units are converted to seconds
(`state_time_trigger_test.go`). **agrees** (by exclusion).

**SM40. Change triggers.** *v2/KerML:* §7.18.3 `accept when <condition>`, a `ChangeSignal`
accepted when its condition becomes true; the project takes a rising edge. *Runtime:*
`state_change_trigger.go` polls the conditions of the active configuration's transitions once
per `runStep`, after the do round and before the queued occurrence, firing on a false-to-true
edge through the same `dispatchInOrder` as a signal (`state_change_trigger_rising_edge`,
`state_change_trigger_event_order`, `state_change_trigger_autonomous`). **agrees** (by exclusion).

**SM41. The shared clock: quiescence before time moves.** PSSM §2.3: a tool "may be limited to a
single centralized time source such that all time measurements can be fully ordered", a
constraint it permits, not one it makes. *v2/KerML:* `Clocks.kerml` `universalClock`, one
timeline per clock. *Runtime:* one `Context.Clock` for every executor of a context;
`runCounting` drains the current instant — every executor with a due event or a runnable do
action — before `advanceToNextDue` moves the clock to the earliest queued wait, and a machine
with nothing due never moves it (`clock_two_machines_share_clock`, `clock_action_state_due_together`
with its `.declared` and `.seed-1` goldens, `clock_do_action_while_token_waits`). `Context.Advance`
runs what is due, advances to each queued wait in turn, and stops at the requested deadline
(`robustness_test.go:clock_advance`). **agrees** (by exclusion).

**SM42. Due order at one instant.** With two executors due at one instant, PSSM has no rule
(each machine is its own active object; fUML §8.8 leaves inter-object scheduling to the tool).
*Runtime:* `Context.runDue` runs them in creation order under the default and records a
`ChoiceDueOrder` (`scheduler_test.go:TestDueOrderChoice`, `clock_action_state_due_together`).
**agrees** (by exclusion).

#### Object lifecycle

**SM43. Starting the classifier behavior.** fUML §8.8.1 (`ObjectActivation`): an active object's
classifier behavior starts when the object is started — `StartObjectBehaviorAction`
(§8.10.2 `StartObjectBehaviorActionActivation`, `Object::startBehavior`), not its creation:
`CreateObjectAction` (§8.10.2 `CreateObjectActionActivation`) creates the object and offers it
on its result pin, and the behavior of an object nobody starts never runs — each behavior in an
execution of its own, sharing the object's event pool; PSSM §8.5.1 makes a state machine such a
classifier behavior. *v2/KerML:* §7.18.4 an
`exhibit state` "must be carried out entirely within the lifetime of the performing occurrence";
`Objects.kerml`/`Occurrences.kerml` `performances`. *Runtime:* two paths, told apart by what the
type declares. A behavior the type **exhibits or performs** is bound to every object of it, so
`Context.Instantiate` → `classifier_behavior.go:runAttachedBehaviors` starts every exhibited state
machine and performed action of a part as its own executor on the shared bus and clock
(`TestInstantiateStartsExhibitedStateMachine`, `TestExhibitedMachinesOfTwoObjectsAreIndependent`,
`TestExhibitedMachineWritesItsObjectsFeatureValues` in `classifier_behavior_test.go`) — the v2
reading, where a performance a type declares is carried out within every occurrence's lifetime. A
behavior the type **merely declares** (`action beh : Beh;` in a `part def`,
`lower.StartableBehaviorOf`) is the fUML reading: construction is passive — `new T()` and a
materialization run nothing of it — and an explicit `StartObjectBehaviorAction`, spelled
`perform obj.beh.start;` (`lower.EffectStart`, `start_behavior.go:startBehaviorOn`), starts the
classifier behavior as the object's own execution, `this` in it the object, a later message waking
an accept it parks at, a second start of a running behavior starting nothing more, and a start
that fails undone whole — the start's own work: an older parked behavior a message of the
started one wakes is drained only after the start is kept (a run boundary, as a store's), so its
move is never undone with a start (`robustness_classifier_behavior_test.go`,
`TestStartedActionAwaitingAMessageIsWokenByASibling`). Lowering tells the start shot from a feature
declared under that name through the scope tree (`action_graph.go:namesStartableBehavior`): `perform
vehicle.start;` where `Vehicle` declares `action start : Launch;` performs that action
(`TestPerformOfDeclaredStartActionStaysPerform`). The fUML referee's emitter takes the
second path for an active class, so `ActiveClassBehaviorSender` runs the reference's order:
create, start, send. One pool per machine rather than per object (SM1) is the one structural
difference, and it is the v2 one. **agrees.**

**SM44. The machine ends; the object does not.** fUML §8.8.1: a classifier behavior completing
does not destroy its object; the object persists, receives occurrences, and handles them with
whatever behaviors remain. PSSM: a machine that reached its final state stays in it, occurrences
addressed to it are lost. *v2/KerML:* `done` ends the state performance, `Life` of the object is
separate. *Runtime:* `completeIfDone` ends the performance (`endPerformanceLife`) and the machine
reports `StateCompleted`; the object remains, later messages to the machine are dropped
(`robustness_test.go:state_event_after_completion`, `state_completion_rests_in_done`,
`classifier_behavior_test.go:TestMessageLeftForACompletedMachineDoesNotBlockANewObject`); a
started classifier behavior completing preserves its owner the same way — the object, its feature
values and the behavior's writes to them outlive the behavior, and an activity may still hand the
object out through a parameter (`robustness_classifier_behavior_test.go:start_runs_the_behavior_as_the_object`,
`TestStartedActionAwaitingAMessageIsWokenByASibling`). **agrees.**

**SM45. Destroying the object.** fUML §8.7.2.4 `Object::destroy`: "Stop the object activation
(if any), clear all types, clear all feature values and destroy the object" — a running
classifier behavior is stopped by the destruction (`DestroyObjectActionActivation`, §8.10.2.16). PSSM: the same, through terminate or
destruction of the context object. *v2/KerML:* §7.18.4's "entirely within the lifetime" is a
constraint on the model, and `Occurrences.kerml` ends portions with their owner; no library text
says what a `destroy` of a still-performing object does. *Runtime:* `lifetimes.go:Context.destroy` refuses
while a performance of the object is still active (`lifetimes_test.go:TestDestroyRefusedWhilePerforming`),
ends the object's portions and refuses later reads once it goes ahead
(`TestDestroyEndsPortionsAndRefusesReads`, `TestDestroyedObjectPerformsNothing`), and a binding to
a destroyed end is refused (`TestBindingRefusesADestroyedEnd`). fUML aborts the behaviors and
destroys; the runtime refuses the destruction. **differs, v2 silent.**

### Actions (fUML)

fUML's activity semantics (formal/21-03-01 §8.9 and §8.10) are the ground PSSM's behaviors
stand on, and the part of the family closest to the action executor
(`action_executor.go`, described in [action-executor.md](action-executor.md)). The rows are
briefer than the state-machine rows: the v2 side is the KerML `HappensBefore` succession and the
`ControlPerformances.kerml` fork, join, merge and decision performances that
`docs/project/spec-compliance.md` already quotes row by row, so each row cites that record rather
than repeating it. The scheduling choice points `ChoiceTokenOrder`, `ChoiceDecisionBranch` and
`ChoiceWriteOrder` of [scheduling.md](scheduling.md) belong here.

**A1. Offers, readiness and firing.** fUML §8.9.1 and §8.10.1: a node activation receives an
*offer* on each incoming edge, `isReady` holds when every incoming control flow and every input
pin has an offer, and only then does `takeOfferedTokens` accept the tokens and `fire` run the
node — checked and taken inside one isolated region so no other activation can take the offer
meanwhile. *v2/KerML:* each succession is a `HappensBefore` link whose later occurrence is the
one target performance, and a step with no declared multiplicity holds one value (KerML 1.0
§7.4.5), so a node reached over several successions is one performance after all of them; the
compliance record's "An action node reached over several successions is one performance that
follows all of them" row states the derivation. *Runtime:* tokens rather than offers — a `Token`
stands at a node and `action_executor.go:stepToken` steps it; a node with several incoming
successions holds arrivals until every succession has delivered one (`synchronize`, `arrivals`,
`awaitedSuccessions`), and a node whose successions are all in is stepped in one step. No
two-phase offer exists because nothing but the executor's own loop can take a token. The
observable difference — fUML fires when the *last* offer arrives, the runtime when the last
token arrives — is none. `action_node_with_two_incoming_successions_runs_once` and its trace
golden, `action_nested_node_two_successions_per_performance`. **agrees.**

**A2. Control tokens and object tokens.** fUML §8.9.2.10 `ControlToken` ("has no value" and
"all control tokens are interchangeable"), §8.9.2.21 `ObjectToken` (carries one value), §8.9.2.22
`Offer` (an offer may group control and object tokens); an object flow moves a value from an
output pin to an input pin and also sequences the target, an action with unsatisfied input pins
never fires. *v2/KerML:* a succession carries no value (`HappensBefore` links occurrences only);
a `flow` is a `Transfer` (Kernel Semantic Library `Transfers.kerml`) of a payload from a source
output feature to a target input feature, and SysML v2 §7.16.1 distinguishes a *streaming flow*
(`flow`, ongoing while both ends perform) from a *succession flow* (`succession flow`, whose
"transfer source must complete before the transfer starts, and the transfer must complete
before the target can start") and from a *message* with no ordering of its own; a value read
through `node.pin` is a feature read in the target's frame, not a token. *Runtime:* one `Token` carries
control, and values move by `action_executor.go:performances.applyDataFlows` — when a performance
ends, "what the completed performance produced" is written along `ActionGraph.DataFlows` to the
target pins, a source pin holding nothing being an error, not a no-op; an input pin never gates
the target on its own, only successions do. The compliance record's "Object flow (pin-to-pin
data)" row (`applyDataFlows`, `action_output`) and its "One feature space per performance" row
(a succession "orders occurrences and carries no values") pin this; `action_output`,
`action_nested_flow_writes_value`, `action_invoked_node_body_writes_output`. fUML's
value-bearing offer that also sequences is v2's succession flow, which the runtime has; an fUML
input pin gating its target is not a v2 rule, and v2's streaming `flow` is A8. **agrees.**

**A3. Fork.** fUML §8.9.2.16 `ForkNodeActivation`: a token arriving at a fork is offered on every
outgoing edge as a *forked token* whose base token is consumed when every copy has been taken;
the fork adds no ordering between the branches. *v2/KerML:* SysML v2 §7.17.3 rule 4 (target
multiplicity 1..1 on every outgoing succession), `Actions::ForkAction` duplicates control, not
values (compliance record, "Fork node (1→N parallelism)"). *Runtime:*
`action_executor.go:stepForkNode` spawns one token per outgoing succession in one step; the
branches share the performance's one feature space, which fUML's forked object tokens would not
(a forked value is a copy). `action_fork_branches_share_features` and its trace golden,
`TestActionExecutor_ForkNode`, `TestActionExecutor_ForkNode_SharedFeatureSpace`,
`robustness_test.go:fork_without_a_successor`. The shared space is v2's "one feature space per
performance" rule, quoted in the row above. **agrees.**

**A4. Join.** fUML §8.9.2.18 `JoinNodeActivation`: ready only when every incoming edge has an
offer; it then takes all of them and offers one token on (and a join with `joinSpec` other than
`and` is outside the subset). *v2/KerML:* §7.17.3 rule 3 and `Actions::JoinAction` (compliance
record, "Join node (N→1 synchronization)": the join follows "exactly one performance of every
source — one per succession, not one per token"). *Runtime:* `stepJoinNode` holds a token until
`synchronize` reports every incoming succession delivered; a second token on one succession
waits for the next round (`action_join_same_succession_twice`); a join that can never complete
is `ErrActionDeadlock` (`deadlock_join_starvation`, `deadlock_join_same_succession_twice`).
`action_join_waits_for_slowest_branch`, `action_join_three_two_arrive_together`,
`action_join_one_token_per_incoming_succession`, `TestActionExecutor_JoinNode`,
`TestActionExecutor_JoinNode_PartialArrival`. fUML has no deadlock verdict — an activity whose
join never fires simply never ends; the runtime's typed error is a tool addition where both
texts stop. **agrees.**

**A5. Decision.** fUML §8.9.2.12 `DecisionNodeActivation`: guards on the outgoing edges are
tested against the incoming token's value (or the decision input behavior's result) and the
offer is forwarded "over the edges for which the test succeeds" — every one of them, so where
several guards hold, which edge's target takes the token is left to the acceptance that follows. *v2/KerML:* §7.17.3 rule 2,
`Actions::DecisionAction` "selects exactly one outgoing `HappensBeforeLink`", an `else`
succession is the guardless fallback (compliance record, "Decision node (guarded branching)" and
"`else` branch"). *Runtime:* `stepDecisionNode` evaluates the guards in scheduling-policy order
until one holds and takes that one; the others are previewed in a restored probe
(`probeGuard`), several holding guards are the `ChoiceDecisionBranch` choice point
(`action_choice.go:noteDecisionBranches`), no holding guard and no fallback is
`ErrNoEnabledSuccession`. `action_decision_guarded_branch`, `action_decision_else_branch`,
`action_decision_merge_guarded_branch`, `action_decision_same_declared_target`,
`TestActionExecutor_DecisionNode`. fUML offers to all true edges and lets one be taken; v2 says
exactly one is selected; the runtime selects exactly one and records the alternatives. Which
one, where several hold, is a tool choice in all three texts. **agrees.**

**A6. Merge.** fUML §8.9.2.19 `MergeNodeActivation`: no synchronization — each offer is passed
on as it arrives. *v2/KerML:* §7.17.3 rule 1 and `ControlPerformances.kerml`
`MergePerformance::incomingHBLink: HappensBefore[1]` — every arrival is its own merge
performance (compliance record, "Merge node (N→1 non-blocking)"). *Runtime:* `stepMergeNode`
passes each token on and is the one node kind `synchronize` exempts (`synchronizes`).
`action_merge_loop_reenters`, `action_merge_loop_three_passes`,
`action_merge_fork_branch_and_loop`, `TestActionExecutor_MergeNode`,
`TestActionExecutor_Integration_DecisionMerge`. **agrees.**

**A7. Token order in a step, and same-step writes.** fUML §8.9.1: token flow is inherently
concurrent, the execution model makes no commitment to the order in which ready activations
fire, and the same holds for concurrent writes to one structural feature. *v2/KerML:* two
performances no succession joins are unordered (compliance record, "Concurrent performances are
unordered"). *Runtime:* the order tokens act in one step is the scheduling policy's, recorded as
`ChoiceTokenOrder` (`action_choice.go:noteTokenOrder`), and the write that stands where several
tokens wrote one feature in one step is `ChoiceWriteOrder` (`stepWriteLedger.noteChoices`);
`action_fork_branches_write_one_feature` and its trace golden pin the runtime's pick as
tool-defined, `action_fork_branches_share_features` the order. fUML, v2 and the runtime all
leave the order open; the runtime alone names its pick. **agrees.**

**A8. Streaming.** fUML §8.8.1, §8.8.2.15 `StreamingParameterListener` and §8.8.2.16
`StreamingParameterValue`: a streaming parameter posts values while the behavior runs, a
listener forwards each to the invoker's output pin, and an activity with a streaming output may
stay alive after its last token retires. *v2/KerML:* SysML v2 §7.16.1 — a streaming `flow`
"can be ongoing while both the source and target action are being performed", a `succession
flow` cannot begin until the source completes; `Flows.sysml` `Flow :> Message, FlowTransfer`
and `SuccessionFlow :> Flow, FlowTransferBefore` over `Transfers.kerml`. *Runtime:* both
spellings run as the succession form — a flow is applied once, when the source performance ends
(`applyDataFlows`, A2); the parser keeps the distinction (`ast.Usage.IsSuccessionFlow`),
`lower.ObjectFlow` carries no kind. The roadmap's "streaming flows" entry under Track E records
the limitation, its v2 basis and the work; nothing here re-scopes it. This is a v2 gap that fUML
happens to share a name with — fUML's streaming parameters are not v2's streaming flows, and a
port of them would not close it. **gap.**

**A9. Expansion regions.** fUML §8.10.2.19 `ExpansionRegionActivation`: the region body runs
once per element of the input collection, `iterative` in order or `parallel` as concurrent
activations (`stream` mode is excluded, §7.11.2.6). *v2/KerML:* SysML v2 §7.17.12 `for`
loops perform the body once per element in sequence; the `ForLoopAction` library model has no
parallel form. *Runtime:* `action_for_loop`, `action_for_over_a_part_collection`,
`action_for_over_produced_collections` — sequential only, through the loop node
(`stepStatementNode`). fUML's `iterative` mode is v2's `for`; its `parallel` mode is the
roadmap's "concurrent per-element performance" entry under Track E, which nothing here
re-scopes; the row is counted for the mode the runtime lacks. **gap.**

**A10. Interruptible regions.** fUML §7.1 (Table 7.2 and the list following it):
"Interruptible regions are excluded from fUML" as more appropriate to a higher conformance
level — fUML itself has no interruptible-region semantics. *v2/KerML:* SysML v2 has no
interruptible-region notation either; the nearest is a `do` body, whose outgoing transition
"interrupts" it (§7.18.3, SM14). *Runtime:* none in action bodies; the compliance record lists
"Interruptible regions (spec exists, needs token cancellation)" under *Implementable But Not Yet
Done*, and the roadmap's "interrupting an ongoing performance" entry under Track E scopes it.
Because fUML excludes the concept, a PSSM/fUML port would not supply it: this is a UML 2.5.1
question, not a precise-semantics one. **gap** — with no fUML rule to port.

**A11. Structured nodes: sequence, conditional, loop.** fUML §8.10.2.42
`StructuredActivityNodeActivation` (a nested activation group with its own token flow, its
`mustIsolate` region checked and taken atomically), §8.10.2.12 `ConditionalNodeActivation`
(clauses tested in order, one body run), §8.10.2.23 `LoopNodeActivation` (setup, test, body,
`isTestedFirst`). *v2/KerML:* an action node stating a flow of its own runs it as its
subperformances (`Performances.kerml` `subperformances`; compliance record, "An action node that
states a flow of its own runs that flow"); SysML v2 §7.17.11 `if`/`else`, §7.17.12
`while`/`until`/`for` with `Actions::IfThenElseAction`, `WhileLoopAction`. *Runtime:*
`stepNestedAction` runs a nested node's own flow as a performance of its own (`action_subflow.go`
`Token.positionIn`), `stepStatementNode` runs `if`, `while`, `until` and `for` nodes
(`statements.go:runBlock`, `blockFlow`, `blockNode`), a loop body being a block with a token flow
of its own (compliance record, "A block has a token flow of its own"). `action_nested_flow_two_levels`,
`action_nested_flow_in_fork_join`, `action_nested_flow_guarded_inner_succession`,
`action_if_else_then_branch`, `action_if_else_else_branch`, `action_if_no_else`,
`action_while_loop`, `action_while_loop_zero_iterations`, `action_loop_until`,
`action_loop_until_repeats`, `action_block_flow_perform_in_loop`. fUML's `mustIsolate` has no
v2 counterpart and no runtime one; the runtime's steps are atomic anyway because nothing runs
between two tokens' steps. **agrees.**

**A12. Accept event.** fUML §8.10.2.2 `AcceptEventActionActivation`: on firing, the action
registers an *event accepter* with the object activation and waits; when the pool dispatches a
matching signal occurrence the accepter fires, the action outputs the signal (or a control token
for a trigger with no result) and, if not `isUnmarshall`, offers on its outgoing edges; an
occurrence no accepter matches is lost (§8.8.1). *v2/KerML:* SysML v2 §7.17.8 — an
`AcceptAction` accepts "the transfer of a payload received by the given receiver, and then
output[s] that payload"; `accept ... via`, `accept at`/`after`, `accept when`. *Runtime:*
`stepNestedAction`/`stepActionExecutionNode` park a token at an accept until a matching message
is in flight (`triggerHolds`), then hand it the payload; an unmatched message stays in flight for
a later accept rather than being lost (`action_accept_two_waiters`,
`action_accept_suspends_until_message`, `TestMessageLeftForACompletedMachineDoesNotBlockANewObject`);
an accept every waiting token could have answered is part of `ChoiceTokenOrder`
(`offeredMessage`). `action_accept_message`, `action_accept_subsets_event`,
`action_accept_when_trigger`, `TestAcceptRoutingAgreesBetweenDispatchAndAcceptance`. fUML loses
an unmatched occurrence; the runtime keeps a message in flight for an accept in an action body
while a *state machine* drops what no active state takes (SM1). SysML v2 says a transfer is
accepted by the receiver, not when it is dropped; `Occurrences.kerml`'s `incomingTransferSort`
says which unaccepted transfers "are never to be accepted" and the default sort names none, so
keeping the message is the v2 reading. **differs because v2 differs.**

**A13. Send signal.** fUML §8.10.2.38 `SendSignalActionActivation`: construct the signal
instance from the argument pins, wrap it in a `SignalEventOccurrence` and `send` it to the
target object's event pool — asynchronous, the sender continues. *v2/KerML:* SysML v2 §7.17.7 —
`send <payload> via <sender> to <receiver>` is a `SendAction` whose behavior is "to transfer the
payload from the sender to the receiver"; `Transfers.kerml` orders the transfer after the source
and before the target's acceptance. *Runtime:* `signal.go:Context.send` resolves every destination
before queuing any copy, posts one message per receiving end (`send_connection_multivalued_end_fans_out`),
and a send that reaches no receiver is `UnroutableSendError` (`send_no_reachable_receiver`,
`send_identity_unroutable_target`) rather than a message dropped. `action_send_accept`,
`TestSendViaPortReachesConnectedAccept`. The unroutable-send error is where PSCS's "the event
occurrence is lost" (C8) and the runtime part: v2 gives the transfer a receiver and says nothing
about a send with none, so the runtime's refusal is a tool choice. **agrees** on the send that
routes; the unroutable case is C8's.

**A14. Call behavior and call operation.** fUML §7.11.2.3 and §7.11.2.4 require
`isSynchronous = true` for both; §8.10.2.7 `CallBehaviorActionActivation` runs the behavior to
completion (or as a nested execution whose streaming outputs are forwarded) and then offers its
results; §8.10.2.8 `CallOperationActionActivation` dispatches the operation on the target object
through its `dispatch` strategy and runs the method the same way. *v2/KerML:* SysML v2 §7.17.6
`perform` performs the named action as a subperformance (`Actions::Action :> Performance`,
`subactions :> subperformances`); an operation call event is `accept op(args)` on a transition
(SM17). *Runtime:* `perform` is a nested performance of its own with parameters bound by name
(`action_frame.go:performances.bindArguments`, `performInvocation`;
`action_perform_reference`, `action_perform_shorthand`,
`perform_action_binding_package_sibling`); `invoke_operation.go:Context.InvokeOperation` runs a
member of an object's type with the object as performer, synchronously, and returns the outputs
(`TestInvokeOperationPerformedByTheObject`, `TestInvokeOperationFailureModes` in
`classifier_behavior_test.go`); a call event reaches a machine through the same entry
(`state_call_trigger`, `state_call_trigger_guard`, `state_call_trigger_regions`). Arguments bind
by name only; the roadmap's "operation invocation with positional arguments" entry under Track E
holds the positional form. Both fUML calls and both runtime paths are synchronous and
by-position versus by-name is notation, not semantics. *The caller's return:* PSSM §8.5.9
(`CallEventOccurrence`, `SM_ObjectActivation`) releases the caller of a synchronous call once the
run-to-completion step (§8.5.10) the call event triggers is done, with the return values the triggered
behaviors — the transition's effect, an entry or an exit — wrote to the operation's output
parameters, the last write winning. `StateExecutor.Call` (`perform.go`) does the same: it queues
the call event, runs the machine at the current instant through the step dispatching it and no
further — a completion event that step queued or a timer it armed is the machine's next step,
after the caller has resumed, as §8.5.10 makes each occurrence its own step, while a call a
state defers holds its caller through the steps until the machine recalls and dispatches it, as
§8.5.9's blocked caller waits for the deferred occurrence — collects what a
behavior the event fires `return`s or assigns to an output parameter of that name, and hands the
outputs back typed and by name — under the `out`/`inout` parameters the operation declares as a
member of the machine's owner (or of the machine standing alone) when it declares one — among
several so named, the one the call's arguments select as `InvokeOperation` would, and
`ErrAmbiguousInvocation` before the call is queued when they select none; the arguments
checked against the declaration's inputs as `InvokeOperation` checks them, so an unbound or
unknown one is `ErrUnboundParameter` and one of the wrong type `ErrTypeMismatch` before the call
is queued, and an input the caller omits carries its default, since §8.5.9's
`CallEventExecution` holds a value for each of the operation's parameters; the call event carries
the selected declaration (`Call.Declared`) and fires only the triggers naming it — a trigger
`accept op(x)` naming the owner's `op` declarations with an input for each trigger parameter,
those with exactly the trigger's parameters when any has, and all of several differing in
their parameters' types alone, since the notation writes no types (`callTriggerOperations`), as
a UML `CallEvent` names one `Operation` (UML §13.3.3) — so same-named overloads whose parameter
names differ reach their own transitions — as §8.5.9
returns the operation's own parameters — an `inout` no behavior of the step wrote going back as the
caller passed it, since §8.5.9's `CallEventExecution` holds the argument as that parameter's value
until a behavior writes it — and under every name the step returned when the trigger
names no declared operation; a call the run left queued or deferred is `ErrCallNotReturned`,
since its caller would still be waiting, and an unhandled call returns nothing, as PSSM's
discarded occurrence does (`state_call_trigger_results`; `TestRuntimeRobustnessCallResults`:
held, recalled, untaken, empty, repeated and erroring calls, the caller released ahead of the
completion step and the timer its step set up, a declared operation returning its own
parameters alone and an `inout` argument as passed, as written and not at all when nothing takes
the call, a wrong-typed argument refused, a default filled in, an overload firing the trigger
naming its declaration). Only a write to a behavior's own output parameter comes back — a nested action's
`return` or its assignment to an `out`/`inout` (`state_statements.go:returnAround`) — because
§8.5.9 collects the values of the triggered Behavior's output parameters and nothing else; an inline
`assign` in an entry, exit or effect writes a feature of the machine's owner, not a parameter, so it
stays state and is no result even when its name coincides with a declared `out`.
**agrees.**

**A15. One firing per token, or one performance per node.** fUML §8.9.1 and §8.10.1
(`ActionActivation::fire`, `isReady`, `takeOfferedTokens`): an action whose input pin has
multiplicity 1 takes one object token per firing, and an action offered several tokens on such
a pin fires once per token — `ActionActivation::fire` sends its offers, then fires again while
`isReady` still holds and `takeOfferedTokens` yields tokens — so a decision fed two values
routes each on its own, and a node re-fires for every value a fork delivers to it. *v2/KerML:*
an action node reached over several successions is one performance that follows all of them
(A1; KerML 1.0 §7.4.5 — a step with no declared multiplicity holds one value), and every flow
into that performance delivers to the one input feature of that one performance ("One feature
space per performance" in the compliance record). *Runtime:* `action_executor.go:synchronize`
holds the arrivals and steps the node once with every delivery in hand; a second delivery to a
multiplicity-1 input is not a second performance (`action_node_concurrent_performances` and its
trace golden), and a multi-valued one to it is a multiplicity violation. The fUML
referee (`docs/project/fuml-referee.md`) detects the fUML side of this row in the reference
implementation's trace — one action fired more than once within one execution, with an object
flow feeding it — and files the activity as `differs-by-design`: `DecisionJoin`,
`ForkMergeData`, `TestSimpleActivities` through both, and `TestBooleanFunctions`, whose
four-row truth tables reach each function through a multiplicity-1 pin. **differs because v2
differs.**

### Composite structures (PSCS)

PSCS (formal/19-02-01) §8.1 extends fUML with runtime manifestations of parts, ports and
connectors (§8.4: `CS_Object`, `CS_InteractionPoint`, `CS_Link`, `CS_Reference`), two semantic
strategies (§8.4.2.9 `CS_RequestPropagationStrategy`, §8.6.2.6 `CS_ConstructStrategy`) with
default realizations, and port-aware variants of send, dispatch and call (§8.6). The v2 side is
KerML 1.0 §7.4.6 (connectors are features typed by associations whose values are links, "limited
to linking things identified via related features on the same instance of the connector's
domain") and SysML v2 §7.12–§7.14 (ports, connections, interfaces); the runtime side is
`instance.go`, `connector.go`, `binding.go` and `routing.go`, recorded in the Structural map of
`docs/project/spec-compliance.md`.

**C1. Parts: default construction to the lower bound.** PSCS §8.6.2.9
`CS_DefaultConstructStrategy`: "Parts (i.e., composite Properties) are instantiated according to
their multiplicity lower bound"; a lower bound of 0 yields an empty topology (Figure 8.12). The
strategy runs when a `CreateObjectAction` targets a class with a default constructor
(§8.6.2.4). *v2/KerML:* KerML 1.0 §7.4.5 multiplicity; a composite feature's values are parts of
the featuring object (`Objects.kerml` `Object::subobjects`). *Runtime:* `instance.go:Context.Instantiate`
creates the object and `Instance.GetFeatureValue` materializes a composite feature on first read
to its required lower bound — `part cell : Component[4]` holds four objects, an optional `[0..1]`
holds none, a bound past `maxMaterializedLowerBound` is `ErrMultiplicityViolation` (compliance
record, "A required lower bound is materialized eagerly", "A bracket multiplicity is the
population", "An optional composite feature fills to its lower bound like a collection";
`robustness_test.go:multiplicity_infinite_lower_bound`, `:multiplicity_lower_bound_too_large`,
`instance_test.go`). Materialization is on first read rather than at construction, which no
observable trace distinguishes because a feature is read before it is used. **agrees.**

**C2. Ports as interaction points.** PSCS §8.6.2.9: "Instantiation of a value for a Port results
in the creation of a `CS_InteractionPoint`, which itself refers to a `CS_Object` typed by the
type of the Port" — a port is a reference to an object standing in for the owner, not a value of
its own; a port typed by an interface gets a dynamically generated realizing class. *v2/KerML:*
SysML v2 §7.12.1 — a port is a usage whose definition's features are directed (`in`/`out`/
`inout`), a conjugated port reverses them (§7.12.3), and a port is an object of its own
(`Ports.sysml` `abstract port def Port :> Object`). *Runtime:* a
port is a feature of the owning object whose value is an object of the port definition
(`GetFeatureValue`, as for a part), and its directed features decide which way a send through it
travels (`routing.go:receivingEnds`, `outbound`; `port_direction_conjugation`,
`port_interface_typed_connection`, `send_into_outbound_only_end`). PSCS's realizing-class step is
UML's interface machinery; v2 types a port by a port definition directly. **agrees.**

**C3. Connector instantiation.** PSCS §8.6.2.9: "Instantiation of Connectors occurs only for
binary Connectors", one `CS_Link` per pair of connected part/port values, following the *array*
pattern (equal lower bounds, end multiplicity 1: element *i* to element *i*), the *star* pattern
(end lower bounds equal the connected parts' lower bounds: every element to every element), or
none where the end lower bound is 0 (§9.4, Figure 9.4). *v2/KerML:* KerML 1.0 §7.4.6 — a
connector's values are links, one per related pair the ends identify; SysML v2 §7.13.1 — "a
connection usage redefines the connection ends from its definition, associating those ends with
the specific usage elements", the single values of `[1]` ends being connected. Neither text names
array and star. *Runtime:* `connector.go:Context.materializeConnector` creates *one* connector
object whose ends hold the whole value of each connected feature — an end reached through a
multi-valued feature denotes every element it holds (`send_connection_multivalued_end_fans_out`),
an end with no value materializes no link of its own (`TestOptionalConnectorLinksNothingOfItsOwn`),
n-ary connectors are kept whole (`TestNaryConnectorKeepsEveryEnd`,
`action_port_communication_nary`), an end is the connected feature itself, not a copy
(`connector_end_identity`, `TestConnectorEndsAreTheConnectedFeatures`,
`TestWritingAConnectedPortIsReadThroughTheEnd`). One object standing for every pair is the
runtime's representation; the links PSCS enumerates are the pairs that object's ends span. The
star pattern (every element to every element) is what a multi-valued end fans out to; the array
pattern (element *i* to element *i*) has no spelling in either text and no runtime rule.
**differs, v2 silent** — on the array pattern only; the rest agrees.

**C4. Delegation connectors.** PSCS §8.4.2.7 `CS_Object.sendIn`/`sendOut`: a request arriving at
a port of the composite that is not a behavior port is forwarded along the connectors from that
port to the parts' ports inside (`selectTargetsForSending` with `toInteractionPoint`), the
composite's own behavior never seeing it; the delegation connector is a `CS_Link` like any other,
the direction of propagation decided by which side the request came from. *v2/KerML:* SysML v2
§7.13.1 — a connection between a boundary port and a nested part's port is a connection usage
like any other; §7.13.3 `bind` makes two features hold the same value; the library has no
"delegation" kind. *Runtime:* two spellings. A `connect boundary to inner.port` is a connector
whose ends route (`routing.go:connectedDeliveries`, `endDeliveries`;
`send_connection_into_nested_part`, `send_via_owner_nested_port_path`,
`send_via_owner_port_from_nested_part`, `TestSendViaPortToNestedReceiver`). A `bind boundary =
inner.port` makes the two ports *one object* (`binding.go:resolveBindings`;
`binding_chained_object_ends` — "bindings chained through two levels of assembly join three ports
into one object"), so a message at the boundary is at the inner port with no propagation step
(`send_bind_relay_inbound`, `send_bind_relay_outbound`, `send_bind_relay_nested`,
`send_bind_relay_connector_order`, `send_bind_relay_aliased_ends`). PSCS forwards a copy along a
link; the runtime either routes along a connection or identifies the ports. Same receivers, same
payload, one fewer hop in the binding case; no trace distinguishes them. **agrees.**

**C5. Assembly connectors.** PSCS §8.4.2.7 `CS_Object.sendOut`: a request leaving a part's port
is propagated along the connectors from that port to the ports at their other ends, which
`sendIn` the request there; §9.4's `TestCase_Assembly_P_P` pins part-to-part communication.
*v2/KerML:* SysML v2 §7.13.1 connection usages between the ports of two parts of one owner,
§7.14.1 interfaces typing them with conjugated ends. *Runtime:* `routing.go:performerConnections`
collects the connections of the sender's owner that the sending port is an end of,
`receivingEnds` picks the other ends, `ownerDeliveries` delivers one copy per receiving end held
to that end's object identity (`send_via_owner_connection`, `action_port_communication`,
`action_port_communication_nary_anonymous`, `TestSendViaPortReachesConnectedAccept`,
`TestPortRoutedMessageBypassesPortlessAccept`, `TestSendViaPortRoutesInEitherDirection`); a
variant's selected connection is the one that routes (`variant_connection_per_owner`,
`TestRoutingHonorsTheSelectedVariantConnection`, `TestSelectedVariantInterfaceIsRealized`).
**agrees.**

**C6. Behavior ports and the owner's behavior.** PSCS §8.4.2.7 `CS_Object.sendIn`: "if the
interaction point is a behavior port, the event occurrence is sent directly to the target
object" — the owner's classifier behavior receives it; a non-behavior port only forwards. UML
2.5.1 `Port::isBehavior` is the switch. *v2/KerML:* no `isBehavior`; SysML v2 §7.17.8 `accept
Sig via port` in the owner's behavior is what makes a port the owner's own receiver, and a
transition's `accept ... via` the same for a state machine. *Runtime:* a message routed to a port
is taken by an accept naming that port (`triggerHolds`, `endReceivesMessage`), and an accept
naming no port takes only a message routed to the owner itself
(`TestPortRoutedMessageBypassesPortlessAccept`, `TestAcceptRoutingAgreesBetweenDispatchAndAcceptance`,
`action_send_invocation_via_port`). The same port can be accepted on by the owner and connected
inward to a part; which of the two takes a message is decided by the accept that matches, not by
a property of the port. **differs because v2 differs** — v2 makes "behavior port" a property of
the accept, not of the port.

**C7. Signal broadcast versus first target.** PSCS §8.4.2.1 `CS_DefaultRequestPropagationStrategy`:
"If the request concerns the emission of a Signal and there are multiple possible targets, the
signal is broadcasted to all the targets. If the request concerns an Operation call and there
are multiple possible targets, the call is propagated to the first target." *v2/KerML:* SysML v2
§7.17.7 transfers the payload "from the sender to the receiver"; a connection end over a
multi-valued feature denotes every value (SysML v2 §7.13.1, KerML §7.4.6); no operation call
travels over a connection in v2 — an operation is invoked on an object. *Runtime:* a send fans
out to every receiving end (`send_connection_multivalued_end_fans_out`, "one send through the
console's port delivers one message per unit"), which is PSCS's broadcast; an operation call
goes to one object by `InvokeOperation` and never selects among connector targets, so PSCS's
"first target" rule has nothing to apply to. **agrees** on signals; the operation-call half is
moot in v2 and is not counted as a difference.

**C8. A request with no target.** PSCS §8.4.2.7 `sendIn`/`sendOut` and §8.6.2.4
`CS_CallOperationActionActivation`: when the propagation strategy selects no target "the event
occurrence is lost" / "the operation call is lost" — silently. *v2/KerML:* silent; SysML v2
§7.17.7 assumes a receiver. *Runtime:* `signal.go:Context.send` refuses with
`UnroutableSendError` at the send ("reported where it was written, rather than dropping the
message and leaving the accept to time out": `send_no_reachable_receiver`,
`send_identity_unroutable_target`), and a send to a port whose type cannot carry the payload with
`SendPortTypeMismatchError`. PSCS drops; the runtime errors. **differs, v2 silent.**

**C9. Interface-typed ports and dispatch.** PSCS §8.4.2.2 `CS_DispatchOperationOfInterfaceStrategy`
and §8.4.2.6 `CS_NameBased_StructuralFeatureOfInterfaceAccessStrategy`: an operation called on an
interface-typed port is dispatched to the realizing class's operation of the same name, and a
structural feature of the interface is accessed by name on the realizing object. *v2/KerML:*
SysML v2 §7.14 interfaces type *connections*, not ports; a port is typed by a port definition
whose directed features are the port's own (§7.12.2), and a port's feature is read or written as
a feature (§7.13.4). *Runtime:* a port's features are feature values of the port object,
written through one end and read through the other (`TestWritingAConnectedPortIsReadThroughTheEnd`,
`port_interface_typed_connection`), and an interface's end ports must be conjugates (compliance
record, "The ports at the two ends of an interface must have conjugate directed features"). The
name-based realization step is UML's interface/class split, absent from v2.
**differs because v2 differs.**

**C10. What the runtime binds that PSCS links.** PSCS has no binding connector: two features that
must hold one value are two ends of a `CS_Link` and stay two values. *v2/KerML:* KerML 1.0
§7.4.6 "Binding connectors are binary connectors that require their source and target features
to have the same values on each instance of their domain"; SysML v2 §7.13.3 `bind`, §7.13.4
feature values. *Runtime:* `binding.go:resolveBindings` gives both ends one value on first read
of either, in either direction (`binding_value_forward`, `binding_value_reverse`,
`binding_object_end`, `binding_nested_end`, `binding_nested_end_reverse`,
`binding_multivalued`, `binding_expression_end`, `binding_calc_result`), refuses two disagreeing
bindings at one feature (`ErrBindingConflict`), and treats a binding cycle and an undetermined
end as errors rather than guesses; a binding to a destroyed end is refused
(`TestBindingRefusesADestroyedEnd`). The compliance record lists "Binding and flow connector
objects" as *Implementable But Not Yet Done*: the ends reach routing, but neither is materialized
as a connector object of its own. Nothing in PSCS would supply that object; it is a v2 concept.
**agrees** — PSCS has no rule here to differ from, and the runtime's binding is v2's.

## The count

Seventy rows: 45 for state machines, 15 for actions, 10 for composite structures. Each
carries one verdict.

| Verdict | Rows |
|---|---:|
| **agrees** | 50 |
| **differs because v2 differs** | 7 |
| **differs, v2 silent** | 10 |
| **gap** | 3 |
| **Total** | **70** |

The seven **differs because v2 differs** rows are SM15 (a do activity and the machine competing
for one occurrence), SM36 (local transitions), SM37 (internal transitions), A12 (accept event:
a message no accepter takes stays in flight), A15 (one firing per token: fUML re-fires an
action for every token on a multiplicity-1 pin, the runtime performs the node once with every
delivery), C6 (behavior ports) and C9 (interface-typed ports and name-based dispatch). On each, SysML v2 or the Kernel Semantic Library states the rule the
runtime follows, quoted in the row; adopting PSSM, fUML or PSCS there would move the runtime
away from the specification it implements, so none of them is a candidate for a port.

The ten **differs, v2 silent** rows, the only ones on which a port could change behavior
without contradicting v2:

- **SM7** — a deferrable occurrence that also enables a transition in an enclosing state or a
  sibling region: PSSM defers it unless the transition is more deeply nested than the deferring
  state, the runtime lets any enabled transition consume it.
- **SM9** — when a completion transition's guard is read: PSSM reads it when the completion
  occurrence is dispatched, the runtime once, at completion; the library orders the guard within
  the source's performance but fixes no instant.
- **SM11** — what the completion of a composite state completes: PSSM fires the composite
  state's own completion transition and the machine goes on, the runtime propagates the completion
  outward to the machine and never fires a completion transition out of a composite state.
- **SM28** — history with nothing to restore and no default history transition: PSSM enters the
  region's initial pseudostate, the runtime refuses the run.
- **SM30** — choice guards: PSSM reads them on arrival, the runtime reads them before the step,
  as for a junction.
- **SM32** — a junction none of whose outgoing guards holds, or a join whose only way out is
  guarded false: PSSM disables the compound transition and the occurrence is deferred or lost,
  the runtime selects the incoming transition and fails the run.
- **SM34** — the orders within a join's firing: PSSM fires each incoming segment on its own
  occurrence, so two completion transitions into one join fire in the order their sources
  completed, and leaves the owner with the last source, before that segment's effect; the
  runtime holds the segments until the join is ready, draws their order, and leaves the owner
  after the last effect.
- **SM45** — destroying an object whose behavior is still performing: fUML stops the behavior and
  destroys, the runtime refuses the destruction.
- **C3** — a connector between multi-valued ends: PSCS instantiates one link per matching pair
  (array pattern) or a full cross product (star pattern), the runtime one connector object whose
  ends span the collections.
- **C8** — a send that reaches no receiver: PSCS loses the occurrence, the runtime fails the
  send with a typed error.

The three **gap** rows:

- **A8** — streaming flows (the roadmap's "streaming flows" entry).
- **A9** — parallel expansion regions (the roadmap's "concurrent per-element performance"
  entry).
- **A10** — interruptible regions (the roadmap's "interrupting an ongoing performance" entry).

Every gap is already a Track E entry with a v2 basis of its own; a port of the precise-semantics
text would not close any of them, because each is a v2 concept the runtime lacks, not a UML
concept v2 lacks. All three are fUML rows no PSSM test reaches; terminate (SM38), the one gap
the suite tested directly, is closed.

## The test suite as a referee: a capability map

The question this section answers is the one the pilot execution referee's note asks of the
pilot: how far can the PSSM test suite adjudicate the runtime's behavior, and what would a
verdict from it mean? The comparison assumes a harness that translates each test's UML model
into a `.sysml` model by hand or by rule, drives it with the test's stimulation sequence, and
compares what the translated model records with the expected trace(s). That harness is
`cmd/pssm-referee` (`docs/project/pssm-referee.md`); the counts below are its classifier's,
which supersede the hand count this section was first written with — the moves are listed under
"Moves from the hand count".

| Aspect of the suite | Verdict | Why |
|---|---|---|
| **Expressing the test model in SysML v2 textual notation** | **Can, for 66 of 103** (35 with standard notation, 31 with this project's extensions); **cannot, for 37** (29 use a construct v2 has no spelling for, 8 more use a behavior shape the translation does not spell) | Every test's state machine is classified by the UML constructs it uses; the table below gives the construct-to-notation mapping and the per-area result |
| **Driving the test** | **Can, in the tester's order** | PSSM's `Tester` sends `Start` and the follow-up signals from its own behavior, interleaved with the target's steps by fUML's scheduling, and blocks on each operation it calls until the call's run-to-completion step is done (§8.5.9, `CallEventOccurrence`). The referee's driver (`tools/referee/pssm/run.go:drive`) performs the tester's steps in that order: a send is queued where the tester sends it, a call is `StateExecutor.Call` and returns the operation's outputs, and a `trace(...)` of the tester's own is appended to the target's `log` where the tester makes it, once the call it embeds has returned. The sends coincide with the conformance harness's queued `events` when every send precedes the target's first reaction, which is what the tests' "received when in configuration ..." lists state; a test that needs a signal to arrive mid-run needs a tester `part` in the model instead |
| **Comparing the expected trace** | **Can, on a model-level string; `%trace` is not the comparand** | PSSM's expected trace is built by the model — every entry, exit and effect behavior calls `trace("<state>(entry)")` on the `TraceBuilder` (501 call actions target the `trace` operation in the XMI). Its translation is an `assign log := log + "<state>(entry)"` in the corresponding `entry`/`exit`/`do` body, compared through the case's `slots`/`outputs`; the runtime's `%trace` and `TestExecutionTrace` goldens record steps, not segments, and would need a projection (enter/exit/effect lines to segments, everything else dropped) to be comparable at all |
| **Alternative expected traces** | **Can, and exactly** | 36 tests declare more than one admissible trace. The conformance schema's `outcomes` with the `explore` policy replays a case once per linearization of its choice points (`ChoiceRegionOrder`, `ChoiceTransition`, `ChoiceDueOrder`) and fails when a listed outcome is unreachable or an unlisted one is reached — the same set-equality PSSM's alternatives ask for, and stricter than the single-run comparison the PSSM harness performs |
| **The run-to-completion step table** | **Cannot compare** | Each test's "RTC steps" table lists the pool's contents and the fired transitions per step, including completion events (`CE(<state>)`). The runtime has no pool of completion occurrences (SM9) and the `%trace` records no pool; only the fired transitions and the final trace are comparable |
| **A pass as evidence about SysML v2 semantics** | **Only on the ten `differs, v2 silent` rows and as corroboration on the `agrees` rows** | Where PSSM and v2 coincide (50 rows) a pass says the runtime does what both texts say — worth having, but not a second opinion on v2. Where they differ because v2 differs (6 rows) the corresponding tests fail by design and their failure means nothing. Where v2 is silent (10 rows) a pass or a fail reports on a tool choice, which is the one place the suite is informative about this runtime's rules |

### Which UML construct maps to which notation

| UML construct in the tests | SysML v2 spelling | Class |
|---|---|---|
| State machine as classifier behavior, states, initial pseudostate, final state | `state def`/`state`, `exhibit state`, `entry; then s`, `then done` (§7.18.2) | standard |
| Entry, exit and do behaviors | `entry action`, `exit action`, `do action` (§7.18.2) | standard |
| External transition with a signal trigger, guard and effect | `transition first s accept Sig if g do { … } then t` (§7.18.2, §7.18.3) | standard |
| Completion transition | `transition first s then t` / `succession first s then t` with no accepter (§7.18.3) | standard |
| Orthogonal regions | `parallel` states (§7.18.1) | standard |
| Call event trigger | `accept op(args)` on an operation invocation, as `state_call_trigger` spells it | standard, with the caller-return caveat of A14 |
| Deferrable trigger | `defer Sig;` — this project's extension | extension |
| Fork, join, junction, choice, shallow and deep history pseudostates | State-body `fork`/`join`/`junction`/`choice`/`history`/`deep history` — this project's extensions | extension |
| Terminate pseudostate | A terminate action usage in the region, `action t terminate;`, that a transition ends at with `then t` (§7.18.3; SM38) | standard |
| Entry point, exit point (connection points and connection point references) | none in standard notation. The migrator's reading, established since this table was drawn: a composite state's entry or exit point is a `junction` of that state (a `fork`/`join` when its transitions each start, or come from, a different orthogonal region), reached by path — `then Work::start;`, `first Work::leave then Idle;` — so the runtime runs the state's entry behavior before the junction's outgoing transition, and the transition into the junction before the state's exit behavior, the order UML §14.2.3.4.5 and PSSM §8.5 give connection points; see [sysml-v1-migration.md](../../reference/sysml-v1-migration.md#behaviors). The referee's classifier still counts the construct here, so the table below is unchanged until the referee is rewired onto the migrator | no spelling (referee); extension (migrator) |
| Local transition, internal transition | none (SM36, SM37) | no spelling |
| State machine generalization: extended regions, redefined transitions | none | no spelling |
| Entry or do behavior with parameters, or a transition effect with parameters, reading the triggering event's data | the notation binds event data on the transition (`accept d : Data`, `accept op(p1, p2)`, §7.18.2; `TransitionPerformances.kerml`'s `accepter`), never on an `entry`/`do` action, so the accepting transition's effect stores each value in an attribute of the machine (`assign trigger_Data := data;`) and the action declares its parameter bound to it (`in data : Data = trigger_Data;`); an effect reads the `accept`'s own parameters. Holds when every transition entering the state accepts the one event whose data conforms to the parameters — the reading is recorded under [Behavior parameters](#behavior-parameters-operation-results-tester-traces-and-standalone-machines) below | standard |
| Exit behavior with parameters, reading the leaving transition's data | the exit's parameters are bound to the leaving transition's own payload, `exit action { in data : Data = 'T1.2'.data; }` — the transition's `accept d : Data` declares `d` a feature of the transition (§7.18.2, `StateTransitionAction::payload` bound to `accepter.payload`), the exit is a step of the transition performance that accepted it (`StatePerformances.kerml`: `accept then transitionLinkSource.exit`, `TransitionPerformances.kerml`: `transitionLinkSource then effect`), and a transition not being taken reads as nothing; an exit several transitions leave spells `T3.d ?? T4.d`, each binding on its own firing, and an outer transition's payload binds the nested exits it performs (PSSM §8.5.5 `exit`). Holds when every triggered transition leaving the state whose data the signature takes accepts the one event; a leaving path binding nothing (a completion, PSSM §8.5.5) binds nothing and leaves the parameter its default — see [Behavior parameters](#behavior-parameters-operation-results-tester-traces-and-standalone-machines) | standard |
| Entry or do behavior with parameters reached by a completion transition, or by paths accepting different events | none: a completion binds nothing in UML (PSSM 8.5.5) while an attribute the last accepting transition stored would be stale, so the state's action cannot spell both; no test of the suite has the shape | no translation |
| Call event whose operation returns a value the tester traces | the runtime returns the outputs the triggered behaviors wrote to the caller (A14, `StateExecutor.Call`) and the driver traces them where the tester does; the producing behavior — an effect or entry with an `out`/`return` parameter — is an `action def` with `out` parameters named as the operation's (§7.16.2), used by the effect or entry with its inputs bound as above, and the runtime returns what it assigned. A do activity with outputs has no caller left to return to when it runs — see below | standard |
| A `trace(...)` call in the tester's own behavior | the tester is the referee's driver, not a model element: its trace is appended to the target's `log` where the tester makes it, once the call it embeds has returned (`run.go:drive`, `tracer.value`); a trace that embeds no call and does not follow one is refused, since the machine may still be running (`stimulation.go:traceStimulus`) | standard, driven |
| The UML `StateMachine` as the class under test (a standalone machine with attributes, operations, a constructor) | `part def` with `attribute`s, `action def`s and `exhibit state`, as an owned machine's: the reader (`reader.go`) reads the machine as the `Target` whose `Machine` is itself, with its attributes, operations and their methods, and its constructor | standard |
| A guard whose behavior acts on the model (calls `trace(...)` before returning its value) | none: a v2 guard is a Boolean expression (§7.18.3, `validateTransitionFeatureMembershipGuardExpression`; `bool guard[*]` in `TransitionPerformances.kerml`, the effect a separate `step`), and an expression has no spelling for an action. UML 2.5.1 §14.5.11 `Transition::guard` itself calls such a guard ill formed. Recording the runtime's guard reads as the referee's observable instead is refused too: the reads the suite traces are a junction's on the target's default entry, read at the incoming transition's selection, where the library orders every transition inside a state after the state's `entry` — see [A guard whose behavior acts on the model](#a-guard-whose-behavior-acts-on-the-model) | no translation |
| A guard whose behavior is an opaque behavior, not an activity | none: the reader follows an activity's nodes to tell whether the behavior acts, and does not read an opaque body, so the guard is refused rather than carried as its Boolean text alone. A `FunctionBehavior` is the exception — it accesses no object by UML's contract (§13.2.3.3) — and is translated as the expression it spells | no translation |
| Fork into states of orthogonal regions that have no initial pseudostate | `parallel` regions spell the shape and the `fork` extension the fork; a region a fork enters needs no `entry; then` (finding 6 below, fixed) | extension |

The classification is by construct, in the order of the table: a test whose model uses any
construct with no spelling or no translation is counted as not expressible whatever else it
uses; otherwise it is counted under the extensions if it uses any of them, and as standard
otherwise (the three terminate tests were a class of their own, *terminate gap*, while the
runtime did not execute `terminate`; they are standard since it does). `internal/pssm/classify.go`
is the classifier and `TestSuiteClassification` pins this table against the pinned suite. By
area:

| Area | Tests | Standard | Extension | Not expressible |
|---|---:|---:|---:|---:|
| Behavior | 5 | 4 | 0 | 1 |
| Transition | 15 | 8 | 1 | 6 |
| Event | 16 | 16 | 0 | 0 |
| Entering | 5 | 4 | 0 | 1 |
| Exiting | 5 | 4 | 0 | 1 |
| Entry (entry points) | 6 | 0 | 0 | 6 |
| Exit (exit points) | 3 | 0 | 0 | 3 |
| Choice | 5 | 0 | 4 | 1 |
| Junction | 6 | 0 | 5 | 1 |
| Fork | 2 | 0 | 1 | 1 |
| Join | 3 | 0 | 3 | 0 |
| Final | 1 | 1 | 0 | 0 |
| Terminate | 3 | 3 | 0 | 0 |
| History | 8 | 0 | 8 | 0 |
| Deferred | 10 | 0 | 10 | 0 |
| Redefinition | 6 | 0 | 0 | 6 |
| Standalone | 3 | 1 | 0 | 2 |
| Other | 1 | 0 | 0 | 1 |
| **Total** | **103** | **41** | **32** | **30** |

Of the 29 with no v2 spelling, 14 use an entry point, 12 an exit point, 9 a local transition, 2
an internal transition and 6 the redefinition machinery (several use more than one). Of the
expressible tests, 20 use orthogonal regions, 9 a do activity, 9 deferral, 8
history, 6 a junction, 4 a choice and 5 a fork or join; seven (*Event 019-A* to *019-E*,
*Deferred 007*, *Standalone 003*) have a call event the tester calls synchronously, five of
them tracing after it and four of those its result. Three with a call event (*Event 019-B*,
*019-C*) or a signal (*Event 017-B*) have an exit behavior that reads the leaving transition's
data.

#### Moves from the hand count

This section was first written with a hand count of 37 / 33 / 3 / 30, which classified by the
state-machine constructs alone. Writing the emitter showed nine of those 73 tests to have no
exact translation, for reasons the construct table did not list; two of the nine (*Fork 002*,
*Join 001*) have one since the lowerer accepts a fork-entered region without an initial, a
third (*Event 019-A*) since the driver performs the tester's calls and traces in the tester's
order, and four more (*Event 019-D*, *019-E*, *Deferred 007*, *Standalone 003*) since the
emitter binds an entry's or effect's parameters from the accepted event's data and returns
what the behavior assigns to its outputs, and three more (*Event 017-B*, *019-B*, *019-C*)
since an exit's parameters read the leaving transition's payload; adjudicating the failures
found a tenth. Each is
recorded here with the classifier's reason; the count ratchet in `docs/project/pssm-referee.md`
is where a later translation moves them back.

| Test | Was | Reason |
|---|---|---|
| *Event 017-B* | standard | *translated since the exit reads the leaving transition's payload:* the nested state's exit behavior `exit(data)` reads the data of the second `Data` occurrence, the one firing `T1.2` out of it (its exit traces `[in=false]` after an entry with `true`); an attribute the entering `T2` stored would give `[in=true]`, a trace the suite does not admit, so the exit's parameter is bound to `'T1.2'.data`, the transition's own payload, read while `T1.2` is taken. The composite's and the nested state's entries and the nested do activity bind from `T2`; the two admitted traces, with and without the do activity's segment, are both reached and nothing else |
| *Event 019-A* | standard | *translated since the driver performs the tester's steps in order:* the tester itself calls `trace("End")` after the target's operation returns; the driver now makes the call synchronously and appends the trace to `log` when it returns, reaching the one admitted trace: the source's exit, the call transition's effect, `End`, the next state's segment |
| *Event 019-B* | standard | *translated since the exit reads the leaving transition's payload:* the source state's exit behavior `exit(p1, p2)` reads the arguments of the `op(42, "input")` call that fires `T2` out of it, and no transition entering the source (the initial's) carries them; its parameters are bound to `T2.p1`, `T2.p2`. `T2`'s effect and the target state's entry bind from the call; the one admitted trace is reached |
| *Event 019-C* | standard | *translated since the exit reads the leaving transition's payload:* the innermost state's exit behavior `exit(p1 : Boolean)` reads the argument of the `op2(true)` call that fires `T1.1.2` out of it, bound to `'T1.1.2'.p1`; the data entering it is `op1`'s `(42, "input")`, of other types, and the three entries bind from `op1`. The exit activity's `ToString` call that nothing feeds is left out, as UML never executes it; the one admitted trace is reached |
| *Event 019-D* | standard | *translated since the emitter spells a returning effect:* `T2`'s effect is an action with an `out` parameter named as `op`'s (`out output : String`), its `return "output"` an assignment to it; the runtime returns the assigned value to the caller and the driver traces it as the tester does, reaching the one admitted trace |
| *Event 019-E* | standard | *translated since the emitter binds entry parameters and returns outputs:* `T2` accepts `'or'(left, right)` and stores both in `trigger_v_or_left`, `trigger_v_or_right`; the two regions' substate entries declare `in left = trigger_v_or_left; in right = trigger_v_or_right;` and assign the operation's two outputs (`out result`, `out 'return'`), each region's entry writing them in the order the regions are entered, so the caller receives the second region's values and the two admitted traces are both reached and nothing else |
| *Deferred 007* | extension | *translated since the emitter spells a returning effect with inputs:* the deferred `op(p1)` call fires `T4` once the second state is active; `T4`'s effect is `action def T4_effect` with `in p` bound to the accept's `p1` and `out 'return'`, its `return` an assignment of `not p`; the one admitted trace, the first state's exit, `T3(effect)`, `T4(effect)[in=true][out=false]` and the tester's `[out=false]`, is reached |
| *Standalone 003* | standard | *the standalone machine is read as the target since the reader does so, and translated since the emitter binds entry parameters and returns outputs:* the same shape as *Event 019-E*, the machine itself the class under test with `or` its operation; both admitted traces are reached |
| *Fork 002* | extension | *translated since finding 6 was fixed:* the fork enters the two regions of a nested composite state, which have no initial pseudostate; the lowerer used to refuse a `parallel` region with no `entry; then` — this project's gap, not v2's |
| *Join 001* | extension | *translated since finding 6 was fixed:* the fork enters the two regions of the top-level composite state, which have no initial pseudostate; the same lowerer refusal |
| *Choice 005* | extension | *refused, settled:* the guards of the junction's and the choice's four outgoing transitions each call `trace("T1.n(guard)")` and the admitted trace records the calls, to show when each guard is read; a v2 guard is an expression with no room for an action, so the translation keeps only the guard's value and cannot reach the trace, and is refused rather than run short. Making the runtime's guard reads the referee's observable would not reach the trace either: the junction sits on the composite's default entry and the suite reads its guards before `T2(effect)` and the composite's entry, where the library reads a transition inside a state after the state's `entry` — see [A guard whose behavior acts on the model](#a-guard-whose-behavior-acts-on-the-model) |

The last two were kept apart from the other seven and from the 29 with no spelling: UML allows
a fork to target states inside orthogonal regions that have no initial pseudostate, SysML v2
`parallel` regions can spell the shape, and only the lowerer's check stood in the way. The
lowerer now accepts a region a fork enters (finding 6), so the two run and the referee reports
them in its expressible buckets; a region with neither an entry transition nor a fork branch
into it is still refused, and the classifier names that *lowerer refuses an orthogonal region
with neither an entry transition nor a fork branch into it* (*Entry 002 E*, which is not
expressible on other grounds too).

#### Behavior parameters, operation results, tester traces and standalone machines

Four of the reasons above are not a missing v2 spelling but a translation, driver or runtime
that did not carry the construct. Each is read here against PSSM and against SysML v2/KerML,
and either translated — the same behaviors in the same order, the suite's admitted traces the
oracle — or left refused with what a spelling has to reach.

**The tester's own `trace(...)`** — *translated, in the driver.* PSSM Clause 9.2: the tester is
the test's second object; it sends signals to the target from its own behavior and, where it
calls one of the target's operations, blocks until the call returns (§8.5.9
`CallEventOccurrence`: the caller is released once the run-to-completion step the call event
triggers is done), then goes on — in *Event 019-A*, to `this.testable.trace("End")`, which
appends to the target's trace after the source's exit and the call transition's effect, and
before the `Continue` it sends next makes the next segment. The tester is not a model element of the translation; it is the referee's
driver, so its steps are performed by `run.go:drive` in the tester's order: a send is queued and
not waited for, as the tester does not wait for a signal (the pool is FIFO, so every send before
a call is dispatched before the call event, whatever the tester's and the target's relative
speed), a call is `StateExecutor.Call` (A14) and returns the operation's outputs, and a trace is the value
the tester computes appended to the target's `log` — the same store the target's own `trace`
writes — where the tester makes it. The value is evaluated by `tracer.value` over the suite's test
library read into the model (`library.go`: `Concat`, `ToString` for Boolean, Integer and
UnlimitedNatural, `formatParameterValue` spelling `[in=v]`/`[out=v]` as `Util::Tracing` does),
and a call the trace embeds is the same synchronous `Call`, made once per call action however
many of its output pins the trace reads (*Event 019-E* reads `result` and `return` of one
`or(true, true)`; `TestTracerMakesEachCallOnce`). The ordering is PSSM's under every
scheduling policy because it is fixed by the call's return, not by a draw: nothing of the
target's runs between the step's end and the tester's next action, and the referee's result is
identical under `-jobs 1` and `-jobs 8`. A trace that embeds no call and does not directly follow
one has no such anchor — the tester's `trace` and the target's steps would be interleaved by
fUML's scheduling — and `stimulation.go:traceStimulus` refuses it rather than order it by fiat;
no test of the suite is refused on that ground. *Event 019-A* moves from not expressible to
`pass` on its one admitted trace, `End` third of four segments; `TestDriveTesterTraceAfterCallReturns`,
`TestDriveRefusesATraceWhileTheMachineMayRun`, `TestDriveCallNotReturnedFails`.

**The standalone machine** — *translated, in the reader.* UML 2.5.1 §13.2.3 and §14.2: a
`StateMachine` is a `Behavior`, hence a `Class`; the suite's *Standalone* tests type the
tester's `testable` by the machine itself and give it attributes, operations with method
activities and a constructor. Its v2 reading is the one an owned
machine already has: a `part def` with `attribute`s, `action def`s for the operations and
`exhibit state` for the machine, since a `part def` is what the emitter spells a target class
as and the machine's regions, states and transitions are read the same way whichever element
owns them. `reader.go` reads the standalone machine as the `Target` whose `Machine` is itself,
with the attributes, operations and their methods, and the constructor; the constructor's
literal writes are the attributes' initial values, as for an owned class (the suite's
standalone constructors call the base constructor and return `this`). The classifier therefore no longer refuses the
kind; *Standalone 001* and *Standalone 002* are refused on their entry and exit points, the
reasons otherwise unchanged
(`TestSuiteNoTranslationReasons`); *Standalone 003* runs and passes. `standalone_test.go` reads and runs a standalone
machine with an attribute the constructor initialises and a method that writes it.

**A call trigger whose operation returns a value** — *translated: the runtime and driver carry
it, and the emitter spells the behavior that produces it.* PSSM §8.5.9 `CallEventOccurrence`
and `SM_ObjectActivation`: the call's arguments bind the operation's `in` parameters, the
behaviors the occurrence fires may write the operation's `out`/`return` parameters, and the
caller is released with those values once the step is done. In the suite the value is produced
by a *behavior*: *Event 019-D*'s `T2` effect `return "output"`; *Deferred 007*'s `T4` effect
`return T4_effect(p)` from the call's `in`; *Event 019-E*'s and *Standalone 003*'s entry
behaviors of two orthogonal regions' substates, each returning its own value, the trace admitting
either region's as the one the tester sees — the last write wins. The runtime side is A14:
`StateExecutor.Call` returns what a fired behavior returned or assigned to an output parameter
of the operation's name, typed and by name, and the driver traces it (above). The producing
behavior is an `action def` with `out` parameters named as the operation's outputs (§7.16.2), a
nameless `return` named `return` by the reader wherever the parameter appears (the operation's,
the behavior's and the tester's read of the result alike) and spelled `out 'return'`, and its
`return` statement an assignment to that parameter; the effect or entry is a usage of it, its
inputs bound as the next paragraph says:

```sysml
action def T2_effect {
    inout log : String;
    out output : String;
    first start;
    then action body { assign output := "output"; assign log := …; }
    then done;
}
transition first waiting accept op() do { action : T2_effect { inout log = log; } } then done;
```

The runtime returns what the usage assigned to `output` to the caller of `op()`. The behavior's
outputs must match the operation's by position and type as its inputs must: §8.5.9 returns the
values of the operation's output parameters, so `out value : Integer` against
`op(out result : String)` is a refusal, not an integer under the name `result`. An `inout`
parameter is one feature of the definition, declared `inout` under the operation's name and
bound `inout count = trigger_bump_count;` in the usage, so the runtime returns it as the
operation's `inout`. The body's write to it is evaluated where UML feeds the output parameter
node, into an `attribute count_written` of the definition, and `assign count := count_written;`
closes the body: UML posts the node's value when the activity completes, and every read of the
parameter before that — by the body's later statements or by another inout's write — is of the
input (`TestParametersInoutBindsOnceAndReturns`, `TestParametersInoutWritesReadInputs`). A do activity
with outputs is refused instead (`binding.go`): the step that dispatched the call ends while the
activity runs, so its outputs return to nobody; no test of the suite has the shape. The four
tests run and pass, each on exactly its admitted traces: the effect's `return` is traced by the
tester after the effect's own `trace`, and in the two-region case each region's entry assigns
the output in the order the regions are entered, the second's the value returned
(`TestParametersCallBindsInputsAndReturnsOutputs`).

**Entry, exit or do behavior with parameters** — *all translated; the exit reads the leaving
transition's payload.* PSSM §8.5.5 (`StateActivation::enter`, `exit`, `getExecutionFor`): a
state's behaviors are executed with the triggering occurrence's data — a signal's attribute
values, a call's `in` arguments — bound to their parameters in order when the behavior's
signature conforms to the data; a completion or a data-less occurrence binds nothing. In SysML
v2 the data is the transition's: `accept d : Data` (§7.18.2) declares a payload the transition's
guard and effect read, and KerML's `TransitionPerformance::accepter` holds the transfer, while
`StatePerformance::entryAction`, `exitAction` and `doAction` (`StatePerformances.kerml`) are the
state's, performed with no reference to the transfer that caused them. The spelling routes the
data through the machine: `binding.go` finds, for each behavior with parameters, the one event
every triggered transition into its site accepts (a transition's own triggers, or those
reaching a pseudostate it leaves; a transition into a state's initial pseudostate carries what
entered the state; a transition from a substate into the state enclosing it enters nothing, as
PSSM §8.5.8 leaves the target active and completes its region — SM35 — so it neither binds nor
refuses the entry, `TestParametersEnclosingTargetIsNotEntered`), checks the signature against
the event's data by position and type, and
the emitter then stores each value the accept binds in an attribute of the machine named for
the event, and declares the action's parameter bound to it. A signal `Data` with one scalar
attribute `value` and an operation `op(p1 : Integer, p2 : String)`:

```sysml
attribute trigger_Data : Data;                                        // one carrier per signal
attribute trigger_op_p1 : Integer = 0; attribute trigger_op_p2 : String = "";   // one per input
transition first wait accept data : Data do { assign trigger_Data := data; } then working;
transition first working accept op(p1, p2) do {
    assign trigger_op_p1 := p1;
    assign trigger_op_p2 := p2;
} then resting;
state working { entry action : working_entry { inout log = log; in data : Data = trigger_Data; } }
state resting { entry action : resting_entry { inout log = log; in p1 = trigger_op_p1; in p2 = trigger_op_p2; } }
```

The values are the occurrence's own, they are read in the step the occurrence is dispatched in
(the effect stores before the entry runs, §7.18.3), and the store is no observable unit: it is a
statement of the transition's effect, which the traces record only through its own `trace`
calls. An effect that is itself the parameterised behavior reads the `accept`'s parameters
directly and stores nothing. A bound do activity declares its inputs on the `do action` and
chains its statements one action each (`first start; then action step1 {…}; then done;`), so
the declaration is evaluated when the activity starts and not as a step of its own — the plain
body would spend a do round on it, and a two-step body never finishes before the next event is
dispatched (`state_executor.go:oneUnit` dispatches after every closed round), so the admitted
trace with the activity's segment would be unreachable. Each binding is re-read per occurrence
(`TestParametersRebindPerOccurrence`); `TestParametersSignalBindsEffectAndEntries`,
`TestParametersDoActivityBindsInputs` pin the two spellings.

An *exit* cannot read a carrier: it runs before the leaving transition's effect (§7.18.3:
exit, effect, entry), so nothing the leaving transition stores is there yet, and the attribute
the *entering* transition stored holds the entering occurrence's value — *Event 017-B*'s nested
exit traces `[in=false]`, the second `Data`, after an entry with `true`. What the exit can read
is the leaving transition's own payload, since it is a step of that transition's performance.
Two spellings were weighed against the admitted traces of *Event 017 B*, *019 B* and *019 C*:

| Candidate | The reading | Against the admitted traces |
|---|---|---|
| **Read the leaving transition's payload** — `exit action { in p1 : Integer = T2.p1; … }`, each leaving transition's `accept` naming the exit's `in` when it fires | The exit is performed *within* the transition performance that accepted the occurrence: `StateTransitionPerformance` (`StatePerformances.kerml`) orders `accept then transitionLinkSource.exit` and `TransitionPerformance` (`TransitionPerformances.kerml`) orders `transitionLinkSource then effect`, so the accepted transfer is held when the source's exit starts and released only after the effect; its `trigger` subsets `transitionLinkSource.accepted`, the source state performance's own `accepted` transfer (`StatePerformance::accepted`, `acceptable then exit`), so the exit reads a transfer of the performance it is a step of, not a stranger's. In notation `accept d : Data` (§7.18.2) declares `d` a feature of the transition usage — `StateTransitionAction` binds `payload = accepter.payload` (`States.sysml`) — and a feature chain from the transition's name reads it; the parser and name resolution accept `T2.p1` and `'T1.1.2'.p1` as they accept any qualified feature. A transition not being taken performs nothing, so the chain reads nothing then, and the exit shared by several leaving transitions spells the alternatives `T3.d ?? T4.d`, each binding on its own firing. Nested exits read the outer transition's payload the same way: PSSM §8.5.5 `exit` passes the same event to every state left | **translated.** *Event 017 B* reaches its 2 admitted traces and no other, *019 B* its 1, *019 C* its 1; *Standalone 002*'s behavior-parameter reason (its second state's exit) drops, its entry- and exit-point reasons stay; every other row byte-identical |
| **Stage the value at accept time** — a machine attribute assigned as the transition accepts, before the exit | An `accept` is the transition's action and has no statement of its own; the assignment would be a statement of the effect (§7.18.3), which the library orders after `transitionLinkSource.exit` — so the store either runs after the exit (the exit reads the entering occurrence's value, the refused trace) or is moved ahead of the exit by no text of the specification | **not needed**: the first candidate reaches the admitted sets; recorded so the spelling is not revisited |

The runtime binds the exit's parameters from the lowered transition, not from the notation:
`lower.Transition.Accepted` names what the trigger binds (the signal payload, the operation's
inputs, `state_graph.go:AcceptedNames`), the executor knows the transition it is taking before
the exit runs (`state_executor.go:taking`, chosen by the RTC step's selection) and copies its
payload into the frame every behavior of the firing reads (`state_statements.go:currentFiring`,
`stateStmtHost.dataFrame`), and `T.d` evaluates against that frame (`transition_payload.go`):
the taken transition's value, the null value for a transition not being taken, a `NoValueError`
when the taken transition accepts `d` but bound nothing. The frame survives joins, queued
events and nested exits (`state_exit_nested_reads_outer_transition`,
`state_exit_shared_by_two_transitions`; the transition is the one the name resolves to, so an
outer transition a nested same-named one shadows is read qualified, `Machine::T.c`,
`state_exit_shadowed_transition_name` — the emitter declares the transitions an exit reads by
names unique in the machine instead), a body snapshotted for a later read
(`frame.snapshot`, `state_exit_payload_deferred_read`), the frames a predicate the state
declares closes over (`invoke_predicate.go:flattenFrames`, `state_exit_payload_nested_predicate`)
and the whole of a compound transition — a segment past a choice or a junction declares no
trigger, so its effect reads the accepting segment's payload by that segment's name, within
whose performance it runs (`state_route_effect_reads_accepting_segment`) — and a completion
firing binds nothing, leaving the parameter's default
(`state_exit_completion_binds_nothing`) or, for an `in level : Integer[0..1]` bound to a
transition not taken, no value at all (`state_exit_payload_optional_input`);
`TestRuntimeRobustnessExitParameters` pins the typed errors. A do behavior reads the transition that entered its state for its whole
run, not whichever firing its steps happen to fall in: the state performance holds the transfer
that triggered the transition into it (`StatePerformance::incomingTransitionTrigger`,
`StatePerformances.kerml`), so the executor copies the entering firing into the do behavior as
it starts (`state_executor.go:startDoAction`, `doAction.firing`) and every step reads it, whether
the entry front draws the first step before or after the entries of the substates entered with
it (`state_do_reads_entering_transition`, agreed across every schedule by the checker). The
emitter (`emit_behavior.go:exitValue`) spells the exit's inputs as the chains; the reader
(`activity.go:dry`) leaves out an activity node whose required input pin no token ever reaches,
since UML never executes it (*Event 019 C*'s exit holds a `ToString` call nothing feeds and
nothing reads; `TestReadStarvedActions`). An exit some leaving paths bind nothing on — a
completion, or an occurrence whose data the signature does not take — still runs on those
firings with its inputs empty, and only the activity nodes a token from the input must reach
never fire (§8.5.5: the input parameter node holds no token, so what it feeds, directly or
through the nodes before it, never offers): the reader records those inputs per statement
(`activity.go:needs`, `Statement.Needs`, `TestReadStatementNeeds`) by walking the activity
once with every input fed and once per input with its parameter node absent, the binder marks
the site (`Binding.Partial`), and the emitter declares such inputs `[0..1]` and wraps only the
statements needing them in `if notEmpty(<input>) { … }` (`emit.go:guarded`), importing
`SequenceFunctions::*` for the guard; a statement needing no input runs as before
(`TestParametersExitLeftWithoutData`).
`TestParametersExitBindsLeavingTransition` and `TestParametersExitSharedAndNested` pin the
spelling. What is refused, with the reason the classifier
reports under *behavior parameter* (`TestParametersRefusals`):

- a site reached by a completion transition, from the machine's start, or by paths accepting
  different events: UML binds nothing or a different occurrence there while the carrier would
  hold the last store; an exit left *only* by paths binding nothing — completion, data-less
  occurrences, data its signature does not take — is refused the same way, while one left by
  such a path beside conforming ones binds on the conforming firings and nothing on the
  others, as §8.5.5 reads (`bindsNothing`, `Binding.Partial`);
- a signature that does not conform (arity, or a type with no `ScalarValues` counterpart), a
  signal with several attributes, a transition with several triggers, and a do activity with
  outputs.

Same-named operations are told apart by identity throughout, as a UML `CallEvent` names one
`Operation`: the tester's call binds its arguments to the parameters of the one it targets
(`stimulation.go`), `sameEvent` compares the operations, not their names, and each carries its
own attributes — the overload's number among the machine's call triggers in document order joins
the name, `trigger_bump_1_count` for `bump(inout count : Integer)` and `trigger_bump_2_flag` for
`bump(in flag : Boolean)`, so one carrier never holds the other's value
(`TestParametersOverloadsByIdentity`, `TestParametersOverloadsBindApart`,
`TestParametersOverloadsCarryApart`); a `CallBehaviorAction` likewise names one `Behavior`, so
the reader carries its `xmi:id` and the emitter inlines that owned behavior, not the first of
its name (`TestParametersAppliesBehaviorByIdentity`). The trigger itself is where the notation runs out:
`accept bump(count)` names the operation by name and parameter names, the emitted machine
declares no operation for the runtime to select among, and a call of either overload binds a
`count`, so two same-named operations one of whose input names cover the other's —
`bump(inout count : Integer)` and `bump(in count : Boolean)` — have one accept spelling between
them and a trigger naming either is refused (`emit.go:indistinctOverload`,
`TestParametersOverloadsWithOneSpellingRefused`); no test of the suite has the shape.

The classifier reports only the behaviors `binding.go` refuses, so *Entry 002-F*'s and
*Standalone 002*'s entries and exits bind and their reasons name their entry and exit points
alone.

#### A guard whose behavior acts on the model

*Choice 005* (PSSM §9.4.10) is the one test whose guards act: `T2` (`Start`, effect
`T2(effect)`) leads `wait` to a composite state with an entry behavior, whose region starts
through `T1.1` into the junction `Junction1`, out of which `T1.2` (`true`) reaches the choice
`Choice1` and `T1.3` (`false`) a second substate; out of the choice `T1.4` (`true`) reaches the
first substate, which has an entry behavior, and `T1.5` (`false`) the second. Each of the four
guards is an activity that calls `trace("T1.n(guard)")` and then returns its literal, and the
one admitted trace has seven segments — `T1.2(guard)::T1.3(guard)::T2(effect)`, the
composite's entry, `T1.4(guard)::T1.5(guard)`, the first substate's entry (the trace is quoted
in [the referee record](../../project/pssm-referee.md)): the junction's two guards read at
`T2`'s selection, before its effect and the composite's entry — the static evaluation of §8.5.6
reaching through the composite's default entry, the same reach SM32 records for *Junction 004*
— and the choice's two read on arrival, after the composite's entry, the dynamic evaluation of
§8.5.7. The suite observes *when each guard is read*, and the calls are its only
means of observing it. Whether the referee can observe the same without giving a v2 guard a
side effect was tried three ways; none reaches the trace, so the test stays refused with the
classifier's reason (`guard side effect T1.2; …`), and the table below records why.

*The v2/KerML reading.* A guard is a Boolean expression: `bool guard[*] subsets
enclosedPerformances;` on `TransitionPerformance` (`TransitionPerformances.kerml`), an
`Evaluation`, with the transition's action a separate `step effect[*]`, ordered
`private succession all [*] guard then [*] effect;`. `StateTransitionPerformance`
(`StatePerformances.kerml`) orders a transition's guard after the acceptable transfers and
before the source's exit (`private succession all [*] acceptable then [*] guard;`,
`private succession [*] guard then [1] transitionLinkSource.exit;`), and `StatePerformance`
orders a state's entry before every middle step (`private succession [1] entry then [*]
middle;`), the region's transition performances among them. So the library places a guard's
reading in time — after the trigger, before the exit and the effect — and places every
transition inside the composite, `T1.1` through the junction included, after the composite's
`entry`. UML 2.5.1
§14.5.11 states the other half: guards "should be pure expressions without side effects",
and "guard expressions with side effects are ill formed".

| Candidate | What it would do | Result against the admitted trace |
|---|---|---|
| Guard reads as the referee's observable: the emitter spells the four guards as the pure `true`/`false` they return, the runtime records each guard evaluation as a trace event naming the transition, and the run maps the events to `T1.n(guard)` in `log` | the guard stays a Boolean expression and the model is never mutated; the observable is the runtime's, not the model's | **refused.** Run under the shape the emitter produces (an initial into a junction is spelled as a helper start state whose completion transition reaches the junction, `emit.go`, `TestEmitInitialIntoPseudostate`), the runtime's execution trace reads the junction's guards *after* the composite's `enter` line and the helper state's entry, as the completion transition out of it is selected (`state_route.go:resolveRoute` → `followOut`, SM29: static for *that* transition, whose source is inside the composite), and the choice's after the helper state's exit — `T2(effect)`, the composite's entry, then `T1.2(guard)::T1.3(guard)::T1.4(guard)::T1.5(guard)` and the first substate's entry at best, which the suite does not admit; its one trace needs the junction read at `T2`'s selection, before `T2(effect)`, a reach through the default entry that SM32 records as *differs, v2 silent* and the library's `entry then middle` places the other way. The channel is also short of the suite's reads: `enabledBranches` reads the first guard of a vertex in the open and every further one under `beginProbe`, which restores `ctx.trace` — so of the four reads the execution trace holds two `eval` lines, `T1.2`'s and `T1.4`'s, and `T1.3`'s and `T1.5`'s are rolled back with the probe; a choice branch past the first is read once probed and once more when drawn (`resolveChoice`), a read the suite never traces. Exposing every read as an event would need a rule for probe reads, repeated reads and their identity that no test of the suite fixes and this one contradicts on its first two entries. Reached against admitted: 0 of 1, with one trace the suite refuses. Nothing else moves — no other expressible test has an acting guard (`TestClassifyGuardSideEffect`), and the runtime is unchanged |
| A `calc def` or an expression with a side effect: spell each guard as a calculation that appends to `log` and returns its literal | the trace would be reached | **refused.** A v2 expression is pure — a `calc def` is a `Function` and a `calc` usage an `Expression` (SysML v2 §7.17), an `Evaluation` that computes a result and performs no action; a transition's guard is an `Expression` (§7.18.3, `validateTransitionFeatureMembershipGuardExpression`), so `bool guard[*]` is an `Evaluation`, not a `step`, and no `assign`, `send` or `perform` may stand in one. What acts is `step effect[*]`, ordered after the guard. UML 2.5.1 §14.5.11 calls the guard with the side effect ill formed, so the spelling would encode a construct the source specification declares malformed to observe an order the target library places differently. Not spelled |
| A `differs-by-design` row: adjudicate the test as differing because v2 orders the default entry after the composite's entry (SM32) | the test would run with pure guards and its `fail` on the missing four segments would be attributed to the tool choice | **refused.** `differs-by-design` names a *differs because v2 differs* row a test reaches (`rows.go:TestRows`); SM32 is *differs, v2 silent*, so the bucket does not apply, and the test does not run at all: with pure guards it reaches `T2(effect)`, the composite's entry and the first substate's entry, three segments where seven are admitted, and the difference is the construct's, not the order's alone. The classifier's refusal stands; SM31 (choice with no guard true) and SM32 (junction with no path through) are unchanged, and *Junction 002*, *Junction 004* and *Join 003* keep their buckets and reasons |

The classification is the settled one: *guard side effect* is a construct with no translation
and no faithful observable, `not-expressible` with the reason naming the four transitions,
byte-identical to the baseline: the settled refusal of a construct v2 cannot spell faithfully,
not a defect of the suite — the order the test observes is the one §8.5.6 and §8.5.7 define, and
no v2 spelling observes it.

### What a translated test looks like

*Deferred 001* (PSSM §9.3.16.2, Figure 9.90) exercises deferral in a simple state: `Continue`
arrives while the first state defers it, `AnotherSignal` moves the machine to the second, the
recalled `Continue` fires its transition ahead of the later `Pending`, and the third state's
completion transition ends the machine with `Pending` never dispatched. The expected trace is the
first state's exit segment, the second's entry, the recalled transition's effect and the third's
entry, in that order. Written by hand from the figure with this project's `defer`, the states
and transitions renamed for the translation (`deferring`, `released`, `last`; `recall`,
`lapsed`):

```sysml
package Deferred001 {
    private import ScalarValues::*;

    state def Deferred001 {
        attribute log : String = "";

        entry; then wait;
        state wait;
        state deferring {
            defer Continue;
            exit action { assign log := log + "deferring(exit)::"; }
        }
        state released {
            entry action { assign log := log + "released(entry)::"; }
        }
        state last {
            entry action { assign log := log + "last(entry)"; }
        }

        transition first wait accept Start then deferring;
        transition first deferring accept AnotherSignal then released;
        transition recall first released accept Continue
            do { assign log := log + "recall(effect)::"; } then last;
        transition lapsed first released accept Pending
            do { assign log := log + "lapsed(effect)::"; } then last;
        succession first last then done;
    }
}
```

The case's `events` would list `Start`, `Continue`, `AnotherSignal`, `Pending` and its `slots`
the expected `log`, `deferring(exit)::released(entry)::recall(effect)::last(entry)`. Every step
of it lands on an **agrees** row (SM4 dispatch order, SM5 deferral, SM6 recall order, SM8/SM10
completion ahead of `Pending`), which is exactly why a pass would corroborate but not adjudicate: v2 and PSSM say the same thing here. The example is prose,
not a fixture, and whether the runtime passes it is not asserted.

*Transition 011-D* (PSSM §9.3.3.7, Figure 9.17) is the other kind: a *local* transition out of
a composite state to one of its own exit points, whose purpose is "to demonstrate that, when a local
transition leaves the containing state, then this state is not exited". It has no SysML v2
spelling (SM36) and none of this project's extensions reaches it; a translation to an external
transition would exit the composite state and produce a trace PSSM rejects, and the disagreement would be v2's,
not the runtime's. Its two admissible traces (the two regions' exit segments in either order) are the
pattern the `outcomes` mechanism already handles for the expressible orthogonal-region tests.

### What the suite is, and is not, for this runtime

An advisory PSSM comparison would generally test reproduction of UML behavior unless the UML
model and semantics have a defensible SysML v2 mapping; it must not be described as proof of
SysML v2 conformance. Concretely: of the 61 expressible and runnable tests, every one whose
requirement lands on an **agrees** row checks that the runtime does what UML and v2 both say;
the tests that land on SM7, SM9, SM11, SM28, SM30 and SM32 — *Deferred 004-A/B* and their kin,
whose deferring state has a competing transition in a sibling region; a completion transition
whose guard changes between the completion and its dispatch step; the tests whose composite
state owns a completion transition; a history test entered with nothing recorded and no default;
a choice whose guard reads what the incoming effect wrote; and *Junction 002* — are the ones that
would report on a tool choice; and the 37 tests
with no spelling or translation, together with any test that reaches SM15, SM36 or SM37, would fail for reasons
that are v2's, and a harness would have to exclude them by classification rather than report
them as failures. Used that way, the suite is a second opinion on ten rows and a regression
oracle for forty-nine; it is never a conformance statement about SysML v2.

## Options

Four courses of action, each judged on scope, on what it depends on, on its order against the
state-machine stage of the [bounded model checking](bounded-model-checking.md) note (state
executor snapshot/restore, dispatch-order choice points, time ties — stage 3 there) and against
Track E of the roadmap, on its acceptance gate, and on what a user would see.

### (a) Keep the v2/KerML position; adopt PSSM's rule on the `differs, v2 silent` rows where it is better grounded

- **Scope.** Nine rows. For each, decide between the runtime's rule and PSSM's, record the
  decision in the row and in `docs/project/spec-compliance.md`, and implement the changes that
  fall out. The map suggests the split: **adopt PSSM** on SM7 (deferral outranks a transition in
  an enclosing state or a sibling region — the runtime's rule makes `defer` ineffective whenever
  any other region reacts, which defeats the purpose of the extension) and SM28 (an empty history
  with no default falls back to the region's initial transition — a refusal serves no v2 rule,
  and UML is the extension's reference); **keep ours, and say so** on SM30 and SM32 (the
  runtime's static evaluation of choice and junction guards is one rule for both vertices and is
  what makes `pseudostates.md`'s reading of a junction hold; a change would split them), on SM9
  (a guard read once at completion is the simpler rule, and a completion-occurrence pool would be
  a second event kind for the one case the row names), on SM45
  and C8 (a refused destroy and a failed send are typed errors a modeler sees, where fUML/PSCS
  silently drop; that is a deliberate tool choice this project makes everywhere), and on C3 (the
  connector model is the library's, and the pattern strategies would be a second one); **decide**
  SM11 — it is the one row where PSSM's rule (a composite state's completion fires the composite's
  completion transition) is arguably what a v2 modeler expects and the v2 sentence is uneasy, so
  it is listed in *Open decisions*.
- **Dependencies.** None on the bounded-model-checking stages: SM7 and SM28 are changes to
  `selectTransitions`/`dispatchEvent` and `fireHistoryTransition`, both already inside the
  snapshotted state (stage 1 of that note is implemented and captures the event queue, deferred
  events and history), and neither adds a choice point. None on Track E. SM11, if changed,
  touches `completeIfDone`/`scheduleFromLeaf` and the completion-event ordering the trace goldens
  pin, so it would come with golden updates and belongs *before* stage 3 (dispatch-order choice
  points) rather than after, so the choice points are added over the final completion rule.
- **Acceptance gate.** Per row: a conformance case for the new rule, the existing cases
  unchanged except where the row's decision says otherwise (SM7 changes the outcome
  `TestEventConsumedByAnotherRegionIsNotDeferred` pins, and that test is rewritten *with* the
  decision, not deleted), the spec-compliance row updated, and the semantic-map row here gaining
  a "*Decided:*" sentence.
- **User-visible change.** Two models behave differently: one with `defer` beside a reacting
  sibling region, and one entering an unvisited `history` without a default transition (a run
  that failed now proceeds). Nothing else moves.

### (b) Also build the referee harness as an advisory, opt-in gate

- **Scope.** Option (a) plus a `cmd/pssm-referee` in the shape of `cmd/pilot-exec-diff`: a
  script fetching `ptc/18-11-06` at its pinned checksum into a git-ignored directory (never
  vendored), a translator from the suite's UML XMI to `.sysml` for the constructs in the mapping
  table above, refusing every test that uses a construct with no spelling and bucketing it
  `not-expressible`, a driver queuing the test's stimulation sequence and running the translated
  machine, a comparator normalizing the `TraceBuilder` string against the model's `log` and
  matching the result against the test's set of expected traces (as a set, under the `explore`
  policy), and a committed baseline of bucket counts (`pass`, `fail`, `not-expressible`,
  `differs-by-design` for the tests reaching SM15/SM36/SM37) that CI compares
  by count and never by pass/fail.
- **Dependencies.** The translator's hardest part, the `Tester`/`Target` protocol, is a
  fixed pattern in the suite (one `Start`, then the listed signals), so the harness's driver is
  the conformance harness's `injectEvents` and nothing more; tests that need mid-run arrival
  would be bucketed, not driven. Independent of the bounded-model-checking stages, since the
  `explore` policy already exists; it *benefits* from stage 3 (dispatch-order choice points make
  the set comparison exhaustive for the orthogonal-region tests rather than budget-bounded). No
  Track E dependency; the three terminate tests, held in a `terminate-gap` bucket until Track E
  closed terminate, run since (*Terminate 003* and *001* passed at once, *002* once the do step
  was drawn on the entry front; all three pass).
- **Acceptance gate.** The harness reproduces its committed baseline deterministically; the
  `pass` bucket is not a CI gate, only its *count* is, adjudicated on every movement like the
  corpus ratchets. The tool's `-h` says in one sentence what a pass means, in the words of the
  paragraph closing the capability map.
- **User-visible change.** None to the runtime beyond (a). A maintainer gains a second oracle for
  the ten rows and a regression net of about fifty translated machines.
- **Cost.** A UML XMI reader for the suite's subset (state machines, regions, vertices,
  transitions, triggers, opaque behaviors whose bodies are `trace(...)` calls, signals) and a
  `.sysml` emitter; a checksum-pinned download script; a baseline document under `docs/project/`
  in the shape of `pilot-execution-referee.md`. It is the size of `cmd/pilot-exec-diff`, not
  of an executor.

### (c) A user-selectable PSSM-conformant execution mode

- **Scope.** A `-semantics pssm` (or `%semantics`) switch under which the state executor follows
  PSSM on every row where it differs: SM7, SM11, SM15, SM28, SM30, SM32, SM34, SM36, SM37, SM45,
  plus
  the local and internal transition kinds themselves, which need new notation and lowering
  (`transition local first …`), a completion-event pool (SM9 exactly rather than equivalently),
  and PSSM's variation points (time source, choice of conflicting transitions) made explicit.
  Two semantics in one executor: every `switch` on the mode in `selectTransitions`,
  `broadcastEvent`, `completeIfDone`, `resolveRoute`, `fireHistoryTransition`, `destroy` and the
  lowering of transition kinds, with each conformance case, trace golden and robustness case
  doubled or annotated by mode.
- **Dependencies.** Blocks on stage 3 of the bounded-model-checking note and then doubles it:
  the dispatch-order choice points and the `do` interleaving representation would have to be
  defined per mode, and the [analysis framework](analysis-framework.md)'s rule that "the
  interpreter is normative" would have two normative interpreters — every engine (`smt`, `check`)
  either supports both or is silently wrong under one. Terminate is closed for both modes at
  once (the rules coincide, SM38); Track E's interrupting-performance and streaming entries are
  fUML rows the mode would not touch.
- **Acceptance gate.** The PSSM suite itself, passing on every expressible test — which is the
  one thing this option buys that (b) does not, and only for the 61 expressible tests, since the
  30 with no spelling need new notation first.
- **User-visible change.** A mode switch a user has to understand, whose meaning ("this model
  now runs as UML") contradicts the architecture's stated position; two answers to "what does
  this model do"; and the `differs because v2 differs` rows, where the mode would make the runtime
  disagree with the SysML v2 text it implements. This is the cost the question asks for stating
  plainly: the two semantics are not two configurations of one engine but two engines, and the
  project's normativity rule cannot hold for both.

### (d) Do nothing

- **Scope.** Leave the ten `differs, v2 silent` rows as they are, this note as the record
  that they were examined, and the three gaps to Track E.
- **Dependencies, gate, user-visible change.** None.
- **What it leaves.** SM7 — a `defer` that any sibling region's reaction overrides — is a rule
  this project chose without the alternative in view; it stays chosen. SM28 stays a run failure
  where UML, the extension's own reference, proceeds. The architecture's fallback clause ("UML
  2.5.1 where v2 has no production and the library no performance") would then be applied to the
  *existence* of `history`, `fork`, `join`, `choice`, `junction` and `defer` but not to their
  *semantics* on these rows, which is a harder position to defend than either (a) or (d)'s
  simplicity suggests.

## Recommendation

**Option (a), with (b) as a follow-on once (a)'s two changes have landed.** The map shows no case
for porting the precise-semantics family: 50 of 70 rows agree already, 7 differ because SysML v2
says otherwise and must stay as they are, and the 3 gaps are v2 gaps Track E already owns. What
remains is nine tool choices, and on two of them — SM7 and SM28 — PSSM's rule is the reference
this project's own extensions name (UML) applied consistently, while ours is an accident of
implementation order; on the other seven the runtime's rule is deliberate and better for a modeler
(typed errors over silent drops, one guard-evaluation rule for both pseudostates, one reading of
a completion guard, the library's connector model), and the row records why. Option (b) is worth having *after* that, as an
advisory oracle in the established `cmd/pilot-*` shape and with its meaning stated in its own
words: an advisory PSSM comparison tests reproduction of UML behavior unless the model and
semantics have a defensible SysML v2 mapping, and it is never proof of SysML v2 conformance.
Option (c) is rejected: two normative interpreters break the analysis framework's rule, double
stage 3 of the model checker, and make the runtime disagree with the v2 text on six rows for
users who select the mode. Option (d) is rejected because SM7 and SM28 are findings this note
has now made and leaving them stands the extensions on UML for their syntax and on nothing for
their semantics.

*Decided:* option (a), and on its open rows PSSM's rule throughout — SM7 and SM28 as
recommended, SM11 as leaned and as the library's own `done`/`endShot` binding reads, and SM30 as
the project's own `pseudostates.md` already promised, so the "keep ours" position above holds for
SM9, SM32, SM45, C8 and C3 only. Each of the four rows carries
its *Decided:* sentence with the functions and fixtures; option (b) remains the follow-on it was.

Nothing here changes the architecture's position. SysML v2 and the Kernel Semantic Library
govern; UML 2.5.1 — and now, on the state-body extensions, PSSM's reading of UML — is the
reference where v2 has no production and the library no performance; the runtime is not a fUML
activity engine and does not become one.

## Findings about our own conformance

The rows below report the runtime differing from, or falling short of, SysML v2's or the Kernel
Semantic Library's *own* text, or from this project's own design notes. They are bug reports and
unsupported-feature records, not alignment questions: PSSM has nothing to do with them and they
are not alignment questions. Each names its evidence; items 1, 4 to 10 are fixed, and say where;
item 11 is adjudicated — a translation limit on three tests, the suite's defect on two, its two
sites of the runtime's fixed, the pool's order and the do step drawn on the entry front.

1. **Terminate was parsed and lowered but not executed** (SM38). SysML v2 §7.17.10 and §7.18.3
   define `terminate`; `Performances.kerml` provides `TerminatePerformance`; the parser accepted
   it and `lower.EffectTerminate` carried it, and `action_statements.go:actionStmtHost.effect`
   refused it as "'terminate' in a body is not executable". Fixed: the row SM38 names the
   lowering (`lower.Effect.Terminates`, `StateGraph.Terminates`) and the execution
   (`action_terminate.go`, `occurrence_terminate.go`, `state_route.go:terminateAt`) of every
   position the parser accepts; the calculation refusal alone stays
   (`robustness_test.go:calc_terminate_is_rejected`). That PSSM's *Terminate 001–002* describe
   the same behavior is a coincidence of the two texts and did not make this a PSSM alignment
   item: the implementation follows §7.17.10, and *Terminate 003* passes as a consequence,
   *Terminate 001* since item 9's region-entry order is drawn, and *002* since item 11's do
   step on the entry front is drawn: it reaches all five admitted traces and nothing else.
2. **Streaming flows, parallel expansion and interrupting an ongoing performance** (A8, A9,
   A10). `Flows.sysml` distinguishes `Flow` from `SuccessionFlow` and SysML v2 §7.16.1 says a
   streaming flow may be ongoing while both actions perform; the runtime applies every flow at
   source completion (`action_frame.go:applyDataFlows`) and `lower.ObjectFlow` carries no flow
   kind. Roadmap Track E, "streaming flows", "concurrent per-element performance",
   "interrupting an ongoing performance". Recorded there; not re-scoped here.
3. **Run-to-completion values and scopes are lowered and applied** (SM1).
   `Occurrences.kerml` declares both features with defaults. `lower/run_to_completion.go:resolveRunToCompletion`
   records each state's effective Boolean value and ancestor scope in `StateGraph`; literal values
   avoid evaluator traces, while dynamic values use the lowered evaluation scope and report typed
   failures. `state_run_to_completion.go:holdEntry` identifies the entering chain and
   `entryStep` exposes free dispatch versus held entry as one scheduling choice, with checker,
   exploration and replay support. The conformance fixtures
   `state_run_to_completion_false_self_signal`, `state_run_to_completion_scope_sibling_region`
   and `state_run_to_completion_false_machine` exercise state, scoped and machine redefinitions;
   sibling and missing scopes remain typed lowering refusals. **agrees.**

4. **A composite state's completion ends the machine and never fires the composite's own
   completion transition** (SM11). SysML v2 §7.18.3 says a transition to `done` completes "the
   containing state performance" and that the containing state "does not necessarily terminate
   immediately"; `States.sysml` binds `done` to the `StatePerformance::endShot` of the state whose
   body names it, and `TransitionPerformances.kerml` places a transition's `effect` and target
   after its `transitionLinkSource`'s performance — so `then done` in a composite's body ends that
   composite, and `transition first outer then next` is the firing that follows its `endShot`.
   `completeIfDone` → `machineComplete` instead propagated the completion to the machine and
   `scheduleFromLeaf` never scheduled a nil-trigger transition out of a composite state
   (`state_completion_nested_regions`), reading a substate's `done` as the machine's `endShot`
   with no basis in either file. The spec-compliance record stated the rule as adopted, so this
   was a finding against the library text, not against the record. *Fixed* with the first open
   decision: the composite's own completion transitions fire (SM11's *Decided* sentence names the
   library declarations, the functions and the fixtures), and the roadmap's Track E records the
   finding as landed.
5. **Choice guards are read before the incoming effect runs, where the project's own note says
   otherwise** (SM30). `pseudostates.md` describes a choice as "a dynamic conditional branch whose
   outgoing guards are evaluated when the choice is entered"; `resolveRoute`/`pseudostateBranch`
   pick the branch before the incoming transition's effect runs, and the code comment says the
   two pseudostates are "indistinguishable for a guard over state data". v2 has no choice vertex,
   so this is not a v2 finding; it is a disagreement between a design note and the code, and one
   of them has to change (second open decision). `state_choice_pseudostate` does not reach it.
   *Fixed* with the second open decision: the code now matches the note (SM30's *Decided*
   sentence), and Track E records the finding as landed.
6. **The lowerer refuses a fork into orthogonal regions that have no initial pseudostate.**
   UML lets a fork's outgoing transitions enter states inside a composite state's orthogonal
   regions directly, with no initial pseudostate in those regions (PSSM *Fork 002* and *Join
   001* are built this way); SysML v2 `parallel` regions can spell the shape, and this
   project's `fork` extension can spell the fork. `lower.ToStateGraph` refuses it — "region
   `<name>` has no initial state; write `entry; then <state>;` inside the region" — because it
   required every region to name its own start even when a fork was the only way in. A gap of
   ours, which the PSSM referee's classifier recorded as *lowerer refuses fork into a region
   without an entry transition*. *Fixed:* `lower/fork_plan.go:planForks` reads every fork's
   branches into a `ForkPlan` — one target state per orthogonal region of one composite state,
   no guard, at least two branches — and `ToStateGraph` accepts a region with no entry
   transition when `ForkStarted` says a fork enters it, still refusing one with neither
   (`robustness_test.go:fork_leaves_a_region_without_a_way_in`). Such a region has no default
   start, so `checkForkOnlyRegion` also refuses a machine where any other way into the composite
   — a transition to the composite itself, to a state in another of its regions or to its
   history, its own self-transition, or the machine's entry naming it, directly or through a
   junction, choice or join — would start the region by default, naming that way in
   (`robustness_test.go:fork_only_region_entered_by_default`). The runtime consumes the plan
   (`state_executor.go:fireForkTransition` → `state_region_entry.go:enterForkBranches`): the
   source configuration is left down to the least common ancestor of the source and the
   composite, as for a move to a single state (`leaveForFork`), so an active ancestor is
   neither exited nor entered again; then each branch runs its effect, enters what is left of
   the way down to the composite and its target directly (PSSM §8.5.7), a region no branch
   names taking its own initial. Pinned by `state_fork_enters_regions_without_initial`,
   `state_fork_in_composite_enters_parallel_substate`, `state_fork_within_active_ancestor`,
   `state_fork_within_active_region` (all with trace goldens) and `lower/fork_plan_test.go`.
   *Fork 002* and *Join 001* translate and run; the branches are still entered in the regions'
   declaration order, so the interleavings PSSM admits beyond that one are SM22's open decision,
   and `docs/project/pssm-referee.md` records where each landed.
7. **A transition from a composite state into its own history pseudostate reads the record
   before the state is left.** The configuration a history restores is written when its owner
   is exited (`state_executor.go:exitState` → `recordChildHistory`, `recordRegionHistory`), but
   `state_route.go:resolveRoute` and `state_executor.go:historyEntry` read it before the
   transition's exits run — the code says so: "read before the source configuration is left".
   For a transition whose source is the owner itself the read sees the owner's *previous* exit,
   not the configuration being left, where UML and PSSM restore the "most recent" one (the
   *History 001* and *History 005* requirements quoted at SM26 and SM27). PSSM *History 001-A*
   (a composite's self-transition into its own deep history while a nested substate is active)
   finds no record and performs a default entry where the suite restores the substate;
   *History 002-D* (a composite's completion transition into its own shallow history, its
   region having reached a final state) finds the record the completed substate's exit left —
   which the composite's own exit would have cleared under SM28's rule — so the history's
   default transition is skipped, the substate is re-entered, the composite completes again and
   the run exhausts its step budget. Surfaced by the PSSM referee once SM11 and SM28 landed
   (before them 001-A was a typed error and 002-D ended at the machine's completion).
   *Fixed:* `resolveRoute` leaves a history route unsettled, and `fireHistoryTransition` →
   `moveToHistory` exits the source configuration and runs the transition's effects before
   `historyEntry` reads the record; a default transition is taken from inside the owner once
   the owner is entered (`defaultHistoryRoute`), so the owner's `entry` runs before the default
   transition's effect and the region's ordinary initial transition never runs beside it. A
   history declared in the machine's own body, restoring the machine's top-level configuration
   (PSSM *History 001-D* is built this way), is owned by the graph's root state. Pinned by
   `state_deep_history_self_transition`, `state_shallow_history_completion_default`,
   `state_machine_body_deep_history` and `robustness_test.go:history_outside_composite_state`; the
   two tests pass, and *History 001-B*, *001-D*, *002-A* and *002-C* with them.
8. **A junction with several enabled branches took the first in declaration order, where a
   choice with several draws one.** `pseudostates.md` names the choice/junction difference as
   *when* the guards are read, and SM30 makes several enabled choice branches a transition
   choice point (`ChoiceTaken`, enumerated by `explore`); `state_route.go:pseudostateBranch`
   stopped at the first junction guard that held, so the same shape at a junction was
   deterministic and `explore` reported the run complete after one branch. KerML's
   `DecisionPerformance` (`outgoingHBLink: HappensBefore[1]`) fixes that one succession is
   taken and ranks none, for a junction as for a choice; UML says the same of a junction whose
   several outgoing guards hold (PSSM *Junction 003*: "one is chosen; the algorithm for making
   this selection is not defined"). PSSM *Junction 003* admits two traces, one per branch, and
   the referee demands that exploration reach both; the runtime reached the first only.
   *Fixed:* `enabledBranches` reads every outgoing guard of a junction at the static instant —
   the later ones in a preview that is undone, as at a choice — and several enabled leave the
   route open at the junction, where `settleDraws` → `pickBranch` makes the transition choice
   point only as the transition fires: after the region order among several candidates is drawn
   and the transition's own guard is read again, so a witness lists the order before the draw, a
   candidate another region's reaction disarms draws nothing, and a seed replays its draw while
   `explore` enumerates the branches; a choice takes `pickBranch` after its effects ran, and a
   history's default transition through a junction draws and records the same way. Pinned by
   `state_junction_several_enabled_branches`, `state_junction_drawn_as_its_transition_fires`,
   `state_history_default_through_junction` (+ trace goldens, `.check.expected.json`),
   `explore_test.go:TestExploreStaticJunctionBranches`, `TestExploreJunctionDrawnAsTransitionFires`,
   `TestExploreHistoryDefaultThroughJunction`, the oracle sections *Two branches of a junction
   enabled when the transition is selected*, *A junction with two branches enabled in a region
   another region's reaction may disarm* and *A history without a record takes its default
   transition through a junction with two branches enabled*; the test passes.

9. **The order in which orthogonal regions are entered, exited and stepped is not a recorded
   choice point.** SM21, SM22, SM23 and SM13 each said so of their own site; taken together the
   sites are one gap, and the PSSM referee measures it: every trace the runtime reached in the
   tests below is one the suite admits, and the suite admits others no policy produced, so
   `explore` reported each run complete after the one interleaving. Four sites. Entering the
   regions of a composite state (`state_region_entry.go:enterRegionsInto`) and a fork's branches
   (`enterForkBranches`) in declaration order — *Entering 010* and *Entering 011* (§9.4.5) admit
   one region's initial-transition effect between the other's entries, *Fork 002* the
   two branch effects in either order and the branch target's entry among them, *History 001-C* and
   *History 002-B* (§9.4.15) the two regions' restored entries and exits interleaved,
   *Terminate 001* and *Terminate 002* (§9.4.13) the second region's entry before the first's.
   Exiting them (`state_executor.go:exitState`) in declaration order, each innermost first —
   *Exiting 001* and *Exiting 003* (§9.4.6) admit the two regions' exits in either order. Firing
   the transitions one occurrence selects one whole firing at a time (`dispatchInOrder`, SM21) —
   *Transition 019* (§9.3.3.12) admits both sources' exits before either effect. And stepping a
   due do action before the occurrence at the head of the pool is dispatched
   (`state_executor.go:runStep`, `runDoRound`, SM13) — *Behavior 003 A* admits the
   machine's `AnotherSignal` transition before the do activity's first segment, so that
   the first state's entry alone is a complete log, *Transition 017* the do activity's step
   at any point among the sibling regions' completion effects, and *Terminate 002* the do
   activity's step before the terminating completion transition or not at all. SysML v2 §7.18.1
   has parallel substates "performed concurrently" and `StatePerformance::do` a sub-performance
   concurrent with `middle`, so the runs PSSM admits are runs v2 admits, and the runtime's one
   order per site is a linearization v2 admits too: not a defect of behavior, a gap of
   exploration — the `explore` driver enumerates the choice points a run records
   (`scheduling.md`), and these sites recorded none.
   *Fixed at three sites*, as [recording the order of orthogonal regions](region-order-scheduling.md)
   lays out: the regions a composite state, a fork or a history enters, the regions a state
   leaves, and the firings one occurrence selects across regions each run as the queues of one
   front (`state_unit_front.go`), a unit — one state's entry, one state's exit, one segment's
   effect — at a time, and each draw of which queue's next unit runs is a choice point the
   policy makes (`ChoiceEntryOrder`, `ChoiceExitOrder`, `ChoiceRegionOrder` per unit; SM22, SM23,
   SM21), written to the trace and the witness, replayed, refused and rolled back with its move,
   and enumerated by `explore` and the checker. `declared` and `reverse` take the order the
   runtime always took, unit for unit, so no event order under either moved; a unit that
   performs no behavior is drawn with the performing unit beside it, so *Event 016 B*'s three
   silent firings across nested regions explore 1152 linearizations rather than some 320 000.
   *Exiting 001*, *Exiting 003*, *Fork 002*, *Terminate 001* and *Deferred 006 C* reach every
   admitted trace and pass; *Transition 019* reaches its six and stays `fail` on SM34 alone.
   *Fixed at the fourth site* at the grain of a token move: a due do step against the
   dispatch the machine would make at the same instant — "dispatch now" against "keep moving
   the do flow" — is drawn under `check`, `replay` and `explore` (`ChoiceStepOrder`,
   `state_executor.go:oneUnit`, `stepDue`), one token move of a due do behavior's flow — an
   inline body's statement, a step of a do behavior given as an action, a token inside a nested
   perform — or the dispatch per move, drawn again after every move while a do behavior is due,
   so a dispatch cuts the flow anywhere or waits for it to rest and a body parked at an
   `accept` offers no move; `declared`, `reverse` and `seed:<n>` finish the whole round before
   they dispatch as they always did, so no default trace moved, and their run is one path of
   the checker's enumeration. *Behavior 003 A* passes; *Transition 017* reaches every placement of
   its do step and stays `fail` on item 11's pool order and the suite's defect
   ([omg-issues](../../project/omg-issues.md#pssm-transition-017-admits-a-parents-completion-before-its-regions)),
   *Terminate 002* on item 11 alone. *Fixed on the entry front too*: once a state's entry
   unit has performed and started its do behavior (`startDoAction`), each due token move of
   that behavior is a unit of its region's queue on the entry front, drawn against the sibling
   regions' remaining entry units under the front's own draw (`state_unit_front.go:offerDoSteps`,
   `ChoiceEntryOrder`'s `entering <state>` with `do <state>` an alternative beside the
   entries — no new choice kind) while a sibling has a unit left, then against the dispatch as
   above; the fixed policies offer no such unit and no golden of theirs moved. *Terminate 002*
   reaches its fifth admitted trace and passes; every site of this item is closed.
10. **A segment leaving a junction inside a composite state runs its effect before the
    composite is entered.** PSSM *Junction 005* (§9.4.11): a transition from outside targets a
    junction that lies in one region of an orthogonal state, and the segment out of the
    junction, `T1.3`, has an effect; the suite admits the orthogonal state's entry, then
    `T1.3(effect)` interleaved with the other region's default entry — every order after the
    owner's entry. The runtime reached `T1.3(effect)` before the owner's entry:
    `state_executor.go:moveTo` ran every effect of the route (`route.effects`) after the exits
    and before `enterBelow` entered the way down to the target, so a segment that lies inside
    the target's ancestor ran before that ancestor's `entry`. UML (PSSM §8.5.8,
    `TransitionActivation::enterTarget` on the way to a vertex owned by a region of the
    orthogonal state) enters that state before the junction is reached, and v2 says the same of
    the project's junction: the transition out of it is declared in the owner's body, an
    `enclosedPerformance` of the owner "happening during the state performance"
    (`StatePerformances.kerml`), so its effect follows the owner's `entry` — the reading the
    oracle's join section and item 7's `defaultHistoryRoute` (a history's default transition
    taken from inside the owner once the owner is entered) already apply. A runtime defect,
    then, of the same family item 7 fixed for the history's default transition.
    *Fixed:* a route's effects carry the state declaring the pseudostate each segment leaves
    (`state_route.go:routeEffect`, from `lower.StateGraph.PseudostateOwner`), and
    `runEffects` runs them in path order, activating the states on the way down to that owner
    first (`enterAhead`, outermost first, each `entry` run once — the move entering them
    afterwards finds them activated and goes on with their regions and do behaviors), so the
    order is the owner's `entry`, the segment's effect, then the entries below it, at every
    depth and for a pseudostate in one region of a parallel state (the owner entered, the
    segment's effect, then every region as usual). `travel` serves a junction and a choice
    alike: before a choice's guards are read, the states down to the choice's own owner are
    activated (`enterOwnerOf`) and, of the rest, only the states every branch enters
    (`certainEntries`, the counterpart of `certainExits`), so the guards read what the owner's
    `entry` and the effects into the choice wrote, and a state no branch shares waits for the
    branch. A history's
    default transition runs the same way from inside the owner (`defaultHistoryRoute` hands its
    settled effects to `moveToHistory`, which enters down to each segment's owner before it).
    A move that activated a state ahead and did not enter it is a typed error, never a silent
    skip. Pinned by `state_junction_inside_composite`, `state_choice_inside_composite`,
    `state_choice_guard_reads_owner_entry`,
    `state_junction_inside_nested_composite`, `state_junction_inside_orthogonal_region`,
    `state_junction_then_choice_inside_composite` and
    `state_history_default_junction_inside_nested` (+ trace goldens); no golden on `develop`
    moved. *Junction 005* now reaches an admitted trace — the owner's entry, `T1.3(effect)`,
    then the other region's `T2.1(effect)` and its target's entry — and misses only the two
    orders in which the other region's initial effect and entry come before or around
    `T1.3(effect)`: the other region's `T2.1` is a completion transition, fired as a step of its
    own after the entry settles, so the two are item 11's and it stays `fail` citing item 11
    alone. What a pseudostate's owner does when the route only passes
    through it — a transition from outside a composite state through its junction to a target
    outside it again — is not entered on the way, as before: the owner lies on no entry chain of
    the move, and no PSSM test or fixture pins that shape.
11. **A completion's firing is not drawn against the entry front — and should not be.** Found
    while fixing item 9's entry site: every admitted trace *Entering 010*, *Entering 011*,
    *Junction 005*, *History 001-C* and *History 002-B* still miss interleaves the firing of a
    **completion transition** — a region's initial transition, translated as a completion out
    of a start state (`T2.1(effect)`, *Entering 010*), or the restored state's exit and its
    successor's entry in the History tests — with the entry units of the sibling region *in the
    same step*. The runtime dispatches a completion as a run-to-completion step of its own once
    the entry move has settled (SM9, SM10), so no draw among the entry units reaches such an
    order. The finding was first read as one gap of exploration, with the fix that a state whose
    entry leaves it complete offers its completion's firing as a unit of its region's queue on
    the front that entered it. Enumerated against the five admitted sets in
    [recording the order of orthogonal regions](region-order-scheduling.md#finding-11-a-pending-completion-inside-the-entry-front),
    that rule reaches every admitted trace and, on three tests, traces PSSM refuses (two on
    *Junction 005*, eight and thirty-one on the History pair), and no narrower rule reaches the
    five; the sets want three different things, none of them that rule. *Adjudicated, not a
    runtime change:*
    - *Entering 010*, *Entering 011* and *Junction 005* are about the **initial transition's
      effect**, which UML runs as part of the region's default entry (UML 2.5.1 §14.2.3.4.5)
      and which SysML v2 cannot place there: an entry transition (§7.18.3
      `EntryTransitionMember`) carries a guard at most, so the referee translates the effect as
      that of an unguarded completion transition out of a behavior-less start state
      (`emit.go:startTarget`), and a v2 completion transition fires by a step of its own
      (SM9). The other spellings fail (the design note's candidate table): folded into the
      target state's `entry` action ahead of its own entry behavior, the effect becomes a step
      of that state's `StatePerformance` (`step entry[1]`, `StatePerformances.kerml`) and runs
      on every entry of the state, not only by the initial transition — *Entering 010*'s
      region 1 enters its initial transition's target explicitly and refuses every trace the
      fold reaches — and makes one unit of two behaviors PSSM interleaves separately (two of
      *Entering 011*'s six), and *Junction 005*'s initial transition ends at a junction, so it
      has no target entry to fold into; folded into the region's entry action it repeats on a
      history restore (*History 001-B*). *Junction 005* pins that the class is right: the real
      completion of its default-entered substate is admitted only *after* the sibling's
      remaining entry unit. A runtime rule that fired a start state's completion in the entry
      front and not that substate's would tell them
      apart by the source performing nothing, which neither v2 nor PSSM does. The three stay
      `fail`, their reasons citing the language difference. Whether that difference takes a
      *differs because v2 differs* row — moving the three to `differs-by-design` through
      `tools/referee/pssm/rows.go:TestRows` — is open decision 8.
    - *History 001-C*'s first half is about the **pool's order** (SM10): PSSM
      generates a completion event as its source is entered and dispatches in generation order
      (§8.5.9), so the entry draw decides the pool's order, which `scheduleTransitionEvents`
      did not follow. A runtime gap, a divergence under `reverse`, `seed:<n>` and `explore`
      alone; *fixed*: a state's completion is queued as its entry unit is performed, and an
      entry that generates one is drawn rather than silent (the design note's *silent units*
      section). As enumerated, it moved no bucket: *History 001-C* reaches both first-half
      orders, *History 002-B* one more admitted trace and the two §8.5.9 gives that the suite
      does not register, *Entering 011* its second, and *Transition 017*'s three finding-11
      traces are reached beside item 9's fourth site.
    - The rest of the History pair is a **suite defect**, recorded in
      `docs/project/omg-issues.md`: *002-B* registers a dispatch order §8.5.9's pool cannot give
      and omits the one it gives, which *001-C* registers for the identical half; both register
      alternatives that dispatch a default-entered state's completion inside the restore step
      their own notes and RTC tables end first, and disagree with each other on whether that
      firing splits around a restored entry. No runtime rule is chosen to reach either; the two
      stay `fail` citing the defect, and against the specification's own text neither can pass
      on the downloaded XMI.
    - *Terminate 002*'s one remaining trace had this shape with a **do step** in place of the
      completion's firing: a due do step of the entered state drawn against a sibling's entry
      unit, item 9's fourth site extended to the entry front, where that site drew it against
      the dispatch alone. Fixed (item 9): the step is a unit of its region's queue on the
      front, and the test reaches the trace and passes. *Transition 017*'s three were
      the pool's order, now reached, beside the suite defect of its two anomalous traces.

    No exploration model changes under this item: the pool's fix added a `ChoiceEntryOrder`
    draw where a completing entry had ridden silently, the do step's fix a `do <state>`
    alternative at the same draw, and no `check` verdict moved. The five tests of the first
    two bullets stay `fail` in `docs/project/pssm-referee.md` with the reasons above.

Item 3 has no fixture on `develop`; the first thing it needs is the conformance case that pins
the behavior, then the fix, in a change set of its own — Track E of the roadmap holds its
entry. Items 4 to 8 and 10 took that path in the change set that decided them, item 9 at three
of its four sites, the fourth in the change set that drew the do step against the dispatch, and
item 11's pool order in the change set that queued completions at entry, and the do step against
a sibling's entry unit in the change set that drew it on the entry front.

## Open decisions

Addressed to the maintainers; each gives the options and the lean.

1. **SM11 — does a composite state's completion fire the composite's own completion transition?**
   *Options:* (i) keep the runtime's rule — completion propagates to the machine, a nil-trigger
   transition out of a composite state is unreachable — and amend the spec-compliance record and
   `orthogonal-regions.md` to say so explicitly with the §7.18.3 sentence quoted; (ii) adopt
   PSSM's rule — the composite generates a completion occurrence, its completion transition fires
   if enabled, the machine ends only when its top-level regions complete — as the reading of
   "does not necessarily terminate immediately", with the trace goldens that change adjudicated.
   *Lean:* (ii), because a `state outer { … then done; } transition first outer then next;` that
   never reaches `next` is a model no v2 author would write with the current meaning intended,
   and because the change belongs before stage 3 of the model checker adds choice points over
   completion dispatch. But the v2 sentence does not decide it, and the goldens that move are the
   cost. *Decided:* (ii), on the library's own text rather than on PSSM's: `States.sysml`'s
   `done` is the `endShot` of the state whose body names it, and `TransitionPerformances.kerml`
   places `transition first outer then next` after `outer`'s performance; see SM11. What the
   library leaves open is the composite with no enabled completion transition: it stays
   completed and active and the machine runs on, since neither PSSM nor §7.18.3 ends anything
   there. The goldens that moved are `state_completion_nested_regions`, `state_entry_transition_nested_done`
   and `state_anonymous_action_body`, each rewritten with a completion transition out of the
   composite so the case they pinned survives, with a `_stay_active` sibling for the first two.
2. **SM30 — dynamic or static choice guards?** *Options:* (i) make the code match
   `pseudostates.md`: evaluate a choice's guards after the incoming effect, keeping a junction's
   static; (ii) make the note match the code: one static rule for both, the choice/junction
   distinction being one of notation. *Lean:* (i). UML's one reason to have two vertices is this
   distinction, the project's note already promises it, and PSSM's *Choice 001* test would be the
   conformance case. The cost is that `resolveRoute` must resolve a choice lazily, after the
   segment's effect, which the route-resolution code does not do today. *Decided:* (i); see
   SM30, and `pseudostates.md` for what a chain of pseudostates does. Only
   `state_region_exit_pseudostate`'s outcome moved: its choice guard now reads the count the
   exit actions wrote.
3. **SM7 — does a deferral outrank a transition in a sibling region?** *Options:* (i) adopt
   PSSM: only a more deeply nested transition overrides a deferral; (ii) keep the runtime's rule
   and document it as the meaning of `defer` in this project. *Lean:* (i), for the reason given
   under option (a); it rewrites `TestEventConsumedByAnotherRegionIsNotDeferred`'s expectation,
   which is the decision's cost and should be made knowingly. *Decided:* (i); see SM7. That test
   is now `TestDeferralOutranksASiblingRegionsTransition`, pinning the sibling's transition
   waiting for the release. With two orthogonal regions each deferring, every deferring state must
   be overridden for the occurrence to fire.
4. **SM28 — empty history with no default transition.** *Options:* (i) adopt PSSM: perform the
   region's default entry; (ii) keep the run failure. *Lean:* (i); the failure protects no v2
   rule, and UML is the extension's stated reference. *Decided:* (i); see SM28. The run failure
   remains, typed, for the one case default entry cannot serve: an owner with no entry transition.
5. **Whether to build option (b) at all, and when.** *Options:* (i) after (a)'s changes land;
   (ii) never — the ten rows are decided by this note and the suite adds only a regression net
   over behavior the conformance cases already pin; (iii) now, in parallel with (a), so the suite
   can be run against the current runtime once as evidence for decisions 1–4. *Lean:* (iii) for
   the one-time evidence if a maintainer has the appetite, otherwise (i). The translator's cost
   is the only argument against, and the capability map bounds it.
6. **Licensing of the harness's download.** PSSM's front matter licenses the specification for
   informational, non-commercial use without modification; a harness that downloads
   `ptc/18-11-06` at build time, translates it in memory and commits only bucket counts does not
   copy or modify the suite in this repository, which is how `cmd/pilot-*` treat the OMG pilot
   corpora. *Options:* (i) proceed on that reading; (ii) ask OMG before building (b). *Lean:* (i),
   by the corpora's precedent; not a decision this note can settle.
7. **Whether the five findings above take issues or a roadmap entry.** *Options:* (i) one roadmap
   Track E entry each for items 3–5 (1 and 2 already have theirs); (ii) issues only. *Lean:* (i),
   so the record that lists "what we don't (yet) support" stays the one place a user looks.
   *Decided:* (i); Track E holds the three entries, items 4 and 5 as landed.
8. **Finding 11 — does an initial transition's effect take a *differs because v2 differs* row?**
   *Options:* (i) add a row saying that SysML v2 spells a UML initial transition's effect only as
   the effect of a completion transition, which fires by a step of its own, and move
   *Entering 010*, *Entering 011* and *Junction 005* to `differs-by-design`; (ii) leave the three
   in `fail` with their reasons naming the difference, as the record does now. *Lean:* (ii), for
   the present: the one *differs because v2 differs* row so far (SM15) names a semantic of a
   construct both languages have, and the runtime implements the v2 side; this one would name a construct
   v2 lacks, and a `differs-by-design` count that grows by three on a translation limit reads as
   conformance gained. The three reasons are precise and stable, and the bucket can be moved in a
   change of its own if the maintainers read it otherwise.
