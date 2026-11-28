using System;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// No service could be connected to: none was named and none could be started, or the one reached
/// is not the one asked for (<see cref="StaleServiceException"/>).
/// </summary>
public class ServiceStartException : OpenSysMLException
{
    /// <summary>Creates the exception.</summary>
    /// <param name="message">What failed.</param>
    public ServiceStartException(string message)
        : base(message)
    {
    }

    /// <summary>Creates the exception with a cause.</summary>
    /// <param name="message">What failed.</param>
    /// <param name="innerException">The underlying failure.</param>
    public ServiceStartException(string message, Exception? innerException)
        : base(message, innerException)
    {
    }
}
