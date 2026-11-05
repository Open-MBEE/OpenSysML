using System;

namespace OpenMBEE.OpenSysML;

/// <summary>An attribute of a <see cref="Symbol"/>, with the value the service resolved for it.</summary>
public sealed record AttributeInfo
{
    /// <summary>Creates an attribute.</summary>
    /// <param name="name">The attribute's name.</param>
    /// <param name="type">Its type as written; empty when none is declared.</param>
    /// <param name="value">The value resolved for it; null when the service resolved none.</param>
    /// <param name="unit">The unit as written; empty when the value carries none.</param>
    /// <exception cref="ArgumentNullException"><paramref name="name"/>, <paramref name="type"/> or <paramref name="unit"/> is null.</exception>
    public AttributeInfo(string name, string type, Value? value, string unit)
    {
        ArgumentNullException.ThrowIfNull(name);
        ArgumentNullException.ThrowIfNull(type);
        ArgumentNullException.ThrowIfNull(unit);
        Name = name;
        Type = type;
        Value = value;
        Unit = unit;
    }

    /// <summary>The attribute's name.</summary>
    public string Name { get; }

    /// <summary>Its type as written; empty when none is declared.</summary>
    public string Type { get; }

    /// <summary>The value resolved for it; null when the service resolved none.</summary>
    public Value? Value { get; }

    /// <summary>The unit as written; empty when the value carries none.</summary>
    public string Unit { get; }
}
