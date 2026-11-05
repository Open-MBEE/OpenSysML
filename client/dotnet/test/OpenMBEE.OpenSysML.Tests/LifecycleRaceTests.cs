using System;
using System.Collections.Generic;
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
/// Races repeated rather than hoped for: opens, closes, a stop of every service, a cancelled wait and a
/// service's death released from one barrier at the same moment, round after round, first against the
/// registry's state machine alone, then against real children. Each round must end with nothing
/// registered, every service stopped exactly once and in order, and no operation failing but in the
/// one way its race allows.
/// </summary>
public sealed class LifecycleRaceTests
{
    private const int StateMachineRounds = 200;
    private const int ProcessRounds = 3;
    private const int Connections = 8;
    private const string Notation = "package Demo { part def Vehicle; }";

    private static CancellationToken Token => TestContext.Current.CancellationToken;

    [Fact]
    [Trait(Category.Name, Category.Unit)]
    public async Task RacingOpensClosesStopsCancellationsAndDeathsStopEveryServiceOnceAndLeaveNothing()
    {
        for (var round = 0; round < StateMachineRounds; round++)
        {
            var launcher = new FakeLauncher();
            var registry = launcher.Registry;
            var identity = FakeLauncher.Identity();
            var parties = new List<Func<CancellationToken, Task>>();
            parties.AddRange(Enumerable.Range(0, 4).Select(_ => (Func<CancellationToken, Task>)(token => HoldAsync(registry, identity, isolated: false, token))));
            parties.Add(token => HoldAsync(registry, identity, isolated: true, token));
            parties.Add(token => GiveUpAsync(registry, identity, token));
            parties.Add(token => registry.StopAllAsync(token));
            parties.Add(token => registry.StopAllAsync(token));
            parties.Add(_ =>
            {
                if (launcher.Launches is [var launch, ..])
                {
                    launch.Process?.Die(1);
                }

                return Task.CompletedTask;
            });

            var failures = await Race.RunAsync(parties, Token);

            // The one failure a race allows: a start that a stop of every service ended as it started.
            Assert.All(failures, failure => Assert.Contains("was stopped as it started", Assert.IsType<ServiceStartException>(failure).Message, StringComparison.Ordinal));
            await AwaitCleanupAsync(registry);
            Assert.Empty(registry.Entries());
            Assert.All(launcher.Launches, launch => Assert.Equal(1, launch.Process!.Stops));
        }
    }

    [Fact]
    [Trait(Category.Name, Category.Unit)]
    public async Task BackgroundDeathCleanupIsAwaitedBeforeTheRegistryIsAssertedEmpty()
    {
        using var finish = new ManualResetEventSlim();
        var stopping = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
        var launcher = new FakeLauncher
        {
            StopFailure = _ =>
            {
                stopping.TrySetResult();
                Assert.True(finish.Wait(TimeSpan.FromSeconds(10), Token), "the test did not release the background stop");
                return null;
            },
        };
        var registry = launcher.Registry;
        var lease = await registry.AcquireAsync(FakeLauncher.Identity(), isolated: false, NullLogger.Instance, Token);
        var service = Assert.Single(launcher.Launches).Process!;
        Task? cleanup = null;
        try
        {
            service.Die(1);
            await stopping.Task.WaitAsync(TimeSpan.FromSeconds(10), Token);
            cleanup = AwaitCleanupAsync(registry);
            Assert.False(cleanup.IsCompleted);
            Assert.Equal("Failed", Assert.Single(registry.Entries()).State);
        }
        finally
        {
            finish.Set();
            await (cleanup ?? AwaitCleanupAsync(registry));
            await lease.ReleaseAsync(NullLogger.Instance);
        }

        Assert.Empty(registry.Entries());
        Assert.Equal(1, service.Stops);
    }

    [Theory]
    [Trait(Category.Name, Category.Unit)]
    [MemberData(nameof(ClientWires.All), MemberType = typeof(ClientWires))]
    public async Task ClosesRacingAStopOfEveryServiceEndTheChildInOrder(ClientWire wire)
    {
        for (var round = 0; round < ProcessRounds; round++)
        {
            using var child = FakeChild.Serving();
            using var handler = new FakeServiceHandler(wire);
            var logger = new ListLogger();

            await RaceClosesAgainstAStopAsync(new ConnectionOptions
            {
                BinaryPath = child.Path,
                HttpMessageHandler = handler,
                Protocol = wire.Protocol(),
                Encoding = wire.Encoding(),
                Logger = logger,
            });

            Assert.True(File.Exists(child.ExitMarker), "the child was killed rather than stopped by closing its input");
            Assert.DoesNotContain(logger.Entries, entry => entry.Level >= LogLevel.Warning);
        }
    }

    [Theory]
    [Trait(Category.Name, Category.Integration)]
    [MemberData(nameof(ClientWires.All), MemberType = typeof(ClientWires))]
    public async Task ClosesRacingAStopOfEveryServiceEndTheRealServiceInOrder(ClientWire wire)
    {
        for (var round = 0; round < ProcessRounds; round++)
        {
            var logger = new ListLogger();

            await RaceClosesAgainstAStopAsync(new ConnectionOptions { BinaryPath = Binary, Protocol = wire.Protocol(), Encoding = wire.Encoding(), Logger = logger });

            Assert.DoesNotContain(logger.Entries, entry => entry.Level >= LogLevel.Warning);
        }
    }

    [Theory]
    [Trait(Category.Name, Category.Integration)]
    [MemberData(nameof(ClientWires.All), MemberType = typeof(ClientWires))]
    public async Task OpensRacingTheLastCloseEachGetAServiceThatAnswers(ClientWire wire)
    {
        for (var round = 0; round < ProcessRounds; round++)
        {
            await using var run = new LifecycleRun();
            var registry = run.Registry;
            var options = new ConnectionOptions { BinaryPath = Binary, Protocol = wire.Protocol(), Encoding = wire.Encoding() };
            var first = await run.OpenAsync(options, Token);
            var opened = new Connection?[Connections / 2];
            var parties = new List<Func<CancellationToken, Task>> { _ => first.DisposeAsync().AsTask() };
            parties.AddRange(Enumerable.Range(0, opened.Length).Select(i => (Func<CancellationToken, Task>)(async token =>
            {
                opened[i] = await run.OpenAsync(options, token);
                await opened[i]!.LoadContentAsync(Notation, cancellationToken: token);
            })));

            Assert.Empty(await Race.RunAsync(parties, Token));
            await Task.WhenAll(opened.OfType<Connection>().Select(connection => connection.DisposeAsync().AsTask()));

            Assert.Empty(registry.Entries());
            foreach (var service in run.ProcessIds)
            {
                await FakeChild.AssertGoneAsync(service);
            }
        }
    }

    private static string Binary => ServiceGate.FromProcess.RequireBinary();

    // Opens connections sharing one service, then closes every one, one twice, while every service is stopped.
    private static async Task RaceClosesAgainstAStopAsync(ConnectionOptions options)
    {
        await using var run = new LifecycleRun();
        var registry = run.Registry;
        var connections = await Task.WhenAll(Enumerable.Range(0, Connections).Select(_ => run.OpenAsync(options, Token)));
        var service = Assert.Single(registry.Entries()).ProcessId!.Value;
        var stopped = -1;
        var parties = connections.Select(connection => (Func<CancellationToken, Task>)(_ => connection.DisposeAsync().AsTask())).ToList();
        parties.Add(_ => connections[0].DisposeAsync().AsTask());
        parties.Add(async token => stopped = await registry.StopAllAsync(token));

        Assert.Empty(await Race.RunAsync(parties, Token));

        Assert.InRange(stopped, 0, 1);
        Assert.Empty(registry.Entries());
        await FakeChild.AssertGoneAsync(service);
    }

    private static Task AwaitCleanupAsync(ServiceRegistry registry) =>
        ServiceRegistryTests.WaitUntilAsync(() => registry.Entries().Count == 0);

    private static async Task HoldAsync(ServiceRegistry registry, ServiceIdentity identity, bool isolated, CancellationToken cancellationToken)
    {
        var lease = await registry.AcquireAsync(identity, isolated, NullLogger.Instance, cancellationToken);
        await Task.Yield();
        await lease.ReleaseAsync(NullLogger.Instance);
    }

    // Waits, gives up at once, and releases what it got if the wait won.
    private static async Task GiveUpAsync(ServiceRegistry registry, ServiceIdentity identity, CancellationToken cancellationToken)
    {
        using var giveUp = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken);
        var acquiring = registry.AcquireAsync(identity, isolated: false, NullLogger.Instance, giveUp.Token);
        await giveUp.CancelAsync();
        try
        {
            await (await acquiring).ReleaseAsync(NullLogger.Instance);
        }
        catch (OperationCanceledException) when (giveUp.IsCancellationRequested)
        {
        }
    }
}
