using System;
using System.IO;
using System.Text;
using System.Threading;
using System.Threading.Tasks;
using Microsoft.Extensions.Logging;
using OpenMBEE.OpenSysML.Internal;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// The offline signed-release fixtures the Python client's verifier is tested against
/// (client/python/tests/fixtures/signed_release, made by client/python/scripts/make_signed_release_fixture.py),
/// served as a release this client pins nothing for. This client verifies pins alone: it does not check the
/// signature on a release's SHA256SUMS.txt, so whatever the signature is, valid, valid only historically, by
/// another identity, over a changed manifest, or absent, the release is refused as unpinned, and is never taken
/// for verified because a signature was there.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed class SignedReleaseTests : IAsyncLifetime
{
    private const string Tag = "v9.9.9";
    private const string Manifest = "SHA256SUMS.txt";
    private const string Bundle = "SHA256SUMS.txt.bundle";

    private readonly ReleaseServer server = new();
    private readonly TestCache cache = new();
    private readonly ListLogger logger = new();

    private static CancellationToken Token => TestContext.Current.CancellationToken;

    private static byte[] Binary => Fixture(ReleaseServer.Asset);

    [Theory]
    [InlineData("SHA256SUMS.txt.bundle", false)]
    [InlineData("SHA256SUMS.txt.expired.bundle", false)]
    [InlineData("SHA256SUMS.txt.other-identity.bundle", false)]
    [InlineData("SHA256SUMS.txt.bundle", true)]
    [InlineData(null, false)]
    public async Task ASignedReleaseNothingPinsIsRefusedAsUnpinnedWhateverItsSignatureAndTheCacheIsLeftAlone(string? bundle, bool manifestChanged)
    {
        await cache.InstallAsync(Encoding.ASCII.GetBytes("a working release"), "v9.9.8");
        Publish(bundle, manifestChanged);

        var error = await Assert.ThrowsAsync<UnpinnedReleaseException>(() => Resolve(new ConnectionOptions { DownloadVersion = Tag }, server.Source()));

        Assert.Equal((ReleaseServer.Repo, Tag, ReleaseServer.Asset), (error.Repository, error.Version, error.Asset));
        Assert.Contains("it does not verify the signature on a release's SHA256SUMS.txt", error.Message, StringComparison.Ordinal);
        Assert.Empty(server.Requested);
        Assert.Equal("a working release", File.ReadAllText(cache.Layout.Executable));
        Assert.Equal("v9.9.8", CacheMetadata.Read(cache.Layout.Metadata).Metadata!.Version);
    }

    [Fact]
    public async Task ThePinIsWhatIsTrustedAndTheSignatureIsNeverFetched()
    {
        Publish(Bundle, manifestChanged: false);

        var resolved = await Resolve(new ConnectionOptions { DownloadVersion = Tag }, server.Source(ReleaseServer.Pinning(Tag, Binary)));

        Assert.Equal(TestCache.Sha256(Binary), resolved.Sha256);
        Assert.Equal("e225eea32ee248b90215a1a65c2287befcbd8e3a24692dbb5d8b98e6427ecd29", resolved.Sha256);
        Assert.Equal([ReleaseServer.AssetPath(Tag) + ".sha256", ReleaseServer.AssetPath(Tag)], server.Requested);
    }

    [Fact]
    public async Task SameOriginTrustTakesTheServedChecksumNotTheSignatureAndStillRefusesBytesThatDoNotMatchIt()
    {
        Publish("SHA256SUMS.txt.other-identity.bundle", manifestChanged: false);
        var environment = Environment(ReleaseServer.Repo);

        var resolved = await Resolve(new ConnectionOptions { DownloadVersion = Tag }, server.Source(), environment);

        Assert.Equal(TestCache.Sha256(Binary), resolved.Sha256);
        Assert.DoesNotContain(server.Requested, path => path.EndsWith(Manifest, StringComparison.Ordinal) || path.EndsWith(".bundle", StringComparison.Ordinal));
        Assert.Contains(logger.Entries, entry => entry.Level == LogLevel.Warning && entry.Message.StartsWith("This client pins no digest for", StringComparison.Ordinal));

        server.Serve(ReleaseServer.AssetPath("v9.9.10"), new ReleaseServer.Answer { Body = Encoding.ASCII.GetBytes("tampered") }).Served("v9.9.10", TestCache.Sha256(Binary));
        await Assert.ThrowsAsync<ChecksumMismatchException>(() => Resolve(new ConnectionOptions { DownloadVersion = "v9.9.10" }, server.Source(), environment));
        Assert.Equal(Binary, File.ReadAllBytes(cache.Layout.Executable));
    }

    public ValueTask InitializeAsync() => ValueTask.CompletedTask;

    public async ValueTask DisposeAsync()
    {
        await server.DisposeAsync();
        cache.Dispose();
    }

    private static byte[] Fixture(string name) => File.ReadAllBytes(Repository.PathOf("client", "python", "tests", "fixtures", "signed_release", name));

    // The release as the pipeline publishes it: the binary and its .sha256, the manifest and the bundle signing it.
    private void Publish(string? bundle, bool manifestChanged)
    {
        server.Publish(Tag, Binary);
        var manifest = Fixture(Manifest);
        server.Serve(ReleaseServer.AssetPath(Tag, Manifest), new ReleaseServer.Answer { Body = manifestChanged ? [.. manifest, .. "\n"u8] : manifest });
        if (bundle is not null)
        {
            server.Serve(ReleaseServer.AssetPath(Tag, Bundle), new ReleaseServer.Answer { Body = Fixture(bundle) });
        }
    }

    private Task<ResolvedBinary> Resolve(ConnectionOptions options, ReleaseSource source, Func<string, string?>? environment = null) =>
        BinaryResolver.ResolveAsync(new ConnectionOptions { DownloadVersion = options.DownloadVersion, Logger = logger }, environment ?? Environment(null), source, Token);

    private Func<string, string?> Environment(string? allowUnpinned) =>
        name => name switch
        {
            "HOME" or "USERPROFILE" => cache.Root,
            ConnectionOptions.AllowUnpinnedDownloadVariable => allowUnpinned,
            _ => null,
        };
}
