using System;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// A release of <c>sysml-grpc</c> was not downloaded because nothing this client trusts vouches for it: this
/// checkout pins no SHA-256 digest for the asset, and this client does not verify the signature on a release's
/// checksum manifest, so the only digest left is the one the release serves beside the binary, which a
/// compromised release would serve too. Ask for a release this checkout pins, or accept that same-origin trust
/// for the repository with <see cref="ConnectionOptions.AllowUnpinnedDownloadVariable"/>.
/// </summary>
public class UnpinnedReleaseException : ServiceStartException
{
    /// <summary>Creates the exception.</summary>
    /// <param name="repository">The repository, <c>owner/repo</c>.</param>
    /// <param name="version">The release tag.</param>
    /// <param name="asset">The asset, as <c>sysml-grpc-linux-amd64</c>.</param>
    /// <param name="message">Why the release was refused and what to do about it.</param>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    public UnpinnedReleaseException(string repository, string version, string asset, string message)
        : base(message)
    {
        ArgumentNullException.ThrowIfNull(repository);
        ArgumentNullException.ThrowIfNull(version);
        ArgumentNullException.ThrowIfNull(asset);
        Repository = repository;
        Version = version;
        Asset = asset;
    }

    /// <summary>The repository, <c>owner/repo</c>.</summary>
    public string Repository { get; }

    /// <summary>The release tag.</summary>
    public string Version { get; }

    /// <summary>The asset, as <c>sysml-grpc-linux-amd64</c>.</summary>
    public string Asset { get; }
}
