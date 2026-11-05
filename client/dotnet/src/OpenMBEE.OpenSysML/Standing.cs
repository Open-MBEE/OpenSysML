using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// How strongly an answer stands: the analysis engine that gave it, the strength of its evidence and
/// the bounds it ran under. A service without <see cref="Capabilities.Engines"/> reports none of it.
/// </summary>
public sealed record Standing
{
    /// <summary>The standing of an answer no engine was named for: no engine, no strength, no bounds.</summary>
    public static Standing None { get; } = new(string.Empty, string.Empty, []);

    /// <summary>Creates a standing, copying its bounds.</summary>
    /// <param name="engine">The engine that answered; empty when the service named none.</param>
    /// <param name="strength">The strength of its evidence; empty when the service reported none.</param>
    /// <param name="bounds">The bounds it ran under.</param>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    /// <exception cref="ArgumentException">A bound is null.</exception>
    public Standing(string engine, string strength, IEnumerable<Bound> bounds)
    {
        ArgumentNullException.ThrowIfNull(engine);
        ArgumentNullException.ThrowIfNull(strength);
        Engine = engine;
        Strength = strength;
        Bounds = new ValueList<Bound>(bounds);
    }

    /// <summary>The engine that answered, as the service names it (<c>check</c>, <c>smt</c>); empty when it named none.</summary>
    public string Engine { get; }

    /// <summary>
    /// The strength of the evidence: <c>observed</c> for one run, <c>witnessed</c> for a replayed
    /// counterexample, <c>bounded</c> for an exhaustive search within the bounds, <c>proved</c> for every
    /// case, <c>not covered</c> when no engine decided it; empty when the service reported none.
    /// </summary>
    public string Strength { get; }

    /// <summary>The bounds the engine ran under, each marked when it stopped the run.</summary>
    public ValueList<Bound> Bounds { get; }

    /// <summary>Whether the service said which engine answered.</summary>
    public bool IsReported => Engine.Length > 0;
}
