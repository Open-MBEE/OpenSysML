# The Java client API

This page covers what `org.openmbee:opensysml-client` exposes, what it deliberately keeps
out of its public surface, and where its v1 stops. To choose between the clients, see
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
else if (value instanceof Value.BooleanValue v)         rendered = Boolean.toString(v.value());
else if (value instanceof Value.StringValue v)          rendered = v.value();
else if (value instanceof Value.QuantityValue v)        rendered = v.quantity().toString();
else if (value instanceof Value.ArrayValue v)           rendered = v.dimensions() + v.elements().toString();
else if (value instanceof Value.VectorValue v)          rendered = v.components().toString();   // IntegerValue | RealValue
else if (value instanceof Value.VectorQuantityValue v)  rendered = v.components().toString();   // one Quantity each
else if (value instanceof Value.MeasurementRefValue v)  rendered = v.unit();                    // a bare unit and its reduction
else if (value instanceof Value.FunctionValue v)        rendered = v.calcId();                  // a calc held as a value; selfId() when read off an object
else if (value instanceof Value.EnumerationValue v)     rendered = v.literal().name();
else if (value instanceof Value.InstanceReference v)    rendered = "instance " + v.instanceId();
else if (value instanceof Value.Sequence v)             rendered = v.elements().toString();
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
| `ModelException` | the call succeeded and the answer reports a model failure |
| `TransportException` | HTTP or IO failure; the service was not reached or answered |
| `CapabilityException` | the service does not advertise a capability the call needs |
| `ServiceStartException` | no binary, a digest mismatch, or a child that would not start |
| `ChecksumMismatchException` | a binary's bytes are not the digest required of them |
| `UnpinnedReleaseException` / `UnsignedReleaseException` / `ManifestSignatureException` | nothing pins the release, nothing signs it, or a signature does not verify |

The `ServiceException`/`ModelException` split is the one the conformance suite
draws too: an expression that will not evaluate is a successful call carrying an
error, not a service problem.

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

## What v1 does not do

Deliberately out of scope, rather than half-implemented: the edit API
(`ApplyEdits`), RDF conversion (`Convert`), the verification helpers
(`VerifyConstraint`, `VerifyRequirement`, `VerifySatisfaction`), behaviour
execution (`ExecuteAction`, `ExecuteState`), `EvaluateCalc`, `RunAnalysis`, `Query`/OSLC, and
generated model-ergonomics types. The service still serves all of them, but the
public API offers no generic call: `org.openmbee.opensysml.proto` carries the request and
response messages, and the transport that would send one is
`org.openmbee.opensysml.internal`, which is internal and not a compatibility promise. Reach
those RPCs from the Go or Python client until a v2 wraps them here.

## Conformance

`opensysml-conformance` runs the language-neutral scenarios **through the public
API** and writes the report shape `cmd/conformance` writes; `mvn -f
clients/java/pom.xml test` is what CI runs. Of 59 scenarios, 25 run and pass over
both `connect` and `connect-json`, and 34 are skipped — the scenarios of the RPCs
v1 does not cover, plus one the public API cannot express (a `ParseFile` naming no
source). gRPC is not run at all: this client does not speak it. `-mutate` corrupts
every answer before it is compared, and a test asserts each corruption is caught,
which is what keeps the run from being vacuous.
