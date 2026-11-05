using System;
using System.Diagnostics.CodeAnalysis;
using System.Globalization;
using OpenMBEE.OpenSysML.Internal;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// A value the service evaluated, or one sent to it: an immutable variant of the service's
/// <c>Value</c>, one nested sealed class per arm, which <see cref="Kind"/> names.
/// </summary>
/// <remarks>
/// <para>No type outside this library can derive from it, so the arms are exactly the nested
/// classes. A switch over them still needs a default: a later release adds an arm whenever the
/// service does, and <see cref="ValueKind"/> is a discriminator, not an exhaustiveness guarantee.</para>
/// <para>Values compare by content. A Real is compared as <see cref="double.Equals(double)"/> does,
/// so NaN equals NaN and <c>0.0</c> equals <c>-0.0</c>, while each is kept, and sent, exactly as
/// given. An Integer and a Real are never equal, whatever their magnitudes: equality is structural,
/// not the model's numeric comparison.</para>
/// </remarks>
public abstract partial class Value : IEquatable<Value>
{
    private Value()
    {
    }

    /// <summary>Which kind of value this is.</summary>
    public abstract ValueKind Kind { get; }

    /// <summary>The Integer an integer is: an <see cref="IntegerValue"/> within <see cref="long"/>, else a <see cref="BigIntegerValue"/>.</summary>
    /// <param name="value">The integer.</param>
    /// <returns>The value.</returns>
    public static NumberValue FromInteger(System.Numerics.BigInteger value) =>
        value >= long.MinValue && value <= long.MaxValue ? new IntegerValue((long)value) : new BigIntegerValue(value);

    /// <inheritdoc/>
    public abstract bool Equals([NotNullWhen(true)] Value? other);

    /// <inheritdoc/>
    public sealed override bool Equals([NotNullWhen(true)] object? obj) => obj is Value other && Equals(other);

    /// <inheritdoc/>
    public abstract override int GetHashCode();

    /// <summary>The value as a reader writes it, culture-invariant: <c>42</c>, <c>4.0</c>, <c>"text"</c>, <c>5.4 [km/h]</c>.</summary>
    /// <returns>The value's text.</returns>
    public abstract override string ToString();

    /// <summary>Whether two values are equal.</summary>
    /// <param name="left">A value.</param>
    /// <param name="right">Another value.</param>
    /// <returns>Whether they are equal.</returns>
    public static bool operator ==(Value? left, Value? right) => left is null ? right is null : left.Equals(right);

    /// <summary>Whether two values differ.</summary>
    /// <param name="left">A value.</param>
    /// <param name="right">Another value.</param>
    /// <returns>Whether they differ.</returns>
    public static bool operator !=(Value? left, Value? right) => !(left == right);

    /// <summary>
    /// A number on the real axis: an <see cref="IntegerValue"/>, a <see cref="BigIntegerValue"/>, a
    /// <see cref="RationalValue"/> or a <see cref="RealValue"/>. It is what a quantity's magnitude
    /// and a vector's components are.
    /// </summary>
    public abstract class NumberValue : Value
    {
        private protected NumberValue()
        {
        }

        /// <summary>The nearest <see cref="double"/>, ties to even; infinite only beyond the finite range.</summary>
        /// <returns>The nearest double.</returns>
        public abstract double ToDouble();
    }

    /// <summary>An Integer within <see cref="long"/>.</summary>
    public sealed class IntegerValue : NumberValue
    {
        /// <summary>Creates an Integer.</summary>
        /// <param name="value">The integer.</param>
        public IntegerValue(long value) => Value = value;

        /// <summary>The integer.</summary>
        public long Value { get; }

        /// <inheritdoc/>
        public override ValueKind Kind => ValueKind.Integer;

        /// <inheritdoc/>
        public override double ToDouble() => Value;

        /// <inheritdoc/>
        public override bool Equals([NotNullWhen(true)] Value? other) => other is IntegerValue that && Value == that.Value;

        /// <inheritdoc/>
        public override int GetHashCode() => HashCode.Combine(Kind, Value);

        /// <summary>The integer, as <c>42</c>.</summary>
        /// <returns>The integer's text.</returns>
        public override string ToString() => ValueText.Whole(Value);
    }

    /// <summary>
    /// An Integer beyond <see cref="long"/>: KerML Integers are unbounded. An integer within
    /// <see cref="long"/> is always an <see cref="IntegerValue"/>, never this, so two integers are
    /// equal exactly when their kinds and values are; <see cref="FromInteger"/> chooses.
    /// </summary>
    public sealed class BigIntegerValue : NumberValue
    {
        /// <summary>Creates an Integer beyond <see cref="long"/>.</summary>
        /// <param name="value">The integer, beyond <see cref="long"/>.</param>
        /// <exception cref="ArgumentOutOfRangeException">The integer is within <see cref="long"/>, which <see cref="IntegerValue"/> holds.</exception>
        public BigIntegerValue(System.Numerics.BigInteger value)
        {
            if (value >= long.MinValue && value <= long.MaxValue)
            {
                throw new ArgumentOutOfRangeException(nameof(value), value, "The integer is within long, which IntegerValue holds.");
            }

            Value = value;
        }

        /// <summary>The integer.</summary>
        public System.Numerics.BigInteger Value { get; }

        /// <inheritdoc/>
        public override ValueKind Kind => ValueKind.BigInteger;

        /// <inheritdoc/>
        public override double ToDouble() => new Rational(Value, System.Numerics.BigInteger.One).ToDouble();

        /// <inheritdoc/>
        public override bool Equals([NotNullWhen(true)] Value? other) => other is BigIntegerValue that && Value == that.Value;

        /// <inheritdoc/>
        public override int GetHashCode() => HashCode.Combine(Kind, Value);

        /// <summary>The integer's decimal digits, as <c>-18446744073709551616</c>.</summary>
        /// <returns>The integer's text.</returns>
        public override string ToString() => Value.ToString(CultureInfo.InvariantCulture);
    }

    /// <summary>
    /// An exact Rational, such as <c>1/3</c>. The service answers one a <see cref="double"/> holds
    /// exactly as a <see cref="RealValue"/>; one sent as this stays a Rational to a service that
    /// reads Rationals, and is sent as that double to one that does not.
    /// </summary>
    public sealed class RationalValue : NumberValue
    {
        /// <summary>Creates an exact Rational.</summary>
        /// <param name="value">The rational.</param>
        /// <exception cref="ArgumentNullException"><paramref name="value"/> is null.</exception>
        public RationalValue(Rational value)
        {
            ArgumentNullException.ThrowIfNull(value);
            Value = value;
        }

        /// <summary>The rational.</summary>
        public Rational Value { get; }

        /// <inheritdoc/>
        public override ValueKind Kind => ValueKind.Rational;

        /// <inheritdoc/>
        public override double ToDouble() => Value.ToDouble();

        /// <inheritdoc/>
        public override bool Equals([NotNullWhen(true)] Value? other) => other is RationalValue that && Value.Equals(that.Value);

        /// <inheritdoc/>
        public override int GetHashCode() => HashCode.Combine(Kind, Value);

        /// <summary>The rational, as <c>1/3</c>.</summary>
        /// <returns>The rational's text.</returns>
        public override string ToString() => Value.ToString();
    }

    /// <summary>A Real, kept exactly as given: NaN, the infinities and a negative zero included.</summary>
    public sealed class RealValue : NumberValue
    {
        /// <summary>Creates a Real.</summary>
        /// <param name="value">The real.</param>
        public RealValue(double value) => Value = value;

        /// <summary>The real.</summary>
        public double Value { get; }

        /// <inheritdoc/>
        public override ValueKind Kind => ValueKind.Real;

        /// <inheritdoc/>
        public override double ToDouble() => Value;

        /// <summary>Whether another value is a Real equal by <see cref="double.Equals(double)"/>: NaN equals NaN, and <c>0.0</c> equals <c>-0.0</c>.</summary>
        /// <param name="other">The other value.</param>
        /// <returns>Whether they are equal.</returns>
        public override bool Equals([NotNullWhen(true)] Value? other) => other is RealValue that && Value.Equals(that.Value);

        /// <inheritdoc/>
        public override int GetHashCode() => HashCode.Combine(Kind, Value);

        /// <summary>The real, shortest round-trip, as <c>4.0</c>, <c>1E+20</c>, <c>-0.0</c>, <c>NaN</c> or <c>-Infinity</c>.</summary>
        /// <returns>The real's text.</returns>
        public override string ToString() => ValueText.Real(Value);
    }

    /// <summary>A complex number in rectangular form: one value, never a sequence of two Reals.</summary>
    public sealed class ComplexValue : Value
    {
        /// <summary>Creates a complex number.</summary>
        /// <param name="real">The real part.</param>
        /// <param name="imaginary">The imaginary part.</param>
        public ComplexValue(double real, double imaginary)
        {
            Real = real;
            Imaginary = imaginary;
        }

        /// <summary>The real part.</summary>
        public double Real { get; }

        /// <summary>The imaginary part.</summary>
        public double Imaginary { get; }

        /// <inheritdoc/>
        public override ValueKind Kind => ValueKind.Complex;

        /// <summary>Whether another value is a complex number with parts equal by <see cref="double.Equals(double)"/>.</summary>
        /// <param name="other">The other value.</param>
        /// <returns>Whether they are equal.</returns>
        public override bool Equals([NotNullWhen(true)] Value? other) =>
            other is ComplexValue that && Real.Equals(that.Real) && Imaginary.Equals(that.Imaginary);

        /// <inheritdoc/>
        public override int GetHashCode() => HashCode.Combine(Kind, Real, Imaginary);

        /// <summary>The number as SysML writes it, <c>1.5 - 2.0i</c>; the sign between the parts is the imaginary part's.</summary>
        /// <returns>The number's text.</returns>
        public override string ToString() =>
            $"{ValueText.Real(Real)} {(double.IsNegative(Imaginary) ? '-' : '+')} {ValueText.Real(Math.Abs(Imaginary))}i";
    }
}
