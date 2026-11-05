using System;
using System.IO;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// The SysML v1 model <see cref="Connection.MigrateAsync"/> migrates: a file the service reads, or the document carried
/// inline as bytes, since a <c>.mdzip</c> archive is binary. Exactly one of <see cref="FilePath"/> and
/// <see cref="Content"/> is set.
/// </summary>
public sealed class MigrateSource
{
    private readonly byte[]? content;

    private MigrateSource(string? filePath, byte[]? content)
    {
        FilePath = filePath;
        this.content = content;
    }

    /// <summary>The normalized, fully-qualified file path sent to the service and guarded by a disk write; null for inline content.</summary>
    public string? FilePath { get; }

    /// <summary>The document carried inline, a copy of what it was made from; null for a file.</summary>
    public ReadOnlyMemory<byte>? Content => content is null ? default(ReadOnlyMemory<byte>?) : content;

    /// <summary>
    /// A <c>.xmi</c>, <c>.uml</c> or <c>.mdzip</c> file the service reads, in its own file system. The path must be
    /// fully-qualified in this process's path syntax: it is normalized once and the same path is sent to the service
    /// and guarded by <see cref="Migration.WriteAsync"/>. Relative paths are refused because the service's working
    /// directory is not known. Its form is inferred from its extension unless <see cref="MigrateOptions.FromFormat"/>
    /// names one. For a service on another host, use a path shared with this process or carry the document inline.
    /// </summary>
    /// <param name="path">The file.</param>
    /// <returns>The source.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="path"/> is null.</exception>
    /// <exception cref="ArgumentException"><paramref name="path"/> is empty, is not fully-qualified or is invalid.</exception>
    public static MigrateSource FromFile(string path)
    {
        ArgumentException.ThrowIfNullOrEmpty(path);
        if (!Path.IsPathFullyQualified(path))
        {
            throw new ArgumentException("A migration file path must be fully-qualified so the service and the source guard use the same path.", nameof(path));
        }

        return new MigrateSource(Path.GetFullPath(path), null);
    }

    /// <summary>
    /// The document carried inline, copied, which has no extension to infer its form from, so
    /// <see cref="MigrateOptions.FromFormat"/> must name it; the service refuses one that does not with
    /// <see cref="StatusCode.InvalidArgument"/>.
    /// </summary>
    /// <param name="content">The v1 model's bytes.</param>
    /// <returns>The source.</returns>
    public static MigrateSource Inline(ReadOnlySpan<byte> content) => new(null, content.ToArray());
}
