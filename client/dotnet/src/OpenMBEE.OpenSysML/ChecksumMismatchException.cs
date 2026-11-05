using System;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// A <c>sysml-grpc</c> binary does not have the SHA-256 digest required of it, so it was not
/// started. Nothing else is tried in its place.
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

    /// <summary>The binary that was hashed.</summary>
    public string BinaryPath { get; }

    /// <summary>The digest required, in lower-case hexadecimal.</summary>
    public string ExpectedSha256 { get; }

    /// <summary>The digest it has, in lower-case hexadecimal.</summary>
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
