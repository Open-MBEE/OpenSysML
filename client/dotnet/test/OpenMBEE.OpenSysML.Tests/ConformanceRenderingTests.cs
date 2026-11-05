using System;
using System.Collections;
using System.Collections.Generic;
using System.Linq;
using Google.Protobuf;
using Google.Protobuf.Reflection;
using OpenMBEE.OpenSysML.Conformance;
using OpenMBEE.OpenSysML.Internal;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// Answers that set every field the model RPCs' responses can reach, so a test can hold the
/// conformance adapters to rendering each one: a value of every arm, with every part of it, and a
/// symbol, an object and a diagnostic with every fact.
/// </summary>
internal static class CompleteWire
{
    /// <summary>The method the samples are read as answers of.</summary>
    internal const string Method = "sysml.SysMLService/Evaluate";

    /// <summary>One value of every arm, and of every quantity magnitude, each part set.</summary>
    internal static IReadOnlyList<Wire.Value> Values { get; } =
    [
        new() { IntValue = -7 },
        new() { RealValue = 2.5 },
        new() { BoolValue = true },
        new() { StringValue = "text" },
        new() { InstanceId = 42 },
        new() { Sequence = new Wire.ValueSequence { Elements = { new Wire.Value { IntValue = 1 }, new Wire.Value { StringValue = "a" } } } },
        new() { Null = "complex_values" },
        new() { Quantity = Quantity(new Wire.Quantity { IntMagnitude = 3 }) },
        new() { Quantity = Quantity(new Wire.Quantity { RealMagnitude = 1.25 }) },
        new() { Quantity = Quantity(new Wire.Quantity { BigIntMagnitude = "9223372036854775808" }) },
        new() { Quantity = Quantity(new Wire.Quantity { RationalMagnitude = new Wire.Rational { Numerator = "-1", Denominator = "3" } }) },
        new() { EnumLiteral = new Wire.EnumLiteral { LiteralId = "D::Color::red", EnumerationId = "D::Color", Name = "red", Value = new Wire.Value { IntValue = 1 } } },
        new() { Unset = true },
        new() { Complex = new Wire.Complex { Real = 1.5, Imaginary = -2.0 } },
        new() { Array = new Wire.Array { Dimensions = { 1, 2 }, Elements = { new Wire.Value { IntValue = 1 }, new Wire.Value { IntValue = 2 } } } },
        new()
        {
            Vector = new Wire.Vector
            {
                Components =
                {
                    new Wire.Value { RealValue = 3 },
                    new Wire.Value { IntValue = 4 },
                    new Wire.Value { BigIntValue = "-9223372036854775809" },
                    new Wire.Value { RationalValue = new Wire.Rational { Numerator = "2", Denominator = "3" } },
                },
            },
        },
        new() { VectorQuantity = new Wire.VectorQuantity { Components = { Quantity(new Wire.Quantity { RealMagnitude = 3 }), Quantity(new Wire.Quantity { RealMagnitude = 4 }) } } },
        new() { MeasurementRef = new Wire.MeasurementRef { Unit = "km", UnitTerm = Term(), UnitId = "SI::kilometre" } },
        new() { Infinity = true },
        new() { Function = new Wire.Function { CalcId = "F::Scaler::scale", SelfId = 7 } },
        new() { Set = new Wire.ValueSet { Elements = { new Wire.Value { IntValue = 3 }, new Wire.Value { IntValue = 1 } } } },
        new() { TensorQuantity = new Wire.TensorQuantity { Dimensions = { 2 }, Components = { Quantity(new Wire.Quantity { IntMagnitude = 1 }), Quantity(new Wire.Quantity { IntMagnitude = 2 }) } } },
        new() { Metaobject = new Wire.Metaobject { ElementId = "Meta::seatBelt", MetaclassId = "SysML::Systems::PartUsage" } },
        new() { Undetermined = new Wire.Undetermined { Reason = "d has no value", Count = new Wire.MultiplicityInfo { Lower = "1", Upper = "*" } } },
        new() { BigIntValue = "-9223372036854775809" },
        new() { RationalValue = new Wire.Rational { Numerator = "1", Denominator = "3" } },
    ];

    /// <summary>Values whose optional parts are left out, which must stay left out.</summary>
    internal static IReadOnlyList<Wire.Value> Sparse { get; } =
    [
        new() { Null = string.Empty },
        new() { Quantity = new Wire.Quantity { RealMagnitude = 2, Unit = "m" } },
        new() { EnumLiteral = new Wire.EnumLiteral { LiteralId = "D::Color::red", EnumerationId = "D::Color" } },
        new() { MeasurementRef = new Wire.MeasurementRef { Unit = "m/s", UnitTerm = Term() } },
        new() { Function = new Wire.Function { CalcId = "F::Sq" } },
        new() { Undetermined = new Wire.Undetermined { Reason = "open" } },
        new() { Sequence = new Wire.ValueSequence() },
        new() { RealValue = -0.0 },
        new() { RealValue = double.NaN },
    ];

    /// <summary>A symbol with every fact set.</summary>
    internal static Wire.SymbolInfo Symbol { get; } = new()
    {
        Id = "Demo::Vehicle",
        Name = "Vehicle",
        Kind = "partDef",
        Metadata = { ["type"] = "Base", ["specializes"] = "Demo::Base" },
        ChildIds = { "Demo::Vehicle::mass", "Demo::Vehicle::engine" },
        Attributes =
        {
            new Wire.AttributeInfo { Name = "mass", Type = "Real", Value = new Wire.Value { RealValue = 1500 }, Unit = "kg" },
            new Wire.AttributeInfo { Name = "label", Type = "String" },
        },
        TypeInfo = new Wire.TypeInfo
        {
            Declared = "MassValue",
            ResolvedId = "ISQ::MassValue",
            ResolvedKind = "attributeDef",
            Primitive = "Real",
            PrimitiveSource = "ScalarValues::Real",
            Quantity = true,
            Unit = "kg",
        },
        Multiplicity = new Wire.MultiplicityInfo { Lower = "0", Upper = "*" },
        Specializations = { new Wire.Specialization { Kind = "subclassification", Declared = "Base", TargetId = "Demo::Base", TargetKind = "partDef" } },
        WithheldLibraryAttributes = 3,
    };

    /// <summary>An object whose feature holds every fact a feature value has.</summary>
    internal static Wire.Instance Instance { get; } = new()
    {
        Id = 9,
        TypeSymbolId = "Demo::Vehicle",
        FeatureValues =
        {
            ["mass"] = new Wire.FeatureValue
            {
                FeatureName = "mass",
                Value = new Wire.Value { RealValue = 1500 },
                Values = { new Wire.Value { RealValue = 1500 }, new Wire.Value { InstanceId = 10 } },
                Materialized = true,
                Error = "cyclic",
                OrderDependent = 2,
            },
            ["engine"] = new Wire.FeatureValue { FeatureName = "engine", Value = new Wire.Value { InstanceId = 10 } },
        },
    };

    /// <summary>A diagnostic with every fact set.</summary>
    internal static Wire.Diagnostic Diagnostic { get; } = new()
    {
        Severity = "warning",
        Message = "rounded",
        Span = new Wire.Span { File = "/models/a.sysml", StartLine = 1, StartCol = 2, EndLine = 3, EndCol = 4 },
        Code = "rounded-real-literal",
        OrderDependent = { "Demo::a", "Demo::b" },
        Independent = true,
    };

    /// <summary>A diagnostic of no code and no span, with a severity the client does not name.</summary>
    internal static Wire.Diagnostic BareDiagnostic { get; } = new() { Severity = "hint", Message = "consider" };

    /// <summary>Every field the samples set, by full name, at any depth.</summary>
    /// <param name="messages">The samples.</param>
    /// <returns>The fields.</returns>
    internal static HashSet<string> SetFields(IEnumerable<IMessage> messages)
    {
        var set = new HashSet<string>(StringComparer.Ordinal);
        foreach (var message in messages)
        {
            Collect(message, set);
        }

        return set;
    }

    /// <summary>Every field reachable from some messages, by full name, through message and map fields.</summary>
    /// <param name="roots">Where to start.</param>
    /// <returns>The fields.</returns>
    internal static SortedSet<string> ReachableFields(params MessageDescriptor[] roots)
    {
        var fields = new SortedSet<string>(StringComparer.Ordinal);
        var seen = new HashSet<string>(StringComparer.Ordinal);
        var pending = new Queue<MessageDescriptor>(roots);
        while (pending.TryDequeue(out var message))
        {
            if (!seen.Add(message.FullName))
            {
                continue;
            }

            foreach (var field in message.Fields.InFieldNumberOrder())
            {
                fields.Add(field.FullName);
                if (field.IsMap && field.MessageType.FindFieldByNumber(2) is { FieldType: FieldType.Message } mapped)
                {
                    pending.Enqueue(mapped.MessageType);
                }
                else if (!field.IsMap && field.FieldType == FieldType.Message)
                {
                    pending.Enqueue(field.MessageType);
                }
            }
        }

        return fields;
    }

    private static Wire.Quantity Quantity(Wire.Quantity magnitude)
    {
        magnitude.Unit = "m";
        magnitude.UnitTerm = Term();
        return magnitude;
    }

    private static Wire.UnitTerm Term() =>
        new() { ScaleNum = 1000, ScaleDen = 1, Factors = { new Wire.UnitFactor { UnitId = "SI::metre", Exponent = 1 }, new Wire.UnitFactor { UnitId = "SI::second", Exponent = -1 } } };

    private static void Collect(IMessage message, HashSet<string> set)
    {
        foreach (var field in message.Descriptor.Fields.InFieldNumberOrder())
        {
            var value = field.Accessor.GetValue(message);
            if (!Normalizer.IsSet(field, message, value))
            {
                continue;
            }

            set.Add(field.FullName);
            IEnumerable items = field.IsMap ? ((IDictionary)value).Values : field.IsRepeated ? (IList)value : new[] { value };
            foreach (var item in items)
            {
                if (item is IMessage nested)
                {
                    Collect(nested, set);
                }
            }
        }
    }
}

/// <summary>
/// The conformance adapters render what the public API returned as wire messages, reading only public
/// domain types. Every field a model RPC's response can reach is set in a sample, and each sample, read
/// by the client and rendered back, comes out as it went in; so a fact the client drops, or a field the
/// rendering forgets, fails here before a scenario could miss it.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed class ConformanceRenderingTests
{
    [Fact]
    public void TheSamplesSetEveryFieldTheModelResponsesReach()
    {
        var reachable = CompleteWire.ReachableFields(Wire.SymbolInfo.Descriptor, Wire.Instance.Descriptor, Wire.Diagnostic.Descriptor, Wire.Value.Descriptor);
        var set = CompleteWire.SetFields([.. CompleteWire.Values, CompleteWire.Symbol, CompleteWire.Instance, CompleteWire.Diagnostic]);

        Assert.Equal([], reachable.Except(set, StringComparer.Ordinal).ToArray());
    }

    [Fact]
    public void EveryKindOfValueIsRenderedAsItWasRead()
    {
        var kinds = new HashSet<Type>();
        foreach (var sample in CompleteWire.Values.Concat(CompleteWire.Sparse))
        {
            var read = ValueWire.FromWire(sample, CompleteWire.Method, "result");
            kinds.Add(read.GetType());

            Assert.Equal(sample, Rendering.Value(read));
        }

        Assert.Equal(
            typeof(Value).GetNestedTypes().Where(type => !type.IsAbstract).Select(type => type.Name).Order(StringComparer.Ordinal),
            kinds.Select(type => type.Name).Order(StringComparer.Ordinal));
    }

    [Fact]
    public void ASymbolIsRenderedWithEveryFact()
    {
        var bare = new Wire.SymbolInfo { Id = "Demo::x", Kind = "attributeUsage" };

        Assert.Equal(CompleteWire.Symbol, Rendering.Symbol(ModelWire.Symbol(CompleteWire.Symbol, CompleteWire.Method, "symbol")));
        Assert.Equal(bare, Rendering.Symbol(ModelWire.Symbol(bare, CompleteWire.Method, "symbol")));
    }

    [Fact]
    public void AnObjectIsRenderedWithWhatEachFeatureHolds()
    {
        var bare = new Wire.Instance { Id = 1 };

        Assert.Equal(CompleteWire.Instance, Rendering.Instance(ModelWire.Instance(CompleteWire.Instance, CompleteWire.Method, "instance")));
        Assert.Equal(bare, Rendering.Instance(ModelWire.Instance(bare, CompleteWire.Method, "instance")));
    }

    [Fact]
    public void ADiagnosticIsRenderedWithItsSeverityAsTheServiceSpelledIt()
    {
        Assert.Equal(CompleteWire.Diagnostic, Rendering.Diagnostic(ModelWire.Diagnostic(CompleteWire.Diagnostic)));
        Assert.Equal(CompleteWire.BareDiagnostic, Rendering.Diagnostic(ModelWire.Diagnostic(CompleteWire.BareDiagnostic)));
        Assert.Equal(
            [CompleteWire.Diagnostic, CompleteWire.BareDiagnostic],
            Rendering.Diagnostics(ModelWire.Diagnostics([CompleteWire.Diagnostic, CompleteWire.BareDiagnostic])));
    }
}
