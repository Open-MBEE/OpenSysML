using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// How <see cref="Model.CompareResultsAsync"/> runs the configurations a tool's results index, as
/// <c>sysml -compare-results</c> does: which configurations, how many runs, the seed, draws and clock step they run
/// under, the observables compared and the engine asked. Collections are copied when they are set. A negative number of
/// runs, an unknown draw policy and a clock step no clock ticks by are refused by the service with
/// <see cref="StatusCode.InvalidArgument"/>.
/// </summary>
public sealed class CompareOptions
{
    private readonly ValueList<string> actions;
    private readonly ValueList<ObservablePair> observe;

    /// <summary>Every configuration, each run as its tool ran it, every stored observable compared; no seed.</summary>
    public static CompareOptions Default { get; } = new();

    /// <summary>
    /// The configurations compared, by qualified or simple name (<c>sysml -action</c>); empty compares every one the
    /// results index. A name no configuration has is answered as an unresolved configuration.
    /// </summary>
    /// <exception cref="ArgumentNullException">The list is null.</exception>
    /// <exception cref="ArgumentException">A name is null.</exception>
    public IReadOnlyList<string> Actions
    {
        get => actions;
        init => actions = new ValueList<string>(value);
    }

    /// <summary>
    /// The runs made of every configuration when positive (<c>sysml -runs</c>); 0, the default, makes each
    /// configuration's own <c>numberOfRuns</c>, or one.
    /// </summary>
    public long Runs { get; init; }

    /// <summary>
    /// The seed the runs derive theirs from (<c>sysml -seed</c>), so the same seed makes the same runs; null sends none,
    /// and a configuration whose runs draw at random is then answered unresolved unless <see cref="Draws"/> fixes them.
    /// </summary>
    public ulong? Seed { get; init; }

    /// <summary>
    /// The draw policy replacing every configuration's <c>durationSimulationMode</c> (<c>sysml -draws</c>):
    /// <c>random</c>, <c>min</c>, <c>max</c> or <c>average</c>; null keeps each configuration's own.
    /// </summary>
    public string? Draws { get; init; }

    /// <summary>
    /// The step, in seconds, replacing every configuration's clock step (<c>sysml -clock-step</c>); 0 is a continuous
    /// clock, null keeps each configuration's own.
    /// </summary>
    public double? ClockStep { get; init; }

    /// <summary>
    /// The stored observables compared, each with the feature of the run that answers it (<c>sysml -observe</c>), in
    /// order; empty reads every stored observable from the target's feature of its own name.
    /// </summary>
    /// <exception cref="ArgumentNullException">The list is null.</exception>
    /// <exception cref="ArgumentException">A pair is null.</exception>
    public IReadOnlyList<ObservablePair> Observe
    {
        get => observe;
        init => observe = new ValueList<ObservablePair>(value);
    }

    /// <summary>
    /// The engine the runs are put to, as <see cref="VerifyOptions.Engine"/> names one; null leaves the choice to the
    /// service. Naming one needs <see cref="Capabilities.Engines"/>, and <c>explore</c> also <see cref="Capabilities.ScheduleExplore"/>.
    /// </summary>
    public string? Engine { get; init; }
}
