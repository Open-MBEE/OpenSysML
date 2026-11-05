using System;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// A check the service refused in its answer, before any search: the behavior does not resolve, a
/// property names no constraint or requirement, the performer could not be made. It keeps what the
/// answer reported beside the failure, including its <see cref="CheckResult.Verdict"/> when supplied.
/// </summary>
public class CheckException : ModelException
{
    /// <summary>Creates the exception.</summary>
    /// <param name="message">The failure, as the service worded it.</param>
    /// <param name="method">The method whose answer reported it.</param>
    /// <param name="partial">What the answer reported beside the failure.</param>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    public CheckException(string message, string method, CheckResult partial)
        : base(message, method, [])
    {
        ArgumentNullException.ThrowIfNull(partial);
        Partial = partial;
    }

    /// <summary>What the answer reported beside the failure: its verdict, and anything else the service set.</summary>
    public CheckResult Partial { get; }
}
