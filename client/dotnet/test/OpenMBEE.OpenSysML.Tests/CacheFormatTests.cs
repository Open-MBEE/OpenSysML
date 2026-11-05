using System;
using System.IO;
using System.Linq;
using System.Text;
using OpenMBEE.OpenSysML.Internal;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>The cache's file names, which must be those the Python, Java and Rust clients use, and its record's format.</summary>
[Trait(Category.Name, Category.Unit)]
public sealed class CacheFormatTests
{
    private const string Digest = "0123456789ABCDEF0123456789abcdef0123456789abcdef0123456789abcdef";

    [Fact]
    public void TheFilesAreNamedAsEveryClientNamesThem()
    {
        var directory = Path.Combine(Path.GetTempPath(), "cache");
        var layout = new CacheLayout(directory);
        var executable = OperatingSystem.IsWindows() ? "sysml-grpc.exe" : "sysml-grpc";

        Assert.Equal(Path.GetFullPath(directory), layout.Directory);
        Assert.Equal(Path.Combine(layout.Directory, executable), layout.Executable);
        Assert.Equal(Path.Combine(layout.Directory, executable + ".json"), layout.Metadata);
        Assert.Equal(Path.Combine(layout.Directory, executable + ".lock"), layout.Lock);
        Assert.Equal(Path.Combine(layout.Directory, executable + ".lock.dotnet"), layout.Guard);
        Assert.Equal(
            Path.Combine(layout.Directory, OperatingSystem.IsWindows() ? "sysml-grpc-0123456789abcdef.exe" : "sysml-grpc-0123456789abcdef"),
            layout.Snapshot(Digest));
        Assert.Throws<ArgumentException>(() => layout.Snapshot("0123456789abcde"));
        Assert.Throws<ArgumentException>(() => layout.Snapshot("0123456789abcdeg"));
    }

    [Fact]
    public void TheCacheIsInTheHomeDirectoryWhenThereIsOne()
    {
        var home = Path.Combine(Path.GetTempPath(), "home");

        Assert.Equal(Path.Combine(home, ".opensysml", "bin"), CacheLayout.InHome(name => name == BinaryResolver.HomeVariable ? home : null)!.Directory);
        Assert.Null(CacheLayout.InHome(_ => null));
        Assert.Null(CacheLayout.InHome(_ => "  "));
    }

    [Fact]
    public void StagedFilesAreBesideWhatTheyPublishAndOnlyThisClientsAreTakenForItsLeftovers()
    {
        var layout = new CacheLayout(Path.Combine(Path.GetTempPath(), "cache"));
        var staged = new[] { CacheLayout.Temporary(layout.Executable), CacheLayout.Temporary(layout.Metadata), CacheLayout.Temporary(layout.Snapshot(Digest)) };

        Assert.All(staged, path => Assert.Equal(layout.Directory, Path.GetDirectoryName(path)));
        Assert.All(staged, path => Assert.True(CacheLayout.IsTemporary(Path.GetFileName(path)), path));
        Assert.Equal(staged.Length, staged.Distinct().Count());
        Assert.NotEqual(CacheLayout.Temporary(layout.Executable), CacheLayout.Temporary(layout.Executable));

        // What the Python and Java clients stage, and the cache's own files, are not this client's to remove.
        foreach (var name in new[]
        {
            "sysml-grpc.tmp", "sysml-grpc-0123456789abcdef.tmp", "sysml-grpc.12345678901234567890.tmp", "sysml-grpc.1234.json",
            "sysml-grpc.1234.link", "sysml-grpc", "sysml-grpc.json", "sysml-grpc.lock", "sysml-grpc.lock.dotnet",
            "other.dotnet-0123456789abcdef.tmp", "sysml-grpc.dotnet-0123456789ABCDEF.tmp", "sysml-grpc.dotnet-0123456789abcde.tmp",
        })
        {
            Assert.False(CacheLayout.IsTemporary(name), name);
        }
    }

    [Fact]
    public void ARecordIsWrittenByteForByteAsPythonsJsonDumpWritesIt()
    {
        var record = new CacheMetadata("v1.2.3", Digest.ToLowerInvariant(), "Open-MBEE/OpenSysML");

        Assert.Equal(
            "{\"version\": \"v1.2.3\", \"sha256\": \"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef\", \"repo\": \"Open-MBEE/OpenSysML\"}",
            Encoding.ASCII.GetString(record.Format()));
    }

    [Fact]
    public void EveryCharacterIsEscapedAsPythonEscapesIt()
    {
        // json.dumps({"version": s, "sha256": "ab" * 32, "repo": "Open-MBEE/OpenSysML"}) with Python 3.
        var record = new CacheMetadata("q\"b\\n\n\r\t\b\f\u0001\u001f~\u007f\u00e9\U0001F600 /", string.Concat(Enumerable.Repeat("ab", 32)), "Open-MBEE/OpenSysML");

        Assert.Equal(
            "{\"version\": \"q\\\"b\\\\n\\n\\r\\t\\b\\f\\u0001\\u001f~\\u007f\\u00e9\\ud83d\\ude00 /\", \"sha256\": \"abababababababababababababababababababababababababababababababab\", \"repo\": \"Open-MBEE/OpenSysML\"}",
            Encoding.ASCII.GetString(record.Format()));
        Assert.Equal(record, CacheMetadata.Parse(record.Format()).Metadata);
    }

    [Fact]
    public void ARecordWithoutARepositoryIsReadButNeverWritten()
    {
        var read = CacheMetadata.Parse(Bytes($"{{\"version\": \"v1\", \"sha256\": \"{Digest}\"}}"));

        Assert.Equal(new CacheMetadata("v1", Digest, null), read.Metadata);
        Assert.Null(read.Problem);
        Assert.Null(CacheMetadata.Parse(Bytes($"{{\"version\": \"v1\", \"sha256\": \"{Digest}\", \"repo\": null}}")).Metadata!.Repo);
        Assert.Null(CacheMetadata.Parse(Bytes($"{{\"version\": \"v1\", \"sha256\": \"{Digest}\", \"repo\": \"\"}}")).Metadata!.Repo);
        Assert.Throws<InvalidOperationException>(() => read.Metadata!.Format());
    }

    [Fact]
    public void AReadRecordDescribesAnExecutableByItsDigestInEitherCase()
    {
        var record = CacheMetadata.Parse(Bytes($"{{\"version\": \"v1\", \"sha256\": \"{Digest}\", \"repo\": \"o/r\", \"extra\": [1]}}")).Metadata!;

        Assert.True(record.Describes(Digest.ToLowerInvariant()));
        Assert.True(record.Describes(Digest.ToUpperInvariant()));
        Assert.False(record.Describes(new string('0', 64)));
    }

    [Fact]
    public void TheLastOfARepeatedMemberCountsAsPythonReadsIt()
    {
        var read = CacheMetadata.Parse(Bytes($"{{\"version\": \"v1\", \"sha256\": \"{Digest}\", \"repo\": \"a/b\", \"version\": \"v2\", \"repo\": \"c/d\"}}"));

        Assert.Equal(new CacheMetadata("v2", Digest, "c/d"), read.Metadata);
    }

    [Theory]
    [InlineData("", "not JSON")]
    [InlineData("{\"version\": \"v1\", \"sha256\": \"01234", "not JSON")]
    [InlineData("[]", "a JSON array, not an object")]
    [InlineData("\"v1\"", "a JSON string, not an object")]
    [InlineData("null", "a JSON null, not an object")]
    [InlineData("{}", "without a version")]
    [InlineData("{\"version\": \"\", \"sha256\": \"" + Digest + "\"}", "without a version")]
    [InlineData("{\"version\": 1, \"sha256\": \"" + Digest + "\"}", "without a version")]
    [InlineData("{\"version\": \"v1\"}", "without a SHA-256 digest")]
    [InlineData("{\"version\": \"v1\", \"sha256\": \"abc\"}", "without a SHA-256 digest")]
    [InlineData("{\"version\": \"v1\", \"sha256\": \"" + "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdeg" + "\"}", "without a SHA-256 digest")]
    [InlineData("{\"version\": \"v1\", \"sha256\": 7}", "without a SHA-256 digest")]
    [InlineData("{\"version\": \"v1\", \"sha256\": \"" + Digest + "\", \"repo\": 3}", "with a repository that is not a string")]
    public void WhatIsNotARecordSaysWhy(string text, string problem)
    {
        var read = CacheMetadata.Parse(Bytes(text));

        Assert.Null(read.Metadata);
        Assert.StartsWith(problem, read.Problem, StringComparison.Ordinal);
    }

    [Fact]
    public void InvalidUtf8InARecordIsReportedRatherThanThrown()
    {
        byte[] bytes = [.. Bytes("{\"version\": \""), 0xff, .. Bytes($"\", \"sha256\": \"{Digest}\", \"repo\": \"o/r\"}}")];

        AssertInvalidUnicode(bytes);
    }

    [Theory]
    [InlineData("version", "\\ud800")]
    [InlineData("version", "\\udc00")]
    [InlineData("version", "\\ud800x")]
    [InlineData("version", "\\udc00\\ud800")]
    [InlineData("sha256", "\\ud800")]
    [InlineData("repo", "\\ud800")]
    [InlineData("\\ud800", "extra")]
    public void InvalidUnicodeInRecordStringsOrNamesIsReportedRatherThanThrown(string name, string value)
    {
        AssertInvalidUnicode(Bytes($"{{\"version\": \"v1\", \"sha256\": \"{Digest}\", \"repo\": \"o/r\", \"{name}\": \"{value}\"}}"));
    }

    [Fact]
    public void ARecordFileThatIsAbsentADirectoryOrTooLargeIsNone()
    {
        var directory = Directory.CreateTempSubdirectory("opensysml-record-").FullName;
        try
        {
            var path = Path.Combine(directory, "sysml-grpc.json");
            Assert.Equal(new CacheMetadataRead(null, "absent"), CacheMetadata.Read(path));

            Directory.CreateDirectory(path);
            Assert.Equal(new CacheMetadataRead(null, "a directory"), CacheMetadata.Read(path));
            Directory.Delete(path);

            File.WriteAllText(path, $"{{\"version\": \"v1\", \"sha256\": \"{Digest}\", \"repo\": \"{new string('r', CacheMetadata.MaxBytes)}\"}}");
            Assert.Equal(new CacheMetadataRead(null, "larger than 64 KiB"), CacheMetadata.Read(path));

            var whole = new CacheMetadata("v1", Digest, "o/r").Format();
            File.WriteAllBytes(path, whole);
            Assert.Equal(new CacheMetadata("v1", Digest, "o/r"), CacheMetadata.Read(path).Metadata);

            // A record another client was still writing in place, as the Python client writes it.
            File.WriteAllBytes(path, whole[..(whole.Length / 2)]);
            Assert.StartsWith("not JSON", CacheMetadata.Read(path).Problem, StringComparison.Ordinal);
        }
        finally
        {
            Directory.Delete(directory, recursive: true);
        }
    }

    private static void AssertInvalidUnicode(byte[] bytes)
    {
        var read = CacheMetadata.Parse(bytes);
        Assert.Null(read.Metadata);
        Assert.StartsWith("not JSON", read.Problem, StringComparison.Ordinal);

        using var cache = new TestCache();
        Directory.CreateDirectory(cache.Layout.Directory);
        File.WriteAllBytes(cache.Layout.Metadata, bytes);
        Assert.Equal(read, CacheMetadata.Read(cache.Layout.Metadata));
    }

    private static byte[] Bytes(string text) => Encoding.UTF8.GetBytes(text);
}
