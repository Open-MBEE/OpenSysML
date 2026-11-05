using System;
using System.Threading;
using System.Threading.Tasks;
using OpenMBEE.OpenSysML.Internal;

namespace OpenMBEE.OpenSysML;

/// <summary>The notation of one document of a model as an operation left it, named as the model names the document.</summary>
public sealed record EditedDocument
{
    /// <summary>Creates a document.</summary>
    /// <param name="name">The document's name as the parse named it, or the name the service gave a document it made.</param>
    /// <param name="content">The document's notation, byte-identical to the source outside what the operation changed.</param>
    /// <exception cref="ArgumentNullException">An argument is null.</exception>
    public EditedDocument(string name, string content)
    {
        ArgumentNullException.ThrowIfNull(name);
        ArgumentNullException.ThrowIfNull(content);
        Name = name;
        Content = content;
    }

    /// <summary>
    /// The document's name as the parse named it: the file path of a file, the name of an inline document, or the
    /// position it was given when it was named neither; or the name the service gave a document it made.
    /// </summary>
    public string Name { get; }

    /// <summary>The document's notation, byte-identical to the source outside what the operation changed.</summary>
    public string Content { get; }

    /// <summary>
    /// Writes <see cref="Content"/> to a file, byte for byte as UTF-8: written whole to a temporary file beside it, then
    /// moved into place, so a failure or cancellation leaves no partial file and an existing one as it was. The path is
    /// chosen by the caller, never taken from <see cref="Name"/>, which names the document as the service read it.
    /// </summary>
    /// <param name="path">The file to write, in this process's file system.</param>
    /// <param name="options">Whether an existing file is replaced; null for <see cref="SaveOptions.Default"/>, which refuses to.</param>
    /// <param name="cancellationToken">Cancels the write, which then leaves nothing behind.</param>
    /// <returns>The full path written.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="path"/> is null.</exception>
    /// <exception cref="ArgumentException"><paramref name="path"/> is empty.</exception>
    /// <exception cref="UnwritableException">The file exists and <see cref="SaveOptions.Overwrite"/> is not set, or it is a directory or a link, or its directory does not exist; nothing was written.</exception>
    /// <exception cref="System.IO.IOException">The file system failed the write; nothing is left of it.</exception>
    /// <exception cref="UnauthorizedAccessException">The file system denied the write; nothing is left of it.</exception>
    public Task<string> SaveAsync(string path, SaveOptions? options = null, CancellationToken cancellationToken = default) =>
        PublishAsync(path, options, null, cancellationToken);

    /// <summary>Writes the content, calling <paramref name="beforePublish"/> just before the file is moved into place.</summary>
    internal Task<string> PublishAsync(string path, SaveOptions? options, Action<string>? beforePublish, CancellationToken cancellationToken)
    {
        ArgumentException.ThrowIfNullOrEmpty(path);
        options ??= SaveOptions.Default;
        return SafeWriter.WriteAsync(path, SafeWriter.Utf8(Content), [], options.Overwrite, null, beforePublish, cancellationToken);
    }
}
