using System;
using System.Collections.Generic;
using System.Linq;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// What <see cref="Model.CompareResultsAsync"/> answers: one comparison per configuration, in the order the results
/// index them, then one per action named that no configuration has, and the engine that ran the runs.
/// </summary>
public sealed record Comparison
{
    /// <summary>Creates a comparison, copying its collections.</summary>
    /// <param name="source">The tool the results record as their source.</param>
    /// <param name="configurations">One comparison per configuration.</param>
    /// <param name="diagnostics">What the service reported.</param>
    /// <param name="standing">The engine that ran the runs, the strength of its evidence and its bounds.</param>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    /// <exception cref="ArgumentException">A collection holds a null.</exception>
    public Comparison(string source, IEnumerable<ConfigurationComparison> configurations, IEnumerable<Diagnostic> diagnostics, Standing standing)
    {
        ArgumentNullException.ThrowIfNull(source);
        ArgumentNullException.ThrowIfNull(standing);
        Source = source;
        Configurations = new ValueList<ConfigurationComparison>(configurations);
        Diagnostics = new ValueList<Diagnostic>(diagnostics);
        Standing = standing;
    }

    /// <summary>The tool the results record as their source, as the document they were migrated from.</summary>
    public string Source { get; }

    /// <summary>One comparison per configuration, in the order the results index them, then one per action named that no configuration has.</summary>
    public ValueList<ConfigurationComparison> Configurations { get; }

    /// <summary>What the service reported; empty when it reported nothing.</summary>
    public ValueList<Diagnostic> Diagnostics { get; }

    /// <summary>The engine that ran the configurations' runs, the strength of its evidence and the bounds it ran under; <see cref="Standing.None"/> when nothing was run.</summary>
    public Standing Standing { get; }

    /// <summary>Whether there is a configuration and every one holds.</summary>
    public bool Holds => !Configurations.IsEmpty && Configurations.All(configuration => configuration.Holds);

    /// <summary>The comparison as <c>sysml -compare-results</c> prints it.</summary>
    /// <returns>Every configuration's lines, one per line.</returns>
    public override string ToString() => string.Join('\n', Configurations.SelectMany(configuration => configuration.Lines));
}
