using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Reflection;
using System.Text.Json;
using System.Text.RegularExpressions;
using System.Threading;
using System.Threading.Tasks;
using OpenMBEE.OpenSysML.Conformance;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// The conformance suite through the public API against a real sysml-grpc: every scenario of a covered
/// RPC runs over all three protocols and under withheld capabilities, skipping only what the expected
/// skips list for the reason they give; each corruption <c>-mutate</c> names fails the run on every
/// protocol; and a run whose list or adapters have drifted from what ran fails.
/// </summary>
[Trait(Category.Name, Category.Integration)]
public sealed class ServiceConformanceTests(ServiceFixture services) : IClassFixture<ServiceFixture>, IDisposable
{
    private const string Withheld = "strict_conformance,oslc_query";

    private readonly string temporary = Directory.CreateTempSubdirectory("opensysml-conformance-").FullName;

    private static CancellationToken Token => TestContext.Current.CancellationToken;

    private static string Suite => Repository.PathOf("conformance");

    private static string Skips => Repository.PathOf("client", "dotnet", "test", "testdata", "expected-skips.json");

    public void Dispose() => Directory.Delete(temporary, recursive: true);

    [Fact]
    public async Task EveryCoveredScenarioRunsOverEveryProtocolAndOnlyTheReviewedSkipsAreSkipped()
    {
        var options = Options() with
        {
            Withhold = Withheld.Split(','),
            Report = Path.Combine(temporary, "report.json"),
            JUnit = Path.Combine(temporary, "report.xml"),
        };

        var (exit, output, errors) = await RunAsync(options);

        Assert.True(exit == 0, errors + output);
        using var report = JsonDocument.Parse(File.ReadAllBytes(options.Report!));
        var expected = ExpectedSkips.Load(Skips);
        var configurations = report.RootElement.GetProperty("configurations").EnumerateArray().ToList();
        Assert.Equal(["default", "without-strict_conformance-oslc_query"], configurations.Select(c => c.GetProperty("name").GetString()));
        foreach (var configuration in configurations)
        {
            var protocols = configuration.GetProperty("protocols").EnumerateArray().ToList();
            Assert.Equal(["grpc", "connect", "connect-json"], protocols.Select(p => p.GetProperty("protocol").GetString()));
            foreach (var protocol in protocols)
            {
                var results = protocol.GetProperty("results").EnumerateArray().ToList();
                Assert.All(results, result => Assert.True(result.GetProperty("outcome").GetString() is "pass" or "skip", result.ToString()));
                foreach (var result in results.Where(result => result.GetProperty("outcome").GetString() == "skip"))
                {
                    var reason = expected.ReasonFor(result.GetProperty("id").GetString()!, result.GetProperty("rpc").GetString()!);
                    Assert.True(
                        reason == result.GetProperty("reason").GetString() || result.TryGetProperty("missing_capabilities", out _),
                        result.ToString());
                }

                Assert.Contains(results, result => result.GetProperty("rpc").GetString() == "Instantiate" && result.GetProperty("outcome").GetString() == "pass");
            }
        }

        var fallbacks = configurations[1].GetProperty("protocols").EnumerateArray()
            .SelectMany(protocol => protocol.GetProperty("results").EnumerateArray())
            .Where(result => result.GetProperty("id").GetString() == "parse/strict_conformance_is_asked_for_by_field")
            .Select(result => (result.GetProperty("outcome").GetString(), result.GetProperty("status").GetString(), result.GetProperty("without_capability").GetBoolean()));
        Assert.Equal([("pass", "UNIMPLEMENTED", true), ("pass", "UNIMPLEMENTED", true), ("pass", "UNIMPLEMENTED", true)], fallbacks);
        var oslc = configurations[1].GetProperty("protocols").EnumerateArray()
            .SelectMany(protocol => protocol.GetProperty("results").EnumerateArray())
            .Where(result => result.GetProperty("id").GetString() == "query/an_oslc_text_query_is_parsed")
            .Select(result => (result.GetProperty("outcome").GetString(), result.GetProperty("status").GetString(), result.GetProperty("without_capability").GetBoolean()));
        Assert.Equal([("pass", "UNIMPLEMENTED", true), ("pass", "UNIMPLEMENTED", true), ("pass", "UNIMPLEMENTED", true)], oslc);
        Assert.StartsWith("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<testsuites name=\"conformance\"", File.ReadAllText(options.JUnit!), StringComparison.Ordinal);
        Assert.DoesNotContain("no without-capability expectation", output, StringComparison.Ordinal);
    }

    [Theory]
    [InlineData("perturb-reals")]
    [InlineData("truncate-lists")]
    [InlineData("rewrite-strings")]
    public async Task EveryCorruptionFailsTheRunOnEveryProtocol(string name)
    {
        Assert.True(Mutations.TryParse(name, out var mutation));
        var options = Options() with { Mutation = mutation, Report = Path.Combine(temporary, $"{name}.json") };

        var (exit, _, errors) = await RunAsync(options);

        Assert.Equal(1, exit);
        Assert.Matches(new Regex(@"^conformance: \d+ of \d+ scenarios failed\n"), errors);
        using var report = JsonDocument.Parse(File.ReadAllBytes(options.Report!));
        var failed = report.RootElement.GetProperty("configurations")[0].GetProperty("protocols").EnumerateArray()
            .Select(protocol => (protocol.GetProperty("protocol").GetString(), protocol.GetProperty("failed").GetInt32()));
        Assert.All(failed, protocol => Assert.True(protocol.Item2 > 0, $"{protocol.Item1} caught no {name}"));
    }

    [Fact]
    public async Task AGrpcOnlyServiceIsTestedOverGrpc()
    {
        var (exit, output, errors) = await RunAsync(Options() with { Transport = "grpc", Protocols = ["grpc"] });

        Assert.True(exit == 0, errors + output);
        Assert.Contains($"[grpc] {ScenarioSuite.Load(Path.Combine(Suite, "scenarios")).Count} scenarios: ", output, StringComparison.Ordinal);
        Assert.DoesNotContain("[connect]", output, StringComparison.Ordinal);
    }

    [Fact]
    public async Task AServiceAlreadyRunningIsTestedWhereItIs()
    {
        var service = await services.ServiceAsync("connect");
        var options = Options() with { Binary = null, Service = service.Address, Protocols = ["connect-json"], Report = Path.Combine(temporary, "r.json") };

        var (exit, output, errors) = await RunAsync(options);

        Assert.True(exit == 0, errors + output);
        using var report = JsonDocument.Parse(File.ReadAllBytes(options.Report!));
        Assert.Equal(service.Address, report.RootElement.GetProperty("service").GetString());
        Assert.False(service.HasExited);
    }

    [Fact]
    public async Task AReviewedSkipMissingFromTheListOrAListedSkipThatRunsFailsTheRun()
    {
        var expected = ExpectedSkips.Load(Skips);
        var missing = Write("missing.json", expected.UncoveredRpcs, new Dictionary<string, string>());
        var stale = Write("stale.json", expected.UncoveredRpcs, new Dictionary<string, string>(expected.Unexpressible) { ["evaluate/arithmetic_is_an_integer"] = "no reason" });
        var options = Options() with { Protocols = ["grpc"], Filter = new Regex("^(parse|evaluate)/", RegexOptions.None, TimeSpan.FromSeconds(1)) };

        var (missed, _, missedErrors) = await RunAsync(options with { ExpectedSkips = missing });
        var (ran, _, ranErrors) = await RunAsync(options with { ExpectedSkips = stale });

        Assert.Equal(1, missed);
        Assert.Equal(
            "conformance: default grpc: parse/naming_no_source_is_invalid was skipped, which the expected skips do not list: "
                + "the public API always names a source, so it cannot send a request naming none\n",
            missedErrors);
        Assert.Equal(1, ran);
        Assert.Equal(
            "conformance: default grpc: evaluate/arithmetic_is_an_integer is listed as an expected skip (no reason), but it ran and ended pass\n",
            ranErrors);
    }

    [Fact]
    public async Task AnRpcTheAdaptersStopCoveringIsAnUnexpectedSkipOfEachOfItsScenarios()
    {
        var service = await services.ServiceAsync("connect");
        await using var connection = await Connection.OpenAsync(ClientWire.Grpc.To(service.Address), Token);
        var adapters = ApiAdapters.All.Where(adapter => adapter.Key != "Instantiate").ToDictionary();
        var client = new ApiScenarioClient("grpc", service.Address, connection, adapters);
        var scenarios = ScenarioSuite.Load(Path.Combine(Suite, "scenarios"));
        var runner = new ScenarioRunner(client, new FixtureDirectory(Path.Combine(Suite, "fixtures")));

        var summary = await runner.RunAllAsync(scenarios, new Regex("^instantiate/", RegexOptions.None, TimeSpan.FromSeconds(1)), Token);
        var problems = SkipAccounting.Skips("default", [], summary, ExpectedSkips.Load(Skips), allowSkips: false).ToList();

        Assert.Equal(scenarios.Count(scenario => scenario.Method == "Instantiate"), problems.Count);
        Assert.All(problems, problem => Assert.EndsWith(", which the expected skips do not list: the public API does not cover Instantiate", problem, StringComparison.Ordinal));
    }

    [Fact]
    public async Task TheRunnerProcessFindsItsSuiteInTheCheckoutAndExitsByWhatRan()
    {
        var binary = ServiceGate.FromProcess.RequireBinary();
        var directory = Repository.PathOf("client", "dotnet");

        var passed = await RunnerProcess.RunAsync(directory, "-binary", binary, "-protocols", "grpc", "-run", "^parse/", "-report", "-");
        var corrupted = await RunnerProcess.RunAsync(directory, "-binary", binary, "-protocols", "grpc", "-run", "^symbol/", "-mutate", "rewrite-strings");

        Assert.True(passed.Exit == 0, passed.Errors + passed.Output);
        Assert.StartsWith("\nConfiguration default\n", passed.Output, StringComparison.Ordinal);
        Assert.Contains("SKIP parse/naming_no_source_is_invalid", passed.Output, StringComparison.Ordinal);
        using var report = JsonDocument.Parse(passed.Output[passed.Output.IndexOf("\n{\n", StringComparison.Ordinal)..]);
        Assert.Equal(binary, report.RootElement.GetProperty("service").GetString());
        Assert.Equal(1, corrupted.Exit);
        Assert.Matches(new Regex(@"^conformance: \d+ of \d+ scenarios failed\n$"), corrupted.Errors);
    }

    [Theory]
    [InlineData("-report", false)]
    [InlineData("-report", true)]
    [InlineData("-junit", false)]
    [InlineData("-junit", true)]
    public async Task AReportThatCannotBeWrittenIsAContextualFailureNotAnUnhandledException(string flag, bool blockedParent)
    {
        var service = await services.ServiceAsync("connect");
        var blocked = Path.Combine(temporary, "blocked");
        if (blockedParent)
        {
            File.WriteAllText(blocked, "not a directory");
        }
        else
        {
            Directory.CreateDirectory(blocked);
        }

        var path = blockedParent ? Path.Combine(blocked, "report.out") : blocked;
        var result = await RunnerProcess.RunAsync(
            Repository.PathOf("client", "dotnet"),
            "-service", service.Address, "-protocols", "grpc", "-run", "^server_info/", flag, path);

        Assert.Equal(1, result.Exit);
        Assert.Contains("[grpc] 1 scenarios: 1 passed", result.Output, StringComparison.Ordinal);
        var format = flag == "-report" ? "JSON" : "JUnit";
        var prefix = $"conformance: cannot write {format} report to {Spelling.Quote(path)}: ";
        Assert.StartsWith(prefix, result.Errors, StringComparison.Ordinal);
        Assert.True(result.Errors.Length > prefix.Length + 1, result.Errors);
        Assert.DoesNotContain("Unhandled exception", result.Errors, StringComparison.Ordinal);
        Assert.False(service.HasExited);
    }

    [Fact]
    public async Task AnAdapterOfAnotherRequestTypeIsAFaultInTheRunnerNotAnAnswer()
    {
        var service = await services.ServiceAsync("connect");
        await using var connection = await Connection.OpenAsync(ClientWire.Grpc.To(service.Address), Token);
        var adapters = new Dictionary<string, RpcAdapter> { ["Evaluate"] = ApiAdapters.All["GetSymbol"] };
        var client = new ApiScenarioClient("grpc", service.Address, connection, adapters);

        var fault = await Assert.ThrowsAsync<InvalidOperationException>(() =>
            client.CallAsync(ScenarioRequests.Method("Evaluate"), new Wire.EvaluateRequest { ModelHash = "h" }, Token));

        Assert.Equal(
            "the Evaluate adapter reads a OpenMBEE.OpenSysML.Wire.GetSymbolRequest, not the OpenMBEE.OpenSysML.Wire.EvaluateRequest the scenario decoded",
            fault.Message);
    }

    private static Conformance.RunOptions Options() => new() { Directory = Suite, ExpectedSkips = Skips, Binary = ServiceGate.FromProcess.RequireBinary() };

    private static async Task<(int Exit, string Output, string Errors)> RunAsync(Conformance.RunOptions options)
    {
        using var output = new StringWriter();
        using var errors = new StringWriter();
        var exit = await ConformanceRun.RunAsync(options, output, errors, Token);
        return (exit, output.ToString(), errors.ToString());
    }

    private string Write(string name, IEnumerable<string> uncovered, IReadOnlyDictionary<string, string> unexpressible)
    {
        var path = Path.Combine(temporary, name);
        File.WriteAllText(path, JsonSerializer.Serialize(new { uncovered_rpcs = uncovered, unexpressible }));
        return path;
    }
}

/// <summary>The command line of the runner the solution builds, as <c>make conformance-dotnet</c> runs it.</summary>
[Trait(Category.Name, Category.Unit)]
public sealed class ConformanceRunnerProcessTests
{
    [Theory]
    [InlineData("-run")]
    [InlineData("-report")]
    [InlineData("-junit")]
    [InlineData("-binary")]
    public async Task AMissingValueBeforeAnOptionIsAUsageErrorWithoutStartingARun(string flag)
    {
        var result = await RunnerProcess.RunAsync(Repository.PathOf("client", "dotnet"), flag, "-v");

        Assert.Equal((2, ""), (result.Exit, result.Output));
        Assert.Equal($"conformance: {flag} wants a value\n" + RunArguments.Usage, result.Errors);
    }

    [Fact]
    public async Task HelpPrintsTheUsageAndAMalformedLineIsAUsageError()
    {
        var directory = Repository.PathOf("client", "dotnet");

        var help = await RunnerProcess.RunAsync(directory, "-h");
        var unknown = await RunnerProcess.RunAsync(directory, "-binary", "/b", "-bogus");
        var unnamed = await RunnerProcess.RunAsync(directory);

        Assert.Equal((0, ""), (help.Exit, help.Errors));
        Assert.Equal(RunArguments.Usage, help.Output);
        Assert.Equal((2, ""), (unknown.Exit, unknown.Output));
        Assert.Equal("conformance: unknown flag -bogus\n" + RunArguments.Usage, unknown.Errors);
        Assert.Equal(2, unnamed.Exit);
        Assert.StartsWith("conformance: name the service to test: ", unnamed.Errors, StringComparison.Ordinal);
    }
}

/// <summary>Runs the runner the solution built from its own output, as a process, and collects what it wrote.</summary>
internal static class RunnerProcess
{
    private static readonly TimeSpan Deadline = TimeSpan.FromMinutes(2);

    /// <summary>Runs the runner to its exit.</summary>
    /// <param name="workingDirectory">Where it runs, which its defaults are found above.</param>
    /// <param name="arguments">Its command line.</param>
    /// <returns>Its exit status, standard output and standard error.</returns>
    internal static async Task<(int Exit, string Output, string Errors)> RunAsync(string workingDirectory, params string[] arguments)
    {
        FakeChild.EnsureRuntimeIsFound();
        const string Name = "OpenMBEE.OpenSysML.Conformance";
        var launcher = Repository.PathOf(
            "client", "dotnet", "src", Name, "bin",
            typeof(RunnerProcess).Assembly.GetCustomAttribute<AssemblyConfigurationAttribute>()?.Configuration ?? "Release",
            "net10.0",
            OperatingSystem.IsWindows() ? Name + ".exe" : Name);
        var start = new ProcessStartInfo(launcher)
        {
            WorkingDirectory = workingDirectory,
            RedirectStandardOutput = true,
            RedirectStandardError = true,
            UseShellExecute = false,
            CreateNoWindow = true,
        };
        foreach (var argument in arguments)
        {
            start.ArgumentList.Add(argument);
        }

        using var process = Process.Start(start) ?? throw new InvalidOperationException($"{launcher} did not start.");
        var output = process.StandardOutput.ReadToEndAsync(TestContext.Current.CancellationToken);
        var errors = process.StandardError.ReadToEndAsync(TestContext.Current.CancellationToken);
        using var deadline = CancellationTokenSource.CreateLinkedTokenSource(TestContext.Current.CancellationToken);
        deadline.CancelAfter(Deadline);
        try
        {
            await process.WaitForExitAsync(deadline.Token);
        }
        catch (OperationCanceledException)
        {
            process.Kill(entireProcessTree: true);
            await process.WaitForExitAsync(CancellationToken.None);
            throw;
        }

        return (process.ExitCode, await output, await errors);
    }
}
