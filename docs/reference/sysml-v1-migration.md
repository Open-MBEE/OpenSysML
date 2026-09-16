# SysML v1 to v2 migration

## Status: experimental

The migration is **experimental**. It covers the structural, requirement, constraint, instance
and allocation content listed under [Mapping](#mapping), reports every element
it approximates or leaves behind, and refuses input it cannot read; behaviors, operations and
units are not migrated yet, and what a v1 element is written as may change between releases
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
| Property whose default is an InstanceSpecification of a block | the individual added to the usage's types, or its only type when the property is untyped; no `default` (a definition is not a v2 value). A port, or a usage of another kind than the individual, keeps its types and the default is a comment | approximated |
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
| `private` feature reached from outside by a connector, a slot, a redefinition, a subset or an expression | visibility dropped so the reference resolves; the report names the reacher | approximated |
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
| Activity, StateMachine, Interaction, OpaqueBehavior | comment placeholder | **unmapped** — behaviors come in a follow-up |
| Operation, Reception | comment placeholder | **unmapped** — v2 has no operation |
| «Unit», «QuantityKind» instance specifications | comment placeholder | **unmapped** — use the `SI`/`ISQ` libraries |
| Profiles, the SysML/UML libraries themselves | — | skipped |

The v1 element's `xmi:id` is kept as the reason a report line can be found in the source
model; the notation itself carries no IDs. Stable identity annotations for a re-migration are
future work (see [element identity annotations](../project/element-identity-annotations.md)).

Names that are not v2 identifiers — with spaces, punctuation, or starting with a digit — are
quoted (`'Vehicle Design'`).

The mapping has been run over the XMI of the [OpenMBEE TMT SysML model](https://github.com/Open-MBEE/TMT-SysML-Model)
(27 MB, 34,660 elements): it writes 6 MB of notation that passes the gate
below in about a second, and 150 MB of Turtle in four. Roughly half the elements map or are
approximated; the unmapped rest is dominated by behaviors (activities, signal and time events,
operations), instance specifications without a classifier, simulation verdicts stored in slots
of constraint properties, and views.

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
