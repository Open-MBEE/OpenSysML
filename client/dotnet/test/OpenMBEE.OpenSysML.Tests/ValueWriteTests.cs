using System;
using System.Collections.Generic;
using System.Linq;
using System.Numerics;
using OpenMBEE.OpenSysML.Internal;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// Writing a value for a request: what no service reads as an argument is refused before anything is
/// sent, and the capabilities a value needs are found by the service's own predicates, at any depth,
/// in the service's own order.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed class ValueWriteTests
{
    private static readonly Capabilities All = new(ServiceCapabilities.Names);

    public static TheoryData<Value, string> Unsendable => new()
    {
        { new Value.UnsetValue(), "is unset" },
        { new Value.UndeterminedValue("u has no value", "1", "1"), "is undetermined" },
        { new Value.NullValue("unsupported: variant selection"), "could not send (unsupported: variant selection)" },
        { new Value.FunctionValue("F::Scaler::scale", 3), "closing over instance(3)" },
        { new Value.QuantityValue(new Quantity(new Value.IntegerValue(3), "km", null)), "is in [km] with no reduction" },
        { new Value.QuantityValue(new Quantity(new Value.IntegerValue(3), "m", new UnitTerm(0, 1, []))), "unit scale of 0/1" },
        { new Value.QuantityValue(new Quantity(new Value.IntegerValue(3), "m", new UnitTerm(1, 0, []))), "unit scale of 1/0" },
        { new Value.QuantityValue(new Quantity(new Value.IntegerValue(3), "m", new UnitTerm(double.NaN, 1, []))), "unit scale of NaN/1" },
        { new Value.QuantityValue(new Quantity(new Value.IntegerValue(3), "m", new UnitTerm(1, double.PositiveInfinity, []))), "reduces to no base unit" },
        { new Value.QuantityValue(new Quantity(new Value.IntegerValue(3), "m", new UnitTerm(1, 1, [new UnitFactor(string.Empty, 1)]))), "naming no base unit" },
        { new Value.MeasurementRefValue("m", new UnitTerm(0, 0, [])), "reduces to no base unit" },
        { new Value.VectorQuantityValue([ValueArmTests.Metres(1), new Quantity(new Value.RealValue(2), "s", null)]), "is in [s] with no reduction" },
        { new Value.TensorQuantityValue([1], [new Quantity(new Value.RealValue(2), "s", null)]), "is in [s] with no reduction" },
    };

    [Theory]
    [MemberData(nameof(Unsendable))]
    public void AValueNoServiceReadsAsAnArgumentIsRefusedAtAnyDepth(Value value, string why)
    {
        var alone = Assert.Throws<InvalidRequestException>(() => ValueWire.ToWire(value, "arguments[0]"));
        var nested = Assert.Throws<InvalidRequestException>(() => ValueWire.ToWire(
            new Value.SequenceValue([new Value.IntegerValue(1), new Value.SetValue([new Value.ArrayValue([], [value])])]),
            "arguments[0]"));

        Assert.Contains(why, alone.Message, StringComparison.Ordinal);
        Assert.StartsWith("The value at arguments[0]", alone.Message, StringComparison.Ordinal);
        Assert.StartsWith("The value at arguments[0].sequence[1].set[0].array[0]", nested.Message, StringComparison.Ordinal);
        Assert.Contains(why, nested.Message, StringComparison.Ordinal);
    }

    [Fact]
    public void AnUnsendableScalarOfAnEnumerationLiteralIsRefusedAtItsPath()
    {
        var refused = Assert.Throws<InvalidRequestException>(() => ValueWire.ToWire(new Value.EnumLiteralValue("D::C::x", "D::C", "C::x", new Value.UnsetValue()), "v"));

        Assert.StartsWith("The value at v.enum_literal.value is unset", refused.Message, StringComparison.Ordinal);
    }

    [Fact]
    public void AQuantityWithNeitherUnitNorReductionIsSentWithoutOne()
    {
        var wire = ValueWire.ToWire(new Value.QuantityValue(new Quantity(new Value.IntegerValue(3), string.Empty, null)), "v");

        Assert.Null(wire.Quantity.UnitTerm);
        Assert.Equal((3L, string.Empty), (wire.Quantity.IntMagnitude, wire.Quantity.Unit));
    }

    public static TheoryData<Value, string> GatedKinds => new()
    {
        { new Value.ComplexValue(1, 2), Capabilities.ComplexValues },
        { new Value.ArrayValue([1], [new Value.IntegerValue(1)]), Capabilities.StructuredValues },
        { new Value.VectorValue([new Value.IntegerValue(1)]), Capabilities.StructuredValues },
        { new Value.VectorQuantityValue([ValueArmTests.Metres(1)]), Capabilities.StructuredValues },
        { new Value.MeasurementRefValue("m", UnitTerm.One), Capabilities.MeasurementRefs },
        { new Value.FunctionValue("F::Sq"), Capabilities.FunctionValues },
        { new Value.InfinityValue(), Capabilities.InfinityValue },
        { new Value.SetValue([]), Capabilities.SetValues },
        { new Value.TensorQuantityValue([1], [ValueArmTests.Metres(1)]), Capabilities.TensorValues },
        { new Value.MetaobjectValue("V::seatBelt"), Capabilities.MetaobjectValues },
        { new Value.BigIntegerValue(BigInteger.One << 64), Capabilities.BigIntValues },
        { new Value.RationalValue(new Rational(1, 3)), Capabilities.RationalValues },
        { new Value.QuantityValue(new Quantity(new Value.BigIntegerValue(BigInteger.One << 64), string.Empty, null)), Capabilities.BigIntValues },
        { new Value.QuantityValue(new Quantity(new Value.RationalValue(new Rational(1, 3)), string.Empty, null)), Capabilities.RationalValues },
    };

    [Theory]
    [MemberData(nameof(GatedKinds))]
    public void AGatedKindNeedsItsCapabilityWhereverItIsNested(Value value, string capability)
    {
        Value[] carriers =
        [
            value,
            new Value.SequenceValue([new Value.TextValue("a"), value]),
            new Value.SequenceValue([new Value.SequenceValue([new Value.SequenceValue([value])])]),
        ];

        foreach (var carrier in carriers)
        {
            var missing = Assert.Throws<CapabilityException>(() => Fit(carrier, Without(capability)));
            Assert.Equal(capability, missing.Capability);
            Assert.Equal(Capabilities.UpgradeRemedy(capability), missing.Remedy);
            Assert.Contains(capability, ValueWire.RequiredCapabilities(ValueWire.ToWire(carrier, "v")));
            Fit(carrier, All);
        }
    }

    public static TheoryData<Value> UngatedKinds => new()
    {
        new Value.IntegerValue(1),
        new Value.RealValue(1.5),
        new Value.BooleanValue(false),
        new Value.TextValue("t"),
        new Value.InstanceRefValue(4),
        new Value.NullValue(),
        new Value.SequenceValue([new Value.IntegerValue(1), new Value.SequenceValue([])]),
        new Value.EnumLiteralValue("D::C::x", "D::C", "C::x"),
        new Value.QuantityValue(ValueArmTests.Metres(3)),
        new Value.RationalValue(new Rational(1, 2)),
    };

    [Theory]
    [MemberData(nameof(UngatedKinds))]
    public void AnUngatedKindIsSentToAServiceAdvertisingNothing(Value value)
    {
        Assert.Empty(ValueWire.RequiredCapabilities(Fit(value, Capabilities.None)));
    }

    [Fact]
    public void TheCapabilitiesAValueNeedsAreRequiredInTheServicesOrder()
    {
        var everything = new Value.SequenceValue(
        [
            new Value.RationalValue(new Rational(1, 3)),
            new Value.MetaobjectValue("V::seatBelt"),
            new Value.BigIntegerValue(BigInteger.One << 70),
            new Value.TensorQuantityValue([1], [ValueArmTests.Metres(1)]),
            new Value.SetValue([new Value.InfinityValue()]),
            new Value.FunctionValue("F::Sq"),
            new Value.MeasurementRefValue("m", UnitTerm.One),
            new Value.VectorValue([]),
            new Value.ComplexValue(0, 1),
        ]);
        var held = new List<string>(ValueWire.SendGateOrder);

        Assert.Equal(ValueWire.SendGateOrder, ValueWire.RequiredCapabilities(ValueWire.ToWire(everything, "v")));
        foreach (var capability in ValueWire.SendGateOrder)
        {
            Assert.Equal(capability, Assert.Throws<CapabilityException>(() => Fit(everything, new Capabilities(ServiceCapabilities.Names.Except(held)))).Capability);
            held.Remove(capability);
        }

        Fit(everything, new Capabilities(ServiceCapabilities.Names.Except(held)));
    }

    [Fact]
    public void AnEnumerationLiteralsScalarIsGatedOnlyForItsIntegerOrRationalAsTheServiceGatesIt()
    {
        Value Literal(Value scalar) => new Value.SequenceValue([new Value.EnumLiteralValue("D::C::x", "D::C", "C::x", scalar)]);

        Assert.Empty(ValueWire.RequiredCapabilities(ValueWire.ToWire(Literal(new Value.ComplexValue(1, 1)), "v")));
        Assert.Empty(ValueWire.RequiredCapabilities(ValueWire.ToWire(Literal(new Value.SetValue([])), "v")));
        Assert.Equal([Capabilities.BigIntValues], ValueWire.RequiredCapabilities(ValueWire.ToWire(Literal(new Value.SequenceValue([new Value.BigIntegerValue(BigInteger.One << 64)])), "v")));
        Assert.Equal([Capabilities.RationalValues], ValueWire.RequiredCapabilities(ValueWire.ToWire(
            Literal(new Value.EnumLiteralValue("D::E::y", "D::E", "E::y", new Value.RationalValue(new Rational(1, 3)))), "v")));
    }

    [Fact]
    public void AQuantitysMagnitudeIsGatedInAVectorQuantityAndATensorButAnArrayElementOnlyAsAValue()
    {
        var big = new Quantity(new Value.BigIntegerValue(BigInteger.One << 64), "m", ValueArmTests.Metres(1).Reduction);
        var third = new Quantity(new Value.RationalValue(new Rational(1, 3)), "m", ValueArmTests.Metres(1).Reduction);

        Assert.Equal([Capabilities.StructuredValues, Capabilities.BigIntValues], ValueWire.RequiredCapabilities(ValueWire.ToWire(new Value.VectorQuantityValue([big]), "v")));
        Assert.Equal([Capabilities.TensorValues, Capabilities.RationalValues], ValueWire.RequiredCapabilities(ValueWire.ToWire(new Value.TensorQuantityValue([], [third]), "v")));
        Assert.Equal([Capabilities.StructuredValues, Capabilities.BigIntValues], ValueWire.RequiredCapabilities(ValueWire.ToWire(new Value.ArrayValue([1], [new Value.QuantityValue(big)]), "v")));
        Assert.Equal([Capabilities.StructuredValues, Capabilities.RationalValues], ValueWire.RequiredCapabilities(ValueWire.ToWire(new Value.VectorValue([new Value.RationalValue(new Rational(1, 3))]), "v")));
    }

    [Fact]
    public void ToAServiceWithoutRationalsARationalADoubleHoldsIsSentAsThatDoubleWhereverItIs()
    {
        var half = new Value.RationalValue(new Rational(1, 2));
        var halfQuantity = new Quantity(half, "m", ValueArmTests.Metres(1).Reduction);
        var everywhere = new Value.SequenceValue(
        [
            half,
            new Value.QuantityValue(halfQuantity),
            new Value.VectorValue([half]),
            new Value.VectorQuantityValue([halfQuantity]),
            new Value.TensorQuantityValue([1], [halfQuantity]),
            new Value.SetValue([new Value.ArrayValue([1], [half])]),
            new Value.EnumLiteralValue("D::C::x", "D::C", "C::x", half),
        ]);
        var withRationals = Fit(everywhere, All);

        var withoutRationals = Fit(everywhere, Without(Capabilities.RationalValues));

        var sent = withoutRationals.Sequence.Elements;
        Assert.Equal(0.5, sent[0].RealValue);
        Assert.Equal(0.5, sent[1].Quantity.RealMagnitude);
        Assert.Equal(0.5, sent[2].Vector.Components[0].RealValue);
        Assert.Equal(0.5, sent[3].VectorQuantity.Components[0].RealMagnitude);
        Assert.Equal(0.5, sent[4].TensorQuantity.Components[0].RealMagnitude);
        Assert.Equal(0.5, sent[5].Set.Elements[0].Array.Elements[0].RealValue);
        Assert.Equal(0.5, sent[6].EnumLiteral.Value.RealValue);
        Assert.Equal(ValueWire.ToWire(everywhere, "v"), withRationals);
        Assert.Equal("1", withRationals.Sequence.Elements[0].RationalValue.Numerator);
        Assert.Contains(Capabilities.RationalValues, ValueWire.RequiredCapabilities(withRationals));
        Assert.DoesNotContain(Capabilities.RationalValues, ValueWire.RequiredCapabilities(withoutRationals));
    }

    [Fact]
    public void ARationalNoDoubleHoldsIsNeverSentAsADouble()
    {
        var third = new Value.SequenceValue([new Value.RationalValue(new Rational(1, 2)), new Value.RationalValue(new Rational(1, 3))]);

        Assert.Equal(Capabilities.RationalValues, Assert.Throws<CapabilityException>(() => Fit(third, Without(Capabilities.RationalValues))).Capability);
    }

    [Fact]
    public void TheGatesAreFoundAtAnyDepthWithoutRecursion()
    {
        var deep = new Wire.Value { Complex = new Wire.Complex { Real = 1 } };
        for (var i = 0; i < 100_000; i++)
        {
            deep = new Wire.Value { Sequence = new Wire.ValueSequence { Elements = { deep } } };
        }

        Assert.Equal([Capabilities.ComplexValues], ValueWire.RequiredCapabilities(deep));
        Assert.Equal(Capabilities.ComplexValues, Assert.Throws<CapabilityException>(() => ValueWire.Fit(deep, Capabilities.None)).Capability);
    }

    [Fact]
    public void WritingIsBoundedByDepth()
    {
        Value Nest(int depth)
        {
            Value value = new Value.IntegerValue(1);
            for (var i = 0; i < depth; i++)
            {
                value = new Value.SequenceValue([value]);
            }

            return value;
        }

        Assert.Equal(ValueReadTests.Nest(ValueWire.MaxDepth), ValueWire.ToWire(Nest(ValueWire.MaxDepth), "v"));
        Assert.Contains($"nests deeper than {ValueWire.MaxDepth} values", Assert.Throws<InvalidRequestException>(() => ValueWire.ToWire(Nest(ValueWire.MaxDepth + 1), "v")).Message, StringComparison.Ordinal);
    }

    [Fact]
    public void EveryWriteIsAFreshMessageThatChangesNothingItCameFrom()
    {
        var value = new Value.SequenceValue([new Value.TextValue("a"), new Value.QuantityValue(ValueArmTests.Metres(2)), new Value.RationalValue(new Rational(1, 2))]);
        var first = ValueWire.ToWire(value, "v");
        var pristine = first.Clone();

        first.Sequence.Elements[0].StringValue = "changed";
        first.Sequence.Elements[1].Quantity.UnitTerm.Factors.Clear();
        ValueWire.Fit(first, Capabilities.None);

        Assert.Equal(pristine, ValueWire.ToWire(value, "v"));
        Assert.Equal(new Value.TextValue("a"), value.Elements[0]);
        Assert.Equal(ValueArmTests.Metres(2), Assert.IsType<Value.QuantityValue>(value.Elements[1]).Quantity);
    }

    [Fact]
    public void ARealIsSentBitForBit()
    {
        double[] reals = [-0.0, 0.0, double.NaN, double.PositiveInfinity, double.NegativeInfinity, double.Epsilon, BitConverter.Int64BitsToDouble(unchecked((long)0xFFF8_0000_0000_0001))];

        foreach (var real in reals)
        {
            var wire = Google.Protobuf.MessageExtensions.ToByteArray(ValueWire.ToWire(new Value.RealValue(real), "v"));
            var read = Assert.IsType<Value.RealValue>(ValueWire.FromWire(Wire.Value.Parser.ParseFrom(wire), ValueArmTests.Method, "r"));
            Assert.Equal(BitConverter.DoubleToInt64Bits(real), BitConverter.DoubleToInt64Bits(read.Value));
        }
    }

    [Fact]
    public void RepeatedAndMapFieldsAreWrittenInOrderWithTheirPositionsInThePath()
    {
        var map = new ValueDictionary<Value>([KeyValuePair.Create<string, Value>("b", new Value.IntegerValue(2)), KeyValuePair.Create<string, Value>("a", new Value.IntegerValue(1))]);

        Assert.Equal(["a", "b"], ValueWire.ToWire(map, "inputs").Select(entry => entry.Key));
        Assert.Equal([1L, 2L], ValueWire.ToWire(new ValueList<Value>([new Value.IntegerValue(1), new Value.IntegerValue(2)]), "arguments").Select(value => value.IntValue));
        Assert.StartsWith("The value at inputs[\"z\"] is unset", Assert.Throws<InvalidRequestException>(() => ValueWire.ToWire(
            new ValueDictionary<Value>([KeyValuePair.Create<string, Value>("z", new Value.UnsetValue())]), "inputs")).Message, StringComparison.Ordinal);
        Assert.StartsWith("The value at arguments[1] is unset", Assert.Throws<InvalidRequestException>(() => ValueWire.ToWire(
            new ValueList<Value>([new Value.IntegerValue(1), new Value.UnsetValue()]), "arguments")).Message, StringComparison.Ordinal);
    }

    private static Capabilities Without(string capability) => new(ServiceCapabilities.Names.Where(name => name != capability));

    private static Wire.Value Fit(Value value, Capabilities capabilities)
    {
        var wire = ValueWire.ToWire(value, "v");
        ValueWire.Fit(wire, capabilities);
        return wire;
    }
}
