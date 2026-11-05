using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Text;
using System.Threading;
using System.Threading.Tasks;
using Microsoft.Extensions.Logging.Abstractions;
using OpenMBEE.OpenSysML.Internal;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// The shared cache's lock against the Python, Java and Rust clients' own, each taken by a holder making that
/// client's exact calls (test/interop): each shuts the other out, in both directions, waits for the other's
/// release, and is released by the kernel when its holder is killed.
/// </summary>
[Trait(Category.Name, Category.Interop)]
public sealed class CacheInteropTests : IDisposable
{
    private static readonly byte[] Release = Encoding.ASCII.GetBytes("#!/bin/sh\necho a release\n");

    private readonly TestCache cache = new();

    private static CancellationToken Token => TestContext.Current.CancellationToken;

    private CacheLayout Layout => cache.Layout;

    [Theory]
    [MemberData(nameof(InteropGate.All), MemberType = typeof(InteropGate))]
    public async Task WhileThisClientHoldsTheLockTheOtherCannotTakeItUntilItIsReleased(ForeignClient client)
    {
        await using (await Acquire(TimeSpan.Zero))
        {
            Assert.False(await ForeignLock.IsFreeAsync(client, Layout.Lock), $"the {client} client took the lock the .NET client held");
        }

        Assert.True(await ForeignLock.IsFreeAsync(client, Layout.Lock), $"the {client} client could not take the lock once the .NET client released it");
    }

    [Theory]
    [MemberData(nameof(InteropGate.All), MemberType = typeof(InteropGate))]
    public async Task WhileTheOtherHoldsTheLockThisClientWaitsAndTakesItPromptlyOnRelease(ForeignClient client)
    {
        await using var holder = await ForeignLock.HoldAsync(client, Layout.Lock);

        var error = await Assert.ThrowsAsync<ServiceStartException>(() => Acquire(TimeSpan.Zero));
        Assert.Contains("is locked by another process", error.Message, StringComparison.Ordinal);
        var waiting = Acquire(TimeSpan.FromSeconds(30));
        await Task.Delay(500, Token);
        Assert.False(waiting.IsCompleted, $"the .NET client took the lock the {client} client held");

        var released = Stopwatch.StartNew();
        await holder.ReleaseAsync();
        await using var taken = await waiting.WaitAsync(TimeSpan.FromSeconds(10), Token);
        Assert.True(released.Elapsed < TimeSpan.FromSeconds(3), $"the .NET client saw the {client} client's release after {released.Elapsed}");
    }

    [Theory]
    [MemberData(nameof(InteropGate.All), MemberType = typeof(InteropGate))]
    public async Task TheOtherClientWaitingForThisOneTakesTheLockOnceItIsReleased(ForeignClient client)
    {
        await using var held = await Acquire(TimeSpan.Zero);
        await using var waiting = await ForeignLock.StartAsync(client, Layout.Lock);
        Assert.False(await waiting.HeldWithinAsync(TimeSpan.FromMilliseconds(1500)), $"the {client} client took the lock the .NET client held");

        await held.DisposeAsync();
        Assert.True(await waiting.HeldWithinAsync(TimeSpan.FromSeconds(30)), $"the {client} client did not take the lock once the .NET client released it");
        Assert.False(await ForeignLock.IsFreeAsync(client, Layout.Lock));
        await waiting.ReleaseAsync();
    }

    [Theory]
    [MemberData(nameof(InteropGate.All), MemberType = typeof(InteropGate))]
    public async Task AKilledHolderOfEitherClientReleasesTheLockToTheOther(ForeignClient client)
    {
        var foreign = await ForeignLock.HoldAsync(client, Layout.Lock);
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
            Assert.False(await ForeignLock.IsFreeAsync(client, Layout.Lock));
            dotnet.KillProbe();
        }

        Assert.True(await ForeignLock.IsFreeAsync(client, Layout.Lock), $"the {client} client could not take the lock of a killed .NET holder");
    }

    [Theory]
    [MemberData(nameof(InteropGate.All), MemberType = typeof(InteropGate))]
    public async Task ATransactionHoldsTheLockAgainstTheOtherClientAtEveryStep(ForeignClient client)
    {
        var checkedSteps = new List<CacheStep>();
        var seams = new CacheSeams
        {
            Reached = async (step, _) =>
            {
                Assert.False(await ForeignLock.IsFreeAsync(client, Layout.Lock), $"the {client} client took the lock at {step}");
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
        Assert.True(await ForeignLock.IsFreeAsync(client, Layout.Lock));
    }

    [Theory]
    [MemberData(nameof(InteropGate.All), MemberType = typeof(InteropGate))]
    public async Task ACopyOfThisClientInAnotherLoadContextHoldsTheLockAgainstTheOtherClient(ForeignClient client)
    {
        await using var probe = ProbeRun.Start(ProbeRun.Cache, "copies", Layout.Directory);
        Assert.Equal("held", (await probe.ReadAsync()).GetProperty("event").GetString());

        Assert.False(await ForeignLock.IsFreeAsync(client, Layout.Lock), $"the {client} client took the lock a collectible copy of the .NET client held");

        await probe.SendLineAsync();
        var events = await probe.ReadToExitAsync();
        Assert.False(events[0].GetProperty("free").GetBoolean());
        Assert.True(await ForeignLock.IsFreeAsync(client, Layout.Lock));
    }

    [Fact]
    public async Task TheRecordIsReadAndWrittenAsThePythonClientReadsAndWritesIt()
    {
        var python = InteropGate.FromProcess.Require(ForeignClient.Python);
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
