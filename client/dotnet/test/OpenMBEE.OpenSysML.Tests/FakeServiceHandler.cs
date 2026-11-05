using System;
using System.Buffers.Binary;
using System.Collections.Concurrent;
using System.Collections.Generic;
using System.Globalization;
using System.IO;
using System.IO.Compression;
using System.Linq;
using System.Net;
using System.Net.Http;
using System.Net.Http.Headers;
using System.Text;
using System.Text.Json;
using System.Threading;
using System.Threading.Tasks;
using Google.Protobuf;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// An HTTP handler standing in for the service on one <see cref="ClientWire"/>: it records every
/// request a connection sends and answers GetServerInfo itself, and every other method through
/// <see cref="Respond"/>. Responses are built by hand, so a test controls every byte, header and
/// trailer a client reads.
/// </summary>
public sealed class FakeServiceHandler : HttpMessageHandler
{
    /// <summary>The address a connection over this handler is opened at.</summary>
    internal const string Address = "http://fake.invalid:50051";

    private readonly ConcurrentQueue<SentRequest> requests = new();
    private int disposals;

    /// <summary>Creates a fake speaking a wire.</summary>
    /// <param name="wire">The wire it answers on.</param>
    internal FakeServiceHandler(ClientWire wire = ClientWire.Grpc)
    {
        Wire = wire;
        Respond = (_, _) => Task.FromResult(Refuse(StatusCode.Unimplemented, "not faked"));
    }

    /// <summary>The wire it answers on.</summary>
    internal ClientWire Wire { get; }

    /// <summary>The version GetServerInfo reports.</summary>
    internal string Version { get; init; } = "v9.9.9";

    /// <summary>The capabilities GetServerInfo reports.</summary>
    internal IReadOnlyList<string> Capabilities { get; init; } = [OpenSysML.Capabilities.Query, OpenSysML.Capabilities.Convert];

    /// <summary>Answers GetServerInfo instead of the default answer.</summary>
    internal Func<SentRequest, CancellationToken, Task<HttpResponseMessage>>? ServerInfo { get; init; }

    /// <summary>Answers every method but GetServerInfo.</summary>
    internal Func<SentRequest, CancellationToken, Task<HttpResponseMessage>> Respond { get; set; }

    /// <summary>Every request sent, in order.</summary>
    internal IReadOnlyList<SentRequest> Requests => [.. requests];

    /// <summary>How many times the handler was disposed.</summary>
    internal int Disposals => Volatile.Read(ref disposals);

    /// <summary>The requests sent for a method, by bare name.</summary>
    internal IReadOnlyList<SentRequest> RequestsFor(string method) => [.. requests.Where(request => request.Method == method)];

    /// <summary>A successful answer carrying a message, as a wire encodes it.</summary>
    internal static HttpResponseMessage AnswerOn(ClientWire wire, IMessage message) =>
        wire == ClientWire.Grpc ? GrpcResponses.Message(message) : ConnectResponses.Answer(message, wire.Encoding());

    /// <summary>A refusal the service sends, as a wire carries it.</summary>
    internal static HttpResponseMessage RefuseOn(ClientWire wire, StatusCode status, string message) =>
        wire == ClientWire.Grpc ? GrpcResponses.TrailersOnly((int)status, message) : ConnectResponses.Error(status, message);

    /// <summary>A successful answer carrying a message, as this fake's wire encodes it.</summary>
    internal HttpResponseMessage Answer(IMessage message) => AnswerOn(Wire, message);

    /// <summary>A refusal the service sends, as this fake's wire carries it.</summary>
    internal HttpResponseMessage Refuse(StatusCode status, string message) => RefuseOn(Wire, status, message);

    /// <summary>The bytes a size limit measures a message as on this wire: the message in its encoding, unframed.</summary>
    internal byte[] Encode(IMessage message) =>
        Wire == ClientWire.ConnectJson ? Encoding.UTF8.GetBytes(JsonFormatter.Default.Format(message)) : message.ToByteArray();

    protected override async Task<HttpResponseMessage> SendAsync(HttpRequestMessage request, CancellationToken cancellationToken)
    {
        var body = request.Content is null ? [] : await request.Content.ReadAsByteArrayAsync(cancellationToken);
        var headers = request.Headers.Concat(request.Content?.Headers ?? Enumerable.Empty<KeyValuePair<string, IEnumerable<string>>>())
            .ToDictionary(header => header.Key, header => string.Join(",", header.Value), StringComparer.OrdinalIgnoreCase);
        var sent = new SentRequest(
            Wire,
            request.RequestUri!.AbsolutePath.Split('/')[^1],
            request.RequestUri,
            request.Method,
            request.Version,
            headers,
            request.Content?.Headers.ContentType?.MediaType,
            body);
        requests.Enqueue(sent);
        if (sent.Method == "GetServerInfo")
        {
            if (ServerInfo is not null)
            {
                return await ServerInfo(sent, cancellationToken);
            }

            var info = new Wire.ServerInfoResponse { Version = Version };
            info.Capabilities.AddRange(Capabilities);
            return Answer(info);
        }

        return await Respond(sent, cancellationToken);
    }

    protected override void Dispose(bool disposing)
    {
        Interlocked.Increment(ref disposals);
        base.Dispose(disposing);
    }
}

/// <summary>One request a <see cref="FakeServiceHandler"/> received.</summary>
/// <param name="Wire">The wire it was sent on.</param>
/// <param name="Method">The bare method name.</param>
/// <param name="Uri">The request's URL.</param>
/// <param name="HttpMethod">Its HTTP method.</param>
/// <param name="HttpVersion">Its HTTP version.</param>
/// <param name="Headers">Its headers and content headers, by name.</param>
/// <param name="ContentType">Its media type.</param>
/// <param name="Body">Its body, as sent.</param>
internal sealed record SentRequest(
    ClientWire Wire,
    string Method,
    Uri Uri,
    HttpMethod HttpMethod,
    Version HttpVersion,
    IReadOnlyDictionary<string, string> Headers,
    string? ContentType,
    byte[] Body)
{
    /// <summary>The message the body carries: framed for gRPC, the whole body for Connect.</summary>
    internal byte[] MessageBytes => Wire == ClientWire.Grpc ? Body.AsSpan(5).ToArray() : Body;

    /// <summary>The message the body carries, decoded as the wire encodes it.</summary>
    internal T Message<T>(MessageParser<T> parser)
        where T : IMessage<T>, new() =>
        Wire == ClientWire.ConnectJson ? JsonParser.Default.Parse<T>(Encoding.UTF8.GetString(Body)) : parser.ParseFrom(MessageBytes);
}

/// <summary>gRPC responses framed by hand.</summary>
internal static class GrpcResponses
{
    /// <summary>A response carrying one message and the status in its trailers.</summary>
    internal static HttpResponseMessage Message(IMessage message, int status = 0) => Framed(message.ToByteArray(), status);

    /// <summary>A response carrying one frame of these bytes, with the status in its trailers.</summary>
    internal static HttpResponseMessage Framed(byte[] payload, int? status = 0, bool compressed = false) =>
        Raw(Frame(payload, compressed), status);

    /// <summary>A response with this body, as is, and the status in its trailers when there is one.</summary>
    internal static HttpResponseMessage Raw(byte[] body, int? status = 0, string contentType = "application/grpc", HttpStatusCode code = HttpStatusCode.OK)
    {
        var response = new HttpResponseMessage(code)
        {
            Version = HttpVersion.Version20,
            Content = new ByteArrayContent(body),
        };
        response.Content.Headers.ContentType = MediaTypeHeaderValue.Parse(contentType);
        if (status is { } value)
        {
            response.TrailingHeaders.Add("grpc-status", value.ToString(CultureInfo.InvariantCulture));
        }

        return response;
    }

    /// <summary>A trailers-only response: the status, and no message, in its headers.</summary>
    internal static HttpResponseMessage TrailersOnly(int status, string message) => TrailersOnly(status.ToString(CultureInfo.InvariantCulture), message);

    /// <summary>A trailers-only response with a grpc-status header of this text.</summary>
    internal static HttpResponseMessage TrailersOnly(string status, string message)
    {
        var response = new HttpResponseMessage(HttpStatusCode.OK)
        {
            Version = HttpVersion.Version20,
            Content = new ByteArrayContent([]),
        };
        response.Content.Headers.ContentType = MediaTypeHeaderValue.Parse("application/grpc");
        response.Headers.Add("grpc-status", status);
        response.Headers.Add("grpc-message", Uri.EscapeDataString(message));
        return response;
    }

    /// <summary>One length-prefixed gRPC frame.</summary>
    internal static byte[] Frame(byte[] payload, bool compressed = false)
    {
        var frame = new byte[5 + payload.Length];
        frame[0] = compressed ? (byte)1 : (byte)0;
        BinaryPrimitives.WriteUInt32BigEndian(frame.AsSpan(1), (uint)payload.Length);
        payload.CopyTo(frame, 5);
        return frame;
    }
}

/// <summary>Connect unary responses built by hand.</summary>
internal static class ConnectResponses
{
    /// <summary>
    /// Each status's Connect code and the HTTP status the protocol pairs it with, written out from the
    /// specification rather than read from the client, so that the two can disagree.
    /// </summary>
    internal static readonly IReadOnlyDictionary<StatusCode, (string Code, HttpStatusCode Http)> Codes = new Dictionary<StatusCode, (string, HttpStatusCode)>
    {
        [StatusCode.Cancelled] = ("canceled", (HttpStatusCode)499),
        [StatusCode.Unknown] = ("unknown", HttpStatusCode.InternalServerError),
        [StatusCode.InvalidArgument] = ("invalid_argument", HttpStatusCode.BadRequest),
        [StatusCode.DeadlineExceeded] = ("deadline_exceeded", HttpStatusCode.GatewayTimeout),
        [StatusCode.NotFound] = ("not_found", HttpStatusCode.NotFound),
        [StatusCode.AlreadyExists] = ("already_exists", HttpStatusCode.Conflict),
        [StatusCode.PermissionDenied] = ("permission_denied", HttpStatusCode.Forbidden),
        [StatusCode.ResourceExhausted] = ("resource_exhausted", HttpStatusCode.TooManyRequests),
        [StatusCode.FailedPrecondition] = ("failed_precondition", HttpStatusCode.BadRequest),
        [StatusCode.Aborted] = ("aborted", HttpStatusCode.Conflict),
        [StatusCode.OutOfRange] = ("out_of_range", HttpStatusCode.BadRequest),
        [StatusCode.Unimplemented] = ("unimplemented", HttpStatusCode.NotImplemented),
        [StatusCode.Internal] = ("internal", HttpStatusCode.InternalServerError),
        [StatusCode.Unavailable] = ("unavailable", HttpStatusCode.ServiceUnavailable),
        [StatusCode.DataLoss] = ("data_loss", HttpStatusCode.InternalServerError),
        [StatusCode.Unauthenticated] = ("unauthenticated", HttpStatusCode.Unauthorized),
    };

    /// <summary>A successful answer: the message, encoded, optionally gzipped.</summary>
    internal static HttpResponseMessage Answer(IMessage message, MessageEncoding encoding, bool gzip = false)
    {
        var body = encoding == MessageEncoding.Json ? Encoding.UTF8.GetBytes(JsonFormatter.Default.Format(message)) : message.ToByteArray();
        return Raw(
            HttpStatusCode.OK,
            encoding == MessageEncoding.Json ? "application/json" : "application/proto",
            gzip ? Gzip(body) : body,
            gzip ? "gzip" : null);
    }

    /// <summary>A Connect error, at the HTTP status its code is paired with.</summary>
    internal static HttpResponseMessage Error(StatusCode status, string message) =>
        Error(Codes[status].Http, $$"""{"code":"{{Codes[status].Code}}","message":{{JsonSerializer.Serialize(message)}}}""");

    /// <summary>A JSON error body at an HTTP status.</summary>
    internal static HttpResponseMessage Error(HttpStatusCode http, string json, string contentType = "application/json") =>
        Raw(http, contentType, Encoding.UTF8.GetBytes(json));

    /// <summary>A response with this body, as is.</summary>
    internal static HttpResponseMessage Raw(HttpStatusCode code, string? contentType, byte[] body, string? contentEncoding = null, long? contentLength = null)
    {
        var response = new HttpResponseMessage(code)
        {
            Version = HttpVersion.Version11,
            Content = new ByteArrayContent(body),
        };
        if (contentType is not null)
        {
            response.Content.Headers.ContentType = MediaTypeHeaderValue.Parse(contentType);
        }

        if (contentEncoding is not null)
        {
            response.Content.Headers.TryAddWithoutValidation("Content-Encoding", contentEncoding);
        }

        if (contentLength is not null)
        {
            response.Content.Headers.ContentLength = contentLength;
        }

        return response;
    }

    /// <summary>Bytes gzipped as one member.</summary>
    internal static byte[] Gzip(byte[] data, CompressionLevel level = CompressionLevel.Optimal)
    {
        using var compressed = new MemoryStream();
        using (var gzip = new GZipStream(compressed, level, leaveOpen: true))
        {
            gzip.Write(data);
        }

        return compressed.ToArray();
    }
}

/// <summary>Connections over a <see cref="FakeServiceHandler"/>.</summary>
internal static class FakeConnection
{
    /// <summary>Options addressing the fake on its wire, through it, with any of these overridden.</summary>
    internal static ConnectionOptions Options(
        FakeServiceHandler handler,
        TimeSpan? requestTimeout = null,
        IReadOnlyDictionary<string, string>? headers = null,
        int? maxRequestSize = null,
        int? maxResponseSize = null,
        Microsoft.Extensions.Logging.ILogger? logger = null,
        string? version = null,
        IReadOnlyList<string>? requireCapabilities = null) => new()
        {
            Address = FakeServiceHandler.Address,
            Protocol = handler.Wire.Protocol(),
            Encoding = handler.Wire.Encoding(),
            HttpMessageHandler = handler,
            RequestTimeout = requestTimeout ?? ConnectionOptions.DefaultRequestTimeout,
            Headers = headers ?? new Dictionary<string, string>(),
            MaxRequestSize = maxRequestSize ?? ConnectionOptions.DefaultMaxMessageSize,
            MaxResponseSize = maxResponseSize ?? ConnectionOptions.DefaultMaxMessageSize,
            Logger = logger ?? Microsoft.Extensions.Logging.Abstractions.NullLogger.Instance,
            Version = version,
            RequireCapabilities = requireCapabilities ?? [],
        };

    /// <summary>Opens a connection over the fake.</summary>
    internal static Task<Connection> OpenAsync(FakeServiceHandler handler, ConnectionOptions? options = null) =>
        Connection.OpenAsync(options ?? Options(handler), Xunit.TestContext.Current.CancellationToken);

    /// <summary>A raw GetSymbol call, the method the tests make when the method does not matter.</summary>
    internal static Task<Wire.SymbolResponse> CallGetSymbolAsync(this Connection connection, CancellationToken cancellationToken) =>
        connection.CallAsync<Wire.GetSymbolRequest, Wire.SymbolResponse>(
            "GetSymbol",
            new Wire.GetSymbolRequest { ModelHash = "m", SymbolId = "A" },
            cancellationToken);

    /// <summary>A raw GetSymbol call under the test's token.</summary>
    internal static Task<Wire.SymbolResponse> GetSymbolAsync(this Connection connection) =>
        connection.CallGetSymbolAsync(Xunit.TestContext.Current.CancellationToken);
}
