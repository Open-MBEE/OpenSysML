using System;
using System.IO;
using System.Security.Cryptography;
using System.Text.RegularExpressions;

namespace OpenMBEE.OpenSysML.Internal;

/// <summary>
/// The files of the shared binary cache every OpenSysML client installs into, named exactly as the
/// Python, Java and Rust clients name them: the executable, its <c>.json</c> record of the release it
/// is, the <c>.lock</c> file every client locks while it changes them, and the executable linked under
/// its own digest. The <c>.dotnet</c> guard beside the lock is this client's alone.
/// </summary>
internal sealed partial class CacheLayout
{
    /// <summary>How many hexadecimal digits of the digest name a snapshot, as every client names it.</summary>
    internal const int SnapshotDigits = 16;

    /// <summary>Creates the layout of a cache directory.</summary>
    /// <param name="directory">The directory; made absolute.</param>
    internal CacheLayout(string directory)
    {
        ArgumentException.ThrowIfNullOrEmpty(directory);
        Directory = Path.GetFullPath(directory);
    }

    /// <summary>The cache directory.</summary>
    internal string Directory { get; }

    /// <summary>The cached executable, which every client replaces in place.</summary>
    internal string Executable => Path.Combine(Directory, BinaryResolver.ExecutableName);

    /// <summary>The record of which release of which repository the executable is.</summary>
    internal string Metadata => Executable + ".json";

    /// <summary>The file every client locks while it changes the cache.</summary>
    internal string Lock => Executable + ".lock";

    /// <summary>The file copies of this client lock before <see cref="Lock"/>, since its lock belongs to the process.</summary>
    internal string Guard => Lock + ".dotnet";

    /// <summary>The cache in the home directory, <c>~/.opensysml/bin</c>; null when no home is set.</summary>
    /// <param name="environment">Reads an environment variable; null when it is unset.</param>
    /// <returns>The layout, or null.</returns>
    internal static CacheLayout? InHome(Func<string, string?> environment)
    {
        var home = environment(BinaryResolver.HomeVariable);
        return string.IsNullOrWhiteSpace(home) ? null : new CacheLayout(Path.Combine(home, ".opensysml", "bin"));
    }

    /// <summary>Where the executable with a digest is linked under that digest, so no install over the cache changes it.</summary>
    /// <param name="sha256">The executable's digest, in hexadecimal.</param>
    /// <returns>The path, <c>sysml-grpc-0123456789abcdef</c> with <c>.exe</c> on Windows.</returns>
    internal string Snapshot(string sha256)
    {
        if (sha256.Length < SnapshotDigits || !IsHex(sha256))
        {
            throw new ArgumentException($"{sha256} is not a hexadecimal digest.", nameof(sha256));
        }

        var name = BinaryResolver.ExecutableName;
        var extension = name.EndsWith(".exe", StringComparison.Ordinal) ? ".exe" : string.Empty;
        return Path.Combine(Directory, $"{name[..^extension.Length]}-{sha256[..SnapshotDigits].ToLowerInvariant()}{extension}");
    }

    /// <summary>A new name to stage a file under beside where it is published, which no other file has.</summary>
    /// <param name="final">The path the file is published at.</param>
    /// <returns>The temporary path, in the same directory.</returns>
    internal static string Temporary(string final) =>
        $"{final}.dotnet-{Convert.ToHexStringLower(RandomNumberGenerator.GetBytes(8))}.tmp";

    /// <summary>Whether a file name is one <see cref="Temporary"/> gives a cache file.</summary>
    /// <param name="name">The file name, without its directory.</param>
    /// <returns>Whether it is.</returns>
    internal static bool IsTemporary(string name) => TemporaryName().IsMatch(name);

    /// <summary>Whether text is hexadecimal digits.</summary>
    /// <param name="text">The text.</param>
    /// <returns>Whether every character is a hexadecimal digit.</returns>
    internal static bool IsHex(string text)
    {
        foreach (var c in text)
        {
            if (!char.IsAsciiHexDigit(c))
            {
                return false;
            }
        }

        return text.Length > 0;
    }

    [GeneratedRegex(@"^sysml-grpc[^/\\]*\.dotnet-[0-9a-f]{16}\.tmp$", RegexOptions.CultureInvariant)]
    private static partial Regex TemporaryName();
}
