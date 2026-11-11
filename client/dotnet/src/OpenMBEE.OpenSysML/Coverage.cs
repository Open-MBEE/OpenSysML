using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// What every run of an <see cref="Exploration"/>, or every schedule of a check's <see cref="CheckSearch"/>, entered and
/// took, with how many did so: not only the witness of each outcome.
/// </summary>
public sealed record Coverage
{
    /// <summary>Creates a coverage, copying its lists.</summary>
    /// <param name="states">The states entered.</param>
    /// <param name="transitions">The transitions taken.</param>
    /// <exception cref="ArgumentNullException">A list is null.</exception>
    /// <exception cref="ArgumentException">A list holds a null.</exception>
    internal Coverage(IEnumerable<CoverageEntry> states, IEnumerable<CoverageEntry> transitions)
    {
        States = new ValueList<CoverageEntry>(states);
        Transitions = new ValueList<CoverageEntry>(transitions);
    }

    /// <summary>The states entered, in the order the service reports them.</summary>
    public ValueList<CoverageEntry> States { get; }

    /// <summary>The transitions taken, in the order the service reports them.</summary>
    public ValueList<CoverageEntry> Transitions { get; }
}
