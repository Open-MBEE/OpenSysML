using System;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// The service answered a call with a status other than <see cref="StatusCode.Ok"/>: a well-formed
/// refusal it sent, distinct from a <see cref="TransportException"/>, where no such answer was read.
/// A deadline the service reports exceeded is a <see cref="TransportException"/>, as a local one is.
/// </summary>
public class ServiceException : OpenSysMLException
{
    /// <summary>Creates the exception for a status the service sent.</summary>
    /// <param name="statusCode">The status; never <see cref="StatusCode.Ok"/>.</param>
    /// <param name="serviceMessage">What the service said with it.</param>
    /// <param name="method">The method called, as <c>sysml.SysMLService/ParseFile</c>.</param>
    /// <param name="innerException">The protocol library's report of the status, if any.</param>
    /// <exception cref="ArgumentException"><paramref name="statusCode"/> is <see cref="StatusCode.Ok"/>.</exception>
    public ServiceException(StatusCode statusCode, string serviceMessage, string method, Exception? innerException = null)
        : base(Describe(statusCode, serviceMessage, method), innerException)
    {
        StatusCode = statusCode;
        ServiceMessage = serviceMessage;
        Method = method;
    }

    /// <summary>The status the service sent.</summary>
    public StatusCode StatusCode { get; }

    /// <summary>What the service said, without the method and status the message prefixes it with.</summary>
    public string ServiceMessage { get; }

    /// <summary>The method called, as <c>sysml.SysMLService/ParseFile</c>.</summary>
    public string Method { get; }

    private static string Describe(StatusCode statusCode, string serviceMessage, string method)
    {
        ArgumentNullException.ThrowIfNull(serviceMessage);
        ArgumentNullException.ThrowIfNull(method);
        if (statusCode == StatusCode.Ok)
        {
            throw new ArgumentException("A service exception reports a status other than Ok.", nameof(statusCode));
        }

        return $"{method} was refused by the service with {statusCode}: {serviceMessage}";
    }
}
