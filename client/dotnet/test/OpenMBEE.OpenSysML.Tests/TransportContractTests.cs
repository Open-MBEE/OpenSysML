using System;
using System.Linq;
using System.Net.Http;
using System.Net.Sockets;
using System.Threading.Tasks;
using OpenMBEE.OpenSysML.Internal;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// The transport contract every wire keeps: how a status the service sends, a network failure and
/// a size limit are reported, and what a connection owns. Each test runs over gRPC, Connect with
/// protobuf and Connect with JSON.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed class TransportContractTests
{
    private const string GetSymbol = "sysml.SysMLService/GetSymbol";
    private static readonly Uri Address = new("http://fake.invalid:50051/");

    public static MatrixTheoryData<ClientWire, StatusCode> RefusalStatuses =>
        new([ClientWire.Grpc, ClientWire.ConnectProto, ClientWire.ConnectJson], [.. Enum.GetValues<StatusCode>().Where(code => code != StatusCode.Ok)]);

    [Theory]
    [MemberData(nameof(RefusalStatuses))]
    public async Task EveryStatusTheServiceSendsIsAServiceExceptionButADeadline(ClientWire wire, StatusCode status)
    {
        using var handler = new FakeServiceHandler(wire);
        await using var connection = await FakeConnection.OpenAsync(handler);
        handler.Respond = (_, _) => Task.FromResult(handler.Refuse(status, "said the service"));

        var error = await Assert.ThrowsAnyAsync<OpenSysMLException>(() => connection.GetSymbolAsync());

        if (status == StatusCode.DeadlineExceeded)
        {
            var transport = Assert.IsType<TransportException>(error);
            Assert.Equal(StatusCode.DeadlineExceeded, transport.StatusCode);
            Assert.Contains("reports that the call outlived its deadline", transport.Message, StringComparison.Ordinal);
        }
        else
        {
            // Unauthenticated is the one status with a type of its own, which says what the call carried.
            Assert.IsType(status == StatusCode.Unauthenticated ? typeof(AuthenticationException) : typeof(ServiceException), error);
            var service = (ServiceException)error;
            Assert.Equal(status, service.StatusCode);
            Assert.Equal("said the service", service.ServiceMessage);
            Assert.Equal(GetSymbol, service.Method);
        }
    }

    [Theory]
    [MemberData(nameof(ClientWires.All), MemberType = typeof(ClientWires))]
    public async Task UnavailableFromTheServiceIsNotUnavailableFromTheNetwork(ClientWire wire)
    {
        using var handler = new FakeServiceHandler(wire);
        await using var connection = await FakeConnection.OpenAsync(handler);

        handler.Respond = (_, _) => Task.FromResult(handler.Refuse(StatusCode.Unavailable, "draining"));
        var service = await Assert.ThrowsAsync<ServiceException>(() => connection.GetSymbolAsync());

        handler.Respond = (_, _) => throw new HttpRequestException(
            HttpRequestError.ConnectionError, "refused", new SocketException((int)SocketError.ConnectionRefused));
        var network = await Assert.ThrowsAsync<TransportException>(() => connection.GetSymbolAsync());

        Assert.Equal(StatusCode.Unavailable, service.StatusCode);
        Assert.Equal(StatusCode.Unavailable, network.StatusCode);
        Assert.Equal(SocketError.ConnectionRefused, Assert.IsType<SocketException>(Assert.IsType<HttpRequestException>(network.InnerException).InnerException).SocketErrorCode);
        Assert.Contains("Whether the service received the call is unknown", network.Message, StringComparison.Ordinal);
    }

    [Theory]
    [MemberData(nameof(ClientWires.All), MemberType = typeof(ClientWires))]
    public async Task ARefusedCallIsSentOnceOnItsOwnWireAndNeverRetriedOnAnother(ClientWire wire)
    {
        using var handler = new FakeServiceHandler(wire);
        await using var connection = await FakeConnection.OpenAsync(handler);
        handler.Respond = (_, _) => Task.FromResult(handler.Refuse(StatusCode.Unavailable, "draining"));

        await Assert.ThrowsAsync<ServiceException>(() => connection.GetSymbolAsync());

        var expected = wire switch
        {
            ClientWire.Grpc => "application/grpc",
            ClientWire.ConnectProto => "application/proto",
            _ => "application/json",
        };
        Assert.Single(handler.RequestsFor("GetSymbol"));
        Assert.All(handler.Requests, request => Assert.Equal(expected, request.ContentType));
    }

    [Theory]
    [MemberData(nameof(ClientWires.All), MemberType = typeof(ClientWires))]
    public async Task AResponseAtTheLimitIsReadAndOneByteMoreIsNot(ClientWire wire)
    {
        const int Limit = 1024;
        using var handler = new FakeServiceHandler(wire);
        await using var connection = await FakeConnection.OpenAsync(handler, FakeConnection.Options(handler, maxResponseSize: Limit));

        var atLimit = ResponseOfSize(handler, Limit);
        handler.Respond = (_, _) => Task.FromResult(handler.Answer(atLimit));
        Assert.Equal(atLimit, await connection.GetSymbolAsync());

        var overLimit = ResponseOfSize(handler, Limit + 1);
        handler.Respond = (_, _) => Task.FromResult(handler.Answer(overLimit));
        var error = await Assert.ThrowsAsync<TransportException>(() => connection.GetSymbolAsync());
        Assert.Equal(StatusCode.ResourceExhausted, error.StatusCode);
    }

    [Theory]
    [MemberData(nameof(ClientWires.All), MemberType = typeof(ClientWires))]
    public async Task ARequestAtTheLimitIsSentAndOneByteMoreIsNot(ClientWire wire)
    {
        const int Limit = 512;
        using var handler = new FakeServiceHandler(wire) { Respond = (_, _) => Task.FromResult(FakeServiceHandler.AnswerOn(wire, new Wire.SymbolResponse())) };
        await using var connection = await FakeConnection.OpenAsync(handler, FakeConnection.Options(handler, maxRequestSize: Limit));

        var atLimit = RequestOfSize(handler, Limit);
        await connection.CallAsync<Wire.GetSymbolRequest, Wire.SymbolResponse>("GetSymbol", atLimit, TestContext.Current.CancellationToken);
        var sent = handler.RequestsFor("GetSymbol").Single();
        Assert.Equal(Limit, sent.MessageBytes.Length);
        Assert.Equal(atLimit, sent.Message(Wire.GetSymbolRequest.Parser));

        var error = await Assert.ThrowsAsync<TransportException>(() =>
            connection.CallAsync<Wire.GetSymbolRequest, Wire.SymbolResponse>("GetSymbol", RequestOfSize(handler, Limit + 1), TestContext.Current.CancellationToken));
        Assert.Equal(StatusCode.ResourceExhausted, error.StatusCode);
        Assert.Single(handler.RequestsFor("GetSymbol"));
    }

    [Theory]
    [MemberData(nameof(ClientWires.All), MemberType = typeof(ClientWires))]
    public async Task DisposingTheTransportDisposesAnOwnedHandlerAndOnlyThat(ClientWire wire)
    {
        var owned = new FakeServiceHandler(wire);
        var borrowed = new FakeServiceHandler(wire);
        using (borrowed)
        {
            var options = new ConnectionOptions { Protocol = wire.Protocol(), Encoding = wire.Encoding() };

            await Transport(owned, ownsHandler: true, options).DisposeAsync();
            var transport = Transport(borrowed, ownsHandler: false, options);
            await transport.DisposeAsync();
            await transport.DisposeAsync();

            Assert.Equal(1, owned.Disposals);
            Assert.Equal(0, borrowed.Disposals);
        }
    }

    private static ITransport Transport(FakeServiceHandler handler, bool ownsHandler, ConnectionOptions options) =>
        handler.Wire == ClientWire.Grpc
            ? new GrpcTransport(Address, handler, ownsHandler, options, CallHeaders.None)
            : new ConnectTransport(Address, handler, ownsHandler, options, CallHeaders.None);

    // Each 'x' adds one byte to the encoded message in either encoding, except where a protobuf length prefix grows.
    private static Wire.SymbolResponse ResponseOfSize(FakeServiceHandler handler, int size)
    {
        var length = size;
        while (handler.Encode(new Wire.SymbolResponse { Error = new string('x', length) }).Length > size)
        {
            length--;
        }

        var response = new Wire.SymbolResponse { Error = new string('x', length) };
        Assert.Equal(size, handler.Encode(response).Length);
        return response;
    }

    private static Wire.GetSymbolRequest RequestOfSize(FakeServiceHandler handler, int size)
    {
        var length = size;
        while (handler.Encode(new Wire.GetSymbolRequest { SymbolId = new string('a', length) }).Length > size)
        {
            length--;
        }

        var request = new Wire.GetSymbolRequest { SymbolId = new string('a', length) };
        Assert.Equal(size, handler.Encode(request).Length);
        return request;
    }
}
