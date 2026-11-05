using System;
using System.Linq;
using System.Text;
using OpenMBEE.OpenSysML.Internal;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// Reading what the service answered: what the schema makes canonical is checked and never
/// repaired, a malformed value is an Internal transport failure naming where it is, and a value of
/// no kind this client knows is an <see cref="UnsupportedValueException"/>, never a null.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed class ValueReadTests
{
    private const string Method = ValueArmTests.Method;

    [Theory]
    [InlineData("9223372036854775808")]
    [InlineData("-9223372036854775809")]
    [InlineData("123456789012345678901234567890123456789012345678901234567890")]
    [InlineData("-18446744073709551616")]
    public void AnIntegerBeyondInt64IsRead(string digits)
    {
        var expected = System.Numerics.BigInteger.Parse(digits, System.Globalization.CultureInfo.InvariantCulture);

        Assert.Equal(new Value.BigIntegerValue(expected), Read(new Wire.Value { BigIntValue = digits }));
        Assert.Equal(new Value.BigIntegerValue(expected), Assert.IsType<Value.QuantityValue>(Read(new Wire.Value { Quantity = new Wire.Quantity { BigIntMagnitude = digits } })).Quantity.Magnitude);
        Assert.Equal(new Value.BigIntegerValue(expected), Assert.IsType<Value.VectorValue>(Read(new Wire.Value { Vector = new Wire.Vector { Components = { new Wire.Value { BigIntValue = digits } } } })).Components[0]);
    }

    [Theory]
    [InlineData("9223372036854775807", "within int64")]
    [InlineData("-9223372036854775808", "within int64")]
    [InlineData("1", "within int64")]
    [InlineData("0", "not canonical")]
    [InlineData("-0", "not canonical")]
    [InlineData("", "not canonical")]
    [InlineData("-", "not canonical")]
    [InlineData("--9223372036854775809", "not canonical")]
    [InlineData("09223372036854775808", "not canonical")]
    [InlineData("+9223372036854775808", "not canonical")]
    [InlineData(" 9223372036854775808", "not canonical")]
    [InlineData("9223372036854775808 ", "not canonical")]
    [InlineData("9.3e18", "not canonical")]
    [InlineData("\u0669\u0662\u0662\u0663\u0663\u0667\u0662\u0660\u0663\u0666\u0668\u0665\u0664\u0667\u0667\u0665\u0668\u0660\u0668", "not canonical")]
    public void AnIntegerBigIntValueMustSpellCanonicallyAndBeyondInt64(string digits, string why)
    {
        Assert.Contains(why, Malformed(new Wire.Value { BigIntValue = digits }, "result").Message, StringComparison.Ordinal);
        Malformed(new Wire.Value { Quantity = new Wire.Quantity { BigIntMagnitude = digits } }, "result.quantity");
        Malformed(new Wire.Value { Vector = new Wire.Vector { Components = { new Wire.Value { IntValue = 1 }, new Wire.Value { BigIntValue = digits } } } }, "result.vector[1]");
    }

    [Theory]
    [InlineData("1", "3")]
    [InlineData("-1", "3")]
    [InlineData("1", "10")]
    [InlineData("123456789012345678901234567890", "11")]
    [InlineData("1", "2" + "0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001")]
    public void ARationalNoDoubleHoldsIsRead(string numerator, string denominator)
    {
        var expected = new Rational(Integer(numerator), Integer(denominator));
        var terms = new Wire.Rational { Numerator = numerator, Denominator = denominator };

        Assert.Equal(new Value.RationalValue(expected), Read(new Wire.Value { RationalValue = terms }));
        Assert.Equal(new Value.RationalValue(expected), Assert.IsType<Value.QuantityValue>(Read(new Wire.Value { Quantity = new Wire.Quantity { RationalMagnitude = terms } })).Quantity.Magnitude);
    }

    [Theory]
    [InlineData("2", "6", "not in lowest terms")]
    [InlineData("-2", "6", "not in lowest terms")]
    [InlineData("1", "-3", "not canonical")]
    [InlineData("-1", "-3", "not canonical")]
    [InlineData("1", "0", "not canonical")]
    [InlineData("-0", "3", "not canonical")]
    [InlineData("01", "3", "not canonical")]
    [InlineData("1", "03", "not canonical")]
    [InlineData("+1", "3", "not canonical")]
    [InlineData("", "3", "not canonical")]
    [InlineData("1", "", "not canonical")]
    [InlineData("1/3", "1", "not canonical")]
    [InlineData("0", "1", "a double")]
    [InlineData("1", "2", "a double")]
    [InlineData("3", "1", "a double")]
    [InlineData("-5", "4", "a double")]
    public void ARationalMustBeCanonicalLowestTermsAndNoDouble(string numerator, string denominator, string why)
    {
        var terms = new Wire.Rational { Numerator = numerator, Denominator = denominator };

        Assert.Contains(why, Malformed(new Wire.Value { RationalValue = terms }, "result").Message, StringComparison.Ordinal);
        Malformed(new Wire.Value { Quantity = new Wire.Quantity { RationalMagnitude = terms, Unit = "m" } }, "result.quantity");
        Malformed(new Wire.Value { Vector = new Wire.Vector { Components = { new Wire.Value { RationalValue = terms } } } }, "result.vector[0]");
    }

    [Fact]
    public void ARationalOfTheSmallestSubnormalIsADoubleButOneHalfAsLargeIsNot()
    {
        var smallest = (System.Numerics.BigInteger.One << 1074).ToString(System.Globalization.CultureInfo.InvariantCulture);
        var half = (System.Numerics.BigInteger.One << 1075).ToString(System.Globalization.CultureInfo.InvariantCulture);

        Malformed(new Wire.Value { RationalValue = new Wire.Rational { Numerator = "1", Denominator = smallest } }, "result");
        Assert.IsType<Value.RationalValue>(Read(new Wire.Value { RationalValue = new Wire.Rational { Numerator = "1", Denominator = half } }));
    }

    [Fact]
    public void AMalformedAnswerIsAnInternalTransportFailureNamingTheMethodAndThePath()
    {
        var failure = Assert.Throws<TransportException>(() => ValueWire.FromWire(
            new Wire.Value { Sequence = new Wire.ValueSequence { Elements = { new Wire.Value { IntValue = 1 }, new Wire.Value { Set = new Wire.ValueSet { Elements = { new Wire.Value { Infinity = false } } } } } } },
            "sysml.SysMLService/EvaluateCalc",
            "result"));

        Assert.Equal(StatusCode.Internal, failure.StatusCode);
        Assert.Equal("sysml.SysMLService/EvaluateCalc", failure.Method);
        Assert.Equal("The service answered a malformed value at result.sequence[1].set[0]: the infinity arm states no value unless it is true.", failure.Message);
    }

    [Fact]
    public void AShapeTheElementsDoNotFillIsMalformed()
    {
        Assert.IsType<ArgumentException>(Malformed(new Wire.Value { Array = new Wire.Array { Dimensions = { 2, 2 }, Elements = { Ints(3) } } }, "result").InnerException);
        Malformed(new Wire.Value { Array = new Wire.Array { Dimensions = { 2, 0 }, Elements = { } } }, "result");
        Malformed(new Wire.Value { Array = new Wire.Array { Dimensions = { -1 }, Elements = { Ints(1) } } }, "result");
        Malformed(new Wire.Value { Array = new Wire.Array { Dimensions = { 1L << 62, 4 } } }, "result");
        Malformed(new Wire.Value { Array = new Wire.Array() }, "result");
        Malformed(new Wire.Value { TensorQuantity = new Wire.TensorQuantity { Dimensions = { 3 }, Components = { ValueArmTests.WireMetres(1), ValueArmTests.WireMetres(2) } } }, "result");
        Malformed(new Wire.Value { TensorQuantity = new Wire.TensorQuantity { Dimensions = { 0 } } }, "result");
        Malformed(new Wire.Value { VectorQuantity = new Wire.VectorQuantity() }, "result");

        Assert.Equal("The service answered a malformed value at result: An array of dimensions [2, 2] holds 3 elements, not 4.", Malformed(new Wire.Value { Array = new Wire.Array { Dimensions = { 2, 2 }, Elements = { Ints(3) } } }, "result").Message);
        Assert.Equal(new Value.ArrayValue([], [new Value.IntegerValue(0)]), Read(new Wire.Value { Array = new Wire.Array { Elements = { Ints(1) } } }));
    }

    [Fact]
    public void AnArmMissingWhatItsSchemaRequiresIsMalformed()
    {
        Assert.Contains("no magnitude", Malformed(new Wire.Value { Quantity = new Wire.Quantity { Unit = "m" } }, "result.quantity").Message, StringComparison.Ordinal);
        Malformed(new Wire.Value { VectorQuantity = new Wire.VectorQuantity { Components = { new Wire.Quantity { Unit = "m" } } } }, "result.vector_quantity[0]");
        Malformed(new Wire.Value { TensorQuantity = new Wire.TensorQuantity { Dimensions = { 1 }, Components = { new Wire.Quantity() } } }, "result.tensor_quantity[0]");
        Assert.Contains("no reduction", Malformed(new Wire.Value { MeasurementRef = new Wire.MeasurementRef { Unit = "km" } }, "result").Message, StringComparison.Ordinal);
        Malformed(new Wire.Value { Function = new Wire.Function { SelfId = 3 } }, "result");
        Malformed(new Wire.Value { Metaobject = new Wire.Metaobject { MetaclassId = "SysML::Systems::PartUsage" } }, "result");
        Malformed(new Wire.Value { EnumLiteral = new Wire.EnumLiteral { Name = "red" } }, "result");
        Malformed(new Wire.Value { Unset = false }, "result");
        Malformed(new Wire.Value { Infinity = false }, "result");
        Malformed(new Wire.Value { Vector = new Wire.Vector { Components = { new Wire.Value { RealValue = 3 }, new Wire.Value { StringValue = "four" } } } }, "result.vector[1]");
        Malformed(new Wire.Value { EnumLiteral = new Wire.EnumLiteral { LiteralId = "D::C::x", Value = new Wire.Value { Infinity = false } } }, "result.enum_literal.value");
    }

    [Fact]
    public void ASetListingAMemberTwiceIsMalformed()
    {
        var twice = new Wire.Value { Set = new Wire.ValueSet { Elements = { new Wire.Value { IntValue = 1 }, new Wire.Value { StringValue = "a" }, new Wire.Value { IntValue = 1 } } } };

        Assert.Contains("lists the member 1 twice", Malformed(twice, "result").Message, StringComparison.Ordinal);
        Assert.Equal(2, Assert.IsType<Value.SetValue>(Read(new Wire.Value { Set = new Wire.ValueSet { Elements = { new Wire.Value { IntValue = 1 }, new Wire.Value { RealValue = 1 } } } })).Count);
    }

    [Fact]
    public void AValueOfAnUnknownKindIsUnsupportedAndKeepsItsBytes()
    {
        byte[] unknownArm = [0x98, 0x06, 0x2A];
        var value = Wire.Value.Parser.ParseFrom(unknownArm);

        var unsupported = Assert.Throws<UnsupportedValueException>(() => ValueWire.FromWire(value, Method, "result"));

        Assert.Equal(Wire.Value.KindOneofCase.None, value.KindCase);
        Assert.Equal((Method, "result"), (unsupported.Method, unsupported.Path));
        Assert.Equal(unknownArm, unsupported.ValueBytes.ToArray());
        Assert.Equal("The service answered a value at result that is of a kind this client does not know; a newer client may read it.", unsupported.Message);
        Assert.IsAssignableFrom<OpenSysMLException>(unsupported);
    }

    [Fact]
    public void AValueNamingNoKindIsUnsupportedRatherThanNull()
    {
        var unsupported = Assert.Throws<UnsupportedValueException>(() => ValueWire.FromWire(new Wire.Value(), Method, "result"));

        Assert.Equal("The service answered a value at result that names no kind of value.", unsupported.Message);
        Assert.True(unsupported.ValueBytes.IsEmpty);
    }

    [Fact]
    public void AnUnknownKindNestedAnywhereFailsTheWholeValueAtItsPath()
    {
        var unknown = Wire.Value.Parser.ParseFrom([0x98, 0x06, 0x01]);
        var nested = new Wire.Value
        {
            Array = new Wire.Array
            {
                Dimensions = { 2 },
                Elements = { new Wire.Value { IntValue = 1 }, new Wire.Value { Sequence = new Wire.ValueSequence { Elements = { unknown } } } },
            },
        };

        Assert.Equal("result.array[1].sequence[0]", Assert.Throws<UnsupportedValueException>(() => ValueWire.FromWire(nested, Method, "result")).Path);
        Assert.Equal("result.enum_literal.value", Assert.Throws<UnsupportedValueException>(() => ValueWire.FromWire(
            new Wire.Value { EnumLiteral = new Wire.EnumLiteral { LiteralId = "D::C::x", Value = new Wire.Value() } }, Method, "result")).Path);
    }

    [Fact]
    public void OverJsonAnUnknownArmIsDroppedByTheParserSoItsValueNamesNoKind()
    {
        var parsed = (Wire.Value)JsonBodies.Parse(Encoding.UTF8.GetBytes("""{"quaternionValue":{"w":1}}"""), Wire.Value.Descriptor);

        var unsupported = Assert.Throws<UnsupportedValueException>(() => ValueWire.FromWire(parsed, Method, "result"));

        Assert.True(unsupported.ValueBytes.IsEmpty);
    }

    [Fact]
    public void ReadingIsBoundedByDepth()
    {
        Assert.IsType<Value.SequenceValue>(Read(Nest(ValueWire.MaxDepth)));
        Assert.Contains($"nests deeper than {ValueWire.MaxDepth} values", Assert.Throws<TransportException>(() => Read(Nest(ValueWire.MaxDepth + 1))).Message, StringComparison.Ordinal);
    }

    [Fact]
    public void AValueReadIsItsOwnWhateverHappensToTheMessageAfterwards()
    {
        var wire = new Wire.Value
        {
            Sequence = new Wire.ValueSequence
            {
                Elements = { new Wire.Value { StringValue = "a" }, new Wire.Value { Quantity = ValueArmTests.WireMetres(2) } },
            },
        };
        var read = ValueWire.FromWire(wire, Method, "result");
        var before = read.ToString();

        wire.Sequence.Elements[0].StringValue = "changed";
        wire.Sequence.Elements[1].Quantity.UnitTerm.Factors[0].UnitId = "SI::s";
        wire.Sequence.Elements.Add(new Wire.Value { IntValue = 3 });

        Assert.Equal(before, read.ToString());
        Assert.Equal(new Value.SequenceValue([new Value.TextValue("a"), new Value.QuantityValue(ValueArmTests.Metres(2))]), read);
    }

    [Fact]
    public void RepeatedAndMapFieldsAreReadWithTheirPositionsInThePath()
    {
        var repeated = new Google.Protobuf.Collections.RepeatedField<Wire.Value> { new Wire.Value { IntValue = 1 }, new Wire.Value { Infinity = true } };
        var map = new Google.Protobuf.Collections.MapField<string, Wire.Value> { ["b"] = new Wire.Value { IntValue = 2 }, ["a"] = new Wire.Value { IntValue = 1 } };

        Assert.Equal(new ValueList<Value>([new Value.IntegerValue(1), new Value.InfinityValue()]), ValueWire.FromWire(repeated, Method, "arguments"));
        Assert.Equal(["a", "b"], ValueWire.FromWire(map, Method, "outputs").Keys);
        repeated.Add(new Wire.Value { Unset = false });
        map["c"] = new Wire.Value();
        Assert.Equal("The service answered a malformed value at arguments[2]: the unset arm states no value unless it is true.", Assert.Throws<TransportException>(() => ValueWire.FromWire(repeated, Method, "arguments")).Message);
        Assert.Equal("outputs[\"c\"]", Assert.Throws<UnsupportedValueException>(() => ValueWire.FromWire(map, Method, "outputs")).Path);
    }

    internal static Wire.Value Nest(int depth)
    {
        var value = new Wire.Value { IntValue = 1 };
        for (var i = 0; i < depth; i++)
        {
            value = new Wire.Value { Sequence = new Wire.ValueSequence { Elements = { value } } };
        }

        return value;
    }

    private static Wire.Value[] Ints(int count) => Enumerable.Range(0, count).Select(i => new Wire.Value { IntValue = i }).ToArray();

    private static Value Read(Wire.Value value) => ValueWire.FromWire(value, Method, "result");

    private static TransportException Malformed(Wire.Value value, string path)
    {
        var failure = Assert.Throws<TransportException>(() => Read(value));
        Assert.Equal(StatusCode.Internal, failure.StatusCode);
        Assert.StartsWith($"The service answered a malformed value at {path}: ", failure.Message, StringComparison.Ordinal);
        return failure;
    }

    private static System.Numerics.BigInteger Integer(string digits) =>
        System.Numerics.BigInteger.Parse(digits, System.Globalization.CultureInfo.InvariantCulture);
}
