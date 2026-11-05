using System;
using System.Collections.Frozen;
using System.Collections.Generic;
using System.IO;
using System.Text.Json;

namespace OpenMBEE.OpenSysML.Internal;

/// <summary>
/// The SHA-256 digests this checkout pins for release assets, by repository, release tag and
/// asset name: the embedded copy of client/release-digests.json.
/// </summary>
internal sealed class ReleaseDigests
{
    /// <summary>The manifest resource name of the embedded table.</summary>
    internal const string ResourceName = "OpenMBEE.OpenSysML.release-digests.json";

    private static readonly Lazy<ReleaseDigests> EmbeddedTable = new(LoadEmbedded);

    private readonly FrozenDictionary<string, FrozenDictionary<string, FrozenDictionary<string, string>>> table;

    private ReleaseDigests(FrozenDictionary<string, FrozenDictionary<string, FrozenDictionary<string, string>>> table)
    {
        this.table = table;
    }

    /// <summary>The table embedded in this assembly.</summary>
    /// <exception cref="InvalidDataException">The embedded table is missing or malformed.</exception>
    internal static ReleaseDigests Embedded => EmbeddedTable.Value;

    /// <summary>The number of pinned assets, over every repository and release.</summary>
    internal int Count { get; private init; }

    /// <summary>The digest pinned for one release asset.</summary>
    /// <param name="repository">The GitHub repository, as owner/repo; matched exactly.</param>
    /// <param name="tag">The release tag, resolved (never "latest"); matched exactly.</param>
    /// <param name="asset">The asset name, for example sysml-grpc-linux-amd64; matched exactly.</param>
    /// <returns>The lowercase hexadecimal SHA-256 digest, or null when nothing is pinned for the asset.</returns>
    internal string? Pinned(string repository, string tag, string asset)
    {
        ArgumentNullException.ThrowIfNull(repository);
        ArgumentNullException.ThrowIfNull(tag);
        ArgumentNullException.ThrowIfNull(asset);
        return table.TryGetValue(repository, out var releases)
            && releases.TryGetValue(tag, out var assets)
            && assets.TryGetValue(asset, out var digest)
            ? digest
            : null;
    }

    /// <summary>Reads a table in the format of client/release-digests.json.</summary>
    /// <param name="utf8Json">The table as UTF-8 JSON.</param>
    /// <returns>The table.</returns>
    /// <exception cref="InvalidDataException">
    /// The JSON is malformed, repeats a name, is not an object of repositories of releases of
    /// assets, or holds a digest that is not 64 lowercase hexadecimal digits.
    /// </exception>
    internal static ReleaseDigests Parse(ReadOnlyMemory<byte> utf8Json)
    {
        JsonDocument document;
        try
        {
            document = JsonDocument.Parse(utf8Json, new JsonDocumentOptions { AllowDuplicateProperties = false });
        }
        catch (JsonException e)
        {
            throw new InvalidDataException($"The release digest table is not valid JSON: {e.Message}", e);
        }

        using (document)
        {
            var count = 0;
            var repositories = new Dictionary<string, FrozenDictionary<string, FrozenDictionary<string, string>>>(StringComparer.Ordinal);
            foreach (var repository in Members(document.RootElement, "the table"))
            {
                var releases = new Dictionary<string, FrozenDictionary<string, string>>(StringComparer.Ordinal);
                foreach (var release in Members(repository.Value, $"repository {repository.Name}"))
                {
                    var assets = new Dictionary<string, string>(StringComparer.Ordinal);
                    foreach (var asset in Members(release.Value, $"release {repository.Name} {release.Name}"))
                    {
                        assets.Add(asset.Name, Digest(asset, repository.Name, release.Name));
                        count++;
                    }

                    releases.Add(release.Name, assets.ToFrozenDictionary(StringComparer.Ordinal));
                }

                repositories.Add(repository.Name, releases.ToFrozenDictionary(StringComparer.Ordinal));
            }

            return new ReleaseDigests(repositories.ToFrozenDictionary(StringComparer.Ordinal)) { Count = count };
        }
    }

    private static ReleaseDigests LoadEmbedded()
    {
        using var stream = typeof(ReleaseDigests).Assembly.GetManifestResourceStream(ResourceName)
            ?? throw new InvalidDataException($"The assembly embeds no {ResourceName}.");
        using var buffer = new MemoryStream();
        stream.CopyTo(buffer);
        return Parse(buffer.ToArray());
    }

    private static IEnumerable<JsonProperty> Members(JsonElement element, string what)
    {
        if (element.ValueKind != JsonValueKind.Object)
        {
            throw new InvalidDataException($"In the release digest table, {what} is a JSON {element.ValueKind}, not an object.");
        }

        foreach (var member in element.EnumerateObject())
        {
            if (member.Name.Length == 0)
            {
                throw new InvalidDataException($"In the release digest table, {what} has an empty name.");
            }

            yield return member;
        }
    }

    private static string Digest(JsonProperty asset, string repository, string release)
    {
        var digest = asset.Value.ValueKind == JsonValueKind.String ? asset.Value.GetString() : null;
        if (digest is null || digest.Length != 64 || !IsLowercaseHex(digest))
        {
            throw new InvalidDataException(
                $"In the release digest table, {repository} {release} {asset.Name} is not a SHA-256 digest of 64 lowercase hexadecimal digits.");
        }

        return digest;
    }

    private static bool IsLowercaseHex(string value)
    {
        foreach (var c in value)
        {
            if (!char.IsAsciiDigit(c) && c is not (>= 'a' and <= 'f'))
            {
                return false;
            }
        }

        return true;
    }
}
