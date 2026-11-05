using System;
using System.Collections.Concurrent;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Net.Http;
using System.Threading;
using System.Threading.Tasks;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// Running and checking behavior against a real sysml-grpc over every client wire, on the conformance
/// fixtures: actions with their inputs, schedule, performer and clock, explorations complete and cut short
/// by their budget, state machines with their trace, checks that witness a violation and a divergence, hold
/// exhaustively or stop at a bound, every failure in band or refused by status, and, against a service
/// withholding the capabilities these calls gate on, refusals made before anything is sent.
/// </summary>
[Trait(Category.Name, Category.Integration)]
public sealed class ServiceExecutionTests(ServiceFixture services) : IClassFixture<ServiceFixture>
{
    private const string Withheld = "schedule,schedule_explore,performer,state_trace,check";

    public static TheoryData<ClientWire> Wires => ClientWires.All;

    private static CancellationToken Token => TestContext.Current.CancellationToken;

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AnActionRunsWithItsInputsScheduleAndClock(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var behavior = await LoadAsync(connection, "behavior.sysml");
        var timed = await LoadAsync(connection, "timed.sysml");
        var tank = await LoadAsync(connection, "tank.sysml");

        var plain = await behavior.ExecuteActionAsync("Test::addFive", cancellationToken: Token);
        var bound = await behavior.ExecuteActionAsync("Test::addFive", new Dictionary<string, Value> { ["result"] = new Value.IntegerValue(10) }, cancellationToken: Token);
        var reverse = await behavior.ExecuteActionAsync("Test::race", cancellationToken: Token);
        var declared = await behavior.ExecuteActionAsync("Test::race", options: new RunOptions { Schedule = "declared" }, cancellationToken: Token);
        var advanced = await timed.ExecuteActionAsync("Timed::delayed", options: new RunOptions { Advance = 10 }, cancellationToken: Token);
        var stepped = await timed.ExecuteActionAsync("Timed::delayed", options: new RunOptions { ClockStep = 4 }, cancellationToken: Token);
        var performed = await tank.ExecuteActionAsync("Plant::Tank::fill", options: new RunOptions { PerformerSymbolId = "Plant::tank" }, cancellationToken: Token);

        Assert.Equal(new Value.IntegerValue(5), plain.Outputs["result"]);
        Assert.Equal(new Value.IntegerValue(15), bound.Outputs["result"]);
        Assert.Equal(new Value.IntegerValue(1), reverse.Outputs["winner"]);
        Assert.Equal(new Value.IntegerValue(2), declared.Outputs["winner"]);
        Assert.Equal((new Value.IntegerValue(1), 10.0), (advanced.Outputs["count"], advanced.FinalTime));
        Assert.Equal(8.0, stepped.FinalTime);
        Assert.IsType<Value.IntegerValue>(performed.PerformerAttributes["this.level"]);
        Assert.Contains(reverse.Diagnostics, diagnostic => diagnostic.Code == "choice-point");
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AFailedRunIsInBandAndAnUnreadableOptionIsRefused(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var behavior = await LoadAsync(connection, "behavior.sysml");
        var timed = await LoadAsync(connection, "timed.sysml");

        var noStart = await Assert.ThrowsAsync<ExecutionException>(() => behavior.ExecuteActionAsync("Test::noStart", cancellationToken: Token));
        var waiting = await Assert.ThrowsAsync<ExecutionException>(() => timed.ExecuteActionAsync("Timed::delayed", options: new RunOptions { Advance = 2 }, cancellationToken: Token));
        var noMachine = await Assert.ThrowsAsync<ExecutionException>(() => behavior.ExecuteStateAsync("Test::NoMachine", cancellationToken: Token));
        var draws = await Assert.ThrowsAsync<ServiceException>(() => timed.ExecuteActionAsync("Timed::delayed", options: new RunOptions { Draws = "sometimes" }, cancellationToken: Token));
        var seed = await Assert.ThrowsAsync<ServiceException>(() => behavior.ExecuteActionAsync("Test::race", options: new RunOptions { Schedule = "seed:abc" }, cancellationToken: Token));
        var budget = await Assert.ThrowsAsync<ServiceException>(() => behavior.ExploreActionAsync("Test::race", options: new RunOptions { Schedule = "explore:runs=none" }, cancellationToken: Token));
        var gone = await Assert.ThrowsAsync<ModelNotFoundException>(() => connection.ModelByHash("0000").ExecuteActionAsync("Test::addFive", cancellationToken: Token));

        Assert.NotEmpty(noStart.Message);
        Assert.Equal(Model.ExecuteActionMethod, noStart.Method);
        Assert.Contains("without completing", waiting.Message, StringComparison.Ordinal);
        Assert.Equal(2.0, waiting.FinalTime);
        Assert.Contains("Test::NoMachine", noMachine.Message, StringComparison.Ordinal);
        Assert.All([draws, seed, budget], refused => Assert.Equal(StatusCode.InvalidArgument, refused.StatusCode));
        Assert.Contains("draw policy", draws.ServiceMessage, StringComparison.Ordinal);
        Assert.Contains("seed:abc", seed.ServiceMessage, StringComparison.Ordinal);
        Assert.Equal(StatusCode.NotFound, gone.StatusCode);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AnExplorationReachesEveryOrderOrSaysWhichBudgetCutItShort(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var behavior = await LoadAsync(connection, "behavior.sysml");

        var whole = await behavior.ExploreActionAsync("Test::race", cancellationToken: Token);
        var cut = await behavior.ExploreActionAsync("Test::race", options: new RunOptions { Schedule = "explore:runs=1" }, cancellationToken: Token);
        var machine = await behavior.ExploreStateAsync("Test::Machine", cancellationToken: Token);

        Assert.True(whole.Complete);
        Assert.Empty(whole.BudgetsHit);
        Assert.Equal((1024, 64), (whole.RunsBudget, whole.DepthBudget));
        Assert.Equal([1L, 2L], whole.Outcomes.Select(outcome => ((Value.IntegerValue)outcome.Outputs["winner"]).Value).Order());
        Assert.All(whole.Outcomes, outcome => Assert.True(outcome.Completed && outcome.Linearizations > 0 && !outcome.Witness.IsEmpty));
        Assert.Equal(whole.Runs, whole.Outcomes.Sum(outcome => outcome.Linearizations));
        Assert.False(cut.Complete);
        Assert.Equal(["runs"], cut.BudgetsHit);
        Assert.Equal((1, 1), (cut.Runs, cut.RunsBudget));
        Assert.True(machine.Complete);
        Assert.Equal("done", Assert.Single(machine.Outcomes).FinalState);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AStateMachineReportsTheStatesItVisitedAndItsTrace(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var behavior = await LoadAsync(connection, "behavior.sysml");

        var run = await behavior.ExecuteStateAsync("Test::Machine", cancellationToken: Token);
        var traced = await behavior.ExecuteStateAsync("Test::Machine", options: new RunOptions { Trace = true }, cancellationToken: Token);

        Assert.Equal(["init", "Running", "done"], run.StatesVisited);
        Assert.Empty(run.Trace);
        Assert.Equal(run.StatesVisited, traced.StatesVisited);
        Assert.Equal(0, traced.TraceDropped);
        var transitions = traced.Trace.Where(record => record.Kind == "transition").ToList();
        Assert.Equal(["transition: init -> Running", "transition: Running -> done"], transitions.Select(record => record.Text));
        Assert.Equal([("init", "Running"), ("Running", "done")], transitions.Select(record => (record.From, record.To)));
        Assert.All(traced.Trace, record => Assert.True(record.Time is DocumentValue.NumberValue or DocumentValue.QuantityValue, record.ToString()));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ACheckWitnessesWhatBreaksAPropertyAndHoldsWhereEveryScheduleKeepsIt(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var tank = await LoadAsync(connection, "tank.sysml");
        var performed = new CheckOptions { PerformerSymbolId = "Plant::tank", Properties = ["Plant::Tank::low"], Diverge = ["this.level"], Witness = true };

        var fails = await tank.CheckActionAsync("Plant::Tank::fill", options: performed, cancellationToken: Token);
        var holds = await tank.CheckActionAsync(
            "Plant::Tank::fill", options: new CheckOptions { PerformerSymbolId = "Plant::tank", Properties = ["Plant::Tank::capped"], Diverge = ["this.capacity"] }, cancellationToken: Token);
        var bounded = await tank.CheckActionAsync("Plant::Tank::fill", options: new CheckOptions { PerformerSymbolId = "Plant::tank", Properties = ["Plant::Tank::low"], Depth = 2 }, cancellationToken: Token);

        Assert.Equal("fails", fails.Verdict);
        Assert.Equal((11, 10, 6), (fails.Search!.States, fails.Search.Moves, fails.Search.MaxDepth));
        Assert.Empty(fails.Search.BoundsHit);
        var violation = Assert.Single(fails.Violations);
        Assert.Equal(("property", "Plant::Tank::low", 3), (violation.Kind, violation.Property, violation.Depth));
        Assert.Equal("replay", violation.Witness!.Schedule);
        Assert.NotEmpty(violation.Witness.Choices);
        Assert.NotEmpty(violation.Witness.FileName);
        Assert.NotEmpty(violation.Witness.FileContent);
        var divergence = Assert.Single(fails.Divergences);
        Assert.Equal("this.level", divergence.Feature);
        Assert.Equal(["1", "2"], divergence.Values.Select(value => value.Value));
        Assert.All(divergence.Values, value => Assert.NotEmpty(value.Witness!.Schedule));
        Assert.Equal("check", fails.Standing.Engine);
        Assert.NotEmpty(fails.Standing.Strength);
        Assert.NotEmpty(fails.Lines);
        Assert.True(holds.Holds);
        Assert.NotEmpty(holds.Finals);
        Assert.Empty(holds.Violations);
        Assert.Empty(holds.Divergences);
        Assert.Equal("undecided", bounded.Verdict);
        Assert.NotEmpty(bounded.Search!.BoundsHit);
        Assert.Empty(bounded.Violations);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ACheckOfAMissingBehaviorIsInBandAndANegativeBoundIsRefused(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var tank = await LoadAsync(connection, "tank.sysml");

        var missing = await Assert.ThrowsAsync<CheckException>(() => tank.CheckActionAsync("Plant::Tank::drain", cancellationToken: Token));
        var property = await Assert.ThrowsAsync<CheckException>(() =>
            tank.CheckActionAsync("Plant::Tank::fill", options: new CheckOptions { Properties = ["Plant::Tank::nope"] }, cancellationToken: Token));
        var negative = await Assert.ThrowsAsync<ServiceException>(() => tank.CheckActionAsync("Plant::Tank::fill", options: new CheckOptions { Depth = -1 }, cancellationToken: Token));
        var neither = await Assert.ThrowsAsync<ServiceException>(() => tank.CheckActionAsync(string.Empty, cancellationToken: Token));
        var engine = await Assert.ThrowsAsync<ServiceException>(() => tank.CheckActionAsync("Plant::Tank::fill", options: new CheckOptions { Engine = "no-such-engine" }, cancellationToken: Token));

        Assert.Contains("Plant::Tank::drain", missing.Message, StringComparison.Ordinal);
        Assert.Equal("undecided", missing.Partial.Verdict);
        Assert.Equal(Model.CheckBehaviorMethod, missing.Method);
        Assert.Contains("Plant::Tank::nope", property.Message, StringComparison.Ordinal);
        Assert.All([negative, neither, engine], refused => Assert.Equal(StatusCode.InvalidArgument, refused.StatusCode));
        Assert.Contains("check_depth", negative.ServiceMessage, StringComparison.Ordinal);
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
            await Assert.ThrowsAsync<CapabilityException>(() => model.ExecuteActionAsync("Test::race", options: new RunOptions { Schedule = "declared" }, cancellationToken: Token)),
            await Assert.ThrowsAsync<CapabilityException>(() => model.ExploreActionAsync("Test::race", cancellationToken: Token)),
            await Assert.ThrowsAsync<CapabilityException>(() => model.ExecuteActionAsync("Test::race", options: new RunOptions { PerformerSymbolId = "Plant::tank" }, cancellationToken: Token)),
            await Assert.ThrowsAsync<CapabilityException>(() => model.ExecuteStateAsync("Test::Machine", options: new RunOptions { Trace = true }, cancellationToken: Token)),
            await Assert.ThrowsAsync<CapabilityException>(() => model.CheckStateAsync("Test::Machine", cancellationToken: Token)),
        };
        Assert.Equal(["GetServerInfo"], counting.Methods);

        // Without them, a run under the default schedule outside any object still runs.
        var behavior = await connection.LoadContentAsync(File.ReadAllText(Fixture("behavior.sysml")), cancellationToken: Token);
        var run = await behavior.ExecuteActionAsync("Test::race", cancellationToken: Token);

        Assert.Equal(
            [Capabilities.Schedule, Capabilities.Schedule, Capabilities.Performer, Capabilities.StateTrace, Capabilities.Check],
            refused.Select(error => error.Capability));
        Assert.Equal(new Value.IntegerValue(1), run.Outputs["winner"]);
        Assert.Equal(["GetServerInfo", "ParseFile", "ExecuteAction"], counting.Methods);
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
