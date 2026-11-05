using System.Collections.Generic;
using System.IO;
using Google.Protobuf;
using Google.Protobuf.WellKnownTypes;
using Xunit;

namespace OpenMBEE.OpenSysML.Conformance.Tests;

/// <summary>
/// The normalized values of conformance/README.md, read from real schema messages: what appears,
/// how scalars keep their kind, and which values are replaced.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed class NormalizerTests
{
    [Fact]
    public void OnlySetFieldsAreNormalized()
    {
        var tree = Normalize(new Wire.EvaluateResponse { Result = new Wire.Value { IntValue = 4 } });

        Assert.Equal(["result"], tree.Keys);
        Assert.True(Checks.Lookup(tree, "result.int_value", out var value));
        Assert.Equal(4L, value);
    }

    [Fact]
    public void AFieldWithPresenceAppearsEvenAtItsDefault()
    {
        var oneof = Normalize(new Wire.EvaluateResponse { Result = new Wire.Value { IntValue = 0 } });
        Assert.True(Checks.Lookup(oneof, "result.int_value", out var zero));
        Assert.Equal(0L, zero);

        var optional = Normalize(new Wire.ExecuteActionRequest { Seed = 0 });
        Assert.True(Checks.Lookup(optional, "seed", out var seed));
        Assert.Equal(0UL, seed);
        Assert.Empty(Normalize(new Wire.ExecuteActionRequest()));

        var message = Normalize(new Wire.EvaluateResponse { Result = new Wire.Value() });
        Assert.True(Checks.Lookup(message, "result", out var empty));
        Assert.Empty(Assert.IsAssignableFrom<IReadOnlyDictionary<string, object?>>(empty));
    }

    [Fact]
    public void ANegativeZeroIsNotTheDefault()
    {
        Assert.Empty(Normalize(new Wire.Complex { Real = 0.0, Imaginary = 0.0 }));

        var tree = Normalize(new Wire.Complex { Real = -0.0 });

        Assert.True(Checks.Lookup(tree, "real", out var real));
        Assert.True(double.IsNegative(Assert.IsType<double>(real)));
        Assert.DoesNotContain("imaginary", tree.Keys);
    }

    [Fact]
    public void IntegralFieldsStayIntegralAndRealsReal()
    {
        var tree = Normalize(new Wire.DiagnosticsResponse
        {
            Diagnostics = { new Wire.Diagnostic { Span = new Wire.Span { StartLine = 7 } } },
        });
        Assert.True(Checks.Lookup(tree, "diagnostics.0.span.start_line", out var line));
        Assert.IsType<long>(line);
        Assert.Equal(7L, line);

        var real = Normalize(new Wire.EvaluateResponse { Result = new Wire.Value { RealValue = 1500 } });
        Assert.True(Checks.Lookup(real, "result.real_value", out var mass));
        Assert.IsType<double>(mass);

        var unsigned = Normalize(new Wire.ExecuteActionRequest { Seed = ulong.MaxValue });
        Assert.True(Checks.Lookup(unsigned, "seed", out var seed));
        Assert.Equal(ulong.MaxValue, seed);
    }

    [Fact]
    public void EveryScalarKindKeepsItsKind()
    {
        AssertAt(Normalize(new FloatValue { Value = 1.5f }), "value", 1.5);
        AssertAt(Normalize(new Int32Value { Value = -7 }), "value", -7L);
        AssertAt(Normalize(new UInt32Value { Value = uint.MaxValue }), "value", (ulong)uint.MaxValue);
        AssertAt(Normalize(new UInt64Value { Value = ulong.MaxValue }), "value", ulong.MaxValue);
        AssertAt(Normalize(new BoolValue { Value = true }), "value", true);
        Assert.Empty(Normalize(new FloatValue()));
        Assert.Empty(Normalize(new Int32Value()));
        Assert.Empty(Normalize(new UInt32Value()));
        Assert.Empty(Normalize(new UInt64Value()));
        Assert.Empty(Normalize(new BoolValue()));
    }

    [Fact]
    public void AnObservationIsRenderedAsSortedJsonOfEveryKind()
    {
        var tree = Trees.Map(
            ("z", Trees.List(true, false, null, -3L, ulong.MaxValue, 1.5, double.NaN, double.PositiveInfinity, "té")),
            ("a", Trees.Map(("b", "x"), ("a", Trees.Map()))));

        Assert.Equal(
            """
            {
              "a": {
                "a": {},
                "b": "x"
              },
              "z": [
                true,
                false,
                null,
                -3,
                18446744073709551615,
                1.5,
                "NaN",
                "+Inf",
                "té"
              ]
            }
            """.ReplaceLineEndings("\n"),
            ObservationJson.Render(tree));
    }

    [Fact]
    public void InstanceIdsAreLabelledInOrderOfFirstAppearance()
    {
        var response = new Wire.InstantiateResponse
        {
            Instance = new Wire.Instance
            {
                Id = 41,
                FeatureValues =
                {
                    ["engine"] = new Wire.FeatureValue { Value = new Wire.Value { InstanceId = 77 } },
                    ["scale"] = new Wire.FeatureValue { Value = new Wire.Value { Function = new Wire.Function { CalcId = "T::scale", SelfId = 41 } } },
                },
            },
            Instances = { new Wire.Instance { Id = 41 }, new Wire.Instance { Id = 77 } },
        };

        var tree = Normalize(response);

        AssertAt(tree, "instance.id", "@1");
        AssertAt(tree, "instance.feature_values.engine.value.instance_id", "@2");
        AssertAt(tree, "instance.feature_values.scale.value.function.self_id", "@1");
        AssertAt(tree, "instances.0.id", "@1");
        AssertAt(tree, "instances.1.id", "@2");
    }

    [Fact]
    public void LabelsFollowMapKeysNotInsertionOrder()
    {
        Wire.Instance Holding(params (string Name, long Id)[] values)
        {
            var instance = new Wire.Instance { TypeSymbolId = "T" };
            foreach (var (name, id) in values)
            {
                instance.FeatureValues[name] = new Wire.FeatureValue { Value = new Wire.Value { InstanceId = id } };
            }

            return instance;
        }

        var forward = Normalize(new Wire.InstantiateResponse { Instance = Holding(("a", 9), ("b", 5)) });
        var backward = Normalize(new Wire.InstantiateResponse { Instance = Holding(("b", 5), ("a", 9)) });

        Assert.Equal(ObservationJson.Render(forward), ObservationJson.Render(backward));
        AssertAt(forward, "instance.feature_values.a.value.instance_id", "@1");
        AssertAt(forward, "instance.feature_values.b.value.instance_id", "@2");
    }

    [Fact]
    public void LabelsArePerResponse()
    {
        var normalizer = new Normalizer("");
        var first = normalizer.Normalize(new Wire.InstantiateResponse { Instance = new Wire.Instance { Id = 5 } });
        var second = new Normalizer("").Normalize(new Wire.InstantiateResponse { Instance = new Wire.Instance { Id = 6 } });

        AssertAt(first, "instance.id", "@1");
        AssertAt(second, "instance.id", "@1");
    }

    [Fact]
    public void AnIdFieldOutsideTheRuntimeIdsIsNotLabelled()
    {
        var tree = Normalize(new Wire.EvaluateResponse { Result = new Wire.Value { IntValue = 41 } });
        AssertAt(tree, "result.int_value", 41L);
    }

    [Fact]
    public void TheVersionAndTheModelHashAreReplaced()
    {
        AssertAt(Normalize(new Wire.ServerInfoResponse { Version = "0.9.1-dev" }), "version", "${version}");

        var parsed = new Normalizer("abc123").Normalize(new Wire.ParseFileResponse { ModelHash = "abc123" });
        AssertAt(parsed, "model_hash", "${model_hash}");

        var elsewhere = new Normalizer("abc123").Normalize(new Wire.EvaluateResponse { Result = new Wire.Value { StringValue = "abc123" } });
        AssertAt(elsewhere, "result.string_value", "${model_hash}");

        AssertAt(Normalize(new Wire.ParseFileResponse { ModelHash = "abc123" }), "model_hash", "abc123");
    }

    [Fact]
    public void AnAbsolutePathIsReplacedAndARelativeOneKept()
    {
        var tree = Normalize(new Wire.DiagnosticsResponse
        {
            Diagnostics =
            {
                new Wire.Diagnostic { Severity = "error", Span = new Wire.Span { File = Path.Combine(Path.GetTempPath(), "model.sysml") } },
                new Wire.Diagnostic { Severity = "error", Span = new Wire.Span { File = "model.sysml" } },
            },
        });

        AssertAt(tree, "diagnostics.0.span.file", "${path}");
        AssertAt(tree, "diagnostics.1.span.file", "model.sysml");
    }

    [Fact]
    public void AnEnumIsItsSchemaName()
    {
        AssertAt(Normalize(new Wire.ApplyEditsResponse { Failure = Wire.EditFailure.UnknownTarget }), "failure", "EDIT_FAILURE_UNKNOWN_TARGET");
        AssertAt(Normalize(new Wire.ApplyEditsResponse { Failure = (Wire.EditFailure)99 }), "failure", 99L);
        Assert.Empty(Normalize(new Wire.ApplyEditsResponse { Failure = Wire.EditFailure.Unspecified }));
    }

    [Fact]
    public void MapsAreKeyedByTheirKeysAndHoldTheirValuesKind()
    {
        var response = new Wire.ExecuteActionResponse { OrderDependent = { ["b"] = 2, ["a"] = 1 } };

        var tree = Normalize(response);

        Assert.True(Checks.Lookup(tree, "order_dependent", out var map));
        Assert.Equal(["a", "b"], Assert.IsAssignableFrom<IReadOnlyDictionary<string, object?>>(map).Keys);
        AssertAt(tree, "order_dependent.b", 2L);
    }

    [Fact]
    public void BytesAreTheirProtobufJsonBase64()
    {
        AssertAt(Normalize(new Wire.MigrationFile { Path = "a.xmi", Content = ByteString.CopyFromUtf8("hi") }), "content", "aGk=");
        Assert.DoesNotContain("content", Normalize(new Wire.MigrationFile { Path = "a.xmi" }).Keys);
    }

    private static IReadOnlyDictionary<string, object?> Normalize(IMessage message) => new Normalizer("").Normalize(message);

    private static void AssertAt(IReadOnlyDictionary<string, object?> tree, string path, object expected)
    {
        Assert.True(Checks.Lookup(tree, path, out var value), $"{path} reaches nothing in {Checks.Render(tree)}");
        Assert.Equal(expected, value);
    }
}
