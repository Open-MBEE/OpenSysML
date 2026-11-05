using System;

namespace OpenMBEE.OpenSysML;

/// <summary>What a service said of itself when a connection opened, through GetServerInfo.</summary>
public sealed class ServerInfo : IEquatable<ServerInfo>
{
    /// <summary>Creates a description of a service.</summary>
    /// <param name="version">The build version it reported; empty when it did not answer.</param>
    /// <param name="capabilities">The capabilities it advertised.</param>
    /// <param name="answered">Whether it answered GetServerInfo.</param>
    public ServerInfo(string version, Capabilities capabilities, bool answered)
    {
        ArgumentNullException.ThrowIfNull(version);
        ArgumentNullException.ThrowIfNull(capabilities);
        Version = version;
        Capabilities = capabilities;
        Answered = answered;
    }

    /// <summary>
    /// The build version of the service, informational only: <c>dev</c> for a build without release
    /// stamping, not comparable across forks, and empty when the service did not answer.
    /// </summary>
    public string Version { get; }

    /// <summary>The capabilities the service advertised.</summary>
    public Capabilities Capabilities { get; }

    /// <summary>
    /// Whether the service answered GetServerInfo. A service too old to have the method refuses it
    /// with <see cref="StatusCode.Unimplemented"/>, which is itself an answer: it claims nothing.
    /// </summary>
    public bool Answered { get; }

    /// <inheritdoc/>
    public bool Equals(ServerInfo? other) =>
        other is not null
        && Answered == other.Answered
        && string.Equals(Version, other.Version, StringComparison.Ordinal)
        && Capabilities.Equals(other.Capabilities);

    /// <inheritdoc/>
    public override bool Equals(object? obj) => Equals(obj as ServerInfo);

    /// <inheritdoc/>
    public override int GetHashCode() => HashCode.Combine(Answered, StringComparer.Ordinal.GetHashCode(Version), Capabilities);

    /// <summary>The version and the number of capabilities.</summary>
    /// <returns>As <c>sysml-grpc v1.2.3, 72 capabilities</c>.</returns>
    public override string ToString() =>
        Answered
            ? $"sysml-grpc {(Version.Length == 0 ? "(version unknown)" : Version)}, {Capabilities.Names.Count} capabilities"
            : "sysml-grpc (did not answer GetServerInfo)";
}
