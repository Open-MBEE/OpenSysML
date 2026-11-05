using System;
using System.Collections.Concurrent;
using System.Collections.Generic;
using System.Diagnostics;
using System.Linq;
using System.Net;
using System.Net.Http;
using System.Text.Json;
using System.Threading;
using System.Threading.Tasks;
using OpenMBEE.OpenSysML.Conformance;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// A connection to a real sysml-grpc named by its address, over every client wire against the
/// service's default connect transport, which serves gRPC and Connect on one port, and over gRPC
/// against its grpc transport, which is grpc-go alone and refuses Connect.
/// </summary>
[Trait(Category.Name, Category.Integration)]
public sealed class ServiceConnectionTests(ServiceFixture services) : IClassFixture<ServiceFixture>
{
    /// <summary>Every pairing of a service transport and a client wire that it serves.</summary>
    public static TheoryData<string, ClientWire> Configurations => new()
    {
        { "connect", ClientWire.Grpc },
        { "connect", ClientWire.ConnectProto },
        { "connect", ClientWire.ConnectJson },
        { "grpc", ClientWire.Grpc },
    };

    [Theory]
    [MemberData(nameof(Configurations))]
    public async Task OpensAnExternalServiceAndReportsWhatItIs(string transport, ClientWire wire)
    {
        var service = await services.ServiceAsync(transport);

        await using var connection = await Connection.OpenAsync(wire.To(service.Address), TestContext.Current.CancellationToken);

        Assert.Equal(new Uri($"http://{service.Address}/"), connection.Address);
        Assert.Equal((wire.Protocol(), wire.Encoding()), (connection.Protocol, connection.Encoding));
        Assert.True(connection.ServerInfo.Answered);
        Assert.Equal(services.Version, connection.ServerInfo.Version);
        Assert.NotEmpty(connection.Capabilities.Names);
        Assert.All(ServerInfoScenarioCapabilities(), capability => Assert.True(connection.Capabilities.Has(capability), capability));
        Assert.Empty(connection.Capabilities.Names.Except(ServiceCapabilities.Names));
    }

    [Theory]
    [MemberData(nameof(Configurations))]
    public async Task TheServiceCanBeNamedByTheEnvironment(string transport, ClientWire wire)
    {
        var service = await services.ServiceAsync(transport);

        await using var connection = await Connection.OpenAsync(
            new ConnectionOptions { Protocol = wire.Protocol(), Encoding = wire.Encoding() },
            name => name == ConnectionOptions.ServiceVariable ? service.Address : null,
            TestContext.Current.CancellationToken);

        Assert.Equal(services.Version, connection.ServerInfo.Version);
    }

    [Theory]
    [MemberData(nameof(Configurations))]
    public async Task ARawCallReachesTheServiceAndItsRefusalIsAServiceException(string transport, ClientWire wire)
    {
        var service = await services.ServiceAsync(transport);
        await using var connection = await Connection.OpenAsync(wire.To(service.Address), TestContext.Current.CancellationToken);

        var info = await connection.CallAsync<Wire.ServerInfoRequest, Wire.ServerInfoResponse>(
            "sysml.SysMLService/GetServerInfo", new Wire.ServerInfoRequest(), TestContext.Current.CancellationToken);
        var refused = await Assert.ThrowsAsync<ServiceException>(() => connection.CallAsync<Wire.GetSymbolRequest, Wire.SymbolResponse>(
            "GetSymbol", new Wire.GetSymbolRequest { ModelHash = "no-such-model", SymbolId = "A" }, TestContext.Current.CancellationToken));

        Assert.Equal(connection.ServerInfo.Version, info.Version);
        Assert.Equal(connection.Capabilities, new Capabilities(info.Capabilities));
        Assert.Equal(StatusCode.NotFound, refused.StatusCode);
        Assert.Equal("sysml.SysMLService/GetSymbol", refused.Method);
        Assert.Equal("model not found: no-such-model", refused.ServiceMessage);
    }

    [Theory]
    [MemberData(nameof(Configurations))]
    public async Task ConcurrentCallsShareOneConnection(string transport, ClientWire wire)
    {
        var service = await services.ServiceAsync(transport);
        await using var connection = await Connection.OpenAsync(wire.To(service.Address), TestContext.Current.CancellationToken);

        var answers = await Task.WhenAll(Enumerable.Range(0, 8).Select(_ => connection.CallAsync<Wire.ServerInfoRequest, Wire.ServerInfoResponse>(
            "GetServerInfo", new Wire.ServerInfoRequest(), TestContext.Current.CancellationToken)));

        Assert.All(answers, answer => Assert.Equal(connection.ServerInfo.Version, answer.Version));
    }

    [Theory]
    [MemberData(nameof(Configurations))]
    public async Task RequiredCapabilitiesAndVersionAreCheckedAgainstTheServicesAnswer(string transport, ClientWire wire)
    {
        var service = await services.ServiceAsync(transport);
        var token = TestContext.Current.CancellationToken;

        await using (await Connection.OpenAsync(
            new ConnectionOptions
            {
                Address = service.Address,
                Protocol = wire.Protocol(),
                Encoding = wire.Encoding(),
                Version = services.Version,
                RequireCapabilities = [Capabilities.Query, Capabilities.TypeFacts],
            },
            token))
        {
        }

        var missing = await Assert.ThrowsAsync<CapabilityException>(() => Connection.OpenAsync(
            new ConnectionOptions { Address = service.Address, Protocol = wire.Protocol(), Encoding = wire.Encoding(), RequireCapabilities = ["no_such_capability"] },
            token));
        var stale = await Assert.ThrowsAsync<StaleServiceException>(() => Connection.OpenAsync(
            new ConnectionOptions { Address = service.Address, Protocol = wire.Protocol(), Encoding = wire.Encoding(), Version = "v0.0.0-not-this-one" },
            token));

        Assert.Equal("no_such_capability", missing.Capability);
        Assert.Equal(services.Version, stale.ServiceVersion);
        Assert.False(service.HasExited);
    }

    [Theory]
    [MemberData(nameof(Configurations))]
    public async Task AResponseOverTheLimitIsRefusedHoweverTheServiceCouldCompressIt(string transport, ClientWire wire)
    {
        var service = await services.ServiceAsync(transport);
        var token = TestContext.Current.CancellationToken;
        var size = await AnswerSizeAsync(service, wire, token);
        using var observer = new ResponseObserver();
        ConnectionOptions Limited(int limit) => new()
        {
            Address = service.Address,
            Protocol = wire.Protocol(),
            Encoding = wire.Encoding(),
            MaxResponseSize = limit,
            HttpMessageHandler = observer,
        };

        await using (await Connection.OpenAsync(Limited(size), token))
        {
        }

        // Compressed, the answer would fit a byte below its size; the limit must bound what it inflates to.
        var error = await Assert.ThrowsAsync<TransportException>(() => Connection.OpenAsync(Limited(size - 1), token));

        Assert.Equal(StatusCode.ResourceExhausted, error.StatusCode);
        if (wire != ClientWire.Grpc)
        {
            Assert.Equal(["gzip", "gzip"], observer.ContentEncodings);
        }
    }

    [Theory]
    [MemberData(nameof(ClientWires.Connect), MemberType = typeof(ClientWires))]
    public async Task TheServiceGzipsAConnectAnswerAndTheClientReadsItAsGrpcDoes(ClientWire wire)
    {
        var service = await services.ServiceAsync("connect");
        var token = TestContext.Current.CancellationToken;
        using var observer = new ResponseObserver();

        await using var overGrpc = await Connection.OpenAsync(ClientWire.Grpc.To(service.Address), token);
        await using var overConnect = await Connection.OpenAsync(
            new ConnectionOptions { Address = service.Address, Protocol = wire.Protocol(), Encoding = wire.Encoding(), HttpMessageHandler = observer },
            token);

        Assert.Equal(overGrpc.ServerInfo, overConnect.ServerInfo);
        Assert.Equal(["gzip"], observer.ContentEncodings);
    }

    [Theory]
    [MemberData(nameof(ClientWires.Connect), MemberType = typeof(ClientWires))]
    public async Task TheGrpcOnlyTransportRefusesConnectAndTheClientDoesNotFallBackToGrpc(ClientWire wire)
    {
        var service = await services.ServiceAsync("grpc");
        using var observer = new ResponseObserver();

        var error = await Assert.ThrowsAsync<TransportException>(() => Connection.OpenAsync(
            new ConnectionOptions { Address = service.Address, Protocol = wire.Protocol(), Encoding = wire.Encoding(), HttpMessageHandler = observer },
            TestContext.Current.CancellationToken));

        Assert.Equal(StatusCode.Unavailable, error.StatusCode);
        Assert.Equal("sysml.SysMLService/GetServerInfo", error.Method);
        var sent = Assert.Single(observer.Requests);
        Assert.Equal(HttpVersion.Version11, sent.Version);
        Assert.Empty(observer.ContentEncodings);
        Assert.False(service.HasExited);
    }

    [Theory]
    [MemberData(nameof(Configurations))]
    public async Task ClosingConnectionsNeverStopsAnExternalService(string transport, ClientWire wire)
    {
        var binary = ServiceGate.FromProcess.RequireBinary();
        var token = TestContext.Current.CancellationToken;
        await using var service = await ServiceProcess.StartAsync(binary, transport, token);
        var options = wire.To(service.Address);

        var first = await Connection.OpenAsync(options, token);
        var second = await Connection.OpenAsync(options, token);
        await first.DisposeAsync();
        await second.DisposeAsync();
        await Task.Delay(TimeSpan.FromMilliseconds(200), token);

        Assert.False(service.HasExited, service.ToString());
        await using var third = await Connection.OpenAsync(options, token);
        Assert.True(third.ServerInfo.Answered);
    }

    private static IEnumerable<string> ServerInfoScenarioCapabilities()
    {
        using var file = JsonDocument.Parse(System.IO.File.ReadAllBytes(Repository.PathOf("conformance", "scenarios", "01-server-info.json")));
        var scenario = file.RootElement.GetProperty("scenarios").EnumerateArray()
            .Single(scenario => scenario.GetProperty("id").GetString() == "server_info/reports_the_capabilities_clients_negotiate_on");
        return [.. scenario.GetProperty("expect").GetProperty("contains_all").GetProperty("capabilities").EnumerateArray().Select(name => name.GetString()!)];
    }

    // The size a limit measures GetServerInfo's answer at: the protobuf message, or the JSON body the service writes, uncompressed.
    private static async Task<int> AnswerSizeAsync(ServiceProcess service, ClientWire wire, CancellationToken token)
    {
        if (wire != ClientWire.ConnectJson)
        {
            await using var measured = await Connection.OpenAsync(wire.To(service.Address), token);
            var answer = new Wire.ServerInfoResponse { Version = measured.ServerInfo.Version };
            answer.Capabilities.AddRange(measured.Capabilities.Names);
            return answer.CalculateSize();
        }

        using var http = new HttpClient();
        using var request = new HttpRequestMessage(HttpMethod.Post, $"http://{service.Address}/sysml.SysMLService/GetServerInfo")
        {
            Content = new StringContent("{}", System.Text.Encoding.UTF8, "application/json"),
        };
        request.Headers.Add("Connect-Protocol-Version", "1");
        using var response = await http.SendAsync(request, token);
        Assert.Empty(response.Content.Headers.ContentEncoding);
        return (await response.Content.ReadAsByteArrayAsync(token)).Length;
    }

    /// <summary>A handler lent to a connection that records what it sends and the coding of what it is answered.</summary>
    private sealed class ResponseObserver() : DelegatingHandler(new SocketsHttpHandler())
    {
        private readonly ConcurrentQueue<HttpRequestMessage> requests = new();
        private readonly ConcurrentQueue<string> contentEncodings = new();

        internal IReadOnlyList<HttpRequestMessage> Requests => [.. requests];

        internal IReadOnlyList<string> ContentEncodings => [.. contentEncodings];

        protected override async Task<HttpResponseMessage> SendAsync(HttpRequestMessage request, CancellationToken cancellationToken)
        {
            requests.Enqueue(request);
            var response = await base.SendAsync(request, cancellationToken);
            foreach (var coding in response.Content.Headers.ContentEncoding)
            {
                contentEncodings.Enqueue(coding);
            }

            return response;
        }
    }
}

/// <summary>One sysml-grpc per transport, started when a test first needs it and stopped after the class.</summary>
public sealed class ServiceFixture : IAsyncDisposable
{
    private readonly ConcurrentDictionary<string, Lazy<Task<ServiceProcess>>> services = new(StringComparer.Ordinal);
    private readonly Lazy<string> version = new(ReadVersion);

    /// <summary>The version the configured binary reports.</summary>
    internal string Version => version.Value;

    /// <summary>The service serving a transport; skips or fails the test without a binary.</summary>
    /// <param name="transport">connect or grpc.</param>
    /// <param name="withheld">Capabilities it withholds, comma-separated; null for none.</param>
    internal Task<ServiceProcess> ServiceAsync(string transport, string? withheld = null)
    {
        var binary = ServiceGate.FromProcess.RequireBinary();
        return services.GetOrAdd(
            withheld is null ? transport : $"{transport} without {withheld}",
            _ => new Lazy<Task<ServiceProcess>>(() => ServiceProcess.StartAsync(binary, transport, CancellationToken.None, withheld))).Value;
    }

    public async ValueTask DisposeAsync()
    {
        foreach (var service in services.Values.Where(service => service.IsValueCreated))
        {
            if (service.Value.IsCompletedSuccessfully)
            {
                await (await service.Value).DisposeAsync();
            }
        }
    }

    private static string ReadVersion()
    {
        var binary = ServiceGate.FromProcess.RequireBinary();
        var start = new ProcessStartInfo(binary, "-version") { RedirectStandardOutput = true, UseShellExecute = false };
        using var process = Process.Start(start) ?? throw new InvalidOperationException($"{binary} did not start.");
        var first = process.StandardOutput.ReadLine() ?? string.Empty;
        process.StandardOutput.ReadToEnd();
        process.WaitForExit();
        const string Prefix = "sysml-grpc version ";
        Assert.StartsWith(Prefix, first, StringComparison.Ordinal);
        return first[Prefix.Length..].Trim();
    }
}
