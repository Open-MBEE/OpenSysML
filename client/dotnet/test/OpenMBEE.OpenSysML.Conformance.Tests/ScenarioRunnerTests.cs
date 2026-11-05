using System;
using System.IO;
using System.Linq;
using System.Text.RegularExpressions;
using System.Threading;
using System.Threading.Tasks;
using Xunit;
using static OpenMBEE.OpenSysML.Conformance.Tests.Trees;

namespace OpenMBEE.OpenSysML.Conformance.Tests;

/// <summary>
/// How the runner gates, parses, calls and judges, against an adapter the test scripts. Nothing here
/// talks to a service: the runner's rules hold whatever the adapter.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed class ScenarioRunnerTests : IDisposable
{
    private readonly TempDirectory directory = new();
    private readonly FixtureDirectory fixtures;

    public ScenarioRunnerTests()
    {
        directory.Write("fixtures/vehicle.sysml", "part def Vehicle;\n");
        directory.Write("fixtures/engine.sysml", "part def Engine;\n");
        fixtures = new FixtureDirectory(Path.Combine(directory.Path, "fixtures"));
    }

    public void Dispose() => directory.Dispose();

    [Fact]
    public async Task AScenarioNeedingAnUnreportedCapabilityIsSkipped()
    {
        var client = new FakeScenarioClient { Capabilities = ["query"] };

        var result = await Run(client, """{"id": "c", "rpc": "Convert", "requires_capabilities": ["query", "convert"], "model": {"fixture": "vehicle.sysml"}}""");

        Assert.Equal(ScenarioOutcome.Skip, result.Outcome);
        Assert.Equal(("-", "the service does not report convert"), (result.Status, result.Reason));
        Assert.Equal(["convert"], result.MissingCapabilities);
        Assert.False(result.WithoutCapability);
        Assert.Empty(client.Calls);
        Assert.Empty(client.Parses);
    }

    [Fact]
    public async Task AWithoutCapabilityExpectationJudgesAServiceLackingTheCapability()
    {
        const string Scenario = """
            {"id": "p", "rpc": "ParseFile", "requires_capabilities": ["strict_conformance"],
             "request": {"content": "part p;", "strict_conformance": true},
             "expect": {"response": {"error": ""}},
             "expect_without_capability": {"status": "UNIMPLEMENTED", "status_message_contains": "strict_conformance"}}
            """;
        var refusing = new FakeScenarioClient
        {
            OnCall = (_, _, _) => Task.FromResult<ScenarioAnswer>(new ScenarioAnswer.Refused(StatusCode.Unimplemented, "strict_conformance is not available")),
        };

        var refused = await Run(refusing, Scenario);

        Assert.Equal(ScenarioOutcome.Pass, refused.Outcome);
        Assert.Equal("UNIMPLEMENTED", refused.Status);
        Assert.True(refused.WithoutCapability);
        Assert.Equal(["strict_conformance"], refused.MissingCapabilities);
        Assert.Equal("the service does not report strict_conformance, so the without-capability expectation applies", refused.Reason);

        var accepting = await Run(new FakeScenarioClient(), Scenario);
        Assert.Equal(ScenarioOutcome.Fail, accepting.Outcome);
        Assert.Equal(["the call succeeded, want status UNIMPLEMENTED"], accepting.Failures);
        Assert.True(accepting.WithoutCapability);
    }

    [Fact]
    public async Task EachModelIsParsedOncePerRunAndItsHashFillsTheRequest()
    {
        var client = new FakeScenarioClient();
        var runner = new ScenarioRunner(client, fixtures);
        Scenario[] scenarios =
        [
            Scenario("""{"id": "a", "rpc": "Evaluate", "model": {"fixture": "vehicle.sysml"}, "request": {"model_hash": "${model_hash}"}}"""),
            Scenario("""{"id": "b", "rpc": "Evaluate", "model": {"fixture": "vehicle.sysml"}, "request": {"model_hash": "${model_hash}"}}"""),
            Scenario("""{"id": "c", "rpc": "Evaluate", "model": {"fixture": "vehicle.sysml", "strict_conformance": true}}"""),
            Scenario("""{"id": "d", "rpc": "GetSymbol", "model": {"fixtures": ["engine.sysml", "vehicle.sysml"], "language": "sysml"}, "request": {"model_hash": "${model_hash}"}}"""),
        ];

        var summary = await runner.RunAllAsync(scenarios, filter: null, CancellationToken.None);

        Assert.All(summary.Results, result => Assert.Equal(ScenarioOutcome.Pass, result.Outcome));
        Assert.Equal(3, client.Parses.Count);
        Assert.True(client.Parses[1].Model.StrictConformance);
        Assert.Equal(
            [new FixtureSource("engine.sysml", "part def Engine;\n"), new FixtureSource("vehicle.sysml", "part def Vehicle;\n")],
            client.Parses[2].Documents);
        Assert.Equal("hash-of-vehicle.sysml", Assert.IsType<Wire.EvaluateRequest>(client.Calls[1].Request).ModelHash);
        Assert.Equal("hash-of-engine.sysml", Assert.IsType<Wire.GetSymbolRequest>(client.Calls[3].Request).ModelHash);
    }

    public static TheoryData<string, string> UncleanParses => new()
    {
        { "error", "parsing fixture \"vehicle.sysml\": the parse failed" },
        { "diagnostic", "fixture \"vehicle.sysml\" does not parse clean: no such type" },
        { "hashless", "parsing fixture \"vehicle.sysml\" returned no model hash" },
        { "thrown", "parsing fixture \"vehicle.sysml\": refused" },
    };

    [Theory]
    [MemberData(nameof(UncleanParses))]
    public async Task AModelThatDoesNotParseCleanIsAnErrorInTheScenario(string how, string reason)
    {
        var client = new FakeScenarioClient
        {
            OnParse = (_, _, _) => how switch
            {
                "error" => Task.FromResult(new ModelParse("h", "the parse failed", [])),
                "diagnostic" => Task.FromResult(new ModelParse("h", "", [new("warning", "unused"), new("Error", "no such type")])),
                "hashless" => Task.FromResult(new ModelParse("", "", [new("warning", "unused")])),
                _ => throw new InvalidOperationException("refused"),
            },
        };
        var runner = new ScenarioRunner(client, fixtures);
        var scenario = Scenario("""{"id": "e", "rpc": "Evaluate", "model": {"fixture": "vehicle.sysml"}}""");

        var first = await runner.RunAsync(scenario, CancellationToken.None);
        var second = await runner.RunAsync(scenario, CancellationToken.None);

        Assert.Equal((ScenarioOutcome.Error, "-", reason), (first.Outcome, first.Status, first.Reason));
        Assert.Equal((ScenarioOutcome.Error, reason), (second.Outcome, second.Reason));
        Assert.Equal(2, client.Parses.Count);
        Assert.Empty(client.Calls);
    }

    [Fact]
    public async Task AModelWhoseFixtureCannotBeReadIsAnErrorInTheScenario()
    {
        var result = await Run(new FakeScenarioClient(), """{"id": "e", "rpc": "Evaluate", "model": {"fixture": "missing.sysml"}}""");

        Assert.Equal(ScenarioOutcome.Error, result.Outcome);
        Assert.StartsWith("reading fixture \"missing.sysml\": ", result.Reason, StringComparison.Ordinal);
    }

    [Fact]
    public async Task ARequestThatCannotBeBuiltIsAnErrorInTheScenario()
    {
        var unknown = await Run(new FakeScenarioClient(), """{"id": "f", "rpc": "Forecast"}""");
        Assert.Equal((ScenarioOutcome.Error, "sysml.SysMLService has no RPC \"Forecast\""), (unknown.Outcome, unknown.Reason));

        var misfit = await Run(new FakeScenarioClient(), """{"id": "e", "rpc": "Evaluate", "request": {"expresion": "x"}}""");
        Assert.Equal(ScenarioOutcome.Error, misfit.Outcome);
        Assert.StartsWith("request does not fit sysml.EvaluateRequest", misfit.Reason, StringComparison.Ordinal);
    }

    [Fact]
    public async Task ARequestTheClientCannotExpressIsSkippedWithItsReason()
    {
        var client = new FakeScenarioClient
        {
            OnCall = (_, _, _) => Task.FromResult<ScenarioAnswer>(new ScenarioAnswer.Unexpressible("the API cannot send a query with no operator")),
        };

        var result = await Run(client, """{"id": "q", "rpc": "Query", "expect": {"status": "INVALID_ARGUMENT"}}""");

        Assert.Equal((ScenarioOutcome.Skip, "-", "the API cannot send a query with no operator"), (result.Outcome, result.Status, result.Reason));
        Assert.Empty(result.MissingCapabilities);
    }

    [Fact]
    public async Task ARefusalIsJudgedByItsStatusThenItsMessage()
    {
        var client = new FakeScenarioClient
        {
            OnCall = (_, _, _) => Task.FromResult<ScenarioAnswer>(new ScenarioAnswer.Refused(StatusCode.NotFound, "no symbol Demo::Missing")),
        };

        var expected = await Run(client, """{"id": "s", "rpc": "GetSymbol", "expect": {"status": "NOT_FOUND", "status_message_contains": "Demo::Missing"}}""");
        Assert.Equal((ScenarioOutcome.Pass, "NOT_FOUND"), (expected.Outcome, expected.Status));

        var otherStatus = await Run(client, """{"id": "s", "rpc": "GetSymbol", "expect": {"status": "INVALID_ARGUMENT"}}""");
        Assert.Equal(ScenarioOutcome.Fail, otherStatus.Outcome);
        Assert.Equal(["status: NOT_FOUND (no symbol Demo::Missing), want INVALID_ARGUMENT"], otherStatus.Failures);

        var otherMessage = await Run(client, """{"id": "s", "rpc": "GetSymbol", "expect": {"status": "NOT_FOUND", "status_message_contains": "Demo::Other"}}""");
        Assert.Equal(["status message \"no symbol Demo::Missing\" does not contain \"Demo::Other\""], otherMessage.Failures);

        var success = await Run(client, """{"id": "s", "rpc": "GetSymbol"}""");
        Assert.Equal(["status: NOT_FOUND (no symbol Demo::Missing), want OK"], success.Failures);
    }

    [Fact]
    public async Task ASuccessWhereARefusalIsExpectedFails()
    {
        var result = await Run(new FakeScenarioClient(), """{"id": "s", "rpc": "GetSymbol", "expect": {"status": "NOT_FOUND", "response": {}}}""");

        Assert.Equal((ScenarioOutcome.Fail, "OK"), (result.Outcome, result.Status));
        Assert.Equal(["the call succeeded, want status NOT_FOUND"], result.Failures);
    }

    [Fact]
    public async Task AnAnswerIsNormalizedThenChecked()
    {
        var client = new FakeScenarioClient
        {
            OnCall = (_, request, _) => Task.FromResult<ScenarioAnswer>(new ScenarioAnswer.Answered(new Wire.InstantiateResponse
            {
                Instance = new Wire.Instance { Id = 900, TypeSymbolId = ((Wire.InstantiateRequest)request).ModelHash },
                Instances = { new Wire.Instance { Id = 900 } },
            })),
        };
        const string Holds = """{"id": "i", "rpc": "Instantiate", "model": {"fixture": "vehicle.sysml"}, "request": {"model_hash": "${model_hash}"}, "expect": {"response": {"instance": {"id": "@1", "type_symbol_id": "${model_hash}"}, "instances": [{"id": "@1"}]}}}""";
        const string Disagrees = """{"id": "i", "rpc": "Instantiate", "model": {"fixture": "vehicle.sysml"}, "request": {"model_hash": "${model_hash}"}, "expect": {"response": {"instance": {"id": "@2"}}, "counts": {"instances": 2}}}""";

        var holds = await Run(client, Holds);
        var disagrees = await Run(client, Disagrees);

        Assert.Equal((ScenarioOutcome.Pass, "OK"), (holds.Outcome, holds.Status));
        Assert.Empty(holds.Failures);
        Assert.Equal((ScenarioOutcome.Fail, "OK"), (disagrees.Outcome, disagrees.Status));
        Assert.Equal(["instance.id: \"@1\", want \"@2\"", "instances: 1 entries, want 2"], disagrees.Failures);
    }

    [Fact]
    public async Task AClientThatThrowsIsAnErrorNotADisagreement()
    {
        var client = new FakeScenarioClient
        {
            Protocol = "grpc",
            OnCall = (_, _, _) => throw new InvalidOperationException("broken"),
        };

        var result = await Run(client, """{"id": "s", "rpc": "GetServerInfo"}""");

        Assert.Equal((ScenarioOutcome.Error, "-"), (result.Outcome, result.Status));
        Assert.Equal("the grpc client failed: System.InvalidOperationException: broken", result.Reason);
    }

    [Fact]
    public async Task ACallOutlivingItsDeadlineEndsAsDeadlineExceeded()
    {
        var client = new FakeScenarioClient
        {
            OnCall = async (_, _, cancellationToken) =>
            {
                await Task.Delay(Timeout.Infinite, cancellationToken);
                throw new InvalidOperationException("unreachable");
            },
        };
        var options = new ScenarioRunnerOptions { CallTimeout = TimeSpan.FromMilliseconds(50) };

        var unexpected = await new ScenarioRunner(client, fixtures, options).RunAsync(Scenario("""{"id": "s", "rpc": "GetServerInfo"}"""), CancellationToken.None);
        var expected = await new ScenarioRunner(client, fixtures, options).RunAsync(Scenario("""{"id": "s", "rpc": "GetServerInfo", "expect": {"status": "DEADLINE_EXCEEDED"}}"""), CancellationToken.None);

        Assert.Equal(["status: DEADLINE_EXCEEDED (the call did not end within 0.05 s), want OK"], unexpected.Failures);
        Assert.Equal((ScenarioOutcome.Pass, "DEADLINE_EXCEEDED"), (expected.Outcome, expected.Status));
    }

    [Fact]
    public async Task AParseOutlivingItsDeadlineIsAnErrorInTheScenario()
    {
        var client = new FakeScenarioClient
        {
            OnParse = async (_, _, cancellationToken) =>
            {
                await Task.Delay(Timeout.Infinite, cancellationToken);
                throw new InvalidOperationException("unreachable");
            },
        };
        var runner = new ScenarioRunner(client, fixtures, new ScenarioRunnerOptions { CallTimeout = TimeSpan.FromMilliseconds(50) });

        var result = await runner.RunAsync(Scenario("""{"id": "e", "rpc": "Evaluate", "model": {"fixture": "vehicle.sysml"}}"""), CancellationToken.None);

        Assert.Equal((ScenarioOutcome.Error, "parsing fixture \"vehicle.sysml\": the parse did not end within 0.05 s"), (result.Outcome, result.Reason));
    }

    [Fact]
    public async Task CancellingTheRunIsThrownRatherThanRecorded()
    {
        using var cancel = new CancellationTokenSource();
        var client = new FakeScenarioClient
        {
            OnCall = async (_, _, cancellationToken) =>
            {
                await cancel.CancelAsync();
                await Task.Delay(Timeout.Infinite, cancellationToken);
                throw new InvalidOperationException("unreachable");
            },
        };
        var runner = new ScenarioRunner(client, fixtures);

        await Assert.ThrowsAnyAsync<OperationCanceledException>(() =>
            runner.RunAllAsync([Scenario("""{"id": "s", "rpc": "GetServerInfo"}""")], filter: null, cancel.Token));
    }

    [Fact]
    public async Task AFilterSelectsByIdAndAWithholdingRunOmitsTheHandshake()
    {
        var client = new FakeScenarioClient();
        Scenario[] scenarios =
        [
            Scenario("""{"id": "server-info/reports", "rpc": "GetServerInfo"}"""),
            Scenario("""{"id": "evaluate/one", "rpc": "Evaluate"}"""),
            Scenario("""{"id": "symbol/evaluate_like", "rpc": "GetSymbol"}"""),
            Scenario("""{"id": "query/one", "rpc": "Query"}"""),
        ];

        var filtered = await new ScenarioRunner(client, fixtures).RunAllAsync(scenarios, new Regex("evaluate"), CancellationToken.None);
        var withholding = await new ScenarioRunner(client, fixtures, new ScenarioRunnerOptions { OmitHandshakeScenarios = true })
            .RunAllAsync(scenarios, filter: null, CancellationToken.None);

        Assert.Equal(["evaluate/one", "symbol/evaluate_like"], filtered.Results.Select(result => result.Id));
        Assert.Equal(["evaluate/one", "symbol/evaluate_like", "query/one"], withholding.Results.Select(result => result.Id));
    }

    [Fact]
    public async Task TheSummaryNamesTheProtocolAndItsSortedCapabilities()
    {
        var client = new FakeScenarioClient { Protocol = "connect-json", Service = "bin/sysml-grpc", Capabilities = ["query", "convert", "\U0001F600", "\uE000"] };

        var summary = await new ScenarioRunner(client, fixtures).RunAllAsync([], filter: null, CancellationToken.None);

        Assert.Equal(("connect-json", "bin/sysml-grpc"), (summary.Protocol, summary.Service));
        Assert.Equal(["convert", "query", "\uE000", "\U0001F600"], summary.Capabilities);
        Assert.Equal(new Totals(0, 0, 0, 0, 0, 0), summary.Totals);
    }

    [Fact]
    public async Task ADurationIsMeasuredByTheRunnersClockToTheMicrosecond()
    {
        var options = new ScenarioRunnerOptions { TimeProvider = new StepClock(TimeSpan.FromTicks(12_345_678)) };

        var result = await new ScenarioRunner(new FakeScenarioClient(), fixtures, options).RunAsync(Scenario("""{"id": "s", "rpc": "GetServerInfo"}"""), CancellationToken.None);

        Assert.Equal(1234.567, result.DurationMs);
    }

    [Fact]
    public async Task EachOutcomeIsLoggedAfterItsObservationAndTheTotalsLast()
    {
        var log = new StringWriter();
        var output = new StringWriter();
        var client = new FakeScenarioClient
        {
            Protocol = "grpc",
            Capabilities = ["query"],
            OnCall = (method, _, _) => Task.FromResult<ScenarioAnswer>(method.Name == "GetSymbol"
                ? new ScenarioAnswer.Refused(StatusCode.NotFound, "missing")
                : new ScenarioAnswer.Answered(new Wire.ServerInfoResponse { Version = "9.9", Capabilities = { "query" } })),
        };
        var runner = new ScenarioRunner(client, fixtures, new ScenarioRunnerOptions { Verbose = true, ScenarioLog = log, Output = output });
        Scenario[] scenarios =
        [
            Scenario("""{"id": "info", "rpc": "GetServerInfo", "expect": {"response": {"version": "${version}"}}}"""),
            Scenario("""{"id": "symbol", "rpc": "GetSymbol"}"""),
            Scenario("""{"id": "convert", "rpc": "Convert", "requires_capabilities": ["convert"]}"""),
            Scenario("""{"id": "broken", "rpc": "Forecast"}"""),
        ];

        await runner.RunAllAsync(scenarios, filter: null, CancellationToken.None);

        string[] lines =
        [
            "       {",
            "         \"capabilities\": [",
            "           \"query\"",
            "         ],",
            "         \"version\": \"${version}\"",
            "       }",
            $"PASS {"info",-46} OK",
            $"FAIL {"symbol",-46} NOT_FOUND",
            "       status: NOT_FOUND (missing), want OK",
            $"SKIP {"convert",-46} -",
            "       the service does not report convert",
            $"ERR  {"broken",-46} -",
            "       sysml.SysMLService has no RPC \"Forecast\"",
        ];
        Assert.Equal(string.Concat(lines.Select(line => line + "\n")), log.ToString());
        Assert.Equal("\n[grpc] 4 scenarios: 1 passed, 1 failed, 1 skipped, 1 in error\n", output.ToString());
    }

    private Task<ScenarioResult> Run(FakeScenarioClient client, string scenario) =>
        new ScenarioRunner(client, fixtures).RunAsync(Scenario(scenario), CancellationToken.None);
}
