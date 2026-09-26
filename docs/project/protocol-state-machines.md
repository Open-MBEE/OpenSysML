# Protocol state machines: what SysML v2 has, and what the runtime does with it (roadmap E5)

> **Labels.** This is an engineering record. `E5` is the roadmap item under
> [Track E of the roadmap](roadmap.md#e5--protocol-state-machines-design-record-landed); `SM…` numbers refer to the
> state-machine rows of `docs/internals/design/precise-semantics-alignment.md`. A reader outside
> the repository needs none of them.

The roadmap item asked one question and refused to invent an answer: UML 2.5.1 has a *protocol
state machine* (§14.4) that states the legal order of operation calls and receptions on a
classifier; SysML v2 has no notation named that; is there a **standard** SysML v2 or KerML
construct that says the same thing, and does OpenSysML already execute or check it? This record
answers from the specification text, the standard library as vendored, the OMG corpora and the
runtime as it stands; the probes it reports needed no change to the runtime.

## The question

A UML protocol state machine (`ProtocolStateMachine`, `ProtocolTransition`, `ProtocolConformance`;
UML 2.5.1 §14.4) is attached to a classifier — typically an interface or a port's type — and, read
at run time, guarantees four things (paraphrased from the clause; the normative text is not quoted
here, see *Sources*):

1. **Order.** Each transition names an operation (or a reception) as its trigger; the machine's
   states and transitions therefore enumerate which operations may be called in which order.
   An operation not enabled in the current state is a *protocol violation*, not an ordinary
   unhandled event.
2. **Pre- and post-conditions.** A protocol transition carries a guard read as the operation's
   precondition and a post-condition on the classifier's state after the call; it has no effect
   behavior of its own. The machine constrains behavior, it does not perform it.
3. **Conformance.** `ProtocolConformance` relates a behavior state machine (the one that actually
   runs) to the protocol it must obey; a general classifier's protocol constrains its
   specializations.
4. **Static and dynamic reading.** The same machine is a checkable specification for a caller
   (which sequences are allowed) and a run-time monitor for the callee (which arrivals to refuse).

The design question is which of these a SysML v2 model can *spell* with standard constructs, and
which of those spellings OpenSysML runs today.

## What the specification offers

### SysML v2 (v2.0, clause 7)

- **§7.18.4 — exhibited states.** A part, and any occurrence, may `exhibit state`: the exhibited
  state machine is performed by the exhibiting occurrence and "must be carried out entirely within
  the lifetime of the performing occurrence". Nothing in the clause restricts the exhibiting
  definition to parts: a `port def` is an occurrence definition (its base is `Ports::Port`, which
  specializes `Objects::Object`, itself an `Occurrence`), so a port definition may exhibit a state
  machine, and so may an `interface def` (`Interfaces::Interface :> Connection`).
- **§7.18.3 — transitions.** A transition has at most one of each optional clause, in this order:
  `first <source> [accept <trigger>] [if <guard>] [do <effect>] then <target>`. The trigger is an
  `AcceptActionUsage`; the guard is a Boolean expression over the performer's features; the effect
  is optional. Dropping the effect leaves a
  transition that is exactly a "guarded change of state on a reception" — the shape of a protocol
  transition minus its post-condition.
- **§7.17.8 — accept actions and `via`.** `accept <payload> via <port>` accepts a transfer whose
  receiver is that port of the performer; without `via`, the performer itself is the receiver. A
  machine exhibited *by a port* therefore accepts, with a bare `accept`, exactly the transfers
  addressed to that port.
- **§7.12 — ports.** A port usage is an object in its own right, with directed features (`in item`,
  `out item`) that state what may flow across it; an interface connects two ports. Nothing in the
  clause orders those flows in time.
- **Operations.** SysML v2 has no `Operation` metaclass. A "call" is `perform` of an action
  owned by the target (§7.16), or in the KerML reading a `Performance` whose performer is the
  object; a "reception" is a `MessageTransfer` accepted by an `accept`. The nearest spelling of
  UML's "call event as trigger" is an `accept` of a signal type, or a transition triggered by the
  performance of a named action of the same occurrence — which the pilot corpora do not use.

### KerML (v1.0, clause 7 and the Kernel Semantic Library, §8.3.18 / §8.4)

- **`StatePerformances.kerml`** (`StatePerformance`, `TransitionPerformance`): a state
  performance has `acceptable: Transfer[*]`, `accepted: Transfer[0..1] subsets acceptable` and
  `deferrable: Transfer[0..*] subsets acceptable`; the library invariant `isEmpty(accepted) ==
  isEmpty(acceptable)` says a state with something acceptable accepts *one* of them. The
  vocabulary is "which transfers this state may accept", which is the run-time half of a protocol —
  but the library says nothing about a transfer that arrives and is *not* acceptable: it is simply
  not the `accepted` one. There is no `violation`, `refused` or `unexpected` feature.
- **`Transfers.kerml`**: `Transfer :> Occurrence` with `source`/`target` and payload;
  `MessageTransfer`; `TransferBefore :> Transfer, HappensBefore` for a transfer that must
  complete before its target proceeds. Ordering of *one* transfer relative to *one* occurrence,
  not a language of legal sequences.
- **`Occurrences.kerml` / `OccurrenceFunctions.kerml`**: `HappensBefore`, `HappensDuring`,
  `HappensWhile`, `successors`/`predecessors`, and the functions over them. A model can write
  `constraint { open.happensBefore(read) }` or a `succession` between two performances. This is a
  *constraint over particular occurrences* — it says two given performances are ordered; it does
  not say "every `Read` on this port is preceded by an `Open` with no `Close` between", which is
  what a protocol machine states over an unbounded sequence.
- **Metamodel (§8.3.18 in SysML, `SysML.sysml`)**: `ExhibitStateUsage` redefines
  `performedAction` as `exhibitedState : StateUsage[1..1]`; `Parts::Part` carries
  `exhibitedStates : StateAction[0..*]`; `Ports::Port :> Object`; there is no metaclass, library
  type or keyword whose name or documentation mentions a protocol.

**Conclusion from the text.** SysML v2 does not have a protocol state machine as a distinct
construct, and neither the specification nor the library defines a "protocol violation". What it
has is a *behavior* state machine that any occurrence — a port included — may exhibit, whose
transitions may have no effect, and whose triggers are receptions on the exhibiting port. That is
guarantee 1 (order) spelled as an ordinary machine; guarantee 2 is partly spelled by the guard;
guarantees 3 and 4 have no spelling at all.

## What the corpora write

Searched: the OMG training corpus (`examples/sysml-v2-training`) and the three pilot corpora
(`examples/pilot-corpora/kerml-examples`, `sysml-examples`, `sysml-validation`), 374 model files
at the pinned revisions.

| Pattern | Lines / files | What it is |
|---|---|---|
| `accept … via <port>` | 29 / 13 | Transitions of a *part's* exhibited machine, routed by port |
| `HappensBefore` | 17 / 5 | Library declarations and the `Occurrences` examples; no user-model protocol |
| `HappensDuring` | 11 / 2 | Same |
| `exhibit` inside a `port def` or `interface def` body | 0 / 0 | — |
| `Protocol` (any case) | 0 / 0 | — |

The representative model is the pilot's `Interaction Sequencing Examples/ServerSequenceRealization-2.sysml`:
`part server_2` owns two ports and an `exhibit state serverBehavior` whose transitions
`accept sub : Subscribe via subscriptionPort` and `accept pub : Publish via publicationPort` —
a behavior machine on the part, ordering receptions across its ports, with a `do send`
effect. The corpora never place the machine on the port definition and never write a
machine with no effects whose purpose is to constrain rather than behave. Whatever the OMG
authors mean by "protocol" they write as the part's own behavior.

## What OpenSysML runs today

| Guarantee | SysML v2 spelling | Runtime | Evidence |
|---|---|---|---|
| 1 Order, machine on the part | `part def P { port p; exhibit state m { … accept S via p … } }` | Runs at materialization of the part (`lower.ExhibitedState`, `runtime/classifier_behavior.go:startClassifierBehaviors`); a message to `p` reaches the transition (`state_executor.go:matchesEvent`, `acceptsSignalFrom`) | `classifier_behavior_test.go:TestInstantiateStartsExhibitedStateMachine`, conformance `state_transition_accept_via_port`, `accept_via_bound_context_port`; compliance rows *Classifier behaviors* and *`accept … via <port>`* |
| 1 Order, machine on the port definition | `port def FilePort { in item open : Open; exhibit state protocol { … accept Open then opened; … } }` | Runs at materialization of the port object, which is an object of the port definition; a bare `accept` takes a transfer addressed **to the port object itself** (the REPL's `%send … to #1.f`). A transfer a model action sends `to file.f` is routed to the port *of the part* (`signal.go:deliveryOf` → `DeliverPort`) and the port's own machine does not take it: it stays on the bus | Probed against the tree, not a fixture — see *Probes* below |
| 1 Order, an arrival the current state does not accept | — | **Directly injected event** (`StateExecutor.SendSignal`, a debugger driving one machine): dispatched, no transition fires, dropped and reported (`advance.go:dueProgress.noteDispatch`, `AdvanceReport.Dropped`); the machine stays in its state. **Model-posted message** (an action's `send`, `signal.go:Context.postFrom`): left on the context-wide bus — a machine takes a message only when its active configuration accepts or defers it (`state_executor.go:takesMessage`, `acceptableMessage`), so nothing is dispatched, nothing is reported, and the message is **taken later** by the first state that accepts it. **Debugger surface:** the REPL's `%send` refuses to enqueue it — `object … accepts no signal Read now: state machine "protocol" in state closed` (`frontend/repl/send.go`) | `state_deferred_test.go:TestUndeferredEventIsDroppedWhereNoTransitionHandlesIt` (direct injection only), `robustness_test.go:testCallOfUnhandledOperation` (state stays `waiting`), `repl/send_test.go` (`accepts no signal … now`); the model-posted case has no test — see *Probes* |
| 1 Order, deferral | `state s { defer S; }` | Held and re-dispatched after the state is left | `TestDeferredEventIsDeliveredAfterLeavingTheDeferringState`, conformance `state_deferred_event` |
| 2 Pre-condition | `accept S if <guard>` | Guard evaluated against the performer at dispatch | compliance *State Machine* rows for guards; conformance `state_transition_guard_exit_effect_entry_order` |
| 2 Post-condition | no spelling (a transition has no post-condition slot) | — | — |
| 2 "Constrains, does not perform" | a machine whose transitions have no `do` | Runs like any other; nothing marks it as a protocol or forbids effects | — |
| 3 Conformance of a behavior machine to a protocol machine | no spelling | — | — |
| 4 Static reading (legal call sequences checked for a caller) | no spelling | — | — |
| Order of *operation calls* (perform, not send) | `perform action` on the object | `runtime/invoke_operation.go` executes the action directly; the object's exhibited machine is not consulted and receives no event. `StateExecutor.InvokeOperation` injects an `EventCall` into one machine, but only when a debugger drives that machine directly | compliance *Classifier behaviors*: "an operation call … does not travel over connections"; `pssm-referee.md` on call events whose results do not return to the caller |
| Occurrence-order constraints (`HappensBefore`, `succession`) | `first a then b`; `constraint { a.happensBefore(b) }` | Successions between action nodes and state transitions are executed as the graph; a user-written `HappensBefore` constraint or a `happensBefore` call in a `constraint` is not evaluated at run time over recorded performances | compliance *Structural, Interface and Analysis Notation* (§8.3.9.11 Occurrences); nothing asserts over a performance log |

### Probes

The models are not in the repository; they are reproduced here so a follow-up can turn them into
fixtures.

**Probe 1 — a port definition's machine, driven from the debugger.**

```sysml
package ProbePortMachine {
    attribute def Open; attribute def Read; attribute def Close;
    port def FilePort {
        in item open : Open; in item read : Read; in item close : Close;
        attribute reads : ScalarValues::Integer = 0;
        exhibit state protocol {
            entry; then closed;
            state closed; state opened;
            transition first closed accept Open then opened;
            transition first opened accept Read do assign reads := reads + 1 then opened;
            transition first opened accept Close then closed;
        }
    }
    part def File { port f : FilePort; }
    part file : File;
}
```

REPL transcript, abridged: `%instantiate ProbePortMachine::file` materializes `file` and its port
`#2 of "#1.f"`, whose exhibited machine starts in `closed`. `%send Read to #1.f` is refused —
`accepts no signal Read now: state machine "protocol" in state closed`. `%send Open to #1.f` then
`%advance` moves to `opened`; two `Read`s each fire `opened -> opened` and `file.f.reads` reads
`2`; `Close` returns to `closed`, after which `Read` is refused again. So a machine exhibited by a
port definition is lowered, started with the port object and driven by messages addressed to the
port object, and the debugger reports an out-of-order arrival by name and state.

**Probe 2 — the same arrivals sent by the model.** The sender is a sibling machine on the
enclosing part:

```sysml
part def System {
    part file : File;
    exhibit state driving {
        entry; then run;
        state run {
            entry {
                send new Read() to file.f;
                send new Open() to file.f;
                send new Read() to file.f;
            }
        }
    }
}
```

Instantiating `System` and advancing (`Context.Instantiate`, `Context.Advance(5)`) gives, in two
variants:

- *Machine on the port definition* (`FilePort` as in probe 1): all three messages remain in
  `Context.PendingMessages()` after the advance (`Delivery: DeliverPort`, `Object` the `File`,
  `PortID` the port), the port's machine stays in `closed`, `reads` is `0`, `AdvanceReport.Dropped`
  is empty. A message routed to the port of a part is not taken by the port object's own machine.
- *Machine on the part* (`File` exhibiting `protocol`, triggers `accept Open via f`,
  `accept Read via f` — the corpora's spelling): the bus is empty after the advance, the machine is
  in `opened`, and `reads` is **`2`**: the `Read` sent while the machine was in `closed` was neither
  dropped nor reported; it waited on the bus and was taken once `Open` had moved the machine to
  `opened`. `AdvanceReport.Dropped` is empty.

So at the model surface the runtime does not refuse an out-of-order reception; the context-wide
bus holds it until some state accepts it, which is an implicit, unbounded deferral that the model
did not write (`defer` is the spelling for it, §7.18.3). UML's behavior-state-machine semantics,
and the PSSM suite the referee runs, discard an event the active configuration neither accepts nor
defers; the direct-injection path (`SendSignal` → dispatch → `Dropped`) does that, the bus path
does not.

## The gap

Against the four guarantees:

- **Guarantee 1 is spelled, and is enforced only at the debugger.** The ordinary behavior state
  machine — on the part with `accept … via`, as the corpora write it — states the legal order of
  *receptions*, and the runtime fires its transitions in that order. But a reception the active
  state does not accept is not refused when a model sends it: it waits on the bus and is taken by
  a later state (probe 2), so a model that violates the order it declared runs as if it had
  respected it. Only the REPL's `%send` refuses such an arrival, and only a directly injected event
  is dropped and reported. For **operation calls** (`perform` of the object's action) the guarantee
  is not spelled at all: a call runs whether or not the object's machine is in a state that would
  accept it.
- **A machine on the port definition is reachable only from the debugger.** It runs and reacts to
  messages addressed to the port object (probe 1), but a model's `send … to part.port` is routed to
  the port of the part and is not taken by the port object's machine (probe 2).
- **A protocol violation has no run-time identity.** Nothing in the runtime names a machine, a
  state and a transfer as "refused"; the REPL message is the only place the notion exists.
- **Guarantees 2 (post-condition), 3 (conformance) and 4 (static checking) have no SysML v2
  spelling**, standard or otherwise, and no library vocabulary to anchor one on.

## Options

**A — Close: not a SysML v2 construct; receptions covered by exhibited machines.** Record that
"protocol state machine" is a UML metaclass with no SysML v2 counterpart, that the order of
*receptions* on a port or part is stated by an exhibited behavior state machine (§7.18.4 +
§7.17.8), and that post-conditions, conformance and static checking are out of scope because the
language does not spell them. Cost: this record. Leaves: the machine states an order the runtime
does not enforce for model-posted messages (probe 2), so the compliance bullet could not honestly
say the construct runs; operation calls unchecked; no typed violation. **Rejected** on the first
point: the item asked whether OpenSysML *executes or checks* the construct, and for the model
surface the answer is no.

**B — Keep open; make the runtime discard, and report, a model-posted reception the active
configuration neither accepts nor defers (follow-up).** Two changes in the state executor, no IR
change, no new notation:

1. *An addressed message no machine of its destination takes is consumed and dropped, not held.*
   When a message with a destination (`DeliverPort`, `DeliverPortReceiver`, `DeliverReceiver`,
   `DeliverObject` — not `DeliverAnyone`, which has no destination to hold it to) reaches a
   performer whose exhibited machines are all started and none of which accepts or defers it in
   its active configuration (`takesMessage` false for every one), the machine that would have been
   its receiver takes it off the bus and dispatches it as a non-firing `Dispatch`, so it lands in
   `AdvanceReport.Dropped` through `dueProgress.noteDispatch` exactly as a directly injected event
   does. The REPL's `droppedDispatchNote` then reports it unchanged. This is the UML/PSSM discard
   rule the direct path already implements; a message to an object with no started machine, or
   with none at all, stays on the bus as today (an action's later `accept` may be its consumer).
2. *A port definition's machine takes the messages routed to its port.* `acceptableMessage`'s
   reach test (`m.reaches(name, m.Port, objectID(e.self))`) must accept a message whose `PortID`
   is the port object performing the machine, not only one whose `Object` is.

Optionally, on top: an opt-in execution policy under which such a drop ends the advance with a
typed error naming the machine, its active state and the transfer (`ErrUnacceptedTransfer`, say),
for a model whose machine is *meant* as a protocol. Proof: probe 2, both variants, as conformance
fixtures (`state_model_send_out_of_order_dropped`: final state `opened`, `reads` **`1`**, one
dropped dispatch naming `Read`; `state_port_def_exhibited_machine`: the port machine reaches
`opened`, `reads` `1`), a trace golden for the dispatch order, a runtime test posting through
`Context.PostMessage` (not `SendSignal`) and asserting `AdvanceReport.Dropped`, a robustness case
for the typed error, and probe 1 as a REPL `send_test.go` case. Risk: every fixture whose sender
runs before its receiver's machine has started, or whose message is taken by a state entered after
the send, now drops it — the conformance suite, `state_deferred_event` and the PSSM referee must be
run to find them, and each is either the bug this fixes or a sender to be moved. Cost: one session.

**C — Invent a protocol construct** (a `protocol` keyword, or a metadata annotation on an
exhibited machine that forbids effects and turns unaccepted arrivals and unenabled operation
calls into errors). Rejected by the roadmap's own rule — no target is to be invented — and by the
compliance record's rule that non-standard notation is warned about. Nothing in the corpora asks
for it.

**D — Run-time evaluation of `HappensBefore`/`OccurrenceFunctions` over a performance log.**
Independent of protocols: it would let a `constraint` assert an order between two named
performances, which is a different (and weaker) statement than a protocol machine's. Belongs
with the occurrence-semantics work, not here.

## Recommendation

**Option B: keep the item open with the follow-up above; do not reword the compliance bullet as
covered.** The language finding stands: SysML v2 does not have protocol state machines and does
not need a separate construct for the half of the idea it can express — the legal order of
*receptions* on a port or a part is an exhibited behavior state machine, and what SysML v2 cannot
express (ordering *operation calls*, post-conditions, conformance between machines, static
sequence checking) is a UML feature the language dropped, not an OpenSysML gap. But the item also
asked whether OpenSysML executes or checks that construct, and at the model surface it does not:
a reception the active state does not accept is held on the bus and taken later (probe 2), which
is neither the machine's declared order nor the UML discard rule the direct-injection path
follows, and a machine on a port definition never sees a model's messages at all. Until the
follow-up lands, the compliance bullet should say that the notation runs and the order it declares
is enforced only for debugger-injected events.

The follow-up should settle two routing points first, since both decide which machine drops and
what the error names: whether the port machine and the owning part's `accept … via f` machine
share one arrival or the first taker wins (each machine has its own pool, SM1), and whether a
message addressed to an object whose only consumer is an action not yet at its `accept` stays on
the bus (it should).

## Sources

- SysML v2 v2.0, §7.12 (ports), §7.16 (actions, `perform`), §7.17.8 (accept actions), §7.18.3
  (transitions), §7.18.4 (exhibited states); §8.3.18 (states, metamodel) — read as the vendored
  `internal/workspace/libs/stdlib/Systems Library/SysML.sysml`, `Ports.sysml`, `Parts.sysml`,
  `States.sysml`.
- KerML v1.0, Kernel Semantic Library: `StatePerformances.kerml`, `Performances.kerml`,
  `Transfers.kerml`, `Occurrences.kerml`; Kernel Function Library: `OccurrenceFunctions.kerml`
  (all vendored under `internal/workspace/libs/stdlib/Kernel Libraries/`).
- UML 2.5.1 §14.4 Protocol State Machines — the four guarantees above are a paraphrase from the
  clause's structure (`ProtocolStateMachine`, `ProtocolTransition` with pre/post-conditions and
  no effect, `ProtocolConformance`); the OMG PDF was not readable from this environment, so no
  sentence of it is quoted.
- Repository: `docs/project/spec-compliance.md` (*State Machine*, *Classifier Behaviors*,
  *Structural, Interface and Analysis Notation*), `docs/project/pssm-referee.md` (call events),
  `docs/internals/design/precise-semantics-alignment.md` (SM1 pools, behavior ports,
  `accept … via`, signal versus operation routing).
