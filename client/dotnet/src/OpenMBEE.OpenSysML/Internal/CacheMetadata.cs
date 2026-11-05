using System;
using System.Globalization;
using System.IO;
using System.Text;
using System.Text.Json;

namespace OpenMBEE.OpenSysML.Internal;

/// <summary>
/// The record every client keeps beside the cached executable of which release of which repository it is:
/// <c>{"version": "v1.2.3", "sha256": "…", "repo": "owner/repo"}</c>, written as Python's <c>json.dump</c>
/// writes it. It is advisory: it vouches for the executable only while <see cref="Sha256"/> is the
/// executable's digest, which a reader checks, since the executable and the record are published one after
/// the other and another client may write the record without a temporary file.
/// </summary>
/// <param name="Version">The release tag.</param>
/// <param name="Sha256">The executable's digest when the record was written, in hexadecimal.</param>
/// <param name="Repo">The repository, <c>owner/repo</c>; null in a record that predates one being kept.</param>
internal sealed record CacheMetadata(string Version, string Sha256, string? Repo)
{
    /// <summary>The most of a record that is read; a longer file is no record any client wrote.</summary>
    internal const int MaxBytes = 64 * 1024;

    /// <summary>The problem <see cref="Read"/> reports when there is no record at all.</summary>
    internal const string Absent = "absent";

    /// <summary>Reads a record, saying what is wrong with it when it is not one.</summary>
    /// <param name="path">The record's path.</param>
    /// <returns>The record, or why there is none.</returns>
    internal static CacheMetadataRead Read(string path)
    {
        byte[] bytes;
        try
        {
            if (!File.Exists(path))
            {
                return new(null, Directory.Exists(path) ? "a directory" : Absent);
            }

            using var file = new FileStream(path, FileMode.Open, FileAccess.Read, FileShare.ReadWrite | FileShare.Delete);
            bytes = new byte[MaxBytes + 1];
            var length = file.ReadAtLeast(bytes, bytes.Length, throwOnEndOfStream: false);
            if (length > MaxBytes)
            {
                return new(null, $"larger than {MaxBytes / 1024} KiB");
            }

            Array.Resize(ref bytes, length);
        }
        catch (Exception error) when (error is IOException or UnauthorizedAccessException)
        {
            return new(null, $"unreadable ({error.Message})");
        }

        return Parse(bytes);
    }

    /// <summary>Reads a record from its bytes, as the other clients read it: the last of a repeated member counts.</summary>
    /// <param name="bytes">The record's bytes.</param>
    /// <returns>The record, or why there is none.</returns>
    internal static CacheMetadataRead Parse(ReadOnlyMemory<byte> bytes)
    {
        try
        {
            using var document = JsonDocument.Parse(bytes);
            var root = document.RootElement;

            if (root.ValueKind != JsonValueKind.Object)
            {
                return new(null, $"a JSON {root.ValueKind.ToString().ToLowerInvariant()}, not an object");
            }

            JsonElement? version = null, sha256 = null, repo = null;
            foreach (var member in root.EnumerateObject())
            {
                switch (member.Name)
                {
                    case "version":
                        version = member.Value;
                        break;
                    case "sha256":
                        sha256 = member.Value;
                        break;
                    case "repo":
                        repo = member.Value;
                        break;
                }
            }

            if (Text(version) is not { Length: > 0 } recordedVersion)
            {
                return new(null, "without a version");
            }

            if (Text(sha256) is not { } recordedSha256 || recordedSha256.Length != 64 || !CacheLayout.IsHex(recordedSha256))
            {
                return new(null, "without a SHA-256 digest");
            }

            if (repo is { ValueKind: not JsonValueKind.String and not JsonValueKind.Null })
            {
                return new(null, "with a repository that is not a string");
            }

            return new(new CacheMetadata(recordedVersion, recordedSha256, Text(repo) is { Length: > 0 } recordedRepo ? recordedRepo : null), null);
        }
        // JSON string decoding is lazy and may reject invalid Unicode after the document parses.
        catch (Exception error) when (error is JsonException or InvalidOperationException)
        {
            return new(null, $"not JSON ({error.Message})");
        }
    }

    /// <summary>Whether the record vouches for an executable with a digest.</summary>
    /// <param name="sha256">The executable's digest.</param>
    /// <returns>Whether it is the digest recorded, in either case.</returns>
    internal bool Describes(string sha256) => string.Equals(Sha256, sha256, StringComparison.OrdinalIgnoreCase);

    /// <summary>The record as Python's <c>json.dump</c> writes it, ASCII alone; <see cref="Repo"/> must be set.</summary>
    /// <returns>The bytes to write.</returns>
    internal byte[] Format()
    {
        if (Repo is null)
        {
            throw new InvalidOperationException("A record is written with its repository.");
        }

        var text = new StringBuilder("{\"version\": ");
        Quote(text, Version);
        text.Append(", \"sha256\": ");
        Quote(text, Sha256);
        text.Append(", \"repo\": ");
        Quote(text, Repo);
        text.Append('}');
        return Encoding.ASCII.GetBytes(text.ToString());
    }

    private static string? Text(JsonElement? element) => element is { ValueKind: JsonValueKind.String } value ? value.GetString() : null;

    // json.dumps with ensure_ascii: printable ASCII as it is, the usual short escapes, and \u with
    // lower-case digits for everything else, a character outside the BMP as its two UTF-16 halves.
    private static void Quote(StringBuilder text, string value)
    {
        text.Append('"');
        foreach (var c in value)
        {
            switch (c)
            {
                case '"':
                    text.Append("\\\"");
                    break;
                case '\\':
                    text.Append("\\\\");
                    break;
                case '\n':
                    text.Append("\\n");
                    break;
                case '\r':
                    text.Append("\\r");
                    break;
                case '\t':
                    text.Append("\\t");
                    break;
                case '\b':
                    text.Append("\\b");
                    break;
                case '\f':
                    text.Append("\\f");
                    break;
                case >= ' ' and <= '~':
                    text.Append(c);
                    break;
                default:
                    text.Append("\\u").Append(((int)c).ToString("x4", CultureInfo.InvariantCulture));
                    break;
            }
        }

        text.Append('"');
    }
}

/// <summary>What reading a <see cref="CacheMetadata"/> record found.</summary>
/// <param name="Metadata">The record, when there is one.</param>
/// <param name="Problem">Why there is none, when there is not: absent, unreadable or malformed.</param>
internal readonly record struct CacheMetadataRead(CacheMetadata? Metadata, string? Problem);
