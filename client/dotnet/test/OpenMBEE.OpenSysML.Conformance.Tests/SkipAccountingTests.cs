using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Text;
using Xunit;
using static OpenMBEE.OpenSysML.Conformance.Tests.Trees;

namespace OpenMBEE.OpenSysML.Conformance.Tests;

/// <summary>
/// The expected-skips list is read strictly and held to the suite, the schema and the adapters; and a
/// run's skips are held to it: a skip it does not list, a listed skip for another reason, a listed skip
/// that ran, a capability skip no withheld capability explains, and a withheld capability whose
/// fallback did not run each fail the run.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed class SkipAccountingTests
{
    private static readonly ExpectedSkips List = Parse("""
        {"uncovered_rpcs": ["Query"], "unexpressible": {"parse/no_source": "the public API always names a source"}}
        """);

    [Fact]
    public void TheListIsReadByRpcAndById()
    {
        Assert.Equal(["Query"], List.UncoveredRpcs);
        Assert.Equal("the public API does not cover Query", List.ReasonFor("query/anything", "Query"));
        Assert.Equal("the public API always names a source", List.ReasonFor("parse/no_source", "ParseFile"));
        Assert.Null(List.ReasonFor("parse/other", "ParseFile"));
        Assert.Empty(Parse("{}").UncoveredRpcs);
    }

    [Theory]
    [InlineData("""{"uncovered": []}""", "unknown member \"uncovered\"")]
    [InlineData("""{"uncovered_rpcs": [], "uncovered_rpcs": []}""", "member \"uncovered_rpcs\" is stated twice")]
    [InlineData("""{"uncovered_rpcs": ["Query", "Query"]}""", "uncovered_rpcs lists \"Query\" twice")]
    [InlineData("""{"uncovered_rpcs": "Query"}""", "uncovered_rpcs is text, want a list")]
    [InlineData("""{"uncovered_rpcs": [1]}""", "an entry of uncovered_rpcs is a number, want text")]
    [InlineData("""{"unexpressible": {"a": "x", "a": "y"}}""", "member \"a\" is stated twice")]
    [InlineData("""{"unexpressible": {"a": ""}}""", "unexpressible gives \"a\" no reason")]
    [InlineData("""{"unexpressible": ["a"]}""", "unexpressible is a list, want an object")]
    [InlineData("""{"uncovered_rpcs": [],}""", "list.json:")]
    public void AMalformedListIsRefused(string json, string problem)
    {
        var refused = Assert.Throws<InvalidDataException>(() => Parse(json));

        Assert.Contains(problem, refused.Message, StringComparison.Ordinal);
    }

    [Fact]
    public void TheListIsHeldToTheSuiteTheSchemaAndTheAdapters()
    {
        var list = Parse("""
            {"uncovered_rpcs": ["Forecast", "Evaluate", "Query"],
             "unexpressible": {"parse/gone": "r", "query/oslc": "r"}}
            """);
        IReadOnlyList<Scenario> scenarios = [Scenario("""{"id": "query/oslc", "rpc": "Query"}""")];
        var covered = ApiAdapters.All.Keys.Where(rpc => rpc is not ("GetDiagnostics" or "Query"));

        Assert.Equal(
            [
                "the expected skips list Evaluate as uncovered, but an adapter covers it",
                "the expected skips list Forecast as uncovered, which is no RPC of sysml.SysMLService",
                .. ScenarioRequests.Service.Methods
                    .Where(method => (!ApiAdapters.All.ContainsKey(method.Name) && method.Name != "Query") || method.Name == "GetDiagnostics")
                    .Select(method => $"no adapter covers {method.Name}, and the expected skips do not list it as uncovered"),
                "the expected skips name scenario parse/gone, which the suite does not declare",
                "the expected skips name scenario query/oslc as unexpressible, but its RPC Query is listed as uncovered",
            ],
            list.Problems(scenarios, ScenarioRequests.Service, covered));
    }

    [Fact]
    public void ASkipTheListGivesForThatReasonIsAccounted()
    {
        var results = Protocol(
            Skip("parse/no_source", "ParseFile", "the public API always names a source"),
            Skip("query/a", "Query", "the public API does not cover Query"),
            Result("evaluate/a", "Evaluate", ScenarioOutcome.Pass));

        Assert.Empty(SkipAccounting.Skips("default", [], results, List, allowSkips: false));
    }

    [Fact]
    public void ASkipTheListDoesNotGiveFailsTheRunUnlessEverySkipIsAllowed()
    {
        var results = Protocol(
            Skip("evaluate/a", "Evaluate", "the public API does not cover Evaluate"),
            Skip("parse/no_source", "ParseFile", "another reason"));

        Assert.Equal(
            [
                "default grpc: evaluate/a was skipped, which the expected skips do not list: the public API does not cover Evaluate",
                "default grpc: parse/no_source was skipped because another reason, but the expected skips give the public API always names a source",
            ],
            SkipAccounting.Skips("default", [], results, List, allowSkips: false));
        Assert.Empty(SkipAccounting.Skips("default", [], results, List, allowSkips: true));
    }

    [Fact]
    public void AListedSkipThatRanFailsTheRunEvenWhenSkipsAreAllowed()
    {
        var results = Protocol(Result("parse/no_source", "ParseFile", ScenarioOutcome.Fail), Result("query/a", "Query", ScenarioOutcome.Pass));
        string[] stale =
        [
            "default grpc: parse/no_source is listed as an expected skip (the public API always names a source), but it ran and ended fail",
            "default grpc: query/a is listed as an expected skip (the public API does not cover Query), but it ran and ended pass",
        ];

        Assert.Equal(stale, SkipAccounting.Skips("default", [], results, List, allowSkips: false));
        Assert.Equal(stale, SkipAccounting.Skips("default", [], results, List, allowSkips: true));
    }

    [Fact]
    public void ACapabilitySkipIsAccountedOnlyByACapabilityTheConfigurationWithholds()
    {
        var results = Protocol(
            Skip("parse/strict", "ParseFile", "the service does not report strict_conformance", ["strict_conformance"]),
            Skip("query/oslc", "Query", "the service does not report oslc_query", ["oslc_query"]));

        Assert.Equal(
            ["without-strict grpc: query/oslc was skipped: the service does not report oslc_query"],
            SkipAccounting.Skips("without-strict", ["strict_conformance"], results, Parse("{}"), allowSkips: false));
        Assert.Equal(2, SkipAccounting.Skips("default", [], results, Parse("{}"), allowSkips: false).Count());
        Assert.Empty(SkipAccounting.Skips("default", [], results, Parse("{}"), allowSkips: true));
    }

    [Fact]
    public void AWithheldCapabilityMustHaveItsFallbackExecuted()
    {
        var executed = Protocol(Fallback("parse/strict", "ParseFile", ScenarioOutcome.Pass, "strict_conformance", "fallback"));
        var failed = Protocol(Fallback("parse/strict", "ParseFile", ScenarioOutcome.Fail, "strict_conformance", "fallback"));
        var listed = Protocol(Fallback("query/oslc", "Query", ScenarioOutcome.Skip, "oslc_query", "the public API does not cover Query"));
        var unlisted = Protocol(Fallback("parse/strict", "ParseFile", ScenarioOutcome.Skip, "strict_conformance", "the public API cannot"));
        var none = Protocol(Result("evaluate/a", "Evaluate", ScenarioOutcome.Pass));

        Assert.Equal((0, 0), Counts(SkipAccounting.Fallbacks("w", ["strict_conformance"], executed, List)));
        Assert.Equal((0, 0), Counts(SkipAccounting.Fallbacks("w", ["strict_conformance"], failed, List)));
        var (problems, notes) = SkipAccounting.Fallbacks("w", ["oslc_query"], listed, List);
        Assert.Empty(problems);
        Assert.Equal(["w grpc: no without-capability expectation of oslc_query ran; each is an expected skip: query/oslc"], notes);
        Assert.Equal(
            ["w grpc: no without-capability expectation executed for strict_conformance"],
            SkipAccounting.Fallbacks("w", ["strict_conformance"], unlisted, List).Problems);
        Assert.Equal(
            ["w grpc: no without-capability expectation executed for strict_conformance"],
            SkipAccounting.Fallbacks("w", ["strict_conformance"], none, List).Problems);
    }

    [Fact]
    public void AWithholdingServiceMustAdvertiseExactlyTheDefaultSetWithoutWhatItWithholds()
    {
        string[] defaults = ["query", "oslc_query", "strict_conformance"];

        Assert.Null(SkipAccounting.WithheldSet(defaults, ["query"], ["strict_conformance", "oslc_query"]));
        Assert.Equal(
            "capabilities = [oslc_query query strict_conformance], want the default set without [strict_conformance]: [oslc_query query]",
            SkipAccounting.WithheldSet(defaults, ["strict_conformance", "query", "oslc_query"], ["strict_conformance"]));
        Assert.Equal("the default service does not advertise \"render_view\"", SkipAccounting.WithheldSet(defaults, defaults, ["render_view"]));
    }

    private static ExpectedSkips Parse(string json) => ExpectedSkips.Parse(Encoding.UTF8.GetBytes(json), "list.json");

    private static (int, int) Counts((IReadOnlyList<string> Problems, IReadOnlyList<string> Notes) found) => (found.Problems.Count, found.Notes.Count);

    private static ProtocolSummary Protocol(params ScenarioResult[] results) => new("grpc", "svc", [], results);

    private static ScenarioResult Result(string id, string rpc, ScenarioOutcome outcome) => new(id, outcome, rpc, "", [], "OK", false, [], 1);

    private static ScenarioResult Skip(string id, string rpc, string reason, IReadOnlyList<string>? missing = null) =>
        new(id, ScenarioOutcome.Skip, rpc, reason, [], "-", false, missing ?? [], 1);

    private static ScenarioResult Fallback(string id, string rpc, ScenarioOutcome outcome, string capability, string reason) =>
        new(id, outcome, rpc, reason, [], outcome == ScenarioOutcome.Skip ? "-" : "UNIMPLEMENTED", true, [capability], 1);
}
