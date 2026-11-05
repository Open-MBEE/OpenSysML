using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading.Tasks;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// ExecuteAction and ExecuteState through the typed API, over every wire: the exact request each option
/// makes, every field of a run and of an exploration read, an action's trace and posted events, the schedule, trace, performer and value gates
/// checked in the service's order before anything is sent, a run and an exploration refusing each other's
/// schedule, a failed run kept in band with its clock and trace, and an answer of the wrong shape refused.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed class ModelExecutionTests
{
    private const string Hash = "h";

    public static TheoryData<ClientWire> Wires => ClientWires.All;

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ExecuteActionSendsEveryOptionAndReadsEveryFieldOfTheRun(ClientWire wire)
    {
        using var handler = Serving(wire).On("ExecuteAction", Wire.ExecuteActionRequest.Parser, _ => CompleteExecutionWire.ActionRun);
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash(Hash);
        var inputs = new Dictionary<string, Value> { ["b"] = new Value.IntegerValue(10), ["a"] = new Value.ComplexValue(1, 2) };
        var options = new RunOptions
        {
            Schedule = "seed:7",
            PerformerSymbolId = "Plant::tank",
            Trace = true,
            Events = ["Fill", "Signals::Drain"],
            Advance = 2.5,
            Seed = ulong.MaxValue,
            Draws = "max",
            ClockStep = 0.5,
        };

        var plain = await model.ExecuteActionAsync("Test::addFive", cancellationToken: TestContext.Current.CancellationToken);
        var run = await model.ExecuteActionAsync("Test::addFive", inputs, options, TestContext.Current.CancellationToken);

        var sent = handler.RequestsFor("ExecuteAction").Select(request => request.Message(Wire.ExecuteActionRequest.Parser)).ToList();
        Assert.Equal(new Wire.ExecuteActionRequest { ModelHash = Hash, ActionSymbolId = "Test::addFive" }, sent[0]);
        Assert.False(sent[0].HasAdvance);
        Assert.False(sent[0].HasSeed);
        Assert.Equal(
            new Wire.ExecuteActionRequest
            {
                ModelHash = Hash,
                ActionSymbolId = "Test::addFive",
                Inputs = { ["a"] = new Wire.Value { Complex = new Wire.Complex { Real = 1, Imaginary = 2 } }, ["b"] = new Wire.Value { IntValue = 10 } },
                Schedule = "seed:7",
                PerformerSymbolId = "Plant::tank",
                Advance = 2.5,
                Seed = ulong.MaxValue,
                Draws = "max",
                ClockStep = 0.5,
                Trace = true,
                Events = { "Fill", "Signals::Drain" },
            },
            sent[1]);
        Assert.Equal(plain, run);
        Assert.Equal(["mass", "result"], run.Outputs.Keys);
        Assert.Equal(new Value.IntegerValue(15), run.Outputs["result"]);
        Assert.Equal("choice-point", Assert.Single(run.Diagnostics).Code);
        Assert.Equal(8, run.FinalTime);
        Assert.Equal(new Value.IntegerValue(2), run.PerformerAttributes["this.level"]);
        Assert.Equal(1, run.OrderDependent["result"]);
        Assert.Equal(5, run.TraceDropped);
        Assert.Equal(["assign", "complete"], run.Trace.Select(record => record.Kind));
        var assign = run.Trace[0];
        Assert.Equal((4L, 7, "count", "fill", "assign count := 3"), (assign.Token, assign.Step, assign.Feature, assign.Machine, assign.Text));
        Assert.Equal(new Value.IntegerValue(3), assign.Value);
        Assert.Equal(new DocumentValue.NumberValue(new Value.RealValue(1.5)), assign.Time);
        var complete = run.Trace[1];
        Assert.Equal((0L, 8, string.Empty), (complete.Token, complete.Step, complete.Feature));
        Assert.Null(complete.Value);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ExploreSendsExploreUnlessABudgetIsNamedAndReadsEveryOutcome(ClientWire wire)
    {
        using var handler = Serving(wire)
            .On("ExecuteAction", Wire.ExecuteActionRequest.Parser, _ => CompleteExecutionWire.ActionExploration)
            .On("ExecuteState", Wire.ExecuteStateRequest.Parser, _ => CompleteExecutionWire.StateExploration);
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash(Hash);
        var token = TestContext.Current.CancellationToken;

        var bounded = await model.ExploreActionAsync("Test::race", options: new RunOptions { Schedule = "explore:runs=8,depth=4" }, cancellationToken: token);
        var whole = await model.ExploreActionAsync("Test::race", cancellationToken: token);
        var machine = await model.ExploreStateAsync("Test::Machine", ["go"], new RunOptions { PerformerSymbolId = "Test::box" }, token);

        Assert.Equal(["explore:runs=8,depth=4", "explore"], handler.RequestsFor("ExecuteAction").Select(request => request.Message(Wire.ExecuteActionRequest.Parser).Schedule));
        Assert.Equal(
            new Wire.ExecuteStateRequest { ModelHash = Hash, StateMachineSymbolId = "Test::Machine", Events = { "go" }, Schedule = "explore", PerformerSymbolId = "Test::box" },
            Assert.Single(handler.RequestsFor("ExecuteState")).Message(Wire.ExecuteStateRequest.Parser));
        Assert.Equal(bounded, whole);
        Assert.False(bounded.Complete);
        Assert.Equal((8, 8, 4, 1), (bounded.Runs, bounded.RunsBudget, bounded.DepthBudget, bounded.FailedLinearizations));
        Assert.Equal(["runs", "depth"], bounded.BudgetsHit);
        Assert.True(bounded.ProbabilitiesLowerBound);
        Assert.Equal(["Test::race::split"], bounded.Unenumerated);
        var outcome = bounded.Outcomes[0];
        Assert.Equal(new Value.IntegerValue(2), outcome.Outputs["winner"]);
        Assert.Equal(("done", 3, 0.25), (outcome.FinalState, outcome.Linearizations, outcome.Probability));
        Assert.Equal(["init", "done"], outcome.StatesVisited);
        Assert.False(outcome.Completed);
        Assert.Equal("action execution failed: deadlock", outcome.Error);
        Assert.Equal(["split: [left right] took right"], outcome.Witness);
        Assert.Equal("choice-point", Assert.Single(outcome.Diagnostics).Code);
        Assert.Equal(new ProbabilityRange(0.25, 0.75), outcome.ProbabilityRange);
        Assert.True(bounded.Outcomes[1].Completed);
        Assert.Null(bounded.Outcomes[1].ProbabilityRange);
        Assert.Equal(string.Empty, bounded.Outcomes[1].FinalState);
        Assert.True(machine.Complete);
        Assert.Equal((2, 1024, 64), (machine.Runs, machine.RunsBudget, machine.DepthBudget));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ExecuteStateSendsItsEventsAndTraceAndReadsTheTraceItAnswers(ClientWire wire)
    {
        using var handler = Serving(wire).On("ExecuteState", Wire.ExecuteStateRequest.Parser, _ => CompleteExecutionWire.StateRun);
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash(Hash);
        var options = new RunOptions { Schedule = "declared", Trace = true, Advance = 0, Seed = 3, Draws = "average", ClockStep = 1 };

        var run = await model.ExecuteStateAsync("Test::Machine", ["start", "stop"], options, TestContext.Current.CancellationToken);

        var sent = Assert.Single(handler.RequestsFor("ExecuteState")).Message(Wire.ExecuteStateRequest.Parser);
        Assert.Equal(
            new Wire.ExecuteStateRequest
            {
                ModelHash = Hash,
                StateMachineSymbolId = "Test::Machine",
                Events = { "start", "stop" },
                Schedule = "declared",
                Trace = true,
                Advance = 0,
                Seed = 3,
                Draws = "average",
                ClockStep = 1,
            },
            sent);
        Assert.True(sent.HasAdvance);
        Assert.Equal(["init", "Running", "done"], run.StatesVisited);
        Assert.Equal(new Value.RealValue(1.5), run.FinalContext["this.level"]);
        Assert.Equal("choice-point", Assert.Single(run.Diagnostics).Code);
        Assert.Equal((4.5, 2), (run.FinalTime, run.TraceDropped));
        Assert.Equal(2, run.OrderDependent["count"]);
        Assert.Equal(CompleteExecutionWire.Trace.Count, run.Trace.Count);
        var send = run.Trace[0];
        Assert.Equal(("send", "lp", "on", "off", "on", "Switch", "b", "send Switch to #3"), (send.Kind, send.Machine, send.State, send.From, send.To, send.Event, send.Taken, send.Text));
        Assert.Equal(new DocumentObject(2, "Garage::car", new DocumentValue.ElementValue("Garage::car", "PartUsage")), send.Subject);
        Assert.Equal(new DocumentObject(3, "#3", null), send.Target);
        Assert.Equal(["level = 2"], send.Payload);
        Assert.Equal(["a", "b"], send.Alternatives);
        var time = Assert.IsType<DocumentValue.QuantityValue>(send.Time);
        Assert.Equal("2.5 [s]", time.ToString());
        Assert.Equal("send Switch to #3", send.ToString());
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AFailedRunIsAnExecutionExceptionKeepingItsClockTraceAndDiagnostics(ClientWire wire)
    {
        using var handler = Serving(wire)
            .On("ExecuteAction", Wire.ExecuteActionRequest.Parser, request => request.Schedule.Length > 0
                ? new Wire.ExecuteActionResponse { Error = "action not found: Test::nope" }
                : CompleteExecutionWire.FailedAction)
            .On("ExecuteState", Wire.ExecuteStateRequest.Parser, _ => CompleteExecutionWire.FailedState);
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash(Hash);
        var token = TestContext.Current.CancellationToken;

        var action = await Assert.ThrowsAsync<ExecutionException>(() => model.ExecuteActionAsync("Timed::delayed", options: new RunOptions { Advance = 2, Trace = true }, cancellationToken: token));
        var explored = await Assert.ThrowsAsync<ExecutionException>(() => model.ExploreActionAsync("Test::nope", cancellationToken: token));
        var state = await Assert.ThrowsAsync<ExecutionException>(() => model.ExecuteStateAsync("Test::Machine", options: new RunOptions { Trace = true }, cancellationToken: token));

        Assert.Equal("action execution failed: still waiting at 5 s without completing", action.Message);
        Assert.Equal(Model.ExecuteActionMethod, action.Method);
        Assert.Equal((2.0, 1), (action.FinalTime, action.TraceDropped));
        Assert.Equal(new Value.IntegerValue(3), Assert.Single(action.Trace).Value);
        Assert.True(handler.RequestsFor("ExecuteAction")[0].Message(Wire.ExecuteActionRequest.Parser).Trace);
        Assert.Equal("choice-point", Assert.Single(action.Diagnostics).Code);
        Assert.Equal("action not found: Test::nope", explored.Message);
        Assert.Equal(Model.ExecuteStateMethod, state.Method);
        Assert.Equal((1.0, 3), (state.FinalTime, state.TraceDropped));
        Assert.Equal("send Switch to #3", Assert.Single(state.Trace).Text);
        Assert.IsAssignableFrom<ModelException>(state);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task RunGatesAreCheckedInTheServicesOrderBeforeAnythingIsSent(ClientWire wire)
    {
        var token = TestContext.Current.CancellationToken;
        var complex = new Dictionary<string, Value> { ["z"] = new Value.ComplexValue(1, 1) };
        var everything = new RunOptions { Schedule = "explore", PerformerSymbolId = "P::p" };

        Assert.Equal(Capabilities.Schedule, await RefusedAsync(wire, [Capabilities.Schedule], model => model.ExploreActionAsync("A", complex, everything, token)));
        Assert.Equal(Capabilities.ScheduleExplore, await RefusedAsync(wire, [Capabilities.ScheduleExplore], model => model.ExploreActionAsync("A", complex, everything, token)));
        Assert.Equal(Capabilities.Performer, await RefusedAsync(wire, [Capabilities.Performer], model => model.ExploreActionAsync("A", complex, everything, token)));
        Assert.Equal(Capabilities.ComplexValues, await RefusedAsync(wire, [Capabilities.ComplexValues], model => model.ExploreActionAsync("A", complex, everything, token)));
        Assert.Equal(Capabilities.Schedule, await RefusedAsync(wire, [Capabilities.Schedule], model => model.ExecuteActionAsync("A", options: new RunOptions { Schedule = "declared" }, cancellationToken: token)));
        Assert.Equal(
            Capabilities.ActionTrace,
            await RefusedAsync(wire, [Capabilities.ActionTrace, Capabilities.Performer], model => model.ExecuteActionAsync("A", options: new RunOptions { Trace = true, PerformerSymbolId = "P::p", Events = ["Go"] }, cancellationToken: token)));
        Assert.Equal(
            Capabilities.Performer,
            await RefusedAsync(wire, [Capabilities.Performer], model => model.ExecuteActionAsync("A", options: new RunOptions { PerformerSymbolId = "P::p", Events = ["Go"] }, cancellationToken: token)));
        Assert.Equal(
            Capabilities.StateTrace,
            await RefusedAsync(wire, [Capabilities.StateTrace, Capabilities.Performer], model => model.ExecuteStateAsync("S", options: new RunOptions { Trace = true, PerformerSymbolId = "P::p" }, cancellationToken: token)));
        Assert.Equal(Capabilities.Performer, await RefusedAsync(wire, [Capabilities.Performer], model => model.ExploreStateAsync("S", options: everything, cancellationToken: token)));
        Assert.Equal(Capabilities.ScheduleExplore, await RefusedAsync(wire, [Capabilities.ScheduleExplore], model => model.ExploreStateAsync("S", cancellationToken: token)));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ARunAndAnExplorationEachRefuseTheOthersOptionsBeforeSending(ClientWire wire)
    {
        using var handler = Serving(wire);
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash(Hash);
        var token = TestContext.Current.CancellationToken;

        var exploring = await Assert.ThrowsAsync<InvalidRequestException>(() => model.ExecuteActionAsync("A", options: new RunOptions { Schedule = "explore:runs=2" }, cancellationToken: token));
        var once = await Assert.ThrowsAsync<InvalidRequestException>(() => model.ExploreActionAsync("A", options: new RunOptions { Schedule = "declared" }, cancellationToken: token));
        var stateExploring = await Assert.ThrowsAsync<InvalidRequestException>(() => model.ExecuteStateAsync("S", options: new RunOptions { Schedule = "explore" }, cancellationToken: token));
        var stateOnce = await Assert.ThrowsAsync<InvalidRequestException>(() => model.ExploreStateAsync("S", options: new RunOptions { Schedule = "reverse" }, cancellationToken: token));
        var actionTrace = await Assert.ThrowsAsync<InvalidRequestException>(() => model.ExploreActionAsync("A", options: new RunOptions { Trace = true }, cancellationToken: token));
        var exploredEvents = await Assert.ThrowsAsync<InvalidRequestException>(() => model.ExploreActionAsync("A", options: new RunOptions { PerformerSymbolId = "P::p", Events = ["Go"] }, cancellationToken: token));
        var noPerformer = await Assert.ThrowsAsync<InvalidRequestException>(() => model.ExecuteActionAsync("A", options: new RunOptions { Events = ["Go"] }, cancellationToken: token));
        var stateEvents = await Assert.ThrowsAsync<InvalidRequestException>(() => model.ExecuteStateAsync("S", options: new RunOptions { Events = ["Go"] }, cancellationToken: token));
        var exploredStateEvents = await Assert.ThrowsAsync<InvalidRequestException>(() => model.ExploreStateAsync("S", options: new RunOptions { Events = ["Go"] }, cancellationToken: token));
        var exploredTrace = await Assert.ThrowsAsync<InvalidRequestException>(() => model.ExploreStateAsync("S", options: new RunOptions { Trace = true }, cancellationToken: token));
        var unset = await Assert.ThrowsAsync<InvalidRequestException>(() =>
            model.ExecuteActionAsync("A", new Dictionary<string, Value> { ["x"] = new Value.UnsetValue() }, cancellationToken: token));
        await Assert.ThrowsAsync<InvalidRequestException>(() => model.ExecuteActionAsync("A", options: new RunOptions { Schedule = string.Empty }, cancellationToken: token));
        await Assert.ThrowsAsync<InvalidRequestException>(() => model.ExecuteStateAsync("S", options: new RunOptions { PerformerSymbolId = string.Empty }, cancellationToken: token));
        await Assert.ThrowsAsync<InvalidRequestException>(() => model.ExploreActionAsync("A", options: new RunOptions { Draws = string.Empty }, cancellationToken: token));
        await Assert.ThrowsAsync<ArgumentException>(() => model.ExecuteActionAsync("A", new Dictionary<string, Value> { ["x"] = null! }, cancellationToken: token));
        await Assert.ThrowsAsync<ArgumentException>(() => model.ExecuteStateAsync("S", [null!], cancellationToken: token));
        await Assert.ThrowsAsync<ArgumentException>(() => model.ExecuteActionAsync("A", options: new RunOptions { PerformerSymbolId = "P::p", Events = [null!] }, cancellationToken: token));
        await Assert.ThrowsAsync<ArgumentNullException>(() => model.ExecuteActionAsync(null!, cancellationToken: token));
        await Assert.ThrowsAsync<ArgumentNullException>(() => model.ExploreStateAsync(null!, cancellationToken: token));

        Assert.Contains(nameof(Model.ExploreActionAsync), exploring.Message, StringComparison.Ordinal);
        Assert.Contains(nameof(Model.ExecuteActionAsync), once.Message, StringComparison.Ordinal);
        Assert.Contains(nameof(Model.ExploreStateAsync), stateExploring.Message, StringComparison.Ordinal);
        Assert.Contains(nameof(Model.ExecuteStateAsync), stateOnce.Message, StringComparison.Ordinal);
        Assert.Contains("exploration", actionTrace.Message, StringComparison.Ordinal);
        Assert.Contains("exploration", exploredEvents.Message, StringComparison.Ordinal);
        Assert.Contains(nameof(RunOptions.PerformerSymbolId), noPerformer.Message, StringComparison.Ordinal);
        Assert.Contains("events argument", stateEvents.Message, StringComparison.Ordinal);
        Assert.Contains("events argument", exploredStateEvents.Message, StringComparison.Ordinal);
        Assert.Contains("exploration", exploredTrace.Message, StringComparison.Ordinal);
        Assert.Contains("inputs[\"x\"]", unset.Message, StringComparison.Ordinal);
        Assert.Equal(["GetServerInfo"], handler.Requests.Select(request => request.Method));
        Assert.True(new RunOptions { Schedule = "explore:depth=2" }.Explores);
        Assert.False(new RunOptions { Schedule = "explorer" }.Explores);
        Assert.False(RunOptions.Default.Explores);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AnAnswerOfTheOtherShapeOrAMalformedValueIsRefused(ClientWire wire)
    {
        var badOutput = new Wire.ExecuteActionResponse { Outputs = { ["n"] = new Wire.Value { BigIntValue = "007" } } };
        var badTrace = new Wire.ExecuteStateResponse { Trace = { new Wire.DocumentEvent { Time = new Wire.DocumentValue { IntValue = 1, ElementType = "PartUsage" } } } };
        using var handler = Serving(wire)
            .On("ExecuteAction", Wire.ExecuteActionRequest.Parser, request => request.ActionSymbolId switch
            {
                "run-as-exploration" => CompleteExecutionWire.ActionExploration,
                "exploration-without-status" => new Wire.ExecuteActionResponse { Outcomes = { CompleteExecutionWire.Outcome } },
                "exploration-with-a-run" => new Wire.ExecuteActionResponse { Exploration = CompleteExecutionWire.Status, Outputs = { ["x"] = new Wire.Value { IntValue = 1 } } },
                "exploration-with-trace" => new Wire.ExecuteActionResponse { Exploration = CompleteExecutionWire.Status, Trace = { CompleteExecutionWire.Event } },
                "exploration-with-dropped-trace" => new Wire.ExecuteActionResponse { Exploration = CompleteExecutionWire.Status, TraceDropped = 1 },
                "bad-trace" or "failed-bad-trace" => new Wire.ExecuteActionResponse
                {
                    Error = request.ActionSymbolId == "failed-bad-trace" ? "failed" : string.Empty,
                    Trace = { badTrace.Trace },
                },
                "exploration-with-a-trace" => new Wire.ExecuteActionResponse { Exploration = CompleteExecutionWire.Status, Trace = { CompleteExecutionWire.Assign } },
                "bad-assigned-value" => new Wire.ExecuteActionResponse { Trace = { new Wire.DocumentEvent { Kind = "assign", Value = new Wire.Value { BigIntValue = "007" } } } },
                _ => badOutput,
            })
            .On("ExecuteState", Wire.ExecuteStateRequest.Parser, request => request.StateMachineSymbolId == "explored"
                ? new Wire.ExecuteStateResponse { Exploration = CompleteExecutionWire.Status, Trace = { CompleteExecutionWire.Event } }
                : badTrace);
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash(Hash);
        var token = TestContext.Current.CancellationToken;

        var asExploration = await Assert.ThrowsAsync<TransportException>(() => model.ExecuteActionAsync("run-as-exploration", cancellationToken: token));
        var noStatus = await Assert.ThrowsAsync<TransportException>(() => model.ExploreActionAsync("exploration-without-status", cancellationToken: token));
        var withRun = await Assert.ThrowsAsync<TransportException>(() => model.ExploreActionAsync("exploration-with-a-run", cancellationToken: token));
        var withTrace = await Assert.ThrowsAsync<TransportException>(() => model.ExploreStateAsync("explored", cancellationToken: token));
        var actionWithTrace = await Assert.ThrowsAsync<TransportException>(() => model.ExploreActionAsync("exploration-with-a-trace", cancellationToken: token));
        var assigned = await Assert.ThrowsAsync<TransportException>(() => model.ExecuteActionAsync("bad-assigned-value", cancellationToken: token));
        var output = await Assert.ThrowsAsync<TransportException>(() => model.ExecuteActionAsync("bad", cancellationToken: token));
        var time = await Assert.ThrowsAsync<TransportException>(() => model.ExecuteStateAsync("bad", cancellationToken: token));

        Assert.All([asExploration, noStatus, withRun, withTrace, actionWithTrace, assigned, output, time], error => Assert.Equal(StatusCode.Internal, error.StatusCode));
        Assert.Contains("trace", actionWithTrace.Message, StringComparison.Ordinal);
        Assert.Contains("trace[0].value", assigned.Message, StringComparison.Ordinal);
        Assert.Contains("one run", asExploration.Message, StringComparison.Ordinal);
        Assert.Contains("no exploration status", noStatus.Message, StringComparison.Ordinal);
        Assert.Contains("outputs", withRun.Message, StringComparison.Ordinal);
        Assert.Contains("trace", withTrace.Message, StringComparison.Ordinal);
        Assert.Contains("outputs[\"n\"]", output.Message, StringComparison.Ordinal);
        Assert.Contains("trace[0].time", time.Message, StringComparison.Ordinal);
        foreach (var id in new[] { "bad-trace", "failed-bad-trace" })
        {
            var malformed = await Assert.ThrowsAsync<TransportException>(() => model.ExecuteActionAsync(id, cancellationToken: token));
            Assert.Contains("trace[0].time", malformed.Message, StringComparison.Ordinal);
        }

        foreach (var id in new[] { "exploration-with-trace", "exploration-with-dropped-trace" })
        {
            var malformed = await Assert.ThrowsAsync<TransportException>(() => model.ExploreActionAsync(id, cancellationToken: token));
            Assert.Contains("trace", malformed.Message, StringComparison.Ordinal);
        }
    }

    [Fact]
    public void ARunsCollectionsAreCopies()
    {
        var outputs = new Dictionary<string, Value> { ["x"] = new Value.IntegerValue(1) };
        var events = new List<DocumentEvent>();
        var action = new ActionRun(outputs, [], 0, [], [], events, 0);
        var state = new StateRun([], outputs, [], 0, events, 0, []);

        outputs["y"] = new Value.IntegerValue(2);
        events.Add(new DocumentEvent("entry", null, null, "m", "s", string.Empty, string.Empty, null, string.Empty, [], [], string.Empty, "entry s", 0, 0, string.Empty, null));

        Assert.Single(action.Outputs);
        Assert.Single(state.FinalContext);
        Assert.Empty(state.Trace);
        Assert.Empty(action.Trace);
        Assert.Equal(0, action.TraceDropped);
        Assert.Throws<ArgumentException>(() => new Outcome([KeyValuePair.Create("x", (Value)null!)], string.Empty, [], string.Empty, 0, [], [], 0, null));
        Assert.Throws<ArgumentException>(() => new Exploration([null!], true, 0, [], 0, 0, false, 0, [], Standing.None));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task InvalidActionOrStateOptionsEventsSendNothing(ClientWire wire)
    {
        var options = new RunOptions { Events = ["go", "go"] };
        Assert.Null(RunOptions.Default.Events);
        using var handler = Serving(wire);
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash(Hash);
        var token = TestContext.Current.CancellationToken;

        var performer = await Assert.ThrowsAsync<InvalidRequestException>(() => model.ExecuteActionAsync("A", options: options, cancellationToken: token));
        var exploration = await Assert.ThrowsAsync<InvalidRequestException>(() =>
            model.ExploreActionAsync("A", options: new RunOptions { Events = ["go"], PerformerSymbolId = "P::p" }, cancellationToken: token));
        var state = await Assert.ThrowsAsync<InvalidRequestException>(() => model.ExecuteStateAsync("S", ["go"], options, token));

        Assert.Contains("PerformerSymbolId", performer.Message, StringComparison.Ordinal);
        Assert.Contains("exploration", exploration.Message, StringComparison.Ordinal);
        Assert.Contains("events argument", state.Message, StringComparison.Ordinal);
        Assert.Equal(["GetServerInfo"], handler.Requests.Select(request => request.Method));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task StateEventsUseTheExistingArgumentAndOptionsEventsAreRefused(ClientWire wire)
    {
        using var handler = Serving(wire).On("ExecuteState", Wire.ExecuteStateRequest.Parser, _ => new Wire.ExecuteStateResponse());
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash(Hash);
        var token = TestContext.Current.CancellationToken;

        await model.ExecuteStateAsync("S", ["go"], cancellationToken: token);
        await model.ExecuteStateAsync("S", ["go"], new RunOptions { Events = [] }, token);
        await Assert.ThrowsAsync<InvalidRequestException>(() => model.ExecuteStateAsync("S", options: new RunOptions { Events = ["go"] }, cancellationToken: token));

        var sent = handler.RequestsFor("ExecuteState").Select(request => request.Message(Wire.ExecuteStateRequest.Parser)).ToList();
        Assert.Equal(2, sent.Count);
        Assert.Equal(sent[0], sent[1]);
        Assert.Equal(["go"], sent[1].Events);
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
