using System;
using System.Collections.Concurrent;
using System.IO;
using System.Linq;
using System.Net.Http;
using System.Text.Json;
using System.Threading;
using System.Threading.Tasks;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// Exporting graphs, importing data and comparing a tool's results against a real sysml-grpc over every client wire: an
/// action's and a state machine's form kept as the service wrote it, the same text on every wire; CSV, TSV, JSON and
/// JSON Lines imported in order into a new model while the old one stays as it was; a dry run and a refused import that
/// change nothing and make no model; a model of several documents imported into as one; a tool's stored and pooled
/// distributions compared with seeded runs, the case's declared statistics, failed runs and unresolved configurations;
/// the service's refusals as statuses; and a service without the capabilities asked nothing.
/// </summary>
[Trait(Category.Name, Category.Integration)]
public sealed class ServiceGraphDataTests(ServiceFixture services) : IClassFixture<ServiceFixture>, IDisposable
{
    private const string Withheld = "export_graphs,import_data,compare_results";

    // Two documents, the engine's power overridable where the other declares a motor.
    private static readonly SourceDocument[] EngineDocuments =
    [
        SourceDocument.Inline("package Lib {\n\tprivate import ScalarValues::*;\n\tpart def Engine {\n\t\tattribute power : Integer default 150;\n\t}\n}\n", "lib.sysml"),
        SourceDocument.Inline("package User {\n\tprivate import Lib::*;\n\tpart def Car {\n\t\tpart motor : Engine;\n\t}\n}\n", "user.sysml"),
    ];

    private const string CompareModel = """
        package Cfg {
        	private import ScalarValues::*;
        	private import RandomFunctions::*;
        	part def Probe {
        		attribute total : Real = 1.0;
        		action shaky {
        			attribute k : Integer = uniformInteger(0, 1);
        			first start;
        			then assign total := 10.0 / k;
        			then done;
        		}
        		action steady { first start; then assign total := 4.0; then done; }
        	}
        	action def 'Group 0' { part target : Probe; perform action run ::> target.shaky; }
        	action def 'Group 1' { part target : Probe; perform action run ::> target.steady; }
        }
        """;

    private const string Results = """
        {"source":"probe.xmi","configurations":[
         {"id":"_c1","name":"Cfg::'Group 1'","runs":2,"target":"target","behavior":"run","resultLocation":"Results",
          "analysis":"total","analysisCase":"Cfg::TotalCase","statistics":["N","Mean","Deviation"],"observables":["total"],
          "snapshots":[{"id":"_m","name":"summary","values":{},"statistics":{"observable":"total","runs":10,"mean":5.0,"deviation":1.0}}]},
         {"id":"_c0","name":"Cfg::'Group 0'","runs":3,"target":"target","behavior":"run","observables":["total"],
          "snapshots":[{"id":"_s1","values":{"total":10.0}}]},
         {"id":"_cn","name":"Cfg::Nothing","observables":[],"snapshots":[],"notes":["migrated without a behavior"]}]}
        """;

    private readonly string directory = Directory.CreateTempSubdirectory("opensysml-graph-data-").FullName;

    public static TheoryData<ClientWire> Wires => ClientWires.All;

    private static CancellationToken Token => TestContext.Current.CancellationToken;

    public void Dispose() => Directory.Delete(directory, recursive: true);

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AnActionAndAStateMachineExportTheFormAsTheServiceWroteIt(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var model = (await connection.LoadContentAsync(await File.ReadAllTextAsync(Fixture("behavior.sysml"), Token), cancellationToken: Token)).RequireNoErrors();

        var action = await model.ExportGraphsAsync("Test::race", Token);
        var machine = await model.ExportGraphsAsync("Test::Machine", Token);

        Assert.Equal((1, "Test::race", 1, "Test::Machine"), (action.Version, action.Subject, machine.Version, machine.Subject));
        Assert.StartsWith("{\"version\":1,\"subject\":\"Test::race\",\"actions\":[", action.Content, StringComparison.Ordinal);
        Assert.Contains("\"states\"", machine.Content, StringComparison.Ordinal);
        Assert.All([action, machine], graphs =>
        {
            Assert.EndsWith("}\n", graphs.Content, StringComparison.Ordinal);
            Assert.DoesNotContain("\n", graphs.Content.TrimEnd('\n'), StringComparison.Ordinal);
            using var form = JsonDocument.Parse(graphs.Content);
            Assert.Equal(graphs.Subject, form.RootElement.GetProperty("subject").GetString());
            Assert.Equal(graphs.Version, form.RootElement.GetProperty("version").GetInt32());
        });

        // The service escapes the markup characters of the inline document's name, <content>; the client keeps the escapes.
        Assert.Contains("\"document\":\"\\u003ccontent\\u003e\"", action.Content, StringComparison.Ordinal);
    }

    [Fact]
    public async Task TheFormIsTheSameTextOverEveryWire()
    {
        var service = await services.ServiceAsync("connect");
        var model = await LoadOverAsync(ClientWire.Grpc, service.Address, "behavior.sysml");
        var forms = new string[3];
        foreach (var (wire, i) in new[] { ClientWire.Grpc, ClientWire.ConnectProto, ClientWire.ConnectJson }.Select((wire, i) => (wire, i)))
        {
            await using var connection = await Connection.OpenAsync(wire.To(service.Address), Token);
            forms[i] = (await connection.ModelByHash(model).ExportGraphsAsync("Test::Machine", Token)).Content;
        }

        Assert.Equal(forms[0], forms[1]);
        Assert.Equal(forms[0], forms[2]);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AnExportOfAMissingSubjectANonBehaviorOrAMissingModelIsRefused(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var model = (await connection.LoadAsync(Fixture("behavior.sysml"), cancellationToken: Token)).RequireNoErrors();

        var missing = await Assert.ThrowsAsync<ServiceException>(() => model.ExportGraphsAsync("Test::Missing", Token));
        var invalid = await Assert.ThrowsAsync<ServiceException>(() => model.ExportGraphsAsync("Test", Token));
        var gone = await Assert.ThrowsAsync<ModelNotFoundException>(() => connection.ModelByHash("0000").ExportGraphsAsync("Test::race", Token));

        Assert.Equal((typeof(ServiceException), StatusCode.NotFound), (missing.GetType(), missing.StatusCode));
        Assert.Contains("Test::Missing", missing.ServiceMessage, StringComparison.Ordinal);
        Assert.Equal(StatusCode.InvalidArgument, invalid.StatusCode);
        Assert.Contains("no lowered graph", invalid.ServiceMessage, StringComparison.Ordinal);
        Assert.Equal("model not found: 0000", gone.ServiceMessage);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task EveryFormatIsImportedInOrderIntoANewModelLeavingTheOldOneAsItWas(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var model = (await connection.LoadAsync(Fixture("import.sysml"), cancellationToken: Token)).RequireNoErrors();
        var before = (await model.ToNotationAsync(cancellationToken: Token)).Content;
        DataImport[] imports =
        [
            new("fleet.csv", "element,count,label\nV::vehicle,7,fast\nV::car,9,\n"),
            new("fleet.tsv", "element\tlabel\nV::car\tslow\n"),
            new("fleet.json", "[{\"element\":\"V::vehicle\",\"label\":\"quick\"}]"),
            new("fleet.txt", "{\"id\":\"car\",\"n\":3}\n") { Format = "jsonl", Map = "{\"element\":{\"path\":\"/id\",\"prefix\":\"V::\"},\"features\":{\"count\":{\"path\":\"/n\"}}}" },
        ];

        var result = await model.ImportDataAsync(imports, cancellationToken: Token);

        Assert.Equal(["fleet.csv", "fleet.tsv", "fleet.json", "fleet.txt"], result.Files.Select(file => file.Name));
        Assert.Equal([3L, 1, 1, 1], result.Files.Select(file => file.Values));
        Assert.Equal(6, result.Values);
        Assert.Equal(
            ["V::vehicle::count = 7 (redefines V::Vehicle::count)", "V::vehicle::label = \"fast\" (redefines V::Vehicle::label)", "V::car::count = 9"],
            result.Files[0].Assignments.Select(value => value.Description));
        Assert.Equal(("V::car", "count", "3", string.Empty), (result.Files[3].Assignments[0].Element, result.Files[3].Assignments[0].Feature, result.Files[3].Assignments[0].Value, result.Files[3].Assignments[0].Redefines));
        Assert.All(result.Files.SelectMany(file => file.Assignments), value => Assert.NotEmpty(value.Where));
        Assert.Contains("attribute :>> count = 3;", result.Content, StringComparison.Ordinal);
        Assert.Contains("attribute :>> label = \"quick\";", result.Content, StringComparison.Ordinal);
        Assert.Contains("attribute :>> label = \"slow\";", result.Content, StringComparison.Ordinal);
        Assert.Equal(result.Content, Assert.Single(result.Documents).Content);
        Assert.NotEqual(model.Hash, result.Model!.Hash);
        Assert.Contains("count = 3", (await result.Model.ToNotationAsync(cancellationToken: Token)).Content, StringComparison.Ordinal);
        Assert.Equal(before, (await model.ToNotationAsync(cancellationToken: Token)).Content);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ADryRunReportsWhatTheImportWouldSetAndChangesNothing(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var model = (await connection.LoadAsync(Fixture("import.sysml"), cancellationToken: Token)).RequireNoErrors();
        var before = (await model.ToNotationAsync(cancellationToken: Token)).Content;
        DataImport[] imports = [new("fleet.tsv", "element\tcount\nV::vehicle\t7\n")];

        var dry = await model.ImportDataAsync(imports, new ImportOptions { DryRun = true }, Token);
        var made = await model.ImportDataAsync(imports, cancellationToken: Token);

        Assert.True(dry.IsDryRun);
        Assert.Null(dry.Model);
        Assert.Null(dry.Content);
        Assert.Empty(dry.Documents);
        Assert.Equal(made.Files, dry.Files);
        Assert.Equal(before, (await model.ToNotationAsync(cancellationToken: Token)).Content);
        Assert.NotNull(made.Model);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ARefusedImportIsAModelExceptionAndChangesNothing(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var model = (await connection.LoadAsync(Fixture("import.sysml"), cancellationToken: Token)).RequireNoErrors();
        var before = (await model.ToNotationAsync(cancellationToken: Token)).Content;

        var unknown = await Assert.ThrowsAsync<ModelException>(
            () => model.ImportDataAsync([new DataImport("a.csv", "element,count\nV::car,9\n"), new DataImport("b.csv", "element,count\nV::Nowhere,1\n")], cancellationToken: Token));
        var unread = await Assert.ThrowsAsync<ModelException>(() => model.ImportDataAsync([new DataImport("c.csv", "element,count\nV::car,x\n")], cancellationToken: Token));
        var unmapped = await Assert.ThrowsAsync<ModelException>(() => model.ImportDataAsync([new DataImport("d.txt", "{}") { Format = "jsonl", Map = "{\"element\":1}" }], cancellationToken: Token));
        var none = await Assert.ThrowsAsync<ServiceException>(() => model.ImportDataAsync([], cancellationToken: Token));
        var form = await Assert.ThrowsAsync<ServiceException>(() => model.ImportDataAsync([new DataImport("a.csv", "element\n")], new ImportOptions { As = "parts" }, Token));
        var gone = await Assert.ThrowsAsync<ModelNotFoundException>(() => connection.ModelByHash("0000").ImportDataAsync([new DataImport("a.csv", "element\n")], cancellationToken: Token));

        Assert.All([unknown, unread, unmapped], refused => Assert.Equal((typeof(ModelException), Model.ImportDataMethod), (refused.GetType(), refused.Method)));
        Assert.Contains("V::Nowhere", unknown.Message, StringComparison.Ordinal);
        Assert.Contains("c.csv line 2", unread.Message, StringComparison.Ordinal);
        Assert.Contains("mapping", unmapped.Message, StringComparison.Ordinal);
        Assert.Equal((StatusCode.InvalidArgument, StatusCode.InvalidArgument), (none.StatusCode, form.StatusCode));
        Assert.Contains("imports is required", none.ServiceMessage, StringComparison.Ordinal);
        Assert.Contains("no longer cached", gone.ServiceMessage, StringComparison.Ordinal);
        Assert.Equal(before, (await model.ToNotationAsync(cancellationToken: Token)).Content);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AModelOfSeveralDocumentsIsImportedIntoAsOne(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var several = (await connection.ParseSourcesAsync(EngineDocuments, cancellationToken: Token)).RequireNoErrors();
        DataImport[] imports = [new("power.csv", "element,power\nUser::Car::motor,200\n")];

        var result = await several.ImportDataAsync(imports, cancellationToken: Token);
        var declined = await Assert.ThrowsAsync<ServiceException>(() => several.ImportDataAsync(imports, new ImportOptions { AcceptDocuments = false }, Token));

        Assert.Null(result.Content);
        var edited = Assert.Single(result.Documents);
        Assert.Equal("user.sysml", edited.Name);
        Assert.Contains(":>> power = 200;", edited.Content, StringComparison.Ordinal);
        Assert.Equal("Lib::Engine::power", Assert.Single(result.Files[0].Assignments).Redefines);
        Assert.NotEqual(several.Hash, result.Model!.Hash);
        Assert.Equal(StatusCode.FailedPrecondition, declined.StatusCode);
        var written = await edited.SaveAsync(Path.Combine(directory, edited.Name), cancellationToken: Token);
        Assert.Equal(edited.Content, await File.ReadAllTextAsync(written, Token));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task TheImportedModelIsHeldByTheServiceAndItsNotationSavesAndLoadsBack(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var model = (await connection.LoadAsync(Fixture("import.sysml"), cancellationToken: Token)).RequireNoErrors();

        var first = await model.ImportDataAsync([new DataImport("a.csv", "element,count\nV::car,9\n")], cancellationToken: Token);
        var second = await first.Model!.ImportDataAsync([new DataImport("b.csv", "element,label\nV::car,fast\n")], cancellationToken: Token);
        var unchanged = await second.Model!.ImportDataAsync([new DataImport("c.csv", "element\nV::car\n")], cancellationToken: Token);
        var path = await second.SaveAsync(Path.Combine(directory, "fleet.sysml"), cancellationToken: Token);
        var reloaded = (await connection.LoadAsync(path, cancellationToken: Token)).RequireNoErrors();

        Assert.Contains("count = 9", second.Content, StringComparison.Ordinal);
        Assert.Contains("label = \"fast\"", second.Content, StringComparison.Ordinal);
        Assert.Equal(second.Model.Hash, unchanged.Model!.Hash);
        Assert.Null(unchanged.Content);
        Assert.Equal(second.Content, await File.ReadAllTextAsync(path, Token));
        Assert.Contains("label = \"fast\"", (await reloaded.ToNotationAsync(cancellationToken: Token)).Content, StringComparison.Ordinal);
        await Assert.ThrowsAsync<UnwritableException>(() => second.SaveAsync(path, cancellationToken: Token));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task StoredAndPooledResultsAreComparedWithSeededRunsBesideFailedAndUnresolvedConfigurations(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var model = (await connection.LoadContentAsync(CompareModel, cancellationToken: Token)).RequireNoErrors();

        var comparison = await model.CompareResultsAsync(Results, new CompareOptions { Seed = 1, Runs = 4, ClockStep = 0.5, Observe = [new ObservablePair("total", "target.total")] }, Token);

        Assert.Equal("probe.xmi", comparison.Source);
        Assert.Equal(("sweep", "observed"), (comparison.Standing.Engine, comparison.Standing.Strength));
        Assert.False(comparison.Holds);
        var (pooled, failing, unresolved) = (comparison.Configurations[0], comparison.Configurations[1], comparison.Configurations[2]);
        Assert.Equal(("_c1", "holds", 10L, 1L, 4L, 4L, "random", (ulong?)1, 0.5), (pooled.Id, pooled.Status, pooled.StoredRuns, pooled.StoredSnapshots, pooled.Runs, pooled.Completed, pooled.Draws, pooled.Seed, pooled.ClockStep));
        Assert.Equal(4, pooled.Rows.Count);
        Assert.All(pooled.Rows, row => Assert.Empty(row.Error));
        var total = Assert.Single(pooled.Observables);
        Assert.True(total.Pooled && total.Compared);
        Assert.Equal(new ComparedDistribution(10, 0, 5, 0, 0, 0, 0), total.Tool);
        Assert.Equal(new ComparedDistribution(4, 4, 4, 4, 4, 4, 0), total.Run);
        Assert.Equal(new StatisticDifference(string.Empty, "-20.0%", string.Empty, string.Empty, string.Empty), total.Difference);
        Assert.Equal("Cfg::TotalCase", total.Declared!.Analysis);
        Assert.Equal(
            [new CaseStatistic("N", "return N", "10", "4", string.Empty), new CaseStatistic("Mean", "return Mean", "5.0", "4.0", "-20.0%"), new CaseStatistic("Deviation", "return Deviation", "1.0", "0.0", "-100.0%")],
            total.Declared.Rows);
        Assert.Contains(pooled.Lines, line => line.StartsWith("statistics of total by Cfg::TotalCase", StringComparison.Ordinal));
        Assert.Equal(("fails", 4L), (failing.Status, failing.Runs));
        Assert.True(failing.Completed < 4);
        Assert.Contains("division by zero", Assert.Single(failing.Errors), StringComparison.Ordinal);
        Assert.Contains(failing.Rows, row => row.Error.Contains("division by zero", StringComparison.Ordinal));
        var stored = Assert.Single(failing.Observables);
        Assert.False(stored.Pooled);
        Assert.Equal(new ComparedDistribution(1, 10, 10, 10, 10, 10, 0), stored.Tool);
        Assert.Equal(new StatisticDifference("+0.0%", "+0.0%", "+0.0%", "+0.0%", "+0.0%"), stored.Difference);
        Assert.Equal(("unresolved", (ulong?)null, 0), (unresolved.Status, unresolved.Seed, unresolved.Rows.Count));
        Assert.Contains("migrated without a behavior", unresolved.Unresolved, StringComparison.Ordinal);
        Assert.Empty(unresolved.Observables);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task TheSameSeedMakesTheSameComparisonAndFixedDrawsNeedNone(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var model = (await connection.LoadContentAsync(CompareModel, cancellationToken: Token)).RequireNoErrors();
        var seeded = new CompareOptions { Actions = ["Group 0"], Seed = 11, Runs = 6 };

        var first = await model.CompareResultsAsync(Results, seeded, Token);
        var again = await model.CompareResultsAsync(Results, seeded, Token);
        var unseeded = await model.CompareResultsAsync(Results, new CompareOptions { Actions = ["Group 0"] }, Token);
        var fixedDraws = await model.CompareResultsAsync(Results, new CompareOptions { Actions = ["Cfg::'Group 1'"], Draws = "average" }, Token);
        var unknown = await model.CompareResultsAsync(Results, new CompareOptions { Actions = ["Missing"], Seed = 1 }, Token);

        var (a, b) = (Assert.Single(first.Configurations), Assert.Single(again.Configurations));
        Assert.Equal((a.Completed, a.Errors, a.Observables, a.Lines), (b.Completed, b.Errors, b.Observables, b.Lines));
        Assert.Equal(a.Rows.Select(row => (row.Inputs, row.Outputs, row.Error)), b.Rows.Select(row => (row.Inputs, row.Outputs, row.Error)));
        Assert.Contains("draw at random", Assert.Single(unseeded.Configurations).Unresolved, StringComparison.Ordinal);
        Assert.All(fixedDraws.Configurations, configuration => Assert.Equal(("holds", "average", (ulong?)null), (configuration.Status, configuration.Draws, configuration.Seed)));
        Assert.Contains("no configuration is named Missing", Assert.Single(unknown.Configurations).Unresolved, StringComparison.Ordinal);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ResultsTheServiceCannotReadAndOptionsItRefusesAreStatuses(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var model = (await connection.LoadContentAsync(CompareModel, cancellationToken: Token)).RequireNoErrors();

        var malformed = await Assert.ThrowsAsync<ServiceException>(() => model.CompareResultsAsync("not json", cancellationToken: Token));
        var negative = await Assert.ThrowsAsync<ServiceException>(() => model.CompareResultsAsync(Results, new CompareOptions { Runs = -1 }, Token));
        var draws = await Assert.ThrowsAsync<ServiceException>(() => model.CompareResultsAsync(Results, new CompareOptions { Draws = "often" }, Token));
        var step = await Assert.ThrowsAsync<ServiceException>(() => model.CompareResultsAsync(Results, new CompareOptions { ClockStep = -1 }, Token));
        var stored = await Assert.ThrowsAsync<ServiceException>(() => model.CompareResultsAsync(Results, new CompareOptions { Engine = "no-such-engine" }, Token));
        var gone = await Assert.ThrowsAsync<ModelNotFoundException>(() => connection.ModelByHash("0000").CompareResultsAsync(Results, cancellationToken: Token));

        Assert.All([malformed, negative, draws, step, stored], refused => Assert.Equal(StatusCode.InvalidArgument, refused.StatusCode));
        Assert.Contains("results", malformed.ServiceMessage, StringComparison.Ordinal);
        Assert.Contains("runs", negative.ServiceMessage, StringComparison.Ordinal);
        Assert.Contains("clock_step", step.ServiceMessage, StringComparison.Ordinal);
        Assert.Equal("model not found: 0000", gone.ServiceMessage);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AServiceWithoutTheCapabilitiesIsAskedNothingItCannotDo(ClientWire wire)
    {
        var withheld = await services.ServiceAsync("connect", Withheld);
        using var counting = new CountingHandler();
        await using var connection = await Connection.OpenAsync(
            new ConnectionOptions { Address = withheld.Address, Protocol = wire.Protocol(), Encoding = wire.Encoding(), HttpMessageHandler = counting },
            Token);
        var model = (await connection.LoadAsync(Fixture("import.sysml"), cancellationToken: Token)).RequireNoErrors();

        var export = await Assert.ThrowsAsync<CapabilityException>(() => model.ExportGraphsAsync("V::car", Token));
        var import = await Assert.ThrowsAsync<CapabilityException>(() => model.ImportDataAsync([new DataImport("a.csv", "element\n")], cancellationToken: Token));
        var compare = await Assert.ThrowsAsync<CapabilityException>(() => model.CompareResultsAsync(Results, cancellationToken: Token));

        Assert.Equal([Capabilities.ExportGraphs, Capabilities.ImportData, Capabilities.CompareResults], new[] { export, import, compare }.Select(refused => refused.Capability));
        Assert.Equal(["GetServerInfo", "ParseFile"], counting.Methods);
    }

    private static string Fixture(string name) => Repository.PathOf("conformance", "fixtures", name);

    private static async Task<string> LoadOverAsync(ClientWire wire, string address, string fixture)
    {
        await using var connection = await Connection.OpenAsync(wire.To(address), Token);
        return (await connection.LoadAsync(Fixture(fixture), cancellationToken: Token)).RequireNoErrors().Hash;
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
