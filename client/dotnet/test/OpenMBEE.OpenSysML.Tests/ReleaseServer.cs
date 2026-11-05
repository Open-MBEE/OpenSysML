using System;
using System.Collections.Concurrent;
using System.Collections.Generic;
using System.IO;
using System.IO.Compression;
using System.Linq;
using System.Net;
using System.Net.Sockets;
using System.Text;
using System.Text.Json;
using System.Threading;
using System.Threading.Tasks;
using OpenMBEE.OpenSysML.Internal;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// A GitHub release host and releases API on a loopback port the kernel chooses, so no test reaches the network.
/// It speaks just enough HTTP/1.1 over a socket of its own to misbehave on purpose: a body that stalls, never
/// ends, claims a length it does not have, arrives compressed, or redirects elsewhere. Every connection is closed
/// after one answer unless an answer keeps it alive, and disposing it closes the ones still open.
/// </summary>
internal sealed class ReleaseServer : IAsyncDisposable
{
    /// <summary>The repository the fixtures publish under unless a test names another.</summary>
    internal const string Repo = ConnectionOptions.DefaultGithubRepo;

    /// <summary>The asset every test downloads, whatever platform runs it.</summary>
    internal const string Asset = "sysml-grpc-linux-amd64";

    private readonly TcpListener listener;
    private readonly CancellationTokenSource stopping = new();
    private readonly ConcurrentDictionary<string, Answer> answers = new(StringComparer.Ordinal);
    private readonly ConcurrentQueue<string> requested = new();
    private readonly ConcurrentQueue<(string Path, string? UserAgent, string? Accept)> headers = new();
    private readonly ConcurrentDictionary<int, TcpClient> open = new();
    private readonly ConcurrentBag<Task> serving = [];
    private readonly Task accepting;
    private int nextConnection;

    internal ReleaseServer()
    {
        listener = new TcpListener(IPAddress.Loopback, 0);
        listener.Start();
        BaseUrl = new Uri($"http://127.0.0.1:{((IPEndPoint)listener.LocalEndpoint).Port}/");
        accepting = Task.Run(AcceptAsync);
    }

    /// <summary>The server's root, which is both the download host and the API.</summary>
    internal Uri BaseUrl { get; }

    /// <summary>Every path asked for, in order.</summary>
    internal IReadOnlyList<string> Requested => [.. requested];

    /// <summary>Every request's path, User-Agent and Accept headers.</summary>
    internal IReadOnlyList<(string Path, string? UserAgent, string? Accept)> Headers => [.. headers];

    /// <summary>How many TCP connections have been accepted.</summary>
    internal int Connections => Volatile.Read(ref nextConnection);

    /// <summary>The path a release asset is served at.</summary>
    internal static string AssetPath(string tag, string asset = Asset, string repo = Repo) => $"/{repo}/releases/download/{tag}/{asset}";

    /// <summary>A source of releases from this server, pinning what <paramref name="pins"/> pins.</summary>
    internal ReleaseSource Source(ReleaseDigests? pins = null, TimeSpan? idle = null, long? maxBinary = null, long? maxMetadata = null) =>
        new()
        {
            Downloads = BaseUrl,
            Api = BaseUrl,
            Pins = pins ?? Pins(),
            Asset = Asset,
            LoopbackHttp = true,
            IdleTimeout = idle ?? TimeSpan.FromSeconds(15),
            MaxBinaryBytes = maxBinary ?? 512L * 1024 * 1024,
            MaxMetadataBytes = maxMetadata ?? 8L * 1024 * 1024,
        };

    /// <summary>A source whose every request is refused: a loopback port nothing listens on.</summary>
    internal static ReleaseSource Offline(ReleaseDigests? pins = null)
    {
        var closed = new TcpListener(IPAddress.Loopback, 0);
        closed.Start();
        var port = ((IPEndPoint)closed.LocalEndpoint).Port;
        closed.Stop();
        var url = new Uri($"http://127.0.0.1:{port}/");
        return new ReleaseSource { Downloads = url, Api = url, Pins = pins ?? Pins(), Asset = Asset, LoopbackHttp = true };
    }

    /// <summary>A pin table, from (repo, tag, asset, digest) rows.</summary>
    internal static ReleaseDigests Pins(params (string Repo, string Tag, string Asset, string Sha256)[] rows)
    {
        var table = new SortedDictionary<string, SortedDictionary<string, SortedDictionary<string, string>>>(StringComparer.Ordinal);
        foreach (var (repo, tag, asset, sha256) in rows)
        {
            if (!table.TryGetValue(repo, out var releases))
            {
                table[repo] = releases = new(StringComparer.Ordinal);
            }

            if (!releases.TryGetValue(tag, out var assets))
            {
                releases[tag] = assets = new(StringComparer.Ordinal);
            }

            assets[asset] = sha256;
        }

        return ReleaseDigests.Parse(JsonSerializer.SerializeToUtf8Bytes(table));
    }

    /// <summary>A pin table pinning bytes as the asset of a release.</summary>
    internal static ReleaseDigests Pinning(string tag, byte[] bytes, string repo = Repo) => Pins((repo, tag, Asset, TestCache.Sha256(bytes)));

    /// <summary>Publishes an asset, and the <c>.sha256</c> a release serves beside it.</summary>
    internal ReleaseServer Publish(string tag, byte[] bytes, string repo = Repo, string asset = Asset)
    {
        Serve(AssetPath(tag, asset, repo), new Answer { Body = bytes });
        return Served(tag, TestCache.Sha256(bytes), repo, asset);
    }

    /// <summary>Serves a <c>.sha256</c> naming a digest, whatever the asset is.</summary>
    internal ReleaseServer Served(string tag, string sha256, string repo = Repo, string asset = Asset)
    {
        Serve(AssetPath(tag, asset + ".sha256", repo), new Answer { Body = Encoding.ASCII.GetBytes($"{sha256}  {asset}\n") });
        return this;
    }

    /// <summary>Answers the releases API's latest release of a repository with a tag.</summary>
    internal ReleaseServer Latest(string tag, string repo = Repo)
    {
        Serve($"/repos/{repo}/releases/latest", new Answer { Body = Encoding.UTF8.GetBytes($"{{\"tag_name\": \"{tag}\", \"name\": \"{tag}\"}}") });
        return this;
    }

    /// <summary>Answers a path as described.</summary>
    internal ReleaseServer Serve(string path, Answer answer)
    {
        answers[path] = answer;
        return this;
    }

    /// <summary>Gzips bytes, for an answer that arrives compressed.</summary>
    internal static byte[] Gzip(byte[] bytes)
    {
        using var compressed = new MemoryStream();
        using (var gzip = new GZipStream(compressed, CompressionLevel.SmallestSize, leaveOpen: true))
        {
            gzip.Write(bytes);
        }

        return compressed.ToArray();
    }

    public async ValueTask DisposeAsync()
    {
        await stopping.CancelAsync();
        listener.Stop();
        foreach (var client in open.Values)
        {
            client.Dispose();
        }

        await accepting.ConfigureAwait(false);
        await Task.WhenAll(serving).WaitAsync(TimeSpan.FromSeconds(10)).ConfigureAwait(false);
        stopping.Dispose();
    }

    private async Task AcceptAsync()
    {
        while (!stopping.IsCancellationRequested)
        {
            TcpClient client;
            try
            {
                client = await listener.AcceptTcpClientAsync(stopping.Token).ConfigureAwait(false);
            }
            catch (Exception error) when (error is OperationCanceledException or SocketException or ObjectDisposedException)
            {
                return;
            }

            var id = Interlocked.Increment(ref nextConnection);
            open[id] = client;
            serving.Add(Task.Run(async () =>
            {
                try
                {
                    await AnswerAsync(client).ConfigureAwait(false);
                }
                catch (Exception error) when (error is IOException or SocketException or ObjectDisposedException or OperationCanceledException)
                {
                    // The client gave up on what it was being sent, which is what several answers are for.
                }
                finally
                {
                    open.TryRemove(id, out _);
                    client.Dispose();
                }
            }));
        }
    }

    private async Task AnswerAsync(TcpClient client)
    {
        while (await AnswerOnceAsync(client).ConfigureAwait(false))
        {
        }
    }

    private async Task<bool> AnswerOnceAsync(TcpClient client)
    {
        var stream = client.GetStream();
        var request = await ReadRequestAsync(stream).ConfigureAwait(false);
        if (request is null)
        {
            return false;
        }

        var (path, requestHeaders) = request.Value;
        requested.Enqueue(path);
        headers.Enqueue((path, requestHeaders.GetValueOrDefault("user-agent"), requestHeaders.GetValueOrDefault("accept")));
        var answer = answers.GetValueOrDefault(path) ?? new Answer { Status = 404, Body = Encoding.ASCII.GetBytes("Not Found") };
        if (answer.Silent)
        {
            await Task.Delay(Timeout.Infinite, stopping.Token).ConfigureAwait(false);
        }

        var head = new StringBuilder();
        head.Append(CultureInvariant($"HTTP/1.1 {answer.Status} {Reason(answer.Status)}\r\nConnection: {(answer.KeepAlive ? "keep-alive" : "close")}\r\n"));
        if (answer.Location is not null)
        {
            head.Append(CultureInvariant($"Location: {answer.Location}\r\n"));
        }

        if (answer.ContentEncoding is not null)
        {
            head.Append(CultureInvariant($"Content-Encoding: {answer.ContentEncoding}\r\n"));
        }

        if (!answer.Endless && !answer.OmitLength)
        {
            head.Append(CultureInvariant($"Content-Length: {answer.ClaimedLength ?? answer.Body.Length}\r\n"));
        }

        head.Append("\r\n");
        var header = Encoding.ASCII.GetBytes(head.ToString()).AsMemory();
        if (answer.HeaderStallAfter is { } headerStall)
        {
            header = header[..Math.Min(headerStall, header.Length)];
        }

        await WriteChunksAsync(stream, header, answer.HeaderChunkBytes, answer.HeaderChunkDelay, stopping.Token).ConfigureAwait(false);
        if (answer.HeaderStallAfter is not null)
        {
            await Task.Delay(Timeout.Infinite, stopping.Token).ConfigureAwait(false);
        }

        if (answer.Endless)
        {
            var chunk = new byte[64 * 1024];
            while (true)
            {
                await stream.WriteAsync(chunk, stopping.Token).ConfigureAwait(false);
            }
        }

        var sent = answer.StallAfter is { } stallAfter ? answer.Body.AsMemory(0, Math.Min(stallAfter, answer.Body.Length)) : answer.Body;
        await WriteChunksAsync(stream, sent, answer.ChunkBytes, answer.ChunkDelay, stopping.Token).ConfigureAwait(false);
        if (answer.StallAfter is not null)
        {
            await Task.Delay(Timeout.Infinite, stopping.Token).ConfigureAwait(false);
        }

        if (!answer.KeepAlive)
        {
            client.Client.Shutdown(SocketShutdown.Send);
        }

        return answer.KeepAlive;
    }

    private static async Task WriteChunksAsync(NetworkStream stream, ReadOnlyMemory<byte> bytes, int chunkBytes, TimeSpan? delay, CancellationToken cancellationToken)
    {
        if (delay is null)
        {
            await stream.WriteAsync(bytes, cancellationToken).ConfigureAwait(false);
            await stream.FlushAsync(cancellationToken).ConfigureAwait(false);
            return;
        }

        for (var offset = 0; offset < bytes.Length; offset += chunkBytes)
        {
            await stream.WriteAsync(bytes[offset..Math.Min(offset + chunkBytes, bytes.Length)], cancellationToken).ConfigureAwait(false);
            await stream.FlushAsync(cancellationToken).ConfigureAwait(false);
            await Task.Delay(delay.Value, cancellationToken).ConfigureAwait(false);
        }
    }

    private async Task<(string Path, Dictionary<string, string> Headers)?> ReadRequestAsync(NetworkStream stream)
    {
        var bytes = new List<byte>();
        var buffer = new byte[1];
        while (bytes.Count < 64 * 1024)
        {
            if (await stream.ReadAsync(buffer, stopping.Token).ConfigureAwait(false) == 0)
            {
                return null;
            }

            bytes.Add(buffer[0]);
            if (bytes.Count >= 4 && bytes[^4] == '\r' && bytes[^3] == '\n' && bytes[^2] == '\r' && bytes[^1] == '\n')
            {
                break;
            }
        }

        var lines = Encoding.ASCII.GetString([.. bytes]).Split("\r\n", StringSplitOptions.RemoveEmptyEntries);
        var target = lines[0].Split(' ')[1];
        var fields = new Dictionary<string, string>(StringComparer.OrdinalIgnoreCase);
        foreach (var line in lines.Skip(1))
        {
            var colon = line.IndexOf(':', StringComparison.Ordinal);
            fields[line[..colon].Trim().ToLowerInvariant()] = line[(colon + 1)..].Trim();
        }

        return (Uri.UnescapeDataString(target.Split('?')[0]), fields);
    }

    private static string CultureInvariant(FormattableString text) => FormattableString.Invariant(text);

    private static string Reason(int status) => status switch
    {
        200 => "OK",
        301 => "Moved Permanently",
        302 => "Found",
        307 => "Temporary Redirect",
        404 => "Not Found",
        500 => "Internal Server Error",
        _ => "Status",
    };

    /// <summary>How a path is answered.</summary>
    internal sealed record Answer
    {
        /// <summary>The HTTP status.</summary>
        internal int Status { get; init; } = 200;

        /// <summary>The body, as sent, compressed or not.</summary>
        internal byte[] Body { get; init; } = [];

        /// <summary>The Location header of a redirect.</summary>
        internal string? Location { get; init; }

        /// <summary>The Content-Encoding the body is sent with.</summary>
        internal string? ContentEncoding { get; init; }

        /// <summary>A Content-Length other than the body's.</summary>
        internal long? ClaimedLength { get; init; }

        /// <summary>Whether no Content-Length is sent, so the body ends when the connection does.</summary>
        internal bool OmitLength { get; init; }

        /// <summary>When set, this many bytes of the body are sent and then nothing, until the server stops.</summary>
        internal int? StallAfter { get; init; }

        /// <summary>Whether the body never ends.</summary>
        internal bool Endless { get; init; }

        /// <summary>Whether nothing at all is answered, not even headers, until the server stops.</summary>
        internal bool Silent { get; init; }

        /// <summary>Whether the connection is kept open to answer another request.</summary>
        internal bool KeepAlive { get; init; }

        /// <summary>When set, headers are sent <see cref="HeaderChunkBytes"/> at a time with this pause after each.</summary>
        internal TimeSpan? HeaderChunkDelay { get; init; }

        /// <summary>How much of the headers each paced chunk carries.</summary>
        internal int HeaderChunkBytes { get; init; } = 16;

        /// <summary>When set, this many header bytes are sent and then nothing, until the server stops.</summary>
        internal int? HeaderStallAfter { get; init; }

        /// <summary>When set, the body is sent <see cref="ChunkBytes"/> at a time with this pause after each.</summary>
        internal TimeSpan? ChunkDelay { get; init; }

        /// <summary>How much of the body each paced chunk carries.</summary>
        internal int ChunkBytes { get; init; } = 1024;
    }
}
