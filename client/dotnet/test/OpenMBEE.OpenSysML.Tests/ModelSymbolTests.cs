using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading.Tasks;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// GetSymbol through the typed API, and name lookup over it and Query: every field of a symbol read,
/// a symbol the model lacks told apart from a model the service lacks, the outermost of several
/// homonyms chosen, a model without Query walked, and typos answered with the closest declared names.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed class ModelSymbolTests
{
    public static TheoryData<ClientWire> Wires => ClientWires.All;

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task SymbolAsyncReadsEveryFieldOfTheSymbol(ClientWire wire)
    {
        var info = FakeModels.Symbol("Demo::Vehicle", "Vehicle", "partDef", "Demo::Vehicle::mass", "Demo::Vehicle::engine");
        info.Metadata["type"] = "Base";
        info.Metadata["specializes"] = "Base";
        info.Attributes.Add(new Wire.AttributeInfo { Name = "mass", Type = "Real", Value = new Wire.Value { RealValue = 1500.0 }, Unit = "kg" });
        info.Attributes.Add(new Wire.AttributeInfo { Name = "label" });
        info.TypeInfo = new Wire.TypeInfo { Declared = "Base", ResolvedId = "Demo::Base", ResolvedKind = "partDef", Primitive = "Real", PrimitiveSource = "value", Quantity = true, Unit = "kg" };
        info.Multiplicity = new Wire.MultiplicityInfo { Lower = "0", Upper = "*" };
        info.Specializations.Add(new Wire.Specialization { Kind = "specializes", Declared = "Base", TargetId = "Demo::Base", TargetKind = "partDef" });
        info.Specializations.Add(new Wire.Specialization { Kind = "subsets", Declared = "nowhere" });
        info.WithheldLibraryAttributes = 4;
        using var handler = new FakeServiceHandler(wire) { Capabilities = FakeModels.ModelCapabilities }
            .Parsing("h", FakeModels.Symbol("r", string.Empty, "RootNamespace"))
            .On("GetSymbol", Wire.GetSymbolRequest.Parser, _ => new Wire.SymbolResponse { Symbol = info });
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = await connection.LoadAsync();

        var symbol = await model.SymbolAsync("Demo::Vehicle", TestContext.Current.CancellationToken);

        Assert.Equal(new Wire.GetSymbolRequest { ModelHash = "h", SymbolId = "Demo::Vehicle" }, Assert.Single(handler.RequestsFor("GetSymbol")).Message(Wire.GetSymbolRequest.Parser));
        Assert.Equal(
            new Symbol(
                "Demo::Vehicle",
                "Vehicle",
                "partDef",
                new Dictionary<string, string> { ["specializes"] = "Base", ["type"] = "Base" },
                ["Demo::Vehicle::mass", "Demo::Vehicle::engine"],
                [new AttributeInfo("mass", "Real", new Value.RealValue(1500.0), "kg"), new AttributeInfo("label", string.Empty, null, string.Empty)],
                new TypeFacts("Base", "Demo::Base", "partDef", "Real", "value", true, "kg"),
                new Multiplicity("0", "*"),
                [new Specialization("specializes", "Base", "Demo::Base", "partDef"), new Specialization("subsets", "nowhere", string.Empty, string.Empty)],
                4),
            symbol);
        Assert.Equal("Demo::Vehicle (partDef)", symbol.ToString());
        Assert.Equal("[0..*]", symbol.Multiplicity!.ToString());
        Assert.Equal("[?..1]", new Multiplicity(string.Empty, "1").ToString());
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ASymbolTheModelLacksIsNotFoundInBandAndAnAnswerOfNeitherIsMalformed(ClientWire wire)
    {
        using var handler = new FakeServiceHandler(wire) { Capabilities = FakeModels.ModelCapabilities }
            .Parsing("h", FakeModels.Symbol("r", string.Empty, "RootNamespace"))
            .On("GetSymbol", Wire.GetSymbolRequest.Parser, request => request.SymbolId == "Demo::Nope"
                ? new Wire.SymbolResponse { Error = "symbol not found: Demo::Nope" }
                : new Wire.SymbolResponse());
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = await connection.LoadAsync();

        var missing = await Assert.ThrowsAsync<SymbolNotFoundException>(() => model.SymbolAsync("Demo::Nope", TestContext.Current.CancellationToken));
        var found = await model.FindSymbolAsync("Demo::Nope", TestContext.Current.CancellationToken);
        var malformed = await Assert.ThrowsAsync<TransportException>(() => model.FindSymbolAsync("Demo::Empty", TestContext.Current.CancellationToken));

        Assert.Equal("Demo::Nope", missing.Name);
        Assert.True(missing.Suggestions.IsEmpty);
        Assert.Equal(Model.GetSymbolMethod, missing.Method);
        Assert.Equal("symbol not found: Demo::Nope", missing.Message);
        Assert.Null(found);
        Assert.Equal(StatusCode.Internal, malformed.StatusCode);
    }

    public static TheoryData<ClientWire, string> UnheldModelCalls()
    {
        var data = new TheoryData<ClientWire, string>();
        foreach (var wire in new[] { ClientWire.Grpc, ClientWire.ConnectProto, ClientWire.ConnectJson })
        {
            foreach (var method in new[] { "GetSymbol", "GetDiagnostics", "Evaluate", "Instantiate", "Query" })
            {
                data.Add(wire, method);
            }
        }

        return data;
    }

    [Theory]
    [MemberData(nameof(UnheldModelCalls))]
    public async Task AModelTheServiceDoesNotHoldIsModelNotFound(ClientWire wire, string method)
    {
        using var handler = new FakeServiceHandler(wire) { Capabilities = FakeModels.ModelCapabilities }.Refusing(method, StatusCode.NotFound, "model not found: gone");
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash("gone");
        Func<Task> call = method switch
        {
            "GetSymbol" => () => model.SymbolAsync("A", TestContext.Current.CancellationToken),
            "GetDiagnostics" => () => model.GetDiagnosticsAsync(cancellationToken: TestContext.Current.CancellationToken),
            "Evaluate" => () => model.EvalAsync("1", cancellationToken: TestContext.Current.CancellationToken),
            "Instantiate" => () => model.InstantiateAsync("A", TestContext.Current.CancellationToken),
            _ => () => model.FindAsync("A", TestContext.Current.CancellationToken),
        };

        var error = await Assert.ThrowsAsync<ModelNotFoundException>(call);

        Assert.Equal(StatusCode.NotFound, error.StatusCode);
        Assert.Equal("model not found: gone", error.ServiceMessage);
        Assert.Equal($"sysml.SysMLService/{method}", error.Method);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ModelByHashSendsNothingUntilItsFirstCall(ClientWire wire)
    {
        using var handler = new FakeServiceHandler(wire) { Capabilities = FakeModels.ModelCapabilities };
        await using var connection = await FakeConnection.OpenAsync(handler);

        var model = connection.ModelByHash("kept");

        Assert.Equal("kept", model.Hash);
        Assert.True(model.Roots.IsEmpty);
        Assert.Null(model.Root);
        Assert.True(model.ParseDiagnostics.IsEmpty);
        Assert.True(model.Documents.IsEmpty);
        Assert.Same(model, model.RequireNoErrors());
        Assert.Throws<ArgumentException>(() => connection.ModelByHash(" "));
        Assert.Throws<ArgumentNullException>(() => connection.ModelByHash(null!));
        Assert.Equal(["GetServerInfo"], handler.Requests.Select(request => request.Method));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task FindAsyncAcceptsARootAnIdOrAShortNameAndPrefersTheOutermost(ClientWire wire)
    {
        var tree = new FakeModelTree().WithRoot("doc")
            .With("doc", "Outer", "Outer", "package")
            .With("Outer", "Outer::Inner", "Inner", "package")
            .With("Outer::Inner", "Outer::Inner::Wheel", "Wheel")
            .With("Outer", "Outer::Wheel", "Wheel")
            .WithAlias("Elsewhere::Wheel", "Outer::Wheel");
        using var handler = tree.Serve(new FakeServiceHandler(wire) { Capabilities = FakeModels.ModelCapabilities }, "h");
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = await connection.LoadAsync();

        Assert.Equal("doc", (await model.FindAsync("doc", TestContext.Current.CancellationToken))!.Id);
        Assert.Equal("Outer::Wheel", (await model.FindAsync("Wheel", TestContext.Current.CancellationToken))!.Id);
        Assert.Equal("Outer::Inner::Wheel", (await model.FindAsync("Outer::Inner::Wheel", TestContext.Current.CancellationToken))!.Id);
        Assert.Null(await model.FindAsync("Elsewhere::Wheel", TestContext.Current.CancellationToken));
        Assert.Null(await model.FindAsync("Nothing", TestContext.Current.CancellationToken));
        await Assert.ThrowsAsync<ArgumentException>(() => model.FindAsync(string.Empty, TestContext.Current.CancellationToken));

        // Two homonyms, so their owners not yet known were asked for together, by id, in ordinal order.
        var asked = handler.RequestsFor("Query").Select(sent => sent.Message(Wire.QueryRequest.Parser).Query).ToList();
        Assert.Contains(asked, query => query.Where.Primitive.Property == "name" && query.Where.Primitive.Value.SequenceEqual(["Wheel"]) && query.Select.SequenceEqual(["owner"]));
        Assert.Contains(asked, query => query.Where.Primitive.Property == "@id" && query.Where.Primitive.Value.SequenceEqual(["Outer", "Outer::Inner"]));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task WithoutQueryTheModelIsWalkedFromItsRoots(ClientWire wire)
    {
        var tree = new FakeModelTree().WithRoot("doc")
            .With("doc", "Outer", "Outer", "package")
            .With("Outer", "Outer::Inner", "Inner", "package")
            .With("Outer::Inner", "Outer::Inner::Wheel", "Wheel")
            .With("Outer", "Outer::Wheel", "Wheel");
        using var handler = tree.Serve(new FakeServiceHandler(wire) { Capabilities = [.. FakeModels.ModelCapabilities.Except([Capabilities.Query])] }, "h");
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = await connection.LoadAsync();

        var wheel = await model.FindAsync("Wheel", TestContext.Current.CancellationToken);
        var typo = await Assert.ThrowsAsync<SymbolNotFoundException>(() => model.LookupAsync("Wheal", TestContext.Current.CancellationToken));
        var byHash = await Assert.ThrowsAsync<CapabilityException>(() => connection.ModelByHash("h").FindAsync("Wheel", TestContext.Current.CancellationToken));

        Assert.Equal("Outer::Wheel", wheel!.Id);
        Assert.Equal(["Wheel"], typo.Suggestions);
        Assert.Equal(Capabilities.Query, byHash.Capability);
        Assert.Empty(handler.RequestsFor("Query"));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task LookupAsyncNamesTheClosestDeclaredNames(ClientWire wire)
    {
        var tree = new FakeModelTree().WithRoot("doc")
            .With("doc", "Demo", "Demo", "package")
            .With("Demo", "Demo::Vehicle", "Vehicle")
            .With("Demo", "Demo::Engine", "Engine")
            .With("Demo::Vehicle", "Demo::Vehicle::mass", "mass", "attributeUsage");
        using var handler = tree.Serve(new FakeServiceHandler(wire) { Capabilities = FakeModels.ModelCapabilities }, "h");
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = await connection.LoadAsync();

        var found = await model.LookupAsync("Vehicle", TestContext.Current.CancellationToken);
        var typo = await Assert.ThrowsAsync<SymbolNotFoundException>(() => model.LookupAsync("Vehicel", TestContext.Current.CancellationToken));
        var qualified = await Assert.ThrowsAsync<SymbolNotFoundException>(() => model.LookupAsync("Demo::Vehicl", TestContext.Current.CancellationToken));
        var nothing = await Assert.ThrowsAsync<SymbolNotFoundException>(() => model.LookupAsync("zzz", TestContext.Current.CancellationToken));

        Assert.Equal("Demo::Vehicle", found.Id);
        Assert.Equal(["Vehicle", "Demo::Vehicle"], typo.Suggestions);
        Assert.Equal("No symbol named 'Vehicel' in this model; did you mean 'Vehicle', 'Demo::Vehicle'?", typo.Message);
        Assert.Equal(["Demo::Vehicle", "Demo::Vehicle::mass", "Vehicle"], qualified.Suggestions);
        Assert.True(nothing.Suggestions.IsEmpty);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AModelWithErrorsIsWalkedAsDeepAsTheQuerysBestAnswer(ClientWire wire)
    {
        // The query sees only the deep Wheel; the shallow one, which a model with errors may hold, is found by the walk.
        var tree = new FakeModelTree().WithRoot("doc")
            .With("doc", "A", "A", "package")
            .With("A", "A::B", "B", "package")
            .With("A::B", "A::B::Wheel", "Wheel")
            .With("A", "A::hidden", "Wheel", "partUsage", queryable: false);
        using var handler = tree.Serve(new FakeServiceHandler(wire) { Capabilities = FakeModels.ModelCapabilities }, "h");
        handler.Parsing("h", tree.Root.Clone(), FakeModels.Diagnostic("error", "unresolved"));
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = await connection.LoadAsync();

        var wheel = await model.FindAsync("Wheel", TestContext.Current.CancellationToken);

        Assert.True(model.HasErrors);
        Assert.Equal("A::hidden", wheel!.Id);
    }

    [Fact]
    public async Task OwnersThatFormACycleAreAMalformedAnswer()
    {
        static Wire.QueryResultElement Owned(string id, string owner) => new() { Id = id, Type = "PartUsage", Properties = { ["owner"] = owner } };
        using var handler = new FakeServiceHandler { Capabilities = FakeModels.ModelCapabilities }
            .Parsing("h", FakeModels.Symbol("doc", string.Empty, "RootNamespace"))
            .On("Query", Wire.QueryRequest.Parser, request => request.Query.Where.Primitive.Property == "name"
                ? new Wire.QueryResponse { Elements = { Owned("A::x", "A"), Owned("B::x", "B") } }
                : new Wire.QueryResponse { Elements = { Owned("A", "B"), Owned("B", "A") } });
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = await connection.LoadAsync();

        var error = await Assert.ThrowsAsync<TransportException>(() => model.FindAsync("x", TestContext.Current.CancellationToken));

        Assert.Equal(StatusCode.Internal, error.StatusCode);
        Assert.Equal(Model.QueryMethod, error.Method);
        Assert.Contains("cycle", error.Message, StringComparison.Ordinal);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ConcurrentReadsOfOneModelEachGetTheirOwnAnswer(ClientWire wire)
    {
        using var handler = new FakeServiceHandler(wire) { Capabilities = FakeModels.ModelCapabilities }
            .Parsing("h", FakeModels.Symbol("r", string.Empty, "RootNamespace"))
            .On("GetSymbol", Wire.GetSymbolRequest.Parser, request => new Wire.SymbolResponse { Symbol = FakeModels.Symbol(request.SymbolId, request.SymbolId, "partDef") })
            .On("Evaluate", Wire.EvaluateRequest.Parser, request => new Wire.EvaluateResponse { Result = new Wire.Value { StringValue = request.Expression } });
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = await connection.LoadAsync();
        var hash = model.Hash;

        var symbols = await Task.WhenAll(Enumerable.Range(0, 32).Select(i => model.SymbolAsync($"S{i}", TestContext.Current.CancellationToken)));
        var values = await Task.WhenAll(Enumerable.Range(0, 32).Select(i => model.EvalAsync($"e{i}", cancellationToken: TestContext.Current.CancellationToken)));

        Assert.Equal(Enumerable.Range(0, 32).Select(i => $"S{i}"), symbols.Select(symbol => symbol.Id));
        Assert.Equal(Enumerable.Range(0, 32).Select(i => new Value.TextValue($"e{i}")), values);
        Assert.All(handler.RequestsFor("GetSymbol"), sent => Assert.Equal(hash, sent.Message(Wire.GetSymbolRequest.Parser).ModelHash));
        Assert.Equal(hash, model.Hash);
    }
}
