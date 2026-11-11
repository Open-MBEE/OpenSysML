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
public class TransportException : CallException
{
    /// <summary>Creates the exception.</summary>
    /// <param name="statusCode">The canonical status the failure maps to.</param>
    /// <param name="message">What failed.</param>
    /// <param name="method">The method called, as <c>sysml.SysMLService/ParseFile</c>.</param>
    /// <param name="innerException">The underlying failure, if any.</param>
    /// <exception cref="ArgumentNullException"><paramref name="method"/> is null.</exception>
    public TransportException(StatusCode statusCode, string message, string method, Exception? innerException = null)
        : base(statusCode, message, method, innerException)
    {
    }

    // An answer that does not read as its schema says, which the service would not send: "{method} answered {what}."
    internal static TransportException Malformed(string method, string what, Exception? cause = null) =>
        new(StatusCode.Internal, $"{method} answered {what}.", method, cause);
}
