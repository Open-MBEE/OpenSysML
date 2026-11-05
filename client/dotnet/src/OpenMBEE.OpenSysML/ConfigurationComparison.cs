using System;
using System.Collections.Generic;
using OpenMBEE.OpenSysML.Internal;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// One configuration's runs beside the tool's results: how it was run, one row per run as a sweep tables them, each
/// observable compared, and the comparison as <c>sysml -compare-results</c> prints it. A configuration that could not be
/// run, or whose runs failed, is a comparison with that <see cref="Status"/>, never an exception.
/// </summary>
public sealed record ConfigurationComparison
{
    /// <summary>Creates a configuration's comparison, copying its collections.</summary>
    /// <param name="id">The configuration's identifier in the results.</param>
    /// <param name="name">The qualified name of the configuration.</param>
    /// <param name="behavior">The migrated behavior it performs; empty when it performs none.</param>
    /// <param name="target">The feature of the configuration the observables are read from.</param>
    /// <param name="resultLocation">Where the tool kept its results; empty for none.</param>
    /// <param name="status"><c>holds</c>, <c>undecided</c>, <c>fails</c> or <c>unresolved</c>.</param>
    /// <param name="unresolved">Why the configuration could not be run; empty when it was.</param>
    /// <param name="storedRuns">How many runs the tool stored.</param>
    /// <param name="storedSnapshots">Over how many snapshots it stored them.</param>
    /// <param name="runs">How many runs OpenSysML made.</param>
    /// <param name="completed">How many of them completed.</param>
    /// <param name="draws">The draw policy the runs took.</param>
    /// <param name="seed">The seed the runs derived theirs from; null when they drew nothing.</param>
    /// <param name="clockStep">The clock step the runs ticked by, in seconds; 0 for a continuous clock.</param>
    /// <param name="rows">One row per run, in run order.</param>
    /// <param name="instances">Every object a row's value names.</param>
    /// <param name="observables">Each observable compared.</param>
    /// <param name="errors">The errors failed runs ended in, deduplicated.</param>
    /// <param name="notes">The notes on the configuration and its runs.</param>
    /// <param name="lines">The comparison as <c>sysml -compare-results</c> prints it.</param>
    /// <exception cref="ArgumentNullException">A string or a collection is null.</exception>
    /// <exception cref="ArgumentException">A collection holds a null.</exception>
    public ConfigurationComparison(
        string id,
        string name,
        string behavior,
        string target,
        string resultLocation,
        string status,
        string unresolved,
        long storedRuns,
        long storedSnapshots,
        long runs,
        long completed,
        string draws,
        ulong? seed,
        double clockStep,
        IEnumerable<SweepRow> rows,
        IEnumerable<Instance> instances,
        IEnumerable<ObservableComparison> observables,
        IEnumerable<string> errors,
        IEnumerable<string> notes,
        IEnumerable<string> lines)
    {
        ArgumentNullException.ThrowIfNull(id);
        ArgumentNullException.ThrowIfNull(name);
        ArgumentNullException.ThrowIfNull(behavior);
        ArgumentNullException.ThrowIfNull(target);
        ArgumentNullException.ThrowIfNull(resultLocation);
        ArgumentNullException.ThrowIfNull(status);
        ArgumentNullException.ThrowIfNull(unresolved);
        ArgumentNullException.ThrowIfNull(draws);
        Id = id;
        Name = name;
        Behavior = behavior;
        Target = target;
        ResultLocation = resultLocation;
        Status = status;
        Unresolved = unresolved;
        StoredRuns = storedRuns;
        StoredSnapshots = storedSnapshots;
        Runs = runs;
        Completed = completed;
        Draws = draws;
        Seed = seed;
        ClockStep = clockStep;
        Rows = new ValueList<SweepRow>(rows);
        Instances = new ValueList<Instance>(instances);
        Observables = new ValueList<ObservableComparison>(observables);
        Errors = new ValueList<string>(errors);
        Notes = new ValueList<string>(notes);
        Lines = new ValueList<string>(lines);
    }

    /// <summary>The configuration's identifier in the results, its <c>xmi:id</c>.</summary>
    public string Id { get; }

    /// <summary>The qualified name of the configuration, as the results index it.</summary>
    public string Name { get; }

    /// <summary>The migrated behavior the configuration performs; empty when it performs none.</summary>
    public string Behavior { get; }

    /// <summary>The feature of the configuration the observables are read from.</summary>
    public string Target { get; }

    /// <summary>Where the tool kept its results, as the qualified name of its result package; empty for none.</summary>
    public string ResultLocation { get; }

    /// <summary>
    /// <c>holds</c> when the runs completed and every observable compared; <c>undecided</c> when some observable did not;
    /// <c>fails</c> when a run failed; <c>unresolved</c> when the configuration could not be run, saying why in
    /// <see cref="Unresolved"/>. A status this release does not name, from a newer service, is kept as it was spelled.
    /// </summary>
    public string Status { get; }

    /// <summary>Why the configuration could not be run, when <see cref="Status"/> is <c>unresolved</c>; empty otherwise.</summary>
    public string Unresolved { get; }

    /// <summary>How many runs the tool stored.</summary>
    public long StoredRuns { get; }

    /// <summary>Over how many snapshots the tool stored its runs: a summary snapshot holds several.</summary>
    public long StoredSnapshots { get; }

    /// <summary>How many runs OpenSysML made.</summary>
    public long Runs { get; }

    /// <summary>How many of <see cref="Runs"/> completed.</summary>
    public long Completed { get; }

    /// <summary>The draw policy the runs took: <c>random</c>, <c>min</c>, <c>max</c> or <c>average</c>.</summary>
    public string Draws { get; }

    /// <summary>The seed the runs derived theirs from; null when the runs drew nothing at random.</summary>
    public ulong? Seed { get; }

    /// <summary>The step, in seconds, the runs' clock ticked by; 0 for a continuous clock.</summary>
    public double ClockStep { get; }

    /// <summary>One row per run, in run order, as <see cref="Model.RunSweepAsync"/> tables them: what each bound, produced and failed with.</summary>
    public ValueList<SweepRow> Rows { get; }

    /// <summary>Every object a row's value names, from every run.</summary>
    public ValueList<Instance> Instances { get; }

    /// <summary>Each observable compared, in the order the comparison lists them.</summary>
    public ValueList<ObservableComparison> Observables { get; }

    /// <summary>The errors failed runs ended in, deduplicated and sorted.</summary>
    public ValueList<string> Errors { get; }

    /// <summary>The notes on the configuration and its runs.</summary>
    public ValueList<string> Notes { get; }

    /// <summary>The comparison as <c>sysml -compare-results</c> prints it, one line each.</summary>
    public ValueList<string> Lines { get; }

    /// <summary>Whether <see cref="Status"/> is <c>holds</c>: every run completed and every observable compared.</summary>
    public bool Holds => Status == "holds";

    /// <summary>The object of an id.</summary>
    /// <param name="instanceId">The id the service gave the object.</param>
    /// <returns>The object; null when it is not among <see cref="Instances"/>.</returns>
    public Instance? FindInstance(long instanceId) => InstanceSearch.Find(Instances, instanceId);

    /// <summary>The object a reference refers to.</summary>
    /// <param name="reference">A reference among this configuration's values.</param>
    /// <returns>The object; null when it is not among <see cref="Instances"/>.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="reference"/> is null.</exception>
    public Instance? Resolve(Value.InstanceRefValue reference)
    {
        ArgumentNullException.ThrowIfNull(reference);
        return FindInstance(reference.InstanceId);
    }

    /// <summary>The comparison as <c>sysml -compare-results</c> prints it.</summary>
    /// <returns><see cref="Lines"/>, one per line.</returns>
    public override string ToString() => string.Join('\n', Lines);
}
