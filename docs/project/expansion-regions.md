# Concurrent per-element performance ("expansion regions"): adjudicated and closed

UML 2.5.1 activities have an *expansion region*: a structured node performed once per element
of an input collection, in `parallel`, `iterative` or `stream` mode, its per-element outputs
gathered back into a collection. SysML v2 has no such notation. The
[roadmap](roadmap.md#e3--concurrent-per-element-performance-expansion-regions-closed) records the
iterative half as the `for` loop, which the runtime executes, and asks one question about the
parallel half before any executor is built for it: whether SysML v2 §7.17.2's multiplicity on a
performed action usage, with a `flow` delivering the collection to its input, is the standard
spelling of "perform the body once per element, all performances ongoing at once" — and if so,
what `Performances.kerml` says about the order of those performances. If no such spelling exists,
the item closes as *the iterative form is `for`; the parallel form is not SysML v2*.

This record settles it against the pinned specifications, the bundled Kernel Semantic Library, the
OMG training and pilot corpora, the pinned pilot implementation and the runtime as it stands. The
answer is that **no standard spelling exists**, and the item is **closed** as the roadmap
foresaw. The record states the reading so a later model that needs per-element concurrency can
be judged against it without re-deriving anything.

**Specifications cited.** SysML v2 Part 1 (formal/2026-03-02) and KerML 1.0
(formal/2026-03-01), the same documents the [compliance mapping](spec-compliance.md) cites;
library text is quoted from the bundled copies under `internal/workspace/libs/stdlib/`. Corpus
paths are those `scripts/download-training-examples.sh` and `scripts/download-pilot-corpora.sh`
create under `examples/`, at the `2026-08` pin of `scripts/pilot-pin.sh`.

## The candidate spelling

The construct under adjudication is an action usage with an unbounded multiplicity, its input fed
by a flow from a collection-valued output:

```sysml
action outer {
    first start;
    then action src { out items : Integer[*]; assign items := (1, 2, 3); }
    then action dbl : Double[*];
    flow src.items to dbl.x;
    then action fin { assign total := dbl.y->size(); }
    then done;
}
```

The reading the roadmap asks about is: `dbl[*]` denotes as many performances of `Double` as
`src.items` has elements, each performed with one element at its `x`, all ongoing at once, and
`dbl.y` afterwards holds the three results. For that to be the *standard* spelling, the
specification or the library would have to fix (a) that the number of performances is the number
of elements, (b) that each performance receives exactly one distinct element, and (c) when they
happen relative to each other. None of the three is stated anywhere, and the pieces the texts do
state each mean something else.

## What the specifications say

### SysML v2 §7.17.2 — multiplicity is not mentioned in connection with parameters

§7.17.2 (*Action Definitions and Usages*) is the clause the roadmap names. Its whole statement
about connecting subactions is:

> Binding connection usages (see 7.13.3) and flow usages (see 7.16) can be used to connect
> subactions in the body of an action definition or usage. In addition, the feature value
> shorthand for binding (see 7.13.4) is often useful for action parameters.

Its example is `TakePicture`, with `focus : Focus` and `shoot : Shoot` each singular and `flow
focus.image to shoot.image;` between them. The clause says nothing about the multiplicity of an
action usage, and nothing in §7.17 gives a multiplicity on an action usage any meaning beyond the
one every usage has. The word *expansion* does not occur in either specification.

### SysML v2 §7.17.1 and §8.4.13.2 — an action usage is a KerML step

> The features of an action definition or usage that are themselves action usages specify the
> performance of the action in terms of the performances of each of the subactions. (§7.17.1)

> An `ActionUsage` is a kind of `OccurrenceUsage` and a kind of KerML `Step`. As such, all the
> general semantic constraints for an `OccurrenceUsage` (see 8.4.5) and a `Step` (see [KerML,
> 8.4.4.7.2]) also apply to an `ActionUsage`. (§8.4.13.2)

So the meaning of `action dbl : Double[*]` is the meaning KerML gives a step with multiplicity
`[*]`: a feature of the enclosing performance whose values are performances of `Double`, any
number of them. The multiplicity bounds *how many values the feature has*; it is not an operator
over another feature's values.

### SysML v2 §7.17.6 and §8.4.13.11 — a perform action usage refers to performances

> A perform action usage is an action usage that specifies that an action is performed by the
> owner of the performed action usage. (§7.17.6)

> Thus, the values of `p.perf` will be some subset of the `Actions` represented by `act` that
> are performed by `p`. (§8.4.13.11)

`perform action takePhoto[*] ordered references takePicture;` therefore says that the part
performs `takePicture` some number of times over its lifetime, in an order; it does not say how
many times, and it relates the count to no collection.

### SysML v2 §7.17.3 — `fork` duplicates control, not a collection

> // Both action1 and action2 will proceed concurrently after fork1. … // join1 will be performed
> after both action1 and action2 have completed. (§7.17.3, comments of the normative example)

The concurrency SysML v2 spells is between *distinct, declared* action usages downstream of a
fork, each with a target multiplicity of one on its succession (§7.17.3, rule 4 for forks). A
fork does not create performances from a collection's elements; the runtime already executes this
form (`runtime/action_executor.go` `stepForkNode`, `stepJoinNode`).

### SysML v2 §7.17.12 — `for` is the iterative form, and it is sequential

> The behavior of the for loop action usage is to first evaluate the sequence expression, which
> should result in a sequence of values. The body clause is then performed iteratively, with the
> loop variable assigned to each value sequentially for each iteration. (§7.17.12)

The library agrees (`Systems Library/Actions.sysml`):

```sysml
action def ForLoopAction :> LoopAction {
    doc
    /*
     * A ForLoopAction is a LoopAction that iterates over an ordered sequence of values.
     */
    protected ref var[0..1] :> seq;
    in ref seq;
    in action body;
    private attribute index : Positive;
    ...
}
```

`ControlPerformances.kerml`'s `LoopPerformance`, which it specializes, describes its `body` as
occurring repeatedly in sequence. The iterative half of an expansion region is this construct, by
the specification's own words, and it is sequential by construction: one `var`, one `index`.

### SysML v2 §7.16 — a flow transfers a payload; it does not distribute one

A `flow` is a `Flows::Flow :> Message, FlowTransfer` (`Systems Library/Flows.sysml`), and a
`Transfer` carries its whole payload at once (`Kernel Semantic Library/Transfers.kerml`):

```kerml
feature payload: Anything[1..*] { ... }
feature payloadNum: Natural [1] = size(payload);
connector targetInputLink: BinaryLinkObject[payloadNum] {
    end [1] feature transferTarget references target;
    end [payloadNum] feature transferPayload references payload subsets transferTarget.targetInput;
}
```

One transfer has one `target` occurrence and one to many payload values, every one of them
delivered to that target's input. A flow from `src.items` to `dbl.x` is a connector between the
`src` performance and the `dbl` performances; each of its links is one transfer to one `dbl`
performance, and nothing says how many links there are or which elements of `items` each carries.
The collection reaching one performance whole, three performances each receiving all three, or
three each receiving one are all consistent with the text. A spelling whose element-per-performance
reading is one of several the specification admits is not a standard spelling of that reading.

### KerML §7.4.7 — declared steps carry no order; multiplicity counts performances

> Though the performance of a behavior takes place over time, the order in which its steps are
> declared has no implication for temporal ordering of the performance of those steps. Any
> restriction on temporal order, or any other connections between the steps, must be modeled
> explicitly. (§7.4.7)

> `composite step focus[*] : Focus;` `composite step shoot[1] : Shoot;` (§7.4.7.3, example)

Annex A.3.6 (*Timing for behaviors, Sequences*) is the one place the specification works through
a step with multiplicity `[*]`, and it shows what the multiplicity is for:

> Only the first one (`paint`) is required (by its multiplicity), indicating the behavior starts
> there, while the rest (`dry` and `ship`) are unrestricted, to prevent the instantiation
> procedure from giving values to (performing) them too early. This is left to the end
> multiplicities of the timing connectors (`p_before_d` and `d_before_s`), which require their
> later step to happen once each time the earlier step does, and vice-versa. (A.3.6)

```kerml
behavior Manufacture {
    step paint : Paint [1];
    step dry : Dry [*];
    succession p_before_d first [1] paint then [1] dry;
    step ship : Ship [*];
    succession d_before_s first [1] dry then [1] ship;
}
```

The `[*]` on `dry` and `ship` means *as many performances as the connectors require* — here
exactly one each, fixed by the `[1]` end multiplicities of the successions. The count of a
`[*]` step's performances comes from the connectors attached to it, never from the elements of a
value that flows into one of its parameters. The same corpus file
(`examples/pilot-corpora/kerml-examples/KerML Spec Annex A Examples/A-3-6-Sequences.kerml`)
carries this example unchanged.

### `Performances.kerml` — collections of performances, no order among them

The library declares the collections the roadmap asks about:

```kerml
abstract step performances: Performance[0..*] nonunique subsets occurrences
step enclosedPerformances: Performance[0..*] subsets performances, timeEnclosedOccurrences
composite step subperformances: Performance[0..*] subsets enclosedPerformances, suboccurrences
```

and `Actions.sysml` specializes them: `abstract action actions: Action[0..*] nonunique :>
performances`, `action subactions: Action[0..*] :> actions, subperformances`. Every one of these
is `[0..*]`, unordered; `subperformances` are time-enclosed by their owner and nothing more. There
is no `HappensBefore` among the values of a multi-valued step, and none between the values of a
`[*]` action usage: the library is silent on the order of several performances of one step
because, as §7.4.7 says, that order is modeled explicitly with successions or not at all. Had a
spelling existed, "ordering per `Performances.kerml`" would have been *none* — the performances
are unordered occurrences enclosed in their owner's lifetime — and that is also the answer the
runtime's existing fork/join concurrency already implements for distinct nodes.

## Corpus evidence

Fifty-three non-comment action or step declarations with a multiplicity other than `[1]` occur
across the four corpora (100 training files, 99 pilot SysML examples, 56 pilot validation files,
58 KerML examples). None is fed a collection by a `flow` or a `bind`; every one is one of two
other things.

**A count of performances over a lifetime.** The training corpus's *Action Performance* lesson
(`examples/sysml-v2-training/18. Action Performance/Action Performance Example.sysml`):

```sysml
part camera : Camera {
    perform action takePhoto[*] ordered
        references takePicture;
    part f : AutoFocus { perform takePhoto.focus; }
    part i : Imager { perform takePhoto.shoot; }
}
```

and its pilot counterpart (`examples/pilot-corpora/sysml-examples/Camera Example/Camera.sysml`,
`perform action takePicture[*] :> PictureTaking::takePicture;`). The camera takes pictures
any number of times; inside `takePicture` the subactions are `focus: Focus[1]` and
`shoot: Shoot[1]` with a scalar `flow of Exposure from focus.xrsl to shoot.xsf;`
(`PictureTaking.sysml`). The KerML examples say the same with steps: `takePics : TakePicture
[0..*]` (`Variable Feature Examples/Enhancements/TimeVaryingSteps.kerml`), `step dry : Dry [*]`
(the Annex A.3.6 file above).

**A collection processed by iteration.** Where a corpus model applies an action to each element
of a collection, it writes a `for`
(`examples/sysml-v2-training/20. Assignment Actions/Assignment Example.sysml`):

```sysml
for vehiclePower in powerProfile {
    perform action dynamics : StraightLineDynamics {
        in power = vehiclePower;
    }
}
```

The collection-valued inputs the corpora do write —
`Analysis Examples/Vehicle Analysis Demo.sysml`, `Individuals
Examples/AnalysisIndividualExample.sysml`, `33. Analysis/Analysis Case Definition
Example.sysml`, `10-Analysis and Trades/10d-Dynamics Analysis.sysml` — are supplied by direct
assignment, a default, indexing or iteration, to a parameter whose own multiplicity holds the
collection. Where the vehicle model wants "concurrent actions" (`Vehicle Example/SysML v2 Spec
Annex A SimpleVehicleModel.sysml`, `TransportPassengerScenario`), it declares distinct action
usages — `driverGetInVehicle`, `passenger1GetInVehicle`, each `[1]` — nested in one action with no
successions between them, "vs fork and join", as its own comment says: unordered by KerML §7.4.7,
never replicated. The single file named *Expansion*
(`Simple Tests/Expansion.kerml`) is a `->select` body-expression test and has nothing to do with
actions.

No published OMG model spells per-element concurrent performance with a multiplicity and a flow.
Where the training material wants a body per element, it teaches `for`.

## The pinned pilot implementation

The pinned pilot (`jupyter-sysml-kernel` 0.62.0, release `2026-08`) evaluates expressions and
does not perform actions; the [pilot execution referee](pilot-execution-referee.md) records that
surface exactly, and no action, state, transition or token trace is adjudicable against it. Its
validator accepts `perform action takePicture[*]` — the Camera example above is in its own
corpus and parses clean in this repository's [pilot corpora gate](pilot-corpora.md) — as it
accepts any usage with a multiplicity. It has no execution behavior to observe for the construct,
so the pilot neither supports nor contradicts the per-element reading; it is silent.

## What the runtime does today

Run against the candidate model at the top of this record (`sysml -trace -action <package>::outer`), the
runtime performs `dbl` **once**: the token reaches `dbl`, the flow delivers all three values of
`src.items` to that one performance's `x : Integer`, and the write is refused —
`multiplicity violation: 3 value(s) bound to a feature with multiplicity upper bound 1` — as an
`ErrMultiplicityViolation` from `runtime/write_conformance.go` `checkTargetAs`, the same
check every write of a declared feature passes through. The `[*]` on the node
is carried by lowering as the usage's declared multiplicity and is not consulted by the action
executor, which creates one performance per token arrival as it does for every node (the
compliance mapping's *one feature space per performance* row). The same collection processed by
a `for` around a node — `for i in items { action dbl : Double { in x = i; } assign total :=
total + dbl.y; }` — completes with `total = 12`, one performance per iteration, the latest
standing for the node in `Results()`.

That is a correct outcome under the reading above: the model states three values for a
one-valued input of one performance, and the runtime says so. It is not a refusal of the
construct — `dbl[*]` with a flow carrying exactly one value runs as any node does — and the
executor has no rule that reads the multiplicity as a replication count. Nothing here is a
known failure; the row in the compliance mapping stays a scope statement, not a `🚧`.

## The options

**(a) Adopt the per-element reading as this implementation's semantics.** Lower a node whose
declared multiplicity admits several values and whose input pin is fed by a flow from a
multi-valued source into one performance per element, each with its own token and pins, joined
when all complete; order them as `Performances.kerml` orders them — not at all — and record the
interleaving under the scheduling policy; leave the streaming interaction to the
[streaming-flows item](roadmap.md#e4--streaming-flows-streaming-pins). This is the design the
roadmap sketched *if a spelling exists*. It would be an extension: the specification admits the
reading among others and mandates none, so a conformant model written for another tool could
mean one performance receiving the whole collection, and a model relying on this reading would
be portable to no other implementation. It would also invent the two rules the texts lack —
that the count is the element count and that each performance gets one distinct element — and
decide a third (what a second flow into the same node, or a `[2..5]` multiplicity, or an empty
collection means) with nothing to check the decisions against: no specification clause, no
library declaration, no corpus model, no pilot behavior.

**(b) Refuse the construct.** Report a node with a multi-valued multiplicity fed by a
multi-valued flow as not executable, on the ground that its performance count is
under-determined. This over-refuses: `dbl[*]` fed one value at a time is well-defined and runs,
and `perform action takePhoto[*]` on a part is the corpus's own idiom for "any number of times".

**(c) Close the item: the iterative form is `for`; the parallel form is not SysML v2.** Leave
the executor as it is — one performance per token, the multiplicity a declaration about the
feature and not a replication count, a collection at a one-valued pin the multiplicity violation
it is — and reword the compliance mapping's bullet from "not implemented" to the statement that
SysML v2 has no expansion region: its iterative form is the `for` loop, which runs, and its
parallel form has no standard spelling. A model that needs per-element concurrency today writes
the elements as distinct nodes under a `fork`, or accepts the sequential `for`.

## Decision

**Closed under option (c) — the iterative form is `for`; the parallel form is not SysML v2.**
Every source the roadmap named was consulted and every one points the same way: §7.17.2 says
nothing about multiplicity; §7.17.1, §8.4.13.2 and KerML §7.4.7 with Annex A.3.6 make an action
usage's multiplicity a count of performances fixed by the connectors attached to it, not by the
elements of a value flowing in; §7.16 and `Transfers.kerml` deliver a whole payload to one
target and partition nothing; `Performances.kerml` and `Actions.sysml` declare the collections
of performances unordered and say nothing about replication; the corpora write `[*]` for "any
number of times over a lifetime" and `for` for "once per element"; the pilot performs no actions.
Adopting the reading would be an OpenSysML extension with nothing to adjudicate its rules
against, so it is not adopted.

**Consequences.**

- No executor change. The IR shape, per-element performance, join-on-completion, streaming
  interaction and ordering the roadmap listed *if a spelling exists* are not designed, because
  the condition is not met; the proof fixtures it named (three elements performed concurrently
  with outputs collected, the interleaving golden, the empty collection, one failing element)
  are not written, since there is no rule for them to prove.
- The [compliance mapping](spec-compliance.md#major-features-not-implemented-uml-referenced-no-sysml-v2-notation-or-kerml-performance)'s
  expansion-regions bullet states the closure and links here; the
  [roadmap item](roadmap.md#e3--concurrent-per-element-performance-expansion-regions-closed) is
  recorded as closed by this record.
- The reading stands as the reference for any later request: a model whose sequential `for`
  result is wrong or too slow is the trigger the roadmap named, and a session answering it
  starts from option (a) above as an *explicitly non-standard* extension, under a notation the
  compliance mapping would have to flag as such — or from a change to the specification. Should
  a later SysML v2 revision give a multiplicity on an action usage a per-element meaning, this
  record is re-adjudicated against that text, not reopened on its own.
- The runtime's outcome for the candidate model — one performance, the multiplicity violation
  at the one-valued pin — is the correct one and is what the compliance mapping's *one feature
  space per performance* row describes; it needs no new fixture.
