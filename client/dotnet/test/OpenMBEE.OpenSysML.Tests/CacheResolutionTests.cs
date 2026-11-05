using System;
using System.Collections.Generic;
using System.IO;
using System.Runtime.Versioning;
using System.Text;
using System.Threading;
using System.Threading.Tasks;
using Microsoft.Extensions.Logging;
using OpenMBEE.OpenSysML.Internal;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// A private service started from the shared cache: the cache is read with its lock held, and what starts is
/// its executable linked under its digest, which no install over the cache afterwards changes.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed class CacheResolutionTests : IDisposable
{
    private static readonly byte[] First = Encoding.ASCII.GetBytes("#!/bin/sh\necho first release\n");
    private static readonly byte[] Second = Encoding.ASCII.GetBytes("#!/bin/sh\necho second release\n");

    private readonly TestCache cache = new();

    private static CancellationToken Token => TestContext.Current.CancellationToken;

    private CacheLayout Layout => cache.Layout;

    [Fact]
    public async Task TheSharedCacheIsStartedFromItsExecutableLinkedUnderItsDigest()
    {
        await cache.InstallAsync(First, ReleaseRequest.BuiltAgainst);
        var logger = new ListLogger();

        var first = await Resolve(new ConnectionOptions { Logger = logger });

        Assert.Equal(new ResolvedBinary(BinaryResolver.Canonical(Layout.Snapshot(TestCache.Sha256(First))), TestCache.Sha256(First)), first);
        Assert.Contains(logger.Entries, entry => entry.Level == LogLevel.Debug
            && entry.Message == $"Starting the shared cache's {Layout.Executable} as {first.Path} (Linked)");

        await cache.InstallAsync(Second, ReleaseRequest.BuiltAgainst);
        var second = await Resolve(new ConnectionOptions());

        Assert.Equal(First, File.ReadAllBytes(first.Path));
        Assert.Equal(new ResolvedBinary(BinaryResolver.Canonical(Layout.Snapshot(TestCache.Sha256(Second))), TestCache.Sha256(Second)), second);
        Assert.Equal(cache.SettledFiles(first.Path, second.Path), cache.Files());
        Assert.DoesNotContain(logger.Entries, entry => entry.Level >= LogLevel.Warning);
    }

    [Fact]
    public async Task ASymbolicLinkSnapshotDoesNotResolveBackToTheMutableCache()
    {
        Assert.SkipWhen(OperatingSystem.IsWindows(), "Creating links needs privileges Windows runners may lack.");
        await cache.InstallAsync(First);
        var digest = TestCache.Sha256(First);
        var snapshot = Layout.Snapshot(digest);
        File.CreateSymbolicLink(snapshot, Path.GetFileName(Layout.Executable));

        var resolved = await Resolve(new ConnectionOptions { ExpectedBinarySha256 = digest });

        Assert.Equal(Path.Combine(BinaryResolver.Canonical(Layout.Directory), Path.GetFileName(snapshot)), resolved.Path);
        await cache.InstallAsync(Second);
        Assert.Equal(First, File.ReadAllBytes(resolved.Path));
        Assert.Equal(resolved.Sha256, await BinaryResolver.Sha256Async(resolved.Path, Token));
    }

    [Fact]
    public async Task AnExecutableInstalledByHandWithoutARecordIsStartedTheSameWay()
    {
        Directory.CreateDirectory(Layout.Directory);
        File.WriteAllBytes(Layout.Executable, First);
        if (!OperatingSystem.IsWindows())
        {
            File.SetUnixFileMode(Layout.Executable, UnixFileMode.UserRead | UnixFileMode.UserExecute);
        }

        var resolved = await Resolve(new ConnectionOptions());

        Assert.Equal(new ResolvedBinary(BinaryResolver.Canonical(Layout.Snapshot(TestCache.Sha256(First))), TestCache.Sha256(First)), resolved);
        Assert.False(File.Exists(Layout.Metadata));
    }

    [Fact]
    public async Task ADigestRequiredOfTheCacheIsCheckedAgainstWhatWouldStart()
    {
        await cache.InstallAsync(First, ReleaseRequest.BuiltAgainst);
        var snapshot = BinaryResolver.Canonical(Layout.Snapshot(TestCache.Sha256(First)));

        var error = await Assert.ThrowsAsync<ChecksumMismatchException>(() => Resolve(new ConnectionOptions { ExpectedBinarySha256 = TestCache.Sha256(Second) }));

        Assert.Equal((snapshot, TestCache.Sha256(Second), TestCache.Sha256(First)), (error.BinaryPath, error.ExpectedSha256, error.ActualSha256));
        Assert.Equal(snapshot, (await Resolve(new ConnectionOptions { ExpectedBinarySha256 = TestCache.Sha256(First).ToUpperInvariant() })).Path);
    }

    [Fact]
    public async Task ResolvingTheCacheWaitsForItsLockAndTheWaitCanBeCancelled()
    {
        await cache.InstallAsync(First, ReleaseRequest.BuiltAgainst);
        await using var holder = await cache.HeldByAnotherClientAsync();
        using var cancel = CancellationTokenSource.CreateLinkedTokenSource(Token);
        cancel.CancelAfter(300);

        await Assert.ThrowsAnyAsync<OperationCanceledException>(() => BinaryResolver.ResolveAsync(new ConnectionOptions(), Environment(), ReleaseServer.Offline(), cancel.Token));
        Assert.False(File.Exists(Layout.Snapshot(TestCache.Sha256(First))), "the cache was linked while another client held its lock");

        var waiting = Resolve(new ConnectionOptions());
        await Task.Delay(300, Token);
        Assert.False(waiting.IsCompleted);
        await TestCache.ReleaseAsync(holder);

        Assert.Equal(TestCache.Sha256(First), (await waiting.WaitAsync(TimeSpan.FromSeconds(10), Token)).Sha256);
    }

    [Fact]
    [UnsupportedOSPlatform("windows")]
    public async Task ACacheThatCannotBeLockedIsStartedAsItIsWithAWarning()
    {
        Assert.SkipUnless(cache.PermissionsAreEnforced(), "This user writes where file permissions forbid it, or this platform has no such permissions.");
        Directory.CreateDirectory(Layout.Directory);
        File.WriteAllBytes(Layout.Executable, First);
        File.SetUnixFileMode(Layout.Executable, UnixFileMode.UserRead | UnixFileMode.UserExecute);
        File.SetUnixFileMode(Layout.Directory, UnixFileMode.UserRead | UnixFileMode.UserExecute);
        var logger = new ListLogger();

        var resolved = await Resolve(new ConnectionOptions { Logger = logger });

        Assert.Equal(new ResolvedBinary(BinaryResolver.Canonical(Layout.Executable), TestCache.Sha256(First)), resolved);
        var warning = Assert.Single(logger.Entries, entry => entry.Level == LogLevel.Warning);
        Assert.StartsWith($"Could not lock {Layout.Lock} (", warning.Message, StringComparison.Ordinal);
        Assert.EndsWith($"); using {Layout.Executable} unlocked, so a release another client installs over it may be started instead", warning.Message, StringComparison.Ordinal);
    }

    [Fact]
    public async Task TheCacheExecutableNamedExplicitlyIsStartedAsNamedWithoutTouchingTheCache()
    {
        await cache.InstallAsync(First);
        File.Delete(Layout.Lock);
        File.Delete(Layout.Guard);

        var resolved = await Resolve(new ConnectionOptions { BinaryPath = Layout.Executable });

        Assert.Equal(new ResolvedBinary(BinaryResolver.Canonical(Layout.Executable), TestCache.Sha256(First)), resolved);
        Assert.Equal([Path.GetFileName(Layout.Executable), Path.GetFileName(Layout.Metadata)], cache.Files());
    }

    [Fact]
    public async Task APrivateServiceFromTheSharedCacheRunsItsSnapshot()
    {
        using var child = FakeChild.Serving();
        Directory.CreateDirectory(Layout.Directory);
        foreach (var file in Directory.EnumerateFiles(child.Directory))
        {
            File.Copy(file, Path.Combine(Layout.Directory, Path.GetFileName(file)));
        }

        var digest = TestCache.Sha256(File.ReadAllBytes(Layout.Executable));
        using var handler = new FakeServiceHandler();
        var registry = new ServiceRegistry(PrivateService.StartAsync);
        var options = new ConnectionOptions { HttpMessageHandler = handler, ShutdownTimeout = TimeSpan.FromSeconds(10) };

        var connection = await Connection.OpenAsync(options, Environment(), registry, ReleaseServer.Offline(), Token);
        var entry = Assert.Single(registry.Entries());
        await connection.DisposeAsync();

        Assert.Equal((BinaryResolver.Canonical(Layout.Snapshot(digest)), digest), (entry.Identity.Path, entry.Identity.Sha256));
        Assert.Equal(PrivateService.Arguments, await File.ReadAllLinesAsync(child.ArgumentsFile, Token));
        await FakeChild.AssertGoneAsync(entry.ProcessId!.Value);
    }

    [Fact]
    public async Task ACacheRemovedWhileItsLockWasAwaitedIsNotStartedAndPathIsUsedInstead()
    {
        await cache.InstallAsync(First, ReleaseRequest.BuiltAgainst);
        var path = Path.Combine(cache.Root, "path");
        Directory.CreateDirectory(path);
        var onPath = Path.Combine(path, BinaryResolver.ExecutableName);
        File.WriteAllBytes(onPath, Second);
        if (!OperatingSystem.IsWindows())
        {
            File.SetUnixFileMode(onPath, UnixFileMode.UserRead | UnixFileMode.UserExecute);
        }

        var logger = new ListLogger();
        await using var holder = await cache.HeldByAnotherClientAsync();
        var waiting = BinaryResolver.ResolveAsync(new ConnectionOptions { Logger = logger }, Environment(path), ReleaseServer.Offline(), Token);
        await Task.Delay(300, Token);
        File.Delete(Layout.Executable);
        await TestCache.ReleaseAsync(holder);

        Assert.Equal(new ResolvedBinary(BinaryResolver.Canonical(onPath), TestCache.Sha256(Second)), await waiting.WaitAsync(TimeSpan.FromSeconds(10), Token));
        var warning = Assert.Single(logger.Entries, entry => entry.Level == LogLevel.Warning);
        Assert.StartsWith($"Starting {onPath} from $PATH: sysml-grpc {ReleaseRequest.BuiltAgainst} of {ReleaseServer.Repo} could not be had (sysml-grpc {ReleaseRequest.BuiltAgainst} of {ReleaseServer.Repo} was not downloaded: this client pins no SHA-256 digest", warning.Message, StringComparison.Ordinal);
        Assert.False(File.Exists(Layout.Snapshot(TestCache.Sha256(First))), "the removed executable's snapshot was made");
    }

    public void Dispose() => cache.Dispose();

    private Func<string, string?> Environment(string? path = null)
    {
        var variables = new Dictionary<string, string?> { ["HOME"] = cache.Root, ["USERPROFILE"] = cache.Root, ["PATH"] = path };
        return name => variables.GetValueOrDefault(name);
    }

    // Nothing here downloads: a source refusing every connection, and pinning nothing, proves it.
    private Task<ResolvedBinary> Resolve(ConnectionOptions options) => BinaryResolver.ResolveAsync(options, Environment(), ReleaseServer.Offline(), Token);
}
