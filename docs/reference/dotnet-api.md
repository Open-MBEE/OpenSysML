# The .NET client API

This page lists what the `OpenMBEE.OpenSysML` assembly exposes and the rules it keeps. To choose
between the clients, see [client libraries](clients.md); for a task-oriented walkthrough, see the
[.NET client guide](../clients/dotnet.md). The client's own README,
[client/dotnet/README.md](../../client/dotnet/README.md), states every rule below in full, with
its build, test and conformance procedures.

The client is consumed from a checkout by `ProjectReference`, never from NuGet:

```xml
<ProjectReference Include="path/to/OpenSysML/client/dotnet/src/OpenMBEE.OpenSysML/OpenMBEE.OpenSysML.csproj" />
```

It targets **`net10.0`** alone, built with the SDK the repository-root `global.json` pins. Every
call is asynchronous, returns a `Task` and takes an optional `CancellationToken`; a cancelled call
throws `OperationCanceledException` carrying the caller's token. Results are immutable and safe
to share between threads. The assembly makes no `IsAotCompatible` or `IsTrimmable` claim.

## Connection

```csharp
using OpenMBEE.OpenSysML;

await using var connection = await Connection.OpenAsync(new ConnectionOptions
{
    Address = "localhost:50051",
    RequireCapabilities = [Capabilities.Query],
});
Console.WriteLine(connection.ServerInfo);          // sysml-grpc <version>, <count> capabilities
bool converts = connection.Capabilities.Has(Capabilities.Convert);
```

`Connection.OpenAsync` reaches the service `Address` names, else the one `OPENSYSML_SERVICE`
names, else starts a private one. It asks `GetServerInfo` once, keeps the answer as `ServerInfo`
and `Capabilities`, and checks `Version` and `RequireCapabilities` against it. `DisposeAsync`
refuses new calls, cancels those in flight and releases the connection's hold on a private
service; it is idempotent and never throws.

| `ConnectionOptions` | Meaning |
| --- | --- |
| `Address` | `host:port`, `[::1]:port` or an `http`/`https` URL with nothing after its port |
| `Protocol`, `Encoding` | `Grpc` (default) or `Connect`; `Protobuf` (default) or `Json`, which only `Connect` carries |
| `RequestTimeout` | Each call's deadline, 60 seconds by default |
| `Headers` | Sent with every call; protocol-controlled headers are refused and values are never logged |
| `Token` | The bearer token every call carries, else the one `OPENSYSML_GRPC_TOKEN` names unless `Headers` holds an `authorization`; refused beside one, and never logged |
| `MaxRequestSize`, `MaxResponseSize` | 512 MiB each by default, a response measured after decompression |
| `Logger` | An `ILogger`, `NullLogger` by default |
| `HttpMessageHandler` | A handler to borrow, never disposed by the connection |
| `Version`, `RequireCapabilities` | An exact service version (`StaleServiceException`) and required capabilities (`CapabilityException`) |
| `AutoStart` | Whether a connection naming no service starts a private one; true by default |
| `BinaryPath`, `ExpectedBinarySha256` | The private service's binary, and the digest it must have |
| `IsolatedService` | A private service of this connection's own rather than a shared one |
| `StartupTimeout`, `ShutdownTimeout` | 30 and 5 seconds by default |
| `DownloadVersion`, `GithubRepo` | The release to download, a tag or `latest`, and its repository |

The private-service options are refused alongside `Address` or with `AutoStart` false. Every
option is checked before anything is sent; an invalid one is an `InvalidRequestException`.
`ConnectionOptions` also names the environment variables as constants: `ServiceVariable`,
`BinaryVariable`, `VersionVariable`, `GithubRepoVariable`, `AllowUnpinnedDownloadVariable` and
`TokenVariable`. A private service is started without `OPENSYSML_GRPC_TOKEN`, and requires no
token; credentials sent over cleartext `http` to another machine are logged as a warning.

| `Connection` member | Answers |
| --- | --- |
| `LoadAsync(path)` | A `Model` of a file the service reads, by its path as given |
| `LoadContentAsync(content)` | A `Model` of inline notation, in `ParseOptions.Language` |
| `ParseSourcesAsync(documents)` | One `Model` of several `SourceDocument`s, files or named inline notation |
| `ModelByHash(hash)` | A `Model` the service already holds, without a call |
| `ConvertAsync`, `MigrateAsync` | A `Conversion` between notation, Turtle and API JSON; a `Migration` of a SysML v1 model |
| `ListEnginesAsync` | Each `EngineInfo` the service can answer with |
| `CallAsync<TRequest, TResponse>(method, request)` | Any RPC with the generated `OpenMBEE.OpenSysML.Wire` messages, unchecked by capability |
| `Protocol`, `Encoding`, `Address`, `ServerInfo`, `Capabilities` | What the connection speaks, where, and what the service reported |
| `StopSharedServicesAsync()` (static) | Stops every shared private service of this copy of the client |

`OpenSysMLClient.LoadAsync` and `LoadContentAsync` open a connection of their own, which the
returned model owns and closes when it is disposed.

## Model

A `Model` is the service's parse of its sources, named by `Hash`: its `Roots`, `Documents`,
`ParseDiagnostics` and their `Errors`. `HasErrors` says whether it has errors, and
`RequireNoErrors` raises `ModelErrorsException` when it does. A model loaded through a connection
borrows it. Each method below is one RPC:

| Area | Methods | Result |
| --- | --- | --- |
| Symbols | `SymbolAsync`, `FindSymbolAsync`, `FindAsync`, `LookupAsync` | `Symbol`, or null from the `Find` forms |
| Diagnostics | `GetDiagnosticsAsync` | `ValueList<Diagnostic>`, under a lint selection |
| Expressions | `EvalAsync`, `EvaluateAsync` | `Value`; `Evaluation` keeps its diagnostics |
| Objects | `InstantiateAsync` | `Instantiation` |
| Behavior | `ExecuteActionAsync`, `ExecuteStateAsync`, `ExploreActionAsync`, `ExploreStateAsync`, `CheckActionAsync`, `CheckStateAsync` | `ActionRun`, `StateRun`, `Exploration`, `CheckResult` |
| Verification | `VerifyConstraintAsync`, `VerifyRequirementAsync`, `VerifySatisfactionAsync`, `SatisfiedAsync`, `ValidateInstanceAsync` | `Verification`, `Satisfaction`, `bool`, `Validation` |
| Calculation and analysis | `CalcAsync`, `RunAnalysisAsync`, `ExploreAnalysisAsync`, `RunSweepAsync` | `Calculation`, `Analysis`, `Exploration`, `Sweep` |
| Repeated runs | `RunActionRunsAsync`, `RunAnalysisRunsAsync`, `SelfCheckAsync`, `RecordRunAsync` | `Runs`, `SelfCheck`, `Recording` |
| Conversion | `ConvertAsync`, `ToNotationAsync`, `ToTurtleAsync`, `ToApiJsonAsync`, `SaveAsync` | `Conversion` |
| Queries and documents | `QueryAsync`, `QueryOslcAsync`, `RunDocumentQueryAsync`, `RenderDocumentAsync`, `RenderViewAsync` | `QueryResult`, `DocumentQueryResult`, `RenderedDocument`, `RenderedView` |
| Editing | `Edit()`, `ApplyEditsAsync` | an `Editor`; `EditResult` |
| Graphs and data | `ExportGraphsAsync`, `ImportDataAsync`, `CompareResultsAsync` | `Graphs`, `ImportResult`, `Comparison` |

Each takes its own options record (`EvalOptions`, `RunOptions`, `CheckOptions`, `VerifyOptions`,
`AnalysisOptions`, `SweepOptions`, `ConvertOptions`, `EditOptions`, …), so a call accepts only
what its RPC can use; a combination its RPC cannot answer, such as an exploring schedule passed to
`ExecuteActionAsync`, is refused before anything is sent. A false verdict, a failed sweep row and
a failed run among repeated runs are answers, never exceptions.

A `Query` filters with `QueryConstraint`s (`Equal`, `In`, `Greater`, `GreaterOrEqual`, `Less`,
`LessOrEqual`, `InstanceOf`, and `And`/`Or` composites of at least two) and sorts by `OrderBy`,
which needs `query_order_by`. `RenderDocumentAsync` renders `DocumentForm.Markdown`, `Html` or
`AsciiDoc`. `RenderViewOptions.Compartments` (`RenderViewCompartments`) folds a tree's members into
rows, each a `ViewNode` naming its `Compartment`; `ViewNode.Inherited` and `ViewPort.Inherited` mark
what an owner has from its type. A solver's `Verdict.Notes` are the `VerdictNote`s its answer rests
on, and an `api-json` `Conversion` lists the `LibraryReference`s it refers to, which
`ConvertOptions.IncludeLibraryStubs` also writes as roots.

## Writing files

Nothing is written to disk until `Model.SaveAsync`, `Conversion.SaveAsync`,
`Migration.WriteAsync`, `EditResult.SaveAsync` or `EditedDocument.SaveAsync` is called. Each
writes a file whole through a temporary file beside it, never replaces an existing file unless
`SaveOptions.Overwrite` asks, and refuses an unsafe destination (a symbolic link, the v1 source
of a migration, a migration asset path that escapes its directory or is not a portable file name)
with `UnwritableException` before writing anything.

## Values

`Value` is an immutable sealed hierarchy with one nested class per arm of the schema's `Value`:
`IntegerValue`, `BigIntegerValue`, `RationalValue`, `RealValue`, `ComplexValue`, `BooleanValue`,
`TextValue`, `QuantityValue`, `VectorValue`, `VectorQuantityValue`, `TensorQuantityValue`,
`ArrayValue`, `SequenceValue`, `SetValue`, `EnumLiteralValue`, `InstanceRefValue`,
`MeasurementRefValue`, `FunctionValue`, `MetaobjectValue`, `NullValue`, `UnsetValue`,
`UndeterminedValue` and `InfinityValue`, named by `Kind`. The four numeric arms derive from
`Value.NumberValue`. A `switch` over the arms needs a default case, since a release adds an arm
whenever the service does.

Numbers never pass through a `double`: an Integer beyond `long` is a `BigIntegerValue`, and a
Rational is exact in lowest terms. Equality is by content, never the model's numeric comparison,
so `IntegerValue(1)` is not `RealValue(1.0)`. `ToString` writes a value as a reader would, in any
culture: `1800.0`, `1/3`, `5.4 [km/h]`, `{1, 2}`, `*`. Collections are `ValueList<T>` and
`ValueDictionary<TValue>`, compared by content.

## Typed model classes

`src/OpenMBEE.OpenSysML.Generate` is a console project of the checkout, run from the repository
root, that writes one C# file of typed views over a model's definitions:

```bash
dotnet run --project client/dotnet/src/OpenMBEE.OpenSysML.Generate -- SOURCE... --namespace NAMESPACE [-o OUTPUT] [--check] [--address ADDRESS | --binary PATH]
make dotnet-generate SOURCES='SOURCE ...' NAMESPACE=NAMESPACE [OUTPUT=OUTPUT] [CHECK=1]
```

A `SOURCE` is a `.sysml` or `.kerml` file, or a directory whose model files are read in path order,
hidden directories skipped; together they are parsed as one model. Without `-o` the file goes to
standard output. `--check` writes nothing and exits 1 unless `OUTPUT` is what the model generates;
a model with errors, one that cannot be generated soundly and a service without `type_facts` exit 1
too, before anything is written; misuse, filesystem failures, non-UTF-8 input and an invalid namespace
exit 2 with a diagnostic. An output path naming a directory is refused. The service is a connection's
default unless `--address` or `--binary` names one. Package children include declarations from every
input document declaring that package, including nested packages.

| Model | Generated |
| --- | --- |
| package `P` | namespace `NAMESPACE.P`; a C# keyword with `@` |
| definition `D` (not an enumeration) | `public interface ID` and `public class D : ID` |
| definition nested in `D` or a usage `u` | `D_Inner`, `u_Inner`, in the package's namespace |
| specialization of generated `A`, `B` | `ID : IA, IB`; `class D : A, ID` (the first base's class) |
| owned feature `top speed` | `TopSpeed { get; }`, PascalCase of the name's letters and digits |
| `Boolean`, `String`, `Integer`/`Natural`, `Rational`, `Real`, `Complex` | `bool`, `string`, `BigInteger`, `Rational`, `double`, `Complex` |
| a quantity whose value names a unit | `Quantity` |
| an enumeration | `Value.EnumLiteralValue` |
| a generated definition | its interface |
| anything else | `Value`, the property's remarks saying why |
| multiplicity `1` or none, `0..1`, more or an unknown upper bound | `T`, `T?`, `ValueList<T>` |

With an upper bound of one and an unknown lower bound, a property is nullable. Inherited type and
multiplicity facts have no fixed depth limit. Several redefined features contribute intersected
ranges and the most specific type that every inherited property can read; incompatible types or
disjoint ranges are refused with the providers' names.

Every property is an explicit interface implementation. A class implements its own interface and
every inherited property whose value it reads from another feature than its base class does: an
inherited property is read from the most specific feature redefining it in that class, whatever its
name, with the inherited property's decoder; a redefinition of the same name declares the narrower
property with `new`. Each class has `const string SysmlId`, `FromInstance(Instance, Instantiation?)`,
`FromInstance(Instantiation)` and `Unchecked(Instance, Instantiation?)`, returning its interface.
`FromInstance` views an instance as the generated class of its type (a usage as the definition it is
typed by), which a specialization's class implements too; refuses one of an unrelated generated
definition with `InstanceTypeException`; and views one no generated class describes as its own class.

Generation is refused, with every problem naming the SysML elements behind it, for: two features of
a definition, own or inherited, that neither redefines the other under one property name; two
redefinitions of a feature meeting in a specialization, neither redefining the other; a
redefinition whose type or multiplicity the redefined property cannot read (a narrower numeric type,
a specialization's interface or fewer values can be read); two elements generating one C# type or
namespace name, or a type and a namespace of one name; a property named `Instance`, `Instantiation`,
`Equals`, `GetHashCode`, `GetType`, `ToString`, `MemberwiseClone`, `Finalize`, `ReferenceEquals` or
its interface's name; a class named `SysmlId`, `FromInstance`, `Unchecked` or, in the root
namespace, `SysmlTypes`; a definition specializing itself; an unnamed definition or a feature name
holding no letter or digit; and a reference the documents and the standard library do not resolve.
An unnamed feature has no property, and the interface's remarks count it. Contextual type keywords
(`record`, `required`, `file`, `scoped`) and compiler-intrinsic keywords are escaped with `@`.
Unicode formatting characters are removed before identifier generation and collision checks, so
names the C# compiler considers equal cannot silently produce duplicate declarations.

The root namespace's `SysmlTypes` holds `GeneratorVersion`, the emission schema, bumped whenever the
file generated from an unchanged model changes; `ModelHash`, `sha256:` and the SHA-256 digest of the
document count, then for each document in ordinal order of its name the name's length and UTF-8
bytes and the content's length and exact bytes, each length a big-endian 64-bit integer (a file
argument is named by its file name, a directory's files by the directory's name and their path
within it); and `Registry`, the `TypedRegistry` every view is created through. The file has no
timestamp, path or service version.

Generated code reads through `OpenMBEE.OpenSysML.Typed`:

| Type | Role |
| --- | --- |
| `ITypedObject`, `TypedObject` | a view: `Instance` and the `Instantiation` nested objects resolve through; equal over equal instances of one class |
| `TypedFeatures` | `Required`, `Optional`, `OptionalValue` and `List`, reading a feature by name |
| `FeatureDecoders` | `AsBoolean`, `AsString`, `AsInteger`, `AsRational`, `AsReal`, `AsComplex`, `AsQuantity`, `AsEnumLiteral`, `AsValue`; a number widens without loss (an Integer to a Real, a finite Real to its exact Rational) |
| `TypedRegistry` | definitions to classes and usages to definitions; `FromInstance<T>` and `Decoder<T>` |

A feature the instance does not have, or holding null or `Unset`, reads as null or an empty list,
and fails a required property; a sequence reads as several values.

## Metamodel types

The `OpenMBEE.OpenSysML.Metamodel` assembly is the SysML v2 metamodel as C# types and a reader of
exported model JSON into them, standalone: it references neither `OpenMBEE.OpenSysML` nor any package
at run time, needs no service, and is consumed by its own `ProjectReference`:

```xml
<ProjectReference Include="path/to/OpenSysML/client/dotnet/src/OpenMBEE.OpenSysML.Metamodel/OpenMBEE.OpenSysML.Metamodel.csproj" />
```

`tools/gen/csmetamodel` generates its types from the repository's metamodel table (`make
dotnet-metamodel`; `make dotnet-metamodel-check` fails when they are stale):

| Generated | For | Shape |
| --- | --- | --- |
| `IPartUsage` and one interface per metaclass | every metaclass | extends its superclasses' interfaces, so `is` and `OfType<T>()` select by metamodel ancestry; declares the metaclass's own properties, read-only and PascalCased |
| `Elements.PartUsage` and one sealed class per concrete metaclass | concrete metaclasses | derives from `MetamodelElement` and implements every inherited property explicitly; created only by the reader |
| an internal sealed class per abstract metaclass | abstract metaclasses | the same, for elements a document types as the abstract metaclass or as a type the reader maps to it; read through the interface |
| `FeatureDirectionKind` and one enum per enumeration | every enumeration | members are the literals PascalCased, valued by position |
| `Metaclasses.PartUsage` and one `MetaclassInfo` per metaclass | every metaclass | `MetamodelVersion`, `SourceTag` and `SourceCommit` record the metamodel release |

A single-valued property reads as a nullable (`string?`, `bool?`, `int?`, `double?`, an enum or a
metaclass interface), a many-valued one as an `IReadOnlyList<T>`. A redefinition that keeps its
property's name narrows the type in the subclass's interface, and every interface of the chain reads
the most specific value.

| Type | Role |
| --- | --- |
| `IMetamodelElement` | what every element has: `Id` (`@id`), `JsonType` (`@type` as written), `MetaclassInfo`, `GetValue(jsonKey)`, an untyped read, and `Extensions`, the keys no property has; no property may take these names |
| `MetamodelElement` | base of the element classes; `ToString` is the metaclass and `@id` |
| `Metaclasses` | `All`, `Enumerations`, `TryGet` by name or generated type, `Of<T>()`, `TryGetEnumeration` |
| `MetaclassInfo` | `Name`, `IsAbstract`, `InterfaceType`, `ElementType`, `Parents`, `Ancestors` (linearized, nearest first), `DeclaredProperties`, `Properties`, `ConformsTo`, `TryGetProperty` and `GetProperty` by JSON key, `TryGetPropertyByName` |
| `MetaProperty` | `JsonKey`, `Name`, `QualifiedName`, `Iri`, `DeclaringMetaclass`, `RangeKind`, `RangeMetaclass`, `RangeEnumeration`, `ValueType`, `IsMany`, `IsOrdered`, `IsUnique`, `IsDerived`, `LowerBound`, `DefaultLiteral`, `Redefines`, `Subsets`, `Opposite` |
| `PropertyRangeKind` | `Metaclass`, `Enumeration` or `Primitive` |
| `EnumerationInfo` | `Name`, `EnumType`, `Literals`, `TryParse`, `GetLiteral`, `Of<TEnum>()` |

Metadata collections are immutable snapshots: registry lists, parents and ancestors, property lists
and association links, and enumeration literals expose no writable backing arrays. The generator
rejects names that hide runtime or framework types or inherited registry members, and property kinds
that do not match their primitive, enumeration or metaclass ranges.

A property the document does not supply throws `NotSuppliedException`, never a default the
metamodel states; one supplied as null reads as null, or as empty when many-valued.

### Reading exported JSON

```csharp
var graph = MetamodelGraph.ReadJson("vehicle.json");   // a path, never JSON text
var vehicle = graph.All<IPartDefinition>().Single(definition => definition.DeclaredName == "Vehicle");
var parts = vehicle.OwnedFeature.OfType<IPartUsage>();
```

| Type | Role |
| --- | --- |
| `MetamodelGraph` | `ReadJson` from a path, a `Stream`, a `JsonDocument` or `JsonElement`s, each with or without `ReadOptions`; `Elements` in document order; `this[id]`, `TryGet`, `Contains`; `All<T>()` by metamodel ancestry; `Roots`, the namespaces with no recorded owner; `CheckRanges`; `KeepsExtensions` |
| `ReadOptions` | `CheckRanges`, true by default; `Supertypes`, the metaclass chain of `@type`s the metamodel lacks; `KeepExtensions`, false by default |

A document is an element object, an array of them, a DataVersion or an array of them, or a Commit
whose `change` holds DataVersions; a null payload holds no element. `ReadJson` neither disposes a
caller's stream or document nor keeps any of its JSON: the graph is complete, every value converted
and every reference resolved, when it returns, and immutable. `@type` is matched without a `sysml:`
prefix or SysML namespace IRI, versioned or not, and kept as written; a type the metamodel lacks is
read as the first metaclass along `Supertypes`, else as `Element`. A key the element's metaclass has
no property for, such as an exporter's `sysx:` annotations, is not converted; with `KeepExtensions`
each element's `Extensions` holds those members as written, copied out of the source, and without it
reading `Extensions` throws `InvalidOperationException`.

A document that is not an element graph fails the read with `MalformedDocumentException`. Every
source overload rejects invalid UTF-8, undecodable Unicode property names or element identities,
and duplicate keys anywhere, including ignored extensions and envelope metadata. A value
that does not fit its property, a reference outside its metaclass or an undecodable Unicode string
among them, fails only reads of that
property, every time, with `MalformedValueException`, and a reference to an `@id` the document does
not include, or any `@ref`, with `UnresolvedReferenceException`. With `CheckRanges` off an
out-of-range reference is kept: the typed property, which cannot hold it, still throws, and `GetValue`
returns it. OpenSysML's `api-json` export omits the derived `feature`, `inheritedFeature` and
`definition`, which read as `NotSuppliedException` with `IsDerived`, and refers to standard-library
elements it does not include. The compact form is not read.

## Exceptions

Every failure derives from `OpenSysMLException`, except cancellation and use after disposal:

```text
OpenSysMLException
├── InvalidRequestException        refused before anything was sent
├── CapabilityException            the service lacks a capability; Capability, Remedy
├── ServiceStartException          no service could be started or reached
│   ├── BinaryNotFoundException       SearchedLocations
│   ├── ChecksumMismatchException     BinaryPath, ExpectedSha256, ActualSha256
│   ├── UnpinnedReleaseException      Repository, Version, Asset
│   ├── UnsupportedPlatformException  Platform
│   └── StaleServiceException
├── ServiceStopException           Failures
├── ServiceException               a status the service sent; StatusCode, ServiceMessage, Method
│   ├── AuthenticationException       Unauthenticated: no token, or a refused one; CredentialsSent, Remedy
│   ├── ModelNotFoundException
│   └── ModelFileNotFoundException
├── ModelException                 the service answered a failure about the model; Diagnostics
│   ├── ModelErrorsException, SymbolNotFoundException, ExecutionException, CheckException
│   ├── QueryException, ConversionException, MigrationException, EditException
│   └── VerificationException
│       └── AnalysisException, RunsException, RecordException, CompareException
├── TransportException             no well-formed answer was read; StatusCode, Method
├── UnsupportedValueException      a value of a kind this client does not know
├── UnwritableException            a destination that cannot be written safely
├── FeatureValueException          a typed property's feature failed or was never materialized; FeatureName, Reason
├── TypeMismatchException          a typed property's feature holds another type or number; FeatureName, Expected, Actual
└── InstanceTypeException          a typed view of an instance of an unrelated definition; Expected, Actual
```

The metamodel assembly's failures derive from `Exception` directly, since it does not depend on the
client library:

```text
MetamodelException
├── NotSuppliedException           a property the document does not supply; Property, ElementId, IsDerived
├── UnresolvedReferenceException   a property's reference the document does not resolve; Property, ElementId, TargetId, TargetRef
├── MalformedValueException        a property's value that does not fit it; Property, ElementId
├── MalformedDocumentException     a document that is not an element graph
└── UnknownJsonKeyException        a JSON key the metaclass has no property for; Metaclass, JsonKey
```

A `ServiceException` is only ever a status the service itself sent. A status the transport made
up, a malformed answer, an over-limit response and a passed deadline are `TransportException`s;
so is a proxy's HTTP 401 page, which is not an `AuthenticationException`.
A failure that kept a partial result carries it: a check's or an analysis's `Partial`, a run's
`Trace`, a refused edit's `Referrers`.

## Capability negotiation

`Capabilities` holds a constant for each capability name the service can advertise. Decide on
those names, never on the version. Before sending, every typed call checks the capabilities its
arguments need, in the order the service checks them, and raises `CapabilityException` naming the
first one missing, so a gated option is never sent to a service that would ignore it. Response
fields a service fills only under a capability read as empty or null, never invented, from a
service without it. `CallAsync` is the escape hatch below this and checks nothing.

## The service binary

A private service is started from `BinaryPath`, else `OPENSYSML_GRPC_BINARY`, else the release
asked for in the shared cache `~/.opensysml/bin/sysml-grpc` (`sysml-grpc.exe` on Windows),
which it downloads, verifies and installs under the cache's cross-client lock when the cache does
not hold it. When the release the client was built with cannot be had, the first `sysml-grpc` on
`PATH` is started instead. The release is `DownloadVersion`, else `OPENSYSML_GRPC_VERSION`, else
`v` and the client's version; its repository is `GithubRepo`, else `OPENSYSML_GITHUB_REPO`, else
`Open-MBEE/OpenSysML`. Releases are published for Linux and macOS on x64 and arm64 and for
Windows on x64; another platform that asks for one fails with `UnsupportedPlatformException`.

A download is trusted only against the SHA-256 pins embedded in the assembly, a copy of
`client/release-digests.json`; the digest the release serves must agree with the pin. The client
does not verify the Sigstore signature on a release's checksum manifest, so a release it pins no
digest for fails with `UnpinnedReleaseException` before anything is fetched, unless
`OPENSYSML_ALLOW_UNPINNED_DOWNLOAD` grants same-origin trust. That trusts the digest served beside
the binary, which detects corruption but not a compromised release. Releases come over HTTPS
alone, from `github.com` and `api.github.com`; a stalled or oversized response is abandoned and
never retried.

## What the client does not do

- It is not on NuGet, and has no packaged `dotnet` tool, symbol package or SourceLink.
- It targets `net10.0` only, and makes no Native AOT, trimming, browser or Blazor claim.
- It has no synchronous API, no notebook display hooks and no DataFrame integration.
- Its standalone metamodel reader reads the API's element array and sysml-toolkit JSON, not the
  compact form, and computes no derived property an export omits. Its typed-class generator is a
  checkout console, not a Roslyn source generator, MSBuild task or packaged `dotnet` tool.
- It never retries a call, falls back to another protocol, or bypasses TLS validation.
- It does not decode the `QueryOutcome` detail a refused query carries: the refusal is a
  `ServiceException` with its status and message, as in the other clients.
- It starts no service implicitly from the compiler or MSBuild.

## Conformance

The client's conformance runner runs the shared [conformance suite](../../conformance/) through
the public API, over gRPC, Connect with protobuf and Connect with JSON, and again against a
service withholding `strict_conformance` and `oslc_query`. The only skips are the reviewed
requests the public types cannot express:

```bash
make build-grpc
make conformance-dotnet          # bin/conformance-report-dotnet.json and .xml
make conformance-dotnet-check    # the same run beside the reference runner's, scenario by scenario
```

The scenario counts, the reviewed skips and the runner's flags are in
[client/dotnet/README.md](../../client/dotnet/README.md#conformance).
