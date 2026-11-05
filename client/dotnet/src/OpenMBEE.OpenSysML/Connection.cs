using System;
using System.Threading;
using System.Threading.Tasks;
using Google.Protobuf;
using Microsoft.Extensions.Logging;
using OpenMBEE.OpenSysML.Internal;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// A connection to a <c>sysml-grpc</c> service, and the entry point of this client. Opening one asks
/// the service what it is, once; <see cref="ServerInfo"/> keeps the answer. A connection is safe to
/// share between threads and owns one channel to the service, which disposing it closes. A service
/// this client did not start is never stopped by it.
/// </summary>
/// <remarks>
/// <para>
/// A connection speaks gRPC, or the Connect protocol with protobuf or JSON bodies, as
/// <see cref="ConnectionOptions.Protocol"/> and <see cref="ConnectionOptions.Encoding"/> say, and
/// never another. It reaches the service <see cref="ConnectionOptions.Address"/> names, else the one
/// <see cref="ConnectionOptions.ServiceVariable"/> names, else a private service it starts.
/// </para>
/// <para>
/// A private service is a <c>sysml-grpc</c> child of this process that only this process can reach,
/// and that exits when this process does, however it ends. The connections of one copy of this client
/// that would start the same service share it: the same binary, with the same contents, under the
/// same <see cref="ConnectionOptions.StartupTimeout"/>, <see cref="ConnectionOptions.ShutdownTimeout"/>
/// and <see cref="ConnectionOptions.Version"/>. The last of them to close stops it.
/// <see cref="ConnectionOptions.IsolatedService"/> gives a connection one of its own. A private service
/// that exits on its own is not restarted: its connections fail from then on with
/// <see cref="TransportException"/>, and a connection opened afterwards starts another.
/// </para>
/// </remarks>
public sealed partial class Connection : IAsyncDisposable
{
    private const string ServerInfoMethod = "GetServerInfo";

    private readonly ITransport transport;
    private readonly ServiceRegistry.Lease? lease;
    private readonly ILogger logger;
    private readonly TimeSpan requestTimeout;
    private readonly CancellationTokenSource closing = new();
    private readonly TaskCompletionSource drained = new(TaskCreationOptions.RunContinuationsAsynchronously);
    private readonly TaskCompletionSource closed = new(TaskCreationOptions.RunContinuationsAsynchronously);
    private ServerInfo? serverInfo;
    private int active;
    private int disposing;

    private Connection(ITransport transport, Uri address, ConnectionOptions options, ServiceRegistry.Lease? lease)
    {
        this.transport = transport;
        this.lease = lease;
        Address = address;
        logger = options.Logger;
        requestTimeout = options.RequestTimeout;
    }

    /// <summary>The base URL of the service: for a private service, the loopback address it reported.</summary>
    public Uri Address { get; }

    /// <summary>The protocol the connection speaks.</summary>
    public Protocol Protocol => transport.Protocol;

    /// <summary>How the connection encodes messages.</summary>
    public MessageEncoding Encoding => transport.Encoding;

    /// <summary>What the service said of itself when the connection opened.</summary>
    public ServerInfo ServerInfo => serverInfo ?? throw new InvalidOperationException("The connection has not finished opening.");

    /// <summary>The capabilities the service advertised when the connection opened.</summary>
    public Capabilities Capabilities => ServerInfo.Capabilities;

    /// <summary>
    /// Opens a connection: to the service named, or else to a private one, started unless one the
    /// connection can share is running; asks the service for its <see cref="ServerInfo"/>, then checks
    /// <see cref="ConnectionOptions.Version"/> and <see cref="ConnectionOptions.RequireCapabilities"/>
    /// against the answer. A failure after the channel was made closes it again, and gives up its hold
    /// on a private service, which stops when nothing else holds it.
    /// </summary>
    /// <param name="options">How to reach the service; null for <see cref="ConnectionOptions.Default"/>.</param>
    /// <param name="cancellationToken">Cancels the open, and the start of a private service no other open waits for.</param>
    /// <returns>The open connection, which the caller disposes.</returns>
    /// <exception cref="InvalidRequestException">An option is invalid, or <see cref="ConnectionOptions.TokenVariable"/> names no bearer token.</exception>
    /// <exception cref="BinaryNotFoundException">No service is named, no binary was found to start one, and no release could be downloaded.</exception>
    /// <exception cref="ChecksumMismatchException">
    /// The binary does not have <see cref="ConnectionOptions.ExpectedBinarySha256"/>, or a release does not have the digest pinned or required for it.
    /// </exception>
    /// <exception cref="UnpinnedReleaseException">The release asked for is one this client pins no digest for.</exception>
    /// <exception cref="UnsupportedPlatformException">No service is named, and this platform cannot start one, or has no release to download that was asked for.</exception>
    /// <exception cref="ServiceStartException">
    /// No service is named and <see cref="ConnectionOptions.AutoStart"/> is false, the release asked for could not be downloaded,
    /// or the private service did not start.
    /// </exception>
    /// <exception cref="StaleServiceException">The service is not the <see cref="ConnectionOptions.Version"/> asked for.</exception>
    /// <exception cref="CapabilityException">The service lacks a required capability.</exception>
    /// <exception cref="TransportException">The service could not be reached.</exception>
    /// <exception cref="AuthenticationException">The service requires a bearer token the connection does not carry, or refused the one it carries.</exception>
    /// <exception cref="ServiceException">The service refused GetServerInfo.</exception>
    public static Task<Connection> OpenAsync(ConnectionOptions? options = null, CancellationToken cancellationToken = default) =>
        OpenAsync(options, Environment.GetEnvironmentVariable, cancellationToken);

    /// <summary>
    /// Stops every private service the connections of this copy of the client share, whoever holds
    /// them, as a host does before unloading the client. Connections to them fail from then on with
    /// <see cref="TransportException"/>; they are never moved to another service. A connection opened
    /// afterwards starts a new one. Isolated services are left to their connections, and a service
    /// this client did not start is never touched. Every service is tried before any failure is reported.
    /// </summary>
    /// <param name="cancellationToken">Stops the wait; the services are stopped regardless.</param>
    /// <returns>How many services were stopped.</returns>
    /// <exception cref="ServiceStopException">Some services could not be stopped cleanly.</exception>
    /// <exception cref="OperationCanceledException"><paramref name="cancellationToken"/> was cancelled.</exception>
    public static Task<int> StopSharedServicesAsync(CancellationToken cancellationToken = default) =>
        ServiceRegistry.Shared.StopAllAsync(cancellationToken);

    /// <summary>
    /// Calls a method of the service directly, with the connection's headers, timeout, size limits,
    /// error mapping and disposal checks. This is the escape hatch below the typed API: it checks
    /// no capability and interprets no result, so a failure the answer reports in band is returned
    /// as an ordinary response. The service still validates the request.
    /// </summary>
    /// <typeparam name="TRequest">The request message the method takes.</typeparam>
    /// <typeparam name="TResponse">The response message the method returns.</typeparam>
    /// <param name="method">The method, bare (<c>ParseFile</c>) or qualified (<c>sysml.SysMLService/ParseFile</c>).</param>
    /// <param name="request">The request; the call does not change it.</param>
    /// <param name="cancellationToken">Cancels the call.</param>
    /// <returns>The response.</returns>
    /// <exception cref="InvalidRequestException">The service has no such method, or not with these messages; nothing was sent.</exception>
    /// <exception cref="ServiceException">The service refused the call.</exception>
    /// <exception cref="TransportException">The call failed in transport.</exception>
    /// <exception cref="OperationCanceledException"><paramref name="cancellationToken"/> was cancelled.</exception>
    /// <exception cref="ObjectDisposedException">The connection was disposed before or during the call.</exception>
    public Task<TResponse> CallAsync<TRequest, TResponse>(string method, TRequest request, CancellationToken cancellationToken = default)
        where TRequest : class, IMessage<TRequest>
        where TResponse : class, IMessage<TResponse>
    {
        ArgumentNullException.ThrowIfNull(method);
        ArgumentNullException.ThrowIfNull(request);
        ObjectDisposedException.ThrowIf(Volatile.Read(ref disposing) != 0, this);
        if (!RpcMethods.TryResolve<TRequest, TResponse>(method, out var resolved, out var problem))
        {
            throw new InvalidRequestException(problem);
        }

        return InvokeAsync(resolved, request, cancellationToken);
    }

    /// <summary>
    /// Closes the connection: refuses new calls, cancels the calls in flight, which fail with
    /// <see cref="ObjectDisposedException"/>, waits for them, and closes the channel; then gives up its
    /// hold on a private service, stopping it when it was the last. Disposing again, or concurrently,
    /// waits for the same close; it never throws. A failure to close or to stop is logged.
    /// </summary>
    /// <returns>When the connection is closed.</returns>
    public async ValueTask DisposeAsync()
    {
        if (Interlocked.Exchange(ref disposing, 1) != 0)
        {
            await closed.Task.ConfigureAwait(false);
            return;
        }

        try
        {
            await closing.CancelAsync().ConfigureAwait(false);
        }
        catch (AggregateException error)
        {
            LogCloseFailed(logger, Address, error);
        }

        if (Volatile.Read(ref active) == 0)
        {
            drained.TrySetResult();
        }

        await drained.Task.ConfigureAwait(false);
        try
        {
            await transport.DisposeAsync().ConfigureAwait(false);
        }
#pragma warning disable CA1031 // Disposal never throws: a failure to close is logged, as the contract says.
        catch (Exception error)
#pragma warning restore CA1031
        {
            LogCloseFailed(logger, Address, error);
        }

        closing.Dispose();
        LogClosed(logger, Address);
        if (lease is not null)
        {
            await lease.ReleaseAsync(logger).ConfigureAwait(false);
        }

        closed.TrySetResult();
    }

    /// <summary>Opens a connection, reading the environment through <paramref name="environment"/>.</summary>
    /// <param name="options">How to reach the service; null for <see cref="ConnectionOptions.Default"/>.</param>
    /// <param name="environment">Reads an environment variable; null when it is unset.</param>
    /// <param name="cancellationToken">Cancels the open.</param>
    /// <returns>The open connection.</returns>
    internal static Task<Connection> OpenAsync(
        ConnectionOptions? options,
        Func<string, string?> environment,
        CancellationToken cancellationToken) =>
        OpenAsync(options, environment, ServiceRegistry.Shared, cancellationToken);

    /// <summary>Opens a connection, starting a private service through <paramref name="registry"/> when none is named.</summary>
    /// <param name="options">How to reach the service; null for <see cref="ConnectionOptions.Default"/>.</param>
    /// <param name="environment">Reads an environment variable; null when it is unset.</param>
    /// <param name="registry">The registry a private service is taken from.</param>
    /// <param name="cancellationToken">Cancels the open.</param>
    /// <returns>The open connection.</returns>
    internal static Task<Connection> OpenAsync(
        ConnectionOptions? options,
        Func<string, string?> environment,
        ServiceRegistry registry,
        CancellationToken cancellationToken) =>
        OpenAsync(options, environment, registry, ReleaseSource.Production, cancellationToken);

    /// <summary>
    /// Opens a connection, starting a private service through <paramref name="registry"/> when none is named, from a
    /// release of <paramref name="releases"/> when no binary is named.
    /// </summary>
    /// <param name="options">How to reach the service; null for <see cref="ConnectionOptions.Default"/>.</param>
    /// <param name="environment">Reads an environment variable; null when it is unset.</param>
    /// <param name="registry">The registry a private service is taken from.</param>
    /// <param name="releases">Where a release is downloaded from.</param>
    /// <param name="cancellationToken">Cancels the open.</param>
    /// <returns>The open connection.</returns>
    internal static async Task<Connection> OpenAsync(
        ConnectionOptions? options,
        Func<string, string?> environment,
        ServiceRegistry registry,
        ReleaseSource releases,
        CancellationToken cancellationToken)
    {
        options ??= ConnectionOptions.Default;
        options.Validate();
        var address = ResolveAddress(options, environment);
        cancellationToken.ThrowIfCancellationRequested();
        var headers = CallHeaders.Resolve(options, environment);
        if (address is not null)
        {
            if (options.BinaryPath is not null || options.ExpectedBinarySha256 is not null || options.IsolatedService
                || options.DownloadVersion is not null || options.GithubRepo is not null)
            {
                LogPrivateOptionsUnused(options.Logger, address);
            }

            if (headers.Credentials != CredentialSource.None && address.Scheme == Uri.UriSchemeHttp && !address.IsLoopback)
            {
                LogCleartextCredentials(options.Logger, address);
            }

            return await OpenAsync(CreateTransport(address, options, headers), address, options, null, cancellationToken).ConfigureAwait(false);
        }

        var binary = await BinaryResolver.ResolveAsync(options, environment, releases, cancellationToken).ConfigureAwait(false);
        var lease = await registry.AcquireAsync(ServiceIdentity.Of(binary, options), options.IsolatedService, options.Logger, cancellationToken).ConfigureAwait(false);
        ITransport transport;
        try
        {
            transport = CreateTransport(lease.Address, options, headers);
        }
        catch
        {
            await lease.ReleaseAsync(options.Logger).ConfigureAwait(false);
            throw;
        }

        return await OpenAsync(transport, lease.Address, options, lease, cancellationToken).ConfigureAwait(false);
    }

    /// <summary>
    /// Opens a connection over a transport, which the connection then owns, with a lease it then holds,
    /// even when the open fails.
    /// </summary>
    /// <param name="transport">The transport.</param>
    /// <param name="address">The service's base URL.</param>
    /// <param name="options">The connection's options, already validated.</param>
    /// <param name="lease">The hold on the private service, or null for a named one.</param>
    /// <param name="cancellationToken">Cancels the open.</param>
    /// <returns>The open connection.</returns>
    internal static async Task<Connection> OpenAsync(
        ITransport transport,
        Uri address,
        ConnectionOptions options,
        ServiceRegistry.Lease? lease,
        CancellationToken cancellationToken)
    {
        var connection = new Connection(transport, address, options, lease);
        try
        {
            connection.serverInfo = await connection.AskServerInfoAsync(cancellationToken).ConfigureAwait(false);
            connection.RequireOptions(options);
        }
        catch
        {
            await connection.DisposeAsync().ConfigureAwait(false);
            throw;
        }

        LogOpened(connection.logger, address, connection.Protocol, connection.Encoding, connection.serverInfo);
        return connection;
    }

    /// <summary>The address options name, null for a private service, or the refusal of a connection that names none and starts none.</summary>
    /// <param name="options">The connection's options.</param>
    /// <param name="environment">Reads an environment variable.</param>
    /// <returns>The service's base URL; null when a private service is to be started.</returns>
    /// <exception cref="ServiceStartException">No service is named and <see cref="ConnectionOptions.AutoStart"/> is false.</exception>
    internal static Uri? ResolveAddress(ConnectionOptions options, Func<string, string?> environment)
    {
        if (options.Address is not null)
        {
            return ServiceAddress.Parse(options.Address, "ConnectionOptions.Address");
        }

        var named = environment(ConnectionOptions.ServiceVariable);
        if (!string.IsNullOrWhiteSpace(named))
        {
            return ServiceAddress.Parse(named, "$" + ConnectionOptions.ServiceVariable);
        }

        if (options.AutoStart)
        {
            return null;
        }

        throw new ServiceStartException("No service was named and ConnectionOptions.AutoStart is false: "
            + $"name a running sysml-grpc with ConnectionOptions.Address or ${ConnectionOptions.ServiceVariable}, or allow a private one.");
    }

    private static ITransport CreateTransport(Uri address, ConnectionOptions options, CallHeaders headers) =>
        options.Protocol == Protocol.Connect ? new ConnectTransport(address, options, headers) : new GrpcTransport(address, options, headers);

    private async Task<ServerInfo> AskServerInfoAsync(CancellationToken cancellationToken)
    {
        if (!RpcMethods.TryResolve<Wire.ServerInfoRequest, Wire.ServerInfoResponse>(ServerInfoMethod, out var method, out var problem))
        {
            throw new InvalidOperationException(problem);
        }

        try
        {
            var response = await InvokeAsync(method, new Wire.ServerInfoRequest(), cancellationToken).ConfigureAwait(false);
            return new ServerInfo(response.Version, new Capabilities(response.Capabilities, response.Version), answered: true);
        }
        catch (ServiceException error) when (error.StatusCode == StatusCode.Unimplemented)
        {
            LogServerInfoUnanswered(logger, Address);
            return new ServerInfo(string.Empty, Capabilities.None, answered: false);
        }
    }

    private void RequireOptions(ConnectionOptions options)
    {
        var info = ServerInfo;
        if (options.Version is { } version)
        {
            string? reason = null;
            if (!info.Answered)
            {
                reason = $"it did not answer GetServerInfo, so it cannot be shown to be the {version} asked for";
            }
            else if (!string.Equals(info.Version, version, StringComparison.Ordinal))
            {
                reason = $"it reports version {(info.Version.Length == 0 ? "(unknown)" : info.Version)}, but {version} was asked for";
            }

            if (reason is not null)
            {
                throw new StaleServiceException(
                    Address.ToString(),
                    reason,
                    lease is null
                        ? $"connect to a service of version {version}, or accept the one at {Address} by leaving ConnectionOptions.Version null"
                        : $"start a sysml-grpc of version {version}, named by ConnectionOptions.BinaryPath or ${ConnectionOptions.BinaryVariable}, "
                            + "or accept the one found by leaving ConnectionOptions.Version null",
                    info.Version);
            }
        }

        foreach (var capability in options.RequireCapabilities)
        {
            info.Capabilities.Require(capability);
        }
    }

    private async Task<TResponse> InvokeAsync<TRequest, TResponse>(RpcMethod<TRequest, TResponse> method, TRequest request, CancellationToken cancellationToken)
        where TRequest : class, IMessage<TRequest>
        where TResponse : class, IMessage<TResponse>
    {
        Enter();
        try
        {
            cancellationToken.ThrowIfCancellationRequested();
            if (lease?.EndReason is { } ended)
            {
                throw new TransportException(StatusCode.Unavailable, ended, method.Untyped.QualifiedName);
            }

            using var call = lease is null
                ? CancellationTokenSource.CreateLinkedTokenSource(cancellationToken, closing.Token)
                : CancellationTokenSource.CreateLinkedTokenSource(cancellationToken, closing.Token, lease.Ended);
            var deadline = DateTime.UtcNow + requestTimeout;
            try
            {
                return await transport.UnaryAsync(method, request, deadline, call.Token).ConfigureAwait(false);
            }
            catch (Exception error) when (error is OperationCanceledException or TransportException { StatusCode: StatusCode.Cancelled or StatusCode.DeadlineExceeded })
            {
                if (cancellationToken.IsCancellationRequested)
                {
                    throw new OperationCanceledException($"{method.Untyped.QualifiedName} was cancelled.", error, cancellationToken);
                }

                if (closing.IsCancellationRequested)
                {
                    throw new ObjectDisposedException($"{method.Untyped.QualifiedName} was cancelled because its connection was disposed.", error);
                }

                if (lease?.EndReason is { } reason)
                {
                    throw new TransportException(StatusCode.Unavailable, reason, method.Untyped.QualifiedName, error);
                }

                throw;
            }
            catch (TransportException error) when (lease?.EndReason is { } reason)
            {
                // The private service went: whatever the transport made of that, this is what happened.
                throw new TransportException(StatusCode.Unavailable, reason, method.Untyped.QualifiedName, error);
            }
        }
        finally
        {
            Exit();
        }
    }

    private void Enter()
    {
        Interlocked.Increment(ref active);
        if (Volatile.Read(ref disposing) != 0)
        {
            Exit();
            ObjectDisposedException.ThrowIf(true, this);
        }
    }

    private void Exit()
    {
        if (Interlocked.Decrement(ref active) == 0 && Volatile.Read(ref disposing) != 0)
        {
            drained.TrySetResult();
        }
    }

    [LoggerMessage(Level = LogLevel.Debug, Message = "Connected to {Address} over {Protocol} with {Encoding}: {ServerInfo}")]
    private static partial void LogOpened(ILogger logger, Uri address, Protocol protocol, MessageEncoding encoding, ServerInfo serverInfo);

    [LoggerMessage(Level = LogLevel.Warning, Message = "The service at {Address} did not answer GetServerInfo; it is assumed to advertise no capability")]
    private static partial void LogServerInfoUnanswered(ILogger logger, Uri address);

    [LoggerMessage(Level = LogLevel.Warning, Message = "Connecting to {Address}, which ${Variable} names: the options describing a private service are not used")]
    private static partial void LogPrivateOptionsUnused(ILogger logger, Uri address, string variable = ConnectionOptions.ServiceVariable);

    [LoggerMessage(Level = LogLevel.Warning, Message = "Calls to {Address} carry credentials over cleartext HTTP to another machine: they cross the network unencrypted unless the network itself is protected; reach the service over https to keep them private")]
    private static partial void LogCleartextCredentials(ILogger logger, Uri address);

    [LoggerMessage(Level = LogLevel.Debug, Message = "Closed the connection to {Address}")]
    private static partial void LogClosed(ILogger logger, Uri address);

    [LoggerMessage(Level = LogLevel.Warning, Message = "Closing the connection to {Address} failed")]
    private static partial void LogCloseFailed(ILogger logger, Uri address, Exception error);
}
