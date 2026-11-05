using System;
using System.Globalization;
using System.IO;
using System.Net;
using System.Net.Http;
using System.Net.Http.Headers;
using System.Text;
using System.Text.Json;
using System.Threading;
using System.Threading.Tasks;
using Microsoft.Extensions.Logging;

namespace OpenMBEE.OpenSysML.Internal;

/// <summary>
/// Where releases of <c>sysml-grpc</c> are fetched from and the limits on fetching them. <see cref="Production"/>
/// is GitHub over HTTPS with the digests this checkout pins; tests name a loopback server of their own.
/// </summary>
internal sealed record ReleaseSource
{
    /// <summary>GitHub, over HTTPS, with the digests embedded in this assembly.</summary>
    internal static ReleaseSource Production { get; } = new();

    /// <summary>Where release assets are served: <c>{Downloads}{owner}/{repo}/releases/download/{tag}/{asset}</c>.</summary>
    internal Uri Downloads { get; init; } = new("https://github.com/");

    /// <summary>Where the releases API answers: <c>{Api}repos/{owner}/{repo}/releases/latest</c>.</summary>
    internal Uri Api { get; init; } = new("https://api.github.com/");

    /// <summary>The digests a release asset must have; <see cref="ReleaseDigests.Embedded"/> when null.</summary>
    internal ReleaseDigests? Pins { get; init; }

    /// <summary>The asset to download; the one built for this machine when null.</summary>
    internal string? Asset { get; init; }

    /// <summary>How long a response may send nothing, its headers included: 15 seconds, as every client waits.</summary>
    internal TimeSpan IdleTimeout { get; init; } = TimeSpan.FromSeconds(15);

    /// <summary>The largest binary read: 512 MiB. A release binary is tens of megabytes.</summary>
    internal long MaxBinaryBytes { get; init; } = 512L * 1024 * 1024;

    /// <summary>The largest checksum or API answer read: 8 MiB. They are kilobytes.</summary>
    internal long MaxMetadataBytes { get; init; } = 8L * 1024 * 1024;

    /// <summary>The most redirects one fetch follows.</summary>
    internal int MaxRedirects { get; init; } = 10;

    /// <summary>Whether plain HTTP to a loopback host is fetched from: only a test's own server is served that way.</summary>
    internal bool LoopbackHttp { get; init; }
}

/// <summary>A release asset, and the digest it must have.</summary>
/// <param name="Repo">The repository, <c>owner/repo</c>.</param>
/// <param name="Tag">The release tag.</param>
/// <param name="Name">The asset, as <c>sysml-grpc-linux-amd64</c>.</param>
/// <param name="Url">Where it is downloaded from.</param>
/// <param name="Sha256">The digest it must have, in lower-case hexadecimal.</param>
/// <param name="Pinned">Whether that digest is one this checkout pins, rather than one served beside the asset.</param>
internal sealed record ReleaseAsset(string Repo, string Tag, string Name, Uri Url, string Sha256, bool Pinned);

/// <summary>
/// Fetches <c>sysml-grpc</c> releases: resolves <c>latest</c> through the releases API, decides the digest a
/// release must have, and streams it into the shared cache through a <see cref="CacheTransaction"/>, which
/// publishes nothing that does not have that digest.
/// </summary>
/// <remarks>
/// The digest is the one this checkout pins for the repository, tag and asset; the checksum the release serves
/// beside the binary must agree with it. A release nothing is pinned for is refused with
/// <see cref="UnpinnedReleaseException"/> before anything is fetched, since this client does not verify the
/// signature on a release's checksum manifest, unless <see cref="ConnectionOptions.AllowUnpinnedDownloadVariable"/>
/// grants same-origin trust to the repository, when the served checksum is used with a warning. Every response
/// is bounded in size, after decompression, and in silence: one that sends nothing for
/// <see cref="ReleaseSource.IdleTimeout"/> is abandoned. Every URL, redirects included, is HTTPS; plain HTTP is
/// fetched only from a loopback host of a source that allows it, and never after HTTPS.
/// </remarks>
internal sealed partial class BinaryDownloader : IDisposable
{
    private const int ChunkBytes = 64 * 1024;

    private readonly ReleaseSource source;
    private readonly ILogger logger;
    private readonly HttpClient http;

    /// <summary>Creates a downloader.</summary>
    /// <param name="source">Where releases come from.</param>
    /// <param name="logger">Where it logs.</param>
    /// <exception cref="ServiceStartException">The source names a URL that is not fetched from.</exception>
    internal BinaryDownloader(ReleaseSource source, ILogger logger)
    {
        this.source = source;
        this.logger = logger;
        Allow(source, source.Downloads, null);
        Allow(source, source.Api, null);
        var handler = new SocketsHttpHandler
        {
            AllowAutoRedirect = false,
            AutomaticDecompression = DecompressionMethods.All,
            ConnectTimeout = source.IdleTimeout,
            PlaintextStreamFilter = (context, _) => new ValueTask<Stream>(new IdleStream(context.PlaintextStream, source.IdleTimeout)),
        };
        http = new HttpClient(handler) { Timeout = Timeout.InfiniteTimeSpan };
        http.DefaultRequestHeaders.UserAgent.Add(new ProductInfoHeaderValue("OpenMBEE.OpenSysML", ReleaseRequest.ClientVersion));
    }

    /// <summary>The tag of a repository's latest published release, from the releases API.</summary>
    /// <param name="repo">The repository, <c>owner/repo</c>.</param>
    /// <param name="cancellationToken">Cancels the request.</param>
    /// <returns>The tag.</returns>
    /// <exception cref="ServiceStartException">The API could not be asked, or answered no tag.</exception>
    internal async Task<string> LatestAsync(string repo, CancellationToken cancellationToken)
    {
        var url = new Uri(source.Api, $"repos/{repo}/releases/latest");
        var body = await GetAsync(url, "application/vnd.github+json", source.MaxMetadataBytes, cancellationToken).ConfigureAwait(false);
        string? tag;
        try
        {
            using var document = JsonDocument.Parse(body);
            tag = document.RootElement.ValueKind == JsonValueKind.Object
                && document.RootElement.TryGetProperty("tag_name", out var name)
                && name.ValueKind == JsonValueKind.String
                ? name.GetString()
                : null;
        }
        catch (Exception error) when (error is JsonException or InvalidOperationException)
        {
            throw new ServiceStartException($"The latest release of {repo} could not be resolved: {url} answered something other than JSON ({error.Message}).", error);
        }

        if (tag is null || !ReleaseRequest.IsTag(tag))
        {
            throw new ServiceStartException(
                tag is null
                    ? $"The latest release of {repo} could not be resolved: {url} answered no tag_name."
                    : $"The latest release of {repo} could not be resolved: {url} answered the tag \"{tag}\", which is not one a URL can carry.");
        }

        LogLatest(logger, repo, tag);
        return tag;
    }

    /// <summary>The asset of a release for this machine, and the digest it must have.</summary>
    /// <param name="request">The release asked for.</param>
    /// <param name="tag">The release's tag, resolved (never <see cref="ReleaseRequest.Latest"/>).</param>
    /// <param name="required">The digest the caller requires of the binary; null for none.</param>
    /// <param name="cancellationToken">Cancels the request.</param>
    /// <returns>The asset.</returns>
    /// <exception cref="UnsupportedPlatformException">No release is built for this machine.</exception>
    /// <exception cref="UnpinnedReleaseException">Nothing is pinned for the asset and same-origin trust was not granted.</exception>
    /// <exception cref="ChecksumMismatchException">The checksum served contradicts the pin, or the digest the caller requires.</exception>
    /// <exception cref="ServiceStartException">The checksum could not be fetched or is not a digest.</exception>
    internal async Task<ReleaseAsset> VouchAsync(ReleaseRequest request, string tag, string? required, CancellationToken cancellationToken)
    {
        var name = source.Asset ?? ReleasePlatform.HostAsset();
        var url = AssetUrl(request.Repo, tag, name);
        var pinned = (source.Pins ?? ReleaseDigests.Embedded).Pinned(request.Repo, tag, name);
        if (pinned is null && !request.UnpinnedAllowed)
        {
            throw new UnpinnedReleaseException(
                request.Repo,
                tag,
                name,
                $"sysml-grpc {tag} of {request.Repo} was not downloaded: this client pins no SHA-256 digest for {name} of it, and it does not "
                + "verify the signature on a release's SHA256SUMS.txt, as the Python, Java and Node clients do, so the only digest left is "
                + "the one served beside the binary, which a compromised release would serve too.\n"
                + $"  fix: ask for a release this client pins with ConnectionOptions.DownloadVersion or ${ConnectionOptions.VersionVariable}, "
                + $"use a checkout of this client that pins {tag}, or accept same-origin trust for this repository by setting "
                + $"${ConnectionOptions.AllowUnpinnedDownloadVariable}={request.Repo} (or =1 for any repository).");
        }

        var served = await ServedAsync(request.Repo, tag, name, cancellationToken).ConfigureAwait(false);
        if (pinned is not null && served != pinned)
        {
            throw new ChecksumMismatchException(
                url.ToString(),
                pinned,
                served,
                $"{request.Repo} serves that digest beside {name} of {tag}, but this client pins {pinned}: the release was republished with "
                + "another binary, or the download is being tampered with. Nothing was downloaded and the cache was left as it was; "
                + "ask for a release that has not changed since this client pinned it");
        }

        if (pinned is null)
        {
            LogSameOrigin(logger, name, tag, request.Repo, ConnectionOptions.AllowUnpinnedDownloadVariable);
        }

        var expected = pinned ?? served;
        if (required is not null && !string.Equals(required, expected, StringComparison.OrdinalIgnoreCase))
        {
            throw new ChecksumMismatchException(
                url.ToString(),
                required.ToLowerInvariant(),
                expected,
                $"that is the digest {(pinned is null ? "served beside" : "this client pins for")} {name} of {tag} of {request.Repo}, "
                + "so nothing was downloaded and the cache was left as it was; require the digest of the release asked for, "
                + "or ask for the release that has the one required");
        }

        return new ReleaseAsset(request.Repo, tag, name, url, expected, pinned is not null);
    }

    /// <summary>Streams a release into the cache, which publishes it only once it has its digest.</summary>
    /// <param name="transaction">The cache, locked.</param>
    /// <param name="asset">The release.</param>
    /// <param name="cancellationToken">Cancels the download until the release is published.</param>
    /// <returns>What was installed.</returns>
    /// <exception cref="ChecksumMismatchException">What was received does not have the digest.</exception>
    /// <exception cref="ServiceStartException">The release could not be downloaded or installed.</exception>
    internal Task<CacheInstall> InstallAsync(CacheTransaction transaction, ReleaseAsset asset, CancellationToken cancellationToken)
    {
        LogDownloading(logger, asset.Tag, asset.Repo, asset.Url);
        return transaction.InstallAsync(
            (file, token) => FetchAsync(asset.Url, "*/*", file, source.MaxBinaryBytes, token),
            asset.Sha256,
            asset.Tag,
            asset.Repo,
            cancellationToken);
    }

    /// <summary>Closes the HTTP client.</summary>
    public void Dispose() => http.Dispose();

    private Uri AssetUrl(string repo, string tag, string name) => new(source.Downloads, $"{repo}/releases/download/{tag}/{name}");

    private async Task<string> ServedAsync(string repo, string tag, string name, CancellationToken cancellationToken)
    {
        var url = AssetUrl(repo, tag, name + ".sha256");
        var body = await GetAsync(url, "*/*", source.MaxMetadataBytes, cancellationToken).ConfigureAwait(false);
        string text;
        try
        {
            text = new UTF8Encoding(false, throwOnInvalidBytes: true).GetString(body);
        }
        catch (DecoderFallbackException)
        {
            text = string.Empty;
        }

        var digest = text.Split((char[]?)null, 2, StringSplitOptions.RemoveEmptyEntries) is [var first, ..] ? first : string.Empty;
        if (digest.Length != 64 || !CacheLayout.IsHex(digest))
        {
            throw new ServiceStartException($"The checksum {repo} serves for {name} of {tag} at {url} is not a SHA-256 digest; nothing was downloaded.");
        }

        return digest.ToLowerInvariant();
    }

    private async Task<byte[]> GetAsync(Uri url, string accept, long limit, CancellationToken cancellationToken)
    {
        using var body = new MemoryStream();
        await FetchAsync(url, accept, body, limit, cancellationToken).ConfigureAwait(false);
        return body.ToArray();
    }

    // The caller's cancellation is its own; anything else that ends a fetch is the release being unavailable.
    private async Task FetchAsync(Uri url, string accept, Stream sink, long limit, CancellationToken cancellationToken)
    {
        var current = url;
        try
        {
            for (var redirects = 0; ; redirects++)
            {
                using var request = new HttpRequestMessage(HttpMethod.Get, current);
                request.Headers.Accept.ParseAdd(accept);
                using var response = await http.SendAsync(request, HttpCompletionOption.ResponseHeadersRead, cancellationToken).ConfigureAwait(false);
                if (IsRedirect(response.StatusCode))
                {
                    current = Redirected(url, current, response.Headers.Location, redirects);
                    continue;
                }

                if (response.StatusCode != HttpStatusCode.OK)
                {
                    throw Unavailable(url, $"{Where(url, current)} answered HTTP {(int)response.StatusCode} ({response.ReasonPhrase})", null);
                }

                await ReadAsync(url, response.Content, sink, limit, cancellationToken).ConfigureAwait(false);
                return;
            }
        }
        catch (OperationCanceledException error) when (!cancellationToken.IsCancellationRequested)
        {
            throw TimedOut(url, current, error);
        }
        catch (OperationCanceledException error) when (error.CancellationToken != cancellationToken)
        {
            throw new OperationCanceledException(error.Message, error, cancellationToken);
        }
        catch (Exception error) when (error is HttpRequestException or IOException or TimeoutException)
        {
            for (Exception? cause = error; cause is not null; cause = cause.InnerException)
            {
                if (cause is TimeoutException)
                {
                    throw TimedOut(url, current, error);
                }
            }

            throw Unavailable(url, error.Message, error);
        }
    }

    private static async Task ReadAsync(Uri url, HttpContent content, Stream sink, long limit, CancellationToken cancellationToken)
    {
        if (content.Headers.ContentLength is long length && length > limit)
        {
            throw TooLarge(url, limit, $"says it is {length} bytes");
        }

        var body = await content.ReadAsStreamAsync(cancellationToken).ConfigureAwait(false);
        await using (body.ConfigureAwait(false))
        {
            var buffer = new byte[ChunkBytes];
            long total = 0;
            while (true)
            {
                var read = await body.ReadAsync(buffer, cancellationToken).ConfigureAwait(false);
                if (read == 0)
                {
                    return;
                }

                total += read;
                if (total > limit)
                {
                    throw TooLarge(url, limit, "sent more");
                }

                await sink.WriteAsync(buffer.AsMemory(0, read), cancellationToken).ConfigureAwait(false);
            }
        }
    }

    private Uri Redirected(Uri url, Uri from, Uri? location, int redirects)
    {
        if (location is null)
        {
            throw Unavailable(url, $"{Where(url, from)} redirected without saying where", null);
        }

        if (redirects == source.MaxRedirects)
        {
            throw Unavailable(url, $"it was redirected more than {source.MaxRedirects} times", null);
        }

        var next = location.IsAbsoluteUri ? location : new Uri(from, location);
        Allow(source, next, from);
        return next;
    }

    /// <summary>Refuses a URL a source does not fetch from: HTTPS always is, plain HTTP only on a loopback host of a source that allows it, and never after HTTPS.</summary>
    /// <param name="source">The source.</param>
    /// <param name="url">The URL.</param>
    /// <param name="from">The URL that redirected to it; null when it is not a redirect.</param>
    /// <exception cref="ServiceStartException">It is not fetched from.</exception>
    internal static void Allow(ReleaseSource source, Uri url, Uri? from)
    {
        if (url.Scheme == Uri.UriSchemeHttps)
        {
            return;
        }

        if (url.Scheme == Uri.UriSchemeHttp && source.LoopbackHttp && url.IsLoopback && (from is null || from.Scheme == Uri.UriSchemeHttp))
        {
            return;
        }

        throw new ServiceStartException(
            from is null
                ? $"Releases are fetched over HTTPS alone, so {url} is not fetched from."
                : from.Scheme == Uri.UriSchemeHttps && url.Scheme == Uri.UriSchemeHttp
                    ? $"{from} redirected to {url}, which would give up HTTPS; it was not followed."
                    : $"{from} redirected to {url}, which is not HTTPS; it was not followed.");
    }

    private static string Where(Uri url, Uri current) => current == url ? "it" : $"{current}, where it was redirected,";

    private static bool IsRedirect(HttpStatusCode status) =>
        status is HttpStatusCode.MovedPermanently or HttpStatusCode.Found or HttpStatusCode.SeeOther
            or HttpStatusCode.TemporaryRedirect or HttpStatusCode.PermanentRedirect;

    private static ServiceStartException TooLarge(Uri url, long limit, string how) =>
        Unavailable(url, $"it {how}, more than the {limit} bytes it may be, so it was not read further", null);

    private static ServiceStartException Unavailable(Uri url, string reason, Exception? cause) =>
        new($"{url} could not be downloaded: {reason}.", cause);

    private ServiceStartException TimedOut(Uri url, Uri current, Exception cause)
    {
        var seconds = source.IdleTimeout.TotalSeconds.ToString("0.###", CultureInfo.InvariantCulture);
        return Unavailable(url, $"nothing was received for {seconds} s", new TimeoutException($"{current} sent nothing for {seconds} s.", cause));
    }

    // Below HTTP parsing and decompression, each completed read is progress, even within partial headers.
    private sealed class IdleStream(Stream inner, TimeSpan timeout) : Stream
    {
        public override bool CanRead => inner.CanRead;

        public override bool CanSeek => false;

        public override bool CanWrite => inner.CanWrite;

        public override long Length => throw new NotSupportedException();

        public override long Position
        {
            get => throw new NotSupportedException();
            set => throw new NotSupportedException();
        }

        public override int Read(byte[] buffer, int offset, int count) =>
            ReadAsync(buffer.AsMemory(offset, count)).AsTask().GetAwaiter().GetResult();

        public override Task<int> ReadAsync(byte[] buffer, int offset, int count, CancellationToken cancellationToken) =>
            ReadAsync(buffer.AsMemory(offset, count), cancellationToken).AsTask();

        public override async ValueTask<int> ReadAsync(Memory<byte> buffer, CancellationToken cancellationToken = default)
        {
            using var idle = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken);
            idle.CancelAfter(timeout);
            try
            {
                return await inner.ReadAsync(buffer, idle.Token).ConfigureAwait(false);
            }
            catch (OperationCanceledException error) when (!cancellationToken.IsCancellationRequested)
            {
                throw new TimeoutException("No response bytes were received before the idle timeout.", error);
            }
        }

        public override void Write(byte[] buffer, int offset, int count) => inner.Write(buffer, offset, count);

        public override Task WriteAsync(byte[] buffer, int offset, int count, CancellationToken cancellationToken) =>
            inner.WriteAsync(buffer, offset, count, cancellationToken);

        public override ValueTask WriteAsync(ReadOnlyMemory<byte> buffer, CancellationToken cancellationToken = default) =>
            inner.WriteAsync(buffer, cancellationToken);

        public override void Flush() => inner.Flush();

        public override Task FlushAsync(CancellationToken cancellationToken) => inner.FlushAsync(cancellationToken);

        public override long Seek(long offset, SeekOrigin origin) => throw new NotSupportedException();

        public override void SetLength(long value) => throw new NotSupportedException();

        protected override void Dispose(bool disposing)
        {
            if (disposing)
            {
                inner.Dispose();
            }

            base.Dispose(disposing);
        }

    }

    [LoggerMessage(Level = LogLevel.Debug, Message = "The latest release of {Repo} is {Tag}")]
    private static partial void LogLatest(ILogger logger, string repo, string tag);

    [LoggerMessage(Level = LogLevel.Information, Message = "Downloading sysml-grpc {Tag} of {Repo} from {Url}")]
    private static partial void LogDownloading(ILogger logger, string tag, string repo, Uri url);

    [LoggerMessage(Level = LogLevel.Warning, Message = "This client pins no digest for {Asset} of {Tag} of {Repo}; verifying it against the checksum served beside it, which detects corruption but not a compromised release (${Variable} grants that trust)")]
    private static partial void LogSameOrigin(ILogger logger, string asset, string tag, string repo, string variable);
}
