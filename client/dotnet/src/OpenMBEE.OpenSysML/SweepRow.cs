using System;
using System.Collections.Generic;
using System.Linq;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// One run of a sweep: what it bound, what it produced and decided, how long it took, and what stopped it where
/// it failed. A failed run is a row carrying its <see cref="Error"/> beside what it computed first, so one
/// failure never ends the table.
/// </summary>
public sealed record SweepRow
{
    /// <summary>Creates a row, copying its collections.</summary>
    /// <param name="inputs">The parameters this row bound, in the order their ranges were given.</param>
    /// <param name="outputs">What the run produced: an analysis case's outputs, or a calc's value named <c>result</c>.</param>
    /// <param name="verdicts">What the case's objective and assertions decided; empty for a calc.</param>
    /// <param name="evaluations">Each application the run made of one of the case's calcs as a function value.</param>
    /// <param name="elapsedMicroseconds">The run's wall time, in microseconds.</param>
    /// <param name="error">Why this run failed; empty when it did not.</param>
    /// <param name="failureReason">What kind of failure <paramref name="error"/> reports.</param>
    /// <exception cref="ArgumentNullException">A collection or <paramref name="error"/> is null.</exception>
    /// <exception cref="ArgumentException">A collection holds a null.</exception>
    public SweepRow(
        IEnumerable<CalcOutput> inputs,
        IEnumerable<CalcOutput> outputs,
        IEnumerable<Verdict> verdicts,
        IEnumerable<CaseEvaluation> evaluations,
        long elapsedMicroseconds,
        string error,
        FailureReason failureReason)
    {
        ArgumentNullException.ThrowIfNull(error);
        Inputs = new ValueList<CalcOutput>(inputs);
        Outputs = new ValueList<CalcOutput>(outputs);
        Verdicts = new ValueList<Verdict>(verdicts);
        Evaluations = new ValueList<CaseEvaluation>(evaluations);
        ElapsedMicroseconds = elapsedMicroseconds;
        Error = error;
        FailureReason = failureReason;
    }

    /// <summary>The parameters this row bound, in the order their ranges were given.</summary>
    public ValueList<CalcOutput> Inputs { get; }

    /// <summary>What the run produced, before any failure: an analysis case's outputs, or a calc's returned value named <c>result</c>.</summary>
    public ValueList<CalcOutput> Outputs { get; }

    /// <summary>What the case's objective and assertions decided; empty for a calc.</summary>
    public ValueList<Verdict> Verdicts { get; }

    /// <summary>Each application this run made of one of the case's own calcs as a function value.</summary>
    public ValueList<CaseEvaluation> Evaluations { get; }

    /// <summary>The run's wall time, in microseconds.</summary>
    public long ElapsedMicroseconds { get; }

    /// <summary>Why this run failed rather than the table; empty when it did not.</summary>
    public string Error { get; }

    /// <summary>What kind of failure <see cref="Error"/> reports.</summary>
    public FailureReason FailureReason { get; }

    /// <summary>Whether this run failed.</summary>
    public bool Failed => Error.Length > 0;

    /// <summary>Whether the run did not fail and every verdict is decided and holds.</summary>
    public bool Holds => !Failed && Verdicts.All(verdict => verdict.Decided && verdict.Holds);
}
