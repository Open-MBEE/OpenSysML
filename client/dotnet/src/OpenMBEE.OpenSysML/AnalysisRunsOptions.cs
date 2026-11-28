using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// How <see cref="Model.RunAnalysisRunsAsync"/> runs a <c>Simulation::MonteCarlo</c> analysis case repeatedly, as
/// <c>sysml -runs</c> does: its subject and the values every run binds, the seed and policy its draws are taken under,
/// the clock step and the engine asked. A case reports the value it declares it observes, so it takes no features to
/// observe. Collections are copied when they are set. Random draws without a seed, an unknown draw policy and a negative
/// clock step are refused by the service with <see cref="StatusCode.InvalidArgument"/>.
/// </summary>
public sealed class AnalysisRunsOptions
{
    private readonly ValueList<Value> arguments;
    private readonly ValueDictionary<Value> namedArguments;

    /// <summary>The case's own subject and parameter defaults, the service's default draws and engine; no seed.</summary>
    public static AnalysisRunsOptions Default { get; } = new();

    /// <summary>
    /// The case's subject, as <see cref="AnalysisOptions.SubjectSymbolId"/> names one, built anew for every run; null
    /// leaves the case's own <c>subject</c> binding to supply it.
    /// </summary>
    public string? SubjectSymbolId { get; init; }

    /// <summary>Values for the case's <c>in</c> parameters, in declaration order, as every run binds them.</summary>
    /// <exception cref="ArgumentNullException">The list is null.</exception>
    /// <exception cref="ArgumentException">A value is null.</exception>
    public IReadOnlyList<Value> Arguments
    {
        get => arguments;
        init => arguments = new ValueList<Value>(value);
    }

    /// <summary>Values for the case's <c>in</c> parameters, by name, as every run binds them.</summary>
    /// <exception cref="ArgumentNullException">The dictionary is null.</exception>
    /// <exception cref="ArgumentException">A name or a value is null.</exception>
    public IReadOnlyDictionary<string, Value> NamedArguments
    {
        get => namedArguments;
        init => namedArguments = new ValueDictionary<Value>(value);
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
