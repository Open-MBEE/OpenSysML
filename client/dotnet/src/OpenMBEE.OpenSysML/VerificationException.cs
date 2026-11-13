using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// A verification, validation, calculation, analysis, sweep, set of runs or recording the service could not answer at
/// all, reported in its answer: an unknown symbol, a symbol of another kind, a subject that could not be built, an
/// argument that does not read, a range no values follow from. A verdict of false, or an undecided verdict, is never
/// this exception; nor is one failed row of a sweep or of runs. It keeps what the answer reported beside the failure.
/// </summary>
public class VerificationException : ModelException
{
    /// <summary>Creates the exception, copying its collections.</summary>
    /// <param name="message">The failure, as the service worded it.</param>
    /// <param name="method">The method whose answer reported it.</param>
    /// <param name="diagnostics">The diagnostics the answer carried.</param>
    /// <param name="reason">What kind of failure the answer reported.</param>
    /// <param name="instances">The objects the answer reported beside the failure.</param>
    /// <param name="verifications">The verification verdicts the answer reported beside the failure.</param>
    /// <param name="standing">The engine the answer named, the strength of its evidence and its bounds.</param>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    /// <exception cref="ArgumentException">A collection holds a null.</exception>
    public VerificationException(
        string message,
        string method,
        IEnumerable<Diagnostic> diagnostics,
        FailureReason reason,
        IEnumerable<Instance> instances,
        IEnumerable<VerificationVerdict> verifications,
        Standing standing)
        : base(message, method, diagnostics)
    {
        ArgumentNullException.ThrowIfNull(standing);
        Reason = reason;
        Instances = new ValueList<Instance>(instances);
        Verifications = new ValueList<VerificationVerdict>(verifications);
        Standing = standing;
    }

    /// <summary>
    /// What kind of failure the answer reported: <see cref="FailureReason.WrongKind"/> for a symbol declaring something
    /// else, which is a wrong request; <see cref="FailureReason.Unspecified"/> where the answer classifies none.
    /// </summary>
    public FailureReason Reason { get; }

    /// <summary>The objects the answer reported beside the failure, as a validation's object it could not validate; empty when none.</summary>
    public ValueList<Instance> Instances { get; }

    /// <summary>The verification verdicts the answer reported beside the failure; empty when none.</summary>
    public ValueList<VerificationVerdict> Verifications { get; }

    /// <summary>The engine the answer named, the strength of its evidence and the bounds it ran under; <see cref="Standing.None"/> when no engine was asked.</summary>
    public Standing Standing { get; }
}
