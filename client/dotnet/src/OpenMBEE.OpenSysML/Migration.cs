using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using OpenMBEE.OpenSysML.Internal;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// A SysML v1 model migrated to v2 by <see cref="Connection.MigrateAsync"/>, with the account of what became of every
/// v1 element. Migration is ledgered, not lossless: every element lands in <see cref="Report"/> as mapped,
/// approximated, unmapped or skipped. It is experimental.
/// </summary>
public sealed record Migration
{
    /// <summary>Creates a migration, copying its files.</summary>
    /// <param name="content">The migrated model.</param>
    /// <param name="fromFormat">The v1 form read, canonically <c>xmi</c>.</param>
    /// <param name="toFormat">The format <paramref name="content"/> is written in.</param>
    /// <param name="report">What became of every element.</param>
    /// <param name="results">The JSON index of the result snapshots the v1 tool stored, when asked for; else empty.</param>
    /// <param name="files">The files the model refers to by relative path.</param>
    /// <param name="experimental">Whether the service marked the migration experimental, as it marks every one.</param>
    /// <param name="experimentalNotice">What is experimental about it.</param>
    /// <param name="sourcePath">The full path of the v1 file migrated, in this process's file system, when the source was a file; null for inline content.</param>
    /// <exception cref="ArgumentNullException">A string, <paramref name="report"/> or <paramref name="files"/> is null.</exception>
    /// <exception cref="ArgumentException">A file is null.</exception>
    public Migration(
        string content,
        string fromFormat,
        string toFormat,
        MigrationReport report,
        string results,
        IEnumerable<MigrationFile> files,
        bool experimental,
        string experimentalNotice,
        string? sourcePath)
    {
        ArgumentNullException.ThrowIfNull(content);
        ArgumentNullException.ThrowIfNull(fromFormat);
        ArgumentNullException.ThrowIfNull(toFormat);
        ArgumentNullException.ThrowIfNull(report);
        ArgumentNullException.ThrowIfNull(results);
        ArgumentNullException.ThrowIfNull(experimentalNotice);
        Content = content;
        FromFormat = fromFormat;
        ToFormat = toFormat;
        Report = report;
        Results = results;
        Files = new ValueList<MigrationFile>(files);
        Experimental = experimental;
        ExperimentalNotice = experimentalNotice;
        SourcePath = sourcePath;
    }

    /// <summary>The migrated model, in <see cref="ToFormat"/>.</summary>
    public string Content { get; }

    /// <summary>The v1 form read, canonically <c>xmi</c>: <c>uml</c> and <c>mdzip</c> name the same reader.</summary>
    public string FromFormat { get; }

    /// <summary>The format <see cref="Content"/> is written in, canonically named.</summary>
    public string ToFormat { get; }

    /// <summary>What became of every element: its summary and counts always, every verdict and the report's text when asked for.</summary>
    public MigrationReport Report { get; }

    /// <summary>
    /// The result snapshots the v1 tool stored, as the JSON <c>sysml -compare-results</c> reads, when
    /// <see cref="MigrateOptions.Results"/> asked and the model holds any; empty otherwise.
    /// </summary>
    public string Results { get; }

    /// <summary>The files the model refers to by relative path, such as pictures v1 comments embed, in the service's order.</summary>
    public ValueList<MigrationFile> Files { get; }

    /// <summary>Whether the service marked the migration experimental, as it marks every one.</summary>
    public bool Experimental { get; }

    /// <summary>What is experimental about the migration, in the service's own wording.</summary>
    public string ExperimentalNotice { get; }

    /// <summary>
    /// The full path, in this process's file system, of the v1 file migrated when the source was one; null for inline
    /// content. <see cref="WriteAsync"/> never replaces it.
    /// </summary>
    public string? SourcePath { get; }

    /// <summary>
    /// Writes the migrated model to a file and each of <see cref="Files"/> at its relative path below that file's
    /// directory, as <c>sysml -migrate -o</c> writes them. Every destination is judged before anything is written: a
    /// file path that is absolute, names a drive or a share, climbs out with <c>..</c>, is not a portable file name, lands
    /// where another of the files does, or reaches outside the directory through a symbolic link or junction is refused,
    /// and so is the v1 model the migration came from, whatever <see cref="SaveOptions.Overwrite"/> says. Each file is
    /// then written whole beside its destination and moved into place, the model last. A failure or cancellation removes
    /// every file and directory the write created; several files are not published atomically, so a file an overwrite
    /// already replaced stays replaced.
    /// </summary>
    /// <param name="path">The file to write the model to, in this process's file system.</param>
    /// <param name="options">Whether existing files are replaced; null for <see cref="SaveOptions.Default"/>, which refuses to.</param>
    /// <param name="cancellationToken">Cancels the write, which then leaves nothing of it behind.</param>
    /// <returns>The full path the model was written to.</returns>
    /// <exception cref="ArgumentNullException"><paramref name="path"/> is null.</exception>
    /// <exception cref="ArgumentException"><paramref name="path"/> is empty.</exception>
    /// <exception cref="UnwritableException">A destination was refused; nothing was written.</exception>
    /// <exception cref="System.IO.IOException">The file system failed the write; what it created is removed.</exception>
    /// <exception cref="UnauthorizedAccessException">The file system denied the write; what it created is removed.</exception>
    public Task<string> WriteAsync(string path, SaveOptions? options = null, CancellationToken cancellationToken = default) =>
        PublishAsync(path, options, null, cancellationToken);

    /// <summary>The model's text.</summary>
    /// <returns><see cref="Content"/>.</returns>
    public override string ToString() => Content;

    /// <summary>Writes the migration, calling <paramref name="beforePublish"/> just before each file is moved into place.</summary>
    internal Task<string> PublishAsync(string path, SaveOptions? options, Action<string>? beforePublish, CancellationToken cancellationToken)
    {
        ArgumentException.ThrowIfNullOrEmpty(path);
        options ??= SaveOptions.Default;
        var assets = Files.Select(file => (file.Path, file.Content)).ToList();
        return SafeWriter.WriteAsync(path, SafeWriter.Utf8(Content), assets, options.Overwrite, SourcePath, beforePublish, cancellationToken);
    }
}
