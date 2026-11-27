using System;

namespace OpenMBEE.OpenSysML;

/// <summary>One point on a scale and the same point on the reference the scale is defined against.</summary>
public sealed record QuantityValueMapping
{
    internal QuantityValueMapping(Quantity mapped, Quantity reference)
    {
        ArgumentNullException.ThrowIfNull(mapped);
        ArgumentNullException.ThrowIfNull(reference);
        Mapped = mapped;
        Reference = reference;
    }

    /// <summary>The point on the scale, in the scale's unit.</summary>
    public Quantity Mapped { get; }

    /// <summary>The same point on the reference.</summary>
    public Quantity Reference { get; }
}
