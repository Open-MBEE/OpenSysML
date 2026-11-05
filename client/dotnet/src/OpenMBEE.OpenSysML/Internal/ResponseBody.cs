using System;
using System.Buffers.Binary;
using System.IO;
using System.IO.Compression;
using System.Net.Http;
using System.Threading;
using System.Threading.Tasks;

namespace OpenMBEE.OpenSysML.Internal;

/// <summary>
/// Reads an HTTP response body under a size limit, inflating a gzip body as it reads. The limit
/// bounds the body after inflation; the compressed body is bounded too, by the largest a deflate
/// encoder makes of that many bytes, so neither a bomb nor an endless stream is read past it. No
/// buffer of the limit's size is made up front.
/// </summary>
internal static class ResponseBody
{
    /// <summary>The content coding a Connect client accepts besides identity.</summary>
    internal const string Gzip = "gzip";

    // The gzip header and trailer around one member's deflate data, with no optional header field.
    private const int GzipWrapper = 18;
    private const int GzipTrailer = 8;
    private const int ChunkSize = 81920;
    private const int MaxInitialCapacity = 1024 * 1024;

    /// <summary>
    /// The longest compressed body that can inflate to <paramref name="limit"/> bytes: zlib's
    /// worst-case bound for any deflate setting, plus the gzip wrapper.
    /// </summary>
    /// <param name="limit">The largest body after inflation.</param>
    /// <returns>The largest compressed body read.</returns>
    internal static long CompressedLimit(int limit) =>
        limit + ((limit + 7L) >> 3) + ((limit + 63L) >> 6) + 5 + GzipWrapper;

    /// <summary>
    /// Reads a whole body. A gzip body must be one gzip member, as an HTTP server writes it, whose
    /// CRC-32 and length match what it inflates to; anything after the member is refused with it.
    /// </summary>
    /// <param name="content">The response content.</param>
    /// <param name="gzip">Whether the body is gzip-compressed.</param>
    /// <param name="limit">The largest body to accept, after inflation.</param>
    /// <param name="method">The method called, for the refusal.</param>
    /// <param name="cancellationToken">Cancels the read.</param>
    /// <returns>The body, inflated.</returns>
    /// <exception cref="TransportException">The body is over the limit, with <see cref="StatusCode.ResourceExhausted"/>.</exception>
    /// <exception cref="InvalidDataException">The body is shorter than its Content-Length, or its gzip is corrupt or cut short.</exception>
    internal static async Task<ReadOnlyMemory<byte>> ReadAsync(HttpContent content, bool gzip, int limit, string method, CancellationToken cancellationToken)
    {
        var declared = content.Headers.ContentLength;
        var rawLimit = gzip ? CompressedLimit(limit) : limit;
        if (declared > rawLimit)
        {
            throw TooLarge(method, gzip
                ? $"its compressed body is {declared} bytes, more than {rawLimit} could inflate to within the {limit}-byte limit"
                : $"its body is {declared} bytes, more than the {limit}-byte limit");
        }

        var stream = await content.ReadAsStreamAsync(cancellationToken).ConfigureAwait(false);
        await using (stream.ConfigureAwait(false))
        {
            var raw = new CountingStream(stream, rawLimit, () => TooLarge(method, gzip
                ? $"its compressed body is more than {rawLimit} bytes, more than could inflate to within the {limit}-byte limit"
                : $"its body is more than the {limit}-byte limit"));
            var capacity = gzip ? 0 : (int)Math.Min(declared ?? 0, Math.Min(limit, MaxInitialCapacity));
            using var body = new MemoryStream(capacity);
            if (gzip)
            {
                var inflate = new GZipStream(raw, CompressionMode.Decompress, leaveOpen: true);
                await using (inflate.ConfigureAwait(false))
                {
                    await CopyAsync(inflate, body, limit, method, cancellationToken).ConfigureAwait(false);
                }

                await raw.DrainAsync(cancellationToken).ConfigureAwait(false);
            }
            else
            {
                await CopyAsync(raw, body, limit, method, cancellationToken).ConfigureAwait(false);
            }

            if (declared is { } length && raw.Count != length)
            {
                throw new InvalidDataException($"the body ended after {raw.Count} of the {length} bytes its Content-Length declares");
            }

            var read = new ReadOnlyMemory<byte>(body.GetBuffer(), 0, (int)body.Length);
            if (gzip && raw.Count != 0)
            {
                CheckTrailer(raw, read.Span);
            }

            return read;
        }
    }

    /// <summary>Reads at most the first bytes of a body, inflated, for a diagnostic; the rest is left unread.</summary>
    /// <param name="content">The response content.</param>
    /// <param name="gzip">Whether the body is gzip-compressed.</param>
    /// <param name="count">How many bytes to read at most.</param>
    /// <param name="cancellationToken">Cancels the read.</param>
    /// <returns>The bytes read.</returns>
    internal static async Task<byte[]> ReadPrefixAsync(HttpContent content, bool gzip, int count, CancellationToken cancellationToken)
    {
        var stream = await content.ReadAsStreamAsync(cancellationToken).ConfigureAwait(false);
        await using (stream.ConfigureAwait(false))
        {
            var source = gzip ? new GZipStream(stream, CompressionMode.Decompress, leaveOpen: true) : stream;
            try
            {
                var prefix = new byte[count];
                var read = await source.ReadAtLeastAsync(prefix, count, throwOnEndOfStream: false, cancellationToken).ConfigureAwait(false);
                return prefix[..read];
            }
            finally
            {
                if (gzip)
                {
                    await source.DisposeAsync().ConfigureAwait(false);
                }
            }
        }
    }

    private static async Task CopyAsync(Stream source, MemoryStream destination, int limit, string method, CancellationToken cancellationToken)
    {
        var chunk = new byte[ChunkSize];
        while (await source.ReadAsync(chunk, cancellationToken).ConfigureAwait(false) is var read and > 0)
        {
            if (destination.Length + read > limit)
            {
                throw TooLarge(method, $"its body is more than the {limit}-byte limit");
            }

            destination.Write(chunk, 0, read);
        }
    }

    // GZipStream stops at the end of the deflate data without reading the trailer, so a body cut
    // short inflates to a prefix of the message with no error: the trailer is checked here instead.
    private static void CheckTrailer(CountingStream raw, ReadOnlySpan<byte> inflated)
    {
        if (raw.Count < GzipWrapper)
        {
            throw new InvalidDataException($"the gzip body is {raw.Count} bytes, shorter than a gzip member can be");
        }

        var trailer = raw.Tail;
        var crc = BinaryPrimitives.ReadUInt32LittleEndian(trailer);
        var size = BinaryPrimitives.ReadUInt32LittleEndian(trailer[4..]);
        if (size != unchecked((uint)inflated.Length) || crc != Crc32.Of(inflated))
        {
            throw new InvalidDataException(
                "the gzip body's trailer does not match what it inflates to: it was cut short, corrupted, or is more than one gzip member");
        }
    }

    private static TransportException TooLarge(string method, string why) =>
        new(StatusCode.ResourceExhausted, $"{method}: the answer is over ConnectionOptions.MaxResponseSize: {why}.", method);

    /// <summary>A read-only stream that counts what it reads, refuses to read past a limit and keeps its last bytes.</summary>
    private sealed class CountingStream(Stream inner, long limit, Func<Exception> overflow) : Stream
    {
        private readonly byte[] tail = new byte[GzipTrailer];

        /// <summary>How many bytes have been read.</summary>
        internal long Count { get; private set; }

        /// <summary>The last bytes read, oldest first: the gzip trailer once the body has been read.</summary>
        internal ReadOnlySpan<byte> Tail => tail;

        public override bool CanRead => true;

        public override bool CanSeek => false;

        public override bool CanWrite => false;

        public override long Length => throw new NotSupportedException();

        public override long Position
        {
            get => throw new NotSupportedException();
            set => throw new NotSupportedException();
        }

        /// <summary>Reads to the end, so that what follows the gzip member is counted and checked.</summary>
        internal async Task DrainAsync(CancellationToken cancellationToken)
        {
            var chunk = new byte[ChunkSize];
            while (await ReadAsync(chunk, cancellationToken).ConfigureAwait(false) > 0)
            {
            }
        }

        public override int Read(byte[] buffer, int offset, int count) => Read(buffer.AsSpan(offset, count));

        public override int Read(Span<byte> buffer) => Counted(buffer, inner.Read(buffer));

        public override Task<int> ReadAsync(byte[] buffer, int offset, int count, CancellationToken cancellationToken) =>
            ReadAsync(buffer.AsMemory(offset, count), cancellationToken).AsTask();

        public override async ValueTask<int> ReadAsync(Memory<byte> buffer, CancellationToken cancellationToken = default)
        {
            var count = await inner.ReadAsync(buffer, cancellationToken).ConfigureAwait(false);
            return Counted(buffer.Span, count);
        }

        public override void Flush()
        {
        }

        public override long Seek(long offset, SeekOrigin origin) => throw new NotSupportedException();

        public override void SetLength(long value) => throw new NotSupportedException();

        public override void Write(byte[] buffer, int offset, int count) => throw new NotSupportedException();

        private int Counted(ReadOnlySpan<byte> buffer, int count)
        {
            if (Count + count > limit)
            {
                throw overflow();
            }

            var read = buffer[..count];
            if (read.Length >= GzipTrailer)
            {
                read[^GzipTrailer..].CopyTo(tail);
            }
            else
            {
                tail.AsSpan(read.Length).CopyTo(tail);
                read.CopyTo(tail.AsSpan(GzipTrailer - read.Length));
            }

            Count += count;
            return count;
        }
    }

    /// <summary>CRC-32 as gzip computes it: the IEEE 802.3 polynomial, reflected.</summary>
    private static class Crc32
    {
        private static readonly uint[] Table = MakeTable();

        internal static uint Of(ReadOnlySpan<byte> data)
        {
            var crc = 0xFFFFFFFFu;
            foreach (var b in data)
            {
                crc = Table[(crc ^ b) & 0xFF] ^ (crc >> 8);
            }

            return ~crc;
        }

        private static uint[] MakeTable()
        {
            var table = new uint[256];
            for (var n = 0u; n < table.Length; n++)
            {
                var c = n;
                for (var k = 0; k < 8; k++)
                {
                    c = (c & 1) != 0 ? 0xEDB88320u ^ (c >> 1) : c >> 1;
                }

                table[n] = c;
            }

            return table;
        }
    }
}
