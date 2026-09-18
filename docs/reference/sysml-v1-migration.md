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
| InstanceSpecification naming no classifier, whose slots are of features of one block (a simulation tool's result snapshot) | the `individual part def` of the owner of its slots' features, with its slots; the note says which owner classified it | mapped |
| InstanceSpecification naming no classifier and holding no slot of a written feature | comment | **unmapped** — nothing classifies it |
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
| OpaqueExpression defaults and constraints | copied verbatim when it parses as a v2 expression and every name it uses is a written element visible where it is written (a parameter, an inherited feature, an enclosing member); a script's `java.util…` path, a bare enumeration literal or an operation is not, and the body stays a `comment` | mapped / approximated |
| Activity | `action def` (see [Behaviors](#behaviors)); a block's `classifierBehavior` is also performed by a `perform action` usage of the `part def` | mapped |
| Parameter, ActivityParameterNode | `in`/`out`/`inout` parameter of the `action def`; a `return` parameter is `out`; the parameter node's flows bind the parameter | mapped (return: approximated) |
| InitialNode, ActivityFinalNode, FlowFinalNode | `first start then …`; `action x terminate;`; the token ends where a flow final does | mapped |
| ForkNode, JoinNode, DecisionNode, MergeNode | `fork`, `join`, `decide`, `merge`; a node several edges leave or reach without a control node gets one written for it | mapped (implicit fork/join: approximated) |
| CallBehaviorAction | `action x : Def;` with `bind`/`flow` for its pins; a call of a state machine, of a behavior with no v2 declaration, or of no behavior at all | mapped / **unmapped** |
| CallOperationAction | `action x : Owner::Op;`, or `perform action x ::> target.op;` when the target pin's value is an object whose type owns the operation | mapped |
| ControlFlow | `first a then b;`, `if <guard>` when the guard parses and resolves; otherwise the guard text as a comment and the edge unguarded | mapped / approximated |
| «Probability» on the edges out of a decision, a number | `first d then x { @Stochastic::Probability { p = <value>; } }`; constants not summing to 1 are scaled by their sum; a value outside `[0, 1]` leaves the decision unweighted | mapped / approximated |
| «Probability» naming a property (by name or `xmi:id`) visible from the activity — its own, or one of the block whose classifier behavior it is, inherited included — typed by a numeric value type and holding one value | `p = <property>;`, a reference the run reads from the object performing the action when the decision is reached, checking then that it lies in `[0, 1]` and the branches sum to 1 | mapped |
| «Probability» naming a property that is not visible, private to another block, not numeric, or of a multiplicity other than one; naming an element that is no property; or no property or number at all | the decision is written unweighted; the note says what the tag names and why it is no weight | approximated |
| Edge out of a weighted decision carrying no «Probability» | weighted with its share of what the marked edges leave of 1 — a constant, or `1.0 - <property>` read when the decision is reached | approximated |
| «SimulationConfig» (MagicDraw's SimulationProfile) | `action def` holding `@Simulation::Configuration { runs = …; draws = …; timeVariable = …; startTime = …; stepSize = …; timeUnit = …; parallelForks = …; }`, `part target : <the migrated executionTarget>;` and `perform action run ::> target.<its classifier behavior>;` (see [Run configurations](#run-configurations)); its remaining tags a comment | mapped |
| «SimulationConfig» whose `executionTarget` is absent, several, outside the document, not migrated, or written as something no part can be typed by; whose target has no classifier behavior, or one that is a state machine | the `action def` with its metadata and, where the target is written, its `target` part, performing nothing; the note says why | approximated |
| «SimulationConfig» `durationSimulationMode` that is none of `min`, `max`, `average`, `random` | kept among the tags in the comment | approximated |
| Result snapshots of a «SimulationConfig» (the instances under its `resultLocation` packages classified — by name or by their slots — by its target's classifiers, recording no other values of the features the target's slots set) | the individuals above, and one row per snapshot in the JSON `-migration-results` writes, its numeric slots by defining feature; a slot holding no one finite number, and a feature two slots hold numbers for, are counted in the configuration's notes | mapped |
| ObjectFlow | `flow a.out to b.in;`, or `bind` to a parameter; each producer-pin pair is written once however many edges carry it; a flow from or to an action that is not migrated is a comment | mapped / approximated |
| SendSignalAction | `action x send new Sig(args) to <target>;`, `via <port>` when `onPort` is set; the target is read from the target pin's flow: `this`, `this.part` where a structural read feeds the pin, else the pin itself (`in target;` bound to what feeds it, an activity parameter or another node's output), which the runtime evaluates to the object it holds | mapped / approximated |
| AcceptEventAction | `action x accept p : Sig;` (signal trigger), `accept after <d> [SI::s]` (relative TimeEvent), `accept when <cond>` (ChangeEvent) | mapped |
| AcceptEventAction on an absolute TimeEvent (`when` is an instant, not a duration) | comment | **unmapped** — no literal writes a `TimeInstantValue` |
| OpaqueAction, ValueSpecificationAction, ReadStructuralFeatureAction, AddStructuralFeatureValueAction | `assign`/`out result = …` when the body parses as a v2 expression whose names resolve (a script's `x = expr;` statements are read as assignments); otherwise the body as a comment inside `action x { }` naming the language | mapped / approximated |
| DurationConstraint on an action | a wait before the action: `accept after lo [SI::s]` when the interval is a point, `accept after RandomFunctions::uniform(lo, hi) [SI::s]` otherwise; `1s`, `0.5 s`, `80ms`, `2 min`, `1 h` and `t = 1 minute 30 seconds` literals are scaled to seconds | approximated (a tool's min/max/average/random mode is the run's `-draws` policy, which its configuration records) |
| DurationConstraint whose bounds are not numbers with time units (`setup s`), DurationObservation, TimeObservation | comment | **unmapped** — the runtime reports a run's clock |
| ActivityPartition | comment naming the partition and its nodes (`perform … by` has no legal form for a partition of arbitrary nodes) | approximated |
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
| OpaqueBehavior, FunctionBehavior | `calc def` with its parameters when its one body is a v2 expression whose names resolve; otherwise `action def` keeping the body as a comment | mapped / approximated |
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
unparseable time events, call actions that call no behavior, simulation verdicts stored in
slots of constraint properties, and views.

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
script is read statement by statement: `Time_Acq_Total = simtime;` and its like become
`assign this.Time_Acq_Total := …;` when every name resolves to a written feature, and the body
is otherwise kept as a comment naming its language. The tool's time variable (`simtime`) is
not a feature of the model but a simulation setting, so a body reading it stays a comment;
the total duration of a run is what the runtime's clock reports at its end, which `%runs`
measures directly.

**Durations and probabilities.** A `DurationConstraint` on an action is a wait the action's
token takes before it: `accept after 3.0 [SI::s]` for a point interval, and
`accept after RandomFunctions::uniform(1.0, 80.0) [SI::s]` for a proper one — a draw from
the [model seed](../guide/06-behavior.md#seeds-where-the-draws-come-from). A simulation
tool's `min`/`max`/`average`/`random` duration mode belongs to its run configuration, not to
the model, so the interval is migrated faithfully as a random duration and the mode is the
[draw policy](../guide/06-behavior.md#draw-policies-min-max-average-and-random) of the run —
`-draws random -seed <n>` reproduces the tool's random mode, `-draws max` its max mode — which
each migrated configuration records (below). «Probability» on the edges out of a decision is
written as `@Stochastic::Probability { p = … }` on each succession: a tag that is a number is
the constant `p = 0.5;`, and one that names a property of the activity or of the block whose
classifier behavior it is — the v1 idiom of an analysis block whose `ProbabilityBTOOP : Real`
each run configuration sets to `1.0` or `0.0` through its execution target's slots — is the
reference `p = ProbabilityBTOOP;`, which the run reads from the object performing the action
when the decision is reached, so the same behavior takes different odds on differently
configured objects. The property must be reachable from the action's execution context: a
numeric property holding one value, visible from the activity or inherited by its context
block; a tag naming anything else leaves the decision unweighted and the report says what it
names. An edge without a tag beside tagged ones takes its share of the remainder, `1.0 -
ProbabilityBTOOP` when the tagged one is a reference; constants that do not sum to 1 are scaled
by their sum, and a constant outside `[0, 1]` leaves the decision unweighted, each reported.
Guards that are opaque English (`[Align BTO]`) are kept as comments and the
edge written unguarded, so such a decision is a scheduling choice the runtime draws at random
with the model seed; the report says so.

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
does the same from the command line.

## Run configurations

A simulation tool's run configuration — MagicDraw's «SimulationConfig», recognised by the
provenance of its profile (`…/schemas/SimulationProfile.xmi`), not by its name — states which
object a behavior ran on, how many times, and how the tool resolved its random durations. Each
becomes an `action def` a user runs as any other:

```sysml
action def 'Group 0' {
    @Simulation::Configuration {
        runs = 1000;
        draws = Simulation::DrawPolicy::random;
        timeVariable = "simtime";
        startTime = 0.0;
        stepSize = 0.01;
        timeUnit = "second";
        parallelForks = true;
    }
    part target : Analysis::'Analysis Group 0';
    perform action run ::> target.'acquire Target - Logical';
    /* results of the simulation tool: 13 snapshot(s) in Analysis::Results::'Group 0' holding ProbabilityBTOOP, Time_Acq_Total, … */
    /* «SimulationConfig» settings of the simulation tool: animationSpeed = 97; silent = true; … */
}
```

- The `executionTarget` is the `target` part, typed by the individual the target instance
  became — so its slots, the configuration's property values, are the attributes the run reads,
  its «Probability» references included — and the classifier behavior of the target's
  classifier (the nearest written one, up the generalizations) is performed on it by `run`. A
  target that is absent, several, outside the document, not migrated, or written as a
  definition no part can be typed by (a port def, an attribute def), a classifier with no
  classifier behavior, and a classifier behavior that is a state machine each leave the
  configuration performing nothing, with the reason in the report and the `action def` still
  written, holding its metadata and whatever part it could.
- `numberOfRuns` and `durationSimulationMode` are the `runs` and `draws` of
  [`Simulation::Configuration`](../guide/06-behavior.md#draw-policies-min-max-average-and-random):
  the count to pass as `-runs` and the policy to pass as `-draws`, which OpenSysML's runs do not
  read from the model but the harness below applies. `timeVariableName`, `startTime`, `stepSize`,
  `timeUnit` and `runForksInParallel` are recorded as `timeVariable`, `startTime`, `stepSize`,
  `timeUnit` and `parallelForks`: they describe the clock the tool ran on, and OpenSysML's clock
  is the run's own, so they are recorded, not applied. A mode that is none of the four policies,
  and a run count beyond what a Monte Carlo can make (a 64-bit count), are kept among the
  tool's other tags in the trailing comment, as are `animationSpeed`,
  `silent` and every setting with no v2 meaning; `autostartActiveObjects` and
  `treatAllClassifiersAsActive` set to true state what every v2 object does anyway, so they are
  consumed, and set to false they are kept in the comment and reported as having no v2 form.
- The tool's own results — the snapshots it stored of the configuration's runs under its
  `resultLocation` packages, one instance per run whose slots hold the observed values, most
  naming no classifier — are migrated as individuals of the block their slots' features belong
  to, and indexed per configuration in the JSON sidecar
  `-convert sysml … -migration-results results.json` writes beside the notation:

  ```json
  {"source": "model.xmi", "configurations": [
    {"id": "_g0", "name": "Group 0", "runs": 1000, "draws": "random",
     "target": "target", "behavior": "run", "resultLocation": "Analysis::Results::Group 0",
     "observables": ["Time_Acq_Total", "Time_Dither"],
     "snapshots": [{"id": "_s1", "name": "Acq 1", "values": {"Time_Acq_Total": 80.228, "Time_Dither": 0.0}}],
     "notes": ["the slot of Verdict holds a LiteralBoolean, which is no number in 13 snapshot(s), so it is not among the results"]}]}
  ```

  A JSON sidecar rather than a v2 result table, because the snapshots are the tool's
  measurements of the *tool's* run, not facts of the model: the notation carries them as
  individuals a reader can inspect, and the sidecar carries them in the form the comparison
  reads without re-parsing the model. A snapshot records the state of the object the run was
  made on, its configured features included, so one whose slots record other values of the
  features the target's own slots set — several configurations often share one result
  package — is of another configuration and left out with a note counting it. Locations that
  repeat or nest (a package and a sub-package of it) index each snapshot once. A
  `resultLocation` outside the document, a snapshot slot with no defining feature in the
  document or holding no one finite number, a feature two slots of one snapshot hold numbers
  for (left out of that snapshot: it has no one result there), and a target with no
  classifier to match snapshots against are each noted in the configuration's `notes`.

`sysml model.sysml -compare-results results.json` then runs every configuration the sidecar
indexes — with its `runs` and `draws`, or the `-runs` and `-draws` given, seeded from `-seed` —
and prints, per observable, the tool's and OpenSysML's min, mean, p50, p90 and max with their
relative difference; see
[Comparing a migrated configuration with the tool's results](cli.md#comparing-a-migrated-configuration-with-the-tools-results).
A stored observable is read off the target by default (`Time_Acq_Total` beside
`target.Time_Acq_Total`), or off the feature `-observe Time_Acq_Total=clock` names, so a total
the tool read from its time variable is set beside the run's clock. An observable the completed
runs produce in more than one unit (a quantity in some, a bare number or another unit in others)
has no one distribution to set beside the tool's and is noted, not pooled. The numbers are printed
as they are: a difference is a fact about the migration's fidelity, to be read against the
report's approximations, not tuned away.

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
goes to stderr. `-migration-results x.json` writes beside it the
[result snapshots](#run-configurations) of every run configuration, and reports what it wrote
in one line: `results of 3 run configuration(s): 2 with 15 stored snapshot(s)`.

## Guarantees

Every migrated model is gated in the test suite to:

1. parse and analyse clean under the v2 semantic passes (`go test ./internal/core/migrate`),
2. round-trip through Turtle (notation → `.ttl` → notation → `.ttl`) without changing its graph,
3. account for every element in the report, and leave a comment for every unmapped one.

A model the reader cannot make sense of — not XMI, a zipped project container, a document
without a model — is refused with an error naming the reason rather than migrated partially.
