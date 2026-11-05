using System;
using System.Collections.Generic;
using System.Threading;
using System.Threading.Tasks;
using Xunit;

namespace OpenMBEE.OpenSysML.Conformance.Tests;

/// <summary>
/// Each corruption <c>-mutate</c> names changes what it says at every depth, and nothing else: no
/// unset field gains a value, no oneof changes arm, no map key is renamed, and the answer the
/// client gave is left as it was.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed class MutationsTests
{
    private static Wire.InstantiateResponse Answer() => new()
    {
        Instance = new Wire.Instance
        {
            Id = 1,
            TypeSymbolId = "Demo::Vehicle",
            FeatureValues =
            {
                ["mass"] = new Wire.FeatureValue
                {
                    FeatureName = "mass",
                    Value = new Wire.Value { RealValue = 1500 },
                    Values = { new Wire.Value { RealValue = 1 }, new Wire.Value { StringValue = "a" } },
                },
            },
        },
        Diagnostics = { new Wire.Diagnostic { Severity = "warning", Message = "m" }, new Wire.Diagnostic { Severity = "error" } },
    };

    [Fact]
    public void PerturbingMovesEveryRealBeyondTheToleranceAndNothingElse()
    {
        var original = Answer();

        var mutated = (Wire.InstantiateResponse)Mutations.Apply(original, Mutation.PerturbReals);

        var feature = mutated.Instance.FeatureValues["mass"];
        Assert.Equal(1500 * 1.000001 + 1e-9, feature.Value.RealValue);
        Assert.Equal(1 * 1.000001 + 1e-9, feature.Values[0].RealValue);
        Assert.Equal(Wire.Value.KindOneofCase.StringValue, feature.Values[1].KindCase);
        Assert.Equal("a", feature.Values[1].StringValue);
        Assert.Equal(Answer(), original);
        Assert.Equal(Answer().Diagnostics, mutated.Diagnostics);
    }

    [Fact]
    public void TruncatingDropsTheLastEntryOfEveryListAtEveryDepthButNoMapEntry()
    {
        var mutated = (Wire.InstantiateResponse)Mutations.Apply(Answer(), Mutation.TruncateLists);

        Assert.Equal([new Wire.Diagnostic { Severity = "warning", Message = "m" }], mutated.Diagnostics);
        Assert.Equal([new Wire.Value { RealValue = 1 }], mutated.Instance.FeatureValues["mass"].Values);
        Assert.Equal(["mass"], mutated.Instance.FeatureValues.Keys);
    }

    [Fact]
    public void RewritingChangesEveryStringTheWireCarriesButNoKeyAndNoEmptyString()
    {
        var mutated = (Wire.InstantiateResponse)Mutations.Apply(Answer(), Mutation.RewriteStrings);

        var feature = mutated.Instance.FeatureValues["mass"];
        Assert.Equal(Mutations.Rewritten, mutated.Instance.TypeSymbolId);
        Assert.Equal(Mutations.Rewritten, feature.FeatureName);
        Assert.Equal(Mutations.Rewritten, feature.Values[1].StringValue);
        Assert.Equal(Wire.Value.KindOneofCase.RealValue, feature.Values[0].KindCase);
        Assert.Equal([Mutations.Rewritten, Mutations.Rewritten], [mutated.Diagnostics[0].Severity, mutated.Diagnostics[0].Message]);
        Assert.Equal(string.Empty, mutated.Diagnostics[1].Message);
        Assert.Equal(string.Empty, mutated.Error);
    }

    [Fact]
    public void NoMutationIsAnEqualCopy()
    {
        var original = Answer();

        var copy = Mutations.Apply(original, Mutation.None);

        Assert.Equal(original, copy);
        Assert.NotSame(original, copy);
    }

    [Theory]
    [InlineData("perturb-reals", nameof(Mutation.PerturbReals))]
    [InlineData("truncate-lists", nameof(Mutation.TruncateLists))]
    [InlineData("rewrite-strings", nameof(Mutation.RewriteStrings))]
    public void AMutationIsNamedAsTheCommandLineNamesIt(string name, string member)
    {
        var mutation = Enum.Parse<Mutation>(member);
        Assert.True(Mutations.TryParse(name, out var parsed));
        Assert.Equal(mutation, parsed);
        Assert.Equal(name, Mutations.Name(mutation));
        Assert.False(Mutations.TryParse("none", out _));
        Assert.Equal(["perturb-reals", "truncate-lists", "rewrite-strings"], Mutations.Names);
    }

    [Fact]
    public async Task OnlyAnAnswerIsMutatedAndParsesPassThrough()
    {
        var parse = new ModelParse("h", "", []);
        var answers = new Queue<ScenarioAnswer>([
            new ScenarioAnswer.Answered(new Wire.ServerInfoResponse { Version = "v", Capabilities = { "a", "b" } }),
            new ScenarioAnswer.Refused(StatusCode.NotFound, "gone"),
            new ScenarioAnswer.Unexpressible("cannot"),
        ]);
        var inner = new FakeScenarioClient
        {
            Protocol = "grpc",
            Service = "svc",
            Capabilities = ["a"],
            OnParse = (_, _, _) => Task.FromResult(parse),
            OnCall = (_, _, _) => Task.FromResult(answers.Dequeue()),
        };
        var client = new MutatingScenarioClient(inner, Mutation.TruncateLists);
        var method = ScenarioRequests.Method("GetServerInfo");

        var answered = await client.CallAsync(method, new Wire.ServerInfoRequest(), CancellationToken.None);
        var refused = await client.CallAsync(method, new Wire.ServerInfoRequest(), CancellationToken.None);
        var unexpressible = await client.CallAsync(method, new Wire.ServerInfoRequest(), CancellationToken.None);

        Assert.Equal(new ScenarioAnswer.Answered(new Wire.ServerInfoResponse { Version = "v", Capabilities = { "a" } }), answered);
        Assert.Equal(new ScenarioAnswer.Refused(StatusCode.NotFound, "gone"), refused);
        Assert.Equal(new ScenarioAnswer.Unexpressible("cannot"), unexpressible);
        Assert.Same(parse, await client.ParseAsync(new ScenarioModel("a", [], "", false), [], CancellationToken.None));
        Assert.Equal(("grpc", "svc"), (client.Protocol, client.Service));
        Assert.Equal(["a"], client.Capabilities);
        Assert.Equal(3, inner.Calls.Count);
    }
}
