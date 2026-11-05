using System;
using System.Collections.Generic;
using System.Linq;
using OpenMBEE.OpenSysML.Internal;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// What <see cref="Model.RunSweepAsync"/> answers: one row per run, in the order the runs were made, each an
/// ordinary run of the target with the swept parameters bound to the row's values. A run that failed is a row
/// with its error, never an exception.
/// </summary>
public sealed record Sweep
{
    /// <summary>Creates a sweep, copying its collections.</summary>
    /// <param name="rows">One row per run, in the order the runs were made.</param>
    /// <param name="parameters">The swept parameters, in the order their ranges were given.</param>
    /// <param name="sampled">Whether the rows were drawn rather than stepped through.</param>
    /// <param name="seed">The seed the rows were drawn from.</param>
    /// <param name="instances">Every object a row's verdict or value names, from every run.</param>
    /// <param name="diagnostics">What the service reported.</param>
    /// <param name="standing">The engine that ran the table, the strength of its evidence and its bounds.</param>
    /// <exception cref="ArgumentNullException">A collection or <paramref name="standing"/> is null.</exception>
    /// <exception cref="ArgumentException">A collection holds a null.</exception>
    public Sweep(
        IEnumerable<SweepRow> rows,
        IEnumerable<string> parameters,
        bool sampled,
        ulong seed,
        IEnumerable<Instance> instances,
        IEnumerable<Diagnostic> diagnostics,
        Standing standing)
    {
        ArgumentNullException.ThrowIfNull(standing);
        Rows = new ValueList<SweepRow>(rows);
        Parameters = new ValueList<string>(parameters);
        Sampled = sampled;
        Seed = seed;
        Instances = new ValueList<Instance>(instances);
        Diagnostics = new ValueList<Diagnostic>(diagnostics);
        Standing = standing;
    }

    /// <summary>
    /// One row per run, in the order the runs were made: lexicographically over the ranges as given, the first
    /// varying slowest, for a stepped table, and in draw order for a sampled one.
    /// </summary>
    public ValueList<SweepRow> Rows { get; }

    /// <summary>The swept parameters, in the order their ranges were given, which is the order each row's inputs are in.</summary>
    public ValueList<string> Parameters { get; }

    /// <summary>Whether the rows were drawn rather than stepped through.</summary>
    public bool Sampled { get; }

    /// <summary>The seed the rows were drawn from, echoed so a table can be reproduced; 0 for a stepped table.</summary>
    public ulong Seed { get; }

    /// <summary>Every object a row's verdict or value names, from every run, each row's numbered after the rows before it.</summary>
    public ValueList<Instance> Instances { get; }

    /// <summary>What the service reported.</summary>
    public ValueList<Diagnostic> Diagnostics { get; }

    /// <summary>The engine that ran the table, the strength of its evidence and the bounds it ran under; each row's verdicts carry the same.</summary>
    public Standing Standing { get; }

    /// <summary>The rows whose run failed.</summary>
    public ValueList<SweepRow> Failures => new(Rows.Where(row => row.Failed));

    /// <summary>Whether there is a row and every row holds.</summary>
    public bool Holds => !Rows.IsEmpty && Rows.All(row => row.Holds);

    /// <summary>The object of an id.</summary>
    /// <param name="instanceId">The id the service gave the object.</param>
    /// <returns>The object; null when it is not among <see cref="Instances"/>.</returns>
    public Instance? FindInstance(long instanceId) => InstanceSearch.Find(Instances, instanceId);

    /// <summary>The object a reference refers to.</summary>
    /// <param name="reference">A reference among this sweep's values.</param>
    /// <returns>The object; null when it is not among <see cref="Instances"/>.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="reference"/> is null.</exception>
    public Instance? Resolve(Value.InstanceRefValue reference)
    {
        ArgumentNullException.ThrowIfNull(reference);
        return FindInstance(reference.InstanceId);
    }
}
