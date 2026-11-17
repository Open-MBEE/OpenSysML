using System;
using System.Collections.Generic;
using Xunit;
using static OpenMBEE.OpenSysML.Conformance.Tests.Trees;

namespace OpenMBEE.OpenSysML.Conformance.Tests;

/// <summary>
/// The comparison rules of conformance/README.md, each pinned by a case that must hold and one that
/// must not, so a rule that went vacuous or too strict fails here.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed class ChecksTests
{
    private static SortedDictionary<string, object?> Response() => Map(
        ("content", "part def Vehicle {\n\tattribute mass = 1500.0;\n}"),
        ("instance", Map(
            ("id", "@1"),
            ("type_symbol_id", "Demo::Vehicle"),
            ("feature_values", Map(("mass", Map(("value", Map(("real_value", 1500.0))))))))),
        ("elements", List(
            Map(("id", "Demo::vehicle"), ("type", "PartUsage")),
            Map(("id", "Demo::spare"), ("type", "PartUsage")))),
        ("capabilities", List("query", "convert")),
        ("step_count", 1_000_000_000_000L),
        ("withheld", 3UL),
        ("failure", "EDIT_FAILURE_UNKNOWN_TARGET"),
        ("experimental", true),
        ("tiny", 1e-12),
        ("labels", Map(("b", "x"), ("a", "y"))),
        ("ordinals", List(1L, 2L)),
        ("blank", ""));

    [Fact]
    public void TheResponseIsComparedOnlyWhereNamed()
    {
        Passes("""{"response": {"instance": {"type_symbol_id": "Demo::Vehicle"}}}""");
        Fails("""{"response": {"instance": {"type_symbol_id": "Demo::Car"}}}""", "instance.type_symbol_id");
    }

    [Fact]
    public void ANestedMismatchIsReportedByItsPath()
    {
        Assert.Equal(
            ["instance.feature_values.mass.value.real_value: 1500, want 1200.0"],
            Check("""{"response": {"instance": {"feature_values": {"mass": {"value": {"real_value": 1200.0}}}}}}"""));
    }

    [Fact]
    public void RealsCompareWithinARelativeToleranceOfOneInABillion()
    {
        Passes("""{"response": {"instance": {"feature_values": {"mass": {"value": {"real_value": 1500.000001}}}}}}""");
        Passes("""{"response": {"instance": {"feature_values": {"mass": {"value": {"real_value": 1500}}}}}}""");
        Fails("""{"response": {"instance": {"feature_values": {"mass": {"value": {"real_value": 1500.000003}}}}}}""", "1500.000003");
        Fails("""{"response": {"instance": {"feature_values": {"mass": {"value": {"real_value": 1500.1}}}}}}""", "1500.1");
    }

    [Fact]
    public void TheRealToleranceIsRelativeNotAbsolute()
    {
        Passes("""{"response": {"tiny": 1.0000000005e-12}}""");
        // Within any absolute tolerance near 1e-9, yet half again as large.
        Fails("""{"response": {"tiny": 1.5e-12}}""", "tiny");
    }

    [Fact]
    public void IntegersCompareExactly()
    {
        Passes("""{"response": {"step_count": 1000000000000.0, "withheld": 3.0}}""");
        // Within 1e-9 of the actual value, so a Real's tolerance would accept it.
        Fails("""{"response": {"step_count": 1000000000500.0}}""", "step_count: 1000000000000, want 1000000000500.0");
        Fails("""{"response": {"withheld": 4}}""", "withheld: 3, want 4");
    }

    [Fact]
    public void IntegersCompareByTheirDigits()
    {
        var huge = Map(("step_count", 9_007_199_254_740_993L), ("unsigned", ulong.MaxValue));
        Assert.Empty(Checks.Check(Expect("""{"response": {"step_count": 9007199254740993, "unsigned": 18446744073709551615}}"""), huge));
        Assert.NotEmpty(Checks.Check(Expect("""{"response": {"step_count": 9007199254740992}}"""), huge));
        Assert.NotEmpty(Checks.Check(Expect("""{"response": {"unsigned": 18446744073709551614}}"""), huge));
    }

    [Fact]
    public void AWholeNumberMayBeSpelledAsAReal()
    {
        Passes("""{"response": {"step_count": 1e12, "withheld": 3e0}}""");
        Fails("""{"response": {"step_count": 1000000000000.5}}""", "step_count");
    }

    [Theory]
    [InlineData("9007199254740993.0", 9_007_199_254_740_993L)]
    [InlineData("90071992547409930e-1", 9_007_199_254_740_993L)]
    [InlineData("-9007199254740993e0", -9_007_199_254_740_993L)]
    [InlineData("-9223372036854775808.0", long.MinValue)]
    [InlineData("9223372036854775807e0", long.MaxValue)]
    [InlineData("0e99999999999999999999", 0L)]
    public void SignedWholeNumberSpellingsKeepEveryDigit(string literal, long actual)
    {
        Assert.Empty(Checks.Check(Expect("""{"response":{"n":""" + literal + "}}"), Map(("n", actual))));
    }

    [Theory]
    [InlineData("18446744073709551615.0")]
    [InlineData("184467440737095516150e-1")]
    [InlineData("0.18446744073709551615e20")]
    public void UnsignedWholeNumberSpellingsKeepEveryDigit(string literal)
    {
        Assert.Empty(Checks.Check(Expect("""{"response":{"n":""" + literal + "}}"), Map(("n", ulong.MaxValue))));
    }

    [Theory]
    [InlineData("9007199254740993.0")]
    [InlineData("9007199254740993e0")]
    [InlineData("9007199254740992.5")]
    [InlineData("90071992547409925e-1")]
    public void AnIntegralExpectationNeverMatchesItsRoundedValue(string literal)
    {
        Assert.NotEmpty(Checks.Check(Expect("""{"response":{"n":""" + literal + "}}"), Map(("n", 9_007_199_254_740_992L))));
    }

    [Theory]
    [InlineData("18446744073709551616.0")]
    [InlineData("1e2147483648")]
    [InlineData("-1e2147483648")]
    [InlineData("1e-2147483649")]
    public void AnOutOfRangeIntegralExpectationNeverMatches(string literal)
    {
        Assert.NotEmpty(Checks.Check(Expect("""{"response":{"n":""" + literal + "}}"), Map(("n", ulong.MaxValue))));
    }

    [Theory]
    [InlineData(double.PositiveInfinity)]
    [InlineData(double.NegativeInfinity)]
    [InlineData(double.NaN)]
    public void ASpecialRealNeverMatchesAFiniteNumber(double actual)
    {
        Assert.NotEmpty(Checks.Check(Expect("""{"response":{"n":1}}"""), Map(("n", actual))));
        var spelling = System.Text.Json.JsonSerializer.Serialize(Spelling.Real(actual));
        Assert.Empty(Checks.Check(Expect("""{"response":{"n":""" + spelling + "}}"), Map(("n", actual))));
    }

    [Theory]
    [InlineData("1e400", double.PositiveInfinity)]
    [InlineData("-1e400", double.NegativeInfinity)]
    [InlineData("1e-400", 0.0)]
    public void ARealExpectationIsNotRoundedToInfinityOrZero(string literal, double actual)
    {
        Assert.NotEmpty(Checks.Check(Expect("""{"response":{"n":""" + literal + "}}"), Map(("n", actual))));
    }

    [Fact]
    public void ADefaultExpectationMustBeExactlyZeroNotAnUnderflow()
    {
        Assert.NotEmpty(Checks.Check(Expect("""{"response":{"missing":1e-400}}"""), Map()));
        Assert.Empty(Checks.Check(Expect("""{"response":{"missing":-0.0e99999999999999999999}}"""), Map()));
    }

    [Fact]
    public void ANumberExpectationRejectsANonNumber()
    {
        Fails("""{"response": {"content": 1.0}}""", "want the number 1.0");
        Fails("""{"response": {"failure": 2}}""", "want the number 2");
    }

    [Fact]
    public void ADefaultExpectationMatchesAnUnsetField()
    {
        Passes("""{"response": {"error": "", "flag": false, "count": 0, "real": 0.0, "items": [], "nested": {}, "nothing": null}}""");
        Fails("""{"response": {"error": "boom"}}""", "error: not set, want \"boom\"");
        Fails("""{"response": {"flag": true}}""", "flag: not set, want true");
        Fails("""{"response": {"count": 2}}""", "count: not set, want 2");
        Fails("""{"response": {"items": ["x"]}}""", "items: not set, want [\"x\"]");
        Fails("""{"response": {"nested": {"a": 1}}}""", "nested: not set, want {a: 1}");
    }

    [Fact]
    public void ADefaultExpectationDoesNotMatchASetField()
    {
        Fails("""{"response": {"content": ""}}""", "content");
        Fails("""{"response": {"experimental": false}}""", "experimental: true, want false");
    }

    [Fact]
    public void ANamedListMustHaveExactlyTheNamedEntries()
    {
        Passes("""{"response": {"elements": [{"id": "Demo::vehicle"}, {"id": "Demo::spare"}]}}""");
        Fails("""{"response": {"elements": [{"id": "Demo::vehicle"}]}}""", "elements: 2 entries, want 1");
        Fails("""{"response": {"elements": [{"id": "Demo::vehicle"}, {"id": "Demo::spare"}, {"id": "Demo::third"}]}}""", "elements: 2 entries, want 3");
    }

    [Fact]
    public void AListIsComparedInOrder()
    {
        Passes("""{"response": {"capabilities": ["query", "convert"]}}""");
        Fails("""{"response": {"capabilities": ["convert", "query"]}}""", "capabilities.0");
    }

    [Fact]
    public void AnObjectOrListMustMeetItsLike()
    {
        Fails("""{"response": {"elements": {}}}""", "want an object");
        Fails("""{"response": {"instance": []}}""", "want a list");
    }

    [Fact]
    public void AWildcardTakesOneFieldFromEveryEntryOfAListOrMap()
    {
        Passes("""{"contains_all": {"elements.*.id": ["Demo::vehicle", "Demo::spare"], "labels.*": ["x", "y"]}}""");
        Fails("""{"contains_all": {"elements.*.id": ["Demo::missing"]}}""", "Demo::missing");
    }

    [Fact]
    public void AWildcardReachesNothingUnlessEveryEntryHasTheField()
    {
        var partial = Map(("elements", List(Map(("id", "a")), Map(("type", "b")))));
        Assert.Equal(
            ["elements.*.id: neither text nor a list, want it to contain [a]"],
            Checks.Check(Expect("""{"contains_all": {"elements.*.id": ["a"]}}"""), partial));
        Assert.Empty(Checks.Check(Expect("""{"contains_all": {"elements.*.type": []}}"""), Map(("elements", List(Map(("type", "b")))))));
        Fails("""{"contains_all": {"content.*": ["part"]}}""", "neither text nor a list");
    }

    [Fact]
    public void ContainsAllReadsTextAsSubstrings()
    {
        Passes("""{"contains_all": {"content": ["part def Vehicle", "attribute mass"]}}""");
        Fails("""{"contains_all": {"content": ["part def Wheel"]}}""", "content: does not contain \"part def Wheel\"");
    }

    [Fact]
    public void ContainsAllReadsAListAsTextMembers()
    {
        Passes("""{"contains_all": {"capabilities": ["convert", "query"]}}""");
        Fails("""{"contains_all": {"capabilities": ["validate"]}}""", "capabilities: [\"query\", \"convert\"] does not contain \"validate\"");
        Fails("""{"contains_all": {"ordinals": ["1"]}}""", "ordinals: [1, 2] does not contain \"1\"");
    }

    [Fact]
    public void ContainsReadsText()
    {
        Passes("""{"contains": {"content": "attribute mass"}}""");
        Fails("""{"contains": {"content": "attribute speed"}}""", "does not contain \"attribute speed\"");
        Fails("""{"contains": {"step_count": "1"}}""", "step_count: 1000000000000 is not text, want it to contain \"1\"");
        Fails("""{"contains": {"error": "x"}}""", "error: not set, want it to contain \"x\"");
    }

    [Fact]
    public void CountsAreExactAndMinCountsALowerBound()
    {
        Passes("""{"counts": {"elements": 2, "instance.feature_values": 1, "labels": 2}, "min_counts": {"elements": 1, "capabilities": 2}}""");
        Fails("""{"counts": {"elements": 3}}""", "elements: 2 entries, want 3");
        Fails("""{"counts": {"instance.feature_values": 0}}""", "instance.feature_values: 1 entries, want 0");
        Fails("""{"min_counts": {"elements": 5}}""", "elements: 2 entries, want at least 5");
    }

    [Fact]
    public void NothingAtACountedPathIsZeroEntries()
    {
        Passes("""{"counts": {"diagnostics": 0}}""");
        Fails("""{"counts": {"diagnostics": 2}}""", "diagnostics: 0 entries, want 2");
        Fails("""{"min_counts": {"diagnostics": 1}}""", "diagnostics: 0 entries, want at least 1");
    }

    [Fact]
    public void OnlyAListOrMapIsCounted()
    {
        Fails("""{"counts": {"content": 1}}""", "content: not a list or map, want 1 entries");
        Fails("""{"min_counts": {"content": 1}}""", "content: not a list or map, want at least 1 entries");
    }

    [Fact]
    public void NonEmptyAndAbsentReadTheDefaultAsUnset()
    {
        Passes("""{"non_empty": ["instance.type_symbol_id", "elements", "instance.feature_values.mass"], "absent": ["error", "instance.feature_values.mass.error", "blank"]}""");
        Fails("""{"non_empty": ["error"]}""", "error: not set, want a value");
        Fails("""{"non_empty": ["blank"]}""", "blank: empty, want a value");
        Fails("""{"absent": ["instance.id"]}""", "instance.id: set to \"@1\", want it unset");
    }

    [Fact]
    public void NonEmptyAndAbsentReadEveryKindsDefaultAsUnset()
    {
        var defaults = Map(
            ("flag", false), ("integral", 0L), ("unsigned", 0UL), ("real", 0.0), ("text", ""),
            ("list", List()), ("map", Map()), ("nothing", null));
        var set = Map(
            ("flag", true), ("integral", -1L), ("unsigned", 1UL), ("real", -0.5), ("text", "x"),
            ("list", List(0L)), ("map", Map(("k", 0L))), ("nothing", "y"));
        const string Paths = """["flag", "integral", "unsigned", "real", "text", "list", "map", "nothing"]""";

        Assert.Empty(Checks.Check(Expect($$"""{"absent": {{Paths}} }"""), defaults));
        Assert.Equal(8, Checks.Check(Expect($$"""{"non_empty": {{Paths}} }"""), defaults).Count);
        Assert.Empty(Checks.Check(Expect($$"""{"non_empty": {{Paths}} }"""), set));
        Assert.Equal(8, Checks.Check(Expect($$"""{"absent": {{Paths}} }"""), set).Count);
    }

    [Fact]
    public void AScalarExpectationRejectsAnObjectOrAList()
    {
        Fails(
            """{"response": {"instance": "Demo::Vehicle"}}""",
            "instance: {feature_values: {mass: {value: {real_value: 1500}}}, id: \"@1\", type_symbol_id: \"Demo::Vehicle\"}, want \"Demo::Vehicle\"");
        Fails("""{"response": {"elements": true}}""", "want true");
        Fails("""{"response": {"content": null}}""", "content: \"part def Vehicle {\\n\\tattribute mass = 1500.0;\\n}\", want nothing");
        // As the reference compares: by a list's spelling under Go's fmt.Sprint.
        Passes("""{"response": {"ordinals": "[1 2]"}}""");
    }

    [Fact]
    public void AnEnumComparesByItsSchemaName()
    {
        Passes("""{"response": {"failure": "EDIT_FAILURE_UNKNOWN_TARGET"}}""");
        Fails("""{"response": {"failure": "EDIT_FAILURE_NO_OPERATIONS"}}""", "failure: \"EDIT_FAILURE_UNKNOWN_TARGET\", want \"EDIT_FAILURE_NO_OPERATIONS\"");
    }

    [Fact]
    public void AScalarThatIsNoNumberComparesByItsSpelling()
    {
        Passes("""{"response": {"experimental": true}}""");
        // As the reference compares: by what each prints as, so the text "true" is the boolean true.
        Passes("""{"response": {"experimental": "true", "withheld": "3"}}""");
        Fails("""{"response": {"experimental": "yes"}}""", "experimental: true, want \"yes\"");
    }

    [Fact]
    public void MismatchesAreReportedInAStableOrder()
    {
        var expect = """
            {
              "min_counts": {"elements": 5},
              "counts": {"elements": 3},
              "contains_all": {"capabilities": ["validate"]},
              "contains": {"content": "speed"},
              "absent": ["instance.id"],
              "non_empty": ["zz", "error"],
              "response": {"withheld": 4, "instance": {"type_symbol_id": "Demo::Car"}}
            }
            """;

        Assert.Equal(
            [
                "instance.type_symbol_id: \"Demo::Vehicle\", want \"Demo::Car\"",
                "withheld: 3, want 4",
                "error: not set, want a value",
                "zz: not set, want a value",
                "instance.id: set to \"@1\", want it unset",
                "content: \"part def Vehicle {\\n\\tattribute mass = 1500.0;\\n}\" does not contain \"speed\"",
                "capabilities: [\"query\", \"convert\"] does not contain \"validate\"",
                "elements: 2 entries, want 3",
                "elements: 2 entries, want at least 5",
            ],
            Check(expect));
    }

    [Fact]
    public void APathReadsFieldsMapKeysAndListIndices()
    {
        var tree = Response();
        Assert.True(Checks.Lookup(tree, "instance.feature_values.mass.value.real_value", out var real));
        Assert.Equal(1500.0, real);
        Assert.True(Checks.Lookup(tree, "elements.1.id", out var id));
        Assert.Equal("Demo::spare", id);
        Assert.True(Checks.Lookup(tree, "labels.a", out var label));
        Assert.Equal("y", label);

        Assert.False(Checks.Lookup(tree, "elements.2.id", out _));
        Assert.False(Checks.Lookup(tree, "elements.-1.id", out _));
        Assert.False(Checks.Lookup(tree, "elements.first.id", out _));
        Assert.False(Checks.Lookup(tree, "content.length", out _));
        Assert.False(Checks.Lookup(tree, "instance.missing", out _));
    }

    [Fact]
    public void AValueIsRenderedAsAScenarioWouldWriteIt()
    {
        Assert.Equal(
            "{a: 3, b: [1, 2.5, \"x\\n\", nothing, true, 1e+06]}",
            Checks.Render(Map(("b", List(1L, 2.5, "x\n", null, true, 1e6)), ("a", 3UL))));
    }

    private static IReadOnlyList<string> Check(string expect) => Checks.Check(Expect(expect), Response());

    private static void Passes(string expect)
    {
        var failures = Check(expect);
        Assert.True(failures.Count == 0, $"The expectation failed: {string.Join("; ", failures)}");
    }

    private static void Fails(string expect, string mention)
    {
        var failures = Check(expect);
        Assert.True(failures.Count > 0, "The expectation held; want a mismatch.");
        Assert.Contains(failures, failure => failure.Contains(mention, StringComparison.Ordinal));
    }
}
