using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// How <see cref="Model.RunActionRunsAsync"/> runs an action repeatedly, as <c>sysml -runs</c> does: the object
/// performing it, the features each run reports, the seed and policy its draws are taken under, the clock step and the
/// engine asked. Collections are copied when they are set. Random draws without a seed, an unknown draw policy, a
/// negative clock step and a feature named twice are refused by the service with <see cref="StatusCode.InvalidArgument"/>.
/// </summary>
public sealed class ActionRunsOptions
{
    private readonly ValueList<string> observe;

    /// <summary>Every feature observed, outside any object, by the service's default draws and engine; no seed.</summary>
    public static ActionRunsOptions Default { get; } = new();

    /// <summary>
    /// The object every run is performed by, as <see cref="RunOptions.PerformerSymbolId"/> names one, made anew for each
    /// run; null runs outside any object. Needs <see cref="Capabilities.Performer"/>.
    /// </summary>
    public string? PerformerSymbolId { get; init; }

    /// <summary>
    /// The features of the action each run reports (<c>sysml -observe</c>), <c>clock</c> for the time it completed at, in
    /// column order; empty reports every feature the action holds and the clock.
    /// </summary>
    /// <exception cref="ArgumentNullException">The list is null.</exception>
    /// <exception cref="ArgumentException">A name is null.</exception>
    public IReadOnlyList<string> Observe
    {
        get => observe;
        init => observe = new ValueList<string>(value);
    }

    /// <summary>
    /// The seed the runs draw their modeled randomness from, one seed derived per run: the same seed makes the same runs
    /// on every platform. Required unless <see cref="Draws"/> fixes the draws; null sends none.
    /// </summary>
    public ulong? Seed { get; init; }

    /// <summary>
    /// How the runs resolve the draws of RandomFunctions (<c>sysml -draws</c>): <c>random</c>, the service's default,
    /// <c>min</c>, <c>max</c> or <c>average</c>; null for the default.
    /// </summary>
    public string? Draws { get; init; }

    /// <summary>The step, in seconds, each run's clock ticks by, as <see cref="RunOptions.ClockStep"/>; 0, the default, is a continuous clock.</summary>
    public double ClockStep { get; init; }

    /// <summary>
    /// The engine the runs are put to, as <see cref="VerifyOptions.Engine"/> names one; null leaves the choice to the
    /// service. Naming one needs <see cref="Capabilities.Engines"/>, and <c>explore</c> also <see cref="Capabilities.ScheduleExplore"/>.
    /// </summary>
    public string? Engine { get; init; }
}
