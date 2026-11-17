using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Text;
using System.Threading;
using System.Threading.Tasks;
using Microsoft.Extensions.Logging.Abstractions;
using OpenMBEE.OpenSysML.Internal.Provisioning;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// The shared cache's lock against the Python client's, taken by a holder making that client's exact calls
/// (test/interop/lock_holder.py): each shuts the other out, in both directions, waits for the other's release, and
/// is released by the kernel when its holder is killed. The Java and Rust clients take the same lock, an
/// <c>fcntl</c> lock over the whole file on POSIX and a byte-range lock from its start on Windows, so the Python
/// client's calls stand for theirs, as they do in the Rust client's own test of its lock.
/// </summary>
[Trait(Category.Name, Category.Interop)]
public sealed class CacheInteropTests : IDisposable
{
    private static readonly byte[] Release = Encoding.ASCII.GetBytes("#!/bin/sh\necho a release\n");

    private readonly TestCache cache = new();

    private static CancellationToken Token => TestContext.Current.CancellationToken;

    private CacheLayout Layout => cache.Layout;

    [Fact]
    public async Task WhileThisClientHoldsTheLockTheOtherCannotTakeItUntilItIsReleased()
    {
        await using (await Acquire(TimeSpan.Zero))
        {
            Assert.False(await ForeignLock.IsFreeAsync(Layout.Lock), $"the Python client took the lock the .NET client held");
        }

        Assert.True(await ForeignLock.IsFreeAsync(Layout.Lock), $"the Python client could not take the lock once the .NET client released it");
    }

    [Fact]
    public async Task WhileTheOtherHoldsTheLockThisClientWaitsAndTakesItPromptlyOnRelease()
    {
        await using var holder = await ForeignLock.HoldAsync(Layout.Lock);

        var error = await Assert.ThrowsAsync<ServiceStartException>(() => Acquire(TimeSpan.Zero));
        Assert.Contains("is locked by another process", error.Message, StringComparison.Ordinal);
        var waiting = Acquire(TimeSpan.FromSeconds(30));
        await Task.Delay(500, Token);
        Assert.False(waiting.IsCompleted, $"the .NET client took the lock the Python client held");

        var released = Stopwatch.StartNew();
        await holder.ReleaseAsync();
        await using var taken = await waiting.WaitAsync(TimeSpan.FromSeconds(10), Token);
        Assert.True(released.Elapsed < TimeSpan.FromSeconds(3), $"the .NET client saw the Python client's release after {released.Elapsed}");
    }

    [Fact]
    public async Task TheOtherClientWaitingForThisOneTakesTheLockOnceItIsReleased()
    {
        await using var held = await Acquire(TimeSpan.Zero);
        await using var waiting = ForeignLock.Start(Layout.Lock);
        Assert.False(await waiting.HeldWithinAsync(TimeSpan.FromMilliseconds(1500)), $"the Python client took the lock the .NET client held");

        await held.DisposeAsync();
        Assert.True(await waiting.HeldWithinAsync(TimeSpan.FromSeconds(30)), $"the Python client did not take the lock once the .NET client released it");
        Assert.False(await ForeignLock.IsFreeAsync(Layout.Lock));
        await waiting.ReleaseAsync();
    }

    [Fact]
    public async Task AKilledHolderOfEitherClientReleasesTheLockToTheOther()
    {
        var foreign = await ForeignLock.HoldAsync(Layout.Lock);
        await using (foreign)
        {
            foreign.Kill();
        }

        await using (await Acquire(TimeSpan.FromSeconds(5)))
        {
        }

        var dotnet = await cache.HeldByAnotherProcessAsync();
        await using (dotnet)
        {
            Assert.False(await ForeignLock.IsFreeAsync(Layout.Lock));
            dotnet.KillProbe();
        }

        Assert.True(await ForeignLock.IsFreeAsync(Layout.Lock), $"the Python client could not take the lock of a killed .NET holder");
    }

    [Fact]
    public async Task ATransactionHoldsTheLockAgainstTheOtherClientAtEveryStep()
    {
        var checkedSteps = new List<CacheStep>();
        var seams = new CacheSeams
        {
            Reached = async (step, _) =>
            {
                Assert.False(await ForeignLock.IsFreeAsync(Layout.Lock), $"the Python client took the lock at {step}");
                checkedSteps.Add(step);
            },
        };

        await using (var transaction = await CacheTransaction.BeginAsync(Layout, NullLogger.Instance, Token, seams: seams))
        {
            await transaction.InspectAsync(Token);
            await transaction.InstallAsync(TestCache.Writing(Release), TestCache.Sha256(Release), "v1.0.0", "Open-MBEE/OpenSysML", Token);
            await transaction.SnapshotAsync(Token);
        }

        Assert.Equal(Enum.GetValues<CacheStep>(), checkedSteps);
        Assert.True(await ForeignLock.IsFreeAsync(Layout.Lock));
    }

    [Fact]
    public async Task ACopyOfThisClientInAnotherLoadContextHoldsTheLockAgainstTheOtherClient()
    {
        await using var probe = ProbeRun.Start(ProbeRun.Cache, "copies", Layout.Directory);
        Assert.Equal("held", (await probe.ReadAsync()).GetProperty("event").GetString());

        Assert.False(await ForeignLock.IsFreeAsync(Layout.Lock), $"the Python client took the lock a collectible copy of the .NET client held");

        await probe.SendLineAsync();
        var events = await probe.ReadToExitAsync();
        Assert.False(events[0].GetProperty("free").GetBoolean());
        Assert.True(await ForeignLock.IsFreeAsync(Layout.Lock));
    }

    [Fact]
    public async Task TheRecordIsReadAndWrittenAsThePythonClientReadsAndWritesIt()
    {
        var python = InteropGate.FromProcess.Require();
        await cache.InstallAsync(Release, "v1.2.3", "Open-MBEE/OpenSysML");

        // cached_release reads with json.load; write_metadata writes with json.dump.
        Assert.Equal(
            $"{{\"repo\": \"Open-MBEE/OpenSysML\", \"sha256\": \"{TestCache.Sha256(Release)}\", \"version\": \"v1.2.3\"}}",
            await RunPython(python, "import json, sys; print(json.dumps(json.load(open(sys.argv[1])), sort_keys=True))", Layout.Metadata));
        await RunPython(
            python,
            "import json, sys; f = open(sys.argv[1], 'w'); json.dump({'version': 'v9.9.9-é', 'sha256': sys.argv[2], 'repo': 'fork/OpenSysML'}, f); f.close()",
            Layout.Metadata,
            TestCache.Sha256(Release));

        Assert.Equal(new CacheMetadata("v9.9.9-é", TestCache.Sha256(Release), "fork/OpenSysML"), CacheMetadata.Read(Layout.Metadata).Metadata);
        Assert.Equal(new CacheMetadata("v9.9.9-é", TestCache.Sha256(Release), "fork/OpenSysML").Format(), File.ReadAllBytes(Layout.Metadata));
    }

    public void Dispose() => cache.Dispose();

    private static async Task<string> RunPython(string python, string program, params string[] arguments)
    {
        var start = new ProcessStartInfo(python) { RedirectStandardOutput = true, RedirectStandardError = true, UseShellExecute = false };
        start.ArgumentList.Add("-c");
        start.ArgumentList.Add(program);
        foreach (var argument in arguments)
        {
            start.ArgumentList.Add(argument);
        }

        using var process = Process.Start(start)!;
        var output = process.StandardOutput.ReadToEndAsync(Token);
        var error = process.StandardError.ReadToEndAsync(Token);
        await process.WaitForExitAsync(Token);
        Assert.True(process.ExitCode == 0, $"python failed: {await error}");
        return (await output).Trim();
    }

    private Task<CacheLock> Acquire(TimeSpan wait) => CacheLock.AcquireAsync(Layout, wait, NullLogger.Instance, Token);
}
