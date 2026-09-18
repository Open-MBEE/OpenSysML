# SysML v1 to v2 migration

## Status: experimental

The migration is **experimental**. It covers the structural, requirement, constraint, instance,
allocation and behavioral content listed under [Mapping](#mapping), reports every element
it approximates or leaves behind, and refuses input it cannot read; units are not migrated
yet, and what a v1 element is written as may change between releases
without a compatibility path. Every run says so: `sysml -convert` prints `note:` to stderr,
`ConvertResponse` carries `experimental` and `experimental_notice` (the Python client raises
an `ExperimentalFeatureWarning`), and the wording lives once, in `export.MigrationNotice`.

`sysml Model.xmi -convert sysml` reads a SysML v1 model as UML XMI — the open OMG interchange
format every v1 tool exports — and writes it as SysML v2 textual notation. `-convert ttl` writes the same model as
RDF, through [the RDF mapping](rdf-mapping.md). Every run also produces a **migration report**
that accounts for every v1 element: what it became, or why it did not.

```bash
sysml Model.xmi   -convert sysml -o Model.sysml -migration-report Model.report.txt
sysml Model.uml   -convert ttl   -o Model.ttl   -migration-report Model.report.json
sysml Model.mdzip -convert sysml -o Model.sysml
sysml export.xml  -convert sysml -from xmi
```

The input format is inferred from the `.xmi`, `.uml` and `.mdzip` extensions; any other name
needs `-from xmi` (`uml` and `mdzip` are accepted as synonyms). XMI is read only — `-convert xmi` is refused,
since v2 has no v1 form — and it is never loaded into the REPL, an `-eval` or a check directly:
migrate first, then work with the notation.

The same conversion is available over gRPC (`Convert` with `from_format: "xmi"`, or a
`file_path` ending in `.xmi`/`.uml`/`.mdzip`) and so from every client library; the report is not
returned over the service yet.

## Input

- **[OMG XMI 2.5.1](https://www.omg.org/spec/XMI/2.5.1) with
  [UML 2.5.x](https://www.omg.org/spec/UML/2.5.1)**, with the
  [OMG SysML 1.x profile](https://www.omg.org/spec/SysML/1.6)'s stereotypes applied
  (`sysml:Block`, `sysml:Requirement`, … as `base_Class`/`base_Property` applications). This is
  the interchange export every SysML v1 tool offers, and a tool's export is read as far as it
  is this standard. The UML and SysML namespace versions are not pinned; any
  `http://www.omg.org/spec/UML/…` / `…/SysML/…` namespace is recognized.
- **Zip archives holding the XMI**, a MagicDraw / Cameo `.mdzip` project among them: the
  archive's `uml_model.model` entries are read, or, in an archive without them, every `.xmi`,
  `.xml` and `.uml` entry that is an XMI document. Used projects (archives the model refers
  to) are not read; elements they hold appear as external proxies and are reported as
  unmapped where they are relationship ends.
- **Eclipse UML2 `.uml` files** as [Papyrus](https://eclipse.dev/papyrus/) and its SysML 1.6
  component write them: a bare `uml:Model` root in the `http://www.eclipse.org/uml2/…/UML`
  namespace, with the SysML profile in `http://www.eclipse.org/papyrus/sysml/1.6/SysML/…`.
  Elements referenced by `href` into another resource are not read; they appear as external
  proxies and are reported as unmapped where they are relationship ends.
- `xmi:Extension` elements — diagrams, layout, tool-internal state — are skipped; the
  report says so once per skipped profile or library package. A package is library content
  when it is a profile, is marked «ModelLibrary» or «auxiliaryResource», or is a document root
  beside the user's Model or package bearing a standard library name; a user package named
  `SysML` or `Libraries` inside the model, or standing alone as the document's only root, is
  migrated like any other.
- Only stereotypes from the OMG SysML and UML standard profiles, in the OMG namespaces or
  Papyrus' `…/papyrus/sysml/…` ones, classify elements; any other profile's «Block» or
  «Requirement» — a user's own, a tool's customization layer over SysML, or another profile
  Papyrus hosts — is preserved as an applied-stereotype comment like any other.
- A requirement's `id` and `text` tags are read in the profile's spelling and in the
  capitalized `Id`/`Text` some exporters write.
- Multiplicity follows UML's defaults: an omitted bound is 1, and a bound element without a
  `value` is 0.
- A type referenced by `href` is a `ScalarValues` type only when the href points into the UML
  or SysML primitive libraries — by a plain fragment (`PrimitiveTypes.xmi#Real`), a dotted one
  (`SysML.xmi#SysML_dataType.Real`), or an opaque id whose qualified name the tool records
  beside it (MagicDraw's `referentPath`) under a standard library root, out of a module named
  for that library (`UML_Standard_Profile.mdzip`). A tool library's own
  machine-level datatypes (`float`, `double`, `int`, `long`, `short`, `byte`, `boolean`) are
  written as `Real`/`Integer`/`Boolean` and reported as approximations; a used project's own
  `Real` stays external, even when the project's top package borrows a library's name.
- A constraint block's properties are its parameters, `in attribute`s when typed by a value
  type and `in ref part` / `in ref item` when typed by a block or signal, whether the tool
  stores them as UML Properties or (as MagicDraw does, under a «ConstraintParameter» marker) as
  UML Ports; a `private` or `protected` parameter is written public, since the owning block's
  binding connectors reach it from outside, and the report notes the dropped visibility.

## Mapping

| SysML v1 | SysML v2 | Verdict |
|---|---|---|
| Model, Package | `package` | mapped |
| «Block», plain Class, Actor | `part def` | mapped (Actor and plain Class: approximated) |
| «InterfaceBlock» | `port def` | mapped |
| «ValueType» DataType, PrimitiveType | `attribute def` (`Real`/`Integer`/`Boolean`/`String` for the SysML primitives) | mapped |
| Signal | `item def`; properties typed by it are `item` / `ref item` | mapped |
| Enumeration and its literals | `enum def` | mapped |
| «ConstraintBlock» | `constraint def` with its parameters | mapped |
| «Requirement», «AbstractRequirement» | `requirement def <id>` with `doc` holding the text; any tool-specific requirement kind applied beside it as a comment | mapped |
| «TestCase» | `verification def` | approximated: its behavior is not migrated |
| InstanceSpecification of a block, of a constraint block | `individual part def`, `individual constraint def`, with its slots | mapped |
| InstanceSpecification of an interface block | `individual def` (v2 has no individual port def); an instance classified by an interface block beside a block is an `individual part def` of the block alone | mapped |
| Slot of a value property | `attribute :>> x = value;` | mapped |
| Slot of a part, item or constraint property holding one instance | `individual part :>> x : 'the instance';` — `ref` when the property is | mapped |
| Slot of a part, item or constraint property holding several instances | `part :>> x [n];` then one `individual part : 'the instance' :> x;` each | mapped |
| InstanceSpecification of a value type | `attribute` typed by it, holding its slot values (an individual cannot specialize an attribute def) | mapped |
| Slot contradicting its feature (more values than the multiplicity allows, a repeated value of a unique feature, a feature of a classifier the instance is not written to specialize, an instance that is not of the property's type or of its default individual, a value outside the document) | comment | **unmapped** |
| Slot of a port, or of an untyped property | comment (no individual can type a port; a `ref` without a type takes none) | **unmapped** |
| Property whose default is an InstanceSpecification of a block | the individual added to the usage's types, or its only type when the property is untyped; no `default` (a definition is not a v2 value). A port, a usage of another kind than the individual, or a usage whose type the individual is not an instance of, keeps its types and the default is a comment | approximated |
| Literal default on a value type with no scalar base (a structured value type, an enumeration) | comment | approximated |
| Real literal on an `Integer`/`Natural` feature, numeric string on a scalar feature | converted to the feature's scalar | mapped |
| Association with a name, «AssociationBlock» | `connection def` | mapped |
| Anonymous association with a classifier-owned end | nothing: the end property carries it | mapped |
| Anonymous association owning every end | a named `connection def` | approximated |
| Value property | `attribute`, with multiplicity and default | mapped |
| Composite part property | `part` | mapped |
| Reference property (no aggregation) | `ref part` | mapped |
| Undirected part or item property of an «InterfaceBlock» | `ref part` / `ref item`; a port owns no composite parts | approximated |
| Shared aggregation | `ref part` | approximated |
| «ConstraintProperty» | `constraint` usage | mapped |
| Redefinition | `:>>` | mapped |
| Generalization | `:>` | mapped |
| Port, «ProxyPort», «FullPort» typed by an InterfaceBlock / Block | `port`, `~` when conjugated | mapped |
| «FlowPort» typed by a value type | `port` holding one `in`/`out`/`inout` attribute | approximated |
| «FlowProperty» | directed `attribute`/`item` in the port def | mapped |
| `private` feature reached from outside by a connector, a slot, a redefinition, a subset or an expression | visibility dropped so the reference resolves; the report names the reacher — a connector or slot that is itself left as a comment reaches nothing | approximated |
| `private`, `package` or `protected` packaged element (a block, value type, enumeration…) | visibility dropped: a v2 private member is out of reach of every other package, which v1 tools do not enforce, so an import brings it in | approximated |
| Property or port sharing the name of an inherited feature without redefining it | `:>>` the inherited feature when both are the same kind of usage; otherwise the collision is reported and left | approximated |
| Connector, nested ends | `connect a.b to c.d` | mapped |
| «BindingConnector» | `bind`, or `binding name bind` when named | mapped |
| InformationFlow / «ItemFlow» over a connector | `flow of Item from a.x to b.y` | mapped |
| «Satisfy» | `satisfy requirement … by …` in the satisfying usage's owner | mapped |
| «Verify» from a test case | `verify` in the verification def | mapped |
| «DeriveReqt» | `connection … :> RequirementDerivation::Derivation` | mapped |
| «Allocate» | `allocate a to b`, or `allocation name allocate a to b` when named | mapped |
| «Refine» | `dependency` carrying `@ModelingMetadata::Refinement` | mapped |
| «Trace», «Copy», other stereotyped dependencies | plain `dependency` with the stereotype as a comment; named relationships keep their name | approximated |
| Comment, Documentation | `doc` (first) / `comment`, HTML tags stripped | mapped |
| Custom-profile stereotypes and tags | preserved as `/* applied stereotype «Name»: tag = value */` | mapped |
| SysML stereotype tags without a v2 form (`Block.isEncapsulated`, `ValueType.unit`, …) | preserved as `/* «Name» tags with no v2 form: tag = value */` | approximated |
| Two members of one namespace with the same name (UML allows it, v2 does not) | the later one renamed `Name 2`; a connection end named like a member of its connection def renamed `name2` | approximated |
| Anonymous property with no v2 type | a `ref` named after its type, or `unnamed` | approximated |
| Multiplicity bounds that are not natural numbers (a tool's `492x21` array dimensions) | omitted | approximated |
| `NaN`/infinite real literals | comment | approximated |
| References to ids the document does not define | the resolvable ends are written; the missing ids are named in the report | approximated |
| OpaqueExpression defaults and constraints | copied verbatim when it parses as a v2 expression and every name it uses is a written element visible where it is written (a parameter, an inherited feature, an enclosing member); a JavaScript or English body is translated through the [opaque-language subset](#the-opaque-language-subset) when every name resolves the same way (`V = R * i` → `V == R * i`, `java.util.Collections.max(s)` → `RealFunctions::max(s)`); a body outside the subset stays a `comment` and the report names the offending token | mapped / approximated |
| Activity | `action def` (see [Behaviors](#behaviors)); a block's `classifierBehavior` is also performed by a `perform action` usage of the `part def` | mapped |
| Parameter, ActivityParameterNode | `in`/`out`/`inout` parameter of the `action def`; a `return` parameter is `out`; the parameter node's flows bind the parameter | mapped (return: approximated) |
| InitialNode, ActivityFinalNode, FlowFinalNode | `first start then …`; `action x terminate;`; the token ends where a flow final does | mapped |
| ForkNode, JoinNode, DecisionNode, MergeNode | `fork`, `join`, `decide`, `merge`; a node several edges leave or reach without a control node gets one written for it | mapped (implicit fork/join: approximated) |
| CallBehaviorAction | `action x : Def;` with `bind`/`flow` for its pins; a call of no behavior whose only content is a duration is a leaf step, the wait written for it; a call of a state machine, of a behavior with no v2 declaration, or of no behavior with pins to feed | mapped (a leaf step: mapped, "a step with a duration and no further behavior") / **unmapped** |
| CallOperationAction | `action x : Owner::Op;`, or `perform action x ::> target.op;` when the target pin's value is an object whose type owns the operation | mapped |
| ControlFlow | `first a then b;`, `if <guard>` when the guard parses and resolves as a v2 expression or translates from JavaScript or English (`i >= Retries`, `GS_Found`, `not Found and i < 3`, `TRUE`) through the [subset](#the-opaque-language-subset); otherwise the guard text as a comment and the edge unguarded, the report naming the token refused | mapped / approximated |
| «Probability» on the edges out of a decision | `first d then x { @Stochastic::Probability { p = <value>; } }` when every edge carries one; weights not summing to 1 are scaled by their sum; a value outside `[0, 1]`, or a decision only some of whose edges carry one, is written unweighted | mapped / approximated |
| ObjectFlow | `flow a.out to b.in;`, or `bind` to a parameter; each producer-pin pair is written once however many edges carry it; a flow from or to an action that is not migrated, or from an output pin a translated opaque body never assigns, is a comment | mapped / approximated |
| SendSignalAction | `action x send new Sig(args) to <target>;`, `via <port>` when `onPort` is set; the target is read from the target pin's flow: `this`, `this.part` where a structural read feeds the pin, else the pin itself (`in target;` bound to what feeds it, an activity parameter or another node's output), which the runtime evaluates to the object it holds | mapped / approximated |
| AcceptEventAction | `action x accept p : Sig;` (signal trigger), `accept after <d> [SI::s]` (relative TimeEvent), `accept when <cond>` (ChangeEvent) | mapped |
| AcceptEventAction on an absolute TimeEvent (`when` is an instant, not a duration) | comment | **unmapped** — no literal writes a `TimeInstantValue` |
| OpaqueAction, ValueSpecificationAction, ReadStructuralFeatureAction, AddStructuralFeatureValueAction | `assign`/`out result = …` when the body parses as a v2 expression whose names resolve, or is a JavaScript body of the [subset](#the-opaque-language-subset): `i = 1; GS_Found = true;` is a sequence of `assign` statements, `i += 1` an assignment of `i + 1`, `var t = 0` a local `attribute`; names resolve against the swimlane's represented object first, then the activity, then the owning block; otherwise the body as a comment inside `action x { }` naming the language and the token refused | mapped / approximated |
| DurationConstraint on an action | a wait before the action: `accept after lo [SI::s]` when the interval is a point, `accept after RandomFunctions::uniform(lo, hi) [SI::s]` otherwise; `1s`, `0.5 s`, `80ms`, `2 min`, `1 h` and `t = 1 minute 30 seconds` literals are scaled to seconds; a symbolic bound (`ditSetup s`, `setup * 2 min`) is an expression whose names resolve like an action body's, `accept after this.tcs.ditSetup [SI::s]` | approximated (a tool's min/max/random mode is a run setting) |
| DurationConstraint whose bounds name nothing the activity can read | comment | **unmapped** — the note names the unresolved name |
| DurationObservation whose events are two nodes of one activity | an `attribute <name> : Real` of the `action def`, stamped with `localClock.currentTime` when the first node starts and assigned the elapsed clock when the second ends (`assign this.T := localClock.currentTime - this.T;`); one node observed is its own duration; the attribute is one a run can `-observe` | mapped |
| DurationObservation whose events are not nodes of the activity, or none; a DurationObservation or TimeObservation owned outside an activity; TimeObservation | comment | **unmapped** — the note names the events, or the owner |
| ActivityPartition | comment naming the partition, what it `represents` and its nodes; a name a body or guard in the partition uses is resolved against the represented property first and written through it, `this.tcs.i` for a partition representing the part `tcs` (a nested partition through its enclosing ones, `this.tank.valve.open`; a partition representing the context block itself, `this.x`) | mapped when the partition resolved a name / approximated when nothing in it needed one, when `represents` is unset, names nothing the document defines, a property of no v2 type, or a classifier the activity does not run in |
| StructuredActivityNode, SequenceNode | `action x { }` holding the nested flow | mapped |
| ExpansionRegion, LoopNode, ConditionalNode | `action x { }` holding the body's flow once; the expansion, the loop test and the clause tests are not written | approximated |
| StateMachine | `state def` (see [Behaviors](#behaviors)); a block's `classifierBehavior` is also exhibited by an `exhibit state` usage of the `part def` | mapped |
| State, composite State, Region | `state`; the regions of an orthogonal state are sub-states of a `parallel` state | mapped |
| State with `submachine` | `state s : SubMachineDef;` — the referenced state machine's own `state def`, not inlined | mapped |
| Pseudostate initial, FinalState | `entry; then s;`, `done` | mapped |
| Pseudostate choice, junction | a `state` its guarded transitions leave at once | approximated |
| Pseudostate exitPoint, terminate | a transition into it is written to `done` | approximated |
| Pseudostate entryPoint, ConnectionPointReference, deep/shallow history, fork/join pseudostates | comment | **unmapped** — no v2 form |
| `entry`, `doActivity`, `exit` behaviors | `entry action { … }` / `do action { … }` / `exit action { … }` inline when the behavior is owned by the state, `entry x;` / `do x : Def;` by reference otherwise | mapped |
| Transition | `transition first s accept Sig if <guard> do <effect> then t;`; several triggers are several transitions; a completion transition is `transition first s then t;` | mapped (several triggers: approximated) |
| Transition `effect` with `in` parameters | the accepted signal is named, `accept sig : Sig`, and each parameter typed by the signal (or a general of it), or the sole untyped one, is bound to it: `in p : Sig = sig;`; a parameter of another type takes no value | mapped (an unbound parameter: approximated) |
| State `deferrableTrigger` on a SignalEvent | `defer Sig;` in the state's body — the OpenSysML `defer` extension (see [Behavior](../guide/06-behavior.md)), which the runtime executes and the validator reports as non-standard notation | approximated |
| Internal transition (`kind = internal`), `deferrableTrigger` on any other event | comment | **unmapped** — no v2 form |
| State `stateInvariant` | comment in the state's body quoting the constraint; the state is written with a body so the comment has a place | **unmapped** — no v2 form |
| Initial transition with a trigger or guard | the region's `entry; then s;`; each trigger and the guard are dropped and reported apart from the transition | approximated (the trigger, the guard: unmapped) |
| SignalEvent, ChangeEvent, relative TimeEvent | written where a trigger refers to them, as `accept Sig`, `accept when <cond>`, `accept after <d> [SI::s]`; an event no trigger refers to is a comment | mapped / approximated |
| Absolute TimeEvent, TimeEvent whose `when` is not a number with a time unit | comment | **unmapped** |
| Interaction | a scenario `action def` of `send`s in occurrence order, when every message is an asynchronous signal send received on a lifeline standing for a part of the interaction's owner | approximated |
| Interaction with a synchronous call, a reply, a message to a lifeline that is not a part, or no message; DurationConstraint on an interaction | comment | **unmapped** — the reason names the message |
| OpaqueBehavior, FunctionBehavior | `calc def` with its parameters when its one body is a v2 expression whose names resolve or a JavaScript expression of the [subset](#the-opaque-language-subset) (`Math.max(a, b)` → `RealFunctions::max(a, b)`); an `action def` whose body is the translated `assign` sequence when the script is statements; otherwise `action def` keeping the body as a comment and the report naming the token refused | mapped / approximated |
| Operation | `action def <Op>` owned by the owner, with its parameters; the `method` behavior is written as its body (an Activity as the flow, an OpaqueBehavior as expression or comment), its parameters standing for the operation's at the same position, direction and type under the operation's names; a method parameter matching none is declared and reported, since a call binds only the operation's; no method: `abstract action def`; an `action <op> : <Op>;` usage of the owner performs it, as a call on an object does | mapped |
| Operation `precondition`, `postcondition`, `bodyCondition` | `assert constraint { <expr> }` in the action def when the expression parses and resolves; otherwise a comment | mapped / approximated |
| Reception | comment on the `part def` naming the signal (the state machine's `accept sig : Sig` already carries it) | approximated |
| «Unit», «QuantityKind» instance specifications | comment placeholder | **unmapped** — use the `SI`/`ISQ` libraries |
| Profiles, the SysML/UML libraries themselves | — | skipped |

The v1 element's `xmi:id` is kept as the reason a report line can be found in the source
model; the notation itself carries no IDs. Stable identity annotations for a re-migration are
future work (see [element identity annotations](../project/element-identity-annotations.md)).

Names that are not v2 identifiers — with spaces, punctuation, or starting with a digit — are
quoted (`'Vehicle Design'`).

The mapping has been run over the XMI of the [OpenMBEE TMT SysML model](https://github.com/Open-MBEE/TMT-SysML-Model)
(27 MB; 44,600 elements once the nodes and edges of its behaviors are counted): it writes 7 MB
of notation that passes the gate below in a few seconds, and its Turtle in a few more. Five
elements in six map or are approximated; the unmapped rest is dominated by absolute and
unparseable time events, call actions that call no behavior, instance specifications without a
classifier, simulation verdicts stored in slots of constraint properties, and views.

## Behaviors

A behavior is migrated so that it *runs*: the `action def` an activity becomes is a token flow
the [action executor](../guide/06-behavior.md) performs, and the `state def` a state machine
becomes is one the state debugger steps. Every generated model is gated to analyse clean, and
the migration tests execute a generated activity and a generated state machine, not only parse
them.

**Activities.** The nodes are written first, then the edges. A node's name is its v1 name when
it has one, else its kind (`call`, `decide`, `fork`, …) made unique within the activity. A
call action is `action call : Def;`, so the callee's flow runs as a nested performance; its pins
are `bind`/`flow` statements from the object flows that reach them. A node several edges leave
without a fork is written through one (`fork fork2;`), and a node several edges reach without
a join waits through one, both reported as approximations. An opaque action whose body is a
script is read statement by statement through the [opaque-language subset](#the-opaque-language-subset):
`i = 1; GS_Found = false;` becomes two `assign` statements, `i += 1` an
`assign this.tcs.i := this.tcs.i + 1;`, and the body is kept as a comment naming its language
and the token refused when any statement is outside the subset or names something unwritten.

**Swimlanes.** An `ActivityPartition` that `represents` a property of the activity's context
block names the object whose features the nodes inside it read and write: a body `i = 1` in
the partition of the part `tcs` is `assign this.tcs.i := 1;`, and a guard `GS_Found` on an
edge whose source sits in that partition is `if this.tcs.GS_Found`. Names are looked up in the
represented object first, then among the activity's own parameters and locals, then in the
owning block; a nested partition reads through its enclosing ones (`this.tank.valve.open`), and
a partition representing the context block itself reads `this`. A node in no partition, and a
partition whose `represents` is unset, names an id the document does not define, a property
with no v2 type, or a classifier the activity does not run in, fall back to the activity and
its block, and the partition's report line says which of these it is. A node held by two
partitions that do not nest — a diagram's two dimensions — resolves through the one that
represents an object when the other represents nothing, and through either when both represent
the same object; when they represent different objects, no partition applies, the node's names
fall back to the activity and its block, and both the node's and the partitions' report lines
say so. An edge in no partition takes its source node's partitions, its target's only when
the source is in none — so a guard leaving such a node is refused the same way, never read
through the target's partition. A name read through a part that holds more than one object —
a partition representing `cells : Gauge[2]`, or a dotted path `cells.reading` — is a
collection, so it feeds `java.util.Collections.max` but not arithmetic or a scalar assignment,
and an assignment through it (`cells.reading = 1`) is refused as writing several objects; a
`CallBehaviorAction` in such a partition runs in the caller's context, no one of the objects
performing it, and its report line says so. The partition's comment stays as documentation of its
membership; its verdict is
*mapped* when a name was resolved through it.

**The clock.** The tool's time variable — `simtime`, or whatever the model's
`SimulationConfig.timeVariableName` names — reads the simulation clock, so a body reading it
is executable; the report line names the configuration that names it, or counts the
configurations when several do. The variable is the tool's global — every configuration's
name is recognized in every body, whichever activity the configuration targets — so a
parameter, pin or property of the same name visible where the body lands shadows it and is
read as that feature. `Time_Acq_Total = simtime;` is
`assign this.Time_Acq_Total := localClock.currentTime;`, and
`Time_Acq_Total = simtime - Time_Acq_Total;` the elapsed time since. `localClock.currentTime`
is the standard library's own form (`Occurrences::Occurrence::localClock`, a `Clock` whose
`currentTime` the [runtime](../guide/06-behavior.md#reading-the-clock) evaluates against the
run's clock), so a migrated model needs no extension library and the attribute is one a run
reports: `-observe this.Time_Acq_Total`, or `%runs` with the same. The clock is read, never
written: a script assigning `simtime` is refused. A `DurationObservation` whose two events are
nodes of the activity is the same bookkeeping written for the modeler: an `attribute` of the
`action def` named after the observation, stamped when the first node starts and assigned the
elapsed clock when the second ends (`firstEvent` chooses, per event, the instant the node's
execution enters it or the instant it exits, as UML defines; a node executed again in a loop
stamps again, so the attribute holds the span between the latest executions of the two
nodes); observations whose events are not nodes of the activity,
and observations owned outside any activity, are comments whose report line says which.

**Durations and probabilities.** A `DurationConstraint` on an action is a wait the action's
token takes before it: `accept after 3.0 [SI::s]` for a point interval, and
`accept after RandomFunctions::uniform(1.0, 80.0) [SI::s]` for a proper one — a draw from
the [model seed](../guide/06-behavior.md#seeds-where-the-draws-come-from). A simulation
tool's `min`/`max`/`random` duration mode belongs to its run configuration, not to the model,
so the interval is migrated faithfully as a random duration; a run with `-seed`/`%seed`
reproduces the tool's random mode, and the fixed modes are a run setting to add rather than a
fact to bake into the notation. «Probability» on the edges out of a decision is written as
`@Stochastic::Probability { p = 0.5; }` on each succession when every edge carries one — the
rule v1 states itself — and the weights are scaled to sum to 1 when they do not; a decision
with weights on only some edges, or a weight outside `[0, 1]`, is written unweighted and the
report says why. Guards that are opaque English (`[Align BTO]`) are kept as comments and the
edge written unguarded, so such a decision is a scheduling choice the runtime draws at random
with the model seed; the report says so. A guard in English that the subset reads — `TRUE`, a
Boolean property's name, `not Found and i < Retries` — is written as the `if` it means.

**State machines.** A composite state's regions become sub-states of a `parallel` state, so
the orthogonal regions run together; a submachine state is a `state` usage typed by the
referenced machine's `state def`, composing through any depth. Triggers are written on the
transition that refers to them — `accept Sig`, `accept after 2.0 [SI::s]`,
`accept when this.temperature > 200.0` — and the event's own report line says where. An effect
with parameters reads the accepted signal: the accept names it, `accept sig : Sig`, and the
parameters the signal fits are bound to that name. Entry, do and exit behaviors owned by the
state are inline action bodies, on a submachine state as on any other; those it only refers to
are `entry x;` references. A transition into an exit point or a terminate pseudostate is written to `done`; entry points,
connection point references, history pseudostates and internal transitions have no v2 form and
are comments.

**Running a migrated behavior.** Instantiate the block whose classifier behavior the activity
or state machine is, then step it or run it many times with the model seed:

```text
%runs 100 1 Model::'Mission'::'Acquire Target'::'Acquire Target - Logical'
%instantiate Model::APS::'Acquisition Pointing and Tracking Assembly'
%state Model::APS::'Acquisition Pointing and Tracking Assembly'::CC_APT
%send Model::APS::Signals::'Select APT Filter_Cmd'
%step
```

`%runs` reports the clock at the end of each run — the workflow's total duration — as min,
mean, max, p50 and p90 with a histogram, and `sysml model.sysml -action <name> -runs 100 -seed 1`
does the same from the command line. An attribute the migrated body assigns from the clock is
observed beside it with `-observe this.Time_Acq_Total`; an action that reads its performer's
features (the `this.tcs.i` of a swimlane) is run through the performer, `-action "'Block' 'Action'"`.

## The opaque-language subset

A v1 body carries a `language` and a text the tool executed — JavaScript, in Cameo's case, or
"English" for guards written as prose. The migrator translates a bounded subset of each into
v2 expressions and statements, and refuses the rest with a typed reason naming the token, so a
translation is always complete or absent — never partial.

**Scripts** (`language` JavaScript, ECMAScript, Java, or none) are read as statements:

| Script | v2 |
|---|---|
| `x = e;` `x += e;` `-=` `*=` `/=` `x++` `x--` | `assign x := e;` `assign x := x + e;` … |
| `var x = e;` `let x = e;` `const x = e;` (one name, initialized) | `attribute x : ScalarValues::T;` `assign x := e;` with `T` the type of `e`; a later assignment to a `const` is refused, as is a declaration of a name already declared, of a pin, parameter or property visible where the body lands, or of a member every action has (`start`, `done`, `self`) |
| several statements, on `;` or newlines | a sequence of the above |
| integer, real, Boolean and string literals | the same literal; a whole number is refused beyond what an `Integer` holds (2⁶³ − 1), and in a JavaScript body beyond 2⁵³ − 1, since the script would round it to a `Number` (a Java body's `long` is exact); a string's `\n` `\t` `\r` `\b` `\f` `\\` `\'` `\"` `\xHH` `\uHHHH` `\u{H…}` escapes and line continuations are decoded, a high and low surrogate escape pair as the one character they spell, while a legacy octal escape or a character the notation cannot spell (`\0`, `\v`, other control characters, a lone surrogate) is refused |
| `a`, `a.b.c` naming features that resolve | `this.a`, `this.a.b.c` (through the swimlane's object when it has one) |
| `+ - * / %`, comparisons, `&& \|\| !`, parentheses | `+ - * / %`, comparisons, `and or not`, parentheses; a Java body's `/` of two whole numbers drops the remainder, so it is `OpenSysMLMathFunctions::quotient(x, y)` (the exact Integer quotient truncated toward zero, refused at run time only for the least Integer by `-1`, whose quotient no Integer holds), and is refused when the operands' types cannot tell whether both are whole |
| `c ? a : b` | `if c ? a else b` when `a` and `b` are of one scalar type |
| `Math.min` `Math.max` `Math.abs` `Math.floor` `Math.ceil` `Math.round` `Math.sqrt` `Math.pow`, `a ** b` | `RealFunctions::min` … `RealFunctions::sqrt`, `**`; `Math.ceil(x)` is `-RealFunctions::floor(-x)` and `Math.round(x)` is `RealFunctions::floor(x + 0.5)`, which rounds a half toward +∞ as JavaScript does; `-a ** b` is refused, as JavaScript rejects a unary operand of `**` without parentheses, and a Java body's `**` is refused, Java having no such operator |
| `java.util.Collections.max(s)` / `.min(s)` | `RealFunctions::max(s)` / `RealFunctions::min(s)` over a collection |
| the tool's time variable (`simtime`) | `localClock.currentTime` |

**English** (`language` English, natural language, text) is read as one Boolean expression:
`TRUE` / `FALSE` / `true` / `false`; a property name of Boolean type, spaces and all; `not X`;
`X and Y`; `X or Y`; comparisons written with `=` or `==`, `<`, `>`, `<=`, `>=`, `!=`.

**Refusals.** Anything else is refused, and the report line carries the reason with the token
that caused it: a language not in the table (`the language "Groovy" is not translated`), text
that is not expression syntax, a construct outside the subset (`for`, `while`, `if` statements,
`new`, `function`, a declaration of several names or of a name a feature already has, an
assignment to a `const`, to an `in` parameter or to an input pin, a string method, a regular expression, an expression that assigns nothing, text
after the one expression a guard or default is), a call not in the table (`the call "print" is not in the
translated function table`), a name that resolves to nothing readable (`this.` in a context with
no object, a property of no v2 type, a name no scope defines), or types that disagree (an
`Integer` guard, a `Boolean` added to a `Real`, a plural where a scalar is wanted, a feature
typed by an enumeration or a block where a number or Boolean is wanted, assigned to a feature
of a type that neither is nor generalizes its own, or compared with or chosen beside one sharing
no type with it). A feature whose type the migrator does not know is trusted to fit. A body
whose language the translator reads but whose text it refuses is never re-read as v2 syntax:
the refusal is final, and the body is a comment.

A translation is emitted only when every name resolves to a written feature visible where the
statement lands, the types agree wherever they can be told (a guard is `Boolean`, an
assignment fits its target, a default or the value of a typed pin or result is of the feature's
type — its scalar, or a block or enumeration it is or specializes — and one value unless the
feature holds several, a duration is `Real`), and the result parses with the v2 parser.

## The report

Nothing is dropped silently. Every element the reader saw is in the report exactly once with
one of four verdicts:

- **mapped** — a faithful v2 form.
- **approximated** — written, but not one-to-one; the note says what was lost or changed.
- **unmapped** — no v2 form was written. The element appears in the notation as a comment at
  the place it would have gone, so a reader of the migrated model can see the gap.
- **skipped** — profile and library content that is not part of the user's model.

The text form (default) groups by verdict, unmapped first, one line per element: kind with its
stereotypes, qualified v1 name, `xmi:id`, the v2 name it became, and a note. The JSON form
(`-migration-report x.json`) is the same content as `{source, exporter, entries: [{id, kind,
name, target, verdict, note}]}` for tooling. Without `-migration-report`, the one-line summary
goes to stderr.

## Guarantees

Every migrated model is gated in the test suite to:

1. parse and analyse clean under the v2 semantic passes (`go test ./internal/core/migrate`),
2. round-trip through Turtle (notation → `.ttl` → notation → `.ttl`) without changing its graph,
3. account for every element in the report, and leave a comment for every unmapped one.

A model the reader cannot make sense of — not XMI, a zipped project container, a document
without a model — is refused with an error naming the reason rather than migrated partially.
