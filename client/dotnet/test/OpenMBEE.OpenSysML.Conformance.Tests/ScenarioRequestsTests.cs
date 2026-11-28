using System;
using System.IO;
using System.Linq;
using System.Text.Json;
using Xunit;
using static OpenMBEE.OpenSysML.Conformance.Tests.Trees;

namespace OpenMBEE.OpenSysML.Conformance.Tests;

/// <summary>Placeholder expansion, fixture access and decoding a scenario's request into its RPC's message.</summary>
[Trait(Category.Name, Category.Unit)]
public sealed class ScenarioRequestsTests : IDisposable
{
    private readonly TempDirectory directory = new();
    private readonly FixtureDirectory fixtures;

    public ScenarioRequestsTests()
    {
        directory.Write("fixtures/vehicle.sysml", "part def Vehicle; // é\n");
        directory.Write("fixtures/archive.mdzip", [0xFF, 0x00, 0x80, 0x7F]);
        directory.Write("secret.txt", "outside");
        fixtures = new FixtureDirectory(Path.Combine(directory.Path, "fixtures"));
    }

    public void Dispose() => directory.Dispose();

    [Fact]
    public void TheModelHashIsResolvedWherever()
    {
        var request = Build<Wire.EvaluateRequest>("""{"id": "e", "rpc": "Evaluate", "request": {"model_hash": "${model_hash}", "expression": "mass"}}""", "h1");

        Assert.Equal(("h1", "mass"), (request.ModelHash, request.Expression));
    }

    [Fact]
    public void AFixtureIsResolvedAsTextOrBase64AtAnyDepth()
    {
        var sources = Build<Wire.ParseSourcesRequest>("""{"id": "p", "rpc": "ParseSources", "request": {"documents": [{"name": "vehicle.sysml", "content": "${fixture:vehicle.sysml}"}]}}""");
        Assert.Equal("part def Vehicle; // é\n", sources.Documents.Single().Content);

        var migrate = Build<Wire.MigrateRequest>("""{"id": "m", "rpc": "Migrate", "request": {"content": "${fixture_base64:archive.mdzip}", "from_format": "mdzip"}}""");
        Assert.Equal(new byte[] { 0xFF, 0x00, 0x80, 0x7F }, migrate.Content.ToByteArray());
    }

    [Theory]
    [InlineData("x ${model_hash}")]
    [InlineData("${model_hash} ")]
    [InlineData("${fixture:}")]
    [InlineData("${fixture:vehicle.sysml} and more")]
    [InlineData("${fixture:vehicle.sysml}\n")]
    public void OnlyAWholeStringIsAPlaceholder(string text)
    {
        var request = Build<Wire.EvaluateRequest>($$"""{"id": "e", "rpc": "Evaluate", "request": {"expression": {{JsonSerializer.Serialize(text)}} } }""", "h1");

        Assert.Equal(text, request.Expression);
    }

    [Fact]
    public void TheModelHashNeedsAModel()
    {
        AssertRefused(
            """{"id": "e", "rpc": "Evaluate", "request": {"model_hash": "${model_hash}"}}""",
            "the request names ${model_hash} but the scenario declares no model");
    }

    [Fact]
    public void AFixtureOutsideTheDirectoryIsRefused()
    {
        AssertRefused("""{"id": "e", "rpc": "Evaluate", "request": {"expression": "${fixture:../secret.txt}"}}""", "fixture \"../secret.txt\" is outside");
        var absolute = JsonSerializer.Serialize("${fixture:" + Path.Combine(directory.Path, "secret.txt") + "}");
        AssertRefused($$"""{"id": "e", "rpc": "Evaluate", "request": {"expression": {{absolute}} } }""", "is outside");
        Assert.Throws<InvalidDataException>(() => fixtures.Text(""));
    }

    [Fact]
    public void AMissingFixtureIsRefused()
    {
        AssertRefused("""{"id": "e", "rpc": "Evaluate", "request": {"expression": "${fixture:missing.sysml}"}}""", "reading fixture \"missing.sysml\"");
    }

    [Theory]
    [InlineData("""{"expresion": "mass"}""")]
    [InlineData("""{"expression": 5}""")]
    [InlineData("""["mass"]""")]
    [InlineData("""{"expression": "a", "expression": "b"}""")]
    public void ARequestThatDoesNotFitItsMessageIsRefused(string request)
    {
        var error = Assert.Throws<InvalidDataException>(() =>
            ScenarioRequests.Build(Scenario($$"""{"id": "e", "rpc": "Evaluate", "request": {{request}}}"""), "", fixtures));

        Assert.True(
            error.Message.StartsWith("request does not fit sysml.EvaluateRequest: ", StringComparison.Ordinal)
                || error.Message.Contains("twice", StringComparison.Ordinal),
            error.Message);
    }

    [Fact]
    public void ARequestNamingNoRpcOfTheServiceIsRefused()
    {
        AssertRefused("""{"id": "f", "rpc": "Forecast"}""", "sysml.SysMLService has no RPC \"Forecast\"");
    }

    [Fact]
    public void AnAbsentRequestIsTheEmptyMessage()
    {
        var request = Build<Wire.ServerInfoRequest>("""{"id": "s", "rpc": "GetServerInfo"}""");

        Assert.Equal(new Wire.ServerInfoRequest(), request);
    }

    [Fact]
    public void FieldsMayBeNamedAsTheSchemaOrAsProtobufJsonNamesThem()
    {
        var request = Build<Wire.EvaluateRequest>("""{"id": "e", "rpc": "Evaluate", "request": {"modelHash": "${model_hash}", "subject_symbol_id": "Demo::car"}}""", "h1");

        Assert.Equal(("h1", "Demo::car"), (request.ModelHash, request.SubjectSymbolId));
    }

    [Fact]
    public void A64BitIntegerKeepsEveryDigit()
    {
        var verify = Build<Wire.VerifyConstraintRequest>("""
            {"id": "v", "rpc": "VerifyConstraint", "request": {
              "arguments": [{"int_value": 9007199254740993}, {"sequence": {"elements": [{"intValue": -9007199254740993}]}}],
              "namedArguments": {"n": {"int_value": 9007199254740995}, "s": {"int_value": "9007199254740997"}, "r": {"real_value": 1e300}}
            }}
            """);

        Assert.Equal(9_007_199_254_740_993L, verify.Arguments[0].IntValue);
        Assert.Equal(-9_007_199_254_740_993L, verify.Arguments[1].Sequence.Elements[0].IntValue);
        Assert.Equal(9_007_199_254_740_995L, verify.NamedArguments["n"].IntValue);
        Assert.Equal(9_007_199_254_740_997L, verify.NamedArguments["s"].IntValue);
        Assert.Equal(1e300, verify.NamedArguments["r"].RealValue);

        var seeded = Build<Wire.ExecuteActionRequest>("""{"id": "x", "rpc": "ExecuteAction", "request": {"seed": 18446744073709551615}}""");
        Assert.Equal(ulong.MaxValue, seeded.Seed);
    }

    [Theory]
    [InlineData("9007199254740993.0", 9_007_199_254_740_993L)]
    [InlineData("90071992547409930e-1", 9_007_199_254_740_993L)]
    [InlineData("\"9007199254740993.0\"", 9_007_199_254_740_993L)]
    [InlineData("\"90071992547409930e-1\"", 9_007_199_254_740_993L)]
    [InlineData("-9223372036854775808.0", long.MinValue)]
    [InlineData("9223372036854775807e0", long.MaxValue)]
    [InlineData("0e99999999999999999999", 0L)]
    public void SignedRequestSpellingsKeepEveryDigitInListsAndMaps(string literal, long expected)
    {
        var request = Build<Wire.VerifyConstraintRequest>(
            """{"id":"v","rpc":"VerifyConstraint","request":{"arguments":[{"int_value":"""
            + literal + """}],"named_arguments":{"n":{"int_value":""" + literal + "}}}}");

        Assert.Equal(expected, request.Arguments[0].IntValue);
        Assert.Equal(expected, request.NamedArguments["n"].IntValue);
    }

    [Theory]
    [InlineData("18446744073709551615.0")]
    [InlineData("184467440737095516150e-1")]
    [InlineData("\"18446744073709551615.0\"")]
    [InlineData("\"184467440737095516150e-1\"")]
    public void AnUnsignedRequestSpellingKeepsEveryDigit(string literal)
    {
        var request = Build<Wire.ExecuteActionRequest>(
            """{"id":"x","rpc":"ExecuteAction","request":{"seed":""" + literal + "}}");

        Assert.Equal(ulong.MaxValue, request.Seed);
    }

    [Theory]
    [InlineData("9007199254740992.5")]
    [InlineData("\"9007199254740992.5\"")]
    [InlineData("9223372036854775808.0")]
    [InlineData("-9223372036854775809e0")]
    [InlineData("1e2147483648")]
    [InlineData("1e-2147483649")]
    public void ASignedRequestCannotBecomeAnIntegerByRounding(string literal)
    {
        AssertRefused(
            """{"id":"v","rpc":"VerifyConstraint","request":{"arguments":[{"int_value":""" + literal + "}]}}",
            "request does not fit sysml.VerifyConstraintRequest");
    }

    [Theory]
    [InlineData("-1.0")]
    [InlineData("18446744073709551616.0")]
    [InlineData("\"18446744073709551616e0\"")]
    public void AnUnsignedRequestCannotCrossItsRangeByRounding(string literal)
    {
        AssertRefused(
            """{"id":"x","rpc":"ExecuteAction","request":{"seed":""" + literal + "}}",
            "request does not fit sysml.ExecuteActionRequest");
    }

    [Theory]
    [InlineData("0001", null)]
    [InlineData("+1", null)]
    [InlineData("1.", 1UL)]
    [InlineData(" 1", null)]
    [InlineData("1 ", null)]
    [InlineData(".0", 0UL)]
    [InlineData("1e+0", 1UL)]
    [InlineData("0x1", null)]
    [InlineData("Infinity", null)]
    [InlineData("01.0", null)]
    public void QuotedIntegerSyntaxIsNeitherBroadenedNorNarrowed(string spelling, ulong? expected)
    {
        var scenario = """{"id":"x","rpc":"ExecuteAction","request":{"seed":""" + JsonSerializer.Serialize(spelling) + "}}";
        if (expected.HasValue)
        {
            Assert.Equal(expected.Value, Build<Wire.ExecuteActionRequest>(scenario).Seed);
        }
        else
        {
            AssertRefused(scenario, "request does not fit sysml.ExecuteActionRequest");
        }
    }

    [Fact]
    public void A64BitIntegerThatIsNoWholeNumberIsStillRefused()
    {
        AssertRefused("""{"id": "x", "rpc": "ExecuteAction", "request": {"seed": 1.5}}""", "request does not fit sysml.ExecuteActionRequest");
    }

    [Fact]
    public void AnInvalidFixturePathIsAContextualInputError()
    {
        var error = Assert.Throws<InvalidDataException>(() => fixtures.Text("\0"));

        Assert.StartsWith("reading fixture \"\\x00\": ", error.Message, StringComparison.Ordinal);
        Assert.IsAssignableFrom<ArgumentException>(error.InnerException);
    }

    [Fact]
    public void TheSchemaImportsNoWellKnownTypeWhoseJsonTheQuotingWouldMiss()
    {
        Assert.Empty(Wire.SysmlReflection.Descriptor.Dependencies);
    }

    private T Build<T>(string scenario, string modelHash = "")
        where T : Google.Protobuf.IMessage => Assert.IsType<T>(ScenarioRequests.Build(Scenario(scenario), modelHash, fixtures));

    private void AssertRefused(string scenario, string problem)
    {
        var error = Assert.Throws<InvalidDataException>(() => ScenarioRequests.Build(Scenario(scenario), "", fixtures));
        Assert.Contains(problem, error.Message, StringComparison.Ordinal);
    }
}
