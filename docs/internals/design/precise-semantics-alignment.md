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
| **Bundled semantic library** | the pilot implementation's library snapshot shipped under `internal/core/libs/stdlib` (EPL-2.0, see its `NOTICE`) | `Kernel Semantic Library/Occurrences.kerml`, `StatePerformances.kerml`, `TransitionPerformances.kerml`, `ControlPerformances.kerml`, `Clocks.kerml` — the text the runtime executes against, quoted where it differs in wording from the specification PDF |

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

Test names are conformance fixtures under `internal/core/runtime/testdata/conformance/` (run by
`TestExecutionConformance`; a `.trace.golden` beside one is compared by `TestExecutionTrace`),
subtests of `TestRuntimeRobustness` in `internal/core/runtime/robustness_test.go`, or unit tests
in the runtime package. File paths are relative to `internal/core/runtime/` unless stated.

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
pins that the completion step follows the entering step. The runtime never reads a redefinition
of `isRunToCompletion`, so it implements the library default only (noted under
*Findings*, as an unsupported v2 feature rather than a PSSM question). **agrees.**

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
`eventHeap.Less`. **agrees** — PSSM and the v2 text coincide, and the runtime's shortfall against
both is listed under *Findings*, not as an alignment question.

**SM10. Priority of completion events, and their scope.** PSSM §8.5.9: completion events are
dispatched before any other occurrence in the pool, in the order generated; a completion event
is bound to the state activation that produced it, and once that state has left the configuration
the occurrence can trigger nothing. *v2/KerML:* silent (no completion occurrence exists in the
library). *Runtime:* `eventHeap.Less` puts completion events first at one timestamp, by ID among
themselves; `dispatchEvent` drops an `EventTime` event whose source "was left before this event
came up" (`inActiveConfiguration`). So a current-time `EventTime` completion transition *is*
equivalent to PSSM's activation-scoped completion event in priority and in scope, with the SM9
finding as the one proviso and one more: the runtime's stamp is the clock instant, so a completion scheduled
at `t` ranks behind an ordinary occurrence still queued with a stamp earlier than `t`; such an
occurrence can only exist if the instant it arrived was left undrained, which `runCounting` does
not do. `state_time_trigger_restarts_on_re_entry` pins the withdrawal of a left state's timer,
which is what makes a stale `EventTime` event rare; the drop itself has no fixture and rests on
`dispatchEvent`. **agrees.**

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
`state_executor.go:enterStateInto` runs the entry body to its end, then `enterRegionsInto`
(each region in declaration order to its initial leaf, entry behaviors along the way), then
`startDoAction` for the state; a substate's do action is likewise started by its own entry.
`state_parallel_entry_behavior`, `state_nested_parallel_entry_exit_behavior`,
`state_do_action_declaration_order`, `state_entry_exit_action_successions`. The one ordering difference — the runtime
enters the regions *before* starting the composite's own do action, PSSM starts the do activity
before entering the regions — is unobservable in the trace, because the do action does not run
until the next do round (SM13) and PSSM's do activity runs asynchronously as well; both leave
"entry, region entries" in that order. **agrees.**

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
here is inadmissible there. **agrees.**

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
`state_call_trigger_guard`, `state_entry_transition_guard_first`. **agrees.**

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
`explore` enumerates the set. `state_choice_transition_conflict`,
`state_explore_transition_conflict`. **agrees**: the runtime's choice is one PSSM admits and is
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
fires the selected transitions one at a time, drawing the next among the candidates still active
from the scheduling policy (`chooseRegion`, declaration order by default) and recording a
`ChoiceRegionOrder`; a firing may leave a sibling's leaf, which is why the draw is per firing.
`state_parallel_broadcast`, `state_explore_region_order`, `state_composite_region_depth_order` (a
`.declared` and a `.seed-1` golden beside the default). **agrees**: every linearization PSSM admits
is a run the policies can produce, and the default is one of them.

**SM22. Entering the regions of a composite state.** PSSM §8.5.5 enters regions concurrently
(*Entering 004*, §9.3.5.2, enters two regions from one transition; *Transition 011-D*'s alternative
trace is the interleaving PSSM admits for the exits).
*v2/KerML:* silent on order among `parallel` substates; §7.18.1 says only that they are performed
concurrently. *Runtime:* `enterRegionsInto` enters the regions sequentially in declaration order
(`state_parallel_standard`, `state_typed_region_order`, `state_parallel_entry_behavior`). No
`ChoicePoint` is recorded for this order — the run is deterministic and the alternative
interleavings PSSM admits are not explored. **agrees** on admissibility, and the missing choice
point is an *Open decision*.

**SM23. Exiting the regions of a composite state.** PSSM §8.5.5 exits regions concurrently as
well (*Exiting 003*, §9.3.6.4, exits nested orthogonal regions). *Runtime:* `exitState` exits the regions' active states in declaration
order, each innermost first (`state_composite_orthogonal_exit` and its trace golden,
`state_composite_nested_regions_exit_once`); like SM22, no choice is recorded. **agrees**, same
caveat.

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
`fireHistoryTransition` re-enters the owner and restores that substate, which for a shallow
history is entered through its own initial transition (`state_shallow_history`,
`state_history_revisit`). **agrees.**

**SM27. Deep history.** PSSM requirement *History 001* (§9.4.15): "the full state configuration of the most recent
visit ... including execution of all entry Behaviors encountered along the way", outermost
first (*History 004*). *Runtime:* `historyRecord` keeps the recorded child per region
recursively; `fireHistoryTransition` with a deep history restores the chain through
`enterStateInto` along the recorded path, running each entry body (`state_deep_history`,
`state_deep_history_region_composite`). **agrees.**

**SM28. History with nothing to restore.** PSSM requirements *History 002–003* (§9.4.15): with no prior visit, or a
region that "had reached its FinalState", the history's outgoing transition (the default history
transition) is taken; with no such transition "standard default entry of the Region is performed"
(the region's initial transition). *Runtime:* `fireHistoryTransition` takes the history's own
outgoing transition when no configuration is recorded; with neither it fails the run with "no
recorded configuration" (`robustness_test.go:history_without_record_or_default`), where PSSM
enters the region's initial state. A region that reached `done` is not a case here, because that
completes the machine (SM11); a region left with no active state is forgotten
(`forgetRegionHistory`). **differs, v2 silent.**

#### Choice and junction

**SM29. Junction: guards read before the step.** PSSM requirement *Junction 001* (§9.4.11): a junction's
outgoing guards "are evaluated before any compound transition containing this Pseudostate is
executed" — statically, as part of deciding whether the incoming transition is enabled — and
*Junction 003*: with several true, one is chosen, algorithm undefined. *v2/KerML:* no junction
in v2; the project's `junction` is a UML-referenced extension (`pseudostates.md`, "a static
conditional branch"). *Runtime:* `state_region_transition.go:resolveRoute` treats a junction as a
`transientPseudostate` and resolves the route — `pseudostateTarget` → `pseudostateBranch`, the
first outgoing transition whose guard holds, in declaration order — *before* the incoming
transition fires, so the guards read the data as it stands before the incoming effect.
`state_junction_pseudostate`, `state_completion_through_pseudostate`. **agrees.**

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

**SM31. Choice with no guard true.** PSSM requirement *Choice 003* (§9.4.10): "the model is considered ill formed".
*Runtime:* `pseudostateBranch` fails the run with "no guard evaluated to true"
(`robustness_test.go:region_pseudostate_without_satisfied_guard`). **agrees.**

**SM32. Junction with no path through.** PSSM requirement *Junction 002* (§9.4.11): when no outgoing guard holds, "the
entire compound transition is disabled even though its Triggers are enabled" — the incoming
transition is not selected, and the occurrence is deferred or lost like any other unhandled one.
*Runtime:* the incoming transition is selected on its own trigger and guard; the failure to route
surfaces as the same "no guard evaluated to true" run failure as SM31, not as a disabled
transition (`region_pseudostate_without_satisfied_guard` uses a junction).
**differs, v2 silent.**

#### Fork and join

**SM33. Fork.** PSSM §8.5.7 (`ForkPseudostateActivation`) and requirement *Entering 010* (§9.4.5, the
forked regions "are entered explicitly and the others by default"): the parent is entered under the
common-ancestor rule, then every outgoing transition fires "without any guard evaluation, since
UML does not allow Transitions outgoing a fork Pseudostate to have guards", each into a different
region of one orthogonal state. *v2/KerML:* no state-body fork in v2 (`fork` is an action node);
the extension follows UML. *Runtime:* `fireForkTransition` → `forkPlan` enters the target
composite with each branch's target as that region's initial configuration; `forkPlan` refuses a
guarded branch ("outgoing transitions cannot be guarded"), a branch outside an orthogonal region
and two branches into one region (`robustness_test.go:fork_branches_share_region`).
`state_fork_join_pseudostate`. **agrees.**

**SM34. Join.** PSSM requirement *Join 001* (§9.4.12): "all incoming Transitions have to complete before execution
can continue through an outgoing Transition"; the join fires when every source state is active
and the occurrence enables all incoming segments. *Runtime:* `fireJoinTransition` fires only when
every state in `joinSources` is active, exiting them all and entering the join's target; a join
with a single incoming branch is refused (`join_with_one_incoming_branch`).
`state_fork_join_pseudostate`. **agrees.**

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
the effect"). **agrees.**

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
state performance"; `Performances.kerml` `TerminatePerformance`. *Runtime:* the parser accepts
`terminate` and lowering preserves it (`lower.EffectTerminate`); `action_statements.go:
actionStmtHost.effect` refuses it with "'terminate' in a body is not executable", and a
transition to a terminate action fails to resolve its target. The refusal is pinned for a
calculation only (`robustness_test.go:calc_terminate_is_rejected`, where it is refused as a side
effect); the runtime has no `terminate` execution for actions or states either. Roadmap Track E
lists it. **gap** — a v2 gap first, and PSSM's rules for it coincide with §7.17.10's, so the
same implementation closes both.

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
classifier behavior starts when the object is started (`StartObjectBehaviorAction`, or on
creation with `isActive`), each behavior in an execution of its own, sharing the object's event
pool; PSSM §8.5.1 makes a state machine such a classifier behavior. *v2/KerML:* §7.18.4 an
`exhibit state` "must be carried out entirely within the lifetime of the performing occurrence";
`Objects.kerml`/`Occurrences.kerml` `performances`. *Runtime:* `Context.Instantiate` →
`classifier_behavior.go:runAttachedBehaviors` starts every exhibited state machine and performed action of a part as
its own executor on the shared bus and clock (`TestInstantiateStartsExhibitedStateMachine`,
`TestExhibitedMachinesOfTwoObjectsAreIndependent`, `TestExhibitedMachineWritesItsObjectsFeatureValues`
in `classifier_behavior_test.go`). One pool per machine rather than per object (SM1) is the one
structural difference, and it is the v2 one. **agrees.**

**SM44. The machine ends; the object does not.** fUML §8.8.1: a classifier behavior completing
does not destroy its object; the object persists, receives occurrences, and handles them with
whatever behaviors remain. PSSM: a machine that reached its final state stays in it, occurrences
addressed to it are lost. *v2/KerML:* `done` ends the state performance, `Life` of the object is
separate. *Runtime:* `completeIfDone` ends the performance (`endPerformanceLife`) and the machine
reports `StateCompleted`; the object remains, later messages to the machine are dropped
(`robustness_test.go:state_event_after_completion`, `state_completion_rests_in_done`,
`classifier_behavior_test.go:TestMessageLeftForACompletedMachineDoesNotBlockANewObject`). **agrees.**

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
by-position versus by-name is notation, not semantics. **agrees.**

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

Sixty-nine rows: 45 for state machines, 14 for actions, 10 for composite structures. Each
carries one verdict.

| Verdict | Rows |
|---|---:|
| **agrees** | 51 |
| **differs because v2 differs** | 6 |
| **differs, v2 silent** | 8 |
| **gap** | 4 |
| **Total** | **69** |

The six **differs because v2 differs** rows are SM15 (a do activity and the machine competing
for one occurrence), SM36 (local transitions), SM37 (internal transitions), A12 (accept event:
a message no accepter takes stays in flight), C6 (behavior ports) and C9 (interface-typed ports
and name-based dispatch). On each, SysML v2 or the Kernel Semantic Library states the rule the
runtime follows, quoted in the row; adopting PSSM, fUML or PSCS there would move the runtime
away from the specification it implements, so none of them is a candidate for a port.

The eight **differs, v2 silent** rows, the only ones on which a port could change behavior
without contradicting v2:

- **SM7** — a deferrable occurrence that also enables a transition in an enclosing state or a
  sibling region: PSSM defers it unless the transition is more deeply nested than the deferring
  state, the runtime lets any enabled transition consume it.
- **SM11** — what the completion of a composite state completes: PSSM fires the composite
  state's own completion transition and the machine goes on, the runtime propagates the completion
  outward to the machine and never fires a completion transition out of a composite state.
- **SM28** — history with nothing to restore and no default history transition: PSSM enters the
  region's initial pseudostate, the runtime refuses the run.
- **SM30** — choice guards: PSSM reads them on arrival, the runtime reads them before the step,
  as for a junction.
- **SM32** — a junction none of whose outgoing guards holds: PSSM disables the compound
  transition and the occurrence is deferred or lost, the runtime selects the incoming transition
  and fails the run.
- **SM45** — destroying an object whose behavior is still performing: fUML stops the behavior and
  destroys, the runtime refuses the destruction.
- **C3** — a connector between multi-valued ends: PSCS instantiates one link per matching pair
  (array pattern) or a full cross product (star pattern), the runtime one connector object whose
  ends span the collections.
- **C8** — a send that reaches no receiver: PSCS loses the occurrence, the runtime fails the
  send with a typed error.

The four **gap** rows:

- **SM38** — terminate (the roadmap's "terminate in a body" entry under Track E).
- **A8** — streaming flows (the roadmap's "streaming flows" entry).
- **A9** — parallel expansion regions (the roadmap's "concurrent per-element performance"
  entry).
- **A10** — interruptible regions (the roadmap's "interrupting an ongoing performance" entry).

Every gap is already a Track E entry with a v2 basis of its own; a port of the precise-semantics
text would not close any of them, because each is a v2 concept the runtime lacks, not a UML
concept v2 lacks. Terminate is the one gap the PSSM suite tests directly (three tests); the
other three are fUML rows no PSSM test reaches.

## The test suite as a referee: a capability map

The question this section answers is the one the pilot execution referee's note asks of the
pilot: how far can the PSSM test suite adjudicate the runtime's behavior, and what would a
verdict from it mean? The comparison assumes a harness that translates each test's UML model
into a `.sysml` model by hand or by rule, drives it with the test's stimulation sequence, and
compares what the translated model records with the expected trace(s). No such harness exists;
nothing below describes one that has been run.

| Aspect of the suite | Verdict | Why |
|---|---|---|
| **Expressing the test model in SysML v2 textual notation** | **Can, for 73 of 103** (37 with standard notation, 33 with this project's extensions, 3 spellable but reaching the terminate gap); **cannot, for 30** | Every test's state machine was classified by the UML constructs it uses; the table below gives the construct-to-notation mapping and the per-area result |
| **Driving the test** | **Can, with one normalization** | PSSM's `Tester` sends `Start` and the follow-up signals from its own behavior, interleaved with the target's steps by fUML's scheduling; the conformance harness queues a case's `events` before the first step (`conformance_test.go:injectEvents`). The two coincide when every send precedes the target's first reaction, which is what the tests' "received when in configuration ..." lists state; a test that needs a signal to arrive mid-run needs a tester `part` in the model instead |
| **Comparing the expected trace** | **Can, on a model-level string; `%trace` is not the comparand** | PSSM's expected trace is built by the model — every entry, exit and effect behavior calls `trace("<state>(entry)")` on the `TraceBuilder` (501 call actions target the `trace` operation in the XMI). Its translation is an `assign log := log + "<state>(entry)"` in the corresponding `entry`/`exit`/`do` body, compared through the case's `slots`/`outputs`; the runtime's `%trace` and `TestExecutionTrace` goldens record steps, not segments, and would need a projection (enter/exit/effect lines to segments, everything else dropped) to be comparable at all |
| **Alternative expected traces** | **Can, and exactly** | 36 tests declare more than one admissible trace. The conformance schema's `outcomes` with the `explore` policy replays a case once per linearization of its choice points (`ChoiceRegionOrder`, `ChoiceTransition`, `ChoiceDueOrder`) and fails when a listed outcome is unreachable or an unlisted one is reached — the same set-equality PSSM's alternatives ask for, and stricter than the single-run comparison the PSSM harness performs |
| **The run-to-completion step table** | **Cannot compare** | Each test's "RTC steps" table lists the pool's contents and the fired transitions per step, including completion events (`CE(<state>)`). The runtime has no pool of completion occurrences (SM9) and the `%trace` records no pool; only the fired transitions and the final trace are comparable |
| **A pass as evidence about SysML v2 semantics** | **Only on the eight `differs, v2 silent` rows and as corroboration on the `agrees` rows** | Where PSSM and v2 coincide (51 rows) a pass says the runtime does what both texts say — worth having, but not a second opinion on v2. Where they differ because v2 differs (6 rows) the corresponding tests fail by design and their failure means nothing. Where v2 is silent (8 rows) a pass or a fail reports on a tool choice, which is the one place the suite is informative about this runtime's rules |

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
| Terminate pseudostate | `terminate` — accepted by the parser, refused by the runtime (SM38) | spellable, runtime gap |
| Entry point, exit point (connection points and connection point references) | none | no spelling |
| Local transition, internal transition | none (SM36, SM37) | no spelling |
| State machine generalization: extended regions, redefined transitions | none | no spelling |

The classification is by construct, in the order of the table: a test whose model uses any
construct with no spelling is counted as not expressible whatever else it uses; otherwise it is
counted under the gap if it uses `terminate`, under the extensions if it uses any of them, and as
standard otherwise. By area:

| Area | Tests | Standard | Extension | Terminate gap | No spelling |
|---|---:|---:|---:|---:|---:|
| Behavior | 5 | 4 | 0 | 0 | 1 |
| Transition | 15 | 8 | 1 | 0 | 6 |
| Event | 16 | 16 | 0 | 0 | 0 |
| Entering | 5 | 4 | 0 | 0 | 1 |
| Exiting | 5 | 4 | 0 | 0 | 1 |
| Entry (entry points) | 6 | 0 | 0 | 0 | 6 |
| Exit (exit points) | 3 | 0 | 0 | 0 | 3 |
| Choice | 5 | 0 | 5 | 0 | 0 |
| Junction | 6 | 0 | 5 | 0 | 1 |
| Fork | 2 | 0 | 1 | 0 | 1 |
| Join | 3 | 0 | 3 | 0 | 0 |
| Final | 1 | 1 | 0 | 0 | 0 |
| Terminate | 3 | 0 | 0 | 3 | 0 |
| History | 8 | 0 | 8 | 0 | 0 |
| Deferred | 10 | 0 | 10 | 0 | 0 |
| Redefinition | 6 | 0 | 0 | 0 | 6 |
| Standalone | 3 | 0 | 0 | 0 | 3 |
| Other | 1 | 0 | 0 | 0 | 1 |
| **Total** | **103** | **37** | **33** | **3** | **30** |

Of the 30 with no spelling, 14 use an entry point, 12 an exit point, 9 a local transition, 2 an
internal transition and 6 the redefinition machinery (several use more than one). Of the 70
expressible and runnable tests, 21 use orthogonal regions, 10 a do activity, 10 deferral, 8
history, 6 a junction, 5 a choice, 6 a fork or join and 6 a call event.

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
SysML v2 conformance. Concretely: of the 70 expressible and runnable tests, every one whose
requirement lands on an **agrees** row checks that the runtime does what UML and v2 both say;
the tests that land on SM7, SM11, SM28, SM30 and SM32 — *Deferred 004-A/B* and their kin, whose
deferring state has a competing transition in a sibling region; the tests whose composite state
owns a completion transition; a history test entered with nothing recorded and no default; a
choice whose guard reads what the incoming effect wrote; and *Junction 002* — are the ones that
would report on a tool choice; and the 30 tests
with no spelling, together with any test that reaches SM15, SM36 or SM37, would fail for reasons
that are v2's, and a harness would have to exclude them by classification rather than report
them as failures. Used that way, the suite is a second opinion on eight rows and a regression
oracle for fifty-one; it is never a conformance statement about SysML v2.

## Options

Four courses of action, each judged on scope, on what it depends on, on its order against the
state-machine stage of the [bounded model checking](bounded-model-checking.md) note (state
executor snapshot/restore, dispatch-order choice points, time ties — stage 3 there) and against
Track E of the roadmap, on its acceptance gate, and on what a user would see.

### (a) Keep the v2/KerML position; adopt PSSM's rule on the `differs, v2 silent` rows where it is better grounded

- **Scope.** Eight rows. For each, decide between the runtime's rule and PSSM's, record the
  decision in the row and in `docs/project/spec-compliance.md`, and implement the changes that
  fall out. The map suggests the split: **adopt PSSM** on SM7 (deferral outranks a transition in
  an enclosing state or a sibling region — the runtime's rule makes `defer` ineffective whenever
  any other region reacts, which defeats the purpose of the extension) and SM28 (an empty history
  with no default falls back to the region's initial transition — a refusal serves no v2 rule,
  and UML is the extension's reference); **keep ours, and say so** on SM30 and SM32 (the
  runtime's static evaluation of choice and junction guards is one rule for both vertices and is
  what makes `pseudostates.md`'s reading of a junction hold; a change would split them), on SM45
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
  `terminate-gap`, `differs-by-design` for the tests reaching SM15/SM36/SM37) that CI compares
  by count and never by pass/fail.
- **Dependencies.** The translator's hardest part, the `Tester`/`Target` protocol, is a
  fixed pattern in the suite (one `Start`, then the listed signals), so the harness's driver is
  the conformance harness's `injectEvents` and nothing more; tests that need mid-run arrival
  would be bucketed, not driven. Independent of the bounded-model-checking stages, since the
  `explore` policy already exists; it *benefits* from stage 3 (dispatch-order choice points make
  the set comparison exhaustive for the orthogonal-region tests rather than budget-bounded). No
  Track E dependency, but the three terminate tests stay in their bucket until Track E closes
  terminate, after which they move to `pass`/`fail` and the count moves.
- **Acceptance gate.** The harness reproduces its committed baseline deterministically; the
  `pass` bucket is not a CI gate, only its *count* is, adjudicated on every movement like the
  corpus ratchets. The tool's `-h` says in one sentence what a pass means, in the words of the
  paragraph closing the capability map.
- **User-visible change.** None to the runtime beyond (a). A maintainer gains a second oracle for
  the eight rows and a regression net of about fifty translated machines.
- **Cost.** A UML XMI reader for the suite's subset (state machines, regions, vertices,
  transitions, triggers, opaque behaviors whose bodies are `trace(...)` calls, signals) and a
  `.sysml` emitter; a checksum-pinned download script; a baseline document under `docs/project/`
  in the shape of `pilot-execution-referee.md`. It is the size of `cmd/pilot-exec-diff`, not
  of an executor.

### (c) A user-selectable PSSM-conformant execution mode

- **Scope.** A `-semantics pssm` (or `%semantics`) switch under which the state executor follows
  PSSM on every row where it differs: SM7, SM11, SM15, SM28, SM30, SM32, SM36, SM37, SM45, plus
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
  either supports both or is silently wrong under one. Track E's terminate closes for both modes
  at once (the rules coincide, SM38); its interrupting-performance and streaming entries are fUML
  rows the mode would not touch.
- **Acceptance gate.** The PSSM suite itself, passing on every expressible test — which is the
  one thing this option buys that (b) does not, and only for the 70 expressible tests, since the
  30 with no spelling need new notation first.
- **User-visible change.** A mode switch a user has to understand, whose meaning ("this model
  now runs as UML") contradicts the architecture's stated position; two answers to "what does
  this model do"; and the `differs because v2 differs` rows, where the mode would make the runtime
  disagree with the SysML v2 text it implements. This is the cost the question asks for stating
  plainly: the two semantics are not two configurations of one engine but two engines, and the
  project's normativity rule cannot hold for both.

### (d) Do nothing

- **Scope.** Leave the eight `differs, v2 silent` rows as they are, this note as the record
  that they were examined, and the four gaps to Track E.
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
for porting the precise-semantics family: 51 of 69 rows agree already, 6 differ because SysML v2
says otherwise and must stay as they are, and the 4 gaps are v2 gaps Track E already owns. What
remains is eight tool choices, and on two of them — SM7 and SM28 — PSSM's rule is the reference
this project's own extensions name (UML) applied consistently, while ours is an accident of
implementation order; on the other six the runtime's rule is deliberate and better for a modeler
(typed errors over silent drops, one guard-evaluation rule for both pseudostates, the library's
connector model), and the row records why. Option (b) is worth having *after* that, as an
advisory oracle in the established `cmd/pilot-*` shape and with its meaning stated in its own
words: an advisory PSSM comparison tests reproduction of UML behavior unless the model and
semantics have a defensible SysML v2 mapping, and it is never proof of SysML v2 conformance.
Option (c) is rejected: two normative interpreters break the analysis framework's rule, double
stage 3 of the model checker, and make the runtime disagree with the v2 text on six rows for
users who select the mode. Option (d) is rejected because SM7 and SM28 are findings this note
has now made and leaving them stands the extensions on UML for their syntax and on nothing for
their semantics.

Nothing here changes the architecture's position. SysML v2 and the Kernel Semantic Library
govern; UML 2.5.1 — and now, on the state-body extensions, PSSM's reading of UML — is the
reference where v2 has no production and the library no performance; the runtime is not a fUML
activity engine and does not become one.

## Findings about our own conformance

The rows below report the runtime differing from, or falling short of, SysML v2's or the Kernel
Semantic Library's *own* text, or from this project's own design notes. They are bug reports and
unsupported-feature records, not alignment questions: PSSM has nothing to do with them and they
are not fixed in this note's change set. Each names its evidence.

1. **Terminate is parsed and lowered but not executed** (SM38). SysML v2 §7.17.10 and §7.18.3
   define `terminate`; `Performances.kerml` provides `TerminatePerformance`; the parser accepts
   it and `lower.EffectTerminate` carries it; `action_statements.go:actionStmtHost.effect`
   refuses it as "'terminate' in a body is not executable"
   (`robustness_test.go:calc_terminate_is_rejected`). Roadmap Track E, "terminate in a body".
   That PSSM's *Terminate 001–002* describe the same behavior is a coincidence of the two texts
   and does not make this a PSSM alignment item: the implementation follows §7.17.10, and the
   PSSM tests would then pass as a consequence.
2. **Streaming flows, parallel expansion and interrupting an ongoing performance** (A8, A9,
   A10). `Flows.sysml` distinguishes `Flow` from `SuccessionFlow` and SysML v2 §7.16.1 says a
   streaming flow may be ongoing while both actions perform; the runtime applies every flow at
   source completion (`action_frame.go:applyDataFlows`) and `lower.ObjectFlow` carries no flow
   kind. Roadmap Track E, "streaming flows", "concurrent per-element performance",
   "interrupting an ongoing performance". Recorded there; not re-scoped here.
3. **`isRunToCompletion` and `runToCompletionScope` redefinitions are not read** (SM1).
   `Occurrences.kerml` declares both as redefinable features with defaults; the runtime
   implements the defaults (one occurrence per step, the whole machine as scope) and never
   consults a model's redefinition. A model that redefines them is accepted and run under the
   defaults without a diagnostic. Unsupported v2 feature; no fixture on `develop` exercises a
   redefinition.
4. **A completion transition whose guard turns true between completion and dispatch is never
   scheduled** (SM9). SysML v2 §7.18.3 lets an unaccepted transition trigger whenever its guard
   holds during the source's performance; `scheduleCompletionTransitions` reads the guard at
   completion and schedules only the transitions then enabled, so a guard made true by a sibling
   region's completion transition dispatched first is missed. Follows from
   `scheduleCompletionTransitions` and `eventHeap.Less`; no fixture on `develop` reaches it.
5. **A composite state's completion ends the machine and never fires the composite's own
   completion transition** (SM11). SysML v2 §7.18.3 says a transition to `done` completes "the
   containing state performance" and that the containing state "does not necessarily terminate
   immediately"; `completeIfDone` → `machineComplete` propagates the completion to the machine and
   `scheduleFromLeaf` never schedules a nil-trigger transition out of a composite state
   (`state_completion_nested_regions`). The v2 sentence is at least uneasy with this; whether it
   forbids it is the first open decision below. The spec-compliance record states the current
   rule as adopted, so this is a finding against the specification text, not against the record.
6. **Choice guards are read before the incoming effect runs, where the project's own note says
   otherwise** (SM30). `pseudostates.md` describes a choice as "a dynamic conditional branch whose
   outgoing guards are evaluated when the choice is entered"; `resolveRoute`/`pseudostateBranch`
   pick the branch before the incoming transition's effect runs, and the code comment says the
   two pseudostates are "indistinguishable for a guard over state data". v2 has no choice vertex,
   so this is not a v2 finding; it is a disagreement between a design note and the code, and one
   of them has to change (second open decision). `state_choice_pseudostate` does not reach it.

Items 3–6 have no fixture on `develop`; the first thing each needs is the conformance case that
pins the behavior, then the fix or the documentation change, in a change set of its own.

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
   cost.
2. **SM30 — dynamic or static choice guards?** *Options:* (i) make the code match
   `pseudostates.md`: evaluate a choice's guards after the incoming effect, keeping a junction's
   static; (ii) make the note match the code: one static rule for both, the choice/junction
   distinction being one of notation. *Lean:* (i). UML's one reason to have two vertices is this
   distinction, the project's note already promises it, and PSSM's *Choice 001* test would be the
   conformance case. The cost is that `resolveRoute` must resolve a choice lazily, after the
   segment's effect, which the route-resolution code does not do today.
3. **SM7 — does a deferral outrank a transition in a sibling region?** *Options:* (i) adopt
   PSSM: only a more deeply nested transition overrides a deferral; (ii) keep the runtime's rule
   and document it as the meaning of `defer` in this project. *Lean:* (i), for the reason given
   under option (a); it rewrites `TestEventConsumedByAnotherRegionIsNotDeferred`'s expectation,
   which is the decision's cost and should be made knowingly.
4. **SM28 — empty history with no default transition.** *Options:* (i) adopt PSSM: perform the
   region's default entry; (ii) keep the run failure. *Lean:* (i); the failure protects no v2
   rule, and UML is the extension's stated reference.
5. **Whether to build option (b) at all, and when.** *Options:* (i) after (a)'s changes land;
   (ii) never — the eight rows are decided by this note and the suite adds only a regression net
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
7. **Whether the six findings above take issues or a roadmap entry.** *Options:* (i) one roadmap
   Track E entry each for items 3–6 (1 and 2 already have theirs); (ii) issues only. *Lean:* (i),
   so the record that lists "what we don't (yet) support" stays the one place a user looks.
