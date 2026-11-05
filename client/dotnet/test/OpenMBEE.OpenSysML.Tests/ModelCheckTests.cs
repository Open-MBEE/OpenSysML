using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading.Tasks;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// CheckBehavior through the typed API, over every wire: the exact request each option makes, every
/// finding read with its witnesses, the check, performer, engine and value gates checked in the
/// service's order before anything is sent, a failing property answered as a verdict, and a check the
/// service refuses in band kept with what the answer reported.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed class ModelCheckTests
{
    private const string Hash = "h";

    public static TheoryData<ClientWire> Wires => ClientWires.All;

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task CheckActionSendsEveryOptionAndReadsEveryFinding(ClientWire wire)
    {
        using var handler = Serving(wire).On("CheckBehavior", Wire.CheckBehaviorRequest.Parser, _ => CompleteExecutionWire.Check);
        await using var connection = await FakeConnection.OpenAsync(handler);
        var options = new CheckOptions
        {
            Properties = ["Plant::Tank::low", "Plant::Tank::capped"],
            Diverge = ["this.level"],
            FreeInputs = ["x"],
            Assume = ["Plant::Tank::capped"],
            Witness = true,
            Depth = 6,
            States = 100,
            Unroll = 2,
            Timeout = 1.5,
            Engine = "smt",
            PerformerSymbolId = "Plant::tank",
        };

        var result = await connection.ModelByHash(Hash).CheckActionAsync(
            "Plant::Tank::fill", new Dictionary<string, Value> { ["x"] = new Value.IntegerValue(3) }, options, TestContext.Current.CancellationToken);

        Assert.Equal(
            new Wire.CheckBehaviorRequest
            {
                ModelHash = Hash,
                ActionSymbolId = "Plant::Tank::fill",
                Inputs = { ["x"] = new Wire.Value { IntValue = 3 } },
                PerformerSymbolId = "Plant::tank",
                CheckProperty = { "Plant::Tank::low", "Plant::Tank::capped" },
                CheckDiverge = { "this.level" },
                CheckInput = { "x" },
                CheckAssume = { "Plant::Tank::capped" },
                CheckWitness = true,
                CheckDepth = 6,
                CheckStates = 100,
                CheckUnroll = 2,
                CheckTimeout = 1.5,
                Engine = "smt",
            },
            Assert.Single(handler.RequestsFor("CheckBehavior")).Message(Wire.CheckBehaviorRequest.Parser));
        Assert.Equal("fails", result.Verdict);
        Assert.False(result.Holds);
        Assert.Equal(string.Join('\n', result.Lines), result.ToString());
        var search = result.Search!;
        Assert.Equal((11, 10, 6, 5.0, true, true, "bounded"), (search.States, search.Moves, search.MaxDepth, search.Horizon, search.HorizonSet, search.MassBounded, search.Status));
        Assert.Equal(["depth"], search.BoundsHit);
        Assert.Equal(["clock stopped at the horizon"], search.Notes);
        Assert.Equal(["horizon"], search.Scope);
        var violation = result.Violations[0];
        Assert.Equal(("property", "Plant::Tank::low", 3, 0.5), (violation.Kind, violation.Property, violation.Depth, violation.Probability));
        var witness = new CheckWitness("replay", ["0.5"], ["split: [a b] took b"], ["x = 3"], "fill-1.witness", "# witness\nsplit: b\n");
        Assert.Equal(witness, violation.Witness);
        Assert.Equal(("failure", "assignment failed"), (result.Violations[1].Kind, result.Violations[1].Error));
        Assert.Null(result.Violations[1].Witness);
        var divergence = Assert.Single(result.Divergences);
        Assert.Equal("this.level", divergence.Feature);
        Assert.Equal([new CheckDivergentValue("1", witness), new CheckDivergentValue("2", null)], divergence.Values);
        Assert.Equal(["this.capacity", "this.level"], result.Finals[0].Values.Keys);
        Assert.Equal(witness, result.Finals[0].Witness);
        Assert.Equal(("violated", "a bound was hit"), (result.Claim, result.Reason));
        Assert.Equal(["x"], result.FreeInputs);
        Assert.Equal(["Plant::Tank::capped"], result.Assumptions);
        Assert.Equal(witness, result.Witness);
        Assert.Equal("declared", result.Contrast!.Schedule);
        Assert.Equal(new Standing("check", "witnessed", [new Bound("depth", 64, true), new Bound("states", 100000, false)]), result.Standing);
        Assert.True(result.Standing.IsReported);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task CheckStateSendsTheMachineAndAHoldingCheckIsAVerdict(ClientWire wire)
    {
        using var handler = Serving(wire).On("CheckBehavior", Wire.CheckBehaviorRequest.Parser, _ => new Wire.CheckBehaviorResponse { Verdict = "holds" });
        await using var connection = await FakeConnection.OpenAsync(handler);

        var result = await connection.ModelByHash(Hash).CheckStateAsync("Test::Machine", cancellationToken: TestContext.Current.CancellationToken);

        Assert.Equal(
            new Wire.CheckBehaviorRequest { ModelHash = Hash, StateMachineSymbolId = "Test::Machine" },
            Assert.Single(handler.RequestsFor("CheckBehavior")).Message(Wire.CheckBehaviorRequest.Parser));
        Assert.True(result.Holds);
        Assert.Equal("holds", result.ToString());
        Assert.Null(result.Search);
        Assert.Null(result.Witness);
        Assert.False(result.Standing.IsReported);
        Assert.Empty(result.Standing.Bounds);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ACheckRefusedInBandIsACheckExceptionKeepingThePartialAnswer(ClientWire wire)
    {
        using var handler = Serving(wire).On("CheckBehavior", Wire.CheckBehaviorRequest.Parser, _ => CompleteExecutionWire.FailedCheck);
        await using var connection = await FakeConnection.OpenAsync(handler);

        var refused = await Assert.ThrowsAsync<CheckException>(() =>
            connection.ModelByHash(Hash).CheckActionAsync("Plant::Tank::drain", cancellationToken: TestContext.Current.CancellationToken));

        Assert.Equal("behavior not found: Plant::Tank::drain", refused.Message);
        Assert.Equal(Model.CheckBehaviorMethod, refused.Method);
        Assert.Equal("undecided", refused.Partial.Verdict);
        Assert.Empty(refused.Diagnostics);
        Assert.IsAssignableFrom<ModelException>(refused);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AnAnswerWithoutAVerdictOrFailureIsMalformed(ClientWire wire)
    {
        using var handler = Serving(wire).On("CheckBehavior", Wire.CheckBehaviorRequest.Parser, _ => new Wire.CheckBehaviorResponse());
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash(Hash);
        var token = TestContext.Current.CancellationToken;

        var action = await Assert.ThrowsAsync<TransportException>(() => model.CheckActionAsync("Test::race", cancellationToken: token));
        var state = await Assert.ThrowsAsync<TransportException>(() => model.CheckStateAsync("Test::Machine", cancellationToken: token));

        Assert.All(new[] { action, state }, malformed =>
        {
            Assert.Equal(StatusCode.Internal, malformed.StatusCode);
            Assert.Equal(Model.CheckBehaviorMethod, malformed.Method);
            Assert.Contains("neither a verdict nor a failure", malformed.Message, StringComparison.Ordinal);
        });
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ACheckRefusedWithoutAVerdictKeepsItsPartialAnswer(ClientWire wire)
    {
        var response = CompleteExecutionWire.FailedCheck.Clone();
        response.Verdict = string.Empty;
        response.Lines.Add("The search did not start.");
        using var handler = Serving(wire).On("CheckBehavior", Wire.CheckBehaviorRequest.Parser, _ => response);
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash(Hash);
        var token = TestContext.Current.CancellationToken;

        var action = await Assert.ThrowsAsync<CheckException>(() => model.CheckActionAsync("Plant::Tank::drain", cancellationToken: token));
        var state = await Assert.ThrowsAsync<CheckException>(() => model.CheckStateAsync("Test::NoMachine", cancellationToken: token));

        Assert.All(new[] { action, state }, refused =>
        {
            Assert.Equal(response.Error, refused.Message);
            Assert.Equal(Model.CheckBehaviorMethod, refused.Method);
            Assert.Empty(refused.Partial.Verdict);
            Assert.Equal(response.Lines, refused.Partial.Lines);
        });
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task CheckGatesAreCheckedInTheServicesOrderBeforeAnythingIsSent(ClientWire wire)
    {
        var token = TestContext.Current.CancellationToken;
        var set = new Dictionary<string, Value> { ["s"] = new Value.SetValue([new Value.IntegerValue(1)]) };
        var everything = new CheckOptions { PerformerSymbolId = "P::p", Engine = " explore " };

        Assert.Equal(Capabilities.Check, await RefusedAsync(wire, [Capabilities.Check, Capabilities.Performer], model => model.CheckActionAsync("A", set, everything, token)));
        Assert.Equal(Capabilities.Performer, await RefusedAsync(wire, [Capabilities.Performer, Capabilities.Engines], model => model.CheckActionAsync("A", set, everything, token)));
        Assert.Equal(Capabilities.Engines, await RefusedAsync(wire, [Capabilities.Engines, Capabilities.ScheduleExplore], model => model.CheckActionAsync("A", set, everything, token)));
        Assert.Equal(Capabilities.ScheduleExplore, await RefusedAsync(wire, [Capabilities.ScheduleExplore, Capabilities.SetValues], model => model.CheckActionAsync("A", set, everything, token)));
        Assert.Equal(Capabilities.SetValues, await RefusedAsync(wire, [Capabilities.SetValues], model => model.CheckActionAsync("A", set, everything, token)));
        Assert.Equal(Capabilities.Check, await RefusedAsync(wire, [Capabilities.Check], model => model.CheckStateAsync("S", cancellationToken: token)));
        Assert.Equal(Capabilities.Engines, await RefusedAsync(wire, [Capabilities.Engines], model => model.CheckStateAsync("S", new CheckOptions { Engine = "check" }, token)));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AnEmptyNameOrAnUnsendableInputIsRefusedBeforeSending(ClientWire wire)
    {
        using var handler = Serving(wire);
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash(Hash);
        var token = TestContext.Current.CancellationToken;

        await Assert.ThrowsAsync<InvalidRequestException>(() => model.CheckActionAsync("A", options: new CheckOptions { Engine = string.Empty }, cancellationToken: token));
        await Assert.ThrowsAsync<InvalidRequestException>(() => model.CheckStateAsync("S", new CheckOptions { PerformerSymbolId = string.Empty }, token));
        var undetermined = await Assert.ThrowsAsync<InvalidRequestException>(() =>
            model.CheckActionAsync("A", new Dictionary<string, Value> { ["u"] = new Value.UndeterminedValue("open", string.Empty, string.Empty) }, cancellationToken: token));
        await Assert.ThrowsAsync<ArgumentNullException>(() => model.CheckActionAsync(null!, cancellationToken: token));
        await Assert.ThrowsAsync<ArgumentNullException>(() => model.CheckStateAsync(null!, cancellationToken: token));

        Assert.Contains("inputs[\"u\"]", undetermined.Message, StringComparison.Ordinal);
        Assert.Equal(["GetServerInfo"], handler.Requests.Select(request => request.Method));
    }

    [Fact]
    public void CheckOptionsCopyTheirListsAndRefuseANullName()
    {
        var properties = new List<string> { "P::a" };
        var options = new CheckOptions { Properties = properties };

        properties.Add("P::b");

        Assert.Equal(["P::a"], options.Properties);
        Assert.Empty(CheckOptions.Default.Diverge);
        Assert.Throws<ArgumentException>(() => new CheckOptions { Diverge = [null!] });
        Assert.Throws<ArgumentNullException>(() => new CheckOptions { Assume = null! });
        Assert.Throws<ArgumentException>(() => new CheckFinal("done", [KeyValuePair.Create("x", (string)null!)], null));
        Assert.Throws<ArgumentNullException>(() => new CheckException("m", Model.CheckBehaviorMethod, null!));
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
