using System;
using System.Collections.Generic;
using System.Linq;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// A recording the service did not make, reported in its answer: the case could not be found or run, no run completed,
/// the package named could not take the records, or the records would not analyze cleanly. Nothing was recorded, so
/// the service holds no new model. It keeps what the answer reported beside the failure: the run, sweep or sample made,
/// and, when the records were generated but refused, their declarations.
/// <see cref="VerificationException.Instances"/>, <see cref="VerificationException.Verifications"/> and
/// <see cref="VerificationException.Standing"/> are those of the run, sweep or sample.
/// </summary>
public class RecordException : VerificationException
{
    /// <summary>Creates the exception, copying its collections.</summary>
    /// <param name="message">The failure, as the service worded it.</param>
    /// <param name="method">The method whose answer reported it.</param>
    /// <param name="reason">What kind of failure the answer reported.</param>
    /// <param name="diagnostics">The diagnostics the answer carried: those of the recorded model, when it would not analyze cleanly.</param>
    /// <param name="run">The run of the case, when it was asked to run once and got that far.</param>
    /// <param name="sweep">The sweep's table, when it was asked to run a sweep and got that far.</param>
    /// <param name="sample">The sample's runs, when it was asked to run a sample and got that far.</param>
    /// <param name="runFailed">Whether the run, sweep or sample itself reported the failure.</param>
    /// <param name="package">The qualified name of the package the records were to go into; empty when they were not generated.</param>
    /// <param name="definition">The qualified name of the record definition generated; empty when none was.</param>
    /// <param name="records">The qualified names of the records generated.</param>
    /// <param name="generatedSource">The record declarations generated; empty when none were.</param>
    /// <param name="skipped">The rows or runs that failed and were not to be recorded.</param>
    /// <param name="skipReasons">Why runs were not to be recorded.</param>
    /// <exception cref="ArgumentNullException">A string or a collection is null.</exception>
    /// <exception cref="ArgumentException">
    /// A collection holds a null, more than one of <paramref name="run"/>, <paramref name="sweep"/> and
    /// <paramref name="sample"/> is given, or <paramref name="runFailed"/> is set with none.
    /// </exception>
    public RecordException(
        string message,
        string method,
        FailureReason reason,
        IEnumerable<Diagnostic> diagnostics,
        Analysis? run,
        Sweep? sweep,
        Runs? sample,
        bool runFailed,
        string package,
        string definition,
        IEnumerable<string> records,
        string generatedSource,
        long skipped,
        IEnumerable<string> skipReasons)
        : base(
            message,
            method,
            diagnostics,
            reason,
            Checked(run, sweep, sample, runFailed),
            run?.Verifications ?? default,
            run?.Standing ?? sweep?.Standing ?? sample?.Standing ?? Standing.None)
    {
        ArgumentNullException.ThrowIfNull(package);
        ArgumentNullException.ThrowIfNull(definition);
        ArgumentNullException.ThrowIfNull(generatedSource);
        Run = run;
        Sweep = sweep;
        Sample = sample;
        RunFailed = runFailed;
        Package = package;
        Definition = definition;
        Records = new ValueList<string>(records);
        GeneratedSource = generatedSource;
        Skipped = skipped;
        SkipReasons = new ValueList<string>(skipReasons);
    }

    /// <summary>The run of the case, when it was asked to run once and got that far; null otherwise.</summary>
    public Analysis? Run { get; }

    /// <summary>The sweep's table, when it was asked to run a sweep and got that far; null otherwise.</summary>
    public Sweep? Sweep { get; }

    /// <summary>The sample's runs, when it was asked to run a sample and got that far; null otherwise.</summary>
    public Runs? Sample { get; }

    /// <summary>
    /// Whether the run, sweep or sample itself reported this failure, as a run of the case reports one: the case could
    /// not be run, or no row or run of it could be made. False when runs were made and recording them failed, or
    /// nothing was run at all.
    /// </summary>
    public bool RunFailed { get; }

    /// <summary>The qualified name of the package the records were to go into; empty when they were not generated.</summary>
    public string Package { get; }

    /// <summary>The qualified name of the record definition generated; empty when none was.</summary>
    public string Definition { get; }

    /// <summary>The qualified names of the records generated, which the model was refused; empty when none were.</summary>
    public ValueList<string> Records { get; }

    /// <summary>The record declarations generated, as <see cref="Recording.Source"/> holds them, which the model was refused; empty when none were.</summary>
    public string GeneratedSource { get; }

    /// <summary>The rows or runs that failed and were not to be recorded.</summary>
    public long Skipped { get; }

    /// <summary>Why runs were not to be recorded, as <c>sysml -record-run</c> reports them.</summary>
    public ValueList<string> SkipReasons { get; }

    // The objects the runs reported, after checking the runs are one kind and a failure of theirs has them.
    private static ValueList<Instance> Checked(Analysis? run, Sweep? sweep, Runs? sample, bool runFailed)
    {
        var given = new object?[] { run, sweep, sample }.Count(answer => answer is not null);
        if (given > 1)
        {
            throw new ArgumentException("A recording runs its case one way, so it reports at most one of a run, a sweep and a sample.", nameof(run));
        }

        if (runFailed && given == 0)
        {
            throw new ArgumentException("A failure the runs reported needs the run, sweep or sample that reported it.", nameof(runFailed));
        }

        return run?.Instances ?? sweep?.Instances ?? sample?.Instances ?? default;
    }
}
