using System;

namespace OpenMBEE.OpenSysML;

/// <summary>One final value a divergent feature can end with, and a schedule reaching it.</summary>
public sealed record CheckDivergentValue
{
    /// <summary>Creates a divergent value.</summary>
    /// <param name="value">The final value, as the report prints it.</param>
    /// <param name="witness">A schedule reaching it; null when the service reported none.</param>
    /// <exception cref="ArgumentNullException"><paramref name="value"/> is null.</exception>
    public CheckDivergentValue(string value, CheckWitness? witness)
    {
        ArgumentNullException.ThrowIfNull(value);
        Value = value;
        Witness = witness;
    }

    /// <summary>The final value, as the report prints it.</summary>
    public string Value { get; }

    /// <summary>A schedule reaching it, which a replay follows; null when the service reported none.</summary>
    public CheckWitness? Witness { get; }
}
