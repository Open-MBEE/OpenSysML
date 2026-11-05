using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>The statistics a <c>Simulation::MonteCarlo</c> case declares of an observable, the tool's pooled values beside the runs', one row per statistic.</summary>
public sealed record CaseStatistics
{
    /// <summary>Creates the statistics, copying their collections.</summary>
    /// <param name="analysis">The qualified name of the case.</param>
    /// <param name="rows">One row per statistic, in the order the case declares them.</param>
    /// <param name="notes">What was not compared, and why.</param>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    /// <exception cref="ArgumentException">A collection holds a null.</exception>
    public CaseStatistics(string analysis, IEnumerable<CaseStatistic> rows, IEnumerable<string> notes)
    {
        ArgumentNullException.ThrowIfNull(analysis);
        Analysis = analysis;
        Rows = new ValueList<CaseStatistic>(rows);
        Notes = new ValueList<string>(notes);
    }

    /// <summary>The qualified name of the case.</summary>
    public string Analysis { get; }

    /// <summary>One row per statistic, in the order the case declares them.</summary>
    public ValueList<CaseStatistic> Rows { get; }

    /// <summary>What was not compared, and why.</summary>
    public ValueList<string> Notes { get; }
}
