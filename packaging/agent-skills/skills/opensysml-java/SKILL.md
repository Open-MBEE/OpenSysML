---
name: opensysml-java
description: Parse, query, instantiate, execute and verify SysML v2 models from Java (JDK 17+) with the `org.openmbee:opensysml` client, which drives the OpenSysML sysml-grpc service over Connect from inside a host JVM — requiring a clean model, reading sealed Value records, running actions and calcs, checking requirements, telling ServiceException from ModelException, stopping the shared service. Use when writing Java or other JVM code that reads or runs SysML v2 models.
---

# Using OpenSysML from Java

The Java client talks to the `sysml-grpc` service and starts a private one for you. It is built for
host applications it does not own (Eclipse tools, Cameo plugins, web services): JDK 17, and
`protobuf-java` as its only compile-scope dependency. Reference: `docs/guide/09-clients.md` (From
Java), `client/java/README.md` and `docs/reference/java-api.md` in the OpenSysML repository.

## Install and get the service binary

The artifact is not on Maven Central. Build it from an OpenSysML checkout into the local
repository, then depend on it:

```bash
make build                               # bin/sysml-grpc
mvn -f client/java/pom.xml install
```

```xml
<dependency>
  <groupId>org.openmbee</groupId>
  <artifactId>opensysml</artifactId>
  <version>0.12.1</version>  <!-- the version in client/java/pom.xml -->
</dependency>
```

Binary resolution is `ConnectionOptions.binaryPath(...)`, then `$OPENSYSML_GRPC_BINARY`, then
`~/.opensysml/bin/sysml-grpc` (shared with the other clients), then `PATH`. With none installed
the client downloads and verifies a release into that cache: the one
`ConnectionOptions.builder().downloadVersion(...)` or `$OPENSYSML_GRPC_VERSION` names, else the
release the client was built against. `$OPENSYSML_SERVICE=host:port`, or `ConnectionOptions.builder().service(host, port)`,
uses a running service instead; closing that connection never stops it.

## Load, and require a clean model

```java
import org.openmbee.opensysml.*;

try (Connection connection = Connection.open()) {      // private child, shared per classloader
  Model model = connection.load(Path.of("model.sysml"));
  for (Diagnostic d : model.errors()) {                // (severity, message, code, span)
    System.err.println(d.code() + ": " + d.message());
  }
  model.requireNoErrors();                             // ModelException naming the parse errors
}
```

A model with syntax errors still loads; branch on `d.code()` (`"syntax"`, `"unresolved"`, ...),
never on message text. `ParseOptions.defaults().withStrictConformance(true)` makes OpenSysML-only
notation an error. For several files use `connection.parseSources(List.of(...))` with
`SourceDocument`s; `load` reads one document, so imports across files do not resolve.
`connection.parse(String)` parses inline source.

## Evaluate, instantiate, run

```java
Value mass = model.eval("Demo::sedan::mass");                  // RealValue[value=1200.0]
Value same = model.evalWithSubject("mass", "Demo::sedan");

Instantiation built = model.instantiate("Demo::sedan");
built.root().featureValues().get("mass").value();              // Optional[RealValue[value=1200.0]]

ActionRun run = model.executeAction("Demo::addFive",
    Map.of("x", new Value.IntegerValue(10)));
run.outputs().get("result");                                   // IntegerValue[value=15]

Calculation calc = model.evaluateCalc("Demo::Margin",
    List.of(new Value.RealValue(1200), new Value.RealValue(2000)));
calc.result();                                                 // Optional[RealValue[value=800.0]]
```

`Value` is a sealed interface over records (`IntegerValue`, `BigIntegerValue`, `RealValue`,
`BooleanValue`, `StringValue`, `QuantityValue`, `EnumerationValue`, `InstanceReference`,
`Sequence`, ...), so a `switch` with record patterns is exhaustive. Three "no value" records are
not errors and must not be read as 0 or `false`: `UndeterminedValue` (the model leaves it open),
`UnsetValue` (an object holds nothing for the feature) and `NullValue`.

## Members, element properties and object paths

```java
Symbol m = model.symbol("Demo::m");
m.members();                      // every member, own then inherited: Symbol.Member(id, name, kind, inherited, inheritedFrom, library)
m.inheritedMembers();             // the inherited ones; inheritedFrom() names the declaring type
m.withheldLibraryMembers();       // standard-library members counted, not listed, until:
model.symbol("Demo::m", SymbolOptions.defaults().withLibraryMembers(true));

ElementProperties read = model.elementProperties("Demo::Pair", "inheritedFeature", "isAbstract");
read.get("inheritedFeature").orElseThrow().elements();       // ElementRefs; their id() is accepted back
read.get("connectorEnd").map(p -> p.unsupported());          // why a property has no values

Instance engine = built.reachable().get(1);
engine.reachedBy();               // Optional<Instance.Reach>: owner id, feature, index, path; empty for the root
built.owner(engine);              // the holding object
model.instancePaths(engine);      // InstancePaths: every root-relative path, aliases included
```

`members()` is empty from a service without `symbol_members` (`childIds()` still names the owned
ones); library members, `elementProperties` and `instancePaths` are refused with
`CapabilityException` before the call when the service lacks `symbol_members`,
`element_properties` or `instance_paths`. `annotatingMetadataIds()` names the metadata usages
annotating a symbol (`#M`, `@M` in its body, and `metadata m : M about X;` written elsewhere, which
`childIds()` leaves out); look each up with `model.symbol(id)`. Empty without
`symbol_annotating_metadata`.

## Verify

```java
Verification v = model.verifyRequirement("Demo::Vehicle::lightEnough", "Demo::truck");
v.holds();                         // false — an answer, not an exception
v.verdict().decided();             // true: the model answered; false only when evaluation failed
v.verdict().condition();           // Optional[mass < 2000.0]
v.verdict().error();               // set, with failureReason(), when undecided
model.verifySatisfaction().holds(); // every `assert satisfy R by x;`
```

`verifyConstraint` takes the same subject argument; `VerifyOptions` picks an engine or question.

## Errors

Everything thrown is unchecked and descends from `OpenSysMLException`:

- `ServiceException` (with a `StatusCode`) — the call was refused. `ModelFileNotFoundException`
  (an unreadable path) and `ModelNotFoundException` are subclasses.
- `ModelException` — the call was answered and reports a model failure (an expression that will
  not evaluate, `model.symbol` of an unknown name, `requireNoErrors`); `failureReason()` classifies
  it. Subclasses: `SymbolNotFoundException` from `model.lookup` (with near-name `suggestions()`),
  `AnalysisException` (with `partial()`), `EditException`, `ConversionException`.
- `TransportException`, `ServiceStartException`, `CapabilityException` (service too old; `remedy()`
  says what to install), `ChecksumMismatchException`.

## Pitfalls

- One child service per classloader. In a plugin or web app call
  `Connection.stopSharedServices()` from `stop()` / a `ServletContextListener`; unloading the
  classloader alone does not stop it. `isolatedService(true)` gives one connection its own child.
- `findSymbol(name)` returns `Optional`; `symbol(name)` and `lookup(name)` throw.
- Use qualified names (`Package::Def::feature`) wherever a symbol is named.
- Write models with the `sysml-v2-modeling` skill; `sysml -validate` reports the same diagnostics as
  `model.diagnostics()`.
