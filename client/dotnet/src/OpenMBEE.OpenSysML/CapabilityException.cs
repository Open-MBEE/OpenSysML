using System;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// The service does not advertise a capability that was required of it. Raised before any request
/// that needs the capability is sent, since the advertised list is the only reliable answer.
/// </summary>
public class CapabilityException : OpenSysMLException
{
    /// <summary>Creates the exception.</summary>
    /// <param name="capability">The capability the service does not advertise.</param>
    /// <param name="message">What needed it.</param>
    /// <param name="remedy">How to reach a service that advertises it.</param>
    public CapabilityException(string capability, string message, string remedy)
        : base(message)
    {
        ArgumentNullException.ThrowIfNull(capability);
        ArgumentNullException.ThrowIfNull(remedy);
        Capability = capability;
        Remedy = remedy;
    }

    /// <summary>The capability the service does not advertise.</summary>
    public string Capability { get; }

    /// <summary>How to reach a service that advertises it.</summary>
    public string Remedy { get; }
}
