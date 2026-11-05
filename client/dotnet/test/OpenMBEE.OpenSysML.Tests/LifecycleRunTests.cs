using System;
using System.IO;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>Race cleanup owns successful opens and resolved children even when acquisition, assertions or disposal fail.</summary>
[Trait(Category.Name, Category.Unit)]
public sealed class LifecycleRunTests
{
    private static CancellationToken Token => TestContext.Current.CancellationToken;

    [Theory]
    [MemberData(nameof(ClientWires.All), MemberType = typeof(ClientWires))]
    public async Task AFailedConcurrentOpenStillDisposesTheSuccessfulConnections(ClientWire wire)
    {
        using var child = FakeChild.Serving();
        using var success = new FakeServiceHandler(wire);
        using var failure = new FakeServiceHandler(wire)
        {
            ServerInfo = (_, _) => Task.FromResult(FakeServiceHandler.RefuseOn(wire, StatusCode.Unavailable, "handshake failed")),
        };
        var run = new LifecycleRun();
        Task<Connection>[] openings = [];
        await using (run)
        {
            openings =
            [
                run.OpenAsync(Options(child, success, wire), Token),
                run.OpenAsync(Options(child, failure, wire), Token),
                run.OpenAsync(Options(child, success, wire), Token),
            ];
            var refused = await Assert.ThrowsAsync<ServiceException>(() => Task.WhenAll(openings));
            Assert.Equal(StatusCode.Unavailable, refused.StatusCode);
            Assert.Contains("handshake failed", refused.Message, StringComparison.Ordinal);
            Assert.Equal(2, openings.Count(opening => opening.IsCompletedSuccessfully));
            Assert.True(FakeChild.IsRunning(Assert.Single(run.ProcessIds)));
        }

        foreach (var opening in openings.Where(opening => opening.IsCompletedSuccessfully))
        {
            var connection = await opening;
            await Assert.ThrowsAsync<ObjectDisposedException>(() => connection.LoadContentAsync("package Demo;", cancellationToken: Token));
        }

        await AssertGoneAsync(run);
        Assert.True(File.Exists(child.ExitMarker), "partial acquisition cleanup killed the child instead of releasing its connections");
        Assert.Empty(run.Registry.Entries());
    }

    [Theory]
    [MemberData(nameof(ClientWires.All), MemberType = typeof(ClientWires))]
    public async Task AFailureAfterOpeningStillStopsEveryOwnedChild(ClientWire wire)
    {
        using var child = FakeChild.Serving();
        using var handler = new FakeServiceHandler(wire);
        var run = new LifecycleRun();
        var failure = new InvalidOperationException("an assertion failed after opening");

        var thrown = await Assert.ThrowsAsync<InvalidOperationException>(async () =>
        {
            await using (run)
            {
                await run.OpenAsync(Options(child, handler, wire), Token);
                throw failure;
            }
        });

        Assert.Same(failure, thrown);
        await AssertGoneAsync(run);
        Assert.True(File.Exists(child.ExitMarker), "assertion failure cleanup killed the child instead of releasing its connections");
        Assert.Empty(run.Registry.Entries());
        await Assert.ThrowsAsync<ObjectDisposedException>(() => run.OpenAsync(Options(child, handler, wire), Token));
    }

    [Theory]
    [MemberData(nameof(ClientWires.All), MemberType = typeof(ClientWires))]
    public async Task AnOrderlyCleanupTimeoutIsReportedAfterTheOwnedChildIsKilled(ClientWire wire)
    {
        using var child = FakeChild.Create(_ => new FakeScript { Report = "127.0.0.1:50051", IgnoreStdinEnd = true });
        using var handler = new FakeServiceHandler(wire);
        var run = new LifecycleRun(TimeSpan.FromMilliseconds(100));

        var failure = await Assert.ThrowsAsync<TimeoutException>(async () =>
        {
            await using (run)
            {
                await run.OpenAsync(Options(child, handler, wire, TimeSpan.FromMinutes(1)), Token);
            }
        });

        Assert.Contains("Orderly lifecycle cleanup", failure.Message, StringComparison.Ordinal);
        await AssertGoneAsync(run);
        await ServiceRegistryTests.WaitUntilAsync(() => run.Registry.Entries().Count == 0);
    }

    private static ConnectionOptions Options(FakeChild child, FakeServiceHandler handler, ClientWire wire, TimeSpan? shutdownTimeout = null) =>
        new()
        {
            BinaryPath = child.Path,
            HttpMessageHandler = handler,
            Protocol = wire.Protocol(),
            Encoding = wire.Encoding(),
            ShutdownTimeout = shutdownTimeout ?? ConnectionOptions.DefaultShutdownTimeout,
        };

    private static async Task AssertGoneAsync(LifecycleRun run)
    {
        Assert.Single(run.ProcessIds);
        foreach (var processId in run.ProcessIds)
        {
            await FakeChild.AssertGoneAsync(processId);
        }
    }
}
