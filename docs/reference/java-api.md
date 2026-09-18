# The Java client API

This page covers what `org.openmbee:opensysml-client` exposes, what it deliberately keeps
out of its public surface, and where it stops. To choose between the clients, see
[client libraries](clients.md); for a task-oriented walkthrough, see
[guide chapter 9](../guide/09-clients.md#from-java). The client's own notes on its
dependency footprint, service ownership and release verification are in
[clients/java/README.md](../../clients/java/README.md).

```xml
<dependency>
  <groupId>org.openmbee</groupId>
  <artifactId>opensysml-client</artifactId>
  <version>0.1.0-SNAPSHOT</version>
</dependency>
```

Nothing is published yet, so a checkout installs it: `make build` for the service
binary the tests start, then `mvn -f clients/java/pom.xml install`. The compiler
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
`requestTimeout`/`startupTimeout`, and the binary controls `binaryPath`,
`expectedBinarySha256`, `downloadVersion`, `githubRepo` and
`allowUnpinnedDownload`. Each has an environment form —
`OPENSYSML_SERVICE`, `OPENSYSML_GRPC_BINARY`, `OPENSYSML_GRPC_VERSION`,
`OPENSYSML_GITHUB_REPO`, `OPENSYSML_ALLOW_UNPINNED_DOWNLOAD` — named as constants
on `ConnectionOptions`.

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
model.root();                                   // Optional: absent for an adopted model

Symbol vehicle = model.symbol("Demo::Vehicle"); // throws if the model has no such symbol
model.findSymbol("Demo::Vehicle");              // Optional, for a name that may be absent

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
`final_time`. `ExecutionOptions` carries a `schedule` and a `performer`; the
capabilities they need (`schedule`, `performer`) are checked before the call, and a
schedule the service does not know is refused as `INVALID_ARGUMENT`.
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
            Condition.equal("@type", List.of("PartUsage")),
            Condition.greater("mass", "1000").negated()))));
parts.get(0).id(); parts.get(0).type(); parts.get(0).properties();  // qualified name, metaclass, selected values
List<QueryElement> same = model.queryOslc("oslc.where=rdf:type=\"PartUsage\"&oslc.select=sysml:name");
```

`Condition` is sealed over `Comparison` (`equal`, `greater`, `less`, each
`negated()`) and `Combination` (`all`, `any`), so the query is a value a caller can
build and inspect. `queryOslc` needs the `oslc_query` capability; a scope naming
no element is refused as `INVALID_ARGUMENT`.

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
| `ModelException` | the call succeeded and the answer reports a model failure; `failureReason()` classifies it and `diagnostics()` carry what the service said |
| `AnalysisException` | a `ModelException` from `runAnalysis` whose `partial()` holds what the run computed before it stopped |
| `TransportException` | HTTP or IO failure; the service was not reached or answered |
| `CapabilityException` | the service does not advertise a capability the call needs |
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
as `EVALUATE_SUBJECT`, `FEATURE_VALUES`, `STRICT_CONFORMANCE`, `INLINE_LANGUAGE` —
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
[clients/java/README.md](../../clients/java/README.md) states the trust model, its
opt-out and its limitations in full.

## What the client does not do

Deliberately out of scope, rather than half-implemented: the edit API
(`ApplyEdits`), models of several documents (`ParseSources`), RDF conversion
(`Convert`), parameter sweeps (`RunSweep`), native document queries and rendering
(`RunDocumentQuery`, `RenderDocument`), and generated model-ergonomics types. The
service still serves all of them, but the
public API offers no generic call: `org.openmbee.opensysml.proto` carries the request and
response messages — `ApplyEditsResponse.getDocumentsList()` carries the edited
notation of every document an edit rewrote, by parse name, beside the
sole-document `content` — and the transport that would send one is
`org.openmbee.opensysml.internal`, which is internal and not a compatibility promise. Reach
those RPCs from the Go or Python client until this one wraps them.

## Conformance

`opensysml-conformance` runs the language-neutral scenarios **through the public
API** and writes the report shape `cmd/conformance` writes; `mvn -f
clients/java/pom.xml test` is what CI runs. Of 134 scenarios, 94 run and pass over
both `connect` and `connect-json`, and 40 are skipped — the scenarios of the RPCs
the client does not cover, plus the requests the public API cannot express: a
`ParseFile` naming no source, a `Query` with both a structured and an OSLC query or
a comparison with no operator, and an `EvaluateCalc` argument the client's own
`Value` reader would refuse. gRPC is not run at all: this client does not speak it. `-mutate` corrupts
every answer before it is compared, and a test asserts each corruption is caught,
which is what keeps the run from being vacuous.
