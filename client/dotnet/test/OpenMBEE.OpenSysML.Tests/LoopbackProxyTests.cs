using System;
using System.Net;
using System.Net.Http;
using System.Threading.Tasks;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>Tests that replace the process's default proxy, one at a time.</summary>
[CollectionDefinition(nameof(ProcessDefaultProxy), DisableParallelization = true)]
public sealed class ProcessDefaultProxy;

/// <summary>
/// A private service is reached directly, whatever proxy the process defaults to. HTTP_PROXY with
/// NO_PROXY=localhost proxies 127.0.0.1, where the service listens: the proxy would read a Connect
/// call in cleartext, and could not carry a gRPC one, so the open would fail either way.
/// </summary>
[Trait(Category.Name, Category.Integration)]
[Collection(nameof(ProcessDefaultProxy))]
public sealed class LoopbackProxyTests
{
    [Theory]
    [MemberData(nameof(ClientWires.All), MemberType = typeof(ClientWires))]
    public async Task APrivateServiceIsReachedWithoutTheDefaultProxy(ClientWire wire)
    {
        var binary = ServiceGate.FromProcess.RequireBinary();
        await using var proxy = RawTcpServer.Start("HTTP/1.1 502 Bad Gateway\r\nContent-Length: 0\r\nConnection: close\r\n\r\n"u8.ToArray());
        var previous = HttpClient.DefaultProxy;
        HttpClient.DefaultProxy = new EveryAddressProxy(new Uri($"http://127.0.0.1:{proxy.Port}"));
        try
        {
            await using var connection = await Connection.OpenAsync(
                new ConnectionOptions { BinaryPath = binary, IsolatedService = true, Protocol = wire.Protocol(), Encoding = wire.Encoding() },
                TestContext.Current.CancellationToken);
            Assert.True(connection.Address.IsLoopback);
        }
        finally
        {
            HttpClient.DefaultProxy = previous;
        }

        Assert.Equal(0, proxy.Connections);
    }

    /// <summary>A proxy for every address, loopback included, as HTTP_PROXY with NO_PROXY=localhost is for 127.0.0.1.</summary>
    private sealed class EveryAddressProxy(Uri address) : IWebProxy
    {
        public ICredentials? Credentials { get; set; }

        public Uri GetProxy(Uri destination) => address;

        public bool IsBypassed(Uri host) => false;
    }
}
