using System;
using System.IO;
using System.IO.Compression;
using System.Net.Http;
using System.Threading;
using System.Threading.Tasks;
using OpenMBEE.OpenSysML.Internal;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>The bounds on reading a response body, compressed or not.</summary>
[Trait(Category.Name, Category.Unit)]
public sealed class ResponseBodyTests
{
    private const string Method = "sysml.SysMLService/GetSymbol";

    public static TheoryData<CompressionLevel> Levels =>
        [CompressionLevel.NoCompression, CompressionLevel.Fastest, CompressionLevel.Optimal, CompressionLevel.SmallestSize];

    [Theory]
    [MemberData(nameof(Levels))]
    public async Task AnIncompressibleBodyAtTheLimitFitsItsCompressedBoundAtAnyLevel(CompressionLevel level)
    {
        const int Limit = 300_000;
        var body = new byte[Limit];
        new Random(7).NextBytes(body);
        var gzip = ConnectResponses.Gzip(body, level);

        var read = await ResponseBody.ReadAsync(new ByteArrayContent(gzip), gzip: true, Limit, Method, TestContext.Current.CancellationToken);

        Assert.True(gzip.Length <= ResponseBody.CompressedLimit(Limit), $"{gzip.Length} > {ResponseBody.CompressedLimit(Limit)}");
        Assert.Equal(body, read.ToArray());
    }

    [Fact]
    public void TheCompressedBoundIsZlibsWorstCasePlusTheGzipWrapperAndNeverOverflows()
    {
        Assert.Equal(23, ResponseBody.CompressedLimit(0));
        Assert.Equal(1024 + 128 + 16 + 5 + 18, ResponseBody.CompressedLimit(1024));
        Assert.True(ResponseBody.CompressedLimit(int.MaxValue) > int.MaxValue);
    }

    [Fact]
    public async Task ABodyDeclaredLongerThanTheBoundIsRefusedUnread()
    {
        const int Limit = 1024;
        using var stream = new UnreadableStream();
        using var content = new StreamContent(stream);
        content.Headers.ContentLength = ResponseBody.CompressedLimit(Limit) + 1;

        var error = await Assert.ThrowsAsync<TransportException>(() =>
            ResponseBody.ReadAsync(content, gzip: true, Limit, Method, TestContext.Current.CancellationToken));

        Assert.Equal(StatusCode.ResourceExhausted, error.StatusCode);
        Assert.False(stream.WasRead);
    }

    [Fact]
    public async Task AnEndlessCompressedStreamThatInflatesToNothingIsCutOffAtTheBound()
    {
        const int Limit = 4096;
        using var stream = new EmptyDeflateBlocks();
        using var content = new StreamContent(stream);

        var error = await Assert.ThrowsAsync<TransportException>(() =>
            ResponseBody.ReadAsync(content, gzip: true, Limit, Method, TestContext.Current.CancellationToken));

        Assert.Equal(StatusCode.ResourceExhausted, error.StatusCode);
        Assert.Contains("compressed body is more than", error.Message, StringComparison.Ordinal);
        Assert.InRange(stream.Served, ResponseBody.CompressedLimit(Limit), ResponseBody.CompressedLimit(Limit) + (128 * 1024));
    }

    [Fact]
    public async Task AnIdentityBodyWithoutALengthIsCutOffAtTheLimit()
    {
        const int Limit = 4096;
        using var stream = new EmptyDeflateBlocks();
        using var content = new StreamContent(stream);

        var error = await Assert.ThrowsAsync<TransportException>(() =>
            ResponseBody.ReadAsync(content, gzip: false, Limit, Method, TestContext.Current.CancellationToken));

        Assert.Equal(StatusCode.ResourceExhausted, error.StatusCode);
        Assert.Contains($"its body is more than the {Limit}-byte limit", error.Message, StringComparison.Ordinal);
    }

    [Fact]
    public async Task APrefixIsReadWithoutReadingTheRest()
    {
        using var stream = new EmptyDeflateBlocks();
        using var content = new StreamContent(stream);

        var prefix = await ResponseBody.ReadPrefixAsync(content, gzip: false, 10, TestContext.Current.CancellationToken);

        Assert.Equal(10, prefix.Length);
        Assert.InRange(stream.Served, 10, 64 * 1024);
    }

    /// <summary>A gzip header followed by empty, non-final stored deflate blocks, without end.</summary>
    private sealed class EmptyDeflateBlocks : Stream
    {
        private static readonly byte[] Header = [0x1F, 0x8B, 8, 0, 0, 0, 0, 0, 0, 0xFF];
        private static readonly byte[] EmptyStoredBlock = [0x00, 0x00, 0x00, 0xFF, 0xFF];

        internal long Served { get; private set; }

        public override bool CanRead => true;

        public override bool CanSeek => false;

        public override bool CanWrite => false;

        public override long Length => throw new NotSupportedException();

        public override long Position
        {
            get => throw new NotSupportedException();
            set => throw new NotSupportedException();
        }

        public override int Read(byte[] buffer, int offset, int count) => Read(buffer.AsSpan(offset, count));

        public override int Read(Span<byte> buffer)
        {
            for (var i = 0; i < buffer.Length; i++, Served++)
            {
                buffer[i] = Served < Header.Length ? Header[Served] : EmptyStoredBlock[(Served - Header.Length) % EmptyStoredBlock.Length];
            }

            return buffer.Length;
        }

        public override ValueTask<int> ReadAsync(Memory<byte> buffer, CancellationToken cancellationToken = default) =>
            ValueTask.FromResult(Read(buffer.Span));

        public override void Flush()
        {
        }

        public override long Seek(long offset, SeekOrigin origin) => throw new NotSupportedException();

        public override void SetLength(long value) => throw new NotSupportedException();

        public override void Write(byte[] buffer, int offset, int count) => throw new NotSupportedException();
    }

    /// <summary>A stream that records whether anything tried to read it.</summary>
    private sealed class UnreadableStream : Stream
    {
        internal bool WasRead { get; private set; }

        public override bool CanRead => true;

        public override bool CanSeek => false;

        public override bool CanWrite => false;

        public override long Length => throw new NotSupportedException();

        public override long Position
        {
            get => throw new NotSupportedException();
            set => throw new NotSupportedException();
        }

        public override int Read(byte[] buffer, int offset, int count)
        {
            WasRead = true;
            throw new InvalidOperationException("the body was read");
        }

        public override void Flush()
        {
        }

        public override long Seek(long offset, SeekOrigin origin) => throw new NotSupportedException();

        public override void SetLength(long value) => throw new NotSupportedException();

        public override void Write(byte[] buffer, int offset, int count) => throw new NotSupportedException();
    }
}
