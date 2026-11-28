using System;

namespace OpenMBEE.OpenSysML;

/// <summary>One feature of a metadata usage <see cref="Edit.AddMetadata"/> writes, bound to a value expression.</summary>
public sealed record MetadataFeatureValue
{
    /// <summary>Creates a binding.</summary>
    /// <param name="feature">The feature bound, as a reference.</param>
    /// <param name="value">The value expression, written as notation.</param>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    public MetadataFeatureValue(string feature, string value)
    {
        ArgumentNullException.ThrowIfNull(feature);
        ArgumentNullException.ThrowIfNull(value);
        Feature = feature;
        Value = value;
    }

    /// <summary>The feature bound, as a reference.</summary>
    public string Feature { get; }

    /// <summary>The value expression, written as notation.</summary>
    public string Value { get; }
}
