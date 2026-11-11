using System;
using System.Net.Http;

namespace OpenMBEE.OpenSysML.Internal;

/// <summary>Reads the chain of causes an exception holds, as both transports classify failures by it.</summary>
internal static class Causes
{
    /// <summary>The first exception of a type in a chain of causes, starting with the exception itself.</summary>
    /// <typeparam name="T">The type sought.</typeparam>
    /// <param name="error">The exception, or null.</param>
    /// <returns>The exception found, or null.</returns>
    internal static T? Find<T>(Exception? error)
        where T : Exception
    {
        for (; error is not null; error = error.InnerException)
        {
            if (error is T found)
            {
                return found;
            }
        }

        return null;
    }

    /// <summary>The last cause of a chain: the failure everything else wraps.</summary>
    /// <param name="error">The exception.</param>
    /// <returns>Its innermost cause, or the exception itself when it has none.</returns>
    internal static Exception Innermost(Exception error)
    {
        while (error.InnerException is not null)
        {
            error = error.InnerException;
        }

        return error;
    }

    /// <summary>What went wrong with an HTTP request in a chain of causes, as its first request or response-body failure says.</summary>
    /// <param name="error">The exception, or null.</param>
    /// <returns>The first <see cref="HttpRequestException"/>'s or <see cref="HttpIOException"/>'s error, or null when there is none.</returns>
    internal static HttpRequestError? RequestError(Exception? error)
    {
        for (; error is not null; error = error.InnerException)
        {
            switch (error)
            {
                case HttpRequestException request:
                    return request.HttpRequestError;
                case HttpIOException io:
                    return io.HttpRequestError;
            }
        }

        return null;
    }

    /// <summary>
    /// Whether the endpoint could not be reached at all: its name did not resolve, the connection or its TLS handshake
    /// failed, or a proxy would not tunnel to it. gRPC calls that <see cref="StatusCode.Unavailable"/>, whatever an HTTP
    /// library says, and so does this client over either protocol.
    /// </summary>
    /// <param name="error">What went wrong with the request, if anything.</param>
    /// <returns>Whether the endpoint was unreachable.</returns>
    internal static bool Unreachable(HttpRequestError? error) =>
        error is HttpRequestError.NameResolutionError or HttpRequestError.ConnectionError
            or HttpRequestError.SecureConnectionError or HttpRequestError.ProxyTunnelError;
}
