using System;

namespace OpenMBEE.OpenSysML.Internal;

/// <summary>The exception for a status the service sent, made the same way by every transport.</summary>
internal static class ServiceErrors
{
    /// <summary>
    /// The exception for a status the service sent: <see cref="AuthenticationException"/> for
    /// <see cref="StatusCode.Unauthenticated"/>, saying what the call carried, else <see cref="ServiceException"/>.
    /// </summary>
    /// <param name="status">The status; never <see cref="StatusCode.Ok"/>.</param>
    /// <param name="message">What the service said with it.</param>
    /// <param name="method">The method called.</param>
    /// <param name="credentials">Where the credentials the call carried came from.</param>
    /// <param name="innerException">The protocol library's report of the status, if any.</param>
    /// <returns>The exception to raise.</returns>
    internal static ServiceException FromService(StatusCode status, string message, string method, CredentialSource credentials, Exception? innerException = null) =>
        status == StatusCode.Unauthenticated
            ? new AuthenticationException(message, method, credentials != CredentialSource.None, Remedy(credentials), innerException)
            : new ServiceException(status, message, method, innerException);

    private static string Remedy(CredentialSource credentials)
    {
        var supply = $"ConnectionOptions.Token or ${ConnectionOptions.TokenVariable}";
        return credentials switch
        {
            CredentialSource.Token => "the bearer token ConnectionOptions.Token gives was refused: give the token this service was started with",
            CredentialSource.TokenVariable => $"the bearer token ${ConnectionOptions.TokenVariable} names was refused: give the token this service was started with, by {supply}",
            CredentialSource.Header => $"the authorization header ConnectionOptions.Headers gives was refused: correct it, or remove it and give the token this service was started with by {supply}",
            _ => $"the service requires a bearer token and none was sent: give the token it was started with, by {supply}",
        };
    }
}
