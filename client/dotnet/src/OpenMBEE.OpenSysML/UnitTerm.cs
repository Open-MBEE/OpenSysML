using System;
using System.Collections.Generic;
using System.Linq;
using OpenMBEE.OpenSysML.Internal;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// A unit reduced to a scale over base units: <c>km/h</c> reduces to <c>1000/3600</c> over
/// <c>SI::m</c> and <c>SI::s^-1</c>. The scale is kept as a ratio so an exact conversion stays exact.
/// It is what decides whether two quantities are commensurable and how one converts to the other.
/// </summary>
public sealed record UnitTerm
{
    /// <summary>Creates a unit term.</summary>
    /// <param name="scaleNumerator">The scale's numerator.</param>
    /// <param name="scaleDenominator">The scale's denominator.</param>
    /// <param name="factors">The base units, copied.</param>
    /// <exception cref="ArgumentNullException"><paramref name="factors"/> is null.</exception>
    /// <exception cref="ArgumentException">A factor is null.</exception>
    public UnitTerm(double scaleNumerator, double scaleDenominator, IEnumerable<UnitFactor> factors)
    {
        ScaleNumerator = scaleNumerator;
        ScaleDenominator = scaleDenominator;
        Factors = new ValueList<UnitFactor>(factors);
    }

    /// <summary>The term of a quantity of dimension one: scale <c>1/1</c> over no base unit.</summary>
    public static UnitTerm One { get; } = new(1, 1, []);

    /// <summary>The scale's numerator.</summary>
    public double ScaleNumerator { get; }

    /// <summary>The scale's denominator.</summary>
    public double ScaleDenominator { get; }

    /// <summary>The base units, in the order the service gave them.</summary>
    public ValueList<UnitFactor> Factors { get; }

    /// <summary>The term as <c>1000/3600 SI::m^1 SI::s^-1</c>.</summary>
    /// <returns>The term's text.</returns>
    public override string ToString() =>
        string.Join(' ', Factors.Select(factor => factor.ToString()).Prepend($"{ValueText.Plain(ScaleNumerator)}/{ValueText.Plain(ScaleDenominator)}"));
}
