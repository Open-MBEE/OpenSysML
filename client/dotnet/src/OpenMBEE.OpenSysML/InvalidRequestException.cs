using System;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// A request or option was invalid and was refused locally, before anything was sent: an option
/// out of range, an address or header the client cannot send, or a raw call whose method and
/// message types the schema does not pair.
/// </summary>
public class InvalidRequestException : OpenSysMLException
{
    /// <summary>Creates the exception.</summary>
    /// <param name="message">What is invalid.</param>
    public InvalidRequestException(string message)
        : base(message)
    {
    }

    /// <summary>Creates the exception with a cause.</summary>
    /// <param name="message">What is invalid.</param>
    /// <param name="innerException">The underlying failure.</param>
    public InvalidRequestException(string message, Exception? innerException)
        : base(message, innerException)
    {
    }
}
