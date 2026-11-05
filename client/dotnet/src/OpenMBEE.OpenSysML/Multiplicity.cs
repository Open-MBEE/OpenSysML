using System;

namespace OpenMBEE.OpenSysML;

/// <summary>A declared multiplicity range. A bound the service cannot evaluate statically is empty.</summary>
public sealed record Multiplicity
{
    /// <summary>Creates a multiplicity.</summary>
    /// <param name="lower">The lower bound (<c>0</c>, <c>1</c>, … or <c>*</c>); empty when not static.</param>
    /// <param name="upper">The upper bound (<c>1</c>, <c>*</c>, …); empty when not static.</param>
    /// <exception cref="ArgumentNullException">A bound is null.</exception>
    public Multiplicity(string lower, string upper)
    {
        ArgumentNullException.ThrowIfNull(lower);
        ArgumentNullException.ThrowIfNull(upper);
        Lower = lower;
        Upper = upper;
    }

    /// <summary>The lower bound (<c>0</c>, <c>1</c>, … or <c>*</c>); empty when not static.</summary>
    public string Lower { get; }

    /// <summary>The upper bound (<c>1</c>, <c>*</c>, …); empty when not static.</summary>
    public string Upper { get; }

    /// <summary>The range, as <c>[0..*]</c>; an unknown bound is <c>?</c>.</summary>
    /// <returns>The range's text.</returns>
    public override string ToString() => $"[{Bound(Lower)}..{Bound(Upper)}]";

    private static string Bound(string bound) => bound.Length == 0 ? "?" : bound;
}
