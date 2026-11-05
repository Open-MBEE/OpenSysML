using System;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Microsoft.Extensions.Logging;
using Microsoft.Extensions.Logging.Abstractions;
using OpenMBEE.OpenSysML.Internal;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// The shared cache's lock against threads of this copy of the client, other copies in this process, other
/// processes using this client, and another process holding the cross-client lock alone as the Python, Java
/// and Rust clients do: waiting, giving up, cancelling, crashing, and what a started process inherits.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed class CacheLockTests : IDisposable
{
    private readonly TestCache cache = new();

    private static CancellationToken Token => TestContext.Current.CancellationToken;

    private CacheLayout Layout => cache.Layout;

    [Fact]
    public void PausesBetweenAttemptsDoubleFromTenMillisecondsToAQuarterSecond()
    {
        var backoff = default(CacheLock.Backoff);

        Assert.Equal([10, 20, 40, 80, 160, 250, 250, 250], Enumerable.Range(0, 8).Select(_ => backoff.Next().TotalMilliseconds).ToArray());
    }

    [Fact]
    public async Task TakingTheLockMakesTheCacheAndBothLockFilesWhichOutliveIt()
    {
        await using (await Acquire(TimeSpan.Zero))
        {
            Assert.True(File.Exists(Layout.Lock));
            Assert.True(File.Exists(Layout.Guard));
            if (!OperatingSystem.IsWindows())
            {
                Assert.Equal(UnixFileMode.UserRead | UnixFileMode.UserWrite, File.GetUnixFileMode(Layout.Lock));
                Assert.Equal(UnixFileMode.UserRead | UnixFileMode.UserWrite, File.GetUnixFileMode(Layout.Guard));
            }
        }

        Assert.Equal([Path.GetFileName(Layout.Lock), Path.GetFileName(Layout.Guard)], cache.Files());
        await using (await Acquire(TimeSpan.Zero))
        {
        }
    }

    [Fact]
    public async Task AnotherThreadWaitsForTheLockAndTakesItAsSoonAsItIsReleased()
    {
        var first = await Acquire(TimeSpan.Zero);
        var second = Acquire(TimeSpan.FromSeconds(30));
        await Task.Delay(300, Token);
        Assert.False(second.IsCompleted, "a second holder took the lock while the first held it");

        var released = Stopwatch.StartNew();
        await first.DisposeAsync();
        await using var taken = await second.WaitAsync(TimeSpan.FromSeconds(5), Token);

        Assert.True(released.Elapsed < TimeSpan.FromSeconds(2), $"the waiting thread took {released.Elapsed} to see the release");
    }

    [Fact]
    public async Task TryingOnceWhileAnotherThreadHoldsItFailsAtOnceNamingTheLock()
    {
        await using var held = await Acquire(TimeSpan.Zero);

        var watch = Stopwatch.StartNew();
        var error = await Assert.ThrowsAsync<ServiceStartException>(() => Acquire(TimeSpan.Zero));

        Assert.True(watch.Elapsed < TimeSpan.FromSeconds(1));
        Assert.Contains($"is locked by another thread of this process, which held {Layout.Lock}", error.Message, StringComparison.Ordinal);
        Assert.Contains("fix: ", error.Message, StringComparison.Ordinal);
    }

    [Fact]
    public async Task DisposingTwiceReleasesOnceSoItNeverLetsTwoHoldersIn()
    {
        var first = await Acquire(TimeSpan.Zero);
        first.Dispose();
        await using var second = await Acquire(TimeSpan.Zero);

        first.Dispose();
        await first.DisposeAsync();

        await Assert.ThrowsAsync<ServiceStartException>(() => Acquire(TimeSpan.Zero));
    }

    [Fact]
    public async Task AProcessUsingThisClientIsShutOutBothWays()
    {
        await using (await Acquire(TimeSpan.Zero))
        {
            Assert.False(await cache.IsFreeToAnotherProcessAsync(), "another process took the lock this one held");
            Assert.False(await cache.IsSharedLockFreeAsync(), "the cross-client lock was free while this process held the cache");
        }

        Assert.True(await cache.IsFreeToAnotherProcessAsync());
        Assert.True(await cache.IsSharedLockFreeAsync());

        await using var holder = await cache.HeldByAnotherProcessAsync();
        var error = await Assert.ThrowsAsync<ServiceStartException>(() => Acquire(TimeSpan.Zero));
        Assert.Contains("locked by another copy of the .NET client", error.Message, StringComparison.Ordinal);

        await TestCache.ReleaseAsync(holder);
        await using (await Acquire(TimeSpan.Zero))
        {
        }
    }

    [Fact]
    public async Task AWaitForAnotherClientEndsWhenItReleasesTheLock()
    {
        await using var holder = await cache.HeldByAnotherClientAsync();
        var waiting = Acquire(TimeSpan.FromSeconds(30));
        await Task.Delay(500, Token);
        Assert.False(waiting.IsCompleted, "the lock was taken while another client held it");

        var released = Stopwatch.StartNew();
        await TestCache.ReleaseAsync(holder);
        await using var taken = await waiting.WaitAsync(TimeSpan.FromSeconds(5), Token);

        Assert.True(released.Elapsed < TimeSpan.FromSeconds(2), $"the release was seen after {released.Elapsed}");
    }

    [Fact]
    public async Task ACancelledWaitLeavesNothingHeld()
    {
        await using var holder = await cache.HeldByAnotherClientAsync();
        using var cancel = CancellationTokenSource.CreateLinkedTokenSource(Token);
        cancel.CancelAfter(300);

        var error = await Assert.ThrowsAnyAsync<OperationCanceledException>(() => CacheLock.AcquireAsync(Layout, TimeSpan.FromSeconds(30), NullLogger.Instance, cancel.Token));
        Assert.Equal(cancel.Token, error.CancellationToken);

        // The guard it took while waiting for the cross-client lock, and its place among this copy's threads, are given up.
        Assert.False(await cache.IsFreeToAnotherProcessAsync());
        await TestCache.ReleaseAsync(holder);
        Assert.True(await cache.IsFreeToAnotherProcessAsync());
        await using (await Acquire(TimeSpan.Zero))
        {
        }
    }

    [Fact]
    public async Task ACancelledWaitForAnotherThreadLeavesNothingHeld()
    {
        var first = await Acquire(TimeSpan.Zero);
        using var cancel = CancellationTokenSource.CreateLinkedTokenSource(Token);
        cancel.CancelAfter(200);

        await Assert.ThrowsAnyAsync<OperationCanceledException>(() => CacheLock.AcquireAsync(Layout, TimeSpan.FromSeconds(30), NullLogger.Instance, cancel.Token));

        await first.DisposeAsync();
        await using (await Acquire(TimeSpan.Zero))
        {
        }
    }

    [Fact]
    public async Task WaitingGivesUpAfterTheWaitAndWarnsOnceAfterTwoSeconds()
    {
        await using var holder = await cache.HeldByAnotherClientAsync();
        var logger = new ListLogger();

        var watch = Stopwatch.StartNew();
        var error = await Assert.ThrowsAsync<ServiceStartException>(() => CacheLock.AcquireAsync(Layout, TimeSpan.FromSeconds(2.5), logger, Token));

        Assert.True(watch.Elapsed >= TimeSpan.FromSeconds(2.5), $"gave up after {watch.Elapsed}");
        Assert.Equal(
            $"The shared sysml-grpc cache at {Layout.Directory} is locked by another process, which held {Layout.Lock} for all of the 2.5 s waited.",
            error.Message.Split('\n')[0]);
        var warning = Assert.Single(logger.Entries, entry => entry.Level == LogLevel.Warning);
        Assert.Equal($"Waiting for {Layout.Lock}, which another process holds while it changes the shared sysml-grpc cache", warning.Message);

        await TestCache.ReleaseAsync(holder);
        await using (await Acquire(TimeSpan.Zero))
        {
        }
    }

    [Fact]
    public async Task AHolderThatIsKilledReleasesEverythingItHeld()
    {
        var holder = await cache.HeldByAnotherProcessAsync();
        await using (holder)
        {
            holder.KillProbe();
        }

        Assert.True(await cache.IsSharedLockFreeAsync());
        await using (await Acquire(TimeSpan.Zero))
        {
        }

        var client = await cache.HeldByAnotherClientAsync();
        await using (client)
        {
            client.KillProbe();
        }

        await using (await Acquire(TimeSpan.Zero))
        {
        }
    }

    [Fact]
    public async Task AProcessStartedWhileTheLockIsHeldKeepsNoneOfItWhenTheHolderIsKilled()
    {
        await using var probe = ProbeRun.Start(ProbeRun.Cache, "hold-and-start", Layout.Directory);
        var held = await probe.ReadAsync();
        var child = held.GetProperty("siblings")[0].GetInt32();

        probe.KillProbe();

        Assert.True(FakeChild.IsRunning(child), "the process the holder started ended with it");
        if (OperatingSystem.IsLinux())
        {
            var open = Directory.EnumerateFileSystemEntries($"/proc/{child}/fd").Select(fd => new FileInfo(fd).LinkTarget).ToList();
            Assert.DoesNotContain(Layout.Lock, open);
            Assert.DoesNotContain(Layout.Guard, open);
        }

        Assert.True(await cache.IsFreeToAnotherProcessAsync(), "a process started by a killed holder kept the lock");
        Assert.True(await cache.IsSharedLockFreeAsync());
        Assert.True(FakeChild.IsRunning(child), "the started process ended before its holds were checked");
    }

    [Fact]
    public async Task TwoCopiesOfTheClientInOneProcessShutEachOtherOutAndAreCollected()
    {
        await using var probe = ProbeRun.Start(ProbeRun.Cache, "copies", Layout.Directory);
        var held = await probe.ReadAsync();
        Assert.Equal(("held", 0), (held.GetProperty("event").GetString(), held.GetProperty("copy").GetInt32()));
        Assert.False(await cache.IsSharedLockFreeAsync(), "the copy holding the cache does not hold the cross-client lock");

        await probe.SendLineAsync();
        var events = await probe.ReadToExitAsync();

        string[] Step(int i) => [events[i].GetProperty("event").GetString()!, events[i].GetProperty("copy").GetInt32().ToString(System.Globalization.CultureInfo.InvariantCulture)];
        Assert.Equal(["tried", "1"], Step(0));
        Assert.False(events[0].GetProperty("free").GetBoolean(), "a second copy of the client took the lock the first held in the same process");
        Assert.NotEqual(held.GetProperty("context").GetString(), events[0].GetProperty("context").GetString());
        Assert.Equal(["released", "0"], Step(1));
        Assert.Equal(["tried", "1"], Step(2));
        Assert.True(events[2].GetProperty("free").GetBoolean(), "the second copy could not take the lock once the first released it");
        Assert.Equal(["tried", "0"], Step(3));
        Assert.False(events[3].GetProperty("free").GetBoolean(), "the first copy took the lock the second held");
        Assert.Equal(["released", "1"], Step(4));
        var collected = events[5];
        Assert.Equal("collected", collected.GetProperty("event").GetString());
        Assert.All(collected.GetProperty("copies").EnumerateArray(), copy => Assert.True(copy.GetBoolean(), "a copy of the client was not collected"));
        Assert.False(collected.GetProperty("libraryInDefault").GetBoolean());
        Assert.True(await cache.IsSharedLockFreeAsync());
    }

    [Fact]
    public async Task ACacheThatCannotBeMadeIsAnInputOutputFailureThatHoldsNothing()
    {
        File.WriteAllBytes(Path.Combine(cache.Root, ".opensysml"), []);

        await Assert.ThrowsAnyAsync<IOException>(() => Acquire(TimeSpan.Zero));

        File.Delete(Path.Combine(cache.Root, ".opensysml"));
        await using (await Acquire(TimeSpan.Zero))
        {
        }
    }

    public void Dispose() => cache.Dispose();

    private Task<CacheLock> Acquire(TimeSpan wait) => CacheLock.AcquireAsync(Layout, wait, NullLogger.Instance, Token);
}
