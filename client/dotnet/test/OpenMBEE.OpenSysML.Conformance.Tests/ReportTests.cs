using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Text;
using System.Text.Json;
using System.Threading;
using System.Threading.Tasks;
using System.Xml.Linq;
using Google.Protobuf.Reflection;
using OpenMBEE.OpenSysML.Tests;
using Xunit;

namespace OpenMBEE.OpenSysML.Conformance.Tests;

/// <summary>
/// The machine-readable reports: the JSON and JUnit shapes the reference runner writes, and goldens
/// of a whole run of a small suite, whose results, order, normalized observations and log are
/// pinned. Set OPENSYSML_UPDATE_GOLDENS=1 to rewrite the goldens after an intended change, and
/// review the difference.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed class ReportTests
{
    private static readonly string TestData = Repository.PathOf("client", "dotnet", "test", "OpenMBEE.OpenSysML.Conformance.Tests", "testdata");

    [Fact]
    public void TheJsonReportHasTheReferenceShape()
    {
        using var json = JsonDocument.Parse(Json(SampleReport()));
        var root = json.RootElement;

        Assert.Equal(["service", "total", "passed", "failed", "skipped", "errored", "without_capability", "configurations"], Names(root));
        Assert.Equal((6, 3, 1, 1, 1, 1), Totals(root));

        var configuration = root.GetProperty("configurations")[0];
        Assert.Equal(["name", "total", "passed", "failed", "skipped", "errored", "without_capability", "protocols"], Names(configuration));
        var withheld = root.GetProperty("configurations")[1];
        Assert.Equal(["name", "unavailable_capabilities", "total", "passed", "failed", "skipped", "errored", "without_capability", "protocols"], Names(withheld));
        Assert.Equal("oslc_query", withheld.GetProperty("unavailable_capabilities")[0].GetString());
        Assert.Equal((1, 1, 0, 0, 0, 1), Totals(withheld));

        var protocol = configuration.GetProperty("protocols")[0];
        Assert.Equal(["protocol", "service", "capabilities", "total", "passed", "failed", "skipped", "errored", "without_capability", "results"], Names(protocol));
        Assert.Equal(("grpc", "bin/sysml-grpc"), (protocol.GetProperty("protocol").GetString(), protocol.GetProperty("service").GetString()));

        var results = protocol.GetProperty("results").EnumerateArray().ToList();
        Assert.Equal(["id", "outcome", "rpc", "status", "duration_ms"], Names(results[0]));
        Assert.Equal(["id", "outcome", "rpc", "failures", "status", "duration_ms"], Names(results[1]));
        Assert.Equal(["id", "outcome", "rpc", "reason", "status", "missing_capabilities", "duration_ms"], Names(results[2]));
        Assert.Equal(["pass", "fail", "skip", "error", "pass"], results.Select(result => result.GetProperty("outcome").GetString()));
        Assert.Equal(12.5, results[0].GetProperty("duration_ms").GetDouble());

        var fallback = withheld.GetProperty("protocols")[0].GetProperty("results")[0];
        Assert.Equal(["id", "outcome", "rpc", "reason", "status", "without_capability", "missing_capabilities", "duration_ms"], Names(fallback));
        Assert.True(fallback.GetProperty("without_capability").GetBoolean());
    }

    [Fact]
    public void TheJsonReportIsIndentedAndEndsWithANewline()
    {
        var text = Encoding.UTF8.GetString(Json(new ConformanceReport("s", [])));

        Assert.Equal("{\n  \"service\": \"s\",\n  \"total\": 0,\n  \"passed\": 0,\n  \"failed\": 0,\n  \"skipped\": 0,\n  \"errored\": 0,\n  \"without_capability\": 0,\n  \"configurations\": []\n}\n", text);
    }

    [Fact]
    public void TheJUnitReportHasOneSuitePerConfigurationAndProtocol()
    {
        var report = new ConformanceReport("svc",
        [
            new ConfigurationSummary("default", [],
            [
                new ProtocolSummary("grpc", "svc", [],
                [
                    Result("parse-01", ScenarioOutcome.Pass, durationMs: 12),
                    Result("verify-02", ScenarioOutcome.Fail, reason: "response mismatch", failures: ["status = INVALID_ARGUMENT, want OK", "second"]),
                    Result("query-03", ScenarioOutcome.Skip, reason: "missing capability query"),
                    Result("bad-04", ScenarioOutcome.Error, reason: "unknown RPC"),
                ]),
                new ProtocolSummary("connect", "svc", [], [Result("parse-01", ScenarioOutcome.Pass)]),
            ]),
            new ConfigurationSummary("without-query", ["query"], [new ProtocolSummary("grpc", "svc", [], [Result("parse-01", ScenarioOutcome.Pass)])]),
        ]);

        var document = XDocument.Parse(Encoding.UTF8.GetString(JUnit(report)));
        var root = document.Root!;

        Assert.Equal("testsuites", root.Name.LocalName);
        Assert.Equal(("conformance", "6", "1", "1", "1"), ((string)root.Attribute("name")!, (string)root.Attribute("tests")!, (string)root.Attribute("failures")!, (string)root.Attribute("errors")!, (string)root.Attribute("skipped")!));
        var suites = root.Elements("testsuite").ToList();
        Assert.Equal(["default/grpc", "default/connect", "without-query/grpc"], suites.Select(suite => (string)suite.Attribute("name")!));
        Assert.Equal("4", (string)suites[0].Attribute("tests")!);

        var cases = suites[0].Elements("testcase").ToList();
        Assert.Equal(("parse-01", "default/grpc", "0.012"), ((string)cases[0].Attribute("name")!, (string)cases[0].Attribute("classname")!, (string)cases[0].Attribute("time")!));
        Assert.Empty(cases[0].Elements());
        var failure = Assert.Single(cases[1].Elements("failure"));
        Assert.Equal(("response mismatch", "status = INVALID_ARGUMENT, want OK\nsecond"), ((string)failure.Attribute("message")!, failure.Value));
        Assert.Equal("missing capability query", (string)Assert.Single(cases[2].Elements("skipped")).Attribute("message")!);
        Assert.Equal("unknown RPC", (string)Assert.Single(cases[3].Elements("error")).Attribute("message")!);
    }

    [Fact]
    public void TheJUnitReportDeclaresUtf8AndEndsWithANewline()
    {
        var text = Encoding.UTF8.GetString(JUnit(new ConformanceReport("s", [])));

        Assert.Equal("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<testsuites name=\"conformance\" tests=\"0\" failures=\"0\" errors=\"0\" skipped=\"0\" time=\"0\" />\n", text);
    }

    [Theory]
    [InlineData("\0", "\uFFFD")]
    [InlineData("\u0001", "\uFFFD")]
    [InlineData("\u000B", "\uFFFD")]
    [InlineData("\u001F", "\uFFFD")]
    [InlineData("\uFFFE", "\uFFFD")]
    [InlineData("\uFFFF", "\uFFFD")]
    [InlineData("\t", "\t")]
    [InlineData("\n", "\n")]
    [InlineData("\r", "\r")]
    [InlineData("\U0001F600", "\U0001F600")]
    public void JUnitReplacesOnlyXmlInvalidCharactersInTextAndAttributes(string character, string replacement)
    {
        AssertJUnitCharacters(character, replacement);
    }

    [Fact]
    public void JUnitReplacesUnpairedSurrogates()
    {
        AssertJUnitCharacters("\uD800", "\uFFFD");
        AssertJUnitCharacters("\uDC00", "\uFFFD");
    }

    private static void AssertJUnitCharacters(string character, string replacement)
    {
        var raw = $"before{character}after";
        var safe = $"before{replacement}after";
        var report = new ConformanceReport("service",
        [
            new ConfigurationSummary(raw, [],
            [
                new ProtocolSummary(raw, "service", [], [Result(raw, ScenarioOutcome.Fail, raw, [raw])]),
            ]),
        ]);

        var document = XDocument.Parse(Encoding.UTF8.GetString(JUnit(report)));
        var suite = Assert.Single(document.Root!.Elements("testsuite"));
        var test = Assert.Single(suite.Elements("testcase"));
        var failure = Assert.Single(test.Elements("failure"));

        Assert.Equal($"{safe}/{safe}", (string?)suite.Attribute("name"));
        Assert.Equal($"{safe}/{safe}", (string?)test.Attribute("classname"));
        Assert.Equal(safe, (string?)test.Attribute("name"));
        Assert.Equal(safe, (string?)failure.Attribute("message"));
        Assert.Equal(safe, failure.Value);
    }

    [Fact]
    public async Task ARefusalWithANulCharacterStillProducesAReadableJUnitReport()
    {
        var client = new FakeScenarioClient
        {
            OnCall = (_, _, _) => Task.FromResult<ScenarioAnswer>(new ScenarioAnswer.Refused(StatusCode.InvalidArgument, "bad character \0")),
        };
        var result = await new ScenarioRunner(client, new FixtureDirectory(Path.GetTempPath())).RunAsync(
            Trees.Scenario("""{"id":"refused","rpc":"GetServerInfo"}"""), CancellationToken.None);
        var report = new ConformanceReport("service",
            [new ConfigurationSummary("default", [], [new ProtocolSummary("fake", "service", [], [result])])]);

        Assert.Equal(ScenarioOutcome.Fail, result.Outcome);
        var document = XDocument.Parse(Encoding.UTF8.GetString(JUnit(report)));
        Assert.Contains("bad character \uFFFD", document.Descendants("failure").Single().Value, StringComparison.Ordinal);
        using var json = JsonDocument.Parse(Json(report));
        var failure = json.RootElement.GetProperty("configurations")[0].GetProperty("protocols")[0]
            .GetProperty("results")[0].GetProperty("failures")[0].GetString()!;
        Assert.Contains("bad character \0", failure, StringComparison.Ordinal);
    }

    [Fact]
    public async Task AWholeRunMatchesItsGoldens()
    {
        var log = new StringWriter();
        var report = await GoldenRun(log);

        AssertGolden("report.json", Encoding.UTF8.GetString(Json(report)));
        AssertGolden("report.xml", Encoding.UTF8.GetString(JUnit(report)));
        AssertGolden("run.log", log.ToString());
    }

    [Fact]
    public async Task AWholeRunIsReproducible()
    {
        var first = await GoldenRun(new StringWriter());
        var second = await GoldenRun(new StringWriter());

        Assert.Equal(Json(first), Json(second));
        Assert.Equal(JUnit(first), JUnit(second));
    }

    /// <summary>
    /// Runs testdata/suite against a scripted service under two configurations, as a run that
    /// withholds strict_conformance and oslc_query does: every outcome, a fallback, labelled ids and
    /// a tolerance-equal Real appear, in file order (1, 10, 2: by name, not by number).
    /// </summary>
    private static async Task<ConformanceReport> GoldenRun(StringWriter log)
    {
        var scenarios = ScenarioSuite.Load(Path.Combine(TestData, "suite", "scenarios"));
        var fixtures = new FixtureDirectory(Path.Combine(TestData, "suite", "fixtures"));
        string[] all = ["strict_conformance", "query", "evaluate", "oslc_query"];
        (string Name, string[] Unavailable, string[] Protocols)[] configurations =
        [
            ("default", [], ["fake-proto", "fake-json"]),
            ("without-strict_conformance-oslc_query", ["strict_conformance", "oslc_query"], ["fake-proto"]),
        ];

        var summaries = new List<ConfigurationSummary>();
        foreach (var (name, unavailable, protocols) in configurations)
        {
            log.Write($"\nConfiguration {name}\n");
            var capabilities = all.Except(unavailable).ToList();
            var results = new List<ProtocolSummary>();
            foreach (var protocol in protocols)
            {
                var client = GoldenClient(protocol, capabilities);
                var runner = new ScenarioRunner(client, fixtures, new ScenarioRunnerOptions
                {
                    Verbose = true,
                    OmitHandshakeScenarios = unavailable.Length > 0,
                    Output = log,
                    ScenarioLog = log,
                    TimeProvider = new StepClock(TimeSpan.FromTicks(12_345)),
                });
                results.Add(await runner.RunAllAsync(scenarios, filter: null, CancellationToken.None));
            }

            summaries.Add(new ConfigurationSummary(name, unavailable, results));
        }

        return new ConformanceReport("fake-service", summaries);
    }

    private static FakeScenarioClient GoldenClient(string protocol, IReadOnlyList<string> capabilities) => new()
    {
        Protocol = protocol,
        Service = "fake-service",
        Capabilities = capabilities,
        OnParse = (model, documents, _) => Task.FromResult(model.Fixture == "broken.sysml"
            ? new ModelParse("", "the fake service refuses this model", [])
            : new ModelParse($"hash-{documents[0].Content.Length}", "", [new("warning", "unused")])),
        OnCall = (method, request, _) => Task.FromResult(Answer(method, request, capabilities)),
    };

    private static ScenarioAnswer Answer(MethodDescriptor method, Google.Protobuf.IMessage request, IReadOnlyList<string> capabilities) => method.Name switch
    {
        "GetServerInfo" => new ScenarioAnswer.Answered(new Wire.ServerInfoResponse { Version = "0.0.0-fake", Capabilities = { capabilities } }),
        "Evaluate" => new ScenarioAnswer.Answered(new Wire.EvaluateResponse { Result = new Wire.Value { RealValue = 1500.0000000001 } }),
        "Instantiate" => new ScenarioAnswer.Answered(new Wire.InstantiateResponse
        {
            Instance = new Wire.Instance
            {
                Id = 41,
                TypeSymbolId = "Demo::Car",
                FeatureValues = { ["engine"] = new Wire.FeatureValue { Value = new Wire.Value { InstanceId = 77 } } },
            },
            Instances = { new Wire.Instance { Id = 41 }, new Wire.Instance { Id = 77 } },
        }),
        "ParseFile" when ((Wire.ParseFileRequest)request).StrictConformance && !capabilities.Contains("strict_conformance") =>
            new ScenarioAnswer.Refused(StatusCode.Unimplemented, "strict_conformance is not available"),
        "ParseFile" => new ScenarioAnswer.Answered(new Wire.ParseFileResponse { ModelHash = "hash-strict" }),
        "GetSymbol" => new ScenarioAnswer.Refused(StatusCode.NotFound, "no symbol Demo::Missing"),
        "Convert" => new ScenarioAnswer.Unexpressible("the fake client cannot express Convert"),
        _ => new ScenarioAnswer.Answered(method.OutputType.Parser.ParseFrom(Google.Protobuf.ByteString.Empty)),
    };

    private static void AssertGolden(string name, string actual)
    {
        var path = Path.Combine(TestData, "golden", name);
        if (Environment.GetEnvironmentVariable("OPENSYSML_UPDATE_GOLDENS") == "1")
        {
            File.WriteAllText(path, actual, new UTF8Encoding(encoderShouldEmitUTF8Identifier: false));
            return;
        }

        Assert.True(File.Exists(path), $"{path} does not exist; run with OPENSYSML_UPDATE_GOLDENS=1 to write it.");
        Assert.Equal(File.ReadAllText(path).ReplaceLineEndings("\n"), actual);
    }

    private static ConformanceReport SampleReport() => new("bin/sysml-grpc",
    [
        new ConfigurationSummary("default", [],
        [
            new ProtocolSummary("grpc", "bin/sysml-grpc", ["query"],
            [
                Result("a", ScenarioOutcome.Pass, durationMs: 12.5),
                Result("b", ScenarioOutcome.Fail, failures: ["x: 1, want 2"]),
                Result("c", ScenarioOutcome.Skip, reason: "the service does not report convert", status: "-", missing: ["convert"]),
                Result("d", ScenarioOutcome.Error, reason: "sysml.SysMLService has no RPC \"Forecast\"", status: "-"),
                Result("e", ScenarioOutcome.Pass),
            ]),
        ]),
        new ConfigurationSummary("without-oslc_query", ["oslc_query"],
        [
            new ProtocolSummary("grpc", "bin/sysml-grpc", [],
            [
                Result("f", ScenarioOutcome.Pass, reason: "the service does not report oslc_query, so the without-capability expectation applies", missing: ["oslc_query"], withoutCapability: true),
            ]),
        ]),
    ]);

    private static ScenarioResult Result(
        string id,
        ScenarioOutcome outcome,
        string reason = "",
        IReadOnlyList<string>? failures = null,
        string status = "OK",
        IReadOnlyList<string>? missing = null,
        bool withoutCapability = false,
        double durationMs = 0) =>
        new(id, outcome, "Evaluate", reason, failures ?? [], status, withoutCapability, missing ?? [], durationMs);

    private static byte[] Json(ConformanceReport report)
    {
        using var stream = new MemoryStream();
        ReportJson.Write(report, stream);
        return stream.ToArray();
    }

    private static byte[] JUnit(ConformanceReport report)
    {
        using var stream = new MemoryStream();
        JUnitReport.Write(report, stream);
        return stream.ToArray();
    }

    private static List<string> Names(JsonElement element) => [.. element.EnumerateObject().Select(member => member.Name)];

    private static (int, int, int, int, int, int) Totals(JsonElement element) => (
        element.GetProperty("total").GetInt32(),
        element.GetProperty("passed").GetInt32(),
        element.GetProperty("failed").GetInt32(),
        element.GetProperty("skipped").GetInt32(),
        element.GetProperty("errored").GetInt32(),
        element.GetProperty("without_capability").GetInt32());
}
