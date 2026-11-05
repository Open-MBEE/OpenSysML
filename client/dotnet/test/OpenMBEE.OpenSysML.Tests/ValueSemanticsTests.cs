using System;
using System.Collections.Generic;
using System.Linq;
using System.Numerics;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// What a value is, independent of the wire: equality and hashing under .NET's numeric semantics,
/// identity where the schema names one, the invariants each arm refuses to break, immutability, and
/// display that no culture changes.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed class ValueSemanticsTests
{
    [Fact]
    public void ARealEqualsAsDoubleEqualsDoesNaNEqualToNaNAndTheZerosEqualButEachKeptAndShown()
    {
        var negativeZero = new Value.RealValue(-0.0);
        var positiveZero = new Value.RealValue(0.0);
        var nan = new Value.RealValue(double.NaN);

        Assert.Equal(nan, new Value.RealValue(BitConverter.Int64BitsToDouble(unchecked((long)0xFFF8_0000_0000_0001))));
        Assert.Equal(nan.GetHashCode(), new Value.RealValue(BitConverter.Int64BitsToDouble(unchecked((long)0xFFF8_0000_0000_0001))).GetHashCode());
        Assert.Equal(negativeZero, positiveZero);
        Assert.Equal(negativeZero.GetHashCode(), positiveZero.GetHashCode());
        Assert.True(double.IsNegative(negativeZero.Value));
        Assert.Equal(("-0.0", "0.0", "NaN"), (negativeZero.ToString(), positiveZero.ToString(), nan.ToString()));
        Assert.Equal(new Value.ComplexValue(-0.0, double.NaN), new Value.ComplexValue(0.0, double.NaN));
        Assert.Equal("-0.0 - NaNi", new Value.ComplexValue(-0.0, BitConverter.Int64BitsToDouble(unchecked((long)0xFFF8_0000_0000_0000))).ToString());
        Assert.Equal("0.0 + NaNi", new Value.ComplexValue(0.0, BitConverter.Int64BitsToDouble(0x7FF8_0000_0000_0000)).ToString());
        Assert.Equal(new Quantity(new Value.RealValue(double.NaN), "m", null), new Quantity(new Value.RealValue(double.NaN), "m", null));
        Assert.NotEqual<Value>(new Value.RealValue(double.PositiveInfinity), new Value.InfinityValue());
    }

    [Fact]
    public void EqualityIsStructuralSoAnIntegerIsNoRealAndARationalIsNoReal()
    {
        Assert.NotEqual<Value>(new Value.IntegerValue(1), new Value.RealValue(1.0));
        Assert.NotEqual<Value>(new Value.RationalValue(new Rational(1, 2)), new Value.RealValue(0.5));
        Assert.NotEqual<Value>(new Value.ComplexValue(1, 0), new Value.RealValue(1));
        Assert.NotEqual<Value>(new Value.NullValue(), new Value.SequenceValue([]));
        Assert.Equal(2, new Value.SetValue([new Value.IntegerValue(1), new Value.RealValue(1.0)]).Count);
    }

    [Fact]
    public void ASetIsTheSameInEveryOrderAndKeepsTheOrderItWasGiven()
    {
        Value[] members = [new Value.IntegerValue(1), new Value.TextValue("b"), new Value.SequenceValue([new Value.BooleanValue(true)]), new Value.RealValue(-0.5)];
        var first = new Value.SetValue(members);

        foreach (var order in Permutations(members))
        {
            var permuted = new Value.SetValue(order);
            Assert.Equal(first, permuted);
            Assert.Equal(first.GetHashCode(), permuted.GetHashCode());
            Assert.Equal($"{{{string.Join(", ", order.Select(member => member.ToString()))}}}", permuted.ToString());
            Assert.Equal(order, permuted.Elements);
        }

        Assert.NotEqual(first, new Value.SetValue(members[..3]));
        Assert.NotEqual(first, new Value.SetValue([.. members[..3], new Value.RealValue(0.5)]));
        Assert.NotEqual<Value>(first, new Value.SequenceValue(members));
    }

    [Fact]
    public void ASetRefusesAMemberItHoldsAlready()
    {
        Assert.Throws<ArgumentException>(() => new Value.SetValue([new Value.IntegerValue(1), new Value.IntegerValue(1)]));
        Assert.Throws<ArgumentException>(() => new Value.SetValue([new Value.RealValue(0.0), new Value.RealValue(-0.0)]));
        Assert.Throws<ArgumentException>(() => new Value.SetValue([new Value.RealValue(double.NaN), new Value.RealValue(double.NaN)]));
        Assert.Throws<ArgumentException>(() => new Value.SetValue([new Value.SequenceValue([new Value.TextValue("a")]), new Value.SequenceValue([new Value.TextValue("a")])]));
        Assert.Throws<ArgumentException>(() => new Value.SetValue([new Value.MetaobjectValue("V::a", "M::A"), new Value.MetaobjectValue("V::a", "M::B")]));
        Assert.Throws<ArgumentException>(() => new Value.SetValue([new Value.IntegerValue(1), null!]));
    }

    [Fact]
    public void AnIdentityTheSchemaNamesIsTheWholeEquality()
    {
        Assert.Equal(new Value.EnumLiteralValue("D::C::red", "D::C", "C::red", new Value.IntegerValue(1)), new Value.EnumLiteralValue("D::C::red", "X", "red"));
        Assert.NotEqual(new Value.EnumLiteralValue("D::C::red", "D::C", "C::red"), new Value.EnumLiteralValue("D::C::Red", "D::C", "C::red"));
        Assert.Equal(new Value.MetaobjectValue("V::a", "SysML::PartUsage"), new Value.MetaobjectValue("V::a"));
        Assert.Equal(new Value.MetaobjectValue("V::a", "SysML::PartUsage").GetHashCode(), new Value.MetaobjectValue("V::a").GetHashCode());
        Assert.NotEqual(new Value.FunctionValue("F::f"), new Value.FunctionValue("F::f", 1));
        Assert.NotEqual(new Value.FunctionValue("F::f", 2), new Value.FunctionValue("F::f", 1));
        Assert.NotEqual(new Value.MeasurementRefValue("km", UnitTerm.One, "SI::kilometre"), new Value.MeasurementRefValue("km", UnitTerm.One));
    }

    [Fact]
    public void FromIntegerChoosesTheArmAtTheInt64Boundary()
    {
        Assert.Equal(new Value.IntegerValue(long.MaxValue), Value.FromInteger(long.MaxValue));
        Assert.Equal(new Value.IntegerValue(long.MinValue), Value.FromInteger(long.MinValue));
        Assert.IsType<Value.BigIntegerValue>(Value.FromInteger(new BigInteger(long.MaxValue) + 1));
        Assert.IsType<Value.BigIntegerValue>(Value.FromInteger(new BigInteger(long.MinValue) - 1));
        Assert.Throws<ArgumentOutOfRangeException>(() => new Value.BigIntegerValue(long.MaxValue));
        Assert.Throws<ArgumentOutOfRangeException>(() => new Value.BigIntegerValue(long.MinValue));
    }

    [Fact]
    public void ANumberIsItsNearestDouble()
    {
        Assert.Equal(-7.0, new Value.IntegerValue(-7).ToDouble());
        Assert.Equal(9007199254740992.0, new Value.BigIntegerValue((BigInteger.One << 64) + 1).ToDouble() / 2048);
        Assert.Equal(18446744073709551616.0, new Value.BigIntegerValue((BigInteger.One << 64) + 2047).ToDouble());
        Assert.Equal(1.0 / 3, new Value.RationalValue(new Rational(1, 3)).ToDouble());
        Assert.Equal(-0.0, new Value.RealValue(-0.0).ToDouble());
    }

    [Fact]
    public void EachArmRefusesWhatItCannotBe()
    {
        Assert.Throws<ArgumentException>(() => new Value.FunctionValue(string.Empty));
        Assert.Throws<ArgumentNullException>(() => new Value.FunctionValue(null!));
        Assert.Throws<ArgumentOutOfRangeException>(() => new Value.FunctionValue("F::f", 0));
        Assert.Throws<ArgumentException>(() => new Value.MetaobjectValue(string.Empty));
        Assert.Throws<ArgumentException>(() => new Value.EnumLiteralValue(string.Empty, "D::C", "C::x"));
        Assert.Throws<ArgumentException>(() => new Value.MeasurementRefValue("km", UnitTerm.One, string.Empty));
        Assert.Throws<ArgumentNullException>(() => new Value.MeasurementRefValue("km", null!));
        Assert.Throws<ArgumentNullException>(() => new Value.TextValue(null!));
        Assert.Throws<ArgumentNullException>(() => new Value.NullValue(null!));
        Assert.Throws<ArgumentNullException>(() => new Value.QuantityValue(null!));
        Assert.Throws<ArgumentNullException>(() => new Quantity(null!, "m", null));
        Assert.Throws<ArgumentNullException>(() => new UnitFactor(null!, 1));
        Assert.Throws<ArgumentException>(() => new Value.SequenceValue([new Value.IntegerValue(1), null!]));
        Assert.Throws<ArgumentException>(() => new Value.VectorQuantityValue([]));
        Assert.Throws<ArgumentException>(() => new Value.VectorValue([null!]));
    }

    [Fact]
    public void AnArrayOrATensorRefusesAShapeItsElementsDoNotFill()
    {
        Value[] four = [.. Enumerable.Range(1, 4).Select(i => new Value.IntegerValue(i))];

        Assert.Contains("holds 4 elements, not 6", Assert.Throws<ArgumentException>(() => new Value.ArrayValue([2, 3], four)).Message, StringComparison.Ordinal);
        Assert.Contains("dimension 1 is 0", Assert.Throws<ArgumentException>(() => new Value.ArrayValue([4, 0], four)).Message, StringComparison.Ordinal);
        Assert.Throws<ArgumentException>(() => new Value.ArrayValue([-4], four));
        Assert.Contains("hold more than", Assert.Throws<ArgumentException>(() => new Value.ArrayValue([1L << 40, 1L << 40], four)).Message, StringComparison.Ordinal);
        Assert.Throws<ArgumentException>(() => new Value.ArrayValue([], four));
        Assert.Throws<ArgumentException>(() => new Value.TensorQuantityValue([2], [ValueArmTests.Metres(1)]));
        Assert.Throws<ArgumentException>(() => new Value.TensorQuantityValue([0], []));
        Assert.Equal(four[0], new Value.ArrayValue([], four[..1]).Get());
        Assert.Equal(ValueArmTests.Metres(1), new Value.TensorQuantityValue([], [ValueArmTests.Metres(1)]).Get());
    }

    [Fact]
    public void AnIndexOfTheWrongRankOrOutsideTheShapeIsRefused()
    {
        var array = new Value.ArrayValue([2, 3], Enumerable.Range(0, 6).Select(i => new Value.IntegerValue(i)));
        var tensor = new Value.TensorQuantityValue([3, 1], [ValueArmTests.Metres(0), ValueArmTests.Metres(1), ValueArmTests.Metres(2)]);

        Assert.Equal(new Value.IntegerValue(5), array.Get(1, 2));
        Assert.Equal(new Value.IntegerValue(3), array.Get(1, 0));
        Assert.Equal(ValueArmTests.Metres(2), tensor.Get(2, 0));
        Assert.Contains("has rank 2", Assert.Throws<ArgumentException>(() => array.Get(1)).Message, StringComparison.Ordinal);
        Assert.Throws<ArgumentException>(() => array.Get(0, 0, 0));
        Assert.Throws<ArgumentOutOfRangeException>(() => array.Get(2, 0));
        Assert.Throws<ArgumentOutOfRangeException>(() => array.Get(0, 3));
        Assert.Throws<ArgumentOutOfRangeException>(() => array.Get(-1, 0));
        Assert.Throws<ArgumentOutOfRangeException>(() => tensor.Get(0, 1));
        Assert.Throws<ArgumentException>(() => tensor.Get());
    }

    [Fact]
    public void AValueCopiesWhatItIsBuiltFrom()
    {
        var elements = new List<Value> { new Value.IntegerValue(1), new Value.IntegerValue(2) };
        var dimensions = new List<long> { 2 };
        var factors = new List<UnitFactor> { new("SI::m", 1) };
        var sequence = new Value.SequenceValue(elements);
        var set = new Value.SetValue(elements);
        var array = new Value.ArrayValue(dimensions, elements);
        var term = new UnitTerm(1, 1, factors);

        elements[0] = new Value.TextValue("changed");
        elements.Add(new Value.IntegerValue(3));
        dimensions[0] = 3;
        factors.Clear();

        Assert.Equal("(1, 2)", sequence.ToString());
        Assert.Equal("{1, 2}", set.ToString());
        Assert.True(set.Contains(new Value.IntegerValue(1)));
        Assert.Equal("Array(2)[1, 2]", array.ToString());
        Assert.Single(term.Factors);
    }

    [Fact]
    public void NoArmCanBeChangedAfterItIsBuilt()
    {
        foreach (var type in typeof(Value).GetNestedTypes().Append(typeof(Quantity)).Append(typeof(UnitTerm)).Append(typeof(UnitFactor)).Append(typeof(Rational)))
        {
            Assert.All(type.GetProperties(), property => Assert.False(property.CanWrite, $"{type.Name}.{property.Name} has a setter."));
            Assert.All(
                type.GetFields(System.Reflection.BindingFlags.Instance | System.Reflection.BindingFlags.Public | System.Reflection.BindingFlags.NonPublic),
                field => Assert.True(field.IsInitOnly, $"{type.Name}.{field.Name} is not read-only."));
        }
    }

    [Fact]
    public void AValueIsShownTheSameWhateverTheCulture()
    {
        Value[] values =
        [
            new Value.IntegerValue(-3),
            new Value.RealValue(-1.5),
            new Value.RealValue(1e20),
            new Value.RealValue(1e-7),
            new Value.RealValue(double.NegativeInfinity),
            new Value.ComplexValue(-1.5, 2.25),
            new Value.BigIntegerValue(-(BigInteger.One << 70)),
            new Value.RationalValue(new Rational(-1, 3)),
            new Value.VectorValue([new Value.RealValue(-0.5), new Value.IntegerValue(-2)]),
            new Value.QuantityValue(new Quantity(new Value.RealValue(-2.5), "m", new UnitTerm(0.5, 3, [new UnitFactor("SI::m", -1.5)]))),
            new Value.ArrayValue([2], [new Value.RealValue(1.25), new Value.IntegerValue(-1)]),
        ];
        var invariant = values.Select(value => value.ToString()).ToList();

        using (new CultureScope("sv-SE"))
        {
            Assert.NotEqual("-1.5", (-1.5).ToString(System.Globalization.CultureInfo.CurrentCulture));
            Assert.Equal(invariant, values.Select(value => value.ToString()));
            Assert.Equal("[-0.5, 1E+20]", new ValueList<double>([-0.5, 1e20]).ToString());
            Assert.Equal("0.5/3 SI::m^-1.5", new UnitTerm(0.5, 3, [new UnitFactor("SI::m", -1.5)]).ToString());
        }

        Assert.Equal(
            ["-3", "-1.5", "1E+20", "1E-07", "-Infinity", "-1.5 + 2.25i", "-1180591620717411303424", "-1/3", "Vector(-0.5, -2)", "-2.5 [m]", "Array(2)[1.25, -1]"],
            invariant);
    }

    private static IEnumerable<Value[]> Permutations(Value[] items)
    {
        if (items.Length <= 1)
        {
            yield return items;
            yield break;
        }

        for (var i = 0; i < items.Length; i++)
        {
            foreach (var rest in Permutations([.. items[..i], .. items[(i + 1)..]]))
            {
                yield return [items[i], .. rest];
            }
        }
    }
}
