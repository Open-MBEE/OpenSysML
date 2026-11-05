using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading.Tasks;
using Google.Protobuf;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// GetDiagnostics, Evaluate and Instantiate through the typed API, over every wire: the exact request,
/// every field of the answer, the subject gate checked before sending, failures the service reports in
/// band kept apart from answers that are malformed.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed class ModelEvaluateTests
{
    public static TheoryData<ClientWire> Wires => ClientWires.All;

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task GetDiagnosticsSendsTheLintSelectionAndReadsTheAnswer(ClientWire wire)
    {
        using var handler = new FakeServiceHandler(wire) { Capabilities = FakeModels.ModelCapabilities }
            .Parsing("h", FakeModels.Symbol("r", string.Empty, "RootNamespace"))
            .On("GetDiagnostics", Wire.DiagnosticsRequest.Parser, request => request.EnableLint.Count > 1
                ? new Wire.DiagnosticsResponse { Error = "lints failed", Diagnostics = { FakeModels.Diagnostic("error", "inner") } }
                : new Wire.DiagnosticsResponse { Diagnostics = { FakeModels.Diagnostic("warning", "signal", "undeclared-signal") } });
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = await connection.LoadAsync();

        var all = await model.GetDiagnosticsAsync(cancellationToken: TestContext.Current.CancellationToken);
        var selected = await model.GetDiagnosticsAsync(
            new DiagnosticsOptions { DisableLint = ["port-type-mismatch"], EnableLint = ["rounded-real-literal"] }, TestContext.Current.CancellationToken);
        var failed = await Assert.ThrowsAsync<ModelException>(() =>
            model.GetDiagnosticsAsync(new DiagnosticsOptions { EnableLint = ["a", "b"] }, TestContext.Current.CancellationToken));

        Assert.Equal(
            [
                new Wire.DiagnosticsRequest { ModelHash = "h" },
                new Wire.DiagnosticsRequest { ModelHash = "h", DisableLint = { "port-type-mismatch" }, EnableLint = { "rounded-real-literal" } },
                new Wire.DiagnosticsRequest { ModelHash = "h", EnableLint = { "a", "b" } },
            ],
            handler.RequestsFor("GetDiagnostics").Select(sent => sent.Message(Wire.DiagnosticsRequest.Parser)));
        Assert.Equal("undeclared-signal", Assert.Single(all).Code);
        Assert.Equal(all, selected);
        Assert.Equal("lints failed", failed.Message);
        Assert.Equal(Model.GetDiagnosticsMethod, failed.Method);
        Assert.Equal("inner", Assert.Single(failed.Diagnostics).Message);
        Assert.Throws<ArgumentException>(() => new DiagnosticsOptions { DisableLint = [null!] });
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task EvalSendsTheExpressionAndItsScopeAndReadsTheValue(ClientWire wire)
    {
        using var handler = new FakeServiceHandler(wire) { Capabilities = FakeModels.ModelCapabilities }
            .Parsing("h", FakeModels.Symbol("r", string.Empty, "RootNamespace"))
            .On("Evaluate", Wire.EvaluateRequest.Parser, request => new Wire.EvaluateResponse
            {
                Result = new Wire.Value { Quantity = new Wire.Quantity { RealMagnitude = 1500.0, Unit = "kg", UnitTerm = new Wire.UnitTerm { ScaleNum = 1, ScaleDen = 1, Factors = { new Wire.UnitFactor { UnitId = "SI::kilogram", Exponent = 1 } } } } },
                Diagnostics = { FakeModels.Diagnostic("info", request.Expression) },
            });
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = await connection.LoadAsync();
        var mass = new Value.QuantityValue(new Quantity(new Value.RealValue(1500.0), "kg", new UnitTerm(1, 1, [new UnitFactor("SI::kilogram", 1)])));

        var plain = await model.EvalAsync("mass", cancellationToken: TestContext.Current.CancellationToken);
        var evaluation = await model.EvaluateAsync(
            "mass", new EvalOptions { ContextSymbolId = "Demo::Vehicle", SubjectSymbolId = "Demo::sedan" }, TestContext.Current.CancellationToken);

        Assert.Equal(
            [
                new Wire.EvaluateRequest { ModelHash = "h", Expression = "mass" },
                new Wire.EvaluateRequest { ModelHash = "h", Expression = "mass", ContextSymbolId = "Demo::Vehicle", SubjectSymbolId = "Demo::sedan" },
            ],
            handler.RequestsFor("Evaluate").Select(sent => sent.Message(Wire.EvaluateRequest.Parser)));
        Assert.Equal(mass, plain);
        Assert.Equal(new Evaluation(mass, [new Diagnostic("info", "mass", string.Empty, null, [], false)]), evaluation);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ASubjectNeedsEvaluateSubjectAndAnEmptySymbolIsRefusedBeforeSending(ClientWire wire)
    {
        using var handler = new FakeServiceHandler(wire) { Capabilities = [.. FakeModels.ModelCapabilities.Except([Capabilities.EvaluateSubject])] };
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash("h");

        var gated = await Assert.ThrowsAsync<CapabilityException>(() =>
            model.EvalAsync("mass", new EvalOptions { SubjectSymbolId = "Demo::sedan" }, TestContext.Current.CancellationToken));
        await Assert.ThrowsAsync<InvalidRequestException>(() => model.EvalAsync("mass", new EvalOptions { ContextSymbolId = string.Empty }, TestContext.Current.CancellationToken));
        await Assert.ThrowsAsync<InvalidRequestException>(() => model.EvalAsync("mass", new EvalOptions { SubjectSymbolId = string.Empty }, TestContext.Current.CancellationToken));
        await Assert.ThrowsAsync<ArgumentNullException>(() => model.EvalAsync(null!, cancellationToken: TestContext.Current.CancellationToken));

        Assert.Equal(Capabilities.EvaluateSubject, gated.Capability);
        Assert.Equal(["GetServerInfo"], handler.Requests.Select(request => request.Method));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AnEvaluationFailureIsInBandAndAnAnswerOfNeitherOrOfAnUnknownKindIsRefused(ClientWire wire)
    {
        var unknown = new Wire.Value();
        unknown.MergeFrom(new byte[] { 0xFA, 0x03, 0x01, 0x00 }); // field 63, an arm no client knows yet
        using var handler = new FakeServiceHandler(wire) { Capabilities = FakeModels.ModelCapabilities }
            .On("Evaluate", Wire.EvaluateRequest.Parser, request => request.Expression switch
            {
                "(((" => new Wire.EvaluateResponse { Error = "expression parse failed", Diagnostics = { FakeModels.Diagnostic("error", "unexpected (") } },
                "nothing" => new Wire.EvaluateResponse(),
                _ => new Wire.EvaluateResponse { Result = unknown },
            });
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash("h");

        var failed = await Assert.ThrowsAsync<ModelException>(() => model.EvalAsync("(((", cancellationToken: TestContext.Current.CancellationToken));
        var empty = await Assert.ThrowsAsync<TransportException>(() => model.EvalAsync("nothing", cancellationToken: TestContext.Current.CancellationToken));
        var unsupported = await Assert.ThrowsAsync<UnsupportedValueException>(() => model.EvalAsync("future", cancellationToken: TestContext.Current.CancellationToken));

        Assert.Equal("expression parse failed", failed.Message);
        Assert.Equal(Model.EvaluateMethod, failed.Method);
        Assert.Equal("unexpected (", Assert.Single(failed.Diagnostics).Message);
        Assert.Equal(StatusCode.Internal, empty.StatusCode);
        Assert.Equal("result", unsupported.Path);
        Assert.Equal(Model.EvaluateMethod, unsupported.Method);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task InstantiateReadsTheObjectGraphAndResolvesItsReferences(ClientWire wire)
    {
        var root = new Wire.Instance { Id = 1, TypeSymbolId = "Demo::Vehicle" };
        root.FeatureValues["mass"] = new Wire.FeatureValue { FeatureName = "mass", Value = new Wire.Value { RealValue = 1500.0 }, Materialized = true, OrderDependent = 2 };
        root.FeatureValues["engine"] = new Wire.FeatureValue { FeatureName = "engine", Value = new Wire.Value { InstanceId = 2 }, Materialized = true };
        root.FeatureValues["wheels"] = new Wire.FeatureValue { FeatureName = "wheels", Values = { new Wire.Value { IntValue = 1 }, new Wire.Value { IntValue = 2 } }, Materialized = true };
        root.FeatureValues["a"] = new Wire.FeatureValue { FeatureName = "a", Error = "cyclic dependency: a -> b -> a", Materialized = true };
        root.FeatureValues["spare"] = new Wire.FeatureValue { FeatureName = "spare" };
        var engine = new Wire.Instance { Id = 2, TypeSymbolId = "Demo::Engine" };
        using var handler = new FakeServiceHandler(wire) { Capabilities = FakeModels.ModelCapabilities }
            .On("Instantiate", Wire.InstantiateRequest.Parser, _ => new Wire.InstantiateResponse { Instance = root, Instances = { root, engine }, Diagnostics = { FakeModels.Diagnostic("warning", "w") } });
        await using var connection = await FakeConnection.OpenAsync(handler);

        var built = await connection.ModelByHash("h").InstantiateAsync("Demo::Vehicle", TestContext.Current.CancellationToken);

        Assert.Equal(new Wire.InstantiateRequest { ModelHash = "h", SymbolId = "Demo::Vehicle" }, Assert.Single(handler.RequestsFor("Instantiate")).Message(Wire.InstantiateRequest.Parser));
        Assert.Equal(1, built.Root.Id);
        Assert.Equal("Demo::Vehicle", built.Root.TypeSymbolId);
        Assert.Equal("Demo::Vehicle#1", built.Root.ToString());
        Assert.Equal(["a", "engine", "mass", "spare", "wheels"], built.Root.FeatureValues.Keys);
        Assert.Equal(new FeatureValue("mass", new Value.RealValue(1500.0), [], true, string.Empty, 2), built.Root.FeatureValues["mass"]);
        Assert.Equal(new FeatureValue("wheels", null, [new Value.IntegerValue(1), new Value.IntegerValue(2)], true, string.Empty, 0), built.Root.FeatureValues["wheels"]);
        Assert.Equal(new FeatureValue("a", null, [], true, "cyclic dependency: a -> b -> a", 0), built.Root.FeatureValues["a"]);
        Assert.False(built.Root.FeatureValues["spare"].IsMaterialized);
        Assert.Equal([1L, 2L], built.Reachable.Select(instance => instance.Id));
        var reference = Assert.IsType<Value.InstanceRefValue>(built.Root.FeatureValues["engine"].Value);
        Assert.Equal(new Instance(2, "Demo::Engine", []), built.Resolve(reference));
        Assert.Same(built.Root, built.FindInstance(1));
        Assert.Null(built.FindInstance(9));
        Assert.Equal("w", Assert.Single(built.Diagnostics).Message);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AnInstantiationFailureIsInBandAndAMalformedAnswerNamesWhereItIs(ClientWire wire)
    {
        var bad = new Wire.Instance { Id = 1, TypeSymbolId = "P::Q" };
        bad.FeatureValues["n"] = new Wire.FeatureValue { FeatureName = "n", Value = new Wire.Value { BigIntValue = "007" } };
        using var handler = new FakeServiceHandler(wire) { Capabilities = FakeModels.ModelCapabilities }
            .On("Instantiate", Wire.InstantiateRequest.Parser, request => request.SymbolId switch
            {
                "P::Nope" => new Wire.InstantiateResponse { Error = "symbol not found: P::Nope", Diagnostics = { FakeModels.Diagnostic("error", "d") } },
                "P::Empty" => new Wire.InstantiateResponse(),
                _ => new Wire.InstantiateResponse { Instance = bad, Instances = { bad } },
            });
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash("h");

        var failed = await Assert.ThrowsAsync<ModelException>(() => model.InstantiateAsync("P::Nope", TestContext.Current.CancellationToken));
        var empty = await Assert.ThrowsAsync<TransportException>(() => model.InstantiateAsync("P::Empty", TestContext.Current.CancellationToken));
        var malformed = await Assert.ThrowsAsync<TransportException>(() => model.InstantiateAsync("P::Q", TestContext.Current.CancellationToken));

        Assert.Equal("symbol not found: P::Nope", failed.Message);
        Assert.Equal(Model.InstantiateMethod, failed.Method);
        Assert.Single(failed.Diagnostics);
        Assert.Equal(StatusCode.Internal, empty.StatusCode);
        Assert.Equal(StatusCode.Internal, malformed.StatusCode);
        Assert.Contains("instance.feature_values[\"n\"].value", malformed.Message, StringComparison.Ordinal);
    }

    [Fact]
    public void AnInstantiationsCollectionsAreCopies()
    {
        var reachable = new List<Instance> { new(1, "A", []) };
        var built = new Instantiation(reachable[0], reachable, []);

        reachable.Add(new Instance(2, "B", []));

        Assert.Single(built.Reachable);
        Assert.Throws<ArgumentNullException>(() => built.Resolve(null!));
        Assert.Throws<ArgumentException>(() => new Instance(1, "A", [KeyValuePair.Create("x", (FeatureValue)null!)]));
    }
}
