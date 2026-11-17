using System;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// Runs of an action or a Monte Carlo case the service could not table, reported in its answer: an unknown symbol, a
/// case that is not a <c>Simulation::MonteCarlo</c> one, an input that does not read, a feature asked to be observed
/// that no completed run produced. A run that failed is a row, never this exception. It keeps the <see cref="Partial"/>
/// table: the rows made, their distributions and the standing of the engine, whatever the answer reported beside the failure.
/// </summary>
public class RunsException : VerificationException
{
    /// <summary>Creates the exception.</summary>
    /// <param name="message">The failure, as the service worded it.</param>
    /// <param name="method">The method whose answer reported it.</param>
    /// <param name="reason">What kind of failure the answer reported.</param>
    /// <param name="partial">What the answer reported beside the failure.</param>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    public RunsException(string message, string method, FailureReason reason, Runs partial)
        : base(message, method, Checked(partial).Diagnostics, reason, partial.Instances, [], partial.Standing)
    {
        Partial = partial;
    }

    /// <summary>What the answer reported beside the failure; its collections are empty when no run was made.</summary>
    public Runs Partial { get; }

    private static Runs Checked(Runs partial)
    {
        ArgumentNullException.ThrowIfNull(partial);
        return partial;
    }
}
