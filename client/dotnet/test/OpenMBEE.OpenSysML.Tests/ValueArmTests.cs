using System.Numerics;
using Google.Protobuf;
using OpenMBEE.OpenSysML.Internal;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// One test per <see cref="ValueKind"/>, named after it: the arm's value is equal to a twin and hashes
/// alike, differs from another of its kind and from another kind, is written as exactly the expected
/// message, is read back from it, survives the binary wire, and is displayed as specified.
/// <see cref="ValueInventoryTests"/> holds this class to one test per kind.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed class ValueArmTests
{
    internal const string Method = "sysml.SysMLService/Evaluate";

    private static readonly UnitTerm Metre = new(1, 1, [new UnitFactor("SI::m", 1)]);

    [Fact]
    public void IntegerArm() =>
        Sendable(new Value.IntegerValue(-42), new Value.IntegerValue(-42), new Value.IntegerValue(42), new Wire.Value { IntValue = -42 }, "-42");

    [Fact]
    public void BigIntegerArm()
    {
        var beyond = BigInteger.Parse("-18446744073709551616", System.Globalization.CultureInfo.InvariantCulture);

        Sendable(
            new Value.BigIntegerValue(beyond),
            Value.FromInteger(beyond),
            new Value.BigIntegerValue(-beyond),
            new Wire.Value { BigIntValue = "-18446744073709551616" },
            "-18446744073709551616");
    }

    [Fact]
    public void RealArm() =>
        Sendable(new Value.RealValue(4.0), new Value.RealValue(4.0), new Value.RealValue(4.5), new Wire.Value { RealValue = 4.0 }, "4.0");

    [Fact]
    public void RationalArm() =>
        Sendable(
            new Value.RationalValue(new Rational(2, 6)),
            new Value.RationalValue(new Rational(-1, -3)),
            new Value.RationalValue(new Rational(-1, 3)),
            new Wire.Value { RationalValue = new Wire.Rational { Numerator = "1", Denominator = "3" } },
            "1/3");

    [Fact]
    public void ComplexArm() =>
        Sendable(
            new Value.ComplexValue(1.5, -2.0),
            new Value.ComplexValue(1.5, -2.0),
            new Value.ComplexValue(1.5, 2.0),
            new Wire.Value { Complex = new Wire.Complex { Real = 1.5, Imaginary = -2.0 } },
            "1.5 - 2.0i");

    [Fact]
    public void BooleanArm() =>
        Sendable(new Value.BooleanValue(true), new Value.BooleanValue(true), new Value.BooleanValue(false), new Wire.Value { BoolValue = true }, "true");

    [Fact]
    public void TextArm() =>
        Sendable(
            new Value.TextValue("a \"b\" \\"),
            new Value.TextValue("a \"b\" \\"),
            new Value.TextValue("a"),
            new Wire.Value { StringValue = "a \"b\" \\" },
            "\"a \\\"b\\\" \\\\\"");

    [Fact]
    public void InstanceRefArm() =>
        Sendable(new Value.InstanceRefValue(7), new Value.InstanceRefValue(7), new Value.InstanceRefValue(8), new Wire.Value { InstanceId = 7 }, "instance(7)");

    [Fact]
    public void EnumLiteralArm()
    {
        var red = new Value.EnumLiteralValue("D::Level::high", "D::Level", "Level::high", new Value.IntegerValue(3));
        var wire = new Wire.Value
        {
            EnumLiteral = new Wire.EnumLiteral { LiteralId = "D::Level::high", EnumerationId = "D::Level", Name = "Level::high", Value = new Wire.Value { IntValue = 3 } },
        };

        Sendable(red, new Value.EnumLiteralValue("D::Level::high", "Other", "high"), new Value.EnumLiteralValue("D::Level::low", "D::Level", "Level::high"), wire, "Level::high");
        var read = Assert.IsType<Value.EnumLiteralValue>(ValueWire.FromWire(wire, Method, "result"));
        Assert.Equal(("D::Level", "Level::high", new Value.IntegerValue(3)), (read.EnumerationId, read.Name, read.Scalar));
        Assert.Equal("D::Level::low", new Value.EnumLiteralValue("D::Level::low", "D::Level", string.Empty).ToString());
        Assert.Null(Assert.IsType<Value.EnumLiteralValue>(ValueWire.FromWire(new Wire.Value { EnumLiteral = new Wire.EnumLiteral { LiteralId = "D::C::x" } }, Method, "r")).Scalar);
    }

    [Fact]
    public void SequenceArm() =>
        Sendable(
            new Value.SequenceValue([new Value.IntegerValue(1), new Value.TextValue("a")]),
            new Value.SequenceValue([new Value.IntegerValue(1), new Value.TextValue("a")]),
            new Value.SequenceValue([new Value.TextValue("a"), new Value.IntegerValue(1)]),
            new Wire.Value { Sequence = new Wire.ValueSequence { Elements = { new Wire.Value { IntValue = 1 }, new Wire.Value { StringValue = "a" } } } },
            "(1, \"a\")");

    [Fact]
    public void SetArm()
    {
        Value[] members = [new Value.IntegerValue(1), new Value.IntegerValue(2), new Value.TextValue("c")];
        var set = new Value.SetValue(members);

        Sendable(
            set,
            new Value.SetValue([members[2], members[0], members[1]]),
            new Value.SetValue(members[..2]),
            new Wire.Value { Set = new Wire.ValueSet { Elements = { new Wire.Value { IntValue = 1 }, new Wire.Value { IntValue = 2 }, new Wire.Value { StringValue = "c" } } } },
            "{1, 2, \"c\"}");
        Assert.True(set.Contains(new Value.IntegerValue(2)));
        Assert.False(set.Contains(new Value.RealValue(2.0)));
        Assert.Equal(3, set.Count);
    }

    [Fact]
    public void QuantityArm()
    {
        var perHour = new UnitTerm(1000, 3600, [new UnitFactor("SI::m", 1), new UnitFactor("SI::s", -1)]);
        var speed = new Quantity(new Value.RealValue(5.4), "km/h", perHour);

        Sendable(
            new Value.QuantityValue(speed),
            new Value.QuantityValue(new Quantity(new Value.RealValue(5.4), "km/h", new UnitTerm(1000, 3600, [new UnitFactor("SI::m", 1), new UnitFactor("SI::s", -1)]))),
            new Value.QuantityValue(new Quantity(new Value.RealValue(5.4), "km/h", null)),
            new Wire.Value
            {
                Quantity = new Wire.Quantity
                {
                    RealMagnitude = 5.4,
                    Unit = "km/h",
                    UnitTerm = new Wire.UnitTerm
                    {
                        ScaleNum = 1000,
                        ScaleDen = 3600,
                        Factors = { new Wire.UnitFactor { UnitId = "SI::m", Exponent = 1 }, new Wire.UnitFactor { UnitId = "SI::s", Exponent = -1 } },
                    },
                },
            },
            "5.4 [km/h]");
        Assert.Equal("1000/3600 SI::m^1 SI::s^-1", perHour.ToString());
        Assert.False(speed.IsExact);
        Assert.True(new Quantity(new Value.RationalValue(new Rational(1, 3)), string.Empty, UnitTerm.One).IsExact);

        (Value.NumberValue Magnitude, Wire.Quantity.MagnitudeOneofCase Arm)[] magnitudes =
        [
            (new Value.IntegerValue(3), Wire.Quantity.MagnitudeOneofCase.IntMagnitude),
            (new Value.BigIntegerValue(BigInteger.One << 64), Wire.Quantity.MagnitudeOneofCase.BigIntMagnitude),
            (new Value.RationalValue(new Rational(1, 3)), Wire.Quantity.MagnitudeOneofCase.RationalMagnitude),
            (new Value.RealValue(-0.5), Wire.Quantity.MagnitudeOneofCase.RealMagnitude),
        ];
        foreach (var (magnitude, arm) in magnitudes)
        {
            var quantity = new Value.QuantityValue(new Quantity(magnitude, "m", Metre));
            var wire = ValueWire.ToWire(quantity, "value");
            Assert.Equal(arm, wire.Quantity.MagnitudeCase);
            Assert.Equal(quantity, ValueWire.FromWire(Wire.Value.Parser.ParseFrom(wire.ToByteArray()), Method, "result"));
        }
    }

    [Fact]
    public void ArrayArm()
    {
        var array = new Value.ArrayValue([2, 2], [new Value.IntegerValue(1), new Value.IntegerValue(2), new Value.IntegerValue(3), new Value.IntegerValue(4)]);

        Sendable(
            array,
            new Value.ArrayValue([2, 2], array.Elements),
            new Value.ArrayValue([4], array.Elements),
            new Wire.Value
            {
                Array = new Wire.Array
                {
                    Dimensions = { 2, 2 },
                    Elements = { new Wire.Value { IntValue = 1 }, new Wire.Value { IntValue = 2 }, new Wire.Value { IntValue = 3 }, new Wire.Value { IntValue = 4 } },
                },
            },
            "Array(2, 2)[1, 2, 3, 4]");
        Assert.Equal((2, new Value.IntegerValue(3)), (array.Rank, array.Get(1, 0)));
    }

    [Fact]
    public void VectorArm()
    {
        var vector = new Value.VectorValue([new Value.IntegerValue(3), new Value.RealValue(4.0)]);

        Sendable(
            vector,
            new Value.VectorValue([new Value.IntegerValue(3), new Value.RealValue(4.0)]),
            new Value.VectorValue([new Value.RealValue(3.0), new Value.RealValue(4.0)]),
            new Wire.Value { Vector = new Wire.Vector { Components = { new Wire.Value { IntValue = 3 }, new Wire.Value { RealValue = 4.0 } } } },
            "Vector(3, 4.0)");
        Assert.Equal(2, vector.Dimension);
    }

    [Fact]
    public void VectorQuantityArm()
    {
        var vector = new Value.VectorQuantityValue([Metres(3.0), Metres(4.0)]);

        Sendable(
            vector,
            new Value.VectorQuantityValue([Metres(3.0), Metres(4.0)]),
            new Value.VectorQuantityValue([Metres(4.0), Metres(3.0)]),
            new Wire.Value { VectorQuantity = new Wire.VectorQuantity { Components = { WireMetres(3.0), WireMetres(4.0) } } },
            "VectorQuantity(3.0 [m], 4.0 [m])");
        Assert.Equal((2, "m"), (vector.Dimension, vector.Unit));
        Assert.Null(new Value.VectorQuantityValue([Metres(3.0), new Quantity(new Value.RealValue(1), "s", UnitTerm.One)]).Unit);
    }

    [Fact]
    public void TensorQuantityArm()
    {
        var tensor = new Value.TensorQuantityValue([2], [Metres(1.0), Metres(2.0)]);

        Sendable(
            tensor,
            new Value.TensorQuantityValue([2], [Metres(1.0), Metres(2.0)]),
            new Value.TensorQuantityValue([2, 1], [Metres(1.0), Metres(2.0)]),
            new Wire.Value { TensorQuantity = new Wire.TensorQuantity { Dimensions = { 2 }, Components = { WireMetres(1.0), WireMetres(2.0) } } },
            "TensorQuantity(2)[1.0 [m], 2.0 [m]]");
        Assert.Equal((1, "m", Metres(2.0)), (tensor.Rank, tensor.Unit, tensor.Get(1)));
    }

    [Fact]
    public void MeasurementRefArm()
    {
        var kilometre = new UnitTerm(1000, 1, [new UnitFactor("SI::m", 1)]);
        var wire = new Wire.Value
        {
            MeasurementRef = new Wire.MeasurementRef
            {
                Unit = "km",
                UnitId = "SI::kilometre",
                UnitTerm = new Wire.UnitTerm { ScaleNum = 1000, ScaleDen = 1, Factors = { new Wire.UnitFactor { UnitId = "SI::m", Exponent = 1 } } },
            },
        };

        Sendable(
            new Value.MeasurementRefValue("km", kilometre, "SI::kilometre"),
            new Value.MeasurementRefValue("km", new UnitTerm(1000, 1, [new UnitFactor("SI::m", 1)]), "SI::kilometre"),
            new Value.MeasurementRefValue("km", kilometre),
            wire,
            "[km]");
        Assert.Equal("[SI::kilometre]", new Value.MeasurementRefValue(string.Empty, kilometre, "SI::kilometre").ToString());
        Assert.Equal("[1000/1 SI::m^1]", new Value.MeasurementRefValue(string.Empty, kilometre).ToString());
        Assert.Null(Assert.IsType<Value.MeasurementRefValue>(ValueWire.FromWire(new Wire.Value { MeasurementRef = new Wire.MeasurementRef { Unit = "m/s", UnitTerm = new Wire.UnitTerm { ScaleNum = 1, ScaleDen = 1 } } }, Method, "r")).UnitId);
    }

    [Fact]
    public void FunctionArm()
    {
        Sendable(
            new Value.FunctionValue("Analysis::Sq"),
            new Value.FunctionValue("Analysis::Sq"),
            new Value.FunctionValue("Analysis::Sq", 7),
            new Wire.Value { Function = new Wire.Function { CalcId = "Analysis::Sq" } },
            "Analysis::Sq");

        var closing = Assert.IsType<Value.FunctionValue>(ValueWire.FromWire(new Wire.Value { Function = new Wire.Function { CalcId = "F::Scaler::scale", SelfId = 7 } }, Method, "r"));
        Assert.Equal((long?)7, closing.SelfId);
        Assert.Equal("F::Scaler::scale of instance(7)", closing.ToString());
        Assert.Equal(new Value.FunctionValue("F::Scaler::scale", 7), closing);
        Assert.Equal(closing.GetHashCode(), new Value.FunctionValue("F::Scaler::scale", 7).GetHashCode());
    }

    [Fact]
    public void MetaobjectArm()
    {
        var wire = new Wire.Value { Metaobject = new Wire.Metaobject { ElementId = "Vehicle::seatBelt", MetaclassId = "SysML::Systems::PartUsage" } };

        Sendable(
            new Value.MetaobjectValue("Vehicle::seatBelt", "SysML::Systems::PartUsage"),
            new Value.MetaobjectValue("Vehicle::seatBelt"),
            new Value.MetaobjectValue("Vehicle::engine", "SysML::Systems::PartUsage"),
            wire,
            "meta(Vehicle::seatBelt : SysML::Systems::PartUsage)");
        Assert.Equal("SysML::Systems::PartUsage", Assert.IsType<Value.MetaobjectValue>(ValueWire.FromWire(wire, Method, "r")).MetaclassId);
        Assert.Equal("meta(Vehicle::seatBelt)", new Value.MetaobjectValue("Vehicle::seatBelt").ToString());
    }

    [Fact]
    public void NullArm()
    {
        var unsupported = new Value.NullValue("unsupported: variant selection");

        Sendable(new Value.NullValue(), new Value.NullValue(string.Empty), unsupported, new Wire.Value { Null = string.Empty }, "null");
        ResponseOnly(unsupported, new Value.NullValue("unsupported: variant selection"), new Value.NullValue(), new Wire.Value { Null = "unsupported: variant selection" }, "null (unsupported: variant selection)");
        Assert.True(unsupported.IsUnsupported);
        Assert.False(new Value.NullValue().IsUnsupported);
    }

    [Fact]
    public void UnsetArm() =>
        ResponseOnly(new Value.UnsetValue(), new Value.UnsetValue(), new Value.InfinityValue(), new Wire.Value { Unset = true }, "<unset>");

    [Fact]
    public void UndeterminedArm()
    {
        var open = new Value.UndeterminedValue("u has no value in the model", "1", "1");

        ResponseOnly(
            open,
            new Value.UndeterminedValue("u has no value in the model", "1", "1"),
            new Value.UndeterminedValue("u has no value in the model", "1", "*"),
            new Wire.Value { Undetermined = new Wire.Undetermined { Reason = "u has no value in the model", Count = new Wire.MultiplicityInfo { Lower = "1", Upper = "1" } } },
            "<undetermined>");
        Assert.Equal(
            new Value.UndeterminedValue("open", string.Empty, string.Empty),
            ValueWire.FromWire(new Wire.Value { Undetermined = new Wire.Undetermined { Reason = "open" } }, Method, "r"));
    }

    [Fact]
    public void InfinityArm() =>
        Sendable(new Value.InfinityValue(), new Value.InfinityValue(), new Value.UnsetValue(), new Wire.Value { Infinity = true }, "*");

    internal static Quantity Metres(double magnitude) => new(new Value.RealValue(magnitude), "m", Metre);

    internal static Wire.Quantity WireMetres(double magnitude) => new()
    {
        RealMagnitude = magnitude,
        Unit = "m",
        UnitTerm = new Wire.UnitTerm { ScaleNum = 1, ScaleDen = 1, Factors = { new Wire.UnitFactor { UnitId = "SI::m", Exponent = 1 } } },
    };

    private static void Sendable(Value value, Value twin, Value other, Wire.Value wire, string text)
    {
        Common(value, twin, other, wire, text);
        Assert.Equal(wire, ValueWire.ToWire(value, "value"));
        Assert.Equal(twin, ValueWire.FromWire(ValueWire.ToWire(twin, "value"), Method, "result"));
    }

    private static void ResponseOnly(Value value, Value twin, Value other, Wire.Value wire, string text)
    {
        Common(value, twin, other, wire, text);
        var refused = Assert.Throws<InvalidRequestException>(() => ValueWire.ToWire(value, "arguments[0]"));
        Assert.StartsWith("The value at arguments[0] ", refused.Message, System.StringComparison.Ordinal);
    }

    private static void Common(Value value, Value twin, Value other, Wire.Value wire, string text)
    {
        Assert.Equal(value.Kind, twin.Kind);
        Assert.Equal($"{value.Kind}Value", value.GetType().Name);
        Assert.True(value.Equals(twin));
        Assert.True(value.Equals((object)twin));
        Assert.True(value == twin);
        Assert.False(value != twin);
        Assert.Equal(value.GetHashCode(), twin.GetHashCode());
        Assert.False(value.Equals(other));
        Assert.True(value != other);
        Assert.False(value.Equals(null));
        Assert.False(value.Equals(value is Value.TextValue ? new Value.BooleanValue(true) : new Value.TextValue(text)));
        Assert.Equal(value, ValueWire.FromWire(wire, Method, "result"));
        Assert.Equal(value, ValueWire.FromWire(Wire.Value.Parser.ParseFrom(wire.ToByteArray()), Method, "result"));
        Assert.Equal(value.Kind.ToString(), ValueWire.FromWire(wire, Method, "result").Kind.ToString());
        Assert.Equal(text, value.ToString());
    }
}
