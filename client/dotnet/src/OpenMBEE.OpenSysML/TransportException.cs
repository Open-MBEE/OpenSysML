using System;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// A call failed without a well-formed answer from the service: the endpoint could not be reached
/// or did not speak the protocol, the answer was malformed or over a size limit, or the call
/// outlived its deadline. <see cref="Exception.InnerException"/> holds the underlying cause.
/// </summary>
/// <remarks>
/// That a call failed in transport does not show that the service never received it: a request
/// can be executed and its answer lost. This client never retries a call.
/// </remarks>
public class TransportException : OpenSysMLException
{
    /// <summary>Creates the exception.</summary>
    /// <param name="statusCode">The canonical status the failure maps to.</param>
    /// <param name="message">What failed.</param>
    /// <param name="method">The method called, as <c>sysml.SysMLService/ParseFile</c>.</param>
    /// <param name="innerException">The underlying failure, if any.</param>
    public TransportException(StatusCode statusCode, string message, string method, Exception? innerException = null)
        : base(message, innerException)
    {
        ArgumentNullException.ThrowIfNull(method);
        StatusCode = statusCode;
        Method = method;
    }

    /// <summary>The canonical status the failure maps to.</summary>
    public StatusCode StatusCode { get; }

    /// <summary>The method called, as <c>sysml.SysMLService/ParseFile</c>.</summary>
    public string Method { get; }
}
