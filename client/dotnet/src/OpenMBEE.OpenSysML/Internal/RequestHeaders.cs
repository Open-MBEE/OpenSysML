using System;
using System.Collections.Generic;

namespace OpenMBEE.OpenSysML.Internal;

/// <summary>
/// The rules for the headers a caller adds to every call, shared by every protocol: they carry
/// authorization and custom metadata, never what the protocol itself controls.
/// </summary>
internal static class RequestHeaders
{
    private static readonly HashSet<string> Reserved = new(StringComparer.OrdinalIgnoreCase)
    {
        "accept",
        "accept-encoding",
        "connection",
        "content-encoding",
        "content-length",
        "content-type",
        "host",
        "keep-alive",
        "proxy-connection",
        "te",
        "trailer",
        "transfer-encoding",
        "upgrade",
        "user-agent",
    };

    private static readonly string[] ReservedPrefixes = ["grpc-", "connect-"];

    /// <summary>Refuses a header set that cannot be sent as given.</summary>
    /// <param name="headers">The headers, by name.</param>
    /// <exception cref="InvalidRequestException">A header is malformed, reserved or repeated.</exception>
    internal static void Validate(IReadOnlyDictionary<string, string> headers)
    {
        var seen = new HashSet<string>(StringComparer.OrdinalIgnoreCase);
        foreach (var (name, value) in headers)
        {
            if (name.Length == 0 || !name.AsSpan().ContainsOnlyNameCharacters())
            {
                throw new InvalidRequestException(
                    $"Header \"{name}\" is not a valid name: use ASCII letters, digits, '-', '_' and '.'.");
            }

            if (IsReserved(name))
            {
                throw new InvalidRequestException(
                    $"Header \"{name}\" is controlled by the protocol and cannot be set; add only authorization and custom metadata.");
            }

            if (name.EndsWith("-bin", StringComparison.OrdinalIgnoreCase))
            {
                throw new InvalidRequestException($"Header \"{name}\" names binary metadata, which a text header cannot carry.");
            }

            if (!seen.Add(name))
            {
                throw new InvalidRequestException($"Header \"{name}\" is given more than once, in different cases.");
            }

            foreach (var c in value)
            {
                if (c is < ' ' or > '~')
                {
                    throw new InvalidRequestException($"The value of header \"{name}\" holds a character that is not printable ASCII.");
                }
            }
        }
    }

    /// <summary>Whether the protocol controls a header, so a caller may not set it.</summary>
    /// <param name="name">The header's name, in any case.</param>
    /// <returns>Whether it is reserved.</returns>
    internal static bool IsReserved(string name)
    {
        if (Reserved.Contains(name))
        {
            return true;
        }

        foreach (var prefix in ReservedPrefixes)
        {
            if (name.StartsWith(prefix, StringComparison.OrdinalIgnoreCase))
            {
                return true;
            }
        }

        return false;
    }

    private static bool ContainsOnlyNameCharacters(this ReadOnlySpan<char> name)
    {
        foreach (var c in name)
        {
            if (!(char.IsAsciiLetterOrDigit(c) || c is '-' or '_' or '.'))
            {
                return false;
            }
        }

        return true;
    }
}
