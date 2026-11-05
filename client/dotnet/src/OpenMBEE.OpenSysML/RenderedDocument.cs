using System;

namespace OpenMBEE.OpenSysML;

/// <summary>A document the model declares, rendered in one form, byte-for-byte what the CLI's <c>-render-document</c> writes.</summary>
public sealed record RenderedDocument
{
    /// <summary>Creates a rendered document.</summary>
    /// <param name="form">The form it was rendered to.</param>
    /// <param name="content">The rendered text.</param>
    /// <param name="fileName">The file a rendered set writes it to; null when the service named none.</param>
    /// <exception cref="ArgumentNullException"><paramref name="content"/> is null.</exception>
    /// <exception cref="ArgumentException"><paramref name="fileName"/> is empty.</exception>
    public RenderedDocument(DocumentForm form, string content, string? fileName)
    {
        ArgumentNullException.ThrowIfNull(content);
        if (fileName is { Length: 0 })
        {
            throw new ArgumentException("A file name is not empty; null names none.", nameof(fileName));
        }

        Form = form;
        Content = content;
        FileName = fileName;
    }

    /// <summary>The form it was rendered to.</summary>
    public DocumentForm Form { get; }

    /// <summary>The rendered Markdown, HTML or AsciiDoc.</summary>
    public string Content { get; }

    /// <summary>
    /// The file a rendered set (<c>-render-documents</c>) writes the document to in this form, which the other
    /// documents' links to it name; null when the service named none, as one without
    /// <see cref="Capabilities.RenderDocumentFileName"/> does.
    /// </summary>
    public string? FileName { get; }

    /// <summary>The rendered text itself.</summary>
    /// <returns><see cref="Content"/>.</returns>
    public override string ToString() => Content;
}
