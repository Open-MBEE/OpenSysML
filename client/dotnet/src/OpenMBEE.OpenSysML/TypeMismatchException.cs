using System;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// A feature holds a value of another type, or another number of values, than the generated typed class
/// reading it declares; the value is reported rather than returned as the declared type.
/// </summary>
public class TypeMismatchException : OpenSysMLException
{
    /// <summary>Creates the exception.</summary>
    /// <param name="featureName">The feature's name.</param>
    /// <param name="expected">What the generated class declares, as <c>double</c> or <c>a single value</c>.</param>
    /// <param name="actual">The value the feature holds; null when it holds none.</param>
    /// <exception cref="ArgumentNullException"><paramref name="featureName"/> or <paramref name="expected"/> is null.</exception>
    public TypeMismatchException(string featureName, string expected, Value? actual)
        : base($"feature value '{featureName}': expected {expected}, got {(actual is null ? "no value" : actual.ToString())}")
    {
        ArgumentNullException.ThrowIfNull(featureName);
        ArgumentNullException.ThrowIfNull(expected);
        FeatureName = featureName;
        Expected = expected;
        Actual = actual;
    }

    /// <summary>The feature's name.</summary>
    public string FeatureName { get; }

    /// <summary>What the generated class declares, as <c>double</c> or <c>a single value</c>.</summary>
    public string Expected { get; }

    /// <summary>The value the feature holds, its values as one sequence when it holds several; null when it holds none.</summary>
    public Value? Actual { get; }
}
