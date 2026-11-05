using System;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// A private <c>sysml-grpc</c> cannot be provided on this platform: a browser, for instance, cannot
/// start a process. Name a running service with <see cref="ConnectionOptions.Address"/> instead.
/// </summary>
public class UnsupportedPlatformException : ServiceStartException
{
    /// <summary>Creates the exception.</summary>
    /// <param name="platform">The platform, as <c>browser</c>.</param>
    /// <param name="message">What cannot be done there, and what to do instead.</param>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    public UnsupportedPlatformException(string platform, string message)
        : base(message)
    {
        ArgumentNullException.ThrowIfNull(platform);
        Platform = platform;
    }

    /// <summary>The platform, as <c>browser</c>.</summary>
    public string Platform { get; }
}
