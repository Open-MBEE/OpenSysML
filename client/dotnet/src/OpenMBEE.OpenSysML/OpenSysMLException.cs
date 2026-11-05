using System;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// The base of every failure this client reports. Cancellation by the caller's token is the
/// standard <see cref="OperationCanceledException"/>, and a call on a disposed connection the
/// standard <see cref="ObjectDisposedException"/>; neither derives from this class.
/// </summary>
public class OpenSysMLException : Exception
{
    /// <summary>Creates an exception.</summary>
    public OpenSysMLException()
    {
    }

    /// <summary>Creates an exception.</summary>
    /// <param name="message">What failed.</param>
    public OpenSysMLException(string message)
        : base(message)
    {
    }

    /// <summary>Creates an exception with a cause.</summary>
    /// <param name="message">What failed.</param>
    /// <param name="innerException">The underlying failure.</param>
    public OpenSysMLException(string message, Exception? innerException)
        : base(message, innerException)
    {
    }
}
