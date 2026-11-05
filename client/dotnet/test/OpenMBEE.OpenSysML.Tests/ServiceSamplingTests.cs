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
/// Running behaviors repeatedly, self-checking and recording runs against a real sysml-grpc over every client wire, on
/// the conformance fixtures: seeded runs that replay alike, a Monte Carlo case concluded over its sample, refusals of
/// requests no run follows from, a self-check that holds and one naming an unknown rule package, recordings of a run, a
/// sweep and a sample answered as a new model the original is untouched by, and, against a service withholding the
/// capabilities these calls gate on, refusals made before anything is sent.
/// </summary>
[Trait(Category.Name, Category.Integration)]
public sealed class ServiceSamplingTests(ServiceFixture services) : IClassFixture<ServiceFixture>
{
    private const string Withheld = "runs,self_check,record_run";

    public static TheoryData<ClientWire> Wires => ClientWires.All;

    private static CancellationToken Token => TestContext.Current.CancellationToken;

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task SeededActionRunsReplayAlikeAndTableEachObservablesDistribution(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var model = await LoadAsync(connection, "monte_carlo.sysml");
        var seeded = new ActionRunsOptions { Seed = 7, Observe = ["t"] };

        var first = await model.RunActionRunsAsync("MC::Settle", 4, options: seeded, cancellationToken: Token);
        var again = await model.RunActionRunsAsync("MC::Settle", 4, options: seeded, cancellationToken: Token);
        var other = await model.RunActionRunsAsync("MC::Settle", 4, options: new ActionRunsOptions { Seed = 8, Observe = ["t"] }, cancellationToken: Token);
        var fixedDraws = await model.RunActionRunsAsync("MC::Settle", 2, options: new ActionRunsOptions { Draws = "min" }, cancellationToken: Token);

        Assert.Equal(4, first.Rows.Count);
        Assert.Equal([1L, 2L, 3L, 4L], first.Rows.Select(row => ((Value.IntegerValue)Assert.Single(row.Inputs).Value).Value));
        Assert.Empty(first.Failures);
        Assert.Equal(["t"], first.Observables);
        var t = first.Distribution("t")!;
        Assert.Equal((4L, true), (t.Count, t.Numeric));
        Assert.All(first.Rows, row => Assert.True(row.ElapsedMicroseconds >= 0));
        Assert.InRange(((Value.RealValue)t.Min!).Value, 1.0, 5.0);
        Assert.InRange(((Value.RealValue)t.Max!).Value, ((Value.RealValue)t.Min!).Value, 5.0);
        Assert.Equal([50.0, 90.0], t.Percentiles.Select(percentile => percentile.Percentile));
        Assert.Equal(4, t.Histogram.Sum(bin => bin.Count));
        Assert.NotEmpty(first.Lines);
        Assert.Equal(7UL, first.Seed);
        Assert.Equal("sweep", first.Standing.Engine);
        Assert.Equal(first.Rows.Select(row => row.Outputs), again.Rows.Select(row => row.Outputs));
        Assert.Equal(t, again.Distribution("t"));
        Assert.NotEqual(first.Rows.Select(row => row.Outputs), other.Rows.Select(row => row.Outputs));
        Assert.Null(fixedDraws.Seed);
        Assert.Contains("clock", fixedDraws.Observables);
        Assert.Empty(first.Outputs);
        Assert.Empty(first.Unconcluded);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AMonteCarloCaseIsConcludedOverItsSampleAndDrawsAsTheActionItPerforms(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var model = await LoadAsync(connection, "monte_carlo.sysml");

        var concluded = await model.RunAnalysisRunsAsync("MC::Mc", 3, new AnalysisRunsOptions { SubjectSymbolId = "MC::probe", Seed = 7 }, Token);
        var action = await model.RunActionRunsAsync("MC::Settle", 3, options: new ActionRunsOptions { Seed = 7, Observe = ["t"] }, cancellationToken: Token);
        var unbound = await model.RunAnalysisRunsAsync("MC::Mc", 2, new AnalysisRunsOptions { Seed = 1 }, Token);

        Assert.Equal(["observed"], concluded.Observables);
        Assert.Equal(3, concluded.Rows.Count);
        Assert.Contains(concluded.Outputs, output => output.Name == "Mean");
        Assert.Equal(new Value.IntegerValue(3), concluded.Outputs.Single(output => output.Name == "runs").Value);
        Assert.Empty(concluded.Unconcluded);
        Assert.Equal("MC::probe", concluded.Instances[0].TypeSymbolId);
        Assert.Equal(
            action.Rows.Select(row => ((Value.SequenceValue)Assert.Single(row.Outputs).Value).Elements.Single()),
            concluded.Rows.Select(row => Assert.Single(row.Outputs).Value));
        Assert.Equal(2, unbound.Failures.Count);
        Assert.All(unbound.Rows, row => Assert.Equal(FailureReason.Evaluation, row.FailureReason));
        Assert.Equal("no run completed, so the case is not concluded", unbound.Unconcluded);
        Assert.Empty(unbound.Outputs);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task RunsNoRunFollowsFromAreRefusedOrRaisedWithTheTableMade(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var model = await LoadAsync(connection, "monte_carlo.sysml");

        var unseeded = await Assert.ThrowsAsync<ServiceException>(() => model.RunActionRunsAsync("MC::Settle", 2, cancellationToken: Token));
        var none = await Assert.ThrowsAsync<ServiceException>(() => model.RunActionRunsAsync("MC::Settle", 0, options: new ActionRunsOptions { Seed = 1 }, cancellationToken: Token));
        var draws = await Assert.ThrowsAsync<ServiceException>(() => model.RunActionRunsAsync("MC::Settle", 1, options: new ActionRunsOptions { Draws = "median" }, cancellationToken: Token));
        var twice = await Assert.ThrowsAsync<ServiceException>(() =>
            model.RunActionRunsAsync("MC::Settle", 1, options: new ActionRunsOptions { Seed = 1, Observe = ["t", "t"] }, cancellationToken: Token));
        var unobserved = await Assert.ThrowsAsync<RunsException>(() =>
            model.RunActionRunsAsync("MC::Settle", 2, options: new ActionRunsOptions { Seed = 7, Observe = ["nope"] }, cancellationToken: Token));
        var missing = await Assert.ThrowsAsync<RunsException>(() => model.RunActionRunsAsync("MC::Nope", 2, options: new ActionRunsOptions { Seed = 7 }, cancellationToken: Token));
        var notMonteCarlo = await Assert.ThrowsAsync<RunsException>(() => model.RunAnalysisRunsAsync("MC::Cost", 2, new AnalysisRunsOptions { Seed = 1 }, Token));

        Assert.All([unseeded, none, draws, twice], refused => Assert.Equal(StatusCode.InvalidArgument, refused.StatusCode));
        Assert.Contains("seed", unseeded.Message, StringComparison.Ordinal);
        Assert.Contains("positive", none.Message, StringComparison.Ordinal);
        Assert.Equal(2, unobserved.Partial.Rows.Count);
        Assert.Equal(7UL, unobserved.Partial.Seed);
        Assert.Equal("sweep", unobserved.Standing.Engine);
        Assert.Contains("nope", unobserved.Message, StringComparison.Ordinal);
        Assert.Empty(missing.Partial.Rows);
        Assert.Contains("MC::Nope", missing.Message, StringComparison.Ordinal);
        Assert.Equal(FailureReason.Evaluation, notMonteCarlo.Reason);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ASelfCheckHoldsOverAWellFormedModelAndAnUnknownRulePackageIsInBand(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var model = await LoadAsync(connection, "monte_carlo.sysml");

        var check = await model.SelfCheckAsync(cancellationToken: Token);
        var unknown = await Assert.ThrowsAsync<ModelException>(() => model.SelfCheckAsync(["Nope"], Token));

        Assert.True(check.Holds);
        Assert.Equal(SelfCheck.HoldsStatus, check.Status);
        Assert.True(check.Elements > 0);
        Assert.Equal(check.Checks, check.Applications.Sum(application => application.Count));
        Assert.Equal(check.Applications.Select(application => application.Constraint).Order(StringComparer.Ordinal), check.Applications.Select(application => application.Constraint));
        Assert.Equal((0L, 0L), (check.Violations, check.EvaluationErrors));
        Assert.StartsWith("Self-model check:", check.Summary, StringComparison.Ordinal);
        Assert.Equal(Model.SelfCheckMethod, unknown.Method);
        Assert.Contains("Nope", unknown.Message, StringComparison.Ordinal);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ARecordedRunIsANewModelHoldingTheRecordsAndTheOriginalIsUntouched(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var model = await LoadAsync(connection, "monte_carlo.sysml");
        var sweep = await LoadAsync(connection, "sweep.sysml");

        var once = await model.RecordRunAsync("MC::Cost", new RecordOptions { SubjectSymbolId = "MC::p" }, Token);
        var swept = await sweep.RecordRunAsync(
            "Sw::CostAnalysis",
            new RecordOptions { SubjectSymbolId = "Sw::barge", Ranges = [new SweepRange("limit", new Value.RealValue(0), new Value.RealValue(40), new Value.RealValue(20))] },
            Token);
        var sampled = await model.RecordRunAsync("MC::Mc", new RecordOptions { SubjectSymbolId = "MC::probe", Runs = 2, Seed = 3 }, Token);
        var merged = await sweep.RecordRunAsync("Sw::CostAnalysis", new RecordOptions { SubjectSymbolId = "Sw::barge", RecordInto = "Sw" }, Token);

        Assert.Equal(new CalcOutput("total", new Value.SequenceValue([new Value.RealValue(6)])), Assert.Single(once.Run!.Outputs));
        Assert.Equal(("Records", "Records::CostRun"), (once.Package, once.Definition));
        Assert.Equal(["Records::Cost_run1"], once.Records);
        Assert.Contains("part Cost_run1 : CostRun", once.Source, StringComparison.Ordinal);
        Assert.Empty(once.Content);
        Assert.Equal(new EditedDocument("Records.sysml", once.Source), Assert.Single(once.Documents));
        Assert.NotEqual(model.Hash, once.Model.Hash);
        Assert.Same(connection, once.Model.Connection);
        Assert.Equal("Records::Cost_run1", (await once.Model.SymbolAsync("Records::Cost_run1", Token)).Id);
        await Assert.ThrowsAsync<SymbolNotFoundException>(() => model.SymbolAsync("Records::Cost_run1", Token));
        Assert.Equal(3, swept.Sweep!.Rows.Count);
        Assert.Equal(["Records::CostAnalysis_run1", "Records::CostAnalysis_run2", "Records::CostAnalysis_run3"], swept.Records);
        Assert.Equal(0, swept.Skipped);
        Assert.Equal(["Records::Mc_run1", "Records::Mc_run2", "Records::Mc_run3"], sampled.Records);
        Assert.Equal(3UL, sampled.Sample!.Seed);
        Assert.Equal("Sw", merged.Package);
        Assert.Contains("part barge : Ship;", merged.Content, StringComparison.Ordinal);
        Assert.Contains("CostAnalysis_run1", merged.Content, StringComparison.Ordinal);
        Assert.Equal(merged.Content, Assert.Single(merged.Documents).Content);
        Assert.True((await merged.Model.SelfCheckAsync(cancellationToken: Token)).Elements > (await sweep.SelfCheckAsync(cancellationToken: Token)).Elements);
        Assert.False(model.HasErrors);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ARecordingNotMadeIsInBandAndARequestNoRunFollowsFromIsRefused(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var model = await LoadAsync(connection, "monte_carlo.sysml");
        var sweep = await LoadAsync(connection, "sweep.sysml");

        var missing = await Assert.ThrowsAsync<RecordException>(() => model.RecordRunAsync("MC::Nope", cancellationToken: Token));
        var wrong = await Assert.ThrowsAsync<RecordException>(() => model.RecordRunAsync("MC::P", cancellationToken: Token));
        var unbound = await Assert.ThrowsAsync<RecordException>(() => model.RecordRunAsync("MC::Cost", cancellationToken: Token));
        var placed = await Assert.ThrowsAsync<RecordException>(() => sweep.RecordRunAsync("Sw::CostAnalysis", new RecordOptions { SubjectSymbolId = "Sw::barge", RecordInto = "Sw::barge" }, Token));
        var calc = await Assert.ThrowsAsync<RecordException>(() =>
            sweep.RecordRunAsync("Sw::Sum", new RecordOptions { Ranges = [new SweepRange("b", new Value.RealValue(0), new Value.RealValue(1))] }, Token));
        var unseeded = await Assert.ThrowsAsync<ServiceException>(() => model.RecordRunAsync("MC::Mc", new RecordOptions { Runs = 2 }, Token));
        var both = await Assert.ThrowsAsync<ServiceException>(() =>
            sweep.RecordRunAsync("Sw::CostAnalysis", new RecordOptions { Runs = 2, Seed = 1, Ranges = [new SweepRange("limit", new Value.RealValue(0), new Value.RealValue(1))] }, Token));

        Assert.Equal((false, null), (missing.RunFailed, missing.Run));
        Assert.Contains("MC::Nope", missing.Message, StringComparison.Ordinal);
        Assert.True(wrong.RunFailed);
        Assert.Equal(FailureReason.WrongKind, wrong.Reason);
        Assert.NotNull(wrong.Run);
        Assert.True(unbound.RunFailed);
        Assert.Equal(FailureReason.Evaluation, unbound.Reason);
        Assert.Equal("run", unbound.Standing.Engine);
        Assert.False(placed.RunFailed);
        Assert.NotEmpty(placed.Run!.Outputs);
        Assert.Contains("not a package", placed.Message, StringComparison.Ordinal);
        Assert.True(calc.RunFailed);
        Assert.Equal(FailureReason.WrongKind, calc.Reason);
        Assert.NotNull(calc.Sweep);
        Assert.All([unseeded, both], refused => Assert.Equal(StatusCode.InvalidArgument, refused.StatusCode));
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
        var model = connection.ModelByHash("h");

        var refused = new[]
        {
            await Assert.ThrowsAsync<CapabilityException>(() => model.RunActionRunsAsync("MC::Settle", 1, cancellationToken: Token)),
            await Assert.ThrowsAsync<CapabilityException>(() => model.RunAnalysisRunsAsync("MC::Mc", 1, cancellationToken: Token)),
            await Assert.ThrowsAsync<CapabilityException>(() => model.SelfCheckAsync(cancellationToken: Token)),
            await Assert.ThrowsAsync<CapabilityException>(() => model.RecordRunAsync("MC::Cost", cancellationToken: Token)),
        };

        Assert.Equal([Capabilities.Runs, Capabilities.Runs, Capabilities.SelfCheck, Capabilities.RecordRun], refused.Select(error => error.Capability));
        Assert.Equal(["GetServerInfo"], counting.Methods);
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
