---
name: opensysml-dotnet
description: Parse, query, instantiate, execute and verify SysML v2 models from C# with the asynchronous `OpenMBEE.OpenSysML` client for .NET 10, referenced from an OpenSysML checkout, which drives the sysml-grpc service — requiring a clean model, switching on Value arms, running actions and calcs, checking requirements, and catching the OpenSysMLException family that keeps refused calls apart from reported model failures. Use when writing C# that reads or runs SysML v2 models.
---

# Using OpenSysML from C# and .NET

`OpenMBEE.OpenSysML` is an asynchronous client for the `sysml-grpc` service; it starts a private
one for you and stops it when the connection is disposed. It targets `net10.0` only and is not on
NuGet. Reference: `docs/clients/dotnet.md`, `docs/reference/dotnet-api.md` and
`client/dotnet/README.md` in the OpenSysML repository.

## Install and get the service binary

Reference the project from a checkout of OpenSysML, built with the SDK its `global.json` pins:

```xml
<ItemGroup>
  <ProjectReference Include="path/to/OpenSysML/client/dotnet/src/OpenMBEE.OpenSysML/OpenMBEE.OpenSysML.csproj" />
</ItemGroup>
```

Binary resolution is `ConnectionOptions.BinaryPath`, then `$OPENSYSML_GRPC_BINARY`, then the
shared cache `~/.opensysml/bin/sysml-grpc`, downloading the release the client was built with (or
`$OPENSYSML_GRPC_VERSION`) into it when the checkout pins that release's digest, then `sysml-grpc`
on `PATH`. A checkout whose own version is unreleased pins nothing for it: point
`$OPENSYSML_GRPC_BINARY` at a `make build-grpc` binary. Without one, opening fails with
`BinaryNotFoundException` listing `SearchedLocations`.

## Load, and require a clean model

```csharp
using OpenMBEE.OpenSysML;

await using var connection = await Connection.OpenAsync();   // $OPENSYSML_SERVICE, else a private child
var model = await connection.LoadAsync("model.sysml");        // the service reads the path
foreach (var d in model.Errors)                               // Diagnostic: Severity, Code, Message, Span
{
    Console.Error.WriteLine($"{d.Code}: {d.Message}");
}
model.RequireNoErrors();                                      // ModelErrorsException carrying them
```

A model with syntax errors still parses; branch on `d.Code` (`"syntax"`, `"unresolved"`, ...),
never on message text. `LoadContentAsync(source)` parses inline notation;
`new ParseOptions { StrictConformance = true }` makes OpenSysML-only notation an error. For several
files use `ParseSourcesAsync([SourceDocument.FromFile("a.sysml"), SourceDocument.FromFile("b.sysml")])`;
`LoadAsync` reads one document, so imports across files do not resolve. Pass
`new ConnectionOptions { Address = "localhost:50051" }` to reach a running service, which the client
never stops.

## Evaluate, instantiate, run

```csharp
var mass = await model.EvalAsync("mass", new EvalOptions { SubjectSymbolId = "Demo::sedan" });   // 1800.0

var built = await model.InstantiateAsync("Demo::sedan");
var builtMass = built.Root.FeatureValues["mass"].Value;                                           // 1800.0

var run = await model.ExecuteActionAsync("Demo::addFive",
    new Dictionary<string, Value> { ["x"] = new Value.IntegerValue(10) });
var result = run.Outputs["result"];                                                               // 15, for out result : Integer[1]

var calc = await model.CalcAsync("Demo::Margin", [new Value.RealValue(1200.0), new Value.RealValue(2000.0)]);
var margin = calc.Result;                                                                         // 800.0, for return : Real[1]
```

An output or calc result declared without `[1]` answers a `SequenceValue` of its values, `(15)`;
declare `[1]` to read the scalar.

`Value` is a sealed hierarchy (`IntegerValue`, `BigIntegerValue`, `RationalValue`, `RealValue`,
`BooleanValue`, `TextValue`, `QuantityValue`, `EnumLiteralValue`, `InstanceRefValue`,
`SequenceValue`, `SetValue`, ...); switch on it with a default case. Equality is by content, not
SysML's numeric comparison: `IntegerValue(1)` is not `RealValue(1.0)`. `UnsetValue`,
`UndeterminedValue` and `NullValue` are answers, never errors; do not read them as 0 or `false`.

## Verify

```csharp
var v = await model.VerifyRequirementAsync("Demo::Vehicle::lightEnough",
    new VerifyOptions { SubjectSymbolId = "Demo::truck" });
_ = v.Holds;                 // false — an answer, not an exception
_ = v.Verdict.Decided;       // true: the model answered; false when evaluation failed (see v.Verdict.Error)
_ = v.Verdict.Condition;     // "mass < 2000.0"
var all = await model.SatisfiedAsync();   // every `assert satisfy R by x;` decided and holds
```

`VerifyConstraintAsync` takes the same options; `VerifySatisfactionAsync` returns every verdict.

## Errors

Everything derives from `OpenSysMLException`, except `OperationCanceledException` and
`ObjectDisposedException`:

- `ServiceException` — the service refused the call with a status (`StatusCode`, `ServiceMessage`);
  `ModelFileNotFoundException` for an unreadable file, `ModelNotFoundException` for a model it no
  longer holds.
- `ModelException` — the call was answered and reports a model failure, with `Diagnostics`:
  `ModelErrorsException` (`RequireNoErrors`), `SymbolNotFoundException` (`Name`, `Suggestions`),
  `ExecutionException`, `VerificationException` (`Reason`), `AnalysisException` (`Partial`),
  `EditException`, `ConversionException`, `QueryException`.
- `CapabilityException` (service too old, raised before sending), `InvalidRequestException`,
  `ServiceStartException` and its `BinaryNotFoundException`, `ChecksumMismatchException`,
  `UnpinnedReleaseException`, `TransportException`, `UnsupportedValueException`.

## Pitfalls

- Reuse one `Connection` and dispose it with `await using`; connections that agree on the binary
  share the private child.
- `FindAsync` and `FindSymbolAsync` answer null for an unknown name; `LookupAsync` and `SymbolAsync`
  throw `SymbolNotFoundException`.
- Use qualified names (`Package::Def::feature`) wherever a symbol id is asked for.
- There is no synchronous API, no NuGet package, no Native AOT or trimming support and no browser
  host; do not suggest them.
- `Connection.CallAsync` reaches any RPC with the generated `OpenMBEE.OpenSysML.Wire` messages when
  the typed surface does not cover a field.
- To read an exported JSON model without a service, reference
  `client/dotnet/src/OpenMBEE.OpenSysML.Metamodel` alone and call `MetamodelGraph.ReadJson`; a property
  the export omits throws `NotSuppliedException`, never a default.
- Write models with the `sysml-v2-modeling` skill; `sysml -validate` reports the same diagnostics as
  `model.ParseDiagnostics`.
