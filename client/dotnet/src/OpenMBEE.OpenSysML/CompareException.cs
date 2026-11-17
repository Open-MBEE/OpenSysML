using System;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// Results the service could not compare at all, reported in its answer, with nothing run. A configuration that could
/// not be run, or whose runs failed, is never this exception: it is a <see cref="ConfigurationComparison"/> with that
/// status. It keeps the <see cref="Partial"/> comparison: the source, any configuration and the standing of the engine,
/// whatever the answer reported beside the failure.
/// </summary>
public class CompareException : VerificationException
{
    /// <summary>Creates the exception.</summary>
    /// <param name="message">The failure, as the service worded it.</param>
    /// <param name="method">The method whose answer reported it.</param>
    /// <param name="reason">What kind of failure the answer reported.</param>
    /// <param name="partial">What the answer reported beside the failure.</param>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    public CompareException(string message, string method, FailureReason reason, Comparison partial)
        : base(message, method, Checked(partial).Diagnostics, reason, [], [], partial.Standing)
    {
        Partial = partial;
    }

    /// <summary>What the answer reported beside the failure; its configurations are empty when nothing was compared.</summary>
    public Comparison Partial { get; }

    private static Comparison Checked(Comparison partial)
    {
        ArgumentNullException.ThrowIfNull(partial);
        return partial;
    }
}
