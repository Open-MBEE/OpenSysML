using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>A feature whose final value the schedule of a checked behavior decides.</summary>
public sealed record CheckDivergence
{
    /// <summary>Creates a divergence, copying its values.</summary>
    /// <param name="feature">The feature, as the check named it.</param>
    /// <param name="values">Every final value some complete schedule leaves it with, in canonical order.</param>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    /// <exception cref="ArgumentException">A value is null.</exception>
    public CheckDivergence(string feature, IEnumerable<CheckDivergentValue> values)
    {
        ArgumentNullException.ThrowIfNull(feature);
        Feature = feature;
        Values = new ValueList<CheckDivergentValue>(values);
    }

    /// <summary>The feature, as <see cref="CheckOptions.Diverge"/> named it.</summary>
    public string Feature { get; }

    /// <summary>Every final value some complete schedule leaves the feature with, in canonical order, each with a schedule reaching it.</summary>
    public ValueList<CheckDivergentValue> Values { get; }
}
