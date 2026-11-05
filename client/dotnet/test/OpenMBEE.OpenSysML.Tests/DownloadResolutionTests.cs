using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Runtime.Versioning;
using System.Text;
using System.Threading;
using System.Threading.Tasks;
using Microsoft.Extensions.Logging;
using OpenMBEE.OpenSysML.Internal;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// A private service's binary provided from a release when none is named: what the shared cache holds is used,
/// replaced or kept according to the release asked for and whether it can be had, and nothing that contradicts a
/// pin or a digest the caller requires is ever installed or started, however good the cache.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed class DownloadResolutionTests : IAsyncLifetime
{
    private const string Tag = "v2.0.0";
    private static readonly byte[] Old = Encoding.ASCII.GetBytes("#!/bin/sh\necho an older release\n");
    private static readonly byte[] New = Encoding.ASCII.GetBytes("#!/bin/sh\necho the release asked for\n");

    private readonly ReleaseServer server = new();
    private readonly TestCache cache = new();
    private readonly ListLogger logger = new();

    private static CancellationToken Token => TestContext.Current.CancellationToken;

    private static string Repo => ReleaseServer.Repo;

    private static string BuiltAgainst => ReleaseRequest.BuiltAgainst;

    private CacheLayout Layout => cache.Layout;

    private IEnumerable<string> Warnings => logger.Entries.Where(entry => entry.Level == LogLevel.Warning).Select(entry => entry.Message);

    [Fact]
    public async Task WithNoCacheTheReleaseBuiltAgainstIsDownloadedVerifiedRecordedAndStartedFromItsSnapshot()
    {
        server.Publish(BuiltAgainst, New);

        var resolved = await Resolve(new ConnectionOptions(), server.Source(ReleaseServer.Pinning(BuiltAgainst, New)));

        Assert.Equal(new ResolvedBinary(BinaryResolver.Canonical(Layout.Snapshot(TestCache.Sha256(New))), TestCache.Sha256(New)), resolved);
        Assert.Equal(New, File.ReadAllBytes(Layout.Executable));
        Assert.Equal(new CacheMetadata(BuiltAgainst, TestCache.Sha256(New), Repo), CacheMetadata.Read(Layout.Metadata).Metadata);
        Assert.Equal(cache.SettledFiles(resolved.Path), cache.Files());
        Assert.Equal([ReleaseServer.AssetPath(BuiltAgainst) + ".sha256", ReleaseServer.AssetPath(BuiltAgainst)], server.Requested);
        Assert.Empty(Warnings);
        if (!OperatingSystem.IsWindows())
        {
            Assert.Equal(UnixFileMode.UserRead | UnixFileMode.UserWrite | UnixFileMode.UserExecute, File.GetUnixFileMode(Layout.Executable));
        }
    }

    [Theory]
    [InlineData(true)]
    [InlineData(false)]
    public async Task TheReleaseAskedForAlreadyCachedIsStartedWithoutAnyRequest(bool byOption)
    {
        await cache.InstallAsync(New, Tag);
        var options = byOption ? new ConnectionOptions { DownloadVersion = Tag } : new ConnectionOptions();
        var environment = byOption ? Environment() : Environment(new() { [ConnectionOptions.VersionVariable] = Tag });

        var resolved = await Resolve(options, server.Source(ReleaseServer.Pinning(Tag, New)), environment);

        Assert.Equal(TestCache.Sha256(New), resolved.Sha256);
        Assert.Empty(server.Requested);
        Assert.Empty(Warnings);
    }

    [Fact]
    public async Task TheReleaseBuiltAgainstAlreadyCachedIsStartedWithoutAnyRequest()
    {
        await cache.InstallAsync(New, BuiltAgainst);

        Assert.Equal(TestCache.Sha256(New), (await Resolve(new ConnectionOptions(), server.Source())).Sha256);
        Assert.Empty(server.Requested);
    }

    [Theory]
    [InlineData("tag")]
    [InlineData("implicit")]
    [InlineData("digest")]
    [InlineData("latest")]
    [InlineData("latest-unresolved")]
    [InlineData("fallback")]
    public async Task ACachedReleaseContradictingItsPinIsNeverStarted(string selection)
    {
        var cachedTag = selection == "implicit" ? BuiltAgainst : Tag;
        await cache.InstallAsync(Old, cachedTag);
        server.Latest(Tag);
        if (selection == "latest-unresolved")
        {
            server.Serve($"/repos/{Repo}/releases/latest", new ReleaseServer.Answer { Status = 404 });
        }

        var options = selection switch
        {
            "tag" => new ConnectionOptions { DownloadVersion = Tag },
            "digest" => new ConnectionOptions { ExpectedBinarySha256 = TestCache.Sha256(Old) },
            "latest" or "latest-unresolved" => new ConnectionOptions { DownloadVersion = "latest" },
            _ => new ConnectionOptions(),
        };
        var environment = Environment(new() { [ConnectionOptions.AllowUnpinnedDownloadVariable] = "1" }, PathBinary(New));

        var error = await Assert.ThrowsAsync<ChecksumMismatchException>(() =>
            Resolve(options, server.Source(ReleaseServer.Pinning(cachedTag, New)), environment));

        Assert.Equal((Layout.Executable, TestCache.Sha256(New), TestCache.Sha256(Old)), (error.BinaryPath, error.ExpectedSha256, error.ActualSha256));
        Assert.Equal(Old, File.ReadAllBytes(Layout.Executable));
        Assert.Equal(new CacheMetadata(cachedTag, TestCache.Sha256(Old), Repo), CacheMetadata.Read(Layout.Metadata).Metadata);
        Assert.Empty(server.Requested);
        Assert.False(File.Exists(Layout.Snapshot(TestCache.Sha256(Old))));
        Assert.DoesNotContain(cache.Files(), name => CacheLayout.IsTemporary(name));
    }

    [Theory]
    [InlineData("someone/fork", Tag, ReleaseServer.Asset)]
    [InlineData(ReleaseServer.Repo, "v3.0.0", ReleaseServer.Asset)]
    [InlineData(ReleaseServer.Repo, Tag, "sysml-grpc-other-platform")]
    public async Task ACachedReleaseIsNotComparedWithAPinForAnotherRepositoryTagOrAsset(string repo, string tag, string asset)
    {
        await cache.InstallAsync(Old, Tag);
        var source = server.Source(ReleaseServer.Pins((repo, tag, asset, TestCache.Sha256(New))));

        Assert.Equal(TestCache.Sha256(Old), (await Resolve(new ConnectionOptions { DownloadVersion = Tag }, source)).Sha256);
        Assert.Empty(server.Requested);
    }

    [Fact]
    public async Task ACacheOfAnotherReleaseIsReplacedWithAWarningOnceTheReleaseAskedForVerifies()
    {
        await cache.InstallAsync(Old, "v1.0.0");
        var oldSnapshot = (await Resolve(new ConnectionOptions { DownloadVersion = "v1.0.0" }, server.Source())).Path;
        server.Publish(Tag, New);

        var resolved = await Resolve(new ConnectionOptions { DownloadVersion = Tag }, server.Source(ReleaseServer.Pinning(Tag, New)));

        Assert.Equal(TestCache.Sha256(New), resolved.Sha256);
        Assert.Equal(new CacheMetadata(Tag, TestCache.Sha256(New), Repo), CacheMetadata.Read(Layout.Metadata).Metadata);
        Assert.Equal(Old, File.ReadAllBytes(oldSnapshot));
        Assert.Equal(
            [$"Replacing the cached sysml-grpc: the binary cached at {Layout.Executable} is v1.0.0, but {Tag} was asked for. Downloading {Tag} of {Repo}"],
            Warnings);
    }

    [Fact]
    public async Task ACacheOfAnotherRepositoryIsNotTakenForThisOnesRelease()
    {
        await cache.InstallAsync(Old, Tag, "someone/fork");
        Assert.Equal(TestCache.Sha256(Old), (await Resolve(new ConnectionOptions { DownloadVersion = Tag, GithubRepo = "someone/fork" }, server.Source())).Sha256);
        Assert.Empty(server.Requested);
        server.Publish(Tag, New);

        var resolved = await Resolve(new ConnectionOptions { DownloadVersion = Tag }, server.Source(ReleaseServer.Pinning(Tag, New)));

        Assert.Equal(TestCache.Sha256(New), resolved.Sha256);
        Assert.Equal(
            [$"Replacing the cached sysml-grpc: the binary cached at {Layout.Executable} was downloaded from someone/fork, but {Tag} of {Repo} was asked for. Downloading {Tag} of {Repo}"],
            Warnings);
    }

    [Fact]
    public async Task ABuildInstalledByHandIsKeptWhenNoReleaseIsAskedForAndReplacedWhenOneIs()
    {
        InstallByHand(Old);
        server.Publish(Tag, New);
        var source = server.Source(ReleaseServer.Pinning(Tag, New));

        Assert.Equal(TestCache.Sha256(Old), (await Resolve(new ConnectionOptions(), source)).Sha256);
        Assert.Empty(server.Requested);
        Assert.Empty(Warnings);

        Assert.Equal(TestCache.Sha256(New), (await Resolve(new ConnectionOptions { DownloadVersion = Tag }, source)).Sha256);
        Assert.Equal(
            [$"Replacing the cached sysml-grpc: the binary cached at {Layout.Executable} has no usable record (absent), so which release it is cannot be told, and {Tag} of {Repo} was asked for. Downloading {Tag} of {Repo}"],
            Warnings);
    }

    [Fact]
    public async Task AnExecutableItsRecordDoesNotVouchForIsNotTakenForABuildInstalledByHand()
    {
        await cache.InstallAsync(Old, BuiltAgainst);
        File.WriteAllBytes(Layout.Executable, New.Reverse().ToArray());
        server.Publish(BuiltAgainst, New);

        var resolved = await Resolve(new ConnectionOptions(), server.Source(ReleaseServer.Pinning(BuiltAgainst, New)));

        Assert.Equal(TestCache.Sha256(New), resolved.Sha256);
        Assert.Equal(
            [$"Replacing the cached sysml-grpc: the binary cached at {Layout.Executable} is not the {BuiltAgainst} its record names, whose digest is another, and {BuiltAgainst} of {Repo} was asked for. Downloading {BuiltAgainst} of {Repo}"],
            Warnings);
    }

    [Fact]
    public async Task ARecordThatNamesNoRepositoryIsNotTakenForTheRepositoryAskedFor()
    {
        await cache.InstallAsync(Old, Tag);
        File.WriteAllText(Layout.Metadata, $"{{\"version\": \"{Tag}\", \"sha256\": \"{TestCache.Sha256(Old)}\"}}");
        server.Publish(Tag, New);

        var resolved = await Resolve(new ConnectionOptions { DownloadVersion = Tag }, server.Source(ReleaseServer.Pinning(Tag, New)));

        Assert.Equal(TestCache.Sha256(New), resolved.Sha256);
        Assert.Equal(
            [$"Replacing the cached sysml-grpc: the binary cached at {Layout.Executable} is {Tag} of a repository its record does not name, and {Tag} of {Repo} was asked for. Downloading {Tag} of {Repo}"],
            Warnings);
    }

    [Theory]
    [InlineData(true)]
    [InlineData(false)]
    public async Task WhenTheReleaseBuiltAgainstCannotBeHadAWorkingCacheIsKeptWithAWarning(bool pinned)
    {
        await cache.InstallAsync(Old, "v1.0.0");
        var source = ReleaseServer.Offline(pinned ? ReleaseServer.Pinning(BuiltAgainst, New) : null);

        var resolved = await Resolve(new ConnectionOptions(), source);

        Assert.Equal(new ResolvedBinary(BinaryResolver.Canonical(Layout.Snapshot(TestCache.Sha256(Old))), TestCache.Sha256(Old)), resolved);
        Assert.Equal(Old, File.ReadAllBytes(Layout.Executable));
        Assert.Collection(
            Warnings,
            replacing => Assert.StartsWith("Replacing the cached sysml-grpc: ", replacing, StringComparison.Ordinal),
            keeping =>
            {
                Assert.StartsWith($"Keeping the cached sysml-grpc at {Layout.Executable}: {BuiltAgainst} of {Repo} could not be had (", keeping, StringComparison.Ordinal);
                Assert.Contains(pinned ? "could not be downloaded" : "this client pins no SHA-256 digest", keeping, StringComparison.Ordinal);
                Assert.EndsWith("). It may be an older release than the one this client was built with", keeping, StringComparison.Ordinal);
            });
    }

    [Theory]
    [InlineData(true)]
    [InlineData(false)]
    public async Task WhenAReleaseAskedForCannotBeHadTheCacheIsNotStartedInItsPlace(bool pinned)
    {
        await cache.InstallAsync(Old, "v1.0.0");
        var path = PathBinary(New);
        var source = ReleaseServer.Offline(pinned ? ReleaseServer.Pinning(Tag, New) : null);

        var error = await Assert.ThrowsAnyAsync<ServiceStartException>(() => Resolve(new ConnectionOptions { DownloadVersion = Tag }, source, Environment(path: path)));

        if (pinned)
        {
            Assert.IsType<ServiceStartException>(error);
            Assert.IsType<System.Net.Http.HttpRequestException>(error.InnerException);
        }
        else
        {
            Assert.Equal(Tag, Assert.IsType<UnpinnedReleaseException>(error).Version);
        }

        Assert.Equal(Old, File.ReadAllBytes(Layout.Executable));
        Assert.Equal("v1.0.0", CacheMetadata.Read(Layout.Metadata).Metadata!.Version);
    }

    [Theory]
    [InlineData(false, false)]
    [InlineData(false, true)]
    [InlineData(true, false)]
    [InlineData(true, true)]
    public async Task AReleaseContradictingItsPinFailsEvenWithAWorkingCache(bool asked, bool tamperedBody)
    {
        await cache.InstallAsync(Old, "v1.0.0");
        var tag = asked ? Tag : BuiltAgainst;
        if (tamperedBody)
        {
            server.Serve(ReleaseServer.AssetPath(tag), new ReleaseServer.Answer { Body = Encoding.ASCII.GetBytes("tampered") }).Served(tag, TestCache.Sha256(New));
        }
        else
        {
            server.Publish(tag, Encoding.ASCII.GetBytes("republished"));
        }

        var error = await Assert.ThrowsAsync<ChecksumMismatchException>(() =>
            Resolve(asked ? new ConnectionOptions { DownloadVersion = Tag } : new ConnectionOptions(), server.Source(ReleaseServer.Pinning(tag, New)), Environment(path: PathBinary(New))));

        Assert.Equal(TestCache.Sha256(New), error.ExpectedSha256);
        Assert.Equal(Old, File.ReadAllBytes(Layout.Executable));
        Assert.Equal("v1.0.0", CacheMetadata.Read(Layout.Metadata).Metadata!.Version);
        Assert.DoesNotContain(cache.Files(), name => CacheLayout.IsTemporary(name));
        Assert.DoesNotContain(Warnings, warning => warning.StartsWith("Keeping", StringComparison.Ordinal));
    }

    [Fact]
    public async Task WithNoCacheAReleaseThatCannotBeHadGivesWayToPathWithAWarning()
    {
        var path = PathBinary(New);

        var resolved = await Resolve(new ConnectionOptions(), ReleaseServer.Offline(ReleaseServer.Pinning(BuiltAgainst, New)), Environment(path: path));

        var onPath = Path.Combine(path, BinaryResolver.ExecutableName);
        Assert.Equal(new ResolvedBinary(BinaryResolver.Canonical(onPath), TestCache.Sha256(New)), resolved);
        Assert.False(File.Exists(Layout.Executable));
        var warning = Assert.Single(Warnings);
        Assert.StartsWith($"Starting {onPath} from $PATH: sysml-grpc {BuiltAgainst} of {Repo} could not be had (", warning, StringComparison.Ordinal);
    }

    [Fact]
    public async Task WithNoCacheAndNothingOnPathTheFailureIsBinaryNotFoundCarryingWhy()
    {
        var error = await Assert.ThrowsAsync<BinaryNotFoundException>(() => Resolve(new ConnectionOptions(), server.Source()));

        Assert.Equal(
            [
                "$OPENSYSML_GRPC_BINARY: unset",
                $"{Layout.Executable}: absent",
                $"release {BuiltAgainst} of {Repo}: sysml-grpc {BuiltAgainst} of {Repo} was not downloaded: this client pins no SHA-256 digest for {ReleaseServer.Asset} of it, and it does not verify the signature on a release's SHA256SUMS.txt, as the Python, Java and Node clients do, so the only digest left is the one served beside the binary, which a compromised release would serve too",
                "$PATH: unset or empty",
            ],
            error.SearchedLocations);
        Assert.IsType<UnpinnedReleaseException>(error.InnerException);
        Assert.Empty(server.Requested);
    }

    [Fact]
    public async Task LatestIsResolvedOnceAnOpenAndInstalledUnderTheTagItResolvesTo()
    {
        server.Latest(Tag).Publish(Tag, New);
        var source = server.Source(ReleaseServer.Pinning(Tag, New));

        var first = await Resolve(new ConnectionOptions { DownloadVersion = "latest" }, source);
        var second = await Resolve(new ConnectionOptions(), source, Environment(new() { [ConnectionOptions.VersionVariable] = "latest" }));

        Assert.Equal(first, second);
        Assert.Equal(new CacheMetadata(Tag, TestCache.Sha256(New), Repo), CacheMetadata.Read(Layout.Metadata).Metadata);
        Assert.Equal(
            [$"/repos/{Repo}/releases/latest", ReleaseServer.AssetPath(Tag) + ".sha256", ReleaseServer.AssetPath(Tag), $"/repos/{Repo}/releases/latest"],
            server.Requested);
    }

    [Fact]
    public async Task WhenLatestCannotBeResolvedACachedReleaseOfTheRepositoryIsKeptAndNothingElseIs()
    {
        await cache.InstallAsync(Old, "v1.0.0");

        Assert.Equal(TestCache.Sha256(Old), (await Resolve(new ConnectionOptions { DownloadVersion = "latest" }, ReleaseServer.Offline())).Sha256);
        var warning = Assert.Single(Warnings);
        Assert.StartsWith($"Keeping the cached sysml-grpc v1.0.0 of {Repo} at {Layout.Executable}: the latest release could not be resolved (", warning, StringComparison.Ordinal);

        var error = await Assert.ThrowsAsync<ServiceStartException>(() => Resolve(new ConnectionOptions { DownloadVersion = "latest", GithubRepo = "someone/fork" }, ReleaseServer.Offline()));
        Assert.StartsWith("http://127.0.0.1:", error.Message, StringComparison.Ordinal);
        Assert.Contains("/repos/someone/fork/releases/latest could not be downloaded", error.Message, StringComparison.Ordinal);

        File.Delete(Layout.Metadata);
        await Assert.ThrowsAsync<ServiceStartException>(() => Resolve(new ConnectionOptions { DownloadVersion = "latest" }, ReleaseServer.Offline()));

        File.Delete(Layout.Executable);
        var nothing = await Assert.ThrowsAsync<ServiceStartException>(() => Resolve(new ConnectionOptions { DownloadVersion = "latest" }, ReleaseServer.Offline(), Environment(path: PathBinary(New))));
        Assert.IsNotType<BinaryNotFoundException>(nothing);
        Assert.Contains($"/repos/{Repo}/releases/latest could not be downloaded", nothing.Message, StringComparison.Ordinal);
    }

    [Fact]
    public async Task ACachedBinaryWithTheDigestRequiredIsStartedWithoutLookingForARelease()
    {
        await cache.InstallAsync(Old, "v1.0.0");

        var resolved = await Resolve(new ConnectionOptions { ExpectedBinarySha256 = TestCache.Sha256(Old) }, server.Source());

        Assert.Equal(TestCache.Sha256(Old), resolved.Sha256);
        Assert.Empty(server.Requested);
        Assert.Empty(Warnings);
    }

    [Theory]
    [InlineData(true, true)]
    [InlineData(true, false)]
    [InlineData(false, true)]
    [InlineData(false, false)]
    public async Task AnExplicitReleaseAndDigestMustBothHoldWithOrWithoutACache(bool cached, bool byOption)
    {
        if (cached)
        {
            await cache.InstallAsync(Old, "v1.0.0");
        }

        server.Publish(Tag, New);
        var options = new ConnectionOptions { DownloadVersion = byOption ? Tag : null, ExpectedBinarySha256 = TestCache.Sha256(Old) };
        var environment = byOption ? Environment() : Environment(new() { [ConnectionOptions.VersionVariable] = Tag });

        var error = await Assert.ThrowsAsync<ChecksumMismatchException>(() =>
            Resolve(options, server.Source(ReleaseServer.Pinning(Tag, New)), environment));

        Assert.Equal((TestCache.Sha256(Old), TestCache.Sha256(New)), (error.ExpectedSha256, error.ActualSha256));
        Assert.Equal([ReleaseServer.AssetPath(Tag) + ".sha256"], server.Requested);
        Assert.Equal(cached, File.Exists(Layout.Executable));
        if (cached)
        {
            Assert.Equal(Old, File.ReadAllBytes(Layout.Executable));
            Assert.Equal("v1.0.0", CacheMetadata.Read(Layout.Metadata).Metadata!.Version);
        }
    }

    [Fact]
    public async Task AnExplicitReleaseOfAnotherRepositoryDoesNotGiveWayToACacheMatchingTheRequiredDigest()
    {
        await cache.InstallAsync(Old, Tag, "someone/fork");
        server.Publish(Tag, New);

        await Assert.ThrowsAsync<ChecksumMismatchException>(() =>
            Resolve(new ConnectionOptions { DownloadVersion = Tag, ExpectedBinarySha256 = TestCache.Sha256(Old) }, server.Source(ReleaseServer.Pinning(Tag, New))));

        Assert.Equal([ReleaseServer.AssetPath(Tag) + ".sha256"], server.Requested);
        Assert.Equal("someone/fork", CacheMetadata.Read(Layout.Metadata).Metadata!.Repo);
    }

    [Fact]
    public async Task LatestIsStillResolvedWhenTheCacheMatchesTheRequiredDigest()
    {
        await cache.InstallAsync(Old, "v1.0.0");
        server.Latest(Tag).Publish(Tag, New);

        await Assert.ThrowsAsync<ChecksumMismatchException>(() =>
            Resolve(new ConnectionOptions { DownloadVersion = "latest", ExpectedBinarySha256 = TestCache.Sha256(Old) }, server.Source(ReleaseServer.Pinning(Tag, New))));

        Assert.Equal([$"/repos/{Repo}/releases/latest", ReleaseServer.AssetPath(Tag) + ".sha256"], server.Requested);
        Assert.Equal(Old, File.ReadAllBytes(Layout.Executable));
    }

    [Fact]
    public async Task AnExplicitReleaseDoesNotGiveWayToAHandInstalledBuildMatchingTheRequiredDigest()
    {
        InstallByHand(Old);
        server.Publish(Tag, New);

        await Assert.ThrowsAsync<ChecksumMismatchException>(() =>
            Resolve(new ConnectionOptions { DownloadVersion = Tag, ExpectedBinarySha256 = TestCache.Sha256(Old) }, server.Source(ReleaseServer.Pinning(Tag, New))));

        Assert.Equal([ReleaseServer.AssetPath(Tag) + ".sha256"], server.Requested);
        Assert.Equal(Old, File.ReadAllBytes(Layout.Executable));
        Assert.False(File.Exists(Layout.Metadata));
    }

    [Theory]
    [InlineData("\\uD800")]
    [InlineData("\\uDC00")]
    public async Task LatestWithInvalidUnicodeKeepsAVouchedCacheWithAWarning(string tag)
    {
        await cache.InstallAsync(Old, Tag);
        server.Serve($"/repos/{Repo}/releases/latest", new ReleaseServer.Answer { Body = Encoding.ASCII.GetBytes($"{{\"tag_name\":\"{tag}\"}}") });

        var resolved = await Resolve(new ConnectionOptions { DownloadVersion = "latest" }, server.Source(ReleaseServer.Pinning(Tag, Old)));

        Assert.Equal(TestCache.Sha256(Old), resolved.Sha256);
        Assert.StartsWith($"Keeping the cached sysml-grpc {Tag} of {Repo}", Assert.Single(Warnings), StringComparison.Ordinal);
        Assert.Equal([$"/repos/{Repo}/releases/latest"], server.Requested);
    }

    [Theory]
    [InlineData("missing")]
    [InlineData("another-repository")]
    [InlineData("unrecorded")]
    public async Task LatestWithInvalidUnicodeAndNoAcceptableCacheIsATypedFailure(string cached)
    {
        if (cached == "another-repository")
        {
            await cache.InstallAsync(Old, Tag, "someone/fork");
        }
        else if (cached == "unrecorded")
        {
            InstallByHand(Old);
        }

        server.Serve($"/repos/{Repo}/releases/latest", new ReleaseServer.Answer { Body = "{\"tag_name\":\"\\uD800\"}"u8.ToArray() });

        var error = await Assert.ThrowsAsync<ServiceStartException>(() => Resolve(new ConnectionOptions { DownloadVersion = "latest" }, server.Source()));

        Assert.IsType<InvalidOperationException>(error.InnerException);
        Assert.Equal([$"/repos/{Repo}/releases/latest"], server.Requested);
    }

    [Fact]
    public async Task AReleaseWithoutTheDigestRequiredIsNotDownloaded()
    {
        server.Publish(Tag, New);

        var error = await Assert.ThrowsAsync<ChecksumMismatchException>(() =>
            Resolve(new ConnectionOptions { DownloadVersion = Tag, ExpectedBinarySha256 = TestCache.Sha256(Old) }, server.Source(ReleaseServer.Pinning(Tag, New))));

        Assert.Equal((TestCache.Sha256(Old), TestCache.Sha256(New)), (error.ExpectedSha256, error.ActualSha256));
        Assert.Equal([ReleaseServer.AssetPath(Tag) + ".sha256"], server.Requested);
        Assert.False(File.Exists(Layout.Executable));
    }

    [Theory]
    [InlineData("1")]
    [InlineData("Open-MBEE/OpenSysML")]
    public async Task SameOriginTrustInstallsAnUnpinnedReleaseWithAWarningButExcusesNoContradiction(string allow)
    {
        server.Publish(Tag, New);
        var environment = Environment(new() { [ConnectionOptions.AllowUnpinnedDownloadVariable] = allow });

        var error = await Assert.ThrowsAsync<ChecksumMismatchException>(() =>
            Resolve(new ConnectionOptions { DownloadVersion = Tag, ExpectedBinarySha256 = TestCache.Sha256(Old) }, server.Source(), environment));
        Assert.Equal(TestCache.Sha256(New), error.ActualSha256);
        Assert.False(File.Exists(Layout.Executable));

        var resolved = await Resolve(new ConnectionOptions { DownloadVersion = Tag }, server.Source(), environment);

        Assert.Equal(TestCache.Sha256(New), resolved.Sha256);
        Assert.Contains(Warnings, warning => warning.StartsWith($"This client pins no digest for {ReleaseServer.Asset} of {Tag} of {Repo}; verifying it against the checksum served beside it", StringComparison.Ordinal));
        await Assert.ThrowsAsync<UnpinnedReleaseException>(() =>
            Resolve(new ConnectionOptions { DownloadVersion = "v3.0.0" }, server.Source(), Environment(new() { [ConnectionOptions.AllowUnpinnedDownloadVariable] = "someone/fork" })));
    }

    [Fact]
    public async Task ConcurrentOpensOfOneReleaseDownloadItOnce()
    {
        server.Publish(Tag, New);
        var source = server.Source(ReleaseServer.Pinning(Tag, New));

        var resolved = await Task.WhenAll(Enumerable.Range(0, 8).Select(_ => Task.Run(() => Resolve(new ConnectionOptions { DownloadVersion = Tag }, source), Token)));

        Assert.Single(resolved.Distinct());
        Assert.Single(server.Requested, path => path == ReleaseServer.AssetPath(Tag));
        Assert.Equal(cache.SettledFiles(resolved[0].Path), cache.Files());
    }

    [Fact]
    public async Task ABinaryNamedByTheEnvironmentIsNeverDownloadedAndTheReleaseOptionsAreSaidToBeUnused()
    {
        var path = PathBinary(Old);
        var named = Path.Combine(path, BinaryResolver.ExecutableName);

        var resolved = await Resolve(new ConnectionOptions { DownloadVersion = Tag }, server.Source(), Environment(new() { [ConnectionOptions.BinaryVariable] = named }));

        Assert.Equal(TestCache.Sha256(Old), resolved.Sha256);
        Assert.Empty(server.Requested);
        Assert.Equal(
            [$"Starting {named}, which $OPENSYSML_GRPC_BINARY names: ConnectionOptions.DownloadVersion and GithubRepo describe a release to download, and are not used"],
            Warnings);
    }

    [Fact]
    public async Task AnInvalidVariableIsRefusedBeforeAnythingIsFetched()
    {
        await Assert.ThrowsAsync<InvalidRequestException>(() => Resolve(new ConnectionOptions(), server.Source(), Environment(new() { [ConnectionOptions.GithubRepoVariable] = "../x" })));
        await Assert.ThrowsAsync<InvalidRequestException>(() => Resolve(new ConnectionOptions(), server.Source(), Environment(new() { [ConnectionOptions.VersionVariable] = "v1/../x" })));

        Assert.Empty(server.Requested);
        Assert.False(Directory.Exists(Layout.Directory));
    }

    [Fact]
    [UnsupportedOSPlatform("windows")]
    public async Task ACacheThatCannotBeLockedServesNoReleaseAskedForAndInstallsNothing()
    {
        Assert.SkipUnless(cache.PermissionsAreEnforced(), "This user writes where file permissions forbid it, or this platform has no such permissions.");
        Directory.CreateDirectory(Layout.Directory);
        File.SetUnixFileMode(Layout.Directory, UnixFileMode.UserRead | UnixFileMode.UserExecute);
        var path = PathBinary(New);
        server.Publish(Tag, New);

        var error = await Assert.ThrowsAsync<ServiceStartException>(() =>
            Resolve(new ConnectionOptions { DownloadVersion = Tag }, server.Source(ReleaseServer.Pinning(Tag, New)), Environment(path: path)));
        var resolved = await Resolve(new ConnectionOptions(), server.Source(ReleaseServer.Pinning(BuiltAgainst, New)), Environment(path: path));

        Assert.StartsWith($"sysml-grpc {Tag} of {Repo} was asked for, but {Layout.Lock} could not be locked (", error.Message, StringComparison.Ordinal);
        Assert.Equal(BinaryResolver.Canonical(Path.Combine(path, BinaryResolver.ExecutableName)), resolved.Path);
        Assert.Empty(server.Requested);
    }

    [Fact]
    [UnsupportedOSPlatform("windows")]
    public async Task AnUnlockedCacheCannotContradictItsReleasePin()
    {
        Assert.SkipUnless(cache.PermissionsAreEnforced(), "This user writes where file permissions forbid it, or this platform has no such permissions.");
        await cache.InstallAsync(Old, BuiltAgainst);
        File.SetUnixFileMode(Layout.Guard, UnixFileMode.None);

        var error = await Assert.ThrowsAsync<ChecksumMismatchException>(() => Resolve(new ConnectionOptions(), server.Source(ReleaseServer.Pinning(BuiltAgainst, New))));

        Assert.Equal((TestCache.Sha256(New), TestCache.Sha256(Old)), (error.ExpectedSha256, error.ActualSha256));
        Assert.Equal(Old, File.ReadAllBytes(Layout.Executable));
        Assert.Empty(server.Requested);
    }

    [Fact]
    [UnsupportedOSPlatform("windows")]
    public async Task AReleaseThatCannotBeStagedInTheCacheLeavesItAsItWas()
    {
        Assert.SkipUnless(cache.PermissionsAreEnforced(), "This user writes where file permissions forbid it, or this platform has no such permissions.");
        await cache.InstallAsync(Old, "v1.0.0");
        File.SetUnixFileMode(Layout.Directory, UnixFileMode.UserRead | UnixFileMode.UserExecute);
        server.Publish(Tag, New).Publish(BuiltAgainst, New);
        var pins = ReleaseServer.Pins((Repo, Tag, ReleaseServer.Asset, TestCache.Sha256(New)), (Repo, BuiltAgainst, ReleaseServer.Asset, TestCache.Sha256(New)));

        var error = await Assert.ThrowsAsync<ServiceStartException>(() => Resolve(new ConnectionOptions { DownloadVersion = Tag }, server.Source(pins)));
        var kept = await Resolve(new ConnectionOptions(), server.Source(pins));

        Assert.Equal($"sysml-grpc {Tag} of {Repo} could not be installed in {Layout.Directory}: ", error.Message[..^error.InnerException!.Message.Length]);
        Assert.IsType<UnauthorizedAccessException>(error.InnerException);
        Assert.Equal(new ResolvedBinary(BinaryResolver.Canonical(Layout.Executable), TestCache.Sha256(Old)), kept);
        Assert.Equal(Old, File.ReadAllBytes(Layout.Executable));
        Assert.Contains(Warnings, warning => warning.StartsWith($"Keeping the cached sysml-grpc at {Layout.Executable}: {BuiltAgainst} of {Repo} could not be had (sysml-grpc {BuiltAgainst} of {Repo} could not be installed in", StringComparison.Ordinal));
    }

    public ValueTask InitializeAsync() => ValueTask.CompletedTask;

    public async ValueTask DisposeAsync()
    {
        await server.DisposeAsync();
        cache.Dispose();
    }

    private Task<ResolvedBinary> Resolve(ConnectionOptions options, ReleaseSource source, Func<string, string?>? environment = null) =>
        BinaryResolver.ResolveAsync(
            new ConnectionOptions
            {
                DownloadVersion = options.DownloadVersion,
                GithubRepo = options.GithubRepo,
                ExpectedBinarySha256 = options.ExpectedBinarySha256,
                Logger = logger,
            },
            environment ?? Environment(),
            source,
            Token);

    private Func<string, string?> Environment(Dictionary<string, string?>? variables = null, string? path = null)
    {
        var all = new Dictionary<string, string?>(variables ?? []) { ["HOME"] = cache.Root, ["USERPROFILE"] = cache.Root, ["PATH"] = path };
        return name => all.GetValueOrDefault(name);
    }

    private void InstallByHand(byte[] bytes)
    {
        Directory.CreateDirectory(Layout.Directory);
        File.WriteAllBytes(Layout.Executable, bytes);
        if (!OperatingSystem.IsWindows())
        {
            File.SetUnixFileMode(Layout.Executable, UnixFileMode.UserRead | UnixFileMode.UserExecute);
        }
    }

    // A directory holding an executable sysml-grpc, to put on PATH.
    private string PathBinary(byte[] bytes)
    {
        var directory = Directory.CreateDirectory(Path.Combine(cache.Root, "path-" + Guid.NewGuid().ToString("N"))).FullName;
        var file = Path.Combine(directory, BinaryResolver.ExecutableName);
        File.WriteAllBytes(file, bytes);
        if (!OperatingSystem.IsWindows())
        {
            File.SetUnixFileMode(file, UnixFileMode.UserRead | UnixFileMode.UserExecute);
        }

        return directory;
    }
}
