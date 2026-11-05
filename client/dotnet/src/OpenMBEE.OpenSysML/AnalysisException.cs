using System;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// An analysis case the service could not run to its end, reported in its answer: an unknown symbol, a subject that
/// could not be built or bound, an input with no value, a failed step. It keeps the <see cref="Partial"/> run: the
/// outputs computed, the verdicts (each undecided), the evaluations made and the objects named before the failure.
/// </summary>
public class AnalysisException : VerificationException
{
    /// <summary>Creates the exception.</summary>
    /// <param name="message">The failure, as the service worded it.</param>
    /// <param name="method">The method whose answer reported it.</param>
    /// <param name="reason">What kind of failure the answer reported.</param>
    /// <param name="partial">What the run reported before it failed.</param>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    public AnalysisException(string message, string method, FailureReason reason, Analysis partial)
        : base(message, method, Checked(partial).Diagnostics, reason, partial.Instances, partial.Verifications, partial.Standing)
    {
        Partial = partial;
    }

    /// <summary>What the run reported before it failed; its collections are empty when it failed before running.</summary>
    public Analysis Partial { get; }

    private static Analysis Checked(Analysis partial)
    {
        ArgumentNullException.ThrowIfNull(partial);
        return partial;
    }
}
