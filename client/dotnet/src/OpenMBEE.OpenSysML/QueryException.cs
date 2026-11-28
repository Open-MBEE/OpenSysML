using System;
using System.Linq;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// A document query or document the model declares that could not be planned or run: a fault in the model's own
/// declarations (an operation the engine does not support, a property no element has, a document that does not
/// compose), which the service fails the call for with <see cref="StatusCode.FailedPrecondition"/>, naming the
/// declaration behind it where it can. The call was refused, but what is wrong is the model, not the request or the
/// service, so this is a <see cref="ModelException"/> keeping the status and message the service gave.
/// </summary>
public class QueryException : ModelException
{
    /// <summary>Creates the exception.</summary>
    /// <param name="statusCode">The status the service failed the call with.</param>
    /// <param name="serviceMessage">What the service said.</param>
    /// <param name="method">The method called, as <c>sysml.SysMLService/RunDocumentQuery</c>.</param>
    /// <param name="innerException">The protocol library's report of the status, if any.</param>
    /// <exception cref="ArgumentNullException"><paramref name="serviceMessage"/> or <paramref name="method"/> is null.</exception>
    public QueryException(StatusCode statusCode, string serviceMessage, string method, Exception? innerException = null)
        : base(serviceMessage, method, Enumerable.Empty<Diagnostic>(), innerException)
    {
        StatusCode = statusCode;
        ServiceMessage = serviceMessage;
    }

    /// <summary>The status the service failed the call with.</summary>
    public StatusCode StatusCode { get; }

    /// <summary>What the service said, verbatim.</summary>
    public string ServiceMessage { get; }
}
