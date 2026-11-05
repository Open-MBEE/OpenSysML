using System;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// One statistic a <c>Simulation::MonteCarlo</c> case declares of an observable: which, where the case declares it, the
/// tool's value, the runs' value and their relative difference, each as the comparison prints it.
/// </summary>
public sealed record CaseStatistic
{
    /// <summary>Creates a statistic's row.</summary>
    /// <param name="statistic">Which statistic, as the tool names it (<c>N</c>, <c>Mean</c>, <c>Deviation</c>, <c>OutOfSpec</c>).</param>
    /// <param name="of">Where the case declares it, as <c>return Mean</c>.</param>
    /// <param name="tool">The tool's value.</param>
    /// <param name="run">The runs' value.</param>
    /// <param name="difference">Their relative difference; empty where it is not compared.</param>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    public CaseStatistic(string statistic, string of, string tool, string run, string difference)
    {
        ArgumentNullException.ThrowIfNull(statistic);
        ArgumentNullException.ThrowIfNull(of);
        ArgumentNullException.ThrowIfNull(tool);
        ArgumentNullException.ThrowIfNull(run);
        ArgumentNullException.ThrowIfNull(difference);
        Statistic = statistic;
        Of = of;
        Tool = tool;
        Run = run;
        Difference = difference;
    }

    /// <summary>Which statistic, as the tool names it (<c>N</c>, <c>Mean</c>, <c>Deviation</c>, <c>OutOfSpec</c>).</summary>
    public string Statistic { get; }

    /// <summary>Where the case declares it, as <c>return Mean</c>.</summary>
    public string Of { get; }

    /// <summary>The tool's value, as the comparison prints it.</summary>
    public string Tool { get; }

    /// <summary>The runs' value, as the comparison prints it.</summary>
    public string Run { get; }

    /// <summary>The relative difference of the runs' value from the tool's, as a signed percentage; empty where it is not compared, as for a run count.</summary>
    public string Difference { get; }
}
