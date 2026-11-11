using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Threading.Tasks;
using Google.Protobuf;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// Convert and Migrate through the typed API, over every wire: the exact request each source and option makes, every
/// field of each answer read, a conversion or migration the service could not make raised with the formats and
/// diagnostics it kept, a malformed answer refused, the wrong verb left to the service to refuse, a missing file or
/// model told apart, and the gates checked in the service's order before anything is sent.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed class ModelConversionTests
{
    private const string Hash = "h";

    private static readonly string[] Verdicts = ["mapped", "approximated", "unmapped", "skipped"];

    private static readonly string[] Named = ["a.sysml", "b.KerML", "c.ttl", "d.turtle", "e.json", "f.xmi", "g", "h.txt"];

    public static TheoryData<ClientWire> Wires => ClientWires.All;

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ConvertSendsEverySourceAndOptionAndReadsEveryField(ClientWire wire)
    {
        using var handler = Serving(wire).On("Convert", Wire.ConvertRequest.Parser, request => request.ToFormat == "api-json" ? CompleteConversionWire.ApiJsonConversion : CompleteConversionWire.Conversion);
        await using var connection = await FakeConnection.OpenAsync(handler);
        var token = TestContext.Current.CancellationToken;
        var options = new ConvertOptions
        {
            FromFormat = "kerml",
            TolerateSyntaxErrors = true,
            IdForm = IdForm.Uuid,
            Documents = ["a.sysml", "b.sysml"],
            Compact = true,
            OmitDerived = true,
            KeepDerived = ["owner", "qualifiedName"],
            IncludeCheckDiagnostics = true,
            IncludeLibraryStubs = true,
        };

        var conversion = await connection.ConvertAsync(ConvertSource.Inline("package P;"), "turtle", options, token);
        await connection.ConvertAsync(ConvertSource.FromFile("/models/a.ttl"), ConversionFormat.SysML, new ConvertOptions { IdForm = IdForm.Qualified }, token);
        var apiJson = await connection.ModelByHash(Hash).ConvertAsync(ConversionFormat.ApiJson, cancellationToken: token);
        await connection.ConvertAsync(ConvertSource.FromModelHash(Hash), string.Empty, cancellationToken: token);

        Assert.Equal(
            [
                new Wire.ConvertRequest
                {
                    Content = "package P;",
                    FromFormat = "kerml",
                    ToFormat = "turtle",
                    TolerateSyntaxErrors = true,
                    IdForm = "uuid",
                    Documents = { "a.sysml", "b.sysml" },
                    Compact = true,
                    OmitDerived = true,
                    KeepDerived = { "owner", "qualifiedName" },
                    IncludeCheckDiagnostics = true,
                    IncludeLibraryStubs = true,
                },
                new Wire.ConvertRequest { FilePath = "/models/a.ttl", ToFormat = "sysml", IdForm = "qualified" },
                new Wire.ConvertRequest { ModelHash = Hash, ToFormat = "api-json" },
                new Wire.ConvertRequest { ModelHash = Hash },
            ],
            handler.RequestsFor("Convert").Select(sent => sent.Message(Wire.ConvertRequest.Parser)));
        Assert.Equal(CompleteConversionWire.Conversion.Content, conversion.Content);
        Assert.Equal(conversion.Content, conversion.ToString());
        Assert.Equal(("sysml", "ttl"), (conversion.FromFormat, conversion.ToFormat));
        Assert.True(conversion.Experimental);
        Assert.Equal("RDF conversion is experimental", conversion.ExperimentalNotice);
        Assert.Equal(["warning", "hint"], conversion.Diagnostics.Select(diagnostic => diagnostic.SeverityName));
        Assert.Equal(CompleteWire.Diagnostic.Code, conversion.Diagnostics[0].Code);
        Assert.Empty(conversion.LibraryReferences);
        Assert.Equal(
            [
                new LibraryReference("0c539f7c-20a1-520d-bae0-f3f855517aea", "SI::second", "AttributeUsage", string.Empty),
                new LibraryReference("2a413ac9-972d-5182-afd3-46cdde4e0eba", "SI", "LibraryPackage", string.Empty),
                new LibraryReference("1e506913-dbf3-55da-a841-56afa1d5e151", string.Empty, "BindingConnector", "Occurrences::Occurrence/BindingConnector[0]"),
            ],
            apiJson.LibraryReferences);
        Assert.Equal("SI (LibraryPackage)", apiJson.LibraryReferences[1].ToString());
        Assert.Equal("Occurrences::Occurrence/BindingConnector[0] (BindingConnector)", apiJson.LibraryReferences[2].ToString());
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task TheModelHelpersConvertTheParsedSourceToTheFormatTheyName(ClientWire wire)
    {
        using var handler = Serving(wire).On("Convert", Wire.ConvertRequest.Parser, request => new Wire.ConvertResponse
        {
            Content = request.ToFormat,
            FromFormat = "sysml",
            ToFormat = request.ToFormat,
        });
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash(Hash);
        var token = TestContext.Current.CancellationToken;
        var compact = new ConvertOptions { Compact = true };

        var notation = await model.ToNotationAsync(cancellationToken: token);
        var turtle = await model.ToTurtleAsync(cancellationToken: token);
        var json = await model.ToApiJsonAsync(compact, token);

        Assert.Equal(["sysml", "ttl", "api-json"], new[] { notation, turtle, json }.Select(conversion => conversion.Content));
        Assert.Equal(
            [
                new Wire.ConvertRequest { ModelHash = Hash, ToFormat = "sysml" },
                new Wire.ConvertRequest { ModelHash = Hash, ToFormat = "ttl" },
                new Wire.ConvertRequest { ModelHash = Hash, ToFormat = "api-json", Compact = true },
            ],
            handler.RequestsFor("Convert").Select(sent => sent.Message(Wire.ConvertRequest.Parser)));
        Assert.False(notation.Experimental);
        Assert.Equal(string.Empty, notation.ExperimentalNotice);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AConversionNotMadeIsAConversionExceptionKeepingItsFormatsAndAMalformedAnswerIsRefused(ClientWire wire)
    {
        using var handler = Serving(wire).On("Convert", Wire.ConvertRequest.Parser, request => request.Content switch
        {
            "refused" => CompleteConversionWire.FailedConversion,
            "unreadable" => new Wire.ConvertResponse { Error = "1 syntax error", FromFormat = "sysml", ToFormat = "sysml", Diagnostics = { CompleteWire.Diagnostic } },
            "both" => new Wire.ConvertResponse { Error = "e", Content = "package P;", FromFormat = "sysml", ToFormat = "sysml" },
            "unmarked" => new Wire.ConvertResponse { Content = "c", FromFormat = "sysml", ToFormat = "sysml", ExperimentalNotice = "experimental" },
            "formatless" => new Wire.ConvertResponse { Content = "c" },
            _ => new Wire.ConvertResponse { FromFormat = "sysml", ToFormat = "sysml" },
        });
        await using var connection = await FakeConnection.OpenAsync(handler);
        var token = TestContext.Current.CancellationToken;
        Task<Conversion> Convert(string content) => connection.ConvertAsync(ConvertSource.Inline(content), "api-json", new ConvertOptions { FromFormat = "sysml" }, token);

        var refused = await Assert.ThrowsAsync<ConversionException>(() => Convert("refused"));
        var unreadable = await Assert.ThrowsAsync<ConversionException>(() => Convert("unreadable"));
        var both = await Assert.ThrowsAsync<TransportException>(() => Convert("both"));
        var unmarked = await Assert.ThrowsAsync<TransportException>(() => Convert("unmarked"));
        var formatless = await Assert.ThrowsAsync<TransportException>(() => Convert("formatless"));
        var empty = await Convert("empty");

        Assert.Equal(("cannot write feature declaration", Connection.ConvertMethod), (refused.Message, refused.Method));
        Assert.Equal(("sysml", "api-json", true, "RDF conversion is experimental"), (refused.FromFormat, refused.ToFormat, refused.Experimental, refused.ExperimentalNotice));
        Assert.Single(refused.Diagnostics);
        Assert.IsAssignableFrom<ModelException>(refused);
        Assert.False(unreadable.Experimental);
        Assert.Equal(string.Empty, unreadable.ExperimentalNotice);
        Assert.Contains("beside content", both.Message, StringComparison.Ordinal);
        Assert.Contains("did not mark experimental", unmarked.Message, StringComparison.Ordinal);
        Assert.Contains("neither the formats of a conversion nor a failure", formatless.Message, StringComparison.Ordinal);
        Assert.All([both, unmarked, formatless], failure => Assert.Equal(StatusCode.Internal, failure.StatusCode));
        Assert.Equal(string.Empty, empty.Content);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task NotFoundIsTheFilesForAFileAndTheModelsForAHash(ClientWire wire)
    {
        using var handler = Serving(wire)
            .Refusing("Convert", StatusCode.NotFound, "file not found: /nope.sysml")
            .Refusing("Migrate", StatusCode.NotFound, "file not found: /nope_mtip.xml");
        await using var connection = await FakeConnection.OpenAsync(handler);
        var token = TestContext.Current.CancellationToken;

        var file = await Assert.ThrowsAsync<ModelFileNotFoundException>(() => connection.ConvertAsync(ConvertSource.FromFile("/nope.sysml"), "ttl", cancellationToken: token));
        var model = await Assert.ThrowsAsync<ModelNotFoundException>(() => connection.ModelByHash(Hash).ToTurtleAsync(cancellationToken: token));
        var inline = await Assert.ThrowsAsync<ServiceException>(() => connection.ConvertAsync(ConvertSource.Inline("x"), "ttl", cancellationToken: token));
        var layout = await Assert.ThrowsAsync<ModelFileNotFoundException>(() =>
            connection.MigrateAsync(MigrateSource.Inline([1]), "sysml", new MigrateOptions { FromFormat = "xmi", LayoutPath = "/nope_mtip.xml" }, token));

        Assert.Equal("file not found: /nope.sysml", file.ServiceMessage);
        Assert.Equal(Connection.ConvertMethod, model.Method);
        Assert.Equal(typeof(ServiceException), inline.GetType());
        Assert.Equal(StatusCode.NotFound, inline.StatusCode);
        Assert.Equal(Connection.MigrateMethod, layout.Method);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task TheWrongVerbIsSentAndRefusedByTheServiceNamingTheOtherOne(ClientWire wire)
    {
        const string Migrated = "inline content is a SysML v1 model, which is migrated, not converted; call Migrate with the same source";
        const string Converted = "inline content is sysml input, which is converted, not migrated; call Convert with the same source";
        using var handler = Serving(wire)
            .Refusing("Convert", StatusCode.InvalidArgument, Migrated)
            .Refusing("Migrate", StatusCode.InvalidArgument, Converted);
        await using var connection = await FakeConnection.OpenAsync(handler);
        var token = TestContext.Current.CancellationToken;

        var convert = await Assert.ThrowsAsync<ServiceException>(() =>
            connection.ConvertAsync(ConvertSource.Inline("<xmi:XMI/>"), "sysml", new ConvertOptions { FromFormat = ConversionFormat.Xmi }, token));
        var file = await Assert.ThrowsAsync<ServiceException>(() => connection.ConvertAsync(ConvertSource.FromFile("/m/Model.mdzip"), "sysml", cancellationToken: token));
        var migrate = await Assert.ThrowsAsync<ServiceException>(() =>
            connection.MigrateAsync(MigrateSource.Inline("package P;"u8), "sysml", new MigrateOptions { FromFormat = ConversionFormat.SysML }, token));
        var target = await Assert.ThrowsAsync<ServiceException>(() => connection.MigrateAsync(MigrateSource.FromFile(Path.GetFullPath("/m/a.xmi")), ConversionFormat.Xmi, cancellationToken: token));

        Assert.Equal((StatusCode.InvalidArgument, Migrated), (convert.StatusCode, convert.ServiceMessage));
        Assert.Equal(StatusCode.InvalidArgument, file.StatusCode);
        Assert.Equal((StatusCode.InvalidArgument, Converted), (migrate.StatusCode, migrate.ServiceMessage));
        Assert.Equal(StatusCode.InvalidArgument, target.StatusCode);
        Assert.Equal(
            [
                new Wire.ConvertRequest { Content = "<xmi:XMI/>", FromFormat = "xmi", ToFormat = "sysml" },
                new Wire.ConvertRequest { FilePath = "/m/Model.mdzip", ToFormat = "sysml" },
            ],
            handler.RequestsFor("Convert").Select(sent => sent.Message(Wire.ConvertRequest.Parser)));
        Assert.Equal(2, handler.RequestsFor("Migrate").Count);
    }

    [Fact]
    public async Task ConvertGatesAreCheckedInTheServicesOrderBeforeAnythingIsSent()
    {
        var token = TestContext.Current.CancellationToken;
        var all = new ConvertOptions { Documents = ["a.sysml"], IncludeLibraryStubs = true, KeepDerived = ["owner"], IncludeCheckDiagnostics = true };
        Task Convert(Model model, ConvertOptions options) => model.ConvertAsync("ttl", options, token);

        Assert.Equal(Capabilities.Convert, await RefusedAsync(ClientWire.Grpc, [Capabilities.Convert, Capabilities.ConvertDocuments], model => Convert(model, all)));
        Assert.Equal(Capabilities.ConvertDocuments, await RefusedAsync(ClientWire.Grpc, [Capabilities.ConvertDocuments, Capabilities.ConvertLibraryStubs], model => Convert(model, all)));
        Assert.Equal(Capabilities.ConvertLibraryStubs, await RefusedAsync(ClientWire.Grpc, [Capabilities.ConvertLibraryStubs, Capabilities.ConvertCompact], model => Convert(model, all)));
        Assert.Equal(Capabilities.ConvertLibraryStubs, await RefusedAsync(ClientWire.Grpc, [Capabilities.ConvertLibraryStubs], model => Convert(model, new ConvertOptions { IncludeLibraryStubs = true })));
        Assert.Equal(Capabilities.ConvertCompact, await RefusedAsync(ClientWire.Grpc, [Capabilities.ConvertCompact, Capabilities.ConvertCheckDiagnostics], model => Convert(model, all)));
        Assert.Equal(Capabilities.ConvertCheckDiagnostics, await RefusedAsync(ClientWire.Grpc, [Capabilities.ConvertCheckDiagnostics], model => Convert(model, all)));
        Assert.Equal(Capabilities.ConvertCompact, await RefusedAsync(ClientWire.Grpc, [Capabilities.ConvertCompact], model => Convert(model, new ConvertOptions { Compact = true })));
        Assert.Equal(Capabilities.ConvertCompact, await RefusedAsync(ClientWire.Grpc, [Capabilities.ConvertCompact], model => Convert(model, new ConvertOptions { OmitDerived = true })));
        Assert.Equal(Capabilities.Migrate, await RefusedAsync(ClientWire.Grpc, [Capabilities.Migrate], model => model.Connection.MigrateAsync(MigrateSource.FromFile(Path.GetFullPath("/m/a.xmi")), "sysml", cancellationToken: token)));

        using var handler = Serving(ClientWire.Grpc);
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash(Hash);
        var source = MigrateSource.FromFile(Path.GetFullPath("/m/a.xmi"));
        await Assert.ThrowsAsync<ArgumentException>(() => Convert(model, new ConvertOptions { FromFormat = string.Empty }));
        await Assert.ThrowsAsync<ArgumentException>(() => Convert(model, new ConvertOptions { IdForm = (IdForm)7 }));
        await Assert.ThrowsAsync<ArgumentException>(() => connection.MigrateAsync(source, "sysml", new MigrateOptions { FromFormat = string.Empty }, token));
        await Assert.ThrowsAsync<ArgumentException>(() => connection.MigrateAsync(source, "sysml", new MigrateOptions { LayoutPath = string.Empty }, token));
        await Assert.ThrowsAsync<ArgumentException>(() => connection.MigrateAsync(source, "sysml", new MigrateOptions { LayoutContent = string.Empty }, token));
        await Assert.ThrowsAsync<ArgumentException>(() => connection.MigrateAsync(source, "sysml", new MigrateOptions { ImageBaseUrl = string.Empty }, token));
        var layouts = await Assert.ThrowsAsync<ArgumentException>(() =>
            connection.MigrateAsync(source, "sysml", new MigrateOptions { LayoutPath = "/m/a_mtip.xml", LayoutContent = "<mtip/>" }, token));
        await Assert.ThrowsAsync<ArgumentNullException>(() => connection.ConvertAsync(null!, "ttl", cancellationToken: token));
        await Assert.ThrowsAsync<ArgumentNullException>(() => connection.ConvertAsync(ConvertSource.Inline("x"), null!, cancellationToken: token));
        await Assert.ThrowsAsync<ArgumentNullException>(() => connection.MigrateAsync(null!, "sysml", cancellationToken: token));
        await Assert.ThrowsAsync<ArgumentNullException>(() => connection.MigrateAsync(source, null!, cancellationToken: token));
        var unnamed = await Assert.ThrowsAsync<ArgumentException>(() => model.SaveAsync(Path.Combine(Path.GetTempPath(), "model.txt"), cancellationToken: token));
        Assert.Contains("one MTIP export", layouts.Message, StringComparison.Ordinal);
        Assert.Contains("names a format", unnamed.Message, StringComparison.Ordinal);
        Assert.Equal(["GetServerInfo"], handler.Requests.Select(request => request.Method));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task MigrationSourcesRequireFullyQualifiedPathsBeforeAnythingIsSent(ClientWire wire)
    {
        using var handler = Serving(wire).On("Migrate", Wire.MigrateRequest.Parser, _ => CompleteConversionWire.Migration);
        await using var connection = await FakeConnection.OpenAsync(handler);
        foreach (var path in new[] { "Vehicle.xmi", "../Vehicle.xmi", "models/Vehicle.xmi", @"C:Vehicle.xmi" })
        {
            var refused = await Assert.ThrowsAsync<ArgumentException>(() =>
                connection.MigrateAsync(MigrateSource.FromFile(path), "sysml", cancellationToken: TestContext.Current.CancellationToken));
            Assert.Equal("path", refused.ParamName);
            Assert.Contains("fully-qualified", refused.Message, StringComparison.Ordinal);
        }

        Assert.Throws<ArgumentException>(() => MigrateSource.FromFile(Path.Combine(Path.GetTempPath(), "Mo\0del.xmi")));
        if (OperatingSystem.IsWindows())
        {
            Assert.Throws<ArgumentException>(() => MigrateSource.FromFile(@"\models\Vehicle.xmi"));
            Assert.Throws<ArgumentException>(() => MigrateSource.FromFile("/models/Vehicle.xmi"));
        }

        Assert.Equal(["GetServerInfo"], handler.Requests.Select(request => request.Method));
        var normalized = Path.Combine(Path.GetTempPath(), "models", "..", "Vehicle.xmi");
        var source = MigrateSource.FromFile(normalized);
        var migration = await connection.MigrateAsync(source, "sysml", cancellationToken: TestContext.Current.CancellationToken);

        Assert.Equal(Path.GetFullPath(normalized), source.FilePath);
        Assert.Equal(source.FilePath, Assert.Single(handler.RequestsFor("Migrate")).Message(Wire.MigrateRequest.Parser).FilePath);
        Assert.Equal(source.FilePath, migration.SourcePath);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task MigrateSendsEverySourceAndOptionAndReadsEveryField(ClientWire wire)
    {
        using var handler = Serving(wire).On("Migrate", Wire.MigrateRequest.Parser, _ => CompleteConversionWire.Migration);
        await using var connection = await FakeConnection.OpenAsync(handler);
        var token = TestContext.Current.CancellationToken;
        byte[] bytes = [0x50, 0x4b, 0x03, 0x04];
        var inline = MigrateSource.Inline(bytes);
        bytes[0] = 0;
        var options = new MigrateOptions
        {
            FromFormat = "mdzip",
            Report = true,
            Results = true,
            LayoutContent = "<mtip/>",
            ImageBaseUrl = "https://example.com",
            Strict = true,
        };

        var migration = await connection.MigrateAsync(inline, "ttl", options, token);
        var sourcePath = Path.GetFullPath("Model.xmi");
        var fromFile = await connection.MigrateAsync(MigrateSource.FromFile(sourcePath), "sysml", new MigrateOptions { LayoutPath = "/m/Model_mtip.xml" }, token);

        Assert.Equal(
            [
                new Wire.MigrateRequest
                {
                    Content = ByteString.CopyFrom(0x50, 0x4b, 0x03, 0x04),
                    FromFormat = "mdzip",
                    ToFormat = "ttl",
                    Report = true,
                    Results = true,
                    LayoutContent = "<mtip/>",
                    ImageBaseUrl = "https://example.com",
                    Strict = true,
                },
                new Wire.MigrateRequest { FilePath = sourcePath, ToFormat = "sysml", LayoutPath = "/m/Model_mtip.xml" },
            ],
            handler.RequestsFor("Migrate").Select(sent => sent.Message(Wire.MigrateRequest.Parser)));
        Assert.Equal(CompleteConversionWire.Migration.Content, migration.Content);
        Assert.Equal(migration.Content, migration.ToString());
        Assert.Equal(("xmi", "sysml", true, "SysML v1 migration is experimental"), (migration.FromFormat, migration.ToFormat, migration.Experimental, migration.ExperimentalNotice));
        Assert.Null(migration.SourcePath);
        Assert.Equal(sourcePath, fromFile.SourcePath);
        var report = migration.Report;
        Assert.Equal(("<content>", "MagicDraw Clean XMI Exporter"), (report.Source, report.Exporter));
        Assert.StartsWith("migrated 4 element(s)", report.Summary, StringComparison.Ordinal);
        Assert.Equal((1, 1, 1, 1, 4L), (report.Mapped, report.Approximated, report.Unmapped, report.Skipped, report.Total));
        Assert.Equal(report.Total, report.Entries.Count);
        Assert.Equal(
            new MigrationEntry("_p1", "«ValueProperty»", "Vehicle::mass", "attribute Vehicle::Vehicle::mass", "approximated", "unit restated"),
            Assert.Single(report.ByVerdict("approximated")));
        Assert.Equal(string.Empty, Assert.Single(report.ByVerdict("unmapped")).Target);
        Assert.Empty(report.ByVerdict("unknown"));
        Assert.Equal(
            [report.Mapped, report.Approximated, report.Unmapped, report.Skipped],
            Verdicts.Select(verdict => report.ByVerdict(verdict).Count));
        Assert.Equal("# SysML v1 to v2 migration report\n", report.Text);
        Assert.Equal("{\n  \"runs\": []\n}\n", migration.Results);
        Assert.Equal([new MigrationFile("images/fleet.png", CompleteConversionWire.Picture), new MigrationFile("images/empty.png", [])], migration.Files);
        Assert.Equal(CompleteConversionWire.Picture, migration.Files[0].Content.ToArray());
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AMigrationNotMadeIsAMigrationExceptionAndAMalformedAnswerIsRefused(ClientWire wire)
    {
        using var handler = Serving(wire).On("Migrate", Wire.MigrateRequest.Parser, request => Path.GetFileName(request.FilePath) switch
        {
            "failed" => CompleteConversionWire.FailedMigration,
            "content" => new Wire.MigrateResponse { Error = "e", Content = "c" },
            "report" => new Wire.MigrateResponse { Error = "e", Report = new Wire.MigrationReport() },
            "results" => new Wire.MigrateResponse { Error = "e", Results = "{}" },
            "files" => new Wire.MigrateResponse { Error = "e", Files = { new Wire.MigrationFile { Path = "a.png" } } },
            "unmarked" => new Wire.MigrateResponse { FromFormat = "xmi", ToFormat = "sysml", Report = new Wire.MigrationReport(), ExperimentalNotice = "n" },
            "formatless" => new Wire.MigrateResponse { Report = new Wire.MigrationReport() },
            _ => new Wire.MigrateResponse { Content = "c", FromFormat = "xmi", ToFormat = "sysml", Experimental = true },
        });
        await using var connection = await FakeConnection.OpenAsync(handler);
        var token = TestContext.Current.CancellationToken;
        Task<Migration> Migrate(string path) => connection.MigrateAsync(MigrateSource.FromFile(Path.Combine(Path.GetTempPath(), path)), "sysml", cancellationToken: token);

        var failed = await Assert.ThrowsAsync<MigrationException>(() => Migrate("failed"));
        var malformed = new List<(string Phrase, TransportException Failure)>();
        foreach (var (path, phrase) in new Dictionary<string, string>
        {
            ["content"] = "beside content",
            ["report"] = "beside report",
            ["results"] = "beside results",
            ["files"] = "beside files",
            ["unmarked"] = "did not mark experimental",
            ["formatless"] = "neither a migration",
            ["reportless"] = "neither a migration",
        })
        {
            malformed.Add((phrase, await Assert.ThrowsAsync<TransportException>(() => Migrate(path))));
        }

        Assert.Equal(("<content>: not an XMI document", Connection.MigrateMethod), (failed.Message, failed.Method));
        Assert.Equal(("xmi", "ttl", true, "SysML v1 migration is experimental"), (failed.FromFormat, failed.ToFormat, failed.Experimental, failed.ExperimentalNotice));
        Assert.Empty(failed.Diagnostics);
        Assert.Equal(7, malformed.Count);
        Assert.All(malformed, entry =>
        {
            Assert.Equal(StatusCode.Internal, entry.Failure.StatusCode);
            Assert.Contains(entry.Phrase, entry.Failure.Message, StringComparison.Ordinal);
        });
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task SavingAModelConvertsItToTheFormatItsExtensionNamesAndWritesItWhole(ClientWire wire)
    {
        using var handler = Serving(wire).On("Convert", Wire.ConvertRequest.Parser, request => new Wire.ConvertResponse
        {
            Content = $"written as {request.ToFormat}\r\n",
            FromFormat = "sysml",
            ToFormat = request.ToFormat,
        });
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash(Hash);
        var token = TestContext.Current.CancellationToken;
        var directory = Directory.CreateTempSubdirectory("opensysml-save-").FullName;
        try
        {
            var turtle = Path.Combine(directory, "model.TTL");
            var named = Path.Combine(directory, "model.out");

            var saved = await model.SaveAsync(turtle, cancellationToken: token);
            var refused = await Assert.ThrowsAsync<UnwritableException>(() => model.SaveAsync(turtle, cancellationToken: token));
            await model.SaveAsync(turtle, ConversionFormat.KerML, saveOptions: new SaveOptions { Overwrite = true }, cancellationToken: token);
            await model.SaveAsync(named, "json", new ConvertOptions { Compact = true }, cancellationToken: token);

            Assert.Equal("ttl", saved.ToFormat);
            Assert.Equal("written as kerml\r\n"u8.ToArray(), await File.ReadAllBytesAsync(turtle, token));
            Assert.Equal("written as json\r\n", await File.ReadAllTextAsync(named, token));
            Assert.Equal(turtle, refused.Path);
            Assert.Equal(["ttl", "ttl", "kerml", "json"], handler.RequestsFor("Convert").Select(sent => sent.Message(Wire.ConvertRequest.Parser).ToFormat));
            Assert.True(handler.RequestsFor("Convert")[^1].Message(Wire.ConvertRequest.Parser).Compact);
            Assert.Equal(["model.TTL", "model.out"], Directory.GetFileSystemEntries(directory).Select(Path.GetFileName).Order(StringComparer.Ordinal));
        }
        finally
        {
            Directory.Delete(directory, recursive: true);
        }
    }

    [Fact]
    public async Task TheOptionsCopyTheirCollectionsAndTheRecordsRefuseWhatTheyCannotHold()
    {
        var documents = new List<string> { "a.sysml" };
        var keep = new List<string> { "owner" };
        var options = new ConvertOptions { Documents = documents, KeepDerived = keep };
        byte[] bytes = [1, 2, 3];
        var file = new MigrationFile("a.png", bytes);
        bytes[0] = 9;

        documents.Clear();
        keep.Clear();

        Assert.Equal(["a.sysml"], options.Documents);
        Assert.Equal(["owner"], options.KeepDerived);
        Assert.Empty(ConvertOptions.Default.Documents);
        Assert.Empty(ConvertOptions.Default.KeepDerived);
        Assert.Null(ConvertOptions.Default.IdForm);
        Assert.False(SaveOptions.Default.Overwrite);
        Assert.Null(MigrateOptions.Default.FromFormat);
        Assert.Equal([1, 2, 3], file.Content.ToArray());
        Assert.Equal(new MigrationFile("a.png", [1, 2, 3]), file);
        Assert.Equal(new MigrationFile("a.png", [1, 2, 3]).GetHashCode(), file.GetHashCode());
        Assert.NotEqual(new MigrationFile("a.png", [1, 2]), file);
        Assert.NotEqual(new MigrationFile("b.png", [1, 2, 3]), file);
        Assert.False(file.Equals(null));
        Assert.Equal("a.png (3 bytes)", file.ToString());
        Assert.Equal(Path.GetFullPath("/m/a.xmi"), MigrateSource.FromFile(Path.GetFullPath("/m/a.xmi")).FilePath);
        Assert.Null(MigrateSource.FromFile(Path.GetFullPath("/m/a.xmi")).Content);
        Assert.Null(MigrateSource.Inline([1]).FilePath);
        Assert.Equal([1, 2, 3], MigrateSource.Inline([1, 2, 3]).Content!.Value.ToArray());
        Assert.Equal(("a", null, null), (ConvertSource.FromFile("a").FilePath, ConvertSource.FromFile("a").Content, ConvertSource.FromFile("a").ModelHash));
        Assert.Equal((null, "", null), (ConvertSource.Inline(string.Empty).FilePath, ConvertSource.Inline(string.Empty).Content, ConvertSource.Inline(string.Empty).ModelHash));
        Assert.Equal("h", ConvertSource.FromModelHash("h").ModelHash);
        Assert.Equal(
            [ConversionFormat.SysML, ConversionFormat.KerML, ConversionFormat.Turtle, ConversionFormat.Turtle, ConversionFormat.ApiJson, null, null, null],
            Named.Select(ConversionFormat.OfPath));
        Assert.Throws<ArgumentException>(() => new ConvertOptions { Documents = [null!] });
        Assert.Throws<ArgumentException>(() => new ConvertOptions { KeepDerived = [null!] });
        Assert.Throws<ArgumentException>(() => ConvertSource.FromFile(string.Empty));
        Assert.Throws<ArgumentNullException>(() => ConvertSource.Inline(null!));
        Assert.Throws<ArgumentException>(() => ConvertSource.FromModelHash(" "));
        Assert.Throws<ArgumentException>(() => MigrateSource.FromFile(string.Empty));
        Assert.Throws<ArgumentNullException>(() => ConversionFormat.OfPath(null!));
        Assert.Throws<ArgumentNullException>(() => new MigrationFile(null!, []));
        Assert.Throws<ArgumentNullException>(() => new MigrationEntry("i", "k", "n", "t", "v", null!));
        Assert.Throws<ArgumentNullException>(() => new MigrationReport("s", "e", "m", 0, 0, 0, 0, [], null!));
        Assert.Throws<ArgumentException>(() => new MigrationReport("s", "e", "m", 0, 0, 0, 0, [null!], ""));
        Assert.Throws<ArgumentNullException>(() => new MigrationReport("s", "e", "m", 0, 0, 0, 0, [], "").ByVerdict(null!));
        Assert.Throws<ArgumentNullException>(() => new Conversion("c", "sysml", "sysml", [], false, null!, []));
        Assert.Throws<ArgumentNullException>(() => new Conversion("c", "sysml", "sysml", [], false, string.Empty, null!));
        Assert.Throws<ArgumentException>(() => new Conversion("c", "sysml", "sysml", [], false, string.Empty, [null!]));
        Assert.Throws<ArgumentNullException>(() => new LibraryReference("SI::s", null!, "AttributeUsage", string.Empty));
        Assert.Throws<ArgumentNullException>(() => new LibraryReference("SI::s", "SI::s", "AttributeUsage", null!));
        Assert.Throws<ArgumentNullException>(() => new Migration("c", "xmi", "sysml", null!, "", [], true, "n", null));
        Assert.Throws<ArgumentException>(() => new Migration("c", "xmi", "sysml", new MigrationReport("s", "e", "m", 0, 0, 0, 0, [], ""), "", [null!], true, "n", null));
        Assert.Throws<ArgumentNullException>(() => new ConversionException("m", Connection.ConvertMethod, [], "sysml", null!, false, ""));
        Assert.Throws<ArgumentNullException>(() => new MigrationException("m", Connection.MigrateMethod, null!, "sysml", true, ""));
        Assert.Throws<ArgumentNullException>(() => new UnwritableException("m", null!));
        var conversion = new Conversion("c", "sysml", "sysml", [], false, string.Empty, []);
        await Assert.ThrowsAsync<ArgumentException>(() => conversion.SaveAsync(string.Empty, cancellationToken: TestContext.Current.CancellationToken));
        await Assert.ThrowsAsync<ArgumentNullException>(() =>
            new Migration("c", "xmi", "sysml", new MigrationReport("s", "e", "m", 0, 0, 0, 0, [], ""), "", [], true, "n", null).WriteAsync(null!, cancellationToken: TestContext.Current.CancellationToken));
    }

    private static FakeServiceHandler Serving(ClientWire wire) => new(wire) { Capabilities = FakeModels.ModelCapabilities };

    // The capability a call is refused for when the service lacks the given ones, after checking nothing was sent.
    private static async Task<string> RefusedAsync(ClientWire wire, string[] withheld, Func<Model, Task> call)
    {
        using var handler = new FakeServiceHandler(wire) { Capabilities = [.. FakeModels.ModelCapabilities.Except(withheld)] };
        await using var connection = await FakeConnection.OpenAsync(handler);

        var refused = await Assert.ThrowsAsync<CapabilityException>(() => call(connection.ModelByHash(Hash)));

        Assert.Equal(["GetServerInfo"], handler.Requests.Select(request => request.Method));
        return refused.Capability;
    }
}
