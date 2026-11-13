using System;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// The service does not hold the model a call named: a hash it never issued, or one its bounded
/// model cache has since evicted. Load the model again. Raised from the method's
/// <see cref="StatusCode.NotFound"/>, which for a call on a model means nothing else.
/// </summary>
public class ModelNotFoundException : ServiceException
{
    /// <summary>Creates the exception.</summary>
    /// <param name="serviceMessage">What the service said.</param>
    /// <param name="method">The method called, as <c>sysml.SysMLService/GetSymbol</c>.</param>
    /// <param name="innerException">The protocol library's report of the status, if any.</param>
    public ModelNotFoundException(string serviceMessage, string method, Exception? innerException = null)
        : base(StatusCode.NotFound, serviceMessage, method, innerException)
    {
    }
}
