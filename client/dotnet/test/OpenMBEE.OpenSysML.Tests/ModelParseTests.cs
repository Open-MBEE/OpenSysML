using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading.Tasks;
using OpenMBEE.OpenSysML.Internal;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// ParseFile and ParseSources through the typed API, over every wire: the exact request each sends,
/// every field of the answer read into the model, the capability gates checked in the service's order
/// before anything is sent, and a parse's failures told apart by kind.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed class ModelParseTests
{
    private static readonly Wire.Span At = new() { File = "a.sysml", StartLine = 3, StartCol = 5, EndLine = 3, EndCol = 9 };

    public static TheoryData<ClientWire> Wires => ClientWires.All;

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task LoadSendsThePathAloneAndReadsTheParse(ClientWire wire)
    {
        using var handler = new FakeServiceHandler(wire) { Capabilities = FakeModels.ModelCapabilities }
            .Parsing("h1", FakeModels.Symbol("a.sysml", string.Empty, "RootNamespace", "P"), FakeModels.Diagnostic("warning", "odd", "lint", At), FakeModels.Diagnostic("error", "bad", "syntax"));
        await using var connection = await FakeConnection.OpenAsync(handler);

        var model = await connection.LoadAsync("models/a.sysml", cancellationToken: TestContext.Current.CancellationToken);

        Assert.Equal(new Wire.ParseFileRequest { FilePath = "models/a.sysml" }, Assert.Single(handler.RequestsFor("ParseFile")).Message(Wire.ParseFileRequest.Parser));
        Assert.Equal("h1", model.Hash);
        Assert.Same(connection, model.Connection);
        Assert.False(model.OwnsConnection);
        Assert.Equal(new Symbol("a.sysml", string.Empty, "RootNamespace", [], ["P"], [], null, null, [], 0), Assert.Single(model.Roots));
        Assert.Same(model.Roots[0], model.Root);
        Assert.Equal(["models/a.sysml"], model.Documents);
        Assert.Equal(["warning", "error"], model.ParseDiagnostics.Select(diagnostic => diagnostic.SeverityName));
        Assert.Equal("bad", Assert.Single(model.Errors).Message);
        Assert.True(model.HasErrors);
        Assert.Null(model.BaseModelHash);
        Assert.True(model.Affected.IsEmpty);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task LoadContentSendsTheContentAndItsLanguage(ClientWire wire)
    {
        using var handler = new FakeServiceHandler(wire) { Capabilities = FakeModels.ModelCapabilities }.Parsing("h2", FakeModels.Symbol("r", string.Empty, "RootNamespace"));
        await using var connection = await FakeConnection.OpenAsync(handler);

        var plain = await connection.LoadContentAsync("package P;", cancellationToken: TestContext.Current.CancellationToken);
        await connection.LoadContentAsync("package Q;", new ParseOptions { Language = SourceLanguage.KerML, StrictConformance = true }, TestContext.Current.CancellationToken);
        await connection.LoadContentAsync("package R;", new ParseOptions { Language = SourceLanguage.SysML }, TestContext.Current.CancellationToken);

        Assert.Equal(
            [
                new Wire.ParseFileRequest { Content = "package P;" },
                new Wire.ParseFileRequest { Content = "package Q;", Language = "kerml", StrictConformance = true },
                new Wire.ParseFileRequest { Content = "package R;", Language = "sysml" },
            ],
            handler.RequestsFor("ParseFile").Select(sent => sent.Message(Wire.ParseFileRequest.Parser)));
        Assert.True(plain.Documents.IsEmpty);
        Assert.False(plain.HasErrors);
        Assert.Same(plain, plain.RequireNoErrors());
    }

    [Fact]
    public async Task LoadingAFileWithALanguageOrAnUnknownLanguageIsRefusedBeforeSending()
    {
        using var handler = new FakeServiceHandler { Capabilities = FakeModels.ModelCapabilities };
        await using var connection = await FakeConnection.OpenAsync(handler);

        var file = await Assert.ThrowsAsync<InvalidRequestException>(() =>
            connection.LoadAsync("a.kerml", new ParseOptions { Language = SourceLanguage.KerML }, TestContext.Current.CancellationToken));
        await Assert.ThrowsAsync<InvalidRequestException>(() =>
            connection.LoadContentAsync("x", new ParseOptions { Language = (SourceLanguage)7 }, TestContext.Current.CancellationToken));
        await Assert.ThrowsAsync<ArgumentException>(() => connection.LoadAsync(string.Empty, cancellationToken: TestContext.Current.CancellationToken));
        await Assert.ThrowsAsync<ArgumentNullException>(() => connection.LoadContentAsync(null!, cancellationToken: TestContext.Current.CancellationToken));

        Assert.Contains("a.kerml is a file", file.Message, StringComparison.Ordinal);
        Assert.Equal(["GetServerInfo"], handler.Requests.Select(request => request.Method));
    }

    public static TheoryData<ClientWire, string[], string> ParseGates => new()
    {
        { ClientWire.Grpc, [Capabilities.StrictConformance], Capabilities.StrictConformance },
        { ClientWire.ConnectProto, [Capabilities.InlineLanguage], Capabilities.InlineLanguage },
        { ClientWire.ConnectJson, [Capabilities.StrictConformance, Capabilities.InlineLanguage], Capabilities.StrictConformance },
    };

    [Theory]
    [MemberData(nameof(ParseGates))]
    public async Task AParseOptionGateWithheldSendsNothing(ClientWire wire, string[] withheld, string refused)
    {
        using var handler = new FakeServiceHandler(wire) { Capabilities = [.. FakeModels.ModelCapabilities.Except(withheld)] };
        await using var connection = await FakeConnection.OpenAsync(handler);

        var error = await Assert.ThrowsAsync<CapabilityException>(() =>
            connection.LoadContentAsync("package P;", new ParseOptions { Language = SourceLanguage.KerML, StrictConformance = true }, TestContext.Current.CancellationToken));

        Assert.Equal(refused, error.Capability);
        Assert.Equal(["GetServerInfo"], handler.Requests.Select(request => request.Method));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ParseSourcesSendsEveryDocumentAndReadsEveryRoot(ClientWire wire)
    {
        using var handler = new FakeServiceHandler(wire) { Capabilities = FakeModels.ModelCapabilities };
        handler.On("ParseSources", Wire.ParseSourcesRequest.Parser, _ => new Wire.ParseSourcesResponse
        {
            ModelHash = "multi",
            Roots = { FakeModels.Symbol("lib.sysml", string.Empty, "RootNamespace", "Lib"), FakeModels.Symbol("user", string.Empty, "RootNamespace") },
            Diagnostics = { FakeModels.Diagnostic("info", "note", span: At) },
            Affected = { "user" },
        });
        await using var connection = await FakeConnection.OpenAsync(handler);
        List<SourceDocument> documents = [SourceDocument.FromFile("lib.sysml"), SourceDocument.Inline("package U;", "user", SourceLanguage.KerML), SourceDocument.Inline("package V;")];

        var model = await connection.ParseSourcesAsync(documents, new SourcesOptions { StrictConformance = true, BaseModelHash = "before" }, TestContext.Current.CancellationToken);
        documents.Clear();

        var sent = Assert.Single(handler.RequestsFor("ParseSources")).Message(Wire.ParseSourcesRequest.Parser);
        Assert.Equal(
            new Wire.ParseSourcesRequest
            {
                Documents =
                {
                    new Wire.SourceDocument { FilePath = "lib.sysml" },
                    new Wire.SourceDocument { Content = "package U;", Name = "user", Language = "kerml" },
                    new Wire.SourceDocument { Content = "package V;" },
                },
                StrictConformance = true,
                BaseModelHash = "before",
            },
            sent);
        Assert.Equal("multi", model.Hash);
        Assert.Equal(["lib.sysml", "user"], model.Roots.Select(root => root.Id));
        Assert.Equal(["Lib"], model.Root!.ChildIds);
        Assert.Equal(["lib.sysml", "user", string.Empty], model.Documents);
        Assert.Equal(new SourceSpan("a.sysml", 3, 5, 3, 9), Assert.Single(model.ParseDiagnostics).Span);
        Assert.Equal("before", model.BaseModelHash);
        Assert.Equal(["user"], model.Affected);
    }

    public static TheoryData<string[], string> SourcesGates => new()
    {
        { [Capabilities.ParseSources, Capabilities.StrictConformance, Capabilities.ParseSourcesAffected, Capabilities.InlineLanguage], Capabilities.ParseSources },
        { [Capabilities.StrictConformance, Capabilities.ParseSourcesAffected, Capabilities.InlineLanguage], Capabilities.StrictConformance },
        { [Capabilities.ParseSourcesAffected, Capabilities.InlineLanguage], Capabilities.ParseSourcesAffected },
        { [Capabilities.InlineLanguage], Capabilities.InlineLanguage },
    };

    [Theory]
    [MemberData(nameof(SourcesGates))]
    public async Task ParseSourcesChecksItsGatesInTheServicesOrderAndSendsNothing(string[] withheld, string refused)
    {
        using var handler = new FakeServiceHandler { Capabilities = [.. FakeModels.ModelCapabilities.Except(withheld)] };
        await using var connection = await FakeConnection.OpenAsync(handler);

        var error = await Assert.ThrowsAsync<CapabilityException>(() => connection.ParseSourcesAsync(
            [SourceDocument.Inline("package A;", "a"), SourceDocument.Inline("package B;", "b", SourceLanguage.SysML)],
            new SourcesOptions { StrictConformance = true, BaseModelHash = "before" },
            TestContext.Current.CancellationToken));

        Assert.Equal(refused, error.Capability);
        Assert.Equal(["GetServerInfo"], handler.Requests.Select(request => request.Method));
    }

    [Fact]
    public async Task ParseSourcesRefusesWhatItCannotSendAndLeavesTheRestToTheService()
    {
        using var handler = new FakeServiceHandler { Capabilities = FakeModels.ModelCapabilities }
            .Refusing("ParseSources", StatusCode.InvalidArgument, "documents must name at least one document");
        await using var connection = await FakeConnection.OpenAsync(handler);

        await Assert.ThrowsAsync<InvalidRequestException>(() =>
            connection.ParseSourcesAsync([SourceDocument.Inline("x")], new SourcesOptions { BaseModelHash = string.Empty }, TestContext.Current.CancellationToken));
        await Assert.ThrowsAsync<ArgumentException>(() => connection.ParseSourcesAsync([null!], cancellationToken: TestContext.Current.CancellationToken));
        Assert.Throws<ArgumentException>(() => SourceDocument.Inline("x", string.Empty));
        Assert.Throws<ArgumentException>(() => SourceDocument.Inline("x", "n", (SourceLanguage)9));
        Assert.Throws<ArgumentException>(() => SourceDocument.FromFile(string.Empty));
        Assert.Equal("file a.sysml", SourceDocument.FromFile("a.sysml").ToString());
        Assert.Equal("inline \"lib\" (KerML)", SourceDocument.Inline("x", "lib", SourceLanguage.KerML).ToString());
        Assert.Equal("inline (unnamed)", SourceDocument.Inline("x").ToString());
        Assert.Equal(SourceDocument.Inline("x", "n"), SourceDocument.Inline("x", "n"));
        var refused = await Assert.ThrowsAsync<ServiceException>(() => connection.ParseSourcesAsync([], cancellationToken: TestContext.Current.CancellationToken));

        Assert.Equal(StatusCode.InvalidArgument, refused.StatusCode);
        Assert.Equal(new Wire.ParseSourcesRequest(), Assert.Single(handler.RequestsFor("ParseSources")).Message(Wire.ParseSourcesRequest.Parser));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AnInBandParseFailureIsAModelExceptionAndAnAnswerWithoutAHashIsMalformed(ClientWire wire)
    {
        using var handler = new FakeServiceHandler(wire) { Capabilities = FakeModels.ModelCapabilities }
            .On("ParseFile", Wire.ParseFileRequest.Parser, request => request.Content == "fails"
                ? new Wire.ParseFileResponse { Error = "the parser gave up", Diagnostics = { FakeModels.Diagnostic("error", "here") } }
                : new Wire.ParseFileResponse())
            .On("ParseSources", Wire.ParseSourcesRequest.Parser, _ => new Wire.ParseSourcesResponse { Error = "no model" });
        await using var connection = await FakeConnection.OpenAsync(handler);

        var parse = await Assert.ThrowsAsync<ModelException>(() => connection.LoadContentAsync("fails", cancellationToken: TestContext.Current.CancellationToken));
        var sources = await Assert.ThrowsAsync<ModelException>(() => connection.ParseSourcesAsync([SourceDocument.Inline("x")], cancellationToken: TestContext.Current.CancellationToken));
        var hashless = await Assert.ThrowsAsync<TransportException>(() => connection.LoadContentAsync("hashless", cancellationToken: TestContext.Current.CancellationToken));

        Assert.Equal("the parser gave up", parse.Message);
        Assert.Equal(Connection.ParseFileMethod, parse.Method);
        Assert.Equal("here", Assert.Single(parse.Diagnostics).Message);
        Assert.Equal(Connection.ParseSourcesMethod, sources.Method);
        Assert.Equal(StatusCode.Internal, hashless.StatusCode);
        Assert.Contains("neither a model hash nor a failure", hashless.Message, StringComparison.Ordinal);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AnUnreadableFileIsModelFileNotFound(ClientWire wire)
    {
        using var handler = new FakeServiceHandler(wire) { Capabilities = FakeModels.ModelCapabilities }
            .Refusing("ParseFile", StatusCode.NotFound, "file not found: open /no/a.sysml: no such file or directory")
            .Refusing("ParseSources", StatusCode.NotFound, "file not found: open /no/b.sysml: no such file or directory");
        await using var connection = await FakeConnection.OpenAsync(handler);

        var file = await Assert.ThrowsAsync<ModelFileNotFoundException>(() => connection.LoadAsync("/no/a.sysml", cancellationToken: TestContext.Current.CancellationToken));
        var sources = await Assert.ThrowsAsync<ModelFileNotFoundException>(() =>
            connection.ParseSourcesAsync([SourceDocument.FromFile("/no/b.sysml")], cancellationToken: TestContext.Current.CancellationToken));

        Assert.Equal(StatusCode.NotFound, file.StatusCode);
        Assert.Equal("file not found: open /no/a.sysml: no such file or directory", file.ServiceMessage);
        Assert.Equal(Connection.ParseFileMethod, file.Method);
        Assert.Equal(Connection.ParseSourcesMethod, sources.Method);
        Assert.Contains("/no/b.sysml", sources.Message, StringComparison.Ordinal);
    }

    [Fact]
    public async Task RequireNoErrorsReportsTheFirstThreeErrorsAndHowManyMore()
    {
        var errors = Enumerable.Range(1, 5).Select(i => FakeModels.Diagnostic("error", $"e{i}")).Prepend(FakeModels.Diagnostic("warning", "w")).ToArray();
        using var handler = new FakeServiceHandler { Capabilities = FakeModels.ModelCapabilities }.Parsing("h", FakeModels.Symbol("r", string.Empty, "RootNamespace"), errors);
        await using var connection = await FakeConnection.OpenAsync(handler);
        var loaded = await connection.LoadAsync("m.sysml", cancellationToken: TestContext.Current.CancellationToken);
        var inline = await connection.LoadAsync();

        var file = Assert.Throws<ModelErrorsException>(() => loaded.RequireNoErrors());
        var content = Assert.Throws<ModelErrorsException>(() => inline.RequireNoErrors());

        Assert.Equal("m.sysml has 5 error(s): e1; e2; e3; ... and 2 more", file.Message);
        Assert.Equal(["e1", "e2", "e3", "e4", "e5"], file.Diagnostics.Select(diagnostic => diagnostic.Message));
        Assert.Equal(Connection.ParseFileMethod, file.Method);
        Assert.StartsWith("the model has 5 error(s)", content.Message, StringComparison.Ordinal);
        Assert.Equal(5, loaded.Errors.Count);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task EveryFieldOfADiagnosticIsRead(ClientWire wire)
    {
        var choice = FakeModels.Diagnostic("warning", "alternatives conflict", "choice-point", At);
        choice.OrderDependent.Add(["x", "y"]);
        var independent = FakeModels.Diagnostic("info", "commute", "choice-point");
        independent.Independent = true;
        using var handler = new FakeServiceHandler(wire) { Capabilities = FakeModels.ModelCapabilities }
            .Parsing("h", FakeModels.Symbol("r", string.Empty, "RootNamespace"), choice, independent, FakeModels.Diagnostic("fatal", "new severity"));
        await using var connection = await FakeConnection.OpenAsync(handler);

        var diagnostics = (await connection.LoadAsync()).ParseDiagnostics;

        Assert.Equal(new Diagnostic("warning", "alternatives conflict", "choice-point", new SourceSpan("a.sysml", 3, 5, 3, 9), ["x", "y"], false), diagnostics[0]);
        Assert.Equal(DiagnosticSeverity.Warning, diagnostics[0].Severity);
        Assert.Equal("warning at a.sysml:3:5: alternatives conflict [choice-point]", diagnostics[0].ToString());
        Assert.True(diagnostics[1].Independent);
        Assert.Null(diagnostics[1].Span);
        Assert.Equal(DiagnosticSeverity.Info, diagnostics[1].Severity);
        Assert.Equal(DiagnosticSeverity.Unknown, diagnostics[2].Severity);
        Assert.Equal("fatal", diagnostics[2].SeverityName);
        Assert.Equal("fatal: new severity", diagnostics[2].ToString());
    }

    [Fact]
    public void AReadSymbolIsASnapshotTheWireMessageCannotChange()
    {
        var wire = FakeModels.Symbol("P::A", "A", "partDef", "P::A::x");
        wire.Metadata["type"] = "B";
        wire.Attributes.Add(new Wire.AttributeInfo { Name = "x", Value = new Wire.Value { IntValue = 1 } });
        var symbol = ModelWire.Symbol(wire, Model.GetSymbolMethod, "symbol");

        wire.Id = "P::Changed";
        wire.ChildIds.Add("P::A::y");
        wire.Metadata["type"] = "C";
        wire.Attributes[0].Value.IntValue = 2;
        var options = new List<string> { "undeclared-signal" };
        var selection = new DiagnosticsOptions { DisableLint = options };
        options.Add("rounded-real-literal");

        Assert.Equal("P::A", symbol.Id);
        Assert.Equal(["P::A::x"], symbol.ChildIds);
        Assert.Equal("B", symbol.Metadata["type"]);
        Assert.Equal(new Value.IntegerValue(1), symbol.Attributes[0].Value);
        Assert.Equal(["undeclared-signal"], selection.DisableLint);
    }

    [Fact]
    public async Task ADisposedConnectionRefusesEveryTypedCallBeforeItsOtherChecks()
    {
        using var handler = new FakeServiceHandler { Capabilities = [] }.Parsing("h", FakeModels.Symbol("r", string.Empty, "RootNamespace"));
        var connection = await FakeConnection.OpenAsync(handler);
        var model = await connection.LoadAsync();
        await connection.DisposeAsync();

        await Assert.ThrowsAsync<ObjectDisposedException>(() => connection.LoadContentAsync("x", new ParseOptions { StrictConformance = true }, TestContext.Current.CancellationToken));
        await Assert.ThrowsAsync<ObjectDisposedException>(() => connection.LoadAsync("a.sysml", cancellationToken: TestContext.Current.CancellationToken));
        await Assert.ThrowsAsync<ObjectDisposedException>(() => connection.ParseSourcesAsync([SourceDocument.Inline("x")], cancellationToken: TestContext.Current.CancellationToken));
        await Assert.ThrowsAsync<ObjectDisposedException>(() => model.EvalAsync("1", new EvalOptions { SubjectSymbolId = "P::a" }, TestContext.Current.CancellationToken));
        await Assert.ThrowsAsync<ObjectDisposedException>(() => model.SymbolAsync("P", TestContext.Current.CancellationToken));
        await Assert.ThrowsAsync<ObjectDisposedException>(() => model.FindAsync("P", TestContext.Current.CancellationToken));
        await Assert.ThrowsAsync<ObjectDisposedException>(() => model.GetDiagnosticsAsync(cancellationToken: TestContext.Current.CancellationToken));
        await Assert.ThrowsAsync<ObjectDisposedException>(() => model.InstantiateAsync("P::A", TestContext.Current.CancellationToken));
        Assert.Throws<ObjectDisposedException>(() => connection.ModelByHash("h"));
        Assert.Equal(["GetServerInfo", "ParseFile"], handler.Requests.Select(request => request.Method));
    }
}
