using System;
using OpenMBEE.OpenSysML.Internal;

namespace OpenMBEE.OpenSysML;

/// <summary>One base unit of a <see cref="UnitTerm"/>, raised to an exponent.</summary>
public sealed record UnitFactor
{
    /// <summary>Creates a unit factor.</summary>
    /// <param name="unitId">The base unit's qualified name, as <c>SI::m</c>.</param>
    /// <param name="exponent">The exponent it is raised to.</param>
    /// <exception cref="ArgumentNullException"><paramref name="unitId"/> is null.</exception>
    public UnitFactor(string unitId, double exponent)
    {
        ArgumentNullException.ThrowIfNull(unitId);
        UnitId = unitId;
        Exponent = exponent;
    }

    /// <summary>The base unit's qualified name, as <c>SI::m</c>.</summary>
    public string UnitId { get; }

    /// <summary>The exponent the base unit is raised to.</summary>
    public double Exponent { get; }

    /// <summary>The factor as <c>SI::s^-1</c>.</summary>
    /// <returns>The factor's text.</returns>
    public override string ToString() => $"{UnitId}^{ValueText.Plain(Exponent)}";
}
