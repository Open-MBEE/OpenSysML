using System;
using System.Numerics;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>Exact rationals: canonical terms, ordering, and the one correctly rounded double each is nearest.</summary>
[Trait(Category.Name, Category.Unit)]
public sealed class RationalTests
{
    private static readonly BigInteger TwoTo53 = BigInteger.One << 53;

    [Fact]
    public void ARationalIsHeldInLowestTermsOverAPositiveDenominator()
    {
        Assert.Equal((new BigInteger(-1), new BigInteger(2)), Terms(new Rational(2, -4)));
        Assert.Equal((new BigInteger(1), new BigInteger(3)), Terms(new Rational(-5, -15)));
        Assert.Equal((BigInteger.Zero, BigInteger.One), Terms(new Rational(0, -7)));
        Assert.Equal((new BigInteger(7), BigInteger.One), Terms(new Rational(7, 1)));
        Assert.True(new Rational(14, 2).IsInteger);
        Assert.False(new Rational(1, 2).IsInteger);
        Assert.Equal("denominator", Assert.Throws<ArgumentOutOfRangeException>(() => new Rational(1, 0)).ParamName);
    }

    [Fact]
    public void RationalsAreEqualExactlyWhenTheyAreTheSameNumber()
    {
        Assert.Equal(new Rational(1, 3), new Rational(-2, -6));
        Assert.Equal(new Rational(1, 3).GetHashCode(), new Rational(-2, -6).GetHashCode());
        Assert.True(new Rational(1, 3) == new Rational(2, 6));
        Assert.True(new Rational(1, 3) != new Rational(1, 4));
        Assert.False(new Rational(1, 3).Equals(null));
        Assert.False(new Rational(1, 3).Equals((object)"1/3"));
        Assert.True((Rational?)null == null);
    }

    [Fact]
    public void RationalsAreOrderedByValueAndNullIsLeast()
    {
        var third = new Rational(1, 3);
        var half = new Rational(1, 2);

        Assert.True(third < half && half > third && third <= half && half >= third && third <= new Rational(2, 6));
        Assert.True(new Rational(-1, 2) < new Rational(-1, 3));
        Assert.Equal(0, third.CompareTo(new Rational(2, 6)));
        Assert.Equal(1, third.CompareTo(null));
        Assert.Equal(1, third.CompareTo((object?)null));
        Assert.Equal(-1, third.CompareTo((object)half));
        Assert.True(null < third);
        Assert.Throws<ArgumentException>(() => third.CompareTo((object)0.5));
    }

    [Fact]
    public void ARationalIsSpelledAsItsTermsWhateverTheCulture()
    {
        using var culture = new CultureScope("sv-SE");

        Assert.NotEqual("-1", (-1).ToString(System.Globalization.CultureInfo.CurrentCulture));
        Assert.Equal("-1/2", new Rational(2, -4).ToString());
        Assert.Equal("0/1", new Rational(0, 3).ToString());
    }

    [Fact]
    public void TheNearestDoubleIsTheCorrectlyRoundedQuotient()
    {
        for (long numerator = -97; numerator <= 97; numerator += 3)
        {
            for (long denominator = 1; denominator <= 1_000_003; denominator = (denominator * 7) + 3)
            {
                Assert.Equal((double)numerator / denominator, new Rational(numerator, denominator).ToDouble());
            }
        }

        var large = (BigInteger.One << 52) + 12345;
        Assert.Equal((double)(long)large / 3.0, new Rational(large, 3).ToDouble());
        Assert.Equal(0.1, new Rational(1, 10).ToDouble());
        Assert.Equal(-0.1, new Rational(-1, 10).ToDouble());
        Assert.Equal(0.0, new Rational(0, 5).ToDouble());
    }

    [Fact]
    public void AHalfwayQuotientRoundsToEven()
    {
        Assert.Equal(9007199254740992.0, new Rational(TwoTo53 + 1, 1).ToDouble());
        Assert.Equal(9007199254740996.0, new Rational(TwoTo53 + 3, 1).ToDouble());
        Assert.Equal(-9007199254740992.0, new Rational(-(TwoTo53 + 1), 1).ToDouble());
        Assert.Equal(9007199254740994.0, new Rational((2 * TwoTo53) + 3, 2).ToDouble());
        Assert.Equal(9007199254740994.0, new Rational((4 * TwoTo53) + 5, 4).ToDouble());
        Assert.Equal(9007199254740996.0, new Rational((4 * TwoTo53) + 13, 4).ToDouble());
    }

    [Fact]
    public void ARationalBeyondTheFiniteRangeIsInfiniteAndJustBelowItIsTheLargestDouble()
    {
        var halfwayAboveMax = ((BigInteger.One << 54) - 1) << 970;

        Assert.Equal(double.PositiveInfinity, new Rational(halfwayAboveMax, 1).ToDouble());
        Assert.Equal(double.MaxValue, new Rational(halfwayAboveMax - 1, 1).ToDouble());
        Assert.Equal(double.NegativeInfinity, new Rational(-(BigInteger.One << 1024), 1).ToDouble());
        Assert.Equal(double.MaxValue, new Rational(((BigInteger.One << 53) - 1) << 971, 1).ToDouble());
    }

    [Fact]
    public void ARationalBelowTheNormalRangeRoundsOnceToASubnormal()
    {
        var subnormalUnit = BigInteger.One << 1074;

        Assert.Equal(double.Epsilon, new Rational(1, subnormalUnit).ToDouble());
        Assert.Equal(0.0, new Rational(1, subnormalUnit * 2).ToDouble());
        Assert.Equal(double.Epsilon, new Rational(3, subnormalUnit * 4).ToDouble());
        Assert.Equal(2 * double.Epsilon, new Rational(3, subnormalUnit * 2).ToDouble());
        Assert.Equal(2 * double.Epsilon, new Rational(5, subnormalUnit * 2).ToDouble());
        Assert.Equal(0.0, new Rational(1, subnormalUnit * 3).ToDouble());
        Assert.Equal(-double.Epsilon, new Rational(-1, subnormalUnit).ToDouble());
        Assert.Equal(2.2250738585072014E-308, new Rational(1, BigInteger.One << 1022).ToDouble());
        Assert.Equal(0.0, new Rational(1, BigInteger.Pow(10, 5000)).ToDouble());
        Assert.Equal(1E-310, new Rational(1, BigInteger.Pow(10, 310)).ToDouble());
    }

    [Fact]
    public void ADoubleHoldsARationalExactlyOnlyWhenItIsADyadicFractionInRange()
    {
        Assert.True(new Rational(1, 2).TryGetExactDouble(out var half));
        Assert.Equal(0.5, half);
        Assert.True(new Rational(-3, 1).TryGetExactDouble(out var three));
        Assert.Equal(-3.0, three);
        Assert.True(new Rational(1, BigInteger.One << 1074).TryGetExactDouble(out var tiny));
        Assert.Equal(double.Epsilon, tiny);
        Assert.True(new Rational(TwoTo53, 1).TryGetExactDouble(out _));
        Assert.False(new Rational(TwoTo53 + 1, 1).TryGetExactDouble(out _));
        Assert.False(new Rational(1, 3).TryGetExactDouble(out _));
        Assert.False(new Rational(1, 10).TryGetExactDouble(out _));
        Assert.False(new Rational(1, BigInteger.One << 1075).TryGetExactDouble(out _));
        Assert.False(new Rational(BigInteger.One << 1024, 1).TryGetExactDouble(out _));
        Assert.Equal(new Rational(3602879701896397, 36028797018963968), Rational.FromExactDouble(0.1));
    }

    [Theory]
    [InlineData(0.1)]
    [InlineData(-0.0)]
    [InlineData(double.Epsilon)]
    [InlineData(double.MaxValue)]
    [InlineData(-123.456)]
    [InlineData(2.2250738585072014E-308)]
    public void EveryFiniteDoubleIsARationalThatConvertsBackToIt(double value)
    {
        var exact = Rational.FromExactDouble(value);

        Assert.True(exact.TryGetExactDouble(out var back));
        Assert.Equal(Math.Abs(value), Math.Abs(back));
        Assert.Equal(value == 0 ? 0 : Math.Sign(value), exact.Numerator.Sign);
    }

    private static (BigInteger, BigInteger) Terms(Rational rational) => (rational.Numerator, rational.Denominator);
}
