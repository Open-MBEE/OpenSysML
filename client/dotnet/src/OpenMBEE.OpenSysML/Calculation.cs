using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// What <see cref="Model.CalcAsync"/> answers: the value an invocation returned, or, for a calc usage evaluated
/// from its own members, every output feature it computed, in declaration order.
/// </summary>
public sealed record Calculation
{
    /// <summary>Creates a calculation, copying its collections.</summary>
    /// <param name="result">The value an invocation returned; null when <paramref name="outputs"/> carries the answer.</param>
    /// <param name="outputs">The output features a calc usage computed, in declaration order.</param>
    /// <param name="diagnostics">What the service reported.</param>
    /// <param name="standing">The engine that computed the answer, the strength of its evidence and its bounds.</param>
    /// <exception cref="ArgumentNullException">A collection or <paramref name="standing"/> is null.</exception>
    /// <exception cref="ArgumentException">A collection holds a null.</exception>
    public Calculation(Value? result, IEnumerable<CalcOutput> outputs, IEnumerable<Diagnostic> diagnostics, Standing standing)
    {
        ArgumentNullException.ThrowIfNull(standing);
        Result = result;
        Outputs = new ValueList<CalcOutput>(outputs);
        Diagnostics = new ValueList<Diagnostic>(diagnostics);
        Standing = standing;
    }

    /// <summary>The value an invocation returned; null when <see cref="Outputs"/> carries the answer.</summary>
    public Value? Result { get; }

    /// <summary>The output features a calc usage evaluated from its own members computed, in declaration order; empty for an invocation.</summary>
    public ValueList<CalcOutput> Outputs { get; }

    /// <summary>What the service reported.</summary>
    public ValueList<Diagnostic> Diagnostics { get; }

    /// <summary>The engine that computed the answer, the strength of its evidence and the bounds it ran under.</summary>
    public Standing Standing { get; }

    /// <summary>The one value computed: <see cref="Result"/>, or the only output; null when there are several outputs.</summary>
    public Value? Value => Result ?? (Outputs.Count == 1 ? Outputs[0].Value : null);
}
