using System;
using System.IO;
using System.Linq;
using System.Text;
using System.Text.Json;
using OpenMBEE.OpenSysML.Internal;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

[Trait(Category.Name, Category.Unit)]
public sealed class ReleaseDigestsTests
{
    private const string Repo = "Open-MBEE/OpenSysML";
    private const string Digest = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef";

    [Fact]
    public void EmbeddedCopyIsTheRepositoryTable()
    {
        using var stream = typeof(ReleaseDigests).Assembly.GetManifestResourceStream(ReleaseDigests.ResourceName);
        Assert.NotNull(stream);
        using var embedded = new MemoryStream();
        stream.CopyTo(embedded);
        var table = File.ReadAllBytes(Repository.PathOf("client", "release-digests.json"));

        Assert.True(
            embedded.ToArray().AsSpan().SequenceEqual(table),
            "The embedded release-digests.json is not client/release-digests.json; run python3 scripts/sync-release-digests.py.");
    }

    [Fact]
    public void EmbeddedTablePinsEveryAssetOfTheRepositoryTable()
    {
        using var table = JsonDocument.Parse(File.ReadAllBytes(Repository.PathOf("client", "release-digests.json")));
        var expected = 0;
        foreach (var repository in table.RootElement.EnumerateObject())
        {
            foreach (var release in repository.Value.EnumerateObject())
            {
                foreach (var asset in release.Value.EnumerateObject())
                {
                    Assert.Equal(asset.Value.GetString(), ReleaseDigests.Embedded.Pinned(repository.Name, release.Name, asset.Name));
                    expected++;
                }
            }
        }

        Assert.True(expected > 0, "client/release-digests.json pins nothing.");
        Assert.Equal(expected, ReleaseDigests.Embedded.Count);
    }

    [Fact]
    public void LookupsAreExact()
    {
        var digests = Parse($$"""{"{{Repo}}": {"v1.0.0": {"sysml-grpc-linux-amd64": "{{Digest}}" } } }""");

        Assert.Equal(Digest, digests.Pinned(Repo, "v1.0.0", "sysml-grpc-linux-amd64"));
        Assert.Null(digests.Pinned("open-mbee/opensysml", "v1.0.0", "sysml-grpc-linux-amd64"));
        Assert.Null(digests.Pinned(Repo, "1.0.0", "sysml-grpc-linux-amd64"));
        Assert.Null(digests.Pinned(Repo, "v1.0.0", "sysml-grpc-linux-arm64"));
        Assert.Null(digests.Pinned("other/repo", "v1.0.0", "sysml-grpc-linux-amd64"));
    }

    [Fact]
    public void AnEmptyTablePinsNothing()
    {
        var digests = Parse("{}");

        Assert.Equal(0, digests.Count);
        Assert.Null(digests.Pinned(Repo, "v1.0.0", "sysml-grpc-linux-amd64"));
    }

    [Fact]
    public void LookupRejectsNullArguments()
    {
        var digests = Parse("{}");

        Assert.Throws<ArgumentNullException>(() => digests.Pinned(null!, "v1.0.0", "a"));
        Assert.Throws<ArgumentNullException>(() => digests.Pinned(Repo, null!, "a"));
        Assert.Throws<ArgumentNullException>(() => digests.Pinned(Repo, "v1.0.0", null!));
    }

    [Theory]
    [InlineData("{", "not valid JSON")]
    [InlineData("[]", "the table is a JSON Array")]
    [InlineData("""{"r/r": []}""", "repository r/r is a JSON Array")]
    [InlineData("""{"r/r": {"v1": "x"}}""", "release r/r v1 is a JSON String")]
    [InlineData("""{"": {}}""", "the table has an empty name")]
    [InlineData("""{"r/r": {"v1": {"": "DIGEST"}}}""", "release r/r v1 has an empty name")]
    [InlineData("""{"r/r": {"v1": {"a": 1}}}""", "r/r v1 a is not a SHA-256 digest")]
    [InlineData("""{"r/r": {"v1": {"a": null}}}""", "r/r v1 a is not a SHA-256 digest")]
    [InlineData("""{"r/r": {"v1": {"a": "0123"}}}""", "r/r v1 a is not a SHA-256 digest")]
    [InlineData("""{"r/r": {"v1": {"a": "DIGEST0"}}}""", "r/r v1 a is not a SHA-256 digest")]
    [InlineData("""{"r/r": {"v1": {"a": "UPPER"}}}""", "r/r v1 a is not a SHA-256 digest")]
    [InlineData("""{"r/r": {"v1": {"a": "NONHEX"}}}""", "r/r v1 a is not a SHA-256 digest")]
    [InlineData("""{"r/r": {"v1": {"a": "DIGEST", "a": "DIGEST"}}}""", "not valid JSON")]
    [InlineData("""{"r/r": {"v1": {}}, "r/r": {}}""", "not valid JSON")]
    public void MalformedTablesAreRejected(string json, string message)
    {
        var table = json
            .Replace("NONHEX", Digest[..^1] + "g", StringComparison.Ordinal)
            .Replace("UPPER", Digest.ToUpperInvariant(), StringComparison.Ordinal)
            .Replace("DIGEST", Digest, StringComparison.Ordinal);

        var error = Assert.Throws<InvalidDataException>(() => Parse(table));
        Assert.Contains(message, error.Message, StringComparison.Ordinal);
    }

    private static ReleaseDigests Parse(string json) => ReleaseDigests.Parse(Encoding.UTF8.GetBytes(json));
}
