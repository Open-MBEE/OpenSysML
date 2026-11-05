using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Text.Json;
using OpenMBEE.OpenSysML.Tests;
using Xunit;
using static OpenMBEE.OpenSysML.Conformance.Tests.Trees;

namespace OpenMBEE.OpenSysML.Conformance.Tests;

/// <summary>Reading the scenario files: their order, and every way a malformed one is refused.</summary>
[Trait(Category.Name, Category.Unit)]
public sealed class ScenarioSuiteTests
{
    private static readonly string Scenarios = Repository.PathOf("conformance", "scenarios");
    private static readonly string Fixtures = Repository.PathOf("conformance", "fixtures");

    [Fact]
    public void ScenariosArriveInFileNameThenDeclarationOrder()
    {
        using var suite = new TempDirectory();
        suite.Write("9-a.json", Suite(Minimal("a1")));
        suite.Write("10-b.json", Suite(Minimal("b1"), Minimal("b2")));
        suite.Write("B.json", Suite(Minimal("c")));
        suite.Write("a.json", Suite(Minimal("d")));
        suite.Write("notes.txt", "not a scenario file");
        suite.Write("upper.JSON", Suite(Minimal("ignored")));
        suite.Write("nested/deeper.json", Suite(Minimal("nested")));

        var scenarios = ScenarioSuite.Load(suite.Path);

        Assert.Equal(["b1", "b2", "a1", "c", "d"], scenarios.Select(scenario => scenario.Id));
        Assert.Equal(Path.Combine(suite.Path, "10-b.json"), scenarios[0].File);
    }

    [Fact]
    public void EveryMemberIsRead()
    {
        var scenario = Scenario("""
            {
              "id": "evaluate/subject",
              "description": "why",
              "rpc": "sysml.SysMLService/Evaluate",
              "requires_capabilities": ["evaluate_subject"],
              "model": {"fixtures": ["a.sysml", "b.sysml"], "language": "kerml", "strict_conformance": true},
              "request": {"model_hash": "${model_hash}", "expression": "mass"},
              "expect": {
                "status": "NOT_FOUND", "status_message_contains": "mass", "response": {"error": ""},
                "contains": {"error": "x"}, "contains_all": {"diagnostics.*.message": ["y"]},
                "non_empty": ["a"], "absent": ["b"], "counts": {"c": 2}, "min_counts": {"d": 1}
              },
              "expect_without_capability": {"status": "UNIMPLEMENTED", "status_message_contains": "evaluate_subject"}
            }
            """);

        Assert.Equal(("evaluate/subject", "why", "Evaluate"), (scenario.Id, scenario.Description, scenario.Method));
        Assert.Equal(["evaluate_subject"], scenario.RequiresCapabilities);
        Assert.Equal(new ScenarioModel("", ["a.sysml", "b.sysml"], "kerml", true).Key, scenario.Model!.Key);
        Assert.Equal("mass", scenario.Request!.Value.GetProperty("expression").GetString());
        var expect = scenario.Expect;
        Assert.Equal(("NOT_FOUND", "mass"), (expect.Status, expect.StatusMessageContains));
        Assert.Equal(JsonValueKind.Object, expect.Response!.Value.ValueKind);
        Assert.Equal("x", expect.Contains["error"]);
        Assert.Equal(["y"], expect.ContainsAll["diagnostics.*.message"]);
        Assert.Equal(["a"], expect.NonEmpty);
        Assert.Equal(["b"], expect.Absent);
        Assert.Equal((2, 1), (expect.Counts["c"], expect.MinCounts["d"]));
        Assert.Equal("UNIMPLEMENTED", scenario.ExpectWithoutCapability!.WantStatus);
    }

    [Fact]
    public void AFileMayDeclareNoScenarios()
    {
        using var suite = new TempDirectory();
        suite.Write("a.json", "{}");
        suite.Write("b.json", Suite(Minimal("b")));
        suite.Write("c.json", """{"scenarios": null}""");

        Assert.Equal(["b"], ScenarioSuite.Load(suite.Path).Select(scenario => scenario.Id));
    }

    [Fact]
    public void AFalseStrictConformanceIsTheDefaultParse()
    {
        var model = Scenario("""{"id": "a", "rpc": "Evaluate", "model": {"fixture": "x.sysml", "strict_conformance": false}}""").Model!;

        Assert.False(model.StrictConformance);
        Assert.Equal(new ScenarioModel("x.sysml", [], "", false).Key, model.Key);
    }

    [Fact]
    public void AnOmittedOrNullMemberIsAbsent()
    {
        var omitted = Scenario("""{"id": "a", "rpc": "GetServerInfo"}""");
        var nulled = Scenario("""{"id": "a", "rpc": "GetServerInfo", "model": null, "request": null, "expect": null, "expect_without_capability": null, "requires_capabilities": null}""");

        foreach (var scenario in new[] { omitted, nulled })
        {
            Assert.Null(scenario.Model);
            Assert.Null(scenario.Request);
            Assert.Null(scenario.ExpectWithoutCapability);
            Assert.Empty(scenario.RequiresCapabilities);
            Assert.Same(Expectation.Success, scenario.Expect);
            Assert.Equal("OK", scenario.Expect.WantStatus);
        }
    }

    [Theory]
    [InlineData("""{"scenarios": [], "extra": 1}""", "the file: unknown member \"extra\"")]
    [InlineData("""{"scenarios": [{"id": "a", "rpc": "Evaluate", "expect": {"respones": {}}}]}""", "a: expect: unknown member \"respones\"")]
    [InlineData("""{"scenarios": [{"id": "a", "rpc": "Evaluate", "expect_without_capability": {"stauts": "X"}}]}""", "unknown member \"stauts\"")]
    [InlineData("""{"scenarios": [{"id": "a", "rpc": "Evaluate", "model": {"fixture": "x", "lang": "kerml"}}]}""", "a: model: unknown member \"lang\"")]
    [InlineData("""{"scenarios": [{"id": "a", "rpc": "Evaluate", "requires": []}]}""", "scenario 0: unknown member \"requires\"")]
    public void AnUnknownMemberIsRefused(string file, string problem)
    {
        AssertRefused(file, problem);
    }

    [Theory]
    [InlineData("""{"scenarios": [{"id": "a", "id": "b", "rpc": "Evaluate"}]}""", "member \"id\" is stated twice")]
    [InlineData("""{"scenarios": [{"id": "a", "rpc": "Evaluate", "expect": {"response": {"x": {"y": 1, "y": 2}}}}]}""", "a: expect: response.x: member \"y\" is stated twice")]
    [InlineData("""{"scenarios": [{"id": "a", "rpc": "Evaluate", "expect": {"counts": {"p": 1, "p": 2}}}]}""", "member \"p\" is stated twice")]
    public void AMemberStatedTwiceIsRefused(string file, string problem)
    {
        AssertRefused(file, problem);
    }

    [Theory]
    [InlineData("""{"scenarios": {}}""", "scenarios is an object, want a list")]
    [InlineData("""{"scenarios": [[]]}""", "scenario 0 is a list, want an object")]
    [InlineData("""{"scenarios": [{"id": 5, "rpc": "Evaluate"}]}""", "scenario 0: id is a number, want text")]
    [InlineData("""{"scenarios": [{"id": "a", "rpc": "Evaluate", "requires_capabilities": "query"}]}""", "a: requires_capabilities is text, want a list of text")]
    [InlineData("""{"scenarios": [{"id": "a", "rpc": "Evaluate", "requires_capabilities": ["query", 1]}]}""", "a: requires_capabilities.1 is a number, want text")]
    [InlineData("""{"scenarios": [{"id": "a", "rpc": "Evaluate", "model": {"fixture": "x", "strict_conformance": "yes"}}]}""", "strict_conformance is text, want true or false")]
    [InlineData("""{"scenarios": [{"id": "a", "rpc": "Evaluate", "expect": {"status": 404}}]}""", "a: expect: status is a number, want text")]
    [InlineData("""{"scenarios": [{"id": "a", "rpc": "Evaluate", "expect": {"response": []}}]}""", "a: expect: response is a list, want an object")]
    [InlineData("""{"scenarios": [{"id": "a", "rpc": "Evaluate", "expect": {"counts": {"p": 2.5}}}]}""", "a: expect: counts.p is 2.5, want a whole number")]
    [InlineData("""{"scenarios": [{"id": "a", "rpc": "Evaluate", "expect": {"min_counts": {"p": "2"}}}]}""", "a: expect: min_counts.p is \"2\", want a whole number")]
    [InlineData("""{"scenarios": [{"id": "a", "rpc": "Evaluate", "expect": {"contains_all": {"p": "x"}}}]}""", "a: expect: contains_all.p is text, want a list of text")]
    [InlineData("""{"scenarios": [{"id": "a", "rpc": "Evaluate", "expect": {"contains": {"p": ["x"]}}}]}""", "a: expect: contains.p is a list, want text")]
    [InlineData("""{"scenarios": [{"id": "a", "rpc": "Evaluate", "expect": []}]}""", "a: expect is a list, want an object")]
    [InlineData("""[]""", "the file is a list, want an object")]
    [InlineData("""{"scenarios": [{"id": true, "rpc": "Evaluate"}]}""", "scenario 0: id is a boolean, want text")]
    [InlineData("""{"scenarios": [{"id": "a", "rpc": "Evaluate", "expect": {"non_empty": [null]}}]}""", "a: expect: non_empty.0 is null, want text")]
    public void AValueOfTheWrongKindIsRefused(string file, string problem)
    {
        AssertRefused(file, problem);
    }

    [Theory]
    [InlineData("""{"scenarios": [{"id": "a", "rpc": "Evaluate",}]}""")]
    [InlineData("""{"scenarios": [{"id": "a", /* note */ "rpc": "Evaluate"}]}""")]
    [InlineData("""{"scenarios": [""")]
    public void AFileThatIsNotStrictJsonIsRefused(string file)
    {
        using var suite = new TempDirectory();
        var path = suite.Write("s.json", file);

        var error = Assert.Throws<InvalidDataException>(() => ScenarioSuite.Load(suite.Path));
        Assert.StartsWith(path + ": ", error.Message, StringComparison.Ordinal);
    }

    [Theory]
    [InlineData("""{"scenarios": [{"id": "a"}]}""")]
    [InlineData("""{"scenarios": [{"rpc": "Evaluate"}]}""")]
    [InlineData("""{"scenarios": [{"id": "", "rpc": "Evaluate"}]}""")]
    public void AScenarioNeedsAnIdAndAnRpc(string file)
    {
        AssertRefused(file, "every scenario needs an id and an rpc");
    }

    [Theory]
    [InlineData("""{}""", "a: model: a model needs a fixture or fixtures")]
    [InlineData("""{"fixture": "", "fixtures": []}""", "a: model: a model needs a fixture or fixtures")]
    [InlineData("""{"fixture": "x.sysml", "fixtures": ["y.sysml"]}""", "a: model: a model names either a fixture or fixtures, not both")]
    public void AModelNamesExactlyOneForm(string model, string problem)
    {
        AssertRefused($$"""{"scenarios": [{"id": "a", "rpc": "Evaluate", "model": {{model}}}]}""", problem);
    }

    [Fact]
    public void AnIdDeclaredTwiceIsRefusedNamingBothFiles()
    {
        using var suite = new TempDirectory();
        var first = suite.Write("01.json", Suite(Minimal("a")));
        var second = suite.Write("02.json", Suite(Minimal("a")));

        var error = Assert.Throws<InvalidDataException>(() => ScenarioSuite.Load(suite.Path));
        Assert.Equal($"{second}: scenario id \"a\" is already declared in {first}", error.Message);

        using var single = new TempDirectory();
        single.Write("s.json", Suite(Minimal("b"), Minimal("b")));
        Assert.Contains("scenario id \"b\" is already declared", Assert.Throws<InvalidDataException>(() => ScenarioSuite.Load(single.Path)).Message, StringComparison.Ordinal);
    }

    [Fact]
    public void ASuiteWithoutScenarioFilesIsRefused()
    {
        using var empty = new TempDirectory();
        Assert.Equal($"no scenario files in {empty.Path}", Assert.Throws<InvalidDataException>(() => ScenarioSuite.Load(empty.Path)).Message);

        var missing = Path.Combine(empty.Path, "missing");
        Assert.Throws<InvalidDataException>(() => ScenarioSuite.Load(missing));
    }

    [Theory]
    [InlineData("Evaluate")]
    [InlineData("sysml.SysMLService/Evaluate")]
    [InlineData("/sysml.SysMLService/Evaluate")]
    public void TheMethodIsTheRpcsBareName(string rpc)
    {
        Assert.Equal("Evaluate", Scenario($$"""{"id": "a", "rpc": "{{rpc}}"}""").Method);
    }

    [Fact]
    public void AModelsParseIsIdentifiedByEverythingThatChangesIt()
    {
        var one = new ScenarioModel("a.sysml", [], "", false);

        Assert.Equal(one.Key, new ScenarioModel("a.sysml", [], "", false).Key);
        Assert.NotEqual(one.Key, new ScenarioModel("a.sysml", [], "", true).Key);
        Assert.NotEqual(one.Key, new ScenarioModel("a.sysml", [], "kerml", false).Key);
        Assert.NotEqual(one.Key, new ScenarioModel("b.sysml", [], "", false).Key);
        Assert.NotEqual(one.Key, new ScenarioModel("", ["a.sysml"], "", false).Key);
        Assert.NotEqual(new ScenarioModel("", ["a.sysml", "b.sysml"], "", false).Key, new ScenarioModel("", ["b.sysml", "a.sysml"], "", false).Key);
    }

    [Fact]
    public void TheCommittedSuiteLoadsInFileThenDeclarationOrder()
    {
        var expected = new List<string>();
        foreach (var file in Directory.GetFiles(Scenarios, "*.json").Order(StringComparer.Ordinal))
        {
            using var document = JsonDocument.Parse(File.ReadAllBytes(file));
            expected.AddRange(document.RootElement.GetProperty("scenarios").EnumerateArray().Select(scenario => scenario.GetProperty("id").GetString()!));
        }

        var loaded = ScenarioSuite.Load(Scenarios);

        Assert.NotEmpty(loaded);
        Assert.Equal(expected, loaded.Select(scenario => scenario.Id));
    }

    [Fact]
    public void EveryCommittedScenarioNamesAnRpcOfTheService()
    {
        var unknown = ScenarioSuite.Load(Scenarios).Where(scenario => ScenarioRequests.Service.FindMethodByName(scenario.Method) is null).Select(scenario => scenario.Id);
        Assert.Empty(unknown);
    }

    [Fact]
    public void EveryRpcIsCoveredByACommittedScenario()
    {
        var covered = ScenarioSuite.Load(Scenarios).Select(scenario => scenario.Method).ToHashSet(StringComparer.Ordinal);
        var uncovered = ScenarioRequests.Service.Methods.Select(method => method.Name).Where(name => !covered.Contains(name)).ToList();
        Assert.True(uncovered.Count == 0, $"No scenario covers {string.Join(", ", uncovered)}.");
    }

    [Fact]
    public void TheCommittedSuiteCoversBothKindsOfFailure()
    {
        var statuses = new HashSet<string>(StringComparer.Ordinal);
        var withoutCapability = new HashSet<string>(StringComparer.Ordinal);
        var inBand = 0;
        foreach (var scenario in ScenarioSuite.Load(Scenarios))
        {
            var expects = new List<Expectation> { scenario.Expect };
            if (scenario.ExpectWithoutCapability is { } fallback)
            {
                expects.Add(fallback);
                Assert.Equal("UNIMPLEMENTED", fallback.Status);
                Assert.Contains(fallback.StatusMessageContains, scenario.RequiresCapabilities);
                withoutCapability.Add(fallback.StatusMessageContains);
            }

            foreach (var expect in expects)
            {
                if (expect.Status.Length > 0)
                {
                    statuses.Add(expect.Status);
                }

                inBand += expect.NonEmpty.Concat(expect.Contains.Keys).Count(path => path.EndsWith("error", StringComparison.Ordinal));
            }
        }

        Assert.Superset(new HashSet<string>(["INVALID_ARGUMENT", "NOT_FOUND", "UNIMPLEMENTED"]), statuses);
        Assert.True(inBand > 0, "No scenario expects a failure reported in the response's error field.");
        Assert.Superset(new HashSet<string>(["strict_conformance", "oslc_query"]), withoutCapability);
    }

    [Fact]
    public void EveryCommittedFixtureIsRead()
    {
        var scenarios = ScenarioSuite.Load(Scenarios);
        var unread = Directory.GetFiles(Fixtures, "*.sysml")
            .Select(Path.GetFileName)
            .Where(name => !scenarios.Any(scenario =>
                (scenario.Model?.FixtureNames.Contains(name) ?? false)
                || (scenario.Request?.GetRawText().Contains("${fixture:" + name + "}", StringComparison.Ordinal) ?? false)))
            .ToList();

        Assert.NotEmpty(Directory.GetFiles(Fixtures, "*.sysml"));
        Assert.Empty(unread);
    }

    private static string Minimal(string id) => $$"""{"id": "{{id}}", "rpc": "GetServerInfo"}""";

    private static string Suite(params string[] scenarios) => $$"""{"scenarios": [{{string.Join(", ", scenarios)}}]}""";

    private static void AssertRefused(string file, string problem)
    {
        using var suite = new TempDirectory();
        var path = suite.Write("s.json", file);

        var error = Assert.Throws<InvalidDataException>(() => ScenarioSuite.Load(suite.Path));

        Assert.StartsWith(path + ": ", error.Message, StringComparison.Ordinal);
        Assert.Contains(problem, error.Message, StringComparison.Ordinal);
    }
}
