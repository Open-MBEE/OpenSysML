using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// What <see cref="Model.SelfCheckAsync"/> found: every finding of the <c>SysMLValidation</c> constraints, and those of
/// any rule packages, applied reflectively to every element of the model, the counts, and the verdict over them. A
/// violation is a finding, never an exception.
/// </summary>
public sealed record SelfCheck
{
    /// <summary>The verdict when no constraint is violated and no evaluation errored.</summary>
    public const string HoldsStatus = "holds";

    /// <summary>The verdict when a constraint is violated.</summary>
    public const string FailsStatus = "fails";

    /// <summary>The verdict when no constraint is violated but an evaluation errored.</summary>
    public const string UnresolvedStatus = "unresolved";

    /// <summary>Creates a self-check report, copying its collections.</summary>
    /// <param name="status">The verdict: <see cref="HoldsStatus"/>, <see cref="FailsStatus"/> or <see cref="UnresolvedStatus"/>.</param>
    /// <param name="findings">Every finding, in the order reported.</param>
    /// <param name="elements">The elements the check visited.</param>
    /// <param name="checks">The constraint applications made.</param>
    /// <param name="violations">The applications answered false.</param>
    /// <param name="unevaluated">The applications the runtime could not make.</param>
    /// <param name="evaluationErrors">The applications and collections that errored.</param>
    /// <param name="applications">The elements each constraint was applied to, by constraint, in name order.</param>
    /// <param name="summary">The one-line total <c>sysml -self-check</c> prints last.</param>
    /// <param name="diagnostics">What the service reported.</param>
    /// <exception cref="ArgumentNullException"><paramref name="status"/>, <paramref name="summary"/> or a collection is null.</exception>
    /// <exception cref="ArgumentException">A collection holds a null.</exception>
    public SelfCheck(
        string status,
        IEnumerable<SelfCheckFinding> findings,
        long elements,
        long checks,
        long violations,
        long unevaluated,
        long evaluationErrors,
        IEnumerable<SelfCheckApplication> applications,
        string summary,
        IEnumerable<Diagnostic> diagnostics)
    {
        ArgumentNullException.ThrowIfNull(status);
        ArgumentNullException.ThrowIfNull(summary);
        Status = status;
        Findings = new ValueList<SelfCheckFinding>(findings);
        Elements = elements;
        Checks = checks;
        Violations = violations;
        Unevaluated = unevaluated;
        EvaluationErrors = evaluationErrors;
        Applications = new ValueList<SelfCheckApplication>(applications);
        Summary = summary;
        Diagnostics = new ValueList<Diagnostic>(diagnostics);
    }

    /// <summary>
    /// The verdict, as the service spells it: <see cref="HoldsStatus"/>, <see cref="FailsStatus"/> or
    /// <see cref="UnresolvedStatus"/>; a newer service's spelling is kept as given.
    /// </summary>
    public string Status { get; }

    /// <summary>Every finding, in the order reported.</summary>
    public ValueList<SelfCheckFinding> Findings { get; }

    /// <summary>The elements the check visited.</summary>
    public long Elements { get; }

    /// <summary>The constraint applications made.</summary>
    public long Checks { get; }

    /// <summary>The applications answered false.</summary>
    public long Violations { get; }

    /// <summary>The applications the runtime could not make.</summary>
    public long Unevaluated { get; }

    /// <summary>The applications and collections that errored.</summary>
    public long EvaluationErrors { get; }

    /// <summary>The elements each constraint was applied to, by constraint, in name order.</summary>
    public ValueList<SelfCheckApplication> Applications { get; }

    /// <summary>The one-line total <c>sysml -self-check</c> prints last.</summary>
    public string Summary { get; }

    /// <summary>What the service reported.</summary>
    public ValueList<Diagnostic> Diagnostics { get; }

    /// <summary>Whether the verdict is <see cref="HoldsStatus"/>: no constraint violated and no evaluation errored.</summary>
    public bool Holds => Status == HoldsStatus;
}
