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
| InstanceSpecification naming no classifier, under a `SimulationConfig`'s `resultLocation`, whose slots are of features of one lineage of blocks ending in the configuration's target classifier or a general of it (a simulation tool's result snapshot) | the `individual part def` of the most special of those blocks, with its slots; the note says which owner classified it and for which configuration | mapped |
| InstanceSpecification naming no classifier, anywhere else, or under a `resultLocation` with slots of features of blocks that are no one lineage or none the target is of | comment | **unmapped** — nothing classifies it; under a `resultLocation` the note says which owners its slots have and why they type no snapshot |
| Slot contradicting its feature (more values than the multiplicity allows, a repeated value of a unique feature, a feature of a classifier the instance is not written to specialize, an instance that is not of the property's type or of its default individual, a value outside the document) | comment | **unmapped** |
| Slot of a port, or of an untyped property | comment (no individual can type a port; a `ref` without a type takes none) | **unmapped** |
| Property whose default is an InstanceSpecification of a block | the individual added to the usage's types, or its only type when the property is untyped; no `default` (a definition is not a v2 value). A port, a usage of another kind than the individual, or a usage whose type the individual is not an instance of, keeps its types and the default is a comment | approximated |
| Literal default on a value type with no scalar base (a structured value type, an enumeration) | comment | approximated |
| Real literal on an `Integer`/`Natural` feature, numeric string on a scalar feature | converted to the feature's scalar | mapped |
| Constraint whose specification is a literal, instance or opaque body yielding no Boolean (an integer, a real, a string spelling no `true`/`false`, an enumeration literal) | comment naming the value and the Boolean the constraint yields | **unmapped** — no v2 checker accepts a constraint body of another type; a string `"true"`/`"false"` is written as the Boolean it spells |
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
| Two members of one namespace with the same name (UML allows it, v2 does not) | the later one renamed `Name 2`, a state, pseudostate or history a machine's region puts beside its attributes included, as is a written connection point of a state whichever region a tool listed it in, while one written as no member takes no name; a connection end named like a member of its connection def renamed `name2` | approximated |
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
| CallOperationAction | `perform action x ::> target.op;` when the target pin's value is an object whose type owns the operation, or when `onPort` names a port a connector of the caller's block joins to a part that owns it (a port of the target itself names it); otherwise `action x : Owner::Op;`, which runs in the caller's context | mapped / approximated (unresolved target: the reason names it) |
| ControlFlow | `first a then b;`, `if <guard>` when the guard parses and resolves as a v2 expression or translates from JavaScript or English (`i >= Retries`, `GS_Found`, `not Found and i < 3`, `TRUE`) through the [subset](#the-opaque-language-subset); otherwise the guard text as a comment and the edge unguarded, the report naming the token refused | mapped / approximated |
| «Probability» on the edges out of a decision, a number | `first d then x { @Stochastic::Probability { p = <value>; } }`; constants not summing to 1 are scaled by their sum; a value outside `[0, 1]` leaves the decision unweighted | mapped / approximated |
| «Probability» naming a property (by name or `xmi:id`) visible from the activity — its own, or one of the block whose classifier behavior it is, inherited included — typed by a numeric value type and holding one value | `p = <property>;`, a reference the run reads from the object performing the action when the decision is reached, checking then that it lies in `[0, 1]` and the branches sum to 1 | mapped |
| «Probability» naming a property that is not visible, private to another block, not numeric, or of a multiplicity other than one; naming an element that is no property; or no property or number at all | the decision is written unweighted; the note says what the tag names and why it is no weight | approximated |
| Edge out of a weighted decision carrying no «Probability» | weighted with its share of what the marked edges leave of 1 — a constant, or `1.0 - <property>` read when the decision is reached | approximated |
| «SimulationConfig» (MagicDraw's SimulationProfile) | `action def` holding `@Simulation::Configuration { runs = …; draws = …; timeVariable = …; startTime = …; stepSize = …; timeUnit = …; parallelForks = …; }`, `part target : <the migrated executionTarget>;` and `perform action run ::> target.<its classifier behavior>;` (see [Run configurations](#run-configurations)); its remaining tags a comment | mapped |
| «SimulationConfig» whose `executionTarget` is absent, several, outside the document, not migrated, or written as something no part can be typed by; whose target has no classifier behavior, or one that is a state machine | the `action def` with its metadata and, where the target is written, its `target` part, performing nothing; the note says why | approximated |
| «SimulationConfig» `durationSimulationMode` that is none of `min`, `max`, `average`, `random` | kept among the tags in the comment | approximated |
| Result snapshots of a «SimulationConfig» (the instances under its `resultLocation` packages classified — by name or by their slots — by its target's classifiers, recording no other values of the features the target's slots set) | the individuals above, and one row per snapshot in the JSON `-migration-results` writes, its numeric slots by defining feature; a slot holding no one finite number a float64 spells exactly, and a feature two slots hold numbers for, are counted in the configuration's notes | mapped |
| ObjectFlow | `flow a.out to b.in;`, or `bind` to a parameter; each producer-pin pair is written once however many edges carry it; a flow from or to an action that is not migrated, or from an output pin a translated opaque body never assigns, is a comment | mapped / approximated |
| SendSignalAction | `action x send new Sig(args) to <target>;`, `via <port>` when `onPort` is set; the target is read from the target pin's flow: `this`, `this.part` where a structural read feeds the pin, else the pin itself (`in target;` bound to what feeds it, an activity parameter or another node's output), which the runtime evaluates to the object it holds | mapped / approximated |
| AcceptEventAction | `action x accept p : Sig;` (signal trigger), `accept after <d> [SI::s]` (relative TimeEvent), `accept when <cond>` (ChangeEvent) | mapped |
| AcceptEventAction on an absolute TimeEvent (`when` is an instant, not a duration) | `accept at <instant>`, the instant a `Time::TimeInstantValue` attribute of the `action def` when `when` is a number with a time unit or an expression that resolves; otherwise a comment | approximated (the instant is read on the simulation clock, which starts at 0) / **unmapped** |
| OpaqueAction, ValueSpecificationAction, ReadStructuralFeatureAction, AddStructuralFeatureValueAction | `assign`/`out result = …` when the body parses as a v2 expression whose names resolve, or is a JavaScript body of the [subset](#the-opaque-language-subset): `i = 1; GS_Found = true;` is a sequence of `assign` statements, `i += 1` an assignment of `i + 1`, `var t = 0` a local `attribute`; names resolve against the action's own pins first, then the swimlane's represented object, then the activity, then the owning block; otherwise the body as a comment inside `action x { }` naming the language and the token refused | mapped / approximated |
| DurationConstraint on an action | a wait before the action: `accept after lo [SI::s]` when the interval is a point, `accept after RandomFunctions::uniform(lo, hi) [SI::s]` otherwise; `1s`, `0.5 s`, `80ms`, `2 min`, `1 h` and `t = 1 minute 30 seconds` literals are scaled to seconds; a symbolic bound (`ditSetup s`, `setup * 2 min`) is an expression whose names resolve like an action body's, `accept after this.tcs.ditSetup [SI::s]` | approximated (a tool's min/max/average/random mode is the run's `-draws` policy, which its configuration records) |
| DurationConstraint whose interval is open on one side (a min with no max, a max of `*`, a max with no min) | comment naming the bound it lacks | **unmapped** — every wait past the bound satisfies the interval, so no one delay stands for it; a MagicDraw document's min beside a max that is a duration with no expression is that tool's encoding of a one-valued `{60s}` and is written as its fixed wait, approximated |
| DurationConstraint whose bounds name nothing the activity can read | comment | **unmapped** — the note names the unresolved name |
| DurationObservation whose events are two nodes of one activity | an `attribute <name> : Real [0..1]` of the `action def`, stamped with `localClock.currentTime` when the first node starts and assigned the elapsed clock when the second ends (`assign T := localClock.currentTime - 'T start';`, guarded on the stamp having happened); one node observed is its own duration; an initial node's start is the activity's `start`, a flow final's or a control node no edge leaves the token's arrival before `done`; the attribute is one a run can `-observe`, and a run that does not reach both nodes leaves it without a value | mapped |
| DurationObservation reading the clock at the end of a node that is no action — an initial, final, flow final or control node has no end of its own | comment | **unmapped** — the note names the node |
| DurationObservation whose events are not nodes of the activity, or none, or name an element the document does not define; a DurationObservation or TimeObservation owned outside an activity; TimeObservation | comment | **unmapped** — the note names the events, or the owner |
| ActivityPartition | comment naming the partition, what it `represents` and its nodes; a name a body or guard in the partition uses is resolved against the represented property first and written through it, `this.tcs.i` for a partition representing the part `tcs` (a nested partition through its enclosing ones, `this.tank.valve.open`; a partition representing the context block itself, `this.x`) | mapped when the partition resolved a name / approximated when nothing in it needed one, when `represents` is unset, names nothing the document defines, a property of no v2 type, or a classifier the activity does not run in |
| StructuredActivityNode, SequenceNode | `action x { }` holding the nested flow | mapped |
| ExpansionRegion, LoopNode, ConditionalNode | `action x { }` holding the body's flow once; the expansion, the loop test and the clause tests are not written | approximated |
| StateMachine | `state def` (see [Behaviors](#behaviors)); a block's `classifierBehavior` is also exhibited by an `exhibit state` usage of the `part def` | mapped |
| State, composite State, Region | `state`; the regions of an orthogonal state are sub-states of a `parallel` state | mapped |
| State with `submachine` | `state s : SubMachineDef;` — the referenced state machine's own `state def`, not inlined | mapped |
| Pseudostate initial, FinalState | `entry; then s;`, `done` | mapped |
| Pseudostate choice, junction | `junction x;` / `choice x;` — a transient node the guarded transitions leave at once | mapped |
| Pseudostate fork, join | `fork x;` / `join x;` — the transitions out of a fork enter the states of several regions of a `parallel` state, those into a join leave them | mapped |
| Pseudostate shallowHistory, deepHistory | `history x;` / `deep history x;` in the composite state; a transition targeting it re-enters the substate (the innermost substates) active when the state was last left, the history's own outgoing transition being its default | mapped |
| Pseudostate entryPoint, exitPoint on a state machine | a `state` of the submachine's `state def`; a transition into an entry point continues by the entry point's own transition, a transition out of an exit point leaves the submachine state | mapped |
| Pseudostate entryPoint on a composite State (`State.connectionPoint`) | `junction x;` of the state, a transition into it written `then Work::x` by path; the runtime runs the state's entry behavior, then the junction's outgoing transition, then the target's entries, in one run-to-completion step. One whose outgoing transitions each start a different orthogonal region is `fork x;`; one no transition leaves is the state's default entry, and the transition is written to the state | mapped |
| Pseudostate exitPoint on a composite State | `junction x;` of the state, a transition out of it written `first Work::x` by path; the runtime runs the transition into it (its source's exits, its effect), the state's exit behavior, then the outgoing transition. One reached from several orthogonal regions is `join x;`, left through when every region's transition has fired. A connection point a tool lists among a region's vertices belongs to the state all the same; a region listing nothing else is skipped, not written as a region of a `parallel` state | mapped |
| Entry point leading straight to an exit point of the same state, back to the state itself, out of the state, into a history pseudostate or to no target, or whose outgoing transition has a trigger, or several of whose outgoing transitions start the same region; an exit point reached from outside its state, or one several regions reach that is also reached twice from one region, from the state's own local transition, or from a pseudostate; a connection point route into a history pseudostate | refused with the shape named | unmapped |
| Pseudostate exitPoint on a region, terminate | a transition into it is written to `done` | approximated |
| ConnectionPointReference on a submachine state | the transition is written to `s.<entryPoint>` / from `s.<exitPoint>`, the submachine's state named by its path | mapped |
| Transition between regions or nesting levels (source or target not a sibling) | the transition names the far end by its path, `Work::Run`; a local transition into a substate of its source is written external, so the composite state exits and re-enters | mapped (local into own substate: approximated) |
| `entry`, `doActivity`, `exit` behaviors | `entry action { … }` / `do action { … }` / `exit action { … }` inline when the behavior is owned by the state, `entry x;` / `do x : Def;` by reference otherwise | mapped |
| Transition | `transition first s accept Sig if <guard> do <effect> then t;`; several triggers are several transitions; a completion transition is `transition first s then t;`; the guard is the transition's `guard` child or the owned rule its `guard` reference names, and a `LiteralBoolean` guard whose value the file omits is `false`, the UML default | mapped (several triggers: approximated) |
| Transition `effect` with `in` parameters | the accepted signal is named, `accept sig : Sig`, and each parameter typed by the signal (or a general of it), or the sole untyped one, is bound to it: `in p : Sig = sig;`; a parameter of another type takes no value | mapped (an unbound parameter: approximated) |
| State `deferrableTrigger` on a SignalEvent | `defer Sig;` in the state's body — the OpenSysML `defer` extension (see [Behavior](../guide/06-behavior.md)), which the runtime executes and the validator reports as non-standard notation | approximated |
| Internal transition (`kind = internal`) | a self transition of the state; faithful when the state has no entry, exit or do behavior and no substates (re-entry is not observable), otherwise the exit and entry run where v1 stayed in the state; one without a trigger is a comment, as a self transition would fire again on every re-entry | mapped / approximated / **unmapped** |
| `deferrableTrigger` on any other event | comment | **unmapped** — no v2 form |
| State `stateInvariant` | comment in the state's body quoting the constraint; the state is written with a body so the comment has a place | **unmapped** — no v2 form |
| Initial transition with a trigger or guard | the region's `entry; then s;`; each trigger and the guard are dropped and reported apart from the transition | approximated (the trigger, the guard: unmapped) |
| SignalEvent, ChangeEvent, relative TimeEvent | written where a trigger refers to them, as `accept Sig`, `accept when <cond>`, `accept after <d> [SI::s]` | mapped / approximated |
| Absolute TimeEvent a trigger refers to | `accept at <instant>` on the transition or accept action, the instant an attribute of the `state def`/`action def` typed `Time::TimeInstantValue` when `when` is a number with a time unit or an expression that resolves, read on the simulation clock, which starts at 0 | approximated (the clock's origin is the run's, not the calendar's) |
| Event (of any kind) no trigger refers to | — | skipped, counted as a model element nothing refers to |
| SignalEvent whose signal is not written, TimeEvent whose `when` is not a number with a time unit | comment; the transition that refers to it drops the trigger | **unmapped** — the reason names the signal or the time |
| Interaction | a scenario `action def` on the owning block: each message in occurrence order as a step — a signal send `send new Sig(args) to this.part`, a `synchCall`/`asynchCall` of an operation `perform action x : Owner::Op ::> part.op { in p = arg; }` with the arguments bound to the `in` parameters by name or position, a `reply` an assignment of the call's `out` to the caller lifeline's attribute the reply's argument names | approximated (the lifelines' own behavior is not part of it; an `asynchCall` waits for the callee where v1 did not) |
| Lifeline | the feature path from the owning block to the part, port or reference it `represents`, through the parts and their types (`drive.motor`), or the interaction's `in` parameter | mapped |
| Lifeline with a `selector`, standing for an `out` parameter, a property no part of the owning block reaches or one reached along two paths, or for no ConnectableElement; Interaction owned by a Collaboration or by no block | comment: the whole interaction is refused | **unmapped** — the reason names the lifeline |
| CombinedFragment `alt`, `opt` | `action x { if <guard> { … } else { … } }` when every guard parses as a v2 expression whose names resolve | mapped |
| CombinedFragment `loop` | `for i in 1..n { … }` when `minint = maxint`, `while <guard> { … }` when the guard is unbounded; a guard with bounds is refused | mapped |
| CombinedFragment `par` | `fork x;` … `join xEnd;` around the operands | mapped |
| CombinedFragment `seq`, `strict` | the operands in order | mapped |
| CombinedFragment with a guard that does not parse or resolve, an `alt` with an unguarded operand before its last, or of another operator (`critical`, `neg`, `assert`, `ignore`, `consider`, `break`) | comment: the whole interaction is refused | **unmapped** — the reason quotes the guard or names the operator |
| Message `createMessage`, `deleteMessage`; a message without a signal or operation | comment where the step would go; the steps around it are written | **unmapped** — a part exists for as long as its owner does |
| Message binding no argument to a parameter or signal attribute that must hold a value (no default, lower bound above zero), or one whose argument is not written | comment: the whole interaction is refused | **unmapped** — the reason names the parameter or attribute |
| DurationConstraint on a message, or on the occurrences of two messages of a scenario | a wait before the message's step, `accept after lo [SI::s]` or `accept after RandomFunctions::uniform(lo, hi) [SI::s]` as for an action; between two messages, a wait before the later step when they are adjacent, else `fork`ed after the earlier step and `join`ed before the later one, so the steps between count toward the interval | approximated (a v2 send arrives at once; a tool's duration mode is a run setting) |
| DurationConstraint between two messages of a scenario whose steps lie in different fragments (one in an `alt` operand, the other outside it) | comment before the later step | **unmapped** — a wait forked in one fragment cannot be joined in another |
| Interaction with no message | comment naming what it records (state invariants under time constraints: a timing trace); DurationConstraint, TimeConstraint, observation on an interaction | **unmapped** — no scenario step performs it |
| OpaqueBehavior, FunctionBehavior | `calc def` with its parameters when its one body is a v2 expression whose names resolve or a JavaScript expression of the [subset](#the-opaque-language-subset) (`Math.max(a, b)` → `RealFunctions::max(a, b)`) of the type of its one return or output parameter — a behavior with several has no one result and is written as an `action def`; an `action def` whose body is the translated `assign` sequence when the script is statements; otherwise `action def` keeping the body as a comment and the report naming the token refused | mapped / approximated |
| Operation | `action def <Op>` owned by the owner, with its parameters; the `method` behavior is written as its body (an Activity as the flow, an OpaqueBehavior as expression or comment), its parameters standing for the operation's at the same position, direction and type under the operation's names; a method parameter matching none is declared and reported, since a call binds only the operation's; no method: `abstract action def`; an `action <op> : <Op>;` usage of the owner performs it, as a call on an object does | mapped |
| Operation `precondition`, `postcondition`, `bodyCondition` | `assert constraint { <expr> }` in the action def when the expression parses and resolves; otherwise a comment | mapped / approximated |
| Reception with a `signal` and an Activity `method` | `action def <Sig> { action receive accept sig : Sig; action run : <Method> { in p = sig.p; } first run then receive; }` on the `part def`, plus `perform action sig : <Sig>;`, so every object of the block runs it from creation and accepts the signal again after each: the signal's attributes bind the method's `in` parameters of the same name whose type they conform to and whose multiplicity holds theirs, defaulted and optional parameters stay unbound; a parameter that must hold a value no attribute supplies, or whose type or multiplicity the same-named attribute does not fit, leaves the method unrun, with the reason. Where the signal arrives at ports of the block over the document's connectors or declarations, a `fork` after `start` adds one such loop per port, `accept … : Sig via <port>;` | mapped (a required parameter unsupplied, or an attribute not fitting its parameter: approximated, the signal is only accepted) |
| Reception without a method, or whose method is not an Activity | the same performed `action def`, accepting the signal and accepting again; the method is named in the report | approximated |
| Reception whose signal is not written | comment | **unmapped** — the reason names the signal |
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
script is read statement by statement through the [opaque-language subset](#the-opaque-language-subset):
`i = 1; GS_Found = false;` becomes two `assign` statements, `i += 1` an
`assign this.tcs.i := this.tcs.i + 1;`, and the body is kept as a comment naming its language
and the token refused when any statement is outside the subset or names something unwritten.

**Swimlanes.** An `ActivityPartition` that `represents` a property of the activity's context
block names the object whose features the nodes inside it read and write: a body `i = 1` in
the partition of the part `tcs` is `assign this.tcs.i := 1;`, and a guard `GS_Found` on an
edge whose source sits in that partition is `if this.tcs.GS_Found`. Names are looked up among
the node's own pins first — the tool binds a pin as a script variable, so a pin `Retries` on a
node in the partition is the value flowing into that node, not `this.tcs.Retries` — then in the
represented object, then among the activity's own parameters and locals, then in the owning
block; a nested partition reads through its enclosing ones (`this.tank.valve.open`), a
partition representing the context block itself reads `this`, and one representing a classifier,
or a property of one, that the context holds only through a chain of composite parts reads
through the whole chain, however long (`this.site.control.rack.controller.status`), when exactly
one such chain exists. A body's explicit `this` is the same object: the tool runs a node in a
partition in the represented object's context, so `this.status = true` there is the part's
`status`, and a feature the part lacks is refused (`Tank has no feature level`) rather than read
from the context block — a node that needs the block's own features sits outside the partition
or in one representing the block. A node in no partition, and a
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
performing it, and its report line says so. A part whose multiplicity is not written in numbers
(`banks : Bank[1..n]`) may hold one object or several, and the migrator cannot tell which: a
name read through it, a partition representing it, and a partition whose object is reached
through it are refused with the part named, never read as one object. The partition's comment
stays as documentation of its membership; its verdict is
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
execution enters it or the instant it exits, as UML defines; UML gives the omitted flag no
default, and the span then covers both nodes whole, from the first's start to the second's
end; a node executed again in a loop
stamps again, so the attribute holds the span between the latest executions of the two
nodes). An initial node is the point the activity's `start` reaches, so an observation
beginning there is stamped right after `start`, before the activity's own wait and its first
nodes, and spans the run when it ends at the final node; a flow final is the point a token
ends, so an observation ending there is stamped as the token reaches it, before `done` (the
edges into it lead to the stamp, through a `merge` when there are several); only an action has
an end of its own, so a flag asking for the end of an initial, final, flow final or control node
is refused with the node named. Both attributes are `[0..1]` with no default, and the elapsed
clock is assigned only when the stamp has happened, so a run that reaches neither node, or only
one, leaves the attribute without a value — a blank cell in the `-observe` table, outside the
summary — rather than a duration of zero; observations whose events are not nodes of the
activity, whose `event` list names an element the document does not define (the observation
is refused whole, never read as the one event that does resolve), and observations owned
outside any activity, are comments whose report line says which.

**The object an activity acts on.** A block's own activity acts on the block's object, `this`.
An activity no block owns, or one whose sends, accepts and calls all go through the ports of
another block, acts in v1 on whichever object ran it; it is written with a reference parameter
for that object, `in ref context : Host;`, its ports read `context.tx`, and every call of it
binds the parameter, `bind hit.context = this;` from that block's behaviors or `= context` from
another such activity. The block is the one whose ports the activity or the behaviors it calls
name; activities calling each other in a cycle name the ports of the whole cycle and take the
same block. An activity naming ports of several blocks none of which specializes the others
takes no parameter, and the report says which blocks; an activity naming none accepts through
the ports the signals it waits for arrive at, on the blocks whose behaviors run it.

An action whose input pin must hold a value (`lower` of 1 or more) but which only flows from
parameters nothing values, or from object flows that trace back to no pin or parameter at all (a
buffer nothing fills, an expansion node whose collection is not expanded), can never fire — the
token would wait forever at it — so it is written and reported as approximated with the pin that
starves it, no succession reaches or leaves it, and the report on the activity says
which of its parameters the caller has to value. A call whose target pin is fed from a part of
the context block, or from the activity's `context` parameter, performs the callee on that
object, `perform action x ::> drive.motor.spin;`.

**Values that never arrive.** v1 lets a call or a signal send fire holding no value for a
parameter or attribute that must have one; v2 does not admit a typed perform or `send new
Sig(x)` with an input unbound, so such a step keeps its place in the flow but performs nothing:
it is written as an empty action carrying the token, with the reason in its comment and in the
report. The reasons are the ones the model itself decides: the call passes no argument for a
required parameter (one with no default and a lower bound above zero; `out` and `return`
parameters and the operation's target pin are not arguments), the send passes no argument for a
required attribute of its signal, inherited ones included, or the pin it passes is fed only
by flows no value travels — from a parameter nothing values, from an action that is not
migrated (an opaque action's result, or a value specification action whose literal is no value
of its result's type), from a call whose callee gives that `out` parameter no value, judged
by the same analysis of the callee's own activity, through any depth of nesting, or from a
call's result pin past the callee's `out` parameters, which stands for none. Every such object flow is kept as a comment naming its source, never written as a
`flow` from a feature that will hold nothing, and the receiving action's report line says which
input receives no value. A call whose callee acts on an object the caller does not hold — the
method reads ports of its block, and the caller is a behavior of another block with no part of
that type — is refused the same way, since running it on the caller's object would go through
ports it lacks.

**Control nodes carrying data.** A fork, join, merge, decision or buffer node that lies on no
control path and whose every outgoing edge leads to an action's pin routes values, not control:
the flows through it are written from their sources to the pins it leads to, and the node
itself is reported as routing data only. A control node no edge leaves ends the token that reaches it, as
`done` does — through the stamp or wait a DurationObservation or DurationConstraint places on it, as a
flow final does — and one no edge reaches is skipped as a node nothing refers to. An action whose
input is fed by an object flow from an action outside its control path waits for the value as
well as for the control flow — a `join` of the two — but only when the producer runs on every
pass of the surrounding loop; a producer a later pass can skip, through a decision or a guarded
edge, is not waited on, since the wait would starve the consumer where v1 would go on with the
value the last pass left.

**Durations and probabilities.** A `DurationConstraint` on an action is a wait the action's
token takes before it: `accept after 3.0 [SI::s]` for a point interval, and
`accept after RandomFunctions::uniform(1.0, 80.0) [SI::s]` for a proper one — a draw from
the [model seed](../guide/06-behavior.md#seeds-where-the-draws-come-from). A simulation
tool's `min`/`max`/`average`/`random` duration mode belongs to its run configuration, not to
the model, so the interval is migrated faithfully as a random duration and the mode is the
[draw policy](../guide/06-behavior.md#draw-policies-min-max-average-and-random) of the run —
`-draws random -seed <n>` reproduces the tool's random mode, `-draws max` its max mode — which
each migrated configuration records (below). An interval open on one side — `{5s..}`, a max of
`*` — is satisfied by every wait past its bound, so no one delay stands for it and the
constraint is reported with the bound it lacks; the exception is a MagicDraw document, where a
constraint written with one value, `{60s}`, is stored as that min beside a max that is a
duration with no expression, and is written as the fixed wait it shows. «Probability» on the edges out of a decision is
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
with the model seed; the report says so. A guard in English that the subset reads — `TRUE`, a
Boolean property's name, `not Found and i < Retries` — is written as the `if` it means.

**State machines.** A composite state's regions become sub-states of a `parallel` state, so
the orthogonal regions run together — a region holding no vertex is skipped as content
nothing enters, so a machine whose one other region is populated is written inline and its
paths hold no parallel state; a submachine state is a `state` usage typed by the
referenced machine's `state def`, composing through any depth. Triggers are written on the
transition that refers to them — `accept Sig`, `accept after 2.0 [SI::s]`,
`accept when this.temperature > 200.0`, `accept at dawn` for an absolute time the `state def`
holds as a `Time::TimeInstantValue` attribute — and the event's own report line says where. An
event no trigger refers to is not a gap in the migration: nothing would ever accept it, so it
is skipped and the summary counts it apart from profile content. An effect with parameters
reads the accepted signal: the accept names it, `accept sig : Sig`, and the parameters the
signal fits are bound to that name. Entry, do and exit behaviors owned by the state are inline
action bodies, on a submachine state as on any other; those it only refers to are `entry x;`
references.

A state whose entry or do behavior takes parameters is entered by transitions that carry no
arguments, so the parameters are valued from the signal those transitions accept when every
transition into the state accepts the same signal and its attributes match the parameters in
order, type and multiplicity — the signal's own attributes first, then those it inherits from
its generals: the `state def` declares an item of the signal's type,
`item setPoint : SetPoint;`, each transition into the state assigns what it accepted to it,
`accept setPoint2 : SetPoint … assign setPoint := setPoint2;`, and the behavior's parameters
read its attributes, `in target : ScalarValues::Real = setPoint.level;` inline, or
`entry action : Handle { in level = setPoint.level; }` where the state refers to a behavior
written elsewhere, an `inout` parameter bound as `inout` so its value is written back. A state some
transition enters without a signal — from the initial pseudostate, on a time or change event,
or carrying a different signal — or whose parameters the signal's attributes do not fit, keeps
the parameters unvalued and the report says which transition or attribute is the reason; an
exit behavior with parameters is refused the same way, since nothing of the exit carries a
signal. A referred-to behavior whose parameter must hold a value is then not run, as a call
passing no argument for such a parameter is not.

A trigger naming a port of the behavior's owner is `accept Sig via rx`; one naming a port of
another block is written without it and the report says whose port it is. A trigger naming no
port is written plain, and it is also written accepting via each port of the owner the signal
arrives at, so a message a connector delivers to the port is taken as one addressed to the
object is. A signal arrives at a port when the document sends it through a port the connectors
and delegations join to it, when an item flow a connector realizes conveys it there, or when the
port's type — its generals and the interfaces it realizes included — declares a flow property of
the signal's type flowing in (out on a conjugated port) or a reception of the signal; a special
of a declared type arrives as well. A port that is untyped, or whose type declares neither flow
property nor reception, says nothing about what reaches it, so a signal nothing sends there is
not accepted via it and the report names the port left unrouted.

A transition whose ends lie in different regions or nesting levels names the far end by its
path — `transition first Idle accept Resume then Work::Run;` — which the runtime executes as
the compound transition v1 meant, exiting and entering the enclosing states along the way; a
local transition from a composite state into its own substate has no v2 form that stays inside
the state, so it is written external and reported as running the exit and entry behaviors. The
pseudostates are written as the v2 nodes of the same name: `junction`/`choice` for the guarded
chains, `fork`/`join` to enter and leave the regions of an orthogonal state, `history`/`deep
history` to re-enter what was active when the state was last left. An entry or exit point of a
state machine is a `state` of its `state def` whose own transition continues into the machine,
and a submachine state's connection point references address them by path,
`then Cell::warmStart;` / `first Cell::spent then Idle;`. An entry or exit point of a composite
state (UML `State.connectionPoint`) is a `junction` of that state — its transient node, so a
transition in from outside, `then Work::start;`, runs the state's entry behavior, then the
junction's own transition and the target's entries in the same step, and a transition out,
`first Work::leave then Idle;`, runs the inner transition's exits and effect, the state's exit
behavior, then the outgoing effect and the target's entry — the order UML 2.5.1 §14.2.3.4.5 and
PSSM give connection points. An entry point whose transitions each start a region of an
orthogonal state is a `fork`, an exit point its regions reach from each side a `join`; an entry
point no transition leaves is the state's default entry, and the transition is written to the
state. An entry point that leads straight to an exit point of the same state, so the state is
crossed without settling in it, is refused: the runtime would run neither its entry nor its exit
behavior; so is an entry point whose transition leads back to the state itself (v1 enters it by
its default entry where the runtime would leave and re-enter it), out of the state, on into a
history pseudostate or to no target, one whose transition has a trigger (a junction's transition
is followed at once, not on an event), and any point whose transitions do not form one of the
shapes above. A guard on that transition is kept: UML evaluates a junction's guards with the
rest of the compound transition's before it fires, not after entering the state, and the
runtime evaluates the junction's guard when it selects the transition; where UML leaves the
compound transition disabled by a false guard, the runtime reports it, as at any junction.
A connection point a tool lists among a region's vertices rather than as the state's
`connectionPoint` is still the state's, and is named through the state, not the region. An
internal transition is a self transition, faithful when re-entering the state is not observable (no entry, exit, do or
substates) and reported otherwise; one written with no target stays in its source, one that
targets another vertex or leaves a pseudostate is refused, and one without a trigger is a
comment, as a self transition would fire again on every re-entry. A transition into an exit point of a
region, or a terminate pseudostate, is written to `done`.

**Interactions.** An interaction owned by a block is a scenario: an `action def` of the block
whose steps are the messages in the order their occurrences take on the lifelines. Each
lifeline is resolved to a feature path from the block through its parts, ports and references
and their types — `drive.motor` — or to an `in` parameter of the interaction (two parts of one
type are two paths, `left.motor` and `right.motor`, so a lifeline standing for their shared
`motor` is ambiguous), and a lifeline that resolves to nothing, to two paths, to an `out`
parameter or through a `selector` refuses the whole interaction, since the scenario could not
address its steps. A signal message is
`send new Sig(n = 3) to this.drive.motor;`; a call message is a typed perform of the
operation's usage on the object, `perform action spin : Motor::Spin ::> drive.motor.spin
{ in rpm = 30.0; }`, its arguments bound to the operation's `in` and `inout` parameters by
name or by position, each with the parameter's direction so an `inout` value is written back
to what the argument named, and a call that leaves a required parameter (no default, lower bound above zero)
unbound refuses the interaction; a reply answers the latest call of its operation between
its lifelines that no earlier reply has answered, so nested calls pair with their replies
stack-like, and assigns that call's `out` to the attribute of the caller's lifeline the reply
names when the reply lies in the call's fragment or one nested in it. The operands of an `alt`,
`opt` or `loop` are alternative paths, so each may answer a call made before the fragment, and a
call answered on any of those paths (or made on only some of them) is open to no reply after the
fragment; the operands of a `par` are unordered between themselves, so none answers a call
another makes, while the calls they make are open after the join. Combined fragments become the
corresponding action structure when their guards are v2
expressions whose names resolve — `if`/`else` for `alt` and `opt`, `for`/`while` for `loop`,
`fork`/`join` for `par` — and refuse the interaction, quoting the guard, when they are not.
A duration constraint on a message is a wait before its step, as on an action; one between
two messages measures the interval from the earlier step to the later one, so it is a wait
before the later step when nothing lies between them and otherwise a wait forked after the
earlier step and joined before the later — the steps between count toward the interval, and
the bound is drawn once at the earlier step. Two steps in different fragments cannot share a
fork and join, so such a constraint is reported instead.
Create and delete messages are comments where the step would go, since a part exists for as
long as its owner does; the steps around them are written. An interaction with no message is
not a scenario: the report names what it records (state invariants under time constraints are
a timing trace). An interaction that is a `TestCase` is a `verification def` whose subject is
the block, the steps addressing the parts through it.

**Receptions.** A block's reception is an `action def` of the block that accepts its signal,
`action receive accept setLevel : Signals::SetLevel;`, and runs the method as a nested typed
action whose `in` parameters read the accepted signal's attributes of the same name,
`action run : 'Apply Level' { in value = setLevel.value; }`, then returns to the accept,
`first run then receive;`; a method that is also the method of an operation of the block is
written once, as that operation's body, so the reception runs the operation's `action def`,
binding the parameters it declares. The block performs it, `perform action setLevel : SetLevel;`, so
every object of the block listens from the moment it is created — nothing starts the reception —
and a signal sent to the object at any time is accepted and its method runs against the object,
not the signal, as many times as the signal arrives. The runtime keeps a message delivered to a
port apart from one addressed to the object, so where the document's connectors or port
declarations bring the signal to ports of the block, the accept is forked: after `start` a
`fork spread;` leads to the accept from the object and to one accept per port,
`action 'receive via rx' accept 'setLevel via rx' : Signals::SetLevel via rx;`, each running the
method and returning to its own accept, so a signal sent to the object or through any of those
ports runs the method; a port nothing declares or sends the signal to is named in the report as
not accepting it, as for a trigger. A reception without a method, or with one
that is not an Activity, accepts the signal and listens again, and the report names the method
it does not run; so does one whose method has an `in` parameter with no default and a lower bound
above zero that no attribute of the signal supplies, since v2 does not run an action holding no
value for it, and one whose same-named attribute is typed by a type that does not conform to the
parameter's (a `String` attribute for an `Integer` parameter) or whose multiplicity does not lie
within the parameter's (`[0..*]` for `[1]`), since binding it would violate the parameter.

**Operation calls over ports.** A `CallOperationAction` with `onPort` is resolved the way the
connector paths are: a connector of the caller's block from that port to a port of a part
whose type owns the operation makes the call a perform of the part's usage,
`perform action 'spin over p' ::> motor.spin;`; a port of the target itself names the target;
a port no connector joins, or one whose connectors reach several parts that own the operation
(the call names no one of them), leaves the call an action typed by the operation, running in
the caller's context, and the report says so.

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

**Scripts** (`language` JavaScript, ECMAScript, Java, or none) are read as statements. A
JavaScript label is `JavaScript`, `ECMAScript` or `JS`, or its engine (`Rhino`, `Nashorn`), in
any combination with at most one version (`Javascript Rhino`, `JavaScript (Nashorn)`,
`ECMAScript 2015`, `JavaScript 1.8 Rhino`); a Java label is `Java` alone or followed by one
version (`Java 8`, `Java 1.8.0_202`, `Java 17.0.2+8`). Any other word makes the label a
language the translator does not read: `JavaCC`, `Java Expression Language`, `JavaScript
Expression Language`, `ECMAScript for XML`, `JSON`:

| Script | v2 |
|---|---|
| `x = e;` `x += e;` `-=` `*=` `/=` `x++` `x--` | `assign x := e;` `assign x := x + e;` … |
| `var x = e;` `let x = e;` `const x = e;` (one name, initialized) | `attribute x : ScalarValues::T;` `assign x := e;` with `T` the type of `e`; a later assignment to a `const` is refused, as is a declaration of a name already declared, of a pin, parameter or property visible where the body lands, or of a member every action has (`start`, `done`, `self`) |
| several statements, on `;` or newlines | a sequence of the above |
| integer, real, Boolean and string literals | the same literal; a whole number is refused beyond what an `Integer` holds (2⁶³ − 1), and in a JavaScript body beyond 2⁵³ − 1, since the script would round it to a `Number` (a Java body's `long` is exact); a string's `\n` `\t` `\r` `\b` `\f` `\\` `\'` `\"` `\xHH` `\uHHHH` `\u{H…}` escapes and line continuations are decoded, a high and low surrogate escape pair as the one character they spell, while a legacy octal escape or a character the notation cannot spell (`\0`, `\v`, other control characters, a lone surrogate) is refused |
| `a`, `a.b.c` naming features that resolve | `this.a`, `this.a.b.c` (through the swimlane's object when it has one) |
| `+ - * / %`, comparisons, `&& \|\| !`, parentheses | `+ - * / %`, comparisons, `and or not`, parentheses; a Java body's `/` of two whole numbers drops the remainder, so it is `OpenSysMLMathFunctions::quotient(x, y)` (the exact Integer quotient truncated toward zero, refused at run time only for the least Integer by `-1`, whose quotient no Integer holds), and is refused when the operands' types cannot tell whether both are whole. Whole-number arithmetic is the exact arithmetic of a v2 `Integer`: a script that rounds a result beyond 2⁵³ to a `Number`, or a Java `int`/`long` that wraps past its range, computes something else there, which the translation does not reproduce — the translated feature holds the modeler's `Integer`, not a floating-point or fixed-width number |
| Java's `a.equals(b)` / `"x".equals(b)` on strings | `a == b`, the comparison of their content; a Java body's `==` or `!=` with an operand known to be a string is refused, since Java compares strings there by identity, which no comparison of their values reproduces, and `equals` is refused where a side is known not to be a string or neither side's type is known (a script's `==` on strings compares their content and translates as it stands) |
| `c ? a : b` | `if c ? a else b` when `a` and `b` are of one scalar type |
| `Math.min` `Math.max` `Math.abs` `Math.floor` `Math.ceil` `Math.round` `Math.sqrt` `Math.pow`, `a ** b` | `RealFunctions::min` … `RealFunctions::sqrt`, `**`; `Math.min` and `Math.max` take any number of arguments in a script, folded pairwise (`max(max(a, b), c)`; one argument is that argument, none is refused as the infinity the script answers), and exactly two in a Java body, as Java's do; `Math.ceil(x)` is `OpenSysMLMathFunctions::ceiling(x)` (the extension library's `Integer` ceiling, so the least Integer is a value where `-floor(-x)` would overflow on its negation) and `Math.round(x)` is `RealFunctions::floor(x + 0.5)`, which rounds a half toward +∞ as JavaScript does. The three answer the library's `Integer` in a script, and in a Java body `Math.round` does where `Math.floor` and `Math.ceil` answer a `Real` as Java's answer a `double` (so a Java `/` after them is real division, not `quotient`); each result is exact up to the `Integer` range and a whole Real at or beyond 2⁶³ (or below −2⁶³), which the script would keep as a `Number` and Java's `Math.round` would clamp to a `long`, is a typed arithmetic-overflow error at run time, never a wrapped Integer; `-a ** b` is refused, as JavaScript rejects a unary operand of `**` without parentheses, and a Java body's `**` is refused, Java having no such operator |
| `java.util.Collections.max(s)` / `.min(s)` | `RealFunctions::max(s)` / `RealFunctions::min(s)` over a collection |
| the tool's time variable (`simtime`) | `localClock.currentTime` |

**English** (`language` English, natural language, text) is read as one Boolean expression:
`TRUE` / `FALSE` / `true` / `false`; a property name, spaces and all; `not X`; `X and Y`;
`X or Y`; comparisons written with `=` or `==`, `<`, `>`, `<=`, `>=`, `!=`; parentheses. A run
of words between operators is one name (`not Guide Star Lost and i < Retries` reads the
property `Guide Star Lost`), refused whole when nothing visible is called that. English has no
calls: `Math.sqrt(t) > 3` or `name.equals(other)` in an English body is refused as a construct
outside the subset, never run as the script functions of the same name.

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
whose language the translator reads but whose text it refuses — as not that language's syntax
any more than as a construct, call, name or type it rejects — is never re-read as v2 syntax:
the refusal is final, and the body is a comment. A body in a language the translator does not
read, or in none, is read as v2 syntax.

A translation is emitted only when every name resolves to a written feature visible where the
statement lands, the types agree wherever they can be told (a guard is `Boolean`, an
assignment fits its target, a property's or parameter's default, the value of a typed pin or result, or the result
expression of a `calc def` is of the feature's or the return parameter's type — its scalar, or a
block or enumeration it is or specializes — and one value unless the feature holds several, a
duration is `Real`), and the result parses with the v2 parser.

## Run configurations

A simulation tool's run configuration — MagicDraw's «SimulationConfig», recognised by the
provenance of its profile (`magicdraw.com` or `nomagic.com`, at `/schemas/SimulationProfile.xmi` and no
other path; a stereotype so named from any other profile is kept as a comment), not by its name — states which
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
  naming no classifier — are migrated as individuals of the most special block their slots'
  features belong to, provided those blocks are one lineage ending in the configuration's target
  classifier or a general of it (a classifier-less instance anywhere else, or whose slots are of
  unrelated blocks, is unmapped with the reason), and indexed per configuration — a snapshot
  classified by the target's classifier, a general or a special of it, not one classified by a
  sibling special sharing only a general with it, which is of a run on another kind — in the JSON sidecar
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
  package — is of another configuration and left out with a note counting it. A configured
  value tells snapshots apart whatever its kind — a number, a Boolean, a string or an
  enumeration literal — though only the numbers are results; a number is compared exactly,
  however its literal spells it (`1` is `1.0`); a literal left blank configures nothing.
  Locations that
  repeat or nest (a package and a sub-package of it) index each snapshot once. A
  `resultLocation` outside the document, a snapshot slot with no defining feature in the
  document or holding no one finite number, a number the sidecar's float64 cannot spell
  exactly (an integer beyond 2^53, a decimal of more digits than a float64 keeps; the
  statistics are float64s, so it is noted rather than rounded), a feature two slots of one
  snapshot hold numbers
  for (left out of that snapshot whether or not a float64 spells each: it has no one result
  there, so it also records no other value than the target configures and does not put the
  snapshot out), and a target with no
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
has no one distribution to set beside the tool's and is noted, not pooled; so is one some completed
runs produce as no number, with the count of those runs. The numbers are printed
as they are: a difference is a fact about the migration's fidelity, to be read against the
report's approximations, not tuned away.

## The report

Nothing is dropped silently. Every element the reader saw is in the report exactly once with
one of four verdicts:

- **mapped** — a faithful v2 form.
- **approximated** — written, but not one-to-one; the note says what was lost or changed.
- **unmapped** — no v2 form was written. The element appears in the notation as a comment at
  the place it would have gone, so a reader of the migrated model can see the gap.
- **skipped** — profile and library content that is not part of the user's model, and the
  user's own elements nothing in the model refers to (an event no trigger names), which no v2
  form would represent; the summary counts the two apart.

The text form (default) groups by verdict, unmapped first, one line per element: kind with its
stereotypes, qualified v1 name, `xmi:id`, the v2 name it became, and a note. The JSON form
(`-migration-report x.json`) is the same content as `{source, exporter, entries: [{id, kind,
name, target, verdict, note}]}` for tooling. Without `-migration-report`, the one-line summary
goes to stderr. `-migration-results x.json` writes beside it the
[result snapshots](#run-configurations) of every run configuration, and reports what it wrote
in one line: `results of 3 run configuration(s): 2 with 15 stored snapshot(s)`.

## Guarantees

Every migrated model is gated in the test suite to:

1. parse and analyse clean under the v2 semantic passes (`go test ./internal/translate/migrate`),
2. round-trip through Turtle (notation → `.ttl` → notation → `.ttl`) without changing its graph,
3. account for every element in the report, and leave a comment for every unmapped one.

A model the reader cannot make sense of — not XMI, a zipped project container, a document
without a model — is refused with an error naming the reason rather than migrated partially.
