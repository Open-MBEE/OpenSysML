using System;
using System.Buffers.Binary;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Security.Cryptography;
using System.Text;

namespace OpenMBEE.OpenSysML.Generate;

/// <summary>One model document: where it was read from, the name it is parsed and stamped under, and its exact bytes.</summary>
/// <param name="Path">The path it was read from.</param>
/// <param name="LogicalName">The name it is parsed and stamped under, which no machine path or working directory changes.</param>
/// <param name="Bytes">The file's exact bytes.</param>
/// <param name="Text">The notation the bytes hold, without a leading byte-order mark.</param>
internal sealed record SourceFile(string Path, string LogicalName, byte[] Bytes, string Text)
{
    /// <summary>The notation, by the file's extension, as an inline document names it: null for SysML, the service's default.</summary>
    internal SourceLanguage? InlineLanguage =>
        LogicalName.EndsWith(".kerml", StringComparison.Ordinal) ? SourceLanguage.KerML : null;
}

/// <summary>
/// The documents a model is generated from, read the way the <c>sysml</c> command reads them, and the stamp
/// that records exactly what they held.
/// </summary>
internal static class SourceClosure
{
    /// <summary>The algorithm a stamp names before its digest.</summary>
    internal const string HashAlgorithm = "sha256";

    private static readonly string[] ModelSuffixes = [".sysml", ".kerml"];

    /// <summary>
    /// Reads the documents the arguments name, in order: a file as named, under its file name; a directory as
    /// every <c>.sysml</c> and <c>.kerml</c> file beneath it in path order, hidden directories skipped, each under
    /// the directory's name and its path within it. A file named twice is read once.
    /// </summary>
    /// <exception cref="UsageException">A path does not exist, cannot be read, is not UTF-8, names a directory with no model file, or two documents would share a name.</exception>
    internal static IReadOnlyList<SourceFile> Read(IEnumerable<string> arguments)
    {
        var named = new List<(string Path, string LogicalName)>();
        var seen = new HashSet<string>(StringComparer.Ordinal);
        foreach (var argument in arguments)
        {
            if (Directory.Exists(argument))
            {
                var root = Path.GetFullPath(argument);
                var rootName = Path.GetFileName(Path.TrimEndingDirectorySeparator(root));
                var files = new List<string>();
                try
                {
                    ModelFiles(root, new HashSet<string>(StringComparer.Ordinal), files);
                }
                catch (Exception exception) when (exception is IOException or UnauthorizedAccessException)
                {
                    throw new UsageException($"cannot read {argument}: {exception.Message}");
                }
                if (files.Count == 0)
                {
                    throw new UsageException($"{argument} holds no .sysml or .kerml file");
                }

                foreach (var file in files.Order(StringComparer.Ordinal))
                {
                    if (seen.Add(Path.GetFullPath(file)))
                    {
                        named.Add((file, rootName + "/" + Path.GetRelativePath(root, file).Replace('\\', '/')));
                    }
                }
            }
            else if (File.Exists(argument))
            {
                if (seen.Add(Path.GetFullPath(argument)))
                {
                    named.Add((argument, Path.GetFileName(argument)));
                }
            }
            else
            {
                throw new UsageException($"{argument} does not exist");
            }
        }

        var byName = new Dictionary<string, string>(StringComparer.Ordinal);
        foreach (var (path, logicalName) in named)
        {
            if (!byName.TryAdd(logicalName, path))
            {
                throw new UsageException(
                    $"{byName[logicalName]} and {path} would both be the document {logicalName}; name the directory that holds them instead");
            }
        }

        return [.. named.Select(document => ReadFile(document.Path, document.LogicalName))];
    }

    /// <summary>
    /// The stamp of a closure: the SHA-256 digest of its document count, then each document in ordinal order of
    /// its name, as the name's length and UTF-8 bytes followed by the content's length and exact bytes, every
    /// length a big-endian 64-bit integer.
    /// </summary>
    internal static string Stamp(IEnumerable<SourceFile> documents)
    {
        var ordered = documents.OrderBy(document => document.LogicalName, StringComparer.Ordinal).ToList();
        using var hash = IncrementalHash.CreateHash(HashAlgorithmName.SHA256);
        AppendLength(hash, ordered.Count);
        foreach (var document in ordered)
        {
            var name = Encoding.UTF8.GetBytes(document.LogicalName);
            AppendLength(hash, name.Length);
            hash.AppendData(name);
            AppendLength(hash, document.Bytes.Length);
            hash.AppendData(document.Bytes);
        }

        return $"{HashAlgorithm}:{Convert.ToHexStringLower(hash.GetHashAndReset())}";
    }

    private static void AppendLength(IncrementalHash hash, long length)
    {
        Span<byte> buffer = stackalloc byte[sizeof(long)];
        BinaryPrimitives.WriteInt64BigEndian(buffer, length);
        hash.AppendData(buffer);
    }

    private static SourceFile ReadFile(string path, string logicalName)
    {
        byte[] bytes;
        try
        {
            bytes = File.ReadAllBytes(path);
        }
        catch (Exception exception) when (exception is IOException or UnauthorizedAccessException)
        {
            throw new UsageException($"cannot read {path}: {exception.Message}");
        }

        string text;
        try
        {
            text = new UTF8Encoding(encoderShouldEmitUTF8Identifier: false, throwOnInvalidBytes: true).GetString(bytes);
        }
        catch (DecoderFallbackException)
        {
            throw new UsageException($"{path} is not UTF-8 text");
        }

        return new SourceFile(path, logicalName, bytes, text.StartsWith('\uFEFF') ? text[1..] : text);
    }

    // Every model file beneath a directory; a directory reached twice through links is read once.
    private static void ModelFiles(string directory, HashSet<string> visited, List<string> files)
    {
        var resolved = new DirectoryInfo(directory).ResolveLinkTarget(returnFinalTarget: true)?.FullName ?? Path.GetFullPath(directory);
        if (!visited.Add(resolved))
        {
            return;
        }

        foreach (var entry in new DirectoryInfo(directory).EnumerateFileSystemInfos().OrderBy(entry => entry.Name, StringComparer.Ordinal))
        {
            if (entry is DirectoryInfo subdirectory)
            {
                if (!subdirectory.Name.StartsWith('.'))
                {
                    ModelFiles(subdirectory.FullName, visited, files);
                }
            }
            else if (ModelSuffixes.Any(suffix => entry.Name.EndsWith(suffix, StringComparison.Ordinal)) && File.Exists(entry.FullName))
            {
                files.Add(entry.FullName);
            }
        }
    }
}
