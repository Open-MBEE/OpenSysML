using System;
using System.Collections.Concurrent;
using System.Collections.Generic;
using System.Net;
using System.Net.Http;
using System.Net.Sockets;
using System.Threading;
using System.Threading.Tasks;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// What a connection reports when the endpoint is not the service: real sockets on the loopback
/// interface, through the handler a connection makes for itself. A failure every protocol shares is
/// a theory over all three wires; one particular to a protocol names it.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed class NetworkFailureTests
{
    private static readonly TimeSpan Timeout = TimeSpan.FromSeconds(20);

    [Theory]
    [MemberData(nameof(ClientWires.All), MemberType = typeof(ClientWires))]
    public async Task ARefusedPortIsATransportUnavailableCausedByTheRefusal(ClientWire wire)
    {
        var listener = new TcpListener(IPAddress.Loopback, 0);
        listener.Start();
        var port = ((IPEndPoint)listener.LocalEndpoint).Port;
        listener.Stop();

        var error = await Assert.ThrowsAsync<TransportException>(() => OpenAsync(wire, $"127.0.0.1:{port}"));

        Assert.Equal(StatusCode.Unavailable, error.StatusCode);
        Assert.Equal("sysml.SysMLService/GetServerInfo", error.Method);
        Assert.Contains(Chain(error), cause => cause is SocketException { SocketErrorCode: SocketError.ConnectionRefused });
    }

    [Theory]
    [MemberData(nameof(ClientWires.All), MemberType = typeof(ClientWires))]
    public async Task AnUnresolvableHostIsATransportUnavailableCausedByTheLookup(ClientWire wire)
    {
        var error = await Assert.ThrowsAsync<TransportException>(() => OpenAsync(wire, "no-such-host.invalid:50051"));

        Assert.Equal(StatusCode.Unavailable, error.StatusCode);
        Assert.Contains(Chain(error), cause => cause is SocketException);
    }

    [Theory]
    [MemberData(nameof(ClientWires.All), MemberType = typeof(ClientWires))]
    public async Task AnEndpointThatDoesNotSpeakTlsFailsAnHttpsConnection(ClientWire wire)
    {
        await using var server = RawTcpServer.Start("this is not TLS\r\n"u8.ToArray());

        var error = await Assert.ThrowsAsync<TransportException>(() => OpenAsync(wire, $"https://127.0.0.1:{server.Port}"));

        Assert.Equal(StatusCode.Unavailable, error.StatusCode);
        Assert.Contains(Chain(error), cause => cause is HttpRequestException { HttpRequestError: HttpRequestError.SecureConnectionError });
    }

    [Theory]
    [MemberData(nameof(ClientWires.All), MemberType = typeof(ClientWires))]
    public async Task AnEndpointThatNeverAnswersFailsAtTheClientsOwnDeadline(ClientWire wire)
    {
        await using var server = RawTcpServer.Silent();
        var deadline = TimeSpan.FromMilliseconds(500);
        var clock = System.Diagnostics.Stopwatch.StartNew();

        // Only the client's own deadline can end the call; the caller's token is a backstop that fails the test instead of hanging it.
        using var backstop = CancellationTokenSource.CreateLinkedTokenSource(TestContext.Current.CancellationToken);
        backstop.CancelAfter(Timeout);
        var error = await Assert.ThrowsAsync<TransportException>(() =>
            Connection.OpenAsync(new ConnectionOptions { Address = $"127.0.0.1:{server.Port}", Protocol = wire.Protocol(), Encoding = wire.Encoding(), RequestTimeout = deadline }, backstop.Token));

        Assert.Equal(StatusCode.DeadlineExceeded, error.StatusCode);
        Assert.Contains("did not complete before its deadline", error.Message, StringComparison.Ordinal);
        Assert.InRange(clock.Elapsed, deadline - TimeSpan.FromMilliseconds(50), Timeout);
    }

    [Fact]
    public async Task OverGrpcAnEndpointThatSpeaksOnlyHttp11IsATransportInternalThatSaysSo()
    {
        await using var server = RawTcpServer.Start("HTTP/1.1 400 Bad Request\r\nContent-Length: 0\r\nConnection: close\r\n\r\n"u8.ToArray());

        var error = await Assert.ThrowsAsync<TransportException>(() => OpenAsync(ClientWire.Grpc, $"127.0.0.1:{server.Port}"));

        Assert.Equal(StatusCode.Internal, error.StatusCode);
        Assert.Contains(Chain(error), cause => cause is HttpProtocolException
            or HttpRequestException { HttpRequestError: HttpRequestError.VersionNegotiationError });
        Assert.Contains("did not answer as an HTTP/2 gRPC endpoint", error.Message, StringComparison.Ordinal);
        Assert.Contains("Protocol.Connect", error.Message, StringComparison.Ordinal);
    }

    [Theory]
    [MemberData(nameof(ClientWires.Connect), MemberType = typeof(ClientWires))]
    public async Task OverConnectAnHttpRefusalWithoutAConnectErrorTakesItsStatusFromHttp(ClientWire wire)
    {
        await using var server = RawTcpServer.Start(
            "HTTP/1.1 400 Bad Request\r\nContent-Type: text/plain\r\nContent-Length: 11\r\nConnection: close\r\n\r\nbad request"u8.ToArray());

        var error = await Assert.ThrowsAsync<TransportException>(() => OpenAsync(wire, $"127.0.0.1:{server.Port}"));

        Assert.Equal(StatusCode.Internal, error.StatusCode);
        Assert.Contains("answered HTTP 400 Bad Request with text/plain and \"bad request\"", error.Message, StringComparison.Ordinal);
    }

    [Theory]
    [MemberData(nameof(ClientWires.Connect), MemberType = typeof(ClientWires))]
    public async Task OverConnectARedirectIsNotFollowed(ClientWire wire)
    {
        await using var target = RawTcpServer.Start("HTTP/1.1 200 OK\r\nContent-Length: 0\r\nConnection: close\r\n\r\n"u8.ToArray());
        await using var server = RawTcpServer.Start(System.Text.Encoding.ASCII.GetBytes(
            $"HTTP/1.1 307 Temporary Redirect\r\nLocation: http://127.0.0.1:{target.Port}/sysml.SysMLService/GetServerInfo\r\nContent-Length: 0\r\nConnection: close\r\n\r\n"));

        var error = await Assert.ThrowsAsync<TransportException>(() => OpenAsync(wire, $"127.0.0.1:{server.Port}"));

        Assert.Equal(StatusCode.Unknown, error.StatusCode);
        Assert.Contains("HTTP 307", error.Message, StringComparison.Ordinal);
        Assert.Equal(0, target.Connections);
    }

    [Theory]
    [MemberData(nameof(ClientWires.Connect), MemberType = typeof(ClientWires))]
    public async Task OverConnectAnEndpointThatDoesNotSpeakHttpIsATransportInternal(ClientWire wire)
    {
        await using var server = RawTcpServer.Start("this is not HTTP\r\n\r\n"u8.ToArray());

        var error = await Assert.ThrowsAsync<TransportException>(() => OpenAsync(wire, $"127.0.0.1:{server.Port}"));

        Assert.Equal(StatusCode.Internal, error.StatusCode);
        Assert.Contains("did not answer as an HTTP/1.1 endpoint", error.Message, StringComparison.Ordinal);
    }

    [Theory]
    [MemberData(nameof(ClientWires.Connect), MemberType = typeof(ClientWires))]
    public async Task OverConnectAConnectionClosedBeforeAnAnswerIsUnavailable(ClientWire wire)
    {
        await using var server = RawTcpServer.Start([]);

        var error = await Assert.ThrowsAsync<TransportException>(() => OpenAsync(wire, $"127.0.0.1:{server.Port}"));

        Assert.Equal(StatusCode.Unavailable, error.StatusCode);
        Assert.Contains("Whether the service received the call is unknown", error.Message, StringComparison.Ordinal);
    }

    [Theory]
    [MemberData(nameof(ClientWires.Connect), MemberType = typeof(ClientWires))]
    public async Task OverConnectAnAnswerCutShortIsATransportInternal(ClientWire wire)
    {
        var type = wire == ClientWire.ConnectJson ? "application/json" : "application/proto";
        await using var server = RawTcpServer.Start(System.Text.Encoding.ASCII.GetBytes(
            $"HTTP/1.1 200 OK\r\nContent-Type: {type}\r\nContent-Length: 100\r\nConnection: close\r\n\r\n{{\"vers"));

        var error = await Assert.ThrowsAsync<TransportException>(() => OpenAsync(wire, $"127.0.0.1:{server.Port}"));

        Assert.Equal(StatusCode.Internal, error.StatusCode);
    }

    private static async Task<Connection> OpenAsync(ClientWire wire, string address)
    {
        using var timeout = CancellationTokenSource.CreateLinkedTokenSource(TestContext.Current.CancellationToken);
        timeout.CancelAfter(Timeout);
        return await Connection.OpenAsync(
            new ConnectionOptions { Address = address, Protocol = wire.Protocol(), Encoding = wire.Encoding(), RequestTimeout = Timeout },
            timeout.Token);
    }

    private static List<Exception> Chain(Exception error)
    {
        var chain = new List<Exception>();
        for (Exception? cause = error; cause is not null; cause = cause.InnerException)
        {
            chain.Add(cause);
        }

        return chain;
    }
}

/// <summary>
/// A TCP server on the loopback interface that answers every connection with fixed bytes and closes
/// it, or, when silent, accepts and holds every connection open, answering nothing, until disposed.
/// </summary>
internal sealed class RawTcpServer : IAsyncDisposable
{
    private readonly TcpListener listener;
    private readonly CancellationTokenSource stopping = new();
    private readonly ConcurrentBag<Task> serving = [];
    private readonly Task accepting;
    private int connections;

    private RawTcpServer(TcpListener listener, byte[]? answer)
    {
        this.listener = listener;
        accepting = AcceptAsync(answer);
    }

    /// <summary>The port it listens on.</summary>
    internal int Port => ((IPEndPoint)listener.LocalEndpoint).Port;

    /// <summary>How many connections it accepted.</summary>
    internal int Connections => Volatile.Read(ref connections);

    /// <summary>Starts a server answering with these bytes.</summary>
    internal static RawTcpServer Start(byte[] answer) => new(Listen(), answer);

    /// <summary>Starts a server that answers nothing.</summary>
    internal static RawTcpServer Silent() => new(Listen(), answer: null);

    public async ValueTask DisposeAsync()
    {
        await stopping.CancelAsync();
        listener.Stop();
        await accepting;
        await Task.WhenAll(serving);
        stopping.Dispose();
    }

    /// <summary>Where it listens, for a test's message.</summary>
    public override string ToString() => $"{nameof(RawTcpServer)} on {Port}";

    private static TcpListener Listen()
    {
        var listener = new TcpListener(IPAddress.Loopback, 0);
        listener.Start();
        return listener;
    }

    private async Task AcceptAsync(byte[]? answer)
    {
        while (!stopping.IsCancellationRequested)
        {
            Socket client;
            try
            {
                client = await listener.AcceptSocketAsync(stopping.Token);
            }
            catch (Exception error) when (error is OperationCanceledException or SocketException or ObjectDisposedException)
            {
                return;
            }

            Interlocked.Increment(ref connections);
            serving.Add(ServeAsync(client, answer));
        }
    }

    private async Task ServeAsync(Socket client, byte[]? answer)
    {
        using (client)
        {
            try
            {
                if (answer is null)
                {
                    await Task.Delay(System.Threading.Timeout.Infinite, stopping.Token);
                    return;
                }

                var buffer = new byte[4096];
                using var read = CancellationTokenSource.CreateLinkedTokenSource(stopping.Token);
                read.CancelAfter(TimeSpan.FromSeconds(2));
                await client.ReceiveAsync(buffer, SocketFlags.None, read.Token);
                await client.SendAsync(answer, SocketFlags.None, stopping.Token);
                client.Shutdown(SocketShutdown.Send);

                // Closing with part of the request unread would reset the connection before the client reads the answer.
                using var drain = CancellationTokenSource.CreateLinkedTokenSource(stopping.Token);
                drain.CancelAfter(TimeSpan.FromSeconds(2));
                while (await client.ReceiveAsync(buffer, SocketFlags.None, drain.Token) > 0)
                {
                }
            }
            catch (Exception error) when (error is OperationCanceledException or SocketException)
            {
            }
        }
    }
}
