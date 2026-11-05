using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// ExportGraphs, ImportData and CompareResults through the typed API, over every wire: every field and option sent as the
/// schema writes it, every field of every answer read, a graph's form kept byte for byte, an import's new model told from
/// the one imported into and from a dry run that made none, the notation of a model of one document and of several told
/// apart, a refusal kept with what it reported, the gates and option shapes judged before anything is sent, statuses
/// mapped as the service means them, and a malformed answer refused rather than read.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed class ModelGraphDataTests : IDisposable
{
    private const string Hash = CompleteGraphDataWire.Hash;

    private readonly string directory = Directory.CreateTempSubdirectory("opensysml-import-").FullName;

    public static TheoryData<ClientWire> Wires => ClientWires.All;

    /// <summary>Each malformed import answer, by the first file's name its request is routed on, with what the refusal says and how the import is made.</summary>
    public static TheoryData<ClientWire, string> MalformedImports()
    {
        var data = new TheoryData<ClientWire, string>();
        foreach (var wire in new[] { ClientWire.Grpc, ClientWire.ConnectProto, ClientWire.ConnectJson })
        {
            foreach (var name in Malformed.Keys)
            {
                data.Add(wire, name);
            }
        }

        return data;
    }

    private static CancellationToken Token => TestContext.Current.CancellationToken;

    // Answers malformed in one way each, how the import that reads them is made, and what the refusal names.
    private static Dictionary<string, (Wire.ImportDataResponse Answer, Importing Import, string Says)> Malformed => new(StringComparer.Ordinal)
    {
        ["refusal beside files"] = (new() { Error = "no", Files = { CompleteGraphDataWire.Files } }, Importing.Hashed, "a refused import beside what it set"),
        ["refusal beside a model"] = (new() { Error = "no", ModelHash = "x" }, Importing.Hashed, "a refused import beside what it set"),
        ["refusal beside notation"] = (new() { Error = "no", Content = "x" }, Importing.Hashed, "a refused import beside what it set"),
        ["refusal beside documents"] = (new() { Error = "no", Documents = { new Wire.EditedDocument { Name = "a" } } }, Importing.Hashed, "a refused import beside what it set"),
        ["a file too few"] = (new() { ModelHash = "x" }, Importing.Hashed, "0 data file(s) for an import of 1"),
        ["a file too many"] = (new() { Files = { new Wire.ImportedFile { Name = "a file too many" }, new Wire.ImportedFile { Name = "b" } }, ModelHash = "x" }, Importing.Hashed, "2 data file(s) for an import of 1"),
        ["another file"] = (new() { Files = { new Wire.ImportedFile { Name = "b.csv" } }, ModelHash = "x" }, Importing.Hashed, "files[0] for \"b.csv\", though the data file imports[0] sent is \"another file\""),
        ["negative values"] = (new() { Files = { new Wire.ImportedFile { Name = "negative values", Values = -1 } }, ModelHash = "x" }, Importing.Hashed, "setting -1 value(s) on 0 element(s)"),
        ["negative elements"] = (new() { Files = { new Wire.ImportedFile { Name = "negative elements", Elements = -2 } }, ModelHash = "x" }, Importing.Hashed, "setting 0 value(s) on -2 element(s)"),
        ["no model"] = (new() { Files = { new Wire.ImportedFile { Name = "no model" } } }, Importing.Hashed, "neither the imported model's hash nor a refusal"),
        ["a dry run's model"] = (new() { Files = { new Wire.ImportedFile { Name = "a dry run's model" } }, ModelHash = "x" }, Importing.DryRun, "a model, notation or documents for a dry run"),
        ["a dry run's notation"] = (new() { Files = { new Wire.ImportedFile { Name = "a dry run's notation" } }, Content = "x" }, Importing.DryRun, "a model, notation or documents for a dry run"),
        ["a dry run's documents"] = (new() { Files = { new Wire.ImportedFile { Name = "a dry run's documents" } }, Documents = { new Wire.EditedDocument { Name = "a", Content = "x" } } }, Importing.DryRun, "a model, notation or documents for a dry run"),
        ["an unnamed document"] = (new() { Files = { new Wire.ImportedFile { Name = "an unnamed document" } }, ModelHash = "x", Documents = { new Wire.EditedDocument { Content = "x" } } }, Importing.Several, "documents[0] without its name"),
        ["a repeated document"] = (
            new() { Files = { new Wire.ImportedFile { Name = "a repeated document" } }, ModelHash = "x", Documents = { new Wire.EditedDocument { Name = "a" }, new Wire.EditedDocument { Name = "a" } } },
            Importing.Several,
            "documents[1] repeating the document \"a\""),
        ["notation of several"] = (new() { Files = { new Wire.ImportedFile { Name = "notation of several" } }, ModelHash = "x", Content = "x" }, Importing.Several, "notation in content for a model of several documents"),
        ["other documents"] = (
            new() { Files = { new Wire.ImportedFile { Name = "other documents" } }, ModelHash = "x", Content = "x", Documents = { new Wire.EditedDocument { Name = "a", Content = "y" } } },
            Importing.Hashed,
            "documents other than the one document of a single-document model"),
        ["two documents of one"] = (
            new() { Files = { new Wire.ImportedFile { Name = "two documents of one" } }, ModelHash = "x", Content = "x", Documents = { new Wire.EditedDocument { Name = "a", Content = "x" }, new Wire.EditedDocument { Name = "b", Content = "x" } } },
            Importing.Hashed,
            "documents other than the one document of a single-document model"),
        ["a document without its notation"] = (
            new() { Files = { new Wire.ImportedFile { Name = "a document without its notation" } }, ModelHash = "x", Documents = { new Wire.EditedDocument { Name = "a", Content = "x" } } },
            Importing.Loaded,
            "the edited document of a single-document model without its notation in content"),
        ["notation under the same hash"] = (new() { Files = { new Wire.ImportedFile { Name = "notation under the same hash" } }, ModelHash = Hash, Content = "x" }, Importing.Hashed, "edited notation for the model imported into, under its own hash"),
    };

    public void Dispose() => Directory.Delete(directory, recursive: true);

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ExportGraphsSendsTheSubjectAndKeepsTheFormByteForByte(ClientWire wire)
    {
        using var handler = Serving(wire).On("ExportGraphs", Wire.ExportGraphsRequest.Parser, _ => CompleteGraphDataWire.Graphs);
        await using var connection = await FakeConnection.OpenAsync(handler);

        var graphs = await connection.ModelByHash(Hash).ExportGraphsAsync("Démo::räce", Token);

        Assert.Equal(CompleteGraphDataWire.ExportRequest, Assert.Single(handler.RequestsFor("ExportGraphs")).Message(Wire.ExportGraphsRequest.Parser));
        Assert.Equal(new Graphs(CompleteGraphDataWire.Form, 1, "Démo::räce"), graphs);
        Assert.Equal(CompleteGraphDataWire.Form, graphs.Content);
        Assert.Equal(CompleteGraphDataWire.Form, graphs.ToString());
    }

    [Fact]
    public async Task ExportGraphsNeedsItsCapabilityAndASubjectBeforeAnythingIsSent()
    {
        using var handler = new FakeServiceHandler { Capabilities = [.. FakeModels.ModelCapabilities.Where(capability => capability != Capabilities.ExportGraphs)] };
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash(Hash);

        var withheld = await Assert.ThrowsAsync<CapabilityException>(() => model.ExportGraphsAsync("Test::race", Token));
        await Assert.ThrowsAsync<ArgumentNullException>(() => model.ExportGraphsAsync(null!, Token));
        await Assert.ThrowsAsync<ArgumentException>(() => model.ExportGraphsAsync(string.Empty, Token));

        Assert.Equal(Capabilities.ExportGraphs, withheld.Capability);
        Assert.Equal(["GetServerInfo"], handler.Requests.Select(request => request.Method));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ExportGraphsTellsTheMissingModelFromTheMissingSubject(ClientWire wire)
    {
        using var handler = Serving(wire);
        var previous = handler.Respond;
        handler.Respond = (sent, token) => sent.Method != "ExportGraphs" ? previous(sent, token) : Task.FromResult(sent.Message(Wire.ExportGraphsRequest.Parser) switch
        {
            { ModelHash: "gone" } => handler.Refuse(StatusCode.NotFound, "model not found: gone"),
            { Subject: "Test::Missing" } => handler.Refuse(StatusCode.NotFound, "symbol not found: Test::Missing"),
            { Subject: "Test" } => handler.Refuse(StatusCode.InvalidArgument, "Test: no lowered graph"),
            _ => handler.Refuse(StatusCode.FailedPrecondition, "Test::broken: the graph does not lower"),
        });
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash(Hash);

        var gone = await Assert.ThrowsAsync<ModelNotFoundException>(() => connection.ModelByHash("gone").ExportGraphsAsync("Test::race", Token));
        var missing = await Assert.ThrowsAsync<ServiceException>(() => model.ExportGraphsAsync("Test::Missing", Token));
        var invalid = await Assert.ThrowsAsync<ServiceException>(() => model.ExportGraphsAsync("Test", Token));
        var unlowered = await Assert.ThrowsAsync<ServiceException>(() => model.ExportGraphsAsync("Test::broken", Token));

        Assert.Equal("model not found: gone", gone.ServiceMessage);
        Assert.All([missing, invalid, unlowered], error => Assert.Equal(typeof(ServiceException), error.GetType()));
        Assert.Equal(
            [(StatusCode.NotFound, "symbol not found: Test::Missing"), (StatusCode.InvalidArgument, "Test: no lowered graph"), (StatusCode.FailedPrecondition, "Test::broken: the graph does not lower")],
            new[] { missing, invalid, unlowered }.Select(error => (error.StatusCode, error.ServiceMessage)));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AnExportWithoutItsFormVersionOrSubjectIsMalformed(ClientWire wire)
    {
        var answers = new Dictionary<string, Wire.ExportGraphsResponse>(StringComparer.Ordinal)
        {
            ["A::form"] = new() { Version = 1, Subject = "A::form" },
            ["A::version"] = new() { Content = "{}\n", Subject = "A::version" },
            ["A::negative"] = new() { Content = "{}\n", Version = -1, Subject = "A::negative" },
            ["A::subject"] = new() { Content = "{}\n", Version = 1 },
        };
        using var handler = Serving(wire).On("ExportGraphs", Wire.ExportGraphsRequest.Parser, request => answers[request.Subject]);
        await using var connection = await FakeConnection.OpenAsync(handler);

        foreach (var subject in answers.Keys)
        {
            var error = await Assert.ThrowsAsync<TransportException>(() => connection.ModelByHash(Hash).ExportGraphsAsync(subject, Token));

            Assert.Equal(StatusCode.Internal, error.StatusCode);
            Assert.Contains("though every export is a form of a positive version naming its subject", error.Message, StringComparison.Ordinal);
        }
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ImportDataSendsEveryFileAndOptionAsTheSchemaWritesIt(ClientWire wire)
    {
        using var handler = Serving(wire).On("ImportData", Wire.ImportDataRequest.Parser, request => request.ImportDryRun
            ? CompleteGraphDataWire.DryRun
            : request.Imports.Count == 2 ? CompleteGraphDataWire.Imported : new Wire.ImportDataResponse { Files = { new Wire.ImportedFile { Name = "a.tsv" } }, ModelHash = Hash });
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash(Hash);

        await model.ImportDataAsync(CompleteGraphDataWire.Imports, CompleteGraphDataWire.ImportOptions, Token);
        await model.ImportDataAsync(CompleteGraphDataWire.Imports, new ImportOptions { As = "values", DryRun = true }, Token);
        await model.ImportDataAsync([new DataImport("a.tsv", string.Empty)], cancellationToken: Token);
        await model.ImportDataAsync([new DataImport("a.tsv", string.Empty)], new ImportOptions { AcceptDocuments = false }, Token);

        var sent = handler.RequestsFor("ImportData").Select(request => request.Message(Wire.ImportDataRequest.Parser)).ToList();
        Assert.Equal(
            [
                CompleteGraphDataWire.ImportRequest,
                CompleteGraphDataWire.DryRunRequest,
                new Wire.ImportDataRequest { ModelHash = Hash, Imports = { new Wire.DataImport { Name = "a.tsv" } }, AcceptDocuments = true },
                new Wire.ImportDataRequest { ModelHash = Hash, Imports = { new Wire.DataImport { Name = "a.tsv" } } },
            ],
            sent);
    }

    [Fact]
    public void TheSampleImportsSetEveryFieldAnImportRequestReaches()
    {
        Assert.Equal(
            [],
            CompleteWire.ReachableFields(Wire.ImportDataRequest.Descriptor)
                .Except(CompleteWire.SetFields([CompleteGraphDataWire.ImportRequest, CompleteGraphDataWire.DryRunRequest]))
                .ToArray());
        Assert.Equal(
            [],
            CompleteWire.ReachableFields(Wire.ExportGraphsRequest.Descriptor, Wire.CompareResultsRequest.Descriptor)
                .Except(CompleteWire.SetFields([CompleteGraphDataWire.ExportRequest, CompleteGraphDataWire.CompareRequest]))
                .ToArray());
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AnImportReadsEveryFileAndAnswersANewModelLeavingTheOldOneAsItWas(ClientWire wire)
    {
        using var handler = Serving(wire)
            .On("ImportData", Wire.ImportDataRequest.Parser, _ => CompleteGraphDataWire.Imported)
            .On("ExportGraphs", Wire.ExportGraphsRequest.Parser, _ => CompleteGraphDataWire.Graphs);
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = await connection.LoadAsync();

        var result = await model.ImportDataAsync(CompleteGraphDataWire.Imports, CompleteGraphDataWire.ImportOptions, Token);
        await result.Model!.ExportGraphsAsync("Démo::räce", Token);

        Assert.Equal(
            new ImportedFile(
                "fleet.csv",
                3,
                2,
                [
                    new ImportedValue("V::vehicle", "count", "7", "V::Vehicle::count", "fleet.csv line 2, column count", "V::vehicle::count = 7 (redefines V::Vehicle::count)"),
                    new ImportedValue("V::vehicle", "label", "\"fast\"", "V::Vehicle::label", "fleet.csv line 2, column label", "V::vehicle::label = \"fast\" (redefines V::Vehicle::label)"),
                    new ImportedValue("V::car", "engine", string.Empty, "V::Vehicle::engine", "fleet.csv line 3", "V::car::engine (redefines V::Vehicle::engine)"),
                    new ImportedValue("V::car", "count", "9", string.Empty, "fleet.csv line 3, column count", "V::car::count = 9"),
                ]),
            result.Files[0]);
        Assert.Equal(new ImportedFile("fleet.txt", 0, 0, []), result.Files[1]);
        Assert.Equal("V::car::count = 9", result.Files[0].Assignments[3].ToString());
        Assert.Equal(3, result.Values);
        Assert.Equal(CompleteGraphDataWire.Notation, result.Content);
        Assert.Equal([new EditedDocument("fleet.sysml", CompleteGraphDataWire.Notation)], result.Documents);
        Assert.Equal([CompleteWire.Diagnostic.Message], result.Diagnostics.Select(diagnostic => diagnostic.Message));
        Assert.False(result.IsDryRun);
        Assert.Equal(CompleteGraphDataWire.ImportedHash, result.Model.Hash);
        Assert.Same(connection, result.Model.Connection);
        Assert.False(result.Model.OwnsConnection);
        Assert.Empty(result.Model.Roots);
        Assert.Equal(Hash, model.Hash);
        Assert.Equal(CompleteGraphDataWire.ImportedHash, Assert.Single(handler.RequestsFor("ExportGraphs")).Message(Wire.ExportGraphsRequest.Parser).ModelHash);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ADryRunAnswersWhatEachFileWouldSetAndMakesNoModel(ClientWire wire)
    {
        using var handler = Serving(wire).On("ImportData", Wire.ImportDataRequest.Parser, _ => CompleteGraphDataWire.DryRun);
        await using var connection = await FakeConnection.OpenAsync(handler);

        var result = await connection.ModelByHash(Hash).ImportDataAsync(CompleteGraphDataWire.Imports, new ImportOptions { DryRun = true }, Token);

        Assert.True(result.IsDryRun);
        Assert.Null(result.Model);
        Assert.Null(result.Content);
        Assert.Empty(result.Documents);
        Assert.Equal(["fleet.csv", "fleet.txt"], result.Files.Select(file => file.Name));
        Assert.Equal(4, result.Files[0].Assignments.Count);
        Assert.Equal([CompleteWire.BareDiagnostic.Message], result.Diagnostics.Select(diagnostic => diagnostic.Message));
        var refused = await Assert.ThrowsAsync<InvalidOperationException>(() => result.SaveAsync(Path.Combine(directory, "dry.sysml"), cancellationToken: Token));
        Assert.Contains("dry run", refused.Message, StringComparison.Ordinal);
        Assert.Empty(Directory.GetFileSystemEntries(directory));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AModelOfSeveralDocumentsAnswersEachChangedDocumentAndNoNotationOfOne(ClientWire wire)
    {
        using var handler = Serving(wire)
            .On("ParseSources", Wire.ParseSourcesRequest.Parser, _ => new Wire.ParseSourcesResponse { ModelHash = Hash })
            .On("ImportData", Wire.ImportDataRequest.Parser, _ => CompleteGraphDataWire.ImportedSeveral);
        await using var connection = await FakeConnection.OpenAsync(handler);
        var several = await connection.ParseSourcesAsync([SourceDocument.Inline("package V;", "vehicles.sysml"), SourceDocument.Inline("package F;", "fleet.sysml")], cancellationToken: Token);

        var parsed = await several.ImportDataAsync(CompleteGraphDataWire.Imports, cancellationToken: Token);
        var hashed = await connection.ModelByHash(Hash).ImportDataAsync(CompleteGraphDataWire.Imports, cancellationToken: Token);

        Assert.Equal((parsed.Files, parsed.Content, parsed.Documents, parsed.Diagnostics), (hashed.Files, hashed.Content, hashed.Documents, hashed.Diagnostics));
        Assert.Null(parsed.Content);
        Assert.Equal(["vehicles.sysml", "fleet.sysml"], parsed.Documents.Select(document => document.Name));
        Assert.Equal(CompleteGraphDataWire.ImportedHash, parsed.Model!.Hash);
        var refused = await Assert.ThrowsAsync<InvalidOperationException>(() => parsed.SaveAsync(Path.Combine(directory, "model.sysml"), cancellationToken: Token));
        Assert.Contains("EditedDocument.SaveAsync", refused.Message, StringComparison.Ordinal);
        var written = await parsed.Documents[1].SaveAsync(Path.Combine(directory, "fleet.sysml"), cancellationToken: Token);
        Assert.Equal("package F { import V::*; }\n", await File.ReadAllTextAsync(written, Token));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AnImportThatSetNoValueAnswersTheModelItWasMadeOf(ClientWire wire)
    {
        using var handler = Serving(wire).On("ImportData", Wire.ImportDataRequest.Parser, _ => new Wire.ImportDataResponse { Files = { new Wire.ImportedFile { Name = "empty.csv" } }, ModelHash = Hash });
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = await connection.LoadAsync();

        var result = await model.ImportDataAsync([new DataImport("empty.csv", "element\nV::car\n")], cancellationToken: Token);

        Assert.Equal(Hash, result.Model!.Hash);
        Assert.NotSame(model, result.Model);
        Assert.Null(result.Content);
        Assert.Empty(result.Documents);
        Assert.Equal(0, result.Values);
        await Assert.ThrowsAsync<InvalidOperationException>(() => result.SaveAsync(Path.Combine(directory, "same.sysml"), cancellationToken: Token));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ARefusedImportIsAModelExceptionKeepingItsDiagnosticsAndMakesNoModel(ClientWire wire)
    {
        using var handler = Serving(wire).On("ImportData", Wire.ImportDataRequest.Parser, _ => CompleteGraphDataWire.Refusal);
        await using var connection = await FakeConnection.OpenAsync(handler);

        var refused = await Assert.ThrowsAsync<ModelException>(() => connection.ModelByHash(Hash).ImportDataAsync(CompleteGraphDataWire.Imports, cancellationToken: Token));

        Assert.Equal(typeof(ModelException), refused.GetType());
        Assert.Equal(CompleteGraphDataWire.Refusal.Error, refused.Message);
        Assert.Equal(Model.ImportDataMethod, refused.Method);
        Assert.Equal([CompleteWire.Diagnostic.Code], refused.Diagnostics.Select(diagnostic => diagnostic.Code));
    }

    [Theory]
    [MemberData(nameof(MalformedImports))]
    public async Task AMalformedImportAnswerIsRefusedRatherThanRead(ClientWire wire, string name)
    {
        var (answer, importing, says) = Malformed[name];
        using var handler = Serving(wire)
            .On("ParseSources", Wire.ParseSourcesRequest.Parser, _ => new Wire.ParseSourcesResponse { ModelHash = Hash })
            .On("ImportData", Wire.ImportDataRequest.Parser, _ => answer);
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = importing switch
        {
            Importing.Loaded => await connection.LoadAsync(),
            Importing.Several => await connection.ParseSourcesAsync([SourceDocument.Inline("package A;", "a"), SourceDocument.Inline("package B;", "b")], cancellationToken: Token),
            _ => connection.ModelByHash(Hash),
        };

        var error = await Assert.ThrowsAsync<TransportException>(
            () => model.ImportDataAsync([new DataImport(name, string.Empty)], new ImportOptions { DryRun = importing == Importing.DryRun }, Token));

        Assert.Equal(StatusCode.Internal, error.StatusCode);
        Assert.Contains(says, error.Message, StringComparison.Ordinal);
    }

    [Fact]
    public async Task AnImportsShapeAndGateAreCheckedBeforeAnythingIsSent()
    {
        using var handler = new FakeServiceHandler { Capabilities = FakeModels.ModelCapabilities };
        using var withheld = new FakeServiceHandler { Capabilities = [.. FakeModels.ModelCapabilities.Where(capability => capability != Capabilities.ImportData)] };
        await using var connection = await FakeConnection.OpenAsync(handler);
        await using var old = await FakeConnection.OpenAsync(withheld);
        var model = connection.ModelByHash(Hash);
        var file = new DataImport("a.csv", "element\n");

        await Assert.ThrowsAsync<ArgumentNullException>(() => model.ImportDataAsync(null!, cancellationToken: Token));
        await Assert.ThrowsAsync<ArgumentException>(() => model.ImportDataAsync([file, null!], cancellationToken: Token));
        var format = await Assert.ThrowsAsync<InvalidRequestException>(() => model.ImportDataAsync([file, file with { Format = string.Empty }], cancellationToken: Token));
        var map = await Assert.ThrowsAsync<InvalidRequestException>(() => model.ImportDataAsync([file with { Map = string.Empty }], cancellationToken: Token));
        var form = await Assert.ThrowsAsync<InvalidRequestException>(() => model.ImportDataAsync([file], new ImportOptions { As = string.Empty }, Token));
        var gate = await Assert.ThrowsAsync<CapabilityException>(() => old.ModelByHash(Hash).ImportDataAsync([file], cancellationToken: Token));

        Assert.Contains("imports[1]", format.Message, StringComparison.Ordinal);
        Assert.Contains("imports[0]", map.Message, StringComparison.Ordinal);
        Assert.Contains("ImportOptions.As", form.Message, StringComparison.Ordinal);
        Assert.Equal(Capabilities.ImportData, gate.Capability);
        Assert.Equal(["GetServerInfo"], handler.Requests.Select(request => request.Method));
        Assert.Equal(["GetServerInfo"], withheld.Requests.Select(request => request.Method));
        Assert.Throws<ArgumentException>(() => new DataImport(string.Empty, "x"));
        Assert.Throws<ArgumentNullException>(() => new DataImport(null!, "x"));
        Assert.Throws<ArgumentNullException>(() => new DataImport("a.csv", null!));
        Assert.Throws<ArgumentException>(() => file with { Name = string.Empty });
        Assert.Throws<ArgumentNullException>(() => file with { Content = null! });
        Assert.Throws<ArgumentException>(() => new ImportResult([], string.Empty, [], null, []));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ImportStatusesAreMappedAsTheServiceMeansThemAndAnEmptyListIsSent(ClientWire wire)
    {
        using var handler = Serving(wire);
        var previous = handler.Respond;
        handler.Respond = (sent, token) => sent.Method != "ImportData" ? previous(sent, token) : Task.FromResult(sent.Message(Wire.ImportDataRequest.Parser) switch
        {
            { ModelHash: "gone" } => handler.Refuse(StatusCode.NotFound, "model gone is no longer cached: parse it again before importing into it"),
            { Imports.Count: 0 } => handler.Refuse(StatusCode.InvalidArgument, "imports is required: name at least one data file"),
            _ => handler.Refuse(StatusCode.FailedPrecondition, "the model has 2 documents, and this service edits one alone"),
        });
        await using var connection = await FakeConnection.OpenAsync(handler);

        var gone = await Assert.ThrowsAsync<ModelNotFoundException>(() => connection.ModelByHash("gone").ImportDataAsync([new DataImport("a.csv", string.Empty)], cancellationToken: Token));
        var none = await Assert.ThrowsAsync<ServiceException>(() => connection.ModelByHash(Hash).ImportDataAsync([], cancellationToken: Token));
        var several = await Assert.ThrowsAsync<ServiceException>(() => connection.ModelByHash(Hash).ImportDataAsync([new DataImport("a.csv", string.Empty)], cancellationToken: Token));

        Assert.Contains("no longer cached", gone.ServiceMessage, StringComparison.Ordinal);
        Assert.Equal((typeof(ServiceException), StatusCode.InvalidArgument), (none.GetType(), none.StatusCode));
        Assert.Equal((typeof(ServiceException), StatusCode.FailedPrecondition), (several.GetType(), several.StatusCode));
        Assert.Equal(new Wire.ImportDataRequest { ModelHash = Hash, AcceptDocuments = true }, handler.RequestsFor("ImportData")[1].Message(Wire.ImportDataRequest.Parser));
    }

    [Fact]
    public async Task TheImportedNotationIsSavedWholeAndNeverOverAnExistingFileUnlessAsked()
    {
        using var handler = Serving(ClientWire.Grpc).On("ImportData", Wire.ImportDataRequest.Parser, _ => CompleteGraphDataWire.Imported);
        await using var connection = await FakeConnection.OpenAsync(handler);
        var result = await connection.ModelByHash(Hash).ImportDataAsync(CompleteGraphDataWire.Imports, cancellationToken: Token);
        var path = Path.Combine(directory, "fleet.sysml");
        await File.WriteAllTextAsync(path, "package Old;", Token);

        await Assert.ThrowsAsync<UnwritableException>(() => result.SaveAsync(path, cancellationToken: Token));
        Assert.Equal("package Old;", await File.ReadAllTextAsync(path, Token));
        var written = await result.SaveAsync(path, new SaveOptions { Overwrite = true }, Token);

        Assert.Equal(Path.GetFullPath(path), written);
        Assert.Equal(CompleteGraphDataWire.Notation, await File.ReadAllTextAsync(path, Token));
        Assert.Equal(["fleet.sysml"], Directory.GetFileSystemEntries(directory).Select(Path.GetFileName));
        await Assert.ThrowsAsync<ArgumentException>(() => result.SaveAsync(string.Empty, cancellationToken: Token));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task CompareResultsSendsEveryOptionAsTheSchemaWritesIt(ClientWire wire)
    {
        using var handler = Serving(wire).On("CompareResults", Wire.CompareResultsRequest.Parser, _ => new Wire.CompareResultsResponse { Source = "probe.xmi" });
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash(Hash);

        await model.CompareResultsAsync(CompleteGraphDataWire.Results, CompleteGraphDataWire.CompareOptions, Token);
        await model.CompareResultsAsync(CompleteGraphDataWire.Results, cancellationToken: Token);
        await model.CompareResultsAsync(CompleteGraphDataWire.Results, new CompareOptions { Seed = 0, ClockStep = 0, Runs = -1 }, Token);

        var sent = handler.RequestsFor("CompareResults").Select(request => request.Message(Wire.CompareResultsRequest.Parser)).ToList();
        Assert.Equal(CompleteGraphDataWire.CompareRequest, sent[0]);
        Assert.Equal(new Wire.CompareResultsRequest { ModelHash = Hash, Results = CompleteGraphDataWire.Results }, sent[1]);
        Assert.False(sent[1].HasSeed || sent[1].HasClockStep);
        Assert.Equal(new Wire.CompareResultsRequest { ModelHash = Hash, Results = CompleteGraphDataWire.Results, Seed = 0, ClockStep = 0, Runs = -1 }, sent[2]);
        Assert.True(sent[2].HasSeed && sent[2].HasClockStep);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AComparisonReadsEveryConfigurationObservableAndTheEngineThatRanIt(ClientWire wire)
    {
        using var handler = Serving(wire).On("CompareResults", Wire.CompareResultsRequest.Parser, _ => CompleteGraphDataWire.Comparison);
        await using var connection = await FakeConnection.OpenAsync(handler);

        var comparison = await connection.ModelByHash(Hash).CompareResultsAsync(CompleteGraphDataWire.Results, cancellationToken: Token);

        Assert.Equal("probe.xmi", comparison.Source);
        Assert.Equal(new Standing("sweep", "observed", CompleteVerificationWire.Bounds.Select(bound => new Bound(bound.Name, bound.Limit, bound.Reached))), comparison.Standing);
        Assert.Equal([CompleteWire.Diagnostic.Code], comparison.Diagnostics.Select(diagnostic => diagnostic.Code));
        Assert.False(comparison.Holds);
        var (ran, drew, unresolved) = (comparison.Configurations[0], comparison.Configurations[1], comparison.Configurations[2]);
        Assert.Equal(
            ("_c1", "Cfg::'Group 1'", "run", "target", "Results", "fails", string.Empty, 10L, 3L, 4L, 2L, "average", (ulong?)ulong.MaxValue, 0.5),
            (ran.Id, ran.Name, ran.Behavior, ran.Target, ran.ResultLocation, ran.Status, ran.Unresolved, ran.StoredRuns, ran.StoredSnapshots, ran.Runs, ran.Completed, ran.Draws, ran.Seed, ran.ClockStep));
        Assert.Equal(3, ran.Rows.Count);
        Assert.Equal("division by zero", ran.Rows[1].Error);
        Assert.Equal(CompleteWire.Instance.TypeSymbolId, ran.FindInstance(9)!.TypeSymbolId);
        Assert.Equal(ran.FindInstance(9), ran.Resolve(new Value.InstanceRefValue(9)));
        Assert.Equal(["division by zero"], ran.Errors);
        Assert.Equal(["the configuration states no numberOfRuns"], ran.Notes);
        Assert.Equal("compare Cfg::'Group 1' — 10 stored run(s)\nerror: division by zero", ran.ToString());
        Assert.False(ran.Holds);
        Assert.Equal(
            new ObservableComparison(
                "total",
                "target.total",
                "kg",
                new ComparedDistribution(2, 4, 5, 4.5, 5.9, 6, 1.4142135623730951),
                false,
                false,
                new ComparedDistribution(2, 4, 4, 4, 4, 4, 0),
                2,
                true,
                new StatisticDifference("+0.0%", "-20.0%", "-11.1%", "-32.2%", "-33.3%"),
                null,
                []),
            ran.Observables[0]);
        var pooled = ran.Observables[1];
        Assert.True(pooled.Pooled);
        Assert.Equal(new ComparedDistribution(10, 0, 5, 0, 0, 0, 0), pooled.Tool);
        Assert.Equal(new ComparedDistribution(2, double.NegativeInfinity, 1.5, double.NaN, 1e-300, double.MaxValue, 0), pooled.Run);
        Assert.True(double.IsNaN(pooled.Run!.P50));
        Assert.Equal(new StatisticDifference(string.Empty, "-20.0%", string.Empty, string.Empty, string.Empty), pooled.Difference);
        Assert.Equal(
            new CaseStatistics("Cfg::MassCase", [new CaseStatistic("N", "return N", "10", "2", string.Empty), new CaseStatistic("Mean", "return Mean", "5.0", "4.0", "-20.0%")], ["the case declares no OutOfSpec"]),
            pooled.Declared);
        Assert.Equal((true, true, (ComparedDistribution?)null, (ComparedDistribution?)null), (ran.Observables[2].Unstored, ran.Observables[2].Compared, ran.Observables[2].Tool, ran.Observables[2].Run));
        Assert.Equal(
            new ObservableComparison("speed", "target.speed", string.Empty, null, false, false, null, 0, false, null, null, ["note: the tool stored no observable named speed"]),
            ran.Observables[3]);
        Assert.Null(drew.Seed);
        Assert.True(drew.Holds);
        Assert.Empty(drew.Rows);
        Assert.Equal(("unresolved", "no configuration is named Cfg::Nothing"), (unresolved.Status, unresolved.Unresolved));
        Assert.Equal(string.Join('\n', ran.Lines.Concat(unresolved.Lines)), comparison.ToString());
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ResultsNothingOfWhichWasComparedAreACompareExceptionKeepingWhatTheAnswerReported(ClientWire wire)
    {
        var newer = new Wire.CompareResultsResponse { Error = "a failure of a newer kind", FailureReason = (Wire.FailureReason)99 };
        using var handler = Serving(wire).On("CompareResults", Wire.CompareResultsRequest.Parser, request => request.Runs == 1 ? newer : CompleteGraphDataWire.CompareRefusal);
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash(Hash);

        var refused = await Assert.ThrowsAsync<CompareException>(() => model.CompareResultsAsync(CompleteGraphDataWire.Results, cancellationToken: Token));
        var unnamed = await Assert.ThrowsAsync<CompareException>(() => model.CompareResultsAsync(CompleteGraphDataWire.Results, new CompareOptions { Runs = 1 }, Token));

        Assert.Equal(CompleteGraphDataWire.CompareRefusal.Error, refused.Message);
        Assert.Equal(Model.CompareResultsMethod, refused.Method);
        Assert.Equal(FailureReason.Evaluation, refused.Reason);
        Assert.Equal(("sweep", "not covered"), (refused.Standing.Engine, refused.Standing.Strength));
        Assert.Equal(refused.Standing, refused.Partial.Standing);
        Assert.Equal("probe.xmi", refused.Partial.Source);
        Assert.Empty(refused.Partial.Configurations);
        Assert.Equal([CompleteWire.Diagnostic.Code], refused.Diagnostics.Select(diagnostic => diagnostic.Code));
        Assert.Equal((FailureReason)99, unnamed.Reason);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AFailureKindWithoutTheFailureItNamesIsMalformed(ClientWire wire)
    {
        using var handler = Serving(wire).On("CompareResults", Wire.CompareResultsRequest.Parser, _ => new Wire.CompareResultsResponse { FailureReason = Wire.FailureReason.WrongKind });
        await using var connection = await FakeConnection.OpenAsync(handler);

        var error = await Assert.ThrowsAsync<TransportException>(() => connection.ModelByHash(Hash).CompareResultsAsync(CompleteGraphDataWire.Results, cancellationToken: Token));

        Assert.Equal(StatusCode.Internal, error.StatusCode);
        Assert.Contains("the failure kind WrongKind without the failure it names", error.Message, StringComparison.Ordinal);
    }

    [Fact]
    public async Task CompareGatesAreJudgedInTheServicesOrderBeforeAnythingIsSent()
    {
        static FakeServiceHandler Without(params string[] withheld) =>
            new() { Capabilities = [.. FakeModels.ModelCapabilities.Where(capability => !withheld.Contains(capability))] };
        using var none = Without(Capabilities.CompareResults, Capabilities.Engines, Capabilities.ScheduleExplore);
        using var engines = Without(Capabilities.Engines, Capabilities.ScheduleExplore);
        using var explore = Without(Capabilities.ScheduleExplore);
        var explored = new CompareOptions { Engine = " explore " };
        var judged = new List<string>();

        foreach (var (handler, options) in new[] { (none, explored), (engines, explored), (explore, explored) })
        {
            await using var connection = await FakeConnection.OpenAsync(handler);
            var refused = await Assert.ThrowsAsync<CapabilityException>(() => connection.ModelByHash(Hash).CompareResultsAsync(CompleteGraphDataWire.Results, options, Token));
            judged.Add(refused.Capability);
            Assert.Equal(["GetServerInfo"], handler.Requests.Select(request => request.Method));
        }

        Assert.Equal([Capabilities.CompareResults, Capabilities.Engines, Capabilities.ScheduleExplore], judged);
    }

    [Fact]
    public async Task ComparisonOptionsNamingEmptyTextAreRefusedBeforeAnythingIsSent()
    {
        using var handler = new FakeServiceHandler { Capabilities = FakeModels.ModelCapabilities };
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash(Hash);

        await Assert.ThrowsAsync<ArgumentNullException>(() => model.CompareResultsAsync(null!, cancellationToken: Token));
        await Assert.ThrowsAsync<ArgumentException>(() => model.CompareResultsAsync(string.Empty, cancellationToken: Token));
        var draws = await Assert.ThrowsAsync<InvalidRequestException>(() => model.CompareResultsAsync("{}", new CompareOptions { Draws = string.Empty }, Token));
        var engine = await Assert.ThrowsAsync<InvalidRequestException>(() => model.CompareResultsAsync("{}", new CompareOptions { Engine = string.Empty }, Token));
        var feature = await Assert.ThrowsAsync<InvalidRequestException>(
            () => model.CompareResultsAsync("{}", new CompareOptions { Observe = [new ObservablePair("total"), new ObservablePair("mass", string.Empty)] }, Token));

        Assert.Contains("Draws or Engine", draws.Message, StringComparison.Ordinal);
        Assert.Contains("Draws or Engine", engine.Message, StringComparison.Ordinal);
        Assert.Contains("CompareOptions.Observe[1]", feature.Message, StringComparison.Ordinal);
        Assert.Equal(["GetServerInfo"], handler.Requests.Select(request => request.Method));
        Assert.Throws<ArgumentException>(() => new ObservablePair(" "));
        Assert.Throws<ArgumentNullException>(() => new ObservablePair(null!));
        Assert.Throws<ArgumentException>(() => new ObservablePair("total") with { Stored = string.Empty });
        Assert.Throws<ArgumentNullException>(() => new CompareOptions { Actions = null! });
        Assert.Throws<ArgumentException>(() => new CompareOptions { Observe = [null!] });
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task CompareStatusesAreMappedAsTheServiceMeansThem(ClientWire wire)
    {
        using var handler = Serving(wire);
        var previous = handler.Respond;
        handler.Respond = (sent, token) => sent.Method != "CompareResults" ? previous(sent, token) : Task.FromResult(sent.Message(Wire.CompareResultsRequest.Parser) switch
        {
            { ModelHash: "gone" } => handler.Refuse(StatusCode.NotFound, "model not found: gone"),
            _ => handler.Refuse(StatusCode.InvalidArgument, "runs must not be negative, got -1"),
        });
        await using var connection = await FakeConnection.OpenAsync(handler);

        await Assert.ThrowsAsync<ModelNotFoundException>(() => connection.ModelByHash("gone").CompareResultsAsync("{}", cancellationToken: Token));
        var negative = await Assert.ThrowsAsync<ServiceException>(() => connection.ModelByHash(Hash).CompareResultsAsync("{}", new CompareOptions { Runs = -1 }, Token));

        Assert.Equal((typeof(ServiceException), StatusCode.InvalidArgument), (negative.GetType(), negative.StatusCode));
    }

    private static FakeServiceHandler Serving(ClientWire wire) =>
        new FakeServiceHandler(wire) { Capabilities = FakeModels.ModelCapabilities }.Parsing(Hash, FakeModels.Symbol("V", "V", "package"));

    /// <summary>How the import answering a malformed answer is made: of which model, and whether as a dry run.</summary>
    public enum Importing
    {
        /// <summary>Of a model named by hash, whose documents the client does not know.</summary>
        Hashed,

        /// <summary>Of a model loaded from one document.</summary>
        Loaded,

        /// <summary>Of a model parsed from two documents.</summary>
        Several,

        /// <summary>As a dry run, of a model named by hash.</summary>
        DryRun,
    }
}
