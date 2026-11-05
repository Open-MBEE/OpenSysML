using System;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Text;
using System.Threading;
using System.Threading.Tasks;
using Microsoft.Extensions.Logging;
using Microsoft.Extensions.Logging.Abstractions;
using OpenMBEE.OpenSysML.Internal;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// Fetching releases from a loopback release host: resolving <c>latest</c>, deciding the digest a release must
/// have from the pins alone, and streaming it into the cache within bounds of size and silence, over redirects
/// that never leave HTTPS.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed class BinaryDownloaderTests : IAsyncLifetime
{
    private const string Tag = "v1.2.3";
    private static readonly byte[] Release = Encoding.ASCII.GetBytes("#!/bin/sh\necho a release\n");

    private readonly ReleaseServer server = new();
    private readonly TestCache cache = new();
    private readonly ListLogger logger = new();

    private static CancellationToken Token => TestContext.Current.CancellationToken;

    private static string Repo => ReleaseServer.Repo;

    [Fact]
    public async Task LatestIsResolvedThroughTheReleasesApiAsThisClient()
    {
        server.Latest("v4.5.6");
        using var downloader = Downloader(server.Source());

        Assert.Equal("v4.5.6", await downloader.LatestAsync(Repo, Token));

        var request = Assert.Single(server.Headers);
        Assert.Equal(($"/repos/{Repo}/releases/latest", $"OpenMBEE.OpenSysML/{ReleaseRequest.ClientVersion}", "application/vnd.github+json"), request);
    }

    [Theory]
    [InlineData("{}", "answered no tag_name")]
    [InlineData("[]", "answered no tag_name")]
    [InlineData("{\"tag_name\": 5}", "answered no tag_name")]
    [InlineData("{\"tag_name\": \"\"}", "answered the tag \"\", which is not one a URL can carry")]
    [InlineData("{\"tag_name\": \"../../x\"}", "answered the tag \"../../x\", which is not one a URL can carry")]
    [InlineData("{\"tag_name\": \"\\uD800\"}", "answered something other than JSON")]
    [InlineData("{\"tag_name\": \"\\uDC00\"}", "answered something other than JSON")]
    [InlineData("<html>rate limited</html>", "answered something other than JSON")]
    public async Task ALatestReleaseWithoutAUsableTagIsAStartFailure(string body, string reason)
    {
        server.Serve($"/repos/{Repo}/releases/latest", new ReleaseServer.Answer { Body = Encoding.UTF8.GetBytes(body) });
        using var downloader = Downloader(server.Source());

        var error = await Assert.ThrowsAsync<ServiceStartException>(() => downloader.LatestAsync(Repo, Token));

        Assert.StartsWith($"The latest release of {Repo} could not be resolved: {server.BaseUrl}repos/{Repo}/releases/latest {reason}", error.Message, StringComparison.Ordinal);
    }

    [Fact]
    public async Task APinnedReleaseMustHaveItsPinAndTheChecksumServedMustAgreeWithIt()
    {
        server.Publish(Tag, Release);
        using var downloader = Downloader(server.Source(ReleaseServer.Pinning(Tag, Release)));

        var asset = await downloader.VouchAsync(Request(), Tag, null, Token);

        Assert.Equal(new ReleaseAsset(Repo, Tag, ReleaseServer.Asset, new Uri(server.BaseUrl, ReleaseServer.AssetPath(Tag)), TestCache.Sha256(Release), Pinned: true), asset);
        Assert.Equal([ReleaseServer.AssetPath(Tag) + ".sha256"], server.Requested);
        Assert.DoesNotContain(logger.Entries, entry => entry.Level >= LogLevel.Warning);
    }

    [Fact]
    public async Task AServedChecksumContradictingThePinIsRefusedBeforeTheBinaryIsFetched()
    {
        var pin = new string('c', 64);
        server.Publish(Tag, Release);
        using var downloader = Downloader(server.Source(ReleaseServer.Pins((Repo, Tag, ReleaseServer.Asset, pin))));

        var error = await Assert.ThrowsAsync<ChecksumMismatchException>(() => downloader.VouchAsync(Request(unpinnedAllowed: true), Tag, null, Token));

        Assert.Equal((new Uri(server.BaseUrl, ReleaseServer.AssetPath(Tag)).ToString(), pin, TestCache.Sha256(Release)), (error.BinaryPath, error.ExpectedSha256, error.ActualSha256));
        Assert.Contains($"{Repo} serves that digest beside {ReleaseServer.Asset} of {Tag}, but this client pins {pin}", error.Message, StringComparison.Ordinal);
        Assert.Equal([ReleaseServer.AssetPath(Tag) + ".sha256"], server.Requested);
    }

    [Fact]
    public async Task AReleaseNothingPinsIsRefusedNamingTheGapWithoutFetchingAnything()
    {
        server.Publish(Tag, Release);
        using var downloader = Downloader(server.Source());

        var error = await Assert.ThrowsAsync<UnpinnedReleaseException>(() => downloader.VouchAsync(Request(), Tag, null, Token));

        Assert.Equal((Repo, Tag, ReleaseServer.Asset), (error.Repository, error.Version, error.Asset));
        Assert.StartsWith($"sysml-grpc {Tag} of {Repo} was not downloaded: this client pins no SHA-256 digest for {ReleaseServer.Asset} of it, and it does not verify the signature on a release's SHA256SUMS.txt", error.Message, StringComparison.Ordinal);
        Assert.Contains($"$OPENSYSML_ALLOW_UNPINNED_DOWNLOAD={Repo} (or =1 for any repository)", error.Message, StringComparison.Ordinal);
        Assert.IsAssignableFrom<ServiceStartException>(error);
        Assert.Empty(server.Requested);
    }

    [Fact]
    public async Task SameOriginTrustTakesTheServedChecksumWithAWarning()
    {
        server.Publish(Tag, Release);
        using var downloader = Downloader(server.Source());

        var asset = await downloader.VouchAsync(Request(unpinnedAllowed: true), Tag, null, Token);

        Assert.Equal((TestCache.Sha256(Release), false), (asset.Sha256, asset.Pinned));
        var warning = Assert.Single(logger.Entries, entry => entry.Level == LogLevel.Warning);
        Assert.Equal(
            $"This client pins no digest for {ReleaseServer.Asset} of {Tag} of {Repo}; verifying it against the checksum served beside it, "
            + "which detects corruption but not a compromised release ($OPENSYSML_ALLOW_UNPINNED_DOWNLOAD grants that trust)",
            warning.Message);
    }

    [Theory]
    [InlineData(true)]
    [InlineData(false)]
    public async Task TheDigestTheCallerRequiresMustBeTheOneVouchedForWhateverVouchesForIt(bool pinned)
    {
        server.Publish(Tag, Release);
        using var downloader = Downloader(server.Source(pinned ? ReleaseServer.Pinning(Tag, Release) : null));
        var required = new string('E', 64);

        var error = await Assert.ThrowsAsync<ChecksumMismatchException>(() => downloader.VouchAsync(Request(unpinnedAllowed: true), Tag, required, Token));

        Assert.Equal((new string('e', 64), TestCache.Sha256(Release)), (error.ExpectedSha256, error.ActualSha256));
        Assert.Contains($"that is the digest {(pinned ? "this client pins for" : "served beside")} {ReleaseServer.Asset} of {Tag} of {Repo}, so nothing was downloaded", error.Message, StringComparison.Ordinal);
        Assert.Equal(TestCache.Sha256(Release), (await downloader.VouchAsync(Request(unpinnedAllowed: true), Tag, TestCache.Sha256(Release).ToUpperInvariant(), Token)).Sha256);
        Assert.DoesNotContain(ReleaseServer.AssetPath(Tag), server.Requested);
    }

    [Theory]
    [InlineData("")]
    [InlineData("not a digest  sysml-grpc-linux-amd64\n")]
    [InlineData("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcde  sysml-grpc-linux-amd64\n")]
    [InlineData("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdefg  sysml-grpc-linux-amd64\n")]
    public async Task AServedChecksumThatIsNotADigestIsRefused(string served)
    {
        server.Serve(ReleaseServer.AssetPath(Tag) + ".sha256", new ReleaseServer.Answer { Body = Encoding.ASCII.GetBytes(served) });
        using var downloader = Downloader(server.Source());

        var error = await Assert.ThrowsAsync<ServiceStartException>(() => downloader.VouchAsync(Request(unpinnedAllowed: true), Tag, null, Token));

        Assert.Equal($"The checksum {Repo} serves for {ReleaseServer.Asset} of {Tag} at {server.BaseUrl}{ReleaseServer.AssetPath(Tag)[1..]}.sha256 is not a SHA-256 digest; nothing was downloaded.", error.Message);
    }

    [Fact]
    public async Task AChecksumOfInvalidUtf8IsNotADigest()
    {
        server.Serve(ReleaseServer.AssetPath(Tag) + ".sha256", new ReleaseServer.Answer { Body = [.. Encoding.ASCII.GetBytes(new string('a', 64)), 0xff, 0xfe] });
        using var downloader = Downloader(server.Source());

        await Assert.ThrowsAsync<ServiceStartException>(() => downloader.VouchAsync(Request(unpinnedAllowed: true), Tag, null, Token));
    }

    [Fact]
    public async Task AChecksumThatIsNotThereIsAStartFailureQuotingTheStatus()
    {
        using var downloader = Downloader(server.Source(ReleaseServer.Pinning(Tag, Release)));

        var error = await Assert.ThrowsAsync<ServiceStartException>(() => downloader.VouchAsync(Request(), Tag, null, Token));

        Assert.Equal($"{server.BaseUrl}{ReleaseServer.AssetPath(Tag)[1..]}.sha256 could not be downloaded: it answered HTTP 404 (Not Found).", error.Message);
        Assert.IsNotType<ChecksumMismatchException>(error);
    }

    [Fact]
    public async Task AReleaseIsStreamedIntoTheCacheVerifiedPublishedAndRecorded()
    {
        server.Publish(Tag, Release);
        using var downloader = Downloader(server.Source(ReleaseServer.Pinning(Tag, Release)));
        await using var transaction = await Begin();

        var installed = await downloader.InstallAsync(transaction, await downloader.VouchAsync(Request(), Tag, null, Token), Token);

        Assert.Equal(new CacheInstall(TestCache.Sha256(Release), Recorded: true), installed);
        Assert.Equal(Release, File.ReadAllBytes(cache.Layout.Executable));
        Assert.Equal(new CacheMetadata(Tag, TestCache.Sha256(Release), Repo), CacheMetadata.Read(cache.Layout.Metadata).Metadata);
        if (!OperatingSystem.IsWindows())
        {
            Assert.Equal(UnixFileMode.UserRead | UnixFileMode.UserWrite | UnixFileMode.UserExecute, File.GetUnixFileMode(cache.Layout.Executable));
        }

        Assert.Equal([ReleaseServer.AssetPath(Tag) + ".sha256", ReleaseServer.AssetPath(Tag)], server.Requested);
        Assert.Contains(logger.Entries, entry => entry.Level == LogLevel.Information
            && entry.Message == $"Downloading sysml-grpc {Tag} of {Repo} from {new Uri(server.BaseUrl, ReleaseServer.AssetPath(Tag))}");
        AssertNothingStaged();
    }

    [Fact]
    public async Task BytesWithAnotherDigestThanThePinAreNeverPublished()
    {
        await cache.InstallAsync(Encoding.ASCII.GetBytes("a working release"), "v1.0.0");
        server.Serve(ReleaseServer.AssetPath(Tag), new ReleaseServer.Answer { Body = Encoding.ASCII.GetBytes("tampered") }).Served(Tag, TestCache.Sha256(Release));
        using var downloader = Downloader(server.Source(ReleaseServer.Pinning(Tag, Release)));
        await using var transaction = await Begin();

        var error = await Assert.ThrowsAsync<ChecksumMismatchException>(async () =>
            await downloader.InstallAsync(transaction, await downloader.VouchAsync(Request(), Tag, null, Token), Token));

        Assert.Equal((TestCache.Sha256(Release), TestCache.Sha256(Encoding.ASCII.GetBytes("tampered"))), (error.ExpectedSha256, error.ActualSha256));
        Assert.Equal("a working release", File.ReadAllText(cache.Layout.Executable));
        Assert.Equal("v1.0.0", CacheMetadata.Read(cache.Layout.Metadata).Metadata!.Version);
        AssertNothingStaged();
    }

    [Fact]
    public async Task ADeclaredLengthOverTheLimitIsRefusedBeforeItsBodyIsRead()
    {
        var error = await InstallFailingAsync(new ReleaseServer.Answer { Body = new byte[64], ClaimedLength = 1L << 40, StallAfter = 64 }, maxBinary: 1024);

        Assert.EndsWith($"could not be downloaded: it says it is {1L << 40} bytes, more than the 1024 bytes it may be, so it was not read further.", error.Message, StringComparison.Ordinal);
    }

    [Fact]
    public async Task ABodyWithoutALengthIsCutOffAtTheLimit()
    {
        var error = await InstallFailingAsync(new ReleaseServer.Answer { Endless = true }, maxBinary: 1024 * 1024);

        Assert.EndsWith("could not be downloaded: it sent more, more than the 1048576 bytes it may be, so it was not read further.", error.Message, StringComparison.Ordinal);
    }

    [Fact]
    public async Task ACompressedBodyIsBoundedAfterItIsDecompressed()
    {
        var bomb = ReleaseServer.Gzip(new byte[8 * 1024 * 1024]);
        Assert.True(bomb.Length < 64 * 1024);

        var error = await InstallFailingAsync(new ReleaseServer.Answer { Body = bomb, ContentEncoding = "gzip" }, maxBinary: 1024 * 1024);

        Assert.EndsWith("it sent more, more than the 1048576 bytes it may be, so it was not read further.", error.Message, StringComparison.Ordinal);
    }

    [Fact]
    public async Task ACompressedReleaseIsVerifiedAsTheBytesItDecompressesTo()
    {
        server.Serve(ReleaseServer.AssetPath(Tag), new ReleaseServer.Answer { Body = ReleaseServer.Gzip(Release), ContentEncoding = "gzip" });
        server.Serve(ReleaseServer.AssetPath(Tag) + ".sha256", new ReleaseServer.Answer { Body = ReleaseServer.Gzip(Encoding.ASCII.GetBytes(TestCache.Sha256(Release))), ContentEncoding = "gzip" });
        using var downloader = Downloader(server.Source(ReleaseServer.Pinning(Tag, Release)));
        await using var transaction = await Begin();

        await downloader.InstallAsync(transaction, await downloader.VouchAsync(Request(), Tag, null, Token), Token);

        Assert.Equal(Release, File.ReadAllBytes(cache.Layout.Executable));
    }

    [Fact]
    public async Task AChecksumOverTheMetadataLimitIsRefused()
    {
        server.Serve(ReleaseServer.AssetPath(Tag) + ".sha256", new ReleaseServer.Answer { Endless = true });
        using var downloader = Downloader(server.Source(maxMetadata: 4096));

        var error = await Assert.ThrowsAsync<ServiceStartException>(() => downloader.VouchAsync(Request(unpinnedAllowed: true), Tag, null, Token));

        Assert.EndsWith("it sent more, more than the 4096 bytes it may be, so it was not read further.", error.Message, StringComparison.Ordinal);
    }

    [Fact]
    public async Task ABodyEndingShortOfItsDeclaredLengthIsAFailureNotARelease()
    {
        var error = await InstallFailingAsync(new ReleaseServer.Answer { Body = Release, ClaimedLength = Release.Length + 100 });

        Assert.IsNotType<ChecksumMismatchException>(error);
        Assert.IsAssignableFrom<IOException>(error.InnerException);
    }

    [Fact]
    public async Task ABodyLongerThanItsDeclaredLengthIsReadOnlyToThatLength()
    {
        server.Serve(ReleaseServer.AssetPath(Tag), new ReleaseServer.Answer { Body = [.. Release, .. new byte[1024 * 1024]], ClaimedLength = Release.Length });
        server.Served(Tag, TestCache.Sha256(Release));
        using var downloader = Downloader(server.Source(ReleaseServer.Pinning(Tag, Release), maxBinary: 1024));
        await using var transaction = await Begin();

        await downloader.InstallAsync(transaction, await downloader.VouchAsync(Request(), Tag, null, Token), Token);

        Assert.Equal(Release, File.ReadAllBytes(cache.Layout.Executable));
    }

    [Fact]
    public async Task AnAnswerThatNeverComesIsAbandonedAfterTheIdleTimeout()
    {
        server.Serve(ReleaseServer.AssetPath(Tag) + ".sha256", new ReleaseServer.Answer { Silent = true });
        using var downloader = Downloader(server.Source(idle: TimeSpan.FromMilliseconds(500)));
        var clock = Stopwatch.StartNew();

        var error = await Assert.ThrowsAsync<ServiceStartException>(() => downloader.VouchAsync(Request(unpinnedAllowed: true), Tag, null, Token));

        Assert.InRange(clock.Elapsed, TimeSpan.FromMilliseconds(400), TimeSpan.FromSeconds(10));
        Assert.EndsWith("could not be downloaded: nothing was received for 0.5 s.", error.Message, StringComparison.Ordinal);
        Assert.IsType<TimeoutException>(error.InnerException);
    }

    [Theory]
    [InlineData(false)]
    [InlineData(true)]
    public async Task TheIdleTimeoutIsResetByHeaderProgressIncludingRedirects(bool redirect)
    {
        var latest = $"/repos/{Repo}/releases/latest";
        var answer = new ReleaseServer.Answer { Body = Encoding.ASCII.GetBytes($"{{\"tag_name\":\"{Tag}\"}}"), HeaderChunkBytes = 4, HeaderChunkDelay = TimeSpan.FromMilliseconds(100) };
        server.Serve(latest, redirect ? answer with { Status = 302, Location = "/latest", Body = [] } : answer);
        server.Serve("/latest", answer);
        using var downloader = Downloader(server.Source(idle: TimeSpan.FromMilliseconds(600)));
        var clock = Stopwatch.StartNew();

        Assert.Equal(Tag, await downloader.LatestAsync(Repo, Token));

        Assert.True(clock.Elapsed > TimeSpan.FromMilliseconds(1200), $"headers arrived in {clock.Elapsed}, too soon to prove an idle timeout");
        Assert.Equal(redirect ? [latest, "/latest"] : new[] { latest }, server.Requested);
    }

    [Fact]
    public async Task HeadersThatStopMidwayTimeOutAfterTheLastProgress()
    {
        server.Serve($"/repos/{Repo}/releases/latest", new ReleaseServer.Answer
        {
            HeaderChunkBytes = 2,
            HeaderChunkDelay = TimeSpan.FromMilliseconds(100),
            HeaderStallAfter = 16,
        });
        using var downloader = Downloader(server.Source(idle: TimeSpan.FromMilliseconds(600)));
        var clock = Stopwatch.StartNew();

        var error = await Assert.ThrowsAsync<ServiceStartException>(() => downloader.LatestAsync(Repo, Token));

        Assert.InRange(clock.Elapsed, TimeSpan.FromMilliseconds(1200), TimeSpan.FromSeconds(10));
        Assert.IsType<TimeoutException>(error.InnerException);
    }

    [Fact]
    public async Task CancellingDuringTrickledHeadersIsTheCallersCancellation()
    {
        server.Serve($"/repos/{Repo}/releases/latest", new ReleaseServer.Answer
        {
            HeaderChunkBytes = 2,
            HeaderChunkDelay = TimeSpan.FromMilliseconds(100),
        });
        using var downloader = Downloader(server.Source(idle: TimeSpan.FromMilliseconds(600)));
        using var cancel = CancellationTokenSource.CreateLinkedTokenSource(Token);
        cancel.CancelAfter(300);

        var error = await Assert.ThrowsAnyAsync<OperationCanceledException>(() => downloader.LatestAsync(Repo, cancel.Token));

        Assert.Equal(cancel.Token, error.CancellationToken);
    }

    [Fact]
    public async Task HeaderProgressTimeoutsWorkOnReusedConnectionsAndAfterAnIdlePool()
    {
        server.Serve($"/repos/{Repo}/releases/latest", new ReleaseServer.Answer
        {
            Body = Encoding.ASCII.GetBytes($"{{\"tag_name\":\"{Tag}\"}}"),
            HeaderChunkBytes = 4,
            HeaderChunkDelay = TimeSpan.FromMilliseconds(100),
            KeepAlive = true,
        });
        using var downloader = Downloader(server.Source(idle: TimeSpan.FromMilliseconds(600)));

        Assert.Equal(Tag, await downloader.LatestAsync(Repo, Token));
        await Task.Delay(200, Token);
        Assert.Equal(Tag, await downloader.LatestAsync(Repo, Token));
        Assert.Equal(1, server.Connections);

        await Task.Delay(800, Token);
        Assert.Equal(Tag, await downloader.LatestAsync(Repo, Token));
        Assert.Equal(3, server.Requested.Count);
    }

    [Fact]
    public async Task AHeaderTimeoutDoesNotPoisonTheNextRequest()
    {
        server.Serve($"/repos/{Repo}/releases/latest", new ReleaseServer.Answer { HeaderStallAfter = 16 });
        using var downloader = Downloader(server.Source(idle: TimeSpan.FromMilliseconds(300)));

        var error = await Assert.ThrowsAsync<ServiceStartException>(() => downloader.LatestAsync(Repo, Token));
        Assert.IsType<TimeoutException>(error.InnerException);

        server.Latest(Tag);
        Assert.Equal(Tag, await downloader.LatestAsync(Repo, Token));
    }

    [Fact]
    public async Task ABodyThatStopsMidwayIsAbandonedAfterTheIdleTimeoutAndNothingIsLeftStaged()
    {
        var clock = Stopwatch.StartNew();

        var error = await InstallFailingAsync(new ReleaseServer.Answer { Body = new byte[256 * 1024], StallAfter = 100 * 1024 }, idle: TimeSpan.FromMilliseconds(500));

        Assert.InRange(clock.Elapsed, TimeSpan.FromMilliseconds(400), TimeSpan.FromSeconds(10));
        Assert.IsType<TimeoutException>(error.InnerException);
        Assert.False(File.Exists(cache.Layout.Executable));
    }

    [Fact]
    public async Task TheTimeoutIsOnSilenceNotOnTheWholeDownload()
    {
        var body = new byte[16 * 1024];
        Random.Shared.NextBytes(body);
        server.Serve(ReleaseServer.AssetPath(Tag), new ReleaseServer.Answer { Body = body, ChunkBytes = 1024, ChunkDelay = TimeSpan.FromMilliseconds(100) }).Served(Tag, TestCache.Sha256(body));
        using var downloader = Downloader(server.Source(ReleaseServer.Pinning(Tag, body), idle: TimeSpan.FromMilliseconds(600)));
        await using var transaction = await Begin();
        var clock = Stopwatch.StartNew();

        await downloader.InstallAsync(transaction, await downloader.VouchAsync(Request(), Tag, null, Token), Token);

        Assert.True(clock.Elapsed > TimeSpan.FromMilliseconds(1200), $"the download took {clock.Elapsed}, too short to show the timeout is not on its whole");
        Assert.Equal(body, File.ReadAllBytes(cache.Layout.Executable));
    }

    [Fact]
    public async Task CancellingDuringAStallIsTheCallersCancellationNotATimeout()
    {
        server.Serve(ReleaseServer.AssetPath(Tag), new ReleaseServer.Answer { Body = new byte[4096], StallAfter = 1024 }).Served(Tag, new string('a', 64));
        using var downloader = Downloader(server.Source(ReleaseServer.Pins((Repo, Tag, ReleaseServer.Asset, new string('a', 64)))));
        await using var transaction = await Begin();
        var asset = await downloader.VouchAsync(Request(), Tag, null, Token);
        using var cancel = CancellationTokenSource.CreateLinkedTokenSource(Token);
        cancel.CancelAfter(300);

        var error = await Assert.ThrowsAnyAsync<OperationCanceledException>(() => downloader.InstallAsync(transaction, asset, cancel.Token));

        Assert.Equal(cancel.Token, error.CancellationToken);
        Assert.False(File.Exists(cache.Layout.Executable));
        AssertNothingStaged();
    }

    [Theory]
    [InlineData(false)]
    [InlineData(true)]
    public async Task ARedirectOnTheLoopbackHostIsFollowedRelativeOrAbsolute(bool absolute)
    {
        server.Serve(ReleaseServer.AssetPath(Tag), new ReleaseServer.Answer { Status = 302, Location = absolute ? $"{server.BaseUrl}cdn/blob" : "/cdn/blob" });
        server.Serve("/cdn/blob", new ReleaseServer.Answer { Body = Release }).Served(Tag, TestCache.Sha256(Release));
        using var downloader = Downloader(server.Source(ReleaseServer.Pinning(Tag, Release)));
        await using var transaction = await Begin();

        await downloader.InstallAsync(transaction, await downloader.VouchAsync(Request(), Tag, null, Token), Token);

        Assert.Equal(Release, File.ReadAllBytes(cache.Layout.Executable));
        Assert.Equal([ReleaseServer.AssetPath(Tag) + ".sha256", ReleaseServer.AssetPath(Tag), "/cdn/blob"], server.Requested);
    }

    [Fact]
    public async Task RedirectsAreFollowedOnlySoFar()
    {
        server.Serve("/loop", new ReleaseServer.Answer { Status = 307, Location = "/loop" });
        server.Serve(ReleaseServer.AssetPath(Tag), new ReleaseServer.Answer { Status = 301, Location = "/loop" });

        var error = await InstallFailingAsync(null);

        Assert.EndsWith("could not be downloaded: it was redirected more than 10 times.", error.Message, StringComparison.Ordinal);
        Assert.Equal(11, server.Requested.Count(path => path == "/loop" || path == ReleaseServer.AssetPath(Tag)));
    }

    [Fact]
    public async Task ARedirectThatSaysNotWhereIsAFailure()
    {
        var error = await InstallFailingAsync(new ReleaseServer.Answer { Status = 302 });

        Assert.EndsWith("could not be downloaded: it redirected without saying where.", error.Message, StringComparison.Ordinal);
    }

    [Fact]
    public async Task ARedirectToPlainHttpOffTheLoopbackHostIsNotFollowed()
    {
        var error = await InstallFailingAsync(new ReleaseServer.Answer { Status = 302, Location = "http://example.invalid/blob" });

        Assert.Equal($"{server.BaseUrl}{ReleaseServer.AssetPath(Tag)[1..]} redirected to http://example.invalid/blob, which is not HTTPS; it was not followed.", error.Message);
        Assert.Equal([ReleaseServer.AssetPath(Tag) + ".sha256", ReleaseServer.AssetPath(Tag)], server.Requested);
    }

    [Fact]
    public void HttpsIsAlwaysFetchedFromAndPlainHttpOnlyOnALoopbackHostOfATestSourceAndNeverAfterHttps()
    {
        var test = new ReleaseSource { LoopbackHttp = true };
        var loopback = new Uri("http://127.0.0.1:1/x");
        var secure = new Uri("https://objects.example/x");

        BinaryDownloader.Allow(ReleaseSource.Production, secure, null);
        BinaryDownloader.Allow(ReleaseSource.Production, secure, new Uri("https://github.com/a"));
        BinaryDownloader.Allow(test, loopback, null);
        BinaryDownloader.Allow(test, loopback, new Uri("http://localhost:2/y"));
        BinaryDownloader.Allow(test, new Uri("http://[::1]:1/x"), null);

        Assert.Equal(
            "Releases are fetched over HTTPS alone, so http://127.0.0.1:1/x is not fetched from.",
            Assert.Throws<ServiceStartException>(() => BinaryDownloader.Allow(ReleaseSource.Production, loopback, null)).Message);
        Assert.Equal(
            "https://github.com/a redirected to http://127.0.0.1:1/x, which would give up HTTPS; it was not followed.",
            Assert.Throws<ServiceStartException>(() => BinaryDownloader.Allow(test, loopback, new Uri("https://github.com/a"))).Message);
        Assert.Throws<ServiceStartException>(() => BinaryDownloader.Allow(test, new Uri("http://example.invalid/x"), null));
        Assert.Throws<ServiceStartException>(() => BinaryDownloader.Allow(test, new Uri("ftp://127.0.0.1/x"), null));
        Assert.Throws<ServiceStartException>(() => BinaryDownloader.Allow(test, new Uri("file:///tmp/x"), null));
    }

    [Fact]
    public void TheProductionSourceIsGitHubOverHttpsWithThePinsOfThisCheckoutAndTheSharedLimits()
    {
        var production = ReleaseSource.Production;

        Assert.Equal((new Uri("https://github.com/"), new Uri("https://api.github.com/")), (production.Downloads, production.Api));
        Assert.Null(production.Pins);
        Assert.Null(production.Asset);
        Assert.False(production.LoopbackHttp);
        Assert.Equal(TimeSpan.FromSeconds(15), production.IdleTimeout);
        Assert.Equal((512L * 1024 * 1024, 8L * 1024 * 1024, 10), (production.MaxBinaryBytes, production.MaxMetadataBytes, production.MaxRedirects));
        Assert.Throws<ServiceStartException>(() => new BinaryDownloader(production with { Downloads = new Uri("http://github.com/") }, NullLogger.Instance));
        Assert.Throws<ServiceStartException>(() => new BinaryDownloader(production with { Api = new Uri("http://127.0.0.1:1/") }, NullLogger.Instance));
    }

    public ValueTask InitializeAsync() => ValueTask.CompletedTask;

    public async ValueTask DisposeAsync()
    {
        await server.DisposeAsync();
        cache.Dispose();
    }

    private static ReleaseRequest Request(bool unpinnedAllowed = false) => new(Repo, Tag, Explicit: true, unpinnedAllowed);

    private BinaryDownloader Downloader(ReleaseSource source) => new(source, logger);

    private Task<CacheTransaction> Begin() => CacheTransaction.BeginAsync(cache.Layout, logger, Token);

    // Installs the release from an answer for its binary, its served checksum and pin being a digest it cannot have.
    private async Task<ServiceStartException> InstallFailingAsync(ReleaseServer.Answer? answer, long? maxBinary = null, TimeSpan? idle = null)
    {
        var pin = new string('a', 64);
        if (answer is not null)
        {
            server.Serve(ReleaseServer.AssetPath(Tag), answer);
        }

        server.Served(Tag, pin);
        using var downloader = Downloader(server.Source(ReleaseServer.Pins((Repo, Tag, ReleaseServer.Asset, pin)), idle, maxBinary));
        var error = await Assert.ThrowsAnyAsync<ServiceStartException>(async () =>
        {
            await using var transaction = await Begin();
            await downloader.InstallAsync(transaction, await downloader.VouchAsync(Request(), Tag, null, Token), Token);
        });
        AssertNothingStaged();
        return error;
    }

    private void AssertNothingStaged() =>
        Assert.DoesNotContain(cache.Files(), name => CacheLayout.IsTemporary(name));
}
