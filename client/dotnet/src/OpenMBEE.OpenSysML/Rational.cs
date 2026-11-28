using System;
using System.Diagnostics.CodeAnalysis;
using System.Globalization;
using System.Numerics;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// An exact KerML <c>Rational</c>: a numerator over a positive denominator in lowest terms, so two
/// rationals are the same number exactly when they are equal. It is never converted through a
/// <see cref="double"/>.
/// </summary>
/// <remarks>
/// The service answers a Rational a <see cref="double"/> holds exactly as a Real, so only one no
/// double holds, such as <c>1/3</c> or <c>1/10</c>, arrives as this. A caller may send any.
/// </remarks>
public sealed class Rational : IEquatable<Rational>, IComparable<Rational>, IComparable
{
    private const int SignificandBits = 53;
    private const int MinExponent = -1022;
    private const int MaxExponent = 1023;
    private const int SubnormalExponent = -1074;

    /// <summary>Creates <c>numerator / denominator</c>, reduced to lowest terms over a positive denominator.</summary>
    /// <param name="numerator">The numerator.</param>
    /// <param name="denominator">The denominator, not zero.</param>
    /// <exception cref="ArgumentOutOfRangeException">The denominator is zero.</exception>
    public Rational(BigInteger numerator, BigInteger denominator)
    {
        if (denominator.IsZero)
        {
            throw new ArgumentOutOfRangeException(nameof(denominator), "A rational's denominator is zero.");
        }

        var divisor = BigInteger.GreatestCommonDivisor(numerator, denominator);
        if (denominator.Sign < 0)
        {
            divisor = -divisor;
        }

        Numerator = numerator / divisor;
        Denominator = denominator / divisor;
    }

    /// <summary>The numerator in lowest terms, signed.</summary>
    public BigInteger Numerator { get; }

    /// <summary>The denominator in lowest terms, positive.</summary>
    public BigInteger Denominator { get; }

    /// <summary>Whether this rational is a whole number.</summary>
    public bool IsInteger => Denominator.IsOne;

    /// <summary>The nearest <see cref="double"/>, ties to even; infinite only beyond the finite range.</summary>
    /// <returns>The nearest double.</returns>
    public double ToDouble()
    {
        if (Numerator.IsZero)
        {
            return 0.0;
        }

        var magnitude = BigInteger.Abs(Numerator);
        var shift = SignificandBits + 1 - (int)(magnitude.GetBitLength() - Denominator.GetBitLength());
        var quotient = BigInteger.DivRem(
            shift >= 0 ? magnitude << shift : magnitude,
            shift >= 0 ? Denominator : Denominator << -shift,
            out var remainder);

        // The quotient holds 54 or 55 bits worth 2^-shift each; keep 53, or fewer below the normal range.
        var bits = (int)quotient.GetBitLength();
        var exponent = -shift;
        var leading = bits - 1 + exponent;
        if (leading > MaxExponent)
        {
            return Numerator.Sign < 0 ? double.NegativeInfinity : double.PositiveInfinity;
        }

        var dropped = leading >= MinExponent ? bits - SignificandBits : SubnormalExponent - exponent;
        var kept = dropped >= bits ? BigInteger.Zero : quotient >> dropped;
        var half = BigInteger.One << (dropped - 1);
        var low = quotient & ((BigInteger.One << dropped) - 1);
        var comparison = low.CompareTo(half);
        if (comparison > 0 || (comparison == 0 && (!remainder.IsZero || !kept.IsEven)))
        {
            kept += 1;
        }

        var result = Math.ScaleB((double)(long)kept, exponent + dropped);
        return Numerator.Sign < 0 ? -result : result;
    }

    /// <summary>Whether a <see cref="double"/> holds this rational exactly, and that double.</summary>
    /// <param name="value">The double this rational is, when one holds it.</param>
    /// <returns>Whether a double holds it exactly.</returns>
    public bool TryGetExactDouble(out double value)
    {
        value = ToDouble();
        return double.IsFinite(value) && Equals(FromExactDouble(value));
    }

    /// <summary>The rational a finite <see cref="double"/> is exactly.</summary>
    /// <param name="value">A finite double.</param>
    /// <returns>The rational.</returns>
    internal static Rational FromExactDouble(double value)
    {
        var bits = BitConverter.DoubleToInt64Bits(value);
        var biased = (int)((bits >> 52) & 0x7FF);
        var significand = bits & 0xF_FFFF_FFFF_FFFF;
        if (biased != 0)
        {
            significand |= 1L << 52;
        }

        var exponent = (biased == 0 ? 1 : biased) - 1075;
        BigInteger numerator = value < 0 ? -significand : significand;
        return exponent >= 0
            ? new Rational(numerator << exponent, BigInteger.One)
            : new Rational(numerator, BigInteger.One << -exponent);
    }

    /// <inheritdoc/>
    public int CompareTo(Rational? other) =>
        other is null ? 1 : (Numerator * other.Denominator).CompareTo(other.Numerator * Denominator);

    /// <inheritdoc/>
    public int CompareTo(object? obj) => obj switch
    {
        null => 1,
        Rational other => CompareTo(other),
        _ => throw new ArgumentException("A rational compares only with a rational.", nameof(obj)),
    };

    /// <inheritdoc/>
    public bool Equals([NotNullWhen(true)] Rational? other) =>
        other is not null && Numerator == other.Numerator && Denominator == other.Denominator;

    /// <inheritdoc/>
    public override bool Equals([NotNullWhen(true)] object? obj) => Equals(obj as Rational);

    /// <inheritdoc/>
    public override int GetHashCode() => HashCode.Combine(Numerator, Denominator);

    /// <summary><c>numerator/denominator</c>, as the wire spells a Rational's terms.</summary>
    /// <returns>The rational's text.</returns>
    public override string ToString() =>
        string.Create(CultureInfo.InvariantCulture, $"{Numerator}/{Denominator}");

    /// <summary>Whether two rationals are the same number.</summary>
    /// <param name="left">A rational.</param>
    /// <param name="right">Another rational.</param>
    /// <returns>Whether they are equal.</returns>
    public static bool operator ==(Rational? left, Rational? right) => left is null ? right is null : left.Equals(right);

    /// <summary>Whether two rationals are different numbers.</summary>
    /// <param name="left">A rational.</param>
    /// <param name="right">Another rational.</param>
    /// <returns>Whether they differ.</returns>
    public static bool operator !=(Rational? left, Rational? right) => !(left == right);

    /// <summary>Whether one rational is less than another; null is less than any rational.</summary>
    /// <param name="left">A rational.</param>
    /// <param name="right">Another rational.</param>
    /// <returns>Whether <paramref name="left"/> is the lesser.</returns>
    public static bool operator <(Rational? left, Rational? right) => Compare(left, right) < 0;

    /// <summary>Whether one rational is at most another; null is less than any rational.</summary>
    /// <param name="left">A rational.</param>
    /// <param name="right">Another rational.</param>
    /// <returns>Whether <paramref name="left"/> is at most <paramref name="right"/>.</returns>
    public static bool operator <=(Rational? left, Rational? right) => Compare(left, right) <= 0;

    /// <summary>Whether one rational is greater than another; null is less than any rational.</summary>
    /// <param name="left">A rational.</param>
    /// <param name="right">Another rational.</param>
    /// <returns>Whether <paramref name="left"/> is the greater.</returns>
    public static bool operator >(Rational? left, Rational? right) => Compare(left, right) > 0;

    /// <summary>Whether one rational is at least another; null is less than any rational.</summary>
    /// <param name="left">A rational.</param>
    /// <param name="right">Another rational.</param>
    /// <returns>Whether <paramref name="left"/> is at least <paramref name="right"/>.</returns>
    public static bool operator >=(Rational? left, Rational? right) => Compare(left, right) >= 0;

    private static int Compare(Rational? left, Rational? right) =>
        left is null ? (right is null ? 0 : -1) : left.CompareTo(right);
}
