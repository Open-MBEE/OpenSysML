using System;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// A magnitude and the measurement unit it is expressed in, as written: <c>5.4 [km/h]</c> is 5.4
/// <c>km/h</c>, never 1.5 <c>m/s</c>.
/// </summary>
/// <remarks>
/// The magnitude keeps an Integer, an Integer beyond <see cref="long"/>, an exact Rational and a
/// Real apart, as <see cref="Value"/> does; none is converted on the way in or out.
/// </remarks>
public sealed record Quantity
{
    /// <summary>Creates a quantity.</summary>
    /// <param name="magnitude">The magnitude.</param>
    /// <param name="unit">The unit as written (<c>km/h</c>); empty for one never written down.</param>
    /// <param name="reduction">What the unit reduces to; null when the service sent none. A named unit
    /// cannot be sent to the service without it.</param>
    /// <exception cref="ArgumentNullException"><paramref name="magnitude"/> or <paramref name="unit"/> is null.</exception>
    public Quantity(Value.NumberValue magnitude, string unit, UnitTerm? reduction)
    {
        ArgumentNullException.ThrowIfNull(magnitude);
        ArgumentNullException.ThrowIfNull(unit);
        Magnitude = magnitude;
        Unit = unit;
        Reduction = reduction;
    }

    /// <summary>The magnitude: an Integer, an Integer beyond <see cref="long"/>, an exact Rational or a Real.</summary>
    public Value.NumberValue Magnitude { get; }

    /// <summary>The unit as written (<c>km/h</c>); empty for one never written down.</summary>
    public string Unit { get; }

    /// <summary>What the unit reduces to; null when the service sent none.</summary>
    public UnitTerm? Reduction { get; }

    /// <summary>Whether the magnitude is exact, an Integer or a Rational, rather than a Real.</summary>
    public bool IsExact => Magnitude is not Value.RealValue;

    /// <summary>The quantity as <c>5.4 [km/h]</c>; <c>[]</c> when its unit was never written down.</summary>
    /// <returns>The quantity's text.</returns>
    public override string ToString() => $"{Magnitude} [{Unit}]";
}
