using System;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// One document of a model <see cref="Connection.ParseSourcesAsync"/> parses: a file the service
/// reads, or notation carried inline. Two documents of one model need distinct names: an inline
/// document is named by <see cref="Name"/>, or by its position when it has none, and a file by
/// its path. Diagnostics name the document they came from.
/// </summary>
public sealed record SourceDocument
{
    private SourceDocument(string? filePath, string? content, string? name, SourceLanguage? language)
    {
        FilePath = filePath;
        Content = content;
        Name = name;
        Language = language;
    }

    /// <summary>The path the service reads, as the service resolves it; null for inline content.</summary>
    public string? FilePath { get; }

    /// <summary>The notation carried inline; null for a file.</summary>
    public string? Content { get; }

    /// <summary>The name an inline document is reported and indexed under; null for a file, or to be named by position.</summary>
    public string? Name { get; }

    /// <summary>
    /// The notation inline content is written in; null for a file, whose extension says which, or for
    /// the service's default, SysML. Naming one needs the <see cref="Capabilities.InlineLanguage"/> capability.
    /// </summary>
    public SourceLanguage? Language { get; }

    /// <summary>A document the service reads from a file.</summary>
    /// <param name="path">The path, which the service resolves in its own file system.</param>
    /// <returns>The document.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="path"/> is null.</exception>
    /// <exception cref="ArgumentException"><paramref name="path"/> is empty.</exception>
    public static SourceDocument FromFile(string path)
    {
        ArgumentException.ThrowIfNullOrEmpty(path);
        return new SourceDocument(path, null, null, null);
    }

    /// <summary>A document of inline notation.</summary>
    /// <param name="content">The notation.</param>
    /// <param name="name">The name it is reported and indexed under; null to be named by its position.</param>
    /// <param name="language">Its notation; null for the service's default, SysML.</param>
    /// <returns>The document.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="content"/> is null.</exception>
    /// <exception cref="ArgumentException"><paramref name="name"/> is empty, or <paramref name="language"/> is not a <see cref="SourceLanguage"/>.</exception>
    public static SourceDocument Inline(string content, string? name = null, SourceLanguage? language = null)
    {
        ArgumentNullException.ThrowIfNull(content);
        if (name is { Length: 0 })
        {
            throw new ArgumentException("An inline document's name is null, to be named by position, or not empty.", nameof(name));
        }

        if (language is { } named && !Enum.IsDefined(named))
        {
            throw new ArgumentException($"{named} is not a SourceLanguage.", nameof(language));
        }

        return new SourceDocument(null, content, name, language);
    }

    /// <summary>The document, as <c>file a.sysml</c> or <c>inline "name" (kerml)</c>.</summary>
    /// <returns>The document's text.</returns>
    public override string ToString() =>
        FilePath is not null
            ? $"file {FilePath}"
            : $"inline {(Name is null ? "(unnamed)" : $"\"{Name}\"")}{(Language is { } language ? $" ({language})" : string.Empty)}";
}
