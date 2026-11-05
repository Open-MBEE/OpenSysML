using System;
using System.IO;
using System.Linq;
using System.Security.Cryptography;
using System.Threading;
using System.Threading.Tasks;
using OpenMBEE.OpenSysML.Internal;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// Private services of the real sysml-grpc, each test with a registry of its own so that counts are
/// its own: started with no service named, shared, isolated, killed, stopped and started again, over
/// every client wire.
/// </summary>
[Trait(Category.Name, Category.Integration)]
public sealed class ServiceLifecycleTests(ServiceFixture services) : IClassFixture<ServiceFixture>
{
    private const string Notation = "package Demo { part def Vehicle { attribute mass default = 1500.0; } }";

    private static CancellationToken Token => TestContext.Current.CancellationToken;

    [Theory]
    [MemberData(nameof(ClientWires.All), MemberType = typeof(ClientWires))]
    public async Task OpeningWithNoServiceNamedStartsTheBinaryPrivatelyAndClosingStopsIt(ClientWire wire)
    {
        var registry = Registry();

        var connection = await OpenAsync(Options(wire), registry);
        var entry = Assert.Single(registry.Entries());
        var model = (await connection.LoadContentAsync(Notation, cancellationToken: Token)).RequireNoErrors();
        var vehicle = await model.LookupAsync("Vehicle", Token);

        Assert.Equal(services.Version, connection.ServerInfo.Version);
        Assert.True(connection.Address.IsLoopback, connection.Address.ToString());
        Assert.Equal(BinaryResolver.Canonical(Binary), entry.Identity.Path);
        Assert.Equal("1500.0", (await model.EvalAsync("mass", new EvalOptions { ContextSymbolId = vehicle.Id }, Token)).ToString());
        Assert.True(FakeChild.IsRunning(entry.ProcessId!.Value));

        await connection.DisposeAsync();

        Assert.Empty(registry.Entries());
        await FakeChild.AssertGoneAsync(entry.ProcessId.Value);
    }

    [Theory]
    [MemberData(nameof(ClientWires.All), MemberType = typeof(ClientWires))]
    public async Task EightConcurrentOpensShareOneServiceWhichStopsWithTheLast(ClientWire wire)
    {
        var registry = Registry();

        var connections = await Task.WhenAll(Enumerable.Range(0, 8).Select(_ => Task.Run(() => OpenAsync(Options(wire), registry), Token)));
        var entry = Assert.Single(registry.Entries());
        var models = await Task.WhenAll(connections.Select(connection => connection.LoadContentAsync(Notation, cancellationToken: Token)));

        Assert.Equal(8, entry.Holders);
        Assert.Single(connections.Select(connection => connection.Address).Distinct());
        Assert.Single(models.Select(model => model.Hash).Distinct());
        foreach (var connection in connections.Skip(1))
        {
            await connection.DisposeAsync();
        }

        await connections[0].LoadContentAsync(Notation, cancellationToken: Token);
        await connections[0].DisposeAsync();
        await FakeChild.AssertGoneAsync(entry.ProcessId!.Value);
        Assert.Empty(registry.Entries());
    }

    [Fact]
    public async Task IsolatedAndDifferentlyConfiguredOpensStartServicesOfTheirOwn()
    {
        var registry = Registry();

        await using var basis = await OpenAsync(Options(ClientWire.Grpc), registry);
        await using var isolated = await OpenAsync(new ConnectionOptions { BinaryPath = Binary, IsolatedService = true }, registry);
        await using var slower = await OpenAsync(new ConnectionOptions { BinaryPath = Binary, StartupTimeout = TimeSpan.FromSeconds(29) }, registry);
        await using var same = await OpenAsync(new ConnectionOptions { BinaryPath = Binary, Protocol = Protocol.Connect, RequestTimeout = TimeSpan.FromSeconds(9) }, registry);

        Assert.Equal(3, registry.Entries().Select(entry => entry.ProcessId).Distinct().Count());
        Assert.Equal(3, new[] { basis.Address, isolated.Address, slower.Address }.Distinct().Count());
        Assert.Equal(basis.Address, same.Address);
        foreach (var connection in new[] { basis, isolated, slower, same })
        {
            await connection.LoadContentAsync(Notation, cancellationToken: Token);
        }
    }

    [Theory]
    [MemberData(nameof(ClientWires.All), MemberType = typeof(ClientWires))]
    public async Task AServiceKilledFromOutsideFailsItsConnectionAndIsNotRestarted(ClientWire wire)
    {
        var registry = Registry();
        await using var connection = await OpenAsync(Options(wire), registry);
        var processId = Assert.Single(registry.Entries()).ProcessId!.Value;

        FakeChild.Kill(processId);
        await ServiceRegistryTests.WaitUntilAsync(() => registry.Entries().Count == 0);

        var error = await Assert.ThrowsAsync<TransportException>(() => connection.LoadContentAsync(Notation, cancellationToken: Token));
        Assert.Equal(StatusCode.Unavailable, error.StatusCode);
        Assert.Contains($"(process {processId}) exited with code", error.Message, StringComparison.Ordinal);
        Assert.Contains("it is not restarted", error.Message, StringComparison.Ordinal);
        Assert.Empty(registry.Entries());

        await using var reopened = await OpenAsync(Options(wire), registry);
        Assert.NotEqual(processId, Assert.Single(registry.Entries()).ProcessId);
        await reopened.LoadContentAsync(Notation, cancellationToken: Token);
        await Assert.ThrowsAsync<TransportException>(() => connection.LoadContentAsync(Notation, cancellationToken: Token));
    }

    [Fact]
    public async Task StoppingEveryServiceEndsItsConnectionsAndALaterOpenStartsAnother()
    {
        var registry = Registry();
        await using var connection = await OpenAsync(Options(ClientWire.Grpc), registry);
        var processId = Assert.Single(registry.Entries()).ProcessId!.Value;

        Assert.Equal(1, await registry.StopAllAsync(Token));
        await FakeChild.AssertGoneAsync(processId);

        var error = await Assert.ThrowsAsync<TransportException>(() => connection.LoadContentAsync(Notation, cancellationToken: Token));
        Assert.Contains("stopped by Connection.StopSharedServicesAsync", error.Message, StringComparison.Ordinal);
        await using var reopened = await OpenAsync(Options(ClientWire.Grpc), registry);
        Assert.NotEqual(processId, Assert.Single(registry.Entries()).ProcessId);
        await reopened.LoadContentAsync(Notation, cancellationToken: Token);
    }

    [Fact]
    public async Task AServiceOfAnotherVersionIsRefusedAndStopped()
    {
        var registry = Registry();

        var error = await Assert.ThrowsAsync<StaleServiceException>(() => OpenAsync(new ConnectionOptions { BinaryPath = Binary, Version = "v0.0.0-other" }, registry));

        Assert.Equal(services.Version, error.ServiceVersion);
        Assert.Empty(registry.Entries());
    }

    [Fact]
    public async Task TheBinarysOwnDigestIsAcceptedAndAnyOtherRefused()
    {
        var registry = Registry();
        var digest = Convert.ToHexStringLower(SHA256.HashData(await File.ReadAllBytesAsync(Binary, Token)));

        await using (await OpenAsync(new ConnectionOptions { BinaryPath = Binary, ExpectedBinarySha256 = digest }, registry))
        {
            Assert.Equal(digest, Assert.Single(registry.Entries()).Identity.Sha256);
        }

        var error = await Assert.ThrowsAsync<ChecksumMismatchException>(() => OpenAsync(new ConnectionOptions { BinaryPath = Binary, ExpectedBinarySha256 = new string('f', 64) }, registry));
        Assert.Equal(digest, error.ActualSha256);
        Assert.Empty(registry.Entries());
    }

    private static string Binary => ServiceGate.FromProcess.RequireBinary();

    private static ServiceRegistry Registry() => new(PrivateService.StartAsync);

    private static ConnectionOptions Options(ClientWire wire) =>
        new() { BinaryPath = Binary, Protocol = wire.Protocol(), Encoding = wire.Encoding() };

    private static Task<Connection> OpenAsync(ConnectionOptions options, ServiceRegistry registry) =>
        Connection.OpenAsync(options, _ => null, registry, Token);
}

/// <summary>Tests that use the registry every connection of this test run shares, one at a time.</summary>
[CollectionDefinition(nameof(SharedRegistry), DisableParallelization = true)]
public sealed class SharedRegistry;

/// <summary>
/// The public path to a private service: the registry this copy of the client shares, the binary
/// variable, one-shot loads, and stopping every shared service.
/// </summary>
[Trait(Category.Name, Category.Integration)]
[Collection(nameof(SharedRegistry))]
public sealed class SharedServiceTests
{
    private static CancellationToken Token => TestContext.Current.CancellationToken;

    [Fact]
    public async Task ConnectionsAndOneShotLoadsShareTheServiceUntilItIsStoppedForAll()
    {
        var binary = ServiceGate.FromProcess.RequireBinary();
        var options = new ConnectionOptions { BinaryPath = binary };
        Assert.Empty(ServiceRegistry.Shared.Entries());

        await using var connection = await Connection.OpenAsync(options, Token);
        await using var model = await OpenSysMLClient.LoadContentAsync("package P { part def A; }", options, cancellationToken: Token);
        var entry = Assert.Single(ServiceRegistry.Shared.Entries());

        Assert.True(model.OwnsConnection);
        Assert.Equal(connection.Address, model.Connection.Address);
        Assert.Equal(2, entry.Holders);
        Assert.Equal(1, await Connection.StopSharedServicesAsync(Token));
        await FakeChild.AssertGoneAsync(entry.ProcessId!.Value);

        var error = await Assert.ThrowsAsync<TransportException>(() => connection.LoadContentAsync("package Q;", cancellationToken: Token));
        Assert.Equal(StatusCode.Unavailable, error.StatusCode);
        Assert.Equal(0, await Connection.StopSharedServicesAsync(Token));

        await using (var reopened = await Connection.OpenAsync(options, Token))
        {
            Assert.NotEqual(entry.ProcessId, Assert.Single(ServiceRegistry.Shared.Entries()).ProcessId);
            await reopened.LoadContentAsync("package Q;", cancellationToken: Token);
        }

        Assert.Empty(ServiceRegistry.Shared.Entries());
    }

    [Fact]
    public async Task TheBinaryVariableNamesTheServiceADefaultOpenStarts()
    {
        var binary = ServiceGate.FromProcess.RequireBinary();

        await using (var connection = await Connection.OpenAsync(null, name => name == ConnectionOptions.BinaryVariable ? binary : null, Token))
        {
            Assert.True(connection.Address.IsLoopback);
            Assert.Equal(BinaryResolver.Canonical(binary), Assert.Single(ServiceRegistry.Shared.Entries()).Identity.Path);
        }

        Assert.Empty(ServiceRegistry.Shared.Entries());
    }
}
