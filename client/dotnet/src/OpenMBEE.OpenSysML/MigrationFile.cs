using System;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// A file a migrated model refers to by relative path, such as a picture a v1 comment embeds, to be written beside it.
/// Two are equal when their paths and bytes are.
/// </summary>
public sealed class MigrationFile : IEquatable<MigrationFile>
{
    private readonly byte[] content;

    /// <summary>Creates a file, copying its bytes.</summary>
    /// <param name="path">Its path relative to the migrated model, <c>/</c>-separated.</param>
    /// <param name="content">Its bytes.</param>
    /// <exception cref="ArgumentNullException"><paramref name="path"/> is null.</exception>
    public MigrationFile(string path, ReadOnlySpan<byte> content)
    {
        ArgumentNullException.ThrowIfNull(path);
        Path = path;
        this.content = content.ToArray();
    }

    /// <summary>
    /// Its path relative to the migrated model's directory, <c>/</c>-separated, as the model refers to it.
    /// <see cref="Migration.WriteAsync"/> refuses one that would land anywhere but below that directory.
    /// </summary>
    public string Path { get; }

    /// <summary>Its bytes.</summary>
    public ReadOnlyMemory<byte> Content => content;

    /// <inheritdoc/>
    public bool Equals(MigrationFile? other) =>
        other is not null && (ReferenceEquals(this, other) || (Path == other.Path && content.AsSpan().SequenceEqual(other.content)));

    /// <inheritdoc/>
    public override bool Equals(object? obj) => Equals(obj as MigrationFile);

    /// <inheritdoc/>
    public override int GetHashCode()
    {
        var hash = default(HashCode);
        hash.Add(Path, StringComparer.Ordinal);
        hash.AddBytes(content);
        return hash.ToHashCode();
    }

    /// <summary>The file's path and size, as <c>images/fleet.png (2048 bytes)</c>.</summary>
    /// <returns>The file's text.</returns>
    public override string ToString() => $"{Path} ({content.Length} bytes)";
}
