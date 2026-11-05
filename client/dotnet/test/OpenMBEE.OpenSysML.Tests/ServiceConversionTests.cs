using System;
using System.Collections.Concurrent;
using System.IO;
using System.IO.Compression;
using System.Linq;
using System.Net.Http;
using System.Threading;
using System.Threading.Tasks;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// Converting and migrating against a real sysml-grpc over every client wire: notation, Turtle and the API's JSON element
/// form from a file, inline content and a parsed model, with ids, documents, the compact form and the check's
/// diagnostics; syntax errors tolerated or failing; the wrong verb, a missing file and an evicted model refused; SysML v1
/// models migrated with every verdict accounted for, their results, layouts, strictness and image files, written to disk
/// by the safe-write contract; and, against a service withholding these capabilities, refusals made before anything is
/// sent.
/// </summary>
[Trait(Category.Name, Category.Integration)]
public sealed class ServiceConversionTests(ServiceFixture services) : IClassFixture<ServiceFixture>, IDisposable
{
    private const string Withheld = "convert_documents,convert_library_stubs,convert_compact,convert_check_diagnostics,migrate";

    private readonly string directory = Directory.CreateTempSubdirectory("opensysml-conversion-").FullName;

    public static TheoryData<ClientWire> Wires => ClientWires.All;

    private static CancellationToken Token => TestContext.Current.CancellationToken;

    public void Dispose() => Directory.Delete(directory, recursive: true);

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task NotationConvertsToEveryFormatAndBackFromEverySource(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var vehicle = await File.ReadAllTextAsync(Fixture("vehicle.sysml"), Token);
        var model = (await connection.LoadContentAsync(vehicle, cancellationToken: Token)).RequireNoErrors();

        var notation = await connection.ConvertAsync(ConvertSource.Inline(vehicle), "text", new ConvertOptions { FromFormat = "sysml" }, Token);
        var turtle = await model.ToTurtleAsync(cancellationToken: Token);
        var back = await connection.ConvertAsync(ConvertSource.Inline(turtle.Content), ConversionFormat.SysML, new ConvertOptions { FromFormat = "rdf" }, Token);
        var json = await model.ToApiJsonAsync(cancellationToken: Token);
        var fromFile = await connection.ConvertAsync(ConvertSource.FromFile(Fixture("vehicle.sysml")), "turtle", cancellationToken: Token);
        var byHash = await model.ToNotationAsync(cancellationToken: Token);

        Assert.Equal(("sysml", "sysml", false, string.Empty), (notation.FromFormat, notation.ToFormat, notation.Experimental, notation.ExperimentalNotice));
        Assert.Contains("part def Vehicle", notation.Content, StringComparison.Ordinal);
        Assert.Equal(("sysml", "ttl", true), (turtle.FromFormat, turtle.ToFormat, turtle.Experimental));
        Assert.NotEmpty(turtle.ExperimentalNotice);
        Assert.Equal(("ttl", "sysml", true), (back.FromFormat, back.ToFormat, back.Experimental));
        Assert.Contains("part def Vehicle", back.Content, StringComparison.Ordinal);
        Assert.Equal("api-json", json.ToFormat);
        Assert.StartsWith("[", json.Content.TrimStart(), StringComparison.Ordinal);
        Assert.Equal(("sysml", "ttl"), (fromFile.FromFormat, fromFile.ToFormat));
        Assert.Contains("Vehicle", fromFile.Content, StringComparison.Ordinal);
        Assert.Equal(notation.Content, byHash.Content);
        Assert.Empty(byHash.Diagnostics);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task IdsDocumentsTheCompactFormAndTheChecksDiagnosticsAreWhatTheServiceWrites(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var model = (await connection.LoadContentAsync(await File.ReadAllTextAsync(Fixture("vehicle.sysml"), Token), cancellationToken: Token)).RequireNoErrors();
        var sources = await connection.ParseSourcesAsync(
            [SourceDocument.Inline("package A { part def Engine; }", "a.sysml"), SourceDocument.Inline("package B { part e : A::Engine; }", "b.sysml")],
            cancellationToken: Token);

        var qualified = await model.ToTurtleAsync(new ConvertOptions { IdForm = IdForm.Qualified }, Token);
        var uuid = await model.ToTurtleAsync(new ConvertOptions { IdForm = IdForm.Uuid }, Token);
        var whole = await sources.ToTurtleAsync(cancellationToken: Token);
        var one = await sources.ToTurtleAsync(new ConvertOptions { Documents = ["b.sysml"] }, Token);
        var compact = await model.ToApiJsonAsync(new ConvertOptions { Compact = true, OmitDerived = true, KeepDerived = ["owner"] }, Token);
        var checkedNotation = await connection.ConvertAsync(
            ConvertSource.Inline("package P { part x : Nope; }"), "sysml", new ConvertOptions { FromFormat = "sysml", IncludeCheckDiagnostics = true }, Token);
        var unknown = await Assert.ThrowsAsync<ServiceException>(() => sources.ToTurtleAsync(new ConvertOptions { Documents = ["c.sysml"] }, Token));
        var several = await Assert.ThrowsAsync<ServiceException>(() => sources.ToNotationAsync(cancellationToken: Token));

        Assert.Equal(await model.ToTurtleAsync(cancellationToken: Token), qualified);
        Assert.NotEqual(qualified.Content, uuid.Content);
        Assert.Contains("Engine", whole.Content, StringComparison.Ordinal);
        Assert.NotEqual(whole.Content, one.Content);
        Assert.Contains("api-json-compact/1", compact.Content, StringComparison.Ordinal);
        Assert.Contains(checkedNotation.Diagnostics, diagnostic => diagnostic.Severity == DiagnosticSeverity.Error && diagnostic.Message.Contains("Nope", StringComparison.Ordinal));
        Assert.Contains("part x : Nope", checkedNotation.Content, StringComparison.Ordinal);
        Assert.Equal(StatusCode.InvalidArgument, unknown.StatusCode);
        Assert.Contains("c.sysml", unknown.ServiceMessage, StringComparison.Ordinal);
        Assert.Equal(StatusCode.FailedPrecondition, several.StatusCode);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task TheLibraryAModelRefersToIsListedBesideItsElementsAndWrittenAsRootsOnlyWhenAsked(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var model = (await connection.LoadContentAsync("package P { private import SI::*; attribute time = 20 [s]; }", cancellationToken: Token)).RequireNoErrors();

        var bare = await model.ToApiJsonAsync(cancellationToken: Token);
        var stubbed = await model.ToApiJsonAsync(new ConvertOptions { IncludeLibraryStubs = true }, Token);
        var compact = await model.ToApiJsonAsync(new ConvertOptions { Compact = true }, Token);
        var turtle = await model.ToTurtleAsync(cancellationToken: Token);
        var refused = await Assert.ThrowsAsync<ServiceException>(() => model.ToTurtleAsync(new ConvertOptions { IncludeLibraryStubs = true }, Token));

        Assert.Equal(
            [("SI::second", "AttributeUsage"), ("SI", "LibraryPackage")],
            bare.LibraryReferences.Where(reference => reference.QualifiedName.StartsWith("SI", StringComparison.Ordinal)).Select(reference => (reference.QualifiedName, reference.Type)));
        Assert.DoesNotContain(bare.LibraryReferences, reference => reference.QualifiedName.Length > 0 && reference.LibraryPath.Length > 0);
        Assert.Contains(bare.LibraryReferences, reference => (reference.QualifiedName, reference.Type, reference.LibraryPath) == (string.Empty, "Documentation", "Base::Anything/Documentation[0]"));
        var bareIds = Ids(bare.Content);
        var stubbedIds = Ids(stubbed.Content);
        Assert.All(bare.LibraryReferences, reference => Assert.DoesNotContain(reference.Id, bareIds));
        Assert.Equal(bareIds.Union(bare.LibraryReferences.Select(reference => reference.Id)).Order(StringComparer.Ordinal), stubbedIds.Order(StringComparer.Ordinal));
        Assert.Equal(bare.LibraryReferences, stubbed.LibraryReferences);
        Assert.Equal(bare.LibraryReferences, compact.LibraryReferences);
        Assert.Empty(turtle.LibraryReferences);
        Assert.Equal(StatusCode.InvalidArgument, refused.StatusCode);
        Assert.Contains("api-json", refused.ServiceMessage, StringComparison.Ordinal);

        static string[] Ids(string content) =>
            [.. System.Text.Json.JsonDocument.Parse(content).RootElement.EnumerateArray().Select(element => element.GetProperty("@id").GetString()!)];
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ASyntaxErrorFailsTheConversionUnlessToleratedAndAMappingRefusalIsInBand(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var broken = await File.ReadAllTextAsync(Fixture("syntax_error.sysml"), Token);

        var refused = await Assert.ThrowsAsync<ConversionException>(() =>
            connection.ConvertAsync(ConvertSource.Inline(broken), "sysml", new ConvertOptions { FromFormat = "sysml" }, Token));
        var tolerated = await connection.ConvertAsync(ConvertSource.Inline(broken), "sysml", new ConvertOptions { FromFormat = "sysml", TolerateSyntaxErrors = true }, Token);
        var graph = await Assert.ThrowsAsync<ConversionException>(() =>
            connection.ConvertAsync(ConvertSource.Inline(broken), "ttl", new ConvertOptions { FromFormat = "sysml" }, Token));

        Assert.NotEmpty(refused.Diagnostics);
        Assert.Equal(("sysml", "sysml", false), (refused.FromFormat, refused.ToFormat, refused.Experimental));
        Assert.Equal(Connection.ConvertMethod, refused.Method);
        Assert.NotEmpty(tolerated.Diagnostics);
        Assert.All(tolerated.Diagnostics, diagnostic => Assert.Equal(DiagnosticSeverity.Error, diagnostic.Severity));
        Assert.Equal(("ttl", true), (graph.ToFormat, graph.Experimental));
        Assert.NotEmpty(graph.ExperimentalNotice);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task TheWrongVerbAMissingFileAndAnEvictedModelAreRefused(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var xmi = await File.ReadAllTextAsync(Fixture("vehicle.xmi"), Token);

        var converted = await Assert.ThrowsAsync<ServiceException>(() =>
            connection.ConvertAsync(ConvertSource.Inline(xmi), "sysml", new ConvertOptions { FromFormat = ConversionFormat.Xmi }, Token));
        var convertedFile = await Assert.ThrowsAsync<ServiceException>(() => connection.ConvertAsync(ConvertSource.FromFile(Fixture("vehicle.xmi")), "sysml", cancellationToken: Token));
        var migrated = await Assert.ThrowsAsync<ServiceException>(() =>
            connection.MigrateAsync(MigrateSource.Inline(File.ReadAllBytes(Fixture("vehicle.sysml"))), "sysml", new MigrateOptions { FromFormat = "sysml" }, Token));
        var target = await Assert.ThrowsAsync<ServiceException>(() => connection.MigrateAsync(MigrateSource.FromFile(Fixture("vehicle.xmi")), ConversionFormat.Mdzip, cancellationToken: Token));
        var missing = await Assert.ThrowsAsync<ModelFileNotFoundException>(() =>
            connection.ConvertAsync(ConvertSource.FromFile(Path.Combine(directory, "missing.sysml")), "ttl", cancellationToken: Token));
        var missingV1 = await Assert.ThrowsAsync<ModelFileNotFoundException>(() =>
            connection.MigrateAsync(MigrateSource.FromFile(Path.Combine(directory, "missing.xmi")), "sysml", cancellationToken: Token));
        var evicted = await Assert.ThrowsAsync<ModelNotFoundException>(() => connection.ModelByHash("0000").ToTurtleAsync(cancellationToken: Token));

        Assert.All([converted, convertedFile, migrated, target], refusal => Assert.Equal(StatusCode.InvalidArgument, refusal.StatusCode));
        Assert.Contains("is a SysML v1 model, which is migrated, not converted", converted.ServiceMessage, StringComparison.Ordinal);
        Assert.Contains("Migrate", convertedFile.ServiceMessage, StringComparison.Ordinal);
        Assert.Contains("is converted, not migrated", migrated.ServiceMessage, StringComparison.Ordinal);
        Assert.Contains("Convert", migrated.ServiceMessage, StringComparison.Ordinal);
        Assert.Contains("never written", target.ServiceMessage, StringComparison.Ordinal);
        Assert.Equal(Connection.ConvertMethod, missing.Method);
        Assert.Equal(Connection.MigrateMethod, missingV1.Method);
        Assert.Equal(Connection.ConvertMethod, evicted.Method);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AV1ModelIsMigratedWithEveryElementAccountedFor(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var path = Fixture("vehicle.xmi");

        var summary = await connection.MigrateAsync(MigrateSource.Inline(File.ReadAllBytes(path)), "sysml", new MigrateOptions { FromFormat = ConversionFormat.Uml }, Token);
        var full = await connection.MigrateAsync(MigrateSource.FromFile(path), ConversionFormat.Turtle, new MigrateOptions { Report = true, Results = true }, Token);

        Assert.Equal(("xmi", "sysml", true), (summary.FromFormat, summary.ToFormat, summary.Experimental));
        Assert.NotEmpty(summary.ExperimentalNotice);
        Assert.Null(summary.SourcePath);
        Assert.Contains("part def Vehicle", summary.Content, StringComparison.Ordinal);
        Assert.Equal((77, 15, 3, 2), (summary.Report.Mapped, summary.Report.Approximated, summary.Report.Unmapped, summary.Report.Skipped));
        Assert.StartsWith("migrated 95 element(s)", summary.Report.Summary, StringComparison.Ordinal);
        Assert.Empty(summary.Report.Entries);
        Assert.Equal((string.Empty, string.Empty), (summary.Report.Text, summary.Results));
        Assert.Empty(summary.Files);
        Assert.Equal(Path.GetFullPath(path), full.SourcePath);
        Assert.Equal("ttl", full.ToFormat);
        Assert.Equal(full.Report.Total, full.Report.Entries.Count);
        Assert.Equal(97L, full.Report.Total);
        foreach (var (verdict, count) in new[] { ("mapped", full.Report.Mapped), ("approximated", full.Report.Approximated), ("unmapped", full.Report.Unmapped), ("skipped", full.Report.Skipped) })
        {
            Assert.Equal(count, full.Report.ByVerdict(verdict).Count);
        }

        Assert.All(full.Report.Entries, entry => Assert.NotEmpty(entry.Id));
        Assert.All(full.Report.ByVerdict("unmapped"), entry => Assert.NotEmpty(entry.Note));
        Assert.StartsWith("# SysML v1 to v2 migration report", full.Report.Text, StringComparison.Ordinal);
        Assert.NotEmpty(full.Results);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AnArchivesImageFilesAreWrittenBesideTheMigratedModelAndTheV1ModelIsNeverReplaced(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        byte[] picture = [0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x20, 0x66];
        var archive = Archive(("documents.xmi", File.ReadAllBytes(Migrations("documents.xmi"))), ("attachments/fleet.png", picture));
        var path = Path.Combine(directory, "Fleet.sysml");
        var v1 = Path.Combine(directory, "Vehicle.xmi");
        File.Copy(Fixture("vehicle.xmi"), v1);
        var original = await File.ReadAllBytesAsync(v1, Token);

        var migration = await connection.MigrateAsync(MigrateSource.Inline(archive), "sysml", new MigrateOptions { FromFormat = ConversionFormat.Mdzip }, Token);
        var written = await migration.WriteAsync(path, cancellationToken: Token);
        var again = await Assert.ThrowsAsync<UnwritableException>(() => migration.WriteAsync(path, cancellationToken: Token));
        await migration.WriteAsync(path, new SaveOptions { Overwrite = true }, Token);
        Assert.Throws<ArgumentException>(() => MigrateSource.FromFile(Path.GetRelativePath(Environment.CurrentDirectory, v1)));
        var fromFile = await connection.MigrateAsync(MigrateSource.FromFile(Path.Combine(directory, ".", "Vehicle.xmi")), "sysml", cancellationToken: Token);
        var itself = await Assert.ThrowsAsync<UnwritableException>(() => fromFile.WriteAsync(v1, new SaveOptions { Overwrite = true }, Token));

        Assert.Equal(new MigrationFile("images/fleet.png", picture), Assert.Single(migration.Files));
        Assert.Contains("images/fleet.png", migration.Content, StringComparison.Ordinal);
        Assert.Contains("wrote 1 image file(s)", migration.Report.Summary, StringComparison.Ordinal);
        Assert.Equal(Path.GetFullPath(path), written);
        Assert.Equal(migration.Content, await File.ReadAllTextAsync(path, Token));
        Assert.Equal(picture, await File.ReadAllBytesAsync(Path.Combine(directory, "images", "fleet.png"), Token));
        Assert.Equal(path, again.Path);
        Assert.Equal(v1, fromFile.SourcePath);
        Assert.Equal(v1, itself.Path);
        Assert.Equal(original, await File.ReadAllBytesAsync(v1, Token));
        Assert.Equal(["Fleet.sysml", "Vehicle.xmi", "images"], Directory.GetFileSystemEntries(directory).Select(Path.GetFileName).Order(StringComparer.Ordinal));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ALayoutLaysTheViewsOutAndStrictnessWritesNoExtension(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var layout = Migrations("layout.layout.xml");
        var source = MigrateSource.FromFile(Migrations("layout.xmi"));
        var decision = MigrateSource.FromFile(Migrations("decision_property_probability.xmi"));

        var byPath = await connection.MigrateAsync(source, "sysml", new MigrateOptions { LayoutPath = layout }, Token);
        var inline = await connection.MigrateAsync(source, "sysml", new MigrateOptions { LayoutContent = await File.ReadAllTextAsync(layout, Token) }, Token);
        var unreadable = await Assert.ThrowsAsync<ServiceException>(() => connection.MigrateAsync(source, "sysml", new MigrateOptions { LayoutContent = "<not mtip" }, Token));
        var missing = await Assert.ThrowsAsync<ModelFileNotFoundException>(() =>
            connection.MigrateAsync(source, "sysml", new MigrateOptions { LayoutPath = Path.Combine(directory, "none_mtip.xml") }, Token));
        var lax = await connection.MigrateAsync(decision, "sysml", cancellationToken: Token);
        var strict = await connection.MigrateAsync(decision, "sysml", new MigrateOptions { Strict = true, ImageBaseUrl = "https://example.com" }, Token);

        Assert.Contains("laid out", byPath.Report.Summary, StringComparison.Ordinal);
        Assert.Contains("DiagramLayout", byPath.Content, StringComparison.Ordinal);
        Assert.Equal(byPath.Content, inline.Content);
        Assert.Equal(StatusCode.InvalidArgument, unreadable.StatusCode);
        Assert.Equal(Connection.MigrateMethod, missing.Method);
        Assert.NotEqual(lax.Content, strict.Content);
        Assert.DoesNotContain(strict.Content.Split('\n'), line => line.TrimStart().StartsWith("choice ", StringComparison.Ordinal));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task SavingAModelWritesWhatItConvertsTo(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var model = (await connection.LoadContentAsync(await File.ReadAllTextAsync(Fixture("vehicle.sysml"), Token), cancellationToken: Token)).RequireNoErrors();
        var path = Path.Combine(directory, "vehicle.ttl");

        var saved = await model.SaveAsync(path, cancellationToken: Token);
        var refused = await Assert.ThrowsAsync<UnwritableException>(() => model.SaveAsync(path, cancellationToken: Token));
        var renamed = await model.SaveAsync(path, ConversionFormat.SysML, saveOptions: new SaveOptions { Overwrite = true }, cancellationToken: Token);

        Assert.Equal("ttl", saved.ToFormat);
        Assert.Equal(path, refused.Path);
        Assert.Equal(renamed.Content, await File.ReadAllTextAsync(path, Token));
        Assert.Equal(["vehicle.ttl"], Directory.GetFileSystemEntries(directory).Select(Path.GetFileName));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AWithheldCapabilityIsRefusedBeforeAnythingIsSent(ClientWire wire)
    {
        var withheld = await services.ServiceAsync("connect", Withheld);
        var unconverting = await services.ServiceAsync("connect", "convert");
        using var counting = new CountingHandler();
        await using var connection = await Connection.OpenAsync(
            new ConnectionOptions { Address = withheld.Address, Protocol = wire.Protocol(), Encoding = wire.Encoding(), HttpMessageHandler = counting },
            Token);
        await using var bare = await Connection.OpenAsync(wire.To(unconverting.Address), Token);
        var model = connection.ModelByHash("h");

        var refused = new[]
        {
            await Assert.ThrowsAsync<CapabilityException>(() => model.ToTurtleAsync(new ConvertOptions { Documents = ["a.sysml"], Compact = true }, Token)),
            await Assert.ThrowsAsync<CapabilityException>(() => model.ToApiJsonAsync(new ConvertOptions { IncludeLibraryStubs = true, Compact = true }, Token)),
            await Assert.ThrowsAsync<CapabilityException>(() => model.ToApiJsonAsync(new ConvertOptions { Compact = true, IncludeCheckDiagnostics = true }, Token)),
            await Assert.ThrowsAsync<CapabilityException>(() => model.ToNotationAsync(new ConvertOptions { IncludeCheckDiagnostics = true }, Token)),
            await Assert.ThrowsAsync<CapabilityException>(() => connection.MigrateAsync(MigrateSource.FromFile(Path.Combine(directory, "a.xmi")), "sysml", cancellationToken: Token)),
            await Assert.ThrowsAsync<CapabilityException>(() => bare.ConvertAsync(ConvertSource.Inline("package P;"), "sysml", cancellationToken: Token)),
        };

        Assert.Equal(
            [Capabilities.ConvertDocuments, Capabilities.ConvertLibraryStubs, Capabilities.ConvertCompact, Capabilities.ConvertCheckDiagnostics, Capabilities.Migrate, Capabilities.Convert],
            refused.Select(error => error.Capability));
        Assert.Equal(["GetServerInfo"], counting.Methods);
    }

    private static string Fixture(string name) => Repository.PathOf("conformance", "fixtures", name);

    private static string Migrations(string name) => Repository.PathOf("tests", "migrate", "testdata", "xmi", name);

    // A Cameo/MagicDraw archive holding these entries.
    private static byte[] Archive(params (string Name, byte[] Content)[] entries)
    {
        using var buffer = new MemoryStream();
        using (var zip = new ZipArchive(buffer, ZipArchiveMode.Create, leaveOpen: true))
        {
            foreach (var (name, content) in entries)
            {
                using var stream = zip.CreateEntry(name).Open();
                stream.Write(content);
            }
        }

        return buffer.ToArray();
    }

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
