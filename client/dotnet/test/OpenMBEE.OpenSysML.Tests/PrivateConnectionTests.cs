using System;
using System.IO;
using System.Linq;
using System.Security.Cryptography;
using System.Threading;
using System.Threading.Tasks;
using Microsoft.Extensions.Logging;
using OpenMBEE.OpenSysML.Internal;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// Connections to a private service: real child processes of the FakeService stand-in, started by a
/// registry of the test's own, with the fake handler answering what the connection sends, so every
/// lifecycle path is driven without the real service.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed class PrivateConnectionTests
{
    private static CancellationToken Token => TestContext.Current.CancellationToken;

    [Theory]
    [MemberData(nameof(ClientWires.All), MemberType = typeof(ClientWires))]
    public async Task OpeningWithNoServiceNamedStartsAPrivateOneAndClosingStopsIt(ClientWire wire)
    {
        using var child = FakeChild.Serving();
        using var handler = new FakeServiceHandler(wire);
        var registry = Registry();

        var connection = await OpenAsync(Options(child, handler), registry);
        var entry = Assert.Single(registry.Entries());

        Assert.Equal(new Uri("http://127.0.0.1:50051/"), connection.Address);
        Assert.Equal("v9.9.9", connection.ServerInfo.Version);
        Assert.Equal("GetServerInfo", Assert.Single(handler.Requests).Method);
        Assert.Equal(("Ready", 1, true), (entry.State, entry.Holders, entry.IsShared));
        Assert.Equal(BinaryResolver.Canonical(child.Path), entry.Identity.Path);
        Assert.Equal(Sha256(child.Path), entry.Identity.Sha256);
        Assert.True(FakeChild.IsRunning(entry.ProcessId!.Value));

        await connection.DisposeAsync();

        Assert.Empty(registry.Entries());
        Assert.True(File.Exists(child.ExitMarker), "the last connection to close stops the service in order");
        await FakeChild.AssertGoneAsync(entry.ProcessId.Value);
    }

    [Fact]
    public async Task ConnectionsThatAgreeShareOneServiceUntilTheLastCloses()
    {
        using var child = FakeChild.Serving();
        using var handler = new FakeServiceHandler();
        using var connectHandler = new FakeServiceHandler(ClientWire.ConnectJson);
        var registry = Registry();

        var first = await OpenAsync(Options(child, handler), registry);
        var second = await OpenAsync(Options(child, connectHandler, requestTimeout: TimeSpan.FromSeconds(5)), registry);
        var entry = Assert.Single(registry.Entries());

        Assert.Equal(2, entry.Holders);
        Assert.Equal(first.Address, second.Address);
        await first.DisposeAsync();
        Assert.Equal(1, Assert.Single(registry.Entries()).Holders);
        handler.Respond = (_, _) => Task.FromResult(handler.Answer(new Wire.SymbolResponse()));
        connectHandler.Respond = (_, _) => Task.FromResult(connectHandler.Answer(new Wire.SymbolResponse()));
        await second.GetSymbolAsync();
        Assert.True(FakeChild.IsRunning(entry.ProcessId!.Value));

        await second.DisposeAsync();
        await FakeChild.AssertGoneAsync(entry.ProcessId.Value);
        Assert.Empty(registry.Entries());
    }

    [Fact]
    public async Task OptionsThatChangeTheServiceStartOneOfTheirOwn()
    {
        using var child = FakeChild.Serving();
        using var other = FakeChild.Serving();
        using var handler = new FakeServiceHandler();
        var registry = Registry();

        await using var basis = await OpenAsync(Options(child, handler), registry);
        await using var otherBinary = await OpenAsync(Options(other, handler), registry);
        await using var otherStartup = await OpenAsync(Options(child, handler, startupTimeout: TimeSpan.FromSeconds(29)), registry);
        await using var otherVersion = await OpenAsync(Options(child, handler, version: "v9.9.9"), registry);
        await using var isolated = await OpenAsync(Options(child, handler, isolated: true), registry);
        await using var same = await OpenAsync(Options(child, handler, headers: new() { ["x-tenant"] = "a" }, requestTimeout: TimeSpan.FromSeconds(1)), registry);

        var entries = registry.Entries();
        Assert.Equal(5, entries.Count);
        Assert.Equal(5, entries.Select(entry => entry.ProcessId).Distinct().Count());
        var basisIdentity = ServiceIdentity.Of(new ResolvedBinary(BinaryResolver.Canonical(child.Path), Sha256(child.Path)), Options(child, handler));
        Assert.Equal(2, entries.Single(entry => entry.IsShared && entry.Identity == basisIdentity).Holders);
        Assert.Equal((basisIdentity, 1), entries.Where(entry => !entry.IsShared).Select(entry => (entry.Identity, entry.Holders)).Single());
    }

    [Fact]
    public async Task AFailedHandshakeGivesUpTheHoldAndTheServiceStops()
    {
        using var child = FakeChild.Serving();
        using var handler = new FakeServiceHandler();
        var registry = Registry();

        var error = await Assert.ThrowsAsync<StaleServiceException>(() => OpenAsync(Options(child, handler, version: "v1.0.0"), registry));

        Assert.Equal("v9.9.9", error.ServiceVersion);
        Assert.Contains("start a sysml-grpc of version v1.0.0, named by ConnectionOptions.BinaryPath or $OPENSYSML_GRPC_BINARY", error.Remedy, StringComparison.Ordinal);
        Assert.Empty(registry.Entries());
        Assert.True(File.Exists(child.ExitMarker));
    }

    [Fact]
    public async Task AMissingCapabilityGivesUpTheHoldAndTheServiceStops()
    {
        using var child = FakeChild.Serving();
        using var handler = new FakeServiceHandler();
        var registry = Registry();

        await Assert.ThrowsAsync<CapabilityException>(() => OpenAsync(Options(child, handler, requireCapabilities: [Capabilities.Migrate]), registry));

        Assert.Empty(registry.Entries());
        Assert.True(File.Exists(child.ExitMarker));
    }

    [Fact]
    public async Task AMissingExplicitBinaryFailsBeforeAnythingStarts()
    {
        using var handler = new FakeServiceHandler();
        var registry = Registry();
        var missing = Path.Combine(Path.GetTempPath(), "opensysml-missing-" + Guid.NewGuid().ToString("N"), "sysml-grpc");

        var error = await Assert.ThrowsAsync<BinaryNotFoundException>(() => Connection.OpenAsync(
            new ConnectionOptions { HttpMessageHandler = handler },
            name => name == ConnectionOptions.BinaryVariable ? missing : null,
            registry,
            Token));

        Assert.Contains($"$OPENSYSML_GRPC_BINARY names {missing}, which is absent", error.Message, StringComparison.Ordinal);
        Assert.Empty(registry.Entries());
        Assert.Empty(handler.Requests);
    }

    [Fact]
    public async Task ABinaryWithoutTheDigestRequiredIsNeverExecuted()
    {
        using var child = FakeChild.Serving();
        using var handler = new FakeServiceHandler();
        var registry = Registry();

        var error = await Assert.ThrowsAsync<ChecksumMismatchException>(() => OpenAsync(Options(child, handler, expectedSha256: new string('0', 64)), registry));

        Assert.Equal(Sha256(child.Path), error.ActualSha256);
        Assert.False(File.Exists(child.ArgumentsFile), "the binary was not run");
        Assert.Empty(registry.Entries());
        Assert.Empty(handler.Requests);

        await using var connection = await OpenAsync(Options(child, handler, expectedSha256: Sha256(child.Path).ToUpperInvariant()), registry);
        Assert.True(File.Exists(child.ArgumentsFile));
    }

    [Fact]
    public async Task KillingTheChildAsItStartsFailsTheOpenAndLeavesNoHold()
    {
        using var child = FakeChild.Create(fake => new FakeScript { ProcessIdFile = fake.ProcessIdFile });
        using var handler = new FakeServiceHandler();
        var registry = Registry();

        var opening = OpenAsync(Options(child, handler), registry);
        var processId = await child.ProcessIdAsync();
        Assert.Equal("Starting", Assert.Single(registry.Entries()).State);
        FakeChild.Kill(processId);

        var error = await Assert.ThrowsAsync<ServiceStartException>(() => opening);
        Assert.Contains("ended its output without reporting the address it listens on", error.Message, StringComparison.Ordinal);
        Assert.Empty(registry.Entries());
        Assert.Empty(handler.Requests);
        await FakeChild.AssertGoneAsync(processId);
    }

    [Fact]
    public async Task ACancelledOpenStopsAServiceNoOtherOpenWaitsFor()
    {
        using var child = FakeChild.Create(fake => new FakeScript { ArgumentsFile = fake.ArgumentsFile, ExitMarker = fake.ExitMarker });
        using var handler = new FakeServiceHandler();
        using var cancel = CancellationTokenSource.CreateLinkedTokenSource(Token);
        var registry = Registry();

        var opening = OpenAsync(Options(child, handler), registry, cancel.Token);
        await ServiceRegistryTests.WaitUntilAsync(() => File.Exists(child.ArgumentsFile));
        await cancel.CancelAsync();

        var error = await Assert.ThrowsAnyAsync<OperationCanceledException>(() => opening);
        Assert.Equal(cancel.Token, error.CancellationToken);
        Assert.Empty(registry.Entries());
        Assert.True(File.Exists(child.ExitMarker), "the open returns once the child is stopped");
    }

    [Theory]
    [MemberData(nameof(ClientWires.All), MemberType = typeof(ClientWires))]
    public async Task ACallInFlightWhenItsConnectionClosesFailsAsDisposedAndTheServiceThenStops(ClientWire wire)
    {
        using var child = FakeChild.Serving();
        var started = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
        using var handler = new FakeServiceHandler(wire)
        {
            Respond = async (_, token) =>
            {
                started.TrySetResult();
                await Task.Delay(Timeout.Infinite, token);
                throw new InvalidOperationException("unreachable");
            },
        };
        var registry = Registry();
        var connection = await OpenAsync(Options(child, handler), registry);
        var processId = Assert.Single(registry.Entries()).ProcessId!.Value;

        var call = connection.GetSymbolAsync();
        await started.Task.WaitAsync(Token);
        await connection.DisposeAsync();

        await Assert.ThrowsAsync<ObjectDisposedException>(() => call);
        Assert.Empty(registry.Entries());
        await FakeChild.AssertGoneAsync(processId);
    }

    [Theory]
    [MemberData(nameof(ClientWires.All), MemberType = typeof(ClientWires))]
    public async Task AServiceThatDiesFailsItsCallsClearlyAndANewOpenStartsAnother(ClientWire wire)
    {
        using var child = FakeChild.Serving();
        var started = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
        using var handler = new FakeServiceHandler(wire)
        {
            Respond = async (_, token) =>
            {
                started.TrySetResult();
                await Task.Delay(Timeout.Infinite, token);
                throw new InvalidOperationException("unreachable");
            },
        };
        var logger = new ListLogger();
        var registry = Registry();
        await using var connection = await OpenAsync(Options(child, handler, logger: logger), registry);
        var processId = Assert.Single(registry.Entries()).ProcessId!.Value;

        var inFlight = connection.GetSymbolAsync();
        await started.Task.WaitAsync(Token);
        FakeChild.Kill(processId);

        // Promptly, not when the call's deadline passes: the service's end cancels the call.
        var failed = await Assert.ThrowsAsync<TransportException>(() => inFlight.WaitAsync(TimeSpan.FromSeconds(10), Token));
        Assert.Equal(StatusCode.Unavailable, failed.StatusCode);
        Assert.Equal("sysml.SysMLService/GetSymbol", failed.Method);
        Assert.Contains($"(process {processId}) exited with code", failed.Message, StringComparison.Ordinal);
        Assert.Contains("on its own; it is not restarted", failed.Message, StringComparison.Ordinal);
        var requests = handler.Requests.Count;
        var later = await Assert.ThrowsAsync<TransportException>(() => connection.GetSymbolAsync());
        Assert.Equal(failed.Message, later.Message);
        Assert.Equal(requests, handler.Requests.Count);
        Assert.Contains(logger.Entries, entry => entry.Level == LogLevel.Warning && entry.Message.Contains($"process {processId}", StringComparison.Ordinal));
        await ServiceRegistryTests.WaitUntilAsync(() => registry.Entries().Count == 0);

        handler.Respond = (_, _) => Task.FromResult(handler.Answer(new Wire.SymbolResponse()));
        await using var reopened = await OpenAsync(Options(child, handler), registry);
        Assert.NotEqual(processId, Assert.Single(registry.Entries()).ProcessId);
        await reopened.GetSymbolAsync();
    }

    [Fact]
    public async Task ATransportFailureAfterTheServiceWentReportsThatItWent()
    {
        var launcher = new FakeLauncher();
        var registry = launcher.Registry;
        var lease = await registry.AcquireAsync(FakeLauncher.Identity(), isolated: false, Microsoft.Extensions.Logging.Abstractions.NullLogger.Instance, Token);
        var reset = new TransportException(StatusCode.Unavailable, "connection reset by peer", "sysml.SysMLService/GetSymbol");
        var transport = new FakeTransport(async (method, _, _) =>
        {
            if (method == "GetServerInfo")
            {
                return new Wire.ServerInfoResponse { Version = "v9.9.9" };
            }

            launcher.Launches[0].Process!.Die(9);
            await ServiceRegistryTests.WaitUntilAsync(() => lease.EndReason is not null);
            throw reset;
        });
        await using var connection = await Connection.OpenAsync(transport, lease.Address, new ConnectionOptions(), lease, Token);

        var error = await Assert.ThrowsAsync<TransportException>(() => connection.GetSymbolAsync());

        Assert.Equal(StatusCode.Unavailable, error.StatusCode);
        Assert.Contains("exited with code 9 on its own", error.Message, StringComparison.Ordinal);
        Assert.Same(reset, error.InnerException);
    }

    [Fact]
    public async Task StoppingSharedServicesFailsTheirConnectionsAndNeverMovesThemToAnother()
    {
        using var child = FakeChild.Serving();
        using var handler = new FakeServiceHandler { Respond = (_, _) => Task.FromResult(FakeServiceHandler.AnswerOn(ClientWire.Grpc, new Wire.SymbolResponse())) };
        var registry = Registry();
        var connection = await OpenAsync(Options(child, handler), registry);
        var isolated = await OpenAsync(Options(child, handler, isolated: true), registry);

        Assert.Equal(1, await registry.StopAllAsync(Token));

        var error = await Assert.ThrowsAsync<TransportException>(() => connection.GetSymbolAsync());
        Assert.Equal(StatusCode.Unavailable, error.StatusCode);
        Assert.Contains("stopped by Connection.StopSharedServicesAsync", error.Message, StringComparison.Ordinal);
        await isolated.GetSymbolAsync();

        await using var reopened = await OpenAsync(Options(child, handler), registry);
        await reopened.GetSymbolAsync();
        await Assert.ThrowsAsync<TransportException>(() => connection.GetSymbolAsync());
        Assert.Equal(2, registry.Entries().Count);

        await connection.DisposeAsync();
        await isolated.DisposeAsync();
        Assert.Single(registry.Entries());
    }

    [Fact]
    public async Task AServiceNamedByTheEnvironmentLeavesThePrivateOptionsUnusedAndSaysSo()
    {
        using var handler = new FakeServiceHandler();
        var logger = new ListLogger();
        var registry = Registry();

        await using var connection = await Connection.OpenAsync(
            new ConnectionOptions { BinaryPath = "/nowhere/sysml-grpc", IsolatedService = true, HttpMessageHandler = handler, Logger = logger },
            name => name == ConnectionOptions.ServiceVariable ? "fake.invalid:50051" : null,
            registry,
            Token);

        Assert.Equal(new Uri("http://fake.invalid:50051/"), connection.Address);
        Assert.Empty(registry.Entries());
        Assert.Contains(logger.Entries, entry => entry.Level == LogLevel.Warning
            && entry.Message == "Connecting to http://fake.invalid:50051/, which $OPENSYSML_SERVICE names: the options describing a private service are not used");
    }

    [Theory]
    [InlineData("latest", null)]
    [InlineData(null, "someone/fork")]
    public async Task AServiceNamedByTheEnvironmentDownloadsNothingAndSaysTheReleaseOptionsAreUnused(string? version, string? repo)
    {
        using var handler = new FakeServiceHandler();
        await using var releases = new ReleaseServer();
        releases.Latest("v9.9.9");
        var logger = new ListLogger();
        var registry = Registry();

        await using var connection = await Connection.OpenAsync(
            new ConnectionOptions { DownloadVersion = version, GithubRepo = repo, HttpMessageHandler = handler, Logger = logger },
            name => name == ConnectionOptions.ServiceVariable ? "fake.invalid:50051" : null,
            registry,
            releases.Source(),
            Token);

        Assert.Empty(registry.Entries());
        Assert.Empty(releases.Requested);
        Assert.Contains(logger.Entries, entry => entry.Level == LogLevel.Warning
            && entry.Message == "Connecting to http://fake.invalid:50051/, which $OPENSYSML_SERVICE names: the options describing a private service are not used");
    }

    [Fact]
    public async Task NoServiceNamedAndNoAutoStartIsAStartFailureThatStartsNothing()
    {
        var registry = Registry();

        var error = await Assert.ThrowsAsync<ServiceStartException>(() => Connection.OpenAsync(new ConnectionOptions { AutoStart = false }, _ => null, registry, Token));

        Assert.Contains("ConnectionOptions.AutoStart is false", error.Message, StringComparison.Ordinal);
        Assert.Empty(registry.Entries());
    }

    [Fact]
    public async Task EightConcurrentOpensShareOneChild()
    {
        using var child = FakeChild.Create(fake => new FakeScript { Report = "127.0.0.1:50051", DelayMilliseconds = 300, ExitMarker = fake.ExitMarker });
        using var handler = new FakeServiceHandler();
        var registry = Registry();

        var connections = await Task.WhenAll(Enumerable.Range(0, 8).Select(_ => Task.Run(() => OpenAsync(Options(child, handler), registry), Token)));

        var entry = Assert.Single(registry.Entries());
        Assert.Equal(8, entry.Holders);
        foreach (var connection in connections)
        {
            await connection.DisposeAsync();
        }

        Assert.Empty(registry.Entries());
        await FakeChild.AssertGoneAsync(entry.ProcessId!.Value);
    }

    private static ServiceRegistry Registry() => new(PrivateService.StartAsync);

    private static ConnectionOptions Options(
        FakeChild child,
        FakeServiceHandler handler,
        string? version = null,
        TimeSpan? startupTimeout = null,
        bool isolated = false,
        string? expectedSha256 = null,
        string[]? requireCapabilities = null,
        ILogger? logger = null,
        TimeSpan? requestTimeout = null,
        System.Collections.Generic.Dictionary<string, string>? headers = null) => new()
        {
            BinaryPath = child.Path,
            HttpMessageHandler = handler,
            Protocol = handler.Wire.Protocol(),
            Encoding = handler.Wire.Encoding(),
            ShutdownTimeout = TimeSpan.FromSeconds(10),
            Version = version,
            StartupTimeout = startupTimeout ?? ConnectionOptions.DefaultStartupTimeout,
            IsolatedService = isolated,
            ExpectedBinarySha256 = expectedSha256,
            RequireCapabilities = requireCapabilities ?? [],
            Logger = logger ?? Microsoft.Extensions.Logging.Abstractions.NullLogger.Instance,
            RequestTimeout = requestTimeout ?? ConnectionOptions.DefaultRequestTimeout,
            Headers = headers ?? [],
        };

    private static Task<Connection> OpenAsync(ConnectionOptions options, ServiceRegistry registry, CancellationToken? cancellationToken = null) =>
        Connection.OpenAsync(options, _ => null, registry, cancellationToken ?? Token);

    private static string Sha256(string path) => Convert.ToHexStringLower(SHA256.HashData(File.ReadAllBytes(path)));
}
