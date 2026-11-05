using System;

namespace OpenMBEE.OpenSysML.Internal;

/// <summary>
/// What makes two private services interchangeable: the same file with the same contents, started
/// and stopped under the same limits, and required to be the same version. Connections whose options
/// agree on these share one service; protocol, encoding, headers, logger, request timeout and HTTP
/// handler belong to each connection and do not split it.
/// </summary>
/// <param name="Path">The binary's canonical path.</param>
/// <param name="Sha256">The SHA-256 digest of its contents.</param>
/// <param name="StartupTimeout">How long it has to report its address.</param>
/// <param name="ShutdownTimeout">How long it has to exit once asked to.</param>
/// <param name="Version">The version every connection to it requires, or null.</param>
internal sealed record ServiceIdentity(string Path, string Sha256, TimeSpan StartupTimeout, TimeSpan ShutdownTimeout, string? Version)
{
    /// <summary>The identity a connection's options and resolved binary describe.</summary>
    /// <param name="binary">The binary to start.</param>
    /// <param name="options">The connection's options.</param>
    /// <returns>The identity.</returns>
    internal static ServiceIdentity Of(ResolvedBinary binary, ConnectionOptions options) =>
        new(binary.Path, binary.Sha256, options.StartupTimeout, options.ShutdownTimeout, options.Version);
}
