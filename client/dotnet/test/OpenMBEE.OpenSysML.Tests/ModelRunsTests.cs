using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading.Tasks;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// RunRuns, SelfCheck and RecordRun through the typed API, over every wire: the exact request each option makes, every
/// field of each answer read, a failed run kept as a row, runs and recordings the service could not make raised with
/// what they kept, a recording answered as a new model leaving the old one as it was, a malformed answer refused, and
/// the gates checked in the service's order before anything is sent.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed class ModelRunsTests
{
    private const string Hash = "h";

    public static TheoryData<ClientWire> Wires => ClientWires.All;

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ActionRunsSendEveryOptionAndReadEveryFieldOfTheTable(ClientWire wire)
    {
        using var handler = Serving(wire).On("RunRuns", Wire.RunRunsRequest.Parser, _ => CompleteSamplingWire.ActionRuns);
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash(Hash);
        var token = TestContext.Current.CancellationToken;
        var options = new ActionRunsOptions
        {
            PerformerSymbolId = "MC::probe",
            Observe = ["t", "mode", "clock"],
            Seed = ulong.MaxValue,
            Draws = "min",
            ClockStep = 0.25,
            Engine = "sweep",
        };

        var runs = await model.RunActionRunsAsync("MC::Settle", 3, new Dictionary<string, Value> { ["x"] = new Value.IntegerValue(2) }, options, token);
        await model.RunActionRunsAsync("MC::Settle", -1, cancellationToken: token);

        Assert.Equal(
            [
                new Wire.RunRunsRequest
                {
                    ModelHash = Hash,
                    ActionSymbolId = "MC::Settle",
                    Inputs = { ["x"] = new Wire.Value { IntValue = 2 } },
                    PerformerSymbolId = "MC::probe",
                    Runs = 3,
                    Seed = ulong.MaxValue,
                    Draws = "min",
                    ClockStep = 0.25,
                    Observe = { "t", "mode", "clock" },
                    Engine = "sweep",
                },
                new Wire.RunRunsRequest { ModelHash = Hash, ActionSymbolId = "MC::Settle", Runs = -1 },
            ],
            handler.RequestsFor("RunRuns").Select(sent => sent.Message(Wire.RunRunsRequest.Parser)));
        Assert.False(handler.RequestsFor("RunRuns")[1].Message(Wire.RunRunsRequest.Parser).HasSeed);
        Assert.Equal(3, runs.Rows.Count);
        Assert.Equal([new CalcOutput("run", new Value.IntegerValue(1))], runs.Rows[0].Inputs);
        Assert.Equal(new CalcOutput("t", new Value.RealValue(1.25)), runs.Rows[0].Outputs[0]);
        Assert.Equal(812, runs.Rows[0].ElapsedMicroseconds);
        Assert.Equal([runs.Rows[1]], runs.Failures);
        Assert.Equal(("action execution failed: deadlock", FailureReason.Evaluation), (runs.Rows[1].Error, runs.Rows[1].FailureReason));
        Assert.Equal(new CalcOutput("t", new Value.RealValue(3.75)), Assert.Single(runs.Rows[1].Outputs));
        Assert.Equal(["t", "mode"], runs.Observables);
        Assert.Equal(["t: 3 run(s), min 1.25, mean 2.25, max 3.75", "  1.25..2.5 ####  2"], runs.Lines);
        var t = runs.Distribution("t")!;
        Assert.Equal(("t", "s", 3L, true), (t.Name, t.Unit, t.Count, t.Numeric));
        Assert.Equal(new Value.RealValue(1.25), t.Min);
        Assert.Equal(new Value.RealValue(3.75), t.Max);
        Assert.Equal(new Value.RationalValue(new Rational(7, 3)), t.Mean);
        Assert.Equal(1.3227, t.Deviation);
        Assert.Equal([new RunPercentile(50, new Value.RealValue(1.75)), new RunPercentile(90, new Value.RealValue(3.75))], t.Percentiles);
        Assert.Equal(new RunHistogramBin(new Value.RealValue(1.25), new Value.RealValue(2.5), 2), t.Histogram[0]);
        Assert.Empty(t.Values);
        var mode = runs.Distribution("mode")!;
        Assert.False(mode.Numeric);
        Assert.Equal((null, null, null), (mode.Min, mode.Max, mode.Mean));
        Assert.Equal([new RunCountedValue(new Value.TextValue("Demo::idle"), 2), new RunCountedValue(new Value.TextValue("Demo::busy"), 1)], mode.Values);
        Assert.Null(runs.Distribution("clock"));
        Assert.Equal(9, runs.Resolve((Value.InstanceRefValue)runs.Rows[0].Outputs[1].Value)!.Id);
        Assert.Null(runs.FindInstance(404));
        Assert.Equal(7UL, runs.Seed);
        Assert.Empty(runs.Outputs);
        Assert.Empty(runs.Verdicts);
        Assert.Empty(runs.Unconcluded);
        Assert.Single(runs.Diagnostics);
        Assert.Equal(new Standing("sweep", "observed", [new Bound("solver", 5000, true), new Bound("depth", 64, false)]), runs.Standing);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task MonteCarloRunsSendTheCasesOptionsAndReadItsConclusion(ClientWire wire)
    {
        var answers = new Queue<Wire.RunRunsResponse>([CompleteSamplingWire.CaseRuns, CompleteSamplingWire.Unconcluded]);
        using var handler = Serving(wire).On("RunRuns", Wire.RunRunsRequest.Parser, _ => answers.Dequeue());
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash(Hash);
        var token = TestContext.Current.CancellationToken;
        var options = new AnalysisRunsOptions
        {
            SubjectSymbolId = "MC::probe",
            Arguments = [new Value.RealValue(10)],
            NamedArguments = new Dictionary<string, Value> { ["limit"] = new Value.RealValue(50) },
            Seed = 0,
            Draws = "average",
            ClockStep = 2,
            Engine = "sweep",
        };

        var concluded = await model.RunAnalysisRunsAsync("MC::Mc", 1, options, token);
        var unconcluded = await model.RunAnalysisRunsAsync("MC::Mc", 1, cancellationToken: token);

        Assert.Equal(
            [
                new Wire.RunRunsRequest
                {
                    ModelHash = Hash,
                    AnalysisSymbolId = "MC::Mc",
                    SubjectSymbolId = "MC::probe",
                    Arguments = { new Wire.Value { RealValue = 10 } },
                    NamedArguments = { ["limit"] = new Wire.Value { RealValue = 50 } },
                    Runs = 1,
                    Seed = 0,
                    Draws = "average",
                    ClockStep = 2,
                    Engine = "sweep",
                },
                new Wire.RunRunsRequest { ModelHash = Hash, AnalysisSymbolId = "MC::Mc", Runs = 1 },
            ],
            handler.RequestsFor("RunRuns").Select(sent => sent.Message(Wire.RunRunsRequest.Parser)));
        Assert.True(handler.RequestsFor("RunRuns")[0].Message(Wire.RunRunsRequest.Parser).HasSeed);
        Assert.Equal(["runs", "mean"], concluded.Outputs.Select(output => output.Name));
        Assert.Equal([true, false], concluded.Verdicts.Select(verdict => verdict.Decided));
        Assert.True(Assert.Single(concluded.Rows).Verdicts.Single().Violated);
        Assert.True(concluded.Rows[0].Evaluations.Single().Selected);
        Assert.Equal(ulong.MaxValue, concluded.Seed);
        Assert.Empty(concluded.Unconcluded);
        Assert.Equal("no run completed, so the case is not concluded", unconcluded.Unconcluded);
        Assert.Null(unconcluded.Seed);
        Assert.Equal(new Standing("sweep", string.Empty, []), unconcluded.Standing);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task RunsTheServiceCouldNotTableAreARunsExceptionKeepingTheTableAndAMalformedAnswerIsRefused(ClientWire wire)
    {
        using var handler = Serving(wire).On("RunRuns", Wire.RunRunsRequest.Parser, request => request.ActionSymbolId switch
        {
            "unobserved" => CompleteSamplingWire.Unobserved,
            "missing" => new Wire.RunRunsResponse { Error = "action not found: MC::Nope" },
            "valueless" => new Wire.RunRunsResponse
            {
                Rows = { new Wire.SweepRow() },
                Distributions = { new Wire.RunDistribution { Name = "t", Percentiles = { new Wire.RunPercentile { Percentile = 50 } } } },
            },
            "binless" => new Wire.RunRunsResponse
            {
                Rows = { new Wire.SweepRow() },
                Distributions = { new Wire.RunDistribution { Name = "t", Histogram = { new Wire.RunHistogramBin { Low = new Wire.Value { IntValue = 1 } } } } },
            },
            "uncounted" => new Wire.RunRunsResponse { Rows = { new Wire.SweepRow() }, Distributions = { new Wire.RunDistribution { Name = "m", Values = { new Wire.RunCountedValue { Count = 1 } } } } },
            _ => new Wire.RunRunsResponse { Observables = { "t" } },
        });
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash(Hash);
        var token = TestContext.Current.CancellationToken;

        var unobserved = await Assert.ThrowsAsync<RunsException>(() => model.RunActionRunsAsync("unobserved", 1, cancellationToken: token));
        var missing = await Assert.ThrowsAsync<RunsException>(() => model.RunActionRunsAsync("missing", 1, cancellationToken: token));
        var neither = await Assert.ThrowsAsync<TransportException>(() => model.RunActionRunsAsync("neither", 1, cancellationToken: token));
        var valueless = await Assert.ThrowsAsync<TransportException>(() => model.RunActionRunsAsync("valueless", 1, cancellationToken: token));
        var binless = await Assert.ThrowsAsync<TransportException>(() => model.RunActionRunsAsync("binless", 1, cancellationToken: token));
        var uncounted = await Assert.ThrowsAsync<TransportException>(() => model.RunActionRunsAsync("uncounted", 1, cancellationToken: token));

        Assert.Equal((Model.RunRunsMethod, FailureReason.Evaluation), (unobserved.Method, unobserved.Reason));
        Assert.StartsWith("invalid run request: no completed run", unobserved.Message, StringComparison.Ordinal);
        Assert.Equal(261, Assert.Single(unobserved.Partial.Rows).ElapsedMicroseconds);
        Assert.Equal(7UL, unobserved.Partial.Seed);
        Assert.Equal(unobserved.Partial.Standing, unobserved.Standing);
        Assert.Equal(unobserved.Partial.Instances, unobserved.Instances);
        Assert.Single(unobserved.Diagnostics);
        Assert.IsAssignableFrom<VerificationException>(unobserved);
        Assert.Equal(FailureReason.Unspecified, missing.Reason);
        Assert.Empty(missing.Partial.Rows);
        Assert.Same(Standing.None, missing.Standing);
        Assert.Contains("neither a run nor a failure", neither.Message, StringComparison.Ordinal);
        Assert.Contains("distributions[0].percentiles[0]", valueless.Message, StringComparison.Ordinal);
        Assert.Contains("distributions[0].histogram[0]", binless.Message, StringComparison.Ordinal);
        Assert.Contains("high", binless.Message, StringComparison.Ordinal);
        Assert.Contains("distributions[0].values[0]", uncounted.Message, StringComparison.Ordinal);
        Assert.All([neither, valueless, binless, uncounted], failure => Assert.Equal(StatusCode.Internal, failure.StatusCode));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task RunsGatesAreCheckedInTheServicesOrderBeforeAnythingIsSent(ClientWire wire)
    {
        var token = TestContext.Current.CancellationToken;
        var inputs = new Dictionary<string, Value> { ["x"] = new Value.ComplexValue(1, 1) };
        var action = new ActionRunsOptions { PerformerSymbolId = "MC::probe", Engine = "explore", Seed = 1 };
        var analysis = new AnalysisRunsOptions
        {
            Engine = "explore",
            Arguments = [new Value.ComplexValue(1, 1)],
            NamedArguments = new Dictionary<string, Value> { ["s"] = new Value.SetValue([]) },
        };

        Assert.Equal(Capabilities.Runs, await RefusedAsync(wire, [Capabilities.Runs, Capabilities.Performer], model => model.RunActionRunsAsync("A", 1, inputs, action, token)));
        Assert.Equal(Capabilities.Performer, await RefusedAsync(wire, [Capabilities.Performer, Capabilities.ComplexValues], model => model.RunActionRunsAsync("A", 1, inputs, action, token)));
        Assert.Equal(Capabilities.ComplexValues, await RefusedAsync(wire, [Capabilities.ComplexValues, Capabilities.Engines], model => model.RunActionRunsAsync("A", 1, inputs, action, token)));
        Assert.Equal(Capabilities.Engines, await RefusedAsync(wire, [Capabilities.Engines, Capabilities.ScheduleExplore], model => model.RunActionRunsAsync("A", 1, inputs, action, token)));
        Assert.Equal(Capabilities.ScheduleExplore, await RefusedAsync(wire, [Capabilities.ScheduleExplore], model => model.RunActionRunsAsync("A", 1, inputs, action, token)));
        Assert.Equal(Capabilities.Runs, await RefusedAsync(wire, [Capabilities.Runs, Capabilities.Engines], model => model.RunAnalysisRunsAsync("C", 1, analysis, token)));
        Assert.Equal(Capabilities.Engines, await RefusedAsync(wire, [Capabilities.Engines, Capabilities.ComplexValues], model => model.RunAnalysisRunsAsync("C", 1, analysis, token)));
        Assert.Equal(Capabilities.ScheduleExplore, await RefusedAsync(wire, [Capabilities.ScheduleExplore, Capabilities.ComplexValues], model => model.RunAnalysisRunsAsync("C", 1, analysis, token)));
        Assert.Equal(Capabilities.ComplexValues, await RefusedAsync(wire, [Capabilities.ComplexValues, Capabilities.SetValues], model => model.RunAnalysisRunsAsync("C", 1, analysis, token)));
        Assert.Equal(Capabilities.SetValues, await RefusedAsync(wire, [Capabilities.SetValues], model => model.RunAnalysisRunsAsync("C", 1, analysis, token)));

        using var handler = Serving(wire);
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash(Hash);
        var unset = await Assert.ThrowsAsync<InvalidRequestException>(() =>
            model.RunActionRunsAsync("A", 1, new Dictionary<string, Value> { ["x"] = new Value.UnsetValue() }, cancellationToken: token));
        var argument = await Assert.ThrowsAsync<InvalidRequestException>(() =>
            model.RunAnalysisRunsAsync("C", 1, new AnalysisRunsOptions { Arguments = [new Value.UnsetValue()] }, token));
        await Assert.ThrowsAsync<InvalidRequestException>(() => model.RunActionRunsAsync("A", 1, options: new ActionRunsOptions { PerformerSymbolId = string.Empty }, cancellationToken: token));
        await Assert.ThrowsAsync<InvalidRequestException>(() => model.RunActionRunsAsync("A", 1, options: new ActionRunsOptions { Draws = string.Empty }, cancellationToken: token));
        await Assert.ThrowsAsync<InvalidRequestException>(() => model.RunActionRunsAsync("A", 1, options: new ActionRunsOptions { Engine = string.Empty }, cancellationToken: token));
        await Assert.ThrowsAsync<InvalidRequestException>(() => model.RunAnalysisRunsAsync("C", 1, new AnalysisRunsOptions { SubjectSymbolId = string.Empty }, token));
        await Assert.ThrowsAsync<InvalidRequestException>(() => model.RunAnalysisRunsAsync("C", 1, new AnalysisRunsOptions { Draws = string.Empty }, token));
        await Assert.ThrowsAsync<InvalidRequestException>(() => model.RunAnalysisRunsAsync("C", 1, new AnalysisRunsOptions { Engine = string.Empty }, token));
        await Assert.ThrowsAsync<ArgumentNullException>(() => model.RunActionRunsAsync(null!, 1, cancellationToken: token));
        await Assert.ThrowsAsync<ArgumentNullException>(() => model.RunAnalysisRunsAsync(null!, 1, cancellationToken: token));
        await Assert.ThrowsAsync<ArgumentException>(() => model.RunActionRunsAsync("A", 1, new Dictionary<string, Value> { ["x"] = null! }, cancellationToken: token));
        Assert.Contains("inputs[\"x\"]", unset.Message, StringComparison.Ordinal);
        Assert.Contains("arguments[0]", argument.Message, StringComparison.Ordinal);
        Assert.Equal(["GetServerInfo"], handler.Requests.Select(request => request.Method));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task SelfCheckSendsTheRulePackagesAndReadsEveryFindingAndCount(ClientWire wire)
    {
        var answers = new Queue<Wire.SelfCheckResponse>([CompleteSamplingWire.SelfCheck, new Wire.SelfCheckResponse { Status = "holds", Summary = "0 violations" }]);
        using var handler = Serving(wire).On("SelfCheck", Wire.SelfCheckRequest.Parser, _ => answers.Dequeue());
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash(Hash);
        var token = TestContext.Current.CancellationToken;

        var failing = await model.SelfCheckAsync(["Rules", "More::Rules"], token);
        var holding = await model.SelfCheckAsync(cancellationToken: token);

        Assert.Equal(
            [new Wire.SelfCheckRequest { ModelHash = Hash, SelfCheckPackage = { "Rules", "More::Rules" } }, new Wire.SelfCheckRequest { ModelHash = Hash }],
            handler.RequestsFor("SelfCheck").Select(sent => sent.Message(Wire.SelfCheckRequest.Parser)));
        Assert.Equal((SelfCheck.FailsStatus, false), (failing.Status, failing.Holds));
        Assert.Equal(
            new SelfCheckFinding(
                "fails",
                "SysMLValidation::validatePartUsage for engine",
                "m.sysml:4:5",
                "SysMLValidation::validatePartUsage",
                "engine",
                string.Empty,
                "✗ SysMLValidation::validatePartUsage for engine (m.sysml:4:5)"),
            failing.Findings[0]);
        Assert.Equal(("warning", "Rules", "the rule package yields no constraint"), (failing.Findings[1].Status, failing.Findings[1].Subject, failing.Findings[1].Message));
        Assert.Equal(3, failing.Findings.Count);
        Assert.Equal((33L, 280L, 1L, 2L, 3L), (failing.Elements, failing.Checks, failing.Violations, failing.Unevaluated, failing.EvaluationErrors));
        Assert.Equal(
            [new SelfCheckApplication("SysMLValidation::validateAttributeUsageFeatures", 5), new SelfCheckApplication("SysMLValidation::validatePartUsage", 0)],
            failing.Applications);
        Assert.StartsWith("Self-model check: 33 elements", failing.Summary, StringComparison.Ordinal);
        Assert.Single(failing.Diagnostics);
        Assert.True(holding.Holds);
        Assert.False(new SelfCheck(SelfCheck.UnresolvedStatus, [], 0, 0, 0, 0, 1, [], string.Empty, []).Holds);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ASelfCheckThatCannotStartIsAModelExceptionAndAMalformedAnswerIsRefused(ClientWire wire)
    {
        using var handler = Serving(wire).On("SelfCheck", Wire.SelfCheckRequest.Parser, request => request.SelfCheckPackage.FirstOrDefault() switch
        {
            "Nope" => new Wire.SelfCheckResponse { Error = "-self-check-package Nope names no package", Diagnostics = { CompleteWire.Diagnostic } },
            "both" => new Wire.SelfCheckResponse { Error = "e", Checks = 3 },
            "summary" => new Wire.SelfCheckResponse { Error = "e", Summary = "s" },
            _ => new Wire.SelfCheckResponse { Summary = "a summary without a status" },
        });
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash(Hash);
        var token = TestContext.Current.CancellationToken;

        var unknown = await Assert.ThrowsAsync<ModelException>(() => model.SelfCheckAsync(["Nope"], token));
        var both = await Assert.ThrowsAsync<TransportException>(() => model.SelfCheckAsync(["both"], token));
        var summary = await Assert.ThrowsAsync<TransportException>(() => model.SelfCheckAsync(["summary"], token));
        var neither = await Assert.ThrowsAsync<TransportException>(() => model.SelfCheckAsync(cancellationToken: token));
        await Assert.ThrowsAsync<ArgumentException>(() => model.SelfCheckAsync([null!], token));

        Assert.Equal(typeof(ModelException), unknown.GetType());
        Assert.Equal((Model.SelfCheckMethod, "-self-check-package Nope names no package"), (unknown.Method, unknown.Message));
        Assert.Single(unknown.Diagnostics);
        Assert.Contains("beside checks", both.Message, StringComparison.Ordinal);
        Assert.Contains("beside summary", summary.Message, StringComparison.Ordinal);
        Assert.Contains("neither a status nor a failure", neither.Message, StringComparison.Ordinal);
        Assert.All([both, summary, neither], failure => Assert.Equal(StatusCode.Internal, failure.StatusCode));
        Assert.Equal(Capabilities.SelfCheck, await RefusedAsync(wire, [Capabilities.SelfCheck], model => model.SelfCheckAsync(cancellationToken: token)));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task RecordRunSendsEveryOptionAndAnswersANewModelLeavingTheOldOneAsItWas(ClientWire wire)
    {
        var answers = new Queue<Wire.RecordRunResponse>([CompleteSamplingWire.Recording, CompleteSamplingWire.SweepRecording, CompleteSamplingWire.SampleRecording]);
        using var handler = Serving(wire).On("RecordRun", Wire.RecordRunRequest.Parser, _ => answers.Dequeue());
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash(Hash);
        var token = TestContext.Current.CancellationToken;
        var once = new RecordOptions
        {
            SubjectSymbolId = "Sw::barge",
            Arguments = [new Value.RealValue(10)],
            NamedArguments = new Dictionary<string, Value> { ["limit"] = new Value.RealValue(50) },
            Engine = "run",
            RecordInto = "Sw",
        };
        SweepRange[] ranges = [new("limit", new Value.RealValue(0), new Value.RealValue(40), new Value.RealValue(20))];

        var run = await model.RecordRunAsync("Sw::CostAnalysis", once, token);
        var swept = await model.RecordRunAsync("Sw::CostAnalysis", new RecordOptions { Ranges = ranges }, token);
        var sampled = await model.RecordRunAsync("MC::Mc", new RecordOptions { Runs = 2, Seed = 3 }, token);

        Assert.Equal(
            [
                new Wire.RecordRunRequest
                {
                    ModelHash = Hash,
                    AnalysisSymbolId = "Sw::CostAnalysis",
                    SubjectSymbolId = "Sw::barge",
                    Arguments = { new Wire.Value { RealValue = 10 } },
                    NamedArguments = { ["limit"] = new Wire.Value { RealValue = 50 } },
                    Engine = "run",
                    RecordInto = "Sw",
                    AcceptDocuments = true,
                },
                new Wire.RecordRunRequest
                {
                    ModelHash = Hash,
                    AnalysisSymbolId = "Sw::CostAnalysis",
                    Ranges = { new Wire.SweepRange { Parameter = "limit", Start = new Wire.Value { RealValue = 0 }, End = new Wire.Value { RealValue = 40 }, Step = new Wire.Value { RealValue = 20 } } },
                    AcceptDocuments = true,
                },
                new Wire.RecordRunRequest { ModelHash = Hash, AnalysisSymbolId = "MC::Mc", Runs = 2, Seed = 3, AcceptDocuments = true },
            ],
            handler.RequestsFor("RecordRun").Select(sent => sent.Message(Wire.RecordRunRequest.Parser)));
        Assert.Equal(["total", "best"], run.Run!.Outputs.Select(output => output.Name));
        Assert.Null(run.Sweep);
        Assert.Null(run.Sample);
        Assert.Equal(("Sw", "Sw::CostAnalysisRun"), (run.Package, run.Definition));
        Assert.Equal(["Sw::CostAnalysis_run1"], run.Records);
        Assert.StartsWith("package Sw {", run.Source, StringComparison.Ordinal);
        Assert.Contains("part barge", run.Content, StringComparison.Ordinal);
        Assert.Equal(new EditedDocument("sweep.sysml", run.Content), Assert.Single(run.Documents));
        Assert.Equal(1, run.Skipped);
        Assert.Equal(["run 2 not recorded: mass is unread"], run.SkipReasons);
        Assert.Single(run.Diagnostics);
        Assert.Equal("recorded", run.Model.Hash);
        Assert.Same(connection, run.Model.Connection);
        Assert.False(run.Model.OwnsConnection);
        Assert.Empty(run.Model.Roots);
        Assert.Equal(Hash, model.Hash);
        Assert.Equal(3, swept.Sweep!.Rows.Count);
        Assert.Null(swept.Run);
        Assert.Empty(swept.Content);
        Assert.Equal("Records.sysml", swept.Documents.Single().Name);
        Assert.Equal("swept", swept.Model.Hash);
        Assert.Equal(["runs", "mean"], sampled.Sample!.Outputs.Select(output => output.Name));
        Assert.Equal("sampled", sampled.Model.Hash);
        Assert.Equal(Hash, model.Hash);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ARecordingNotMadeIsARecordExceptionKeepingTheRunsAndWhetherTheyFailed(ClientWire wire)
    {
        using var handler = Serving(wire).On("RecordRun", Wire.RecordRunRequest.Parser, request => request.AnalysisSymbolId switch
        {
            "refused" => CompleteSamplingWire.RefusedRecording,
            "failed" => CompleteSamplingWire.FailedRunRecording,
            "sweep" => new Wire.RecordRunResponse
            {
                Sweep = new Wire.RunSweepResponse { Error = "Sw::Sum is not an analysis case", FailureReason = Wire.FailureReason.WrongKind },
                Error = "Sw::Sum is not an analysis case",
                FailureReason = Wire.FailureReason.WrongKind,
            },
            "sample" => new Wire.RecordRunResponse
            {
                Sample = CompleteSamplingWire.Unconcluded,
                Skipped = 1,
                Error = "nothing recorded: no run completed",
            },
            _ => new Wire.RecordRunResponse { Error = "symbol not found: MC::Nope" },
        });
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash(Hash);
        var token = TestContext.Current.CancellationToken;
        var sweep = new RecordOptions { Ranges = [new SweepRange("b", new Value.RealValue(0), new Value.RealValue(1))] };

        var refused = await Assert.ThrowsAsync<RecordException>(() => model.RecordRunAsync("refused", cancellationToken: token));
        var failed = await Assert.ThrowsAsync<RecordException>(() => model.RecordRunAsync("failed", cancellationToken: token));
        var wrong = await Assert.ThrowsAsync<RecordException>(() => model.RecordRunAsync("sweep", sweep, token));
        var none = await Assert.ThrowsAsync<RecordException>(() => model.RecordRunAsync("sample", new RecordOptions { Runs = 1, Seed = 1 }, token));
        var missing = await Assert.ThrowsAsync<RecordException>(() => model.RecordRunAsync("MC::Nope", cancellationToken: token));

        Assert.Equal((Model.RecordRunMethod, FailureReason.Unspecified, false), (refused.Method, refused.Reason, refused.RunFailed));
        Assert.Equal(("Records", "Records::CostAnalysisRun", "package Records;\n"), (refused.Package, refused.Definition, refused.GeneratedSource));
        Assert.Equal(["Records::CostAnalysis_run1"], refused.Records);
        Assert.Equal((2L, "run 2 not recorded: mass is unread"), (refused.Skipped, Assert.Single(refused.SkipReasons)));
        Assert.Single(refused.Diagnostics);
        Assert.Equal(refused.Run!.Instances, refused.Instances);
        Assert.Equal(refused.Run.Verifications, refused.Verifications);
        Assert.Equal(refused.Run.Standing, refused.Standing);
        Assert.True(failed.RunFailed);
        Assert.Equal(("analysis run failed: step budget exhausted", FailureReason.Evaluation), (failed.Message, failed.Reason));
        Assert.Equal(new CalcOutput("total", new Value.RealValue(7)), Assert.Single(failed.Run!.Outputs));
        Assert.Empty(failed.Records);
        Assert.True(wrong.RunFailed);
        Assert.Equal(FailureReason.WrongKind, wrong.Reason);
        Assert.NotNull(wrong.Sweep);
        Assert.False(none.RunFailed);
        Assert.Equal("no run completed, so the case is not concluded", none.Sample!.Unconcluded);
        Assert.Equal(1, none.Skipped);
        Assert.Equal((false, null, null, null), (missing.RunFailed, missing.Run, missing.Sweep, missing.Sample));
        Assert.Same(Standing.None, missing.Standing);
        Assert.IsAssignableFrom<VerificationException>(missing);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ARecordingAnsweredInAShapeTheRequestDoesNotMakeIsMalformed(ClientWire wire)
    {
        using var handler = Serving(wire).On("RecordRun", Wire.RecordRunRequest.Parser, request => request.AnalysisSymbolId switch
        {
            "content" => new Wire.RecordRunResponse { Error = "e", Content = "package P;" },
            "documents" => new Wire.RecordRunResponse { Error = "e", Documents = { new Wire.EditedDocument { Name = "a" } } },
            "hash" => new Wire.RecordRunResponse { Error = "e", ModelHash = "m" },
            "other failure" => new Wire.RecordRunResponse { Run = new Wire.RunAnalysisResponse { Error = "x" }, Error = "y" },
            "other reason" => new Wire.RecordRunResponse
            {
                Run = new Wire.RunAnalysisResponse { Error = "x", FailureReason = Wire.FailureReason.WrongKind },
                Error = "x",
                FailureReason = Wire.FailureReason.Evaluation,
            },
            "failed run beside" => new Wire.RecordRunResponse { Run = new Wire.RunAnalysisResponse { Error = "x" }, ModelHash = "m" },
            "hashless" => new Wire.RecordRunResponse { Run = new Wire.RunAnalysisResponse() },
            "runless" => new Wire.RecordRunResponse { ModelHash = "m" },
            "two" => new Wire.RecordRunResponse { Run = new Wire.RunAnalysisResponse(), Sweep = new Wire.RunSweepResponse(), ModelHash = "m" },
            "explored" => new Wire.RecordRunResponse { Run = CompleteVerificationWire.Exploration, ModelHash = "m" },
            _ => new Wire.RecordRunResponse { Sweep = new Wire.RunSweepResponse(), ModelHash = "m" },
        });
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash(Hash);
        var token = TestContext.Current.CancellationToken;

        async Task<string> MalformedAsync(string answer, RecordOptions? options = null)
        {
            var failure = await Assert.ThrowsAsync<TransportException>(() => model.RecordRunAsync(answer, options, token));
            Assert.Equal(StatusCode.Internal, failure.StatusCode);
            return failure.Message;
        }

        Assert.Contains("beside content", await MalformedAsync("content"), StringComparison.Ordinal);
        Assert.Contains("beside documents", await MalformedAsync("documents"), StringComparison.Ordinal);
        Assert.Contains("beside model_hash", await MalformedAsync("hash"), StringComparison.Ordinal);
        Assert.Contains("not the failure it reported", await MalformedAsync("other failure"), StringComparison.Ordinal);
        Assert.Contains("not the failure it reported", await MalformedAsync("other reason"), StringComparison.Ordinal);
        Assert.Contains("beside a failure of the runs", await MalformedAsync("failed run beside"), StringComparison.Ordinal);
        Assert.Contains("neither a recorded model", await MalformedAsync("hashless"), StringComparison.Ordinal);
        Assert.Contains("neither a recorded model", await MalformedAsync("runless"), StringComparison.Ordinal);
        Assert.Contains("made by run and sweep", await MalformedAsync("two"), StringComparison.Ordinal);
        Assert.Contains("exploration", await MalformedAsync("explored"), StringComparison.Ordinal);
        Assert.Contains("made by sweep, which a request for run", await MalformedAsync("swept"), StringComparison.Ordinal);
        Assert.Contains("made by sweep, which a request for sample", await MalformedAsync("swept", new RecordOptions { Runs = 2, Seed = 1 }), StringComparison.Ordinal);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task RecordGatesAreCheckedInTheServicesOrderBeforeAnythingIsSent(ClientWire wire)
    {
        var token = TestContext.Current.CancellationToken;
        var options = new RecordOptions
        {
            Engine = "explore",
            Arguments = [new Value.ComplexValue(1, 1)],
            NamedArguments = new Dictionary<string, Value> { ["s"] = new Value.SetValue([]) },
            Ranges = [new("x", new Value.IntegerValue(0), new Value.InfinityValue(), new Value.FunctionValue("F::step", null))],
        };

        Assert.Equal(Capabilities.RecordRun, await RefusedAsync(wire, [Capabilities.RecordRun, Capabilities.Engines], model => model.RecordRunAsync("C", options, token)));
        Assert.Equal(Capabilities.Engines, await RefusedAsync(wire, [Capabilities.Engines, Capabilities.ScheduleExplore], model => model.RecordRunAsync("C", options, token)));
        Assert.Equal(Capabilities.ScheduleExplore, await RefusedAsync(wire, [Capabilities.ScheduleExplore, Capabilities.ComplexValues], model => model.RecordRunAsync("C", options, token)));
        Assert.Equal(Capabilities.ComplexValues, await RefusedAsync(wire, [Capabilities.ComplexValues, Capabilities.SetValues], model => model.RecordRunAsync("C", options, token)));
        Assert.Equal(Capabilities.SetValues, await RefusedAsync(wire, [Capabilities.SetValues, Capabilities.InfinityValue], model => model.RecordRunAsync("C", options, token)));
        Assert.Equal(Capabilities.InfinityValue, await RefusedAsync(wire, [Capabilities.InfinityValue, Capabilities.FunctionValues], model => model.RecordRunAsync("C", options, token)));
        Assert.Equal(Capabilities.FunctionValues, await RefusedAsync(wire, [Capabilities.FunctionValues], model => model.RecordRunAsync("C", options, token)));

        using var handler = Serving(wire);
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash(Hash);
        var step = await Assert.ThrowsAsync<InvalidRequestException>(() =>
            model.RecordRunAsync("C", new RecordOptions { Ranges = [new SweepRange("x", new Value.IntegerValue(0), new Value.IntegerValue(1), new Value.UnsetValue())] }, token));
        await Assert.ThrowsAsync<InvalidRequestException>(() => model.RecordRunAsync("C", new RecordOptions { SubjectSymbolId = string.Empty }, token));
        await Assert.ThrowsAsync<InvalidRequestException>(() => model.RecordRunAsync("C", new RecordOptions { Engine = string.Empty }, token));
        await Assert.ThrowsAsync<InvalidRequestException>(() => model.RecordRunAsync("C", new RecordOptions { RecordInto = string.Empty }, token));
        await Assert.ThrowsAsync<ArgumentNullException>(() => model.RecordRunAsync(null!, cancellationToken: token));
        Assert.Contains("ranges[0].step", step.Message, StringComparison.Ordinal);
        Assert.Equal(["GetServerInfo"], handler.Requests.Select(request => request.Method));
    }

    [Fact]
    public async Task TheOptionsCopyTheirCollectionsAndTheRecordsRefuseWhatTheyCannotHold()
    {
        var observe = new List<string> { "t" };
        var ranges = new List<SweepRange> { new("x", new Value.IntegerValue(0), new Value.IntegerValue(1)) };
        var action = new ActionRunsOptions { Observe = observe };
        var record = new RecordOptions { Ranges = ranges };
        using var handler = new FakeServiceHandler { Capabilities = FakeModels.ModelCapabilities };
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash("m");
        var analysis = new Analysis([], [], [], [], [], [], Standing.None, null);
        var sweep = new Sweep([], [], false, 0, [], [], Standing.None);
        var runs = new Runs([], [], [], [], [], [], string.Empty, null, [], [], Standing.None);

        observe.Clear();
        ranges.Clear();

        Assert.Equal(["t"], action.Observe);
        Assert.Single(record.Ranges);
        Assert.Empty(ActionRunsOptions.Default.Observe);
        Assert.Null(ActionRunsOptions.Default.Seed);
        Assert.Empty(AnalysisRunsOptions.Default.Arguments);
        Assert.Empty(AnalysisRunsOptions.Default.NamedArguments);
        Assert.Equal((0L, null, null), (RecordOptions.Default.Runs, RecordOptions.Default.Seed, RecordOptions.Default.RecordInto));
        Assert.Empty(RecordOptions.Default.Ranges);
        Assert.Throws<ArgumentException>(() => new ActionRunsOptions { Observe = [null!] });
        Assert.Throws<ArgumentException>(() => new RecordOptions { Ranges = [null!] });
        Assert.Throws<ArgumentNullException>(() => new RunPercentile(50, null!));
        Assert.Throws<ArgumentNullException>(() => new RunHistogramBin(new Value.IntegerValue(0), null!, 1));
        Assert.Throws<ArgumentNullException>(() => new RunCountedValue(null!, 1));
        Assert.Throws<ArgumentNullException>(() => new RunDistribution("t", null!, 0, false, null, null, null, 0, [], [], []));
        Assert.Throws<ArgumentNullException>(() => new Runs([], [], [], [], [], [], null!, null, [], [], Standing.None));
        Assert.Throws<ArgumentNullException>(() => new SelfCheckFinding("fails", "s", "l", "c", "e", "m", null!));
        Assert.Throws<ArgumentNullException>(() => new SelfCheckApplication(null!, 1));
        Assert.Throws<ArgumentNullException>(() => new SelfCheck("holds", [], 0, 0, 0, 0, 0, [], null!, []));
        Assert.Throws<ArgumentNullException>(() => new EditedDocument("a", null!));
        Assert.Throws<ArgumentNullException>(() => new Recording(analysis, null, null, "p", "d", [], "s", "c", [], 0, [], [], null!));
        Assert.Throws<ArgumentException>(() => new Recording(null, null, null, "p", "d", [], "s", "c", [], 0, [], [], model));
        Assert.Throws<ArgumentException>(() => new Recording(analysis, sweep, null, "p", "d", [], "s", "c", [], 0, [], [], model));
        Assert.Throws<ArgumentNullException>(() => new RunsException("m", Model.RunRunsMethod, FailureReason.Unspecified, null!));
        Assert.Throws<ArgumentException>(() => new RecordException("m", Model.RecordRunMethod, FailureReason.Unspecified, [], analysis, null, runs, false, "", "", [], "", 0, []));
        Assert.Throws<ArgumentException>(() => new RecordException("m", Model.RecordRunMethod, FailureReason.Unspecified, [], null, null, null, true, "", "", [], "", 0, []));
        Assert.Throws<ArgumentNullException>(() => new RecordException("m", Model.RecordRunMethod, FailureReason.Unspecified, [], null, null, null, false, "", "", [], null!, 0, []));
        Assert.Equal(sweep.Standing, new RecordException("m", Model.RecordRunMethod, FailureReason.Unspecified, [], null, sweep, null, true, "", "", [], "", 0, []).Standing);
        Assert.Equal(runs, new RecordException("m", Model.RecordRunMethod, FailureReason.Unspecified, [], null, null, runs, false, "", "", [], "", 0, []).Sample);
        Assert.Equal(runs, new Recording(null, null, runs, "p", "d", [], "s", "c", [], 0, [], [], model).Sample);
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
