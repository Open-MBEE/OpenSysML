using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// One observable of a configuration: the tool's distribution beside the runs' with the relative difference of each
/// statistic, or the notes saying why they are not compared. Absent distributions, differences and case statistics are
/// null, never zeros.
/// </summary>
public sealed record ObservableComparison
{
    /// <summary>Creates an observable's comparison, copying its notes.</summary>
    /// <param name="stored">The observable the tool stored.</param>
    /// <param name="feature">The feature of the runs that answers it.</param>
    /// <param name="unit">The unit the runs' numbers came in; empty for none.</param>
    /// <param name="tool">The tool's distribution; null when it stored no number.</param>
    /// <param name="pooled">Whether the tool's distribution was pooled from summaries.</param>
    /// <param name="unstored">Whether the tool stored no result at all to compare against.</param>
    /// <param name="run">The completed runs' distribution; null when they are not compared.</param>
    /// <param name="produced">How many runs produced a number for the feature.</param>
    /// <param name="compared">Whether the runs were made and are reported.</param>
    /// <param name="difference">The relative difference of each statistic; null without both distributions.</param>
    /// <param name="declared">The case's statistics of the observable compared; null when no case analyses it.</param>
    /// <param name="notes">What the comparison remarks on.</param>
    /// <exception cref="ArgumentNullException">A string or <paramref name="notes"/> is null.</exception>
    /// <exception cref="ArgumentException">A note is null.</exception>
    public ObservableComparison(
        string stored,
        string feature,
        string unit,
        ComparedDistribution? tool,
        bool pooled,
        bool unstored,
        ComparedDistribution? run,
        long produced,
        bool compared,
        StatisticDifference? difference,
        CaseStatistics? declared,
        IEnumerable<string> notes)
    {
        ArgumentNullException.ThrowIfNull(stored);
        ArgumentNullException.ThrowIfNull(feature);
        ArgumentNullException.ThrowIfNull(unit);
        Stored = stored;
        Feature = feature;
        Unit = unit;
        Tool = tool;
        Pooled = pooled;
        Unstored = unstored;
        Run = run;
        Produced = produced;
        Compared = compared;
        Difference = difference;
        Declared = declared;
        Notes = new ValueList<string>(notes);
    }

    /// <summary>The observable the tool stored.</summary>
    public string Stored { get; }

    /// <summary>The feature of the runs that answers it, as <c>target.total</c>.</summary>
    public string Feature { get; }

    /// <summary>The unit the runs' numbers came in; empty for none.</summary>
    public string Unit { get; }

    /// <summary>
    /// The tool's distribution; null when it stored no number for the observable. A <see cref="Pooled"/> one knows its
    /// count and mean alone.
    /// </summary>
    public ComparedDistribution? Tool { get; }

    /// <summary>Whether <see cref="Tool"/> was pooled from the tool's summaries, so only its count and mean are known.</summary>
    public bool Pooled { get; }

    /// <summary>Whether the tool stored no result at all to compare against.</summary>
    public bool Unstored { get; }

    /// <summary>The distribution of the completed runs; null when they are not compared.</summary>
    public ComparedDistribution? Run { get; }

    /// <summary>How many runs produced a number for the feature.</summary>
    public long Produced { get; }

    /// <summary>Whether the runs were made and are reported; false when the tool stored no observable of the name or no number for it.</summary>
    public bool Compared { get; }

    /// <summary>The relative difference of each statistic; null without both distributions. A pooled tool distribution compares the mean alone.</summary>
    public StatisticDifference? Difference { get; }

    /// <summary>The per-statistic comparison of the <c>Simulation::MonteCarlo</c> case the observable is analysed by; null when none is.</summary>
    public CaseStatistics? Declared { get; }

    /// <summary>What the comparison remarks on, as it prints each note.</summary>
    public ValueList<string> Notes { get; }
}
