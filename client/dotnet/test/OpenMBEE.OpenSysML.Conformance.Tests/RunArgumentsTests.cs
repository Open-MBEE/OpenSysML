using System;
using System.IO;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Xunit;

namespace OpenMBEE.OpenSysML.Conformance.Tests;

/// <summary>
/// The runner's command line: every flag, the defaults it finds in the checkout, and every malformed,
/// incomplete or contradictory line refused as a usage error rather than run as something else; and
/// a run that cannot start failing before it starts a service.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed class RunArgumentsTests : IDisposable
{
    private readonly TempDirectory checkout = new();

    public RunArgumentsTests()
    {
        checkout.Write("conformance/scenarios/01.json", """{"scenarios": []}""");
        Directory.CreateDirectory(Path.Combine(checkout.Path, "sub", "dir"));
    }

    public void Dispose() => checkout.Dispose();

    [Fact]
    public void TheDefaultsComeFromTheCheckoutAboveTheWorkingDirectory()
    {
        var options = Parse("-binary", "bin/sysml-grpc")!;

        Assert.Equal(Path.Combine(checkout.Path, "conformance"), options.Directory);
        Assert.Equal(Path.Combine(checkout.Path, "client", "dotnet", "test", "testdata", "expected-skips.json"), options.ExpectedSkips);
        Assert.Equal(Path.Combine(checkout.Path, "sub", "dir", "bin", "sysml-grpc"), options.Binary);
        Assert.Null(options.Service);
        Assert.Equal("connect", options.Transport);
        Assert.Equal(["grpc", "connect", "connect-json"], options.Protocols);
        Assert.Empty(options.Withhold);
        Assert.Null(options.Filter);
        Assert.Null(options.Report);
        Assert.Null(options.JUnit);
        Assert.False(options.AllowSkips);
        Assert.Equal(Mutation.None, options.Mutation);
        Assert.False(options.Verbose);
    }

    [Fact]
    public void EveryFlagIsRead()
    {
        var options = Parse(
            "-service", "localhost:50051",
            "-dir", "/suite",
            "-expected-skips", "skips.json",
            "-run", "^evaluate/",
            "-protocols", " connect-json , grpc ",
            "-report", "-",
            "-junit", "out/r.xml",
            "-allow-skips",
            "-mutate", "rewrite-strings",
            "-v")!;

        Assert.Equal(Path.GetFullPath("/suite"), options.Directory);
        Assert.Equal(Path.Combine(checkout.Path, "sub", "dir", "skips.json"), options.ExpectedSkips);
        Assert.Equal("localhost:50051", options.Service);
        Assert.Null(options.Binary);
        Assert.Matches(options.Filter!, "evaluate/a");
        Assert.DoesNotMatch(options.Filter!, "x/evaluate/a");
        Assert.Equal(["connect-json", "grpc"], options.Protocols);
        Assert.Equal("-", options.Report);
        Assert.Equal(Path.Combine(checkout.Path, "sub", "dir", "out", "r.xml"), options.JUnit);
        Assert.True(options.AllowSkips);
        Assert.Equal(Mutation.RewriteStrings, options.Mutation);
        Assert.True(options.Verbose);
    }

    [Fact]
    public void InlineValuesPreserveLiteralOptionTokensAndEqualsSigns()
    {
        var options = Parse(
            "-binary=bin/service=local",
            "-dir=suite=local",
            "-expected-skips=skips=local.json",
            "-run=-v=literal",
            "-protocols=grpc,connect-json",
            "-transport=connect",
            "-withhold-capabilities=strict_conformance",
            "-report=-v",
            "-junit=-h",
            "-mutate=rewrite-strings",
            "-v")!;

        var workingDirectory = Path.Combine(checkout.Path, "sub", "dir");
        Assert.Equal(Path.Combine(workingDirectory, "bin", "service=local"), options.Binary);
        Assert.Equal(Path.Combine(workingDirectory, "suite=local"), options.Directory);
        Assert.Equal(Path.Combine(workingDirectory, "skips=local.json"), options.ExpectedSkips);
        Assert.Matches(options.Filter!, "-v=literal");
        Assert.Equal(["grpc", "connect-json"], options.Protocols);
        Assert.Equal("connect", options.Transport);
        Assert.Equal(["strict_conformance"], options.Withhold);
        Assert.Equal(Path.Combine(workingDirectory, "-v"), options.Report);
        Assert.Equal(Path.Combine(workingDirectory, "-h"), options.JUnit);
        Assert.Equal(Mutation.RewriteStrings, options.Mutation);
        Assert.True(options.Verbose);
        Assert.Equal("localhost:50051", Parse("-service=localhost:50051")!.Service);
    }

    [Theory]
    [InlineData("-binary")]
    [InlineData("-service")]
    [InlineData("-dir")]
    [InlineData("-expected-skips")]
    [InlineData("-run")]
    [InlineData("-protocols")]
    [InlineData("-transport")]
    [InlineData("-withhold-capabilities")]
    [InlineData("-report")]
    [InlineData("-junit")]
    [InlineData("-mutate")]
    public void AnOptionCannotBeConsumedAsAnotherOptionsValue(string flag)
    {
        foreach (var next in new[] { "-v", "-h", "-bogus", "-protocols=grpc" })
        {
            var refused = Assert.Throws<UsageException>(() => Parse(flag, next));

            Assert.Equal($"{flag} wants a value", refused.Message);
        }
    }

    [Theory]
    [InlineData("-v")]
    [InlineData("-allow-skips")]
    [InlineData("-h")]
    [InlineData("-help")]
    [InlineData("--help")]
    public void ASwitchDoesNotAcceptAnInlineValue(string flag)
    {
        var refused = Assert.Throws<UsageException>(() => Parse($"{flag}=true"));

        Assert.Equal($"{flag} does not take a value", refused.Message);
    }

    [Fact]
    public void AStartedServiceMayServeGrpcAloneAndWithholdCapabilities()
    {
        var options = Parse("-binary", "/b", "-transport", "grpc", "-protocols", "grpc", "-withhold-capabilities", "strict_conformance,,oslc_query")!;

        Assert.Equal("grpc", options.Transport);
        Assert.Equal(["strict_conformance", "oslc_query"], options.Withhold);
    }

    [Theory]
    [InlineData("-h")]
    [InlineData("-binary", "/b", "-help")]
    [InlineData("--help")]
    public void HelpAsksForTheUsageAlone(params string[] args)
    {
        Assert.Null(Parse(args));
        Assert.Contains("-mutate <name>        corrupt every answer: perturb-reals, truncate-lists, rewrite-strings", RunArguments.Usage, StringComparison.Ordinal);
    }

    [Theory]
    [InlineData("name the service to test: -binary to start one, or -service for one already running")]
    [InlineData("name the service to test: -binary to start one, or -service for one already running", "-binary", "/b", "-service", "h:1")]
    [InlineData("unknown flag -bogus", "-binary", "/b", "-bogus")]
    [InlineData("unexpected argument \"extra\"", "-binary", "/b", "extra")]
    [InlineData("-binary wants a value", "-binary")]
    [InlineData("-run wants a value", "-binary", "/b", "-run", "")]
    [InlineData("-run wants a value", "-binary", "/b", "-run=")]
    [InlineData("-binary wants a value", "-binary=")]
    [InlineData("-v is given more than once", "-binary", "/b", "-v", "-v")]
    [InlineData("-binary is given more than once", "-binary=/b", "-binary", "/c")]
    [InlineData("-binary is given more than once", "-binary", "/b", "-binary=/c")]
    [InlineData("-run is given more than once", "-binary=/b", "-run=a", "-run=b")]
    [InlineData("-transport chooses what a started service serves; it does not apply to -service", "-service", "h:1", "-transport", "grpc")]
    [InlineData("-withhold-capabilities needs a service the run starts; it does not apply to -service", "-service", "h:1", "-withhold-capabilities", "query")]
    [InlineData("unknown -transport \"stdio\"; want connect or grpc", "-binary", "/b", "-transport", "stdio")]
    [InlineData("protocol connect needs -transport connect; a grpc-only service speaks grpc alone", "-binary", "/b", "-transport", "grpc")]
    [InlineData("unknown protocol \"mcp\"; want grpc, connect or connect-json", "-binary", "/b", "-protocols", "grpc,mcp")]
    [InlineData("protocol \"grpc\" is listed more than once", "-binary", "/b", "-protocols", "grpc,grpc")]
    [InlineData("-protocols must name at least one protocol", "-binary", "/b", "-protocols", ",")]
    [InlineData("capability \"query\" is listed more than once", "-binary", "/b", "-withhold-capabilities", "query,query")]
    [InlineData("unknown -mutate \"none\"; want one of perturb-reals, truncate-lists, rewrite-strings", "-binary", "/b", "-mutate", "none")]
    [InlineData("bad -run pattern \"(\"", "-binary", "/b", "-run", "(")]
    public void AMalformedCommandLineIsAUsageError(string problem, params string[] args)
    {
        var refused = Assert.Throws<UsageException>(() => Parse(args));

        Assert.StartsWith(problem, refused.Message, StringComparison.Ordinal);
    }

    [Fact]
    public void WithoutACheckoutTheDefaultsMustBeNamed()
    {
        using var elsewhere = new TempDirectory();

        var refused = Assert.Throws<UsageException>(() => RunArguments.Parse(["-binary", "/b"], elsewhere.Path));
        var named = RunArguments.Parse(["-binary", "/b", "-dir", "/suite", "-expected-skips", "/skips.json"], elsewhere.Path)!;

        Assert.StartsWith("no checkout holding conformance/scenarios above ", refused.Message, StringComparison.Ordinal);
        Assert.Equal(Path.GetFullPath("/suite"), named.Directory);
    }

    [Fact]
    public async Task ARunWhoseSuiteOrListCannotBeReadFailsBeforeStartingAService()
    {
        using var suite = new TempDirectory();
        suite.Write("scenarios/01.json", """{"scenarios": [{"id": "a", "rpc": "Evaluate"}]}""");
        suite.Write("skips.json", """{"uncovered_rpcs": ["Evaluate"]}""");
        var binary = Path.Combine(suite.Path, "no-such-binary");
        var options = new RunOptions { Directory = suite.Path, ExpectedSkips = Path.Combine(suite.Path, "skips.json"), Binary = binary };

        var (stale, staleErrors) = await RunAsync(options);
        var (unread, unreadErrors) = await RunAsync(options with { ExpectedSkips = Path.Combine(suite.Path, "absent.json") });
        var (empty, emptyErrors) = await RunAsync(options with { Directory = Path.Combine(suite.Path, "absent") });

        Assert.Equal(1, stale);
        Assert.Contains("conformance: the expected skips list Evaluate as uncovered, but an adapter covers it\n", staleErrors, StringComparison.Ordinal);
        Assert.Equal(1, unread);
        Assert.Contains("absent.json", unreadErrors, StringComparison.Ordinal);
        Assert.Equal(1, empty);
        Assert.Contains("no scenario files in", emptyErrors, StringComparison.Ordinal);
    }

    [Fact]
    public async Task ARunWhoseServiceCannotStartFailsWithTheCauseAndWritesNoReport()
    {
        using var suite = new TempDirectory();
        suite.Write("scenarios/01.json", """{"scenarios": []}""");
        var uncovered = ScenarioRequests.Service.Methods.Where(method => !ApiAdapters.All.ContainsKey(method.Name)).Select(method => $"\"{method.Name}\"");
        var skips = suite.Write("skips.json", $$"""{"uncovered_rpcs": [{{string.Join(", ", uncovered)}}]}""");
        var report = Path.Combine(suite.Path, "report.json");
        var options = new RunOptions { Directory = suite.Path, ExpectedSkips = skips, Binary = Path.Combine(suite.Path, "no-such-binary"), Report = report };

        var (exit, errors) = await RunAsync(options);

        Assert.Equal(1, exit);
        Assert.StartsWith("conformance: default: ", errors, StringComparison.Ordinal);
        Assert.False(File.Exists(report));
    }

    private RunOptions? Parse(params string[] args) => RunArguments.Parse(args, Path.Combine(checkout.Path, "sub", "dir"));

    private static async Task<(int Exit, string Errors)> RunAsync(RunOptions options)
    {
        using var output = new StringWriter();
        using var errors = new StringWriter();
        var exit = await ConformanceRun.RunAsync(options, output, errors, CancellationToken.None);
        return (exit, errors.ToString());
    }
}
