using System;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// One free variable's value in the assignment a solver found and the evaluator replayed: what witnesses
/// a <c>violated</c> answer to a holds question or a <c>satisfiable</c> one.
/// </summary>
public sealed record WitnessAssignment
{
    /// <summary>Creates an assignment.</summary>
    /// <param name="feature">The variable's feature, a qualified name with chain steps appended with <c>.</c>.</param>
    /// <param name="value">The value the evaluator replayed; null when the service sent none.</param>
    /// <param name="unit">The base units a magnitude is expressed in; empty when dimensionless.</param>
    /// <param name="exact">The solver's exact value as text: a rational for a real, a literal's name for an enumeration.</param>
    /// <exception cref="ArgumentNullException"><paramref name="feature"/>, <paramref name="unit"/> or <paramref name="exact"/> is null.</exception>
    public WitnessAssignment(string feature, Value? value, string unit, string exact)
    {
        ArgumentNullException.ThrowIfNull(feature);
        ArgumentNullException.ThrowIfNull(unit);
        ArgumentNullException.ThrowIfNull(exact);
        Feature = feature;
        Value = value;
        Unit = unit;
        Exact = exact;
    }

    /// <summary>The variable's feature, a qualified name with chain steps appended with <c>.</c>.</summary>
    public string Feature { get; }

    /// <summary>
    /// The value the evaluator replayed: a boolean, an integer, a real as the <see cref="double"/> of the exact
    /// rational, an enumeration literal; null when the service sent none.
    /// </summary>
    public Value? Value { get; }

    /// <summary>The base units a magnitude is expressed in; empty when dimensionless.</summary>
    public string Unit { get; }

    /// <summary>The solver's exact value as text: a rational for a real, a literal's name for an enumeration.</summary>
    public string Exact { get; }
}
