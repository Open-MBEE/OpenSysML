using System;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// A call that failed with a status rather than an answer: a <see cref="ServiceException"/> for a status the service
/// sent, a <see cref="TransportException"/> for one this client gave a call that read no well-formed answer. Catching
/// it reads the status of any failed call in one place.
/// </summary>
/// <remarks>
/// A <see cref="QueryException"/> keeps the status the service sent too, but is a <see cref="ModelException"/>: the call
/// was refused for a fault in the model's own declarations, not in the request or the service.
/// </remarks>
public abstract class CallException : OpenSysMLException
{
    /// <summary>Creates the exception.</summary>
    /// <param name="statusCode">The status the call failed with.</param>
    /// <param name="message">What failed.</param>
    /// <param name="method">The method called, as <c>sysml.SysMLService/ParseFile</c>.</param>
    /// <param name="innerException">The underlying failure, if any.</param>
    /// <exception cref="ArgumentNullException"><paramref name="method"/> is null.</exception>
    protected CallException(StatusCode statusCode, string message, string method, Exception? innerException)
        : base(message, innerException)
    {
        ArgumentNullException.ThrowIfNull(method);
        StatusCode = statusCode;
        Method = method;
    }

    /// <summary>The status the call failed with.</summary>
    public StatusCode StatusCode { get; }

    /// <summary>The method called, as <c>sysml.SysMLService/ParseFile</c>.</summary>
    public string Method { get; }
}
