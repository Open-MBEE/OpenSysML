# The Java client API

This page covers what `org.openmbee:opensysml` exposes, what it deliberately keeps
out of its public surface, and where it stops. To choose between the clients, see
[client libraries](clients.md); for a task-oriented walkthrough, see
the [Java client guide](../clients/java.md). The client's own notes on its
dependency footprint, service ownership and release verification are in
[client/java/README.md](../../client/java/README.md).

```xml
<dependency>
  <groupId>org.openmbee</groupId>
  <artifactId>opensysml</artifactId>
  <version>0.9.1</version>
</dependency>
```

The Java artifact is not on Maven Central yet. Until then, a checkout installs it:
`make build` for the service binary the tests start, then
`mvn -f client/java/pom.xml install`. The compiler
release is **17**, the lowest baseline a realistic host — Eclipse 2023-03,
IntelliJ 2023.2, Spring Boot 3 — can offer. The only compile-scope dependency is
`protobuf-java`; there is no gRPC and no Netty, because the transport is the JDK's
own `java.net.http.HttpClient` speaking the Connect protocol.

## Connection

```java
try (Connection connection = Connection.open()) {          // private child service
  Model model = connection.load(Path.of("model.sysml"));
  Model inline = connection.parse("package Demo { part def Car; }");
  Model adopted = connection.model(hashFromAnotherProcess);
}
```

| member | what it does |
| --- | --- |
| `open()` / `open(ConnectionOptions)` | connects, and starts a private service unless one was named |
| `load(Path)`, `load(Path, ParseOptions)` | parses a file the service can read |
| `parse(String)`, `parse(String, ParseOptions)` | parses inline content |
| `parseSources(List<SourceDocument>)`, `parseSources(List, ParseOptions)` | parses several documents as one model |
| `convert(String content, String toFormat[, ConversionOptions])`, `convertFile(Path, ...)` | translates source between notations (`sysml`, `kerml`, `ttl`, `api-json`); refuses a SysML v1 model, which is migrated |
| `migrate(byte[] content, String toFormat, MigrationOptions)`, `migrateFile(Path, String[, MigrationOptions])` | migrates a SysML v1 model (`xmi`, `uml`, `mdzip`) to v2, answering a `Migration` with its element-by-element `MigrationReport` |
| `model(String modelHash)` | adopts a model the service already holds |
| `capabilities()` | what `GetServerInfo` reported, asked once at open |
| `listEngines()` | the analysis engines the service can put a question to, as `EngineInfo` |
| `address()`, `ownsService()` | where this connection talks, and whether it started that service |
| `close()` | idempotent; releases a private child, only disconnects from an external one |
| `Connection.stopSharedServices()` | stops what this classloader still owns; returns how many |

`Connection` and `Model` are safe for concurrent use by many threads: one
`HttpClient` per connection, a single lock over the shared-service registry, and a
compare-and-set `close()`.

`ConnectionOptions.builder()` covers `service(host, port)`, `autoStart(false)` to
require a service someone else runs, `isolatedService(true)` for a child that is
not shared, `encoding(Encoding.JSON)` for bodies `curl` can read,
`requestTimeout`/`startupTimeout`, `requireCapabilities(...)` to refuse at open a
service that lacks what the caller needs, and the binary controls `binaryPath`,
`expectedBinarySha256`, `downloadVersion`, `githubRepo` and
`allowUnpinnedDownload`. Each has an environment form —
`OPENSYSML_SERVICE`, `OPENSYSML_GRPC_BINARY`, `OPENSYSML_GRPC_VERSION`,
`OPENSYSML_GITHUB_REPO`, `OPENSYSML_ALLOW_UNPINNED_DOWNLOAD` — named as constants
on `ConnectionOptions`. A connection asked for a release — `downloadVersion` or
`$OPENSYSML_GRPC_VERSION`, `"latest"` resolved once — refuses at open a service
that reports another version with `StaleServiceException`, whose `reason()` and
`remedy()` say what was found and how to reach the release asked for.

One private child is started **per classloader**, so an Eclipse plugin, a web
application and a copy shaded inside a third library each own one, while every
connection made through one copy shares a child and therefore its parse cache. It
cannot be orphaned: the client holds the write end of the child's stdin and never
writes to it, so the kernel closes that pipe however the JVM dies — `SIGKILL`
included — and the child exits at end of file.

## Model

```java
Model model = connection.load(Path.of("model.sysml"));
model.hash();                                   // what the service holds it under
model.parseDiagnostics();                       // from the parse that produced it
model.diagnostics();                            // asked of the service now
model.roots();                                  // one Symbol per document; empty for an adopted model

Symbol vehicle = model.symbol("Demo::Vehicle"); // throws if the model has no such symbol
model.findSymbol("Demo::Vehicle");              // Optional, for a name that may be absent
model.find("Vehicle");                          // Optional, by short name or qualified id
model.get("Demo::Vehicle");                     // Optional, by qualified id only
model.lookup("Vehicle");                        // throws SymbolNotFoundException with suggestions
model.contains("Vehicle");                      // whether find answers
model.documents();                              // the document names it was parsed from
model.ok();                                     // no error among parseDiagnostics()
model.errors();                                 // the error diagnostics
model.requireNoErrors();                        // the model itself, or a ModelException naming the errors

Value sum = model.eval("1 + 2 * 3");                        // Value.IntegerValue[value=7]
Value here = model.evalInContext("radius", "Demo::Wheel");  // resolved in a scope
Value mass = model.evalWithSubject("mass", "Demo::sedan");  // with a `self`
Instantiation built = model.instantiate("Demo::Vehicle");
```

`ParseOptions` is a record of `Language` (`SYSML` or `KERML`) and
`strictConformance`, with `defaults()` and `withLanguage`/`withStrictConformance`.

### Execution

```java
ActionRun run = model.executeAction("Test::addFive");                 // declared values as inputs
run.outputs().get("result");                                          // every attribute, when it stopped
ActionRun seeded = model.executeAction("Test::addFive",
    Map.of("result", new Value.IntegerValue(10)));
ActionRun ordered = model.executeAction("Test::race", Map.of(),
    ExecutionOptions.defaults().withSchedule("declared"));            // or "seed:7", "random"
StateRun machine = model.executeState("Test::Machine", List.of("go"));
machine.statesVisited();                                              // ["init", "Running", "done"]
machine.finalState();                                                 // Optional: the last of them

Exploration all = model.exploreAction("Test::race");                  // every schedule
all.complete();                                                       // false when a budget stopped it
for (Outcome outcome : all.outcomes()) {
  outcome.outputs(); outcome.witness(); outcome.linearizations();     // one order that reaches it
}
```

`executeAction`/`executeState` run once and answer an `ActionRun` (`outputs`,
`finalTime`, `diagnostics`) or a `StateRun` (`statesVisited`, `finalContext`,
`finalTime`, `diagnostics`). `finalTime` is filled only by a service advertising
`final_time`. `ExecutionOptions` carries a `schedule`, a `performer`, and the
state-run-only `withTrace()` option. The client checks their respective
capabilities (`schedule`, `performer`, `state_trace`) before the call; an unknown
schedule or trace with exploration is refused as `INVALID_ARGUMENT`. A requested
trace is returned as typed `DocumentEvent` values in `StateRun.trace()` with
`traceDropped()` reporting records discarded by the service's bound.
A failed traced run remains a `ModelException`; its `trace()` and
`traceDropped()` carry the records made before failure and the discarded count.
`exploreAction`/`exploreState` take an exploration schedule (`explore`,
`explore:runs=N,depth=M`; a non-exploring schedule is an `IllegalArgumentException`)
and answer an `Exploration` of `Outcome`s, each a distinct end state with the
`witness` order that reached it and how many `linearizations` do, plus whether the
search was `complete`, how many `runs` it made and which `budgetsHit`.

### Verification

```java
Verification v = model.verifyConstraint("Demo::Vehicle::massLight");   // over declared values
Verification about = model.verifyConstraint("Demo::Vehicle::massLight", "Demo::sedan");
Verification req = model.verifyRequirement("Demo::Vehicle::lightEnough");
Satisfaction all = model.verifySatisfaction();                          // every `assert satisfy`
Satisfaction scoped = model.verifySatisfaction("Demo::analysis");
Validation checked = model.validateInstance("Demo::sedan");             // every constraint of one object

v.holds();                       // false here: mass is 1500, the constraint asks < 100
v.verdict().decided();           // true — a false answer is an answer
v.verdict().condition();         // Optional: the condition that was false
v.verdict().error();             // Optional: why no answer could be reached, when decided() is false
v.verdict().failureReason();     // EVALUATION, WRONG_KIND, AMBIGUOUS_SUBJECT, …
```

A `Verdict` records `kind` (`constraint`, `requirement`, `satisfy`, …),
`elementId`/`element`, `holds`, the `condition` it evaluated, the instance it was
checked on (`instanceId`, `instanceTypeId`, `instancePath`), the `requirementId` a
satisfaction stands for, and the engine `standing` that produced it. **A false
verdict with no error is a decided answer, and is returned, not thrown**: the model
says the constraint does not hold. `decided()` is false only when `error` is filled
— the condition could not be evaluated, the symbol is of another kind, the subject
is ambiguous — and `failureReason` classifies which. `Verification` carries one
verdict; `Satisfaction` carries one per assertion, with `violated()` and
`undecided()` selecting among them; `Validation` carries a `summary` over one
object's constraints beside every `verdict`, and `bounded` says whether the check
ran out of budget. Each also carries the `instances` it materialised (to follow
`instanceId`), the `verifications` — a `VerificationVerdict` per verification-case
body that verifies the requirement, `PASS`/`FAIL`/`INCONCLUSIVE`/`ERROR` — and
`diagnostics`. A request that cannot be verified at all (no such symbol, a symbol of
another kind asked of `validateInstance`) is a `ModelException` whose
`failureReason()` says why.

### Calculation and analysis

```java
Calculation sum = model.evaluateCalc("Demo::add",
    List.of(new Value.IntegerValue(2), new Value.IntegerValue(3)));
sum.value();                     // Optional: the direct result, or the sole `out`
sum.outputs();                   // every named `out`, when there are several

Analysis study = model.runAnalysis("Trade::lightest");
study.holds();                   // every verdict the objective reached
study.outputs().get("selectedAlternative");
study.selected();                // Optional<CaseEvaluation>: the alternative the objective chose
study.evaluations();             // one CaseEvaluation per alternative: arguments, result, error, selected, tied
Analysis bound = model.runAnalysis("Trade::perOffset",
    AnalysisOptions.defaults()
        .withNamedArguments(Map.of("offset", new Value.IntegerValue(3)))
        .withSubject("Trade::sedan"));
```

`Calculation`, `Analysis` and every `Verdict` carry the `Standing` of the answer:
which `engine` answered (`run`, `explore`, `solve`, `sweep`), the `strength` of
its answer (`observed`, `witnessed`, `bounded`, `proved`, or `not covered`) and
the `bounds` it ran under, each `reached` or not. `model.withEngine("solve")` (or
`Standing.ENGINE_AUTO`/`ENGINE_ALL`) binds every question the model is asked to an
engine, and `connection.listEngines()` names what is available; both need the
`engines` capability. A run whose objective stops on an error throws an `AnalysisException`
carrying, in `partial()`, the outputs, verdicts, evaluations and instances the run
left behind — including the `CaseEvaluation` whose `error` stopped it — so a
caller can still show what was computed; a run that produced nothing throws the
plain `ModelException`.

### Query

```java
List<QueryElement> parts = model.query(
    Query.all()
        .withScope(List.of("Demo::vehicle"))                        // beneath these elements
        .withSelect(List.of("name", "owner"))                       // properties to read
        .where(Condition.all(List.of(
            Condition.equalTo("@type", List.of("PartUsage")),
            Condition.greater("mass", "1000").negated()))));
parts.get(0).id(); parts.get(0).type(); parts.get(0).properties();  // qualified name, metaclass, selected values
List<QueryElement> same = model.queryOslc("oslc.where=rdf:type=\"PartUsage\"&oslc.select=sysml:name");
```

`Condition` is sealed over `Comparison` (`equal`, `greater`, `less`, each
`negated()`) and `Combination` (`all`, `any`), so the query is a value a caller can
build and inspect. `queryOslc` needs the `oslc_query` capability; a scope naming
no element is refused as `INVALID_ARGUMENT`.

### Several documents, and conversion

`Connection.parseSources` reads a list of `SourceDocument`s — `file(Path)` for
what the service can read, `inline(name, content)` for what it cannot — as **one
model**, so the references between them resolve:

```java
Model model = connection.parseSources(List.of(
    SourceDocument.inline("library.sysml", librarySource),
    SourceDocument.inline("use.sysml", useSource)));
model.roots();   // one Symbol per document, in request order
```

`Connection.convert`/`convertFile` translate source text between notations and
answer a `Conversion` (`content`, the resolved `fromFormat`/`toFormat`,
`experimental`/`experimentalNotice`, `diagnostics`); `Model.convert` converts the
parsed model itself. `ConversionOptions` carries a `fromFormat` (else the service
sniffs it), `tolerateSyntaxErrors` — without it, a syntax error throws a
`ModelException` carrying the diagnostics rather than converting anyway — and
`idForm` (`ID_FORM_QUALIFIED`, the default, or `ID_FORM_UUID`) for the derived
element ids of a graph notation (`ttl`, `api-json`). `Conversion.write(Path)`
saves the content as UTF-8, `Conversion.formatOf(Path)` names a notation by its
extension, and `EditResult.save(Path)` writes a one-document edit and refuses one whose
`severalDocuments()` is set, whose text is in `documents()` alone; a model
adopted by hash first edits without reading documents, which the service refuses
for a model of several, and only then with them.

A SysML v1 model — UML XMI, an Eclipse UML2 `.uml` file or a `.mdzip` archive — is **migrated,
not converted**: `convert`/`convertFile` refuse it with a `ServiceException` of
`INVALID_ARGUMENT` whose message says so and names `migrate`, since a migration is ledgered
rather than lossless. `Connection.migrate(byte[] content, String toFormat, MigrationOptions)` and
`migrateFile(Path, String[, MigrationOptions])` answer a `Migration` (`content`, the canonical
`fromFormat`/`toFormat`, `experimentalNotice`, the `report`, the `results` index and the image
`files`). `MigrationReport` always carries the `summary` and the `mapped`, `approximated`,
`unmapped` and `skipped` counts; `MigrationOptions.withReport(true)` adds every element's
`MigrationEntry` (`byVerdict("unmapped")` selects them) and the `text` the `sysml
-migration-report` flag writes. The other options are the command's companion flags:
`withFromFormat` (inline content must name `xmi`, `uml` or `mdzip`), `withResults`,
`withLayoutFile`/`withLayoutContent` for an MTIP export, `withImageBaseUrl` and `withStrict`.
Inline content is `byte[]`, since a `.mdzip` archive is binary.

```java
Migration migration =
    connection.migrateFile(
        Path.of("Vehicle.mdzip"), "sysml", MigrationOptions.defaults().withReport(true));
Files.writeString(Path.of("Vehicle.sysml"), migration.content());
MigrationReport report = migration.report();
System.err.println(report.summary());
for (MigrationEntry left : report.byVerdict("unmapped")) {
  System.err.println(left.name() + ": " + left.note());
}
```

### Edits

```java
EditResult result = model.applyEdits(List.of(
    new Edit.SetValue("Demo::sc::unitMass", "1200.0[SI::kg]"),
    new Edit.Rename("Demo::sc", "subsystem"),
    Edit.AddMember.of("Demo::SC", "attribute", "margin")
        .withType("ISQ::MassValue"),
    new Edit.Delete("Demo::old", true),      // cascade the referred declarations
    new Edit.Move("Demo::helper", "Demo::Internal")),
    EditOptions.defaults().withAcceptDocuments(true));
result.content();        // the rewritten source of a one-document model
result.documents();      // EditedDocument per rewritten document, by parse name
result.applied();        // AppliedEdit per operation: the span it rewrote, old/new text
```

`EditOptions` `acceptDocuments` defaults to **true**: a batch that reaches
documents beyond the one the first edit touched is refused unless the caller
accepts a multi-document answer, and `document` limits the batch to one named
document. A batch the service refuses — an unknown target, a delete of an
element still referred to — throws `EditException`, a `ModelException` whose
`failure()` is an `EditFailure` (`UNKNOWN_TARGET`, `TARGET_REFERRED`, …) and
whose `referringElements()`/`referrers()` name the references that refused it.

`Model.edit()` returns an `Editor`, a fluent builder over the same `Edit`
records that is applied once:

```java
EditResult result = model.edit()
    .setValue("Demo::sc::unitMass", "1050.0[SI::kg]")
    .addPart("Demo::Vehicle", "engine", m -> m.withType("Engine"))
    .addRequireConstraint("Demo::R", "mass < 2000")
    .addDocumentation("Demo::Vehicle", "The vehicle under study.")
    .addIf("Demo::Drive", "speed > 0",
        new Editor.Body().addAssign("distance", "distance + speed"),
        new Editor.Body().addTerminate())
    .apply();
```

Its `add*` shorthands name the declaration kinds (`addPackage`, `addPartDef`,
`addAttribute`, `addActionDef`/`addCalcDef` with `Editor.Parameter`s,
`addConstraint`, `addRequirement`, `addState`, …), the relationships
(`addSatisfy`, `addVerify`, `addObjective`, `addTransition`, `addConnection`,
`addAllocation`, `addFlow`, `addSuccession`, `addImport`), the annotations
(`addMetadata`, `addMetadataPrefix`, `addDocumentation`, `addComment`, `addNote`)
and the action-body statements (`addFirst`, `addThen`, `addAccept`, `addSend`,
`addAssign`, `addIf`, `addWhile`, `addLoop`, `addFor`, `addTerminate`,
`addGuardedThen`, `addElse`); an `Editor.Body` collects nested statements, the
first written without `then`. `add(Edit)` takes any record, `edits()` shows the
batch, and each gated kind is checked against the capabilities before the call.

### Parameter sweeps

```java
Sweep sweep = model.runSweep("Sw::CostAnalysis",
    List.of(SweepRange.of("count",
        new Value.IntegerValue(1), new Value.IntegerValue(5)).withStep(new Value.IntegerValue(1))),
    SweepOptions.defaults().withSamples(0));            // or .withSamples(50).withSeed(7)
sweep.rows();                // one SweepRow per run: inputs, outputs, verdicts, elapsed
sweep.holds();               // every row's verdicts all held, no row failed
sweep.failures();            // the rows that carried an error
```

A `SweepRange` is `of(parameter, start, end)` plus an optional `withStep` — a
discrete grid the sweep runs once per value; `SweepOptions` carries `subject`,
`arguments`/`namedArguments`, and `samples`/`seed` for a sampled rather than
enumerated sweep (the answer's `sampled` and `seed` echo what ran). A run that
fails is a row carrying `error` and `failureReason` — a row is an answer, not an
exception — while a sweep that cannot start at all throws a `ModelException`.

### Document queries and rendering

```java
DocumentQueryResult table = model.runDocumentQuery(
    "Observatory::SubsystemTable",
    Map.of("root", List.of(new DocumentValue.ElementRef("Observatory::telescope", "PartUsage"))));
table.columns();             // the declared columns
for (DocumentRow row : table.rows()) {
  row.element().elementId(); // the element the row is about
  row.cells();               // List<List<DocumentValue>>, one per column
}
RenderedDocument page = model.renderDocument("Observatory::MassReport");
page.markdown();             // the rendered notation
RenderedDocument html = model.renderDocument("Observatory::MassReport", DocumentForm.HTML);
html.html();                 // the same document as HTML
```

`model.renderView("Demo::Overview")` returns a typed `RenderedView` with
ordered nodes, edges, ports, source spans, table data and notes. Optional
geometry, style and canvas remain `Optional` when the view does not state them.
Use `RenderViewPorts.FULL` to include every declared port; the default is
`MINIMAL`. This call requires the `render_view` capability.

`DocumentValue` is sealed over `ElementRef`, `ObjectRef` (an `Instance` plus its
element ids), `Verdict`, `State`, `Event`, `LiteralValue` (wrapping a `Value`),
`Quantity`, `Range`, `QuantityRange`, `InstanceRef` and `Unit` — the kinds a
native document's cells and bindings speak. A `DocumentRow` carries its subject
whichever kind it is — `element()` for an element row, `verdict()`/
`state()`/`event()`/`object()` as `Optional`s for the typed rows — beside the
cells. `runDocumentQuery` needs the `document_query` capability and
`renderDocument` the `render_document` capability, and `render_document_html`
besides for `DocumentForm.HTML`; an unknown query or document
id is a `ServiceException` `NOT_FOUND`.

## Values and the rest of the domain

Every answer is immutable, and no generated protobuf message or builder appears in
the public API. `Value` is a **sealed** interface over records, so its variants are
closed and a caller can enumerate them exhaustively. The snippets here stay inside
the JDK 17 baseline, so they use type patterns rather than a pattern `switch`,
which JDK 17 offers only as a preview:

```java
String rendered;
if (value instanceof Value.IntegerValue v)              rendered = Long.toString(v.value());
else if (value instanceof Value.RealValue v)            rendered = Double.toString(v.value());
else if (value instanceof Value.ComplexValue v)         rendered = v.real() + " + " + v.imaginary() + "i";  // one value
else if (value instanceof Value.BooleanValue v)         rendered = Boolean.toString(v.value());
else if (value instanceof Value.StringValue v)          rendered = v.value();
else if (value instanceof Value.QuantityValue v)        rendered = v.quantity().toString();
else if (value instanceof Value.ArrayValue v)           rendered = v.dimensions() + v.elements().toString();
else if (value instanceof Value.VectorValue v)          rendered = v.components().toString();   // IntegerValue | RealValue
else if (value instanceof Value.VectorQuantityValue v)  rendered = v.components().toString();   // one Quantity each
else if (value instanceof Value.MeasurementRefValue v)  rendered = v.unit();                    // a bare unit and its reduction
else if (value instanceof Value.FunctionValue v)        rendered = v.calcId();                  // a calc held as a value; selfId() when read off an object
else if (value instanceof Value.SetValue v)             rendered = v.elements().toString();     // each member once, unordered
else if (value instanceof Value.TensorQuantityValue v)  rendered = v.dimensions() + v.components().toString();  // any rank, row-major
else if (value instanceof Value.MetaobjectValue v)      rendered = v.elementId();               // x meta KerML::Feature; metaclassId() is the element's own
else if (value instanceof Value.EnumerationValue v)     rendered = v.literal().name();          // literal().value() is the scalar a `high = 3` literal carries
else if (value instanceof Value.InstanceReference v)    rendered = "instance " + v.instanceId();
else if (value instanceof Value.Sequence v)             rendered = v.elements().toString();
else if (value instanceof Value.UndeterminedValue v)    rendered = "<undetermined>: " + v.reason();  // left open by the model
else if (value instanceof Value.InfinityValue v)        rendered = "*";       // unbounded
else if (value instanceof Value.NullValue v)            rendered = "null";    // evaluated, no value
else                                                    rendered = "unset";   // declared, never given one
```

On a host running JDK 21 or later the same variants are a pattern `switch` needing
no default, since the interface is sealed.

`Symbol` is a record of `id`, `name`, `kind`, `metadata`, `childIds`,
`attributes`, `typeFacts`, `multiplicity`, `specializations` and
`withheldLibraryAttributes`; children are followed by looking their ids up, which
keeps the record a value rather than a handle on a connection. `Instantiation`
carries the `root` instance, everything `reachable` from it and its `diagnostics`,
with `instance(long)` and `resolve(Value.InstanceReference)` to follow a reference.
`Diagnostic` is `severity`, `message` and an optional `Span` of file and 1-based
line/column pairs.

## Exceptions: unchecked, and one distinction that matters

Everything thrown is unchecked and descends from `OpenSysMLException`; `close()`
throws nothing.

| exception | what happened |
| --- | --- |
| `ServiceException` | the call was refused, carrying a `StatusCode` (`NOT_FOUND`, …) |
| `ModelNotFoundException` / `ModelFileNotFoundException` | a `ServiceException` `NOT_FOUND`: the service holds no model of that hash, or cannot read the named file |
| `SymbolNotFoundException` | a `ModelException` from `Model.lookup` naming the missing `name()` and near `suggestions()` |
| `StaleServiceException` | a `ServiceStartException`: the service is not the release asked for |
| `ModelException` | the call succeeded and the answer reports a model failure; `failureReason()` classifies it and `diagnostics()` carry what the service said |
| `ConversionException` | a `ModelException` from a conversion the service could not write; its `diagnostics()` say why when the source did not parse |
| `MigrationException` | a `ModelException` from a SysML v1 model the service could not migrate at all; an element it has no v2 form for is reported in the `MigrationReport`, not thrown |
| `AnalysisException` | a `ModelException` from `runAnalysis` whose `partial()` holds what the run computed before it stopped |
| `EditException` | a `ModelException` from `applyEdits` carrying the `EditFailure` kind and the `referrers` a refused edit named |
| `TransportException` | HTTP or IO failure; the service was not reached or answered |
| `CapabilityException` | the service does not advertise a capability the call needs; `remedy()` says how to reach one that does |
| `ServiceStartException` | no binary, a digest mismatch, or a child that would not start |
| `ChecksumMismatchException` | a binary's bytes are not the digest required of them |
| `UnpinnedReleaseException` / `UnsignedReleaseException` / `ManifestSignatureException` | nothing pins the release, nothing signs it, or a signature does not verify |

The `ServiceException`/`ModelException` split is the one the conformance suite
draws too: an expression that will not evaluate is a successful call carrying an
error, not a service problem. A verdict that is *false* is neither: it is an answer,
and `verifyConstraint` returns it.

## Capability negotiation

`Connection.open` calls `GetServerInfo` once and keeps what it reported.
Negotiation is on the advertised **names** — the constants on `Capabilities`, such
as `EVALUATE_SUBJECT`, `FEATURE_VALUES`, `STRICT_CONFORMANCE`, `INLINE_LANGUAGE`,
`PARSE_SOURCES`, `DOCUMENT_QUERY`, `RENDER_DOCUMENT`, `RENDER_DOCUMENT_HTML` —
never on the version string:

```java
connection.capabilities().require(Capabilities.FEATURE_VALUES);
if (connection.capabilities().has(Capabilities.ENUM_VALUES)) { }
```

The client checks before a gated call rather than relying on the refusal, because
a capability that only describes how a response is *populated* omits its fields
instead of failing, and a call that relied on failure alone would read an answer
computed without them.

## The service binary

Resolution is `ConnectionOptions.binaryPath(...)`, then `$OPENSYSML_GRPC_BINARY`,
then `~/.opensysml/bin/sysml-grpc` — the cache the Python, Node and Rust clients
share, read and written in the same format — then `$PATH`.
`downloadVersion("v0.3.0")` or `"latest"` installs a release into that cache, and
**no version means no download**: without one the client only runs what is already
there. A download must match either the digest pinned in the jar's
`release-digests.json` or the release's sigstore-signed `SHA256SUMS.txt`, verified
against the release pipeline's own identity; a release with neither is refused
rather than trusted from the checksum served beside it.
[client/java/README.md](../../client/java/README.md) states the trust model, its
opt-out and its limitations in full.

## What the client does not do

Deliberately out of scope, rather than half-implemented: **generated
model-ergonomics types** — no code generation from a model into Java classes.
Every RPC the service serves is a public method now. The generated messages stay
what they are — `org.openmbee.opensysml.proto` carries every request and
response the service speaks, but no public call sends one directly: the
transport that would is `org.openmbee.opensysml.internal`, which is internal and
not a compatibility promise.

## Conformance

`opensysml-conformance` runs the language-neutral scenarios **through the public
API** and writes the report shape `tools/cmd/conformance` writes; `mvn -f
client/java/pom.xml test` is what CI runs. Of 138 scenarios, 133 run and pass over
both `connect` and `connect-json`, and 5 are skipped — the requests the public API
cannot express: a
`ParseFile` naming no source, a `Query` with both a structured and an OSLC query or
a comparison with no operator, and an `EvaluateCalc` argument the client's own
`Value` reader would refuse. gRPC is not run at all: this client does not speak it. `-mutate` corrupts
every answer before it is compared, and a test asserts each corruption is caught,
which is what keeps the run from being vacuous.
