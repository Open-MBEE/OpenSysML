using System;
using System.Collections.Concurrent;
using System.IO;
using System.Linq;
using System.Net.Http;
using System.Threading;
using System.Threading.Tasks;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// Models against a real sysml-grpc over every client wire, on the conformance fixtures: parsing inline,
/// from a file and as several documents, symbols and name lookup, diagnostics under a lint selection,
/// evaluation and instantiation, ownership of a one-shot model, and, against a service withholding the
/// capabilities these calls gate on, refusals made before anything is sent.
/// </summary>
[Trait(Category.Name, Category.Integration)]
public sealed class ServiceModelTests(ServiceFixture services) : IClassFixture<ServiceFixture>
{
    private const string Withheld = "strict_conformance,inline_language,parse_sources,parse_sources_affected,evaluate_subject,query";

    public static TheoryData<ClientWire> Wires => ClientWires.All;

    private static CancellationToken Token => TestContext.Current.CancellationToken;

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ParsesInlineNotationAndAFileTheServiceReads(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var path = Fixture("vehicle.sysml");

        var inline = await connection.LoadContentAsync(File.ReadAllText(path), cancellationToken: Token);
        var file = await connection.LoadAsync(path, cancellationToken: Token);
        var strict = await connection.LoadContentAsync(File.ReadAllText(path), new ParseOptions { StrictConformance = true, Language = SourceLanguage.SysML }, Token);

        Assert.NotEmpty(inline.Hash);
        Assert.Equal("RootNamespace", inline.Root!.Kind);
        Assert.True(inline.ParseDiagnostics.IsEmpty);
        Assert.True(inline.Documents.IsEmpty);
        Assert.Equal([path], file.Documents);
        Assert.Same(file, file.RequireNoErrors());
        Assert.True(strict.ParseDiagnostics.IsEmpty);
        Assert.NotEqual(inline.Hash, strict.Hash);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ASyntaxErrorIsADiagnosticAndAnUnreadableFileIsNotFound(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);

        var broken = await connection.LoadContentAsync(File.ReadAllText(Fixture("syntax_error.sysml")), cancellationToken: Token);
        var file = await Assert.ThrowsAsync<ModelFileNotFoundException>(() => connection.LoadAsync("/nonexistent/path/to/file.sysml", cancellationToken: Token));
        var document = await Assert.ThrowsAsync<ModelFileNotFoundException>(() => connection.ParseSourcesAsync(
            [SourceDocument.Inline(File.ReadAllText(Fixture("engine_library.sysml")), "engine_library.sysml"), SourceDocument.FromFile("/nonexistent/path/to/file.sysml")],
            cancellationToken: Token));

        Assert.NotEmpty(broken.Hash);
        Assert.True(broken.HasErrors);
        Assert.Equal(DiagnosticSeverity.Error, broken.Errors[0].Severity);
        Assert.Equal(broken.Errors, Assert.Throws<ModelErrorsException>(() => broken.RequireNoErrors()).Diagnostics);
        Assert.Equal(StatusCode.NotFound, file.StatusCode);
        Assert.Contains("/nonexistent/path/to/file.sysml", file.ServiceMessage, StringComparison.Ordinal);
        Assert.Equal(Connection.ParseSourcesMethod, document.Method);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task SeveralDocumentsParseAsOneModel(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var library = File.ReadAllText(Fixture("engine_library.sysml"));
        var user = File.ReadAllText(Fixture("engine_user.sysml"));

        var model = await connection.ParseSourcesAsync([SourceDocument.Inline(library, "engine_library.sysml"), SourceDocument.Inline(user, "engine_user.sysml")], cancellationToken: Token);
        var unchanged = await connection.ParseSourcesAsync(
            [SourceDocument.Inline(library, "engine_library.sysml"), SourceDocument.Inline(user, "engine_user.sysml")],
            new SourcesOptions { BaseModelHash = model.Hash },
            Token);
        var edited = await connection.ParseSourcesAsync(
            [SourceDocument.Inline(library, "engine_library.sysml"), SourceDocument.Inline(user + "\n// edited", "engine_user.sysml")],
            new SourcesOptions { BaseModelHash = model.Hash },
            Token);
        var duplicate = await Assert.ThrowsAsync<ServiceException>(() =>
            connection.ParseSourcesAsync([SourceDocument.Inline(library, "same.sysml"), SourceDocument.Inline(user, "same.sysml")], cancellationToken: Token));
        var none = await Assert.ThrowsAsync<ServiceException>(() => connection.ParseSourcesAsync([], cancellationToken: Token));

        Assert.Equal(["RootNamespace", "RootNamespace"], model.Roots.Select(root => root.Kind));
        Assert.True(model.ParseDiagnostics.IsEmpty);
        Assert.Equal(["engine_library.sysml", "engine_user.sysml"], model.Documents);
        Assert.Equal("EngineUser::Car::motor", (await model.SymbolAsync("EngineUser::Car::motor", Token)).Id);
        Assert.Equal(model.Hash, unchanged.Hash);
        Assert.Subset(unchanged.Documents.ToHashSet(), unchanged.Affected.ToHashSet());
        Assert.Equal(model.Hash, edited.BaseModelHash);
        Assert.Contains("engine_user.sysml", edited.Affected);
        Assert.Subset(edited.Documents.ToHashSet(), edited.Affected.ToHashSet());
        Assert.Equal(StatusCode.InvalidArgument, duplicate.StatusCode);
        Assert.Equal(StatusCode.InvalidArgument, none.StatusCode);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task SymbolsAreReadAndFoundByName(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var model = await LoadAsync(connection, "vehicle.sysml");

        var vehicle = await model.SymbolAsync("Demo::Vehicle", Token);
        var mass = await model.SymbolAsync("Demo::Vehicle::mass", Token);
        var found = await model.FindAsync("Vehicle", Token);
        var typo = await Assert.ThrowsAsync<SymbolNotFoundException>(() => model.LookupAsync("Vehicel", Token));
        var missing = await Assert.ThrowsAsync<SymbolNotFoundException>(() => model.SymbolAsync("Demo::Nope", Token));

        Assert.Equal(("Demo::Vehicle", "Vehicle", "partDef"), (vehicle.Id, vehicle.Name, vehicle.Kind));
        Assert.Contains("Demo::Vehicle::engine", vehicle.ChildIds);
        Assert.Equal(new Value.RealValue(1500.0), vehicle.Attributes.Single(attribute => attribute.Name == "mass").Value);
        Assert.Equal(("Demo::Vehicle::mass", "attributeUsage"), (mass.Id, mass.Kind));
        Assert.Equal(vehicle, found);
        Assert.Equal(vehicle, await model.LookupAsync("Demo::Vehicle", Token));
        Assert.Contains("Vehicle", typo.Suggestions);
        Assert.Null(await model.FindSymbolAsync("Demo::Nope", Token));
        Assert.Equal("Demo::Nope", missing.Name);
        Assert.Contains("symbol not found", missing.Message, StringComparison.Ordinal);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AModelTheServiceDoesNotHoldIsModelNotFound(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var model = connection.ModelByHash("no-such-model");

        var symbol = await Assert.ThrowsAsync<ModelNotFoundException>(() => model.SymbolAsync("Demo::Vehicle", Token));
        var diagnostics = await Assert.ThrowsAsync<ModelNotFoundException>(() => model.GetDiagnosticsAsync(cancellationToken: Token));
        var evaluation = await Assert.ThrowsAsync<ModelNotFoundException>(() => model.EvalAsync("2 + 2", cancellationToken: Token));
        var instance = await Assert.ThrowsAsync<ModelNotFoundException>(() => model.InstantiateAsync("Demo::Vehicle", Token));
        var query = await Assert.ThrowsAsync<ModelNotFoundException>(() => model.FindAsync("Vehicle", Token));

        Assert.All([symbol, diagnostics, evaluation, instance, query], error => Assert.Contains("no-such-model", error.ServiceMessage, StringComparison.Ordinal));
        Assert.Equal(Model.QueryMethod, query.Method);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task DiagnosticsFollowTheLintSelection(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var model = await LoadAsync(connection, "lints.sysml");

        var defaults = await model.GetDiagnosticsAsync(cancellationToken: Token);
        var enabled = await model.GetDiagnosticsAsync(new DiagnosticsOptions { EnableLint = ["rounded-real-literal"] }, Token);
        var disabled = await model.GetDiagnosticsAsync(new DiagnosticsOptions { DisableLint = ["undeclared-signal"] }, Token);
        var unknown = await Assert.ThrowsAsync<ServiceException>(() => model.GetDiagnosticsAsync(new DiagnosticsOptions { DisableLint = ["no-such-lint"] }, Token));

        Assert.Equal(["undeclared-signal"], defaults.Select(diagnostic => diagnostic.Code));
        Assert.Equal(["rounded-real-literal", "undeclared-signal"], enabled.Select(diagnostic => diagnostic.Code).Order(StringComparer.Ordinal));
        Assert.Empty(disabled);
        Assert.Equal(StatusCode.InvalidArgument, unknown.StatusCode);
        Assert.Contains("no-such-lint", unknown.ServiceMessage, StringComparison.Ordinal);
        Assert.Equal(model.ParseDiagnostics, defaults);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ExpressionsEvaluateInTheirScopeAndAgainstASubject(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var model = await LoadAsync(connection, "vehicle.sysml");

        Assert.Equal(new Value.IntegerValue(4), await model.EvalAsync("2 + 2", cancellationToken: Token));
        Assert.Equal(new Value.BooleanValue(false), await model.EvalAsync("true and false", cancellationToken: Token));
        Assert.Equal(new Value.RealValue(1500.0), await model.EvalAsync("mass", new EvalOptions { ContextSymbolId = "Demo::Vehicle" }, Token));
        var subject = await model.EvaluateAsync("mass", new EvalOptions { SubjectSymbolId = "Demo::sedan" }, Token);
        var absent = await Assert.ThrowsAsync<ModelException>(() => model.EvalAsync("mass", new EvalOptions { SubjectSymbolId = "Demo::nope" }, Token));
        var trailing = await Assert.ThrowsAsync<ModelException>(() => model.EvalAsync("invalid syntax (((", cancellationToken: Token));
        var unparsable = await Assert.ThrowsAsync<ModelException>(() => model.EvalAsync("(((", cancellationToken: Token));

        Assert.Equal(new Value.RealValue(1200.0), subject.Result);
        Assert.Empty(subject.Diagnostics);
        Assert.Contains("Demo::nope", absent.Message, StringComparison.Ordinal);
        Assert.Contains("expression parse failed", trailing.Message, StringComparison.Ordinal);
        Assert.Equal(Model.EvaluateMethod, unparsable.Method);
        Assert.NotEmpty(unparsable.Diagnostics);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task InstantiationBuildsTheObjectGraphAndResolvesItsReferences(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var vehicle = await LoadAsync(connection, "vehicle.sysml");
        var simple = await LoadAsync(connection, "simple_part.sysml");
        var cyclic = await LoadAsync(connection, "cyclic.sysml");

        var built = await vehicle.InstantiateAsync("Demo::Vehicle", Token);
        var part = await simple.InstantiateAsync("Test::SimplePart", Token);
        var cycle = await cyclic.InstantiateAsync("Demo::Cyclic", Token);
        var unknown = await Assert.ThrowsAsync<ModelException>(() => simple.InstantiateAsync("Test::DoesNotExist", Token));

        Assert.Equal("Demo::Vehicle", built.Root.TypeSymbolId);
        Assert.Equal(new Value.RealValue(1500.0), built.Root.FeatureValues["mass"].Value);
        var engine = built.Resolve(Assert.IsType<Value.InstanceRefValue>(built.Root.FeatureValues["engine"].Value));
        Assert.NotNull(engine);
        Assert.True(built.Reachable.Count >= 2);
        Assert.Contains(built.Root, built.Reachable);
        Assert.Equal(new FeatureValue("mass", new Value.IntegerValue(100), [], true, string.Empty, 0), part.Root.FeatureValues["mass"]);
        Assert.Contains("cyclic", cycle.Root.FeatureValues["a"].Error, StringComparison.Ordinal);
        Assert.Contains("not found", unknown.Message, StringComparison.Ordinal);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AOneShotModelOwnsItsConnectionAndConcurrentReadsAgree(ClientWire wire)
    {
        var service = await services.ServiceAsync("connect");
        var model = await OpenSysMLClient.LoadContentAsync(File.ReadAllText(Fixture("vehicle.sysml")), wire.To(service.Address), cancellationToken: Token);
        var ids = new[] { "Demo::Vehicle", "Demo::Engine", "Demo::sedan", "Demo::Vehicle::mass" };

        var sequential = (await Task.WhenAll(ids.Select(id => model.SymbolAsync(id, Token)))).ToList();
        var concurrent = await Task.WhenAll(Enumerable.Range(0, 24).Select(i => model.SymbolAsync(ids[i % ids.Length], Token)));
        await model.DisposeAsync();

        Assert.True(model.OwnsConnection);
        Assert.Equal(Enumerable.Range(0, 24).Select(i => sequential[i % ids.Length]), concurrent);
        await Assert.ThrowsAsync<ObjectDisposedException>(() => model.SymbolAsync("Demo::Vehicle", Token));
        Assert.False(service.HasExited);
        await using var again = await OpenAsync(wire);
        Assert.Equal("Demo::Vehicle", (await again.ModelByHash(model.Hash).SymbolAsync("Demo::Vehicle", Token)).Id);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AWithheldCapabilityIsRefusedBeforeAnythingIsSent(ClientWire wire)
    {
        var service = await services.ServiceAsync("connect", Withheld);
        using var counting = new CountingHandler();
        await using var connection = await Connection.OpenAsync(
            new ConnectionOptions { Address = service.Address, Protocol = wire.Protocol(), Encoding = wire.Encoding(), HttpMessageHandler = counting },
            Token);
        var vehicle = File.ReadAllText(Fixture("vehicle.sysml"));

        var strict = await Assert.ThrowsAsync<CapabilityException>(() => connection.LoadContentAsync(vehicle, new ParseOptions { StrictConformance = true }, Token));
        var language = await Assert.ThrowsAsync<CapabilityException>(() => connection.LoadContentAsync(vehicle, new ParseOptions { Language = SourceLanguage.SysML }, Token));
        var sources = await Assert.ThrowsAsync<CapabilityException>(() => connection.ParseSourcesAsync([SourceDocument.Inline(vehicle)], cancellationToken: Token));
        var byHash = await Assert.ThrowsAsync<CapabilityException>(() => connection.ModelByHash("h").FindAsync("Vehicle", Token));
        Assert.Equal(["GetServerInfo"], counting.Methods);

        var model = await connection.LoadContentAsync(vehicle, cancellationToken: Token);
        var subject = await Assert.ThrowsAsync<CapabilityException>(() => model.EvalAsync("mass", new EvalOptions { SubjectSymbolId = "Demo::sedan" }, Token));
        Assert.Equal(["GetServerInfo", "ParseFile"], counting.Methods);

        // Without query, a name is found by walking the model from its root.
        var found = await model.FindAsync("Vehicle", Token);
        var typo = await Assert.ThrowsAsync<SymbolNotFoundException>(() => model.LookupAsync("Vehicel", Token));

        Assert.Equal(
            [Capabilities.StrictConformance, Capabilities.InlineLanguage, Capabilities.ParseSources, Capabilities.Query, Capabilities.EvaluateSubject],
            new[] { strict, language, sources, byHash, subject }.Select(error => error.Capability));
        Assert.Equal("Demo::Vehicle", found!.Id);
        Assert.Contains("Vehicle", typo.Suggestions);
        Assert.DoesNotContain("Query", counting.Methods);
    }

    private static string Fixture(string name) => Repository.PathOf("conformance", "fixtures", name);

    private static async Task<Model> LoadAsync(Connection connection, string fixture) =>
        (await connection.LoadContentAsync(File.ReadAllText(Fixture(fixture)), cancellationToken: Token)).RequireNoErrors();

    private async Task<Connection> OpenAsync(ClientWire wire)
    {
        var service = await services.ServiceAsync("connect");
        return await Connection.OpenAsync(wire.To(service.Address), Token);
    }

    /// <summary>A borrowed handler that records the method of every request it carries to the service.</summary>
    private sealed class CountingHandler() : DelegatingHandler(new SocketsHttpHandler())
    {
        private readonly ConcurrentQueue<string> methods = new();

        internal string[] Methods => [.. methods];

        protected override Task<HttpResponseMessage> SendAsync(HttpRequestMessage request, CancellationToken cancellationToken)
        {
            methods.Enqueue(request.RequestUri!.AbsolutePath.Split('/')[^1]);
            return base.SendAsync(request, cancellationToken);
        }
    }
}
