using System;
using System.Collections.Generic;
using System.Threading;
using System.Threading.Tasks;
using OpenMBEE.OpenSysML.Internal;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// The notation a batch of edits produced, and what each edit changed. The model the edits were applied to is as it
/// was: the service answers the edited notation without parsing it, so load it to read the edited model.
/// </summary>
public sealed record EditResult
{
    /// <summary>Creates a result, copying its collections.</summary>
    /// <param name="content">The edited notation of a single-document model; null for a model of several documents, whose notation is in <paramref name="documents"/> alone.</param>
    /// <param name="applied">What each edit changed, grouped by document in the order <paramref name="documents"/> lists them and in batch order within one.</param>
    /// <param name="documents">The edited notation of every document the edits rewrote, the edited document first, then the others in name order.</param>
    /// <param name="diagnostics">What the service reported while editing.</param>
    /// <exception cref="ArgumentNullException">A collection is null.</exception>
    /// <exception cref="ArgumentException">An element of a collection is null.</exception>
    public EditResult(string? content, IEnumerable<AppliedEdit> applied, IEnumerable<EditedDocument> documents, IEnumerable<Diagnostic> diagnostics)
    {
        Content = content;
        Applied = new ValueList<AppliedEdit>(applied);
        Documents = new ValueList<EditedDocument>(documents);
        Diagnostics = new ValueList<Diagnostic>(diagnostics);
    }

    /// <summary>
    /// The edited notation of a single-document model, byte-identical to its source outside what the edits changed;
    /// null for a model of several documents, whose edited notation is in <see cref="Documents"/> alone, so it is never
    /// mistaken for the whole model. A model named by hash alone, whose documents this client does not know, reads as
    /// one of several when the service answers no notation for it while naming documents.
    /// </summary>
    public string? Content { get; }

    /// <summary>Whether the edited model has several documents, so <see cref="Content"/> is null and <see cref="SaveAsync"/> refuses.</summary>
    public bool HasSeveralDocuments => Content is null;

    /// <summary>What each edit changed, grouped by document in the order <see cref="Documents"/> lists them and in batch order within one.</summary>
    public ValueList<AppliedEdit> Applied { get; }

    /// <summary>
    /// The edited notation of every document the edits rewrote, named as the parse named it: the document edited first,
    /// then the others in name order. A document of several the edits left as parsed is not listed; the one document of
    /// a single-document model always is. Empty from a service without <see cref="Capabilities.EditDocuments"/>, which
    /// answers <see cref="Content"/> alone.
    /// </summary>
    public ValueList<EditedDocument> Documents { get; }

    /// <summary>What the service reported while editing; empty when it reported nothing.</summary>
    public ValueList<Diagnostic> Diagnostics { get; }

    /// <summary>
    /// Writes <see cref="Content"/> to a file, byte for byte as UTF-8: written whole to a temporary file beside it, then
    /// moved into place, so a failure or cancellation leaves no partial file and an existing one as it was.
    /// </summary>
    /// <param name="path">The file to write, in this process's file system.</param>
    /// <param name="options">Whether an existing file is replaced; null for <see cref="SaveOptions.Default"/>, which refuses to.</param>
    /// <param name="cancellationToken">Cancels the write, which then leaves nothing behind.</param>
    /// <returns>The full path written.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="path"/> is null.</exception>
    /// <exception cref="ArgumentException"><paramref name="path"/> is empty.</exception>
    /// <exception cref="InvalidOperationException">The edits rewrote a model of several documents; write each of <see cref="Documents"/> with <see cref="EditedDocument.SaveAsync"/>.</exception>
    /// <exception cref="UnwritableException">The file exists and <see cref="SaveOptions.Overwrite"/> is not set, or it is a directory or a link, or its directory does not exist; nothing was written.</exception>
    /// <exception cref="System.IO.IOException">The file system failed the write; nothing is left of it.</exception>
    /// <exception cref="UnauthorizedAccessException">The file system denied the write; nothing is left of it.</exception>
    public Task<string> SaveAsync(string path, SaveOptions? options = null, CancellationToken cancellationToken = default) =>
        PublishAsync(path, options, null, cancellationToken);

    /// <summary>Writes the content, calling <paramref name="beforePublish"/> just before the file is moved into place.</summary>
    internal Task<string> PublishAsync(string path, SaveOptions? options, Action<string>? beforePublish, CancellationToken cancellationToken)
    {
        ArgumentException.ThrowIfNullOrEmpty(path);
        if (Content is null)
        {
            throw new InvalidOperationException(
                "The edits rewrote a model of several documents, which no one file holds; write each of Documents with EditedDocument.SaveAsync.");
        }

        options ??= SaveOptions.Default;
        return SafeWriter.WriteAsync(path, SafeWriter.Utf8(Content), [], options.Overwrite, null, beforePublish, cancellationToken);
    }
}
