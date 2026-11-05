using System;
using System.Collections.Generic;
using System.Linq;
using OpenMBEE.OpenSysML.Internal;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// What <see cref="Model.RunActionRunsAsync"/> and <see cref="Model.RunAnalysisRunsAsync"/> answer: one row per run, in
/// run order, each observable's distribution over the completed runs, and, for a Monte Carlo case, its conclusion over
/// the sample. A run that failed is a row with its error, never an exception.
/// </summary>
public sealed record Runs
{
    /// <summary>Creates the runs' table, copying its collections.</summary>
    /// <param name="rows">One row per run, in run order.</param>
    /// <param name="observables">The observables tabled, in column order.</param>
    /// <param name="distributions">Each observable's distribution over the completed runs, in column order.</param>
    /// <param name="lines">Each distribution and a case's conclusion as <c>sysml -runs</c> prints them under the table.</param>
    /// <param name="outputs">A case's conclusion: its outputs over the completed runs.</param>
    /// <param name="verdicts">A case's conclusion: its verdicts, which settled every row's.</param>
    /// <param name="unconcluded">Why a case has no conclusion; empty when it has one, or for an action.</param>
    /// <param name="seed">The seed the runs drew from; null when they drew none.</param>
    /// <param name="instances">Every object a row's verdict is about.</param>
    /// <param name="diagnostics">What the service reported.</param>
    /// <param name="standing">The engine that made the runs, the strength of its evidence and its bounds.</param>
    /// <exception cref="ArgumentNullException">A collection, <paramref name="unconcluded"/> or <paramref name="standing"/> is null.</exception>
    /// <exception cref="ArgumentException">A collection holds a null.</exception>
    public Runs(
        IEnumerable<SweepRow> rows,
        IEnumerable<string> observables,
        IEnumerable<RunDistribution> distributions,
        IEnumerable<string> lines,
        IEnumerable<CalcOutput> outputs,
        IEnumerable<Verdict> verdicts,
        string unconcluded,
        ulong? seed,
        IEnumerable<Instance> instances,
        IEnumerable<Diagnostic> diagnostics,
        Standing standing)
    {
        ArgumentNullException.ThrowIfNull(unconcluded);
        ArgumentNullException.ThrowIfNull(standing);
        Rows = new ValueList<SweepRow>(rows);
        Observables = new ValueList<string>(observables);
        Distributions = new ValueList<RunDistribution>(distributions);
        Lines = new ValueList<string>(lines);
        Outputs = new ValueList<CalcOutput>(outputs);
        Verdicts = new ValueList<Verdict>(verdicts);
        Unconcluded = unconcluded;
        Seed = seed;
        Instances = new ValueList<Instance>(instances);
        Diagnostics = new ValueList<Diagnostic>(diagnostics);
        Standing = standing;
    }

    /// <summary>
    /// One row per run, in run order: its <see cref="SweepRow.Inputs"/> carry the run's number as <c>run</c>, its
    /// <see cref="SweepRow.Outputs"/> what the run observed, and its <see cref="SweepRow.Verdicts"/> a case's in-run verdicts.
    /// </summary>
    public ValueList<SweepRow> Rows { get; }

    /// <summary>The observables tabled, in column order: those asked for, else every feature the action holds and <c>clock</c>; <c>observed</c> for a case.</summary>
    public ValueList<string> Observables { get; }

    /// <summary>Each observable's distribution over the completed runs, in column order.</summary>
    public ValueList<RunDistribution> Distributions { get; }

    /// <summary>Each distribution and a case's conclusion as <c>sysml -runs</c> prints them under the table.</summary>
    public ValueList<string> Lines { get; }

    /// <summary>A case's conclusion over the completed runs: its outputs; empty for an action, or a case with no conclusion.</summary>
    public ValueList<CalcOutput> Outputs { get; }

    /// <summary>A case's conclusion over the completed runs: its verdicts, which settled every row's; empty for an action.</summary>
    public ValueList<Verdict> Verdicts { get; }

    /// <summary>Why a case has no conclusion: no run completed, or the sample was refused; empty when it has one, or for an action.</summary>
    public string Unconcluded { get; }

    /// <summary>The seed the runs drew from, echoed so the runs can be made again; null when they drew from none.</summary>
    public ulong? Seed { get; }

    /// <summary>Every object a row's verdict, or the conclusion's, is about, each row's numbered after the rows before it.</summary>
    public ValueList<Instance> Instances { get; }

    /// <summary>What the service reported.</summary>
    public ValueList<Diagnostic> Diagnostics { get; }

    /// <summary>The engine that made the runs, the strength of its evidence and the bounds it ran under.</summary>
    public Standing Standing { get; }

    /// <summary>The rows whose run failed.</summary>
    public ValueList<SweepRow> Failures => new(Rows.Where(row => row.Failed));

    /// <summary>The distribution of an observable.</summary>
    /// <param name="observable">The observable's name, as <see cref="Observables"/> lists it.</param>
    /// <returns>Its distribution; null when no observable is so named.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="observable"/> is null.</exception>
    public RunDistribution? Distribution(string observable)
    {
        ArgumentNullException.ThrowIfNull(observable);
        return Distributions.FirstOrDefault(distribution => distribution.Name == observable);
    }

    /// <summary>The object of an id.</summary>
    /// <param name="instanceId">The id the service gave the object.</param>
    /// <returns>The object; null when it is not among <see cref="Instances"/>.</returns>
    public Instance? FindInstance(long instanceId) => InstanceSearch.Find(Instances, instanceId);

    /// <summary>The object a reference refers to.</summary>
    /// <param name="reference">A reference among these runs' values.</param>
    /// <returns>The object; null when it is not among <see cref="Instances"/>.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="reference"/> is null.</exception>
    public Instance? Resolve(Value.InstanceRefValue reference)
    {
        ArgumentNullException.ThrowIfNull(reference);
        return FindInstance(reference.InstanceId);
    }
}
