using System;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// A feature of an <see cref="Instance"/> holds no value to read: the service reported that evaluating it
/// failed, or never materialized it. Raised by the accessors of generated typed classes.
/// </summary>
public class FeatureValueException : OpenSysMLException
{
    /// <summary>Creates the exception.</summary>
    /// <param name="featureName">The feature's name.</param>
    /// <param name="reason">Why it holds no value, as the service reported it.</param>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    public FeatureValueException(string featureName, string reason)
        : base($"feature value '{featureName}': {reason}")
    {
        ArgumentNullException.ThrowIfNull(featureName);
        ArgumentNullException.ThrowIfNull(reason);
        FeatureName = featureName;
        Reason = reason;
    }

    /// <summary>The feature's name.</summary>
    public string FeatureName { get; }

    /// <summary>Why it holds no value, as the service reported it.</summary>
    public string Reason { get; }
}
