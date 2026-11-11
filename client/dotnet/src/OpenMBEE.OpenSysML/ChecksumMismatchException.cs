using System;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// A <c>sysml-grpc</c> binary does not have the SHA-256 digest required of it, so it was not
/// started. Nothing else is tried in its place. An <see cref="UnpinnedReleaseException"/> is one too: a release
/// nothing this client trusts vouches for, which has no digest to compare.
/// </summary>
public class ChecksumMismatchException : ServiceStartException
{
    /// <summary>Creates the exception.</summary>
    /// <param name="binaryPath">The binary that was hashed.</param>
    /// <param name="expectedSha256">The digest required, in lower-case hexadecimal.</param>
    /// <param name="actualSha256">The digest it has, in lower-case hexadecimal.</param>
    /// <param name="remedy">What to do about it.</param>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    public ChecksumMismatchException(string binaryPath, string expectedSha256, string actualSha256, string remedy)
        : base(Describe(binaryPath, expectedSha256, actualSha256, remedy))
    {
        BinaryPath = binaryPath;
        ExpectedSha256 = expectedSha256;
        ActualSha256 = actualSha256;
    }

    /// <summary>Creates the exception for a release that has no digest to compare, which hashed nothing.</summary>
    /// <param name="message">Why the release was refused and what to do about it.</param>
    /// <exception cref="ArgumentNullException"><paramref name="message"/> is null.</exception>
    protected ChecksumMismatchException(string message)
        : base(message ?? throw new ArgumentNullException(nameof(message)))
    {
        BinaryPath = string.Empty;
        ExpectedSha256 = string.Empty;
        ActualSha256 = string.Empty;
    }

    /// <summary>The binary that was hashed; empty when nothing was.</summary>
    public string BinaryPath { get; }

    /// <summary>The digest required, in lower-case hexadecimal; empty when nothing was hashed.</summary>
    public string ExpectedSha256 { get; }

    /// <summary>The digest it has, in lower-case hexadecimal; empty when nothing was hashed.</summary>
    public string ActualSha256 { get; }

    private static string Describe(string binaryPath, string expectedSha256, string actualSha256, string remedy)
    {
        ArgumentNullException.ThrowIfNull(binaryPath);
        ArgumentNullException.ThrowIfNull(expectedSha256);
        ArgumentNullException.ThrowIfNull(actualSha256);
        ArgumentNullException.ThrowIfNull(remedy);
        return $"sysml-grpc at {binaryPath} has SHA-256 {actualSha256}, but {expectedSha256} was required; it was not started.\n"
            + $"  fix: {remedy}";
    }
}
