namespace OpenMBEE.OpenSysML.Internal;

/// <summary>
/// The bearer token a service may require (RFC 6750): what one may be, as the service itself reads it, and the
/// <c>authorization</c> value that carries it. A token is never written into a message or a log line.
/// </summary>
internal static class BearerToken
{
    /// <summary>The header that carries the token, as every protocol sends it: in lower case.</summary>
    internal const string Header = "authorization";

    /// <summary>What a token may be, for a refusal that names the rule but never the value.</summary>
    internal const string Rule = "a token is letters, digits and -._~+/ with trailing = only (RFC 6750 b64token), with no spaces or line breaks";

    /// <summary>Whether <paramref name="token"/> is a b64token of RFC 6750 § 2.1, the only form the service accepts.</summary>
    /// <param name="token">The token.</param>
    /// <returns>Whether it can be sent.</returns>
    internal static bool IsValid(string token)
    {
        var body = token.TrimEnd('=');
        if (body.Length == 0)
        {
            return false;
        }

        foreach (var c in body)
        {
            if (!(char.IsAsciiLetterOrDigit(c) || c is '-' or '.' or '_' or '~' or '+' or '/'))
            {
                return false;
            }
        }

        return true;
    }

    /// <summary>The <see cref="Header"/> value carrying <paramref name="token"/>.</summary>
    /// <param name="token">A valid token.</param>
    /// <returns>The header value.</returns>
    internal static string HeaderValue(string token) => "Bearer " + token;
}
