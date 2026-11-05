using System;
using System.Collections.Generic;
using System.Security.Cryptography;
using System.Threading;
using System.Threading.Tasks;
using OpenMBEE.OpenSysML.Conformance;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// A real sysml-grpc that requires a bearer token, reached over every wire with the right token, none and a wrong
/// one; and a private service started while <see cref="ConnectionOptions.TokenVariable"/> names a token, which must
/// require none.
/// </summary>
[Trait(Category.Name, Category.Integration)]
public sealed class ServiceTokenTests(TokenServiceFixture service) : IClassFixture<TokenServiceFixture>
{
    [Theory]
    [MemberData(nameof(ClientWires.All), MemberType = typeof(ClientWires))]
    public async Task TheServicesTokenOpensIt(ClientWire wire)
    {
        var running = await service.ServiceAsync();

        await using var connection = await Connection.OpenAsync(Options(wire, running.Address, service.Token), NoEnvironment, TestContext.Current.CancellationToken);

        Assert.True(connection.ServerInfo.Answered);
        Assert.NotEmpty(connection.Capabilities.Names);
    }

    [Theory]
    [MemberData(nameof(ClientWires.All), MemberType = typeof(ClientWires))]
    public async Task TheTokenTheVariableNamesOpensIt(ClientWire wire)
    {
        var running = await service.ServiceAsync();

        await using var connection = await Connection.OpenAsync(
            Options(wire, running.Address, token: null),
            name => name == ConnectionOptions.TokenVariable ? service.Token : null,
            TestContext.Current.CancellationToken);

        Assert.True(connection.ServerInfo.Answered);
    }

    [Theory]
    [MemberData(nameof(ClientWires.All), MemberType = typeof(ClientWires))]
    public async Task NoTokenIsAnAuthenticationExceptionSayingToSupplyOne(ClientWire wire)
    {
        var running = await service.ServiceAsync();

        var error = await Assert.ThrowsAsync<AuthenticationException>(() =>
            Connection.OpenAsync(Options(wire, running.Address, token: null), NoEnvironment, TestContext.Current.CancellationToken));

        Assert.False(error.CredentialsSent);
        Assert.Contains("requires a bearer token", error.ServiceMessage, StringComparison.Ordinal);
        Assert.StartsWith("the service requires a bearer token and none was sent", error.Remedy, StringComparison.Ordinal);
    }

    [Theory]
    [MemberData(nameof(ClientWires.All), MemberType = typeof(ClientWires))]
    public async Task AWrongTokenIsAnAuthenticationExceptionSayingItWasRefused(ClientWire wire)
    {
        var running = await service.ServiceAsync();

        var error = await Assert.ThrowsAsync<AuthenticationException>(() =>
            Connection.OpenAsync(Options(wire, running.Address, "not-the-services-token"), NoEnvironment, TestContext.Current.CancellationToken));

        Assert.True(error.CredentialsSent);
        Assert.Contains("the bearer token is not this server's", error.ServiceMessage, StringComparison.Ordinal);
        Assert.StartsWith("the bearer token ConnectionOptions.Token gives was refused", error.Remedy, StringComparison.Ordinal);
        Assert.DoesNotContain(service.Token, error.ToString(), StringComparison.Ordinal);
    }

    [Theory]
    [MemberData(nameof(ClientWires.All), MemberType = typeof(ClientWires))]
    public async Task APrivateServiceStartedWhileTheVariableNamesATokenRequiresNone(ClientWire wire)
    {
        var binary = ServiceGate.FromProcess.RequireBinary();
        var environment = new Dictionary<string, string> { [ConnectionOptions.TokenVariable] = "a-token-meant-for-a-named-service" };
        await using var probe = ProbeRun.Start(ProbeRun.Orphan, environment, "serve", binary, ProbeRun.Argument(wire), "0");
        var report = await probe.ReadAsync();
        var address = report.GetProperty("address").GetString()!;

        // The probe's own connection carried the token, which proves nothing; a call carrying none shows the child demands none.
        await using var connection = await Connection.OpenAsync(Options(wire, address, token: null), NoEnvironment, TestContext.Current.CancellationToken);

        Assert.True(connection.ServerInfo.Answered);
    }

    private static string? NoEnvironment(string name) => null;

    private static ConnectionOptions Options(ClientWire wire, string address, string? token) =>
        new() { Address = address, Protocol = wire.Protocol(), Encoding = wire.Encoding(), Token = token };
}

/// <summary>One sysml-grpc that requires a random bearer token, started when a test first needs it and stopped after the class.</summary>
public sealed class TokenServiceFixture : IAsyncDisposable
{
    private readonly Lazy<Task<ServiceProcess>> service;

    /// <summary>Creates the fixture; nothing starts until a test asks for the service.</summary>
    public TokenServiceFixture()
    {
        service = new Lazy<Task<ServiceProcess>>(() =>
            ServiceProcess.StartAsync(ServiceGate.FromProcess.RequireBinary(), "connect", CancellationToken.None, token: Token));
    }

    /// <summary>The token the service requires: random, and of the RFC 6750 form base64 is.</summary>
    internal string Token { get; } = Convert.ToBase64String(RandomNumberGenerator.GetBytes(24));

    /// <summary>The service, serving the connect transport, which carries every client wire; skips or fails the test without a binary.</summary>
    internal Task<ServiceProcess> ServiceAsync()
    {
        ServiceGate.FromProcess.RequireBinary();
        return service.Value;
    }

    public async ValueTask DisposeAsync()
    {
        if (service.IsValueCreated && service.Value.IsCompletedSuccessfully)
        {
            await (await service.Value).DisposeAsync();
        }
    }
}
