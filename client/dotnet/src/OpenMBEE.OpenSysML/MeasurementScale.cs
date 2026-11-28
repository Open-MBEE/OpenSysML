using System;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// What a measurement scale states beyond its one axis: the unit it is measured in and, when the
/// model defines it against another reference, the point defining it.
/// </summary>
public sealed record MeasurementScale
{
    internal MeasurementScale(Value.MeasurementRefValue unit, QuantityValueMapping? mapping)
    {
        ArgumentNullException.ThrowIfNull(unit);
        Unit = unit;
        Mapping = mapping;
    }

    /// <summary>The unit a magnitude on the scale is expressed in.</summary>
    public Value.MeasurementRefValue Unit { get; }

    /// <summary>The scale's <c>quantityValueMapping</c>, or null when the model states none.</summary>
    public QuantityValueMapping? Mapping { get; }
}
