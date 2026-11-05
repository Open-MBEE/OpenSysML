using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>One distinct outcome of the complete schedules of a checked behavior.</summary>
public sealed record CheckFinal
{
    /// <summary>Creates a final outcome, copying its values.</summary>
    /// <param name="outcome">The outcome, as the report prints it.</param>
    /// <param name="values">Each tracked feature's final value, as the report prints it, by feature.</param>
    /// <param name="witness">A schedule reaching it; null when the service reported none.</param>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    /// <exception cref="ArgumentException">A value is null or a feature repeats.</exception>
    public CheckFinal(string outcome, IEnumerable<KeyValuePair<string, string>> values, CheckWitness? witness)
    {
        ArgumentNullException.ThrowIfNull(outcome);
        Outcome = outcome;
        Values = new ValueDictionary<string>(values);
        Witness = witness;
    }

    /// <summary>The outcome, as the report prints it.</summary>
    public string Outcome { get; }

    /// <summary>Each tracked feature's final value, as the report prints it, by feature.</summary>
    public ValueDictionary<string> Values { get; }

    /// <summary>A schedule reaching the outcome; null when the service reported none.</summary>
    public CheckWitness? Witness { get; }
}
