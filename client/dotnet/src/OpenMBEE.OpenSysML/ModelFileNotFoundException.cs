using System;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// The service could not read a source file a parse named. The path is read by the service, in its
/// own file system, not by this client. Raised from the parse's <see cref="StatusCode.NotFound"/>,
/// which for a parse means nothing else.
/// </summary>
public class ModelFileNotFoundException : ServiceException
{
    /// <summary>Creates the exception.</summary>
    /// <param name="serviceMessage">What the service said, naming the file.</param>
    /// <param name="method">The method called, as <c>sysml.SysMLService/ParseFile</c>.</param>
    /// <param name="innerException">The protocol library's report of the status, if any.</param>
    public ModelFileNotFoundException(string serviceMessage, string method, Exception? innerException = null)
        : base(StatusCode.NotFound, serviceMessage, method, innerException)
    {
    }
}
