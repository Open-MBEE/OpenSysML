using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Net;
using System.Net.Http;
using System.Net.Sockets;
using System.Text;
using System.Threading;
using System.Threading.Tasks;
using Google.Protobuf;
using OpenMBEE.OpenSysML.Internal;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// What is particular to the Connect protocol: the request line and headers, gzip answers, the error
/// envelope and the HTTP-status fallback, and answers that are malformed in ways only Connect can be.
/// Each runs with protobuf and with JSON bodies.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed class ConnectTransportTests
{
    private const string GetSymbol = "sysml.SysMLService/GetSymbol";
    private static readonly Uri Address = new("http://fake.invalid:50051/");
    private static readonly ClientWire[] Wires = [ClientWire.ConnectProto, ClientWire.ConnectJson];

    // Refusals whose body is not a Connect error: the status is inferred from the HTTP status, and the failure is the transport's.
    private static readonly Dictionary<string, (Func<HttpResponseMessage> Response, StatusCode Inferred)> NotConnectErrors = new()
    {
        ["router 404"] = (() => ConnectResponses.Raw(HttpStatusCode.NotFound, "text/plain; charset=utf-8", "404 page not found\n"u8.ToArray()), StatusCode.Unimplemented),
        ["html 502"] = (() => ConnectResponses.Raw(HttpStatusCode.BadGateway, "text/html", "<html><body>bad gateway</body></html>"u8.ToArray()), StatusCode.Unavailable),
        ["empty 503"] = (() => ConnectResponses.Raw(HttpStatusCode.ServiceUnavailable, null, []), StatusCode.Unavailable),
        ["text 400"] = (() => ConnectResponses.Raw(HttpStatusCode.BadRequest, "text/plain", "bad request"u8.ToArray()), StatusCode.Internal),
        ["text 401"] = (() => ConnectResponses.Raw(HttpStatusCode.Unauthorized, "text/plain", "who are you"u8.ToArray()), StatusCode.Unauthenticated),
        ["text 403"] = (() => ConnectResponses.Raw(HttpStatusCode.Forbidden, "text/plain", "no"u8.ToArray()), StatusCode.PermissionDenied),
        ["text 429"] = (() => ConnectResponses.Raw(HttpStatusCode.TooManyRequests, "text/plain", "slow down"u8.ToArray()), StatusCode.Unavailable),
        ["text 504"] = (() => ConnectResponses.Raw(HttpStatusCode.GatewayTimeout, "text/plain", "late"u8.ToArray()), StatusCode.Unavailable),
        ["text 418"] = (() => ConnectResponses.Raw((HttpStatusCode)418, "text/plain", "teapot"u8.ToArray()), StatusCode.Unknown),
        ["text 500"] = (() => ConnectResponses.Raw(HttpStatusCode.InternalServerError, "text/plain", "oops"u8.ToArray()), StatusCode.Unknown),
        ["redirect 307"] = (() => ConnectResponses.Raw(HttpStatusCode.TemporaryRedirect, null, []), StatusCode.Unknown),
        ["json not an object"] = (() => ConnectResponses.Error(HttpStatusCode.ServiceUnavailable, "[\"unavailable\"]"), StatusCode.Unavailable),
        ["json not json"] = (() => ConnectResponses.Error(HttpStatusCode.ServiceUnavailable, "upstream connect error"), StatusCode.Unavailable),
        ["json empty object"] = (() => ConnectResponses.Error(HttpStatusCode.ServiceUnavailable, "{}"), StatusCode.Unavailable),
        ["json null code"] = (() => ConnectResponses.Error(HttpStatusCode.BadRequest, """{"code":null,"message":"m"}"""), StatusCode.Internal),
        ["json numeric code"] = (() => ConnectResponses.Error(HttpStatusCode.NotFound, """{"code":5,"message":"m"}"""), StatusCode.Unimplemented),
        ["json unrecognized code"] = (() => ConnectResponses.Error(HttpStatusCode.InternalServerError, """{"code":"from_the_future","message":"m"}"""), StatusCode.Unknown),
        ["json ok code"] = (() => ConnectResponses.Error(HttpStatusCode.InternalServerError, """{"code":"ok"}"""), StatusCode.Unknown),
        ["json grpc-spelled code"] = (() => ConnectResponses.Error(HttpStatusCode.NotFound, """{"code":"NOT_FOUND"}"""), StatusCode.Unimplemented),
        ["json numeric message"] = (() => ConnectResponses.Error(HttpStatusCode.NotFound, """{"code":"not_found","message":5}"""), StatusCode.Unimplemented),
        ["json details not a list"] = (() => ConnectResponses.Error(HttpStatusCode.NotFound, """{"code":"not_found","details":{}}"""), StatusCode.Unimplemented),
        ["json error as text/plain"] = (() => ConnectResponses.Error(HttpStatusCode.NotFound, """{"code":"not_found","message":"m"}""", "text/plain"), StatusCode.Unimplemented),
        ["json error in latin-1"] = (() => ConnectResponses.Error(HttpStatusCode.NotFound, """{"code":"not_found","message":"m"}""", "application/json; charset=iso-8859-1"), StatusCode.Unimplemented),
        ["json error not utf-8"] = (() => ConnectResponses.Raw(HttpStatusCode.NotFound, "application/json", [.. "{\"code\":\"not_found\",\"message\":\""u8, 0xFF, .. "\"}"u8]), StatusCode.Unimplemented),
        ["json empty body"] = (() => ConnectResponses.Raw(HttpStatusCode.ServiceUnavailable, "application/json", []), StatusCode.Unavailable),
        ["json error with corrupt gzip"] = (() => ConnectResponses.Raw(HttpStatusCode.NotFound, "application/json", """{"code":"not_found"}"""u8.ToArray(), "gzip"), StatusCode.Unimplemented),
        ["gzipped html 502"] = (() => ConnectResponses.Raw(HttpStatusCode.BadGateway, "text/html", ConnectResponses.Gzip("<html>bad gateway</html>"u8.ToArray()), "gzip"), StatusCode.Unavailable),
        ["html 502 with corrupt gzip"] = (() => ConnectResponses.Raw(HttpStatusCode.BadGateway, "text/html", "<html>bad gateway</html>"u8.ToArray(), "gzip"), StatusCode.Unavailable),
        ["json error over an unknown coding"] = (() => ConnectResponses.Raw(HttpStatusCode.NotFound, "application/json", """{"code":"not_found"}"""u8.ToArray(), "br"), StatusCode.Unimplemented),
    };

    // Successful HTTP answers that are not a readable response: never a success, never the service's.
    private static readonly Dictionary<string, (Func<ClientWire, HttpResponseMessage> Response, StatusCode Expected)> Malformed = new()
    {
        ["invalid body"] = (wire => ConnectResponses.Raw(HttpStatusCode.OK, MediaType(wire), wire == ClientWire.ConnectJson ? "{\"version\": nope}"u8.ToArray() : [0xFF, 0xFF, 0xFF]), StatusCode.Internal),
        ["shorter than its length"] = (wire => ConnectResponses.Raw(HttpStatusCode.OK, MediaType(wire), Body(wire, Info()), contentLength: Body(wire, Info()).Length + 10), StatusCode.Internal),
        ["the other encoding"] = (wire => ConnectResponses.Raw(HttpStatusCode.OK, wire == ClientWire.ConnectJson ? "application/proto" : "application/json", Body(wire, Info())), StatusCode.Internal),
        ["a streaming content type"] = (wire => ConnectResponses.Raw(HttpStatusCode.OK, "application/connect+proto", Body(wire, Info())), StatusCode.Internal),
        ["html"] = (wire => ConnectResponses.Raw(HttpStatusCode.OK, "text/html", "<html>sign in</html>"u8.ToArray()), StatusCode.Unknown),
        ["no content type"] = (wire => ConnectResponses.Raw(HttpStatusCode.OK, null, Body(wire, Info())), StatusCode.Unknown),
        ["a content type with parameters"] = (wire => ConnectResponses.Raw(HttpStatusCode.OK, MediaType(wire) + "; version=2", Body(wire, Info())), StatusCode.Internal),
        ["an unknown coding"] = (wire => ConnectResponses.Raw(HttpStatusCode.OK, MediaType(wire), Body(wire, Info()), "br"), StatusCode.Internal),
        ["two codings"] = (wire => ConnectResponses.Raw(HttpStatusCode.OK, MediaType(wire), Body(wire, Info()), "gzip, gzip"), StatusCode.Internal),
        ["gzip cut short"] = (wire => Gzipped(wire, gzip => gzip[..(gzip.Length / 2)]), StatusCode.Internal),
        ["gzip without its trailer"] = (wire => Gzipped(wire, gzip => gzip[..^8]), StatusCode.Internal),
        ["gzip with a wrong crc"] = (wire => Gzipped(wire, gzip => [.. gzip[..^8], (byte)(gzip[^8] ^ 1), .. gzip[^7..]]), StatusCode.Internal),
        ["gzip with a wrong length"] = (wire => Gzipped(wire, gzip => [.. gzip[..^1], (byte)(gzip[^1] ^ 1)]), StatusCode.Internal),
        ["gzip with bytes after it"] = (wire => Gzipped(wire, gzip => [.. gzip, .. "trailing"u8]), StatusCode.Internal),
        ["two gzip members"] = (wire => Gzipped(wire, gzip => [.. gzip, .. ConnectResponses.Gzip(" "u8.ToArray())]), StatusCode.Internal),
        ["gzip header only"] = (wire => Gzipped(wire, gzip => gzip[..10]), StatusCode.Internal),
        ["not gzip"] = (wire => ConnectResponses.Raw(HttpStatusCode.OK, MediaType(wire), Body(wire, Info()), "gzip"), StatusCode.Internal),
    };

    public static MatrixTheoryData<ClientWire, string> NotConnectErrorRows => new(Wires, [.. NotConnectErrors.Keys]);

    public static MatrixTheoryData<ClientWire, string> MalformedRows => new(Wires, [.. Malformed.Keys]);

    public static TheoryData<int, StatusCode> HttpStatuses => new()
    {
        { 400, StatusCode.Internal },
        { 401, StatusCode.Unauthenticated },
        { 403, StatusCode.PermissionDenied },
        { 404, StatusCode.Unimplemented },
        { 429, StatusCode.Unavailable },
        { 502, StatusCode.Unavailable },
        { 503, StatusCode.Unavailable },
        { 504, StatusCode.Unavailable },
        { 301, StatusCode.Unknown },
        { 405, StatusCode.Unknown },
        { 409, StatusCode.Unknown },
        { 415, StatusCode.Unknown },
        { 499, StatusCode.Unknown },
        { 500, StatusCode.Unknown },
        { 501, StatusCode.Unknown },
    };

    [Theory]
    [MemberData(nameof(ClientWires.Connect), MemberType = typeof(ClientWires))]
    public async Task ACallIsAnHttp11PostOfTheBareMessageWithTheConnectHeaders(ClientWire wire)
    {
        using var handler = new FakeServiceHandler(wire) { Respond = (_, _) => Task.FromResult(FakeServiceHandler.AnswerOn(wire, new Wire.SymbolResponse())) };
        var headers = new Dictionary<string, string> { ["Authorization"] = "token-for-the-test", ["X-Tenant"] = "t1", ["Content-Language"] = "en" };
        await using var connection = await FakeConnection.OpenAsync(handler, FakeConnection.Options(handler, headers: headers, requestTimeout: TimeSpan.FromSeconds(30)));

        await connection.GetSymbolAsync();

        var sent = handler.RequestsFor("GetSymbol").Single();
        Assert.Equal(HttpMethod.Post, sent.HttpMethod);
        Assert.Equal(HttpVersion.Version11, sent.HttpVersion);
        Assert.Equal("http://fake.invalid:50051/sysml.SysMLService/GetSymbol", sent.Uri.ToString());
        Assert.Equal(MediaType(wire), sent.ContentType);
        Assert.Equal(MediaType(wire), sent.Headers["content-type"]);
        Assert.Equal("1", sent.Headers["connect-protocol-version"]);
        Assert.Equal("gzip", sent.Headers["accept-encoding"]);
        var timeout = long.Parse(sent.Headers["connect-timeout-ms"], System.Globalization.CultureInfo.InvariantCulture);
        Assert.InRange(timeout, 25_000, 30_000);
        Assert.Matches("^[0-9]{1,10}$", sent.Headers["connect-timeout-ms"]);
        Assert.Equal("token-for-the-test", sent.Headers["authorization"]);
        Assert.Equal("t1", sent.Headers["x-tenant"]);
        Assert.Equal("en", sent.Headers["content-language"]);
        Assert.False(sent.Headers.ContainsKey("content-encoding"));
        Assert.DoesNotContain(sent.Headers.Keys, name => name.StartsWith("grpc-", StringComparison.OrdinalIgnoreCase) || name.Equals("te", StringComparison.OrdinalIgnoreCase));
        var request = new Wire.GetSymbolRequest { ModelHash = "m", SymbolId = "A" };
        Assert.Equal(
            wire == ClientWire.ConnectJson ? """{ "modelHash": "m", "symbolId": "A" }"""u8.ToArray() : request.ToByteArray(),
            sent.Body);
    }

    [Fact]
    public async Task TheLongestTimeoutFitsTheTenDigitsConnectAllows()
    {
        using var handler = new FakeServiceHandler(ClientWire.ConnectProto) { Respond = (_, _) => Task.FromResult(FakeServiceHandler.AnswerOn(ClientWire.ConnectProto, new Wire.SymbolResponse())) };
        await using var connection = await FakeConnection.OpenAsync(handler, FakeConnection.Options(handler, requestTimeout: ConnectionOptions.MaxRequestTimeout));

        await connection.GetSymbolAsync();

        var timeout = handler.RequestsFor("GetSymbol").Single().Headers["connect-timeout-ms"];
        Assert.Matches("^[0-9]{1,10}$", timeout);
        Assert.InRange(long.Parse(timeout, System.Globalization.CultureInfo.InvariantCulture), int.MaxValue - 10_000L, int.MaxValue);
    }

    [Theory]
    [MemberData(nameof(ClientWires.Connect), MemberType = typeof(ClientWires))]
    public async Task AGzipAnswerIsInflatedAndRead(ClientWire wire)
    {
        var answer = new Wire.SymbolResponse { Error = string.Concat(Enumerable.Repeat("compressible ", 500)) };
        using var handler = new FakeServiceHandler(wire) { Respond = (_, _) => Task.FromResult(ConnectResponses.Answer(answer, wire.Encoding(), gzip: true)) };
        await using var connection = await FakeConnection.OpenAsync(handler);

        Assert.Equal(answer, await connection.GetSymbolAsync());
    }

    [Theory]
    [MemberData(nameof(ClientWires.Connect), MemberType = typeof(ClientWires))]
    public async Task AnEmptyGzipLabelledBodyIsNotInflated(ClientWire wire)
    {
        using var handler = new FakeServiceHandler(wire) { Respond = (_, _) => Task.FromResult(ConnectResponses.Raw(HttpStatusCode.OK, MediaType(wire), [], "gzip")) };
        await using var connection = await FakeConnection.OpenAsync(handler);

        if (wire == ClientWire.ConnectProto)
        {
            Assert.Equal(new Wire.SymbolResponse(), await connection.GetSymbolAsync());
        }
        else
        {
            // An empty body is a protobuf message with every field at its default, but it is not JSON.
            var error = await Assert.ThrowsAsync<TransportException>(() => connection.GetSymbolAsync());
            Assert.Equal(StatusCode.Internal, error.StatusCode);
        }
    }

    [Theory]
    [MemberData(nameof(ClientWires.Connect), MemberType = typeof(ClientWires))]
    public async Task AGzipAnswerIsBoundedByWhatItInflatesToNotItsCompressedSize(ClientWire wire)
    {
        const int Limit = 64 * 1024;
        using var handler = new FakeServiceHandler(wire);
        await using var connection = await FakeConnection.OpenAsync(handler, FakeConnection.Options(handler, maxResponseSize: Limit));

        var atLimit = ResponseOfSize(handler, Limit);
        handler.Respond = (_, _) => Task.FromResult(ConnectResponses.Answer(atLimit, wire.Encoding(), gzip: true));
        Assert.Equal(atLimit, await connection.GetSymbolAsync());

        var overLimit = ResponseOfSize(handler, Limit + 1);
        var bomb = ConnectResponses.Gzip(handler.Encode(overLimit));
        Assert.True(bomb.Length < Limit / 50, $"the bomb is {bomb.Length} bytes");
        handler.Respond = (_, _) => Task.FromResult(ConnectResponses.Raw(HttpStatusCode.OK, MediaType(wire), bomb, "gzip"));
        var error = await Assert.ThrowsAsync<TransportException>(() => connection.GetSymbolAsync());
        Assert.Equal(StatusCode.ResourceExhausted, error.StatusCode);
        Assert.Contains("MaxResponseSize", error.Message, StringComparison.Ordinal);
    }

    [Theory]
    [MemberData(nameof(MalformedRows))]
    public async Task AMalformedAnswerIsNeverASuccessOrTheServices(ClientWire wire, string shape)
    {
        var (response, expected) = Malformed[shape];
        using var handler = new FakeServiceHandler(wire) { Respond = (_, _) => Task.FromResult(response(wire)) };
        await using var connection = await FakeConnection.OpenAsync(handler);

        var error = await Assert.ThrowsAsync<TransportException>(() => connection.GetSymbolAsync());

        Assert.True(expected == error.StatusCode, $"{shape}: {error.StatusCode}, {error.Message}");
        Assert.Equal(GetSymbol, error.Method);
        Assert.Single(handler.RequestsFor("GetSymbol"));
    }

    [Theory]
    [MemberData(nameof(NotConnectErrorRows))]
    public async Task ARefusalThatIsNotAConnectErrorTakesItsStatusFromHttpAndIsTheTransports(ClientWire wire, string shape)
    {
        var (response, inferred) = NotConnectErrors[shape];
        using var handler = new FakeServiceHandler(wire) { Respond = (_, _) => Task.FromResult(response()) };
        await using var connection = await FakeConnection.OpenAsync(handler);

        var error = await Assert.ThrowsAsync<TransportException>(() => connection.GetSymbolAsync());

        Assert.True(inferred == error.StatusCode, $"{shape}: {error.StatusCode}, {error.Message}");
        Assert.Contains("which is not a Connect error, so the status is inferred from the HTTP status", error.Message, StringComparison.Ordinal);
        Assert.Contains($"HTTP {(int)response().StatusCode}", error.Message, StringComparison.Ordinal);
        Assert.Single(handler.RequestsFor("GetSymbol"));
    }

    [Theory]
    [MemberData(nameof(ClientWires.Connect), MemberType = typeof(ClientWires))]
    public async Task ARefusalQuotesABoundedSanitizedExcerptOfWhatWasAnswered(ClientWire wire)
    {
        var page = "<html>\r\n<title>Bad\tGateway</title>\0" + new string('x', 10_000) + "</html>";
        using var handler = new FakeServiceHandler(wire) { Respond = (_, _) => Task.FromResult(ConnectResponses.Raw(HttpStatusCode.BadGateway, "text/html", Encoding.UTF8.GetBytes(page))) };
        await using var connection = await FakeConnection.OpenAsync(handler);

        var error = await Assert.ThrowsAsync<TransportException>(() => connection.GetSymbolAsync());

        Assert.Contains("\"<html>  <title>Bad Gateway</title> xxx", error.Message, StringComparison.Ordinal);
        Assert.Contains("x…\"", error.Message, StringComparison.Ordinal);
        Assert.DoesNotContain(error.Message, char.IsControl);
        Assert.True(error.Message.Length < ConnectTransport.ExcerptLength + 400, $"{error.Message.Length} characters");
    }

    [Theory]
    [MemberData(nameof(ClientWires.Connect), MemberType = typeof(ClientWires))]
    public async Task ARefusalTooLargeToReadStillTakesItsStatusFromHttp(ClientWire wire)
    {
        const int Limit = 1024;
        using var handler = new FakeServiceHandler(wire);
        await using var connection = await FakeConnection.OpenAsync(handler, FakeConnection.Options(handler, maxResponseSize: Limit));
        var message = new string('m', Limit);
        handler.Respond = (_, _) => Task.FromResult(ConnectResponses.Error(HttpStatusCode.NotFound, $$"""{"code":"not_found","message":"{{message}}"}"""));
        var plain = await Assert.ThrowsAsync<TransportException>(() => connection.GetSymbolAsync());

        handler.Respond = (_, _) => Task.FromResult(ConnectResponses.Raw(
            HttpStatusCode.NotFound, "application/json", ConnectResponses.Gzip(Encoding.UTF8.GetBytes($$"""{"code":"not_found","message":"{{message}}"}""")), "gzip"));
        var gzipped = await Assert.ThrowsAsync<TransportException>(() => connection.GetSymbolAsync());

        Assert.All([plain, gzipped], error =>
        {
            Assert.Equal(StatusCode.Unimplemented, error.StatusCode);
            Assert.Contains("a body that could not be read", error.Message, StringComparison.Ordinal);
            Assert.Contains("MaxResponseSize", error.Message, StringComparison.Ordinal);
        });
    }

    [Theory]
    [MemberData(nameof(ClientWires.Connect), MemberType = typeof(ClientWires))]
    public async Task ACallWhoseDeadlineHasPassedIsNotSent(ClientWire wire)
    {
        using var handler = new FakeServiceHandler(wire);
        await using var transport = new ConnectTransport(Address, handler, ownsHandler: false, new ConnectionOptions { Protocol = wire.Protocol(), Encoding = wire.Encoding() }, CallHeaders.None);
        Assert.True(RpcMethods.TryResolve<Wire.GetSymbolRequest, Wire.SymbolResponse>("GetSymbol", out var method, out _));

        var error = await Assert.ThrowsAsync<TransportException>(() =>
            transport.UnaryAsync(method, new Wire.GetSymbolRequest(), DateTime.UtcNow - TimeSpan.FromSeconds(1), TestContext.Current.CancellationToken));

        Assert.Equal(StatusCode.DeadlineExceeded, error.StatusCode);
        Assert.Empty(handler.Requests);
    }

    [Theory]
    [MemberData(nameof(ClientWires.Connect), MemberType = typeof(ClientWires))]
    public async Task AConnectErrorIsTheServicesWithItsMessageAndAnyDetailsIgnored(ClientWire wire)
    {
        using var handler = new FakeServiceHandler(wire);
        await using var connection = await FakeConnection.OpenAsync(handler);

        handler.Respond = (_, _) => Task.FromResult(ConnectResponses.Error(
            HttpStatusCode.NotFound,
            """{"code":"not_found","message":"model not found: m","details":[{"type":"google.rpc.RetryInfo","value":"CgIIPA","debug":{"retryDelay":"30s"}}]}"""));
        var detailed = await Assert.ThrowsAsync<ServiceException>(() => connection.GetSymbolAsync());

        handler.Respond = (_, _) => Task.FromResult(ConnectResponses.Error(HttpStatusCode.ServiceUnavailable, """{"code":"unavailable"}""", "application/json; charset=utf-8"));
        var bare = await Assert.ThrowsAsync<ServiceException>(() => connection.GetSymbolAsync());

        handler.Respond = (_, _) => Task.FromResult(ConnectResponses.Raw(
            HttpStatusCode.Forbidden, "application/json", ConnectResponses.Gzip("""{"code":"permission_denied","message":"no"}"""u8.ToArray()), "gzip"));
        var gzipped = await Assert.ThrowsAsync<ServiceException>(() => connection.GetSymbolAsync());

        // The code, not the HTTP status, says what the service meant.
        handler.Respond = (_, _) => Task.FromResult(ConnectResponses.Error(HttpStatusCode.InternalServerError, """{"code":"failed_precondition","message":"two documents"}"""));
        var mismatched = await Assert.ThrowsAsync<ServiceException>(() => connection.GetSymbolAsync());

        Assert.Equal((StatusCode.NotFound, "model not found: m"), (detailed.StatusCode, detailed.ServiceMessage));
        Assert.Equal((StatusCode.Unavailable, string.Empty), (bare.StatusCode, bare.ServiceMessage));
        Assert.Equal((StatusCode.PermissionDenied, "no"), (gzipped.StatusCode, gzipped.ServiceMessage));
        Assert.Equal((StatusCode.FailedPrecondition, "two documents"), (mismatched.StatusCode, mismatched.ServiceMessage));
    }

    [Theory]
    [MemberData(nameof(HttpStatuses))]
    public void TheHttpStatusFallbackIsTheProtocols(int http, StatusCode expected) =>
        Assert.Equal(expected, ConnectTransport.FromHttpStatus(http));

    [Fact]
    public void FailuresAreClassifiedByTheirCauses()
    {
        var transport = new ConnectTransport(Address, new FakeServiceHandler(ClientWire.ConnectProto), ownsHandler: true, new ConnectionOptions { Protocol = Protocol.Connect }, CallHeaders.None);
        var none = CancellationToken.None;
        StatusCode Status(Exception error, bool answered = false) =>
            Assert.IsType<TransportException>(transport.Map(error, GetSymbol, answered, none, none)).StatusCode;

        Assert.Equal(StatusCode.Unavailable, Status(new HttpRequestException(HttpRequestError.NameResolutionError, "dns", new SocketException((int)SocketError.HostNotFound))));
        Assert.Equal(StatusCode.Unavailable, Status(new HttpRequestException(HttpRequestError.ConnectionError, "refused")));
        Assert.Equal(StatusCode.Unavailable, Status(new HttpRequestException(HttpRequestError.SecureConnectionError, "tls")));
        Assert.Equal(StatusCode.Unavailable, Status(new HttpRequestException(HttpRequestError.ProxyTunnelError, "proxy")));
        Assert.Equal(StatusCode.Unavailable, Status(new HttpRequestException(HttpRequestError.Unknown, "reset", new IOException("io", new SocketException((int)SocketError.ConnectionReset)))));
        Assert.Equal(StatusCode.Unavailable, Status(new HttpRequestException(HttpRequestError.ResponseEnded, "closed"), answered: false));
        Assert.Equal(StatusCode.Internal, Status(new HttpIOException(HttpRequestError.ResponseEnded, "cut short"), answered: true));
        Assert.Equal(StatusCode.Internal, Status(new HttpRequestException(HttpRequestError.InvalidResponse, "garbage")));
        Assert.Equal(StatusCode.Internal, Status(new HttpRequestException(HttpRequestError.HttpProtocolError, "protocol")));
        Assert.Equal(StatusCode.Internal, Status(new HttpRequestException(HttpRequestError.ConfigurationLimitExceeded, "headers too long")));
        Assert.Equal(StatusCode.Unavailable, Status(new IOException("io", new SocketException((int)SocketError.ConnectionAborted)), answered: true));
        Assert.Equal(StatusCode.Internal, Status(Undecodable()));
        Assert.Equal(StatusCode.Internal, Status(Unparsable()));
        Assert.Equal(StatusCode.Internal, Status(new InvalidDataException("bad gzip")));
        Assert.Equal(StatusCode.Internal, Status(new DecoderFallbackException("not utf-8")));
        Assert.Equal(StatusCode.Unknown, Status(new InvalidOperationException("handler bug")));
        Assert.Contains("Whether the service received the call is unknown", transport.Map(new HttpRequestException(HttpRequestError.ConnectionError, "refused"), GetSymbol, false, none, none).Message, StringComparison.Ordinal);
    }

    [Fact]
    public async Task ACancelledOrLateCallIsSoUnlessItsAnswerDidNotDecode()
    {
        var transport = new ConnectTransport(Address, new FakeServiceHandler(ClientWire.ConnectProto), ownsHandler: true, new ConnectionOptions { Protocol = Protocol.Connect }, CallHeaders.None);
        using var cancelled = new CancellationTokenSource();
        await cancelled.CancelAsync();
        var aborted = new HttpRequestException(HttpRequestError.Unknown, "aborted", new IOException("io", new SocketException((int)SocketError.OperationAborted)));

        var cancellation = Assert.IsType<OperationCanceledException>(transport.Map(new TaskCanceledException(), GetSymbol, true, cancelled.Token, CancellationToken.None));
        var abortedByCancellation = Assert.IsType<OperationCanceledException>(transport.Map(aborted, GetSymbol, true, cancelled.Token, CancellationToken.None));
        var late = Assert.IsType<TransportException>(transport.Map(aborted, GetSymbol, true, CancellationToken.None, cancelled.Token));
        var undecodable = Assert.IsType<TransportException>(transport.Map(Undecodable(), GetSymbol, true, cancelled.Token, cancelled.Token));

        Assert.Equal(cancelled.Token, cancellation.CancellationToken);
        Assert.Equal(cancelled.Token, abortedByCancellation.CancellationToken);
        Assert.Equal(StatusCode.DeadlineExceeded, late.StatusCode);
        Assert.Contains("did not complete before its deadline", late.Message, StringComparison.Ordinal);
        Assert.Equal(StatusCode.Internal, undecodable.StatusCode);
        await transport.DisposeAsync();
    }

    private static InvalidProtocolBufferException Undecodable() =>
        Assert.Throws<InvalidProtocolBufferException>(() => Wire.SymbolResponse.Parser.ParseFrom([0xFF, 0xFF, 0xFF]));

    private static InvalidJsonException Unparsable() =>
        Assert.Throws<InvalidJsonException>(() => JsonParser.Default.Parse<Wire.SymbolResponse>("{"));

    private static string MediaType(ClientWire wire) => wire == ClientWire.ConnectJson ? "application/json" : "application/proto";

    private static Wire.ServerInfoResponse Info() => new() { Version = "v1", Capabilities = { string.Concat(Enumerable.Repeat("capability ", 100)) } };

    private static byte[] Body(ClientWire wire, IMessage message) =>
        wire == ClientWire.ConnectJson ? Encoding.UTF8.GetBytes(JsonFormatter.Default.Format(message)) : message.ToByteArray();

    private static HttpResponseMessage Gzipped(ClientWire wire, Func<byte[], byte[]> change) =>
        ConnectResponses.Raw(HttpStatusCode.OK, MediaType(wire), change(ConnectResponses.Gzip(Body(wire, new Wire.SymbolResponse { Error = string.Concat(Enumerable.Repeat("long enough to be cut ", 400)) }))), "gzip");

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
}
