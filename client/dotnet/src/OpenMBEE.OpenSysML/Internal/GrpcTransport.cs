using System;
using System.Globalization;
using System.Linq;
using System.Net.Http;
using System.Net.Sockets;
using System.Threading;
using System.Threading.Tasks;
using Google.Protobuf;
using Grpc.Core;
using Grpc.Net.Client;
using Grpc.Net.Compression;

namespace OpenMBEE.OpenSysML.Internal;

/// <summary>
/// Unary calls over gRPC on one channel, which this transport owns. The channel advertises identity
/// encoding alone, since its receive limit bounds a compressed frame, not the message it inflates
/// to; it never retries; and it reports caller cancellation through the call's token rather than
/// letting the library raise it, so a deadline and a cancellation stay distinguishable.
/// </summary>
internal sealed class GrpcTransport : ITransport
{
    private readonly GrpcChannel channel;
    private readonly CallInvoker invoker;
    private readonly CallHeaders headers;
    private int disposed;

    /// <summary>Creates the transport for a connection.</summary>
    /// <param name="address">The service's base URL.</param>
    /// <param name="options">The connection's options, already validated.</param>
    /// <param name="headers">The headers every call carries.</param>
    internal GrpcTransport(Uri address, ConnectionOptions options, CallHeaders headers)
        : this(address, options.HttpMessageHandler ?? CreateHandler(), ownsHandler: options.HttpMessageHandler is null, options, headers)
    {
    }

    /// <summary>Creates the transport over a handler.</summary>
    /// <param name="address">The service's base URL.</param>
    /// <param name="handler">The handler to send through.</param>
    /// <param name="ownsHandler">Whether disposing the transport disposes the handler; a borrowed one is left alone.</param>
    /// <param name="options">The connection's options, already validated.</param>
    /// <param name="headers">The headers every call carries.</param>
    internal GrpcTransport(Uri address, HttpMessageHandler handler, bool ownsHandler, ConnectionOptions options, CallHeaders headers)
    {
        Address = address;
        this.headers = headers;
        var observer = new ResponseObserver { InnerHandler = ownsHandler ? handler : new BorrowedHandler(handler) };
        try
        {
            channel = GrpcChannel.ForAddress(address, new GrpcChannelOptions
            {
                HttpHandler = observer,
                DisposeHttpClient = true,
                MaxSendMessageSize = options.MaxRequestSize,
                MaxReceiveMessageSize = options.MaxResponseSize,
                CompressionProviders = Array.Empty<ICompressionProvider>(),
                ThrowOperationCanceledOnCancellation = false,
                ServiceConfig = null,
            });
        }
        catch
        {
            observer.Dispose();
            throw;
        }

        invoker = channel.CreateCallInvoker();
    }

    /// <inheritdoc/>
    public Protocol Protocol => Protocol.Grpc;

    /// <inheritdoc/>
    public MessageEncoding Encoding => MessageEncoding.Protobuf;

    /// <summary>The service's base URL.</summary>
    internal Uri Address { get; }

    /// <inheritdoc/>
    public async Task<TResponse> UnaryAsync<TRequest, TResponse>(
        RpcMethod<TRequest, TResponse> method,
        TRequest request,
        DateTime deadline,
        CancellationToken cancellationToken)
        where TRequest : class, IMessage<TRequest>
        where TResponse : class, IMessage<TResponse>
    {
        var metadata = new Metadata();
        foreach (var (name, value) in headers.Entries)
        {
            metadata.Add(name, value);
        }

        var observation = new ResponseObservation();
        ResponseObserver.Current.Value = observation;
        try
        {
            using var call = invoker.AsyncUnaryCall(method.Grpc, host: null, new CallOptions(metadata, deadline, cancellationToken), request);
            return await call.ResponseAsync.ConfigureAwait(false);
        }
        catch (RpcException error)
        {
            throw Map(error, observation.ServerStatus, method.Untyped.QualifiedName, Address, headers.Credentials, cancellationToken);
        }
        finally
        {
            ResponseObserver.Current.Value = null;
        }
    }

    /// <inheritdoc/>
    public ValueTask DisposeAsync()
    {
        if (Interlocked.Exchange(ref disposed, 1) == 0)
        {
            channel.Dispose();
        }

        return ValueTask.CompletedTask;
    }

    /// <summary>
    /// The client's report of a failed gRPC call. A status is the service's only when the service
    /// sent it, read from the response's own grpc-status, and the library raised it unchanged with no
    /// local cause; anything else, a status the library made up included, failed in transport.
    /// </summary>
    /// <param name="error">The library's report.</param>
    /// <param name="serverStatus">The grpc-status the response carried, if one was read.</param>
    /// <param name="method">The method called.</param>
    /// <param name="address">The service's base URL.</param>
    /// <param name="credentials">Where the credentials the call carried came from.</param>
    /// <param name="cancellationToken">The call's token.</param>
    /// <returns>The exception to raise.</returns>
    internal static Exception Map(RpcException error, int? serverStatus, string method, Uri address, CredentialSource credentials, CancellationToken cancellationToken)
    {
        var raw = (int)error.StatusCode;
        var status = Enum.IsDefined((StatusCode)raw) ? (StatusCode)raw : StatusCode.Unknown;
        if (cancellationToken.IsCancellationRequested && status is StatusCode.Cancelled or StatusCode.DeadlineExceeded)
        {
            return new OperationCanceledException($"{method} was cancelled.", error, cancellationToken);
        }

        var cause = error.Status.DebugException;
        var fromService = cause is null && raw != 0 && serverStatus == raw;
        if (fromService && status != StatusCode.DeadlineExceeded)
        {
            return ServiceErrors.FromService(status, error.Status.Detail, method, credentials, error);
        }

        var requestFailure = Causes.Find<HttpRequestException>(cause);
        var protocolFailure = Causes.Find<HttpProtocolException>(cause);
        var negotiationFailed = requestFailure?.HttpRequestError == HttpRequestError.VersionNegotiationError;

        // Malformed responses and failed HTTP/2 negotiation are INTERNAL, regardless of the library's status.
        if (Causes.Find<InvalidProtocolBufferException>(cause) is not null || negotiationFailed)
        {
            status = StatusCode.Internal;
        }

        // gRPC makes a connection that could not be established UNAVAILABLE; the library says Internal for some, as a failed TLS handshake.
        if (requestFailure is
            {
                HttpRequestError: HttpRequestError.NameResolutionError or HttpRequestError.ConnectionError
                    or HttpRequestError.SecureConnectionError or HttpRequestError.ProxyTunnelError,
            })
        {
            status = StatusCode.Unavailable;
        }

        string message;
        if (status == StatusCode.DeadlineExceeded)
        {
            message = fromService
                ? $"{method}: the service at {address} reports that the call outlived its deadline."
                : $"{method} did not complete before its deadline; the service at {address} may still be running it.";
        }
        else if (status == StatusCode.Internal && (protocolFailure is not null || negotiationFailed))
        {
            var detail = protocolFailure is null
                ? "HTTP/2 negotiation failed"
                : $"HTTP/2 protocol error {protocolFailure.ErrorCode.ToString(CultureInfo.InvariantCulture)}";
            message = $"{method}: {address} did not answer as an HTTP/2 gRPC endpoint ({detail}). "
                + "A server or proxy that speaks only HTTP/1.1 answers gRPC this way; "
                + "if it serves the Connect protocol, connect with ConnectionOptions.Protocol set to Protocol.Connect.";
        }
        else if (requestFailure is not null || Causes.Find<SocketException>(cause) is not null)
        {
            message = $"{method} failed with {status} talking to {address}: {Causes.Innermost(cause!).Message} "
                + "Whether the service received the call is unknown.";
        }
        else if (cause is not null)
        {
            message = $"{method} failed with {status}: the answer from {address} could not be read: {Causes.Innermost(cause).Message}";
        }
        else
        {
            message = $"{method} failed with {status} before an answer from {address} was read: {error.Status.Detail}";
        }

        return new TransportException(status, message, method, cause ?? error);
    }

    private static SocketsHttpHandler CreateHandler() => new()
    {
        PooledConnectionLifetime = TimeSpan.FromMinutes(5),
        EnableMultipleHttp2Connections = true,
    };

    /// <summary>What the response to one call carried, as the handler saw it.</summary>
    private sealed class ResponseObservation
    {
        private HttpResponseMessage? response;

        /// <summary>The grpc-status the response carried in its headers or trailers, if any could be read.</summary>
        internal int? ServerStatus
        {
            get
            {
                var seen = Volatile.Read(ref response);
                if (seen is null)
                {
                    return null;
                }

                return Status(seen.Headers) ?? Status(seen.TrailingHeaders);
            }
        }

        internal void Observe(HttpResponseMessage observed) => Volatile.Write(ref response, observed);

        private static int? Status(System.Net.Http.Headers.HttpHeaders headers) =>
            headers.TryGetValues("grpc-status", out var values)
                && values.SingleOrDefault() is { } value
                && int.TryParse(value, NumberStyles.None, CultureInfo.InvariantCulture, out var status)
                ? status
                : null;
    }

    /// <summary>Hands each call's response to the observation the call made.</summary>
    private sealed class ResponseObserver : DelegatingHandler
    {
        /// <summary>The observation of the call being made on this flow of execution.</summary>
        internal static readonly AsyncLocal<ResponseObservation?> Current = new();

        protected override async Task<HttpResponseMessage> SendAsync(HttpRequestMessage request, CancellationToken cancellationToken)
        {
            var observation = Current.Value;
            var response = await base.SendAsync(request, cancellationToken).ConfigureAwait(false);
            observation?.Observe(response);
            return response;
        }
    }

    /// <summary>Sends through a handler the connection does not own, and never disposes it.</summary>
    private sealed class BorrowedHandler(HttpMessageHandler handler) : HttpMessageHandler
    {
        private readonly HttpMessageInvoker invoker = new(handler, disposeHandler: false);

        protected override Task<HttpResponseMessage> SendAsync(HttpRequestMessage request, CancellationToken cancellationToken) =>
            invoker.SendAsync(request, cancellationToken);

        protected override void Dispose(bool disposing)
        {
            if (disposing)
            {
                invoker.Dispose();
            }

            base.Dispose(disposing);
        }
    }
}
