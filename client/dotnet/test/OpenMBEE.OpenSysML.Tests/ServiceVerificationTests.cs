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
/// Verifying, validating, calculating, analyzing, sweeping and listing engines against a real sysml-grpc over every
/// client wire, on the conformance fixtures: verdicts that hold, fail and decide nothing, the verification cases
/// reported beside them, calculations with structured arguments, analyses run, failed and explored, sweeps whose
/// failed row stays a row, and, against a service withholding the capabilities these calls gate on, refusals made
/// before anything is sent.
/// </summary>
[Trait(Category.Name, Category.Integration)]
public sealed class ServiceVerificationTests(ServiceFixture services) : IClassFixture<ServiceFixture>
{
    private const string Withheld = "verification,verification_questions,verification_arguments,engines";

    public static TheoryData<ClientWire> Wires => ClientWires.All;

    private static CancellationToken Token => TestContext.Current.CancellationToken;

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ConstraintsAndRequirementsAnswerHoldingFalseAndUndecidedVerdicts(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var model = await LoadAsync(connection, "verification.sysml");
        var questions = await LoadAsync(connection, "verification_questions.sysml");
        var cases = await LoadAsync(connection, "verification_cases.sysml");

        var holds = await model.VerifyConstraintAsync("Demo::Vehicle::massPositive", cancellationToken: Token);
        var light = await model.VerifyConstraintAsync("Demo::Vehicle::massLight", cancellationToken: Token);
        var subject = await model.VerifyConstraintAsync("Demo::Vehicle::massPositive", new VerifyOptions { SubjectSymbolId = "Demo::sedan" }, Token);
        var wrong = await model.VerifyConstraintAsync("Demo::sedan", cancellationToken: Token);
        var evaluate = await model.VerifyConstraintAsync("Demo::Vehicle::massPositive", new VerifyOptions { Question = VerifyOptions.Evaluate }, Token);
        var undecided = await questions.VerifyConstraintAsync("Demo::sq", new VerifyOptions { Question = VerifyOptions.Holds }, Token);
        var tiny = await model.VerifyRequirementAsync("Demo::Vehicle::tiny", cancellationToken: Token);
        var bounded = await cases.VerifyRequirementAsync("Demo::bounded", cancellationToken: Token);

        Assert.True(holds.Holds);
        Assert.Equal(("constraint", "Demo::Vehicle::massPositive"), (holds.Verdict.Kind, holds.Verdict.ElementId));
        Assert.True(light.Verdict.Violated);
        Assert.NotEmpty(light.Verdict.Condition);
        Assert.True(subject.Holds);
        Assert.NotNull(subject.Subject);
        Assert.Equal(subject.Verdict.InstanceTypeId, subject.Subject!.TypeSymbolId);
        Assert.Equal(FailureReason.WrongKind, wrong.Verdict.FailureReason);
        Assert.False(wrong.Verdict.Decided);
        Assert.Equal(("evaluate", "holds"), (evaluate.Verdict.Question, evaluate.Verdict.Status));
        Assert.Equal(("holds", "undecided", FailureReason.Undecided), (undecided.Verdict.Question, undecided.Verdict.Status, undecided.Verdict.FailureReason));
        Assert.NotEmpty(undecided.Verdict.Error);
        Assert.Equal("requirement", tiny.Verdict.Kind);
        Assert.True(tiny.Verdict.Violated);
        Assert.True(bounded.Holds);
        var verified = Assert.Single(bounded.Verifications);
        Assert.Equal(("Demo::checkBound", "fail", "Demo::bounded"), (verified.CaseId, verified.Kind, verified.RequirementId));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ASolversAnswerCarriesTheAssumptionItRestsOnAndAnEvaluationCarriesNone(ClientWire wire)
    {
        const string Tanks = """
            package Tanks {
                private import ScalarValues::Real;
                class Tank {
                    feature level : Real;
                    inv nonNegative { level >= 0.0 }
                }
            }
            """;
        await using var connection = await OpenAsync(wire);
        await SolverGate.FromProcess.RequireAsync(connection, Token);
        var model = (await connection.LoadContentAsync(Tanks, new ParseOptions { Language = SourceLanguage.KerML }, Token)).RequireNoErrors();

        var held = await model.VerifyConstraintAsync("Tanks::Tank::nonNegative", new VerifyOptions { Question = VerifyOptions.Holds }, Token);
        var evaluated = await model.VerifyConstraintAsync("Tanks::Tank::nonNegative", cancellationToken: Token);

        Assert.Equal("violated", held.Verdict.Status);
        Assert.Equal(
            [new VerdictNote(VerdictNote.AssumedOne, "Tanks::Tank::level", "Assuming Tanks::Tank::level holds one value: it declares no multiplicity, so the default [0..*] governs it")],
            held.Verdict.Notes);
        Assert.Empty(evaluated.Verdict.Notes);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task SatisfactionsAndValidationsAnswerEveryAssertionWithTheirVerificationCases(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var model = await LoadAsync(connection, "verification.sysml");
        var cases = await LoadAsync(connection, "verification_cases.sysml");

        var every = await model.VerifySatisfactionAsync(cancellationToken: Token);
        var scoped = await model.VerifySatisfactionAsync("Demo::analysis", cancellationToken: Token);
        var satisfied = await model.SatisfiedAsync(cancellationToken: Token);
        var checks = await cases.VerifySatisfactionAsync("Demo::checks", cancellationToken: Token);
        var sedan = await model.ValidateInstanceAsync("Demo::sedan", cancellationToken: Token);
        var good = await cases.ValidateInstanceAsync("Demo::good", cancellationToken: Token);

        Assert.Equal([true, false], every.Verdicts.Select(verdict => verdict.Holds));
        Assert.All(every.Verdicts, verdict => Assert.Equal("satisfy", verdict.Kind));
        Assert.False(every.Holds);
        Assert.False(satisfied);
        Assert.Equal(2, scoped.Verdicts.Count);
        Assert.Equal(["Demo::zeroed", "Demo::bounded"], checks.Verifications.Select(verdict => verdict.RequirementId));
        Assert.Equal(
            [("requirement", "Demo::Vehicle::lightEnough", true), ("requirement", "Demo::Vehicle::tiny", false)],
            sedan.Verdicts.Take(2).Select(verdict => (verdict.Kind, verdict.ElementId, verdict.Holds)));
        Assert.Equal(["Demo::massLimit", "Demo::massTiny"], sedan.Verdicts.Skip(2).Select(verdict => verdict.RequirementId));
        Assert.Equal(("object", "Demo::sedan"), (sedan.Summary.Kind, sedan.Summary.ElementId));
        Assert.True(sedan.Summary.Violated);
        Assert.False(sedan.Bounded);
        Assert.Equal(sedan.Summary.InstanceId, sedan.Root!.Id);
        Assert.True(good.Holds);
        Assert.Equal(["Demo::checkZero", "Demo::checkBound"], good.Verifications.Select(verdict => verdict.CaseId));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AVerificationTheServiceCannotAnswerIsInBandAndAnUnknownQuestionOrEngineIsRefused(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var model = await LoadAsync(connection, "verification.sysml");

        var unknown = await Assert.ThrowsAsync<VerificationException>(() => model.VerifyConstraintAsync("Demo::nosuch", cancellationToken: Token));
        var noObject = await Assert.ThrowsAsync<VerificationException>(() => model.ValidateInstanceAsync("Demo", cancellationToken: Token));
        var missing = await Assert.ThrowsAsync<VerificationException>(() => model.ValidateInstanceAsync("Demo::nosuch", cancellationToken: Token));
        var question = await Assert.ThrowsAsync<ServiceException>(() => model.VerifyConstraintAsync("Demo::Vehicle::massPositive", new VerifyOptions { Question = "bogus" }, Token));
        var engine = await Assert.ThrowsAsync<ServiceException>(() => model.VerifyRequirementAsync("Demo::Vehicle::tiny", new VerifyOptions { Engine = "no-such-engine" }, Token));
        var gone = await Assert.ThrowsAsync<ModelNotFoundException>(() => connection.ModelByHash("0000").VerifySatisfactionAsync(cancellationToken: Token));

        Assert.Contains("Demo::nosuch", unknown.Message, StringComparison.Ordinal);
        Assert.Equal(Model.VerifyConstraintMethod, unknown.Method);
        Assert.Equal(FailureReason.WrongKind, noObject.Reason);
        Assert.NotEmpty(missing.Message);
        Assert.All([question, engine], refused => Assert.Equal(StatusCode.InvalidArgument, refused.StatusCode));
        Assert.Contains("bogus", question.ServiceMessage, StringComparison.Ordinal);
        Assert.Equal(StatusCode.NotFound, gone.StatusCode);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ACalculationTakesStructuredArgumentsAndFailsInBand(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var verification = await LoadAsync(connection, "verification.sysml");
        var structured = await LoadAsync(connection, "structured.sysml");
        var function = await LoadAsync(connection, "function.sysml");
        var setTensor = await LoadAsync(connection, "set_tensor.sysml");

        var added = await verification.CalcAsync("Demo::add", [new Value.IntegerValue(2), new Value.IntegerValue(3)], cancellationToken: Token);
        var length = await structured.CalcAsync("S::length", [new Value.VectorValue([new Value.RealValue(3), new Value.RealValue(4)])], cancellationToken: Token);
        var applied = await function.CalcAsync("F::apply", [new Value.FunctionValue("F::Cube", null), new Value.RealValue(2)], cancellationToken: Token);
        var members = await setTensor.CalcAsync("T::members", [new Value.SetValue([new Value.IntegerValue(7), new Value.IntegerValue(5), new Value.IntegerValue(9)])], cancellationToken: Token);
        var wrong = await Assert.ThrowsAsync<VerificationException>(() => verification.CalcAsync("Demo::sedan", cancellationToken: Token));
        var notCalc = await Assert.ThrowsAsync<VerificationException>(() =>
            function.CalcAsync("F::apply", [new Value.FunctionValue("F::holder", null), new Value.RealValue(2)], cancellationToken: Token));

        Assert.Equal(Held(new Value.IntegerValue(5)), added.Value);
        Assert.Equal(Held(new Value.RealValue(5)), length.Result);
        Assert.Equal(Held(new Value.RealValue(8)), applied.Result);
        Assert.Equal(Held(new Value.IntegerValue(3)), members.Result);
        Assert.Empty(added.Outputs);
        Assert.Equal((Model.EvaluateCalcMethod, FailureReason.WrongKind), (wrong.Method, wrong.Reason));
        Assert.Equal(FailureReason.Evaluation, notCalc.Reason);
        Assert.Contains("F::holder is not a calc", notCalc.Message, StringComparison.Ordinal);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AnAnalysisRunsOnItsSubjectWithItsArgumentsIsExploredAndKeepsWhatAFailedRunComputed(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var model = await LoadAsync(connection, "analysis.sysml");

        var ship = await model.RunAnalysisAsync("An::shipCost", cancellationToken: Token);
        var plain = await model.RunAnalysisAsync("An::plain", cancellationToken: Token);
        var barge = await model.RunAnalysisAsync("An::CostAnalysis", new AnalysisOptions { SubjectSymbolId = "An::barge" }, Token);
        var named = await model.RunAnalysisAsync(
            "An::CostAnalysis", new AnalysisOptions { SubjectSymbolId = "An::barge", NamedArguments = new Dictionary<string, Value> { ["limit"] = new Value.RealValue(50) } }, Token);
        var positional = await model.RunAnalysisAsync("An::CostAnalysis", new AnalysisOptions { SubjectSymbolId = "An::barge", Arguments = [new Value.RealValue(10)] }, Token);
        var explored = await model.ExploreAnalysisAsync("An::shipCost", cancellationToken: Token);
        var noSubject = await Assert.ThrowsAsync<AnalysisException>(() => model.RunAnalysisAsync("An::CostAnalysis", cancellationToken: Token));
        var wrong = await Assert.ThrowsAsync<AnalysisException>(() => model.RunAnalysisAsync("An::ship", cancellationToken: Token));
        var schedule = await Assert.ThrowsAsync<ServiceException>(() => model.RunAnalysisAsync("An::plain", new AnalysisOptions { Schedule = "seed:abc" }, Token));

        Assert.Equal([new CalcOutput("total", Held(new Value.RealValue(12)))], ship.Outputs);
        var objective = ship.Objective!;
        Assert.Equal(("objective", "An::CostAnalysis::affordable", "affordable", true, "An::ship"), (objective.Kind, objective.ElementId, objective.Element, objective.Holds, objective.InstanceTypeId));
        Assert.True(ship.Holds);
        Assert.NotEmpty(ship.Instances);
        Assert.Null(ship.ToolDryRun);
        Assert.Equal([new CalcOutput("x", Held(new Value.RealValue(3)))], plain.Outputs);
        Assert.Empty(plain.Verdicts);
        Assert.Equal(Held(new Value.RealValue(37)), barge.Outputs[0].Value);
        Assert.True(barge.Objective!.Violated);
        Assert.True(named.Holds);
        Assert.False(positional.Holds);
        Assert.True(explored.Complete);
        Assert.Equal(Held(new Value.RealValue(12)), Assert.Single(explored.Outcomes).Outputs["total"]);
        Assert.Equal(FailureReason.Evaluation, noSubject.Reason);
        Assert.Contains("subject", noSubject.Message, StringComparison.Ordinal);
        Assert.Equal(FailureReason.WrongKind, wrong.Reason);
        Assert.Empty(wrong.Partial.Outputs);
        Assert.Equal(StatusCode.InvalidArgument, schedule.StatusCode);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ASweepTablesEveryRowKeepsAFailedRowAndDrawsReproducibly(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var model = await LoadAsync(connection, "sweep.sysml");
        var one = new SweepOptions { Arguments = [new Value.RealValue(1)] };

        var stepped = await model.RunSweepAsync("Sw::Sum", [new SweepRange("b", new Value.RealValue(0), new Value.RealValue(4), new Value.RealValue(2))], one, Token);
        var analysis = await model.RunSweepAsync(
            "Sw::CostAnalysis", [new SweepRange("limit", new Value.RealValue(20), new Value.RealValue(40), new Value.RealValue(20))], new SweepOptions { SubjectSymbolId = "Sw::barge" }, Token);
        var ratio = await model.RunSweepAsync("Sw::Ratio", [new SweepRange("b", new Value.IntegerValue(-1), new Value.IntegerValue(1))], new SweepOptions { Arguments = [new Value.RealValue(4)] }, Token);
        var sampled = new SweepOptions { Arguments = [new Value.RealValue(1)], Samples = 3, Seed = 42 };
        SweepRange[] drawn = [new("b", new Value.RealValue(0), new Value.RealValue(10))];
        var first = await model.RunSweepAsync("Sw::Sum", drawn, sampled, Token);
        var again = await model.RunSweepAsync("Sw::Sum", drawn, sampled, Token);
        var missing = await Assert.ThrowsAsync<VerificationException>(() =>
            model.RunSweepAsync("Sw::Sum", [new SweepRange("missing", new Value.RealValue(0), new Value.RealValue(4), new Value.RealValue(2))], one, Token));
        var none = await Assert.ThrowsAsync<VerificationException>(() => model.RunSweepAsync("Sw::Sum", [], new SweepOptions { Arguments = [new Value.RealValue(1), new Value.RealValue(2)] }, Token));
        var wrong = await Assert.ThrowsAsync<VerificationException>(() =>
            model.RunSweepAsync("Sw::barge", [new SweepRange("limit", new Value.RealValue(0), new Value.RealValue(4), new Value.RealValue(2))], cancellationToken: Token));

        Assert.Equal(["b"], stepped.Parameters);
        Assert.False(stepped.Sampled);
        Assert.Equal([Held(new Value.RealValue(1)), Held(new Value.RealValue(3)), Held(new Value.RealValue(5))], stepped.Rows.Select(row => Assert.Single(row.Outputs).Value));
        Assert.Equal([0.0, 2.0, 4.0], stepped.Rows.Select(row => ((Value.RealValue)Assert.Single(row.Inputs).Value).Value));
        Assert.All(stepped.Rows, row => Assert.True(row.ElapsedMicroseconds >= 0));
        Assert.Equal([false, true], analysis.Rows.Select(row => Assert.Single(row.Verdicts).Holds));
        Assert.Equal("Sw::barge", analysis.Rows[0].Verdicts[0].InstanceTypeId);
        Assert.Equal([false, true, false], ratio.Rows.Select(row => row.Failed));
        Assert.Equal(FailureReason.Evaluation, ratio.Rows[1].FailureReason);
        Assert.Contains("division by zero", ratio.Rows[1].Error, StringComparison.Ordinal);
        Assert.Equal([ratio.Rows[1]], ratio.Failures);
        Assert.True(first.Sampled);
        Assert.Equal(42UL, first.Seed);
        Assert.Equal(3, first.Rows.Count);
        Assert.Equal(first.Rows.Select(row => row.Inputs), again.Rows.Select(row => row.Inputs));
        Assert.NotEmpty(missing.Message);
        Assert.NotEmpty(none.Message);
        Assert.Equal(FailureReason.WrongKind, wrong.Reason);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task TheEnginesAreListedInNameOrderWithAndWithoutAProbe(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);

        var listed = await connection.ListEnginesAsync(cancellationToken: Token);
        var probed = await connection.ListEnginesAsync(new ListEnginesOptions { Probe = true }, Token);

        Assert.Superset(new HashSet<string> { "explore", "run", "solve", "sweep" }, listed.Select(engine => engine.Name).ToHashSet());
        Assert.Equal(listed.Select(engine => engine.Name).Order(StringComparer.Ordinal), listed.Select(engine => engine.Name));
        Assert.All(listed, engine => Assert.NotEmpty(engine.Authority));
        Assert.Contains(listed, engine => engine.Answers.Contains("satisfiable"));
        Assert.All(listed.Where(engine => engine.Kind == "built-in"), engine => Assert.True(engine.Served));
        Assert.Equal(listed.Select(engine => engine.Name), probed.Select(engine => engine.Name));
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
            await Assert.ThrowsAsync<CapabilityException>(() => model.VerifyConstraintAsync("Demo::Vehicle::massPositive", cancellationToken: Token)),
            await Assert.ThrowsAsync<CapabilityException>(() => model.VerifySatisfactionAsync(cancellationToken: Token)),
            await Assert.ThrowsAsync<CapabilityException>(() => model.ValidateInstanceAsync("Demo::sedan", cancellationToken: Token)),
            await Assert.ThrowsAsync<CapabilityException>(() => model.CalcAsync("Demo::add", cancellationToken: Token)),
            await Assert.ThrowsAsync<CapabilityException>(() => model.RunAnalysisAsync("An::plain", cancellationToken: Token)),
            await Assert.ThrowsAsync<CapabilityException>(() => model.RunSweepAsync("Sw::Sum", [], cancellationToken: Token)),
            await Assert.ThrowsAsync<CapabilityException>(() => connection.ListEnginesAsync(cancellationToken: Token)),
        };

        Assert.Equal([.. Enumerable.Repeat(Capabilities.Verification, 6), Capabilities.Engines], refused.Select(error => error.Capability));
        Assert.Equal(["GetServerInfo"], counting.Methods);
    }

    private static string Fixture(string name) => Repository.PathOf("conformance", "fixtures", name);

    // A result or out parameter stating no multiplicity is [0..*], so the service holds one value as a sequence.
    private static Value.SequenceValue Held(Value value) => new([value]);

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
