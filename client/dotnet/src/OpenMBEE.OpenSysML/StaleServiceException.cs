using System;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// The service reached is not the version <see cref="ConnectionOptions.Version"/> asked for. A
/// service this client did not start is reported, never stopped.
/// </summary>
public class StaleServiceException : ServiceStartException
{
    /// <summary>Creates the exception.</summary>
    /// <param name="address">The address of the service reached.</param>
    /// <param name="reason">How it differs from the one asked for.</param>
    /// <param name="remedy">What to do about it.</param>
    /// <param name="serviceVersion">The version it reported; empty when it did not answer GetServerInfo.</param>
    public StaleServiceException(string address, string reason, string remedy, string serviceVersion)
        : base(Describe(address, reason, remedy, serviceVersion))
    {
        Address = address;
        Reason = reason;
        Remedy = remedy;
        ServiceVersion = serviceVersion;
    }

    /// <summary>The address of the service reached.</summary>
    public string Address { get; }

    /// <summary>How it differs from the one asked for.</summary>
    public string Reason { get; }

    /// <summary>What to do about it.</summary>
    public string Remedy { get; }

    /// <summary>The version it reported; empty when it did not answer GetServerInfo.</summary>
    public string ServiceVersion { get; }

    private static string Describe(string address, string reason, string remedy, string serviceVersion)
    {
        ArgumentNullException.ThrowIfNull(address);
        ArgumentNullException.ThrowIfNull(reason);
        ArgumentNullException.ThrowIfNull(remedy);
        ArgumentNullException.ThrowIfNull(serviceVersion);
        var version = serviceVersion.Length == 0 ? "(version unknown)" : serviceVersion;
        return $"The sysml-grpc service at {address} is not the one asked for: {reason}.\n"
            + $"  service: sysml-grpc {version} at {address}\n"
            + $"  fix:     {remedy}";
    }
}
