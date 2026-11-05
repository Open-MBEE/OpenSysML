using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading.Tasks;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// VerifyConstraint, VerifyRequirement, VerifySatisfaction and ValidateInstance through the typed API, over every
/// wire: the exact request each option makes, every field of a verdict read, a false verdict and an undecided one
/// answered rather than raised, a request the service could not answer raised as a <see cref="VerificationException"/>
/// keeping what the answer reported, a malformed answer refused, and the gates checked in the service's order before
/// anything is sent.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed class ModelVerificationTests
{
    private const string Hash = "h";

    public static TheoryData<ClientWire> Wires => ClientWires.All;

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task VerifyConstraintSendsEveryOptionAndReadsEveryFieldOfTheVerdict(ClientWire wire)
    {
        using var handler = Serving(wire).On("VerifyConstraint", Wire.VerifyConstraintRequest.Parser, _ => CompleteVerificationWire.Constraint);
        await using var connection = await FakeConnection.OpenAsync(handler);
        var options = new VerifyOptions
        {
            Question = VerifyOptions.Evaluate,
            SubjectSymbolId = "Demo::sedan",
            Arguments = [new Value.RealValue(2.5), new Value.IntegerValue(3)],
            NamedArguments = new Dictionary<string, Value> { ["limit"] = new Value.RealValue(1000), ["count"] = new Value.IntegerValue(2) },
            Engine = "run",
        };

        var verification = await connection.ModelByHash(Hash).VerifyConstraintAsync("Demo::Vehicle::massLight", options, TestContext.Current.CancellationToken);

        Assert.Equal(
            new Wire.VerifyConstraintRequest
            {
                ModelHash = Hash,
                SymbolId = "Demo::Vehicle::massLight",
                SubjectSymbolId = "Demo::sedan",
                Engine = "run",
                Question = "evaluate",
                Arguments = { new Wire.Value { RealValue = 2.5 }, new Wire.Value { IntValue = 3 } },
                NamedArguments = { ["limit"] = new Wire.Value { RealValue = 1000 }, ["count"] = new Wire.Value { IntValue = 2 } },
            },
            Assert.Single(handler.RequestsFor("VerifyConstraint")).Message(Wire.VerifyConstraintRequest.Parser));
        var verdict = verification.Verdict;
        Assert.Equal(
            ("constraint", "Demo::Vehicle::massLight", "Demo::Vehicle::massLight", false, "mass < 1000 [kg]", 9L, "Demo::Vehicle"),
            (verdict.Kind, verdict.ElementId, verdict.Element, verdict.Holds, verdict.Condition, verdict.InstanceId, verdict.InstanceTypeId));
        Assert.Equal(("", FailureReason.Unspecified, "Demo::massLimit", "engine", "holds", "violated"), (verdict.Error, verdict.FailureReason, verdict.RequirementId, verdict.InstancePath, verdict.Question, verdict.Status));
        Assert.Equal(new Standing("solve", "witnessed", [new Bound("solver", 5000, true), new Bound("depth", 64, false)]), verdict.Standing);
        Assert.Equal(
            [new WitnessAssignment("Demo::Vehicle::mass", new Value.RealValue(1500.5), "kg", "3001/2"), new WitnessAssignment("Demo::Vehicle::gear", null, "", "Gear::low")],
            verdict.Witness);
        Assert.Equal(
            [
                new VerdictNote(VerdictNote.Uncovered, "engine", "Not in the query: what engine asserts about its own objects, as it may hold more than one object; instantiate and validate an object to check it"),
                new VerdictNote(VerdictNote.AssumedOne, "Demo::Vehicle::gear", "Assuming Demo::Vehicle::gear holds one value: it declares no multiplicity, so the default [0..*] governs it"),
            ],
            verdict.Notes);
        Assert.Equal(verdict.Notes[1].Text, verdict.Notes[1].ToString());
        Assert.True(verdict.Decided);
        Assert.True(verdict.Violated);
        Assert.False(verification.Holds);
        Assert.Equal(9, verification.Subject!.Id);
        Assert.Null(verification.FindInstance(0));
        Assert.Null(verification.FindInstance(404));
        Assert.Equal("rounded", Assert.Single(verification.Diagnostics).Message);
        Assert.Empty(verification.Verifications);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task VerifyRequirementReadsTheVerificationVerdictsBesideItsVerdict(ClientWire wire)
    {
        using var handler = Serving(wire).On("VerifyRequirement", Wire.VerifyRequirementRequest.Parser, _ => CompleteVerificationWire.Requirement);
        await using var connection = await FakeConnection.OpenAsync(handler);

        var verification = await connection.ModelByHash(Hash).VerifyRequirementAsync(
            "Demo::bounded", new VerifyOptions { Question = VerifyOptions.Holds }, TestContext.Current.CancellationToken);

        Assert.Equal(
            new Wire.VerifyRequirementRequest { ModelHash = Hash, SymbolId = "Demo::bounded", Question = "holds" },
            Assert.Single(handler.RequestsFor("VerifyRequirement")).Message(Wire.VerifyRequirementRequest.Parser));
        var case_ = Assert.Single(verification.Verifications);
        Assert.Equal(new VerificationVerdict("Demo::checkBound", "inconclusive", "the bound was not reached", true, "Demo::bounded", 9, "Demo::Vehicle"), case_);
        Assert.False(case_.Passed);
        Assert.Equal("Demo::Vehicle::massLight", verification.Verdict.ElementId);
        Assert.Single(verification.Instances);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AFalseVerdictIsDecidedAndAnUndecidedOneIsAnsweredNeitherRaised(ClientWire wire)
    {
        var holding = new Wire.VerifyConstraintResponse { Verdict = new Wire.Verdict { Kind = "constraint", Holds = true, Question = "evaluate", Status = "holds" } };
        var wrongKind = new Wire.VerifyConstraintResponse
        {
            Verdict = new Wire.Verdict { Kind = "constraint", Element = "Demo::sedan", Error = "Demo::sedan is not a constraint", FailureReason = Wire.FailureReason.WrongKind },
        };
        var answers = new Queue<Wire.VerifyConstraintResponse>([holding, new Wire.VerifyConstraintResponse { Verdict = CompleteVerificationWire.Undecided }, wrongKind]);
        using var handler = Serving(wire).On("VerifyConstraint", Wire.VerifyConstraintRequest.Parser, _ => answers.Dequeue());
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash(Hash);
        var token = TestContext.Current.CancellationToken;

        var holds = await model.VerifyConstraintAsync("Demo::Vehicle::massPositive", cancellationToken: token);
        var undecided = await model.VerifyConstraintAsync("Demo::sq", new VerifyOptions { Question = VerifyOptions.Holds }, token);
        var wrong = await model.VerifyConstraintAsync("Demo::sedan", cancellationToken: token);

        Assert.True(holds.Holds);
        Assert.True(holds.Verdict.Decided);
        Assert.False(holds.Verdict.Violated);
        Assert.False(holds.Verdict.Standing.IsReported);
        Assert.Same(Standing.None, holds.Verdict.Standing);
        Assert.Null(holds.Subject);
        Assert.False(undecided.Holds);
        Assert.False(undecided.Verdict.Decided);
        Assert.False(undecided.Verdict.Violated);
        Assert.Equal((FailureReason.Undecided, "undecided"), (undecided.Verdict.FailureReason, undecided.Verdict.Status));
        Assert.Equal(FailureReason.WrongKind, wrong.Verdict.FailureReason);
        Assert.False(wrong.Verdict.Decided);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AVerificationTheServiceCouldNotAnswerIsAVerificationExceptionKeepingWhatItReported(ClientWire wire)
    {
        using var handler = Serving(wire)
            .On("VerifyConstraint", Wire.VerifyConstraintRequest.Parser, _ => new Wire.VerifyConstraintResponse { Error = "symbol not found: Demo::nope", Diagnostics = { CompleteWire.Diagnostic } })
            .On("VerifyRequirement", Wire.VerifyRequirementRequest.Parser, _ => new Wire.VerifyRequirementResponse
            {
                Error = "subject not found: Demo::nobody",
                Instances = { CompleteWire.Instance },
                VerificationVerdicts = { CompleteVerificationWire.Verification },
            })
            .On("VerifySatisfaction", Wire.VerifySatisfactionRequest.Parser, _ => new Wire.VerifySatisfactionResponse
            {
                Error = "Demo states no satisfaction assertion",
                FailureReason = Wire.FailureReason.WrongKind,
                VerificationVerdicts = { CompleteVerificationWire.Verification },
            })
            .On("ValidateInstance", Wire.ValidateInstanceRequest.Parser, _ => new Wire.ValidateInstanceResponse
            {
                Error = "Demo declares no object",
                FailureReason = Wire.FailureReason.WrongKind,
                Instances = { CompleteWire.Instance },
                Diagnostics = { CompleteWire.Diagnostic },
            });
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash(Hash);
        var token = TestContext.Current.CancellationToken;

        var constraint = await Assert.ThrowsAsync<VerificationException>(() => model.VerifyConstraintAsync("Demo::nope", cancellationToken: token));
        var requirement = await Assert.ThrowsAsync<VerificationException>(() => model.VerifyRequirementAsync("Demo::r", new VerifyOptions { SubjectSymbolId = "Demo::nobody" }, token));
        var satisfaction = await Assert.ThrowsAsync<VerificationException>(() => model.VerifySatisfactionAsync("Demo", cancellationToken: token));
        var validation = await Assert.ThrowsAsync<VerificationException>(() => model.ValidateInstanceAsync("Demo", cancellationToken: token));

        Assert.Equal(("symbol not found: Demo::nope", Model.VerifyConstraintMethod, FailureReason.Unspecified), (constraint.Message, constraint.Method, constraint.Reason));
        Assert.Equal("rounded", Assert.Single(constraint.Diagnostics).Message);
        Assert.Empty(constraint.Instances);
        Assert.Same(Standing.None, constraint.Standing);
        Assert.Equal(9, Assert.Single(requirement.Instances).Id);
        Assert.Equal("Demo::checkBound", Assert.Single(requirement.Verifications).CaseId);
        Assert.Equal((Model.VerifySatisfactionMethod, FailureReason.WrongKind), (satisfaction.Method, satisfaction.Reason));
        Assert.Single(satisfaction.Verifications);
        Assert.Equal((Model.ValidateInstanceMethod, FailureReason.WrongKind), (validation.Method, validation.Reason));
        Assert.Single(validation.Instances);
        Assert.Single(validation.Diagnostics);
        Assert.All<ModelException>([constraint, requirement, satisfaction, validation], failure => Assert.IsNotType<AnalysisException>(failure));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AnAnswerOfNeitherVerdictNorFailureOrOfBothIsMalformed(ClientWire wire)
    {
        using var handler = Serving(wire)
            .On("VerifyConstraint", Wire.VerifyConstraintRequest.Parser, request => request.SymbolId == "both"
                ? new Wire.VerifyConstraintResponse { Verdict = CompleteVerificationWire.Verdict, Error = "failed" }
                : new Wire.VerifyConstraintResponse())
            .On("VerifyRequirement", Wire.VerifyRequirementRequest.Parser, _ => new Wire.VerifyRequirementResponse())
            .On("VerifySatisfaction", Wire.VerifySatisfactionRequest.Parser, _ => new Wire.VerifySatisfactionResponse { Verdicts = { CompleteVerificationWire.Verdict }, Error = "failed" })
            .On("ValidateInstance", Wire.ValidateInstanceRequest.Parser, request => request.SymbolId switch
            {
                "none" => new Wire.ValidateInstanceResponse { Verdicts = { CompleteVerificationWire.Verdict } },
                "summary" => new Wire.ValidateInstanceResponse { Error = "failed", Summary = CompleteVerificationWire.Verdict },
                _ => new Wire.ValidateInstanceResponse { Error = "failed", Bounded = true },
            });
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash(Hash);
        var token = TestContext.Current.CancellationToken;

        TransportException[] malformed =
        [
            await Assert.ThrowsAsync<TransportException>(() => model.VerifyConstraintAsync("neither", cancellationToken: token)),
            await Assert.ThrowsAsync<TransportException>(() => model.VerifyConstraintAsync("both", cancellationToken: token)),
            await Assert.ThrowsAsync<TransportException>(() => model.VerifyRequirementAsync("neither", cancellationToken: token)),
            await Assert.ThrowsAsync<TransportException>(() => model.VerifySatisfactionAsync(cancellationToken: token)),
            await Assert.ThrowsAsync<TransportException>(() => model.ValidateInstanceAsync("none", cancellationToken: token)),
            await Assert.ThrowsAsync<TransportException>(() => model.ValidateInstanceAsync("summary", cancellationToken: token)),
            await Assert.ThrowsAsync<TransportException>(() => model.ValidateInstanceAsync("bounded", cancellationToken: token)),
        ];

        Assert.All(malformed, failure => Assert.Equal(StatusCode.Internal, failure.StatusCode));
        Assert.Contains("neither a verdict nor a failure", malformed[0].Message, StringComparison.Ordinal);
        Assert.Contains("beside verdict", malformed[1].Message, StringComparison.Ordinal);
        Assert.Contains("beside verdicts", malformed[3].Message, StringComparison.Ordinal);
        Assert.Contains("neither a summary nor a failure", malformed[4].Message, StringComparison.Ordinal);
        Assert.Contains("beside summary", malformed[5].Message, StringComparison.Ordinal);
        Assert.Contains("beside bounded", malformed[6].Message, StringComparison.Ordinal);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task VerifySatisfactionSendsTheScopeAndQuestionAndReadsEveryVerdict(ClientWire wire)
    {
        var answers = new Queue<Wire.VerifySatisfactionResponse>(
            [CompleteVerificationWire.Satisfaction, new Wire.VerifySatisfactionResponse(), new Wire.VerifySatisfactionResponse { Verdicts = { new Wire.Verdict { Kind = "satisfy", Holds = true } } }]);
        using var handler = Serving(wire).On("VerifySatisfaction", Wire.VerifySatisfactionRequest.Parser, _ => answers.Dequeue());
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash(Hash);
        var token = TestContext.Current.CancellationToken;

        var satisfaction = await model.VerifySatisfactionAsync("Demo::checks", new VerifyOptions { Question = VerifyOptions.Satisfiable, Engine = "auto" }, token);
        var none = await model.SatisfiedAsync(cancellationToken: token);
        var all = await model.SatisfiedAsync("Demo::ok", cancellationToken: token);

        Assert.Equal(
            [
                new Wire.VerifySatisfactionRequest { ModelHash = Hash, SymbolId = "Demo::checks", Engine = "auto", Question = "satisfiable" },
                new Wire.VerifySatisfactionRequest { ModelHash = Hash },
                new Wire.VerifySatisfactionRequest { ModelHash = Hash, SymbolId = "Demo::ok" },
            ],
            handler.RequestsFor("VerifySatisfaction").Select(sent => sent.Message(Wire.VerifySatisfactionRequest.Parser)));
        Assert.Equal(2, satisfaction.Verdicts.Count);
        Assert.False(satisfaction.Holds);
        Assert.Equal(FailureReason.Undecided, satisfaction.Verdicts[1].FailureReason);
        Assert.Single(satisfaction.Verifications);
        Assert.Equal(9, satisfaction.FindInstance(9)!.Id);
        Assert.Single(satisfaction.Diagnostics);
        Assert.True(none);
        Assert.True(all);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ValidateInstanceReadsTheSummaryEveryVerdictAndTheObjectsReached(ClientWire wire)
    {
        using var handler = Serving(wire).On("ValidateInstance", Wire.ValidateInstanceRequest.Parser, _ => CompleteVerificationWire.Validation);
        await using var connection = await FakeConnection.OpenAsync(handler);

        var validation = await connection.ModelByHash(Hash).ValidateInstanceAsync("Demo::sedan", new ValidateOptions { Engine = "all" }, TestContext.Current.CancellationToken);

        Assert.Equal(
            new Wire.ValidateInstanceRequest { ModelHash = Hash, SymbolId = "Demo::sedan", Engine = "all" },
            Assert.Single(handler.RequestsFor("ValidateInstance")).Message(Wire.ValidateInstanceRequest.Parser));
        Assert.Equal(("object", "Demo::sedan", "violated"), (validation.Summary.Kind, validation.Summary.ElementId, validation.Summary.Status));
        Assert.True(validation.Summary.Violated);
        Assert.False(validation.Holds);
        Assert.True(validation.Bounded);
        Assert.Equal("engine", Assert.Single(validation.Verdicts).InstancePath);
        Assert.Equal(9, validation.Root!.Id);
        Assert.Equal("Demo::Engine", validation.FindInstance(10)!.TypeSymbolId);
        Assert.Single(validation.Verifications);
        Assert.Single(validation.Diagnostics);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task VerificationGatesAreCheckedInTheServicesOrderBeforeAnythingIsSent(ClientWire wire)
    {
        var token = TestContext.Current.CancellationToken;
        var everything = new VerifyOptions
        {
            Question = VerifyOptions.Holds,
            Engine = " explore ",
            Arguments = [new Value.SetValue([new Value.IntegerValue(1)])],
            NamedArguments = new Dictionary<string, Value> { ["c"] = new Value.ComplexValue(1, 2) },
        };

        Assert.Equal(Capabilities.Verification, await RefusedAsync(wire, [Capabilities.Verification, Capabilities.VerificationQuestions], model => model.VerifyConstraintAsync("C", everything, token)));
        Assert.Equal(Capabilities.VerificationQuestions, await RefusedAsync(wire, [Capabilities.VerificationQuestions, Capabilities.Engines], model => model.VerifyRequirementAsync("R", everything, token)));
        Assert.Equal(Capabilities.Engines, await RefusedAsync(wire, [Capabilities.Engines, Capabilities.ScheduleExplore], model => model.VerifyConstraintAsync("C", everything, token)));
        Assert.Equal(Capabilities.ScheduleExplore, await RefusedAsync(wire, [Capabilities.ScheduleExplore, Capabilities.VerificationArguments], model => model.VerifyConstraintAsync("C", everything, token)));
        Assert.Equal(Capabilities.VerificationArguments, await RefusedAsync(wire, [Capabilities.VerificationArguments, Capabilities.SetValues], model => model.VerifyConstraintAsync("C", everything, token)));
        Assert.Equal(Capabilities.SetValues, await RefusedAsync(wire, [Capabilities.SetValues, Capabilities.ComplexValues], model => model.VerifyConstraintAsync("C", everything, token)));
        Assert.Equal(Capabilities.ComplexValues, await RefusedAsync(wire, [Capabilities.ComplexValues], model => model.VerifyConstraintAsync("C", everything, token)));
        Assert.Equal(Capabilities.VerificationQuestions, await RefusedAsync(wire, [Capabilities.VerificationQuestions], model => model.VerifySatisfactionAsync(options: new VerifyOptions { Question = VerifyOptions.Satisfiable }, cancellationToken: token)));
        Assert.Equal(Capabilities.Verification, await RefusedAsync(wire, [Capabilities.Verification], model => model.VerifySatisfactionAsync(cancellationToken: token)));
        Assert.Equal(Capabilities.Verification, await RefusedAsync(wire, [Capabilities.Verification, Capabilities.Engines], model => model.ValidateInstanceAsync("P", new ValidateOptions { Engine = "explore" }, token)));
        Assert.Equal(Capabilities.Engines, await RefusedAsync(wire, [Capabilities.Engines], model => model.ValidateInstanceAsync("P", new ValidateOptions { Engine = "run" }, token)));
        Assert.Equal(Capabilities.ScheduleExplore, await RefusedAsync(wire, [Capabilities.ScheduleExplore], model => model.ValidateInstanceAsync("P", new ValidateOptions { Engine = "explore" }, token)));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AQuestionTheServiceDoesNotGateIsSentForItToJudge(ClientWire wire)
    {
        using var handler = new FakeServiceHandler(wire) { Capabilities = [.. FakeModels.ModelCapabilities.Except([Capabilities.VerificationQuestions])] }
            .Refusing("VerifyConstraint", StatusCode.InvalidArgument, "unknown verification question \"bogus\": evaluate, holds or satisfiable")
            .On("VerifyRequirement", Wire.VerifyRequirementRequest.Parser, _ => CompleteVerificationWire.Requirement);
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash(Hash);
        var token = TestContext.Current.CancellationToken;

        var bogus = await Assert.ThrowsAsync<ServiceException>(() => model.VerifyConstraintAsync("C", new VerifyOptions { Question = "bogus" }, token));
        await model.VerifyRequirementAsync("R", new VerifyOptions { Question = VerifyOptions.Evaluate }, token);

        Assert.Equal(StatusCode.InvalidArgument, bogus.StatusCode);
        Assert.Equal("evaluate", Assert.Single(handler.RequestsFor("VerifyRequirement")).Message(Wire.VerifyRequirementRequest.Parser).Question);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AnEmptyOptionASatisfactionSubjectOrAnUnsendableArgumentIsRefusedBeforeSending(ClientWire wire)
    {
        using var handler = Serving(wire);
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash(Hash);
        var token = TestContext.Current.CancellationToken;

        await Assert.ThrowsAsync<InvalidRequestException>(() => model.VerifyConstraintAsync("C", new VerifyOptions { Question = string.Empty }, token));
        await Assert.ThrowsAsync<InvalidRequestException>(() => model.VerifyRequirementAsync("R", new VerifyOptions { SubjectSymbolId = string.Empty }, token));
        await Assert.ThrowsAsync<InvalidRequestException>(() => model.VerifyConstraintAsync("C", new VerifyOptions { Engine = string.Empty }, token));
        await Assert.ThrowsAsync<InvalidRequestException>(() => model.VerifySatisfactionAsync(string.Empty, cancellationToken: token));
        await Assert.ThrowsAsync<InvalidRequestException>(() => model.VerifySatisfactionAsync(options: new VerifyOptions { SubjectSymbolId = "Demo::sedan" }, cancellationToken: token));
        await Assert.ThrowsAsync<InvalidRequestException>(() => model.VerifySatisfactionAsync(options: new VerifyOptions { Arguments = [new Value.IntegerValue(1)] }, cancellationToken: token));
        await Assert.ThrowsAsync<InvalidRequestException>(() => model.ValidateInstanceAsync("P", new ValidateOptions { Engine = string.Empty }, token));
        var unset = await Assert.ThrowsAsync<InvalidRequestException>(() =>
            model.VerifyConstraintAsync("C", new VerifyOptions { NamedArguments = new Dictionary<string, Value> { ["x"] = new Value.UnsetValue() } }, token));
        var undetermined = await Assert.ThrowsAsync<InvalidRequestException>(() =>
            model.VerifyRequirementAsync("R", new VerifyOptions { Arguments = [new Value.IntegerValue(1), new Value.UndeterminedValue("open", string.Empty, string.Empty)] }, token));
        await Assert.ThrowsAsync<ArgumentNullException>(() => model.VerifyConstraintAsync(null!, cancellationToken: token));
        await Assert.ThrowsAsync<ArgumentNullException>(() => model.VerifyRequirementAsync(null!, cancellationToken: token));
        await Assert.ThrowsAsync<ArgumentNullException>(() => model.ValidateInstanceAsync(null!, cancellationToken: token));

        Assert.Contains("named_arguments[\"x\"]", unset.Message, StringComparison.Ordinal);
        Assert.Contains("arguments[1]", undetermined.Message, StringComparison.Ordinal);
        Assert.Equal(["GetServerInfo"], handler.Requests.Select(request => request.Method));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AFailureReasonThisClientDoesNotNameIsKeptAsItsNumber(ClientWire wire)
    {
        using var handler = Serving(wire).On("VerifyConstraint", Wire.VerifyConstraintRequest.Parser, _ =>
            new Wire.VerifyConstraintResponse { Verdict = new Wire.Verdict { Kind = "constraint", Error = "new kind", FailureReason = (Wire.FailureReason)42 } });
        await using var connection = await FakeConnection.OpenAsync(handler);

        var verification = await connection.ModelByHash(Hash).VerifyConstraintAsync("C", cancellationToken: TestContext.Current.CancellationToken);

        Assert.Equal((FailureReason)42, verification.Verdict.FailureReason);
    }

    [Fact]
    public void VerifyOptionsCopyTheirArgumentsAndTheRecordsRefuseANull()
    {
        var positional = new List<Value> { new Value.IntegerValue(1) };
        var named = new Dictionary<string, Value> { ["a"] = new Value.IntegerValue(2) };
        var options = new VerifyOptions { Arguments = positional, NamedArguments = named };

        positional.Add(new Value.IntegerValue(3));
        named["b"] = new Value.IntegerValue(4);

        Assert.Equal([new Value.IntegerValue(1)], options.Arguments);
        Assert.Equal(["a"], options.NamedArguments.Keys);
        Assert.Empty(VerifyOptions.Default.Arguments);
        Assert.Empty(VerifyOptions.Default.NamedArguments);
        Assert.Throws<ArgumentException>(() => new VerifyOptions { Arguments = [null!] });
        Assert.Throws<ArgumentNullException>(() => new VerifyOptions { NamedArguments = null! });
        Assert.Throws<ArgumentNullException>(() => new Verification(null!, [], [], []));
        Assert.Throws<ArgumentNullException>(() => new WitnessAssignment("f", null, null!, ""));
        Assert.Throws<ArgumentNullException>(() => new VerdictNote("uncovered", null!, ""));
        Assert.Equal(new VerdictNote("a_kind_from_the_future", "x", "t"), new VerdictNote("a_kind_from_the_future", "x", "t"));
        Assert.Throws<ArgumentNullException>(() => new VerificationVerdict("c", "pass", "", false, null!, 0, ""));
        Assert.Throws<ArgumentException>(() => new Satisfaction([null!], [], [], []));
        Assert.Throws<ArgumentNullException>(() => new Validation(null!, [], [], [], [], false));
        Assert.Throws<ArgumentNullException>(() => new VerificationException("m", Model.VerifyConstraintMethod, [], FailureReason.Unspecified, [], [], null!));
        Assert.True(new VerificationVerdict("c", "pass", "", false, "", 0, "").Passed);
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
