using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Microsoft.Extensions.Logging;
using Microsoft.Extensions.Logging.Abstractions;
using OpenMBEE.OpenSysML.Internal;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>The registry's state machine, over in-memory services: sharing, reservations, failure, death and stopping.</summary>
[Trait(Category.Name, Category.Unit)]
public sealed class ServiceRegistryTests
{
    private static CancellationToken Token => TestContext.Current.CancellationToken;

    [Fact]
    public async Task EightConcurrentAcquirersOfOneIdentityShareOneStartAndTheLastReleaseStopsIt()
    {
        var launcher = new FakeLauncher { Gate = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously) };
        var registry = launcher.Registry;

        var acquiring = Enumerable.Range(0, 8).Select(_ => registry.AcquireAsync(FakeLauncher.Identity(), isolated: false, NullLogger.Instance, Token)).ToArray();
        await WaitUntilAsync(() => registry.Entries().SingleOrDefault()?.Holders == 8);
        launcher.Gate.SetResult();
        var leases = await Task.WhenAll(acquiring);

        var started = Assert.Single(launcher.Launches).Process!;
        Assert.All(leases, lease => Assert.Equal(started.ProcessId, lease.ProcessId));
        Assert.Equal(new ServiceRegistry.EntryView(FakeLauncher.Identity(), true, "Ready", 8, started.ProcessId), Assert.Single(registry.Entries()));

        foreach (var lease in leases.Skip(1))
        {
            await lease.ReleaseAsync(NullLogger.Instance);
        }

        Assert.Equal(0, started.Stops);
        Assert.Null(leases[0].EndReason);
        await leases[0].ReleaseAsync(NullLogger.Instance);
        Assert.Equal(1, started.Stops);
        Assert.Empty(registry.Entries());
        Assert.True(leases[0].Ended.IsCancellationRequested);
        Assert.Contains("its last connection closed", leases[0].EndReason, StringComparison.Ordinal);
    }

    [Fact]
    public async Task IdentitiesThatDifferInAnyPartStartServicesOfTheirOwn()
    {
        var launcher = new FakeLauncher();
        var registry = launcher.Registry;
        var basis = FakeLauncher.Identity();
        ServiceIdentity[] identities =
        [
            basis,
            basis with { Path = "/other/sysml-grpc" },
            basis with { Sha256 = "bb" },
            basis with { StartupTimeout = TimeSpan.FromSeconds(1) },
            basis with { ShutdownTimeout = TimeSpan.FromSeconds(1) },
            basis with { Version = "v1.0.0" },
        ];

        var leases = await Task.WhenAll(identities.Select(identity => registry.AcquireAsync(identity, isolated: false, NullLogger.Instance, Token)));
        var again = await registry.AcquireAsync(basis with { }, isolated: false, NullLogger.Instance, Token);

        Assert.Equal(identities.ToHashSet(), launcher.Launches.Select(launch => launch.Identity).ToHashSet());
        Assert.Equal(identities.Length, launcher.Launches.Count);
        Assert.Equal(identities.Length, leases.Select(lease => lease.ProcessId).Distinct().Count());
        Assert.Equal(leases[0].ProcessId, again.ProcessId);
    }

    [Fact]
    public async Task AnIsolatedServiceIsStartedForEachLeaseAndStoppedWithIt()
    {
        var launcher = new FakeLauncher();
        var registry = launcher.Registry;

        var shared = await registry.AcquireAsync(FakeLauncher.Identity(), isolated: false, NullLogger.Instance, Token);
        var first = await registry.AcquireAsync(FakeLauncher.Identity(), isolated: true, NullLogger.Instance, Token);
        var second = await registry.AcquireAsync(FakeLauncher.Identity(), isolated: true, NullLogger.Instance, Token);

        Assert.Equal(3, launcher.Launches.Count);
        Assert.Equal(3, new[] { shared.ProcessId, first.ProcessId, second.ProcessId }.Distinct().Count());
        Assert.False(first.IsShared);
        Assert.True(shared.IsShared);
        Assert.Equal(2, registry.Entries().Count(entry => !entry.IsShared));

        await first.ReleaseAsync(NullLogger.Instance);

        Assert.Equal(1, launcher.Launches[1].Process!.Stops);
        Assert.Equal(0, launcher.Launches[2].Process!.Stops);
        Assert.Equal(0, launcher.Launches[0].Process!.Stops);
    }

    [Fact]
    public async Task AWaiterThatGivesUpReleasesItsOwnReservationAndNoOther()
    {
        var launcher = new FakeLauncher { Gate = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously) };
        var registry = launcher.Registry;
        using var impatient = CancellationTokenSource.CreateLinkedTokenSource(Token);

        var patient = registry.AcquireAsync(FakeLauncher.Identity(), isolated: false, NullLogger.Instance, Token);
        var giving = registry.AcquireAsync(FakeLauncher.Identity(), isolated: false, NullLogger.Instance, impatient.Token);
        await WaitUntilAsync(() => registry.Entries().SingleOrDefault()?.Holders == 2 && launcher.Launches.Count == 1);
        await impatient.CancelAsync();

        var cancelled = await Assert.ThrowsAnyAsync<OperationCanceledException>(() => giving);
        Assert.Equal(impatient.Token, cancelled.CancellationToken);
        Assert.Equal(1, Assert.Single(registry.Entries()).Holders);
        Assert.False(launcher.Launches[0].Token.IsCancellationRequested);

        launcher.Gate.SetResult();
        var lease = await patient;

        Assert.Equal(launcher.Launches[0].Process!.ProcessId, lease.ProcessId);
        Assert.Equal("Ready", Assert.Single(registry.Entries()).State);
    }

    [Fact]
    public async Task WhenEveryWaiterGivesUpTheStartIsCancelledAndTheNextAcquirerStartsAfresh()
    {
        var launcher = new FakeLauncher { Gate = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously) };
        var registry = launcher.Registry;
        using var impatient = CancellationTokenSource.CreateLinkedTokenSource(Token);

        var waiting = new[]
        {
            registry.AcquireAsync(FakeLauncher.Identity(), isolated: false, NullLogger.Instance, impatient.Token),
            registry.AcquireAsync(FakeLauncher.Identity(), isolated: false, NullLogger.Instance, impatient.Token),
        };
        await WaitUntilAsync(() => registry.Entries().SingleOrDefault()?.Holders == 2);
        await impatient.CancelAsync();

        foreach (var waiter in waiting)
        {
            await Assert.ThrowsAnyAsync<OperationCanceledException>(() => waiter);
        }

        Assert.True(launcher.Launches[0].Token.IsCancellationRequested);
        Assert.Empty(registry.Entries());

        launcher.Gate = null;
        var lease = await registry.AcquireAsync(FakeLauncher.Identity(), isolated: false, NullLogger.Instance, Token);
        Assert.Equal(2, launcher.Launches.Count);
        Assert.Equal(launcher.Launches[1].Process!.ProcessId, lease.ProcessId);
    }

    [Fact]
    public async Task AServiceThatFinishesStartingAfterItsLastWaiterLeftIsStopped()
    {
        var launcher = new FakeLauncher { Gate = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously), IgnoreCancellation = true };
        var registry = launcher.Registry;
        using var impatient = CancellationTokenSource.CreateLinkedTokenSource(Token);

        var waiting = registry.AcquireAsync(FakeLauncher.Identity(), isolated: false, NullLogger.Instance, impatient.Token);
        await WaitUntilAsync(() => launcher.Launches.Count == 1);
        await impatient.CancelAsync();
        await WaitUntilAsync(() => registry.Entries().SingleOrDefault()?.State == "Stopping");
        Assert.False(waiting.IsCompleted, "the waiter that gave up waits for the start it cancelled to be undone");
        launcher.Gate.SetResult();

        await Assert.ThrowsAnyAsync<OperationCanceledException>(() => waiting);
        Assert.Equal(1, launcher.Launches[0].Process!.Stops);
        Assert.Empty(registry.Entries());
    }

    [Fact]
    public async Task StoppingAllWaitsForAStopAlreadyUnderWay()
    {
        var stopping = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
        var registry = new ServiceRegistry((_, _, _) => Task.FromResult<IServiceProcess>(new SlowStop(7, stopping.Task)));
        var lease = await registry.AcquireAsync(FakeLauncher.Identity(), isolated: false, NullLogger.Instance, Token);
        var releasing = lease.ReleaseAsync(NullLogger.Instance);
        await WaitUntilAsync(() => registry.Entries().SingleOrDefault()?.State == "Stopping");

        var stoppingAll = registry.StopAllAsync(Token);
        await Task.Delay(50, Token);
        Assert.False(stoppingAll.IsCompleted);
        stopping.SetResult();

        Assert.Equal(0, await stoppingAll);
        await releasing;
        Assert.Empty(registry.Entries());
    }

    [Fact]
    public async Task StoppingAllAggregatesFailuresFromExistingStopsAndThoseItStarts()
    {
        var stopping = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
        var existingFailure = new OpenSysMLException("process 7 failed to stop");
        var newFailure = new OpenSysMLException("process 8 failed to stop");
        var registry = new ServiceRegistry((identity, _, _) => Task.FromResult<IServiceProcess>(
            identity.Path switch
            {
                "/existing" => new SlowStop(7, stopping.Task),
                "/new" => new SlowStop(8, Task.FromException(newFailure)),
                _ => new SlowStop(9, Task.CompletedTask),
            }));
        var existing = await registry.AcquireAsync(FakeLauncher.Identity("/existing"), isolated: false, NullLogger.Instance, Token);
        var newlyStopped = await registry.AcquireAsync(FakeLauncher.Identity("/new"), isolated: false, NullLogger.Instance, Token);
        var clean = await registry.AcquireAsync(FakeLauncher.Identity("/clean"), isolated: false, NullLogger.Instance, Token);
        var releasing = existing.ReleaseAsync(NullLogger.Instance);

        var stoppingAll = registry.StopAllAsync(Token);
        Assert.False(stoppingAll.IsCompleted);
        stopping.SetException(existingFailure);

        var error = await Assert.ThrowsAsync<ServiceStopException>(() => stoppingAll);
        Assert.Equal(2, error.Failures.Count);
        Assert.Contains(existingFailure, error.Failures);
        Assert.Contains(newFailure, error.Failures);
        await releasing;
        await newlyStopped.ReleaseAsync(NullLogger.Instance);
        await clean.ReleaseAsync(NullLogger.Instance);
        Assert.Empty(registry.Entries());
    }

    [Fact]
    public async Task AFailedStartGivesEveryWaiterTheSameFailureAndIsForgotten()
    {
        var failure = new ServiceStartException("sysml-grpc did not start");
        var launcher = new FakeLauncher { Gate = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously), Failure = failure };
        var registry = launcher.Registry;

        var waiting = Enumerable.Range(0, 3).Select(_ => registry.AcquireAsync(FakeLauncher.Identity(), isolated: false, NullLogger.Instance, Token)).ToArray();
        await WaitUntilAsync(() => registry.Entries().SingleOrDefault()?.Holders == 3);
        launcher.Gate.SetResult();

        foreach (var waiter in waiting)
        {
            Assert.Same(failure, await Assert.ThrowsAsync<ServiceStartException>(() => waiter));
        }

        Assert.Empty(registry.Entries());
        launcher.Failure = null;
        launcher.Gate = null;
        await registry.AcquireAsync(FakeLauncher.Identity(), isolated: false, NullLogger.Instance, Token);
        Assert.Equal(2, launcher.Launches.Count);
    }

    [Fact]
    public async Task AServiceThatExitsOnItsOwnEndsItsLeasesAndIsNotRestarted()
    {
        var launcher = new FakeLauncher();
        var registry = launcher.Registry;
        var logger = new ListLogger();
        var lease = await registry.AcquireAsync(FakeLauncher.Identity(), isolated: false, logger, Token);
        var other = await registry.AcquireAsync(FakeLauncher.Identity(), isolated: false, logger, Token);

        launcher.Launches[0].Process!.Die(3);
        await WaitUntilAsync(() => lease.Ended.IsCancellationRequested && registry.Entries().Count == 0);

        Assert.Contains("exited with code 3 on its own; it is not restarted", lease.EndReason, StringComparison.Ordinal);
        Assert.Equal(lease.EndReason, other.EndReason);
        Assert.Contains(logger.Entries, entry => entry.Level == LogLevel.Warning && entry.Message.Contains("exited with code 3", StringComparison.Ordinal)
            && entry.Message.Contains("a fake", StringComparison.Ordinal));
        Assert.Single(launcher.Launches);

        await lease.ReleaseAsync(NullLogger.Instance);
        await other.ReleaseAsync(NullLogger.Instance);
        var next = await registry.AcquireAsync(FakeLauncher.Identity(), isolated: false, NullLogger.Instance, Token);

        Assert.Equal(2, launcher.Launches.Count);
        Assert.NotEqual(lease.ProcessId, next.ProcessId);
        Assert.Null(next.EndReason);
    }

    [Fact]
    public async Task AnAcquirerNeverJoinsAServiceThatHasAlreadyExited()
    {
        var launcher = new FakeLauncher();
        var registry = launcher.Registry;
        var lease = await registry.AcquireAsync(FakeLauncher.Identity(), isolated: false, NullLogger.Instance, Token);

        launcher.Launches[0].Process!.Die(1);
        var next = await registry.AcquireAsync(FakeLauncher.Identity(), isolated: false, NullLogger.Instance, Token);

        Assert.Equal(2, launcher.Launches.Count);
        Assert.NotEqual(lease.ProcessId, next.ProcessId);
        Assert.Null(next.EndReason);
        Assert.Equal(1, Assert.Single(registry.Entries()).Holders);
    }

    [Fact]
    public async Task StoppingAllEndsEverySharedLeaseLeavesIsolatedOnesAndLetsALaterAcquirerStartAfresh()
    {
        var launcher = new FakeLauncher();
        var registry = launcher.Registry;
        var first = await registry.AcquireAsync(FakeLauncher.Identity(), isolated: false, NullLogger.Instance, Token);
        var second = await registry.AcquireAsync(FakeLauncher.Identity("/other"), isolated: false, NullLogger.Instance, Token);
        var isolated = await registry.AcquireAsync(FakeLauncher.Identity(), isolated: true, NullLogger.Instance, Token);

        Assert.Equal(2, await registry.StopAllAsync(Token));

        Assert.Contains("stopped by Connection.StopSharedServicesAsync", first.EndReason, StringComparison.Ordinal);
        Assert.True(second.Ended.IsCancellationRequested);
        Assert.Null(isolated.EndReason);
        Assert.Equal([1, 1, 0], launcher.Launches.Select(launch => launch.Process!.Stops));
        Assert.False(Assert.Single(registry.Entries()).IsShared);

        await first.ReleaseAsync(NullLogger.Instance);
        var reopened = await registry.AcquireAsync(FakeLauncher.Identity(), isolated: false, NullLogger.Instance, Token);
        Assert.Equal(4, launcher.Launches.Count);
        Assert.NotEqual(first.ProcessId, reopened.ProcessId);
        Assert.Equal(1, await registry.StopAllAsync(Token));
    }

    [Fact]
    public async Task StoppingAllCancelsAStartInProgressAndItsWaitersFailAsAStartFailure()
    {
        var launcher = new FakeLauncher { Gate = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously) };
        var registry = launcher.Registry;
        var waiting = registry.AcquireAsync(FakeLauncher.Identity(), isolated: false, NullLogger.Instance, Token);
        await WaitUntilAsync(() => registry.Entries().Count == 1);

        Assert.Equal(1, await registry.StopAllAsync(Token));

        var error = await Assert.ThrowsAsync<ServiceStartException>(() => waiting);
        Assert.Contains("stopped as it started. The private sysml-grpc was stopped by Connection.StopSharedServicesAsync", error.Message, StringComparison.Ordinal);
        Assert.IsAssignableFrom<OperationCanceledException>(error.InnerException);
        Assert.True(launcher.Launches[0].Token.IsCancellationRequested);
        Assert.Empty(registry.Entries());
    }

    [Fact]
    public async Task AnAcquirerArrivingWhileAServiceStopsWaitsForItToBeGoneThenStartsAnother()
    {
        var stopping = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
        var started = 0;
        var registry = new ServiceRegistry((_, _, _) => Task.FromResult<IServiceProcess>(new SlowStop(Interlocked.Increment(ref started), stopping.Task)));
        var lease = await registry.AcquireAsync(FakeLauncher.Identity(), isolated: false, NullLogger.Instance, Token);

        var releasing = lease.ReleaseAsync(NullLogger.Instance);
        await WaitUntilAsync(() => registry.Entries().SingleOrDefault()?.State == "Stopping");
        var arriving = registry.AcquireAsync(FakeLauncher.Identity(), isolated: false, NullLogger.Instance, Token);
        await Task.Delay(50, Token);

        Assert.False(arriving.IsCompleted);
        Assert.Equal("Stopping", Assert.Single(registry.Entries()).State);
        stopping.SetResult();
        await releasing;
        var next = await arriving;

        Assert.NotEqual(lease.ProcessId, next.ProcessId);
        Assert.Equal("Ready", Assert.Single(registry.Entries()).State);
    }

    [Fact]
    public async Task StoppingAllTriesEveryServiceAndReportsEachThatFailedToStop()
    {
        var launcher = new FakeLauncher();
        var registry = launcher.Registry;
        var leases = await Task.WhenAll(((string[])["/a", "/b", "/c"]).Select(path => registry.AcquireAsync(FakeLauncher.Identity(path), isolated: false, NullLogger.Instance, Token)));
        launcher.StopFailure = process => process.ProcessId == leases[1].ProcessId ? null : new OpenSysMLException($"process {process.ProcessId} would not go");

        var error = await Assert.ThrowsAsync<ServiceStopException>(() => registry.StopAllAsync(Token));

        Assert.Equal(2, error.Failures.Count);
        Assert.Contains($"process {leases[0].ProcessId} would not go", error.Message, StringComparison.Ordinal);
        Assert.Contains($"process {leases[2].ProcessId} would not go", error.Message, StringComparison.Ordinal);
        Assert.IsType<AggregateException>(error.InnerException);
        Assert.All(launcher.Launches, launch => Assert.Equal(1, launch.Process!.Stops));
        Assert.Empty(registry.Entries());
    }

    [Fact]
    public async Task ALastReleaseThatFailsToStopLogsItAndNeverThrows()
    {
        var launcher = new FakeLauncher { StopFailure = process => new OpenSysMLException($"process {process.ProcessId} would not go") };
        var registry = launcher.Registry;
        var logger = new ListLogger();
        var lease = await registry.AcquireAsync(FakeLauncher.Identity(), isolated: false, NullLogger.Instance, Token);

        await lease.ReleaseAsync(logger);
        await lease.ReleaseAsync(logger);

        var logged = Assert.Single(logger.Entries);
        Assert.Equal(LogLevel.Warning, logged.Level);
        Assert.Contains($"process {lease.ProcessId}, failed", logged.Message, StringComparison.Ordinal);
        Assert.Contains("would not go", logged.Exception!.Message, StringComparison.Ordinal);
        Assert.Empty(registry.Entries());
    }

    [Fact]
    public void StopFailuresAreCopiedAndCannotBeChangedThroughTheirPublicCollection()
    {
        var failure = new OpenSysMLException("process 7 failed to stop");
        Exception[] source = [failure];
        var error = new ServiceStopException(source);
        source[0] = new OpenSysMLException("another failure");

        Assert.Same(failure, Assert.Single(error.Failures));
        if (error.Failures is IList<Exception> list)
        {
            Assert.True(list.IsReadOnly);
            Assert.Throws<NotSupportedException>(() => list[0] = source[0]);
        }

        Assert.Same(failure, Assert.Single(error.Failures));
        Assert.Same(failure, error.InnerException);
    }

    [Fact]
    public async Task ReleasingALeaseTwiceGivesUpOneHold()
    {
        var launcher = new FakeLauncher();
        var registry = launcher.Registry;
        var first = await registry.AcquireAsync(FakeLauncher.Identity(), isolated: false, NullLogger.Instance, Token);
        var second = await registry.AcquireAsync(FakeLauncher.Identity(), isolated: false, NullLogger.Instance, Token);

        await Task.WhenAll(Enumerable.Range(0, 8).Select(_ => first.ReleaseAsync(NullLogger.Instance)));

        Assert.Equal(1, Assert.Single(registry.Entries()).Holders);
        Assert.Null(second.EndReason);
    }

    [Fact]
    public async Task ClosingEveryInputReachesRunningServicesEndsTheirLeasesAndCancelsStartsInProgress()
    {
        var launcher = new FakeLauncher();
        var registry = launcher.Registry;
        var shared = await registry.AcquireAsync(FakeLauncher.Identity(), isolated: false, NullLogger.Instance, Token);
        var isolated = await registry.AcquireAsync(FakeLauncher.Identity(), isolated: true, NullLogger.Instance, Token);
        launcher.Gate = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
        var starting = registry.AcquireAsync(FakeLauncher.Identity("/starting"), isolated: false, NullLogger.Instance, Token);
        await WaitUntilAsync(() => launcher.Launches.Count == 3);

        registry.CloseAllInputs();

        Assert.True(launcher.Launches[0].Process!.InputClosed);
        Assert.True(launcher.Launches[1].Process!.InputClosed);
        Assert.True(launcher.Launches[2].Token.IsCancellationRequested);
        Assert.Contains("is being unloaded", shared.EndReason, StringComparison.Ordinal);
        Assert.Contains("is being unloaded", isolated.EndReason, StringComparison.Ordinal);
        var stopped = await Assert.ThrowsAsync<ServiceStartException>(() => starting);
        Assert.Contains("is being unloaded", stopped.Message, StringComparison.Ordinal);
    }

    [Theory]
    [InlineData(false)]
    [InlineData(true)]
    public async Task UnloadingStopsAServiceThatFinishesStartingAfterItsInputWasClosed(bool isolated)
    {
        var launcher = new FakeLauncher
        {
            Gate = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously),
            IgnoreCancellation = true,
        };
        var registry = launcher.Registry;
        var starting = registry.AcquireAsync(FakeLauncher.Identity(), isolated, NullLogger.Instance, Token);
        await WaitUntilAsync(() => launcher.Launches.Count == 1);

        registry.CloseAllInputs();
        launcher.Gate.SetResult();

        var error = await Assert.ThrowsAsync<ServiceStartException>(() => starting);
        Assert.Contains("is being unloaded", error.Message, StringComparison.Ordinal);
        Assert.Equal(1, launcher.Launches[0].Process!.Stops);
        Assert.Empty(registry.Entries());
    }

    [Fact]
    public async Task ACancelledAcquirerSendsNothingAndTakesNothing()
    {
        var launcher = new FakeLauncher();
        var registry = launcher.Registry;

        await Assert.ThrowsAnyAsync<OperationCanceledException>(() => registry.AcquireAsync(FakeLauncher.Identity(), isolated: false, NullLogger.Instance, new CancellationToken(canceled: true)));

        Assert.Empty(launcher.Launches);
        Assert.Empty(registry.Entries());
    }

    [Fact]
    public void TheSharedRegistryIsOneInstance()
    {
        Assert.Same(ServiceRegistry.Shared, ServiceRegistry.Shared);
    }

    /// <summary>Waits for a condition the registry reaches on another thread.</summary>
    internal static async Task WaitUntilAsync(Func<bool> condition)
    {
        var deadline = DateTime.UtcNow + TimeSpan.FromSeconds(10);
        while (!condition())
        {
            Assert.True(DateTime.UtcNow < deadline, "the condition did not hold within 10 seconds");
            await Task.Delay(5, CancellationToken.None);
        }
    }

    // A service whose stop completes only when the test lets it.
    private sealed class SlowStop(int processId, Task stopped) : IServiceProcess
    {
        private readonly TaskCompletionSource<string> exited = new(TaskCreationOptions.RunContinuationsAsynchronously);

        public Uri Address { get; } = new("http://127.0.0.1:1/");

        public int ProcessId { get; } = processId;

        public Task<string> Exited => exited.Task;

        public async Task StopAsync()
        {
            try
            {
                await stopped;
            }
            finally
            {
                exited.TrySetResult("exited with code 0");
            }
        }

        public void CloseInput()
        {
        }

        public string DescribeTail() => string.Empty;
    }
}
