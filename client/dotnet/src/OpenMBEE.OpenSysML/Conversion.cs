using System;
using System.Collections.Generic;
using System.Threading;
using System.Threading.Tasks;
using OpenMBEE.OpenSysML.Internal;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// A model written out in another format by <see cref="Connection.ConvertAsync"/> or <see cref="Model.ConvertAsync"/>.
/// A round trip is defined on the model, not on the bytes: notation written back out is re-indented from its source,
/// and a trip through Turtle returns an equivalent model rather than identical text.
/// </summary>
public sealed record Conversion
{
    /// <summary>Creates a conversion, copying its diagnostics and library references.</summary>
    /// <param name="content">The converted model.</param>
    /// <param name="fromFormat">The format the source was read as, canonically named.</param>
    /// <param name="toFormat">The format <paramref name="content"/> is written in, canonically named.</param>
    /// <param name="diagnostics">The syntax errors tolerated, and the semantic check's diagnostics when asked for.</param>
    /// <param name="experimental">Whether either format goes through the experimental RDF mapping.</param>
    /// <param name="experimentalNotice">What is experimental about it; empty when it is not.</param>
    /// <param name="libraryReferences">The standard-library elements an <c>api-json</c> conversion refers to.</param>
    /// <exception cref="ArgumentNullException">A string, <paramref name="diagnostics"/> or <paramref name="libraryReferences"/> is null.</exception>
    /// <exception cref="ArgumentException">A diagnostic or a library reference is null.</exception>
    public Conversion(
        string content,
        string fromFormat,
        string toFormat,
        IEnumerable<Diagnostic> diagnostics,
        bool experimental,
        string experimentalNotice,
        IEnumerable<LibraryReference> libraryReferences)
    {
        ArgumentNullException.ThrowIfNull(content);
        ArgumentNullException.ThrowIfNull(fromFormat);
        ArgumentNullException.ThrowIfNull(toFormat);
        ArgumentNullException.ThrowIfNull(experimentalNotice);
        Content = content;
        FromFormat = fromFormat;
        ToFormat = toFormat;
        Diagnostics = new ValueList<Diagnostic>(diagnostics);
        Experimental = experimental;
        ExperimentalNotice = experimentalNotice;
        LibraryReferences = new ValueList<LibraryReference>(libraryReferences);
    }

    /// <summary>The converted model.</summary>
    public string Content { get; }

    /// <summary>The format the source was read as, canonically named (<c>sysml</c>, <c>ttl</c>, <c>api-json</c>), so a caller that let it be inferred learns what it was inferred as.</summary>
    public string FromFormat { get; }

    /// <summary>The format <see cref="Content"/> is written in, canonically named.</summary>
    public string ToFormat { get; }

    /// <summary>
    /// The syntax errors tolerated under <see cref="ConvertOptions.TolerateSyntaxErrors"/>, and, under
    /// <see cref="ConvertOptions.IncludeCheckDiagnostics"/>, the semantic check's diagnostics, each with its own severity.
    /// </summary>
    public ValueList<Diagnostic> Diagnostics { get; }

    /// <summary>
    /// Whether either format is RDF or the API's JSON element form, whose mapping is experimental: it covers model
    /// structure and the behavior its bodies state, refuses what it cannot write back, and its vocabulary may change
    /// without a compatibility path. Notation to notation is stable.
    /// </summary>
    public bool Experimental { get; }

    /// <summary>What is experimental about the conversion, in the service's own wording; empty when it is not.</summary>
    public string ExperimentalNotice { get; }

    /// <summary>
    /// The standard-library elements a successful <c>api-json</c> conversion refers to, compact output included, listed
    /// here rather than written as roots of the element array; empty for every other format, and from a service without
    /// <see cref="Capabilities.ConvertLibraryReferences"/>. <see cref="ConvertOptions.IncludeLibraryStubs"/> writes them
    /// as roots as well.
    /// </summary>
    public ValueList<LibraryReference> LibraryReferences { get; }

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
    /// <exception cref="UnwritableException">The file exists and <see cref="SaveOptions.Overwrite"/> is not set, or it is a directory or a link, or its directory does not exist; nothing was written.</exception>
    /// <exception cref="System.IO.IOException">The file system failed the write; nothing is left of it.</exception>
    /// <exception cref="UnauthorizedAccessException">The file system denied the write; nothing is left of it.</exception>
    public Task<string> SaveAsync(string path, SaveOptions? options = null, CancellationToken cancellationToken = default) =>
        PublishAsync(path, options, null, cancellationToken);

    /// <summary>The model's text.</summary>
    /// <returns><see cref="Content"/>.</returns>
    public override string ToString() => Content;

    /// <summary>Writes the content, calling <paramref name="beforePublish"/> just before the file is moved into place.</summary>
    internal Task<string> PublishAsync(string path, SaveOptions? options, Action<string>? beforePublish, CancellationToken cancellationToken)
    {
        ArgumentException.ThrowIfNullOrEmpty(path);
        options ??= SaveOptions.Default;
        return SafeWriter.WriteAsync(path, SafeWriter.Utf8(Content), [], options.Overwrite, null, beforePublish, cancellationToken);
    }
}
