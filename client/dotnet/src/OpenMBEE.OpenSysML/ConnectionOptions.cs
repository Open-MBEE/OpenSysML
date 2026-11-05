using System;
using System.Collections.Frozen;
using System.Collections.Generic;
using System.Linq;
using System.Net.Http;
using Microsoft.Extensions.Logging;
using Microsoft.Extensions.Logging.Abstractions;
using OpenMBEE.OpenSysML.Internal;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// How a <see cref="Connection"/> reaches a service and calls it. Collections are copied when they
/// are set, so changing the collection afterwards changes nothing here. Values are checked when a
/// connection opens, which refuses an invalid one with <see cref="InvalidRequestException"/>
/// before it does any work.
/// </summary>
public sealed class ConnectionOptions
{
    /// <summary>The environment variable naming the service to connect to when <see cref="Address"/> does not.</summary>
    public const string ServiceVariable = "OPENSYSML_SERVICE";

    /// <summary>
    /// The environment variable naming the <c>sysml-grpc</c> binary to start as a private service when
    /// <see cref="BinaryPath"/> does not.
    /// </summary>
    public const string BinaryVariable = "OPENSYSML_GRPC_BINARY";

    /// <summary>
    /// The environment variable naming the release to download for a private service when
    /// <see cref="DownloadVersion"/> does not: a tag, as <c>v0.10.1</c>, or <c>latest</c>.
    /// </summary>
    public const string VersionVariable = "OPENSYSML_GRPC_VERSION";

    /// <summary>
    /// The environment variable naming the repository, <c>owner/repo</c>, releases are downloaded from when
    /// <see cref="GithubRepo"/> does not.
    /// </summary>
    public const string GithubRepoVariable = "OPENSYSML_GITHUB_REPO";

    /// <summary>
    /// The environment variable that accepts a release this client pins no digest for on the strength of the
    /// checksum served beside it: <c>1</c> for any repository, or a comma-separated list of the <c>owner/repo</c>
    /// repositories it is granted for. That is same-origin trust, which detects corruption but not a compromised
    /// release; a warning says so whenever it is relied on.
    /// </summary>
    public const string AllowUnpinnedDownloadVariable = "OPENSYSML_ALLOW_UNPINNED_DOWNLOAD";

    /// <summary>
    /// The environment variable naming the bearer token every call carries when <see cref="Token"/> does not, and
    /// <see cref="Headers"/> holds no <c>authorization</c> of its own. A blank value is no token. The service reads
    /// the same variable to require one, so a private service is started without it.
    /// </summary>
    public const string TokenVariable = "OPENSYSML_GRPC_TOKEN";

    /// <summary>The repository releases are downloaded from unless another is named: <c>Open-MBEE/OpenSysML</c>.</summary>
    public const string DefaultGithubRepo = "Open-MBEE/OpenSysML";

    /// <summary>The default largest request and response message, in bytes: 512 MiB.</summary>
    public const int DefaultMaxMessageSize = 512 * 1024 * 1024;

    private IReadOnlyDictionary<string, string> headers = FrozenDictionary<string, string>.Empty;
    private IReadOnlyList<string> requireCapabilities = [];
    private ILogger logger = NullLogger.Instance;

    /// <summary>The default options: the service <see cref="ServiceVariable"/> names, else a private one, over gRPC.</summary>
    public static ConnectionOptions Default { get; } = new();

    /// <summary>The default time a call may take: 60 seconds.</summary>
    public static TimeSpan DefaultRequestTimeout => TimeSpan.FromSeconds(60);

    /// <summary>The longest <see cref="RequestTimeout"/> a connection accepts: <see cref="int.MaxValue"/> milliseconds.</summary>
    public static TimeSpan MaxRequestTimeout => TimeSpan.FromMilliseconds(int.MaxValue);

    /// <summary>The default time a private service has to report its address: 30 seconds.</summary>
    public static TimeSpan DefaultStartupTimeout => TimeSpan.FromSeconds(30);

    /// <summary>The default time a private service has to exit once asked to: 5 seconds.</summary>
    public static TimeSpan DefaultShutdownTimeout => TimeSpan.FromSeconds(5);

    /// <summary>
    /// The service to connect to: <c>host:port</c>, which means <c>http://host:port</c>, or an
    /// <c>http</c> or <c>https</c> URL with no path, query, fragment or user information. A bracketed
    /// IPv6 host, as <c>[::1]:50051</c>, is accepted. When null, <see cref="ServiceVariable"/> names
    /// it, and when that is unset too, a private service is started (<see cref="AutoStart"/>). A named
    /// service is never started, stopped or downloaded by this client.
    /// </summary>
    public string? Address { get; init; }

    /// <summary>
    /// The protocol to speak; <see cref="Protocol.Grpc"/> by default. The connection speaks it alone:
    /// a call that fails is never retried over another.
    /// </summary>
    public Protocol Protocol { get; init; } = Protocol.Grpc;

    /// <summary>
    /// How messages are encoded; <see cref="MessageEncoding.Protobuf"/> by default.
    /// <see cref="MessageEncoding.Json"/> needs <see cref="Protocol.Connect"/>: gRPC carries protobuf
    /// alone, so asking for JSON over it fails the open before anything is sent.
    /// </summary>
    public MessageEncoding Encoding { get; init; } = MessageEncoding.Protobuf;

    /// <summary>
    /// Whether to start a private service when none is named; true by default. A private service is a
    /// <c>sysml-grpc</c> child of this process, on a port the kernel chooses, which only this process
    /// can reach and which exits when this process does, however it ends. Connections share one
    /// while they are open (see <see cref="IsolatedService"/>), and the last to close stops it. When
    /// false, a connection with no service named fails with <see cref="ServiceStartException"/>.
    /// </summary>
    public bool AutoStart { get; init; } = true;

    /// <summary>
    /// The <c>sysml-grpc</c> binary to start as a private service, or null to look for one: the binary
    /// <see cref="BinaryVariable"/> names, else the release asked for in the shared cache
    /// <c>~/.opensysml/bin</c>, downloaded and verified there when it is not already (see
    /// <see cref="DownloadVersion"/>), else the first <c>sysml-grpc</c> on <c>PATH</c> when the release this
    /// client was built with cannot be had. A binary named here or by <see cref="BinaryVariable"/> that is not
    /// an executable file fails the open with <see cref="BinaryNotFoundException"/> rather than another being
    /// looked for, and is never downloaded. Only for a private service: refused with <see cref="Address"/> set
    /// or <see cref="AutoStart"/> false.
    /// </summary>
    public string? BinaryPath { get; init; }

    /// <summary>
    /// The SHA-256 digest, as 64 hexadecimal digits in either case, the private service's binary
    /// must have, or null to require none. A binary with another fails the open with
    /// <see cref="ChecksumMismatchException"/> before it is started, and a release whose pinned or served
    /// digest is another is not downloaded. A cached binary that has it is started without looking for a
    /// release only when no release was explicitly requested; <see cref="DownloadVersion"/> and
    /// <see cref="VersionVariable"/> remain constraints, and any committed pin for a recorded cache release
    /// is enforced too. Only for a private service: refused with <see cref="Address"/> set or
    /// <see cref="AutoStart"/> false.
    /// </summary>
    public string? ExpectedBinarySha256 { get; init; }

    /// <summary>
    /// The release of <c>sysml-grpc</c> a private service is started from when no binary is named: a tag, as
    /// <c>v0.10.1</c>, or <c>latest</c>, which the GitHub releases API resolves once per open. When null,
    /// <see cref="VersionVariable"/> names it, and when that is unset too, it is the release this client was built
    /// with. The release is kept in the shared cache <c>~/.opensysml/bin</c>, downloaded there and verified when
    /// the cache does not already hold it. A release asked for here or by <see cref="VersionVariable"/> is that
    /// release or a failure; the release this client was built with gives way to the cache, or to a
    /// <c>sysml-grpc</c> on <c>PATH</c>, when it cannot be had. Only for a private service whose binary is looked
    /// for: refused with <see cref="Address"/> or <see cref="BinaryPath"/> set or <see cref="AutoStart"/> false.
    /// </summary>
    public string? DownloadVersion { get; init; }

    /// <summary>
    /// The repository, <c>owner/repo</c>, releases are downloaded from, or null for the one
    /// <see cref="GithubRepoVariable"/> names, else <see cref="DefaultGithubRepo"/>. A cached release of another
    /// repository is not taken for one of this. Only for a private service whose binary is looked for: refused
    /// with <see cref="Address"/> or <see cref="BinaryPath"/> set or <see cref="AutoStart"/> false.
    /// </summary>
    public string? GithubRepo { get; init; }

    /// <summary>
    /// Whether the private service is this connection's alone, rather than shared with the other
    /// connections of this copy of the client that would start the same one; false by default. An
    /// isolated service is stopped when its connection closes, and never by
    /// <see cref="Connection.StopSharedServicesAsync"/>. Only for a private service: refused with
    /// <see cref="Address"/> set or <see cref="AutoStart"/> false.
    /// </summary>
    public bool IsolatedService { get; init; }

    /// <summary>
    /// How long a private service has to report the address it listens on once started:
    /// <see cref="DefaultStartupTimeout"/> by default. Positive and at most <see cref="MaxRequestTimeout"/>.
    /// </summary>
    public TimeSpan StartupTimeout { get; init; } = DefaultStartupTimeout;

    /// <summary>
    /// How long a private service has to exit once its last connection closes its input, after which
    /// it and any process it started are killed: <see cref="DefaultShutdownTimeout"/> by default.
    /// Positive and at most <see cref="MaxRequestTimeout"/>.
    /// </summary>
    public TimeSpan ShutdownTimeout { get; init; } = DefaultShutdownTimeout;

    /// <summary>
    /// How long a call may take, from when it is made: <see cref="DefaultRequestTimeout"/> by default.
    /// Positive and at most <see cref="MaxRequestTimeout"/>.
    /// </summary>
    public TimeSpan RequestTimeout { get; init; } = DefaultRequestTimeout;

    /// <summary>
    /// Headers sent with every call, as authorization or other custom metadata. Names are matched
    /// without regard to case and are sent in lower case. A name is ASCII letters, digits, <c>-</c>,
    /// <c>_</c> and <c>.</c>, and a value printable ASCII. A header the protocol controls (content
    /// type and encoding, timeouts, the host, and any <c>grpc-</c> or <c>connect-</c> header) and a
    /// binary <c>-bin</c> header are refused. Values are never logged. An <c>authorization</c> header
    /// here takes the place of <see cref="TokenVariable"/>, and is refused alongside <see cref="Token"/>.
    /// </summary>
    public IReadOnlyDictionary<string, string> Headers
    {
        get => headers;
        init
        {
            ArgumentNullException.ThrowIfNull(value);
            if (value.Any(header => header.Key is null || header.Value is null))
            {
                throw new ArgumentException("A header name or value is null.", nameof(value));
            }

            headers = value.ToFrozenDictionary(StringComparer.Ordinal);
        }
    }

    /// <summary>
    /// The bearer token every call carries, as <c>authorization: Bearer</c>, for a service that requires one: one
    /// started with <c>-token</c>, <c>-token-file</c> or <see cref="TokenVariable"/>, as a service other machines
    /// reach is. When null, <see cref="TokenVariable"/> names it, unless <see cref="Headers"/> holds an
    /// <c>authorization</c> of its own. A token is letters, digits and <c>-._~+/</c> with trailing <c>=</c> only
    /// (RFC 6750 <c>b64token</c>), which is all the service accepts. A private service requires none, and ignores
    /// one sent. Sent over cleartext <c>http</c> to another machine, it crosses the network unencrypted, which is
    /// logged as a warning. It is never logged and never written into a message.
    /// </summary>
    public string? Token { get; init; }

    /// <summary>
    /// Capabilities the service must advertise, checked once the connection has asked it what it
    /// is; a missing one fails the open with <see cref="CapabilityException"/>. A repeated name counts once.
    /// </summary>
    public IReadOnlyList<string> RequireCapabilities
    {
        get => requireCapabilities;
        init
        {
            ArgumentNullException.ThrowIfNull(value);
            if (value.Contains(null!))
            {
                throw new ArgumentException("A required capability is null.", nameof(value));
            }

            requireCapabilities = [.. value.Distinct(StringComparer.Ordinal).Order(StringComparer.Ordinal)];
        }
    }

    /// <summary>
    /// The exact build version the service must report, or null to accept any. A service that
    /// reports another, or does not answer, fails the open with <see cref="StaleServiceException"/>.
    /// </summary>
    public string? Version { get; init; }

    /// <summary>Where the connection logs; <see cref="NullLogger.Instance"/> by default.</summary>
    public ILogger Logger
    {
        get => logger;
        init => logger = value ?? throw new ArgumentNullException(nameof(value));
    }

    /// <summary>
    /// The HTTP handler to send through, or null for one the connection creates and disposes. A
    /// handler given here is borrowed: the connection never disposes it, so one handler can serve
    /// several connections and outlive them.
    /// </summary>
    public HttpMessageHandler? HttpMessageHandler { get; init; }

    /// <summary>
    /// The largest request message to send, in bytes as <see cref="Encoding"/> encodes it; positive,
    /// <see cref="DefaultMaxMessageSize"/> by default. A larger request fails without being sent.
    /// </summary>
    public int MaxRequestSize { get; init; } = DefaultMaxMessageSize;

    /// <summary>
    /// The largest response message to accept, in bytes as <see cref="Encoding"/> encodes it and
    /// after any decompression; positive, <see cref="DefaultMaxMessageSize"/> by default.
    /// </summary>
    public int MaxResponseSize { get; init; } = DefaultMaxMessageSize;

    /// <summary>Refuses options no connection can use.</summary>
    /// <exception cref="InvalidRequestException">An option is invalid.</exception>
    internal void Validate()
    {
        if (!Enum.IsDefined(Protocol))
        {
            throw new InvalidRequestException($"ConnectionOptions.Protocol is {(int)Protocol}, which is no protocol of this client.");
        }

        if (!Enum.IsDefined(Encoding))
        {
            throw new InvalidRequestException($"ConnectionOptions.Encoding is {(int)Encoding}, which is no encoding of this client.");
        }

        if (Protocol == Protocol.Grpc && Encoding != MessageEncoding.Protobuf)
        {
            throw new InvalidRequestException(
                $"ConnectionOptions.Encoding is {Encoding}, which gRPC does not carry: use Protocol.Connect for {Encoding}, or MessageEncoding.Protobuf over gRPC.");
        }

        if (RequestTimeout <= TimeSpan.Zero || RequestTimeout > MaxRequestTimeout)
        {
            throw new InvalidRequestException(
                $"ConnectionOptions.RequestTimeout is {RequestTimeout}; it must be positive and at most {MaxRequestTimeout}.");
        }

        if (StartupTimeout <= TimeSpan.Zero || StartupTimeout > MaxRequestTimeout)
        {
            throw new InvalidRequestException(
                $"ConnectionOptions.StartupTimeout is {StartupTimeout}; it must be positive and at most {MaxRequestTimeout}.");
        }

        if (ShutdownTimeout <= TimeSpan.Zero || ShutdownTimeout > MaxRequestTimeout)
        {
            throw new InvalidRequestException(
                $"ConnectionOptions.ShutdownTimeout is {ShutdownTimeout}; it must be positive and at most {MaxRequestTimeout}.");
        }

        if (MaxRequestSize <= 0)
        {
            throw new InvalidRequestException($"ConnectionOptions.MaxRequestSize is {MaxRequestSize}; it must be positive.");
        }

        if (MaxResponseSize <= 0)
        {
            throw new InvalidRequestException($"ConnectionOptions.MaxResponseSize is {MaxResponseSize}; it must be positive.");
        }

        if (Version is not null && string.IsNullOrWhiteSpace(Version))
        {
            throw new InvalidRequestException("ConnectionOptions.Version is blank; name a version, or leave it null to accept any.");
        }

        if (RequireCapabilities.FirstOrDefault(string.IsNullOrWhiteSpace) is not null)
        {
            throw new InvalidRequestException("ConnectionOptions.RequireCapabilities holds a blank capability name.");
        }

        RequestHeaders.Validate(Headers);
        ValidateToken();
        ValidatePrivateService();
    }

    private void ValidateToken()
    {
        if (Token is null)
        {
            return;
        }

        if (string.IsNullOrWhiteSpace(Token))
        {
            throw new InvalidRequestException(
                $"ConnectionOptions.Token is blank; give the service's bearer token, or leave it null for the one ${TokenVariable} names.");
        }

        if (!BearerToken.IsValid(Token))
        {
            throw new InvalidRequestException($"ConnectionOptions.Token is not a bearer token: {BearerToken.Rule}.");
        }

        if (Headers.Keys.Any(name => string.Equals(name, BearerToken.Header, StringComparison.OrdinalIgnoreCase)))
        {
            throw new InvalidRequestException(
                "ConnectionOptions.Token and the authorization header in ConnectionOptions.Headers both give the calls' credentials; set one or the other.");
        }
    }

    private void ValidatePrivateService()
    {
        if (BinaryPath is not null && string.IsNullOrWhiteSpace(BinaryPath))
        {
            throw new InvalidRequestException("ConnectionOptions.BinaryPath is blank; name a sysml-grpc binary, or leave it null to look for one.");
        }

        if (ExpectedBinarySha256 is not null && !IsSha256(ExpectedBinarySha256))
        {
            throw new InvalidRequestException(
                "ConnectionOptions.ExpectedBinarySha256 is not a SHA-256 digest: it must be 64 hexadecimal digits.");
        }

        if (DownloadVersion is not null && !ReleaseRequest.IsVersion(DownloadVersion))
        {
            throw new InvalidRequestException(
                $"ConnectionOptions.DownloadVersion is \"{DownloadVersion}\", which is neither \"{ReleaseRequest.Latest}\" nor a release tag "
                + "of letters, digits, '.', '-', '_' and '+'; leave it null for the release this client was built with.");
        }

        if (GithubRepo is not null && !ReleaseRequest.IsRepo(GithubRepo))
        {
            throw new InvalidRequestException(
                $"ConnectionOptions.GithubRepo is \"{GithubRepo}\", which is not an owner/repo of letters, digits, '.', '-' and '_'; leave it null for {DefaultGithubRepo}.");
        }

        var download = DownloadVersion is not null ? nameof(DownloadVersion) : GithubRepo is not null ? nameof(GithubRepo) : null;
        if (download is not null && BinaryPath is not null)
        {
            throw new InvalidRequestException(
                $"ConnectionOptions.{download} describes a release to download, but ConnectionOptions.BinaryPath names the binary to start, which is never downloaded; set one or the other.");
        }

        var named = BinaryPath is not null ? nameof(BinaryPath)
            : ExpectedBinarySha256 is not null ? nameof(ExpectedBinarySha256)
            : IsolatedService ? nameof(IsolatedService)
            : download;
        if (named is null)
        {
            return;
        }

        if (Address is not null)
        {
            throw new InvalidRequestException(
                $"ConnectionOptions.{named} describes a private service, but ConnectionOptions.Address names a running one; set one or the other.");
        }

        if (!AutoStart)
        {
            throw new InvalidRequestException(
                $"ConnectionOptions.{named} describes a private service, but ConnectionOptions.AutoStart is false, so none is started.");
        }
    }

    private static bool IsSha256(string digest) => digest.Length == 64 && digest.All(char.IsAsciiHexDigit);
}
