using System;
using System.Collections.Frozen;
using System.Collections.Generic;
using System.Globalization;
using System.IO;
using System.Linq;
using System.Net;
using System.Net.Http;
using System.Net.Http.Headers;
using System.Net.Sockets;
using System.Text;
using System.Text.Json;
using System.Text.Unicode;
using System.Threading;
using System.Threading.Tasks;
using Google.Protobuf;

namespace OpenMBEE.OpenSysML.Internal;

/// <summary>
/// Unary calls over the Connect protocol: one HTTP/1.1 POST to <c>/sysml.SysMLService/Method</c> per
/// call, its body the request in protobuf or JSON. The transport sends its deadline as
/// <c>Connect-Timeout-Ms</c> and also enforces it itself, since the service does not cut a running
/// call short; it accepts a gzip answer and bounds both what it reads and what that inflates to; it
/// follows no redirect and never retries. A refusal is the service's only when its body is a Connect
/// error; otherwise the status is inferred from the HTTP status, as the protocol specifies, and the
/// failure is a transport failure carrying an excerpt of what was answered.
/// </summary>
internal sealed class ConnectTransport : ITransport
{
    /// <summary>The Connect protocol version this transport speaks.</summary>
    internal const string ProtocolVersion = "1";

    /// <summary>The media type of a protobuf body.</summary>
    internal const string ProtobufMediaType = "application/proto";

    /// <summary>The longest timeout <c>Connect-Timeout-Ms</c> can carry: ten digits.</summary>
    internal const long MaxTimeoutMilliseconds = 9_999_999_999;

    /// <summary>How many characters of an answer that is not a Connect error a failure quotes.</summary>
    internal const int ExcerptLength = 200;

    private static readonly FrozenDictionary<string, StatusCode> Codes = new Dictionary<string, StatusCode>(StringComparer.Ordinal)
    {
        ["canceled"] = StatusCode.Cancelled,
        ["unknown"] = StatusCode.Unknown,
        ["invalid_argument"] = StatusCode.InvalidArgument,
        ["deadline_exceeded"] = StatusCode.DeadlineExceeded,
        ["not_found"] = StatusCode.NotFound,
        ["already_exists"] = StatusCode.AlreadyExists,
        ["permission_denied"] = StatusCode.PermissionDenied,
        ["resource_exhausted"] = StatusCode.ResourceExhausted,
        ["failed_precondition"] = StatusCode.FailedPrecondition,
        ["aborted"] = StatusCode.Aborted,
        ["out_of_range"] = StatusCode.OutOfRange,
        ["unimplemented"] = StatusCode.Unimplemented,
        ["internal"] = StatusCode.Internal,
        ["unavailable"] = StatusCode.Unavailable,
        ["data_loss"] = StatusCode.DataLoss,
        ["unauthenticated"] = StatusCode.Unauthenticated,
    }.ToFrozenDictionary(StringComparer.Ordinal);

    private readonly HttpMessageInvoker invoker;
    private readonly CallHeaders headers;
    private readonly string mediaType;
    private readonly int maxRequestSize;
    private readonly int maxResponseSize;
    private int disposed;

    /// <summary>Creates the transport for a connection.</summary>
    /// <param name="address">The service's base URL.</param>
    /// <param name="options">The connection's options, already validated.</param>
    /// <param name="headers">The headers every call carries.</param>
    internal ConnectTransport(Uri address, ConnectionOptions options, CallHeaders headers)
        : this(address, options.HttpMessageHandler ?? CreateHandler(), ownsHandler: options.HttpMessageHandler is null, options, headers)
    {
    }

    /// <summary>Creates the transport over a handler.</summary>
    /// <param name="address">The service's base URL.</param>
    /// <param name="handler">The handler to send through.</param>
    /// <param name="ownsHandler">Whether disposing the transport disposes the handler; a borrowed one is left alone.</param>
    /// <param name="options">The connection's options, already validated.</param>
    /// <param name="headers">The headers every call carries.</param>
    internal ConnectTransport(Uri address, HttpMessageHandler handler, bool ownsHandler, ConnectionOptions options, CallHeaders headers)
    {
        Address = address;
        Encoding = options.Encoding;
        mediaType = Encoding == MessageEncoding.Json ? JsonBodies.MediaType : ProtobufMediaType;
        this.headers = headers;
        maxRequestSize = options.MaxRequestSize;
        maxResponseSize = options.MaxResponseSize;
        invoker = new HttpMessageInvoker(handler, disposeHandler: ownsHandler);
    }

    /// <inheritdoc/>
    public Protocol Protocol => Protocol.Connect;

    /// <inheritdoc/>
    public MessageEncoding Encoding { get; }

    /// <summary>The service's base URL.</summary>
    internal Uri Address { get; }

    /// <summary>The status the Connect protocol infers from an HTTP status when the body is not a Connect error.</summary>
    /// <param name="httpStatus">The HTTP status.</param>
    /// <returns>The inferred status.</returns>
    internal static StatusCode FromHttpStatus(int httpStatus) => httpStatus switch
    {
        400 => StatusCode.Internal,
        401 => StatusCode.Unauthenticated,
        403 => StatusCode.PermissionDenied,
        404 => StatusCode.Unimplemented,
        429 or 502 or 503 or 504 => StatusCode.Unavailable,
        _ => StatusCode.Unknown,
    };

    /// <inheritdoc/>
    public async Task<TResponse> UnaryAsync<TRequest, TResponse>(
        RpcMethod<TRequest, TResponse> method,
        TRequest request,
        DateTime deadline,
        CancellationToken cancellationToken)
        where TRequest : class, IMessage<TRequest>
        where TResponse : class, IMessage<TResponse>
    {
        var name = method.Untyped.QualifiedName;
        var body = Encoding == MessageEncoding.Json ? JsonBodies.Serialize(request) : request.ToByteArray();
        if (body.Length > maxRequestSize)
        {
            throw new TransportException(
                StatusCode.ResourceExhausted,
                $"{name}: the request is {body.Length} bytes, more than ConnectionOptions.MaxRequestSize ({maxRequestSize}); it was not sent.",
                name);
        }

        var remaining = deadline - DateTime.UtcNow;
        if (remaining <= TimeSpan.Zero)
        {
            throw Late(name, cause: null);
        }

        using var timer = new CancellationTokenSource(remaining);
        using var call = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken, timer.Token);
        using var message = Request(name, body, remaining);
        var answered = false;
        try
        {
            using var response = await invoker.SendAsync(message, call.Token).ConfigureAwait(false);
            answered = true;
            return await ReadAsync(method, response, call.Token).ConfigureAwait(false);
        }
        catch (Exception error) when (error is not OpenSysMLException)
        {
            throw Map(error, name, answered, cancellationToken, timer.Token);
        }
    }

    /// <inheritdoc/>
    public ValueTask DisposeAsync()
    {
        if (Interlocked.Exchange(ref disposed, 1) == 0)
        {
            invoker.Dispose();
        }

        return ValueTask.CompletedTask;
    }

    /// <summary>
    /// The client's report of a call that failed without an answer it could read: cancellation when
    /// the call's token was cancelled, the deadline when it passed, and otherwise a transport failure
    /// classified by its causes.
    /// </summary>
    /// <param name="error">The failure.</param>
    /// <param name="method">The method called.</param>
    /// <param name="answered">Whether the response's headers had arrived.</param>
    /// <param name="cancellationToken">The call's token.</param>
    /// <param name="deadline">The token the deadline cancels.</param>
    /// <returns>The exception to raise.</returns>
    internal Exception Map(Exception error, string method, bool answered, CancellationToken cancellationToken, CancellationToken deadline)
    {
        // A read that fails once the call is cancelled or late failed because it was; an answer that did not decode did not.
        if (!IsUndecodable(error))
        {
            if (cancellationToken.IsCancellationRequested)
            {
                return new OperationCanceledException($"{method} was cancelled.", error, cancellationToken);
            }

            if (deadline.IsCancellationRequested)
            {
                return Late(method, error);
            }
        }

        var (status, why) = Classify(error, answered);
        return new TransportException(status, $"{method} failed with {status}: {why}", method, error);
    }

    private static bool IsUndecodable(Exception error) =>
        error is InvalidProtocolBufferException or InvalidJsonException or InvalidDataException or DecoderFallbackException;

    private static HttpRequestError? RequestError(Exception error)
    {
        for (Exception? cause = error; cause is not null; cause = cause.InnerException)
        {
            switch (cause)
            {
                case HttpRequestException request:
                    return request.HttpRequestError;
                case HttpIOException io:
                    return io.HttpRequestError;
            }
        }

        return null;
    }

    private static SocketsHttpHandler CreateHandler() => new()
    {
        PooledConnectionLifetime = TimeSpan.FromMinutes(5),
        AutomaticDecompression = DecompressionMethods.None,
        AllowAutoRedirect = false,
        UseCookies = false,
    };

    private static bool? IsGzip(ICollection<string> codings) => codings.Count switch
    {
        0 => false,
        1 when string.Equals(codings.Single(), ResponseBody.Gzip, StringComparison.OrdinalIgnoreCase) => true,
        1 when string.Equals(codings.Single(), "identity", StringComparison.OrdinalIgnoreCase) => false,
        _ => null,
    };

    private static bool IsJson(MediaTypeHeaderValue? type) =>
        type is not null
        && string.Equals(type.MediaType, JsonBodies.MediaType, StringComparison.OrdinalIgnoreCase)
        && type.Parameters.All(parameter => string.Equals(parameter.Name, "charset", StringComparison.OrdinalIgnoreCase)
            && string.Equals(parameter.Value?.Trim('"'), "utf-8", StringComparison.OrdinalIgnoreCase));

    private static string Describe(MediaTypeHeaderValue? type) => type is null ? "no content type" : type.ToString();

    private static string Excerpt(ReadOnlySpan<byte> body)
    {
        var text = System.Text.Encoding.UTF8.GetString(body[..Math.Min(body.Length, ExcerptLength * 4)]);
        var excerpt = new StringBuilder(ExcerptLength + 1);
        foreach (var c in text)
        {
            if (excerpt.Length == ExcerptLength)
            {
                excerpt.Append('…');
                break;
            }

            excerpt.Append(char.IsControl(c) ? ' ' : c);
        }

        return excerpt.ToString().Trim();
    }

    private static bool TryReadError(ReadOnlyMemory<byte> body, out StatusCode code, out string message)
    {
        code = StatusCode.Unknown;
        message = string.Empty;

        // JsonDocument checks the UTF-8 of a string only when it is read, so the body is checked whole first.
        if (!Utf8.IsValid(body.Span))
        {
            return false;
        }

        try
        {
            using var document = JsonDocument.Parse(body);
            var error = document.RootElement;
            if (error.ValueKind != JsonValueKind.Object
                || !error.TryGetProperty("code", out var name)
                || name.ValueKind != JsonValueKind.String
                || !Codes.TryGetValue(name.GetString()!, out code))
            {
                return false;
            }

            if (error.TryGetProperty("message", out var text) && text.ValueKind != JsonValueKind.Null)
            {
                if (text.ValueKind != JsonValueKind.String)
                {
                    return false;
                }

                message = text.GetString()!;
            }

            return !error.TryGetProperty("details", out var details) || details.ValueKind is JsonValueKind.Array or JsonValueKind.Null;
        }
        catch (JsonException)
        {
            return false;
        }
    }

    private (StatusCode Status, string Why) Classify(Exception error, bool answered)
    {
        switch (error)
        {
            case InvalidProtocolBufferException or InvalidJsonException:
                return (StatusCode.Internal, $"the answer from {Address} is not the response message in {Encoding}: {error.Message}");
            case DecoderFallbackException:
                return (StatusCode.Internal, $"the answer from {Address} is not UTF-8 JSON: {error.Message}");
            case InvalidDataException:
                return (StatusCode.Internal, $"the answer from {Address} could not be read: {error.Message}.");
        }

        var unknown = $"{Causes.Innermost(error).Message} Whether the service received the call is unknown.";
        switch (RequestError(error))
        {
            case HttpRequestError.NameResolutionError or HttpRequestError.ConnectionError
                or HttpRequestError.SecureConnectionError or HttpRequestError.ProxyTunnelError:
                return (StatusCode.Unavailable, $"{Address} could not be reached: {unknown}");
            case HttpRequestError.InvalidResponse or HttpRequestError.HttpProtocolError
                or HttpRequestError.VersionNegotiationError or HttpRequestError.ConfigurationLimitExceeded:
                return (StatusCode.Internal, $"{Address} did not answer as an HTTP/1.1 endpoint: {unknown}");
            case HttpRequestError.ResponseEnded when answered:
                return (StatusCode.Internal, $"the answer from {Address} ended before it was complete: {unknown}");
        }

        if (error is HttpRequestException or IOException || Causes.Find<SocketException>(error) is not null)
        {
            return (StatusCode.Unavailable, answered
                ? $"the connection to {Address} failed while the answer was read: {unknown}"
                : $"the connection to {Address} failed before an answer was read: {unknown}");
        }

        return (StatusCode.Unknown, $"talking to {Address} failed: {unknown}");
    }

    private TransportException Late(string method, Exception? cause) =>
        new(StatusCode.DeadlineExceeded, $"{method} did not complete before its deadline; the service at {Address} may still be running it.", method, cause);

    private HttpRequestMessage Request(string method, byte[] body, TimeSpan remaining)
    {
        var request = new HttpRequestMessage(HttpMethod.Post, new Uri(Address, method))
        {
            Version = HttpVersion.Version11,
            VersionPolicy = HttpVersionPolicy.RequestVersionOrLower,
            Content = new ByteArrayContent(body),
        };
        request.Content.Headers.ContentType = new MediaTypeHeaderValue(mediaType);
        foreach (var (name, value) in headers.Entries)
        {
            if (!request.Headers.TryAddWithoutValidation(name, value))
            {
                request.Content.Headers.TryAddWithoutValidation(name, value);
            }
        }

        var timeout = Math.Clamp((long)Math.Ceiling(remaining.TotalMilliseconds), 1, MaxTimeoutMilliseconds);
        request.Headers.TryAddWithoutValidation("connect-protocol-version", ProtocolVersion);
        request.Headers.TryAddWithoutValidation("connect-timeout-ms", timeout.ToString(CultureInfo.InvariantCulture));
        request.Headers.TryAddWithoutValidation("accept-encoding", ResponseBody.Gzip);
        return request;
    }

    private async Task<TResponse> ReadAsync<TRequest, TResponse>(RpcMethod<TRequest, TResponse> method, HttpResponseMessage response, CancellationToken cancellationToken)
        where TRequest : class, IMessage<TRequest>
        where TResponse : class, IMessage<TResponse>
    {
        var name = method.Untyped.QualifiedName;
        var codings = response.Content.Headers.ContentEncoding;
        var gzip = IsGzip(codings);
        if (response.StatusCode != HttpStatusCode.OK)
        {
            throw await RefusalAsync(name, response, gzip, cancellationToken).ConfigureAwait(false);
        }

        var type = response.Content.Headers.ContentType;
        if (!(Encoding == MessageEncoding.Json
            ? IsJson(type)
            : string.Equals(type?.MediaType, ProtobufMediaType, StringComparison.OrdinalIgnoreCase) && type!.Parameters.Count == 0))
        {
            // The protocol's own reading: an answer of another application/ type is broken, anything else not a Connect answer at all.
            var status = type?.MediaType?.StartsWith("application/", StringComparison.OrdinalIgnoreCase) == true ? StatusCode.Internal : StatusCode.Unknown;
            throw new TransportException(status, $"{name} failed with {status}: {Address} answered {Describe(type)}, where a Connect answer is {mediaType}.", name);
        }

        if (gzip is null)
        {
            throw new TransportException(
                StatusCode.Internal,
                $"{name} failed with Internal: {Address} encoded its answer as \"{string.Join(", ", codings)}\", where this client accepts only gzip.",
                name);
        }

        var body = await ResponseBody.ReadAsync(response.Content, gzip.Value, maxResponseSize, name, cancellationToken).ConfigureAwait(false);
        return Encoding == MessageEncoding.Json
            ? (TResponse)JsonBodies.Parse(body.Span, method.Untyped.Descriptor.OutputType)
            : method.ResponseParser.ParseFrom(body.Span);
    }

    private async Task<Exception> RefusalAsync(string method, HttpResponseMessage response, bool? gzip, CancellationToken cancellationToken)
    {
        var http = (int)response.StatusCode;
        var type = response.Content.Headers.ContentType;
        string detail;
        if (gzip is null)
        {
            detail = $"its body is encoded as \"{string.Join(", ", response.Content.Headers.ContentEncoding)}\", which this client did not ask for";
        }
        else if (IsJson(type))
        {
            try
            {
                var body = await ResponseBody.ReadAsync(response.Content, gzip.Value, maxResponseSize, method, cancellationToken).ConfigureAwait(false);
                if (TryReadError(body, out var code, out var message))
                {
                    return code == StatusCode.DeadlineExceeded
                        ? new TransportException(code, $"{method}: the service at {Address} reports that the call outlived its deadline: {message}", method)
                        : ServiceErrors.FromService(code, message, method, headers.Credentials);
                }

                detail = body.Length == 0 ? "an empty body" : $"\"{Excerpt(body.Span)}\"";
            }
            catch (Exception error) when (error is not OperationCanceledException && (error is TransportException or InvalidDataException or IOException or HttpRequestException))
            {
                detail = $"a body that could not be read: {error.Message}";
            }
        }
        else
        {
            try
            {
                var prefix = await ResponseBody.ReadPrefixAsync(response.Content, gzip.Value, ExcerptLength * 4, cancellationToken).ConfigureAwait(false);
                detail = prefix.Length == 0 ? "an empty body" : $"\"{Excerpt(prefix)}\"";
            }
            catch (Exception error) when (error is not OperationCanceledException && (error is InvalidDataException or IOException or HttpRequestException))
            {
                detail = $"a body that could not be read: {error.Message}";
            }
        }

        var inferred = FromHttpStatus(http);
        return new TransportException(
            inferred,
            $"{method} failed with {inferred}: {Address} answered HTTP {http.ToString(CultureInfo.InvariantCulture)} {response.ReasonPhrase} "
                + $"with {Describe(type)} and {detail}, which is not a Connect error, so the status is inferred from the HTTP status.",
            method);
    }
}
