using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Text.RegularExpressions;
using System.Threading;
using System.Threading.Tasks;
using Xunit;
using static OpenMBEE.OpenSysML.Conformance.Tests.Trees;

namespace OpenMBEE.OpenSysML.Conformance.Tests;

/// <summary>Deadlines, outer cancellation and invalid fixtures must not depend on an adapter's cooperation.</summary>
[Trait(Category.Name, Category.Unit)]
public sealed class ScenarioRunnerSafetyTests : IDisposable
{
    private static readonly TimeSpan Deadline = TimeSpan.FromMilliseconds(100);
    private static readonly TimeSpan TestLimit = TimeSpan.FromSeconds(5);
    private readonly TempDirectory directory = new();
    private readonly FixtureDirectory fixtures;

    public ScenarioRunnerSafetyTests()
    {
        directory.Write("fixtures/vehicle.sysml", "part def Vehicle;");
        fixtures = new FixtureDirectory(Path.Combine(directory.Path, "fixtures"));
    }

    public void Dispose() => directory.Dispose();

    [Theory]
    [InlineData(false)]
    [InlineData(true)]
    public async Task ACallDeadlineBoundsAnUncooperativeAdapterAndAllowsALateCompletion(bool lateFault)
    {
        var pending = new TaskCompletionSource<ScenarioAnswer>(TaskCreationOptions.RunContinuationsAsynchronously);
        var calls = 0;
        var client = new FakeScenarioClient
        {
            OnCall = (_, _, _) => Interlocked.Increment(ref calls) == 1
                ? pending.Task
                : Task.FromResult<ScenarioAnswer>(new ScenarioAnswer.Answered(new Wire.ServerInfoResponse())),
        };
        var runner = new ScenarioRunner(client, fixtures, new ScenarioRunnerOptions { CallTimeout = Deadline });
        var running = runner.RunAsync(
            Scenario("""{"id":"deadline","rpc":"GetServerInfo","expect":{"status":"DEADLINE_EXCEEDED"}}"""),
            CancellationToken.None);
        try
        {
            var result = await running.WaitAsync(TestLimit, TestContext.Current.CancellationToken);

            Assert.Equal((ScenarioOutcome.Pass, "DEADLINE_EXCEEDED"), (result.Outcome, result.Status));
            Assert.False(pending.Task.IsCompleted);
        }
        finally
        {
            if (lateFault)
            {
                pending.TrySetException(new InvalidOperationException("late call failure"));
            }
            else
            {
                pending.TrySetResult(new ScenarioAnswer.Answered(new Wire.ServerInfoResponse()));
            }

            await running.WaitAsync(TestLimit, TestContext.Current.CancellationToken);
        }

        var next = await runner.RunAsync(Scenario("""{"id":"next","rpc":"GetServerInfo"}"""), CancellationToken.None);
        Assert.Equal((ScenarioOutcome.Pass, "OK"), (next.Outcome, next.Status));
    }

    [Theory]
    [InlineData(false)]
    [InlineData(true)]
    public async Task AParseDeadlineBoundsAnUncooperativeAdapterAndNeverCachesItsLateHash(bool lateFault)
    {
        var pending = new TaskCompletionSource<ModelParse>(TaskCreationOptions.RunContinuationsAsynchronously);
        var parses = 0;
        var client = new FakeScenarioClient
        {
            OnParse = (_, _, _) => Interlocked.Increment(ref parses) == 1
                ? pending.Task
                : Task.FromResult(new ModelParse("fresh-hash", "", [])),
        };
        var runner = new ScenarioRunner(client, fixtures, new ScenarioRunnerOptions { CallTimeout = Deadline });
        var scenario = Scenario("""{"id":"parse","rpc":"Evaluate","model":{"fixture":"vehicle.sysml"},"request":{"model_hash":"${model_hash}"}}""");
        var running = runner.RunAsync(scenario, CancellationToken.None);
        try
        {
            var result = await running.WaitAsync(TestLimit, TestContext.Current.CancellationToken);

            Assert.Equal((ScenarioOutcome.Error, "-"), (result.Outcome, result.Status));
            Assert.Contains("the parse did not end within 0.1 s", result.Reason, StringComparison.Ordinal);
            Assert.False(pending.Task.IsCompleted);
            Assert.Empty(client.Calls);
        }
        finally
        {
            if (lateFault)
            {
                pending.TrySetException(new InvalidOperationException("late parse failure"));
            }
            else
            {
                pending.TrySetResult(new ModelParse("late-hash", "", []));
            }

            await running.WaitAsync(TestLimit, TestContext.Current.CancellationToken);
        }

        var next = await runner.RunAsync(scenario, CancellationToken.None);
        Assert.Equal(ScenarioOutcome.Pass, next.Outcome);
        Assert.Equal(2, client.Parses.Count);
        Assert.Equal("fresh-hash", Assert.IsType<Wire.EvaluateRequest>(Assert.Single(client.Calls).Request).ModelHash);
    }

    [Theory]
    [InlineData(false)]
    [InlineData(true)]
    public async Task CancellingAnUncooperativeAdapterPreservesTheCallersToken(bool parsing)
    {
        using var caller = new CancellationTokenSource();
        var parse = new TaskCompletionSource<ModelParse>(TaskCreationOptions.RunContinuationsAsynchronously);
        var call = new TaskCompletionSource<ScenarioAnswer>(TaskCreationOptions.RunContinuationsAsynchronously);
        var client = new FakeScenarioClient
        {
            OnParse = (_, _, _) => parse.Task,
            OnCall = (_, _, _) => call.Task,
        };
        var scenario = parsing
            ? Scenario("""{"id":"parse","rpc":"Evaluate","model":{"fixture":"vehicle.sysml"}}""")
            : Scenario("""{"id":"call","rpc":"GetServerInfo"}""");
        var running = new ScenarioRunner(client, fixtures).RunAsync(scenario, caller.Token);
        try
        {
            await caller.CancelAsync();
            var failure = await Assert.ThrowsAnyAsync<OperationCanceledException>(() =>
                running.WaitAsync(TestLimit, TestContext.Current.CancellationToken));

            Assert.Equal(caller.Token, failure.CancellationToken);
        }
        finally
        {
            parse.TrySetResult(new ModelParse("late-hash", "", []));
            call.TrySetResult(new ScenarioAnswer.Answered(new Wire.ServerInfoResponse()));
            try
            {
                await running.WaitAsync(TestLimit, TestContext.Current.CancellationToken);
            }
            catch (OperationCanceledException) when (caller.IsCancellationRequested)
            {
            }
        }
    }

    [Theory]
    [InlineData("skip")]
    [InlineData("filter")]
    [InlineData("omit")]
    [InlineData("empty")]
    [InlineData("call")]
    [InlineData("parse")]
    public async Task AnAlreadyCancelledRunNeverSkipsFiltersParsesOrCalls(string mode)
    {
        using var caller = new CancellationTokenSource();
        await caller.CancelAsync();
        var client = new FakeScenarioClient();
        var runner = new ScenarioRunner(client, fixtures, new ScenarioRunnerOptions { OmitHandshakeScenarios = mode == "omit" });
        var scenario = mode switch
        {
            "skip" => Scenario("""{"id":"skip","rpc":"GetServerInfo","requires_capabilities":["missing"]}"""),
            "parse" => Scenario("""{"id":"parse","rpc":"Evaluate","model":{"fixture":"vehicle.sysml"}}"""),
            _ => Scenario("""{"id":"call","rpc":"GetServerInfo"}"""),
        };

        var failure = await Assert.ThrowsAnyAsync<OperationCanceledException>(() =>
            runner.RunAllAsync(mode == "empty" ? [] : [scenario], mode == "filter" ? new Regex("never") : null, caller.Token));

        Assert.Equal(caller.Token, failure.CancellationToken);
        Assert.Empty(client.Parses);
        Assert.Empty(client.Calls);
    }

    [Fact]
    public async Task AnAlreadyCancelledSingleScenarioCannotBecomeASkip()
    {
        using var caller = new CancellationTokenSource();
        await caller.CancelAsync();
        var scenario = Scenario("""{"id":"skip","rpc":"GetServerInfo","requires_capabilities":["missing"]}""");

        var failure = await Assert.ThrowsAnyAsync<OperationCanceledException>(() =>
            new ScenarioRunner(new FakeScenarioClient(), fixtures).RunAsync(scenario, caller.Token));

        Assert.Equal(caller.Token, failure.CancellationToken);
    }

    [Fact]
    public async Task CancellationAtTheEndOfAFilteredEnumerationIsStillThrown()
    {
        using var caller = new CancellationTokenSource();
        IEnumerable<Scenario> Cancelling()
        {
            yield return Scenario("""{"id":"filtered","rpc":"GetServerInfo"}""");
            caller.Cancel();
        }

        var failure = await Assert.ThrowsAnyAsync<OperationCanceledException>(() =>
            new ScenarioRunner(new FakeScenarioClient(), fixtures).RunAllAsync(Cancelling(), new Regex("never"), caller.Token));

        Assert.Equal(caller.Token, failure.CancellationToken);
    }

    [Theory]
    [InlineData(false)]
    [InlineData(true)]
    public async Task AnAdapterCannotTurnOuterCancellationIntoASuccess(bool parsing)
    {
        using var caller = new CancellationTokenSource();
        var client = new FakeScenarioClient
        {
            OnParse = (_, _, _) =>
            {
                caller.Cancel();
                return Task.FromResult(new ModelParse("hash", "", []));
            },
            OnCall = (_, _, _) =>
            {
                caller.Cancel();
                return Task.FromResult<ScenarioAnswer>(new ScenarioAnswer.Answered(new Wire.ServerInfoResponse()));
            },
        };
        var scenario = parsing
            ? Scenario("""{"id":"parse","rpc":"Evaluate","model":{"fixture":"vehicle.sysml"}}""")
            : Scenario("""{"id":"call","rpc":"GetServerInfo"}""");

        var failure = await Assert.ThrowsAnyAsync<OperationCanceledException>(() =>
            new ScenarioRunner(client, fixtures).RunAsync(scenario, caller.Token));

        Assert.Equal(caller.Token, failure.CancellationToken);
        if (parsing)
        {
            Assert.Empty(client.Calls);
        }
    }

    [Theory]
    [InlineData(false)]
    [InlineData(true)]
    public async Task AnInvalidFixturePathIsAnErrorAndDoesNotAbortTheRemainingScenarios(bool inRequest)
    {
        var client = new FakeScenarioClient();
        var bad = inRequest
            ? Scenario("""{"id":"bad","rpc":"Evaluate","request":{"expression":"${fixture:\u0000}"}}""")
            : Scenario("""{"id":"bad","rpc":"Evaluate","model":{"fixture":"\u0000"}}""");
        var summary = await new ScenarioRunner(client, fixtures).RunAllAsync(
            [bad, Scenario("""{"id":"good","rpc":"GetServerInfo"}""")], null, CancellationToken.None);

        Assert.Equal([ScenarioOutcome.Error, ScenarioOutcome.Pass], summary.Results.Select(result => result.Outcome));
        Assert.StartsWith("reading fixture \"\\x00\": ", summary.Results[0].Reason, StringComparison.Ordinal);
        Assert.Empty(client.Parses);
        Assert.Single(client.Calls);
    }
}
