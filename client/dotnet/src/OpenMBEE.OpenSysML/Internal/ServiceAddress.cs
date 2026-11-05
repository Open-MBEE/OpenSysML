using System;

namespace OpenMBEE.OpenSysML.Internal;

/// <summary>Reads the address of a service a caller names.</summary>
internal static class ServiceAddress
{
    /// <summary>
    /// The base URL of a named service: <c>host:port</c> as <c>http://host:port</c>, or an
    /// <c>http</c> or <c>https</c> URL that names nothing but its scheme, host and port.
    /// </summary>
    /// <param name="text">The address as given.</param>
    /// <param name="source">Where it was given, as ConnectionOptions.Address, for the refusal.</param>
    /// <returns>The base URL, as <c>http://host:port/</c>.</returns>
    /// <exception cref="InvalidRequestException">The address is not one this client can connect to.</exception>
    internal static Uri Parse(string text, string source)
    {
        var address = text.Trim();
        if (address.Length == 0)
        {
            throw Invalid(text, source, "it is empty");
        }

        if (address.Contains('?', StringComparison.Ordinal) || address.Contains('#', StringComparison.Ordinal))
        {
            throw Invalid(text, source, "it has a query or fragment");
        }

        Uri? uri;
        if (address.Contains("://", StringComparison.Ordinal))
        {
            if (!Uri.TryCreate(address, UriKind.Absolute, out uri))
            {
                throw Invalid(text, source, "it is not a URL");
            }

            if (uri.Scheme != Uri.UriSchemeHttp && uri.Scheme != Uri.UriSchemeHttps)
            {
                throw Invalid(text, source, $"its scheme is {uri.Scheme}, not http or https");
            }
        }
        else
        {
            if (address.Contains('/', StringComparison.Ordinal) || address.Contains('@', StringComparison.Ordinal)
                || !Uri.TryCreate("http://" + address, UriKind.Absolute, out uri) || !HasPort(address))
            {
                throw Invalid(text, source, "it is neither host:port nor an http or https URL");
            }
        }

        if (uri.UserInfo.Length != 0)
        {
            throw Invalid(text, source, "it carries user information; send credentials as a header");
        }

        if (uri.AbsolutePath != "/")
        {
            throw Invalid(text, source, "it has a path, and the service is served at the root");
        }

        return new Uri(uri.GetLeftPart(UriPartial.Authority) + "/", UriKind.Absolute);
    }

    // Uri has already checked that a port is digits in range, but it defaults one that is absent or empty.
    private static bool HasPort(string address)
    {
        var colon = address.LastIndexOf(':');
        return colon > address.LastIndexOf(']') && colon < address.Length - 1;
    }

    private static InvalidRequestException Invalid(string text, string source, string why) =>
        new($"{source} is \"{text}\", which is not a service address: {why}. Name one as host:port or as an http or https URL.");
}
