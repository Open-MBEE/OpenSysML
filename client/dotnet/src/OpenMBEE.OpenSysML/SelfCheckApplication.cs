using System;

namespace OpenMBEE.OpenSysML;

/// <summary>How many elements a self-check applied one constraint to.</summary>
public sealed record SelfCheckApplication
{
    /// <summary>Creates an application count.</summary>
    /// <param name="constraint">The qualified name of the constraint.</param>
    /// <param name="count">The elements it was applied to.</param>
    /// <exception cref="ArgumentNullException"><paramref name="constraint"/> is null.</exception>
    public SelfCheckApplication(string constraint, long count)
    {
        ArgumentNullException.ThrowIfNull(constraint);
        Constraint = constraint;
        Count = count;
    }

    /// <summary>The qualified name of the constraint.</summary>
    public string Constraint { get; }

    /// <summary>The elements it was applied to.</summary>
    public long Count { get; }
}
