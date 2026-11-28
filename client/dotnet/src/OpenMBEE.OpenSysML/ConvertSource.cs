using System;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// The model <see cref="Connection.ConvertAsync"/> converts: a file the service reads, notation or a graph carried
/// inline, or a model the service parsed, named by its hash. Exactly one of <see cref="FilePath"/>, <see cref="Content"/>
/// and <see cref="ModelHash"/> is set.
/// </summary>
public sealed record ConvertSource
{
    private ConvertSource(string? filePath, string? content, string? modelHash)
    {
        FilePath = filePath;
        Content = content;
        ModelHash = modelHash;
    }

    /// <summary>The file the service reads, afresh, in its own file system; null for another source.</summary>
    public string? FilePath { get; }

    /// <summary>The model carried inline; null for another source.</summary>
    public string? Content { get; }

    /// <summary>The hash of a model the service parsed, whose parsed source is converted; null for another source.</summary>
    public string? ModelHash { get; }

    /// <summary>
    /// A file the service reads, in its own file system: a relative path resolves against its working directory. Its
    /// format is inferred from its extension unless <see cref="ConvertOptions.FromFormat"/> names one.
    /// </summary>
    /// <param name="path">The file.</param>
    /// <returns>The source.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="path"/> is null.</exception>
    /// <exception cref="ArgumentException"><paramref name="path"/> is empty.</exception>
    public static ConvertSource FromFile(string path)
    {
        ArgumentException.ThrowIfNullOrEmpty(path);
        return new ConvertSource(path, null, null);
    }

    /// <summary>
    /// A model carried inline, which has no extension to infer its format from, so <see cref="ConvertOptions.FromFormat"/>
    /// must name it; the service refuses one that does not with <see cref="StatusCode.InvalidArgument"/>.
    /// </summary>
    /// <param name="content">The model's text.</param>
    /// <returns>The source.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="content"/> is null.</exception>
    public static ConvertSource Inline(string content)
    {
        ArgumentNullException.ThrowIfNull(content);
        return new ConvertSource(null, content, null);
    }

    /// <summary>
    /// A model the service parsed, named by its hash: the source that parse read is converted, so a file edited since
    /// does not change the answer. <see cref="Model.ConvertAsync"/> converts a model this way.
    /// </summary>
    /// <param name="modelHash">A hash a parse returned.</param>
    /// <returns>The source.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="modelHash"/> is null.</exception>
    /// <exception cref="ArgumentException"><paramref name="modelHash"/> is empty or white space.</exception>
    public static ConvertSource FromModelHash(string modelHash)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(modelHash);
        return new ConvertSource(null, null, modelHash);
    }
}
