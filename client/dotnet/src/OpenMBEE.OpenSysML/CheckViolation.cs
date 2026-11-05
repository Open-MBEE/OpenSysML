using System;

namespace OpenMBEE.OpenSysML;

/// <summary>One schedule of a checked behavior that reached a violated property, a deadlock or a failure.</summary>
public sealed record CheckViolation
{
    /// <summary>Creates a violation.</summary>
    /// <param name="kind"><c>property</c>, <c>deadlock</c> or <c>failure</c>.</param>
    /// <param name="property">The property violated; empty for a deadlock or a failure.</param>
    /// <param name="error">The deadlock or the failure as the executor reported it; empty for a property.</param>
    /// <param name="depth">How many moves the witness makes.</param>
    /// <param name="probability">The probability of the schedules reaching the violation.</param>
    /// <param name="witness">The schedule reaching it; null when the service reported none.</param>
    /// <param name="text">The violation as the command reports it.</param>
    /// <exception cref="ArgumentNullException">A string is null.</exception>
    public CheckViolation(string kind, string property, string error, int depth, double probability, CheckWitness? witness, string text)
    {
        ArgumentNullException.ThrowIfNull(kind);
        ArgumentNullException.ThrowIfNull(property);
        ArgumentNullException.ThrowIfNull(error);
        ArgumentNullException.ThrowIfNull(text);
        Kind = kind;
        Property = property;
        Error = error;
        Depth = depth;
        Probability = probability;
        Witness = witness;
        Text = text;
    }

    /// <summary>What was violated: <c>property</c>, <c>deadlock</c> or <c>failure</c>.</summary>
    public string Kind { get; }

    /// <summary>The property violated; empty for a deadlock or a failure.</summary>
    public string Property { get; }

    /// <summary>The deadlock or the failure as the executor reported it; empty for a property.</summary>
    public string Error { get; }

    /// <summary>How many moves the witness makes.</summary>
    public int Depth { get; }

    /// <summary>The probability of the schedules reaching this violation; a lower bound when <see cref="CheckSearch.MassBounded"/> holds.</summary>
    public double Probability { get; }

    /// <summary>The schedule reaching the violation, which a replay follows; null when the service reported none.</summary>
    public CheckWitness? Witness { get; }

    /// <summary>The violation as the command reports it.</summary>
    public string Text { get; }
}
