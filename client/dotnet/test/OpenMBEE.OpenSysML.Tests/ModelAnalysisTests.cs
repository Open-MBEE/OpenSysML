using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading.Tasks;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// EvaluateCalc, RunAnalysis, RunSweep and ListEngines through the typed API, over every wire: the exact request each
/// option makes, every field of each answer read, a failed analysis raised with its partial run, a failed sweep row
/// kept as a row, a run and an exploration refusing each other's options, a malformed answer refused, and the gates
/// checked in the service's order before anything is sent.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed class ModelAnalysisTests
{
    private const string Hash = "h";

    public static TheoryData<ClientWire> Wires => ClientWires.All;

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task CalcSendsItsArgumentsInOrderAndReadsTheValueOutputsAndStanding(ClientWire wire)
    {
        var answers = new Queue<Wire.EvaluateCalcResponse>([CompleteVerificationWire.Calculation, new Wire.EvaluateCalcResponse { Result = new Wire.Value { IntValue = 5 } }]);
        using var handler = Serving(wire).On("EvaluateCalc", Wire.EvaluateCalcRequest.Parser, _ => answers.Dequeue());
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash(Hash);
        var token = TestContext.Current.CancellationToken;

        var usage = await model.CalcAsync("Demo::rect", options: new CalcOptions { Engine = "run" }, cancellationToken: token);
        var invoked = await model.CalcAsync("Demo::add", [new Value.IntegerValue(2), new Value.RationalValue(new Rational(1, 3))], cancellationToken: token);

        Assert.Equal(
            [
                new Wire.EvaluateCalcRequest { ModelHash = Hash, SymbolId = "Demo::rect", Engine = "run" },
                new Wire.EvaluateCalcRequest
                {
                    ModelHash = Hash,
                    SymbolId = "Demo::add",
                    Arguments = { new Wire.Value { IntValue = 2 }, new Wire.Value { RationalValue = new Wire.Rational { Numerator = "1", Denominator = "3" } } },
                },
            ],
            handler.RequestsFor("EvaluateCalc").Select(sent => sent.Message(Wire.EvaluateCalcRequest.Parser)));
        Assert.Equal(new Value.RealValue(5), usage.Result);
        Assert.Equal([new CalcOutput("area", new Value.RealValue(12)), new CalcOutput("perimeter", new Value.IntegerValue(14))], usage.Outputs);
        Assert.Equal("area = 12.0", usage.Outputs[0].ToString());
        Assert.Equal(new Standing("run", "observed", [new Bound("solver", 5000, true), new Bound("depth", 64, false)]), usage.Standing);
        Assert.Single(usage.Diagnostics);
        Assert.Equal(new Value.IntegerValue(5), invoked.Value);
        Assert.Same(Standing.None, invoked.Standing);
        Assert.Equal(new Value.IntegerValue(3), new Calculation(null, [new CalcOutput("x", new Value.IntegerValue(3))], [], Standing.None).Value);
        Assert.Null(new Calculation(null, [new CalcOutput("x", new Value.IntegerValue(3)), new CalcOutput("y", new Value.IntegerValue(4))], [], Standing.None).Value);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ACalcThatFailsIsAVerificationExceptionAndAnAnswerOfNeitherOrBothIsMalformed(ClientWire wire)
    {
        using var handler = Serving(wire).On("EvaluateCalc", Wire.EvaluateCalcRequest.Parser, request => request.SymbolId switch
        {
            "Demo::sedan" => new Wire.EvaluateCalcResponse
            {
                Error = "Demo::sedan declares no calc",
                FailureReason = Wire.FailureReason.WrongKind,
                Diagnostics = { CompleteWire.Diagnostic },
                Engine = "run",
                Strength = "not covered",
            },
            "both" => new Wire.EvaluateCalcResponse { Error = "failed", Outputs = { new Wire.CalcOutput { Name = "x", Value = new Wire.Value { IntValue = 1 } } } },
            "valueless" => new Wire.EvaluateCalcResponse { Outputs = { new Wire.CalcOutput { Name = "x" } } },
            _ => new Wire.EvaluateCalcResponse(),
        });
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash(Hash);
        var token = TestContext.Current.CancellationToken;

        var wrong = await Assert.ThrowsAsync<VerificationException>(() => model.CalcAsync("Demo::sedan", cancellationToken: token));
        var neither = await Assert.ThrowsAsync<TransportException>(() => model.CalcAsync("neither", cancellationToken: token));
        var both = await Assert.ThrowsAsync<TransportException>(() => model.CalcAsync("both", cancellationToken: token));
        var valueless = await Assert.ThrowsAsync<TransportException>(() => model.CalcAsync("valueless", cancellationToken: token));

        Assert.Equal((Model.EvaluateCalcMethod, FailureReason.WrongKind), (wrong.Method, wrong.Reason));
        Assert.Equal(("run", "not covered"), (wrong.Standing.Engine, wrong.Standing.Strength));
        Assert.Single(wrong.Diagnostics);
        Assert.Contains("neither a value nor a failure", neither.Message, StringComparison.Ordinal);
        Assert.Contains("beside outputs", both.Message, StringComparison.Ordinal);
        Assert.Contains("outputs[0]", valueless.Message, StringComparison.Ordinal);
        Assert.All([neither, both, valueless], failure => Assert.Equal(StatusCode.Internal, failure.StatusCode));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task RunAnalysisSendsEveryOptionAndReadsEveryFieldOfTheRun(ClientWire wire)
    {
        using var handler = Serving(wire).On("RunAnalysis", Wire.RunAnalysisRequest.Parser, _ => CompleteVerificationWire.Analysis);
        await using var connection = await FakeConnection.OpenAsync(handler);
        var options = new AnalysisOptions
        {
            SubjectSymbolId = "An::barge",
            Arguments = [new Value.RealValue(10)],
            NamedArguments = new Dictionary<string, Value> { ["limit"] = new Value.RealValue(50) },
            Schedule = "declared",
            Engine = "run",
            ToolDryRun = true,
        };

        var analysis = await connection.ModelByHash(Hash).RunAnalysisAsync("An::CostAnalysis", options, TestContext.Current.CancellationToken);

        Assert.Equal(
            new Wire.RunAnalysisRequest
            {
                ModelHash = Hash,
                SymbolId = "An::CostAnalysis",
                SubjectSymbolId = "An::barge",
                Arguments = { new Wire.Value { RealValue = 10 } },
                NamedArguments = { ["limit"] = new Wire.Value { RealValue = 50 } },
                Schedule = "declared",
                Engine = "run",
                ToolDryRun = true,
            },
            Assert.Single(handler.RequestsFor("RunAnalysis")).Message(Wire.RunAnalysisRequest.Parser));
        Assert.Equal(["total", "best"], analysis.Outputs.Select(output => output.Name));
        Assert.Equal(9, analysis.Resolve((Value.InstanceRefValue)analysis.Outputs[1].Value)!.Id);
        Assert.Equal("affordable", analysis.Objective!.Element);
        Assert.False(analysis.Holds);
        Assert.Equal("Demo::checkBound", Assert.Single(analysis.Verifications).CaseId);
        Assert.Equal(new CaseEvaluation("An::score", [new Value.InstanceRefValue(9)], new Value.RealValue(0.5), string.Empty, true, false), analysis.Selected);
        Assert.Equal((null, "division by zero", true), (analysis.Evaluations[1].Result, analysis.Evaluations[1].Error, analysis.Evaluations[1].Tied));
        Assert.Single(analysis.Instances);
        Assert.Single(analysis.Diagnostics);
        Assert.Equal("run", analysis.Standing.Engine);
        var preview = analysis.ToolDryRun!;
        Assert.Equal(
            ("Demo::simulate", "solver", "1.2", "/tools/solver.json", "/usr/bin/solver", "argv+none/csv", "/work", "template", "mass=1500", true, "file:out.csv"),
            (preview.Action, preview.Tool, preview.Version, preview.Manifest, preview.Executable, preview.Protocol, preview.WorkingDirectory, preview.Stdin, preview.StdinData, preview.OutputDirectory, preview.ReplySource));
        Assert.Equal(["--mass", "1500"], preview.Args);
        Assert.Equal(["HOME=<from this process>", "MODE=fast"], preview.Environment);
        Assert.Equal(new ToolDryRunFile("in.csv", "csv", "mass\n1500\n"), preview.InputFile);
        Assert.Equal(new ToolDryRunInput("m", "mass", "1500", "kg"), Assert.Single(preview.Inputs));
        Assert.Equal(new ToolDryRunOutput("t", "time"), Assert.Single(preview.Outputs));
        Assert.Equal("tool solver 1.2\nreply from file:out.csv", preview.ToString());
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AFailedAnalysisIsAnAnalysisExceptionKeepingThePartialRun(ClientWire wire)
    {
        using var handler = Serving(wire).On("RunAnalysis", Wire.RunAnalysisRequest.Parser, request => request.SymbolId == "An::missing"
            ? new Wire.RunAnalysisResponse { Error = "symbol not found: An::missing" }
            : CompleteVerificationWire.FailedAnalysis);
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash(Hash);
        var token = TestContext.Current.CancellationToken;

        var failed = await Assert.ThrowsAsync<AnalysisException>(() => model.RunAnalysisAsync("An::CostAnalysis", cancellationToken: token));
        var missing = await Assert.ThrowsAsync<AnalysisException>(() => model.RunAnalysisAsync("An::missing", cancellationToken: token));

        Assert.Equal(("analysis run failed: step budget exhausted", Model.RunAnalysisMethod, FailureReason.Evaluation), (failed.Message, failed.Method, failed.Reason));
        Assert.Equal(new Value.RealValue(7), Assert.Single(failed.Partial.Outputs).Value);
        Assert.False(Assert.Single(failed.Partial.Verdicts).Decided);
        Assert.Equal("no value", Assert.Single(failed.Partial.Evaluations).Error);
        Assert.Equal(failed.Partial.Instances, failed.Instances);
        Assert.Equal(failed.Partial.Diagnostics, failed.Diagnostics);
        Assert.Equal(failed.Partial.Standing, failed.Standing);
        Assert.Equal("not covered", failed.Standing.Strength);
        Assert.IsAssignableFrom<VerificationException>(failed);
        Assert.Equal(FailureReason.Unspecified, missing.Reason);
        Assert.Empty(missing.Partial.Outputs);
        Assert.Same(Standing.None, missing.Partial.Standing);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ExploreAnalysisSendsExploreUnlessAScheduleOrTheExploreEngineIsNamedAndReadsTheStanding(ClientWire wire)
    {
        using var handler = Serving(wire).On("RunAnalysis", Wire.RunAnalysisRequest.Parser, _ => CompleteVerificationWire.Exploration);
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash(Hash);
        var token = TestContext.Current.CancellationToken;

        var exploration = await model.ExploreAnalysisAsync("An::race", new AnalysisOptions { SubjectSymbolId = "An::ship" }, token);
        await model.ExploreAnalysisAsync("An::race", new AnalysisOptions { Schedule = "explore:runs=4,depth=2" }, token);
        await model.ExploreAnalysisAsync("An::race", new AnalysisOptions { Engine = "explore" }, token);

        Assert.Equal(
            [
                new Wire.RunAnalysisRequest { ModelHash = Hash, SymbolId = "An::race", SubjectSymbolId = "An::ship", Schedule = "explore" },
                new Wire.RunAnalysisRequest { ModelHash = Hash, SymbolId = "An::race", Schedule = "explore:runs=4,depth=2" },
                new Wire.RunAnalysisRequest { ModelHash = Hash, SymbolId = "An::race", Engine = "explore" },
            ],
            handler.RequestsFor("RunAnalysis").Select(sent => sent.Message(Wire.RunAnalysisRequest.Parser)));
        Assert.Equal(2, exploration.Outcomes.Count);
        Assert.Equal(new Standing("explore", "bounded", [new Bound("solver", 5000, true), new Bound("depth", 64, false)]), exploration.Standing);
        Assert.True(new AnalysisOptions { Engine = " explore " }.Explores);
        Assert.True(new AnalysisOptions { Schedule = "explore:runs=1" }.Explores);
        Assert.False(new AnalysisOptions { Schedule = "declared", Engine = "run" }.Explores);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ARunAndAnExplorationOfACaseEachRefuseTheOthersOptionsBeforeSending(ClientWire wire)
    {
        using var handler = Serving(wire);
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash(Hash);
        var token = TestContext.Current.CancellationToken;

        var schedule = await Assert.ThrowsAsync<InvalidRequestException>(() => model.RunAnalysisAsync("A", new AnalysisOptions { Schedule = "explore" }, token));
        var engine = await Assert.ThrowsAsync<InvalidRequestException>(() => model.RunAnalysisAsync("A", new AnalysisOptions { Engine = "explore" }, token));
        var once = await Assert.ThrowsAsync<InvalidRequestException>(() => model.ExploreAnalysisAsync("A", new AnalysisOptions { Schedule = "seed:3" }, token));
        var dryRun = await Assert.ThrowsAsync<InvalidRequestException>(() => model.ExploreAnalysisAsync("A", new AnalysisOptions { ToolDryRun = true }, token));
        await Assert.ThrowsAsync<InvalidRequestException>(() => model.RunAnalysisAsync("A", new AnalysisOptions { SubjectSymbolId = string.Empty }, token));
        await Assert.ThrowsAsync<InvalidRequestException>(() => model.ExploreAnalysisAsync("A", new AnalysisOptions { Schedule = string.Empty }, token));
        await Assert.ThrowsAsync<InvalidRequestException>(() => model.RunAnalysisAsync("A", new AnalysisOptions { Engine = string.Empty }, token));
        await Assert.ThrowsAsync<InvalidRequestException>(() =>
            model.RunAnalysisAsync("A", new AnalysisOptions { Arguments = [new Value.FunctionValue("F::f", 3)] }, token));
        await Assert.ThrowsAsync<ArgumentNullException>(() => model.RunAnalysisAsync(null!, cancellationToken: token));
        await Assert.ThrowsAsync<ArgumentNullException>(() => model.ExploreAnalysisAsync(null!, cancellationToken: token));

        Assert.Contains(nameof(Model.ExploreAnalysisAsync), schedule.Message, StringComparison.Ordinal);
        Assert.Contains("engine explore", engine.Message, StringComparison.Ordinal);
        Assert.Contains(nameof(Model.RunAnalysisAsync), once.Message, StringComparison.Ordinal);
        Assert.Contains("ToolDryRun", dryRun.Message, StringComparison.Ordinal);
        Assert.Equal(["GetServerInfo"], handler.Requests.Select(request => request.Method));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AnAnalysisAnswerOfTheOtherShapeIsMalformed(ClientWire wire)
    {
        using var handler = Serving(wire).On("RunAnalysis", Wire.RunAnalysisRequest.Parser, request => request.SymbolId switch
        {
            "run answered as explored" => CompleteVerificationWire.Exploration,
            "explored answered as run" => new Wire.RunAnalysisResponse { Outputs = { new Wire.CalcOutput { Name = "x", Value = new Wire.Value { IntValue = 1 } } }, Exploration = new Wire.ExplorationStatus() },
            "explored without status" => new Wire.RunAnalysisResponse(),
            _ => new Wire.RunAnalysisResponse { Error = "failed", Exploration = new Wire.ExplorationStatus() },
        });
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash(Hash);
        var token = TestContext.Current.CancellationToken;

        TransportException[] malformed =
        [
            await Assert.ThrowsAsync<TransportException>(() => model.RunAnalysisAsync("run answered as explored", cancellationToken: token)),
            await Assert.ThrowsAsync<TransportException>(() => model.ExploreAnalysisAsync("explored answered as run", cancellationToken: token)),
            await Assert.ThrowsAsync<TransportException>(() => model.ExploreAnalysisAsync("explored without status", cancellationToken: token)),
            await Assert.ThrowsAsync<TransportException>(() => model.ExploreAnalysisAsync("failure beside status", cancellationToken: token)),
        ];

        Assert.All(malformed, failure => Assert.Equal((StatusCode.Internal, Model.RunAnalysisMethod), (failure.StatusCode, failure.Method)));
        Assert.Contains("only one run reports", malformed[1].Message, StringComparison.Ordinal);
        Assert.Contains("beside exploration", malformed[3].Message, StringComparison.Ordinal);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AnalysisGatesAreCheckedInTheServicesOrderBeforeAnythingIsSent(ClientWire wire)
    {
        var token = TestContext.Current.CancellationToken;
        var everything = new AnalysisOptions
        {
            Schedule = "explore",
            Engine = "explore",
            Arguments = [new Value.VectorValue([new Value.IntegerValue(1)])],
            NamedArguments = new Dictionary<string, Value> { ["m"] = new Value.MetaobjectValue("Demo::x", "SysML::Systems::PartUsage") },
        };
        var run = new AnalysisOptions { Schedule = "declared", Engine = "run" };

        Assert.Equal(Capabilities.Schedule, await RefusedAsync(wire, [Capabilities.Schedule, Capabilities.Verification], model => model.ExploreAnalysisAsync("A", everything, token)));
        Assert.Equal(Capabilities.ScheduleExplore, await RefusedAsync(wire, [Capabilities.ScheduleExplore, Capabilities.Verification], model => model.ExploreAnalysisAsync("A", everything, token)));
        Assert.Equal(Capabilities.Verification, await RefusedAsync(wire, [Capabilities.Verification, Capabilities.Engines], model => model.ExploreAnalysisAsync("A", everything, token)));
        Assert.Equal(Capabilities.Engines, await RefusedAsync(wire, [Capabilities.Engines, Capabilities.StructuredValues], model => model.ExploreAnalysisAsync("A", everything, token)));
        Assert.Equal(Capabilities.StructuredValues, await RefusedAsync(wire, [Capabilities.StructuredValues, Capabilities.MetaobjectValues], model => model.ExploreAnalysisAsync("A", everything, token)));
        Assert.Equal(Capabilities.MetaobjectValues, await RefusedAsync(wire, [Capabilities.MetaobjectValues], model => model.ExploreAnalysisAsync("A", everything, token)));
        Assert.Equal(Capabilities.Schedule, await RefusedAsync(wire, [Capabilities.Schedule], model => model.RunAnalysisAsync("A", run, token)));
        Assert.Equal(Capabilities.Verification, await RefusedAsync(wire, [Capabilities.Verification], model => model.RunAnalysisAsync("A", cancellationToken: token)));
        Assert.Equal(Capabilities.ScheduleExplore, await RefusedAsync(wire, [Capabilities.ScheduleExplore], model => model.ExploreAnalysisAsync("A", new AnalysisOptions { Engine = "explore" }, token)));
        Assert.Equal(Capabilities.Verification, await RefusedAsync(wire, [Capabilities.Verification, Capabilities.Engines], model => model.CalcAsync("C", options: new CalcOptions { Engine = "run" }, cancellationToken: token)));
        Assert.Equal(Capabilities.Engines, await RefusedAsync(wire, [Capabilities.Engines, Capabilities.SetValues], model => model.CalcAsync("C", [new Value.SetValue([])], new CalcOptions { Engine = "run" }, token)));
        Assert.Equal(Capabilities.SetValues, await RefusedAsync(wire, [Capabilities.SetValues], model => model.CalcAsync("C", [new Value.SetValue([])], cancellationToken: token)));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task RunSweepSendsEveryRangeAndOptionAndReadsEveryRow(ClientWire wire)
    {
        using var handler = Serving(wire).On("RunSweep", Wire.RunSweepRequest.Parser, _ => CompleteVerificationWire.Sweep);
        await using var connection = await FakeConnection.OpenAsync(handler);
        var options = new SweepOptions
        {
            SubjectSymbolId = "Sw::barge",
            Arguments = [new Value.RealValue(1)],
            NamedArguments = new Dictionary<string, Value> { ["rate"] = new Value.IntegerValue(2) },
            Samples = 3,
            Seed = ulong.MaxValue,
            Engine = "sweep",
        };
        SweepRange[] ranges = [new("limit", new Value.RealValue(0), new Value.RealValue(40), new Value.RealValue(20)), new("b", new Value.IntegerValue(-1), new Value.IntegerValue(1))];

        var sweep = await connection.ModelByHash(Hash).RunSweepAsync("Sw::CostAnalysis", ranges, options, TestContext.Current.CancellationToken);

        Assert.Equal(
            new Wire.RunSweepRequest
            {
                ModelHash = Hash,
                SymbolId = "Sw::CostAnalysis",
                SubjectSymbolId = "Sw::barge",
                Arguments = { new Wire.Value { RealValue = 1 } },
                NamedArguments = { ["rate"] = new Wire.Value { IntValue = 2 } },
                Ranges =
                {
                    new Wire.SweepRange { Parameter = "limit", Start = new Wire.Value { RealValue = 0 }, End = new Wire.Value { RealValue = 40 }, Step = new Wire.Value { RealValue = 20 } },
                    new Wire.SweepRange { Parameter = "b", Start = new Wire.Value { IntValue = -1 }, End = new Wire.Value { IntValue = 1 } },
                },
                Samples = 3,
                Seed = ulong.MaxValue,
                Engine = "sweep",
            },
            Assert.Single(handler.RequestsFor("RunSweep")).Message(Wire.RunSweepRequest.Parser));
        Assert.Equal(["limit"], sweep.Parameters);
        Assert.True(sweep.Sampled);
        Assert.Equal(ulong.MaxValue, sweep.Seed);
        Assert.Equal(3, sweep.Rows.Count);
        var first = sweep.Rows[0];
        Assert.Equal([new CalcOutput("limit", new Value.RealValue(20))], first.Inputs);
        Assert.Equal([new CalcOutput("total", new Value.RealValue(37))], first.Outputs);
        Assert.Equal(("affordable", 220L, false), (Assert.Single(first.Verdicts).Element, first.ElapsedMicroseconds, first.Failed));
        Assert.True(Assert.Single(first.Evaluations).Selected);
        Assert.True(first.Holds);
        var failed = sweep.Rows[1];
        Assert.Equal(("division by zero", FailureReason.Evaluation, true, false), (failed.Error, failed.FailureReason, failed.Failed, failed.Holds));
        Assert.Equal(new Value.RealValue(3), Assert.Single(failed.Outputs).Value);
        Assert.Equal([failed], sweep.Failures);
        Assert.False(sweep.Holds);
        Assert.Equal(9, sweep.Resolve(new Value.InstanceRefValue(9))!.Id);
        Assert.Single(sweep.Diagnostics);
        Assert.Equal("sweep", sweep.Standing.Engine);
        Assert.False(new Sweep([], [], false, 0, [], [], Standing.None).Holds);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ASweepThatMadeNoRunIsAVerificationExceptionAndOneAnsweringRowsBesideIsMalformed(ClientWire wire)
    {
        using var handler = Serving(wire).On("RunSweep", Wire.RunSweepRequest.Parser, request => request.SymbolId == "rows"
            ? new Wire.RunSweepResponse { Error = "failed", Rows = { new Wire.SweepRow() } }
            : new Wire.RunSweepResponse
            {
                Error = "Sw::barge declares neither an analysis case nor a calc",
                FailureReason = Wire.FailureReason.WrongKind,
                Diagnostics = { CompleteWire.Diagnostic },
                Instances = { CompleteWire.Instance },
                Engine = "sweep",
                Bounds = { CompleteVerificationWire.Bounds },
            });
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash(Hash);
        var token = TestContext.Current.CancellationToken;

        var wrong = await Assert.ThrowsAsync<VerificationException>(() => model.RunSweepAsync("Sw::barge", [], cancellationToken: token));
        var malformed = await Assert.ThrowsAsync<TransportException>(() => model.RunSweepAsync("rows", [], cancellationToken: token));

        Assert.Equal((Model.RunSweepMethod, FailureReason.WrongKind, "sweep"), (wrong.Method, wrong.Reason, wrong.Standing.Engine));
        Assert.Equal(2, wrong.Standing.Bounds.Count);
        Assert.Single(wrong.Instances);
        Assert.Single(wrong.Diagnostics);
        Assert.Equal(StatusCode.Internal, malformed.StatusCode);
        Assert.Contains("beside rows", malformed.Message, StringComparison.Ordinal);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task SweepGatesAreCheckedInTheServicesOrderBeforeAnythingIsSent(ClientWire wire)
    {
        var token = TestContext.Current.CancellationToken;
        var options = new SweepOptions
        {
            Engine = "explore",
            Arguments = [new Value.ComplexValue(1, 1)],
            NamedArguments = new Dictionary<string, Value> { ["s"] = new Value.SetValue([]) },
        };
        SweepRange[] ranges = [new("x", new Value.IntegerValue(0), new Value.InfinityValue(), new Value.FunctionValue("F::step", null))];

        Assert.Equal(Capabilities.Verification, await RefusedAsync(wire, [Capabilities.Verification, Capabilities.Engines], model => model.RunSweepAsync("S", ranges, options, token)));
        Assert.Equal(Capabilities.Engines, await RefusedAsync(wire, [Capabilities.Engines, Capabilities.ScheduleExplore], model => model.RunSweepAsync("S", ranges, options, token)));
        Assert.Equal(Capabilities.ScheduleExplore, await RefusedAsync(wire, [Capabilities.ScheduleExplore, Capabilities.ComplexValues], model => model.RunSweepAsync("S", ranges, options, token)));
        Assert.Equal(Capabilities.ComplexValues, await RefusedAsync(wire, [Capabilities.ComplexValues, Capabilities.SetValues], model => model.RunSweepAsync("S", ranges, options, token)));
        Assert.Equal(Capabilities.SetValues, await RefusedAsync(wire, [Capabilities.SetValues, Capabilities.InfinityValue], model => model.RunSweepAsync("S", ranges, options, token)));
        Assert.Equal(Capabilities.InfinityValue, await RefusedAsync(wire, [Capabilities.InfinityValue, Capabilities.FunctionValues], model => model.RunSweepAsync("S", ranges, options, token)));
        Assert.Equal(Capabilities.FunctionValues, await RefusedAsync(wire, [Capabilities.FunctionValues], model => model.RunSweepAsync("S", ranges, options, token)));

        using var handler = Serving(wire);
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash(Hash);
        var step = await Assert.ThrowsAsync<InvalidRequestException>(() =>
            model.RunSweepAsync("S", [new SweepRange("x", new Value.IntegerValue(0), new Value.IntegerValue(1), new Value.UnsetValue())], cancellationToken: token));
        await Assert.ThrowsAsync<InvalidRequestException>(() => model.RunSweepAsync("S", [], new SweepOptions { Engine = string.Empty }, token));
        await Assert.ThrowsAsync<InvalidRequestException>(() => model.RunSweepAsync("S", [], new SweepOptions { SubjectSymbolId = string.Empty }, token));
        await Assert.ThrowsAsync<ArgumentException>(() => model.RunSweepAsync("S", [null!], cancellationToken: token));
        await Assert.ThrowsAsync<ArgumentNullException>(() => model.RunSweepAsync("S", null!, cancellationToken: token));
        await Assert.ThrowsAsync<ArgumentNullException>(() => model.RunSweepAsync(null!, [], cancellationToken: token));
        Assert.Contains("ranges[0].step", step.Message, StringComparison.Ordinal);
        Assert.Equal(["GetServerInfo"], handler.Requests.Select(request => request.Method));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ListEnginesSendsTheProbeAndReadsEveryFieldOfEveryEngine(ClientWire wire)
    {
        using var handler = Serving(wire).On("ListEngines", Wire.ListEnginesRequest.Parser, _ => CompleteVerificationWire.Engines);
        await using var connection = await FakeConnection.OpenAsync(handler);
        var token = TestContext.Current.CancellationToken;

        var listed = await connection.ListEnginesAsync(cancellationToken: token);
        await connection.ListEnginesAsync(new ListEnginesOptions { Probe = true }, token);

        Assert.Equal(
            [new Wire.ListEnginesRequest(), new Wire.ListEnginesRequest { Probe = true }],
            handler.RequestsFor("ListEngines").Select(sent => sent.Message(Wire.ListEnginesRequest.Parser)));
        Assert.Equal(new EngineInfo("solve", "proved", ["holds", "satisfiable"], ["solver"], "z3", "/usr/bin/z3", true, string.Empty, "built-in", "-", string.Empty, string.Empty, string.Empty, true), listed[0]);
        Assert.Equal(
            new EngineInfo(
                "ext", "observed", ["compute"], [], "ext-engine", string.Empty, false, "not served: start the service with -serve-external-engines ext",
                "engine", "stdio/jsonrpc", "/engines/ext.json", "/opt/ext/bin/ext-engine", "0.3", false),
            listed[1]);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ListingEnginesNeedsTheEnginesCapability(ClientWire wire)
    {
        using var handler = new FakeServiceHandler(wire) { Capabilities = [.. FakeModels.ModelCapabilities.Except([Capabilities.Engines])] };
        await using var connection = await FakeConnection.OpenAsync(handler);

        var refused = await Assert.ThrowsAsync<CapabilityException>(() => connection.ListEnginesAsync(cancellationToken: TestContext.Current.CancellationToken));

        Assert.Equal(Capabilities.Engines, refused.Capability);
        Assert.Equal(["GetServerInfo"], handler.Requests.Select(request => request.Method));
    }

    [Fact]
    public void TheOptionsCopyTheirArgumentsAndTheRecordsRefuseANull()
    {
        var positional = new List<Value> { new Value.IntegerValue(1) };
        var analysis = new AnalysisOptions { Arguments = positional };
        var sweep = new SweepOptions { Arguments = positional, NamedArguments = new Dictionary<string, Value> { ["x"] = new Value.IntegerValue(2) } };

        positional.Clear();

        Assert.Single(analysis.Arguments);
        Assert.Single(sweep.Arguments);
        Assert.Empty(AnalysisOptions.Default.NamedArguments);
        Assert.Empty(SweepOptions.Default.Arguments);
        Assert.False(ListEnginesOptions.Default.Probe);
        Assert.Null(CalcOptions.Default.Engine);
        Assert.Null(ValidateOptions.Default.Engine);
        Assert.Throws<ArgumentNullException>(() => new CalcOutput("x", null!));
        Assert.Throws<ArgumentNullException>(() => new SweepRange("x", new Value.IntegerValue(0), null!));
        Assert.Throws<ArgumentNullException>(() => new CaseEvaluation("f", [], null, null!, false, false));
        Assert.Throws<ArgumentNullException>(() => new Analysis([], [], [], [], [], [], null!, null));
        Assert.Throws<ArgumentNullException>(() => new SweepRow([], [], [], [], 0, null!, FailureReason.Unspecified));
        Assert.Throws<ArgumentNullException>(() => new EngineInfo("e", "observed", [], [], "", "", true, "", "built-in", "-", "", "", null!, true));
        Assert.Throws<ArgumentNullException>(() => new ToolDryRunFile("f", null!, ""));
        Assert.Throws<ArgumentNullException>(() => new AnalysisException("m", Model.RunAnalysisMethod, FailureReason.Evaluation, null!));
        Assert.Throws<ArgumentException>(() => new Exploration([], true, 0, [null!], 0, 0, false, 0, [], Standing.None));
        Assert.Throws<ArgumentNullException>(() => new Exploration([], true, 0, [], 0, 0, false, 0, [], null!));
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
