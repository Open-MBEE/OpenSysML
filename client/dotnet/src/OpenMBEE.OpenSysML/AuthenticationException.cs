using System;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// The service requires a bearer token, and the call carried none or one it refused: its
/// <see cref="StatusCode.Unauthenticated"/>. A service requires one when it serves an address other machines
/// reach; supply it with <see cref="ConnectionOptions.Token"/> or <see cref="ConnectionOptions.TokenVariable"/>.
/// An HTTP 401 that is not the service's own refusal, as a proxy's page, is a <see cref="TransportException"/>.
/// </summary>
public class AuthenticationException : ServiceException
{
    /// <summary>Creates the exception.</summary>
    /// <param name="serviceMessage">What the service said.</param>
    /// <param name="method">The method called, as <c>sysml.SysMLService/ParseFile</c>.</param>
    /// <param name="credentialsSent">Whether the call carried credentials, which the service then refused.</param>
    /// <param name="remedy">What to do about it.</param>
    /// <param name="innerException">The protocol library's report of the status, if any.</param>
    public AuthenticationException(string serviceMessage, string method, bool credentialsSent, string remedy, Exception? innerException = null)
        : base(StatusCode.Unauthenticated, serviceMessage, method, innerException)
    {
        ArgumentNullException.ThrowIfNull(remedy);
        CredentialsSent = credentialsSent;
        Remedy = remedy;
    }

    /// <summary>Whether the call carried credentials, which the service then refused; false when it carried none.</summary>
    public bool CredentialsSent { get; }

    /// <summary>What to do about it.</summary>
    public string Remedy { get; }

    /// <inheritdoc/>
    public override string Message => $"{base.Message}\n  fix: {Remedy}";
}
