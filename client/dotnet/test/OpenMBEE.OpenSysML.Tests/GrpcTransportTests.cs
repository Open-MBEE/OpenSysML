using System;
using System.Collections.Generic;
using System.Linq;
using System.Net;
using System.Net.Http;
using System.Net.Sockets;
using System.Threading;
using System.Threading.Tasks;
using Grpc.Core;
using OpenMBEE.OpenSysML.Internal;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

[Trait(Category.Name, Category.Unit)]
public sealed class GrpcTransportTests
{
    private const string GetSymbol = "sysml.SysMLService/GetSymbol";
    private static readonly Uri Address = new("http://fake.invalid:50051/");

    public static TheoryData<string, Func<HttpResponseMessage>, StatusCode> MalformedResponses => new()
    {
        { "truncated frame", () => GrpcResponses.Raw([0, 0, 0, 0, 10, 1, 2], status: 0), StatusCode.Internal },
        { "truncated prefix", () => GrpcResponses.Raw([0, 0, 0], status: 0), StatusCode.Internal },
        { "invalid protobuf", () => GrpcResponses.Framed([0xFF, 0xFF, 0xFF], status: 0), StatusCode.Internal },
        { "no message", () => GrpcResponses.Raw([], status: 0), StatusCode.Internal },
        { "two messages", () => GrpcResponses.Raw([.. GrpcResponses.Frame([]), .. GrpcResponses.Frame([])], status: 0), StatusCode.Internal },
        { "no grpc-status", () => GrpcResponses.Framed([], status: null), StatusCode.Cancelled },
        { "non-numeric grpc-status", () => GrpcResponses.TrailersOnly("five", "x"), StatusCode.Internal },
        { "compressed without an encoding", () => GrpcResponses.Framed([1], status: 0, compressed: true), StatusCode.Internal },
        { "html content", () => GrpcResponses.Raw("<html>bad gateway</html>"u8.ToArray(), status: null, contentType: "text/html", code: HttpStatusCode.BadGateway), StatusCode.Unavailable },
        { "wrong content type", () => GrpcResponses.Raw(GrpcResponses.Frame([]), status: 0, contentType: "application/json"), StatusCode.Cancelled },
        { "http 404", () => GrpcResponses.Raw([], status: null, contentType: "text/plain", code: HttpStatusCode.NotFound), StatusCode.Unimplemented },
        { "http 503", () => GrpcResponses.Raw([], status: null, contentType: "text/plain", code: HttpStatusCode.ServiceUnavailable), StatusCode.Unavailable },
    };

    [Fact]
    public async Task AStatusInTheTrailersAfterAMessageIsTheServices()
    {
        using var handler = new FakeServiceHandler();
        await using var connection = await FakeConnection.OpenAsync(handler);
        handler.Respond = (_, _) => Task.FromResult(GrpcResponses.Framed([], status: 5));

        var error = await Assert.ThrowsAsync<ServiceException>(() => connection.GetSymbolAsync());

        Assert.Equal(StatusCode.NotFound, error.StatusCode);
    }

    [Fact]
    public async Task AStatusThisClientDoesNotKnowIsUnknown()
    {
        using var handler = new FakeServiceHandler();
        await using var connection = await FakeConnection.OpenAsync(handler);
        handler.Respond = (_, _) => Task.FromResult(GrpcResponses.TrailersOnly(99, "from the future"));

        var error = await Assert.ThrowsAsync<ServiceException>(() => connection.GetSymbolAsync());

        Assert.Equal(StatusCode.Unknown, error.StatusCode);
        Assert.Equal("from the future", error.ServiceMessage);
    }

    [Theory]
    [MemberData(nameof(MalformedResponses))]
    public async Task AMalformedResponseIsNeverASuccessOrTheServices(string shape, Func<HttpResponseMessage> response, StatusCode expected)
    {
        using var handler = new FakeServiceHandler();
        await using var connection = await FakeConnection.OpenAsync(handler);
        handler.Respond = (_, _) => Task.FromResult(response());

        var error = await Assert.ThrowsAsync<TransportException>(() => connection.GetSymbolAsync());

        Assert.True(expected == error.StatusCode, $"{shape}: {error.StatusCode}, {error.Message}");
        Assert.Equal(GetSymbol, error.Method);
        Assert.NotNull(error.InnerException);
    }

    [Fact]
    public async Task ACallIsAUnaryGrpcPostWithTheHeadersTheDeadlineAndIdentityEncodingOnly()
    {
        using var handler = new FakeServiceHandler { Respond = (_, _) => Task.FromResult(GrpcResponses.Message(new Wire.SymbolResponse())) };
        var headers = new Dictionary<string, string> { ["Authorization"] = "Bearer abc", ["X-Tenant"] = "t1" };
        await using var connection = await FakeConnection.OpenAsync(handler, FakeConnection.Options(handler, headers: headers, requestTimeout: TimeSpan.FromSeconds(30)));

        await connection.GetSymbolAsync();

        var sent = handler.RequestsFor("GetSymbol").Single();
        Assert.Equal("/sysml.SysMLService/GetSymbol", sent.Uri.AbsolutePath);
        Assert.Equal("application/grpc", sent.ContentType);
        Assert.Equal("Bearer abc", sent.Headers["authorization"]);
        Assert.Equal("t1", sent.Headers["x-tenant"]);
        Assert.Equal(["identity"], sent.Headers["grpc-accept-encoding"].Split(',', StringSplitOptions.RemoveEmptyEntries | StringSplitOptions.TrimEntries));
        Assert.False(sent.Headers.ContainsKey("grpc-encoding"));
        Assert.Matches("^[0-9]+[HMSmun]$", sent.Headers["grpc-timeout"]);
        Assert.Equal(new Wire.GetSymbolRequest { ModelHash = "m", SymbolId = "A" }, sent.Message(Wire.GetSymbolRequest.Parser));
        Assert.All(handler.Requests, request => Assert.Equal("Bearer abc", request.Headers["authorization"]));
    }

    [Fact]
    public void AnInternalFailureCausedByAnHttp2ProtocolErrorNamesAnHttp11Endpoint()
    {
        var cause = new HttpRequestException(
            HttpRequestError.HttpProtocolError,
            "The HTTP/2 server sent invalid data on the connection.",
            new HttpProtocolException(1, "PROTOCOL_ERROR", null));
        var error = new RpcException(new Status(Grpc.Core.StatusCode.Internal, "Error starting gRPC call.", cause));

        var mapped = Assert.IsType<TransportException>(GrpcTransport.Map(error, serverStatus: null, GetSymbol, Address, CredentialSource.None, CancellationToken.None));

        Assert.Equal(StatusCode.Internal, mapped.StatusCode);
        Assert.Same(cause, mapped.InnerException);
        Assert.Contains("did not answer as an HTTP/2 gRPC endpoint (HTTP/2 protocol error 1)", mapped.Message, StringComparison.Ordinal);
        Assert.Contains("HTTP/1.1", mapped.Message, StringComparison.Ordinal);
        Assert.Contains("ConnectionOptions.Protocol set to Protocol.Connect", mapped.Message, StringComparison.Ordinal);
    }

    [Theory]
    [InlineData(Grpc.Core.StatusCode.Internal)]
    [InlineData(Grpc.Core.StatusCode.Unavailable)]
    public void AnHttp2NegotiationFailureNamesAnHttp11EndpointWhateverStatusGrpcAssigned(Grpc.Core.StatusCode status)
    {
        var cause = new HttpRequestException(
            HttpRequestError.VersionNegotiationError,
            "An HTTP/2 connection could not be established because the server did not complete the HTTP/2 handshake.");
        var error = new RpcException(new Status(status, "Error starting gRPC call.", cause));

        var mapped = Assert.IsType<TransportException>(GrpcTransport.Map(error, serverStatus: null, GetSymbol, Address, CredentialSource.None, CancellationToken.None));

        Assert.Equal(StatusCode.Internal, mapped.StatusCode);
        Assert.Equal(GetSymbol, mapped.Method);
        Assert.Same(cause, mapped.InnerException);
        Assert.Contains("did not answer as an HTTP/2 gRPC endpoint (HTTP/2 negotiation failed)", mapped.Message, StringComparison.Ordinal);
        Assert.Contains("HTTP/1.1", mapped.Message, StringComparison.Ordinal);
        Assert.Contains("ConnectionOptions.Protocol set to Protocol.Connect", mapped.Message, StringComparison.Ordinal);
    }

    [Fact]
    public void AnUnavailableConnectionResetIsNotBlamedOnHttp11()
    {
        var cause = new HttpRequestException(HttpRequestError.Unknown, "reset", new SocketException((int)SocketError.ConnectionReset));
        var error = new RpcException(new Status(Grpc.Core.StatusCode.Unavailable, "Error starting gRPC call.", cause));

        var mapped = Assert.IsType<TransportException>(GrpcTransport.Map(error, serverStatus: null, GetSymbol, Address, CredentialSource.None, CancellationToken.None));

        Assert.Equal(StatusCode.Unavailable, mapped.StatusCode);
        Assert.DoesNotContain("HTTP/1.1", mapped.Message, StringComparison.Ordinal);
        Assert.DoesNotContain("Protocol.Connect", mapped.Message, StringComparison.Ordinal);
    }

    [Fact]
    public void AStatusWithALocalCauseIsNeverTheServicesEvenWhenTheResponseCarriedIt()
    {
        var error = new RpcException(new Status(Grpc.Core.StatusCode.Internal, "decode", new InvalidOperationException("local")));

        Assert.IsType<TransportException>(GrpcTransport.Map(error, serverStatus: 13, GetSymbol, Address, CredentialSource.None, CancellationToken.None));
        Assert.IsType<ServiceException>(GrpcTransport.Map(new RpcException(new Status(Grpc.Core.StatusCode.Internal, "x")), 13, GetSymbol, Address, CredentialSource.None, CancellationToken.None));
        Assert.IsType<TransportException>(GrpcTransport.Map(new RpcException(new Status(Grpc.Core.StatusCode.Internal, "x")), 0, GetSymbol, Address, CredentialSource.None, CancellationToken.None));
    }
}
