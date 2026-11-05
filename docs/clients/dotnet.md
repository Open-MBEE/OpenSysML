# .NET

`OpenMBEE.OpenSysML` is the C# client for the service. Choose it when a .NET 10 application
needs typed, asynchronous access to the whole service surface: models and symbols, execution and
behavior checks, verification and analysis, conversion and migration, queries and documents,
source edits, data import and result comparison.

## Install

The client is built from a checkout of this repository and consumed by `ProjectReference`; it is
not published to NuGet. It targets `net10.0` only, built with the .NET SDK the repository-root
`global.json` pins. Run `dotnet` from the repository root so that pin applies, and check it with
`make dotnet-preflight`.

```xml
<ItemGroup>
  <ProjectReference Include="path/to/OpenSysML/client/dotnet/src/OpenMBEE.OpenSysML/OpenMBEE.OpenSysML.csproj" />
</ItemGroup>
```

Its version is the core version of the checkout, and the release of a version is the checkout of
the core `v<version>` tag.

## First model

This program is the checkout's
[`samples/Vehicle`](../../client/dotnet/samples/Vehicle/)
consumer, which the client's test target runs against a real service. It parses inline notation,
checks the parse diagnostics, looks the sedan up and evaluates its redefined mass:

```csharp
using System;
using OpenMBEE.OpenSysML;

const string Source = """
    package Demo {
      part def Vehicle {
        attribute mass default = 1500.0;
      }
      part sedan : Vehicle {
        attribute :>> mass = 1800.0;
      }
    }
    """;

try
{
    await using var connection = await Connection.OpenAsync();
    Console.WriteLine(connection.ServerInfo);

    var model = await connection.LoadContentAsync(Source);
    if (model.HasErrors)
    {
        foreach (var diagnostic in model.Errors)
        {
            Console.Error.WriteLine(diagnostic);
        }
        return 1;
    }

    var sedan = await model.LookupAsync("sedan");
    var mass = await model.EvalAsync("mass", new EvalOptions { SubjectSymbolId = sedan.Id });
    Console.WriteLine($"{sedan}: mass = {mass}");
    return 0;
}
catch (OpenSysMLException error)
{
    Console.Error.WriteLine(error.Message);
    return 1;
}
```

Build the service and run the sample from the repository root:

```bash
make build-grpc
OPENSYSML_GRPC_BINARY="$PWD/bin/sysml-grpc" dotnet run --project client/dotnet/samples/Vehicle -c Release
```

Keep the binary setting when running the sample: with neither `OPENSYSML_SERVICE` nor
`OPENSYSML_GRPC_BINARY` set, it prints only the assembly identity and exits without running the
walkthrough.

```text
OpenMBEE.OpenSysML <version> (.NETCoreApp,Version=v10.0)
sysml-grpc <version>, <count> capabilities
Demo::sedan (partUsage): mass = 1800.0
```

The first line is the referenced assembly's version, the checkout's, and its target framework; the
second is the version and capability count the service reports. With nothing named,
`Connection.OpenAsync` starts a private `sysml-grpc` as a child of the program and stops it when
the connection is disposed. A source with errors still parses into a `Model`, whose `Errors`
say what is wrong; `RequireNoErrors` turns them into an exception instead. Every failure the
client reports derives from `OpenSysMLException`, while a false verdict is an answer, not an
exception.

## Connections and protocols

`ConnectionOptions.Address`, or else `OPENSYSML_SERVICE`, names a service that is already
running; the client never starts, stops or downloads a service named that way. Without one, it
starts a private child, shared by the connections of one copy of the client (one assembly load
context) that agree on its binary and timeouts; `IsolatedService` gives a connection its own.
The child exits when its input closes, so it ends with the process however the process ends.

A service that serves an address other machines reach requires a bearer token. Give it as
`ConnectionOptions.Token`, or set `OPENSYSML_GRPC_TOKEN`, which the client reads when `Token` is
null; every call then carries `authorization: Bearer <token>`. A missing or wrong token is an
`AuthenticationException` saying which. The private child is started without the variable, so it
never demands the token meant for a remote service. See
[Service transports](../reference/service-transports.md#authentication).

A connection speaks the one protocol it is given, never retries a call and never falls back to
another protocol:

| `Protocol` | `Encoding` | On the wire |
| --- | --- | --- |
| `Grpc` (default) | `Protobuf` (default) | gRPC over HTTP/2 |
| `Connect` | `Protobuf` | one HTTP/1.1 `POST` per call, `application/proto` |
| `Connect` | `Json` | the same, `application/json`, for reading the traffic |

## Service source

The private service's binary is `ConnectionOptions.BinaryPath`, else `OPENSYSML_GRPC_BINARY`,
else the release asked for in the shared cache `~/.opensysml/bin`, downloaded there when the
cache does not hold it. Only when the release the client was built with cannot be had does it
fall back to `sysml-grpc` on `PATH`. The cache, its record and its lock are the ones every
OpenSysML client shares.

A release is downloaded only when the checkout pins the SHA-256 digest of its asset. The client
does not verify the Sigstore signature of a release's checksum manifest, so a release it pins no
digest for is refused with `UnpinnedReleaseException` unless `OPENSYSML_ALLOW_UNPINNED_DOWNLOAD`
grants same-origin trust. A checkout whose own version is not yet released pins nothing for it,
so from a development checkout set `OPENSYSML_GRPC_BINARY` to a `make build-grpc` binary, as
above. See the
[service-binary reference](../reference/clients.md#providing-the-service-binary).

## Typed model classes

The checkout's generator writes typed C# interfaces and classes over your model's definitions,
which you commit and read instances through. Run it from the repository root against a service
that reports static type facts (the same default service as any connection, or `--address`):

```bash
dotnet run --project client/dotnet/src/OpenMBEE.OpenSysML.Generate -- model.sysml -o ModelTypes.cs --namespace Acme.Model
```

Each definition gets an interface, `IVehicle`, with a read-only property per feature, typed from the
model (`double`, `BigInteger`, `Rational`, `Quantity`, a generated interface, or `Value` where no C#
type is sound), and a class implementing it. Packages become namespaces under `--namespace`.

```csharp
using Acme.Model.Demo;

var vehicle = Vehicle.FromInstance(await model.InstantiateAsync("Demo::Vehicle"));
Console.WriteLine(vehicle.Engine.Power);
```

`FromInstance` returns the most specific generated class for the instance, so an instantiated
`car : Car` is an `ICar`. Generation refuses, naming the SysML elements, any model it cannot type
soundly: two unrelated features meeting under one property name, a redefinition the redefined
property cannot read, two elements generating one C# name, a reference outside the documents.
Commit the file and gate it with `--check`, which exits 1 when the model has changed. The
[.NET API reference](../reference/dotnet-api.md#typed-model-classes) lists every rule.

## Reading exported JSON

The checkout's `OpenMBEE.OpenSysML.Metamodel` project holds the SysML metamodel as C# types, one
interface per metaclass, and reads an exported model into them without a service. Reference it alone:

```xml
<ProjectReference Include="path/to/OpenSysML/client/dotnet/src/OpenMBEE.OpenSysML.Metamodel/OpenMBEE.OpenSysML.Metamodel.csproj" />
```

```csharp
using OpenMBEE.OpenSysML.Metamodel;

var graph = MetamodelGraph.ReadJson("vehicle.json");   // what ToApiJsonAsync wrote, or sysml-toolkit JSON
foreach (var part in graph.All<IPartUsage>())          // every element whose metaclass specializes PartUsage
{
    Console.WriteLine($"{part.DeclaredName}: {string.Join(", ", part.Type.Select(type => type.DeclaredName))}");
}
```

An export need not carry every property. One it omits throws `NotSuppliedException`, never a default;
OpenSysML's export omits derived properties such as `feature` and `definition`. A reference to an
element it does not include, such as a standard-library type, throws `UnresolvedReferenceException`,
and a value that does not fit its property `MalformedValueException`, when that property is read; the
rest of the graph reads. A document that is not an element graph fails `ReadJson` itself with
`MalformedDocumentException`. [`samples/MetamodelReader`](../../client/dotnet/samples/MetamodelReader/)
is a complete program, and the
[.NET API reference](../reference/dotnet-api.md#reading-exported-json) lists the rules.

## Limitations

- Distributed as a checkout only: no NuGet package and no packaged `dotnet` tool.
- `net10.0` only. Native AOT and trimming are not supported, and nothing is claimed for
  browsers or Blazor: a platform that cannot start a process fails a private start with
  `UnsupportedPlatformException`.
- Asynchronous only, with no synchronous mirrors, notebook display or DataFrame integration.
- Typed model classes come from a checkout console, not a Roslyn source generator, MSBuild task
  or packaged `dotnet` tool, and no build starts a service.
- The metamodel reader computes no derived property an export omits and does not read the compact
  `api-json` form; it implements no metamodel operation and loads nothing into a service.

## Next steps

- [.NET API reference](../reference/dotnet-api.md)
- [Client README, testing and conformance runner](../../client/dotnet/README.md)
- [Shared SysML model examples](../../examples/)
