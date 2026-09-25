# Exception handlers: adjudicated and closed

UML 2.5.1 activities have an *exception handler*: an `ExceptionHandler` attached to a protected
node, catching an exception of a given type raised inside it by a `RaiseExceptionAction` (or by
the runtime) and running a handler body in its place, the exception propagating outward through
enclosing nodes until a handler matches. fUML 1.4 executes this model, and OpenSysML's
[fUML referee](fuml-referee.md) already classifies its exception tests `not-expressible`. The
[compliance mapping](spec-compliance.md) nevertheless still listed *exception handlers* under
*Actions (Advanced)* and, under *Implementable But Not Yet Done*, as "spec exists, needs
exception propagation" — the last UML-referenced action item still presented as work to do. This
record asks whether that is so: whether SysML v2 textual notation spells an exception handler or
exception propagation at all, and if not, how a failure inside an action body is written and
handled in SysML v2 as it stands.

The answer is that **no such spelling exists**, in the language, in the library, in the corpora
or in the pinned reference implementation, and the item is **closed as not a SysML v2
construct**. A failure in SysML v2 is a modeled fact — a value, a signal, an ended performance —
handled by the ordinary action constructs; the record documents that idiom, runs it, and
enumerates the runtime surfaces that already cover the need, so that a later model needing
"exception handling" can be judged against it without re-deriving anything.

**Specifications cited.** SysML v2 Part 1 (formal/2026-03-02) and KerML 1.0
(formal/2026-03-01), the same documents the [compliance mapping](spec-compliance.md) cites;
library text is quoted from the bundled copies under `internal/workspace/libs/stdlib/`. Corpus
paths are those `scripts/download-training-examples.sh` and `scripts/download-pilot-corpora.sh`
create under `examples/`, and the reference implementation is the pilot
`scripts/download-pilot-sysml-validator.sh` builds, both at the `2026-08` pin of
`scripts/pilot-pin.sh`.

## The question

The UML construct has three parts, and a SysML v2 counterpart would need a spelling for each:

1. **Raising.** A statement that ends the enclosing performance abnormally with a payload
   (`RaiseExceptionAction`).
2. **Catching.** A body attached to a protected action, keyed by the payload's type, that runs
   when a matching exception ends the protected action (`ExceptionHandler`,
   `exceptionType`, `handlerBody`).
3. **Propagation.** The rule that an uncaught exception ends the enclosing performance in turn,
   outward, until a handler matches or the outermost performance ends.

The compliance bullet's "spec exists" presupposes at least one of these has a normative spelling.
Each is checked below against the notation clauses, the abstract syntax, the semantic library,
the corpora and the pilot.

## What the specifications say

### SysML v2 §7.17 — the action kinds are enumerated, and none raises or catches

Clause 7.17 *Actions* defines the notation for every kind of action usage SysML v2 has: action
definitions and usages (§7.17.2), control nodes — `fork`, `join`, `merge`, `decide` (§7.17.3),
successions and their guards, `perform` (§7.17.6), `send` (§7.17.7), `accept` (§7.17.8),
`assign`, `terminate` (§7.17.10), `if`/`else` (§7.17.11) and the `while`/`for` loops (§7.17.12).
The words *exception*, *handler*, *raise*, *throw*, *catch* and *fault* do not occur as keywords
or as concepts anywhere in the clause. No action kind carries a type-keyed body that runs in
place of another action, and no succession is described as taken "on failure".

The one construct that ends a performance from outside itself is `terminate` (§7.17.10): a
terminate action usage "forces the lifetime of the terminated occurrence to end", the
occurrence being the enclosing performance by default or the one an expression names. The
clause's own worked example, `MonitoredActivity`, is the closest SysML v2 comes to the *shape* of
an exception handler — a `fork` runs `performCriticalActivity` beside a monitoring branch whose
`waitForTimeOut` accepts a signal and then terminates the critical branch — and it is written
entirely with `fork`, `accept` and `terminate`. Nothing propagates: the terminated performance
ends, the branch that terminated it goes on along its own succession, and the enclosing action
continues.

### SysML v2 §8.3.17 and the reflective metamodel — no `ExceptionHandler` metaclass

Clause 8.3.17 *Actions Abstract Syntax* declares the metaclasses behind §7.17: `ActionDefinition`,
`ActionUsage`, `AcceptActionUsage`, `SendActionUsage`, `AssignmentActionUsage`,
`TerminateActionUsage`, `PerformActionUsage`, `IfActionUsage`, `LoopActionUsage`,
`WhileLoopActionUsage`, `ForLoopActionUsage`, `ControlNode`, `DecisionNode`, `ForkNode`,
`JoinNode`, `MergeNode`, `TriggerInvocationExpression`. The bundled *Systems Library* carries the
same list as a reflective model of the abstract syntax, `SysML.sysml` ("a reflective SysML
model of the SysML abstract syntax", 93 `metadata def`s), and its `Kernel Semantic Library`
counterpart `KerML.kerml` (83 metaclasses) does the same for KerML. Neither declares a metaclass
whose name contains *Exception*, *Handler*, *Raise*, *Throw* or *Fault*:

```text
$ rg -n -o 'metadata def [A-Za-z]*(Action|Node)[A-Za-z]* ' \
    'internal/workspace/libs/stdlib/Systems Library/SysML.sysml'
13:metadata def AcceptActionUsage      52:metadata def AssignmentActionUsage
19:metadata def ActionDefinition       123:metadata def ControlNode
23:metadata def ActionUsage            125:metadata def DecisionNode
196:metadata def ForLoopActionUsage    201:metadata def ForkNode
210:metadata def IfActionUsage         234:metadata def JoinNode
236:metadata def LoopActionUsage       242:metadata def MergeNode
274:metadata def PerformActionUsage    357:metadata def SendActionUsage
405:metadata def TerminateActionUsage  532:metadata def WhileLoopActionUsage
```

UML's `ExceptionHandler` is a metaclass of its own with `exceptionInput`, `exceptionType`,
`handlerBody` and `protectedNode`. SysML v2's abstract syntax has no counterpart, no
`ExceptionHandlerMembership`, and no feature on `ActionUsage` that could hold one.

### KerML §7.4.7, §8.3.18 and `Performances.kerml` — a performance ends; it does not fail

KerML gives every action its semantics as a `Performance` (§7.4.7 *Behaviors*, §8.3.18). The
Kernel Semantic Library's `Performances.kerml` describes `Performance` — "the most general class
of behavioral Occurrences that may be performed over time" — by its `performers`, its
`involvedObjects`, its `enclosedPerformances` and `subperformances`, and the `Evaluation`s it
comprises; the `Occurrences.kerml` it builds on gives every occurrence a lifetime with one start
and one end. There is no *outcome* of a performance beyond having ended, no failed or
exceptional end, and no relationship between a performance and a handler that follows it. A
search of the bundled library for the whole words `exception`, `handler`, `fault`, `raise`,
`throw` finds one line, in the license `NOTICE` file, and none in any `.kerml` or `.sysml`
library text.

`ControlPerformances.kerml` (`DecisionPerformance`, `MergePerformance`, `IfPerformance` and its
three forms, `LoopPerformance`) and `TransitionPerformances.kerml` define the semantic types
behind `decide`, `merge`, `if`, the loops and transitions; every one of them selects among or
sequences ordinary successions. `Transfers.kerml`, behind `send`/`accept`, moves a
payload from a source to a target. Where UML's exception carries a payload out of a
performance, KerML's carrier of a payload is a `Transfer` — which is why the idiom below spells
the failure as a signal.

### `Actions.sysml` — the base types match the notation, one for one

The *Systems Library*'s `Actions.sysml` declares the base type of every action usage kind:
`Action`, `SendAction`, `AcceptAction`, `AcceptMessageAction`, `AssignmentAction`,
`TerminateAction`, `IfThenAction`, `IfThenElseAction`, `LoopAction`, `WhileLoopAction`,
`ForLoopAction`, `ControlAction`, `DecisionAction`, `MergeAction`, `ForkAction`, `JoinAction`,
`TransitionAction` and `DecisionTransitionAction`. `TerminateAction` is documented as
"an Action that terminates a given Occurrence, meaning that the Occurrence ends during the
performance of this Action" and takes one `in occurrence terminatedOccurrence[1]`. There is no
`RaiseAction`, no `HandlerAction`, and no parameter on `Action` naming a handler.

## Corpus evidence

Across the OMG training corpus (100 files) and the three pilot corpora (213 files: 99
`sysml-examples`, 56 `sysml-validation`, 58 `kerml-examples`), a case-insensitive search for
`exception|handler|fault|raise|throw|failure` finds six lines, all in the language-extension
chapters, and all of them *modeling* failure as a concept rather than handling one:

```text
41. Language Extension/Semantic Metadata Example.sysml:13:  metadata def failure :> SemanticMetadata {
41. Language Extension/Model Library Example.sysml:15:      abstract occurrence def Failure {
41. Language Extension/Model Library Example.sysml:19:      abstract occurrence failures : Failure[*] nonunique :> situations;
41. Language Extension/User Keyword Example.sysml:28:       #failure 'device shutoff' {
14-Language Extensions/14c-Language Extensions.sysml:98:    metadata def <failure> FailureModeMetadata :> SituationMetadata {
14-Language Extensions/14c-Language Extensions.sysml:171:   #failure occurrence 'battery cannot be charged' {
```

A `Failure` here is an `occurrence def` — a situation a system can be in, tagged with semantic
metadata so that `#failure` becomes a user keyword. No corpus model writes a construct that
catches anything, and no corpus action body has a branch described as taken on an error. The
fUML referee's own corpus is the confirming negative: fUML's `fUML-Exception-Tests.uml` (twelve
activities) has no SysML v2 translation because there is nothing to translate them to.

## The pinned pilot implementation

The pilot's `SysML.ecore` (in the `2026-08` pin's `jupyter-sysml-kernel-0.62.0-all.jar`, entry
`model/SysML.ecore`) declares 175 `EClass`es and its `kerml.ecore` 82; none has a name matching
`Exception`, `Handler`, `Raise`, `Throw` or `Fault`, case-insensitively. This is the abstract
syntax the pilot parses to and validates against, so a model spelling an exception handler in any
notation would be rejected by the reference implementation as well. The pilot performs no
actions, so it offers no runtime reading of "propagation" to compare against.

## What OpenSysML runs today

The need behind an exception handler — *notice that a step failed, stop what it was doing, and
run something else instead* — is served in SysML v2 by constructs the runtime already executes
faithfully. Each is a compliance row of its own; this is the inventory the record relies on.

| Need | SysML v2 construct | Runtime surface | Evidence |
|---|---|---|---|
| Report a failure from a step | An `out` parameter (`ok : Boolean`, a status enum, a `Rational` result) assigned in the body | `runtime/action_frame.go` pin binding; `runtime/action_executor.go` assignment steps | conformance `action_decision_guarded_branch`, `action_decision_else_branch` |
| Branch on the report | `decide` with guards and `else` (§7.17.3), `if`/`else` (§7.17.11) | `runtime/action_executor.go` `stepDecisionNode` | conformance `action_decision_else_done`, `action_decision_merge_guarded_branch` |
| Signal a failure out of a step | `send new Fault(...) to <accepter>` (§7.17.7) | `runtime/action_executor.go` send steps, `runtime/signal.go` | conformance `action_accept_suspends_until_message`, `send_this_part_addressed` |
| Wait for a failure while work proceeds | `fork` beside the work, `accept fault : Fault` (§7.17.3, §7.17.8) | `runtime/action_executor.go` `acceptMatch`; `ErrAcceptDeadlock` when nothing can post the message | conformance `action_accept_two_waiters`, `action_terminate_names_waiting_accept` |
| Stop the failed work, keeping what it did | `terminate <node>;` from the handling branch (§7.17.10 `MonitoredActivity`); `terminate;` inside the step | `runtime/action_terminate.go` `terminateTargets`, `terminatedUsage`; `ErrTerminateTarget`, `ErrTerminateOccurrence` | conformance `action_terminate_fork_drops_sibling`, `action_terminate_names_sibling_flow`, `action_terminate_nested_body` |
| End the whole object on an unrecoverable failure | `terminate this;` (§7.17.10) | `runtime/action_terminate.go`; `%instances` shows `ended` | conformance `action_terminate_this_ends_part` |
| A failure the model did not anticipate | None to write: the run ends with a typed error naming the cause (`division by zero`, `multiplicity violation`, `unbound parameter`, `accept deadlock`, `step limit exceeded`) | `runtime/errors.go`; `ErrDivisionByZero` from `semantics/quantity_eval.go` | `TestRuntimeRobustness*` suites; `-check` reports the standing `not covered` with the error's text |
| Judge a failed run | A verification case whose body fails is verdict `error` carrying the error's own text | `runtime/verification_run.go` `Context.RunVerification` | conformance `verification_verdict_error`, `robustness_test.go:verification_body_that_cannot_run` |

The last two rows are the runtime's side of the contract: an error the model has no construct for
is never swallowed and never a panic; it is a typed error at the boundary (`execution failed:`
on the command line, a `not covered` standing, an `error` verdict), so a model that wants to
*handle* a failure must first *model* it, with the first six rows.

### The worked model

Two actions, one per idiom; both validate clean and run to completion under `bin/sysml`.

**By result.** The step reports its outcome on an `out` parameter; a `decide` routes on it. Where
UML would raise inside `attempt` and catch in an enclosing handler, SysML v2 keeps `attempt` as an
ordinary node whose result is data:

```sysml
package FailureHandling {
    private import ScalarValues::*;

    action def Attempt {
        in divisor : Integer;
        out ok : Boolean;
        out quotient : Rational;
    }

    action byResult {
        attribute divisor : Integer = 0;
        out attribute ok : Boolean = false;
        out attribute result : Rational = -1;

        first start;
        then action attempt : Attempt {
            in divisor = byResult::divisor;
            first start;
            then decide;
                if divisor == 0 then fail;
                else compute;
            action fail { assign ok := false; assign quotient := 0; }
            action compute { assign ok := true; assign quotient := 100 / divisor; }
            succession first fail then done;
            succession first compute then done;
        }
        then action report { assign ok := attempt.ok; }
        then decide route;
            if ok then keep;
            else recover;
        action keep { assign result := attempt.quotient; }
        action recover { assign result := 0; }
        succession first keep then done;
        succession first recover then done;
    }
}
```

```text
$ sysml -action FailureHandling::byResult failure_handling.sysml
✓ Action completed
  Final state: Completed
  Results:
    attempt.ok = false
    attempt.quotient = 0
    ok = false
    result = 0
```

With `divisor = 4` the same run takes `keep`: `ok = true`, `result = 25.0`.

**By signal.** The step signals its failure; a branch forked beside it accepts the signal,
terminates the step whatever it has reached, and handles the payload — §7.17.10's
`MonitoredActivity` with the signal coming from the work itself. This is the shape of a UML
handler, written with three ordinary nodes:

```sysml
attribute def Fault { attribute reason : String; }

action bySignal {
    out attribute progress : Integer = 0;
    out attribute handled : String = "";

    first start;
    then fork split;
        then work;
        then caught;

    action work {
        first start;
        then action step1 { assign progress := 1; }
        then action raise send new Fault(reason = "sensor offline") to caught;
        then action wait accept go : Integer;
        then action step2 { assign progress := 99; }
        then done;
    }

    action caught accept fault : Fault;
    then action stop { terminate work; }
    then action handle { assign handled := fault.reason; }
    then sync;

    join sync;
    succession first work then sync;
    succession first sync then done;
}
```

```text
$ sysml -action FailureHandling::bySignal failure_handling.sysml
✓ Action completed
  Final state: Completed
  Results:
    fault = Instance(ID: 1)
    handled = "sensor offline"
    progress = 1
```

`progress` stays at 1: `work` was parked at `wait` when `stop` terminated it, so `step2` never ran
and the join released on `work`'s ended performance. `-trace` records the termination
(`terminate action node work`). The handler is not keyed by a type on a protected node; it is a
node of the flow that accepts a typed signal, and the "protected region" is whatever `terminate`
names.

**Unhandled.** A failure the model does not spell is the third case, and needs no construct:

```sysml
action unhandled {
    attribute divisor : Integer = 0;
    out attribute result : Rational = -1;
    first start;
    then action compute { assign result := 100 / divisor; }
    then done;
}
```

```text
$ sysml -action Unhandled::unhandled unhandled.sysml
sysml: execution failed: eval assignment RHS: division by zero
  standing: not covered (execution failed: eval assignment RHS: division by zero)
```

The process exits nonzero with the typed error's text, never a stack trace; under a verification
case the same failure is the verdict `error`.

## The options

**(a) Implement UML exception handlers as an OpenSysML extension.** A `handle <Type> { … }` body
on an action usage, a `raise <expr>;` statement, propagation through enclosing performances in
`runtime/action_executor.go`. Rejected: the notation would be accepted by no other SysML v2 tool
and rejected by the pilot (no metaclass to serialize it to); it would need an `ActionGraph`
edge kind the lowered IR does not have and the specification does not define; and `-strict`
would have to refuse it, making it an extension no conforming model could use.

**(b) Keep the item open pending a specification revision.** Rejected: unlike protocol state
machines, where a runtime follow-up (reception ordering on a port) remained, nothing here is left
for the runtime to do — every row of the inventory is landed and gated, and the fUML referee's
classification already states the language boundary. An open item with no work under it misleads
the reader the compliance mapping is for.

**(c) Close as not a SysML v2 construct, documenting the idiom.** Chosen. The two idioms above
are standard notation (§7.17.3, §7.17.7, §7.17.8, §7.17.10), run today, and are what a SysML v2
modeler migrating a UML activity with handlers should write; the guide's behavior chapter gains a
short note so the question is answered where a modeler will look for it.

## Decision

**Closed under option (c) — exception handlers are not a SysML v2 construct; the idiom is
`decide` on a result, or `fork`/`accept`/`terminate` on a failure signal.**

SysML v2 1.0 spells no raise, no handler and no propagation: §7.17 enumerates the action kinds
and has none; §8.3.17 and the reflective `SysML.sysml` metamodel declare no `ExceptionHandler`;
KerML's `Performance` ends but does not fail; the library's `Actions.sysml` declares a base type
for every notation kind and none for a handler; no corpus model writes one; and the pinned
pilot's `SysML.ecore` (175 classes) has no such metaclass. The compliance mapping's "spec exists,
needs exception propagation" was mistaken on both counts — the spec that exists is UML's, and
propagation is a rule about a construct SysML v2 does not have.

Consequences:

- The compliance mapping's two entries are reworded to point here; the roadmap records the
  closure beside the other two design-record closures of Track E; the behavior guide's
  *Terminate* section gains a short *Handling a failure* note with the by-signal shape.
- No AST, IR or runtime change, and no diagnostic: a modeler who writes a UML-style `raise` or
  `handle` gets the parser's ordinary error for an unknown construct, as for any non-SysML
  spelling.
- A later SysML v2 revision that adds an action kind for exceptions (it would appear in §7.17,
  §8.3.17 and `Actions.sysml` together) reopens the item with a spelling to implement; until
  then, a model that needs handling writes one of the two idioms, and a model that needs an
  *unrecoverable* failure to be visible relies on the typed-error contract rather than on any
  construct.

## Sources

- SysML v2 Part 1, formal/2026-03-02: §7.17.1–7.17.12 (Actions; in particular §7.17.3 Control
  Nodes, §7.17.7 Send Actions, §7.17.8 Accept Actions, §7.17.10 Terminate Actions and its
  `MonitoredActivity` example), §8.3.17 Actions Abstract Syntax.
- KerML 1.0, formal/2026-03-01: §7.4.7 Behaviors, §8.3.18 Kernel Semantic Library —
  `Performances`, `ControlPerformances`, `Transfers`.
- Bundled library: `internal/workspace/libs/stdlib/Systems Library/Actions.sysml`
  (`TerminateAction`, the action base types), `Systems Library/SysML.sysml` (reflective
  metamodel), `Kernel Libraries/Kernel Semantic Library/Performances.kerml`, `KerML.kerml`,
  `Transfers.kerml`.
- Corpora: `examples/sysml-v2-training/41. Language Extension/*.sysml`,
  `examples/pilot-corpora/sysml-validation/14-Language Extensions/14c-Language Extensions.sysml`
  (`2026-08` pin).
- Pilot: `model/SysML.ecore` and `model/kerml.ecore` in `jupyter-sysml-kernel-0.62.0-all.jar`
  (`scripts/download-pilot-sysml-validator.sh`, `2026-08` pin).
- OpenSysML: [fuml-referee.md](fuml-referee.md) (exception model `not-expressible`);
  [spec-compliance.md](spec-compliance.md) rows for terminate, accept, decision nodes,
  verification verdicts and runtime robustness; [roadmap.md](roadmap.md#track-e--behavior-execution);
  the precedent records [expansion-regions.md](expansion-regions.md) and
  [protocol-state-machines.md](protocol-state-machines.md).
