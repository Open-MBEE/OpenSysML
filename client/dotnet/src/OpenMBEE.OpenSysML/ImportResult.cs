using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using OpenMBEE.OpenSysML.Internal;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// What <see cref="Model.ImportDataAsync"/> did: what each data file set, the notation the values went into, and the
/// <see cref="Model"/> they made, which the service holds under a hash of its own. The model imported into is left as it
/// was; nothing is written to disk.
/// </summary>
public sealed record ImportResult
{
    /// <summary>Creates a result, copying its collections.</summary>
    /// <param name="files">What each data file set, one per file, in import order.</param>
    /// <param name="content">The edited notation of a single-document model the import changed; null otherwise.</param>
    /// <param name="documents">The edited notation of every document the import changed.</param>
    /// <param name="model">The imported model; null for a dry run.</param>
    /// <param name="diagnostics">What the service reported.</param>
    /// <exception cref="ArgumentNullException">A collection is null.</exception>
    /// <exception cref="ArgumentException">A collection holds a null, or <paramref name="content"/> is empty.</exception>
    public ImportResult(IEnumerable<ImportedFile> files, string? content, IEnumerable<EditedDocument> documents, Model? model, IEnumerable<Diagnostic> diagnostics)
    {
        if (content is { Length: 0 })
        {
            throw new ArgumentException("Edited notation is not empty; null answers none.", nameof(content));
        }

        Files = new ValueList<ImportedFile>(files);
        Content = content;
        Documents = new ValueList<EditedDocument>(documents);
        Model = model;
        Diagnostics = new ValueList<Diagnostic>(diagnostics);
    }

    /// <summary>What each data file set, one per file, in the order they were imported.</summary>
    public ValueList<ImportedFile> Files { get; }

    /// <summary>
    /// The edited notation of a single-document model the import changed, byte-identical to its source outside the
    /// values set; null on a dry run, when the import set no value (the model is then as it was), and for a model of
    /// several documents, whose edited notation is in <see cref="Documents"/> alone.
    /// </summary>
    public string? Content { get; }

    /// <summary>
    /// The edited notation of every document the import changed, named as the parse named it; empty on a dry run, when
    /// it set no value, and from a service without <see cref="Capabilities.EditDocuments"/>, which answers
    /// <see cref="Content"/> alone.
    /// </summary>
    public ValueList<EditedDocument> Documents { get; }

    /// <summary>
    /// The imported model, which the service parsed and holds under its own hash: a new handle borrowing the connection
    /// the import was made over, with no roots and no parse diagnostics, as a model named by hash alone. An import that
    /// set no value answers the model imported into, under its hash. Null on a dry run, which made no model.
    /// </summary>
    public Model? Model { get; }

    /// <summary>What the service reported; empty when it reported nothing.</summary>
    public ValueList<Diagnostic> Diagnostics { get; }

    /// <summary>Whether the import only reported the values it would set: <see cref="Model"/>, <see cref="Content"/> and <see cref="Documents"/> are then empty.</summary>
    public bool IsDryRun => Model is null;

    /// <summary>How many values every file set together.</summary>
    public long Values => Files.Sum(file => file.Values);

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
    /// <exception cref="InvalidOperationException">
    /// There is no <see cref="Content"/>: the import was a dry run, set no value, or changed a model of several documents;
    /// write each of <see cref="Documents"/> with <see cref="EditedDocument.SaveAsync"/> instead.
    /// </exception>
    /// <exception cref="UnwritableException">The file exists and <see cref="SaveOptions.Overwrite"/> is not set, or it is a directory or a link, or its directory does not exist; nothing was written.</exception>
    /// <exception cref="System.IO.IOException">The file system failed the write; nothing is left of it.</exception>
    /// <exception cref="UnauthorizedAccessException">The file system denied the write; nothing is left of it.</exception>
    public Task<string> SaveAsync(string path, SaveOptions? options = null, CancellationToken cancellationToken = default)
    {
        ArgumentException.ThrowIfNullOrEmpty(path);
        if (Content is null)
        {
            throw new InvalidOperationException(
                IsDryRun
                    ? "A dry run changes nothing, so it has no notation to write."
                    : "The import answered no single document's notation: it set no value, or changed a model of several documents; write each of Documents with EditedDocument.SaveAsync.");
        }

        options ??= SaveOptions.Default;
        return SafeWriter.WriteAsync(path, SafeWriter.Utf8(Content), [], options.Overwrite, null, null, cancellationToken);
    }
}
