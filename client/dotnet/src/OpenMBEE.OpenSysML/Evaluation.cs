using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>What <see cref="Model.EvaluateAsync"/> answered: the value, and what the service reported with it.</summary>
public sealed record Evaluation
{
    /// <summary>Creates an evaluation.</summary>
    /// <param name="result">The value the expression evaluated to.</param>
    /// <param name="diagnostics">What the service reported with it.</param>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    /// <exception cref="ArgumentException">A diagnostic is null.</exception>
    public Evaluation(Value result, IEnumerable<Diagnostic> diagnostics)
    {
        ArgumentNullException.ThrowIfNull(result);
        Result = result;
        Diagnostics = new ValueList<Diagnostic>(diagnostics);
    }

    /// <summary>The value the expression evaluated to.</summary>
    public Value Result { get; }

    /// <summary>What the service reported with it.</summary>
    public ValueList<Diagnostic> Diagnostics { get; }
}
