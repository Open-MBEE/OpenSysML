using System;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// A generated typed class was asked to view an <see cref="Instance"/> whose type another generated class
/// describes, one that is not the requested definition or a specialization of it.
/// </summary>
public class InstanceTypeException : OpenSysMLException
{
    /// <summary>Creates the exception.</summary>
    /// <param name="expected">The qualified name of the definition the requested view was generated from.</param>
    /// <param name="actual">The qualified name the instance reports as its type.</param>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    public InstanceTypeException(string expected, string actual)
        : base($"instance of '{actual}' is not a '{expected}'; call the generated class's Unchecked(instance) to view it without this check")
    {
        ArgumentNullException.ThrowIfNull(expected);
        ArgumentNullException.ThrowIfNull(actual);
        Expected = expected;
        Actual = actual;
    }

    /// <summary>The qualified name of the definition the requested view was generated from.</summary>
    public string Expected { get; }

    /// <summary>The qualified name the instance reports as its type.</summary>
    public string Actual { get; }
}
