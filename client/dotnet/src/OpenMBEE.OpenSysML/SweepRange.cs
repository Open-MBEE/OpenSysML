using System;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// One input parameter a sweep varies, between two endpoints and, for a stepped sweep, by a step. A sampled
/// sweep states no step; a range between whole numbers steps by one where it states none, and one with a
/// fractional endpoint must state one. The service refuses a range it can follow no values from, in its answer.
/// </summary>
public sealed record SweepRange
{
    /// <summary>Creates a range.</summary>
    /// <param name="parameter">The name of the input parameter the range binds.</param>
    /// <param name="start">The endpoint the range's runs start at.</param>
    /// <param name="end">The endpoint they run to, inclusive where the step lands on it.</param>
    /// <param name="step">The step between runs; null for none.</param>
    /// <exception cref="ArgumentNullException"><paramref name="parameter"/>, <paramref name="start"/> or <paramref name="end"/> is null.</exception>
    public SweepRange(string parameter, Value start, Value end, Value? step = null)
    {
        ArgumentNullException.ThrowIfNull(parameter);
        ArgumentNullException.ThrowIfNull(start);
        ArgumentNullException.ThrowIfNull(end);
        Parameter = parameter;
        Start = start;
        End = end;
        Step = step;
    }

    /// <summary>The name of the input parameter the range binds, which the target must declare and the call's own arguments must not bind.</summary>
    public string Parameter { get; }

    /// <summary>The endpoint the range's runs start at.</summary>
    public Value Start { get; }

    /// <summary>The endpoint the range's runs go to, which is inclusive where the step lands on it.</summary>
    public Value End { get; }

    /// <summary>The step between runs; null for none.</summary>
    public Value? Step { get; }
}
