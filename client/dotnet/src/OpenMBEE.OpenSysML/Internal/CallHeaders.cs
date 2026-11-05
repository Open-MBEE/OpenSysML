using System;
using System.Collections.Generic;
using System.Linq;

namespace OpenMBEE.OpenSysML.Internal;

/// <summary>Where the credentials a connection's calls carry come from.</summary>
internal enum CredentialSource
{
    /// <summary>The calls carry none.</summary>
    None,

    /// <summary><see cref="ConnectionOptions.Token"/>.</summary>
    Token,

    /// <summary><see cref="ConnectionOptions.TokenVariable"/>.</summary>
    TokenVariable,

    /// <summary>An <c>authorization</c> entry of <see cref="ConnectionOptions.Headers"/>.</summary>
    Header,
}

/// <summary>
/// The headers every call of a connection carries, resolved once when it opens: the caller's
/// <see cref="ConnectionOptions.Headers"/>, in lower case, and the bearer token, from
/// <see cref="ConnectionOptions.Token"/>, else from <see cref="ConnectionOptions.TokenVariable"/> unless the
/// caller's headers already carry an <c>authorization</c> of their own.
/// </summary>
internal sealed class CallHeaders
{
    private CallHeaders(IReadOnlyList<KeyValuePair<string, string>> entries, CredentialSource credentials)
    {
        Entries = entries;
        Credentials = credentials;
    }

    /// <summary>No headers and no credentials.</summary>
    internal static CallHeaders None { get; } = new([], CredentialSource.None);

    /// <summary>The headers, names in lower case, in the order the caller gave them and the token last.</summary>
    internal IReadOnlyList<KeyValuePair<string, string>> Entries { get; }

    /// <summary>Where the credentials the calls carry come from.</summary>
    internal CredentialSource Credentials { get; }

    /// <summary>Resolves the headers of a connection's calls.</summary>
    /// <param name="options">The connection's options, already validated.</param>
    /// <param name="environment">Reads an environment variable; null when it is unset.</param>
    /// <returns>The headers.</returns>
    /// <exception cref="InvalidRequestException"><see cref="ConnectionOptions.TokenVariable"/> names something that is not a bearer token.</exception>
    internal static CallHeaders Resolve(ConnectionOptions options, Func<string, string?> environment)
    {
        List<KeyValuePair<string, string>> entries = [.. options.Headers.Select(header => KeyValuePair.Create(header.Key.ToLowerInvariant(), header.Value))];
        var credentials = entries.Any(entry => entry.Key == BearerToken.Header) ? CredentialSource.Header : CredentialSource.None;
        string? token = null;
        if (options.Token is not null)
        {
            (token, credentials) = (options.Token, CredentialSource.Token);
        }
        else if (credentials == CredentialSource.None && environment(ConnectionOptions.TokenVariable)?.Trim() is { Length: > 0 } named)
        {
            if (!BearerToken.IsValid(named))
            {
                throw new InvalidRequestException(
                    $"${ConnectionOptions.TokenVariable} does not name a bearer token: {BearerToken.Rule}. Correct it, or unset it if no service needs one.");
            }

            (token, credentials) = (named, CredentialSource.TokenVariable);
        }

        if (token is not null)
        {
            entries.Add(KeyValuePair.Create(BearerToken.Header, BearerToken.HeaderValue(token)));
        }

        return entries.Count == 0 ? None : new CallHeaders(entries, credentials);
    }
}
